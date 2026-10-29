import assert from 'node:assert/strict';
import { describe, it } from 'node:test';
import { readFileSync } from 'node:fs';
import type {
  SkillSpectorManifest,
  SkillSpectorRecord,
  SkillSpectorShardDescriptor,
} from '../types/skillSpector.ts';
import { skillSpectorAssessment } from '../utils/skillSpector.ts';
import {
  createSkillSpectorClient,
  decodeSkillSpectorManifest,
  decodeSkillSpectorShard,
  skillSpectorV2ProfileDigest,
} from '../utils/skillSpectorCatalog.ts';

const fixture = JSON.parse(
  readFileSync(new URL('./fixtures/skillspector-report.json', import.meta.url), 'utf8'),
);
const manifestUrl = 'https://example.test/v2/index.json';
const legacyUrl = 'https://example.test/v1/index.json';
const generatedAt = new Date().toISOString().replace(/\.\d{3}Z$/, 'Z');
async function catalog(repositories = ['owner/one', 'owner/two']) {
  const bodies = new Map<string, string>();
  const descriptors: SkillSpectorShardDescriptor[] = [];
  for (const [index, repository] of repositories.entries()) {
    const record: SkillSpectorRecord = structuredClone(fixture.records[0]);
    record.id = `record-${index}`;
    record.kind = 'standalone_skill';
    record.plugin_path = null;
    record.source.repository = repository;
    const shard = {
      schema_version: 2,
      scanner: fixture.scanner,
      profile: { id: 'static-only-v2', digest: skillSpectorV2ProfileDigest },
      repository,
      records: [record],
    };
    const body = JSON.stringify(shard);
    const bytes = new TextEncoder().encode(body);
    const hash = Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256', bytes)), (byte) =>
      byte.toString(16).padStart(2, '0'),
    ).join('');
    const path = `shards/${hash}.json`;
    bodies.set(new URL(path, manifestUrl).href, body);
    descriptors.push({
      repository,
      path,
      digest: `sha256:${hash}`,
      bytes: bytes.byteLength,
      record_count: 1,
    });
  }
  const manifest: SkillSpectorManifest = {
    schema_version: 2,
    generated_at: generatedAt,
    source_commit: 'a'.repeat(40),
    scanner: fixture.scanner,
    profile: { id: 'static-only-v2', digest: skillSpectorV2ProfileDigest },
    scope: 'selected_sources',
    selection_digest: `sha256:${'b'.repeat(64)}`,
    coverage: { selected: repositories.length + 5, reported: repositories.length, pending: 5 },
    shards: descriptors,
    plugin_coverage: [],
  };
  bodies.set(manifestUrl, JSON.stringify(manifest));
  return { manifest, bodies };
}
async function updateShard(
  data: Awaited<ReturnType<typeof catalog>>,
  index: number,
  update: (value: any) => void,
) {
  const descriptor = data.manifest.shards[index]!;
  const oldUrl = new URL(descriptor.path, manifestUrl).href;
  const shard = JSON.parse(data.bodies.get(oldUrl)!);
  update(shard);
  const body = JSON.stringify(shard);
  const bytes = new TextEncoder().encode(body);
  const hash = Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256', bytes)), (byte) =>
    byte.toString(16).padStart(2, '0'),
  ).join('');
  data.manifest.shards[index] = {
    ...descriptor,
    path: `shards/${hash}.json`,
    digest: `sha256:${hash}`,
    bytes: bytes.byteLength,
  };
  data.bodies.set(new URL(data.manifest.shards[index]!.path, manifestUrl).href, body);
  data.bodies.set(manifestUrl, JSON.stringify(data.manifest));
}

function fetchFrom(bodies: Map<string, string>, requests: string[] = []) {
  return (async (input: string | URL | Request) => {
    const url = String(input);
    requests.push(url);
    const body = bodies.get(url);
    return new Response(body ?? '', { status: body === undefined ? 404 : 200 });
  }) as typeof fetch;
}

describe('bounded SkillSpector repository catalog', () => {
  it('validates explicit coverage, immutable relative shard paths and pinned metadata', async () => {
    const { manifest } = await catalog();
    assert.equal(decodeSkillSpectorManifest(manifest).coverage.pending, 5);
    for (const change of [
      (value: SkillSpectorManifest) => {
        value.plugin_coverage = [
          {
            repository: 'owner/one',
            plugin_path: '../outside',
            selected: 1,
            reported: 0,
            pending: 1,
          },
        ];
      },
      (value: SkillSpectorManifest) => {
        value.plugin_coverage = [
          { repository: 'owner/one', plugin_path: '', selected: 0, reported: 0, pending: 0 },
        ];
      },
      (value: SkillSpectorManifest) => {
        value.plugin_coverage = [
          { repository: 'owner/one', plugin_path: '', selected: 6, reported: 0, pending: 6 },
        ];
      },
      (value: SkillSpectorManifest) => {
        value.coverage.pending++;
      },
      (value: SkillSpectorManifest) => {
        value.coverage.selected = 10_001;
      },
      (value: SkillSpectorManifest) => {
        value.profile.digest = `sha256:${'0'.repeat(64)}`;
      },
      (value: SkillSpectorManifest) => {
        value.scanner.revision = '0'.repeat(40);
      },
      (value: SkillSpectorManifest) => {
        value.shards[0]!.path = '../outside.json';
      },
      (value: SkillSpectorManifest) => {
        value.shards[0]!.bytes = (8 << 20) + 1;
      },
      (value: SkillSpectorManifest) => {
        value.shards[0]!.record_count = 257;
      },
      (value: SkillSpectorManifest) => {
        value.shards.push(value.shards[0]!);
      },
      (value: SkillSpectorManifest) => {
        value.generated_at = '2026-02-30T00:00:00Z';
      },
    ]) {
      const malformed = structuredClone(manifest);
      change(malformed);
      assert.throws(() => decodeSkillSpectorManifest(malformed), /invalid/);
    }
    const empty = { ...manifest, shards: [], coverage: { selected: 5, reported: 0, pending: 5 } };
    assert.equal(decodeSkillSpectorManifest(empty).shards.length, 0);
  });

  it('accepts 10,000 single-root repository descriptors within the total record bound', async () => {
    const { manifest } = await catalog();
    manifest.coverage = { selected: 10_000, reported: 10_000, pending: 0 };
    manifest.shards = Array.from({ length: 10_000 }, (_, index) => {
      const hash = index.toString(16).padStart(64, '0');
      return {
        repository: `owner/repo-${index}`,
        path: `shards/${hash}.json`,
        digest: `sha256:${hash}`,
        bytes: 100,
        record_count: 1,
      };
    });
    assert.equal(decodeSkillSpectorManifest(manifest).shards.length, 10_000);
    manifest.shards.push({ ...manifest.shards[0]! });
    assert.throws(() => decodeSkillSpectorManifest(manifest), /invalid/);
  });

  it('rejects aggregate shard bytes above the 256 MiB catalog bound', async () => {
    const { manifest } = await catalog();
    manifest.coverage = { selected: 33, reported: 33, pending: 0 };
    manifest.shards = Array.from({ length: 33 }, (_, index) => {
      const hash = index.toString(16).padStart(64, '0');
      return {
        repository: `owner/repo-${index}`,
        path: `shards/${hash}.json`,
        digest: `sha256:${hash}`,
        bytes: 8 << 20,
        record_count: 1,
      };
    });
    assert.throws(() => decodeSkillSpectorManifest(manifest), /invalid/);
    manifest.shards.pop();
    manifest.coverage = { selected: 32, reported: 32, pending: 0 };
    assert.equal(decodeSkillSpectorManifest(manifest).shards.length, 32);
  });

  it('rejects mismatched shard repository and incomplete record evidence', async () => {
    const { manifest, bodies } = await catalog();
    const descriptor = manifest.shards[0]!;
    const shard = JSON.parse(bodies.get(new URL(descriptor.path, manifestUrl).href)!);
    assert.equal(decodeSkillSpectorShard(shard, manifest, descriptor).length, 1);
    for (const change of [
      (value: typeof shard) => {
        value.repository = 'other/repository';
      },
      (value: typeof shard) => {
        value.records[0].source.repository = 'other/repository';
      },
      (value: typeof shard) => {
        value.records[0].source.content_digest = null;
      },
      (value: typeof shard) => {
        value.records[0].scanned_at = '2099-01-01T00:00:00Z';
      },
      (value: typeof shard) => {
        value.profile.id = 'static-only-v1';
      },
    ]) {
      const malformed = structuredClone(shard);
      change(malformed);
      assert.throws(() => decodeSkillSpectorShard(malformed, manifest, descriptor), /invalid/);
    }
  });

  it('shares requests, loads only requested repositories, and leaves pending subjects neutral', async () => {
    const { bodies } = await catalog();
    const requests: string[] = [];
    const client = createSkillSpectorClient(manifestUrl, legacyUrl, fetchFrom(bodies, requests));
    const [first, same] = await Promise.all([
      client.reportFor('OWNER/one'),
      client.reportFor('owner/one'),
    ]);
    assert.equal(first, same);
    assert.equal(first.records.length, 1);
    assert.equal(requests.length, 2);
    assert.equal((await client.reportFor('not/selected')).records.length, 0);
    assert.equal(requests.length, 2);
  });

  it('isolates a tampered or failed shard without downgrading to v1', async () => {
    const { manifest, bodies } = await catalog();
    bodies.set(new URL(manifest.shards[0]!.path, manifestUrl).href, 'tampered');
    const requests: string[] = [];
    const client = createSkillSpectorClient(manifestUrl, legacyUrl, fetchFrom(bodies, requests));
    await assert.rejects(client.reportFor('owner/one'), /invalid/);
    assert.equal((await client.reportFor('owner/two')).records.length, 1);
    assert.equal(requests.includes(legacyUrl), false);
  });

  it('rejects duplicated immutable subjects across repository parts even under different IDs', async () => {
    const { bodies } = await catalog(['owner/one', 'owner/one']);
    await assert.rejects(
      createSkillSpectorClient(manifestUrl, legacyUrl, fetchFrom(bodies)).reportFor('owner/one'),
      /invalid/,
    );
  });

  it('waits for every repository part before exposing any assessment records', async () => {
    const { manifest, bodies } = await catalog(['owner/one', 'owner/one']);
    const oldUrl = new URL(manifest.shards[1]!.path, manifestUrl).href;
    const second = JSON.parse(bodies.get(oldUrl)!);
    second.records[0].source.path += '-other';
    const body = JSON.stringify(second);
    const bytes = new TextEncoder().encode(body);
    const hash = Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256', bytes)), (byte) =>
      byte.toString(16).padStart(2, '0'),
    ).join('');
    manifest.shards[1] = {
      repository: 'owner/one',
      path: `shards/${hash}.json`,
      digest: `sha256:${hash}`,
      bytes: bytes.byteLength,
      record_count: 1,
    };
    const secondUrl = new URL(manifest.shards[1]!.path, manifestUrl).href;
    bodies.set(secondUrl, body);
    bodies.set(manifestUrl, JSON.stringify(manifest));
    let release!: () => void;
    const blocked = new Promise<void>((resolve) => {
      release = resolve;
    });
    let firstRead = false;
    const fetcher = (async (input: string | URL | Request) => {
      const url = String(input);
      if (url === secondUrl) await blocked;
      else if (url !== manifestUrl) firstRead = true;
      return new Response(bodies.get(url));
    }) as typeof fetch;
    let settled = false;
    const pending = createSkillSpectorClient(manifestUrl, legacyUrl, fetcher)
      .reportFor('owner/one')
      .then((report) => {
        settled = true;
        return report;
      });
    while (!firstRead) await new Promise((resolve) => setImmediate(resolve));
    await new Promise((resolve) => setImmediate(resolve));
    assert.equal(settled, false);
    release();
    assert.equal((await pending).records.length, 2);
  });

  it('never presents one of thirteen selected plugin roots as a complete clean assessment', async () => {
    const data = await catalog(['owner/one']);
    data.manifest.coverage = { selected: 13, reported: 1, pending: 12 };
    data.manifest.plugin_coverage = [
      {
        repository: 'owner/one',
        plugin_path: 'plugins/example',
        selected: 13,
        reported: 1,
        pending: 12,
      },
    ];
    await updateShard(data, 0, (shard) => {
      shard.records[0].kind = 'plugin_skill';
      shard.records[0].plugin_path = 'plugins/example';
      shard.records[0].source.path = 'plugins/example/skills/example';
    });
    const report = await createSkillSpectorClient(
      manifestUrl,
      legacyUrl,
      fetchFrom(data.bodies),
    ).reportFor('owner/one');
    const subject = {
      kind: 'plugin' as const,
      repository: 'owner/one',
      path: 'plugins/example',
      revision: report.records[0]!.source.revision,
    };
    const assessment = skillSpectorAssessment(report, subject);
    assert.equal(assessment.status, 'partial');
    assert.equal(assessment.coverage?.pending, 12);
    const empty = {
      ...report,
      records: [],
      plugin_coverage: [{ ...data.manifest.plugin_coverage[0]!, reported: 0, pending: 13 }],
    };
    assert.equal(skillSpectorAssessment(empty, subject).status, 'notChecked');
    const failed = {
      ...report,
      records: report.records.map((record) => ({
        ...record,
        status: 'error' as const,
        errors: [{ code: 'scan_failed', message: 'Failed' }],
      })),
    };
    assert.equal(skillSpectorAssessment(failed, subject).status, 'error');
  });

  it('rejects a clean plugin record whose scanned root does not belong to that plugin', async () => {
    for (const path of [
      'skills/unrelated',
      'plugins/other/skills/example',
      'plugins/example/skills/',
      'plugins/example/skills/nested/child',
    ]) {
      const data = await catalog(['owner/one']);
      data.manifest.plugin_coverage = [
        {
          repository: 'owner/one',
          plugin_path: 'plugins/example',
          selected: 1,
          reported: 1,
          pending: 0,
        },
      ];
      await updateShard(data, 0, (shard) => {
        shard.records[0].kind = 'plugin_skill';
        shard.records[0].plugin_path = 'plugins/example';
        shard.records[0].source.path = path;
      });
      await assert.rejects(
        createSkillSpectorClient(manifestUrl, legacyUrl, fetchFrom(data.bodies)).reportFor(
          'owner/one',
        ),
        /invalid/,
      );
    }
    const root = await catalog(['owner/one']);
    root.manifest.plugin_coverage = [
      { repository: 'owner/one', plugin_path: '', selected: 1, reported: 1, pending: 0 },
    ];
    await updateShard(root, 0, (shard) => {
      shard.records[0].kind = 'plugin_skill';
      shard.records[0].plugin_path = '';
      shard.records[0].source.path = 'skills/example';
    });
    assert.equal(
      (
        await createSkillSpectorClient(manifestUrl, legacyUrl, fetchFrom(root.bodies)).reportFor(
          'owner/one',
        )
      ).records.length,
      1,
    );
  });

  it('requires plugin coverage counts to agree with all loaded repository records', async () => {
    const data = await catalog(['owner/one']);
    data.manifest.coverage = { selected: 13, reported: 1, pending: 12 };
    data.manifest.plugin_coverage = [
      {
        repository: 'owner/one',
        plugin_path: 'plugins/example',
        selected: 13,
        reported: 0,
        pending: 13,
      },
    ];
    await updateShard(data, 0, (shard) => {
      shard.records[0].kind = 'plugin_skill';
      shard.records[0].plugin_path = 'plugins/example';
      shard.records[0].source.path = 'plugins/example/skills/example';
    });
    await assert.rejects(
      createSkillSpectorClient(manifestUrl, legacyUrl, fetchFrom(data.bodies)).reportFor(
        'owner/one',
      ),
      /invalid/,
    );
    data.manifest.plugin_coverage = [];
    data.bodies.set(manifestUrl, JSON.stringify(data.manifest));
    await assert.rejects(
      createSkillSpectorClient(manifestUrl, legacyUrl, fetchFrom(data.bodies)).reportFor(
        'owner/one',
      ),
      /invalid/,
    );
  });

  it('falls back only when the v2 manifest is missing, preserving strict legacy bounds', async () => {
    for (const status of [404, 500]) {
      const requests: string[] = [];
      const fetcher = (async (input: string | URL | Request) => {
        requests.push(String(input));
        return String(input) === manifestUrl
          ? new Response('', { status })
          : new Response(JSON.stringify(fixture));
      }) as typeof fetch;
      const client = createSkillSpectorClient(manifestUrl, legacyUrl, fetcher);
      if (status === 404) assert.equal((await client.reportFor('owner/one')).schema_version, 1);
      else await assert.rejects(client.reportFor('owner/one'), /500/);
      assert.equal(requests.includes(legacyUrl), status === 404);
    }
  });

  it('bounds concurrent shard fetches to four', async () => {
    const { bodies } = await catalog(
      Array.from({ length: 9 }, (_, index) => `owner/repo-${index}`),
    );
    let active = 0,
      peak = 0;
    const releases: (() => void)[] = [];
    const fetcher = (async (input: string | URL | Request) => {
      const url = String(input);
      if (url !== manifestUrl) {
        active++;
        peak = Math.max(active, peak);
        await new Promise<void>((resolve) =>
          releases.push(() => {
            active--;
            resolve();
          }),
        );
      }
      return new Response(bodies.get(url));
    }) as typeof fetch;
    const client = createSkillSpectorClient(manifestUrl, legacyUrl, fetcher);
    const all = Promise.all(
      Array.from({ length: 9 }, (_, index) => client.reportFor(`owner/repo-${index}`)),
    );
    while (releases.length < 4) await new Promise((resolve) => setImmediate(resolve));
    assert.equal(peak, 4);
    for (let completed = 0; completed < 9;) {
      const release = releases.shift();
      if (release) {
        release();
        completed++;
      }
      await new Promise((resolve) => setImmediate(resolve));
    }
    assert.equal((await all).length, 9);
    assert.equal(peak, 4);
  });

  it('cancels a response immediately when its declared content length exceeds the bound', async () => {
    let cancelled = false;
    const fetcher = (async () =>
      new Response(
        new ReadableStream({
          cancel() {
            cancelled = true;
          },
        }),
        { headers: { 'content-length': String((4 << 20) + 1) } },
      )) as typeof fetch;
    await assert.rejects(
      createSkillSpectorClient(manifestUrl, legacyUrl, fetcher).reportFor('owner/one'),
      /invalid/,
    );
    assert.equal(cancelled, true);
  });

  it('cancels oversized streams before allocating a complete manifest', async () => {
    let cancelled = false;
    const fetcher = (async () =>
      new Response(
        new ReadableStream({
          pull(controller) {
            controller.enqueue(new Uint8Array((4 << 20) + 1));
          },
          cancel() {
            cancelled = true;
          },
        }),
      )) as typeof fetch;
    await assert.rejects(
      createSkillSpectorClient(manifestUrl, legacyUrl, fetcher).reportFor('owner/one'),
      /invalid/,
    );
    assert.equal(cancelled, true);
  });
});
