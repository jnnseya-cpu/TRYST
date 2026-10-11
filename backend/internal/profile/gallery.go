package profile

import (
	"errors"
	"fmt"
)

// Profile photo rules (FR-092, FR-093; D-38).
//
// What other members see: one main photo that clearly shows the member's face (for a
// couple, both partners), their age in years, their area and a distance band, and up to
// MaxPhotos photos in total. Every photo must be a real photograph of the member: no
// AI-generated or AI-edited images, no other people, no images found elsewhere online.

const (
	MaxPhotos = 10

	// FaceMatchMin is the 1:1 match score (provider-calibrated, 0..1) a face must reach
	// against the member's verified reference (taken at V1/V2 liveness).
	FaceMatchMin = 0.90
	// SyntheticReject and SyntheticReview are synthetic-image detector thresholds.
	SyntheticReject = 0.90
	SyntheticReview = 0.50
)

// Face is one face found in a photo, with its match score against each owner.
// A solo profile has one owner; a couple profile has two.
type Face struct {
	Match []float64 // score against owner[0], owner[1]...
}

// Checks are the results of the upload pipeline for one photo (03 §7.5). The pipeline
// runs before the photo can be seen by anyone.
type Checks struct {
	Provenance      Provenance
	SyntheticScore  float64 // synthetic-image detector, 0..1
	DetectorRan     bool    // false if the detector was unavailable: fail closed to review
	ReverseImageHit bool    // found online (stock, social, celebrity, other accounts)
	CSAMHashHit     bool    // known-CSAM hash list match
	Faces           []Face
	Explicit        bool // nudity classifier
}

// Outcome is the moderation decision for one photo.
type Outcome string

const (
	Approve Outcome = "approve"
	Review  Outcome = "review" // hidden until a person checks it
	Reject  Outcome = "reject"
	Report  Outcome = "reject_and_report" // CSAM: preserve and report (03 §7.3)
)

// Decision explains an outcome in words the member is shown (except for Report).
type Decision struct {
	Outcome Outcome
	Reason  string
}

// Moderate decides one photo. owners is 1 for a solo profile, 2 for a couple.
func Moderate(c Checks, owners int, main bool) Decision {
	switch {
	case c.CSAMHashHit:
		return Decision{Report, "removed"}
	case c.Provenance.Synthetic:
		return Decision{Reject, "This image is marked as AI-generated or AI-edited. Please upload a real photo of yourself."}
	case c.Provenance.NotPhoto:
		return Decision{Reject, "Please upload a photograph, not artwork or a screenshot."}
	case c.ReverseImageHit:
		return Decision{Reject, "This photo appears elsewhere online. Please upload a photo of yourself that you took."}
	case c.DetectorRan && c.SyntheticScore >= SyntheticReject:
		return Decision{Reject, "This image looks AI-generated. Please upload a real photo of yourself."}
	}
	// Every face must be one of the owners: no third parties, no borrowed photos.
	matched := make([]bool, owners)
	for _, f := range c.Faces {
		ok := false
		for i := 0; i < owners && i < len(f.Match); i++ {
			if f.Match[i] >= FaceMatchMin {
				matched[i], ok = true, true
			}
		}
		if !ok {
			return Decision{Reject, "Someone in this photo isn't you. Only photos of you (and your partner, for a couple) are allowed."}
		}
	}
	if main {
		if c.Explicit {
			return Decision{Reject, "Your main photo can't be explicit."}
		}
		for i, m := range matched {
			if !m {
				if owners == 2 {
					return Decision{Reject, fmt.Sprintf("Your main photo must clearly show both of you (partner %d not recognised).", i+1)}
				}
				return Decision{Reject, "Your main photo must clearly show your face."}
			}
		}
	}
	if !c.DetectorRan || c.SyntheticScore >= SyntheticReview {
		return Decision{Review, "We're taking a closer look at this photo. It will appear once checked."}
	}
	return Decision{Approve, ""}
}

// Photo is one gallery entry.
type Photo struct {
	ID       string
	Main     bool
	Outcome  Outcome
	Revealed bool // false: blurred until the member grants a reveal (reveal ladder, FR-008)
}

var (
	ErrTooMany     = fmt.Errorf("a profile can show at most %d photos", MaxPhotos)
	ErrNoMain      = errors.New("a profile needs one main photo")
	ErrTwoMain     = errors.New("only one photo can be the main photo")
	ErrMainHidden  = errors.New("the main photo is always shown to members who can see the profile")
	ErrMainPending = errors.New("the main photo must be approved before the profile is visible")
)

// ValidateGallery checks a gallery before the profile can be published.
func ValidateGallery(photos []Photo) error {
	if len(photos) > MaxPhotos {
		return ErrTooMany
	}
	mains := 0
	for _, p := range photos {
		if !p.Main {
			continue
		}
		mains++
		if !p.Revealed {
			return ErrMainHidden
		}
		if p.Outcome != Approve {
			return ErrMainPending
		}
	}
	switch {
	case mains == 0:
		return ErrNoMain
	case mains > 1:
		return ErrTwoMain
	}
	return nil
}

// Visible returns the photos another member is shown, main photo first: approved photos
// only, at most MaxPhotos.
func Visible(photos []Photo) []Photo {
	var out []Photo
	for _, p := range photos {
		if p.Main && p.Outcome == Approve {
			out = append([]Photo{p}, out...)
		} else if p.Outcome == Approve {
			out = append(out, p)
		}
	}
	if len(out) > MaxPhotos {
		out = out[:MaxPhotos]
	}
	return out
}
