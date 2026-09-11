import { runSiteConsumerSmoke } from "./site-consumer-browser.mjs";
import fs from "node:fs/promises";
import http from "node:http";
import path from "node:path";
import process from "node:process";
import { chromium } from "playwright";
import { docsBasePath, runtimeRoot, websiteRoot } from "../config/site.mjs";
import { ensureDir, rimraf } from "../lib/fs.mjs";

const artifactsRoot = path.join(websiteRoot, ".playwright-artifacts");
const serverRoot = path.join(artifactsRoot, "server-root");
const distRoot = path.join(websiteRoot, "dist");
const errors = [];

await ensureDir(artifactsRoot);
await rimraf(serverRoot);
await ensureDir(serverRoot);
const docsMountPath = path.join(serverRoot, docsBasePath.replace(/^\/|\/$/g, ""));
await ensureDir(path.dirname(docsMountPath));
await fs.symlink(distRoot, docsMountPath, "dir");

const server = createStaticServer(serverRoot);

try {
  await new Promise((resolve, reject) => {
    server.once("error", reject);
    server.listen(0, "127.0.0.1", () => {
      server.off("error", reject);
      resolve(undefined);
    });
  });
  const address = server.address();
  if (!address || typeof address === "string") {
    throw new Error("Static docs server did not expose a numeric port.");
  }
  const base = `http://127.0.0.1:${address.port}${docsBasePath.replace(/\/$/, "")}`;
  await waitForServer(`${base}/`);
  const browser = await chromium.launch({ headless: true });
  try {
    await runSmoke(browser, base);
    await runSiteConsumerSmoke(browser, base, artifactsRoot);
  } finally {
    await browser.close();
  }
} finally {
  await new Promise((resolve, reject) => server.close((error) => (error ? reject(error) : resolve())));
}

if (errors.length > 0) {
  for (const error of errors) {
    console.error(error);
  }
  process.exit(1);
}

async function runSmoke(browser, base) {
  const context = await browser.newContext({ viewport: { width: 1440, height: 900 }, locale: "en-US" });
  const page = await context.newPage();
  page.on("pageerror", (error) => {
    errors.push(`Browser pageerror: ${error.message}`);
  });
  page.on("console", (msg) => {
    if (msg.type() === "error") {
      errors.push(`Browser console error: ${msg.text()}`);
    }
  });

  const desktopChecks = [
    ["en home", `${base}/en/`, "plugin-kit-ai"],
    ["ru home", `${base}/ru/`, "plugin-kit-ai"],
    ["what you can build", `${base}/en/guide/what-you-can-build`, "One Repo, Many Supported Outputs"],
    ["one project multiple targets", `${base}/en/guide/one-project-multiple-targets`, "The Short Rule"],
    ["choose a target", `${base}/en/guide/choose-a-target`, "Target Directory"],
    ["why plugin-kit-ai", `${base}/en/concepts/why-plugin-kit-ai`, "What It Gives You"],
    ["authoring architecture", `${base}/en/concepts/authoring-architecture`, "The Core Shape"],
    ["choosing runtime", `${base}/en/concepts/choosing-runtime`, "Safe Default Matrix"],
    ["target model", `${base}/en/concepts/target-model`, "Quick Rule"],
    ["production readiness", `${base}/en/guide/production-readiness`, "Pick The Right Path On Purpose"],
    ["python runtime guide", `${base}/en/guide/python-runtime`, "Build A Python Runtime Plugin"],
    ["team ready plugin", `${base}/en/guide/team-ready-plugin`, "Outcome"],
    ["examples and recipes", `${base}/en/guide/examples-and-recipes`, "Production Plugin Examples"],
    ["choose starter", `${base}/en/guide/choose-a-starter`, "Starter Matrix"],
    ["choose delivery model", `${base}/en/guide/choose-delivery-model`, "The Two Modes"],
    ["bundle handoff", `${base}/en/guide/bundle-handoff`, "What It Covers"],
    ["package and workspace targets", `${base}/en/guide/package-and-workspace-targets`, "The Short Rule"],
    ["how to publish plugins", `${base}/en/guide/how-to-publish-plugins`, "Quick Comparison"],
    ["ci integration", `${base}/en/guide/ci-integration`, "The Minimal CI Gate"],
    ["faq", `${base}/en/reference/faq`, "Should I Start With Go, Python, Or Node?"],
    ["glossary", `${base}/en/reference/glossary`, "Authored State"],
    ["repository standard", `${base}/en/reference/repository-standard`, "The Main Rule"],
    ["support boundary", `${base}/en/reference/support-boundary`, "Safe Defaults"],
    ["troubleshooting", `${base}/en/reference/troubleshooting`, "The CLI Installs But Does Not Run"],
    ["api home", `${base}/en/api/`, "API Surfaces"],
    ["cli reference", `${base}/en/api/cli/`, "CLI Reference"],
    ["go sdk", `${base}/en/api/go-sdk/`, "Go SDK"],
    ["node runtime", `${base}/en/api/runtime-node/`, "Node Runtime"],
    ["python runtime", `${base}/en/api/runtime-python/`, "Python Runtime"],
    ["target support", `${base}/en/reference/target-support`, "Target Support"],
    ["latest release", `${base}/en/releases/v1-1-2`, "Why This Release Matters"]
  ];

  for (const [name, url, expected] of desktopChecks) {
    const response = await page.goto(url, { waitUntil: "networkidle" });
    if (!response || response.status() !== 200) {
      errors.push(`${name}: expected 200 but got ${response?.status() ?? "no response"}`);
      continue;
    }
    const body = await page.textContent("body");
    if (!body?.includes(expected)) {
      errors.push(`${name}: expected body to include "${expected}"`);
    }
    await assertMermaidRendered(page, name);
  }

  await page.goto(`${base}/?gateway=1`, { waitUntil: "networkidle" });
  const manualGatewayBody = await page.textContent("body");
  if (!manualGatewayBody?.includes("Choose your language")) {
    errors.push("Manual gateway view did not generate the language chooser.");
  }

  await page.goto(`${base}/`, { waitUntil: "networkidle" });
  if (!page.url().includes("/en/")) {
    errors.push(`Root gateway did not auto-redirect to English. Final URL: ${page.url()}`);
  }

  await page.goto(`${base}/en/`, { waitUntil: "networkidle" });
  await page.locator(".VPNavBarMenu").getByRole("link", { name: "API", exact: true }).click();
  if (!page.url().includes("/en/api/")) {
    errors.push("Top navigation API link did not navigate to /en/api/.");
  }

  await page.goto(`${base}/en/`, { waitUntil: "networkidle" });
  const editLink = page.getByRole("link", { name: "Edit this page", exact: true });
  if ((await editLink.count()) < 1) {
    errors.push("Hand-authored page is missing the visible edit link.");
  }

  await page.goto(`${base}/en/api/cli/plugin-kit-ai`, { waitUntil: "networkidle" });
  const generatedEditLink = page.getByRole("link", { name: "Edit this page", exact: true });
  if ((await generatedEditLink.count()) > 0) {
    errors.push("Generated CLI page should not show a hand-authored edit link.");
  }
  const sourceLink = page.getByRole("link", { name: "Source", exact: true });
  if ((await sourceLink.count()) < 1) {
    errors.push("Generated CLI page is missing the Source link.");
  }

  await page.goto(`${base}/?gateway=1`, { waitUntil: "networkidle" });
  const robots = await page.evaluate(() => document.querySelector('meta[name="robots"]')?.getAttribute("content"));
  if (robots !== "noindex,follow") {
    errors.push(`Gateway robots meta mismatch: ${robots}`);
  }
  await page.locator('.language-gateway__card[href$="/en/"]').click();
  await page.goto(`${base}/`, { waitUntil: "networkidle" });
  if (!page.url().includes("/en/")) {
    errors.push(`Saved locale did not reopen English from root. Final URL: ${page.url()}`);
  }

  const notFoundResponse = await fetch(`${base}/missing-page`);
  if (notFoundResponse.status !== 404) {
    errors.push(`Unknown route should return 404 but got ${notFoundResponse.status}`);
  }
  const notFoundBody = await notFoundResponse.text();
  if (!notFoundBody.includes("Page Not Found")) {
    errors.push("404 response did not include the expected fallback markup.");
  }

  const ruContext = await browser.newContext({ viewport: { width: 1440, height: 900 }, locale: "ru-RU" });
  const ruPage = await ruContext.newPage();
  const ruResponse = await ruPage.goto(`${base}/`, { waitUntil: "networkidle" });
  if (!ruResponse || ruResponse.status() !== 200) {
    errors.push(`Russian auto-redirect root expected 200 but got ${ruResponse?.status() ?? "no response"}`);
  } else if (!ruPage.url().includes("/ru/")) {
    errors.push(`Root gateway did not auto-redirect to Russian. Final URL: ${ruPage.url()}`);
  }
  await ruPage.close();
  await ruContext.close();

  const mobileContext = await browser.newContext({ viewport: { width: 390, height: 844 }, locale: "en-US" });
  const mobilePage = await mobileContext.newPage();
  const mobileChecks = [
    ["gateway mobile manual", `${base}/?gateway=1`, "Choose your language"],
    ["cli mobile", `${base}/en/api/cli/`, "CLI Reference"]
  ];

  for (const [name, url, expected] of mobileChecks) {
    const response = await mobilePage.goto(url, { waitUntil: "networkidle" });
    if (!response || response.status() !== 200) {
      errors.push(`${name}: expected 200 but got ${response?.status() ?? "no response"}`);
      continue;
    }
    const body = await mobilePage.textContent("body");
    if (!body?.includes(expected)) {
      errors.push(`${name}: expected body to include "${expected}"`);
    }
  }

  await mobilePage.close();
  await mobileContext.close();
  await page.close();
  await context.close();

  const runtimeExists = await fs
    .access(path.join(runtimeRoot, "en", "api", "runtime-node", "index.md"))
    .then(() => true)
    .catch(() => false);
  if (!runtimeExists) {
    errors.push("Assembled runtime source is missing Node runtime API index.");
  }
}

async function assertMermaidRendered(page, name) {
  const diagramCount = await page.locator(".mermaid-diagram").count();
  if (diagramCount < 1) {
    return;
  }

  const errorBox = page.locator(".mermaid-diagram__error").first();
  if ((await errorBox.count()) > 0) {
    const message = (await errorBox.textContent())?.trim() || "unknown Mermaid render error";
    errors.push(`${name}: ${message}`);
    return;
  }

  const svgCount = await page.locator(".mermaid-diagram__surface svg").count();
  if (svgCount < 1) {
    errors.push(`${name}: expected Mermaid SVG output.`);
  }
}

async function waitForServer(url, attempts = 30) {
  for (let i = 0; i < attempts; i += 1) {
    try {
      const response = await fetch(url);
      if (response.ok) {
        return;
      }
    } catch {
      // retry
    }
    await new Promise((resolve) => setTimeout(resolve, 250));
  }
  throw new Error(`Local docs server did not become ready: ${url}`);
}

function createStaticServer(rootDir) {
  return http.createServer(async (req, res) => {
    try {
      const filePath = await resolveRequestPath(rootDir, req.url || "/");
      const body = await fs.readFile(filePath);
      res.writeHead(200, { "Content-Type": contentType(filePath) });
      res.end(body);
    } catch {
      try {
        const fallback404 = path.join(rootDir, docsBasePath.replace(/^\/|\/$/g, ""), "404.html");
        const body = await fs.readFile(fallback404);
        res.writeHead(404, { "Content-Type": "text/html; charset=utf-8" });
        res.end(body);
      } catch {
        res.writeHead(404, { "Content-Type": "text/plain; charset=utf-8" });
        res.end("Not Found");
      }
    }
  });
}

async function resolveRequestPath(rootDir, requestUrl) {
  const pathname = new URL(requestUrl, "http://127.0.0.1").pathname;
  const normalizedBase = docsBasePath.endsWith("/") ? docsBasePath : `${docsBasePath}/`;
  if (!pathname.startsWith(normalizedBase)) {
    throw new Error(`Request path escaped docs base: ${pathname}`);
  }

  const relativePath = pathname.slice(normalizedBase.length);
  const candidates = [];
  if (!relativePath || relativePath.endsWith("/")) {
    candidates.push(path.join(rootDir, normalizedBase.replace(/^\/|\/$/g, ""), relativePath, "index.html"));
  } else {
    candidates.push(path.join(rootDir, normalizedBase.replace(/^\/|\/$/g, ""), relativePath));
    candidates.push(path.join(rootDir, normalizedBase.replace(/^\/|\/$/g, ""), `${relativePath}.html`));
    candidates.push(path.join(rootDir, normalizedBase.replace(/^\/|\/$/g, ""), relativePath, "index.html"));
  }

  for (const candidate of candidates) {
    try {
      const stat = await fs.stat(candidate);
      if (stat.isFile()) {
        return candidate;
      }
    } catch {
      // continue
    }
  }

  throw new Error(`No file for ${requestUrl}`);
}

function contentType(filePath) {
  if (filePath.endsWith(".html")) return "text/html; charset=utf-8";
  if (filePath.endsWith(".css")) return "text/css; charset=utf-8";
  if (filePath.endsWith(".js")) return "text/javascript; charset=utf-8";
  if (filePath.endsWith(".json")) return "application/json; charset=utf-8";
  if (filePath.endsWith(".svg")) return "image/svg+xml";
  if (filePath.endsWith(".woff2")) return "font/woff2";
  if (filePath.endsWith(".png")) return "image/png";
  return "application/octet-stream";
}
