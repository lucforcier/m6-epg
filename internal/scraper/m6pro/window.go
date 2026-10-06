package m6pro

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

type Week struct {
	Year int
	Week int
}

func WeeksForRange(start, end time.Time) []Week {
	start, end = start.UTC(), end.UTC()
	if end.Before(start) { return nil }
	first, last := monday(start), monday(end)
	var weeks []Week
	for d := first; !d.After(last); d = d.AddDate(0, 0, 7) {
		y, w := d.ISOWeek()
		weeks = append(weeks, Week{Year: y, Week: w})
	}
	return weeks
}

func FetchRange(ctx context.Context, client *http.Client, location *time.Location, start, end time.Time) (map[Week][]Programme, error) {
	if location == nil { return nil, fmt.Errorf("location is required") }
	result := make(map[Week][]Programme)
	for _, week := range WeeksForRange(start, end) {
		programmes, err := FetchWeek(ctx, client, location, week.Year, week.Week)
		if err != nil { return nil, err }
		}
		result[week] = programmes
	}
	return result, nil
}

func monday(t time.Time) time.Time {
	t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	weekday := int(t.Weekday())
	if weekday == 0 { weekday = 7 }
	return t.AddDate(0, 0, -(weekday - 1))
}
