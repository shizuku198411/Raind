#!/usr/bin/env bash

set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

main() {
  require_workshop
  cd "${ROOT_DIR}"

  ./scripts/build.sh build
  install_binaries_to_usr_local
  prepare_runtime_dirs

  log "installed. try: workshop run raind-dev -- dev-start"
}

main "$@"

