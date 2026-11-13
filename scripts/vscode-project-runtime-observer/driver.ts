/** Closed UI driver for the pinned VS Code trusted development-host fixture.
 * It starts the genuine publisher in this driver's already isolated namespace.
 * Native gateway operations belong to runner.ts; this file never calls a probe.
 */
import { spawn, type ChildProcess } from 'node:child_process';
import { constants, closeSync, fstatSync, lstatSync, openSync, readSync,
  realpathSync, renameSync, statSync, writeFileSync } from 'node:fs';
import { basename, dirname, isAbsolute, normalize, relative } from 'node:path';
import { performance } from 'node:perf_hooks';
import { chromium, type Browser, type Locator, type Page } from 'playwright-core';

const LIMIT = 1 << 20;
const PROOF = 'UAP_VSCODE_ISOLATION_V1 ';
// Keep byte-for-byte equal to vscodeworkspace/capture_linux.go isolationVerifier.
const isolationVerifier = `
import json,os,socket,sys
parent,uid,gid,env,argv=sys.argv[1:]
p={"pid":os.getpid(),"network_namespace":os.stat("/proc/self/ns/net").st_ino,"parent_network_namespace":int(parent),"uid":os.getuid(),"gid":os.getgid(),"euid":os.geteuid(),"egid":os.getegid(),"groups":os.getgroups(),"interfaces":[name for _,name in socket.if_nameindex()]}
if p["network_namespace"]==int(parent) or p["uid"]!=int(uid) or p["gid"]!=int(gid) or p["uid"]==0 or p["gid"]==0 or p["euid"]!=int(uid) or p["egid"]!=int(gid) or p["groups"] or p["interfaces"]!=["lo"]:raise SystemExit("isolated process admission failed")
with open("/proc/self/status") as f:status=dict(line.rstrip().split(":",1) for line in f if ":" in line)
if any(int(status[k].strip(),16) for k in ("CapInh","CapPrm","CapEff","CapBnd","CapAmb")) or status["NoNewPrivs"].strip()!="1":raise SystemExit("native privilege drop failed")
p["capabilities_zero"]=True;p["no_new_privileges"]=True
os.write(1,("UAP_VSCODE_ISOLATION_V1 "+json.dumps(p,separators=(",",":"))+"\\n").encode())
os.dup2(1,2)
os.execve(json.loads(argv)[0],json.loads(argv),json.loads(env))
`;

type ObjectValue = Record<string, unknown>;
type Server = { label: string; originUri: string };
type Request = {
  mode: 'trusted-project-v1'; stage: string; project: string;
  publisher: string; publisherArgs: string[]; publisherEnv: Record<string, string>;
  originalNetworkNamespace: string; uid: number; gid: number;
  expectedServers: Server[]; triggerFile: string; resultFile: string; driverResultFile: string;
  nonce: string; timeoutMs: number; trustExpected: 'prompt' | 'persisted';
};
type PublisherProof = {
  pid: number; network_namespace: number; parent_network_namespace: number;
  uid: number; gid: number; euid: number; egid: number; groups: unknown[];
  interfaces: string[]; capabilities_zero: true; no_new_privileges: true;
};

function strictJSON(body: string): unknown {
  if (Buffer.byteLength(body) > LIMIT) throw new Error('JSON body exceeds bound');
  let position = 0, nodes = 0;
  const whitespace = (): void => { while (/[ \t\r\n]/u.test(body[position] ?? 'x')) position++; };
  const string = (): string => {
    const start = position++;
    while (position < body.length) {
      const char = body[position++];
      if (char === '\\') { position++; continue; }
      if (char === '"') return JSON.parse(body.slice(start, position)) as string;
    }
    throw new Error('unterminated JSON string');
  };
  const value = (depth: number): unknown => {
    whitespace();
    if (++nodes > 8192 || depth > 32) throw new Error('JSON structure exceeds bound');
    const char = body[position];
    if (char === '"') return string();
    if (char === '{') {
      position++; whitespace();
      const result: ObjectValue = Object.create(null) as ObjectValue, seen = new Set<string>();
      if (body[position] === '}') { position++; return result; }
      while (position < body.length) {
        whitespace();
        if (body[position] !== '"') throw new Error('JSON object key missing');
        const key = string(); whitespace();
        if (seen.has(key) || body[position++] !== ':') throw new Error('duplicate key or JSON separator');
        seen.add(key); result[key] = value(depth + 1); whitespace();
        const next = body[position++];
        if (next === '}') return result;
        if (next !== ',') throw new Error('JSON object separator missing');
      }
    } else if (char === '[') {
      position++; whitespace();
      const result: unknown[] = [];
      if (body[position] === ']') { position++; return result; }
      while (position < body.length) {
        result.push(value(depth + 1)); whitespace();
        const next = body[position++];
        if (next === ']') return result;
        if (next !== ',') throw new Error('JSON array separator missing');
      }
    } else {
      const token = /^(?:true|false|null|-?(?:0|[1-9][0-9]*)(?:\.[0-9]+)?(?:[eE][+-]?[0-9]+)?)/u.exec(body.slice(position))?.[0];
      if (token) {
        position += token.length;
        const parsed = JSON.parse(token) as unknown;
        if (typeof parsed === 'number' && !Number.isFinite(parsed)) throw new Error('nonfinite JSON number');
        return parsed;
      }
    }
    throw new Error('malformed JSON value');
  };
  const result = value(0); whitespace();
  if (position !== body.length) throw new Error('trailing JSON data');
  return result;
}

function object(value: unknown): ObjectValue {
  if (value === null || typeof value !== 'object' || Array.isArray(value)) throw new Error('expected closed object');
  return value as ObjectValue;
}
function keys(value: ObjectValue, allowed: string[]): void {
  const actual = Object.keys(value).sort();
  if (JSON.stringify(actual) !== JSON.stringify([...allowed].sort())) throw new Error('closed fields differ');
}
function text(value: unknown, max = 4096): string {
  if (typeof value !== 'string' || value.length === 0 || Buffer.byteLength(value) > max || /[\x00-\x1f\x7f]/u.test(value)) throw new Error('bounded literal text required');
  return value;
}
function canonical(value: unknown, directory: boolean): string {
  const path = text(value);
  if (!isAbsolute(path) || normalize(path) !== path || realpathSync(path) !== path) throw new Error('canonical path required');
  const info = lstatSync(path);
  if (directory ? !info.isDirectory() : !info.isFile() || info.nlink !== 1) throw new Error('ordinary path required');
  return path;
}
function outputPath(value: unknown): string {
  const path = text(value);
  if (!isAbsolute(path) || normalize(path) !== path || canonical(dirname(path), true) !== dirname(path)) throw new Error('canonical output parent required');
  try { lstatSync(path); } catch (error) {
    if ((error as NodeJS.ErrnoException).code === 'ENOENT') return path;
    throw error;
  }
  throw new Error('output already exists');
}
function contained(root: string, path: string): boolean {
  const rel = relative(root, path);
  return rel !== '' && rel !== '..' && !rel.startsWith('../') && !isAbsolute(rel);
}
function request(value: unknown): Request {
  const v = object(value);
  keys(v, ['mode', 'stage', 'project', 'publisher', 'publisherArgs', 'publisherEnv',
    'originalNetworkNamespace', 'uid', 'gid', 'expectedServers', 'triggerFile', 'resultFile',
    'nonce', 'timeoutMs', 'trustExpected', 'driverResultFile']);
  if (v.mode !== 'trusted-project-v1' || typeof v.stage !== 'string' || !/^[a-z][a-z0-9-]{0,80}$/u.test(v.stage)
    || typeof v.nonce !== 'string' || !/^[0-9a-f]{32,64}$/u.test(v.nonce)
    || typeof v.originalNetworkNamespace !== 'string' || !/^[1-9][0-9]{0,15}$/u.test(v.originalNetworkNamespace)
    || !Number.isSafeInteger(v.uid) || Number(v.uid) <= 0 || !Number.isSafeInteger(v.gid) || Number(v.gid) <= 0
    || v.uid !== process.getuid?.() || v.gid !== process.getgid?.()
    || v.timeoutMs !== 60000 || !['prompt', 'persisted'].includes(String(v.trustExpected))) throw new Error('stage identity or limits differ');
  const project = canonical(v.project, true), publisher = canonical(v.publisher, false);
  if (basename(publisher) !== 'code') throw new Error('exact publisher entrypoint required');
  const args = v.publisherArgs;
  if (!Array.isArray(args) || args.some(arg => typeof arg !== 'string') || args.length !== 14) throw new Error('fixed publisher argv required');
  const a = args as string[];
  const profile = canonical(a[5], true), extensions = canonical(a[7], true), observer = canonical(a[9], true);
  const expected = ['--new-window', '--disable-crash-reporter', '--locale', 'en', '--user-data-dir', profile,
    '--extensions-dir', extensions, '--extensionDevelopmentPath', observer,
    '--enable-proposed-api', 'uap-research.project-runtime-observer', '--remote-debugging-port=0', project];
  if (JSON.stringify(a) !== JSON.stringify(expected)) throw new Error('publisher argv differs from frozen vector');
  if ([profile, extensions, observer].some(path => path === project || contained(project, path) || contained(path, project)))
    throw new Error('native profile/tools must be separate from the project');
  const env = object(v.publisherEnv), allowed = new Set(['HOME', 'USERPROFILE', 'PATH', 'LANG', 'TMPDIR', 'DISPLAY', 'XAUTHORITY',
    'NO_COLOR', 'TERM', 'XDG_CONFIG_HOME', 'XDG_DATA_HOME', 'XDG_CACHE_HOME', 'XDG_STATE_HOME',
    'UAP_VSCODE_WORKSPACE_FOLDER', 'UAP_VSCODE_TRUSTED_REQUEST', 'UAP_VSCODE_TRUSTED_READY', 'UAP_VSCODE_TRUSTED_RESULT']);
  for (const [name, val] of Object.entries(env)) if (!allowed.has(name) || typeof val !== 'string' || /[\x00\r\n]/u.test(val)) throw new Error('publisher environment refused');
  const home = canonical(env.HOME, true), triggerFile = outputPath(v.triggerFile), resultFile = outputPath(v.resultFile),
    driverResultFile = outputPath(v.driverResultFile);
  if (dirname(driverResultFile) !== dirname(resultFile) || [triggerFile, resultFile].includes(driverResultFile)) throw new Error('driver result handoff differs');
  if (env.USERPROFILE !== home || env.PATH !== '/usr/bin:/bin' || env.LANG !== 'C.UTF-8'
    || env.UAP_VSCODE_WORKSPACE_FOLDER !== project || env.UAP_VSCODE_TRUSTED_READY !== triggerFile || env.UAP_VSCODE_TRUSTED_RESULT !== resultFile
    || project === home || contained(project, home) || contained(home, project) || triggerFile === resultFile) throw new Error('original profile or handoff differs');
  const observerRequest = object(ordinaryJSON(canonical(env.UAP_VSCODE_TRUSTED_REQUEST, false)));
  if (observerRequest.stage !== v.stage || observerRequest.nonce !== v.nonce) throw new Error('observer request stage differs');
  if (!Array.isArray(v.expectedServers) || v.expectedServers.length > 8) throw new Error('server selection bound differs');
  const servers: Server[] = [], seen = new Set<string>();
  for (const entry of v.expectedServers) {
    const row = object(entry); keys(row, ['label', 'originUri']);
    const label = text(row.label, 256), originUri = text(row.originUri);
    const uri = new URL(originUri);
    if (seen.has(label) || uri.protocol !== 'file:' || uri.host !== '' || uri.search || uri.hash) throw new Error('exact server origin required');
    seen.add(label); servers.push({ label, originUri });
  }
  return { ...v, project, publisher, publisherArgs: a, publisherEnv: env, expectedServers: servers,
    triggerFile, resultFile, driverResultFile } as Request;
}

function ordinaryJSON(path: string): unknown {
  canonical(path, false);
  const before = lstatSync(path);
  if (before.size > LIMIT) throw new Error('result bound exceeded');
  const fd = openSync(path, constants.O_RDONLY | constants.O_NOFOLLOW | constants.O_NONBLOCK);
  try {
    const held = fstatSync(fd);
    if (held.dev !== before.dev || held.ino !== before.ino || held.size !== before.size || held.nlink !== 1) throw new Error('held result identity differs');
    const bytes = Buffer.alloc(before.size + 1);
    const length = readSync(fd, bytes, 0, bytes.length, 0);
    const after = fstatSync(fd), linked = lstatSync(path);
    if (length !== before.size || after.mtimeMs !== held.mtimeMs || after.ctimeMs !== held.ctimeMs || after.size !== held.size
      || linked.dev !== held.dev || linked.ino !== held.ino) throw new Error('result changed while reading');
    return strictJSON(new TextDecoder('utf-8', { fatal: true }).decode(bytes.subarray(0, length)));
  } finally { closeSync(fd); }
}
function delay(ms: number): Promise<void> { return new Promise(resolve => setTimeout(resolve, ms)); }
async function one(locator: Locator): Promise<Locator> {
  if (await locator.count() !== 1) throw new Error('native UI selection is absent or ambiguous');
  return locator;
}
async function visibleDialogs(page: Page): Promise<Locator> { return page.getByRole('dialog').filter({ visible: true }); }
async function exactDialog(page: Page, title: string): Promise<Locator> {
  const dialogs = await visibleDialogs(page);
  const dialog = dialogs.filter({ has: page.getByText(title, { exact: true }) });
  if (await dialogs.count() !== 1) throw new Error('unexpected native dialog');
  return one(dialog);
}
async function noDialog(page: Page): Promise<void> {
  if (await (await visibleDialogs(page)).count() !== 0) throw new Error('unplanned native dialog');
}
async function quickInput(page: Page, placeholder?: string): Promise<Locator> {
  const input = page.locator('.quick-input-widget:visible input');
  await input.waitFor({ state: 'visible', timeout: 3000 });
  await one(input);
  if (placeholder !== undefined && await input.getAttribute('placeholder') !== placeholder) throw new Error('native picker differs');
  return input;
}
async function labelRow(page: Page, label: string): Promise<Locator> {
  const rows = page.locator('.quick-input-widget:visible .monaco-list-row');
  const count = await rows.count();
  if (count > 256) throw new Error('native picker row bound exceeded');
  let selected: Locator | undefined;
  for (let index = 0; index < count; index++) {
    const row = rows.nth(index), names = row.locator('.label-name');
    if (await names.count() === 1 && await names.innerText() === label) {
      if (selected) throw new Error('native picker label is ambiguous');
      selected = row;
    }
  }
  if (!selected) throw new Error('native picker label missing');
  return selected;
}
async function pick(page: Page, label: string): Promise<void> {
  await (await labelRow(page, label)).click({ timeout: 3000 });
}
async function listServers(page: Page): Promise<void> {
  await noDialog(page);
  await page.keyboard.press('Control+Shift+P');
  const input = await quickInput(page);
  await input.fill('>MCP: List Servers');
  await pick(page, 'MCP: List Servers');
  await quickInput(page, 'Select an MCP Server');
}
async function workspaceTrust(page: Page, req: Request): Promise<boolean> {
  const title = 'Do you trust the authors of the files in this folder?';
  if (req.trustExpected === 'persisted') { await noDialog(page); return false; }
  await page.getByText(title, { exact: true }).waitFor({ state: 'visible', timeout: 15000 });
  const dialog = await exactDialog(page, title);
  await one(dialog.getByText(req.project, { exact: true }));
  const label = "Trust the authors of all files in the parent folder '" + basename(dirname(req.project)) + "'";
  const checkbox = await one(dialog.getByRole('checkbox', { name: label, exact: true }));
  if (await checkbox.isChecked()) await checkbox.uncheck({ timeout: 3000 });
  if (await checkbox.isChecked()) throw new Error('parent workspace trust would be granted');
  await (await one(dialog.getByRole('button', { name: 'Yes, I trust the authors', exact: true }))).click({ timeout: 3000 });
  await dialog.waitFor({ state: 'hidden', timeout: 3000 });
  return true;
}
async function startServer(page: Page, server: Server, stageDeadline: number): Promise<ObjectValue> {
  const deadline = Math.min(stageDeadline, performance.now() + 15000);
  await listServers(page);
  await pick(page, server.label);
  await quickInput(page, "Select action for '" + server.label + "'");
  await pick(page, 'Start Server');
  let trusted = false, catalogOpen = false;
  while (performance.now() < deadline) {
    const dialogs = await visibleDialogs(page);
    if (await dialogs.count()) {
      const dialog = await exactDialog(page, 'Trust and run MCP server ' + server.label + '?');
      const link = await one(dialog.getByRole('link', { name: server.label, exact: true }));
      if (await link.getAttribute('href') !== server.originUri) throw new Error('native MCP trust origin differs');
      await (await one(dialog.getByRole('button', { name: 'Trust', exact: true }))).click({ timeout: 3000 });
      await dialog.waitFor({ state: 'hidden', timeout: 3000 });
      trusted = true; catalogOpen = false;
    } else if (!catalogOpen) {
      await delay(200);
      if (await (await visibleDialogs(page)).count()) continue;
      await listServers(page); catalogOpen = true;
    } else {
      const row = await labelRow(page, server.label);
      const state = (await row.locator('.label-description').innerText()).trim();
      if (state === 'Running') { await page.keyboard.press('Escape'); await noDialog(page); return { label: server.label, originUri: server.originUri, trustPromptAccepted: trusted, nativeState: state }; }
      if (state !== 'Starting' && state !== 'Stopped') throw new Error('native MCP failed or unknown state');
      await delay(100);
    }
  }
  throw new Error('native MCP startup deadline exceeded');
}

async function input(): Promise<unknown> {
  return new Promise((resolve, reject) => {
    let data = Buffer.alloc(0);
    const cleanup = (): void => { process.stdin.removeListener('data', received); process.stdin.removeListener('end', ended); process.stdin.removeListener('error', failed); process.stdin.pause(); };
    const failed = (): void => { cleanup(); reject(new Error('driver input stream failed')); };
    const ended = (): void => { cleanup(); reject(new Error('request frame missing before EOF')); };
    const received = (chunk: Buffer): void => {
      try {
        data = Buffer.concat([data, chunk]);
        if (data.length > LIMIT) throw new Error('driver request exceeds bound');
        const newline = data.indexOf(10);
        if (newline < 0) return;
        if (newline !== data.length - 1) throw new Error('one request frame required');
        const parsed = strictJSON(new TextDecoder('utf-8', { fatal: true }).decode(data));
        cleanup(); resolve(parsed);
      } catch { cleanup(); reject(new Error('driver request frame refused')); }
    };
    process.stdin.on('data', received); process.stdin.once('end', ended); process.stdin.once('error', failed); process.stdin.resume();
  });
}
async function parentShutdown(): Promise<void> {
  if (process.stdin.readableEnded) throw new Error('parent ended the control stream before planned shutdown');
  await new Promise<void>((resolve, reject) => {
    const cleanup = (): void => { process.stdin.removeListener('data', extra); process.stdin.removeListener('end', ended); process.stdin.removeListener('error', failed); };
    const extra = (): void => { cleanup(); reject(new Error('unexpected second control frame')); };
    const ended = (): void => { cleanup(); resolve(); };
    const failed = (): void => { cleanup(); reject(new Error('planned shutdown stream failed')); };
    process.stdin.once('data', extra); process.stdin.once('end', ended); process.stdin.once('error', failed); process.stdin.resume();
  });
}
async function waitResult(req: Request, deadline: number): Promise<unknown> {
  while (performance.now() < deadline) {
    try { return ordinaryJSON(req.resultFile); } catch (error) {
      if ((error as NodeJS.ErrnoException).code !== 'ENOENT') throw error;
    }
    await delay(50);
  }
  throw new Error('observer result deadline exceeded');
}
async function stopped(child: ChildProcess, milliseconds: number): Promise<boolean> {
  if (child.exitCode !== null || child.signalCode !== null) return true;
  return new Promise(resolve => {
    const timer = setTimeout(() => { child.removeListener('exit', exited); resolve(false); }, milliseconds);
    function exited(): void { clearTimeout(timer); resolve(true); }
    child.once('exit', exited);
  });
}

async function within<T>(work: Promise<T>, milliseconds: number): Promise<T> {
  let timer: ReturnType<typeof setTimeout> | undefined;
  try {
    return await Promise.race([work, new Promise<never>((_resolve, reject) => {
      timer = setTimeout(() => reject(new Error('bounded operation deadline')), milliseconds);
    })]);
  } finally { if (timer) clearTimeout(timer); }
}

async function focusWorkbench(page: Page, req: Request, stageDeadline: number): Promise<void> {
  const deadline = Math.min(stageDeadline, performance.now() + 5000);
  const remaining = (): number => {
    const value = deadline - performance.now();
    if (value <= 0) throw new Error('workbench focus deadline exceeded');
    return value;
  };
  const initialReadMs = Math.min(1000, remaining());
  const before = await within(page.evaluate<unknown>('document.hasFocus()'), initialReadMs);
  if (typeof before !== 'boolean') throw new Error('native initial focus is malformed');
  const activationMs = Math.min(1000, remaining());
  await within(page.bringToFront(), activationMs);
  const focusDeadline = Math.min(deadline, performance.now() + 3000);
  let after: boolean | undefined;
  while (performance.now() < focusDeadline) {
    const readMs = Math.min(1000, remaining(), focusDeadline - performance.now());
    if (readMs <= 0) break;
    const observed = await within(page.evaluate<unknown>('document.hasFocus()'), readMs);
    if (typeof observed !== 'boolean') throw new Error('native final focus is malformed');
    after = observed;
    if (after) break;
    await delay(Math.min(50, Math.max(0, focusDeadline - performance.now())));
  }
  if (after === undefined) throw new Error('native final focus is missing');
  process.stdout.write(JSON.stringify({ type: 'driver-focus', stage: req.stage, nonce: req.nonce, before, after }) + '\n');
  if (!after) throw new Error('native workbench did not acquire focus');
}

async function failureUI(page: Page | undefined): Promise<ObjectValue[]> {
  if (!page || page.isClosed()) return [];
  const result: ObjectValue[] = [];
  const safe = (value: string | null): string => (value ?? '').slice(0, 256)
    .replace(/(?:https?|wss?):\/\/[^\s<>"']+/gu, '[private-url]')
    .replace(/[A-Za-z0-9_-]{40,}/gu, '[opaque-value]')
    .replace(/[\x00-\x1f\x7f]/gu, ' ');
  const add = (row: ObjectValue): void => {
    if (result.length < 16 && Buffer.byteLength(JSON.stringify([...result, row])) <= 4096) result.push(row);
  };
  try {
    const dialogs = page.getByRole('dialog').filter({ visible: true });
    for (let index = 0; index < Math.min(await dialogs.count(), 4); index++) {
      const dialog = dialogs.nth(index), title = dialog.locator('.dialog-message');
      add({ kind: 'dialog', label: safe(await dialog.getAttribute('aria-label', { timeout: 100 })),
        title: await title.count() === 1 ? safe(await title.textContent({ timeout: 100 })) : '' });
    }
    const inputs = page.locator('.quick-input-widget:visible input');
    for (let index = 0; index < Math.min(await inputs.count(), 2); index++)
      add({ kind: 'quick-input', placeholder: safe(await inputs.nth(index).getAttribute('placeholder', { timeout: 100 })) });
    const names = page.locator('.quick-input-widget:visible .monaco-list-row .label-name');
    for (let index = 0; index < Math.min(await names.count(), 16); index++)
      add({ kind: 'quick-label', label: safe(await names.nth(index).textContent({ timeout: 100 })) });
  } catch { /* Partial bounded metadata is useful; no fallback action is taken. */ }
  return result;
}

async function main(): Promise<void> {
  const req = request(await input()), deadline = performance.now() + req.timeoutMs;
  const namespace = statSync('/proc/self/ns/net').ino;
  if (!Number.isSafeInteger(namespace) || String(namespace) === req.originalNetworkNamespace) throw new Error('driver isolation namespace differs');
  let child: ChildProcess | undefined, browser: Browser | undefined, page: Page | undefined;
  let phase = 'publisher-startup';
  let diagnostic = Buffer.alloc(0), first = true, proof: PublisherProof | undefined, endpoint = '', captureError: Error | undefined;
  const onData = (chunk: Buffer): void => {
    try {
    diagnostic = Buffer.concat([diagnostic, chunk]);
    if (diagnostic.length > LIMIT) { captureError = new Error('native merged stream overflow'); child?.kill('SIGTERM'); return; }
    if (first) {
      const index = diagnostic.indexOf(10);
      if (index < 0) { if (diagnostic.length > 4096) captureError = new Error('publisher proof bound exceeded'); return; }
      first = false;
      const line = diagnostic.subarray(0, index).toString('utf8');
      if (!line.startsWith(PROOF)) { captureError = new Error('publisher proof missing'); return; }
      const p = object(strictJSON(line.slice(PROOF.length)));
      if (p.pid !== child?.pid || p.network_namespace !== namespace || String(p.parent_network_namespace) !== req.originalNetworkNamespace
        || p.uid !== req.uid || p.gid !== req.gid || p.euid !== req.uid || p.egid !== req.gid || JSON.stringify(p.groups) !== '[]'
        || JSON.stringify(p.interfaces) !== '["lo"]' || p.capabilities_zero !== true || p.no_new_privileges !== true) { captureError = new Error('publisher same-process admission differs'); return; }
      proof = p as PublisherProof;
    }
    const complete = diagnostic.subarray(0, diagnostic.lastIndexOf(10) + 1);
    const lines = new TextDecoder('utf-8', { fatal: true }).decode(complete).split('\n');
    const matches = lines.filter(line => /^DevTools listening on /u.test(line));
    if (matches.length > 1) { captureError = new Error('ambiguous native DevTools endpoint'); return; }
    if (matches.length === 1 && matches[0]?.endsWith('\r') === false) {
      const candidate = matches[0]?.slice('DevTools listening on '.length) ?? '';
      if (!/^ws:\/\/127\.0\.0\.1:([1-9][0-9]{3,4})\/devtools\/browser\/[A-Za-z0-9-]+$/u.test(candidate)) { captureError = new Error('native DevTools endpoint refused'); return; }
      const url = new URL(candidate), port = Number(url.port);
      if (port < 1024 || port > 65535) { captureError = new Error('native DevTools port refused'); return; }
      endpoint = candidate;
    }
    } catch { captureError = new Error('malformed publisher admission stream'); }
  };
  const timer = setTimeout(() => { captureError = new Error('stage deadline exceeded'); child?.kill('SIGTERM'); }, req.timeoutMs);
  try {
    child = spawn('/usr/bin/python3', ['-I', '-c', isolationVerifier, req.originalNetworkNamespace, String(req.uid), String(req.gid),
      JSON.stringify(req.publisherEnv), JSON.stringify([req.publisher, ...req.publisherArgs])],
    { cwd: req.project, env: { PATH: '/usr/bin:/bin', LANG: 'C.UTF-8' }, stdio: ['ignore', 'pipe', 'pipe'], shell: false });
    child.stdout?.on('data', onData);
    child.stderr?.on('data', (chunk: Buffer) => { if (chunk.length) captureError = new Error('unexpected pre-verifier stderr'); });
    child.on('error', () => { captureError = new Error('publisher spawn failed'); });
    while (!endpoint || !proof) {
      if (captureError) throw captureError;
      if (child.exitCode !== null || child.signalCode !== null || performance.now() >= deadline) throw new Error('publisher startup failed');
      await delay(25);
    }
    phase = 'native-devtools-admission';
    browser = await chromium.connectOverCDP(endpoint, { timeout: Math.min(10000, deadline - performance.now()) });
    const contexts = browser.contexts();
    if (contexts.length !== 1) throw new Error('native browser contexts differ');
    while (!page && performance.now() < deadline) {
      const pages = contexts[0]?.pages().filter(candidate => candidate.url().startsWith('vscode-file://vscode-app/')) ?? [];
      if (pages.length > 1) throw new Error('multiple native workbench pages');
      page = pages[0];
      if (!page) await delay(50);
    }
    if (!page) throw new Error('native workbench page missing');
    page.setDefaultTimeout(3000);
    await page.locator('.monaco-workbench').waitFor({ state: 'visible', timeout: Math.min(15000, deadline - performance.now()) });
    phase = 'workbench-focus';
    await focusWorkbench(page, req, deadline);
    phase = 'workspace-trust';
    const trustPromptAccepted = await workspaceTrust(page, req), servers: ObjectValue[] = [];
    phase = 'native-mcp-start';
    for (const server of req.expectedServers) servers.push(await startServer(page, server, deadline));
    await noDialog(page);
    writeFileSync(req.triggerFile, JSON.stringify({ stage: req.stage, nonce: req.nonce }) + '\n', { encoding: 'utf8', mode: 0o600, flag: 'wx' });
    phase = 'observer-result';
    const result = await waitResult(req, deadline);
    if (captureError) throw captureError;
    if (performance.now() >= deadline) throw new Error('stage deadline exceeded');
    clearTimeout(timer);
    phase = 'publisher-shutdown';
    const shutdownDeadline = performance.now() + 5000;
    await within(browser.close(), 1000); browser = undefined;
    child.kill('SIGTERM');
    if (!await stopped(child, Math.max(1, shutdownDeadline - performance.now()))) throw new Error('publisher graceful shutdown incomplete');
    if (!((child.exitCode === 0 && child.signalCode === null) || (child.exitCode === null && child.signalCode === 'SIGTERM')))
      throw new Error('publisher terminal result differs');
    const output = JSON.stringify({ type: 'driver-result', stage: req.stage, nonce: req.nonce, trustPromptAccepted,
      servers, publisherProof: proof, publisherExit: { code: child.exitCode, signal: child.signalCode }, result }) + '\n';
    if (Buffer.byteLength(output) > LIMIT) throw new Error('driver output bound exceeded');
    phase = 'driver-result-publication';
    outputPath(req.driverResultFile);
    const pending = outputPath(req.driverResultFile + '.pending');
    writeFileSync(pending, output, { encoding: 'utf8', mode: 0o600, flag: 'wx' });
    outputPath(req.driverResultFile);
    renameSync(pending, req.driverResultFile);
    process.stdout.write(output);
    // The strict controller must observe the result and complete its exchange
    // callback before this leader exits. No publisher child remains here.
    phase = 'parent-planned-shutdown';
    await parentShutdown();
  } catch {
    const metadata = await within(failureUI(page), Math.max(1, Math.min(500, deadline - performance.now()))).catch(() => []);
    process.stdout.write(JSON.stringify({ type: 'driver-failure', stage: req.stage, nonce: req.nonce, phase,
      code: 'closed-stage-contract-refused', visibleNativeUI: metadata }) + '\n');
    throw new Error('reported closed driver failure');
  } finally {
    clearTimeout(timer);
    if (browser) await within(browser.close(), 1000).catch(() => undefined);
    if (child && child.exitCode === null && child.signalCode === null) { child.kill('SIGTERM'); await stopped(child, 5000); }
    // Native diagnostics include a private DevTools endpoint. They are retained
    // only in bounded memory and never copied to stdout or a public artifact.
    diagnostic.fill(0); endpoint = '';
  }
}
main().catch(() => {
  // Exception strings from browser/protocol libraries can contain endpoints.
  process.stderr.write('trusted VS Code driver failed its closed stage contract\n');
  process.exitCode = 1;
});
