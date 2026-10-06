# m6-epg

Service Go permanent qui récupère le programme de M6 depuis les données M6 PRO, construit un guide XMLTV sur une fenêtre glissante de 14 jours et l'expose par HTTP.

## Objectif

Le service doit fonctionner 24 h/24 et maintenir un guide EPG toujours à jour sans intervention manuelle.

À chaque cycle de scraping :

1. déterminer la fenêtre `aujourd'hui → aujourd'hui + 14 jours` ;
2. identifier les semaines M6 PRO nécessaires ;
3. récupérer et valider les données source ;
4. mettre à jour le stockage SQLite ;
5. générer le XMLTV complet ;
6. valider le XMLTV ;
7. publier le nouveau guide atomiquement.

Un échec de récupération ne doit jamais remplacer le dernier guide valide.

## Configuration prévue

Les paramètres seront fournis par variables d'environnement, notamment :

- `SCRAPE_TIME` : heure locale du scraping quotidien, par exemple `03:00` ;
- `TIMEZONE` : fuseau utilisé par le scheduler, par exemple `Europe/Paris` ;
- `DAYS` : taille de la fenêtre glissante, par défaut 14 ;
- `LISTEN_ADDR` : adresse HTTP, par défaut `0.0.0.0:8080` ;
- `DATABASE` : chemin SQLite ;
- `EPG_FILE` : fichier XMLTV publié.

## HTTP

Endpoints prévus :

- `GET /epg.xml` — guide XMLTV ;
- `GET /health` — santé minimale du service ;
- `GET /status` — état du dernier cycle et prochain cycle.

## Architecture

Voir [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md).

## Développement

Le projet est volontairement structuré en petits composants indépendants :

```
cmd/m6-epg/         entrée du programme
internal/config/    configuration
internal/scheduler/ scheduler
internal/scraper/   source M6 PRO
internal/store/     SQLite
internal/xmltv/     génération/validation XMLTV
internal/httpapi/   HTTP
```

L'implémentation fonctionnelle du scraper sera ajoutée après consolidation des observations faites sur la source M6 PRO.
