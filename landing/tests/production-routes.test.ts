import assert from 'node:assert/strict';
import { test } from 'node:test';
import { createHash } from 'node:crypto';
import { spawnSync } from 'node:child_process';
import { mkdtemp, mkdir, readFile, rm, symlink, writeFile } from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { builtHtmlPath, checkHtml, checkLiveBytes, getBytes, routePaths, validateContract, prepare, live } from '../scripts/check-production-routes.mjs';

test('CLI invocation through a symlink does not silently skip validation', async t => {
  const root = await mkdtemp(path.join(os.tmpdir(), 'uap-route-cli-'));
  try {
    const entry = path.join(root, 'checker.mjs');
    try {
      await symlink(fileURLToPath(new URL('../scripts/check-production-routes.mjs', import.meta.url)), entry);
    } catch (error) {
      if (process.platform === 'win32' && (error as NodeJS.ErrnoException).code === 'EPERM') {
        t.skip('symlink creation requires Windows privileges');
        return;
      }
      throw error;
    }
    const result = spawnSync(process.execPath, [entry], { encoding: 'utf8' });
    assert.notEqual(result.status, 0);
    assert.match(result.stderr, /usage: checker prepare\|live ROOT CONTRACT/);
  } finally {
    await rm(root, { recursive: true, force: true });
  }
});

test('import ignores a non-file process argument', () => {
  const script = new URL('../scripts/check-production-routes.mjs', import.meta.url).href;
  const result = spawnSync(process.execPath, ['-e', `import(${JSON.stringify(script)})`, 'not-an-existing-file'], { encoding: 'utf8' });
  assert.equal(result.status, 0, result.stderr);
});

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

test('selected public source binds homepage and download with 16-client or 14-client routes', () => {
  const selected = routePaths(nativeClients, locales);
  assert.equal(selected.length, 27);
  assert(selected.some(value => value.route === '/'));
  assert(selected.some(value => value.route === '/download/'));
  assert(selected.some(value => value.route === '/agents/hermes/'));
  const rollback = routePaths(oldClients, locales);
  assert.equal(rollback.length, 25);
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
  const missingDownload = contract();
  missingDownload.routes = missingDownload.routes.filter(entry => entry.route !== '/download/');
  assert.throws(() => validateContract(missingDownload, identity), /missing essential live route/);
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
  const route = '/docs/ar/reference/client-compatibility';
  checkHtml(html(route), route);
  assert.throws(() => checkHtml(html('/'), route), /canonical/);
  const twice = Buffer.from(html(route).toString().replace('</head>', `<link rel="canonical" href="https://universal-agent-plugins.com${route}"></head>`));
  assert.throws(() => checkHtml(twice, route), /canonical/);
});

test('HTTP reader requires actual 200, exact URL, no redirect and bounded nonempty body', async () => {
  const url = 'https://universal-agent-plugins.com/agents/hermes/';
  const response = (status = 200, final = url, redirected = false, body = 'HTML') => {
    const result = new Response(body, { status });
    Object.defineProperties(result, { url: { value: final }, redirected: { value: redirected } });
    return result;
  };
  const fetcher: typeof fetch = async (_url, options) => {
    assert.equal(options?.redirect, 'error');
    return response();
  };
  assert.equal((await getBytes(url, fetcher)).toString(), 'HTML');
  await assert.rejects(getBytes(url, async () => response(404)), /HTTP status/);
  await assert.rejects(getBytes(url, async () => response(200, 'https://universal-agent-plugins.com/')), /final URL/);
  await assert.rejects(getBytes(url, async () => response(200, url, true)), /redirect/);
  await assert.rejects(getBytes(url, async () => response(200, url, false, '')), /empty/);
  await assert.rejects(getBytes(url, async () => response(200, url, false, 'x'.repeat(8 * 1024 * 1024 + 1))), /size limit/);
});

test('live HTML must match complete built bytes, including equal-size stale content', () => {
  const entry = contract().routes[0];
  checkLiveBytes(html(entry.route), entry);
  assert.throws(() => checkLiveBytes(Buffer.from(html(entry.route).toString().replace('<body>', '<BODY>')), entry), /SHA256/);
  assert.throws(() => checkLiveBytes(Buffer.concat([html(entry.route), Buffer.from('stale')]), entry), /byte length/);
});

test('robots and canonical attributes are order independent and case insensitive', () => {
  const route = '/agents/hermes/';
  for (const tag of [
    '<meta content="noindex" name="robots">', '<META CONTENT="NOINDEX, FOLLOW" NAME="ROBOTS">',
    '<meta name=robots content=noindex>', '<meta name="robots" content=noindex>',
    '<meta name=robots content="noindex">', '<meta name=robots content=none>',
  ]) {
    assert.throws(() => checkHtml(Buffer.from(html(route).toString().replace('</head>', tag + '</head>')), route), /noindex/);
  }
  checkHtml(Buffer.from(html(route).toString().replace('link rel="canonical"', 'LINK REL="CANONICAL"')), route);
});

test('encoded and duplicate semantic attributes cannot hide robots or canonical metadata', () => {
  const route = '/agents/hermes/';
  for (const tag of [
    '<meta name="robots" content="no&#105;ndex">', '<meta name="rob&#111;ts" content="noindex">',
    '<meta name="robots" content="noindex" content="index">',
    '<meta name="robots" NAME="other" content="noindex">',
    '<link rel="can&#111;nical" href="https://wrong.example/">',
    '<link rel="canonical" rel="alternate" href="https://wrong.example/">',
  ]) {
    assert.throws(() => checkHtml(Buffer.from(html(route).toString().replace('</head>', tag + '</head>')), route), /encoded|duplicate/);
  }
  const duplicateHref = html(route).toString().replace('rel="canonical"', 'href="https://wrong.example/" rel="canonical"');
  assert.throws(() => checkHtml(Buffer.from(duplicateHref), route), /duplicate/);
  assert.throws(() => checkHtml(Buffer.from(html(route).toString().replace('/agents/hermes/', '/agents/herm&#101;s/')), route), /canonical/);
});

test('HTML metadata consumes complete attributes while allowing ordinary generated values', () => {
  const route = '/agents/hermes/';
  const canonical = html(route).toString().replace('link rel="canonical"', 'link disabled data-note="a > b" rel="alternate canonical"');
  checkHtml(Buffer.from(canonical.replace('</head>', '<meta name=description content="Plugins &amp; skills"><meta name=robots content="index,follow"></head>')), route);
  checkHtml(Buffer.from(html(route).toString().replace('rel="canonical"', 'rel=canonical')), route);
  for (const tag of ['<meta name="robots" content=>', '<meta name="robots" content="noindex>']) {
    assert.throws(() => checkHtml(Buffer.from(html(route).toString().replace('</head>', tag + '</head>')), route), /malformed/);
  }
});

test('prepare and live bind selected modules, built HTML, provenance and environment end to end', async () => {
  const root = await mkdtemp(path.join(os.tmpdir(), 'uap-production-routes-'));
  const envKeys = ['GITHUB_SHA', 'GITHUB_RUN_ID', 'GITHUB_RUN_ATTEMPT'] as const;
  const previous = Object.fromEntries(envKeys.map(key => [key, process.env[key]]));
  const provenance = { ...identity };
  const published = new Map<string, string>();
  try {
    process.env.GITHUB_SHA = identity.workflow_commit;
    process.env.GITHUB_RUN_ID = identity.run_id;
    process.env.GITHUB_RUN_ATTEMPT = identity.run_attempt;
    for (const directory of ['landing/data', 'website/tools/config', '.pages-dist', 'website'])
      await mkdir(path.join(root, directory), { recursive: true });
    await writeFile(path.join(root, 'landing/data/clients.ts'), 'export const clientLandingPages = ' + JSON.stringify(nativeClients));
    await writeFile(path.join(root, 'website/tools/config/locales.mjs'), 'export const docsLocales = ' + JSON.stringify(locales));
    await writeFile(path.join(root, 'website/production-source.json'), JSON.stringify({ schema_version: 1, source_commit: identity.source_commit, source_tree: identity.source_tree }));
    await writeFile(path.join(root, '.pages-dist/BUILD_PROVENANCE.json'), JSON.stringify(provenance));
    published.set('/BUILD_PROVENANCE.json', JSON.stringify(provenance));
    for (const entry of routePaths(nativeClients, locales)) {
      let text = 'Hermes Antigravity compatibility';
      if (entry.id === 'hermes') text = 'npx -y universal-agent-plugins add ./my-plugin --target hermes Project installation never modifies User MCP configuration';
      if (entry.id === 'antigravity') text = 'npx -y universal-agent-plugins add ./my-plugin --target antigravity Prepared by CLI';
      const bytes = html(entry.route, text);
      const relative = builtHtmlPath(entry.route);
      const filename = path.join(root, '.pages-dist', relative);
      await mkdir(path.dirname(filename), { recursive: true });
      await writeFile(filename, bytes);
      published.set(entry.route, bytes.toString());
    }
    const contractPath = path.join(root, 'contract.json');
    await prepare(root, contractPath);
    const actual = JSON.parse(await readFile(contractPath, 'utf8'));
    assert.equal(actual.routes.length, 27);
    const requested: string[] = [];
    const fetcher: typeof fetch = async (input, options) => {
      const url = String(input);
      assert.equal(options?.redirect, 'error');
      const route = new URL(url).pathname;
      requested.push(route);
      const result = new Response(published.get(route) || '', { status: published.has(route) ? 200 : 404 });
      Object.defineProperty(result, 'url', { value: url });
      return result;
    };
    const noDelay = async () => {};
    assert.equal(await live(root, contractPath, fetcher, noDelay), 27);
    assert.equal(requested.length, 28);
    assert(requested.includes('/'));
    assert(requested.includes('/download/'));
    assert(requested.includes('/agents/hermes/'));
    assert(requested.includes('/docs/ar/reference/client-compatibility'));
    // A successful Pages deploy can briefly return the previous run's provenance.
    // The verifier must wait for the exact run, not accept or silently skip it.
    let provenanceReads = 0;
    const staleThenCurrent: typeof fetch = async (input, options) => {
      const url = String(input);
      if (new URL(url).pathname === '/BUILD_PROVENANCE.json' && provenanceReads++ < 2) {
        const response = new Response(JSON.stringify({ ...provenance, run_id: '122' }));
        Object.defineProperty(response, 'url', { value: url });
        return response;
      }
      return fetcher(input, options);
    };
    const delays: number[] = [];
    assert.equal(await live(root, contractPath, staleThenCurrent, async ms => { delays.push(ms); }), 25);
    assert.equal(provenanceReads, 3);
    assert.deepEqual(delays, [10000, 10000]);
    process.env.GITHUB_RUN_ATTEMPT = '3';
    await assert.rejects(live(root, contractPath, fetcher, noDelay), /contract run_attempt/);
    process.env.GITHUB_RUN_ATTEMPT = identity.run_attempt;
    const originalDownload = published.get('/download/')!;
    const staleDownload = originalDownload.replace('<body>', '<BODY>');
    assert.notEqual(staleDownload, originalDownload);
    assert.equal(Buffer.byteLength(staleDownload), Buffer.byteLength(originalDownload));
    published.set('/download/', staleDownload);
    await assert.rejects(live(root, contractPath, fetcher, noDelay), /SHA256/);
    published.set('/download/', originalDownload);
    published.set('/agents/hermes/', published.get('/agents/hermes/')!.replace('Project installation', 'Project INSTALLATION'));
    await assert.rejects(live(root, contractPath, fetcher, noDelay), /SHA256|byte length/);
    published.set('/BUILD_PROVENANCE.json', JSON.stringify({ ...provenance, source_tree: 'd'.repeat(40) }));
    const readsBeforeFailure = requested.filter(value => value === '/BUILD_PROVENANCE.json').length;
    await assert.rejects(live(root, contractPath, fetcher, noDelay), /live provenance source_tree/);
    assert.equal(requested.filter(value => value === '/BUILD_PROVENANCE.json').length - readsBeforeFailure, 12);
  } finally {
    for (const key of envKeys) {
      if (previous[key] === undefined) delete process.env[key];
      else process.env[key] = previous[key];
    }
    await rm(root, { recursive: true, force: true });
  }
});

test('docs canonical public route maps to the actual assembled HTML file', () => {
  const route = '/docs/ar/reference/client-compatibility';
  assert.equal(builtHtmlPath(route), 'docs/ar/reference/client-compatibility.html');
  assert.equal(builtHtmlPath('/agents/hermes/'), 'agents/hermes/index.html');
  assert.equal(builtHtmlPath('/'), 'index.html');
  assert.equal(builtHtmlPath('/download/'), 'download/index.html');
  checkHtml(html(route), route);
  const wrong = Buffer.from(html(route).toString().replace('client-compatibility"', 'client-compatibility.html"'));
  assert.throws(() => checkHtml(wrong, route), /canonical/);
});
