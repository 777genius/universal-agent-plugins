<script setup lang="ts">
import { useInstallPreferencesStore } from '~/stores/installPreferences';
import type { ClientID, RegistryIndex } from '~/types/registry';
import { pluginCommands } from '~/utils/commands';
import { countAtElapsed, PLUGIN_COUNT_ANIMATION_MS } from '~/utils/countAnimation';
import { catalogVisiblePlugins, groupCatalogPlugins } from '~/utils/filter';
import { expectedDistribution, resolveDistribution } from '~/utils/registry';

const { t, locale } = useI18n();
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
    : new Intl.NumberFormat(locale.value).format(displayedPluginCount.value),
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
const preferences = useInstallPreferencesStore();
const availableTargets = computed(() => compatibleClients.value.map((client) => client.id));
const selectionIdentity = computed(() => `hero:${demoPlugin.value.install_source}`);
const choice = computed(() =>
  preferences.readPackage(selectionIdentity.value, availableTargets.value),
);
const selectedTargets = computed<ClientID[]>(() =>
  choice.value.targetIds.length
    ? (choice.value.targetIds as ClientID[])
    : [
        compatibleClients.value.find((client) => client.id === 'cursor')?.id ??
          availableTargets.value[0]!,
      ],
);
const autoDetect = computed({
  get: () => choice.value.autoDetect,
  set: (value: boolean) =>
    preferences.selectPackage(
      selectionIdentity.value,
      { ...choice.value, targetIds: selectedTargets.value, autoDetect: value },
      availableTargets.value,
    ),
});
const autoOption = computed(() => ({
  label: t('shell.hero.autoLabel'),
  summary: t('shell.hero.autoSummary'),
  description: t('shell.hero.autoDescription'),
}));

const targetOptions = computed(() =>
  clients.map((client) => ({
    value: client.id,
    label: client.id === 'copilot' ? 'Copilot' : client.name,
    icon: asset(`client-icons/${client.icon}`),
    disabled: !demoPlugin.value.client_support.clients.includes(client.id),
    description: (() => {
      if (!published.value) return t('shell.hero.unavailable');
      if (expired.value) return t('shell.hero.refreshing');
      const target = expectedDistribution(demoPlugin.value, [client.id])?.targets.find(
        (item) => item.client === client.id,
      );
      if (client.id === 'chatgpt')
        return target?.app_binding ? t('shell.hero.chatgptSetup') : t('shell.hero.notAvailable');
      return target
        ? t(`shell.hero.delivery.${target.delivery}`)
        : t('shell.hero.notAvailableForPlugin');
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
// Additive descriptors are supplied by the registry lane. Older/unknown diagnostics
// remain original text; presentation never parses English to make domain decisions.
type ShellDiagnostic = {
  code: string;
  params?: Record<string, string | number>;
  reasons?: ShellDiagnostic[];
};
function diagnosticText(diagnostic: ShellDiagnostic): string | null {
  const codes = [
    'distributionStatus',
    'releaseStatus',
    'missingComponents',
    'unsupportedTargets',
    'blockingFailure',
    'missingEvidence',
    'noReleases',
    'defaultIneligible',
  ];
  const nested = (diagnostic.reasons ?? []).map(diagnosticText);
  if (nested.some((reason) => reason === null)) return null;
  const reasons = nested.join(t('shell.hero.diagnostics.reasonSeparator'));
  if (diagnostic.code === 'reasons') return reasons;
  if (!codes.includes(diagnostic.code)) return null;
  const params = { ...diagnostic.params };
  if (
    typeof params.status === 'string' &&
    ['candidate', 'active', 'suspended', 'superseded', 'revoked'].includes(params.status)
  ) {
    params.status = t(`shell.hero.diagnostics.status.${params.status}`);
  }
  return t(`shell.hero.diagnostics.${diagnostic.code}`, { ...params, reasons });
}
const fallbackReason = computed(() => {
  const diagnostic = (
    resolution.value as typeof resolution.value & {
      fallback_diagnostic?: ShellDiagnostic;
    }
  ).fallback_diagnostic;
  return (
    (diagnostic && diagnosticText(diagnostic)) ??
    t('shell.hero.sourceReason', { reason: resolution.value.fallback_reason ?? '' })
  );
});
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
  if (next.length)
    preferences.selectPackage(
      selectionIdentity.value,
      { ...choice.value, targetIds: next },
      availableTargets.value,
    );
}
</script>

<template>
  <section class="hero-shell">
    <div class="hero container">
      <div class="hero__copy">
        <h1>
          {{ t('shell.hero.title') }}<br ><em>{{ t('shell.hero.subtitle') }}</em>
        </h1>
        <p class="hero__lead">
          {{ t('shell.hero.intro') }}
        </p>
        <div class="hero__actions">
          <a class="button button--primary" href="#plugins">
            <i18n-t
              v-if="pluginCount !== null"
              keypath="shell.hero.exploreCount"
              tag="span"
              :plural="displayedPluginCount ?? 0"
            >
              <template #count
                ><span class="hero__plugin-count">{{ pluginCount }}</span></template
              >
            </i18n-t>
            <span v-else>{{ t('shell.hero.explore') }}</span> <span aria-hidden="true">→</span>
          </a>
          <a
            class="button button--secondary"
            :href="cliRepositoryUrl"
            target="_blank"
            rel="noreferrer"
            >{{ t('shell.hero.github') }}</a
          >
        </div>
      </div>

      <div class="hero__demo">
        <HeroAgentField />
        <div class="hero__window">
          <div class="hero__window-body">
            <div class="hero-quick-start__header">
              <h2>{{ t('shell.hero.install', { name: demoPlugin.display_name }) }}</h2>
              <a href="https://agent-plugins.org/specification" target="_blank" rel="noreferrer"
                >Agent Plugins 1.0</a
              >
            </div>
            <ol class="hero-quick-start__steps">
              <li class="hero-quick-start__step">
                <span class="hero-quick-start__number">1</span>
                <span class="hero-quick-start__label"
                  ><strong>{{ t('shell.hero.choose') }}</strong
                  ><small>{{ t('shell.hero.chooseHint') }}</small></span
                >
                <AppMultiSelect
                  :model-value="selectedTargets"
                  :auto-selected="autoDetect"
                  :auto-option="autoOption"
                  :label="t('shell.accessibility.chooseTargets')"
                  :options="targetOptions"
                  @update:auto-selected="autoDetect = $event"
                  @update:model-value="updateTargets"
                />
              </li>
              <li class="hero-quick-start__step">
                <span class="hero-quick-start__number hero-quick-start__number--run">2</span>
                <span class="hero-quick-start__label"
                  ><strong>{{ t('shell.hero.run') }}</strong
                  ><small>{{ t('shell.hero.terminal') }}</small></span
                >
                <div class="hero-quick-start__command">
                  <CommandSnippet v-if="command" :command="command" kind="add" inline />
                  <p v-else class="install-panel__notice" role="status">
                    {{ t('shell.hero.refreshNotice') }}
                  </p>
                  <a
                    class="hero-quick-start__more-install"
                    href="https://github.com/777genius/universal-agent-plugins#quick-start"
                    target="_blank"
                    rel="noreferrer"
                    >{{ t('shell.hero.otherMethods') }} <span aria-hidden="true">↗</span></a
                  >
                </div>
              </li>
            </ol>
            <div class="hero-quick-start__footer">
              <p><span aria-hidden="true">✓</span> {{ t('shell.hero.planNotice') }}</p>
              <p v-if="!autoDetect && resolution.fallback_reason">
                {{ fallbackReason }}
              </p>
            </div>
          </div>
        </div>
      </div>
    </div>

    <div class="client-section" aria-labelledby="supported-clients-title">
      <div class="client-section__inner container">
        <p id="supported-clients-title">{{ t('shell.hero.supportedClients') }}</p>
        <ClientStrip />
        <p class="client-section__note">
          {{ t('shell.hero.clientNote') }}
        </p>
      </div>
    </div>
  </section>
</template>
