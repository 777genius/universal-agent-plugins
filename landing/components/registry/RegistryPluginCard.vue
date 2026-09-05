<!--
  Card composition adapted from plugin-kit-ai landing/components/plugins/PluginCard.vue (MIT).
  Content model and implementation are new for Universal Agent Plugins.
-->
<script setup lang="ts">
import type { RegistryPlugin } from '~/types/registry';
import {
  authenticationLabel,
  deliveryLabel,
  expectedDistribution,
  githubSourceUrl,
  resolveDistribution,
} from '~/utils/registry';
import { pluginCommands } from '~/utils/commands';

const props = defineProps<{ plugin: RegistryPlugin }>();
const { asset, pluginIcon } = useSite();
const { current, expired, published } = useDirectoryStatus();
const isDiscovered = computed(() => props.plugin.trust_state === 'conformant_unreviewed');
const availableClients = computed(() =>
  clients.filter((client) => props.plugin.client_support.clients.includes(client.id)),
);
const deliverySummary = computed(() => {
  const delivery = props.plugin.client_support.delivery;
  const managed = availableClients.value.filter(
    (client) => delivery[client.id] === 'managed',
  ).length;
  const prepared = availableClients.value.filter((client) =>
    ['prepared', 'manual_activation'].includes(delivery[client.id] ?? ''),
  ).length;
  if (managed + prepared < availableClients.value.length || !availableClients.value.length)
    return 'Agent compatibility checked when you run the command';
  return `${managed} installed automatically · ${prepared} require one final step`;
});
const initialTarget =
  availableClients.value.find((client) => client.id === 'cursor')?.id ??
  availableClients.value[0]?.id;
const targets = ref<(typeof clients)[number]['id'][]>(initialTarget ? [initialTarget] : []);
const autoDetect = ref(true);
const installExpanded = ref(false);
const installPanelId = `plugin-install-${props.plugin.install_source.replace(/[^a-z0-9_-]+/gi, '-')}`;
const resolution = computed(() => resolveDistribution(props.plugin, targets.value));
const selectedDistribution = computed(() =>
  isDiscovered.value || current.value ? resolution.value.distribution : undefined,
);
const canInstall = computed(() =>
  isDiscovered.value
    ? props.plugin.installable && (autoDetect.value || Boolean(selectedDistribution.value))
    : current.value &&
      (autoDetect.value ? props.plugin.installable : Boolean(selectedDistribution.value)),
);
const command = computed(() =>
  canInstall.value
    ? pluginCommands(props.plugin, autoDetect.value ? undefined : targets.value).add
    : '',
);
const iconURL = computed(() => pluginIcon(props.plugin));
const autoOption = {
  label: 'All installed agents (recommended)',
  summary: 'All installed agents',
  description: 'Detected when you run the command',
};
const targetOptions = computed(() =>
  clients.map((client) => ({
    value: client.id,
    label: client.name,
    icon: asset(`client-icons/${client.icon}`),
    disabled: !props.plugin.client_support.clients.includes(client.id),
    description: (() => {
      if (isDiscovered.value)
        return props.plugin.discovery?.availability === 'available'
          ? 'Checked again before installation'
          : 'Unavailable at its indexed source';
      if (!published.value) return 'Unavailable: review data is not installation authority';
      if (expired.value) return 'Unavailable: signed Directory snapshot expired';
      const source = expectedDistribution(props.plugin, [client.id]);
      const target = source?.targets.find((item) => item.client === client.id);
      if (client.id === 'chatgpt')
        return target?.app_binding
          ? 'Verified connection; finish setup in ChatGPT'
          : 'Not available for ChatGPT';
      return target ? deliveryLabel(target.delivery) : 'Not installable from an active release';
    })(),
  })),
);
const authLabel = computed(() =>
  authenticationLabel(resolution.value.distribution, targets.value, props.plugin.authentication),
);
const showAuthentication = computed(
  () => !autoDetect.value && authLabel.value !== 'No account required',
);
const repositoryStars = computed(() =>
  new Intl.NumberFormat('en', {
    notation: 'compact',
    maximumFractionDigits: 1,
  }).format(props.plugin.discovery?.stars ?? 0),
);
const provenanceURL = computed(() => {
  const source = selectedDistribution.value?.source ?? props.plugin.source;
  if (!source?.revision) return '';
  const suffix = source.path ? `/${source.path}` : '';
  return `https://github.com/${source.repository}/tree/${source.revision}${suffix}`;
});
const detailURL = computed(() =>
  isDiscovered.value
    ? { path: '/plugins/community/', query: { source: props.plugin.install_source } }
    : `/plugins/${props.plugin.name}/`,
);
const securityDetailURL = computed(() =>
  isDiscovered.value
    ? {
        path: '/plugins/community/',
        query: { source: props.plugin.install_source },
        hash: '#security-review',
      }
    : `/plugins/${props.plugin.name}/#security-review`,
);

function updateTargets(values: string[]) {
  const allowed = new Set(availableClients.value.map((client) => client.id));
  const next = values.filter((value): value is (typeof clients)[number]['id'] =>
    allowed.has(value as (typeof clients)[number]['id']),
  );
  if (next.length) targets.value = next;
}

function updateAutoDetect(value: boolean) {
  autoDetect.value = value;
}
</script>

<template>
  <article
    class="plugin-card"
    :class="{ 'plugin-card--expanded': installExpanded }"
    :data-trust="isDiscovered ? 'community' : 'reviewed'"
    :data-install-source="plugin.install_source"
  >
    <span
      v-if="!isDiscovered"
      class="plugin-card__ribbon"
      :class="{ 'plugin-card__ribbon--muted': !canInstall }"
      >{{
        canInstall
          ? 'Reviewed listing'
          : expired
            ? 'Temporarily paused'
            : !published
              ? 'Preview only'
              : 'Not available'
      }}</span
    >
    <div
      class="plugin-card__identity"
      :class="{
        'plugin-card__identity--no-icon': !iconURL,
        'plugin-card__identity--with-ribbon': !isDiscovered,
      }"
    >
      <span v-if="iconURL" class="plugin-card__icon"
        ><img :src="iconURL" alt="" width="32" height="32" loading="lazy"
      ></span>
      <div class="plugin-card__identity-copy">
        <h3>
          <NuxtLink class="plugin-card__title-link" :to="detailURL">{{
            plugin.display_name
          }}</NuxtLink>
        </h3>
        <p v-if="isDiscovered" class="plugin-card__source-label">
          {{
            plugin.discovery?.availability === 'available'
              ? 'Found on GitHub'
              : 'Currently unavailable'
          }}
          · {{ plugin.source.repository }}{{ plugin.source.path ? `/${plugin.source.path}` : '' }}
        </p>
        <p v-else-if="canInstall" class="plugin-card__source-label">
          {{ deliverySummary }}
        </p>
      </div>
    </div>
    <p
      v-if="isDiscovered"
      class="plugin-card__author plugin-card__popularity"
      title="Stars belong to the GitHub repository, not this individual package"
    >
      <span aria-hidden="true">★</span> {{ repositoryStars }} stars on repo · Agent Plugins 1.0
    </p>
    <SecurityAssessmentBadge
      v-if="plugin.security"
      :plugin="plugin"
      :details-to="securityDetailURL"
    />
    <p class="plugin-card__description">{{ plugin.description }}</p>
    <div class="plugin-card__bottom">
      <button
        v-if="canInstall"
        type="button"
        class="plugin-card__install-toggle"
        :aria-controls="installPanelId"
        :aria-expanded="installExpanded"
        :aria-label="`Install ${plugin.display_name}`"
        @click="installExpanded = !installExpanded"
      >
        Install
      </button>
      <span v-else class="plugin-card__unavailable">{{
        isDiscovered
          ? 'Unavailable at its indexed source; no install command is generated.'
          : expired
            ? 'Commands disabled because the Directory is temporarily stale.'
            : !published
              ? 'Commands disabled in preview.'
              : resolution.unavailable_reason
      }}</span>
      <Transition name="plugin-install">
        <div
          v-if="installExpanded && canInstall"
          :id="installPanelId"
          class="plugin-card__install-panel"
        >
          <p v-if="!autoDetect && selectedDistribution" class="plugin-card__author">
            By {{ selectedDistribution.publisher }} ·
            <a
              :href="isDiscovered ? provenanceURL : githubSourceUrl(plugin, selectedDistribution)"
              target="_blank"
              rel="noreferrer"
              >View source <span class="sr-only">for {{ plugin.name }}</span></a
            >
          </p>
          <p
            v-if="!autoDetect && resolution.fallback_reason && current"
            class="plugin-card__author"
          >
            {{ resolution.fallback_reason }}
          </p>
          <p v-if="showAuthentication" class="plugin-card__auth">{{ authLabel }}</p>
          <ul
            v-if="!autoDetect && selectedDistribution"
            class="badge-list"
            aria-label="Install candidate components"
          >
            <li v-for="component in selectedDistribution.components" :key="component">
              {{ component }}
            </li>
          </ul>
          <div class="plugin-card__install">
            <AppMultiSelect
              v-if="targets.length"
              :model-value="targets"
              :auto-selected="autoDetect"
              :auto-option="autoOption"
              :label="`Choose clients for ${plugin.display_name}`"
              :options="targetOptions"
              @update:auto-selected="updateAutoDetect"
              @update:model-value="updateTargets"
            />
            <CommandSnippet label="Add" kind="add" :command="command" compact />
          </div>
        </div>
      </Transition>
    </div>
  </article>
</template>
