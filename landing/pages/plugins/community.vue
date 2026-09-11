<script setup lang="ts">
import type { ClientID } from '~/types/registry';
const { t } = useI18n();
const localePath = useLocalePath();

const route = useRoute();
const registry = await useRegistryPage({
  projection: { kind: 'empty' },
  discovery: true,
});
const discovery = useDiscoveryStatus();
const { pluginIcon, sourceUrl } = useSite();
const requestedSource = computed(() => String(route.query.source ?? ''));
const plugin = computed(() =>
  registry.plugins.find(
    (item) =>
      item.trust_state === 'conformant_unreviewed' && item.install_source === requestedSource.value,
  ),
);
const discoverySettled = computed(() =>
  ['current', 'cached', 'stale', 'unavailable'].includes(discovery.value.state),
);
const availableClients = computed(() =>
  plugin.value
    ? clients.filter((client) => plugin.value!.client_support.clients.includes(client.id))
    : [],
);
const sourceUnavailable = computed(() => plugin.value?.discovery?.availability === 'unavailable');
const targets = ref<ClientID[]>([]);
const autoDetect = ref(true);

watch(
  availableClients,
  (next) => {
    if (!targets.value.length && next[0]) targets.value = [next[0].id];
  },
  { immediate: true },
);

usePageSeo(
  () =>
    plugin.value
      ? t('registryUi.community.title', { name: plugin.value.display_name })
      : t('registryUi.community.defaultTitle'),
  () => plugin.value?.description ?? t('registryUi.community.description'),
  {
    translate: false,
    robots: 'noindex, follow',
    canonical: false,
    includeWebPage: false,
  },
);
</script>

<template>
  <div class="registry-surface plugin-page community-plugin-page">
    <PageBackground />
    <div class="container">
      <nav class="breadcrumbs" :aria-label="t('registryUi.community.breadcrumb')">
        <NuxtLink :to="localePath('/plugins/')"> {{ t('registryUi.community.plugins') }} </NuxtLink
        ><span aria-hidden="true">/</span
        ><span>{{ plugin?.display_name ?? t('registryUi.community.communityPlugin') }}</span>
      </nav>

      <div v-if="plugin" class="plugin-page__grid">
        <article class="plugin-profile">
          <div class="plugin-profile__heading">
            <span v-if="pluginIcon(plugin)" class="plugin-profile__icon">
              <img :src="pluginIcon(plugin)" alt="" width="54" height="54" loading="eager" />
            </span>
            <div>
              <div class="plugin-profile__meta">
                {{
                  plugin.installable
                    ? t('registryUi.community.communityPlugin')
                    : t('registryUi.community.communityListing')
                }}
              </div>
              <h1>{{ plugin.display_name }}</h1>
            </div>
          </div>
          <p class="plugin-profile__description">{{ plugin.description }}</p>

          <dl class="plugin-facts">
            <div>
              <dt>{{ t('registryUi.community.author') }}</dt>
              <dd>{{ plugin.author.name || t('registryUi.community.notDeclared') }}</dd>
            </div>
            <div>
              <dt>
                {{
                  sourceUnavailable
                    ? t('registryUi.community.lastKnownAgents')
                    : t('registryUi.community.worksWith')
                }}
              </dt>
              <dd>
                {{
                  availableClients.length
                    ? availableClients.map((client) => client.name).join(', ')
                    : plugin.client_support.resolution === 'install_time' && plugin.installable
                      ? t('registryUi.community.detectedAtInstallTime')
                      : t('registryUi.community.notDeclared')
                }}
              </dd>
            </div>
            <div>
              <dt>
                {{
                  sourceUnavailable
                    ? t('registryUi.community.lastKnownComponents')
                    : t('registryUi.community.components')
                }}
              </dt>
              <dd>
                {{
                  plugin.components.length
                    ? plugin.components
                        .map((component) => t(`registryUi.components.${component}`))
                        .join(', ')
                    : t('registryUi.community.notDeclared')
                }}
              </dd>
            </div>
            <div>
              <dt>{{ t('registryUi.community.source') }}</dt>
              <dd>
                <a :href="sourceUrl(plugin)" target="_blank" rel="noreferrer">
                  {{ t('registryUi.community.viewOnGithub') }}
                </a>
              </dd>
            </div>
          </dl>

          <SecurityAssessmentPanel v-if="plugin.security" :plugin="plugin" />
        </article>

        <InstallPanel v-model:targets="targets" v-model:auto-detect="autoDetect" :plugin="plugin" />
      </div>

      <div v-else-if="!discoverySettled" class="community-plugin-state" role="status">
        <h1>{{ t('registryUi.community.loadingPlugin') }}</h1>
        <p>{{ t('registryUi.community.checkingTheCurrentCommunityDirectory') }}</p>
      </div>
      <div v-else class="community-plugin-state">
        <h1>{{ t('registryUi.community.pluginNotFound') }}</h1>
        <p>{{ t('registryUi.community.thisPackageIsNoLongerAvailableInTheCurrentDirectory') }}</p>
        <NuxtLink class="button button--primary" :to="localePath('/plugins/')">
          {{ t('registryUi.community.explorePlugins') }}
        </NuxtLink>
      </div>
    </div>
  </div>
</template>

<style scoped>
.community-plugin-page .plugin-page__grid {
  grid-template-columns: minmax(0, 1fr);
  max-width: 960px;
  margin-inline: auto;
  gap: 32px;
}

.community-plugin-page :deep(.install-panel) {
  position: static;
}

.plugin-profile,
.plugin-facts dd {
  min-width: 0;
  overflow-wrap: anywhere;
}
</style>
