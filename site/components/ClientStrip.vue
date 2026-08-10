<script setup lang="ts">
const { asset } = useSite()
</script>

<template>
  <ul class="client-strip" aria-label="Supported agents">
    <li v-for="client in clients" :key="client.id">
      <span class="client-strip__icon"><img :src="asset(`client-icons/${client.icon}`)" alt="" width="24" height="24" /></span>
      <span>{{ client.name }}</span>
    </li>
  </ul>
</template>
