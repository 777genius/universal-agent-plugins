<script setup lang="ts">
import { mdiArrowLeft } from '@mdi/js';

// All destinations are already localized by the page that owns the route.
defineProps<{
  backTo: string;
  backLabel: string;
  label: string;
  parents: readonly { label: string; to: string }[];
  current: string;
}>();
</script>

<template>
  <div class="detail-navigation">
    <NuxtLink :to="backTo" :aria-label="backLabel" class="detail-navigation__back">
      <v-icon :icon="mdiArrowLeft" size="22" aria-hidden="true" />
    </NuxtLink>
    <nav class="breadcrumbs" :aria-label="label">
      <template v-for="parent in parents" :key="parent.to">
        <NuxtLink :to="parent.to">{{ parent.label }}</NuxtLink>
        <span aria-hidden="true">/</span>
      </template>
      <span aria-current="page">{{ current }}</span>
    </nav>
  </div>
</template>

<style scoped>
.detail-navigation {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 16px;
  min-width: 0;
}
.detail-navigation__back {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex: 0 0 40px;
  width: 40px;
  height: 40px;
  border-radius: 50%;
  color: var(--muted);
}
.detail-navigation__back:hover {
  background: var(--surface-raised);
  color: var(--text);
}
.detail-navigation__back:focus-visible {
  outline: 2px solid var(--primary-bright);
  outline-offset: 2px;
}
.detail-navigation:dir(rtl) .detail-navigation__back {
  transform: scaleX(-1);
}
.breadcrumbs {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  min-width: 0;
  margin: 0;
  padding: 0;
  color: var(--muted);
  font-size: 0.88rem;
  line-height: 1.4;
  overflow-wrap: anywhere;
}
.breadcrumbs a {
  color: var(--primary-bright);
  text-underline-offset: 3px;
}
.breadcrumbs [aria-current='page'] {
  color: var(--text);
}
</style>
