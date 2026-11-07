import { pluginCommands } from './commands.ts';
import { reachableTargetPolicies, resolveDistribution } from './registry.ts';
import type { ClientID, DistributionResolution, RegistryPlugin } from '../types/registry.ts';

// Published signed Directory v1 authenticates source identity. Released CLI
// 0.1.101 separately checks explicit Project placement for every component
// and mandatory dependency; User native evidence is NOT Project activation.
const PROJECT_COMPONENTS: Partial<Record<ClientID, ReadonlySet<string>>> = {
  claude: new Set(['skills', 'mcp']),
  codex: new Set(['skills', 'mcp']),
  opencode: new Set(['skills', 'mcp']),
  cursor: new Set(['skills', 'mcp']),
  gemini: new Set(['skills', 'mcp']),
  copilot: new Set(['skills', 'mcp']),
  kimi: new Set(['skills', 'mcp']),
  cline: new Set(['skills', 'mcp']),
  kiro: new Set(['skills', 'mcp']),
  vscode: new Set(['skills', 'mcp']),
  pi: new Set(['skills', 'mcp', 'extensions']),
  qoder: new Set(['skills', 'mcp']),
  windsurf: new Set(['skills']), // legacy Cascade has no qualified Project MCP
  grok: new Set(['skills']),
};
const GIT_REPOSITORY = /^[A-Za-z0-9][A-Za-z0-9-]*\/[A-Za-z0-9][A-Za-z0-9._-]*$/;
const FULL_REVISION = /^[0-9a-f]{40}$/;

function projectCompatible(plugin: RegistryPlugin, targets: readonly ClientID[], resolved: DistributionResolution) {
  const dist = resolved.distribution;
  if (!plugin.installable || plugin.trust_state !== 'reviewed' || !targets.length ||
      !dist || !dist.components.length) return false;
  // Reject malformed metadata in test projections too; signed feed validation
  // and exact source verification remain authoritative in the released CLI.
  const { repository, revision, path } = dist.source;
  if (!GIT_REPOSITORY.test(repository) || (revision !== null && !FULL_REVISION.test(revision)) ||
      (path !== '' && path !== '.' && (!path || path.split('/').some(
        (part) => !/^[A-Za-z0-9._-]+$/.test(part) || part === '.' || part === '..',
      )))) return false;
  return targets.every((client) => {
    const allowed = PROJECT_COMPONENTS[client];
    const target = dist.targets.find((candidate) => candidate.client === client);
    return Boolean(allowed && target && !target.app_binding &&
      dist.components.every((component) => allowed.has(component)));
  });
}

export function projectCatalogSelection(
  plugin: RegistryPlugin,
  targets: readonly ClientID[],
): { resolution: DistributionResolution } {
  const signedProject = resolveDistribution(plugin, targets, 'project');
  // Honor explicit project-scoped policy if present. Signed v1 User grants
  // also authenticate sources for the owner's explicit Project request.
  const selected = signedProject.distribution ? signedProject : resolveDistribution(plugin, targets, 'user');
  if (!projectCompatible(plugin, targets, selected)) {
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
