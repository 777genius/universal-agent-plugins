<script setup lang="ts">
import { renderSkillMarkdown } from '~/utils/skillMarkdown';

const props = defineProps<{ markdown: string; rawLabel: string }>();
const html = computed(() => renderSkillMarkdown(props.markdown));
</script>

<template>
  <div class="skill-markdown">
    <!-- Only Markdown-it's HTML-disabled, protocol-filtered output reaches v-html. -->
    <div class="skill-markdown__rendered" v-html="html" />
    <details class="skill-markdown__raw">
      <summary>{{ rawLabel }}</summary>
      <!-- The exact complete document, including YAML frontmatter, is escaped by Vue. -->
      <pre><code>{{ markdown }}</code></pre>
    </details>
  </div>
</template>
