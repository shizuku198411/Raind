#!/usr/bin/env bash

set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

stop_systemd_services() {
  if ! have_cmd systemctl; then
    return
  fi

  log "stop systemd services if present"
  sudo_cmd systemctl stop "${SERVICE_NAME}" 2>/dev/null || true
  sudo_cmd systemctl disable "${SERVICE_NAME}" 2>/dev/null || true
  sudo_cmd rm -f "${SYSTEMD_DIR}/${SERVICE_NAME}"
  sudo_cmd systemctl daemon-reload 2>/dev/null || true
}

stop_direct_services() {
  log "stop direct workshop services"
  stop_pid_file condenser

  sudo_cmd pkill -TERM -f "^${INSTALL_DIR}/condenser$" 2>/dev/null || true
  sleep 0.3
  sudo_cmd pkill -KILL -f "^${INSTALL_DIR}/condenser$" 2>/dev/null || true
}

cleanup_network() {
  log "cleanup raind network artifacts"
  sudo_cmd ip link del raind0 2>/dev/null || true
  sudo_cmd ip link del raindDns 2>/dev/null || true
  while read -r ifname; do
    [[ -n "${ifname}" ]] || continue
    sudo_cmd ip link del "${ifname}" 2>/dev/null || true
  done < <(sudo_cmd ip -o link show 2>/dev/null | awk -F': ' '/: (rd_|rns)/ {print $2}' | cut -d@ -f1)

  for table in filter nat; do
    while read -r rule; do
      [[ -n "${rule}" ]] || continue
      sudo_cmd iptables -t "${table}" ${rule/-A/-D} 2>/dev/null || true
    done < <(sudo_cmd iptables -t "${table}" -S 2>/dev/null | grep 'RAIND-' || true)

    while read -r chain; do
      [[ -n "${chain}" ]] || continue
      sudo_cmd iptables -t "${table}" -F "${chain}" 2>/dev/null || true
      sudo_cmd iptables -t "${table}" -X "${chain}" 2>/dev/null || true
    done < <(sudo_cmd iptables -t "${table}" -S 2>/dev/null | awk '/^-N RAIND-/ {print $2}')
  done
}

cleanup_files() {
  log "cleanup raind files"
  sudo_cmd rm -rf \
    /etc/raind \
    /run/raind \
    "${WORKSHOP_RUN_DIR}" \
    "${WORKSHOP_LOG_DIR}/workshop-condenser.log"

  for bin in "${BINARIES[@]}"; do
    sudo_cmd rm -f "${INSTALL_DIR}/${bin}"
  done

  if sudo_cmd test -d /sys/fs/cgroup/raind; then
    sudo_cmd find /sys/fs/cgroup/raind -mindepth 1 -maxdepth 1 -type d -exec rmdir {} + 2>/dev/null || true
    sudo_cmd rmdir /sys/fs/cgroup/raind 2>/dev/null || true
  fi
}

main() {
  require_workshop
  stop_systemd_services
  stop_direct_services
  cleanup_network
  cleanup_files
  log "cleanup complete"
}

main "$@"
