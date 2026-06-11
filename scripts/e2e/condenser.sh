#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CONDENSER_BIN="${ROOT_DIR}/bin/condenser"
E2E_WORK_DIR="${E2E_WORK_DIR:-/tmp/raind-condenser-e2e}"
LOG_PATH="${E2E_WORK_DIR}/condenser.log"
PID=""

log() {
  printf '==> %s\n' "$*"
}

fail() {
  printf 'error: %s\n' "$*" >&2
  if [[ -f "${LOG_PATH}" ]]; then
    printf '%s\n' '--- condenser log ---' >&2
    tail -120 "${LOG_PATH}" >&2 || true
  fi
  exit 1
}

have_cmd() {
  command -v "$1" >/dev/null 2>&1
}

sudo_cmd() {
  if [[ "${EUID}" -eq 0 ]]; then
    "$@"
    return
  fi

  if ! have_cmd sudo; then
    fail "sudo is required for condenser e2e"
  fi
  sudo -n "$@"
}

require_workshop() {
  if [[ "${ROOT_DIR}" != /project* ]]; then
    fail "condenser e2e must run inside Workshop. use: workshop run raind-dev -- test-condenser-e2e"
  fi
}

build_condenser() {
  log "build all raind components"
  cd "${ROOT_DIR}"
  ./scripts/build.sh build
  [[ -x "${CONDENSER_BIN}" ]] || fail "missing built condenser binary: ${CONDENSER_BIN}"
}

prepare_runtime() {
  log "prepare condenser runtime prerequisites"
  sudo_cmd mkdir -p \
    /etc/raind/log \
    /etc/raind/cert/web \
    /etc/raind/store \
    /etc/raind/container \
    /etc/raind/image/layers \
    /var/log/raind \
    /sys/fs/cgroup/raind
  sudo_cmd chmod 0755 /etc/raind /etc/raind/log /etc/raind/cert /etc/raind/cert/web /etc/raind/store /var/log/raind

  for controller in cpu memory pids io; do
    if sudo_cmd grep -qw "${controller}" /sys/fs/cgroup/raind/cgroup.controllers; then
      sudo_cmd sh -c "echo +${controller} > /sys/fs/cgroup/raind/cgroup.subtree_control 2>/dev/null || true"
    fi
  done
}

assert_port_free() {
  local port="$1"
  if sudo_cmd ss -ltn "( sport = :${port} )" | grep -q ":${port}"; then
    fail "port ${port} is already in use in this Workshop"
  fi
}

cleanup_stale_condenser() {
  sudo_cmd pkill -TERM -f "^${CONDENSER_BIN}$" 2>/dev/null || true
  sleep 0.2
  sudo_cmd pkill -KILL -f "^${CONDENSER_BIN}$" 2>/dev/null || true
}

start_condenser() {
  log "start condenser"
  mkdir -p "${E2E_WORK_DIR}"
  : > "${LOG_PATH}"

  cleanup_stale_condenser

  for port in 7755 7756 7757 7758; do
    assert_port_free "${port}"
  done

  sudo_cmd env PATH="${PATH}" "${CONDENSER_BIN}" >"${LOG_PATH}" 2>&1 &
  PID="$!"
}

stop_condenser() {
  if [[ -n "${PID}" ]]; then
    sudo_cmd kill "${PID}" 2>/dev/null || true
    wait "${PID}" 2>/dev/null || true
    PID=""
  fi
  cleanup_stale_condenser
}

api_curl() {
  local path="$1"
  sudo_cmd curl -sS \
    --connect-timeout 1 \
    --max-time 3 \
    --cert /etc/raind/cert/raindClient.crt \
    --key /etc/raind/cert/raindClient.key \
    --cacert /etc/raind/cert/raind.crt \
    "https://127.0.0.1:7755${path}"
}

wait_ready() {
  log "wait for management API"
  local out="${E2E_WORK_DIR}/ready.json"

  for _ in $(seq 1 400); do
    if ! kill -0 "${PID}" 2>/dev/null; then
      fail "condenser exited before becoming ready"
    fi
    if sudo_cmd test -f /etc/raind/cert/raindClient.crt && api_curl "/v1/images" >"${out}" 2>"${E2E_WORK_DIR}/ready.err"; then
      jq -e '.status == "success"' "${out}" >/dev/null && return
    fi
    sleep 0.1
  done

  cat "${E2E_WORK_DIR}/ready.err" >&2 2>/dev/null || true
  fail "condenser management API did not become ready"
}

assert_api_success() {
  local path="$1"
  local out="${E2E_WORK_DIR}/$(echo "${path}" | tr '/?' '__').json"

  log "GET ${path}"
  api_curl "${path}" >"${out}"
  jq -e '.status == "success"' "${out}" >/dev/null
}

assert_swagger() {
  local out="${E2E_WORK_DIR}/swagger.json"

  log "GET /swagger/doc.json"
  sudo_cmd curl -sk --connect-timeout 1 --max-time 3 \
    "https://127.0.0.1:7758/swagger/doc.json" >"${out}"
  jq -e '.swagger == "2.0"' "${out}" >/dev/null
}

assert_client_cert_required() {
  local code

  log "verify management API requires client certificate"
  code="$(sudo_cmd curl -sk --connect-timeout 1 --max-time 3 \
    -o /dev/null -w '%{http_code}' \
    "https://127.0.0.1:7755/v1/images" || true)"
  if [[ "${code}" == "200" ]]; then
    fail "management API accepted a request without client certificate"
  fi
}

main() {
  require_workshop
  mkdir -p "${E2E_WORK_DIR}"

  build_condenser
  prepare_runtime
  start_condenser
  trap stop_condenser EXIT
  wait_ready

  assert_api_success "/v1/images"
  assert_api_success "/v1/containers"
  assert_api_success "/v1/networks"
  assert_api_success "/v1/pods"
  assert_api_success "/v1/services"
  assert_swagger
  assert_client_cert_required

  log "condenser e2e completed"
}

main "$@"
