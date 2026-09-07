import { computed } from 'vue';
import type { LocaleCode } from '~/data/i18n';
import { replaceDocsLocale } from '~/utils/docsLinks';

export const useDocsLinks = () => {
  const { locale } = useI18n();
  const config = useRuntimeConfig();

  const currentLocale = computed<LocaleCode>(() => {
    const supported = new Set<LocaleCode>(['en', 'ru', 'es', 'fr', 'zh']);
    return supported.has(locale.value as LocaleCode) ? (locale.value as LocaleCode) : 'en';
  });

  const docsUrl = computed(() =>
    replaceDocsLocale(
      config.public.docsUrl || 'https://777genius.github.io/universal-agent-plugins/docs/en/',
      currentLocale.value,
      '',
    ),
  );

  const quickstartUrl = computed(() =>
    replaceDocsLocale(
      config.public.quickstartUrl ||
        'https://777genius.github.io/universal-agent-plugins/docs/en/guide/quickstart.html',
      currentLocale.value,
      'guide/quickstart',
    ),
  );

  const supportBoundaryUrl = computed(() =>
    replaceDocsLocale(
      'https://777genius.github.io/universal-agent-plugins/docs/en/reference/support-boundary.html',
      currentLocale.value,
      'reference/support-boundary',
    ),
  );

  const customLogicGuideUrl = computed(() =>
    replaceDocsLocale(
      'https://777genius.github.io/universal-agent-plugins/docs/en/guide/build-custom-plugin-logic.html',
      currentLocale.value,
      'guide/build-custom-plugin-logic',
    ),
  );

  return { docsUrl, quickstartUrl, supportBoundaryUrl, customLogicGuideUrl };
};
