package cronutils

// Differential tests: compare fire times against two established engines,
// gorhill/cronexpr (Quartz-style, seconds field, L/W/# specials) and
// robfig/cron v3 (standard 5-field Vixie). The generator only produces the
// documented common subset: day-of-week 0-6 (cron-utils also accepts 7 for
// Sunday, a deliberate dialect difference pinned in TestSundaySeven), and no
// year field. Both engines stop searching after ~5-6 years, so only common
// schedules are generated.

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/gorhill/cronexpr"
	robfig "github.com/robfig/cron/v3"
)

func genSimple(r *rand.Rand, lo, hi int, names []string) string {
	pick := func(v int) string {
		if names != nil && r.Intn(3) == 0 {
			return names[v-lo]
		}
		return fmt.Sprint(v)
	}
	switch r.Intn(6) {
	case 0:
		return "*"
	case 1:
		maxStep := hi - lo
		if maxStep > 10 {
			maxStep = 10
		}
		return fmt.Sprintf("*/%d", 1+r.Intn(maxStep))
	case 2:
		a := lo + r.Intn(hi-lo)
		b := a + r.Intn(hi-a)
		return fmt.Sprintf("%s-%s", pick(a), pick(b))
	case 3:
		n := 2 + r.Intn(3)
		s := ""
		for i := 0; i < n; i++ {
			if i > 0 {
				s += ","
			}
			s += pick(lo + r.Intn(hi-lo+1))
		}
		return s
	case 4:
		a := lo + r.Intn(hi-lo)
		b := a + r.Intn(hi-a)
		maxStep := b - a + 1
		if maxStep > 5 {
			maxStep = 5
		}
		return fmt.Sprintf("%d-%d/%d", a, b, 1+r.Intn(maxStep))
	default:
		return pick(lo + r.Intn(hi-lo+1))
	}
}

var monNames = []string{"JAN", "FEB", "MAR", "APR", "MAY", "JUN", "JUL", "AUG", "SEP", "OCT", "NOV", "DEC"}
var dowNamesArr = []string{"SUN", "MON", "TUE", "WED", "THU", "FRI", "SAT"}

func genDOM(r *rand.Rand, specials bool) string {
	if specials {
		switch r.Intn(8) {
		case 0:
			return "L"
		case 1:
			return "LW"
		case 2:
			return fmt.Sprintf("%dW", 1+r.Intn(28))
		case 3:
			return "?"
		}
	}
	return genSimple(r, 1, 28, nil) // 28: every month has one
}

func genDOW(r *rand.Rand, specials bool) string {
	if specials {
		switch r.Intn(8) {
		case 0:
			return fmt.Sprintf("%dL", r.Intn(7))
		case 1:
			return fmt.Sprintf("%sL", dowNamesArr[r.Intn(7)])
		case 2:
			return fmt.Sprintf("%d#%d", r.Intn(7), 1+r.Intn(5))
		case 3:
			return "?"
		}
	}
	return genSimple(r, 0, 6, dowNamesArr)
}

func compareNexts(t *testing.T, expr string, ours *Schedule, other func(time.Time) time.Time, label string) {
	starts := []time.Time{
		at("2026-11-14 09:30:00"),
		at("2027-02-28 23:59:58"),
		at("2028-02-29 12:00:00"), // leap day
	}
	for _, start := range starts {
		mine, otherT := start, start
		for i := 0; i < 4; i++ {
			gotMine, okMine := ours.Next(mine)
			gotOther := other(otherT)
			okOther := !gotOther.IsZero()
			if okMine != okOther {
				t.Fatalf("%s %q from %s step %d: cron-utils ok=%v, %s ok=%v",
					label, expr, start, i, okMine, label, okOther)
			}
			if !okMine {
				break
			}
			if !gotMine.Equal(gotOther) {
				t.Fatalf("%s %q from %s step %d: cron-utils %s, %s %s",
					label, expr, start, i, gotMine, label, gotOther)
			}
			mine, otherT = gotMine, gotOther
		}
	}
}

func TestDifferentialCronexpr(t *testing.T) {
	r := rand.New(rand.NewSource(20261114))
	for i := 0; i < 300; i++ {
		specials := r.Intn(2) == 0
		dom, dow := genDOM(r, specials), genDOW(r, specials)
		// cronexpr applies the OR rule to any field whose text is not
		// literally "*"; Vixie and robfig treat star-prefixed steps like
		// "*/2" as unrestricted. Keep the compared subset where the three
		// engines agree: no stepped day field while the other is restricted.
		// cronexpr applies the OR rule to any field whose text is not
		// literally "*"; robfig (and cron-utils) also treat "*/1" as
		// unrestricted. Keep the compared subset where the engines agree:
		// no full-coverage "*/1" in a day field while the other is
		// restricted.
		fullStep := func(f string) bool { return f == "*/1" }
		if fullStep(dom) && dow != "*" && dow != "?" {
			dow = "?"
		}
		if fullStep(dow) && dom != "*" && dom != "?" {
			dom = "?"
		}
		expr := fmt.Sprintf("%s %s %s %s %s %s",
			genSimple(r, 0, 59, nil), genSimple(r, 0, 59, nil), genSimple(r, 0, 23, nil),
			dom, genSimple(r, 1, 12, monNames), dow)
		ours, err := Parse(expr)
		if err != nil {
			t.Fatalf("cron-utils rejected %q: %v", expr, err)
		}
		// cronexpr takes a year field after day-of-week when seconds lead.
		theirs, err := cronexpr.Parse(expr + " *")
		if err != nil {
			t.Fatalf("cronexpr rejected %q: %v", expr+" *", err)
		}
		compareNexts(t, expr, ours, theirs.Next, "cronexpr")
	}
}

func TestDifferentialRobfig(t *testing.T) {
	r := rand.New(rand.NewSource(42))
	for i := 0; i < 300; i++ {
		expr := fmt.Sprintf("%s %s %s %s %s",
			genSimple(r, 0, 59, nil), genSimple(r, 0, 23, nil),
			genDOM(r, false), genSimple(r, 1, 12, monNames), genDOW(r, false))
		ours, err := Parse(expr)
		if err != nil {
			t.Fatalf("cron-utils rejected %q: %v", expr, err)
		}
		theirs, err := robfig.ParseStandard(expr)
		if err != nil {
			t.Fatalf("robfig rejected %q: %v", expr, err)
		}
		compareNexts(t, expr, ours, theirs.Next, "robfig")
	}
}

// Dialect pin: 7 means Sunday (Vixie). cronexpr accepts 7 too; robfig's
// standard parser rejects it (bounds 0-6), which is why the differential
// generators stay inside 0-6.
func TestSundaySeven(t *testing.T) {
	s, err := Parse("0 9 * * 7")
	if err != nil {
		t.Fatal(err)
	}
	got, _ := s.Next(at("2026-11-14 00:00:00")) // Saturday
	if !got.Equal(at("2026-11-15 09:00:00")) {
		t.Fatalf("7 should mean Sunday, got %s", got)
	}
	theirs, err := cronexpr.Parse("0 0 9 * * 7 *")
	if err != nil {
		t.Fatalf("cronexpr 7-as-Sunday changed: %v", err)
	}
	if !theirs.Next(at("2026-11-14 00:00:00")).Equal(at("2026-11-15 09:00:00")) {
		t.Fatal("cronexpr disagrees on 7 = Sunday")
	}
}

// Impossible schedule: February 31st never fires, and both engines must say so.
func TestImpossibleSchedule(t *testing.T) {
	s, err := Parse("0 0 31 2 *")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Next(at("2026-11-14 00:00:00")); ok {
		t.Fatal("Feb 31 should never fire")
	}
	if _, ok := s.Prev(at("2026-11-14 00:00:00")); ok {
		t.Fatal("Feb 31 should never have fired")
	}
	theirs, err := cronexpr.Parse("0 0 0 31 2 * *")
	if err == nil && !theirs.Next(at("2026-11-14 00:00:00")).IsZero() {
		t.Fatal("cronexpr disagrees: Feb 31 fired")
	}
}
