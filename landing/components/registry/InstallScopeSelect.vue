<script setup lang="ts">
import type { InstallScope } from '~/types/registry';
defineProps<{ modelValue: InstallScope; projectAvailable: boolean; alwaysShow?: boolean }>();
const emit = defineEmits<{ 'update:modelValue': [value: InstallScope] }>();
const { t } = useI18n();
</script>

<template>
  <fieldset v-if="projectAvailable || modelValue === 'project' || alwaysShow" class="install-scope">
    <legend>{{ t('registryUi.install.scope.label') }}</legend>
    <div class="install-scope__choices" :data-selected="modelValue">
      <span class="install-scope__thumb" aria-hidden="true" />
      <button
        v-for="value in ['user', 'project'] as const"
        :key="value"
        type="button"
        :aria-pressed="modelValue === value"
        :class="{ 'install-scope__button--active': modelValue === value }"
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
  margin-bottom: 7px;
  color: var(--subtle);
  font-size: .78rem;
}
.install-scope__choices {
  position: relative;
  isolation: isolate;
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 2px;
  min-width: 0;
  width: min(100%, 360px);
  padding: 3px;
  border: 1px solid var(--line-strong);
  border-radius: 12px;
  background: color-mix(in srgb, var(--surface-solid) 80%, transparent);
}
.install-scope__thumb {
  position: absolute;
  z-index: -1;
  inset: 3px auto 3px 3px;
  width: calc((100% - 8px) / 2);
  border: 1px solid color-mix(in srgb, var(--primary-bright) 58%, var(--line));
  border-radius: 9px;
  background: color-mix(in srgb, var(--primary-bright) 16%, var(--surface));
  box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--primary-bright) 9%, transparent);
  transition: transform .25s cubic-bezier(.2,.8,.2,1);
  pointer-events: none;
}
.install-scope__choices[data-selected='project'] .install-scope__thumb {
  transform: translateX(calc(100% + 2px));
}
/* Logical visual position must match the grid's reversed order in Arabic. */
.install-scope:dir(rtl) .install-scope__thumb {
  left: auto;
  right: 3px;
}
.install-scope:dir(rtl) .install-scope__choices[data-selected='project'] .install-scope__thumb {
  transform: translateX(calc(-100% - 2px));
}
.install-scope button {
  display: grid;
  place-items: center;
  min-width: 0;
  min-height: 42px;
  padding: 7px 8px;
  border: 0;
  border-radius: 9px;
  color: var(--muted);
  background: transparent;
  font: inherit;
  font-size: .85rem;
  font-weight: 570;
  line-height: 1.3;
  cursor: pointer;
  transition: color .2s ease;
  overflow-wrap: anywhere;
}
.install-scope button[aria-pressed='true'] {
  color: var(--text);
  font-weight: 680;
}
.install-scope button:focus-visible {
  outline: 2px solid var(--cyan);
  outline-offset: -2px;
}
.install-scope button:disabled {
  opacity: .55;
  cursor: not-allowed;
}
.install-scope p {
  margin: 8px 0 0;
  color: var(--subtle);
  font-size: .75rem;
  line-height: 1.5;
}
@media (prefers-reduced-motion: reduce) {
  .install-scope__thumb,
  .install-scope button { transition: none; }
}
</style>
