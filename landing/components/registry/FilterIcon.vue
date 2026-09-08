<script setup lang="ts">
type FilterIconName =
  'category' | 'component' | 'source' | 'trust' | 'agent' | 'authentication' | 'owner';

defineProps<{ name: FilterIconName }>();
</script>

<template>
  <svg class="filter-icon" viewBox="0 0 24 24" fill="none" aria-hidden="true">
    <template v-if="name === 'category'">
      <path
        d="M4 7.5V5a1 1 0 0 1 1-1h2.5l11.25 11.25a1.5 1.5 0 0 1 0 2.12l-1.38 1.38a1.5 1.5 0 0 1-2.12 0L4 7.5Z"
      />
      <circle cx="7.4" cy="7.4" r="1" />
    </template>
    <template v-else-if="name === 'component'">
      <rect x="4" y="4" width="6" height="6" rx="1.5" />
      <rect x="14" y="4" width="6" height="6" rx="1.5" />
      <rect x="4" y="14" width="6" height="6" rx="1.5" />
      <rect x="14" y="14" width="6" height="6" rx="1.5" />
    </template>
    <template v-else-if="name === 'source'">
      <circle cx="6" cy="5" r="2" />
      <circle cx="18" cy="7" r="2" />
      <circle cx="6" cy="19" r="2" />
      <path d="M6 7v10M8 9c2.5 0 3 4 6 4h2M16.5 8.5 14 11l2.5 2.5" />
    </template>
    <template v-else-if="name === 'trust'">
      <path d="M12 3.5 19 6v5.5c0 4.5-2.75 7.25-7 9-4.25-1.75-7-4.5-7-9V6l7-2.5Z" />
      <path d="m9 12 2 2 4-4" />
    </template>
    <template v-else-if="name === 'agent'">
      <rect x="5" y="7" width="14" height="11" rx="3" />
      <path d="M12 4v3M9 12h.01M15 12h.01M9 15h6" />
    </template>
    <template v-else-if="name === 'authentication'">
      <circle cx="8" cy="12" r="4" />
      <path d="M12 12h8M17 12v3M20 12v2" />
    </template>
    <template v-else>
      <circle cx="12" cy="8" r="3.5" />
      <path d="M5 20c.6-4 3-6 7-6s6.4 2 7 6" />
    </template>
  </svg>
</template>
