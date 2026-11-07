import { pluginCommands } from './commands.ts';
import capabilities from '../data/clientCapabilities.generated.json' with { type: 'json' };
import { reachableTargetPolicies, resolveDistribution } from './registry.ts';
import type { ClientID, ComponentID, DistributionResolution, RegistryPlugin } from '../types/registry.ts';

// Published signed Directory v1 authenticates source identity. Released CLI
// 0.1.101 separately checks explicit Project placement for every component
// and mandatory dependency; User native evidence is NOT Project activation.
// One generated adapter-derived Project matrix. This is potential component
// delivery, NOT exact MCP-transport approval or runtime/activation evidence.
// The released CLI checks actual package bytes, dependencies and native policy.
const projectCapabilities = new Map(capabilities.map((client) =>
  [client.id, 'project' in client ? client.project : undefined] as const,
));

// Broad capability preview only: the signed release does not expose which MCP
// transports a package needs (including SSE-only). The CLI decides exact fit.
export function projectComponentSupported(client: ClientID, component: ComponentID): boolean {
  const scoped = projectCapabilities.get(client);
  if (!scoped) return false;
  if (component === 'skills') return scoped.skill_support !== 'unsupported';
  if (component === 'extensions') return scoped.extension_support !== 'unsupported';
  return scoped.mcp_transports.stdio !== 'unsupported' ||
    scoped.mcp_transports['streamable-http'] !== 'unsupported';
}

const GIT_REPOSITORY = /^[A-Za-z0-9][A-Za-z0-9-]*\/[A-Za-z0-9][A-Za-z0-9._-]*$/;
const FULL_REVISION = /^[0-9a-f]{40}$/;

function projectCompatible(
  plugin: RegistryPlugin, targets: readonly ClientID[],
  resolved: DistributionResolution, hasExplicitProjectGrant: boolean,
) {
  const dist = resolved.distribution;
  if (!plugin.installable || plugin.trust_state === 'conformant_unreviewed' ||
      (!hasExplicitProjectGrant && plugin.trust_state !== 'reviewed') || !targets.length ||
      !dist || !dist.components.length) return false;
  // Reject malformed metadata in test projections too; signed feed validation
  // and exact source verification remain authoritative in the released CLI.
  const { repository, revision, path } = dist.source;
  if (!GIT_REPOSITORY.test(repository) || (revision !== null && !FULL_REVISION.test(revision)) ||
      (path !== '' && path !== '.' && (!path || path.split('/').some(
        (part) => !/^[A-Za-z0-9._-]+$/.test(part) || part === '.' || part === '..',
      )))) return false;
  return targets.every((client) => {
    const target = dist.targets.find((candidate) => candidate.client === client);
    return Boolean(target && !target.app_binding &&
      dist.components.some((component) => projectComponentSupported(client, component)));
  });
}

export function projectCatalogSelection(
  plugin: RegistryPlugin,
  targets: readonly ClientID[],
): { resolution: DistributionResolution } {
  const signedProject = resolveDistribution(plugin, targets, 'project');
  // Honor explicit project-scoped policy if present. Signed v1 User grants
  // also authenticate sources for the owner's explicit Project request.
  // A signed-v2 scoped closure must never be downgraded into v1's
  // user-grant-as-source behavior. Its explicit Project policy is authoritative.
  const hasCanonicalClosure = plugin.distributions.some((distribution) =>
    distribution.releases.some((release) =>
      release.targets.some((target) =>
        targets.includes(target.client) && target.canonical_closure !== undefined)));
  const selected = signedProject.distribution || hasCanonicalClosure
    ? signedProject
    : resolveDistribution(plugin, targets, 'user');
  if (!projectCompatible(plugin, targets, selected, Boolean(signedProject.distribution))) {
    return { resolution: { ...signedProject, distribution: undefined } };
  }
  return {
    resolution: {
      ...selected,
      distribution: {
        ...selected.distribution!,
        targets: selected.distribution!.targets.map((target) => ({
          ...target, delivery: 'prepared' as const,
        })),
      },
    },
  };
}

export function projectCatalogAutomaticSelection(plugin: RegistryPlugin) {
  const clients = [...new Set(reachableTargetPolicies(plugin.distributions).map((target) => target.client))];
  for (const client of clients) {
    const selection = projectCatalogSelection(plugin, [client]);
    if (selection.resolution.distribution) return selection;
  }
  return undefined;
}

export function projectCatalogCommands(plugin: RegistryPlugin, targets?: readonly ClientID[]) {
  const choice = targets === undefined
    ? projectCatalogAutomaticSelection(plugin)
    : projectCatalogSelection(plugin, targets);
  if (!choice?.resolution.distribution) throw new Error('No eligible project installation source');
  return pluginCommands(plugin, targets, undefined, 'project');
}
