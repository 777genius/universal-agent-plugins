import { pageRouteSeo } from '../../utils/seo';
import type { ProductRoute } from '../../data/routes';

const escapeXml = (value: string) =>
  value.replace(
    /[&<>"']/g,
    (character) =>
      ({
        '&': '&amp;',
        '<': '&lt;',
        '>': '&gt;',
        '"': '&quot;',
        "'": '&apos;',
      })[character]!,
  );

export default defineEventHandler((event) => {
  const config = useRuntimeConfig();
  const routes = config.public.seoRoutes as ProductRoute[];
  const localized = config.seo.sitemapRoutes as Array<{ path: string }>;
  const entries = localized.map(({ path }) => {
    const seo = pageRouteSeo(
      path,
      routes,
      String(config.public.siteUrl),
      String(config.app.baseURL),
    );
    if (!seo.canonical || !seo.current?.indexable)
      throw new Error(`Invalid sitemap route: ${path}`);
    return `  <url><loc>${escapeXml(seo.canonical)}</loc>${seo.alternates
      .map(
        (link) =>
          `<xhtml:link rel="alternate" hreflang="${escapeXml(link.hreflang)}" href="${escapeXml(link.href)}"/>`,
      )
      .join('')}</url>`;
  });
  setHeader(event, 'content-type', 'application/xml; charset=utf-8');
  return `<?xml version="1.0" encoding="UTF-8"?>\n<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9" xmlns:xhtml="http://www.w3.org/1999/xhtml">\n${entries.join('\n')}\n</urlset>\n`;
});
