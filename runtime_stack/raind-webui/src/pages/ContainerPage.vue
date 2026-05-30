<template>
  <section class="panel">
    <div class="section-head">
      <div class="container-head-left">
        <h3>Container List</h3>
        <button class="primary-outline" @click="openCreateContainerOverlay">Create Container</button>
      </div>
      <span class="count">{{ containers.length }} items</span>
    </div>

    <div class="table-scroller">
      <table>
        <thead>
          <tr>
            <th>ID</th>
            <th>Name</th>
            <th>Image</th>
            <th>Ports</th>
            <th>Status</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="c in containers" :key="c.id">
            <td class="mono">{{ c.id }}</td>
            <td>{{ c.name }}</td>
            <td>{{ c.image }}</td>
            <td>{{ c.ports || '-' }}</td>
            <td>
              <span :class="statusClass(c.status)">
                <span class="status-lamp"></span>
                {{ c.status }}
              </span>
            </td>
            <td class="actions">
              <button @click="openContainerDetail(c.id)">Detail</button>
              <button @click="openContainerLog(c.id, c.name)">Log</button>
              <button @click="openContainerSpec(c.id, c.name)">Config Spec</button>
              <button v-if="canAttachContainer(c)" class="primary-outline" @click="openAttachOverlay(c)">Attach</button>
              <button v-if="canExecContainer(c.status)" class="primary-outline" @click="openExecOverlay(c)">Exec</button>
              <button v-if="canStartContainer(c.status)" class="success" @click="containerAction(c.id, 'start')">Start</button>
              <button v-if="canStopContainer(c.status)" class="caution" @click="openActionConfirm('stop', c.id, c.name)">Stop</button>
              <button v-if="canDeleteContainer(c.status)" class="danger" @click="openActionConfirm('delete', c.id, c.name)">Delete</button>
            </td>
          </tr>
          <tr v-if="containers.length === 0">
            <td colspan="6" class="empty">No containers</td>
          </tr>
        </tbody>
      </table>
    </div>
  </section>
</template>

<script setup>
defineProps({
  containers: { type: Array, required: true },
  statusClass: { type: Function, required: true },
  canAttachContainer: { type: Function, required: true },
  canExecContainer: { type: Function, required: true },
  canStartContainer: { type: Function, required: true },
  canStopContainer: { type: Function, required: true },
  canDeleteContainer: { type: Function, required: true },
  openCreateContainerOverlay: { type: Function, required: true },
  openContainerDetail: { type: Function, required: true },
  openContainerLog: { type: Function, required: true },
  openContainerSpec: { type: Function, required: true },
  openAttachOverlay: { type: Function, required: true },
  openExecOverlay: { type: Function, required: true },
  containerAction: { type: Function, required: true },
  openActionConfirm: { type: Function, required: true }
})
</script>
