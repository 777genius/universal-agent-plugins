import { emitRedirects } from "./lib/redirects.mjs";
import fs from "node:fs";
import path from "node:path";
import { docsBaseUrl, websiteRoot, generatedRegistryPaths } from "./config/site.mjs";

const distRoot = path.join(websiteRoot, "dist");
const englishRoot = path.join(distRoot, "en");
const sitemapUrl = new URL("sitemap.xml", docsBaseUrl).toString();

fs.writeFileSync(
  path.join(distRoot, "robots.txt"),
  ["User-agent: *", "Allow: /", `Sitemap: ${sitemapUrl}`, ""].join("\n")
);

const redirects = JSON.parse(fs.readFileSync(generatedRegistryPaths.redirects, "utf8"));
await emitRedirects(distRoot, redirects, docsBaseUrl);

// Preserved former alias helpers; current source-owned emission is above.
function listHtmlFiles(rootDir) {
  if (!fs.existsSync(rootDir)) {
    return [];
  }

  const entries = fs.readdirSync(rootDir, { withFileTypes: true });
  return entries.flatMap((entry) => {
    const fullPath = path.join(rootDir, entry.name);
    if (entry.isDirectory()) {
      return listHtmlFiles(fullPath);
    }
    return entry.isFile() && fullPath.endsWith(".html") ? [fullPath] : [];
  });
}

function toEnglishDocsPath(relativePath) {
  const normalized = relativePath.replace(/\\/g, "/");
  if (normalized.endsWith("/index.html")) {
    return `/en/${normalized.slice(0, -"/index.html".length)}/`;
  }
  return `/en/${normalized.replace(/\.html$/, "")}`;
}

function createRedirectDocument(targetUrl) {
  return [
    "<!doctype html>",
    '<html lang="en-US">',
    "  <head>",
    '    <meta charset="utf-8">',
    '    <meta name="viewport" content="width=device-width, initial-scale=1">',
    "    <title>Redirecting...</title>",
    `    <link rel="canonical" href="${targetUrl}">`,
    `    <meta http-equiv="refresh" content="0; url=${targetUrl}">`,
    `    <script>location.replace(${JSON.stringify(targetUrl)});</script>`,
    "  </head>",
    "  <body>",
    `    <p>Redirecting to <a href="${targetUrl}">${targetUrl}</a>.</p>`,
    "  </body>",
    "</html>",
    ""
  ].join("\n");
}
