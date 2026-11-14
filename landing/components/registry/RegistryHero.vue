<script setup lang="ts">
import { useInstallPreferencesStore } from '~/stores/installPreferences';
import type { RegistryIndex } from '~/types/registry';
import { installTargets, canonicalInstallTargets, cliInstallTargets, reconcileLocalChatGPTTargets, type InstallTargetID } from '~/data/installTargets';
import { pluginInstallAvailability } from '~/utils/pluginInstallAvailability';
import { pluginTargetDescription } from '~/utils/pluginTargetDescription';
import { pluginCommands } from '~/utils/commands';
import { countAtElapsed, PLUGIN_COUNT_ANIMATION_MS } from '~/utils/countAnimation';
import { catalogVisiblePlugins, groupCatalogPlugins } from '~/utils/filter';
import { expectedDistribution, resolveDistribution } from '~/utils/registry';

const { t, locale } = useI18n();
const localePath = useLocalePath();
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
      if (
        !['current', 'cached', 'stale'].includes(state) ||
        (state === 'stale' && !discovery.value.count)
      )
        return;

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
  installTargets.filter(client => Boolean(pluginInstallAvailability(demoPlugin.value, [client.id]))),
);
const preferences = useInstallPreferencesStore();
const availableTargets = computed(() => compatibleClients.value.map((client) => client.id));
const selectionIdentity = computed(() => `hero:${demoPlugin.value.install_source}`);
const choice = computed(() =>
  preferences.readPackage(selectionIdentity.value, availableTargets.value),
);
const selectedTargets = computed<InstallTargetID[]>(() =>
  choice.value.targetIds.length
    ? reconcileLocalChatGPTTargets(choice.value.targetIds as InstallTargetID[], [])
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
  installTargets.map((client) => ({
    value: client.id,
    label: client.id === 'copilot' ? 'Copilot' : client.name,
    icon: asset(`client-icons/${client.icon}`),
    disabled: !availableTargets.value.includes(client.id),
    description: (() => {
      const hint = pluginTargetDescription(demoPlugin.value, client, {
        scope: 'user', published: published.value, expired: expired.value, current: current.value,
      });
      return t(hint.key, hint.params);
    })(),
  })),
);

const selectedClients = computed(() =>
  compatibleClients.value.filter((client) => selectedTargets.value.includes(client.id)),
);
const resolution = computed(() =>
  resolveDistribution(
    demoPlugin.value,
    canonicalInstallTargets(selectedClients.value.map((client) => client.id)),
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
defineExpose({ fallbackReason });
const command = computed(() => {
  const available = pluginInstallAvailability(demoPlugin.value, selectedTargets.value);
  if (!current.value || (!autoDetect.value && !available))
    return '';
  return pluginCommands(
    demoPlugin.value,
    autoDetect.value ? undefined : cliInstallTargets(selectedClients.value.map((client) => client.id)),
  ).add;
});

function updateTargets(values: string[]) {
  const allowed = new Set(compatibleClients.value.map((client) => client.id));
  const next = values.filter((value): value is InstallTargetID => allowed.has(value as InstallTargetID));
  if (!next.length) return;
  preferences.selectPackage(
    selectionIdentity.value,
    { ...choice.value, targetIds: reconcileLocalChatGPTTargets(next, selectedTargets.value) },
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
        <div class="hero__standards">
          <span class="hero__standards-label">{{ t('shell.hero.supportedFormats') }}</span>
          <div class="hero__standards-links">
            <a href="https://agent-plugins.org/specification" target="_blank" rel="noreferrer">
              Agent Plugins 1.0
            </a>
            <span aria-hidden="true">·</span>
            <a href="https://agentskills.io/specification" target="_blank" rel="noreferrer">
              Agent Skills
            </a>
          </div>
        </div>
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
        <div class="hero__install">
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
                  <div class="hero-quick-start__caption">
                    <span class="hero-quick-start__number">1</span>
                    <span class="hero-quick-start__label"
                      ><strong>{{ t('shell.hero.choose') }}</strong></span
                    >
                  </div>
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
                  <div class="hero-quick-start__caption">
                    <span class="hero-quick-start__number hero-quick-start__number--run">2</span>
                    <span class="hero-quick-start__label"
                      ><strong>{{ t('shell.hero.run') }}</strong></span
                    >
                  </div>
                  <div class="hero-quick-start__command">
                    <CommandSnippet v-if="command" :command="command" kind="add" inline />
                    <p v-else class="install-panel__notice" role="status">
                      {{ t('shell.hero.refreshNotice') }}
                    </p>
                    <p
                      v-if="command && !autoDetect && selectedTargets.includes('chatgptweb')"
                      class="install-panel__notice"
                      role="status"
                    >
                      {{ t('shell.hero.chatgptSetup') }}
                    </p>
                    <NuxtLink
                      class="hero-quick-start__more-install"
                      :to="localePath('/download/')"
                      >{{ t('shell.hero.otherMethods') }} <span aria-hidden="true">→</span></NuxtLink
                    >
                  </div>
                </li>
              </ol>
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
  <section class="demo-showcase" :aria-label="t('shell.hero.demoVideoTitle')">
    <HeroDemoVideo />
  </section>
</template>
