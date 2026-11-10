import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

// Read only the subset of pnpm v9 YAML needed for graph reachability; fail
// explicitly if the expected importer/snapshot layout changes.
const scalar = (value) => {
  const quoted = value.match(/^(['"])(.*)\1$/);
  if (!quoted) return value;
  // YAML decodes backslash escapes in double quotes; do not silently audit a different package name.
  if (quoted[1] === '"' && quoted[2].includes('\\')) throw new Error('Unsupported escaped YAML scalar');
  return quoted[2];
};
const lockNode = (name, version) => /^(?:@[^/]+\/)?[^@()]+@\d/.test(version) ? version : `${name}@${version}`;
const rejectInlineMap = ([name, value]) => {
  if (value && value !== '{}') throw new Error(`Unsupported inline pnpm map: ${name}`);
};
const entry = (line, indent) => {
  const match = line.match(new RegExp(`^ {${indent}}(\\S[^:]*?) *:(?:\\s+(.*))?$`));
  return match ? [scalar(match[1]), match[2] ?? ''] : null;
};

const SCOPES = ['dependencies', 'devDependencies', 'configDependencies', 'packageManagerDependencies'];
const ROOT_CATEGORIES = [...SCOPES, 'optionalDependencies'];

function parsePnpmDocument(source) {
  const versions = [...source.matchAll(/^lockfileVersion:\s*(\S+)\s*$/gm)].map(([, value]) => scalar(value));
  if (versions.length === 0 || versions.some((version) => version !== '9.0')) {
    throw new Error('Unsupported pnpm lockfile version');
  }
  const nodes = new Map();
  const roots = Object.fromEntries(SCOPES.map((scope) => [scope, new Set()]));
  let section = '', importer = '', category = '', currentNode = '', edgeCategory = '';
  let pendingRoot = '';
  const requireResolvedRoot = () => {
    if (pendingRoot) throw new Error(`Missing pnpm importer version: ${pendingRoot}`);
  };
  for (const line of source.split(/\r?\n/)) {
    if (/^(importers|snapshots):\s*$/.test(line)) {
      requireResolvedRoot();
      section = line.slice(0, -1); importer = ''; category = '';
      currentNode = ''; edgeCategory = '';
      continue;
    }
    if (/^[^\s#-]/.test(line) || line === '---') { requireResolvedRoot(); section = ''; continue; }
    // pnpm uses fixed-depth mappings here. A YAML-valid alternative indentation
    // must not silently drop dependency edges from this deliberately narrow parser.
    if (/^ +[^#\s]/.test(line)) {
      const indent = line.search(/\S/);
      const depths = section === 'importers' ? [2, 4, 6, 8] : section === 'snapshots' ? [2, 4, 6] : null;
      if (depths && !depths.includes(indent)) throw new Error(`Unsupported pnpm indentation in ${section}`);
    }
    // YAML explicit mapping keys are valid but are not interpreted by this narrow reader.
    // Never return a clean audit when one can conceal dependency edges.
    if (section && /^ +[?:](?:\s|$)/.test(line)) throw new Error('Unsupported explicit pnpm YAML mapping');
    // Reject a sequence where pnpm requires a dependency mapping; otherwise
    // the scanner can silently miss the only vulnerable edge.
    if (/^ {6}-(?:\s|$)/.test(line) && (
      (section === 'importers' && ROOT_CATEGORIES.includes(category)) ||
      (section === 'snapshots' && ['dependencies', 'optionalDependencies'].includes(edgeCategory))
    )) throw new Error('Unsupported pnpm dependency sequence');
    if (section === 'importers') {
      const e2 = entry(line, 2), e4 = entry(line, 4), e6 = entry(line, 6), e8 = entry(line, 8);
      if (e2 || e4 || e6) requireResolvedRoot();
      if (e2) { rejectInlineMap(e2); importer = e2[0]; category = ''; }
      else if (e4) {
        if (ROOT_CATEGORIES.includes(e4[0])) rejectInlineMap(e4);
        category = e4[0];
      }
      else if (importer && ROOT_CATEGORIES.includes(category) && e6) {
        currentNode = e6[0]; pendingRoot = currentNode;
      }
      else if (importer && ROOT_CATEGORIES.includes(category) && e8?.[0] === 'version') {
        roots[category === 'optionalDependencies' ? 'dependencies' : category].add(lockNode(currentNode, scalar(e8[1])));
        pendingRoot = '';
      }
      continue;
    }
    if (section === 'snapshots') {
      const e2 = entry(line, 2), e4 = entry(line, 4), e6 = entry(line, 6);
      if (e2) {
        rejectInlineMap(e2);
        currentNode = e2[0];
        if (nodes.has(currentNode)) throw new Error(`Duplicate pnpm snapshot: ${currentNode}`);
        nodes.set(currentNode, []);
        edgeCategory = '';
      }
      else if (e4) {
        if (['dependencies', 'optionalDependencies'].includes(e4[0])) rejectInlineMap(e4);
        edgeCategory = e4[0];
      }
      else if (e6 && ['dependencies', 'optionalDependencies'].includes(edgeCategory)) {
        nodes.get(currentNode)?.push(lockNode(e6[0], scalar(e6[1])));
      }
    }
  }
  requireResolvedRoot();
  return { nodes, roots };
}

export function parsePnpmLock(source) {
  // pnpm 12 can emit a separate boot document for config and package-manager dependencies.
  // Keep document graphs isolated: identical snapshot IDs can have different edges.
  const documents = source.split(/^---\s*$/gm).filter((part) => part.trim()).map(parsePnpmDocument);
  if (documents.length === 0 || documents.length > 2 || !documents.some(({ nodes, roots }) =>
    nodes.size > 0 && (roots.dependencies.size > 0 || roots.devDependencies.size > 0))) {
    throw new Error('Unsupported pnpm lockfile: missing v9 snapshot graph or root importer');
  }
  return documents;
}

// These exact advisory ranges, not arbitrary versions of the packages.
export function vulnerabilityFor(key) {
  const match = key.match(/^(glob|minimatch)@(\d+)\.(\d+)\.(\d+)(?:\(|$)/);
  if (!match) {
    if (/^(glob|minimatch)@/.test(key)) throw new Error(`Unsupported advisory package version: ${key}`);
    return null;
  }
  const [, name, major, minor, patch] = match;
  const [a, b, c] = [major, minor, patch].map(Number);
  if (name === 'glob' && a >= 9 && (a < 14 || (a === 14 && (b === 0 && c < 2)))) return 'GHSA-3g3m-2fjr-6pw4';
  if (name === 'minimatch' && a >= 7 && (a < 11 || (a === 11 && b === 0 && c < 1))) return 'GHSA-g3hv-75wg-m249';
  return null;
}

export function auditLock(source) {
  const documents = parsePnpmLock(source);
  const findings = new Map();
  for (const { nodes, roots } of documents) {
    for (const scope of SCOPES) {
    const pending = [...roots[scope]].map((key) => [key, [key]]);
    const visited = new Set();
    for (let i = 0; i < pending.length; i++) {
      const [key, path] = pending[i];
      if (visited.has(key)) continue;
      if (!nodes.has(key)) throw new Error(`Unresolved pnpm dependency edge: ${key}`);
      visited.add(key);
      if (vulnerabilityFor(key)) {
        const id = `${scope}:${key}`;
        findings.set(id, { scope, package: key, advisory: vulnerabilityFor(key), path });
      }
      for (const child of nodes.get(key) ?? []) pending.push([child, [...path, child]]);
    }
  }
  }
  return [...findings.values()].sort((a, b) => a.scope.localeCompare(b.scope) || a.package.localeCompare(b.package));
}

const invoked = process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url);
if (invoked) {
  const lockfile = resolve(process.argv[2] ?? fileURLToPath(new URL('../pnpm-lock.yaml', import.meta.url)));
  const findings = auditLock(readFileSync(lockfile, 'utf8'));
  console.log(JSON.stringify({ lockfile, findings }, null, 2));
  if (findings.length) process.exitCode = 1;
}
