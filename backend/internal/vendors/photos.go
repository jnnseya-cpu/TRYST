package vendors

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/jnnseya-cpu/tryst/backend/internal/profile"
)

// Sightengine checks a photo for AI generation and nudity in one call
// (POST /1.0/check.json, models "genai,nudity-2.1"; response fields type.ai_generated and
// nudity.{sexual_activity,sexual_display,erotica}). Field names follow Sightengine's public
// documentation; confirm against the account's API version in P0.
type Sightengine struct {
	User, Secret string
	BaseURL      string
	HTTP         *http.Client
	// ExplicitAt is the score from which a photo counts as explicit (default 0.5).
	ExplicitAt float64
}

type photoResult struct {
	synthetic float64
	explicit  bool
}

func (s *Sightengine) check(ctx context.Context, img []byte) (photoResult, error) {
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	_ = mw.WriteField("models", "genai,nudity-2.1")
	_ = mw.WriteField("api_user", s.User)
	_ = mw.WriteField("api_secret", s.Secret)
	fw, _ := mw.CreateFormFile("media", "photo.jpg")
	_, _ = fw.Write(img)
	_ = mw.Close()
	base := s.BaseURL
	if base == "" {
		base = "https://api.sightengine.com"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(base, "/")+"/1.0/check.json", &body)
	if err != nil {
		return photoResult{}, err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	c := s.HTTP
	if c == nil {
		c = &http.Client{Timeout: 20 * time.Second}
	}
	resp, err := c.Do(req)
	if err != nil {
		return photoResult{}, err
	}
	defer resp.Body.Close()
	var out struct {
		Status string `json:"status"`
		Type   struct {
			AIGenerated *float64 `json:"ai_generated"`
		} `json:"type"`
		Nudity struct {
			SexualActivity float64 `json:"sexual_activity"`
			SexualDisplay  float64 `json:"sexual_display"`
			Erotica        float64 `json:"erotica"`
		} `json:"nudity"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil || resp.StatusCode != 200 || out.Status != "success" || out.Type.AIGenerated == nil {
		return photoResult{}, fmt.Errorf("%w: sightengine http %d", ErrVendor, resp.StatusCode)
	}
	at := s.ExplicitAt
	if at == 0 {
		at = 0.5
	}
	n := out.Nudity
	return photoResult{synthetic: *out.Type.AIGenerated, explicit: n.SexualActivity >= at || n.SexualDisplay >= at || n.Erotica >= at}, nil
}

// PhotoDNA matches against known-CSAM hash lists (Microsoft PhotoDNA Cloud Service,
// POST /photodna/v1.0/Match). Access requires Microsoft's approval for the organisation.
type PhotoDNA struct {
	Key     string
	BaseURL string
	HTTP    *http.Client
}

func (p *PhotoDNA) match(ctx context.Context, img []byte) (bool, error) {
	base := p.BaseURL
	if base == "" {
		base = "https://api.microsoftmoderator.com"
	}
	body, _ := json.Marshal(map[string]string{"DataRepresentation": "inline", "Value": base64.StdEncoding.EncodeToString(img)})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(base, "/")+"/photodna/v1.0/Match?enhance=false", bytes.NewReader(body))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Ocp-Apim-Subscription-Key", p.Key)
	c := p.HTTP
	if c == nil {
		c = &http.Client{Timeout: 20 * time.Second}
	}
	resp, err := c.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	var out struct {
		IsMatch *bool `json:"IsMatch"`
		Status  struct {
			Code int `json:"Code"`
		} `json:"Status"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil || resp.StatusCode != 200 || out.IsMatch == nil {
		return false, fmt.Errorf("%w: photodna http %d", ErrVendor, resp.StatusCode)
	}
	return *out.IsMatch, nil
}

// PhotoModerator is profilesvc.Moderator: all checks run in parallel. If the synthetic
// detector, the hash match or the face match fails, the photo is marked "detector didn't
// run", which sends it to a person (fail closed); a CSAM hit always wins.
type PhotoModerator struct {
	Sightengine *Sightengine
	PhotoDNA    *PhotoDNA
	// Faces calls the identity provider through the gateway: faces in the photo, each
	// scored against the member's verified reference, plus a reverse-image search hit.
	Faces *Gateway
}

func (m *PhotoModerator) Check(ctx context.Context, account string, img []byte) (profile.Checks, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	var (
		wg         sync.WaitGroup
		pr         photoResult
		csam       bool
		faces      []profile.Face
		reverse    bool
		e1, e2, e3 error
	)
	wg.Add(3)
	go func() { defer wg.Done(); pr, e1 = m.Sightengine.check(ctx, img) }()
	go func() { defer wg.Done(); csam, e2 = m.PhotoDNA.match(ctx, img) }()
	go func() {
		defer wg.Done()
		var out struct {
			Faces []struct {
				Match []float64 `json:"match"`
			} `json:"faces"`
			ReverseImageHit bool `json:"reverse_image_hit"`
		}
		e3 = m.Faces.Post(ctx, "/photos/analyse", map[string]string{"reference": account, "image": base64.StdEncoding.EncodeToString(img)}, &out)
		for _, f := range out.Faces {
			faces = append(faces, profile.Face{Match: f.Match})
		}
		reverse = out.ReverseImageHit
	}()
	wg.Wait()
	c := profile.Checks{CSAMHashHit: csam, SyntheticScore: pr.synthetic, Explicit: pr.explicit, Faces: faces, ReverseImageHit: reverse}
	c.DetectorRan = e1 == nil && e2 == nil && e3 == nil
	return c, nil
}
