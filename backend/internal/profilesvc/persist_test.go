package profilesvc

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/jnnseya-cpu/tryst/backend/internal/identity"
	"github.com/jnnseya-cpu/tryst/backend/internal/store"
)

func profileRestart(t *testing.T, st store.Store) {
	ctx := context.Background()
	kek := bytes.Repeat([]byte{3}, 32)
	auth := fakeAuth{identity.Account{ID: "acc-restart", BirthYM: "1985-05"}}
	open := func(k []byte) *Service {
		v, _ := store.NewVault(st, k)
		s, err := NewWithStore(auth, DevModerator{}, "dev", st, v)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	s1 := open(kek)
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("photo", "p.jpg")
	fw.Write(testJPEG(t, 300, 300))
	mw.Close()
	req := httptest.NewRequest("POST", "/v1/profile/photos", &body)
	req.Header.Set("Authorization", "Bearer ok")
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	s1.ServeHTTP(rec, req)
	if rec.Code != 201 {
		t.Fatalf("upload: %d", rec.Code)
	}
	var up map[string]any
	json.Unmarshal(rec.Body.Bytes(), &up)
	photoID := up["photo"].(map[string]any)["id"].(string)
	call(t, s1, "PUT", "/v1/profile", `{"display_name":"Linden","area_id":"`+Areas()[0].ID+`"}`)

	// Stored blob is ciphertext: not a decodable image.
	raw, err := st.GetBlob(ctx, blobID("acc-restart", photoID))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := image.Decode(bytes.NewReader(raw)); err == nil {
		t.Fatal("photo stored in the clear")
	}

	s2 := open(kek) // restart
	_, card, _ := call(t, s2, "GET", "/v1/profile/card", "")
	if card["display_name"] != "Linden" || card["ready"] != true {
		t.Fatalf("card after restart: %v", card)
	}
	c, _, img := call(t, s2, "GET", "/v1/profile/photos/"+photoID, "")
	if c != 200 {
		t.Fatalf("photo after restart: %d", c)
	}
	if _, _, err := image.Decode(bytes.NewReader(img)); err != nil {
		t.Fatalf("photo not decodable after restart: %v", err)
	}
	// Wrong master key: photos cannot be opened.
	s3 := open(bytes.Repeat([]byte{9}, 32))
	if c, _, _ := call(t, s3, "GET", "/v1/profile/photos/"+photoID, ""); c != 404 {
		t.Fatalf("wrong KEK served a photo: %d", c)
	}
	// Deleting the photo removes the blob.
	call(t, s2, "DELETE", "/v1/profile/photos/"+photoID, "")
	if _, err := st.GetBlob(ctx, blobID("acc-restart", photoID)); err == nil {
		t.Fatal("blob survived delete")
	}
}

func TestProfileRestartMemory(t *testing.T) { profileRestart(t, store.NewMemory()) }

func TestProfileRestartPostgres(t *testing.T) {
	dsn := os.Getenv("TRYST_TEST_PG")
	if dsn == "" {
		t.Skip("TRYST_TEST_PG not set")
	}
	p, err := store.OpenPostgres(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	all, _ := p.All(context.Background(), "profile")
	for id := range all {
		if strings.HasPrefix(id, "acc-restart") {
			p.Delete(context.Background(), "profile", id)
		}
	}
	profileRestart(t, p)
}
