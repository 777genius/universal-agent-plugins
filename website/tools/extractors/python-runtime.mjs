import path from "node:path";
import { docsToolsRoot, repoBrowserUrl, sourceRefs, websiteRoot } from "../config/site.mjs";
import { normalizeGeneratedMarkdown, renderMarkdownPage } from "../lib/frontmatter.mjs";
import { ensureDir } from "../lib/fs.mjs";
import { makeEntity } from "../lib/site-model.mjs";
import { run } from "../lib/process.mjs";
import fs from "node:fs/promises";

async function ensurePythonVenv(venvDir) {
  const pythonPath = path.join(venvDir, "bin", "python");
  try {
    await run(pythonPath, ["-c", "import importlib.metadata; assert importlib.metadata.version('pydoc-markdown') == '4.8.2'"], { cwd: websiteRoot });
    return { pythonPath };
  } catch (cause) {
    throw new Error(`Offline docs prerequisite missing: ${pythonPath} with pydoc-markdown==4.8.2; provision separately in the disposable sandbox`, { cause });
  }
}

export async function extractPythonRuntime() {
  const root = path.join(docsToolsRoot, "python-runtime");
  const venvDir = path.join(docsToolsRoot, "python-venv");
  await ensureDir(root);
  const { pythonPath } = await ensurePythonVenv(venvDir);
  const body = normalizeGeneratedMarkdown(await run(
    pythonPath,
    [
      "-m",
      "pydoc_markdown.main",
      "--module",
      "plugin_kit_ai_runtime",
      "--search-path",
      "../python/plugin-kit-ai-runtime/src",
      "--render-toc"
    ],
    { cwd: websiteRoot }
  ));

  const canonicalId = "python-runtime:plugin_kit_ai_runtime";
  const entities = [
    makeEntity({
      canonicalId,
      kind: "package",
      surface: "runtime-python",
      localeStrategy: "mirrored",
      title: "plugin_kit_ai_runtime",
      summary: "Public Python runtime helpers",
      stability: "public-stable",
      maturity: "stable",
      sourceKind: "pydoc-markdown",
      sourceRef: sourceRefs.pythonRuntime,
      pathEn: "/en/api/runtime-python/plugin-kit-ai-runtime",
      pathRu: "/ru/api/runtime-python/plugin-kit-ai-runtime",
      searchTerms: ["plugin_kit_ai_runtime", "python runtime"]
    })
  ];

  const pages = ["en", "ru"].map((locale) => ({
    locale,
    relativePath: path.join(locale, "api", "runtime-python", "plugin-kit-ai-runtime.md"),
    content: renderMarkdownPage(
      {
        title: "plugin_kit_ai_runtime",
        description: "Generated Python runtime reference",
        canonicalId,
        surface: "runtime-python",
        section: "api",
        locale,
        generated: true,
        editLink: false,
        stability: "public-stable",
        maturity: "stable",
        sourceRef: sourceRefs.pythonRuntime,
        translationRequired: false
      },
      `<DocMetaCard surface="runtime-python" stability="public-stable" maturity="stable" source-ref="${sourceRefs.pythonRuntime}" source-href="${repoBrowserUrl(sourceRefs.pythonRuntime)}" />\n\n# plugin_kit_ai_runtime\n\n${locale === "ru" ? "Сгенерировано через pydoc-markdown." : "Generated via pydoc-markdown."}\n\n${buildPythonRuntimeLead(locale)}${localizePythonRuntimeBody(locale, body)}`
    )
  }));

  for (const locale of ["en", "ru"]) {
    pages.push({
      locale,
      relativePath: path.join(locale, "api", "runtime-python", "index.md"),
      content: renderMarkdownPage(
        {
          title: "Python Runtime",
          description: "Generated Python runtime reference",
          canonicalId: "page:api:runtime-python:index",
          surface: "runtime-python",
          section: "api",
          locale,
          generated: true,
          editLink: false,
          stability: "public-stable",
          maturity: "stable",
          sourceRef: sourceRefs.pythonRuntime,
          translationRequired: false
        },
        `# Python Runtime\n\n${
          locale === "ru"
            ? "Открывайте эту зону, когда нужен общий API runtime-хелперов для Python-плагина в репозитории."
            : "Open this area when you need the shared runtime helper API for a repo-local Python plugin."
        }\n\n${
          locale === "ru"
            ? "- Это справочник по API, а не пошаговый гайд.\n- Если нужен простой путь от нуля до рабочего Python-плагина, начните с [гайда по Python runtime](/ru/guide/python-runtime).\n- Если выбираете форму проекта, откройте [Что можно построить](/ru/guide/what-you-can-build) и [Выбор runtime](/ru/concepts/choosing-runtime).\n- Если нужен общий dependency-вариант вместо локально сгенерированного helper-файла, используйте пакет [`plugin-kit-ai-runtime`](https://github.com/777genius/universal-agent-plugins/tree/main/python/plugin-kit-ai-runtime)."
            : "- This page is the API reference, not the setup tutorial.\n- If you want the simplest end-to-end setup, start with [Build A Python Runtime Plugin](/en/guide/python-runtime).\n- If you are still choosing the project shape, read [What You Can Build](/en/guide/what-you-can-build) and [Choosing Runtime](/en/concepts/choosing-runtime).\n- If you want the shared-dependency path instead of a repo-local generated helper file, use the [`plugin-kit-ai-runtime`](https://github.com/777genius/universal-agent-plugins/tree/main/python/plugin-kit-ai-runtime) package."
        }\n\n- [\`plugin_kit_ai_runtime\`](/${locale}/api/runtime-python/plugin-kit-ai-runtime)`
      )
    });
  }

  return { entities, pages };
}

function buildPythonRuntimeLead(locale) {
  if (locale !== "ru") {
    return "";
  }

  return [
    "Официальные runtime-хелперы для Python-плагинов на plugin-kit-ai.",
    "Эта страница собирает публичные типы, константы, функции и классы пакета `plugin_kit_ai_runtime`."
  ].join("\n\n") + "\n\n";
}

function localizePythonRuntimeBody(locale, body) {
  if (locale !== "ru") {
    return body;
  }

  return body
    .replace("# Table of Contents", "# Оглавление")
    .replace("## ClaudeApp Objects", "## Объекты ClaudeApp")
    .replace("## CodexApp Objects", "## Объекты CodexApp")
    .replace("**Arguments**:", "**Аргументы**:")
    .replace("Official Python runtime helpers for plugin-kit-ai executable plugins.", "Официальные runtime-хелперы для исполняемых Python-плагинов на plugin-kit-ai.")
    .replace("JSON-shaped payload used by the Python runtime helpers.", "JSON-представление payload, которое используют Python runtime-хелперы.")
    .replace("Handler signature for Claude hooks that return a JSON object or ``None``.", "Сигнатура обработчика для Claude hooks, который возвращает JSON-объект или ``None``.")
    .replace("Handler signature for Codex events that return an exit code or ``None``.", "Сигнатура обработчика для Codex events, который возвращает код выхода или ``None``.")
    .replace("Stable Claude hook names supported by the public Python runtime lane.", "Имена стабильных Claude hooks, поддерживаемых публичной Python runtime-линией.")
    .replace("Extended Claude hook names exposed by the beta Python runtime lane.", "Имена расширенных Claude hooks, доступных в beta Python runtime-линии.")
    .replace("Return the empty JSON object expected by Claude for an allow response.", "Возвращает пустой JSON-объект, который Claude ожидает для разрешающего ответа.")
    .replace("Return exit code ``0`` for Codex handlers that want normal continuation.", "Возвращает код выхода ``0`` для Codex-обработчиков, которым нужно обычное продолжение.")
    .replace("Minimal Claude hook app that dispatches supported hook names to handlers.", "Минимальное Claude-приложение, которое маршрутизирует поддерживаемые имена hooks к обработчикам.")
    .replace("Minimal Codex app that dispatches the ``notify`` event to a handler.", "Минимальное Codex-приложение, которое маршрутизирует событие ``notify`` к обработчику.")
    .replace("Create a Claude runtime app.", "Создаёт Claude runtime-приложение.")
    .replace("Create a Codex runtime app with no registered notify handler.", "Создаёт Codex runtime-приложение без зарегистрированного обработчика notify.")
    .replace("Return a decorator that registers a handler for ``hook_name``.", "Возвращает декоратор, который регистрирует обработчик для ``hook_name``.")
    .replace("Register a handler for the ``Stop`` hook.", "Регистрирует обработчик для hook ``Stop``.")
    .replace("Register a handler for the ``PreToolUse`` hook.", "Регистрирует обработчик для hook ``PreToolUse``.")
    .replace("Register a handler for the ``UserPromptSubmit`` hook.", "Регистрирует обработчик для hook ``UserPromptSubmit``.")
    .replace("Register a handler for the Codex ``notify`` event.", "Регистрирует обработчик для события Codex ``notify``.")
    .replace("Hook names that this binary accepts on argv.", "Имена hooks, которые этот бинарник принимает через argv.")
    .replace("Usage string printed when the invocation is invalid.", "Строка помощи, которая печатается при некорректном вызове.")
    .replaceAll("Dispatch the current process invocation and return the exit code.", "Обрабатывает текущий запуск процесса и возвращает код выхода.");
}

// Preserved provisioning capability; never called by offline preparation.
export async function provisionLegacyPythonVenv(venvDir) {
  const pythonPath = path.join(venvDir, "bin", "python");
  try {
    await run(pythonPath, ["-c", "import pydoc_markdown"], { cwd: websiteRoot });
    return { pythonPath };
  } catch {
    await fs.rm(venvDir, { recursive: true, force: true });
    await run("python3", ["-m", "venv", venvDir], { cwd: websiteRoot });
    await run(pythonPath, ["-m", "pip", "install", "--disable-pip-version-check", "pydoc-markdown==4.8.2"], {
      cwd: websiteRoot
    });
    return { pythonPath };
  }
}
