<template>
  <section class="panel">
    <div class="section-head">
      <div class="container-head-left">
        <h3>Audit Log (Last 24h)</h3>
      </div>
      <span class="count">{{ total }} items</span>
    </div>

    <div class="section-head-actions">
      <span class="selector">source: {{ source || '-' }}</span>
      <span class="selector">parse errors: {{ parseErrors }}</span>
    </div>

    <div class="log-filter-grid">
      <input v-model.trim="filter.q" class="filter-input" placeholder="Filter: actor / path / correlation / code" />
      <select v-model="filter.actor" class="filter-select">
        <option value="">Actor: All</option>
        <option v-for="actor in actorOptionsResolved" :key="actor" :value="actor">{{ actor }}</option>
      </select>
      <select v-model="filter.severity" class="filter-select">
        <option value="">Severity: All</option>
        <option value="information">information</option>
        <option value="low">low</option>
        <option value="medium">medium</option>
        <option value="high">high</option>
        <option value="critical">critical</option>
      </select>
      <select v-model="filter.action" class="filter-select">
        <option value="">Action: All</option>
        <option value="container.create">container.create</option>
        <option value="container.start">container.start</option>
        <option value="container.stop">container.stop</option>
        <option value="container.delete">container.delete</option>
        <option value="policy.add">policy.add</option>
        <option value="policy.commit">policy.commit</option>
        <option value="policy.revert">policy.revert</option>
        <option value="unknown">unknown</option>
      </select>
      <select v-model="filter.result_status" class="filter-select">
        <option value="">Result: All</option>
        <option value="allow">allow</option>
        <option value="deny">deny</option>
        <option value="error">error</option>
      </select>
      <button class="primary-outline" @click="clearFilter">Clear</button>
    </div>
    <p class="selector">total(after filter): {{ total }} / page: {{ logs.length }}</p>

    <div class="table-scroller">
      <table>
        <thead>
          <tr>
            <th>Generated</th>
            <th>Severity</th>
            <th>Action</th>
            <th>Actor</th>
            <th>Request</th>
            <th>Result</th>
            <th>Correlation ID</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in logs" :key="row.event_id || `${row.generated_ts}-${row.correlation_id}`">
            <td>{{ formatTime(row.generated_ts) }}</td>
            <td>
              <span :class="severityClass(row.severity)">
                <span class="status-lamp"></span>
                {{ row.severity || '-' }}
              </span>
            </td>
            <td>{{ row.action || '-' }}</td>
            <td>{{ row.actor?.spiffe_id || row.actor?.peer_ip || '-' }}</td>
            <td class="mono">
              {{ row.request?.method || '-' }} {{ row.request?.path || '-' }}
            </td>
            <td class="mono">
              {{ row.result?.status || '-' }} / {{ row.result?.code ?? '-' }} / {{ row.result?.latence_ms ?? '-' }}ms
            </td>
            <td class="mono">{{ row.correlation_id || '-' }}</td>
          </tr>
          <tr v-if="logs.length === 0 && !loading">
            <td colspan="7" class="empty">No audit logs</td>
          </tr>
          <tr v-if="loading">
            <td colspan="7" class="empty">Loading...</td>
          </tr>
        </tbody>
      </table>
    </div>

    <div class="section-head" style="margin-top: 10px">
      <span class="count">Page {{ page }} / {{ totalPagesLabel }}</span>
      <div class="actions">
        <button :disabled="loading || page <= 1" @click="changePage(page - 1)">Prev</button>
        <button :disabled="loading || isLastPage" @click="changePage(page + 1)">Next</button>
      </div>
    </div>
  </section>
</template>

<script setup>
import { computed, ref, watch } from 'vue'

const props = defineProps({
  logs: { type: Array, required: true },
  loading: { type: Boolean, required: true },
  page: { type: Number, required: true },
  total: { type: Number, required: true },
  totalPages: { type: Number, required: true },
  parseErrors: { type: Number, required: true },
  source: { type: String, required: true },
  actorOptions: { type: Array, required: true },
  formatTime: { type: Function, required: true },
  changePage: { type: Function, required: true },
  setFilter: { type: Function, required: true }
})

const totalPagesLabel = computed(() => (props.totalPages > 0 ? props.totalPages : 1))
const isLastPage = computed(() => props.totalPages > 0 && props.page >= props.totalPages)
const actorOptionsResolved = computed(() =>
  Array.isArray(props.actorOptions) ? props.actorOptions.filter(Boolean) : []
)

const filter = ref({
  q: '',
  actor: '',
  severity: '',
  action: '',
  result_status: ''
})
let filterTimer = null

function clearFilter() {
  filter.value = { q: '', actor: '', severity: '', action: '', result_status: '' }
}

watch(
  filter,
  () => {
    if (filterTimer) clearTimeout(filterTimer)
    filterTimer = setTimeout(() => {
      props.setFilter({ ...filter.value })
      props.changePage(1)
      filterTimer = null
    }, 220)
  },
  { deep: true }
)

function severityClass(severity) {
  const s = String(severity || '').toLowerCase()
  if (s === 'information' || s === 'info') return 'status-chip severity-information'
  if (s === 'low') return 'status-chip severity-low'
  if (s === 'medium') return 'status-chip severity-medium'
  if (s === 'high') return 'status-chip severity-high'
  if (s === 'critical') return 'status-chip severity-critical'
  return 'status-chip unknown'
}
</script>

<style scoped>
.severity-information {
  color: #8bc9ff;
  border-color: #385f80;
  background: #202f3e;
}

.severity-information .status-lamp {
  background: #6ebeff;
}

.severity-low {
  color: #9ef0be;
  border-color: #2f6b48;
  background: #1f3527;
}

.severity-low .status-lamp {
  background: #56da8d;
}

.severity-medium {
  color: #ffd17c;
  border-color: #7b6134;
  background: #3a3020;
}

.severity-medium .status-lamp {
  background: #ffbe4d;
}

.severity-high {
  color: #ffb178;
  border-color: #7a4b2f;
  background: #3b2a22;
}

.severity-high .status-lamp {
  background: #ff8f45;
}

.severity-critical {
  color: #ff9ea7;
  border-color: #8d3a45;
  background: #311f26;
}

.severity-critical .status-lamp {
  background: #ff6f7e;
}

.log-filter-grid {
  display: grid;
  grid-template-columns: minmax(240px, 2fr) repeat(4, minmax(140px, 1fr)) auto;
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

.table-scroller {
  max-height: calc(100vh - 300px);
}

@media (max-width: 1300px) {
  .log-filter-grid {
    grid-template-columns: 1fr;
  }
}
</style>
