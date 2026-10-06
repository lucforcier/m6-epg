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

	loc := time.UTC
	seed := func(ctx context.Context, client *http.Client, location *time.Location, year, week int) ([]m6pro.Programme, error) {
		return []m6pro.Programme{{
			ProgramID: "seed", BroadcastID: "b", Start: time.Date(year, 10, 1, 12, 0, 0, 0, time.UTC), Title: "seed",
		}}, nil
	}
	if err := EnsureCoverage(context.Background(), store, nil, loc, time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC), 14*24*time.Hour, seed); err != nil {
		t.Fatal(err)
	}

	var fetched []m6pro.WeekRef
	fetch := func(ctx context.Context, client *http.Client, location *time.Location, year, week int) ([]m6pro.Programme, error) {
		fetched = append(fetched, m6pro.WeekRef{Year: year, Number: week})
		return []m6pro.Programme{{
			ProgramID: "p", BroadcastID: string(rune('0' + week)), Start: time.Date(year, 10, 1, 12, 0, 0, 0, time.UTC), Title: "programme",
		}}, nil
	}
	if err := EnsureCoverage(context.Background(), store, nil, loc, time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC), 14*24*time.Hour, fetch); err != nil {
		t.Fatal(err)
	}
	if len(fetched) != 0 {
		t.Fatalf("fetched %d weeks on second run, want 0", len(fetched))
	}
}
