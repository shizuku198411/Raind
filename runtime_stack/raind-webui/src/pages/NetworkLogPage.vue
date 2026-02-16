<template>
  <section class="resource-grid">
    <article class="panel">
      <div class="section-head">
        <h3>Traffic Log (Last {{ traffic.window_hours || 24 }}h)</h3>
        <span class="count">{{ traffic.total || 0 }} items</span>
      </div>

      <div class="stats-grid">
        <div class="stat-card">
          <span>Total</span>
          <strong>{{ traffic.summary?.total ?? 0 }}</strong>
        </div>
        <div class="stat-card">
          <span>Allow</span>
          <strong>{{ traffic.summary?.allow ?? 0 }}</strong>
        </div>
        <div class="stat-card">
          <span>Deny/Error</span>
          <strong>{{ (traffic.summary?.deny ?? 0) + (traffic.summary?.error ?? 0) }}</strong>
        </div>
      </div>

      <div class="network-chart-grid">
        <div class="network-chart-card">
          <h4>Events / Hour</h4>
          <svg :viewBox="`0 0 ${chart.width} ${chart.height}`" class="line-chart">
            <polyline :points="trafficGridLine(0.25)" class="grid-line"></polyline>
            <polyline :points="trafficGridLine(0.5)" class="grid-line"></polyline>
            <polyline :points="trafficGridLine(0.75)" class="grid-line"></polyline>
            <polyline :points="seriesPoints(traffic.series, 'total')" class="line-total"></polyline>
            <polyline :points="seriesPoints(traffic.series, 'allow')" class="line-ok"></polyline>
            <polyline :points="seriesPoints(traffic.series, 'deny')" class="line-ng"></polyline>
            <text class="axis-label y" :x="18" :y="chart.top + chart.plotH / 2">Events</text>
            <text class="axis-label x" :x="chart.left + chart.plotW / 2" :y="chart.height - 8">Time (Hour)</text>
            <text class="axis-tick" :x="chart.left - 28" :y="chart.top + 4">{{ yMaxLabel(traffic.series) }}</text>
            <text class="axis-tick" :x="chart.left - 28" :y="chart.top + chart.plotH / 2 + 4">{{ yMidLabel(traffic.series) }}</text>
            <text class="axis-tick" :x="chart.left - 18" :y="chart.top + chart.plotH + 4">0</text>
            <text
              v-for="(tick, idx) in xTicks(traffic.series)"
              :key="`tx-${idx}`"
              class="axis-tick"
              :x="tick.x"
              :y="chart.top + chart.plotH + 16"
            >
              {{ displayTickLabel(tick.label, traffic) }}
            </text>
          </svg>
          <div class="chart-legend">
            <span><i class="legend-dot total"></i>Total</span>
            <span><i class="legend-dot ok"></i>Allow</span>
            <span><i class="legend-dot ng"></i>Deny</span>
          </div>
        </div>

        <div class="network-chart-card">
          <h4>Verdict Ratio</h4>
          <div class="pie-wrap">
            <div class="pie" :style="trafficPieStyle"></div>
            <div class="pie-legend">
              <span><i class="legend-dot ok"></i>Allow {{ traffic.summary?.allow ?? 0 }}</span>
              <span><i class="legend-dot ng"></i>Deny {{ traffic.summary?.deny ?? 0 }}</span>
              <span><i class="legend-dot err"></i>Error {{ traffic.summary?.error ?? 0 }}</span>
            </div>
          </div>
        </div>
      </div>

      <p class="selector">source: {{ traffic.source || '-' }} / parse errors: {{ traffic.parse_errors || 0 }}</p>
      <div class="log-filter-grid">
        <input v-model.trim="trafficFilter.q" class="filter-input" placeholder="Filter: IP / container / rule / proto" />
        <select v-model="trafficFilter.kind" class="filter-select">
          <option value="">Kind: All</option>
          <option value="north-south">north-south</option>
          <option value="east-west">east-west</option>
        </select>
        <select v-model="trafficFilter.verdict" class="filter-select">
          <option value="">Verdict: All</option>
          <option value="allow">allow</option>
          <option value="deny">deny</option>
          <option value="error">error</option>
        </select>
        <select v-model="trafficFilter.proto" class="filter-select">
          <option value="">Proto: All</option>
          <option value="TCP">TCP</option>
          <option value="UDP">UDP</option>
          <option value="ICMP">ICMP</option>
        </select>
        <button class="primary-outline" @click="clearTrafficFilter">Clear</button>
      </div>
      <p class="selector">total(after filter): {{ traffic.total || 0 }} / page: {{ (traffic.items || []).length }}</p>

      <div class="table-scroller">
        <table>
          <thead>
            <tr>
              <th>Generated</th>
              <th>Kind</th>
              <th>Verdict</th>
              <th>Proto</th>
              <th>Source Container</th>
              <th>Source</th>
              <th>Destination Container</th>
              <th>Destination</th>
              <th>Rule</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in traffic.items || []" :key="row.raw_hash || row.generated_ts">
              <td>{{ formatTime(row.generated_ts) }}</td>
              <td>{{ row.kind || '-' }}</td>
              <td>
                <span :class="verdictClass(row.verdict)">
                  <span class="status-lamp"></span>
                  {{ row.verdict || '-' }}
                </span>
              </td>
              <td>{{ row.proto || '-' }}</td>
              <td>
                <span
                  v-if="row.src?.container_name || row.src?.container_id"
                  class="container-link"
                  role="button"
                  tabindex="0"
                  @click="showContainerDetail(row.src?.container_id, row.src?.container_name)"
                  @keydown.enter.prevent="showContainerDetail(row.src?.container_id, row.src?.container_name)"
                  @keydown.space.prevent="showContainerDetail(row.src?.container_id, row.src?.container_name)"
                >
                  {{ row.src?.container_name || row.src?.container_id }}
                </span>
                <span v-else>-</span>
              </td>
              <td class="mono">
                {{ row.src?.ip || '-' }}:{{ row.src?.port ?? '-' }}
              </td>
              <td>
                <span
                  v-if="row.dst?.container_name || row.dst?.container_id"
                  class="container-link"
                  role="button"
                  tabindex="0"
                  @click="showContainerDetail(row.dst?.container_id, row.dst?.container_name)"
                  @keydown.enter.prevent="showContainerDetail(row.dst?.container_id, row.dst?.container_name)"
                  @keydown.space.prevent="showContainerDetail(row.dst?.container_id, row.dst?.container_name)"
                >
                  {{ row.dst?.container_name || row.dst?.container_id }}
                </span>
                <span v-else>-</span>
              </td>
              <td class="mono">
                {{ row.dst?.ip || '-' }}:{{ row.dst?.port ?? '-' }}
                <span v-if="row.dst?.container_name"> ({{ row.dst.container_name }})</span>
              </td>
              <td class="mono">{{ row.rule_hint || '-' }}</td>
            </tr>
            <tr v-if="(traffic.items || []).length === 0 && !loadingTraffic">
              <td colspan="9" class="empty">No traffic logs in last 24 hours</td>
            </tr>
            <tr v-if="loadingTraffic">
              <td colspan="9" class="empty">Loading...</td>
            </tr>
          </tbody>
        </table>
      </div>

      <div class="section-head" style="margin-top: 10px">
        <span class="count">Page {{ traffic.page || 1 }} / {{ trafficPages }}</span>
        <div class="actions">
          <button :disabled="loadingTraffic || (traffic.page || 1) <= 1" @click="loadTraffic((traffic.page || 1) - 1)">
            Prev
          </button>
          <button :disabled="loadingTraffic || (traffic.page || 1) >= (traffic.total_pages || 1)" @click="loadTraffic((traffic.page || 1) + 1)">
            Next
          </button>
        </div>
      </div>
    </article>

    <article class="panel">
      <div class="section-head">
        <h3>DNS Log (Last {{ dns.window_hours || 24 }}h)</h3>
        <span class="count">{{ dns.total || 0 }} items</span>
      </div>

      <div class="stats-grid">
        <div class="stat-card">
          <span>Total</span>
          <strong>{{ dns.summary?.total ?? 0 }}</strong>
        </div>
        <div class="stat-card">
          <span>Query OK</span>
          <strong>{{ dns.summary?.ok ?? 0 }}</strong>
        </div>
        <div class="stat-card">
          <span>Cache Hit</span>
          <strong>{{ dns.summary?.cache?.hit ?? 0 }}</strong>
        </div>
      </div>

      <div class="network-chart-grid">
        <div class="network-chart-card">
          <h4>Queries / Hour</h4>
          <svg :viewBox="`0 0 ${chart.width} ${chart.height}`" class="line-chart">
            <polyline :points="dnsGridLine(0.25)" class="grid-line"></polyline>
            <polyline :points="dnsGridLine(0.5)" class="grid-line"></polyline>
            <polyline :points="dnsGridLine(0.75)" class="grid-line"></polyline>
            <polyline :points="seriesPoints(dns.series, 'total')" class="line-total"></polyline>
            <polyline :points="seriesPoints(dns.series, 'ok')" class="line-ok"></polyline>
            <polyline :points="seriesPoints(dns.series, 'ng')" class="line-ng"></polyline>
            <text class="axis-label y" :x="18" :y="chart.top + chart.plotH / 2">Queries</text>
            <text class="axis-label x" :x="chart.left + chart.plotW / 2" :y="chart.height - 8">Time (Hour)</text>
            <text class="axis-tick" :x="chart.left - 28" :y="chart.top + 4">{{ yMaxLabel(dns.series) }}</text>
            <text class="axis-tick" :x="chart.left - 28" :y="chart.top + chart.plotH / 2 + 4">{{ yMidLabel(dns.series) }}</text>
            <text class="axis-tick" :x="chart.left - 18" :y="chart.top + chart.plotH + 4">0</text>
            <text
              v-for="(tick, idx) in xTicks(dns.series)"
              :key="`dx-${idx}`"
              class="axis-tick"
              :x="tick.x"
              :y="chart.top + chart.plotH + 16"
            >
              {{ displayTickLabel(tick.label, dns) }}
            </text>
          </svg>
          <div class="chart-legend">
            <span><i class="legend-dot total"></i>Total</span>
            <span><i class="legend-dot ok"></i>OK</span>
            <span><i class="legend-dot ng"></i>NG</span>
          </div>
        </div>

        <div class="network-chart-card">
          <h4>Cache Hit Ratio</h4>
          <div class="pie-wrap">
            <div class="pie" :style="dnsPieStyle"></div>
            <div class="pie-legend">
              <span><i class="legend-dot ok"></i>Hit {{ dns.summary?.cache?.hit ?? 0 }}</span>
              <span><i class="legend-dot ng"></i>Miss {{ dns.summary?.cache?.miss ?? 0 }}</span>
            </div>
          </div>
        </div>
      </div>

      <p class="selector">source: {{ dns.source || '-' }} / parse errors: {{ dns.parse_errors || 0 }}</p>
      <div class="log-filter-grid">
        <input v-model.trim="dnsFilter.q" class="filter-input" placeholder="Filter: query / container / upstream / ip" />
        <select v-model="dnsFilter.result" class="filter-select">
          <option value="">Result: All</option>
          <option value="ok">ok</option>
          <option value="ng">ng</option>
          <option value="error">error</option>
        </select>
        <select v-model="dnsFilter.rcode" class="filter-select">
          <option value="">Rcode: All</option>
          <option value="NOERROR">NOERROR</option>
          <option value="NXDOMAIN">NXDOMAIN</option>
          <option value="SERVFAIL">SERVFAIL</option>
        </select>
        <select v-model="dnsFilter.cache" class="filter-select">
          <option value="">Cache: All</option>
          <option value="hit">hit</option>
          <option value="miss">miss</option>
        </select>
        <button class="primary-outline" @click="clearDnsFilter">Clear</button>
      </div>
      <p class="selector">total(after filter): {{ dns.total || 0 }} / page: {{ (dns.items || []).length }}</p>

      <div class="table-scroller">
        <table>
          <thead>
            <tr>
              <th>Generated</th>
              <th>Source Container</th>
              <th>Query</th>
              <th>Type</th>
              <th>Result</th>
              <th>Cache</th>
              <th>Upstream</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in dns.items || []" :key="`${row.generated_ts}-${row.dns?.id || row.raw_hash || ''}`">
              <td>{{ formatTime(row.generated_ts) }}</td>
              <td>
                <span
                  v-if="row.src?.container_name || row.src?.container_id"
                  class="container-link"
                  role="button"
                  tabindex="0"
                  @click="showContainerDetail(row.src?.container_id, row.src?.container_name)"
                  @keydown.enter.prevent="showContainerDetail(row.src?.container_id, row.src?.container_name)"
                  @keydown.space.prevent="showContainerDetail(row.src?.container_id, row.src?.container_name)"
                >
                  {{ row.src?.container_name || row.src?.container_id }}
                </span>
                <span v-else>-</span>
              </td>
              <td>{{ row.dns?.question?.name || '-' }}</td>
              <td>{{ row.dns?.question?.type || '-' }}</td>
              <td>
                {{ row.query_result || '-' }} / {{ row.dns?.response?.rcode || '-' }}
              </td>
              <td>{{ row.cache?.hit === true ? 'hit' : 'miss' }}</td>
              <td>{{ row.upstream?.server || '-' }}</td>
            </tr>
            <tr v-if="(dns.items || []).length === 0 && !loadingDns">
              <td colspan="7" class="empty">No DNS logs in last 24 hours</td>
            </tr>
            <tr v-if="loadingDns">
              <td colspan="7" class="empty">Loading...</td>
            </tr>
          </tbody>
        </table>
      </div>

      <div class="section-head" style="margin-top: 10px">
        <span class="count">Page {{ dns.page || 1 }} / {{ dnsPages }}</span>
        <div class="actions">
          <button :disabled="loadingDns || (dns.page || 1) <= 1" @click="loadDns((dns.page || 1) - 1)">Prev</button>
          <button :disabled="loadingDns || (dns.page || 1) >= (dns.total_pages || 1)" @click="loadDns((dns.page || 1) + 1)">Next</button>
        </div>
      </div>
    </article>
  </section>
</template>

<script setup>
import { computed, ref, watch } from 'vue'

const props = defineProps({
  traffic: { type: Object, required: true },
  dns: { type: Object, required: true },
  loadingTraffic: { type: Boolean, required: true },
  loadingDns: { type: Boolean, required: true },
  formatTime: { type: Function, required: true },
  loadTraffic: { type: Function, required: true },
  loadDns: { type: Function, required: true },
  setTrafficFilter: { type: Function, required: true },
  setDnsFilter: { type: Function, required: true },
  openContainerDetailByRef: { type: Function, required: true }
})

const trafficPages = computed(() => (props.traffic.total_pages > 0 ? props.traffic.total_pages : 1))
const dnsPages = computed(() => (props.dns.total_pages > 0 ? props.dns.total_pages : 1))
const trafficFilter = ref({
  q: '',
  kind: '',
  verdict: '',
  proto: ''
})
const dnsFilter = ref({
  q: '',
  result: '',
  rcode: '',
  cache: ''
})
const chart = {
  width: 920,
  height: 220,
  left: 56,
  right: 16,
  top: 14,
  bottom: 44
}
chart.plotW = chart.width - chart.left - chart.right
chart.plotH = chart.height - chart.top - chart.bottom

function maxSeriesValue(series, keys) {
  let max = 1
  for (const row of series || []) {
    for (const key of keys) {
      const v = Number(row?.[key] || 0)
      if (v > max) max = v
    }
  }
  return max
}

function seriesPoints(series, key) {
  const rows = Array.isArray(series) ? series : []
  if (rows.length === 0) return ''

  const max = maxSeriesValue(rows, ['total', 'allow', 'deny', 'error', 'ok', 'ng'])
  return rows
    .map((row, idx) => {
      const x = chart.left + (idx / Math.max(rows.length - 1, 1)) * chart.plotW
      const y = chart.top + (1 - Number(row?.[key] || 0) / max) * chart.plotH
      return `${x},${y}`
    })
    .join(' ')
}

function yMaxLabel(series) {
  return String(maxSeriesValue(series, ['total', 'allow', 'deny', 'error', 'ok', 'ng']))
}

function yMidLabel(series) {
  const max = maxSeriesValue(series, ['total', 'allow', 'deny', 'error', 'ok', 'ng'])
  return String(Math.floor(max / 2))
}

function xTicks(series) {
  const rows = Array.isArray(series) ? series : []
  if (rows.length === 0) return []
  const marks = [0, 6, 12, 18, 23]
  return marks
    .filter((idx) => idx < rows.length)
    .map((idx) => ({
      x: chart.left + (idx / Math.max(rows.length - 1, 1)) * chart.plotW - 10,
      label: rows[idx]?.hour || '',
      index: idx
    }))
}

function shiftHourLabel(label, offsetMinutes) {
  const m = String(label || '').match(/^(\d{2}):(\d{2})$/)
  if (!m) return label || ''
  const hh = Number.parseInt(m[1], 10)
  const mm = Number.parseInt(m[2], 10)
  if (!Number.isFinite(hh) || !Number.isFinite(mm)) return label || ''
  const total = hh * 60 + mm + offsetMinutes
  const day = 24 * 60
  const normalized = ((total % day) + day) % day
  const outH = String(Math.floor(normalized / 60)).padStart(2, '0')
  const outM = String(normalized % 60).padStart(2, '0')
  return `${outH}:${outM}`
}

function displayTickLabel(tickLabel, payload) {
  const applied = Boolean(payload?.series_timezone_applied)
  const offset = Number(payload?.timezone_offset_minutes || 0)
  if (applied || offset === 0) return tickLabel
  return shiftHourLabel(tickLabel, offset)
}

function gridLinePoints(ratio) {
  const y = chart.top + ratio * chart.plotH
  return `${chart.left},${y} ${chart.width - chart.right},${y}`
}

function trafficGridLine(ratio) {
  return gridLinePoints(ratio)
}

function dnsGridLine(ratio) {
  return gridLinePoints(ratio)
}

function pieStyle(values, colors) {
  const sum = values.reduce((a, b) => a + b, 0)
  if (sum <= 0) {
    return { background: 'conic-gradient(#2c3240 0deg 360deg)' }
  }
  let start = 0
  const slices = values.map((v, i) => {
    const deg = (v / sum) * 360
    const s = `${colors[i]} ${start}deg ${start + deg}deg`
    start += deg
    return s
  })
  return { background: `conic-gradient(${slices.join(', ')})` }
}

const trafficPieStyle = computed(() =>
  pieStyle(
    [
      Number(props.traffic?.summary?.allow || 0),
      Number(props.traffic?.summary?.deny || 0),
      Number(props.traffic?.summary?.error || 0)
    ],
    ['#56da8d', '#ff7d8a', '#ffbe4d']
  )
)

const dnsPieStyle = computed(() =>
  pieStyle(
    [Number(props.dns?.summary?.cache?.hit || 0), Number(props.dns?.summary?.cache?.miss || 0)],
    ['#56da8d', '#ff7d8a']
  )
)

function verdictClass(verdict) {
  const v = String(verdict || '').toLowerCase()
  if (v === 'allow') return 'status-chip running'
  if (v === 'deny') return 'status-chip stopped'
  return 'status-chip creating'
}

function showContainerDetail(containerId, containerName) {
  props.openContainerDetailByRef(containerId, containerName)
}

function clearTrafficFilter() {
  trafficFilter.value = { q: '', kind: '', verdict: '', proto: '' }
}

function clearDnsFilter() {
  dnsFilter.value = { q: '', result: '', rcode: '', cache: '' }
}

let trafficFilterTimer = null
let dnsFilterTimer = null

function applyTrafficFilterNow() {
  props.setTrafficFilter({
    q: trafficFilter.value.q,
    traffic_kind: trafficFilter.value.kind,
    verdict: trafficFilter.value.verdict,
    proto: trafficFilter.value.proto
  })
  props.loadTraffic(1)
}

function applyDnsFilterNow() {
  props.setDnsFilter({
    q: dnsFilter.value.q,
    result: dnsFilter.value.result,
    rcode: dnsFilter.value.rcode,
    cache: dnsFilter.value.cache
  })
  props.loadDns(1)
}

watch(
  trafficFilter,
  () => {
    if (trafficFilterTimer) clearTimeout(trafficFilterTimer)
    trafficFilterTimer = setTimeout(() => {
      applyTrafficFilterNow()
      trafficFilterTimer = null
    }, 220)
  },
  { deep: true }
)

watch(
  dnsFilter,
  () => {
    if (dnsFilterTimer) clearTimeout(dnsFilterTimer)
    dnsFilterTimer = setTimeout(() => {
      applyDnsFilterNow()
      dnsFilterTimer = null
    }, 220)
  },
  { deep: true }
)
</script>

<style scoped>
.network-chart-grid {
  display: grid;
  grid-template-columns: 4fr 1fr;
  gap: 10px;
  margin-top: 10px;
  margin-bottom: 10px;
}

.network-chart-card {
  border: 1px solid #334059;
  border-radius: 10px;
  background: #1f232b;
  padding: 10px;
}

.network-chart-card h4 {
  margin: 0 0 8px;
  color: #cfe8ff;
  font-size: 13px;
}

.line-chart {
  width: 100%;
  height: 220px;
  display: block;
  background: #161a21;
  border: 1px solid #2d3441;
  border-radius: 8px;
}

.grid-line {
  fill: none;
  stroke: #2a3040;
  stroke-width: 1;
}

.line-total {
  fill: none;
  stroke: #9fd0ff;
  stroke-width: 2.2;
}

.line-ok {
  fill: none;
  stroke: #56da8d;
  stroke-width: 1.8;
}

.line-ng {
  fill: none;
  stroke: #ff7d8a;
  stroke-width: 1.8;
}

.axis-label {
  fill: #8fa3bf;
  font-size: 10px;
}

.axis-label.x {
  text-anchor: middle;
}

.axis-label.y {
  transform-box: fill-box;
  transform-origin: center;
  transform: rotate(-90deg);
  text-anchor: middle;
}

.axis-tick {
  fill: #7f90aa;
  font-size: 9px;
}

.chart-legend {
  display: flex;
  gap: 10px;
  margin-top: 8px;
  color: #adb6c9;
  font-size: 12px;
}

.legend-dot {
  width: 9px;
  height: 9px;
  border-radius: 50%;
  display: inline-block;
  margin-right: 6px;
}

.legend-dot.total {
  background: #9fd0ff;
}

.legend-dot.ok {
  background: #56da8d;
}

.legend-dot.ng {
  background: #ff7d8a;
}

.legend-dot.err {
  background: #ffbe4d;
}

.pie-wrap {
  display: grid;
  justify-items: center;
  gap: 10px;
}

.pie {
  width: 140px;
  height: 140px;
  border-radius: 50%;
  border: 1px solid #3a4150;
}

.pie-legend {
  display: grid;
  gap: 6px;
  color: #adb6c9;
  font-size: 12px;
}

.container-link {
  color: #9fd0ff;
  text-decoration: underline;
  text-underline-offset: 2px;
  cursor: pointer;
}

.container-link:hover {
  color: #cfe8ff;
}

.container-link:focus-visible {
  outline: 1px solid #1789ff;
  outline-offset: 2px;
  border-radius: 3px;
}

.log-filter-grid {
  display: grid;
  grid-template-columns: minmax(260px, 2fr) repeat(3, minmax(140px, 1fr)) auto;
  gap: 8px;
  margin: 10px 0 4px;
}

.filter-input,
.filter-select {
  border: 1px solid #3a4150;
  background: #232a35;
  color: #f2f5fb;
  border-radius: 8px;
  padding: 7px 10px;
}

@media (max-width: 1100px) {
  .network-chart-grid {
    grid-template-columns: 1fr;
  }

  .log-filter-grid {
    grid-template-columns: 1fr;
  }
}
</style>
