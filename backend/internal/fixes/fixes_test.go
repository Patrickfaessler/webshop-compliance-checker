package fixes

import (
	"strings"
	"testing"
)

// Spec: A3-F, A1-F…A6-F — page values are HTML-escaped when they are filled into a template.
func TestFillEscapes(t *testing.T) {
	got := Fill(`<label for="{{id}}">{{t}}</label>`, map[string]string{"id": "plz", "t": `"><script>x</script>`})
	if strings.Contains(got, "<script>") || strings.Contains(got, `"><`) {
		t.Fatalf("not escaped: %s", got)
	}
	if want := `<label for="plz">&#34;&gt;&lt;script&gt;x&lt;/script&gt;</label>`; got != want {
		t.Errorf("Fill = %s, want %s", got, want)
	}
	if got := Fill("{{a}} {{unknown}}", map[string]string{"a": "x"}); got != "x {{unknown}}" {
		t.Errorf("unknown key: %s", got)
	}
}
