#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
SYSTEMD_DIR="${SYSTEMD_DIR:-/etc/systemd/system}"
SERVICE_NAME="${SERVICE_NAME:-raind-daemon.service}"

COMPONENTS=(
  "runtime_stack/raind-cli"
  "runtime_stack/condenser"
  "runtime_stack/droplet"
)

BINARIES=(
  "runtime_stack/raind-cli/bin/raind"
  "runtime_stack/condenser/bin/condenser"
  "runtime_stack/condenser/bin/condenser-hook-agent"
  "runtime_stack/droplet/bin/droplet"
)

need_root() {
  if [[ "${EUID}" -ne 0 ]]; then
    echo "error: root privileges are required. run with sudo." >&2
    exit 1
  fi
}

build_components() {
  for component in "${COMPONENTS[@]}"; do
    echo "==> build ${component}"
    (
      cd "${ROOT_DIR}/${component}"
      bash ./scripts/build.sh
    )
  done
}

install_binaries() {
  need_root

  for bin_path in "${BINARIES[@]}"; do
    local src="${ROOT_DIR}/${bin_path}"
    local dst="${INSTALL_DIR}/$(basename "${bin_path}")"

    if [[ ! -f "${src}" ]]; then
      echo "error: missing binary: ${src}" >&2
      echo "hint: run './scripts/build.sh build' first." >&2
      exit 1
    fi

    echo "==> install ${src} -> ${dst}"
    install -m 0755 "${src}" "${dst}"
  done
}

write_service_file() {
  need_root

  local service_path="${SYSTEMD_DIR}/${SERVICE_NAME}"

  echo "==> write ${service_path}"
  cat > "${service_path}" <<SERVICE_EOF
[Unit]
Description=Raind Condenser Daemon
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=${INSTALL_DIR}/condenser
Restart=always
RestartSec=3
User=root
Group=root

[Install]
WantedBy=multi-user.target
SERVICE_EOF
}

enable_service() {
  need_root
  write_service_file

  echo "==> systemctl daemon-reload"
  systemctl daemon-reload

  echo "==> systemctl enable --now ${SERVICE_NAME}"
  systemctl enable --now "${SERVICE_NAME}"

  echo "==> systemctl status ${SERVICE_NAME} --no-pager"
  systemctl status "${SERVICE_NAME}" --no-pager
}

usage() {
  cat <<USAGE
Usage: $0 [build|install|enable-service|all]

  build           Build all components
  install         Install built binaries to ${INSTALL_DIR}
  enable-service  Create and start ${SERVICE_NAME}
  all             Build, install, and enable service
USAGE
}

main() {
  local target="${1:-all}"

  case "${target}" in
    build)
      build_components
      ;;
    install)
      install_binaries
      ;;
    enable-service)
      enable_service
      ;;
    all)
      build_components
      install_binaries
      enable_service
      ;;
    -h|--help|help)
      usage
      ;;
    *)
      echo "error: unknown target: ${target}" >&2
      usage
      exit 1
      ;;
  esac
}

main "$@"
