import { installCommands, pluginCommands, shellToken } from './commands.ts';
import { resolveDistribution } from './registry.ts';
import type { ClientID, DistributionResolution, RegistryPlugin } from '../types/registry.ts';

// These clients have existing project preparation/lifecycle coverage. This is
// deliberately narrower than the complete client capability matrix: a signed
// *user* policy is not proof that arbitrary project activation works.
const DIRECT_PROJECT_CLIENTS = new Set<ClientID>(['claude', 'codex', 'opencode']);
const GIT_REPOSITORY = /^[A-Za-z0-9][A-Za-z0-9-]*\/[A-Za-z0-9][A-Za-z0-9._-]*$/;
const FULL_REVISION = /^[0-9a-f]{40}$/;

function pinnedSource(plugin: RegistryPlugin, targets: readonly ClientID[]) {
  if (plugin.trust_state !== 'reviewed' || !targets.length ||
      targets.some((client) => !DIRECT_PROJECT_CLIENTS.has(client))) return undefined;
  const selected = resolveDistribution(plugin, targets, 'user');
  const distribution = selected.distribution;
  if (!distribution || !distribution.components.length ||
      distribution.components.some((part) => part !== 'skills' && part !== 'mcp')) return undefined;
  const { repository, revision, path } = distribution.source;
  if (!GIT_REPOSITORY.test(repository) || !revision || !FULL_REVISION.test(revision) ||
      (path !== '.' && (!path || path.split('/').some(
        (part) => !/^[A-Za-z0-9._-]+$/.test(part) || part === '.' || part === '..',
      )))) return undefined;
  return { selection: selected, source: `github:${repository}@${revision}${path === '.' ? '' : `//${path}`}` };
}

export function projectCatalogSelection(
  plugin: RegistryPlugin,
  targets: readonly ClientID[],
): { resolution: DistributionResolution; directSource?: string } {
  // An explicitly qualified signed project policy always takes precedence.
  const signed = resolveDistribution(plugin, targets, 'project');
  if (signed.distribution) return { resolution: signed };
  const pinned = pinnedSource(plugin, targets);
  if (pinned) {
    // A user-target policy can establish a reviewed source, NOT native project
    // activation. Project delivery remains Prepared until the client proves it.
    const distribution = pinned.selection.distribution!;
    return {
      resolution: {
        ...pinned.selection,
        distribution: {
          ...distribution,
          targets: distribution.targets.map((target) => ({
            ...target,
            delivery: 'prepared' as const,
          })),
        },
      },
      directSource: pinned.source,
    };
  }
  return { resolution: signed };
}

export function projectCatalogCommands(
  plugin: RegistryPlugin,
  targets: readonly ClientID[],
) {
  const selected = projectCatalogSelection(plugin, targets);
  if (!selected.resolution.distribution) throw new Error('No eligible project installation source');
  if (selected.directSource) {
    // The signed v1 release authenticates immutable bytes, but its target policy
    // is *user-only*. Use an explicit direct Git route, not a fictitious
    // project-enabled Directory alias. Installation remains Prepared until the
    // real client verifies trust/activation.
    return installCommands(shellToken(selected.directSource), plugin.name, targets,
      undefined, 'project');
  }
  return pluginCommands(plugin, targets, undefined, 'project');
}
