import fs from "node:fs/promises";
import path from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
import { locales, sourceRoot } from "../../website/tools/config/site.mjs";
import { listMarkdownFiles } from "../../website/tools/lib/fs.mjs";
import { isRetiredArchive } from "../../website/tools/lib/public-routes.mjs";
import { readFrontmatter } from "../../website/tools/lib/site-model.mjs";

const scriptDir = path.dirname(fileURLToPath(import.meta.url));
const landingRoot = path.resolve(scriptDir, "..");
const targetPath = path.join(landingRoot, "data", "docsLocaleAvailability.generated.json");

// The immutable disposition inventory attests historical bytes. A later
// translation declares its current state in source frontmatter instead of
// rewriting that inventory, so publication links must inspect the live source.
export async function collectDocsLocaleAvailability(sourceDirectory = sourceRoot) {
  /** @type {Record<string, string[]>} */
  const realPaths = {};
  for (const { code: locale } of locales) {
    const localeRoot = path.join(sourceDirectory, locale);
    realPaths[locale] = [];
    for (const file of await listMarkdownFiles(localeRoot)) {
      const relative = path.relative(localeRoot, file).replaceAll("\\", "/");
      if (isRetiredArchive(`${locale}/${relative}`)) continue;
      const meta = await readFrontmatter(file);
      if (meta.locale !== locale || (locale !== "en" && meta.localeDisposition !== "current-translation")) continue;
      realPaths[locale].push(relative.replace(/\.md$/, "").replace(/(^|\/)index$/, "").replace(/\/$/, ""));
    }
    realPaths[locale].sort();
  }
  return {
    generatedFrom: "website/source public-page frontmatter",
    realPaths,
  };
}

if (process.argv[1] && import.meta.url === pathToFileURL(path.resolve(process.argv[1])).href) {
  await fs.writeFile(targetPath, `${JSON.stringify(await collectDocsLocaleAvailability(), null, 2)}\n`);
  console.log(`Wrote ${targetPath}`);
}
