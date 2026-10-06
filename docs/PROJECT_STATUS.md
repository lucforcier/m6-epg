# État du projet et travaux réalisés

## Objectif

`m6-epg` est un service Go permanent destiné à produire le guide XMLTV de M6 à partir des données M6 PRO.

Le service est conçu pour fonctionner 24 h/24, conserver les données dans SQLite, maintenir une couverture source supérieure à la fenêtre publiée, publier un guide XMLTV glissant de 14 jours, exposer le guide par HTTP et effectuer automatiquement un refresh quotidien à une heure configurable.

## Source M6 PRO

Les investigations ont confirmé que M6 PRO fournit les programmes par semaines de diffusion samedi à vendredi. Le scraper utilise cette granularité plutôt que de télécharger séparément chaque journée.

La couverture applicative est volontairement plus large que le guide publié : par défaut, la base maintient 21 jours de couverture future. Le scheduler complète uniquement les semaines manquantes.

## Fenêtre glissante

La fenêtre publiée est calculée à chaque génération : aujourd'hui → aujourd'hui + 14 jours.

Fuseaux utilisés :
- source M6 PRO : `Europe/Paris` ;
- sortie XMLTV : `America/Toronto`.

Les horaires XMLTV sont donc convertis vers le fuseau du consommateur.

## Stockage SQLite

La base par défaut est `/data/m6.db`. Elle conserve les programmes récupérés par semaine source. Les semaines déjà présentes ne sont pas téléchargées de nouveau lors d'un cycle normal.

Lors des tests, la base a atteint la semaine M6 2026-44 et environ 631 programmes stockés.

## Génération XMLTV

Le fichier publié par défaut est `/data/m6.xmltv`.

La génération sélectionne la fenêtre publiée, convertit les horaires vers `America/Toronto`, produit `xmltv_ns` lorsque les informations d'épisode sont disponibles, échappe correctement le XML et publie le fichier par remplacement atomique.

Le XML produit a été validé avec `xmllint --noout`.

## HTTP

Endpoints actuels :
- `GET /epg.xml` — dernier XMLTV généré ;
- `HEAD /epg.xml` — vérification HTTP du guide ;
- `GET /healthz` — retourne `ok`.

Le endpoint XMLTV ne déclenche aucun scraping et sert directement le dernier fichier publié.

## Scheduler quotidien

Le scheduler est maintenant fonctionnel.

Configuration :
- `REFRESH_TIME` : heure `HH:MM` ;
- `SCHEDULE_LOCATION` : fuseau du scheduler ;
- valeurs par défaut : `03:00` dans `America/Toronto`.

Le processus effectue un refresh immédiatement au démarrage, puis attend l'heure configurée pour les cycles suivants.

Le test de validation a utilisé `REFRESH_TIME=16:10` et `SCHEDULE_LOCATION=America/Toronto`.

Le cycle du 6 octobre 2026 à 16:10 a été observé avec succès : démarrage du refresh, vérification de couverture, régénération XMLTV, fin du refresh et planification du cycle suivant au 7 octobre 2026 à 16:10. Le serveur HTTP est resté disponible pendant l'attente et après le refresh.

## Enrichissement des métadonnées

Une étude préliminaire a évalué la pertinence de fournisseurs externes pour enrichir le guide M6, notamment TMDB, TVmaze, Wikidata, TheTVDB, Plurimedia et EPG Service.

Conclusion actuelle : **aucun enrichissement externe supplémentaire n'est retenu pour `m6-epg` à ce stade**.

M6 PRO fournit déjà les informations nécessaires à la production du guide. Les fournisseurs commerciaux peuvent offrir des métadonnées TV plus riches, mais ils ne correspondent pas à l'objectif actuel d'une solution autonome et sans dépendance payante.

TMDB, TVmaze et Wikidata restent des pistes possibles pour une future fonctionnalité d'enrichissement, mais elles sont volontairement hors du périmètre actuel.

## Validation réalisée

Le fonctionnement local a été vérifié avec :

```bash
go test ./...
go vet ./...
go build -o m6-epg ./cmd/m6-epg
```

Le service a été lancé sur `127.0.0.1:8081` pendant les tests, car le port `8080` de l'hôte est déjà utilisé par GNS.

Les vérifications ont confirmé : processus permanent actif, HTTP en écoute, `/healthz` fonctionnel, `/epg.xml` accessible, XMLTV valide, scheduler quotidien fonctionnel et prochain cycle correctement planifié.

## Prochaine étape

La prochaine étape est la conteneurisation :
1. Dockerfile pour une image Go minimale ;
2. image publique destinée notamment à GHCR ;
3. volume persistant `/data` ;
4. configuration par variables d'environnement ;
5. `docker-compose.yml` ;
6. healthcheck Docker ;
7. test du cycle complet dans le conteneur.

L'enrichissement externe est explicitement laissé de côté pour le moment.