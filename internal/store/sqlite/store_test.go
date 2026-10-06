package sqlite

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/lucforcier/m6-epg/internal/scraper/m6pro"
)

func TestOpenAndReplaceWeek(t *testing.T) {
	path := filepath.Join(t.TempDir(), "m6.db")
	s, err := Open(path)
	if err != nil { t.Fatal(err) }
	defer s.Close()

	p := m6pro.Programme{
		ProgramID: "P1",
		BroadcastID: "B1",
		Start: time.Date(2026, 10, 10, 20, 10, 0, 0, time.FixedZone("CEST", 2*60*60)),
		Title: "Le film",
	}
	if err := s.ReplaceWeek(2026, 41, []m6pro.Programme{p}); err != nil { t.Fatal(err) }
	n, err := s.Count()
	if err != nil { t.Fatal(err) }
	if n != 1 { t.Fatalf("count = %d, want 1", n) }
}

func TestReplaceWeekUsesM6SaturdayWindow(t *testing.T) {
	path := filepath.Join(t.TempDir(), "m6.db")
	s, err := Open(path)
	if err != nil { t.Fatal(err) }
	defer s.Close()

	p := m6pro.Programme{
		ProgramID: "P1", BroadcastID: "B1",
		Start: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
		Title: "Test",
	}
	if err := s.ReplaceWeek(2026, 1, []m6pro.Programme{p}); err != nil { t.Fatal(err) }

	y, w, ok, err := s.LatestSourceWeek()
	if err != nil { t.Fatal(err) }
	if !ok || y != 2026 || w != 1 { t.Fatalf("latest source = %d-%02d, ok=%v", y, w, ok) }
}
