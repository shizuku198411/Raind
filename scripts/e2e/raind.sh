#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
RAIND_BIN="${ROOT_DIR}/bin/raind"
CONDENSER_BIN="${ROOT_DIR}/bin/condenser"
E2E_WORK_DIR="${E2E_WORK_DIR:-/tmp/raind-deploy-e2e}"
LOG_PATH="${E2E_WORK_DIR}/condenser.log"
PID=""
SUFFIX="${E2E_SUFFIX:-$$}"
HOST_ADDR=""

log() {
  printf '==> %s\n' "$*"
}

fail() {
  printf 'error: %s\n' "$*" >&2
  if [[ -f "${LOG_PATH}" ]]; then
    printf '%s\n' '--- condenser log ---' >&2
    tail -220 "${LOG_PATH}" >&2 || true
  fi
  if sudo_cmd test -f /etc/raind/log/droplet_audit.log 2>/dev/null; then
    printf '%s\n' '--- droplet audit log ---' >&2
    sudo_cmd tail -120 /etc/raind/log/droplet_audit.log >&2 || true
  fi
  if sudo_cmd test -d /etc/raind/container 2>/dev/null; then
    printf '%s\n' '--- recent container init logs ---' >&2
    sudo_cmd find /etc/raind/container -maxdepth 3 -path '*/logs/init.log' -type f -printf '%T@ %p\n' 2>/dev/null       | sort -nr       | head -8       | awk '{ $1=""; sub(/^ /, ""); print }'       | while IFS= read -r init_log; do
          printf '%s\n' "----- ${init_log} -----" >&2
          sudo_cmd tail -40 "${init_log}" >&2 || true
        done
  fi
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
    fail "sudo is required for raind e2e"
  fi
  sudo -n "$@"
}

require_workshop() {
  if [[ "${ROOT_DIR}" != /project* ]]; then
    fail "raind deploy e2e must run inside Workshop. use: workshop run raind-dev -- test-e2e"
  fi
}

build_components() {
  log "build all raind components"
  cd "${ROOT_DIR}"
  ./scripts/build.sh build
  [[ -x "${RAIND_BIN}" ]] || fail "missing built raind binary: ${RAIND_BIN}"
  [[ -x "${CONDENSER_BIN}" ]] || fail "missing built condenser binary: ${CONDENSER_BIN}"
}

prepare_runtime() {
  log "prepare runtime prerequisites"
  sudo_cmd mkdir -p \
    /etc/raind/log \
    /etc/raind/cert \
    /etc/raind/store \
    /etc/raind/container \
    /etc/raind/image/layers \
    /var/log/raind \
    /sys/fs/cgroup/raind

  reset_e2e_runtime_state

  sudo_cmd chmod 0755 /etc/raind /etc/raind/log /etc/raind/cert /etc/raind/store /var/log/raind
  sudo_cmd install -m 0755 "${ROOT_DIR}/bin/condenser-hook-agent" /usr/local/bin/condenser-hook-agent
  sudo_cmd install -m 0755 "${ROOT_DIR}/bin/droplet" /usr/local/bin/droplet

  for controller in cpu memory pids io; do
    if sudo_cmd grep -qw "${controller}" /sys/fs/cgroup/raind/cgroup.controllers; then
      sudo_cmd sh -c "echo +${controller} > /sys/fs/cgroup/raind/cgroup.subtree_control 2>/dev/null || true"
    fi
  done

  HOST_ADDR="$(ip -4 -o addr show dev eth0 | awk '{split($4, a, "/"); print a[1]; exit}')"
  [[ -n "${HOST_ADDR}" ]] || fail "could not resolve eth0 address"
}


reset_e2e_runtime_state() {
  log "reset e2e runtime state"

  # The e2e suite runs against the fixed /etc/raind runtime paths. Previous
  # failed runs can leave Pod/ReplicaSet/Deployment store entries behind. When
  # condenser starts, the pod controller may try to reconcile those stale
  # objects and race with the current run, producing confusing 500s such as
  # "podTemplateId=... not found" during a new resource apply.
  sudo_cmd rm -rf /etc/raind/store/*
  sudo_cmd rm -rf /etc/raind/container/*
  sudo_cmd rm -f /etc/raind/log/droplet_audit.log

  # Rootless networking creates short-lived named netns bind mounts. Clean up
  # stale entries from interrupted runs; ignore entries owned by other tools.
  if sudo_cmd test -d /run/netns; then
    sudo_cmd sh -c 'for ns in /run/netns/rn_*; do [ -e "$ns" ] || continue; umount "$ns" 2>/dev/null || true; rm -f "$ns"; done'
  fi

  # Remove stale raind cgroup leaves from previous runs, but keep the root
  # /sys/fs/cgroup/raind directory that prepare_runtime manages.
  if sudo_cmd test -d /sys/fs/cgroup/raind; then
    sudo_cmd find /sys/fs/cgroup/raind -mindepth 1 -depth -type d -exec rmdir {} + 2>/dev/null || true
  fi
}

cleanup_stale_condenser() {
  sudo_cmd pkill -TERM -f "^${CONDENSER_BIN}$" 2>/dev/null || true
  sleep 0.2
  sudo_cmd pkill -KILL -f "^${CONDENSER_BIN}$" 2>/dev/null || true
}

assert_port_free() {
  local port="$1"
  if sudo_cmd ss -ltn "( sport = :${port} )" | grep -q ":${port}"; then
    fail "port ${port} is already in use in this Workshop"
  fi
}

start_condenser() {
  log "start condenser"
  mkdir -p "${E2E_WORK_DIR}"
  : >"${LOG_PATH}"

  cleanup_stale_condenser
  for port in 7755 7756 7757 7758; do
    assert_port_free "${port}"
  done

  sudo_cmd env PATH="${ROOT_DIR}/bin:${PATH}" "${CONDENSER_BIN}" >"${LOG_PATH}" 2>&1 &
  PID="$!"
}

stop_condenser() {
  if [[ -n "${PID}" ]]; then
    sudo_cmd kill "${PID}" 2>/dev/null || true
    wait "${PID}" 2>/dev/null || true
    PID=""
  fi
  cleanup_stale_condenser
}

api_curl() {
  sudo_cmd curl -sS \
    --connect-timeout 1 \
    --max-time 3 \
    --cert /etc/raind/cert/raindClient.crt \
    --key /etc/raind/cert/raindClient.key \
    --cacert /etc/raind/cert/raind.crt \
    "https://127.0.0.1:7755$1"
}

wait_ready() {
  log "wait for condenser management API"
  local out="${E2E_WORK_DIR}/ready.json"

  for _ in $(seq 1 400); do
    if ! kill -0 "${PID}" 2>/dev/null; then
      fail "condenser exited before becoming ready"
    fi
    if sudo_cmd test -f /etc/raind/cert/raindClient.crt && api_curl "/v1/images" >"${out}" 2>"${E2E_WORK_DIR}/ready.err"; then
      jq -e '.status == "success"' "${out}" >/dev/null && return
    fi
    sleep 0.1
  done

  cat "${E2E_WORK_DIR}/ready.err" >&2 2>/dev/null || true
  fail "condenser management API did not become ready"
}

run_raind() {
  local name="$1"
  shift
  local out="${E2E_WORK_DIR}/${name}.out"

  log "raind $*"
  if ! sudo_cmd env PATH="${PATH}" "${RAIND_BIN}" "$@" >"${out}" 2>&1; then
    printf '%s\n' "--- ${out} ---" >&2
    cat "${out}" >&2 || true
    fail "raind $* failed"
  fi
  [[ -s "${out}" ]] || fail "raind $* produced no output"
}

run_raind_allow_empty() {
  local name="$1"
  shift
  local out="${E2E_WORK_DIR}/${name}.out"

  log "raind $*"
  if ! sudo_cmd env PATH="${PATH}" "${RAIND_BIN}" "$@" >"${out}" 2>&1; then
    printf '%s\n' "--- ${out} ---" >&2
    cat "${out}" >&2 || true
    fail "raind $* failed"
  fi
}



is_transient_resource_apply_failure() {
  local out="$1"

  grep -Eqi \
    'pod start failed|replicaset start failed|deployment start failed|hook createContainer\[[0-9]+\] failed|service hook failed: update pod namespaces failed|podId=.*not found|podTemplateId=.*not found' \
    "${out}"
}

run_resource_apply_with_retry() {
  local name="$1"
  local yaml="$2"
  local out="${E2E_WORK_DIR}/${name}.out"
  local cleanup_out
  local attempt

  for attempt in 1 2 3; do
    if [[ "${attempt}" -eq 1 ]]; then
      log "raind resource apply -f ${yaml}"
    else
      log "raind resource apply -f ${yaml} (retry ${attempt})"
    fi

    if sudo_cmd env PATH="${PATH}" "${RAIND_BIN}" resource apply -f "${yaml}" >"${out}" 2>&1; then
      [[ -s "${out}" ]] || fail "raind resource apply -f ${yaml} produced no output"
      return
    fi

    if ! is_transient_resource_apply_failure "${out}"; then
      printf '%s\n' "--- ${out} ---" >&2
      cat "${out}" >&2 || true
      fail "raind resource apply -f ${yaml} failed"
    fi

    if [[ "${attempt}" -eq 3 ]]; then
      printf '%s\n' "--- ${out} ---" >&2
      cat "${out}" >&2 || true
      fail "raind resource apply -f ${yaml} failed after retries"
    fi

    log "transient resource apply failure detected; cleanup partial resources and retry"
    cleanup_out="${E2E_WORK_DIR}/${name}-retry-cleanup-${attempt}.out"
    sudo_cmd env PATH="${PATH}" "${RAIND_BIN}" resource rm -f "${yaml}" >"${cleanup_out}" 2>&1 || true
    sleep 1
  done
}

assert_output_contains() {
  local name="$1"
  local pattern="$2"
  local out="${E2E_WORK_DIR}/${name}.out"

  if ! grep -q "${pattern}" "${out}"; then
    printf '%s\n' "--- ${out} ---" >&2
    cat "${out}" >&2
    fail "expected output to contain: ${pattern}"
  fi
}

wait_file_contains() {
  local path="$1"
  local pattern="$2"

  for _ in $(seq 1 100); do
    if [[ -f "${path}" ]] && grep -q "${pattern}" "${path}"; then
      return 0
    fi
    sleep 0.1
  done

  return 1
}


assert_sudo_path_exists() {
  local path="$1"
  sudo_cmd test -e "${path}" || fail "expected path to exist: ${path}"
}

assert_sudo_path_absent() {
  local path="$1"
  sudo_cmd test ! -e "${path}" || fail "expected path to be absent: ${path}"
}

extract_created_id() {
  local name="$1"
  awk '/: .* (created|applied)$/ || /: .* created / { print $2; exit }' "${E2E_WORK_DIR}/${name}.out"
}

resolve_container_id() {
  local ref="$1"
  local name="$2"
  local out="${E2E_WORK_DIR}/container-ls-resolve-${name}.out"
  local cid=""

  if [[ -n "${ref}" ]] && sudo_cmd test -f "/etc/raind/container/${ref}/config.json"; then
    printf '%s\n' "${ref}"
    return
  fi

  if ! sudo_cmd env PATH="${PATH}" "${RAIND_BIN}" container ls >"${out}" 2>&1; then
    printf '%s\n' "--- ${out} ---" >&2
    cat "${out}" >&2 || true
    fail "raind container ls failed while resolving container id for ${name}"
  fi

  cid="$(awk -v ref="${ref}" -v name="${name}" '
    NR == 1 { next }
    $1 == ref || $NF == ref || $NF == name { print $1; exit }
  ' "${out}")"

  [[ -n "${cid}" ]] || fail "could not resolve container id for ${name} from ref ${ref}"
  printf '%s\n' "${cid}"
}

extract_policy_id() {
  local name="$1"
  awk '/^policy: .* created$/ { print $2; exit }' "${E2E_WORK_DIR}/${name}.out"
}

wait_raind_contains() {
  local name="$1"
  local pattern="$2"
  shift 2

  for _ in $(seq 1 120); do
    if sudo_cmd env PATH="${PATH}" "${RAIND_BIN}" "$@" >"${E2E_WORK_DIR}/${name}.out" 2>&1 &&
      grep -q "${pattern}" "${E2E_WORK_DIR}/${name}.out"; then
      return
    fi
    sleep 0.5
  done

  printf '%s\n' "--- ${E2E_WORK_DIR}/${name}.out ---" >&2
  cat "${E2E_WORK_DIR}/${name}.out" >&2 || true
  fail "timed out waiting for raind $* output to contain: ${pattern}"
}

wait_resource_namespace_absent() {
  local name="$1"
  local ns="$2"
  local out="${E2E_WORK_DIR}/${name}.out"

  for _ in $(seq 1 180); do
    if ! sudo_cmd env PATH="${PATH}" "${RAIND_BIN}" resource namespace show "${ns}" >"${out}" 2>&1; then
      return
    fi
    if grep -Eqi '(not found|does not exist)' "${out}"; then
      return
    fi
    sleep 0.5
  done

  printf '%s\n' "--- ${out} ---" >&2
  cat "${out}" >&2 || true
  fail "timed out waiting for resource namespace to be removed: ${ns}"
}

wait_http_ok() {
  local url="$1"
  local out="${E2E_WORK_DIR}/http.out"

  log "curl ${url}"
  for _ in $(seq 1 120); do
    if curl -fsS --connect-timeout 2 --max-time 3 "${url}" >"${out}" 2>"${E2E_WORK_DIR}/http.err" &&
      grep -qi "nginx" "${out}"; then
      return
    fi
    sleep 0.5
  done

  cat "${E2E_WORK_DIR}/http.err" >&2 2>/dev/null || true
  if [[ -f "${out}" ]]; then
    printf '%s\n' "--- ${out} ---" >&2
    cat "${out}" >&2 || true
  fi
  fail "timed out waiting for ${url}"
}

assert_security_profile_runtime_applied() {
  local cid="$1"
  local state="/etc/raind/container/${cid}/state.json"
  local pid

  pid="$(sudo_cmd jq -r '.pid // 0' "${state}")"
  [[ "${pid}" =~ ^[0-9]+$ && "${pid}" -gt 0 ]] || fail "container ${cid} has no running pid"
  sudo_cmd test -d "/proc/${pid}" || fail "container ${cid} pid ${pid} is not running"
  sudo_cmd grep -q '^NoNewPrivs:[[:space:]]*1$' "/proc/${pid}/status" || fail "NoNewPrivs is not enabled for ${cid}"
  sudo_cmd grep -q '^Seccomp:[[:space:]]*2$' "/proc/${pid}/status" || fail "seccomp filter mode is not enabled for ${cid}"
}

assert_dev_security_profile_applied() {
  local cid="$1"
  local config="/etc/raind/container/${cid}/config.json"

  log "verify dev security profile for ${cid}"
  sudo_cmd jq -e '
    .linux.apparmorProfile == "raind-default" and
    .linux.seccomp.defaultAction == "SCMP_ACT_ALLOW" and
    any(.linux.seccomp.syscalls[]?.names[]?; . == "unshare") and
    any(.process.capabilities.effective[]?; . == "CAP_NET_RAW")
  ' "${config}" >/dev/null || fail "dev security profile was not written to ${config}"

  assert_security_profile_runtime_applied "${cid}"
}

assert_deploy_security_profile_applied() {
  local cid="$1"
  local config="/etc/raind/container/${cid}/config.json"

  log "verify deploy security profile for ${cid}"
  sudo_cmd jq -e '
    .linux.apparmorProfile == "raind-default" and
    .linux.seccomp.defaultAction == "SCMP_ACT_ALLOW" and
    any(.linux.seccomp.syscalls[]?.names[]?; . == "unshare") and
    (any(.process.capabilities.effective[]?; . == "CAP_CHOWN")) and
    ([.process.capabilities.effective[]?] | index("CAP_NET_RAW") | not) and
    ([.process.capabilities.effective[]?] | index("CAP_MKNOD") | not)
  ' "${config}" >/dev/null || fail "deploy security profile was not written to ${config}"

  assert_security_profile_runtime_applied "${cid}"
}

assert_restricted_security_profile_applied() {
  local cid="$1"
  local config="/etc/raind/container/${cid}/config.json"

  log "verify restricted security profile for ${cid}"
  sudo_cmd jq -e '
    .linux.apparmorProfile == "raind-default" and
    .linux.seccomp.defaultAction == "SCMP_ACT_ALLOW" and
    any(.linux.seccomp.syscalls[]?.names[]?; . == "unshare") and
    ((.process.capabilities.effective // []) | length == 0) and
    ((.process.capabilities.bounding // []) | length == 0) and
    ((.process.capabilities.permitted // []) | length == 0)
  ' "${config}" >/dev/null || fail "restricted security profile was not written to ${config}"

  assert_security_profile_runtime_applied "${cid}"
}

assert_unconfined_security_profile_applied() {
  local cid="$1"
  local config="/etc/raind/container/${cid}/config.json"

  log "verify unconfined security profile for ${cid}"
  sudo_cmd jq -e '
    (.linux.seccomp == null) and
    (.linux.apparmorProfile == null) and
    (any(.process.capabilities.effective[]?; . == "CAP_NET_RAW")) and
    ([.process.capabilities.effective[]?] | index("CAP_SYS_ADMIN") | not)
  ' "${config}" >/dev/null || fail "unconfined security profile was not written to ${config}"
}

assert_custom_security_profile_applied() {
  local cid="$1"
  local config="/etc/raind/container/${cid}/config.json"

  log "verify custom security profile for ${cid}"
  sudo_cmd jq -e '
    .linux.apparmorProfile == "raind-default" and
    .linux.seccomp.defaultAction == "SCMP_ACT_ALLOW" and
    any(.linux.seccomp.syscalls[]?.names[]?; . == "unshare") and
    (any(.process.capabilities.effective[]?; . == "CAP_SYS_PTRACE")) and
    ([.process.capabilities.effective[]?] | index("CAP_AUDIT_WRITE") | not) and
    ([.process.capabilities.effective[]?] | index("CAP_NET_RAW") | not) and
    ([.process.capabilities.effective[]?] | index("CAP_MKNOD") | not)
  ' "${config}" >/dev/null || fail "custom security profile was not written to ${config}"

  assert_security_profile_runtime_applied "${cid}"
}

write_static_build_context() {
  local dir="$1"
  mkdir -p "${dir}/assets"
  cat >"${dir}/Dockerfile" <<'DOCKERFILE'
FROM busybox:latest
COPY message.txt /message.txt
CMD ["cat", "/message.txt"]
DOCKERFILE
  printf 'raind image build e2e\n' >"${dir}/message.txt"
  printf 'asset\n' >"${dir}/assets/file.txt"
}

test_image() {
  local tag="local/e2e-image-${SUFFIX}:latest"
  local context="${E2E_WORK_DIR}/image-context"

  log "image test"
  run_raind image-pull-busybox image pull busybox:latest
  assert_output_contains image-pull-busybox "pull completed"
  run_raind image-pull-nginx image pull nginx:latest
  assert_output_contains image-pull-nginx "pull completed"

  write_static_build_context "${context}"
  run_raind image-build image build -t "${tag}" "${context}"
  assert_output_contains image-build "build completed"
  run_raind image-ls image ls
  assert_output_contains image-ls "busybox"
  assert_output_contains image-ls "nginx"
  assert_output_contains image-ls "local/e2e-image-${SUFFIX}"

  local built_cache="/etc/raind/image/layers/e2e-image-${SUFFIX}/latest/rootless-shifted"
  sudo_cmd mkdir -p "${built_cache}/sentinel"
  assert_sudo_path_exists "${built_cache}/sentinel"
  run_raind image-rm image rm "${tag}"
  assert_output_contains image-rm "remove completed"
  assert_sudo_path_absent "${built_cache}"
}

test_network() {
  local bridge="e2enet${SUFFIX}"
  local cid

  log "network test"
  run_raind network-create network create "${bridge}"
  assert_output_contains network-create "created"
  run_raind network-ls network ls
  assert_output_contains network-ls "${bridge}"

  run_raind network-container-create container create --network "${bridge}" --name "e2e-net-${SUFFIX}" busybox:latest sleep 30
  cid="$(extract_created_id network-container-create)"
  [[ -n "${cid}" ]] || fail "network container id not found"
  run_raind network-container-start container start "${cid}"
  assert_output_contains network-container-start "started"
  run_raind network-container-ls container ls
  assert_output_contains network-container-ls "e2e-net-${SUFFIX}"
  run_raind_allow_empty network-container-stop container stop "${cid}"
  run_raind_allow_empty network-container-rm container rm "${cid}"
  run_raind network-rm network rm "${bridge}"
  assert_output_contains network-rm "delete network"
}

test_policy() {
  local source_id
  local dest_id
  local policy_id

  log "policy test"
  run_raind policy-source-create container create --name "e2e-policy-src-${SUFFIX}" busybox:latest sleep 30
  source_id="$(extract_created_id policy-source-create)"
  run_raind policy-dest-create container create --name "e2e-policy-dst-${SUFFIX}" busybox:latest sleep 30
  dest_id="$(extract_created_id policy-dest-create)"
  [[ -n "${source_id}" && -n "${dest_id}" ]] || fail "policy container ids not found"

  run_raind policy-add policy add --type ew --source "e2e-policy-src-${SUFFIX}" --destination "e2e-policy-dst-${SUFFIX}" --protocol tcp --dport 80 --comment "e2e policy"
  policy_id="$(extract_policy_id policy-add)"
  [[ -n "${policy_id}" ]] || fail "policy id not found"
  run_raind policy-ls policy ls --type ew
  assert_output_contains policy-ls "${policy_id}"
  run_raind policy-rm policy rm "${policy_id}"
  assert_output_contains policy-rm "remove"
  run_raind policy-revert policy revert
  assert_output_contains policy-revert "revert success"
  run_raind policy-ns-mode-enforce policy ns-mode enforce
  assert_output_contains policy-ns-mode-enforce "enforce"
  run_raind policy-ns-mode-observe policy ns-mode observe
  assert_output_contains policy-ns-mode-observe "observe"

  run_raind_allow_empty policy-source-rm container rm "${source_id}"
  run_raind_allow_empty policy-dest-rm container rm "${dest_id}"
}

test_container_deploy() {
  local port=$((18100 + SUFFIX % 1000))
  local cid

  log "container deploy test"
  run_raind container-create container create --name "e2e-web-${SUFFIX}" -p "${port}:80" nginx:latest
  cid="$(extract_created_id container-create)"
  [[ -n "${cid}" ]] || fail "container id not found"
  run_raind container-start container start "${cid}"
  assert_output_contains container-start "started"
  wait_http_ok "http://${HOST_ADDR}:${port}/"
  run_raind_allow_empty container-exec container exec "${cid}" nginx -v
  run_raind_allow_empty container-logs container logs --line 20 "${cid}"
  run_raind_allow_empty container-stop container stop "${cid}"
  run_raind_allow_empty container-rm container rm "${cid}"
}

test_dev_security_profile_container() {
  local cid
  local name="e2e-dev-profile-${SUFFIX}"

  log "dev security profile container test"
  run_raind dev-profile-run container run --security-profile dev --name "${name}" busybox:latest sleep 30
  cid="$(extract_created_id dev-profile-run)"
  cid="$(resolve_container_id "${cid}" "${name}")"
  assert_dev_security_profile_applied "${cid}"
  run_raind_allow_empty dev-profile-stop container stop "${cid}"
  run_raind_allow_empty dev-profile-rm container rm "${cid}"
}

test_deploy_security_profile_container() {
  local cid
  local name="e2e-deploy-profile-${SUFFIX}"

  log "deploy security profile container test"
  run_raind deploy-profile-run container run --security-profile deploy --name "${name}" busybox:latest sleep 30
  cid="$(extract_created_id deploy-profile-run)"
  cid="$(resolve_container_id "${cid}" "${name}")"
  assert_deploy_security_profile_applied "${cid}"
  run_raind_allow_empty deploy-profile-stop container stop "${cid}"
  run_raind_allow_empty deploy-profile-rm container rm "${cid}"
}

test_restricted_security_profile_container() {
  local cid
  local name="e2e-restricted-profile-${SUFFIX}"

  log "restricted security profile container test"
  run_raind restricted-profile-run container run --security-profile restricted --name "${name}" busybox:latest sleep 30
  cid="$(extract_created_id restricted-profile-run)"
  cid="$(resolve_container_id "${cid}" "${name}")"
  assert_restricted_security_profile_applied "${cid}"
  run_raind_allow_empty restricted-profile-stop container stop "${cid}"
  run_raind_allow_empty restricted-profile-rm container rm "${cid}"
}

test_unconfined_security_profile_container() {
  local cid
  local name="e2e-unconfined-profile-${SUFFIX}"

  log "unconfined security profile container test"
  run_raind unconfined-profile-run container run --security-profile unconfined --name "${name}" busybox:latest sleep 30
  cid="$(extract_created_id unconfined-profile-run)"
  cid="$(resolve_container_id "${cid}" "${name}")"
  assert_unconfined_security_profile_applied "${cid}"
  run_raind_allow_empty unconfined-profile-stop container stop "${cid}"
  run_raind_allow_empty unconfined-profile-rm container rm "${cid}"
}

test_custom_security_profile_container() {
  local cid
  local name="e2e-custom-profile-container-${SUFFIX}"
  local profile="e2e-custom-profile-${SUFFIX}"

  log "custom security profile register and container test"
  cat >"${E2E_WORK_DIR}/custom-security-profile.yaml" <<YAML
apiVersion: raind.io/v1
kind: SecurityProfile
metadata:
  name: ${profile}
spec:
  extends: deploy
  add-cap:
    - CAP_SYS_PTRACE
  drop-cap:
    - CAP_AUDIT_WRITE
YAML
  run_raind custom-profile-register security profile register -f "${E2E_WORK_DIR}/custom-security-profile.yaml"
  assert_output_contains custom-profile-register "security profile: ${profile} registered"
  run_raind custom-profile-show security profile show "${profile}"
  assert_output_contains custom-profile-show "name: ${profile}"
  assert_output_contains custom-profile-show "type: custom"
  assert_output_contains custom-profile-show "extends: deploy"
  assert_output_contains custom-profile-show "CAP_SYS_PTRACE"

  run_raind custom-profile-run container run --security-profile "${profile}" --name "${name}" busybox:latest sleep 30
  cid="$(extract_created_id custom-profile-run)"
  cid="$(resolve_container_id "${cid}" "${name}")"
  assert_custom_security_profile_applied "${cid}"
  run_raind_allow_empty custom-profile-stop container stop "${cid}"
  run_raind_allow_empty custom-profile-rm container rm "${cid}"
  run_raind custom-profile-delete security profile delete "${profile}"
  assert_output_contains custom-profile-delete "deleted security profile: ${profile}"
  assert_raind_fails custom-profile-show-deleted security profile show "${profile}"
}

test_rootless_container_cache() {
  local first_port=$((18600 + SUFFIX % 1000))
  local second_port=$((19600 + SUFFIX % 1000))
  local first_id
  local second_id
  local nginx_cache="/etc/raind/image/layers/library/nginx/latest/rootless-shifted"

  log "rootless container cache test"
  sudo_cmd rm -rf "${nginx_cache}"

  run_raind rootless-run-first container run --rootless --name "e2e-rootless-a-${SUFFIX}" -p "${first_port}:80" nginx:latest
  assert_output_contains rootless-run-first "creating rootless shifted layer cache"
  wait_http_ok "http://${HOST_ADDR}:${first_port}/"
  first_id="$(extract_created_id rootless-run-first)"
  if [[ -z "${first_id}" ]]; then
    first_id="e2e-rootless-a-${SUFFIX}"
  fi
  assert_sudo_path_exists "${nginx_cache}"
  assert_sudo_path_exists "${nginx_cache}/uid_100000_gid_100000_size_65536_v1/.raind-rootless-shift-complete"
  run_raind_allow_empty rootless-stop-first container stop "${first_id}"
  run_raind_allow_empty rootless-rm-first container rm "${first_id}"

  run_raind rootless-run-second container run --rootless --name "e2e-rootless-b-${SUFFIX}" -p "${second_port}:80" nginx:latest
  assert_output_contains rootless-run-second "rootless shifted layer cache found"
  wait_http_ok "http://${HOST_ADDR}:${second_port}/"
  second_id="$(extract_created_id rootless-run-second)"
  if [[ -z "${second_id}" ]]; then
    second_id="e2e-rootless-b-${SUFFIX}"
  fi
  run_raind_allow_empty rootless-logs-second container logs --line 80 "${second_id}"
  assert_output_contains rootless-logs-second "start worker process"
  run_raind_allow_empty rootless-stop-second container stop "${second_id}"
  run_raind_allow_empty rootless-rm-second container rm "${second_id}"
}

test_login_rootless_bind_mount() {
  local cid
  local login_uid
  local login_gid
  local bind_dir="${E2E_WORK_DIR}/login-root-bind"
  local cache_root
  local shifted_cache_root="/etc/raind/image/layers/library/nginx/latest/rootless-shifted/uid_100000_gid_100000_size_65536_v1"
  local host_owner
  local name="e2e-login-root-${SUFFIX}"

  log "login-root rootless bind mount test"
  login_uid="$(id -u)"
  login_gid="$(id -g)"
  if [[ "${login_uid}" == "0" || "${login_gid}" == "0" ]]; then
    log "skip login-root rootless bind mount test: needs a non-root login user"
    return
  fi

  rm -rf "${bind_dir}"
  mkdir -p "${bind_dir}"
  chmod 0775 "${bind_dir}"

  cache_root="/etc/raind/image/layers/library/nginx/latest/rootless-shifted/mode_login-root_rootuid_${login_uid}_rootgid_${login_gid}_uid_100000_gid_100000_size_65536_v1"
  sudo_cmd rm -rf "${cache_root}"
  assert_sudo_path_exists "${shifted_cache_root}/.raind-rootless-shift-complete"

  run_raind login-root-create container create --rootless-mode login-root --name "${name}" -v "${bind_dir}:/data" nginx:latest /bin/sh -c "sleep 60"
  cid="$(extract_created_id login-root-create)"
  [[ -n "${cid}" ]] || fail "login-root container id not found"
  run_raind login-root-start container start "${cid}"
  assert_output_contains login-root-start "started"
  assert_sudo_path_exists "${cache_root}/rootfs"
  assert_sudo_path_exists "${cache_root}/.raind-rootless-shift-complete"
  assert_sudo_path_exists "${shifted_cache_root}/.raind-rootless-shift-complete"

  run_raind_allow_empty login-root-exec container exec "${cid}" /bin/sh -c "echo login-root-e2e > /data/hello.txt && cat /data/hello.txt"
  if ! wait_file_contains "${bind_dir}/hello.txt" "login-root-e2e"; then
    printf '%s\n' "--- ${E2E_WORK_DIR}/login-root-exec.out ---" >&2
    cat "${E2E_WORK_DIR}/login-root-exec.out" >&2 || true
    run_raind_allow_empty login-root-logs container logs --line 80 "${cid}" || true
    printf '%s\n' "--- ${E2E_WORK_DIR}/login-root-logs.out ---" >&2
    cat "${E2E_WORK_DIR}/login-root-logs.out" >&2 || true
    fail "login-root bind file was not created with expected content on host"
  fi
  host_owner="$(stat -c '%u:%g' "${bind_dir}/hello.txt")"
  [[ "${host_owner}" == "${login_uid}:${login_gid}" ]] || fail "unexpected login-root bind file owner: ${host_owner}, expected ${login_uid}:${login_gid}"

  run_raind_allow_empty login-root-stop container stop "${cid}"
  run_raind_allow_empty login-root-rm container rm "${cid}"
  rm -rf "${bind_dir}"
}

test_bottle_deploy() {
  local port=$((19100 + SUFFIX % 1000))
  local yaml="${E2E_WORK_DIR}/bottle.yaml"
  local name="e2e-bottle-${SUFFIX}"

  log "bottle deploy test"
  cat >"${yaml}" <<YAML
bottle:
  name: ${name}
services:
  web:
    image: nginx:latest
    ports:
      - "${port}:80"
YAML
  run_raind bottle-create bottle create -f "${yaml}"
  assert_output_contains bottle-create "created"
  run_raind bottle-ls bottle ls
  assert_output_contains bottle-ls "${name}"
  run_raind_allow_empty bottle-start bottle start "${name}"
  wait_http_ok "http://${HOST_ADDR}:${port}/"
  run_raind bottle-show bottle show "${name}"
  assert_output_contains bottle-show "${name}"
  run_raind_allow_empty bottle-stop bottle stop "${name}"
  run_raind_allow_empty bottle-rm bottle rm "${name}"
}

test_resource_namespace() {
  local ns="e2e-ns-${SUFFIX}"

  log "resource namespace test"
  run_raind ns-create resource namespace create "${ns}"
  assert_output_contains ns-create "created"
  run_raind ns-show resource namespace show "${ns}"
  assert_output_contains ns-show "${ns}"
  run_raind ns-ls resource namespace ls
  assert_output_contains ns-ls "${ns}"
  run_raind ns-rm resource namespace rm "${ns}"
  assert_output_contains ns-rm "removed"
}

test_resource_pod() {
  local yaml="${E2E_WORK_DIR}/pod.yaml"
  local ns="e2e-pod-ns-${SUFFIX}"

  log "resource pod test"
  cat >"${yaml}" <<YAML
apiVersion: v1
kind: Namespace
metadata:
  name: ${ns}
---
apiVersion: v1
kind: Pod
metadata:
  name: e2e-pod
  namespace: ${ns}
  labels:
    app: e2e-pod
spec:
  containers:
  - name: nginx
    image: nginx:latest
YAML
  run_resource_apply_with_retry pod-apply "${yaml}"
  assert_output_contains pod-apply "pod:"
  wait_raind_contains pod-ls "e2e-pod" resource pod ls --namespace "${ns}"
  wait_raind_contains pod-ready "1/1" resource pod ls --namespace "${ns}"
  run_raind pod-rm-manifest resource rm -f "${yaml}"
  assert_output_contains pod-rm-manifest "pod:"
  assert_output_contains pod-rm-manifest "namespace:"
  wait_resource_namespace_absent pod-namespace-removed "${ns}"
}

test_resource_replicaset() {
  local yaml="${E2E_WORK_DIR}/replicaset.yaml"
  local ns="e2e-rs-ns-${SUFFIX}"

  log "resource replicaset test"
  cat >"${yaml}" <<YAML
apiVersion: v1
kind: Namespace
metadata:
  name: ${ns}
---
apiVersion: apps/v1
kind: ReplicaSet
metadata:
  name: e2e-rs
  namespace: ${ns}
spec:
  replicas: 2
  selector:
    matchLabels:
      app: e2e-rs
  template:
    metadata:
      labels:
        app: e2e-rs
    spec:
      containers:
      - name: nginx
        image: nginx:latest
YAML
  run_resource_apply_with_retry rs-apply "${yaml}"
  assert_output_contains rs-apply "replicaset:"
  wait_raind_contains rs-ready "2        2        2" resource replicaset ls --namespace "${ns}"
  run_raind rs-rm-manifest resource rm -f "${yaml}"
  assert_output_contains rs-rm-manifest "replicaset:"
  assert_output_contains rs-rm-manifest "namespace:"
  wait_resource_namespace_absent rs-namespace-removed "${ns}"
}

test_resource_deployment() {
  local yaml="${E2E_WORK_DIR}/deployment.yaml"
  local ns="e2e-deploy-ns-${SUFFIX}"

  log "resource deployment test"
  cat >"${yaml}" <<YAML
apiVersion: v1
kind: Namespace
metadata:
  name: ${ns}
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: e2e-deploy
  namespace: ${ns}
spec:
  replicas: 2
  selector:
    matchLabels:
      app: e2e-deploy
  template:
    metadata:
      labels:
        app: e2e-deploy
    spec:
      containers:
      - name: nginx
        image: nginx:latest
YAML
  run_resource_apply_with_retry deploy-apply "${yaml}"
  assert_output_contains deploy-apply "deployment:"
  wait_raind_contains deploy-ready "2        2        2" resource deployment ls --namespace "${ns}"
  run_raind deploy-rm-manifest resource rm -f "${yaml}"
  assert_output_contains deploy-rm-manifest "deployment:"
  assert_output_contains deploy-rm-manifest "namespace:"
  wait_resource_namespace_absent deploy-namespace-removed "${ns}"
}

test_resource_service() {
  local port=$((20100 + SUFFIX % 1000))
  local yaml="${E2E_WORK_DIR}/service.yaml"
  local ns="e2e-svc-ns-${SUFFIX}"

  log "resource service test"
  cat >"${yaml}" <<YAML
apiVersion: v1
kind: Namespace
metadata:
  name: ${ns}
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: e2e-svc-web
  namespace: ${ns}
spec:
  replicas: 2
  selector:
    matchLabels:
      app: e2e-svc-web
  template:
    metadata:
      labels:
        app: e2e-svc-web
    spec:
      containers:
      - name: nginx
        image: nginx:latest
---
apiVersion: v1
kind: Service
metadata:
  name: e2e-svc
  namespace: ${ns}
spec:
  type: NodePort
  selector:
    app: e2e-svc-web
  ports:
  - port: ${port}
    targetPort: 80
    protocol: TCP
YAML
  run_resource_apply_with_retry svc-apply "${yaml}"
  assert_output_contains svc-apply "service:"
  wait_raind_contains svc-deploy-ready "2        2        2" resource deployment ls --namespace "${ns}"
  wait_raind_contains svc-ls "e2e-svc" resource service ls --namespace "${ns}"
  wait_http_ok "http://${HOST_ADDR}:${port}/"
  run_raind svc-rm-manifest resource rm -f "${yaml}"
  assert_output_contains svc-rm-manifest "service:"
  assert_output_contains svc-rm-manifest "deployment:"
  assert_output_contains svc-rm-manifest "namespace:"
  wait_resource_namespace_absent svc-namespace-removed "${ns}"
}

test_resource_yaml_all_kinds() {
  local port=$((21100 + SUFFIX % 1000))
  local yaml="${E2E_WORK_DIR}/all-kinds.yaml"
  local ns="e2e-yaml-ns-${SUFFIX}"

  log "resource yaml all-kinds test"
  cat >"${yaml}" <<YAML
apiVersion: v1
kind: Namespace
metadata:
  name: ${ns}
---
apiVersion: v1
kind: Pod
metadata:
  name: e2e-yaml-pod
  namespace: ${ns}
  labels:
    app: e2e-yaml-pod
spec:
  containers:
  - name: nginx
    image: nginx:latest
---
apiVersion: apps/v1
kind: ReplicaSet
metadata:
  name: e2e-yaml-rs
  namespace: ${ns}
spec:
  replicas: 1
  selector:
    matchLabels:
      app: e2e-yaml-rs
  template:
    metadata:
      labels:
        app: e2e-yaml-rs
    spec:
      containers:
      - name: nginx
        image: nginx:latest
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: e2e-yaml-deploy
  namespace: ${ns}
spec:
  replicas: 1
  selector:
    matchLabels:
      app: e2e-yaml-deploy
  template:
    metadata:
      labels:
        app: e2e-yaml-deploy
    spec:
      containers:
      - name: nginx
        image: nginx:latest
---
apiVersion: v1
kind: Service
metadata:
  name: e2e-yaml-svc
  namespace: ${ns}
spec:
  type: NodePort
  selector:
    app: e2e-yaml-deploy
  ports:
  - port: ${port}
    targetPort: 80
    protocol: TCP
YAML
  run_resource_apply_with_retry yaml-apply "${yaml}"
  assert_output_contains yaml-apply "namespace:"
  assert_output_contains yaml-apply "pod:"
  assert_output_contains yaml-apply "replicaset:"
  assert_output_contains yaml-apply "deployment:"
  assert_output_contains yaml-apply "service:"
  wait_raind_contains yaml-deploy-ready "1        1        1" resource deployment ls --namespace "${ns}"
  wait_raind_contains yaml-rs-ready "1        1        1" resource replicaset ls --namespace "${ns}"
  wait_http_ok "http://${HOST_ADDR}:${port}/"
  run_raind yaml-rm resource rm -f "${yaml}"
  assert_output_contains yaml-rm "namespace:"
  assert_output_contains yaml-rm "pod:"
  assert_output_contains yaml-rm "replicaset:"
  assert_output_contains yaml-rm "deployment:"
  assert_output_contains yaml-rm "service:"
  wait_resource_namespace_absent yaml-namespace-removed "${ns}"
}

main() {
  require_workshop
  rm -rf "${E2E_WORK_DIR}"
  mkdir -p "${E2E_WORK_DIR}"

  build_components
  prepare_runtime
  cleanup_stale_condenser
  start_condenser
  trap stop_condenser EXIT
  wait_ready

  test_image
  test_network
  test_policy
  test_container_deploy
  test_dev_security_profile_container
  test_deploy_security_profile_container
  test_restricted_security_profile_container
  test_unconfined_security_profile_container
  test_custom_security_profile_container
  test_rootless_container_cache
  test_login_rootless_bind_mount
  test_bottle_deploy
  test_resource_namespace
  test_resource_pod
  test_resource_replicaset
  test_resource_deployment
  test_resource_service
  test_resource_yaml_all_kinds

  log "raind deploy e2e completed"
}

main "$@"
