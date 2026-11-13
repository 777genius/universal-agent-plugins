/** Actual proposed-API observations and MCP requests through the vendor gateway. */
import * as vscode from 'vscode';
import { createHash } from 'node:crypto';
import { constants, closeSync, fstatSync, lstatSync, openSync, readSync,
  realpathSync, renameSync, statSync, writeFileSync } from 'node:fs';
import { request as httpRequest } from 'node:http';
import { dirname, isAbsolute, normalize } from 'node:path';
import { performance } from 'node:perf_hooks';

const LIMIT = 1 << 20;
let phase = 'request';
const observations: ObjectValue[] = [];
type ObjectValue = Record<string, unknown>;
type Skill = { name: string; path: string; sha256: string; source: string };
type Server = { label: string; command: string; args: string[]; env: Record<string, string>;
  cwd: string; facts: ObjectValue };
type Request = { mode: 'trusted-project-v1'; stage: string; nonce: string; project: string;
  home: string; appRoot: string; skills: Skill[]; absentSkills: string[];
  servers: Server[]; absentServers: string[]; operation: 'write' | 'read'; marker: string };

function object(value: unknown): ObjectValue {
  if (value === null || typeof value !== 'object' || Array.isArray(value)) throw new Error('expected object');
  return value as ObjectValue;
}
function keys(value: ObjectValue, expected: string[]): void {
  if (JSON.stringify(Object.keys(value).sort()) !== JSON.stringify([...expected].sort())) throw new Error('closed fields differ');
}
function literal(value: unknown, max = 4096): string {
  if (typeof value !== 'string' || value.length === 0 || Buffer.byteLength(value) > max || /[\x00-\x1f\x7f]/u.test(value)) throw new Error('bounded literal required');
  return value;
}
function path(value: unknown, directory: boolean): string {
  const selected = literal(value);
  if (!isAbsolute(selected) || normalize(selected) !== selected || realpathSync(selected) !== selected) throw new Error('canonical path required');
  const info = lstatSync(selected);
  if (directory ? !info.isDirectory() : !info.isFile() || info.nlink !== 1) throw new Error('ordinary path required');
  return selected;
}
function read(pathname: string, maximum = LIMIT): Buffer {
  path(pathname, false);
  const before = lstatSync(pathname);
  if (before.size > maximum) throw new Error('file bound exceeded');
  const fd = openSync(pathname, constants.O_RDONLY | constants.O_NOFOLLOW | constants.O_NONBLOCK);
  try {
    const held = fstatSync(fd);
    if (held.dev !== before.dev || held.ino !== before.ino || held.size !== before.size || held.nlink !== 1) throw new Error('held identity differs');
    const body = Buffer.alloc(before.size + 1);
    let count = 0;
    while (count < body.length) {
      const size = readSync(fd, body, count, body.length - count, count);
      if (size === 0) break;
      count += size;
    }
    const after = fstatSync(fd), linked = lstatSync(pathname);
    if (count !== before.size || after.size !== held.size || after.mtimeMs !== held.mtimeMs || after.ctimeMs !== held.ctimeMs
      || linked.dev !== held.dev || linked.ino !== held.ino || linked.mtimeMs !== held.mtimeMs || linked.ctimeMs !== held.ctimeMs) throw new Error('file changed during read');
    return body.subarray(0, count);
  } finally { closeSync(fd); }
}
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

function json(body: Buffer): unknown {
  return strictJSON(new TextDecoder('utf-8', { fatal: true }).decode(body));
}

function same(left: unknown, right: unknown): boolean {
  if (left === null || right === null || typeof left !== 'object' || typeof right !== 'object') return left === right;
  if (Array.isArray(left) || Array.isArray(right)) return Array.isArray(left) && Array.isArray(right) && left.length === right.length && left.every((item, index) => same(item, right[index]));
  const l = object(left), r = object(right), names = Object.keys(l).sort();
  return same(names, Object.keys(r).sort()) && names.every(name => same(l[name], r[name]));
}
function strings(value: unknown, maximum: number): string[] {
  if (!Array.isArray(value) || value.length > maximum) throw new Error('bounded string array required');
  return value.map(item => literal(item));
}
function decodeRequest(value: unknown): Request {
  const v = object(value);
  keys(v, ['mode', 'stage', 'nonce', 'project', 'home', 'appRoot', 'skills', 'absentSkills', 'servers', 'absentServers', 'operation', 'marker']);
  if (v.mode !== 'trusted-project-v1' || !/^[a-z][a-z0-9-]{0,80}$/u.test(literal(v.stage))
    || !/^[0-9a-f]{32,64}$/u.test(literal(v.nonce)) || !['write', 'read'].includes(String(v.operation))) throw new Error('request identity differs');
  const project = path(v.project, true), home = path(v.home, true), appRoot = path(v.appRoot, true);
  if (project === home || process.env.HOME !== home || process.env.UAP_VSCODE_WORKSPACE_FOLDER !== project) throw new Error('original home/workspace differs');
  if (!Array.isArray(v.skills) || v.skills.length > 8 || !Array.isArray(v.servers) || v.servers.length > 8) throw new Error('fixture bound exceeded');
  const skills: Skill[] = v.skills.map(item => {
    const row = object(item); keys(row, ['name', 'path', 'sha256', 'source']);
    if (!/^[0-9a-f]{64}$/u.test(literal(row.sha256))) throw new Error('skill hash differs');
    return { name: literal(row.name), path: path(row.path, false), sha256: String(row.sha256), source: literal(row.source) };
  });
  const servers: Server[] = v.servers.map(item => {
    const row = object(item); keys(row, ['label', 'command', 'args', 'env', 'cwd', 'facts']);
    const env = object(row.env);
    Object.values(env).forEach(item => literal(item));
    return { label: literal(row.label), command: path(row.command, false), args: strings(row.args, 16),
      env: env as Record<string, string>, cwd: row.cwd === '' ? '' : path(row.cwd, true), facts: object(row.facts) };
  });
  if (new Set(skills.map(row => row.name)).size !== skills.length || new Set(servers.map(row => row.label)).size !== servers.length) throw new Error('ambiguous fixture identity');
  return { ...v, project, home, appRoot, skills, servers,
    absentSkills: strings(v.absentSkills, 16), absentServers: strings(v.absentServers, 16), marker: literal(v.marker) } as Request;
}
function delay(ms: number): Promise<void> { return new Promise(resolve => setTimeout(resolve, ms)); }
async function waitReady(req: Request, deadline: number): Promise<void> {
  const ready = literal(process.env.UAP_VSCODE_TRUSTED_READY);
  path(dirname(ready), true);
  while (performance.now() < deadline) {
    try {
      const value = object(json(read(ready, 4096)));
      keys(value, ['stage', 'nonce']);
      if (value.stage !== req.stage || value.nonce !== req.nonce) throw new Error('UI handoff differs');
      return;
    } catch (error) {
      if ((error as NodeJS.ErrnoException).code !== 'ENOENT') throw error;
    }
    await delay(25);
  }
  throw new Error('UI handoff deadline');
}

type Reply = { status: number; session: string; body: Buffer };
async function gatewayHTTP(address: URL, method: 'POST' | 'DELETE', session: string, payload: unknown, deadline: number): Promise<Reply> {
  const body = payload === undefined ? Buffer.alloc(0) : Buffer.from(JSON.stringify(payload));
  if (body.length > LIMIT || performance.now() >= deadline) throw new Error('gateway request bound');
  return new Promise((resolve, reject) => {
    const headers: Record<string, string> = { Accept: 'application/json, text/event-stream' };
    if (method === 'POST') { headers['Content-Type'] = 'application/json'; headers['Content-Length'] = String(body.length); }
    if (session) headers['Mcp-Session-Id'] = session;
    const request = httpRequest(address, { method, headers, agent: false }, response => {
      const chunks: Buffer[] = []; let size = 0;
      response.on('data', (chunk: Buffer) => {
        size += chunk.length;
        if (size > LIMIT) { response.destroy(new Error('gateway response bound')); return; }
        chunks.push(chunk);
      });
      response.on('error', () => { clearTimeout(timer); reject(new Error('gateway response failed')); });
      response.on('aborted', () => { clearTimeout(timer); reject(new Error('gateway response aborted')); });
      response.on('end', () => {
        clearTimeout(timer);
        const token = response.headers['mcp-session-id'];
        if (token !== undefined && (typeof token !== 'string' || !/^[A-Za-z0-9-]{1,128}$/u.test(token))) { reject(new Error('gateway session format')); return; }
        resolve({ status: response.statusCode ?? 0, session: token ?? '', body: Buffer.concat(chunks) });
      });
    });
    const maximum = Math.max(1, Math.min(10000, deadline - performance.now()));
    const timer = setTimeout(() => request.destroy(new Error('absolute gateway deadline')), maximum);
    request.setTimeout(maximum, () => request.destroy(new Error('gateway inactivity deadline')));
    request.on('error', () => { clearTimeout(timer); reject(new Error('gateway transport failed')); });
    request.end(body);
  });
}
function rpc(reply: Reply, id: number): ObjectValue {
  if (reply.status !== 200) throw new Error('gateway request failed');
  const value = object(json(reply.body));
  keys(value, ['jsonrpc', 'id', 'result']);
  if (value.jsonrpc !== '2.0' || value.id !== id) throw new Error('gateway RPC identity differs');
  return object(value.result);
}
async function callGateway(server: vscode.McpGatewayServer, expected: Server, req: Request, deadline: number): Promise<ObjectValue> {
  const address = new URL(server.address.toString());
  if (address.protocol !== 'http:' || address.hostname !== '127.0.0.1' || address.username || address.password || address.search || address.hash
    || !/^\/gateway\/[A-Za-z0-9-]+$/u.test(address.pathname) || Number(address.port) < 1024 || Number(address.port) > 65535) throw new Error('vendor gateway address refused');
  let session = '';
  try {
    phase = 'gateway-initialize';
    const initialized = await gatewayHTTP(address, 'POST', '', { jsonrpc: '2.0', id: 1, method: 'initialize', params: {
      protocolVersion: '2025-06-18', capabilities: {}, clientInfo: { name: 'uap-project-observer', version: '1.0.0' } } }, deadline);
    const init = rpc(initialized, 1);
    observations.push({ phase, label: expected.label, result: init });
    if (init.protocolVersion !== '2025-06-18' || !same(init.serverInfo, { name: 'VS Code MCP Gateway', version: '1.0.0' }) || !initialized.session) throw new Error('vendor gateway initialization differs');
    session = initialized.session;
    const notification = await gatewayHTTP(address, 'POST', session, { jsonrpc: '2.0', method: 'notifications/initialized' }, deadline);
    if (notification.status !== 202 || notification.body.length !== 0 || notification.session !== session) throw new Error('gateway initialized notification differs');
    phase = 'gateway-tools-list';
    const listed = await gatewayHTTP(address, 'POST', session, { jsonrpc: '2.0', id: 2, method: 'tools/list', params: {} }, deadline);
    const catalog = rpc(listed, 2);
    observations.push({ phase, label: expected.label, result: catalog });
    if (listed.session !== session || !Array.isArray(catalog.tools) || catalog.tools.length !== 1) throw new Error('exact gateway tool catalog differs');
    const tool = object(catalog.tools[0]);
    if (tool.name !== 'inspect_runtime' || tool.description !== 'Inspect only disposable fixture runtime facts.'
      || !same(tool.inputSchema, { type: 'object', properties: { nonce: { type: 'string' }, operation: { type: 'string' }, marker: { type: 'string' } },
        required: ['nonce'], additionalProperties: false })) throw new Error('installed gateway tool schema differs');
    const argumentsValue = { nonce: req.nonce, operation: req.operation, marker: req.marker };
    phase = 'gateway-tools-call';
    const called = await gatewayHTTP(address, 'POST', session, { jsonrpc: '2.0', id: 3, method: 'tools/call', params: { name: 'inspect_runtime', arguments: argumentsValue } }, deadline);
    const output = rpc(called, 3);
    if (called.session !== session || output.isError === true || !Array.isArray(output.content) || output.content.length !== 1) throw new Error('gateway tool call failed');
    const content = object(output.content[0]);
    if (content.type !== 'text' || typeof content.text !== 'string' || Buffer.byteLength(content.text) > LIMIT) throw new Error('gateway result content differs');
    const facts = object(json(Buffer.from(content.text)));
    observations.push({ phase, label: expected.label, facts });
    keys(facts, ['nonce', 'revision', 'cwd', 'argv', 'plugin_root', 'plugin_data', 'literal', 'once', 'marker', 'pid', 'session']);
    const { pid, session: probeSession, ...values } = facts;
    if (!Number.isSafeInteger(pid) || Number(pid) <= 0 || typeof probeSession !== 'string' || !probeSession || !same(values, expected.facts)) throw new Error('returned installed probe facts differ');
    return { label: expected.label, initialize: init, tools: catalog, facts, operation: req.operation,
      gatewaySessionSha256: createHash('sha256').update(session).digest('hex'),
      gatewayEndpointSha256: createHash('sha256').update(address.href).digest('hex') };
  } finally {
    if (session) {
      const deleted = await gatewayHTTP(address, 'DELETE', session, undefined, deadline);
      if (deleted.status !== 204 || deleted.body.length !== 0) throw new Error('gateway session cleanup failed');
    }
  }
}

async function observe(req: Request, deadline: number): Promise<ObjectValue> {
  phase = 'workspace-identity';
  const folders = vscode.workspace.workspaceFolders;
  observations.push({ phase, version: vscode.version, appRoot: vscode.env.appRoot, appHost: vscode.env.appHost,
    folders: folders?.map(folder => folder.uri.fsPath), trusted: vscode.workspace.isTrusted });
  if (vscode.version !== '1.143.1' || vscode.env.appRoot !== req.appRoot || vscode.env.appHost !== 'desktop'
    || !folders || folders.length !== 1 || folders[0]?.uri.fsPath !== req.project || !vscode.workspace.isTrusted) throw new Error('native workbench identity/trust differs');
  const observer = vscode.extensions.getExtension('uap-research.project-runtime-observer');
  if (!observer || observer.packageJSON.version !== '1.0.0') throw new Error('native observer identity missing');
  const token = new vscode.CancellationTokenSource();
  const timer = setTimeout(() => token.cancel(), Math.max(1, deadline - performance.now()));
  try {
    phase = 'native-skills';
    const skills = await vscode.chat.getSkills(token.token);
    if (skills.length > 256) throw new Error('native skill count bound');
    observations.push({ phase, skills: skills.map(row => ({ name: row.name, path: row.uri.toString(), source: row.source })) });
    const skillEvidence: ObjectValue[] = [];
    for (const expected of req.skills) {
      const matches = skills.filter(row => row.name === expected.name);
      if (matches.length !== 1 || matches[0]?.uri.scheme !== 'file' || matches[0].uri.fsPath !== expected.path || String(matches[0].source) !== expected.source) throw new Error('native skill source differs');
      const digest = createHash('sha256').update(read(expected.path)).digest('hex');
      if (digest !== expected.sha256) throw new Error('native selected skill body differs');
      skillEvidence.push({ name: expected.name, path: expected.path, sha256: digest, source: matches[0].source });
    }
    if (req.absentSkills.some(name => skills.some(row => row.name === name))) throw new Error('other/removed skill visible');
    phase = 'native-definitions';
    const definitions = vscode.lm.mcpServerDefinitions;
    observations.push({ phase, definitions: definitions.slice(0, 256).map(row => row instanceof vscode.McpStdioServerDefinition
      ? { label: row.label, command: row.command, args: row.args, env: row.env, cwd: row.cwd?.fsPath }
      : { label: row.label, kind: 'http' }) });
    if (definitions.length > 256 || req.absentServers.some(label => definitions.some(row => row.label === label))) throw new Error('other/removed server visible');
    const definitionEvidence: ObjectValue[] = [];
    for (const expected of req.servers) {
      const matches = definitions.filter(row => row.label === expected.label);
      const actual = matches[0];
      if (matches.length !== 1 || !(actual instanceof vscode.McpStdioServerDefinition) || actual.command !== expected.command
        || !same(actual.args, expected.args) || !same(actual.env, expected.env) || (actual.cwd?.fsPath ?? '') !== expected.cwd) throw new Error('actual installed native server definition differs');
      definitionEvidence.push({ label: actual.label, command: actual.command, args: actual.args, env: actual.env, cwd: actual.cwd?.fsPath ?? '' });
    }
    phase = 'native-gateway';
    const gateway = await vscode.lm.startMcpGateway();
    if (!gateway) throw new Error('native gateway unavailable');
    const calls: ObjectValue[] = [];
    try {
      if (gateway.servers.length > 256 || req.absentServers.some(label => gateway.servers.some(row => row.label === label))) throw new Error('gateway server scope differs');
      for (const expected of req.servers) {
        const matches = gateway.servers.filter(row => row.label === expected.label);
        if (matches.length !== 1 || !matches[0]) throw new Error('gateway server missing or ambiguous');
        calls.push(await callGateway(matches[0], expected, req, deadline));
      }
    } finally { gateway.dispose(); }
    return { schemaVersion: 1, mode: req.mode, stage: req.stage, nonce: req.nonce, collected: true,
      version: vscode.version, appRoot: vscode.env.appRoot, appHost: vscode.env.appHost,
      folder: req.project, home: req.home, trusted: vscode.workspace.isTrusted, observerPID: process.pid,
      networkNamespace: Number(statSync('/proc/self/ns/net').ino), skills: skillEvidence, definitions: definitionEvidence, calls };
  } finally { clearTimeout(timer); token.dispose(); }
}

export async function run(): Promise<void> {
  const deadline = performance.now() + 60000;
  const requestFile = literal(process.env.UAP_VSCODE_TRUSTED_REQUEST);
  const req = decodeRequest(json(read(requestFile, 64 << 10)));
  const destination = literal(process.env.UAP_VSCODE_TRUSTED_RESULT);
  path(dirname(destination), true);
  const temporary = destination + '.pending';
  let result: ObjectValue;
  try {
    phase = 'ui-handoff';
    await waitReady(req, deadline);
    result = await observe(req, deadline);
  } catch {
    // Vendor error strings can include private gateway addresses/session IDs.
    result = { schemaVersion: 1, mode: req.mode, stage: req.stage, nonce: req.nonce,
      collected: false, phase, observations, code: 'observer-contract-failed' };
  }
  const body = Buffer.from(JSON.stringify(result) + '\n');
  if (body.length > LIMIT) throw new Error('observer result bound exceeded');
  try { lstatSync(destination); throw new Error('observer result already exists'); }
  catch (error) { if ((error as NodeJS.ErrnoException).code !== 'ENOENT') throw error; }
  writeFileSync(temporary, body, { flag: 'wx', mode: 0o600 });
  renameSync(temporary, destination);
}
