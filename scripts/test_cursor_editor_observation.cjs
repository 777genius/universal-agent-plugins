'use strict';
// Inert grammar/stop controls; importing the helper cannot launch Electron.
const assert = require('node:assert/strict');
const test = require('node:test');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { isolationArguments, gate, ordinary } = require('./cursor_editor_observation.cjs');

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
