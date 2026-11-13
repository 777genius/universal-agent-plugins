<!--
  Card composition adapted from plugin-kit-ai landing/components/plugins/PluginCard.vue (MIT).
  Content model and implementation are new for Universal Agent Plugins.
-->
<script setup lang="ts">
import { usePluginInstallSelection } from '~/composables/usePluginInstallSelection';
import {
  installTargets,
  canonicalInstallTarget,
  canonicalInstallTargets,
  cliInstallTargets,
} from '~/data/installTargets';
import type { RegistryDiagnostic, RegistryPlugin } from '~/types/registry';
import { githubSourceUrl, reachableTargetPolicies, vendorInstallSource } from '~/utils/registry';
import { authenticationIndicator } from '~/utils/authentication';
import { pluginCommands } from '~/utils/commands';
import { projectCatalogCommands, pinnedPiProjectCommands } from '~/utils/projectCatalogSource';
import { pluginBenefit } from '~/data/catalogCuration';
const { t, locale, n } = useI18n();
const localePath = useLocalePath();

const props = withDefaults(
  defineProps<{ plugin: RegistryPlugin; alternatives?: RegistryPlugin[] }>(),
  { alternatives: () => [] },
);
const { asset, pluginIcon, sourceUrl } = useSite();
const { current, expired, published } = useDirectoryStatus();
const isDiscovered = computed(() => props.plugin.trust_state === 'conformant_unreviewed');
const discoveryStale = useDiscoveryIsStale(() => props.plugin);
const selection = usePluginInstallSelection(() => props.plugin);
const {
  targets,
  autoDetect,
  scope,
  installExpanded,
  targetDistributions,
  pinnedPiSource,
  pinnedPiSelected,
  pinnedPiReleaseSource,
  pinnedPiReleaseComponents,
  projectAvailable,
  projectAutomatic,
  updateTargets,
  updateAutoDetect,
  updateScope,
  toggleInstall,
} = selection;
const deliverySummary = computed(() => {
  if (autoDetect.value) return '';
  const canonical = canonicalInstallTargets(targets.value);
  const delivery = selection.resolution.value.distribution?.targets
    .filter((target) => canonical.includes(target.client))
    .map((target) => target.delivery) ?? [];
  const managed = delivery.filter((mode) => mode === 'managed').length;
  const prepared = delivery.filter(
    (mode) => mode === 'prepared' || mode === 'manual_activation',
  ).length;
  if (managed + prepared !== canonical.length || !delivery.length) return '';
  return t('registryUi.card.deliverySummary', { managed: n(managed), prepared: n(prepared) });
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
const resolution = selection.resolution;
const selectedDistribution = computed(() =>
  (isDiscovered.value || current.value) ? resolution.value.distribution : undefined,
);
const canChooseInstall = computed(
  () =>
    props.plugin.installable &&
    props.plugin.discovery?.availability !== 'unavailable' &&
    (isDiscovered.value || current.value),
);
const canInstall = computed(
  () =>
    (scope.value !== 'project' || !autoDetect.value || Boolean(projectAutomatic.value?.resolution.distribution)) &&
    ((pinnedPiSelected.value && current.value && props.plugin.installable) ||
      (isDiscovered.value
        ? props.plugin.installable &&
          props.plugin.discovery?.availability !== 'unavailable' &&
          (autoDetect.value || Boolean(selectedDistribution.value))
        : current.value &&
          (autoDetect.value ? props.plugin.installable : Boolean(selectedDistribution.value)))),
);
const command = computed(() =>
  canInstall.value
    ? (pinnedPiSelected.value
        ? pinnedPiProjectCommands(props.plugin)
        : scope.value === 'project'
          ? projectCatalogCommands(props.plugin, autoDetect.value ? undefined : canonicalInstallTargets(targets.value))
          : pluginCommands(props.plugin, autoDetect.value ? undefined : cliInstallTargets(targets.value))
      ).add
    : '',
);
const showDesktopGuide = computed(() =>
  scope.value === 'user' &&
  !autoDetect.value &&
  targets.value.includes('chatgpt') &&
  canChooseInstall.value &&
  Boolean(targetDistributions.value.get('chatgpt')),
);
const vendorInstall = computed(() => vendorInstallSource(props.plugin));
const vendorAvailable = computed(() => Boolean(vendorInstall.value && current.value));
const iconURL = computed(() => pluginIcon(props.plugin));
const autoOption = computed(() => ({
  label: t('registryUi.card.allInstalledAgentsRecommended'),
  summary: t('registryUi.card.allInstalledAgents'),
  description: t('registryUi.card.detectedWhenYouRunTheCommand'),
}));
const targetOptions = computed(() =>
  installTargets.map((client) => ({
    value: client.id,
    label: client.name,
    icon: asset(`client-icons/${client.icon}`),
    // Keep invalid explicitly saved selections visible for recovery, but an
    // auto-detected project must never make a stale Cursor placeholder selectable.
    disabled: !targetDistributions.value.get(client.id) &&
      !(client.id === 'pi' && pinnedPiSource.value && current.value) &&
      (autoDetect.value || !targets.value.includes(client.id)),
    description: (() => {
      if (isDiscovered.value)
        return props.plugin.discovery?.availability === 'available'
          ? t('registryUi.card.checkedAgainBeforeInstallation')
          : t('registryUi.card.unavailableAtItsIndexedSource');
      if (!published.value)
        return t('registryUi.card.unavailableReviewDataIsNotInstallationAuthority');
      if (expired.value) return t('registryUi.card.unavailableSignedDirectorySnapshotExpired');
      const source = targetDistributions.value.get(client.id);
      const target = source?.targets.find(
        (item) => item.client === canonicalInstallTarget(client.id),
      );
      if (client.id === 'chatgpt')
        return target
          ? t('registryUi.desktopGuide.description')
          : t('registryUi.card.notInstallableFromAnActiveRelease');
      if (client.id === 'chatgptweb')
        return target?.app_binding
          ? t('registryUi.card.verifiedConnectionFinishSetupInChatgpt')
          : t('registryUi.card.notAvailableForChatgpt');
      if (client.id === 'pi' && !target)
        return pinnedPiSource.value && current.value
          ? t('registryUi.install.piPinnedProjectSource')
          : t('registryUi.install.noCompatibleSignedRelease', { agent: client.name });
      return target
        ? t(`registryUi.delivery.${target.delivery}`)
        : t('registryUi.install.noCompatibleSignedRelease', { agent: client.name });
    })(),
  })),
);
const authentication = computed(() =>
  authenticationIndicator(
    autoDetect.value
      ? {
          id: props.plugin.name,
          targets: reachableTargetPolicies(props.plugin.distributions, scope.value),
        }
      : selectedDistribution.value,
    canonicalInstallTargets(targets.value),
    autoDetect.value,
    t,
  ),
);
const showAuthentication = computed(
  () => !autoDetect.value && authentication.value.state !== 'not_required',
);
const repositoryStars = computed(() =>
  new Intl.NumberFormat(locale.value, {
    notation: 'compact',
    maximumFractionDigits: 1,
  }).format(props.plugin.discovery?.stars ?? 0),
);
const assessmentSource = computed(() => pinnedPiReleaseSource.value ?? props.plugin.source);
const assessedRevisionMatches = computed(() => {
  if (!pinnedPiSelected.value) return true;
  const selected = pinnedPiReleaseSource.value;
  const assessed = props.plugin.source;
  return Boolean(selected &&
    selected.repository === assessed.repository &&
    selected.revision === assessed.revision &&
    selected.path === assessed.path &&
    selected.tree_sha256 === assessed.tree_sha256);
});
const provenanceURL = computed(() => {
  const source = pinnedPiReleaseSource.value ?? selectedDistribution.value?.source ?? props.plugin.source;
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
</script>

<template>
  <CatalogCard
    :expanded="installExpanded"
    :data-trust="isDiscovered ? 'community' : 'reviewed'"
    :data-install-source="plugin.install_source"
  >
    <AppTooltip v-if="!isDiscovered" above-header>
      <template #trigger>
        <span
          class="plugin-card__ribbon"
          tabindex="0"
          :class="{ 'plugin-card__ribbon--muted': !canInstall && !vendorAvailable }"
          >{{
            canInstall || vendorAvailable
              ? t('registryUi.card.reviewedListing')
              : expired
                ? t('registryUi.card.temporarilyPaused')
                : !published
                  ? t('registryUi.card.previewOnly')
                  : t('registryUi.card.notAvailable')
          }}</span
        >
      </template>
      <p class="app-tooltip__compact">{{ t('registryUi.card.reviewedListingTooltip') }}</p>
    </AppTooltip>
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
        <p v-else-if="canInstall && deliverySummary" class="plugin-card__source-label">
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
    <div class="plugin-card__assessments">
      <SecurityAssessmentBadge
        v-if="plugin.security && !discoveryStale && assessedRevisionMatches"
        :plugin="plugin"
        :details-to="securityDetailURL"
      />
      <SkillSpectorAssessment
        :applicable="(pinnedPiReleaseComponents ?? plugin.components).includes('skills')"
        :subject="{
          kind: 'plugin',
          repository: assessmentSource.repository,
          path: assessmentSource.path,
          revision: assessmentSource.revision,
        }"
        :name="plugin.display_name"
        scope="plugin"
      />
    </div>
    <p class="plugin-card__description">{{ pluginBenefit(plugin) }}</p>
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
      <p v-if="!canInstall && vendorInstall" class="plugin-card__unavailable">
        {{ t('registryUi.install.vendorInstallerHeading') }}
      </p>
      <div class="plugin-card__actions">
        <button
          v-if="canChooseInstall"
          type="button"
          class="plugin-card__install-toggle"
          :aria-controls="installPanelId"
          :aria-expanded="installExpanded"
          :aria-label="t('registryUi.card.installName', { name: plugin.display_name })"
          @click="toggleInstall"
        >
          {{ t('registryUi.card.install') }}
        </button>
        <a
          v-else-if="vendorInstall"
          class="plugin-card__vendor-install"
          :href="vendorInstall.url"
          target="_blank"
          rel="noreferrer"
          >{{ t('registryUi.install.vendorInstallerCta') }} ↗</a
        >
        <span v-else class="plugin-card__unavailable">{{
          isDiscovered
            ? t('registryUi.card.unavailableAtItsIndexedSourceNoInstallCommandIsGenerated')
            : expired
              ? t('registryUi.card.commandsDisabledBecauseTheDirectoryIsTemporarilyStale')
              : !published
                ? t('registryUi.card.commandsDisabledInPreview')
                : diagnosticText(resolution.unavailable_diagnostic, resolution.unavailable_reason)
        }}</span>
        <AuthenticationIndicator :indicator="authentication" />
      </div>
      <Transition name="plugin-install">
        <div
          v-if="installExpanded && canChooseInstall"
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
          <p v-if="showAuthentication" class="plugin-card__auth">{{ authentication.label }}</p>
          <InstallScopeSelect
            :model-value="scope"
            :project-available="projectAvailable"
            @update:model-value="updateScope"
          />
          <InstallOutcome
            v-if="!autoDetect && selectedDistribution"
            :distribution="selectedDistribution"
            :targets="targets"
            :scope="scope"
          />
          <div class="plugin-card__install">
            <AppMultiSelect
              v-if="targets.length"
              :model-value="targets"
              :auto-selected="autoDetect"
              :auto-option="autoOption"
              :label="t('registryUi.card.chooseClients', { name: plugin.display_name })"
              :options="targetOptions"
              full-width
              @update:auto-selected="updateAutoDetect"
              @update:model-value="updateTargets"
            />
            <p v-if="pinnedPiSelected && canInstall" class="plugin-card__author" role="status">
              {{ t('registryUi.install.piPinnedProjectSource') }}
              <a :href="provenanceURL" target="_blank" rel="noreferrer">
                {{ t('registryUi.card.viewSource') }}
              </a>
            </p>
            <CommandSnippet
              v-if="canInstall"
              :label="t('registryUi.card.add')"
              kind="add"
              :command="command"
              compact
            />
            <p v-else class="plugin-card__unavailable" role="status">
              {{ t('registryUi.install.scope.selectionUnavailable') }}
            </p>
          </div>
          <ChatGPTDesktopGuide v-if="showDesktopGuide" />
        </div>
      </Transition>
    </div>
  </CatalogCard>
</template>

<style scoped lang="scss">
.plugin-card__assessments {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 4px;
  min-width: 0;
  font-size: 0.75rem;
}
.plugin-card__assessments :deep(.plugin-card__security) {
  min-height: 32px;
  margin: 0;
  gap: 8px;
  font-size: inherit;
  line-height: 1.5;
}
.plugin-card__assessments :deep(.skillspector) {
  margin: 0;
  font-size: inherit;
}
.plugin-card__actions {
  display: flex;
  align-items: center;
  gap: 8px;
}
.plugin-card__actions > :is(.plugin-card__install-toggle, .plugin-card__vendor-install) {
  flex: 1;
  width: auto;
}
</style>
