export default defineEventHandler((event) => {
  const config = useRuntimeConfig();
  const siteUrl = String(
    config.public.siteUrl || 'https://777genius.github.io/universal-agent-plugins',
  ).replace(/\/+$/, '');
  const docsSitemapUrl =
    (config.public.docsSitemapUrl as string) ||
    'https://777genius.github.io/universal-agent-plugins/docs/sitemap.xml';

  setHeader(event, 'content-type', 'text/plain; charset=utf-8');

  return `User-agent: *
Allow: /
Sitemap: ${siteUrl}/sitemap.xml
Sitemap: ${docsSitemapUrl}
`;
});
