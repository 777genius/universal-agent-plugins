import assert from "node:assert/strict";
import fs from "node:fs/promises";
import path from "node:path";
import { docsBaseUrl, generatedRegistryPaths } from "../config/site.mjs";

// Invoked by the real built-site smoke, never by the dependency-free fixture
// tests. Canonical redirects are fulfilled from the disposable local server;
// this test must not inspect or publish a remote preparation site.
export async function runSiteConsumerSmoke(browser, base, artifactsRoot) {
  const context = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  const evidence = [];
  const failures = [];
  await context.route("**/*", async (route) => {
    const url = route.request().url();
    if (url.startsWith(docsBaseUrl)) {
      const response = await route.fetch({ url: `${base}/${url.slice(docsBaseUrl.length)}` });
      return route.fulfill({ response });
    }
    if (new URL(url).origin === new URL(base).origin) return route.continue();
    return route.abort();
  });
  const page = await context.newPage();
  page.on("pageerror", (error) => failures.push(error.message));
  try {
    for (const locale of ["en", "ru", "es", "fr", "zh"]) {
      await page.goto(`${base}/${locale}/`, { waitUntil: "networkidle" });
      for (const [label, route] of [["Use plugins", "/en/use/"], ["Build plugins", "/en/build/"]]) {
        const link = page.locator(".VPNavBarMenu").getByRole("link", { name: new RegExp(`^${label}`) });
        assert.equal(await link.count(), 1);
        assert.ok((await link.getAttribute("href")).endsWith(route));
      }
    }
    for (const root of ["plugin-kit-ai", "agentplugins-author"]) {
      await page.goto(`${base}/en/api/cli/prepared-authoring-v2-${root}`, { waitUntil: "networkidle" });
      assert.match(await page.locator(".vp-doc").innerText(), /not released|not a public release/i);
      await page.screenshot({ path: path.join(artifactsRoot, `d2b-${root}.png`), fullPage: true });
    }
    const redirects = JSON.parse(await fs.readFile(generatedRegistryPaths.redirects, "utf8"));
    const samples = ["/use/", "/build/", "/en/legacy/v1/cli/", "/api/cli/plugin-kit-ai-generate",
      "/api/cli/prepared-authoring-v2-plugin-kit-ai-init", "/api/cli/prepared-authoring-v2-agentplugins-author-init"];
    for (const alias of samples) {
      const target = redirects[alias];
      assert.ok(target, `Missing built alias inventory: ${alias}`);
      await page.goto(`${base}${target}`, { waitUntil: "networkidle" });
      const id = await page.locator(".vp-doc h2[id], .vp-doc h3[id]").first().getAttribute("id");
      assert.ok(id, `Missing actual rendered heading: ${target}`);
      const fragment = `#${encodeURIComponent(id)}`;
      for (const spelling of [alias, alias.endsWith("/") ? `${alias}index.html` : `${alias}.html`]) {
        const response = await page.goto(`${base}${spelling}?proof=d2b${fragment}`, { waitUntil: "networkidle" });
        assert.equal(response?.status(), 200);
        const expected = new URL(target.slice(1), docsBaseUrl).href + `?proof=d2b${fragment}`;
        await page.waitForURL(expected);
        assert.ok(await page.evaluate((anchor) => !!document.getElementById(anchor), id));
        assert.equal(await page.locator('link[rel="canonical"]').getAttribute("href"),
          new URL(target.slice(1), docsBaseUrl).href);
        evidence.push({ alias: spelling, target, fragment, finalURL: page.url(), headingExists: true });
      }
    }
    assert.deepEqual(failures, []);
    await fs.writeFile(path.join(artifactsRoot, "d2b-browser.json"), JSON.stringify({
      scope: "Actual VitePress output served locally, canonical origin intercepted locally", evidence
    }, null, 2) + "\n");
  } finally {
    await context.close();
  }
}
