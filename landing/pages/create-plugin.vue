<script setup lang="ts">
const { t } = useI18n();
const { quickstartUrl, docsLabel, quickstartEnglishFallback } = useDocsLinks();
// This checkpoint preserves the existing route and indexing policy.
// Availability is stated in visible copy, independently of robots metadata.
usePageSeo('publicAuthoring.title', 'publicAuthoring.intro', {
  robots: 'noindex, follow',
});
</script>

<template>
  <div class="authoring-page">
    <PageBackground />
    <div class="authoring-content">
      <header>
        <h1>{{ t('publicAuthoring.title') }}</h1>
        <p>{{ t('publicAuthoring.intro') }}</p>
      </header>

      <section id="use-plugins" aria-labelledby="use-title">
        <h2 id="use-title">{{ t('publicAuthoring.useTitle') }}</h2>
        <p>{{ t('publicAuthoring.available') }}</p>
        <pre><code>npx universal-agent-plugins add context7</code></pre>
        <p>{{ t('publicAuthoring.installHelp') }}</p>
        <a href="https://github.com/777genius/universal-agent-plugins#quick-start">
          {{ t('publicAuthoring.installLink') }}
        </a>
        <p>{{ t('publicAuthoring.versions') }}</p>
      </section>

      <section id="build-plugins" aria-labelledby="build-title">
        <h2 id="build-title">{{ t('publicAuthoring.buildTitle') }}</h2>
        <p>{{ t('publicAuthoring.standard') }}</p>
        <p>
          <strong>{{ t('publicAuthoring.preparation') }}</strong>
        </p>
        <p>{{ t('publicAuthoring.unreleased') }}</p>
        <a href="https://agent-plugins.org/specification">
          {{ t('publicAuthoring.specLink') }}
        </a>
        <p>{{ t('publicAuthoring.limitations') }}</p>
        <a href="https://github.com/777genius/universal-agent-plugins#supported-clients">
          {{ t('publicAuthoring.compatibilityLink') }}
        </a>
      </section>

      <section id="historical-v1" aria-labelledby="history-title">
        <h2 id="history-title">{{ t('publicAuthoring.historyTitle') }}</h2>
        <p>{{ t('publicAuthoring.history') }}</p>
        <a :href="quickstartUrl.split('#')[0] + '#historical-v1'">
          {{ docsLabel(t('publicAuthoring.quickstartLink'), quickstartEnglishFallback) }}
        </a>
      </section>
    </div>
  </div>
</template>

<style scoped>
.authoring-page {
  position: relative;
  padding: 8rem 1.5rem 4rem;
}

.authoring-content {
  position: relative;
  max-width: 56rem;
  margin: 0 auto;
  line-height: 1.7;
}

section {
  margin-top: 3rem;
  scroll-margin-top: 6rem;
}

h1,
h2 {
  line-height: 1.2;
  margin-bottom: 1rem;
}

p {
  margin: 1rem 0;
}

pre {
  overflow-x: auto;
  padding: 1rem;
  border: 1px solid currentColor;
  border-radius: 0.5rem;
}

a {
  text-decoration: underline;
  text-underline-offset: 0.2em;
}
</style>
