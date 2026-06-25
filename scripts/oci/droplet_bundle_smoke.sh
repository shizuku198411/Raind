#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DROPLET_BIN="${DROPLET_BIN:-${ROOT_DIR}/bin/droplet}"
E2E_WORK_DIR="${E2E_WORK_DIR:-/tmp/raind-droplet-oci-smoke}"
CID="${RAIND_DROPLET_OCI_CID:-raind-oci-smoke-$$}"

log() {
  printf '==> %s\n' "$*"
}

fail() {
  printf 'error: %s\n' "$*" >&2
  dump_debug || true
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
    fail "sudo is required for droplet OCI bundle smoke test"
  fi
  sudo -n "$@"
}

sudo_droplet() {
  sudo_cmd env PATH="${PATH}" "${DROPLET_BIN}" "$@"
}

bundle_dir() {
  printf '%s\n' "${E2E_WORK_DIR}/oci-bundle/${CID}"
}

rootfs_dir() {
  printf '%s\n' "$(bundle_dir)/rootfs"
}

pid_file() {
  printf '%s\n' "${E2E_WORK_DIR}/oci-bundle/${CID}.pid"
}

container_dir() {
  printf '%s\n' "/etc/raind/container/${CID}"
}

dump_debug() {
  local container_dir
  container_dir="$(container_dir)"

  printf '%s\n' "----- droplet state: ${CID} -----" >&2
  sudo_droplet state "${CID}" >&2 2>/dev/null || true

  printf '%s\n' "----- runtime config -----" >&2
  sudo_cmd cat "${container_dir}/config.json" >&2 2>/dev/null || true

  printf '%s\n' "----- runtime state -----" >&2
  sudo_cmd cat "${container_dir}/state.json" >&2 2>/dev/null || true

  printf '%s\n' "----- container log -----" >&2
  sudo_cmd cat "${container_dir}/logs/container.log" >&2 2>/dev/null || true

  printf '%s\n' "----- shim log -----" >&2
  sudo_cmd cat "${container_dir}/logs/shim.log" >&2 2>/dev/null || true
}

cleanup() {
  local bundle
  local rootfs
  local container_dir
  bundle="$(bundle_dir)"
  rootfs="$(rootfs_dir)"
  container_dir="$(container_dir)"

  if sudo_cmd test -f "${container_dir}/state.json" 2>/dev/null; then
    sudo_droplet kill "${CID}" KILL >/dev/null 2>&1 || true
    sudo_droplet delete "${CID}" >/dev/null 2>&1 || true
  fi
  sudo_cmd umount -R "${rootfs}" >/dev/null 2>&1 || true
  sudo_cmd rm -rf "${container_dir}" "${bundle}" "$(pid_file)"
  sudo_cmd rmdir "/sys/fs/cgroup/raind/${CID}" >/dev/null 2>&1 || true
}

require_prerequisites() {
  [[ -x "${DROPLET_BIN}" ]] || fail "missing droplet binary: ${DROPLET_BIN}"
  have_cmd busybox || fail "busybox is required for droplet OCI bundle smoke test"
  have_cmd jq || fail "jq is required for droplet OCI bundle smoke test"
  sudo_cmd test -w /etc || fail "/etc must be writable for droplet OCI bundle smoke test"
  sudo_cmd test -w /sys/fs/cgroup || fail "/sys/fs/cgroup must be writable for droplet OCI bundle smoke test"
}

prepare_cgroup_parent() {
  sudo_cmd mkdir -p /sys/fs/cgroup/raind
  for controller in cpu memory pids; do
    sudo_cmd sh -c "echo +${controller} > /sys/fs/cgroup/raind/cgroup.subtree_control 2>/dev/null || true"
  done
}

prepare_bundle() {
  local bundle
  local rootfs
  bundle="$(bundle_dir)"
  rootfs="$(rootfs_dir)"

  cleanup
  sudo_cmd mkdir -p \
    "${rootfs}/bin" \
    "${rootfs}/dev" \
    "${rootfs}/etc" \
    "${rootfs}/proc" \
    "${rootfs}/run" \
    "${rootfs}/sys" \
    "${rootfs}/tmp"
  sudo_cmd cp "$(command -v busybox)" "${rootfs}/bin/busybox"
  for applet in sh sleep cat test; do
    sudo_cmd ln -sf busybox "${rootfs}/bin/${applet}"
  done
  sudo_cmd chmod 0755 "${rootfs}/bin/busybox"

  sudo_cmd tee "${bundle}/config.json" >/dev/null <<'JSON'
{
  "ociVersion": "1.3.0",
  "root": {
    "path": "rootfs",
    "readonly": true
  },
  "mounts": [],
  "process": {
    "terminal": false,
    "consoleSize": {
      "height": 24,
      "width": 80
    },
    "cwd": "/",
    "env": [
      "PATH=/bin",
      "TERM=xterm-256color"
    ],
    "args": [
      "/bin/sh",
      "-c",
      "echo oci-start-ok; echo oci-start-ok > /tmp/oci-start-ok; trap 'exit 0' TERM INT; while true; do sleep 1; done"
    ],
    "capabilities": {
      "bounding": [],
      "permitted": [],
      "inheritable": [],
      "effective": [],
      "ambient": []
    },
    "rlimits": [
      {
        "type": "RLIMIT_NOFILE",
        "hard": 1024,
        "soft": 1024
      }
    ],
    "noNewPrivileges": true
  },
  "hostname": "droplet-oci-smoke",
  "linux": {
    "resources": {
      "pids": {
        "limit": 64
      }
    },
    "namespaces": [
      {
        "type": "mount"
      },
      {
        "type": "uts"
      },
      {
        "type": "ipc"
      }
    ],
    "maskedPaths": [
      "/proc/kcore"
    ],
    "readonlyPaths": [
      "/proc/sys"
    ]
  },
  "annotations": {
    "org.opencontainers.image.ref.name": "droplet-oci-smoke"
  }
}
JSON
}

wait_state() {
  local expected="$1"
  local attempts="${2:-80}"

  for _ in $(seq 1 "${attempts}"); do
    if sudo_droplet state "${CID}" 2>/dev/null | jq -e --arg expected "${expected}" '.status == $expected' >/dev/null; then
      return 0
    fi
    sleep 0.1
  done
  return 1
}

wait_log_contains() {
  local expected="$1"
  local log_path="/etc/raind/container/${CID}/logs/container.log"
  local attempts="${2:-80}"

  for _ in $(seq 1 "${attempts}"); do
    if sudo_cmd test -f "${log_path}" && sudo_cmd grep -q "${expected}" "${log_path}"; then
      return 0
    fi
    sleep 0.1
  done
  return 1
}

run_oci_bundle_smoke() {
  local bundle
  local pid_file
  bundle="$(bundle_dir)"
  pid_file="$(pid_file)"

  log "droplet OCI bundle smoke: prepare"
  require_prerequisites
  prepare_cgroup_parent
  prepare_bundle
  trap cleanup EXIT

  log "droplet OCI bundle smoke: create --bundle"
  sudo_droplet create --bundle "${bundle}" --pid-file "${pid_file}" "${CID}"
  wait_state created || fail "container did not reach created state"
  sudo_cmd test -s "${pid_file}" || fail "pid-file was not written: ${pid_file}"
  sudo_droplet state "${CID}" | jq -e --arg bundle "${bundle}" --arg cid "${CID}" '
    .ociVersion and
    .id == $cid and
    .status == "created" and
    .bundle == $bundle and
    (.annotations["org.opencontainers.image.ref.name"] == "droplet-oci-smoke")
  ' >/dev/null || fail "created state is not OCI-compatible"
  sudo_cmd jq -e --arg rootfs "$(rootfs_dir)" '
    .root.path == $rootfs and
    .root.readonly == true and
    .process.consoleSize.height == 24 and
    .process.noNewPrivileges == true and
    .linux.resources.pids.limit == 64 and
    .annotations["org.opencontainers.image.ref.name"] == "droplet-oci-smoke"
  ' "/etc/raind/container/${CID}/config.json" >/dev/null || fail "runtime config was not normalized from OCI bundle"
  sudo_droplet list --format json | jq -e --arg cid "${CID}" '
    map(select(.id == $cid and .status == "created")) | length == 1
  ' >/dev/null || fail "created container missing from droplet list"

  log "droplet OCI bundle smoke: start"
  sudo_droplet start "${CID}"
  wait_state running || fail "container did not reach running state"
  wait_log_contains "oci-start-ok" || fail "start command output was not captured"

  log "droplet OCI bundle smoke: exec"
  sudo_droplet exec "${CID}" /bin/sh -c "cat /tmp/oci-start-ok; echo oci-exec-ok"
  wait_log_contains "oci-exec-ok" || fail "exec command output was not captured"

  log "droplet OCI bundle smoke: kill/delete"
  sudo_droplet kill "${CID}" TERM
  wait_state stopped || fail "container did not reach stopped state"
  sudo_droplet delete "${CID}"
  sudo_cmd test ! -e "/etc/raind/container/${CID}/state.json" || fail "state still exists after delete"

  cleanup
  trap - EXIT
}

run_oci_bundle_smoke "$@"
