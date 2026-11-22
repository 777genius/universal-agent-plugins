'use strict';
// Inert grammar/stop controls; importing the helper cannot launch Electron.
const assert = require('node:assert/strict');
const test = require('node:test');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { isolationArguments, gate, ordinary, inventoryScreenshot } = require('./cursor_editor_observation.cjs');

const directories = { userData: '/owned/userdata', extensions: '/owned/extensions', project: '/owned/project' };
// A synthetic usage control only: actual 3.30.14 help must admit these rows.
const help = 'Cursor 3.30.14\nUsage: cursor [options] [paths...]\n\nOptions:\n'
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
    help.replace('Usage: cursor [options] [paths...]', 'Examples:'),
    help + 'Usage: cursor [options] [paths...]\n',
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
