package cronutils

import (
	"strings"
	"testing"
)

// FuzzParse hammers the parser with arbitrary bytes: it must always return
// either a Schedule or a *ParseError, and never panic. It also round-trips:
// whatever parses must describe and seek without panicking.
func FuzzParse(f *testing.F) {
	seeds := []string{
		"* * * * *", "0 9 * * 1-5", "*/5 * * * *", "0 0 29 2 *", "0 9 L * *",
		"0 9 LW * *", "0 9 15W * *", "0 9 * * FRI#3", "0 9 * * 5L", "@daily",
		"0 9 * JAN-MAR MON-FRI", "*/10 * * * * *", "0 0 ? * 1", "61 * * * *",
		"a,b-c/d e", "0 9 * * 7", "0 0 31 2 *", strings.Repeat("9", 1000),
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, expr string) {
		if len(expr) > 4096 {
			t.Skip("bound input size")
		}
		s, err := Parse(expr)
		if err != nil {
			if pe, ok := err.(*ParseError); !ok || pe.Msg == "" {
				t.Fatalf("%q: non-ParseError or empty message: %T %v", expr, err, err)
			}
			return
		}
		_ = s.Describe()
		_, _ = s.Next(at("2026-11-14 09:00:00"))
		_, _ = s.Prev(at("2026-11-14 09:00:00"))
	})
}
