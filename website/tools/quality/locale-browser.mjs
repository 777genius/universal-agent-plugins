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
  page.setDefaultTimeout(15_000);
  page.setDefaultNavigationTimeout(30_000);
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
    await runLocalizedBuildChecks(page, goto, evidence, artifactsRoot);
    for (const locale of locales.slice(1)) {
      const route = `/${locale}/guide/quickstart`;
      await goto(route);
      const quickstartIdentity = {
        ru: /^Используйте плагины \/ Создавайте плагины/,
        es: /^Usar plugins \/ Crear plugins/,
        fr: /^Utiliser des plugins \/ Créer des plugins/,
        zh: /^使用插件 \/ 构建插件/
      }[locale];
      assert.ok(await page.getByRole("heading", { name: quickstartIdentity }).isVisible(), `${locale}: visible quickstart identity`);
      const id = "build-plugins";
      const fragment = `#${id}`;
      for (const spelling of [route, `${route}.html`]) {
        await goto(spelling + fragment);
        await visibleTarget(id);
        await page.reload({ waitUntil: "networkidle" });
        await visibleTarget(id);
        await goto(`/${locale}/use/`);
        // goto() only waits for networkidle, not VitePress client-router
        // rendering; page.evaluate has no locator auto-wait, so query the
        // node through a real wait instead of racing hydration.
        await page.locator(".vp-doc").first().waitFor({ state: "visible" });
        // A real anchor exercises VitePress's installed client router.
        await page.evaluate(href => {
          const a = document.createElement("a"); a.href = href; a.id = "locale-browser-probe";
          a.textContent = "Build plugins"; document.querySelector(".vp-doc").append(a);
        }, `${base}${spelling}${fragment}`);
        await page.locator("#locale-browser-probe").click();
        await visibleTarget(id);
        await page.goBack({ waitUntil: "networkidle" });
        await page.goForward({ waitUntil: "networkidle" });
        await visibleTarget(id);
        evidence.push({ locale, spelling, fragment, directClientReloadHistory: true });
      }
      await page.setViewportSize({ width: 390, height: 844 });
      await page.locator(".VPLocalNavOutlineDropdown > button").click();
      const outline = page.locator(".VPLocalNavOutlineDropdown .outline-link").first();
      const outlineId = decodeURIComponent((await outline.getAttribute("href")).slice(1));
      await outline.click();
      await visibleTarget(outlineId);
      await page.setViewportSize({ width: 1440, height: 900 });
      evidence.push({ locale, mobileOutline: true });
    }
    for (const variant of ["navbar", "screen"]) {
      await page.setViewportSize(variant === "navbar" ? { width: 1440, height: 900 } : { width: 390, height: 844 });
      for (const current of locales) {
        await goto(`/${current}/use/`);
        assert.equal(await page.locator('link[rel="canonical"]').first().getAttribute("href"), new URL(`${current}/use/`, docsBaseUrl).href);
        for (const code of locales) assert.equal(await page.locator(`link[rel="alternate"][hreflang="${languageTags[code]}"]`).first().getAttribute("href"), new URL(`${code}/use/`, docsBaseUrl).href);
        await inspectSwitcher(`/${current}/use/`, code => `/${code}/use/`, code => code, false);
      }
      await goto("/?gateway=manual");
      await inspectSwitcher("/?gateway=manual", code => `/${code}/`, code => code, false, true);
      const referenceRoute = "/en/api/cli/prepared-authoring-v2-agentplugins-author";
      await inspectSwitcher(referenceRoute, () => referenceRoute, () => "en", true, false, "agentplugins author");
      evidence.push({ variant, fiveCounterparts: true, gatewayHomes: true, versionedEnglishFallback: true });
      async function inspectSwitcher(label, destination, language, fallback, home = false, identityOverride = "") {
        const homeIdentities = { en: "Agent Plugins", ru: "Agent Plugins", es: "Agent Plugins", fr: "Agent Plugins", zh: "Agent Plugins" };
        const useIdentities = { en: "Use plugins", ru: "Использовать плагины", es: "Usar plugins", fr: "Utiliser des plugins", zh: "使用插件" };
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
          const identity = identityOverride || (home ? homeIdentities[actualLanguage] : useIdentities[actualLanguage]);
          const selector = ".vp-doc h1";
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


async function runLocalizedBuildChecks(page, goto, evidence, artifactsRoot) {
  const copy = {
    en: {
      headings: ["Build plugins", "Check a package and its evidence", "Build a stdio MCP package"],
      exclusion: /does not include\s*agentplugins author/i,
      currentSource: /current source builds/i,
      sourceNotice: /Source-build authoring/i,
    },
    ru: {
      headings: ["Создавать плагины", "Проверить пакет и результаты", "Создать MCP-пакет со stdio"],
      exclusion: /нет\s*agentplugins author/i,
      currentSource: /текущей сборке из исходного кода/i,
      sourceNotice: /Средства из исходного кода/i,
    },
    es: {
      headings: ["Crear plugins", "Comprobar un paquete y sus pruebas", "Crear un paquete MCP stdio"],
      exclusion: /no incluye\s*agentplugins author/i,
      currentSource: /compilaciones del código fuente actual/i,
      sourceNotice: /Creación desde el código fuente/i,
    },
    fr: {
      headings: ["Créer des plugins", "Vérifier un paquet et ses preuves", "Créer un paquet MCP stdio"],
      exclusion: /n[’']inclut pas\s*agentplugins author/i,
      currentSource: /compilations du code source actuel/i,
      sourceNotice: /Création depuis le code source/i,
    },
    zh: {
      headings: ["构建插件", "检查包与证据", "构建 stdio MCP 包"],
      exclusion: /不包含\s*agentplugins author/,
      currentSource: /当前源码构建/,
      sourceNotice: /源码创作/,
    },
  };
  const routes = ["build/", "build/checks", "build/mcp-stdio"];
  for (const [locale, expected] of Object.entries(copy)) {
    for (const [index, suffix] of routes.entries()) {
      const route = `/${locale}/${suffix}`;
      const response = await goto(route);
      assert.equal(response?.status(), 200, `${route}: HTTP response`);
      const doc = page.locator(".vp-doc");
      await doc.locator("h1").first().waitFor({ state: "visible", timeout: 15_000 });
      assert.ok(await doc.locator("h1").first().isVisible(), `${route}: visible H1`);
      assert.equal((await doc.locator("h1").first().innerText()).replace(/\u200b/g, "").trim(), expected.headings[index], `${route}: localized H1`);
      const body = (await doc.innerText()).replace(/\s+/g, " ");
      if (index === 0) {
        assert.ok(await doc.locator('a[href$="/agentplugins-v0.1.94"]').isVisible(), `${route}: visible release source`);
        assert.match(body, expected.exclusion, `${route}: installer excludes author`);
        assert.match(body, expected.currentSource, `${route}: current-source authoring`);
        assert.ok(await doc.locator("#build-from-source").isVisible(), `${route}: source setup anchor`);
      } else {
        assert.ok(await doc.locator("p").filter({ hasText: expected.sourceNotice }).first().isVisible(), `${route}: visible source notice`);
        const sourceLink = doc.locator('a[href$="#build-from-source"]').first();
        assert.ok(await sourceLink.isVisible(), `${route}: source setup link`);
        const sourceURL = new URL(await sourceLink.getAttribute("href"), page.url());
        assert.ok(sourceURL.pathname.endsWith(`/${locale}/build/`), `${route}: localized setup destination`);
        for (const prerequisite of ["Linux", "bubblewrap", "cgroup v2"])
          assert.ok(body.includes(prerequisite), `${route}: ${prerequisite} runtime prerequisite`);
        if (index === 1) {
          await sourceLink.click();
          await page.waitForURL(url => url.pathname === sourceURL.pathname && url.hash === "#build-from-source", { timeout: 15_000 });
          await page.locator("#build-from-source").waitFor({ state: "visible", timeout: 15_000 });
          assert.ok(await page.locator("#build-from-source").isVisible(), `${route}: source setup activation`);
        } else {
          for (const prerequisite of ["bootstrap_platform_unsupported", "/usr", "/bin", "/lib", "/lib64"])
            assert.ok(await doc.locator("code").filter({ hasText: new RegExp(`^${prerequisite}$`) }).first().isVisible(), `${route}: ${prerequisite}`);
          await page.setViewportSize({ width: 390, height: 844 });
          await page.reload({ waitUntil: "networkidle" });
          await doc.locator("h1").first().waitFor({ state: "visible", timeout: 15_000 });
          assert.ok(await doc.locator("h1").first().isVisible(), `${route}: mobile H1`);
          assert.equal((await doc.locator("h1").first().innerText()).replace(/\u200b/g, "").trim(), expected.headings[index], `${route}: mobile localized H1`);
          assert.ok(await doc.locator("p").filter({ hasText: expected.sourceNotice }).first().isVisible(), `${route}: mobile source notice`);
          assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), `${route}: mobile page overflow`);
          await page.screenshot({ path: path.join(artifactsRoot, `journey-${locale}-stdio-mobile.png`), fullPage: true });
          await page.setViewportSize({ width: 1440, height: 900 });
        }
      }
      evidence.push({ locale, route, heading: expected.headings[index], responseStatus: response.status(),
        visibleBuildBoundary: true, runtimePrerequisites: index > 0, mobile: index === 2 });
    }
  }
}
