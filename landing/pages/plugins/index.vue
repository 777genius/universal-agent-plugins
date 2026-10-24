<script setup lang="ts">
import { productRootUrl } from '~/utils/seo';
import { localizedPath } from '~/utils/localizedRoutes';
import { catalogVisiblePlugins } from '~/utils/filter';
import { isSkillsCatalog } from '~/utils/skillsCatalog';
import type { KnownLocale } from '~/data/i18n';
const { t, locale } = useI18n();
const registry = await useRegistryPage({ discovery: true });
const config = useRuntimeConfig();
const route = useRoute();
const skillsActive = computed(() => isSkillsCatalog(route.query));
const description = computed(() => t('registryUi.directoryPage.description'));
const siteUrl = productRootUrl(String(config.public.siteUrl), String(config.app.baseURL)).replace(
  /\/+$/,
  '',
);
const localizedUrl = (path: string) =>
  `${siteUrl}${localizedPath(path, locale.value as KnownLocale)}`;
const listId = computed(() => `${localizedUrl('/plugins/')}#plugin-list`);
const reviewedPlugins = catalogVisiblePlugins(registry.plugins).filter(
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
        <p v-if="!skillsActive" class="eyebrow">
          {{ t('registryUi.directoryPage.pluginDirectory') }}
        </p>
        <h1>
          {{
            skillsActive
              ? t('registryUi.skills.pageHeading')
              : t('registryUi.directoryPage.findAgentPlugins10')
          }}
          <br /><em>
            {{ t('registryUi.directoryPage.useThemEverywhere') }}
          </em>
        </h1>
        <p>
          {{
            skillsActive
              ? t('registryUi.skills.intro')
              : t(
                  'registryUi.directoryPage.searchReviewedPackagesAndCommunityPluginsDiscoveredFromPublicGithubRepositories',
                )
          }}
        </p>
      </div>
      <UnifiedCatalog
        hide-skills-heading
        :plugins="registry.plugins"
        :heading="t('registryUi.directoryPage.explorePlugins')"
        :intro="
          t('registryUi.directoryPage.filterByCapabilitySourceAuthenticationOrSupportedAgent')
        "
      />
    </div>
  </div>
</template>
