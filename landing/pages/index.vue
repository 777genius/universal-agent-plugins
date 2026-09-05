<script setup lang="ts">
import { registryFaqItems } from '~/data/registryFaq';
import { productSoftwareSchema } from '~/utils/seo';

const registry = await useRegistryPage({ discovery: true });
const config = useRuntimeConfig();
const description =
  'Install, update, repair, and remove Agent Plugins 1.0 across supported AI agents with one CLI.';
const siteUrl = String(config.public.siteUrl).replace(/\/+$/, '');
const githubUrl = `https://github.com/${config.public.githubRepo}`;
const softwareId = `${siteUrl}/#software`;

usePageSeo('Universal Agent Plugins CLI | Install Agent Plugins 1.0', description, {
  translate: false,
  siteIdentity: true,
  pageProperties: { about: { '@id': softwareId } },
  structuredData: [
    productSoftwareSchema({
      siteUrl,
      githubUrl,
      releasesUrl: String(config.public.githubReleasesUrl),
      docsUrl: String(config.public.docsUrl),
      description,
    }),
    {
      '@type': 'FAQPage',
      '@id': `${siteUrl}/#faq`,
      mainEntity: registryFaqItems.map((item) => ({
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
    <RegistryFaq />
  </div>
</template>
