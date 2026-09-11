"use strict";

const assert = require("node:assert/strict");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const cp = require("node:child_process");
const nodeTest = require("node:test");
// Historical v2 detached packages do not adopt this repository-native producer.
const test = (name, fn) => nodeTest(name, { skip: process.env.AGENTPLUGINS_STAGED_TEST_CHILD === "1" }, fn);
const c = require("../scripts/dual-authoring-candidate");
const release = require("../scripts/release-assets");

const MODE = "release-cli-contract-v1";
const SCOPE = "six-platform-pair";
const ID = { repository: c.REPOSITORY, commit: "a".repeat(40), engine_revision: "a".repeat(40),
  versions: { agentplugins: "0.1.54", "plugin-kit-ai": "2.0.0" } };

// Structural fixtures, not native acceptance. The trusted-tool stub returns
// embedded fixture records only for env/version; any compile or subject launch
// is a hard failure. Real verifier/path/archive/projection code runs unchanged.
function fixture() {
  const sandbox = fs.mkdtempSync(path.join(os.tmpdir(), "authoring-producer-"));
  const root = path.join(sandbox, "candidate");
  const workParent = path.join(sandbox, "work");
  const tools = path.join(sandbox, "tools");
  for (const dir of [root, workParent, tools]) fs.mkdirSync(dir);
  const go = path.join(tools, "structural-go");
  fs.writeFileSync(go, `#!${process.execPath}
const fs = require('node:fs');
const args = process.argv.slice(2);
fs.appendFileSync(${JSON.stringify(path.join(sandbox, "calls.jsonl"))}, JSON.stringify(args) + '\\n');
if (JSON.stringify(args) === JSON.stringify(['env', '-json', 'GOVERSION', 'GOHOSTOS', 'GOHOSTARCH'])) {
  console.log(JSON.stringify({ GOVERSION: 'go1.25.13', GOHOSTOS: 'linux', GOHOSTARCH: 'amd64' }));
} else if (args.length === 4 && args.slice(0, 3).join(' ') === 'version -m -json') {
  const stat = fs.statSync(args[3]);
  if (stat.mode & 0o111) throw Error('subject executable permission');
  process.stdout.write(fs.readFileSync(args[3]));
} else throw Error('compile or unexpected tool invocation forbidden');
`, { mode: 0o755 });
  const manifest = { schema: c.SCHEMA, status: "CANDIDATE", identity: structuredClone(ID), asset_scope: SCOPE,
    build: { method: "controlled-git-archive-go-build/v1", go_version: "go1.25.13", go_sha256: c.digest(c.readFile(go)),
      source_archive_sha256: "c".repeat(64), authoring_mode: MODE }, products: {}, release_eligible: false };
  for (const product of c.PRODUCTS) {
    const assets = {};
    manifest.products[product] = { version: ID.versions[product], assets };
    for (const target of c.TARGETS) {
      const [GOOS, GOARCH] = target.split("-");
      const binary = c.encode({ GoVersion: "go1.25.13", Path: `github.com/777genius/plugin-kit-ai/cli/cmd/${product}`,
        Settings: Object.entries({ GOOS, GOARCH, CGO_ENABLED: "0", "-buildmode": "exe", "-compiler": "gc",
          "-ldflags": c.linkerFlags(product, ID, MODE) }).map(([Key, Value]) => ({ Key, Value })) });
      const file = c.assetName(product, ID.versions[product], target);
      const name = c.executableName(product, target);
      const body = product === "plugin-kit-ai" ? c.archive(binary, name) : binary;
      fs.writeFileSync(path.join(root, file), body, { mode: 0o444 });
      assets[target] = { file, ...c.metadata(body), binary: { file: name, ...c.metadata(binary) } };
    }
  }
  const options = { candidate: true, root, identity: structuredClone(ID), manifestDigest: "", go, workParent,
    assetScope: SCOPE, authoringMode: MODE,
    outputs: { agentplugins: path.join(sandbox, "agentplugins"), "plugin-kit-ai": path.join(sandbox, "plugin-kit-ai") },
    pairMarker: path.join(sandbox, "pair-prepared.json") };
  const save = () => {
    fs.writeFileSync(path.join(root, "candidate.json"), c.encode(manifest));
    options.manifestDigest = c.digest(c.encode(manifest));
  };
  save();
  return { sandbox, manifest, options, save };
}

function noOutput(f) {
  assert.equal(fs.existsSync(f.options.pairMarker), false);
  for (const root of Object.values(f.options.outputs)) assert.equal(fs.existsSync(root), false);
}

function rewriteBinary(f, product, change) {
  const asset = f.manifest.products[product].assets["linux-amd64"];
  const file = path.join(f.options.root, asset.file);
  const before = fs.readFileSync(file);
  const info = JSON.parse(product === "plugin-kit-ai" ? c.unpack(before, asset.binary.file) : before);
  change(info);
  const binary = c.encode(info);
  const body = product === "plugin-kit-ai" ? c.archive(binary, asset.binary.file) : binary;
  fs.chmodSync(file, 0o644);
  fs.writeFileSync(file, body);
  Object.assign(asset, c.metadata(body));
  Object.assign(asset.binary, c.metadata(binary));
  f.save();
}

test("v3 prepares and verifies exactly both pinned products without compilation or execution", () => {
  const f = fixture();
  const candidateBefore = Object.fromEntries(fs.readdirSync(f.options.root).map((name) => [name, c.digest(c.readFile(path.join(f.options.root, name)))]));
  const prepared = release.prepareAuthoringRelease(f.options);
  const verified = release.verifyAuthoringRelease(f.options);
  assert.equal(verified.consistency_verified, true);
  assert.deepEqual(prepared, JSON.parse(fs.readFileSync(f.options.pairMarker)));
  for (const claims of [prepared, verified]) for (const flag of ["release_eligible", "platform_acceptance", "attested"]) assert.equal(claims[flag], false);
  for (const product of c.PRODUCTS) {
    const root = f.options.outputs[product];
    const manifest = JSON.parse(fs.readFileSync(path.join(root, "release-manifest.json")));
    assert.equal(manifest.schema_version, 3);
    assert.equal(manifest.product, product);
    assert.equal(manifest.repository, c.REPOSITORY);
    assert.equal(manifest.commit, ID.commit);
    assert.equal(manifest.engine_revision, ID.commit);
    assert.deepEqual(manifest.versions, ID.versions);
    assert.equal(manifest.candidate_sha256, f.options.manifestDigest);
    assert.equal(manifest.tag, product === "agentplugins" ? "agentplugins-v0.1.54" : "v2.0.0");
    assert.equal(manifest.authoring_mode, MODE);
    assert.equal(manifest.status, "CANDIDATE");
    assert.deepEqual(Object.keys(manifest.assets), c.TARGETS);
    assert.equal(fs.readdirSync(root).length, 8);
    for (const asset of Object.values(manifest.assets)) {
      assert.deepEqual(c.readFile(path.join(root, asset.file)), c.readFile(path.join(f.options.root, asset.file)));
      assert.notEqual(fs.statSync(path.join(root, asset.file)).ino, fs.statSync(path.join(f.options.root, asset.file)).ino);
      assert.equal(asset.binary.sha256, f.manifest.products[product].assets[Object.keys(manifest.assets).find((k) => manifest.assets[k] === asset)].binary.sha256);
    }
  }
  for (const [name, digest] of Object.entries(candidateBefore)) assert.equal(c.digest(c.readFile(path.join(f.options.root, name))), digest);
  const calls = fs.readFileSync(path.join(f.sandbox, "calls.jsonl"), "utf8").trim().split("\n").map(JSON.parse);
  assert.equal(calls.filter((args) => args[0] === "version").length, 24);
  assert.equal(calls.filter((args) => args[0] === "env").length, 2);
  assert.throws(() => release.verifyRelease(f.options.outputs.agentplugins, "agentplugins-v0.1.54", ID.commit), /schema/);
});

const invalidCandidates = {
  "missing product": (f) => { delete f.manifest.products["plugin-kit-ai"]; },
  "missing target": (f) => { delete f.manifest.products.agentplugins.assets["darwin-arm64"]; },
  "Linux-only": (f) => { f.manifest.asset_scope = "linux-amd64-pair"; },
  "vertical mode": (f) => { f.manifest.build.authoring_mode = "vertical-slice-v1"; },
  "empty engine": (f) => { f.manifest.identity.engine_revision = ""; },
  "unversioned engine": (f) => { f.manifest.identity.engine_revision = "unversioned"; },
  "stale engine": (f) => { f.manifest.identity.engine_revision = "b".repeat(40); },
  "stale source and engine": (f) => { f.manifest.identity.commit = f.manifest.identity.engine_revision = "b".repeat(40); },
  "historical repository": (f) => { f.manifest.identity.repository = "777genius/plugin-kit-ai"; },
  "version swap": (f) => { f.manifest.identity.versions = { agentplugins: "2.0.0", "plugin-kit-ai": "0.1.54" }; },
  "product swap": (f) => { [f.manifest.products.agentplugins, f.manifest.products["plugin-kit-ai"]] = [f.manifest.products["plugin-kit-ai"], f.manifest.products.agentplugins]; },
  "asset corruption": (f) => { const file = path.join(f.options.root, f.manifest.products["plugin-kit-ai"].assets["linux-amd64"].file); fs.chmodSync(file, 0o644); fs.writeFileSync(file, "corrupt"); },
  "inner binary digest": (f) => { f.manifest.products["plugin-kit-ai"].assets["linux-amd64"].binary.sha256 = "d".repeat(64); },
  "extra file": (f) => { fs.writeFileSync(path.join(f.options.root, "unrelated"), "preserve"); },
  "approval flag": (f) => { f.manifest.release_eligible = true; }
};
for (const [name, mutate] of Object.entries(invalidCandidates)) test(`reject candidate ${name} before outputs`, () => {
  const f = fixture(); mutate(f); f.save();
  assert.throws(() => release.prepareAuthoringRelease(f.options));
  noOutput(f);
});

test("wrong or missing independent pin and invalid caller identity fail closed", () => {
  for (const change of [
    (o) => { o.manifestDigest = "d".repeat(64); }, (o) => { o.manifestDigest = ""; },
    (o) => { o.identity.repository = "777genius/plugin-kit-ai"; },
    (o) => { o.identity.versions["plugin-kit-ai"] = "2.0.1"; },
    (o) => { o.identity.commit = o.identity.engine_revision = "0".repeat(40); },
    (o) => { o.authoringMode = "vertical-slice-v1"; }, (o) => { delete o.authoringMode; },
    (o) => { o.assetScope = "linux-amd64-pair"; }, (o) => { o.candidate = false; }
  ]) {
    const f = fixture(); change(f.options);
    assert.throws(() => release.prepareAuthoringRelease(f.options)); noOutput(f);
  }
});

for (const product of c.PRODUCTS) test(`independent byte inspection rejects repinned ${product} inner identity`, () => {
  for (const change of [
    (info) => { info.Path = info.Path.replace(`/cmd/${product}`, "/cmd/other-product"); },
    (info) => { info.Settings.at(-1).Value = c.linkerFlags(product, { ...ID, commit: "b".repeat(40) }, MODE); },
    (info) => { info.Settings.at(-1).Value = c.linkerFlags(product, ID); },
    (info) => { info.Settings.at(-1).Value = c.linkerFlags(product, { ...ID, versions: { ...ID.versions, [product]: "9.9.9" } }, MODE); }
  ]) {
    const f = fixture(); rewriteBinary(f, product, change);
    assert.throws(() => release.prepareAuthoringRelease(f.options), /embedded Go/); noOutput(f);
  }
});

test("absent external disjoint output policy rejects alias, overlap, existing and symlink paths", () => {
  for (const kind of ["same", "nested", "case", "candidate", "scratch", "repo", "root-scratch", "existing", "symlink", "marker", "marker-existing"]) {
    const f = fixture();
    const original = f.options.outputs.agentplugins;
    if (kind === "same") f.options.outputs["plugin-kit-ai"] = original;
    if (kind === "nested") f.options.outputs["plugin-kit-ai"] = path.join(original, "nested");
    if (kind === "case") f.options.outputs["plugin-kit-ai"] = original.replace(/agentplugins$/, "AGENTPLUGINS");
    if (kind === "candidate") f.options.outputs.agentplugins = path.join(f.options.root, "out");
    if (kind === "scratch") f.options.outputs.agentplugins = path.join(f.options.workParent, "out");
    if (kind === "repo") f.options.outputs.agentplugins = path.resolve(__dirname, "../out");
    if (kind === "root-scratch") f.options.workParent = path.parse(f.sandbox).root;
    if (kind === "existing") { fs.mkdirSync(original); fs.writeFileSync(path.join(original, "unrelated"), "preserve"); }
    if (kind === "symlink") { const link = path.join(f.sandbox, "link"); fs.symlinkSync(f.options.workParent, link); f.options.outputs.agentplugins = path.join(link, "out"); }
    if (kind === "marker") f.options.pairMarker = path.join(original, "pair.json");
    if (kind === "marker-existing") fs.writeFileSync(f.options.pairMarker, "unrelated");
    assert.throws(() => release.prepareAuthoringRelease(f.options));
    assert.equal(fs.existsSync(path.join(f.sandbox, "calls.jsonl")), false, "placement must fail before tool/scratch effects");
    if (kind === "existing") assert.equal(fs.readFileSync(path.join(original, "unrelated"), "utf8"), "preserve");
    if (kind === "marker-existing") assert.equal(fs.readFileSync(f.options.pairMarker, "utf8"), "unrelated");
  }
});

test("verification rejects mutated outputs, missing pair marker, links and metadata/claim substitution", () => {
  const f = fixture(); release.prepareAuthoringRelease(f.options);
  const root = f.options.outputs.agentplugins;
  const files = Object.values(f.options.outputs).flatMap((dir) => fs.readdirSync(dir).map((name) => path.join(dir, name)));
  for (const file of [...files, f.options.pairMarker]) {
    const body = fs.readFileSync(file);
    fs.chmodSync(file, 0o644);
    fs.writeFileSync(file, "corrupt");
    assert.throws(() => release.verifyAuthoringRelease(f.options));
    fs.writeFileSync(file, body);
  }
  const heldMarker = path.join(f.sandbox, "held-marker");
  fs.renameSync(f.options.pairMarker, heldMarker);
  assert.throws(() => release.verifyAuthoringRelease(f.options), /ENOENT/);
  fs.renameSync(heldMarker, f.options.pairMarker);
  const manifestFile = path.join(root, "release-manifest.json");
  const original = fs.readFileSync(manifestFile);
  for (const change of [
    (v) => { v.product = "plugin-kit-ai"; }, (v) => { v.attested = true; },
    (v) => { v.repository = "777genius/plugin-kit-ai"; }, (v) => { v.engine_revision = "b".repeat(40); }
  ]) {
    const value = JSON.parse(original); change(value); fs.writeFileSync(manifestFile, c.encode(value));
    assert.throws(() => release.verifyAuthoringRelease(f.options), /manifest/);
  }
  fs.writeFileSync(manifestFile, original);
  for (const link of [false, true]) {
    const held = path.join(f.sandbox, link ? "held-symbolic" : "held-hard");
    fs.renameSync(manifestFile, held);
    if (link) fs.symlinkSync(held, manifestFile); else fs.linkSync(held, manifestFile);
    assert.throws(() => release.verifyAuthoringRelease(f.options), /unaliased/);
    fs.unlinkSync(manifestFile); fs.renameSync(held, manifestFile);
  }
  fs.writeFileSync(path.join(root, "unrelated"), "preserve");
  assert.throws(() => release.verifyAuthoringRelease(f.options), /extra/);
  assert.equal(fs.readFileSync(path.join(root, "unrelated"), "utf8"), "preserve");
});

for (const fault of ["second-product", "corrupt-copy", "marker-write", "marker-close"]) test(`partial ${fault} never leaves pair success`, (t) => {
  const f = fixture();
  const write = fs.writeFileSync;
  let markerFd;
  const open = fs.openSync;
  t.mock.method(fs, "openSync", function(file, ...rest) {
    const fd = open.call(this, file, ...rest);
    if (file === f.options.pairMarker) markerFd = fd;
    return fd;
  });
  t.mock.method(fs, "writeFileSync", function(file, body, ...rest) {
    if (fault === "second-product" && typeof file === "string" && file.startsWith(f.options.outputs["plugin-kit-ai"] + path.sep)) throw Error("copy failure");
    if (fault === "marker-write" && file === markerFd) { write.call(this, file, "partial"); throw Error("marker failure"); }
    if (fault === "corrupt-copy" && typeof file === "string" && file.startsWith(f.options.outputs["plugin-kit-ai"] + path.sep)) body = Buffer.from("corrupt");
    return write.call(this, file, body, ...rest);
  });
  const close = fs.closeSync;
  t.mock.method(fs, "closeSync", function(fd) {
    const result = close.call(this, fd);
    if (fault === "marker-close" && fd === markerFd) throw Error("close failure");
    return result;
  });
  assert.throws(() => release.prepareAuthoringRelease(f.options));
  assert.equal(fs.existsSync(f.options.pairMarker), false);
  assert.equal(fs.readdirSync(f.options.outputs.agentplugins).length, 8, "first product remains inspectable");
});

test("explicit CLI prepare-authoring and verify-authoring use the pinned options contract", () => {
  const f = fixture();
  const script = path.resolve(__dirname, "../scripts/release-assets.js");
  const config = path.join(f.sandbox, "options.json");
  fs.writeFileSync(config, c.encode(f.options));
  for (const command of ["prepare-authoring", "verify-authoring"]) {
    const result = cp.spawnSync(process.execPath, [script, command, config], { encoding: "utf8" });
    assert.equal(result.status, 0, result.stderr);
    assert.equal(JSON.parse(result.stdout).attested, false);
  }
  for (const args of [["verify-authoring"], ["verify-authoring", "relative"], ["verify-authoring", config, "extra"]]) {
    const result = cp.spawnSync(process.execPath, [script, ...args], { encoding: "utf8" });
    assert.equal(result.status, 1); assert.match(result.stderr, /absolute options/);
  }
});

// Existing publication scripts can still copy just release-assets.js.
test("historical standalone v1/v2 script does not require the authoring adapter", () => {
  const f = fixture();
  const standalone = path.join(f.sandbox, "legacy-release-assets.js");
  fs.copyFileSync(path.resolve(__dirname, "../scripts/release-assets.js"), standalone);
  const legacy = require(standalone);
  const assets = path.join(f.sandbox, "historical");
  fs.mkdirSync(assets);
  for (const file of Object.values(legacy.expectedAssets("0.1.23"))) fs.writeFileSync(path.join(assets, file), file);
  legacy.prepareRelease(assets, "agentplugins-v0.1.23", ID.commit);
  const verified = legacy.verifyRelease(assets, "agentplugins-v0.1.23", ID.commit);
  assert.equal(verified.repository, "777genius/plugin-kit-ai");
  assert.equal(verified.manifest_schema, 2);
  assert.equal(verified.gate_eligible, true); // Historical contract, not authoring approval.
});
