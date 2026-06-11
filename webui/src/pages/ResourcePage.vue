<template>
  <section class="resource-grid">
    <article class="panel relation-panel">
      <div class="section-head">
        <div class="container-head-left">
          <h3>Resource Relations (ReplicaSet -> Pod -> Service)</h3>
          <button class="primary-outline" @click="openResourceCreateOverlay">Create</button>
        </div>
        <span class="count">{{ resourceRelations.length }}</span>
      </div>

      <div class="relation-scroller">
        <div class="relation-list">
          <div v-for="row in resourceRelations" :key="row.replicaset.id" class="relation-row">
            <div class="relation-block">
              <p class="relation-title">ReplicaSet</p>
              <strong>{{ row.replicaset.name }}</strong>
              <span class="mono">{{ row.replicaset.id }}</span>
              <span :class="replicasetHealthClass(row.replicaset.desired, row.replicaset.ready)">
                <span class="status-lamp"></span>
                {{ replicasetHealthLabel(row.replicaset.desired, row.replicaset.ready) }}
              </span>
              <span class="selector">desired/current/ready: {{ row.replicaset.desired }} / {{ row.replicaset.current }} / {{ row.replicaset.ready }}</span>
              <span class="selector">selector: {{ selectorText(row.replicaset.selector) }}</span>
              <button class="tiny-btn" @click="openResourceDetail('replicaset', row.replicaset.id)">Detail</button>
            </div>

            <div class="relation-arrow" aria-hidden="true"></div>

            <div class="relation-block">
              <p class="relation-title">Pod ({{ row.pods.length }})</p>
              <ul class="relation-items">
                <li v-for="p in row.pods" :key="p.id">
                  <strong>{{ p.name }}</strong>
                  <span class="mono">{{ p.id }}</span>
                  <span :class="statusClass(p.status)"><span class="status-lamp"></span>{{ p.status }}</span>
                  <button class="tiny-btn" @click="togglePodExpand(p.id)">
                    {{ isPodExpanded(p.id) ? 'Hide Containers' : 'Show Containers' }} ({{ podContainers(p.id).length }})
                  </button>
                  <div v-if="isPodExpanded(p.id)" class="pod-container-cards">
                    <div v-for="c in podContainers(p.id)" :key="`rel-${p.id}-${c.id}`" class="pod-container-card">
                      <strong>{{ c.name }}</strong>
                      <span class="mono">{{ c.id }}</span>
                      <span class="selector">{{ c.image }}</span>
                      <span :class="statusClass(c.status)"><span class="status-lamp"></span>{{ c.status }}</span>
                      <button class="tiny-btn" @click="openContainerLog(c.id, c.name)">Log</button>
                      <button class="tiny-btn" @click="openContainerSpec(c.id, c.name)">Config Spec</button>
                      <button class="tiny-btn" @click="openContainerDetail(c.id)">Detail</button>
                      <button
                        v-if="canAttachContainer(c)"
                        class="tiny-btn"
                        @click="openAttachOverlay(c)"
                      >
                        Attach
                      </button>
                      <button
                        v-if="canExecContainer(c.status)"
                        class="tiny-btn"
                        @click="openExecOverlay(c)"
                      >
                        Exec
                      </button>
                    </div>
                    <div v-if="podContainers(p.id).length === 0" class="empty">No container</div>
                  </div>
                  <button class="tiny-btn" @click="openResourceDetail('pod', p.id)">Detail</button>
                </li>
                <li v-if="row.pods.length === 0" class="empty">No matched pod</li>
              </ul>
            </div>

            <div class="relation-arrow" aria-hidden="true"></div>

            <div class="relation-block">
              <p class="relation-title">Service ({{ row.services.length }})</p>
              <ul class="relation-items">
                <li v-for="s in row.services" :key="s.id">
                  <strong>{{ s.name }}</strong>
                  <span class="mono">{{ s.id }}</span>
                  <span class="selector">ports: {{ servicePortsText(s.ports) }}</span>
                  <span class="selector">selector: {{ selectorText(s.selector) }}</span>
                  <button class="tiny-btn" @click="openResourceDetail('service', s.id)">Detail</button>
                </li>
                <li v-if="row.services.length === 0" class="empty">No matched service</li>
              </ul>
            </div>
          </div>

          <div v-if="resourceRelations.length === 0" class="empty relation-empty">No ReplicaSet relation found</div>
        </div>
      </div>
    </article>

    <article class="panel resource-panel">
      <button class="section-head section-toggle" @click="toggleResourcePanel('replicaset')">
        <h3>ReplicaSet</h3>
        <span class="count">{{ replicasets.length }}</span>
        <span class="count">{{ resourcePanels.replicaset ? 'Hide' : 'Show' }}</span>
      </button>
      <ul v-if="resourcePanels.replicaset" class="list">
        <li v-for="item in replicasets" :key="item.id">
          <strong>{{ item.name }}</strong>
          <span class="mono">{{ item.id }}</span>
          <span :class="replicasetHealthClass(item.desired, item.ready)">
            <span class="status-lamp"></span>
            {{ replicasetHealthLabel(item.desired, item.ready) }}
          </span>
          <span class="selector">desired/current/ready: {{ item.desired }} / {{ item.current }} / {{ item.ready }}</span>
        </li>
        <li v-if="replicasets.length === 0" class="empty">No ReplicaSet</li>
      </ul>
      <p v-else class="empty">Click to show list</p>
    </article>

    <article class="panel resource-panel">
      <button class="section-head section-toggle" @click="toggleResourcePanel('pod')">
        <h3>Pod</h3>
        <span class="count">{{ pods.length }}</span>
        <span class="count">{{ resourcePanels.pod ? 'Hide' : 'Show' }}</span>
      </button>
      <ul v-if="resourcePanels.pod" class="list">
        <li v-for="item in pods" :key="item.id">
          <strong>{{ item.name }}</strong>
          <span class="mono">{{ item.id }}</span>
          <span :class="statusClass(item.status)"><span class="status-lamp"></span>{{ item.status }}</span>
          <button class="tiny-btn" @click="togglePodExpand(item.id)">
            {{ isPodExpanded(item.id) ? 'Hide Containers' : 'Show Containers' }} ({{ podContainers(item.id).length }})
          </button>
          <div v-if="isPodExpanded(item.id)" class="pod-container-cards">
            <div v-for="c in podContainers(item.id)" :key="`list-${item.id}-${c.id}`" class="pod-container-card">
              <strong>{{ c.name }}</strong>
              <span class="mono">{{ c.id }}</span>
              <span class="selector">{{ c.image }}</span>
              <span :class="statusClass(c.status)"><span class="status-lamp"></span>{{ c.status }}</span>
              <button class="tiny-btn" @click="openContainerLog(c.id, c.name)">Log</button>
              <button class="tiny-btn" @click="openContainerSpec(c.id, c.name)">Config Spec</button>
              <button class="tiny-btn" @click="openContainerDetail(c.id)">Detail</button>
              <button
                v-if="canAttachContainer(c)"
                class="tiny-btn"
                @click="openAttachOverlay(c)"
              >
                Attach
              </button>
              <button
                v-if="canExecContainer(c.status)"
                class="tiny-btn"
                @click="openExecOverlay(c)"
              >
                Exec
              </button>
            </div>
            <div v-if="podContainers(item.id).length === 0" class="empty">No container</div>
          </div>
        </li>
        <li v-if="pods.length === 0" class="empty">No Pod</li>
      </ul>
      <p v-else class="empty">Click to show list</p>
    </article>

    <article class="panel resource-panel">
      <button class="section-head section-toggle" @click="toggleResourcePanel('service')">
        <h3>Service</h3>
        <span class="count">{{ services.length }}</span>
        <span class="count">{{ resourcePanels.service ? 'Hide' : 'Show' }}</span>
      </button>
      <ul v-if="resourcePanels.service" class="list">
        <li v-for="item in services" :key="item.id">
          <strong>{{ item.name }}</strong>
          <span class="mono">{{ item.id }}</span>
          <span class="selector">ports: {{ servicePortsText(item.ports) }}</span>
        </li>
        <li v-if="services.length === 0" class="empty">No Service</li>
      </ul>
      <p v-else class="empty">Click to show list</p>
    </article>
  </section>
</template>

<script setup>
defineProps({
  resourceRelations: { type: Array, required: true },
  replicasets: { type: Array, required: true },
  pods: { type: Array, required: true },
  services: { type: Array, required: true },
  resourcePanels: { type: Object, required: true },
  statusClass: { type: Function, required: true },
  replicasetHealthClass: { type: Function, required: true },
  replicasetHealthLabel: { type: Function, required: true },
  selectorText: { type: Function, required: true },
  servicePortsText: { type: Function, required: true },
  podContainers: { type: Function, required: true },
  isPodExpanded: { type: Function, required: true },
  togglePodExpand: { type: Function, required: true },
  openResourceDetail: { type: Function, required: true },
  openContainerLog: { type: Function, required: true },
  openContainerSpec: { type: Function, required: true },
  openContainerDetail: { type: Function, required: true },
  openAttachOverlay: { type: Function, required: true },
  openExecOverlay: { type: Function, required: true },
  canAttachContainer: { type: Function, required: true },
  canExecContainer: { type: Function, required: true },
  toggleResourcePanel: { type: Function, required: true },
  openResourceCreateOverlay: { type: Function, required: true }
})
</script>
