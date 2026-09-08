import fs from "node:fs";
import path from "node:path";
import { defineConfig } from "vitepress";

const websiteRoot = path.resolve(__dirname, "..", "..");
const generatedRoot = path.join(websiteRoot, "generated", "registries");
type RegistryEntity = {
  canonicalId?: string;
  pathEn?: string;
  pathRu?: string;
  pathEs?: string;
  pathFr?: string;
  pathZh?: string;
};

function readJson<T>(fileName: string, fallback: T): T {
  const full = path.join(generatedRoot, fileName);
  if (!fs.existsSync(full)) {
    return fallback;
  }
  return JSON.parse(fs.readFileSync(full, "utf8")) as T;
}

export const docsBasePath = process.env.DOCS_BASE_PATH || "/plugin-kit-ai/docs/";
export const docsHostname = process.env.DOCS_HOSTNAME || "https://777genius.github.io";
const docsBaseUrl = new URL(docsBasePath, docsHostname).toString();
const socialImageUrl = new URL("og-docs.svg", docsBaseUrl).toString();
const entities = readJson<RegistryEntity[]>("entities.json", []);
const entityByCanonicalId = new Map(
  entities
    .filter((entity) => typeof entity.canonicalId === "string" && entity.canonicalId.length > 0)
    .map((entity) => [entity.canonicalId as string, entity])
);
const localeHrefLang = {
  en: "en-US",
  ru: "ru-RU",
  es: "es-ES",
  fr: "fr-FR",
  zh: "zh-CN"
} as const;

function escapeForRegExp(value: string): string {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

function toDocsAbsoluteUrl(value: string): string {
  return new URL(value.replace(/^\/+/, ""), docsBaseUrl).toString();
}

function toDocsPageUrl(relativePath: string): string {
  const normalized = relativePath.replace(/\\/g, "/");
  if (normalized === "gateway/index.md") {
    return docsBaseUrl;
  }

  const withoutExtension = normalized.replace(/\.md$/, "");
  const publicPath = withoutExtension.endsWith("/index")
    ? `${withoutExtension.slice(0, -"/index".length)}/`
    : withoutExtension;

  return toDocsAbsoluteUrl(publicPath);
}

export const sharedConfig = defineConfig({
  srcDir: ".site",
  publicDir: path.join(websiteRoot, "public"),
  outDir: "dist",
  cleanUrls: true,
  lastUpdated: true,
  base: docsBasePath,
  title: "plugin-kit-ai Docs",
  description: "Public documentation for plugin-kit-ai",
  head: [
    ["link", { rel: "icon", href: `${docsBasePath}icon.svg`, type: "image/svg+xml" }],
    ["link", { rel: "manifest", href: `${docsBasePath}site.webmanifest` }],
    ["meta", { name: "theme-color", content: "#f7f7f8" }],
    ["meta", { name: "color-scheme", content: "light dark" }]
  ],
  transformHead({ pageData, title, description }) {
    const relativePath = pageData.relativePath.replace(/\\/g, "/");
    const isGateway = relativePath.startsWith("gateway/");
    const isNotFound = pageData.isNotFound || relativePath === "gateway/404.md";
    const pageTitle = title || pageData.title || "plugin-kit-ai Docs";
    const pageDescription =
      description || pageData.description || "Public documentation for plugin-kit-ai.";
    const pageUrl = isNotFound ? null : toDocsPageUrl(relativePath);
    const head = [
      ["meta", { property: "og:type", content: "website" }],
      ["meta", { property: "og:site_name", content: "plugin-kit-ai Docs" }],
      ["meta", { property: "og:title", content: pageTitle }],
      ["meta", { property: "og:description", content: pageDescription }],
      ["meta", { property: "og:image", content: socialImageUrl }],
      ["meta", { name: "twitter:card", content: "summary" }],
      ["meta", { name: "twitter:title", content: pageTitle }],
      ["meta", { name: "twitter:description", content: pageDescription }],
      ["meta", { name: "twitter:image", content: socialImageUrl }]
    ] as [string, Record<string, string>][];

    if (pageUrl) {
      head.push(["meta", { property: "og:url", content: pageUrl }]);
    }

    if (!isGateway && !isNotFound && pageUrl) {
      head.push(["link", { rel: "canonical", href: pageUrl }]);
    }

    const canonicalId =
      typeof pageData.frontmatter?.canonicalId === "string"
        ? pageData.frontmatter.canonicalId
        : null;
    const entity = canonicalId ? entityByCanonicalId.get(canonicalId) : null;

    if (!isGateway && !isNotFound && entity) {
      const localePaths = [
        ["en", entity.pathEn],
        ["ru", entity.pathRu],
        ["es", entity.pathEs],
        ["fr", entity.pathFr],
        ["zh", entity.pathZh]
      ] as const;

      for (const [code, localePath] of localePaths) {
        if (typeof localePath !== "string" || !localePath) {
          continue;
        }

        head.push([
          "link",
          {
            rel: "alternate",
            hreflang: localeHrefLang[code],
            href: toDocsAbsoluteUrl(localePath)
          }
        ]);
      }

      const defaultPath =
        entity.pathEn || entity.pathRu || entity.pathEs || entity.pathFr || entity.pathZh;
      if (typeof defaultPath === "string" && defaultPath) {
        head.push([
          "link",
          {
            rel: "alternate",
            hreflang: "x-default",
            href: toDocsAbsoluteUrl(defaultPath)
          }
        ]);
      }
    }

    return head;
  },
  sitemap: {
    hostname: docsBaseUrl,
    transformItems(items) {
      const gatewayUrl = docsBaseUrl;
      const normalize = (value: string) => (value.endsWith("/") ? value : `${value}/`);
      return items.filter((item) => {
        const url = normalize(item.url);
        const pathname = new URL(item.url, docsBaseUrl).pathname.replace(/\/+$/, "");
        return (
          url !== "/" &&
          !pathname.endsWith("/404") &&
          url !== normalize(docsBasePath) &&
          url !== normalize(gatewayUrl)
        );
      });
    }
  },
  async buildEnd() {
    const distRoot = path.join(websiteRoot, "dist");
    const sitemapPath = path.join(distRoot, "sitemap.xml");
    if (!fs.existsSync(sitemapPath)) {
      return;
    }
    const excludedUrls = [docsBaseUrl, new URL("404", docsBaseUrl).toString()];
    const current = fs.readFileSync(sitemapPath, "utf8");
    const next = excludedUrls.reduce((body, url) => {
      const entryPattern = new RegExp(
        `<url><loc>${escapeForRegExp(url)}\\/?<\\/loc>[\\s\\S]*?<\\/url>`,
        "g"
      );
      return body.replace(entryPattern, "");
    }, current);
    if (next !== current) {
      fs.writeFileSync(sitemapPath, next);
    }

    const robotsPath = path.join(distRoot, "robots.txt");
    const robotsBody = [`User-agent: *`, `Allow: /`, `Sitemap: ${new URL("sitemap.xml", docsBaseUrl).toString()}`, ``].join(
      "\n"
    );
    fs.writeFileSync(robotsPath, robotsBody);
  },
  locales: {
    root: { label: "Language Gateway", lang: "en-US", link: "/" },
    en: { label: "English", lang: "en-US" },
    ru: { label: "Русский", lang: "ru-RU" },
    es: { label: "Español", lang: "es-ES" },
    fr: { label: "Français", lang: "fr-FR" },
    zh: { label: "简体中文", lang: "zh-CN" }
  },
  rewrites: readJson<Record<string, string>>("redirects.json", {}),
  themeConfig: {
    logo: "/icon.svg",
    siteTitle: "plugin-kit-ai Docs",
    search: {
      provider: "local"
    },
    socialLinks: [{ icon: "github", link: "https://github.com/777genius/plugin-kit-ai" }]
  }
});
