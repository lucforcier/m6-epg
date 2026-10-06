type WeekRef struct {
	Year int
	Number int
}

package m6pro

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

func WeeksForRange(start, end time.Time) []WeekRef {
	start, end = start.UTC(), end.UTC()
	if end.Before(start) {
		return nil
	}
	first, last := monday(start), monday(end)
	var weeks []Week
	for d := first; !d.After(last); d = d.AddDate(0, 0, 7) {
		y, w := d.ISOWeek()
		weeks = append(weeks, WeekRef{Year: y, Number: w})
	}
	return weeks
}

func FetchRange(ctx context.Context, client *http.Client, location *time.Location, start, end time.Time) (map[WeekRef][]Programme, error) {
	if location == nil {
		return nil, fmt.Errorf("location is required")
	}
	result := make(map[Week][]Programme)
	for _, week := range WeeksForRange(start, end) {
		programmes, err := FetchWeek(ctx, client, location, week.Year, week.Number)
		if err != nil {
			return nil, err
		}
		result[week] = programmes
	}
	return result, nil
}

func monday(t time.Time) time.Time {
	t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	weekday := int(t.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	return t.AddDate(0, 0, -(weekday - 1))
}
