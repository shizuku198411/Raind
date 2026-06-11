#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
RAIND_BIN="${ROOT_DIR}/bin/raind"
CONDENSER_BIN="${ROOT_DIR}/bin/condenser"
E2E_WORK_DIR="${E2E_WORK_DIR:-/tmp/raind-cli-e2e}"
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
    fail "sudo is required for raind e2e"
  fi
  sudo -n "$@"
}

require_workshop() {
  if [[ "${ROOT_DIR}" != /project* ]]; then
    fail "raind e2e must run inside Workshop. use: workshop run raind-dev -- test-raind-e2e"
  fi
}

build_components() {
  log "build all raind components"
  cd "${ROOT_DIR}"
  ./scripts/build.sh build
  [[ -x "${RAIND_BIN}" ]] || fail "missing built raind binary: ${RAIND_BIN}"
  [[ -x "${CONDENSER_BIN}" ]] || fail "missing built condenser binary: ${CONDENSER_BIN}"
}

prepare_runtime() {
  log "prepare runtime prerequisites"
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

cleanup_stale_condenser() {
  sudo_cmd pkill -TERM -f "^${CONDENSER_BIN}$" 2>/dev/null || true
  sleep 0.2
  sudo_cmd pkill -KILL -f "^${CONDENSER_BIN}$" 2>/dev/null || true
}

assert_port_free() {
  local port="$1"
  if sudo_cmd ss -ltn "( sport = :${port} )" | grep -q ":${port}"; then
    fail "port ${port} is already in use in this Workshop"
  fi
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
  sudo_cmd curl -sS \
    --connect-timeout 1 \
    --max-time 3 \
    --cert /etc/raind/cert/raindClient.crt \
    --key /etc/raind/cert/raindClient.key \
    --cacert /etc/raind/cert/raind.crt \
    "https://127.0.0.1:7755$1"
}

wait_ready() {
  log "wait for condenser management API"
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

run_raind() {
  local name="$1"
  shift
  local out="${E2E_WORK_DIR}/${name}.out"

  log "raind $*"
  sudo_cmd env PATH="${PATH}" "${RAIND_BIN}" "$@" >"${out}" 2>&1
  [[ -s "${out}" ]] || fail "raind $* produced no output"
}

assert_output_contains() {
  local name="$1"
  local pattern="$2"
  local out="${E2E_WORK_DIR}/${name}.out"

  if ! grep -q "${pattern}" "${out}"; then
    printf '%s\n' "--- ${out} ---" >&2
    cat "${out}" >&2
    fail "expected output to contain: ${pattern}"
  fi
}

run_cli_checks() {
  log "run raind cli checks"

  run_raind version --version
  assert_output_contains version "raind version"

  run_raind image-ls image ls
  assert_output_contains image-ls "REPOSITORY"

  run_raind container-ls container ls
  assert_output_contains container-ls "CONTAINER ID"

  run_raind network-ls network ls
  assert_output_contains network-ls "NETWORK"

  run_raind pod-ls resource pod ls
  assert_output_contains pod-ls "POD ID"

  run_raind service-ls resource service ls
  assert_output_contains service-ls "SERVICE ID"

  run_raind bottle-ls bottle ls
  assert_output_contains bottle-ls "BOTTLE ID"
}

main() {
  require_workshop
  mkdir -p "${E2E_WORK_DIR}"

  build_components
  prepare_runtime
  start_condenser
  trap stop_condenser EXIT
  wait_ready
  run_cli_checks

  log "raind e2e completed"
}

main "$@"
