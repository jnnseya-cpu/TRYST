package profilesvc

import (
	"context"
	"testing"
)

func TestExportAndErase(t *testing.T) {
	w, tid := mutualPair(t)
	ctx := context.Background()
	w.do("ana", "PUT", "/v1/devices/key", `{"device_id":"dev-ana-0001","public_jwk":{"kty":"EC","crv":"P-256","x":"f83OJ3D2xF1Bg8vub9tLe1gHMzV76e8Tus9uPHvRVEU","y":"x_FEzRu9m36HLN_tue659LNpXW6pCyStikYjKIWI5a0"}}`)
	if c, _ := send(w, "ana", tid, "dev-ana-0001", "c1", "dev-ana-0001"); c != 201 {
		t.Fatalf("send: %d", c)
	}

	out, err := w.s.Export(ctx, "acc-ana")
	if err != nil {
		t.Fatal(err)
	}
	ex := out.(map[string]any)
	photos := ex["photos"].([]map[string]any)
	if len(photos) != 1 || photos[0]["data_base64"] == nil {
		t.Fatalf("export photos: %v", photos)
	}
	if ex["profile"].(map[string]any)["display_name"] != "Ana" || ex["conversations_note"] == nil {
		t.Fatalf("export: %v", ex["profile"])
	}

	w.s.mu.Lock()
	ph := w.s.data["acc-ana"].Photos[0]
	w.s.mu.Unlock()
	blob, _ := w.s.st.GetBlob(ctx, blobID("acc-ana", ph.ID))

	rc, err := w.s.Erase(ctx, "acc-ana")
	if err != nil || rc.Erased < 4 || len(rc.Retained) != 0 {
		t.Fatalf("erase: %+v %v", rc, err)
	}
	// The key is gone: the sealed photo (as in any backup) can't be opened.
	if _, err := w.s.vault.Open(ctx, "acc-ana", blob, []byte("photo:"+ph.ID)); err == nil {
		t.Fatal("photo still decryptable after erasure")
	}
	if _, err := w.s.st.GetBlob(ctx, blobID("acc-ana", ph.ID)); err == nil {
		t.Fatal("photo blob not deleted")
	}
	if c, _ := w.do("ben", "GET", "/v1/threads/"+tid, ""); c != 404 {
		t.Fatalf("conversation still there for the other member: %d", c)
	}
	if docs, _ := w.s.st.All(ctx, "msg:"+tid); len(docs) != 0 {
		t.Fatalf("messages left: %d", len(docs))
	}
	_, o := w.do("eve", "GET", "/v1/tonight", "")
	for _, p := range people(o, "people") {
		if p["display_name"] == "Ana" {
			t.Fatal("erased member still in Tonight")
		}
	}
	if _, err := w.s.st.Get(ctx, "profile", "acc-ana"); err == nil {
		t.Fatal("profile document left")
	}
	// Idempotent.
	if _, err := w.s.Erase(ctx, "acc-ana"); err != nil {
		t.Fatal(err)
	}

	// A permanent removal keeps only the standing.
	w.s.SetStanding("acc-ben", "permanent")
	rc, _ = w.s.Erase(ctx, "acc-ben")
	if len(rc.Retained) != 1 {
		t.Fatalf("permanent removal exception not reported: %+v", rc)
	}
	if b, err := w.s.st.Get(ctx, "standing", "acc-ben"); err != nil || string(b) != `"permanent"` {
		t.Fatal("permanent standing not kept")
	}
}
