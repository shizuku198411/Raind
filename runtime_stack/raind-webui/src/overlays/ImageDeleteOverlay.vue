<template>
  <div v-if="visible" class="overlay" @click.self="onClose">
    <section class="detail-modal confirm-modal">
      <header class="detail-head">
        <h3>Warning</h3>
        <button @click="onClose">Close</button>
      </header>

      <p class="confirm-message">Delete image <code>{{ imageDeleteTarget || '-' }}</code>?</p>
      <div class="confirm-notice notice-warning">
        <strong>Warning</strong>
        <p>This operation may impact containers using this image.</p>
      </div>

      <footer class="modal-actions">
        <button @click="onClose">Cancel</button>
        <button class="danger" :disabled="imageDeleteSubmitting" @click="onSubmit">
          {{ imageDeleteSubmitting ? 'Deleting...' : 'Delete' }}
        </button>
      </footer>
    </section>
  </div>
</template>

<script setup>
defineProps({
  visible: { type: Boolean, required: true },
  imageDeleteTarget: { type: String, required: true },
  imageDeleteSubmitting: { type: Boolean, required: true },
  onClose: { type: Function, required: true },
  onSubmit: { type: Function, required: true }
})
</script>
