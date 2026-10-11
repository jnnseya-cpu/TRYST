package profile

import (
	"fmt"
	"math"
	"math/rand"
	"testing"
)

func TestEncodeKnownPoint(t *testing.T) {
	// Big Ben, London.
	if got := Encode(51.5007, -0.1246, 5); got != "gcpuv" {
		t.Fatalf("geohash: %s", got)
	}
}

func TestDistanceLabelsAreHonest(t *testing.T) {
	islington := Encode(51.5362, -0.1033, 5)
	if got, _ := DistanceBand(islington, islington); got != "Nearby" {
		t.Fatalf("same cell: %q", got)
	}
	cases := []struct {
		name      string
		lat, lon  float64
		trueKm    float64
		tolerance float64
	}{
		{"Brixton", 51.4613, -0.1156, 8.4, 7.5},
		{"Lewisham", 51.4545, 0.0090, 11.7, 7.5},
		{"Oxford", 51.7520, -1.2577, 83, 12},
	}
	for _, c := range cases {
		got, err := DistanceBand(islington, Encode(c.lat, c.lon, 5))
		var n float64
		if _, scanErr := fmt.Sscanf(got, "About %g km away", &n); err != nil || scanErr != nil {
			t.Fatalf("%s: %q %v", c.name, got, err)
		}
		if math.Abs(n-c.trueKm) > c.tolerance {
			t.Errorf("%s: shown %q, true %.1f km", c.name, got, c.trueKm)
		}
	}
	if _, err := DistanceBand("gcp!!", "gcpuv"); err == nil {
		t.Fatal("bad cell accepted")
	}
}

// Trilateration resistance: wherever a member stands inside their cell, every viewer sees
// exactly the same band, even if the client sends a finer cell than allowed.
func TestNoPrecisionBelowTheCell(t *testing.T) {
	rng := rand.New(rand.NewSource(3))
	member := Encode(51.5362, -0.1033, 5)
	la, lo, _ := centre(member)
	for v := 0; v < 200; v++ {
		viewer := Encode(51.3+rng.Float64()*0.5, -0.5+rng.Float64()*0.8, 5)
		want, _ := DistanceBand(viewer, member)
		for i := 0; i < 20; i++ {
			// a point somewhere else inside the same geohash-5 cell, sent at precision 9
			p := Encode(la+(rng.Float64()-0.5)*0.04, lo+(rng.Float64()-0.5)*0.04, 9)
			if p[:5] != member {
				continue
			}
			if got, _ := DistanceBand(viewer, p); got != want {
				t.Fatalf("band leaked position inside the cell: %q vs %q", got, want)
			}
		}
	}
}

func TestAreaLabel(t *testing.T) {
	names := map[string]string{"gcpvj": "Islington, London", "gcpv": "North London, London"}
	if got := AreaLabel("gcpvj", names, false); got != "Islington, London" {
		t.Fatal(got)
	}
	if got := AreaLabel("gcpvj", names, true); got != "London" {
		t.Fatal(got)
	}
	if got := AreaLabel("gcpvk", names, false); got != "North London, London" {
		t.Fatal(got)
	}
	if got := AreaLabel("u10hb", names, false); got != "Location hidden" {
		t.Fatal(got)
	}
}
