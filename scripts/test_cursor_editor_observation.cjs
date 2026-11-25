'use strict';
// Inert grammar/stop controls; importing the helper cannot launch Electron.
const assert = require('node:assert/strict');
const test = require('node:test');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawnSync } = require('node:child_process');
const { isolationArguments, gate, ordinary, inventoryScreenshot } = require('./cursor_editor_observation.cjs');

const directories = { userData: '/owned/userdata', extensions: '/owned/extensions', project: '/owned/project' };
// Required usage/option rows from pinned 3.30.14 help; this is a grammar
// control, not evidence of an Editor launch or component observation.
const help = 'Cursor 3.30.14\nUsage: cursor [options][paths...]\n\nOptions:\n'
  + '  -n --new-window              Open a new window\n'
  + '  --user-data-dir <dir>        Isolated user data\n'
  + '  --extensions-dir <dir>       Isolated extensions\n';

test('only help-admitted ordinary isolation arguments are emitted', () => {
  assert.deepEqual(isolationArguments(help, directories), ['--new-window', '--user-data-dir',
    '/owned/userdata', '--extensions-dir', '/owned/extensions', '/owned/project']);
});

test('missing, ambiguous and value-taking lookalikes do not admit a launch', () => {
  for (const body of [
    help.replace('  -n --new-window              Open a new window\n', ''),
    help.replace('--user-data-dir <dir>', '--user-data-dir-file <dir>'),
    help.replace('--extensions-dir <dir>', '--extensions-dir <source>'),
    help + '  --user-data-dir <dir>        Second option\n',
    help.replace('Cursor 3.30.14', 'Cursor 3.30.15'),
    help.replace('Usage: cursor [options][paths...]', 'Examples:'),
    help.replace('Usage: cursor [options][paths...]', 'Usage: cursor [options] [paths...]'),
    help + 'Usage: cursor [options][paths...]\n',
    help + '\x00',
    'x'.repeat(65537),
  ]) assert.throws(() => isolationArguments(body, directories), /help-refused/u);
});

test('blocking login/trust/import policy refuses without treating a sign-in button as mandatory', () => {
  for (const [text, expected] of [
    ['Sign in to continue', 'login-gate'],
    ['Log in to continue', 'login-gate'],
    ['Do you trust the authors of these files?', 'trust-gate'],
    ['Local plugin imports are disabled', 'import-policy-gate'],
    ['Blocked by your administrator', 'import-policy-gate'],
    ['Sign in\nCustomize\nSkills', ''],
    ['Project\nCustomize\nSkills\nPlugins', ''],
  ]) assert.equal(gate(text), expected);
});

test('ordinary input paths refuse aliases, hardlinks and incorrect types', () => {
  const root = fs.realpathSync(fs.mkdtempSync(path.join(os.tmpdir(), 'uap-editor-contract-')));
  try {
    const file = path.join(root, 'file'), alias = path.join(root, 'alias'), link = path.join(root, 'link');
    fs.writeFileSync(file, 'inert', { mode: 0o600 });
    assert.equal(ordinary(root, true), root);
    assert.equal(ordinary(file, false), file);
    assert.throws(() => ordinary(root, false), /path-refused/u);
    assert.throws(() => ordinary(file, true), /path-refused/u);
    fs.symlinkSync(file, alias);
    assert.throws(() => ordinary(alias, false), /path-refused/u);
    fs.linkSync(file, link);
    assert.throws(() => ordinary(file, false), /path-refused/u);
    assert.throws(() => ordinary(link, false), /path-refused/u);
  } finally { fs.rmSync(root, { recursive: true, force: true }); }
});

function inventoryPage(options = {}) {
  const events = [], waited = new Set();
  const page = {
    locator(selector) {
      assert.equal(selector, 'body');
      return { async evaluate() {
        events.push('text');
        return { text: options.text ? options.text(events, waited) : 'Skills first second', overflow: false };
      } };
    },
    getByText(name, match) {
      assert.deepEqual(match, { exact: true });
      return { filter(filter) {
        assert.deepEqual(filter, { visible: true });
        return {
          async waitFor(bound) {
            assert.deepEqual(bound, { state: 'visible', timeout: 5000 });
            events.push('wait:' + name);
            if (options.wait) await options.wait(name, events, waited);
            await Promise.resolve(); // Synthetic asynchronous rendering, no real UI.
            waited.add(name);
          },
          async count() {
            events.push('count:' + name);
            return options.count ? options.count(name, events, waited) : Number(waited.has(name));
          },
        };
      } };
    },
    async screenshot(bound) {
      assert.deepEqual(bound, { timeout: 5000 });
      events.push('screenshot');
      return Buffer.from('inert screenshot bytes; not native UI evidence');
    },
  };
  return { page, events };
}

test('inventory capture waits for every marker before facts and screenshot, then rechecks', async () => {
  const root = fs.realpathSync(fs.mkdtempSync(path.join(os.tmpdir(), 'uap-editor-sequence-')));
  try {
    const { page, events } = inventoryPage();
    const result = await inventoryScreenshot(page, root, 'skills.png', ['first', 'second']);
    assert.deepEqual(result.markers, { first: true, second: true });
    assert.deepEqual(events, ['text', 'wait:first', 'wait:second', 'text', 'count:first',
      'count:second', 'screenshot', 'text', 'count:first', 'count:second']);
    assert.equal(result.image.bytes, fs.statSync(path.join(root, 'skills.png')).size);
    assert.equal(result.image.sha256.length, 64);
  } finally { fs.rmSync(root, { recursive: true, force: true }); }
});

test('missing, duplicate, disappearing and gated markers refuse before screenshot', async () => {
  const rendered = new Set();
  const cases = [
    { options: { wait: async () => { throw new Error('private-timeout'); } }, cause: /component-unobserved/u },
    { options: { count: () => 2 }, cause: /component-unobserved/u },
    { options: {
      wait: async name => {
        rendered.add(name);
        if (name === 'second') rendered.delete('first');
      },
      count: name => Number(rendered.has(name)),
    }, cause: /component-unobserved/u },
    { options: { text: () => 'Do you trust the authors?' }, cause: /trust-gate/u },
    { options: { text: (_events, waited) => waited.size ? 'Sign in to continue' : 'Skills' }, cause: /login-gate/u },
    { options: {
      wait: async () => { throw new Error('private-timeout'); },
      text: events => events.includes('wait:first') ? 'Local plugin imports are disabled' : 'Skills',
    }, cause: /import-policy-gate/u },
  ];
  for (const sample of cases) {
    const root = fs.realpathSync(fs.mkdtempSync(path.join(os.tmpdir(), 'uap-editor-refusal-')));
    try {
      const { page, events } = inventoryPage(sample.options);
      await assert.rejects(inventoryScreenshot(page, root, 'skills.png', ['first', 'second']), sample.cause);
      assert.equal(events.includes('screenshot'), false);
      assert.equal(fs.existsSync(path.join(root, 'skills.png')), false);
    } finally { fs.rmSync(root, { recursive: true, force: true }); }
  }
});

test('a marker disappearing during capture cannot produce a positive inventory result', async () => {
  const root = fs.realpathSync(fs.mkdtempSync(path.join(os.tmpdir(), 'uap-editor-drift-')));
  try {
    const { page, events } = inventoryPage({ count: (_name, observed) => Number(!observed.includes('screenshot')) });
    await assert.rejects(inventoryScreenshot(page, root, 'plugins.png', ['first']), /component-unobserved/u);
    assert.equal(events.includes('screenshot'), true);
    assert.equal(fs.existsSync(path.join(root, 'plugins.png')), true);
  } finally { fs.rmSync(root, { recursive: true, force: true }); }
});

const terminalModes = [
  ['caught launch refusal', "throw Error('PRIVATE-CANARY')", 'editor-observation-refused'],
  ['abrupt launch exit', 'process.exit(1)', 'driver-exited-before-report'],
  ['uncaught launch exception', "setImmediate(() => { throw Error('PRIVATE-CANARY'); }); return new Promise(() => {});", 'driver-crashed'],
  ['unhandled launch rejection', "Promise.reject(Error('PRIVATE-CANARY')); return new Promise(() => {});", 'driver-crashed'],
  ['known Electron launch rejection', "Promise.reject(Error('Process failed to launch!')); return new Promise(() => {});", 'electron-launch-failed'],
];
for (const [name, action, observation] of terminalModes) {
  test(`CLI retains one private terminal report after ${name}`, () => {
    const root = fs.realpathSync(fs.mkdtempSync(path.join(os.tmpdir(), 'cursor-terminal-')));
    try {
      for (const directory of ['project', 'userdata', 'extensions', 'output', 'playwright']) {
        fs.mkdirSync(path.join(root, directory));
      }
      fs.writeFileSync(path.join(root, 'electron'), 'inert executable placeholder');
      fs.writeFileSync(path.join(root, 'playwright', 'package.json'),
        JSON.stringify({ version: '1.64.1', main: 'index.js' }));
      // Controlled failure of the library/child boundary, never native proof.
      fs.writeFileSync(path.join(root, 'playwright', 'index.js'),
        `module.exports = { _electron: { launch: async () => { ${action} } } };`);
      const input = {
        root, project: path.join(root, 'project'), userData: path.join(root, 'userdata'),
        extensions: path.join(root, 'extensions'), output: path.join(root, 'output'),
        playwright: path.join(root, 'playwright'), electron: path.join(root, 'electron'), help,
      };
      const result = spawnSync(process.execPath, [path.join(__dirname, 'cursor_editor_observation.cjs')], {
        input: JSON.stringify(input), encoding: 'utf8', timeout: 10000,
      });
      assert.equal(result.error, undefined);
      assert.equal(result.status, 1);
      assert.equal(result.signal, null);
      assert.ok(Buffer.byteLength(result.stdout) <= 65536);
      assert.equal(result.stdout.trim().split('\n').length, 1);
      assert.equal(result.stdout.includes('PRIVATE-CANARY'), false);
      const report = JSON.parse(result.stdout);
      assert.equal(report.status, 'unqualified');
      assert.equal(report.stage, 'editor-launch');
      assert.equal(report.observation, observation);
      for (const key of ['skills_ready', 'model_runtime_qualified',
        'authentication_qualified', 'source_attribution_qualified']) {
        assert.equal(report[key], false);
      }
      if (name.includes('uncaught') || name.includes('rejection')) {
        assert.ok(result.stderr.length > 0, 'default Node crash diagnostics remain enabled');
      }
    } finally {
      fs.rmSync(root, { recursive: true, force: true });
    }
  });
}

test('library import installs no terminal or crash handlers', () => {
  const events = ['exit', 'uncaughtExceptionMonitor', 'uncaughtException', 'unhandledRejection'];
  const result = spawnSync(process.execPath, ['-e', `
    const assert = require('node:assert/strict');
    const events = ${JSON.stringify(events)};
    const before = events.map(event => process.listenerCount(event));
    require(${JSON.stringify(path.join(__dirname, 'cursor_editor_observation.cjs'))});
    assert.deepEqual(events.map(event => process.listenerCount(event)), before);
  `], { encoding: 'utf8', timeout: 10000 });
  assert.equal(result.error, undefined);
  assert.equal(result.status, 0);
  assert.equal(result.stdout, '');
});
