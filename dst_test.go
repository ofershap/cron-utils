package cronutils

import (
	"testing"
	"time"
)

// DST behavior is pinned, not derived: cron-utils computes fire times by
// adding the time-of-day to the local midnight, which is Go's time.Date
// normalization. A time that falls into the spring-forward gap lands on the
// first valid instant after it; a time in the fall fold fires once, at the
// first (daylight) occurrence.

func TestDSTGap(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip(err)
	}
	s, err := Parse("30 2 * * *")
	if err != nil {
		t.Fatal(err)
	}
	// 2026-03-08 02:30 does not exist in New York (clocks jump 02:00->03:00);
	// the first wall-clock time at or after it is 03:00.
	from := time.Date(2026, 3, 7, 12, 0, 0, 0, loc)
	got, ok := s.Next(from)
	if !ok {
		t.Fatal("no fire time")
	}
	want := time.Date(2026, 3, 8, 3, 0, 0, 0, loc) // first existing wall time at or after 02:30
	if !got.Equal(want) {
		t.Fatalf("gap: got %s, want %s", got, want)
	}
	// The day after is back to normal.
	got, _ = s.Next(got)
	if want := time.Date(2026, 3, 9, 2, 30, 0, 0, loc); !got.Equal(want) {
		t.Fatalf("after gap: got %s, want %s", got, want)
	}
}

func TestDSTFold(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip(err)
	}
	s, err := Parse("30 1 * * *")
	if err != nil {
		t.Fatal(err)
	}
	// 2026-11-01 01:30 happens twice; cron-utils fires once, at the first.
	from := time.Date(2026, 10, 31, 12, 0, 0, 0, loc)
	got, _ := s.Next(from)
	want := time.Date(2026, 11, 1, 1, 30, 0, 0, time.FixedZone("EDT", -4*3600))
	if !got.Equal(want) {
		t.Fatalf("fold: got %s, want first 01:30 (EDT)", got)
	}
	got, _ = s.Next(got)
	if want := time.Date(2026, 11, 2, 1, 30, 0, 0, loc); !got.Equal(want) {
		t.Fatalf("after fold: got %s, want %s", got, want)
	}
}

// Ordinary wall-clock times must not drift on transition days: 09:00 local
// fires at 09:00 local, before and after both transitions.
func TestDSTOrdinaryTimesDoNotDrift(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip(err)
	}
	s, err := Parse("0 9 * * *")
	if err != nil {
		t.Fatal(err)
	}
	from := time.Date(2026, 3, 6, 12, 0, 0, 0, loc)
	for _, wantDay := range []time.Time{
		time.Date(2026, 3, 7, 9, 0, 0, 0, loc),
		time.Date(2026, 3, 8, 9, 0, 0, 0, loc), // spring-forward day
		time.Date(2026, 3, 9, 9, 0, 0, 0, loc),
	} {
		got, _ := s.Next(from)
		if !got.Equal(wantDay) || got.Hour() != 9 {
			t.Fatalf("drift: got %s, want %s", got, wantDay)
		}
		from = got
	}
	from = time.Date(2026, 10, 31, 12, 0, 0, 0, loc)
	for _, wantDay := range []time.Time{
		time.Date(2026, 11, 1, 9, 0, 0, 0, loc), // fall-back day
		time.Date(2026, 11, 2, 9, 0, 0, 0, loc),
	} {
		got, _ := s.Next(from)
		if !got.Equal(wantDay) || got.Hour() != 9 {
			t.Fatalf("drift: got %s, want %s", got, wantDay)
		}
		from = got
	}
}

// Half-hour transition zone: Lord Howe Island shifts 30 minutes.
func TestDSTHalfHourZone(t *testing.T) {
	loc, err := time.LoadLocation("Australia/Lord_Howe")
	if err != nil {
		t.Skip(err)
	}
	s, err := Parse("30 2 * * *")
	if err != nil {
		t.Fatal(err)
	}
	// 2026-10-04 02:00 -> 02:30 there; 02:30 exists (first instant after gap).
	from := time.Date(2026, 10, 3, 12, 0, 0, 0, loc)
	got, _ := s.Next(from)
	if got.Hour() != 2 || got.Minute() != 30 {
		t.Fatalf("half-hour zone: got %s", got)
	}
}
