<template>
  <div v-if="visible" class="overlay" @click.self="onClose">
    <section class="detail-modal">
      <header class="detail-head">
        <h3>Resource Detail ({{ resourceDetailType }})</h3>
        <button @click="onClose">Close</button>
      </header>

      <div class="detail-meta">
        <p><span>ID:</span> <code>{{ resourceDetailId }}</code></p>
        <p><span>Status:</span> {{ resourceDetailLoading ? 'loading...' : 'loaded' }}</p>
      </div>

      <div v-if="resourceDetailData" class="detail-sections">
        <article class="detail-card">
          <h4>Basic</h4>
          <dl>
            <dt>Name</dt><dd>{{ resourceField('name') }}</dd>
            <dt>Namespace</dt><dd>{{ resourceField('namespace') }}</dd>
            <dt>Created At</dt><dd>{{ formatTime(resourceFieldRaw('createdAt')) }}</dd>
            <dt>Selector</dt><dd class="mono">{{ selectorText(resourceFieldRaw('selector')) }}</dd>
          </dl>
        </article>

        <article class="detail-card" v-if="resourceDetailType === 'replicaset'">
          <h4>ReplicaSet</h4>
          <dl>
            <dt>Replicas</dt><dd>{{ resourceField('replicas') }}</dd>
            <dt>Desired</dt><dd>{{ resourceField('desired') }}</dd>
            <dt>Current</dt><dd>{{ resourceField('current') }}</dd>
            <dt>Ready</dt><dd>{{ resourceField('ready') }}</dd>
            <dt>Template ID</dt><dd class="mono">{{ resourceField('templateId') }}</dd>
          </dl>
        </article>

        <article class="detail-card" v-if="resourceDetailType === 'pod'">
          <h4>Pod</h4>
          <dl>
            <dt>Pod ID</dt><dd class="mono">{{ resourceField('podId') }}</dd>
            <dt>State</dt><dd>{{ resourceField('state') }}</dd>
            <dt>Owner PID</dt><dd>{{ resourceField('ownerPid') }}</dd>
            <dt>Template ID</dt><dd class="mono">{{ resourceField('templateId') }}</dd>
          </dl>
        </article>

        <article class="detail-card" v-if="resourceDetailType === 'service'">
          <h4>Service</h4>
          <dl>
            <dt>Service ID</dt><dd class="mono">{{ resourceField('serviceId') }}</dd>
            <dt>Ports</dt><dd class="mono">{{ servicePortsText(resourceFieldRaw('ports')) }}</dd>
          </dl>
        </article>

        <article class="detail-card">
          <h4>Labels / Annotations</h4>
          <dl>
            <dt>Labels</dt><dd class="mono">{{ objectText(resourceFieldRaw('labels')) }}</dd>
            <dt>Annotations</dt><dd class="mono">{{ objectText(resourceFieldRaw('annotations')) }}</dd>
          </dl>
        </article>
      </div>
      <p v-else-if="!resourceDetailLoading" class="empty">No detail data</p>
    </section>
  </div>
</template>

<script setup>
defineProps({
  visible: { type: Boolean, required: true },
  resourceDetailType: { type: String, required: true },
  resourceDetailId: { type: String, required: true },
  resourceDetailLoading: { type: Boolean, required: true },
  resourceDetailData: { type: Object, default: null },
  resourceField: { type: Function, required: true },
  resourceFieldRaw: { type: Function, required: true },
  formatTime: { type: Function, required: true },
  selectorText: { type: Function, required: true },
  servicePortsText: { type: Function, required: true },
  objectText: { type: Function, required: true },
  onClose: { type: Function, required: true }
})
</script>
