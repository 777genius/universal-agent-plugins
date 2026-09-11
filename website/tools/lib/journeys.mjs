export const docsLocales = ["en", "ru", "es", "fr", "zh"];
export const localePathField = (locale) => `path${locale[0].toUpperCase()}${locale.slice(1)}`;
export const entityPath = (entry, locale) => entry[localePathField(locale)] || entry.pathEn || "";
export const isPreparedSource = (relative) => /^(use|build|legacy\/v1)\//.test(relative);

export const journeyLabels = {
  "en": [
    "Use plugins",
    "Build plugins",
    "Historical v1 · baseline 1.2.4",
    "English"
  ],
  "ru": [
    "Использовать плагины",
    "Создавать плагины",
    "История v1 · базовая версия 1.2.4",
    "На английском"
  ],
  "es": [
    "Usar plugins",
    "Crear plugins",
    "Histórico v1 · referencia 1.2.4",
    "En inglés"
  ],
  "fr": [
    "Utiliser des plugins",
    "Créer des plugins",
    "Historique v1 · référence 1.2.4",
    "En anglais"
  ],
  "zh": [
    "使用插件",
    "构建插件",
    "历史 v1 · 基准 1.2.4",
    "英语"
  ]
};

export function journeyNav(locale) {
  return ["use", "build"].map((section, index) => ({ text: journeyLabels[locale][index], link: `/${locale}/${section}/` }));
}

export const journeyPageLabels = {
  "ru": {
    "page:use:index": "Обзор",
    "page:use:install": "Установка",
    "page:use:manage": "Управление",
    "page:build:index": "Обзор",
    "page:build:skill": "Один Skill",
    "page:build:mcp-remote": "Удалённый MCP",
    "page:build:mcp-stdio": "Локальный MCP",
    "page:build:hybrid": "Гибридный плагин",
    "page:build:skills": "Несколько Skills",
    "page:build:layout": "Структура",
    "page:build:checks": "Проверки",
    "page:build:handoff": "Передача",
    "page:legacy:v1:index": "Исторический контекст"
  },
  "es": {
    "page:use:index": "Resumen",
    "page:use:install": "Instalar",
    "page:use:manage": "Gestionar",
    "page:build:index": "Resumen",
    "page:build:skill": "Un Skill",
    "page:build:mcp-remote": "MCP remoto",
    "page:build:mcp-stdio": "MCP local",
    "page:build:hybrid": "Plugin híbrido",
    "page:build:skills": "Varios Skills",
    "page:build:layout": "Estructura",
    "page:build:checks": "Comprobaciones",
    "page:build:handoff": "Entrega",
    "page:legacy:v1:index": "Contexto histórico"
  },
  "fr": {
    "page:use:index": "Présentation",
    "page:use:install": "Installer",
    "page:use:manage": "Gérer",
    "page:build:index": "Présentation",
    "page:build:skill": "Un Skill",
    "page:build:mcp-remote": "MCP distant",
    "page:build:mcp-stdio": "MCP local",
    "page:build:hybrid": "Plugin hybride",
    "page:build:skills": "Plusieurs Skills",
    "page:build:layout": "Structure",
    "page:build:checks": "Vérifications",
    "page:build:handoff": "Transmission",
    "page:legacy:v1:index": "Contexte historique"
  },
  "zh": {
    "page:use:index": "概览",
    "page:use:install": "安装",
    "page:use:manage": "管理",
    "page:build:index": "概览",
    "page:build:skill": "单个 Skill",
    "page:build:mcp-remote": "远程 MCP",
    "page:build:mcp-stdio": "本地 MCP",
    "page:build:hybrid": "混合插件",
    "page:build:skills": "多个 Skills",
    "page:build:layout": "结构",
    "page:build:checks": "检查",
    "page:build:handoff": "交接",
    "page:legacy:v1:index": "历史背景"
  }
};

const preparationLabels = {
  en: "prepared, not released", ru: "подготовка, не релиз", es: "preparación, no publicado",
  fr: "préparation, non publié", zh: "准备中，尚未发布"
};

export function journeySidebar(locale, entities) {
  const groups = [
    [journeyLabels[locale][0], "use", ["index", "install", "manage"]],
    [journeyLabels[locale][1], "build", ["index", "skill", "mcp-remote", "mcp-stdio", "hybrid", "skills", "layout", "checks", "handoff"]],
    [journeyLabels[locale][2], "legacy:v1", ["index"]]
  ].map(([text, section, pages]) => ({ text, items: pages.map((page) => {
    const id = `page:${section}:${page}`;
    const entity = entities.find((entry) => entry.canonicalId === id);
    if (!entity?.pathEn) throw new Error(`Missing journey source: ${id}`);
    return { text: (journeyPageLabels[locale]?.[id] || entity.title) + (locale === "en" || entity[localePathField(locale)] ? "" : ` (${journeyLabels[locale][3]})`),
      link: entityPath(entity, locale) };
  }) }));
  groups.push(...["plugin-kit-ai", "agentplugins author"].map((surface) => ({
    text: `${surface} · ${preparationLabels[locale]}`,
    items: entities.filter((entry) => entry.surface === "authoring-cli" &&
      (entry.title === surface || entry.title.startsWith(`${surface} `)))
      .map((entry) => ({ text: entry.title + (locale === "en" || entry[localePathField(locale)] ? "" : ` (${journeyLabels[locale][3]})`), link: entityPath(entry, locale) }))
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
