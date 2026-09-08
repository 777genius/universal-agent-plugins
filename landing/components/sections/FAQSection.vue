<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import {
  mdiPackageVariantClosed,
  mdiAccountGroupOutline,
  mdiSourceBranch,
  mdiShieldLockOutline,
  mdiRocketLaunchOutline,
  mdiCodeTags,
  mdiArrowTopRight,
  mdiHelpCircleOutline,
  mdiFrequentlyAskedQuestions,
} from '@mdi/js';

const { content } = useLandingContent();
const { t } = useI18n();
const { trackFaqExpand } = useAnalytics();

const openPanels = ref<number[]>([]);

watch(openPanels, (newVal, oldVal) => {
  const prev = new Set(oldVal ?? []);
  const opened = (newVal ?? []).filter((i) => !prev.has(i));
  for (const idx of opened) {
    const faq = content.value?.faq?.[idx];
    if (faq) trackFaqExpand(faq.id, faq.question);
  }
});

const faqIconById: Record<string, string> = {
  whatIsIt: mdiPackageVariantClosed,
  whoShouldUseIt: mdiAccountGroupOutline,
  oneRepoPerAgent: mdiSourceBranch,
  stableBoundary: mdiShieldLockOutline,
  fastestStart: mdiRocketLaunchOutline,
  whichPathFirst: mdiCodeTags,
};

const { locale } = useI18n();

const faqLabelById = computed<Record<string, string>>(() => ({
  whatIsIt: t('faq.labels.whatIsIt'),
  whoShouldUseIt: t('faq.labels.whoShouldUseIt'),
  oneRepoPerAgent: t('faq.labels.oneRepoPerAgent'),
  stableBoundary: t('faq.labels.stableBoundary'),
  fastestStart: t('faq.labels.fastestStart'),
  whichPathFirst: t('faq.labels.whichPathFirst'),
}));

const faqQuickLinks = computed(() => {
  const docsBase = `https://777genius.github.io/universal-agent-plugins/docs/${locale.value}`;

  return [
    {
      title: t('faq.quickLinks.quickstartTitle'),
      body: t('faq.quickLinks.quickstartBody'),
      href: `${docsBase}/guide/quickstart.html`,
    },
    {
      title: t('faq.quickLinks.pythonTitle'),
      body: t('faq.quickLinks.pythonBody'),
      href: `${docsBase}/guide/python-runtime.html`,
    },
    {
      title: t('faq.quickLinks.boundaryTitle'),
      body: t('faq.quickLinks.boundaryBody'),
      href: `${docsBase}/reference/support-boundary.html`,
    },
  ];
});
</script>

<template>
  <section id="faq" class="faq-section section anchor-offset">
    <v-container>
      <div class="faq-section__header">
        <h2 class="faq-section__title">{{ t('faq.sectionTitle') }}</h2>
        <p class="faq-section__subtitle">{{ t('faq.subtitle') }}</p>
      </div>

      <div class="faq-section__content">
        <div class="faq-section__list">
          <v-expansion-panels
            v-model="openPanels"
            multiple
            variant="accordion"
            class="faq-section__panels"
          >
            <v-expansion-panel
              v-for="(item, index) in content.faq"
              :key="item.id"
              class="faq-section__panel"
              :style="{ '--delay': `${index * 0.08}s` }"
              elevation="0"
            >
              <v-expansion-panel-title class="faq-section__panel-title">
                <div class="faq-section__panel-header">
                  <div class="faq-section__panel-icon-wrap">
                    <v-icon
                      size="22"
                      class="faq-section__panel-icon"
                      :icon="faqIconById[item.id] || mdiHelpCircleOutline"
                    />
                  </div>
                  <div class="faq-section__panel-copy">
                    <span class="faq-section__panel-label">{{
                      faqLabelById[item.id] || t('faq.labels.default')
                    }}</span>
                    <span class="faq-section__panel-question">{{ item.question }}</span>
                  </div>
                </div>
              </v-expansion-panel-title>
              <v-expansion-panel-text class="faq-section__panel-text">
                <!-- eslint-disable-next-line vue/no-v-html -->
                <div class="faq-section__answer" v-html="item.answer" />
              </v-expansion-panel-text>
            </v-expansion-panel>
          </v-expansion-panels>
        </div>

        <div class="faq-section__decoration">
          <div class="faq-section__guide-card">
            <div class="faq-section__guide-badge">
              <v-icon
                size="18"
                class="faq-section__guide-badge-icon"
                :icon="mdiFrequentlyAskedQuestions"
              />
              <span>{{ t('faq.quickLinks.badge') }}</span>
            </div>
            <h3 class="faq-section__guide-title">{{ t('faq.quickLinks.title') }}</h3>
            <p class="faq-section__guide-text">{{ t('faq.quickLinks.subtitle') }}</p>
            <a
              v-for="link in faqQuickLinks"
              :key="link.href"
              :href="link.href"
              class="faq-section__guide-link"
            >
              <div class="faq-section__guide-link-copy">
                <span class="faq-section__guide-link-title">{{ link.title }}</span>
                <span class="faq-section__guide-link-body">{{ link.body }}</span>
              </div>
              <v-icon size="18" class="faq-section__guide-link-icon" :icon="mdiArrowTopRight" />
            </a>
          </div>
        </div>
      </div>
    </v-container>
  </section>
</template>

<style scoped>
.faq-section {
  position: relative;
}

.faq-section__header {
  text-align: center;
  max-width: 640px;
  margin: 0 auto 56px;
  position: relative;
  z-index: 1;
}

.faq-section__title {
  font-size: 2.4rem;
  font-weight: 800;
  letter-spacing: -0.03em;
  line-height: 1.15;
  margin-bottom: 16px;
  background: linear-gradient(135deg, #e0e6ff 0%, #ffd700 100%);
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
}

.faq-section__subtitle {
  font-size: 1.1rem;
  color: #8892b0;
  line-height: 1.6;
  margin: 0;
}

.faq-section__content {
  display: grid;
  grid-template-columns: minmax(0, 1.45fr) minmax(360px, 0.9fr);
  gap: 32px;
  align-items: start;
  position: relative;
  z-index: 1;
}

.faq-section__list {
  min-width: 0;
}

.faq-section__panels {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.faq-section__panel {
  border-radius: 16px !important;
  background: rgba(10, 10, 15, 0.8) !important;
  border: 1px solid rgba(0, 240, 255, 0.08) !important;
  backdrop-filter: blur(12px);
  transition:
    transform 0.3s ease,
    box-shadow 0.3s ease,
    border-color 0.3s ease;
  overflow: hidden;
  animation: faqFadeIn 0.5s ease both;
  animation-delay: var(--delay, 0s);
}

.faq-section__panel:hover {
  transform: translateY(-2px);
  border-color: rgba(0, 240, 255, 0.2) !important;
  box-shadow: 0 8px 32px rgba(0, 240, 255, 0.06);
}

.faq-section__panel::after {
  display: none;
}

:deep(.faq-section__panel .v-expansion-panel__shadow) {
  display: none;
}

.faq-section__panel-title {
  padding: 20px 24px !important;
  min-height: unset !important;
}

:deep(.faq-section__panel-title .v-expansion-panel-title__overlay) {
  opacity: 0 !important;
}

.faq-section__panel-header {
  display: flex;
  align-items: flex-start;
  gap: 16px;
  width: 100%;
}

.faq-section__panel-copy {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 0;
}

.faq-section__panel-icon-wrap {
  flex-shrink: 0;
  width: 42px;
  height: 42px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, rgba(0, 240, 255, 0.1), rgba(255, 0, 255, 0.08));
  border: 1px solid rgba(0, 240, 255, 0.12);
  transition: background 0.3s ease;
}

.faq-section__panel:hover .faq-section__panel-icon-wrap {
  background: linear-gradient(135deg, rgba(0, 240, 255, 0.16), rgba(255, 0, 255, 0.12));
}

.faq-section__panel-icon {
  color: #00f0ff;
}

.faq-section__panel-label {
  font-size: 0.72rem;
  font-weight: 700;
  letter-spacing: 0.14em;
  text-transform: uppercase;
  color: rgba(0, 240, 255, 0.72);
}

.faq-section__panel-question {
  font-size: 1rem;
  font-weight: 600;
  line-height: 1.4;
  color: #e0e6ff;
  display: block;
}

:deep(.faq-section__panel-text .v-expansion-panel-text__wrapper) {
  padding: 0 24px 20px 82px !important;
}

.faq-section__answer {
  font-size: 0.95rem;
  line-height: 1.7;
  color: #8892b0;
}

.faq-section__answer :deep(a) {
  color: #00f0ff;
  text-decoration: none;
  font-weight: 500;
}

.faq-section__answer :deep(a:hover) {
  text-decoration: underline;
}

/* Decoration */
.faq-section__decoration {
  position: relative;
  align-self: stretch;
}

.faq-section__guide-card {
  position: sticky;
  top: 104px;
  display: flex;
  flex-direction: column;
  min-height: 100%;
  gap: 18px;
  padding: 24px;
  border-radius: 24px;
  background:
    linear-gradient(180deg, rgba(12, 18, 28, 0.96), rgba(8, 10, 18, 0.92)),
    linear-gradient(135deg, rgba(0, 240, 255, 0.12), rgba(255, 0, 255, 0.08));
  border: 1px solid rgba(0, 240, 255, 0.12);
  box-shadow: 0 18px 48px rgba(0, 0, 0, 0.34);
}

.faq-section__guide-badge {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  align-self: flex-start;
  padding: 8px 12px;
  border-radius: 999px;
  background: rgba(0, 240, 255, 0.08);
  border: 1px solid rgba(0, 240, 255, 0.14);
  color: #a4eef7;
  font-size: 0.74rem;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.faq-section__guide-badge-icon {
  color: #00f0ff;
}

.faq-section__guide-title {
  margin: 0;
  font-size: 1.35rem;
  line-height: 1.15;
  color: #f7f8ff;
}

.faq-section__guide-text {
  margin: 0;
  color: #91a0bf;
  font-size: 0.96rem;
  line-height: 1.65;
}

.faq-section__guide-link {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  padding: 16px 18px;
  border-radius: 16px;
  text-decoration: none;
  color: inherit;
  background: rgba(255, 255, 255, 0.02);
  border: 1px solid rgba(0, 240, 255, 0.08);
  transition:
    transform 0.25s ease,
    border-color 0.25s ease,
    background 0.25s ease;
}

.faq-section__guide-link:hover {
  transform: translateY(-2px);
  background: rgba(0, 240, 255, 0.05);
  border-color: rgba(0, 240, 255, 0.2);
}

.faq-section__guide-link-copy {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.faq-section__guide-link-title {
  color: #eef2ff;
  font-weight: 700;
  line-height: 1.3;
}

.faq-section__guide-link-body {
  color: #8892b0;
  font-size: 0.9rem;
  line-height: 1.55;
}

.faq-section__guide-link-icon {
  color: #00f0ff;
  flex-shrink: 0;
  margin-top: 2px;
}

@keyframes faqFadeIn {
  from {
    opacity: 0;
    transform: translateY(16px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

/* Light Theme */
.v-theme--light .faq-section__title {
  background: linear-gradient(135deg, #1e293b 0%, #d97706 100%);
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
}

.v-theme--light .faq-section__subtitle {
  color: #475569;
}

.v-theme--light .faq-section__panel {
  background: rgba(255, 255, 255, 0.75) !important;
  border-color: rgba(0, 0, 0, 0.06) !important;
}

.v-theme--light .faq-section__panel:hover {
  box-shadow: 0 8px 32px rgba(0, 180, 200, 0.08);
}

.v-theme--light .faq-section__panel-question {
  color: #1e293b;
}

.v-theme--light .faq-section__answer {
  color: #475569;
}

.v-theme--light .faq-section__panel-label {
  color: rgba(8, 145, 178, 0.82);
}

.v-theme--light .faq-section__guide-card {
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.96), rgba(244, 248, 255, 0.92)),
    linear-gradient(135deg, rgba(8, 145, 178, 0.08), rgba(217, 119, 6, 0.06));
  border-color: rgba(8, 145, 178, 0.12);
  box-shadow: 0 16px 40px rgba(15, 23, 42, 0.08);
}

.v-theme--light .faq-section__guide-badge {
  background: rgba(8, 145, 178, 0.08);
  border-color: rgba(8, 145, 178, 0.12);
  color: #0f766e;
}

.v-theme--light .faq-section__guide-badge-icon,
.v-theme--light .faq-section__guide-link-icon {
  color: #0891b2;
}

.v-theme--light .faq-section__guide-title,
.v-theme--light .faq-section__guide-link-title {
  color: #0f172a;
}

.v-theme--light .faq-section__guide-text,
.v-theme--light .faq-section__guide-link-body {
  color: #475569;
}

.v-theme--light .faq-section__guide-link {
  background: rgba(255, 255, 255, 0.7);
  border-color: rgba(8, 145, 178, 0.08);
}

.v-theme--light .faq-section__guide-link:hover {
  background: rgba(240, 249, 255, 0.9);
  border-color: rgba(8, 145, 178, 0.16);
}

@media (max-width: 960px) {
  .faq-section__header {
    margin-bottom: 40px;
  }
  .faq-section__title {
    font-size: 1.85rem;
  }
  .faq-section__subtitle {
    font-size: 1rem;
  }
  .faq-section__content {
    grid-template-columns: 1fr;
    gap: 40px;
  }
  .faq-section__guide-card {
    position: static;
  }
}

@media (max-width: 600px) {
  .faq-section__header {
    margin-bottom: 32px;
  }
  .faq-section__title {
    font-size: 1.6rem;
  }
  .faq-section__panel-title {
    padding: 16px 18px !important;
  }
  .faq-section__panel-icon-wrap {
    width: 36px;
    height: 36px;
    border-radius: 10px;
  }
  .faq-section__panel-header {
    gap: 12px;
  }
  .faq-section__panel-label {
    font-size: 0.66rem;
    letter-spacing: 0.12em;
  }
  .faq-section__panel-question {
    font-size: 0.92rem;
  }
  :deep(.faq-section__panel-text .v-expansion-panel-text__wrapper) {
    padding: 0 18px 16px 18px !important;
  }
  .faq-section__answer {
    font-size: 0.9rem;
  }
  .faq-section__guide-card {
    padding: 20px;
  }
}
</style>
