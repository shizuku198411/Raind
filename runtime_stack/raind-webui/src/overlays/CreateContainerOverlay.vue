<template>
  <div v-if="visible" class="overlay" @click.self="onClose">
    <section class="detail-modal create-modal">
      <header class="detail-head">
        <h3>Create Container</h3>
        <button @click="onClose">Close</button>
      </header>

      <div class="create-form">
        <label class="form-item">
          <span>Container Name (optional)</span>
          <input v-model.trim="createForm.name" type="text" placeholder="my-container" />
        </label>

        <label class="form-item">
          <span>Image *</span>
          <div class="image-combobox">
            <input
              :value="imageFilter"
              type="text"
              class="image-search"
              placeholder="Search image (e.g. alpine)"
              @focus="onOpenImageDropdown"
              @input="onImageFilterInput($event.target.value)"
              @blur="onScheduleCloseImageDropdown"
            />
            <button type="button" class="combo-toggle" @mousedown.prevent @click="onToggleImageDropdown">
              ▾
            </button>
            <div v-if="imageDropdownOpen" class="image-dropdown" @mousedown.prevent>
              <button
                v-for="img in filteredImageOptions"
                :key="img"
                type="button"
                class="image-option"
                @click="onSelectImageOption(img)"
              >
                {{ img }}
              </button>
              <p v-if="filteredImageOptions.length === 0" class="empty image-empty">No matched image</p>
            </div>
          </div>
        </label>

        <div class="form-item">
          <span>TTY</span>
          <label class="tty-toggle">
            <input v-model="createForm.tty" type="checkbox" />
            <span class="tty-slider"></span>
            <span class="tty-label">{{ createForm.tty ? 'true' : 'false' }}</span>
          </label>
        </div>

        <section class="create-group">
          <div class="group-head">
            <h4>Ports (optional)</h4>
            <button @click="onAddPortRow">Add</button>
          </div>
          <div v-if="createForm.ports.length === 0" class="empty">No ports</div>
          <div v-for="(row, idx) in createForm.ports" :key="`port-${idx}`" class="row-grid row-3">
            <input v-model.trim="row.host" type="text" placeholder="host (e.g. 18080)" />
            <input v-model.trim="row.target" type="text" placeholder="target (e.g. 8080)" />
            <div class="inline-actions">
              <select v-model="row.protocol">
                <option value="tcp">tcp</option>
                <option value="udp">udp</option>
              </select>
              <button class="danger" @click="onRemovePortRow(idx)">Remove</button>
            </div>
          </div>
        </section>

        <section class="create-group">
          <div class="group-head">
            <h4>Mounts (optional)</h4>
            <button @click="onAddMountRow">Add</button>
          </div>
          <div v-if="createForm.mounts.length === 0" class="empty">No mounts</div>
          <div v-for="(row, idx) in createForm.mounts" :key="`mount-${idx}`" class="row-grid row-2">
            <input v-model.trim="row.host" type="text" placeholder="host path" />
            <div class="inline-actions">
              <input v-model.trim="row.target" type="text" placeholder="container path" />
              <button class="danger" @click="onRemoveMountRow(idx)">Remove</button>
            </div>
          </div>
        </section>

        <section class="create-group">
          <div class="group-head">
            <h4>Env (optional)</h4>
            <button @click="onAddEnvRow">Add</button>
          </div>
          <div v-if="createForm.envs.length === 0" class="empty">No env</div>
          <div v-for="(row, idx) in createForm.envs" :key="`env-${idx}`" class="row-grid row-2">
            <input v-model.trim="row.key" type="text" placeholder="key" />
            <div class="inline-actions">
              <input v-model.trim="row.value" type="text" placeholder="value" />
              <button class="danger" @click="onRemoveEnvRow(idx)">Remove</button>
            </div>
          </div>
        </section>
      </div>

      <footer class="modal-actions">
        <button @click="onClose">Cancel</button>
        <button class="primary" :disabled="createSubmitting" @click="onSubmit">
          {{ createSubmitting ? 'Creating...' : 'Create' }}
        </button>
      </footer>
    </section>
  </div>
</template>

<script setup>
defineProps({
  visible: { type: Boolean, required: true },
  createForm: { type: Object, required: true },
  imageFilter: { type: String, required: true },
  imageDropdownOpen: { type: Boolean, required: true },
  filteredImageOptions: { type: Array, required: true },
  createSubmitting: { type: Boolean, required: true },
  onClose: { type: Function, required: true },
  onImageFilterInput: { type: Function, required: true },
  onOpenImageDropdown: { type: Function, required: true },
  onScheduleCloseImageDropdown: { type: Function, required: true },
  onToggleImageDropdown: { type: Function, required: true },
  onSelectImageOption: { type: Function, required: true },
  onAddPortRow: { type: Function, required: true },
  onRemovePortRow: { type: Function, required: true },
  onAddMountRow: { type: Function, required: true },
  onRemoveMountRow: { type: Function, required: true },
  onAddEnvRow: { type: Function, required: true },
  onRemoveEnvRow: { type: Function, required: true },
  onSubmit: { type: Function, required: true }
})
</script>
