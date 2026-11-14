import fs from "node:fs/promises";
import path from "node:path";

const routePattern = /^\/(?:[a-zA-Z0-9_-]+\/)*[a-zA-Z0-9_-]*$/;
export const htmlPath = (route) => `${route.replace(/^\//, "")}${route.endsWith("/") ? "index" : ""}.html`;

export function buildRedirects(inventory, pageRoutes) {
  if (inventory.schema !== "docs-aliases-v1") throw new Error("Unknown alias inventory");
  const routes = new Set(pageRoutes);
  const result = { ...inventory.aliases };
  if (inventory.englishAliases) {
    for (const route of [...routes].sort()) {
      if (!route.startsWith("/en/") || inventory.excludedEnglishPages.includes(route)) continue;
      const alias = route.slice(3);
      if (!routes.has(alias) && !(alias in result)) result[alias] = route;
    }
  }
  for (const [alias, target] of Object.entries(result)) {
    if (!routePattern.test(alias) || !routePattern.test(target) || alias === "/" ||
        routes.has(alias) || !routes.has(target) || target in result) {
      throw new Error(`Invalid alias, collision, missing page or chain: ${alias} -> ${target}`);
    }
  }
  return Object.fromEntries(Object.entries(result).sort(([a], [b]) => a.localeCompare(b)));
}

export function createRedirectDocument(targetUrl) {
  const target = new URL(targetUrl);
  if (!/^https?:$/.test(target.protocol) || target.hash || target.search) throw new Error("Invalid canonical alias target");
  const escaped = target.href.replaceAll("&", "&amp;").replaceAll('"', "&quot;").replaceAll("<", "&lt;");
  const literal = JSON.stringify(target.href).replaceAll("<", "\\u003c");
  // Execute before the no-script refresh. Same-page aliases preserve every
  // fragment (including percent encoding) and the incoming query verbatim.
  return `<!doctype html>
<html lang="en-US"><head><meta charset="utf-8">
<meta name="robots" content="noindex,follow">
<link rel="canonical" href="${escaped}">
<script>location.replace(${literal} + location.search + location.hash);</script>
<noscript><meta http-equiv="refresh" content="0; url=${escaped}"></noscript>
<title>Redirecting...</title></head><body><a href="${escaped}">Continue to the canonical page</a></body></html>
`;
}

export async function emitRedirects(distRoot, redirects, docsBaseUrl) {
  // Validate all destinations before creating any aliases. Existing HTML is
  // never overwritten, so source pages cannot disappear behind redirects.
  for (const [alias, target] of Object.entries(redirects)) {
    if (!routePattern.test(alias) || !routePattern.test(target) || alias === "/" || target in redirects)
      throw new Error(`Unsafe alias ${alias}`);
    const destination = await fs.readFile(path.join(distRoot, htmlPath(target)), "utf8");
    if (!/<(?:html|!doctype)/i.test(destination)) throw new Error(`Missing rendered target ${target}`);
    try {
      await fs.access(path.join(distRoot, htmlPath(alias)));
      throw new Error(`Alias would overwrite output: ${alias}`);
    } catch (error) {
      if (error.code !== "ENOENT") throw error;
    }
  }
  for (const [alias, target] of Object.entries(redirects)) {
    const file = path.join(distRoot, htmlPath(alias));
    await fs.mkdir(path.dirname(file), { recursive: true });
    await fs.writeFile(file, createRedirectDocument(new URL(target.slice(1), docsBaseUrl).href));
  }
}
