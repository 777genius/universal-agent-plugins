<script setup lang="ts">
import type { RegistryPlugin } from '~/types/registry';
import { catalogModeQuery, isSkillsCatalog } from '~/utils/skillsCatalog';

const props = withDefaults(
  defineProps<{
    plugins: RegistryPlugin[];
    intro?: string;
    directoryPage?: boolean;
  }>(),
  {
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
const catalogIntro = computed(() =>
  skillsActive.value
    ? t('registryUi.skills.intro')
    : props.directoryPage
      ? t(
          'registryUi.directoryPage.searchReviewedPackagesAndCommunityPluginsDiscoveredFromPublicGithubRepositories',
        )
      : (props.intro ?? t('registryUi.catalog.searchByCapabilityComponentOrSource')),
);
const { repositoryUrl } = useSite();
const route = useRoute();
const skillsActive = computed(() => isSkillsCatalog(route.query));
const headingLabel = computed(
  () =>
    `${t('registryUi.skills.headingPrefix')} ${t(skillsActive.value ? 'registryUi.skills.skills' : 'registryUi.skills.plugins')}`,
);
const modeLink = (skills: boolean) => ({
  path: route.path,
  query: catalogModeQuery(route.query, skills),
  hash: route.hash,
});
</script>

<template>
  <div class="unified-catalog">
    <div class="unified-catalog__heading" :class="directoryPage ? 'page-intro' : 'section-heading'">
      <div class="catalog-heading-row">
        <component
          :is="directoryPage ? 'h1' : 'h2'"
          id="unified-catalog-title"
          class="unified-catalog__title"
          :aria-label="headingLabel"
        >
          <span>{{ t('registryUi.skills.headingPrefix') }}</span>
          <span class="catalog-modes" role="group" :aria-label="t('registryUi.skills.catalogType')">
            <NuxtLink
              :to="modeLink(false)"
              :aria-current="!skillsActive ? 'page' : undefined"
              class="catalog-mode"
              :class="{ 'catalog-mode--active': !skillsActive }"
            >
              {{ t('registryUi.skills.plugins') }}
            </NuxtLink>
            <NuxtLink
              :to="modeLink(true)"
              :aria-current="skillsActive ? 'page' : undefined"
              class="catalog-mode"
              :class="{ 'catalog-mode--active': skillsActive }"
            >
              {{ t('registryUi.skills.skills') }}
            </NuxtLink>
          </span>
        </component>
        <a
          v-if="!skillsActive"
          class="catalog-add-button"
          :href="`${repositoryUrl}/blob/main/registry/README.md#submit-an-external-package`"
          target="_blank"
          rel="noreferrer"
          :aria-label="t('registryUi.catalog.addAPlugin')"
        >
          <span aria-hidden="true">＋</span><span>{{ t('registryUi.catalog.addPlugin') }}</span>
        </a>
      </div>
      <p>{{ catalogIntro }}</p>
    </div>
    <SkillsCatalog v-if="skillsActive" hide-heading />
    <PluginCatalog v-else :plugins="plugins" hide-heading />
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
