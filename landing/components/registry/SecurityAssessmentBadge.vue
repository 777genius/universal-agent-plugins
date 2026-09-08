<script setup lang="ts">
import type { RegistryPlugin } from '~/types/registry';
import { securityAssessmentLabel, securityAssessmentTooltip } from '~/utils/securityPresentation';

const props = defineProps<{
  plugin: RegistryPlugin;
  detailsTo: string | { path: string; query?: Record<string, string>; hash?: string };
}>();
const assessment = computed(() => props.plugin.security!);
const label = computed(() => securityAssessmentLabel(assessment.value));
const tooltip = computed(() => securityAssessmentTooltip(props.plugin));
</script>

<template>
  <AppTooltip>
    <template #trigger>
      <NuxtLink
        class="plugin-card__security"
        :class="`plugin-card__security--${assessment.outcome}`"
        :to="detailsTo"
        :aria-label="`${label}. Open the full review for ${plugin.display_name}`"
      >
        <span aria-hidden="true">{{
          assessment.outcome === 'blocking_findings'
            ? '!'
            : assessment.outcome === 'warnings'
              ? 'i'
              : '✓'
        }}</span>
        {{ label }}
      </NuxtLink>
    </template>
    <div class="app-tooltip__review">
      <p class="app-tooltip__eyebrow">Automated static review</p>
      <strong>{{ tooltip.label }}</strong>
      <p>{{ tooltip.scope }}</p>
      <ul v-if="tooltip.findings.length">
        <li
          v-for="finding in tooltip.findings"
          :key="`${finding.code}:${finding.path}:${finding.line}`"
        >
          <code>{{ finding.code }}</code>
          <span>{{ finding.message }}</span>
        </li>
      </ul>
      <p v-if="tooltip.remaining" class="app-tooltip__more">
        +{{ tooltip.remaining }} more in the full review
      </p>
      <p class="app-tooltip__disclaimer">{{ tooltip.disclaimer }}</p>
      <small>Open the plugin page for full details.</small>
    </div>
  </AppTooltip>
</template>
