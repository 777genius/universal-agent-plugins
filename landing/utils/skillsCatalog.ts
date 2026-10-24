import type { SkillsView, SkillListing } from '../types/skillsCatalog.ts';
import { skillClients } from '../data/skillClients.ts';

const skillTargetIds = new Set<string>(skillClients.map((client) => client.id));

function queryValue(value: unknown): string {
  const first = Array.isArray(value) ? value[0] : value;
  return typeof first === 'string' ? first.trim() : '';
}
export function isSkillsCatalog(query: Record<string, unknown>): boolean {
  return queryValue(query.tab) === 'skills';
}
export function catalogModeQuery(query: Record<string, unknown>, skills: boolean) {
  const next = { ...query } as Record<string, string | string[] | null | undefined>;
  if (skills) next.tab = 'skills';
  else delete next.tab;
  return next;
}
export function restoreSkillsQuery(query: Record<string, unknown>) {
  const view = queryValue(query.skillView);
  return {
    query: queryValue(query.q),
    view: (view === 'trending' || view === 'hot' ? view : 'popular') as SkillsView,
  };
}
export function skillsCatalogQuery(
  query: string,
  view: SkillsView,
  existing: Record<string, unknown>,
) {
  const next = catalogModeQuery(existing, true);
  if (query.trim()) next.q = query.trim();
  else delete next.q;
  if (view !== 'popular') next.skillView = view;
  else delete next.skillView;
  return next;
}
export function skillRepositoryUrl(skill: SkillListing): string | undefined {
  const repository = skill.source.repository;
  return skill.source.kind === 'github' &&
    repository &&
    /^[A-Za-z0-9][A-Za-z0-9-]*\/[A-Za-z0-9][A-Za-z0-9._-]*$/.test(repository)
    ? `https://github.com/${repository}`
    : undefined;
}
export function externalListingUrl(value: string): string | undefined {
  try {
    const url = new URL(value);
    return url.protocol === 'https:' && !url.username && !url.password ? url.href : undefined;
  } catch {
    return undefined;
  }
}
export function skillInstallCommand(
  skill: SkillListing,
  targets?: readonly string[],
): string | undefined {
  const source = skillRepositoryUrl(skill);
  if (
    !source ||
    skill.availability === 'unsupported-source' ||
    // Match the installer's normative Skill-name grammar before offering a
    // command. Restricting metadata also keeps copied commands safe in both
    // POSIX shells and PowerShell, whose quote rules differ.
    [...skill.skillNameHint].length > 64 ||
    !/^[\p{Ll}\p{N}]+(?:-[\p{Ll}\p{N}]+)*$/u.test(skill.skillNameHint)
  )
    return undefined;
  const tokens = [
    'npx',
    'universal-agent-plugins@latest',
    'skills',
    'add',
    source,
    '--skill',
    skill.skillNameHint,
  ];
  if (targets !== undefined) {
    const ids = [...new Set(targets.map((id) => id.trim()).filter(Boolean))];
    if (!ids.length || ids.some((id) => !skillTargetIds.has(id))) return undefined;
    tokens.push('--target', ids.join(','));
  }
  // Every dynamic token is a validated repository, normative name or known
  // target ID. Double quotes preserve Unicode in CMD and CSV in PowerShell;
  // these identifiers cannot contain shell expansions or quote delimiters.
  return tokens
    .map((token) => (/^[A-Za-z0-9_./:@+=-]+$/.test(token) ? token : `"${token}"`))
    .join(' ');
}
