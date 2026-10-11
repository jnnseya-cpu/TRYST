package commercesvc

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jnnseya-cpu/tryst/backend/internal/gate"
	"github.com/jnnseya-cpu/tryst/backend/internal/identity"
	"github.com/jnnseya-cpu/tryst/backend/internal/payments"
	"github.com/jnnseya-cpu/tryst/backend/internal/store"
)

type members map[string]gate.Tier

func (m members) Authenticate(r *http.Request) (identity.Account, bool) {
	id := r.Header.Get("X-Test-Member")
	t, ok := m[id]
	return identity.Account{ID: id, Tier: t}, ok
}

const whsec = "whsec_test_secret"

type stripeCall struct {
	Method, Path string
	Form         url.Values
}

type rig struct {
	t      *testing.T
	svc    *Service
	now    time.Time
	mu     sync.Mutex
	calls  []stripeCall
	stripe *payments.Stripe
}

func newRig(t *testing.T, st store.Store) *rig {
	t.Helper()
	g := &rig{t: t, now: time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		f, _ := url.ParseQuery(string(b))
		g.mu.Lock()
		g.calls = append(g.calls, stripeCall{r.Method, r.URL.Path, f})
		g.mu.Unlock()
		if r.URL.Path == "/v1/checkout/sessions" {
			io.WriteString(w, `{"id":"cs_1","url":"https://checkout.stripe.com/c/pay/cs_1"}`)
			return
		}
		io.WriteString(w, `{}`)
	}))
	t.Cleanup(srv.Close)
	g.stripe = &payments.Stripe{BaseURL: srv.URL, SecretKey: "sk_test_1", WebhookSecret: whsec,
		Prices: map[string]payments.StripePrice{
			"tryst_plus": {PriceID: "price_plus", Subscription: true}, "ghost": {PriceID: "price_ghost", Subscription: true},
			"keys_60": {PriceID: "price_keys"}, "deposit": {PriceID: "price_deposit"},
		}, SuccessURL: "https://app.example/billing?done=1", CancelURL: "https://app.example/billing"}
	svc, err := New(Config{
		Members: members{"ana": gate.V2, "ben": gate.V2, "newbie": gate.V0}, Provider: g.stripe, Store: st, Env: "prod", MinTier: gate.V1,
		StaffLead: func(r *http.Request) (string, bool) { return "lead1", r.Header.Get("X-Test-Staff") == "lead" },
	})
	if err != nil {
		t.Fatal(err)
	}
	g.stripe.SubscriptionFor = svc.SubscriptionFor
	svc.now = func() time.Time { return g.now }
	g.svc = svc
	return g
}

func (g *rig) do(method, path, who string, body any) (int, map[string]any) {
	g.t.Helper()
	var rd io.Reader = bytes.NewReader(nil)
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	r := httptest.NewRequest(method, path, rd)
	if who == "lead" {
		r.Header.Set("X-Test-Staff", "lead")
	} else if who != "" {
		r.Header.Set("X-Test-Member", who)
	}
	w := httptest.NewRecorder()
	g.svc.ServeHTTP(w, r)
	out := map[string]any{}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

// send delivers a signed webhook event.
func (g *rig) send(id, typ string, obj map[string]any) (int, map[string]any) {
	g.t.Helper()
	raw, _ := json.Marshal(map[string]any{"id": id, "type": typ, "data": map[string]any{"object": obj}})
	ts := fmt.Sprint(g.now.Unix())
	mac := hmac.New(sha256.New, []byte(whsec))
	mac.Write([]byte(ts + "." + string(raw)))
	r := httptest.NewRequest("POST", "/v1/billing/stripe/webhook", bytes.NewReader(raw))
	r.Header.Set("Stripe-Signature", "t="+ts+",v1="+hex.EncodeToString(mac.Sum(nil)))
	w := httptest.NewRecorder()
	g.svc.ServeHTTP(w, r)
	out := map[string]any{}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func (g *rig) ref(who string) string {
	g.svc.mu.Lock()
	defer g.svc.mu.Unlock()
	return g.svc.acct(who).BillingRef
}

func (g *rig) ent(who string) map[string]any {
	_, out := g.do("GET", "/v1/entitlements", who, nil)
	return out
}

func (g *rig) last() stripeCall {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.calls[len(g.calls)-1]
}

func TestSubscribeWebhookAndCoolingOffCancel(t *testing.T) {
	g := newRig(t, store.NewMemory())
	if c, _ := g.do("POST", "/v1/subscription", "ana", map[string]any{"plan": "tryst_plus"}); c != 422 {
		t.Fatalf("no pre-contract ack: %d", c)
	}
	if c, out := g.do("POST", "/v1/subscription", "ana", map[string]any{"plan": "tryst_plus", "pre_contract_ack": true}); c != 422 || out["code"] != "start_now_required" {
		t.Fatalf("no start-now request: %d %v", c, out)
	}
	c, out := g.do("POST", "/v1/subscription", "ana", map[string]any{"plan": "tryst_plus", "pre_contract_ack": true, "start_now": true})
	if c != 201 || !strings.HasPrefix(out["checkout_url"].(string), "https://checkout.stripe.com/") {
		t.Fatalf("subscribe: %d %v", c, out)
	}
	ref := g.ref("ana")
	if f := g.last().Form; f.Get("client_reference_id") != ref || f.Get("metadata[item]") != "tryst_plus" {
		t.Fatalf("checkout form: %v", f)
	}
	if g.ent("ana")["plan"] != "" {
		t.Fatal("plan active before payment")
	}

	// Unsigned or wrongly signed events change nothing.
	r := httptest.NewRequest("POST", "/v1/billing/stripe/webhook", strings.NewReader(`{"id":"evt_x","type":"checkout.session.completed"}`))
	r.Header.Set("Stripe-Signature", "t=1,v1=00")
	w := httptest.NewRecorder()
	g.svc.ServeHTTP(w, r)
	if w.Code != 400 {
		t.Fatalf("bad signature: %d", w.Code)
	}

	session := map[string]any{"mode": "subscription", "payment_status": "paid", "client_reference_id": ref,
		"metadata": map[string]string{"billing_ref": ref, "item": "tryst_plus"}, "subscription": "sub_1", "amount_total": 2499}
	if c, out := g.send("evt_1", "checkout.session.completed", session); c != 200 || out["duplicate"] != false {
		t.Fatalf("webhook: %d %v", c, out)
	}
	if c, out := g.send("evt_1", "checkout.session.completed", session); c != 200 || out["duplicate"] != true {
		t.Fatalf("replay: %d %v", c, out)
	}
	periodEnd := g.now.AddDate(0, 1, 0)
	g.send("evt_2", "invoice.paid", map[string]any{"subscription": "sub_1", "payment_intent": "pi_sub_1", "amount_paid": 2499,
		"lines": map[string]any{"data": []any{map[string]any{"period": map[string]any{"end": periodEnd.Unix()}}}}})
	e := g.ent("ana")
	if e["plan"] != "tryst_plus" {
		t.Fatalf("plan after payment: %v", e)
	}
	if c, out := g.do("POST", "/v1/subscription", "ana", map[string]any{"plan": "envoy", "pre_contract_ack": true, "start_now": true}); c != 409 || out["code"] != "change_plan" {
		t.Fatalf("second plan: %d %v", c, out)
	}

	// Day 3 of 30: cancel ends now and refunds 27 days.
	g.now = g.now.Add(2*24*time.Hour + time.Hour)
	c, out = g.do("POST", "/v1/subscription/cancel", "ana", nil)
	if c != 200 {
		t.Fatalf("cancel: %d %v", c, out)
	}
	want := int64(2499 - (2499*3+15)/30) // 2499 - round(2499*3/30)
	if int64(out["refund_minor"].(float64)) != want {
		t.Fatalf("refund = %v, want %d", out["refund_minor"], want)
	}
	g.mu.Lock()
	calls := g.calls
	g.mu.Unlock()
	del, refund := calls[len(calls)-2], calls[len(calls)-1]
	if del.Method != "DELETE" || del.Path != "/v1/subscriptions/sub_1" {
		t.Fatalf("cancel call: %+v", del)
	}
	if refund.Path != "/v1/refunds" || refund.Form.Get("payment_intent") != "pi_sub_1" || refund.Form.Get("amount") != fmt.Sprint(want) {
		t.Fatalf("refund call: %+v", refund)
	}
	if g.ent("ana")["plan"] != "" {
		t.Fatal("plan still active after cooling-off cancel")
	}
	// Stripe's later update doesn't resurrect it.
	g.send("evt_3", "customer.subscription.updated", map[string]any{"id": "sub_1", "status": "active", "current_period_end": periodEnd.Unix()})
	if g.ent("ana")["plan"] != "" {
		t.Fatal("update resurrected a cancelled plan")
	}
}

func TestCancelAfterCoolingOffRunsToPeriodEnd(t *testing.T) {
	g := newRig(t, store.NewMemory())
	g.do("POST", "/v1/subscription", "ben", map[string]any{"plan": "tryst_plus", "pre_contract_ack": true, "start_now": true})
	ref := g.ref("ben")
	// Out of order: the invoice arrives before the checkout event.
	end := g.now.AddDate(0, 1, 0)
	g.send("evt_a", "invoice.paid", map[string]any{"subscription": "sub_b", "payment_intent": "pi_b", "amount_paid": 2499,
		"subscription_details": map[string]any{"metadata": map[string]string{"billing_ref": ref, "item": "tryst_plus"}},
		"lines":                map[string]any{"data": []any{map[string]any{"period": map[string]any{"end": end.Unix()}}}}})
	g.send("evt_b", "checkout.session.completed", map[string]any{"mode": "subscription", "client_reference_id": ref,
		"metadata": map[string]string{"billing_ref": ref, "item": "tryst_plus"}, "subscription": "sub_b", "amount_total": 2499})
	if g.ent("ben")["plan"] != "tryst_plus" {
		t.Fatal("plan not active")
	}
	g.now = g.now.Add(20 * 24 * time.Hour)
	c, out := g.do("POST", "/v1/subscription/cancel", "ben", nil)
	if c != 200 || out["refund_minor"].(float64) != 0 {
		t.Fatalf("cancel: %d %v", c, out)
	}
	if l := g.last(); l.Path != "/v1/subscriptions/sub_b" || l.Form.Get("cancel_at_period_end") != "true" {
		t.Fatalf("cancel call: %+v", l)
	}
	if e := g.ent("ben"); e["plan"] != "tryst_plus" || !e["subscriptions"].([]any)[0].(map[string]any)["cancel_at_period_end"].(bool) {
		t.Fatalf("plan should run to period end: %v", e)
	}
	if c, _ := g.do("POST", "/v1/subscription/cancel", "ben", nil); c != 404 {
		t.Fatalf("second cancel: %d", c)
	}
	g.send("evt_c", "customer.subscription.deleted", map[string]any{"id": "sub_b", "status": "canceled"})
	if g.ent("ben")["plan"] != "" {
		t.Fatal("plan active after deletion")
	}
}

func TestKeysDepositRefundsAndDisputes(t *testing.T) {
	g := newRig(t, store.NewMemory())
	if c, _ := g.do("POST", "/v1/keys/purchase", "ana", map[string]any{"pack": "tryst_plus"}); c != 422 {
		t.Fatalf("bad pack: %d", c)
	}
	if c, _ := g.do("POST", "/v1/keys/purchase", "ana", map[string]any{"pack": "keys_60"}); c != 201 {
		t.Fatalf("keys checkout: %d", c)
	}
	ref := g.ref("ana")
	pay := func(evt, item, pi string) {
		g.send(evt, "checkout.session.completed", map[string]any{"mode": "payment", "payment_status": "paid", "client_reference_id": ref,
			"metadata": map[string]string{"billing_ref": ref, "item": item}, "payment_intent": pi, "amount_total": 999})
	}
	pay("evt_k1", "keys_60", "pi_k1")
	pay("evt_k1b", "keys_60", "pi_k1") // a second event for the same payment
	if k := g.ent("ana")["keys"].(float64); k != 60 {
		t.Fatalf("keys = %v", k)
	}
	g.do("POST", "/v1/deposit", "ana", nil)
	pay("evt_d1", "deposit", "pi_d1")
	e := g.ent("ana")
	if e["keys"].(float64) != 120 || e["deposit_paid"] != true {
		t.Fatalf("after deposit: %v", e)
	}
	if c, _ := g.do("POST", "/v1/deposit", "ana", nil); c != 409 {
		t.Fatalf("second deposit: %d", c)
	}
	// Half refund of the Keys pack removes half its Keys.
	g.send("evt_r1", "charge.refunded", map[string]any{"payment_intent": "pi_k1", "amount": 999, "amount_refunded": 500})
	if k := g.ent("ana")["keys"].(float64); k != 90 {
		t.Fatalf("after partial refund: %v", k)
	}
	// A chargeback on the deposit removes its Keys and the deposit.
	g.send("evt_x1", "charge.dispute.created", map[string]any{"payment_intent": "pi_d1"})
	if e := g.ent("ana"); e["keys"].(float64) != 30 || e["deposit_paid"] != false {
		t.Fatalf("after dispute: %v", e)
	}
	// A payment that isn't settled yet credits nothing.
	g.send("evt_p", "checkout.session.completed", map[string]any{"mode": "payment", "payment_status": "unpaid", "client_reference_id": ref,
		"metadata": map[string]string{"billing_ref": ref, "item": "keys_60"}, "payment_intent": "pi_p"})
	if k := g.ent("ana")["keys"].(float64); k != 30 {
		t.Fatalf("unpaid credited: %v", k)
	}
}

func TestGatesAndUnavailableProvider(t *testing.T) {
	g := newRig(t, store.NewMemory())
	if c, out := g.do("POST", "/v1/deposit", "newbie", nil); c != 403 || out["code"] != "tier_required" {
		t.Fatalf("V0 paying: %d %v", c, out)
	}
	if c, _ := g.do("GET", "/v1/entitlements", "", nil); c != 401 {
		t.Fatalf("signed out: %d", c)
	}
	svc, _ := New(Config{Members: members{"ana": gate.V2}, Store: store.NewMemory(), Env: "prod"})
	r := httptest.NewRequest("POST", "/v1/deposit", nil)
	r.Header.Set("X-Test-Member", "ana")
	w := httptest.NewRecorder()
	svc.ServeHTTP(w, r)
	if w.Code != 503 {
		t.Fatalf("no provider: %d", w.Code)
	}
	if _, err := New(Config{Members: members{}, Provider: NewDevProvider(), Store: store.NewMemory(), Env: "prod"}); err == nil {
		t.Fatal("dev provider accepted in prod")
	}
}

func TestVouchers(t *testing.T) {
	g := newRig(t, store.NewMemory())
	if c, _ := g.do("POST", "/ops/v1/vouchers", "ana", map[string]any{"grant": "keys:60", "count": 1}); c != 401 {
		t.Fatalf("member issuing vouchers: %d", c)
	}
	if c, _ := g.do("POST", "/ops/v1/vouchers", "lead", map[string]any{"grant": "plan:ghost:30", "count": 1}); c != 422 {
		t.Fatalf("add-on as plan voucher: %d", c)
	}
	_, out := g.do("POST", "/ops/v1/vouchers", "lead", map[string]any{"grant": "keys:60", "count": 2})
	codes := out["codes"].([]any)
	_, out = g.do("POST", "/ops/v1/vouchers", "lead", map[string]any{"grant": "plan:tryst_plus:30", "count": 1})
	plan := out["codes"].([]any)[0].(string)

	if c, out := g.do("POST", "/v1/vouchers/redeem", "ana", map[string]any{"code": strings.ToLower(codes[0].(string))}); c != 200 || out["keys"].(float64) != 60 {
		t.Fatalf("redeem keys: %d %v", c, out)
	}
	if c, _ := g.do("POST", "/v1/vouchers/redeem", "ben", map[string]any{"code": codes[0]}); c != 422 {
		t.Fatalf("reuse: %d", c)
	}
	if c, out := g.do("POST", "/v1/vouchers/redeem", "ben", map[string]any{"code": plan}); c != 200 || out["plan"] != "tryst_plus" {
		t.Fatalf("redeem plan: %d %v", c, out)
	}
	g.now = g.now.Add(31 * 24 * time.Hour)
	if g.ent("ben")["plan"] != "" {
		t.Fatal("voucher plan didn't expire")
	}
	for i := 0; i < 5; i++ {
		g.do("POST", "/v1/vouchers/redeem", "ben", map[string]any{"code": "WRONG-CODE"})
	}
	if c, _ := g.do("POST", "/v1/vouchers/redeem", "ben", map[string]any{"code": codes[1]}); c != 429 {
		t.Fatalf("rate limit: %d", c)
	}
}

func TestDevCheckout(t *testing.T) {
	dp := NewDevProvider()
	svc, err := New(Config{Members: members{"ana": gate.V0}, Provider: dp, Store: store.NewMemory(), Env: "dev"})
	if err != nil {
		t.Fatal(err)
	}
	call := func(method, path string, body any) map[string]any {
		b, _ := json.Marshal(body)
		r := httptest.NewRequest(method, path, bytes.NewReader(b))
		r.Header.Set("X-Test-Member", "ana")
		w := httptest.NewRecorder()
		svc.ServeHTTP(w, r)
		out := map[string]any{}
		_ = json.Unmarshal(w.Body.Bytes(), &out)
		return out
	}
	u := call("POST", "/v1/subscription", map[string]any{"plan": "ghost", "pre_contract_ack": true, "start_now": true})["checkout_url"].(string)
	sess := strings.TrimPrefix(u, "/billing/checkout?session=")
	call("POST", "/v1/billing/dev/complete", map[string]any{"session": sess})
	if e := call("GET", "/v1/entitlements", nil); e["ghost"] != true {
		t.Fatalf("ghost not active: %v", e)
	}
	if out := call("POST", "/v1/billing/dev/complete", map[string]any{"session": sess}); out["code"] != "not_found" {
		t.Fatal("dev session reusable")
	}
	call("POST", "/v1/subscription/cancel", map[string]any{"item": "ghost"})
	if len(dp.Calls) != 2 || !strings.HasPrefix(dp.Calls[0], "cancel_now") || !strings.HasPrefix(dp.Calls[1], "refund") {
		t.Fatalf("dev calls: %v", dp.Calls)
	}
}

type prefixed struct {
	store.Store
	ns string
}

func (p prefixed) Put(ctx context.Context, k, id string, b []byte) error {
	return p.Store.Put(ctx, p.ns+k, id, b)
}
func (p prefixed) Get(ctx context.Context, k, id string) ([]byte, error) {
	return p.Store.Get(ctx, p.ns+k, id)
}
func (p prefixed) All(ctx context.Context, k string) (map[string][]byte, error) {
	return p.Store.All(ctx, p.ns+k)
}

func restart(t *testing.T, st store.Store) {
	g := newRig(t, st)
	g.do("POST", "/v1/subscription", "ana", map[string]any{"plan": "tryst_plus", "pre_contract_ack": true, "start_now": true})
	ref := g.ref("ana")
	g.send("evt_s", "checkout.session.completed", map[string]any{"mode": "subscription", "client_reference_id": ref,
		"metadata": map[string]string{"billing_ref": ref, "item": "tryst_plus"}, "subscription": "sub_r", "amount_total": 2499})
	g.send("evt_k", "checkout.session.completed", map[string]any{"mode": "payment", "payment_status": "paid", "client_reference_id": ref,
		"metadata": map[string]string{"billing_ref": ref, "item": "keys_60"}, "payment_intent": "pi_r", "amount_total": 999})

	g2 := newRig(t, st)
	g2.now = g.now
	if e := g2.ent("ana"); e["plan"] != "tryst_plus" || e["keys"].(float64) != 60 {
		t.Fatalf("after restart: %v", e)
	}
	if c, out := g2.send("evt_k", "checkout.session.completed", map[string]any{}); c != 200 || out["duplicate"] != true {
		t.Fatalf("event replay after restart: %d %v", c, out)
	}
	if id, err := g2.svc.SubscriptionFor(ref + "|tryst_plus"); err != nil || id != "sub_r" {
		t.Fatalf("subscription mapping after restart: %q %v", id, err)
	}
}

func TestRestartMemory(t *testing.T) { restart(t, store.NewMemory()) }

func TestRestartPostgres(t *testing.T) {
	dsn := os.Getenv("TRYST_TEST_PG")
	if dsn == "" {
		t.Skip("TRYST_TEST_PG not set")
	}
	p, err := store.OpenPostgres(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	ns := make([]byte, 6)
	_, _ = rand.Read(ns)
	restart(t, prefixed{p, "t" + hex.EncodeToString(ns) + ":"})
}

func TestExportAndErase(t *testing.T) {
	g := newRig(t, store.NewMemory())
	ctx := context.Background()
	g.do("POST", "/v1/subscription", "ana", map[string]any{"plan": "tryst_plus", "pre_contract_ack": true, "start_now": true})
	ref := g.ref("ana")
	g.send("evt_s", "checkout.session.completed", map[string]any{"mode": "subscription", "client_reference_id": ref,
		"metadata": map[string]string{"billing_ref": ref, "item": "tryst_plus"}, "subscription": "sub_e", "payment_intent": "pi_sub", "amount_total": 2499})
	g.send("evt_d", "checkout.session.completed", map[string]any{"mode": "payment", "payment_status": "paid", "client_reference_id": ref,
		"metadata": map[string]string{"billing_ref": ref, "item": "deposit"}, "payment_intent": "pi_dep", "amount_total": 999})

	ex, _ := g.svc.Export(ctx, "ana")
	if m := ex.(map[string]any); m["keys"] != 60 || m["deposit_paid"] != true {
		t.Fatalf("export: %v", m)
	}
	g.now = g.now.Add(24 * time.Hour)
	rc, err := g.svc.Erase(ctx, "ana")
	if err != nil {
		t.Fatal(err)
	}
	g.mu.Lock()
	calls := append([]stripeCall(nil), g.calls...)
	g.mu.Unlock()
	var cancelled, subRefund, depRefund bool
	for _, c := range calls {
		switch {
		case c.Method == "DELETE" && c.Path == "/v1/subscriptions/sub_e":
			cancelled = true
		case c.Path == "/v1/refunds" && c.Form.Get("payment_intent") == "pi_sub":
			subRefund = true
		case c.Path == "/v1/refunds" && c.Form.Get("payment_intent") == "pi_dep" && c.Form.Get("amount") == "999":
			depRefund = true
		}
	}
	if !cancelled || !subRefund || !depRefund {
		t.Fatalf("cancel=%v subRefund=%v depositRefund=%v", cancelled, subRefund, depRefund)
	}
	if len(rc.Retained) != 1 {
		t.Fatalf("receipt: %+v", rc)
	}
	if _, err := g.svc.cfg.Store.Get(ctx, "account", "ana"); err == nil {
		t.Fatal("commerce record left")
	}
	if _, err := g.svc.SubscriptionFor(ref + "|tryst_plus"); err == nil {
		t.Fatal("subscription mapping left")
	}
	if _, err := g.svc.Erase(ctx, "ana"); err != nil {
		t.Fatal("erase not idempotent")
	}
}

func TestBillingNotices(t *testing.T) {
	g := newRig(t, store.NewMemory())
	var got []string
	g.svc.cfg.Notify = func(acc, kind, _ string) { got = append(got, acc+":"+kind) }
	ref := g.ref("ana")
	md := map[string]string{"billing_ref": ref, "item": "tryst_plus"}
	g.send("evt_1", "checkout.session.completed", map[string]any{"mode": "subscription", "client_reference_id": ref, "metadata": md, "subscription": "sub_n", "amount_total": 2499})
	g.send("evt_2", "invoice.upcoming", map[string]any{"subscription": "sub_n"})
	g.send("evt_3", "customer.subscription.deleted", map[string]any{"id": "sub_n", "status": "canceled"})
	g.send("evt_3b", "customer.subscription.deleted", map[string]any{"id": "sub_n", "status": "canceled"})
	if strings.Join(got, ",") != "ana:payment,ana:renewal,ana:plan_ended" {
		t.Fatalf("notices: %v", got)
	}
}
