import { pluginCommands, installCommands } from './commands.ts';
import capabilities from '../data/clientCapabilities.generated.json' with { type: 'json' };
import { reachableTargetPolicies, resolveDistribution } from './registry.ts';
import { PLUGIN_NAME } from './registry-codecs/common.ts';
import type { ClientID, ComponentID, DistributionResolution, InstallScope, PluginSource, RegistryPlugin } from '../types/registry.ts';

// Published signed Directory v1 authenticates source identity. Released CLI
// 0.1.101 separately checks explicit Project placement for every component
// and mandatory dependency; User native evidence is NOT Project activation.
// One generated adapter-derived Project matrix. This is potential component
// delivery, NOT exact MCP-transport approval or runtime/activation evidence.
// The released CLI checks actual package bytes, dependencies and native policy.
const clientCapabilities = new Map(capabilities.map((client) => [client.id, client] as const));
// Pin the signed Git source, not the CLI launcher: npm's stable tag follows releases.
export const CATALOG_CLI_VERSION: string = 'latest';

// Broad capability preview only: the signed release does not expose which MCP
// transports a package needs (including SSE-only). The CLI decides exact fit.
export function projectComponentSupported(client: ClientID, component: ComponentID): boolean {
  return scopedComponentSupported(client, component, 'project');
}

export function scopedComponentSupported(client: ClientID, component: ComponentID, scope: InstallScope): boolean {
  const row = clientCapabilities.get(client);
  const scoped = scope === 'user' ? row : row && 'project' in row ? row.project : undefined;
  if (!scoped) return false;
  if (component === 'skills') return scoped.skill_support !== 'unsupported';
  if (component === 'extensions') return scoped.extension_support !== 'unsupported';
  return component === 'mcp' && (scoped.mcp_transports.stdio !== 'unsupported' ||
    scoped.mcp_transports['streamable-http'] !== 'unsupported');
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
    // Kiro's native extension only approves tools of a selected MCP server.
    // An extension-only summary cannot qualify without a portable component.
    if (client === 'kiro' && dist.components.every(component => component === 'extensions')) return false;
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

/**
 * Explicit single-agent alternative when a reviewed signed-v1 release has no
 * target policy for that agent. This uses the released exact Git source route;
 * it is never presented as a signed Directory target or used by auto-detect.
 */
export function pinnedCatalogSelection(
  plugin: RegistryPlugin, client: ClientID, scope: InstallScope,
): { selector: string; manifestName: string; source: PluginSource; components: readonly ComponentID[]; client: ClientID; scope: InstallScope } | undefined {
  if (plugin.trust_state !== 'reviewed' || !plugin.installable ||
      !clientCapabilities.get(client)?.saved_dependency ||
      (scope !== 'user' && scope !== 'project')) return undefined;
  // An explicit policy or denial, even on an earlier release, is not a grant to
  // reinterpret as a different install channel. Likewise v2 scoped closure.
  if (plugin.distributions.some((distribution) => distribution.releases.some((release) =>
    release.blocking_clients.includes(client) ||
    release.targets.some((target) =>
      target.client === client || target.canonical_closure !== undefined)))) return undefined;

  // Reuse the signed resolver for an ACTIVE source. Do not synthesize source
  // availability from an expired, revoked, blocked or review-only release.
  const userTargets = reachableTargetPolicies(plugin.distributions, 'user');
  for (const target of userTargets) {
    // An app-only binding authenticates a remote ChatGPT connection, not a
    // portable native package.
    if (target.app_binding) continue;
    const distribution = resolveDistribution(plugin, [target.client], 'user').distribution;
    // Generic extension capability does not prove a foreign extension carries
    // the selected agent's native manifest binding. Only portable Skills/MCP
    // may use this route; signed releases keep their normal path.
    if (!distribution || !distribution.components.length ||
        !distribution.components.every((component) =>
          ['skills', 'mcp'].includes(component) &&
          scopedComponentSupported(client, component, scope))) continue;

    const signedRelease = distribution.releases.find((release) =>
      release.release_sequence === distribution.release_sequence);
    // Never downgrade a v2 scoped closure into an alternate v1 grant.
    if (!signedRelease?.selectable || signedRelease.release_status !== 'active' ||
        !signedRelease.manifest_name || !PLUGIN_NAME.test(signedRelease.manifest_name) ||
        signedRelease.targets.some((item) => item.canonical_closure !== undefined)) continue;

    const { repository, revision, path } = distribution.source;
    const safePath = path === '' || (path !== '.' && path.split('/').every((part) =>
      /^[A-Za-z0-9._-]+$/.test(part) && part !== '.' && part !== '..'));
    if (!GIT_REPOSITORY.test(repository) || !revision || !FULL_REVISION.test(revision) ||
        !safePath || !/^sha256:[a-f0-9]{64}$/.test(distribution.source.tree_sha256) ||
        !/^sha256:[a-f0-9]{64}$/.test(distribution.source.manifest_sha256)) continue;
    return {
      selector: `github:${repository}@${revision}${path ? `//${path}` : ''}`,
      manifestName: signedRelease.manifest_name,
      source: distribution.source,
      components: distribution.components,
      client,
      scope,
    };
  }
  return undefined;
}

/**
 * One exact-source request for explicit local choices whose existing routes
 * identify the same immutable portable package. Signed grants are unchanged.
 */
export function pinnedCatalogGroupSelection(
  plugin: RegistryPlugin, clients: readonly ClientID[], scope: InstallScope,
) {
  const selections = clients.map(client => pinnedCatalogSelection(plugin, client, scope));
  const source = selections.find(selection => selection);
  if (!source || !clients.length) return undefined;
  const sameSource = (candidate: PluginSource) =>
    candidate.repository === source.source.repository && candidate.revision === source.source.revision &&
    candidate.path === source.source.path && candidate.tree_sha256 === source.source.tree_sha256 &&
    candidate.manifest_sha256 === source.source.manifest_sha256;
  if (!clients.every((client, index) => {
    if (!clientCapabilities.get(client)?.saved_dependency ||
        !source.components.every(component => scopedComponentSupported(client, component, scope))) return false;
    const pinned = selections[index];
    if (pinned) return sameSource(pinned.source) && pinned.manifestName === source.manifestName;
    // A signed target may join only through its own eligible scoped policy.
    // pinnedCatalogSelection already excludes every canonical scoped closure.
    const signed = scope === 'project'
      ? projectCatalogSelection(plugin, [client]).resolution.distribution
      : resolveDistribution(plugin, [client], 'user').distribution;
    return Boolean(signed && sameSource(signed.source) &&
      signed.targets.some(target => target.client === client && !target.app_binding) &&
      signed.components.length === source.components.length &&
      signed.components.every(component => source.components.includes(component)));
  })) return undefined;
  return { ...source, clients: [...new Set(clients)] };
}

export function pinnedCatalogCommands(plugin: RegistryPlugin, client: ClientID | readonly ClientID[], scope: InstallScope) {
  const clients = typeof client === 'string' ? [client] : [...client];
  const selection = clients.length === 1
    ? pinnedCatalogSelection(plugin, clients[0]!, scope)
    : pinnedCatalogGroupSelection(plugin, clients, scope);
  if (!selection) throw new Error('No eligible explicit exact-source installation');
  return installCommands(selection.selector, selection.manifestName, clients, undefined, scope,
    `npx -y universal-agent-plugins@${CATALOG_CLI_VERSION}`, clients);
}

/** Current release presentation only; source eligibility and CLI diagnostics stay unchanged. */
export function pinnedCatalogRepairLimitation(selection: ReturnType<typeof pinnedCatalogSelection>) {
  if (CATALOG_CLI_VERSION !== '0.1.108' || selection?.client !== 'antigravity' ||
      !selection.components.includes('mcp')) return undefined;
  return {
    key: 'registryUi.install.antigravityRepairUnavailable' as const,
    params: { version: CATALOG_CLI_VERSION },
  };
}
