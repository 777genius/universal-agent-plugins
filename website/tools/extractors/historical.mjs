import { renderMarkdownPage } from "../lib/frontmatter.mjs";
import { repoRoot } from "../config/site.mjs";
import { run } from "../lib/process.mjs";

export const historicalSHA = "9beca10448ac50fbe526a52101d1433a12471980";
export const historicalVersion = "1.2.4";
const repository = "https://github.com/777genius/universal-agent-plugins";

// Read committed reference artifacts, never execute the old product exporter.
// The legacy exporter implementation remains preserved at its existing source.
export async function extractHistorical(surfaces, referencePages = []) {
  const show = (file) => run("git", ["show", `${historicalSHA}:${file}`], { cwd: repoRoot });
  const registry = JSON.parse(await show("website/generated/registries/entities.json"));
  const entities = registry.filter((entry) => surfaces.includes(entry.surface)).map((entry) => ({
    ...entry, historicalVersion, sourceSHA: historicalSHA, status: "historical",
    historicalSourceRef: entry.sourceRef,
    sourceRef: entry.sourceRef.startsWith("cli:")
      ? `${repository}/tree/${historicalSHA}/cli/plugin-kit-ai`
      : `${repository}/${/\.[a-z0-9]+$/i.test(entry.sourceRef) ? "blob" : "tree"}/${historicalSHA}/${entry.sourceRef}`,
    stability: "historical", maturity: "historical"
  }));
  const files = (await run("git", ["ls-tree", "-r", "--name-only", historicalSHA,
    ...["en", "ru", "es", "fr", "zh"].flatMap((locale) => surfaces.map((surface) =>
      `website/generated/${locale}/api/${surface}`).concat(referencePages.map((page) =>
      `website/generated/${locale}/reference/${page}.md`)))], { cwd: repoRoot })).trim().split("\n").filter(Boolean);
  // The pinned registry declares Claude, but its tracked generated tree omits
  // all five pages. Recover only this confirmed gap from the canonical pinned
  // matrix. Do not invoke the legacy exporter or infer present-day support.
  const recovered = new Map();
  if (surfaces.includes("platform-events")) {
    const claude = registry.find(entry => entry.canonicalId === "event-platform:claude");
    if (claude?.sourceRef !== "docs/generated/support_matrix.md")
      throw new Error("Unexpected pinned Claude source");
    const matrix = await show(claude.sourceRef);
    const rows = matrix.split("\n").filter(line => line.startsWith("| claude |"))
      .map(line => line.split("|").slice(1, -1).map(cell => cell.trim()));
    if (!rows.length || rows.some(row => row.length !== 14) ||
        JSON.stringify(rows.map(row => row[1])) !== JSON.stringify(claude.searchTerms.slice(1)))
      throw new Error("Pinned Claude matrix and registry disagree");
    for (const locale of ["en", "ru", "es", "fr", "zh"]) {
      const file = `website/generated/${locale}/api/platform-events/claude.md`;
      if (files.includes(file)) continue;
      // English canonical facts are mirrored verbatim; no new locale claims.
      const table = rows.map(row => `| ${[row[1], row[3], row[4], row[13]].join(" | ")} |`).join("\n");
      recovered.set(file, renderMarkdownPage({
        title: "claude", description: "Historical event reference for claude",
        canonicalId: claude.canonicalId, surface: claude.surface, section: "api",
        locale, generated: true, editLink: false, stability: "historical",
        maturity: "historical", sourceRef: claude.sourceRef, translationRequired: false
      }, `# claude\n\n| Event | Maturity | Contract | Summary |\n| --- | --- | --- | --- |\n${table}\n`));
      files.push(file);
    }
  }
  const pages = [];
  for (const file of files) {
    if (!file.endsWith(".md")) continue;
    let content = recovered.get(file) ?? await show(file);
    content = content.replace(/^(stability|maturity): .*$/gm, '$1: "historical"')
      .replace(/\bstability="[^"]*"/g, 'stability="historical"')
      .replace(/\bmaturity="[^"]*"/g, 'maturity="historical"')
      .replace(/https:\/\/github\.com\/777genius\/plugin-kit-ai\/(tree|blob)\/main\//g,
        `${repository}/$1/${historicalSHA}/`);
    const banner = `> Historical plugin-kit-ai v1, baseline **1.2.4** ([exact source](${repository}/tree/${historicalSHA})). Project migration is not available in v2 yet. Maintain legacy projects using the v1 1.2.4 command set.\n\n`;
    content = content.replace(/^(---\n[\s\S]*?\n)(---\n)/,
      `$1historicalVersion: "1.2.4"\nsourceSHA: "${historicalSHA}"\nstatus: "historical"\n$2\n${banner}`);
    pages.push({ locale: file.split("/")[2], mirror: false,
      relativePath: file.slice("website/generated/".length), content });
  }
  if (!pages.length || !entities.length) throw new Error("Missing pinned historical reference");
  return { entities, pages };
}
