<script setup lang="ts">
import type { RegistryPlugin } from '~/types/registry';
import { securityAssessmentLabel, securityAssessmentTooltip } from '~/utils/securityPresentation';
const { t, n } = useI18n();
const translate = (key: string, params: Record<string, string | number> = {}, plural?: number) =>
  plural === undefined ? t(key, params) : t(key, { ...params, count: n(plural) }, plural);

const props = defineProps<{
  plugin: RegistryPlugin;
  detailsTo: string | { path: string; query?: Record<string, string>; hash?: string };
}>();
const assessment = computed(() => props.plugin.security!);
const label = computed(() => securityAssessmentLabel(assessment.value, translate));
const tooltip = computed(() => securityAssessmentTooltip(props.plugin, translate));
</script>

<template>
  <AppTooltip>
    <template #trigger>
      <NuxtLink
        class="plugin-card__security"
        :class="`plugin-card__security--${assessment.outcome}`"
        :to="detailsTo"
        :aria-label="t('registryUi.security.openReview', { label, name: plugin.display_name })"
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
      <p class="app-tooltip__eyebrow">{{ t('registryUi.security.staticReview') }}</p>
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
        {{ t('registryUi.security.more', { count: n(tooltip.remaining) }) }}
      </p>
      <p class="app-tooltip__disclaimer">{{ tooltip.disclaimer }}</p>
      <small>{{ t('registryUi.security.fullDetails') }}</small>
    </div>
  </AppTooltip>
</template>
