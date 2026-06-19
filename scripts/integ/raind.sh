#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
RAIND_BIN="${ROOT_DIR}/bin/raind"
CONDENSER_BIN="${ROOT_DIR}/bin/condenser"
E2E_WORK_DIR="${E2E_WORK_DIR:-/tmp/raind-cli-e2e}"
LOG_PATH="${E2E_WORK_DIR}/condenser.log"
PID=""
CSM_STORE="/etc/raind/store/container/csm.json"
PSM_STORE="/etc/raind/store/resource/pod/psm.json"

log() {
  printf '==> %s\n' "$*"
}

fail() {
  printf 'error: %s\n' "$*" >&2
  if [[ -f "${LOG_PATH}" ]]; then
    printf '%s\n' '--- condenser log ---' >&2
    tail -120 "${LOG_PATH}" >&2 || true
  fi
  if sudo_cmd test -f "${CSM_STORE}" 2>/dev/null; then
    printf '%s\n' '--- csm.json ---' >&2
    sudo_cmd cat "${CSM_STORE}" >&2 || true
  fi
  if sudo_cmd test -f "${PSM_STORE}" 2>/dev/null; then
    printf '%s\n' '--- psm.json ---' >&2
    sudo_cmd cat "${PSM_STORE}" >&2 || true
  fi
  if sudo_cmd test -d /etc/raind/container 2>/dev/null; then
    printf '%s\n' '--- recent container logs ---' >&2
    sudo_cmd find /etc/raind/container -maxdepth 3 \( -path '*/logs/init.log' -o -path '*/logs/console.log' \) -type f -printf '%T@ %p\n' 2>/dev/null \
      | sort -nr \
      | head -8 \
      | awk '{ $1=""; sub(/^ /, ""); print }' \
      | while IFS= read -r runtime_log; do
          printf '%s\n' "----- ${runtime_log} -----" >&2
          sudo_cmd tail -40 "${runtime_log}" >&2 || true
        done
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
    fail "sudo is required for raind integration test"
  fi
  sudo -n "$@"
}

require_workshop() {
  if [[ "${ROOT_DIR}" != /project* ]]; then
    fail "raind integration test must run inside Workshop. use: workshop run raind-dev -- test-raind-integ"
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
    /etc/raind/cert \
    /etc/raind/store \
    /etc/raind/container \
    /etc/raind/image/layers \
    /var/log/raind \
    /sys/fs/cgroup/raind

  reset_integ_runtime_state

  sudo_cmd chmod 0755 /etc/raind /etc/raind/log /etc/raind/cert /etc/raind/store /var/log/raind

  for controller in cpu memory pids io; do
    if sudo_cmd grep -qw "${controller}" /sys/fs/cgroup/raind/cgroup.controllers; then
      sudo_cmd sh -c "echo +${controller} > /sys/fs/cgroup/raind/cgroup.subtree_control 2>/dev/null || true"
    fi
  done
}

reset_integ_runtime_state() {
  log "reset integration runtime state"

  # The integration suite exercises CLI/API contracts and should not depend on
  # leftover runtime state from previous manual runs or failed tests. This is
  # especially important now that short-lived non-TTY containers are supervised
  # by shim and correctly leave terminal CSM entries behind.
  sudo_cmd rm -rf /etc/raind/store/*
  sudo_cmd rm -rf /etc/raind/container/*
  sudo_cmd rm -f /etc/raind/log/droplet_audit.log

  if sudo_cmd test -d /run/netns; then
    sudo_cmd sh -c 'for ns in /run/netns/rn_*; do [ -e "$ns" ] || continue; umount "$ns" 2>/dev/null || true; rm -f "$ns"; done'
  fi

  if sudo_cmd test -d /sys/fs/cgroup/raind; then
    sudo_cmd find /sys/fs/cgroup/raind -mindepth 1 -depth -type d -exec rmdir {} + 2>/dev/null || true
  fi
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

api_curl_json() {
  local method="$1"
  local path="$2"
  local body="$3"

  sudo_cmd curl -sS \
    --connect-timeout 1 \
    --max-time 3 \
    --cert /etc/raind/cert/raindClient.crt \
    --key /etc/raind/cert/raindClient.key \
    --cacert /etc/raind/cert/raind.crt \
    -X "${method}" \
    -H 'Content-Type: application/json' \
    --data-binary "@${body}" \
    "https://127.0.0.1:7755${path}"
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
  if ! sudo_cmd env PATH="${PATH}" "${RAIND_BIN}" "$@" >"${out}" 2>&1; then
    printf '%s\n' "--- ${out} ---" >&2
    cat "${out}" >&2 || true
    fail "raind $* failed"
  fi
  [[ -s "${out}" ]] || fail "raind $* produced no output"
}

run_raind_allow_empty() {
  local name="$1"
  shift
  local out="${E2E_WORK_DIR}/${name}.out"

  log "raind $*"
  if ! sudo_cmd env PATH="${PATH}" "${RAIND_BIN}" "$@" >"${out}" 2>&1; then
    printf '%s\n' "--- ${out} ---" >&2
    cat "${out}" >&2 || true
    fail "raind $* failed"
  fi
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

  if ! grep -Fq -- "${pattern}" "${out}"; then
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

  run_raind namespace-ls resource namespace ls
  assert_output_contains namespace-ls "NAME"
  assert_output_contains namespace-ls "default"

  run_raind bottle-ls bottle ls
  assert_output_contains bottle-ls "BOTTLE ID"

  run_raind security-profile-ls security profile ls
  assert_output_contains security-profile-ls "NAME"
  assert_output_contains security-profile-ls "default"
  assert_output_contains security-profile-ls "dev"
  assert_output_contains security-profile-ls "deploy"
  assert_output_contains security-profile-ls "restricted"
  assert_output_contains security-profile-ls "privileged"
  assert_output_contains security-profile-ls "unconfined"

  run_raind security-profile-show security profile show default
  assert_output_contains security-profile-show "name: default"
  assert_output_contains security-profile-show "apparmorProfile: raind-default"

  run_raind security-profile-show-deploy security profile show deploy
  assert_output_contains security-profile-show-deploy "name: deploy"
  assert_output_contains security-profile-show-deploy "apparmorProfile: raind-default"

  run_raind security-profile-show-restricted security profile show restricted
  assert_output_contains security-profile-show-restricted "name: restricted"
  assert_output_contains security-profile-show-restricted "base: []"

  run_raind security-profile-show-privileged security profile show privileged
  assert_output_contains security-profile-show-privileged "name: privileged"
  assert_output_contains security-profile-show-privileged "CAP_SYS_ADMIN"

  run_raind security-profile-show-unconfined security profile show unconfined
  assert_output_contains security-profile-show-unconfined "name: unconfined"
  assert_output_contains security-profile-show-unconfined "CAP_NET_RAW"

  local custom_profile_name="integ-custom-profile-$$"
  cat >"${E2E_WORK_DIR}/custom-security-profile.yaml" <<YAML
apiVersion: raind.io/v1
kind: SecurityProfile
metadata:
  name: ${custom_profile_name}
spec:
  extends: deploy
  add-cap:
    - CAP_SYS_PTRACE
  drop-cap:
    - CAP_AUDIT_WRITE
YAML
  run_raind security-profile-register security profile register -f "${E2E_WORK_DIR}/custom-security-profile.yaml"
  assert_output_contains security-profile-register "security profile: ${custom_profile_name} registered"
  run_raind security-profile-show-custom security profile show "${custom_profile_name}"
  assert_output_contains security-profile-show-custom "name: ${custom_profile_name}"
  assert_output_contains security-profile-show-custom "type: custom"
  assert_output_contains security-profile-show-custom "extends: deploy"
  assert_output_contains security-profile-show-custom "CAP_SYS_PTRACE"
  run_raind security-profile-delete security profile delete "${custom_profile_name}"
  assert_output_contains security-profile-delete "security profile: ${custom_profile_name} deleted"
  assert_raind_fails security-profile-show-deleted security profile show "${custom_profile_name}"

  run_raind help --help
  assert_output_contains help "container"
  assert_output_contains help "image"
  assert_output_contains help "network"
  assert_output_contains help "resource"
  assert_output_contains help "security"
  assert_output_contains help "logs"

  run_raind security-help security --help
  assert_output_contains security-help "profile"
  assert_output_contains security-help "policy"

  run_raind container-run-help container run --help
  assert_output_contains container-run-help "rootless-mode"
  assert_output_contains container-run-help "login-root"
  assert_output_contains container-run-help "security-profile"

  run_raind container-create-help container create --help
  assert_output_contains container-create-help "rootless-mode"
  assert_output_contains container-create-help "login-root"
  assert_output_contains container-create-help "security-profile"

  run_raind completion-bash completion bash
  assert_output_contains completion-bash "_raind_complete"

  run_raind completion-zsh completion zsh
  assert_output_contains completion-zsh "#compdef raind"

  run_raind policy-ls-ew security policy ls --type ew
  assert_output_contains policy-ls-ew "POLICY TYPE"

  run_raind policy-ls-ns-obs security policy ls --type ns-obs
  assert_output_contains policy-ls-ns-obs "POLICY TYPE"

  run_raind policy-ls-ns-enf security policy ls --type ns-enf
  assert_output_contains policy-ls-ns-enf "POLICY TYPE"

  run_raind resource-replicaset-ls resource replicaset ls
  assert_output_contains resource-replicaset-ls "REPLICASET ID"

  run_raind_allow_empty logs-netflow logs netflow --line 5

  assert_raind_fails invalid-top-level definitely-not-a-command
  assert_raind_fails invalid-subcommand container definitely-not-a-subcommand
  assert_raind_fails invalid-policy-type security policy ls --type invalid
  assert_raind_fails invalid-rootless-run-mode container run --rootless-mode invalid-mode busybox:latest true
  assert_raind_fails invalid-rootless-create-mode container create --rootless-mode invalid-mode busybox:latest true
  assert_raind_fails unknown-container-stop container stop unknown-e2e-container
  assert_raind_fails unknown-container-rm container rm unknown-e2e-container
  assert_raind_fails image-status-missing image rm ""
}

run_cli_write_checks() {
  local bridge="rcli$$"
  local ns_name="e2e-cli-ns-$$"
  local ns_pod_name="e2e-cli-ns-pod-$$"
  local manifest_ns_name="e2e-cli-manifest-ns-$$"
  local pod_name="e2e-cli-pod-$$"
  local svc_name="e2e-cli-svc-$$"
  local resource_svc_name="e2e-cli-apply-svc-$$"
  local ns_pod_id
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

  run_raind namespace-create resource namespace create "${ns_name}"
  assert_output_contains namespace-create "created"
  assert_output_contains namespace-create "${ns_name}"
  run_raind namespace-show resource namespace show "${ns_name}"
  assert_output_contains namespace-show "${ns_name}"
  assert_output_contains namespace-show "NETWORK"
  run_raind namespace-ls-after-create resource namespace ls
  assert_output_contains namespace-ls-after-create "${ns_name}"

  run_raind ns-pod-create resource pod create --name "${ns_pod_name}" --namespace "${ns_name}" --label app=e2e-ns
  assert_output_contains ns-pod-create "pod:"
  ns_pod_id="$(extract_created_id ns-pod-create)"
  [[ -n "${ns_pod_id}" ]] || fail "namespace pod create did not print pod id"
  run_raind ns-pod-ls resource pod ls --namespace "${ns_name}"
  assert_output_contains ns-pod-ls "${ns_pod_name}"
  run_raind_allow_empty ns-pod-rm resource pod rm "${ns_pod_id}"
  run_raind namespace-rm resource namespace rm "${ns_name}"
  assert_output_contains namespace-rm "removed"

  run_raind pod-create resource pod create --name "${pod_name}" --namespace default --label app=e2e --annotation suite=raind
  assert_output_contains pod-create "pod:"
  pod_id="$(extract_created_id pod-create)"
  [[ -n "${pod_id}" ]] || fail "pod create did not print pod id"
  run_raind pod-ls-after-create resource pod ls
  assert_output_contains pod-ls-after-create "${pod_name}"
  # Keep integration pod checks focused on CLI/API persistence contracts.
  # Runtime lifecycle behavior for pod/deployment/service resources is covered
  # by scripts/e2e/raind.sh. Empty CLI-created pods are reconciled by the pod
  # controller and may be recreated with a new pod ID, so remove this API-only
  # pod immediately after the list assertion instead of keeping the original
  # ID around for late cleanup.
  run_raind pod-rm resource pod rm "${pod_id}"
  assert_output_contains pod-rm "removed"

  local rootless_pod_name="e2e-cli-rootless-pod-$$"
  local rootless_pod_json="${E2E_WORK_DIR}/rootless-pod.json"
  local rootless_pod_out="${E2E_WORK_DIR}/rootless-pod-create-api.out"
  local rootless_pod_id
  cat >"${rootless_pod_json}" <<JSON
{
  "name": "${rootless_pod_name}",
  "namespace": "default",
  "hostUsers": false,
  "labels": {
    "app": "e2e-cli-rootless"
  }
}
JSON
  log "POST /v1/pods hostUsers=false"
  api_curl_json POST /v1/pods "${rootless_pod_json}" >"${rootless_pod_out}"
  jq -e '.status == "success" and (.data.podId // .data.id // "") != ""' "${rootless_pod_out}" >/dev/null || {
    cat "${rootless_pod_out}" >&2 || true
    fail "rootless pod API create failed"
  }
  rootless_pod_id="$(jq -r '.data.podId // .data.id' "${rootless_pod_out}")"
  local rootless_psm_dump="${E2E_WORK_DIR}/rootless-pod-psm.json"
  sudo_cmd cat "${PSM_STORE}" >"${rootless_psm_dump}"
  jq -e --arg pod "${rootless_pod_id}" --arg name "${rootless_pod_name}" '
    (.pods[$pod] // (.pods | to_entries | map(select(.value.name == $name)) | first | .value)) as $p |
    $p != null and
    $p.rootless == true and
    (.podTemplates[$p.templateId].spec.rootless == true)
  ' "${rootless_psm_dump}" >/dev/null || fail "rootless pod was not persisted in PSM"
  run_raind rootless-pod-ls resource pod ls
  assert_output_contains rootless-pod-ls "${rootless_pod_name}"
  run_raind_allow_empty rootless-pod-rm resource pod rm "${rootless_pod_id}"

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

  cat >"${E2E_WORK_DIR}/resource-namespace-service.yaml" <<YAML
apiVersion: v1
kind: Namespace
metadata:
  name: ${manifest_ns_name}
---
apiVersion: v1
kind: Service
metadata:
  name: ${resource_svc_name}-ns
  namespace: ${manifest_ns_name}
spec:
  selector:
    app: e2e-resource-ns
  ports:
    - port: 9191
      targetPort: 91
      protocol: TCP
YAML
  run_raind resource-namespace-apply resource apply -f "${E2E_WORK_DIR}/resource-namespace-service.yaml"
  assert_output_contains resource-namespace-apply "namespace:"
  assert_output_contains resource-namespace-apply "service:"
  run_raind resource-service-ls-ns resource service ls --namespace "${manifest_ns_name}"
  assert_output_contains resource-service-ls-ns "${resource_svc_name}-ns"
  run_raind resource-namespace-rm resource rm -f "${E2E_WORK_DIR}/resource-namespace-service.yaml"
  assert_output_contains resource-namespace-rm "namespace:"
  assert_output_contains resource-namespace-rm "service:"

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

  log "raind integration test completed"
}

main "$@"
