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

## Gestion des horaires M6 PRO et des rediffusions

### Heure de grille, pas conversion de fuseau

Le champ M6 PRO `<dateheure>` représente l'heure de la grille de diffusion à utiliser pour M6-EPG. Il ne doit pas être interprété comme un instant absolu situé en `Europe/Paris` puis converti vers `America/Toronto`.

Autrement dit, pour la grille :

```
M6 PRO : 2026-10-07 17:25
        ↓
XMLTV : 2026-10-07 17:25 America/Toronto
        ↓
UTC    : 2026-10-07 21:25Z (en période EDT)
```

Cette distinction est importante pour la distribution québécoise. Une ancienne interprétation produisait par exemple `11:25 -0400` à partir de `17:25`, soit un décalage de six heures qui ne correspondait pas à la diffusion observée.

Le parseur utilise donc `time.ParseInLocation` avec le fuseau de sortie pour préserver l'heure de grille. Le test associé vérifie explicitement cette sémantique.

### Rediffusions et décalages de programmation

Une rediffusion ne doit pas être reconstruite à partir de l'heure de la première diffusion ni corrigée avec un décalage fixe. Chaque occurrence du XML M6 PRO est traitée comme une occurrence de grille indépendante : sa propre valeur `<dateheure>` détermine son heure de diffusion.

Ainsi, si M6 PRO place une rediffusion à une heure différente, cette heure différente est conservée. Le fait qu'il s'agisse du même programme ne doit jamais amener le générateur à réutiliser l'heure d'une autre occurrence.

La correction de fuseau et la gestion des rediffusions sont donc deux sujets distincts :

- **fuseau** : préserver l'heure de grille M6 PRO lors de la normalisation ;
- **rediffusion** : conserver l'heure propre à chaque occurrence fournie par M6 PRO.

### Persistance et correction des anciennes données

Le service utilise SQLite comme cache persistant. Une correction du parseur ne corrige donc pas rétroactivement des semaines déjà présentes si elles ne sont jamais re-téléchargées.

Le fonctionnement est maintenant séparé en deux modes :

- au démarrage, seules les semaines manquantes sont récupérées ;
- lors du refresh quotidien, les semaines nécessaires sont re-téléchargées et remplacées dans SQLite, même si elles existent déjà.

Cette seconde règle garantit notamment qu'une correction de parsing ou de sémantique horaire finisse par être appliquée aux données persistées sans devoir supprimer manuellement la base.
