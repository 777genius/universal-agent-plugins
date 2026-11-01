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
/** A publisher root, never the third-party catalog's detail URL. */
export function skillPublisherUrl(skill: SkillListing): string | undefined {
  if (skill.source.kind !== 'external' || skill.availability !== 'unverified') return undefined;
  const raw = skill.source.url;
  try {
    const url = new URL(raw);
    const host = url.hostname;
    if (
      url.protocol !== 'https:' ||
      url.host !== host ||
      url.username ||
      url.password ||
      url.pathname !== '/' ||
      url.search ||
      url.hash ||
      !/^[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?\.[a-z]{2,}$/.test(host) ||
      raw !== `https://${host}`
    ) return undefined;
    return raw;
  } catch {
    return undefined;
  }
}

/** Derive a source solely from a bounded, canonical catalog identity. */
export function skillIdentityFromId(id: string): SkillListing['source'] | undefined {
  const match = /^skills-sh:([^:]+):([^:]+)$/.exec(id);
  if (!match || id.length > 1024) return undefined;
  const source = match[1]!;
  if (/^[A-Za-z0-9][A-Za-z0-9-]*\/[A-Za-z0-9][A-Za-z0-9._-]*$/.test(source)) {
    return { kind: 'github', repository: source, url: `https://github.com/${source}` };
  }
  if (
    source.length <= 253 &&
    /^[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?\.[a-z]{2,}$/i.test(source)
  ) {
    return { kind: 'external', url: `https://${source.toLowerCase()}` };
  }
  return undefined;
}

export function skillDetailRoute(skill: SkillListing) {
  return {
    path: '/skills/',
    query: { id: skill.id, q: skill.skillNameHint },
  };
}

function supportedSkillName(name: string): boolean {
  return [...name].length <= 64 && /^[\p{Ll}\p{N}]+(?:-[\p{Ll}\p{N}]+)*$/u.test(name);
}

/** A safe repository-only command lets the CLI present its real Skill picker. */
export function skillNeedsManualSelection(skill: SkillListing): boolean {
  return Boolean(
    skillRepositoryUrl(skill) &&
      skill.availability !== 'unsupported-source' &&
      !supportedSkillName(skill.skillNameHint),
  );
}

export function skillInstallCommand(
  skill: SkillListing,
  targets?: readonly string[],
): string | undefined {
  const source = skillRepositoryUrl(skill) ?? skillPublisherUrl(skill);
  if (!source || skill.availability === 'unsupported-source') return undefined;
  // A website has no Git selector/picker: it must use a safe exact Skill name.
  if (skill.source.kind === 'external' && !supportedSkillName(skill.skillNameHint)) return undefined;
  const tokens = ['npx', 'universal-agent-plugins@latest', 'skills', 'add', source];
  if (!skillNeedsManualSelection(skill)) tokens.push('--skill', skill.skillNameHint);
  if (targets !== undefined) {
    const ids = [...new Set(targets.map((id) => id.trim()).filter(Boolean))];
    if (!ids.length || ids.some((id) => !skillTargetIds.has(id))) return undefined;
    tokens.push('--target', ids.join(','));
  }
  // Every dynamic token is a validated repository, normative name or known
  // target ID. Unverified names open the CLI picker instead of being guessed.
  // Double quotes preserve Unicode in CMD and CSV in PowerShell; these
  // identifiers cannot contain shell expansions or quote delimiters.
  return tokens
    .map((token) => (/^[A-Za-z0-9_./:@+=-]+$/.test(token) ? token : `"${token}"`))
    .join(' ');
}
