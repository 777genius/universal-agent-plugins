<script setup lang="ts">
import { productSoftwareSchema } from '~/utils/seo';

const config = useRuntimeConfig();
const description =
  'Install the Universal Agent Plugins CLI on macOS, Linux, or Windows and manage Agent Plugins 1.0 from one command.';
const siteUrl = String(config.public.siteUrl).replace(/\/+$/, '');
const githubUrl = `https://github.com/${config.public.githubRepo}`;

usePageSeo('Download Universal Agent Plugins CLI', description, {
  translate: false,
  pageProperties: { mainEntity: { '@id': `${siteUrl}/#software` } },
  structuredData: [
    productSoftwareSchema({
      siteUrl,
      githubUrl,
      releasesUrl: String(config.public.githubReleasesUrl),
      docsUrl: String(config.public.docsUrl),
      description,
    }),
  ],
});
</script>

<template>
  <div class="download-page">
    <DownloadSection heading-tag="h1" />
  </div>
</template>
