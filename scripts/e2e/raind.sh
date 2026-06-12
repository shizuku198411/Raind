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

run_raind_allow_empty() {
  local name="$1"
  shift
  local out="${E2E_WORK_DIR}/${name}.out"

  log "raind $*"
  sudo_cmd env PATH="${PATH}" "${RAIND_BIN}" "$@" >"${out}" 2>&1
}

assert_raind_fails() {
  local name="$1"
  shift
  local out="${E2E_WORK_DIR}/${name}.err"

  log "raind $* fails"
  if sudo_cmd env PATH="${PATH}" "${RAIND_BIN}" "$@" >"${out}" 2>&1; then
    cat "${out}" >&2 || true
    fail "expected raind $* to fail"
  fi
  [[ -s "${out}" ]] || fail "raind $* failed without output"
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

extract_created_id() {
  local name="$1"
  awk '/: .* (created|applied)$/ { print $2; exit }' "${E2E_WORK_DIR}/${name}.out"
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

  run_raind help --help
  assert_output_contains help "container"
  assert_output_contains help "image"
  assert_output_contains help "network"
  assert_output_contains help "resource"
  assert_output_contains help "policy"
  assert_output_contains help "logs"

  run_raind completion-bash completion bash
  assert_output_contains completion-bash "_raind_complete"

  run_raind completion-zsh completion zsh
  assert_output_contains completion-zsh "#compdef raind"

  run_raind policy-ls-ew policy ls --type ew
  assert_output_contains policy-ls-ew "POLICY TYPE"

  run_raind policy-ls-ns-obs policy ls --type ns-obs
  assert_output_contains policy-ls-ns-obs "POLICY TYPE"

  run_raind policy-ls-ns-enf policy ls --type ns-enf
  assert_output_contains policy-ls-ns-enf "POLICY TYPE"

  run_raind resource-replicaset-ls resource replicaset ls
  assert_output_contains resource-replicaset-ls "REPLICASET ID"

  run_raind_allow_empty logs-netflow logs netflow --line 5

  assert_raind_fails invalid-top-level definitely-not-a-command
  assert_raind_fails invalid-subcommand container definitely-not-a-subcommand
  assert_raind_fails invalid-policy-type policy ls --type invalid
  assert_raind_fails unknown-container-stop container stop unknown-e2e-container
  assert_raind_fails unknown-container-rm container rm unknown-e2e-container
  assert_raind_fails image-status-missing image rm ""
}

run_cli_write_checks() {
  local bridge="rcli$$"
  local pod_name="e2e-cli-pod-$$"
  local svc_name="e2e-cli-svc-$$"
  local resource_svc_name="e2e-cli-apply-svc-$$"
  local pod_id
  local service_id

  log "run raind write-path checks"

  run_raind network-create network create "${bridge}"
  assert_output_contains network-create "created"
  run_raind network-ls-after-create network ls
  assert_output_contains network-ls-after-create "${bridge}"
  assert_raind_fails network-create-duplicate network create "${bridge}"
  run_raind network-rm network rm "${bridge}"
  assert_output_contains network-rm "delete network"

  run_raind pod-create resource pod create --name "${pod_name}" --namespace default --label app=e2e --annotation suite=raind
  assert_output_contains pod-create "pod:"
  pod_id="$(extract_created_id pod-create)"
  [[ -n "${pod_id}" ]] || fail "pod create did not print pod id"
  run_raind pod-ls-after-create resource pod ls
  assert_output_contains pod-ls-after-create "${pod_name}"
  run_raind_allow_empty pod-start resource pod start "${pod_id}"
  run_raind_allow_empty pod-stop resource pod stop "${pod_id}"

  cat >"${E2E_WORK_DIR}/service.yaml" <<YAML
apiVersion: v1
kind: Service
metadata:
  name: ${svc_name}
  namespace: default
spec:
  selector:
    app: e2e
  ports:
    - port: 8080
      targetPort: 80
      protocol: TCP
YAML
  run_raind service-create resource service create -f "${E2E_WORK_DIR}/service.yaml"
  assert_output_contains service-create "service:"
  service_id="$(extract_created_id service-create)"
  [[ -n "${service_id}" ]] || fail "service create did not print service id"
  run_raind service-ls-after-create resource service ls
  assert_output_contains service-ls-after-create "${svc_name}"
  run_raind service-show resource service show "${service_id}"
  assert_output_contains service-show "${svc_name}"
  run_raind service-rm resource service rm "${service_id}"
  assert_output_contains service-rm "removed"

  cat >"${E2E_WORK_DIR}/resource-service.yaml" <<YAML
apiVersion: v1
kind: Service
metadata:
  name: ${resource_svc_name}
  namespace: default
spec:
  selector:
    app: e2e-resource
  ports:
    - port: 9090
      targetPort: 90
      protocol: TCP
YAML
  run_raind resource-apply resource apply -f "${E2E_WORK_DIR}/resource-service.yaml"
  assert_output_contains resource-apply "service:"
  assert_output_contains resource-apply "applied"
  run_raind resource-rm resource rm -f "${E2E_WORK_DIR}/resource-service.yaml"
  assert_output_contains resource-rm "service:"

  run_raind_allow_empty pod-rm resource pod rm "${pod_id}"

  assert_raind_fails service-create-invalid resource service create -f "${E2E_WORK_DIR}/missing-service.yaml"
  assert_raind_fails container-create-missing-image container create raind/e2e-missing:latest --name "e2e-cli-missing-$$"
}

main() {
  require_workshop
  mkdir -p "${E2E_WORK_DIR}"

  build_components
  prepare_runtime
  cleanup_stale_condenser
  assert_raind_fails condenser-down image ls
  start_condenser
  trap stop_condenser EXIT
  wait_ready
  run_cli_checks
  run_cli_write_checks

  log "raind e2e completed"
}

main "$@"
