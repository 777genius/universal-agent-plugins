import { computed, toValue } from 'vue';
import { pageRouteSeo, productRootUrl, productImageUrl } from '~/utils/seo';
import type { ProductRoute } from '~/data/routes';
import type { MaybeRefOrGetter } from 'vue';

type PageSeoImage = {
  url: string;
  width?: number;
  height?: number;
  type?: string;
  alt?: string;
};

type PageSeoOptions = {
  type?: 'website' | 'article';
  robots?: string;
  image?: PageSeoImage;
  translate?: boolean;
  canonical?: boolean;
  /** @deprecated Canonical identity comes from the actual published route. */
  canonicalPath?: MaybeRefOrGetter<string>;
  includeWebPage?: boolean;
  pageType?: 'WebPage' | 'CollectionPage';
  pageProperties?: MaybeRefOrGetter<Record<string, unknown>>;
  siteIdentity?: boolean;
  structuredData?: MaybeRefOrGetter<readonly Record<string, unknown>[]>;
};

export const usePageSeo = (
  titleSource: MaybeRefOrGetter<string>,
  descriptionSource: MaybeRefOrGetter<string>,
  options: PageSeoOptions = {},
) => {
  const { t, locale } = useI18n();
  const route = useRoute();
  const config = useRuntimeConfig();
  const siteUrl = productRootUrl(
    String(config.public.siteUrl || 'https://777genius.github.io/universal-agent-plugins'),
    String(config.app.baseURL),
  ).replace(/\/+$/, '');
  const siteName = 'Universal Agent Plugins';
  const githubUrl = `https://github.com/${config.public.githubRepo}`;

  const shouldTranslate = options.translate !== false;
  const title = computed(() => (shouldTranslate ? t(toValue(titleSource)) : toValue(titleSource)));
  const description = computed(() =>
    shouldTranslate ? t(toValue(descriptionSource)) : toValue(descriptionSource),
  );
  const routeSeo = computed(() =>
    pageRouteSeo(
      route.path,
      config.public.seoRoutes as ProductRoute[],
      siteUrl,
      String(config.app.baseURL),
    ),
  );
  const canonicalUrl = computed(() => routeSeo.value.canonical);
  const includeAlternates = computed(
    () =>
      routeSeo.value.current?.indexable &&
      !/\bnoindex\b/.test(options.robots || '') &&
      options.canonical !== false,
  );
  const includeCanonical = computed(
    () => options.canonical !== false && Boolean(canonicalUrl.value),
  );

  const resolvedImage = computed<PageSeoImage>(() => {
    if (options.image) {
      return options.image;
    }

    return {
      url: 'og-image.png',
      width: 1200,
      height: 630,
      type: 'image/png',
      alt: t('shell.seo.imageAlt'),
    };
  });

  const resolvedImageUrl = computed(() =>
    productImageUrl(resolvedImage.value.url, siteUrl, String(config.app.baseURL)),
  );

  useSeoMeta({
    title,
    description,
    ogTitle: title,
    ogDescription: description,
    ogType: options.type || 'website',
    ogSiteName: siteName,
    ogLocale: computed(() => routeSeo.value.ogLocale),
    ogUrl: canonicalUrl,
    ogImage: resolvedImageUrl,
    ogImageType: computed(() => resolvedImage.value.type) as never,
    ogImageWidth: computed(() =>
      resolvedImage.value.width ? String(resolvedImage.value.width) : undefined,
    ),
    ogImageHeight: computed(() =>
      resolvedImage.value.height ? String(resolvedImage.value.height) : undefined,
    ),
    ogImageAlt: computed(() => resolvedImage.value.alt),
    twitterCard: 'summary_large_image',
    twitterTitle: title,
    twitterDescription: description,
    twitterImage: resolvedImageUrl,
    twitterImageAlt: computed(() => resolvedImage.value.alt),
    robots: computed(
      () =>
        options.robots ||
        (!routeSeo.value.current?.indexable
          ? 'noindex, follow'
          : 'index, follow, max-snippet:-1, max-image-preview:large, max-video-preview:-1'),
    ),
  });

  useHead(() => {
    const webSiteId = `${siteUrl}/#website`;
    const organizationId = `${siteUrl}/#organization`;
    const pageId = `${canonicalUrl.value}#webpage`;
    const jsonLd: Record<string, unknown>[] = [];

    if (options.includeWebPage !== false && canonicalUrl.value) {
      jsonLd.push({
        '@type': options.pageType || 'WebPage',
        '@id': pageId,
        url: canonicalUrl.value,
        name: title.value,
        description: description.value,
        inLanguage: locale.value || 'en',
        isPartOf: { '@id': webSiteId },
        ...(options.pageProperties ? toValue(options.pageProperties) : {}),
      });
    }

    if (options.siteIdentity) {
      jsonLd.push({
        '@type': 'WebSite',
        '@id': webSiteId,
        name: siteName,
        alternateName: 'UAP',
        url: `${siteUrl}/`,
        inLanguage: 'en',
        description: description.value,
        publisher: { '@id': organizationId },
      });
      jsonLd.push({
        '@type': 'Organization',
        '@id': organizationId,
        name: siteName,
        alternateName: 'UAP',
        description: t('shell.seo.organizationDescription'),
        url: `${siteUrl}/`,
        logo: {
          '@type': 'ImageObject',
          url: `${siteUrl}/logo-192.png`,
          contentUrl: `${siteUrl}/logo-192.png`,
          width: 192,
          height: 192,
        },
        sameAs: [githubUrl, 'https://www.npmjs.com/package/universal-agent-plugins'],
      });
    }

    if (options.structuredData) jsonLd.push(...toValue(options.structuredData));

    return {
      htmlAttrs: {
        lang: locale.value || 'en',
      },
      link: includeCanonical.value
        ? [
            { key: 'canonical', rel: 'canonical', href: canonicalUrl.value },
            ...(includeAlternates.value ? routeSeo.value.alternates : []).map((link) => ({
              key: `alternate-${link.hreflang}`,
              rel: 'alternate',
              ...link,
            })),
          ]
        : [],
      meta: [
        ...(includeAlternates.value ? routeSeo.value.ogAlternates : []).map((content) => ({
          key: `og-locale-${content}`,
          property: 'og:locale:alternate',
          content,
        })),
        { name: 'author', content: 'Universal Agent Plugins' },
        { name: 'application-name', content: siteName },
        { name: 'apple-mobile-web-app-title', content: siteName },
        { name: 'format-detection', content: 'telephone=no' },
        { name: 'theme-color', content: '#00f0ff' },
      ],
      script: jsonLd.length
        ? [
            {
              key: 'seo-jsonld',
              type: 'application/ld+json',
              children: JSON.stringify({ '@context': 'https://schema.org', '@graph': jsonLd }),
            },
          ]
        : [],
    };
  });
};
