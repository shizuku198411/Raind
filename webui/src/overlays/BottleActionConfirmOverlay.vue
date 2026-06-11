<template>
  <div v-if="visible" class="overlay" @click.self="onClose">
    <section class="detail-modal confirm-modal">
      <header class="detail-head">
        <h3>{{ actionType === 'stop' ? 'Caution' : 'Warning' }}</h3>
        <button @click="onClose">Close</button>
      </header>

      <p class="confirm-message">
        {{ actionType === 'stop' ? 'Stop' : 'Delete' }} bottle
        <code>{{ targetName || targetId }}</code>?
      </p>
      <p class="selector">Action: {{ actionType }} / Target ID: {{ targetId }}</p>
      <div :class="['confirm-notice', actionType === 'stop' ? 'notice-caution' : 'notice-warning']">
        <strong>{{ actionType === 'stop' ? 'Caution' : 'Warning' }}</strong>
        <p v-if="actionType === 'stop'">
          This operation may impact the service. Dependent traffic or processing may be interrupted.
        </p>
        <p v-else>
          This operation impacts the service, and resources inside the bottle containers will be permanently lost.
          This action cannot be undone.
        </p>
      </div>

      <footer class="modal-actions">
        <button @click="onClose">Cancel</button>
        <button :class="actionType === 'stop' ? 'caution' : 'danger'" :disabled="submitting" @click="onSubmit">
          {{ submitting ? 'Processing...' : actionType === 'stop' ? 'Stop' : 'Delete' }}
        </button>
      </footer>
    </section>
  </div>
</template>

<script setup>
defineProps({
  visible: { type: Boolean, required: true },
  actionType: { type: String, required: true },
  targetId: { type: String, required: true },
  targetName: { type: String, required: true },
  submitting: { type: Boolean, required: true },
  onClose: { type: Function, required: true },
  onSubmit: { type: Function, required: true }
})
</script>
