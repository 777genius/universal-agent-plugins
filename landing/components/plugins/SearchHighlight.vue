<script setup lang="ts">
import { computed } from 'vue';

const props = defineProps<{
  text: string;
  query?: string;
}>();

function escapeRegExp(value: string) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

const tokens = computed(() => {
  const query = props.query?.trim() ?? '';
  if (!query) return [];

  return [...new Set(query.split(/\s+/).map((token) => token.trim()).filter(Boolean))]
    .sort((left, right) => right.length - left.length);
});

const parts = computed(() => {
  if (!tokens.value.length) {
    return [{ text: props.text, matched: false }];
  }

  const matcher = new RegExp(`(${tokens.value.map(escapeRegExp).join('|')})`, 'gi');

  return props.text
    .split(matcher)
    .filter(Boolean)
    .map((part) => ({
      text: part,
      matched: tokens.value.some((token) => token.toLowerCase() === part.toLowerCase()),
    }));
});
</script>

<template>
  <span class="search-highlight">
    <template v-for="(part, index) in parts" :key="`${part.text}-${index}`">
      <mark v-if="part.matched" class="search-highlight__mark">{{ part.text }}</mark>
      <template v-else>{{ part.text }}</template>
    </template>
  </span>
</template>

<style scoped>
.search-highlight__mark {
  background: rgba(0, 240, 255, 0.16);
  color: inherit;
  border-radius: 0.42em;
  padding: 0.04em 0.2em;
  box-shadow: inset 0 0 0 1px rgba(0, 240, 255, 0.2);
}

.v-theme--light .search-highlight__mark {
  background: rgba(8, 145, 178, 0.14);
  box-shadow: inset 0 0 0 1px rgba(8, 145, 178, 0.16);
}
</style>
