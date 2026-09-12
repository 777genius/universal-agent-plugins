import test from "node:test";
import vm from "node:vm";
import assert from "node:assert/strict";
import fs from "node:fs/promises";
import path from "node:path";
import { inventory, dispositionErrors } from "./locale-dispositions.mjs";
import { journeyNav, journeyLabels, journeySidebar } from "../lib/journeys.mjs";
import { sourceRoot, repoRoot } from "../config/site.mjs";
import { listMarkdownFiles } from "../lib/fs.mjs";
import { readFrontmatter } from "../lib/site-model.mjs";
import { localeCodes, localeDestination, localeFromPath, preferredLocale } from "../../.vitepress/theme/components/locale-routes.mjs";

const routeFor = (relative) => `/${relative.replace(/index\.md$/, "").replace(/\.md$/, "")}`;
const routes = new Map();
const entities = new Map();
for (const locale of localeCodes) for (const file of await listMarkdownFiles(path.join(sourceRoot, locale))) {
  const relative = path.relative(sourceRoot, file);
  const meta = await readFrontmatter(file);
  routes.set(routeFor(relative), file);
  const entity = entities.get(meta.canonicalId) || {};
  entity[`path${locale[0].toUpperCase()}${locale.slice(1)}`] = routeFor(relative);
  entities.set(meta.canonicalId, entity);
}

test("all five locales switch through existing counterpart routes and matching canonical IDs", async () => {
  for (const [id, entity] of entities) for (const locale of localeCodes) {
    const target = localeDestination(entity, locale);
    assert.equal(target.fallback, !entity[`path${locale[0].toUpperCase()}${locale.slice(1)}`], `${id}/${locale}`);
    assert.ok(routes.has(target.path), target.path);
    assert.equal((await readFrontmatter(routes.get(target.path))).canonicalId, id);
  }
});
test("missing generated translation uses English and never fabricates a locale URL", () => {
  for (const code of localeCodes.slice(1)) {
    assert.deepEqual(localeDestination({ pathEn: "/en/api/prepared-authoring-v2/plugin-kit-ai" }, code), {
      path: "/en/api/prepared-authoring-v2/plugin-kit-ai", language: "en", fallback: true, home: false
    });
    assert.equal(localeDestination(null, code).path, `/${code}/`);
  }
});
test("base paths, html aliases, queries and fragments do not corrupt locale detection", () => {
  for (const code of localeCodes) for (const basePath of ["/", "/universal-agent-plugins/docs/"]) {
    assert.equal(localeFromPath(`${basePath}${code}/build/skill.html?x=1#checks`, basePath), code);
  }
  assert.equal(localeFromPath("/docs-other/ru/build/", "/docs/"), null);
});
test("gateway honors saved locale then browser preference order including English", async () => {
  for (const code of localeCodes) assert.equal(preferredLocale(code.toUpperCase(), ["en"]), code);
  assert.equal(preferredLocale("invalid", ["fr-CA", "zh-CN"]), "fr");
  assert.equal(preferredLocale("", ["en-GB", "ru"]), "en");
  assert.equal(preferredLocale("", ["de", "es-MX"]), "es");
  assert.equal(preferredLocale("", []), "en");
  const gateway = await fs.readFile(path.join(sourceRoot, "gateway/index.md"), "utf8");
  const inline = gateway.split("    - |\n")[1].split("\n---")[0].split("\n").map(line => line.slice(6)).join("\n");
  for (const [saved, languages] of [["FR", ["en", "zh"]], ["invalid", ["en-GB", "ru"]], ["fr-CA", ["es-MX", "zh"]], ["", ["zh_CN", "fr"]]]) {
    let target;
    vm.runInNewContext(inline, { URLSearchParams, window: {
      location: { search: "", pathname: "/docs/", hash: "", replace: value => { target = value; } },
      localStorage: { getItem: () => saved, setItem() {} }, navigator: { languages }
    } });
    assert.equal(target, `/docs/${preferredLocale(saved, languages)}/`);
  }
});
test("inventoried fallbacks require English links; current translations permit commands and releases", async () => {
  for (const [relative, entry] of Object.entries(inventory.pages)) {
    if (entry.disposition !== "english-fallback") continue;
    const file = path.join(sourceRoot, relative);
    const body = await fs.readFile(file, "utf8");
    assert.deepEqual(dispositionErrors(relative, body, await readFrontmatter(file)), [], relative);
    for (const match of body.matchAll(/\]\((\/[^)#]+)\)/g)) assert.ok(routes.has(match[1]), match[1]);
  }
  for (const relative of ["fr/build/tutorial.md", "fr/releases/2.0.md"]) {
    const body = "# Current translation\n```sh\nagentplugins author check\n```\n";
    assert.deepEqual(dispositionErrors(relative, body, { localeDisposition: "current-translation" }), []);
    assert.ok(dispositionErrors(relative, body, { localeDisposition: "english-fallback" }).length);
    assert.ok(dispositionErrors(relative, body, {}).length);
  }
});
test("all 192 inventoried original files survive independently of private Git history", async () => {
  const archived = Object.entries(inventory.pages).filter(([, entry]) => entry.disposition === "historical-snapshot");
  assert.equal(archived.length, 192);
  for (const [relative, entry] of archived) {
    const file = path.join(sourceRoot, relative);
    const body = await fs.readFile(file, "utf8");
    const meta = await readFrontmatter(file);
    assert.deepEqual(dispositionErrors(relative, body, meta), [], relative);
    const damaged = body.includes("</details>")
      ? body.replace("</details>", "lost archive")
      : body.replace("locale-historical-source:end -->", "lost archive");
    assert.ok(dispositionErrors(relative, damaged, meta).length);
    // A future adapted translation is current prose outside the immutable
    // archival disclosure; preserving old frontmatter needs no metadata edit.
    entry.currentDisposition = "current-translation";
    try {
      const marker = body.includes("<details><summary>")
        ? "<details><summary>"
        : "<!-- locale-historical-source:start";
      const adapted = body.replace(marker, `\`\`\`sh\nagentplugins author check\n\`\`\`\n\n${marker}`);
      assert.deepEqual(dispositionErrors(relative, adapted, meta), [], relative);
    } finally { delete entry.currentDisposition; }
  }
});
test("localized navigation targets produced source pages; both switcher variants label actual destination language", async () => {
  for (const locale of localeCodes.slice(1)) {
    const config = await fs.readFile(path.join(repoRoot, `website/.vitepress/config/locales.${locale}.ts`), "utf8");
    assert.ok(config.includes(`...journeyNav("${locale}")`));
    assert.deepEqual(journeyNav(locale).map(x => x.link), [`/${locale}/use/`, `/${locale}/build/`]);
    assert.equal(journeyNav(locale)[0].text, journeyLabels[locale][0]);
    for (const match of config.matchAll(/link: "(\/[^\"]+)"/g)) assert.ok(routes.has(match[1]), match[1]);
  }
  const component = await fs.readFile(path.join(repoRoot, "website/.vitepress/theme/components/LocaleSwitcher.vue"), "utf8");
  assert.ok(component.includes("localeDestination(currentEntity.value, locale.code)"));
  assert.equal(component.match(/:hreflang="locale.language"/g)?.length, 2);
  assert.ok(!component.includes("buildLocalePath"));
});
