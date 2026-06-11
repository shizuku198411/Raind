<template>
  <div v-if="visible" class="overlay" @click.self="onClose">
    <section class="detail-modal">
      <header class="detail-head">
        <h3>Container Detail</h3>
        <button @click="onClose">Close</button>
      </header>

      <div class="detail-meta">
        <p><span>ID:</span> <code>{{ detailTargetId }}</code></p>
        <p><span>Status:</span> {{ detailLoading ? 'loading...' : 'loaded' }}</p>
      </div>

      <div v-if="detailData" class="detail-sections">
        <article class="detail-card">
          <h4>Basic</h4>
          <dl>
            <dt>Name</dt><dd>{{ detailData.container_name || '-' }}</dd>
            <dt>Container ID</dt><dd class="mono">{{ detailData.container_id || '-' }}</dd>
            <dt>Status</dt><dd>{{ detailData.status || '-' }}</dd>
            <dt>PID</dt><dd>{{ detailData.pid ?? '-' }}</dd>
            <dt>Image</dt><dd>{{ formatImage(detailData) }}</dd>
            <dt>Command</dt><dd class="mono">{{ formatCommand(detailData.command) }}</dd>
          </dl>
        </article>

        <article class="detail-card">
          <h4>Runtime</h4>
          <dl>
            <dt>Created</dt><dd>{{ formatTime(detailData.created_at) }}</dd>
            <dt>Started</dt><dd>{{ formatTime(detailData.started_at) }}</dd>
            <dt>Stopped</dt><dd>{{ formatTime(detailData.stopped_at) }}</dd>
            <dt>Exit Code</dt><dd>{{ runtimeExitCode(detailData) }}</dd>
            <dt>Reason</dt><dd>{{ runtimeReason(detailData) }}</dd>
            <dt>Message</dt><dd>{{ runtimeMessage(detailData) }}</dd>
          </dl>
        </article>

        <article class="detail-card">
          <h4>CPU</h4>
          <dl>
            <dt>CPU %</dt><dd>{{ formatPercent(detailData.cpu_percent) }}</dd>
            <dt>Usage</dt><dd>{{ detailData.cpu_usage_usec ?? 0 }} usec</dd>
            <dt>User</dt><dd>{{ detailData.cpu_user_usec ?? 0 }} usec</dd>
            <dt>System</dt><dd>{{ detailData.cpu_system_usec ?? 0 }} usec</dd>
            <dt>Throttled</dt><dd>{{ detailData.cpu_throttled_usec ?? 0 }} usec</dd>
          </dl>
        </article>

        <article class="detail-card">
          <h4>Memory</h4>
          <dl>
            <dt>Memory %</dt><dd>{{ formatPercent(detailData.memory_percent) }}</dd>
            <dt>Current</dt><dd>{{ formatBytes(detailData.memory_current_bytes) }}</dd>
            <dt>Max</dt><dd>{{ formatBytes(detailData.memory_max_bytes) }}</dd>
            <dt>Limited</dt><dd>{{ detailData.memory_limited ? 'yes' : 'no' }}</dd>
            <dt>OOM</dt><dd>{{ detailData.memory_oom ?? 0 }}</dd>
            <dt>OOM Kill</dt><dd>{{ detailData.memory_oom_kill ?? 0 }}</dd>
          </dl>
        </article>

        <article class="detail-card">
          <h4>I/O</h4>
          <dl>
            <dt>Read Bytes</dt><dd>{{ formatBytes(detailData.io_read_bytes) }}</dd>
            <dt>Write Bytes</dt><dd>{{ formatBytes(detailData.io_write_bytes) }}</dd>
            <dt>Read Ops</dt><dd>{{ detailData.io_read_ops ?? 0 }}</dd>
            <dt>Write Ops</dt><dd>{{ detailData.io_write_ops ?? 0 }}</dd>
          </dl>
        </article>

        <article class="detail-card">
          <h4>Meta</h4>
          <dl>
            <dt>Pod ID</dt>
            <dd class="mono">
              <button
                v-if="detailData.pod_id"
                class="link-btn mono"
                @click="onOpenPodDetail(detailData.pod_id)"
              >
                {{ detailData.pod_id }}
              </button>
              <span v-else>-</span>
            </dd>
            <dt>Bottle ID</dt>
            <dd class="mono">
              <button
                v-if="detailData.bottle_id"
                class="link-btn mono"
                @click="onOpenBottleDetail(detailData.bottle_id)"
              >
                {{ detailData.bottle_id }}
              </button>
              <span v-else>-</span>
            </dd>
            <dt>SPIFFE ID</dt><dd class="mono">{{ detailData.spiffe_id || '-' }}</dd>
            <dt>Cgroup</dt><dd class="mono">{{ detailData.cgroup_path || '-' }}</dd>
            <dt>Log Path</dt><dd class="mono">{{ detailData.log_path || '-' }}</dd>
            <dt>Tty</dt><dd>{{ detailData.tty ? 'true' : 'false' }}</dd>
          </dl>
        </article>
      </div>
      <p v-else-if="!detailLoading" class="empty">No detail data</p>
    </section>
  </div>
</template>

<script setup>
defineProps({
  visible: { type: Boolean, required: true },
  detailTargetId: { type: String, required: true },
  detailLoading: { type: Boolean, required: true },
  detailData: { type: Object, default: null },
  formatTime: { type: Function, required: true },
  formatImage: { type: Function, required: true },
  formatCommand: { type: Function, required: true },
  runtimeExitCode: { type: Function, required: true },
  runtimeReason: { type: Function, required: true },
  runtimeMessage: { type: Function, required: true },
  formatPercent: { type: Function, required: true },
  formatBytes: { type: Function, required: true },
  onOpenPodDetail: { type: Function, required: true },
  onOpenBottleDetail: { type: Function, required: true },
  onClose: { type: Function, required: true }
})
</script>

<style scoped>
.link-btn {
  border: none;
  background: transparent;
  color: #9fd0ff;
  cursor: pointer;
  padding: 0;
  text-decoration: underline;
}

.link-btn:hover {
  color: #cfe8ff;
}
</style>
