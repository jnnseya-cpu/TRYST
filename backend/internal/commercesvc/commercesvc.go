// Package commercesvc is TRYST's commerce service (docs/spec/03_Backend.md §10; 02 §5.9;
// ToS cl. 16; FR-078, FR-117; D-22, D-53): entitlements, checkout for subscriptions, Keys
// packs and the refundable deposit, signature-verified idempotent webhooks, the Keys
// ledger, one-step cancellation with the 14-day cooling-off refund, and vouchers. It keeps
// its own database (COMMERCE-DB).
//
// The processor sees only an opaque billing reference and the product key. Entitlements
// are updated only from verified events, and never depend on a single processor.
package commercesvc

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"math"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jnnseya-cpu/tryst/backend/internal/gate"
	"github.com/jnnseya-cpu/tryst/backend/internal/identity"
	"github.com/jnnseya-cpu/tryst/backend/internal/keys"
	"github.com/jnnseya-cpu/tryst/backend/internal/payments"
	"github.com/jnnseya-cpu/tryst/backend/internal/problem"
	"github.com/jnnseya-cpu/tryst/backend/internal/store"
)

// Item is a catalogue entry (01 §10.2).
type Item struct {
	Key          string `json:"key"`
	Name         string `json:"name"`
	PriceMinor   int64  `json:"price_minor"`
	Currency     string `json:"currency"`
	Period       string `json:"period,omitempty"` // month | year for subscriptions
	Keys         int    `json:"keys,omitempty"`   // Keys credited by a one-off purchase
	Kind         string `json:"kind"`             // plan | addon | keys | deposit
	Rank         int    `json:"-"`
	Description  string `json:"description"`
	subscription bool
}

// Catalogue is the UK price list. Plans are mutually exclusive; GHOST is an add-on.
var Catalogue = []Item{
	{Key: "tryst_plus", Name: "TRYST+", PriceMinor: 2499, Currency: "GBP", Period: "month", Kind: "plan", Rank: 1, subscription: true,
		Description: "Unlimited intents, filters, incognito, travel mode, extended media, read-receipt control"},
	{Key: "tryst_plus_annual", Name: "TRYST+ (annual)", PriceMinor: 17988, Currency: "GBP", Period: "year", Kind: "plan", Rank: 1, subscription: true,
		Description: "TRYST+ billed yearly: £14.99 a month"},
	{Key: "duo", Name: "DUO", PriceMinor: 3999, Currency: "GBP", Period: "month", Kind: "plan", Rank: 2, subscription: true,
		Description: "For couples: linked profile, dual-consent controls, joint veto"},
	{Key: "envoy", Name: "ENVOY", PriceMinor: 4999, Currency: "GBP", Period: "month", Kind: "plan", Rank: 3, subscription: true,
		Description: "Envoy proposes outreach and handles logistics; you approve every intent"},
	{Key: "ghost", Name: "GHOST", PriceMinor: 799, Currency: "GBP", Period: "month", Kind: "addon", subscription: true,
		Description: "Advanced discretion: decoy skins, custom disappearing times"},
	{Key: "keys_60", Name: "60 Keys", PriceMinor: 999, Currency: "GBP", Keys: 60, Kind: "keys",
		Description: "An intent costs 5 Keys (15 from a couple to a single member), fully refunded when they reply"},
	{Key: "deposit", Name: "Refundable deposit", PriceMinor: 999, Currency: "GBP", Keys: 60, Kind: "deposit",
		Description: "£9.99, credited in full as 60 Keys"},
}

func item(key string) (Item, bool) {
	for _, it := range Catalogue {
		if it.Key == key {
			return it, true
		}
	}
	return Item{}, false
}

// CoolingOff is the cancellation period for distance contracts (CCR 2013; ToS 16.4).
const CoolingOff = 14 * 24 * time.Hour

// MemberAuth is satisfied by *identity.Server.
type MemberAuth interface {
	Authenticate(r *http.Request) (identity.Account, bool)
}

// ImmediateCanceller is implemented by providers that can end a subscription now.
type ImmediateCanceller interface {
	CancelSubscriptionNow(ctx context.Context, billingRef string) error
}

// sub is one subscription (a plan, or the GHOST add-on).
type sub struct {
	Item        string    `json:"item"`
	ID          string    `json:"id"`
	Status      string    `json:"status"` // active | past_due | canceled
	Started     time.Time `json:"started"`
	PeriodEnd   time.Time `json:"period_end"`
	CancelAtEnd bool      `json:"cancel_at_period_end"`
	Payment     string    `json:"payment,omitempty"` // latest payment intent
	Amount      int64     `json:"amount"`
	StartNow    bool      `json:"start_now"` // member asked to start within cooling-off
	NoticeAt    time.Time `json:"notice_at"` // pre-contract information acknowledged
	Refunded    int64     `json:"refunded,omitempty"`
}

func (s *sub) live(now time.Time) bool {
	if s == nil {
		return false
	}
	switch s.Status {
	case "active", "past_due", "trialing":
		return s.PeriodEnd.IsZero() || now.Before(s.PeriodEnd) || !s.CancelAtEnd
	}
	return false
}

type voucherPlan struct {
	Plan  string    `json:"plan"`
	Until time.Time `json:"until"`
}

// account is the commerce aggregate for one member (commerce.entitlement + keys_ledger).
type account struct {
	BillingRef string                `json:"billing_ref"`
	Subs       map[string]*sub       `json:"subs"`    // by catalogue key
	Pending    map[string]*sub       `json:"pending"` // notices acknowledged, checkout not completed
	Ledger     []keys.Entry          `json:"ledger"`
	Deposit    string                `json:"deposit_payment,omitempty"`
	Voucher    *voucherPlan          `json:"voucher_plan,omitempty"`
	Disputes   int                   `json:"disputes,omitempty"`
	Payments   map[string]paymentRec `json:"payments,omitempty"` // payment intent -> what it bought
}

type paymentRec struct {
	Item   string    `json:"item"`
	Amount int64     `json:"amount"`
	At     time.Time `json:"at"`
}

type voucher struct {
	Grant   string    `json:"grant"` // keys:<n> | plan:<key>:<days>
	Created time.Time `json:"created"`
	By      string    `json:"by"`
	UsedBy  string    `json:"used_by,omitempty"`
	UsedAt  time.Time `json:"used_at,omitempty"`
}

// Config wires the service.
type Config struct {
	Members  MemberAuth
	Provider payments.Provider // nil: purchases answer 503 payments_unavailable
	Store    store.Store
	Env      string
	// MinTier is the tier needed to pay (V1 in production: 02 §5.9).
	MinTier gate.Tier
	// StaffLead authenticates a T&S or finance lead for voucher issuing.
	StaffLead func(r *http.Request) (string, bool)
	// Notify records a masked notice for a member. May be nil.
	Notify func(account, kind, link string)
}

// Service is the HTTP handler.
type Service struct {
	cfg      Config
	mu       sync.Mutex
	now      func() time.Time
	accounts map[string]*account
	byRef    map[string]string // billing ref -> account
	bySub    map[string]string // subscription id -> account
	byPay    map[string]string // payment intent -> account
	failures map[string][]time.Time
	seen     map[string]bool // event ids applied by this process
	mux      *http.ServeMux
	dirty    map[string]bool
	dev      *DevProvider
	// SkipProcessor makes Erase leave the payment processor alone (re-applying erasures
	// after a restore; admin only).
	SkipProcessor bool
}

func New(cfg Config) (*Service, error) {
	if _, ok := cfg.Provider.(*DevProvider); ok && cfg.Env != "dev" {
		return nil, errors.New("the dev payment provider is forbidden outside the dev environment")
	}
	s := &Service{cfg: cfg, now: time.Now, accounts: map[string]*account{}, byRef: map[string]string{}, bySub: map[string]string{}, byPay: map[string]string{},
		failures: map[string][]time.Time{}, seen: map[string]bool{}, mux: http.NewServeMux(), dirty: map[string]bool{}}
	if dp, ok := cfg.Provider.(*DevProvider); ok {
		s.dev = dp
	}
	if err := s.load(context.Background()); err != nil {
		return nil, err
	}
	m := s.mux
	m.HandleFunc("GET /v1/entitlements", s.handleEntitlements)
	m.HandleFunc("POST /v1/deposit", s.handleDeposit)
	m.HandleFunc("POST /v1/keys/purchase", s.handleKeys)
	m.HandleFunc("POST /v1/subscription", s.handleSubscribe)
	m.HandleFunc("POST /v1/subscription/cancel", s.handleCancel)
	m.HandleFunc("POST /v1/vouchers/redeem", s.handleRedeem)
	m.HandleFunc("POST /v1/billing/stripe/webhook", s.handleWebhook)
	m.HandleFunc("POST /v1/billing/dev/complete", s.handleDevComplete)
	m.HandleFunc("POST /ops/v1/vouchers", s.handleIssueVouchers)
	return s, nil
}

// SubscriptionFor resolves "<billing ref>|<item>" to the processor's subscription id. Wire it
// into the provider adapter.
func (s *Service) SubscriptionFor(key string) (string, error) {
	ref, it, _ := strings.Cut(key, "|")
	s.mu.Lock()
	defer s.mu.Unlock()
	a := s.accounts[s.byRef[ref]]
	if a == nil || a.Subs[it] == nil || a.Subs[it].ID == "" {
		return "", errors.New("no subscription")
	}
	return a.Subs[it].ID, nil
}

func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	s.mux.ServeHTTP(w, r)
	s.flush(r.Context())
}

func fail(w http.ResponseWriter, status int, code, title string) {
	problem.Write(w, problem.Problem{Title: title, Status: status, Code: code})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func randomRef(prefix string, n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return prefix + hex.EncodeToString(b)
}

// --- persistence ---

func (s *Service) load(ctx context.Context) error {
	docs, err := s.cfg.Store.All(ctx, "account")
	if err != nil {
		return err
	}
	for id, b := range docs {
		var a account
		if err := json.Unmarshal(b, &a); err != nil {
			return err
		}
		s.index(id, &a)
	}
	return nil
}

func (s *Service) index(id string, a *account) {
	if a.Subs == nil {
		a.Subs = map[string]*sub{}
	}
	if a.Pending == nil {
		a.Pending = map[string]*sub{}
	}
	if a.Payments == nil {
		a.Payments = map[string]paymentRec{}
	}
	s.accounts[id] = a
	s.byRef[a.BillingRef] = id
	for _, sb := range a.Subs {
		if sb.ID != "" {
			s.bySub[sb.ID] = id
		}
	}
	for pi := range a.Payments {
		s.byPay[pi] = id
	}
}

func (s *Service) flush(ctx context.Context) {
	s.mu.Lock()
	docs := map[string][]byte{}
	for id := range s.dirty {
		docs[id], _ = json.Marshal(s.accounts[id])
	}
	s.dirty = map[string]bool{}
	s.mu.Unlock()
	for id, b := range docs {
		if err := s.cfg.Store.Put(ctx, "account", id, b); err != nil {
			log.Printf("persist commerce account failed: %v", err)
		}
	}
}

// acct returns the member's commerce record, creating it with a fresh billing reference.
// Caller holds s.mu.
func (s *Service) acct(id string) *account {
	a, ok := s.accounts[id]
	if !ok {
		a = &account{BillingRef: randomRef("br_", 12)}
		s.index(id, a)
		s.dirty[id] = true
	}
	return a
}

// --- entitlements ---

// Entitlement is what the member platform reads.
type Entitlement struct {
	Plan     string `json:"plan"` // "" (Verified, free) or a plan key
	Ghost    bool   `json:"ghost"`
	Keys     int    `json:"keys"`
	Deposit  bool   `json:"deposit_paid"`
	Disputed bool   `json:"-"`
}

// Entitled returns the member's current entitlement.
func (s *Service) Entitled(accountID string) Entitlement {
	s.mu.Lock()
	defer s.mu.Unlock()
	a := s.accounts[accountID]
	if a == nil {
		return Entitlement{}
	}
	return s.entitlement(a)
}

func (s *Service) entitlement(a *account) Entitlement {
	now := s.now()
	e := Entitlement{Keys: keys.Restore(a.Ledger).Balance(), Deposit: a.Deposit != "", Ghost: a.Subs["ghost"].live(now)}
	best := -1
	for k, sb := range a.Subs {
		it, _ := item(k)
		if it.Kind == "plan" && sb.live(now) && it.Rank > best {
			e.Plan, best = k, it.Rank
		}
	}
	if e.Plan == "" && a.Voucher != nil && now.Before(a.Voucher.Until) {
		e.Plan = a.Voucher.Plan
	}
	return e
}

func (s *Service) member(w http.ResponseWriter, r *http.Request) (identity.Account, bool) {
	acc, ok := s.cfg.Members.Authenticate(r)
	if !ok {
		fail(w, 401, "auth_required", "Please sign in again.")
	}
	return acc, ok
}

func (s *Service) handleEntitlements(w http.ResponseWriter, r *http.Request) {
	acc, ok := s.member(w, r)
	if !ok {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	a := s.acct(acc.ID)
	e := s.entitlement(a)
	type subView struct {
		Item        string    `json:"item"`
		Name        string    `json:"name"`
		Status      string    `json:"status"`
		PeriodEnd   time.Time `json:"period_end"`
		CancelAtEnd bool      `json:"cancel_at_period_end"`
		CoolingOff  bool      `json:"in_cooling_off"`
	}
	subs := []subView{}
	now := s.now()
	for k, sb := range a.Subs {
		if sb.Status == "canceled" {
			continue
		}
		it, _ := item(k)
		subs = append(subs, subView{k, it.Name, sb.Status, sb.PeriodEnd, sb.CancelAtEnd, now.Sub(sb.Started) < CoolingOff})
	}
	sort.Slice(subs, func(i, j int) bool { return subs[i].Item < subs[j].Item })
	out := map[string]any{
		"plan": e.Plan, "ghost": e.Ghost, "keys": e.Keys, "deposit_paid": e.Deposit, "subscriptions": subs,
		"catalogue": Catalogue, "payments_available": s.cfg.Provider != nil,
	}
	if a.Voucher != nil && now.Before(a.Voucher.Until) {
		out["voucher_plan"] = a.Voucher
	}
	writeJSON(w, 200, out)
}

// --- checkout ---

// checkout starts a processor checkout. Caller holds s.mu; it is released here, before the
// network call, and not re-taken.
func (s *Service) checkout(w http.ResponseWriter, r *http.Request, acc identity.Account, a *account, key string) {
	ref := a.BillingRef
	s.mu.Unlock()
	if s.cfg.Provider == nil {
		fail(w, 503, "payments_unavailable", "Payments aren't available right now. Please try again later.")
		return
	}
	if acc.Tier < s.cfg.MinTier {
		fail(w, 403, "tier_required", "Finish verifying your account before paying.")
		return
	}
	u, err := s.cfg.Provider.CreateCheckout(r.Context(), ref, key)
	if err != nil {
		log.Printf("checkout failed (%s): %v", s.cfg.Provider.Name(), err)
		fail(w, 502, "checkout_failed", "We couldn't start the payment. Please try again.")
		return
	}
	writeJSON(w, 201, map[string]any{"checkout_url": u})
}

func (s *Service) handleDeposit(w http.ResponseWriter, r *http.Request) {
	acc, ok := s.member(w, r)
	if !ok {
		return
	}
	s.mu.Lock()
	a := s.acct(acc.ID)
	if a.Deposit != "" {
		s.mu.Unlock()
		fail(w, 409, "deposit_paid", "Your deposit is already paid.")
		return
	}
	s.checkout(w, r, acc, a, "deposit")
}

func (s *Service) handleKeys(w http.ResponseWriter, r *http.Request) {
	acc, ok := s.member(w, r)
	if !ok {
		return
	}
	var in struct {
		Pack string `json:"pack"`
	}
	_ = json.NewDecoder(io.LimitReader(r.Body, 4<<10)).Decode(&in)
	if it, ok := item(in.Pack); !ok || it.Kind != "keys" {
		fail(w, 422, "validation_failed", "Choose a Keys pack.")
		return
	}
	s.mu.Lock()
	s.checkout(w, r, acc, s.acct(acc.ID), in.Pack)
}

// handleSubscribe starts a subscription checkout. The member must have seen the pre-contract
// information and must separately ask to start now, which affects the cooling-off refund
// (ToS 16.4); both are recorded.
func (s *Service) handleSubscribe(w http.ResponseWriter, r *http.Request) {
	acc, ok := s.member(w, r)
	if !ok {
		return
	}
	var in struct {
		Plan       string `json:"plan"`
		PreContrac bool   `json:"pre_contract_ack"`
		StartNow   bool   `json:"start_now"`
	}
	_ = json.NewDecoder(io.LimitReader(r.Body, 4<<10)).Decode(&in)
	it, ok := item(in.Plan)
	if !ok || !it.subscription {
		fail(w, 422, "validation_failed", "Choose a plan.")
		return
	}
	if !in.PreContrac {
		fail(w, 422, "pre_contract_required", "Please read the plan details before you subscribe.")
		return
	}
	if !in.StartNow {
		fail(w, 422, "start_now_required", "Confirm that you want the plan to start now. You can still cancel within 14 days and pay only for the days you used.")
		return
	}
	s.mu.Lock()
	a := s.acct(acc.ID)
	now := s.now()
	if a.Subs[in.Plan].live(now) {
		s.mu.Unlock()
		fail(w, 409, "already_subscribed", "You already have this.")
		return
	}
	if it.Kind == "plan" {
		for k, sb := range a.Subs {
			if o, _ := item(k); o.Kind == "plan" && sb.live(now) && !sb.CancelAtEnd {
				s.mu.Unlock()
				fail(w, 409, "change_plan", "Cancel your current plan first. You keep it until the end of the period you paid for.")
				return
			}
		}
	}
	a.Pending[in.Plan] = &sub{Item: in.Plan, StartNow: true, NoticeAt: now}
	s.dirty[acc.ID] = true
	s.checkout(w, r, acc, a, in.Plan)
}

// handleCancel is one-step cancellation (ToS 16.3): no questions, no retention offer. Within
// the 14-day cooling-off period the plan ends now and the unused part is refunded (16.4);
// after it, the plan runs to the end of the paid period.
func (s *Service) handleCancel(w http.ResponseWriter, r *http.Request) {
	acc, ok := s.member(w, r)
	if !ok {
		return
	}
	var in struct {
		Item string `json:"item"`
	}
	_ = json.NewDecoder(io.LimitReader(r.Body, 4<<10)).Decode(&in)
	s.mu.Lock()
	a := s.acct(acc.ID)
	now := s.now()
	key := in.Item
	if key == "" {
		for k, sb := range a.Subs {
			if it, _ := item(k); it.Kind == "plan" && sb.live(now) {
				key = k
			}
		}
	}
	sb := a.Subs[key]
	if !sb.live(now) || sb.CancelAtEnd {
		s.mu.Unlock()
		fail(w, 404, "not_found", "There's nothing to cancel.")
		return
	}
	pref, cooling, refund, payment := a.BillingRef+"|"+key, now.Sub(sb.Started) < CoolingOff, proportionateRefund(sb, now), sb.Payment
	s.mu.Unlock()
	if s.cfg.Provider == nil {
		fail(w, 503, "payments_unavailable", "We couldn't reach our payment provider. Please try again shortly.")
		return
	}

	// No lock across processor calls (the adapter resolves the subscription through us).
	var err error
	if cooling {
		if ic, ok := s.cfg.Provider.(ImmediateCanceller); ok {
			err = ic.CancelSubscriptionNow(r.Context(), pref)
		} else {
			err = s.cfg.Provider.CancelSubscription(r.Context(), pref)
		}
		if err == nil && refund > 0 && payment != "" {
			err = s.cfg.Provider.Refund(r.Context(), payment, refund)
		}
	} else {
		err = s.cfg.Provider.CancelSubscription(r.Context(), pref)
	}
	if err != nil {
		log.Printf("cancel failed: %v", err)
		fail(w, 502, "cancel_failed", "We couldn't reach our payment provider. Please try again shortly.")
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.dirty[acc.ID] = true
	if cooling {
		sb.Status, sb.PeriodEnd, sb.Refunded = "canceled", now, refund
		writeJSON(w, 200, map[string]any{"ended": now, "refund_minor": refund,
			"message": "Cancelled. Your plan has ended and we've refunded the days you didn't use. It reaches your card within 14 days."})
		return
	}
	sb.CancelAtEnd = true
	writeJSON(w, 200, map[string]any{"ends": sb.PeriodEnd, "refund_minor": 0,
		"message": "Cancelled. You keep your plan until the end of the period you've paid for, and you won't be charged again."})
}

// proportionateRefund refunds the unused part of the current payment: the member pays for
// whole days used, counting the first day (ToS 16.4).
func proportionateRefund(sb *sub, now time.Time) int64 {
	if sb.Amount <= 0 {
		return 0
	}
	it, _ := item(sb.Item)
	days := 30.0
	if it.Period == "year" {
		days = 365
	}
	used := math.Ceil(now.Sub(sb.Started).Hours() / 24)
	if used < 1 {
		used = 1
	}
	r := sb.Amount - int64(math.Round(float64(sb.Amount)*used/days))
	if r < 0 {
		return 0
	}
	return r
}

// --- vouchers ---

var voucherEnc = base32.StdEncoding.WithPadding(base32.NoPadding)

func normaliseCode(c string) string {
	return strings.NewReplacer("-", "", " ", "").Replace(strings.ToUpper(strings.TrimSpace(c)))
}

func codeID(c string) string {
	h := sha256.Sum256([]byte("tryst/voucher/v1\x00" + normaliseCode(c)))
	return hex.EncodeToString(h[:])
}

func (s *Service) handleRedeem(w http.ResponseWriter, r *http.Request) {
	acc, ok := s.member(w, r)
	if !ok {
		return
	}
	var in struct {
		Code string `json:"code"`
	}
	_ = json.NewDecoder(io.LimitReader(r.Body, 4<<10)).Decode(&in)
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	recent := s.failures[acc.ID][:0]
	for _, t := range s.failures[acc.ID] {
		if now.Sub(t) < time.Hour {
			recent = append(recent, t)
		}
	}
	s.failures[acc.ID] = recent
	if len(recent) >= 5 {
		fail(w, 429, "rate_limited", "Too many attempts. Try again in an hour.")
		return
	}
	id := codeID(in.Code)
	b, err := s.cfg.Store.Get(r.Context(), "voucher", id)
	var v voucher
	if err != nil || json.Unmarshal(b, &v) != nil || v.UsedBy != "" {
		s.failures[acc.ID] = append(s.failures[acc.ID], now)
		fail(w, 422, "voucher_invalid", "That code isn't valid or has already been used.")
		return
	}
	a := s.acct(acc.ID)
	parts := strings.Split(v.Grant, ":")
	switch {
	case parts[0] == "keys" && len(parts) == 2:
		n := 0
		for _, c := range parts[1] {
			n = n*10 + int(c-'0')
		}
		l := keys.Restore(a.Ledger)
		l.CreditOnce("voucher:"+id[:16], n, "voucher")
		a.Ledger = l.Entries()
	case parts[0] == "plan" && len(parts) == 3:
		days := 0
		for _, c := range parts[2] {
			days = days*10 + int(c-'0')
		}
		start := now
		if a.Voucher != nil && a.Voucher.Plan == parts[1] && a.Voucher.Until.After(now) {
			start = a.Voucher.Until
		}
		a.Voucher = &voucherPlan{Plan: parts[1], Until: start.Add(time.Duration(days) * 24 * time.Hour)}
	default:
		fail(w, 422, "voucher_invalid", "That code isn't valid or has already been used.")
		return
	}
	v.UsedBy, v.UsedAt = acc.ID, now
	vb, _ := json.Marshal(v)
	if err := s.cfg.Store.Put(r.Context(), "voucher", id, vb); err != nil {
		fail(w, 503, "unavailable", "Please try again.")
		return
	}
	s.dirty[acc.ID] = true
	e := s.entitlement(a)
	writeJSON(w, 200, map[string]any{"plan": e.Plan, "keys": e.Keys, "message": "Code applied."})
}

// handleIssueVouchers creates single-use codes (staff lead only). Codes are shown once and
// stored only as hashes.
func (s *Service) handleIssueVouchers(w http.ResponseWriter, r *http.Request) {
	if s.cfg.StaffLead == nil {
		fail(w, 401, "staff_required", "Staff sign-in required.")
		return
	}
	staff, ok := s.cfg.StaffLead(r)
	if !ok {
		fail(w, 401, "staff_required", "Staff sign-in required.")
		return
	}
	var in struct {
		Grant string `json:"grant"`
		Count int    `json:"count"`
	}
	_ = json.NewDecoder(io.LimitReader(r.Body, 4<<10)).Decode(&in)
	parts := strings.Split(in.Grant, ":")
	valid := (len(parts) == 2 && parts[0] == "keys" && isDigits(parts[1])) ||
		(len(parts) == 3 && parts[0] == "plan" && isDigits(parts[2]) && isPlan(parts[1]))
	if !valid || in.Count < 1 || in.Count > 500 {
		fail(w, 422, "validation_failed", "grant is keys:<n> or plan:<plan>:<days>; count 1–500.")
		return
	}
	codes := make([]string, 0, in.Count)
	for i := 0; i < in.Count; i++ {
		b := make([]byte, 10)
		_, _ = rand.Read(b)
		raw := voucherEnc.EncodeToString(b) // 16 chars
		code := raw[0:4] + "-" + raw[4:8] + "-" + raw[8:12] + "-" + raw[12:16]
		vb, _ := json.Marshal(voucher{Grant: in.Grant, Created: s.now(), By: staff})
		if err := s.cfg.Store.Put(r.Context(), "voucher", codeID(code), vb); err != nil {
			fail(w, 503, "unavailable", "Please try again.")
			return
		}
		codes = append(codes, code)
	}
	log.Printf("vouchers issued: %d x %s by %s", in.Count, in.Grant, staff)
	writeJSON(w, 201, map[string]any{"codes": codes})
}

func isDigits(s string) bool {
	if s == "" || len(s) > 4 {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return s != "0"
}

func isPlan(k string) bool {
	it, ok := item(k)
	return ok && it.Kind == "plan"
}

var errProviderNeeded = errors.New("a live subscription can't be stopped without the payment processor")
