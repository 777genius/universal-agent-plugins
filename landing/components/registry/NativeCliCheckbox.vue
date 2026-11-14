<script setup lang="ts">
import { useInstallPreferencesStore } from '~/stores/installPreferences';

const preferences = useInstallPreferencesStore();
const { t } = useI18n({ useScope: 'global' });
</script>

<template>
  <label class="native-cli-checkbox">
    <input v-model="preferences.useNativeCli" type="checkbox">
    <span>{{ t('registryUi.command.useNativeCli') }}</span>
  </label>
</template>

<style scoped>
.native-cli-checkbox {
  display: inline-flex;
  align-items: flex-start;
  gap: 10px;
  margin-top: 12px;
  color: var(--muted, #64748b);
  font-size: 0.875rem;
  line-height: 1.45;
  cursor: pointer;
}

.native-cli-checkbox input {
  flex: 0 0 auto;
  width: 18px;
  height: 18px;
  margin: 1px 0 0;
  accent-color: var(--violet, #7c3aed);
  cursor: pointer;
}

.native-cli-checkbox input:focus-visible {
  outline: 2px solid var(--violet, #7c3aed);
  outline-offset: 3px;
}
</style>
