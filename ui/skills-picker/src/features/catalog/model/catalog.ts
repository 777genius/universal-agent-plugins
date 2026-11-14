import type { Candidate } from '../../../shared/api/types';

export function candidateTitle(candidate: Candidate) {
  return candidate.name.trim() || candidate.path.split('/').filter(Boolean).at(-1) || 'Repository root skill';
}
const diagnosticDescriptions: Record<string, string> = {
  skill_name_mismatch: 'The declared skill name does not match its folder name.',
  skill_description_length: 'The skill description exceeds the supported length.',
  skill_description_missing: 'The skill needs a description in its metadata.',
  skill_yaml_invalid: 'The skill metadata could not be read.',
  skill_identity_invalid: 'The skill name is missing or invalid.',
  skill_name_collision: 'Another skill has the same name.',
  skill_source_gate_failed: 'The source did not pass validation.',
  skill_required_field: 'A required skill metadata field is missing.',
  skill_required_type: 'A required skill metadata field has the wrong value type.',
  skill_name_invalid: 'The declared skill name has an unsupported format.',
  skill_frontmatter_open: 'SKILL.md must begin with a metadata block delimited by --- lines.',
  skill_frontmatter_close: 'The skill metadata block is missing its closing --- line.',
  skill_document_too_large: 'SKILL.md exceeds the scan document size limit.',
  skill_document_unavailable: 'The skill document could not be read from this source.',
  skill_document_wrong_kind: 'SKILL.md must be a regular file.',
  skill_aggregate_bytes_limit: 'The scan reached its total document size limit. Try a narrower scan scope.',
};
export function diagnosticDescription(code: string) {
  return diagnosticDescriptions[code] ?? (code.startsWith('yaml_') ? 'The skill metadata contains invalid YAML.' : 'This skill did not pass source validation.');
}

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
    nodes.set(id, { id, name: candidateTitle(candidate), path: candidate.path, children: [], eligible, candidate });
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
