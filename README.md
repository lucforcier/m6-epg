# m6-epg

Service Go permanent qui récupère le programme de M6 depuis les données M6 PRO, maintient une couverture source de 21 jours et expose un guide XMLTV glissant de 14 jours par HTTP.

## État

La première version fonctionnelle est maintenant validée :
- scraper M6 PRO par semaines samedi-vendredi ;
- stockage SQLite ;
- couverture future configurable, 21 jours par défaut ;
- guide XMLTV de 14 jours ;
- conversion Europe/Paris → America/Toronto ;
- publication atomique du guide ;
- HTTP `/epg.xml` et `/healthz` ;
- refresh quotidien configurable avec `REFRESH_TIME` ;
- fuseau du scheduler configurable avec `SCHEDULE_LOCATION` ;
- processus permanent 24 h/24.

Voir [`docs/PROJECT_STATUS.md`](docs/PROJECT_STATUS.md) pour l'historique des travaux et les validations.

## Configuration actuelle

Variables d'environnement principales :
- `DB_PATH` : base SQLite, par défaut `/data/m6.db` ;
- `GUIDE_PATH` : XMLTV publié, par défaut `/data/m6.xmltv` ;
- `COVERAGE_DAYS` : couverture source, par défaut `21` ;
- `M6_LOCATION` : fuseau source, par défaut `Europe/Paris` ;
- `OUTPUT_LOCATION` : fuseau XMLTV, par défaut `America/Toronto` ;
- `HTTP_ADDR` : adresse HTTP, par défaut `0.0.0.0:8080` ;
- `REFRESH_TIME` : heure quotidienne du refresh, par défaut `03:00` ;
- `SCHEDULE_LOCATION` : fuseau du scheduler, par défaut `America/Toronto`.

## HTTP

- `GET /epg.xml` — dernier guide XMLTV valide ;
- `HEAD /epg.xml` — vérification du guide ;
- `GET /healthz` — retourne `ok`.

Le endpoint XMLTV ne déclenche pas de scraping.

## Enrichissement

L'enrichissement externe des métadonnées est volontairement hors périmètre pour le moment. Des pistes comme TMDB, TVmaze et Wikidata ont été étudiées mais ne sont pas nécessaires à la première version.

## Développement

```text
cmd/m6-epg/         entrée du programme
internal/coverage/  couverture des semaines M6 PRO
internal/scraper/   source M6 PRO
internal/store/     SQLite
internal/xmltv/     génération XMLTV
```

Validation :

```bash
go test ./...
go vet ./...
go build -o m6-epg ./cmd/m6-epg
```

## Prochaine étape

La prochaine étape est la création de l'image Docker publique et de la configuration Compose avec stockage persistant dans `/data`.