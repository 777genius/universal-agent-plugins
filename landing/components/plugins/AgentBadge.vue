<script setup lang="ts">
import { computed } from 'vue';
import { resolveAgentBadge } from '~/data/agentBadges';

const props = defineProps<{
  label: string;
  tone?: 'card' | 'detail';
  highlightQuery?: string;
}>();

const iconSrc = computed(() => resolveAgentBadge(props.label));
</script>

<template>
  <span class="agent-badge" :class="[`agent-badge--${tone ?? 'card'}`]">
    <img
      v-if="iconSrc"
      :src="iconSrc"
      :alt="`${label} icon`"
      class="agent-badge__icon"
      loading="lazy"
      decoding="async"
    >
    <span class="agent-badge__text">
      <SearchHighlight :text="label" :query="highlightQuery" />
    </span>
  </span>
</template>

<style scoped>
.agent-badge {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  border-radius: 999px;
  border: 1px solid rgba(255, 255, 255, 0.08);
  color: #e0e6ff;
}

.agent-badge--card {
  padding: 8px 10px;
  background: rgba(255, 255, 255, 0.05);
  font-size: 0.76rem;
}

.agent-badge--detail {
  padding: 9px 12px;
  background: rgba(255, 255, 255, 0.05);
  font-size: 0.8rem;
  font-weight: 700;
}

.agent-badge__icon {
  width: 16px;
  height: 16px;
  object-fit: contain;
  flex-shrink: 0;
}

.agent-badge__text {
  line-height: 1;
}

.v-theme--light .agent-badge {
  color: #0f172a;
}
</style>
