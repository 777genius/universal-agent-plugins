<script setup lang="ts">
import { onMounted, watch } from "vue";
import { useData, useRoute } from "vitepress";

import { localeFromPath as routeLocale } from "./locale-routes.mjs";

const storageKey = "plugin-kit-ai-docs-locale";
const { site } = useData();
const route = useRoute();

function persistLocale(path: string) {
  if (typeof window === "undefined") {
    return;
  }
  const locale = routeLocale(path, site.value.base);
  if (!locale) {
    return;
  }
  try {
    window.localStorage.setItem(storageKey, locale);
  } catch {
    // localStorage is optional enhancement only.
  }
}

onMounted(() => {
  persistLocale(route.path);
  watch(
    () => route.path,
    (path) => persistLocale(path)
  );
});
</script>

<template>
  <span hidden aria-hidden="true"></span>
</template>
