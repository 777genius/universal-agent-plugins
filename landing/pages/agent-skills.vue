<script setup lang="ts">
// This is the stable, indexable overview. Query-specific third-party Skill
// details still live at /skills/?id=... and remain noindex.
const { t } = useI18n({ useScope: 'local' });
const localePath = useLocalePath();
const { useGuideUrl } = useDocsLinks();

usePageSeo(
  () => t('registryUi.skills.overview.seoTitle'),
  () => t('registryUi.skills.overview.metaDescription'),
  { translate: false },
);
</script>

<template>
  <div class="registry-surface skills-overview">
    <PageBackground />
    <main class="container skills-overview__main">
      <header class="skills-overview__intro">
        <h1>{{ t('registryUi.skills.overview.title') }}</h1>
        <p>{{ t('registryUi.skills.overview.lead') }}</p>
        <div class="skills-overview__links">
          <NuxtLink class="skills-overview__primary" :to="localePath({ path: '/plugins/', query: { tab: 'skills' } })">
            {{ t('registryUi.skills.overview.browse') }}
          </NuxtLink>
          <a :href="useGuideUrl">{{ t('registryUi.skills.overview.installGuide') }}</a>
        </div>
      </header>
      <section class="skills-overview__grid" :aria-label="t('registryUi.skills.overview.title')">
        <article>
          <h2>{{ t('registryUi.skills.overview.standaloneTitle') }}</h2>
          <p>{{ t('registryUi.skills.overview.standaloneDescription') }}</p>
        </article>
        <article>
          <h2>{{ t('registryUi.skills.overview.scopeTitle') }}</h2>
          <p>{{ t('registryUi.skills.overview.scopeDescription') }}</p>
        </article>
        <article>
          <h2>{{ t('registryUi.skills.overview.reviewTitle') }}</h2>
          <p>{{ t('registryUi.skills.overview.reviewDescription') }}</p>
        </article>
      </section>
      <section class="skills-overview__example">
        <h2>{{ t('registryUi.skills.overview.exampleCaption') }}</h2>
        <pre dir="ltr"><code>agentplugins skills add owner/repository --ui --scope project</code></pre>
      </section>
    </main>
  </div>
</template>

<i18n locale="en" lang="json" src="../features/skills-catalog/locales/en.json" />
<i18n locale="ru" lang="json" src="../features/skills-catalog/locales/ru.json" />
<i18n locale="uk" lang="json" src="../features/skills-catalog/locales/uk.json" />
<i18n locale="zh" lang="json" src="../features/skills-catalog/locales/zh.json" />
<i18n locale="es" lang="json" src="../features/skills-catalog/locales/es.json" />
<i18n locale="hi" lang="json" src="../features/skills-catalog/locales/hi.json" />
<i18n locale="ar" lang="json" src="../features/skills-catalog/locales/ar.json" />
<i18n locale="pt" lang="json" src="../features/skills-catalog/locales/pt.json" />
<i18n locale="fr" lang="json" src="../features/skills-catalog/locales/fr.json" />

<style scoped>
.skills-overview { min-height: 75vh; }
.skills-overview__main { position: relative; z-index: 1; padding-top: 5rem; padding-bottom: 6rem; }
.skills-overview__intro { max-width: 800px; }
.skills-overview__intro h1 { font-size: clamp(2rem, 5vw, 3.6rem); line-height: 1.15; letter-spacing: -0.04em; }
.skills-overview__intro p { margin-top: 1.2rem; font-size: 1.14rem; color: var(--muted); }
.skills-overview__links { display: flex; flex-wrap: wrap; gap: 1rem; margin: 2rem 0 3rem; }
.skills-overview__links a { padding: .7rem 1.1rem; border: 1px solid var(--line-strong); border-radius: .75rem; text-decoration: none; font-weight: 650; }
.skills-overview__links a:hover, .skills-overview__links a:focus-visible { border-color: var(--cyan); }
.skills-overview__links .skills-overview__primary { color: var(--bg); background: var(--cyan); border-color: var(--cyan); }
.skills-overview__grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 1rem; }
.skills-overview__grid article, .skills-overview__example { padding: 1.5rem; border: 1px solid var(--line); background: var(--surface); border-radius: 1rem; }
.skills-overview__grid h2, .skills-overview__example h2 { font-size: 1.25rem; }
.skills-overview__grid p { margin-top: .75rem; color: var(--muted); }
.skills-overview__example { margin-top: 1.25rem; }
.skills-overview__example pre { overflow-x: auto; margin-top: .8rem; padding: 1rem; background: var(--surface-solid); border-radius: .6rem; }
@media (max-width: 800px) {
  .skills-overview__main { padding-top: 3rem; padding-bottom: 4rem; }
  .skills-overview__grid { grid-template-columns: 1fr; }
}
</style>
