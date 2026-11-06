import { isReviewedMarketingKit } from '../data/marketingKitDemo.ts';
import { computed, ref, watch, type Ref } from 'vue';
import { clients } from '../data/clients.ts';
import { useInstallPreferencesStore } from '../stores/installPreferences.ts';
import { reachableTargetPolicies, resolveDistribution } from '../utils/registry.ts';
import { projectCatalogAutomaticSelection, projectCatalogSelection } from '../utils/projectCatalogSource.ts';
import type { ClientID, InstallScope, RegistryPlugin } from '../types/registry';

/** Card and detail share one selection/persistence policy; no machine detection occurs here. */
export function usePluginInstallSelection(
  plugin: () => RegistryPlugin,
  models: { targets?: Ref<ClientID[]>; autoDetect?: Ref<boolean> } = {},
) {
  const preferences = useInstallPreferencesStore();
  const targets = models.targets ?? ref<ClientID[]>([]);
  const autoDetect = models.autoDetect ?? ref(true);
  const installExpanded = ref(false);
  const scope = ref<InstallScope>('user');
  const availableClients = computed(() => {
    const policies = reachableTargetPolicies(plugin().distributions);
    return clients.filter((client) => policies.some((policy) => policy.client === client.id));
  });
  const select = (selected: ClientID[]) => scope.value === 'project'
    ? projectCatalogSelection(plugin(), selected).resolution
    : resolveDistribution(plugin(), selected, 'user');
  const targetDistributions = computed(
    () =>
      new Map(
        clients.map((client) => [client.id, select([client.id]).distribution]),
      ),
  );
  // One authorized project source is enough to offer CLI-side target discovery.
  // This must not broaden any signed release scope or assume local client state.
  const projectAutomatic = computed(() => projectCatalogAutomaticSelection(plugin()));
  const projectAvailable = computed(() => Boolean(projectAutomatic.value?.resolution.distribution));
  // The published CLI still needs the exact Git fallback for user-only signed
  // entries. The source-built CLI's short-name Project path is not released.
  const projectSourceFallback = computed(() => scope.value === 'project' && Boolean(
    (autoDetect.value ? projectAutomatic.value : projectCatalogSelection(plugin(), targets.value))
      ?.directSource,
  ));
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
      targets.value = saved.targetIds.length
        ? (saved.targetIds as ClientID[])
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
    if (scope.value === 'project') return; // Invalid project choices stay visible; never become user installs.
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
      (value): value is ClientID =>
        clients.some((client) => client.id === value) &&
        (Boolean(targetDistributions.value.get(value as ClientID)) ||
          targets.value.includes(value as ClientID)),
    );
    if (!next.length) return;
    choicePending = true;
    targets.value = [...new Set(next)];
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
    choicePending = true;
    scope.value = value;
    // Entering project starts with CLI-side detection, even if the user
    // manually selected Cursor in their *user* environment. An explicit
    // project choice remains persistent across detail/navigation remounts.
    if (previousScope !== 'project' && value === 'project') autoDetect.value = true;
    // Previously saved project intent may have pinned a single target (Cursor).
    // When the user explicitly switches a user-only Marketing Kit back to
    // This computer, return to CLI-side installed-agent detection.
    if (
      previousScope === 'project' &&
      value === 'user' &&
      isReviewedMarketingKit(plugin())
    ) autoDetect.value = true;
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
    projectAvailable,
    projectAutomatic,
    projectSourceFallback,
    resolution,
    saveChoice,
    updateTargets,
    updateAutoDetect,
    updateScope,
    toggleInstall,
  };
}
