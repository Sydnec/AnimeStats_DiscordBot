# AnimeStats — bot Discord de statistiques AniList

Ce bot envoie en message privé un récapitulatif de vos statistiques de
visionnage AniList : temps passé, épisodes vus, jour le plus actif et liste des
animes suivis.

Écrit en Go, il se déploie sous la forme d'un binaire statique unique — aucun
environnement d'exécution à installer sur la machine cible.

---

# Guide utilisateur

## Autoriser l'application

Le bot fonctionne en **application utilisateur** : vous l'autorisez sur votre
compte Discord, sans avoir à l'inviter sur un serveur.

<https://discord.com/oauth2/authorize?client_id=1416068432977203250>

Une fois l'autorisation donnée, les commandes ci-dessous sont disponibles
partout, et le bot peut vous écrire en privé.

## S'abonner — `/follow`

```
/follow username:VOTRE_PSEUDO mask:11
```

| Option | Rôle |
| --- | --- |
| `username` | Votre pseudo AniList |
| `mask` | Deux caractères : le premier pour le récap **mensuel**, le second pour l'**annuel** |

Chaque caractère du masque vaut :

- `1` — activer
- `0` — désactiver
- `*` — conserver le réglage actuel

Quelques exemples :

| Masque | Effet |
| --- | --- |
| `10` | Mensuel seulement |
| `01` | Annuel seulement |
| `11` | Les deux |
| `*1` | Ajoute l'annuel, laisse le mensuel tel quel |
| `00` | Désactive les deux envois, sans supprimer votre pseudo |

Un récapitulatif vous est envoyé immédiatement pour chaque fréquence activée.

## Se désabonner — `/unfollow`

Supprime votre abonnement et les données associées.

## Récap à la demande — `/recap`

```
/recap days:30
```

| Option | Rôle |
| --- | --- |
| `days` | Nombre de jours à couvrir, de 1 à 365 |
| `username` | Pseudo AniList à utiliser (facultatif : par défaut le vôtre, s'il est enregistré) |

### Si le récap n'arrive pas

La réponse du bot indique ce qui a échoué, et donc s'il sert à quelque chose de
réessayer.

| Réponse | Cause | Quoi faire |
| --- | --- | --- |
| *Pseudo AniList introuvable* | Le compte AniList a été renommé, supprimé, ou le pseudo est mal orthographié | Corriger l'option `username`, ou refaire un `/follow` |
| *AniList ne répond pas* | Panne d'AniList, ou quota de requêtes dépassé | Réessayer dans quelques minutes |
| *Le récapitulatif a mis trop de temps* | Le calcul a dépassé deux minutes, typiquement sur une longue période | Réessayer, ou demander moins de jours |
| *Impossible de vous écrire en message privé* | Vos MP sont fermés pour ce bot | Ouvrir vos messages privés, puis réessayer |
| *Impossible de produire le récapitulatif* | Cause imprévue | Consulter le journal du service (`journalctl -u animestats`) |

## Quand partent les récapitulatifs

| Fréquence | Envoi | Période couverte |
| --- | --- | --- |
| Mensuel | Le 1er de chaque mois à 10 h | Le mois calendaire précédent |
| Annuel | Le 1er janvier à 12 h | L'année précédente |

Les heures suivent le fuseau configuré sur le serveur, `Europe/Paris` par
défaut.

## Bon à savoir

- **Ouvrez vos messages privés.** Sans cela, le bot ne peut rien vous envoyer.
- **Seules les activités publiques d'AniList sont visibles.** Une liste privée
  produira un récapitulatif vide.
- **Le temps est calculé au plus juste.** Trois minutes sont retirées de chaque
  épisode pour tenir compte de l'opening et de l'ending.
- **Pour un suivi fidèle**, l'extension navigateur
  [MAL-Sync](https://malsync.moe/) répercute automatiquement votre progression
  sur AniList au fil de vos visionnages.

## Confidentialité

Seuls votre identifiant Discord et votre pseudo AniList sont enregistrés, dans
une base SQLite locale. Rien n'est partagé avec un tiers. `/unfollow` efface
votre ligne.

---

# Guide de l'hébergeur

## Déploiement

La procédure complète, de la création du conteneur LXC Proxmox jusqu'au bot en
fonctionnement, est décrite dans **[deploy/lxc-setup.md](deploy/lxc-setup.md)**.

En résumé, sur une machine Debian ou Ubuntu :

```bash
curl -fsSL https://raw.githubusercontent.com/Sydnec/AnimeStats_DiscordBot/main/deploy/install.sh | sudo bash
# renseignez DISCORD_TOKEN dans /etc/animestats/animestats.env
sudo systemctl enable --now animestats
```

Les mises à jour sont ensuite **automatiques** : un minuteur systemd vérifie
chaque dimanche vers 4 h s'il existe une version plus récente et l'applique. Si
le service ne redémarre pas, la version précédente est restaurée et le bot reste
en ligne. Une vérification sans nouveauté ne télécharge rien, ne redémarre rien
et ne laisse aucune trace dans le journal.

```bash
systemctl list-timers animestats-update.timer     # prochaine échéance
systemctl start animestats-update.service         # vérifier tout de suite
systemctl disable --now animestats-update.timer   # repasser en manuel
```

Le même script sert aux mises à jour manuelles et au retour arrière
(`… | sudo bash -s -- v1.0.0`) : il n'écrase jamais la configuration ni la base,
et refuse d'installer une version dont la configuration ne passe pas la
vérification.

## Configuration

Toutes les options se règlent par variables d'environnement, fournies en
production par `/etc/animestats/animestats.env`. En développement, un fichier
`.env` à la racine fait l'affaire — voir [`.env.example`](.env.example).

| Variable | Défaut | Rôle |
| --- | --- | --- |
| `DISCORD_TOKEN` | — | **Requis.** Token du bot Discord |
| `DB_PATH` | `/var/lib/animestats/animestats.db` | Base SQLite ; le dossier est créé au besoin |
| `TZ` | `Europe/Paris` | Fuseau des tâches planifiées et du découpage par jour |
| `OP_ED_MINUTES` | `3` | Minutes retirées par épisode pour l'opening et l'ending |
| `ANILIST_MAX_PAGES` | `20` | Garde-fou de pagination, par pages de 50 activités (maximum de l’API) |
| `ANILIST_CACHE_TTL` | `0` | Cache mémoire des activités (`5m`, `90s`, ou un nombre de secondes) |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn` ou `error` |
| `SEND_RECAP_ON_FOLLOW` | `true` | Récapitulatif immédiat après un `/follow` |
| `CATCHUP_MISSED_RUNS` | `true` | Rattrape au démarrage un envoi manqué machine éteinte |
| `MONTHLY_CRON` | `0 10 1 * *` | Planification du récap mensuel |
| `YEARLY_CRON` | `0 12 1 1 *` | Planification du récap annuel |
| `DEV_GUILD_ID` | — | Publie les commandes sur un seul serveur, avec effet immédiat |

Deux drapeaux en ligne de commande :

```bash
animestats -version   # affiche la version
animestats -check     # valide configuration et base, sans se connecter à Discord
```

## Configuration côté Discord

Dans le portail développeur, pour votre application :

- **Installation** → cochez *User Install* (et *Guild Install* si vous voulez
  aussi pouvoir l'inviter sur un serveur), avec `applications.commands` en
  scope.
- **General Information** → le champ *Interactions Endpoint URL* doit rester
  **vide**. S'il est renseigné, Discord cesse d'envoyer les interactions par la
  passerelle et le bot ne reçoit plus rien.
- Aucun *privileged intent* n'est nécessaire.

## Développement

```bash
make test    # tests avec détecteur de compétition
make cover   # couverture (nécessite un SDK Go complet)
make lint    # gofmt + go vet
make build   # binaire statique dans bin/
make run     # lance le bot
make dist    # binaires linux/amd64 et linux/arm64 dans dist/
```

Le code ne dépend d'aucun service externe pour ses tests : le client AniList
est exercé contre un serveur HTTP local, et le moteur de statistiques comme la
mise en forme sont des fonctions pures.

## Architecture

```
cmd/animestats      point d'entrée, câblage, arrêt propre
internal/anilist    client GraphQL : limitation de débit, réessais, pagination
internal/stats      calcul des épisodes et du temps de visionnage (pur)
internal/period     bornes des périodes et en-têtes
internal/render     mise en forme française et troncature (pur)
internal/report     enchaînement récupération → calcul → rendu → envoi
internal/bot        session Discord, commandes, messages privés
internal/scheduler  tâches planifiées et rattrapage
internal/store      persistance SQLite
internal/mask       masque d'abonnement de /follow
internal/config     configuration
deploy/             unités systemd, script d'installation, guide LXC
```

## Publier une version

Poussez un tag `vX.Y.Z` : la CI compile pour `amd64` et `arm64`, vérifie que
les binaires sont bien statiques, et publie les archives accompagnées d'un
fichier `SHA256SUMS` que le script d'installation vérifie.

```bash
git tag -a v1.0.0 -m "Version 1.0.0"
git push origin v1.0.0
```

## Licence

MIT — voir [LICENSE](LICENSE).
