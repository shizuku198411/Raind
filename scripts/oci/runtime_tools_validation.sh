#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
RUNTIME_TOOLS_DIR="${RUNTIME_TOOLS_DIR:-${ROOT_DIR}/runtime-tools}"
RUNTIME="${RUNTIME:-droplet}"
RUNTIME_PATH="${RUNTIME_PATH:-/usr/local/bin:${ROOT_DIR}/bin:${PATH}}"

log() {
  printf '==> %s\n' "$*"
}

fail() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

usage() {
  cat <<'USAGE'
Usage:
  scripts/oci/runtime_tools_validation.sh [TEST...]

Examples:
  scripts/oci/runtime_tools_validation.sh
  scripts/oci/runtime_tools_validation.sh state create
  scripts/oci/runtime_tools_validation.sh ./validation/state/state.t

If no TEST is supplied, runs all runtime-tools validation tests.
USAGE
}

normalize_test() {
  case "$1" in
    -h|--help)
      usage
      exit 0
      ;;
    ./*.t|/*)
      printf '%s\n' "$1"
      ;;
    validation/*.t)
      printf './%s\n' "$1"
      ;;
    */*)
      printf './validation/%s.t\n' "$1"
      ;;
    *)
      printf './validation/%s/%s.t\n' "$1" "$1"
      ;;
  esac
}

list_all_validation_tests() {
  make -s print-validation-tests | tr ' ' '\n' | sed '/^$/d' | sort
}

host_uses_cgroup2() {
  [[ "$(findmnt -T /sys/fs/cgroup -no FSTYPE 2>/dev/null || true)" == "cgroup2" ]]
}

requires_cgroup_v1() {
  case "$1" in
    ./validation/delete_only_create_resources/delete_only_create_resources.t | \
    ./validation/delete_resources/delete_resources.t | \
    ./validation/linux_cgroups_*/linux_cgroups_*.t)
      return 0
      ;;
    *)
      return 1
      ;;
  esac
}

prepare_arm64_rootfs() {
  local arch
  arch="$(dpkg --print-architecture 2>/dev/null || true)"
  if [[ "${arch}" != "arm64" ]]; then
    return 0
  fi
  if [[ -f "${RUNTIME_TOOLS_DIR}/rootfs-arm64.tar.gz" ]]; then
    return 0
  fi
  command -v busybox >/dev/null 2>&1 || fail "busybox is required to build runtime-tools rootfs-arm64.tar.gz"

  log "prepare runtime-tools rootfs-arm64.tar.gz"
  local tmp
  tmp="$(mktemp -d)"
  mkdir -p "${tmp}/bin" "${tmp}/dev" "${tmp}/etc" "${tmp}/proc" "${tmp}/sys" "${tmp}/tmp" "${tmp}/run"
  cp "$(command -v busybox)" "${tmp}/bin/busybox"
  chmod 0755 "${tmp}/bin/busybox"
  for applet in sh true false echo cat test sleep hostname id env grep mount umount mkdir touch ls pwd stat readlink chmod chown; do
    ln -sf busybox "${tmp}/bin/${applet}"
  done
  printf 'root:x:0:0:root:/root:/bin/sh\n' > "${tmp}/etc/passwd"
  printf 'root:x:0:\n' > "${tmp}/etc/group"
  tar -C "${tmp}" -czf "${RUNTIME_TOOLS_DIR}/rootfs-arm64.tar.gz" .
  rm -rf "${tmp}"
}

cleanup_droplet_runtime_state() {
  sudo rm -rf /etc/raind/container
  sudo mkdir -p /etc/raind/container
  if sudo test -d /sys/fs/cgroup/raind; then
    sudo find /sys/fs/cgroup/raind -mindepth 1 -maxdepth 1 -type d -exec rmdir {} + 2>/dev/null || true
  fi
}

main() {
  if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
    usage
    exit 0
  fi

  [[ -d "${RUNTIME_TOOLS_DIR}" ]] || fail "runtime-tools directory not found: ${RUNTIME_TOOLS_DIR}"
  command -v go >/dev/null 2>&1 || fail "go is required"
  command -v sudo >/dev/null 2>&1 || fail "sudo is required"

  cd "${RUNTIME_TOOLS_DIR}"
  prepare_arm64_rootfs

  local tests=()
  if [[ "$#" -eq 0 ]]; then
    mapfile -t tests < <(list_all_validation_tests)
  else
    tests=("$@")
  fi

  local test_paths=()
  local test_path
  for test_name in "${tests[@]}"; do
    test_path="$(normalize_test "${test_name}")"
    [[ -f "${test_path%.t}.go" || -x "${test_path}" ]] || fail "unknown runtime-tools validation test: ${test_name} (${test_path})"
    test_paths+=("${test_path}")
  done

  log "selected runtime-tools validation tests: ${#test_paths[@]}"
  log "build runtime-tools validation executables"
  make tool runtimetest "${test_paths[@]}"

  local failed=0
  local test_failed=0
  local passed_count=0
  local skipped_count=0
  local failed_tests=()
  local skipped_tests=()
  local output
  local cgroup2=false
  if host_uses_cgroup2; then
    cgroup2=true
  fi
  for test_path in "${test_paths[@]}"; do
    cleanup_droplet_runtime_state
    log "runtime-tools validation: ${test_path}"
    if [[ "${cgroup2}" == "true" ]] && requires_cgroup_v1 "${test_path}"; then
      printf 'TAP version 13\n'
      printf 'ok 1 # SKIP %s requires cgroup v1; host uses cgroup2\n' "${test_path}"
      printf '1..1\n'
      skipped_count=$((skipped_count + 1))
      skipped_tests+=("${test_path}")
      continue
    fi
    test_failed=0
    set +e
    output="$(sudo env PATH="${RUNTIME_PATH}" RUNTIME="${RUNTIME}" "${test_path}" 2>&1)"
    status=$?
    set -e
    printf '%s\n' "${output}"
    if [[ "${status}" -ne 0 ]]; then
      printf 'error: %s exited with status %d\n' "${test_path}" "${status}" >&2
      failed=1
      test_failed=1
    fi
    if ! grep -q '^1\.\.' <<<"${output}"; then
      printf 'error: %s did not emit a TAP plan\n' "${test_path}" >&2
      failed=1
      test_failed=1
    fi
    if grep -Eq 'not ok[[:space:]][0-9]+' <<<"${output}"; then
      printf 'error: %s reported failing TAP assertions\n' "${test_path}" >&2
      failed=1
      test_failed=1
    fi
    if [[ "${test_failed}" -ne 0 ]]; then
      failed_tests+=("${test_path}")
    else
      passed_count=$((passed_count + 1))
    fi
  done

  log "runtime-tools validation summary: ${passed_count}/${#test_paths[@]} passed, ${skipped_count} skipped"
  if [[ "${#skipped_tests[@]}" -ne 0 ]]; then
    printf 'skipped runtime-tools validation tests:\n' >&2
    printf '  %s\n' "${skipped_tests[@]}" >&2
  fi
  if [[ "${#failed_tests[@]}" -ne 0 ]]; then
    printf 'failed runtime-tools validation tests:\n' >&2
    printf '  %s\n' "${failed_tests[@]}" >&2
  fi

  if [[ "${failed}" -ne 0 ]]; then
    fail "runtime-tools validation failed"
  fi
}

main "$@"
