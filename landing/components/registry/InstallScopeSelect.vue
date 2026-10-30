<script setup lang="ts">
import type { InstallScope } from '~/types/registry';
defineProps<{ modelValue: InstallScope; projectAvailable: boolean }>();
const emit = defineEmits<{ 'update:modelValue': [value: InstallScope] }>();
const { t } = useI18n();
</script>

<template>
  <fieldset class="install-scope">
    <legend>{{ t('registryUi.install.scope.label') }}</legend>
    <div class="install-scope__choices">
      <button
        v-for="value in ['user', 'project'] as const"
        :key="value"
        type="button"
        :aria-pressed="modelValue === value"
        :disabled="value === 'project' && !projectAvailable && modelValue !== 'project'"
        @click="emit('update:modelValue', value)"
      >
        {{ t(`registryUi.install.scope.${value}`) }}
      </button>
    </div>
    <p v-if="modelValue === 'project'">{{ t('registryUi.install.scope.projectHint') }}</p>
    <p v-else-if="!projectAvailable">{{ t('registryUi.install.scope.projectUnavailable') }}</p>
  </fieldset>
</template>

<style scoped>
.install-scope {
  min-width: 0;
  margin: 0;
  padding: 0;
  border: 0;
}
.install-scope legend {
  margin-bottom: 6px;
  color: var(--subtle);
  font-size: 0.75rem;
}
.install-scope__choices {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}
.install-scope button {
  padding: 7px 11px;
  border: 1px solid var(--line);
  border-radius: 8px;
  background: var(--surface);
  color: var(--ink);
  font: inherit;
  font-size: 0.8rem;
  cursor: pointer;
}
.install-scope button[aria-pressed='true'] {
  border-color: var(--violet);
  background: color-mix(in srgb, var(--violet) 12%, var(--surface));
}
.install-scope button:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}
.install-scope p {
  margin: 7px 0 0;
  color: var(--subtle);
  font-size: 0.75rem;
  line-height: 1.5;
}
</style>
