<template>
  <div v-if="visible" class="overlay" @click.self="onClose">
    <section class="detail-modal attach-modal">
      <header class="detail-head">
        <h3>Container Attach (TTY)</h3>
        <button @click="onClose">Close</button>
      </header>

      <div class="detail-meta">
        <p><span>ID:</span> <code>{{ attachTargetId }}</code></p>
        <p><span>Name:</span> {{ attachTargetName || '-' }}</p>
      </div>

      <p class="selector">
        Status:
        {{
          attachConnecting
            ? 'connecting...'
            : attachConnected
              ? 'connected'
              : attachError
                ? `error (${attachError})`
                : 'closed'
        }}
      </p>

      <div :ref="setTerminalEl" class="attach-terminal" tabindex="0" @click="onFocusTerminal"></div>
    </section>
  </div>
</template>

<script setup>
defineProps({
  visible: { type: Boolean, required: true },
  attachTargetId: { type: String, required: true },
  attachTargetName: { type: String, required: true },
  attachConnecting: { type: Boolean, required: true },
  attachConnected: { type: Boolean, required: true },
  attachError: { type: String, required: true },
  onClose: { type: Function, required: true },
  onFocusTerminal: { type: Function, required: true },
  setTerminalEl: { type: Function, required: true }
})
</script>
