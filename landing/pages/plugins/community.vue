<script setup lang="ts">
import type { ClientID } from '~/types/registry';

const route = useRoute();
const registry = useRegistry();
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
      ? `${plugin.value.display_name} Agent Plugin | Universal Agent Plugins`
      : 'Community Agent Plugin | Universal Agent Plugins',
  () => plugin.value?.description ?? 'Install a community Agent Plugin across supported AI agents.',
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
      <nav class="breadcrumbs" aria-label="Breadcrumb">
        <NuxtLink to="/plugins/">Plugins</NuxtLink><span aria-hidden="true">/</span
        ><span>{{ plugin?.display_name ?? 'Community plugin' }}</span>
      </nav>

      <div v-if="plugin" class="plugin-page__grid">
        <article class="plugin-profile">
          <div class="plugin-profile__heading">
            <span v-if="pluginIcon(plugin)" class="plugin-profile__icon">
              <img :src="pluginIcon(plugin)" alt="" width="54" height="54" loading="eager" >
            </span>
            <div>
              <div class="plugin-profile__meta">
                {{ plugin.installable ? 'Community plugin' : 'Community listing' }}
              </div>
              <h1>{{ plugin.display_name }}</h1>
            </div>
          </div>
          <p class="plugin-profile__description">{{ plugin.description }}</p>

          <dl class="plugin-facts">
            <div>
              <dt>Author</dt>
              <dd>{{ plugin.author.name || 'Not declared' }}</dd>
            </div>
            <div>
              <dt>{{ sourceUnavailable ? 'Last known agents' : 'Works with' }}</dt>
              <dd>
                {{
                  availableClients.length
                    ? availableClients.map((client) => client.name).join(', ')
                    : plugin.client_support.resolution === 'install_time' && plugin.installable
                      ? 'Detected at install time'
                      : 'Not declared'
                }}
              </dd>
            </div>
            <div>
              <dt>{{ sourceUnavailable ? 'Last known components' : 'Components' }}</dt>
              <dd>
                {{ plugin.components.length ? plugin.components.join(', ') : 'Not declared' }}
              </dd>
            </div>
            <div>
              <dt>Source</dt>
              <dd>
                <a :href="sourceUrl(plugin)" target="_blank" rel="noreferrer">View on GitHub</a>
              </dd>
            </div>
          </dl>

          <SecurityAssessmentPanel v-if="plugin.security" :plugin="plugin" />
        </article>

        <InstallPanel v-model:targets="targets" v-model:auto-detect="autoDetect" :plugin="plugin" />
      </div>

      <div v-else-if="!discoverySettled" class="community-plugin-state" role="status">
        <h1>Loading plugin…</h1>
        <p>Checking the current community directory.</p>
      </div>
      <div v-else class="community-plugin-state">
        <h1>Plugin not found</h1>
        <p>This package is no longer available in the current directory.</p>
        <NuxtLink class="button button--primary" to="/plugins/">Explore plugins</NuxtLink>
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
