/** Owned public docs IDs only. Landing publication does not publish a docs locale. */
export const docsAvailability = {
  home: '',
  quickstart: 'guide/quickstart.html',
  use: 'use/',
  build: 'build/',
  // The former standalone support-boundary page is preserved historically and
  // is no longer a published route. The maintained build journey carries the
  // current support limits.
  supportBoundary: 'build/',
  customLogicGuide: 'build/checks.html',
  restoreOnAnotherMachine: 'use/manage.html',
} as const;
export type DocsPageId = keyof typeof docsAvailability;
export const ownedDocsRoot = 'https://777genius.github.io/universal-agent-plugins/docs/';
const ownedDocsRoots = [
  ownedDocsRoot,
  'https://777genius.github.io/docs/',
  'https://universal-agent-plugins.com/docs/',
];
const docsLanguages = ['en', 'ru', 'uk', 'zh', 'es', 'hi', 'ar', 'pt', 'fr'] as const;

export function resolveDocsLink(id: DocsPageId, locale: string, configuredUrl?: string) {
  const language = (docsLanguages as readonly string[]).includes(locale) ? locale : 'en';
  const defaultUrl = `${ownedDocsRoot}en/${docsAvailability[id]}`;
  const source = configuredUrl || defaultUrl;
  // Preserve unrecognized/custom destinations byte-for-byte, including external /en/ paths.
  for (const root of ownedDocsRoots) {
    for (const existing of docsLanguages) {
      const owned = `${root}${existing}/${docsAvailability[id]}`;
      const suffix = source.slice(owned.length);
      if (source === owned || (source.startsWith(owned) && /^[?#]/.test(suffix))) {
        return {
          url: `${root}${language}/${docsAvailability[id]}${suffix}`,
          language,
          englishFallback: locale !== language,
        };
      }
    }
  }
  return { url: source, language: null, englishFallback: false };
}
