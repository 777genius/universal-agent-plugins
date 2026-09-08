const escapeXml = (value: string) =>
  value
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&apos;');

export default defineEventHandler((event) => {
  const config = useRuntimeConfig();
  const siteUrl = String(
    config.public.siteUrl || 'https://777genius.github.io/universal-agent-plugins',
  ).replace(/\/+$/, '');

  setHeader(event, 'content-type', 'application/xml; charset=utf-8');

  const routes = Array.isArray(config.seo.sitemapRoutes)
    ? (config.seo.sitemapRoutes as string[])
    : ['/'];
  const body = `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
${routes
  .map((path) => `  <url>\n    <loc>${escapeXml(`${siteUrl}${path}`)}</loc>\n  </url>`)
  .join('\n')}
</urlset>
`;

  return body;
});
