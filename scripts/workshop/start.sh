#!/usr/bin/env bash

set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

start_with_systemd() {
  log "start raind services with systemd"
  sudo_cmd env INSTALL_DIR="${INSTALL_DIR}" SYSTEMD_DIR="${SYSTEMD_DIR}" SERVICE_NAME="${SERVICE_NAME}" ./scripts/build.sh enable-service
  wait_condenser_ready
  sudo_cmd env INSTALL_DIR="${INSTALL_DIR}" SYSTEMD_DIR="${SYSTEMD_DIR}" SERVICE_NAME="${SERVICE_NAME}" UI_GATEWAY_SERVICE_NAME="${UI_GATEWAY_SERVICE_NAME}" ./scripts/build.sh enable-ui-gateway-service
}

start_direct() {
  local condenser_log="${WORKSHOP_LOG_DIR}/workshop-condenser.log"
  local ui_log="${WORKSHOP_LOG_DIR}/workshop-ui-gateway.log"

  log "start condenser directly"
  sudo_cmd mkdir -p "${WORKSHOP_RUN_DIR}" "${WORKSHOP_LOG_DIR}"
  sudo_cmd rm -f "${condenser_log}" "${ui_log}"
  sudo_cmd touch "${condenser_log}" "${ui_log}"
  sudo_cmd chmod 0666 "${condenser_log}" "${ui_log}"

  sudo_cmd env PATH="${INSTALL_DIR}:${PATH}" "${INSTALL_DIR}/condenser" >"${condenser_log}" 2>&1 &
  write_pid condenser "$!"

  wait_condenser_ready

  log "start raind-ui-gateway directly"
  sudo_cmd env PATH="${INSTALL_DIR}:${PATH}" "${INSTALL_DIR}/raind-ui-gateway" >"${ui_log}" 2>&1 &
  write_pid raind-ui-gateway "$!"
}

main() {
  require_workshop
  cd "${ROOT_DIR}"

  if [[ ! -x "${INSTALL_DIR}/raind" || ! -x "${INSTALL_DIR}/condenser" || ! -x "${INSTALL_DIR}/droplet" ]]; then
    ./scripts/workshop/install.sh
  fi

  prepare_runtime_dirs
  assert_ports_free

  if systemd_available; then
    start_with_systemd
  else
    start_direct
  fi

  log "ready. examples:"
  log "  workshop shell raind-dev"
  log "  sudo raind image ls"
  log "  sudo raind container run -p 9988:80 nginx:latest"
  log "cleanup: workshop run raind-dev -- dev-cleanup"
}

main "$@"
