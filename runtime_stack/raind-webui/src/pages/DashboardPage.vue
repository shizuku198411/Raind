<template>
  <section class="dashboard-grid">
    <article class="panel dashboard-overview">
      <div class="section-head">
        <h3>Runtime Overview</h3>
      </div>
      <div class="overview-grid">
        <div class="overview-item">
          <span class="overview-label">Runtime</span>
          <strong>{{ runtimeName }}</strong>
        </div>
        <div class="overview-item">
          <span class="overview-label">Version</span>
          <strong>{{ runtimeVersion }}</strong>
        </div>
        <div class="overview-item">
          <span class="overview-label">Connection</span>
          <span :class="connectionStatusClass">
            <span class="status-lamp"></span>
            {{ connectionStatus }}
          </span>
        </div>
      </div>
    </article>

    <article class="panel dashboard-stats">
      <div class="section-head">
        <h3>Quick Stats</h3>
      </div>
      <div class="stats-grid">
        <div class="stat-card">
          <span>Total Containers</span>
          <strong>{{ totalContainers }}</strong>
        </div>
        <div class="stat-card">
          <span>Running</span>
          <strong>{{ statusCounts.running }}</strong>
        </div>
        <div class="stat-card">
          <span>Bottles</span>
          <strong>{{ bottlesCount }}</strong>
        </div>
        <div class="stat-card">
          <span>Pods</span>
          <strong>{{ podsCount }}</strong>
        </div>
        <div class="stat-card">
          <span>ReplicaSets</span>
          <strong>{{ replicasetsCount }}</strong>
        </div>
        <div class="stat-card">
          <span>Services</span>
          <strong>{{ servicesCount }}</strong>
        </div>
      </div>
    </article>

    <article class="panel dashboard-donut">
      <div class="section-head">
        <h3>Container Status Ratio</h3>
      </div>
      <div class="donut-wrap">
        <div class="status-donut" :style="donutStyle">
          <span>{{ totalContainers }}</span>
        </div>
        <ul class="donut-legend">
          <li><span class="dot creating"></span>creating: {{ statusCounts.creating }}</li>
          <li><span class="dot created"></span>created: {{ statusCounts.created }}</li>
          <li><span class="dot running"></span>running: {{ statusCounts.running }}</li>
          <li><span class="dot stopped"></span>stopped: {{ statusCounts.stopped }}</li>
        </ul>
      </div>
    </article>

    <article class="panel dashboard-pulse">
      <div class="section-head">
        <h3>Runtime Pulse</h3>
        <span class="count">{{ totalPulseBlocks }} blocks</span>
      </div>
      <div class="pulse-sections">
        <div class="pulse-section">
          <div class="pulse-head">
            <span>ReplicaSet</span>
            <span class="count">{{ rsPulseBlocks.length }}</span>
          </div>
          <div class="pulse-stage">
            <div
              v-for="(status, idx) in rsPulseBlocks"
              :key="`pulse-rs-${idx}`"
              :class="['pulse-block', `pulse-${status}`]"
              :style="pulseStyle(idx)"
              @mouseenter="handlePulseHover"
            ></div>
            <p v-if="rsPulseBlocks.length === 0" class="empty pulse-empty">No ReplicaSet</p>
          </div>
        </div>

            <div class="pulse-section">
              <div class="pulse-head">
                <span>Pod</span>
                <span class="count">{{ podPulseBlocks.length }}</span>
              </div>
          <div class="pulse-stage">
            <div
              v-for="(status, idx) in podPulseBlocks"
              :key="`pulse-pod-${idx}`"
              :class="['pulse-block', `pulse-${status}`]"
              :style="pulseStyle(idx)"
              @mouseenter="handlePulseHover"
            ></div>
                <p v-if="podPulseBlocks.length === 0" class="empty pulse-empty">No Pod</p>
              </div>
            </div>

            <div class="pulse-section">
              <div class="pulse-head">
                <span>Bottle</span>
                <span class="count">{{ bottlePulseBlocks.length }}</span>
              </div>
              <div class="pulse-stage">
                <div
                  v-for="(status, idx) in bottlePulseBlocks"
                  :key="`pulse-bottle-${idx}`"
                  :class="['pulse-block', `pulse-${status}`]"
                  :style="pulseStyle(idx)"
                  @mouseenter="handlePulseHover"
                ></div>
                <p v-if="bottlePulseBlocks.length === 0" class="empty pulse-empty">No Bottle</p>
              </div>
            </div>

            <div class="pulse-section">
              <div class="pulse-head">
                <span>Container</span>
                <span class="count">{{ containerPulseBlocks.length }}</span>
              </div>
          <div class="pulse-stage">
            <div
              v-for="(status, idx) in containerPulseBlocks"
              :key="`pulse-container-${idx}`"
              :class="['pulse-block', `pulse-${status}`]"
              :style="pulseStyle(idx)"
              @mouseenter="handlePulseHover"
            ></div>
            <p v-if="containerPulseBlocks.length === 0" class="empty pulse-empty">No container</p>
          </div>
        </div>
      </div>
    </article>

    <article class="panel dashboard-alerts">
      <div class="section-head">
        <h3>Runtime Alerts</h3>
      </div>
      <div class="stats-grid">
        <div class="stat-card">
          <span>Unhealthy ReplicaSets</span>
          <strong>{{ unhealthyReplicasets }}</strong>
        </div>
        <div class="stat-card">
          <span>Non-running Containers</span>
          <strong>{{ nonRunningContainers }}</strong>
        </div>
        <div class="stat-card">
          <span>Pending Policy Changes</span>
          <strong>{{ pendingPolicyChanges }}</strong>
        </div>
      </div>
    </article>

    <article class="panel dashboard-log-insights">
      <div class="section-head">
        <h3>Log Insights (24h)</h3>
      </div>
      <div class="stats-grid">
        <div class="stat-card">
          <span>Audit Events</span>
          <strong>{{ logInsights.audit_total_24h }}</strong>
        </div>
        <div class="stat-card">
          <span>Audit Deny</span>
          <strong>{{ logInsights.audit_deny_24h }}</strong>
        </div>
        <div class="stat-card">
          <span>Network Traffic</span>
          <strong>{{ logInsights.traffic_total_24h }}</strong>
        </div>
        <div class="stat-card">
          <span>Traffic Deny</span>
          <strong>{{ logInsights.traffic_deny_24h }}</strong>
        </div>
        <div class="stat-card">
          <span>DNS Queries</span>
          <strong>{{ logInsights.dns_total_24h }}</strong>
        </div>
        <div class="stat-card">
          <span>DNS Cache Hit Ratio</span>
          <strong>{{
            logInsights.dns_total_24h > 0
              ? `${Math.round((logInsights.dns_cache_hit_24h / (logInsights.dns_cache_hit_24h + logInsights.dns_cache_miss_24h || 1)) * 100)}%`
              : '0%'
          }}</strong>
        </div>
      </div>
    </article>
  </section>
</template>

<script setup>
defineProps({
  runtimeName: { type: String, required: true },
  runtimeVersion: { type: String, required: true },
  connectionStatus: { type: String, required: true },
  connectionStatusClass: { type: String, required: true },
  totalContainers: { type: Number, required: true },
  statusCounts: { type: Object, required: true },
  bottlesCount: { type: Number, required: true },
  podsCount: { type: Number, required: true },
  replicasetsCount: { type: Number, required: true },
  servicesCount: { type: Number, required: true },
  donutStyle: { type: Object, required: true },
  totalPulseBlocks: { type: Number, required: true },
  rsPulseBlocks: { type: Array, required: true },
  podPulseBlocks: { type: Array, required: true },
  bottlePulseBlocks: { type: Array, required: true },
  containerPulseBlocks: { type: Array, required: true },
  pulseStyle: { type: Function, required: true },
  handlePulseHover: { type: Function, required: true },
  unhealthyReplicasets: { type: Number, required: true },
  nonRunningContainers: { type: Number, required: true },
  pendingPolicyChanges: { type: Number, required: true },
  logInsights: { type: Object, required: true }
})
</script>
