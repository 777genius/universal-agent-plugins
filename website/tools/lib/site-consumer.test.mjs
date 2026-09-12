import assert from "node:assert/strict";
import fs from "node:fs/promises";
import path from "node:path";
import os from "node:os";
import vm from "node:vm";
import test from "node:test";
import { consumePreparedCLI, extractPreparedCLI, namespace } from "../extractors/prepared-cli.mjs";
import { extractPlatformData } from "../extractors/platform.mjs";
import { extractHistorical, historicalSHA } from "../extractors/historical.mjs";
import { scanSourceEntities, buildSidebar } from "../generate.mjs";
import { bindGeneratedPaths, docsLocales, entityPath, journeyNav, journeyLabels, journeyPageLabels, localePathField, requirePublicationBoundary } from "./journeys.mjs";
import { buildRedirects, createRedirectDocument, emitRedirects, htmlPath } from "./redirects.mjs";
import { sourceRoot, repoRoot, docsBaseUrl, repoBrowserUrl } from "../config/site.mjs";
import { run } from "./process.mjs";

const actualOutput = process.env.DOCS_TEST_ADAPTER_OUTPUT;
if (!actualOutput) throw new Error("Required fresh adapter fixture missing; run pnpm docs:test (integration runner)");
const sourceSHA = process.env.DOCS_AUTHORING_SOURCE_SHA;
const sourceEntities = await scanSourceEntities();
const inventory = JSON.parse(await fs.readFile(new URL("../config/routes.json", import.meta.url), "utf8"));
const artifacts = process.env.DOCS_TEST_ARTIFACTS;

async function fixture(t) {
  const directory = await fs.mkdtemp(path.join(os.tmpdir(), "d2b-consumer-"));
  t.after(() => fs.rm(directory, { recursive: true, force: true }));
  return directory;
}

function flattenLinks(sidebar) {
  const visit = (node) => Array.isArray(node) ? node.flatMap(visit) :
    [node.link, ...(node.items ? visit(node.items) : [])].filter(Boolean);
  return Object.values(sidebar).flatMap(visit);
}

test("production publication requires truthful preparation metadata and maintained locale routes", () => {
  assert.throws(() => requirePublicationBoundary([], {}), /non-empty/);
  assert.equal(requirePublicationBoundary(sourceEntities, {}), "truthful-public-checkpoint");
  const prepared = sourceEntities.find((entry) => entry.publicVisibility === "preparation");
  assert.ok(prepared);
  assert.throws(() => requirePublicationBoundary([{ ...prepared, released: true }], {}), /misclassifies/);
  assert.throws(() => requirePublicationBoundary([{ ...prepared, pathZh: "" }], {}), /maintained-locale/);
  assert.equal(requirePublicationBoundary([], { DOCS_PREPARATION_PREVIEW: "1" }), "disposable-preview");
  assert.throws(() => requirePublicationBoundary([], { DOCS_PREPARATION_PREVIEW: "true" }), /non-empty/);
});

test("all five navigations expose real Use and Build journeys with truthful English fallback", async () => {
  const prepared = sourceEntities.filter((entry) => entry.publicVisibility === "preparation");
  const requiredJourneys = ["use:index", "use:install", "use:manage", "build:index", "build:skill",
    "build:mcp-remote", "build:mcp-stdio", "build:hybrid", "build:skills", "build:layout",
    "build:checks", "build:handoff", "legacy:v1:index"];
  for (const id of requiredJourneys) assert.ok(prepared.some(entry => entry.canonicalId === `page:${id}`), id);
  for (const entry of prepared) {
    assert.equal(entry.released, false);
    assert.equal(entry.stability, "prepared-not-release");
    for (const locale of docsLocales) {
      const expected = entry.pathEn.replace(/^\/en\//, `/${locale}/`);
      assert.equal(entityPath(entry, locale), expected);
      const file = path.join(sourceRoot, expected.endsWith("/") ? `${expected}index.md` : `${expected}.md`);
      const body = await fs.readFile(file, "utf8");
      assert.ok(body.includes(`canonicalId: "${entry.canonicalId}"`), file);
      assert.ok(body.includes(`locale: "${locale}"`), file);
    }
  }
  for (const locale of docsLocales) {
    assert.deepEqual(journeyNav(locale).map((entry) => entry.link), [`/${locale}/use/`, `/${locale}/build/`]);
    const sidebar = buildSidebar(locale, sourceEntities);
    assert.equal(sidebar[`/${locale}/use/`][0].text, journeyLabels[locale][0]);
    assert.equal(sidebar[`/${locale}/build/`][1].text, journeyLabels[locale][1]);
    if (locale !== "en") assert.equal(sidebar[`/${locale}/use/`][0].items[0].text, journeyPageLabels[locale]["page:use:index"]);
    const links = flattenLinks({ use: sidebar[`/${locale}/use/`], build: sidebar[`/${locale}/build/`] });
    for (const entry of prepared.filter(entry => requiredJourneys.includes(entry.canonicalId.slice("page:".length))))
      assert.ok(links.includes(entityPath(entry, locale)), entry.canonicalId);
    const missing = { canonicalId: "generated:missing", title: "plugin-kit-ai missing translation", surface: "authoring-cli", pathEn: "/en/api/missing" };
    assert.equal(entityPath(missing, locale), missing.pathEn);
    assert.ok(flattenLinks(buildSidebar(locale, [...sourceEntities, missing])).includes(missing.pathEn));
  }
});

test("locale registry fields require actual generated pages; explicit translated routes win", () => {
  const entity = { pathEn: "/en/api/a", pathEs: "/es/api/translated", localeStrategy: "mirrored" };
  const [bound] = bindGeneratedPaths([entity], [
    { relativePath: "en/api/a.md" }, { relativePath: "es/api/translated.md" }, { relativePath: "fr/api/a.md" }
  ]);
  assert.equal(bound.pathEs, "/es/api/translated");
  assert.equal(bound.pathFr, "/fr/api/a");
  assert.equal(bound.pathRu, "");
  assert.equal(bound.pathZh, "");
  assert.equal(entityPath(bound, "zh"), "/en/api/a");
});

test("D1 journey relative links resolve against real source and retained reference", async () => {
  const errors = [];
  for (const entity of sourceEntities.filter((entry) => entry.publicVisibility === "preparation")) {
    const file = path.join(sourceRoot, "en", entity.sourceRef);
    const body = await fs.readFile(file, "utf8");
    for (const match of body.matchAll(/\]\(([^)]+)\)/g)) {
      const target = match[1].split("#")[0];
      if (!target || /^https?:/.test(target)) continue;
      const absolute = target.startsWith("/") ? path.join(sourceRoot, target) : path.resolve(path.dirname(file), target);
      const candidates = [absolute, `${absolute}.md`, path.join(absolute, "index.md")];
      let found = false;
      for (const candidate of candidates) {
        try { if ((await fs.stat(candidate)).isFile()) found = true; } catch { /* next suffix */ }
      }
      if (!found) errors.push(`${file}: ${target}`);
    }
  }
  assert.deepEqual(errors, []);
});

test("historical 1.2.4 keeps every pinned CLI page, command fact and old URL", async () => {
  const historical = await extractHistorical(["cli"]);
  const paths = (await run("git", ["ls-tree", "-r", "--name-only", historicalSHA, "website/generated"], { cwd: repoRoot }))
    .trim().split("\n").filter((file) => /\/(en|ru|es|fr|zh)\/api\/cli\/.*\.md$/.test(file));
  assert.equal(historical.pages.length, paths.length);
  const before = JSON.parse(await run("git", ["show", `${historicalSHA}:website/generated/registries/entities.json`], { cwd: repoRoot }))
    .filter((entry) => entry.surface === "cli");
  assert.deepEqual(historical.entities.map((entry) => entry.canonicalId), before.map((entry) => entry.canonicalId));
  assert.ok(historical.entities.some((entry) => entry.title === "plugin-kit-ai generate"));
  for (const entry of historical.entities) {
    assert.equal(entry.historicalVersion, "1.2.4");
    assert.equal(entry.sourceSHA, historicalSHA);
  }
  for (const page of historical.pages) {
    assert.match(page.content, /Historical plugin-kit-ai v1, baseline \*\*1\.2\.4\*\*/);
    assert.match(page.content, /Project migration is not available in v2 yet/);
    const original = await run("git", ["show", `${historicalSHA}:website/generated/${page.relativePath}`], { cwd: repoRoot });
    // Preserve every code fence verbatim, including flags, defaults and examples.
    assert.deepEqual([...page.content.matchAll(/```[\s\S]*?```/g)].map((m) => m[0]),
      [...original.matchAll(/```[\s\S]*?```/g)].map((m) => m[0]), page.relativePath);
    assert.deepEqual([...page.content.matchAll(/^#{1,6} .+$/gm)].map((m) => m[0]),
      [...original.matchAll(/^#{1,6} .+$/gm)].map((m) => m[0]), page.relativePath);
  }
});

test("actual accepted adapter envelope preserves all commands/flags/provenance and source parent", {}, async () => {
  const bundle = await consumePreparedCLI(actualOutput, sourceSHA);
  assert.equal(bundle.entities.length, bundle.envelope.surfaces.flatMap((surface) => surface.commands).length);
  const second = await consumePreparedCLI(actualOutput, sourceSHA);
  assert.deepEqual(second, bundle);
  for (const surface of bundle.envelope.surfaces) {
    for (const command of surface.commands) {
      const entity = bundle.entities.find((entry) => entry.canonicalId === command.identity);
      assert.deepEqual(entity.command, command);
      assert.equal(entity.released, false);
      assert.equal(entity.stability, "prepared-not-release");
      assert.equal(entity.sourceSHA, sourceSHA);
      assert.deepEqual(entity.sources, bundle.envelope.sources);
      assert.ok(!/\b(__\w+|migrate|migration|bootstrap|dev|generate|import|export|bundle|publish|normalize)\b/.test(command.command_path));
      assert.ok(!entity.pathEn.endsWith(`/api/cli/${command.command_path.replaceAll(" ", "-")}`));
    }
  }
  const parent = bundle.pages.find((page) => page.relativePath.endsWith("prepared-authoring-v2-agentplugins-author.md"));
  assert.match(parent.content, new RegExp(`https://github.com/777genius/universal-agent-plugins/blob/${sourceSHA}/cli/plugin-kit-ai/internal/agentpluginscli/root.go`));
  assert.match(parent.content, /--accept-security-risk/);
  assert.match(parent.content, /installer-only\nand rejected here/);
  const skill = bundle.pages.find((page) => page.relativePath.endsWith("plugin-kit-ai-skills-init.md"));
  assert.match(skill.content, /skills\/&lt;name&gt;/);
  assert.match(skill.content, /plugin-kit-ai skills init <name>/);
  for (const page of bundle.pages) {
    assert.equal(page.mirror, false);
    assert.ok(!/\]\([^)]*\.md\)/.test(page.content));
    for (const match of page.content.matchAll(/\]\((\/en\/[^)]+)\)/g)) {
      assert.ok(bundle.entities.some((entry) => entry.pathEn === match[1]), match[1]);
    }
  }
  for (const locale of docsLocales) {
    const sidebar = buildSidebar(locale, [...sourceEntities, ...bundle.entities]);
    const links = flattenLinks({ prepared: sidebar[`/${locale}/api/cli/prepared-authoring-v2`] });
    for (const entity of bundle.entities) assert.ok(links.includes(entity.pathEn));
  }
  if (artifacts) {
    await fs.mkdir(path.join(artifacts, "consumer-output"), { recursive: true });
    await fs.writeFile(path.join(artifacts, "consumer-output", "entities.json"), JSON.stringify(bundle.entities, null, 2) + "\n");
    for (const page of bundle.pages) {
      const destination = path.join(artifacts, "consumer-output", page.relativePath);
      await fs.mkdir(path.dirname(destination), { recursive: true });
      await fs.writeFile(destination, page.content);
    }
  }
});

test("extractor invokes the actual docs adapter on the clean explicit source, deterministically", {

}, async () => {
  const first = await extractPreparedCLI();
  const second = await extractPreparedCLI();
  assert.deepEqual(first, second);
  assert.deepEqual(first, await consumePreparedCLI(actualOutput, sourceSHA));
});

test("reject legacy arrays, wrong source/release, path traversal, missing surfaces and broken links", {}, async (t) => {
  const directory = await fixture(t);
  const manifestFile = path.join(directory, namespace, "manifest.json");
  const original = JSON.parse(await fs.readFile(path.join(actualOutput, namespace, "manifest.json"), "utf8"));
  await fs.cp(path.join(actualOutput, namespace), path.join(directory, namespace), { recursive: true });
  const mutations = [
    () => [],
    (m) => ({ ...m, released: true }),
    (m) => ({ ...m, status: "public-stable" }),
    (m) => ({ ...m, source_sha: "0".repeat(40) }),
    (m) => ({ ...m, surfaces: m.surfaces.slice(0, 1) }),
    (m) => { m.surfaces[0].commands[0].file_name = "../../v1.md"; return m; },
    (m) => { m.surfaces[0].commands.push(m.surfaces[0].commands[0]); return m; },
    (m) => { m.surfaces[0].commands[0].inherited_flags = null; return m; }
  ];
  for (const mutate of mutations) {
    await fs.writeFile(manifestFile, JSON.stringify(mutate(structuredClone(original))));
    await assert.rejects(consumePreparedCLI(directory, sourceSHA), /Prepared CLI contract/);
  }
  await fs.writeFile(manifestFile, JSON.stringify(original));
  const file = path.join(directory, original.surfaces[0].commands[0].file_name);
  await fs.appendFile(file, "\n[missing](missing.md)\n");
  await assert.rejects(consumePreparedCLI(directory, sourceSHA), /unresolved command link/);
});

test("source-owned aliases reject missing destinations, loops, chains and existing page collisions", () => {
  const routes = ["/en/", "/en/use/", "/en/build/", "/en/api/cli/", "/en/api/cli/plugin-kit-ai-generate"];
  const aliases = buildRedirects(inventory, routes);
  assert.equal(aliases["/use/"], "/en/use/");
  assert.equal(aliases["/api/cli/plugin-kit-ai-generate"], "/en/api/cli/plugin-kit-ai-generate");
  assert.ok(!( "/" in aliases));
  for (const extra of [
    { "/bad": "/missing" }, { "/bad": "/bad" }, { "/bad": "/other", "/other": "/en/use/" },
    { "/en/use/": "/en/build/" }, { "/../bad": "/en/use/" }
  ]) assert.throws(() => buildRedirects({ ...inventory, aliases: extra }, routes));
});

test("emitted HTML redirects preserve query/deep fragments at canonical base; no v1-to-v2 redirect", async (t) => {
  const directory = await fixture(t);
  const routes = ["/en/use/", "/en/build/", "/en/api/cli/", "/en/api/cli/plugin-kit-ai-generate"];
  const aliases = buildRedirects(inventory, routes);
  for (const route of routes) {
    const file = path.join(directory, htmlPath(route));
    await fs.mkdir(path.dirname(file), { recursive: true });
    await fs.writeFile(file, '<!doctype html><html><body><h2 id="options">Options</h2></body></html>');
  }
  await emitRedirects(directory, aliases, docsBaseUrl);
  const evidence = [];
  for (const [alias, target] of Object.entries(aliases)) {
    const html = await fs.readFile(path.join(directory, htmlPath(alias)), "utf8");
    assert.ok(html.includes(`<link rel="canonical" href="${new URL(target.slice(1), docsBaseUrl).href}">`));
    for (const fragment of ["#options", "#a%20b", ""]) {
      let actual;
      const location = { search: "?view=legacy&x=1", hash: fragment, replace: (url) => { actual = url; } };
      const script = html.match(/<script>([\s\S]*?)<\/script>/)[1];
      vm.runInNewContext(script, { location });
      assert.equal(actual, new URL(target.slice(1), docsBaseUrl).href + location.search + fragment);
      evidence.push({ alias, target, fragment, actual });
    }
  }
  await assert.rejects(emitRedirects(directory, aliases, docsBaseUrl), /overwrite/);
  assert.throws(() => createRedirectDocument("javascript:alert(1)"));
  if (artifacts) {
    await fs.cp(directory, path.join(artifacts, "redirect-output"), { recursive: true });
    await fs.writeFile(path.join(artifacts, "redirect-evidence.json"), JSON.stringify({
      scope: "Emitted fixture HTML and Node VM script execution; not VitePress/browser evidence", evidence
    }, null, 2) + "\n");
  }
});

test("new canonical source links retain Go module identity", () => {
  assert.equal(docsBaseUrl, "https://777genius.github.io/universal-agent-plugins/docs/");
  assert.equal(repoBrowserUrl("cli:x"), "https://github.com/777genius/universal-agent-plugins/tree/main/cli/plugin-kit-ai");
});


test("whole historical platform inventory retains all five support pages, facts and English alias", async () => {
  const bundle = await extractPlatformData();
  const expected = (await run("git", ["ls-tree", "-r", "--name-only", historicalSHA, "website/generated"], { cwd: repoRoot }))
    .trim().split("\n").filter((file) => /\/(en|ru|es|fr|zh)\/(api\/(platform-events|capabilities)\/.*|reference\/target-support)\.md$/.test(file));
  assert.equal(expected.length, 160);
  const tracked = bundle.pages.filter(p => !p.relativePath.endsWith("/platform-events/claude.md"));
  assert.deepEqual(tracked.map(p => `website/generated/${p.relativePath}`).sort(), expected.sort());
  assert.equal(bundle.pages.length, 165);
  assert.equal(bundle.pages.filter(p => p.relativePath.endsWith("reference/target-support.md")).length, 5);
  for (const page of tracked) {
    const original = await run("git", ["show", `${historicalSHA}:website/generated/${page.relativePath}`], { cwd: repoRoot });
    // Entire body facts preserved except the explicit historical banner and metadata links.
    const facts = text => text.split("\n").filter(line => !/^(stability:|maturity:|historicalVersion:|sourceSHA:|status:|> Historical)/.test(line))
      .join("\n").replace(/stability="[^"]*"/g, 'stability="historical"').replace(/maturity="[^"]*"/g, 'maturity="historical"')
      .replace(/https:\/\/github.com\/777genius\/plugin-kit-ai\/(tree|blob)\/main\//g, `https://github.com/777genius/universal-agent-plugins/$1/${historicalSHA}/`).replace(/\n+/g, "\n");
    assert.equal(facts(page.content), facts(original), page.relativePath);
    assert.ok(page.content.includes(`sourceSHA: "${historicalSHA}"`));
  }
  const aliases = buildRedirects(inventory, bundle.pages.map(p => `/${p.relativePath.replace(/\.md$/, "")}`).concat(["/en/api/cli/"]));
  assert.equal(aliases["/reference/target-support"], "/en/reference/target-support");
});


test("every pinned historical registry route produces a page, including Claude facts in five locales", async () => {
  const bundles = [await extractHistorical(["cli"]), await extractPlatformData()];
  const pages = bundles.flatMap(bundle => bundle.pages);
  const routes = new Set(pages.map(page => `/${page.relativePath.replace(/\.md$/, "")}`));
  const registry = JSON.parse(await run("git", ["show", `${historicalSHA}:website/generated/registries/entities.json`], { cwd: repoRoot }));
  for (const entry of registry.filter(entry => ["cli", "platform-events", "capabilities"].includes(entry.surface))) {
    for (const [key, route] of Object.entries(entry).filter(([key, value]) => /^path[A-Z]/.test(key) && value))
      assert.ok(routes.has(route), `${entry.canonicalId} ${key}: ${route}`);
    if (entry.localeStrategy === "mirrored")
      for (const locale of ["en", "ru", "es", "fr", "zh"])
        assert.ok(routes.has(entry.pathEn.replace(/^\/en\//, `/${locale}/`)), `${entry.canonicalId}: ${locale}`);
  }
  const matrix = await run("git", ["show", `${historicalSHA}:docs/generated/support_matrix.md`], { cwd: repoRoot });
  const facts = matrix.split("\n").filter(line => line.startsWith("| claude |"))
    .map(line => line.split("|").slice(1, -1).map(cell => cell.trim()))
    .map(row => [row[1], row[3], row[4], row[13]]);
  assert.equal(facts.length, 18);
  assert.equal(facts.filter(row => row[1] === "stable").length, 3);
  assert.equal(facts.filter(row => row[1] === "beta").length, 15);
  for (const locale of ["en", "ru", "es", "fr", "zh"]) {
    const page = pages.find(page => page.relativePath === `${locale}/api/platform-events/claude.md`);
    assert.ok(page);
    assert.deepEqual(page.content.split("\n").filter(line => line.startsWith("| ")).slice(2)
      .map(line => line.split("|").slice(1, -1).map(cell => cell.trim())), facts);
    assert.match(page.content, /status: "historical"/);
    assert.ok(page.content.includes(`sourceSHA: "${historicalSHA}"`));
    assert.ok(page.content.includes("Project migration is not available in v2 yet."));
  }
  assert.deepEqual(await extractPlatformData(), bundles[1]);
});
