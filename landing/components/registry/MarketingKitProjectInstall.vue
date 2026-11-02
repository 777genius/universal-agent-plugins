<script setup lang="ts">
import { ref } from 'vue';
import {
  marketingKitProjectManifest,
  marketingKitInstallCommand,
} from '~/data/marketingKitDemo';

const { t } = useI18n();
const copied = ref(false);
async function copyManifest() {
  try {
    await navigator.clipboard.writeText(marketingKitProjectManifest);
    copied.value = true;
  } catch {
    copied.value = false;
  }
}
</script>

<template>
  <div class="marketing-kit-project">
    <p class="marketing-kit-project__intro">{{ t('registryUi.install.marketingKitProject.intro') }}</p>
    <div class="marketing-kit-project__file">
      <div class="marketing-kit-project__file-heading">
        <strong>uap.toml</strong>
        <button
          type="button"
          :aria-label="t('registryUi.command.copyLabel')"
          @click="copyManifest"
        >{{ copied ? t('registryUi.command.copied') : t('registryUi.command.copy') }}</button>
      </div>
      <pre dir="ltr"><code>{{ marketingKitProjectManifest }}</code></pre>
    </div>
    <p class="marketing-kit-project__agents">{{ t('registryUi.install.marketingKitProject.agents') }}</p>
    <CommandSnippet
      kind="add"
      :label="t('registryUi.card.install')"
      :command="marketingKitInstallCommand"
      compact
    />
    <p class="marketing-kit-project__help">{{ t('registryUi.install.marketingKitProject.help') }}</p>
  </div>
</template>

<style scoped>
.marketing-kit-project {
  display: grid;
  gap: 11px;
  min-width: 0;
}
.marketing-kit-project p { margin: 0; line-height: 1.55; }
.marketing-kit-project__intro,
.marketing-kit-project__help { color: var(--muted); font-size: .83rem; }
.marketing-kit-project__agents { color: var(--text); font-size: .8rem; font-weight: 600; }
.marketing-kit-project__file {
  min-width: 0;
  overflow: hidden;
  border-radius: 12px;
  border: 1px solid color-mix(in srgb, var(--primary-bright) 42%, var(--line));
  background: color-mix(in srgb, var(--surface-solid) 92%, black);
}
.marketing-kit-project__file-heading {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 8px;
  padding: 9px 12px;
  border-bottom: 1px solid var(--line);
  color: var(--text);
  font-size: .8rem;
}
.marketing-kit-project__file-heading button {
  min-height: 30px;
  border: 0;
  border-radius: 6px;
  padding-inline: 9px;
  background: transparent;
  color: var(--primary-bright);
  cursor: pointer;
  font-size: .75rem;
  font-weight: 700;
}
.marketing-kit-project__file-heading button:focus-visible {
  outline: 2px solid var(--cyan);
  outline-offset: 2px;
}
.marketing-kit-project__file pre {
  margin: 0;
  padding: 12px;
  max-width: 100%;
  overflow-x: auto;
  color: #c9e9fb;
  font-size: clamp(.65rem, 1.25vw, .76rem);
  font-family: 'JetBrains Mono', monospace;
  line-height: 1.55;
  text-align: left;
  white-space: pre;
}
</style>
