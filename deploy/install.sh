#!/usr/bin/env bash
#
# Installe ou met à jour le bot AnimeStats sur une machine Debian ou Ubuntu.
# Le script est idempotent : le relancer sur une version déjà installée ne
# change rien et n'écrase jamais le fichier de configuration.
#
# Deux usages :
#   1. depuis une archive de version décompressée :  sudo ./install.sh
#   2. directement depuis GitHub :
#        curl -fsSL https://raw.githubusercontent.com/Sydnec/AnimeStats_DiscordBot/main/deploy/install.sh \
#          | sudo bash -s -- v1.0.0
#
# Sans argument de version, la dernière version publiée est utilisée.

set -euo pipefail

REPO="Sydnec/AnimeStats_DiscordBot"
SERVICE="animestats"
USER_NAME="animestats"
BIN_PATH="/usr/local/bin/${SERVICE}"
CONF_DIR="/etc/${SERVICE}"
CONF_FILE="${CONF_DIR}/${SERVICE}.env"
STATE_DIR="/var/lib/${SERVICE}"
UNIT_PATH="/etc/systemd/system/${SERVICE}.service"

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
VERSION="${1:-latest}"

log()  { printf '\033[1;34m==>\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m/!\\\033[0m %s\n' "$*" >&2; }
die()  { printf '\033[1;31merreur:\033[0m %s\n' "$*" >&2; exit 1; }

[[ ${EUID} -eq 0 ]] || die "à lancer en root (sudo ./install.sh)"
command -v systemctl >/dev/null || die "systemd est requis"

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) echo amd64 ;;
    aarch64|arm64) echo arm64 ;;
    *) die "architecture non prise en charge : $(uname -m)" ;;
  esac
}

# resolve_latest interroge l'API GitHub pour connaître la dernière version.
resolve_latest() {
  command -v curl >/dev/null || die "curl est requis pour résoudre la dernière version"
  local tag
  tag="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
        | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -1)"
  [[ -n "${tag}" ]] || die "impossible de déterminer la dernière version publiée"
  echo "${tag}"
}

ARCH="$(detect_arch)"
STAGE=""

# Une archive décompressée contient déjà le binaire : inutile de télécharger.
if [[ -x "${SCRIPT_DIR}/${SERVICE}" ]]; then
  log "binaire local détecté dans ${SCRIPT_DIR}"
  STAGE="${SCRIPT_DIR}"
else
  [[ "${VERSION}" == "latest" ]] && VERSION="$(resolve_latest)"
  command -v curl >/dev/null || die "curl est requis"
  command -v tar  >/dev/null || die "tar est requis"

  TMP="$(mktemp -d)"
  trap 'rm -rf "${TMP}"' EXIT

  ARCHIVE="${SERVICE}_${VERSION}_linux_${ARCH}.tar.gz"
  BASE_URL="https://github.com/${REPO}/releases/download/${VERSION}"

  log "téléchargement de ${ARCHIVE}"
  curl -fsSL -o "${TMP}/${ARCHIVE}" "${BASE_URL}/${ARCHIVE}" \
    || die "téléchargement impossible (version ${VERSION} et architecture ${ARCH} publiées ?)"
  curl -fsSL -o "${TMP}/SHA256SUMS" "${BASE_URL}/SHA256SUMS" \
    || die "empreintes indisponibles"

  log "vérification de l'empreinte"
  ( cd "${TMP}" && sha256sum --ignore-missing --check SHA256SUMS ) \
    || die "empreinte invalide, installation interrompue"

  tar -xzf "${TMP}/${ARCHIVE}" -C "${TMP}"
  STAGE="${TMP}/${SERVICE}_${VERSION}_linux_${ARCH}"
  [[ -x "${STAGE}/${SERVICE}" ]] || die "archive inattendue : binaire introuvable"
fi

NEW_SUM="$(sha256sum "${STAGE}/${SERVICE}" | cut -d' ' -f1)"

# --- Utilisateur et arborescence -------------------------------------------
if ! getent passwd "${USER_NAME}" >/dev/null; then
  log "création de l'utilisateur système ${USER_NAME}"
  adduser --system --group --home "${STATE_DIR}" --no-create-home \
          --shell /usr/sbin/nologin "${USER_NAME}" >/dev/null
fi

install -d -o root -g "${USER_NAME}" -m 0750 "${CONF_DIR}"
install -d -o "${USER_NAME}" -g "${USER_NAME}" -m 0750 "${STATE_DIR}"

FRESH_CONFIG=0
if [[ ! -f "${CONF_FILE}" ]]; then
  FRESH_CONFIG=1
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

# --- Validation avant remplacement ------------------------------------------
# La configuration est vérifiée avec le binaire candidat, avant qu'il ne
# remplace celui en place : une version fautive ne doit jamais s'installer.
CAN_START=1
if [[ ${FRESH_CONFIG} -eq 1 ]] || ! grep -qE '^DISCORD_TOKEN=.+' "${CONF_FILE}"; then
  CAN_START=0
else
  log "vérification de la configuration"
  # shellcheck disable=SC1090
  if ! ( set -a; . "${CONF_FILE}"; set +a; "${STAGE}/${SERVICE}" -check >/dev/null ); then
    die "la configuration est invalide, rien n'a été modifié"
  fi
fi

# --- Binaire ----------------------------------------------------------------
CURRENT_SUM=""
[[ -x "${BIN_PATH}" ]] && CURRENT_SUM="$(sha256sum "${BIN_PATH}" | cut -d' ' -f1)"

if [[ "${CURRENT_SUM}" == "${NEW_SUM}" ]]; then
  log "binaire déjà à jour"
else
  log "installation du binaire dans ${BIN_PATH}"
  # install(1) remplace de façon atomique : le service en cours continue de
  # tourner sur l'ancien code jusqu'à son redémarrage.
  install -o root -g root -m 0755 "${STAGE}/${SERVICE}" "${BIN_PATH}"
fi

# --- Unité systemd ----------------------------------------------------------
UNIT_SRC="${STAGE}/${SERVICE}.service"
[[ -f "${UNIT_SRC}" ]] || UNIT_SRC="${SCRIPT_DIR}/${SERVICE}.service"
[[ -f "${UNIT_SRC}" ]] || die "unité systemd introuvable"

if ! cmp -s "${UNIT_SRC}" "${UNIT_PATH}"; then
  log "installation de l'unité ${UNIT_PATH}"
  install -o root -g root -m 0644 "${UNIT_SRC}" "${UNIT_PATH}"
fi
systemctl daemon-reload

# --- Démarrage --------------------------------------------------------------
if [[ ${CAN_START} -eq 0 ]]; then
  warn "aucun token dans ${CONF_FILE} : le service n'est pas démarré."
  warn "Renseignez DISCORD_TOKEN puis lancez :"
  warn "  systemctl enable --now ${SERVICE}"
  exit 0
fi

log "démarrage du service"
systemctl enable "${SERVICE}" >/dev/null
systemctl restart "${SERVICE}"

sleep 2
if systemctl is-active --quiet "${SERVICE}"; then
  log "service actif — $("${BIN_PATH}" -version)"
  log "journal : journalctl -u ${SERVICE} -f"
else
  journalctl -u "${SERVICE}" -n 30 --no-pager || true
  die "le service n'a pas démarré"
fi
