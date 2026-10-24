package vendorkeys

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"hash/crc32"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
)

func TestValidate(t *testing.T) {
	ok := map[string]string{
		"SMTP_HOST": "smtp.hostinger.com", "SMTP_PORT": "465", "SMTP_USER": "hello@example.org",
		"OTP_EMAIL_FROM": "Member services <hello@example.org>", "ARC_URL": "https://arc.example/v1",
		"STRIPE_SECRET_KEY": "rk_live_abcdef", "STRIPE_WEBHOOK_SECRET": "whsec_abcdef", "STRIPE_PRICE_TRYST_PLUS": "price_12345",
		"ASSURANCE_PROVIDER": "yoti", "ASSURANCE_CALLBACK_SECRET": strings.Repeat("x", 32),
		"ASSURANCE_CERTS": `[{"name":"yoti","certs":[{"scheme":"pad_iso30107_3_level2","lab":"iBeta","evidence":"r1","valid_until":"2027-06-30T00:00:00Z"}]}]`,
	}
	for n, v := range ok {
		if _, err := Validate(n, v); err != nil {
			t.Errorf("%s %q refused: %v", n, v, err)
		}
	}
	bad := map[string]string{
		"SMTP_HOST": "smtp.hostinger.com:465", "SMTP_PORT": "25", "SMTP_USER": "Member <a@b.c>", "OTP_EMAIL_FROM": "hello@example.org",
		"ARC_URL": "http://arc.example", "STRIPE_SECRET_KEY": "pk_live_abcdef", "STRIPE_PRICE_TRYST_PLUS": "prod_123",
		"ASSURANCE_PROVIDER": "Yoti Ltd", "ASSURANCE_CALLBACK_SECRET": "short", "ASSURANCE_CERTS": `[{"name":"yoti","certs":[{"scheme":"x"}]}]`,
		"ARC_SECRET": "two\nlines-of-secret-value",
	}
	for n, v := range bad {
		if _, err := Validate(n, v); err == nil {
			t.Errorf("%s %q accepted", n, v)
		}
	}
	if _, err := Validate("DATA_KEK", "00"); err != ErrUnknown {
		t.Fatal("internal key accepted")
	}
	if v, _ := Validate("SMTP_PORT", "  465 \n"); v != "465" {
		t.Fatalf("not trimmed: %q", v)
	}
}

// Every catalogue setting must exist as a secret (Terraform), be read at boot
// (fetch-secrets.sh), be writable by the VM (Terraform) and be loadable by setup.sh.
func TestDeployFilesListEverySetting(t *testing.T) {
	files := map[string]string{}
	for _, f := range []string{"../../../deploy/gcp/terraform/secrets.tf", "../../../deploy/gcp/fetch-secrets.sh", "../../../deploy/gcp/setup.sh"} {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		files[f] = string(b)
	}
	for f, s := range files {
		for _, n := range append(Names(), RevisionSecret) {
			if n == RevisionSecret && !strings.HasSuffix(f, "secrets.tf") {
				continue
			}
			if !strings.Contains(s, `"`+n+`"`) && !strings.Contains(s, " "+n+" ") && !strings.Contains(s, " "+n+"\n") && !strings.Contains(s, " "+n+";") {
				t.Errorf("%s doesn't list %s", f, n)
			}
		}
	}
}

// fakeGoogle is the metadata server and Secret Manager's REST API.
type fakeGoogle struct {
	mu       sync.Mutex
	versions map[string][]string
	tokens   int
}

func (f *fakeGoogle) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch {
	case strings.HasPrefix(r.URL.Path, "/computeMetadata/"):
		if r.Header.Get("Metadata-Flavor") != "Google" {
			w.WriteHeader(403)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/project-id") {
			io.WriteString(w, "tryst-prod-123")
			return
		}
		f.tokens++
		io.WriteString(w, `{"access_token":"tok","expires_in":3600}`)
	case r.Header.Get("Authorization") != "Bearer tok":
		w.WriteHeader(401)
	case strings.HasSuffix(r.URL.Path, ":addVersion"):
		name := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/projects/tryst-prod-123/secrets/"), ":addVersion")
		var in struct {
			Payload struct{ Data, DataCrc32c string } `json:"payload"`
		}
		json.NewDecoder(r.Body).Decode(&in)
		data, _ := base64.StdEncoding.DecodeString(in.Payload.Data)
		if strconv.FormatUint(uint64(crc32.Checksum(data, crc32.MakeTable(crc32.Castagnoli))), 10) != in.Payload.DataCrc32c {
			w.WriteHeader(400)
			return
		}
		if _, ok := f.versions[name]; !ok {
			w.WriteHeader(404)
			return
		}
		f.versions[name] = append(f.versions[name], string(data))
		io.WriteString(w, `{}`)
	case strings.HasSuffix(r.URL.Path, "/versions"):
		name := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/projects/tryst-prod-123/secrets/"), "/versions")
		v, ok := f.versions[name]
		if !ok {
			w.WriteHeader(404)
			return
		}
		if len(v) == 0 {
			io.WriteString(w, `{}`)
			return
		}
		io.WriteString(w, `{"versions":[{"createTime":"2026-10-24T19:00:00Z"}]}`)
	default:
		w.WriteHeader(404)
	}
}

func TestSecretManagerStore(t *testing.T) {
	f := &fakeGoogle{versions: map[string][]string{"tryst-prod-ARC_SECRET": {}, "tryst-prod-ARC_URL": {}, "tryst-prod-SETTINGS_REVISION": {}}}
	srv := httptest.NewServer(f)
	defer srv.Close()
	g := &SecretManager{Prefix: "tryst-prod-", MetadataURL: srv.URL, APIURL: srv.URL}
	ctx := context.Background()

	st, err := g.Status(ctx, []string{"ARC_SECRET", "ARC_URL", "PHOTODNA_KEY"})
	if err != nil || st["ARC_SECRET"].Set || st["PHOTODNA_KEY"].Set {
		t.Fatalf("status before: %v %v", st, err)
	}
	if err := g.Set(ctx, "ARC_SECRET", "s3cret-value"); err != nil {
		t.Fatal(err)
	}
	if f.versions["tryst-prod-ARC_SECRET"][0] != "s3cret-value" {
		t.Fatalf("stored: %v", f.versions)
	}
	st, _ = g.Status(ctx, []string{"ARC_SECRET"})
	if !st["ARC_SECRET"].Set || st["ARC_SECRET"].Updated.IsZero() {
		t.Fatalf("status after: %v", st)
	}
	if err := g.Set(ctx, "DATA_KEK", "x"); err != ErrUnknown {
		t.Fatal("internal key written")
	}
	if err := g.Apply(ctx); err != nil || len(f.versions["tryst-prod-SETTINGS_REVISION"]) != 1 {
		t.Fatalf("apply: %v", err)
	}
	if f.tokens != 1 {
		t.Fatalf("token fetched %d times, want cached", f.tokens)
	}
}
