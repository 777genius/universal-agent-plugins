import { readFile } from 'node:fs/promises';
import { pathToFileURL } from 'node:url';

const levels = new Set(['unsupported', 'prepared', 'projected', 'native']);
function supportSnapshot(rows) {
  if (!Array.isArray(rows) || !rows.length) throw new Error('Invalid capability snapshot');
  const clients = new Set();
  const fields = new Map();
  const add = (key, value) => {
    if (!levels.has(value)) throw new Error(`Invalid support field: ${key}`);
    fields.set(key, value);
  };
  for (const row of rows) {
    if (!row || typeof row.id !== 'string' || !/^[a-z][a-z0-9-]*$/.test(row.id) || clients.has(row.id))
      throw new Error('Invalid or duplicate client identity');
    clients.add(row.id);
    for (const [scope, value] of [['user', row], ['project', row.project]]) {
      if (scope === 'project' && value === undefined) continue;
      if (!value || typeof value !== 'object') throw new Error('Invalid scoped capabilities');
      for (const field of scope === 'user' ? ['skill_support', 'app_support', 'extension_support'] : ['skill_support', 'extension_support'])
        add(`${row.id}/${scope}/${field}`, value[field]);
      const transports = value.mcp_transports;
      if (!transports || typeof transports !== 'object' || Array.isArray(transports) ||
          Object.keys(transports).sort().join(',') !== 'sse,stdio,streamable-http') throw new Error('Invalid MCP transports');
      for (const [transport, level] of Object.entries(transports)) add(`${row.id}/${scope}/mcp/${transport}`, level);
    }
  }
  return fields;
}

export function requireReleasedCapabilities(source, released) {
  const advertised = supportSnapshot(source), available = supportSnapshot(released);
  for (const [key, level] of advertised) {
    if (level === 'unsupported') continue;
    const actual = available.get(key);
    // Delivery modes are distinct contracts; native delivery also covers a
    // projected/prepared component, but weaker modes cannot justify native.
    if (actual !== level && actual !== 'native') throw new Error(`Published CLI lacks ${key} (${level})`);
  }
}

export function stableVersion(version) {
  if (typeof version !== 'string' || !/^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$/.test(version))
    throw new Error('npm latest must be a stable semantic version');
  return version;
}

async function json(url) {
  const response = await fetch(url, { redirect: 'error', signal: AbortSignal.timeout(30000) });
  if (!response.ok) throw new Error(`Release capability fetch failed for ${url}: ${response.status}`);
  const body = await response.text();
  if (body.length > 2 * 1024 * 1024) throw new Error('Release metadata exceeds limit');
  return JSON.parse(body);
}

async function main() {
  const version = stableVersion((await json('https://registry.npmjs.org/universal-agent-plugins/latest')).version);
  const released = await json(`https://raw.githubusercontent.com/777genius/universal-agent-plugins/agentplugins-v${version}/landing/data/clientCapabilities.generated.json`);
  const source = JSON.parse(await readFile(new URL('../data/clientCapabilities.generated.json', import.meta.url), 'utf8'));
  requireReleasedCapabilities(source, released);
  console.log(`Published npm ${version} covers advertised source capabilities`);
}
if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  main().catch(error => { console.error(error.message); process.exitCode = 1; });
}
