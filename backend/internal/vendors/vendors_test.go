package vendors

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jnnseya-cpu/tryst/backend/internal/identity"
	"github.com/jnnseya-cpu/tryst/backend/internal/meet"
	"github.com/jnnseya-cpu/tryst/backend/internal/profile"
)

// vendor is a fake vendor that checks the gateway signature like a real shim would.
type vendor struct {
	t       *testing.T
	srv     *httptest.Server
	mu      sync.Mutex
	bodies  map[string]map[string]any
	answers map[string]string
	status  map[string]int
}

func newVendor(t *testing.T) *vendor {
	v := &vendor{t: t, bodies: map[string]map[string]any{}, answers: map[string]string{}, status: map[string]int{}}
	v.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		ts, _ := strconv.ParseInt(r.Header.Get("X-Tryst-Timestamp"), 10, 64)
		if r.Header.Get("X-Tryst-Signature") != Sign("shared-secret", ts, b) || r.Header.Get("Authorization") != "Bearer key-1" {
			w.WriteHeader(401)
			return
		}
		m := map[string]any{}
		_ = json.Unmarshal(b, &m)
		v.mu.Lock()
		v.bodies[r.URL.Path] = m
		st, ans := v.status[r.URL.Path], v.answers[r.URL.Path]
		v.mu.Unlock()
		if st != 0 {
			w.WriteHeader(st)
			return
		}
		io.WriteString(w, ans)
	}))
	t.Cleanup(v.srv.Close)
	return v
}

func (v *vendor) gw() *Gateway {
	return &Gateway{BaseURL: v.srv.URL, APIKey: "key-1", Secret: "shared-secret"}
}

func TestGatewaySignatureAndLiveness(t *testing.T) {
	v := newVendor(t)
	l := &Liveness{G: v.gw()}
	cases := []struct {
		answer string
		want   bool
	}{
		{`{"match":true,"confidence":0.97,"pad_passed":true}`, true},
		{`{"match":true,"confidence":0.80,"pad_passed":true}`, false},                           // low confidence
		{`{"match":true,"confidence":0.99,"pad_passed":false}`, false},                          // presentation attack
		{`{"match":true,"confidence":0.99,"pad_passed":true,"injection_detected":true}`, false}, // virtual camera
		{`{"match":false,"confidence":0.99,"pad_passed":true}`, false},
	}
	for _, c := range cases {
		v.answers["/liveness/verify"] = c.answer
		got, err := l.Verify(context.Background(), "acc-1", "sdk-evidence-token")
		if err != nil || got != c.want {
			t.Fatalf("%s: got %v %v", c.answer, got, err)
		}
	}
	if b := v.bodies["/liveness/verify"]; b["reference"] != "acc-1" || len(b) != 2 {
		t.Fatalf("liveness body: %v", b)
	}
	if ok, _ := l.Verify(context.Background(), "acc-1", ""); ok {
		t.Fatal("empty evidence passed")
	}
	bad := v.gw()
	bad.Secret = "wrong"
	if ok, err := (&Liveness{G: bad}).Verify(context.Background(), "acc-1", "x"); ok || err == nil {
		t.Fatal("unsigned request accepted or error swallowed")
	}
}

func fakeSightengine(t *testing.T, ai float64, sexual float64, status int) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1 << 20); err != nil || r.FormValue("api_user") != "u" || r.FormValue("models") != "genai,nudity-2.1" {
			w.WriteHeader(400)
			return
		}
		if _, _, err := r.FormFile("media"); err != nil {
			w.WriteHeader(400)
			return
		}
		if status != 0 {
			w.WriteHeader(status)
			io.WriteString(w, `{"status":"failure"}`)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"status": "success", "type": map[string]any{"ai_generated": ai},
			"nudity": map[string]any{"sexual_activity": sexual, "sexual_display": 0.0, "erotica": 0.0, "none": 1 - sexual}})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func fakePhotoDNA(t *testing.T, match bool) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in map[string]string
		_ = json.NewDecoder(r.Body).Decode(&in)
		if r.Header.Get("Ocp-Apim-Subscription-Key") != "pdna" || in["DataRepresentation"] != "inline" || in["Value"] == "" {
			w.WriteHeader(401)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"Status": map[string]any{"Code": 3000}, "IsMatch": match})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestPhotoModerator(t *testing.T) {
	v := newVendor(t)
	v.answers["/photos/analyse"] = `{"faces":[{"match":[0.96]}],"reverse_image_hit":false}`
	mod := func(se, pd *httptest.Server) *PhotoModerator {
		return &PhotoModerator{Sightengine: &Sightengine{User: "u", Secret: "s", BaseURL: se.URL}, PhotoDNA: &PhotoDNA{Key: "pdna", BaseURL: pd.URL}, Faces: v.gw()}
	}
	img := []byte("jpeg-bytes")
	c, _ := mod(fakeSightengine(t, 0.03, 0.01, 0), fakePhotoDNA(t, false)).Check(context.Background(), "acc-1", img)
	if !c.DetectorRan || c.CSAMHashHit || c.Explicit || c.SyntheticScore != 0.03 || len(c.Faces) != 1 || c.Faces[0].Match[0] != 0.96 {
		t.Fatalf("clean photo: %+v", c)
	}
	if d := profile.Moderate(c, 1, true); d.Outcome != profile.Approve {
		t.Fatalf("clean photo outcome: %+v", d)
	}
	c, _ = mod(fakeSightengine(t, 0.99, 0.9, 0), fakePhotoDNA(t, false)).Check(context.Background(), "acc-1", img)
	if !c.Explicit || c.SyntheticScore != 0.99 {
		t.Fatalf("synthetic explicit: %+v", c)
	}
	c, _ = mod(fakeSightengine(t, 0.01, 0, 0), fakePhotoDNA(t, true)).Check(context.Background(), "acc-1", img)
	if d := profile.Moderate(c, 1, false); !c.CSAMHashHit || d.Outcome != profile.Report {
		t.Fatalf("CSAM: %+v %+v", c, d)
	}
	// One vendor down: fail closed to a person.
	c, _ = mod(fakeSightengine(t, 0, 0, 503), fakePhotoDNA(t, false)).Check(context.Background(), "acc-1", img)
	if c.DetectorRan {
		t.Fatal("vendor failure didn't fail closed")
	}
	if d := profile.Moderate(c, 1, false); d.Outcome != profile.Review {
		t.Fatalf("vendor down outcome: %+v", d)
	}
}

func TestARCAlertingSendsOnlyPinnedFields(t *testing.T) {
	v := newVendor(t)
	v.answers["/alerts"] = `{"reference":"ARC-123"}`
	var msgs []string
	a := &ARCAlerting{ARC: v.gw(), Contact: messengerFunc(func(_ context.Context, to, text string) error {
		msgs = append(msgs, to+"|"+text)
		return nil
	})}
	al := meet.Alert{ID: "al_1", Reason: "missed_check_in", MemberAlias: "Blue", CallbackPhone: "+447700900123", Location: "near Angel", MeetWindow: "20:00–23:00", Started: time.Now()}
	if err := a.SafetyLine(context.Background(), al); err != nil {
		t.Fatal(err)
	}
	var keys []string
	for k := range v.bodies["/alerts"] {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	want := []string{"callback_phone", "id", "location", "meet_window", "member_alias", "priority", "reason", "started"}
	if !reflect.DeepEqual(keys, want) || len(meet.Allowed) != 7 {
		t.Fatalf("ARC fields = %v (meet.Allowed = %v)", keys, meet.Allowed)
	}
	if err := a.Emergency(context.Background(), al); err != nil || v.bodies["/alerts"]["priority"] != "emergency" {
		t.Fatalf("emergency: %v %v", err, v.bodies["/alerts"])
	}
	v.answers["/alerts"] = `{}`
	if err := a.SafetyLine(context.Background(), al); err == nil {
		t.Fatal("alert without an ARC reference counted as delivered")
	}
	v.status["/alerts"] = 503
	if err := a.Emergency(context.Background(), al); err == nil {
		t.Fatal("ARC failure swallowed")
	}
	_ = a.TrustedContact(context.Background(), "+447700900999", "Blue", "23:00", false)
	_ = a.TrustedContact(context.Background(), "friend@example.org", "Blue", "23:00", true)
	for _, m := range msgs {
		if strings.Contains(strings.ToLower(m), "tryst") || strings.Contains(strings.ToLower(m), "date") || !strings.Contains(m, "Blue") {
			t.Fatalf("trusted contact message: %q", m)
		}
	}
	if !strings.Contains(msgs[1], "999") {
		t.Fatalf("duress message should be urgent: %q", msgs[1])
	}
}

type messengerFunc func(ctx context.Context, to, text string) error

func (f messengerFunc) Message(ctx context.Context, to, text string) error { return f(ctx, to, text) }

func TestAssuranceAdapter(t *testing.T) {
	v := newVendor(t)
	v.answers["/assurance/sessions"] = `{"url":"https://idv.example/s/abc"}`
	now := time.Unix(1_800_000_000, 0)
	a := &Assurance{G: v.gw(), CallbackSecret: "cb-secret", Now: func() time.Time { return now }}
	u, err := a.Start(context.Background(), identity.AssuranceSession{ID: "as_1", Kind: "age", Account: "acc-1"}, "https://app.example/verify")
	if err != nil || u != "https://idv.example/s/abc" {
		t.Fatalf("start: %q %v", u, err)
	}
	if b := v.bodies["/assurance/sessions"]; b["reference"] != "acc-1" || b["kind"] != "age" || len(b) != 4 {
		t.Fatalf("start body: %v", b)
	}
	body := []byte(`{"session_id":"as_1","result":"pass"}`)
	req := func(ts int64, sig string) *http.Request {
		r := httptest.NewRequest("POST", "/v1/assurance/callback", nil)
		r.Header.Set("X-Provider-Timestamp", strconv.FormatInt(ts, 10))
		r.Header.Set("X-Provider-Signature", sig)
		return r
	}
	if err := a.VerifyCallback(req(now.Unix(), Sign("cb-secret", now.Unix(), body)), body); err != nil {
		t.Fatalf("genuine callback refused: %v", err)
	}
	for name, r := range map[string]*http.Request{
		"wrong secret": req(now.Unix(), Sign("other", now.Unix(), body)),
		"stale":        req(now.Add(-10*time.Minute).Unix(), Sign("cb-secret", now.Add(-10*time.Minute).Unix(), body)),
		"other body":   req(now.Unix(), Sign("cb-secret", now.Unix(), []byte(`{"result":"fail"}`))),
	} {
		if a.VerifyCallback(r, body) == nil {
			t.Errorf("%s accepted", name)
		}
	}
}

func TestLivenessStartOpensAHostedCapture(t *testing.T) {
	v := newVendor(t)
	l := &Liveness{G: v.gw()}
	v.answers["/liveness/sessions"] = `{"session_id":"lv_1","url":"https://capture.provider.example/lv_1"}`
	ref, url, err := l.Start(context.Background(), "acc-1", "https://app.example/liveness-done")
	if err != nil || ref != "lv_1" || url != "https://capture.provider.example/lv_1" {
		t.Fatalf("start: %q %q %v", ref, url, err)
	}
	if b := v.bodies["/liveness/sessions"]; b["reference"] != "acc-1" || b["return_url"] != "https://app.example/liveness-done" || len(b) != 2 {
		t.Fatalf("start body: %v", b)
	}
	for _, ans := range []string{`{"session_id":"lv_1","url":"http://capture.provider.example/lv_1"}`, `{"url":"https://capture.provider.example/x"}`} {
		v.answers["/liveness/sessions"] = ans
		if _, _, err := l.Start(context.Background(), "acc-1", "https://app.example/liveness-done"); err == nil {
			t.Fatalf("accepted %s", ans)
		}
	}
}
