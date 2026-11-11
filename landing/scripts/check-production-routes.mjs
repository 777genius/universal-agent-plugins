import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { readFile, writeFile } from 'node:fs/promises';
import path from 'node:path';
import { pathToFileURL } from 'node:url';

const origin = 'https://universal-agent-plugins.com';
const maxBytes = 8 * 1024 * 1024;
const hash = bytes => createHash('sha256').update(bytes).digest('hex');
const sha = value => typeof value === 'string' && /^[a-f0-9]{40}$/.test(value);
const fields = ['source_commit', 'source_tree', 'workflow_commit', 'run_id', 'run_attempt'];
const normalize = html => html.replace(/<[^>]*>/g, ' ').replace(/&nbsp;|&#160;/g, ' ').replace(/\s+/g, ' ').trim();

export function routePaths(clients, locales) {
  assert(clients.length && locales.length);
  const routes = [
    ...clients.map(({ id, slug }) => {
      assert(/^[a-z0-9-]+$/.test(id) && /^[a-z0-9-]+$/.test(slug));
      return { route: `/agents/${slug}/`, id };
    }),
    ...locales.map(locale => {
      assert(/^[a-z]{2}$/.test(locale));
      return { route: `/docs/${locale}/reference/client-compatibility.html` };
    }),
  ];
  assert.equal(new Set(routes.map(value => value.route)).size, routes.length);
  return routes;
}

export function checkHtml(bytes, route, id) {
  assert(bytes.length > 0 && bytes.length <= maxBytes, `${route}: HTML size`);
  const html = bytes.toString('utf8');
  const links = [...html.matchAll(/<link\b[^>]*>/gi)].map(([tag]) =>
    Object.fromEntries([...tag.matchAll(/([\w-]+)\s*=\s*["']([^"']*)["']/g)].map(([, key, value]) => [key.toLowerCase(), value])));
  assert.deepEqual(links.filter(link => link.rel === 'canonical').map(link => link.href), [origin + route], `${route}: canonical`);
  assert(!/<meta\b[^>]*name=["']robots["'][^>]*content=["'][^"']*noindex/i.test(html), `${route}: noindex`);
  const text = normalize(html);
  if (id === 'hermes' || id === 'antigravity') {
    assert(text.includes(`npx -y universal-agent-plugins add ./my-plugin --target ${id}`), `${id}: local target command`);
    if (id === 'hermes')
      assert(text.includes('Project installation never modifies User MCP configuration'), 'Hermes: Project/User MCP boundary');
    else assert(text.includes('Prepared by CLI'), 'Antigravity: Prepared boundary');
  }
}

export function validateContract(contract, expected) {
  assert.equal(contract.schema_version, 1);
  for (const key of fields) assert.equal(contract[key], expected[key], `contract ${key}`);
  for (const key of ['source_commit', 'source_tree', 'workflow_commit']) assert(sha(contract[key]));
  assert(/^[1-9][0-9]*$/.test(contract.run_id) && /^[1-9][0-9]*$/.test(contract.run_attempt));
  assert(Array.isArray(contract.routes) && contract.routes.length > 9 && contract.routes.length <= 100);
  assert.equal(new Set(contract.routes.map(value => value.route)).size, contract.routes.length);
  assert.equal(contract.routes.filter(value => value.route.startsWith('/docs/')).length, 9, 'nine compatibility locales');
  for (const entry of contract.routes) {
    assert(/^\/(?:agents\/[a-z0-9-]+\/|docs\/[a-z]{2}\/reference\/client-compatibility\.html)$/.test(entry.route));
    assert(/^[a-f0-9]{64}$/.test(entry.sha256));
    assert(Number.isInteger(entry.bytes) && entry.bytes > 0 && entry.bytes <= maxBytes);
  }
}

export async function prepare(root, output) {
  const provenance = JSON.parse(await readFile(path.join(root, '.pages-dist/BUILD_PROVENANCE.json'), 'utf8'));
  const { clientLandingPages } = await import(pathToFileURL(path.join(root, 'landing/data/clients.ts')).href);
  const { docsLocales } = await import(pathToFileURL(path.join(root, 'website/tools/config/locales.mjs')).href);
  const contract = { schema_version: 1, ...Object.fromEntries(fields.map(key => [key, provenance[key]])), routes: [] };
  for (const entry of routePaths(clientLandingPages, docsLocales)) {
    const relative = entry.route.endsWith('/') ? entry.route.slice(1) + 'index.html' : entry.route.slice(1);
    const bytes = await readFile(path.join(root, '.pages-dist', relative));
    checkHtml(bytes, entry.route, entry.id);
    if (entry.route.startsWith('/docs/')) {
      const text = normalize(bytes.toString('utf8'));
      for (const id of ['hermes', 'antigravity']) {
        if (clientLandingPages.some(client => client.id === id))
          assert(text.includes(id === 'hermes' ? 'Hermes' : 'Antigravity'), `${entry.route}: missing ${id} compatibility content`);
      }
    }
    contract.routes.push({ route: entry.route, sha256: hash(bytes), bytes: bytes.length });
  }
  validateContract(contract, environmentIdentity(provenance));
  await writeFile(output, JSON.stringify(contract, null, 2) + '\n');
}

function environmentIdentity(pin) {
  return { source_commit: pin.source_commit, source_tree: pin.source_tree,
    workflow_commit: process.env.GITHUB_SHA, run_id: process.env.GITHUB_RUN_ID,
    run_attempt: process.env.GITHUB_RUN_ATTEMPT };
}

export async function getBytes(url, fetcher = fetch) {
  const response = await fetcher(url, { redirect: 'error', signal: AbortSignal.timeout(30000), headers: { 'accept-encoding': 'identity' } });
  assert.equal(response.status, 200, `${url}: HTTP status`);
  assert.equal(response.url, url, `${url}: final URL`);
  assert(!response.redirected, `${url}: redirect`);
  const chunks = [];
  let size = 0;
  for await (const chunk of response.body) {
    size += chunk.length;
    assert(size <= maxBytes, `${url}: response size limit`);
    chunks.push(Buffer.from(chunk));
  }
  assert(size > 0, `${url}: empty response`);
  return Buffer.concat(chunks);
}

export function checkLiveBytes(bytes, entry) {
  assert.equal(bytes.length, entry.bytes, `${entry.route}: byte length`);
  assert.equal(hash(bytes), entry.sha256, `${entry.route}: built HTML SHA256`);
  checkHtml(bytes, entry.route);
}

export async function live(root, filename, fetcher = fetch) {
  const contract = JSON.parse(await readFile(filename, 'utf8'));
  const pin = JSON.parse(await readFile(path.join(root, 'website/production-source.json'), 'utf8'));
  validateContract(contract, environmentIdentity(pin));
  const provenance = JSON.parse((await getBytes(origin + '/BUILD_PROVENANCE.json', fetcher)).toString('utf8'));
  for (const key of fields) assert.equal(provenance[key], contract[key], `live provenance ${key}`);
  for (const entry of contract.routes) {
    // Retry propagation/network failures, never accept a redirect or stale content.
    let failure;
    for (let attempt = 0; attempt < 6; attempt++) {
      try {
        const bytes = await getBytes(origin + entry.route, fetcher);
        checkLiveBytes(bytes, entry);
        failure = undefined;
        break;
      } catch (error) {
        failure = error;
        if (attempt < 5) await new Promise(resolve => setTimeout(resolve, 5000));
      }
    }
    if (failure) throw failure;
  }
  console.log(`Live selected-source route acceptance passed: ${contract.routes.length} exact HTML routes.`);
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  const [mode, root, filename] = process.argv.slice(2);
  assert(root && filename && ['prepare', 'live'].includes(mode), 'usage: checker prepare|live ROOT CONTRACT');
  await (mode === 'prepare' ? prepare(root, filename) : live(root, filename));
}
