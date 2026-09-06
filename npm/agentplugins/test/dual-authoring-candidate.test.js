"use strict";

const assert = require("node:assert/strict");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const cp = require("node:child_process");
const zlib = require("node:zlib");
const test = require("node:test");
const c = require("../scripts/dual-authoring-candidate");
const producer = require("../scripts/stage-dual-authoring-candidate");
const legacy = require("../scripts/release-assets");

const SCOPE = "six-platform-pair";
const freeze = (root, id, hash) => c.frozenCandidate(root, id, hash, SCOPE);
const ID = {
  repository: c.REPOSITORY, commit: "a".repeat(40), engine_revision: "a".repeat(40),
  versions: { agentplugins: "0.1.23", "plugin-kit-ai": "2.0.0" }
};
const temp = () => fs.mkdtempSync(path.join(os.tmpdir(), "dual-candidate-test-"));
// Disposable roots are retained for inspection. No recursive deletion of roots
// (which can contain Git fixtures) is needed by this suite.

function structuralFixture() {
  const root = temp();
  const manifest = {
    schema: c.SCHEMA, status: "CANDIDATE", asset_scope: SCOPE, identity: structuredClone(ID),
    build: { method: "controlled-git-archive-go-build/v1", go_version: "go1.25.13",
      go_sha256: "b".repeat(64), source_archive_sha256: "c".repeat(64), authoring_mode: "vertical-slice-v1" },
    products: {}, release_eligible: false
  };
  for (const product of c.PRODUCTS) {
    const assets = {};
    manifest.products[product] = { version: ID.versions[product], assets };
    for (const target of c.TARGETS) {
      // These are byte-shape fixtures ONLY. No native or revision proof claimed.
      const binary = Buffer.from(`STRUCTURAL ONLY: ${product}/${target}\n`);
      const name = c.executableName(product, target);
      const body = product === "plugin-kit-ai" ? c.archive(binary, name) : binary;
      const file = c.assetName(product, ID.versions[product], target);
      fs.writeFileSync(path.join(root, file), body, { flag: "wx" });
      assets[target] = { file, ...c.metadata(body), binary: { file: name, ...c.metadata(binary) } };
    }
  }
  const save = () => {
    const body = c.encode(manifest);
    fs.writeFileSync(path.join(root, "candidate.json"), body);
    return c.digest(body);
  };
  return { root, manifest, save, hash: save() };
}

test("candidate explicitly separates current producer from unchanged historic identity and six-asset v2", () => {
  assert.equal(c.REPOSITORY, "777genius/universal-agent-plugins");
  assert.equal(legacy.PRODUCER_REPOSITORY, "777genius/plugin-kit-ai");
  assert.deepEqual(Object.keys(legacy.expectedAssets(ID.versions.agentplugins)), c.TARGETS);
  for (const target of c.TARGETS) {
    assert.equal(c.assetName("agentplugins", ID.versions.agentplugins, target), legacy.expectedAssets(ID.versions.agentplugins)[target]);
    assert.equal(c.assetName("plugin-kit-ai", "2.0.0", target), `plugin-kit-ai_2.0.0_${target.replace("-", "_")}.tar.gz`);
  }
  const historical = path.join(__dirname, "fixtures", "historical-evidence");
  assert.equal(c.digest(fs.readFileSync(path.join(historical, "AGENTPLUGINS_CLIENT_E2E.md"))),
    "df6769bf430a337f116cd9df75bcc3ea26df166a016eacf9bc9fbc6cfbf9b100");
  assert.equal(c.digest(fs.readFileSync(path.join(historical, "evidence", "agentplugins-client-e2e-2026-08-30.json"))),
    "437da1bc7423a85b231be139ff9bfbd7e89c942ef216a61ebde668c08a9c2ee3");
});

test("exact two-product identity rejects aliases, ambiguous versions and engine/source drift", () => {
  assert.deepEqual(c.identity(ID), ID);
  const mutations = [
    (v) => { v.repository = "777genius/plugin-kit-ai"; },
    (v) => { v.repository = "other/universal-agent-plugins"; },
    (v) => { v.commit = "a".repeat(39); },
    (v) => { v.commit = "A".repeat(40); },
    (v) => { v.engine_revision = "b".repeat(40); },
    (v) => { v.engine_revision = "unversioned"; },
    (v) => { v.versions.agentplugins = "2.0.0"; },
    (v) => { v.versions.agentplugins = "01.2.3"; },
    (v) => { v.versions.agentplugins = "latest"; },
    (v) => { v.versions.agentplugins = "1.2.3+metadata"; },
    (v) => { v.versions.agentplugins = "1.2.3-rc.1"; },
    (v) => { v.versions.agentplugins = 123; },
    (v) => { v.versions["universal-agent-plugins"] = "0.1.23"; },
    (v) => { delete v.versions["plugin-kit-ai"]; },
    (v) => { v.tag = "v2.0.0"; }
  ];
  for (const mutation of mutations) {
    const invalid = structuredClone(ID);
    mutation(invalid);
    assert.throws(() => c.identity(invalid));
  }
  assert.throws(() => c.assetName("plugin-kit-ai-runtime", "2.0.0", "linux-amd64"));
  assert.throws(() => c.assetName("agentplugins", "2.0.0", "linux-386"));
});

test("STRUCTURAL ONLY: closed candidate schema, every nested field and both exact product sets", () => {
  const fixture = structuralFixture();
  const frozen = freeze(fixture.root, ID, fixture.hash);
  assert.equal(frozen.binaries.length, 12);
  assert.equal(frozen.manifest.release_eligible, false);
  const cases = [
    (v) => { v.schema = 2; }, (v) => { v.asset_scope = "linux-amd64-pair"; }, (v) => { v.status = "RELEASE"; },
    (v) => { v.release_eligible = true; }, (v) => { delete v.release_eligible; },
    (v) => { v.attestation = true; }, (v) => { v.identity.commit = "b".repeat(40); v.identity.engine_revision = v.identity.commit; },
    (v) => { v.identity.engine_revision = "b".repeat(40); },
    (v) => { v.identity.repository = "777genius/plugin-kit-ai"; },
    (v) => { v.identity.versions.agentplugins = "0.1.24"; },
    (v) => { v.build.extra = false; }, (v) => { delete v.build.go_sha256; },
    (v) => { v.build.method = "metadata-only"; }, (v) => { v.build.go_version = "go1.24.0"; },
    (v) => { v.build.go_sha256 = "z".repeat(64); }, (v) => { v.build.authoring_mode = "enabled"; },
    (v) => { v.build.go_sha256 = ["b".repeat(64)]; },
    (v) => { v.build.source_archive_sha256 = ["c".repeat(64)]; },
    (v) => { delete v.products["plugin-kit-ai"]; }, (v) => { v.products.other = v.products.agentplugins; },
    (v) => { v.products.agentplugins.version = "0.1.24"; },
    (v) => { v.products.agentplugins.extra = 1; },
    (v) => { delete v.products.agentplugins.assets["linux-amd64"]; },
    (v) => { v.products.agentplugins.assets["linux-386"] = v.products.agentplugins.assets["linux-amd64"]; },
    (v) => { v.products.agentplugins.assets["linux-amd64"].file = "../payload"; },
    (v) => { v.products.agentplugins.assets["linux-amd64"].extra = true; },
    (v) => { v.products.agentplugins.assets["linux-amd64"].binary.file = "plugin-kit-ai"; },
    (v) => { v.products.agentplugins.assets["linux-amd64"].binary.extra = 0; }
  ];
  for (const change of cases) {
    const value = structuredClone(fixture.manifest);
    change(value);
    assert.throws(() => c.manifestShape(value, ID, SCOPE));
  }
  const pin = fixture.hash;
  fixture.manifest.build.source_archive_sha256 = "d".repeat(64);
  fixture.save();
  assert.throws(() => freeze(fixture.root, ID, pin), /manifest digest/);
  assert.throws(() => freeze(fixture.root, ID), /independent manifest digest/);
});

test("STRUCTURAL ONLY: rejects duplicate JSON keys even when repinned", () => {
  const fixture = structuralFixture();
  const body = Buffer.from(c.encode(fixture.manifest).toString().replace('"status": "CANDIDATE",', '"status": "CANDIDATE", "status": "CANDIDATE",'));
  fs.writeFileSync(path.join(fixture.root, "candidate.json"), body);
  assert.throws(() => freeze(fixture.root, ID, c.digest(body)), /noncanonical/);
});

test("STRUCTURAL ONLY: hash/size fields, missing bytes, extra files, and substituted executable bytes fail", () => {
  for (const field of ["sha256", "size"]) for (const binary of [false, true]) {
    const fixture = structuralFixture();
    const asset = fixture.manifest.products.agentplugins.assets["linux-amd64"];
    (binary ? asset.binary : asset)[field] = field === "size" ? 999 : "0".repeat(64);
    assert.throws(() => freeze(fixture.root, ID, fixture.save()), /digest or size/);
  }
  for (const size of ["4", 0, -1, 1.5, Number.MAX_SAFE_INTEGER + 1]) {
    const fixture = structuralFixture();
    fixture.manifest.products.agentplugins.assets["linux-amd64"].size = size;
    assert.throws(() => freeze(fixture.root, ID, fixture.save()), /digest or size/);
  }
  const missing = structuralFixture();
  fs.unlinkSync(path.join(missing.root, missing.manifest.products.agentplugins.assets["linux-amd64"].file));
  assert.throws(() => freeze(missing.root, ID, missing.hash), /ENOENT/);
  const extra = structuralFixture();
  fs.writeFileSync(path.join(extra.root, "unrelated"), "preserve");
  assert.throws(() => freeze(extra.root, ID, extra.hash), /extra or missing/);
  assert.equal(fs.readFileSync(path.join(extra.root, "unrelated"), "utf8"), "preserve");
  const duplicate = structuralFixture();
  const raw = duplicate.manifest.products.agentplugins.assets["linux-amd64"];
  const kit = duplicate.manifest.products["plugin-kit-ai"].assets["linux-amd64"];
  const rawBytes = fs.readFileSync(path.join(duplicate.root, raw.file));
  const substitute = c.archive(rawBytes, kit.binary.file);
  Object.assign(kit, c.metadata(substitute));
  Object.assign(kit.binary, c.metadata(rawBytes));
  fs.writeFileSync(path.join(duplicate.root, kit.file), substitute);
  assert.throws(() => freeze(duplicate.root, ID, duplicate.save()), /duplicate executable/);
});

test("STRUCTURAL ONLY: verifies frozen bytes independent of later source mutation", () => {
  const fixture = structuralFixture();
  const frozen = freeze(fixture.root, ID, fixture.hash);
  const first = frozen.binaries[0];
  const before = Buffer.from(first.binary);
  const asset = fixture.manifest.products[first.product].assets[first.target];
  fs.writeFileSync(path.join(fixture.root, asset.file), "changed");
  assert.deepEqual(first.binary, before);
  assert.throws(() => freeze(fixture.root, ID, fixture.hash), /digest or size/);
});

test("STRUCTURAL ONLY: symlinks, hardlinks, empty files and directory assets rejected", () => {
  for (const kind of ["symlink", "hardlink", "empty", "directory"]) for (const name of ["candidate.json", "agentplugins_0.1.23_linux_amd64"]) {
    const fixture = structuralFixture();
    const file = path.join(fixture.root, name);
    const held = path.join(temp(), "held");
    fs.renameSync(file, held);
    if (kind === "symlink") fs.symlinkSync(held, file);
    if (kind === "hardlink") fs.linkSync(held, file);
    if (kind === "empty") fs.writeFileSync(file, "");
    if (kind === "directory") fs.mkdirSync(file);
    assert.throws(() => freeze(fixture.root, ID, fixture.hash), /regular.*unaliased/);
  }
  const fixture = structuralFixture();
  const link = path.join(temp(), "link");
  fs.symlinkSync(fixture.root, link, "dir");
  assert.throws(() => freeze(link, ID, fixture.hash), /symlink/);
});

test("safe placement rejects normalization, overlaps, existing outputs and symlink ancestors", () => {
  const root = temp();
  const repo = path.join(root, "repo");
  const scratch = path.join(root, "scratch");
  fs.mkdirSync(repo); fs.mkdirSync(scratch);
  c.outputPlacement(path.join(root, "candidate"), [repo, scratch]);
  for (const output of [repo, root, path.join(repo, "new"), path.join(scratch, "new"), "relative", root + "/../escape", root + "/bad name"]) {
    assert.throws(() => c.outputPlacement(output, [repo, scratch]));
  }
  const link = path.join(root, "link");
  fs.symlinkSync(scratch, link, "dir");
  assert.throws(() => c.outputPlacement(path.join(link, "new"), [repo]), /symlink/);
  const dangling = path.join(root, "dangling");
  fs.symlinkSync(path.join(root, "absent"), dangling);
  assert.throws(() => c.outputPlacement(dangling, [repo]), /already exists/);
});

test("canonical archive rejects path traversal, links, headers, extra records and corrupt gzip", () => {
  const binary = Buffer.from("structural archive fixture");
  const good = c.archive(binary, "plugin-kit-ai");
  assert.deepEqual(c.unpack(good, "plugin-kit-ai"), binary);
  assert.deepEqual(c.archive(binary, "plugin-kit-ai"), good);
  assert.throws(() => c.unpack(good, "plugin-kit-ai.exe"), /noncanonical/);
  for (const [offset, value] of [[0, "../outside"], [156, "1"], [156, "2"], [156, "x"], [345, "prefix"], [148, "000000"], [100, "0004755"]]) {
    const tar = zlib.gunzipSync(good);
    tar.write(value, offset, "ascii");
    assert.throws(() => c.unpack(zlib.gzipSync(tar), "plugin-kit-ai"), /noncanonical/);
  }
  const extra = zlib.gzipSync(Buffer.concat([zlib.gunzipSync(good), Buffer.alloc(512)]));
  assert.throws(() => c.unpack(extra, "plugin-kit-ai"), /noncanonical/);
  assert.throws(() => c.unpack(Buffer.from("not gzip"), "plugin-kit-ai"));
});

test("Go build-info parser binds actual main path, flags, product version, engine, target and compiler", () => {
  const valid = {
    GoVersion: "go1.25.13", Path: "github.com/777genius/plugin-kit-ai/cli/cmd/agentplugins",
    Settings: Object.entries({ GOOS: "linux", GOARCH: "amd64", CGO_ENABLED: "0", "-buildmode": "exe", "-compiler": "gc",
      "-ldflags": c.linkerFlags("agentplugins", ID) }).map(([Key, Value]) => ({ Key, Value }))
  };
  producer.buildInfo(valid, "agentplugins", "linux-amd64", ID);
  const mutations = [
    (v) => { v.Path = v.Path.replace("agentplugins", "plugin-kit-ai"); },
    (v) => { v.GoVersion = "go1.25.12"; },
    (v) => { v.Settings[0].Value = "darwin"; },
    (v) => { v.Settings[1].Value = "arm64"; },
    (v) => { v.Settings[2].Value = "1"; },
    (v) => { v.Settings.pop(); },
    (v) => { v.Settings.at(-1).Value = c.linkerFlags("plugin-kit-ai", ID); },
    (v) => { v.Settings.at(-1).Value = c.linkerFlags("agentplugins", { ...ID, commit: "b".repeat(40) }); },
    (v) => { v.Settings.push(v.Settings[0]); }
  ];
  for (const change of mutations) {
    const invalid = structuredClone(valid); change(invalid);
    assert.throws(() => producer.buildInfo(invalid, "agentplugins", "linux-amd64", ID));
  }
});

test("CLI requires explicit opt-in, exact options and no trailing arguments before any build", () => {
  const root = temp();
  const config = path.join(root, "config.json");
  fs.writeFileSync(config, JSON.stringify({ candidate: false }));
  for (const argv of [[], ["stage", config], ["stage", "--candidate", config, "extra"], ["publish", "--candidate", config]]) {
    assert.throws(() => producer.main(argv), /usage/);
  }
  assert.throws(() => producer.main(["stage", "--candidate", config]), /fields/);
  const options = { candidate: false, repo: root, output: path.join(root, "out"), workParent: root, go: "/missing",
    modCache: root, identity: ID, assetScope: SCOPE };
  assert.throws(() => producer.stageCandidate(options), /opt-in/);
  assert.throws(() => producer.stageCandidate({ ...options, candidate: true, execute: true }), /fields/);
  assert.equal(fs.existsSync(options.output), false);
});

test("offline controlled snapshot matches exact HEAD blobs and rejects a different HEAD", {
  skip: process.env.AGENTPLUGINS_STAGED_TEST_CHILD === "1"
}, () => {
  const repo = path.resolve(__dirname, "../../..");
  const context = producer.privateContext(temp());
  const git = (...args) => cp.execFileSync("/usr/bin/git", args, { cwd: repo, env: context.env, encoding: "utf8" }).trim();
  const commit = git("rev-parse", "HEAD");
  assert.match(producer.sourceSnapshot(repo, commit, context), /^[0-9a-f]{64}$/);
  for (const file of ["go.work", "cli/plugin-kit-ai/cmd/agentplugins/main.go", "cli/plugin-kit-ai/cmd/plugin-kit-ai/main.go"]) {
    const expected = cp.execFileSync("/usr/bin/git", ["show", `${commit}:${file}`], { cwd: repo, env: context.env });
    assert.deepEqual(fs.readFileSync(path.join(context.root, "source", file)), expected);
  }
  assert.equal(fs.existsSync(path.join(context.root, "source", "npm")), false);
  assert.throws(() => producer.sourceSnapshot(repo, "b".repeat(40), producer.privateContext(temp())), /HEAD/);
});

test("private mode is explicit, closed and bound to embedded bytes", () => {
  const mode = "release-cli-contract-v1";
  assert.equal(c.authoringMode(), "vertical-slice-v1");
  assert.throws(() => c.authoringMode("enabled"), /unknown private/);
  assert.notEqual(c.linkerFlags("agentplugins", ID), c.linkerFlags("agentplugins", ID, mode));
  const info = { GoVersion: "go1.25.13", Path: "github.com/777genius/plugin-kit-ai/cli/cmd/agentplugins",
    Settings: Object.entries({ GOOS: "linux", GOARCH: "amd64", CGO_ENABLED: "0", "-buildmode": "exe", "-compiler": "gc",
      "-ldflags": c.linkerFlags("agentplugins", ID, mode) }).map(([Key, Value]) => ({ Key, Value })) };
  producer.buildInfo(info, "agentplugins", "linux-amd64", ID, mode);
  assert.throws(() => producer.buildInfo(info, "agentplugins", "linux-amd64", ID), /build setting mismatch/);
  info.Settings.at(-1).Value = c.linkerFlags("agentplugins", ID);
  assert.throws(() => producer.buildInfo(info, "agentplugins", "linux-amd64", ID, mode), /build setting mismatch/);
});

test("manifest expected mode cannot silently reinterpret historical candidates", () => {
  const f = structuralFixture();
  assert.throws(() => c.frozenCandidate(f.root, ID, f.hash, SCOPE, "release-cli-contract-v1"), /build description/);
  f.manifest.build.authoring_mode = "release-cli-contract-v1";
  const hash = f.save();
  assert.throws(() => freeze(f.root, ID, hash), /build description/);
  assert.equal(c.frozenCandidate(f.root, ID, hash, SCOPE, "release-cli-contract-v1").binaries.length, 12);
});
