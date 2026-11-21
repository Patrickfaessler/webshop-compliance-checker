package fixes

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// ParseColor reads #rgb, #rrggbb, rgb(r, g, b) and rgba(r, g, b, 1). Colours with alpha < 1 are not opaque
// and return ok=false.
func ParseColor(s string) (r, g, b uint8, ok bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	if strings.HasPrefix(s, "#") {
		h := s[1:]
		if len(h) == 3 {
			h = string([]byte{h[0], h[0], h[1], h[1], h[2], h[2]})
		}
		if len(h) != 6 {
			return 0, 0, 0, false
		}
		v, err := strconv.ParseUint(h, 16, 32)
		if err != nil {
			return 0, 0, 0, false
		}
		return uint8(v >> 16), uint8(v >> 8), uint8(v), true
	}
	var inner string
	switch {
	case strings.HasPrefix(s, "rgba(") && strings.HasSuffix(s, ")"):
		inner = s[5 : len(s)-1]
	case strings.HasPrefix(s, "rgb(") && strings.HasSuffix(s, ")"):
		inner = s[4 : len(s)-1]
	default:
		return 0, 0, 0, false
	}
	parts := strings.Split(inner, ",")
	if len(parts) != 3 && len(parts) != 4 {
		return 0, 0, 0, false
	}
	if len(parts) == 4 {
		if a, err := strconv.ParseFloat(strings.TrimSpace(parts[3]), 64); err != nil || a < 1 {
			return 0, 0, 0, false
		}
	}
	var c [3]uint8
	for i := 0; i < 3; i++ {
		v, err := strconv.Atoi(strings.TrimSpace(parts[i]))
		if err != nil || v < 0 || v > 255 {
			return 0, 0, 0, false
		}
		c[i] = uint8(v)
	}
	return c[0], c[1], c[2], true
}

func luminance(c [3]uint8) float64 {
	ch := func(v uint8) float64 {
		x := float64(v) / 255
		if x <= 0.03928 {
			return x / 12.92
		}
		return math.Pow((x+0.055)/1.055, 2.4)
	}
	return 0.2126*ch(c[0]) + 0.7152*ch(c[1]) + 0.0722*ch(c[2])
}

// Contrast is the WCAG contrast ratio of two colours.
func Contrast(fg, bg [3]uint8) float64 {
	a, b := luminance(fg), luminance(bg)
	if a < b {
		a, b = b, a
	}
	return (a + 0.05) / (b + 0.05)
}

func hex(c [3]uint8) string { return fmt.Sprintf("#%02x%02x%02x", c[0], c[1], c[2]) }

func toHSL(c [3]uint8) (h, s, l float64) {
	r, g, b := float64(c[0])/255, float64(c[1])/255, float64(c[2])/255
	mx, mn := math.Max(r, math.Max(g, b)), math.Min(r, math.Min(g, b))
	l = (mx + mn) / 2
	if mx == mn {
		return 0, 0, l
	}
	d := mx - mn
	if l > 0.5 {
		s = d / (2 - mx - mn)
	} else {
		s = d / (mx + mn)
	}
	switch mx {
	case r:
		h = math.Mod((g-b)/d+6, 6)
	case g:
		h = (b-r)/d + 2
	default:
		h = (r-g)/d + 4
	}
	return h / 6, s, l
}

func fromHSL(h, s, l float64) [3]uint8 {
	if s == 0 {
		v := uint8(math.Round(l * 255))
		return [3]uint8{v, v, v}
	}
	q := l * (1 + s)
	if l >= 0.5 {
		q = l + s - l*s
	}
	p := 2*l - q
	hue := func(t float64) uint8 {
		t = math.Mod(t+1, 1)
		var v float64
		switch {
		case t < 1.0/6:
			v = p + (q-p)*6*t
		case t < 0.5:
			v = q
		case t < 2.0/3:
			v = p + (q-p)*(2.0/3-t)*6
		default:
			v = p
		}
		return uint8(math.Round(math.Max(0, math.Min(1, v)) * 255))
	}
	return [3]uint8{hue(h + 1.0/3), hue(h), hue(h - 1.0/3)}
}

// SuggestColor returns the colour nearest to fg (same hue and saturation, lightness changed) that reaches
// min contrast on bg (A1-3). A passing fg is returned unchanged. ok is false when fg or bg is not an opaque colour.
func SuggestColor(fg, bg string, min float64) (string, float64, bool) {
	fr, fgc, fb, ok1 := ParseColor(fg)
	br, bgc, bb, ok2 := ParseColor(bg)
	if !ok1 || !ok2 {
		return "", 0, false
	}
	f, b := [3]uint8{fr, fgc, fb}, [3]uint8{br, bgc, bb}
	if c := Contrast(f, b); c >= min {
		return hex(f), c, true
	}
	h, s, l0 := toHSL(f)
	best, bestDist := [3]uint8{}, math.Inf(1)
	for _, end := range []float64{0, 1} {
		if Contrast(fromHSL(h, s, end), b) < min {
			continue
		}
		near, far := l0, end // near fails, far passes
		for i := 0; i < 40; i++ {
			mid := (near + far) / 2
			if Contrast(fromHSL(h, s, mid), b) >= min {
				far = mid
			} else {
				near = mid
			}
		}
		if d := math.Abs(far - l0); d < bestDist {
			best, bestDist = fromHSL(h, s, far), d
		}
	}
	if math.IsInf(bestDist, 1) || Contrast(best, b) < min { // rounding or no passing lightness: black or white
		best = [3]uint8{0, 0, 0}
		if Contrast([3]uint8{255, 255, 255}, b) > Contrast(best, b) {
			best = [3]uint8{255, 255, 255}
		}
	}
	return hex(best), Contrast(best, b), true
}
