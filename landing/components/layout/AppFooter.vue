<script setup lang="ts">
const { t } = useI18n();
const config = useRuntimeConfig();
const year = new Date().getFullYear();
const githubUrl = `https://github.com/${config.public.githubRepo}`;
const githubOwner =
  (config.public.githubRepo || '777genius/universal-agent-plugins').split('/')[0] || '777genius';
const githubOwnerUrl = `https://github.com/${githubOwner}`;
</script>

<template>
  <footer class="app-footer">
    <v-container class="app-footer__inner">
      <span class="app-footer__copy">
        {{ t('footer.copyright', { year }) }}
        <a
          class="app-footer__author"
          :href="githubOwnerUrl"
          target="_blank"
          rel="noopener noreferrer"
          >{{ githubOwner }}</a
        >
        · {{ t('footer.tagline') }}
      </span>
      <div class="app-footer__links">
        <a class="app-footer__link" :href="githubUrl" target="_blank" rel="noopener noreferrer">{{
          t('footer.github')
        }}</a>
        <span class="app-footer__divider" />
        <a class="app-footer__link" href="https://www.apache.org/licenses/LICENSE-2.0" target="_blank" rel="noopener noreferrer">Apache 2.0</a>
      </div>
    </v-container>
  </footer>
</template>

<style scoped>
.app-footer {
  border-top: 1px solid rgba(0, 240, 255, 0.08);
  padding: 20px 0;
}

.app-footer__inner {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.app-footer__copy {
  font-size: 13px;
  opacity: 0.5;
  font-family: 'JetBrains Mono', monospace;
}

.app-footer__author {
  color: inherit;
  text-decoration: none;
  border-bottom: 1px solid transparent;
  transition:
    border-color 0.2s ease,
    opacity 0.2s ease;
}

.app-footer__author:hover {
  opacity: 1;
  border-bottom-color: currentColor;
}

.app-footer__links {
  display: flex;
  align-items: center;
  gap: 12px;
}

.app-footer__link {
  color: #00f0ff;
  text-decoration: none;
  font-size: 13px;
  opacity: 0.7;
  transition: opacity 0.2s ease;
  font-family: 'JetBrains Mono', monospace;
}

.app-footer__link:hover {
  opacity: 1;
}

.app-footer__divider {
  width: 1px;
  height: 14px;
  background: rgba(0, 240, 255, 0.2);
}

.v-theme--light .app-footer {
  border-top-color: rgba(0, 0, 0, 0.08);
}

@media (max-width: 600px) {
  .app-footer__inner {
    flex-direction: column;
    gap: 10px;
    text-align: center;
  }
}
</style>
