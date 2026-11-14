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
	// 2026-03-08 02:30 does not exist in New York (clocks jump 02:00->03:00).
	from := time.Date(2026, 3, 7, 12, 0, 0, 0, loc)
	got, ok := s.Next(from)
	if !ok {
		t.Fatal("no fire time")
	}
	want := time.Date(2026, 3, 8, 3, 30, 0, 0, loc) // normalized forward
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
