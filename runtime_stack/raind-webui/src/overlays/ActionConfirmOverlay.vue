<template>
  <div v-if="visible" class="overlay" @click.self="onClose">
    <section class="detail-modal confirm-modal">
      <header class="detail-head">
        <h3>{{ actionConfirmType === 'stop' ? 'Caution' : 'Warning' }}</h3>
        <button @click="onClose">Close</button>
      </header>

      <p class="confirm-message">
        {{ actionConfirmType === 'stop' ? 'Stop' : 'Delete' }} container
        <code>{{ actionConfirmName || actionConfirmId }}</code>?
      </p>
      <p class="selector">Action: {{ actionConfirmType }} / Target ID: {{ actionConfirmId }}</p>
      <div :class="['confirm-notice', actionConfirmType === 'stop' ? 'notice-caution' : 'notice-warning']">
        <strong>{{ actionConfirmType === 'stop' ? 'Caution' : 'Warning' }}</strong>
        <p v-if="actionConfirmType === 'stop'">
          This operation may impact the service. Dependent traffic or processing may be interrupted.
        </p>
        <p v-else>
          This operation impacts the service, and resources inside the container will be permanently lost.
          This action cannot be undone.
        </p>
      </div>

      <footer class="modal-actions">
        <button @click="onClose">Cancel</button>
        <button :class="actionConfirmType === 'stop' ? 'caution' : 'danger'" :disabled="actionConfirmSubmitting" @click="onSubmit">
          {{ actionConfirmSubmitting ? 'Processing...' : actionConfirmType === 'stop' ? 'Stop' : 'Delete' }}
        </button>
      </footer>
    </section>
  </div>
</template>

<script setup>
defineProps({
  visible: { type: Boolean, required: true },
  actionConfirmType: { type: String, required: true },
  actionConfirmId: { type: String, required: true },
  actionConfirmName: { type: String, required: true },
  actionConfirmSubmitting: { type: Boolean, required: true },
  onClose: { type: Function, required: true },
  onSubmit: { type: Function, required: true }
})
</script>
