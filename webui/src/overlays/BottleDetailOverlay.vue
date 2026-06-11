<template>
  <div v-if="visible" class="overlay" @click.self="onClose">
    <section class="detail-modal">
      <header class="detail-head">
        <h3>Bottle Detail</h3>
        <button @click="onClose">Close</button>
      </header>

      <div class="detail-meta">
        <p><span>ID:</span> <code>{{ bottleDetailId }}</code></p>
        <p><span>Status:</span> {{ bottleDetailLoading ? 'loading...' : 'loaded' }}</p>
      </div>

      <div v-if="bottleDetailData" class="detail-sections">
        <article class="detail-card">
          <h4>Basic</h4>
          <dl>
            <dt>Bottle ID</dt><dd class="mono">{{ bottleField('bottleId') }}</dd>
            <dt>Name</dt><dd>{{ bottleField('bottleName') }}</dd>
            <dt>Network</dt><dd>{{ bottleField('network') }}</dd>
            <dt>Network Auto</dt><dd>{{ bottleField('networkAuto') }}</dd>
            <dt>Created At</dt><dd>{{ formatTime(bottleFieldRaw('createdAt')) }}</dd>
          </dl>
        </article>

        <article class="detail-card">
          <h4>Start Order</h4>
          <p class="mono">{{ startOrderText }}</p>
        </article>

        <article class="detail-card">
          <h4>Services</h4>
          <p class="mono">{{ servicesText }}</p>
        </article>

        <article class="detail-card">
          <h4>Containers</h4>
          <p class="mono">{{ containersText }}</p>
        </article>
      </div>
      <p v-else-if="!bottleDetailLoading" class="empty">No detail data</p>
    </section>
  </div>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  visible: { type: Boolean, required: true },
  bottleDetailId: { type: String, required: true },
  bottleDetailLoading: { type: Boolean, required: true },
  bottleDetailData: { type: Object, default: null },
  formatTime: { type: Function, required: true },
  onClose: { type: Function, required: true }
})

function bottleFieldRaw(key) {
  return props.bottleDetailData?.[key]
}

function bottleField(key) {
  const v = bottleFieldRaw(key)
  if (v == null || v === '') return '-'
  if (typeof v === 'boolean') return v ? 'true' : 'false'
  return String(v)
}

const startOrderText = computed(() => {
  const list = props.bottleDetailData?.startOrder
  if (!Array.isArray(list) || list.length === 0) return '-'
  return list.join(' -> ')
})

const servicesText = computed(() => {
  const services = props.bottleDetailData?.services
  if (!services || typeof services !== 'object') return '-'
  const lines = Object.entries(services).map(([name, svc]) => {
    const image = String(svc?.image || '-')
    const tty = svc?.tty ? 'tty=true' : 'tty=false'
    return `${name}: ${image} (${tty})`
  })
  return lines.length > 0 ? lines.join('\n') : '-'
})

const containersText = computed(() => {
  const containers = props.bottleDetailData?.containers
  if (!containers || typeof containers !== 'object') return '-'
  const lines = Object.entries(containers).map(([name, c]) => {
    const id = String(c?.containerId || '-')
    const state = String(c?.state || '-')
    const imageRepo = String(c?.imageRepository || '')
    const imageRef = String(c?.imageReference || '')
    const image = imageRepo || imageRef ? `${imageRepo}:${imageRef}` : '-'
    return `${name}: ${id} (${state}) ${image}`
  })
  return lines.length > 0 ? lines.join('\n') : '-'
})
</script>
