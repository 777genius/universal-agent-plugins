import type { RegistryPlugin } from '../types/registry';

export interface CatalogFilters {
  query?: string;
  category?: string;
  component?: RegistryPlugin['components'][number];
  source?: 'all' | 'upstream' | 'community_bridge' | 'community' | 'direct';
  trust?: 'all' | 'reviewed' | 'conformant_unreviewed';
  client?: 'all' | RegistryPlugin['client_support']['clients'][number];
  authentication?: 'all' | 'none' | 'required_or_unknown';
  owner?: string;
}

const retiredCatalogRepositories = new Set(['777genius/universal-agent-plugins-registry']);

/** Keep retired and metadata-only Discovery records available by direct URL, not in the catalog. */
export function catalogVisiblePlugins(plugins: RegistryPlugin[]): RegistryPlugin[] {
  return plugins.filter(
    (plugin) =>
      !(
        plugin.trust_state === 'conformant_unreviewed' &&
        retiredCatalogRepositories.has(plugin.source.repository.toLowerCase())
      ) &&
      (plugin.trust_state !== 'conformant_unreviewed' || plugin.installable),
  );
}

function normalized(value: string): string {
  return value.trim().toLowerCase();
}

export const catalogQueryKeys = [
  'q',
  'category',
  'component',
  'source',
  'trust',
  'client',
  'auth',
  'owner',
] as const;

/** Only the catalog's keys are changed; unrelated route state is preserved. */
export function catalogQuery(values: string[], existing: Record<string, unknown> = {}) {
  const result = { ...existing };
  catalogQueryKeys.forEach((key, index) => {
    const value = values[index]?.trim();
    if (value && (key === 'q' || value !== 'all')) result[key] = value;
    else Reflect.deleteProperty(result, key);
  });
  return result;
}

export function restoreCatalogQuery(query: Record<string, unknown>): string[] {
  return catalogQueryKeys.map((key) => {
    const value = Array.isArray(query[key]) ? query[key][0] : query[key];
    return typeof value === 'string' && value.trim() ? value.trim() : key === 'q' ? '' : 'all';
  });
}

function primaryPriority(plugin: RegistryPlugin): number[] {
  return [
    plugin.trust_state === 'conformant_unreviewed' ? 1 : 0,
    plugin.distributions.some((item) => item.kind === 'upstream') ? 0 : 1,
    plugin.installable && plugin.discovery?.availability !== 'unavailable' ? 0 : 1,
    /(^|[/_.-])(tests?|fixtures?|examples?|demo)([/_.-]|$)/i.test(
      `${plugin.source.repository}/${plugin.source.path}`,
    )
      ? 1
      : 0,
    -(plugin.discovery?.stars ?? 0),
  ];
}

export function groupCatalogPlugins(plugins: RegistryPlugin[]) {
  type SourceGroup = { members: RegistryPlugin[]; keys: Set<string>; order: number };
  const groups = new Set<SourceGroup>();
  const byIdentity = new Map<string, SourceGroup>();
  for (const [order, plugin] of plugins.entries()) {
    const keys = new Set([
      `name:${normalized(plugin.name)}`,
      `source:${normalized(plugin.source.repository)}/${plugin.source.path.replace(/^\/+|\/+$/g, '')}`,
      ...(plugin.discovery?.reviewed_distribution_id
        ? [`distribution:${plugin.discovery.reviewed_distribution_id}`]
        : []),
      ...((plugin.trust_state ?? 'reviewed') === 'reviewed'
        ? plugin.distributions.map((item) => `distribution:${item.id}`)
        : []),
    ]);
    const overlapping = [
      ...new Set([...keys].flatMap((key) => (byIdentity.has(key) ? [byIdentity.get(key)!] : []))),
    ];
    const group = {
      members: [plugin, ...overlapping.flatMap((item) => item.members)],
      keys: new Set([...keys, ...overlapping.flatMap((item) => [...item.keys])]),
      order: Math.min(order, ...overlapping.map((item) => item.order)),
    };
    overlapping.forEach((item) => groups.delete(item));
    groups.add(group);
    group.keys.forEach((key) => byIdentity.set(key, group));
  }
  return [...groups]
    .sort((left, right) => left.order - right.order)
    .map((group) => {
      group.members.sort((left, right) => {
        const a = primaryPriority(left);
        const b = primaryPriority(right);
        return (
          a.map((value, index) => value - b[index]!).find((value) => value !== 0) ??
          compareText(left.install_source, right.install_source)
        );
      });
      return { primary: group.members[0]!, alternatives: group.members.slice(1) };
    });
}

/** One insertion/deletion/substitution or adjacent transposition, for meaningful words only. */
function typoMatch(value: string, query: string): boolean {
  if (query.length < 4) return false;
  return normalized(value)
    .split(/[^\p{L}\p{N}]+/u)
    .some((word) => {
      if (Math.abs(word.length - query.length) > 1) return false;
      let index = 0;
      while (index < Math.min(word.length, query.length) && word[index] === query[index]) index++;
      if (word.length === query.length)
        return (
          word.slice(index + 1) === query.slice(index + 1) ||
          (word[index] === query[index + 1] &&
            word[index + 1] === query[index] &&
            word.slice(index + 2) === query.slice(index + 2))
        );
      return word.length > query.length
        ? word.slice(index + 1) === query.slice(index)
        : word.slice(index) === query.slice(index + 1);
    });
}

function sourceOwner(plugin: RegistryPlugin): string {
  return plugin.source.repository.split('/', 1)[0] ?? plugin.source.repository;
}

function compareText(left: string, right: string): number {
  return left < right ? -1 : left > right ? 1 : 0;
}

function compareDiscoveryPackages(left: RegistryPlugin, right: RegistryPlugin): number {
  const leftDepth = left.source.path ? left.source.path.split('/').length : 0;
  const rightDepth = right.source.path ? right.source.path.split('/').length : 0;
  return (
    leftDepth - rightDepth ||
    compareText(normalized(left.display_name), normalized(right.display_name))
  );
}

/** Stars belong to repositories, so popular monorepos are interleaved. */
function diversifiedDiscovery(plugins: RegistryPlugin[]): RegistryPlugin[] {
  const groups = new Map<string, RegistryPlugin[]>();
  for (const plugin of plugins) {
    const group = groups.get(plugin.source.repository) ?? [];
    group.push(plugin);
    groups.set(plugin.source.repository, group);
  }
  const repositories = [...groups.entries()].sort(
    ([leftRepository, left], [rightRepository, right]) =>
      (right[0]?.discovery?.stars ?? 0) - (left[0]?.discovery?.stars ?? 0) ||
      compareText(leftRepository, rightRepository),
  );
  repositories.forEach(([, group]) => group.sort(compareDiscoveryPackages));
  const ranked: RegistryPlugin[] = [];
  for (let index = 0; ranked.length < plugins.length; index += 1) {
    for (const [, group] of repositories) {
      if (group[index]) ranked.push(group[index]);
    }
  }
  return ranked;
}

function matchQuality(value: string, query: string): number | undefined {
  const candidate = normalized(value);
  if (candidate === query) return 0;
  if (candidate.startsWith(query)) return 1;
  if (candidate.split(/[^\p{L}\p{N}]+/u).some((part) => part.startsWith(query))) return 2;
  if (candidate.includes(query)) return 3;
  return undefined;
}

function textRelevance(plugin: RegistryPlugin, query: string): number {
  if (!query) return 0;
  const fields: Array<[number, string[]]> = [
    [0, [plugin.name, plugin.display_name]],
    [4, [...plugin.keywords, ...plugin.categories, ...plugin.components]],
    [8, [sourceOwner(plugin), plugin.source.repository]],
    [12, [plugin.author.name]],
    [16, [plugin.description]],
  ];
  return Math.min(
    ...fields.flatMap(([weight, values]) =>
      values
        .map((value) => matchQuality(value, query))
        .filter((quality): quality is number => quality !== undefined)
        .map((quality) => weight + quality),
    ),
    Number.MAX_SAFE_INTEGER,
  );
}

export function filterPlugins(
  plugins: RegistryPlugin[],
  filters: CatalogFilters,
): RegistryPlugin[] {
  const query = normalized(filters.query ?? '');
  const eligible = plugins.filter((plugin) => {
    return (
      (!filters.category || plugin.categories.includes(filters.category)) &&
      (!filters.component || plugin.components.includes(filters.component)) &&
      (!filters.source ||
        filters.source === 'all' ||
        plugin.distributions.find((item) => item.id === plugin.default_distribution)?.kind ===
          filters.source) &&
      (!filters.trust ||
        filters.trust === 'all' ||
        (plugin.trust_state ?? 'reviewed') === filters.trust) &&
      (!filters.client ||
        filters.client === 'all' ||
        plugin.client_support.clients.includes(filters.client)) &&
      (!filters.authentication ||
        filters.authentication === 'all' ||
        (filters.authentication === 'none'
          ? plugin.authentication === 'none'
          : plugin.authentication !== 'none')) &&
      (!filters.owner || normalized(sourceOwner(plugin)) === normalized(filters.owner))
    );
  });
  const searchable = (plugin: RegistryPlugin) => [
    plugin.name,
    plugin.display_name,
    plugin.description,
    plugin.author.name,
    plugin.source.repository,
    ...plugin.categories,
    ...plugin.keywords,
    ...plugin.components,
  ];
  let matches = eligible.filter(
    (plugin) => !query || searchable(plugin).some((value) => normalized(value).includes(query)),
  );
  if (query && !matches.length)
    matches = eligible.filter((plugin) =>
      [plugin.name, plugin.display_name, ...plugin.keywords].some((value) =>
        typoMatch(value, query),
      ),
    );
  if (!query) {
    const reviewed = matches
      .filter((plugin) => (plugin.trust_state ?? 'reviewed') === 'reviewed')
      .sort((left, right) =>
        compareText(normalized(left.display_name), normalized(right.display_name)),
      );
    const discovered = diversifiedDiscovery(
      matches.filter((plugin) => plugin.trust_state === 'conformant_unreviewed'),
    );
    return [...reviewed, ...discovered];
  }
  return matches.sort((left, right) => {
    const score = (plugin: RegistryPlugin) => {
      const reviewed = (plugin.trust_state ?? 'reviewed') === 'reviewed';
      const name = normalized(plugin.name);
      if (query && reviewed && name === query) return 0;
      if (query && reviewed && name.startsWith(query)) return 1;
      if (query && !reviewed && name === query) return 2;
      return 3;
    };
    return (
      score(left) - score(right) ||
      textRelevance(left, query) - textRelevance(right, query) ||
      (right.discovery?.stars ?? 0) - (left.discovery?.stars ?? 0) ||
      compareText(left.install_source, right.install_source)
    );
  });
}

export function availableFilters(plugins: RegistryPlugin[]) {
  const categories = [...new Set(plugins.flatMap((plugin) => plugin.categories))].sort();
  const components = [...new Set(plugins.flatMap((plugin) => plugin.components))].sort();
  const owners = [...new Set(plugins.map(sourceOwner))].sort(compareText);
  return { categories, components, owners };
}
