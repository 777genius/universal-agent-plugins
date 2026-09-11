import availability from '../data/docsLocaleAvailability.generated.json' with { type: 'json' };

export type DocsLocale = 'en' | 'ru' | 'es' | 'fr' | 'zh';

const docsLocalePattern = /\/(en|ru|es|fr|zh)(?=\/|$)/;
const realPaths = availability.realPaths as Record<string, string[]>;

/** `docPath` is the locale-relative page path with no extension, e.g. "guide/quickstart"; "" means the locale root. */
export const hasRealDocsContent = (locale: DocsLocale, docPath: string): boolean =>
  locale === 'en' || (realPaths[locale] ?? []).includes(docPath);

export const resolveDocsLocale = (locale: DocsLocale, docPath: string): DocsLocale =>
  hasRealDocsContent(locale, docPath) ? locale : 'en';

export const replaceDocsLocale = (url: string, locale: DocsLocale, docPath: string): string => {
  if (!url || !docsLocalePattern.test(url)) {
    return url;
  }

  return url.replace(docsLocalePattern, `/${resolveDocsLocale(locale, docPath)}`);
};
