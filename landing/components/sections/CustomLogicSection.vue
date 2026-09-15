<script setup lang="ts">
import CommandSnippetCard from '~/components/shared/CommandSnippetCard.vue';

const { t } = useI18n();
const { buildGuideUrl } = useDocsLinks();

const pathCards = computed(() => [
  {
    id: 'online-service',
    title: t('hero.paths.onlineService.title'),
    description: t('hero.paths.onlineService.description'),
    command:
      "agentplugins author init ./my-plugin --template mcp-remote --name my-plugin --description 'Connect an online service' --url https://example.com/mcp",
    href: buildGuideUrl.value,
    cta: t('createPlugin.onlineServiceCta'),
    accent: '#00f0ff',
  },
  {
    id: 'local-tool',
    title: t('hero.paths.localTool.title'),
    description: t('hero.paths.localTool.description'),
    command:
      "agentplugins author init ./my-plugin --template mcp-stdio --name my-plugin --description 'Provide local tools' --runtime node",
    href: buildGuideUrl.value,
    cta: t('createPlugin.localToolCta'),
    accent: '#ff6ee7',
  },
  {
    id: 'custom-logic',
    title: t('hero.paths.customLogic.title'),
    description: t('createPlugin.customLogicDescription'),
    command:
      "agentplugins author init ./my-plugin --template hybrid --mcp-template mcp-remote --name my-plugin --description 'Instructions and tools' --url https://example.com/mcp",
    href: buildGuideUrl.value,
    cta: t('createPlugin.customLogicCta'),
    badge: t('hero.paths.customLogic.badge'),
    accent: '#ffd166',
  },
]);
</script>

<template>
  <section id="custom-logic" class="custom-logic-section section">
    <v-container>
      <div class="custom-logic-section__header">
        <p class="custom-logic-section__eyebrow">
          {{ t('createPlugin.eyebrow') }}
        </p>
        <h2 class="custom-logic-section__title">
          {{ t('createPlugin.title') }}
        </h2>
        <p class="custom-logic-section__subtitle">
          {{ t('createPlugin.subtitle') }}
        </p>
      </div>

      <div class="custom-logic-section__grid">
        <article v-for="path in pathCards" :key="path.id" class="custom-logic-section__card">
          <div class="custom-logic-section__card-top">
            <h3 class="custom-logic-section__card-title">{{ path.title }}</h3>
            <span v-if="path.badge" class="custom-logic-section__badge">{{ path.badge }}</span>
          </div>
          <p class="custom-logic-section__card-copy">{{ path.description }}</p>
          <CommandSnippetCard
            class="custom-logic-section__command"
            :label="t('download.command')"
            :command="path.command"
            :copy-label="t('download.copy')"
            :copied-label="t('download.copied')"
            :accent="path.accent"
          />
          <a
            class="custom-logic-section__link"
            :href="path.href"
            target="_blank"
            rel="noopener noreferrer"
          >
            {{ path.cta }}
          </a>
        </article>
      </div>
    </v-container>
  </section>
</template>

<style scoped>
.custom-logic-section__header {
  max-width: 900px;
  margin: 0 auto 28px;
  text-align: center;
}

.custom-logic-section__eyebrow {
  margin: 0 0 12px;
  color: #ffd166;
  text-transform: uppercase;
  letter-spacing: 0.16em;
  font-size: 0.74rem;
  font-family: 'JetBrains Mono', monospace;
}

.custom-logic-section__title {
  margin: 0 0 14px;
  font-size: clamp(2rem, 4vw, 3rem);
  line-height: 1.05;
  letter-spacing: -0.04em;
  color: #eff6ff;
}

.custom-logic-section__subtitle {
  margin: 0 auto;
  color: #91a0bf;
  font-size: 1.02rem;
  line-height: 1.72;
  max-width: 760px;
}

.custom-logic-section__grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 20px;
}

.custom-logic-section__card {
  border-radius: 24px;
  border: 1px solid rgba(255, 209, 102, 0.14);
  background:
    radial-gradient(circle at top right, rgba(255, 209, 102, 0.08) 0%, transparent 38%),
    rgba(10, 10, 15, 0.82);
  box-shadow: 0 20px 70px rgba(0, 0, 0, 0.24);
  backdrop-filter: blur(14px);
  padding: 24px;
  display: flex;
  flex-direction: column;
}

.custom-logic-section__card-top {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
}

.custom-logic-section__card-title {
  margin: 0;
  color: #eff6ff;
  font-size: 1.2rem;
  line-height: 1.2;
}

.custom-logic-section__badge {
  flex-shrink: 0;
  border-radius: 999px;
  padding: 5px 10px;
  font-size: 0.66rem;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  font-weight: 700;
  background: rgba(255, 209, 102, 0.1);
  color: #ffd166;
  border: 1px solid rgba(255, 209, 102, 0.18);
}

.custom-logic-section__card-copy {
  margin: 0 0 14px;
  color: #91a0bf;
  line-height: 1.68;
  font-size: 0.96rem;
}

.custom-logic-section__command {
  margin-top: auto;
}

.custom-logic-section__link {
  display: inline-flex;
  margin-top: 16px;
  color: #ffd166;
  font-size: 0.82rem;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  text-decoration: none;
}

.custom-logic-section__link:hover {
  text-decoration: underline;
}

@media (max-width: 1100px) {
  .custom-logic-section__grid {
    grid-template-columns: 1fr;
  }
}
</style>
