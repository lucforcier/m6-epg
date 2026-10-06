package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/lucforcier/m6-epg/internal/coverage"
	"github.com/lucforcier/m6-epg/internal/store/sqlite"
)

const (
	defaultDBPath      = "/data/m6.db"
	defaultGuidePath   = "/data/m6.xmltv"
	defaultCoverageDay = 21
	defaultLocation    = "Europe/Paris"
)

func main() {
	dbPath := envString("DB_PATH", defaultDBPath)
	guidePath := envString("GUIDE_PATH", defaultGuidePath)
	locationName := envString("M6_LOCATION", defaultLocation)
	coverageDays := envInt("COVERAGE_DAYS", defaultCoverageDay)

	location, err := time.LoadLocation(locationName)
	if err != nil {
		log.Fatalf("load location %q: %v", locationName, err)
	}

	store, err := sqlite.Open(dbPath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer store.Close()

	log.Printf("m6-epg: database=%s guide=%s coverage=%dd location=%s", dbPath, guidePath, coverageDays, locationName)

	now := time.Now().In(location)
	if err := coverage.EnsureCoverage(
		context.Background(),
		store,
		http.DefaultClient,
		location,
		now,
		time.Duration(coverageDays)*24*time.Hour,
		nil,
	); err != nil {
		log.Fatalf("ensure coverage: %v", err)
	}

	count, err := store.Count()
	if err != nil {
		log.Fatalf("count programmes: %v", err)
	}

	year, week, ok, err := store.LatestSourceWeek()
	if err != nil {
		log.Fatalf("find latest source week: %v", err)
	}
	if ok {
		log.Printf("m6-epg: coverage ready; latest source week=%04d-%02d programmes=%d", year, week, count)
	} else {
		log.Printf("m6-epg: coverage ready; no source weeks stored programmes=%d", count)
	}
}

func envString(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func envInt(name string, fallback int) int {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	n, err := strconv.Atoi(value)
	if err != nil || n < 0 {
		log.Printf("m6-epg: invalid %s=%q; using %d", name, value, fallback)
		return fallback
	}
	return n
}
