import { computed, ref, watch, type Ref } from 'vue';
import {
  installTargets,
  reconcileLocalChatGPTTargets,
  canonicalInstallTargets,
  type InstallTargetID,
} from '../data/installTargets.ts';
import { pluginInstallAvailability, hasPluginInstallTarget, reconcilePluginSourceTargets } from '../utils/pluginInstallAvailability.ts';
import { useInstallPreferencesStore } from '../stores/installPreferences.ts';
import { resolveDistribution } from '../utils/registry.ts';
import { pinnedCatalogCommands, pinnedCatalogRepairLimitation, projectCatalogAutomaticSelection, projectCatalogSelection } from '../utils/projectCatalogSource.ts';
import type { DistributionResolution, InstallScope, RegistryPlugin } from '../types/registry';

/** Card and detail share one selection/persistence policy; no machine detection occurs here. */
export function usePluginInstallSelection(
  plugin: () => RegistryPlugin,
  models: { targets?: Ref<InstallTargetID[]>; autoDetect?: Ref<boolean> } = {},
) {
  const preferences = useInstallPreferencesStore();
  const targets = models.targets ?? ref<InstallTargetID[]>([]);
  const autoDetect = models.autoDetect ?? ref(true);
  const installExpanded = ref(false);
  const scope = ref<InstallScope>('user');
  const availableClients = computed(() =>
    installTargets.filter(client => hasPluginInstallTarget(plugin(), client.id)),
  );
  const select = (selected: InstallTargetID[]): DistributionResolution => {
    // A local Desktop plugin belongs to the Codex user profile, not a project.
    if (scope.value === 'project' && selected.includes('chatgpt'))
      return { unavailable_reason: 'ChatGPT Desktop supports only This computer; select Codex for a project install.' };
    const canonical = canonicalInstallTargets(selected);
    const resolved = scope.value === 'project'
      ? projectCatalogSelection(plugin(), canonical).resolution
      : resolveDistribution(plugin(), canonical, 'user');
    // Unreviewed community metadata is not an authority for an account-bound
    // ChatGPT Web app, even when it advertises an app_binding object.
    if (selected.includes('chatgptweb') && plugin().trust_state === 'conformant_unreviewed') {
      return { unavailable_reason: 'ChatGPT Web needs a reviewed signed Directory release.' };
    }
    // A signed ChatGPT Web target is meaningful only with its registered app
    // identity. Older/preview projections must not enable a cloud install.
    if (selected.includes('chatgptweb') &&
        !resolved.distribution?.targets.some((target) =>
          target.client === 'chatgpt' && Boolean(target.app_binding))) {
      return {
        unavailable_reason: 'ChatGPT Web needs a verified app binding in this release.',
      };
    }
    return resolved;
  };
  const targetDistributions = computed(
    () =>
      new Map(
        installTargets.map((client) => [client.id, select([client.id]).distribution]),
      ),
  );
  const targetAvailability = computed(() => new Map(installTargets.map(client =>
    [client.id, pluginInstallAvailability(plugin(), [client.id], scope.value)])));
  const availability = computed(() => pluginInstallAvailability(plugin(), targets.value, scope.value));
  const guidedWebSelected = computed(() => !autoDetect.value && availability.value?.kind === 'guided_web');
  const pinnedSelection = computed(() => !autoDetect.value && availability.value?.kind === 'pinned_git'
    ? availability.value.selection : undefined);
  const pinnedSourceSelected = computed(() => Boolean(pinnedSelection.value));
  const pinnedReleaseSource = computed(() => pinnedSelection.value?.source);
  const pinnedReleaseComponents = computed(() => pinnedSelection.value?.components);
  const pinnedSourceCommands = computed(() => pinnedSelection.value
    ? pinnedCatalogCommands(plugin(), pinnedSelection.value.client, scope.value) : undefined);
  const pinnedRepairLimitation = computed(() => pinnedCatalogRepairLimitation(pinnedSelection.value));
  // One authorized project source is enough to offer CLI-side target discovery.
  // This must not broaden any signed release scope or assume local client state.
  const projectAutomatic = computed(() => projectCatalogAutomaticSelection(plugin()));
  const projectAvailable = computed(() =>
    Boolean(projectAutomatic.value?.resolution.distribution || installTargets.some(client =>
      pluginInstallAvailability(plugin(), [client.id], 'project')?.kind === 'pinned_git')),
  );
  const resolution = computed(() => select(targets.value));
  let choicePending = false;
  function saveChoice() {
    preferences.selectPackage(
      plugin().install_source,
      {
        targetIds: targets.value,
        autoDetect: autoDetect.value,
        expanded: installExpanded.value,
        scope: scope.value,
      },
      availableClients.value.map((client) => client.id),
    );
  }
  function saveUnboundChoice() {
    if (models.targets || models.autoDetect) return;
    choicePending = false;
    saveChoice();
  }
  watch(
    () => plugin().install_source,
    (identity) => {
      choicePending = false;
      const allowed = availableClients.value.map((client) => client.id);
      if (preferences.package.identity === identity)
        preferences.reconcilePackage(identity, allowed);
      const saved = preferences.readPackage(identity, allowed);
      const initial =
        availableClients.value.find((client) => client.id === 'cursor') ??
        availableClients.value[0];
      // Persisted preferences may predate the public ChatGPT target rename.
      // Never restore both public Codex and Desktop as two local installs.
      targets.value = saved.targetIds.length
        ? reconcileLocalChatGPTTargets(saved.targetIds as InstallTargetID[], [])
        : initial
          ? [initial.id]
          : [];
      // Preserve explicit scope intent; a now-unsupported project never
      // silently falls back to modifying the user's computer.
      scope.value = saved.scope;
      autoDetect.value = saved.autoDetect;
      installExpanded.value = saved.expanded;
    },
    { immediate: true, flush: 'sync' },
  );
  // A detail v-model emits to its parent before the updated props come back.
  // Persist after that update, including the picker's auto→manual event order.
  watch(
    [targets, autoDetect, scope, installExpanded],
    () => {
      if (!choicePending) return;
      choicePending = false;
      saveChoice();
    },
    { flush: 'post' },
  );
  watch(availableClients, (next) => {
    // A revoked explicit source must not silently become automatic installation
    // for other agents. Invalid manual choices stay visible for recovery.
    if (scope.value === 'project' || !autoDetect.value) return;
    const allowed = next.map((client) => client.id);
    if (targets.value.some((target) => !allowed.includes(target))) {
      targets.value = next[0] ? [next[0].id] : [];
      autoDetect.value = true;
      if (preferences.package.identity === plugin().install_source)
        preferences.reconcilePackage(plugin().install_source, allowed);
    }
  });
  function updateTargets(values: string[]) {
    const next = values.filter(
      (value): value is InstallTargetID =>
        installTargets.some((client) => client.id === value) &&
        (Boolean(targetAvailability.value.get(value as InstallTargetID)) ||
          targets.value.includes(value as InstallTargetID)),
    );
    if (!next.length) return;
    choicePending = true;
    const selected = reconcilePluginSourceTargets(plugin(), [...new Set(next)], targets.value, scope.value);
    targets.value = reconcileLocalChatGPTTargets(selected, targets.value);
    saveUnboundChoice();
  }
  function updateAutoDetect(value: boolean) {
    choicePending = true;
    autoDetect.value = value;
    saveUnboundChoice();
  }
  function updateScope(value: InstallScope) {
    if (value === 'project' && !projectAvailable.value) return;
    const previousScope = scope.value;
    const wasPinnedSource = pinnedSourceSelected.value;
    choicePending = true;
    scope.value = value;
    // Moving between environments requires a fresh explicit source choice;
    // an independent Git source never becomes a signed automatic target.
    if (previousScope === 'project' && value === 'user' && wasPinnedSource &&
        targets.value.length === 1 &&
        !resolveDistribution(plugin(), canonicalInstallTargets(targets.value), 'user').distribution) autoDetect.value = true;
    // Entering project starts with CLI-side detection, even if the user
    // manually selected Cursor in their *user* environment. An explicit
    // project choice remains persistent across detail/navigation remounts.
    if (previousScope !== 'project' && value === 'project') autoDetect.value = true;
    saveUnboundChoice();
  }
  function toggleInstall() {
    choicePending = true;
    installExpanded.value = !installExpanded.value;
    saveUnboundChoice();
  }
  return {
    targets,
    autoDetect,
    scope,
    installExpanded,
    availableClients,
    targetDistributions,
    targetAvailability,
    availability,
    guidedWebSelected,
    pinnedSourceSelected,
    pinnedReleaseSource,
    pinnedReleaseComponents,
    pinnedSourceCommands,
    pinnedRepairLimitation,
    projectAvailable,
    projectAutomatic,
    resolution,
    saveChoice,
    updateTargets,
    updateAutoDetect,
    updateScope,
    toggleInstall,
  };
}
