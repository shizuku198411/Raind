#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CONDENSER_BIN="${ROOT_DIR}/bin/condenser"
E2E_WORK_DIR="${E2E_WORK_DIR:-/tmp/raind-condenser-e2e}"
LOG_PATH="${E2E_WORK_DIR}/condenser.log"
PID=""

log() {
  printf '==> %s\n' "$*"
}

fail() {
  printf 'error: %s\n' "$*" >&2
  if [[ -f "${LOG_PATH}" ]]; then
    printf '%s\n' '--- condenser log ---' >&2
    tail -120 "${LOG_PATH}" >&2 || true
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
    fail "sudo is required for condenser integration test"
  fi
  sudo -n "$@"
}

require_workshop() {
  if [[ "${ROOT_DIR}" != /project* ]]; then
    fail "condenser integration test must run inside Workshop. use: workshop run raind-dev -- test-condenser-integ"
  fi
}

build_condenser() {
  log "build all raind components"
  cd "${ROOT_DIR}"
  ./scripts/build.sh build
  [[ -x "${CONDENSER_BIN}" ]] || fail "missing built condenser binary: ${CONDENSER_BIN}"
}

prepare_runtime() {
  log "prepare condenser runtime prerequisites"
  sudo_cmd mkdir -p \
    /etc/raind/log \
    /etc/raind/cert \
    /etc/raind/store \
    /etc/raind/container \
    /etc/raind/image/layers \
    /var/log/raind \
    /sys/fs/cgroup/raind
  sudo_cmd chmod 0755 /etc/raind /etc/raind/log /etc/raind/cert /etc/raind/store /var/log/raind

  for controller in cpu memory pids io; do
    if sudo_cmd grep -qw "${controller}" /sys/fs/cgroup/raind/cgroup.controllers; then
      sudo_cmd sh -c "echo +${controller} > /sys/fs/cgroup/raind/cgroup.subtree_control 2>/dev/null || true"
    fi
  done
}

assert_port_free() {
  local port="$1"
  if sudo_cmd ss -ltn "( sport = :${port} )" | grep -q ":${port}"; then
    fail "port ${port} is already in use in this Workshop"
  fi
}

cleanup_stale_condenser() {
  sudo_cmd pkill -TERM -f "^${CONDENSER_BIN}$" 2>/dev/null || true
  sleep 0.2
  sudo_cmd pkill -KILL -f "^${CONDENSER_BIN}$" 2>/dev/null || true
}

start_condenser() {
  log "start condenser"
  mkdir -p "${E2E_WORK_DIR}"
  : > "${LOG_PATH}"

  cleanup_stale_condenser

  for port in 7755 7756 7757 7758; do
    assert_port_free "${port}"
  done

  sudo_cmd env PATH="${PATH}" "${CONDENSER_BIN}" >"${LOG_PATH}" 2>&1 &
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
  local path="$1"
  api_request GET "${path}"
}

api_request() {
  local method="$1"
  local path="$2"
  local body="${3:-}"

  local args=(
    -sS
    --connect-timeout 1
    --max-time 5
    --cert /etc/raind/cert/raindClient.crt
    --key /etc/raind/cert/raindClient.key
    --cacert /etc/raind/cert/raind.crt
    -X "${method}"
  )

  if [[ -n "${body}" ]]; then
    args+=(-H "Content-Type: application/json" -d "${body}")
  fi

  sudo_cmd curl -sS \
    "${args[@]}" \
    "https://127.0.0.1:7755${path}"
}

wait_ready() {
  log "wait for management API"
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

assert_api_success() {
  local path="$1"
  local out="${E2E_WORK_DIR}/$(echo "${path}" | tr '/?' '__').json"

  log "GET ${path}"
  api_curl "${path}" >"${out}"
  jq -e '.status == "success"' "${out}" >/dev/null
}

assert_api_success_method() {
  local name="$1"
  local method="$2"
  local path="$3"
  local body="${4:-}"
  local out="${E2E_WORK_DIR}/${name}.json"

  log "${method} ${path}"
  api_request "${method}" "${path}" "${body}" >"${out}"
  jq -e '.status == "success"' "${out}" >/dev/null
}

assert_api_fail_method() {
  local name="$1"
  local method="$2"
  local path="$3"
  local body="${4:-}"
  local out="${E2E_WORK_DIR}/${name}.json"

  log "${method} ${path} fails"
  api_request "${method}" "${path}" "${body}" >"${out}"
  jq -e '.status == "fail" and (.message | length > 0)' "${out}" >/dev/null
}

assert_swagger() {
  local out="${E2E_WORK_DIR}/swagger.json"

  log "GET /swagger/doc.json"
  sudo_cmd curl -sk --connect-timeout 1 --max-time 3 \
    "https://127.0.0.1:7758/swagger/doc.json" >"${out}"
  jq -e '.swagger == "2.0"' "${out}" >/dev/null
}

assert_client_cert_required() {
  local code

  log "verify management API requires client certificate"
  code="$(sudo_cmd curl -sk --connect-timeout 1 --max-time 3 \
    -o /dev/null -w '%{http_code}' \
    "https://127.0.0.1:7755/v1/images" || true)"
  if [[ "${code}" == "200" ]]; then
    fail "management API accepted a request without client certificate"
  fi
}

assert_cert_contracts() {
  if ! have_cmd openssl; then
    log "skip certificate contract checks: openssl is unavailable"
    return
  fi

  log "verify certificate SAN/SPIFFE contracts"
  sudo_cmd openssl x509 -in /etc/raind/cert/raind.crt -noout -text |
    grep -Eq "DNS:localhost|IP Address:127.0.0.1"
  sudo_cmd openssl x509 -in /etc/raind/cert/raindClient.crt -noout -text |
    grep -q "URI:spiffe://raind/cli/admin"
  sudo_cmd openssl x509 -in /etc/raind/cert/raindHookClient.crt -noout -text |
    grep -q "URI:spiffe://raind/droplet/container"
}

assert_spiffe_rejections() {
  local code

  log "verify management API rejects non-cli SPIFFE client"
  code="$(sudo_cmd curl -sk --connect-timeout 1 --max-time 3 \
    --cert /etc/raind/cert/raindHookClient.crt \
    --key /etc/raind/cert/raindHookClient.key \
    --cacert /etc/raind/cert/raind.crt \
    -o /dev/null -w '%{http_code}' \
    "https://127.0.0.1:7755/v1/images" || true)"
  [[ "${code}" == "403" ]] || fail "management API returned ${code}, expected 403 for hook client cert"
}

issue_e2e_cli_cert() {
  local spiffe="$1"
  local name="$2"
  local key="${E2E_WORK_DIR}/${name}.key"
  local csr="${E2E_WORK_DIR}/${name}.csr"
  local crt="${E2E_WORK_DIR}/${name}.crt"

  sudo_cmd openssl req \
    -new \
    -newkey rsa:2048 \
    -nodes \
    -keyout "${key}" \
    -out "${csr}" \
    -subj "/CN=raind-${name}" \
    -addext "subjectAltName=URI:${spiffe}" >/dev/null 2>&1

  sudo_cmd openssl x509 \
    -req \
    -in "${csr}" \
    -CA /etc/raind/cert/raindClientCA.crt \
    -CAkey /etc/raind/cert/raindClientCA.key \
    -CAcreateserial \
    -out "${crt}" \
    -days 1 \
    -sha256 \
    -copy_extensions copy >/dev/null 2>&1

  printf '%s\n%s\n' "${crt}" "${key}"
}

scoped_api_request() {
  local cert="$1"
  local key="$2"
  local method="$3"
  local path="$4"
  local out="$5"
  local body="${6:-}"

  local args=(
    -sS
    --connect-timeout 1
    --max-time 5
    --cert "${cert}"
    --key "${key}"
    --cacert /etc/raind/cert/raind.crt
    -X "${method}"
  )

  if [[ -n "${body}" ]]; then
    args+=(-H "Content-Type: application/json" -d "${body}")
  fi

  sudo_cmd curl \
    "${args[@]}" \
    -o "${out}" \
    -w '%{http_code}' \
    "https://127.0.0.1:7755${path}"
}

assert_cli_scope_authorization() {
  if ! have_cmd openssl; then
    log "skip CLI scope authorization checks: openssl is unavailable"
    return
  fi

  local cert
  local key
  local code
  local out="${E2E_WORK_DIR}/read-scope-images.json"

  log "verify read-only CLI scope can read but cannot write"
  mapfile -t issued < <(issue_e2e_cli_cert "spiffe://raind/cli/read" "read-scope-client")
  cert="${issued[0]}"
  key="${issued[1]}"

  code="$(scoped_api_request "${cert}" "${key}" GET "/v1/images" "${out}")"
  [[ "${code}" == "200" ]] || fail "read-scope GET returned ${code}, expected 200"
  jq -e '.status == "success"' "${out}" >/dev/null

  code="$(scoped_api_request "${cert}" "${key}" POST "/v1/networks" "${E2E_WORK_DIR}/read-scope-write.txt" '{"bridge":"scope-denied"}' || true)"
  [[ "${code}" == "403" ]] || fail "read-scope POST returned ${code}, expected 403"
}

assert_read_api_surface() {
  assert_api_success "/v1/images"
  assert_api_success "/v1/containers"
  assert_api_success "/v1/networks"
  assert_api_success "/v1/pods"
  assert_api_success "/v1/services"
  assert_api_success "/v1/namespaces"
  assert_api_success "/v1/bottle"
  assert_api_success "/v1/containers/stats"
  assert_api_success "/v1/policies/RAIND-EW"
  assert_api_success "/v1/policies/RAIND-NS-OBS"
  assert_api_success "/v1/policies/RAIND-NS-ENF"
  assert_api_success "/v1/replicasets"

  log "GET /v1/logs/netflow?tail_lines=5"
  api_curl "/v1/logs/netflow?tail_lines=5" >"${E2E_WORK_DIR}/netflow.log"
}

assert_error_contracts() {
  assert_api_fail_method container-create-missing-image POST "/v1/containers" '{"image":"raind/e2e-missing:latest","name":"e2e-missing-image","network":"raind0"}'
  assert_api_fail_method container-get-unknown GET "/v1/containers/unknown-e2e-container"
  assert_api_fail_method image-status-missing-query GET "/v1/images/status"
  assert_api_fail_method image-fs-missing-query GET "/v1/images/fs"
  assert_api_fail_method image-remove-missing DELETE "/v1/images"
  assert_api_fail_method invalid-json POST "/v1/networks" '{"bridge":'
}

assert_network_lifecycle() {
  local bridge="re2e$$"

  assert_api_success_method network-create POST "/v1/networks" "{\"bridge\":\"${bridge}\"}"
  api_curl "/v1/networks" >"${E2E_WORK_DIR}/networks-after-create.json"
  jq -e --arg bridge "${bridge}" '.data | map(select(.Interface == $bridge or .interface == $bridge or .bridge == $bridge)) | length >= 1' \
    "${E2E_WORK_DIR}/networks-after-create.json" >/dev/null
  assert_api_fail_method network-create-duplicate POST "/v1/networks" "{\"bridge\":\"${bridge}\"}"
  assert_api_success_method network-delete DELETE "/v1/networks/${bridge}/actions/delete"
}

assert_namespace_lifecycle() {
  local ns="e2e-api-ns-$$"

  assert_api_success_method namespace-create POST "/v1/namespaces" "{\"name\":\"${ns}\"}"
  api_curl "/v1/namespaces/${ns}" >"${E2E_WORK_DIR}/namespace-get.json"
  jq -e --arg ns "${ns}" '.status == "success" and .data.name == $ns and (.data.network | length > 0)' \
    "${E2E_WORK_DIR}/namespace-get.json" >/dev/null

  api_curl "/v1/namespaces" >"${E2E_WORK_DIR}/namespaces-after-create.json"
  jq -e --arg ns "${ns}" '.data | map(select(.name == $ns)) | length == 1' \
    "${E2E_WORK_DIR}/namespaces-after-create.json" >/dev/null

  assert_api_success_method namespace-delete DELETE "/v1/namespaces/${ns}/actions/delete"
}

assert_pod_and_service_lifecycle() {
  local pod_name="e2e-pod-$$"
  local svc_name="e2e-svc-$$"
  local apply_svc_name="e2e-apply-svc-$$"
  local pod_id
  local service_id

  assert_api_success_method pod-create POST "/v1/pods" "{\"name\":\"${pod_name}\",\"namespace\":\"default\",\"uid\":\"${pod_name}\",\"labels\":{\"app\":\"e2e\"},\"annotations\":{\"suite\":\"condenser\"},\"containers\":[]}"
  pod_id="$(jq -r '.data.podId' "${E2E_WORK_DIR}/pod-create.json")"
  [[ -n "${pod_id}" && "${pod_id}" != "null" ]] || fail "pod create did not return podId"
  assert_api_success "/v1/pods/${pod_id}"
  assert_api_success_method pod-start POST "/v1/pods/${pod_id}/actions/start"
  assert_api_success_method pod-stop POST "/v1/pods/${pod_id}/actions/stop"

  cat >"${E2E_WORK_DIR}/service.yaml" <<YAML
apiVersion: v1
kind: Service
metadata:
  name: ${svc_name}
  namespace: default
spec:
  selector:
    app: e2e
  ports:
    - port: 8080
      targetPort: 80
      protocol: TCP
YAML
  log "POST /v1/services"
  sudo_cmd curl -sS --connect-timeout 1 --max-time 5 \
    --cert /etc/raind/cert/raindClient.crt \
    --key /etc/raind/cert/raindClient.key \
    --cacert /etc/raind/cert/raind.crt \
    -H "Content-Type: text/plain" \
    --data-binary @"${E2E_WORK_DIR}/service.yaml" \
    "https://127.0.0.1:7755/v1/services" >"${E2E_WORK_DIR}/service-create.json"
  jq -e '.status == "success"' "${E2E_WORK_DIR}/service-create.json" >/dev/null
  service_id="$(jq -r '.data.serviceId' "${E2E_WORK_DIR}/service-create.json")"
  [[ -n "${service_id}" && "${service_id}" != "null" ]] || fail "service create did not return serviceId"
  assert_api_success "/v1/services/${service_id}"

  cat >"${E2E_WORK_DIR}/resource-service.yaml" <<YAML
apiVersion: v1
kind: Service
metadata:
  name: ${apply_svc_name}
  namespace: default
spec:
  selector:
    app: e2e-apply
  ports:
    - port: 9090
      targetPort: 90
      protocol: TCP
YAML
  log "POST /v1/resource/apply"
  sudo_cmd curl -sS --connect-timeout 1 --max-time 5 \
    --cert /etc/raind/cert/raindClient.crt \
    --key /etc/raind/cert/raindClient.key \
    --cacert /etc/raind/cert/raind.crt \
    -H "Content-Type: text/plain" \
    --data-binary @"${E2E_WORK_DIR}/resource-service.yaml" \
    "https://127.0.0.1:7755/v1/resource/apply" >"${E2E_WORK_DIR}/resource-apply-service.json"
  jq -e '.status == "success" and (.data.services | length == 1)' "${E2E_WORK_DIR}/resource-apply-service.json" >/dev/null

  log "POST /v1/resource/delete"
  sudo_cmd curl -sS --connect-timeout 1 --max-time 5 \
    --cert /etc/raind/cert/raindClient.crt \
    --key /etc/raind/cert/raindClient.key \
    --cacert /etc/raind/cert/raind.crt \
    -H "Content-Type: text/plain" \
    --data-binary @"${E2E_WORK_DIR}/resource-service.yaml" \
    "https://127.0.0.1:7755/v1/resource/delete" >"${E2E_WORK_DIR}/resource-delete-service.json"
  jq -e '.status == "success" and (.data.services | length == 1)' "${E2E_WORK_DIR}/resource-delete-service.json" >/dev/null

  assert_api_success_method service-delete DELETE "/v1/services/${service_id}"
  assert_api_success_method pod-delete DELETE "/v1/pods/${pod_id}"
}

main() {
  require_workshop
  mkdir -p "${E2E_WORK_DIR}"

  build_condenser
  prepare_runtime
  start_condenser
  trap stop_condenser EXIT
  wait_ready

  assert_read_api_surface
  assert_swagger
  assert_client_cert_required
  assert_cert_contracts
  assert_spiffe_rejections
  assert_cli_scope_authorization
  assert_error_contracts
  assert_network_lifecycle
  assert_namespace_lifecycle
  assert_pod_and_service_lifecycle

  log "condenser integration test completed"
}

main "$@"
