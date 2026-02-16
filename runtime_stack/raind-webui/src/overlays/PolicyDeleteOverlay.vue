<template>
  <div v-if="visible" class="overlay" @click.self="onClose">
    <section class="detail-modal confirm-modal">
      <header class="detail-head">
        <h3>Warning</h3>
        <button @click="onClose">Close</button>
      </header>

      <p class="confirm-message">Delete policy <code>{{ policyDeleteId || '-' }}</code>?</p>
      <p class="selector">Chain: {{ policyDeleteChain || '-' }}</p>
      <div class="confirm-notice notice-warning">
        <strong>Warning</strong>
        <p>This operation may impact service communication and traffic control behavior.</p>
      </div>

      <footer class="modal-actions">
        <button @click="onClose">Cancel</button>
        <button class="danger" :disabled="policyDeleteSubmitting" @click="onSubmit">
          {{ policyDeleteSubmitting ? 'Deleting...' : 'Delete' }}
        </button>
      </footer>
    </section>
  </div>
</template>

<script setup>
defineProps({
  visible: { type: Boolean, required: true },
  policyDeleteSubmitting: { type: Boolean, required: true },
  policyDeleteId: { type: String, required: true },
  policyDeleteChain: { type: String, required: true },
  onClose: { type: Function, required: true },
  onSubmit: { type: Function, required: true }
})
</script>
