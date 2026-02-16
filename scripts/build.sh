#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
SYSTEMD_DIR="${SYSTEMD_DIR:-/etc/systemd/system}"
SERVICE_NAME="${SERVICE_NAME:-raind-daemon.service}"
UI_GATEWAY_SERVICE_NAME="${UI_GATEWAY_SERVICE_NAME:-raind-ui-gateway.service}"
VERSION_FILE="${ROOT_DIR}/VERSION"

COMPONENTS=(
  "runtime_stack/raind-cli"
  "runtime_stack/condenser"
  "runtime_stack/droplet"
  "runtime_stack/raind-ui-gateway"
)

BINARIES=(
  "runtime_stack/raind-cli/bin/raind"
  "runtime_stack/condenser/bin/condenser"
  "runtime_stack/condenser/bin/condenser-hook-agent"
  "runtime_stack/droplet/bin/droplet"
  "runtime_stack/raind-ui-gateway/bin/raind-ui-gateway"
)

need_root() {
  if [[ "${EUID}" -ne 0 ]]; then
    echo "error: root privileges are required. run with sudo." >&2
    exit 1
  fi
}

build_components() {
  local build_version="dev"
  local build_commit="unknown"
  local build_date
  build_date="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"

  if [[ -f "${VERSION_FILE}" ]]; then
    build_version="$(tr -d '[:space:]' < "${VERSION_FILE}")"
  fi

  if git -C "${ROOT_DIR}" rev-parse --short HEAD >/dev/null 2>&1; then
    build_commit="$(git -C "${ROOT_DIR}" rev-parse --short HEAD)"
  fi

  echo "==> build metadata: version=${build_version} commit=${build_commit} built_at=${build_date}"

  for component in "${COMPONENTS[@]}"; do
    echo "==> build ${component}"
    (
      cd "${ROOT_DIR}/${component}"
      BUILD_VERSION="${build_version}" \
      BUILD_COMMIT="${build_commit}" \
      BUILD_DATE="${build_date}" \
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

write_ui_gateway_service_file() {
  need_root

  local service_path="${SYSTEMD_DIR}/${UI_GATEWAY_SERVICE_NAME}"

  echo "==> write ${service_path}"
  cat > "${service_path}" <<SERVICE_EOF
[Unit]
Description=Raind UI UDS Gateway
After=network-online.target ${SERVICE_NAME}
Wants=network-online.target

[Service]
Type=simple
ExecStart=${INSTALL_DIR}/raind-ui-gateway
Restart=always
RestartSec=3
User=root
Group=root

[Install]
WantedBy=multi-user.target
SERVICE_EOF
}

enable_ui_gateway_service() {
  need_root
  write_ui_gateway_service_file

  echo "==> systemctl daemon-reload"
  systemctl daemon-reload

  echo "==> systemctl enable --now ${UI_GATEWAY_SERVICE_NAME}"
  systemctl enable --now "${UI_GATEWAY_SERVICE_NAME}"

  echo "==> systemctl status ${UI_GATEWAY_SERVICE_NAME} --no-pager"
  systemctl status "${UI_GATEWAY_SERVICE_NAME}" --no-pager
}

usage() {
  cat <<USAGE
Usage: $0 [build|install|enable-service|enable-ui-gateway-service|all]

  build           Build all components
  install         Install built binaries to ${INSTALL_DIR}
  enable-service  Create and start ${SERVICE_NAME}
  enable-ui-gateway-service Create and start ${UI_GATEWAY_SERVICE_NAME}
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
    enable-ui-gateway-service)
      enable_ui_gateway_service
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
