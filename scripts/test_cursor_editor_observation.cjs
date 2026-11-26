'use strict';
// Inert grammar/stop controls; importing the helper cannot launch Electron.
const assert = require('node:assert/strict');
const test = require('node:test');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawnSync } = require('node:child_process');
const { isolationArguments, gate, ordinary, inventoryScreenshot, clickNamed, controlFailureScreenshot, startupScreenshot } = require('./cursor_editor_observation.cjs');

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
    ['Cursor’s AI features require you to be logged in', 'login-gate'],
    ["Cursor's AI features require you to be logged in", 'login-gate'],
    ['Do you trust the authors of these files?', 'trust-gate'],
    ['Local plugin imports are disabled', 'import-policy-gate'],
    ['Blocked by your administrator', 'import-policy-gate'],
    ['Sign in\nCustomize\nSkills', ''],
    ['Log In\nSign Up\nCustomize\nSkills', ''],
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
      if (name instanceof RegExp) return { filter(filter) {
        assert.deepEqual(filter, { visible: true });
        return { async count() {
          const text = options.locatorText ? options.locatorText(events, waited) : '';
          return Number(name.test(text));
        } };
      } };
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
        `module.exports = { _electron: { launch: async ({ env }) => {
          if (process.env.DEBUG !== 'pw:browser' || env.DEBUG !== undefined) throw Error('diagnostic-environment-leak');
          ${action}
        } } };`);
      const input = {
        root, project: path.join(root, 'project'), userData: path.join(root, 'userdata'),
        extensions: path.join(root, 'extensions'), output: path.join(root, 'output'),
        playwright: path.join(root, 'playwright'), electron: path.join(root, 'electron'), help,
      };
      const result = spawnSync(process.execPath, [path.join(__dirname, 'cursor_editor_observation.cjs')], {
        input: JSON.stringify(input), encoding: 'utf8', timeout: 10000,
        env: { ...process.env, DEBUG: 'pw:browser' },
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

// Model the locator union's bounded wake-up, not Electron or native UI proof.
function controlPage(states, afterCount) {
  let state = states[0];
  const events = [];
  const target = {
    filter(options) { assert.deepEqual(options, { visible: true }); return this; },
    or(refusal) {
      assert.equal(refusal, blocker);
      return { first() { return { async waitFor(options) {
        assert.deepEqual(options, { state: 'visible', timeout: 5000 });
        events.push('wait');
        for (const next of states) {
          state = next;
          await Promise.resolve(); // Controlled rendering turn, no clock sleep.
          if (state.visible && state.count > 0 || blocker.pattern.test(state.locatorText ?? state.text)) return;
        }
        events.push('deadline');
        throw new Error('private timeout');
      } }; } };
    },
    async count() {
      events.push('count');
      const count = state.visible ? state.count : 0;
      if (afterCount) state = afterCount;
      return count;
    },
    async click(options) {
      assert.deepEqual(options, { timeout: 5000 });
      events.push('click');
    },
  };
  const blocker = {
    filter(options) { assert.deepEqual(options, { visible: true }); return this; },
    async count() { return Number(this.pattern.test(state.locatorText ?? state.text)); },
  };
  const page = {
    locator(selector) {
      assert.equal(selector, 'body');
      return { async evaluate() {
        events.push('text');
        return { text: state.text, overflow: false };
      } };
    },
    getByText(name, options) {
      if (typeof name === 'string') {
        assert.equal(name, 'Customize');
        assert.deepEqual(options, { exact: true });
        return target;
      }
      assert.ok(name instanceof RegExp);
      blocker.pattern = name;
      return blocker;
    },
  };
  return { page, events };
}

const blankControl = { text: '', count: 0, visible: false };
test('observed mandatory Cursor login sentence refuses body and shadow text before click', async () => {
  const ready = { text: 'Customize', count: 1, visible: true };
  for (const sentence of ['Cursor’s AI features require you to be logged in',
    "Cursor's AI features require you to be logged in"]) {
    for (const shadow of [false, true]) {
      const gated = shadow ? { ...ready, locatorText: sentence } : { ...ready, text: sentence };
      for (const afterCount of [false, true]) {
        const { page, events } = controlPage(afterCount ? [ready] : [blankControl, gated],
          afterCount ? gated : undefined);
        await assert.rejects(clickNamed(page, 'Customize'), /login-gate/u);
        assert.equal(events.includes('click'), false);
        assert.equal(events.includes('deadline'), false);
        assert.equal(events.includes('count'), afterCount);
      }
    }
  }
});

test('control click waits for delayed visible rendering and rechecks gates before acting', async () => {
  const { page, events } = controlPage([blankControl,
    { text: 'Customize', count: 1, visible: true }]);
  await clickNamed(page, 'Customize');
  assert.deepEqual(events, ['text', 'wait', 'text', 'count', 'text', 'click']);
});

test('each blocking gate appearing during control wait refuses without clicking', async () => {
  for (const [text, cause] of [
    ['Sign in to continue', /login-gate/u],
    ['Do you trust the authors?', /trust-gate/u],
    ['Local plugin imports are disabled', /import-policy-gate/u],
  ]) {
    const { page, events } = controlPage([blankControl, { text, count: 0, visible: false }]);
    await assert.rejects(clickNamed(page, 'Customize'), cause);
    assert.equal(events.includes('click'), false);
    assert.equal(events.includes('deadline'), false);
  }
});

test('absent and invisible controls exhaust only the finite locator wait without clicking', async () => {
  for (const state of [blankControl, { text: 'Customize', count: 1, visible: false }]) {
    const { page, events } = controlPage([state]);
    await assert.rejects(clickNamed(page, 'Customize'), /ui-control-unobserved/u);
    assert.deepEqual(events, ['text', 'wait', 'deadline', 'text']);
  }
});

test('ambiguous visible controls refuse after readiness without clicking', async () => {
  const { page, events } = controlPage([blankControl,
    { text: 'Customize Customize', count: 2, visible: true }]);
  await assert.rejects(clickNamed(page, 'Customize'), /ui-control-unobserved/u);
  assert.deepEqual(events, ['text', 'wait', 'text', 'count']);
});

test('a gate appearing after uniqueness check refuses immediately before click', async () => {
  const { page, events } = controlPage([
    { text: 'Customize', count: 1, visible: true },
  ], { text: 'Trust this workspace', count: 1, visible: true });
  await assert.rejects(clickNamed(page, 'Customize'), /trust-gate/u);
  assert.deepEqual(events, ['text', 'wait', 'text', 'count', 'text']);
});

test('visible shadow-root gates refuse even when body text omits them and target is unique', async () => {
  for (const [locatorText, cause] of [
    ['Sign in to continue', /login-gate/u],
    ['Do you trust the authors?', /trust-gate/u],
    ['Local plugin imports are disabled', /import-policy-gate/u],
  ]) {
    const { page, events } = controlPage([blankControl,
      { text: 'Customize', locatorText, count: 1, visible: true }]);
    await assert.rejects(clickNamed(page, 'Customize'), cause);
    assert.equal(events.includes('click'), false);
  }
});

test('shadow-root gate appearing after target count refuses before clicking', async () => {
  const { page, events } = controlPage([
    { text: 'Customize', count: 1, visible: true },
  ], { text: 'Customize', locatorText: 'Trust this workspace', count: 1, visible: true });
  await assert.rejects(clickNamed(page, 'Customize'), /trust-gate/u);
  assert.deepEqual(events, ['text', 'wait', 'text', 'count', 'text']);
});

test('control failure capture retains a fresh diagnostic receipt without inventory qualification', async () => {
  const root = fs.realpathSync(fs.mkdtempSync(path.join(os.tmpdir(), 'uap-control-diagnostic-')));
  try {
    const { page, events } = inventoryPage();
    const result = { status: 'unqualified', observation: 'ui-control-unobserved', screenshots: [], skills_ready: false };
    await controlFailureScreenshot(page, root, result);
    assert.equal(result.diagnostic_screenshot.name, 'control-unobserved.png');
    assert.equal(result.diagnostic_screenshot.bytes, fs.statSync(path.join(root, 'control-unobserved.png')).size);
    assert.equal(result.diagnostic_screenshot.sha256.length, 64);
    assert.deepEqual(events, ['screenshot']);
    assert.equal(result.observation, 'ui-control-unobserved');
    assert.equal(result.status, 'unqualified');
    assert.equal(result.skills_ready, false);
    assert.deepEqual(result.screenshots, []);
    const before = fs.readFileSync(path.join(root, 'control-unobserved.png'));
    await controlFailureScreenshot(page, root, result); // wx refuses overwrite.
    assert.deepEqual(fs.readFileSync(path.join(root, 'control-unobserved.png')), before);
    assert.deepEqual(events, ['screenshot']);
  } finally { fs.rmSync(root, { recursive: true, force: true }); }
});

test('bounded failure capture errors preserve the original refusal', async () => {
  const root = fs.realpathSync(fs.mkdtempSync(path.join(os.tmpdir(), 'uap-control-capture-refused-')));
  try {
    let captures = 0;
    const page = { async screenshot(options) {
      assert.deepEqual(options, { timeout: 5000 });
      captures++;
      throw new Error('PRIVATE-CAPTURE-FAILURE');
    } };
    const result = { status: 'unqualified', observation: 'ui-control-unobserved', screenshots: [] };
    await controlFailureScreenshot(page, root, result);
    assert.deepEqual(result, { status: 'unqualified', observation: 'ui-control-unobserved', screenshots: [],
      diagnostic_screenshot_failure: { phase: 'capture', category: 'other' } });
    assert.equal(JSON.stringify(result).includes('PRIVATE-CAPTURE-FAILURE'), false);
    assert.equal(captures, 1);
    await controlFailureScreenshot(page, root, { observation: 'trust-gate' });
    await controlFailureScreenshot(undefined, root, result);
    assert.equal(captures, 1);
  } finally { fs.rmSync(root, { recursive: true, force: true }); }
});

test('startup capture waits for a unique visible Customize control without clicking', async () => {
  const root = fs.realpathSync(fs.mkdtempSync(path.join(os.tmpdir(), 'uap-startup-ready-')));
  try {
    const { page, events } = controlPage([blankControl, { text: 'Customize', count: 1, visible: true }]);
    page.screenshot = async options => {
      assert.deepEqual(options, { timeout: 5000 });
      events.push('screenshot');
      return Buffer.from('inert capture bytes; not native UI evidence');
    };
    const receipt = await startupScreenshot(page, root);
    assert.deepEqual(events, ['text', 'wait', 'text', 'count', 'text', 'screenshot', 'text', 'count', 'text']);
    assert.equal(receipt.name, 'startup.png');
    assert.equal(receipt.bytes, fs.statSync(path.join(root, 'startup.png')).size);
    assert.equal(receipt.sha256.length, 64);
  } finally { fs.rmSync(root, { recursive: true, force: true }); }
});

test('startup gates and unobserved controls refuse before creating an image', async () => {
  const ready = { text: 'Customize', count: 1, visible: true };
  const cases = [
    [blankControl, undefined, /ui-control-unobserved/u],
    [{ text: 'Customize Customize', count: 2, visible: true }, undefined, /ui-control-unobserved/u],
    [{ ...ready, locatorText: 'Sign in to continue' }, undefined, /login-gate/u],
    [ready, { ...ready, locatorText: 'Trust this workspace' }, /trust-gate/u],
    [{ ...ready, locatorText: 'Local plugin imports are disabled' }, undefined, /import-policy-gate/u],
  ];
  for (const [state, afterCount, cause] of cases) {
    const root = fs.realpathSync(fs.mkdtempSync(path.join(os.tmpdir(), 'uap-startup-refused-')));
    try {
      const { page, events } = controlPage([state], afterCount);
      page.screenshot = async () => { throw new Error('screenshot-must-not-run'); };
      await assert.rejects(startupScreenshot(page, root), cause);
      assert.equal(events.includes('click'), false);
      assert.equal(fs.existsSync(path.join(root, 'startup.png')), false);
    } finally { fs.rmSync(root, { recursive: true, force: true }); }
  }
});

test('capture failure retains only its phase and an allowlisted category', async () => {
  for (const [name, code, category] of [
    ['TimeoutError', undefined, 'TimeoutError'],
    ['TargetClosedError', undefined, 'TargetClosedError'],
    ['Error', 'EIO', 'EIO'],
    ['PRIVATE-NAME', 'PRIVATE-CODE', 'other'],
  ]) {
    const root = fs.realpathSync(fs.mkdtempSync(path.join(os.tmpdir(), 'uap-startup-capture-')));
    try {
      const { page } = controlPage([{ text: 'Customize', count: 1, visible: true }]);
      page.screenshot = async () => {
        const error = new Error('PRIVATE-MESSAGE');
        error.name = name;
        error.code = code;
        throw error;
      };
      await assert.rejects(startupScreenshot(page, root), error => {
        assert.equal(error.message, 'screenshot-refused');
        assert.equal(error.phase, 'capture');
        assert.equal(error.category, category);
        assert.equal(JSON.stringify(error).includes('PRIVATE-'), false);
        return true;
      });
      assert.equal(fs.existsSync(path.join(root, 'startup.png')), false);
    } finally { fs.rmSync(root, { recursive: true, force: true }); }
  }
});

test('empty, nonbinary and oversized capture bodies cannot produce receipts', async () => {
  for (const body of [Buffer.alloc(0), 'PRIVATE-NONBINARY', Buffer.alloc((8 << 20) + 1)]) {
    const root = fs.realpathSync(fs.mkdtempSync(path.join(os.tmpdir(), 'uap-startup-bound-')));
    try {
      const { page } = controlPage([{ text: 'Customize', count: 1, visible: true }]);
      page.screenshot = async () => body;
      await assert.rejects(startupScreenshot(page, root), error => {
        assert.equal(error.message, 'screenshot-bound');
        assert.equal(error.phase, 'content');
        assert.equal(error.category, 'screenshot-bound');
        return true;
      });
      assert.equal(fs.existsSync(path.join(root, 'startup.png')), false);
    } finally { fs.rmSync(root, { recursive: true, force: true }); }
  }
});

test('startup capture refuses an existing output without overwriting or capturing', async () => {
  const root = fs.realpathSync(fs.mkdtempSync(path.join(os.tmpdir(), 'uap-startup-existing-')));
  try {
    fs.writeFileSync(path.join(root, 'startup.png'), 'owned sentinel', { mode: 0o600 });
    const { page } = controlPage([{ text: 'Customize', count: 1, visible: true }]);
    page.screenshot = async () => { throw new Error('capture-must-not-run'); };
    await assert.rejects(startupScreenshot(page, root), error => {
      assert.equal(error.message, 'screenshot-refused');
      assert.equal(error.phase, 'open');
      assert.equal(error.category, 'EEXIST');
      return true;
    });
    assert.equal(fs.readFileSync(path.join(root, 'startup.png'), 'utf8'), 'owned sentinel');
  } finally { fs.rmSync(root, { recursive: true, force: true }); }
});

for (const [name, category] of [['TimeoutError', 'TimeoutError'], ['PRIVATE-NAME', 'other']]) {
  test(`inert CLI waits before startup capture and privately reports ${category} with ordinary Quit`, () => {
    const root = fs.realpathSync(fs.mkdtempSync(path.join(os.tmpdir(), 'uap-startup-cli-')));
    try {
      for (const directory of ['project', 'userdata', 'extensions', 'output', 'playwright']) {
        fs.mkdirSync(path.join(root, directory));
      }
      fs.writeFileSync(path.join(root, 'electron'), 'inert executable placeholder');
      fs.writeFileSync(path.join(root, 'playwright', 'package.json'),
        JSON.stringify({ version: '1.64.1', main: 'index.js' }));
      // Controlled library/window boundary; neither this page nor child is native proof.
      fs.writeFileSync(path.join(root, 'playwright', 'index.js'), `
        const { EventEmitter } = require('node:events');
        module.exports = { _electron: { launch: async options => {
          if (options.chromiumSandbox !== true || options.env.DEBUG !== undefined) throw Error('launch-refused');
          let ready = false;
          const child = Object.assign(new EventEmitter(), {
            spawnfile: options.executablePath, spawnargs: [options.executablePath, ...options.args],
          });
          const control = {
            filter() { return this; }, count: async () => Number(ready),
            or() { return { first() { return { async waitFor(bound) {
              if (bound.state !== 'visible' || bound.timeout !== 5000) throw Error('wait-refused');
              await Promise.resolve(); ready = true;
            } }; } }; },
            async click() { throw Error('unexpected-click'); },
          };
          const page = {
            setDefaultTimeout(value) { if (value !== 5000) throw Error('timeout-refused'); },
            locator() { return { async evaluate() { return { text: ready ? 'Customize' : '', overflow: false }; } }; },
            getByText(value) {
              if (typeof value === 'string') {
                if (value !== 'Customize') throw Error('control-refused');
                return control;
              }
              return { filter() { return this; }, count: async () => 0 };
            },
            async screenshot(bound) {
              if (!ready || bound.timeout !== 5000) throw Error('PRIVATE-UNREADY');
              const error = Error('PRIVATE-MESSAGE'); error.name = ${JSON.stringify(name)}; throw error;
            },
          };
          return {
            process: () => child,
            firstWindow: async () => page,
            evaluate: async fn => fn({ app: { quit() { child.emit('close', 0, null); } } }),
          };
        } } };
      `);
      const input = {
        root, project: path.join(root, 'project'), userData: path.join(root, 'userdata'),
        extensions: path.join(root, 'extensions'), output: path.join(root, 'output'),
        playwright: path.join(root, 'playwright'), electron: path.join(root, 'electron'), help,
      };
      const run = spawnSync(process.execPath, [path.join(__dirname, 'cursor_editor_observation.cjs')], {
        input: JSON.stringify(input), encoding: 'utf8', timeout: 10000,
        env: { ...process.env, DEBUG: 'pw:browser' },
      });
      assert.equal(run.error, undefined);
      assert.equal(run.status, 1);
      assert.equal(run.signal, null);
      assert.equal(run.stdout.trim().split('\n').length, 1);
      assert.ok(Buffer.byteLength(run.stdout) <= 65536);
      assert.equal(run.stdout.includes('PRIVATE-'), false);
      const report = JSON.parse(run.stdout);
      assert.equal(report.status, 'unqualified');
      assert.equal(report.stage, 'startup');
      assert.equal(report.observation, 'screenshot-refused');
      assert.deepEqual(report.screenshot_failure, { phase: 'capture', category });
      assert.deepEqual(report.screenshots, []);
      assert.equal(report.quit_requested, true);
      assert.equal(report.quit_acknowledged, true);
      assert.deepEqual(report.native_exit, { code: 0, signal: null });
      for (const key of ['skills_ready', 'model_runtime_qualified', 'authentication_qualified', 'source_attribution_qualified']) {
        assert.equal(report[key], false);
      }
    } finally { fs.rmSync(root, { recursive: true, force: true }); }
  });
}

test('inventory refuses shadow-root gates before capture, after failed wait and after capture', async () => {
  for (const when of ['initial', 'wait-failure', 'capture']) {
    const root = fs.realpathSync(fs.mkdtempSync(path.join(os.tmpdir(), 'uap-inventory-shadow-')));
    try {
      const { page, events } = inventoryPage({
        locatorText: events => when === 'initial'
          || when === 'wait-failure' && events.includes('wait:first')
          || when === 'capture' && events.includes('screenshot')
          ? 'Cursor’s AI features require you to be logged in' : '',
        wait: async () => { if (when === 'wait-failure') throw Error('private-timeout'); },
      });
      await assert.rejects(inventoryScreenshot(page, root, 'skills.png', ['first']), /login-gate/u);
      assert.equal(events.includes('screenshot'), when === 'capture');
    } finally { fs.rmSync(root, { recursive: true, force: true }); }
  }
});

test('startup refuses gates and nonunique controls appearing during capture', async () => {
  for (const change of [
    { text: 'Cursor’s AI features require you to be logged in' },
    { locatorText: 'Trust this workspace' },
    { count: 0, visible: false },
    { count: 2 },
  ]) {
    const root = fs.realpathSync(fs.mkdtempSync(path.join(os.tmpdir(), 'uap-startup-drift-')));
    try {
      const ready = { text: 'Customize', count: 1, visible: true };
      const { page, events } = controlPage([ready]);
      page.screenshot = async () => {
        Object.assign(ready, change);
        return Buffer.from('inert captured bytes');
      };
      await assert.rejects(startupScreenshot(page, root),
        change.text ? /login-gate/u : change.locatorText ? /trust-gate/u : /ui-control-unobserved/u);
      assert.equal(events.includes('click'), false);
    } finally { fs.rmSync(root, { recursive: true, force: true }); }
  }
});

test('capture refuses output raced in during capture without overwriting it', async () => {
  const root = fs.realpathSync(fs.mkdtempSync(path.join(os.tmpdir(), 'uap-capture-race-')));
  try {
    const { page } = controlPage([{ text: 'Customize', count: 1, visible: true }]);
    page.screenshot = async () => {
      fs.writeFileSync(path.join(root, 'startup.png'), 'raced sentinel', { flag: 'wx', mode: 0o600 });
      return Buffer.from('inert capture bytes');
    };
    await assert.rejects(startupScreenshot(page, root), error => {
      assert.equal(error.phase, 'open');
      assert.equal(error.category, 'EEXIST');
      return true;
    });
    assert.equal(fs.readFileSync(path.join(root, 'startup.png'), 'utf8'), 'raced sentinel');
  } finally { fs.rmSync(root, { recursive: true, force: true }); }
});
