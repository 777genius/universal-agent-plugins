<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import { useData, useRoute, withBase } from "vitepress";
import { localeDestination, localeFromPath as routeLocale } from "./locale-routes.mjs";
import entities from "../../../generated/registries/entities.json";

type Variant = "navbar" | "screen";
type LocaleCode = "en" | "ru" | "es" | "fr" | "zh";
type RegistryEntity = {
  canonicalId?: string;
  pathEn?: string;
  pathRu?: string;
  pathEs?: string;
  pathFr?: string;
  pathZh?: string;
};

const props = withDefaults(
  defineProps<{
    variant?: Variant;
  }>(),
  {
    variant: "navbar"
  }
);

const entityByCanonicalId = new Map(
  (entities as RegistryEntity[])
    .filter((entity) => typeof entity.canonicalId === "string" && entity.canonicalId.length > 0)
    .map((entity) => [entity.canonicalId as string, entity])
);

const { page, site } = useData();
const route = useRoute();
const open = ref(false);
const rootEl = ref<HTMLElement | null>(null);

const locales = [
  { code: "en" as const, title: "English", shortTitle: "EN", basePath: "/en/" },
  { code: "ru" as const, title: "Русский", shortTitle: "RU", basePath: "/ru/" },
  { code: "es" as const, title: "Español", shortTitle: "ES", basePath: "/es/" },
  { code: "fr" as const, title: "Français", shortTitle: "FR", basePath: "/fr/" },
  { code: "zh" as const, title: "简体中文", shortTitle: "ZH", basePath: "/zh/" }
];

const currentLocale = computed(() => {
  const code = localeFromPath(route.path);
  return locales.find((locale) => locale.code === code) ?? null;
});

const currentEntity = computed(() => {
  const canonicalId = page.value.frontmatter?.canonicalId;
  return typeof canonicalId === "string" ? entityByCanonicalId.get(canonicalId) ?? null : null;
});

const fallbackLabels = {
  en: "Translation unavailable — English", ru: "Перевод недоступен — English",
  es: "Traducción no disponible — English", fr: "Traduction indisponible — English",
  zh: "暂无译文 — English"
};
const localeLinks = computed(() => locales.map((locale) => {
  const target = localeDestination(currentEntity.value, locale.code);
  return { ...locale, href: withBase(target.path), language: target.language,
    title: target.fallback ? `${locale.title} · ${fallbackLabels[locale.code]}`
      : target.home ? `${locale.title} · Home` : locale.title };
}));

const buttonLabel = computed(() => currentLocale.value?.title || "Language");
const buttonCode = computed(() => currentLocale.value?.shortTitle || "EN");
const currentLocaleCode = computed(() => currentLocale.value?.code || null);

function localeFromPath(path: string): LocaleCode | null {
  return routeLocale(path, site.value.base) as LocaleCode | null;
}

function toggle() {
  open.value = !open.value;
}

function close() {
  open.value = false;
}

function handleDocumentClick(event: MouseEvent) {
  if (!(event.target instanceof Node)) {
    return;
  }
  if (!rootEl.value?.contains(event.target)) {
    close();
  }
}

function handleEscape(event: KeyboardEvent) {
  if (event.key === "Escape") {
    close();
  }
}

onMounted(() => {
  document.addEventListener("click", handleDocumentClick);
  document.addEventListener("keydown", handleEscape);
});

onBeforeUnmount(() => {
  document.removeEventListener("click", handleDocumentClick);
  document.removeEventListener("keydown", handleEscape);
});
</script>

<template>
  <div
    ref="rootEl"
    :class="['locale-switcher', `locale-switcher--${props.variant}`, { 'is-open': open }]"
    @mouseenter="props.variant === 'navbar' ? (open = true) : undefined"
    @mouseleave="props.variant === 'navbar' ? (open = false) : undefined"
  >
    <button
      type="button"
      class="locale-switcher__button"
      :aria-expanded="open"
      aria-haspopup="true"
      :aria-label="buttonLabel"
      @click="toggle"
    >
      <span class="vpi-languages locale-switcher__button-icon" />
      <span v-if="props.variant === 'screen'" class="locale-switcher__button-text">{{ buttonLabel }}</span>
      <span v-else class="locale-switcher__button-code">{{ buttonCode }}</span>
      <span class="vpi-chevron-down locale-switcher__button-chevron" />
    </button>

    <div v-if="props.variant === 'navbar'" class="locale-switcher__menu">
      <div class="locale-switcher__panel">
        <p class="locale-switcher__title">{{ buttonLabel }}</p>
        <a
          v-for="locale in localeLinks"
          :key="locale.code"
          class="locale-switcher__link"
          :class="{ 'is-active': locale.code === currentLocaleCode }"
          :href="locale.href"
          :lang="locale.language"
          :hreflang="locale.language"
          @click="close"
        >
          <span>{{ locale.title }}</span>
        </a>
      </div>
    </div>

    <div v-else class="locale-switcher__screen" v-show="open">
      <a
        v-for="locale in localeLinks"
        :key="locale.code"
        class="locale-switcher__screen-link"
        :class="{ 'is-active': locale.code === currentLocaleCode }"
        :href="locale.href"
        :lang="locale.language"
        :hreflang="locale.language"
        @click="close"
      >
        <span>{{ locale.title }}</span>
        <span class="locale-switcher__screen-meta">{{ locale.shortTitle }}</span>
      </a>
    </div>
  </div>
</template>
