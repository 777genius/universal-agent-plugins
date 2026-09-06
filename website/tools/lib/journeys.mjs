export const docsLocales = ["en", "ru", "es", "fr", "zh"];
export const localePathField = (locale) => `path${locale[0].toUpperCase()}${locale.slice(1)}`;
export const entityPath = (entry, locale) => entry[localePathField(locale)] || entry.pathEn || "";
export const isPreparedSource = (relative) => /^(use|build|legacy\/v1)\//.test(relative);

export function journeyNav(locale) {
  const suffix = locale === "en" ? "" : " (English preview)";
  return [
    { text: `Use plugins${suffix}`, link: "/en/use/" },
    { text: `Build plugins${suffix}`, link: "/en/build/" }
  ];
}

export function journeySidebar(locale, entities) {
  const groups = [
    ["Use plugins", "use", ["index", "install", "manage"]],
    ["Build plugins", "build", ["index", "skill", "mcp-remote", "mcp-stdio", "hybrid", "skills", "layout", "checks", "handoff"]],
    ["Historical v1 · 1.2.4", "legacy:v1", ["index"]]
  ].map(([text, section, pages]) => ({ text, items: pages.map((page) => {
    const id = `page:${section}:${page}`;
    const entity = entities.find((entry) => entry.canonicalId === id);
    if (!entity?.pathEn) throw new Error(`Missing journey source: ${id}`);
    return { text: entity.title + (locale === "en" || entity[localePathField(locale)] ? "" : " (English)"),
      link: entityPath(entity, locale) };
  }) }));
  groups.push(...["plugin-kit-ai", "agentplugins author"].map((surface) => ({
    text: `${surface} · prepared, not released`,
    items: entities.filter((entry) => entry.surface === "authoring-cli" &&
      (entry.title === surface || entry.title.startsWith(`${surface} `)))
      .map((entry) => ({ text: entry.title, link: entityPath(entry, locale) }))
  })));
  return groups;
}

// A path enters the registry only after its page has been produced. Never infer
// ES/FR/ZH existence from EN, including for generated English-only references.
export function bindGeneratedPaths(entities, pages) {
  const available = new Set(pages.map((page) => `/${page.relativePath.replace(/index\.md$/, "").replace(/\.md$/, "")}`));
  return entities.map((entity) => {
    const result = { ...entity };
    for (const locale of docsLocales) {
      const field = localePathField(locale);
      const candidate = result[field] || (entity.localeStrategy === "mirrored" && entity.pathEn
        ? entity.pathEn.replace(/^\/en\//, `/${locale}/`) : "");
      result[field] = available.has(candidate) ? candidate : "";
    }
    return result;
  });
}

export function requirePreparationPreview(env = process.env) {
  if (env.DOCS_PREPARATION_PREVIEW !== "1") {
    throw new Error("D2b is preparation only. D3 locale parity/fallback and D5 release activation remain required; use DOCS_PREPARATION_PREVIEW=1 only in a disposable, non-published preview.");
  }
}
