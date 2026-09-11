<script setup lang="ts">
import type { DocsLocale } from '~/utils/docsLinks';
import { replaceDocsLocale } from '~/utils/docsLinks';

const { t, locale } = useI18n();
const config = useRuntimeConfig();

const currentLocale = computed<DocsLocale>(() => {
  const supported = new Set<DocsLocale>(['en', 'ru', 'es', 'fr', 'zh']);
  return supported.has(locale.value as DocsLocale) ? (locale.value as DocsLocale) : 'en';
});

// The template always carries an "en" locale segment; replaceDocsLocale
// rewrites it to the resolved locale (falling back to English per-docPath,
// unlike a plain string suffix on docsUrl, which only fallback-checks the
// docs root and would silently serve untranslated pages once landing gets
// locale-aware routing).
const docsRoot = computed(() => {
  const url = String(config.public.docsUrl || 'https://777genius.github.io/universal-agent-plugins/docs/en/');
  return url.replace(/\/+$/, '') + '/';
});
const useGuideUrl = computed(() =>
  replaceDocsLocale(`${docsRoot.value}use/`, currentLocale.value, 'use'),
);
const buildGuideUrl = computed(() =>
  replaceDocsLocale(`${docsRoot.value}build/`, currentLocale.value, 'build'),
);
const historicalGuideUrl = computed(() =>
  replaceDocsLocale(`${docsRoot.value}legacy/v1/`, currentLocale.value, 'legacy/v1'),
);

const journeys = computed(() => [
  {
    id: 'use',
    badge: t('pluginAuthoring.use.badge'),
    title: t('pluginAuthoring.use.title'),
    description: t('pluginAuthoring.use.description'),
    cta: t('pluginAuthoring.use.cta'),
    href: useGuideUrl.value,
  },
  {
    id: 'build',
    badge: t('pluginAuthoring.build.badge'),
    title: t('pluginAuthoring.build.title'),
    description: t('pluginAuthoring.build.description'),
    cta: t('pluginAuthoring.build.cta'),
    href: buildGuideUrl.value,
  },
]);
</script>

<template>
  <section id="authoring-journeys" class="authoring-journeys-section section">
    <v-container>
      <div class="authoring-journeys-section__grid">
        <article
          v-for="journey in journeys"
          :key="journey.id"
          class="authoring-journeys-section__card"
          :class="`authoring-journeys-section__card--${journey.id}`"
        >
          <span class="authoring-journeys-section__badge">{{ journey.badge }}</span>
          <h2 class="authoring-journeys-section__card-title">{{ journey.title }}</h2>
          <p class="authoring-journeys-section__card-copy">{{ journey.description }}</p>
          <a
            class="authoring-journeys-section__link"
            :href="journey.href"
            target="_blank"
            rel="noopener noreferrer"
          >
            {{ journey.cta }}
          </a>
        </article>
      </div>

      <p class="authoring-journeys-section__historical">
        {{ t('pluginAuthoring.historical.eyebrow') }}
        <a :href="historicalGuideUrl" target="_blank" rel="noopener noreferrer">
          {{ t('pluginAuthoring.historical.cta') }}
        </a>
      </p>
    </v-container>
  </section>
</template>

<style scoped>
.authoring-journeys-section__grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 20px;
  max-width: 980px;
  margin: 0 auto;
}

.authoring-journeys-section__card {
  border-radius: 24px;
  border: 1px solid rgba(114, 87, 255, 0.16);
  background:
    radial-gradient(circle at top right, rgba(114, 87, 255, 0.08) 0%, transparent 38%),
    rgba(10, 10, 15, 0.82);
  box-shadow: 0 20px 70px rgba(0, 0, 0, 0.24);
  backdrop-filter: blur(14px);
  padding: 28px;
  display: flex;
  flex-direction: column;
}

.authoring-journeys-section__card--use {
  border-color: rgba(0, 240, 255, 0.16);
}

.authoring-journeys-section__badge {
  align-self: flex-start;
  border-radius: 999px;
  padding: 5px 10px;
  margin-bottom: 14px;
  font-size: 0.66rem;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  font-weight: 700;
  background: rgba(114, 87, 255, 0.1);
  color: #7257ff;
  border: 1px solid rgba(114, 87, 255, 0.18);
}

.authoring-journeys-section__card--use .authoring-journeys-section__badge {
  background: rgba(0, 240, 255, 0.1);
  color: #00f0ff;
  border-color: rgba(0, 240, 255, 0.18);
}

.authoring-journeys-section__card-title {
  margin: 0 0 12px;
  color: #eff6ff;
  font-size: 1.5rem;
  line-height: 1.2;
}

.authoring-journeys-section__card-copy {
  margin: 0;
  color: #91a0bf;
  line-height: 1.68;
  font-size: 0.96rem;
  flex-grow: 1;
}

.authoring-journeys-section__link {
  display: inline-flex;
  margin-top: 20px;
  color: #eff6ff;
  font-size: 0.82rem;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  text-decoration: none;
}

.authoring-journeys-section__link:hover {
  text-decoration: underline;
}

.authoring-journeys-section__historical {
  margin: 32px auto 0;
  max-width: 980px;
  text-align: center;
  color: #91a0bf;
  font-size: 0.9rem;
}

.authoring-journeys-section__historical a {
  color: #ffd166;
  margin-left: 6px;
  text-decoration: none;
}

.authoring-journeys-section__historical a:hover {
  text-decoration: underline;
}

@media (max-width: 800px) {
  .authoring-journeys-section__grid {
    grid-template-columns: 1fr;
  }
}
</style>
