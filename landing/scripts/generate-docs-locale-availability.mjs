import fs from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const scriptDir = path.dirname(fileURLToPath(import.meta.url));
const landingRoot = path.resolve(scriptDir, "..");
const repoRoot = path.resolve(landingRoot, "..");
const sourcePath = path.join(repoRoot, "website", "tools", "quality", "locale-dispositions.json");
const targetPath = path.join(landingRoot, "data", "docsLocaleAvailability.generated.json");

const source = JSON.parse(await fs.readFile(sourcePath, "utf8"));

const realPaths = {};
for (const [pageKey, entry] of Object.entries(source.pages)) {
  const [locale, ...rest] = pageKey.split("/");
  if (locale === "en") continue;
  if (entry.disposition !== "historical-snapshot" && entry.disposition !== "current-translation") continue;
  const relative = rest.join("/").replace(/\.md$/, "");
  const docPath = relative.replace(/(^|\/)index$/, "").replace(/\/$/, "");
  (realPaths[locale] ??= []).push(docPath);
}
for (const locale of Object.keys(realPaths)) realPaths[locale].sort();

const output = {
  generatedFrom: "website/tools/quality/locale-dispositions.json",
  reviewedCommit: source.reviewedCommit,
  realPaths,
};

await fs.writeFile(targetPath, `${JSON.stringify(output, null, 2)}\n`);
console.log(`Wrote ${targetPath}`);
