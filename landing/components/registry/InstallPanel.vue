<script setup lang="ts">
import { useInstallPreferencesStore } from '~/stores/installPreferences';
import type { RegistryDiagnostic, ClientID, RegistryPlugin } from '~/types/registry';
import { pluginCommands } from '~/utils/commands';
import { expectedDistribution, resolveDistribution } from '~/utils/registry';
const { t } = useI18n();

const props = defineProps<{ plugin: RegistryPlugin }>();
const targets = defineModel<ClientID[]>('targets', { required: true });
const autoDetect = defineModel<boolean>('autoDetect', { required: true });
const { asset, sourceUrl } = useSite();
const { current, expired, published } = useDirectoryStatus();
const autoOption = computed(() => ({
  label: t('registryUi.install.allInstalledAgentsRecommended'),
  summary: t('registryUi.install.allInstalledAgents'),
  description: t('registryUi.install.detectedWhenYouRunTheCommand'),
}));
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
      if (!published.value)
        return t('registryUi.install.unavailableReviewDataIsNotInstallationAuthority');
      if (expired.value) return t('registryUi.install.unavailableSignedDirectorySnapshotExpired');
      const source = expectedDistribution(props.plugin, [client.id]);
      const target = source?.targets.find((item) => item.client === client.id);
      if (client.id === 'chatgpt')
        return target?.app_binding
          ? t('registryUi.install.verifiedConnectionFinishSetupInChatgpt')
          : t('registryUi.install.notAvailableForChatgpt');
      if (target) return t(`registryUi.delivery.${target.delivery}`);
      return t('registryUi.install.noActiveReleaseSupportsThisClient');
    })(),
  })),
);
const preferences = useInstallPreferencesStore();
watch(
  () => props.plugin.install_source,
  (identity, previous) => {
    if (preferences.package.identity)
      preferences.reconcilePackage(
        identity,
        availableClients.value.map((client) => client.id),
      );
    const saved = preferences.readPackage(
      identity,
      availableClients.value.map((client) => client.id),
    );
    if (saved.targetIds.length) targets.value = saved.targetIds as ClientID[];
    else if (previous && previous !== identity) {
      const initial =
        availableClients.value.find((client) => client.id === 'cursor') ??
        availableClients.value[0];
      targets.value = initial ? [initial.id] : [];
    }
    autoDetect.value = saved.autoDetect;
  },
  { immediate: true, flush: 'sync' },
);
const choicePending = ref(false);
watch([targets, autoDetect], () => {
  if (!choicePending.value) return;
  choicePending.value = false;
  saveChoice();
}, { flush: 'post' });
function saveChoice() {
  preferences.selectPackage(
    props.plugin.install_source,
    {
      targetIds: targets.value,
      autoDetect: autoDetect.value,
      expanded: true,
    },
    availableClients.value.map((client) => client.id),
  );
}
const commands = computed(() =>
  props.plugin.installable &&
  current.value &&
  (autoDetect.value || (targets.value.length > 0 && hasCompleteSource.value))
    ? pluginCommands(props.plugin, autoDetect.value ? undefined : targets.value)
    : undefined,
);
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
const expectedSource = computed(() => (current.value ? resolution.value.distribution : undefined));
const expectedSourceLabel = computed(() =>
  expectedSource.value ? t(`registryUi.distribution.${expectedSource.value.kind}`) : '',
);
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
    return t('registryUi.install.thisPackageIsNoLongerAvailableFromItsSource');
  if (!props.plugin.components.length)
    return t(
      'registryUi.install.weFoundThisProjectButItDoesnTIncludeAnyToolsTheInstallerCanAddYet',
    );
  return t('registryUi.install.thisPackageDoesNotSupportAnyOfTheAgentsAvailableInTheInstallerYet');
});

function updateTargets(values: string[]) {
  const allowed = new Set(availableClients.value.map((client) => client.id));
  const next = values.filter((value): value is (typeof clients)[number]['id'] =>
    allowed.has(value as (typeof clients)[number]['id']),
  );
  if (next.length) {
    choicePending.value = true;
    targets.value = next;
  }
}

function updateAutoDetect(value: boolean) {
  choicePending.value = true;
  autoDetect.value = value;
}

watch(availableClients, (next) => {
  const allowed = new Set(next.map((client) => client.id));
  const retained = targets.value.filter((target) => allowed.has(target));
  if (retained.length !== targets.value.length) {
    targets.value = retained.length ? retained : next[0] ? [next[0].id] : [];
    autoDetect.value = true;
    if (preferences.package.identity)
      preferences.reconcilePackage(props.plugin.install_source, [...allowed]);
  }
});
</script>

<template>
  <aside class="install-panel" aria-labelledby="install-title">
    <div class="install-panel__heading">
      <div>
        <p class="eyebrow">{{ t('registryUi.install.installer') }}</p>
        <h2 id="install-title">
          {{
            unavailableDiscoveryReason
              ? t('registryUi.install.notReadyToInstall')
              : t('registryUi.install.useWithYourAgent')
          }}
        </h2>
      </div>
      <a
        v-if="!unavailableDiscoveryReason"
        class="install-panel__cli-link"
        href="https://github.com/777genius/universal-agent-plugins#quick-start"
        target="_blank"
        rel="noreferrer"
        >{{ t('registryUi.install.runWithNpx') }}</a
      >
    </div>
    <div v-if="unavailableDiscoveryReason" class="install-panel__empty" role="status">
      <span class="install-panel__empty-icon" aria-hidden="true">i</span>
      <div>
        <h3>{{ t('registryUi.install.thisPluginIsNotInstallableYet') }}</h3>
        <p>{{ unavailableDiscoveryReason }}</p>
        <a :href="sourceUrl(plugin)" target="_blank" rel="noreferrer">{{
          t('registryUi.install.viewPackageSource')
        }}</a>
      </div>
    </div>
    <div v-else-if="commands" class="command-stack">
      <div class="install-command-row">
        <div class="target-select">
          <span>{{ t('registryUi.install.agents') }}</span
          ><AppMultiSelect
            :model-value="targets"
            :auto-selected="autoDetect"
            :auto-option="autoOption"
            :label="t('registryUi.install.chooseTargetAgents')"
            :options="targetOptions"
            @update:auto-selected="updateAutoDetect"
            @update:model-value="updateTargets"
          />
        </div>
        <CommandSnippet :label="t('registryUi.install.add')" kind="add" :command="commands.add" />
      </div>
      <CommandSnippet
        :label="t('registryUi.install.update')"
        kind="update"
        :command="commands.update"
      />
      <CommandSnippet
        :label="t('registryUi.install.repair')"
        kind="repair"
        :command="commands.repair"
      />
      <CommandSnippet
        :label="t('registryUi.install.remove')"
        kind="remove"
        :command="commands.remove"
      />
    </div>
    <p v-if="!unavailableDiscoveryReason && expired" class="install-panel__notice" role="status">
      <strong>{{ t('registryUi.install.commandsUnavailableStaleDirectory') }}</strong>
      {{
        t(
          'registryUi.install.thisSignedSnapshotHasExpiredBrowseItsHistoryThenReturnAfterAFreshSnapshotIsPublished',
        )
      }}
    </p>
    <p
      v-else-if="!unavailableDiscoveryReason && !published"
      class="install-panel__notice"
      role="status"
    >
      <strong>{{ t('registryUi.install.commandsUnavailableInReviewPreview') }}</strong>
      {{
        t(
          'registryUi.install.unresolvedDataIsForReviewOnlyProductionCommandsRequireAPublishedSignedDirectorySnapshot',
        )
      }}
    </p>
    <p
      v-else-if="!unavailableDiscoveryReason && !autoDetect && !hasCompleteSource"
      class="install-panel__notice"
      role="status"
    >
      <strong>{{ t('registryUi.install.commandsUnavailable') }}</strong>
      {{ diagnosticText(resolution.unavailable_diagnostic, resolution.unavailable_reason) }}
    </p>
    <p v-if="autoDetect && commands" class="install-panel__notice">
      <strong>{{ t('registryUi.install.automaticDetection') }}</strong>
      {{
        t(
          'registryUi.install.theCliChecksThisPluginAgainstInstalledAgentsSkipsIncompatibleOnesAndLetsYouConfirmTheTargetsChatgptIsIncludedOnlyWhenThisPluginProvidesAVerifiedConnection',
        )
      }}
    </p>
    <p v-if="chatgptSelected" class="install-panel__notice">
      <strong>{{ t('registryUi.install.oneStepRemainsInChatgpt') }}</strong>
      {{ t('registryUi.install.chatgptFinish', { name: plugin.display_name }) }}
    </p>
    <p
      v-if="!unavailableDiscoveryReason && !autoDetect && resolution.fallback_reason && current"
      class="install-panel__notice"
    >
      <strong>{{
        t('registryUi.install.expectedFallback', {
          source: expectedSourceLabel,
        })
      }}</strong>
      {{ diagnosticText(resolution.fallback_diagnostic, resolution.fallback_reason) }}
    </p>
    <p
      v-else-if="!unavailableDiscoveryReason && !autoDetect && !hasCompleteSource && current"
      class="install-panel__notice"
    >
      <strong>{{ t('registryUi.install.noSingleSourceServesThisTargetSet') }}</strong>
      {{
        t(
          'registryUi.install.theCliWillFailBeforeMutationAndSuggestCompatibleTargetSourceCombinationsItNeverMixesDistributionsAcrossClients',
        )
      }}
    </p>
    <p v-if="commands" class="install-panel__footnote">
      {{
        t(
          'registryUi.install.theCliChecksEverySelectedAgentBeforeChangingFilesAndShowsAnySignInOrActivationStepOauthCredentialsStayWithTheAgent',
        )
      }}
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
