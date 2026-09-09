<script setup lang="ts">
import { productRootUrl } from '~/utils/seo';
import { localizedPath } from '~/utils/localizedRoutes';
import type { KnownLocale } from '~/data/i18n';
const { t, locale } = useI18n();
const registry = await useRegistryPage({ discovery: true });
const config = useRuntimeConfig();
const description = computed(() => t('registryUi.directoryPage.description'));
const siteUrl = productRootUrl(
  String(config.public.siteUrl),
  String(config.app.baseURL),
).replace(/\/+$/, '');
const localizedUrl = (path: string) => `${siteUrl}${localizedPath(path, locale.value as KnownLocale)}`;
const listId = computed(() => `${localizedUrl('/plugins/')}#plugin-list`);
const reviewedPlugins = registry.plugins.filter(
  (plugin) => plugin.trust_state !== 'conformant_unreviewed',
);

usePageSeo(() => t('registryUi.directoryPage.title'), description, {
  translate: false,
  pageType: 'CollectionPage',
  pageProperties: () => ({ mainEntity: { '@id': listId.value } }),
  structuredData: () => [
    {
      '@type': 'ItemList',
      '@id': listId.value,
      name: t('registryUi.directoryPage.listName'),
      numberOfItems: reviewedPlugins.length,
      itemListElement: reviewedPlugins.map((plugin, index) => ({
        '@type': 'ListItem',
        position: index + 1,
        name: plugin.display_name,
        url: localizedUrl(`/plugins/${plugin.name}/`),
      })),
    },
  ],
});
</script>

<template>
  <div class="registry-surface directory-page">
    <PageBackground />
    <div class="container">
      <div class="page-intro">
        <p class="eyebrow">{{ t('registryUi.directoryPage.pluginDirectory') }}</p>
        <h1>
          {{ t('registryUi.directoryPage.findAgentPlugins10') }} <br /><em>
            {{ t('registryUi.directoryPage.useThemEverywhere') }}
          </em>
        </h1>
        <p>
          {{
            t(
              'registryUi.directoryPage.searchReviewedPackagesAndCommunityPluginsDiscoveredFromPublicGithubRepositories',
            )
          }}
        </p>
      </div>
      <PluginCatalog
        :plugins="registry.plugins"
        :heading="t('registryUi.directoryPage.explorePlugins')"
        :intro="
          t('registryUi.directoryPage.filterByCapabilitySourceAuthenticationOrSupportedAgent')
        "
      />
    </div>
  </div>
</template>
