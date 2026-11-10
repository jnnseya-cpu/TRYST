package growth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"html"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jnnseya-cpu/tryst/backend/internal/staffauth"
)

// Staff roles (from the staff roster, never from the request):
//
//	growth  imports data, refreshes scores, drafts briefs and changes
//	editor  approves briefs and changes, applies, rolls back, creates pages, freezes
//	lead    may read and freeze
const (
	RoleGrowth = "growth"
	RoleEditor = "editor"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, err error) {
	var e *Err
	if errors.As(err, &e) {
		body := map[string]any{"code": e.Code, "message": e.Msg}
		if len(e.Details) > 0 {
			body["details"] = e.Details
		}
		writeJSON(w, e.Status, body)
		return
	}
	log.Printf("growth: %v", err)
	writeJSON(w, 500, map[string]any{"code": "internal", "message": "Something went wrong."})
}

func (s *Service) staffWith(w http.ResponseWriter, r *http.Request, roles ...string) (staffauth.Staff, bool) {
	st, ok := s.staff.Authenticate(r)
	if !ok || !(st.Has(RoleGrowth) || st.Has(RoleEditor) || st.Has("lead")) {
		writeJSON(w, 401, map[string]any{"code": "staff_required", "message": "Staff sign-in required."})
		return st, false
	}
	if len(roles) == 0 {
		return st, true
	}
	for _, role := range roles {
		if st.Has(role) {
			return st, true
		}
	}
	writeJSON(w, 403, map[string]any{"code": "role_required", "message": "This needs the " + strings.Join(roles, " or ") + " role."})
	return st, false
}

func decode(r *http.Request, limit int64, v any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, limit))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return bad("The request body isn't valid JSON for this operation.")
	}
	return nil
}

// replay runs fn once per Idempotency-Key: an identical replay returns the stored
// response; the same key with a different body is a conflict.
func (s *Service) replay(w http.ResponseWriter, r *http.Request, op string, body []byte, fn func() (int, any, error)) {
	key := r.Header.Get("Idempotency-Key")
	if key == "" || len(key) > 128 {
		writeJSON(w, 428, map[string]any{"code": "idempotency_key_required", "message": "Send an Idempotency-Key header."})
		return
	}
	id, h := op+"|"+key, digest(string(body))
	s.mu.Lock()
	prev, seen := s.dedup[id]
	s.mu.Unlock()
	if seen {
		if prev.Hash != h {
			writeJSON(w, 409, map[string]any{"code": "idempotency_conflict", "message": "This key was used with a different request."})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(prev.Status)
		_, _ = w.Write(prev.Body)
		return
	}
	status, out, err := fn()
	if err != nil {
		writeErr(w, err)
		return
	}
	b, _ := json.Marshal(out)
	s.mu.Lock()
	s.dedup[id] = dedupEntry{Hash: h, Status: status, Body: b}
	_ = s.put(r.Context(), kDedup, id, s.dedup[id])
	s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write(b)
}

// Routes registers the console API (/ops/v1/growth/*, staff only) and the public guide
// pages (/guides/*). The console API is contract-tested from shared/contracts/openapi/ops.v1.yaml.
func (s *Service) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /ops/v1/growth/overview", s.hOverview)
	mux.HandleFunc("POST /ops/v1/growth/observations", s.hIngest)
	mux.HandleFunc("POST /ops/v1/growth/refresh", s.hRefresh)
	mux.HandleFunc("GET /ops/v1/growth/opportunities", s.hOpportunities)
	mux.HandleFunc("POST /ops/v1/growth/briefs", s.hCreateBrief)
	mux.HandleFunc("GET /ops/v1/growth/briefs/effective", s.hEffective)
	mux.HandleFunc("GET /ops/v1/growth/briefs/{id}", s.hBrief)
	mux.HandleFunc("POST /ops/v1/growth/briefs/{id}/approve", s.hApproveBrief)
	mux.HandleFunc("POST /ops/v1/growth/briefs/{id}/drafts", s.hDrafts)
	mux.HandleFunc("POST /ops/v1/growth/actions", s.hPropose)
	mux.HandleFunc("GET /ops/v1/growth/actions/{id}", s.hAction)
	mux.HandleFunc("POST /ops/v1/growth/actions/{id}/approve", s.hApproveAction)
	mux.HandleFunc("POST /ops/v1/growth/actions/{id}/execute", s.hExecute)
	mux.HandleFunc("POST /ops/v1/growth/actions/{id}/rollback", s.hRollback)
	mux.HandleFunc("GET /ops/v1/growth/actions/{id}/export", s.hExport)
	mux.HandleFunc("POST /ops/v1/growth/pages", s.hCreatePage)
	mux.HandleFunc("GET /ops/v1/growth/experiments/{id}", s.hExperiment)
	mux.HandleFunc("POST /ops/v1/growth/freeze", s.hFreeze)
	mux.HandleFunc("GET /ops/v1/growth/audit", s.hAudit)
}

func (s *Service) hOverview(w http.ResponseWriter, r *http.Request) {
	st, ok := s.staffWith(w, r)
	if !ok {
		return
	}
	s.mu.Lock()
	pages := []map[string]any{}
	for _, a := range s.prof.Assets {
		p := s.pages[a.ID]
		row := map[string]any{"asset_id": a.ID, "slug": a.Slug, "created": p != nil}
		if p != nil {
			row["active_revision"], row["fields"] = p.Active, p.current().Fields
		}
		pages = append(pages, row)
	}
	sources := []map[string]any{}
	now := s.now()
	for _, src := range s.prof.Sources {
		var last time.Time
		n := 0
		for _, o := range s.obs {
			if o.SourceID == src.ID {
				n++
				if o.WindowEnd.After(last) {
					last = o.WindowEnd
				}
			}
		}
		row := map[string]any{"source_id": src.ID, "family": src.Family, "access_path": src.AccessPath, "kind": src.Kind,
			"context_only": src.ContextOnly, "observations": n}
		if n > 0 {
			age := now.Sub(last).Hours()
			row["last_window_end"], row["age_hours"], row["stale"] = last.UTC().Format(time.RFC3339), age, age > src.StaleAfterHours
		}
		sources = append(sources, row)
	}
	briefs := []map[string]any{}
	for _, b := range s.briefs {
		briefs = append(briefs, map[string]any{"id": b.ID, "status": b.Status, "topic_id": b.str("topic_id"),
			"brief_id": b.str("brief_id"), "brief_version": b.version(), "expires_at": b.str("expires_at"), "created": b.Created})
	}
	resp := map[string]any{"frozen": s.state.Frozen, "mode": s.prof.Mode, "profile_revision": s.prof.Revision,
		"inference_route": s.prof.Inference.Route, "platform_connected": s.Signals != nil, "pages": pages,
		"sources": sources, "briefs": briefs, "roles": st.Roles,
		"claims": s.prof.Claims, "prohibited_claims": s.prof.ProhibitedClaims}
	s.mu.Unlock()
	writeJSON(w, 200, resp)
}

func (s *Service) hIngest(w http.ResponseWriter, r *http.Request) {
	st, ok := s.staffWith(w, r, RoleGrowth)
	if !ok {
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20+1))
	if err != nil || len(body) > 1<<20 {
		writeJSON(w, 413, map[string]any{"code": "too_large", "message": "Batches are limited to 1 MiB."})
		return
	}
	s.replay(w, r, "observations", body, func() (int, any, error) {
		var in struct {
			Items []Observation `json:"items"`
		}
		dec := json.NewDecoder(bytes.NewReader(body))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&in); err != nil {
			return 0, nil, bad("Send {\"items\": [...]} with the documented fields.")
		}
		res, err := s.Ingest(r.Context(), in.Items, st.ID)
		return 200, res, err
	})
}

func (s *Service) hRefresh(w http.ResponseWriter, r *http.Request) {
	st, ok := s.staffWith(w, r, RoleGrowth)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	res, err := s.Refresh(ctx, st.ID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 202, res)
}

func (s *Service) hOpportunities(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.staffWith(w, r); !ok {
		return
	}
	writeJSON(w, 200, map[string]any{"items": s.Opportunities(r.URL.Query().Get("status"))})
}

func (s *Service) hCreateBrief(w http.ResponseWriter, r *http.Request) {
	st, ok := s.staffWith(w, r, RoleGrowth)
	if !ok {
		return
	}
	var in struct {
		SnapshotID string `json:"snapshot_id"`
	}
	if err := decode(r, 4<<10, &in); err != nil {
		writeErr(w, err)
		return
	}
	b, err := s.CreateBrief(r.Context(), in.SnapshotID, st.ID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 201, b)
}

func (s *Service) hEffective(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.staffWith(w, r); !ok {
		return
	}
	q := r.URL.Query()
	b := s.Effective(q.Get("market"), q.Get("locale"), q.Get("topic"), q.Get("channel"))
	if b == nil {
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(204)
		return
	}
	writeJSON(w, 200, b)
}

func (s *Service) briefView(id string) (map[string]any, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b := s.briefs[id]
	if b == nil {
		return nil, false
	}
	actions := []*Action{}
	for _, a := range s.actions {
		if a.BriefID == b.str("brief_id") && a.BriefVersion == b.version() {
			actions = append(actions, a)
		}
	}
	pages := map[string]any{}
	if t := s.prof.Topic(b.str("topic_id")); t != nil {
		for _, id := range t.AssetIDs {
			if p := s.pages[id]; p != nil {
				pages[id] = map[string]any{"slug": p.Slug, "active_revision": p.Active, "fields": p.current().Fields}
			}
		}
	}
	claims := map[string]string{}
	for _, c := range b.strs("approved_claim_ids") {
		if cl := s.prof.Claim(c); cl != nil {
			claims[c] = cl.Text
		}
	}
	return map[string]any{"brief": b, "actions": actions, "pages": pages, "claims": claims, "snapshot": s.snaps[b.SnapshotID]}, true
}

func (s *Service) hBrief(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.staffWith(w, r); !ok {
		return
	}
	v, found := s.briefView(r.PathValue("id"))
	if !found {
		writeErr(w, notFound())
		return
	}
	writeJSON(w, 200, v)
}

func (s *Service) hApproveBrief(w http.ResponseWriter, r *http.Request) {
	st, ok := s.staffWith(w, r, RoleEditor)
	if !ok {
		return
	}
	b, err := s.ApproveBrief(r.Context(), r.PathValue("id"), r.Header.Get("If-Match"), st.ID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, b)
}

func (s *Service) hDrafts(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.staffWith(w, r, RoleGrowth); !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	out, err := s.Drafts(ctx, r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 201, map[string]any{"actions": out})
}

func (s *Service) hPropose(w http.ResponseWriter, r *http.Request) {
	st, ok := s.staffWith(w, r, RoleGrowth, RoleEditor)
	if !ok {
		return
	}
	body, _ := io.ReadAll(io.LimitReader(r.Body, 64<<10))
	s.replay(w, r, "actions", body, func() (int, any, error) {
		var p Proposal
		dec := json.NewDecoder(bytes.NewReader(body))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&p); err != nil {
			return 0, nil, bad("The change isn't valid JSON with the documented fields.")
		}
		a, created, err := s.Propose(r.Context(), p, st.ID, nil)
		if !created {
			return 200, a, err
		}
		return 201, a, err
	})
}

func (s *Service) action(id string) *Action {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.actions[id]
}

func (s *Service) hAction(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.staffWith(w, r); !ok {
		return
	}
	a := s.action(r.PathValue("id"))
	if a == nil {
		writeErr(w, notFound())
		return
	}
	writeJSON(w, 200, a)
}

func (s *Service) hApproveAction(w http.ResponseWriter, r *http.Request) {
	st, ok := s.staffWith(w, r, RoleEditor)
	if !ok {
		return
	}
	var in struct {
		PayloadHash    string `json:"payload_hash"`
		TargetRevision int    `json:"target_revision"`
	}
	if err := decode(r, 4<<10, &in); err != nil {
		writeErr(w, err)
		return
	}
	a, err := s.ApproveAction(r.Context(), r.PathValue("id"), in.PayloadHash, in.TargetRevision, st.ID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, a)
}

func (s *Service) hExecute(w http.ResponseWriter, r *http.Request) {
	st, ok := s.staffWith(w, r, RoleEditor)
	if !ok {
		return
	}
	a, err := s.Queue(r.Context(), r.PathValue("id"), st.ID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 202, a)
}

func (s *Service) hRollback(w http.ResponseWriter, r *http.Request) {
	st, ok := s.staffWith(w, r, RoleEditor)
	if !ok {
		return
	}
	a, err := s.Rollback(r.Context(), r.PathValue("id"), st.ID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 202, a)
}

func (s *Service) hExport(w http.ResponseWriter, r *http.Request) {
	st, ok := s.staffWith(w, r)
	if !ok {
		return
	}
	a := s.action(r.PathValue("id"))
	if a == nil {
		writeErr(w, notFound())
		return
	}
	if a.Live {
		writeErr(w, conflict("invalid_state", "Live changes are applied, not exported."))
		return
	}
	s.mu.Lock()
	s.audit(r.Context(), st.ID, "action.exported", "action:"+a.ID, nil)
	s.mu.Unlock()
	writeJSON(w, 200, map[string]any{"action_id": a.ID, "channel": a.Channel, "copy": a.Patch,
		"brief": a.BriefID + ":" + strconv.Itoa(a.BriefVersion)})
}

func (s *Service) hCreatePage(w http.ResponseWriter, r *http.Request) {
	st, ok := s.staffWith(w, r, RoleEditor)
	if !ok {
		return
	}
	var in struct {
		AssetID string `json:"asset_id"`
	}
	if err := decode(r, 4<<10, &in); err != nil {
		writeErr(w, err)
		return
	}
	p, err := s.CreatePage(r.Context(), in.AssetID, st.ID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 201, p)
}

func (s *Service) hExperiment(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.staffWith(w, r); !ok {
		return
	}
	rep, err := s.Report(r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, rep)
}

func (s *Service) hFreeze(w http.ResponseWriter, r *http.Request) {
	st, ok := s.staffWith(w, r, RoleEditor, "lead")
	if !ok {
		return
	}
	var in struct {
		Frozen bool `json:"frozen"`
	}
	if err := decode(r, 1<<10, &in); err != nil {
		writeErr(w, err)
		return
	}
	if err := s.SetFrozen(r.Context(), in.Frozen, st.ID); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"frozen": in.Frozen})
}

func (s *Service) hAudit(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.staffWith(w, r); !ok {
		return
	}
	s.mu.Lock()
	n := len(s.audits)
	out := make([]auditEntry, 0, min(n, 500))
	for i := n - 1; i >= 0 && len(out) < 500; i-- {
		out = append(out, s.audits[i])
	}
	s.mu.Unlock()
	writeJSON(w, 200, map[string]any{"items": out})
}

// ---------------------------------------------------------------- public guide pages

// PublicRoutes serves the guide pages: plain HTML, no script, no cookie, no tracker, no
// mention of how the wording was chosen. Views and join clicks are counted per page
// revision and day only.
func (s *Service) PublicRoutes(mux *http.ServeMux, siteURL, appURL string) {
	mux.HandleFunc("GET /guides/{slug}", func(w http.ResponseWriter, r *http.Request) {
		s.servePage(w, r, siteURL)
	})
	mux.HandleFunc("POST /guides/{slug}/join", func(w http.ResponseWriter, r *http.Request) {
		asset, page := s.published(r.PathValue("slug"))
		if page == nil {
			http.NotFound(w, r)
			return
		}
		s.Count(r.Context(), asset.ID, page.Active, "join")
		target := strings.TrimRight(appURL, "/") + "/join"
		if t := s.prof.Topic(topicOf(s.prof, asset.ID)); t != nil {
			if c := s.prof.CTA(t.CTAID); c != nil && strings.HasPrefix(c.URL, "/") {
				target = strings.TrimRight(appURL, "/") + c.URL
			}
		}
		w.Header().Set("Cache-Control", "no-store")
		http.Redirect(w, r, target, http.StatusSeeOther)
	})
	mux.HandleFunc("GET /guides/sitemap.xml", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		var b strings.Builder
		b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n" + `<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")
		for _, a := range s.prof.Assets {
			if p := s.pages[a.ID]; p != nil {
				b.WriteString("  <url><loc>" + html.EscapeString(strings.TrimRight(siteURL, "/")+"/guides/"+a.Slug) + "</loc></url>\n")
			}
		}
		s.mu.Unlock()
		b.WriteString("</urlset>\n")
		w.Header().Set("Content-Type", "application/xml")
		w.Header().Set("Cache-Control", "public, max-age=300")
		_, _ = w.Write([]byte(b.String()))
	})
}

func topicOf(p *Profile, assetID string) string {
	for _, t := range p.Topics {
		for _, a := range t.AssetIDs {
			if a == assetID {
				return t.ID
			}
		}
	}
	return ""
}

func (s *Service) published(slug string) (*Asset, *Page) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a := s.prof.AssetBySlug(slug)
	if a == nil {
		return nil, nil
	}
	p := s.pages[a.ID]
	if p == nil {
		return a, nil
	}
	cp := *p
	return a, &cp
}

const pageCSS = `body{margin:0;font:17px/1.6 system-ui,-apple-system,Segoe UI,Roboto,sans-serif;color:#1d1b20;background:#faf8f5}` +
	`main{max-width:40rem;margin:0 auto;padding:3rem 1rem}h1{font-size:2rem;line-height:1.2;margin:0 0 1rem}` +
	`p{margin:0 0 1.5rem}button{font:inherit;padding:.75rem 1.25rem;border:0;border-radius:.5rem;background:#5b2a86;color:#fff;cursor:pointer}` +
	`a{color:#5b2a86}@media(prefers-color-scheme:dark){body{background:#151317;color:#eee}a{color:#c9a7ef}}`

func (s *Service) servePage(w http.ResponseWriter, r *http.Request, siteURL string) {
	asset, page := s.published(r.PathValue("slug"))
	if page == nil {
		http.NotFound(w, r)
		return
	}
	f := page.current().Fields
	e := html.EscapeString
	canonical := strings.TrimRight(siteURL, "/") + "/guides/" + asset.Slug
	body := `<!doctype html><html lang="en-GB"><head><meta charset="utf-8">` +
		`<meta name="viewport" content="width=device-width,initial-scale=1">` +
		`<title>` + e(f["title"]) + `</title><meta name="description" content="` + e(f["description"]) + `">` +
		`<link rel="canonical" href="` + e(canonical) + `"><style>` + pageCSS + `</style></head><body><main>` +
		`<p><a href="` + e(strings.TrimRight(siteURL, "/")+"/") + `">TRYST</a></p>` +
		`<h1>` + e(f["headline"]) + `</h1><p>` + e(f["description"]) + `</p>` +
		`<form method="post" action="/guides/` + e(asset.Slug) + `/join"><button type="submit">` + e(f["cta_text"]) + `</button></form>` +
		`</main></body></html>`
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; form-action 'self'; frame-ancestors 'none'; base-uri 'none'")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Cache-Control", "public, max-age=60")
	if r.Method == http.MethodGet {
		s.Count(r.Context(), asset.ID, page.Active, "view")
	}
	_, _ = w.Write([]byte(body))
}
