import fs from "node:fs/promises";
import path from "node:path";
import { docsToolsRoot, repoBrowserUrl, sourceRefs, websiteRoot } from "../config/site.mjs";
import { normalizeGeneratedMarkdown, renderMarkdownPage, stripLeadingHeading, stripLeadingTypedocPrelude } from "../lib/frontmatter.mjs";
import { ensureDir, listMarkdownFiles } from "../lib/fs.mjs";
import { makeEntity } from "../lib/site-model.mjs";
import { run } from "../lib/process.mjs";

export async function extractNodeRuntime() {
  const root = path.join(docsToolsRoot, "node-runtime");
  await ensureDir(root);
  const runtimePackage = JSON.parse(
    await fs.readFile(path.join(repoRootForNodeRuntime(), "package.json"), "utf8")
  );
  await run(
    "pnpm",
    [
      "exec",
      "typedoc",
      "--plugin",
      "typedoc-plugin-markdown",
      "--entryPoints",
      "../npm/plugin-kit-ai-runtime/index.d.ts",
      "--tsconfig",
      "../npm/plugin-kit-ai-runtime/tsconfig.docs.json",
      "--readme",
      "none",
      "--out",
      root
    ],
    { cwd: websiteRoot }
  );

  const markdownFiles = await listMarkdownFiles(root);
  const entities = [];
  const pages = [];

  for (const filePath of markdownFiles) {
    const stem = path.basename(filePath, ".md");
    const rawBody = stripLeadingHeading(
      stripLeadingTypedocPrelude(normalizeGeneratedMarkdown(await fs.readFile(filePath, "utf8")))
    );
    const slug = stem === "README" ? "runtime" : stem.toLowerCase();
    const displayTitle = humanizeNodeTitle(stem);
    const canonicalId = `node-runtime:${stem}`;
    entities.push(
      makeEntity({
        canonicalId,
        kind: "package",
        surface: "runtime-node",
        localeStrategy: "mirrored",
        title: displayTitle,
        summary: `Node runtime reference: ${stem}`,
        stability: "public-stable",
        maturity: "stable",
        sourceKind: "typedoc-markdown",
        sourceRef: sourceRefs.nodeRuntime,
        pathEn: `/en/api/runtime-node/${slug}`,
        pathRu: `/ru/api/runtime-node/${slug}`,
        searchTerms: [stem, "plugin-kit-ai-runtime", "node runtime"]
      })
    );
    for (const locale of ["en", "ru"]) {
      const body = localizeNodeRuntimeBody(locale, rawBody);
      const intro =
        locale === "ru"
          ? "Сгенерировано через TypeDoc и typedoc-plugin-markdown."
          : "Generated via TypeDoc and typedoc-plugin-markdown.";
      pages.push({
        locale,
        relativePath: path.join(locale, "api", "runtime-node", `${slug}.md`),
        content: renderMarkdownPage(
          {
            title: localizedNodeTitle(locale, stem, displayTitle),
            description: `Generated Node runtime reference for ${stem}`,
            canonicalId,
            surface: "runtime-node",
            section: "api",
            locale,
            generated: true,
            editLink: false,
            stability: "public-stable",
            maturity: "stable",
            sourceRef: sourceRefs.nodeRuntime,
            translationRequired: false
          },
          `<DocMetaCard surface="runtime-node" stability="public-stable" maturity="stable" source-ref="${sourceRefs.nodeRuntime}" source-href="${repoBrowserUrl(sourceRefs.nodeRuntime)}" />\n\n# ${localizedNodeTitle(locale, stem, displayTitle)}\n\n${intro}\n\n${buildNodeRuntimeLead(locale, stem, runtimePackage)}${body}`
        )
      });
    }
  }

  for (const locale of ["en", "ru"]) {
    const list = entities
      .map((entry) => `- [\`${entry.title}\`](/${locale}/api/runtime-node/${entry.pathEn.split("/").pop()})`)
      .join("\n");
    pages.push({
      locale,
      relativePath: path.join(locale, "api", "runtime-node", "index.md"),
      content: renderMarkdownPage(
        {
          title: "Node Runtime",
          description: "Generated Node runtime reference",
          canonicalId: "page:api:runtime-node:index",
          surface: "runtime-node",
          section: "api",
          locale,
          generated: true,
          editLink: false,
          stability: "public-stable",
          maturity: "stable",
          sourceRef: sourceRefs.nodeRuntime,
          translationRequired: false
        },
        `# Node Runtime\n\n${
          locale === "ru"
            ? "Открывайте эту зону, когда нужен общий API runtime-хелперов для Node- или TypeScript-плагина в репозитории."
            : "Open this area when you need the shared runtime helper API for a repo-local Node or TypeScript plugin."
        }\n\n${
          locale === "ru"
            ? "- Это справочник по API, а не пошаговый гайд.\n- Если нужен пошаговый путь, начните с [гайда по Node/TypeScript runtime](/ru/guide/node-typescript-runtime).\n- Если выбираете форму проекта, откройте [Что можно построить](/ru/guide/what-you-can-build) и [Выбор runtime](/ru/concepts/choosing-runtime)."
            : "- This page is the API reference, not the setup tutorial.\n- If you want the step-by-step setup path, start with [Build A Node/TypeScript Runtime Plugin](/en/guide/node-typescript-runtime).\n- If you are still choosing the project shape, read [What You Can Build](/en/guide/what-you-can-build) and [Choosing Runtime](/en/concepts/choosing-runtime)."
        }\n\n${locale === "ru" ? "Сгенерированные Node runtime страницы:" : "Generated Node runtime pages:"}\n\n${list}`
      )
    });
  }

  return { entities, pages };
}

function humanizeNodeTitle(stem) {
  if (stem === "README") {
    return "Overview";
  }

  return stem.replace(/([a-z])([A-Z])/g, "$1 $2");
}

function localizedNodeTitle(locale, stem, fallback) {
  if (locale !== "ru") {
    return fallback;
  }

  return {
    README: "Обзор",
    ClaudeApp: "Приложение Claude",
    CodexApp: "Приложение Codex",
    ClaudeHandler: "Обработчик Claude",
    CodexHandler: "Обработчик Codex"
  }[stem] || fallback;
}

function repoRootForNodeRuntime() {
  return path.resolve(websiteRoot, "..", "npm", "plugin-kit-ai-runtime");
}

function buildNodeRuntimeLead(locale, stem, runtimePackage) {
  if (stem !== "README") {
    return "";
  }

  const lines =
    locale === "ru"
      ? [
          "Официальные runtime-хелперы для Node- и TypeScript-плагинов на plugin-kit-ai.",
          "Эта страница собирает в одной точке классы, алиасы типов, константы и runtime-хелперы пакета.",
          "Используйте пакет, когда нужен общий dependency-вариант вместо локально сгенерированного helper-файла."
        ]
      : [
          runtimePackage?.description ||
            "Official Node and TypeScript runtime helpers for plugin-kit-ai executable plugins.",
          "This page brings together the package classes, type aliases, constants, and runtime helpers.",
          "Use the package when you want the shared-dependency path instead of a repo-local generated helper file."
        ];
  return `${lines.join("\n\n")}\n\n`;
}

function localizeNodeRuntimeBody(locale, body) {
  if (locale !== "ru") {
    return body;
  }

  return body
    .replace(/^Defined in:/gm, "Определено в:")
    .replace(/^## Classes$/gm, "## Классы")
    .replace(/^## Type Aliases$/gm, "## Алиасы типов")
    .replace(/^## Variables$/gm, "## Константы и переменные")
    .replace(/^## Functions$/gm, "## Функции")
    .replace(/^## Constructors$/gm, "## Конструкторы")
    .replace(/^## Methods$/gm, "## Методы")
    .replace(/^## Parameters$/gm, "## Параметры")
    .replace(/^## Returns$/gm, "## Возвращает")
    .replace(/^### Constructor$/gm, "### Конструктор")
    .replace(/^#### Parameters$/gm, "#### Параметры")
    .replace(/^#### Returns$/gm, "#### Возвращает")
    .replace("Handler signature for Claude hooks that return an object response or no value.", "Сигнатура обработчика для Claude hooks, который возвращает объект ответа или `void`.")
    .replace("Handler signature for Codex events that return an exit code or no value.", "Сигнатура обработчика для Codex events, который возвращает код выхода или `void`.")
    .replace("JSON-shaped payload used by the runtime helpers when a stricter schema is not known.", "JSON-представление payload, которое используется runtime-хелперами, когда строгая схема неизвестна.")
    .replace("Stable Claude hook names supported by the public runtime lane.", "Имена стабильных Claude hooks, поддерживаемых публичной runtime-линией.")
    .replace("Extended Claude hook names exposed by the beta runtime lane.", "Имена расширенных Claude hooks, доступных в beta runtime-линии.")
    .replace("Returns the empty JSON object expected by Claude when a hook allows the action.", "Возвращает пустой JSON-объект, который Claude ожидает при разрешающем ответе hook.")
    .replace("Returns exit code `0` for Codex handlers that want normal continuation.", "Возвращает код выхода `0` для Codex-обработчиков, которым нужно обычное продолжение.")
    .replace("Minimal Claude hook app that dispatches supported hook names to registered handlers.", "Минимальное Claude-приложение, которое маршрутизирует поддерживаемые имена hooks к зарегистрированным обработчикам.")
    .replace("Minimal Codex app that dispatches the `notify` event to a registered handler.", "Минимальное Codex-приложение, которое маршрутизирует событие `notify` к зарегистрированному обработчику.")
    .replace("Creates a Claude runtime app.", "Создаёт Claude runtime-приложение.")
    .replace("Hook names that this binary accepts on argv.", "Имена hooks, которые этот бинарник принимает через argv.")
    .replace("Usage string printed when the invocation is invalid.", "Строка помощи, которая печатается при некорректном вызове.")
    .replace("Registers a handler for an arbitrary Claude hook name.", "Регистрирует обработчик для произвольного имени Claude hook.")
    .replace("Registers a handler for the `Stop` hook.", "Регистрирует обработчик для hook `Stop`.")
    .replace("Registers a handler for the `PreToolUse` hook.", "Регистрирует обработчик для hook `PreToolUse`.")
    .replace("Registers a handler for the `UserPromptSubmit` hook.", "Регистрирует обработчик для hook `UserPromptSubmit`.")
    .replace("Dispatches the current process invocation and returns the exit code.", "Обрабатывает текущий запуск процесса и возвращает код выхода.")
    .replace("Creates a Codex runtime app with no registered handlers.", "Создаёт Codex runtime-приложение без зарегистрированных обработчиков.")
    .replace("Registers a handler for the Codex `notify` event.", "Регистрирует обработчик для события Codex `notify`.");
}
