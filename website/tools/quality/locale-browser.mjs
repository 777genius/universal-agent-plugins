import assert from "node:assert/strict";
import fs from "node:fs/promises";
import path from "node:path";
import { docsLocaleEntries, docsLocales } from "../config/locales.mjs";
import { docsBaseUrl } from "../config/site.mjs";

export async function runLocaleSmoke(browser, base, artifactsRoot) {
  const evidence = [];
  const languageTags = Object.fromEntries(docsLocaleEntries.map(({ code, lang }) => [code, lang]));
  const locales = docsLocales;
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
        uk: /^Використання плагінів \/ Створення плагінів/,
        hi: /^प्लगइन इस्तेमाल करें \/ प्लगइन बनाएँ/,
        ar: /^استخدام الإضافات \/ بناء الإضافات/,
        pt: /^Usar plugins \/ Criar plugins/,
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
      evidence.push({ variant, nineCounterparts: true, gatewayHomes: true, versionedEnglishFallback: true });
      async function inspectSwitcher(label, destination, language, fallback, home = false, identityOverride = "") {
        const homeIdentities = Object.fromEntries(locales.map(code => [code, "Agent Plugins"]));
        const useIdentities = { uk: "Використання плагінів", hi: "प्लगइन इस्तेमाल करें", ar: "استخدام الإضافات", pt: "Usar plugins", en: "Use plugins", ru: "Использовать плагины", es: "Usar plugins", fr: "Utiliser des plugins", zh: "使用插件" };
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
          assert.equal(await links.count(), locales.length, label);
          await links.first().waitFor({ state: "visible" });
          for (const [linkIndex, requested] of locales.entries()) {
            const link = links.nth(linkIndex);
            assert.ok(await link.isVisible());
            assert.ok((await link.getAttribute("href")).endsWith(destination(requested)));
            assert.equal(await link.getAttribute("lang"), language(requested));
            assert.equal(await link.getAttribute("hreflang"), language(requested));
            if (fallback && requested !== "en") assert.match(await link.innerText(), /English/);
            if (home) assert.ok((await link.innerText()).includes(docsLocaleEntries.find(locale => locale.code === requested).home));
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
    await runNativeSearchAndNavigation(page, goto, evidence);
    assert.deepEqual(errors, []);
  } finally { await context.close(); }
  for (const scenario of [
    { saved: "FR", languages: ["en", "ru"], expected: "fr" },
    { saved: "invalid", languages: ["es-MX", "en"], expected: "es" },
    { saved: "", languages: ["en-GB", "ru"], expected: "en" },
    { blocked: true, languages: ["zh-CN", "fr"], expected: "zh" },
    { saved: "AR", languages: ["en"], expected: "ar" },
    { saved: "invalid", languages: ["uk-UA", "en"], expected: "uk" },
    { saved: "", languages: ["hi-IN", "en"], expected: "hi" },
    { saved: "", languages: ["pt-PT", "en"], expected: "pt" },
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
      if (scenario.manual) { assert.ok(await p.locator(".language-gateway").isVisible()); assert.equal(await p.locator(".language-gateway__card").count(), locales.length); }
      else { await p.waitForURL(`${base}/${scenario.expected}/`); if (!scenario.blocked) assert.equal(await p.evaluate(() => localStorage.getItem("plugin-kit-ai-docs-locale")), scenario.expected); }
      evidence.push(scenario);
    } finally { await ctx.close(); }
  }
  await fs.writeFile(path.join(artifactsRoot, "locale-browser.json"), JSON.stringify(evidence, null, 2) + "\n");
}


async function runLocalizedBuildChecks(page, goto, evidence, artifactsRoot) {
  const copy = {
    uk: {
      headings: ["Створення плагінів", "Перевірка пакета та його доказів", "Створення пакета stdio MCP"],
      exclusion: /не (?:містить|включає)\s*agentplugins author/i,
      currentSource: /(?:поточн|вихідн)/i,
      sourceNotice: /Розробка у збірці з вихідного коду/i,
    },
    hi: {
      headings: ["प्लगइन बनाएँ", "पैकेज और उसके प्रमाण जाँचें", "Stdio MCP पैकेज बनाएँ"],
      exclusion: /agentplugins author\s*शामिल नहीं है/,
      currentSource: /सोर्स/,
      sourceNotice: /सोर्स बिल्ड में पैकेज निर्माण/,
    },
    ar: {
      headings: ["بناء الإضافات", "فحص الحزمة وأدلتها", "بناء حزمة MCP عبر stdio"],
      exclusion: /لا يتضمن إصدار المثبّت الحالي\s*agentplugins author/,
      currentSource: /المصدر/,
      sourceNotice: /التأليف من المصدر/,
    },
    pt: {
      headings: ["Criar plugins", "Verificar um pacote e suas evidências", "Criar um pacote MCP stdio"],
      exclusion: /não inclui\s*agentplugins author/i,
      currentSource: /código.fonte atual/i,
      sourceNotice: /Criação a partir do código-fonte/i,
    },
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
        assert.ok(await doc.locator('a[href$="/agentplugins-v0.1.95"]').isVisible(), `${route}: visible release source`);
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


async function runNativeSearchAndNavigation(page, goto, evidence) {
  const queries = { en: "plugins", ru: "плагины", uk: "плагінів", zh: "插件", es: "plugins", hi: "प्लगइन", ar: "الإضافات", pt: "plugins", fr: "plugins" };
  await page.setViewportSize({ width: 1440, height: 900 });
  for (const { code, dir } of docsLocaleEntries) {
    await goto(`/${code}/use/`);
    assert.equal(await page.locator("html").getAttribute("dir"), dir, `${code}: document direction`);
    const skip = page.locator(".VPSkipLink");
    await skip.focus();
    assert.ok(await skip.isVisible(), `${code}: skip link visible on focus`);
    if (code !== "en") assert.notEqual((await skip.innerText()).trim(), "Skip to content", `${code}: localized skip link`);
    const appearance = page.locator(".VPNavBarAppearance .VPSwitchAppearance");
    await page.waitForFunction(() => !!document.querySelector(".VPNavBarAppearance .VPSwitchAppearance")?.getAttribute("title"));
    const themeTitles = [await appearance.getAttribute("title")];
    await appearance.click();
    await page.waitForFunction(previous => document.querySelector(".VPNavBarAppearance .VPSwitchAppearance")?.getAttribute("title") !== previous, themeTitles[0]);
    themeTitles.push(await appearance.getAttribute("title"));
    if (code !== "en") for (const title of themeTitles) assert.ok(title && !/^Switch to (?:dark|light) theme$/.test(title), `${code}: localized appearance title`);
    await page.locator(".VPNavBarSearchButton").click();
    const input = page.locator("#localsearch-input");
    await input.waitFor({ state: "visible" });
    await input.fill(queries[code]);
    const result = page.locator(`.VPLocalSearchBox a[href*="/${code}/"]`).first();
    await result.waitFor({ state: "visible" });
    assert.ok((await result.getAttribute("href")).includes(`/${code}/`), `${code}: native result destination`);
    await page.keyboard.press("Escape");
    await input.waitFor({ state: "hidden" });
    evidence.push({ locale: code, query: queries[code], nativeSearch: true });
  }
  for (const width of [1024, 1280, 1440]) {
    await page.setViewportSize({ width, height: 900 });
    for (const { code } of docsLocaleEntries) {
      await goto(`/${code}/build/`);
      const bounds = await page.locator(".locale-switcher--navbar button").boundingBox();
      assert.ok(bounds && bounds.x >= 0 && bounds.x + bounds.width <= width, `${code}/${width}: language control fits`);
      assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), `${code}/${width}: page overflow`);
      if (code === "ar" && width === 1024) {
        const padding = await page.locator(".VPLocalNav.has-sidebar").evaluate(el => ({ left: getComputedStyle(el).paddingLeft, right: getComputedStyle(el).paddingRight }));
        assert.equal(padding.left, "0px", "Arabic tablet local navigation has no left sidebar offset");
        assert.ok(parseFloat(padding.right) > 0, "Arabic tablet local navigation follows the right sidebar");
      }
    }
  }
  await page.setViewportSize({ width: 1440, height: 900 });
  await goto("/en/use/");
  const pointerWidget = page.locator(".locale-switcher--navbar");
  const pointerButton = await pointerWidget.locator("button").boundingBox();
  assert.ok(pointerButton, "Pointer language button has geometry");
  await page.mouse.move(pointerButton.x + pointerButton.width / 2, pointerButton.y + pointerButton.height / 2);
  await page.waitForFunction(() => document.querySelector(".locale-switcher--navbar button")?.getAttribute("aria-expanded") === "true");
  const pointerTarget = await pointerWidget.locator("a").first().boundingBox();
  assert.ok(pointerTarget, "Pointer language target has geometry");
  await page.mouse.move(pointerTarget.x + pointerTarget.width / 2, pointerTarget.y + pointerTarget.height / 2, { steps: 24 });
  assert.equal(await pointerWidget.locator("button").getAttribute("aria-expanded"), "true", "Language menu stays open along the actual pointer path");
  assert.ok(await pointerWidget.locator("a").first().isVisible(), "Language menu remains usable after pointer travel");
  await page.mouse.move(0, 0);
  evidence.push({ humanPointerLanguageMenu: true });
  await page.setViewportSize({ width: 1024, height: 900 });
  await goto("/uk/build/");
  await page.locator(".VPNavBarHamburger").click();
  assert.ok(await page.locator('.VPNavScreen a[href$="/uk/use/"]').isVisible(), "Tablet navigation exposes the localized use journey");
  await page.setViewportSize({ width: 1440, height: 900 });
  await goto("/en/build/#build-from-source");
  const widget = page.locator(".locale-switcher--navbar");
  const button = widget.locator("button");
  await button.focus();
  await button.press("Enter");
  assert.equal(await button.getAttribute("aria-expanded"), "true");
  for (const { code } of docsLocaleEntries) assert.ok((await widget.locator(`a[hreflang="${code}"]`).getAttribute("href")).endsWith(`#build-from-source`));
  await button.press("Tab");
  assert.ok(await widget.locator("a").first().evaluate(el => el === document.activeElement));
  await page.keyboard.press("Escape");
  assert.equal(await button.getAttribute("aria-expanded"), "false");
  await button.focus();
  await button.press("Enter");
  const arabic = widget.locator('a[hreflang="ar"]');
  await arabic.focus();
  await arabic.press("Enter");
  await page.waitForURL(url => url.pathname.endsWith("/ar/build/") && url.hash === "#build-from-source");
  await page.locator("#build-from-source").waitFor({ state: "visible" });
  await page.waitForFunction(() => document.documentElement.dir === "rtl");
  assert.equal(await page.locator("pre").first().evaluate(el => getComputedStyle(el).direction), "ltr");
  await page.setViewportSize({ width: 390, height: 844 });
  await goto("/ar/use/manage");
  assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), "Arabic mobile page overflow");
  await page.locator(".VPLocalNav .menu").click();
  const sidebar = page.locator(".VPSidebar.open");
  await sidebar.waitFor({ state: "visible" });
  // Visibility begins while the opening slide transition is still running.
  await sidebar.evaluate(async (element) => {
    await Promise.all(element.getAnimations().map((animation) => animation.finished));
  });
  const sideBounds = await sidebar.boundingBox();
  assert.ok(sideBounds && sideBounds.x >= 0 && sideBounds.x + sideBounds.width <= 390, "Arabic mobile sidebar fits");
  evidence.push({ keyboardLanguageMenu: true, attestedFragment: true, rtlClientNavigation: true, arabicMobileSidebar: true, arabicMobileSidebarBounds: sideBounds });
}
