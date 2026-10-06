package m6pro

import (
	"strings"
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	xml := `<?xml version="1.0"?>
<rss><grille_programmes>
<chaine>M6</chaine>
<semaine>
<num_semaine>42</num_semaine><date_debut>2026-10-10</date_debut><date_fin>2026-10-16</date_fin>
<jours><jour><date>2026-10-10</date><diffusions>
<diffusion>
<prid>P123</prid><brid>B456</brid><dateheure>2026-10-10 20:10</dateheure>
<titreprogramme>Le film</titreprogramme><titreoriginal>Original</titreoriginal>
<soustitredif>Version TV</soustitredif><titreepisode>Épisode 1</titreepisode>
<resumesaison>Résumé saison</resumesaison><format>HD</format><saison>2</saison><episode>3</episode>
<annee>2026</annee><pays>France</pays><photo>https://example/photo.jpg</photo>
<copyright_photo>© M6</copyright_photo>
<signaletique><csa>10</csa><hd>1</hd><vost/><soustitre>1</soustitre><ad/><direct>1</direct><inedit>1</inedit><en_clair>1</en_clair></signaletique>
<casting><personne><type>acteur</type><nom>Jean Test</nom><role>Paul</role></personne></casting>
<resume>Un résumé.</resume>
</diffusion>
</diffusions></jour></jours>
</semaine></grille_programmes></rss>`

	loc, err := time.LoadLocation("America/Toronto")
	if err != nil { t.Fatal(err) }
	got, err := Parse(strings.NewReader(xml), loc)
	if err != nil { t.Fatal(err) }
	if len(got) != 1 { t.Fatalf("got %d programmes, want 1", len(got)) }
	p := got[0]
	if p.ProgramID != "P123" || p.BroadcastID != "B456" || p.Title != "Le film" { t.Fatalf("unexpected identity: %+v", p) }
	if p.Start.Format("2006-01-02 15:04") != "2026-10-10 20:10" { t.Fatalf("unexpected start: %v", p.Start) }
	if _, offset := p.Start.Zone(); offset != -4*60*60 { t.Fatalf("unexpected timezone offset: %v", p.Start) }
	if !p.Signage.HD || !p.Signage.VOST || !p.Signage.Subtitle || !p.Signage.Live || !p.Signage.Unreleased || !p.Signage.Clear { t.Fatalf("signage flags not parsed: %+v", p.Signage) }
	if len(p.Cast) != 1 || p.Cast[0].Name != "Jean Test" || p.Cast[0].Role != "Paul" { t.Fatalf("cast not parsed: %+v", p.Cast) }
}

func TestWeekURL(t *testing.T) {
	want := "https://pro.m6.fr/m6/grille/2026-42.xml"
	if got := WeekURL(2026, 42); got != want { t.Fatalf("got %q, want %q", got, want) }
}
