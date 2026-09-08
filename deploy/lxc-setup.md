# Déployer AnimeStats dans un LXC Proxmox

De la création du conteneur au bot en fonctionnement. Toutes les commandes de
la première partie se lancent **sur l'hôte Proxmox**, en root.

Le bot est un binaire statique sans dépendance : le conteneur n'a besoin ni de
Node, ni de Go, ni d'aucun environnement d'exécution.

---

## 1. Créer le conteneur

Récupérez un modèle Debian 13 si vous n'en avez pas déjà un :

```bash
pveam update
pveam available --section system | grep debian-13
pveam download local debian-13-standard_13.0-1_amd64.tar.zst
```

Choisissez un identifiant libre (`pct list` pour voir les existants), puis :

```bash
pct create 110 local:vztmpl/debian-13-standard_13.0-1_amd64.tar.zst \
  --hostname animestats \
  --cores 1 \
  --memory 512 \
  --swap 256 \
  --rootfs local-lvm:2 \
  --net0 name=eth0,bridge=vmbr0,ip=dhcp \
  --features nesting=1 \
  --unprivileged 1 \
  --onboot 1 \
  --start 1
```

Points importants :

- **`--features nesting=1`** : indispensable. Sans cette option, systemd ne peut
  pas créer les espaces de noms utilisés par le durcissement de l'unité, et le
  service échoue avec `status=226/NAMESPACE`.
- **`--unprivileged 1`** : le conteneur n'a aucun besoin de privilèges.
- **`--onboot 1`** : le conteneur redémarre avec l'hôte.
- 512 Mo de mémoire et 2 Go de disque sont confortables ; le bot consomme
  quelques dizaines de mégaoctets.
- Pour une adresse fixe, remplacez `ip=dhcp` par
  `ip=192.168.1.50/24,gw=192.168.1.1`.

Entrez dans le conteneur :

```bash
pct enter 110
```

---

## 2. Préparer le conteneur

Tout ce qui suit se passe **dans le conteneur**.

```bash
apt update && apt upgrade -y
apt install -y ca-certificates curl tzdata
timedatectl set-timezone Europe/Paris
```

`ca-certificates` est nécessaire pour joindre Discord et AniList en HTTPS.
`tzdata` est facultatif — le binaire embarque sa propre base de fuseaux — mais
utile pour que l'horodatage du journal soit lisible.

---

## 3. Installer le bot

```bash
curl -fsSL https://raw.githubusercontent.com/Sydnec/AnimeStats_DiscordBot/main/deploy/install.sh \
  | bash
```

Le script télécharge la dernière version publiée, **vérifie son empreinte
SHA-256**, crée l'utilisateur système `animestats`, installe le binaire dans
`/usr/local/bin`, pose l'unité systemd et écrit un gabarit de configuration.
Il s'arrête avant de démarrer le service tant que le token n'est pas renseigné.

Pour installer une version précise :

```bash
curl -fsSL https://raw.githubusercontent.com/Sydnec/AnimeStats_DiscordBot/main/deploy/install.sh \
  | bash -s -- v1.0.0
```

---

## 4. Renseigner le token

```bash
nano /etc/animestats/animestats.env
```

Renseignez `DISCORD_TOKEN` avec le token du bot (portail développeur Discord →
votre application → **Bot** → *Reset Token*). Le fichier appartient à
`root:animestats` en `0640` : le service peut le lire, personne d'autre.

Puis démarrez :

```bash
systemctl enable --now animestats
systemctl status animestats
journalctl -u animestats -f
```

---

## 5. Reprendre la base de l'ancien bot

Pour conserver les abonnés déjà enregistrés, copiez le fichier SQLite de
l'installation Node vers le nouvel emplacement, service arrêté :

```bash
systemctl stop animestats

# Depuis l'ancienne machine, par exemple :
#   scp /chemin/vers/AnimeStats_DiscordBot/data/animestats.db root@<lxc>:/tmp/

install -o animestats -g animestats -m 0640 \
  /tmp/animestats.db /var/lib/animestats/animestats.db

systemctl start animestats
```

Le schéma est inchangé : aucune migration n'est nécessaire, et l'ancien bot
peut reprendre la main sur le même fichier en cas de retour arrière.

Vérifiez que les abonnés sont bien là :

```bash
sudo -u animestats /usr/local/bin/animestats -check
```

> Si l'ancienne base est accompagnée de fichiers `animestats.db-wal` et
> `animestats.db-shm`, copiez-les aussi, ou fusionnez-les au préalable avec
> `sqlite3 animestats.db "PRAGMA wal_checkpoint(TRUNCATE);"`.

---

## 6. Mettre à jour

Relancez simplement le script d'installation :

```bash
curl -fsSL https://raw.githubusercontent.com/Sydnec/AnimeStats_DiscordBot/main/deploy/install.sh \
  | bash
```

Il ne fait rien si la version en place est déjà la dernière, n'écrase jamais
`animestats.env`, et refuse de redémarrer le service si la configuration est
invalide.

---

## 7. Dépannage

**Le service échoue avec `status=226/NAMESPACE`**

Le conteneur n'accorde pas un espace de noms réclamé par le durcissement.
Vérifiez d'abord que `nesting` est actif, depuis l'hôte :

```bash
pct set 110 --features nesting=1
pct reboot 110
```

Si le problème persiste, commentez les directives une à une dans
`/etc/systemd/system/animestats.service`, dans cet ordre :
`ProtectKernelLogs`, `ProtectControlGroups`, `RestrictNamespaces`,
`ProtectSystem`. Après chaque essai :

```bash
systemctl daemon-reload && systemctl restart animestats
```

**Aucun message privé ne part**

Discord refuse d'écrire à un utilisateur qui a bloqué l'application ou fermé
ses MP. Le journal le dit explicitement :

```bash
journalctl -u animestats | grep -i "message privé"
```

**Les commandes n'apparaissent pas dans Discord**

Les commandes globales mettent jusqu'à une heure à se propager. Pour tester
immédiatement, renseignez `DEV_GUILD_ID` avec l'identifiant d'un serveur de
test : la publication y est instantanée.

Vérifiez aussi, dans le portail développeur, que le champ *Interactions
Endpoint URL* est **vide** : s'il est renseigné, Discord cesse d'envoyer les
interactions par la passerelle et le bot ne reçoit plus rien.

**Voir ce que consomme le bot**

```bash
systemctl status animestats
systemd-cgtop -1 --order=memory | head
```

**Vérifier le durcissement**

```bash
systemd-analyze security animestats
```
