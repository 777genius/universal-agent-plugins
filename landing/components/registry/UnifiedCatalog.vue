<script setup lang="ts">
import type { RegistryPlugin } from '~/types/registry';
import { catalogModeQuery, isSkillsCatalog } from '~/utils/skillsCatalog';

withDefaults(
  defineProps<{
    plugins: RegistryPlugin[];
    heading?: string;
    intro?: string;
    hideSkillsHeading?: boolean;
  }>(),
  {
    heading: undefined,
    intro: undefined,
  },
);
const { t } = useI18n();
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
    <SkillsCatalog v-if="skillsActive" :hide-heading="hideSkillsHeading" />
    <PluginCatalog v-else :plugins="plugins" :heading="heading" :intro="intro" />
  </div>
</template>
