import test from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs/promises";
import path from "node:path";
import { execFileSync } from "node:child_process";
import { sourceRoot, repoRoot } from "../config/site.mjs";
import { listMarkdownFiles } from "../lib/fs.mjs";
import { readFrontmatter } from "../lib/site-model.mjs";
import { localeCodes, localeDestination, localeFromPath, preferredLocale } from "../../.vitepress/theme/components/locale-routes.mjs";
const base = "bb08ee00f2734baa04265a5afee115772a815cf2";
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
    assert.equal(target.fallback, false, `${id}/${locale}`);
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
test("gateway honors saved locale then browser preference order including English", () => {
  for (const code of localeCodes) assert.equal(preferredLocale(code.toUpperCase(), ["en"]), code);
  assert.equal(preferredLocale("invalid", ["fr-CA", "zh-CN"]), "fr");
  assert.equal(preferredLocale("", ["en-GB", "ru"]), "en");
  assert.equal(preferredLocale("", ["de", "es-MX"]), "es");
  assert.equal(preferredLocale("", []), "en");
});
test("52 minimal fallbacks link to exact English counterparts without copied commands", async () => {
  let count = 0;
  for (const [route, file] of routes) {
    if (!/^\/(ru|es|fr|zh)\/(use|build|legacy\/v1)\//.test(route)) continue;
    const body = await fs.readFile(file, "utf8");
    const english = route.replace(/^\/[^/]+\//, "/en/");
    assert.ok(body.includes(`](${english})`), route);
    for (const match of body.matchAll(/\]\((\/[^)#]+)\)/g)) assert.ok(routes.has(match[1]), match[1]);
    assert.ok(!body.includes("```"));
    count++;
  }
  assert.equal(count, 52);
});
test("all original localized content and deep-link headings survive verbatim inside historical details", async () => {
  let count = 0;
  for (const [route, file] of routes) {
    if (!/^\/(ru|es|fr|zh)\//.test(route) || /^\/(ru|es|fr|zh)\/(use|build|legacy\/v1)\//.test(route)) continue;
    const original = execFileSync("git", ["show", `${base}:${path.relative(repoRoot, file)}`], { cwd: repoRoot, encoding: "utf8" });
    const current = await fs.readFile(file, "utf8");
    const end = original.indexOf("\n---", 4) + 4;
    assert.equal(current.slice(0, end), original.slice(0, end));
    const archive = current.slice(current.indexOf("</summary>") + "</summary>".length + 1, -"\n</details>\n".length);
    assert.equal(archive, original.slice(end), route);
    assert.ok(current.includes("1.2.4"));
    count++;
  }
  assert.equal(count, 192);
});
test("localized navigation targets produced source pages; both switcher variants label actual destination language", async () => {
  for (const locale of localeCodes.slice(1)) {
    const config = await fs.readFile(path.join(repoRoot, `website/.vitepress/config/locales.${locale}.ts`), "utf8");
    assert.ok(config.includes(`link: "/${locale}/use/"`));
    assert.ok(config.includes(`link: "/${locale}/build/"`));
    for (const match of config.matchAll(/link: "(\/[^\"]+)"/g)) assert.ok(routes.has(match[1]), match[1]);
  }
  const component = await fs.readFile(path.join(repoRoot, "website/.vitepress/theme/components/LocaleSwitcher.vue"), "utf8");
  assert.ok(component.includes("localeDestination(currentEntity.value, locale.code)"));
  assert.equal(component.match(/:hreflang="locale.language"/g)?.length, 2);
  assert.ok(!component.includes("buildLocalePath"));
});
