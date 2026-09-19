import { requireAuthoringSource } from "../lib/source-contract.mjs";
import { createHash } from "node:crypto";
import fs from "node:fs/promises";
import path from "node:path";
import { docsToolsRoot, repoRoot } from "../config/site.mjs";
import { renderMarkdownPage } from "../lib/frontmatter.mjs";
import { run } from "../lib/process.mjs";

export const namespace = "prepared-authoring-v2";
export const factoryBaseline = "070663efb27f69ecae8609e6b839f86f843efbb0";
const repository = "https://github.com/777genius/universal-agent-plugins";
const commandPathPattern = /^(plugin-kit-ai|agentplugins author)( [a-z0-9-]+)*$/;

// This consumes only the reviewed adapter contract. The explicit Phase 7
// projection below keeps unreleased runtime commands out of the released site;
// it never guesses a legacy array's version or fabricates translated pages.
export async function consumePreparedCLI(root, expectedSHA) {
  const envelope = JSON.parse(await fs.readFile(path.join(root, namespace, "manifest.json"), "utf8"));
  const fail = (message) => { throw new Error(`Prepared CLI contract: ${message}`); };
  if (envelope.schema !== "authoring-docs-manifest-v1" || envelope.namespace !== namespace ||
      envelope.status !== "prepared-not-release" || envelope.released !== false ||
      !/^[0-9a-f]{40}$/.test(expectedSHA || "") || envelope.source_sha !== expectedSHA ||
      envelope.factory_baseline_sha !== factoryBaseline) fail("invalid envelope/provenance");
  if (!Array.isArray(envelope.sources) || !envelope.sources.length || envelope.sources.some((pin) =>
    !/^[a-zA-Z0-9_./-]+$/.test(pin.path) || pin.path.startsWith("/") ||
    pin.path.split("/").includes("..") || !/^[0-9a-f]{64}$/.test(pin.sha256))) fail("invalid source pins");
  if (!Array.isArray(envelope.surfaces) || envelope.surfaces.length !== 2 ||
      envelope.surfaces.map((surface) => surface.command_path).sort().join("|") !==
      "agentplugins author|plugin-kit-ai") fail("expected both actual surfaces");
  const entries = envelope.surfaces.filter((surface) => surface.command_path === "agentplugins author").flatMap((surface) => {
    if (surface.identity !== `${namespace}:${surface.command_path}` || !Array.isArray(surface.commands) ||
        !surface.commands.some((entry) => entry.command_path === surface.command_path)) fail("invalid surface root");
    for (const entry of surface.commands) {
      if (entry.command_path !== surface.command_path && !entry.command_path.startsWith(`${surface.command_path} `))
        fail("command outside surface");
    }
    return surface.commands;
  }).filter((entry) => entry.command_path !== "agentplugins author dev").map((entry) => {
    const current = { ...entry, example: entry.example.replaceAll("plugin-kit-ai skills", "agentplugins author skills") };
    if (current.command_path !== "agentplugins author test") return current;
    const runtimeFlags = new Set(["allow-network", "deadline", "fixture", "runtime", "server", "tool"]);
    const summary = "Check package configuration and hygiene without executing package code";
    return { ...current, short: summary, long: current.long.replace(current.short, summary),
      local_flags: current.local_flags.filter((flag) => !runtimeFlags.has(flag.name)) };
  });
  const ids = new Set();
  const links = new Map();
  for (const entry of entries) {
    if (!commandPathPattern.test(entry.command_path) || entry.identity !== `${namespace}:${entry.command_path}` ||
        entry.slug !== `${namespace}-${entry.command_path.replaceAll(" ", "-")}` ||
        entry.file_name !== `${namespace}/${entry.command_path.replaceAll(" ", "_")}.md` || ids.has(entry.identity))
      fail("invalid or duplicate command identity");
    ids.add(entry.identity);
    for (const key of ["use", "short", "long", "example"]) if (typeof entry[key] !== "string") fail(`missing ${key}`);
    for (const key of ["local_flags", "inherited_flags"]) {
      if (!Array.isArray(entry[key]) || entry[key].some((flag) =>
        ["name", "type", "default", "usage"].some((field) => typeof flag[field] !== "string"))) fail(`invalid ${key}`);
    }
    links.set(path.basename(entry.file_name), `/en/api/cli/${entry.slug}`);
  }
  const pages = [];
  const entities = [];
  for (const entry of entries) {
    const original = await fs.readFile(path.join(root, entry.file_name), "utf8");
    if (!original.startsWith(`<!-- namespace: ${namespace}; status: prepared-not-release; source-sha: ${expectedSHA} -->`))
      fail("Markdown provenance mismatch");
    const releasedMarker = `<!-- namespace: ${namespace}; status: released; source-sha: ${expectedSHA} -->`;
    const linked = original
      .split("\n").filter((line) => !line.includes("agentplugins_author_dev.md") &&
        !(entry.command_path === "agentplugins author test" &&
          /^\s+--(?:allow-network|deadline|fixture|runtime|server|tool)(?:\s|$)/.test(line))).join("\n")
      .replaceAll("Check statically by default, or run one explicit MCP server with --runtime=mcp",
        "Check package configuration and hygiene without executing package code")
      .replaceAll("Check statically; the unreleased Phase 7 candidate can run one explicit MCP server",
        "Check package configuration and hygiene without executing package code")
      .replaceAll("plugin-kit-ai skills", "agentplugins author skills")
      .replace(`<!-- namespace: ${namespace}; status: prepared-not-release; source-sha: ${expectedSHA} -->`, releasedMarker)
      .replace("Prepared reference only; not a public release.", "Released Agent Plugins CLI reference.")
      .replace(/\]\(([^)]+)\)/g, (full, target) => {
      if (/^https:\/\//.test(target)) {
        if (!target.startsWith(`${repository}/blob/${expectedSHA}/`)) fail(`unexpected source link ${target}`);
        return full;
      }
      const match = target.match(/^([^#]+\.md)(#.*)?$/);
      if (!match || !links.has(match[1])) fail(`unresolved command link ${target}`);
      return `](${links.get(match[1])}${match[2] || ""})`;
      });
    if (/\bplugin-kit-ai\s/.test(linked)) fail(`retired command example in ${entry.command_path}`);
    let fence = false;
    const body = linked.split("\n").map((line) => {
      if (line.startsWith("```")) { fence = !fence; return line; }
      if (fence || line.startsWith("<!--")) return line;
      return line.replaceAll("<", "&lt;").replaceAll(">", "&gt;");
    }).join("\n").replace(`## ${entry.command_path}\n`, `# ${entry.command_path}\n`);
    const sourceHref = `${repository}/tree/${expectedSHA}/cli/plugin-kit-ai/internal/authoring/commands`;
    const metadata = {
      namespace, status: "released", released: true, sourceSHA: expectedSHA,
      factoryBaselineSHA: envelope.factory_baseline_sha, sources: envelope.sources,
      stability: "public-stable", maturity: "stable", publicVisibility: "public",
      localeStrategy: "canonical-en", sourceKind: "authoring-docs-adapter", sourceRef: sourceHref
    };
    entities.push({
      ...metadata, canonicalId: entry.identity, kind: "command", surface: "authoring-cli",
      title: entry.command_path, summary: entry.short, pathEn: links.get(path.basename(entry.file_name)),
      pathRu: "", pathEs: "", pathFr: "", pathZh: "", relatedIds: [],
      searchTerms: [entry.command_path, ...(entry.aliases || [])], command: entry
    });
    pages.push({
      locale: "en", mirror: false, relativePath: `en/api/cli/${entry.slug}.md`,
      content: renderMarkdownPage({
        title: entry.command_path, description: entry.short, canonicalId: entry.identity,
        section: "api", surface: "authoring-cli", locale: "en", generated: true, editLink: false,
        translationRequired: false, ...metadata, sources: envelope.sources.map((pin) => `${pin.path}: ${pin.sha256}`)
      }, `> Released Agent Plugins CLI reference from the exact source. [Exact source](${sourceHref}).\n\n${body}`)
    });
  }
  return { entities, pages, envelope };
}

async function releaseAdapterOverlay(checkout, parent) {
  const sourceName = "cli/plugin-kit-ai/tools/authoring-docs/source.go";
  let source = await fs.readFile(path.join(repoRoot, sourceName), "utf8");
  const releasePins = new Map([
    ["cli/plugin-kit-ai/cmd/agentplugins/release_root.go", "c0465f90903c7ad3fcdc2283c241558d1b73bd9633f0e6af247ccd692a0e155e"],
    ["cli/plugin-kit-ai/internal/authoring/commands/commands.go", "9ed49814d6145f61e28ac8c5b914383749f06d946dcece15db2c1059fd290220"],
    ["cli/plugin-kit-ai/internal/authoring/commands/public_contract.go", "39c79f491f0733d352ffc0fa3a8ff4eaa169612e8876e92e5fb74c856663ec65"],
    ["cli/plugin-kit-ai/internal/authoring/commands/version.go", "ffe6cfef352faeb9a3c00722a628a2093876c14120cd6725131b3e105fda6b29"]
  ]);
  for (const [name, digest] of releasePins) {
    const escaped = name.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
    const pattern = new RegExp(`\\{"${escaped}", "[0-9a-f]{64}"\\}`);
    if (!pattern.test(source)) throw new Error(`Released authoring adapter pin missing: ${name}`);
    source = source.replace(pattern, `{"${name}", "${digest}"}`);
  }
  const devPin = /^\s*\{"cli\/plugin-kit-ai\/internal\/authoring\/commands\/dev_session\.go", "[0-9a-f]{64}"\},\n/m;
  if (!devPin.test(source)) throw new Error("Released authoring adapter projection lost the Phase 7 boundary");
  source = source.replace(devPin, "");
  // Phase 8A is present only in current source. The released command tree is
  // compiled from v0.1.65, so its overlaid source inventory must not require
  // current-only packages or broaden the immutable release attestation.
  for (const prefix of [
    "cli/plugin-kit-ai/internal/authoring/commands/maintenance.go",
    "cli/plugin-kit-ai/internal/authoring/jsonmaint/",
    "cli/plugin-kit-ai/internal/authoring/nativeimport/",
    "cli/plugin-kit-ai/internal/authoring/report/",
    "cli/plugin-kit-ai/internal/authoring/scaffold/"
  ]) {
    let found = false;
    source = source.split("\n").filter((line) => {
      const match = line.trim().match(/^\{"([^"]+)", "[0-9a-f]{64}"\},$/);
      const remove = match && (prefix.endsWith("/") ? match[1].startsWith(prefix) : match[1] === prefix);
      found ||= Boolean(remove);
      return !remove;
    }).join("\n");
    if (!found) throw new Error(`Current-only authoring adapter pin missing: ${prefix}`);
  }
  for (const dir of ["jsonmaint", "nativeimport", "report", "scaffold"]) {
    const line = `\t"cli/plugin-kit-ai/internal/authoring/${dir}",\n`;
    if (!source.includes(line)) throw new Error(`Current-only authoring adapter directory missing: ${dir}`);
    source = source.replace(line, "");
  }
  // Current factoryPins track this checkout. The released command tree is
  // compiled from v0.1.65, so remaining pins must attest that tag's bytes, and
  // pins for files the tag does not have drop out (nested agentplugins go.mod
  // and current-only construction such as domain/planning.go).
  const pinLine = /^\s*\{"([^"]+)", "[0-9a-f]{64}"\},$/;
  const attested = new Map();
  for (const line of source.split("\n")) {
    const match = line.match(pinLine);
    if (!match) continue;
    const file = path.join(checkout, match[1]);
    try {
      attested.set(match[1], createHash("sha256").update(await fs.readFile(file)).digest("hex"));
    } catch (error) {
      if (error.code !== "ENOENT") throw error;
    }
  }
  source = source.split("\n").filter((line) => {
    const match = line.match(pinLine);
    return !match || attested.has(match[1]);
  }).join("\n");
  source = source.replace(/\{"([^"]+)", "[0-9a-f]{64}"\}/g, (all, name) => {
    const digest = attested.get(name);
    if (!digest) throw new Error(`Released authoring adapter pin lost after attestation: ${name}`);
    return `{"${name}", "${digest}"}`;
  });
  const projected = path.join(parent, "agentplugins-v0.1.65-source.go");
  const overlay = path.join(parent, "agentplugins-v0.1.65-overlay.json");
  await fs.writeFile(projected, source, { flag: "wx" });
  await fs.writeFile(overlay, JSON.stringify({ Replace: { [path.join(checkout, sourceName)]: projected } }) + "\n", { flag: "wx" });
  return overlay;
}

export async function exportPreparedCLI(output) {
  const { checkout, sha } = await requireAuthoringSource();
  await fs.mkdir(docsToolsRoot, { recursive: true });
  const parent = await fs.mkdtemp(path.join(docsToolsRoot, "authoring-"));
  const overlay = await releaseAdapterOverlay(checkout, parent);
  await run("go", ["run", "-overlay", overlay, "-p", "2", "./cli/plugin-kit-ai/tools/authoring-docs",
    "--source-sha", sha, "--checkout", checkout, "--out-dir", output], {
    // The command tree is compiled wholly from the immutable release checkout.
    // The overlay only refreshes that tag's stale source inventory so it covers
    // unrelated Go files added before the final v0.1.65 tag; it does not replace
    // commands, factories, flags, renderers, or generated documentation.
    cwd: checkout, env: { GOWORK: path.join(checkout, "go.work") }
  });
  return { checkout, sha };
}

export async function extractPreparedCLI() {
  await fs.mkdir(docsToolsRoot, { recursive: true });
  const parent = await fs.mkdtemp(path.join(docsToolsRoot, "authoring-output-"));
  const output = path.join(parent, "export"); // Adapter requires an absent destination.
  const { sha } = await exportPreparedCLI(output);
  return consumePreparedCLI(output, sha);
}
