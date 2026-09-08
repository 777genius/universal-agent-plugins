<!--
  Card composition adapted from plugin-kit-ai landing/components/plugins/PluginCard.vue (MIT).
  Content model and implementation are new for Universal Agent Plugins.
-->
<script setup lang="ts">
import { useInstallPreferencesStore } from '~/stores/installPreferences';
import type { RegistryDiagnostic, RegistryPlugin } from '~/types/registry';
import {
  authenticationState,
  expectedDistribution,
  githubSourceUrl,
  resolveDistribution,
} from '~/utils/registry';
import { pluginCommands } from '~/utils/commands';
const { t, locale, n } = useI18n();
const localePath = useLocalePath();

const props = withDefaults(
  defineProps<{ plugin: RegistryPlugin; alternatives?: RegistryPlugin[] }>(),
  { alternatives: () => [] },
);
const { asset, pluginIcon, sourceUrl } = useSite();
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
    return t('registryUi.card.agentCompatibilityCheckedWhenYouRunTheCommand');
  return t('registryUi.card.deliverySummary', { managed: n(managed), prepared: n(prepared) });
});
const initialTarget =
  availableClients.value.find((client) => client.id === 'cursor')?.id ??
  availableClients.value[0]?.id;
const preferences = useInstallPreferencesStore();
if (preferences.package.identity === props.plugin.install_source) {
  preferences.reconcilePackage(
    props.plugin.install_source,
    availableClients.value.map((client) => client.id),
  );
}
const savedChoice = preferences.readPackage(
  props.plugin.install_source,
  availableClients.value.map((client) => client.id),
);
const targets = ref<(typeof clients)[number]['id'][]>(
  savedChoice.targetIds.length
    ? (savedChoice.targetIds as (typeof clients)[number]['id'][])
    : initialTarget
      ? [initialTarget]
      : [],
);
const autoDetect = ref(savedChoice.autoDetect);
const installExpanded = ref(savedChoice.expanded);
function saveChoice() {
  preferences.selectPackage(
    props.plugin.install_source,
    {
      targetIds: targets.value,
      autoDetect: autoDetect.value,
      expanded: installExpanded.value,
    },
    availableClients.value.map((client) => client.id),
  );
}
function toggleInstall() {
  installExpanded.value = !installExpanded.value;
  saveChoice();
}
watch(availableClients, (next) => {
  const allowed = new Set(next.map((client) => client.id));
  if (targets.value.some((id) => !allowed.has(id))) {
    targets.value = next[0] ? [next[0].id] : [];
    autoDetect.value = true;
    if (preferences.package.identity === props.plugin.install_source)
      preferences.reconcilePackage(props.plugin.install_source, [...allowed]);
  }
});
const installPanelId = `plugin-install-${props.plugin.install_source.replace(/[^a-z0-9_-]+/gi, '-')}`;
function diagnosticText(
  diagnostic: RegistryDiagnostic | undefined,
  original: string | undefined,
): string {
  if (!diagnostic) return original ? t('registryUi.reason.external', { reason: original }) : '';
  const reasons =
    diagnostic.reasons
      ?.map((reason) => diagnosticText(reason, undefined))
      .join(t('registryUi.reason.separator')) ?? '';
  if (diagnostic.code === 'reasons') return reasons;
  const params: Record<string, string | number> = { ...diagnostic.params, reasons };
  if (diagnostic.params?.status) params.status = t(`registryUi.status.${diagnostic.params.status}`);
  return t(`registryUi.reason.${diagnostic.code}`, params);
}
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
const autoOption = computed(() => ({
  label: t('registryUi.card.allInstalledAgentsRecommended'),
  summary: t('registryUi.card.allInstalledAgents'),
  description: t('registryUi.card.detectedWhenYouRunTheCommand'),
}));
const targetOptions = computed(() =>
  clients.map((client) => ({
    value: client.id,
    label: client.name,
    icon: asset(`client-icons/${client.icon}`),
    disabled: !props.plugin.client_support.clients.includes(client.id),
    description: (() => {
      if (isDiscovered.value)
        return props.plugin.discovery?.availability === 'available'
          ? t('registryUi.card.checkedAgainBeforeInstallation')
          : t('registryUi.card.unavailableAtItsIndexedSource');
      if (!published.value)
        return t('registryUi.card.unavailableReviewDataIsNotInstallationAuthority');
      if (expired.value) return t('registryUi.card.unavailableSignedDirectorySnapshotExpired');
      const source = expectedDistribution(props.plugin, [client.id]);
      const target = source?.targets.find((item) => item.client === client.id);
      if (client.id === 'chatgpt')
        return target?.app_binding
          ? t('registryUi.card.verifiedConnectionFinishSetupInChatgpt')
          : t('registryUi.card.notAvailableForChatgpt');
      return target
        ? t(`registryUi.delivery.${target.delivery}`)
        : t('registryUi.card.notInstallableFromAnActiveRelease');
    })(),
  })),
);
const authLabel = computed(() =>
  t(
    `registryUi.authentication.${authenticationState(resolution.value.distribution, targets.value, props.plugin.authentication)}`,
  ),
);
const showAuthentication = computed(
  () =>
    !autoDetect.value &&
    authenticationState(
      resolution.value.distribution,
      targets.value,
      props.plugin.authentication,
    ) !== 'not_required',
);
const repositoryStars = computed(() =>
  new Intl.NumberFormat(locale.value, {
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
    ? { path: localePath('/plugins/community/'), query: { source: props.plugin.install_source } }
    : localePath(`/plugins/${props.plugin.name}/`),
);
const securityDetailURL = computed(() =>
  isDiscovered.value
    ? {
        path: localePath('/plugins/community/'),
        query: { source: props.plugin.install_source },
        hash: '#security-review',
      }
    : `${localePath(`/plugins/${props.plugin.name}/`)}#security-review`,
);

function alternativeDetailURL(alternative: RegistryPlugin) {
  return alternative.trust_state === 'conformant_unreviewed'
    ? { path: localePath('/plugins/community/'), query: { source: alternative.install_source } }
    : localePath(`/plugins/${alternative.name}/`);
}

function updateTargets(values: string[]) {
  const allowed = new Set(availableClients.value.map((client) => client.id));
  const next = values.filter((value): value is (typeof clients)[number]['id'] =>
    allowed.has(value as (typeof clients)[number]['id']),
  );
  if (next.length) {
    targets.value = next;
    saveChoice();
  }
}

function updateAutoDetect(value: boolean) {
  autoDetect.value = value;
  saveChoice();
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
          ? t('registryUi.card.reviewedListing')
          : expired
            ? t('registryUi.card.temporarilyPaused')
            : !published
              ? t('registryUi.card.previewOnly')
              : t('registryUi.card.notAvailable')
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
      /></span>
      <div class="plugin-card__identity-copy">
        <h3>
          <NuxtLink class="plugin-card__title-link" :to="detailURL">{{
            plugin.display_name
          }}</NuxtLink>
        </h3>
        <p v-if="isDiscovered" class="plugin-card__source-label">
          {{
            plugin.discovery?.availability === 'available'
              ? t('registryUi.card.foundOnGithub')
              : t('registryUi.card.currentlyUnavailable')
          }}
          ·
          <a :href="sourceUrl(plugin)" target="_blank" rel="noreferrer">
            {{ plugin.source.repository }}{{ plugin.source.path ? `/${plugin.source.path}` : '' }}
          </a>
        </p>
        <p v-else-if="canInstall" class="plugin-card__source-label">
          {{ deliverySummary }}
        </p>
      </div>
    </div>
    <p v-if="isDiscovered" class="plugin-card__author plugin-card__popularity">
      <AppTooltip>
        <template #trigger>
          <button type="button" class="plugin-card__popularity-trigger">
            <span aria-hidden="true">★</span>
            {{ t('registryUi.card.stars', { count: repositoryStars }) }}
          </button>
        </template>
        <p class="app-tooltip__compact">
          {{
            t(
              'registryUi.card.thisIsTheGithubRepositorySStarCountNotARatingForThisIndividualPlugin',
            )
          }}
        </p>
      </AppTooltip>
      <span aria-hidden="true"> · </span>{{ t('registryUi.card.agentPlugins10') }}
    </p>
    <SecurityAssessmentBadge
      v-if="plugin.security"
      :plugin="plugin"
      :details-to="securityDetailURL"
    />
    <p class="plugin-card__description">{{ plugin.description }}</p>
    <details v-if="alternatives.length" class="plugin-other-sources">
      <summary>
        {{ t('registryUi.card.otherSources', { count: n(alternatives.length) })
        }}<span class="sr-only">
          {{ t('registryUi.card.forPlugin', { name: plugin.display_name }) }}</span
        >
      </summary>
      <ul class="plugin-other-sources__list">
        <li v-for="alternative in alternatives" :key="alternative.install_source">
          <NuxtLink :to="alternativeDetailURL(alternative)">
            {{ alternative.source.repository
            }}{{ alternative.source.path ? `/${alternative.source.path}` : '' }}
          </NuxtLink>
          <span
            >{{
              alternative.trust_state === 'conformant_unreviewed'
                ? t('registryUi.card.communityListing')
                : t('registryUi.card.reviewedListing')
            }}
            ·
            {{
              alternative.distributions.find((item) => item.id === alternative.default_distribution)
                ?.kind === 'upstream'
                ? t('registryUi.card.upstream')
                : t('registryUi.card.communityDirect')
            }}
            ·
            {{
              alternative.installable
                ? t('registryUi.card.installable')
                : t('registryUi.card.unavailable')
            }}</span
          >
        </li>
      </ul>
    </details>
    <div class="plugin-card__bottom">
      <button
        v-if="canInstall"
        type="button"
        class="plugin-card__install-toggle"
        :aria-controls="installPanelId"
        :aria-expanded="installExpanded"
        :aria-label="t('registryUi.card.installName', { name: plugin.display_name })"
        @click="toggleInstall"
      >
        {{ t('registryUi.card.install') }}
      </button>
      <span v-else class="plugin-card__unavailable">{{
        isDiscovered
          ? t('registryUi.card.unavailableAtItsIndexedSourceNoInstallCommandIsGenerated')
          : expired
            ? t('registryUi.card.commandsDisabledBecauseTheDirectoryIsTemporarilyStale')
            : !published
              ? t('registryUi.card.commandsDisabledInPreview')
              : diagnosticText(resolution.unavailable_diagnostic, resolution.unavailable_reason)
      }}</span>
      <Transition name="plugin-install">
        <div
          v-if="installExpanded && canInstall"
          :id="installPanelId"
          class="plugin-card__install-panel"
        >
          <p v-if="!autoDetect && selectedDistribution" class="plugin-card__author">
            {{ t('registryUi.card.publisher', { publisher: selectedDistribution.publisher }) }} ·
            <a
              :href="isDiscovered ? provenanceURL : githubSourceUrl(plugin, selectedDistribution)"
              target="_blank"
              rel="noreferrer"
              >{{ t('registryUi.card.viewSource') }}
              <span class="sr-only">{{
                t('registryUi.card.forPlugin', { name: plugin.name })
              }}</span></a
            >
          </p>
          <p
            v-if="!autoDetect && resolution.fallback_reason && current"
            class="plugin-card__author"
          >
            {{ diagnosticText(resolution.fallback_diagnostic, resolution.fallback_reason) }}
          </p>
          <p v-if="showAuthentication" class="plugin-card__auth">{{ authLabel }}</p>
          <ul
            v-if="!autoDetect && selectedDistribution"
            class="badge-list"
            :aria-label="t('registryUi.card.installCandidateComponents')"
          >
            <li v-for="component in selectedDistribution.components" :key="component">
              {{ t(`registryUi.components.${component}`) }}
            </li>
          </ul>
          <div class="plugin-card__install">
            <AppMultiSelect
              v-if="targets.length"
              :model-value="targets"
              :auto-selected="autoDetect"
              :auto-option="autoOption"
              :label="t('registryUi.card.chooseClients', { name: plugin.display_name })"
              :options="targetOptions"
              @update:auto-selected="updateAutoDetect"
              @update:model-value="updateTargets"
            />
            <CommandSnippet
              :label="t('registryUi.card.add')"
              kind="add"
              :command="command"
              compact
            />
          </div>
        </div>
      </Transition>
    </div>
  </article>
</template>
