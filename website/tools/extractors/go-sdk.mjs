import { verifiedGomarkdoc } from "../lib/preflight.mjs";
import fs from "node:fs/promises";
import path from "node:path";
import { docsToolsRoot, publicGoPackages, repoBrowserUrl, repoRoot } from "../config/site.mjs";
import { normalizeGeneratedMarkdown, renderMarkdownPage, stripLeadingHeading } from "../lib/frontmatter.mjs";
import { ensureDir } from "../lib/fs.mjs";
import { makeEntity, localeTitle } from "../lib/site-model.mjs";
import { run } from "../lib/process.mjs";

export async function extractGoSDK() {
  const gomarkdoc = await verifiedGomarkdoc();
  const root = path.join(docsToolsRoot, "go-sdk");
  await ensureDir(root);
  const entities = [];
  const pages = [];

  for (const pkg of publicGoPackages) {
    const outPath = path.join(root, `${pkg.id}.md`);
    await run(
      gomarkdoc,
      [
        "--repository.url",
        "https://github.com/777genius/universal-agent-plugins",
        "--repository.default-branch",
        "main",
        "--output",
        outPath,
        `./${pkg.relativePath}`
      ],
      { cwd: repoRoot }
    );
    const body = stripLeadingHeading(normalizeGeneratedMarkdown(await fs.readFile(outPath, "utf8")));
    const canonicalId = `go-package:${pkg.importPath}`;
    const slug = pkg.id === "root" ? "sdk" : pkg.id;
    const packageLabel = pkg.id === "root" ? "sdk" : pkg.id;
    entities.push(
      makeEntity({
        canonicalId,
        kind: "package",
        surface: "go-sdk",
        localeStrategy: "mirrored",
        title: packageLabel,
        summary: `Public Go package ${pkg.importPath}`,
        stability: pkg.id === "platformmeta" ? "public-beta" : "public-stable",
        maturity: pkg.id === "platformmeta" ? "beta" : "stable",
        sourceKind: "gomarkdoc",
        sourceRef: pkg.relativePath,
        pathEn: `/en/api/go-sdk/${slug}`,
        pathRu: `/ru/api/go-sdk/${slug}`,
        relatedIds:
          pkg.id === "claude"
            ? ["event-platform:claude"]
            : pkg.id === "codex"
              ? ["event-platform:codex"]
              : pkg.id === "gemini"
                ? ["event-platform:gemini"]
                : [],
        searchTerms: [pkg.importPath, slug]
      })
    );
    for (const locale of ["en", "ru"]) {
      const intro =
        locale === "ru"
          ? "Сгенерировано из публичного Go-пакета через gomarkdoc."
          : "Generated from the public Go package via gomarkdoc.";
      const localizedBody = localizeGoSDKBody(locale, pkg.id, body);
      pages.push({
        locale,
        relativePath: path.join(locale, "api", "go-sdk", `${slug}.md`),
        content: renderMarkdownPage(
          {
            title: localeTitle(locale, packageLabel, packageLabel),
            description: `Generated Go SDK package reference for ${pkg.importPath}`,
            canonicalId,
            surface: "go-sdk",
            section: "api",
            locale,
            generated: true,
            editLink: false,
            stability: pkg.id === "platformmeta" ? "public-beta" : "public-stable",
            maturity: pkg.id === "platformmeta" ? "beta" : "stable",
            sourceRef: pkg.relativePath,
            translationRequired: false
          },
          `<DocMetaCard surface="go-sdk" stability="${pkg.id === "platformmeta" ? "public-beta" : "public-stable"}" maturity="${pkg.id === "platformmeta" ? "beta" : "stable"}" source-ref="${pkg.relativePath}" source-href="${repoBrowserUrl(pkg.relativePath)}" />\n\n# ${packageLabel}\n\n${intro}\n\n**${locale === "ru" ? "Путь импорта" : "Import path"}:** \`${pkg.importPath}\`\n\n${localizedBody}`
        )
      });
    }
  }

  for (const locale of ["en", "ru"]) {
    const packageRows = publicGoPackages
      .map((pkg) => {
        const packageLabel = pkg.id === "root" ? "sdk" : pkg.id;
        const slug = pkg.id === "root" ? "sdk" : pkg.id;
        const summary =
          pkg.id === "root"
            ? locale === "ru"
              ? "Корневой пакет композиции и runtime-входа."
              : "Root composition and runtime entry package."
            : pkg.id === "claude"
              ? locale === "ru"
                ? "Публичные обработчики Claude и подключение событий."
                : "Public Claude-oriented handlers and event wiring."
              : pkg.id === "codex"
                ? locale === "ru"
                  ? "Публичные обработчики Codex и runtime-интеграция."
                  : "Public Codex-oriented handlers and runtime integration."
                : pkg.id === "gemini"
                  ? locale === "ru"
                    ? "Публичные обработчики Gemini и runtime-интеграция."
                    : "Public Gemini-oriented handlers and runtime integration."
                : locale === "ru"
                  ? "Метаданные платформ и служебные хелперы поддержки."
                  : "Platform metadata and support-oriented helpers.";
        return `| [\`${packageLabel}\`](/${locale}/api/go-sdk/${slug}) | ${summary} |`;
      })
      .join("\n");
    const intro =
      locale === "ru"
        ? "Go SDK — рекомендуемый путь по умолчанию, когда нужен самый надёжный и предсказуемый контракт для production."
        : "The Go SDK is the recommended default path when you want the strongest production contract.";
    const guidance =
      locale === "ru"
        ? "- Открывайте эту зону, когда строите production-ориентированный плагин на Go.\n- Это лучший старт, если вы хотите минимальную зависимость от внешних runtime на машинах пользователей.\n- Если вы ещё выбираете между Go, Python и Node, начните с `/guide/what-you-can-build` и `/concepts/choosing-runtime`."
        : "- Open this area when you are building a production-oriented Go plugin.\n- This is the best starting point when you want the least downstream runtime friction.\n- If you are still choosing between Go, Python, and Node, start with `/guide/what-you-can-build` and `/concepts/choosing-runtime`.";
    pages.push({
      locale,
      relativePath: path.join(locale, "api", "go-sdk", "index.md"),
      content: renderMarkdownPage(
        {
          title: "Go SDK",
          description: "Generated Go SDK package reference",
          canonicalId: "page:api:go-sdk:index",
          surface: "go-sdk",
          section: "api",
          locale,
          generated: true,
          editLink: false,
          stability: "public-stable",
          maturity: "stable",
          sourceRef: "sdk",
          translationRequired: false
        },
        `# Go SDK\n\n${intro}\n\n${guidance}\n\n| Package | Summary |\n| --- | --- |\n${packageRows}`
      )
    });
  }

  return { entities, pages };
}

function localizeGoSDKBody(locale, pkgId, body) {
  if (locale !== "ru") {
    return body;
  }

  return body
    .replace(/^## Index$/gm, "## Оглавление")
    .replace("Package pluginkitai exposes the public root SDK for building plugin-kit-ai runtime binaries with typed Claude and Codex registrars.", "Пакет `pluginkitai` публикует корневой SDK для сборки runtime-бинарников plugin-kit-ai с типизированными регистраторами Claude и Codex.")
    .replace("Package pluginkitai exposes the public root SDK for building plugin\\-kit\\-ai runtime binaries with typed Claude and Codex registrars.", "Пакет `pluginkitai` публикует корневой SDK для сборки runtime-бинарников plugin-kit-ai с типизированными регистраторами Claude и Codex.")
    .replace("Package pluginkitai exposes the public root SDK for building plugin-kit-ai runtime binaries with typed Claude, Codex, and Gemini registrars.", "Пакет `pluginkitai` публикует корневой SDK для сборки runtime-бинарников plugin-kit-ai с типизированными регистраторами Claude, Codex и Gemini.")
    .replace("Package pluginkitai exposes the public root SDK for building plugin\\-kit\\-ai runtime binaries with typed Claude, Codex, and Gemini registrars.", "Пакет `pluginkitai` публикует корневой SDK для сборки runtime-бинарников plugin-kit-ai с типизированными регистраторами Claude, Codex и Gemini.")
    .replace("Package codex exposes typed public event inputs, responses, and registrars for Codex runtime integrations.", "Пакет `codex` публикует типизированные входные события, ответы и регистраторы для runtime-интеграций Codex.")
    .replace("Package codex exposes typed public event inputs, responses, and registrars for Codex plugin runtime integrations.", "Пакет `codex` публикует типизированные входные события, ответы и регистраторы для runtime-интеграций Codex.")
    .replace("Package claude exposes typed public hook inputs, responses, and registrars for Claude runtime integrations.", "Пакет `claude` публикует типизированные входные hooks, ответы и регистраторы для runtime-интеграций Claude.")
    .replace("Package claude exposes typed public hook inputs, responses, and registrars for Claude plugin runtime integrations.", "Пакет `claude` публикует типизированные входные hooks, ответы и регистраторы для runtime-интеграций Claude.")
    .replace("Package gemini exposes typed public Gemini hook inputs, responses, and registrars for the production-ready Gemini Go runtime lane, including the current 9-hook runtime surface.", "Пакет `gemini` публикует типизированные входные Gemini hooks, ответы и регистраторы для production-ready Go runtime-пути Gemini, включая текущую стабильную 9-hook surface.")
    .replace("Package gemini exposes typed public Gemini hook inputs, responses, and registrars for the production\\-ready Gemini Go runtime lane, including the current 9\\-hook runtime surface.", "Пакет `gemini` публикует типизированные входные Gemini hooks, ответы и регистраторы для production-ready Go runtime-пути Gemini, включая текущую стабильную 9-hook surface.")
    .replace("Package platformmeta exposes generated public metadata about supported target platforms, scaffolds, validation rules, and managed surfaces.", "Пакет `platformmeta` публикует сгенерированные публичные метаданные о поддерживаемых целевых платформах, scaffold-шаблонах, правилах валидации и управляемых поверхностях.")
    .replace("contains filtered or unexported fields", "содержит скрытые или неэкспортируемые поля")
    .replace("App owns middleware, handler registration, and invocation dispatch.", "App управляет middleware, регистрацией обработчиков и диспетчеризацией вызовов.")
    .replace("New builds an App with sane defaults for argv, process I/O, env, and logging.", "New создаёт `App` с разумными значениями по умолчанию для `argv`, process I/O, окружения и логирования.")
    .replace("Claude returns a registrar for Claude-specific hook handlers.", "Claude возвращает регистратор для Claude-специфичных hook-обработчиков.")
    .replace("Claude returns a registrar for Claude\\-specific hook handlers.", "Claude возвращает регистратор для Claude-специфичных hook-обработчиков.")
    .replace("Codex returns a registrar for Codex-specific event handlers.", "Codex возвращает регистратор для Codex-специфичных обработчиков событий.")
    .replace("Codex returns a registrar for Codex\\-specific event handlers.", "Codex возвращает регистратор для Codex-специфичных обработчиков событий.")
    .replace("Gemini returns a registrar for Gemini-specific hook handlers.", "Gemini возвращает регистратор для Gemini-специфичных hook-обработчиков.")
    .replace("Gemini returns a registrar for Gemini\\-specific hook handlers.", "Gemini возвращает регистратор для Gemini-специфичных hook-обработчиков.")
    .replace("Run dispatches the current process invocation with context.Background().", "Run обрабатывает текущий запуск процесса с `context.Background()`.")
    .replace("RunContext dispatches the current process invocation using the supplied context.", "RunContext обрабатывает текущий запуск процесса с переданным `context.Context`.");
}
