"use strict";

const assert = require("node:assert/strict");
const fs = require("node:fs");
const fsp = require("node:fs/promises");
const os = require("node:os");
const path = require("node:path");
const { EventEmitter } = require("node:events");
const { PassThrough } = require("node:stream");
const test = require("node:test");
const c = require("../scripts/dual-authoring-candidate");
const publicAPI = require("../lib/public-authoring");
const stager = require("../scripts/stage-authoring-npm");
const v = require("../lib/verifier");
const REPO = path.resolve(__dirname, "../../..");

// Synthetic package binding only. This fixture does not claim signed promotion,
// current qualification, or bytes built from this writer's uncommitted source.
function fixture(product, binaryFor = (p, t) => Buffer.from(`fixture ${p} ${t}\n`), parent = os.tmpdir()) {
  const root = fs.mkdtempSync(path.join(parent, "public fixture ü "));
  const packageRoot = path.join(root, product), cacheRoot = path.join(root, "cache");
  fs.mkdirSync(packageRoot, { mode: 0o700 });
  fs.mkdirSync(cacheRoot, { mode: 0o700 });
  const identity = { repository: c.REPOSITORY, commit: "1".repeat(40), engine_revision: "1".repeat(40),
    versions: { agentplugins: "0.1.99", "plugin-kit-ai": "2.0.0" } };
  const candidate = { identity, manifestDigest: "2".repeat(64) };
  const assets = {}, bodies = {};
  for (const t of c.TARGETS) {
    const binary = binaryFor(product, t), name = c.executableName(product, t);
    const body = product === "plugin-kit-ai" ? c.archive(binary, name) : binary;
    const file = c.assetName(product, identity.versions[product], t);
    assets[t] = { file, ...c.metadata(body), binary: { file: name, ...c.metadata(binary) } };
    bodies[file] = body;
  }
  const manifest = { schema_version: 3, status: "CANDIDATE", product, repository: c.REPOSITORY,
    tag: product === "agentplugins" ? "agentplugins-v0.1.99" : "v2.0.0", version: identity.versions[product],
    commit: identity.commit, engine_revision: identity.engine_revision, versions: identity.versions,
    candidate_sha256: candidate.manifestDigest, authoring_mode: publicAPI.MODE, asset_scope: publicAPI.SCOPE,
    assets, release_eligible: false, platform_acceptance: false, attested: false };
  const source = Object.fromEntries(stager.ALLOWLIST.map(n => [n, { bytes: fs.readFileSync(path.join(REPO, n)) }]));
  const files = stager.packageFiles(product, source, c.encode(manifest), candidate);
  for (const [n, bytes] of Object.entries(files)) {
    const file = path.join(packageRoot, n);
    fs.mkdirSync(path.dirname(file), { recursive: true, mode: 0o700 });
    fs.writeFileSync(file, bytes, { mode: n.startsWith("bin/") && n.endsWith(".js") ? 0o755 : 0o644 });
  }
  const descriptor = JSON.parse(files["public-release.json"]);
  const qualify = () => {
    descriptor.qualification = { identity, candidate_sha256: candidate.manifestDigest,
      manifest_sha256: { agentplugins: "3".repeat(64), "plugin-kit-ai": "4".repeat(64), [product]: c.digest(c.encode(manifest)) },
      signed_subject: { sha256: "5".repeat(64), workflow: `${c.REPOSITORY}/.github/workflows/fixture-only.yml`, source: identity.commit } };
    save();
  };
  const save = () => {
    fs.writeFileSync(path.join(packageRoot, "public-release.json"), c.encode(descriptor));
    fs.writeFileSync(path.join(packageRoot, "release-manifest.json"), c.encode(manifest));
  };
  return { root, packageRoot, cacheRoot, descriptor, manifest, bodies, files, identity, qualify, save };
}
function transport(f, seen = [], alter = res => res) {
  return (url, options) => {
    seen.push(url.toString());
    assert.deepEqual(Object.keys(options.headers).sort(), ["Accept", "User-Agent"]);
    assert.equal(url.hostname, "github.com");
    assert.equal(url.pathname, `/${c.REPOSITORY}/releases/download/${f.manifest.tag}/${path.basename(url.pathname)}`);
    const req = new EventEmitter();
    req.destroy = e => req.emit("error", e);
    req.setTimeout = (ms, fn) => { assert.equal(ms, 30000); req.timeout = fn; };
    process.nextTick(() => {
      const res = new PassThrough(); res.statusCode = 200; res.headers = {};
      const body = f.bodies[path.basename(url.pathname)];
      assert.ok(body, "selected product only");
      alter(res, req, body);
      if (!req.done) { req.emit("response", res); res.end(body); }
    });
    return req;
  };
}
function options(f, extra = {}) { return { packageRoot: f.packageRoot, cacheRoot: f.cacheRoot, request: transport(f), ...extra }; }
const target = `${process.platform === "win32" ? "windows" : process.platform}-${process.arch === "x64" ? "amd64" : process.arch}`;

if (require.main === module) {
  for (const p of c.PRODUCTS) {
    test(`${p}: preparation and forged bypass reject before any cache/download effect`, async () => {
      const f = fixture(p);
      const before = fs.readdirSync(f.cacheRoot);
      let requests = 0;
      process.env.UAP_PUBLIC_AUTHORING_VERIFIED = "true";
      try {
        await assert.rejects(publicAPI.ensureBinary(p, options(f, { verified: true, request: () => { requests++; } })), /not qualified/);
        assert.equal(requests, 0); assert.deepEqual(fs.readdirSync(f.cacheRoot), before);
      } finally { delete process.env.UAP_PUBLIC_AUTHORING_VERIFIED; }
    });
    test(`${p}: cold, offline warm, corrupt recovery and isolation`, async () => {
      const f = fixture(p); f.qualify(); const seen = [];
      const cold = await publicAPI.ensureBinary(p, options(f, { request: transport(f, seen) }));
      assert.equal(cold.cacheHit, false); assert.equal(seen.length, 1);
      assert.ok(cold.binaryPath.includes(`/public-authoring-v1/${publicAPI.MODE}/${f.identity.commit}/`));
      const warm = await publicAPI.ensureBinary(p, options(f, { request: () => assert.fail("warm network") }));
      assert.equal(warm.cacheHit, true); assert.equal(warm.binaryPath, cold.binaryPath);
      assert.equal(fs.statSync(cold.binaryPath).mode & 0o777, 0o755);
      fs.writeFileSync(cold.binaryPath, "corrupt");
      assert.equal((await publicAPI.ensureBinary(p, options(f))).cacheHit, false);
      assert.equal(c.digest(fs.readFileSync(cold.binaryPath)), f.manifest.assets[target].binary.sha256);
      f.descriptor.qualification = null; f.save();
      await assert.rejects(publicAPI.ensureBinary(p, options(f)), /not qualified/);
      assert.deepEqual(fs.readdirSync(path.join(f.cacheRoot, "public-authoring-v1")).sort(), [".locks", publicAPI.MODE].sort());
    });
    const mutations = [
      ["product", f => { f.descriptor.product = "peer"; }],
      ["npm", f => { f.descriptor.npm_package = "peer"; }],
      ["repo", f => { f.descriptor.identity.repository = "owner/other"; }],
      ["zero source", f => { f.descriptor.identity.commit = "0".repeat(40); }],
      ["engine", f => { f.manifest.engine_revision = "a".repeat(40); }],
      ["peer version", f => { f.manifest.versions = { ...f.manifest.versions, [p === "agentplugins" ? "plugin-kit-ai" : "agentplugins"]: "9.0.0" }; }],
      ["tag", f => { f.manifest.tag = "latest"; }],
      ["mode", f => { f.manifest.authoring_mode = "vertical-slice-v1"; }],
      ["scope", f => { f.manifest.asset_scope = "linux-amd64-pair"; }],
      ["claim", f => { f.manifest.attested = true; }],
      ["status", f => { f.manifest.status = "RELEASE"; }],
      ["missing target", f => { delete f.manifest.assets["windows-arm64"]; }],
      ["unsafe name", f => { f.manifest.assets[target].file = "../binary"; }],
      ["unsafe inner name", f => { f.manifest.assets[target].binary.file = "peer"; }],
      ["unbounded size", f => { f.manifest.assets[target].size = 2 ** 30; }],
      ["extra field", f => { f.manifest.extra = true; }],
      ["stale proof", f => { f.descriptor.qualification.signed_subject.source = "a".repeat(40); }],
      ["forged proof", f => { f.descriptor.qualification = true; }],
      ["missing proof", f => { delete f.descriptor.qualification; }],
      ["wrong subject", f => { f.descriptor.qualification.signed_subject.sha256 = "bad"; }]
    ];
    for (const [label, mutate] of mutations) test(`${p}: ${label} rejects warm and cold`, async () => {
      for (const warm of [false, true]) {
        const f = fixture(p); f.qualify();
        if (warm) await publicAPI.ensureBinary(p, options(f));
        mutate(f);
        // Recomputing the descriptor's manifest digest cannot repair a stale
        // independently bound proof or invalid producer metadata.
        f.descriptor.release_manifest_sha256 = c.digest(c.encode(f.manifest)); f.save();
        await assert.rejects(publicAPI.ensureBinary(p, options(f, { request: () => assert.fail("invalid metadata downloaded") })));
      }
    });
    test(`${p}: canonical JSON, package binding and unsafe metadata files`, async () => {
      for (const name of ["public-release.json", "release-manifest.json", "package.json"]) {
        for (const change of [b => Buffer.from(b.toString().replace("{", '{"duplicate":1,"duplicate":2,')),
          b => Buffer.concat([b, Buffer.from([0xff])]), b => Buffer.from(b.toString().trim())]) {
          const f = fixture(p); f.qualify(); const file = path.join(f.packageRoot, name);
          fs.writeFileSync(file, change(fs.readFileSync(file)));
          await assert.rejects(publicAPI.ensureBinary(p, options(f)));
        }
      }
      for (const field of ["name", "bin", "version", "engines", "scripts"]) {
        const f = fixture(p); f.qualify(); const file = path.join(f.packageRoot, "package.json");
        const pkg = JSON.parse(fs.readFileSync(file)); pkg[field] = "invalid"; fs.writeFileSync(file, c.encode(pkg));
        await assert.rejects(publicAPI.ensureBinary(p, options(f)));
      }
    });
    test(`${p}: strict cache aliases, parents, overlap and finite lock wait`, async () => {
      for (const alias of ["symlink", "hardlink", "directory"]) {
        const f = fixture(p); f.qualify(); const resolved = await publicAPI.ensureBinary(p, options(f));
        const backup = path.join(f.root, "unrelated"); fs.renameSync(resolved.binaryPath, backup);
        if (alias === "symlink") fs.symlinkSync(backup, resolved.binaryPath);
        if (alias === "hardlink") fs.linkSync(backup, resolved.binaryPath);
        if (alias === "directory") fs.mkdirSync(resolved.binaryPath);
        await assert.rejects(publicAPI.ensureBinary(p, options(f)), /unsafe/);
        assert.equal(c.digest(fs.readFileSync(backup)), f.manifest.assets[target].binary.sha256);
      }
      const f = fixture(p); f.qualify();
      await assert.rejects(publicAPI.ensureBinary(p, options(f, { cacheRoot: f.packageRoot })), /overlaps/);
      const alias = path.join(f.root, "alias"); fs.symlinkSync(f.cacheRoot, alias);
      await assert.rejects(publicAPI.ensureBinary(p, options(f, { cacheRoot: alias })), /safe ancestors/);
      const cold = await publicAPI.ensureBinary(p, options(f));
      const lockRoot = path.join(f.cacheRoot, "public-authoring-v1", ".locks");
      const unlock = await v.acquireLock(cold.binaryPath, { lockRoot });
      try { await assert.rejects(publicAPI.ensureBinary(p, options(f, { lockOptions: { timeoutMs: 2, pollMs: 1 } })), /timed out/); }
      finally { await unlock(); }
      fs.chmodSync(path.dirname(cold.binaryPath), 0o755);
      await assert.rejects(publicAPI.ensureBinary(p, options(f)), /mode-0700/);
    });
    test(`${p}: concurrent cold callers serialize one download and cancellation is effect-free`, async () => {
      const f = fixture(p); f.qualify(); const seen = [];
      const results = await Promise.all([1, 2, 3].map(() => publicAPI.ensureBinary(p, options(f, { request: transport(f, seen) }))));
      assert.equal(seen.length, 1); assert.equal(results.filter(r => !r.cacheHit).length, 1);
      const controller = new AbortController(); controller.abort();
      await assert.rejects(publicAPI.ensureBinary(p, options(f, { signal: controller.signal })), /cancelled/);
      await assert.rejects(publicAPI.ensureBinary(p, options(f, { arch: "mips" })), /arch/);
      await assert.rejects(publicAPI.ensureBinary(p, options(f, { target: "unrecognized" })), /target/);
    });
    test(`${p}: real downloader faults never publish or leave operation files`, async () => {
      for (const kind of ["digest", "overflow", "truncated", "redirect", "timeout"]) {
        const f = fixture(p); f.qualify();
        const request = transport(f, [], (res, req, body) => {
          req.done = true;
          if (kind === "timeout") return req.timeout();
          if (kind === "redirect") { res.statusCode = 302; res.headers.location = "https://unapproved.invalid/binary"; }
          req.emit("response", res);
          res.end(kind === "digest" ? Buffer.alloc(body.length) : kind === "overflow" ? Buffer.concat([body, body]) : body.subarray(1));
        });
        await assert.rejects(publicAPI.ensureBinary(p, options(f, { request })));
        const namespace = path.join(f.cacheRoot, "public-authoring-v1");
        assert.equal(fs.readdirSync(namespace).some(n => n.startsWith(".download-")), false);
        assert.deepEqual(fs.readdirSync(path.join(namespace, ".locks")), []);
      }
    });
  }
  test("kit outer-valid inner-invalid archive fails without cache publication", async () => {
    for (const unsafe of [false, true]) {
      const f = fixture("plugin-kit-ai");
      const a = f.manifest.assets[target];
      const body = unsafe ? Buffer.from("not gzip") : c.archive(Buffer.from("wrong inner bytes"), a.binary.file);
      Object.assign(a, c.metadata(body)); f.bodies[a.file] = body;
      f.descriptor.release_manifest_sha256 = c.digest(c.encode(f.manifest)); f.qualify();
      await assert.rejects(publicAPI.ensureBinary("plugin-kit-ai", options(f)));
    }
  });
  test("cleanup and rename failure surface uncertainty; previous bytes survive", async () => {
    const f = fixture("agentplugins"); f.qualify();
    const cold = await publicAPI.ensureBinary("agentplugins", options(f)); fs.writeFileSync(cold.binaryPath, "old");
    const io = { ...fsp, rename: async (a, b) => {
      if (path.basename(a).startsWith(".agentplugins-staging-")) throw new Error("injected publication failure");
      return fsp.rename(a, b);
    } };
    await assert.rejects(publicAPI.ensureBinary("agentplugins", options(f, { io })), /publication failure/);
    assert.equal(fs.readFileSync(cold.binaryPath, "utf8"), "old");
    const cleanup = { ...fsp, unlink: async file => {
      if (path.basename(file) === "asset") throw new Error("injected cleanup failure");
      return fsp.unlink(file);
    } };
    await assert.rejects(publicAPI.ensureBinary("agentplugins", options(f, { io: cleanup })), /cleanup\/rollback uncertainty/);
  });
  test("new child environment strips private/test controls and retains ordinary user argv environment", () => {
    const env = publicAPI.childEnvironment({ NODE_OPTIONS: "preload", AGENTPLUGINS_INTERNAL_PROOF_BINARY: "private",
      UAP_PUBLIC_AUTHORING_NATIVE_CONFIG: "fixture", PLUGIN_KIT_AI_VERSION: "v9", KEEP: "yes" });
    assert.deepEqual(env, { KEEP: "yes" });
  });
}
module.exports = { fixture, transport, options, target, REPO };

if (require.main === module) {
  test("public acquisition detects parent replacement during download and preserves unrelated data", async () => {
    const f = fixture("agentplugins"); f.qualify();
    const release = publicAPI.loadRelease("agentplugins", f.packageRoot, target);
    const parent = path.dirname(publicAPI.cachePath(f.cacheRoot, "agentplugins", target, release));
    const request = transport(f, [], () => {
      fs.renameSync(parent, parent + "-retained"); fs.mkdirSync(parent, { mode: 0o700 });
      fs.writeFileSync(path.join(parent, "unrelated"), "preserved");
    });
    await assert.rejects(publicAPI.ensureBinary("agentplugins", options(f, { request })), /directory replaced/);
    assert.equal(fs.readFileSync(path.join(parent, "unrelated"), "utf8"), "preserved");
  });
  test("canonical kit archive rejects traversal, links, extra members and oversized expansion", async () => {
    const zlib = require("node:zlib");
    for (const kind of ["traversal", "symlink", "hardlink", "extra", "padding", "bomb"]) {
      const f = fixture("plugin-kit-ai"); const a = f.manifest.assets[target];
      let tar = zlib.gunzipSync(f.bodies[a.file]);
      if (kind === "traversal") tar.write("../plugin-kit-ai\0", 0);
      if (kind === "symlink") tar[156] = 50;
      if (kind === "hardlink") tar[156] = 49;
      if (kind === "extra") tar = Buffer.concat([tar, tar]);
      if (kind === "padding") tar[tar.length - 1] = 1;
      if (kind === "bomb") tar.write("77777777777\0", 124);
      const body = zlib.gzipSync(tar); Object.assign(a, c.metadata(body)); f.bodies[a.file] = body;
      f.descriptor.release_manifest_sha256 = c.digest(c.encode(f.manifest)); f.qualify();
      await assert.rejects(publicAPI.ensureBinary("plugin-kit-ai", options(f)));
    }
  });
}
