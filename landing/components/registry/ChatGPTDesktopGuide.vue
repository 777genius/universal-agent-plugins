<script setup lang="ts">
defineProps<{ command: string }>();
const { t } = useI18n();
</script>

<template>
  <details class="desktop-guide">
    <summary>{{ t('registryUi.desktopGuide.title') }}</summary>
    <p>{{ t('registryUi.desktopGuide.description') }}</p>
    <CommandSnippet
      :command="command"
      :label="t('registryUi.desktopGuide.command')"
      kind="add"
    />
    <p>{{ t('registryUi.desktopGuide.nextStep') }}</p>
    <a
      href="https://github.com/777genius/universal-agent-plugins/blob/main/docs/CHATGPT_DESKTOP_LOCAL_INSTALL.md"
      target="_blank"
      rel="noopener noreferrer"
      >{{ t('registryUi.desktopGuide.help') }}</a
    >
  </details>
</template>

<style scoped>
.desktop-guide {
  min-width: 0;
  padding: 12px;
  border: 1px solid var(--line);
  border-radius: 12px;
  color: var(--text);
  background: var(--surface-raised);
}
.desktop-guide summary {
  cursor: pointer;
  font-size: 0.85rem;
  font-weight: 750;
}
.desktop-guide summary:focus-visible {
  outline: 3px solid var(--cyan);
  outline-offset: 3px;
}
.desktop-guide p {
  margin: 10px 0;
  color: var(--muted);
  font-size: 0.78rem;
  line-height: 1.55;
}
.desktop-guide a {
  color: var(--primary-bright);
  font-size: 0.78rem;
  text-underline-offset: 2px;
}
.desktop-guide :deep(.command-snippet) {
  min-width: 0;
}
.desktop-guide :deep(.command-snippet pre) {
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
</style>
