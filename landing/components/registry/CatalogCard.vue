<script setup lang="ts">
defineProps<{ expanded?: boolean }>();
</script>

<template>
  <article class="plugin-card" :class="{ 'plugin-card--expanded': expanded }">
    <slot />
  </article>
</template>
