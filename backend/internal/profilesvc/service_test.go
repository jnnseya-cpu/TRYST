package profilesvc

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jnnseya-cpu/tryst/backend/internal/identity"
)

type fakeAuth struct{ acc identity.Account }

func (f fakeAuth) Authenticate(r *http.Request) (identity.Account, bool) {
	if r.Header.Get("Authorization") != "Bearer ok" {
		return identity.Account{}, false
	}
	return f.acc, true
}

// testJPEG is a plain gradient test image (not a photo of anyone).
func testJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 120, 255})
		}
	}
	var b bytes.Buffer
	if err := jpeg.Encode(&b, img, nil); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// withSegment inserts an APPn segment right after the JPEG start marker.
func withSegment(j []byte, marker byte, payload string) []byte {
	seg := []byte{0xFF, marker, 0, 0}
	binary.BigEndian.PutUint16(seg[2:], uint16(len(payload)+2))
	seg = append(seg, payload...)
	out := append([]byte{}, j[:2]...)
	out = append(out, seg...)
	return append(out, j[2:]...)
}

func svc(t *testing.T, mod Moderator) *Service {
	t.Helper()
	s, err := New(fakeAuth{identity.Account{ID: "acc1", BirthYM: "1988-04"}}, mod, "dev")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func upload(t *testing.T, s *Service, img []byte) (int, map[string]any) {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("photo", "p.jpg")
	fw.Write(img)
	mw.Close()
	req := httptest.NewRequest("POST", "/v1/profile/photos", &body)
	req.Header.Set("Authorization", "Bearer ok")
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	var out map[string]any
	json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

func call(t *testing.T, s *Service, method, path, body string) (int, map[string]any, []byte) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer ok")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	var out map[string]any
	json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out, rec.Body.Bytes()
}

func TestDevModeratorRefusedInProd(t *testing.T) {
	if _, err := New(fakeAuth{}, DevModerator{}, "prod"); err == nil {
		t.Fatal("dev moderator allowed in prod")
	}
}

func TestSignInRequired(t *testing.T) {
	s := svc(t, DevModerator{})
	req := httptest.NewRequest("GET", "/v1/profile", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != 401 {
		t.Fatal(rec.Code)
	}
}

func TestUploadFlowAndCard(t *testing.T) {
	s := svc(t, DevModerator{})
	code, out := upload(t, s, testJPEG(t, 400, 500))
	if code != 201 {
		t.Fatalf("upload: %d %v", code, out)
	}
	first := out["photo"].(map[string]any)
	if first["main"] != true || first["status"] != "live" {
		t.Fatalf("first eligible photo becomes main: %v", first)
	}
	for i := 0; i < 9; i++ {
		if code, out := upload(t, s, testJPEG(t, 300, 300)); code != 201 {
			t.Fatalf("photo %d: %d %v", i+2, code, out)
		}
	}
	if code, out := upload(t, s, testJPEG(t, 300, 300)); code != 409 || !strings.Contains(out["title"].(string), "10 photos") {
		t.Fatalf("11th photo: %d %v", code, out)
	}
	// Main photo can't be blurred; others can.
	_, prof, _ := call(t, s, "GET", "/v1/profile", "")
	photos := prof["photos"].([]any)
	mainID := photos[0].(map[string]any)["id"].(string)
	otherID := photos[1].(map[string]any)["id"].(string)
	if c, _, _ := call(t, s, "PATCH", "/v1/profile/photos/"+mainID, `{"revealed":false}`); c != 422 {
		t.Fatalf("blurring main: %d", c)
	}
	if c, _, _ := call(t, s, "PATCH", "/v1/profile/photos/"+otherID, `{"revealed":false}`); c != 200 {
		t.Fatalf("blurring other: %d", c)
	}
	// Switch main; it moves first.
	if c, p, _ := call(t, s, "POST", "/v1/profile/photos/"+otherID+"/main", ""); c != 200 || p["photos"].([]any)[0].(map[string]any)["id"] != otherID {
		t.Fatalf("make main: %d", c)
	}
	// Details, with screening.
	if c, p, _ := call(t, s, "PUT", "/v1/profile", `{"display_name":"call 07700900123"}`); c != 422 || p["code"] != "text_refused" {
		t.Fatalf("contact details in name: %d %v", c, p)
	}
	areas := Areas()
	if c, _, _ := call(t, s, "PUT", "/v1/profile", `{"display_name":"Velvet","area_id":"`+areas[0].ID+`"}`); c != 200 {
		t.Fatalf("details: %d", c)
	}
	_, card, _ := call(t, s, "GET", "/v1/profile/card", "")
	if card["ready"] != true || card["display_name"] != "Velvet" || card["distance"] != "Nearby" || card["age"] == nil {
		t.Fatalf("card: %v", card)
	}
	if n := len(card["photos"].([]any)); n != 10 {
		t.Fatalf("card shows %d photos", n)
	}
	if card["photos"].([]any)[0].(map[string]any)["main"] != true {
		t.Fatal("main first on the card")
	}
}

func TestAILabelledPhotoIsRefusedAndNeverStored(t *testing.T) {
	s := svc(t, DevModerator{})
	ai := withSegment(testJPEG(t, 300, 300), 0xE1,
		"http://ns.adobe.com/xap/1.0/\x00<x Iptc4xmpExt:DigitalSourceType=\"http://cv.iptc.org/newscodes/digitalsourcetype/trainedAlgorithmicMedia\"/>")
	code, out := upload(t, s, ai)
	if code != 422 || !strings.Contains(out["title"].(string), "computer-generated") {
		t.Fatalf("AI photo: %d %v", code, out)
	}
	_, prof, _ := call(t, s, "GET", "/v1/profile", "")
	if n := len(prof["photos"].([]any)); n != 0 {
		t.Fatalf("refused photo stored: %d", n)
	}
}

func TestMetadataIsStrippedBeforeStorage(t *testing.T) {
	s := svc(t, DevModerator{})
	img := withSegment(testJPEG(t, 300, 300), 0xE1, "Exif\x00\x00GPSLatitude 51.5362 GPSLongitude -0.1033 SerialNumber 12345")
	code, out := upload(t, s, img)
	if code != 201 {
		t.Fatalf("%d %v", code, out)
	}
	id := out["photo"].(map[string]any)["id"].(string)
	_, _, raw := call(t, s, "GET", "/v1/profile/photos/"+id, "")
	if bytes.Contains(raw, []byte("GPS")) || bytes.Contains(raw, []byte("Serial")) || bytes.Contains(raw, []byte("Exif")) {
		t.Fatal("metadata survived")
	}
	if _, _, err := image.Decode(bytes.NewReader(raw)); err != nil {
		t.Fatalf("stored image is not decodable: %v", err)
	}
}

func TestWithoutVendorsPhotosWaitForAPerson(t *testing.T) {
	s := svc(t, NoModerator{})
	code, out := upload(t, s, testJPEG(t, 300, 300))
	if code != 201 {
		t.Fatalf("%d %v", code, out)
	}
	ph := out["photo"].(map[string]any)
	if ph["status"] != "checking" || ph["main"] == true {
		t.Fatalf("fail closed: %v", ph)
	}
	_, card, _ := call(t, s, "GET", "/v1/profile/card", "")
	if card["photos"] != nil || card["ready"] == true {
		t.Fatalf("unchecked photo visible: %v", card)
	}
}

func TestRejectsTinyAndNonImages(t *testing.T) {
	s := svc(t, DevModerator{})
	if code, _ := upload(t, s, testJPEG(t, 100, 100)); code != 422 {
		t.Fatalf("tiny: %d", code)
	}
	if code, _ := upload(t, s, []byte("not an image at all")); code != 422 {
		t.Fatalf("garbage: %d", code)
	}
}
