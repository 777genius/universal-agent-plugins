import { readFileSync } from 'node:fs';

type GlobalMessages = typeof import('../locales/en.json');
type CatalogMessages = typeof import('../features/skills-catalog/locales/en.json');
type SkillSpectorMessages = typeof import('../features/skillspector/locales/en.json');

/** Release tooling validates every owned feature without adding it to global app messages. */
export function loadI18nMessages(
  locale: string,
): GlobalMessages & CatalogMessages & SkillSpectorMessages {
  const read = <Messages>(folder: string): Messages =>
    JSON.parse(
      readFileSync(new URL(`../${folder}/${locale}.json`, import.meta.url), 'utf8'),
    ) as Messages;
  const base = read<GlobalMessages>('locales');
  const catalog = read<CatalogMessages>('features/skills-catalog/locales');
  const skillSpector = read<SkillSpectorMessages>('features/skillspector/locales');
  return {
    ...base,
    registryUi: {
      ...base.registryUi,
      ...catalog.registryUi,
      ...skillSpector.registryUi,
      directoryPage: { ...base.registryUi.directoryPage, ...catalog.registryUi.directoryPage },
    },
  };
}
