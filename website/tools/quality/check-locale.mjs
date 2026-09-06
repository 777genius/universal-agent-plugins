import { dispositionErrors, inventory } from "./locale-dispositions.mjs";
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
  const canonicalIds = new Set();
  for (const file of await listMarkdownFiles(root)) {
    const relative = path.relative(root, file).replaceAll("\\", "/");
    const body = await fs.readFile(file, "utf8");
    const meta = await readFrontmatter(file);
    seen.add(relative);
    if (typeof meta.canonicalId !== "string" || !meta.canonicalId || canonicalIds.has(meta.canonicalId)) errors.push(`${locale}/${relative}: missing or duplicate canonical ID`);
    canonicalIds.add(meta.canonicalId);
    if (meta.locale !== locale) errors.push(`${locale}/${relative}: wrong locale`);
    if (locale === "en") { english.set(relative, meta); continue; }
    if (!english.has(relative)) errors.push(`${locale}/${relative}: missing English counterpart`);
    if (meta.canonicalId !== english.get(relative)?.canonicalId) errors.push(`${locale}/${relative}: canonical mismatch`);
    for (const error of dispositionErrors(`${locale}/${relative}`, body, meta)) errors.push(`${locale}/${relative}: ${error}`);
  }
  if (locale !== "en") for (const relative of english.keys()) {
    if (!seen.has(relative)) errors.push(`${locale}/${relative}: missing counterpart`);
  }
}
for (const relative of Object.keys(inventory.pages)) {
  try { await fs.access(path.join(sourceRoot, relative)); } catch { errors.push(`${relative}: inventoried page missing`); }
}
if (errors.length) {
  console.error(errors.join("\n"));
  process.exitCode = 1;
} else console.log(`Locale content parity: ${english.size} routes × ${locales.length} locales passed.`);
