import { requireAuthoringSource } from "./lib/source-contract.mjs";
import { pathToFileURL } from "node:url";
import { requireGenerationPrerequisites } from "./lib/preflight.mjs";
import fs from "node:fs/promises";
import { buildRedirects } from "./lib/redirects.mjs";
import { bindGeneratedPaths, isCurrentJourney, journeyLabels, journeySidebar, requirePublicationBoundary } from "./lib/journeys.mjs";
import { isRetiredArchive } from "./lib/public-routes.mjs";
import path from "node:path";
import { extractCLI } from "./extractors/cli.mjs";
import { extractGoSDK } from "./extractors/go-sdk.mjs";
import { extractNodeRuntime } from "./extractors/node-runtime.mjs";
import { extractPythonRuntime } from "./extractors/python-runtime.mjs";
import { extractPlatformData } from "./extractors/platform.mjs";
import { generatedRoot, generatedRegistryPaths, runtimeRoot, sourceRoot, websiteRoot } from "./config/site.mjs";
import { copyTree, ensureDir, listMarkdownFiles, rimraf, writeFile, writeJson } from "./lib/fs.mjs";
import { readFrontmatter } from "./lib/site-model.mjs";

const docsLocales = ["en", "ru", "es", "fr", "zh"];
const sourceLocales = docsLocales;
const mirroredGeneratedLocales = ["es", "fr", "zh"];

export async function generate() {
  await requireAuthoringSource();
  await requireGenerationPrerequisites();
  const bundles = await Promise.all([
    extractCLI(),
    extractGoSDK(),
    extractNodeRuntime(),
    extractPythonRuntime(),
    extractPlatformData()
  ]);

  await assembleBundles(bundles);
}

// Shared production assembly seam: integration writes the real generated tree and runtime.
export async function assembleBundles(bundles) {
  const rawGeneratedEntities = bundles.flatMap((bundle) => bundle.entities)
    .filter((entry) => !entry.pathEn || !isRetiredArchive(entry.pathEn));
  const baseGeneratedPages = bundles.flatMap((bundle) => bundle.pages);
  const generatedPages = [
    ...baseGeneratedPages,
    ...baseGeneratedPages
      .filter((page) => page.locale === "en" && page.mirror !== false)
      .flatMap((page) => mirroredGeneratedLocales.map((locale) => mirrorGeneratedPage(page, locale)))
  ];

  const generatedEntities = bindGeneratedPaths(rawGeneratedEntities, generatedPages);

  for (const locale of docsLocales) {
    await rimraf(path.join(generatedRoot, locale));
  }
  await ensureDir(path.join(generatedRoot, "registries"));

  for (const page of generatedPages) {
    await writeFile(path.join(generatedRoot, page.relativePath), page.content);
  }

  const sourceEntities = await scanSourceEntities();
  const allEntities = [...sourceEntities, ...generatedEntities].sort((a, b) =>
    a.canonicalId.localeCompare(b.canonicalId)
  );
  requirePublicationBoundary(allEntities);
  await writeJson(generatedRegistryPaths.entities, allEntities);
  await writeJson(generatedRegistryPaths.sidebarsEn, buildSidebar("en", allEntities));
  await writeJson(generatedRegistryPaths.sidebarsRu, buildSidebar("ru", allEntities));
  await writeJson(path.join(generatedRoot, "registries", "sidebars.es.json"), buildSidebar("es", allEntities));
  await writeJson(path.join(generatedRoot, "registries", "sidebars.fr.json"), buildSidebar("fr", allEntities));
  await writeJson(path.join(generatedRoot, "registries", "sidebars.zh.json"), buildSidebar("zh", allEntities));
  const inventory = JSON.parse(await fs.readFile(new URL("./config/routes.json", import.meta.url), "utf8"));
  const sourceFiles = await listMarkdownFiles(sourceRoot);
  const routes = [...sourceFiles.map((file) => path.relative(sourceRoot, file)), ...generatedPages.map((page) => page.relativePath)]
    .filter((file) => !file.startsWith("gateway/"))
    .filter((file) => !isRetiredArchive(file))
    .map((file) => `/${file.replace(/index\.md$/, "").replace(/\.md$/, "")}`);
  await writeJson(generatedRegistryPaths.redirects, buildRedirects(inventory, routes));

  await rimraf(runtimeRoot);
  await ensureDir(runtimeRoot);
  await copyTree(path.join(websiteRoot, "public"), path.join(runtimeRoot, "public"));
  await copyTree(path.join(sourceRoot, "gateway"), runtimeRoot);
  for (const locale of docsLocales) {
    await copyCurrentPages(path.join(sourceRoot, locale), path.join(runtimeRoot, locale), locale);
    await copyCurrentPages(path.join(generatedRoot, locale), path.join(runtimeRoot, locale), locale);
  }

}

if (process.argv[1] && import.meta.url === pathToFileURL(path.resolve(process.argv[1])).href) await generate();

export async function scanSourceEntities() {
  const entities = [];
  for (const locale of sourceLocales) {
    const files = await listMarkdownFiles(path.join(sourceRoot, locale));
    for (const filePath of files) {
      const meta = await readFrontmatter(filePath);
      if (!meta.canonicalId) {
        continue;
      }
      const relative = path.relative(path.join(sourceRoot, locale), filePath).replace(/\\/g, "/");
      if (isRetiredArchive(path.join(locale, relative))) continue;
      const existing = entities.find((entry) => entry.canonicalId === meta.canonicalId);
      const targetPath = `/${locale}/${relative.replace(/index\.md$/, "").replace(/\.md$/, "")}`;
      const localeKey = localePathField(locale);
      if (existing) {
        existing[localeKey] = targetPath;
        continue;
      }
      const currentJourney = isCurrentJourney(relative);
      entities.push({
        canonicalId: meta.canonicalId,
        kind: "page",
        surface: meta.section || "page",
        localeStrategy: currentJourney ? "canonical-en" : "mirrored",
        title: meta.title || relative,
        summary: meta.description || "",
        stability: currentJourney ? "public-stable" : meta.stability || "public-stable",
        maturity: currentJourney ? "stable" : meta.maturity || "stable",
        publicVisibility: "public",
        ...(currentJourney ? { status: "released", released: true } : {}),
        sourceKind: "hand-authored",
        sourceRef: relative,
        pathEn: locale === "en" ? targetPath : "",
        pathRu: locale === "ru" ? targetPath : "",
        pathEs: locale === "es" ? targetPath : "",
        pathFr: locale === "fr" ? targetPath : "",
        pathZh: locale === "zh" ? targetPath : "",
        relatedIds: [],
        searchTerms: [meta.title || relative]
      });
    }
  }
  return entities;
}

async function copyCurrentPages(fromRoot, toRoot, locale) {
  for (const file of await listMarkdownFiles(fromRoot)) {
    const relative = path.relative(fromRoot, file);
    if (isRetiredArchive(path.join(locale, relative))) continue;
    await writeFile(path.join(toRoot, relative), await fs.readFile(file, "utf8"));
  }
}

function localePathField(locale) {
  if (locale === "en") {
    return "pathEn";
  }
  if (locale === "ru") {
    return "pathRu";
  }
  if (locale === "es") {
    return "pathEs";
  }
  if (locale === "fr") {
    return "pathFr";
  }
  return "pathZh";
}

export function buildSidebar(locale, entities) {
  const prefix = `/${locale}/`;
  const labels = localeLabels(locale);
  const linkItem = (text, link) => ({ text, link });
  const guideSidebar = [{
    text: labels.guideStart,
    items: [
      linkItem(journeyLabels[locale][0], `${prefix}use/`),
      linkItem(journeyLabels[locale][1], `${prefix}build/`),
      linkItem(labels.quickstart, `${prefix}guide/quickstart`)
    ]
  }];

  const journeys = journeySidebar(locale, entities);
  return {
    [prefix]: journeys,
    [`${prefix}use/`]: journeys,
    [`${prefix}build/`]: journeys,
    [`${prefix}api/cli/prepared-authoring-v2`]: journeys,
    [`${prefix}guide/quickstart`]: guideSidebar,
    [`${prefix}reference/client-compatibility`]: journeys
  };
}

function localeLabels(locale) {
  if (locale === "es") {
    return {
      guide: "Guía",
      guideStart: "Inicio",
      guideCoreIdea: "Idea central",
      guideBuild: "Crear plugins",
      guideStarters: "Starters y ejemplos",
      guideDelivery: "Entrega y targets",
      guideOperate: "Producción y CI",
      guideOverview: "Resumen",
      quickstart: "Inicio rápido",
      installation: "Instalación",
      whatYouCanBuild: "Lo que puedes construir",
      oneProjectMultipleTargets: "Un proyecto, múltiples objetivos",
      chooseTarget: "Elige un objetivo",
      firstPlugin: "Cree su primer complemento",
      pythonRuntimeGuide: "Cree un complemento de tiempo de ejecución Python",
      teamReadyPlugin: "Cree un complemento listo para el equipo",
      claudePlugin: "Cree un complemento Claude",
      nodeTypescriptRuntime: "Cree un complemento de tiempo de ejecución Node/TypeScript",
      starterTemplates: "Plantillas de inicio",
      examplesAndRecipes: "Ejemplos y recetas",
      chooseStarterRepo: "Elija un repositorio inicial",
      chooseDeliveryModel: "Elija el modelo de entrega",
      bundleHandoff: "Transferencia de paquete",
      packageAndWorkspaceTargets: "Configuración de paquetes y integración",
      howToPublishPlugins: "Cómo publicar complementos",
      productionReadiness: "Preparación para la producción",
      ciIntegration: "Integración de CI",
      concepts: "Conceptos",
      conceptsFoundation: "Base",
      conceptsDecisions: "Modelos de decisión",
      conceptsOverview: "Resumen",
      whyPluginKitAi: "Por qué plugin-kit-ai",
      managedProjectModel: "Cómo funciona plugin-kit-ai",
      authoringArchitecture: "Fuente y resultados del proyecto",
      stabilityModel: "Modelo de estabilidad",
      targetModel: "Modelo objetivo",
      choosingRuntime: "Elegir el tiempo de ejecución",
      reference: "Referencia",
      referenceOperational: "Referencia operativa",
      referenceSupport: "Soporte y límites",
      referenceHelp: "Ayuda",
      referenceOverview: "Resumen",
      installChannels: "Canales de instalación",
      versionAndCompatibility: "Política de versión y compatibilidad",
      authoringWorkflow: "Flujo de trabajo de creación",
      repositoryStandard: "Estándar de repositorio",
      supportBoundary: "Límite de soporte",
      targetSupport: "Soporte de targets",
      faq: "FAQ",
      troubleshooting: "Solución de problemas",
      glossary: "Glosario",
      apiOverview: "Resumen de API",
      cliReference: "Referencia CLI",
      cliOverview: "Resumen",
      cliCore: "Comandos principales",
      cliBundle: "Bundle",
      cliCompletion: "Completion",
      cliSkills: "Skills",
      goSdk: "Go SDK",
      goSdkOverview: "Resumen",
      nodeRuntime: "Node Runtime",
      nodeRuntimeOverview: "Resumen",
      pythonRuntime: "Python Runtime",
      pythonRuntimeOverview: "Resumen",
      platformEvents: "Eventos de plataforma",
      platformEventsOverview: "Resumen",
      capabilities: "Capacidades",
      capabilitiesOverview: "Resumen",
      releases: "Lanzamientos",
      releasesOverview: "Resumen"
    };
  }

  if (locale === "fr") {
    return {
      guide: "Guide",
      guideStart: "Démarrer",
      guideCoreIdea: "Idée centrale",
      guideBuild: "Créer des plugins",
      guideStarters: "Starters et exemples",
      guideDelivery: "Livraison et targets",
      guideOperate: "Production et CI",
      guideOverview: "Vue d'ensemble",
      quickstart: "Démarrage rapide",
      installation: "Installation",
      whatYouCanBuild: "Ce que vous pouvez construire",
      oneProjectMultipleTargets: "Un projet, plusieurs cibles",
      chooseTarget: "Choisissez une cible",
      firstPlugin: "Créez votre premier plugin",
      pythonRuntimeGuide: "Créer un plugin d'exécution Python",
      teamReadyPlugin: "Créez un plugin prêt pour l'équipe",
      claudePlugin: "Créer un plugin Claude",
      nodeTypescriptRuntime: "Créer un plugin d'exécution Node/TypeScript",
      starterTemplates: "Modèles de démarrage",
      examplesAndRecipes: "Exemples et recettes",
      chooseStarterRepo: "Choisissez un dépôt de démarrage",
      chooseDeliveryModel: "Choisissez le modèle de livraison",
      bundleHandoff: "Transfert du bundle",
      packageAndWorkspaceTargets: "Packages et configuration de l'intégration",
      howToPublishPlugins: "Comment publier des plugins",
      productionReadiness: "Préparation à la production",
      ciIntegration: "Intégration CI",
      concepts: "Concepts",
      conceptsFoundation: "Fondations",
      conceptsDecisions: "Modèles de décision",
      conceptsOverview: "Vue d'ensemble",
      whyPluginKitAi: "Pourquoi plugin-kit-ai",
      managedProjectModel: "Comment fonctionne plugin-kit-ai",
      authoringArchitecture: "Source et résultats du projet",
      stabilityModel: "Modèle de stabilité",
      targetModel: "Modèle cible",
      choosingRuntime: "Choisir l'environnement d'exécution",
      reference: "Référence",
      referenceOperational: "Référence opérationnelle",
      referenceSupport: "Support et limites",
      referenceHelp: "Aide",
      referenceOverview: "Vue d'ensemble",
      installChannels: "Canaux d'installation",
      versionAndCompatibility: "Politique de version et de compatibilité",
      authoringWorkflow: "Flux de travail de création",
      repositoryStandard: "Norme de référentiel",
      supportBoundary: "Limite de support",
      targetSupport: "Support des targets",
      faq: "FAQ",
      troubleshooting: "Dépannage",
      glossary: "Glossaire",
      apiOverview: "Vue d'ensemble API",
      cliReference: "Référence CLI",
      cliOverview: "Vue d'ensemble",
      cliCore: "Commandes principales",
      cliBundle: "Bundle",
      cliCompletion: "Completion",
      cliSkills: "Skills",
      goSdk: "Go SDK",
      goSdkOverview: "Vue d'ensemble",
      nodeRuntime: "Node Runtime",
      nodeRuntimeOverview: "Vue d'ensemble",
      pythonRuntime: "Python Runtime",
      pythonRuntimeOverview: "Vue d'ensemble",
      platformEvents: "Événements de plateforme",
      platformEventsOverview: "Vue d'ensemble",
      capabilities: "Capacités",
      capabilitiesOverview: "Vue d'ensemble",
      releases: "Versions",
      releasesOverview: "Vue d'ensemble"
    };
  }

  if (locale === "zh") {
    return {
      guide: "指南",
      guideStart: "开始",
      guideCoreIdea: "核心思路",
      guideBuild: "构建插件",
      guideStarters: "Starter 与示例",
      guideDelivery: "交付与 targets",
      guideOperate: "生产与 CI",
      guideOverview: "总览",
      quickstart: "快速入门",
      installation: "安装",
      whatYouCanBuild: "您可以构建什么",
      oneProjectMultipleTargets: "一个项目，多个目标",
      chooseTarget: "选择一个目标",
      firstPlugin: "构建你的第一个插件",
      pythonRuntimeGuide: "构建 Python 运行时插件",
      teamReadyPlugin: "构建一个团队就绪的插件",
      claudePlugin: "构建 Claude 插件",
      nodeTypescriptRuntime: "构建 Node/TypeScript 运行时插件",
      starterTemplates: "入门模板",
      examplesAndRecipes: "示例和食谱",
      chooseStarterRepo: "选择一个入门存储库",
      chooseDeliveryModel: "选择交付模式",
      bundleHandoff: "捆绑交接",
      packageAndWorkspaceTargets: "包和集成设置",
      howToPublishPlugins: "如何发布插件",
      productionReadiness: "生产准备情况",
      ciIntegration: "CI 集成",
      concepts: "概念",
      conceptsFoundation: "基础",
      conceptsDecisions: "决策模型",
      conceptsOverview: "总览",
      whyPluginKitAi: "为什么 plugin-kit-ai",
      managedProjectModel: "plugin-kit-ai 的工作原理",
      authoringArchitecture: "项目来源和产出",
      stabilityModel: "稳定性模型",
      targetModel: "目标模型",
      choosingRuntime: "选择运行时",
      reference: "参考",
      referenceOperational: "操作参考",
      referenceSupport: "支持与边界",
      referenceHelp: "帮助",
      referenceOverview: "总览",
      installChannels: "安装频道",
      versionAndCompatibility: "版本和兼容性政策",
      authoringWorkflow: "创作工作流程",
      repositoryStandard: "存储库标准",
      supportBoundary: "支持边界",
      targetSupport: "目标支持",
      faq: "FAQ",
      troubleshooting: "故障排除",
      glossary: "词汇表",
      apiOverview: "API 总览",
      cliReference: "CLI 参考",
      cliOverview: "总览",
      cliCore: "核心命令",
      cliBundle: "Bundle",
      cliCompletion: "Completion",
      cliSkills: "Skills",
      goSdk: "Go SDK",
      goSdkOverview: "总览",
      nodeRuntime: "Node Runtime",
      nodeRuntimeOverview: "总览",
      pythonRuntime: "Python Runtime",
      pythonRuntimeOverview: "总览",
      platformEvents: "平台事件",
      platformEventsOverview: "总览",
      capabilities: "Capabilities",
      capabilitiesOverview: "总览",
      releases: "发布",
      releasesOverview: "总览"
    };
  }

  if (locale === "ru") {
    return {
      guide: "Гайды",
      guideStart: "Старт",
      guideCoreIdea: "Суть проекта",
      guideBuild: "Сборка плагина",
      guideStarters: "Starter'ы и примеры",
      guideDelivery: "Поставка и target'ы",
      guideOperate: "Продакшен и CI",
      guideOverview: "Обзор",
      quickstart: "Быстрый старт",
      installation: "Установка",
      whatYouCanBuild: "Что можно собрать",
      oneProjectMultipleTargets: "Один проект, несколько target'ов",
      chooseTarget: "Выбор target",
      firstPlugin: "Соберите первый плагин",
      pythonRuntimeGuide: "Соберите Python runtime-плагин",
      teamReadyPlugin: "Сделайте плагин готовым для команды",
      claudePlugin: "Соберите плагин для Claude",
      nodeTypescriptRuntime: "Соберите Node/TypeScript runtime-плагин",
      starterTemplates: "Стартовые шаблоны",
      examplesAndRecipes: "Примеры и рецепты",
      chooseStarterRepo: "Выбор стартового репозитория",
      chooseDeliveryModel: "Выбор модели поставки",
      bundleHandoff: "Передача bundle",
      packageAndWorkspaceTargets: "Пакеты и настройка интеграций",
      howToPublishPlugins: "Как публиковать плагины",
      productionReadiness: "Готовность к продакшену",
      ciIntegration: "Интеграция с CI",
      concepts: "Концепции",
      conceptsFoundation: "Основа",
      conceptsDecisions: "Модели выбора",
      conceptsOverview: "Обзор",
      whyPluginKitAi: "Зачем нужен plugin-kit-ai",
      managedProjectModel: "Как работает plugin-kit-ai",
      authoringArchitecture: "Исходники и generated outputs",
      stabilityModel: "Модель стабильности",
      targetModel: "Модель target'ов",
      choosingRuntime: "Выбор runtime",
      reference: "Справочник",
      referenceOperational: "Рабочий контур",
      referenceSupport: "Поддержка и границы",
      referenceHelp: "Помощь",
      referenceOverview: "Обзор",
      installChannels: "Каналы установки",
      versionAndCompatibility: "Политика версий и совместимости",
      authoringWorkflow: "Процесс авторинга",
      repositoryStandard: "Стандарт репозитория",
      supportBoundary: "Граница поддержки",
      targetSupport: "Поддержка target'ов",
      faq: "Частые вопросы",
      troubleshooting: "Диагностика проблем",
      glossary: "Словарь терминов",
      apiOverview: "Обзор API",
      cliReference: "Справочник CLI",
      cliOverview: "Обзор CLI",
      cliCore: "Основные команды",
      cliBundle: "Bundle",
      cliCompletion: "Completion",
      cliSkills: "Skills",
      goSdk: "Go SDK",
      goSdkOverview: "Обзор Go SDK",
      nodeRuntime: "Node Runtime",
      nodeRuntimeOverview: "Обзор Node Runtime",
      pythonRuntime: "Python Runtime",
      pythonRuntimeOverview: "Обзор Python Runtime",
      platformEvents: "События платформ",
      platformEventsOverview: "Обзор событий",
      capabilities: "Capabilities",
      capabilitiesOverview: "Обзор возможностей",
      releases: "Релизы",
      releasesOverview: "Обзор"
    };
  }

  return {
    guide: "Guide",
    guideStart: "Start",
    guideCoreIdea: "Core Idea",
    guideBuild: "Build Plugins",
    guideStarters: "Starters And Examples",
    guideDelivery: "Delivery And Targets",
    guideOperate: "Production And CI",
    guideOverview: "Overview",
    quickstart: "Quickstart",
    installation: "Installation",
    whatYouCanBuild: "What You Can Build",
    oneProjectMultipleTargets: "One Project, Multiple Targets",
    chooseTarget: "Choose A Target",
    firstPlugin: "Build Your First Plugin",
    pythonRuntimeGuide: "Build A Python Runtime Plugin",
    teamReadyPlugin: "Build A Team-Ready Plugin",
    claudePlugin: "Build A Claude Plugin",
    nodeTypescriptRuntime: "Node/TypeScript Runtime",
    starterTemplates: "Starter Templates",
    examplesAndRecipes: "Examples And Recipes",
    chooseStarterRepo: "Choose A Starter Repo",
    chooseDeliveryModel: "Choose Delivery Model",
    bundleHandoff: "Bundle Handoff",
    packageAndWorkspaceTargets: "Packages And Integration Setup",
    howToPublishPlugins: "How To Publish Plugins",
    productionReadiness: "Production Readiness",
    ciIntegration: "CI Integration",
    concepts: "Concepts",
    conceptsFoundation: "Foundation",
    conceptsDecisions: "Decision Models",
    conceptsOverview: "Overview",
    whyPluginKitAi: "Why plugin-kit-ai",
    managedProjectModel: "How plugin-kit-ai Works",
    authoringArchitecture: "Project Source And Outputs",
    stabilityModel: "Stability Model",
    targetModel: "Target Model",
    choosingRuntime: "Choosing Runtime",
    reference: "Reference",
    referenceOperational: "Operational Reference",
    referenceSupport: "Support And Boundaries",
    referenceHelp: "Help",
    referenceOverview: "Overview",
    installChannels: "Install Channels",
    versionAndCompatibility: "Version And Compatibility Policy",
    authoringWorkflow: "Authoring Workflow",
    repositoryStandard: "Repository Standard",
    supportBoundary: "Support Boundary",
    targetSupport: "Target Support",
    faq: "FAQ",
    troubleshooting: "Troubleshooting",
    glossary: "Glossary",
    apiOverview: "API Overview",
    cliReference: "CLI Reference",
    cliOverview: "Overview",
    cliCore: "Core Commands",
    cliBundle: "Bundle",
    cliCompletion: "Completion",
    cliSkills: "Skills",
    goSdk: "Go SDK",
    goSdkOverview: "Overview",
    nodeRuntime: "Node Runtime",
    nodeRuntimeOverview: "Overview",
    pythonRuntime: "Python Runtime",
    pythonRuntimeOverview: "Overview",
    platformEvents: "Platform Events",
    platformEventsOverview: "Overview",
    capabilities: "Capabilities",
    capabilitiesOverview: "Overview",
    releases: "Releases",
    releasesOverview: "Overview"
  };
}

function mirrorGeneratedPage(page, locale) {
  const relativePath = page.relativePath.replace(/^en\//, `${locale}/`);
  const content = localizeMirroredGeneratedContent(
    page.content
      .replace(/\blocale:\s*"en"/, `locale: "${locale}"`)
      .replaceAll("/en/", `/${locale}/`),
    locale
  );

  return {
    ...page,
    locale,
    relativePath,
    content
  };
}

function localizeMirroredGeneratedContent(content, locale) {
  const replacements = mirroredGeneratedReplacements(locale);
  return replacements.reduce((acc, [from, to]) => acc.replaceAll(from, to), content);
}

function mirroredGeneratedReplacements(locale) {
  if (locale === "es") {
    return [
      ["CLI Reference", "Referencia CLI"],
      ["Generated from the live Cobra command tree.", "Generado a partir del árbol real de comandos Cobra."],
      ["Generated CLI reference", "Referencia CLI generada"],
      ["Core Commands", "Comandos principales"],
      ["Generated via pydoc-markdown.", "Generado mediante pydoc-markdown."],
      ["Generated from the public Go package via gomarkdoc.", "Generado desde el paquete público de Go mediante gomarkdoc."],
      ["Import path", "Ruta de importación"],
      ["Platform Events", "Eventos de plataforma"],
      ["Capabilities", "Capacidades"],
      ["Target Support", "Soporte de targets"],
      ["Generated via TypeDoc and typedoc-plugin-markdown.", "Generado mediante TypeDoc y typedoc-plugin-markdown."],
      ["Generated Node runtime pages:", "Páginas generadas de Node runtime:"],
      ["Generated Python runtime reference", "Referencia generada de Python runtime"],
      ["Generated Go SDK package reference", "Referencia generada del paquete Go SDK"],
      ["Generated platform event reference", "Referencia generada de eventos de plataforma"],
      ["Generated capability reference", "Referencia generada de capacidades"],
      ["Generated target support summary", "Resumen generado de soporte de targets"],
      ["Generated Node runtime reference", "Referencia generada de Node runtime"],
      ["Generated runtime helper API", "API generada de helpers de runtime"]
    ];
  }
  if (locale === "fr") {
    return [
      ["CLI Reference", "Référence CLI"],
      ["Generated from the live Cobra command tree.", "Généré à partir de l'arbre réel de commandes Cobra."],
      ["Generated CLI reference", "Référence CLI générée"],
      ["Core Commands", "Commandes principales"],
      ["Generated via pydoc-markdown.", "Généré via pydoc-markdown."],
      ["Generated from the public Go package via gomarkdoc.", "Généré à partir du package Go public via gomarkdoc."],
      ["Import path", "Chemin d'import"],
      ["Platform Events", "Événements de plateforme"],
      ["Capabilities", "Capacités"],
      ["Target Support", "Support des targets"],
      ["Generated via TypeDoc and typedoc-plugin-markdown.", "Généré via TypeDoc et typedoc-plugin-markdown."],
      ["Generated Node runtime pages:", "Pages Node runtime générées :"],
      ["Generated Python runtime reference", "Référence Python runtime générée"],
      ["Generated Go SDK package reference", "Référence générée du package Go SDK"],
      ["Generated platform event reference", "Référence générée des événements de plateforme"],
      ["Generated capability reference", "Référence générée des capacités"],
      ["Generated target support summary", "Résumé généré du support des targets"],
      ["Generated Node runtime reference", "Référence Node runtime générée"],
      ["Generated runtime helper API", "API générée des helpers runtime"]
    ];
  }
  if (locale === "zh") {
    return [
      ["CLI Reference", "CLI 参考"],
      ["Generated from the live Cobra command tree.", "由实际的 Cobra 命令树生成。"],
      ["Generated CLI reference", "生成的 CLI 参考"],
      ["Core Commands", "核心命令"],
      ["Generated via pydoc-markdown.", "通过 pydoc-markdown 生成。"],
      ["Generated from the public Go package via gomarkdoc.", "通过 gomarkdoc 从公开 Go package 生成。"],
      ["Import path", "导入路径"],
      ["Platform Events", "平台事件"],
      ["Capabilities", "Capabilities"],
      ["Target Support", "Target 支持"],
      ["Generated via TypeDoc and typedoc-plugin-markdown.", "通过 TypeDoc 和 typedoc-plugin-markdown 生成。"],
      ["Generated Node runtime pages:", "生成的 Node runtime 页面："],
      ["Generated Python runtime reference", "生成的 Python runtime 参考"],
      ["Generated Go SDK package reference", "生成的 Go SDK package 参考"],
      ["Generated platform event reference", "生成的平台事件参考"],
      ["Generated capability reference", "生成的 capability 参考"],
      ["Generated target support summary", "生成的 target 支持摘要"],
      ["Generated Node runtime reference", "生成的 Node runtime 参考"],
      ["Generated runtime helper API", "生成的 runtime helper API"]
    ];
  }
  return [];
}
