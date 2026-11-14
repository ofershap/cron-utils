package cronutils

import "time"

// Next returns the first time the schedule fires strictly after t, in t's
// location. It reports false when nothing fires within six years.
func (s *Schedule) Next(t time.Time) (time.Time, bool) {
	return s.seek(t, 1)
}

// Prev returns the most recent time the schedule fired strictly before t.
func (s *Schedule) Prev(t time.Time) (time.Time, bool) {
	return s.seek(t, -1)
}

// MustNext is Next that panics when nothing fires within six years; useful in
// tests and examples where the schedule is known to recur.
func (s *Schedule) MustNext(t time.Time) time.Time {
	n, ok := s.Next(t)
	if !ok {
		panic("cronutils: no fire time within six years")
	}
	return n
}

// NextN returns the next n fire times strictly after t. It stops early if the
// schedule stops firing within six years.
func (s *Schedule) NextN(t time.Time, n int) []time.Time {
	out := make([]time.Time, 0, n)
	for len(out) < n {
		nxt, ok := s.Next(t)
		if !ok {
			break
		}
		out = append(out, nxt)
		t = nxt
	}
	return out
}

func (s *Schedule) seek(t time.Time, dir int) (time.Time, bool) {
	t = t.Truncate(time.Second)
	secs, mins, hrs := s.Seconds.vals, s.Minutes.vals, s.Hours.vals
	if s.Seconds.any {
		secs = all(0, 59)
	}
	if s.Minutes.any {
		mins = all(0, 59)
	}
	if s.Hours.any {
		hrs = all(0, 23)
	}
	day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	for i := 0; i < 366*6; i++ {
		if s.dayMatches(day) {
			start := -1
			if i == 0 {
				start = t.Hour()*3600 + t.Minute()*60 + t.Second()
			}
			if tod, ok := pickTimeOfDay(secs, mins, hrs, start, dir); ok {
				return day.Add(time.Duration(tod) * time.Second), true
			}
		}
		day = day.AddDate(0, 0, dir)
	}
	return time.Time{}, false
}

// pickTimeOfDay finds the smallest (dir=1) or largest (dir=-1) second-of-day
// from the field sets that is strictly after (or before) start.
func pickTimeOfDay(secs, mins, hrs []int, start, dir int) (int, bool) {
	best := -1
	for _, h := range hrs {
		for _, m := range mins {
			for _, sc := range secs {
				tod := h*3600 + m*60 + sc
				if start >= 0 {
					if dir > 0 && tod <= start {
						continue
					}
					if dir < 0 && tod >= start {
						continue
					}
				}
				if best < 0 || (dir > 0 && tod < best) || (dir < 0 && tod > best) {
					best = tod
				}
			}
		}
	}
	return best, best >= 0
}

func all(lo, hi int) []int {
	out := make([]int, 0, hi-lo+1)
	for v := lo; v <= hi; v++ {
		out = append(out, v)
	}
	return out
}

// dayMatches applies the Vixie rule: when both day-of-month and day-of-week
// are restricted, a day matching either fires; otherwise both must match.
func (s *Schedule) dayMatches(d time.Time) bool {
	if !s.Months.has(int(d.Month())) {
		return false
	}
	domOK := s.Dom.matches(d)
	dowOK := s.Dow.matches(d)
	domRestricted := !s.Dom.any
	dowRestricted := !s.Dow.any
	if domRestricted && dowRestricted {
		return domOK || dowOK
	}
	return domOK && dowOK
}

func (s domSpec) matches(d time.Time) bool {
	if s.any {
		return true
	}
	day := d.Day()
	if s.last && day == lastDayOfMonth(d) {
		return true
	}
	if s.lastWeekday && day == lastWeekdayOfMonth(d) {
		return true
	}
	if s.nearestWeekday > 0 && day == nearestWeekday(d.Year(), d.Month(), s.nearestWeekday) {
		return true
	}
	for _, v := range s.days {
		if v == day {
			return true
		}
	}
	return false
}

func (s dowSpec) matches(d time.Time) bool {
	if s.any {
		return true
	}
	wd := int(d.Weekday())
	if s.lastDow >= 0 {
		return wd == s.lastDow && d.Day()+7 > lastDayOfMonth(d)
	}
	if s.nthN > 0 {
		return wd == s.nthDow && (d.Day()-1)/7+1 == s.nthN
	}
	for _, v := range s.dows {
		if v == wd {
			return true
		}
	}
	return false
}

func lastDayOfMonth(d time.Time) int {
	return time.Date(d.Year(), d.Month()+1, 0, 0, 0, 0, 0, d.Location()).Day()
}

func lastWeekdayOfMonth(d time.Time) int {
	last := time.Date(d.Year(), d.Month()+1, 0, 0, 0, 0, 0, d.Location())
	for last.Weekday() == time.Saturday || last.Weekday() == time.Sunday {
		last = last.AddDate(0, 0, -1)
	}
	return last.Day()
}

func nearestWeekday(y int, m time.Month, n int) int {
	last := time.Date(y, m+1, 0, 0, 0, 0, 0, time.UTC).Day()
	if n > last {
		n = last
	}
	d := time.Date(y, m, n, 0, 0, 0, 0, time.UTC)
	switch d.Weekday() {
	case time.Saturday:
		if n == 1 {
			return 3 // Saturday the 1st moves forward to Monday
		}
		return n - 1
	case time.Sunday:
		if n == last {
			return n - 2 // Sunday at month end moves back to Friday
		}
		return n + 1
	}
	return n
}
