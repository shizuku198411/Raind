#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
WORKSHOP_RUN_DIR="${WORKSHOP_RUN_DIR:-/run/raind-workshop}"
WORKSHOP_LOG_DIR="${WORKSHOP_LOG_DIR:-/var/log/raind}"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
SYSTEMD_DIR="${SYSTEMD_DIR:-/etc/systemd/system}"
SERVICE_NAME="${SERVICE_NAME:-raind-daemon.service}"
RAIND_GROUP="${RAIND_GROUP:-raind}"
BASH_COMPLETION_DIR="${BASH_COMPLETION_DIR:-/usr/share/bash-completion/completions}"
ZSH_COMPLETION_DIR="${ZSH_COMPLETION_DIR:-/usr/share/zsh/vendor-completions}"
FISH_COMPLETION_DIR="${FISH_COMPLETION_DIR:-/usr/share/fish/vendor_completions.d}"

BINARIES=(
  "raind"
  "condenser"
  "condenser-hook-agent"
  "droplet"
)

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
    fail "sudo is required"
  fi
  sudo -n "$@"
}

require_workshop() {
  if [[ "${ROOT_DIR}" != /project* ]]; then
    fail "this script is Workshop-only. run with: workshop run raind-dev -- <action>"
  fi
}

systemd_available() {
  have_cmd systemctl && sudo_cmd systemctl list-units >/dev/null 2>&1
}

ensure_raind_group() {
  if ! getent group "${RAIND_GROUP}" >/dev/null 2>&1; then
    log "create ${RAIND_GROUP} group"
    sudo_cmd groupadd --system "${RAIND_GROUP}"
  fi

  local current_user
  current_user="$(id -un)"
  if [[ "${current_user}" != "root" ]] && ! id -nG "${current_user}" | tr ' ' '\n' | grep -qx "${RAIND_GROUP}"; then
    log "add ${current_user} to ${RAIND_GROUP} group"
    sudo_cmd usermod -aG "${RAIND_GROUP}" "${current_user}"
  fi
}

prepare_runtime_dirs() {
  log "prepare raind runtime directories"
  ensure_raind_group
  sudo_cmd mkdir -p \
    /etc/raind/log \
    /etc/raind/cert \
    /etc/raind/store \
    /etc/raind/container \
    /etc/raind/image/layers \
    /run/raind \
    "${WORKSHOP_RUN_DIR}" \
    "${WORKSHOP_LOG_DIR}" \
    /sys/fs/cgroup/raind

  sudo_cmd chmod 0755 \
    /etc/raind \
    /etc/raind/log \
    /etc/raind/cert \
    /etc/raind/store \
    /run/raind \
    "${WORKSHOP_RUN_DIR}" \
    "${WORKSHOP_LOG_DIR}" \
    || true
  sudo_cmd chmod 0777 "${WORKSHOP_RUN_DIR}" "${WORKSHOP_LOG_DIR}" || true

  for controller in cpu memory pids io; do
    if sudo_cmd grep -qw "${controller}" /sys/fs/cgroup/raind/cgroup.controllers 2>/dev/null; then
      sudo_cmd sh -c "echo +${controller} > /sys/fs/cgroup/raind/cgroup.subtree_control 2>/dev/null || true"
    fi
  done
}

install_binaries_to_usr_local() {
  log "install raind binaries to ${INSTALL_DIR}"
  sudo_cmd mkdir -p "${INSTALL_DIR}"
  for bin in "${BINARIES[@]}"; do
    local src="${ROOT_DIR}/bin/${bin}"
    local dst="${INSTALL_DIR}/${bin}"
    [[ -x "${src}" ]] || fail "missing built binary: ${src}. run: workshop run raind-dev -- build"
    sudo_cmd install -m 0755 "${src}" "${dst}"
  done

  install_completions_to_system
}

install_completions_to_system() {
  local raind_bin="${ROOT_DIR}/bin/raind"
  [[ -x "${raind_bin}" ]] || fail "missing built binary: ${raind_bin}. run: workshop run raind-dev -- build"

  local tmp_dir
  tmp_dir="$(mktemp -d)"

  "${raind_bin}" completion bash > "${tmp_dir}/raind"
  "${raind_bin}" completion zsh > "${tmp_dir}/_raind"
  "${raind_bin}" completion fish > "${tmp_dir}/raind.fish"

  log "install shell completions"
  sudo_cmd install -d "${BASH_COMPLETION_DIR}" "${ZSH_COMPLETION_DIR}" "${FISH_COMPLETION_DIR}"
  sudo_cmd install -m 0644 "${tmp_dir}/raind" "${BASH_COMPLETION_DIR}/raind"
  sudo_cmd install -m 0644 "${tmp_dir}/_raind" "${ZSH_COMPLETION_DIR}/_raind"
  sudo_cmd install -m 0644 "${tmp_dir}/raind.fish" "${FISH_COMPLETION_DIR}/raind.fish"
  rm -rf "${tmp_dir}"
}

assert_ports_free() {
  for port in 7755 7756 7757 7758; do
    if sudo_cmd ss -ltn "( sport = :${port} )" 2>/dev/null | grep -q ":${port}"; then
      fail "port ${port} is already in use in this Workshop. run cleanup or stop the process first."
    fi
  done
}

write_pid() {
  local name="$1"
  local pid="$2"
  sudo_cmd mkdir -p "${WORKSHOP_RUN_DIR}"
  printf '%s\n' "${pid}" | sudo_cmd tee "${WORKSHOP_RUN_DIR}/${name}.pid" >/dev/null
}

stop_pid_file() {
  local name="$1"
  local pid_file="${WORKSHOP_RUN_DIR}/${name}.pid"
  if ! sudo_cmd test -f "${pid_file}"; then
    return
  fi

  local pid
  pid="$(sudo_cmd cat "${pid_file}" 2>/dev/null || true)"
  if [[ -n "${pid}" ]]; then
    sudo_cmd kill "${pid}" 2>/dev/null || true
    sleep 0.3
    sudo_cmd kill -KILL "${pid}" 2>/dev/null || true
  fi
  sudo_cmd rm -f "${pid_file}"
}

wait_condenser_ready() {
  local out="${WORKSHOP_RUN_DIR}/ready.json"
  local err="${WORKSHOP_RUN_DIR}/ready.err"

  log "wait for condenser API"
  for _ in $(seq 1 400); do
    if sudo_cmd test -f /etc/raind/cert/raindClient.crt &&
      sudo_cmd curl -sS \
        --connect-timeout 1 \
        --max-time 3 \
        --cert /etc/raind/cert/raindClient.crt \
        --key /etc/raind/cert/raindClient.key \
        --cacert /etc/raind/cert/raind.crt \
        https://127.0.0.1:7755/v1/images >"${out}" 2>"${err}"; then
      if have_cmd jq && jq -e '.status == "success"' "${out}" >/dev/null 2>&1; then
        return
      fi
      if grep -q '"status"[[:space:]]*:[[:space:]]*"success"' "${out}"; then
        return
      fi
    fi
    sleep 0.1
  done

  if [[ -f "${err}" ]]; then
    cat "${err}" >&2 || true
  fi
  fail "condenser API did not become ready"
}
