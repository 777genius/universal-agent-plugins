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
function intake(request) {
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
if (require.main === module) {
  try {
    const [command, file, digest, commit] = process.argv.slice(2);
    if (command === "seal" && process.argv.length === 5) {
      const request = json(file), result = seal(request), output = absolute(digest);
      // Keep config/evidence outside every observed root; exclusive publication.
      for (const root of [request.fixtureRoot, ...result.inputs.snapshots.map(s => s.root)]) {
        assert.ok(output !== root && !output.startsWith(root + "/"), "output overlaps evidence");
      }
      c.safeDirectory(path.dirname(output));
      fs.writeFileSync(output, c.encode(result), { flag: "wx", mode: 0o600 });
      process.stdout.write(hash(output) + "\n");
    } else if (command === "verify" && process.argv.length === 6) {
      process.stdout.write(c.encode(verify(file, digest, commit)));
    } else throw new Error("usage: node packed-installer-bridge.js seal REQUEST OUTPUT | verify CONFIG SHA256 COMMIT");
  } catch (error) { process.stderr.write(`packed installer bridge: ${error.message}\n`); process.exitCode = 1; }
}
module.exports = { LANES, PRODUCTS, intake, seal, verify, snapshot };
