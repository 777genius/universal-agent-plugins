/** Legacy content remains available to its existing consumers, independently of publication. */
export type LegacyContentLocale = 'en' | 'ru' | 'es' | 'fr' | 'zh';
export type LocaleCode = LegacyContentLocale;
export type KnownLocale = LegacyContentLocale | 'uk' | 'hi' | 'ar' | 'pt';
export const candidateLocales = ['en', 'ru', 'uk', 'zh', 'es', 'hi', 'ar', 'pt', 'fr'] as const;
export const publishedLocales = candidateLocales;
export type PublishedLocale = (typeof publishedLocales)[number];
export const defaultLocale = 'en' as const;
export const localeMetadata = {
  en: { code: 'en', language: 'en-US', name: 'English', file: 'en.json' },
  ru: { code: 'ru', language: 'ru-RU', name: 'Русский', file: 'ru.json' },
  uk: { code: 'uk', language: 'uk-UA', name: 'Українська', file: 'uk.json' },
  es: { code: 'es', language: 'es-ES', name: 'Español', file: 'es.json' },
  fr: { code: 'fr', language: 'fr-FR', name: 'Français', file: 'fr.json' },
  zh: { code: 'zh', language: 'zh-CN', name: '简体中文', file: 'zh.json' },
  hi: { code: 'hi', language: 'hi-IN', name: 'हिन्दी', file: 'hi.json' },
  ar: { code: 'ar', language: 'ar', name: 'العربية', file: 'ar.json', dir: 'rtl' },
  pt: { code: 'pt', language: 'pt-BR', name: 'Português (Brasil)', file: 'pt.json' },
} as const;
export const supportedLocales = publishedLocales.map((code) => localeMetadata[code]);
export const isKnownLocale = (value: unknown): value is KnownLocale =>
  typeof value === 'string' && Object.hasOwn(localeMetadata, value);
export const isPublishedLocale = (value: unknown): value is PublishedLocale =>
  typeof value === 'string' && (publishedLocales as readonly string[]).includes(value);
