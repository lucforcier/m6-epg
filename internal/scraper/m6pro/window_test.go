package m6pro

import (
	"testing"
	"time"
)

func TestWeeksForRange(t *testing.T) {
	loc := time.FixedZone("CEST", 2*60*60)
	start := time.Date(2026, 10, 6, 12, 0, 0, 0, loc)
	end := time.Date(2026, 10, 20, 12, 0, 0, 0, loc)

	got := WeeksForRange(start, end)
	want := []WeekRef{{2026, 41}, {2026, 42}, {2026, 43}}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got[i], want[i])
		}
	}
}
