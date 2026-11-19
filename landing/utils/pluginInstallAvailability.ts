import { canonicalInstallTargets, type InstallTargetID } from '../data/installTargets.ts';
import type { ClientID, DistributionResolution, InstallScope, RegistryPlugin } from '../types/registry.ts';
import { expectedDistribution, resolveDistribution } from './registry.ts';
import { pinnedCatalogGroupSelection, pinnedCatalogSelection, projectCatalogSelection } from './projectCatalogSource.ts';

export type PluginInstallAvailability =
  | { kind: 'signed'; resolution: DistributionResolution }
  | { kind: 'guided_web'; resolution: DistributionResolution }
  | { kind: 'pinned_git'; selection: NonNullable<ReturnType<typeof pinnedCatalogSelection>> & { clients?: readonly ClientID[] } };

function context7Preparation(plugin: RegistryPlugin, peers: ClientID[] = []) {
  if (
    plugin.trust_state === 'conformant_unreviewed' ||
    plugin.name !== 'context7' ||
    plugin.install_source !== 'context7' ||
    plugin.client_support.resolution !== 'directory'
  )
    return false;
  const upstream = plugin.distributions.find(
    (item) => item.id === 'upstash/context7' && item.kind === 'upstream',
  );
  if (!upstream) return false;
  const candidate = { ...plugin, distributions: [upstream], default_distribution: upstream.id, declared_default_distribution: upstream.id };
  const promoted = upstream.compatible_clients
    .map((client) => expectedDistribution(candidate, [client]))
    .filter(
      (release) =>
        release?.source.repository === 'upstash/context7' &&
        release.source.path === 'plugins/agent-plugins/context7',
    )
    .sort((a, b) => (b!.release_sequence ?? 0) - (a!.release_sequence ?? 0))[0];
  if (!promoted) return false;
  if (!peers.length) return true;
  candidate.distributions = [
    {
      ...upstream,
      releases: upstream.releases.filter(
        (release) => release.release_sequence === promoted.release_sequence,
      ),
    },
  ];
  return Boolean(expectedDistribution(candidate, peers));
}

/** Presentation of existing install routes; never synthesizes a signed target grant. */
export function pluginInstallAvailability(
  plugin: RegistryPlugin, targets: readonly InstallTargetID[], scope: InstallScope = 'user',
): PluginInstallAvailability | undefined {
  if (!plugin.installable || plugin.discovery?.availability === 'unavailable' || !targets.length)
    return undefined;
  if (scope === 'project' && targets.includes('chatgpt')) return undefined;
  const canonical = canonicalInstallTargets(targets);
  const resolution = scope === 'project'
    ? projectCatalogSelection(plugin, canonical).resolution
    : resolveDistribution(plugin, canonical, 'user');
  const web = targets.includes('chatgptweb');
  if (resolution.distribution && (!web ||
      (plugin.trust_state !== 'conformant_unreviewed' &&
       resolution.distribution.targets.some(target => target.client === 'chatgpt' && target.app_binding))))
    return { kind: 'signed', resolution };
  if (scope === 'user' && web && context7Preparation(plugin,
      canonicalInstallTargets(targets.filter(target => target !== 'chatgptweb'))))
    return { kind: 'guided_web', resolution };
  // A shared explicit source remains a Git request, never a new signed grant.
  // Qualify each visible surface first: Desktop cannot borrow a canonical
  // Codex Git fallback when its own signed route is unavailable.
  if (!web && canonical.length > 1 &&
      targets.every(target => pluginInstallAvailability(plugin, [target], scope))) {
    const selection = pinnedCatalogGroupSelection(plugin, canonical, scope);
    if (selection) return { kind: 'pinned_git', selection };
  }
  // A Desktop-only surface keeps its existing signed Codex route.
  if (targets.length === 1 && canonical[0] === targets[0]) {
    const selection = pinnedCatalogSelection(plugin, canonical[0]!, scope);
    if (selection) return { kind: 'pinned_git', selection };
  }
  return undefined;
}

/** Picker reconciliation preserves intent; route compatibility is checked separately. */
export function reconcilePluginSourceTargets(
  _plugin: RegistryPlugin, next: InstallTargetID[], _previous: readonly InstallTargetID[],
  _scope: InstallScope = 'user',
): InstallTargetID[] {
  return [...new Set(next)];
}

export function hasPluginInstallTarget(plugin: RegistryPlugin, target: InstallTargetID): boolean {
  return Boolean(pluginInstallAvailability(plugin, [target], 'user') ||
    pluginInstallAvailability(plugin, [target], 'project'));
}
