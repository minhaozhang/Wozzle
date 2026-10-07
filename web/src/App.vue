<script setup lang="ts">
import { onMounted } from 'vue'
import Sidebar from './components/Sidebar.vue'
import { startGlobalStreams } from './stores/containers'

onMounted(() => {
  startGlobalStreams()
})
</script>

<template>
  <div class="app-shell">
    <Sidebar />
    <main class="main">
      <RouterView :key="$route.fullPath" />
    </main>
  </div>
</template>
