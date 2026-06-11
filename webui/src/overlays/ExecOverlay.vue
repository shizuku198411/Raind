<template>
  <div v-if="visible" class="overlay" @click.self="onClose">
    <section class="detail-modal exec-modal">
      <header class="detail-head">
        <h3>Container Exec</h3>
        <button @click="onClose">Close</button>
      </header>

      <div class="detail-meta">
        <p><span>ID:</span> <code>{{ execTargetId }}</code></p>
        <p><span>Name:</span> {{ execTargetName || '-' }}</p>
      </div>

      <div class="exec-form">
        <label class="form-item">
          <span>Command</span>
          <input :value="execCommandText" type="text" placeholder="e.g. /bin/sh" @input="onInputCommand($event.target.value)" />
        </label>
        <div class="form-item">
          <span>TTY</span>
          <label class="tty-toggle">
            <input :checked="execTty" type="checkbox" @change="onChangeTty($event.target.checked)" />
            <span class="tty-slider"></span>
            <span class="tty-label">{{ execTty ? 'true' : 'false' }}</span>
          </label>
        </div>
        <div class="modal-actions">
          <button class="primary" :disabled="execSubmitting || !execCommandReady" @click="onSubmit">
            {{ execSubmitting ? 'Executing...' : 'Exec' }}
          </button>
        </div>
      </div>

      <p class="selector">
        Status:
        {{
          execSubmitting
            ? 'executing...'
            : execTty
              ? execConnecting
                ? 'connecting...'
                : execConnected
                  ? 'connected'
                  : execError
                    ? `error (${execError})`
                    : 'idle'
              : execResult || 'idle'
        }}
      </p>

      <div v-if="execTty" :ref="setTerminalEl" class="attach-terminal exec-terminal" @click="onFocusTerminal"></div>
    </section>
  </div>
</template>

<script setup>
defineProps({
  visible: { type: Boolean, required: true },
  execTargetId: { type: String, required: true },
  execTargetName: { type: String, required: true },
  execCommandText: { type: String, required: true },
  execTty: { type: Boolean, required: true },
  execCommandReady: { type: Boolean, required: true },
  execSubmitting: { type: Boolean, required: true },
  execConnecting: { type: Boolean, required: true },
  execConnected: { type: Boolean, required: true },
  execError: { type: String, required: true },
  execResult: { type: String, required: true },
  onClose: { type: Function, required: true },
  onSubmit: { type: Function, required: true },
  onInputCommand: { type: Function, required: true },
  onChangeTty: { type: Function, required: true },
  onFocusTerminal: { type: Function, required: true },
  setTerminalEl: { type: Function, required: true }
})
</script>
