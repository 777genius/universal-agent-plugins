import assert from 'node:assert/strict';
import { test } from 'node:test';
import { createHash } from 'node:crypto';
import { checkHtml, checkLiveBytes, getBytes, routePaths, validateContract } from '../scripts/check-production-routes.mjs';

const locales = ['en', 'ru', 'uk', 'zh', 'es', 'hi', 'ar', 'pt', 'fr'];
// Public catalogue of accepted source 55c2b820; H/A were not advertised there.
const oldClients = [
  ['codex', 'codex'], ['chatgpt', 'chatgpt'], ['cursor', 'cursor'],
  ['copilot', 'github-copilot-cli'], ['kiro', 'kiro'], ['claude', 'claude-code'],
  ['gemini', 'gemini-cli'], ['opencode', 'opencode'], ['cline', 'cline'],
  ['windsurf', 'windsurf'], ['grok', 'grok-build'], ['kimi', 'kimi-code'],
  ['qoder', 'qoder'], ['pi', 'pi'],
].map(([id, slug]) => ({ id, slug }));
const nativeClients = [...oldClients, { id: 'hermes', slug: 'hermes' }, { id: 'antigravity', slug: 'antigravity' }];
const identity = { source_commit: 'a'.repeat(40), source_tree: 'b'.repeat(40), workflow_commit: 'c'.repeat(40), run_id: '123', run_attempt: '2' };
const html = (route: string, text = '') => Buffer.from(`<html><head><link rel="canonical" href="https://universal-agent-plugins.com${route}"></head><body>${text}</body></html>`);
const contract = () => ({ schema_version: 1, ...identity, routes: routePaths(nativeClients, locales).map(({ route }) => {
  const bytes = html(route);
  return { route, bytes: bytes.length, sha256: createHash('sha256').update(bytes).digest('hex') };
}) });

test('selected public source determines 16 routes or legitimate 14-client rollback without H/A', () => {
  const selected = routePaths(nativeClients, locales);
  assert.equal(selected.length, 25);
  assert(selected.some(value => value.route === '/agents/hermes/'));
  const rollback = routePaths(oldClients, locales);
  assert.equal(rollback.length, 23);
  assert(!rollback.some(value => /hermes|antigravity/.test(value.route)));
  assert.equal(rollback.filter(value => value.route.startsWith('/docs/')).length, 9);
  assert.throws(() => routePaths([...oldClients, oldClients[0]], locales));
  assert.throws(() => routePaths([{ id: 'bad', slug: '../bad' }], locales));
});

test('prepared source independently enforces Hermes Project/User MCP boundary and exact target command', () => {
  const route = '/agents/hermes/';
  const command = 'npx -y universal-agent-plugins add ./my-plugin --target hermes';
  const boundary = 'Project installation never modifies User MCP configuration';
  checkHtml(html(route, command + ' ' + boundary), route, 'hermes');
  assert.throws(() => checkHtml(html(route, command), route, 'hermes'), /MCP boundary/);
  assert.throws(() => checkHtml(html(route, command.replace('hermes', 'antigravity') + boundary), route, 'hermes'), /target command/);
});

test('prepared source independently enforces Antigravity Prepared status', () => {
  const route = '/agents/antigravity/';
  const command = 'npx -y universal-agent-plugins add ./my-plugin --target antigravity';
  checkHtml(html(route, command + ' Prepared by CLI'), route, 'antigravity');
  assert.throws(() => checkHtml(html(route, command + ' Supported'), route, 'antigravity'), /Prepared boundary/);
});

test('contract rejects identity, cardinality, digest and path changes', () => {
  validateContract(contract(), identity);
  const altered = contract();
  altered.source_tree = 'd'.repeat(40);
  assert.throws(() => validateContract(altered, identity), /source_tree/);
  const missing = contract();
  missing.routes.pop();
  assert.throws(() => validateContract(missing, identity), /nine compatibility/);
  const duplicate = contract();
  duplicate.routes[1] = duplicate.routes[0];
  assert.throws(() => validateContract(duplicate, identity));
  const digest = contract();
  digest.routes[0].sha256 = 'not-a-digest';
  assert.throws(() => validateContract(digest, identity));
  const invalid = contract();
  invalid.routes[0].route = '//outside.example/';
  assert.throws(() => validateContract(invalid, identity));
});

test('HTML canonical must identify the requested route uniquely', () => {
  const route = '/docs/ar/reference/client-compatibility.html';
  checkHtml(html(route), route);
  assert.throws(() => checkHtml(html('/'), route), /canonical/);
  const twice = Buffer.from(html(route).toString().replace('</head>', `<link rel="canonical" href="https://universal-agent-plugins.com${route}"></head>`));
  assert.throws(() => checkHtml(twice, route), /canonical/);
});

test('HTTP reader requires actual 200, exact URL, no redirect and bounded nonempty body', async () => {
  const url = 'https://universal-agent-plugins.com/agents/hermes/';
  const response = (status = 200, final = url, redirected = false, body = Buffer.from('HTML')) => ({
    status, url: final, redirected, body: (async function* () { yield body; })(),
  });
  const fetcher = async (_url: string, options: { redirect: string }) => {
    assert.equal(options.redirect, 'error');
    return response();
  };
  assert.equal((await getBytes(url, fetcher)).toString(), 'HTML');
  await assert.rejects(getBytes(url, async () => response(404)), /HTTP status/);
  await assert.rejects(getBytes(url, async () => response(200, 'https://universal-agent-plugins.com/')), /final URL/);
  await assert.rejects(getBytes(url, async () => response(200, url, true)), /redirect/);
  await assert.rejects(getBytes(url, async () => response(200, url, false, Buffer.alloc(0))), /empty/);
  await assert.rejects(getBytes(url, async () => response(200, url, false, Buffer.alloc(8 * 1024 * 1024 + 1))), /size limit/);
});

test('live HTML must match complete built bytes, including equal-size stale content', () => {
  const entry = contract().routes[0];
  checkLiveBytes(html(entry.route), entry);
  assert.throws(() => checkLiveBytes(Buffer.from(html(entry.route).toString().replace('<body>', '<BODY>')), entry), /SHA256/);
  assert.throws(() => checkLiveBytes(Buffer.concat([html(entry.route), Buffer.from('stale')]), entry), /byte length/);
});
