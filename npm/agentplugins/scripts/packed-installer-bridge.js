#!/usr/bin/env node
"use strict";
// Read-only intake of ROOT's terminal native run. Never builds, installs, launches
// candidate bytes, regenerates projects, or manufactures native completion.
const fs = require("node:fs");
const path = require("node:path");
const crypto = require("node:crypto");
const assert = require("node:assert/strict");
const c = require("./dual-authoring-candidate");
const PRODUCTS = ["agentplugins", "plugin-kit-ai"];
const LANES = ["skill", "mcp-remote", "mcp-stdio", "hybrid-remote", "hybrid-stdio"];
const MODE = "release-cli-contract-v1";
const REQUEST_KEYS = ["expectedCommit", "nativeConfig", "nativeConfigSha256", "nativeCompletionSha256", "fixtureRoot", "disposableEvidence"];
function absolute(p) {
  assert.equal(typeof p, "string");
  assert.ok(path.isAbsolute(p) && path.normalize(p) === p && p !== path.parse(p).root, "canonical absolute path required");
  return p;
}
function inside(root, p) {
  absolute(root); absolute(p);
  const rel = path.relative(root, p);
  assert.ok(rel && !rel.startsWith("../") && rel !== ".." && !path.isAbsolute(rel), "outside disposable evidence boundary");
}
function hash(p) { return c.digest(c.readFile(absolute(p))); }
function pin(p, digest) {
  assert.match(digest, /^[0-9a-f]{64}$/);
  assert.equal(hash(p), digest, `digest mismatch: ${p}`);
}
function json(p) {
  const bytes = c.readFile(absolute(p), 32 * 1024 * 1024), value = JSON.parse(bytes);
  assert.ok(bytes.equals(c.encode(value)), `canonical JSON required (including no duplicate keys): ${p}`);
  return value;
}
// Root and ALL directories (including empty ones), regular-file modes, sizes,
// bytes and entries. No links, hardlinks, devices, sockets or special mode bits.
function snapshot(root, allowContainedLinks = false) {
  c.safeDirectory(absolute(root));
  const entries = [];
  function walk(relative) {
    const file = relative === "." ? root : path.join(root, relative), st = fs.lstatSync(file);
    assert.equal(st.mode & 0o7000, 0, "special mode bits");
    const entry = { path: relative, mode: st.mode & 0o777, kind: st.isDirectory() ? "directory" : "file" };
    if (st.isSymbolicLink()) {
      assert.ok(allowContainedLinks, "symlink in generated or protected input");
      inside(root, fs.realpathSync(file));
      entries.push({ ...entry, kind: "symlink", target: fs.readlinkSync(file) });
    } else if (st.isDirectory()) {
      entries.push(entry);
      for (const name of fs.readdirSync(file).sort()) walk(relative === "." ? name : relative + "/" + name);
    } else {
      assert.ok(st.isFile() && st.nlink === 1, "only unlinked regular evidence files allowed");
      const fd = fs.openSync(file, fs.constants.O_RDONLY | fs.constants.O_NOFOLLOW);
      let bytes;
      try {
        const opened = fs.fstatSync(fd);
        assert.ok(opened.ino === st.ino && opened.dev === st.dev && opened.nlink === 1);
        bytes = fs.readFileSync(fd);
        const after = fs.fstatSync(fd), named = fs.lstatSync(file);
        assert.ok(after.size === bytes.length && after.mtimeMs === st.mtimeMs && after.ctimeMs === st.ctimeMs && named.ino === st.ino && named.dev === st.dev);
      } finally { fs.closeSync(fd); }
      entries.push({ ...entry, size: bytes.length, sha256: c.digest(bytes) });
    }
  }
  walk(".");
  return { root, sha256: c.digest(c.encode(entries)), entries };
}
function falseClaims(record) {
  for (const key of ["release_eligible", "platform_acceptance", "attested"]) assert.equal(record[key], false, key);
}
function privateIntake(request) {
  c.keys(request, REQUEST_KEYS, "bridge request");
  assert.match(request.expectedCommit, /^[0-9a-f]{40}$/);
  assert.equal(request.disposableEvidence, true);
  pin(request.nativeConfig, request.nativeConfigSha256);
  const cfg = json(request.nativeConfig);
  c.keys(cfg, ["stage", "completionDigest", "evidenceOutput"], "native config");
  const o = cfg.stage;
  c.keys(o, ["candidate", "repo", "root", "identity", "manifestDigest", "assetScope", "authoringMode", "go", "workParent", "output", "node", "npm"], "native stage");
  assert.equal(o.candidate, true); c.identity(o.identity);
  assert.equal(o.identity.commit, request.expectedCommit);
  assert.equal(o.authoringMode, MODE); assert.equal(o.assetScope, "linux-amd64-pair");
  for (const p of [o.repo, o.root, o.workParent, o.output, cfg.evidenceOutput, request.fixtureRoot]) c.safeDirectory(absolute(p));
  // This is the exact disposable context made by the native suite, not a user
  // supplied project locator. Source paths below come ONLY from native completion.
  assert.equal(path.dirname(request.fixtureRoot), o.workParent);
  assert.match(path.basename(request.fixtureRoot), /^dual-authoring-[A-Za-z0-9]+$/);
  for (const p of [o.repo, o.root, o.output, cfg.evidenceOutput]) {
    assert.ok(p !== request.fixtureRoot && !p.startsWith(request.fixtureRoot + "/") && !request.fixtureRoot.startsWith(p + "/"), "overlapping evidence roots");
  }
  const nativePath = path.join(cfg.evidenceOutput, "native-completion.json");
  pin(nativePath, request.nativeCompletionSha256);
  const native = json(nativePath);
  c.keys(native, ["kind", "identity", "candidate_sha256", "completion_sha256", "packs", "tools", "inventory_sha256", "invocations", "projects", "release_eligible", "platform_acceptance", "attested", "installer_planner"], "native completion");
  assert.equal(native.kind, "actual-linux-private-npm-pair"); falseClaims(native);
  assert.deepEqual(native.identity, o.identity);
  assert.equal(native.candidate_sha256, o.manifestDigest);
  assert.equal(native.completion_sha256, cfg.completionDigest);
  assert.match(native.inventory_sha256, /^[0-9a-f]{64}$/);
  assert.equal(typeof native.installer_planner, "string");
  const completionPath = path.join(o.output, "completion.json");
  pin(completionPath, cfg.completionDigest);
  const completion = json(completionPath);
  c.keys(completion, ["schema", "status", "identity", "candidate_sha256", "asset_scope", "authoring_mode", "wrapper_blobs", "generated", "packs", "tools", "evidence", "release_eligible", "platform_acceptance", "attested"], "pack completion");
  assert.equal(completion.schema, "dual-authoring-npm-completion/v1"); assert.equal(completion.status, "CANDIDATE");
  falseClaims(completion); assert.deepEqual(completion.identity, o.identity);
  assert.equal(completion.candidate_sha256, o.manifestDigest);
  assert.equal(completion.asset_scope, o.assetScope); assert.equal(completion.authoring_mode, MODE);
  assert.deepEqual(native.packs, completion.packs); assert.deepEqual(native.tools, completion.tools);
  // Byte/shape consistency only; trusted Go build-info/native execution remains
  // the preceding root-owned gate. This function never executes supplied assets.
  c.frozenCandidate(o.root, o.identity, o.manifestDigest, o.assetScope, MODE);
  c.keys(native.packs, PRODUCTS, "packs"); c.keys(native.projects, PRODUCTS, "projects");
  const invocations = json(path.join(cfg.evidenceOutput, "invocations.json"));
  assert.ok(Array.isArray(invocations) && invocations.length > 0);
  assert.equal(native.invocations, invocations.length);
  for (const row of invocations) {
    c.keys(row, ["product", "argv", "status", "signal", "stdout", "stderr"], "invocation");
    assert.ok(PRODUCTS.includes(row.product) && Array.isArray(row.argv) && row.argv.every(a => typeof a === "string"));
    assert.ok([0, 1, 2].includes(row.status)); assert.equal(row.signal, null);
    assert.equal(row.stderr, ""); assert.equal(typeof row.stdout, "string");
  }
  const projects = [], snapshots = [snapshot(o.root), snapshot(o.output), snapshot(cfg.evidenceOutput), snapshot(request.fixtureRoot, true)];
  for (const product of PRODUCTS) {
    const pack = native.packs[product];
    c.keys(pack, ["file", "sha256", "size", "integrity"], "pack");
    const packageName = product === "agentplugins" ? "universal-agent-plugins" : product;
    assert.equal(pack.file, `${packageName}-${o.identity.versions[product]}.tgz`);
    const bytes = c.readFile(path.join(o.output, pack.file));
    assert.equal(c.digest(bytes), pack.sha256); assert.equal(bytes.length, pack.size);
    assert.equal(pack.integrity, "sha512-" + crypto.createHash("sha512").update(bytes).digest("base64"));
    const parent = native.projects[product];
    assert.equal(parent, path.join(request.fixtureRoot, product + " disposable projects"));
    inside(request.fixtureRoot, parent); c.safeDirectory(parent);
    assert.deepEqual(fs.readdirSync(parent).sort(), [...LANES].sort(), "all and only five generated lanes required");
    snapshots.push(snapshot(parent));
    for (const lane of LANES) {
      const source = path.join(parent, lane); c.safeDirectory(source);
      c.readFile(path.join(source, "plugin.json"), 1024 * 1024);
      const init = invocations.filter(row => {
        const args = product === "agentplugins" ? row.argv.slice(1) : row.argv;
        return row.product === product && (product !== "agentplugins" || row.argv[0] === "author") && args[0] === "init" && args[1] === lane && row.status === 0;
      });
      assert.equal(init.length, 1, "missing/ambiguous successful packed init");
      const result = JSON.parse(init[0].stdout);
      assert.equal(result.result, "success"); assert.equal(result.data.committed, true);
      assert.equal(result.data.revision, request.expectedCommit);
      projects.push({ product, lane, source });
    }
  }
  return { identity: o.identity, repo: o.repo, candidate_sha256: o.manifestDigest, packs: native.packs, projects, snapshots };
}
// Public evidence is deliberately a separate schema and opt-in. Never translate
// its synthetic acquisition into private completion or authenticated promotion.
function publicInit(lane) {
  assert.ok(LANES.includes(lane));
  const template = lane.startsWith("hybrid-") ? "hybrid" : lane;
  return ["init", lane, `--template=${template}`,
    ...(lane.endsWith("remote") ? ["--url=https://docs.example.com/mcp"] : lane.endsWith("stdio") ? ["--runtime=node"] : []),
    ...(template === "hybrid" ? ["--mcp-template=mcp-" + lane.split("-")[1]] : [])];
}
function publicEvidence(cfg, native, configPath) {
  c.keys(cfg, ["prepare", "completionDigest", "evidenceOutput"], "public config");
  const o = cfg.prepare, v = o.candidate;
  c.keys(o, ["candidate", "repo", "node", "npm", "output", "projectionPins", "pairMarkerDigest"], "public preparation");
  c.keys(v, ["candidate", "root", "identity", "manifestDigest", "go", "workParent", "assetScope", "authoringMode", "outputs", "pairMarker"], "public candidate");
  c.identity(v.identity); assert.equal(v.candidate, true);
  assert.equal(v.assetScope, "six-platform-pair"); assert.equal(v.authoringMode, MODE);
  c.keys(native, ["schema", "status", "identity", "candidate_sha256", "completion_sha256", "config_sha256",
    "pair_marker_sha256", "projection_pins", "fixtureRoot", "target", "packs", "binaries", "installations", "tools",
    "invocations", "invocations_sha256", "downloads_sha256", "result_sha256", "projects", "trees",
    "fixture_acquisition_execution", "qualification", "signed_promotion", "public_eligible", "release_eligible",
    "platform_acceptance", "attested", "runtime_evidence"], "public terminal");
  assert.equal(native.schema, "dual-authoring-public-native/v1"); assert.equal(native.status, "completed");
  falseClaims(native); assert.equal(native.signed_promotion, false); assert.equal(native.public_eligible, false);
  assert.equal(native.qualification, null); assert.equal(native.fixture_acquisition_execution, true);
  assert.equal(native.runtime_evidence, "not_evaluated"); assert.equal(native.target, "linux-amd64");
  assert.deepEqual(native.identity, v.identity); assert.equal(native.candidate_sha256, v.manifestDigest);
  pin(configPath, native.config_sha256);
  assert.equal(native.completion_sha256, cfg.completionDigest);
  pin(path.join(o.output, "completion.json"), cfg.completionDigest);
  const prep = json(path.join(o.output, "completion.json"));
  c.keys(prep, ["schema", "identity", "candidate_sha256", "projection_pins", "pair_marker_sha256", "wrapper_blobs",
    "generated", "packs", "tools", "qualification", "release_eligible", "platform_acceptance", "attested"], "public preparation completion");
  assert.equal(prep.schema, "dual-authoring-public-preparation/v1"); falseClaims(prep); assert.equal(prep.qualification, null);
  assert.deepEqual(prep.identity, v.identity); assert.equal(prep.candidate_sha256, v.manifestDigest);
  assert.equal(native.pair_marker_sha256, o.pairMarkerDigest); assert.equal(prep.pair_marker_sha256, o.pairMarkerDigest);
  pin(v.pairMarker, o.pairMarkerDigest);
  assert.deepEqual(native.projection_pins, o.projectionPins); assert.deepEqual(prep.projection_pins, o.projectionPins);
  const frozen = c.frozenCandidate(v.root, v.identity, v.manifestDigest, v.assetScope, MODE);
  const pair = json(v.pairMarker);
  assert.deepEqual(pair, { schema: "authoring-release-pair/v1", status: "CANDIDATE", identity: v.identity,
    candidate_sha256: v.manifestDigest, authoring_mode: MODE, asset_scope: v.assetScope, products: o.projectionPins,
    release_eligible: false, platform_acceptance: false, attested: false });
  const roots = [o.repo, v.root, o.output, cfg.evidenceOutput, ...Object.values(v.outputs)];
  for (const root of [...roots, v.workParent, native.fixtureRoot]) c.safeDirectory(absolute(root));
  assert.equal(path.dirname(native.fixtureRoot), v.workParent);
  assert.match(path.basename(native.fixtureRoot), /^dual-authoring-[A-Za-z0-9]+$/);
  const disjoint = [...roots, native.fixtureRoot];
  for (let i = 0; i < disjoint.length; i++) for (const other of disjoint.slice(i + 1)) {
    const a = disjoint[i].toLowerCase(), b = other.toLowerCase();
    assert.ok(a !== b && !a.startsWith(b + "/") && !b.startsWith(a + "/"), "overlapping public roots");
  }
  c.keys(native.tools, ["node", "npm", "go", "producer_node"], "public tools");
  for (const key of ["node", "npm", "go", "producer_node"]) {
    const tool = native.tools[key];
    c.keys(tool, key === "producer_node" ? ["path", "sha256", "version"] : ["path", "sha256"], "tool pin");
    assert.equal(tool.path, key === "go" ? v.go : key === "producer_node" ? o.node : o[key]); pin(tool.path, tool.sha256);
    if (key === "node" || key === "npm") assert.deepEqual(tool, prep.tools[key]);
  }
  assert.match(native.tools.producer_node.version, /^v22\./);
  assert.equal(native.tools.go.sha256, frozen.manifest.build.go_sha256);
  // Source closure hashes are checked against the planner checkout as well as
  // sealed preparation. The Go gate independently requires its exact clean SHA.
  assert.ok(Object.keys(prep.wrapper_blobs).length > 0);
  for (const [name, record] of Object.entries(prep.wrapper_blobs)) {
    const file = path.join(o.repo, name); inside(o.repo, file); pin(file, record.sha256);
  }
  const invocationPath = path.join(cfg.evidenceOutput, "invocations.json"); pin(invocationPath, native.invocations_sha256);
  pin(path.join(cfg.evidenceOutput, "downloads.log"), native.downloads_sha256);
  pin(path.join(cfg.evidenceOutput, "result.json"), native.result_sha256);
  assert.deepEqual(json(path.join(cfg.evidenceOutput, "result.json")), { source: v.identity.commit,
    fixture_acquisition_execution: true, signed_promotion: false, public_eligible: false, runtime_evidence: "not_evaluated" });
  const invocations = json(invocationPath), expected = [], projects = [];
  function command(product, argv, status = 0, author = false) {
    expected.push({ product, argv: [...(author && product === "agentplugins" ? ["author"] : []), ...argv], status, author });
  }
  for (const product of PRODUCTS) {
    for (const map of [native.packs, native.binaries, native.projects, native.trees, prep.packs, v.outputs, o.projectionPins]) c.keys(map, PRODUCTS, "public product map");
    const projection = v.outputs[product], manifestPath = path.join(projection, "release-manifest.json");
    pin(manifestPath, o.projectionPins[product].manifest_sha256);
    pin(path.join(projection, "checksums.txt"), o.projectionPins[product].checksums_sha256);
    const manifest = json(manifestPath);
    assert.deepEqual(manifest, { schema_version: 3, status: "CANDIDATE", product, repository: v.identity.repository,
      tag: product === "agentplugins" ? `agentplugins-v${v.identity.versions[product]}` : `v${v.identity.versions[product]}`,
      version: v.identity.versions[product], commit: v.identity.commit, engine_revision: v.identity.engine_revision,
      versions: v.identity.versions, candidate_sha256: v.manifestDigest, authoring_mode: MODE, asset_scope: v.assetScope,
      assets: frozen.manifest.products[product].assets, release_eligible: false, platform_acceptance: false, attested: false });
    assert.equal(c.readFile(path.join(projection, "checksums.txt")).toString(),
      [...Object.values(manifest.assets).map(a => `${a.sha256}  ${a.file}`), `${hash(manifestPath)}  release-manifest.json`].join("\n") + "\n");
    const asset = manifest.assets[native.target];
    pin(path.join(projection, asset.file), asset.sha256);
    const binary = native.binaries[product]; c.keys(binary, ["path", "sha256", "size"], "executed binary");
    inside(native.fixtureRoot, binary.path); pin(binary.path, asset.binary.sha256);
    assert.equal(binary.sha256, asset.binary.sha256); assert.equal(binary.size, asset.binary.size);
    const pack = native.packs[product]; c.keys(pack, ["file", "sha256", "size", "integrity"], "executed pack");
    const file = `${product === "agentplugins" ? "universal-agent-plugins" : product}-${v.identity.versions[product]}.tgz`;
    assert.equal(pack.file, path.join(cfg.evidenceOutput, product, file));
    pin(pack.file, pack.sha256); const bytes = c.readFile(pack.file);
    assert.equal(bytes.length, pack.size); assert.equal(pack.integrity, "sha512-" + crypto.createHash("sha512").update(bytes).digest("base64"));
    assert.notEqual(pack.sha256, prep.packs[product].sha256, "preparation pack substituted for executed fixture");
    pin(path.join(o.output, file), prep.packs[product].sha256);
    const parent = native.projects[product]; assert.equal(parent, path.join(native.fixtureRoot, `${product} projects ü`));
    assert.deepEqual(fs.readdirSync(parent).sort(), [...LANES].sort());
    assert.deepEqual(native.trees[product], snapshot(parent));
    command(product, ["version", "--format=json"]); command(product, ["--help"]);
    command(product, ["version", "--format=json"], 0, true); command(product, ["--help", "--format=json"], 0, true);
    for (const lane of LANES) {
      const source = path.join(parent, lane); c.readFile(path.join(source, "plugin.json"));
      c.readFile(path.join(source, "skills/extra-skill/SKILL.md")); projects.push({ product, lane, source });
      for (const args of [publicInit(lane), ["skills", "init", "extra-skill", source, "--description=Disposable fixture."],
        ["skills", "validate", source], ...["validate", "inspect", "test"].map(n => [n, source])]) command(product, [...args, "--format=json"], 0, true);
    }
  }
  command("plugin-kit-ai", ["update", "--all", "--format=json"], 2);
  command("agentplugins", ["add", path.join(native.projects.agentplugins, "skill"), "--target=codex", "--dry-run", "--format=json"]);
  assert.equal(native.invocations, expected.length); assert.equal(invocations.length, expected.length);
  invocations.forEach((row, i) => {
    c.keys(row, ["product", "argv", "status", "signal", "stdout", "stderr"], "public invocation");
    const want = expected[i]; assert.equal(row.product, want.product); assert.deepEqual(row.argv, want.argv);
    assert.equal(row.status, want.status); assert.equal(row.signal, null); assert.equal(row.stderr, ""); assert.equal(typeof row.stdout, "string");
    if (want.author) {
      const result = JSON.parse(row.stdout); assert.equal(result.result, "success"); assert.equal(result.schema_version, 1);
      assert.equal(result.data.revision, v.identity.commit); assert.equal(result.data.engine, "standard-first-slice/1");
      if (row.argv.includes("init")) assert.equal(result.data.committed, true);
    }
  });
  const installs = [...PRODUCTS, ...PRODUCTS, "agentplugins"]; // independent/shared/reinstall
  assert.equal(native.installations.length, installs.length);
  native.installations.forEach((row, i) => {
    c.keys(row, ["product", "argv", "status", "signal", "pack_sha256"], "public installation");
    const p = installs[i], prefix = path.join(native.fixtureRoot, i < 2 ? `${p} independent prefix` : "shared prefix ü");
    assert.equal(row.product, p); assert.equal(row.status, 0); assert.equal(row.signal, null);
    assert.equal(row.pack_sha256, native.packs[p].sha256);
    assert.deepEqual(row.argv, [o.npm, "install", "--global", "--prefix", prefix, "--offline", "--ignore-scripts", "--no-audit", "--no-fund", native.packs[p].file]);
  });
  return { identity: v.identity, repo: o.repo, candidate_sha256: v.manifestDigest, packs: native.packs, projects,
    snapshots: [...roots.slice(1).map(root => snapshot(root)), snapshot(native.fixtureRoot, true)],
    public_evidence: { schema: native.schema, signed_promotion: false, public_eligible: false, qualification: null,
      pair_marker_sha256: native.pair_marker_sha256, tools: native.tools, binaries: native.binaries } };
}
function intake(request) {
  if (!Object.hasOwn(request, "intake")) return privateIntake(request);
  c.keys(request, [...REQUEST_KEYS, "intake"], "public bridge request");
  assert.equal(request.intake, "public-fixture/v1"); assert.equal(request.disposableEvidence, true);
  pin(request.nativeConfig, request.nativeConfigSha256);
  const cfg = json(request.nativeConfig);
  const terminal = path.join(cfg.evidenceOutput, "public-native-completion.json"); pin(terminal, request.nativeCompletionSha256);
  const native = json(terminal);
  assert.equal(native.identity.commit, request.expectedCommit); assert.equal(native.fixtureRoot, request.fixtureRoot);
  return publicEvidence(cfg, native, request.nativeConfig);
}
function seal(request) {
  const inputs = intake(request);
  return { schema: "packed-installer-bridge/v1", request, verifier_sha256: hash(__filename), helper_sha256: hash(require.resolve("./dual-authoring-candidate")), inputs,
    release_eligible: false, platform_acceptance: false, attested: false };
}
function verify(configFile, configDigest, expectedCommit) {
  pin(configFile, configDigest);
  const cfg = json(configFile);
  c.keys(cfg, ["schema", "request", "verifier_sha256", "helper_sha256", "inputs", "release_eligible", "platform_acceptance", "attested"], "bridge config");
  assert.equal(cfg.schema, "packed-installer-bridge/v1"); falseClaims(cfg);
  assert.equal(cfg.request.expectedCommit, expectedCommit);
  assert.deepEqual(cfg, seal(cfg.request), "sealed input changed");
  return cfg.inputs;
}
function publishSeal(request, output) {
  const result = seal(request); absolute(output);
  for (const root of [result.inputs.repo, request.fixtureRoot, ...result.inputs.snapshots.map(s => s.root)]) {
    const a = output.toLowerCase(), b = root.toLowerCase();
    assert.ok(a !== b && !a.startsWith(b + "/") && !b.startsWith(a + "/"), "output overlaps evidence/source");
  }
  c.safeDirectory(path.dirname(output));
  fs.writeFileSync(output, c.encode(result), { flag: "wx", mode: 0o600 });
  return hash(output);
}
if (require.main === module) {
  try {
    const [command, file, digest, commit] = process.argv.slice(2);
    if (command === "seal" && process.argv.length === 5) {
      process.stdout.write(publishSeal(json(file), digest) + "\n");
    } else if (command === "verify" && process.argv.length === 6) {
      process.stdout.write(c.encode(verify(file, digest, commit)));
    } else throw new Error("usage: node packed-installer-bridge.js seal REQUEST OUTPUT | verify CONFIG SHA256 COMMIT");
  } catch (error) { process.stderr.write(`packed installer bridge: ${error.message}\n`); process.exitCode = 1; }
}
module.exports = { LANES, PRODUCTS, intake, seal, verify, snapshot, publicInit, publicEvidence, publishSeal };
