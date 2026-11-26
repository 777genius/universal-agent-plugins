'use strict';
// Proof-only Cursor Editor UI observation. No chat, model, MCP or policy changes.
const fs = require('node:fs');
const path = require('node:path');
const { once } = require('node:events');
const { createHash } = require('node:crypto');

function isolationArguments(help, directories) {
  if (typeof help !== 'string' || help.length > 65536 || /[\x00-\x08\x0b-\x1f\x7f]/u.test(help)) throw new Error('help-refused');
  const rows = help.split(/\r?\n/u);
  if (rows[0] !== 'Cursor 3.30.14'
      || rows.filter(row => row.startsWith('Usage:')).length !== 1
      || !rows.includes('Usage: cursor [options][paths...]')) throw new Error('help-refused');
  for (const pattern of [
    /^\s+(?:-n[ ,]+)?--new-window\s{2,}\S.*$/u,
    /^\s+--user-data-dir <dir>\s{2,}\S.*$/u,
    /^\s+--extensions-dir <dir>\s{2,}\S.*$/u,
  ]) if (rows.filter(row => pattern.test(row)).length !== 1) throw new Error('help-refused');
  return ['--new-window', '--user-data-dir', directories.userData,
    '--extensions-dir', directories.extensions, directories.project];
}

const gateRules = [
  [/sign in to (?:continue|use cursor)|log in to continue|Cursor['’]s AI features require you to be logged in/iu, 'login-gate'],
  [/do you trust the authors|trust this (?:workspace|project)/iu, 'trust-gate'],
  [/local plugin imports (?:are )?(?:disabled|not allowed)|blocked by (?:your )?(?:organization|administrator)/iu, 'import-policy-gate'],
];
const gatePattern = new RegExp(gateRules.map(([pattern]) => `(?:${pattern.source})`).join('|'), 'iu');

function gate(text) {
  return gateRules.find(([pattern]) => pattern.test(text))?.[1] || '';
}

async function bounded(promise, milliseconds) {
  let timer;
  try {
    return await Promise.race([promise, new Promise((_, reject) => {
      timer = setTimeout(() => reject(new Error('phase-deadline')), milliseconds);
    })]);
  } finally { clearTimeout(timer); }
}

function ordinary(value, directory) {
  if (typeof value !== 'string' || !path.isAbsolute(value) || path.resolve(value) !== value
      || fs.realpathSync(value) !== value) throw new Error('path-refused');
  const info = fs.lstatSync(value);
  if (directory ? !info.isDirectory() : !info.isFile() || info.nlink !== 1) throw new Error('path-refused');
  return value;
}

class ScreenshotFailure extends Error {
  constructor(error, phase) {
    const message = error instanceof Error ? error.message : '';
    const known = ['path-refused', 'screenshot-bound', 'screenshot-custody'];
    super(known.includes(message) ? message : 'screenshot-refused');
    this.phase = phase;
    const name = error instanceof Error ? error.name : '';
    const code = error instanceof Error ? error.code : '';
    this.category = ['TimeoutError', 'TargetClosedError'].includes(name) ? name
      : ['EACCES', 'EPERM', 'ENOENT', 'EEXIST', 'ENOSPC', 'EIO', 'EBADF', 'EMFILE', 'ENFILE'].includes(code) ? code
        : known.includes(message) ? message : 'other';
  }
}

async function screenshot(page, directory, name) {
  let phase = 'path';
  try {
    ordinary(directory, true);
    const parentBefore = fs.lstatSync(directory);
    const destination = path.join(directory, name);
    phase = 'open';
    try {
      fs.lstatSync(destination);
      throw Object.assign(new Error('screenshot-refused'), { code: 'EEXIST' });
    } catch (error) {
      if (error.code !== 'ENOENT') throw error;
    }
    phase = 'capture';
    const body = await page.screenshot({ timeout: 5000 });
    phase = 'content';
    if (!Buffer.isBuffer(body) || body.length === 0 || body.length > 8 << 20) throw new Error('screenshot-bound');
    phase = 'custody';
    const parentCaptured = fs.lstatSync(directory);
    if (parentBefore.ino !== parentCaptured.ino || parentBefore.dev !== parentCaptured.dev
        || fs.realpathSync(directory) !== directory) throw new Error('screenshot-custody');
    phase = 'open';
    const descriptor = fs.openSync(destination, 'wx', 0o600);
    try {
      phase = 'file-check';
      const before = fs.fstatSync(descriptor);
      phase = 'write';
      fs.writeFileSync(descriptor, body);
      phase = 'custody';
      const held = fs.fstatSync(descriptor), current = fs.lstatSync(destination);
      const parentAfter = fs.lstatSync(directory);
      if (before.ino !== held.ino || held.ino !== current.ino || held.dev !== current.dev
          || before.mode !== held.mode || held.mode !== current.mode || held.uid !== current.uid
          || !current.isFile() || current.nlink !== 1 || current.size !== body.length
          || parentBefore.ino !== parentAfter.ino || parentBefore.dev !== parentAfter.dev
          || fs.realpathSync(directory) !== directory) throw new Error('screenshot-custody');
      return { name, bytes: body.length, sha256: createHash('sha256').update(body).digest('hex') };
    } catch (error) {
      throw new ScreenshotFailure(error, phase);
    } finally {
      phase = 'close';
      fs.closeSync(descriptor);
    }
  } catch (error) {
    throw error instanceof ScreenshotFailure ? error : new ScreenshotFailure(error, phase);
  }
}

async function visibleText(page) {
  const visible = await page.locator('body').evaluate(node => {
    const text = node.innerText;
    return { text: text.slice(0, 32768), overflow: text.length > 32768 };
  }, undefined, { timeout: 3000 });
  const text = visible.text;
  if (visible.overflow || Buffer.byteLength(text) > 128 << 10) throw new Error('ui-text-bound');
  const refused = gate(text);
  if (refused) throw new Error(refused);
  return text;
}

async function refuseVisibleGate(page) {
  const text = await visibleText(page);
  // Text locators pierce open shadow roots; body.innerText need not include them.
  for (const [pattern, category] of gateRules) {
    if (await page.getByText(pattern).filter({ visible: true }).count() > 0) throw new Error(category);
  }
  return text;
}

async function controlFailureScreenshot(page, directory, result) {
  if (!page || result.observation !== 'ui-control-unobserved') return;
  try {
    // Diagnostic only; never a component inventory or qualification fact.
    result.diagnostic_screenshot = await screenshot(page, directory, 'control-unobserved.png');
  } catch (error) {
    // Keep the original refusal; only fixed facts describe this failed capture.
    if (error instanceof ScreenshotFailure) result.diagnostic_screenshot_failure = {
      phase: error.phase, category: error.category,
    };
  }
}

async function waitNamedControl(page, name) {
  await refuseVisibleGate(page);
  const control = page.getByText(name, { exact: true }).filter({ visible: true });
  const refusal = page.getByText(gatePattern).filter({ visible: true });
  try {
    // Native locator waiting observes rendering and gates together. first()
    // only selects a wake-up candidate; count below still refuses ambiguity.
    await control.or(refusal).first().waitFor({ state: 'visible', timeout: 5000 });
  } catch {
    await refuseVisibleGate(page);
    throw new Error('ui-control-unobserved');
  }
  await refuseVisibleGate(page);
  if (await control.count() !== 1) throw new Error('ui-control-unobserved');
  return control;
}

async function clickNamed(page, name) {
  const control = await waitNamedControl(page, name);
  await refuseVisibleGate(page);
  await control.click({ timeout: 5000 });
}

async function startupScreenshot(page, directory) {
  // firstWindow() does not guarantee a rendered UI. Readiness must precede
  // this mandatory capture, without acting on the control or any gate.
  const control = await waitNamedControl(page, 'Customize');
  await refuseVisibleGate(page);
  const image = await screenshot(page, directory, 'startup.png');
  await refuseVisibleGate(page);
  if (await control.count() !== 1) throw new Error('ui-control-unobserved');
  await refuseVisibleGate(page);
  return image;
}

async function confirmedMarkers(page, names) {
  const markers = {};
  for (const name of names) {
    markers[name] = await page.getByText(name, { exact: true }).filter({ visible: true }).count() === 1;
    if (!markers[name]) throw new Error('component-unobserved');
  }
  return markers;
}

async function inventoryScreenshot(page, directory, filename, names) {
  await refuseVisibleGate(page);
  for (const name of names) {
    try {
      await page.getByText(name, { exact: true }).filter({ visible: true })
        .waitFor({ state: 'visible', timeout: 5000 });
    } catch {
      await refuseVisibleGate(page); // Preserve a gate that appeared during the wait.
      throw new Error('component-unobserved');
    }
  }
  const text = await refuseVisibleGate(page);
  const markers = await confirmedMarkers(page, names);
  const image = await screenshot(page, directory, filename);
  await refuseVisibleGate(page);
  await confirmedMarkers(page, names);
  return { text, markers, image };
}

async function quit(app, closed, result) {
  // Use the real Electron application's ordinary Quit; never app.close(), whose
  // Playwright BrowserContext closure can fall back to process termination.
  result.quit_requested = true;
  const acknowledged = await bounded(app.evaluate(({ app: native }) => {
    setImmediate(() => native.quit());
    return true;
  }), 1000).catch(() => false);
  result.quit_acknowledged = acknowledged === true;
  const [code, signal] = await bounded(closed, 5000);
  result.native_exit = { code, signal };
  if (code !== 0 || signal !== null || acknowledged !== true) throw new Error('non-natural-exit');
}

async function observe(input, progress = () => {}) {
  const result = { status: 'unqualified', stage: 'preflight', observation: 'not-started',
    skills_ready: false, model_runtime_qualified: false, authentication_qualified: false,
    sandbox_requested: true, quit_requested: false, screenshots: [], markers: {},
    source_paths_visible: false, source_attribution_qualified: false };
  progress(result);
  let app, closed, page;
  try {
    for (const name of ['root', 'project', 'userData', 'extensions', 'output', 'playwright']) ordinary(input[name], true);
    ordinary(input.electron, false);
    if (JSON.parse(fs.readFileSync(path.join(input.playwright, 'package.json'), 'utf8')).version !== '1.64.1') throw new Error('tool-version-refused');
    const args = isolationArguments(input.help, input);
    const { _electron } = require(input.playwright);
    const nativeEnv = { ...process.env };
    delete nativeEnv.DEBUG;
    result.stage = 'editor-launch';
    app = await _electron.launch({ executablePath: input.electron, args,
      cwd: input.project, env: nativeEnv, chromiumSandbox: true, timeout: 30000 });
    const child = app.process();
    closed = once(child, 'close');
    if (child.spawnargs.includes('--no-sandbox') || child.spawnfile !== input.electron) throw new Error('launch-authority-refused');
    result.sandbox_request_and_argv_verified = true;
    page = await app.firstWindow({ timeout: 10000 });
    page.setDefaultTimeout(5000);
    result.stage = 'startup';
    result.screenshots.push(await startupScreenshot(page, input.output));
    result.stage = 'customize';
    await clickNamed(page, 'Customize');
    result.stage = 'skills';
    await clickNamed(page, 'Skills');
    const skills = await inventoryScreenshot(page, input.output, 'skills.png', [input.projectSkill, input.pluginSkill]);
    result.screenshots.push(skills.image);
    Object.assign(result.markers, skills.markers);
    result.source_paths_visible = skills.text.includes(input.projectSkillPath) && skills.text.includes(input.pluginSkillPath);
    result.stage = 'plugins';
    await clickNamed(page, 'Plugins');
    const plugins = await inventoryScreenshot(page, input.output, 'plugins.png', [input.pluginName]);
    result.screenshots.push(plugins.image);
    Object.assign(result.markers, plugins.markers);
    if (Object.values(result.markers).some(value => value !== true)) throw new Error('component-unobserved');
    result.observation = 'component-names-observed';
    result.stage = 'ordinary-quit';
    await quit(app, closed, result);
    app = undefined;
    result.status = 'observed';
  } catch (error) {
    const classes = ['help-refused', 'path-refused', 'tool-version-refused', 'phase-deadline', 'screenshot-refused', 'screenshot-bound',
      'screenshot-custody', 'ui-text-bound', 'login-gate', 'trust-gate', 'import-policy-gate',
      'ui-control-unobserved', 'launch-authority-refused', 'non-natural-exit', 'component-unobserved'];
    const message = error instanceof Error ? error.message : '';
    result.observation = classes.includes(message) ? message : 'editor-observation-refused';
    if (error instanceof ScreenshotFailure) result.screenshot_failure = {
      phase: error.phase, category: error.category,
    };
    await controlFailureScreenshot(page, input.output, result);
    if (app && closed) {
      try { await quit(app, closed, result); } catch { result.shutdown_unqualified = true; }
    }
  }
  return result;
}

async function main(write, progress) {
  let body = Buffer.alloc(0);
  for await (const chunk of process.stdin) {
    body = Buffer.concat([body, chunk]);
    if (body.length > 65536) throw new Error('input-bound');
  }
  const result = await observe(JSON.parse(body.toString('utf8')), progress);
  write(result);
  if (result.status !== 'observed') process.exitCode = 1;
}

module.exports = { isolationArguments, gate, ordinary, inventoryScreenshot, clickNamed, controlFailureScreenshot, startupScreenshot };
if (require.main === module) {
  let written = false;
  let latest = { stage: 'input' };
  let crash = 'driver-exited-before-report';
  function write(result) {
    if (written) return;
    const body = JSON.stringify(result) + '\n';
    if (Buffer.byteLength(body) > 65536) throw new Error('report-bound');
    written = true;
    fs.writeSync(1, body);
  }
  process.on('uncaughtExceptionMonitor', error => {
    crash = error instanceof Error && error.message === 'Process failed to launch!'
      ? 'electron-launch-failed' : 'driver-crashed';
  });
  process.on('exit', code => {
    if (written) return;
    // Preserve Node's original crash/exit semantics; this only retains facts.
    write({ status: 'unqualified', stage: latest.stage, observation: crash,
      skills_ready: false, model_runtime_qualified: false,
      authentication_qualified: false, source_attribution_qualified: false,
      terminal_exit_code: code });
  });
  main(write, result => { latest = result; }).catch(() => {
    process.exitCode = 1;
    write({ status: 'unqualified', stage: latest.stage, observation: 'driver-refused',
      skills_ready: false, model_runtime_qualified: false,
      authentication_qualified: false, source_attribution_qualified: false });
  });
}
