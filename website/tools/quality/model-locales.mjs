import { docsLocales, historicalLocales, localePathField } from "../config/locales.mjs";
import { isRetiredArchive } from "../lib/public-routes.mjs";

// Every current authored page is maintained in all public locales. Generated
// canonical-English references may additionally declare real translated pages.
export function requiredEntityLocales(entity) {
  return entity.sourceKind === "hand-authored" || entity.localeStrategy !== "canonical-en"
    ? docsLocales
    : docsLocales.filter((locale) => locale === "en" || entity[localePathField(locale)]);
}

export function generatedLocaleErrors(pages, entities) {
  const errors = [];
  const registry = new Map(entities.map((entity) => [entity.canonicalId, entity]));
  const coverage = new Map();
  for (const page of pages) {
    const { canonicalId, locale } = page;
    const relative = page.relativePath.replaceAll("\\", "/");
    const pathLocale = relative.split("/")[0];
    if (!canonicalId) {
      errors.push(`Generated file is missing canonicalId frontmatter: ${relative}`);
      continue;
    }
    if (!locale) {
      errors.push(`Generated file is missing locale frontmatter: ${relative}`);
      continue;
    }
    if (!docsLocales.includes(pathLocale) || locale !== pathLocale) {
      errors.push(`Generated file has unsupported or mismatched locale: ${relative} (${locale})`);
      continue;
    }
    const archived = isRetiredArchive(relative);
    const group = coverage.get(canonicalId) || { archived, locales: new Set() };
    if (group.archived !== archived) {
      errors.push(`Generated canonicalId spans current and archived routes: ${canonicalId}`);
    }
    if (group.locales.has(locale)) {
      errors.push(`Duplicate generated locale for ${canonicalId}: ${locale}`);
    }
    group.locales.add(locale);
    coverage.set(canonicalId, group);
  }

  for (const [canonicalId, { archived, locales }] of coverage) {
    const entity = registry.get(canonicalId);
    if (!archived && !entity) {
      errors.push(`Current generated page is missing from the public registry: ${canonicalId}`);
    }
    // Archives are intentionally absent from the public registry. Classify by
    // the same route boundary as assembly, never by absence from the registry.
    const required = archived ? historicalLocales : requiredEntityLocales(entity || {});
    if (required.some((locale) => !locales.has(locale)) || [...locales].some((locale) => !required.includes(locale))) {
      errors.push(`Generated locale parity failure for ${canonicalId}: expected [${[...required].sort().join(", ")}], have [${[...locales].sort().join(", ")}]`);
    }
  }
  return errors;
}
