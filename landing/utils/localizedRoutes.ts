import { defaultLocale, isKnownLocale, publishedLocales, type KnownLocale } from '../data/i18n.ts';
import type { ProductRoute } from '../data/routes.ts';

export function normalizeAppBase(base: string): string {
  return `/${base.split('/').filter(Boolean).join('/')}${base.split('/').filter(Boolean).length ? '/' : ''}`;
}
/** Accept app-relative or already based paths; apply the base exactly once. */
export function withAppBase(path: string, base: string): string {
  const normalized = normalizeAppBase(base);
  const absolute = `/${path.replace(/^\/+/, '')}`;
  if (normalized === '/' || absolute === normalized.slice(0, -1) || absolute.startsWith(normalized)) return absolute;
  return normalized + absolute.slice(1);
}
export function localizedPath(path: string, locale: KnownLocale): string {
  const clean = `/${path.split(/[?#]/, 1)[0]!.split('/').filter(Boolean).join('/')}`;
  const trailing = clean === '/' ? '/' : `${clean}/`;
  return locale === defaultLocale ? trailing : `/${locale}${trailing}`;
}
export function expandLocalizedRoutes(routes: readonly ProductRoute[], locales: readonly KnownLocale[] = publishedLocales) {
  if (new Set(locales).size !== locales.length || locales.some(code => !isKnownLocale(code))) throw new Error('Invalid locales');
  return routes.flatMap(route => locales.map(locale => ({ ...route, locale, path: localizedPath(route.path, locale) })));
}
export function isIndexablePath(path: string, routes: readonly ProductRoute[], locales: readonly KnownLocale[] = publishedLocales): boolean {
  const clean = path.split(/[?#]/, 1)[0]!;
  return expandLocalizedRoutes(routes, locales).some(route => route.indexable && route.path === (clean.endsWith('/') ? clean : `${clean}/`));
}
