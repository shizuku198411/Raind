#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

usage() {
  cat <<'EOF'
Usage:
  ./scripts/run.sh gateway
  ./scripts/run.sh vite

Modes:
  gateway  Start API gateway (node server/index.js)
  vite     Start Vite dev server (npm run dev)

Optional environment overrides:
  WEBUI_HOST
  WEBUI_PORT
  RAIND_UI_SOCKET_PATH
  RAIND_WEBUI_DEV_API_TARGET
  RAIND_WEBUI_DEV_HTTPS
  RAIND_WEBUI_DEV_TLS_CERT
  RAIND_WEBUI_DEV_TLS_KEY
EOF
}

MODE="${1:-}"
if [[ -z "${MODE}" ]]; then
  usage
  exit 1
fi

cd "${PROJECT_DIR}"

case "${MODE}" in
  gateway)
    TESTDB_CONTAINER_NAME="${RAIND_WEBUI_TESTDB_CONTAINER_NAME:-raind-webui-testdb}"

    cleanup_testdb() {
      set +e
      raind container stop "${TESTDB_CONTAINER_NAME}" >/dev/null 2>&1 || true
      raind container rm "${TESTDB_CONTAINER_NAME}" >/dev/null 2>&1 || true
      set -e
    }

    trap cleanup_testdb EXIT

    raind container run --name "${TESTDB_CONTAINER_NAME}" \
      -e MYSQL_ROOT_PASSWORD=root \
      -e MYSQL_DATABASE=raind_webui \
      -e MYSQL_USER=raind_webui \
      -e MYSQL_PASSWORD=raind_webui \
      -p 33067:3306 \
      mysql

    sleep "${RAIND_WEBUI_DB_WAIT_SECONDS:-30}"

    WEBUI_HOST="${WEBUI_HOST:-127.0.0.1}" \
    WEBUI_PORT="${WEBUI_PORT:-18081}" \
    RAIND_UI_SOCKET_PATH="${RAIND_UI_SOCKET_PATH:-/run/raind/ui.sock}" \
    RAIND_WEBUI_DB_HOST="${RAIND_WEBUI_DB_HOST:-172.17.20.200}" \
    RAIND_WEBUI_DB_PORT="${RAIND_WEBUI_DB_PORT:-33067}" \
    RAIND_WEBUI_DB_NAME="${RAIND_WEBUI_DB_NAME:-raind_webui}" \
    RAIND_WEBUI_DB_USER="${RAIND_WEBUI_DB_USER:-raind_webui}" \
    RAIND_WEBUI_DB_PASSWORD="${RAIND_WEBUI_DB_PASSWORD:-raind_webui}" \
    RAIND_WEBUI_BOOTSTRAP_USER="${RAIND_WEBUI_BOOTSTRAP_USER:-admin}" \
    RAIND_WEBUI_BOOTSTRAP_PASSWORD="${RAIND_WEBUI_BOOTSTRAP_PASSWORD:-P@ssw0rd}" \
    node server/index.js
    ;;
  vite)
    RAIND_WEBUI_DEV_API_TARGET="${RAIND_WEBUI_DEV_API_TARGET:-https://127.0.0.1:18081}" \
    RAIND_WEBUI_DEV_HTTPS="${RAIND_WEBUI_DEV_HTTPS:-true}" \
    RAIND_WEBUI_DEV_TLS_CERT="${RAIND_WEBUI_DEV_TLS_CERT:-/etc/raind/cert/web/raindWeb.crt}" \
    RAIND_WEBUI_DEV_TLS_KEY="${RAIND_WEBUI_DEV_TLS_KEY:-/etc/raind/cert/web/raindWeb.key}" \
    npm run dev
    ;;
  -h|--help|help)
    usage
    ;;
  *)
    echo "Unknown mode: ${MODE}" >&2
    usage
    exit 1
    ;;
esac
