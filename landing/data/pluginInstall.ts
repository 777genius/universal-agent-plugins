import { SAVED_DEPENDENCIES_CLI_RELEASED } from '../utils/savedDependencies';
import { installCommands } from '../utils/commands.ts';
import { contentByLocale } from '~/data/content';
import type {
  PluginInstallCommandSet,
  PluginInstallScope,
  PluginInstallSpec,
  PluginInstallTargetId,
  PluginManageCommands,
  PluginResolvedInstallLane,
  PluginResolvedInstallSpec,
  PluginTargetInstallLane,
} from '~/types/content';

const targetDefinitions: Record<PluginInstallTargetId, PluginTargetInstallLane> = {
  claude: {
    targetId: 'claude',
    badgeLabel: 'Claude',
    scope: 'user',
    installPath: 'Claude user plugins',
    followUp: 'reload',
    vendorDocsHref: 'https://code.claude.com/docs/en/discover-plugins',
  },
  codex: {
    targetId: 'codex',
    badgeLabel: 'Codex',
    scope: 'user',
    installPath: 'Codex local plugin marketplace',
    followUp: 'activation',
    vendorDocsHref: 'https://developers.openai.com/codex/plugins',
  },
  gemini: {
    targetId: 'gemini',
    badgeLabel: 'Gemini',
    scope: 'user',
    installPath: '~/.gemini/extensions',
    followUp: 'enable',
    vendorDocsHref: 'https://geminicli.com/docs/extensions/reference/',
  },
  opencode: {
    targetId: 'opencode',
    badgeLabel: 'OpenCode',
    scope: 'user',
    installPath: 'OpenCode user configuration',
    followUp: 'none',
    vendorDocsHref: 'https://opencode.ai/docs/plugins/',
  },
  cursor: {
    targetId: 'cursor',
    badgeLabel: 'Cursor',
    scope: 'user',
    installPath: '~/.cursor',
    followUp: 'none',
    vendorDocsHref: 'https://docs.cursor.com/context/mcp',
  },
};

const defaultTargetOrder: PluginInstallTargetId[] = [
  'claude',
  'codex',
  'gemini',
  'opencode',
  'cursor',
];

const defaultSupportedTargets = defaultTargetOrder.map((targetId) => targetDefinitions[targetId]);

const catalogPluginAliases = {
  context7: 'context7',
  gitlab: 'gitlab',
  github: 'github',
  firebase: 'firebase',
  linear: 'linear',
  cloudflare: 'cloudflare',
  'cloudflare-docs': 'cloudflare-docs',
  'cloudflare-observability': 'cloudflare-observability',
  'cloudflare-bindings': 'cloudflare-bindings',
  'cloudflare-radar': 'cloudflare-radar',
  'hubspot-crm': 'hubspot-crm',
  'hubspot-developer': 'hubspot-developer',
  heroku: 'heroku',
  neon: 'neon',
  'docker-hub': 'docker-hub',
  atlassian: 'atlassian',
  notion: 'notion',
  stripe: 'stripe',
  slack: 'slack',
  vercel: 'vercel',
  sentry: 'sentry',
  supabase: 'supabase',
  greptile: 'greptile',
} satisfies Record<string, string>;

const catalogPluginSpecs = Object.fromEntries(
  Object.entries(catalogPluginAliases).map(([slug, cliSource]) => [
    slug,
    {
      slug,
      cliSource,
      integrationName: slug,
      recommendedTargetOrder: defaultTargetOrder,
      supportedTargets: defaultSupportedTargets,
    } satisfies PluginInstallSpec,
  ]),
) as Record<string, PluginInstallSpec>;

const buildInstallCommand = (
  cliSource: string,
  targetIds: PluginInstallTargetId[],
  scope?: PluginInstallScope,
  includeTargets = true,
  savedDependenciesReleased = SAVED_DEPENDENCIES_CLI_RELEASED,
): string => {
  return installCommands(
    cliSource,
    cliSource,
    includeTargets && targetIds.length ? targetIds : undefined,
    savedDependenciesReleased,
    scope ?? 'user',
    'universal-agent-plugins',
  ).add;
};

const buildManageCommands = (
  integrationName: string,
  cliSource: string,
  targetIds: PluginInstallTargetId[],
  savedDependenciesReleased = SAVED_DEPENDENCIES_CLI_RELEASED,
  scope: PluginInstallScope = 'user',
): PluginManageCommands => {
  const { update, repair, remove } = installCommands(
    cliSource,
    integrationName,
    targetIds.length ? targetIds : undefined,
    savedDependenciesReleased,
    scope,
    'universal-agent-plugins',
  );
  return { update, repair, remove };
};

const buildCommandSet = (
  cliSource: string,
  integrationName: string,
  lane: PluginTargetInstallLane,
  savedDependenciesReleased = SAVED_DEPENDENCIES_CLI_RELEASED,
): PluginInstallCommandSet => ({
  install: buildInstallCommand(
    cliSource,
    [lane.targetId],
    lane.scope,
    true,
    savedDependenciesReleased,
  ),
  update: buildManageCommands(
    integrationName,
    cliSource,
    [lane.targetId],
    savedDependenciesReleased,
    lane.scope,
  ).update,
  repair: buildManageCommands(
    integrationName,
    cliSource,
    [lane.targetId],
    savedDependenciesReleased,
    lane.scope,
  ).repair,
  remove: buildManageCommands(
    integrationName,
    cliSource,
    [lane.targetId],
    savedDependenciesReleased,
    lane.scope,
  ).remove,
});

const resolveSupportedTargets = (
  spec: PluginInstallSpec,
  savedDependenciesReleased = SAVED_DEPENDENCIES_CLI_RELEASED,
): PluginResolvedInstallLane[] =>
  spec.supportedTargets.map((lane) => ({
    ...lane,
    commands: buildCommandSet(
      spec.cliSource,
      spec.integrationName,
      lane,
      savedDependenciesReleased,
    ),
  }));

const assertCatalogCoverage = () => {
  const catalogSlugs = contentByLocale.en.plugins.map((plugin) => plugin.slug || plugin.id);
  const missingSlugs = catalogSlugs.filter((slug) => !catalogPluginSpecs[slug]);

  if (missingSlugs.length > 0) {
    throw new Error(
      `landing plugin install registry is missing specs for: ${missingSlugs.join(', ')}`,
    );
  }
};

assertCatalogCoverage();

export const getPluginInstallSpec = (slug: string): PluginInstallSpec | undefined => {
  return catalogPluginSpecs[slug];
};

export const getResolvedPluginInstallSpec = (
  slug: string,
  savedDependenciesReleased = SAVED_DEPENDENCIES_CLI_RELEASED,
): PluginResolvedInstallSpec | undefined => {
  const spec = getPluginInstallSpec(slug);
  if (!spec) {
    return undefined;
  }

  return {
    slug: spec.slug,
    cliSource: spec.cliSource,
    integrationName: spec.integrationName,
    recommendedTargetOrder: spec.recommendedTargetOrder,
    supportedTargets: resolveSupportedTargets(spec, savedDependenciesReleased),
    manageCommands: buildManageCommands(
      spec.integrationName,
      spec.cliSource,
      [],
      savedDependenciesReleased,
    ),
  };
};

const resolveSelection = (
  spec: PluginResolvedInstallSpec,
  selectedTargetIds: PluginInstallTargetId[],
): { targetIds: PluginInstallTargetId[]; scope: PluginInstallScope } => {
  const allTargetIds = spec.recommendedTargetOrder.filter((targetId) =>
    spec.supportedTargets.some((lane) => lane.targetId === targetId),
  );
  const targetIds = [...new Set(selectedTargetIds.length > 0 ? selectedTargetIds : allTargetIds)];
  const selectedLanes = targetIds.map((targetId) =>
    spec.supportedTargets.find((lane) => lane.targetId === targetId),
  );
  if (!selectedLanes.length || selectedLanes.some((lane) => !lane))
    throw new Error('Unsupported install target selection');
  const uniqueScopes = [...new Set(selectedLanes.map((lane) => lane!.scope))];
  if (uniqueScopes.length !== 1)
    throw new Error('Selected agents must use the same installation scope');
  return { targetIds, scope: uniqueScopes[0]! };
};

export const buildInstallCommandForSelection = (
  spec: PluginResolvedInstallSpec,
  selectedTargetIds: PluginInstallTargetId[],
  savedDependenciesReleased = SAVED_DEPENDENCIES_CLI_RELEASED,
): string => {
  const { targetIds, scope } = resolveSelection(spec, selectedTargetIds);
  // Keep generated commands explicit even when the user leaves every target
  // selected. An omitted --target invokes interactive detection and can differ
  // from the targets shown in the UI.
  return buildInstallCommand(spec.cliSource, targetIds, scope, true, savedDependenciesReleased);
};

export const buildManageCommandsForSelection = (
  spec: PluginResolvedInstallSpec,
  selectedTargetIds: PluginInstallTargetId[],
  savedDependenciesReleased = SAVED_DEPENDENCIES_CLI_RELEASED,
): PluginManageCommands => {
  const { targetIds, scope } = resolveSelection(spec, selectedTargetIds);
  return buildManageCommands(
    spec.integrationName,
    spec.cliSource,
    targetIds,
    savedDependenciesReleased,
    scope,
  );
};
