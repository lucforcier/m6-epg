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

func TestM6WeekRef(t *testing.T) {
	cases := []struct {
		date       time.Time
		wantYear   int
		wantNumber int
	}{
		{time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC), 2026, 1},
		{time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC), 2026, 1},
		{time.Date(2026, 1, 3, 12, 0, 0, 0, time.UTC), 2026, 2},
		{time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC), 2026, 41},
		{time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC), 2026, 42},
		{time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC), 2026, 43},
	}

	for _, tc := range cases {
		y, w := m6WeekRef(m6WeekStart(tc.date))
		if y != tc.wantYear || w != tc.wantNumber {
			t.Errorf("%s: got %d-%02d, want %d-%02d",
				tc.date.Format("2006-01-02"), y, w, tc.wantYear, tc.wantNumber)
		}
	}
}
