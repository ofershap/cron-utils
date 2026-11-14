package cronutils

import (
	"strings"
	"testing"
	"time"
)

func TestDescribeGolden(t *testing.T) {
	cases := map[string]string{
		"* * * * *":       "Every minute",
		"*/5 * * * *":     "Every 5 minutes",
		"0 9 * * 1-5":     "At 09:00 AM, Monday through Friday",
		"0 22 * * 1-5":    "At 10:00 PM, Monday through Friday",
		"30 14 * * *":     "At 02:30 PM",
		"0 0 * * 0":       "At 12:00 AM, only on Sunday",
		"0 12 * * *":      "At 12:00 PM",
		"0 0 1 * *":       "At 12:00 AM, on day 1 of the month",
		"0 9 1,15 * *":    "At 09:00 AM, on days 1, 15 of the month",
		"0 9 * JAN *":     "At 09:00 AM, only in January",
		"0 9 * JAN-MAR *": "At 09:00 AM, January through March",
		"5 * * * *":       "At 5 minutes past the hour",
		"0 */2 * * *":     "At 0 minutes past the hour, every 2 hours",
		"0 9 L * *":       "At 09:00 AM, on the last day of the month",
		"0 9 LW * *":      "At 09:00 AM, on the last weekday of the month",
		"0 9 15W * *":     "At 09:00 AM, on the nearest weekday to day 15 of the month",
		"0 9 * * FRIL":    "At 09:00 AM, on the last Friday of the month",
		"0 9 * * FRI#3":   "At 09:00 AM, on the third Friday of the month",
		"@hourly":         "At 0 minutes past the hour",
		"@daily":          "At 12:00 AM",
		"@weekly":         "At 12:00 AM, only on Sunday",
		"@monthly":        "At 12:00 AM, on day 1 of the month",
		"@yearly":         "At 12:00 AM, on day 1 of the month, only in January",
		"0 9 * * 7":       "At 09:00 AM, only on Sunday",
		"*/10 * * * * *":  "Every 10 seconds",
		"30 0 9 * * *":    "At 09:00:30 AM",
	}
	for expr, want := range cases {
		got, err := Describe(expr)
		if err != nil {
			t.Errorf("%q: %v", expr, err)
			continue
		}
		if got != want {
			t.Errorf("%q: got %q, want %q", expr, got, want)
		}
	}
}

func TestParseErrorsNeverPanic(t *testing.T) {
	bad := []string{
		"", "  ", "* * * *", "* * * * * * * *", "abc", "0 9 * *", "@reboot",
		"@sometimes", "61 * * * *", "* 25 * * *", "0 0 32 * *", "0 0 * 13 *",
		"0 0 * * 8", "*/0 * * * *", "5-1 * * * *", "1/2/3 * * * *",
		"0 9 99W * *", "0 9 * * FRI#9", "0 9 * * QL", "- * * * *",
		",,,", "0 9 * FOO *", "0 9 * * MON-", "0 9 * * -FRI",
	}
	for _, expr := range bad {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("%q panicked: %v", expr, r)
				}
			}()
			_, err := Parse(expr)
			if err == nil {
				t.Errorf("%q: expected error", expr)
				return
			}
			pe, ok := err.(*ParseError)
			if !ok {
				t.Errorf("%q: error %T, want *ParseError", expr, err)
				return
			}
			if pe.Field == "" || pe.Msg == "" {
				t.Errorf("%q: incomplete ParseError %+v", expr, pe)
			}
		}()
	}
}

func TestParseErrorMentionsField(t *testing.T) {
	_, err := Parse("0 25 * * *")
	if err == nil || !strings.Contains(err.Error(), "hours") {
		t.Fatalf("want hours named, got %v", err)
	}
}

func at(s string) time.Time {
	t, err := time.ParseInLocation("2006-01-02 15:04:05", s, time.UTC)
	if err != nil {
		panic(err)
	}
	return t
}

func TestNextGolden(t *testing.T) {
	cases := []struct{ expr, from, want string }{
		{"* * * * *", "2026-11-14 09:30:00", "2026-11-14 09:31:00"},
		{"0 9 * * 1-5", "2026-11-13 09:00:00", "2026-11-16 09:00:00"}, // Friday -> Monday
		{"0 0 29 2 *", "2026-11-14 00:00:00", "2028-02-29 00:00:00"},  // leap day
		{"0 9 L * *", "2026-02-01 00:00:00", "2026-02-28 09:00:00"},
		{"0 9 L * *", "2026-11-01 00:00:00", "2026-11-30 09:00:00"},
		{"0 9 LW * *", "2026-11-01 00:00:00", "2026-11-30 09:00:00"},  // Nov 30 2026 is Monday
		{"0 9 LW * *", "2026-05-01 00:00:00", "2026-05-29 09:00:00"},  // May 31 is Sunday
		{"0 9 15W * *", "2026-08-01 00:00:00", "2026-08-14 09:00:00"}, // Aug 15 2026 Saturday
		{"0 9 1W * *", "2026-08-01 00:00:00", "2026-08-03 09:00:00"},  // Aug 1 Saturday -> Mon 3rd
		{"0 9 * * FRI#3", "2026-11-01 00:00:00", "2026-11-20 09:00:00"},
		{"0 9 * * FRIL", "2026-11-01 00:00:00", "2026-11-27 09:00:00"},
		{"*/15 * * * * *", "2026-11-14 09:30:07", "2026-11-14 09:30:15"},
		{"0 30 9 * * *", "2026-11-14 08:30:29", "2026-11-14 09:30:00"},
		{"@daily", "2026-11-14 09:30:00", "2026-11-15 00:00:00"},
	}
	for _, c := range cases {
		s, err := Parse(c.expr)
		if err != nil {
			t.Errorf("%q: %v", c.expr, err)
			continue
		}
		got, ok := s.Next(at(c.from))
		if !ok {
			t.Errorf("%q: no fire time", c.expr)
			continue
		}
		if !got.Equal(at(c.want)) {
			t.Errorf("%q from %s: got %s, want %s", c.expr, c.from, got.Format("2006-01-02 15:04:05"), c.want)
		}
	}
}

func TestPrev(t *testing.T) {
	s, err := Parse("0 9 * * 1-5")
	if err != nil {
		t.Fatal(err)
	}
	got, ok := s.Prev(at("2026-11-16 08:00:00")) // Monday morning
	if !ok || !got.Equal(at("2026-11-13 09:00:00")) {
		t.Fatalf("got %v ok=%v", got, ok)
	}
}

// Vixie rule: both day fields restricted means EITHER matches.
func TestVixieOR(t *testing.T) {
	s, err := Parse("0 0 1 * 5")
	if err != nil {
		t.Fatal(err)
	}
	// Nov 2026: the 1st is a Sunday; next fire is Friday Nov 6.
	got, _ := s.Next(at("2026-11-01 00:00:01"))
	if !got.Equal(at("2026-11-06 00:00:00")) {
		t.Fatalf("Vixie OR: got %s", got)
	}
}

// When one field is ? the other governs alone (Quartz style).
func TestQuestionMark(t *testing.T) {
	s, err := Parse("0 0 ? * 1")
	if err != nil {
		t.Fatal(err)
	}
	got, _ := s.Next(at("2026-11-14 00:00:00")) // Saturday
	if !got.Equal(at("2026-11-16 00:00:00")) {
		t.Fatalf("got %s", got)
	}
}

func TestNextN(t *testing.T) {
	s, err := Parse("0 9 * * 1-5")
	if err != nil {
		t.Fatal(err)
	}
	got := s.NextN(at("2026-11-13 10:00:00"), 3)
	if len(got) != 3 || !got[0].Equal(at("2026-11-16 09:00:00")) || !got[2].Equal(at("2026-11-18 09:00:00")) {
		t.Fatalf("got %v", got)
	}
}

func TestNamesCaseInsensitive(t *testing.T) {
	got, err := Describe("0 9 * jan mon")
	if err != nil {
		t.Fatal(err)
	}
	if got != "At 09:00 AM, only in January, only on Monday" {
		t.Fatalf("got %q", got)
	}
}
