<template>
  <div v-if="visible" class="overlay" @click.self="onClose">
    <section class="detail-modal pull-image-modal">
      <header class="detail-head">
        <h3>Create Policy</h3>
        <button @click="onClose">Close</button>
      </header>

      <div class="create-form">
        <label class="form-item">
          <span>Policy Type</span>
          <select :value="policyCreateForm.type" @change="updateField('type', $event.target.value)">
            <option value="RAIND-EW">Inter Connect</option>
            <option value="RAIND-NS-OBS">External (Observe)</option>
            <option value="RAIND-NS-ENF">External (Enforce)</option>
          </select>
        </label>

        <label class="form-item">
          <span>Source Container</span>
          <div class="image-combobox">
            <input
              :value="sourceQuery"
              type="text"
              class="image-search"
              placeholder="Search container name"
              @input="onSourceInput($event.target.value)"
              @focus="sourceOpen = true"
              @blur="scheduleCloseSource"
            />
            <button type="button" class="combo-toggle" @mousedown.prevent @click="sourceOpen = !sourceOpen">▾</button>
            <div v-if="sourceOpen" class="image-dropdown" @mousedown.prevent>
              <button
                v-for="name in sourceOptions"
                :key="name"
                type="button"
                class="image-option"
                @click="selectSource(name)"
              >
                {{ name }}
              </button>
              <p v-if="sourceOptions.length === 0" class="empty image-empty">No matched container</p>
            </div>
          </div>
        </label>

        <label v-if="isInterConnect" class="form-item">
          <span>Destination Container</span>
          <div class="image-combobox">
            <input
              :value="destContainerQuery"
              type="text"
              class="image-search"
              placeholder="Search destination container"
              @input="onDestContainerInput($event.target.value)"
              @focus="destContainerOpen = true"
              @blur="scheduleCloseDestContainer"
            />
            <button type="button" class="combo-toggle" @mousedown.prevent @click="destContainerOpen = !destContainerOpen">▾</button>
            <div v-if="destContainerOpen" class="image-dropdown" @mousedown.prevent>
              <button
                v-for="name in destContainerOptions"
                :key="`dst-${name}`"
                type="button"
                class="image-option"
                @click="selectDestContainer(name)"
              >
                {{ name }}
              </button>
              <p v-if="destContainerOptions.length === 0" class="empty image-empty">No matched container</p>
            </div>
          </div>
        </label>

        <label v-else class="form-item">
          <span>Destination IP Address</span>
          <input
            :value="policyCreateForm.destinationAddress"
            type="text"
            placeholder="e.g. 10.0.0.10"
            @input="updateField('destinationAddress', $event.target.value)"
          />
        </label>

        <div class="row-grid row-2">
          <label class="form-item">
            <span>Protocol</span>
            <select :value="policyCreateForm.protocol" @change="updateField('protocol', $event.target.value)">
              <option value="tcp">tcp</option>
              <option value="udp">udp</option>
              <option value="icmp">icmp</option>
            </select>
          </label>

          <label v-if="showDport" class="form-item">
            <span>Destination Port</span>
            <input
              :value="policyCreateForm.dport"
              type="number"
              min="1"
              max="65535"
              placeholder="e.g. 443"
              @input="updateField('dport', $event.target.value)"
            />
          </label>
        </div>

        <label class="form-item">
          <span>Comment</span>
          <input
            :value="policyCreateForm.comment"
            type="text"
            placeholder="optional"
            @input="updateField('comment', $event.target.value)"
          />
        </label>
      </div>

      <footer class="modal-actions">
        <button @click="onClose">Cancel</button>
        <button class="primary" :disabled="policyCreateSubmitting" @click="onSubmit">
          {{ policyCreateSubmitting ? 'Creating...' : 'Create' }}
        </button>
      </footer>
    </section>
  </div>
</template>

<script setup>
import { computed, ref, watch } from 'vue'

const props = defineProps({
  visible: { type: Boolean, required: true },
  policyCreateForm: { type: Object, required: true },
  policyCreateSubmitting: { type: Boolean, required: true },
  containerNameOptions: { type: Array, required: true },
  onClose: { type: Function, required: true },
  onSubmit: { type: Function, required: true },
  onUpdateForm: { type: Function, required: true }
})

const sourceQuery = ref('')
const sourceOpen = ref(false)
const destContainerQuery = ref('')
const destContainerOpen = ref(false)
let sourceCloseTimer = null
let destCloseTimer = null

const isInterConnect = computed(() => props.policyCreateForm.type === 'RAIND-EW')
const showDport = computed(() => String(props.policyCreateForm.protocol || 'tcp').toLowerCase() !== 'icmp')

watch(
  () => props.visible,
  (visible) => {
    if (!visible) return
    sourceQuery.value = String(props.policyCreateForm.source || '')
    destContainerQuery.value = String(props.policyCreateForm.destinationContainer || '')
    sourceOpen.value = false
    destContainerOpen.value = false
  }
)

watch(
  () => props.policyCreateForm.type,
  (type) => {
    if (type === 'RAIND-EW') {
      props.onUpdateForm({ destinationAddress: '' })
      return
    }
    props.onUpdateForm({ destinationContainer: '' })
    destContainerQuery.value = ''
  }
)

watch(
  () => props.policyCreateForm.protocol,
  (protocol) => {
    if (String(protocol || '').toLowerCase() === 'icmp') {
      props.onUpdateForm({ dport: '' })
    }
  }
)

const sourceOptions = computed(() => {
  const q = sourceQuery.value.trim().toLowerCase()
  if (!q) return props.containerNameOptions
  return props.containerNameOptions.filter((name) => String(name).toLowerCase().includes(q))
})

const destContainerOptions = computed(() => {
  const q = destContainerQuery.value.trim().toLowerCase()
  if (!q) return props.containerNameOptions
  return props.containerNameOptions.filter((name) => String(name).toLowerCase().includes(q))
})

function updateField(key, value) {
  props.onUpdateForm({ [key]: value })
}

function onSourceInput(value) {
  sourceQuery.value = value
  props.onUpdateForm({ source: value })
  sourceOpen.value = true
}

function onDestContainerInput(value) {
  destContainerQuery.value = value
  props.onUpdateForm({ destinationContainer: value })
  destContainerOpen.value = true
}

function selectSource(name) {
  sourceQuery.value = name
  props.onUpdateForm({ source: name })
  sourceOpen.value = false
}

function selectDestContainer(name) {
  destContainerQuery.value = name
  props.onUpdateForm({ destinationContainer: name })
  destContainerOpen.value = false
}

function scheduleCloseSource() {
  if (sourceCloseTimer) clearTimeout(sourceCloseTimer)
  sourceCloseTimer = setTimeout(() => {
    sourceOpen.value = false
    sourceCloseTimer = null
  }, 120)
}

function scheduleCloseDestContainer() {
  if (destCloseTimer) clearTimeout(destCloseTimer)
  destCloseTimer = setTimeout(() => {
    destContainerOpen.value = false
    destCloseTimer = null
  }, 120)
}
</script>
