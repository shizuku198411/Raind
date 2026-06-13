#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DROPLET_BIN="${ROOT_DIR}/bin/droplet"
E2E_WORK_DIR="${E2E_WORK_DIR:-/tmp/raind-droplet-e2e}"
RUNTIME_CID="${RAIND_DROPLET_E2E_ID:-raind-e2e-$$}"
RUN_RUNTIME="${RAIND_DROPLET_E2E_RUNTIME:-auto}"
REQUIRE_RUNTIME="${RAIND_DROPLET_E2E_REQUIRE_RUNTIME:-0}"

log() {
  printf '==> %s\n' "$*"
}

fail() {
  printf 'error: %s\n' "$*" >&2
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
    fail "sudo is required for droplet runtime e2e setup"
  fi
  sudo -n "$@"
}

droplet() {
  "${DROPLET_BIN}" "$@"
}

sudo_droplet() {
  sudo_cmd env PATH="${PATH}" "${DROPLET_BIN}" "$@"
}

assert_command_fails() {
  local name="$1"
  shift
  local out="${E2E_WORK_DIR}/${name}.err"

  if "$@" >"${out}" 2>&1; then
    cat "${out}" >&2 || true
    fail "expected command to fail: $*"
  fi
}

assert_sudo_file_exists() {
  local path="$1"
  sudo_cmd test -f "${path}" || fail "expected file to exist: ${path}"
}

assert_sudo_path_exists() {
  local path="$1"
  sudo_cmd test -e "${path}" || fail "expected path to exist: ${path}"
}

assert_sudo_fifo_exists() {
  local path="$1"
  sudo_cmd test -p "${path}" || fail "expected fifo to exist: ${path}"
}

assert_sudo_path_absent() {
  local path="$1"
  sudo_cmd test ! -e "${path}" || fail "expected path to be absent: ${path}"
}

assert_audit_event() {
  local cid="$1"
  local event="$2"

  sudo_cmd jq -e --arg cid "${cid}" --arg event "${event}" \
    'select(.container_id == $cid and .event == $event and .result == "success")' \
    /etc/raind/log/droplet_audit.log >/dev/null ||
    fail "missing successful audit event=${event} container=${cid}"
}

dump_runtime_debug() {
  local cid="$1"
  local container_dir="/etc/raind/container/${cid}"
  local pid=""

  echo "===== droplet runtime debug: ${cid} =====" >&2

  echo "----- command/tool info -----" >&2
  {
    echo "DROPLET_BIN=${DROPLET_BIN}"
    echo "E2E_WORK_DIR=${E2E_WORK_DIR}"
    echo "RUNTIME_CID=${cid}"
    echo "busybox=$(command -v busybox 2>/dev/null || true)"
    if have_cmd busybox; then
      file "$(command -v busybox)" 2>/dev/null || true
      ldd "$(command -v busybox)" 2>/dev/null || true
    fi
  } >&2

  echo "----- process security context -----" >&2
  {
    grep -E 'NoNewPrivs|Seccomp|CapEff|CapPrm|CapBnd' "/proc/$$/status" 2>/dev/null || true
    cat "/proc/$$/attr/current" 2>/dev/null || true
    cat "/proc/$$/uid_map" 2>/dev/null || true
    cat "/proc/$$/gid_map" 2>/dev/null || true
  } >&2

  echo "----- droplet state -----" >&2
  sudo_droplet state "${cid}" >&2 2>/dev/null || true

  echo "----- state.json -----" >&2
  sudo_cmd cat "${container_dir}/state.json" >&2 2>/dev/null || true

  if sudo_cmd test -f "${container_dir}/state.json"; then
    pid="$(sudo_cmd jq -r '.pid // 0' "${container_dir}/state.json" 2>/dev/null || true)"
  fi
  if [[ "${pid}" =~ ^[0-9]$ ]] && [[ "${pid}" -gt 0 ]]; then
    echo "----- init process status: ${pid} -----" >&2
    sudo_cmd cat "/proc/${pid}/status" >&2 2>/dev/null || true
  fi

  echo "----- container files -----" >&2
  sudo_cmd find "${container_dir}" -maxdepth 4 -type f -print >&2 2>/dev/null || true

  echo "----- init.log -----" >&2
  sudo_cmd cat "${container_dir}/logs/init.log" >&2 2>/dev/null || true

  echo "----- droplet audit log -----" >&2
  sudo_cmd tail -n 100 /etc/raind/log/droplet_audit.log >&2 2>/dev/null || true
}

assert_cgroup_file_exists() {
  local cid="$1"
  local file="$2"
  local path="/sys/fs/cgroup/raind/${cid}/${file}"

  sudo_cmd test -e "${path}" || fail "expected cgroup file: ${path}"
}

build_droplet() {
  log "build all raind components"
  cd "${ROOT_DIR}"
  ./scripts/build.sh build
  [[ -x "${DROPLET_BIN}" ]] || fail "missing built droplet binary: ${DROPLET_BIN}"
}

require_workshop() {
  if [[ "${ROOT_DIR}" != /project* ]]; then
    fail "droplet integration test must run inside Workshop. use: workshop run raind-dev -- test-droplet-integ"
  fi
}

setup_audit_log() {
  log "prepare droplet audit log"
  sudo_cmd mkdir -p /etc/raind/log
  sudo_cmd touch /etc/raind/log/droplet_audit.log
  sudo_cmd chmod 0666 /etc/raind/log/droplet_audit.log
}

run_smoke_e2e() {
  local smoke_dir="${E2E_WORK_DIR}/smoke"
  local root_dir="${smoke_dir}/state-root"
  local bundle_dir="${root_dir}/smoke-1"
  local dead_bundle_dir="${root_dir}/smoke-dead"
  local rootfs="${smoke_dir}/rootfs"
  local layer="${smoke_dir}/layer"
  local upper="${smoke_dir}/upper"
  local work="${smoke_dir}/work"

  log "run droplet cli smoke integration test"
  rm -rf "${smoke_dir}"
  mkdir -p "${bundle_dir}" "${dead_bundle_dir}" "${rootfs}" "${layer}" "${upper}" "${work}"

  droplet --version >/dev/null

  droplet spec \
    --rootfs "${rootfs}" \
    --cwd "/" \
    --command "/bin/sh -c 'echo smoke'" \
    --ns "mount" --ns "uts" \
    --hostname "smoke-1" \
    --if_name "" --if_addr "" \
    --image_layer "${layer}" \
    --upper_dir "${upper}" \
    --work_dir "${work}" \
    --output "${bundle_dir}"

  jq -e '.ociVersion and .root.path and .process.args[0] == "/bin/sh"' "${bundle_dir}/config.json" >/dev/null

  cat > "${bundle_dir}/state.json" <<STATE_JSON
{
  "ociVersion": "1.0.2",
  "id": "smoke-1",
  "status": "created",
  "exit_code": 0,
  "reason": "",
  "message": "",
  "pid": 0,
  "shimPid": 0,
  "rootfs": "${rootfs}",
  "bundle": "${bundle_dir}",
  "annotations": {}
}
STATE_JSON

  RAIND_ROOT_DIR="${root_dir}" droplet list --format json | jq -e 'map(select(.id == "smoke-1" and .status == "created")) | length == 1' >/dev/null
  RAIND_ROOT_DIR="${root_dir}" droplet state smoke-1 | jq -e '.id == "smoke-1" and .status == "created"' >/dev/null
  RAIND_ROOT_DIR="${root_dir}" droplet list | grep -q "ID.*STATUS.*PID.*BUNDLE"

  cat > "${dead_bundle_dir}/state.json" <<STATE_JSON
{
  "ociVersion": "1.0.2",
  "id": "smoke-dead",
  "status": "running",
  "exit_code": 0,
  "reason": "",
  "message": "",
  "pid": 999999,
  "shimPid": 0,
  "rootfs": "${rootfs}",
  "bundle": "${bundle_dir}",
  "annotations": {}
}
STATE_JSON
  RAIND_ROOT_DIR="${root_dir}" droplet state smoke-dead | jq -e '.status == "stopped"' >/dev/null
  assert_command_fails droplet-state-unknown env RAIND_ROOT_DIR="${root_dir}" "${DROPLET_BIN}" state unknown-container
}

runtime_prerequisites_available() {
  have_cmd busybox || return 1
  have_cmd jq || return 1
  have_cmd mount || return 1
  have_cmd umount || return 1
  sudo_cmd test -w /etc || return 1
  sudo_cmd test -w /sys/fs/cgroup || return 1
}

setup_cgroup() {
  local cid="$1"
  local parent="/sys/fs/cgroup/raind"

  sudo_cmd mkdir -p "${parent}"
  for controller in cpu memory pids; do
    sudo_cmd sh -c "echo +${controller} > '${parent}/cgroup.subtree_control' 2>/dev/null || true"
  done
  sudo_cmd mkdir -p "${parent}/${cid}"
}

prepare_runtime_fixture() {
  local cid="$1"
  local container_dir="/etc/raind/container/${cid}"
  local rootfs="${container_dir}/merged"
  local layer="${container_dir}/layer"
  local upper="${container_dir}/diff"
  local work="${container_dir}/work"

  log "prepare runtime fixture: ${cid}"
  cleanup_runtime_fixture "${cid}"

  sudo_cmd mkdir -p \
    "${container_dir}/etc" \
    "${container_dir}/logs" \
    "${rootfs}" \
    "${upper}" \
    "${work}" \
    "${layer}/bin" \
    "${layer}/dev" \
    "${layer}/etc" \
    "${layer}/proc" \
    "${layer}/run" \
    "${layer}/sys" \
    "${layer}/tmp"
  sudo_cmd cp "$(command -v busybox)" "${layer}/bin/busybox"
  sudo_cmd ln -sf busybox "${layer}/bin/sh"
  sudo_cmd ln -sf busybox "${layer}/bin/sleep"
  sudo_cmd chmod 0755 "${layer}/bin/busybox"
  sudo_cmd sh -c "printf 'nameserver 8.8.8.8\n' > '${container_dir}/etc/resolv.conf'"
  sudo_cmd sh -c "printf '127.0.0.1 localhost\n' > '${container_dir}/etc/hosts'"
  sudo_cmd sh -c "printf '%s\n' '${cid}' > '${container_dir}/etc/hostname'"
  setup_cgroup "${cid}"

  sudo_droplet spec \
    --rootfs "${rootfs}" \
    --cwd "/" \
    --command "/bin/sh -c 'sleep 300'" \
    --ns "mount" --ns "uts" --ns "ipc" \
    --hostname "${cid}" \
    --if_name "" --if_addr "" \
    --image_layer "${layer}" \
    --upper_dir "${upper}" \
    --work_dir "${work}" \
    --output "${container_dir}"

  # Nested Workshop environments can reject installing custom seccomp BPF
  # programs even when the runtime lifecycle itself is usable. Keep this E2E
  # focused on the droplet command lifecycle and cover seccomp behavior in
  # dedicated unit tests.
  sudo_cmd jq '.linux.seccomp.syscalls = []' "${container_dir}/config.json" \
    | sudo_cmd tee "${container_dir}/config.json.tmp" >/dev/null
  sudo_cmd mv "${container_dir}/config.json.tmp" "${container_dir}/config.json"
}

cleanup_runtime_fixture() {
  local cid="$1"
  local container_dir="/etc/raind/container/${cid}"
  local rootfs="${container_dir}/merged"
  local pid=""

  if sudo_cmd test -f "${container_dir}/state.json"; then
    pid="$(sudo_cmd jq -r '.pid // 0' "${container_dir}/state.json" 2>/dev/null || true)"
    if [[ "${pid}" =~ ^[0-9]+$ ]] && [[ "${pid}" -gt 0 ]]; then
      sudo_cmd kill -KILL "${pid}" 2>/dev/null || true
    fi
  fi

  sudo_cmd umount -R "${rootfs}" 2>/dev/null || true
  sudo_cmd rm -rf "${container_dir}"
  sudo_cmd rmdir "/sys/fs/cgroup/raind/${cid}" 2>/dev/null || true
}

assert_runtime_state() {
  local cid="$1"
  local expected="$2"

  local state_out="${E2E_WORK_DIR}/${cid}.state.json"

  if ! sudo_droplet state "${cid}" >"${state_out}" 2>"${state_out}.err"; then
    cat "${state_out}.err" >&2 || true
    dump_runtime_debug "${cid}"
    fail "failed to read runtime state: container=${cid}"
  fi

  if ! jq -e --arg expected "${expected}" '.status == $expected' "${state_out}" >/dev/null; then
    cat "${state_out}" >&2 || true
    dump_runtime_debug "${cid}"
    fail "unexpected runtime state: container=${cid} expected=${expected}"
  fi
}

assert_runtime_state_contract() {
  local cid="$1"

  sudo_droplet state "${cid}" | jq -e '
    .ociVersion and
    .id and
    .status and
    (.pid | type == "number") and
    .bundle and
    .rootfs and
    (.annotations | type == "object")
  ' >/dev/null || {
    dump_runtime_debug "${cid}"
    fail "runtime state contract assertion failed: container=${cid}"
  }
}

assert_runtime_list_contains() {
  local cid="$1"
  local expected="$2"

  sudo_droplet list --format json |
    jq -e --arg cid "${cid}" --arg expected "${expected}" \
      'map(select(.id == $cid and .status == $expected)) | length == 1' >/dev/null || {
        dump_runtime_debug "${cid}"
        fail "runtime list assertion failed: container=${cid} expected=${expected}"
      }
}

run_runtime_lifecycle_e2e() {
  local cid="${RUNTIME_CID}"
  local pid

  if ! runtime_prerequisites_available; then
    if [[ "${REQUIRE_RUNTIME}" == "1" ]]; then
      fail "runtime e2e prerequisites are not available in this workshop"
    fi
    log "skip runtime lifecycle e2e: required privileges/tools are unavailable"
    return
  fi

  trap 'cleanup_runtime_fixture "${RUNTIME_CID}"' EXIT

  prepare_runtime_fixture "${cid}"

  log "droplet create"
  sudo_droplet create "${cid}"
  assert_runtime_state "${cid}" "created"
  assert_runtime_state_contract "${cid}"
  assert_runtime_list_contains "${cid}" "created"
  assert_sudo_fifo_exists "/etc/raind/container/${cid}/exec.fifo"
  assert_sudo_path_absent "/etc/raind/container/${cid}/config_hash.json"
  assert_cgroup_file_exists "${cid}" "memory.max"
  assert_cgroup_file_exists "${cid}" "cpu.max"
  assert_cgroup_file_exists "${cid}" "pids.max"
  assert_cgroup_file_exists "${cid}" "cgroup.procs"
  assert_audit_event "${cid}" "create"

  log "droplet start"
  sudo_droplet start "${cid}"
  assert_runtime_state "${cid}" "running"
  assert_runtime_state_contract "${cid}"
  assert_runtime_list_contains "${cid}" "running"
  pid="$(sudo_droplet state "${cid}" | jq -r '.pid')"
  sudo_cmd test -d "/proc/${pid}"
  assert_sudo_path_absent "/etc/raind/container/${cid}/exec.fifo"
  assert_sudo_path_absent "/etc/raind/container/${cid}/config_hash.json"
  assert_sudo_file_exists "/etc/raind/container/${cid}/logs/init.log"
  assert_audit_event "${cid}" "start"
  assert_command_fails droplet-start-twice sudo_droplet start "${cid}"
  assert_command_fails droplet-delete-running sudo_droplet delete "${cid}"

  log "droplet exec"
  sudo_droplet exec "${cid}" /bin/sh -c "echo exec-ok"
  assert_sudo_file_exists "/etc/raind/container/${cid}/logs/exec.log"
  assert_audit_event "${cid}" "exec"

  log "droplet kill"
  sudo_droplet kill "${cid}" TERM
  assert_runtime_state "${cid}" "stopped"
  assert_runtime_state_contract "${cid}"
  assert_audit_event "${cid}" "kill"
  assert_command_fails droplet-kill-stopped sudo_droplet kill "${cid}" TERM
  assert_command_fails droplet-exec-stopped sudo_droplet exec "${cid}" /bin/sh -c "true"

  log "droplet delete"
  sudo_droplet delete "${cid}"
  assert_sudo_path_absent "/etc/raind/container/${cid}/state.json"
  assert_audit_event "${cid}" "delete"

  cleanup_runtime_fixture "${cid}"
  assert_sudo_path_absent "/etc/raind/container/${cid}"
  trap - EXIT
}

main() {
  require_workshop
  mkdir -p "${E2E_WORK_DIR}"

  build_droplet
  setup_audit_log
  run_smoke_e2e

  case "${RUN_RUNTIME}" in
    0|false|no)
      log "skip runtime lifecycle e2e: RAIND_DROPLET_E2E_RUNTIME=${RUN_RUNTIME}"
      ;;
    1|true|yes|auto)
      run_runtime_lifecycle_e2e
      ;;
    *)
      fail "invalid RAIND_DROPLET_E2E_RUNTIME value: ${RUN_RUNTIME}"
      ;;
  esac

  log "droplet integration test completed"
}

main "$@"
