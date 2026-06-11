<template>
  <div v-if="visible" class="overlay" @click.self="onClose">
    <section class="detail-modal pull-image-modal">
      <header class="detail-head">
        <h3>{{ title }}</h3>
        <button @click="onClose">Close</button>
      </header>

      <div class="create-form">
        <label class="form-item">
          <span>YAML</span>
          <textarea
            class="yaml-input"
            :value="yamlText"
            placeholder="Paste YAML here"
            @input="onUpdateYaml($event.target.value)"
          ></textarea>
        </label>
        <p v-if="!valid" class="error yaml-error">{{ validationMessage }}</p>
      </div>

      <footer class="modal-actions">
        <button @click="onClose">Cancel</button>
        <button class="primary" :disabled="submitting || !valid" @click="onSubmit">
          {{ submitting ? 'Creating...' : 'Create' }}
        </button>
      </footer>
    </section>
  </div>
</template>

<script setup>
defineProps({
  visible: { type: Boolean, required: true },
  title: { type: String, required: true },
  yamlText: { type: String, required: true },
  submitting: { type: Boolean, required: true },
  valid: { type: Boolean, required: true },
  validationMessage: { type: String, required: true },
  onClose: { type: Function, required: true },
  onUpdateYaml: { type: Function, required: true },
  onSubmit: { type: Function, required: true }
})
</script>
