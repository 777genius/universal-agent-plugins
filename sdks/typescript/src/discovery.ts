import { spawn } from 'node:child_process';
import { isAbsolute } from 'node:path';
import { ManagementError } from './client.js';
import type { components } from './generated/schema.js';

export type TargetDiscovery = components['schemas']['TargetDiscovery'];
export interface DiscoverTargetsOptions {
  /** Exact host-provisioned executable. No download, shell or PATH lookup. */
  executable: string;
  /** Source environment; only OS and native-profile locator variables are passed. */
  env?: Record<string, string | undefined>;
  signal?: AbortSignal;
  timeoutMs?: number;
}

const outputLimit = 64 * 1024;
function discoveryEnvironment(source: NodeJS.ProcessEnv): NodeJS.ProcessEnv {
  const result: NodeJS.ProcessEnv = {};
  const inherited = new Set<string>();
  // These are observations/locators, never credentials or native launch arguments.
  for (const name of [
    'PATH',
    'Path',
    'SystemRoot',
    'WINDIR',
    'TEMP',
    'TMP',
    'TMPDIR',
    'HOME',
    'USERPROFILE',
    'APPDATA',
    'LOCALAPPDATA',
    'ProgramFiles',
    'ProgramW6432',
    'ProgramFiles(x86)',
    'XDG_CONFIG_HOME',
    'XDG_CACHE_HOME',
    'XDG_DATA_HOME',
    'XDG_STATE_HOME',
    'CODEX_HOME',
    'CLAUDE_CONFIG_DIR',
    'GEMINI_CLI_HOME',
    'CLINE_DATA_DIR',
    'CLINE_MCP_SETTINGS_PATH',
    'AGENTPLUGINS_HOME',
    'LANG',
    'LC_ALL',
  ]) {
    const folded = name.toUpperCase();
    if (source[name] !== undefined && !inherited.has(folded)) {
      result[name] = source[name];
      inherited.add(folded);
    }
  }
  result.NO_COLOR = '1';
  return result;
}
const record = (value: unknown): value is Record<string, unknown> =>
  typeof value === 'object' && value !== null && !Array.isArray(value);
const text = (value: unknown, maximum: number): value is string =>
  typeof value === 'string' &&
  value.length > 0 &&
  value.length <= maximum &&
  value === value.trim() &&
  !/[\0\r\n]/.test(value);
const status = (value: unknown) => value === 'detected' || value === 'not_detected';
function invalid(): never {
  throw new ManagementError(
    'INVALID_DISCOVERY',
    'The engine returned invalid private client discovery.',
  );
}
function readDiscovery(body: string): TargetDiscovery {
  let value: unknown;
  try {
    value = JSON.parse(body);
  } catch {
    return invalid();
  }
  if (!record(value)) return invalid();
  if (value.apiVersion !== 1)
    throw new ManagementError(
      'UNSUPPORTED_API_VERSION',
      'The engine management API is incompatible with this SDK.',
    );
  if (!text(value.engineVersion, 128) || !Array.isArray(value.targets) || value.targets.length > 32)
    return invalid();
  const clients = new Set<string>();
  for (const item of value.targets) {
    if (
      !record(item) ||
      !text(item.client, 128) ||
      clients.has(item.client) ||
      !text(item.label, 256) ||
      !status(item.detectionStatus) ||
      typeof item.available !== 'boolean' ||
      (item.reason !== undefined && !text(item.reason, 4096))
    )
      return invalid();
    clients.add(item.client);
    if (item.profile !== undefined) {
      const profile = item.profile;
      if (
        !record(profile) ||
        profile.id !== item.client ||
        profile.client !== item.client ||
        profile.detectionStatus !== item.detectionStatus ||
        !text(profile.configRoot, 4096) ||
        !isAbsolute(profile.configRoot) ||
        (profile.executable !== undefined &&
          (!text(profile.executable, 4096) || !isAbsolute(profile.executable)))
      )
        return invalid();
    }
    if (item.available && (item.profile === undefined || item.detectionStatus !== 'detected'))
      return invalid();
  }
  return value as unknown as TargetDiscovery;
}

/** Passive private host discovery. The short-lived owned child never executes a native client. */
export async function discoverTargets(options: DiscoverTargetsOptions): Promise<TargetDiscovery> {
  if (!isAbsolute(options.executable))
    throw new ManagementError(
      'INVALID_EXECUTABLE',
      'Provide an absolute path to the pinned UAP executable.',
    );
  const timeoutMs = options.timeoutMs ?? 10_000;
  if (!Number.isFinite(timeoutMs) || timeoutMs <= 0 || timeoutMs > 2_147_483_647)
    throw new RangeError('timeoutMs must be between 1 and 2147483647 milliseconds.');
  options.signal?.throwIfAborted();
  const child = spawn(options.executable, ['manage', 'discover'], {
    env: discoveryEnvironment(options.env ?? process.env),
    shell: false,
    windowsHide: true,
    stdio: ['ignore', 'pipe', 'pipe'],
  });
  // Neither private profile paths nor arbitrary engine diagnostics belong in logs.
  child.stderr.resume();
  return new Promise<TargetDiscovery>((resolve, reject) => {
    let bytes = Buffer.alloc(0);
    let failure: unknown;
    let stopping = false;
    let settled = false;
    let killTimer: ReturnType<typeof setTimeout> | undefined;
    let reapTimer: ReturnType<typeof setTimeout> | undefined;
    const finish = (error?: unknown, result?: TargetDiscovery) => {
      if (settled) return;
      settled = true;
      clearTimeout(timer);
      if (killTimer !== undefined) clearTimeout(killTimer);
      if (reapTimer !== undefined) clearTimeout(reapTimer);
      options.signal?.removeEventListener('abort', abort);
      child.stdout.removeListener('data', data);
      child.stdout.resume();
      if (error !== undefined) reject(error);
      else resolve(result!);
    };
    const stop = (error: unknown) => {
      if (settled || stopping) return;
      stopping = true;
      failure = error;
      child.kill('SIGTERM');
      killTimer = setTimeout(() => {
        child.kill('SIGKILL');
      }, 250);
      reapTimer = setTimeout(
        () =>
          finish(
            new ManagementError('SHUTDOWN_TIMEOUT', 'The owned discovery process did not exit.'),
          ),
        1_250,
      );
    };
    const abort = () =>
      stop(
        options.signal?.reason ??
          new ManagementError('DISCOVERY_ABORTED', 'Client discovery was canceled.'),
      );
    const timer = setTimeout(
      () =>
        stop(new ManagementError('DISCOVERY_TIMEOUT', 'Client discovery exceeded its deadline.')),
      timeoutMs,
    );
    const data = (chunk: Buffer) => {
      if (stopping || settled) return;
      if (bytes.length + chunk.length > outputLimit) {
        stop(new ManagementError('DISCOVERY_TOO_LARGE', 'The engine discovery exceeds 64 KiB.'));
        return;
      }
      bytes = Buffer.concat([bytes, chunk]);
    };
    child.stdout.on('data', data);
    // Signal errors can recur during cleanup; keep them handled until close.
    child.on('error', () =>
      stop(
        new ManagementError(
          'PROCESS_START_FAILED',
          'Could not start the pinned UAP executable. Check its path, permissions and platform.',
        ),
      ),
    );
    child.once('close', (code) => {
      if (stopping) {
        finish(failure);
        return;
      }
      if (code !== 0) {
        finish(
          new ManagementError(
            'PROCESS_EXITED',
            `The discovery engine exited unsuccessfully (exit code ${code ?? 'unknown'}).`,
          ),
        );
        return;
      }
      try {
        finish(undefined, readDiscovery(bytes.toString('utf8')));
      } catch (error) {
        finish(error);
      }
    });
    options.signal?.addEventListener('abort', abort, { once: true });
    if (options.signal?.aborted) abort();
  });
}
