<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useData, useRoute, withBase } from "vitepress";
import { localeDestination, localeFromPath as routeLocale } from "./locale-routes.mjs";
import { docsLocaleEntries } from "../../../tools/config/locales.mjs";
import entities from "../../../generated/registries/entities.json";

type Variant = "navbar" | "screen";
type LocaleCode = "en" | "ru" | "uk" | "zh" | "es" | "hi" | "ar" | "pt" | "fr";
type RegistryEntity = {
  canonicalId?: string;
  anchorsByLocale?: Partial<Record<LocaleCode, string[]>>;
} & Partial<Record<`path${Capitalize<LocaleCode>}`, string>>;


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
const currentHash = ref("");
function syncHash() { currentHash.value = window.location.hash; }
const open = ref(false);
const openedByHover = ref(false);
const rootEl = ref<HTMLElement | null>(null);

const locales = docsLocaleEntries.map((locale) => ({ ...locale, title: locale.label, shortTitle: locale.code.toUpperCase() }));

const currentLocale = computed(() => {
  const code = localeFromPath(route.path);
  return locales.find((locale) => locale.code === code) ?? null;
});

const currentEntity = computed(() => {
  const canonicalId = page.value.frontmatter?.canonicalId;
  return typeof canonicalId === "string" ? entityByCanonicalId.get(canonicalId) ?? null : null;
});

const localeLinks = computed(() => locales.map((locale) => {
  const target = localeDestination(currentEntity.value, locale.code, currentHash.value);
  return { ...locale, href: withBase(target.path), language: target.language,
    title: target.fallback ? `${locale.title} · ${locale.fallback}`
      : target.home ? `${locale.title} · ${locale.home}` : locale.title };
}));

const buttonLabel = computed(() => currentLocale.value?.title || "Language");
const buttonCode = computed(() => currentLocale.value?.shortTitle || "EN");
const currentLocaleCode = computed(() => currentLocale.value?.code || null);

function localeFromPath(path: string): LocaleCode | null {
  return routeLocale(path, site.value.base) as LocaleCode | null;
}

function toggle(event: MouseEvent) {
  syncHash();
  if (event.detail > 0 && openedByHover.value) openedByHover.value = false;
  else open.value = !open.value;
}
function enter() {
  if (props.variant !== "navbar") return;
  syncHash();
  if (!open.value) openedByHover.value = true;
  open.value = true;
}

function close() {
  open.value = false;
  openedByHover.value = false;
}

function handleDocumentClick(event: MouseEvent) {
  if (!(event.target instanceof Node)) {
    return;
  }
  if (!rootEl.value?.contains(event.target)) {
    close();
  }
}

function handleFocusOut(event: FocusEvent) {
  if (!rootEl.value?.contains(event.relatedTarget as Node | null)) close();
}

function handleEscape(event: KeyboardEvent) {
  if (event.key === "Escape" && open.value) {
    close();
    rootEl.value?.querySelector("button")?.focus();
  }
}

onMounted(() => {
  syncHash();
  watch(() => route.path, syncHash, { flush: "post" });
  window.addEventListener("hashchange", syncHash);
  document.addEventListener("click", handleDocumentClick);
  document.addEventListener("keydown", handleEscape);
});

onBeforeUnmount(() => {
  window.removeEventListener("hashchange", syncHash);
  document.removeEventListener("click", handleDocumentClick);
  document.removeEventListener("keydown", handleEscape);
});
</script>

<template>
  <div
    ref="rootEl"
    :class="['locale-switcher', `locale-switcher--${props.variant}`, { 'is-open': open }]"
    @keydown.down.prevent="syncHash(); open = true"
    @focusout="handleFocusOut"
    @mouseenter="enter"
    @mouseleave="props.variant === 'navbar' ? close() : undefined"
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
          :dir="locale.language === 'ar' ? 'rtl' : 'ltr'"
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
        :dir="locale.language === 'ar' ? 'rtl' : 'ltr'"
        :hreflang="locale.language"
        @click="close"
      >
        <span>{{ locale.title }}</span>
        <span class="locale-switcher__screen-meta">{{ locale.shortTitle }}</span>
      </a>
    </div>
  </div>
</template>
