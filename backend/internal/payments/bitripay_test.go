package payments

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type seen struct {
	path, auth, idem string
	body             map[string]any
}

func fakeBitriPay(t *testing.T, status int, reply string, got *[]seen) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var b map[string]any
		_ = json.NewDecoder(r.Body).Decode(&b)
		*got = append(*got, seen{r.URL.Path, r.Header.Get("Authorization"), r.Header.Get("Idempotency-Key"), b})
		w.WriteHeader(status)
		_, _ = w.Write([]byte(reply))
	}))
}

func adapter(url, key string) *BitriPay {
	return &BitriPay{BaseURL: url, Key: key, WebhookSecret: "whsec_bp", Market: "GB",
		Prices: map[string]Price{
			"tryst_plus_month": {AmountMinor: 2499, Currency: "GBP", Description: "TS DIGITAL SVCS"},
			"bad":              {AmountMinor: 100, Currency: "GBP", Description: "TRYST PLUS"},
		},
		SubscriptionFor: func(ref string) (string, error) { return "sub_" + ref, nil }}
}

func TestBitriPayCheckoutSandbox(t *testing.T) {
	var got []seen
	srv := fakeBitriPay(t, 200, `{"id":"pi_1","checkout_url":"https://pay.example/c/pi_1"}`, &got)
	defer srv.Close()
	b := adapter(srv.URL, "sk_test_abc")
	u, err := b.CreateCheckout(context.Background(), "bref_7", "tryst_plus_month")
	if err != nil || u != "https://pay.example/c/pi_1" {
		t.Fatalf("checkout: %q %v", u, err)
	}
	r := got[0]
	if r.path != "/payment_intents" || r.auth != "Bearer sk_test_abc" || !strings.HasPrefix(r.idem, "tryst-") {
		t.Fatalf("request: %+v", r)
	}
	if r.body["amount_minor"].(float64) != 2499 || r.body["currency"] != "GBP" {
		t.Fatalf("body: %v", r.body)
	}
	for k := range r.body {
		if k != "amount_minor" && k != "currency" && k != "description" {
			t.Fatalf("unexpected field sent to processor: %s", k)
		}
	}
	// Same logical operation → same idempotency key, so a retry cannot double-charge.
	_, _ = b.CreateCheckout(context.Background(), "bref_7", "tryst_plus_month")
	if got[1].idem != got[0].idem {
		t.Fatal("idempotency key not stable")
	}
}

func TestBitriPayLiveRefusedOutsideEnabledMarkets(t *testing.T) {
	var got []seen
	srv := fakeBitriPay(t, 200, `{}`, &got)
	defer srv.Close()
	b := adapter(srv.URL, "sk_live_abc")
	if _, err := b.CreateCheckout(context.Background(), "r", "tryst_plus_month"); !errors.Is(err, ErrLiveNotEnabled) {
		t.Fatalf("live checkout: %v", err)
	}
	if err := b.Refund(context.Background(), "pi_1", 100); !errors.Is(err, ErrLiveNotEnabled) {
		t.Fatalf("live refund: %v", err)
	}
	if len(got) != 0 {
		t.Fatal("request reached the processor")
	}
	b.LiveMarkets = map[string]bool{"GB": true}
	if _, err := b.CreateCheckout(context.Background(), "r", "tryst_plus_month"); err == nil || errors.Is(err, ErrLiveNotEnabled) {
		t.Fatalf("enabled market should reach the processor, got %v", err)
	}
}

func TestBitriPayRejectsPublishableKeyAndBadDescriptor(t *testing.T) {
	b := adapter("http://127.0.0.1:1", "pk_test_abc")
	if _, err := b.CreateCheckout(context.Background(), "r", "tryst_plus_month"); !errors.Is(err, ErrBadKey) {
		t.Fatalf("pk key: %v", err)
	}
	b.Key = "sk_test_abc"
	if _, err := b.CreateCheckout(context.Background(), "r", "bad"); !errors.Is(err, ErrDescriptor) {
		t.Fatalf("descriptor: %v", err)
	}
	if _, err := b.CreateCheckout(context.Background(), "r", "nope"); !errors.Is(err, ErrUnknownPrice) {
		t.Fatalf("price: %v", err)
	}
}

func TestBitriPayRefundCancelAndErrors(t *testing.T) {
	var got []seen
	srv := fakeBitriPay(t, 200, `{}`, &got)
	defer srv.Close()
	b := adapter(srv.URL, "rk_test_abc")
	if err := b.Refund(context.Background(), "pi_9", 500); err != nil {
		t.Fatal(err)
	}
	if err := b.CancelSubscription(context.Background(), "bref_7"); err != nil {
		t.Fatal(err)
	}
	if got[0].path != "/refunds" || got[0].body["payment_intent"] != "pi_9" || got[1].path != "/subscriptions/sub_bref_7/cancel" {
		t.Fatalf("calls: %+v", got)
	}
	if err := b.Refund(context.Background(), "pi_9", 0); err == nil {
		t.Fatal("zero refund accepted")
	}

	var got2 []seen
	bad := fakeBitriPay(t, 403, `{"error":{"code":"scope_denied"}}`, &got2)
	defer bad.Close()
	b.BaseURL = bad.URL
	var apiErr *APIError
	if err := b.Refund(context.Background(), "pi_9", 500); !errors.As(err, &apiErr) || apiErr.Code != "scope_denied" {
		t.Fatalf("api error: %v", err)
	}
}

func TestBitriPayWebhook(t *testing.T) {
	now := time.Unix(1_790_000_000, 0)
	body := []byte(`{"id":"evt_1","type":"payment_intent.succeeded"}`)
	b := adapter("", "sk_test_abc")
	if err := b.VerifyWebhook(body, sign("whsec_bp", now.Unix(), body), now); err != nil {
		t.Fatal(err)
	}
	if err := b.VerifyWebhook(body, sign("whsec_other", now.Unix(), body), now); err == nil {
		t.Fatal("wrong secret accepted")
	}
	b.WebhookSecret = ""
	if err := b.VerifyWebhook(body, sign("", now.Unix(), body), now); err == nil {
		t.Fatal("empty secret must fail closed")
	}
}
