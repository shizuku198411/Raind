#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
RAIND_BIN="${ROOT_DIR}/bin/raind"
CONDENSER_BIN="${ROOT_DIR}/bin/condenser"
E2E_WORK_DIR="${E2E_WORK_DIR:-/tmp/raind-deploy-e2e}"
LOG_PATH="${E2E_WORK_DIR}/condenser.log"
PID=""
SUFFIX="${E2E_SUFFIX:-$$}"
HOST_ADDR=""
BSM_STORE="/etc/raind/store/container/bsm.json"
CSM_STORE="/etc/raind/store/container/csm.json"
IPAM_STORE="/etc/raind/store/network/ipam.json"
ISM_STORE="/etc/raind/store/resource/ingress/ism.json"
CFM_STORE="/etc/raind/store/resource/configmap/cfm.json"
SEC_STORE="/etc/raind/store/resource/secret/sec.json"
NETPOL_STORE="/etc/raind/store/resource/networkpolicy/netpol.json"
NPM_STORE="/etc/raind/store/network/npm.json"
NSM_STORE="/etc/raind/store/resource/namespace/nsm.json"
PSM_STORE="/etc/raind/store/resource/pod/psm.json"
SSM_STORE="/etc/raind/store/resource/service/ssm.json"

log() {
  printf '\r\033[K==> %s\n' "$*"
}

fail() {
  printf 'error: %s\n' "$*" >&2
  if [[ -f "${LOG_PATH}" ]]; then
    printf '%s\n' '--- condenser log ---' >&2
    tail -220 "${LOG_PATH}" >&2 || true
  fi
  if sudo_cmd test -f /etc/raind/log/droplet_audit.log 2>/dev/null; then
    printf '%s\n' '--- droplet audit log ---' >&2
    sudo_cmd tail -120 /etc/raind/log/droplet_audit.log >&2 || true
  fi
  if sudo_cmd test -d /etc/raind/container 2>/dev/null; then
    printf '%s\n' '--- recent container init logs ---' >&2
    sudo_cmd find /etc/raind/container -maxdepth 3 -path '*/logs/init.log' -type f -printf '%T@ %p\n' 2>/dev/null       | sort -nr       | head -8       | awk '{ $1=""; sub(/^ /, ""); print }'       | while IFS= read -r init_log; do
          printf '%s\n' "----- ${init_log} -----" >&2
          sudo_cmd tail -40 "${init_log}" >&2 || true
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
    fail "sudo is required for raind e2e"
  fi
  sudo -n "$@"
}

require_workshop() {
  if [[ "${ROOT_DIR}" != /project* ]]; then
    fail "raind deploy e2e must run inside Workshop. use: workshop run raind-dev -- test-e2e"
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
    /etc/raind/store/container \
    /etc/raind/store/image \
    /etc/raind/store/network \
    /etc/raind/store/resource/ingress \
    /etc/raind/store/resource/configmap \
    /etc/raind/store/resource/secret \
    /etc/raind/store/resource/networkpolicy \
    /etc/raind/store/resource/namespace \
    /etc/raind/store/resource/pod \
    /etc/raind/store/resource/service \
    /etc/raind/container \
    /etc/raind/image/layers \
    /var/log/raind \
    /sys/fs/cgroup/raind

  reset_e2e_runtime_state

  sudo_cmd chmod 0755 /etc/raind /etc/raind/log /etc/raind/cert /etc/raind/store /var/log/raind
  sudo_cmd install -m 0755 "${ROOT_DIR}/bin/condenser-hook-agent" /usr/local/bin/condenser-hook-agent
  sudo_cmd install -m 0755 "${ROOT_DIR}/bin/droplet" /usr/local/bin/droplet

  for controller in cpu memory pids io; do
    if sudo_cmd grep -qw "${controller}" /sys/fs/cgroup/raind/cgroup.controllers; then
      sudo_cmd sh -c "echo +${controller} > /sys/fs/cgroup/raind/cgroup.subtree_control 2>/dev/null || true"
    fi
  done

  HOST_ADDR="$(ip -4 -o addr show dev eth0 | awk '{split($4, a, "/"); print a[1]; exit}')"
  [[ -n "${HOST_ADDR}" ]] || fail "could not resolve eth0 address"
}


reset_e2e_runtime_state() {
  log "reset e2e runtime state"

  # The e2e suite runs against the fixed /etc/raind runtime paths. Previous
  # failed runs can leave Pod/ReplicaSet/Deployment store entries behind. When
  # condenser starts, the pod controller may try to reconcile those stale
  # objects and race with the current run, producing confusing 500s such as
  # "podTemplateId=... not found" during a new resource apply.
  sudo_cmd rm -rf /etc/raind/store/*
  sudo_cmd rm -rf /etc/raind/container/*
  sudo_cmd rm -f /etc/raind/log/droplet_audit.log

  # Rootless networking creates short-lived named netns bind mounts. Clean up
  # stale entries from interrupted runs; ignore entries owned by other tools.
  if sudo_cmd test -d /run/netns; then
    sudo_cmd sh -c 'for ns in /run/netns/rn_*; do [ -e "$ns" ] || continue; umount "$ns" 2>/dev/null || true; rm -f "$ns"; done'
  fi

  cleanup_runtime_links

  # Remove stale raind cgroup leaves from previous runs, but keep the root
  # /sys/fs/cgroup/raind directory that prepare_runtime manages.
  if sudo_cmd test -d /sys/fs/cgroup/raind; then
    sudo_cmd find /sys/fs/cgroup/raind -mindepth 1 -depth -type d -exec rmdir {} + 2>/dev/null || true
  fi
}

cleanup_runtime_links() {
  while read -r ifname; do
    [[ -n "${ifname}" ]] || continue
    sudo_cmd ip link del "${ifname}" 2>/dev/null || true
  done < <(sudo_cmd ip -o link show 2>/dev/null | awk -F': ' '/: (rd_|rns)/ {print $2}' | cut -d@ -f1 | sort -u)
}

reset_resource_runtime_state() {
  log "reset resource runtime state"

  sudo_cmd rm -rf /etc/raind/container/*
  sudo_cmd rm -f \
    "${BSM_STORE}" \
    "${CSM_STORE}" \
    "${IPAM_STORE}" \
    "${ISM_STORE}" \
    "${CFM_STORE}" \
    "${SEC_STORE}" \
    "${NETPOL_STORE}" \
    "${NPM_STORE}" \
    "${NSM_STORE}" \
    "${PSM_STORE}" \
    "${SSM_STORE}"

  if sudo_cmd test -d /run/netns; then
    sudo_cmd sh -c 'for ns in /run/netns/rn_*; do [ -e "$ns" ] || continue; umount "$ns" 2>/dev/null || true; rm -f "$ns"; done'
  fi
  cleanup_runtime_links
  if sudo_cmd test -d /sys/fs/cgroup/raind; then
    sudo_cmd find /sys/fs/cgroup/raind -mindepth 1 -depth -type d -exec rmdir {} + 2>/dev/null || true
  fi
}

restart_condenser_with_clean_resources() {
  stop_condenser
  reset_resource_runtime_state
  start_condenser
  wait_ready
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
  : >"${LOG_PATH}"

  cleanup_stale_condenser
  for port in 7755 7756 7757 7758; do
    assert_port_free "${port}"
  done

  sudo_cmd env PATH="${ROOT_DIR}/bin:${PATH}" "${CONDENSER_BIN}" >"${LOG_PATH}" 2>&1 &
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

run_raind_retry_transient() {
  local name="$1"
  shift
  local out="${E2E_WORK_DIR}/${name}.out"
  local attempt

  for attempt in 1 2 3; do
    if [[ "${attempt}" -eq 1 ]]; then
      log "raind $*"
    else
      log "raind $* (retry ${attempt})"
    fi

    if sudo_cmd env PATH="${PATH}" "${RAIND_BIN}" "$@" >"${out}" 2>&1; then
      return
    fi

    if ! grep -Eqi 'cgroup\.procs: no such process|container init pid file not found|no such process' "${out}"; then
      break
    fi
    sleep 1
  done

  printf '%s\n' "--- ${out} ---" >&2
  cat "${out}" >&2 || true
  fail "raind $* failed"
}

container_is_absent() {
  local cid="$1"
  local csm_path="${CSM_STORE}"

  if sudo_cmd test -e "/etc/raind/container/${cid}"; then
    return 1
  fi
  if sudo_cmd test -f "${csm_path}" &&
    sudo_cmd jq -e --arg cid "${cid}" '.containers[$cid] != null' "${csm_path}" >/dev/null 2>&1; then
    return 1
  fi
  return 0
}

cleanup_container() {
  local name="$1"
  local cid="$2"
  local out="${E2E_WORK_DIR}/${name}.out"
  local attempt

  [[ -n "${cid}" ]] || return
  log "cleanup container ${cid}"

  for attempt in 1 2 3 4 5; do
    if sudo_cmd env PATH="${PATH}" "${RAIND_BIN}" container rm "${cid}" >"${out}" 2>&1; then
      return
    fi
    if grep -Eqi '(not found|does not exist)' "${out}" || container_is_absent "${cid}"; then
      return
    fi
    sleep 0.5
  done

  printf '%s\n' "--- ${out} ---" >&2
  cat "${out}" >&2 || true
  fail "failed to cleanup container ${cid}"
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

is_transient_resource_apply_failure() {
  local out="$1"

  grep -Eqi \
    'pod start failed|replicaset start failed|deployment start failed|hook createContainer\[[0-9]+\] failed|service hook failed: update pod namespaces failed|podId=.*not found|podTemplateId=.*not found|interface: rns[[:xdigit:]]{12} is already created' \
    "${out}"
}

namespace_bridge_name() {
  local ns="$1"
  printf 'rns%s\n' "$(printf '%s' "${ns}" | sha256sum | awk '{print substr($1, 1, 12)}')"
}

manifest_namespaces() {
  local yaml="$1"
  awk '
    /^[[:space:]]*kind:[[:space:]]*Namespace[[:space:]]*$/ { in_ns = 1; next }
    in_ns && /^[[:space:]]*name:[[:space:]]*/ {
      name = $0
      sub(/^[[:space:]]*name:[[:space:]]*/, "", name)
      gsub(/["'\'']/, "", name)
      print name
      in_ns = 0
    }
    /^---[[:space:]]*$/ { in_ns = 0 }
  ' "${yaml}"
}

cleanup_manifest_namespaces() {
  local yaml="$1"
  local ns
  local bridge

  while IFS= read -r ns; do
    [[ -n "${ns}" ]] || continue
    bridge="$(namespace_bridge_name "${ns}")"
    sudo_cmd env PATH="${PATH}" "${RAIND_BIN}" resource namespace rm "${ns}" >"${E2E_WORK_DIR}/cleanup-namespace-${ns}.out" 2>&1 || true
    sudo_cmd env PATH="${PATH}" "${RAIND_BIN}" network rm "${bridge}" >"${E2E_WORK_DIR}/cleanup-network-${bridge}.out" 2>&1 || true
    sudo_cmd ip link del "${bridge}" 2>/dev/null || true
  done < <(manifest_namespaces "${yaml}" | sort -u)
}

run_resource_apply_with_retry() {
  local name="$1"
  local yaml="$2"
  local out="${E2E_WORK_DIR}/${name}.out"
  local cleanup_out
  local attempt

  for attempt in 1 2 3; do
    if [[ "${attempt}" -eq 1 ]]; then
      log "raind resource apply -f ${yaml}"
    else
      log "raind resource apply -f ${yaml} (retry ${attempt})"
    fi

    if sudo_cmd env PATH="${PATH}" "${RAIND_BIN}" resource apply -f "${yaml}" >"${out}" 2>&1; then
      [[ -s "${out}" ]] || fail "raind resource apply -f ${yaml} produced no output"
      return
    fi

    if ! is_transient_resource_apply_failure "${out}"; then
      printf '%s\n' "--- ${out} ---" >&2
      cat "${out}" >&2 || true
      fail "raind resource apply -f ${yaml} failed"
    fi

    if [[ "${attempt}" -eq 3 ]]; then
      printf '%s\n' "--- ${out} ---" >&2
      cat "${out}" >&2 || true
      fail "raind resource apply -f ${yaml} failed after retries"
    fi

    log "transient resource apply failure detected; cleanup partial resources and retry"
    cleanup_out="${E2E_WORK_DIR}/${name}-retry-cleanup-${attempt}.out"
    sudo_cmd env PATH="${PATH}" "${RAIND_BIN}" resource rm -f "${yaml}" >"${cleanup_out}" 2>&1 || true
    cleanup_manifest_namespaces "${yaml}"
    restart_condenser_with_clean_resources
    sleep 1
  done
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

assert_output_not_contains() {
  local name="$1"
  local pattern="$2"
  local out="${E2E_WORK_DIR}/${name}.out"

  if grep -Fq -- "${pattern}" "${out}"; then
    printf '%s\n' "--- ${out} ---" >&2
    cat "${out}" >&2
    fail "expected output not to contain: ${pattern}"
  fi
}

wait_file_contains() {
  local path="$1"
  local pattern="$2"

  for _ in $(seq 1 100); do
    if [[ -f "${path}" ]] && grep -Fq -- "${pattern}" "${path}"; then
      return 0
    fi
    sleep 0.1
  done

  return 1
}


assert_sudo_path_exists() {
  local path="$1"
  sudo_cmd test -e "${path}" || fail "expected path to exist: ${path}"
}

assert_sudo_path_absent() {
  local path="$1"
  sudo_cmd test ! -e "${path}" || fail "expected path to be absent: ${path}"
}

runtime_exit_status_matches() {
  local cid="$1"
  local expected_exit_code="$2"
  local expected_reason="$3"
  local expected_message="$4"
  local state_path="/etc/raind/container/${cid}/state.json"

  sudo_cmd test -f "${state_path}" && sudo_cmd jq -e \
    --arg cid "${cid}" \
    --argjson code "${expected_exit_code}" \
    --arg reason "${expected_reason}" \
    --arg message "${expected_message}" \
    '.id == $cid and .status == "stopped" and .pid == 0 and .shimPid == 0 and .exit_code == $code and .reason == $reason and .message == $message' \
    "${state_path}" >/dev/null 2>&1
}

csm_exit_status_matches() {
  local cid="$1"
  local expected_exit_code="$2"
  local expected_reason="$3"
  local expected_message="$4"
  local csm_path="${CSM_STORE}"

  sudo_cmd test -f "${csm_path}" && sudo_cmd jq -e \
    --arg cid "${cid}" \
    --argjson code "${expected_exit_code}" \
    --arg reason "${expected_reason}" \
    --arg message "${expected_message}" \
    '.containers[$cid].state == "stopped" and .containers[$cid].pid == 0 and .containers[$cid].exit_code == $code and .containers[$cid].reason == $reason and .containers[$cid].message == $message' \
    "${csm_path}" >/dev/null 2>&1
}

assert_container_exit_status() {
  local cid="$1"
  local expected_exit_code="$2"
  local expected_reason="$3"
  local expected_message="$4"
  local state_path="/etc/raind/container/${cid}/state.json"
  local csm_path="${CSM_STORE}"

  # The runtime shim writes the final status to state.json first. Condenser's
  # monitor then observes process-down and reconciles CSM from that runtime
  # state. Short-lived containers can therefore be visible as running in CSM for
  # a brief window after state.json has already reached stopped. Poll both files
  # for the same final status instead of asserting CSM immediately.
  for _ in $(seq 1 120); do
    if runtime_exit_status_matches "${cid}" "${expected_exit_code}" "${expected_reason}" "${expected_message}" && \
       csm_exit_status_matches "${cid}" "${expected_exit_code}" "${expected_reason}" "${expected_message}"; then
      return
    fi
    sleep 0.25
  done

  if ! runtime_exit_status_matches "${cid}" "${expected_exit_code}" "${expected_reason}" "${expected_message}"; then
    printf '%s\n' "--- ${state_path} ---" >&2
    sudo_cmd cat "${state_path}" >&2 || true
    fail "unexpected runtime exit status for ${cid}"
  fi

  printf '%s\n' "--- ${csm_path} ---" >&2
  sudo_cmd cat "${csm_path}" >&2 || true
  fail "unexpected CSM exit status for ${cid}"
}

wait_container_stopped() {
  local cid="$1"
  local state_path="/etc/raind/container/${cid}/state.json"
  local csm_path="${CSM_STORE}"

  for _ in $(seq 1 120); do
    if sudo_cmd test -f "${state_path}" && sudo_cmd jq -e '.status == "stopped"' "${state_path}" >/dev/null 2>&1 && \
       sudo_cmd test -f "${csm_path}" && sudo_cmd jq -e --arg cid "${cid}" '.containers[$cid].state == "stopped"' "${csm_path}" >/dev/null 2>&1; then
      return
    fi
    sleep 0.25
  done

  printf '%s\n' "--- ${state_path} ---" >&2
  sudo_cmd cat "${state_path}" >&2 || true
  printf '%s\n' "--- ${csm_path} ---" >&2
  sudo_cmd cat "${csm_path}" >&2 || true
  fail "timed out waiting for container to stop: ${cid}"
}

extract_created_id() {
  local name="$1"
  awk '
    $1 ~ /^(container|pod):$/ && $2 != "" { print $2; exit }
    /: .* (created|applied|started)$/ || /: .* created / { print $2; exit }
  ' "${E2E_WORK_DIR}/${name}.out"
}

resolve_container_id() {
  local ref="$1"
  local name="$2"
  local out="${E2E_WORK_DIR}/container-ls-resolve-${name}.out"
  local cid=""

  if [[ -n "${ref}" ]] && sudo_cmd test -f "/etc/raind/container/${ref}/config.json"; then
    printf '%s\n' "${ref}"
    return
  fi

  if ! sudo_cmd env PATH="${PATH}" "${RAIND_BIN}" container ls >"${out}" 2>&1; then
    printf '%s\n' "--- ${out} ---" >&2
    cat "${out}" >&2 || true
    fail "raind container ls failed while resolving container id for ${name}"
  fi

  cid="$(awk -v ref="${ref}" -v name="${name}" '
    NR == 1 { next }
    $1 == ref || $NF == ref || $NF == name { print $1; exit }
  ' "${out}")"

  [[ -n "${cid}" ]] || fail "could not resolve container id for ${name} from ref ${ref}"
  printf '%s\n' "${cid}"
}

extract_policy_id() {
  local name="$1"
  awk '/^policy: .* created$/ { print $2; exit }' "${E2E_WORK_DIR}/${name}.out"
}

wait_raind_contains() {
  local name="$1"
  local pattern="$2"
  shift 2

  for _ in $(seq 1 120); do
    if sudo_cmd env PATH="${PATH}" "${RAIND_BIN}" "$@" >"${E2E_WORK_DIR}/${name}.out" 2>&1 &&
      grep -Fq -- "${pattern}" "${E2E_WORK_DIR}/${name}.out"; then
      return
    fi
    sleep 0.5
  done

  printf '%s\n' "--- ${E2E_WORK_DIR}/${name}.out ---" >&2
  cat "${E2E_WORK_DIR}/${name}.out" >&2 || true
  fail "timed out waiting for raind $* output to contain: ${pattern}"
}

wait_resource_row_ready() {
  local name="$1"
  local resource_name="$2"
  local desired="$3"
  shift 3

  for _ in $(seq 1 120); do
    if sudo_cmd env PATH="${PATH}" "${RAIND_BIN}" "$@" >"${E2E_WORK_DIR}/${name}.out" 2>&1 &&
      awk -v resource="${resource_name}" -v desired="${desired}" '
        NR == 1 { next }
        $2 == resource && $4 == desired && $5 == desired && $6 == desired { found = 1 }
        END { exit(found ? 0 : 1) }
      ' "${E2E_WORK_DIR}/${name}.out"; then
      return
    fi
    sleep 0.5
  done

  printf '%s\n' "--- ${E2E_WORK_DIR}/${name}.out ---" >&2
  cat "${E2E_WORK_DIR}/${name}.out" >&2 || true
  fail "timed out waiting for ${resource_name} desired/current/ready=${desired}"
}

wait_pod_row_ready() {
  local name="$1"
  local pod_name="$2"
  shift 2

  for _ in $(seq 1 120); do
    if sudo_cmd env PATH="${PATH}" "${RAIND_BIN}" "$@" >"${E2E_WORK_DIR}/${name}.out" 2>&1 &&
      awk -v pod="${pod_name}" '
        NR == 1 { next }
        $2 == pod && $4 == "1/1" { found = 1 }
        END { exit(found ? 0 : 1) }
      ' "${E2E_WORK_DIR}/${name}.out"; then
      return
    fi
    sleep 0.5
  done

  printf '%s\n' "--- ${E2E_WORK_DIR}/${name}.out ---" >&2
  cat "${E2E_WORK_DIR}/${name}.out" >&2 || true
  fail "timed out waiting for pod ${pod_name} ready=1/1"
}

wait_resource_namespace_absent() {
  local name="$1"
  local ns="$2"
  local out="${E2E_WORK_DIR}/${name}.out"

  for _ in $(seq 1 180); do
    if ! sudo_cmd env PATH="${PATH}" "${RAIND_BIN}" resource namespace show "${ns}" >"${out}" 2>&1; then
      return
    fi
    if grep -Eqi '(not found|does not exist)' "${out}"; then
      return
    fi
    sleep 0.5
  done

  printf '%s\n' "--- ${out} ---" >&2
  cat "${out}" >&2 || true
  fail "timed out waiting for resource namespace to be removed: ${ns}"
}

wait_http_ok() {
  local url="$1"
  local out="${E2E_WORK_DIR}/http.out"

  log "curl ${url}"
  for _ in $(seq 1 120); do
    if curl -fsS --connect-timeout 2 --max-time 3 "${url}" >"${out}" 2>"${E2E_WORK_DIR}/http.err" &&
      grep -qi "nginx" "${out}"; then
      return
    fi
    sleep 0.5
  done

  cat "${E2E_WORK_DIR}/http.err" >&2 2>/dev/null || true
  if [[ -f "${out}" ]]; then
    printf '%s\n' "--- ${out} ---" >&2
    cat "${out}" >&2 || true
  fi
  fail "timed out waiting for ${url}"
}

wait_http_ok_host() {
  local url="$1"
  local host="$2"
  local out="${E2E_WORK_DIR}/http-host.out"

  log "curl -H Host:${host} ${url}"
  for _ in $(seq 1 120); do
    if curl -fsS --connect-timeout 2 --max-time 3 -H "Host: ${host}" "${url}" >"${out}" 2>"${E2E_WORK_DIR}/http-host.err" &&
      grep -qi "nginx" "${out}"; then
      return
    fi
    sleep 0.5
  done

  cat "${E2E_WORK_DIR}/http-host.err" >&2 2>/dev/null || true
  if [[ -f "${out}" ]]; then
    printf '%s\n' "--- ${out} ---" >&2
    cat "${out}" >&2 || true
  fi
  fail "timed out waiting for ${url} with host ${host}"
}

assert_security_profile_runtime_applied() {
  local cid="$1"
  local state="/etc/raind/container/${cid}/state.json"
  local pid

  pid="$(sudo_cmd jq -r '.pid // 0' "${state}")"
  [[ "${pid}" =~ ^[0-9]+$ && "${pid}" -gt 0 ]] || fail "container ${cid} has no running pid"
  sudo_cmd test -d "/proc/${pid}" || fail "container ${cid} pid ${pid} is not running"
  sudo_cmd grep -q '^NoNewPrivs:[[:space:]]*1$' "/proc/${pid}/status" || fail "NoNewPrivs is not enabled for ${cid}"
  sudo_cmd grep -q '^Seccomp:[[:space:]]*2$' "/proc/${pid}/status" || fail "seccomp filter mode is not enabled for ${cid}"
}

assert_dev_security_profile_applied() {
  local cid="$1"
  local config="/etc/raind/container/${cid}/config.json"

  log "verify dev security profile for ${cid}"
  sudo_cmd jq -e '
    .linux.apparmorProfile == "raind-default" and
    .linux.seccomp.defaultAction == "SCMP_ACT_ALLOW" and
    any(.linux.seccomp.syscalls[]?.names[]?; . == "unshare") and
    any(.process.capabilities.effective[]?; . == "CAP_NET_RAW")
  ' "${config}" >/dev/null || fail "dev security profile was not written to ${config}"

  assert_security_profile_runtime_applied "${cid}"
}

assert_deploy_security_profile_applied() {
  local cid="$1"
  local config="/etc/raind/container/${cid}/config.json"

  log "verify deploy security profile for ${cid}"
  sudo_cmd jq -e '
    .linux.apparmorProfile == "raind-default" and
    .linux.seccomp.defaultAction == "SCMP_ACT_ALLOW" and
    any(.linux.seccomp.syscalls[]?.names[]?; . == "unshare") and
    (any(.process.capabilities.effective[]?; . == "CAP_CHOWN")) and
    ([.process.capabilities.effective[]?] | index("CAP_NET_RAW") | not) and
    ([.process.capabilities.effective[]?] | index("CAP_MKNOD") | not)
  ' "${config}" >/dev/null || fail "deploy security profile was not written to ${config}"

  assert_security_profile_runtime_applied "${cid}"
}

assert_restricted_security_profile_applied() {
  local cid="$1"
  local config="/etc/raind/container/${cid}/config.json"

  log "verify restricted security profile for ${cid}"
  sudo_cmd jq -e '
    .linux.apparmorProfile == "raind-default" and
    .linux.seccomp.defaultAction == "SCMP_ACT_ALLOW" and
    any(.linux.seccomp.syscalls[]?.names[]?; . == "unshare") and
    ((.process.capabilities.effective // []) | length == 0) and
    ((.process.capabilities.bounding // []) | length == 0) and
    ((.process.capabilities.permitted // []) | length == 0)
  ' "${config}" >/dev/null || fail "restricted security profile was not written to ${config}"

  assert_security_profile_runtime_applied "${cid}"
}

assert_unconfined_security_profile_applied() {
  local cid="$1"
  local config="/etc/raind/container/${cid}/config.json"

  log "verify unconfined security profile for ${cid}"
  sudo_cmd jq -e '
    (.linux.seccomp == null) and
    (.linux.apparmorProfile == null) and
    (any(.process.capabilities.effective[]?; . == "CAP_NET_RAW")) and
    ([.process.capabilities.effective[]?] | index("CAP_SYS_ADMIN") | not)
  ' "${config}" >/dev/null || fail "unconfined security profile was not written to ${config}"
}

assert_custom_security_profile_applied() {
  local cid="$1"
  local config="/etc/raind/container/${cid}/config.json"

  log "verify custom security profile for ${cid}"
  sudo_cmd jq -e '
    .linux.apparmorProfile == "raind-default" and
    .linux.seccomp.defaultAction == "SCMP_ACT_ALLOW" and
    any(.linux.seccomp.syscalls[]?.names[]?; . == "unshare") and
    (any(.process.capabilities.effective[]?; . == "CAP_SYS_PTRACE")) and
    ([.process.capabilities.effective[]?] | index("CAP_AUDIT_WRITE") | not) and
    ([.process.capabilities.effective[]?] | index("CAP_NET_RAW") | not) and
    ([.process.capabilities.effective[]?] | index("CAP_MKNOD") | not)
  ' "${config}" >/dev/null || fail "custom security profile was not written to ${config}"

  assert_security_profile_runtime_applied "${cid}"
}

write_static_build_context() {
  local dir="$1"
  mkdir -p "${dir}/assets"
  cat >"${dir}/Dockerfile" <<'DOCKERFILE'
FROM busybox:latest
COPY message.txt /message.txt
CMD ["cat", "/message.txt"]
DOCKERFILE
  printf 'raind image build e2e\n' >"${dir}/message.txt"
  printf 'asset\n' >"${dir}/assets/file.txt"
}

test_image() {
  local tag="local/e2e-image-${SUFFIX}:latest"
  local context="${E2E_WORK_DIR}/image-context"

  log "image test"
  run_raind image-pull-busybox image pull busybox:latest
  assert_output_contains image-pull-busybox "pull completed"
  run_raind image-pull-nginx image pull nginx:latest
  assert_output_contains image-pull-nginx "pull completed"

  write_static_build_context "${context}"
  run_raind image-build image build -t "${tag}" "${context}"
  assert_output_contains image-build "build completed"
  run_raind image-ls image ls
  assert_output_contains image-ls "busybox"
  assert_output_contains image-ls "nginx"
  assert_output_contains image-ls "local/e2e-image-${SUFFIX}"

  local built_cache="/etc/raind/image/layers/e2e-image-${SUFFIX}/latest/rootless-shifted"
  sudo_cmd mkdir -p "${built_cache}/sentinel"
  assert_sudo_path_exists "${built_cache}/sentinel"
  run_raind image-rm image rm "${tag}"
  assert_output_contains image-rm "remove completed"
  assert_sudo_path_absent "${built_cache}"
}

test_network() {
  local bridge="e2enet${SUFFIX}"
  local cid

  log "network test"
  run_raind network-create network create "${bridge}"
  assert_output_contains network-create "created"
  run_raind network-ls network ls
  assert_output_contains network-ls "${bridge}"

  run_raind network-container-create container create --network "${bridge}" --name "e2e-net-${SUFFIX}" busybox:latest sleep 30
  cid="$(extract_created_id network-container-create)"
  [[ -n "${cid}" ]] || fail "network container id not found"
  run_raind network-container-ls container ls
  assert_output_contains network-container-ls "e2e-net-${SUFFIX}"
  cleanup_container network-container-rm "${cid}"
  run_raind network-rm network rm "${bridge}"
  assert_output_contains network-rm "delete network"
}

test_policy() {
  local source_id
  local dest_id
  local policy_id

  log "policy test"
  run_raind policy-source-create container create --name "e2e-policy-src-${SUFFIX}" busybox:latest sh -c 'trap "exit 0" TERM INT; while true; do sleep 1; done'
  source_id="$(extract_created_id policy-source-create)"
  run_raind policy-dest-create container create --name "e2e-policy-dst-${SUFFIX}" busybox:latest sh -c 'trap "exit 0" TERM INT; while true; do sleep 1; done'
  dest_id="$(extract_created_id policy-dest-create)"
  [[ -n "${source_id}" && -n "${dest_id}" ]] || fail "policy container ids not found"

  run_raind policy-add security policy add --type ew --source "e2e-policy-src-${SUFFIX}" --destination "e2e-policy-dst-${SUFFIX}" --protocol tcp --dport 80 --comment "e2e policy"
  policy_id="$(extract_policy_id policy-add)"
  [[ -n "${policy_id}" ]] || fail "policy id not found"
  run_raind policy-ls security policy ls --type ew
  assert_output_contains policy-ls "${policy_id}"
  run_raind policy-rm security policy rm "${policy_id}"
  assert_output_contains policy-rm "remove"
  run_raind policy-revert security policy revert
  assert_output_contains policy-revert "revert success"
  run_raind policy-ns-mode-enforce security policy ns-mode enforce
  assert_output_contains policy-ns-mode-enforce "enforce"
  run_raind policy-ns-mode-observe security policy ns-mode observe
  assert_output_contains policy-ns-mode-observe "observe"

  cleanup_container policy-source-rm "${source_id}"
  cleanup_container policy-dest-rm "${dest_id}"
}

test_container_deploy() {
  local port=$((18100 + SUFFIX % 1000))
  local cid

  log "container deploy test"
  run_raind container-create container create --name "e2e-web-${SUFFIX}" -p "${port}:80" nginx:latest
  cid="$(extract_created_id container-create)"
  [[ -n "${cid}" ]] || fail "container id not found"
  run_raind container-start container start "${cid}"
  assert_output_contains container-start "started"
  wait_http_ok "http://${HOST_ADDR}:${port}/"
  run_raind container-inspect container inspect "${cid}"
  assert_output_contains container-inspect "Security Profile:"
  assert_output_contains container-inspect "default"
  run_raind container-inspect-json container inspect "${cid}" --json
  jq -e '.containerId == "'"${cid}"'" and .securityProfile == "default"' "${E2E_WORK_DIR}/container-inspect-json.out" >/dev/null || fail "container inspect json missing expected id/security profile"
  jq -e '(.config.process.capabilities? == null) and (.config.linux.seccomp? == null) and (.config.linux.apparmorProfile? == null)' "${E2E_WORK_DIR}/container-inspect-json.out" >/dev/null || fail "container inspect json leaked security implementation details"
  run_raind_allow_empty container-exec container exec "${cid}" nginx -v
  run_raind_allow_empty container-logs container logs --line 20 "${cid}"
  run_raind_allow_empty container-stop container stop "${cid}"
  cleanup_container container-rm "${cid}"
}

test_container_exit_status() {
  local cid

  log "container exit status test"

  run_raind non-tty-exit-42 container run --name "e2e-exit-42-${SUFFIX}" alpine:latest sh -c 'exit 42'
  cid="$(extract_created_id non-tty-exit-42)"
  cid="$(resolve_container_id "${cid}" "e2e-exit-42-${SUFFIX}")"
  wait_container_stopped "${cid}"
  assert_container_exit_status "${cid}" 42 "Error" "exit status 42"
  cleanup_container non-tty-exit-42-rm "${cid}"

  run_raind non-tty-shell-eof container run --name "e2e-shell-eof-${SUFFIX}" alpine:latest
  cid="$(extract_created_id non-tty-shell-eof)"
  cid="$(resolve_container_id "${cid}" "e2e-shell-eof-${SUFFIX}")"
  wait_container_stopped "${cid}"
  assert_container_exit_status "${cid}" 0 "Completed" "exit code: 0"
  cleanup_container non-tty-shell-eof-rm "${cid}"

  # Do not use `container run -t` here: run attaches to the TTY after start,
  # which is useful for humans but can block a non-interactive e2e script. Use
  # create -t + start -t instead so the runtime still exercises the TTY shim
  # path while the CLI remains non-interactive.
  run_raind tty-exit-42-create container create -t --name "e2e-tty-exit-42-${SUFFIX}" alpine:latest sh -c 'exit 42'
  cid="$(extract_created_id tty-exit-42-create)"
  cid="$(resolve_container_id "${cid}" "e2e-tty-exit-42-${SUFFIX}")"
  run_raind tty-exit-42-start container start -t "${cid}"
  wait_container_stopped "${cid}"
  assert_container_exit_status "${cid}" 42 "Error" "exit status 42"
  cleanup_container tty-exit-42-rm "${cid}"
}

test_dev_security_profile_container() {
  local cid
  local name="e2e-dev-profile-${SUFFIX}"

  log "dev security profile container test"
  run_raind dev-profile-run container run --security-profile dev --name "${name}" busybox:latest sh -c 'trap "exit 0" TERM INT; while true; do sleep 1; done'
  cid="$(extract_created_id dev-profile-run)"
  cid="$(resolve_container_id "${cid}" "${name}")"
  assert_dev_security_profile_applied "${cid}"
  run_raind_allow_empty dev-profile-stop container stop "${cid}"
  cleanup_container dev-profile-rm "${cid}"
}

test_deploy_security_profile_container() {
  local cid
  local name="e2e-deploy-profile-${SUFFIX}"

  log "deploy security profile container test"
  run_raind deploy-profile-run container run --security-profile deploy --name "${name}" busybox:latest sh -c 'trap "exit 0" TERM INT; while true; do sleep 1; done'
  cid="$(extract_created_id deploy-profile-run)"
  cid="$(resolve_container_id "${cid}" "${name}")"
  assert_deploy_security_profile_applied "${cid}"
  run_raind_allow_empty deploy-profile-stop container stop "${cid}"
  cleanup_container deploy-profile-rm "${cid}"
}

test_restricted_security_profile_container() {
  local cid
  local name="e2e-restricted-profile-${SUFFIX}"

  log "restricted security profile container test"
  run_raind restricted-profile-run container run --security-profile restricted --name "${name}" busybox:latest sh -c 'trap "exit 0" TERM INT; while true; do sleep 1; done'
  cid="$(extract_created_id restricted-profile-run)"
  cid="$(resolve_container_id "${cid}" "${name}")"
  assert_restricted_security_profile_applied "${cid}"
  run_raind_allow_empty restricted-profile-stop container stop "${cid}"
  cleanup_container restricted-profile-rm "${cid}"
}

test_unconfined_security_profile_container() {
  local cid
  local name="e2e-unconfined-profile-${SUFFIX}"

  log "unconfined security profile container test"
  run_raind unconfined-profile-run container run --security-profile unconfined --name "${name}" busybox:latest sh -c 'trap "exit 0" TERM INT; while true; do sleep 1; done'
  cid="$(extract_created_id unconfined-profile-run)"
  cid="$(resolve_container_id "${cid}" "${name}")"
  assert_unconfined_security_profile_applied "${cid}"
  run_raind_allow_empty unconfined-profile-stop container stop "${cid}"
  cleanup_container unconfined-profile-rm "${cid}"
}

test_custom_security_profile_container() {
  local cid
  local name="e2e-custom-profile-container-${SUFFIX}"
  local profile="e2e-custom-profile-${SUFFIX}"

  log "custom security profile register and container test"
  cat >"${E2E_WORK_DIR}/custom-security-profile.yaml" <<YAML
apiVersion: raind.io/v1
kind: SecurityProfile
metadata:
  name: ${profile}
spec:
  extends: deploy
  add-cap:
    - CAP_SYS_PTRACE
  drop-cap:
    - CAP_AUDIT_WRITE
YAML
  run_raind custom-profile-register security profile register -f "${E2E_WORK_DIR}/custom-security-profile.yaml"
  assert_output_contains custom-profile-register "security profile: ${profile} registered"
  run_raind custom-profile-show security profile show "${profile}"
  assert_output_contains custom-profile-show "name: ${profile}"
  assert_output_contains custom-profile-show "type: custom"
  assert_output_contains custom-profile-show "extends: deploy"
  assert_output_contains custom-profile-show "CAP_SYS_PTRACE"

  run_raind custom-profile-run container run --security-profile "${profile}" --name "${name}" busybox:latest sh -c 'trap "exit 0" TERM INT; while true; do sleep 1; done'
  cid="$(extract_created_id custom-profile-run)"
  cid="$(resolve_container_id "${cid}" "${name}")"
  assert_custom_security_profile_applied "${cid}"
  run_raind custom-profile-inspect container inspect "${cid}" --json
  jq -e '.securityProfile == "'"${profile}"'"' "${E2E_WORK_DIR}/custom-profile-inspect.out" >/dev/null || fail "custom profile was not stored in container inspect"
  run_raind_allow_empty custom-profile-stop container stop "${cid}"
  cleanup_container custom-profile-rm "${cid}"
  run_raind custom-profile-delete security profile delete "${profile}"
  assert_output_contains custom-profile-delete "security profile: ${profile} deleted"
  assert_raind_fails custom-profile-show-deleted security profile show "${profile}"
}

test_rootless_container_cache() {
  local first_port=$((18600 + SUFFIX % 1000))
  local second_port=$((19600 + SUFFIX % 1000))
  local first_id
  local second_id
  local nginx_cache="/etc/raind/image/layers/library/nginx/latest/rootless-shifted"

  log "rootless container cache test"
  sudo_cmd rm -rf "${nginx_cache}"

  run_raind_retry_transient rootless-run-first container run --rootless --name "e2e-rootless-a-${SUFFIX}" -p "${first_port}:80" nginx:latest
  assert_output_contains rootless-run-first "creating rootless shifted layer cache"
  wait_http_ok "http://${HOST_ADDR}:${first_port}/"
  first_id="$(extract_created_id rootless-run-first)"
  if [[ -z "${first_id}" ]]; then
    first_id="e2e-rootless-a-${SUFFIX}"
  fi
  assert_sudo_path_exists "${nginx_cache}"
  assert_sudo_path_exists "${nginx_cache}/uid_100000_gid_100000_size_65536_v1/.raind-rootless-shift-complete"
  run_raind_allow_empty rootless-stop-first container stop "${first_id}"
  cleanup_container rootless-rm-first "${first_id}"

  run_raind_retry_transient rootless-run-second container run --rootless --name "e2e-rootless-b-${SUFFIX}" -p "${second_port}:80" nginx:latest
  assert_output_contains rootless-run-second "rootless shifted layer cache found"
  wait_http_ok "http://${HOST_ADDR}:${second_port}/"
  second_id="$(extract_created_id rootless-run-second)"
  if [[ -z "${second_id}" ]]; then
    second_id="e2e-rootless-b-${SUFFIX}"
  fi
  run_raind_allow_empty rootless-stop-second container stop "${second_id}"
  cleanup_container rootless-rm-second "${second_id}"
}

test_login_rootless_bind_mount() {
  local cid
  local login_uid
  local login_gid
  local bind_dir="${E2E_WORK_DIR}/login-root-bind"
  local cache_root
  local shifted_cache_root="/etc/raind/image/layers/library/nginx/latest/rootless-shifted/uid_100000_gid_100000_size_65536_v1"
  local host_owner
  local name="e2e-login-root-${SUFFIX}"

  log "login-root rootless bind mount test"
  login_uid="$(id -u)"
  login_gid="$(id -g)"
  if [[ "${login_uid}" == "0" || "${login_gid}" == "0" ]]; then
    log "skip login-root rootless bind mount test: needs a non-root login user"
    return
  fi

  rm -rf "${bind_dir}"
  mkdir -p "${bind_dir}"
  chmod 0775 "${bind_dir}"

  cache_root="/etc/raind/image/layers/library/nginx/latest/rootless-shifted/mode_login-root_rootuid_${login_uid}_rootgid_${login_gid}_uid_100000_gid_100000_size_65536_v1"
  sudo_cmd rm -rf "${cache_root}"
  assert_sudo_path_exists "${shifted_cache_root}/.raind-rootless-shift-complete"

  run_raind login-root-create container create --rootless-mode login-root --name "${name}" -v "${bind_dir}:/data" nginx:latest /bin/sh -c "sleep 60"
  cid="$(extract_created_id login-root-create)"
  [[ -n "${cid}" ]] || fail "login-root container id not found"
  run_raind login-root-start container start "${cid}"
  assert_output_contains login-root-start "started"
  assert_sudo_path_exists "${cache_root}/rootfs"
  assert_sudo_path_exists "${cache_root}/.raind-rootless-shift-complete"
  assert_sudo_path_exists "${shifted_cache_root}/.raind-rootless-shift-complete"

  run_raind_allow_empty login-root-exec container exec "${cid}" /bin/sh -c "echo login-root-e2e > /data/hello.txt && cat /data/hello.txt"
  if ! wait_file_contains "${bind_dir}/hello.txt" "login-root-e2e"; then
    printf '%s\n' "--- ${E2E_WORK_DIR}/login-root-exec.out ---" >&2
    cat "${E2E_WORK_DIR}/login-root-exec.out" >&2 || true
    run_raind_allow_empty login-root-logs container logs --line 80 "${cid}" || true
    printf '%s\n' "--- ${E2E_WORK_DIR}/login-root-logs.out ---" >&2
    cat "${E2E_WORK_DIR}/login-root-logs.out" >&2 || true
    fail "login-root bind file was not created with expected content on host"
  fi
  host_owner="$(stat -c '%u:%g' "${bind_dir}/hello.txt")"
  [[ "${host_owner}" == "${login_uid}:${login_gid}" ]] || fail "unexpected login-root bind file owner: ${host_owner}, expected ${login_uid}:${login_gid}"

  run_raind_allow_empty login-root-stop container stop "${cid}"
  cleanup_container login-root-rm "${cid}"
  rm -rf "${bind_dir}"
}


wait_rootless_pod_runtime_state() {
  local ns="$1"
  local pod_name="$2"
  local psm_path="${PSM_STORE}"
  local csm_path="${CSM_STORE}"
  local pod_id
  local container_id
  local container_pid

  for _ in $(seq 1 120); do
    pod_id="$(sudo_cmd jq -r --arg ns "${ns}" --arg name "${pod_name}" '
      .pods
      | to_entries[]?
      | select(.value.namespace == $ns and .value.name == $name and .value.rootless == true and (.value.userNS // "") != "")
      | .key
    ' "${psm_path}" 2>/dev/null | head -1)"

    if [[ -n "${pod_id}" ]]; then
      container_id="$(sudo_cmd jq -r --arg pod "${pod_id}" '
        .containers
        | to_entries[]?
        | select(.value.podId == $pod and ((.value.name // "") | startswith("condenser-pod-infra-") | not))
        | .key
      ' "${csm_path}" 2>/dev/null | head -1)"

      container_pid="$(sudo_cmd jq -r --arg cid "${container_id}" '
        .containers[$cid].pid // 0
      ' "${csm_path}" 2>/dev/null || true)"

      if [[ -n "${container_id}" && "${container_pid}" =~ ^[0-9]+$ && "${container_pid}" -gt 0 ]] && \
        sudo_cmd test -f "/etc/raind/container/${container_id}/config.json" && \
        sudo_cmd jq -e '
          (.annotations["io.raind.rootless"] | fromjson).enabled == true
        ' "/etc/raind/container/${container_id}/config.json" >/dev/null 2>&1; then
        if ! sudo_cmd cmp -s "/proc/${container_pid}/uid_map" /proc/1/uid_map; then
          return
        fi
      fi
    fi

    sleep 0.5
  done

  printf '%s\n' "--- ${psm_path} ---" >&2
  sudo_cmd cat "${psm_path}" >&2 || true
  printf '%s\n' "--- ${csm_path} ---" >&2
  sudo_cmd cat "${csm_path}" >&2 || true
  if [[ -n "${container_id:-}" ]]; then
    printf '%s\n' "--- /etc/raind/container/${container_id}/config.json ---" >&2
    sudo_cmd cat "/etc/raind/container/${container_id}/config.json" >&2 || true
    if [[ -n "${container_pid:-}" && "${container_pid}" =~ ^[0-9]+$ && "${container_pid}" -gt 0 ]]; then
      printf '%s\n' "--- /proc/${container_pid}/uid_map ---" >&2
      sudo_cmd cat "/proc/${container_pid}/uid_map" >&2 || true
    fi
  fi
  fail "timed out waiting for rootless pod runtime state"
}


test_rootless_resource_pod() {
  local yaml="${E2E_WORK_DIR}/rootless-pod.yaml"
  local ns="e2e-rootless-pod-ns-${SUFFIX}"
  local pod_name="e2e-rootless-pod"

  log "rootless resource pod test"
  cat >"${yaml}" <<YAML
apiVersion: v1
kind: Namespace
metadata:
  name: ${ns}
---
apiVersion: v1
kind: Pod
metadata:
  name: ${pod_name}
  namespace: ${ns}
  labels:
    app: e2e-rootless-pod
spec:
  hostUsers: false
  containers:
  - name: worker
    image: busybox:latest
    command:
    - sh
    - -c
    - 'trap "exit 0" TERM INT; while true; do sleep 1; done'
YAML
  run_resource_apply_with_retry rootless-pod-apply "${yaml}"
  assert_output_contains rootless-pod-apply "pod:"
  wait_raind_contains rootless-pod-ls "${pod_name}" resource pod ls --namespace "${ns}"
  wait_pod_row_ready rootless-pod-ready "${pod_name}" resource pod ls --namespace "${ns}"
  wait_rootless_pod_runtime_state "${ns}" "${pod_name}"
  run_raind rootless-pod-rm resource rm -f "${yaml}"
  assert_output_contains rootless-pod-rm "pod:"
  assert_output_contains rootless-pod-rm "namespace:"
  wait_resource_namespace_absent rootless-pod-namespace-removed "${ns}"
}

test_rootless_resource_ingress() {
  local yaml="${E2E_WORK_DIR}/rootless-ingress.yaml"
  local ns="e2e-rootless-ingress-ns-${SUFFIX}"
  local host="e2e-rootless-${SUFFIX}.local"

  log "rootless resource ingress test"
  cat >"${yaml}" <<YAML
apiVersion: v1
kind: Namespace
metadata:
  name: ${ns}
---
apiVersion: v1
kind: Pod
metadata:
  name: e2e-rootless-web
  namespace: ${ns}
  labels:
    app: e2e-rootless-web
spec:
  hostUsers: false
  containers:
  - name: nginx
    image: nginx:latest
---
apiVersion: v1
kind: Service
metadata:
  name: e2e-rootless-web
  namespace: ${ns}
spec:
  selector:
    app: e2e-rootless-web
  ports:
  - port: 80
    targetPort: 80
    protocol: TCP
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: e2e-rootless-web
  namespace: ${ns}
spec:
  rules:
  - host: ${host}
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: e2e-rootless-web
            port:
              number: 80
YAML
  run_resource_apply_with_retry rootless-ingress-apply "${yaml}"
  assert_output_contains rootless-ingress-apply "pod:"
  assert_output_contains rootless-ingress-apply "service:"
  assert_output_contains rootless-ingress-apply "ingress:"
  wait_pod_row_ready rootless-ingress-pod-ready e2e-rootless-web resource pod ls --namespace "${ns}"
  wait_rootless_pod_runtime_state "${ns}" e2e-rootless-web
  wait_raind_contains rootless-ingress-svc-ls e2e-rootless-web resource service ls --namespace "${ns}"
  wait_http_ok_host "http://${HOST_ADDR}:7780/" "${host}"
  run_raind rootless-ingress-rm resource rm -f "${yaml}"
  assert_output_contains rootless-ingress-rm "ingress:"
  assert_output_contains rootless-ingress-rm "service:"
  assert_output_contains rootless-ingress-rm "pod:"
  assert_output_contains rootless-ingress-rm "namespace:"
  wait_resource_namespace_absent rootless-ingress-namespace-removed "${ns}"
}

test_rootless_deployment_ingress() {
  local yaml="${E2E_WORK_DIR}/rootless-deployment-ingress.yaml"
  local ns="e2e-rootless-deploy-ns-${SUFFIX}"
  local host="e2e-rootless-deploy-${SUFFIX}.local"

  log "rootless deployment ingress test"
  cat >"${yaml}" <<YAML
apiVersion: v1
kind: Namespace
metadata:
  name: ${ns}
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: e2e-rootless-deploy
  namespace: ${ns}
spec:
  replicas: 2
  selector:
    matchLabels:
      app: e2e-rootless-deploy
  template:
    metadata:
      labels:
        app: e2e-rootless-deploy
    spec:
      hostUsers: false
      containers:
      - name: nginx
        image: nginx:latest
        ports:
        - containerPort: 80
---
apiVersion: v1
kind: Service
metadata:
  name: e2e-rootless-deploy
  namespace: ${ns}
spec:
  selector:
    app: e2e-rootless-deploy
  ports:
  - port: 80
    targetPort: 80
    protocol: TCP
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: e2e-rootless-deploy
  namespace: ${ns}
spec:
  rules:
  - host: ${host}
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: e2e-rootless-deploy
            port:
              number: 80
YAML
  run_resource_apply_with_retry rootless-deployment-ingress-apply "${yaml}"
  assert_output_contains rootless-deployment-ingress-apply "deployment:"
  assert_output_contains rootless-deployment-ingress-apply "service:"
  assert_output_contains rootless-deployment-ingress-apply "ingress:"
  wait_resource_row_ready rootless-deployment-ingress-ready e2e-rootless-deploy 2 resource deployment ls --namespace "${ns}"
  wait_raind_contains rootless-deployment-ingress-svc-ls e2e-rootless-deploy resource service ls --namespace "${ns}"
  wait_http_ok_host "http://${HOST_ADDR}:7780/" "${host}"
  run_raind rootless-deployment-ingress-rm resource rm -f "${yaml}"
  assert_output_contains rootless-deployment-ingress-rm "ingress:"
  assert_output_contains rootless-deployment-ingress-rm "service:"
  assert_output_contains rootless-deployment-ingress-rm "deployment:"
  assert_output_contains rootless-deployment-ingress-rm "namespace:"
  wait_resource_namespace_absent rootless-deployment-ingress-namespace-removed "${ns}"
}

test_bottle_deploy() {
  local port=$((19100 + SUFFIX % 1000))
  local yaml="${E2E_WORK_DIR}/bottle.yaml"
  local name="e2e-bottle-${SUFFIX}"

  log "bottle deploy test"
  cat >"${yaml}" <<YAML
bottle:
  name: ${name}
services:
  web:
    image: nginx:latest
    ports:
      - "${port}:80"
YAML
  run_raind bottle-create bottle create -f "${yaml}"
  assert_output_contains bottle-create "created"
  run_raind bottle-ls bottle ls
  assert_output_contains bottle-ls "${name}"
  run_raind_allow_empty bottle-start bottle start "${name}"
  wait_http_ok "http://${HOST_ADDR}:${port}/"
  run_raind bottle-show bottle show "${name}"
  assert_output_contains bottle-show "${name}"
  run_raind_allow_empty bottle-stop bottle stop "${name}"
  run_raind_allow_empty bottle-rm bottle rm "${name}"
}

test_resource_namespace() {
  local ns="e2e-ns-${SUFFIX}"

  log "resource namespace test"
  run_raind ns-create resource namespace create "${ns}"
  assert_output_contains ns-create "created"
  run_raind ns-show resource namespace show "${ns}"
  assert_output_contains ns-show "${ns}"
  run_raind ns-ls resource namespace ls
  assert_output_contains ns-ls "${ns}"
  run_raind ns-rm resource namespace rm "${ns}"
  assert_output_contains ns-rm "removed"
}

test_resource_pod() {
  local yaml="${E2E_WORK_DIR}/pod.yaml"
  local ns="e2e-pod-ns-${SUFFIX}"

  log "resource pod test"
  cat >"${yaml}" <<YAML
apiVersion: v1
kind: Namespace
metadata:
  name: ${ns}
---
apiVersion: v1
kind: Pod
metadata:
  name: e2e-pod
  namespace: ${ns}
  labels:
    app: e2e-pod
spec:
  containers:
  - name: nginx
    image: nginx:latest
YAML
  run_resource_apply_with_retry pod-apply "${yaml}"
  assert_output_contains pod-apply "pod:"
  wait_raind_contains pod-ls "e2e-pod" resource pod ls --namespace "${ns}"
  wait_pod_row_ready pod-ready e2e-pod resource pod ls --namespace "${ns}"
  run_raind pod-rm-manifest resource rm -f "${yaml}"
  assert_output_contains pod-rm-manifest "pod:"
  assert_output_contains pod-rm-manifest "namespace:"
  wait_resource_namespace_absent pod-namespace-removed "${ns}"
}

test_resource_configmap() {
  local yaml="${E2E_WORK_DIR}/configmap.yaml"
  local ns="e2e-cm-ns-${SUFFIX}"

  log "resource configmap test"
  cat >"${yaml}" <<YAML
apiVersion: v1
kind: Namespace
metadata:
  name: ${ns}
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: app-config
  namespace: ${ns}
data:
  APP_ENV: e2e
  LOG_LEVEL: info
  OVERRIDE_ME: from-envfrom
---
apiVersion: v1
kind: Pod
metadata:
  name: e2e-cm-pod
  namespace: ${ns}
  labels:
    app: e2e-cm-pod
spec:
  containers:
  - name: app
    image: busybox:latest
    command:
    - sh
    - -c
    - 'trap "exit 0" TERM INT; while true; do sleep 1; done'
    envFrom:
    - configMapRef:
        name: app-config
    env:
    - name: SINGLE_KEY
      valueFrom:
        configMapKeyRef:
          name: app-config
          key: APP_ENV
    - name: OVERRIDE_ME
      value: explicit
YAML
  run_resource_apply_with_retry configmap-apply "${yaml}"
  assert_output_contains configmap-apply "configmap:"
  assert_output_contains configmap-apply "pod:"
  wait_pod_row_ready configmap-pod-ready e2e-cm-pod resource pod ls --namespace "${ns}"

  local pod_id container_name container_id
  pod_id="$(awk '/^pod: / { print $2; exit }' "${E2E_WORK_DIR}/configmap-apply.out")"
  [[ -n "${pod_id}" ]] || fail "could not extract configmap pod id"
  container_name="app-${pod_id: -8}"
  container_id="$(resolve_container_id "${container_name}" "configmap-app")"
  run_raind_allow_empty configmap-env-app container exec "${container_id}" sh -c 'test "$APP_ENV" = e2e'
  run_raind_allow_empty configmap-env-single container exec "${container_id}" sh -c 'test "$SINGLE_KEY" = e2e'
  run_raind_allow_empty configmap-env-override container exec "${container_id}" sh -c 'test "$OVERRIDE_ME" = explicit'
  run_raind configmap-ls resource configmap ls --namespace "${ns}"
  assert_output_contains configmap-ls "app-config"

  run_raind configmap-rm-manifest resource rm -f "${yaml}"
  assert_output_contains configmap-rm-manifest "configmap:"
  assert_output_contains configmap-rm-manifest "pod:"
  assert_output_contains configmap-rm-manifest "namespace:"
  wait_resource_namespace_absent configmap-namespace-removed "${ns}"
}

test_resource_secret() {
  local yaml="${E2E_WORK_DIR}/secret.yaml"
  local ns="e2e-secret-ns-${SUFFIX}"
  local secret_value="super-secret-${SUFFIX}"
  local secret_b64
  secret_b64="$(printf '%s' "${secret_value}" | base64 | tr -d '\n')"

  log "resource secret test"
  cat >"${yaml}" <<YAML
apiVersion: v1
kind: Namespace
metadata:
  name: ${ns}
---
apiVersion: v1
kind: Secret
metadata:
  name: db-secret
  namespace: ${ns}
type: Opaque
data:
  DB_PASSWORD: ${secret_b64}
stringData:
  API_TOKEN: token-${SUFFIX}
  OVERRIDE_ME: from-secret
---
apiVersion: v1
kind: Pod
metadata:
  name: e2e-secret-pod
  namespace: ${ns}
  labels:
    app: e2e-secret-pod
spec:
  containers:
  - name: app
    image: busybox:latest
    command:
    - sh
    - -c
    - 'trap "exit 0" TERM INT; while true; do sleep 1; done'
    envFrom:
    - secretRef:
        name: db-secret
    env:
    - name: SINGLE_SECRET
      valueFrom:
        secretKeyRef:
          name: db-secret
          key: DB_PASSWORD
    - name: OVERRIDE_ME
      value: explicit
YAML
  run_resource_apply_with_retry secret-apply "${yaml}"
  assert_output_contains secret-apply "secret:"
  assert_output_contains secret-apply "pod:"
  assert_output_not_contains secret-apply "${secret_value}"
  wait_pod_row_ready secret-pod-ready e2e-secret-pod resource pod ls --namespace "${ns}"

  local pod_id container_name container_id
  pod_id="$(awk '/^pod: / { print $2; exit }' "${E2E_WORK_DIR}/secret-apply.out")"
  [[ -n "${pod_id}" ]] || fail "could not extract secret pod id"
  container_name="app-${pod_id: -8}"
  container_id="$(resolve_container_id "${container_name}" "secret-app")"
  run_raind_allow_empty secret-env-password container exec "${container_id}" sh -c "test \"\$DB_PASSWORD\" = '${secret_value}'"
  run_raind_allow_empty secret-env-single container exec "${container_id}" sh -c "test \"\$SINGLE_SECRET\" = '${secret_value}'"
  run_raind_allow_empty secret-env-token container exec "${container_id}" sh -c "test \"\$API_TOKEN\" = 'token-${SUFFIX}'"
  run_raind_allow_empty secret-env-override container exec "${container_id}" sh -c 'test "$OVERRIDE_ME" = explicit'
  run_raind secret-ls resource secret ls --namespace "${ns}"
  assert_output_contains secret-ls "db-secret"
  assert_output_not_contains secret-ls "${secret_value}"
  run_raind secret-show resource secret show db-secret --namespace "${ns}"
  assert_output_contains secret-show "DB_PASSWORD"
  assert_output_contains secret-show "API_TOKEN"
  assert_output_not_contains secret-show "${secret_value}"

  run_raind secret-rm-manifest resource rm -f "${yaml}"
  assert_output_contains secret-rm-manifest "secret:"
  assert_output_contains secret-rm-manifest "pod:"
  assert_output_contains secret-rm-manifest "namespace:"
  wait_resource_namespace_absent secret-namespace-removed "${ns}"
}

test_resource_networkpolicy() {
  local base_yaml="${E2E_WORK_DIR}/networkpolicy-base.yaml"
  local policy_yaml="${E2E_WORK_DIR}/networkpolicy.yaml"
  local ns="e2e-netpol-ns-${SUFFIX}"

  log "resource networkpolicy test"
  cat >"${base_yaml}" <<YAML
apiVersion: v1
kind: Namespace
metadata:
  name: ${ns}
---
apiVersion: v1
kind: Pod
metadata:
  name: e2e-netpol-client
  namespace: ${ns}
  labels:
    app: e2e-netpol
    role: client
spec:
  containers:
  - name: client
    image: busybox:latest
    command:
    - sh
    - -c
    - 'trap "exit 0" TERM INT; while true; do sleep 1; done'
---
apiVersion: v1
kind: Pod
metadata:
  name: e2e-netpol-server
  namespace: ${ns}
  labels:
    app: e2e-netpol
    role: server
spec:
  containers:
  - name: server
    image: busybox:latest
    command:
    - sh
    - -c
    - 'trap "exit 0" TERM INT; while true; do sleep 1; done'
YAML

  cat >"${policy_yaml}" <<YAML
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-client
  namespace: ${ns}
spec:
  podSelector:
    matchLabels:
      role: server
  ingress:
  - from:
    - podSelector:
        matchLabels:
          role: client
    ports:
    - protocol: TCP
      port: 8080
YAML

  run_resource_apply_with_retry netpol-base-apply "${base_yaml}"
  assert_output_contains netpol-base-apply "pod:"
  wait_pod_row_ready netpol-client-ready e2e-netpol-client resource pod ls --namespace "${ns}"
  wait_pod_row_ready netpol-server-ready e2e-netpol-server resource pod ls --namespace "${ns}"

  run_resource_apply_with_retry netpol-apply "${policy_yaml}"
  assert_output_contains netpol-apply "networkpolicy:"
  assert_output_contains netpol-apply "generated rules: 1"
  run_raind netpol-ls resource netpol ls --namespace "${ns}"
  assert_output_contains netpol-ls "allow-client"
  run_raind netpol-show resource netpol show allow-client --namespace "${ns}"
  assert_output_contains netpol-show "GENERATED RULES"
  assert_output_contains netpol-show "1"
  sudo_cmd jq -e --arg ns "${ns}" '.networkPolicies[] | select(.name == "allow-client" and .namespace == $ns and (.generatedRuleIds | length) == 1)' "${NETPOL_STORE}" >/dev/null

  run_raind netpol-rm resource rm -f "${policy_yaml}"
  assert_output_contains netpol-rm "networkpolicy:"
  sudo_cmd jq -e --arg ns "${ns}" '[.networkPolicies[] | select(.name == "allow-client" and .namespace == $ns)] | length == 0' "${NETPOL_STORE}" >/dev/null

  run_raind netpol-base-rm resource rm -f "${base_yaml}"
  assert_output_contains netpol-base-rm "pod:"
  assert_output_contains netpol-base-rm "namespace:"
  wait_resource_namespace_absent netpol-namespace-removed "${ns}"
}

test_resource_replicaset() {
  local yaml="${E2E_WORK_DIR}/replicaset.yaml"
  local ns="e2e-rs-ns-${SUFFIX}"

  log "resource replicaset test"
  cat >"${yaml}" <<YAML
apiVersion: v1
kind: Namespace
metadata:
  name: ${ns}
---
apiVersion: apps/v1
kind: ReplicaSet
metadata:
  name: e2e-rs
  namespace: ${ns}
spec:
  replicas: 2
  selector:
    matchLabels:
      app: e2e-rs
  template:
    metadata:
      labels:
        app: e2e-rs
    spec:
      containers:
      - name: nginx
        image: nginx:latest
YAML
  run_resource_apply_with_retry rs-apply "${yaml}"
  assert_output_contains rs-apply "replicaset:"
  wait_resource_row_ready rs-ready e2e-rs 2 resource replicaset ls --namespace "${ns}"
  run_raind rs-rm-manifest resource rm -f "${yaml}"
  assert_output_contains rs-rm-manifest "replicaset:"
  assert_output_contains rs-rm-manifest "namespace:"
  wait_resource_namespace_absent rs-namespace-removed "${ns}"
}

test_resource_deployment() {
  local yaml="${E2E_WORK_DIR}/deployment.yaml"
  local ns="e2e-deploy-ns-${SUFFIX}"

  log "resource deployment test"
  cat >"${yaml}" <<YAML
apiVersion: v1
kind: Namespace
metadata:
  name: ${ns}
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: e2e-deploy
  namespace: ${ns}
spec:
  replicas: 2
  selector:
    matchLabels:
      app: e2e-deploy
  template:
    metadata:
      labels:
        app: e2e-deploy
    spec:
      containers:
      - name: nginx
        image: nginx:latest
YAML
  run_resource_apply_with_retry deploy-apply "${yaml}"
  assert_output_contains deploy-apply "deployment:"
  wait_resource_row_ready deploy-ready e2e-deploy 2 resource deployment ls --namespace "${ns}"
  run_raind deploy-rm-manifest resource rm -f "${yaml}"
  assert_output_contains deploy-rm-manifest "deployment:"
  assert_output_contains deploy-rm-manifest "namespace:"
  wait_resource_namespace_absent deploy-namespace-removed "${ns}"
}

test_resource_service() {
  local port=$((20100 + SUFFIX % 1000))
  local yaml="${E2E_WORK_DIR}/service.yaml"
  local ns="e2e-svc-ns-${SUFFIX}"

  log "resource service test"
  cat >"${yaml}" <<YAML
apiVersion: v1
kind: Namespace
metadata:
  name: ${ns}
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: e2e-svc-web
  namespace: ${ns}
spec:
  replicas: 2
  selector:
    matchLabels:
      app: e2e-svc-web
  template:
    metadata:
      labels:
        app: e2e-svc-web
    spec:
      containers:
      - name: nginx
        image: nginx:latest
---
apiVersion: v1
kind: Service
metadata:
  name: e2e-svc
  namespace: ${ns}
spec:
  type: NodePort
  selector:
    app: e2e-svc-web
  ports:
  - port: ${port}
    targetPort: 80
    protocol: TCP
YAML
  run_resource_apply_with_retry svc-apply "${yaml}"
  assert_output_contains svc-apply "service:"
  wait_resource_row_ready svc-deploy-ready e2e-svc-web 2 resource deployment ls --namespace "${ns}"
  wait_raind_contains svc-ls "e2e-svc" resource service ls --namespace "${ns}"
  wait_http_ok "http://${HOST_ADDR}:${port}/"
  run_raind svc-rm-manifest resource rm -f "${yaml}"
  assert_output_contains svc-rm-manifest "service:"
  assert_output_contains svc-rm-manifest "deployment:"
  assert_output_contains svc-rm-manifest "namespace:"
  wait_resource_namespace_absent svc-namespace-removed "${ns}"
}

test_resource_yaml_all_kinds() {
  local port=$((21100 + SUFFIX % 1000))
  local yaml="${E2E_WORK_DIR}/all-kinds.yaml"
  local ns="e2e-yaml-ns-${SUFFIX}"

  log "resource yaml all-kinds test"
  cat >"${yaml}" <<YAML
apiVersion: v1
kind: Namespace
metadata:
  name: ${ns}
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: e2e-yaml-config
  namespace: ${ns}
data:
  APP_ENV: all-kinds
---
apiVersion: v1
kind: Secret
metadata:
  name: e2e-yaml-secret
  namespace: ${ns}
type: Opaque
stringData:
  DB_PASSWORD: all-kinds-secret
---
apiVersion: v1
kind: Pod
metadata:
  name: e2e-yaml-pod
  namespace: ${ns}
  labels:
    app: e2e-yaml-pod
spec:
  containers:
  - name: nginx
    image: nginx:latest
    envFrom:
    - configMapRef:
        name: e2e-yaml-config
    - secretRef:
        name: e2e-yaml-secret
---
apiVersion: apps/v1
kind: ReplicaSet
metadata:
  name: e2e-yaml-rs
  namespace: ${ns}
spec:
  replicas: 1
  selector:
    matchLabels:
      app: e2e-yaml-rs
  template:
    metadata:
      labels:
        app: e2e-yaml-rs
    spec:
      containers:
      - name: nginx
        image: nginx:latest
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: e2e-yaml-deploy
  namespace: ${ns}
spec:
  replicas: 1
  selector:
    matchLabels:
      app: e2e-yaml-deploy
  template:
    metadata:
      labels:
        app: e2e-yaml-deploy
    spec:
      containers:
      - name: nginx
        image: nginx:latest
---
apiVersion: v1
kind: Service
metadata:
  name: e2e-yaml-svc
  namespace: ${ns}
spec:
  type: NodePort
  selector:
    app: e2e-yaml-deploy
  ports:
  - port: ${port}
    targetPort: 80
    protocol: TCP
YAML
  run_resource_apply_with_retry yaml-apply "${yaml}"
  assert_output_contains yaml-apply "namespace:"
  assert_output_contains yaml-apply "configmap:"
  assert_output_contains yaml-apply "secret:"
  assert_output_contains yaml-apply "pod:"
  assert_output_contains yaml-apply "replicaset:"
  assert_output_contains yaml-apply "deployment:"
  assert_output_contains yaml-apply "service:"
  wait_resource_row_ready yaml-deploy-ready e2e-yaml-deploy 1 resource deployment ls --namespace "${ns}"
  wait_resource_row_ready yaml-rs-ready e2e-yaml-rs 1 resource replicaset ls --namespace "${ns}"
  wait_http_ok "http://${HOST_ADDR}:${port}/"
  run_raind yaml-rm resource rm -f "${yaml}"
  assert_output_contains yaml-rm "namespace:"
  assert_output_contains yaml-rm "configmap:"
  assert_output_contains yaml-rm "secret:"
  assert_output_contains yaml-rm "pod:"
  assert_output_contains yaml-rm "replicaset:"
  assert_output_contains yaml-rm "deployment:"
  assert_output_contains yaml-rm "service:"
  wait_resource_namespace_absent yaml-namespace-removed "${ns}"
}

main() {
  require_workshop
  rm -rf "${E2E_WORK_DIR}"
  mkdir -p "${E2E_WORK_DIR}"

  build_components
  prepare_runtime
  cleanup_stale_condenser
  start_condenser
  trap stop_condenser EXIT
  wait_ready

  test_image
  test_network
  test_policy
  test_container_deploy
  test_container_exit_status
  test_dev_security_profile_container
  test_deploy_security_profile_container
  test_restricted_security_profile_container
  test_unconfined_security_profile_container
  test_custom_security_profile_container
  test_rootless_container_cache
  test_rootless_resource_pod
  test_rootless_resource_ingress
  test_rootless_deployment_ingress
  test_login_rootless_bind_mount
  test_bottle_deploy
  test_resource_namespace
  test_resource_pod
  test_resource_configmap
  test_resource_secret
  test_resource_networkpolicy
  test_resource_replicaset
  test_resource_deployment
  test_resource_service
  test_resource_yaml_all_kinds

  log "raind deploy e2e completed"
}

main "$@"
