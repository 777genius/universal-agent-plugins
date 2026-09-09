import { productRootUrl } from '../../utils/seo.ts';

export default defineEventHandler((event) => {
  const config = useRuntimeConfig();
  const siteUrl = productRootUrl(
    String(config.public.siteUrl || 'https://777genius.github.io/universal-agent-plugins'),
    String(config.app.baseURL),
  );
  const docsSitemapUrl = (config.public.docsSitemapUrl as string) || `${siteUrl}docs/sitemap.xml`;

  setHeader(event, 'content-type', 'text/plain; charset=utf-8');

  return `User-agent: *
Allow: /
Sitemap: ${siteUrl}sitemap.xml
Sitemap: ${docsSitemapUrl}
`;
});
