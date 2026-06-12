#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
SYSTEMD_DIR="${SYSTEMD_DIR:-/etc/systemd/system}"
SERVICE_NAME="${SERVICE_NAME:-raind-daemon.service}"
UI_GATEWAY_SERVICE_NAME="${UI_GATEWAY_SERVICE_NAME:-raind-ui-gateway.service}"
VERSION_FILE="${ROOT_DIR}/VERSION"

BINARIES=(
  "bin/raind"
  "bin/condenser"
  "bin/condenser-hook-agent"
  "bin/droplet"
  "bin/raind-ui-gateway"
)

bootstrap() {
  echo "==> download go modules"
  go mod download
}

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

  mkdir -p "${ROOT_DIR}/bin"

  echo "==> build cmd/raind"
  go build \
    -ldflags "-X raind/internal/raind/buildinfo.Version=${build_version} -X raind/internal/raind/buildinfo.Commit=${build_commit} -X raind/internal/raind/buildinfo.BuiltAt=${build_date}" \
    -o "${ROOT_DIR}/bin/raind" \
    "${ROOT_DIR}/cmd/raind"

  echo "==> build cmd/condenser"
  go build \
    -ldflags "-X raind/internal/condenser/buildinfo.Version=${build_version} -X raind/internal/condenser/buildinfo.Commit=${build_commit} -X raind/internal/condenser/buildinfo.BuiltAt=${build_date}" \
    -o "${ROOT_DIR}/bin/condenser" \
    "${ROOT_DIR}/cmd/condenser"

  echo "==> build cmd/condenser-hook"
  go build \
    -ldflags "-X raind/internal/condenser/buildinfo.Version=${build_version} -X raind/internal/condenser/buildinfo.Commit=${build_commit} -X raind/internal/condenser/buildinfo.BuiltAt=${build_date}" \
    -o "${ROOT_DIR}/bin/condenser-hook-agent" \
    "${ROOT_DIR}/cmd/condenser-hook"

  echo "==> build cmd/droplet"
  go build \
    -ldflags "-X raind/internal/droplet/buildinfo.Version=${build_version} -X raind/internal/droplet/buildinfo.Commit=${build_commit} -X raind/internal/droplet/buildinfo.BuiltAt=${build_date}" \
    -o "${ROOT_DIR}/bin/droplet" \
    "${ROOT_DIR}/cmd/droplet"

  echo "==> build cmd/raind-ui-gateway"
  go build \
    -ldflags "-X raind/internal/raind-ui-gateway/buildinfo.Version=${build_version} -X raind/internal/raind-ui-gateway/buildinfo.Commit=${build_commit} -X raind/internal/raind-ui-gateway/buildinfo.BuiltAt=${build_date}" \
    -o "${ROOT_DIR}/bin/raind-ui-gateway" \
    "${ROOT_DIR}/cmd/raind-ui-gateway"
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
Usage: $0 [bootstrap|build|install|enable-service|enable-ui-gateway-service|all]

  bootstrap       Download Go modules
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
    bootstrap)
      bootstrap
      ;;
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
