<script setup lang="ts">
import { supportedLocales } from "~/data/i18n";
import type { KnownLocale } from "~/data/i18n";

const { t, locale } = useI18n();
const props = defineProps<{ fullWidth?: boolean; compact?: boolean; iconOnly?: boolean }>();
const { switchLocale, pending, error } = useLocation();
const retryLocale = ref<string>();
const activator = ref<{ $el?: HTMLElement; focus?: () => void }>();
const returnFocus = () => nextTick(() => {
  activator.value?.focus?.();
  activator.value?.$el?.focus();
});

const flagIconMap: Record<string, string> = {
  en: "circle-flags:gb",
  ru: "circle-flags:ru",
  uk: "circle-flags:ua",
  es: "circle-flags:es",
  fr: "circle-flags:fr",
  zh: "circle-flags:cn"
};

const items = computed(() =>
  supportedLocales.map((item) => ({
    title: item.name,
    value: item.code as KnownLocale,
    flagIcon: flagIconMap[item.code] ?? "circle-flags:xx"
  }))
);

const dropdownItems = computed(() =>
  items.value
);

const currentFlagIcon = computed(() => {
  return flagIconMap[locale.value as string] ?? "circle-flags:xx";
});

const iconMenuOpen = ref(false);
const searchQuery = ref("");
const searchInputRef = ref<HTMLInputElement | null>(null);

const filteredDropdownItems = computed(() => {
  const q = searchQuery.value.toLowerCase().trim();
  if (!q) return dropdownItems.value;
  return dropdownItems.value.filter(
    (item) =>
      item.title.toLowerCase().includes(q) ||
      item.value.toLowerCase().includes(q)
  );
});

watch(iconMenuOpen, (open) => {
  if (open) {
    searchQuery.value = "";
    nextTick(() => searchInputRef.value?.focus());
  }
});

const onChange = async (value: unknown) => {
  if (typeof value !== 'string' || pending.value) return;
  retryLocale.value = value;
  await switchLocale(value);
  iconMenuOpen.value = false;
  returnFocus();
};
</script>

<template>
  <!-- Icon-only mode with search dropdown -->
  <v-menu v-if="props.iconOnly" v-model="iconMenuOpen" location="bottom end" :close-on-content-click="false">
    <template #activator="{ props: menuProps }">
      <v-btn ref="activator" variant="text" v-bind="menuProps" :disabled="pending" :aria-label="t('language.label')">
        <Icon :name="currentFlagIcon" class="language-switcher__flag-icon" />
      </v-btn>
    </template>
    <div class="language-switcher__dropdown-panel">
      <div class="language-switcher__search-wrap">
        <input
          ref="searchInputRef"
          v-model="searchQuery"
          type="text"
          class="language-switcher__search-input"
          :placeholder="t('language.search')"
          :aria-label="t('language.search')"
          @keydown.esc="iconMenuOpen = false"
        >
      </div>
      <v-list density="compact" class="language-switcher__menu-list">
        <v-list-item
          v-for="item in filteredDropdownItems"
          :key="item.value"
          :disabled="pending || item.value === locale"
          :active="item.value === locale"
          @click="onChange(item.value)"
        >
          <template #title>
            <span class="language-switcher__item">
              <Icon :name="item.flagIcon" class="language-switcher__flag-icon" />
              <span>{{ item.title }}</span>
            </span>
          </template>
        </v-list-item>
        <v-list-item v-if="filteredDropdownItems.length === 0" disabled>
          <template #title>
            <span class="language-switcher__no-results">—</span>
          </template>
        </v-list-item>
      </v-list>
    </div>
  </v-menu>

  <!-- Standard mode with search -->
  <v-autocomplete
    v-else
    ref="activator"
    :disabled="pending"
    :loading="pending"
    :label="props.compact ? undefined : t('language.label')"
    :placeholder="props.compact ? t('language.label') : undefined"
    :items="items"
    :model-value="locale"
    density="compact"
    :variant="props.compact ? 'plain' : 'outlined'"
    hide-details
    auto-select-first
    :menu-props="{ contentClass: 'language-switcher__dropdown' }"
    :style="props.fullWidth ? { maxWidth: '100%', width: '100%' } : { maxWidth: '220px' }"
    :class="{
      'language-switcher--full': props.fullWidth,
      'language-switcher--compact': props.compact
    }"
    :aria-label="t('language.label')"
    :single-line="props.compact"
    @update:model-value="onChange"
  >
    <template #selection>
      <Icon :name="currentFlagIcon" class="language-switcher__flag-icon" />
    </template>
    <template #item="{ item, props: itemProps }">
      <v-list-item v-bind="itemProps">
        <template #title>
          <span class="language-switcher__item">
            <Icon :name="item.raw.flagIcon" class="language-switcher__flag-icon" />
            <span>{{ item.raw.title }}</span>
          </span>
        </template>
      </v-list-item>
    </template>
  </v-autocomplete>
  <div v-if="error" role="alert">
    {{ t('language.switchError', 'Unable to change language. Please try again.') }}
    <v-btn :disabled="pending" variant="text" @click="onChange(retryLocale)">
      {{ t('language.retry', 'Retry') }}
    </v-btn>
  </div>
</template>

<style scoped>
.language-switcher__flag-icon {
  width: 22px;
  height: 22px;
  flex-shrink: 0;
  border-radius: 50%;
}

.language-switcher__item {
  display: flex;
  align-items: center;
  gap: 8px;
}

.language-switcher--compact :deep(.v-field) {
  min-height: 36px;
}

.language-switcher--compact :deep(.v-field__input) {
  padding-top: 6px;
  padding-bottom: 6px;
  min-height: 36px;
}

.language-switcher--compact {
  min-width: 60px;
  position: relative;
  z-index: 2;
}

.language-switcher--compact :deep(.v-field__outline) {
  display: none;
}

.language-switcher--compact :deep(.v-field__overlay) {
  background-color: transparent;
}

.language-switcher__menu-list {
  min-width: 180px;
  max-height: 280px;
  overflow-y: auto;
}

.language-switcher__dropdown-panel {
  background: rgb(var(--v-theme-surface));
  border-radius: 8px;
  overflow: hidden;
  min-width: 200px;
}

.language-switcher__search-wrap {
  padding: 8px 10px 4px;
}

.language-switcher__search-input {
  width: 100%;
  padding: 6px 10px;
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 6px;
  background: rgba(255, 255, 255, 0.05);
  color: inherit;
  font-size: 0.85rem;
  outline: none;
  transition: border-color 0.2s ease;
}

.language-switcher__search-input:focus {
  border-color: rgba(0, 240, 255, 0.4);
}

.language-switcher__search-input::placeholder {
  color: rgba(255, 255, 255, 0.35);
  font-size: 0.82rem;
}

.language-switcher__no-results {
  color: rgba(255, 255, 255, 0.35);
  font-size: 0.82rem;
  text-align: center;
  display: block;
}

/* Light theme */
.v-theme--light .language-switcher__search-input {
  border-color: rgba(0, 0, 0, 0.12);
  background: rgba(0, 0, 0, 0.03);
}

.v-theme--light .language-switcher__search-input:focus {
  border-color: rgba(0, 140, 180, 0.4);
}

.v-theme--light .language-switcher__search-input::placeholder {
  color: rgba(0, 0, 0, 0.35);
}

.v-theme--light .language-switcher__no-results {
  color: rgba(0, 0, 0, 0.35);
}
</style>
