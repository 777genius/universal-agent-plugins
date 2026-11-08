import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

// Read only the subset of pnpm v9 YAML needed for graph reachability; fail
// explicitly if the expected importer/snapshot layout changes.
const scalar = (value) => value.replace(/^(['"])(.*)\1$/, '$2');
const lockNode = (name, version) => /^(?:@[^/]+\/)?[^@()]+@\d/.test(version) ? version : `${name}@${version}`;
const entry = (line, indent) => {
  const match = line.match(new RegExp(`^ {${indent}}(\\S[^:]*):(?:\\s+(.*))?$`));
  return match ? [scalar(match[1]), match[2] ?? ''] : null;
};

export function parsePnpmLock(source) {
  const nodes = new Map();
  const roots = { dependencies: new Set(), devDependencies: new Set() };
  let section = '', importer = '', category = '', currentNode = '', edgeCategory = '';
  for (const line of source.split(/\r?\n/)) {
    if (/^(importers|snapshots):\s*$/.test(line)) {
      section = line.slice(0, -1); importer = ''; category = '';
      currentNode = ''; edgeCategory = '';
      continue;
    }
    if (/^[^\s#-]/.test(line) || line === '---') { section = ''; continue; }
    if (section === 'importers') {
      const e2 = entry(line, 2), e4 = entry(line, 4), e6 = entry(line, 6), e8 = entry(line, 8);
      if (e2) { importer = e2[0]; category = ''; }
      else if (e4) category = e4[0];
      else if (importer === '.' && (category === 'dependencies' || category === 'devDependencies' || category === 'optionalDependencies') && e6) currentNode = e6[0];
      else if (importer === '.' && (category === 'dependencies' || category === 'devDependencies' || category === 'optionalDependencies') && e8?.[0] === 'version') roots[category === 'optionalDependencies' ? 'dependencies' : category].add(lockNode(currentNode, scalar(e8[1])));
      continue;
    }
    if (section === 'snapshots') {
      const e2 = entry(line, 2), e4 = entry(line, 4), e6 = entry(line, 6);
      if (e2) { currentNode = e2[0]; nodes.set(currentNode, []); edgeCategory = ''; }
      else if (e4) edgeCategory = e4[0];
      else if (e6 && ['dependencies', 'optionalDependencies'].includes(edgeCategory)) {
        nodes.get(currentNode)?.push(lockNode(e6[0], scalar(e6[1])));
      }
    }
  }
  if (nodes.size === 0 || [...roots.dependencies, ...roots.devDependencies].length === 0) {
    throw new Error('Unsupported pnpm lockfile: missing v9 snapshot graph or root importer');
  }
  return { nodes, roots };
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
  const { nodes, roots } = parsePnpmLock(source);
  const findings = new Map();
  for (const scope of ['dependencies', 'devDependencies']) {
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
  return [...findings.values()].sort((a, b) => a.scope.localeCompare(b.scope) || a.package.localeCompare(b.package));
}

const invoked = process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url);
if (invoked) {
  const lockfile = resolve(process.argv[2] ?? fileURLToPath(new URL('../pnpm-lock.yaml', import.meta.url)));
  const findings = auditLock(readFileSync(lockfile, 'utf8'));
  console.log(JSON.stringify({ lockfile, findings }, null, 2));
  if (findings.length) process.exitCode = 1;
}
