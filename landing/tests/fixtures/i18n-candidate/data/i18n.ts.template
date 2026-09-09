// Candidate metadata only: copied production source remains EN-only.
import { candidateLocales, localeMetadata } from './production-i18n';
export * from './production-i18n';
export const publishedLocales = candidateLocales;
export type PublishedLocale = typeof publishedLocales[number];
export const supportedLocales = publishedLocales.map(code => localeMetadata[code]);
export const isPublishedLocale = (value: unknown): value is PublishedLocale =>
  typeof value === 'string' && (publishedLocales as readonly string[]).includes(value);
