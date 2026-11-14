import { canonicalInstallTargets, type InstallTargetID } from '../data/installTargets.ts';
import type { ClientID, DistributionResolution, InstallScope, RegistryPlugin } from '../types/registry.ts';
import { expectedDistribution, resolveDistribution } from './registry.ts';
import { pinnedCatalogSelection, projectCatalogSelection } from './projectCatalogSource.ts';

export type PluginInstallAvailability =
  | { kind: 'signed'; resolution: DistributionResolution }
  | { kind: 'guided_web'; resolution: DistributionResolution }
  | { kind: 'pinned_git'; selection: NonNullable<ReturnType<typeof pinnedCatalogSelection>> };

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
  // Public ChatGPT aliases keep their own routes; only a canonical local
  // agent can explicitly request an independent saved Git source.
  if (targets.length === 1 && canonical[0] === targets[0]) {
    const selection = pinnedCatalogSelection(plugin, canonical[0]!, scope);
    if (selection) return { kind: 'pinned_git', selection };
  }
  return undefined;
}

/** A source request is exclusive; selecting a signed peer returns to that route. */
export function reconcilePluginSourceTargets(
  plugin: RegistryPlugin, next: InstallTargetID[], previous: readonly InstallTargetID[],
  scope: InstallScope = 'user',
): InstallTargetID[] {
  if (next.length < 2) return next;
  const sources = next.filter(target => pluginInstallAvailability(plugin, [target], scope)?.kind === 'pinned_git');
  if (!sources.length) return next;
  const addedSource = sources.find(target => !previous.includes(target));
  if (addedSource) return [addedSource];
  const signed = next.filter(target => !sources.includes(target));
  return signed.length ? signed : [sources[0]!];
}

export function hasPluginInstallTarget(plugin: RegistryPlugin, target: InstallTargetID): boolean {
  return Boolean(pluginInstallAvailability(plugin, [target], 'user') ||
    pluginInstallAvailability(plugin, [target], 'project'));
}
