# AniList Stats Bot — Guide utilisateur

Ce projet est un petit bot Discord qui envoie, en message privé (MP), un récapitulatif des statistiques de visionnage AniList pour chaque utilisateur qui s'y abonne.

But

- Recevoir automatiquement un récap de ses heures/épisodes regardés sur AniList.
- Choisir la fréquence : quotidien / mensuel / annuel, ou combinaison.
- Gérer facilement l'abonnement depuis le serveur via des commandes `/follow` et `/unfollow`.

Comment suivre

1. Ce service fonctionne en tant qu'application utilisateur (pas comme un bot sur un serveur). Pour autoriser l'application pour ton compte Discord, ouvre ce lien dans ton navigateur :

https://discord.com/oauth2/authorize?client_id=1416068432977203250

Suis les étapes pour autoriser l'application. Une fois autorisée, l'application pourra t'envoyer des messages privés.

2. Pour t'abonner, envoie la commande au bot (dans la fenêtre de conversation de l'application ou via l'interface fournie) :

- `/follow username:TON_PSEUDO mask:111` pour t'abonner à tous les récaps (quotidien, mensuel, annuel).
- Le `mask` est une chaîne de 3 caractères correspondant à `daily monthly yearly`.
  - `1` = activer, `0` = désactiver, `*` = conserver la valeur actuelle.
  - Exemple : `1*0` active quotidien, conserve mensuel, désactive annuel.

3. Pour te désabonner, utilise : `/unfollow`.

Notes pratiques

- Le bot envoie les messages en MP. Assure-toi que l'option "Autoriser les MP" aux membres du serveur est activée pour le bot.
- Si tu veux changer la fréquence, relance `/follow` avec un `mask` adapté (ex: `010` pour seulement mensuel).

Commande /recap

- `/recap days:NB [username:PSEUDO]` — Envoie immédiatement en MP un récapitulatif pour les `NB` derniers jours.
  - `days` (obligatoire) : nombre de jours à remonter (ex: `7` pour une semaine).
  - `username` (optionnel) : pseudo AniList à utiliser. Si omis, le bot utilisera le pseudo enregistré pour ton `discord_user_id` (si tu t'es déjà abonné via `/follow`).
  - Exemple : `/recap days:30` — envoie un récap des 30 derniers jours pour le pseudo enregistré.

Recommandation pour suivi en temps réel

- Si tu veux un suivi plus fin et en temps réel (par épisode regardé), il est conseillé de combiner ce service avec l'extension MAL-Sync (navigateur) qui synchronise automatiquement ta progression entre différents services (AniList, MyAnimeList, etc.). Cela garantit que les opérations de lecture sont reflétées rapidement sur AniList et donc dans les rapports envoyés par ce bot.

Lien et installation rapide de MAL-Sync

- Site / extension : https://malsync.moe/ (ou cherche "MAL-Sync" dans le catalogue des extensions de ton navigateur)
- Installation rapide :
  1.  Ouvre la page de l'extension MAL-Sync pour ton navigateur (Chrome / Firefox / Edge).
  2.  Clique sur "Ajouter" / "Installer" puis autorise l'extension.
  3.  Ouvre les options de MAL-Sync et connecte-toi à ton compte AniList (et autres services si souhaité).
  4.  Active la synchronisation automatique pour que MAL-Sync envoie les mises à jour de progression à AniList.

Après installation, les épisodes que tu regardes via des plateformes prises en charge seront rapidement répercutés sur AniList, et donc visibles dans les récapitulatifs envoyés par ce service.

Confidentialité

- Seul ton identifiant Discord et ton pseudo AniList sont stockés localement dans une base SQLite. Les données ne sont pas partagées.

Support

- En cas de problème, contacte l'administrateur du bot avec le message d'erreur des logs.
