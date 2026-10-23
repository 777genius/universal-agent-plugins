import type { Candidate } from '../../../shared/api/types';

export type CatalogNode = { id: string; name: string; path: string; children: string[]; eligible: string[]; candidate?: Candidate };
export function indexCatalog(candidates: Candidate[]) {
  const nodes = new Map<string, CatalogNode>();
  nodes.set('root', { id: 'root', name: 'Repository', path: '', children: [], eligible: [] });
  for (const candidate of candidates) {
    let parent = nodes.get('root')!;
    let path = '';
    for (const part of candidate.path.split('/').filter(Boolean)) {
      path = path ? `${path}/${part}` : part;
      const id = `folder:${path}`;
      if (!nodes.has(id)) {
        nodes.set(id, { id, name: part, path, children: [], eligible: [] });
        parent.children.push(id);
      }
      parent = nodes.get(id)!;
    }
    const id = `skill:${candidate.id}`;
    const eligible = candidate.status === 'available' ? [candidate.id] : [];
    nodes.set(id, { id, name: candidate.name, path: candidate.path, children: [], eligible, candidate });
    parent.children.push(id);
  }
  function collect(id: string): string[] {
    const node = nodes.get(id)!;
    if (node.candidate) return node.eligible;
    node.children.sort((a, b) => {
      const left = nodes.get(a)!, right = nodes.get(b)!;
      return Number(!!left.candidate) - Number(!!right.candidate) || left.path.localeCompare(right.path) || left.name.localeCompare(right.name);
    });
    node.eligible = node.children.flatMap(collect);
    return node.eligible;
  }
  collect('root');
  return nodes;
}
export function checkedState(eligible: readonly string[], selected: readonly string[]) {
  const count = eligible.filter((id) => selected.includes(id)).length;
  return { checked: eligible.length > 0 && count === eligible.length, mixed: count > 0 && count < eligible.length };
}
export function filterCatalog(candidates: Candidate[], query: string, path: string) {
  const words = query.trim().toLocaleLowerCase().split(/\s+/).filter(Boolean);
  const normalized = query.trim().toLocaleLowerCase();
  const rank = (candidate: Candidate) => candidate.name.toLocaleLowerCase() === normalized ? 0 : candidate.name.toLocaleLowerCase().startsWith(normalized) ? 1 : 2;
  return candidates.filter((candidate) => (!path || candidate.path === path || candidate.path.startsWith(`${path}/`)) &&
    words.every((word) => `${candidate.name} ${candidate.description} ${candidate.path}`.toLocaleLowerCase().includes(word)))
    .sort((a, b) => rank(a) - rank(b) || a.path.localeCompare(b.path));
}
export function ancestors(path: string) {
  const parts = path.split('/').filter(Boolean);
  return parts.map((_, index) => `folder:${parts.slice(0, index + 1).join('/')}`);
}
