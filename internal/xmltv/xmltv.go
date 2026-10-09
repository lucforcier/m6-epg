package xmltv

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/lucforcier/m6-epg/internal/store/sqlite"
)

const ChannelID = "m6.fr"

type tv struct {
	XMLName            xml.Name    `xml:"tv"`
	GeneratorInfoName string      `xml:"generator-info-name,attr"`
	Channel            []channel   `xml:"channel"`
	Programmes         []programme `xml:"programme"`
}

type channel struct {
	ID          string `xml:"id,attr"`
	DisplayName string `xml:"display-name"`
}

type programme struct {
	Start      string       `xml:"start,attr"`
	Stop       string       `xml:"stop,attr"`
	Channel    string       `xml:"channel,attr"`
	Title      textElement  `xml:"title"`
	SubTitle   *textElement `xml:"sub-title,omitempty"`
	Desc       *textElement `xml:"desc,omitempty"`
	Icon       *icon        `xml:"icon,omitempty"`
	Credits    *credits     `xml:"credits,omitempty"`
	EpisodeNum *episodeNum  `xml:"episode-num,omitempty"`
}

type textElement struct {
	Lang string `xml:"lang,attr,omitempty"`
	Text string `xml:",chardata"`
}

type icon struct {
	Src string `xml:"src,attr"`
}

type credits struct {
	Presenters []string `xml:"presenter,omitempty"`
}

type castMember struct {
	Type string `json:"Type"`
	Name string `json:"Name"`
	Role string `json:"Role"`
}

type episodeNum struct {
	System string `xml:"system,attr"`
	Text   string `xml:",chardata"`
}

func Write(path string, start, end time.Time, location *time.Location, programmes []sqlite.Programme) error {
	if path == "" {
		return fmt.Errorf("output path is required")
	}
	if location == nil {
		return fmt.Errorf("output location is required")
	}
	if end.Before(start) {
		return fmt.Errorf("output end must not be before start")
	}
	dir := filepath.Dir(path)
	if dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create output directory: %w", err)
		}
	}

	doc := tv{
		GeneratorInfoName: "m6-epg",
		Channel: []channel{{ID: "m6.fr", DisplayName: "M6"}, {ID: "w9.fr", DisplayName: "W9"}},
	}

	// Sort by channel and start time so each programme's stop time is
	// derived from the next programme on that same channel only.
	programmes = append([]sqlite.Programme(nil), programmes...)
	for i := range programmes {
		if programmes[i].ChannelID == "" {
			programmes[i].ChannelID = ChannelID
		}
	}
	sort.SliceStable(programmes, func(i, j int) bool {
		if programmes[i].ChannelID != programmes[j].ChannelID {
			return programmes[i].ChannelID < programmes[j].ChannelID
		}
		if !programmes[i].Start.Equal(programmes[j].Start) {
			return programmes[i].Start.Before(programmes[j].Start)
		}
		return programmes[i].BroadcastID < programmes[j].BroadcastID
	})

	for i, p := range programmes {
		stop := end
		if i+1 < len(programmes) &&
			programmes[i+1].ChannelID == p.ChannelID &&
			programmes[i+1].Start.Before(stop) {
			stop = programmes[i+1].Start
		}
		if stop.Before(p.Start) {
			continue
		}

		item := programme{
			Start:   p.Start.In(location).Format("20060102150405 -0700"),
			Stop:    stop.In(location).Format("20060102150405 -0700"),
			Channel: p.ChannelID,
			Title:   textElement{Lang: "fr", Text: p.Title},
		}
		if p.Subtitle != "" {
			item.SubTitle = &textElement{Lang: "fr", Text: p.Subtitle}
		}
		if p.Synopsis != "" {
			item.Desc = &textElement{Lang: "fr", Text: p.Synopsis}
		}
		if p.Photo != "" {
			item.Icon = &icon{Src: p.Photo}
		}
		presenters, err := presentersFromJSON(p.CastJSON)
		if err != nil {
			return fmt.Errorf("parse cast for %q: %w", p.Title, err)
		}
		if len(presenters) > 0 {
			item.Credits = &credits{Presenters: presenters}
		}
		if value, ok := xmltvNS(p.Season, p.Episode); ok {
			item.EpisodeNum = &episodeNum{System: "xmltv_ns", Text: value}
		}
		doc.Programmes = append(doc.Programmes, item)
	}

	tmp, err := os.CreateTemp(dir, ".m6-epg-xmltv-*")
	if err != nil {
		return fmt.Errorf("create temporary XMLTV file: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}
	defer cleanup()

	if _, err = io.WriteString(tmp, xml.Header); err != nil {
		return fmt.Errorf("write XMLTV header: %w", err)
	}
	enc := xml.NewEncoder(tmp)
	enc.Indent("", "  ")
	if err = enc.Encode(doc); err != nil {
		return fmt.Errorf("encode XMLTV: %w", err)
	}
	if err = enc.Flush(); err != nil {
		return fmt.Errorf("flush XMLTV: %w", err)
	}
	if err = tmp.Close(); err != nil {
		return fmt.Errorf("close XMLTV file: %w", err)
	}
	if err = os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replace XMLTV file: %w", err)
	}
	return nil
}

func presentersFromJSON(value string) ([]string, error) {
	if value == "" || value == "null" || value == "[]" {
		return nil, nil
	}
	var cast []castMember
	if err := json.Unmarshal([]byte(value), &cast); err != nil {
		return nil, err
	}
	var presenters []string
	for _, member := range cast {
		if member.Type == "Animateur / présentateur" && member.Name != "" {
			presenters = append(presenters, member.Name)
		}
	}
	return presenters, nil
}

func xmltvNS(season, episode string) (string, bool) {
	if season == "" || episode == "" {
		return "", false
	}
	s, err := strconv.Atoi(season)
	if err != nil || s < 1 {
		return "", false
	}
	e, err := strconv.Atoi(episode)
	if err != nil || e < 1 {
		return "", false
	}
	return fmt.Sprintf("%d.%d.", s-1, e-1), true
}
