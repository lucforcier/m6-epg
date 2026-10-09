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

func TestW9WeekStorageIsSeparateFromM6(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "m6.db"))
	if err != nil { t.Fatal(err) }
	defer s.Close()

	start := time.Date(2026, 10, 10, 20, 0, 0, 0, time.UTC)
	m6 := m6pro.Programme{ProgramID: "same-id", BroadcastID: "same-broadcast", Start: start, Title: "M6 show"}
	w9 := m6pro.Programme{ProgramID: "same-id", BroadcastID: "same-broadcast", Start: start, Title: "W9 show"}
	if err := s.ReplaceWeek(2026, 42, []m6pro.Programme{m6}); err != nil { t.Fatal(err) }
	if err := s.ReplaceW9Week(2026, 42, []m6pro.Programme{w9}); err != nil { t.Fatal(err) }

	m6Rows, err := s.ProgramsBetween(start.Add(-time.Hour), start.Add(time.Hour))
	if err != nil { t.Fatal(err) }
	w9Rows, err := s.ProgramsBetweenW9(start.Add(-time.Hour), start.Add(time.Hour))
	if err != nil { t.Fatal(err) }
	if len(m6Rows) != 1 || m6Rows[0].Title != "M6 show" || m6Rows[0].ChannelID != "m6.fr" {
		t.Fatalf("M6 rows = %#v", m6Rows)
	}
	if len(w9Rows) != 1 || w9Rows[0].Title != "W9 show" || w9Rows[0].ChannelID != "w9.fr" {
		t.Fatalf("W9 rows = %#v", w9Rows)
	}
	hasM6, err := s.HasWeek(2026, 42)
	if err != nil { t.Fatal(err) }
	hasW9, err := s.HasW9Week(2026, 42)
	if err != nil { t.Fatal(err) }
	if !hasM6 || !hasW9 { t.Fatalf("week presence M6=%v W9=%v", hasM6, hasW9) }
}
