#!/usr/bin/env bash
#
# Installe ou met à jour le bot AnimeStats sur une machine Debian ou Ubuntu.
#
# Le script est idempotent : relancé sur une version déjà installée, il ne
# télécharge rien, ne redémarre rien et n'écrase jamais la configuration.
# C'est ce qui lui permet de servir aussi de mécanisme de mise à jour
# automatique, déclenché chaque semaine par animestats-update.timer.
#
# Usages :
#   sudo ./install.sh                    depuis une archive de version décompressée
#   curl -fsSL <url>/install.sh | sudo bash
#   curl -fsSL <url>/install.sh | sudo bash -s -- v1.0.0
#
# Options :
#   --auto              mode minuteur : silencieux s'il n'y a rien à faire
#   --no-auto-update    n'active pas le minuteur hebdomadaire à l'installation
#   -h, --help          affiche cette aide

set -euo pipefail

REPO="Sydnec/AnimeStats_DiscordBot"
SERVICE="animestats"
USER_NAME="animestats"

BIN_PATH="/usr/local/bin/${SERVICE}"
LIB_DIR="/usr/local/lib/${SERVICE}"
PREVIOUS_BIN="${LIB_DIR}/previous/${SERVICE}"
CONF_DIR="/etc/${SERVICE}"
CONF_FILE="${CONF_DIR}/${SERVICE}.env"
STATE_DIR="/var/lib/${SERVICE}"
UNIT_DIR="/etc/systemd/system"
UPDATE_TIMER="${SERVICE}-update.timer"
LOCK_FILE="/run/${SERVICE}-install.lock"

# Secondes d'attente avant de conclure qu'un démarrage a échoué. Ramené à 0 par
# les tests, qui simulent systemd.
WAIT_SECONDS="${ANIMESTATS_WAIT_SECONDS:-10}"

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]:-$0}")" 2>/dev/null && pwd || echo .)"

# --- Arguments ---------------------------------------------------------------
VERSION="latest"
AUTO=0
ENABLE_TIMER=1

# usage est écrit en clair plutôt qu'extrait de l'en-tête : le script est aussi
# exécuté via « curl | bash », où le fichier source n'est pas lisible.
usage() {
  cat <<'HELP'
Installe ou met à jour le bot AnimeStats.

Usages :
  sudo ./install.sh                  depuis une archive de version décompressée
  sudo ./install.sh v1.0.0           installe une version précise
  curl -fsSL <url>/install.sh | sudo bash

Options :
  --auto              mode minuteur : silencieux s'il n'y a rien à faire
  --no-auto-update    n'active pas le minuteur hebdomadaire à l'installation
  -h, --help          affiche cette aide
HELP
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --auto)           AUTO=1 ;;
    --no-auto-update) ENABLE_TIMER=0 ;;
    -h|--help)        usage; exit 0 ;;
    -*)               printf 'option inconnue : %s\n' "$1" >&2; exit 2 ;;
    *)                VERSION="$1" ;;
  esac
  shift
done

# --- Journalisation ----------------------------------------------------------
# Les couleurs ne sortent que sur un terminal : sous systemd, la sortie part
# dans le journal, où les séquences d'échappement ne feraient que gêner.
if [[ -t 1 ]]; then
  C_LOG=$'\033[1;34m'; C_WARN=$'\033[1;33m'; C_ERR=$'\033[1;31m'; C_OFF=$'\033[0m'
else
  C_LOG=""; C_WARN=""; C_ERR=""; C_OFF=""
fi

# log signale un événement notable, toujours affiché.
log()  { printf '%s==>%s %s\n' "${C_LOG}" "${C_OFF}" "$*"; }
# info commente le déroulé ; le mode automatique le tait pour qu'une exécution
# sans changement ne laisse aucune trace dans le journal.
info() { [[ ${AUTO} -eq 1 ]] || printf '%s==>%s %s\n' "${C_LOG}" "${C_OFF}" "$*"; }
warn() { printf '%s/!\\%s %s\n' "${C_WARN}" "${C_OFF}" "$*" >&2; }
die()  { printf '%serreur:%s %s\n' "${C_ERR}" "${C_OFF}" "$*" >&2; exit 1; }

[[ ${EUID} -eq 0 ]] || die "à lancer en root (sudo ./install.sh)"
command -v systemctl >/dev/null || die "systemd est requis"

# --- Verrou ------------------------------------------------------------------
# Empêche le minuteur et une exécution manuelle de se marcher dessus.
if command -v flock >/dev/null; then
  exec 9>"${LOCK_FILE}"
  flock -w 300 9 || die "une autre installation est déjà en cours"
fi

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64)  echo amd64 ;;
    aarch64|arm64) echo arm64 ;;
    *) die "architecture non prise en charge : $(uname -m)" ;;
  esac
}

# resolve_latest interroge l'API GitHub. Renvoie une chaîne vide si elle est
# injoignable, à charge de l'appelant de décider si c'est fatal.
resolve_latest() {
  curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null \
    | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -1
}

# installed_version lit la version du binaire en place, vide s'il n'y en a pas.
installed_version() {
  [[ -x "${BIN_PATH}" ]] || return 0
  "${BIN_PATH}" -version 2>/dev/null | awk '{print $2}'
}

# service_can_start indique si la configuration comporte un token.
service_can_start() {
  [[ -f "${CONF_FILE}" ]] && grep -qE '^DISCORD_TOKEN=.+' "${CONF_FILE}"
}

# replace_file installe un fichier en écrasant sa destination par renommage.
#
# Écrire directement sur la destination serait une erreur pour deux raisons :
# Linux refuse d'ouvrir en écriture un exécutable en cours d'exécution (ETXTBSY),
# ce qui ferait échouer toute mise à jour du bot tant qu'il tourne ; et réécrire
# ce script pendant qu'il s'exécute corromprait sa propre lecture, bash lisant un
# script au fil de l'eau. Le renommage, lui, est atomique : le processus en cours
# garde l'ancien contenu jusqu'à son redémarrage.
replace_file() {
  local src="$1" dst="$2" mode="$3" tmp
  install -d -o root -g root -m 0755 "$(dirname "${dst}")"
  tmp="$(mktemp "${dst}.XXXXXX")"
  cat "${src}" > "${tmp}"
  chmod "${mode}" "${tmp}"
  chown root:root "${tmp}" 2>/dev/null || true
  mv -f "${tmp}" "${dst}"
}

# wait_active attend que le service soit actif, dans la limite impartie.
wait_active() {
  local waited=0
  while :; do
    systemctl is-active --quiet "${SERVICE}" && return 0
    (( waited >= WAIT_SECONDS )) && return 1
    waited=$(( waited + 1 ))
    sleep 1
  done
}

ARCH="$(detect_arch)"
CHANGED=0
STAGE=""

# --- Provenance du binaire ---------------------------------------------------
# Une archive de version décompressée se reconnaît à la présence conjointe du
# binaire et de l'unité systemd. Exiger les deux évite qu'un exécutable nommé
# « animestats » traînant dans le répertoire courant ne soit réinstallé par
# mégarde à la place de la dernière version.
if [[ -x "${SCRIPT_DIR}/${SERVICE}" && -f "${SCRIPT_DIR}/${SERVICE}.service" ]]; then
  info "archive locale détectée dans ${SCRIPT_DIR}"
  STAGE="${SCRIPT_DIR}"
else
  command -v curl >/dev/null || die "curl est requis"
  command -v tar  >/dev/null || die "tar est requis"

  if [[ "${VERSION}" == "latest" ]]; then
    VERSION="$(resolve_latest)"
    if [[ -z "${VERSION}" ]]; then
      # Une coupure réseau passagère ne doit pas faire passer le minuteur en
      # échec jusqu'à la semaine suivante.
      if [[ ${AUTO} -eq 1 ]]; then
        warn "API GitHub injoignable, vérification reportée"
        exit 0
      fi
      die "impossible de déterminer la dernière version publiée"
    fi
  fi

  # Comparaison avant téléchargement : inutile de tirer plusieurs mégaoctets
  # chaque semaine pour constater qu'il n'y a rien de neuf.
  if [[ "$(installed_version)" == "${VERSION}" ]]; then
    info "déjà en ${VERSION}"
    if service_can_start && ! systemctl is-active --quiet "${SERVICE}"; then
      # Restart=on-failure ne relance plus un service qui a épuisé son quota de
      # tentatives : sans ce filet, le bot resterait muet jusqu'au passage
      # suivant.
      log "service à l'arrêt alors qu'il devrait tourner, relance"
      systemctl start "${SERVICE}" || die "le service refuse de démarrer"
    fi
    exit 0
  fi

  TMP="$(mktemp -d)"
  trap 'rm -rf "${TMP}"' EXIT

  ARCHIVE="${SERVICE}_${VERSION}_linux_${ARCH}.tar.gz"
  BASE_URL="https://github.com/${REPO}/releases/download/${VERSION}"

  log "téléchargement de ${ARCHIVE}"
  curl -fsSL -o "${TMP}/${ARCHIVE}" "${BASE_URL}/${ARCHIVE}" \
    || die "téléchargement impossible (version ${VERSION} et architecture ${ARCH} publiées ?)"
  curl -fsSL -o "${TMP}/SHA256SUMS" "${BASE_URL}/SHA256SUMS" \
    || die "empreintes indisponibles"

  info "vérification de l'empreinte"
  ( cd "${TMP}" && sha256sum --ignore-missing --check SHA256SUMS >/dev/null ) \
    || die "empreinte invalide, installation interrompue"

  tar -xzf "${TMP}/${ARCHIVE}" -C "${TMP}"
  STAGE="${TMP}/${SERVICE}_${VERSION}_linux_${ARCH}"
  [[ -x "${STAGE}/${SERVICE}" ]] || die "archive inattendue : binaire introuvable"
fi

NEW_SUM="$(sha256sum "${STAGE}/${SERVICE}" | cut -d' ' -f1)"

# --- Utilisateur et arborescence ---------------------------------------------
if ! getent passwd "${USER_NAME}" >/dev/null; then
  log "création de l'utilisateur système ${USER_NAME}"
  adduser --system --group --home "${STATE_DIR}" --no-create-home \
          --shell /usr/sbin/nologin "${USER_NAME}" >/dev/null
fi

install -d -o root -g "${USER_NAME}" -m 0750 "${CONF_DIR}"
install -d -o "${USER_NAME}" -g "${USER_NAME}" -m 0750 "${STATE_DIR}"
install -d -o root -g root -m 0755 "${LIB_DIR}"

if [[ ! -f "${CONF_FILE}" ]]; then
  log "création du gabarit de configuration ${CONF_FILE}"
  install -o root -g "${USER_NAME}" -m 0640 /dev/null "${CONF_FILE}"
  cat > "${CONF_FILE}" <<'ENVFILE'
# Configuration du bot AnimeStats.
# Le seul réglage obligatoire est le token Discord.
DISCORD_TOKEN=

# Décommentez pour ajuster (valeurs par défaut indiquées).
#DB_PATH=/var/lib/animestats/animestats.db
#TZ=Europe/Paris
#OP_ED_MINUTES=3
#ANILIST_MAX_PAGES=20
#ANILIST_CACHE_TTL=0
#LOG_LEVEL=info
#SEND_RECAP_ON_FOLLOW=true
#CATCHUP_MISSED_RUNS=true
#MONTHLY_CRON=0 10 1 * *
#YEARLY_CRON=0 12 1 1 *
ENVFILE
  chown root:"${USER_NAME}" "${CONF_FILE}"
  chmod 0640 "${CONF_FILE}"
fi

# --- Validation avant remplacement -------------------------------------------
# La configuration est vérifiée avec le binaire candidat, avant qu'il ne
# remplace celui en place : une version fautive ne doit jamais s'installer.
CAN_START=0
if service_can_start; then
  CAN_START=1
  info "vérification de la configuration"
  # shellcheck disable=SC1090
  if ! ( set -a; . "${CONF_FILE}"; set +a; "${STAGE}/${SERVICE}" -check >/dev/null ); then
    die "la configuration est invalide, rien n'a été modifié"
  fi
fi

# --- Binaire ------------------------------------------------------------------
CURRENT_SUM=""
[[ -x "${BIN_PATH}" ]] && CURRENT_SUM="$(sha256sum "${BIN_PATH}" | cut -d' ' -f1)"

if [[ "${CURRENT_SUM}" == "${NEW_SUM}" ]]; then
  info "binaire déjà à jour"
else
  # Conserver la version sortante permet un retour arrière si la nouvelle ne
  # démarre pas — indispensable pour une mise à jour non surveillée.
  if [[ -x "${BIN_PATH}" ]]; then
    install -D -o root -g root -m 0755 "${BIN_PATH}" "${PREVIOUS_BIN}"
  fi
  log "installation du binaire dans ${BIN_PATH}"
  replace_file "${STAGE}/${SERVICE}" "${BIN_PATH}" 0755
  CHANGED=1
fi

# --- Unités systemd et copie locale du script ---------------------------------
install_file() {
  local src="$1" dst="$2" mode="$3"
  [[ -f "${src}" ]] || return 0
  [[ "${src}" -ef "${dst}" ]] && return 0
  if ! cmp -s "${src}" "${dst}"; then
    info "installation de ${dst}"
    replace_file "${src}" "${dst}" "${mode}"
    CHANGED=1
  fi
}

UNIT_SRC="${STAGE}/${SERVICE}.service"
[[ -f "${UNIT_SRC}" ]] || UNIT_SRC="${SCRIPT_DIR}/${SERVICE}.service"
[[ -f "${UNIT_SRC}" ]] || die "unité systemd introuvable"
install_file "${UNIT_SRC}" "${UNIT_DIR}/${SERVICE}.service" 0644

# Le minuteur exécute cette copie, et non un script téléchargé à chaud : faire
# tourner en root, chaque semaine et sans surveillance, un fichier tiré de la
# branche principale reviendrait à exécuter n'importe quel commit de celle-ci.
install_file "${STAGE}/install.sh" "${LIB_DIR}/install.sh" 0755

# Retenu avant installation : le minuteur n'est activé qu'à sa toute première
# pose, pour ne jamais réactiver un automatisme volontairement désactivé.
TIMER_EXISTED=0
[[ -f "${UNIT_DIR}/${UPDATE_TIMER}" ]] && TIMER_EXISTED=1

install_file "${STAGE}/${SERVICE}-update.service" "${UNIT_DIR}/${SERVICE}-update.service" 0644
install_file "${STAGE}/${UPDATE_TIMER}" "${UNIT_DIR}/${UPDATE_TIMER}" 0644

systemctl daemon-reload

if [[ -f "${UNIT_DIR}/${UPDATE_TIMER}" && ${TIMER_EXISTED} -eq 0 ]]; then
  if [[ ${ENABLE_TIMER} -eq 1 ]]; then
    log "activation des mises à jour automatiques (hebdomadaires)"
    systemctl enable --now "${UPDATE_TIMER}" >/dev/null
  else
    info "mises à jour automatiques non activées (--no-auto-update)"
  fi
fi

# --- Démarrage ----------------------------------------------------------------
if [[ ${CAN_START} -eq 0 ]]; then
  warn "aucun token dans ${CONF_FILE} : le service n'est pas démarré."
  warn "Renseignez DISCORD_TOKEN puis lancez :"
  warn "  systemctl enable --now ${SERVICE}"
  exit 0
fi

systemctl enable "${SERVICE}" >/dev/null

if [[ ${CHANGED} -eq 0 ]] && systemctl is-active --quiet "${SERVICE}"; then
  info "aucun changement, service déjà actif"
  exit 0
fi

log "redémarrage du service"
systemctl restart "${SERVICE}"

if wait_active; then
  log "service actif — $("${BIN_PATH}" -version)"
  [[ ${AUTO} -eq 1 ]] || log "journal : journalctl -u ${SERVICE} -f"
  exit 0
fi

# --- Retour arrière -----------------------------------------------------------
warn "le service n'a pas démarré après l'installation"
if command -v journalctl >/dev/null; then
  journalctl -u "${SERVICE}" -n 30 --no-pager || true
fi

[[ -x "${PREVIOUS_BIN}" ]] || die "aucune version précédente disponible, le service reste arrêté"

warn "retour à la version précédente"
replace_file "${PREVIOUS_BIN}" "${BIN_PATH}" 0755
systemctl restart "${SERVICE}" || true

if wait_active; then
  die "version $("${BIN_PATH}" -version) restaurée et fonctionnelle : la version installée était fautive"
fi
die "le service ne démarre ni avec la nouvelle version ni avec la précédente"
