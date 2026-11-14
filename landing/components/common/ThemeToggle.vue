<script setup lang="ts">
import { mdiWeatherSunny, mdiWeatherNight } from '@mdi/js';

const { t } = useI18n();
const { isDark, toggleTheme } = useBrowserTheme();
const { trackThemeToggle } = useAnalytics();

const tooltip = computed(() => isDark.value ? t('theme.light') : t('theme.dark'));
const ariaLabel = tooltip;

const onToggle = () => {
  const nextTheme = isDark.value ? "light" : "dark";
  toggleTheme();
  trackThemeToggle(nextTheme);
};
</script>

<template>
  <v-tooltip :text="tooltip" location="bottom">
    <template #activator="{ props }">
      <v-btn
        v-bind="props"
        :icon="isDark ? mdiWeatherSunny : mdiWeatherNight"
        variant="text"
        size="small"
        :aria-label="ariaLabel"
        @click="onToggle"
      />
    </template>
  </v-tooltip>
</template>
