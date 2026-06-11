<template>
  <div v-if="visible" class="overlay" @click.self="onClose">
    <section class="detail-modal log-modal">
      <header class="detail-head">
        <h3>Container Log</h3>
        <button @click="onClose">Close</button>
      </header>

      <div class="detail-meta">
        <p><span>ID:</span> <code>{{ logTargetId }}</code></p>
        <p><span>Name:</span> {{ logTargetName || '-' }}</p>
      </div>

      <div class="log-body">
        <p class="selector">Source: {{ logSourcePath || '-' }}</p>
        <p v-if="logLoading" class="empty">loading...</p>
        <pre v-else class="log-text">{{ logData || 'No log output' }}</pre>
      </div>
    </section>
  </div>
</template>

<script setup>
defineProps({
  visible: { type: Boolean, required: true },
  logTargetId: { type: String, required: true },
  logTargetName: { type: String, required: true },
  logSourcePath: { type: String, required: true },
  logLoading: { type: Boolean, required: true },
  logData: { type: String, required: true },
  onClose: { type: Function, required: true }
})
</script>
