import { docsLocales, localePathField } from "../lib/journeys.mjs";
import fs from "node:fs/promises";
import path from "node:path";
import { generatedRegistryPaths, generatedRoot, repoRoot, runtimeRoot, sourceRoot, websiteRoot } from "../config/site.mjs";
import { listMarkdownFiles } from "../lib/fs.mjs";
import { readFrontmatter } from "../lib/site-model.mjs";

const entities = JSON.parse(await fs.readFile(generatedRegistryPaths.entities, "utf8"));
const errors = [];

await checkRegistryEntities(entities);
await checkGeneratedMarkdownParity();

if (errors.length > 0) {
  for (const error of errors) {
    console.error(error);
  }
  process.exit(1);
}

async function checkRegistryEntities(allEntities) {
  const ids = new Set();
  const paths = new Map();
  const entityMap = new Map(allEntities.map((entity) => [entity.canonicalId, entity]));

  for (const entity of allEntities) {
    if (ids.has(entity.canonicalId)) {
      errors.push(`Duplicate canonicalId in entities registry: ${entity.canonicalId}`);
    }
    ids.add(entity.canonicalId);

    for (const locale of docsLocales) {
      const localePath = entity[localePathField(locale)];
      if (!localePath && entity.localeStrategy === "canonical-en" && entity.pathEn && locale !== "en") continue;
      if (!localePath) {
        errors.push(`Missing ${locale.toUpperCase()} path for entity: ${entity.canonicalId}`);
        continue;
      }
      const existingPathOwner = paths.get(localePath);
      if (existingPathOwner && existingPathOwner !== entity.canonicalId) {
        errors.push(`URL collision: ${localePath} is used by both ${existingPathOwner} and ${entity.canonicalId}`);
      } else {
        paths.set(localePath, entity.canonicalId);
      }

      const expectedPage = markdownPathFor(locale, localePath);
      if (!expectedPage) {
        errors.push(`Cannot map entity path to markdown file: ${entity.canonicalId} -> ${localePath}`);
      } else {
        await fs.access(expectedPage).catch(() => {
          errors.push(`Entity path does not map to an assembled page: ${entity.canonicalId} -> ${expectedPage}`);
        });
      }
    }

    for (const relatedId of entity.relatedIds || []) {
      if (!entityMap.has(relatedId)) {
        errors.push(`Entity ${entity.canonicalId} points to missing relatedId ${relatedId}`);
      }
    }

    if (entity.publicVisibility === "preparation" &&
        (entity.status !== "prepared-not-release" || entity.released !== false || entity.stability !== "prepared-not-release")) {
      errors.push(`Invalid preparation status: ${entity.canonicalId}`);
    }
    if (!["public", "preparation"].includes(entity.publicVisibility)) {
      errors.push(`Unexpected non-public entity in public registry: ${entity.canonicalId} (${entity.publicVisibility})`);
    }

    if (typeof entity.sourceRef === "string" && entity.sourceRef && !entity.sourceRef.startsWith("cli:")) {
      const disallowedWrapper =
        entity.sourceRef.includes("npm/plugin-kit-ai/") || entity.sourceRef.includes("python/plugin-kit-ai/");
      if (disallowedWrapper) {
        errors.push(`Wrapper package leaked into public sourceRef for ${entity.canonicalId}: ${entity.sourceRef}`);
      }

      if (!entity.sourceRef.startsWith("http")) {
        const candidatePaths =
          entity.sourceKind === "hand-authored"
            ? docsLocales.map((locale) => path.join(sourceRoot, locale, entity.sourceRef))
            : [path.join(repoRoot, entity.sourceRef)];
        let found = false;
        for (const candidate of candidatePaths) {
          try {
            await fs.access(candidate);
            found = true;
            break;
          } catch {
            // continue
          }
        }
        if (!found) {
          errors.push(`Missing sourceRef target for ${entity.canonicalId}: ${entity.sourceRef}`);
        }
      }
    }

    if (entity.sourceRef?.includes(websiteRoot)) {
      errors.push(`Machine-specific absolute path leaked into sourceRef for ${entity.canonicalId}`);
    }
  }
}

async function checkGeneratedMarkdownParity() {
  const generatedFiles = (await Promise.all(docsLocales.map((locale) => listMarkdownFiles(path.join(generatedRoot, locale))))).flat();
  const localeCoverage = new Map();

  for (const filePath of generatedFiles) {
    const meta = await readFrontmatter(filePath);
    if (!meta.canonicalId) {
      errors.push(`Generated file is missing canonicalId frontmatter: ${filePath}`);
      continue;
    }
    if (!meta.locale) {
      errors.push(`Generated file is missing locale frontmatter: ${filePath}`);
      continue;
    }

    const relative = path.relative(generatedRoot, filePath).replace(/\\/g, "/");
    if (relative.includes("npm/plugin-kit-ai/") || relative.includes("python/plugin-kit-ai/")) {
      errors.push(`Wrapper package leaked into generated tree: ${relative}`);
    }
    if ((await fs.readFile(filePath, "utf8")).includes("/Users/")) {
      errors.push(`Machine-specific absolute path leaked into generated markdown: ${filePath}`);
    }

    const bucket = localeCoverage.get(meta.canonicalId) || new Set();
    bucket.add(meta.locale);
    localeCoverage.set(meta.canonicalId, bucket);
  }

  for (const [canonicalId, locales] of localeCoverage.entries()) {
    const entity = entities.find((entry) => entry.canonicalId === canonicalId);
    const required = entity?.localeStrategy === "canonical-en" ? ["en"] : docsLocales;
    if (required.some((locale) => !locales.has(locale))) {
      errors.push(`Generated locale parity failure for ${canonicalId}: have [${[...locales].sort().join(", ")}]`);
    }
  }
}

function markdownPathFor(locale, localePath) {
  if (!localePath.startsWith(`/${locale}/`)) {
    return "";
  }
  const relative = localePath.slice(`/${locale}/`.length);
  if (!relative) {
    return path.join(runtimeRoot, locale, "index.md");
  }
  if (relative.endsWith("/")) {
    return path.join(runtimeRoot, locale, relative, "index.md");
  }
  return path.join(runtimeRoot, locale, `${relative}.md`);
}
