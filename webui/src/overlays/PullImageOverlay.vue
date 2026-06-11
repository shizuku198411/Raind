<template>
  <div v-if="visible" class="overlay" @click.self="onClose">
    <section class="detail-modal pull-image-modal">
      <header class="detail-head">
        <h3>Pull Image</h3>
        <button @click="onClose">Close</button>
      </header>

      <div class="create-form">
        <label class="form-item">
          <span>Image *</span>
          <input v-model.trim="pullImageForm.image" type="text" placeholder="e.g. alpine:latest" />
        </label>

        <div class="row-grid row-2">
          <label class="form-item">
            <span>OS (optional)</span>
            <input v-model.trim="pullImageForm.os" type="text" placeholder="e.g. linux" />
          </label>
          <label class="form-item">
            <span>Arch (optional)</span>
            <input v-model.trim="pullImageForm.arch" type="text" placeholder="e.g. amd64" />
          </label>
        </div>
      </div>

      <footer class="modal-actions">
        <button @click="onClose">Cancel</button>
        <button class="primary" :disabled="pullImageSubmitting || !pullImageForm.image" @click="onSubmit">
          {{ pullImageSubmitting ? 'Pulling...' : 'Pull' }}
        </button>
      </footer>
    </section>
  </div>
</template>

<script setup>
defineProps({
  visible: { type: Boolean, required: true },
  pullImageForm: { type: Object, required: true },
  pullImageSubmitting: { type: Boolean, required: true },
  onClose: { type: Function, required: true },
  onSubmit: { type: Function, required: true }
})
</script>
