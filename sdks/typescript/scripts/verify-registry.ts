// Read-only post-publication check of one exact npm release: registry metadata,
// dist-tag, the SLSA provenance binding to this repository's tag, commit and
// workflow, `npm audit signatures`, and the downloaded bytes. It changes nothing
// in the registry; npm commands run in the supplied isolated --workdir.
import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import { mkdir, readFile, writeFile } from 'node:fs/promises';
import { join } from 'node:path';
import { setTimeout as delay } from 'node:timers/promises';
import { parseArgs } from 'node:util';

const REPOSITORY = 'https://github.com/777genius/universal-agent-plugins';
const REGISTRY = 'https://registry.npmjs.org/';
const SLSA = 'https://slsa.dev/provenance/v1';
const names = ['package-name', 'version', 'tag', 'commit', 'workflow', 'directory', 'sha256', 'integrity', 'shasum', 'workdir'] as const;
const { values } = parseArgs({ options: Object.fromEntries(names.map((name) => [name, { type: 'string' }])), strict: true, allowPositionals: false });
const arg = (name: (typeof names)[number]): string => {
  const value = values[name];
  if (typeof value !== 'string' || value === '') throw new Error(`--${name} is required`);
  return value;
};
const [name, version, tag, commit, workflow, directory, sha256, integrity, shasum, workdir] = names.map(arg) as [string, string, string, string, string, string, string, string, string, string];
assert.match(version, /^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$/, 'version must be an exact stable semantic version');
assert.match(commit, /^[0-9a-f]{40}$/, 'commit must be a full lowercase SHA-1');
assert.match(sha256, /^[0-9a-f]{64}$/, 'sha256 must be lowercase hex');
assert.match(shasum, /^[0-9a-f]{40}$/, 'shasum must be lowercase hex');
const sha512 = /^sha512-([A-Za-z0-9+/]{86}==)$/.exec(integrity)?.[1];
if (!sha512) throw new Error('integrity must be a SHA-512 SRI value');

const npm = (args: string[], cwd = workdir) => execFileSync('npm', [...args, `--registry=${REGISTRY}`], { cwd, encoding: 'utf8', maxBuffer: 16 * 1024 * 1024 });
async function retry<T>(label: string, action: () => T | Promise<T>): Promise<T> {
  for (let attempt = 1; ; attempt += 1) {
    try {
      return await action();
    } catch (error) {
      if (attempt === 6) throw new Error(`${label} failed after ${attempt} attempts`, { cause: error });
      await delay(attempt * 5_000);
    }
  }
}
const record = <T>(value: unknown): T => (Array.isArray(value) ? (assert.equal(value.length, 1, 'expected one registry record'), value[0]) : value) as T;

await mkdir(join(workdir, 'packed'), { recursive: true });
await writeFile(join(workdir, 'package.json'), JSON.stringify({ name: 'registry-verification', version: '1.0.0', private: true }));

type Metadata = {
  name: string; version: string; repository: unknown; _npmUser: unknown;
  dist: { tarball: string; integrity: string; shasum: string; attestations: { url: string; provenance: unknown } };
};
const metadata = await retry('npm view', () => record<Metadata>(JSON.parse(npm(['view', `${name}@${version}`, '--json']))));
assert.equal(metadata.name, name);
assert.equal(metadata.version, version);
assert.deepEqual(metadata.repository, { type: 'git', url: `git+${REPOSITORY}.git`, directory });
const filename = `${name.split('/').pop()}-${version}.tgz`;
assert.equal(metadata.dist.tarball, `${REGISTRY}${name}/-/${filename}`);
assert.equal(metadata.dist.integrity, integrity, 'registry integrity differs from the attested tarball');
assert.equal(metadata.dist.shasum, shasum, 'registry shasum differs from the attested tarball');
assert.deepEqual(metadata.dist.attestations, { url: `${REGISTRY}-/npm/v1/attestations/${name}@${version}`, provenance: { predicateType: SLSA } });
// The registry exposes the OIDC publisher either as a display string or an object.
const publisher = metadata._npmUser as string | { name?: string; email?: string; trustedPublisher?: { id?: string } };
if (typeof publisher === 'string') assert.equal(publisher, 'GitHub Actions <npm-oidc-no-reply@github.com>');
else assert.ok(publisher?.name === 'GitHub Actions' && publisher.email === 'npm-oidc-no-reply@github.com' && publisher.trustedPublisher?.id === 'github', 'publisher is not a GitHub Actions trusted publisher');

const distTags: Record<string, string> = await retry('npm view dist-tags', () => JSON.parse(npm(['view', name, 'dist-tags', '--json'])));
assert.equal(distTags.latest, version, 'latest dist-tag does not point to this release');

type Attestation = { predicateType: string; bundle: { dsseEnvelope: { payload: string; payloadType: string } } };
const response = await retry('attestation fetch', async () => {
  const result = await fetch(metadata.dist.attestations.url);
  if (!result.ok) throw new Error(`HTTP ${result.status}`);
  return (await result.json()) as { attestations: Attestation[] };
});
const provenance = response.attestations.filter((item) => item.predicateType === SLSA);
assert.equal(provenance.length, 1, 'expected exactly one SLSA provenance attestation');
const envelope = provenance[0]!.bundle.dsseEnvelope;
assert.equal(envelope.payloadType, 'application/vnd.in-toto+json');
const statement = JSON.parse(Buffer.from(envelope.payload, 'base64').toString('utf8'));
assert.deepEqual(statement.subject, [{ name: `pkg:npm/${name.replace(/^@/, '%40')}@${version}`, digest: { sha512: Buffer.from(sha512, 'base64').toString('hex') } }]);
assert.equal(statement.predicate?.buildDefinition?.buildType, 'https://slsa-framework.github.io/github-actions-buildtypes/workflow/v1');
assert.deepEqual(statement.predicate.buildDefinition.externalParameters?.workflow, { ref: `refs/tags/${tag}`, repository: REPOSITORY, path: workflow });
assert.deepEqual(statement.predicate.buildDefinition.resolvedDependencies, [{ uri: `git+${REPOSITORY}@refs/tags/${tag}`, digest: { gitCommit: commit } }]);
assert.equal(statement.predicate.runDetails?.builder?.id, 'https://github.com/actions/runner/github-hosted');

// npm verifies registry signatures and the Sigstore provenance bundles; it exits
// non-zero for invalid signatures, which stops this script.
npm(['install', '--ignore-scripts', '--no-audit', '--no-fund', '--save-exact', `${name}@${version}`]);
const audit = JSON.parse(npm(['audit', 'signatures', '--json', '--include-attestations']));
assert.deepEqual(audit.invalid, [], 'npm audit signatures reported invalid signatures');
assert.deepEqual(audit.missing, [], 'npm audit signatures reported missing signatures');
const verified = audit.verified.filter((item: { name: string; version: string }) => item.name === name && item.version === version);
assert.equal(verified.length, 1, 'npm audit signatures did not verify this release');
assert.deepEqual(verified[0].attestations, metadata.dist.attestations, 'npm audit signatures did not verify the provenance attestation');

const packed = record<{ filename: string }>(JSON.parse(npm(['pack', `${name}@${version}`, '--json', '--pack-destination', join(workdir, 'packed')])));
assert.equal(packed.filename, filename);
const bytes = await readFile(join(workdir, 'packed', filename));
assert.equal(createHash('sha256').update(bytes).digest('hex'), sha256, 'downloaded tarball differs from the attested SHA-256');
assert.equal(`sha512-${createHash('sha512').update(bytes).digest('base64')}`, integrity);
assert.equal(createHash('sha1').update(bytes).digest('hex'), shasum);
console.log(`${name}@${version}: registry metadata, latest tag, provenance (${tag} @ ${commit}), signatures and bytes verified.`);
