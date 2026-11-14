<script setup lang="ts">
import { computed } from "vue";
import { useData } from "vitepress";
import DefaultTheme from "vitepress/theme";
import { useMermaidZoom } from "vitepress-mermaid-zoom";
import { useCodeblockCollapse } from "vitepress-codeblock-collapse";
import LocalePreferenceSync from "./LocalePreferenceSync.vue";
import LocaleSwitcher from "./LocaleSwitcher.vue";
import "vitepress-mermaid-zoom/style.css";
import "vitepress-codeblock-collapse/style.css";

const { page, frontmatter, lang } = useData();
const pagePath = computed(() => page.value.relativePath);
const archiveMessage = computed(() => lang.value === "ru-RU"
  ? "Архив: эта страница описывает снятый с поддержки продукт. Для новых проектов используйте agentplugins author и plugin.json. Миграция YAML не планируется."
  : "Archive: this page describes a retired product. For new projects, use agentplugins author and plugin.json. YAML migration is not planned.");

useMermaidZoom(pagePath);

useCodeblockCollapse(pagePath);
</script>

<template>
  <DefaultTheme.Layout>
    <template #layout-top>
      <LocalePreferenceSync />
    </template>
    <template #nav-bar-content-after>
      <LocaleSwitcher />
    </template>
    <template #nav-screen-content-after>
      <LocaleSwitcher variant="screen" />
    </template>
    <template #doc-before>
      <div v-if="frontmatter.retiredArchive" class="retired-archive-notice" role="note">
        {{ archiveMessage }}
      </div>
    </template>
  </DefaultTheme.Layout>
</template>
