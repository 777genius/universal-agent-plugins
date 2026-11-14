import fs from "node:fs/promises";
import path from "node:path";

export function makeEntity(base) {
  return {
    relatedIds: [],
    searchTerms: [],
    publicVisibility: "public",
    ...base
  };
}

export async function readFrontmatter(filePath) {
  const body = await fs.readFile(filePath, "utf8");
  const match = body.match(/^---\n([\s\S]*?)\n---\n/);
  const meta = {};
  if (!match) {
    return meta;
  }
  for (const line of match[1].split("\n")) {
    const current = line.trim();
    if (!current || current.startsWith("#")) {
      continue;
    }
    const idx = current.indexOf(":");
    if (idx === -1) {
      continue;
    }
    const key = current.slice(0, idx).trim();
    const value = current.slice(idx + 1).trim().replace(/^"|"$/g, "");
    meta[key] = value;
  }
  return meta;
}

export function localeTitle(locale, enTitle, ruTitle) {
  return locale === "ru" ? ruTitle : enTitle;
}

export function docLink(locale, relativePath) {
  return `/${locale}/${relativePath.replace(/\.md$/, "").replace(/index$/, "")}`;
}

export function relativePagePath(locale, ...parts) {
  return path.join(locale, ...parts);
}
