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

const currentName = computed(() => items.value.find(item => item.value === locale.value)?.title);
const currentFlagIcon = computed(() => flagIconMap[locale.value] ?? "circle-flags:xx");
const menuOpen = ref(false);
watch(menuOpen, open => {
  if (!open) returnFocus();
});

const onChange = async (value: unknown) => {
  if (typeof value !== 'string' || pending.value) return;
  if (value === locale.value) {
    menuOpen.value = false;
    returnFocus();
    return;
  }
  retryLocale.value = value;
  await switchLocale(value);
  menuOpen.value = false;
  returnFocus();
};
</script>

<template>
  <v-menu v-model="menuOpen" location="bottom end" :close-on-content-click="false">
    <template #activator="{ props: menuProps }">
      <v-btn
        ref="activator"
        v-bind="menuProps"
        :variant="props.compact || props.iconOnly ? 'text' : 'outlined'"
        :block="props.fullWidth"
        :size="props.compact ? 'small' : 'default'"
        :disabled="pending"
        :loading="pending"
        :aria-busy="pending"
        :aria-label="`${t('language.label')}: ${currentName}`"
      >
        <Icon :name="currentFlagIcon" class="language-switcher__flag-icon" aria-hidden="true" />
        <span v-if="!props.iconOnly" class="language-switcher__name">{{ currentName }}</span>
      </v-btn>
    </template>
    <v-list density="compact" role="menu" :aria-label="t('language.label')" class="language-switcher__menu-list">
      <v-list-item
        v-for="item in items"
        :key="item.value"
        role="menuitemradio"
        :aria-checked="item.value === locale"
        :active="item.value === locale"
        :disabled="pending"
        @click="onChange(item.value)"
      >
        <template #title>
          <span class="language-switcher__item">
            <Icon :name="item.flagIcon" class="language-switcher__flag-icon" aria-hidden="true" />
            <span>{{ item.title }}</span>
          </span>
        </template>
      </v-list-item>
    </v-list>
  </v-menu>
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

.language-switcher__name {
  margin-inline-start: 8px;
}

.language-switcher__item {
  display: flex;
  align-items: center;
  gap: 8px;
}

.language-switcher__menu-list {
  min-width: 180px;
}
</style>
