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

## Développement assisté par IA

Ce projet a été développé avec une assistance substantielle de **ChatGPT d'OpenAI**.

Les détails de cette utilisation, les responsabilités du mainteneur et les principales considérations relatives aux droits, licences, données et services tiers sont documentés dans [`docs/AI-DEVELOPMENT-NOTICE.md`](docs/AI-DEVELOPMENT-NOTICE.md).

Cette mention est une mesure de transparence. OpenAI n'est ni l'auteur, ni le mainteneur, ni le distributeur du projet.

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

## Docker

Une image publique est publiée automatiquement vers GitHub Container Registry à chaque modification de `main`.

Image :

`ghcr.io/lucforcier/m6-epg:latest`

Exemple Compose :

```yaml
services:
  m6-epg:
    image: ghcr.io/lucforcier/m6-epg:latest
    environment:
      REFRESH_TIME: "03:00"
      SCHEDULE_LOCATION: "America/Toronto"
    volumes:
      - m6-data:/data
    ports:
      - "8081:8080"
    restart: unless-stopped

volumes:
  m6-data:
```

Le conteneur écoute sur le port 8080. Le port hôte peut être changé avec `HOST_PORT` dans le fichier `.env`. Le volume `/data` conserve la base SQLite et le guide XMLTV lors des recréations du conteneur.

Copie de départ pour la configuration :

```bash
cp .env.example .env
```

Endpoints :

- `http://localhost:8081/epg.xml`
- `http://localhost:8081/healthz`

Le package GHCR doit être rendu **Public** dans les paramètres GitHub du package après sa première publication si GitHub l'a créé avec une visibilité privée. Les images publiques de GHCR peuvent ensuite être téléchargées sans authentification.

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
