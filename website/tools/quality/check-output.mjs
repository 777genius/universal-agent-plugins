import fs from "node:fs/promises";
import { createRequire } from "node:module";
import path from "node:path";
import { pathToFileURL } from "node:url";
import { docsBaseUrl, runtimeRoot, websiteRoot } from "../config/site.mjs";
import { listMarkdownFiles } from "../lib/fs.mjs";
import { docsLocaleEntries } from "../config/locales.mjs";
import { isRetiredArchive } from "../lib/public-routes.mjs";

// Vue is a direct docs dependency and declares this compiler dependency. Resolve
// from Vue so this also works with pnpm's isolated dependency layout.
const requireVue = createRequire(import.meta.resolve("vue/package.json"));
const { parse, NodeTypes } = requireVue("@vue/compiler-dom");
const nonRenderedTags = new Set(["script", "style", "template", "noscript"]);
const blockTags = new Set([
  "blockquote", "br", "dd", "div", "dl", "dt", "figcaption", "figure", "footer",
  "h1", "h2", "h3", "h4", "h5", "h6", "header", "hr", "li", "main", "nav",
  "ol", "p", "pre", "section", "table", "tbody", "td", "th", "thead", "tr", "ul"
]);

export const quickstartClaims = [
  "Use plugins", "Build plugins", "CLI 0.1.94", "explicit project scope",
  "source-build setup", "universal-agent-plugins", "plugin.json", "agentplugins author"
];

function stripDelimitedBlocks(body, startMarker, endMarker) {
  let output = "";
  let offset = 0;
  while (offset < body.length) {
    const start = body.indexOf(startMarker, offset);
    if (start < 0) {
      output += body.slice(offset);
      break;
    }
    output += body.slice(offset, start);
    const end = body.indexOf(endMarker, start + startMarker.length);
    if (end < 0) {
      output += body.slice(start);
      break;
    }
    offset = end + endMarker.length;
  }
  return output;
}

export const currentSourceBody = (body) => stripDelimitedBlocks(
  body,
  "<!-- locale-historical-source:start",
  "locale-historical-source:end -->"
);

export function currentSourceErrors(body, relative) {
  const current = currentSourceBody(body);
  return /Milestone A|\bplugin-kit-ai (?:install|init|doctor|generate|bootstrap|validate)\b|\/legacy\/v1\//i.test(current)
    ? [`Current public source contains retired authoring copy: ${relative}`]
    : [];
}

function readableElement(node) {
  return node.type === NodeTypes.ELEMENT &&
    !nonRenderedTags.has(node.tag.toLowerCase()) &&
    !node.props.some((prop) => prop.type === NodeTypes.ATTRIBUTE && prop.name === "hidden");
}

function findMain(node) {
  if (node.type === NodeTypes.ELEMENT && !readableElement(node)) return undefined;
  if (node.type === NodeTypes.ELEMENT && node.tag.toLowerCase() === "main") return node;
  for (const child of node.children || []) {
    const main = findMain(child);
    if (main) return main;
  }
}

function nodeText(node, separateBlocks = false) {
  if (node.type === NodeTypes.TEXT) return node.content;
  // These are already-rendered HTML assertions, so template-looking text stays
  // literal. Nothing in the inspected document is compiled or executed.
  if (node.type === NodeTypes.INTERPOLATION) return node.loc.source;
  if (node.type === NodeTypes.COMMENT ||
      (node.type === NodeTypes.ELEMENT && !readableElement(node))) return "";
  const text = (node.children || []).map((child) => nodeText(child, separateBlocks)).join("");
  return separateBlocks && node.type === NodeTypes.ELEMENT && blockTags.has(node.tag.toLowerCase())
    ? ` ${text} ` : text;
}

// Extract assertion data from a parsed document. These strings are never
// rendered as HTML; comments, raw-text containers and hidden elements cannot
// satisfy the visible copy, command or link requirements.
export function documentContent(html) {
  const parseErrors = [];
  const document = parse(html, { comments: true, onError: (error) => parseErrors.push(error.message) });
  const content = findMain(document) || document;
  const commands = [];
  const links = [];
  const inspect = (node) => {
    if (node.type === NodeTypes.ELEMENT) {
      if (!readableElement(node)) return;
      const tag = node.tag.toLowerCase();
      if (tag === "code") commands.push(nodeText(node));
      if (tag === "a") {
        const href = node.props.find((prop) => prop.type === NodeTypes.ATTRIBUTE && prop.name === "href");
        if (href?.value) links.push(href.value.content);
      }
    }
    for (const child of node.children || []) inspect(child);
  };
  inspect(content);
  return { text: nodeText(content, true).replace(/\s+/g, " ").trim(), commands, links, parseErrors };
}

function staleAvailabilityErrors(text, label) {
  return /Agent Plugins authoring (?:is )?available|public-channel E2E are verified/i.test(text)
    ? [`${label} still claims current public-channel authoring availability`]
    : [];
}

export function homeErrors(html) {
  const { text, commands, links, parseErrors } = documentContent(html);
  const errors = parseErrors.map((error) => `EN home page has invalid HTML: ${error}`);
  for (const claim of ["Use plugins", "Build plugins", "Agent Plugins", "plugin.json", "CLI 0.1.94", "explicit project scope", "locked restoration"]) {
    if (!text.includes(claim)) errors.push(`EN home page is missing its current installer guidance: ${claim}`);
  }
  if (!/current installer does not include agentplugins author/i.test(text)) {
    errors.push("EN home page must state that the current installer excludes agentplugins author");
  }
  if (!/implemented in current source/.test(text) || !links.some((href) => /\/build\/$/.test(href))) {
    errors.push("EN home page must link the current-source authoring guide");
  }
  if (!commands.some((command) => command.includes("npx universal-agent-plugins add context7"))) {
    errors.push("EN home page is missing its released installer command");
  }
  for (const retired of ["Supported Node And Python Paths", "codex-runtime --runtime", "delivery model", "repo-managed integration"]) {
    if (text.includes(retired)) errors.push(`EN home page still promotes retired authoring framing: ${retired}`);
  }
  return [...errors, ...staleAvailabilityErrors(text, "EN home page")];
}

export function quickstartErrors(html) {
  const { text, commands, links, parseErrors } = documentContent(html);
  const errors = parseErrors.map((error) => `Quickstart page has invalid HTML: ${error}`);
  for (const claim of quickstartClaims) {
    if (!text.includes(claim)) errors.push(`Quickstart page is missing its current installer/source guidance: ${claim}`);
  }
  if (!/current npm and native installer release does not include agentplugins author/i.test(text)) {
    errors.push("Quickstart must state that the current npm/native installer excludes agentplugins author");
  }
  if (!/Current source implements scaffolding/.test(text) ||
      !links.some((href) => /\/build\/#build-from-source$/.test(href))) {
    errors.push("Quickstart must direct authoring users to the current source-build setup");
  }
  const install = "npx universal-agent-plugins add context7";
  if (!commands.some((text) => text.includes(install))) errors.push(`Quickstart page is missing its released installer command: ${install}`);
  if (commands.some((text) => /npm install (?:--global|-g) universal-agent-plugins@\d+\.\d+\.\d+/.test(text))) {
    errors.push("Quickstart page pins an npm version instead of installing the latest release");
  }
  for (const retired of ["Milestone A", "plugin-kit-ai", "plugin.yaml", "/legacy/v1/", "Historical v1 maintenance", "PyPI", "pipx"]) {
    if (text.includes(retired) || commands.some((text) => text.includes(retired))) errors.push(`Quickstart page still exposes retired authoring copy: ${retired}`);
  }
  for (const jargon of ["runtime language", "repo-managed integration"]) {
    if (text.includes(jargon)) errors.push(`Quickstart page still contains retired front-door wording: ${jargon}`);
  }
  return [...errors, ...staleAvailabilityErrors(text, "Quickstart")];
}

async function listHtmlFiles(root) {
  const result = [];
  for (const entry of await fs.readdir(root, { withFileTypes: true })) {
    const full = path.join(root, entry.name);
    if (entry.isDirectory()) result.push(...await listHtmlFiles(full));
    else if (entry.isFile() && entry.name.endsWith(".html")) result.push(full);
  }
  return result;
}

export async function checkOutput() {
  const distRoot = path.join(websiteRoot, "dist");
  const errors = [];
  const htmlFiles = await listHtmlFiles(distRoot);
  for (const file of htmlFiles) {
    const relative = path.relative(distRoot, file).replaceAll("\\", "/");
    // Historical locale snapshots are embedded for immutable preservation and
    // are not current public copy. Scan only the visible/current source.
    const body = currentSourceBody(await fs.readFile(file, "utf8"));
    if (body.includes("maintainer-docs")) errors.push(`Internal docs leaked into built output: ${relative}`);
    if (/href="\/(en|ru|uk|zh|es|hi|ar|pt|fr)\//.test(body) || /src="\/assets\//.test(body)) errors.push(`Root-relative path detected in built output: ${relative}`);
    if (isRetiredArchive(relative)) errors.push(`Retired documentation was published: ${relative}`);
  }
  for (const relative of ["index.html", "404.html"]) {
    const body = await fs.readFile(path.join(distRoot, relative), "utf8");
    if (!body.includes("noindex,follow")) errors.push(`${relative} is missing robots noindex,follow.`);
  }
  const sitemap = await fs.readFile(path.join(distRoot, "sitemap.xml"), "utf8");
  if (sitemap.includes(`<loc>${docsBaseUrl}</loc>`) || /<loc>[^<]*\/404\/?<\/loc>/.test(sitemap)) errors.push("The gateway or not-found document leaked into sitemap.xml.");
  const robots = await fs.readFile(path.join(distRoot, "robots.txt"), "utf8");
  if (!robots.includes(`Sitemap: ${new URL("sitemap.xml", docsBaseUrl)}`)) errors.push("robots.txt is missing the sitemap declaration.");
  for (const file of await listMarkdownFiles(runtimeRoot)) {
    const body = await fs.readFile(file, "utf8");
    if (!/^generated:\s*true$/m.test(body)) continue;
    const title = body.match(/^title:\s*"([^"]+)"$/m)?.[1]?.trim();
    const heading = body.match(/^#\s+(.+)$/m)?.[1]?.trim();
    if (title && heading && title !== heading) errors.push(`Generated page title/H1 mismatch: ${file} ("${title}" vs "${heading}")`);
  }
  const entities = JSON.parse(await fs.readFile(path.join(websiteRoot, "generated", "registries", "entities.json"), "utf8"));
  for (const entity of entities.filter((entry) => entry.sourceKind === "hand-authored")) {
    for (const locale of docsLocaleEntries) {
      const field = `path${locale.code[0].toUpperCase()}${locale.code.slice(1)}`;
      const route = entity[field];
      if (!route) { errors.push(`${entity.canonicalId}: missing ${locale.code} route`); continue; }
      const file = path.join(distRoot, route.endsWith("/") ? `${route}index.html` : `${route}.html`);
      const html = await fs.readFile(file, "utf8");
      const root = html.match(/<html\b[^>]*>/)?.[0] || "";
      if (!root.includes(`lang="${locale.lang}"`) || !root.includes(`dir="${locale.dir}"`)) errors.push(`${route}: incorrect HTML language/direction`);
      const absolute = new URL(route.slice(1), docsBaseUrl).href;
      if (!html.includes(`rel="canonical" href="${absolute}"`)) errors.push(`${route}: missing canonical URL`);
      for (const alternate of docsLocaleEntries) {
        const href = new URL(route.replace(new RegExp(`^/${locale.code}/`), `${alternate.code}/`), docsBaseUrl).href;
        if (!html.includes(`hreflang="${alternate.lang}" href="${href}"`)) errors.push(`${route}: missing ${alternate.code} hreflang`);
      }
    }
  }
  const editPrefix = "https://github.com/777genius/universal-agent-plugins/edit/main/website/source/";
  const home = await fs.readFile(path.join(distRoot, "en", "index.html"), "utf8");
  if (!home.includes(`${editPrefix}en/index.md`)) errors.push("Hand-authored EN home page is missing its edit link.");
  errors.push(...homeErrors(home));
  const generatedCli = await fs.readFile(path.join(distRoot, "en", "api", "cli", "prepared-authoring-v2-agentplugins-author.html"), "utf8");
  if (generatedCli.includes(`${editPrefix}en/api/cli/`)) errors.push("Generated Agent Plugins CLI page generated a hand-authored edit link.");
  if (!generatedCli.includes(">Exact source<")) errors.push("Generated Agent Plugins CLI page is missing the exact-source link.");
  const quickstart = await fs.readFile(path.join(distRoot, "en", "guide", "quickstart.html"), "utf8");
  errors.push(...quickstartErrors(quickstart));
  const supportPolicy = await fs.readFile(path.join(websiteRoot, "..", "docs", "SUPPORT.md"), "utf8");
  for (const heading of ["## Recommended Production Lanes", "## Public Language And Formal Terms", "## Exact Contract Vocabulary"]) {
    if (!supportPolicy.includes(heading)) errors.push(`SUPPORT.md is missing its expected section: ${heading}`);
  }
  const currentSources = (await listMarkdownFiles(runtimeRoot)).filter((file) => !file.includes(`${path.sep}gateway${path.sep}`));
  for (const file of currentSources) {
    const relative = path.relative(runtimeRoot, file).replaceAll("\\", "/");
    if (isRetiredArchive(relative)) errors.push(`Retired source entered the assembled site: ${relative}`);
    errors.push(...currentSourceErrors(await fs.readFile(file, "utf8"), relative));
  }
  if (errors.length) throw new Error(errors.join("\n"));
  console.log(`Public output passed: ${htmlFiles.length} HTML files, current Agent Plugins routes only.`);
}

if (process.argv[1] && import.meta.url === pathToFileURL(path.resolve(process.argv[1])).href) await checkOutput();
