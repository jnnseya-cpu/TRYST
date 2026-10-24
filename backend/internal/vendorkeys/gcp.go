package vendorkeys

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// SecretManager keeps settings in Google Secret Manager, signed in as the VM's own service
// account through the metadata server (no key file). Terraform gives that account
// "add version" on the catalogue's secrets and on SETTINGS_REVISION only, and read access
// to versions' metadata: it can't read or change anything else.
type SecretManager struct {
	Prefix      string // "tryst-prod-"
	Project     string // empty: ask the metadata server
	MetadataURL string // default http://169.254.169.254 (metadata.google.internal, no DNS needed)
	APIURL      string // default https://secretmanager.googleapis.com
	HTTP        *http.Client

	mu      sync.Mutex
	token   string
	expires time.Time
}

// RevisionSecret is bumped by Apply; deploy/gcp/apply-settings.sh restarts the services when
// it changes.
const RevisionSecret = "SETTINGS_REVISION"

func (g *SecretManager) client() *http.Client {
	if g.HTTP != nil {
		return g.HTTP
	}
	return &http.Client{Timeout: 15 * time.Second}
}

func (g *SecretManager) meta(ctx context.Context, path string) ([]byte, error) {
	base := g.MetadataURL
	if base == "" {
		base = "http://169.254.169.254"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/computeMetadata/v1/"+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Metadata-Flavor", "Google")
	resp, err := g.client().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("metadata %s: http %d", path, resp.StatusCode)
	}
	return b, nil
}

func (g *SecretManager) auth(ctx context.Context) (string, string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.Project == "" {
		b, err := g.meta(ctx, "project/project-id")
		if err != nil {
			return "", "", err
		}
		g.Project = strings.TrimSpace(string(b))
	}
	if g.token == "" || time.Now().After(g.expires) {
		b, err := g.meta(ctx, "instance/service-accounts/default/token")
		if err != nil {
			return "", "", err
		}
		var t struct {
			AccessToken string `json:"access_token"`
			ExpiresIn   int    `json:"expires_in"`
		}
		if json.Unmarshal(b, &t) != nil || t.AccessToken == "" {
			return "", "", errors.New("metadata: no access token")
		}
		g.token, g.expires = t.AccessToken, time.Now().Add(time.Duration(t.ExpiresIn-60)*time.Second)
	}
	return g.token, g.Project, nil
}

func (g *SecretManager) call(ctx context.Context, method, path string, body any, out any) (int, error) {
	tok, project, err := g.auth(ctx)
	if err != nil {
		return 0, err
	}
	base := g.APIURL
	if base == "" {
		base = "https://secretmanager.googleapis.com"
	}
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, base+"/v1/projects/"+url.PathEscape(project)+"/secrets/"+path, rd)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	resp, err := g.client().Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode/100 != 2 {
		return resp.StatusCode, fmt.Errorf("secret manager %s: http %d", method, resp.StatusCode)
	}
	if out != nil {
		return resp.StatusCode, json.Unmarshal(b, out)
	}
	return resp.StatusCode, nil
}

func (g *SecretManager) Status(ctx context.Context, names []string) (map[string]Status, error) {
	out := map[string]Status{}
	for _, n := range names {
		var v struct {
			Versions []struct {
				CreateTime time.Time `json:"createTime"`
			} `json:"versions"`
		}
		code, err := g.call(ctx, http.MethodGet, url.PathEscape(g.Prefix+n)+"/versions?filter=state%3AENABLED&pageSize=1", nil, &v)
		switch {
		case code == http.StatusNotFound:
			out[n] = Status{}
		case err != nil:
			return nil, err
		case len(v.Versions) > 0:
			out[n] = Status{Set: true, Updated: v.Versions[0].CreateTime}
		default:
			out[n] = Status{}
		}
	}
	return out, nil
}

func (g *SecretManager) add(ctx context.Context, name string, data []byte) error {
	sum := crc32.Checksum(data, crc32.MakeTable(crc32.Castagnoli))
	body := map[string]any{"payload": map[string]string{
		"data": base64.StdEncoding.EncodeToString(data), "dataCrc32c": strconv.FormatUint(uint64(sum), 10)}}
	_, err := g.call(ctx, http.MethodPost, url.PathEscape(g.Prefix+name)+":addVersion", body, nil)
	return err
}

func (g *SecretManager) Set(ctx context.Context, name, value string) error {
	if _, ok := Lookup(name); !ok {
		return ErrUnknown
	}
	return g.add(ctx, name, []byte(value))
}

func (g *SecretManager) Apply(ctx context.Context) error {
	return g.add(ctx, RevisionSecret, []byte(time.Now().UTC().Format(time.RFC3339Nano)))
}

// Memory keeps settings in memory: development and tests only.
type Memory struct {
	mu      sync.Mutex
	Values  map[string]string
	Times   map[string]time.Time
	Applied int
	Now     func() time.Time
}

func (m *Memory) Status(_ context.Context, names []string) (map[string]Status, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := map[string]Status{}
	for _, n := range names {
		_, ok := m.Values[n]
		out[n] = Status{Set: ok, Updated: m.Times[n]}
	}
	return out, nil
}

func (m *Memory) Set(_ context.Context, name, value string) error {
	if _, ok := Lookup(name); !ok {
		return ErrUnknown
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Values == nil {
		m.Values, m.Times = map[string]string{}, map[string]time.Time{}
	}
	now := time.Now
	if m.Now != nil {
		now = m.Now
	}
	m.Values[name], m.Times[name] = value, now()
	return nil
}

func (m *Memory) Apply(context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Applied++
	return nil
}
