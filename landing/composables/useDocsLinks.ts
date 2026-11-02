import { computed } from 'vue';
import { docsAvailability, resolveDocsLink, type DocsPageId } from '~/data/docsAvailability';

export const useDocsLinks = () => {
  const { locale, t } = useI18n();
  const config = useRuntimeConfig();
  const link = (id: DocsPageId, configured?: string) =>
    computed(() => resolveDocsLink(id, locale.value, configured));
  const configuredDocsHome = String(config.public.docsUrl || '');
  const ownedHome = resolveDocsLink('home', 'en', configuredDocsHome);
  // Derive children only for recognized owned homes; external overrides retain
  // their existing destination and are never rewritten by a locale heuristic.
  const ownedPageUrl = (id: DocsPageId) =>
    ownedHome.language ? new URL(docsAvailability[id], ownedHome.url).href : undefined;
  const docs = link('home', configuredDocsHome);
  const quickstart = link(
    'quickstart',
    String(config.public.quickstartUrl || '') || ownedPageUrl('quickstart'),
  );
  const useGuide = link('use', ownedPageUrl('use'));
  const buildGuide = link('build', ownedPageUrl('build'));
  const supportBoundary = link('supportBoundary', ownedPageUrl('supportBoundary'));
  const customLogicGuide = link('customLogicGuide', ownedPageUrl('customLogicGuide'));
  // The maintained manage pages share a stable project-restore fragment.
  const restoreGuide = link(
    'restoreOnAnotherMachine',
    ownedPageUrl('restoreOnAnotherMachine') ||
      (configuredDocsHome
        ? `${configuredDocsHome}${docsAvailability.restoreOnAnotherMachine}`
        : undefined),
  );
  const docsLabel = (label: string, englishFallback = docs.value.englishFallback) =>
    englishFallback ? t('shell.docs.englishLabel', { label }) : label;

  return {
    docsUrl: computed(() => docs.value.url),
    quickstartUrl: computed(() => quickstart.value.url),
    useGuideUrl: computed(() => useGuide.value.url),
    buildGuideUrl: computed(() => buildGuide.value.url),
    supportBoundaryUrl: computed(() => supportBoundary.value.url),
    customLogicGuideUrl: computed(() => customLogicGuide.value.url),
    restoreGuideUrl: computed(() => `${restoreGuide.value.url}#restore-project-dependencies`),
    restoreGuideEnglishFallback: computed(() => restoreGuide.value.englishFallback),
    docsLabel,
    quickstartEnglishFallback: computed(() => quickstart.value.englishFallback),
    useGuideEnglishFallback: computed(() => useGuide.value.englishFallback),
    buildGuideEnglishFallback: computed(() => buildGuide.value.englishFallback),
    supportBoundaryEnglishFallback: computed(() => supportBoundary.value.englishFallback),
  };
};
