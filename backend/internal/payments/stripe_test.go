package payments

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

type stripeCall struct {
	Method, Path, Idem, Auth string
	Form                     url.Values
}

func fakeStripe(t *testing.T) (*httptest.Server, *[]stripeCall) {
	var mu sync.Mutex
	calls := &[]stripeCall{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		f, _ := url.ParseQuery(string(b))
		mu.Lock()
		*calls = append(*calls, stripeCall{r.Method, r.URL.Path, r.Header.Get("Idempotency-Key"), r.Header.Get("Authorization"), f})
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/v1/checkout/sessions" && f.Get("line_items[0][price]") == "price_bad":
			w.WriteHeader(400)
			io.WriteString(w, `{"error":{"type":"invalid_request_error","code":"resource_missing"}}`)
		case r.URL.Path == "/v1/checkout/sessions":
			io.WriteString(w, `{"id":"cs_test_1","url":"https://checkout.stripe.com/c/pay/cs_test_1"}`)
		default:
			io.WriteString(w, `{"id":"ok"}`)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, calls
}

func testStripe(base string) *Stripe {
	return &Stripe{
		BaseURL: base, SecretKey: "sk_test_123", WebhookSecret: "whsec_x",
		Prices: map[string]StripePrice{
			"keys_60":    {PriceID: "price_keys"},
			"tryst_plus": {PriceID: "price_plus", Subscription: true},
			"broken":     {PriceID: "price_bad"},
		},
		SuccessURL: "https://app.example/billing?done=1", CancelURL: "https://app.example/billing",
		SubscriptionFor: func(ref string) (string, error) {
			if ref == "br_1" {
				return "sub_1", nil
			}
			return "", errors.New("none")
		},
		now: func() time.Time { return time.Unix(1_800_000_000, 0) },
	}
}

func TestStripeCheckoutSendsOnlyTheBillingRef(t *testing.T) {
	srv, calls := fakeStripe(t)
	s := testStripe(srv.URL)
	u, err := s.CreateCheckout(context.Background(), "br_1", "tryst_plus")
	if err != nil || !strings.HasPrefix(u, "https://checkout.stripe.com/") {
		t.Fatalf("checkout: %q %v", u, err)
	}
	c := (*calls)[0]
	if c.Method != "POST" || c.Auth != "Bearer sk_test_123" || c.Idem == "" {
		t.Fatalf("call: %+v", c)
	}
	want := map[string]string{
		"mode": "subscription", "line_items[0][price]": "price_plus", "client_reference_id": "br_1",
		"metadata[billing_ref]": "br_1", "subscription_data[metadata][billing_ref]": "br_1",
	}
	for k, v := range want {
		if c.Form.Get(k) != v {
			t.Errorf("%s = %q, want %q", k, c.Form.Get(k), v)
		}
	}
	for k := range c.Form {
		if strings.Contains(k, "email") || strings.Contains(k, "name") || strings.Contains(k, "customer") {
			t.Errorf("member data sent to Stripe: %s", k)
		}
	}
	// Same ten-minute window: same idempotency key (a double click can't create two sessions).
	s.CreateCheckout(context.Background(), "br_1", "tryst_plus")
	if (*calls)[1].Idem != c.Idem {
		t.Fatal("idempotency key changed within the window")
	}
	s.CreateCheckout(context.Background(), "br_1", "keys_60")
	if f := (*calls)[2].Form; f.Get("mode") != "payment" || f.Get("payment_intent_data[metadata][billing_ref]") != "br_1" {
		t.Fatalf("one-off checkout: %v", f)
	}
	if _, err := s.CreateCheckout(context.Background(), "br_1", "nope"); !errors.Is(err, ErrUnknownPrice) {
		t.Fatalf("unknown price: %v", err)
	}
	var se *StripeError
	if _, err := s.CreateCheckout(context.Background(), "br_1", "broken"); !errors.As(err, &se) || se.Code != "resource_missing" {
		t.Fatalf("api error: %v", err)
	}
}

func TestStripeCancelRefundAndKeys(t *testing.T) {
	srv, calls := fakeStripe(t)
	s := testStripe(srv.URL)
	ctx := context.Background()
	if err := s.CancelSubscription(ctx, "br_1"); err != nil {
		t.Fatal(err)
	}
	if c := (*calls)[0]; c.Path != "/v1/subscriptions/sub_1" || c.Form.Get("cancel_at_period_end") != "true" {
		t.Fatalf("cancel: %+v", c)
	}
	if err := s.CancelSubscriptionNow(ctx, "br_1"); err != nil || (*calls)[1].Method != "DELETE" {
		t.Fatalf("cancel now: %v %+v", err, (*calls)[1])
	}
	if err := s.Refund(ctx, "pi_1", 1250); err != nil {
		t.Fatal(err)
	}
	if c := (*calls)[2]; c.Path != "/v1/refunds" || c.Form.Get("amount") != "1250" || c.Form.Get("payment_intent") != "pi_1" || c.Idem == "" {
		t.Fatalf("refund: %+v", c)
	}
	if s.Refund(ctx, "pi_1", 0) == nil {
		t.Fatal("zero refund allowed")
	}
	if err := s.CancelSubscription(ctx, "br_unknown"); err == nil {
		t.Fatal("cancel without subscription")
	}
	s.SecretKey = "pk_test_123"
	if _, err := s.CreateCheckout(ctx, "br_1", "keys_60"); !errors.Is(err, ErrStripeKey) {
		t.Fatalf("publishable key accepted: %v", err)
	}
}
