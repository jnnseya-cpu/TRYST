package profile

import (
	"errors"
	"strings"
	"testing"
)

func me(score float64) Face { return Face{Match: []float64{score}} }

var clean = Checks{DetectorRan: true, SyntheticScore: 0.05, Faces: []Face{me(0.97)}}

func TestModerateRejectsAIAndBorrowedPhotos(t *testing.T) {
	cases := map[string]Checks{
		"declared AI":     {Provenance: Provenance{Synthetic: true}, DetectorRan: true, Faces: []Face{me(0.99)}},
		"detector":        {DetectorRan: true, SyntheticScore: 0.95, Faces: []Face{me(0.99)}},
		"found online":    {DetectorRan: true, ReverseImageHit: true, Faces: []Face{me(0.99)}},
		"someone else":    {DetectorRan: true, Faces: []Face{me(0.40)}},
		"third party too": {DetectorRan: true, Faces: []Face{me(0.98), me(0.10)}},
		"artwork":         {Provenance: Provenance{NotPhoto: true}, DetectorRan: true},
	}
	for name, c := range cases {
		if d := Moderate(c, 1, false); d.Outcome != Reject {
			t.Errorf("%s: %s", name, d.Outcome)
		}
	}
	if d := Moderate(Checks{CSAMHashHit: true}, 1, false); d.Outcome != Report {
		t.Fatalf("CSAM must be reported: %s", d.Outcome)
	}
}

func TestModerateMainPhotoRules(t *testing.T) {
	if d := Moderate(clean, 1, true); d.Outcome != Approve {
		t.Fatalf("clean main: %+v", d)
	}
	if d := Moderate(Checks{DetectorRan: true, SyntheticScore: 0.1}, 1, true); d.Outcome != Reject || !strings.Contains(d.Reason, "face") {
		t.Fatalf("faceless main: %+v", d)
	}
	if d := Moderate(Checks{DetectorRan: true, SyntheticScore: 0.1}, 1, false); d.Outcome != Approve {
		t.Fatalf("faceless extra photo (e.g. a view) is fine: %+v", d)
	}
	explicit := clean
	explicit.Explicit = true
	if d := Moderate(explicit, 1, true); d.Outcome != Reject {
		t.Fatalf("explicit main: %+v", d)
	}
	// Couple: main photo must show both partners.
	one := Checks{DetectorRan: true, Faces: []Face{{Match: []float64{0.97, 0.20}}}}
	if d := Moderate(one, 2, true); d.Outcome != Reject {
		t.Fatalf("couple main with one partner: %+v", d)
	}
	both := Checks{DetectorRan: true, Faces: []Face{{Match: []float64{0.97, 0.20}}, {Match: []float64{0.15, 0.95}}}}
	if d := Moderate(both, 2, true); d.Outcome != Approve {
		t.Fatalf("couple main with both: %+v", d)
	}
}

func TestModerateFailsClosedToReview(t *testing.T) {
	grey := clean
	grey.SyntheticScore = 0.7
	if d := Moderate(grey, 1, false); d.Outcome != Review {
		t.Fatalf("grey zone: %s", d.Outcome)
	}
	down := clean
	down.DetectorRan = false
	if d := Moderate(down, 1, true); d.Outcome != Review {
		t.Fatalf("detector down must not auto-approve: %s", d.Outcome)
	}
}

func TestGalleryRules(t *testing.T) {
	main := Photo{ID: "m", Main: true, Outcome: Approve, Revealed: true}
	var eleven []Photo
	for i := 0; i < 11; i++ {
		eleven = append(eleven, Photo{ID: string(rune('a' + i)), Outcome: Approve})
	}
	eleven[0] = main
	if err := ValidateGallery(eleven); !errors.Is(err, ErrTooMany) {
		t.Fatalf("11 photos: %v", err)
	}
	if err := ValidateGallery(eleven[:10]); err != nil {
		t.Fatalf("10 photos: %v", err)
	}
	if err := ValidateGallery([]Photo{{ID: "x", Outcome: Approve, Revealed: true}}); !errors.Is(err, ErrNoMain) {
		t.Fatalf("no main: %v", err)
	}
	if err := ValidateGallery([]Photo{main, main}); !errors.Is(err, ErrTwoMain) {
		t.Fatalf("two mains: %v", err)
	}
	if err := ValidateGallery([]Photo{{ID: "m", Main: true, Outcome: Approve}}); !errors.Is(err, ErrMainHidden) {
		t.Fatalf("blurred main: %v", err)
	}
	if err := ValidateGallery([]Photo{{ID: "m", Main: true, Outcome: Review, Revealed: true}}); !errors.Is(err, ErrMainPending) {
		t.Fatalf("pending main: %v", err)
	}
	v := Visible([]Photo{{ID: "x", Outcome: Approve}, {ID: "r", Outcome: Review}, main, {ID: "j", Outcome: Reject}})
	if len(v) != 2 || v[0].ID != "m" {
		t.Fatalf("visible: %+v", v)
	}
}
