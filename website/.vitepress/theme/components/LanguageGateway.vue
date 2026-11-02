<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { withBase } from "vitepress";

import { preferredLocale as selectLocale } from "./locale-routes.mjs";
import { docsLocaleEntries } from "../../../tools/config/locales.mjs";

const storageKey = "plugin-kit-ai-docs-locale";
const locales = docsLocaleEntries.map((locale) => ({
  ...locale, title: locale.label, href: withBase(`/${locale.code}/`)
}));

const preferredCode = ref("");
const manualMode = ref(false);
const preferredLocale = computed(() => locales.find((locale) => locale.code.toLowerCase() === preferredCode.value) || null);

onMounted(() => {
  try {
    preferredCode.value = (window.localStorage.getItem(storageKey) || "").toLowerCase();
  } catch {
    preferredCode.value = "";
  }

  try {
    const query = new URLSearchParams(window.location.search);
    manualMode.value = ["1", "true", "manual"].includes((query.get("gateway") || "").toLowerCase());
  } catch {
    manualMode.value = false;
  }

  if (!manualMode.value) {
    redirectToPreferredLocale();
  }
});

function rememberLocale(code: string) {
  try {
    window.localStorage.setItem(storageKey, code.toLowerCase());
  } catch {
    // localStorage is optional enhancement only.
  }
}

function redirectToPreferredLocale() {
  const targetCode = selectLocale(preferredCode.value, [
    ...(Array.isArray(window.navigator.languages) ? window.navigator.languages : []),
    window.navigator.language || ""
  ]);
  const targetLocale = locales.find((locale) => locale.code.toLowerCase() === targetCode.toLowerCase());
  if (!targetLocale) {
    return;
  }
  rememberLocale(targetLocale.code);
  window.location.replace(targetLocale.href);
}
</script>

<template>
  <div class="language-gateway">
    <div class="language-gateway__intro">
      <p class="language-gateway__eyebrow">Agent Plugins docs</p>
      <h1>Choose your language</h1>
      <p>
        Install, manage and build plugins. Read the guides in your language.
      </p>
      <div v-if="preferredLocale" class="language-gateway__preferred">
        <span>Your preferred language: {{ preferredLocale.title }}</span>
        <a :href="preferredLocale.href">Continue in {{ preferredLocale.title }}</a>
      </div>
    </div>
    <div class="language-gateway__grid">
      <a
        v-for="locale in locales"
        :key="locale.code"
        :href="locale.href"
        class="language-gateway__card"
        :lang="locale.lang"
        :dir="locale.dir"
        :hreflang="locale.lang"
        @click="rememberLocale(locale.code)"
      >
        <span class="language-gateway__code">{{ locale.code.toUpperCase() }}</span>
        <strong>{{ locale.title }}</strong>
        <small>{{ locale.description }}</small>
      </a>
    </div>
  </div>
</template>
