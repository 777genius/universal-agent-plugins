import assert from 'node:assert/strict';
import { createHash, generateKeyPairSync, sign } from 'node:crypto';
import { describe, it } from 'node:test';
import type { RegistryPlugin } from '../types/registry.ts';
import {
  applySecurityAssessment,
  loadSecurity,
  lookupSecurity,
  verifySecurity,
} from '../utils/security.ts';
import { canonicalJSON, signedMessage } from '../utils/signedFeed.ts';

const encoder = new TextEncoder();
const now = new Date('2026-09-06T00:00:00Z');
const tree = `sha256:${'1'.repeat(64)}`;
const manifest = `sha256:${'2'.repeat(64)}`;

function bytes(value: unknown) {
  return encoder.encode(canonicalJSON(value, 'Security Index'));
}

function digest(value: Uint8Array) {
  return `sha256:${createHash('sha256').update(value).digest('hex')}`;
}

function fixture(sequence = 7) {
  const record = {
    subject: { tree_digest: tree, manifest_digest: manifest },
    outcome: 'warnings' as const,
    counts: { blocking: 0, warnings: 1, total: 1 },
    scanned_files: 2,
    report_digest: `sha256:${'3'.repeat(64)}`,
    findings: [
      {
        code: 'SEC301',
        disposition: 'warning' as const,
        severity: 'warn' as const,
        confidence: 'high' as const,
        category: 'security',
        path: 'mcp.json',
        line: 2,
        message: 'Review this endpoint',
      },
    ],
  };
  const snapshot = {
    security_schema_version: 1 as const,
    sequence,
    publication_id: `security-test-${sequence}`,
    source_commit: 'a'.repeat(40),
    generated_at: '2026-09-05T00:00:00Z',
    expires_at: '2026-10-05T00:00:00Z',
    complete: true as const,
    discovery: { sequence: 20, snapshot_digest: `sha256:${'4'.repeat(64)}` },
    scanner: { id: 'lintai', version: '0.1.2' },
    policy: { id: 'agent-plugin-install', version: 1, digest: `sha256:${'5'.repeat(64)}` },
    coverage: { subjects: 1, checked: 1, unavailable: 0 },
    records: [record],
  };
  const snapshotBytes = bytes(snapshot);
  const { privateKey, publicKey } = generateKeyPairSync('ed25519');
  const rawPublicKey = publicKey
    .export({ format: 'der', type: 'spki' })
    .subarray(-32);
  const envelope = {
    envelope_schema_version: 1 as const,
    snapshot_schema_version: 1 as const,
    sequence,
    key_id: 'security-test',
    algorithm: 'Ed25519' as const,
    signature_domain: 'UAP-SECURITY-INDEX-ED25519-V1' as const,
    snapshot_digest: digest(snapshotBytes),
    signature: sign(null, signedMessage('UAP-SECURITY-INDEX-ED25519-V1', snapshotBytes), privateKey).toString('base64'),
  };
  const stem = String(sequence).padStart(20, '0');
  const pointer = {
    pointer_schema_version: 1 as const,
    snapshot_schema_version: 1 as const,
    sequence,
    snapshot_path: `snapshots/${stem}.json`,
    envelope_path: `snapshots/${stem}.envelope.json`,
    fetch_contract: {
      max_redirects: 0,
      latest_max_bytes: 16 << 10,
      snapshot_max_bytes: 8 << 20,
      envelope_max_bytes: 16 << 10,
      retry_attempts: 3,
    },
  };
  return {
    payloads: {
      pointer: bytes(pointer),
      snapshot: snapshotBytes,
      envelope: bytes(envelope),
    },
    trust: {
      keyID: 'security-test',
      publicKeyBase64: rawPublicKey.toString('base64'),
    },
    snapshot,
  };
}

describe('signed public Security Index', () => {
  it('verifies and decorates only the exact package revision', async () => {
    const data = fixture();
    const bundle = await verifySecurity(
      data.payloads.pointer,
      data.payloads.snapshot,
      data.payloads.envelope,
      data.trust,
      now,
    );
    assert.equal(
      lookupSecurity(bundle.snapshot.records, {
        tree_digest: tree,
        manifest_digest: manifest,
      })?.outcome,
      'warnings',
    );
    assert.equal(
      lookupSecurity(bundle.snapshot.records, {
        tree_digest: tree,
        manifest_digest: `sha256:${'f'.repeat(64)}`,
      }),
      undefined,
    );
    const plugin = {
      discovery: { tree_digest: tree, manifest_digest: manifest },
    } as RegistryPlugin;
    const decorated = applySecurityAssessment(plugin, bundle.snapshot);
    assert.equal(decorated.security?.scanner.version, '0.1.2');
    assert.equal(decorated.security?.counts.warnings, 1);
  });

  it('rejects tampering, stale feeds, and inconsistent counts', async () => {
    const data = fixture();
    const tampered = structuredClone(data.payloads);
    tampered.snapshot[tampered.snapshot.length - 2] ^= 1;
    await assert.rejects(
      verifySecurity(
        tampered.pointer,
        tampered.snapshot,
        tampered.envelope,
        data.trust,
        now,
      ),
      /canonical|digest|JSON/,
    );
    await assert.rejects(
      verifySecurity(
        data.payloads.pointer,
        data.payloads.snapshot,
        data.payloads.envelope,
        data.trust,
        new Date('2026-10-05T00:00:00Z'),
      ),
      /stale/,
    );

    const invalid = fixture();
    const snapshot = JSON.parse(new TextDecoder().decode(invalid.payloads.snapshot));
    snapshot.records[0].counts.total = 0;
    await assert.rejects(
      verifySecurity(
        invalid.payloads.pointer,
        bytes(snapshot),
        invalid.payloads.envelope,
        invalid.trust,
        now,
      ),
      /record|counts|digest/,
    );
  });

  it('loads the three bounded artifacts from one origin', async () => {
    const data = fixture();
    const origin = new URL('https://catalog.example/security/');
    const stem = '00000000000000000007';
    const values = new Map([
      [new URL('latest.json', origin).href, data.payloads.pointer],
      [new URL(`snapshots/${stem}.json`, origin).href, data.payloads.snapshot],
      [new URL(`snapshots/${stem}.envelope.json`, origin).href, data.payloads.envelope],
    ]);
    const fetcher: typeof fetch = async (input) => {
      const value = values.get(String(input));
      return value
        ? new Response(value, { status: 200 })
        : new Response('', { status: 404 });
    };
    const bundle = await loadSecurity({ origin, trust: data.trust, fetcher, now });
    assert.equal(bundle.snapshot.sequence, 7);
  });
});
