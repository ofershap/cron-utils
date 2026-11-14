package cronutils

import "testing"

// The sparse-schedule case: Feb 29 only, from three years out.
func BenchmarkNextFeb29(b *testing.B) {
	s, err := Parse("0 0 29 2 *")
	if err != nil {
		b.Fatal(err)
	}
	from := at("2026-11-14 00:00:00")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, ok := s.Next(from); !ok {
			b.Fatal("no fire time")
		}
	}
}

func BenchmarkNextEveryMinute(b *testing.B) {
	s, _ := Parse("* * * * *")
	from := at("2026-11-14 00:00:00")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Next(from)
	}
}
