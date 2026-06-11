<template>
  <section class="resource-grid">
    <article class="panel">
      <div class="section-head">
        <div class="container-head-left">
          <h3>Policy Mode</h3>
          <button class="primary-outline" @click="openPolicyCreateOverlay">Create Policy</button>
          <button class="caution" @click="openPolicyCommitOverlay">Commit</button>
          <button class="caution" @click="openPolicyRevertOverlay">Revert</button>
        </div>
      </div>
      <div class="stats-grid policy-mode-grid">
        <div class="stat-card policy-mode-card">
          <span>Inter Container Mode</span>
          <strong>{{ policyModeLabel(policyData['RAIND-EW']?.mode) }}</strong>
        </div>
        <div class="stat-card policy-mode-card">
          <span>External Mode</span>
          <strong>{{ policyModeLabel(nsModeSummary) }}</strong>
        </div>
      </div>
    </article>

    <article v-for="chain in visiblePolicyChains()" :key="chain" class="panel resource-panel">
      <div class="section-head">
        <h3>{{ policyChainLabel(chain) }}</h3>
        <span class="count">{{ policyData[chain]?.policies_total ?? 0 }} rules</span>
      </div>
      <p class="selector">mode: {{ policyData[chain]?.mode || '-' }}</p>
      <div class="table-scroller">
        <table>
          <thead>
            <tr>
              <th>Status</th>
              <th>Policy ID</th>
              <th>Source</th>
              <th>Destination</th>
              <th>Protocol</th>
              <th>Destination Port</th>
              <th>Comment</th>
              <th>Reason</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="p in policyData[chain]?.policies || []" :key="`${chain}-${p.id}`">
              <td>
                <span :class="policyStatusClass(p.status)">
                  <span class="status-lamp"></span>
                  {{ p.status || '-' }}
                </span>
              </td>
              <td class="mono">{{ p.id || '-' }}</td>
              <td>
                <span
                  v-if="p.source?.container_name"
                  class="policy-link"
                  role="button"
                  tabindex="0"
                  @click="openPolicyContainerDetail(p.source.container_name)"
                  @keydown.enter.prevent="openPolicyContainerDetail(p.source.container_name)"
                  @keydown.space.prevent="openPolicyContainerDetail(p.source.container_name)"
                >
                  {{ p.source.container_name }}
                </span>
                <span v-else>{{ p.source?.address || '-' }}</span>
              </td>
              <td>
                <span
                  v-if="p.destination?.container_name"
                  class="policy-link"
                  role="button"
                  tabindex="0"
                  @click="openPolicyContainerDetail(p.destination.container_name)"
                  @keydown.enter.prevent="openPolicyContainerDetail(p.destination.container_name)"
                  @keydown.space.prevent="openPolicyContainerDetail(p.destination.container_name)"
                >
                  {{ p.destination.container_name }}
                </span>
                <span v-else>{{ p.destination?.address || '-' }}</span>
              </td>
              <td>{{ p.protocol || '*' }}</td>
              <td>{{ p.dport ?? '*' }}</td>
              <td>{{ p.comment || '-' }}</td>
              <td>{{ p.reason || '-' }}</td>
              <td class="actions">
                <button class="danger" @click="openPolicyDeleteOverlay(p.id, chain)">Delete</button>
              </td>
            </tr>
            <tr v-if="(policyData[chain]?.policies || []).length === 0">
              <td colspan="9" class="empty">{{ policyError[chain] || 'No policies' }}</td>
            </tr>
            <tr :class="['policy-default-row', policyDefaultRowClass(chain)]">
              <td colspan="9">
                <strong>{{ policyDefaultTitle(chain) }}</strong>
                <span>{{ policyDefaultDescription(chain) }}</span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </article>
  </section>
</template>

<script setup>
const props = defineProps({
  policyChains: { type: Array, required: true },
  policyData: { type: Object, required: true },
  policyError: { type: Object, required: true },
  policyModeLabel: { type: Function, required: true },
  policyChainLabel: { type: Function, required: true },
  nsModeSummary: { type: String, required: true },
  openPolicyCreateOverlay: { type: Function, required: true },
  openPolicyDeleteOverlay: { type: Function, required: true },
  openPolicyCommitOverlay: { type: Function, required: true },
  openPolicyRevertOverlay: { type: Function, required: true },
  openPolicyContainerDetail: { type: Function, required: true }
})

function policyStatusClass(status) {
  const s = String(status || '').toLowerCase()
  if (s === 'applied') return 'status-chip policy-applied'
  if (s === 'remove_next_commit') return 'status-chip policy-remove-next-commit'
  if (s === 'before_commit') return 'status-chip policy-before-commit'
  return 'status-chip unknown'
}

function policyDefaultTitle(chain) {
  if (chain === 'RAIND-EW') return 'Deny by default'
  if (chain === 'RAIND-NS-OBS') return 'Observe'
  if (chain === 'RAIND-NS-ENF') return 'Enforce'
  return 'Default'
}

function policyDefaultDescription(chain) {
  if (chain === 'RAIND-EW') return 'All Inter Container Traffic is Denied'
  if (chain === 'RAIND-NS-OBS') return 'All External Traffic is Allowed'
  if (chain === 'RAIND-NS-ENF') return 'All External Traffic is Denied'
  return ''
}

function policyDefaultRowClass(chain) {
  if (chain === 'RAIND-EW' || chain === 'RAIND-NS-ENF') return 'policy-default-denied'
  return 'policy-default-allowed'
}

function visiblePolicyChains() {
  const mode = String(props.nsModeSummary || '').toLowerCase()
  const isObserve = mode.includes('observe')
  const isEnforce = mode.includes('enforce')

  if (isObserve && !isEnforce) {
    return ['RAIND-EW', 'RAIND-NS-OBS']
  }
  if (isEnforce && !isObserve) {
    return ['RAIND-EW', 'RAIND-NS-ENF']
  }
  // Fallback for mixed/unknown mode: keep both external chains visible.
  return ['RAIND-EW', 'RAIND-NS-OBS', 'RAIND-NS-ENF']
}
</script>
