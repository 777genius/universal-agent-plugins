<script setup lang="ts">
import { productRootUrl, productSoftwareSchema } from '~/utils/seo';

const { t } = useI18n();
const config = useRuntimeConfig();
const { docsUrl } = useDocsLinks();
const description = computed(() => t('shell.seo.downloadDescription'));
const siteUrl = productRootUrl(
  String(config.public.siteUrl),
  String(config.app.baseURL),
).replace(/\/+$/, '');
const githubUrl = `https://github.com/${config.public.githubRepo}`;

usePageSeo(() => t('shell.seo.downloadTitle'), description, {
  translate: false,
  pageProperties: { mainEntity: { '@id': `${siteUrl}/#software` } },
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
  ],
});
</script>

<template>
  <div class="download-page">
    <DownloadSection heading-tag="h1" />
  </div>
</template>
