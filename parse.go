package cronutils

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// ParseError identifies exactly what is wrong with an expression. Agents can
// feed it back to an LLM verbatim; Parse and Describe never panic.
type ParseError struct {
	Field string // "seconds", "minutes", "hours", "day of month", "month", "day of week", or "expression"
	Value string // the offending field text
	Msg   string
}

func (e *ParseError) Error() string {
	if e.Field == "expression" {
		return "cron: " + e.Msg
	}
	return fmt.Sprintf("cron: %s field %q: %s", e.Field, e.Value, e.Msg)
}

type kind int

const (
	kindAny kind = iota
	kindSingle
	kindRange
	kindStep // */n or a-b/n
	kindList
)

// fieldInfo is a parsed simple field (seconds, minutes, hours, month).
// any means the values cover the whole range (a seek/describe shortcut);
// star follows robfig/cron's starBit: the field text had a '*' item with
// step 1, which is what decides the day-of-month/day-of-week OR rule.
type fieldInfo struct {
	kind   kind
	vals   []int // sorted, deduped
	step   int
	lo, hi int
	any    bool
	star   bool
}

func (f fieldInfo) has(v int) bool {
	if f.any {
		return true
	}
	i := sort.SearchInts(f.vals, v)
	return i < len(f.vals) && f.vals[i] == v
}

// domSpec covers the day-of-month field including Quartz specials.
// star follows Vixie: true when the field text starts with '*' or is '?',
// which is what decides the OR rule; value matching still applies for
// stepped fields like */2.
type domSpec struct {
	star           bool
	days           []int
	last           bool // L
	lastWeekday    bool // LW
	nearestWeekday int  // nW, 0 when unused
}

// dowSpec covers the day-of-week field including Quartz specials.
type dowSpec struct {
	star    bool
	dows    []int // 0 = Sunday
	lastDow int   // xL, -1 when unused
	nthDow  int   // x#n: weekday
	nthN    int   // x#n: n
}

// Schedule is a parsed cron expression.
type Schedule struct {
	Expr       string
	HasSeconds bool
	Seconds    fieldInfo
	Minutes    fieldInfo
	Hours      fieldInfo
	Months     fieldInfo
	Dom        domSpec
	Dow        dowSpec
}

var monthNames = map[string]int{
	"JAN": 1, "FEB": 2, "MAR": 3, "APR": 4, "MAY": 5, "JUN": 6,
	"JUL": 7, "AUG": 8, "SEP": 9, "OCT": 10, "NOV": 11, "DEC": 12,
}

var dowNames = map[string]int{
	"SUN": 0, "MON": 1, "TUE": 2, "WED": 3, "THU": 4, "FRI": 5, "SAT": 6,
}

var macros = map[string]string{
	"@YEARLY":   "0 0 1 1 *",
	"@ANNUALLY": "0 0 1 1 *",
	"@MONTHLY":  "0 0 1 * *",
	"@WEEKLY":   "0 0 * * 0",
	"@DAILY":    "0 0 * * *",
	"@MIDNIGHT": "0 0 * * *",
	"@HOURLY":   "0 * * * *",
}

// Parse parses a 5-field (minute hour day-of-month month day-of-week) or
// 6-field (leading seconds) cron expression. It accepts the standard @macros,
// month and weekday names, and the Quartz specials L, LW, nW and x#n.
// Day-of-week 0 and 7 are both Sunday. The result never shares state with the
// input and Parse never panics, whatever the expression.
func Parse(expr string) (s *Schedule, err error) {
	defer func() {
		if r := recover(); r != nil {
			s, err = nil, &ParseError{Field: "expression", Msg: fmt.Sprintf("invalid expression (%v)", r)}
		}
	}()
	orig := expr
	expr = strings.Join(strings.Fields(strings.TrimSpace(expr)), " ")
	if expr == "" {
		return nil, &ParseError{Field: "expression", Msg: "empty expression"}
	}
	if strings.HasPrefix(expr, "@") {
		up := strings.ToUpper(expr)
		if up == "@REBOOT" {
			return nil, &ParseError{Field: "expression", Value: expr, Msg: "@reboot fires at daemon startup; it has no schedule"}
		}
		m, ok := macros[up]
		if !ok {
			return nil, &ParseError{Field: "expression", Value: expr, Msg: "unknown macro (want @hourly, @daily, @weekly, @monthly, @yearly)"}
		}
		expr = m
	}
	parts := strings.Split(expr, " ")
	s = &Schedule{Expr: orig}
	var sec, min, hour, dom, mon, dow string
	switch len(parts) {
	case 5:
		min, hour, dom, mon, dow = parts[0], parts[1], parts[2], parts[3], parts[4]
		s.Seconds = fieldInfo{kind: kindSingle, vals: []int{0}}
	case 6:
		s.HasSeconds = true
		sec, min, hour, dom, mon, dow = parts[0], parts[1], parts[2], parts[3], parts[4], parts[5]
	default:
		return nil, &ParseError{Field: "expression", Value: orig, Msg: fmt.Sprintf("got %d fields, want 5 (minute hour day-of-month month day-of-week) or 6 (with leading seconds)", len(parts))}
	}
	if s.HasSeconds {
		if s.Seconds, err = parseSimple(sec, "seconds", 0, 59, nil); err != nil {
			return nil, err
		}
	}
	if s.Minutes, err = parseSimple(min, "minutes", 0, 59, nil); err != nil {
		return nil, err
	}
	if s.Hours, err = parseSimple(hour, "hours", 0, 23, nil); err != nil {
		return nil, err
	}
	if s.Dom, err = parseDOM(dom); err != nil {
		return nil, err
	}
	if s.Months, err = parseSimple(mon, "month", 1, 12, monthNames); err != nil {
		return nil, err
	}
	if s.Dow, err = parseDOW(dow); err != nil {
		return nil, err
	}
	return s, nil
}

func parseSimple(text, field string, min, max int, names map[string]int) (fieldInfo, error) {
	bad := func(msg string) (fieldInfo, error) {
		return fieldInfo{}, &ParseError{Field: field, Value: text, Msg: msg}
	}
	if text == "*" || text == "?" {
		return fieldInfo{kind: kindAny, any: true, star: true}, nil
	}
	if text == "" {
		return bad("empty field")
	}
	items := strings.Split(text, ",")
	var vals []int
	var firstStep, firstLo, firstHi int
	var star bool
	kindSeen := kindSingle
	for _, item := range items {
		lo, hi, step, err := parseItem(item, field, text, min, max, names)
		if err != nil {
			return fieldInfo{}, err
		}
		if strings.HasPrefix(item, "*") && step == 1 {
			star = true
		}
		if len(items) == 1 {
			if step > 1 {
				kindSeen = kindStep
				firstStep, firstLo, firstHi = step, lo, hi
			} else if lo == min && hi == max && strings.HasPrefix(item, "*") {
				kindSeen = kindAny
			} else if lo != hi {
				kindSeen = kindRange
				firstLo, firstHi = lo, hi
			}
		} else {
			kindSeen = kindList
		}
		for v := lo; v <= hi; v += step {
			vals = append(vals, v)
		}
	}
	if len(vals) == 0 {
		return bad("no values")
	}
	sort.Ints(vals)
	vals = dedupe(vals)
	if len(vals) == max-min+1 {
		return fieldInfo{kind: kindAny, any: true, vals: vals, star: star}, nil
	}
	fi := fieldInfo{kind: kindSeen, vals: vals, step: firstStep, lo: firstLo, hi: firstHi, star: star}
	if kindSeen == kindList || (kindSeen == kindSingle && len(vals) > 1) {
		fi.kind = kindList
	}
	return fi, nil
}

func parseItem(item, field, text string, min, max int, names map[string]int) (lo, hi, step int, err error) {
	bad := func(msg string) (int, int, int, error) {
		return 0, 0, 0, &ParseError{Field: field, Value: text, Msg: msg}
	}
	step = 1
	base := item
	if i := strings.Index(item, "/"); i >= 0 {
		base = item[:i]
		sv := item[i+1:]
		n, cerr := strconv.Atoi(sv)
		if cerr != nil || n < 1 {
			return bad(fmt.Sprintf("step %q is not a positive number", sv))
		}
		step = n
		if strings.Contains(sv, "/") {
			return bad("only one step allowed")
		}
	}
	if base == "*" || base == "" {
		return min, max, step, nil
	}
	if i := strings.Index(base, "-"); i >= 0 {
		a, errA := valueOf(base[:i], names)
		b, errB := valueOf(base[i+1:], names)
		if errA != nil || errB != nil {
			return bad("range ends must be numbers or names")
		}
		if a > b {
			return bad(fmt.Sprintf("range %d-%d wraps; ranges must go upward", a, b))
		}
		if a < min || b > max {
			return bad(fmt.Sprintf("range %d-%d is outside %d-%d", a, b, min, max))
		}
		return a, b, step, nil
	}
	v, verr := valueOf(base, names)
	if verr != nil {
		return bad(fmt.Sprintf("%q is not a number or name", base))
	}
	if v < min || v > max {
		return bad(fmt.Sprintf("%d is outside %d-%d", v, min, max))
	}
	// "n/s" means n-max/s in classic cron.
	if strings.Contains(item, "/") {
		return v, max, step, nil
	}
	return v, v, step, nil
}

func valueOf(tok string, names map[string]int) (int, error) {
	if names != nil {
		if v, ok := names[strings.ToUpper(tok)]; ok {
			return v, nil
		}
	}
	return strconv.Atoi(tok)
}

func parseDOM(text string) (domSpec, error) {
	bad := func(msg string) (domSpec, error) {
		return domSpec{}, &ParseError{Field: "day of month", Value: text, Msg: msg}
	}
	up := strings.ToUpper(text)
	switch {
	case up == "*" || up == "?":
		return domSpec{star: true}, nil
	case up == "L":
		return domSpec{last: true}, nil
	case up == "LW":
		return domSpec{lastWeekday: true}, nil
	case strings.HasSuffix(up, "W"):
		n, err := strconv.Atoi(up[:len(up)-1])
		if err != nil || n < 1 || n > 31 {
			return bad("nW wants a day 1-31, e.g. 15W")
		}
		return domSpec{nearestWeekday: n}, nil
	}
	fi, err := parseSimple(text, "day of month", 1, 31, nil)
	if err != nil {
		return domSpec{}, err
	}
	return domSpec{star: fi.star, days: fi.vals}, nil
}

func parseDOW(text string) (dowSpec, error) {
	bad := func(msg string) (dowSpec, error) {
		return dowSpec{}, &ParseError{Field: "day of week", Value: text, Msg: msg}
	}
	up := strings.ToUpper(text)
	if up == "*" || up == "?" {
		return dowSpec{star: true, lastDow: -1}, nil
	}
	if i := strings.Index(up, "#"); i >= 0 {
		d, errD := valueOf(up[:i], dowNames)
		n, errN := strconv.Atoi(up[i+1:])
		if errD != nil || d < 0 || d > 7 || errN != nil || n < 1 || n > 5 {
			return bad("x#n wants a weekday and n 1-5, e.g. FRI#3")
		}
		return dowSpec{nthDow: normDow(d), nthN: n, lastDow: -1}, nil
	}
	if strings.HasSuffix(up, "L") && up != "L" {
		d, err := valueOf(up[:len(up)-1], dowNames)
		if err != nil || d < 0 || d > 7 {
			return bad("xL wants a weekday, e.g. FRIL for the last Friday")
		}
		return dowSpec{lastDow: normDow(d)}, nil
	}
	fi, err := parseSimple(text, "day of week", 0, 7, dowNames)
	if err != nil {
		return dowSpec{}, err
	}
	vals := fi.vals[:0]
	for _, v := range fi.vals {
		vals = append(vals, normDow(v))
	}
	return dowSpec{star: fi.star, dows: dedupe(sorted(vals)), lastDow: -1}, nil
}

func normDow(v int) int {
	if v == 7 {
		return 0
	}
	return v
}

func sorted(v []int) []int { sort.Ints(v); return v }

func dedupe(v []int) []int {
	out := v[:0]
	prev := -1
	for _, x := range v {
		if x != prev {
			out = append(out, x)
			prev = x
		}
	}
	return out
}
