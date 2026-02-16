<template>
  <section class="panel">
    <div class="section-head">
      <div class="container-head-left">
        <h3>Image List</h3>
        <button class="primary-outline" @click="openPullImageOverlay">Pull Image</button>
      </div>
      <span class="count">{{ images.length }} items</span>
    </div>

    <div class="table-scroller">
      <table>
        <thead>
          <tr>
            <th>Image</th>
            <th>Repository</th>
            <th>Reference</th>
            <th>Created</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="img in sortedImages" :key="`${img.repository}:${img.reference}`">
            <td>{{ imageNameShort(img) }}</td>
            <td>{{ img.repository || '-' }}</td>
            <td>{{ img.reference || '-' }}</td>
            <td>{{ formatTime(img.createdAt) }}</td>
            <td class="actions">
              <button class="danger" @click="openImageDeleteOverlay(img)">Delete</button>
            </td>
          </tr>
          <tr v-if="images.length === 0">
            <td colspan="5" class="empty">No images</td>
          </tr>
        </tbody>
      </table>
    </div>
  </section>
</template>

<script setup>
defineProps({
  images: { type: Array, required: true },
  sortedImages: { type: Array, required: true },
  imageNameShort: { type: Function, required: true },
  formatTime: { type: Function, required: true },
  openPullImageOverlay: { type: Function, required: true },
  openImageDeleteOverlay: { type: Function, required: true }
})
</script>
