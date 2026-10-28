import fs from "node:fs/promises";
import path from "node:path";
import { pathToFileURL } from "node:url";
import { docsBaseUrl, runtimeRoot, websiteRoot } from "../config/site.mjs";
import { listMarkdownFiles } from "../lib/fs.mjs";
import { isRetiredArchive } from "../lib/public-routes.mjs";

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

function stripHtmlComments(html) {
  return stripDelimitedBlocks(html, "<!--", "-->");
}

function documentContent(html) {
  const current = stripHtmlComments(html);
  const visible = current.match(/<main\b[\s\S]*?<\/main>/)?.[0] || current;
  const text = visible.replace(/<\/(?:p|h[1-6]|li|div|pre|section)>/g, " ")
    .replace(/<[^>]*>/g, "").replace(/&(?:nbsp|#160);/g, " ")
    .replaceAll("&amp;", "&").replace(/\s+/g, " ").trim();
  const commands = [...visible.matchAll(/<code\b[^>]*>([\s\S]*?)<\/code>/g)]
    .map((match) => match[1].replace(/<\/?span\b[^>]*>/g, ""));
  return { visible, text, commands };
}

function staleAvailabilityErrors(text, label) {
  return /Agent Plugins authoring (?:is )?available|public-channel E2E are verified/i.test(text)
    ? [`${label} still claims current public-channel authoring availability`]
    : [];
}

export function homeErrors(html) {
  const { visible, text, commands } = documentContent(html);
  const errors = [];
  for (const claim of ["Use plugins", "Build plugins", "Agent Plugins", "plugin.json", "CLI 0.1.94", "explicit project scope", "locked restoration"]) {
    if (!text.includes(claim)) errors.push(`EN home page is missing its current installer guidance: ${claim}`);
  }
  if (!/current installer does not include agentplugins author/i.test(text)) {
    errors.push("EN home page must state that the current installer excludes agentplugins author");
  }
  if (!/implemented in current source/.test(text) || !/href=["'][^"']*\/build\/["']/.test(visible)) {
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
  const errors = [];
  const { visible, text, commands } = documentContent(html);
  for (const claim of quickstartClaims) {
    if (!text.includes(claim)) errors.push(`Quickstart page is missing its current installer/source guidance: ${claim}`);
  }
  if (!/current npm and native installer release does not include agentplugins author/i.test(text)) {
    errors.push("Quickstart must state that the current npm/native installer excludes agentplugins author");
  }
  if (!/Current source implements scaffolding/.test(text) || !/href=["'][^"']*\/build\/#build-from-source["']/.test(visible)) {
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
    if (/href="\/(en|ru|es|fr|zh)\//.test(body) || /src="\/assets\//.test(body)) errors.push(`Root-relative path detected in built output: ${relative}`);
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
