package xmltv

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/lucforcier/m6-epg/internal/store/sqlite"
)

const ChannelID = "m6.fr"

type tv struct {
	XMLName xml.Name `xml:"tv"`
	GeneratorInfoName string `xml:"generator-info-name,attr"`
	Channel channel `xml:"channel"`
	Programmes []programme `xml:"programme"`
}
type channel struct {
	ID string `xml:"id,attr"`
	DisplayName string `xml:"display-name"`
}
type programme struct {
	Start string `xml:"start,attr"`
	Stop string `xml:"stop,attr"`
	Channel string `xml:"channel,attr"`
	Title textElement `xml:"title"`
	SubTitle *textElement `xml:"sub-title,omitempty"`
	Desc *textElement `xml:"desc,omitempty"`
	EpisodeNum *episodeNum `xml:"episode-num,omitempty"`
}
type textElement struct {
	Lang string `xml:"lang,attr,omitempty"`
	Text string `xml:",chardata"`
}
type episodeNum struct {
	System string `xml:"system,attr"`
	Text string `xml:",chardata"`
}

func Write(path string, start, end time.Time, programmes []sqlite.Programme) error {
	if path == "" { return fmt.Errorf("output path is required") }
	if end.Before(start) { return fmt.Errorf("output end must not be before start") }
	if dir := filepath.Dir(path); dir != "." { if err := os.MkdirAll(dir, 0755); err != nil { return fmt.Errorf("create output directory: %w", err) } }
	doc := tv{GeneratorInfoName:"m6-epg", Channel:channel{ID:ChannelID, DisplayName:"M6"}}
	for i, p := range programmes {
		stop := end
		if i+1 < len(programmes) && programmes[i+1].Start.Before(stop) { stop = programmes[i+1].Start }
		if stop.Before(p.Start) { continue }
		item := programme{Start:p.Start.Format("20060102150405 -0700"), Stop:stop.Format("20060102150405 -0700"), Channel:ChannelID, Title:textElement{Lang:"fr",Text:p.Title}}
		if p.Subtitle != "" { item.SubTitle=&textElement{Lang:"fr",Text:p.Subtitle} }
		if p.Synopsis != "" { item.Desc=&textElement{Lang:"fr",Text:p.Synopsis} }
		if p.Season != "" || p.Episode != "" { season,episode:=p.Season,p.Episode; if season=="" {season="0"}; if episode=="" {episode="0"}; item.EpisodeNum=&episodeNum{System:"xmltv_ns",Text:fmt.Sprintf("%s.%s.",season,episode)} }
		doc.Programmes=append(doc.Programmes,item)
	}
	f,err:=os.Create(path); if err!=nil{return fmt.Errorf("create XMLTV file: %w",err)}; defer f.Close()
	if _,err=io.WriteString(f,xml.Header);err!=nil{return err}
	enc:=xml.NewEncoder(f); enc.Indent("","  ")
	if err=enc.Encode(doc);err!=nil{return fmt.Errorf("encode XMLTV: %w",err)}
	if err=enc.Flush();err!=nil{return fmt.Errorf("flush XMLTV: %w",err)}
	return nil
}