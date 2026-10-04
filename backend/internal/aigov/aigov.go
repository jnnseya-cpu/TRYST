// Package aigov is the AI Governance gate of the TRYST AI Operating System
// (docs/spec/09_AI_Operating_System.md §5.4, §13; FR-080–FR-083, D-25, D-28).
//
// Every agent action passes through Decide before it runs. The gate is deterministic code,
// not a model: an agent cannot argue its way past it, and it fails closed. It enforces
// TRYST's bounded autonomy:
//
//   - an agent may only use actions listed in its manifest;
//   - an agent's autonomy level caps what it may run without a person;
//   - a permanent action, a safety action against a member, money movement, a model or
//     code change reaching production, and any first contact on a member's behalf always
//     need a named human, whatever the level (P4; UK GDPR Art 22);
//   - a kill switch stops an agent (or every agent) immediately;
//   - every decision is written to a hash-chained, append-only audit log.
package aigov

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// Level is how far an agent may act on its own (09 §5.2).
type Level int

const (
	// L0 observes and reports only.
	L0Observe Level = iota
	// L1 recommends; a person carries out every action.
	L1Recommend
	// L2 runs reversible actions on its own inside its budget; a person can undo them.
	L2Reversible
	// L3 runs pre-approved runbooks (rollback, scale, restart) on its own.
	L3Runbook
)

func (l Level) String() string {
	switch l {
	case L0Observe:
		return "L0-observe"
	case L1Recommend:
		return "L1-recommend"
	case L2Reversible:
		return "L2-reversible"
	case L3Runbook:
		return "L3-runbook"
	}
	return fmt.Sprintf("L%d", int(l))
}

// Class is the kind of effect an action has. The class, not the agent's wishes, decides
// whether a person must approve.
type Class int

const (
	Read             Class = iota // read data inside the agent's scope
	Recommend                     // produce a recommendation, draft or report
	Reversible                    // change something a person can undo (tighten a policy, open a ticket)
	Runbook                       // run a pre-approved operational runbook (rollback, scale, restart)
	Irreversible                  // cannot be undone (delete, publish, send to an outside party)
	MemberSafety                  // a safety action against a member (freeze, removal, ban)
	MoneyMovement                 // refund, payout, price change, credit
	ProductionChange              // a code, model or prompt change reaching production
	ActAsMember                   // contact a person on a member's behalf
)

var classNames = map[Class]string{
	Read: "read", Recommend: "recommend", Reversible: "reversible", Runbook: "runbook",
	Irreversible: "irreversible", MemberSafety: "member_safety", MoneyMovement: "money_movement",
	ProductionChange: "production_change", ActAsMember: "act_as_member",
}

func (c Class) String() string { return classNames[c] }

// alwaysHuman are the classes no autonomy level unlocks.
var alwaysHuman = map[Class]bool{
	Irreversible: true, MemberSafety: true, MoneyMovement: true, ProductionChange: true,
}

// minLevel is the autonomy level needed to run a class without a person.
var minLevel = map[Class]Level{
	Read: L0Observe, Recommend: L1Recommend, Reversible: L2Reversible, Runbook: L3Runbook,
}

// Manifest declares an agent to the registry (09 §5.1). Data scopes name stores or
// datasets; member-data scopes are refused for agents outside the member platform.
type Manifest struct {
	ID           string
	Owner        string // accountable human team
	Level        Level
	Actions      map[string]Class // action name → effect class
	DataScopes   []string
	MemberPlane  bool // runs inside the member platform (may hold member-data scopes)
	DailyBudget  int  // maximum autonomous actions per UTC day; 0 = none allowed
	SelfHosted   bool // its model runs on TRYST infrastructure (required for member data)
	ModelVersion string
}

// memberScopes are data scopes that hold personal or special-category member data.
var memberScopes = map[string]bool{
	"identity_db": true, "profile_db": true, "safety_db": true, "ban_db": true,
	"event_bus": true, "feature_store": true, "vector_store": true,
}

// Outcome of a decision.
type Outcome string

const (
	Allow      Outcome = "allow"
	NeedsHuman Outcome = "needs_human"
	Deny       Outcome = "deny"
)

// Request asks to run one action.
type Request struct {
	AgentID  string
	Action   string
	Subject  string // pseudonymous reference of what the action touches; never a name
	Approver string // named human who approved, if any
	Reason   string
}

// Decision is the gate's answer, with the reason a person will read.
type Decision struct {
	Outcome Outcome
	Reason  string
}

var (
	ErrUnknownAgent  = errors.New("unknown agent")
	ErrInvalidAgent  = errors.New("invalid manifest")
	ErrDuplicateID   = errors.New("agent already registered")
	ErrAuditTampered = errors.New("audit chain broken")
)

// Gate is the registry plus the decision function. Safe for concurrent use.
type Gate struct {
	mu        sync.Mutex
	agents    map[string]Manifest
	killed    map[string]bool
	killAll   bool
	used      map[string]int // agentID|day → autonomous actions used
	log       []Entry
	now       func() time.Time
	approvers map[string]bool // named humans allowed to approve
}

// New returns an empty gate. approvers are the named humans who may approve actions.
func New(now func() time.Time, approvers ...string) *Gate {
	if now == nil {
		now = time.Now
	}
	g := &Gate{agents: map[string]Manifest{}, killed: map[string]bool{}, used: map[string]int{},
		now: now, approvers: map[string]bool{}}
	for _, a := range approvers {
		g.approvers[a] = true
	}
	return g
}

// Register validates and adds a manifest.
func (g *Gate) Register(m Manifest) error {
	if m.ID == "" || m.Owner == "" || len(m.Actions) == 0 {
		return fmt.Errorf("%w: id, owner and actions are required", ErrInvalidAgent)
	}
	if m.Level < L0Observe || m.Level > L3Runbook {
		return fmt.Errorf("%w: level %d", ErrInvalidAgent, m.Level)
	}
	for _, s := range m.DataScopes {
		if memberScopes[s] && !m.MemberPlane {
			return fmt.Errorf("%w: %s is outside the member platform and cannot hold scope %s", ErrInvalidAgent, m.ID, s)
		}
		if memberScopes[s] && !m.SelfHosted {
			return fmt.Errorf("%w: %s holds member data and must use a self-hosted model", ErrInvalidAgent, m.ID)
		}
	}
	for name, c := range m.Actions {
		if c == ActAsMember {
			return fmt.Errorf("%w: action %s: no agent may act as a member (P4)", ErrInvalidAgent, name)
		}
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, ok := g.agents[m.ID]; ok {
		return ErrDuplicateID
	}
	g.agents[m.ID] = m
	g.appendLocked("register", m.ID, "", "", fmt.Sprintf("level=%s owner=%s", m.Level, m.Owner))
	return nil
}

// Kill stops one agent; KillAll stops every agent. Resume undoes it. Each needs a named human.
func (g *Gate) Kill(agentID, by string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.killed[agentID] = true
	g.appendLocked("kill", agentID, "", by, "")
}

func (g *Gate) KillAll(by string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.killAll = true
	g.appendLocked("kill_all", "*", "", by, "")
}

func (g *Gate) Resume(agentID, by string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if agentID == "*" {
		g.killAll = false
	} else {
		delete(g.killed, agentID)
	}
	g.appendLocked("resume", agentID, "", by, "")
}

// Decide answers one request and records the decision.
func (g *Gate) Decide(r Request) Decision {
	g.mu.Lock()
	defer g.mu.Unlock()
	d := g.decideLocked(r)
	g.appendLocked("decide:"+string(d.Outcome), r.AgentID, r.Action, r.Approver, d.Reason+subjectNote(r.Subject))
	return d
}

func subjectNote(s string) string {
	if s == "" {
		return ""
	}
	return " subject=" + s
}

func (g *Gate) decideLocked(r Request) Decision {
	m, ok := g.agents[r.AgentID]
	if !ok {
		return Decision{Deny, "unknown agent"}
	}
	if g.killAll || g.killed[r.AgentID] {
		return Decision{Deny, "agent stopped by kill switch"}
	}
	c, ok := m.Actions[r.Action]
	if !ok {
		return Decision{Deny, "action not in manifest"}
	}
	approved := r.Approver != "" && g.approvers[r.Approver]
	if r.Approver != "" && !approved {
		return Decision{Deny, "approver is not a registered named human"}
	}
	if alwaysHuman[c] {
		if approved {
			return Decision{Allow, c.String() + " approved by named human"}
		}
		return Decision{NeedsHuman, c.String() + " always needs a named human"}
	}
	if m.Level < minLevel[c] {
		if approved {
			return Decision{Allow, c.String() + " above autonomy level; approved by named human"}
		}
		return Decision{NeedsHuman, fmt.Sprintf("%s needs %s; agent is %s", c, minLevel[c], m.Level)}
	}
	if c == Read || c == Recommend {
		return Decision{Allow, c.String() + " within level"}
	}
	key := r.AgentID + "|" + g.now().UTC().Format("2006-01-02")
	if g.used[key] >= m.DailyBudget {
		if approved {
			return Decision{Allow, "budget exhausted; approved by named human"}
		}
		return Decision{NeedsHuman, "daily autonomous budget exhausted"}
	}
	g.used[key]++
	return Decision{Allow, fmt.Sprintf("%s within level and budget (%d/%d)", c, g.used[key], m.DailyBudget)}
}

// Agents lists registered manifests by id.
func (g *Gate) Agents() []Manifest {
	g.mu.Lock()
	defer g.mu.Unlock()
	out := make([]Manifest, 0, len(g.agents))
	for _, m := range g.agents {
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Entry is one audit record. Hash covers the previous hash and this entry's fields, so
// any edit or deletion breaks the chain.
type Entry struct {
	Seq      int
	At       time.Time
	Kind     string
	AgentID  string
	Action   string
	Approver string
	Detail   string
	Prev     string
	Hash     string
}

func (e Entry) digest() string {
	h := sha256.New()
	fmt.Fprintf(h, "%d\x00%s\x00%s\x00%s\x00%s\x00%s\x00%s\x00%s",
		e.Seq, e.At.UTC().Format(time.RFC3339Nano), e.Kind, e.AgentID, e.Action, e.Approver,
		strings.ReplaceAll(e.Detail, "\x00", ""), e.Prev)
	return hex.EncodeToString(h.Sum(nil))
}

func (g *Gate) appendLocked(kind, agentID, action, approver, detail string) {
	prev := ""
	if n := len(g.log); n > 0 {
		prev = g.log[n-1].Hash
	}
	e := Entry{Seq: len(g.log), At: g.now(), Kind: kind, AgentID: agentID, Action: action,
		Approver: approver, Detail: detail, Prev: prev}
	e.Hash = e.digest()
	g.log = append(g.log, e)
}

// Audit returns a copy of the log.
func (g *Gate) Audit() []Entry {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]Entry(nil), g.log...)
}

// VerifyChain checks an exported log end to end.
func VerifyChain(log []Entry) error {
	prev := ""
	for i, e := range log {
		if e.Seq != i || e.Prev != prev || e.digest() != e.Hash {
			return fmt.Errorf("%w at entry %d", ErrAuditTampered, i)
		}
		prev = e.Hash
	}
	return nil
}
