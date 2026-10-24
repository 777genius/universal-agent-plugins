<script setup lang="ts">
import type { RegistryIndex } from '~/types/registry';
import { catalogTrustCounts } from '~/utils/filter';
const { t, n } = useI18n();

const props = defineProps<{ registry: RegistryIndex }>();
const counts = computed(() => catalogTrustCounts(props.registry.plugins));
const reviewedCount = computed(() => counts.value.reviewed);
const discoveryCount = computed(() => counts.value.community);
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
    <UnifiedCatalog
      :plugins="registry.plugins"
      :heading="t('registryUi.directory.heading')"
      :intro="intro"
    />
  </div>
</template>
