package fixes

import (
	"fmt"
	"math/rand"
	"testing"
)

// Spec: A1-3 — the suggested colour always reaches the required ratio (property test).
func TestSuggestColorReachesRatio(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	hex := func() string { return fmt.Sprintf("#%02x%02x%02x", rng.Intn(256), rng.Intn(256), rng.Intn(256)) }
	for i := 0; i < 2000; i++ {
		fg, bg := hex(), hex()
		for _, min := range []float64{4.5, 3.0} {
			got, ratio, ok := SuggestColor(fg, bg, min)
			if !ok {
				t.Fatalf("SuggestColor(%s, %s, %v) not ok", fg, bg, min)
			}
			r, g, b, _ := ParseColor(got)
			br, bgg, bb, _ := ParseColor(bg)
			if c := Contrast([3]uint8{r, g, b}, [3]uint8{br, bgg, bb}); c < min || ratio < min {
				t.Fatalf("SuggestColor(%s, %s, %v) = %s with contrast %.3f (reported %.3f)", fg, bg, min, got, c, ratio)
			}
		}
	}
}

// Spec: A1-3 — the suggestion stays close to the original colour.
func TestSuggestColorNearest(t *testing.T) {
	got, ratio, ok := SuggestColor("#aaaaaa", "#ffffff", 4.5)
	r, g, b, _ := ParseColor(got)
	if !ok || r != g || g != b || ratio < 4.5 || ratio >= 4.7 {
		t.Errorf("SuggestColor(#aaaaaa) = %s ratio %.2f", got, ratio)
	}
	if got, _, _ := SuggestColor("#767676", "#ffffff", 4.5); got != "#767676" {
		t.Errorf("passing colour changed to %s", got)
	}
	if _, _, ok := SuggestColor("rgba(0, 0, 0, 0)", "#ffffff", 4.5); ok {
		t.Error("transparent colour accepted")
	}
}
