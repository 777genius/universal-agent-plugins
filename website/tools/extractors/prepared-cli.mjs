import { requireAuthoringSource } from "../lib/source-contract.mjs";
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
    if (entry.command_path !== "agentplugins author test") return entry;
    const runtimeFlags = new Set(["allow-network", "deadline", "fixture", "runtime", "server", "tool"]);
    const summary = "Check package configuration and hygiene without executing package code";
    return { ...entry, short: summary, long: entry.long.replace(entry.short, summary),
      local_flags: entry.local_flags.filter((flag) => !runtimeFlags.has(flag.name)) };
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

export async function extractPreparedCLI() {
  const { checkout, sha } = await requireAuthoringSource();
  await fs.mkdir(docsToolsRoot, { recursive: true });
  const parent = await fs.mkdtemp(path.join(docsToolsRoot, "authoring-"));
  const output = path.join(parent, "export"); // Adapter requires an absent destination.
  await run("go", ["run", "-p", "2", "./cli/plugin-kit-ai/tools/authoring-docs",
    "--source-sha", sha, "--checkout", checkout, "--out-dir", output], {
    // The adapter belongs to this docs consumer commit. It verifies and reads
    // the immutable release checkout instead of relying on the older adapter
    // bytes that happened to ship in that release.
    cwd: repoRoot, env: { GOWORK: path.join(repoRoot, "go.work") }
  });
  return consumePreparedCLI(output, sha);
}
