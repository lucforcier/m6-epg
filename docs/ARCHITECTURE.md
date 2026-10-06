# Architecture de m6-epg

## 1. Principe général

m6-epg est un service permanent. Il ne s'agit pas d'un générateur lancé ponctuellement : le processus reste actif, fournit le guide HTTP et déclenche automatiquement la mise à jour quotidienne.

```
                         m6-epg
                           │
              ┌────────────┴────────────┐
              │                         │
          scheduler                 HTTP server
              │                         │
              ▼                         ├── GET /epg.xml
        M6 PRO scraper                 ├── GET /health
              │                         └── GET /status
              ▼
            SQLite
              │
              ▼
        XMLTV generator
              │
              ▼
       validation + publication
              │
              ▼
          /data/m6.xml
```

## 2. Fenêtre glissante

Le guide couvre une fenêtre relative à la date d'exécution :

```
aujourd'hui ───────────────────────────────► aujourd'hui + 14 jours
     │                                               │
     └────────────── guide XMLTV ────────────────────┘
```

Le nombre de jours sera configurable par `DAYS`, avec 14 comme valeur cible de la première version.

À chaque nouveau cycle, la fenêtre est recalculée. Le guide avance donc automatiquement d'une journée sans modification de configuration.

## 3. Scheduler

Le service démarre immédiatement. Au démarrage :

1. ouvrir ou créer SQLite ;
2. vérifier si un guide valide existe ;
3. effectuer un premier scrape si nécessaire ;
4. publier le guide s'il est valide ;
5. démarrer le serveur HTTP ;
6. attendre `SCRAPE_TIME` dans `TIMEZONE` ;
7. exécuter le cycle quotidien.

Un cycle planifié doit pouvoir être relancé sans redémarrer le serveur.

En cas d'échec, quelques tentatives différées sont prévues. Une stratégie simple pourra être utilisée au départ, par exemple à T+15 min, T+30 min et T+60 min.

## 4. Garantie de continuité du guide

Le guide actuellement publié est considéré comme un artefact stable.

Le nouveau cycle ne doit pas écrire directement dans le fichier servi :

```
M6 PRO
  ↓
récupération
  ↓
SQLite
  ↓
génération temporaire
  ↓
validation XMLTV
  ↓
rename atomique
  ↓
m6.xml
```

Si le scraping ou la validation échoue, `m6.xml` reste inchangé.

Cela permet aux clients EPG de continuer à utiliser le dernier guide connu même lorsque M6 PRO est temporairement indisponible.

## 5. SQLite

SQLite constitue le stockage applicatif et permet notamment de conserver l'état nécessaire au fonctionnement permanent.

Le modèle initial pourra comporter :

### Sources

Informations sur les données M6 PRO récupérées :

- période/semaine source ;
- URL ;
- date de récupération ;
- empreinte du contenu ;
- données brutes ou référence vers le cache local.

### Programmes

Données normalisées utilisées pour construire le XMLTV :

- identifiant du programme ;
- début/fin ;
- titre ;
- sous-titre ;
- titre d'épisode ;
- saison/épisode ;
- description ;
- métadonnées disponibles ;
- informations d'image lorsque présentes.

### Runs

Historique minimal des cycles :

- début ;
- fin ;
- statut ;
- nombre de programmes ;
- erreur éventuelle.

Le schéma exact sera défini avec l'implémentation du scraper.

## 6. HTTP

### `GET /epg.xml`

Retourne le dernier XMLTV validé.

Le endpoint ne doit pas déclencher de scraping et ne doit pas reconstruire le guide à chaque requête.

### `GET /health`

Endpoint léger destiné notamment au healthcheck Docker.

### `GET /status`

Expose les informations utiles au diagnostic :

- dernier cycle ;
- statut du dernier cycle ;
- prochain cycle ;
- nombre de programmes ;
- début/fin de la fenêtre ;
- dernière erreur éventuelle ;
- âge du guide.

## 7. Organisation Go

```
m6-epg/
├── cmd/
│   └── m6-epg/
│       └── main.go
├── internal/
│   ├── config/
│   ├── scheduler/
│   ├── scraper/
│   │   └── m6pro/
│   ├── store/
│   │   └── sqlite/
│   ├── xmltv/
│   └── httpapi/
├── docs/
│   └── ARCHITECTURE.md
├── Dockerfile
├── docker-compose.yml
├── go.mod
└── README.md
```

Les packages `internal` ne sont pas destinés à être consommés par d'autres projets.

## 8. Docker

Le conteneur sera un processus unique permanent.

Configuration cible :

```yaml
services:
  m6-epg:
    image: ghcr.io/lucforcier/m6-epg:latest
    environment:
      SCRAPE_TIME: "03:00"
      TIMEZONE: "Europe/Paris"
      DAYS: "14"
      LISTEN_ADDR: "0.0.0.0:8080"
    volumes:
      - ./data:/data
    ports:
      - "8080:8080"
    restart: unless-stopped
```

Les valeurs ci-dessus décrivent la configuration cible ; elles ne constituent pas encore une implémentation fonctionnelle.

## 9. Ordre d'implémentation

1. définir la configuration ;
2. implémenter le modèle SQLite ;
3. isoler le scraper M6 PRO ;
4. normaliser les programmes ;
5. générer et valider XMLTV ;
6. publier atomiquement le guide ;
7. ajouter scheduler et retries ;
8. ajouter les endpoints HTTP ;
9. ajouter Docker et healthcheck ;
10. tester un cycle complet sur 14 jours.

L'objectif est de conserver une première version petite et facilement vérifiable.
