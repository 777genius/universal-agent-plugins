import assert from "node:assert/strict";
import fs from "node:fs/promises";
import path from "node:path";
import { docsBaseUrl } from "../config/site.mjs";

export async function runLocaleSmoke(browser, base, artifactsRoot) {
  const evidence = [];
  const languageTags = { en: "en-US", ru: "ru-RU", es: "es-ES", fr: "fr-FR", zh: "zh-CN" };
  const locales = ["en", "ru", "es", "fr", "zh"];
  const context = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  await context.route("**/*", route => new URL(route.request().url()).origin === new URL(base).origin ? route.continue() : route.abort());
  const page = await context.newPage();
  const errors = [];
  page.on("pageerror", error => errors.push(error.message));
  const goto = route => page.goto(`${base}${route}`, { waitUntil: "networkidle" });
  async function visibleTarget(id) {
    await page.waitForFunction(id => {
      const target = document.getElementById(id);
      return target && target.getClientRects().length > 0;
    }, id);
    assert.ok(await page.locator(`[id=${JSON.stringify(id)}]`).isVisible());
  }
  try {
    for (const locale of locales.slice(1)) {
      const route = `/${locale}/guide/installation`;
      await goto(route);
      assert.ok(await page.locator(".locale-historical-identity").isVisible(), `${locale}: visible page identity`);
      const id = await page.locator(".vp-doc details h2[id]").first().getAttribute("id");
      assert.ok(id);
      const fragment = `#${encodeURIComponent(id)}`;
      for (const spelling of [route, `${route}.html`]) {
        await goto(spelling + fragment);
        await visibleTarget(id);
        await page.reload({ waitUntil: "networkidle" });
        await visibleTarget(id);
        await goto(`/${locale}/use/`);
        // A real anchor exercises VitePress's installed client router.
        await page.evaluate(href => {
          const a = document.createElement("a"); a.href = href; a.id = "locale-browser-probe";
          a.textContent = "Archived fragment"; document.querySelector(".vp-doc").append(a);
        }, `${base}${spelling}${fragment}`);
        await page.locator("#locale-browser-probe").click();
        await visibleTarget(id);
        await page.goBack({ waitUntil: "networkidle" });
        await page.goForward({ waitUntil: "networkidle" });
        await visibleTarget(id);
        evidence.push({ locale, spelling, fragment, directClientReloadHistory: true });
      }
      await goto(route);
      const summary = page.locator(".vp-doc details > summary").first();
      await summary.focus(); await page.keyboard.press("Enter");
      assert.ok(await page.locator(".vp-doc details").first().evaluate(el => el.open));
      await page.keyboard.press("Enter");
      assert.equal(await page.locator(".vp-doc details").first().evaluate(el => el.open), false);
      await page.setViewportSize({ width: 390, height: 844 });
      await page.locator(".VPLocalNavOutlineDropdown > button").click();
      const outline = page.locator(".VPLocalNavOutlineDropdown .outline-link").first();
      const outlineId = decodeURIComponent((await outline.getAttribute("href")).slice(1));
      await outline.click();
      await visibleTarget(outlineId);
      await page.setViewportSize({ width: 1440, height: 900 });
      await goto(route);
      // No current archive has nested details. This explicit browser fixture
      // verifies future nested disclosures without changing historical bytes.
      await page.evaluate(() => {
        const outer = document.querySelector(".vp-doc details");
        outer.insertAdjacentHTML("beforeend", '<details><summary>Nested fixture</summary><h3 id="locale-嵌套">Nested target</h3></details>');
        const a = document.createElement("a"); a.href = "#locale-%E5%B5%8C%E5%A5%97";
        a.id = "nested-locale-probe"; a.textContent = "Nested fragment"; document.querySelector(".vp-doc").append(a);
      });
      await page.locator("#nested-locale-probe").click();
      await visibleTarget("locale-嵌套");
      assert.equal(await page.locator(".vp-doc details[open]").count(), 2);
      assert.equal(await page.evaluate(() => document.activeElement?.id), "locale-嵌套");
      evidence.push({ locale, mobileOutline: true, nestedBrowserFixture: true, keyboardDisclosure: true });
    }
    for (const variant of ["navbar", "screen"]) {
      await page.setViewportSize(variant === "navbar" ? { width: 1440, height: 900 } : { width: 390, height: 844 });
      for (const current of locales) {
        await goto(`/${current}/use/`);
        assert.equal(await page.locator('link[rel="canonical"]').first().getAttribute("href"), new URL(`${current}/use/`, docsBaseUrl).href);
        for (const code of locales) assert.equal(await page.locator(`link[rel="alternate"][hreflang="${languageTags[code]}"]`).first().getAttribute("href"), new URL(`${code}/use/`, docsBaseUrl).href);
        await inspectSwitcher(`/${current}/use/`, code => `/${code}/use/`, code => code, false);
      }
      const fallback = "/en/api/cli/prepared-authoring-v2-plugin-kit-ai";
      await goto(fallback);
      await inspectSwitcher(fallback, () => fallback, () => "en", true);
      await goto("/?gateway=manual");
      await inspectSwitcher("/?gateway=manual", code => `/${code}/`, code => code, false, true);
      evidence.push({ variant, fiveCounterparts: true, englishFallback: true, unknownHomes: true });
      async function inspectSwitcher(label, destination, language, fallback, home = false) {
        const homeIdentities = { en: "plugin-kit-ai", ru: "Документация plugin-kit-ai", es: "Documentación de plugin-kit-ai", fr: "Documentation de plugin-kit-ai", zh: "plugin-kit-ai 文档" };
        for (const [index, code] of locales.entries()) {
          // Every activation starts at its own origin, including same-route
          // English fallbacks; no previous selection supplies the next menu.
          await page.mouse.move(0, 0);
          await goto(label);
          await page.waitForFunction(() => !!document.querySelector("#app")?.__vue_app__);
          assert.equal(new URL(page.url()).pathname, new URL(`${base}${label}`).pathname);
          const widget = page.locator(`.locale-switcher--${variant}`);
          const button = widget.locator("button");
          let bounds;
          if (variant === "navbar") {
            assert.deepEqual(await page.evaluate(() => [scrollX, scrollY]), [0, 0], `${label}: unscrolled origin`);
            const buttonBox = await inViewport(button, `${label}: navbar button`);
            // Raw pointer movement cannot actionability-scroll an overflowing
            // control into view (unlike locator.hover()).
            await page.mouse.move(buttonBox.x + buttonBox.width / 2, buttonBox.y + buttonBox.height / 2);
            await page.waitForFunction(() => document.querySelector('.locale-switcher--navbar button')?.getAttribute("aria-expanded") === "true");
            const menu = widget.locator(".locale-switcher__menu");
            await menu.waitFor({ state: "visible" });
            bounds = { button: buttonBox, menu: await inViewport(menu, `${label}: navbar menu`) };
            assert.deepEqual(await page.evaluate(() => [scrollX, scrollY]), [0, 0]);
          } else {
            await page.locator(".VPNavBarHamburger").click();
            await button.click();
          }
          const links = widget.locator("a");
          assert.equal(await links.count(), 5, label);
          await links.first().waitFor({ state: "visible" });
          for (const [linkIndex, requested] of locales.entries()) {
            const link = links.nth(linkIndex);
            assert.ok(await link.isVisible());
            assert.ok((await link.getAttribute("href")).endsWith(destination(requested)));
            assert.equal(await link.getAttribute("lang"), language(requested));
            assert.equal(await link.getAttribute("hreflang"), language(requested));
            if (fallback && requested !== "en") assert.match(await link.innerText(), /English/);
            if (home) assert.match(await link.innerText(), /Home/);
            if (variant === "navbar") await inViewport(link, `${label}: ${requested} navbar link`);
          }
          const expectedPath = new URL(`${base}${destination(code)}`).pathname;
          const actualLanguage = language(code);
          const identity = home ? homeIdentities[actualLanguage] : fallback ? "plugin-kit-ai" : "Use plugins";
          const selector = home && actualLanguage !== "en" ? ".locale-historical-identity" : ".vp-doc h1";
          await links.nth(index).click();
          await page.waitForURL(url => url.pathname === expectedPath);
          // URL changes can precede the client route render and head update.
          // These predicates also work when a fallback points to the origin.
          await page.waitForFunction(({ selector, identity, lang }) => {
            const el = document.querySelector(selector);
            return el && el.getClientRects().length > 0 &&
              el.textContent.replace(/\u200b/g, "").trim() === identity && document.documentElement.lang === lang;
          }, { selector, identity, lang: languageTags[actualLanguage] });
          assert.equal(new URL(page.url()).pathname, expectedPath);
          assert.ok(await page.locator(selector).first().isVisible());
          assert.equal((await page.locator(selector).first().innerText()).replace(/\u200b/g, "").trim(), identity);
          assert.equal(await page.locator("html").getAttribute("lang"), languageTags[actualLanguage]);
          await page.waitForFunction(variant => document.querySelector(`.locale-switcher--${variant} button`)?.getAttribute("aria-expanded") === "false", variant);
          evidence.push({ variant, origin: label, requested: code, pathname: expectedPath, identity, language: languageTags[actualLanguage], bounds, activated: true });
        }
      }
      async function inViewport(locator, label) {
        const box = await locator.boundingBox();
        const viewport = page.viewportSize();
        assert.ok(box && box.width > 0 && box.height > 0 && box.x >= 0 && box.y >= 0 &&
          box.x + box.width <= viewport.width && box.y + box.height <= viewport.height,
        `${label}: ${JSON.stringify({ box, viewport })}`);
        return box;
      }
    }
    assert.deepEqual(errors, []);
  } finally { await context.close(); }
  for (const scenario of [
    { saved: "FR", languages: ["en", "ru"], expected: "fr" },
    { saved: "invalid", languages: ["es-MX", "en"], expected: "es" },
    { saved: "", languages: ["en-GB", "ru"], expected: "en" },
    { blocked: true, languages: ["zh-CN", "fr"], expected: "zh" },
    { saved: "ru", languages: ["en"], manual: true }
  ]) {
    const ctx = await browser.newContext();
    await ctx.route("**/*", route => new URL(route.request().url()).origin === new URL(base).origin ? route.continue() : route.abort());
    await ctx.addInitScript(s => {
      Object.defineProperty(navigator, "languages", { get: () => s.languages });
      if (s.blocked) Object.defineProperty(window, "localStorage", { get() { throw new Error("owned blocked storage"); } });
      else localStorage.setItem("plugin-kit-ai-docs-locale", s.saved);
    }, scenario);
    const p = await ctx.newPage();
    try {
      await p.goto(`${base}/${scenario.manual ? "?gateway=manual" : ""}`, { waitUntil: "networkidle" });
      if (scenario.manual) { assert.ok(await p.locator(".language-gateway").isVisible()); assert.equal(await p.locator(".language-gateway__card").count(), 5); }
      else { await p.waitForURL(`${base}/${scenario.expected}/`); if (!scenario.blocked) assert.equal(await p.evaluate(() => localStorage.getItem("plugin-kit-ai-docs-locale")), scenario.expected); }
      evidence.push(scenario);
    } finally { await ctx.close(); }
  }
  await fs.writeFile(path.join(artifactsRoot, "locale-browser.json"), JSON.stringify(evidence, null, 2) + "\n");
}
