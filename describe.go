package cronutils

import (
	"fmt"
	"strings"
)

var monthLong = []string{"", "January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"}
var dowLong = []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}

// Describe parses expr and returns an English description in the style of
// cronstrue: "0 9 * * 1-5" becomes "At 09:00 AM, Monday through Friday".
func Describe(expr string) (string, error) {
	s, err := Parse(expr)
	if err != nil {
		return "", err
	}
	return s.Describe(), nil
}

// Describe renders the schedule in English, cronstrue-style: the time of day
// first, then day-of-month, month, and day-of-week restrictions.
func (s *Schedule) Describe() string {
	return capitalize(strings.Join(nonEmpty(s.timePart(), s.domPart(), s.monthPart(), s.dowPart()), ", "))
}

func nonEmpty(parts ...string) []string {
	out := parts[:0]
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func (s *Schedule) timePart() string {
	sec := ""
	if s.HasSeconds {
		switch {
		case s.Seconds.any:
			if s.Minutes.any && s.Hours.any {
				return "Every second"
			}
			sec = "every second, "
		case s.Seconds.kind == kindStep:
			if s.Minutes.any && s.Hours.any {
				return fmt.Sprintf("every %d seconds", s.Seconds.step)
			}
			sec = fmt.Sprintf("every %d seconds, ", s.Seconds.step)
		case len(s.Seconds.vals) == 1 && s.Seconds.vals[0] != 0:
			sec = fmt.Sprintf("at %d seconds past the minute, ", s.Seconds.vals[0])
		}
	}
	min, hour := s.Minutes, s.Hours
	switch {
	case min.any && hour.any:
		return sec + "every minute"
	case min.kind == kindStep && hour.any:
		return sec + fmt.Sprintf("every %d minutes", min.step)
	case min.any && hour.kind == kindStep:
		return sec + fmt.Sprintf("every minute, every %d hours", hour.step)
	case len(min.vals) == 1 && hour.any:
		return sec + fmt.Sprintf("at %d minutes past the hour", min.vals[0])
	case len(min.vals) == 1 && hour.kind == kindStep:
		return sec + fmt.Sprintf("at %d minutes past the hour, every %d hours", min.vals[0], hour.step)
	case len(min.vals) == 1 && len(hour.vals) == 1:
		if s.HasSeconds && len(s.Seconds.vals) == 1 && s.Seconds.vals[0] != 0 {
			return "at " + clockSec(hour.vals[0], min.vals[0], s.Seconds.vals[0])
		}
		return sec + "at " + clock(hour.vals[0], min.vals[0])
	case len(min.vals) == 1 && hour.kind == kindRange:
		return sec + fmt.Sprintf("at %d minutes past the hour, between %s and %s", min.vals[0], clock(hour.lo, 0), clock(hour.hi, 0))
	case len(min.vals) == 1 && len(hour.vals) > 1:
		times := make([]string, len(hour.vals))
		for i, h := range hour.vals {
			times[i] = clock(h, min.vals[0])
		}
		return sec + "at " + joinAnd(times)
	default:
		return sec + genericTime(min, hour)
	}
}

func genericTime(min, hour fieldInfo) string {
	parts := []string{}
	if !min.any {
		parts = append(parts, fmt.Sprintf("at minutes %s past the hour", joinInts(min.vals)))
	}
	if !hour.any {
		parts = append(parts, fmt.Sprintf("past hours %s", joinInts(hour.vals)))
	}
	if len(parts) == 0 {
		return "every minute"
	}
	return strings.Join(parts, ", ")
}

// clockSec renders 9:00:30 as "09:00:30 AM".
func clockSec(h, m, sec int) string {
	c := clock(h, m)
	return c[:5] + fmt.Sprintf(":%02d", sec) + c[5:]
}

// clock renders 14:30 as "02:30 PM".
func clock(h, m int) string {
	ampm := "AM"
	switch {
	case h == 0:
		h = 12
	case h == 12:
		ampm = "PM"
	case h > 12:
		h -= 12
		ampm = "PM"
	}
	return fmt.Sprintf("%02d:%02d %s", h, m, ampm)
}

func (s *Schedule) domPart() string {
	d := s.Dom
	switch {
	case d.star:
		return ""
	case d.last:
		return "on the last day of the month"
	case d.lastWeekday:
		return "on the last weekday of the month"
	case d.nearestWeekday > 0:
		return fmt.Sprintf("on the nearest weekday to day %d of the month", d.nearestWeekday)
	case len(d.days) == 1:
		return fmt.Sprintf("on day %d of the month", d.days[0])
	default:
		return fmt.Sprintf("on days %s of the month", joinInts(d.days))
	}
}

func (s *Schedule) monthPart() string {
	m := s.Months
	if m.any {
		return ""
	}
	if m.kind == kindRange {
		return fmt.Sprintf("%s through %s", monthLong[m.lo], monthLong[m.hi])
	}
	names := make([]string, len(m.vals))
	for i, v := range m.vals {
		names[i] = monthLong[v]
	}
	return "only in " + joinAnd(names)
}

func (s *Schedule) dowPart() string {
	d := s.Dow
	switch {
	case d.star:
		return ""
	case d.lastDow >= 0:
		return fmt.Sprintf("on the last %s of the month", dowLong[d.lastDow])
	case d.nthN > 0:
		return fmt.Sprintf("on the %s %s of the month", ordinal(d.nthN), dowLong[d.nthDow])
	case isRange(d.dows, 1, 5):
		return "Monday through Friday"
	case len(d.dows) == 1:
		return "only on " + dowLong[d.dows[0]]
	case isRange(d.dows, d.dows[0], d.dows[len(d.dows)-1]):
		return fmt.Sprintf("%s through %s", dowLong[d.dows[0]], dowLong[d.dows[len(d.dows)-1]])
	default:
		names := make([]string, len(d.dows))
		for i, v := range d.dows {
			names[i] = dowLong[v]
		}
		return "only on " + joinAnd(names)
	}
}

func isRange(vals []int, lo, hi int) bool {
	if len(vals) != hi-lo+1 {
		return false
	}
	for i, v := range vals {
		if v != lo+i {
			return false
		}
	}
	return true
}

func ordinal(n int) string {
	switch n {
	case 1:
		return "first"
	case 2:
		return "second"
	case 3:
		return "third"
	case 4:
		return "fourth"
	default:
		return "fifth"
	}
}

func joinInts(vals []int) string {
	parts := make([]string, len(vals))
	for i, v := range vals {
		parts[i] = fmt.Sprint(v)
	}
	return strings.Join(parts, ", ")
}

func joinAnd(parts []string) string {
	switch len(parts) {
	case 1:
		return parts[0]
	case 2:
		return parts[0] + " and " + parts[1]
	default:
		return strings.Join(parts[:len(parts)-1], ", ") + " and " + parts[len(parts)-1]
	}
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
