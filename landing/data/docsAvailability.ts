/** Owned public docs IDs only. Landing publication does not publish a docs locale. */
export const docsAvailability = {
  home: '',
  quickstart: 'guide/quickstart.html',
  supportBoundary: 'reference/support-boundary.html',
  customLogicGuide: 'guide/build-custom-plugin-logic.html',
} as const;
export type DocsPageId = keyof typeof docsAvailability;
export const ownedDocsRoot = 'https://777genius.github.io/universal-agent-plugins/docs/';
const docsLanguages = ['en', 'ru', 'es', 'fr', 'zh'] as const;

export function resolveDocsLink(id: DocsPageId, locale: string, configuredUrl?: string) {
  const language = locale === 'ru' ? 'ru' : 'en';
  const defaultUrl = `${ownedDocsRoot}en/${docsAvailability[id]}`;
  const source = configuredUrl || defaultUrl;
  // Preserve unrecognized/custom destinations byte-for-byte, including external /en/ paths.
  for (const existing of docsLanguages) {
    const owned = `${ownedDocsRoot}${existing}/${docsAvailability[id]}`;
    const suffix = source.slice(owned.length);
    if (source === owned || (source.startsWith(owned) && /^[?#]/.test(suffix))) {
      return {
        url: `${ownedDocsRoot}${language}/${docsAvailability[id]}${suffix}`,
        language,
        englishFallback: locale === 'uk',
      };
    }
  }
  return { url: source, language: null, englishFallback: false };
}
