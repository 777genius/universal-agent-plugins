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
  if (skill.source.kind === 'github' && repository &&
      /^[A-Za-z0-9][A-Za-z0-9-]*\/[A-Za-z0-9][A-Za-z0-9._-]*$/.test(repository))
    return `https://github.com/${repository}`;
  const source = legacySource(skill);
  return source?.kind === 'github' ? source.url : undefined;
}

type LegacySource = { kind: 'github' | 'publisher'; url: string };

/**
 * Recover an explicit upstream source, never infer one from a skills.sh page.
 * Legacy DTOs only retain the original "source" string in their namespaced ID.
 * Metadata must identify the exact unsupported provider fallback shape.
 */
function legacySource(skill: SkillListing): LegacySource | undefined {
  if (skill.providerId !== 'skills-sh' || skill.availability !== 'unsupported-source' ||
      skill.source.kind !== 'external' || skill.source.repository ||
      skill.providerUrl !== 'https://www.skills.sh/' ||
      skill.source.url !== skill.providerUrl ||
      !skill.id.startsWith('skills-sh:') || skill.id.length > 1024)
    return undefined;
  const identity = catalogSourceIdentity(skill.id);
  // An old unsupported-source DTO is not authoritative when its selector
  // disagrees with the upstream ID. Never install a different Skill by name.
  if (!identity || identity.slug !== skill.skillNameHint) return undefined;
  const github = githubSource(identity.raw);
  // A plain owner/repository should already have kind=github upstream. A
  // contradictory unsupported-source DTO must not gain install authority.
  // An explicit .git remote is not the same ambiguous legacy owner/repo
  // fallback: normalize this unambiguous upstream remote into HTTPS.
  if (github && (!/^[A-Za-z0-9-]+\/[A-Za-z0-9._-]+$/.test(identity.raw) ||
      identity.raw.endsWith('.git')))
    return { kind: 'github', url: github };
  const publisher = publisherBase(identity.raw);
  return publisher ? { kind: 'publisher', url: publisher } : undefined;
}

/** Parse the source and selector boundaries without assuming a colon-free URL. */
function catalogSourceIdentity(id: string): { raw: string; slug: string } | undefined {
  if (typeof id !== 'string' || !id.startsWith('skills-sh:') || id.length > 1024)
    return undefined;
  const separator = id.lastIndexOf(':');
  if (separator <= 'skills-sh:'.length || separator >= id.length - 1) return undefined;
  const raw = id.slice('skills-sh:'.length, separator);
  const slug = id.slice(separator + 1);
  if (raw.length > 512 || slug.length > 256 ||
      /[\x00-\x20\x7f]/.test(raw) || /[\x00-\x1f\x7f]/.test(slug)) return undefined;
  return { raw, slug };
}

/** Accept GitHub's explicit repository and single-segment-ref file/tree forms. */
function githubSource(raw: string): string | undefined {
  if (raw.length < 3 || raw.length > 512) return undefined;
  // The CLI supports an exact raw GitHub SKILL.md URL as a source.
  if (raw.startsWith('https://raw.githubusercontent.com/')) {
    const segments = raw.slice('https://raw.githubusercontent.com/'.length).split('/');
    if (segments.length < 4 || segments.at(-1) !== 'SKILL.md' ||
        !/^[A-Za-z0-9](?:[A-Za-z0-9-]{0,37}[A-Za-z0-9])?$/.test(segments[0]!) ||
        !/^[A-Za-z0-9_.-]{1,100}$/.test(segments[1]!) ||
        segments[1] === '.' || segments[1] === '..' ||
        segments.slice(2).some(part => part === '.' || part === '..' ||
          !/^[A-Za-z0-9_.-]{1,128}$/.test(part))) return undefined;
    return raw;
  }
  let path = raw;
  if (path.startsWith('git+https://github.com/')) {
    path = path.slice('git+https://github.com/'.length);
  } else if (path.startsWith('https://github.com/')) {
    path = path.slice('https://github.com/'.length);
  } else if (path.startsWith('github.com/')) {
    path = path.slice('github.com/'.length);
  } else if (path.startsWith('git@github.com:')) {
    path = path.slice('git@github.com:'.length);
  }
  const parts = path.split('/');
  if (parts.length < 2) return undefined;
  const owner = parts[0]!;
  const originalRepo = parts[1]!;
  const repo = parts.length === 2 ? originalRepo.replace(/\.git$/, '') : originalRepo;
  if (!/^[A-Za-z0-9](?:[A-Za-z0-9-]{0,37}[A-Za-z0-9])?$/.test(owner) ||
      !/^[A-Za-z0-9_.-]{1,100}$/.test(repo) || repo === '.' || repo === '..' ||
      repo.endsWith('.git')) return undefined;
  const base = `https://github.com/${owner}/${repo}`;
  if (parts.length === 2) return base;
  if (parts.length < 4 || (parts[2] !== 'tree' && parts[2] !== 'blob')) return undefined;
  const suffix = parts.slice(3);
  if (suffix.some(segment => segment === '.' || segment === '..' ||
      !/^[A-Za-z0-9_.-]{1,128}$/.test(segment))) return undefined;
  if (parts[2] === 'blob' && (suffix.length < 2 || suffix.at(-1) !== 'SKILL.md'))
    return undefined;
  return `${base}/${parts.slice(2).join('/')}`;
}

/** Canonical HTTPS well-known publisher base, including explicit scoped bases. */
function publisherBase(raw: string): string | undefined {
  try {
    // The directory's source is sometimes "host/scope" without a scheme.
    // Upgrade only an exact public-domain locator; never rewrite arbitrary URLs.
    const bare = /^([A-Za-z0-9](?:[A-Za-z0-9.-]*[A-Za-z0-9])?\.[A-Za-z]{2,})(\/[A-Za-z0-9._~-]+(?:\/[A-Za-z0-9._~-]+)*)?$/.exec(raw);
    const candidate = bare ? `https://${bare[1]!.toLowerCase()}${bare[2] ?? ''}` : raw;
    const url = new URL(candidate);
    const host = url.hostname;
    const scope = url.pathname === '/' ? '' : url.pathname;
    if (url.protocol !== 'https:' || url.host !== host ||
        url.username || url.password || url.search || url.hash ||
        candidate !== `https://${host}${scope}` ||
        !/^[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?\.[a-z]{2,}$/.test(host) ||
        /(?:^|\.)xn--/.test(host) ||
        /\.(?:local|localhost|internal|test|example|invalid)$/.test(host) ||
        host === 'skills.sh' || host === 'www.skills.sh' ||
        host === 'github.com' || host === 'www.github.com' ||
        // Git repositories on other forges are not HTTPS Skill publishers.
        host === 'gitlab.com' || host === 'www.gitlab.com' ||
        host === 'bitbucket.org' || host === 'www.bitbucket.org' ||
        host === 'codeberg.org' || host === 'www.codeberg.org' ||
        (scope !== '' && (!/^\/[A-Za-z0-9._~-]+(?:\/[A-Za-z0-9._~-]+)*$/.test(scope) ||
          scope.split('/').some(part => part === '.' || part === '..'))))
      return undefined;
    return candidate;
  } catch {
    return undefined;
  }
}
/**
 * Recognize the exact website-only DTO produced before the Skills API update.
 * An old API encodes a safe publisher domain in the listing ID, but marks its
 * own skills.sh listing link as unsupported-source. It cannot be installed as
 * a package: only the original HTTPS publisher can be passed to UAP.
 *
 * Require all independent legacy fields to agree. No arbitrary provider URL,
 * guessed GitHub path or inconsistent source identity is accepted. The CLI
 * still verifies the publisher index and exact Skill before installation.
 */
function legacyPublisherUrl(skill: SkillListing): string | undefined {
  if (
    skill.providerId !== 'skills-sh' ||
    skill.availability !== 'unsupported-source' ||
    skill.source.kind !== 'external' ||
    skill.source.repository ||
    skill.source.url !== skill.providerUrl
  ) return undefined;

  const parts = /^skills-sh:([^:]+):([^:]+)$/.exec(skill.id);
  const source = skillIdentityFromId(skill.id);
  if (!parts || parts[2] !== skill.skillNameHint || source?.kind !== 'external') return undefined;

  // Chromium accepts some malformed IDNA hosts rejected by Node (xn--a).
  // Until the original publisher is verified, reject all punycode legacy IDs
  // rather than exposing a browser-only Install action.
  if (/(?:^|\.)xn--/i.test(parts[1]!)) return undefined;

  try {
    const publisher = new URL(source.url);
    const legacyListing = `https://www.skills.sh/site/${publisher.hostname}/${encodeURIComponent(parts[2]!.toLowerCase())}`;
    return skill.providerUrl === legacyListing ? source.url : undefined;
  } catch {
    // Malformed IDNA or lone-surrogate catalog text must disable one entry,
    // never throw while Vue renders the entire Skills catalog.
    return undefined;
  }
}

/** A publisher root, never the third-party catalog's detail URL. */
export function skillPublisherUrl(skill: SkillListing): string | undefined {
  const legacy = legacyPublisherUrl(skill);
  if (legacy) return legacy;
  const compatible = legacySource(skill);
  if (compatible?.kind === 'publisher') return compatible.url;
  return skill.source.kind === 'external' && skill.availability === 'unverified'
    ? publisherBase(skill.source.url)
    : undefined;
}

/** Derive a source solely from a bounded, explicit catalog identity. */
export function skillIdentityFromId(id: string): SkillListing['source'] | undefined {
  const parsed = catalogSourceIdentity(id);
  if (!parsed) return undefined;
  const repository = githubSource(parsed.raw);
  if (repository) {
    const parts = new URL(repository).pathname.split('/').filter(Boolean);
    return { kind: 'github', repository: `${parts[0]}/${parts[1]}`, url: repository };
  }
  const publisher = publisherBase(
    /^[A-Za-z0-9](?:[A-Za-z0-9.-]*[A-Za-z0-9])?\.[A-Za-z]{2,}$/.test(parsed.raw)
      ? `https://${parsed.raw.toLowerCase()}`
      : parsed.raw,
  );
  return publisher ? { kind: 'external', url: publisher } : undefined;
}

/** Do not reuse a catalog entry when the direct-link Skill selector changes. */
export function skillMatchesSelection(skill: SkillListing, id: string, name: string): boolean {
  return skill.id === id && skill.skillNameHint === name;
}

export function skillDetailRoute(skill: SkillListing) {
  return {
    path: '/skills/',
    query: { id: skill.id, q: skill.skillNameHint },
  };
}

function supportedSkillName(name: string): boolean {
  return typeof name === 'string' && [...name].length <= 64 && /^[\p{Ll}\p{N}]+(?:-[\p{Ll}\p{N}]+)*$/u.test(name);
}

/** A safe repository-only command lets the CLI present its real Skill picker. */
export function skillNeedsManualSelection(skill: SkillListing): boolean {
  return Boolean(
    skillRepositoryUrl(skill) &&
      (skill.availability !== 'unsupported-source' ||
        legacySource(skill)?.kind === 'github') &&
      !supportedSkillName(skill.skillNameHint),
  );
}

/**
 * Known real-source refusals: Feishu's lark-mail uses non-string nested YAML
 * metadata, and the scoped Vercel URL lacks a scoped UAP discovery index.
 * These are NOT successes merely because their publisher URL is parseable.
 * Keep both hidden until the published CLI can actually install them.
 */
function unavailablePublisherSkill(source: string, name: string): boolean {
  return name === 'lark-mail' && (
    source === 'https://open.feishu.cn' ||
    source === 'https://open.feishu.cn/lark-cli/skills/regular'
  );
}

export function skillInstallCommand(
  skill: SkillListing,
  targets?: readonly string[],
): string | undefined {
  if (!skill || typeof skill !== 'object' || !skill.source ||
      typeof skill.id !== 'string' || typeof skill.source.kind !== 'string')
    return undefined;
  const source = skillRepositoryUrl(skill) ?? skillPublisherUrl(skill);
  if (source && unavailablePublisherSkill(source, skill.skillNameHint)) return undefined;
  const compatible = legacySource(skill);
  if (!source || (skill.availability === 'unsupported-source' && !legacyPublisherUrl(skill) &&
      !compatible)) return undefined;
  // HTTPS discovery does not have a repository picker: exact name required.
  if (!skillRepositoryUrl(skill) &&
      (typeof skill.skillNameHint !== 'string' || !supportedSkillName(skill.skillNameHint)))
    return undefined;
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
