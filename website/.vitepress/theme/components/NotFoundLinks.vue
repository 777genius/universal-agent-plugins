<script setup lang="ts">
import { withBase } from "vitepress";

import { docsLocaleEntries } from "../../../tools/config/locales.mjs";
const links = [
  { title: "Choose a language", description: "Open the documentation language guide.", href: withBase("/?gateway=manual"), lang: "en-US", dir: "ltr" },
  ...docsLocaleEntries.map(({ label, description, lang, dir, code }) => ({ title: label, description, lang, dir, href: withBase(`/${code}/`) }))
];
</script>

<template>
  <div class="docs-grid">
    <a v-for="link in links" :key="link.href" class="docs-card" :href="link.href" :lang="link.lang" :dir="link.dir">
      <h2>{{ link.title }}</h2>
      <p>{{ link.description }}</p>
    </a>
  </div>
</template>
