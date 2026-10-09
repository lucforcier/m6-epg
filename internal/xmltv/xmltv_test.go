package xmltv

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lucforcier/m6-epg/internal/store/sqlite"
)

func TestXMLTVNSIsZeroBased(t *testing.T) {
	if got, ok := xmltvNS("7", "7"); !ok || got != "6.6." {
		t.Fatalf("xmltvNS(7,7) = %q, %v; want 6.6., true", got, ok)
	}
	if got, ok := xmltvNS("7", ""); ok || got != "" {
		t.Fatalf("xmltvNS(7,'') = %q, %v; want empty, false", got, ok)
	}
	if got, ok := xmltvNS("x", "2"); ok || got != "" {
		t.Fatalf("xmltvNS(x,2) = %q, %v; want empty, false", got, ok)
	}
}

func TestWriteUsesOutputLocationAndAtomicReplacement(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "guide.xmltv")
	loc, err := time.LoadLocation("America/Toronto")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 10, 6, 20, 50, 0, 0, time.UTC)
	programmes := []sqlite.Programme{
		{
			Start:    start,
			Title:    "9-1-1 & Friends",
			Season:   "7",
			Episode:  "7",
			Synopsis: "A test & description.",
			Photo:    "https://images.example/m6.jpg",
			CastJSON: `[{"Type":"Animateur / présentateur","Name":"Éric Antoine","Role":""}]`,
		},
		{
			Start: start.Add(45 * time.Minute),
			Title: "Programmes de nuit",
			CastJSON: "null",
		},
	}
	if err := Write(path, start, start.Add(24*time.Hour), loc, programmes); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, `start="20261006165000 -0400"`) {
		t.Fatalf("missing Toronto start timestamp in %s", text)
	}
	if !strings.Contains(text, `stop="20261006173500 -0400"`) {
		t.Fatalf("missing Toronto stop timestamp in %s", text)
	}
	if !strings.Contains(text, `<episode-num system="xmltv_ns">6.6.</episode-num>`) {
		t.Fatalf("missing zero-based episode number in %s", text)
	}
	if !strings.Contains(text, `<icon src="https://images.example/m6.jpg"></icon>`) {
		t.Fatalf("missing programme icon in %s", text)
	}
	if !strings.Contains(text, "<credits>") || !strings.Contains(text, "<presenter>Éric Antoine</presenter>") {
		t.Fatalf("missing presenter credit in %s", text)
	}
	if !strings.Contains(text, "9-1-1 &amp; Friends") || !strings.Contains(text, "A test &amp; description.") {
		t.Fatalf("XML escaping missing in %s", text)
	}

	var doc struct {
		Programmes []struct {
			Start string `xml:"start,attr"`
		} `xml:"programme"`
	}
	if err := xml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("generated XML is invalid: %v", err)
	}
}


func TestWriteIncludesBothChannelsAndDoesNotCrossChannelStop(t *testing.T) {
	path := filepath.Join(t.TempDir(), "guide.xmltv")
	start := time.Date(2026, 10, 10, 20, 0, 0, 0, time.UTC)
	programmes := []sqlite.Programme{
		{ChannelID: "m6.fr", Start: start, Title: "M6 first"},
		{ChannelID: "w9.fr", Start: start.Add(15 * time.Minute), Title: "W9 first"},
		{ChannelID: "m6.fr", Start: start.Add(time.Hour), Title: "M6 second"},
	}
	end := start.Add(2 * time.Hour)
	if err := Write(path, start, end, time.UTC, programmes); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, `<channel id="m6.fr">`) || !strings.Contains(text, `<channel id="w9.fr">`) {
		t.Fatalf("guide missing M6 or W9 channel declarations: %s", text)
	}
	if !strings.Contains(text, `stop="20261010210000 +0000" channel="m6.fr"`) {
		t.Fatalf("M6 first programme should stop at the next M6 programme: %s", text)
	}
	if !strings.Contains(text, `stop="20261010220000 +0000" channel="w9.fr"`) {
		t.Fatalf("W9 programme should stop at the guide boundary, not the M6 start: %s", text)
	}
}

func TestPresentersFromJSON(t *testing.T) {
	got, err := presentersFromJSON(`[
		{"Type":"Animateur / présentateur","Name":"Éric Antoine","Role":""},
		{"Type":"Autre","Name":"Ignored","Role":""}
	]`)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "Éric Antoine" {
		t.Fatalf("presentersFromJSON() = %#v; want [Éric Antoine]", got)
	}
}
