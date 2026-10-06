package xmltv

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/lucforcier/m6-epg/internal/store/sqlite"
)

const ChannelID = "m6.fr"

type tv struct {
	XMLName            xml.Name    `xml:"tv"`
	GeneratorInfoName string      `xml:"generator-info-name,attr"`
	Channel            channel     `xml:"channel"`
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
	EpisodeNum *episodeNum  `xml:"episode-num,omitempty"`
}

type textElement struct {
	Lang string `xml:"lang,attr,omitempty"`
	Text string `xml:",chardata"`
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
		Channel:            channel{ID: ChannelID, DisplayName: "M6"},
	}

	for i, p := range programmes {
		stop := end
		if i+1 < len(programmes) && programmes[i+1].Start.Before(stop) {
			stop = programmes[i+1].Start
		}
		if stop.Before(p.Start) {
			continue
		}

		item := programme{
			Start:   p.Start.In(location).Format("20060102150405 -0700"),
			Stop:    stop.In(location).Format("20060102150405 -0700"),
			Channel: ChannelID,
			Title:   textElement{Lang: "fr", Text: p.Title},
		}
		if p.Subtitle != "" {
			item.SubTitle = &textElement{Lang: "fr", Text: p.Subtitle}
		}
		if p.Synopsis != "" {
			item.Desc = &textElement{Lang: "fr", Text: p.Synopsis}
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
