#!/usr/bin/env node
"use strict";

// Fixed Milestone A consumer. Admission is read-only; only publish() can write
// to npm, and it accepts an already packed, authenticated artifact.
const assert = require("node:assert/strict");
const cp = require("node:child_process");
const crypto = require("node:crypto");
const fs = require("node:fs");
const path = require("node:path");
const c = require("./dual-authoring-candidate");
const promotion = require("./authoring-promotion");
const milestone = require("./milestone-a-release-admission");
const packing = require("./stage-dual-authoring-npm");
const contract = require("./npm-public-contract");
const VERSION = "0.1.62";
const NAME = "universal-agent-plugins";
function productContract(product = "agentplugins") {
  assert.ok(c.PRODUCTS.includes(product), "fixed paired product required");
  const name = product === "agentplugins" ? NAME : "plugin-kit-ai";
  const version = product === "agentplugins" ? VERSION : "2.0.2";
  return { NAME: name, VERSION: version, FILE: `${name}-${version}.tgz` };
}
const WORKFLOW = ".github/workflows/agentplugins-npm-publish.yml";
const REGISTRY = "https://registry.npmjs.org";
const LIMIT = 128 * 1024 * 1024;
const hash = (bytes, algorithm, encoding = "hex") => crypto.createHash(algorithm).update(bytes).digest(encoding);
const write = (file, body) => fs.writeFileSync(file, body, { flag: "wx", mode: 0o600 });
function run(exe, args, cwd, env = process.env) {
  return cp.execFileSync(exe, args, { cwd, env, timeout: 120000, maxBuffer: LIMIT });
}
function gh(args, cwd) {
  return run("/usr/bin/gh", args, cwd, { PATH: "/usr/local/bin:/usr/bin:/bin", GH_TOKEN: process.env.GH_TOKEN });
}
function api(endpoint, cwd) {
  return JSON.parse(gh(["api", "--hostname", "github.com", `repos/${c.REPOSITORY}/${endpoint}`], cwd));
}
function selection(e) {
  assert.equal(e.GITHUB_ACTIONS, "true");
  assert.equal(e.GITHUB_EVENT_NAME, "workflow_dispatch");
  assert.equal(e.GITHUB_REPOSITORY, c.REPOSITORY);
  assert.equal(e.PRODUCER_MODE, "paired-publish");
  assert.equal(e.TAG, milestone.TAG);
  assert.equal(e.KIT_VERSION, "2.0.2");
  assert.match(e.SOURCE_SHA, /^(?!0{40}$)[0-9a-f]{40}$/);
  assert.equal(e.GITHUB_SHA, e.SOURCE_SHA);
  assert.equal(e.GITHUB_WORKFLOW_SHA, e.SOURCE_SHA);
  assert.equal(e.GITHUB_REF, `refs/tags/${milestone.TAG}`);
  assert.equal(e.GITHUB_WORKFLOW_REF, `${c.REPOSITORY}/${WORKFLOW}@${e.GITHUB_REF}`);
  assert.equal(e.NATIVE_INPUTS || "", "");
  assert.equal(e.INPUT_ARTIFACT || "", "");
  assert.ok(["true", "false"].includes(e.PUBLISH));
  return { tag: milestone.TAG, ref: e.GITHUB_REF, source: e.SOURCE_SHA,
    versions: { agentplugins: VERSION, "plugin-kit-ai": "2.0.2" } };
}
function checkout(repo, source) {
  assert.equal(run("/usr/bin/git", ["rev-parse", "HEAD"], repo).toString().trim(), source);
  assert.equal(run("/usr/bin/git", ["status", "--porcelain", "--untracked-files=all"], repo).length, 0);
  assert.equal(path.resolve(__dirname, "../../.."), repo);
}
function publicPair(observation) {
  assert.deepEqual(observation.states, ["public", "public"]);
  assert.equal(observation.reconciliation_required, false);
  const ids = new Set();
  for (const release of observation.pair) {
    assert.equal(release.draft, false);
    assert.equal(release.prerelease, false);
    assert.equal(release.assets.length, 11);
    assert.deepEqual(release.missing_assets, []);
    for (const asset of release.assets) {
      assert.ok(!ids.has(asset.id), "unique pair asset IDs required");
      ids.add(asset.id);
    }
  }
  return observation;
}
// Provider download counters and timestamps are not release identity. Preserve
// exact release/asset IDs and authenticated pins across independent readbacks.
function pairIdentity(observation) {
  return observation.pair.map(release => ({ id: release.id, tag: release.tag_name,
    assets: release.assets.map(({ id, name, size, digest, state }) => ({ id, name, size, digest, state }))
      .sort((a, b) => a.name.localeCompare(b.name, "en")) }));
}
// Discover only a cryptographically verified invocation. The audited verifier
// then binds that invocation to the complete subject multiset and exact source.
function promotionInvocation(output, source) {
  const verified = JSON.parse(output);
  assert.ok(Array.isArray(verified) && verified.length > 0 && verified.length <= 64);
  const pins = new Map();
  for (const row of verified) {
    const statement = row?.verificationResult?.statement;
    const build = statement?.predicate?.buildDefinition;
    if (build?.resolvedDependencies?.[0]?.digest?.gitCommit !== source) continue;
    const id = statement?.predicate?.runDetails?.metadata?.invocationId;
    const prefix = `https://github.com/${c.REPOSITORY}/actions/runs/`;
    if (typeof id !== "string" || !id.startsWith(prefix)) continue;
    const match = /^([1-9][0-9]*)\/attempts\/([1-9][0-9]{0,3})$/.exec(id.slice(prefix.length));
    if (!match) continue;
    const pin = { run_id: Number(match[1]), run_attempt: Number(match[2]) };
    assert.ok(Number.isSafeInteger(pin.run_id) && pin.run_attempt <= 1000);
    pins.set(id, pin);
  }
  assert.equal(pins.size, 1, "one signed promotion invocation required");
  return [...pins.values()][0];
}
function admit(selected, repo, scratch) {
  checkout(repo, selected.source);
  // Bootstrap record is untrusted until all canonical pins and signatures pass.
  const release = api(`releases/tags/${milestone.TAG}`, scratch);
  assert.equal(release.tag_name, milestone.TAG);
  assert.equal(release.draft, false);
  assert.equal(release.prerelease, false);
  assert.ok(Array.isArray(release.assets) && release.assets.length === 11);
  const records = release.assets.filter(a => a.name === milestone.RECORD_FILE);
  assert.equal(records.length, 1);
  assert.ok(Number.isSafeInteger(records[0].id) && records[0].id > 0);
  const body = gh(["api", "--hostname", "github.com", "-H", "Accept: application/octet-stream",
    `repos/${c.REPOSITORY}/releases/assets/${records[0].id}`], scratch);
  const record = milestone.validateSelection(body, selected);
  const recordFile = path.join(scratch, milestone.RECORD_FILE);
  write(recordFile, body);
  const observation = publicPair(promotion.inspectPair(record, scratch));
  const { receipt_sha256, ...preparation } = record.preparation;
  const state = milestone.admit({ record: recordFile, scratch, workflow_sha: selected.source, preparation,
    selected, milestone_a: { run_id: record.milestone_a.run_id, run_attempt: record.milestone_a.run_attempt } });
  // inspectPair downloads both closed sets against the same hashes. Canonical
  // candidate/marker/record bytes therefore match across releases and retention.
  const output = gh(["attestation", "verify", recordFile, "--repo", c.REPOSITORY,
    "--signer-workflow", `github.com/${c.REPOSITORY}/${milestone.RELEASE_WORKFLOW}`,
    "--signer-digest", selected.source, "--source-digest", selected.source,
    "--source-ref", selected.ref, "--cert-oidc-issuer", "https://token.actions.githubusercontent.com",
    "--deny-self-hosted-runners", "--predicate-type", "https://slsa.dev/provenance/v1", "--format", "json"], scratch);
  const invocation = promotionInvocation(output.toString(), selected.source);
  const subjects = state.subjects.map(s => ({ name: path.basename(s.file), digest: { sha256: s.sha256 } }));
  for (const subject of state.subjects) promotion.verifySubject(subject.file, {
    name: path.basename(subject.file), sha256: subject.sha256, source: selected.source,
    workflow_sha: selected.source, ref: selected.ref, ...invocation, subjects
  }, scratch);
  assert.deepEqual(pairIdentity(publicPair(promotion.inspectPair(record, scratch))), pairIdentity(observation),
    "release identity changed during admission");
  return state;
}
function packageFiles(source, manifest, record, product = "agentplugins") {
  productContract(product);
  milestone.recordShape(record);
  const files = require("./stage-authoring-npm").packageFiles(product, source, manifest,
    { identity: record.identity, manifestDigest: record.candidate_sha256 });
  const descriptor = JSON.parse(files["public-release.json"]);
  descriptor.qualification = { identity: record.identity, candidate_sha256: record.candidate_sha256,
    manifest_sha256: Object.fromEntries(c.PRODUCTS.map(p => [p, record.products[p].manifest_sha256])),
    signed_subject: { sha256: c.digest(milestone.encodeRecord(record)),
      workflow: `${c.REPOSITORY}/${milestone.RELEASE_WORKFLOW}`, source: record.identity.commit } };
  assert.equal(c.digest(manifest), record.products[product].manifest_sha256);
  files["public-release.json"] = c.encode(descriptor);
  const pkg = JSON.parse(files["package.json"]);
  pkg.private = false;
  delete pkg.gitHead;
  for (const name of ["lib/public-authoring-contract.js", "lib/public-authoring-input.js"]) {
    files[name] = source[`npm/agentplugins/${name}`].bytes;
  }
  pkg.files = [...Object.keys(files)].sort();
  files["package.json"] = c.encode(pkg);
  return files;
}
function byteIdentity(body) {
  return { sha256: hash(body, "sha256"), integrity: `sha512-${hash(body, "sha512", "base64")}`,
    shasum: hash(body, "sha1"), size: body.length };
}
// Execute only the exact authenticated tarball in a fresh home/project. This
// runs before artifact transfer and before the protected publisher can run.
function packedSmoke(product, tarball, scratch, npm, context) {
  const { NAME } = productContract(product);
  const project = fs.mkdtempSync(path.join(scratch, "packed-smoke-"));
  const home = path.join(project, "home"); fs.mkdirSync(home);
  const env = { PATH: process.env.PATH, HOME: home, XDG_CACHE_HOME: path.join(home, ".cache"), CI: "true",
    npm_config_userconfig: context.env.npm_config_userconfig,
    npm_config_globalconfig: context.env.npm_config_globalconfig,
    npm_config_cache: path.join(home, "npm-cache"), npm_config_registry: `${REGISTRY}/` };
  try {
    write(path.join(project, "package.json"), c.encode({ name: "paired-packed-smoke", version: "1.0.0", private: true,
      allowScripts: { [`file:${tarball}`]: true } }));
    run(process.execPath, [npm, "install", "--offline", "--ignore-scripts=false", "--strict-allow-scripts", "--no-audit", "--no-fund", tarball], project, env);
    const cli = path.join(project, "node_modules", NAME, `bin/${product}.js`);
    const prefix = product === "agentplugins" ? ["author"] : [];
    run(process.execPath, [cli, ...prefix, "--help"], project, env);
    run(process.execPath, [cli, ...prefix, "init", "smoke", "--template=skill", "--name=paired-smoke", "--description=Disposable packed smoke.", "--format=json"], project, env);
    run(process.execPath, [cli, ...prefix, "validate", "smoke"], project, env);
  } finally { fs.rmSync(project, { recursive: true, force: true }); }
}
function prepare(selected, repo, scratch, output, npm, product = "agentplugins") {
  const { NAME, VERSION, FILE } = productContract(product);
  const state = admit(selected, repo, scratch);
  const context = packing.npmContext(scratch);
  const source = packing.blobs(repo, selected.source, context.env, "stage");
  const manifest = c.readFile(path.join(state.o.root, product, "release-manifest.json"));
  const files = packageFiles(source, manifest, state.record, product);
  const root = path.join(context.root, "package-source");
  fs.mkdirSync(root);
  for (const [name, bytes] of Object.entries(files)) {
    const file = path.join(root, name);
    fs.mkdirSync(path.dirname(file), { recursive: true });
    write(file, bytes);
    fs.chmodSync(file, /^bin\/[^/]+\.js$/.test(name) ? 0o755 : 0o644);
  }
  fs.mkdirSync(output);
  packing.packPackage(product, files, root, { node: process.execPath, npm, output,
    identity: state.record.identity }, context);
  const body = c.readFile(path.join(output, FILE), LIMIT);
  packedSmoke(product, path.join(output, FILE), scratch, npm, context);
  assert.ok(c.readFile(path.join(output, FILE), LIMIT).equals(body), "pack changed during smoke");
  checkout(repo, selected.source);
  publicPair(promotion.inspectPair(state.record, scratch));
  const receipt = { schema: "paired-authoring-npm-publication/v1", name: NAME, version: VERSION, file: FILE,
    source: selected.source, workflow: WORKFLOW, ref: selected.ref, ...byteIdentity(body),
    promotion_sha256: c.digest(milestone.encodeRecord(state.record)),
    entries: Object.fromEntries(Object.entries(files).map(([name, bytes]) => [name, c.digest(bytes)])) };
  write(path.join(output, "publication.json"), c.encode(receipt));
  return receipt;
}
// No npm-view exit-code heuristic: only a completed HTTP 404 means absence.
async function registry(url, allowAbsent = false) {
  assert.ok(url.startsWith(`${REGISTRY}/`));
  const response = await fetch(url, { redirect: "error", signal: AbortSignal.timeout(30000) });
  if (response.status === 404 && allowAbsent) return null;
  assert.equal(response.status, 200, "registry read failed; publication forbidden");
  const chunks = []; let size = 0;
  for await (const chunk of response.body) {
    size += chunk.length;
    assert.ok(size <= LIMIT, "bounded registry response required");
    chunks.push(chunk);
  }
  return Buffer.concat(chunks);
}
function receiptBytes(root, selected, product = "agentplugins") {
  const { NAME, VERSION, FILE } = productContract(product);
  assert.deepEqual(fs.readdirSync(root).sort(), [FILE, "publication.json"].sort());
  const receipt = JSON.parse(c.readFile(path.join(root, "publication.json")));
  c.keys(receipt, ["schema", "name", "version", "file", "source", "workflow", "ref", "sha256", "integrity",
    "shasum", "size", "promotion_sha256", "entries"], "publication receipt");
  assert.deepEqual([receipt.schema, receipt.name, receipt.version, receipt.file, receipt.source, receipt.workflow, receipt.ref],
    ["paired-authoring-npm-publication/v1", NAME, VERSION, FILE, selected.source, WORKFLOW, selected.ref]);
  const body = c.readFile(path.join(root, FILE), LIMIT);
  for (const [key, value] of Object.entries(byteIdentity(body))) assert.equal(receipt[key], value);
  return { receipt, body };
}
async function reconcile(receipt, body, scratch, npm, env, product = "agentplugins") {
  const { NAME, VERSION } = productContract(product);
  const metadata = JSON.parse(await registry(`${REGISTRY}/${NAME}/${VERSION}`));
  contract.validatePublicMetadata(metadata, VERSION, receipt.integrity, receipt.shasum, product);
  assert.ok(metadata.gitHead === undefined || metadata.gitHead === receipt.source, "npm source binding");
  const publicBytes = await registry(metadata.dist.tarball);
  assert.ok(publicBytes.equals(body), "existing npm version has different bytes");
  const response = JSON.parse(await registry(metadata.dist.attestations.url));
  contract.validateSLSAAttestation(response, VERSION, receipt.integrity, milestone.TAG, receipt.source, product);
  const project = fs.mkdtempSync(path.join(scratch, "readback-"));
  // Installed scripts and the smoke do not inherit the publisher's OIDC token.
  env = { ...env };
  delete env.ACTIONS_ID_TOKEN_REQUEST_TOKEN;
  delete env.ACTIONS_ID_TOKEN_REQUEST_URL;
  try {
    write(path.join(project, "package.json"), JSON.stringify({ name: "paired-readback", version: "1.0.0", private: true }));
    run(process.execPath, [npm, "install", "--ignore-scripts", "--no-audit", "--no-fund", "--save-exact", `${NAME}@${VERSION}`], project, env);
    const audit = JSON.parse(run(process.execPath, [npm, "audit", "signatures", "--json", "--include-attestations"], project, env));
    contract.validateAuditSignatures(audit, response, VERSION, product);
    const installed = path.join(project, "node_modules", NAME);
    for (const [name, sha256] of Object.entries(receipt.entries)) {
      assert.equal(c.digest(c.readFile(path.join(installed, name))), sha256, "installed source bytes");
    }
    const descriptor = JSON.parse(c.readFile(path.join(installed, "public-release.json")));
    contract.validatePairedSource(metadata, descriptor, receipt.source, receipt.promotion_sha256, product);
    // Only exact-version authoring in this new disposable project; no installer
    // registry repair, user project, or broad E2E replay.
    if (product === "plugin-kit-ai") run(process.execPath, [path.join(installed, "lib/install.js")], project, env);
    const cli = path.join(installed, `bin/${product}.js`);
    const prefix = product === "agentplugins" ? ["author"] : [];
    run(process.execPath, [cli, ...prefix, "--help"], project, env);
    run(process.execPath, [cli, ...prefix, "init", "smoke", "--template=skill", "--name=paired-smoke", "--description=Disposable publication smoke.", "--format=json"], project, env);
    run(process.execPath, [cli, ...prefix, "validate", "smoke"], project, env);
    return { status: "verified", ...byteIdentity(publicBytes) };
  } finally { fs.rmSync(project, { recursive: true, force: true }); }
}
async function publishOnce({ lookup, publish, reconcile }) {
  const existing = await lookup();
  assert.equal(existing, null, "existing npm version: publication forbidden");
  // Even a success response is not proof. A timeout/error is never retried:
  // exact public bytes plus cryptographic provenance are the only reconciliation.
  try { await publish(); } catch { /* ambiguous; read back once, fail closed */ }
  return reconcile();
}
async function publish(selected, repo, scratch, root, npm, product = "agentplugins") {
  const { NAME, VERSION, FILE } = productContract(product);
  assert.equal(process.env.PUBLISH, "true");
  assert.equal(process.env.RUNNER_ENVIRONMENT, "github-hosted");
  assert.equal(process.env.GITHUB_SERVER_URL, "https://github.com");
  assert.equal(process.env.NPM_TOKEN || "", "");
  assert.equal(process.env.NODE_AUTH_TOKEN || "", "");
  checkout(repo, selected.source);
  const { receipt, body } = receiptBytes(root, selected, product);
  // Reauthenticate releases/retained evidence in the protected job and derive
  // the same closed package bytes, without running npm pack a second time.
  const state = admit(selected, repo, scratch);
  const context = packing.npmContext(scratch);
  const source = packing.blobs(repo, selected.source, context.env, "stage");
  const files = packageFiles(source, c.readFile(path.join(state.o.root, product, "release-manifest.json")), state.record, product);
  assert.deepEqual(receipt.entries, Object.fromEntries(Object.entries(files).map(([n, b]) => [n, c.digest(b)])));
  assert.equal(receipt.promotion_sha256, c.digest(milestone.encodeRecord(state.record)));
  packing.verifyPack(path.join(root, FILE), files, path.join(context.root, "verified"), context.env);
  // Allow only the runner OIDC variables and ordinary tool/home paths. Empty
  // user AND global npmrc exclude inherited auth, registry and lifecycle config.
  const env = { PATH: process.env.PATH, HOME: context.root, CI: "true", ...Object.fromEntries(
    ["ACTIONS_ID_TOKEN_REQUEST_URL", "ACTIONS_ID_TOKEN_REQUEST_TOKEN", "GITHUB_ACTIONS", "GITHUB_REPOSITORY",
      "GITHUB_REF", "GITHUB_SHA", "GITHUB_WORKFLOW_REF", "GITHUB_WORKFLOW_SHA", "GITHUB_RUN_ID", "GITHUB_RUN_ATTEMPT",
      "GITHUB_EVENT_NAME", "GITHUB_SERVER_URL", "GITHUB_API_URL", "RUNNER_ENVIRONMENT", "GITHUB_REPOSITORY_ID", "GITHUB_REPOSITORY_OWNER_ID"].map(k => [k, process.env[k]])),
    npm_config_userconfig: context.env.npm_config_userconfig, npm_config_globalconfig: context.env.npm_config_globalconfig,
    npm_config_cache: path.join(context.root, "online-cache"), npm_config_registry: `${REGISTRY}/`, npm_config_ignore_scripts: "true" };
  assert.equal(fs.readFileSync(env.npm_config_userconfig).length, 0);
  assert.equal(fs.readFileSync(env.npm_config_globalconfig).length, 0);
  return publishOnce({ lookup: () => registry(`${REGISTRY}/${NAME}/${VERSION}`, true),
    publish: () => {
      assert.ok(c.readFile(path.join(root, FILE), LIMIT).equals(body));
      return run(process.execPath, [npm, "publish", path.join(root, FILE), "--ignore-scripts", "--provenance", "--access", "public"], scratch, env);
    }, reconcile: () => reconcile(receipt, body, scratch, npm, env, product) });
}
async function main(args) {
  assert.ok(args.length === 3 || args.length === 4);
  const [mode, root, npm, product = "agentplugins"] = args;
  productContract(product);
  assert.ok(["prepare", "publish"].includes(mode));
  assert.ok(path.isAbsolute(root) && path.isAbsolute(npm));
  const selected = selection(process.env), repo = process.cwd();
  const scratch = fs.mkdtempSync(path.join(process.env.RUNNER_TEMP, "paired-publish-"));
  assert.equal(run(process.execPath, [npm, "--version"], scratch).toString().trim(), "12.0.2");
  return mode === "prepare" ? prepare(selected, repo, scratch, root, npm, product) : publish(selected, repo, scratch, root, npm, product);
}
module.exports = { packedSmoke, productContract, admit, reconcile, selection, publicPair, pairIdentity, promotionInvocation, packageFiles, byteIdentity, registry, receiptBytes, publishOnce };
if (require.main === module) main(process.argv.slice(2)).then(result => process.stdout.write(c.encode(result)))
  .catch(() => { process.stderr.write("paired npm publication failed closed; inspect retained evidence before a new dispatch\n"); process.exitCode = 1; });
