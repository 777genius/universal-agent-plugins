import { repoRoot } from "../config/site.mjs";
import { run } from "../lib/process.mjs";

export const historicalSHA = "9beca10448ac50fbe526a52101d1433a12471980";
export const historicalVersion = "1.2.4";
const repository = "https://github.com/777genius/universal-agent-plugins";

// Read committed reference artifacts, never execute the old product exporter.
// The legacy exporter implementation remains preserved at its existing source.
export async function extractHistorical(surfaces) {
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
      `website/generated/${locale}/api/${surface}`))], { cwd: repoRoot })).trim().split("\n").filter(Boolean);
  const pages = [];
  for (const file of files) {
    if (!file.endsWith(".md")) continue;
    let content = await show(file);
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
