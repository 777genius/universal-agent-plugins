<script setup lang="ts">
import type { RegistryPlugin } from '~/types/registry';
import { catalogModeQuery, isSkillsCatalog } from '~/utils/skillsCatalog';

const props = withDefaults(
  defineProps<{
    plugins: RegistryPlugin[];
    heading?: string;
    intro?: string;
    directoryPage?: boolean;
  }>(),
  {
    heading: undefined,
    intro: undefined,
  },
);
// External SFC resources are compiled by the existing Vue I18n plugin.
// One local composer serves this entry and its Skill children, outside global SSR payloads.
const catalogI18n = useI18n({ useScope: 'local' });
// Generic control labels intentionally fall back to the global dictionaries.
catalogI18n.missingWarn = false;
catalogI18n.fallbackWarn = false;
const { t } = catalogI18n;
const pluginHeading = computed(() =>
  props.directoryPage ? t('registryUi.directoryPage.explorePlugins') : props.heading,
);
const pluginIntro = computed(() =>
  props.directoryPage
    ? t('registryUi.directoryPage.filterByCapabilitySourceAuthenticationOrSupportedAgent')
    : props.intro,
);
const route = useRoute();
const skillsActive = computed(() => isSkillsCatalog(route.query));
const modeLink = (skills: boolean) => ({
  path: route.path,
  query: catalogModeQuery(route.query, skills),
  hash: route.hash,
});
</script>

<template>
  <div class="unified-catalog">
    <div v-if="directoryPage" class="page-intro">
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
    <nav class="catalog-modes" :aria-label="t('registryUi.skills.catalogType')">
      <NuxtLink
        :to="modeLink(false)"
        :aria-current="!skillsActive ? 'page' : undefined"
        class="catalog-mode"
        :class="{ 'catalog-mode--active': !skillsActive }"
      >
        <FilterIcon name="component" />
        <span>{{ t('registryUi.skills.plugins') }}</span>
      </NuxtLink>
      <NuxtLink
        :to="modeLink(true)"
        :aria-current="skillsActive ? 'page' : undefined"
        class="catalog-mode"
        :class="{ 'catalog-mode--active': skillsActive }"
      >
        <FilterIcon name="category" />
        <span>{{ t('registryUi.skills.skills') }}</span>
      </NuxtLink>
    </nav>
    <SkillsCatalog v-if="skillsActive" :hide-heading="directoryPage" />
    <PluginCatalog v-else :plugins="plugins" :heading="pluginHeading" :intro="pluginIntro" />
  </div>
</template>

<i18n locale="en" lang="json" src="../../features/skills-catalog/locales/en.json" />
<i18n locale="ru" lang="json" src="../../features/skills-catalog/locales/ru.json" />
<i18n locale="uk" lang="json" src="../../features/skills-catalog/locales/uk.json" />
<i18n locale="zh" lang="json" src="../../features/skills-catalog/locales/zh.json" />
<i18n locale="es" lang="json" src="../../features/skills-catalog/locales/es.json" />
<i18n locale="hi" lang="json" src="../../features/skills-catalog/locales/hi.json" />
<i18n locale="ar" lang="json" src="../../features/skills-catalog/locales/ar.json" />
<i18n locale="pt" lang="json" src="../../features/skills-catalog/locales/pt.json" />
<i18n locale="fr" lang="json" src="../../features/skills-catalog/locales/fr.json" />

<style lang="scss" src="../../features/skills-catalog/catalog.scss"></style>
