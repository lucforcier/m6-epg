package m6pro

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const baseURL = "https://pro.m6.fr/m6/grille"

type Programme struct {
	ProgramID, BroadcastID string
	Start time.Time
	Title, OriginalTitle, Subtitle, EpisodeTitle string
	Season, Episode, SeasonSummary, Format, Year, Country string
	Photo, PhotoCopyright, Synopsis string
	Signage Signage
	Cast []CastMember
}

type Signage struct {
	CSA string
	HD, VOST, Subtitle, AD, Live, Unreleased, Clear bool
}

type CastMember struct {
	Type string `xml:"type"`
	Name string `xml:"nom"`
	Role string `xml:"role"`
}

type grille struct {
	Grille Grille `xml:"grille_programmes"`
}

type Grille struct {
	Channel string `xml:"chaine"`
	Week Week `xml:"semaine"`
}

type Week struct {
	Number int `xml:"num_semaine"`
	Start string `xml:"date_debut"`
	End string `xml:"date_fin"`
	Days []Day `xml:"jours>jour"`
}

type Day struct {
	Date string `xml:"date"`
	Broadcasts []Broadcast `xml:"diffusions>diffusion"`
}

type Broadcast struct {
	ProgramID string `xml:"prid"`
	BroadcastID string `xml:"brid"`
	DateTime string `xml:"dateheure"`
	Title string `xml:"titreprogramme"`
	OriginalTitle string `xml:"titreoriginal"`
	Subtitle string `xml:"soustitredif"`
	EpisodeTitle string `xml:"titreepisode"`
	SeasonSummary string `xml:"resumesaison"`
	Format string `xml:"format"`
	Season string `xml:"saison"`
	Episode string `xml:"episode"`
	Year string `xml:"annee"`
	Country string `xml:"pays"`
	Photo string `xml:"photo"`
	PhotoCopyright string `xml:"copyright_photo"`
	Signage Signage `xml:"signaletique"`
	Cast []CastMember `xml:"casting>personne"`
	Synopsis string `xml:"resume"`
}

type rawBool struct {
	Value string
	Set   bool
}

func (b *rawBool) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var v string
	if err := d.DecodeElement(&v, &start); err != nil {
		return err
	}
	b.Value = strings.TrimSpace(v)
	b.Set = true
	return nil
}

type rawSignage struct {
	CSA string `xml:"csa"`
	HD rawBool `xml:"hd"`
	VOST rawBool `xml:"vost"`
	Subtitle rawBool `xml:"soustitre"`
	AD rawBool `xml:"ad"`
	Live rawBool `xml:"direct"`
	Unreleased rawBool `xml:"inedit"`
	Clear rawBool `xml:"en_clair"`
}

func (s *Signage) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var v rawSignage
	if err := d.DecodeElement(&v, &start); err != nil {
		return err
	}
	s.CSA = strings.TrimSpace(v.CSA)
	s.HD, s.VOST, s.Subtitle, s.AD = present(v.HD), present(v.VOST), present(v.Subtitle), present(v.AD)
	s.Live, s.Unreleased, s.Clear = present(v.Live), present(v.Unreleased), present(v.Clear)
	return nil
}

func present(v rawBool) bool {
	if !v.Set {
		return false
	}
	switch strings.ToLower(v.Value) {
	case "", "1", "true", "yes", "oui":
		return true
	case "0", "false", "no", "non":
		return false
	default:
		return true
	}
}

func Parse(r io.Reader, location *time.Location) ([]Programme, error) {
	if location == nil {
		return nil, fmt.Errorf("location is required")
	}
	var doc grille
	if err := xml.NewDecoder(r).Decode(&doc); err != nil {
		return nil, fmt.Errorf("decode M6 PRO XML: %w", err)
	}

	var programmes []Programme
	for _, day := range doc.Grille.Week.Days {
		for _, b := range day.Broadcasts {
			// M6 PRO's dateheure is a grid wall-clock time, not an absolute timestamp to convert from the source timezone.
			start, err := time.ParseInLocation("2006-01-02 15:04", strings.TrimSpace(b.DateTime), location)
			if err != nil {
				return nil, fmt.Errorf("parse broadcast %q: %w", b.DateTime, err)
			}
			programmes = append(programmes, Programme{
				ProgramID: b.ProgramID, BroadcastID: b.BroadcastID, Start: start,
				Title: strings.TrimSpace(b.Title), OriginalTitle: strings.TrimSpace(b.OriginalTitle),
				Subtitle: strings.TrimSpace(b.Subtitle), EpisodeTitle: strings.TrimSpace(b.EpisodeTitle),
				Season: strings.TrimSpace(b.Season), Episode: strings.TrimSpace(b.Episode),
				SeasonSummary: strings.TrimSpace(b.SeasonSummary), Format: strings.TrimSpace(b.Format),
				Year: strings.TrimSpace(b.Year), Country: strings.TrimSpace(b.Country),
				Photo: strings.TrimSpace(b.Photo), PhotoCopyright: strings.TrimSpace(b.PhotoCopyright),
				Synopsis: strings.TrimSpace(b.Synopsis), Signage: b.Signage, Cast: b.Cast,
			})
		}
	}
	return programmes, nil
}

func WeekURL(year, week int) string {
	return fmt.Sprintf("%s/%04d-%02d.xml", baseURL, year, week)
}

func FetchWeek(ctx context.Context, client *http.Client, location *time.Location, year, week int) ([]Programme, error) {
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, WeekURL(year, week), nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch M6 PRO week %04d-%02d: %w", year, week, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch M6 PRO week %04d-%02d: HTTP %s", year, week, resp.Status)
	}
	return Parse(resp.Body, location)
}
