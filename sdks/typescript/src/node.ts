import { spawn } from 'node:child_process';
import { isAbsolute } from 'node:path';
import { createClient, ManagementError } from './client.js';
import type { components } from './generated/schema.js';
import type { Session } from './types.js';

export type StartupConfiguration = components['schemas']['Startup'];
type Bootstrap = components['schemas']['Bootstrap'];
export interface StartOptions {
  /** Exact host-provisioned executable. The SDK never downloads or searches PATH. */
  executable: string;
  config: StartupConfiguration;
  startupTimeoutMs?: number;
  shutdownTimeoutMs?: number;
  signal?: AbortSignal;
  /** Explicit process environment. Default inherits only OS, locale and proxy settings. */
  env?: Record<string, string | undefined>;
  requiredCapabilities?: string[];
}
const bootstrapLimit = 64 * 1024;
function boundedTimeout(value: number, name: string): number {
  if (!Number.isFinite(value) || value <= 0 || value > 2_147_483_647) throw new RangeError(`${name} must be between 1 and 2147483647 milliseconds.`);
  return value;
}
function defaultEnvironment(): NodeJS.ProcessEnv {
  const result: NodeJS.ProcessEnv = {};
  for (const name of ['PATH', 'Path', 'SystemRoot', 'WINDIR', 'TEMP', 'TMP', 'TMPDIR', 'HOME', 'USERPROFILE', 'APPDATA', 'LOCALAPPDATA', 'XDG_CONFIG_HOME', 'XDG_CACHE_HOME', 'XDG_DATA_HOME', 'LANG', 'LC_ALL', 'HTTP_PROXY', 'HTTPS_PROXY', 'NO_PROXY', 'http_proxy', 'https_proxy', 'no_proxy', 'SSL_CERT_FILE', 'SSL_CERT_DIR']) {
    if (process.env[name] !== undefined) result[name] = process.env[name];
  }
  result.NO_COLOR = '1';
  return result;
}
function readBootstrap(line: string): Bootstrap {
  let value: unknown;
  try { value = JSON.parse(line); }
  catch { throw new ManagementError('INVALID_BOOTSTRAP', 'The engine did not return a valid bootstrap document.'); }
  if (typeof value !== 'object' || value === null) throw new ManagementError('INVALID_BOOTSTRAP', 'The engine returned an invalid bootstrap shape.');
  const record = value as Record<string, unknown>;
  if (record.apiVersion !== 1) throw new ManagementError('UNSUPPORTED_API_VERSION', 'The engine management API is incompatible with this SDK.');
  for (const name of ['url', 'origin', 'token', 'engineVersion', 'contextId']) {
    if (typeof record[name] !== 'string' || !record[name]) throw new ManagementError('INVALID_BOOTSTRAP', 'The engine bootstrap is missing a required field.');
  }
  let url: URL;
  try { url = new URL(record.url as string); }
  catch { throw new ManagementError('INVALID_BOOTSTRAP', 'The engine returned an invalid local endpoint.'); }
  if (url.protocol !== 'http:' || url.hostname !== '127.0.0.1' || !url.port || url.username || url.password || url.search || url.hash || url.pathname !== '/' || record.url !== url.origin || record.origin !== url.origin || !/^[a-fA-F0-9]{64}$/.test(record.token as string)) {
    throw new ManagementError('INVALID_BOOTSTRAP', 'The engine bootstrap must contain a private IPv4 loopback origin and capability.');
  }
  return record as unknown as Bootstrap;
}
function verifySession(session: Session, bootstrap: Bootstrap, required: string[]) {
  if (session.apiVersion !== 1 || session.contextId !== bootstrap.contextId || session.engineVersion !== bootstrap.engineVersion || !Array.isArray(session.targets) || !Array.isArray(session.capabilities)) {
    throw new ManagementError('SESSION_MISMATCH', 'The authenticated engine session does not match its bootstrap.');
  }
  for (const action of required) {
    if (!session.capabilities.some((capability) => capability.action === action && capability.supported)) {
      throw new ManagementError('UNSUPPORTED_CAPABILITY', `The engine does not support required capability: ${action}.`);
    }
  }
}
async function settledWithin(promise: Promise<void>, ms: number): Promise<boolean> {
  let timer: ReturnType<typeof setTimeout> | undefined;
  try {
    return await Promise.race([promise.then(() => true), new Promise<false>((resolve) => { timer = setTimeout(() => resolve(false), ms); })]);
  } finally { if (timer !== undefined) clearTimeout(timer); }
}

/** Owns one explicitly provisioned engine process. Close requests graceful shutdown through stdin EOF. */
export async function startUap(options: StartOptions) {
  if (!isAbsolute(options.executable)) throw new ManagementError('INVALID_EXECUTABLE', 'Provide an absolute path to the pinned UAP executable.');
  const startupMs = boundedTimeout(options.startupTimeoutMs ?? 30_000, 'startupTimeoutMs');
  const shutdownMs = boundedTimeout(options.shutdownTimeoutMs ?? 10_000, 'shutdownTimeoutMs');
  const configuration = JSON.stringify(options.config) + '\n';
  if (Buffer.byteLength(configuration) > bootstrapLimit) throw new ManagementError('CONFIG_TOO_LARGE', 'The captured host configuration exceeds 64 KiB.');
  options.signal?.throwIfAborted();
  const child = spawn(options.executable, ['manage', 'serve'], {
    env: options.env ?? defaultEnvironment(), shell: false, windowsHide: true, stdio: ['pipe', 'pipe', 'pipe'],
  });
  // Do not forward either stream: stdout contains the private bootstrap capability;
  // stderr belongs to the engine and is not a public SDK diagnostic contract.
  child.stderr.resume();
  child.stdin.on('error', () => undefined);
  let processExited = false;
  let exitCode: number | null = null;
  const exited = new Promise<void>((resolve) => {
    child.once('exit', (code) => { processExited = true; exitCode = code; resolve(); });
    child.once('error', () => { processExited = true; resolve(); });
  });
  let closing: Promise<void> | undefined;
  const close = (): Promise<void> => {
    closing ??= (async () => {
      child.stdin.end();
      if (await settledWithin(exited, shutdownMs)) return;
      child.kill('SIGTERM');
      if (await settledWithin(exited, 1_000)) return;
      child.kill('SIGKILL');
      if (!await settledWithin(exited, 1_000)) throw new ManagementError('SHUTDOWN_TIMEOUT', 'The owned engine did not exit. Its accepted work requires inspection before retrying.');
    })();
    return closing;
  };
  const startup = new AbortController();
  const abort = () => startup.abort(options.signal?.reason);
  options.signal?.addEventListener('abort', abort, { once: true });
  if (options.signal?.aborted) abort();
  const timer = setTimeout(() => startup.abort(new ManagementError('STARTUP_TIMEOUT', 'The engine did not become ready before the startup deadline.')), startupMs);
  try {
    const bootstrap = await new Promise<Bootstrap>((resolve, reject) => {
      let buffered = Buffer.alloc(0);
      let received = false;
      const cleanup = () => {
        child.stdout.removeListener('data', data);
        child.removeListener('error', failed);
        child.removeListener('exit', ended);
        startup.signal.removeEventListener('abort', cancelled);
        child.stdout.resume();
      };
      const failed = () => { cleanup(); reject(new ManagementError('PROCESS_START_FAILED', 'Could not start the pinned UAP executable. Check its path, permissions and platform.')); };
      const ended = () => { cleanup(); reject(new ManagementError('PROCESS_EXITED', `The engine exited before readiness (exit code ${exitCode ?? 'unknown'}).`)); };
      const cancelled = () => { cleanup(); reject(startup.signal.reason); };
      const data = (chunk: Buffer) => {
        if (received) return;
        if (buffered.length + chunk.length > bootstrapLimit) {
          cleanup(); reject(new ManagementError('BOOTSTRAP_TOO_LARGE', 'The engine bootstrap exceeds 64 KiB.')); return;
        }
        buffered = Buffer.concat([buffered, chunk]);
        const newline = buffered.indexOf(10);
        if (newline < 0) return;
        received = true;
        cleanup();
        try {
          if (buffered.subarray(newline + 1).toString('utf8').trim()) throw new ManagementError('INVALID_BOOTSTRAP', 'The engine returned unexpected bootstrap output.');
          resolve(readBootstrap(buffered.subarray(0, newline).toString('utf8')));
        } catch (error) { reject(error); }
      };
      child.stdout.on('data', data);
      child.once('error', failed);
      child.once('exit', ended);
      startup.signal.addEventListener('abort', cancelled, { once: true });
      if (startup.signal.aborted) cancelled();
      child.stdin.write(configuration);
    });
    const client = createClient({ baseUrl: bootstrap.url, origin: bootstrap.origin, token: bootstrap.token });
    const session = await client.session({ signal: startup.signal });
    verifySession(session, bootstrap, options.requiredCapabilities ?? []);
    if (processExited || child.pid === undefined) throw new ManagementError('PROCESS_EXITED', 'The engine exited during readiness verification.');
    return { client, session, pid: child.pid, close };
  } catch (error) {
    await close();
    throw error;
  } finally {
    clearTimeout(timer);
    options.signal?.removeEventListener('abort', abort);
  }
}
export type UapHost = Awaited<ReturnType<typeof startUap>>;
