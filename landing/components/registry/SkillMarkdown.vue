<script setup lang="ts">
import { renderSkillMarkdown } from '~/utils/skillMarkdown';

const props = defineProps<{ markdown: string }>();
const html = computed(() => renderSkillMarkdown(props.markdown));
</script>

<template>
  <!-- Only Markdown-it's HTML-disabled, protocol-filtered output reaches v-html. -->
  <div class="skill-markdown" v-html="html" />
</template>
