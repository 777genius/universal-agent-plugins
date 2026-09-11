import { extractPreparedCLI } from "./prepared-cli.mjs";
import { extractHistorical } from "./historical.mjs";

export async function extractCLI() {
  const historical = await extractHistorical(["cli"]);
  const prepared = await extractPreparedCLI();
  return { entities: [...historical.entities, ...prepared.entities],
    pages: [...historical.pages, ...prepared.pages] };
}

import fs from "node:fs/promises";
import path from "node:path";
import { docsToolsRoot, repoBrowserUrl, repoRoot } from "../config/site.mjs";
import { normalizeGeneratedMarkdown, renderMarkdownPage } from "../lib/frontmatter.mjs";
import { ensureDir, listMarkdownFiles } from "../lib/fs.mjs";
import { makeEntity, localeTitle } from "../lib/site-model.mjs";
import { run } from "../lib/process.mjs";

// Preserved legacy capability; deliberately outside the preparation graph.
export async function extractLegacyCLI() {
  const root = path.join(docsToolsRoot, "cli");
  const markdownDir = path.join(root, "markdown");
  const manifestPath = path.join(root, "manifest.json");
  await ensureDir(markdownDir);
  await run(
    "go",
    [
      "run",
      "./cli/plugin-kit-ai/cmd/plugin-kit-ai",
      "__docs",
      "export-cli",
      "--out-dir",
      markdownDir,
      "--manifest-path",
      manifestPath
    ],
    { cwd: repoRoot }
  );

  const manifest = JSON.parse(await fs.readFile(manifestPath, "utf8"));
  const markdownFiles = await listMarkdownFiles(markdownDir);
  const markdownMap = new Map();
  for (const filePath of markdownFiles) {
    markdownMap.set(path.basename(filePath), await fs.readFile(filePath, "utf8"));
  }

  const entities = [];
  const pages = [];
  const filenameToLink = new Map();
  for (const entry of manifest) {
    filenameToLink.set(entry.file_name, entry.slug);
  }

  for (const entry of manifest) {
    const body = normalizeGeneratedMarkdown(
      rewriteLinks(markdownMap.get(entry.file_name) || "", filenameToLink)
    );
    const canonicalId = `command:${entry.command_path.toLowerCase().replaceAll(" ", ":")}`;
    entities.push(
      makeEntity({
        canonicalId,
        kind: "command",
        surface: "cli",
        localeStrategy: "mirrored",
        title: entry.command_path,
        summary: entry.short || entry.long || "",
        stability: entry.deprecated ? "public-beta" : "public-stable",
        maturity: entry.deprecated ? "deprecated" : "stable",
        sourceKind: "cobra-doc",
        sourceRef: `cli:${entry.command_path}`,
        pathEn: `/en/api/cli/${entry.slug}`,
        pathRu: `/ru/api/cli/${entry.slug}`,
        searchTerms: [entry.command_path, ...(entry.aliases || [])]
      })
    );
    for (const locale of ["en", "ru"]) {
      const intro =
        locale === "ru"
          ? "Сгенерировано из реального Cobra command tree."
          : "Generated from the live Cobra command tree.";
      const synopsis = renderCommandSynopsis(entry, body, locale);
      const localizedBody = localizeCliMarkdown(locale, body);
      pages.push({
        locale,
        relativePath: path.join(locale, "api", "cli", `${entry.slug}.md`),
        content: renderMarkdownPage(
          {
            title: localeTitle(locale, entry.command_path, entry.command_path),
            description: localizeCommandShort(entry.short || entry.long || entry.command_path, entry.command_path, locale),
            canonicalId,
            surface: "cli",
            section: "api",
            locale,
            generated: true,
            editLink: false,
            stability: entry.deprecated ? "public-beta" : "public-stable",
            maturity: entry.deprecated ? "deprecated" : "stable",
            sourceRef: `cli:${entry.command_path}`,
            translationRequired: false
          },
          `<DocMetaCard surface="cli" stability="${entry.deprecated ? "public-beta" : "public-stable"}" maturity="${entry.deprecated ? "deprecated" : "stable"}" source-ref="cli:${entry.command_path}" source-href="${repoBrowserUrl(`cli:${entry.command_path}`)}" />\n\n# ${entry.command_path}\n\n${intro}\n\n${synopsis}${localizedBody}`
        )
      });
    }
  }

  for (const locale of ["en", "ru"]) {
    const heading = locale === "ru" ? "Справочник CLI" : "CLI Reference";
    const coreCommands = manifest.filter((entry) => !["bundle", "completion", "skills"].includes(entry.command_path.split(" ")[1]));
    const grouped = {
      core: coreCommands,
      bundle: manifest.filter((entry) => entry.command_path.split(" ")[1] === "bundle"),
      completion: manifest.filter((entry) => entry.command_path.split(" ")[1] === "completion"),
      skills: manifest.filter((entry) => entry.command_path.split(" ")[1] === "skills")
    };
    const renderList = (entries) =>
      entries
        .map((entry) => `- [\`${entry.command_path}\`](/${locale}/api/cli/${entry.slug})`)
        .join("\n");
    const summary =
      locale === "ru"
        ? "CLI покрывает создание проекта, генерацию, проверку, тесты, импорт и установку."
        : "The CLI covers project creation, generate, validation, testing, import, and install flows.";
    const guidance =
      locale === "ru"
        ? [
            "Используйте `init`, чтобы создать новый репозиторий плагина.",
            "Используйте `generate` и `validate --strict` как основной цикл проверки.",
            "Используйте bundle-команды только для переносимых Python и Node bundle-артефактов."
          ]
        : [
            "Use `init` to create a new plugin repo.",
            "Use `generate` and `validate --strict` as the primary verification loop.",
            "Use bundle commands only for portable handoff of Python or Node runtime bundles."
          ];
    const sections = [
      locale === "ru" ? "## Основные команды" : "## Core Commands",
      renderList(grouped.core),
      locale === "ru" ? "## Bundle" : "## Bundle",
      renderList(grouped.bundle),
      locale === "ru" ? "## Completion" : "## Completion",
      renderList(grouped.completion),
      locale === "ru" ? "## Skills" : "## Skills",
      renderList(grouped.skills)
    ]
      .filter(Boolean)
      .join("\n");
    pages.push({
      locale,
      relativePath: path.join(locale, "api", "cli", "index.md"),
      content: renderMarkdownPage(
        {
          title: heading,
          description: "Generated CLI reference",
          canonicalId: "page:api:cli:index",
          surface: "cli",
          section: "api",
          locale,
          generated: true,
          editLink: false,
          stability: "public-stable",
          maturity: "stable",
          sourceRef: "cli/plugin-kit-ai",
          translationRequired: false
        },
        `# ${heading}\n\n${summary}\n\n${guidance.map((line) => `- ${line}`).join("\n")}\n\n${sections}`
      )
    });
  }

  return { entities, pages };
}

function rewriteLinks(body, filenameToLink) {
  return body.replace(/\(([^)]+)\.md\)/g, (_full, target) => {
    const link = filenameToLink.get(`${target}.md`);
    if (!link) {
      return `(${target}.md)`;
    }
    return `(${link})`;
  });
}

function renderCommandSynopsis(entry, body, locale) {
  const parts = [];
  const short = normalizeSynopsisLine(entry.short);

  if (short) {
    parts.push(localizeCommandShort(short, entry.command_path, locale));
  }
  if (parts.length === 0 && isHelpCommand(entry.command_path)) {
    parts.push(
      locale === "ru"
        ? `Справка по \`${parentCommandPath(entry.command_path)}\` и его подкомандам.`
        : `Help for \`${parentCommandPath(entry.command_path)}\` and its subcommands.`
    );
  }
  if (parts.length === 0 && isThinCommandBody(body)) {
    parts.push(
      locale === "ru"
        ? `Справочная страница для \`${entry.command_path}\`.`
        : `Reference page for \`${entry.command_path}\`.`
    );
  }
  if (parts.length === 0) {
    return "";
  }
  return `${parts.join("\n\n")}\n\n`;
}

function localizeCommandShort(short, commandPath, locale) {
  if (locale !== "ru") {
    return short;
  }
  const translations = new Map([
    ["plugin-kit-ai CLI - scaffold and tooling for AI plugins", "CLI plugin-kit-ai для создания проектов и служебных операций вокруг AI-плагинов."],
    ["Bootstrap repo-local interpreted runtime dependencies", "Подготавливает зависимости локального интерпретируемого runtime в репозитории."],
    ["Inspect repo-local runtime readiness without mutating files", "Проверяет готовность локального runtime в репозитории без изменения файлов."],
    ["Bundle tooling for exported interpreted-runtime handoff archives", "Инструменты bundle-экспорта для переносимых архивов интерпретируемого runtime."],
    ["Fetch and install a remote exported Python/Node bundle into a destination directory", "Загружает и устанавливает удалённый экспортированный Python/Node bundle в целевой каталог."],
    ["Install a local exported Python/Node bundle into a destination directory", "Устанавливает локальный экспортированный Python/Node bundle в целевой каталог."],
    ["Publish an exported Python/Node bundle to GitHub Releases", "Публикует экспортированный Python/Node bundle в GitHub Releases."],
    ["Create a plugin-kit-ai package scaffold", "Создаёт каркас проекта plugin-kit-ai."],
    ["Generate the autocompletion script for the specified shell", "Генерирует скрипт автодополнения для указанной оболочки."],
    ["Print plugin-kit-ai CLI module version (from build info)", "Печатает версию модуля CLI plugin-kit-ai из build info."],
    ["Validate a package-standard plugin-kit-ai project", "Проверяет проект plugin-kit-ai в package-standard формате."],
    ["Compile native target artifacts from the package graph", "Собирает нативные артефакты целевых платформ из package graph."],
    ["Compile native target artifacts from the package graph discovered via canonical plugin/plugin.yaml plus the standard authored directories.", "Собирает нативные артефакты целевых платформ из package graph, используя канонический `plugin/plugin.yaml` и стандартные authored directories."],
    [". discovered via canonical plugin/plugin.yaml plus the standard authored directories.", ", используя канонический `plugin/plugin.yaml` и стандартные authored directories."],
    ["Normalize package-standard plugin.yaml", "Нормализует `plugin.yaml` в package-standard проекте."],
    ["Import current native target artifacts into the package standard layout", "Импортирует текущие нативные артефакты в package-standard структуру."],
    ["Install a plugin binary from GitHub Releases (verified via checksums.txt)", "Устанавливает бинарник плагина из GitHub Releases с проверкой через `checksums.txt`."],
    ["Experimental skill authoring tools", "Экспериментальные инструменты для авторинга skills."],
    ["Watch the project, re-generate, re-validate, rebuild when needed, and rerun fixtures", "Следит за проектом, повторно рендерит, валидирует, пересобирает и перезапускает фикстуры при изменениях."],
    ["Create a portable interpreted-runtime bundle without changing install semantics", "Создаёт переносимый bundle интерпретируемого runtime без смены install-семантики."],
    ["Create a deterministic portable .tar.gz bundle for launcher-based interpreted runtime projects.", "Создаёт детерминированный переносимый `.tar.gz` bundle для launcher-based проектов с интерпретируемым runtime."],
    ["This beta surface is a bounded handoff/export flow for python, node, and shell runtime repos.", "Эта beta-поверхность покрывает ограниченный handoff/export сценарий для runtime-репозиториев на `python`, `node` и `shell`."],
    ["It does not extend plugin-kit-ai install, and it does not imply marketplace packaging or dependency-preinstalled installs.", "Она не расширяет сценарий `plugin-kit-ai install` и не подразумевает packaging для marketplace или поставку с уже предустановленными зависимостями."],
    ["Create a deterministic portable .tar.gz bundle for launcher-based interpreted runtime projects.\n\nThis beta surface is a bounded handoff/export flow for python, node, and shell runtime repos.\nIt does not extend plugin-kit-ai install, and it does not imply marketplace packaging or dependency-preinstalled installs.", "Создаёт детерминированный переносимый `.tar.gz` bundle для launcher-based проектов с интерпретируемым runtime.\n\nЭта beta-поверхность покрывает ограниченный handoff/export сценарий для runtime-репозиториев на `python`, `node` и `shell`.\nОна не расширяет сценарий `plugin-kit-ai install` и не подразумевает packaging для marketplace или поставку с уже предустановленными зависимостями."],
    ["      --output string     write bundle to this .tar.gz path (default: <root>/<name>_<platform>_<runtime>_bundle.tar.gz)", "      --output string     записывает bundle в указанный путь `.tar.gz` (по умолчанию: `<root>/<name>_<platform>_<runtime>_bundle.tar.gz`)"],
    ["      --output string     write bundle to this .tar.gz path (default: &lt;root&gt;/&lt;name&gt;_&lt;platform&gt;_&lt;runtime&gt;_bundle.tar.gz)", "      --output string     записывает bundle в указанный путь `.tar.gz` (по умолчанию: `&lt;root&gt;/&lt;name&gt;_&lt;platform&gt;_&lt;runtime&gt;_bundle.tar.gz`)"],
    ["      --platform string   target override (\"codex-runtime\" or \"claude\")", "      --platform string   переопределяет целевую платформу (`codex-runtime` или `claude`)"],
    ["Downloads checksums.txt and a release asset for your GOOS/GOARCH, verifies SHA256, and writes the binary to --dir\n(default bin). Asset selection: (1) a single *_<goos>_<goarch>.tar.gz (GoReleaser) — file extracted from archive root;\nor (2) a raw binary named *-<goos>-<goarch> or *-<goos>-<goarch>.exe on Windows (e.g. claude-notifications-darwin-arm64).", "Скачивает `checksums.txt` и release-артефакт для ваших `GOOS/GOARCH`, проверяет `SHA256` и записывает бинарник в `--dir`\n(по умолчанию `bin`). Выбор артефакта: (1) один `*_&lt;goos&gt;_&lt;goarch&gt;.tar.gz` от GoReleaser с извлечением файла из корня архива;\nили (2) сырой бинарник с именем `*-&lt;goos&gt;-&lt;goarch&gt;` либо `*-&lt;goos&gt;-&lt;goarch&gt;.exe` на Windows (например, `claude-notifications-darwin-arm64`)."],
    ["Downloads checksums.txt and a release asset for your GOOS/GOARCH, verifies SHA256, and writes the binary to --dir\n(default bin). Asset selection: (1) a single *_&lt;goos&gt;_&lt;goarch&gt;.tar.gz (GoReleaser) — file extracted from archive root;\nor (2) a raw binary named *-&lt;goos&gt;-&lt;goarch&gt; or *-&lt;goos&gt;-&lt;goarch&gt;.exe on Windows (e.g. claude-notifications-darwin-arm64).", "Скачивает `checksums.txt` и release-артефакт для ваших `GOOS/GOARCH`, проверяет `SHA256` и записывает бинарник в `--dir`\n(по умолчанию `bin`). Выбор артефакта: (1) один `*_&lt;goos&gt;_&lt;goarch&gt;.tar.gz` от GoReleaser с извлечением файла из корня архива;\nили (2) сырой бинарник с именем `*-&lt;goos&gt;-&lt;goarch&gt;` либо `*-&lt;goos&gt;-&lt;goarch&gt;.exe` на Windows (например, `claude-notifications-darwin-arm64`)."],
    ["Use exactly one of --tag or --latest. Draft releases are refused; prerelease requires --pre.\nOptional --output-name sets the installed filename (single path segment).", "Используйте ровно один из флагов `--tag` или `--latest`. Draft-релизы не принимаются; для prerelease нужен `--pre`.\nНеобязательный `--output-name` задаёт имя устанавливаемого файла (один сегмент пути)."],
    ["This command installs third-party plugin binaries, not the plugin-kit-ai CLI itself (build plugin-kit-ai from source or use a release installer).", "Эта команда устанавливает сторонние бинарники плагинов, а не сам CLI `plugin-kit-ai` (собирайте `plugin-kit-ai` из исходников или используйте installer для релизов)."],
    ["Downloads checksums.txt and a release asset for your GOOS/GOARCH, verifies SHA256, and writes the binary to --dir\n(default bin). Asset selection: (1) a single *_&lt;goos&gt;_&lt;goarch&gt;.tar.gz (GoReleaser) — file extracted from archive root;\nor (2) a raw binary named *-&lt;goos&gt;-&lt;goarch&gt; or *-&lt;goos&gt;-&lt;goarch&gt;.exe on Windows (e.g. claude-notifications-darwin-arm64).\n\nUse exactly one of --tag or --latest. Draft releases are refused; prerelease requires --pre.\nOptional --output-name sets the installed filename (single path segment).\n\nThis command installs third-party plugin binaries, not the plugin-kit-ai CLI itself (build plugin-kit-ai from source or use a release installer).", "Скачивает `checksums.txt` и release-артефакт для ваших `GOOS/GOARCH`, проверяет `SHA256` и записывает бинарник в `--dir`\n(по умолчанию `bin`). Выбор артефакта: (1) один `*_&lt;goos&gt;_&lt;goarch&gt;.tar.gz` от GoReleaser с извлечением файла из корня архива;\nили (2) сырой бинарник с именем `*-&lt;goos&gt;-&lt;goarch&gt;` либо `*-&lt;goos&gt;-&lt;goarch&gt;.exe` на Windows (например, `claude-notifications-darwin-arm64`).\n\nИспользуйте ровно один из флагов `--tag` или `--latest`. Draft-релизы не принимаются; для prerelease нужен `--pre`.\nНеобязательный `--output-name` задаёт имя устанавливаемого файла (один сегмент пути).\n\nЭта команда устанавливает сторонние бинарники плагинов, а не сам CLI `plugin-kit-ai` (собирайте `plugin-kit-ai` из исходников или используйте installer для релизов)."],
    ["      --dir string            directory for the installed binary (created if missing) (default \"bin\")", "      --dir string            каталог для установленного бинарника (создаётся при отсутствии) (по умолчанию `bin`)"],
    ["  -f, --force                 overwrite existing binary", "  -f, --force                 перезаписывает существующий бинарник"],
    ["      --goarch string         target GOARCH override (default: host GOARCH)", "      --goarch string         переопределяет целевой `GOARCH` (по умолчанию: `GOARCH` хоста)"],
    ["      --goos string           target GOOS override (default: host GOOS)", "      --goos string           переопределяет целевой `GOOS` (по умолчанию: `GOOS` хоста)"],
    ["      --latest                install from GitHub releases/latest (non-prerelease) instead of --tag", "      --latest                устанавливает из `GitHub releases/latest` (без prerelease) вместо `--tag`"],
    ["      --output-name string    write binary under this filename in --dir (default: name from archive)", "      --output-name string    записывает бинарник под этим именем в `--dir` (по умолчанию: имя из архива)"],
    ["      --tag string            Git release tag (required unless --latest), e.g. v0.1.0", "      --tag string            Git release tag (обязателен, если не указан `--latest`), например `v0.1.0`"],
    ["      --dir string            directory for the installed binary (created if missing) (default \"bin\")", "      --dir string            каталог для установленного бинарника (создаётся при отсутствии) (по умолчанию `bin`)"],
    ["  -f, --force                 overwrite existing binary", "  -f, --force                 перезаписывает существующий бинарник"],
    ["      --goarch string         target GOARCH override (default: host GOARCH)", "      --goarch string         переопределяет целевой `GOARCH` (по умолчанию: `GOARCH` хоста)"],
    ["      --goos string           target GOOS override (default: host GOOS)", "      --goos string           переопределяет целевой `GOOS` (по умолчанию: `GOOS` хоста)"],
    ["      --latest                install from GitHub releases/latest (non-prerelease) instead of --tag", "      --latest                устанавливает из `GitHub releases/latest` (без prerelease) вместо `--tag`"],
    ["      --output-name string    write binary under this filename in --dir (default: name from archive)", "      --output-name string    записывает бинарник под этим именем в `--dir` (по умолчанию: имя из архива)"],
    ["      --output string     write bundle to this .tar.gz path (default: &lt;root&gt;/&lt;name&gt;_&lt;platform&gt;_&lt;runtime&gt;_bundle.tar.gz)", "      --output string     записывает bundle в путь `.tar.gz` (по умолчанию: `&lt;root&gt;/&lt;name&gt;_&lt;platform&gt;_&lt;runtime&gt;_bundle.tar.gz`)"],
    ["      --platform string   target override (\"codex-runtime\" or \"claude\")", "      --platform string   переопределяет целевую платформу (`codex-runtime` или `claude`)"],
    ["Run stable fixture-driven smoke tests against the launcher entrypoint", "Запускает стабильные smoke-тесты на фикстурах против launcher entrypoint."],
    ["Inspect the discovered package graph and target coverage", "Показывает найденный package graph и покрытие целевых платформ."],
    ["Show generated target/package or runtime support metadata", "Показывает сгенерированные metadata по целям, пакетам и поддержке runtime."]
  ]);
  if (short === "Help about any command") {
    return `Справка по \`${parentCommandPath(commandPath)}\` и его подкомандам.`;
  }
  return translations.get(short) || short;
}

function normalizeSynopsisLine(value) {
  const text = String(value || "").trim();
  return text ? escapeSynopsisText(text) : "";
}

function normalizeLongText(value) {
  const text = String(value || "").trim();
  return text ? escapeSynopsisText(text) : "";
}

function isHelpCommand(commandPath) {
  return commandPath.split(" ").at(-1) === "help";
}

function parentCommandPath(commandPath) {
  const parts = commandPath.split(" ");
  return parts.slice(0, -1).join(" ") || commandPath;
}

function isThinCommandBody(body) {
  const headings = body.match(/^## /gm) || [];
  return headings.length === 0;
}

function escapeSynopsisText(text) {
  return text.replaceAll("&", "&amp;").replaceAll("<", "&lt;").replaceAll(">", "&gt;");
}

function localizeCliMarkdown(locale, body) {
  if (locale !== "ru") {
    return body;
  }

  const replacements = [
    ["### Synopsis", "### Описание"],
    ["### Options", "### Опции"],
    ["### SEE ALSO", "### См. также"],
    ["help for ", "справка по "],
    ["Bootstrap repo-local interpreted runtime dependencies for package-standard projects.", "Подготавливает зависимости локального интерпретируемого runtime для package-standard проектов."],
    ["plugin-kit-ai CLI - scaffold and tooling for AI plugins", "CLI plugin-kit-ai для создания проектов и служебных операций вокруг AI-плагинов."],
    ["Bootstrap repo-local interpreted runtime dependencies", "Подготавливает зависимости локального интерпретируемого runtime в репозитории."],
    ["This helper is bounded to repo-local launcher-based lanes. It does not replace ecosystem package managers or the binary-only install flow.", "Эта команда рассчитана на сценарии с локальным launcher runtime в репозитории. Она не заменяет штатные пакетные менеджеры экосистемы и binary-only установку."],
    ["Inspect repo-local runtime readiness without mutating files", "Проверяет готовность локального runtime в репозитории без изменения файлов."],
    ["Read-only readiness check for package-standard projects. Reports lane, runtime, detected manager, status, and next commands.", "Проверка готовности package-standard проекта в read-only режиме. Показывает lane, runtime, обнаруженный менеджер, статус и следующие команды."],
    ["Bundle tooling for exported interpreted-runtime handoff archives", "Инструменты bundle-экспорта для переносимых архивов интерпретируемого runtime."],
    ["Fetch and install a remote exported Python/Node bundle into a destination directory", "Загружает и устанавливает удалённый экспортированный Python/Node bundle в целевой каталог."],
    ["Install a local exported Python/Node bundle into a destination directory", "Устанавливает локальный экспортированный Python/Node bundle в целевой каталог."],
    ["Publish an exported Python/Node bundle to GitHub Releases", "Публикует экспортированный Python/Node bundle в GitHub Releases."],
    ["Create a plugin-kit-ai package scaffold", "Создаёт каркас проекта plugin-kit-ai."],
    ["Creates a package-standard plugin-kit-ai project scaffold.", "Создаёт каркас plugin-kit-ai проекта в package-standard формате."],
    ["Choose the lane that matches your goal:", "Выберите lane, который соответствует вашей цели:"],
    ["Fast local plugin:", "Быстрый локальный плагин:"],
    ["Production-ready plugin repo:", "Production-ready репозиторий плагина:"],
    ["Already have native config:", "Уже есть нативная конфигурация:"],
    ["Public flags:", "Публичные флаги:"],
    ["Use --runtime python or --runtime node when repo-local iteration matters more than packaged distribution.", "Используйте `--runtime python` или `--runtime node`, когда локальная итерация в репозитории важнее, чем пакетная поставка."],
    ["These are supported executable-runtime paths, not equal production paths.", "Это поддерживаемые пути для executable runtime, но не равноценные production-пути."],
    ["Plain init keeps the strongest supported runtime path. --runtime go remains the default, and --platform codex-runtime remains the default target.", "Обычный `init` оставляет наиболее надёжный поддерживаемый runtime-путь. `--runtime go` остаётся значением по умолчанию, а `--platform codex-runtime` остаётся целевой платформой по умолчанию."],
    ["Use --platform claude for Claude hooks, and add --claude-extended-hooks only when you intentionally want the wider runtime-supported subset.", "Используйте `--platform claude` для Claude hooks, а `--claude-extended-hooks` добавляйте только когда осознанно нужен более широкий runtime-поддерживаемый набор."],
    ["Use --platform codex-package for the official Codex plugin bundle without local notify/runtime wiring.", "Используйте `--platform codex-package` для официального Codex plugin bundle без локальной `notify`/runtime-обвязки."],
    ["Use --platform opencode for the OpenCode workspace-config lane without launcher/runtime scaffolding.", "Используйте `--platform opencode` для OpenCode workspace-config lane без launcher/runtime scaffold."],
    ["Use --platform cursor for the Cursor workspace-config lane without launcher/runtime scaffolding.", "Используйте `--platform cursor` для Cursor workspace-config lane без launcher/runtime scaffold."],
    ["Use plugin-kit-ai import to bring current Claude/Codex/Gemini/OpenCode/Cursor native files into the package-standard authored layout.", "Используйте `plugin-kit-ai import`, чтобы привести текущие нативные файлы Claude/Codex/Gemini/OpenCode/Cursor к authored layout package-standard проекта."],
    ["init is for creating a new package-standard project, not for preserving native files as the authored source of truth.", "`init` нужен для создания нового package-standard проекта, а не для сохранения нативных файлов как основного authored source of truth."],
    ["  --platform   Supported: \"codex-runtime\" (default), \"codex-package\", \"claude\", \"gemini\", \"opencode\", and \"cursor\".", "  --platform   Поддерживаются: `codex-runtime` (по умолчанию), `codex-package`, `claude`, `gemini`, `opencode` и `cursor`."],
    ["  --runtime    Supported: \"go\" (default), \"python\", \"node\", \"shell\" for launcher-based targets only.", "  --runtime    Поддерживаются: `go` (по умолчанию), `python`, `node`, `shell`; `shell` доступен только для launcher-based targets."],
    ["  --typescript Generate a TypeScript scaffold on top of the node runtime lane (requires --runtime node).", "  --typescript Генерирует TypeScript scaffold поверх node runtime lane (требует `--runtime node`)."],
    ["               For --runtime python or --runtime node, import the shared plugin-kit-ai-runtime package instead of vendoring the helper file into src/.", "               Для `--runtime python` или `--runtime node` импортирует общий пакет `plugin-kit-ai-runtime` вместо вендоринга helper-файла в `src/`."],
    ["               Pin the generated plugin-kit-ai-runtime dependency version. Required on development builds; released CLIs default to their own stable tag.", "               Фиксирует версию зависимости `plugin-kit-ai-runtime`. Обязательно для development build; выпущенные CLI по умолчанию используют собственный stable tag."],
    ["  -o, --output Target directory (default: ./<project-name>).", "  -o, --output Целевой каталог (по умолчанию: `./<project-name>`)."],
    ["  -o, --output Target directory (default: ./&lt;project-name&gt;).", "  -o, --output Целевой каталог (по умолчанию: `./&lt;project-name&gt;`)."],
    ["  -f, --force  Allow writing into a non-empty directory and overwrite generated files.", "  -f, --force  Разрешает запись в непустой каталог и перезапись сгенерированных файлов."],
    ["  --extras     Also emit optional release helpers such as Makefile, .goreleaser.yml, portable skills/, and stable Python/Node bundle-release workflow scaffolding where supported.", "  --extras     Дополнительно генерирует optional release helpers, например `Makefile`, `.goreleaser.yml`, переносимые `skills/` и scaffold для stable Python/Node bundle-release workflow там, где это поддерживается."],
    ["               For --platform claude, scaffold the full runtime-supported hook set instead of the stable default subset.", "               Для `--platform claude` генерирует полный runtime-поддерживаемый набор hooks вместо стабильного поднабора по умолчанию."],
    ["      --claude-extended-hooks            for --platform claude, scaffold the full runtime-supported hook set instead of the stable default subset", "      --claude-extended-hooks            для `--platform claude` генерирует полный runtime-поддерживаемый набор hooks вместо стабильного поднабора по умолчанию"],
    ["      --extras                           include optional scaffold files (runtime-dependent extras plus skills and commands)", "      --extras                           включает optional scaffold-файлы (runtime-зависимые extras, а также skills и команды)"],
    ["  -f, --force                            overwrite generated files; allow non-empty output directory", "  -f, --force                            перезаписывает сгенерированные файлы и разрешает непустой output-каталог"],
    ["  -o, --output string                    output directory (default: ./<project-name>)", "  -o, --output string                    output-каталог (по умолчанию: `./<project-name>`)"],
    ["      --platform string                  target lane (\"codex-runtime\", \"codex-package\", \"claude\", \"gemini\", \"opencode\", or \"cursor\") (default \"codex-runtime\")", "      --platform string                  целевой lane (`codex-runtime`, `codex-package`, `claude`, `gemini`, `opencode` или `cursor`) (по умолчанию `codex-runtime`)"],
    ["      --runtime string                   runtime (\"go\", \"python\", \"node\", or \"shell\") (default \"go\")", "      --runtime string                   runtime (`go`, `python`, `node` или `shell`) (по умолчанию `go`)"],
    ["      --runtime-package                  for --runtime python or --runtime node, import the shared plugin-kit-ai-runtime package instead of vendoring the helper file", "      --runtime-package                  для `--runtime python` или `--runtime node` импортирует общий пакет `plugin-kit-ai-runtime` вместо вендоринга helper-файла"],
    ["      --runtime-package-version string   pin the generated plugin-kit-ai-runtime dependency version", "      --runtime-package-version string   фиксирует версию сгенерированной зависимости `plugin-kit-ai-runtime`"],
    ["      --typescript                       generate a TypeScript scaffold on top of the node runtime lane", "      --typescript                       генерирует TypeScript scaffold поверх node runtime lane"],
    ["Install a local .tar.gz bundle created by plugin-kit-ai export into a destination directory.", "Устанавливает локальный `.tar.gz` bundle, созданный через `plugin-kit-ai export`, в целевой каталог."],
    ["This stable local handoff surface only supports local exported Python/Node bundles for codex-runtime or claude.", "Эта стабильная handoff-поверхность поддерживает только локальные экспортированные Python/Node bundle для `codex-runtime` или `claude`."],
    ["It unpacks bundle contents safely, prints next steps, and does not extend the binary-only plugin-kit-ai install flow.", "Команда безопасно распаковывает содержимое bundle, печатает следующие шаги и не расширяет binary-only сценарий установки `plugin-kit-ai install`."],
    ["Fetch a remote exported Python/Node bundle and install it into a destination directory.", "Загружает удалённый экспортированный Python/Node bundle и устанавливает его в целевой каталог."],
    ["Use either a direct HTTPS bundle URL with --url or a GitHub release reference as owner/repo plus --tag or --latest.", "Используйте либо прямой HTTPS URL bundle через `--url`, либо ссылку на GitHub release в формате `owner/repo` вместе с `--tag` или `--latest`."],
    ["This stable remote handoff surface is intentionally separate from the binary-only plugin-kit-ai install flow.", "Эта стабильная remote handoff-поверхность намеренно отделена от binary-only сценария `plugin-kit-ai install`."],
    ["This stable producer-side handoff surface exports a bundle, creates a published release by default,\nuses --draft to keep the release as draft, uploads the bundle plus a sibling .sha256 asset,\nand remains separate from the binary-only plugin-kit-ai install flow.", "Эта стабильная producer-side handoff-поверхность экспортирует bundle, по умолчанию создаёт опубликованный release,\nиспользует `--draft`, если релиз нужно оставить черновиком, загружает сам bundle и соседний `.sha256`-asset,\nи остаётся отдельной от binary-only сценария `plugin-kit-ai install`."],
    ["Публикует экспортированный Python/Node bundle в GitHub Releases..", "Публикует экспортированный Python/Node bundle в GitHub Releases."],
    ["      --dest string   destination directory for unpacked bundle contents", "      --dest string   целевой каталог для распакованного содержимого bundle"],
    ["      --dest string              destination directory for unpacked bundle contents", "      --dest string              целевой каталог для распакованного содержимого bundle"],
    ["  -f, --force         overwrite an existing destination directory", "  -f, --force         перезаписывает существующий целевой каталог"],
    ["  -f, --force                    overwrite an existing destination directory", "  -f, --force                    перезаписывает существующий целевой каталог"],
    ["  -f, --force                 replace existing bundle assets with the same name", "  -f, --force                 заменяет существующие bundle-артефакты с тем же именем"],
    ["      --draft                 keep the target release as draft instead of published", "      --draft                 оставляет целевой release черновиком вместо публикации"],
    ["      --platform string       target platform to export and publish (codex-runtime or claude)", "      --platform string       целевая платформа для экспорта и публикации (`codex-runtime` или `claude`)"],
    ["      --repo string           GitHub owner/repo that will receive the bundle assets", "      --repo string           GitHub owner/repo, куда будут загружены bundle-артефакты"],
    ["      --tag string            GitHub release tag to reuse or create", "      --tag string            GitHub release tag, который нужно переиспользовать или создать"],
    ["      --asset-name string        specific GitHub release bundle asset name to install", "      --asset-name string        конкретное имя bundle-asset в GitHub release для установки"],
    ["      --github-api-base string   GitHub API base URL override (for tests or GitHub Enterprise)", "      --github-api-base string   переопределение базового URL GitHub API (для тестов или GitHub Enterprise)"],
    ["      --github-token string      GitHub token (optional; default from GITHUB_TOKEN env)", "      --github-token string      GitHub token (необязательно; по умолчанию берётся из `GITHUB_TOKEN`)"],
    ["      --github-token string   GitHub token (optional; default from GITHUB_TOKEN env)", "      --github-token string   GitHub token (необязательно; по умолчанию берётся из `GITHUB_TOKEN`)"],
    ["      --latest                   install from the latest GitHub release instead of --tag", "      --latest                   устанавливает bundle из последнего GitHub release вместо `--tag`"],
    ["      --platform string          bundle platform hint for GitHub mode (codex-runtime or claude)", "      --platform string          подсказка по платформе bundle для GitHub-режима (`codex-runtime` или `claude`)"],
    ["      --runtime string           bundle runtime hint for GitHub mode (python or node)", "      --runtime string           подсказка по runtime bundle для GitHub-режима (`python` или `node`)"],
    ["      --sha256 string            expected SHA256 for URL mode; overrides .sha256 sidecar lookup", "      --sha256 string            ожидаемый SHA256 для URL-режима; переопределяет поиск соседнего `.sha256` файла"],
    ["      --tag string               GitHub release tag for bundle selection", "      --tag string               GitHub release tag для выбора bundle"],
    ["      --url string               direct HTTPS URL to an exported .tar.gz bundle", "      --url string               прямой HTTPS URL к экспортированному `.tar.gz` bundle"],
    ["      --pre                   allow GitHub prerelease (non-stable) releases", "      --pre                   разрешает GitHub prerelease-релизы (не stable)"],
    ["Generate the autocompletion script for the specified shell", "Генерирует скрипт автодополнения для указанной оболочки."],
    ["Print plugin-kit-ai CLI module version (from build info)", "Печатает версию модуля CLI plugin-kit-ai из build info."],
    ["Validate a package-standard plugin-kit-ai project", "Проверяет проект plugin-kit-ai в package-standard формате."],
    ["Compile native target artifacts from the package graph", "Собирает нативные артефакты целевых платформ из package graph."],
    ["Compile native target artifacts from the package graph discovered via canonical plugin/plugin.yaml plus the standard authored directories.", "Собирает нативные артефакты целевых платформ из package graph, используя канонический `plugin/plugin.yaml` и стандартные authored directories."],
    [". discovered via canonical plugin/plugin.yaml plus the standard authored directories.", ", используя канонический `plugin/plugin.yaml` и стандартные authored directories."],
    ["Normalize package-standard plugin.yaml", "Нормализует `plugin.yaml` в package-standard проекте."],
    ["Import current native target artifacts into the package standard layout", "Импортирует текущие нативные артефакты в package-standard структуру."],
    ["Install a plugin binary from GitHub Releases (verified via checksums.txt)", "Устанавливает бинарник плагина из GitHub Releases с проверкой через `checksums.txt`."],
    ["Experimental skill authoring tools", "Экспериментальные инструменты для авторинга skills."],
    ["Watch the project, re-generate, re-validate, rebuild when needed, and rerun fixtures", "Следит за проектом, повторно рендерит, валидирует, пересобирает и перезапускает фикстуры при изменениях."],
    ["Create a portable interpreted-runtime bundle without changing install semantics", "Создаёт переносимый bundle интерпретируемого runtime без смены install-семантики."],
    ["Run stable fixture-driven smoke tests against the launcher entrypoint", "Запускает стабильные smoke-тесты на фикстурах против launcher entrypoint."],
    ["Inspect the discovered package graph and target coverage", "Показывает найденный package graph и покрытие целевых платформ."],
    ["Show generated target/package or runtime support metadata", "Показывает сгенерированные metadata по целям, пакетам и поддержке runtime."]
  ];

  let text = replacements.reduce((current, [from, to]) => current.replaceAll(from, to), body);

  text = text
    .replace(
      /Create a deterministic portable \.tar\.gz bundle for launcher-based interpreted runtime projects\.\n\nThis beta surface is a bounded handoff\/export flow for python, node, and shell runtime repos\.\nIt does not extend plugin-kit-ai install, and it does not imply marketplace packaging or dependency-preinstalled installs\./g,
      "Создаёт детерминированный переносимый `.tar.gz` bundle для launcher-based проектов с интерпретируемым runtime.\n\nЭта beta-поверхность покрывает ограниченный handoff/export сценарий для runtime-репозиториев на `python`, `node` и `shell`.\nОна не расширяет сценарий `plugin-kit-ai install` и не подразумевает packaging для marketplace или поставку с уже предустановленными зависимостями."
    )
    .replace(
      /Downloads checksums\.txt and a release asset for your GOOS\/GOARCH, verifies SHA256, and writes the binary to --dir\n\(default bin\)\. Asset selection: \(1\) a single \*_[^ ]+\.tar\.gz \(GoReleaser\) — file extracted from archive root;\nor \(2\) a raw binary named \*-[^\n]+\n\nUse exactly one of --tag or --latest\. Draft releases are refused; prerelease requires --pre\.\nOptional --output-name sets the installed filename \(single path segment\)\.\n\nThis command installs third-party plugin binaries, not the plugin-kit-ai CLI itself \(build plugin-kit-ai from source or use a release installer\)\./g,
      "Скачивает `checksums.txt` и release-артефакт для ваших `GOOS/GOARCH`, проверяет `SHA256` и записывает бинарник в `--dir`.\nПо умолчанию используется каталог `bin`. Выбор артефакта такой: (1) один архив GoReleaser `*_GOOS_GOARCH.tar.gz` с извлечением файла из корня архива; или (2) сырой бинарник с именем вида `*-GOOS-GOARCH` либо `*.exe` на Windows.\n\nИспользуйте ровно один из флагов `--tag` или `--latest`. Draft-релизы не принимаются; для prerelease нужен `--pre`.\nНеобязательный `--output-name` задаёт имя устанавливаемого файла.\n\nЭта команда устанавливает сторонние бинарники плагинов, а не сам CLI `plugin-kit-ai`."
    );

  text = text
    .replace(
      / {6}--output string\s+write bundle to this \.tar\.gz path \(default: &lt;root&gt;\/&lt;name&gt;_&lt;platform&gt;_&lt;runtime&gt;_bundle\.tar\.gz\)/g,
      "      --output string     записывает bundle в путь `.tar.gz` (по умолчанию: `&lt;root&gt;/&lt;name&gt;_&lt;platform&gt;_&lt;runtime&gt;_bundle.tar.gz`)"
    )
    .replace(
      / {6}--platform string\s+target override \("codex-runtime" or "claude"\)/g,
      "      --platform string   переопределяет целевую платформу (`codex-runtime` или `claude`)"
    )
    .replace(
      / {6}--dir string\s+directory for the installed binary \(created if missing\) \(default "bin"\)/g,
      "      --dir string            каталог для установленного бинарника (создаётся при отсутствии) (по умолчанию `bin`)"
    )
    .replace(
      /^  -f, --force\s+overwrite existing binary$/gm,
      "  -f, --force                 перезаписывает существующий бинарник"
    )
    .replace(
      / {6}--goarch string\s+target GOARCH override \(default: host GOARCH\)/g,
      "      --goarch string         переопределяет целевой `GOARCH` (по умолчанию: `GOARCH` хоста)"
    )
    .replace(
      / {6}--goos string\s+target GOOS override \(default: host GOOS\)/g,
      "      --goos string           переопределяет целевой `GOOS` (по умолчанию: `GOOS` хоста)"
    )
    .replace(
      / {6}--latest\s+install from GitHub releases\/latest \(non-prerelease\) instead of --tag/g,
      "      --latest                устанавливает из `GitHub releases/latest` (без prerelease) вместо `--tag`"
    )
    .replace(
      / {6}--output-name string\s+write binary under this filename in --dir \(default: name from archive\)/g,
      "      --output-name string    записывает бинарник под этим именем в `--dir` (по умолчанию: имя из архива)"
    )
    .replace(
      / {6}--tag string\s+Git release tag \(required unless --latest\), e\.g\. v0\.1\.0/g,
      "      --tag string            Git release tag (обязателен, если не указан `--latest`), например `v0.1.0`"
    );

  return text;
}
