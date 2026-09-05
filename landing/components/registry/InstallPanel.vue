<script setup lang="ts">
import type { ClientID, RegistryPlugin } from '~/types/registry';
import { pluginCommands } from '~/utils/commands';
import { deliveryLabel, expectedDistribution, resolveDistribution } from '~/utils/registry';

const props = defineProps<{ plugin: RegistryPlugin }>();
const targets = defineModel<ClientID[]>('targets', { required: true });
const autoDetect = defineModel<boolean>('autoDetect', { required: true });
const { asset, sourceUrl } = useSite();
const { current, expired, published } = useDirectoryStatus();
const autoOption = {
  label: 'All installed agents (recommended)',
  summary: 'All installed agents',
  description: 'Detected when you run the command',
};
const availableClients = computed(() =>
  clients.filter((client) => props.plugin.client_support.clients.includes(client.id)),
);
const targetOptions = computed(() =>
  clients.map((client) => ({
    value: client.id,
    label: client.name,
    icon: asset(`client-icons/${client.icon}`),
    disabled: !props.plugin.client_support.clients.includes(client.id),
    description: (() => {
      if (!published.value) return 'Unavailable: review data is not installation authority';
      if (expired.value) return 'Unavailable: signed Directory snapshot expired';
      const source = expectedDistribution(props.plugin, [client.id]);
      const target = source?.targets.find((item) => item.client === client.id);
      if (client.id === 'chatgpt')
        return target?.app_binding
          ? 'Verified connection; finish setup in ChatGPT'
          : 'Not available for ChatGPT';
      if (target) return deliveryLabel(target.delivery);
      return 'No active release supports this client';
    })(),
  })),
);
const commands = computed(() =>
  props.plugin.installable &&
  current.value &&
  (autoDetect.value || (targets.value.length > 0 && hasCompleteSource.value))
    ? pluginCommands(props.plugin, autoDetect.value ? undefined : targets.value)
    : undefined,
);
const resolution = computed(() => resolveDistribution(props.plugin, targets.value));
const expectedSource = computed(() => (current.value ? resolution.value.distribution : undefined));
const hasCompleteSource = computed(() => Boolean(expectedSource.value));
const selectedTargets = computed(
  () =>
    expectedSource.value?.targets.filter((target) => targets.value.includes(target.client)) ?? [],
);
const chatgptSelected = computed(
  () =>
    !autoDetect.value &&
    selectedTargets.value.some((target) => target.client === 'chatgpt' && target.app_binding),
);
const unavailableDiscoveryReason = computed(() => {
  if (props.plugin.installable) return '';
  if (props.plugin.discovery?.availability === 'unavailable')
    return 'This package is no longer available from its source.';
  if (!props.plugin.components.length)
    return "We found this project, but it doesn't include any tools the installer can add yet.";
  return 'This package does not support any of the agents available in the installer yet.';
});

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

watch(availableClients, (next) => {
  const allowed = new Set(next.map((client) => client.id));
  const retained = targets.value.filter((target) => allowed.has(target));
  targets.value = retained.length ? retained : next[0] ? [next[0].id] : [];
});
</script>

<template>
  <aside class="install-panel" aria-labelledby="install-title">
    <div class="install-panel__heading">
      <div>
        <p class="eyebrow">Installer</p>
        <h2 id="install-title">
          {{ unavailableDiscoveryReason ? 'Not ready to install' : 'Use with your agent' }}
        </h2>
      </div>
      <a
        v-if="!unavailableDiscoveryReason"
        class="install-panel__cli-link"
        href="https://github.com/777genius/universal-agent-plugins#quick-start"
        target="_blank"
        rel="noreferrer"
        >Run with npx</a
      >
    </div>
    <div v-if="unavailableDiscoveryReason" class="install-panel__empty" role="status">
      <span class="install-panel__empty-icon" aria-hidden="true">i</span>
      <div>
        <h3>This plugin is not installable yet</h3>
        <p>{{ unavailableDiscoveryReason }}</p>
        <a :href="sourceUrl(plugin)" target="_blank" rel="noreferrer">View package source</a>
      </div>
    </div>
    <div v-else-if="commands" class="command-stack">
      <div class="install-command-row">
        <div class="target-select">
          <span>Agents</span
          ><AppMultiSelect
            :model-value="targets"
            :auto-selected="autoDetect"
            :auto-option="autoOption"
            label="Choose target agents"
            :options="targetOptions"
            @update:auto-selected="updateAutoDetect"
            @update:model-value="updateTargets"
          />
        </div>
        <CommandSnippet label="Add" kind="add" :command="commands.add" />
      </div>
      <CommandSnippet label="Update" kind="update" :command="commands.update" />
      <CommandSnippet label="Repair" kind="repair" :command="commands.repair" />
      <CommandSnippet label="Remove" kind="remove" :command="commands.remove" />
    </div>
    <p v-if="!unavailableDiscoveryReason && expired" class="install-panel__notice" role="status">
      <strong>Commands unavailable: stale Directory.</strong> This signed snapshot has expired.
      Browse its history, then return after a fresh snapshot is published.
    </p>
    <p
      v-else-if="!unavailableDiscoveryReason && !published"
      class="install-panel__notice"
      role="status"
    >
      <strong>Commands unavailable in review preview.</strong> Unresolved data is for review only;
      production commands require a published signed Directory snapshot.
    </p>
    <p
      v-else-if="!unavailableDiscoveryReason && !autoDetect && !hasCompleteSource"
      class="install-panel__notice"
      role="status"
    >
      <strong>Commands unavailable.</strong> {{ resolution.unavailable_reason }}
    </p>
    <p v-if="autoDetect && commands" class="install-panel__notice">
      <strong>Automatic detection.</strong> The CLI checks this plugin against installed agents,
      skips incompatible ones, and lets you confirm the targets. ChatGPT is included only when this
      plugin provides a verified connection.
    </p>
    <p v-if="chatgptSelected" class="install-panel__notice">
      <strong>One step remains in ChatGPT.</strong> Open ChatGPT, select
      {{ plugin.display_name }} in Apps or Plugins, connect it, and start a new chat. Availability
      depends on your account and workspace.
    </p>
    <p
      v-if="!unavailableDiscoveryReason && !autoDetect && resolution.fallback_reason && current"
      class="install-panel__notice"
    >
      <strong>Expected source fallback: {{ expectedSource?.label }}.</strong>
      {{ resolution.fallback_reason }}
    </p>
    <p
      v-else-if="!unavailableDiscoveryReason && !autoDetect && !hasCompleteSource && current"
      class="install-panel__notice"
    >
      <strong>No single source serves this target set.</strong> The CLI will fail before mutation
      and suggest compatible target/source combinations; it never mixes distributions across
      clients.
    </p>
    <p v-if="commands" class="install-panel__footnote">
      The CLI checks every selected agent before changing files and shows any sign-in or activation
      step. OAuth credentials stay with the agent.
    </p>
  </aside>
</template>

<style scoped>
.install-panel {
  min-width: 0;
  width: 100%;
}

.target-select :deep(.app-multiselect__trigger) {
  padding-block: 10px;
  text-transform: none;
  letter-spacing: normal;
}

.target-select :deep(.app-multiselect__value > span:last-child) {
  overflow: visible;
  white-space: normal;
  overflow-wrap: anywhere;
  text-align: left;
}
</style>
