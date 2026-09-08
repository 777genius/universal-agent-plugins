<script setup lang="ts">
import type { RegistryIndex } from '~/types/registry';
import { catalogVisiblePlugins } from '~/utils/filter';
const { t, n } = useI18n();

const props = defineProps<{ registry: RegistryIndex }>();
const catalogPlugins = computed(() => catalogVisiblePlugins(props.registry.plugins));
const reviewedCount = computed(
  () =>
    catalogPlugins.value.filter((plugin) => plugin.trust_state !== 'conformant_unreviewed').length,
);
const discoveryCount = computed(() => catalogPlugins.value.length - reviewedCount.value);
const intro = computed(() =>
  discoveryCount.value
    ? t('registryUi.directory.combined', {
        reviewed: t(
          'registryUi.directory.reviewedCount',
          { count: n(reviewedCount.value) },
          reviewedCount.value,
        ),
        community: t(
          'registryUi.directory.communityCount',
          { count: n(discoveryCount.value) },
          discoveryCount.value,
        ),
      })
    : t('registryUi.directory.reviewed', { count: n(reviewedCount.value) }, reviewedCount.value),
);
</script>

<template>
  <div id="plugins" class="container catalog-wrap" data-scroll-reveal>
    <PluginCatalog
      :plugins="registry.plugins"
      :heading="t('registryUi.directory.heading')"
      :intro="intro"
    />
  </div>
</template>
