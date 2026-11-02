// Registry paths are bound by D2 only after a page has been produced.
import { docsLocales } from "../../../tools/config/locales.mjs";
export const localeCodes = docsLocales;
export function localeFromPath(path, base = "/") {
  const prefix = base.replace(/\/+$/, "");
  const pathname = path.split(/[?#]/)[0];
  const local = prefix && pathname.startsWith(`${prefix}/`) ? pathname.slice(prefix.length) : pathname;
  return localeCodes.find((code) => local === `/${code}` || local.startsWith(`/${code}/`)) || null;
}
export function localeDestination(entity, code, hash = "") {
  const field = `path${code[0].toUpperCase()}${code.slice(1)}`;
  const target = entity?.[field]
    ? { path: entity[field], language: code, fallback: false, home: false }
    : entity?.pathEn
      ? { path: entity.pathEn, language: "en", fallback: true, home: false }
      // Unknown pages must not manufacture a translated suffix (or fragment).
      : { path: `/${code}/`, language: code, fallback: false, home: true };
  if (hash && !target.home) {
    try {
      const anchor = decodeURIComponent(hash.replace(/^#/, ""));
      if (entity?.anchorsByLocale?.[target.language]?.includes(anchor)) target.path += `#${encodeURIComponent(anchor)}`;
    } catch { /* Invalid fragments never become navigation targets. */ }
  }
  return target;
}
export function preferredLocale(saved, candidates) {
  if (localeCodes.includes(String(saved).toLowerCase())) return String(saved).toLowerCase();
  for (const candidate of candidates) {
    const code = String(candidate).toLowerCase().split(/[-_]/)[0];
    if (localeCodes.includes(code)) return code;
  }
  return "en";
}
