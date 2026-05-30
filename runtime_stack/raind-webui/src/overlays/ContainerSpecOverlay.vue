<template>
  <div v-if="visible" class="overlay" @click.self="onClose">
    <section class="detail-modal log-modal spec-modal">
      <header class="detail-head">
        <h3>Container Config Spec</h3>
        <button @click="onClose">Close</button>
      </header>

      <div class="detail-meta">
        <p><span>ID:</span> <code>{{ specTargetId }}</code></p>
        <p><span>Name:</span> {{ specTargetName || '-' }}</p>
      </div>

      <div class="log-body spec-body">
        <p class="selector">Source: /etc/raind/container/&lt;containerId&gt;/config.json</p>
        <p v-if="specLoading" class="empty">loading...</p>

        <template v-else-if="specData">
          <div class="detail-sections">
            <article class="detail-card" v-if="generalEntries.length > 0">
              <h4>General</h4>
              <dl>
                <template v-for="item in generalEntries" :key="`general-${item.key}`">
                  <dt class="mono">{{ item.key }}</dt>
                  <dd class="mono multiline">{{ item.value }}</dd>
                </template>
              </dl>
            </article>

            <article class="detail-card" v-if="mountEntries.length > 0">
              <h4>Mounts</h4>
              <div class="flat-groups">
                <div v-for="(mount, idx) in mountEntries" :key="`mount-${idx}`" class="flat-group">
                  <p class="mono flat-title">Mount #{{ idx + 1 }}</p>
                  <dl>
                    <template v-for="item in mount" :key="`mount-${idx}-${item.key}`">
                      <dt class="mono">{{ item.key }}</dt>
                      <dd class="mono multiline">{{ item.value }}</dd>
                    </template>
                  </dl>
                </div>
              </div>
            </article>

            <article class="detail-card" v-if="capabilityEntries.length > 0">
              <h4>Capabilities</h4>
              <dl>
                <template v-for="item in capabilityEntries" :key="`cap-${item.key}`">
                  <dt class="mono">{{ item.key }}</dt>
                  <dd class="mono multiline">{{ item.value }}</dd>
                </template>
              </dl>
            </article>

            <article class="detail-card" v-if="linuxEntries.length > 0">
              <h4>Linux</h4>
              <dl>
                <template v-for="item in linuxEntries" :key="`linux-${item.key}`">
                  <dt class="mono">{{ item.key }}</dt>
                  <dd class="mono multiline">{{ item.value }}</dd>
                </template>
              </dl>
            </article>

            <article class="detail-card" v-if="hookEntries.length > 0">
              <h4>Hooks</h4>
              <div class="flat-groups">
                <div v-for="hook in hookEntries" :key="`hook-${hook.type}`" class="flat-group">
                  <p class="mono flat-title">{{ hook.type }}</p>
                  <div v-if="hook.items.length === 0" class="mono">-</div>
                  <div v-for="(entry, idx) in hook.items" :key="`hook-${hook.type}-${idx}`" class="flat-subgroup">
                    <p class="mono flat-title">Entry #{{ idx + 1 }}</p>
                    <dl>
                      <template v-for="item in entry" :key="`hook-${hook.type}-${idx}-${item.key}`">
                        <dt class="mono">{{ item.key }}</dt>
                        <dd class="mono multiline">{{ item.value }}</dd>
                      </template>
                    </dl>
                  </div>
                </div>
              </div>
            </article>

            <article class="detail-card" v-if="annotationEntries.length > 0">
              <h4>Annotations</h4>
              <dl>
                <template v-for="item in annotationEntries" :key="`annotation-${item.key}`">
                  <dt class="mono">{{ item.key }}</dt>
                  <dd class="mono multiline">{{ item.value }}</dd>
                </template>
              </dl>
            </article>
          </div>

          <details class="raw-json">
            <summary>Raw JSON</summary>
            <pre class="log-text raw-text">{{ rawJson }}</pre>
          </details>
        </template>

        <p v-else class="empty">No spec data</p>
      </div>
    </section>
  </div>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  visible: { type: Boolean, required: true },
  specTargetId: { type: String, required: true },
  specTargetName: { type: String, required: true },
  specLoading: { type: Boolean, required: true },
  specData: { type: Object, default: null },
  onClose: { type: Function, required: true }
})

const rawJson = computed(() => {
  if (!props.specData) return ''
  return JSON.stringify(props.specData, null, 2)
})

const generalEntries = computed(() => {
  const src = props.specData
  if (!src || typeof src !== 'object') return []
  const out = objectEntries(src, new Set(['process', 'root', 'linux', 'mounts', 'hooks', 'annotations']))
  const process = src.process
  if (process && typeof process === 'object') {
    out.push(
      ...objectEntries(process, new Set(['capabilities'])).map((e) => ({
        key: `process.${e.key}`,
        value: e.key === 'args' && Array.isArray(process.args) ? process.args.join(' ') || '-' : e.value
      }))
    )
  }
  const root = src.root
  if (root && typeof root === 'object') {
    out.push(
      ...objectEntries(root).map((e) => ({
        key: `root.${e.key}`,
        value: e.value
      }))
    )
  }
  return out
})

const capabilityEntries = computed(() => {
  const src = props.specData?.process?.capabilities
  if (!src || typeof src !== 'object') return []
  return objectEntries(src)
})

const linuxEntries = computed(() => {
  const src = props.specData?.linux
  if (!src || typeof src !== 'object') return []
  return objectEntries(src)
})

const mountEntries = computed(() => {
  const src = props.specData?.mounts
  if (!Array.isArray(src) || src.length === 0) return []
  return src.map((m) => objectEntries(m || {}))
})

const hookEntries = computed(() => {
  const hooks = props.specData?.hooks
  if (!hooks || typeof hooks !== 'object') return []
  return Object.entries(hooks).map(([type, list]) => ({
    type,
    items: Array.isArray(list) ? list.map((item) => objectEntries(item || {})) : []
  }))
})

const annotationEntries = computed(() => {
  const ann = props.specData?.annotations
  if (!ann || typeof ann !== 'object') return []
  return objectEntries(ann)
})

function objectEntries(obj, exclude = new Set()) {
  if (!obj || typeof obj !== 'object') return []
  return Object.entries(obj)
    .filter(([k]) => !exclude.has(k))
    .map(([key, value]) => ({ key, value: formatValue(value) }))
}

function formatValue(value) {
  const parsed = parseMaybeJson(value)

  if (parsed == null) return '-'
  if (typeof parsed === 'string') return parsed
  if (typeof parsed === 'number' || typeof parsed === 'boolean') return String(parsed)

  if (Array.isArray(parsed)) {
    if (parsed.length === 0) return '-'
    return parsed
      .map((item, idx) => {
        const p = parseMaybeJson(item)
        if (p && typeof p === 'object') {
          const inner = objectEntries(p)
            .map((e) => `${e.key}: ${e.value}`)
            .join(', ')
          return `[${idx}] ${inner}`
        }
        return `[${idx}] ${String(p)}`
      })
      .join('\n')
  }

  const pairs = objectEntries(parsed)
  if (pairs.length === 0) return '-'
  return pairs.map((e) => `${e.key}: ${e.value}`).join('\n')
}

function parseMaybeJson(value) {
  if (typeof value !== 'string') return value
  const t = value.trim()
  if (!t) return ''
  if (!(t.startsWith('{') || t.startsWith('['))) return value
  try {
    return JSON.parse(t)
  } catch {
    return value
  }
}
</script>

<style scoped>
.spec-modal {
  max-height: 86vh;
}

.spec-body {
  max-height: calc(86vh - 130px);
  overflow: auto;
  min-height: 0;
}

.multiline {
  white-space: pre-wrap;
  word-break: break-word;
}

.raw-json {
  margin-top: 10px;
  border: 1px solid #3a4150;
  border-radius: 8px;
  background: #141a22;
  padding: 8px 10px;
}

.raw-json > summary {
  cursor: pointer;
  color: #9fd0ff;
  user-select: none;
}

.raw-json > summary:hover {
  color: #cfe8ff;
}

.raw-text {
  max-height: 36vh;
  overflow: auto;
}

.flat-groups {
  display: grid;
  gap: 8px;
}

.flat-group,
.flat-subgroup {
  padding-top: 2px;
}

.flat-subgroup {
  margin-top: 8px;
}

.flat-title {
  margin: 0 0 6px;
}
</style>
