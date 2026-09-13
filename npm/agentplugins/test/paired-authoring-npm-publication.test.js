"use strict";
const assert = require("node:assert/strict");
const cp = require("node:child_process");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const test = require("node:test");
const p = require("../scripts/publish-paired-authoring-npm");
const c = require("../scripts/dual-authoring-candidate");
const m = require("../scripts/milestone-a-release-admission");
const promotion = require("../scripts/authoring-promotion");
const packing = require("../scripts/stage-dual-authoring-npm");
const contract = require("../scripts/npm-public-contract");
const source = "a".repeat(40), workflow = ".github/workflows/agentplugins-npm-publish.yml";
const repository = `https://github.com/${c.REPOSITORY}`;
const sourceOnly = { skip: process.env.AGENTPLUGINS_STAGED_TEST_CHILD === "1" && "requires the complete repository source tree" };
const clone = value => structuredClone(value);
function root(t) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "paired-publication-test-"));
  t.after(() => fs.rmSync(dir, { recursive: true, force: true }));
  return dir;
}
function env() {
  return { GITHUB_ACTIONS: "true", GITHUB_EVENT_NAME: "workflow_dispatch", GITHUB_REPOSITORY: c.REPOSITORY,
    PRODUCER_MODE: "paired-publish", TAG: m.TAG, KIT_VERSION: "2.0.2", SOURCE_SHA: source,
    GITHUB_SHA: source, GITHUB_WORKFLOW_SHA: source, GITHUB_REF: `refs/tags/${m.TAG}`,
    GITHUB_WORKFLOW_REF: `${c.REPOSITORY}/${workflow}@refs/tags/${m.TAG}`, PUBLISH: "true", NATIVE_INPUTS: "", INPUT_ARTIFACT: "" };
}
function fixture() {
  const bodies = new Map();
  const put = (name, bytes) => { bodies.set(name, bytes); return c.digest(bytes); };
  const products = Object.fromEntries(c.PRODUCTS.map(product => {
    const version = product === "agentplugins" ? "0.1.62" : "2.0.2";
    const assets = Object.fromEntries(c.TARGETS.map(target => {
      const file = c.assetName(product, version, target), bytes = Buffer.from(`${product}/${target}`);
      return [target, { file, sha256: put(file, bytes), size: bytes.length,
        binary: { file: c.executableName(product, target), sha256: c.digest(bytes), size: bytes.length } }];
    }));
    return [product, { tag: product === "agentplugins" ? m.TAG : m.KIT_TAG,
      manifest_sha256: put(`${product}/release-manifest.json`, Buffer.from(`manifest ${product}`)),
      checksums_sha256: put(`${product}/checksums.txt`, Buffer.from(`checksums ${product}`)), assets }];
  }));
  const record = { schema: m.RECORD_SCHEMA, identity: { repository: c.REPOSITORY, commit: source,
    engine_revision: source, versions: { agentplugins: "0.1.62", "plugin-kit-ai": "2.0.2" } },
    authoring_mode: "release-cli-contract-v1", asset_scope: "six-platform-pair",
    candidate_sha256: put("candidate.json", Buffer.from("candidate fixture")),
    pair_marker_sha256: put("pair-prepared.json", Buffer.from("marker fixture")), products,
    preparation: { run_id: 41, run_attempt: 1, artifact_id: 91, artifact_sha256: "b".repeat(64), receipt_sha256: "c".repeat(64) },
    milestone_a: { workflow: m.WORKFLOW, run_id: 42, run_attempt: 1 }, signer: { workflow: m.RELEASE_WORKFLOW, source } };
  put(m.RECORD_FILE, m.encodeRecord(record));
  const releases = c.PRODUCTS.map((product, index) => ({ id: 200 + index,
    tag_name: record.products[product].tag, draft: false, prerelease: false,
    assets: promotion.releasePins(record, product).map((pin, n) => {
      const body = bodies.get(`${product}/${pin.name}`) || bodies.get(pin.name);
      return { id: 1000 + index * 100 + n, name: pin.name, digest: `sha256:${pin.sha256}`, size: body.length, state: "uploaded", body };
    }) }));
  return { record, releases };
}
function provider(t, f, mutate = () => {}) {
  const replies = new Map();
  f.releases.forEach(release => {
    replies.set(`repos/${c.REPOSITORY}/git/ref/tags/${release.tag_name}`, {
      ref: `refs/tags/${release.tag_name}`, object: { type: "commit", sha: source }
    });
    replies.set(`tag=${release.tag_name}`, { data: { repository: { release: { databaseId: release.id } } } });
    replies.set(`repos/${c.REPOSITORY}/releases/${release.id}`, release);
    release.assets.forEach(asset => replies.set(`repos/${c.REPOSITORY}/releases/assets/${asset.id}`, asset.body));
  });
  mutate(replies);
  t.mock.method(cp, "spawnSync", (exe, args) => {
    assert.equal(exe, "/usr/bin/gh");
    assert.ok(args[0] === "api", "fixture admission must remain read-only");
    const result = replies.get(args.at(-1));
    assert.ok(result, `unexpected provider read: ${args.at(-1)}`);
    return { status: 0, stdout: Buffer.isBuffer(result) ? result : JSON.stringify(result) };
  });
}
for (const [key, value] of Object.entries({ TAG: "agentplugins-v0.1.91", KIT_VERSION: "2.0.0", SOURCE_SHA: "0".repeat(40),
  GITHUB_SHA: "b".repeat(40), GITHUB_WORKFLOW_SHA: "b".repeat(40), GITHUB_REF: "refs/heads/main",
  GITHUB_WORKFLOW_REF: `${c.REPOSITORY}/${workflow}@refs/heads/main`, GITHUB_REPOSITORY: "attacker/fork",
  NATIVE_INPUTS: "unrelated stage inputs", PRODUCER_MODE: "paired-stage" })) {
  test(`dispatch rejects ${key} substitution`, () => assert.throws(() => p.selection({ ...env(), [key]: value })));
}
test("fixed selection keeps final source, execution, and workflow identical", () => {
  assert.equal(p.selection(env()).source, source);
  assert.equal(p.selection({ ...env(), PUBLISH: "false" }).source, source);
});
test("audited pair reader authenticates both exact eleven-asset sets", t => {
  const f = fixture(); provider(t, f);
  const result = p.publicPair(promotion.inspectPair(f.record, root(t)));
  assert.deepEqual(result.states, ["public", "public"]);
  assert.ok(result.pair.every(r => !r.assets.some(a => a.name === "THIRD_PARTY_NOTICES.txt")));
});
const mutations = {
  "moved agent tag": (f, replies) => replies.get(`repos/${c.REPOSITORY}/git/ref/tags/${m.TAG}`).object.sha = "b".repeat(40),
  "moved kit tag": (f, replies) => replies.get(`repos/${c.REPOSITORY}/git/ref/tags/${m.KIT_TAG}`).object.sha = "b".repeat(40),
  "wrong release tag": f => { f.releases[0].tag_name = "agentplugins-v0.1.91"; },
  "draft": f => { f.releases[0].draft = true; },
  "prerelease": f => { f.releases[1].prerelease = true; },
  "missing asset": f => { f.releases[1].assets.pop(); },
  "extra notices asset": f => { f.releases[0].assets.push({ ...f.releases[0].assets[0], name: "THIRD_PARTY_NOTICES.txt" }); },
  "duplicate name": f => { f.releases[0].assets[1].name = f.releases[0].assets[0].name; },
  "duplicate id": f => { f.releases[0].assets[1].id = f.releases[0].assets[0].id; },
  "digest mismatch": f => { f.releases[1].assets[0].digest = `sha256:${"f".repeat(64)}`; },
  "size mismatch": f => { f.releases[0].assets[0].size++; },
  "shared record byte mismatch": (f, replies) => replies.set(`repos/${c.REPOSITORY}/releases/assets/${f.releases[1].assets.at(-1).id}`, Buffer.from("bad record")),
  "shared candidate byte mismatch": (f, replies) => replies.set(`repos/${c.REPOSITORY}/releases/assets/${f.releases[1].assets.find(a => a.name === "candidate.json").id}`, Buffer.from("bad candidate"))
};
for (const [name, mutate] of Object.entries(mutations)) test(`pair admission rejects ${name}`, t => {
  const f = fixture(); provider(t, f, replies => mutate(f, replies));
  assert.throws(() => p.publicPair(promotion.inspectPair(f.record, root(t))));
});
test("synthetic Milestone A test versions cannot become release versions", () => {
  const f = fixture(); f.record.identity.versions = m.E2E_VERSIONS;
  assert.throws(() => m.encodeRecord(f.record));
});
function verified() {
  return [{ verificationResult: { statement: { _type: "https://in-toto.io/Statement/v1", predicateType: "https://slsa.dev/provenance/v1",
    subject: [{ name: m.RECORD_FILE, digest: { sha256: "d".repeat(64) } }],
    predicate: { buildDefinition: { buildType: "https://actions.github.io/buildtypes/workflow/v1",
      externalParameters: { workflow: { ref: `refs/tags/${m.TAG}`, repository, path: m.RELEASE_WORKFLOW } },
      resolvedDependencies: [{ uri: `git+${repository}@refs/tags/${m.TAG}`, digest: { gitCommit: source } }] },
    runDetails: { metadata: { invocationId: `${repository}/actions/runs/55/attempts/1` },
      builder: { id: `${repository}/${m.RELEASE_WORKFLOW}@refs/tags/${m.TAG}` } } } } } }];
}
function expected() {
  return { name: m.RECORD_FILE, sha256: "d".repeat(64), source, workflow_sha: source, ref: `refs/tags/${m.TAG}`,
    run_id: 55, run_attempt: 1, subjects: [{ name: m.RECORD_FILE, digest: { sha256: "d".repeat(64) } }] };
}
test("signed promotion uses the audited exact workflow/source/ref/multiset verifier", () => {
  assert.deepEqual(p.promotionInvocation(JSON.stringify(verified()), source), { run_id: 55, run_attempt: 1 });
  promotion.mapVerifiedOutput(JSON.stringify(verified()), expected());
});
for (const [name, change] of Object.entries({
  signature: v => { delete v[0].verificationResult; },
  subject: v => { v[0].verificationResult.statement.subject[0].digest.sha256 = "e".repeat(64); },
  workflow: v => { v[0].verificationResult.statement.predicate.buildDefinition.externalParameters.workflow.path = workflow; },
  ref: v => { v[0].verificationResult.statement.predicate.buildDefinition.externalParameters.workflow.ref = "refs/heads/main"; },
  source: v => { v[0].verificationResult.statement.predicate.buildDefinition.resolvedDependencies[0].digest.gitCommit = "b".repeat(40); }
})) test(`signed promotion rejects ${name} mismatch`, () => {
  const v = verified(); change(v);
  assert.throws(() => promotion.mapVerifiedOutput(JSON.stringify(v), expected()));
});
// Real system tar exercises the reused parser, including its pre-extraction
// closed entry/mode checks. Fixtures never contain executable native payloads.
function tar(t, kind) {
  const dir = root(t), input = path.join(dir, "input"), output = path.join(dir, "test.tgz");
  fs.mkdirSync(path.join(input, "package"), { recursive: true });
  fs.writeFileSync(path.join(input, "package", "good"), "good", { mode: kind === "mode" ? 0o755 : 0o644 });
  if (kind === "extra") fs.writeFileSync(path.join(input, "package", "extra"), "extra");
  if (kind === "link") { fs.unlinkSync(path.join(input, "package", "good")); fs.symlinkSync("/etc/passwd", path.join(input, "package", "good")); }
  const args = ["-czf", output, "-C", input];
  if (kind === "unsafe") args.push("--transform=s|package/good|../good|");
  args.push("package/good");
  if (kind === "extra") args.push("package/extra");
  if (kind === "duplicate") args.push("package/good");
  cp.execFileSync("/usr/bin/tar", args);
  return { dir, output };
}
test("closed tar entry/mode/byte set accepts exact regular bytes", t => {
  const f = tar(t, "good");
  packing.verifyPack(f.output, { good: Buffer.from("good") }, path.join(f.dir, "out"), process.env);
});
for (const kind of ["mode", "extra", "link", "unsafe", "duplicate", "bytes"]) test(`tar rejects ${kind}`, t => {
  const f = tar(t, kind);
  assert.throws(() => packing.verifyPack(f.output, { good: Buffer.from(kind === "bytes" ? "changed" : "good") }, path.join(f.dir, "out"), process.env));
});
for (const status of [401, 403, 429, 500, 503]) test(`registry HTTP ${status} cannot authorize publication`, async t => {
  t.mock.method(globalThis, "fetch", async () => new Response("failed", { status }));
  let writes = 0;
  await assert.rejects(p.publishOnce({ lookup: () => p.registry("https://registry.npmjs.org/universal-agent-plugins/0.1.62", true),
    publish: () => writes++, reconcile: () => assert.fail("unexpected reconciliation") }));
  assert.equal(writes, 0);
});
test("network uncertainty cannot authorize publication", async t => {
  t.mock.method(globalThis, "fetch", async () => { throw new Error("network timeout"); });
  await assert.rejects(p.registry("https://registry.npmjs.org/universal-agent-plugins/0.1.62", true));
});
test("only completed 404 is absence", async t => {
  t.mock.method(globalThis, "fetch", async () => new Response("not found", { status: 404 }));
  assert.equal(await p.registry("https://registry.npmjs.org/universal-agent-plugins/0.1.62", true), null);
  await assert.rejects(p.registry("https://registry.npmjs.org/universal-agent-plugins/0.1.62"));
});
for (const ambiguous of [false, true]) test(`publish ${ambiguous ? "timeout" : "success"} requires reconciliation and never retries`, async () => {
  let writes = 0, reads = 0;
  assert.equal(await p.publishOnce({ lookup: () => null, publish: () => { writes++; if (ambiguous) throw Error("timeout"); },
    reconcile: () => { reads++; return "verified exact public bytes and provenance"; } }), "verified exact public bytes and provenance");
  assert.equal(writes, 1); assert.equal(reads, 1);
});
test("ambiguous publish with missing or mismatched provenance fails without retry", async () => {
  let writes = 0;
  await assert.rejects(p.publishOnce({ lookup: () => null, publish: () => { writes++; throw Error("timeout"); },
    reconcile: () => { throw Error("provenance mismatch"); } }));
  assert.equal(writes, 1);
});
test("existing different bytes fail without overwriting", async () => {
  let writes = 0;
  await assert.rejects(p.publishOnce({ lookup: () => Buffer.from("existing metadata"), publish: () => writes++,
    reconcile: () => { throw Error("existing npm version has different bytes"); } }));
  assert.equal(writes, 0);
});
test("exact paired source contract rejects synthetic versions and different promotion", () => {
  const f = fixture(), metadata = { name: "universal-agent-plugins", version: "0.1.62", gitHead: source };
  const descriptor = { identity: f.record.identity, qualification: { signed_subject: {
    source, sha256: "d".repeat(64), workflow: `${c.REPOSITORY}/${m.RELEASE_WORKFLOW}` } } };
  contract.validatePairedSource(metadata, descriptor, source, "d".repeat(64));
  for (const key of ["gitHead", "version"]) assert.throws(() => contract.validatePairedSource({ ...metadata, [key]: "0.1.91" }, descriptor, source, "d".repeat(64)));
  assert.throws(() => contract.validatePairedSource(metadata, descriptor, source, "e".repeat(64)));
  const bad = clone(descriptor); bad.identity.versions = m.E2E_VERSIONS;
  assert.throws(() => contract.validatePairedSource(metadata, bad, source, "d".repeat(64)));
});
test("workflow isolates paired route and retains exact legacy/stage job bytes", sourceOnly, () => {
  const repo = path.resolve(__dirname, "../../..");
  const file = ".github/workflows/agentplugins-npm-publish.yml";
  const current = fs.readFileSync(path.join(repo, file), "utf8");
  const legacy = current.slice(current.indexOf("  prepare:"), current.indexOf("\n\n  paired_publish_prepare:")).trimEnd();
  assert.equal(c.digest(Buffer.from(legacy)), "a457dc11e3931232c3dec51abffc2c69dc9d9bd51d46b4935093d21e21d20f2c");
  const paired = current.slice(current.indexOf("\n\n  paired_publish_prepare:"));
  assert.equal((paired.match(/actions\/upload-artifact@/g) || []).length, 1);
  assert.match(paired, /environment: npm-agentplugins/);
  assert.match(paired, /id-token: write/);
  assert.match(paired, /artifact-ids: \$\{\{ needs.paired_publish_prepare.outputs.artifact_id \}\}/);
  assert.match(paired, /artifact_digest: \$\{\{ steps.upload.outputs.artifact-digest \}\}/);
  assert.match(paired, /ARTIFACT_DIGEST: \$\{\{ needs.paired_publish_prepare.outputs.artifact_digest \}\}/);
  assert.equal((paired.match(/actions: read/g) || []).length, 2);
  assert.match(paired, /actions\/artifacts\/\$\{ARTIFACT_ID\}/);
  assert.equal((paired.match(/inputs.producer_mode == 'paired-publish'/g) || []).length, 2);
  assert.match(current, /cancel-in-progress: false/);
  assert.doesNotMatch(paired, /THIRD_PARTY_NOTICES|npm publish|npm pack|registry-url:/);
});

test("qualified public package packs once with exact closure and no GitHub notices asset", sourceOnly, t => {
  const f = fixture(), dir = root(t), repo = path.resolve(__dirname, "../../..");
  const stager = require("../scripts/stage-authoring-npm");
  const blobs = Object.fromEntries(stager.STAGE_ALLOWLIST.map(name => [name, { bytes: fs.readFileSync(path.join(repo, name)) }]));
  const manifest = Buffer.from("manifest agentplugins");
  const files = p.packageFiles(blobs, manifest, f.record);
  const pkg = JSON.parse(files["package.json"]), descriptor = JSON.parse(files["public-release.json"]);
  assert.equal(pkg.version, "0.1.62"); assert.equal(pkg.private, false);
  assert.equal(pkg.gitHead, undefined);
  assert.deepEqual(pkg.scripts, { test: "node --test" });
  assert.equal(descriptor.qualification.signed_subject.sha256, c.digest(m.encodeRecord(f.record)));
  assert.ok(!Object.hasOwn(files, "THIRD_PARTY_NOTICES.txt"));
  const packageRoot = path.join(dir, "package"), output = path.join(dir, "output");
  fs.mkdirSync(packageRoot); fs.mkdirSync(output);
  for (const [name, bytes] of Object.entries(files)) {
    const file = path.join(packageRoot, name); fs.mkdirSync(path.dirname(file), { recursive: true });
    fs.writeFileSync(file, bytes, { mode: name.startsWith("bin/") && name.endsWith(".js") ? 0o755 : 0o644 });
  }
  const npm = fs.realpathSync(path.join(path.dirname(process.execPath), "../lib/node_modules/npm/bin/npm-cli.js"));
  const context = packing.npmContext(dir);
  const pack = packing.packPackage("agentplugins", files, packageRoot, { node: process.execPath, npm, output, identity: f.record.identity }, context);
  const identity = p.byteIdentity(fs.readFileSync(path.join(output, pack.file)));
  assert.equal(identity.integrity, pack.integrity); assert.equal(identity.sha256, pack.sha256);
  assert.equal(identity.shasum.length, 40);
});
function publicFixture(product = "agentplugins") {
  const body = Buffer.from("exact fixture tarball"), identity = p.byteIdentity(body), f = fixture();
  const { NAME: name, VERSION: version } = p.productContract(product);
  const descriptor = { identity: f.record.identity, qualification: { signed_subject: {
    source, sha256: "d".repeat(64), workflow: `${c.REPOSITORY}/${m.RELEASE_WORKFLOW}` } } };
  const descriptorBytes = c.encode(descriptor);
  const receipt = { ...identity, source, promotion_sha256: "d".repeat(64), entries: { "public-release.json": c.digest(descriptorBytes) } };
  const metadata = { name, version, gitHead: source, homepage: "https://777genius.github.io/universal-agent-plugins/",
    repository: { type: "git", url: `git+${repository}.git`, directory: "npm/agentplugins" },
    engines: { node: ">=22" }, bin: { agentplugins: "bin/agentplugins.js" }, scripts: { test: "node --test" },
    _npmUser: "GitHub Actions <npm-oidc-no-reply@github.com>",
    dist: { integrity: identity.integrity, shasum: identity.shasum,
      tarball: `https://registry.npmjs.org/${name}/-/${name}-${version}.tgz`,
      attestations: { url: `https://registry.npmjs.org/-/npm/v1/attestations/${name}@${version}`,
        provenance: { predicateType: "https://slsa.dev/provenance/v1" } } } };
  if (product === "plugin-kit-ai") {
    Object.assign(metadata, { homepage: "https://github.com/777genius/universal-agent-plugins",
      repository: { type: "git", url: "git+https://github.com/777genius/universal-agent-plugins.git" },
      engines: { node: ">=18" }, bin: { "plugin-kit-ai": "bin/plugin-kit-ai.js" },
      scripts: { postinstall: "node ./lib/install.js" } });
    delete metadata.gitHead;
  }
  const statement = verified()[0].verificationResult.statement;
  statement.subject = [{ name: `pkg:npm/${name}@${version}`, digest: { sha512: Buffer.from(identity.integrity.slice(7), "base64").toString("hex") } }];
  statement.predicate.buildDefinition.buildType = "https://slsa-framework.github.io/github-actions-buildtypes/workflow/v1";
  statement.predicate.buildDefinition.externalParameters.workflow.path = workflow;
  statement.predicate.runDetails.builder.id = "https://github.com/actions/runner/github-hosted";
  return { body, receipt, metadata, statement, descriptorBytes };
}
function publicReads(t, f) {
  t.mock.method(globalThis, "fetch", async url => {
    if (url.endsWith(`/${f.metadata.name}/${f.metadata.version}`)) return new Response(JSON.stringify(f.metadata));
    if (url === f.metadata.dist.tarball) return new Response(f.body);
    assert.equal(url, f.metadata.dist.attestations.url);
    f.response = { attestations: [{ predicateType: "https://slsa.dev/provenance/v1",
      bundle: { mediaType: "application/vnd.dev.sigstore.bundle.v0.3+json", dsseEnvelope: {
        payloadType: "application/vnd.in-toto+json", payload: Buffer.from(JSON.stringify(f.statement)).toString("base64"),
        signatures: [{ sig: "fixture-not-cryptographic" }] } } }] };
    return new Response(JSON.stringify(f.response));
  });
}
for (const kind of ["different bytes", "provenance workflow", "provenance source", "provenance ref", "integrity"]) {
  test(`real readback validator rejects ${kind} before install or smoke`, async t => {
    const f = publicFixture(), expectedBody = Buffer.from(f.body);
    if (kind === "different bytes") f.body = Buffer.from("wrong public bytes");
    if (kind === "provenance workflow") f.statement.predicate.buildDefinition.externalParameters.workflow.path = m.RELEASE_WORKFLOW;
    if (kind === "provenance source") f.statement.predicate.buildDefinition.resolvedDependencies[0].digest.gitCommit = "b".repeat(40);
    if (kind === "provenance ref") f.statement.predicate.buildDefinition.externalParameters.workflow.ref = "refs/heads/main";
    if (kind === "integrity") f.metadata.dist.integrity = p.byteIdentity(Buffer.from("other")).integrity;
    publicReads(t, f);
    t.mock.method(cp, "execFileSync", () => assert.fail("readback rejection must precede executable effects"));
    await assert.rejects(p.reconcile(f.receipt, expectedBody, root(t), "/fixture/npm.js", {}));
  });
}
test("unsigned or cryptographically invalid npm audit cannot reach authoring smoke", async t => {
  const f = publicFixture(), dir = root(t); publicReads(t, f);
  const calls = [];
  t.mock.method(cp, "execFileSync", (exe, args, options) => {
    calls.push(args); assert.equal(exe, process.execPath);
    assert.ok(options.cwd.startsWith(dir + path.sep));
    if (args.includes("install")) { assert.ok(args.includes("--ignore-scripts")); assert.ok(args.includes("universal-agent-plugins@0.1.62")); return Buffer.from(""); }
    assert.ok(args.includes("audit"));
    return Buffer.from(JSON.stringify({ invalid: [{ name: "universal-agent-plugins" }], missing: [], verified: [] }));
  });
  await assert.rejects(p.reconcile(f.receipt, f.body, dir, "/fixture/npm.js", {}), /cryptographically verify/);
  assert.equal(calls.length, 2);
});

test("dispatch shell admits only exact paired publication identity before checkout", sourceOnly, t => {
  const repo = path.resolve(__dirname, "../../..");
  const text = fs.readFileSync(path.join(repo, ".github/workflows/agentplugins-npm-publish.yml"), "utf8");
  const preflight = text.slice(text.indexOf("        run: |") + "        run: |\n".length, text.indexOf("  prepare:"))
    .split("\n").map(line => line.startsWith("          ") ? line.slice(10) : line).join("\n");
  const execute = values => cp.spawnSync("/bin/bash", ["-e", "-o", "pipefail", "-s"], {
    input: preflight, cwd: root(t), env: { PATH: "/usr/bin:/bin", ...env(), ...values }, encoding: "utf8" });
  assert.equal(execute({}).status, 0);
  assert.equal(execute({ PUBLISH: "false" }).status, 0);
  for (const values of [{ SOURCE_SHA: "b".repeat(40) }, { TAG: "agentplugins-v0.1.91" }, { KIT_VERSION: "2.0.0" },
    { GITHUB_WORKFLOW_SHA: "b".repeat(40) }, { GITHUB_REF: "refs/heads/main" }, { INPUT_ARTIFACT: "{}" },
    { GITHUB_EVENT_NAME: "workflow_run" }, { GITHUB_WORKFLOW_REF: "attacker/workflow" }]) {
    assert.notEqual(execute(values).status, 0);
  }
});

test("verified readback performs only disposable exact-version smoke, with publisher credentials removed", async t => {
  const f = publicFixture(), dir = root(t), calls = []; publicReads(t, f);
  let project;
  t.mock.method(cp, "execFileSync", (exe, args, options) => {
    calls.push(args); assert.equal(exe, process.execPath);
    assert.ok(options.cwd.startsWith(dir + path.sep)); project = options.cwd;
    assert.equal(options.env.ACTIONS_ID_TOKEN_REQUEST_TOKEN, undefined);
    assert.equal(options.env.ACTIONS_ID_TOKEN_REQUEST_URL, undefined);
    if (args.includes("install")) {
      assert.equal(args.at(-1), "universal-agent-plugins@0.1.62");
      const installed = path.join(project, "node_modules/universal-agent-plugins");
      fs.mkdirSync(installed, { recursive: true });
      fs.writeFileSync(path.join(installed, "public-release.json"), f.descriptorBytes);
    } else if (args.includes("audit")) {
      // Fixture assertion of the npm verifier boundary, not a real signature.
      return Buffer.from(JSON.stringify({ invalid: [], missing: [], verified: [{ name: "universal-agent-plugins",
        version: "0.1.62", location: "node_modules/universal-agent-plugins", registry: "https://registry.npmjs.org/",
        attestations: f.metadata.dist.attestations, attestationBundles: f.response.attestations }] }));
    } else assert.equal(args[1], "author");
    return Buffer.from("");
  });
  const result = await p.reconcile(f.receipt, f.body, dir, "/fixture/npm.js", {
    ACTIONS_ID_TOKEN_REQUEST_TOKEN: "never-pass-to-smoke", ACTIONS_ID_TOKEN_REQUEST_URL: "https://fixture.invalid" });
  assert.equal(result.status, "verified");
  assert.deepEqual(calls.slice(2).map(args => args[2]), ["--help", "init", "validate"]);
  assert.equal(fs.existsSync(project), false, "disposable smoke is cleaned up");
});

test("independent public readback binds IDs and pins while allowing download counters to advance", () => {
  const f = fixture(), before = { pair: f.releases }, after = clone(before);
  after.pair[0].assets[0].download_count = 7;
  after.pair[0].updated_at = "2026-09-13T12:00:00Z";
  after.pair[1].assets.reverse();
  assert.deepEqual(p.pairIdentity(after), p.pairIdentity(before));
  after.pair[0].assets[0].id++;
  assert.notDeepEqual(p.pairIdentity(after), p.pairIdentity(before));
});

for (const product of c.PRODUCTS) test(`exact ${product} published pack runs cold-cache lifecycle and launcher`, sourceOnly, t => {
  const dir = root(t), f = fixture(), repo = path.resolve(__dirname, "../../..");
  const stager = require("../scripts/stage-authoring-npm"), bodies = {}, manifests = {};
  const blobs = Object.fromEntries(stager.STAGE_ALLOWLIST.map(name => [name, { bytes: fs.readFileSync(path.join(repo, name)) }]));
  for (const peer of c.PRODUCTS) {
    const pins = f.record.products[peer];
    for (const target of c.TARGETS) {
      const binary = Buffer.from(`#!${process.execPath}\n// ${peer} ${target}\nconsole.log(JSON.stringify({argv:process.argv.slice(2),preload:process.env.NODE_OPTIONS||null}));\n`);
      const body = peer === "agentplugins" ? binary : c.archive(binary, c.executableName(peer, target));
      pins.assets[target] = { file: c.assetName(peer, f.record.identity.versions[peer], target), ...c.metadata(body),
        binary: { file: c.executableName(peer, target), ...c.metadata(binary) } };
      bodies[`https://github.com/${c.REPOSITORY}/releases/download/${pins.tag}/${pins.assets[target].file}`] = body.toString("base64");
    }
    manifests[peer] = c.encode({ schema_version: 3, status: "CANDIDATE", product: peer, repository: c.REPOSITORY,
      tag: pins.tag, version: f.record.identity.versions[peer], commit: source, engine_revision: source,
      versions: f.record.identity.versions, candidate_sha256: f.record.candidate_sha256,
      authoring_mode: f.record.authoring_mode, asset_scope: f.record.asset_scope, assets: pins.assets,
      release_eligible: false, platform_acceptance: false, attested: false });
    pins.manifest_sha256 = c.digest(manifests[peer]);
  }
  const files = p.packageFiles(blobs, manifests[product], f.record, product);
  const pkg = JSON.parse(files["package.json"]);
  assert.equal(pkg.private, false); assert.equal(pkg.gitHead, undefined);
  assert.equal(pkg.repository.url, "git+https://github.com/777genius/universal-agent-plugins.git");
  if (product === "plugin-kit-ai") {
    assert.equal(pkg.homepage, repository);
    assert.equal(pkg.bugs.url, repository + "/issues");
  }
  for (const name of ["public-release.json", "lib/public-authoring.js", "lib/public-authoring-contract.js", "lib/public-authoring-input.js"]) {
    assert.ok(files[name]); assert.ok(pkg.files.includes(name));
  }
  const packageRoot = path.join(dir, "source"), output = path.join(dir, "packs");
  fs.mkdirSync(packageRoot); fs.mkdirSync(output);
  for (const [name, bytes] of Object.entries(files)) {
    const file = path.join(packageRoot, name); fs.mkdirSync(path.dirname(file), { recursive: true });
    fs.writeFileSync(file, bytes, { mode: /^bin\/.*\.js$/.test(name) ? 0o755 : 0o644 });
  }
  const npm = fs.realpathSync(path.join(path.dirname(process.execPath), "../lib/node_modules/npm/bin/npm-cli.js"));
  const context = packing.npmContext(dir);
  const pack = packing.packPackage(product, files, packageRoot, { node: process.execPath, npm, output, identity: f.record.identity }, context);
  const loader = path.join(dir, "transport.cjs"), log = path.join(dir, "requests.log");
  // Transport-only fixture; actual packed postinstall, runtime, and launcher run.
  fs.writeFileSync(loader, `const https=require('node:https'),fs=require('node:fs'),{EventEmitter}=require('node:events'),{PassThrough}=require('node:stream');
    const bodies=${JSON.stringify(bodies)};
    https.get=options=>{const req=new EventEmitter();req.setTimeout=()=>req;req.destroy=e=>req.emit('error',e);
      process.nextTick(()=>{const url='https://'+options.hostname+options.path;fs.appendFileSync(${JSON.stringify(log)},url+'\\n');
        if(!bodies[url]) return req.emit('error',Error('unexpected fixture URL '+url));
        const res=new PassThrough();res.statusCode=200;res.headers={};req.emit('response',res);res.end(Buffer.from(bodies[url],'base64'));});return req;};`);
  const project = path.join(dir, "project"), home = path.join(dir, "home"); fs.mkdirSync(project); fs.mkdirSync(home);
  fs.writeFileSync(path.join(project, "package.json"), JSON.stringify({ name: "fixture", version: "1.0.0", private: true,
    allowScripts: { [`file:${path.join(output, pack.file)}`]: true } }));
  const env = { ...context.env, PATH: process.env.PATH, HOME: home, XDG_CACHE_HOME: path.join(home, ".cache"),
    NODE_OPTIONS: `--require=${JSON.stringify(loader)}` };
  cp.execFileSync(process.execPath, [npm, "install", "--offline", "--ignore-scripts=false", "--strict-allow-scripts", "--no-audit", "--no-fund", path.join(output, pack.file)],
    { cwd: project, env, timeout: 60000, stdio: "pipe" });
  if (product === "plugin-kit-ai") assert.equal(fs.readFileSync(log, "utf8").trim().split("\n").length, 1, "npm postinstall acquired exact native bytes");
  const installed = path.join(project, "node_modules", pkg.name);
  for (const [name, bytes] of Object.entries(files)) assert.deepEqual(fs.readFileSync(path.join(installed, name)), bytes);
  const args = product === "agentplugins" ? ["author", "--help"] : ["--help"];
  const result = cp.execFileSync(process.execPath, [path.join(installed, `bin/${product}.js`), ...args], { cwd: project, env, timeout: 30000 });
  assert.deepEqual(JSON.parse(result), { argv: args, preload: null });
  assert.equal(fs.readFileSync(log, "utf8").trim().split("\n").length, 1);
});

test("existing identical version reconciles without republishing", async () => {
  let reads = 0;
  const result = await p.publishOnce({ lookup: () => Buffer.from("existing metadata"),
    publish: () => assert.fail("must not publish"),
    reconcile: () => { reads++; return { status: "verified" }; } });
  assert.deepEqual(result, { status: "verified" });
  assert.equal(reads, 1);
});

test("prepublication smoke uses exact tarball, actual lifecycle, disposable roots and no publisher credentials", t => {
  const dir = root(t), context = packing.npmContext(dir), calls = [];
  t.mock.method(cp, "execFileSync", (exe, args, options) => {
    calls.push(args);
    assert.equal(exe, process.execPath);
    assert.ok(options.cwd.startsWith(dir + path.sep));
    assert.ok(options.env.HOME.startsWith(options.cwd + path.sep));
    const project = JSON.parse(fs.readFileSync(path.join(options.cwd, "package.json")));
    assert.deepEqual(project.allowScripts, { "file:/exact/plugin-kit-ai-2.0.2.tgz": true });
    for (const name of ["GH_TOKEN", "NPM_TOKEN", "NODE_AUTH_TOKEN", "NODE_OPTIONS", "ACTIONS_ID_TOKEN_REQUEST_TOKEN", "ACTIONS_ID_TOKEN_REQUEST_URL"]) assert.equal(options.env[name], undefined);
    return Buffer.from("");
  });
  p.packedSmoke("plugin-kit-ai", "/exact/plugin-kit-ai-2.0.2.tgz", dir, "/fixture/npm.js", context);
  assert.ok(calls[0].includes("--ignore-scripts=false")); assert.equal(calls[0].at(-1), "/exact/plugin-kit-ai-2.0.2.tgz");
  assert.deepEqual(calls.slice(1).map(args => args[1]), ["--help", "init", "validate"]);
  assert.equal(fs.readdirSync(dir).some(n => n.startsWith("packed-smoke-")), false);
});

test("kit readback verifies product-specific registry, signatures, postinstall and launcher", async t => {
  const product = "plugin-kit-ai", f = publicFixture(product), dir = root(t), calls = [];
  publicReads(t, f);
  t.mock.method(cp, "execFileSync", (exe, args, options) => {
    calls.push(args);
    assert.equal(exe, process.execPath); assert.ok(options.cwd.startsWith(dir + path.sep));
    if (args.includes("install")) {
      assert.equal(args.at(-1), "plugin-kit-ai@2.0.2"); assert.ok(args.includes("--ignore-scripts"));
      const installed = path.join(options.cwd, "node_modules/plugin-kit-ai"); fs.mkdirSync(installed, { recursive: true });
      fs.writeFileSync(path.join(installed, "public-release.json"), f.descriptorBytes);
    } else if (args.includes("audit")) return Buffer.from(JSON.stringify({ invalid: [], missing: [], verified: [{
      name: product, version: "2.0.2", location: `node_modules/${product}`, registry: "https://registry.npmjs.org/",
      attestations: f.metadata.dist.attestations, attestationBundles: f.response.attestations }] }));
    return Buffer.from("");
  });
  const result = await p.reconcile(f.receipt, f.body, dir, "/fixture/npm.js", {}, product);
  assert.equal(result.status, "verified");
  assert.ok(calls[2][0].endsWith("/plugin-kit-ai/lib/install.js"));
  assert.deepEqual(calls.slice(3).map(args => args[1]), ["--help", "init", "validate"]);
});

for (const product of c.PRODUCTS) {
  for (const defect of ["name", "version", "scripts", "repository", "tarball", "attestation", "source", "ref", "subject"]) {
    test(`${product} readback rejects substituted ${defect}`, async t => {
      const f = publicFixture(product), dir = root(t);
      if (defect === "name") f.metadata.name = "other-package";
      if (defect === "version") f.metadata.version = "2.0.3";
      if (defect === "scripts") f.metadata.scripts = { postinstall: "node malicious.js" };
      if (defect === "repository") f.metadata.repository.url = "git+https://github.com/attacker/fork.git";
      if (defect === "tarball") f.metadata.dist.tarball = "https://registry.npmjs.org/other/-/other-2.0.2.tgz";
      if (defect === "attestation") f.metadata.dist.attestations.url += "/wrong";
      if (defect === "source") f.statement.predicate.buildDefinition.resolvedDependencies[0].digest.gitCommit = "b".repeat(40);
      if (defect === "ref") f.statement.predicate.buildDefinition.externalParameters.workflow.ref = "refs/tags/plugin-kit-ai-v2.0.2";
      if (defect === "subject") f.statement.subject[0].name = "pkg:npm/other@2.0.2";
      publicReads(t, f);
      t.mock.method(cp, "execFileSync", () => assert.fail("substitution must fail before installed code executes"));
      await assert.rejects(p.reconcile(f.receipt, f.body, dir, "/fixture/npm.js", {}, product));
    });
  }
}

test("legacy copy-only publisher refuses v2 before authentication or package staging", sourceOnly, t => {
  const workflowText = fs.readFileSync(path.resolve(__dirname, "../../../.github/workflows/npm-publish.yml"), "utf8");
  const block = workflowText.slice(workflowText.indexOf("      - name: Resolve stable tag"), workflowText.indexOf("      - name: Configure npm publish authentication"));
  const shell = block.slice(block.indexOf("        run: |\n") + "        run: |\n".length)
    .split("\n").map(line => line.startsWith("          ") ? line.slice(10) : line).join("\n")
    .replace('${{ inputs.tag }}', '${TEST_TAG}');
  for (const tag of ["1.2.4", "v1.2.4", "plugin-kit-ai-v1.2.4", "2.0.2", "v2.0.2", "plugin-kit-ai-v2.0.2", "agentplugins-v0.1.62"]) {
    const result = cp.spawnSync("/bin/bash", ["-e", "-s"], { input: shell, encoding: "utf8", cwd: root(t),
      env: { PATH: "/usr/bin:/bin", GITHUB_EVENT_NAME: "workflow_dispatch", GITHUB_OUTPUT: path.join(root(t), "output"), TEST_TAG: tag } });
    assert.equal(result.status === 0, tag.includes("1.2.4"), `${tag}: ${result.stderr}`);
  }
});

for (const product of c.PRODUCTS) {
  for (const defect of [null, "bytes", "integrity", "provenance", "signature", "source", "installed", "promotion", "redirected repository"]) {
    test(`resume ${product}: ${defect || "exact receipt"} never republishes`, async t => {
      const f = publicFixture(product), dir = root(t), retained = Buffer.from(f.body);
      if (defect === "bytes") f.body = Buffer.from("different registry bytes");
      if (defect === "integrity") f.metadata.dist.integrity = p.byteIdentity(Buffer.from("other")).integrity;
      if (defect === "provenance") f.statement.predicate.buildDefinition.externalParameters.workflow.path = "other.yml";
      if (defect === "source") f.statement.predicate.buildDefinition.resolvedDependencies[0].digest.gitCommit = "b".repeat(40);
      if (defect === "promotion") f.receipt.promotion_sha256 = "e".repeat(64);
      if (defect === "redirected repository") f.metadata.repository.url = "git+https://github.com/777genius/plugin-kit-ai.git";
      publicReads(t, f);
      let installs = 0, reconciliations = 0, writes = 0;
      t.mock.method(cp, "execFileSync", (exe, args, options) => {
        if (args.includes("install")) {
          installs++;
          const installed = path.join(options.cwd, "node_modules", f.metadata.name);
          fs.mkdirSync(installed, { recursive: true });
          fs.writeFileSync(path.join(installed, "public-release.json"),
            defect === "installed" ? Buffer.from("changed") : f.descriptorBytes);
        } else if (args.includes("audit")) {
          return Buffer.from(JSON.stringify({ invalid: defect === "signature" ? [{}] : [], missing: [], verified: [{
            name: f.metadata.name, version: f.metadata.version, location: `node_modules/${f.metadata.name}`,
            registry: "https://registry.npmjs.org/", attestations: f.metadata.dist.attestations,
            attestationBundles: f.response.attestations
          }] }));
        } else if (defect) assert.fail("mismatch must fail before smoke");
        return Buffer.from("");
      });
      const attempt = p.publishOnce({
        lookup: () => p.registry(`https://registry.npmjs.org/${f.metadata.name}/${f.metadata.version}`, true),
        publish: () => { writes++; throw Error("immutable version must not be republished"); },
        reconcile: () => {
          reconciliations++;
          return p.reconcile(f.receipt, retained, dir, "/fixture/npm.js", {}, product);
        }
      });
      if (defect) await assert.rejects(attempt);
      else {
        assert.deepEqual(await attempt, { status: "verified", ...p.byteIdentity(retained) });
        assert.equal(installs, 1);
      }
      assert.equal(reconciliations, 1);
      assert.equal(writes, 0);
    });
  }
}

test("partial matrix resumes the completed product and publishes only the absent peer", async () => {
  const existing = new Set(["agentplugins"]), writes = [], readbacks = [];
  for (const product of c.PRODUCTS) {
    await p.publishOnce({
      lookup: () => existing.has(product) ? Buffer.from("immutable metadata") : null,
      publish: () => { writes.push(product); existing.add(product); },
      reconcile: () => { readbacks.push(product); return { status: "verified" }; }
    });
  }
  assert.deepEqual(writes, ["plugin-kit-ai"]);
  assert.deepEqual(readbacks, c.PRODUCTS);
});

test("lookup uncertainty stops before publication or reconciliation", async () => {
  await assert.rejects(p.publishOnce({
    lookup: () => { throw Error("registry unavailable"); },
    publish: () => assert.fail("must not publish"),
    reconcile: () => assert.fail("must not reconcile")
  }), /registry unavailable/);
});
