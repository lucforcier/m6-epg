package coverage

import (
	"context"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/lucforcier/m6-epg/internal/scraper/m6pro"
	"github.com/lucforcier/m6-epg/internal/store/sqlite"
)

func TestEnsureCoverageFetchesOnlyMissingWeeks(t *testing.T) {
	store, err := sqlite.Open(filepath.Join(t.TempDir(), "m6.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	seedWeek := func(year, week int) []m6pro.Programme {
		return []m6pro.Programme{{
			ProgramID:   "seed-" + string(rune('0'+week)),
			BroadcastID: "b",
			Start:       time.Date(year, 10, 10, 12, 0, 0, 0, time.UTC),
			Title:       "seed",
		}}
	}
	if err := store.ReplaceWeek(2026, 41, seedWeek(2026, 41)); err != nil {
		t.Fatal(err)
	}

	var fetched []m6pro.WeekRef
	fetch := func(ctx context.Context, client *http.Client, location *time.Location, year, week int) ([]m6pro.Programme, error) {
		fetched = append(fetched, m6pro.WeekRef{Year: year, Number: week})
		return []m6pro.Programme{{
			ProgramID:   "p-" + string(rune('0'+week)),
			BroadcastID: "b",
			Start:       time.Date(year, 10, 10, 12, 0, 0, 0, time.UTC),
			Title:       "programme",
		}}, nil
	}

	now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	if err := EnsureCoverage(context.Background(), store, nil, time.UTC, now, 14*24*time.Hour, fetch); err != nil {
		t.Fatal(err)
	}

	if len(fetched) != 2 {
		t.Fatalf("fetched %d weeks, want 2", len(fetched))
	}
	if fetched[0] != (m6pro.WeekRef{Year: 2026, Number: 42}) ||
		fetched[1] != (m6pro.WeekRef{Year: 2026, Number: 43}) {
		t.Fatalf("fetched weeks = %#v, want 2026-42 and 2026-43", fetched)
	}

	fetched = nil
	if err := EnsureCoverage(context.Background(), store, nil, time.UTC, now, 14*24*time.Hour, fetch); err != nil {
		t.Fatal(err)
	}
	if len(fetched) != 0 {
		t.Fatalf("fetched %d weeks on second run, want 0", len(fetched))
	}
}

func TestRefreshCoverageRefetchesExistingWeeks(t *testing.T) {
	store, err := sqlite.Open(filepath.Join(t.TempDir(), "m6.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	seed := []m6pro.Programme{{
		ProgramID:   "old",
		BroadcastID: "b",
		Start:       time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC),
		Title:       "old",
	}}
	if err := store.ReplaceWeek(2026, 41, seed); err != nil {
		t.Fatal(err)
	}

	var fetched []m6pro.WeekRef
	fetch := func(ctx context.Context, client *http.Client, location *time.Location, year, week int) ([]m6pro.Programme, error) {
		fetched = append(fetched, m6pro.WeekRef{Year: year, Number: week})
		return []m6pro.Programme{{
			ProgramID:   "new",
			BroadcastID: "b",
			Start:       time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC),
			Title:       "new",
		}}, nil
	}

	now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	if err := RefreshCoverage(context.Background(), store, nil, time.UTC, now, 0, fetch); err != nil {
		t.Fatal(err)
	}

	if len(fetched) != 1 || fetched[0] != (m6pro.WeekRef{Year: 2026, Number: 41}) {
		t.Fatalf("fetched weeks = %#v, want 2026-41", fetched)
	}

	programmes, err := store.ProgramsBetween(
		time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(programmes) != 1 || programmes[0].Title != "new" {
		t.Fatalf("stored programmes = %#v, want refreshed programme", programmes)
	}
}

func TestEnsureW9CoverageUsesIndependentW9Cache(t *testing.T) {
	store, err := sqlite.Open(filepath.Join(t.TempDir(), "m6.db"))
	if err != nil { t.Fatal(err) }
	defer store.Close()

	now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	seed := []m6pro.Programme{{ProgramID: "w9-seed", BroadcastID: "b", Start: now, Title: "W9 seed"}}
	if err := store.ReplaceW9Week(2026, 41, seed); err != nil { t.Fatal(err) }

	var fetched []m6pro.WeekRef
	fetch := func(ctx context.Context, client *http.Client, location *time.Location, year, week int) ([]m6pro.Programme, error) {
		fetched = append(fetched, m6pro.WeekRef{Year: year, Number: week})
		return []m6pro.Programme{{ProgramID: "w9-new", BroadcastID: "b", Start: now, Title: "W9 programme"}}, nil
	}
	if err := EnsureW9Coverage(context.Background(), store, nil, time.UTC, now, 14*24*time.Hour, fetch); err != nil {
		t.Fatal(err)
	}
	if len(fetched) != 2 || fetched[0] != (m6pro.WeekRef{Year: 2026, Number: 42}) || fetched[1] != (m6pro.WeekRef{Year: 2026, Number: 43}) {
		t.Fatalf("fetched W9 weeks = %#v, want 2026-42 and 2026-43", fetched)
	}
	m6Rows, err := store.ProgramsBetween(now.Add(-time.Hour), now.Add(time.Hour))
	if err != nil { t.Fatal(err) }
	w9Rows, err := store.ProgramsBetweenW9(now.Add(-time.Hour), now.Add(time.Hour))
	if err != nil { t.Fatal(err) }
	if len(m6Rows) != 0 || len(w9Rows) != 3 {
		t.Fatalf("M6 rows=%d W9 rows=%d, want 0 and 3", len(m6Rows), len(w9Rows))
	}
}
