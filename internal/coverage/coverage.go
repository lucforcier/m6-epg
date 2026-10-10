package coverage

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/lucforcier/m6-epg/internal/scraper/m6pro"
	"github.com/lucforcier/m6-epg/internal/store/sqlite"
)

type WeekFetcher func(context.Context, *http.Client, *time.Location, int, int) ([]m6pro.Programme, error)

// EnsureCoverage extends SQLite coverage through horizon. M6 weeks are fetched
// only when they are not already present in the database.
func EnsureCoverage(
	ctx context.Context,
	store *sqlite.Store,
	client *http.Client,
	location *time.Location,
	now time.Time,
	horizon time.Duration,
	fetch WeekFetcher,
) error {
	return ensureCoverage(ctx, store, client, location, now, horizon, fetch, false)
}

// RefreshCoverage refreshes all M6 weeks needed through horizon, replacing any
// versions already present in SQLite.
func RefreshCoverage(
	ctx context.Context,
	store *sqlite.Store,
	client *http.Client,
	location *time.Location,
	now time.Time,
	horizon time.Duration,
	fetch WeekFetcher,
) error {
	return ensureCoverage(ctx, store, client, location, now, horizon, fetch, true)
}

// EnsureW9Coverage extends W9 coverage, fetching only weeks absent from SQLite.
func EnsureW9Coverage(
	ctx context.Context,
	store *sqlite.Store,
	client *http.Client,
	location *time.Location,
	now time.Time,
	horizon time.Duration,
	fetch WeekFetcher,
) error {
	return ensureW9Coverage(ctx, store, client, location, now, horizon, fetch, false)
}

// RefreshW9Coverage refreshes every W9 week needed through horizon.
func RefreshW9Coverage(
	ctx context.Context,
	store *sqlite.Store,
	client *http.Client,
	location *time.Location,
	now time.Time,
	horizon time.Duration,
	fetch WeekFetcher,
) error {
	return ensureW9Coverage(ctx, store, client, location, now, horizon, fetch, true)
}

func ensureW9Coverage(
	ctx context.Context,
	store *sqlite.Store,
	client *http.Client,
	location *time.Location,
	now time.Time,
	horizon time.Duration,
	fetch WeekFetcher,
	refreshExisting bool,
) error {
	if store == nil {
		return fmt.Errorf("store is required")
	}
	if location == nil {
		return fmt.Errorf("location is required")
	}
	if horizon < 0 {
		return fmt.Errorf("horizon must not be negative")
	}
	if fetch == nil {
		fetch = func(ctx context.Context, client *http.Client, location *time.Location, year, week int) ([]m6pro.Programme, error) {
			return m6pro.FetchWeekFor(ctx, client, location, "w9", year, week)
		}
	}
	if client == nil {
		client = http.DefaultClient
	}

	for _, ref := range m6pro.WeeksForRange(now, now.Add(horizon)) {
		if !refreshExisting {
			present, err := store.HasW9Week(ref.Year, ref.Number)
			if err != nil {
				return err
			}
			if present {
				continue
			}
		}
		programmes, err := fetch(ctx, client, location, ref.Year, ref.Number)
		if err != nil {
			if isUnpublishedFutureWeek(err, ref, now) {
				log.Printf("m6-epg: W9 source week %04d-%02d is not published yet; retaining stored data and retrying on next refresh", ref.Year, ref.Number)
				continue
			}
			return fmt.Errorf("fetch W9 week %04d-%02d: %w", ref.Year, ref.Number, err)
		}
		if err := store.ReplaceW9Week(ref.Year, ref.Number, programmes); err != nil {
			return fmt.Errorf("store W9 week %04d-%02d: %w", ref.Year, ref.Number, err)
		}
	}
	return nil
}

func ensureCoverage(
	ctx context.Context,
	store *sqlite.Store,
	client *http.Client,
	location *time.Location,
	now time.Time,
	horizon time.Duration,
	fetch WeekFetcher,
	refreshExisting bool,
) error {
	if store == nil {
		return fmt.Errorf("store is required")
	}
	if location == nil {
		return fmt.Errorf("location is required")
	}
	if horizon < 0 {
		return fmt.Errorf("horizon must not be negative")
	}
	if fetch == nil {
		fetch = m6pro.FetchWeek
	}
	if client == nil {
		client = http.DefaultClient
	}

	end := now.Add(horizon)
	for _, ref := range m6pro.WeeksForRange(now, end) {
		if !refreshExisting {
			present, err := store.HasWeek(ref.Year, ref.Number)
			if err != nil {
				return err
			}
			if present {
				continue
			}
		}

		programmes, err := fetch(ctx, client, location, ref.Year, ref.Number)
		if err != nil {
			if isUnpublishedFutureWeek(err, ref, now) {
				log.Printf("m6-epg: M6 source week %04d-%02d is not published yet; retaining stored data and retrying on next refresh", ref.Year, ref.Number)
				continue
			}
			return fmt.Errorf("fetch M6 week %04d-%02d: %w", ref.Year, ref.Number, err)
		}
		if err := store.ReplaceWeek(ref.Year, ref.Number, programmes); err != nil {
			return fmt.Errorf("store M6 week %04d-%02d: %w", ref.Year, ref.Number, err)
		}
	}
	return nil
}


// isUnpublishedFutureWeek recognizes a 404 only when the requested source week
// is later than the current Paris source week. Other errors, including a 404
// for the current week, remain fatal. Skipped weeks are retried next refresh.
func isUnpublishedFutureWeek(err error, ref m6pro.WeekRef, now time.Time) bool {
	var statusErr *m6pro.HTTPStatusError
	if !errors.As(err, &statusErr) || statusErr.Code != http.StatusNotFound {
		return false
	}
	current := m6pro.WeeksForRange(now, now)
	if len(current) == 0 {
		return false
	}
	return ref.Year > current[0].Year ||
		(ref.Year == current[0].Year && ref.Number > current[0].Number)
}
