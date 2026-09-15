import { computed } from 'vue';
import { resolveDocsLink, type DocsPageId } from '~/data/docsAvailability';

export const useDocsLinks = () => {
  const { locale, t } = useI18n();
  const config = useRuntimeConfig();
  const link = (id: DocsPageId, configured?: string) =>
    computed(() => resolveDocsLink(id, locale.value, configured));
  const docs = link('home', String(config.public.docsUrl || ''));
  const quickstart = link('quickstart', String(config.public.quickstartUrl || ''));
  const buildGuide = link('build');
  const supportBoundary = link('supportBoundary');
  const docsLabel = (label: string, englishFallback = docs.value.englishFallback) =>
    englishFallback ? t('shell.docs.englishLabel', { label }) : label;

  return {
    docsUrl: computed(() => docs.value.url),
    quickstartUrl: computed(() => quickstart.value.url),
    buildGuideUrl: computed(() => buildGuide.value.url),
    supportBoundaryUrl: computed(() => supportBoundary.value.url),
    docsLabel,
    quickstartEnglishFallback: computed(() => quickstart.value.englishFallback),
    supportBoundaryEnglishFallback: computed(() => supportBoundary.value.englishFallback),
  };
};
