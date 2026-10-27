import type {
  SkillSpectorDisplayReport,
  SkillSpectorManifest,
  SkillSpectorRecord,
  SkillSpectorShardDescriptor,
} from '../types/skillSpector.ts';
import { decodeSkillSpectorRecords, loadSkillSpectorReport } from './skillSpector.ts';

const revision = /^[0-9a-f]{40}$/;
const digest = /^sha256:[0-9a-f]{64}$/;
const repository = /^[A-Za-z0-9][A-Za-z0-9-]*\/[A-Za-z0-9][A-Za-z0-9._-]*$/;
// This is an explicit scanner/profile trust boundary, independent of installation.
export const skillSpectorV2ProfileDigest =
  'sha256:f8844756e3a56036425059aa70d57acd9c823f5054be2960838e38c2ce945731';
const scannerRevision = 'f3b25697d76256998ca9d42c198d256258d4c035';
const maximumShardBytes = 8 << 20;

function invalid(): never {
  throw new Error('SkillSpector catalog is invalid');
}
function object(value: unknown): Record<string, unknown> {
  if (!value || typeof value !== 'object' || Array.isArray(value)) invalid();
  return value as Record<string, unknown>;
}
function keys(value: Record<string, unknown>, expected: string[]) {
  if (
    Object.keys(value).length !== expected.length ||
    !expected.every((key) => Object.hasOwn(value, key))
  )
    invalid();
}
function count(value: unknown, maximum = 10_000): value is number {
  return Number.isSafeInteger(value) && Number(value) >= 0 && Number(value) <= maximum;
}
function validRepository(value: unknown): value is string {
  return typeof value === 'string' && value.length <= 200 && repository.test(value);
}
function metadata(value: Record<string, unknown>) {
  const scanner = object(value.scanner),
    profile = object(value.profile);
  keys(scanner, ['id', 'version', 'revision']);
  keys(profile, ['id', 'digest']);
  if (
    scanner.id !== 'skillspector' ||
    scanner.version !== '2.13.1' ||
    scanner.revision !== scannerRevision ||
    profile.id !== 'static-only-v2' ||
    profile.digest !== skillSpectorV2ProfileDigest
  )
    invalid();
}

export function decodeSkillSpectorManifest(value: unknown, now = new Date()): SkillSpectorManifest {
  const manifest = object(value);
  keys(manifest, [
    'schema_version',
    'generated_at',
    'source_commit',
    'scanner',
    'profile',
    'scope',
    'selection_digest',
    'coverage',
    'shards',
    'plugin_coverage',
  ]);
  metadata(manifest);
  if (
    manifest.schema_version !== 2 ||
    manifest.scope !== 'selected_sources' ||
    typeof manifest.source_commit !== 'string' ||
    !revision.test(manifest.source_commit) ||
    typeof manifest.selection_digest !== 'string' ||
    !digest.test(manifest.selection_digest) ||
    typeof manifest.generated_at !== 'string' ||
    !/^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\dZ$/.test(manifest.generated_at) ||
    !Number.isFinite(Date.parse(manifest.generated_at)) ||
    Date.parse(manifest.generated_at) > now.getTime() ||
    new Date(manifest.generated_at).toISOString() !==
      manifest.generated_at.replace(/Z$/, '.000Z') ||
    !Array.isArray(manifest.shards) ||
    manifest.shards.length > 10_000
  )
    invalid();
  const coverage = object(manifest.coverage);
  keys(coverage, ['selected', 'reported', 'pending']);
  if (
    !count(coverage.selected) ||
    !count(coverage.reported) ||
    !count(coverage.pending) ||
    coverage.selected !== coverage.reported + coverage.pending
  )
    invalid();
  const paths = new Set<string>();
  let records = 0;
  let totalShardBytes = 0;
  for (const item of manifest.shards) {
    const shard = object(item);
    keys(shard, ['repository', 'path', 'digest', 'bytes', 'record_count']);
    if (
      !validRepository(shard.repository) ||
      typeof shard.path !== 'string' ||
      !/^shards\/[0-9a-f]{64}\.json$/.test(shard.path) ||
      shard.digest !== `sha256:${shard.path.slice(7, -5)}` ||
      paths.has(shard.path) ||
      !count(shard.bytes, maximumShardBytes) ||
      !shard.bytes ||
      !count(shard.record_count, 256) ||
      !shard.record_count
    )
      invalid();
    paths.add(shard.path);
    records += shard.record_count;
    totalShardBytes += shard.bytes;
    if (totalShardBytes > 256 << 20) invalid();
  }
  if (
    records !== coverage.reported ||
    !Array.isArray(manifest.plugin_coverage) ||
    manifest.plugin_coverage.length > 10_000
  )
    invalid();
  const plugins = new Set<string>();
  let pluginSelected = 0,
    pluginReported = 0,
    pluginPending = 0;
  for (const item of manifest.plugin_coverage) {
    const plugin = object(item);
    keys(plugin, ['repository', 'plugin_path', 'selected', 'reported', 'pending']);
    if (
      !validRepository(plugin.repository) ||
      typeof plugin.plugin_path !== 'string' ||
      plugin.plugin_path.length > 1000 ||
      !/^(?:[A-Za-z0-9._-]+(?:\/[A-Za-z0-9._-]+)*)?$/.test(plugin.plugin_path) ||
      plugin.plugin_path.split('/').some((part) => ['.', '..', '.git'].includes(part)) ||
      !count(plugin.selected) ||
      !plugin.selected ||
      !count(plugin.reported) ||
      !count(plugin.pending) ||
      plugin.selected !== plugin.reported + plugin.pending
    )
      invalid();
    const key = JSON.stringify([plugin.repository.toLowerCase(), plugin.plugin_path]);
    if (plugins.has(key)) invalid();
    plugins.add(key);
    pluginSelected += plugin.selected;
    pluginReported += plugin.reported;
    pluginPending += plugin.pending;
  }
  if (
    pluginSelected > coverage.selected ||
    pluginReported > coverage.reported ||
    pluginPending > coverage.pending
  )
    invalid();
  return value as SkillSpectorManifest;
}

export function decodeSkillSpectorShard(
  value: unknown,
  manifest: SkillSpectorManifest,
  descriptor: SkillSpectorShardDescriptor,
  now = new Date(),
): SkillSpectorRecord[] {
  const shard = object(value);
  keys(shard, ['schema_version', 'scanner', 'profile', 'repository', 'records']);
  metadata(shard);
  if (shard.schema_version !== 2 || shard.repository !== descriptor.repository) invalid();
  const records = decodeSkillSpectorRecords(shard.records, manifest.generated_at, 256, now);
  if (
    records.length !== descriptor.record_count ||
    records.some(
      (record) => record.source.repository.toLowerCase() !== descriptor.repository.toLowerCase(),
    )
  )
    invalid();
  return records;
}

class HttpError extends Error {
  readonly status: number;
  constructor(status: number) {
    super(`SkillSpector request failed (${status})`);
    this.status = status;
  }
}
async function readBytes(
  url: string,
  maximum: number,
  fetcher: typeof fetch,
): Promise<Uint8Array<ArrayBuffer>> {
  const response = await fetcher(url, {
    headers: { Accept: 'application/json' },
    credentials: 'omit',
    signal: AbortSignal.timeout(15_000),
  });
  if (!response.ok) throw new HttpError(response.status);
  if (Number(response.headers.get('content-length')) > maximum || !response.body) {
    await response.body?.cancel();
    invalid();
  }
  const reader = response.body.getReader();
  const chunks: Uint8Array[] = [];
  let length = 0;
  try {
    while (true) {
      const { value, done } = await reader.read();
      if (done) break;
      length += value.byteLength;
      if (length > maximum) {
        await reader.cancel();
        invalid();
      }
      chunks.push(value);
    }
  } finally {
    reader.releaseLock();
  }
  const bytes = new Uint8Array(length);
  let offset = 0;
  for (const chunk of chunks) {
    bytes.set(chunk, offset);
    offset += chunk.byteLength;
  }
  return bytes;
}
function parse(bytes: Uint8Array): unknown {
  return JSON.parse(new TextDecoder('utf-8', { fatal: true }).decode(bytes));
}

/** A session cache; only requested repositories download findings. Failed promises remain isolated. */
export function createSkillSpectorClient(
  manifestUrl: string,
  legacyUrl: string,
  fetcher: typeof fetch = fetch,
) {
  const manifests = readBytes(manifestUrl, 4 << 20, fetcher)
    .then((bytes) => decodeSkillSpectorManifest(parse(bytes)))
    .catch(async (error) => {
      if (error instanceof HttpError && error.status === 404)
        return loadSkillSpectorReport(legacyUrl, fetcher);
      throw error;
    });
  const repositories = new Map<string, Promise<SkillSpectorDisplayReport>>();
  const shards = new Map<string, Promise<SkillSpectorRecord[]>>();
  let active = 0;
  const waiting: (() => void)[] = [];
  async function limited<T>(work: () => Promise<T>): Promise<T> {
    if (active >= 4) await new Promise<void>((resolve) => waiting.push(resolve));
    else active++;
    try {
      return await work();
    } finally {
      const next = waiting.shift();
      if (next) next();
      else active--;
    }
  }
  function loadShard(manifest: SkillSpectorManifest, descriptor: SkillSpectorShardDescriptor) {
    let pending = shards.get(descriptor.path);
    if (!pending) {
      pending = limited(async () => {
        const bytes = await readBytes(
          new URL(descriptor.path, manifestUrl).href,
          descriptor.bytes,
          fetcher,
        );
        if (bytes.byteLength !== descriptor.bytes) invalid();
        const hash = Array.from(
          new Uint8Array(await crypto.subtle.digest('SHA-256', bytes)),
          (byte) => byte.toString(16).padStart(2, '0'),
        ).join('');
        if (`sha256:${hash}` !== descriptor.digest) invalid();
        return decodeSkillSpectorShard(parse(bytes), manifest, descriptor);
      });
      shards.set(descriptor.path, pending);
    }
    return pending;
  }
  return {
    async reportFor(repositoryName: string): Promise<SkillSpectorDisplayReport> {
      const key = repositoryName.toLowerCase();
      let pending = repositories.get(key);
      if (!pending) {
        pending = (async () => {
          const manifest = await manifests;
          if (manifest.schema_version === 1) return manifest;
          const descriptors = manifest.shards.filter(
            (shard) => shard.repository.toLowerCase() === key,
          );
          const records = (
            await Promise.all(descriptors.map((descriptor) => loadShard(manifest, descriptor)))
          ).flat();
          const ids = new Set<string>();
          const subjects = new Set<string>();
          for (const record of records) {
            const subject = JSON.stringify([
              record.kind,
              record.source.repository.toLowerCase(),
              record.source.revision,
              record.source.path,
              record.plugin_path,
            ]);
            if (ids.has(record.id) || subjects.has(subject)) invalid();
            ids.add(record.id);
            subjects.add(subject);
          }
          const pluginCounts = new Map<string, number>();
          for (const record of records) {
            if (record.kind === 'plugin_skill')
              pluginCounts.set(
                record.plugin_path!,
                (pluginCounts.get(record.plugin_path!) ?? 0) + 1,
              );
          }
          for (const plugin of manifest.plugin_coverage.filter(
            (item) => item.repository.toLowerCase() === key,
          )) {
            if ((pluginCounts.get(plugin.plugin_path) ?? 0) !== plugin.reported) invalid();
            pluginCounts.delete(plugin.plugin_path);
          }
          if (pluginCounts.size) invalid();
          return { ...manifest, records };
        })();
        repositories.set(key, pending);
      }
      return pending;
    },
  };
}
