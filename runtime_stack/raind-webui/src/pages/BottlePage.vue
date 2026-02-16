<template>
  <section class="panel">
    <div class="section-head">
      <div class="container-head-left">
        <h3>Bottle List</h3>
        <button class="primary-outline" @click="openBottleCreateOverlay">Create</button>
      </div>
      <span class="count">{{ bottles.length }} items</span>
    </div>

    <div class="table-scroller">
      <table>
        <thead>
          <tr>
            <th>ID</th>
            <th>Name</th>
            <th>Services</th>
            <th>Status</th>
            <th>Containers</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="b in bottles" :key="b.id">
            <td class="mono">{{ b.id }}</td>
            <td>{{ b.name }}</td>
            <td>{{ b.serviceCount }}</td>
            <td>
              <span :class="statusClass(b.status)">
                <span class="status-lamp"></span>
                {{ b.status }}
              </span>
            </td>
            <td>
              <div class="bottle-containers">
                <div v-for="c in b.containers" :key="`${b.id}-${c.id}-${c.serviceName}`" class="bottle-container-item">
                  <strong>{{ c.serviceName }}</strong>
                  <span class="mono">{{ c.id }}</span>
                  <span class="selector">{{ c.name }}</span>
                  <span class="selector">{{ c.image }}</span>
                  <span :class="statusClass(c.state)">
                    <span class="status-lamp"></span>
                    {{ c.state }}
                  </span>
                  <div class="actions">
                    <button class="tiny-btn" @click="openContainerDetail(c.id)">Detail</button>
                    <button class="tiny-btn" @click="openContainerLog(c.id, c.name)">Log</button>
                  </div>
                </div>
                <div v-if="b.containers.length === 0" class="empty">No containers</div>
              </div>
            </td>
            <td class="actions">
              <button v-if="canStartContainer(b.status)" class="success" @click="bottleAction(b.id, 'start')">Start</button>
              <button
                v-if="canStopContainer(b.status)"
                class="caution"
                @click="openBottleActionConfirm('stop', b.id, b.name)"
              >
                Stop
              </button>
              <button
                v-if="canDeleteContainer(b.status)"
                class="danger"
                @click="openBottleActionConfirm('delete', b.id, b.name)"
              >
                Delete
              </button>
            </td>
          </tr>
          <tr v-if="bottles.length === 0">
            <td colspan="6" class="empty">No bottles</td>
          </tr>
        </tbody>
      </table>
    </div>
  </section>
</template>

<script setup>
defineProps({
  bottles: { type: Array, required: true },
  statusClass: { type: Function, required: true },
  canStartContainer: { type: Function, required: true },
  canStopContainer: { type: Function, required: true },
  canDeleteContainer: { type: Function, required: true },
  bottleAction: { type: Function, required: true },
  openBottleActionConfirm: { type: Function, required: true },
  openContainerDetail: { type: Function, required: true },
  openContainerLog: { type: Function, required: true },
  openBottleCreateOverlay: { type: Function, required: true }
})
</script>
