package profilesvc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jnnseya-cpu/tryst/backend/internal/gate"
	"github.com/jnnseya-cpu/tryst/backend/internal/identity"
)

type multiAuth map[string]identity.Account

func (m multiAuth) Authenticate(r *http.Request) (identity.Account, bool) {
	a, ok := m[strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")]
	return a, ok
}

type world struct {
	t   *testing.T
	s   *Service
	now time.Time
}

func newWorld(t *testing.T, env string, members ...string) *world {
	auth := multiAuth{}
	for _, m := range members {
		auth[m] = identity.Account{ID: "acc-" + m, Tier: gate.V2, BirthYM: "1985-01"}
	}
	var mod Moderator = DevModerator{}
	if env != "dev" {
		mod = NoModerator{}
	}
	s, err := New(auth, mod, env)
	if err != nil {
		t.Fatal(err)
	}
	w := &world{t: t, s: s, now: time.Date(2026, 10, 11, 20, 0, 0, 0, time.UTC)}
	s.now = func() time.Time { return w.now }
	return w
}

func (w *world) do(who, method, path, body string) (int, map[string]any) {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+who)
	rec := httptest.NewRecorder()
	w.s.ServeHTTP(rec, req)
	var out map[string]any
	json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

// ready gives a member a complete profile in an area.
func (w *world) ready(who, district string) {
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("photo", "p.jpg")
	fw.Write(testJPEG(w.t, 300, 300))
	mw.Close()
	req := httptest.NewRequest("POST", "/v1/profile/photos", &body)
	req.Header.Set("Authorization", "Bearer "+who)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	w.s.ServeHTTP(rec, req)
	if rec.Code != 201 {
		w.t.Fatalf("upload for %s: %d", who, rec.Code)
	}
	area := ""
	for _, a := range Areas() {
		if a.District == district {
			area = a.ID
		}
	}
	if c, out := w.do(who, "PUT", "/v1/profile", fmt.Sprintf(`{"display_name":"%s","area_id":"%s"}`, strings.ToUpper(who[:1])+who[1:], area)); c != 200 {
		w.t.Fatalf("details for %s: %d %v", who, c, out)
	}
}

func people(out map[string]any, key string) []map[string]any {
	var res []map[string]any
	for _, p := range out[key].([]any) {
		res = append(res, p.(map[string]any))
	}
	return res
}

func TestTonightIsReciprocalAndNearbyOnly(t *testing.T) {
	w := newWorld(t, "dev", "ana", "ben", "cai", "dee")
	w.ready("ana", "Islington")
	w.ready("ben", "Camden")
	w.ready("cai", "Hackney")
	w.ready("dee", "City Centre") // Manchester (first "City Centre" in the list)

	// Ana switches on; nobody else is on yet.
	if c, out := w.do("ana", "PUT", "/v1/tonight", `{"hours":4,"radius_km":10}`); c != 200 || len(people(out, "people")) != 0 {
		t.Fatalf("ana on: %d %v", c, out)
	}
	// Ben is on; Cai is not: Ana sees Ben only. Cai, who is off, sees nobody.
	w.do("ben", "PUT", "/v1/tonight", `{"hours":2,"radius_km":10}`)
	_, out := w.do("ana", "GET", "/v1/tonight", "")
	ps := people(out, "people")
	if len(ps) != 1 || ps[0]["display_name"] != "Ben" || ps[0]["age"] == nil || ps[0]["distance"] == "" {
		t.Fatalf("ana sees: %v", ps)
	}
	if _, out := w.do("cai", "GET", "/v1/tonight", ""); out["on"] != false || len(people(out, "people")) != 0 {
		t.Fatalf("someone who is off must see nobody: %v", out)
	}
	// Dee is on but in another city: out of range both ways.
	w.do("dee", "PUT", "/v1/tonight", `{"hours":2,"radius_km":50}`)
	if _, out := w.do("dee", "GET", "/v1/tonight", ""); len(people(out, "people")) != 0 {
		t.Fatalf("dee sees London: %v", people(out, "people"))
	}
	// The exact account id never appears; handles differ per viewer.
	_, outB := w.do("ben", "GET", "/v1/tonight", "")
	if ps[0]["handle"] == people(outB, "people")[0]["handle"] || strings.Contains(fmt.Sprint(out), "acc-") {
		t.Fatal("handles must be per viewer and opaque")
	}
}

func TestInterestIsPrivateUntilMutual(t *testing.T) {
	w := newWorld(t, "dev", "ana", "ben")
	w.ready("ana", "Islington")
	w.ready("ben", "Camden")
	w.do("ana", "PUT", "/v1/tonight", `{"hours":4,"radius_km":10}`)
	w.do("ben", "PUT", "/v1/tonight", `{"hours":4,"radius_km":10}`)
	_, out := w.do("ana", "GET", "/v1/tonight", "")
	benHandle := people(out, "people")[0]["handle"].(string)
	if c, _ := w.do("ana", "POST", "/v1/tonight/interest/"+benHandle, ""); c != 200 {
		t.Fatal(c)
	}
	// Ben is not told.
	_, outB := w.do("ben", "GET", "/v1/tonight", "")
	if fmt.Sprint(outB["people"]) != fmt.Sprint(people(outB, "people")) || people(outB, "people")[0]["interested"] != false || len(people(outB, "mutual")) != 0 {
		t.Fatalf("one-sided interest leaked: %v", outB)
	}
	anaHandle := people(outB, "people")[0]["handle"].(string)
	_, outB = w.do("ben", "POST", "/v1/tonight/interest/"+anaHandle, "")
	if m := people(outB, "mutual"); len(m) != 1 || m[0]["display_name"] != "Ana" {
		t.Fatalf("mutual for ben: %v", outB)
	}
	if _, out := w.do("ana", "GET", "/v1/tonight", ""); len(people(out, "mutual")) != 1 {
		t.Fatalf("mutual for ana: %v", out)
	}
	// Another viewer's handle doesn't work for Ana.
	if c, _ := w.do("ana", "POST", "/v1/tonight/interest/"+anaHandle, ""); c != 404 {
		t.Fatalf("foreign handle: %d", c)
	}
}

func TestTonightSwitchesItselfOffAndNeedsAProfile(t *testing.T) {
	w := newWorld(t, "dev", "ana", "ben", "new")
	w.ready("ana", "Islington")
	w.ready("ben", "Camden")
	if c, _ := w.do("new", "PUT", "/v1/tonight", `{"hours":2,"radius_km":10}`); c != 409 {
		t.Fatalf("incomplete profile: %d", c)
	}
	if c, _ := w.do("ana", "PUT", "/v1/tonight", `{"hours":9,"radius_km":10}`); c != 422 {
		t.Fatalf("over 6 hours: %d", c)
	}
	w.do("ana", "PUT", "/v1/tonight", `{"hours":1,"radius_km":10}`)
	w.do("ben", "PUT", "/v1/tonight", `{"hours":6,"radius_km":10}`)
	w.now = w.now.Add(61 * time.Minute)
	if _, out := w.do("ben", "GET", "/v1/tonight", ""); len(people(out, "people")) != 0 {
		t.Fatal("expired member still visible")
	}
	if _, out := w.do("ana", "GET", "/v1/tonight", ""); out["on"] != false {
		t.Fatal("tonight should switch itself off")
	}
	// Stopping hides you at once.
	w.do("ana", "PUT", "/v1/tonight", `{"hours":2,"radius_km":10}`)
	w.do("ana", "DELETE", "/v1/tonight", "")
	if _, out := w.do("ben", "GET", "/v1/tonight", ""); len(people(out, "people")) != 0 {
		t.Fatal("stopped member still visible")
	}
}

func TestTonightGatesAndLimits(t *testing.T) {
	w := newWorld(t, "prod", "ana", "ben")
	w.s.mod = DevModerator{} // test seam: simulate vendors while keeping prod tier rules
	w.ready("ana", "Islington")
	w.ready("ben", "Camden")
	// Stage 0 hook: Ben is in Ana's partner shield.
	w.s.Eligible = func(a, b string) bool { return !(a == "acc-ana" && b == "acc-ben" || a == "acc-ben" && b == "acc-ana") }
	w.do("ana", "PUT", "/v1/tonight", `{"hours":2,"radius_km":10}`)
	w.do("ben", "PUT", "/v1/tonight", `{"hours":2,"radius_km":10}`)
	if _, out := w.do("ana", "GET", "/v1/tonight", ""); len(people(out, "people")) != 0 {
		t.Fatal("shielded partner visible")
	}
	// Production requires V2.
	w.s.auth.(multiAuth)["low"] = identity.Account{ID: "acc-low", Tier: gate.V1}
	w.ready("low", "Hackney")
	if c, _ := w.do("low", "PUT", "/v1/tonight", `{"hours":2,"radius_km":10}`); c != 403 {
		t.Fatalf("V1 in prod: %d", c)
	}
}

func TestInterestLimitAndBlurredPhotosNeverServed(t *testing.T) {
	names := []string{"me"}
	for i := 0; i < 12; i++ {
		names = append(names, fmt.Sprintf("m%02d", i))
	}
	w := newWorld(t, "dev", names...)
	for _, n := range names {
		w.ready(n, "Islington")
		w.do(n, "PUT", "/v1/tonight", `{"hours":3,"radius_km":5}`)
	}
	_, out := w.do("me", "GET", "/v1/tonight", "")
	ps := people(out, "people")
	if len(ps) != 12 {
		t.Fatalf("sees %d", len(ps))
	}
	for i, p := range ps {
		c, _ := w.do("me", "POST", "/v1/tonight/interest/"+p["handle"].(string), "")
		if i < 10 && c != 200 || i >= 10 && c != 429 {
			t.Fatalf("interest %d: %d", i, c)
		}
	}
	// m00 adds a second photo and blurs it: the viewer gets the main photo only.
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("photo", "p.jpg")
	fw.Write(testJPEG(t, 300, 300))
	mw.Close()
	req := httptest.NewRequest("POST", "/v1/profile/photos", &body)
	req.Header.Set("Authorization", "Bearer m00")
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	w.s.ServeHTTP(rec, req)
	var up map[string]any
	json.Unmarshal(rec.Body.Bytes(), &up)
	second := up["photo"].(map[string]any)["id"].(string)
	w.do("m00", "PATCH", "/v1/profile/photos/"+second, `{"revealed":false}`)
	_, out = w.do("me", "GET", "/v1/tonight", "")
	var m00 map[string]any
	for _, p := range people(out, "people") {
		if p["display_name"] == "M00" {
			m00 = p
		}
	}
	if m00["photos"].(float64) != 1 || m00["blurred"].(float64) != 1 {
		t.Fatalf("counts: %v", m00)
	}
	h := m00["handle"].(string)
	if c, _ := w.do("me", "GET", "/v1/tonight/photos/"+h+"/0", ""); c != 200 {
		t.Fatalf("main photo: %d", c)
	}
	if c, _ := w.do("me", "GET", "/v1/tonight/photos/"+h+"/1", ""); c != 404 {
		t.Fatalf("blurred photo served: %d", c)
	}
}
