package growth

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"
)

// Drafts runs the included SEO and marketing workers against one approved brief. They
// propose changes through the same path as anyone else and can never approve or apply.
func (s *Service) Drafts(ctx context.Context, briefID string) ([]*Action, error) {
	s.mu.Lock()
	b := s.briefs[briefID]
	if b == nil {
		s.mu.Unlock()
		return nil, notFound()
	}
	if b.Status != "approved" {
		s.mu.Unlock()
		return nil, conflict("brief_not_active", "Drafts are made from an approved brief.")
	}
	topic := s.prof.Topic(b.str("topic_id"))
	claims := map[string]string{}
	for _, id := range b.strs("approved_claim_ids") {
		if c := s.prof.Claim(id); c != nil {
			claims[id] = c.Text
		}
	}
	cta := s.prof.CTA(b.str("cta_id"))
	var evidence []string
	ev := b.strs("evidence_ids")
	for _, o := range s.obs {
		if slices.Contains(ev, o.ID) && len(evidence) < 5 {
			evidence = append(evidence, Sanitise(o.Term, 200))
		}
	}
	sort.Strings(evidence)
	var target *Page
	var asset *Asset
	for _, id := range topic.AssetIDs {
		if p := s.pages[id]; p != nil {
			target, asset = p, s.prof.Asset(id)
			break
		}
	}
	briefIDStr, version, channels := b.str("brief_id"), b.version(), b.strs("allowed_channels")
	s.mu.Unlock()
	if target == nil {
		return nil, conflict("page_not_created", "Create this topic's guide page first.")
	}
	ctaText := topic.Label
	if cta != nil {
		ctaText = cta.Text
	}
	type job struct{ purpose, kind, channel string }
	var jobs []job
	if slices.Contains(channels, "seo") && slices.Contains(asset.AllowedFields, "title") {
		jobs = append(jobs, job{"seo", "seo_metadata_patch", "seo"})
	}
	if slices.Contains(channels, "owned_placement") && slices.Contains(asset.AllowedFields, "headline") {
		jobs = append(jobs, job{"marketing", "owned_content_patch", "owned_placement"})
	}
	if slices.Contains(channels, "organic_social") {
		jobs = append(jobs, job{"marketing", "organic_social_draft", "organic_social"})
	}
	var out []*Action
	for _, j := range jobs {
		d := s.Gen.Generate(ctx, s.prof, DraftRequest{Purpose: j.purpose, Topic: topic.Label, Audience: topic.Audience,
			Intent: topic.Intent, Claims: claims, CTAText: ctaText, Evidence: evidence, Locale: b.str("locale")})
		patch := map[string]string{}
		if j.kind == "organic_social_draft" {
			patch["post"] = strings.TrimSpace(d.Fields["headline"] + " " + d.Fields["cta_text"])
		} else {
			for k, v := range d.Fields {
				if slices.Contains(asset.AllowedFields, k) {
					patch[k] = v
				}
			}
		}
		var ids []string
		for _, c := range d.ClaimIDs {
			if c != "" {
				ids = append(ids, c)
			}
		}
		a, _, err := s.Propose(ctx, Proposal{BriefID: briefIDStr, BriefVersion: version, Kind: j.kind, Channel: j.channel,
			TargetID: asset.ID, ExpectedRevision: target.Active, Patch: patch, EvidenceIDs: ev, ClaimIDs: ids},
			"agent:"+j.purpose, map[string]any{"route": d.Route, "model": d.Model, "degraded": d.Degraded, "notes": d.Notes})
		if err != nil {
			return out, fmt.Errorf("%s draft: %w", j.kind, err)
		}
		out = append(out, a)
	}
	return out, nil
}
