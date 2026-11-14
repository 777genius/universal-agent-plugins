import assert from 'node:assert/strict';
import { test } from 'node:test';
import { createHash } from 'node:crypto';
import { spawnSync } from 'node:child_process';
import { cp, mkdtemp, mkdir, readFile, rm, symlink, writeFile } from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { builtHtmlPath, checkHtml, checkLiveBytes, expectedAgentCommand, getBytes, routePaths, validateContract, prepare, live } from '../scripts/check-production-routes.mjs';
import { parseDirectoryData } from '../utils/registry.ts';
import { projectRegistry } from '../utils/registryProjection.ts';

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
const agentHtml = (route: string, command: string, boundary = '') => html(route, `<code>${command}</code> ${boundary}`);
const contract = () => ({ schema_version: 1, ...identity, routes: routePaths(nativeClients, locales).map(({ route }) => {
  const bytes = html(route);
  return { route, bytes: bytes.length, sha256: createHash('sha256').update(bytes).digest('hex') };
}) });

async function catalogFixture(root: string, empty = false) {
  // Real snapshot bytes are parser fixtures here, not a fabricated signature.
  // Production prepare runs only after the existing authenticated mirror build.
  const snapshot = JSON.parse(await readFile(new URL('../../cli/cmd/agentplugins/directory_bootstrap_inputs/snapshot.json', import.meta.url), 'utf8'));
  if (empty) for (const key of ['products', 'distributions', 'evidence', 'revocations']) snapshot[key] = [];
  for (const directory of ['utils', 'data'])
    await cp(fileURLToPath(new URL(`../${directory}`, import.meta.url)), path.join(root, 'landing', directory), { recursive: true });
  const feed = path.join(root, '.pages-dist/registry/schemas/1');
  await mkdir(path.join(feed, 'snapshots'), { recursive: true });
  const snapshotPath = `snapshots/${String(snapshot.sequence).padStart(20, '0')}.json`;
  await writeFile(path.join(feed, 'latest.json'), JSON.stringify({ snapshot_path: snapshotPath }));
  await writeFile(path.join(feed, snapshotPath), JSON.stringify(snapshot));
  await mkdir(path.join(root, '.pages-dist/api/registry/client'), { recursive: true });
  const registry = parseDirectoryData(snapshot, 'published_snapshot');
  for (const value of ['hermes', 'antigravity', 'pi'] as const)
    await writeFile(path.join(root, '.pages-dist/api/registry/client', value), JSON.stringify(projectRegistry(registry, { kind: 'client', value })));
  if (empty) return (id: string) => `npx -y universal-agent-plugins add ./my-plugin --target ${id}`;
  // Independently select the same reviewed Context7 fixture as browser E2E.
  const plugin = registry.plugins.find(item => item.name === 'context7')!;
  const distribution = plugin.distributions.find(item => item.id === plugin.default_distribution)!;
  const release = distribution.releases.find(item => item.release_sequence === distribution.release_sequence)!;
  assert.equal(plugin.trust_state, 'reviewed');
  assert.equal(release.selectable, true);
  assert.equal(release.release_status, 'active');
  assert.equal(release.source.repository, 'upstash/context7');
  assert.equal(release.source.path, 'plugins/agent-plugins/context7');
  assert.match(release.source.revision!, /^[a-f0-9]{40}$/);
  const selector = `github:${release.source.repository}@${release.source.revision}//${release.source.path}`;
  return (id: string) => `npx -y universal-agent-plugins@0.1.108 add ${selector} --target ${id}`;
}

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
  checkHtml(agentHtml(route, command, boundary), route, 'hermes', command);
  assert.throws(() => checkHtml(agentHtml(route, command), route, 'hermes', command), /MCP boundary/);
  assert.throws(() => checkHtml(agentHtml(route, command.replace('hermes', 'antigravity'), boundary), route, 'hermes', command), /target command/);
});

test('prepared source independently enforces Antigravity Prepared status', () => {
  const route = '/agents/antigravity/';
  const command = 'npx -y universal-agent-plugins add ./my-plugin --target antigravity';
  checkHtml(agentHtml(route, command, 'Prepared by CLI'), route, 'antigravity', command);
  assert.throws(() => checkHtml(agentHtml(route, command, 'Supported'), route, 'antigravity', command), /Prepared boundary/);
});

test('immutable agent commands bind the actual catalog projection and cannot become stale local examples', async () => {
  const root = await mkdtemp(path.join(os.tmpdir(), 'uap-source-routes-'));
  try {
    const expected = await catalogFixture(root);
    for (const id of ['hermes', 'antigravity', 'pi']) {
      const command = await expectedAgentCommand(root, id);
      assert.equal(command, expected(id));
      const route = `/agents/${id}/`;
      const boundary = 'Project installation never modifies User MCP configuration Prepared by CLI';
      checkHtml(agentHtml(route, command, boundary), route, id, command);
      for (const wrong of [command.replace(/@[a-f0-9]{40}\/\//, `@${'a'.repeat(40)}//`),
        command.replace('upstash/context7', 'foreign/package'), command.replace(`--target ${id}`, '--target codex'),
        `npx -y universal-agent-plugins add ./my-plugin --target ${id}`, command + ' --no-save'])
        assert.throws(() => checkHtml(agentHtml(route, wrong, boundary), route, id, command), /target command/);
      assert.throws(() => checkHtml(html(route, command + ' ' + boundary), route, id, command), /target command/);
      assert.throws(() => checkHtml(html(route, `<code>${command}</code><code>${command}</code> ${boundary}`), route, id, command), /target command/);
    }
    const projection = path.join(root, '.pages-dist/api/registry/client/hermes');
    const value = JSON.parse(await readFile(projection, 'utf8'));
    value.plugins = [];
    await writeFile(projection, JSON.stringify(value));
    await assert.rejects(expectedAgentCommand(root, 'hermes'), /projection differs/);
  } finally {
    await rm(root, { recursive: true, force: true });
  }
});

test('only an actually empty eligible catalog selects the local-package fallback', async () => {
  const root = await mkdtemp(path.join(os.tmpdir(), 'uap-empty-routes-'));
  try {
    const expected = await catalogFixture(root, true);
    for (const id of ['hermes', 'antigravity', 'pi'])
      assert.equal(await expectedAgentCommand(root, id), expected(id));
    await rm(path.join(root, '.pages-dist/api/registry/client/hermes'));
    await assert.rejects(expectedAgentCommand(root, 'hermes'), /ENOENT/);
  } finally {
    await rm(root, { recursive: true, force: true });
  }
});

test('contract rejects identity, cardinality, digest and path changes', () => {
  validateContract(contract(), identity);
  const altered = contract();
  altered.source_tree = 'd'.repeat(40);
  assert.throws(() => validateContract(altered, identity), /source_tree/);
  const missing = contract();
  missing.routes.pop();
  assert.throws(() => validateContract(missing, identity), /nine compatibility/);
  const missingHomepage = contract();
  missingHomepage.routes = missingHomepage.routes.filter(entry => entry.route !== '/');
  assert.throws(() => validateContract(missingHomepage, identity), /missing essential live route/);
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
  invalid.routes.find(entry => entry.route.startsWith('/agents/'))!.route = '//outside.example/';
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
    const expectedCommand = await catalogFixture(root);
    await writeFile(path.join(root, 'landing/data/clients.ts'), 'export const clientLandingPages = ' + JSON.stringify(nativeClients) + '; export const clients = clientLandingPages;');
    await writeFile(path.join(root, 'website/tools/config/locales.mjs'), 'export const docsLocales = ' + JSON.stringify(locales));
    await writeFile(path.join(root, 'website/production-source.json'), JSON.stringify({ schema_version: 1, source_commit: identity.source_commit, source_tree: identity.source_tree }));
    await writeFile(path.join(root, '.pages-dist/BUILD_PROVENANCE.json'), JSON.stringify(provenance));
    published.set('/BUILD_PROVENANCE.json', JSON.stringify(provenance));
    for (const entry of routePaths(nativeClients, locales)) {
      let text = 'Hermes Antigravity compatibility';
      if (entry.id === 'hermes') text = `<code>${expectedCommand(entry.id)}</code> Project installation never modifies User MCP configuration`;
      if (entry.id === 'antigravity') text = `<code>${expectedCommand(entry.id)}</code> Prepared by CLI`;
      if (entry.id === 'pi') text = `<code>${expectedCommand(entry.id)}</code>`;
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
    assert.equal(await live(root, contractPath, staleThenCurrent, async ms => { delays.push(ms); }), 27);
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
    const readsBeforeFailure = requested.length;
    await assert.rejects(live(root, contractPath, fetcher, noDelay), /live provenance source_tree/);
    // A persistent provenance mismatch must never request any shipped HTML route.
    assert.deepEqual(requested.slice(readsBeforeFailure), Array(12).fill('/BUILD_PROVENANCE.json'));
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
