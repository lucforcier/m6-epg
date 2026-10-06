package m6pro

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

type WeekRef struct {
	Year   int
	Number int
}

func WeeksForRange(start, end time.Time) []WeekRef {
	start, end = start.UTC(), end.UTC()
	if end.Before(start) {
		return nil
	}

	first := m6WeekStart(start)
	last := m6WeekStart(end)
	var weeks []WeekRef
	for d := first; !d.After(last); d = d.AddDate(0, 0, 7) {
		y, w := m6WeekRef(d)
		weeks = append(weeks, WeekRef{Year: y, Number: w})
	}
	return weeks
}

func FetchRange(ctx context.Context, client *http.Client, location *time.Location, start, end time.Time) (map[WeekRef][]Programme, error) {
	if location == nil {
		return nil, fmt.Errorf("location is required")
	}
	result := make(map[WeekRef][]Programme)
	for _, week := range WeeksForRange(start, end) {
		programmes, err := FetchWeek(ctx, client, location, week.Year, week.Number)
		if err != nil {
			return nil, err
		}
		result[week] = programmes
	}
	return result, nil
}

// m6WeekStart returns the Saturday starting the M6 PRO week containing t.
func m6WeekStart(t time.Time) time.Time {
	t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	weekday := int(t.Weekday())
	daysSinceSaturday := (weekday - 6 + 7) % 7
	return t.AddDate(0, 0, -daysSinceSaturday)
}

// m6WeekRef maps an M6 week start to the year/number used by pro.m6.fr.
// The M6 week containing January 1 is week 1 of that calendar year.
func m6WeekRef(start time.Time) (int, int) {
	start = m6WeekStart(start)

	year := start.Year()
	jan1 := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
	first := m6WeekStart(jan1)

	if start.Before(jan1) {
		year = jan1.Year()
		first = start
	}

	weeks := int(start.Sub(first).Hours() / (24 * 7))
	return year, weeks + 1
}
