<script setup lang="ts">
import { isKnownLocale } from '~/data/i18n';
import { localizedPath } from '~/utils/localizedRoutes';
import { productRootUrl, productSoftwareSchema } from '~/utils/seo';
const { t, locale } = useI18n();
const registryFaqItems = computed(() =>
  ['0', '1', '2', '3', '4', '5'].map((key) => ({
    question: t(`shell.faq.items.${key}.question`),
    answer: t(`shell.faq.items.${key}.answer`),
  })),
);

const registry = await useRegistryPage({ discovery: true });
const config = useRuntimeConfig();
const { docsUrl } = useDocsLinks();
const homePath = computed(() =>
  localizedPath('/', isKnownLocale(locale.value) ? locale.value : 'en'),
);
const description = computed(() => t('shell.seo.indexDescription'));
const siteUrl = productRootUrl(
  String(config.public.siteUrl),
  String(config.app.baseURL),
).replace(/\/+$/, '');
const githubUrl = `https://github.com/${config.public.githubRepo}`;
const softwareId = `${siteUrl}/#software`;

usePageSeo(() => t('shell.seo.indexTitle'), description, {
  translate: false,
  siteIdentity: true,
  pageProperties: { about: { '@id': softwareId } },
  structuredData: () => [
    {
      ...productSoftwareSchema({
        siteUrl,
        githubUrl,
        releasesUrl: String(config.public.githubReleasesUrl),
        docsUrl: docsUrl.value,
        description: description.value,
        applicationSubCategory: t('shell.seo.applicationSubCategory'),
      softwareRequirements: t('shell.seo.softwareRequirements'),
        featureList: t('shell.seo.featureList'),
      }),
    },
    {
      '@type': 'FAQPage',
      '@id': `${siteUrl}${homePath.value}#faq`,
      mainEntity: registryFaqItems.value.map((item) => ({
        '@type': 'Question',
        name: item.question,
        acceptedAnswer: {
          '@type': 'Answer',
          text: item.answer,
        },
      })),
    },
  ],
});
</script>

<template>
  <div class="registry-surface unified-home">
    <PageBackground />
    <RegistryHero :registry="registry" />
    <RegistryDirectory :registry="registry" />
    <RegistryWhy />
    <RegistryFaq :items="registryFaqItems" />
  </div>
</template>
