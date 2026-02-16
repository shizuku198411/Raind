<template>
  <div v-if="visible" class="overlay" @click.self="onClose">
    <section class="detail-modal confirm-modal">
      <header class="detail-head">
        <h3>Caution</h3>
        <button @click="onClose">Close</button>
      </header>

      <div class="confirm-notice notice-caution">
        <strong>Caution</strong>
        <p>This operation will affect the container network.</p>
        <p>Are you sure you want to commit?</p>
      </div>

      <footer class="modal-actions">
        <button @click="onClose">Cancel</button>
        <button class="caution" :disabled="policyCommitSubmitting" @click="onSubmit">
          {{ policyCommitSubmitting ? 'Committing...' : 'Commit' }}
        </button>
      </footer>
    </section>
  </div>
</template>

<script setup>
defineProps({
  visible: { type: Boolean, required: true },
  policyCommitSubmitting: { type: Boolean, required: true },
  onClose: { type: Function, required: true },
  onSubmit: { type: Function, required: true }
})
</script>
