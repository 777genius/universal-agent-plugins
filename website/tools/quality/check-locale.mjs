import fs from "node:fs/promises";
import path from "node:path";
import { sourceRoot } from "../config/site.mjs";
import { listMarkdownFiles } from "../lib/fs.mjs";
import { readFrontmatter } from "../lib/site-model.mjs";

const locales = ["en", "ru", "es", "fr", "zh"];
const english = new Map();
const errors = [];
for (const locale of locales) {
  const root = path.join(sourceRoot, locale);
  const seen = new Set();
  for (const file of await listMarkdownFiles(root)) {
    const relative = path.relative(root, file).replaceAll("\\", "/");
    const body = await fs.readFile(file, "utf8");
    const meta = await readFrontmatter(file);
    seen.add(relative);
    if (meta.locale !== locale) errors.push(`${locale}/${relative}: wrong locale`);
    if (locale === "en") { english.set(relative, meta); continue; }
    if (!english.has(relative)) errors.push(`${locale}/${relative}: missing English counterpart`);
    if (meta.canonicalId !== english.get(relative)?.canonicalId) errors.push(`${locale}/${relative}: canonical mismatch`);
    if (/^(use|build|legacy\/v1)\//.test(relative)) {
      const target = `/en/${relative.replace(/index\.md$/, "").replace(/\.md$/, "")}`;
      if (!body.includes(`](${target})`)) errors.push(`${locale}/${relative}: missing explicit English pointer`);
      if (/```|`(?:plugin-kit-ai|agentplugins)\s/.test(body)) errors.push(`${locale}/${relative}: executable fallback copy`);
    } else if (!body.includes('<details><summary>') || !body.includes('1.2.4') || !body.trimEnd().endsWith('</details>')) {
      errors.push(`${locale}/${relative}: historical context missing`);
    }
  }
  if (locale !== "en") for (const relative of english.keys()) {
    if (!seen.has(relative)) errors.push(`${locale}/${relative}: missing counterpart`);
  }
}
if (errors.length) {
  console.error(errors.join("\n"));
  process.exitCode = 1;
} else console.log(`Locale content parity: ${english.size} routes × ${locales.length} locales passed.`);
