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
    scope: 'project',
    installPath: 'opencode.json and .opencode/',
    projectRootRequired: true,
    followUp: 'none',
    vendorDocsHref: 'https://opencode.ai/docs/plugins/',
  },
  cursor: {
    targetId: 'cursor',
    badgeLabel: 'Cursor',
    scope: 'project',
    installPath: '.cursor/mcp.json',
    projectRootRequired: true,
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
): string => {
  const args = ['universal-agent-plugins', 'add', cliSource];

  if (includeTargets && targetIds.length > 0) {
    args.push('--target', targetIds.join(','));
  }

  if (scope === 'project') {
    args.push('--scope', 'project');
  }

  return args.join(' ');
};

const buildManageCommands = (integrationName: string): PluginManageCommands => ({
  update: `universal-agent-plugins update ${integrationName}`,
  repair: `universal-agent-plugins repair ${integrationName}`,
  remove: `universal-agent-plugins remove ${integrationName}`,
});

const buildCommandSet = (
  cliSource: string,
  integrationName: string,
  lane: PluginTargetInstallLane,
): PluginInstallCommandSet => ({
  install: buildInstallCommand(cliSource, [lane.targetId], lane.scope, true),
  update: `universal-agent-plugins update ${integrationName}`,
  repair: `universal-agent-plugins repair ${integrationName} --target ${lane.targetId}`,
  remove: `universal-agent-plugins remove ${integrationName}`,
});

const resolveSupportedTargets = (spec: PluginInstallSpec): PluginResolvedInstallLane[] =>
  spec.supportedTargets.map((lane) => ({
    ...lane,
    commands: buildCommandSet(spec.cliSource, spec.integrationName, lane),
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
    supportedTargets: resolveSupportedTargets(spec),
    manageCommands: buildManageCommands(spec.integrationName),
  };
};

export const buildInstallCommandForSelection = (
  spec: PluginResolvedInstallSpec,
  selectedTargetIds: PluginInstallTargetId[],
): string => {
  const allTargetIds = spec.recommendedTargetOrder.filter((targetId) =>
    spec.supportedTargets.some((lane) => lane.targetId === targetId),
  );
  const normalizedTargetIds = selectedTargetIds.length > 0 ? selectedTargetIds : allTargetIds;
  const selectedLanes = normalizedTargetIds
    .map((targetId) => spec.supportedTargets.find((lane) => lane.targetId === targetId))
    .filter((lane): lane is NonNullable<typeof lane> => Boolean(lane));
  // Keep generated commands explicit even when the user leaves every target
  // selected. An omitted --target invokes interactive detection and can differ
  // from the targets shown in the UI.
  const includeTargets = true;
  const uniqueScopes = [...new Set(selectedLanes.map((lane) => lane.scope))];
  const scope = !includeTargets
    ? undefined
    : uniqueScopes.length === 1 && uniqueScopes[0] === 'project'
      ? 'project'
      : undefined;

  return buildInstallCommand(spec.cliSource, normalizedTargetIds, scope, includeTargets);
};
