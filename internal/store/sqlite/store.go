package sqlite

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	"github.com/lucforcier/m6-epg/internal/scraper/m6pro"
)

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	if path == "" {
		return nil, fmt.Errorf("database path is required")
	}
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("create database directory: %w", err)
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open SQLite database: %w", err)
	}
	// SQLite permits only one writer at a time. A single pooled connection
	// keeps concurrent M6/W9 refreshes from racing each other for write locks;
	// the HTTP fetches still run concurrently.
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	if err := s.init(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) init() error {
	const schema = `
PRAGMA journal_mode = WAL;

CREATE TABLE IF NOT EXISTS sources (
	id INTEGER PRIMARY KEY,
	year INTEGER NOT NULL,
	week INTEGER NOT NULL,
	url TEXT NOT NULL,
	fetched_at TEXT NOT NULL,
	UNIQUE(year, week)
);

CREATE TABLE IF NOT EXISTS programmes (
	program_id TEXT NOT NULL,
	broadcast_id TEXT NOT NULL,
	start_time TEXT NOT NULL,
	title TEXT NOT NULL,
	original_title TEXT,
	subtitle TEXT,
	episode_title TEXT,
	season TEXT,
	episode TEXT,
	season_summary TEXT,
	format TEXT,
	year TEXT,
	country TEXT,
	photo TEXT,
	photo_copyright TEXT,
	synopsis TEXT,
	signage_csa TEXT,
	signage_hd INTEGER NOT NULL DEFAULT 0,
	signage_vost INTEGER NOT NULL DEFAULT 0,
	signage_subtitle INTEGER NOT NULL DEFAULT 0,
	signage_ad INTEGER NOT NULL DEFAULT 0,
	signage_live INTEGER NOT NULL DEFAULT 0,
	signage_unreleased INTEGER NOT NULL DEFAULT 0,
	signage_clear INTEGER NOT NULL DEFAULT 0,
	cast_json TEXT,
	PRIMARY KEY(program_id, broadcast_id, start_time)
);

CREATE INDEX IF NOT EXISTS idx_programmes_start ON programmes(start_time);

CREATE TABLE IF NOT EXISTS w9_sources (
	id INTEGER PRIMARY KEY,
	year INTEGER NOT NULL,
	week INTEGER NOT NULL,
	url TEXT NOT NULL,
	fetched_at TEXT NOT NULL,
	UNIQUE(year, week)
);

CREATE TABLE IF NOT EXISTS w9_programmes (
	program_id TEXT NOT NULL,
	broadcast_id TEXT NOT NULL,
	start_time TEXT NOT NULL,
	title TEXT NOT NULL,
	original_title TEXT,
	subtitle TEXT,
	episode_title TEXT,
	season TEXT,
	episode TEXT,
	season_summary TEXT,
	format TEXT,
	year TEXT,
	country TEXT,
	photo TEXT,
	photo_copyright TEXT,
	synopsis TEXT,
	signage_csa TEXT,
	signage_hd INTEGER NOT NULL DEFAULT 0,
	signage_vost INTEGER NOT NULL DEFAULT 0,
	signage_subtitle INTEGER NOT NULL DEFAULT 0,
	signage_ad INTEGER NOT NULL DEFAULT 0,
	signage_live INTEGER NOT NULL DEFAULT 0,
	signage_unreleased INTEGER NOT NULL DEFAULT 0,
	signage_clear INTEGER NOT NULL DEFAULT 0,
	cast_json TEXT,
	PRIMARY KEY(program_id, broadcast_id, start_time)
);

CREATE INDEX IF NOT EXISTS idx_w9_programmes_start ON w9_programmes(start_time);
`
	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("initialize SQLite schema: %w", err)
	}
	return nil
}

func (s *Store) ReplaceWeek(year, week int, programmes []m6pro.Programme) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin SQLite transaction: %w", err)
	}
	defer tx.Rollback()

	location := weekLocation(programmes)
	start, end := weekBounds(year, week, location), weekBounds(year, week+1, location)
	if _, err := tx.Exec("DELETE FROM programmes WHERE start_time >= ? AND start_time < ?", start, end); err != nil {
		return fmt.Errorf("clear week programmes: %w", err)
	}

	stmt, err := tx.Prepare(`
INSERT INTO programmes (
	program_id, broadcast_id, start_time, title, original_title, subtitle, episode_title,
	season, episode, season_summary, format, year, country, photo, photo_copyright, synopsis,
	signage_csa, signage_hd, signage_vost, signage_subtitle, signage_ad, signage_live,
	signage_unreleased, signage_clear, cast_json
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`)
	if err != nil {
		return fmt.Errorf("prepare programme insert: %w", err)
	}
	defer stmt.Close()

	for _, p := range programmes {
		cast, err := json.Marshal(p.Cast)
		if err != nil {
			return fmt.Errorf("encode cast for %q: %w", p.Title, err)
		}
		_, err = stmt.Exec(
			p.ProgramID, p.BroadcastID, p.Start.UTC().Format(time.RFC3339),
			p.Title, p.OriginalTitle, p.Subtitle, p.EpisodeTitle,
			p.Season, p.Episode, p.SeasonSummary, p.Format, p.Year, p.Country,
			p.Photo, p.PhotoCopyright, p.Synopsis,
			p.Signage.CSA, boolInt(p.Signage.HD), boolInt(p.Signage.VOST),
			boolInt(p.Signage.Subtitle), boolInt(p.Signage.AD), boolInt(p.Signage.Live),
			boolInt(p.Signage.Unreleased), boolInt(p.Signage.Clear), string(cast),
		)
		if err != nil {
			return fmt.Errorf("insert programme %q: %w", p.Title, err)
		}
	}

	_, err = tx.Exec(
		"INSERT INTO sources(year, week, url, fetched_at) VALUES (?, ?, ?, ?) ON CONFLICT(year, week) DO UPDATE SET url=excluded.url, fetched_at=excluded.fetched_at",
		year, week, m6pro.WeekURL(year, week), time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("record source week: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit SQLite transaction: %w", err)
	}
	return nil
}

// ReplaceW9Week atomically replaces one W9 PRO week without touching M6 rows.
func (s *Store) ReplaceW9Week(year, week int, programmes []m6pro.Programme) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin SQLite transaction: %w", err)
	}
	defer tx.Rollback()

	location := weekLocation(programmes)
	start, end := weekBounds(year, week, location), weekBounds(year, week+1, location)
	if _, err := tx.Exec("DELETE FROM w9_programmes WHERE start_time >= ? AND start_time < ?", start, end); err != nil {
		return fmt.Errorf("clear W9 week programmes: %w", err)
	}

	stmt, err := tx.Prepare(`
INSERT INTO w9_programmes (
	program_id, broadcast_id, start_time, title, original_title, subtitle, episode_title,
	season, episode, season_summary, format, year, country, photo, photo_copyright, synopsis,
	signage_csa, signage_hd, signage_vost, signage_subtitle, signage_ad, signage_live,
	signage_unreleased, signage_clear, cast_json
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`)
	if err != nil {
		return fmt.Errorf("prepare W9 programme insert: %w", err)
	}
	defer stmt.Close()

	for _, p := range programmes {
		cast, err := json.Marshal(p.Cast)
		if err != nil {
			return fmt.Errorf("encode W9 cast for %q: %w", p.Title, err)
		}
		_, err = stmt.Exec(
			p.ProgramID, p.BroadcastID, p.Start.UTC().Format(time.RFC3339),
			p.Title, p.OriginalTitle, p.Subtitle, p.EpisodeTitle,
			p.Season, p.Episode, p.SeasonSummary, p.Format, p.Year, p.Country,
			p.Photo, p.PhotoCopyright, p.Synopsis,
			p.Signage.CSA, boolInt(p.Signage.HD), boolInt(p.Signage.VOST),
			boolInt(p.Signage.Subtitle), boolInt(p.Signage.AD), boolInt(p.Signage.Live),
			boolInt(p.Signage.Unreleased), boolInt(p.Signage.Clear), string(cast),
		)
		if err != nil {
			return fmt.Errorf("insert W9 programme %q: %w", p.Title, err)
		}
	}

	_, err = tx.Exec(
		"INSERT INTO w9_sources(year, week, url, fetched_at) VALUES (?, ?, ?, ?) ON CONFLICT(year, week) DO UPDATE SET url=excluded.url, fetched_at=excluded.fetched_at",
		year, week, m6pro.WeekURLFor("w9", year, week), time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("record W9 source week: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit W9 transaction: %w", err)
	}
	return nil
}

// HasW9Week reports whether a W9 week has been fetched successfully.
func (s *Store) HasW9Week(year, week int) (bool, error) {
	var n int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM w9_sources WHERE year = ? AND week = ?", year, week).Scan(&n); err != nil {
		return false, fmt.Errorf("check W9 source week: %w", err)
	}
	return n > 0, nil
}

// ProgramsBetweenW9 returns W9 programmes in the requested UTC interval.
func (s *Store) ProgramsBetweenW9(start, end time.Time) ([]Programme, error) {
	rows, err := s.db.Query("SELECT program_id, broadcast_id, start_time, title, original_title, subtitle, episode_title, season, episode, synopsis, photo, cast_json FROM w9_programmes WHERE start_time >= ? AND start_time < ? ORDER BY start_time, broadcast_id", start.UTC().Format(time.RFC3339), end.UTC().Format(time.RFC3339))
	if err != nil {
		return nil, fmt.Errorf("query W9 programmes: %w", err)
	}
	defer rows.Close()
	var programmes []Programme
	for rows.Next() {
		var p Programme
		var startText string
		if err := rows.Scan(&p.ProgramID, &p.BroadcastID, &startText, &p.Title, &p.OriginalTitle, &p.Subtitle, &p.EpisodeTitle, &p.Season, &p.Episode, &p.Synopsis, &p.Photo, &p.CastJSON); err != nil {
			return nil, fmt.Errorf("scan W9 programme: %w", err)
		}
		p.Start, err = time.Parse(time.RFC3339, startText)
		if err != nil {
			return nil, fmt.Errorf("parse W9 programme start %q: %w", startText, err)
		}
		p.ChannelID = "w9.fr"
		programmes = append(programmes, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate W9 programmes: %w", err)
	}
	return programmes, nil
}

type Programme struct {
	ChannelID string
	ProgramID, BroadcastID string
	Start time.Time
	Title, OriginalTitle, Subtitle, EpisodeTitle string
	Season, Episode, Synopsis string
	Photo, CastJSON string
}

func (s *Store) ProgramsBetween(start, end time.Time) ([]Programme, error) {
	rows, err := s.db.Query("SELECT program_id, broadcast_id, start_time, title, original_title, subtitle, episode_title, season, episode, synopsis, photo, cast_json FROM programmes WHERE start_time >= ? AND start_time < ? ORDER BY start_time, broadcast_id", start.UTC().Format(time.RFC3339), end.UTC().Format(time.RFC3339))
	if err != nil { return nil, fmt.Errorf("query programmes: %w", err) }
	defer rows.Close()
	var programmes []Programme
	for rows.Next() {
		var p Programme
		var startText string
		if err := rows.Scan(&p.ProgramID, &p.BroadcastID, &startText, &p.Title, &p.OriginalTitle, &p.Subtitle, &p.EpisodeTitle, &p.Season, &p.Episode, &p.Synopsis, &p.Photo, &p.CastJSON); err != nil { return nil, fmt.Errorf("scan programme: %w", err) }
		p.Start, err = time.Parse(time.RFC3339, startText)
		if err != nil { return nil, fmt.Errorf("parse programme start %q: %w", startText, err) }
		p.ChannelID = "m6.fr"
		programmes = append(programmes, p)
	}
	if err := rows.Err(); err != nil { return nil, fmt.Errorf("iterate programmes: %w", err) }
	return programmes, nil
}

func (s *Store) CountW9() (int, error) {
	var n int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM w9_programmes").Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

func (s *Store) Count() (int, error) {
	var n int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM programmes").Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

func boolInt(v bool) int {
	if v { return 1 }
	return 0
}

// HasWeek reports whether an M6 week has been fetched successfully.
func (s *Store) HasWeek(year, week int) (bool, error) {
	var n int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM sources WHERE year = ? AND week = ?", year, week).Scan(&n); err != nil {
		return false, fmt.Errorf("check source week: %w", err)
	}
	return n > 0, nil
}

// LatestSourceWeek returns the most recently fetched M6 week.
func (s *Store) LatestSourceWeek() (year, week int, ok bool, err error) {
	err = s.db.QueryRow(`SELECT year, week FROM sources ORDER BY year DESC, week DESC LIMIT 1`).Scan(&year, &week)
	if err == sql.ErrNoRows {
		return 0, 0, false, nil
	}
	if err != nil {
		return 0, 0, false, fmt.Errorf("find latest source week: %w", err)
	}
	return year, week, true, nil
}

// weekLocation returns the wall-clock timezone used to parse a source grid.
func weekLocation(programmes []m6pro.Programme) *time.Location {
	if len(programmes) > 0 && programmes[0].Start.Location() != nil {
		return programmes[0].Start.Location()
	}
	return time.UTC
}

// weekBounds returns a source week's Saturday-midnight boundaries converted
// to UTC. Grid timestamps are parsed in the source schedule location, so
// calculating these boundaries in UTC can leave late-Friday programmes
// outside the deletion window and cause duplicate-key failures on refresh.
func weekBounds(year, week int, location *time.Location) string {
	if location == nil {
		location = time.UTC
	}
	return m6WeekStart(year, week, location).UTC().Format(time.RFC3339)
}

func m6WeekStart(year, week int, location *time.Location) time.Time {
	first := saturdayOnOrBefore(time.Date(year, 1, 1, 0, 0, 0, 0, location))
	return first.AddDate(0, 0, (week-1)*7)
}

func saturdayOnOrBefore(t time.Time) time.Time {
	weekday := int(t.Weekday())
	daysSinceSaturday := (weekday - 6 + 7) % 7
	return t.AddDate(0, 0, -daysSinceSaturday)
}
