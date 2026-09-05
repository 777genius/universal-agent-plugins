<script setup lang="ts">
import type { ClientID, RegistryIndex } from '~/types/registry';
import { pluginCommands } from '~/utils/commands';
import { countAtElapsed, PLUGIN_COUNT_ANIMATION_MS } from '~/utils/countAnimation';
import { catalogVisiblePlugins, groupCatalogPlugins } from '~/utils/filter';
import { deliveryLabel, expectedDistribution, resolveDistribution } from '~/utils/registry';

const props = defineProps<{ registry: RegistryIndex }>();
const config = useRuntimeConfig();
const { current, expired, published } = useDirectoryStatus();
const discovery = useDiscoveryStatus();
const { asset } = useSite();
const cliRepositoryUrl = computed(() => `https://github.com/${config.public.githubRepo}`);
const preferredDemoNames = ['context7', 'chrome-devtools', 'cloudflare-docs'];
const displayedPluginCount = ref<number | null>(null);
const pluginCount = computed(() =>
  displayedPluginCount.value === null
    ? null
    : new Intl.NumberFormat('en').format(displayedPluginCount.value),
);
const countAnimationCompleted = useState('hero-plugin-count-animated', () => false);
let countAnimationFrame: number | undefined;

if (import.meta.client) {
  watch(
    () => discovery.value.state,
    (state) => {
      if (state !== 'current' && state !== 'cached') return;

      const target = groupCatalogPlugins(catalogVisiblePlugins(props.registry.plugins)).length;
      if (
        countAnimationCompleted.value ||
        window.matchMedia('(prefers-reduced-motion: reduce)').matches
      ) {
        displayedPluginCount.value = target;
        countAnimationCompleted.value = true;
        return;
      }

      countAnimationCompleted.value = true;
      displayedPluginCount.value = 0;
      const startedAt = performance.now();
      const updateCount = (now: number) => {
        const elapsed = now - startedAt;
        displayedPluginCount.value = countAtElapsed(target, elapsed);
        if (elapsed < PLUGIN_COUNT_ANIMATION_MS) {
          countAnimationFrame = requestAnimationFrame(updateCount);
        }
      };
      countAnimationFrame = requestAnimationFrame(updateCount);
    },
    { immediate: true },
  );

  onBeforeUnmount(() => {
    if (countAnimationFrame !== undefined) cancelAnimationFrame(countAnimationFrame);
  });
}

const demoPlugin = computed(() => {
  const ranked = [
    ...preferredDemoNames.flatMap((name) =>
      props.registry.plugins.filter((item) => item.name === name),
    ),
    ...props.registry.plugins.filter((item) => !preferredDemoNames.includes(item.name)),
  ];
  const plugin = ranked.find((item) =>
    item.client_support.clients.some((client) => Boolean(expectedDistribution(item, [client]))),
  );
  if (!plugin) throw new Error('The homepage requires one installable Directory plugin');
  return plugin;
});

const compatibleClients = computed(() =>
  clients.filter((client) => demoPlugin.value.client_support.clients.includes(client.id)),
);
const initialTarget =
  compatibleClients.value.find((client) => client.id === 'cursor')?.id ??
  compatibleClients.value[0]!.id;
const selectedTargets = ref<ClientID[]>([initialTarget]);
const autoDetect = ref(true);
const autoOption = {
  label: 'All installed agents (recommended)',
  summary: 'All installed agents',
  description: 'Detected when you run the command',
};

const targetOptions = computed(() =>
  clients.map((client) => ({
    value: client.id,
    label: client.id === 'copilot' ? 'Copilot' : client.name,
    icon: asset(`client-icons/${client.icon}`),
    disabled: !demoPlugin.value.client_support.clients.includes(client.id),
    description: (() => {
      if (!published.value) return 'Temporarily unavailable';
      if (expired.value) return 'Refreshing plugin data';
      const target = expectedDistribution(demoPlugin.value, [client.id])?.targets.find(
        (item) => item.client === client.id,
      );
      if (client.id === 'chatgpt')
        return target?.app_binding ? 'Finish setup in ChatGPT' : 'Not available';
      return target ? deliveryLabel(target.delivery) : 'Not available for this plugin';
    })(),
  })),
);

const selectedClients = computed(() =>
  compatibleClients.value.filter((client) => selectedTargets.value.includes(client.id)),
);
const resolution = computed(() =>
  resolveDistribution(
    demoPlugin.value,
    selectedClients.value.map((client) => client.id),
  ),
);
const command = computed(() => {
  if (!current.value || (!autoDetect.value && !resolution.value.distribution)) return '';
  return pluginCommands(
    demoPlugin.value,
    autoDetect.value ? undefined : selectedClients.value.map((client) => client.id),
  ).add;
});

function updateTargets(values: string[]) {
  const allowed = new Set(compatibleClients.value.map((client) => client.id));
  const next = values.filter((value): value is ClientID => allowed.has(value as ClientID));
  if (next.length) selectedTargets.value = next;
}
</script>

<template>
  <section class="hero-shell">
    <div class="hero container">
      <div class="hero__copy">
        <h1>One plugin<br /><em>All your agents</em></h1>
        <p class="hero__lead">
          Install, update, repair, and remove Agent Plugins 1.0 across supported AI agents with one
          command. Let the CLI detect installed agents, or choose exactly where the plugin goes.
        </p>
        <div class="hero__actions">
          <a class="button button--primary" href="#plugins">
            Explore
            <span v-if="pluginCount !== null" class="hero__plugin-count">{{ pluginCount }}</span>
            plugins <span aria-hidden="true">→</span>
          </a>
          <a
            class="button button--secondary"
            :href="cliRepositoryUrl"
            target="_blank"
            rel="noreferrer"
            >Open GitHub</a
          >
        </div>
      </div>

      <div class="hero__demo">
        <HeroAgentField />
        <div class="hero__window">
          <div class="hero__window-body">
            <div class="hero-quick-start__header">
              <h2>Install {{ demoPlugin.display_name }}</h2>
              <a href="https://agent-plugins.org/specification" target="_blank" rel="noreferrer"
                >Agent Plugins 1.0</a
              >
            </div>
            <ol class="hero-quick-start__steps">
              <li class="hero-quick-start__step">
                <span class="hero-quick-start__number">1</span>
                <span class="hero-quick-start__label"
                  ><strong>Choose agents</strong><small>One, many, or auto-detect</small></span
                >
                <AppMultiSelect
                  :model-value="selectedTargets"
                  :auto-selected="autoDetect"
                  :auto-option="autoOption"
                  label="Choose target agents"
                  :options="targetOptions"
                  @update:auto-selected="autoDetect = $event"
                  @update:model-value="updateTargets"
                />
              </li>
              <li class="hero-quick-start__step">
                <span class="hero-quick-start__number hero-quick-start__number--run">2</span>
                <span class="hero-quick-start__label"
                  ><strong>Copy and run</strong><small>In your terminal</small></span
                >
                <CommandSnippet v-if="command" :command="command" kind="add" inline />
                <p v-else class="install-panel__notice" role="status">
                  Plugin data is refreshing. Try again shortly.
                </p>
              </li>
            </ol>
            <div class="hero-quick-start__footer">
              <p>
                <span aria-hidden="true">✓</span> One command plans every selected agent before
                changing files
              </p>
              <p v-if="!autoDetect && resolution.fallback_reason">
                {{ resolution.fallback_reason }}
              </p>
            </div>
          </div>
        </div>
      </div>
    </div>

    <div class="client-section" aria-labelledby="supported-clients-title">
      <div class="client-section__inner container">
        <p id="supported-clients-title">Supported clients</p>
        <ClientStrip />
        <p class="client-section__note">
          The CLI installs or prepares the native format each client supports. Some clients may ask
          you to finish activation or sign in.
        </p>
      </div>
    </div>
  </section>
</template>
