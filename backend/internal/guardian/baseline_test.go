package guardian

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

type vectors struct {
	Vectors []struct {
		Text   string   `json:"text"`
		Expect []string `json:"expect"`
	} `json:"vectors"`
}

// TestLabelledVectors is the release gate for the baseline: every harmful vector fires its
// signal (recall 1.0 per category) and no benign vector fires anything (no false positives).
func TestLabelledVectors(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "shared", "contracts", "vocab", "guardian_vectors.json"))
	if err != nil {
		t.Fatal(err)
	}
	var v vectors
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	harmful, benign := map[string]int{}, 0
	for _, c := range v.Vectors {
		var got []string
		for _, s := range Check(c.Text) {
			got = append(got, s.Kind)
		}
		for _, want := range c.Expect {
			harmful[want]++
			if !slices.Contains(got, want) {
				t.Errorf("missed %s: %q (got %v)", want, c.Text, got)
			}
		}
		if len(c.Expect) == 0 {
			benign++
			if len(got) > 0 {
				t.Errorf("false positive %v: %q", got, c.Text)
			}
		}
	}
	for _, k := range []string{"fraud", "child_safety", "extortion"} {
		if harmful[k] < 6 {
			t.Errorf("too few %s vectors: %d", k, harmful[k])
		}
	}
	if benign < 15 {
		t.Errorf("too few benign vectors: %d", benign)
	}
}

func TestActions(t *testing.T) {
	if s := Check("send me pics of your daughter"); len(s) == 0 || s[0].Action != "freeze_urgent" {
		t.Fatal(s)
	}
	if s := Check("pay me or I'll tell your wife"); len(s) == 0 || s[0].Action != "freeze_support" {
		t.Fatal(s)
	}
	if s := Check("buy me steam cards"); len(s) == 0 || s[0].Action != "warn_recipient" {
		t.Fatal(s)
	}
}
