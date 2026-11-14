import { createHash, generateKeyPairSync, sign } from 'node:crypto';
import type { DiscoveryRecord, DiscoverySnapshot } from '../../types/discovery.ts';
import { canonicalJSON, signedMessage } from '../../utils/signedFeed.ts';

const keys = generateKeyPairSync('ed25519');
export const discoveryTrust = {
  keyID: 'display-test',
  publicKeyBase64: keys.publicKey
    .export({ format: 'der', type: 'spki' })
    .subarray(-32)
    .toString('base64'),
};
const encode = (value: unknown) => new TextEncoder().encode(canonicalJSON(value, 'Discovery'));
const digest = (value: Uint8Array) => `sha256:${createHash('sha256').update(value).digest('hex')}`;

export function signedDiscoveryFixture({
  sequence = 80,
  count = 1,
  generated = '2026-10-03T06:49:08Z',
  expires = '2026-10-04T06:49:08Z',
  description = 'Signed display fixture',
} = {}) {
  const records: DiscoveryRecord[] = Array.from({ length: count }, (_, index) => {
    const repository = `test/plugin-${String(index).padStart(5, '0')}`;
    return {
      slug: `discovery:${repository}`,
      name: `plugin-${index}`,
      description,
      owner: 'test',
      repository,
      package_path: '',
      revision: 'a'.repeat(40),
      version: '1.0.0',
      license: 'MIT',
      schema_version: '1.0.0',
      components: { extensions: 0, mcp: 0, skills: 1 },
      mcp_transports: [],
      compatible_clients: ['codex'],
      authentication: 'not_required',
      status: 'conformant_unreviewed',
      runtime_reviewed: false,
      tree_digest: `sha256:${'1'.repeat(64)}`,
      manifest_digest: `sha256:${'2'.repeat(64)}`,
      stars: 1,
      repository_updated_at: generated,
      reviewed_distribution_id: null,
      availability: 'available',
      author: null,
      first_seen: generated,
      last_seen: generated,
    };
  });
  const stem = String(sequence).padStart(20, '0');
  const search = encode({
    search_schema_version: 1,
    sequence,
    generated_at: generated,
    records: records.map(
      ({ author: _author, first_seen: _first, last_seen: _last, ...record }) => record,
    ),
  });
  const snapshot: DiscoverySnapshot = {
    discovery_schema_version: 1,
    sequence,
    publication_id: `display-${sequence}`,
    source_commit: 'a'.repeat(40),
    generated_at: generated,
    expires_at: expires,
    complete: true,
    query_manifest_digest: `sha256:${'3'.repeat(64)}`,
    partitions: [],
    search_projection: { path: `search/${stem}.json`, digest: digest(search), record_count: count },
    records,
  };
  const snapshotBytes = encode(snapshot);
  const envelope = encode({
    envelope_schema_version: 1,
    snapshot_schema_version: 1,
    sequence,
    key_id: discoveryTrust.keyID,
    algorithm: 'Ed25519',
    signature_domain: 'UAP-DISCOVERY-INDEX-ED25519-V1',
    snapshot_digest: digest(snapshotBytes),
    signature: sign(
      null,
      signedMessage('UAP-DISCOVERY-INDEX-ED25519-V1', snapshotBytes),
      keys.privateKey,
    ).toString('base64'),
  });
  const pointer = encode({
    pointer_schema_version: 1,
    snapshot_schema_version: 1,
    sequence,
    snapshot_path: `snapshots/${stem}.json`,
    envelope_path: `snapshots/${stem}.envelope.json`,
    search_path: `search/${stem}.json`,
    fetch_contract: {
      max_redirects: 0,
      latest_max_bytes: 16 << 10,
      snapshot_max_bytes: 8 << 20,
      envelope_max_bytes: 16 << 10,
      search_max_bytes: 8 << 20,
      retry_attempts: 3,
    },
  });
  return { bytes: { pointer, snapshot: snapshotBytes, envelope, search }, snapshot };
}

export function discoveryFetcher(data: ReturnType<typeof signedDiscoveryFixture>): typeof fetch {
  return async (input) => {
    const path = new URL(String(input)).pathname;
    const value = path.endsWith('/latest.json')
      ? data.bytes.pointer
      : path.includes('/search/')
        ? data.bytes.search
        : path.endsWith('.envelope.json')
          ? data.bytes.envelope
          : data.bytes.snapshot;
    return new Response(value, { headers: { etag: `"${data.snapshot.sequence}"` } });
  };
}
