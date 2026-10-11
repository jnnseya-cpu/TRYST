package profile

import "testing"

func kinds(fs []Finding) map[string]string {
	m := map[string]string{}
	for _, f := range fs {
		m[f.Kind] = f.Action
	}
	return m
}

func TestScreenTextBlocks(t *testing.T) {
	cases := map[string]string{
		"Text me 07700 900 123":                      "contact_details",
		"call +44 7700-900-123":                      "contact_details",
		"me at jane (at) mail (dot) com":             "contact_details",
		"find me on snap: janedoe_92":                "contact_details",
		"insta @jane.d":                              "contact_details",
		"www.myplace.xyz":                            "contact_details",
		"send to paypal.me/jane":                     "payment_details",
		"Cash App $janedoe":                          "payment_details",
		"bc1qar0srrr7xfkvy5l643lydnw9re59gtzzwf5mdq": "payment_details",
		"incall only, 150 roses":                     "commercial_sex",
		"£200 per hour":                              "commercial_sex",
		"generous sugar daddy wanted":                "commercial_sex",
		"selling tina, dm for menu":                  "drug_supply",
		"i'm 16 but mature":                          "minor_reference",
		"17yo looking for fun":                       "minor_reference",
		"schoolgirl roleplay":                        "minor_reference",
		"barely l3gal":                               "minor_reference",
	}
	for text, want := range cases {
		got := kinds(ScreenText(text))
		if _, ok := got[want]; !ok {
			t.Errorf("%q: want %s, got %v", text, want, got)
		}
	}
	if got := kinds(ScreenText("i'm 16 but mature")); got["minor_reference"] != "urgent" {
		t.Fatal("minor references must be urgent")
	}
}

func TestScreenTextAllowsOrdinaryProfiles(t *testing.T) {
	ok := []string{
		"Married, 42, curious about meeting another couple. Coffee first, always.",
		"We're 38 and 41, together 12 years, open relationship. Wine, galleries, slow evenings.",
		"Discreet, kind, love a long walk on the Heath. Not into games.",
		"Gym 5 days a week, 6ft, non-smoker.",
		"I'm 5'9\" and I rate honesty above everything.",
		"Fifteen years married, both 40-something.",
		"I'm 15 minutes from the city centre.",
		"Tribute act fan; the donation box at the gallery gets my change.",
		"We take harm reduction seriously and never mix alcohol with anything.",
	}
	for _, s := range ok {
		if f := ScreenText(s); len(f) != 0 {
			t.Errorf("%q flagged: %v", s, f)
		}
	}
}
