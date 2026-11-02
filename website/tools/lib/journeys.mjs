import { docsLocales, localePathField } from "../config/locales.mjs";
export { docsLocales, localePathField };
export const entityPath = (entry, locale) => entry[localePathField(locale)] || entry.pathEn || "";
export const isCurrentJourney = (relative) => /^(use|build)\//.test(relative);

export const journeyLabels = {
  uk: ["Використовувати плагіни", "Створювати плагіни", "Англійською"],
  hi: ["प्लगइन इस्तेमाल करें", "प्लगइन बनाएँ", "अंग्रेज़ी में"],
  ar: ["استخدام الإضافات", "بناء الإضافات", "بالإنجليزية"],
  pt: ["Usar plugins", "Criar plugins", "Em inglês"],
  "en": [
    "Use plugins",
    "Build plugins",
    "English"
  ],
  "ru": [
    "Использовать плагины",
    "Создавать плагины",
    "На английском"
  ],
  "es": [
    "Usar plugins",
    "Crear plugins",
    "En inglés"
  ],
  "fr": [
    "Utiliser des plugins",
    "Créer des plugins",
    "En anglais"
  ],
  "zh": [
    "使用插件",
    "构建插件",
    "英语"
  ]
};

export function journeyNav(locale) {
  return ["use", "build"].map((section, index) => ({ text: journeyLabels[locale][index], link: `/${locale}/${section}/` }));
}

export const journeyPageLabels = {
  uk: {
      "page:use:index": "Огляд",
      "page:use:install": "Встановлення",
      "page:use:manage": "Керування",
      "page:build:index": "Огляд",
      "page:build:skill": "Один Skill",
      "page:build:mcp-remote": "Віддалений MCP",
      "page:build:mcp-stdio": "Локальний MCP",
      "page:build:hybrid": "Гібридний плагін",
      "page:build:skills": "Кілька Skills",
      "page:build:layout": "Структура",
      "page:build:checks": "Перевірки",
      "page:build:handoff": "Передавання"
  },
  hi: {
      "page:use:index": "अवलोकन",
      "page:use:install": "इंस्टॉल करें",
      "page:use:manage": "प्रबंधित करें",
      "page:build:index": "अवलोकन",
      "page:build:skill": "एक Skill",
      "page:build:mcp-remote": "रिमोट MCP",
      "page:build:mcp-stdio": "स्थानीय MCP",
      "page:build:hybrid": "हाइब्रिड प्लगइन",
      "page:build:skills": "कई Skills",
      "page:build:layout": "संरचना",
      "page:build:checks": "जाँच",
      "page:build:handoff": "हस्तांतरण"
  },
  ar: {
      "page:use:index": "نظرة عامة",
      "page:use:install": "التثبيت",
      "page:use:manage": "الإدارة",
      "page:build:index": "نظرة عامة",
      "page:build:skill": "Skill واحدة",
      "page:build:mcp-remote": "MCP بعيد",
      "page:build:mcp-stdio": "MCP محلي",
      "page:build:hybrid": "إضافة هجينة",
      "page:build:skills": "عدة Skills",
      "page:build:layout": "البنية",
      "page:build:checks": "الفحوصات",
      "page:build:handoff": "التسليم"
  },
  pt: {
      "page:use:index": "Visão geral",
      "page:use:install": "Instalar",
      "page:use:manage": "Gerenciar",
      "page:build:index": "Visão geral",
      "page:build:skill": "Um Skill",
      "page:build:mcp-remote": "MCP remoto",
      "page:build:mcp-stdio": "MCP local",
      "page:build:hybrid": "Plugin híbrido",
      "page:build:skills": "Vários Skills",
      "page:build:layout": "Estrutura",
      "page:build:checks": "Verificações",
      "page:build:handoff": "Entrega"
  },
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
    "page:build:handoff": "Передача"
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
    "page:build:handoff": "Entrega"
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
    "page:build:handoff": "Transmission"
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
    "page:build:handoff": "交接"
  }
};

const referenceLabels = {
  uk: "довідник 0.1.65", hi: "0.1.65 संदर्भ", ar: "مرجع 0.1.65", pt: "referência 0.1.65",
  en: "0.1.65 reference", ru: "справочник 0.1.65", es: "referencia 0.1.65",
  fr: "référence 0.1.65", zh: "0.1.65 版本参考"
};

export function journeySidebar(locale, entities) {
  const groups = [
    [journeyLabels[locale][0], "use", ["index", "install", "manage"]],
    [journeyLabels[locale][1], "build", ["index", "skill", "mcp-remote", "mcp-stdio", "hybrid", "skills", "layout", "checks", "handoff"]]
  ].map(([text, section, pages]) => ({ text, items: pages.map((page) => {
    const id = `page:${section}:${page}`;
    const entity = entities.find((entry) => entry.canonicalId === id);
    if (!entity?.pathEn) throw new Error(`Missing journey source: ${id}`);
    return { text: (journeyPageLabels[locale]?.[id] || entity.title) + (locale === "en" || entity[localePathField(locale)] ? "" : ` (${journeyLabels[locale][2]})`),
      link: entityPath(entity, locale) };
  }) }));
  groups.push(...["agentplugins author"].map((surface) => ({
    text: `${surface} · ${referenceLabels[locale]}`,
    collapsed: true,
    items: entities.filter((entry) => entry.surface === "authoring-cli" &&
      (entry.title === surface || entry.title.startsWith(`${surface} `)))
      .map((entry) => ({ text: entry.title + (locale === "en" || entry[localePathField(locale)] ? "" : ` (${journeyLabels[locale][2]})`), link: entityPath(entry, locale) }))
  })));
  return groups;
}

// A path enters the registry only after its page has been produced. Never infer
// translated existence from EN, including for generated English-only references.
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

export function requirePublicationBoundary(entities, env = process.env) {
  if (env.DOCS_PREPARATION_PREVIEW === "1") return "disposable-preview";
  if (!Array.isArray(entities) || entities.length === 0) {
    throw new Error("Public documentation requires a non-empty entity registry.");
  }
  const stale = entities.filter((entry) => entry.publicVisibility === "preparation" ||
    entry.status === "prepared-not-release" || entry.released === false ||
    entry.stability === "prepared-not-release" || entry.maturity === "prepared");
  if (stale.length) {
    throw new Error(`Public documentation retains unreleased preparation metadata: ${stale[0].canonicalId || "unknown"}`);
  }
  const sources = entities.filter((entry) => entry.sourceKind === "hand-authored");
  for (const entry of sources) {
    if (isCurrentJourney(entry.sourceRef || "") && (entry.status !== "released" || entry.released !== true || entry.publicVisibility !== "public" ||
        entry.stability !== "public-stable" || entry.maturity !== "stable")) {
      throw new Error(`Public documentation misclassifies a released journey: ${entry.canonicalId || "unknown"}`);
    }
    if (docsLocales.some((locale) => !entry[localePathField(locale)])) {
      throw new Error(`Public documentation lacks a maintained-locale release route: ${entry.canonicalId || "unknown"}`);
    }
  }
  return "released-public";
}
