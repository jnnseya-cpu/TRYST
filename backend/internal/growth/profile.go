// Package growth is TRYST's growth intelligence (D-73): it reads aggregate demand signals,
// scores opportunities, issues one shared, time-limited Growth Brief, drafts SEO and
// marketing changes for the public guide pages, applies them only after an editor
// approves, measures guide views and join clicks without cookies, and rolls back.
//
// It runs as growth-svc beside identity-svc and safety-svc. It holds no member data: the
// member platform sends weekly counts by city with small cohorts withheld (k ≥ 50), and
// staff import Search Console exports. It has no credentials to any member database.
//
// Built to the Trend Intelligence build specification v2.3 (docs/spec/10_Growth_Intelligence.md).
package growth

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
)

//go:embed profile.tryst.json
var trystProfile []byte

var idRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$`)
var slugRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,80}$`)

// Channels in the brief contract; only the first three have a route at launch.
var Channels = []string{"seo", "organic_social", "owned_placement", "sales_enablement", "lifecycle", "paid"}
var EnabledChannels = []string{"seo", "owned_placement", "organic_social"}

type Source struct {
	ID              string  `json:"source_id"`
	Family          string  `json:"family"`
	AccessPath      string  `json:"access_path"`
	Kind            string  `json:"kind"`
	Quality         float64 `json:"quality"`
	StaleAfterHours float64 `json:"stale_after_hours"`
	ContextOnly     bool    `json:"context_only,omitempty"`
	MinCount        float64 `json:"min_count,omitempty"`
	MinZ            float64 `json:"min_z,omitempty"`
}

type MarketFit struct {
	Fit    float64 `json:"fit"`
	Intent float64 `json:"intent"`
}

type Topic struct {
	ID         string               `json:"topic_id"`
	Label      string               `json:"label"`
	Aliases    []string             `json:"aliases,omitempty"`
	TrendClass string               `json:"trend_class,omitempty"`
	Markets    map[string]MarketFit `json:"markets"`
	Readiness  float64              `json:"readiness"`
	Audience   string               `json:"audience"`
	Intent     string               `json:"intent"`
	ClaimIDs   []string             `json:"claim_ids"`
	CTAID      string               `json:"cta_id"`
	AssetIDs   []string             `json:"asset_ids"`
	Restricted bool                 `json:"restricted,omitempty"`
}

type Claim struct {
	ID     string `json:"claim_id"`
	Text   string `json:"text"`
	Source string `json:"source"`
}

type CTA struct {
	ID   string `json:"cta_id"`
	Text string `json:"text"`
	URL  string `json:"url"`
}

type Asset struct {
	ID            string            `json:"asset_id"`
	Slug          string            `json:"slug"`
	AllowedFields []string          `json:"allowed_fields"`
	WriteEnabled  bool              `json:"write_enabled"`
	Initial       map[string]string `json:"initial"`
}

type Profile struct {
	SchemaVersion   int      `json:"schema_version"`
	PlatformSlug    string   `json:"platform_slug"`
	Mode            string   `json:"mode"` // shadow | draft_only | live
	DefaultMarket   string   `json:"default_market"`
	DefaultLocale   string   `json:"default_locale"`
	Timezone        string   `json:"timezone"`
	PolicyVersion   int      `json:"policy_version"`
	AllowedChannels []string `json:"allowed_channels"`
	Objective       string   `json:"objective"`
	Outcome         struct {
		Conversion  string `json:"conversion"`
		Denominator string `json:"denominator"`
	} `json:"outcome"`
	ProhibitedClaims []string `json:"prohibited_claims"`
	Sources          []Source `json:"sources"`
	Topics           []Topic  `json:"topics"`
	Claims           []Claim  `json:"claims"`
	CTAs             []CTA    `json:"ctas"`
	Assets           []Asset  `json:"assets"`
	Inference        struct {
		Route     string `json:"route"` // deterministic | deployment
		Alternate string `json:"alternate,omitempty"`
	} `json:"inference"`

	Revision string `json:"-"` // content hash of the profile as loaded
}

// LoadProfile parses and validates a profile; unknown fields are refused.
func LoadProfile(b []byte) (*Profile, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	var p Profile
	if err := dec.Decode(&p); err != nil {
		return nil, fmt.Errorf("growth profile: %w", err)
	}
	if err := p.validate(); err != nil {
		return nil, fmt.Errorf("growth profile: %w", err)
	}
	sum := sha256.Sum256(b)
	p.Revision = hex.EncodeToString(sum[:])[:12]
	return &p, nil
}

// TrystProfile is TRYST's profile, reviewed in the repository (and baseline-locked).
func TrystProfile() *Profile {
	p, err := LoadProfile(trystProfile)
	if err != nil {
		panic(err)
	}
	return p
}

func (p *Profile) validate() error {
	if p.SchemaVersion != 1 {
		return fmt.Errorf("schema_version must be 1")
	}
	if !slices.Contains([]string{"shadow", "draft_only", "live"}, p.Mode) {
		return fmt.Errorf("mode %q", p.Mode)
	}
	if !regexp.MustCompile(`^[A-Z]{2}$`).MatchString(p.DefaultMarket) || p.PolicyVersion < 1 {
		return fmt.Errorf("default_market and policy_version are required")
	}
	if !slices.Contains([]string{"deterministic", "deployment"}, p.Inference.Route) {
		return fmt.Errorf("inference route %q", p.Inference.Route)
	}
	for _, c := range p.AllowedChannels {
		if !slices.Contains(Channels, c) {
			return fmt.Errorf("channel %q", c)
		}
	}
	srcs := map[string]bool{}
	for _, s := range p.Sources {
		if !idRe.MatchString(s.ID) || !idRe.MatchString(s.Family) || srcs[s.ID] {
			return fmt.Errorf("source %q", s.ID)
		}
		if s.Kind != "count" && s.Kind != "index" || s.StaleAfterHours <= 0 || s.Quality < 0 || s.Quality > 100 {
			return fmt.Errorf("source %q settings", s.ID)
		}
		srcs[s.ID] = true
	}
	for _, c := range p.Claims {
		if !idRe.MatchString(c.ID) || c.Text == "" || c.Source == "" {
			return fmt.Errorf("claim %q needs text and a source", c.ID)
		}
		if bad := p.Prohibited(c.Text); bad != "" {
			return fmt.Errorf("claim %q uses prohibited wording %q", c.ID, bad)
		}
	}
	for _, a := range p.Assets {
		if !idRe.MatchString(a.ID) || !slugRe.MatchString(a.Slug) {
			return fmt.Errorf("asset %q", a.ID)
		}
		for _, f := range a.AllowedFields {
			if _, ok := FieldLimits[f]; !ok {
				return fmt.Errorf("asset %q field %q", a.ID, f)
			}
		}
	}
	for _, t := range p.Topics {
		if !idRe.MatchString(t.ID) {
			return fmt.Errorf("topic %q", t.ID)
		}
		for _, c := range t.ClaimIDs {
			if p.Claim(c) == nil {
				return fmt.Errorf("topic %q uses unknown claim %q", t.ID, c)
			}
		}
		for _, a := range t.AssetIDs {
			if p.Asset(a) == nil {
				return fmt.Errorf("topic %q uses unknown asset %q", t.ID, a)
			}
		}
		if t.CTAID != "" && p.CTA(t.CTAID) == nil {
			return fmt.Errorf("topic %q uses unknown CTA", t.ID)
		}
	}
	return nil
}

func (p *Profile) Source(id string) *Source {
	for i := range p.Sources {
		if p.Sources[i].ID == id {
			return &p.Sources[i]
		}
	}
	return nil
}

func (p *Profile) Topic(id string) *Topic {
	for i := range p.Topics {
		if p.Topics[i].ID == id {
			return &p.Topics[i]
		}
	}
	return nil
}

func (p *Profile) Claim(id string) *Claim {
	for i := range p.Claims {
		if p.Claims[i].ID == id {
			return &p.Claims[i]
		}
	}
	return nil
}

func (p *Profile) CTA(id string) *CTA {
	for i := range p.CTAs {
		if p.CTAs[i].ID == id {
			return &p.CTAs[i]
		}
	}
	return nil
}

func (p *Profile) Asset(id string) *Asset {
	for i := range p.Assets {
		if p.Assets[i].ID == id {
			return &p.Assets[i]
		}
	}
	return nil
}

func (p *Profile) AssetBySlug(slug string) *Asset {
	for i := range p.Assets {
		if p.Assets[i].Slug == slug {
			return &p.Assets[i]
		}
	}
	return nil
}

// Prohibited returns the first prohibited phrase found as a whole word or phrase in text
// (01 §15.1 claims discipline), case-insensitive; "" when the text is clean.
func (p *Profile) Prohibited(text string) string {
	for _, w := range p.ProhibitedClaims {
		re := regexp.MustCompile(`(?i)(^|[^\pL\pN])` + regexp.QuoteMeta(w) + `($|[^\pL\pN])`)
		if re.MatchString(text) {
			return w
		}
	}
	return ""
}

// FieldLimits are the fields a guide page has and their lengths.
var FieldLimits = map[string]int{"title": 70, "description": 160, "headline": 120, "cta_text": 40, "post": 500}
