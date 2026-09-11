import { extractHistorical } from "./historical.mjs";

export const extractPlatformData = () => extractHistorical(["platform-events", "capabilities"], ["target-support"]);

import fs from "node:fs/promises";
import path from "node:path";
import { docsToolsRoot, repoBrowserUrl, repoRoot, sourceRefs } from "../config/site.mjs";
import { renderMarkdownPage } from "../lib/frontmatter.mjs";
import { ensureDir } from "../lib/fs.mjs";
import { makeEntity } from "../lib/site-model.mjs";
import { run } from "../lib/process.mjs";

export async function extractLegacyPlatformData() {
  const root = path.join(docsToolsRoot, "platform");
  const eventsPath = path.join(root, "events.json");
  const targetsPath = path.join(root, "targets.json");
  const capabilitiesPath = path.join(root, "capabilities.json");
  await ensureDir(root);
  await run(
    "go",
    [
      "run",
      "./cli/plugin-kit-ai/cmd/plugin-kit-ai",
      "__docs",
      "export-support",
      "--events-path",
      eventsPath,
      "--targets-path",
      targetsPath,
      "--capabilities-path",
      capabilitiesPath
    ],
    { cwd: repoRoot }
  );
  const events = JSON.parse(await fs.readFile(eventsPath, "utf8"));
  const targets = JSON.parse(await fs.readFile(targetsPath, "utf8"));
  const capabilities = JSON.parse(await fs.readFile(capabilitiesPath, "utf8"));

  const entities = [];
  const pages = [];

  const platforms = new Map();
  for (const event of events) {
    if (!platforms.has(event.platform)) {
      platforms.set(event.platform, []);
    }
    platforms.get(event.platform).push(event);
  }

  for (const [platform, platformEvents] of platforms) {
    const canonicalId = `event-platform:${platform}`;
    entities.push(
      makeEntity({
        canonicalId,
        kind: "event",
        surface: "platform-events",
        localeStrategy: "mirrored",
        title: platform,
        summary: `${platform} event surface`,
        stability: "public-stable",
        maturity: "stable",
        sourceKind: "support-export",
        sourceRef: sourceRefs.supportMatrix,
        pathEn: `/en/api/platform-events/${platform}`,
        pathRu: `/ru/api/platform-events/${platform}`,
        searchTerms: [platform, ...platformEvents.map((entry) => entry.event)]
      })
    );
    for (const locale of ["en", "ru"]) {
      const table = platformEvents
        .map(
          (entry) =>
            `| ${entry.event} | ${localizeEventMaturity(entry.maturity, locale)} | ${localizeEventContract(entry.contract_class, locale)} | ${localizeEventSummary(entry, locale)} |\n`
        )
        .join("");
      pages.push({
        locale,
        relativePath: path.join(locale, "api", "platform-events", `${platform}.md`),
        content: renderMarkdownPage(
          {
            title: platform,
            description: `Event reference for ${platform}`,
            canonicalId,
            surface: "platform-events",
            section: "api",
            locale,
            generated: true,
            editLink: false,
            stability: "public-stable",
            maturity: "stable",
            sourceRef: sourceRefs.supportMatrix,
            translationRequired: false
          },
          `# ${platform}\n\n| ${locale === "ru" ? "Событие" : "Event"} | ${locale === "ru" ? "Зрелость" : "Maturity"} | ${locale === "ru" ? "Контракт" : "Contract"} | ${locale === "ru" ? "Сводка" : "Summary"} |\n| --- | --- | --- | --- |\n${table}`
        )
      });
    }
  }

  for (const capability of capabilities) {
    const relatedEvents = events.filter((entry) => entry.capabilities.includes(capability));
    const canonicalId = `capability:${capability}`;
    entities.push(
      makeEntity({
        canonicalId,
        kind: "capability",
        surface: "capabilities",
        localeStrategy: "mirrored",
        title: capability,
        summary: `Capability ${capability}`,
        stability: "public-stable",
        maturity: "stable",
        sourceKind: "support-export",
        sourceRef: sourceRefs.supportMatrix,
        pathEn: `/en/api/capabilities/${capability}`,
        pathRu: `/ru/api/capabilities/${capability}`,
        searchTerms: [capability]
      })
    );
    const list = relatedEvents.map((entry) => `- \`${entry.platform}/${entry.event}\``).join("\n");
    for (const locale of ["en", "ru"]) {
      pages.push({
        locale,
        relativePath: path.join(locale, "api", "capabilities", `${capability}.md`),
        content: renderMarkdownPage(
          {
            title: capability,
            description: `Capability reference for ${capability}`,
            canonicalId,
            surface: "capabilities",
            section: "api",
            locale,
            generated: true,
            editLink: false,
            stability: "public-stable",
            maturity: "stable",
            sourceRef: sourceRefs.supportMatrix,
            translationRequired: false
          },
          `# ${capability}\n\n${locale === "ru" ? "Связанные runtime-события:" : "Related runtime events:"}\n\n${list}`
        )
      });
    }
  }

  for (const locale of ["en", "ru"]) {
    const platformList = [...platforms.keys()]
      .map((platform) => `- [\`${platform}\`](/${locale}/api/platform-events/${platform})`)
      .join("\n");
    const capabilityList = capabilities
      .map((capability) => `- [\`${capability}\`](/${locale}/api/capabilities/${capability})`)
      .join("\n");
    const targetRows = targets
      .map(
        (entry) =>
          `| ${entry.target} | ${compactProductionClass(entry.production_class, locale)} | ${compactRuntimeContract(entry.runtime_contract, entry.target, locale)} | ${compactInstallModel(entry.install_model, locale)} |`
      )
      .join("\n");
    pages.push({
      locale,
      relativePath: path.join(locale, "api", "platform-events", "index.md"),
      content: renderMarkdownPage(
        {
          title: locale === "ru" ? "События платформ" : "Platform Events",
          description: locale === "ru" ? "Сгенерированный справочник по событиям платформ" : "Generated platform event reference",
          canonicalId: "page:api:platform-events:index",
          surface: "platform-events",
          section: "api",
          locale,
          generated: true,
          editLink: false,
          stability: "public-stable",
          maturity: "stable",
          sourceRef: sourceRefs.supportMatrix,
          translationRequired: false
        },
        `# ${locale === "ru" ? "События платформ" : "Platform Events"}\n\n${
          locale === "ru"
            ? "Эта зона показывает точные runtime-события по платформам. Открывайте её, когда уже знаете нужный lane и хотите увидеть текущий контракт на уровне событий."
            : "This area shows exact runtime events by platform. Open it when you already know the lane you care about and want the current event-level contract."
        }\n\n${
          locale === "ru"
            ? "- Открывайте её, когда уже знаете целевую платформу и хотите увидеть контракт на уровне событий.\n- Используйте `Capabilities`, когда нужен взгляд поперёк платформ, а не разбор по одной платформе."
            : "- Open this when you already know the target and need the event-level contract.\n- Use `Capabilities` when you want a cross-platform view instead of a platform-first view."
        }\n\n${platformList}`
      )
    });
    pages.push({
      locale,
      relativePath: path.join(locale, "api", "capabilities", "index.md"),
      content: renderMarkdownPage(
        {
          title: "Capabilities",
          description: "Generated capability reference",
          canonicalId: "page:api:capabilities:index",
          surface: "capabilities",
          section: "api",
          locale,
          generated: true,
          editLink: false,
          stability: "public-stable",
      maturity: "stable",
          sourceRef: sourceRefs.supportMatrix,
          translationRequired: false
        },
        `# ${locale === "ru" ? "Capabilities" : "Capabilities"}\n\n${
          locale === "ru"
            ? "Capabilities показывают runtime-поведение поперёк платформ после того, как вы уже понимаете, какой delivery lane проектируете."
            : "Capabilities give you a cross-platform view of runtime behavior after you already know which delivery lane you are designing for."
        }\n\n${
          locale === "ru"
            ? "- Открывайте эту зону, когда важно само действие или реакция, а не только имя платформы.\n- Это лучший вход, если вы сравниваете похожее поведение между Claude и Codex."
            : "- Open this area when you care about the behavior itself, not only the platform name.\n- This is the better entry point when you compare similar behavior across Claude and Codex."
        }\n\n${capabilityList}`
      )
    });
    pages.push({
      locale,
      relativePath: path.join(locale, "reference", "target-support.md"),
      content: renderMarkdownPage(
        {
          title: locale === "ru" ? "Поддержка target’ов" : "Target Support",
          description: locale === "ru" ? "Сводка по поддержке target’ов" : "Generated target support summary",
          canonicalId: "page:reference:target-support",
          surface: "reference",
          section: "reference",
          locale,
          generated: true,
          editLink: false,
          stability: "public-stable",
          maturity: "stable",
          sourceRef: sourceRefs.targetSupportMatrix,
          translationRequired: false
        },
        `# ${locale === "ru" ? "Поддержка target’ов" : "Target Support"}\n\n${
          locale === "ru"
            ? "Используйте эту страницу, когда нужен компактный lane map по runtime, package, extension и repo-managed integration outputs."
            : "Use this page when you need the compact lane map across runtime, package, extension, and repo-managed integration outputs."
        }\n\n| ${locale === "ru" ? "Цель" : "Target"} | ${locale === "ru" ? "Класс production" : "Production Class"} | ${locale === "ru" ? "Runtime-контракт" : "Runtime Contract"} | ${locale === "ru" ? "Модель установки" : "Install Model"} |\n| --- | --- | --- | --- |\n${targetRows}\n\n${
          locale === "ru"
            ? "Для полной картины свяжите эту матрицу с [Границей поддержки](/ru/reference/support-boundary) и [Моделью target’ов](/ru/concepts/target-model)."
            : "For full framing, pair this matrix with [Support Boundary](/en/reference/support-boundary) and [Target Model](/en/concepts/target-model)."
        }\n`
      )
    });
  }

  return { entities, pages };
}

function compactProductionClass(value, locale) {
	if (value === "production-ready") {
		return locale === "ru" ? "рекомендуемый production lane" : "recommended production lane";
	}
	if (value === "production-ready package lane") {
		return locale === "ru" ? "рекомендуемый package lane" : "recommended package lane";
	}
	if (value === "production-ready runtime lane") {
		return locale === "ru" ? "рекомендуемый runtime lane" : "recommended runtime lane";
	}
	if (value === "packaging-only target") {
		return locale === "ru" ? "repo-managed integration lane" : "repo-managed integration lane";
	}
	return value;
}

function compactRuntimeContract(value, target, locale) {
  if (target === "claude") {
    return locale === "ru" ? "стабильный поднабор runtime" : "stable runtime subset";
  }
  if (target === "codex-runtime") {
    return locale === "ru" ? "стабильный notify-runtime" : "stable notify runtime";
  }
  if (target === "codex-package") {
    return locale === "ru" ? "только официальный пакет" : "official package only";
  }
  if (target === "gemini") {
    return locale === "ru" ? "упаковка, не runtime" : "packaging, not runtime";
  }
  if (target === "cursor" || target === "opencode") {
    return locale === "ru" ? "workspace-config вариант" : "workspace-config lane";
  }
  return value;
}

function compactInstallModel(value, locale) {
  if (value.includes("marketplace")) {
    return locale === "ru" ? "marketplace или локально" : "marketplace or local";
  }
  if (value.includes("plugin directory")) {
    return locale === "ru" ? "каталог плагина или кэш" : "plugin dir or cache";
  }
  if (value.includes("repo-local")) {
    return locale === "ru" ? "локально в репозитории" : "repo-local";
  }
  if (value.includes("workspace")) {
    return locale === "ru" ? "конфигурация workspace" : "workspace config";
  }
  if (value.includes("copy install")) {
    return locale === "ru" ? "установка копированием" : "copy install";
  }
  return value;
}

function localizeEventMaturity(value, locale) {
  if (locale !== "ru") {
    return value;
  }
  if (value === "stable") {
    return "stable";
  }
  if (value === "beta") {
    return "beta";
  }
  return value;
}

function localizeEventContract(value, locale) {
  if (locale !== "ru") {
    return value;
  }
  if (value === "production-ready") {
    return "готово для production";
  }
  if (value === "runtime-supported but not stable") {
    return "runtime поддерживается, но ещё не stable";
  }
  return value;
}

function localizeEventSummary(entry, locale) {
  if (locale !== "ru") {
    return entry.summary;
  }

  if (entry.platform === "claude") {
    if (entry.maturity === "stable") {
      return `Claude hook \`${entry.event}\` со стабильным контрактом.`;
    }
    return `Beta hook Claude \`${entry.event}\`, доступный в runtime, но не входящий в stable-контракт.`;
  }

  if (entry.platform === "codex" && entry.event === "Notify") {
    return "Стабильный hook `notify` для Codex.";
  }

  return entry.summary;
}
