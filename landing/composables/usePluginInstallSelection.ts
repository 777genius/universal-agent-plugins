import { computed, ref, watch, type Ref } from 'vue';
import { clients } from '../data/clients.ts';
import { useInstallPreferencesStore } from '../stores/installPreferences.ts';
import { reachableTargetPolicies, resolveDistribution } from '../utils/registry.ts';
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
  const targetDistributions = computed(
    () =>
      new Map(
        clients.map((client) => [
          client.id,
          resolveDistribution(plugin(), [client.id], scope.value).distribution,
        ]),
      ),
  );
  const projectAvailable = computed(() =>
    availableClients.value.some((client) =>
      Boolean(resolveDistribution(plugin(), [client.id], 'project').distribution),
    ),
  );
  const resolution = computed(() => resolveDistribution(plugin(), targets.value, scope.value));
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
    if (scope.value === 'project' && value) return;
    choicePending = true;
    autoDetect.value = value;
    saveUnboundChoice();
  }
  function updateScope(value: InstallScope) {
    if (value === 'project' && !projectAvailable.value) return;
    choicePending = true;
    scope.value = value;
    if (value === 'project') autoDetect.value = false;
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
    resolution,
    saveChoice,
    updateTargets,
    updateAutoDetect,
    updateScope,
    toggleInstall,
  };
}
