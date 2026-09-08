import type {
  DiscoveryBundle,
  DiscoveryEnvelope,
  DiscoveryPointer,
  DiscoveryRecord,
  DiscoverySearch,
  DiscoverySnapshot,
} from '../types/discovery';
import type { ClientID, ComponentID, RegistryPlugin } from '../types/registry';
import {
  assertExactKeys,
  bytesEqual,
  canonicalJSON,
  decodeBase64,
  encodeBase64,
  fetchSignedArtifact,
  parseCanonicalJSON,
  parseUTCTimestamp,
  sha256Digest,
  signedMessage,
  type ArtifactResponse,
} from './signedFeed.ts';

const signatureDomain = 'UAP-DISCOVERY-INDEX-ED25519-V1';
const digestPattern = /^sha256:[0-9a-f]{64}$/;
const revisionPattern = /^[0-9a-f]{40}$/;
const repositoryPattern = /^[a-z0-9][a-z0-9-]*\/[a-z0-9][a-z0-9._-]*$/;
const packagePathPattern = /^(?:|[A-Za-z0-9._-]+(?:\/[A-Za-z0-9._-]+)*)$/;
// Unreviewed Discovery metadata cannot assert the registered app binding that
// ChatGPT requires. Reviewed Directory products add ChatGPT independently.
const clientOrder: ClientID[] = ['codex', 'cursor', 'copilot', 'vscode', 'kiro'];
const transportOrder = ['sse', 'stdio', 'streamable-http'];

type ArtifactName = keyof DiscoveryBundle['bytes'];

export interface DiscoveryTrust {
  keyID: string;
  publicKeyBase64: string;
}

export interface CachedDiscovery {
  bytes: Record<ArtifactName, Uint8Array>;
  etags: DiscoveryBundle['etags'];
}

export interface DiscoveryCache {
  load(): Promise<CachedDiscovery | undefined>;
  store(value: CachedDiscovery): Promise<void>;
}

export interface DiscoveryLoadOptions {
  origin: URL;
  trust: DiscoveryTrust;
  cache?: DiscoveryCache;
  fetcher?: typeof fetch;
  now?: Date;
}

export class DiscoveryEquivocationError extends Error {}

export class BrowserDiscoveryCache implements DiscoveryCache {
  readonly cacheName = 'uap-discovery-v1';
  readonly entry: URL;

  constructor(origin: URL) {
    this.entry = new URL('.browser-lkg.json', origin);
  }

  async load(): Promise<CachedDiscovery | undefined> {
    if (!globalThis.caches) return undefined;
    const response = await (await caches.open(this.cacheName)).match(this.entry);
    if (!response?.ok) return undefined;
    const value = (await response.json()) as {
      schema: number;
      bytes: Record<ArtifactName, string>;
      etags: DiscoveryBundle['etags'];
    };
    if (value.schema !== 1) return undefined;
    return {
      bytes: {
        pointer: decodeBase64(value.bytes.pointer),
        envelope: decodeBase64(value.bytes.envelope),
        snapshot: decodeBase64(value.bytes.snapshot),
        search: decodeBase64(value.bytes.search),
      },
      etags: value.etags ?? {},
    };
  }

  async store(value: CachedDiscovery): Promise<void> {
    if (!globalThis.caches) return;
    const body = JSON.stringify({
      schema: 1,
      bytes: Object.fromEntries(
        Object.entries(value.bytes).map(([key, bytes]) => [key, encodeBase64(bytes)]),
      ),
      etags: value.etags,
    });
    await (
      await caches.open(this.cacheName)
    ).put(
      this.entry,
      new Response(body, {
        headers: { 'content-type': 'application/json' },
      }),
    );
  }
}

export async function loadDiscovery(options: DiscoveryLoadOptions): Promise<DiscoveryBundle> {
  const fetcher = options.fetcher ?? fetch;
  const now = options.now ?? new Date();
  const cachedRaw = await options.cache?.load().catch(() => undefined);
  const cached = cachedRaw
    ? await verifyDiscovery(cachedRaw.bytes, options.trust, cachedRaw.etags, now).catch(
        () => undefined,
      )
    : undefined;
  const pointerResponse = await fetchSignedArtifact(
    new URL('latest.json', options.origin),
    16 << 10,
    fetcher,
    'Discovery',
    cachedRaw?.etags.pointer,
  ).catch((error: unknown) => ({ error }));

  if ('error' in pointerResponse || pointerResponse.notModified) {
    if (cached) return { ...cached, source: 'cache' };
    throw 'error' in pointerResponse
      ? pointerResponse.error
      : new Error('Discovery returned 304 without a valid cache');
  }

  try {
    const pointer = parsePointer(pointerResponse.bytes);
    if (cached && pointer.sequence < cached.snapshot.sequence)
      return { ...cached, source: 'cache' };
    if (cached && pointer.sequence === cached.snapshot.sequence) {
      if (!bytesEqual(pointerResponse.bytes, cached.bytes.pointer))
        throw new DiscoveryEquivocationError('Discovery sequence equivocation detected');
      return { ...cached, source: 'cache' };
    }

    const artifactSpecs: Array<[Exclude<ArtifactName, 'pointer'>, string, number]> = [
      ['snapshot', pointer.snapshot_path, pointer.fetch_contract.snapshot_max_bytes],
      ['envelope', pointer.envelope_path, pointer.fetch_contract.envelope_max_bytes],
      ['search', pointer.search_path, pointer.fetch_contract.search_max_bytes],
    ];
    const loaded = await Promise.all(
      artifactSpecs.map(async ([name, path, maximum]) => {
        const response = await fetchSignedArtifact(
          new URL(path, options.origin),
          maximum,
          fetcher,
          'Discovery',
          undefined,
        );
        if (response.notModified) throw new Error(`Discovery ${name} returned an unexpected 304`);
        return [name, response] as const;
      }),
    );
    const responses = Object.fromEntries(loaded) as Record<
      Exclude<ArtifactName, 'pointer'>,
      ArtifactResponse
    >;
    const bytes = {
      pointer: pointerResponse.bytes,
      snapshot: responses.snapshot.bytes,
      envelope: responses.envelope.bytes,
      search: responses.search.bytes,
    };
    const etags = {
      pointer: pointerResponse.etag,
      snapshot: responses.snapshot.etag,
      envelope: responses.envelope.etag,
      search: responses.search.etag,
    };
    const verified = await verifyDiscovery(bytes, options.trust, etags, now);
    await options.cache?.store({ bytes, etags }).catch(() => undefined);
    return { ...verified, source: 'remote' };
  } catch (error) {
    if (error instanceof DiscoveryEquivocationError) throw error;
    if (cached) return { ...cached, source: 'cache' };
    throw error;
  }
}

export async function verifyDiscovery(
  bytes: Record<ArtifactName, Uint8Array>,
  trust: DiscoveryTrust,
  etags: DiscoveryBundle['etags'] = {},
  now = new Date(),
): Promise<DiscoveryBundle> {
  const pointer = parsePointer(bytes.pointer);
  const envelope = parseCanonicalJSON<DiscoveryEnvelope>(bytes.envelope, 'Discovery', 'envelope');
  const snapshot = parseCanonicalJSON<DiscoverySnapshot>(bytes.snapshot, 'Discovery', 'snapshot');
  const search = parseCanonicalJSON<DiscoverySearch>(bytes.search, 'Discovery', 'search');
  assertEnvelope(envelope, trust);
  assertSnapshot(snapshot);
  assertSearch(search);
  if (
    pointer.sequence !== snapshot.sequence ||
    envelope.sequence !== snapshot.sequence ||
    search.sequence !== snapshot.sequence
  ) {
    throw new Error('Discovery sequence mismatch');
  }
  if (
    pointer.search_path !== snapshot.search_projection.path ||
    search.generated_at !== snapshot.generated_at
  ) {
    throw new Error('Discovery projection mismatch');
  }
  if (
    search.records.length !== snapshot.records.length ||
    search.records.length !== snapshot.search_projection.record_count
  ) {
    throw new Error('Discovery record count mismatch');
  }
  if (
    (await sha256Digest(bytes.snapshot)) !== envelope.snapshot_digest ||
    (await sha256Digest(bytes.search)) !== snapshot.search_projection.digest
  ) {
    throw new Error('Discovery digest mismatch');
  }
  const publicKey = decodeBase64(trust.publicKeyBase64);
  const signature = decodeBase64(envelope.signature);
  if (publicKey.length !== 32 || signature.length !== 64)
    throw new Error('Discovery key or signature has an invalid length');
  const key = await crypto.subtle.importKey('raw', publicKey, { name: 'Ed25519' }, false, [
    'verify',
  ]);
  if (!(await crypto.subtle.verify('Ed25519', key, signature, signedMessage(signatureDomain, bytes.snapshot)))) {
    throw new Error('Discovery signature is invalid');
  }
  for (let index = 0; index < snapshot.records.length; index += 1) {
    const {
      author: _author,
      first_seen: _firstSeen,
      last_seen: _lastSeen,
      ...compact
    } = snapshot.records[index]!;
    if (canonicalJSON(compact, 'Discovery') !== canonicalJSON(search.records[index], 'Discovery'))
      throw new Error('Discovery search record mismatch');
  }
  const generated = parseUTCTimestamp(snapshot.generated_at, 'Discovery');
  const expires = parseUTCTimestamp(snapshot.expires_at, 'Discovery');
  if (now.getTime() < generated.getTime())
    throw new Error('Discovery local clock is before generation time');
  if (now.getTime() >= expires.getTime()) throw new Error('Discovery snapshot is stale');
  return { pointer, envelope, snapshot, search, bytes, etags, source: 'cache' };
}

export function discoveryPlugin(
  record: DiscoveryRecord,
  snapshot: DiscoverySnapshot,
): RegistryPlugin {
  const components = (Object.entries(record.components) as Array<[ComponentID, number]>)
    .filter(([, count]) => count > 0)
    .map(([component]) => component);
  const targets = record.compatible_clients.map((client) => ({
    client,
    authentication: record.authentication,
    delivery: 'managed' as const,
    scopes: ['user'],
  }));
  const installable =
    record.availability === 'available' && components.length > 0 && targets.length > 0;
  const revision = record.revision;
  const source = {
    repository: record.repository,
    revision,
    path: record.package_path,
    manifest_sha256: record.manifest_digest,
    tree_sha256: record.tree_digest,
  };
  const release = {
    release_sequence: snapshot.sequence,
    source,
    version: record.version ?? 'unknown',
    targets,
    components,
    evidence: [],
    package_evidence: [],
    release_status: 'active' as const,
    selectable: installable,
    blocking_clients: [],
    materialized_clients: [],
    meets_minimum_capabilities: true,
  };
  return {
    name: record.name,
    display_name: record.name,
    version: record.version ?? 'unknown',
    description: record.description || `Agent Plugins 1.0 package from ${record.repository}`,
    author: record.author ?? { name: record.owner, url: `https://github.com/${record.owner}` },
    license: record.license ?? 'Not specified',
    categories: [],
    keywords: [record.owner, record.repository, ...record.mcp_transports],
    source,
    install_source: record.slug,
    built_in: false,
    installable,
    components,
    default_distribution: record.slug,
    declared_default_distribution: record.slug,
    distributions: [
      {
        id: record.slug,
        kind: 'direct',
        label: 'Unreviewed discovery source',
        publisher: record.owner,
        source,
        release_sequence: snapshot.sequence,
        version: release.version,
        compatible_clients: record.compatible_clients,
        evidence: [],
        package_evidence: [],
        status: record.availability === 'available' ? 'active' : 'suspended',
        release_status: 'active',
        selectable: installable,
        targets,
        components,
        releases: [release],
      },
    ],
    evidence: [],
    package_evidence: [],
    authentication: record.authentication === 'not_required' ? 'none' : 'unknown',
    client_support: {
      resolution: 'install_time',
      clients: record.compatible_clients,
      delivery: {},
      scopes: {},
      app_bindings: {},
    },
    trust_state: 'conformant_unreviewed',
    discovery: {
      sequence: snapshot.sequence,
      generated_at: snapshot.generated_at,
      expires_at: snapshot.expires_at,
      repository_updated_at: record.repository_updated_at,
      stars: record.stars,
      schema_version: record.schema_version,
      manifest_digest: record.manifest_digest,
      tree_digest: record.tree_digest,
      mcp_transports: record.mcp_transports,
      availability: record.availability,
      ...(record.reviewed_distribution_id
        ? { reviewed_distribution_id: record.reviewed_distribution_id }
        : {}),
    },
  };
}

function parsePointer(bytes: Uint8Array): DiscoveryPointer {
  const value = parseCanonicalJSON<DiscoveryPointer>(bytes, 'Discovery', 'pointer');
  assertExactKeys(
    value,
    [
      'pointer_schema_version',
      'snapshot_schema_version',
      'sequence',
      'snapshot_path',
      'envelope_path',
      'search_path',
      'fetch_contract',
    ],
    'Discovery',
    'pointer',
  );
  assertExactKeys(
    value.fetch_contract,
    [
      'max_redirects',
      'latest_max_bytes',
      'snapshot_max_bytes',
      'envelope_max_bytes',
      'search_max_bytes',
      'retry_attempts',
    ],
    'Discovery',
    'fetch contract',
  );
  requirePublicSequence(value.sequence, 'pointer');
  const stem = String(value.sequence).padStart(20, '0');
  const contract = value.fetch_contract;
  if (
    value.pointer_schema_version !== 1 ||
    value.snapshot_schema_version !== 1 ||
    value.snapshot_path !== `snapshots/${stem}.json` ||
    value.envelope_path !== `snapshots/${stem}.envelope.json` ||
    value.search_path !== `search/${stem}.json` ||
    contract.max_redirects !== 0 ||
    contract.retry_attempts < 1 ||
    contract.retry_attempts > 3 ||
    contract.latest_max_bytes < 1 ||
    contract.latest_max_bytes > 16 << 10 ||
    contract.snapshot_max_bytes < 1 ||
    contract.snapshot_max_bytes > 16 << 20 ||
    contract.envelope_max_bytes < 1 ||
    contract.envelope_max_bytes > 16 << 10 ||
    contract.search_max_bytes < 1 ||
    contract.search_max_bytes > 10 << 20
  ) {
    throw new Error('Discovery pointer is invalid');
  }
  return value;
}

function assertEnvelope(value: DiscoveryEnvelope, trust: DiscoveryTrust) {
  assertExactKeys(
    value,
    [
      'envelope_schema_version',
      'snapshot_schema_version',
      'sequence',
      'key_id',
      'algorithm',
      'signature_domain',
      'snapshot_digest',
      'signature',
    ],
    'Discovery',
    'envelope',
  );
  requirePublicSequence(value.sequence, 'envelope');
  if (
    value.envelope_schema_version !== 1 ||
    value.snapshot_schema_version !== 1 ||
    value.key_id !== trust.keyID ||
    value.algorithm !== 'Ed25519' ||
    value.signature_domain !== 'UAP-DISCOVERY-INDEX-ED25519-V1' ||
    !digestPattern.test(value.snapshot_digest) ||
    !/^[A-Za-z0-9+/]{86}==$/.test(value.signature)
  ) {
    throw new Error('Discovery envelope is invalid');
  }
}

function assertSnapshot(value: DiscoverySnapshot) {
  assertExactKeys(
    value,
    [
      'discovery_schema_version',
      'sequence',
      'publication_id',
      'source_commit',
      'generated_at',
      'expires_at',
      'complete',
      'query_manifest_digest',
      'partitions',
      'search_projection',
      'records',
    ],
    'Discovery',
    'snapshot',
  );
  assertExactKeys(
    value.search_projection,
    ['path', 'digest', 'record_count'],
    'Discovery',
    'search projection',
  );
  requirePublicSequence(value.sequence, 'snapshot');
  if (
    value.discovery_schema_version !== 1 ||
    !value.complete ||
    !/^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$/.test(value.publication_id) ||
    !revisionPattern.test(value.source_commit) ||
    !digestPattern.test(value.query_manifest_digest) ||
    value.search_projection.path !== `search/${String(value.sequence).padStart(20, '0')}.json` ||
    !digestPattern.test(value.search_projection.digest) ||
    value.search_projection.record_count !== value.records.length ||
    value.records.length > 10_000
  )
    throw new Error('Discovery snapshot is invalid');
  const generated = parseUTCTimestamp(value.generated_at, 'Discovery');
  const expires = parseUTCTimestamp(value.expires_at, 'Discovery');
  if (expires <= generated || expires.getTime() - generated.getTime() > 7 * 86_400_000)
    throw new Error('Discovery lifetime is invalid');
  value.partitions.forEach((partition) => {
    assertExactKeys(
      partition,
      ['query', 'size_min', 'size_max', 'total_count'],
      'Discovery',
      'partition',
    );
    if (
      !partition.query ||
      !integer(partition.size_min) ||
      !integer(partition.size_max) ||
      !integer(partition.total_count) ||
      partition.size_min < 0 ||
      partition.size_max < partition.size_min ||
      partition.total_count < 0 ||
      partition.total_count > 1000
    ) {
      throw new Error('Discovery partition is invalid');
    }
  });
  assertRecords(value.records, true, generated);
}

function assertSearch(value: DiscoverySearch) {
  assertExactKeys(
    value,
    ['search_schema_version', 'sequence', 'generated_at', 'records'],
    'Discovery',
    'search',
  );
  requirePublicSequence(value.sequence, 'search');
  if (value.search_schema_version !== 1 || value.records.length > 10_000) {
    throw new Error('Discovery search projection is invalid');
  }
  assertRecords(value.records, false, parseUTCTimestamp(value.generated_at, 'Discovery'));
}

function assertRecords(records: DiscoveryRecord[], full: boolean, generated: Date) {
  const identities = new Set<string>();
  const slugs = new Set<string>();
  let prior = '';
  records.forEach((record) => {
    const commonFields = [
      'slug',
      'name',
      'description',
      'owner',
      'repository',
      'package_path',
      'revision',
      'version',
      'license',
      'schema_version',
      'components',
      'mcp_transports',
      'compatible_clients',
      'authentication',
      'status',
      'runtime_reviewed',
      'tree_digest',
      'manifest_digest',
      'stars',
      'repository_updated_at',
      'reviewed_distribution_id',
      'availability',
    ];
    assertExactKeys(
      record,
      full ? [...commonFields, 'author', 'first_seen', 'last_seen'] : commonFields,
      'Discovery',
      'record',
    );
    assertExactKeys(
      record.components,
      ['extensions', 'mcp', 'skills'],
      'Discovery',
      'components',
    );
    if (full && record.author) {
      const authorKeys = Object.keys(record.author);
      if (
        !authorKeys.includes('name') ||
        authorKeys.some((key) => !['name', 'email', 'url'].includes(key))
      )
        throw new Error('Discovery author is invalid');
    }
    const expectedSlug = `discovery:${record.repository}${record.package_path ? `//${record.package_path}` : ''}`;
    const identity = `${record.repository}\0${record.package_path.toLocaleLowerCase()}`;
    const order = `${record.repository}\0${record.package_path.toLocaleLowerCase()}\0${record.slug}`;
    if (
      !repositoryPattern.test(record.repository) ||
      record.owner !== record.repository.split('/')[0] ||
      !packagePathPattern.test(record.package_path) ||
      !revisionPattern.test(record.revision) ||
      record.slug !== expectedSlug ||
      identities.has(identity) ||
      slugs.has(record.slug) ||
      order < prior ||
      record.schema_version !== '1.0.0' ||
      record.status !== 'conformant_unreviewed' ||
      record.runtime_reviewed !== false ||
      !digestPattern.test(record.tree_digest) ||
      !digestPattern.test(record.manifest_digest) ||
      !integer(record.stars) ||
      record.stars < 0 ||
      (record.version !== null && typeof record.version !== 'string') ||
      (record.license !== null && typeof record.license !== 'string') ||
      (record.reviewed_distribution_id !== null &&
        typeof record.reviewed_distribution_id !== 'string') ||
      !['available', 'unavailable'].includes(record.availability) ||
      record.name.length < 1 ||
      [...record.name].length > 64 ||
      [...record.description].length > 500 ||
      !Object.values(record.components).every((count) => integer(count) && count >= 0) ||
      !orderedEnums(record.compatible_clients, clientOrder) ||
      !orderedEnums(record.mcp_transports, transportOrder) ||
      !['not_required', 'required', 'unknown'].includes(record.authentication)
    )
      throw new Error(`Discovery record ${record.slug} is invalid`);
    parseUTCTimestamp(record.repository_updated_at, 'Discovery');
    if (full) {
      const first = parseUTCTimestamp(record.first_seen ?? '', 'Discovery');
      const last = parseUTCTimestamp(record.last_seen ?? '', 'Discovery');
      if (first > last || last > generated || (record.author && !record.author.name.trim()))
        throw new Error('Discovery seen interval is invalid');
    } else if (
      record.author !== undefined ||
      record.first_seen !== undefined ||
      record.last_seen !== undefined
    ) {
      throw new Error('Discovery search contains snapshot-only metadata');
    }
    identities.add(identity);
    slugs.add(record.slug);
    prior = order;
  });
}

function orderedEnums(values: readonly string[], allowed: readonly string[]) {
  if (!Array.isArray(values)) return false;
  let previous = -1;
  for (const value of values) {
    const position = allowed.indexOf(value);
    if (position <= previous) return false;
    previous = position;
  }
  return true;
}

function integer(value: number) {
  return Number.isSafeInteger(value);
}

function requirePublicSequence(value: number, label: string) {
  if (!Number.isSafeInteger(value) || value < 1)
    throw new Error(`Discovery ${label} sequence is invalid`);
}
