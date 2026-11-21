package lexicon

import "testing"

// Spec: R2-1, R1-1, R5-1 — German and English lists are complete; other languages have none yet.
func TestListsComplete(t *testing.T) {
	for _, lang := range []string{"de", "en"} {
		l, ok := For(lang)
		if !ok {
			t.Fatalf("%s: no list", lang)
		}
		if len(l.Sections) != 6 {
			t.Errorf("%s: %d sections, want 6", lang, len(l.Sections))
		}
		for _, k := range []Kind{LegalNotice, Privacy, Terms, Withdrawal} {
			if len(l.Links[k]) == 0 {
				t.Errorf("%s: no link words for %s", lang, k)
			}
		}
		if len(l.Withdrawal) == 0 || len(l.Periods) == 0 || len(l.EmailLabels) == 0 || len(l.AddressLabels) == 0 || len(l.NameLabels) == 0 {
			t.Errorf("%s: a word list is empty: %+v", lang, l)
		}
	}
	if _, ok := For("fr"); ok {
		t.Error("fr should have no list yet")
	}
}

// Spec: R1-1 — lexicon hits count distinct list words found in the text.
func TestHits(t *testing.T) {
	de, _ := For("de")
	if n := Hits(de, "Impressum Inhaber: Max Muster Anschrift: Hauptstraße 1"); n < 3 {
		t.Errorf("de hits = %d, want ≥ 3", n)
	}
	en, _ := For("en")
	if n := Hits(en, "Impressum Inhaber Anschrift"); n != 0 {
		t.Errorf("en hits on German text = %d, want 0", n)
	}
}
