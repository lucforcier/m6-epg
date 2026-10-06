package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/lucforcier/m6-epg/internal/coverage"
	"github.com/lucforcier/m6-epg/internal/store/sqlite"
	"github.com/lucforcier/m6-epg/internal/xmltv"
)

const (
	defaultDBPath         = "/data/m6.db"
	defaultGuidePath      = "/data/m6.xmltv"
	defaultCoverageDay    = 21
	defaultOutputLocation = "America/Toronto"
	defaultHTTPAddr       = "0.0.0.0:8080"
	defaultRefreshTime    = "03:00"
	defaultScheduleLoc    = "America/Toronto"
)

func main() {
	dbPath := envString("DB_PATH", defaultDBPath)
	guidePath := envString("GUIDE_PATH", defaultGuidePath)
	outputLocationName := envString("OUTPUT_LOCATION", defaultOutputLocation)
	coverageDays := envInt("COVERAGE_DAYS", defaultCoverageDay)
	httpAddr := envString("HTTP_ADDR", defaultHTTPAddr)
	refreshTime := envString("REFRESH_TIME", defaultRefreshTime)
	scheduleLocationName := envString("SCHEDULE_LOCATION", defaultScheduleLoc)

	outputLocation, err := time.LoadLocation(outputLocationName)
	if err != nil {
		log.Fatalf("load output location %q: %v", outputLocationName, err)
	}
	scheduleLocation, err := time.LoadLocation(scheduleLocationName)
	if err != nil {
		log.Fatalf("load schedule location %q: %v", scheduleLocationName, err)
	}
	if _, _, err := parseRefreshTime(refreshTime); err != nil {
		log.Fatal(err)
	}

	store, err := sqlite.Open(dbPath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer store.Close()

	log.Printf("m6-epg: database=%s guide=%s coverage=%dd location=%s refresh=%s schedule=%s", dbPath, guidePath, coverageDays, outputLocationName, refreshTime, scheduleLocationName)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := refresh(ctx, store, http.DefaultClient, outputLocation, guidePath, coverageDays, false); err != nil {
		if ctx.Err() != nil {
			return
		}
		log.Fatalf("initial refresh: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/epg.xml", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		http.ServeFile(w, r, guidePath)
	})
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})

	server := &http.Server{
		Addr:    httpAddr,
		Handler: mux,
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Printf("m6-epg: HTTP listening on %s", httpAddr)
		serverErr <- server.ListenAndServe()
	}()

	schedulerErr := make(chan error, 1)
	go func() {
		schedulerErr <- runScheduler(ctx, store, http.DefaultClient, outputLocation, guidePath, coverageDays, refreshTime, scheduleLocation)
	}()

	select {
	case err := <-serverErr:
		if err != nil && err != http.ErrServerClosed {
			log.Printf("m6-epg: HTTP server: %v", err)
		}
		stop()
	case err := <-schedulerErr:
		if err != nil && ctx.Err() == nil {
			log.Printf("m6-epg: scheduler stopped: %v", err)
		}
		stop()
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdownCtx)

	select {
	case err := <-serverErr:
		if err != nil && err != http.ErrServerClosed {
			log.Printf("m6-epg: HTTP server: %v", err)
		}
	default:
	}
}

func refresh(ctx context.Context, store *sqlite.Store, client *http.Client, location *time.Location, guidePath string, coverageDays int, refreshExisting bool) error {
	now := time.Now().In(location)
	coverageRefresh := coverage.EnsureCoverage
	if refreshExisting {
		coverageRefresh = coverage.RefreshCoverage
	}
	if err := coverageRefresh(
		ctx,
		store,
		client,
		location,
		now,
		time.Duration(coverageDays)*24*time.Hour,
		nil,
	); err != nil {
		return fmt.Errorf("ensure coverage: %w", err)
	}

	if err := writeGuide(store, guidePath, location, now); err != nil {
		return fmt.Errorf("write XMLTV: %w", err)
	}

	count, err := store.Count()
	if err != nil {
		return fmt.Errorf("count programmes: %w", err)
	}

	year, week, ok, err := store.LatestSourceWeek()
	if err != nil {
		return fmt.Errorf("find latest source week: %w", err)
	}
	if ok {
		log.Printf("m6-epg: coverage ready; latest source week=%04d-%02d programmes=%d", year, week, count)
	} else {
		log.Printf("m6-epg: coverage ready; no source weeks stored programmes=%d", count)
	}
	return nil
}

func runScheduler(ctx context.Context, store *sqlite.Store, client *http.Client, location *time.Location, guidePath string, coverageDays int, refreshTime string, scheduleLocation *time.Location) error {
	for {
		now := time.Now().In(scheduleLocation)
		next, err := nextRefresh(now, refreshTime)
		if err != nil {
			return err
		}

		wait := time.Until(next)
		log.Printf("m6-epg: next refresh at %s (in %s)", next.Format(time.RFC3339), wait.Round(time.Second))

		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return nil
		case <-timer.C:
		}

		log.Printf("m6-epg: scheduled refresh starting")
		if err := refresh(ctx, store, client, location, guidePath, coverageDays, true); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			log.Printf("m6-epg: scheduled refresh failed: %v", err)
			continue
		}
		log.Printf("m6-epg: scheduled refresh completed")
	}
}

func nextRefresh(now time.Time, refreshTime string) (time.Time, error) {
	hour, minute, err := parseRefreshTime(refreshTime)
	if err != nil {
		return time.Time{}, err
	}

	next := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
	if !next.After(now) {
		next = next.AddDate(0, 0, 1)
	}
	return next, nil
}

func parseRefreshTime(value string) (int, int, error) {
	if len(value) != 5 || value[2] != ':' {
		return 0, 0, fmt.Errorf("invalid refresh time %q: expected HH:MM", value)
	}

	hour, err := strconv.Atoi(value[:2])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid refresh time %q: expected HH:MM", value)
	}
	minute, err := strconv.Atoi(value[3:])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid refresh time %q: expected HH:MM", value)
	}
	if hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return 0, 0, fmt.Errorf("invalid refresh time %q: expected HH:MM", value)
	}
	return hour, minute, nil
}

func writeGuide(store *sqlite.Store, guidePath string, outputLocation *time.Location, now time.Time) error {
	guideStart := now
	guideEnd := now.Add(14 * 24 * time.Hour)
	programmes, err := store.ProgramsBetween(guideStart, guideEnd)
	if err != nil {
		return err
	}
	if err := xmltv.Write(guidePath, guideStart, guideEnd, outputLocation, programmes); err != nil {
		return err
	}
	log.Printf("m6-epg: XMLTV written to %s programmes=%d", guidePath, len(programmes))
	return nil
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
