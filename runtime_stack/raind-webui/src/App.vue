<template>
  <LoginPage
    v-if="!isAuthenticated"
    :auth-submitting="authSubmitting"
    :login-form="loginForm"
    :login-ready="loginReady"
    :auth-error="authError"
    :on-submit-login="submitLogin"
  />
  <div v-else class="layout">
    <aside class="sidebar">
      <div class="brand">
        <img src="/raind_icon.png" alt="Raind icon" class="brand-icon" />
        <h1>Raind</h1>
        <p>Zero Trust Container Runtime</p>
      </div>
      <nav class="menu">
        <section class="menu-group">
          <h4>Overview</h4>
          <button :class="{ active: currentMenu === 'dashboard' }" @click="selectMenu('dashboard')">
            Dashboard
          </button>
        </section>
        <section class="menu-group">
          <h4>Runtime</h4>
          <button :class="{ active: currentMenu === 'container' }" @click="selectMenu('container')">
            Container
          </button>
          <button :class="{ active: currentMenu === 'resource' }" @click="selectMenu('resource')">
            Resource
          </button>
          <button :class="{ active: currentMenu === 'bottle' }" @click="selectMenu('bottle')">
            Bottle
          </button>
          <button :class="{ active: currentMenu === 'image' }" @click="selectMenu('image')">
            Image
          </button>
        </section>
        <section class="menu-group">
          <h4>Security</h4>
          <button :class="{ active: currentMenu === 'policy' }" @click="selectMenu('policy')">
            Policy
          </button>
        </section>
        <section class="menu-group">
          <h4>Observability</h4>
          <button :class="{ active: currentMenu === 'audit' }" @click="selectMenu('audit')">
            Audit Log
          </button>
          <button :class="{ active: currentMenu === 'network' }" @click="selectMenu('network')">
            Network Log
          </button>
        </section>
      </nav>
      <footer class="sidebar-footer">
        created by Shizuku -
        <a href="https://github.com/shizuku198411/Raind" target="_blank" rel="noopener noreferrer">Raind</a>
      </footer>
    </aside>

    <main class="content">
      <header class="topbar">
        <h2>{{ titleByMenu[currentMenu] }}</h2>
        <div class="section-head-actions">
          <span class="selector" v-if="currentUser">User: {{ currentUser }}</span>
          <button class="primary" :disabled="loading" @click="refreshAll">
            {{ loading ? 'Loading...' : 'Refresh' }}
          </button>
          <button class="danger" @click="logout">Logout</button>
        </div>
      </header>

      <DashboardPage
        v-if="currentMenu === 'dashboard'"
        :runtime-name="runtimeName"
        :runtime-version="runtimeVersion"
        :connection-status="connectionStatus"
        :connection-status-class="connectionStatusClass"
        :total-containers="totalContainers"
        :status-counts="statusCounts"
        :bottles-count="bottles.length"
        :pods-count="pods.length"
        :replicasets-count="replicasets.length"
        :services-count="services.length"
        :donut-style="donutStyle"
        :total-pulse-blocks="totalPulseBlocks"
        :rs-pulse-blocks="rsPulseBlocks"
        :pod-pulse-blocks="podPulseBlocks"
        :bottle-pulse-blocks="bottlePulseBlocks"
        :container-pulse-blocks="containerPulseBlocks"
        :pulse-style="pulseStyle"
        :handle-pulse-hover="handlePulseHover"
        :unhealthy-replicasets="unhealthyReplicaSets"
        :non-running-containers="nonRunningContainers"
        :pending-policy-changes="pendingPolicyChanges"
        :log-insights="logInsights"
      />

      <ContainerPage
        v-if="currentMenu === 'container'"
        :containers="containers"
        :status-class="statusClass"
        :can-attach-container="canAttachContainer"
        :can-exec-container="canExecContainer"
        :can-start-container="canStartContainer"
        :can-stop-container="canStopContainer"
        :can-delete-container="canDeleteContainer"
        :open-create-container-overlay="openCreateContainerOverlay"
        :open-container-detail="openContainerDetail"
        :open-container-log="openContainerLog"
        :open-container-spec="openContainerSpec"
        :open-attach-overlay="openAttachOverlay"
        :open-exec-overlay="openExecOverlay"
        :container-action="containerAction"
        :open-action-confirm="openActionConfirm"
      />

      <ResourcePage
        v-if="currentMenu === 'resource'"
        :resource-relations="resourceRelations"
        :replicasets="replicasets"
        :pods="pods"
        :services="services"
        :resource-panels="resourcePanels"
        :status-class="statusClass"
        :replicaset-health-class="replicasetHealthClass"
        :replicaset-health-label="replicasetHealthLabel"
        :selector-text="selectorText"
        :service-ports-text="servicePortsText"
        :pod-containers="podContainers"
        :is-pod-expanded="isPodExpanded"
        :toggle-pod-expand="togglePodExpand"
        :open-resource-detail="openResourceDetail"
        :open-container-log="openContainerLog"
        :open-container-spec="openContainerSpec"
        :open-container-detail="openContainerDetail"
        :open-attach-overlay="openAttachOverlay"
        :open-exec-overlay="openExecOverlay"
        :can-attach-container="canAttachContainer"
        :can-exec-container="canExecContainer"
        :toggle-resource-panel="toggleResourcePanel"
        :open-resource-create-overlay="openResourceCreateOverlay"
      />

      <ImagePage
        v-if="currentMenu === 'image'"
        :images="images"
        :sorted-images="sortedImages"
        :image-name-short="imageNameShort"
        :format-time="formatTime"
        :open-pull-image-overlay="openPullImageOverlay"
        :open-image-delete-overlay="openImageDeleteOverlay"
      />

      <BottlePage
        v-if="currentMenu === 'bottle'"
        :bottles="bottles"
        :status-class="statusClass"
        :can-start-container="canStartContainer"
        :can-stop-container="canStopContainer"
        :can-delete-container="canDeleteContainer"
        :bottle-action="bottleAction"
        :open-bottle-action-confirm="openBottleActionConfirm"
        :open-container-detail="openContainerDetail"
        :open-container-log="openContainerLog"
        :open-container-spec="openContainerSpec"
        :open-attach-overlay="openAttachOverlay"
        :open-exec-overlay="openExecOverlay"
        :can-attach-container="canAttachContainer"
        :can-exec-container="canExecContainer"
        :open-bottle-create-overlay="openBottleCreateOverlay"
      />

      <PolicyPage
        v-if="currentMenu === 'policy'"
        :policy-chains="policyChains"
        :policy-data="policyData"
        :policy-error="policyError"
        :policy-mode-label="policyModeLabel"
        :policy-chain-label="policyChainLabel"
        :ns-mode-summary="nsModeSummary"
        :open-policy-create-overlay="openPolicyCreateOverlay"
        :open-policy-delete-overlay="openPolicyDeleteOverlay"
        :open-policy-commit-overlay="openPolicyCommitOverlay"
        :open-policy-revert-overlay="openPolicyRevertOverlay"
        :open-policy-container-detail="openPolicyContainerDetailByName"
      />

      <AuditLogPage
        v-if="currentMenu === 'audit'"
        :logs="auditLogs"
        :loading="auditLoading"
        :page="auditPage"
        :total="auditTotal"
        :total-pages="auditTotalPages"
        :parse-errors="auditParseErrors"
        :source="auditSource"
        :actor-options="auditActorOptions"
        :format-time="formatTime"
        :change-page="loadAuditLogs"
        :set-filter="setAuditFilter"
      />

      <NetworkLogPage
        v-if="currentMenu === 'network'"
        :traffic="networkTraffic"
        :dns="networkDns"
        :loading-traffic="networkTrafficLoading"
        :loading-dns="networkDnsLoading"
        :format-time="formatTime"
        :load-traffic="loadNetworkTraffic"
        :load-dns="loadNetworkDns"
        :set-traffic-filter="setNetworkTrafficFilter"
        :set-dns-filter="setNetworkDnsFilter"
        :open-container-detail-by-ref="openNetworkContainerDetailByRef"
      />

      <p v-if="error" class="error">{{ error }}</p>
    </main>

    <CreateContainerOverlay
      :visible="createContainerVisible"
      :create-form="createForm"
      :image-filter="imageFilter"
      :image-dropdown-open="imageDropdownOpen"
      :filtered-image-options="filteredImageOptions"
      :create-submitting="createSubmitting"
      :on-close="closeCreateContainerOverlay"
      :on-image-filter-input="onImageFilterInputValue"
      :on-open-image-dropdown="openImageDropdown"
      :on-schedule-close-image-dropdown="scheduleCloseImageDropdown"
      :on-toggle-image-dropdown="toggleImageDropdown"
      :on-select-image-option="selectImageOption"
      :on-add-port-row="addPortRow"
      :on-remove-port-row="removePortRow"
      :on-add-mount-row="addMountRow"
      :on-remove-mount-row="removeMountRow"
      :on-add-env-row="addEnvRow"
      :on-remove-env-row="removeEnvRow"
      :on-submit="submitCreateContainer"
    />

    <PullImageOverlay
      :visible="pullImageVisible"
      :pull-image-form="pullImageForm"
      :pull-image-submitting="pullImageSubmitting"
      :on-close="closePullImageOverlay"
      :on-submit="submitPullImage"
    />

    <ImageDeleteOverlay
      :visible="imageDeleteVisible"
      :image-delete-target="imageDeleteTarget"
      :image-delete-submitting="imageDeleteSubmitting"
      :on-close="closeImageDeleteOverlay"
      :on-submit="submitImageDelete"
    />

    <ActionConfirmOverlay
      :visible="actionConfirmVisible"
      :action-confirm-type="actionConfirmType"
      :action-confirm-id="actionConfirmId"
      :action-confirm-name="actionConfirmName"
      :action-confirm-submitting="actionConfirmSubmitting"
      :on-close="closeActionConfirm"
      :on-submit="submitActionConfirm"
    />

    <ContainerLogOverlay
      :visible="logVisible"
      :log-target-id="logTargetId"
      :log-target-name="logTargetName"
      :log-source-path="logSourcePath"
      :log-loading="logLoading"
      :log-data="logData"
      :on-close="closeContainerLog"
    />

    <ContainerSpecOverlay
      :visible="specVisible"
      :spec-target-id="specTargetId"
      :spec-target-name="specTargetName"
      :spec-loading="specLoading"
      :spec-data="specData"
      :on-close="closeContainerSpec"
    />

    <AttachOverlay
      :visible="attachVisible"
      :attach-target-id="attachTargetId"
      :attach-target-name="attachTargetName"
      :attach-connecting="attachConnecting"
      :attach-connected="attachConnected"
      :attach-error="attachError"
      :on-close="closeAttachOverlay"
      :on-focus-terminal="focusAttachTerminal"
      :set-terminal-el="setAttachTerminalEl"
    />

    <ExecOverlay
      :visible="execVisible"
      :exec-target-id="execTargetId"
      :exec-target-name="execTargetName"
      :exec-command-text="execCommandText"
      :exec-tty="execTTY"
      :exec-command-ready="execCommandReady"
      :exec-submitting="execSubmitting"
      :exec-connecting="execConnecting"
      :exec-connected="execConnected"
      :exec-error="execError"
      :exec-result="execResult"
      :on-close="closeExecOverlay"
      :on-submit="submitExec"
      :on-input-command="setExecCommandText"
      :on-change-tty="setExecTTYValue"
      :on-focus-terminal="focusExecTerminal"
      :set-terminal-el="setExecTerminalEl"
    />

    <ContainerDetailOverlay
      :visible="detailVisible"
      :detail-target-id="detailTargetId"
      :detail-loading="detailLoading"
      :detail-data="detailData"
      :format-time="formatTime"
      :format-image="formatImage"
      :format-command="formatCommand"
      :runtime-exit-code="runtimeExitCode"
      :runtime-reason="runtimeReason"
      :runtime-message="runtimeMessage"
      :format-percent="formatPercent"
      :format-bytes="formatBytes"
      :on-open-pod-detail="openPodDetailFromContainerDetail"
      :on-open-bottle-detail="openBottleDetail"
      :on-close="closeContainerDetail"
    />

    <ResourceDetailOverlay
      :visible="resourceDetailVisible"
      :resource-detail-type="resourceDetailType"
      :resource-detail-id="resourceDetailId"
      :resource-detail-loading="resourceDetailLoading"
      :resource-detail-data="resourceDetailData"
      :resource-field="resourceField"
      :resource-field-raw="resourceFieldRaw"
      :format-time="formatTime"
      :selector-text="selectorText"
      :service-ports-text="servicePortsText"
      :object-text="objectText"
      :on-close="closeResourceDetail"
    />

    <BottleDetailOverlay
      :visible="bottleDetailVisible"
      :bottle-detail-id="bottleDetailId"
      :bottle-detail-loading="bottleDetailLoading"
      :bottle-detail-data="bottleDetailData"
      :format-time="formatTime"
      :on-close="closeBottleDetail"
    />

    <PolicyCreateOverlay
      :visible="policyCreateVisible"
      :policy-create-form="policyCreateForm"
      :policy-create-submitting="policyCreateSubmitting"
      :container-name-options="containerNameOptions"
      :on-close="closePolicyCreateOverlay"
      :on-submit="submitPolicyCreate"
      :on-update-form="updatePolicyCreateForm"
    />

    <PolicyDeleteOverlay
      :visible="policyDeleteVisible"
      :policy-delete-submitting="policyDeleteSubmitting"
      :policy-delete-id="policyDeleteId"
      :policy-delete-chain="policyDeleteChain"
      :on-close="closePolicyDeleteOverlay"
      :on-submit="submitPolicyDelete"
    />

    <PolicyCommitOverlay
      :visible="policyCommitVisible"
      :policy-commit-submitting="policyCommitSubmitting"
      :on-close="closePolicyCommitOverlay"
      :on-submit="submitPolicyCommit"
    />

    <PolicyRevertOverlay
      :visible="policyRevertVisible"
      :policy-revert-submitting="policyRevertSubmitting"
      :on-close="closePolicyRevertOverlay"
      :on-submit="submitPolicyRevert"
    />

    <BottleActionConfirmOverlay
      :visible="bottleActionConfirmVisible"
      :action-type="bottleActionConfirmType"
      :target-id="bottleActionConfirmId"
      :target-name="bottleActionConfirmName"
      :submitting="bottleActionConfirmSubmitting"
      :on-close="closeBottleActionConfirm"
      :on-submit="submitBottleActionConfirm"
    />

    <YamlCreateOverlay
      :visible="resourceCreateVisible"
      title="Create Resource"
      :yaml-text="resourceYamlText"
      :submitting="resourceCreateSubmitting"
      :valid="resourceYamlValid"
      :validation-message="resourceYamlValidationMessage"
      :on-close="closeResourceCreateOverlay"
      :on-update-yaml="updateResourceYamlText"
      :on-submit="submitResourceCreate"
    />

    <YamlCreateOverlay
      :visible="bottleCreateVisible"
      title="Create Bottle"
      :yaml-text="bottleYamlText"
      :submitting="bottleCreateSubmitting"
      :valid="bottleYamlValid"
      :validation-message="bottleYamlValidationMessage"
      :on-close="closeBottleCreateOverlay"
      :on-update-yaml="updateBottleYamlText"
      :on-submit="submitBottleCreate"
    />
  </div>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { Terminal } from 'xterm'
import { FitAddon } from 'xterm-addon-fit'
import 'xterm/css/xterm.css'
import DashboardPage from './pages/DashboardPage.vue'
import ContainerPage from './pages/ContainerPage.vue'
import ResourcePage from './pages/ResourcePage.vue'
import ImagePage from './pages/ImagePage.vue'
import BottlePage from './pages/BottlePage.vue'
import PolicyPage from './pages/PolicyPage.vue'
import AuditLogPage from './pages/AuditLogPage.vue'
import NetworkLogPage from './pages/NetworkLogPage.vue'
import LoginPage from './pages/LoginPage.vue'
import CreateContainerOverlay from './overlays/CreateContainerOverlay.vue'
import PullImageOverlay from './overlays/PullImageOverlay.vue'
import ImageDeleteOverlay from './overlays/ImageDeleteOverlay.vue'
import ActionConfirmOverlay from './overlays/ActionConfirmOverlay.vue'
import ContainerLogOverlay from './overlays/ContainerLogOverlay.vue'
import ContainerSpecOverlay from './overlays/ContainerSpecOverlay.vue'
import AttachOverlay from './overlays/AttachOverlay.vue'
import ExecOverlay from './overlays/ExecOverlay.vue'
import ContainerDetailOverlay from './overlays/ContainerDetailOverlay.vue'
import ResourceDetailOverlay from './overlays/ResourceDetailOverlay.vue'
import BottleDetailOverlay from './overlays/BottleDetailOverlay.vue'
import PolicyCreateOverlay from './overlays/PolicyCreateOverlay.vue'
import PolicyDeleteOverlay from './overlays/PolicyDeleteOverlay.vue'
import PolicyCommitOverlay from './overlays/PolicyCommitOverlay.vue'
import PolicyRevertOverlay from './overlays/PolicyRevertOverlay.vue'
import BottleActionConfirmOverlay from './overlays/BottleActionConfirmOverlay.vue'
import YamlCreateOverlay from './overlays/YamlCreateOverlay.vue'

const runtimeName = 'Raind'
const runtimeVersion = import.meta.env.VITE_RAIND_VERSION || 'dev'
const AUTH_MARKER_KEY = 'raind_webui_authenticated'
const LAST_MENU_KEY = 'raind_webui_last_menu'
const defaultMenu = 'dashboard'
const protectedMenus = new Set(['dashboard', 'container', 'resource', 'bottle', 'image', 'policy', 'audit', 'network'])

const currentMenu = ref(defaultMenu)
const isAuthenticated = ref(false)
const authSubmitting = ref(false)
const authError = ref('')
const currentUser = ref('')
const loginForm = ref({
  username: '',
  password: ''
})
const loading = ref(false)
const error = ref('')

const containers = ref([])
const replicasets = ref([])
const pods = ref([])
const services = ref([])
const bottles = ref([])
const images = ref([])
const policyChains = ['RAIND-EW', 'RAIND-NS-OBS', 'RAIND-NS-ENF']
const policyData = ref({})
const policyError = ref({})
const auditLogs = ref([])
const auditLoading = ref(false)
const auditPage = ref(1)
const auditPageSize = ref(50)
const auditTotal = ref(0)
const auditTotalPages = ref(0)
const auditParseErrors = ref(0)
const auditSource = ref('')
const auditActorOptions = ref([])
const auditFilter = ref({
  q: '',
  actor: '',
  severity: '',
  action: '',
  result_status: ''
})
const logInsights = ref({
  audit_total_24h: 0,
  audit_deny_24h: 0,
  traffic_total_24h: 0,
  traffic_deny_24h: 0,
  dns_total_24h: 0,
  dns_cache_hit_24h: 0,
  dns_cache_miss_24h: 0
})
const networkTrafficLoading = ref(false)
const networkDnsLoading = ref(false)
const networkPageSize = ref(50)
const networkTrafficFilter = ref({
  q: '',
  traffic_kind: '',
  verdict: '',
  proto: ''
})
const networkDnsFilter = ref({
  q: '',
  result: '',
  rcode: '',
  cache: ''
})
const networkTraffic = ref({
  kind: 'traffic',
  timezone_offset_minutes: 0,
  series_timezone_applied: false,
  window_hours: 24,
  page: 1,
  page_size: 50,
  total: 0,
  total_pages: 0,
  parse_errors: 0,
  source: '',
  summary: {},
  series: [],
  items: []
})
const networkDns = ref({
  kind: 'dns',
  timezone_offset_minutes: 0,
  series_timezone_applied: false,
  window_hours: 24,
  page: 1,
  page_size: 50,
  total: 0,
  total_pages: 0,
  parse_errors: 0,
  source: '',
  summary: {},
  series: [],
  items: []
})

const connectionStatus = ref('checking...')
const connectionOk = ref(false)

const detailVisible = ref(false)
const detailLoading = ref(false)
const detailTargetId = ref('')
const detailData = ref(null)
const logVisible = ref(false)
const logLoading = ref(false)
const logTargetId = ref('')
const logTargetName = ref('')
const logSourcePath = ref('')
const logData = ref('')
const specVisible = ref(false)
const specLoading = ref(false)
const specTargetId = ref('')
const specTargetName = ref('')
const specData = ref(null)
const attachVisible = ref(false)
const attachConnecting = ref(false)
const attachConnected = ref(false)
const attachError = ref('')
const attachTargetId = ref('')
const attachTargetName = ref('')
const attachTerminalRef = ref(null)
let attachSocket = null
const attachEncoder = new TextEncoder()
const attachDecoder = new TextDecoder()
const frameData = 0x00
const frameResize = 0x01
let attachResizeHandler = null
let attachTerm = null
let attachFit = null
let attachDisposeData = null
const execVisible = ref(false)
const execSubmitting = ref(false)
const execConnecting = ref(false)
const execConnected = ref(false)
const execError = ref('')
const execResult = ref('')
const execTargetId = ref('')
const execTargetName = ref('')
const execCommandText = ref('')
const execTTY = ref(true)
const execTerminalRef = ref(null)
let execSocket = null
let execResizeHandler = null
let execTerm = null
let execFit = null
let execDisposeData = null
let hashChangeHandler = null
const actionConfirmVisible = ref(false)
const actionConfirmSubmitting = ref(false)
const actionConfirmType = ref('')
const actionConfirmId = ref('')
const actionConfirmName = ref('')
const createContainerVisible = ref(false)
const createSubmitting = ref(false)
const imageFilter = ref('')
const imageDropdownOpen = ref(false)
const pullImageVisible = ref(false)
const pullImageSubmitting = ref(false)
const pullImageForm = ref({
  image: '',
  os: '',
  arch: ''
})
const imageDeleteVisible = ref(false)
const imageDeleteSubmitting = ref(false)
const imageDeleteTarget = ref('')
const policyCreateVisible = ref(false)
const policyCreateSubmitting = ref(false)
const policyCreateForm = ref({
  type: 'RAIND-EW',
  source: '',
  destinationContainer: '',
  destinationAddress: '',
  protocol: 'tcp',
  dport: '',
  comment: ''
})
const policyDeleteVisible = ref(false)
const policyDeleteSubmitting = ref(false)
const policyDeleteId = ref('')
const policyDeleteChain = ref('')
const policyCommitVisible = ref(false)
const policyCommitSubmitting = ref(false)
const policyRevertVisible = ref(false)
const policyRevertSubmitting = ref(false)
const bottleActionConfirmVisible = ref(false)
const bottleActionConfirmSubmitting = ref(false)
const bottleActionConfirmType = ref('')
const bottleActionConfirmId = ref('')
const bottleActionConfirmName = ref('')
const resourceCreateVisible = ref(false)
const resourceCreateSubmitting = ref(false)
const resourceYamlText = ref('')
const bottleCreateVisible = ref(false)
const bottleCreateSubmitting = ref(false)
const bottleYamlText = ref('')
const createForm = ref({
  name: '',
  image: '',
  tty: false,
  ports: [],
  mounts: [],
  envs: []
})
const expandedPodIds = ref({})
const resourcePanels = ref({
  replicaset: false,
  pod: false,
  service: false
})
const resourceDetailVisible = ref(false)
const resourceDetailLoading = ref(false)
const resourceDetailType = ref('')
const resourceDetailId = ref('')
const resourceDetailData = ref(null)
const bottleDetailVisible = ref(false)
const bottleDetailLoading = ref(false)
const bottleDetailId = ref('')
const bottleDetailData = ref(null)

const titleByMenu = {
  dashboard: 'Dashboard',
  container: 'Container',
  resource: 'Resource',
  bottle: 'Bottle',
  image: 'Image',
  policy: 'Policy',
  audit: 'Audit Log',
  network: 'Network Log'
}

function normalizeMenuName(input) {
  const menu = String(input || '').toLowerCase()
  return protectedMenus.has(menu) ? menu : defaultMenu
}

function getMenuFromHash() {
  if (typeof window === 'undefined') return defaultMenu
  const raw = String(window.location.hash || '').replace(/^#/, '')
  return normalizeMenuName(raw)
}

function updateHashMenu(menu) {
  if (typeof window === 'undefined') return
  const normalized = normalizeMenuName(menu)
  if (window.location.hash === `#${normalized}`) return
  window.history.replaceState(null, '', `#${normalized}`)
}

function rememberLastMenu(menu) {
  try {
    localStorage.setItem(LAST_MENU_KEY, normalizeMenuName(menu))
  } catch {
    // ignore storage errors
  }
}

function getLastMenu() {
  try {
    return normalizeMenuName(localStorage.getItem(LAST_MENU_KEY) || defaultMenu)
  } catch {
    return defaultMenu
  }
}

function setAuthMarker(enabled) {
  try {
    if (enabled) {
      localStorage.setItem(AUTH_MARKER_KEY, '1')
      return
    }
    localStorage.removeItem(AUTH_MARKER_KEY)
  } catch {
    // ignore storage errors
  }
}

function hasAuthMarker() {
  try {
    return localStorage.getItem(AUTH_MARKER_KEY) === '1'
  } catch {
    return false
  }
}

function selectMenu(menu) {
  const normalized = normalizeMenuName(menu)
  currentMenu.value = normalized
  updateHashMenu(normalized)
  rememberLastMenu(normalized)
}

function redirectToLogin() {
  currentMenu.value = 'login'
  if (typeof window !== 'undefined' && window.location.hash !== '#login') {
    window.history.replaceState(null, '', '#login')
  }
}

async function authApi(path, options = {}) {
  const res = await fetch(path, {
    headers: { 'Content-Type': 'application/json', ...(options.headers || {}) },
    credentials: 'same-origin',
    ...options
  })
  const text = await res.text()
  let body = {}
  try {
    body = text ? JSON.parse(text) : {}
  } catch {
    body = { status: 'fail', message: text }
  }
  if (!res.ok || body.status === 'fail') {
    throw new Error(body.message || `request failed: ${res.status}`)
  }
  return body
}

async function api(path, options = {}) {
  const res = await fetch(`/api${path}`, {
    headers: { 'Content-Type': 'application/json', ...(options.headers || {}) },
    credentials: 'same-origin',
    ...options
  })
  const text = await res.text()
  if (res.status === 401) {
    isAuthenticated.value = false
    currentUser.value = ''
    setAuthMarker(false)
    redirectToLogin()
    throw new Error('unauthorized')
  }

  let body = {}
  try {
    body = text ? JSON.parse(text) : {}
  } catch {
    body = { status: 'fail', message: text }
  }

  if (!res.ok || body.status === 'fail') {
    throw new Error(body.message || `request failed: ${res.status}`)
  }

  return body
}

async function checkAuth() {
  try {
    const res = await authApi('/auth/me')
    isAuthenticated.value = true
    currentUser.value = String(res?.data?.username || '')
    authError.value = ''
    setAuthMarker(true)
    const menu = getMenuFromHash()
    selectMenu(menu || getLastMenu())
  } catch {
    isAuthenticated.value = false
    currentUser.value = ''
    setAuthMarker(false)
    redirectToLogin()
  }
}

const loginReady = computed(
  () => loginForm.value.username.trim().length > 0 && loginForm.value.password.length > 0
)

async function submitLogin() {
  if (!loginReady.value || authSubmitting.value) return
  authSubmitting.value = true
  authError.value = ''
  try {
    const res = await authApi('/auth/login', {
      method: 'POST',
      body: JSON.stringify({
        username: loginForm.value.username.trim(),
        password: loginForm.value.password
      })
    })
    isAuthenticated.value = true
    currentUser.value = String(res?.data?.username || loginForm.value.username.trim())
    loginForm.value.password = ''
    setAuthMarker(true)
    selectMenu(getLastMenu())
    await refreshAll()
  } catch (e) {
    authError.value = e.message
    isAuthenticated.value = false
    setAuthMarker(false)
    redirectToLogin()
  } finally {
    authSubmitting.value = false
  }
}

async function logout() {
  try {
    await authApi('/auth/logout', { method: 'POST' })
  } catch {
    // ignore logout API errors
  }
  isAuthenticated.value = false
  currentUser.value = ''
  loginForm.value.password = ''
  setAuthMarker(false)
  redirectToLogin()
}

function normalizeContainer(raw = {}) {
  const repo = raw.imageRepository || raw.image_repository || raw.repository || ''
  const ref = raw.imageReference || raw.image_reference || raw.reference || ''
  const image = repo && ref ? `${repo}:${ref}` : repo || ref || '-'
  const status = String(raw.state || raw.status || '-').toLowerCase()
  const forwards = Array.isArray(raw.forwards) ? raw.forwards : []
  const ports = forwards
    .map((f) => {
      const host = f?.source ?? f?.hostPort
      const target = f?.destination ?? f?.containerPort
      const proto = String(f?.protocol || 'tcp').toLowerCase()
      if (host == null || target == null) return ''
      return `${host}->${target}/${proto}`
    })
    .filter(Boolean)
    .join(', ')

  return {
    id: raw.containerId || raw.id || '-',
    name: raw.name || raw.containerName || raw.container_name || '-',
    image,
    ports: ports || '-',
    status,
    podId: raw.podId || raw.pod_id || '',
    createdAt: raw.createdAt || raw.created_at || '',
    tty: Boolean(raw.tty)
  }
}

function normalizeResource(raw = {}, idKey) {
  return {
    id: raw[idKey] || raw.id || '-',
    name: raw.name || '-',
    namespace: raw.namespace || 'default',
    selector: raw.selector || {},
    labels: raw.labels || {},
    status: String(raw.state || raw.status || '-').toLowerCase(),
    desired: raw.desired ?? raw.replicas ?? 0,
    current: raw.current ?? 0,
    ready: raw.ready ?? 0,
    ports: Array.isArray(raw.ports) ? raw.ports : []
  }
}

function imageDisplay(img = {}) {
  const repo = img.repository || ''
  const ref = img.reference || ''
  if (!repo && !ref) return '-'
  return repo && ref ? `${repo}:${ref}` : repo || ref
}

function imageNameShort(img = {}) {
  const repo = String(img.repository || '')
  if (!repo) return '-'
  return repo.split('/').filter(Boolean).pop() || repo
}

function policyChainLabel(chain) {
  const labels = {
    'RAIND-EW': 'Inter Container Policy',
    'RAIND-NS-OBS': 'External Policy (Observe)',
    'RAIND-NS-ENF': 'External Policy (Enforce)'
  }
  return labels[chain] || chain
}

function policyModeLabel(mode) {
  const value = String(mode || '').trim().toLowerCase()
  if (!value) return '-'
  if (value === 'observe') return 'Observe'
  if (value === 'enforce') return 'Enforce'
  if (value === 'mixed') return 'Mixed'
  return value.replace(/_/g, ' ')
}

function resetPolicyCreateForm() {
  policyCreateForm.value = {
    type: 'RAIND-EW',
    source: '',
    destinationContainer: '',
    destinationAddress: '',
    protocol: 'tcp',
    dport: '',
    comment: ''
  }
}

function updatePolicyCreateForm(patch) {
  policyCreateForm.value = {
    ...policyCreateForm.value,
    ...(patch || {})
  }
}

function openPolicyCreateOverlay() {
  error.value = ''
  resetPolicyCreateForm()
  policyCreateVisible.value = true
}

function closePolicyCreateOverlay() {
  policyCreateVisible.value = false
  policyCreateSubmitting.value = false
  resetPolicyCreateForm()
}

function openPolicyDeleteOverlay(id, chain) {
  policyDeleteId.value = String(id || '')
  policyDeleteChain.value = String(chain || '')
  policyDeleteVisible.value = true
}

function closePolicyDeleteOverlay() {
  policyDeleteVisible.value = false
  policyDeleteSubmitting.value = false
  policyDeleteId.value = ''
  policyDeleteChain.value = ''
}

function openPolicyCommitOverlay() {
  policyCommitVisible.value = true
}

function closePolicyCommitOverlay() {
  policyCommitVisible.value = false
  policyCommitSubmitting.value = false
}

function openPolicyRevertOverlay() {
  policyRevertVisible.value = true
}

function closePolicyRevertOverlay() {
  policyRevertVisible.value = false
  policyRevertSubmitting.value = false
}

function selectorMatches(selector = {}, labels = {}) {
  const keys = Object.keys(selector || {})
  if (keys.length === 0) return false
  return keys.every((k) => labels?.[k] === selector[k])
}

function selectorText(selector = {}) {
  const entries = Object.entries(selector || {})
  if (entries.length === 0) return '-'
  return entries.map(([k, v]) => `${k}=${v}`).join(', ')
}

async function checkHealth() {
  try {
    const res = await fetch('/api/healthz')
    connectionOk.value = res.ok
    connectionStatus.value = res.ok ? 'connected' : `error (${res.status})`
  } catch (e) {
    connectionOk.value = false
    connectionStatus.value = `error (${e.message})`
  }
}

async function loadContainers() {
  const [res, statsRes] = await Promise.all([api('/v1/containers'), api('/v1/containers/stats')])
  const list = Array.isArray(res.data) ? res.data : []
  const statsList = Array.isArray(statsRes.data) ? statsRes.data : []
  const ttyByContainerId = new Map(
    statsList.map((s) => [String(s.container_id || s.containerId || ''), Boolean(s.tty)])
  )

  containers.value = list
    .map((c) => normalizeContainer({ ...c, tty: ttyByContainerId.get(String(c.containerId || c.id || '')) ?? c.tty }))
    .sort((a, b) => toEpochMs(b.createdAt) - toEpochMs(a.createdAt))
}

async function loadImages() {
  const res = await api('/v1/images')
  const list = Array.isArray(res.data) ? res.data : []
  images.value = list
}

async function loadBottles() {
  const res = await api('/v1/bottle')
  const list = Array.isArray(res.data?.bottles) ? res.data.bottles : []

  const details = await Promise.all(
    list.map(async (b) => {
      try {
        const detailRes = await api(`/v1/bottle/${encodeURIComponent(b.bottleId)}`)
        const bottle = detailRes.data?.bottle || {}
        const containersMap = bottle.containers || {}
        const servicesMap = bottle.services || {}
        const containers = Object.entries(containersMap).map(([serviceName, c]) => ({
          serviceName,
          id: c.containerId || '-',
          name: c.name || serviceName || '-',
          state: String(c.state || '-').toLowerCase(),
          // bottle container state does not include tty; derive from service spec
          tty: Boolean(servicesMap?.[serviceName]?.tty),
          image: c.imageRepository && c.imageReference ? `${c.imageRepository}:${c.imageReference}` : '-'
        }))
        return { containers }
      } catch {
        return { containers: [] }
      }
    })
  )

  bottles.value = list.map((b, idx) => ({
    id: b.bottleId || '-',
    name: b.bottleName || '-',
    serviceCount: Number(b.serviceCount || 0),
    status: String(b.status || '-').toLowerCase(),
    containers: details[idx]?.containers || []
  }))
}

function validateYamlText(text) {
  const src = String(text || '').replace(/\t/g, '  ').trim()
  if (!src) return { valid: false, message: 'YAML is required' }

  const lines = src
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter((line) => line && !line.startsWith('#'))

  if (lines.length === 0) return { valid: false, message: 'YAML is required' }

  // Minimal syntax check:
  // - document marker, sequence item, or key:value style line
  const hasInvalid = lines.some((line) => {
    if (line === '---' || line === '...') return false
    if (line.startsWith('- ')) return false
    if (line.includes(':')) return false
    return true
  })

  if (hasInvalid) {
    return { valid: false, message: 'Invalid YAML format (minimum check failed)' }
  }
  return { valid: true, message: '' }
}

const resourceYamlValidation = computed(() => validateYamlText(resourceYamlText.value))
const resourceYamlValid = computed(() => resourceYamlValidation.value.valid)
const resourceYamlValidationMessage = computed(() => resourceYamlValidation.value.message)

const bottleYamlValidation = computed(() => validateYamlText(bottleYamlText.value))
const bottleYamlValid = computed(() => bottleYamlValidation.value.valid)
const bottleYamlValidationMessage = computed(() => bottleYamlValidation.value.message)

async function loadPolicies() {
  const results = await Promise.allSettled(policyChains.map((chain) => api(`/v1/policies/${chain}`)))
  const nextData = {}
  const nextErr = {}

  for (let i = 0; i < policyChains.length; i += 1) {
    const chain = policyChains[i]
    const r = results[i]
    if (r.status === 'fulfilled') {
      nextData[chain] = r.value?.data || { mode: '-', policies_total: 0, policies: [] }
      continue
    }
    nextData[chain] = { mode: '-', policies_total: 0, policies: [] }
    nextErr[chain] = r.reason?.message || 'failed to load'
  }

  policyData.value = nextData
  policyError.value = nextErr
}

async function loadAuditLogs(page = auditPage.value) {
  auditLoading.value = true
  const nextPage = Number.parseInt(String(page || 1), 10)
  const safePage = Number.isFinite(nextPage) && nextPage > 0 ? nextPage : 1

  try {
    const params = new URLSearchParams({
      page: String(safePage),
      page_size: String(auditPageSize.value)
    })
    for (const [k, v] of Object.entries(auditFilter.value || {})) {
      if (String(v || '').trim()) params.set(k, String(v).trim())
    }
    const res = await api(`/audit/logs?${params.toString()}`)
    const data = res.data || {}
    auditLogs.value = Array.isArray(data.items) ? data.items : []
    auditPage.value = Number(data.page || safePage) || 1
    auditTotal.value = Number(data.total || 0) || 0
    auditTotalPages.value = Number(data.total_pages || 0) || 0
    auditParseErrors.value = Number(data.parse_errors || 0) || 0
    auditSource.value = String(data.source || '')
  } finally {
    auditLoading.value = false
  }
}

async function loadAuditActorOptions() {
  const res = await api('/audit/actors?hours=24')
  const data = res.data || {}
  auditActorOptions.value = Array.isArray(data.items) ? data.items : []
}

function setAuditFilter(filters = {}) {
  auditFilter.value = {
    q: String(filters.q || ''),
    actor: String(filters.actor || ''),
    severity: String(filters.severity || ''),
    action: String(filters.action || ''),
    result_status: String(filters.result_status || '')
  }
}

async function loadLogInsights() {
  const [auditRes, trafficRes, dnsRes] = await Promise.all([
    api('/audit/summary?hours=24'),
    api('/network/logs?kind=traffic&page=1&page_size=1&hours=24'),
    api('/network/logs?kind=dns&page=1&page_size=1&hours=24')
  ])

  const audit = auditRes?.data || {}
  const traffic = trafficRes?.data || {}
  const dns = dnsRes?.data || {}
  logInsights.value = {
    audit_total_24h: Number(audit.total || 0) || 0,
    audit_deny_24h: Number(audit.deny || 0) || 0,
    traffic_total_24h: Number(traffic.summary?.total || traffic.total || 0) || 0,
    traffic_deny_24h: Number(traffic.summary?.deny || 0) || 0,
    dns_total_24h: Number(dns.summary?.total || dns.total || 0) || 0,
    dns_cache_hit_24h: Number(dns.summary?.cache?.hit || 0) || 0,
    dns_cache_miss_24h: Number(dns.summary?.cache?.miss || 0) || 0
  }
}

async function loadNetworkLog(kind, page = 1) {
  const normalizedKind = kind === 'dns' ? 'dns' : 'traffic'
  const nextPage = Number.parseInt(String(page || 1), 10)
  const safePage = Number.isFinite(nextPage) && nextPage > 0 ? nextPage : 1
  if (normalizedKind === 'dns') {
    networkDnsLoading.value = true
  } else {
    networkTrafficLoading.value = true
  }

  try {
    const filters = normalizedKind === 'dns' ? networkDnsFilter.value : networkTrafficFilter.value
    const params = new URLSearchParams({
      kind: normalizedKind,
      page: String(safePage),
      page_size: String(networkPageSize.value),
      hours: '24'
    })
    for (const [k, v] of Object.entries(filters || {})) {
      if (String(v || '').trim()) params.set(k, String(v).trim())
    }
    const res = await api(`/network/logs?${params.toString()}`)
    const data = res.data || {}
    const normalized = {
      kind: data.kind || normalizedKind,
      timezone_offset_minutes: Number(data.timezone_offset_minutes || 0) || 0,
      series_timezone_applied: Boolean(data.series_timezone_applied),
      window_hours: Number(data.window_hours || 24) || 24,
      page: Number(data.page || safePage) || 1,
      page_size: Number(data.page_size || networkPageSize.value) || networkPageSize.value,
      total: Number(data.total || 0) || 0,
      total_pages: Number(data.total_pages || 0) || 0,
      parse_errors: Number(data.parse_errors || 0) || 0,
      source: String(data.source || ''),
      summary: data.summary || {},
      series: Array.isArray(data.series) ? data.series : [],
      items: Array.isArray(data.items) ? data.items : []
    }

    if (normalizedKind === 'dns') {
      networkDns.value = normalized
    } else {
      networkTraffic.value = normalized
    }
  } finally {
    if (normalizedKind === 'dns') {
      networkDnsLoading.value = false
    } else {
      networkTrafficLoading.value = false
    }
  }
}

function loadNetworkTraffic(page = networkTraffic.value.page || 1) {
  return loadNetworkLog('traffic', page)
}

function loadNetworkDns(page = networkDns.value.page || 1) {
  return loadNetworkLog('dns', page)
}

function setNetworkTrafficFilter(filters = {}) {
  networkTrafficFilter.value = {
    q: String(filters.q || ''),
    traffic_kind: String(filters.traffic_kind || ''),
    verdict: String(filters.verdict || ''),
    proto: String(filters.proto || '')
  }
}

function setNetworkDnsFilter(filters = {}) {
  networkDnsFilter.value = {
    q: String(filters.q || ''),
    result: String(filters.result || ''),
    rcode: String(filters.rcode || ''),
    cache: String(filters.cache || '')
  }
}

async function loadNetworkLogs() {
  await Promise.all([loadNetworkTraffic(), loadNetworkDns()])
}

async function submitPolicyCreate() {
  if (policyCreateSubmitting.value) return
  const form = policyCreateForm.value
  const chain = String(form.type || '')
  const source = String(form.source || '').trim()
  const protocol = String(form.protocol || 'tcp').toLowerCase()
  const isInterConnect = chain === 'RAIND-EW'
  const destination = isInterConnect
    ? String(form.destinationContainer || '').trim()
    : String(form.destinationAddress || '').trim()
  const needsDport = protocol !== 'icmp'
  const parsedDport = needsDport ? Number.parseInt(String(form.dport || ''), 10) : 0

  if (!chain) {
    error.value = 'policy type is required'
    return
  }
  if (!source) {
    error.value = 'source container is required'
    return
  }
  if (!destination) {
    error.value = isInterConnect ? 'destination container is required' : 'destination IP address is required'
    return
  }
  if (!['tcp', 'udp', 'icmp'].includes(protocol)) {
    error.value = 'invalid protocol'
    return
  }
  if (needsDport && (!Number.isFinite(parsedDport) || parsedDport < 1 || parsedDport > 65535)) {
    error.value = 'destination port must be between 1 and 65535'
    return
  }

  policyCreateSubmitting.value = true
  error.value = ''
  try {
    await api('/v1/policies', {
      method: 'POST',
      body: JSON.stringify({
        chain,
        source,
        dest: destination,
        protocol,
        dport: needsDport ? parsedDport : 0,
        comment: String(form.comment || '').trim()
      })
    })
    closePolicyCreateOverlay()
    await loadPolicies()
  } catch (e) {
    error.value = e.message
  } finally {
    policyCreateSubmitting.value = false
  }
}

async function submitPolicyDelete() {
  if (policyDeleteSubmitting.value || !policyDeleteId.value) return
  policyDeleteSubmitting.value = true
  error.value = ''
  try {
    await api(`/v1/policies/${policyDeleteId.value}`, { method: 'DELETE' })
    closePolicyDeleteOverlay()
    await loadPolicies()
  } catch (e) {
    error.value = e.message
  } finally {
    policyDeleteSubmitting.value = false
  }
}

async function submitPolicyCommit() {
  if (policyCommitSubmitting.value) return
  policyCommitSubmitting.value = true
  error.value = ''
  try {
    await api('/v1/policies/commit', { method: 'POST' })
    closePolicyCommitOverlay()
    await loadPolicies()
  } catch (e) {
    error.value = e.message
  } finally {
    policyCommitSubmitting.value = false
  }
}

async function submitPolicyRevert() {
  if (policyRevertSubmitting.value) return
  policyRevertSubmitting.value = true
  error.value = ''
  try {
    await api('/v1/policies/revert', { method: 'POST' })
    closePolicyRevertOverlay()
    await loadPolicies()
  } catch (e) {
    error.value = e.message
  } finally {
    policyRevertSubmitting.value = false
  }
}

async function loadResources() {
  const [rsRes, podRes, svcRes] = await Promise.all([
    api('/v1/replicasets'),
    api('/v1/pods'),
    api('/v1/services')
  ])

  const rsList = Array.isArray(rsRes.data) ? rsRes.data : []
  const podList = Array.isArray(podRes.data) ? podRes.data : []
  const svcList = Array.isArray(svcRes.data) ? svcRes.data : []

  const serviceDetails = await Promise.all(
    svcList.map(async (s) => {
      try {
        const detail = await api(`/v1/services/${s.serviceId}`)
        return detail.data || {}
      } catch {
        return {}
      }
    })
  )

  replicasets.value = rsList.map((r) => normalizeResource(r, 'replicaSetId'))
  pods.value = podList.map((p) => normalizeResource(p, 'podId'))
  services.value = svcList.map((s, idx) =>
    normalizeResource(
      {
        ...s,
        selector: serviceDetails[idx]?.selector || {}
      },
      'serviceId'
    )
  )

  // Default state: containers are hidden until user expands each pod.
  expandedPodIds.value = {}
}

const resourceRelations = computed(() => {
  return replicasets.value.map((rs) => {
    const matchedPods = pods.value.filter(
      (p) => p.namespace === rs.namespace && selectorMatches(rs.selector, p.labels)
    )

    const matchedServices = services.value.filter((s) => {
      if (s.namespace !== rs.namespace) return false
      if (selectorMatches(s.selector, rs.selector)) return true
      return matchedPods.some((p) => selectorMatches(s.selector, p.labels))
    })

    return {
      replicaset: rs,
      pods: matchedPods,
      services: matchedServices
    }
  })
})

const statusCounts = computed(() => {
  const counts = {
    creating: 0,
    created: 0,
    running: 0,
    stopped: 0
  }

  for (const c of containers.value) {
    const s = String(c.status || '').toLowerCase()
    if (s in counts) counts[s] += 1
  }

  return counts
})

const totalContainers = computed(() => containers.value.length)

const imageOptions = computed(() => {
  const list = images.value
    .map((img) => {
      const repo = img?.repository || ''
      const ref = img?.reference || ''
      if (!repo) return ''
      return ref ? `${repo}:${ref}` : repo
    })
    .filter(Boolean)
  return [...new Set(list)]
})

const containerNameOptions = computed(() => {
  const list = containers.value.map((c) => String(c.name || '').trim()).filter(Boolean)
  return [...new Set(list)].sort((a, b) => a.localeCompare(b, undefined, { sensitivity: 'base' }))
})

const filteredImageOptions = computed(() => {
  const q = imageFilter.value.toLowerCase()
  if (!q) return imageOptions.value
  return imageOptions.value.filter((img) => img.toLowerCase().includes(q))
})

const sortedImages = computed(() => {
  return [...images.value].sort((a, b) =>
    imageNameShort(a).localeCompare(imageNameShort(b), undefined, { sensitivity: 'base' })
  )
})

const nsModeSummary = computed(() => {
  const obs = String(policyData.value['RAIND-NS-OBS']?.mode || '').trim().toLowerCase()
  const enf = String(policyData.value['RAIND-NS-ENF']?.mode || '').trim().toLowerCase()
  if (!obs && !enf) return ''
  if (obs && enf && obs !== enf) return 'mixed'
  return obs || enf
})

const execCommandReady = computed(() => execCommandText.value.trim().length > 0)

let imageDropdownCloseTimer = null

const donutStyle = computed(() => {
  const total = totalContainers.value
  if (total === 0) {
    return {
      background: 'conic-gradient(#2c3240 0deg 360deg)'
    }
  }

  const creatingDeg = (statusCounts.value.creating / total) * 360
  const createdDeg = (statusCounts.value.created / total) * 360
  const runningDeg = (statusCounts.value.running / total) * 360
  const stoppedDeg = (statusCounts.value.stopped / total) * 360

  const p1 = creatingDeg
  const p2 = p1 + createdDeg
  const p3 = p2 + runningDeg
  const p4 = p3 + stoppedDeg

  return {
    background: `conic-gradient(
      #ffbe4d 0deg ${p1}deg,
      #6ebeff ${p1}deg ${p2}deg,
      #56da8d ${p2}deg ${p3}deg,
      #ff7d8a ${p3}deg ${p4}deg
    )`
  }
})

const rsPulseBlocks = computed(() => {
  return replicasets.value.map((rs) => {
    const desired = Number(rs.desired ?? 0)
    const ready = Number(rs.ready ?? 0)
    if (ready === 0) return 'stopped'
    if (desired === ready) return 'running'
    if (desired > ready && ready > 0) return 'degraded'
    return 'degraded'
  })
})

const podPulseBlocks = computed(() => {
  return pods.value.map((p) => {
    const list = podContainers(p.id)
    if (list.length === 0) return 'stopped'
    const running = list.filter((c) => String(c.status || '').toLowerCase() === 'running').length
    if (running === list.length) return 'running'
    if (running > 0) return 'degraded'
    return 'stopped'
  })
})

const bottlePulseBlocks = computed(() =>
  bottles.value.map((b) => {
    const status = String(b.status || 'unknown').toLowerCase()
    if (status === 'creating') return 'creating'
    if (status === 'created') return 'created'
    if (status === 'running') return 'running'
    if (status === 'stopped') return 'stopped'
    return 'degraded'
  })
)

const containerPulseBlocks = computed(() =>
  containers.value.map((c) => {
    const status = String(c.status || 'unknown').toLowerCase()
    if (status === 'creating') return 'creating'
    if (status === 'created') return 'created'
    if (status === 'running') return 'running'
    if (status === 'stopped') return 'stopped'
    return 'degraded'
  })
)

const totalPulseBlocks = computed(
  () =>
    rsPulseBlocks.value.length +
    podPulseBlocks.value.length +
    bottlePulseBlocks.value.length +
    containerPulseBlocks.value.length
)

const unhealthyReplicaSets = computed(() =>
  replicasets.value.filter((rs) => Number(rs.desired ?? 0) !== Number(rs.ready ?? 0)).length
)

const nonRunningContainers = computed(() =>
  containers.value.filter((c) => String(c.status || '').toLowerCase() !== 'running').length
)

const pendingPolicyChanges = computed(() => {
  const chains = Object.values(policyData.value || {})
  let count = 0
  for (const chain of chains) {
    const list = Array.isArray(chain?.policies) ? chain.policies : []
    count += list.filter((p) => String(p?.status || '').toLowerCase() !== 'applied').length
  }
  return count
})

const connectionStatusClass = computed(() => {
  const s = String(connectionStatus.value || '').toLowerCase()
  if (s === 'connected') return 'status-chip running'
  if (s.startsWith('checking')) return 'status-chip creating'
  return 'status-chip stopped'
})

async function refreshAll() {
  if (!isAuthenticated.value) return
  loading.value = true
  error.value = ''
  try {
    await checkHealth()
    await Promise.all([
      loadContainers(),
      loadResources(),
      loadBottles(),
      loadImages(),
      loadPolicies(),
      loadLogInsights(),
      currentMenu.value === 'audit'
        ? Promise.all([loadAuditActorOptions(), loadAuditLogs(auditPage.value)])
        : Promise.resolve(),
      currentMenu.value === 'network' ? loadNetworkLogs() : Promise.resolve()
    ])
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

async function containerAction(id, action) {
  error.value = ''
  try {
    const options = { method: 'POST' }
    if (action === 'start') {
      options.body = JSON.stringify({ tty: false })
    }
    await api(`/v1/containers/${id}/actions/${action}`, options)
    await refreshAll()
  } catch (e) {
    error.value = e.message
  }
}

async function deleteContainer(id) {
  error.value = ''
  try {
    await api(`/v1/containers/${id}/actions/delete`, { method: 'DELETE' })
    await refreshAll()
  } catch (e) {
    error.value = e.message
  }
}

async function bottleAction(id, action) {
  error.value = ''
  try {
    await api(`/v1/bottle/${encodeURIComponent(id)}/actions/${action}`, { method: 'POST' })
    await loadBottles()
  } catch (e) {
    error.value = e.message
  }
}

function openResourceCreateOverlay() {
  resourceYamlText.value = ''
  resourceCreateVisible.value = true
}

function closeResourceCreateOverlay() {
  resourceCreateVisible.value = false
  resourceCreateSubmitting.value = false
  resourceYamlText.value = ''
}

function updateResourceYamlText(text) {
  resourceYamlText.value = String(text || '')
}

async function submitResourceCreate() {
  if (resourceCreateSubmitting.value || !resourceYamlValid.value) return
  resourceCreateSubmitting.value = true
  error.value = ''
  try {
    const body = new TextEncoder().encode(resourceYamlText.value)
    await api('/v1/resource/apply', {
      method: 'POST',
      headers: { 'Content-Type': 'application/octet-stream' },
      body
    })
    closeResourceCreateOverlay()
    await loadResources()
    await loadContainers()
  } catch (e) {
    error.value = e.message
  } finally {
    resourceCreateSubmitting.value = false
  }
}

function openBottleCreateOverlay() {
  bottleYamlText.value = ''
  bottleCreateVisible.value = true
}

function closeBottleCreateOverlay() {
  bottleCreateVisible.value = false
  bottleCreateSubmitting.value = false
  bottleYamlText.value = ''
}

function updateBottleYamlText(text) {
  bottleYamlText.value = String(text || '')
}

async function submitBottleCreate() {
  if (bottleCreateSubmitting.value || !bottleYamlValid.value) return
  bottleCreateSubmitting.value = true
  error.value = ''
  try {
    const body = new TextEncoder().encode(bottleYamlText.value)
    await api('/v1/bottle', {
      method: 'POST',
      headers: { 'Content-Type': 'application/octet-stream' },
      body
    })
    closeBottleCreateOverlay()
    await loadBottles()
    await loadContainers()
  } catch (e) {
    error.value = e.message
  } finally {
    bottleCreateSubmitting.value = false
  }
}

function canStartContainer(status) {
  const s = String(status || '').toLowerCase()
  return s === 'created' || s === 'stopped'
}

function canAttachContainer(container) {
  const id = String(container?.id || '')
  const matched = id ? containers.value.find((c) => String(c.id || '') === id) : null
  const status = String(container?.status || container?.state || matched?.status || '').toLowerCase()
  const ttyRaw = container?.tty ?? matched?.tty ?? false
  const tty = ttyRaw === true || String(ttyRaw).toLowerCase() === 'true'
  return tty && status === 'running'
}

function canExecContainer(status) {
  return String(status || '').toLowerCase() === 'running'
}

function replicasetHealthy(desired, ready) {
  return Number(desired ?? 0) === Number(ready ?? 0)
}

function replicasetHealthLabel(desired, ready) {
  return replicasetHealthy(desired, ready) ? 'healthy' : 'unhealthy'
}

function replicasetHealthClass(desired, ready) {
  return replicasetHealthy(desired, ready) ? 'status-chip healthy' : 'status-chip unhealthy'
}

function canStopContainer(status) {
  return String(status || '').toLowerCase() === 'running'
}

function canDeleteContainer(status) {
  const s = String(status || '').toLowerCase()
  return s === 'created' || s === 'stopped'
}

function openBottleActionConfirm(type, id, name) {
  bottleActionConfirmType.value = type
  bottleActionConfirmId.value = id
  bottleActionConfirmName.value = name || ''
  bottleActionConfirmVisible.value = true
}

function closeBottleActionConfirm() {
  bottleActionConfirmVisible.value = false
  bottleActionConfirmSubmitting.value = false
  bottleActionConfirmType.value = ''
  bottleActionConfirmId.value = ''
  bottleActionConfirmName.value = ''
}

async function submitBottleActionConfirm() {
  if (bottleActionConfirmSubmitting.value) return
  if (!bottleActionConfirmId.value || !bottleActionConfirmType.value) return
  bottleActionConfirmSubmitting.value = true
  error.value = ''
  try {
    await bottleAction(bottleActionConfirmId.value, bottleActionConfirmType.value)
    closeBottleActionConfirm()
  } catch (e) {
    error.value = e.message
  } finally {
    bottleActionConfirmSubmitting.value = false
  }
}

function openActionConfirm(type, id, name) {
  actionConfirmType.value = type
  actionConfirmId.value = id
  actionConfirmName.value = name || ''
  actionConfirmVisible.value = true
}

function closeActionConfirm() {
  actionConfirmVisible.value = false
  actionConfirmSubmitting.value = false
  actionConfirmType.value = ''
  actionConfirmId.value = ''
  actionConfirmName.value = ''
}

async function submitActionConfirm() {
  if (actionConfirmSubmitting.value) return
  if (!actionConfirmId.value || !actionConfirmType.value) return
  error.value = ''
  actionConfirmSubmitting.value = true
  try {
    if (actionConfirmType.value === 'stop') {
      await api(`/v1/containers/${actionConfirmId.value}/actions/stop`, { method: 'POST' })
    } else if (actionConfirmType.value === 'delete') {
      await api(`/v1/containers/${actionConfirmId.value}/actions/delete`, { method: 'DELETE' })
    } else {
      throw new Error(`unsupported action: ${actionConfirmType.value}`)
    }
    closeActionConfirm()
    await refreshAll()
  } catch (e) {
    error.value = e.message
  } finally {
    actionConfirmSubmitting.value = false
  }
}

async function openContainerDetail(id) {
  detailVisible.value = true
  detailLoading.value = true
  detailTargetId.value = id
  detailData.value = null
  error.value = ''

  try {
    const res = await api(`/v1/containers/${id}/stats`)
    detailData.value = res.data ?? res
  } catch (e) {
    detailData.value = { error: e.message }
  } finally {
    detailLoading.value = false
  }
}

async function openPolicyContainerDetailByName(containerName) {
  const name = String(containerName || '').trim()
  if (!name) {
    error.value = 'container name is missing'
    return
  }

  let container =
    containers.value.find((c) => String(c.name || '').trim() === name) ||
    containers.value.find((c) => String(c.name || '').trim().toLowerCase() === name.toLowerCase())

  if (!container?.id) {
    error.value = `container not found: ${name}`
    return
  }

  await openContainerDetail(container.id)
}

async function openNetworkContainerDetailByRef(containerId, containerName) {
  const id = String(containerId || '').trim()
  if (id) {
    await openContainerDetail(id)
    return
  }

  const name = String(containerName || '').trim()
  if (!name) {
    error.value = 'container id/name is missing'
    return
  }

  let container =
    containers.value.find((c) => String(c.name || '').trim() === name) ||
    containers.value.find((c) => String(c.name || '').trim().toLowerCase() === name.toLowerCase())

  if (!container?.id) {
    error.value = `container not found: ${name}`
    return
  }

  await openContainerDetail(container.id)
}

function openPodDetailFromContainerDetail(podId) {
  const id = String(podId || '').trim()
  if (!id || id === '-') return
  openResourceDetail('pod', id)
}

async function openBottleDetail(id) {
  const bottleId = String(id || '').trim()
  if (!bottleId || bottleId === '-') return
  bottleDetailVisible.value = true
  bottleDetailLoading.value = true
  bottleDetailId.value = bottleId
  bottleDetailData.value = null
  try {
    const res = await api(`/v1/bottle/${encodeURIComponent(bottleId)}`)
    bottleDetailData.value = res.data?.bottle ?? res.data ?? res
  } catch (e) {
    bottleDetailData.value = { error: e.message }
  } finally {
    bottleDetailLoading.value = false
  }
}

function closeBottleDetail() {
  bottleDetailVisible.value = false
  bottleDetailLoading.value = false
  bottleDetailId.value = ''
  bottleDetailData.value = null
}

function closeContainerDetail() {
  detailVisible.value = false
  detailLoading.value = false
  detailTargetId.value = ''
  detailData.value = null
}

async function openContainerLog(id, name) {
  logVisible.value = true
  logLoading.value = true
  logTargetId.value = id
  logTargetName.value = name || ''
  logSourcePath.value = ''
  logData.value = ''
  error.value = ''

  const [logRes, logPathRes] = await Promise.allSettled([
    fetch(`/api/v1/containers/${id}/log?tail_lines=300`),
    api(`/v1/containers/${id}/logpath`)
  ])

  if (logPathRes.status === 'fulfilled') {
    logSourcePath.value = String(logPathRes.value?.data || '-')
  } else {
    logSourcePath.value = '-'
  }

  try {
    if (logRes.status !== 'fulfilled') {
      throw logRes.reason || new Error('failed to fetch log')
    }
    const res = logRes.value
    const text = await res.text()
    if (!res.ok) {
      throw new Error(text || `request failed: ${res.status}`)
    }
    logData.value = text
  } catch (e) {
    logData.value = ''
    error.value = e.message
  } finally {
    logLoading.value = false
  }
}

function closeContainerLog() {
  logVisible.value = false
  logLoading.value = false
  logTargetId.value = ''
  logTargetName.value = ''
  logSourcePath.value = ''
  logData.value = ''
}

async function openContainerSpec(id, name) {
  specVisible.value = true
  specLoading.value = true
  specTargetId.value = id
  specTargetName.value = name || ''
  specData.value = null
  error.value = ''
  try {
    const res = await api(`/v1/containers/${id}/spec`)
    specData.value = res.data ?? {}
  } catch (e) {
    specData.value = null
    error.value = e.message
  } finally {
    specLoading.value = false
  }
}

function closeContainerSpec() {
  specVisible.value = false
  specLoading.value = false
  specTargetId.value = ''
  specTargetName.value = ''
  specData.value = null
}

function setAttachTerminalEl(el) {
  attachTerminalRef.value = el
}

function focusAttachTerminal() {
  attachTerm?.focus()
}

function initAttachTerminal() {
  const el = attachTerminalRef.value
  if (!el) return

  disposeAttachTerminal()
  attachFit = new FitAddon()
  attachTerm = new Terminal({
    cursorBlink: true,
    cursorStyle: 'underline',
    fontFamily: 'JetBrains Mono, monospace',
    fontSize: 13,
    lineHeight: 1.2,
    theme: {
      background: '#0f1217',
      foreground: '#d9e8ff',
      cursor: '#9fd0ff'
    }
  })
  attachTerm.loadAddon(attachFit)
  attachTerm.open(el)
  attachFit.fit()
  attachTerm.focus()
  attachDisposeData = attachTerm.onData((data) => {
    sendAttachFrame(frameData, attachEncoder.encode(data))
  })
}

function disposeAttachTerminal() {
  if (attachDisposeData) {
    attachDisposeData.dispose()
    attachDisposeData = null
  }
  if (attachTerm) {
    attachTerm.dispose()
    attachTerm = null
  }
  attachFit = null
}

function closeAttachSocket() {
  if (!attachSocket) return
  try {
    attachSocket.close()
  } catch {
    // ignore close errors
  }
  attachSocket = null
}

async function closeAttachOverlay() {
  if (attachResizeHandler) {
    window.removeEventListener('resize', attachResizeHandler)
    attachResizeHandler = null
  }
  closeAttachSocket()
  disposeAttachTerminal()
  attachVisible.value = false
  attachConnecting.value = false
  attachConnected.value = false
  attachError.value = ''
  attachTargetId.value = ''
  attachTargetName.value = ''
  try {
    await loadContainers()
  } catch (e) {
    error.value = e.message
  }
}

function buildAttachFrame(frameType, payload) {
  const p = payload instanceof Uint8Array ? payload : new Uint8Array(0)
  const out = new Uint8Array(1 + 4 + p.length)
  out[0] = frameType
  const dv = new DataView(out.buffer)
  dv.setUint32(1, p.length, false)
  if (p.length > 0) out.set(p, 5)
  return out
}

function sendAttachFrame(frameType, payload) {
  if (!attachSocket || attachSocket.readyState !== WebSocket.OPEN) return
  attachSocket.send(buildAttachFrame(frameType, payload))
}

function sendAttachResizeFrame() {
  if (attachFit) attachFit.fit()
  const cols = attachTerm?.cols || 80
  const rows = attachTerm?.rows || 24
  const payload = new Uint8Array(4)
  const dv = new DataView(payload.buffer)
  dv.setUint16(0, rows, false)
  dv.setUint16(2, cols, false)
  sendAttachFrame(frameResize, payload)
}

async function openAttachOverlay(container) {
  if (!canAttachContainer(container)) return
  closeAttachSocket()

  attachVisible.value = true
  attachConnecting.value = true
  attachConnected.value = false
  attachError.value = ''
  attachTargetId.value = container.id
  attachTargetName.value = container.name || ''
  await nextTick()
  initAttachTerminal()

  const wsProto = window.location.protocol === 'https:' ? 'wss' : 'ws'
  const wsUrl = `${wsProto}://${window.location.host}/api/v1/containers/${container.id}/attach`
  const sock = new WebSocket(wsUrl)
  sock.binaryType = 'arraybuffer'
  attachSocket = sock

  sock.onopen = () => {
    attachConnecting.value = false
    attachConnected.value = true
    focusAttachTerminal()
    sendAttachResizeFrame()
    attachResizeHandler = () => sendAttachResizeFrame()
    window.addEventListener('resize', attachResizeHandler)
  }
  sock.onclose = () => {
    if (attachResizeHandler) {
      window.removeEventListener('resize', attachResizeHandler)
      attachResizeHandler = null
    }
    attachConnecting.value = false
    attachConnected.value = false
    if (!attachError.value) attachError.value = 'connection closed'
  }
  sock.onerror = () => {
    if (attachResizeHandler) {
      window.removeEventListener('resize', attachResizeHandler)
      attachResizeHandler = null
    }
    attachConnecting.value = false
    attachConnected.value = false
    attachError.value = 'websocket error'
  }
  sock.onmessage = async (ev) => {
    if (typeof ev.data === 'string') {
      attachTerm?.write(ev.data)
      return
    }
    if (ev.data instanceof Blob) {
      attachTerm?.write(attachDecoder.decode(await ev.data.arrayBuffer(), { stream: true }))
      return
    }
    if (ev.data instanceof ArrayBuffer) {
      attachTerm?.write(attachDecoder.decode(ev.data, { stream: true }))
    }
  }
}

function parseExecCommand(text) {
  const src = String(text || '')
  const out = []
  let cur = ''
  let quote = ''
  let esc = false

  for (let i = 0; i < src.length; i += 1) {
    const ch = src[i]

    if (esc) {
      cur += ch
      esc = false
      continue
    }

    if (ch === '\\') {
      if (quote === "'") {
        cur += ch
      } else {
        esc = true
      }
      continue
    }

    if (quote) {
      if (ch === quote) {
        quote = ''
      } else {
        cur += ch
      }
      continue
    }

    if (ch === '"' || ch === "'" || ch === '`') {
      quote = ch
      continue
    }

    if (/\s/.test(ch)) {
      if (cur.length > 0) {
        out.push(cur)
        cur = ''
      }
      continue
    }

    cur += ch
  }

  if (esc) {
    cur += '\\'
  }
  if (quote) {
    return { args: [], error: `unclosed quote: ${quote}` }
  }
  if (cur.length > 0) out.push(cur)
  return { args: out, error: '' }
}

function focusExecTerminal() {
  execTerm?.focus()
}

function disposeExecTerminal() {
  if (execDisposeData) {
    execDisposeData.dispose()
    execDisposeData = null
  }
  if (execTerm) {
    execTerm.dispose()
    execTerm = null
  }
  execFit = null
}

function initExecTerminal() {
  const el = execTerminalRef.value
  if (!el) return
  disposeExecTerminal()
  execFit = new FitAddon()
  execTerm = new Terminal({
    cursorBlink: true,
    cursorStyle: 'underline',
    fontFamily: 'JetBrains Mono, monospace',
    fontSize: 13,
    lineHeight: 1.2,
    theme: {
      background: '#0f1217',
      foreground: '#d9e8ff',
      cursor: '#9fd0ff'
    }
  })
  execTerm.loadAddon(execFit)
  execTerm.open(el)
  execFit.fit()
  execTerm.focus()
  execDisposeData = execTerm.onData((data) => {
    sendExecFrame(frameData, attachEncoder.encode(data))
  })
}

function closeExecSocket() {
  if (!execSocket) return
  try {
    execSocket.close()
  } catch {
    // ignore close errors
  }
  execSocket = null
}

function sendExecFrame(frameType, payload) {
  if (!execSocket || execSocket.readyState !== WebSocket.OPEN) return
  execSocket.send(buildAttachFrame(frameType, payload))
}

function sendExecResizeFrame() {
  if (execFit) execFit.fit()
  const cols = execTerm?.cols || 80
  const rows = execTerm?.rows || 24
  const payload = new Uint8Array(4)
  const dv = new DataView(payload.buffer)
  dv.setUint16(0, rows, false)
  dv.setUint16(2, cols, false)
  sendExecFrame(frameResize, payload)
}

async function connectExecSocket(containerId) {
  const wsProto = window.location.protocol === 'https:' ? 'wss' : 'ws'
  const wsUrl = `${wsProto}://${window.location.host}/api/v1/containers/${containerId}/exec/attach`
  const sock = new WebSocket(wsUrl)
  sock.binaryType = 'arraybuffer'
  execSocket = sock

  sock.onopen = () => {
    execConnecting.value = false
    execConnected.value = true
    focusExecTerminal()
    sendExecResizeFrame()
    execResizeHandler = () => sendExecResizeFrame()
    window.addEventListener('resize', execResizeHandler)
  }
  sock.onclose = () => {
    if (execResizeHandler) {
      window.removeEventListener('resize', execResizeHandler)
      execResizeHandler = null
    }
    execConnecting.value = false
    execConnected.value = false
    if (!execError.value) execError.value = 'connection closed'
  }
  sock.onerror = () => {
    if (execResizeHandler) {
      window.removeEventListener('resize', execResizeHandler)
      execResizeHandler = null
    }
    execConnecting.value = false
    execConnected.value = false
    execError.value = 'websocket error'
  }
  sock.onmessage = async (ev) => {
    if (typeof ev.data === 'string') {
      execTerm?.write(ev.data)
      return
    }
    if (ev.data instanceof Blob) {
      execTerm?.write(attachDecoder.decode(await ev.data.arrayBuffer(), { stream: true }))
      return
    }
    if (ev.data instanceof ArrayBuffer) {
      execTerm?.write(attachDecoder.decode(ev.data, { stream: true }))
    }
  }
}

function openExecOverlay(container) {
  closeExecOverlay()
  execVisible.value = true
  execError.value = ''
  execResult.value = ''
  execTargetId.value = container.id
  execTargetName.value = container.name || ''
  execCommandText.value = ''
  execTTY.value = true
}

function setExecTerminalEl(el) {
  execTerminalRef.value = el
}

function setExecCommandText(value) {
  execCommandText.value = String(value || '')
}

function setExecTTYValue(value) {
  if (typeof value === 'boolean') {
    execTTY.value = value
    return
  }
  const normalized = String(value ?? '')
    .trim()
    .toLowerCase()
  if (normalized === 'true' || normalized === '1' || normalized === 'yes' || normalized === 'on') {
    execTTY.value = true
    return
  }
  if (normalized === 'false' || normalized === '0' || normalized === 'no' || normalized === 'off') {
    execTTY.value = false
    return
  }
  execTTY.value = Boolean(value)
}

async function closeExecOverlay() {
  if (execResizeHandler) {
    window.removeEventListener('resize', execResizeHandler)
    execResizeHandler = null
  }
  closeExecSocket()
  disposeExecTerminal()
  execVisible.value = false
  execSubmitting.value = false
  execConnecting.value = false
  execConnected.value = false
  execError.value = ''
  execResult.value = ''
  execTargetId.value = ''
  execTargetName.value = ''
  execCommandText.value = ''
  execTTY.value = true
  try {
    await loadContainers()
  } catch (e) {
    error.value = e.message
  }
}

async function submitExec() {
  if (!execCommandReady.value || !execTargetId.value) return
  const parsed = parseExecCommand(execCommandText.value)
  if (parsed.error) {
    execError.value = parsed.error
    return
  }
  const command = parsed.args
  if (command.length === 0) {
    execError.value = 'command is empty'
    return
  }
  const useTTY = execTTY.value === true

  execSubmitting.value = true
  execError.value = ''
  execResult.value = ''
  try {
    await api(`/v1/containers/${execTargetId.value}/actions/exec`, {
      method: 'POST',
      body: JSON.stringify({
        command,
        tty: useTTY
      })
    })

    if (!useTTY) {
      execResult.value = 'succeeded'
      return
    }

    execConnecting.value = true
    execConnected.value = false
    await nextTick()
    initExecTerminal()
    await connectExecSocket(execTargetId.value)
  } catch (e) {
    execError.value = e.message
  } finally {
    execSubmitting.value = false
  }
}

async function openResourceDetail(type, id) {
  resourceDetailVisible.value = true
  resourceDetailLoading.value = true
  resourceDetailType.value = type
  resourceDetailId.value = id
  resourceDetailData.value = null

  const endpointByType = {
    replicaset: `/v1/replicasets/${id}`,
    pod: `/v1/pods/${id}`,
    service: `/v1/services/${id}`
  }

  try {
    const endpoint = endpointByType[type]
    if (!endpoint) {
      throw new Error(`unsupported resource type: ${type}`)
    }
    const res = await api(endpoint)
    resourceDetailData.value = res.data ?? res
  } catch (e) {
    resourceDetailData.value = { error: e.message }
  } finally {
    resourceDetailLoading.value = false
  }
}

function closeResourceDetail() {
  resourceDetailVisible.value = false
  resourceDetailLoading.value = false
  resourceDetailType.value = ''
  resourceDetailId.value = ''
  resourceDetailData.value = null
}

function formatTime(v) {
  if (!v) return '-'
  const d = new Date(v)
  return Number.isNaN(d.getTime()) ? String(v) : d.toLocaleString()
}

function toEpochMs(v) {
  if (!v) return 0
  const t = new Date(v).getTime()
  return Number.isNaN(t) ? 0 : t
}

function formatPercent(v) {
  if (v == null || Number.isNaN(Number(v))) return '-'
  return `${Number(v).toFixed(2)}%`
}

function formatBytes(v) {
  if (v == null) return '-'
  const n = Number(v)
  if (!Number.isFinite(n) || n < 0) return '-'
  if (n === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.min(Math.floor(Math.log(n) / Math.log(1024)), units.length - 1)
  const val = n / 1024 ** i
  return `${val.toFixed(i === 0 ? 0 : 2)} ${units[i]}`
}

function formatImage(data) {
  const repo = data?.image_repository || '-'
  const ref = data?.image_reference || '-'
  if (repo === '-' && ref === '-') return '-'
  return `${repo}:${ref}`
}

function formatCommand(cmd) {
  if (!Array.isArray(cmd) || cmd.length === 0) return '-'
  return cmd.join(' ')
}

function resourceFieldRaw(key) {
  return resourceDetailData.value?.[key]
}

function resourceField(key) {
  const v = resourceFieldRaw(key)
  if (v == null || v === '') return '-'
  return v
}

function objectText(v) {
  if (!v || typeof v !== 'object' || Array.isArray(v)) return '-'
  const entries = Object.entries(v)
  if (entries.length === 0) return '-'
  return entries.map(([k, val]) => `${k}=${val}`).join(', ')
}

function servicePortsText(ports) {
  if (!Array.isArray(ports) || ports.length === 0) return '-'
  return ports
    .map((p) => `${p.port ?? '?'}->${p.targetPort ?? '?'}${p.protocol ? `/${p.protocol}` : ''}`)
    .join(', ')
}

function statusClass(status) {
  const s = String(status || '').toLowerCase()
  if (s === 'running') return 'status-chip running'
  if (s === 'creating') return 'status-chip creating'
  if (s === 'created') return 'status-chip created'
  if (s === 'stopped') return 'status-chip stopped'
  return 'status-chip unknown'
}

function isRunningStatus(data) {
  return String(data?.status || '').toLowerCase() === 'running'
}

function runtimeExitCode(data) {
  if (isRunningStatus(data)) return 0
  return data?.exit_code ?? '-'
}

function runtimeReason(data) {
  if (isRunningStatus(data)) return '-'
  return data?.reason || '-'
}

function runtimeMessage(data) {
  if (isRunningStatus(data)) return '-'
  return data?.message || '-'
}

function podContainers(podId) {
  if (!podId) return []
  return containers.value.filter((c) => c.podId === podId)
}

function isPodExpanded(podId) {
  return !!expandedPodIds.value[podId]
}

function togglePodExpand(podId) {
  expandedPodIds.value = {
    ...expandedPodIds.value,
    [podId]: !expandedPodIds.value[podId]
  }
}

function resetCreateForm() {
  imageFilter.value = ''
  imageDropdownOpen.value = false
  createForm.value = {
    name: '',
    image: '',
    tty: false,
    ports: [],
    mounts: [],
    envs: []
  }
}

function resetPullImageForm() {
  pullImageForm.value = {
    image: '',
    os: '',
    arch: ''
  }
}

function openPullImageOverlay() {
  error.value = ''
  pullImageVisible.value = true
}

function closePullImageOverlay() {
  pullImageVisible.value = false
  pullImageSubmitting.value = false
  resetPullImageForm()
}

async function submitPullImage() {
  if (!pullImageForm.value.image) return
  error.value = ''
  pullImageSubmitting.value = true
  try {
    await api('/v1/images', {
      method: 'POST',
      body: JSON.stringify({
        image: pullImageForm.value.image,
        os: pullImageForm.value.os || '',
        arch: pullImageForm.value.arch || ''
      })
    })
    closePullImageOverlay()
    await loadImages()
  } catch (e) {
    error.value = e.message
  } finally {
    pullImageSubmitting.value = false
  }
}

function normalizeImageRef(img = {}) {
  const repo = String(img.repository || '').trim()
  const ref = String(img.reference || '').trim()
  if (!repo) return ''
  return ref ? `${repo}:${ref}` : repo
}

function stripLibraryPrefix(imageRef) {
  if (!imageRef) return ''
  const [repo, ref] = String(imageRef).split(':', 2)
  const sanitizedRepo = repo.includes('library/') ? repo.replace(/library\//g, '') : repo
  return ref ? `${sanitizedRepo}:${ref}` : sanitizedRepo
}

function openImageDeleteOverlay(image) {
  imageDeleteTarget.value = typeof image === 'string' ? image : normalizeImageRef(image)
  imageDeleteVisible.value = true
}

function closeImageDeleteOverlay() {
  imageDeleteVisible.value = false
  imageDeleteSubmitting.value = false
  imageDeleteTarget.value = ''
}

async function submitImageDelete() {
  if (!imageDeleteTarget.value) return
  if (imageDeleteTarget.value === '-') {
    error.value = 'invalid image reference'
    return
  }
  const requestImage = stripLibraryPrefix(imageDeleteTarget.value)
  error.value = ''
  imageDeleteSubmitting.value = true
  try {
    await api(`/v1/images?image=${encodeURIComponent(requestImage)}`, {
      method: 'DELETE',
      body: JSON.stringify({
        image: requestImage
      })
    })
    closeImageDeleteOverlay()
    await loadImages()
  } catch (e) {
    error.value = e.message
  } finally {
    imageDeleteSubmitting.value = false
  }
}

function openImageDropdown() {
  if (imageDropdownCloseTimer) {
    clearTimeout(imageDropdownCloseTimer)
    imageDropdownCloseTimer = null
  }
  imageDropdownOpen.value = true
}

function toggleImageDropdown() {
  if (imageDropdownOpen.value) {
    imageDropdownOpen.value = false
    return
  }
  openImageDropdown()
}

function scheduleCloseImageDropdown() {
  if (imageDropdownCloseTimer) {
    clearTimeout(imageDropdownCloseTimer)
  }
  imageDropdownCloseTimer = setTimeout(() => {
    imageDropdownOpen.value = false
    imageDropdownCloseTimer = null
  }, 120)
}

function onImageFilterInput() {
  createForm.value.image = ''
  openImageDropdown()
}

function onImageFilterInputValue(value) {
  imageFilter.value = String(value || '')
  onImageFilterInput()
}

function selectImageOption(image) {
  createForm.value.image = image
  imageFilter.value = image
  imageDropdownOpen.value = false
}

async function openCreateContainerOverlay() {
  error.value = ''
  createContainerVisible.value = true
  if (images.value.length === 0) {
    try {
      await loadImages()
    } catch (e) {
      error.value = e.message
    }
  }
}

function closeCreateContainerOverlay() {
  createContainerVisible.value = false
  createSubmitting.value = false
  resetCreateForm()
}

function addPortRow() {
  createForm.value.ports.push({ host: '', target: '', protocol: 'tcp' })
}

function removePortRow(index) {
  createForm.value.ports.splice(index, 1)
}

function addMountRow() {
  createForm.value.mounts.push({ host: '', target: '' })
}

function removeMountRow(index) {
  createForm.value.mounts.splice(index, 1)
}

function addEnvRow() {
  createForm.value.envs.push({ key: '', value: '' })
}

function removeEnvRow(index) {
  createForm.value.envs.splice(index, 1)
}

async function submitCreateContainer() {
  error.value = ''
  if (!createForm.value.image && imageFilter.value) {
    const exact = imageOptions.value.find((img) => img === imageFilter.value)
    if (exact) createForm.value.image = exact
  }
  if (!createForm.value.image) {
    error.value = 'image is required'
    return
  }

  const port = createForm.value.ports
    .filter((p) => p.host && p.target)
    .map((p) => `${p.host}:${p.target}:${p.protocol || 'tcp'}`)
  const mount = createForm.value.mounts.filter((m) => m.host && m.target).map((m) => `${m.host}:${m.target}`)
  const env = createForm.value.envs.filter((e) => e.key).map((e) => `${e.key}=${e.value || ''}`)

  createSubmitting.value = true
  try {
    await api('/v1/containers', {
      method: 'POST',
      body: JSON.stringify({
        name: createForm.value.name || '',
        image: createForm.value.image,
        port,
        mount,
        env,
        tty: !!createForm.value.tty
      })
    })
    closeCreateContainerOverlay()
    await refreshAll()
  } catch (e) {
    error.value = e.message
  } finally {
    createSubmitting.value = false
  }
}

function pulseStyle(idx) {
  return {
    animationDelay: `${idx * 55}ms`
  }
}

function handlePulseHover(event) {
  const el = event.currentTarget
  if (!(el instanceof HTMLElement)) return

  const useJump = Math.random() < 0.22
  const keyframes = useJump
    ? [
        { transform: 'translateY(0) scale(1)' },
        { transform: 'translateY(-14px) scale(1.08)' },
        { transform: 'translateY(2px) scale(0.97)' },
        { transform: 'translateY(0) scale(1)' }
      ]
    : [
        { transform: 'translate(0, 0) rotate(0deg)' },
        { transform: 'translate(-1px, 0) rotate(-3deg)' },
        { transform: 'translate(1px, 0) rotate(3deg)' },
        { transform: 'translate(-1px, 0) rotate(-2deg)' },
        { transform: 'translate(0, 0) rotate(0deg)' }
      ]

  const duration = useJump ? 560 : 420
  // Keep initial drop animation untouched; hover effect is applied as a separate temporary animation.
  el.animate(keyframes, {
    duration,
    easing: useJump ? 'cubic-bezier(0.2, 0.85, 0.3, 1)' : 'ease-in-out',
    iterations: 1
  })
}

function toggleResourcePanel(type) {
  if (!(type in resourcePanels.value)) return
  resourcePanels.value = {
    ...resourcePanels.value,
    [type]: !resourcePanels.value[type]
  }
}

watch(currentMenu, async (menu) => {
  if (!isAuthenticated.value) {
    if (menu !== 'login') {
      redirectToLogin()
    }
    return
  }

  if (menu === 'audit') {
    if (auditLoading.value) return
    if (auditLogs.value.length > 0) return
    try {
      await Promise.all([loadAuditActorOptions(), loadAuditLogs(1)])
    } catch (e) {
      error.value = e.message
    }
    return
  }

  if (menu === 'network') {
    if (networkTrafficLoading.value || networkDnsLoading.value) return
    if (networkTraffic.value.items.length > 0 || networkDns.value.items.length > 0) return
    try {
      await loadNetworkLogs()
    } catch (e) {
      error.value = e.message
    }
  }
})

onMounted(async () => {
  const initialMenu = getMenuFromHash()
  rememberLastMenu(initialMenu)
  if (!hasAuthMarker()) {
    redirectToLogin()
  }

  hashChangeHandler = () => {
    if (!isAuthenticated.value) {
      redirectToLogin()
      return
    }
    selectMenu(getMenuFromHash())
  }
  window.addEventListener('hashchange', hashChangeHandler)

  await checkAuth()
  if (isAuthenticated.value) {
    await refreshAll()
  }
})

onBeforeUnmount(() => {
  if (hashChangeHandler) {
    window.removeEventListener('hashchange', hashChangeHandler)
    hashChangeHandler = null
  }
  closeAttachOverlay()
  closeExecOverlay()
})
</script>

<style>
html,
body,
#app {
  margin: 0;
  min-height: 100%;
  background: #1e2025;
  scrollbar-color: #3a4150 #1e2025;
  scrollbar-width: thin;
}

*::-webkit-scrollbar {
  width: 10px;
  height: 10px;
}

*::-webkit-scrollbar-track {
  background: #1e2025;
}

*::-webkit-scrollbar-thumb {
  background: #3a4150;
  border: 2px solid #1e2025;
  border-radius: 10px;
}

*::-webkit-scrollbar-thumb:hover {
  background: #1789ff;
}

* {
  box-sizing: border-box;
}

.layout {
  --bg-1: #1e2025;
  --bg-2: #242730;
  --bg-3: #2d313d;
  --line: #3a4150;
  --text-1: #f2f5fb;
  --text-2: #adb6c9;
  --primary: #1789ff;
  --primary-2: #0f6fd3;
  --danger: #e34a4a;
  min-height: 100vh;
  height: 100vh;
  display: grid;
  grid-template-columns: 260px 1fr;
  background: var(--bg-1);
  color: var(--text-1);
  font-family: 'Space Grotesk', 'Noto Sans JP', sans-serif;
  overflow: hidden;
}

.sidebar {
  border-right: 1px solid var(--line);
  background: var(--bg-2);
  padding: 20px 16px 14px;
  display: flex;
  flex-direction: column;
  gap: 14px;
  height: 100vh;
  overflow: auto;
}

.brand {
  display: grid;
  justify-items: center;
  text-align: center;
  gap: 3px;
}

.brand h1 {
  margin: 0;
  font-size: 24px;
}

.brand-icon {
  width: 100px;
  height: 100px;
  object-fit: contain;
  display: block;
  margin-bottom: 8px;
}

.brand p {
  margin: 6px 0 0;
  color: var(--text-2);
}

.menu {
  display: grid;
  gap: 10px;
  flex: 1;
  align-content: start;
}

.menu-group {
  display: grid;
  gap: 6px;
}

.menu-group h4 {
  margin: 0;
  font-size: 11px;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--text-2);
}

.menu button {
  border: 1px solid var(--line);
  background: var(--bg-3);
  color: var(--text-1);
  border-radius: 10px;
  padding: 10px 12px;
  text-align: left;
  cursor: pointer;
}

.menu button.active {
  border-color: var(--primary);
  box-shadow: inset 0 0 0 1px var(--primary);
  background: #1c2d45;
}

.sidebar-footer {
  border-top: 1px solid var(--line);
  padding-top: 10px;
  color: var(--text-2);
  font-size: 12px;
  text-align: center;
}

.sidebar-footer a {
  color: #9fd0ff;
}

.sidebar-footer a:hover {
  color: #cfe8ff;
}

.content {
  padding: 22px;
  height: 100vh;
  overflow: auto;
  min-height: 0;
}

.topbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}

.topbar h2 {
  margin: 0;
}

.primary {
  border: 1px solid var(--primary);
  background: var(--primary);
  color: #fff;
  border-radius: 10px;
  padding: 8px 12px;
  cursor: pointer;
}

.primary:disabled {
  opacity: 0.6;
  cursor: wait;
}

.primary-outline {
  border: 1px solid var(--primary);
  background: #203750;
  color: #cfe8ff;
}

.panel-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
}

.dashboard-grid {
  display: grid;
  grid-template-columns: repeat(12, minmax(0, 1fr));
  gap: 12px;
}

.dashboard-overview {
  grid-column: span 4;
}

.dashboard-stats {
  grid-column: span 4;
}

.dashboard-donut {
  grid-column: span 4;
}

.dashboard-pulse {
  grid-column: span 12;
}

.dashboard-alerts {
  grid-column: span 6;
}

.dashboard-log-insights {
  grid-column: span 6;
}

.overview-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10px;
}

.overview-item {
  border: 1px solid var(--line);
  border-radius: 10px;
  background: #1f232b;
  padding: 10px;
  display: grid;
  gap: 6px;
}

.overview-label {
  font-size: 12px;
  color: var(--text-2);
}

.overview-item strong {
  font-size: 20px;
  overflow-wrap: anywhere;
}

.overview-item strong.ok {
  color: #56da8d;
}

.overview-item strong.ng {
  color: #ff9ea7;
}

.overview-item strong.checking {
  color: #ffd17c;
}

.stats-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10px;
}

.policy-mode-grid {
  grid-template-columns: repeat(2, minmax(180px, 220px));
  justify-content: start;
}

.stat-card {
  border: 1px solid var(--line);
  border-radius: 10px;
  background: #1f232b;
  padding: 10px;
  display: grid;
  gap: 6px;
}

.stat-card span {
  color: var(--text-2);
  font-size: 12px;
}

.stat-card strong {
  font-size: 24px;
  color: #cfe8ff;
}

.policy-mode-card strong {
  font-size: 16px;
}

.donut-wrap {
  display: flex;
  gap: 14px;
  align-items: center;
}

.status-donut {
  width: 140px;
  height: 140px;
  border-radius: 50%;
  position: relative;
  border: 1px solid #3a4150;
}

.status-donut::before {
  content: '';
  position: absolute;
  inset: 22px;
  border-radius: 50%;
  background: #1f232b;
  border: 1px solid #3a4150;
}

.status-donut span {
  position: absolute;
  inset: 0;
  display: grid;
  place-items: center;
  z-index: 1;
  font-family: 'JetBrains Mono', monospace;
  font-size: 26px;
  color: #e7f2ff;
}

.donut-legend {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  gap: 8px;
}

.dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  display: inline-block;
  margin-right: 8px;
}

.dot.creating {
  background: #ffbe4d;
}

.dot.created {
  background: #6ebeff;
}

.dot.running {
  background: #56da8d;
}

.dot.stopped {
  background: #ff7d8a;
}

.pulse-sections {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10px;
}

.pulse-section {
  display: grid;
  gap: 8px;
}

.pulse-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  color: #cfe8ff;
  font-size: 13px;
}

.pulse-stage {
  min-height: 190px;
  max-height: 190px;
  overflow: auto;
  border: 1px solid #334059;
  border-radius: 10px;
  background:
    linear-gradient(180deg, #1b2029, #171b22),
    repeating-linear-gradient(
      90deg,
      transparent 0 17px,
      rgba(23, 137, 255, 0.08) 17px 18px
    );
  padding: 12px;
  display: flex;
  flex-wrap: wrap;
  align-content: flex-end;
  gap: 6px;
}

.pulse-block {
  width: 16px;
  height: 16px;
  border-radius: 3px;
  opacity: 0;
  transform: translateY(-90px);
  animation: pulse-drop 380ms ease-out forwards;
  will-change: transform;
}

.pulse-creating {
  background: #ffbe4d;
  box-shadow: 0 0 8px rgba(255, 190, 77, 0.35);
}

.pulse-created {
  background: #6ebeff;
  box-shadow: 0 0 8px rgba(110, 190, 255, 0.35);
}

.pulse-running {
  background: #56da8d;
  box-shadow: 0 0 8px rgba(86, 218, 141, 0.35);
}

.pulse-degraded {
  background: #ffbe4d;
  box-shadow: 0 0 8px rgba(255, 190, 77, 0.35);
}

.pulse-stopped,
.pulse-unknown {
  background: #ff7d8a;
  box-shadow: 0 0 8px rgba(255, 125, 138, 0.35);
}

.pulse-empty {
  width: 100%;
  align-self: center;
}

@keyframes pulse-drop {
  0% {
    transform: translateY(-90px);
    opacity: 0;
  }
  70% {
    transform: translateY(3px);
    opacity: 1;
  }
  100% {
    transform: translateY(0);
    opacity: 1;
  }
}

.resource-grid {
  display: grid;
  grid-template-columns: 1fr;
  gap: 12px;
}

.relation-panel {
  padding-bottom: 10px;
}

.relation-list {
  display: grid;
  gap: 10px;
}

.relation-scroller {
  max-height: calc(100vh - 280px);
  overflow: auto;
  padding-right: 2px;
}

.relation-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto minmax(0, 1fr) auto minmax(0, 1fr);
  gap: 10px;
  align-items: start;
  border: 1px solid var(--line);
  border-radius: 10px;
  padding: 10px;
  background: #1f232b;
}

.relation-block {
  border: 1px solid #334059;
  border-radius: 8px;
  padding: 8px;
  display: grid;
  gap: 4px;
  background: #1d2736;
}

.relation-title {
  margin: 0;
  color: #8fc4ff;
  font-size: 12px;
}

.relation-arrow {
  width: 0;
  height: 0;
  border-top: 12px solid transparent;
  border-bottom: 12px solid transparent;
  border-left: 18px solid #1789ff;
  align-self: center;
  filter: drop-shadow(0 0 4px rgba(23, 137, 255, 0.35));
}

.relation-items {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  gap: 6px;
  max-height: 180px;
  overflow: auto;
}

.relation-items li {
  border: 1px solid #304057;
  border-radius: 7px;
  padding: 6px;
  display: grid;
  gap: 2px;
}

.selector {
  color: var(--text-2);
  font-size: 11px;
}

.bottle-containers {
  display: grid;
  gap: 6px;
}

.bottle-container-item {
  border: 1px solid #304057;
  border-radius: 7px;
  background: #1b2b3d;
  padding: 6px;
  display: grid;
  gap: 3px;
}

.policy-link {
  color: #9fd0ff;
  text-decoration: underline;
  text-underline-offset: 2px;
  cursor: pointer;
}

.policy-link:hover {
  color: #cfe8ff;
}

.policy-link:focus-visible {
  outline: 1px solid #1789ff;
  outline-offset: 2px;
  border-radius: 3px;
}

.policy-default-row td {
  background: #1b2b3d;
  border-top: 1px solid #335071;
}

.policy-default-row strong {
  color: #9fd0ff;
  margin-right: 0.5ch;
}

.policy-default-row span {
  color: var(--text-2);
}

.policy-default-row.policy-default-denied td {
  background: #3a1f25;
  border-top: 1px solid #7a3640;
}

.policy-default-row.policy-default-denied strong {
  color: #ff9ea7;
}

.policy-default-row.policy-default-denied span {
  color: #ffd0d6;
}

.pod-container-cards {
  display: grid;
  gap: 6px;
}

.pod-container-card {
  border: 1px solid #335071;
  border-radius: 7px;
  background: #1b2b3d;
  padding: 6px;
  display: grid;
  gap: 3px;
}

.tiny-btn {
  border: 1px solid #3f5f84;
  background: #20334a;
  color: #cde7ff;
  border-radius: 7px;
  font-size: 11px;
  padding: 4px 8px;
  width: fit-content;
}

.tiny-btn:hover {
  border-color: #1789ff;
  background: #28486e;
}

.relation-empty {
  padding: 8px;
  border: 1px dashed var(--line);
  border-radius: 8px;
}

.panel {
  background: var(--bg-2);
  border: none;
  border-radius: 12px;
  padding: 14px;
}

.panel h3 {
  margin: 0 0 6px;
  color: var(--text-2);
  font-size: 14px;
  font-weight: 500;
}

.value {
  margin: 0;
  font-size: 24px;
  font-weight: 600;
}

.value.ok {
  color: #4ecf8d;
}

.value.ng {
  color: var(--danger);
}

.section-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 10px;
}

.section-head-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.container-head-left {
  display: flex;
  align-items: center;
  gap: 10px;
}

.container-head-left h3 {
  margin: 0;
}

.section-toggle {
  width: 100%;
  margin-bottom: 10px;
  border: 1px solid var(--line);
  background: #232a35;
  border-radius: 10px;
  padding: 8px 10px;
  cursor: pointer;
}

.section-toggle h3 {
  margin: 0;
}

.count {
  color: var(--text-2);
  font-family: 'JetBrains Mono', monospace;
}

.table-scroller {
  max-height: calc(100vh - 250px);
  overflow: auto;
  border: none;
  border-radius: 10px;
}

table {
  width: 100%;
  border-collapse: collapse;
}

th,
td {
  text-align: left;
  padding: 10px;
  border-bottom: 1px solid var(--line);
  font-size: 13px;
}

th {
  position: sticky;
  top: 0;
  background: #20242c;
  z-index: 1;
}

.actions {
  display: flex;
  gap: 6px;
}

button {
  border: 1px solid var(--line);
  background: #232a35;
  color: var(--text-1);
  border-radius: 8px;
  padding: 5px 9px;
  cursor: pointer;
}

button:hover {
  border-color: var(--primary);
}

button:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

button.danger {
  border-color: #8d3a45;
  color: #ff9ea7;
  background: #311f26;
}

button.success {
  border-color: #2f6b48;
  color: #9ef0be;
  background: #1f3527;
}

button.caution {
  border-color: #7b6134;
  color: #ffd17c;
  background: #3a3020;
}

.status-chip {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  border: 1px solid #3a4150;
  border-radius: 999px;
  padding: 3px 10px;
  font-size: 12px;
  text-transform: capitalize;
  width: fit-content;
  justify-self: start;
}

.status-lamp {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #9aa4b7;
  box-shadow: 0 0 0 3px #ffffff0f;
}

.status-chip.creating {
  color: #ffc766;
  border-color: #7b6134;
  background: #3a3020;
}

.status-chip.creating .status-lamp {
  background: #ffbe4d;
}

.status-chip.created {
  color: #8bc9ff;
  border-color: #385f80;
  background: #202f3e;
}

.status-chip.created .status-lamp {
  background: #6ebeff;
}

.status-chip.running {
  color: #80e1a7;
  border-color: #2f6b48;
  background: #1f3527;
}

.status-chip.running .status-lamp {
  background: #56da8d;
}

.status-chip.stopped {
  color: #ff9ea7;
  border-color: #7a3640;
  background: #3a1f25;
}

.status-chip.stopped .status-lamp {
  background: #ff7d8a;
}

.status-chip.unknown {
  color: #c9d3e7;
  border-color: #58627a;
  background: #2a2f3a;
}

.status-chip.unknown .status-lamp {
  background: #b4bfd4;
}

.status-chip.policy-applied {
  color: #80e1a7;
  border-color: #2f6b48;
  background: #1f3527;
}

.status-chip.policy-applied .status-lamp {
  background: #56da8d;
}

.status-chip.policy-remove-next-commit {
  color: #ffd17c;
  border-color: #7b6134;
  background: #3a3020;
}

.status-chip.policy-remove-next-commit .status-lamp {
  background: #ffbe4d;
}

.status-chip.policy-before-commit {
  color: #8bc9ff;
  border-color: #385f80;
  background: #202f3e;
}

.status-chip.policy-before-commit .status-lamp {
  background: #6ebeff;
}

.status-chip.healthy {
  color: #80e1a7;
  border-color: #2f6b48;
  background: #1f3527;
}

.status-chip.healthy .status-lamp {
  background: #56da8d;
}

.status-chip.unhealthy {
  color: #ff9ea7;
  border-color: #7a3640;
  background: #3a1f25;
}

.status-chip.unhealthy .status-lamp {
  background: #ff7d8a;
}

.list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  gap: 8px;
  max-height: 340px;
  overflow: auto;
}

.list li {
  border: 1px solid var(--line);
  border-radius: 8px;
  padding: 8px;
  display: grid;
  gap: 3px;
}

.mono {
  font-family: 'JetBrains Mono', monospace;
  color: var(--text-2);
}

.empty {
  text-align: center;
  color: var(--text-2);
}

.error {
  margin-top: 12px;
  color: #ff8f8f;
}

.overlay {
  position: fixed;
  inset: 0;
  background: rgba(12, 13, 16, 0.78);
  display: grid;
  place-items: center;
  padding: 20px;
  z-index: 1000;
}

.detail-modal {
  width: min(960px, 96vw);
  max-height: 86vh;
  background: #242730;
  border: none;
  border-radius: 12px;
  padding: 14px;
  display: grid;
  gap: 12px;
}

.create-modal {
  width: min(980px, 96vw);
}

.pull-image-modal {
  width: min(760px, 94vw);
}

.confirm-modal {
  width: min(560px, 94vw);
}

.log-modal {
  width: min(1080px, 96vw);
}

.attach-modal {
  width: min(1080px, 96vw);
}

.exec-modal {
  width: min(1080px, 96vw);
}

.exec-form {
  border: 1px solid #3a4150;
  border-radius: 10px;
  background: #1f232b;
  padding: 10px;
  display: grid;
  gap: 10px;
}

.attach-terminal {
  border: 1px solid #3a4150;
  border-radius: 10px;
  background: #0f1217;
  padding: 10px;
  min-height: 420px;
  max-height: 62vh;
  overflow: hidden;
  outline: none;
}

.attach-terminal:focus {
  border-color: #1789ff;
  box-shadow: 0 0 0 3px rgba(23, 137, 255, 0.18);
}

.attach-terminal :deep(.xterm),
.attach-terminal :deep(.xterm-screen) {
  height: 100%;
}

.exec-terminal {
  min-height: 320px;
  max-height: 46vh;
}

.log-body {
  border: 1px solid #3a4150;
  border-radius: 10px;
  background: #1a1e25;
  padding: 10px;
  display: grid;
  gap: 8px;
}

.log-text {
  margin: 0;
  max-height: 56vh;
  overflow: auto;
  background: #0f1217;
  border: 1px solid #2f3645;
  border-radius: 8px;
  padding: 10px;
  color: #d9e8ff;
  font-family: 'JetBrains Mono', monospace;
  font-size: 12px;
  line-height: 1.45;
  white-space: pre-wrap;
}

.confirm-message {
  margin: 0;
  font-size: 14px;
}

.confirm-notice {
  border: 1px solid;
  border-radius: 10px;
  padding: 10px;
  display: grid;
  gap: 6px;
}

.confirm-notice strong {
  font-size: 13px;
}

.confirm-notice p {
  margin: 0;
  font-size: 12px;
  line-height: 1.5;
}

.notice-caution {
  border-color: #7b6134;
  background: #3a3020;
  color: #ffd17c;
}

.notice-warning {
  border-color: #8d3a45;
  background: #311f26;
  color: #ff9ea7;
}

.create-form {
  max-height: 62vh;
  overflow: auto;
  display: grid;
  gap: 12px;
  padding-right: 2px;
}

.form-item {
  display: grid;
  gap: 6px;
}

.form-item span {
  color: var(--text-2);
  font-size: 12px;
}

.form-item input,
.form-item select,
.row-grid input,
.row-grid select {
  width: 100%;
  border: 1px solid #3a4150;
  border-radius: 8px;
  background: #1f232b;
  color: var(--text-1);
  padding: 8px 10px;
}

.yaml-input {
  width: 100%;
  min-height: 240px;
  border: 1px solid #3a4150;
  border-radius: 8px;
  background: #1f232b;
  color: var(--text-1);
  padding: 10px;
  resize: vertical;
  font-family: 'JetBrains Mono', monospace;
  font-size: 12px;
  line-height: 1.45;
}

.yaml-input:focus {
  outline: none;
  border-color: var(--primary);
  box-shadow: 0 0 0 3px rgba(23, 137, 255, 0.18);
}

.yaml-error {
  margin: 0;
}

.image-search,
.row-grid input,
.row-grid select {
  outline: none;
}

.image-search:focus,
.row-grid input:focus,
.row-grid select:focus {
  border-color: var(--primary);
  box-shadow: 0 0 0 3px rgba(23, 137, 255, 0.18);
}

.image-combobox {
  position: relative;
}

.combo-toggle {
  position: absolute;
  right: 6px;
  top: 50%;
  transform: translateY(-50%);
  width: 28px;
  height: 28px;
  border-radius: 8px;
  border: 1px solid #3a4150;
  background: #1d2838;
  color: #9fd0ff;
  padding: 0;
  line-height: 1;
}

.image-search {
  padding-right: 40px;
}

.image-dropdown {
  position: absolute;
  left: 0;
  right: 0;
  top: calc(100% + 6px);
  max-height: 180px;
  overflow: auto;
  border: 1px solid #3a4150;
  border-radius: 10px;
  background: #1f232b;
  z-index: 6;
  display: grid;
  padding: 6px;
  gap: 4px;
}

.image-option {
  width: 100%;
  text-align: left;
  border: 1px solid transparent;
  background: #222733;
  color: #cfe8ff;
  border-radius: 8px;
  padding: 7px 9px;
  font-size: 12px;
}

.image-option:hover {
  border-color: #1789ff;
  background: #203750;
}

.image-empty {
  margin: 2px 0;
}

.tty-toggle {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  width: fit-content;
  cursor: pointer;
}

.tty-toggle input {
  position: absolute;
  opacity: 0;
  pointer-events: none;
}

.tty-slider {
  position: relative;
  width: 44px;
  height: 24px;
  border-radius: 999px;
  border: 1px solid #4a5264;
  background: #2a2f3a;
  transition: all 0.18s ease;
}

.tty-slider::before {
  content: '';
  position: absolute;
  left: 3px;
  top: 3px;
  width: 16px;
  height: 16px;
  border-radius: 50%;
  background: #c9d3e7;
  transition: all 0.18s ease;
}

.tty-toggle input:checked + .tty-slider {
  background: #1d4a79;
  border-color: #2f7fcd;
}

.tty-toggle input:checked + .tty-slider::before {
  transform: translateX(20px);
  background: #8fd0ff;
}

.tty-label {
  color: #cfe8ff;
  font-family: 'JetBrains Mono', monospace;
  font-size: 12px;
}

.create-group {
  border: 1px solid #3a4150;
  border-radius: 10px;
  background: #1f232b;
  padding: 10px;
  display: grid;
  gap: 8px;
}

.group-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.group-head h4 {
  margin: 0;
  font-size: 13px;
  color: #8fc4ff;
}

.row-grid {
  display: grid;
  gap: 8px;
}

.row-grid.row-3 {
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr) minmax(0, 1.2fr);
}

.row-grid.row-2 {
  grid-template-columns: minmax(0, 1fr) minmax(0, 1.2fr);
}

.inline-actions {
  display: grid;
  grid-template-columns: 1fr auto;
  gap: 8px;
  align-items: center;
}

.modal-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

.detail-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.detail-head h3 {
  margin: 0;
}

.detail-meta {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
  color: var(--text-2);
}

.detail-meta p {
  margin: 0;
}

.detail-meta span {
  color: var(--text-1);
  font-weight: 600;
}

.detail-sections {
  max-height: 62vh;
  overflow: auto;
  display: grid;
  gap: 10px;
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.detail-card {
  border: 1px solid #3a4150;
  border-radius: 10px;
  padding: 10px;
  background: #1f232b;
}

.detail-card h4 {
  margin: 0 0 8px;
  font-size: 13px;
  color: #8fc4ff;
}

.detail-card dl {
  margin: 0;
  display: grid;
  grid-template-columns: 120px 1fr;
  row-gap: 6px;
  column-gap: 8px;
  font-size: 12px;
}

.detail-card dt {
  color: var(--text-2);
}

.detail-card dd {
  margin: 0;
  overflow-wrap: anywhere;
}

.overview-item,
.stat-card,
.relation-row,
.relation-block,
.relation-items li,
.list li,
.detail-card,
.create-group,
.log-body,
.log-text,
.attach-terminal,
.image-dropdown {
  border: none;
}

@media (max-width: 1100px) {
  .dashboard-grid,
  .resource-grid,
  .panel-grid,
  .detail-sections {
    grid-template-columns: 1fr;
  }

  .dashboard-overview,
  .dashboard-stats,
  .dashboard-donut,
  .dashboard-pulse,
  .dashboard-alerts,
  .dashboard-log-insights {
    grid-column: span 1;
  }

  .overview-grid,
  .stats-grid {
    grid-template-columns: 1fr;
  }

  .donut-wrap {
    flex-direction: column;
    align-items: flex-start;
  }

  .pulse-sections {
    grid-template-columns: 1fr;
  }

  .row-grid.row-3,
  .row-grid.row-2,
  .inline-actions {
    grid-template-columns: 1fr;
  }

  .relation-row {
    grid-template-columns: 1fr;
  }

  .relation-arrow {
    justify-self: center;
    transform: rotate(90deg);
  }
}

@media (max-width: 840px) {
  .layout {
    grid-template-columns: 1fr;
    height: auto;
    overflow: visible;
  }

  .sidebar {
    border-right: none;
    border-bottom: 1px solid var(--line);
    height: auto;
    overflow: visible;
  }

  .content {
    height: auto;
    overflow: visible;
  }

  .menu {
    grid-template-columns: repeat(4, minmax(0, 1fr));
  }

  .menu button {
    text-align: center;
  }
}
</style>
