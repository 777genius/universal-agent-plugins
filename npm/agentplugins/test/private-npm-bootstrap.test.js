"use strict";

// POSIX STRUCTURAL evidence only: payloads are never executed. Retain private
// fixture roots for audit; these tests do not need recursive cleanup.
const assert = require("node:assert/strict");
const fs = require("node:fs");
const fsp = require("node:fs/promises");
const os = require("node:os");
const path = require("node:path");
const { spawn, spawnSync } = require("node:child_process");
const nodeTest = require("node:test");
const test = (name, fn) => nodeTest(name, { skip: process.platform !== "linux" ? "Linux structural filesystem fixture; native lanes unproven" : false }, fn);
const zlib = require("node:zlib");
const c = require("../scripts/dual-authoring-candidate");
const p = require("../scripts/private-npm/bootstrap");
const v = require("../lib/verifier");
const TARGET = "linux-amd64";
const ID = { repository: c.REPOSITORY, commit: "a".repeat(40), engine_revision: "a".repeat(40),
  versions: { agentplugins: "0.1.23", "plugin-kit-ai": "2.0.0" } };
const mkdir = name => fs.mkdirSync(name, { mode: 0o700 });
const write = (file, body) => { if (fs.existsSync(file)) fs.chmodSync(file, 0o600); fs.writeFileSync(file, body); };

function fixture(scope = "linux-amd64-pair", identity = ID) {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "private-npm-fixture-"));
  const ancestor = path.join(root, "inputs with spaces"); mkdir(ancestor);
  const source = path.join(ancestor, "candidate"); mkdir(source);
  const cache = path.join(root, "cache with spaces"); mkdir(cache);
  const manifest = { schema: c.SCHEMA, status: "CANDIDATE", identity: structuredClone(identity), asset_scope: scope,
    build: { method: "controlled-git-archive-go-build/v1", go_version: "go1.25.13", go_sha256: "b".repeat(64),
      source_archive_sha256: "c".repeat(64), authoring_mode: "vertical-slice-v1" }, products: {}, release_eligible: false };
  const packages = {}, descriptors = {}, bodies = {};
  for (const product of c.PRODUCTS) {
    const packageRoot = path.join(root, product + " package"); mkdir(packageRoot);
    packages[product] = packageRoot;
    manifest.products[product] = { version: identity.versions[product], assets: {} };
    for (const target of c.scopeTargets(scope)) {
      const binary = Buffer.from(`STRUCTURAL ${product} ${target} ${identity.versions[product]}\n`);
      const asset = product === "plugin-kit-ai" ? c.archive(binary, c.executableName(product, target)) : binary;
      const file = c.assetName(product, identity.versions[product], target);
      bodies[product + target] = binary;
      write(path.join(source, file), asset);
      manifest.products[product].assets[target] = { file, ...c.metadata(asset), binary: { file: c.executableName(product, target), ...c.metadata(binary) } };
    }
    descriptors[product] = { schema: p.SCHEMA, product, npm_package: product === "agentplugins" ? "universal-agent-plugins" : product,
      identity: structuredClone(identity), asset_scope: scope, authoring_mode: "vertical-slice-v1", candidate_sha256: "" };
    write(path.join(packageRoot, "package.json"), c.encode({ name: descriptors[product].npm_package,
      version: identity.versions[product], private: true, bin: { [product]: `bin/${product}.js` } }));
  }
  function save() {
    fs.chmodSync(source, 0o700);
    const bytes = c.encode(manifest);
    write(path.join(source, "candidate.json"), bytes);
    for (const product of c.PRODUCTS) {
      descriptors[product].candidate_sha256 = c.digest(bytes);
      write(path.join(packages[product], "candidate.json"), bytes);
      write(path.join(packages[product], "private-release.json"), c.encode(descriptors[product]));
    }
    for (const file of fs.readdirSync(source)) fs.chmodSync(path.join(source, file), 0o444);
    fs.chmodSync(source, 0o555);
  }
  save();
  const options = (product, extra = {}) => ({ packageRoot: packages[product], cacheRoot: cache, candidateRoot: source, target: TARGET, ...extra });
  const release = product => p.loadRelease(product, packages[product], TARGET);
  const binaryPath = product => p.cachePath(cache, product, TARGET, release(product));
  return { root, source, cache, manifest, packages, descriptors, bodies, save, options, release, binaryPath };
}

function snapshot(root) {
  return fs.readdirSync(root).sort().map(name => {
    const stat = fs.lstatSync(path.join(root, name));
    return [name, stat.mode, stat.ino, stat.nlink, c.digest(fs.readFileSync(path.join(root, name)))];
  });
}
const locks = f => fs.existsSync(path.join(f.cache, ".locks")) ? fs.readdirSync(path.join(f.cache, ".locks")) : [];
const run = (f, product = "agentplugins", extra = {}, hooks = {}) => p.ensureBinary(product, f.options(product, extra), hooks);
function changeJSON(file, change) { const value = JSON.parse(fs.readFileSync(file)); change(value); write(file, c.encode(value)); }

for (const product of c.PRODUCTS) test(`STRUCTURAL ${product}: cold, source-free warm, and ordinary corruption repair`, async () => {
  const f = fixture(); const before = snapshot(f.source); const rootMode = fs.statSync(f.source).mode;
  let acquisitions = 0; const hooks = { afterFreeze() { acquisitions++; } };
  const first = await run(f, product, {}, hooks);
  assert.equal(first.cacheHit, false);
  assert.deepEqual(fs.readFileSync(first.binaryPath), f.bodies[product + TARGET]);
  assert.equal(fs.statSync(first.binaryPath).mode & 0o777, 0o755);
  for (const corrupt of [file => write(file, Buffer.alloc(f.bodies[product + TARGET].length)),
    file => write(file, "truncated"), file => fs.chmodSync(file, 0o644)]) {
    corrupt(first.binaryPath);
    const repaired = await run(f, product, {}, hooks);
    assert.equal(repaired.cacheHit, false);
    assert.deepEqual(fs.readFileSync(first.binaryPath), f.bodies[product + TARGET]);
  }
  fs.renameSync(f.source, f.source + "-unavailable");
  assert.equal((await run(f, product, {}, hooks)).cacheHit, true);
  assert.equal((await run(f, product, { candidateRoot: undefined }, hooks)).cacheHit, true);
  assert.equal(acquisitions, 4);
  assert.deepEqual(snapshot(f.source + "-unavailable"), before);
  assert.equal(fs.statSync(f.source + "-unavailable").mode, rootMode);
  assert.deepEqual(locks(f), []);
});

test("STRUCTURAL explicit roots, complete identity and product isolation; legacy bytes never selected", async () => {
  const f = fixture();
  const stale = path.join(f.root, "vendor"); mkdir(stale); write(path.join(stale, "agentplugins"), "stale engine");
  mkdir(path.join(f.cache, ID.versions.agentplugins));
  write(path.join(f.cache, ID.versions.agentplugins, "agentplugins"), "wrong engine");
  const [a, b] = await Promise.all(c.PRODUCTS.map(product => run(f, product)));
  assert.notEqual(a.binaryPath, b.binaryPath);
  assert.match(a.binaryPath, new RegExp(f.descriptors.agentplugins.candidate_sha256));
  assert.match(a.binaryPath, new RegExp(f.manifest.products.agentplugins.assets[TARGET].binary.sha256));
  const newer = fixture("linux-amd64-pair", { ...ID, versions: { agentplugins: "0.1.24", "plugin-kit-ai": "2.0.1" } });
  const unlockVersion = await v.acquireLock(a.binaryPath, { lockRoot: path.join(f.cache, ".locks") });
  try {
    const next = await run(newer, "agentplugins", { cacheRoot: f.cache, lockOptions: { timeoutMs: 20 } });
    assert.notEqual(a.binaryPath, next.binaryPath);
  } finally { await unlockVersion(); }
  f.manifest.build.source_archive_sha256 = "d".repeat(64); f.save();
  const changedIdentity = await run(f);
  assert.notEqual(a.binaryPath, changedIdentity.binaryPath);
  assert.equal(fs.readFileSync(path.join(stale, "agentplugins"), "utf8"), "stale engine");
  assert.equal(fs.readFileSync(path.join(f.cache, ID.versions.agentplugins, "agentplugins"), "utf8"), "wrong engine");
  for (const value of [undefined, "relative", f.packages.agentplugins, f.cache + "/../cache with spaces"]) {
    await assert.rejects(run(f, "agentplugins", { cacheRoot: value }));
  }
});

const identityMutations = [
  ["repository alias", id => { id.repository = "777genius/plugin-kit-ai"; }],
  ["short commit", id => { id.commit = "a"; }], ["uppercase commit", id => { id.commit = "A".repeat(40); }],
  ["wrong commit", id => { id.commit = id.engine_revision = "d".repeat(40); }],
  ["unequal engine", id => { id.engine_revision = "d".repeat(40); }],
  ["same versions", id => { id.versions.agentplugins = id.versions["plugin-kit-ai"]; }],
  ["other version", id => { id.versions["plugin-kit-ai"] = "2.0.1"; }],
  ["numeric version", id => { id.versions.agentplugins = 1; }],
  ["latest", id => { id.versions.agentplugins = "latest"; }],
  ["pre-release", id => { id.versions.agentplugins = "0.1.23-rc.1"; }],
  ["extra identity", id => { id.extra = true; }], ["missing version", id => { delete id.versions.agentplugins; }]
];
for (const [name, mutate] of identityMutations) test(`private expected identity rejects ${name} even warm`, async () => {
  const f = fixture(); const cached = await run(f);
  changeJSON(path.join(f.packages.agentplugins, "private-release.json"), d => mutate(d.identity));
  await assert.rejects(run(f));
  assert.deepEqual(fs.readFileSync(cached.binaryPath), f.bodies["agentplugins" + TARGET]);
});

const descriptorMutations = [
  ["schema", d => { d.schema = c.SCHEMA; }], ["product", d => { d.product = "plugin-kit-ai"; }],
  ["npm name", d => { d.npm_package = "agentplugins"; }], ["missing mode", d => { delete d.authoring_mode; }],
  ["release mode before B", d => { d.authoring_mode = "release-cli-contract-v1"; }],
  ["wrong scope", d => { d.asset_scope = "six-platform-pair"; }], ["unknown scope", d => { d.asset_scope = "all"; }],
  ["digest", d => { d.candidate_sha256 = "e".repeat(64); }], ["digest type", d => { d.candidate_sha256 = ["e".repeat(64)]; }],
  ["extra field", d => { d.environment = {}; }], ["missing identity", d => { delete d.identity; }]
];
for (const [name, mutate] of descriptorMutations) test(`closed descriptor rejects ${name} before cache effects`, async () => {
  const f = fixture();
  changeJSON(path.join(f.packages.agentplugins, "private-release.json"), mutate);
  await assert.rejects(run(f)); assert.deepEqual(fs.readdirSync(f.cache), []);
});
for (const [name, mutate] of [
  ["name", x => { x.name = "agentplugins"; }], ["version", x => { x.version = "0.1.24"; }],
  ["bin", x => { x.bin.agentplugins = "../plugin-kit-ai"; }], ["bin alias", x => { x.bin.other = x.bin.agentplugins; }],
  ["private flag", x => { x.private = false; }]
]) test(`package binding rejects ${name}`, async () => {
  const f = fixture(); changeJSON(path.join(f.packages.agentplugins, "package.json"), mutate);
  await assert.rejects(run(f)); assert.deepEqual(fs.readdirSync(f.cache), []);
});

const candidateMutations = [
  ["public schema", m => { m.schema = 2; }], ["legacy schema", m => { m.schema = 1; }],
  ["release eligible", m => { m.release_eligible = true; }], ["status", m => { m.status = "RELEASE"; }],
  ["extra", m => { m.attested = true; }], ["missing products", m => { delete m.products["plugin-kit-ai"]; }],
  ["extra asset", m => { m.products.agentplugins.assets["linux-386"] = m.products.agentplugins.assets[TARGET]; }],
  ["missing asset", m => { delete m.products.agentplugins.assets[TARGET]; }],
  ["extra product", m => { m.products.other = m.products.agentplugins; }],
  ["unsafe name", m => { m.products.agentplugins.assets[TARGET].file = "../agentplugins"; }],
  ["wrong binary name", m => { m.products["plugin-kit-ai"].assets[TARGET].binary.file = "agentplugins"; }],
  ["archive confusion", m => { m.products["plugin-kit-ai"].assets[TARGET].file = "plugin-kit-ai"; }],
  ["raw pins disagree", m => { m.products.agentplugins.assets[TARGET].binary.sha256 = "e".repeat(64); }],
  ["duplicate executable", m => { Object.assign(m.products["plugin-kit-ai"].assets[TARGET].binary, c.metadata(Buffer.from("STRUCTURAL agentplugins linux-amd64 0.1.23\n"))); }],
  ["missing mode", m => { delete m.build.authoring_mode; }], ["wrong mode", m => { m.build.authoring_mode = "release-cli-contract-v1"; }],
  ["wrong compiler", m => { m.build.go_version = "go1.0"; }], ["wrong build", m => { m.build.method = "untrusted"; }],
  ["extra binary field", m => { m.products.agentplugins.assets[TARGET].binary.extra = 1; }]
];
for (const [name, mutate] of candidateMutations) test(`repinned candidate rejects ${name}`, async () => {
  const f = fixture(); mutate(f.manifest); f.save();
  await assert.rejects(run(f)); assert.deepEqual(fs.readdirSync(f.cache), []);
});

test("every asset and inner pin is bounded and typed, including unselected assets", async () => {
  for (const inner of [false, true]) for (const field of ["size", "sha256"]) {
    for (const value of field === "size" ? [0, -1, 1.5, "5", null, 128 * 1024 * 1024 + 1, Number.MAX_SAFE_INTEGER + 1] :
      ["A".repeat(64), "short", ["a".repeat(64)], null, 12]) {
      const f = fixture(); const asset = f.manifest.products["plugin-kit-ai"].assets[TARGET];
      (inner ? asset.binary : asset)[field] = value; f.save();
      await assert.rejects(run(f), /pin/); assert.deepEqual(fs.readdirSync(f.cache), []);
    }
  }
});

test("canonical JSON and bounded regular metadata at each trusted level", async () => {
  for (const file of ["private-release.json", "candidate.json", "package.json"]) {
    for (const mutate of [body => Buffer.from(body.toString().replace('{', '{"duplicate":1,"duplicate":2,')),
      body => Buffer.concat([body, Buffer.from(" ")]), () => Buffer.alloc(1024 * 1024 + 1, 32),
      () => Buffer.from([0xff, 0xfe]), () => Buffer.alloc(0)]) {
      const f = fixture(); const filename = path.join(f.packages.agentplugins, file);
      write(filename, mutate(fs.readFileSync(filename)));
      await assert.rejects(run(f)); assert.deepEqual(fs.readdirSync(f.cache), []);
    }
  }
});

test("closed scopes and all twelve target entries validate structurally without claiming native support", async () => {
  const f = fixture("six-platform-pair");
  for (const product of c.PRODUCTS) for (const target of c.TARGETS) {
    const result = await run(f, product, { target });
    assert.deepEqual(fs.readFileSync(result.binaryPath), f.bodies[product + target]);
  }
  for (const target of [undefined, "linux-x64", "linux-386", "../linux-amd64", [TARGET]]) await assert.rejects(run(f, "agentplugins", { target }));
  for (const product of ["universal-agent-plugins", "other", "__proto__"]) await assert.rejects(run(f, product));
});

for (const product of c.PRODUCTS) test(`${product}: immutable source faults preserve inputs and unowned entries`, async () => {
  for (const kind of ["bytes", "truncated", "manifest", "extra", "missing", "writable", "root writable", "symlink", "hardlink", "directory", "fifo"]) {
    const f = fixture(); const file = path.join(f.source, f.manifest.products[product].assets[TARGET].file);
    fs.chmodSync(f.source, 0o700);
    if (kind === "bytes") write(file, Buffer.alloc(fs.statSync(file).size));
    if (kind === "truncated") write(file, "x");
    if (kind === "manifest") write(path.join(f.source, "candidate.json"), "{}");
    if (kind === "extra") write(path.join(f.source, "sentinel"), "preserve");
    if (["missing", "symlink", "hardlink", "directory", "fifo"].includes(kind)) fs.unlinkSync(file);
    const sentinel = path.join(f.root, "sentinel"); write(sentinel, "preserve");
    if (kind === "symlink") fs.symlinkSync(sentinel, file);
    if (kind === "hardlink") fs.linkSync(sentinel, file);
    if (kind === "directory") mkdir(file);
    if (kind === "fifo") assert.equal(spawnSync("mkfifo", [file]).status, 0);
    for (const name of fs.readdirSync(f.source)) if (!fs.lstatSync(path.join(f.source, name)).isSymbolicLink()) fs.chmodSync(path.join(f.source, name), 0o444);
    if (kind === "writable") fs.chmodSync(file, 0o644);
    if (kind !== "root writable") fs.chmodSync(f.source, 0o555);
    const before = fs.readdirSync(f.source);
    await assert.rejects(run(f, product));
    assert.equal(fs.existsSync(f.binaryPath(product)), false);
    assert.equal(fs.readFileSync(sentinel, "utf8"), "preserve");
    assert.deepEqual(fs.readdirSync(f.source), before); assert.deepEqual(locks(f), []);
  }
});

test("strict archive rejects compressed and inner pins, corruption, alternate records and bounded bombs", async () => {
  const edits = [
    ["outer pin", (body) => Buffer.alloc(body.length)],
    ["gzip corruption", body => Buffer.from(body).fill(0, 0, 10)],
    ["raw as archive", () => Buffer.from("raw binary bytes")],
    ["inner bytes", () => c.archive(Buffer.from("other binary"), "plugin-kit-ai")],
    ["wrong root", () => c.archive(Buffer.from("other binary"), "agentplugins")],
    ...[0, 100, 156, 257].map(offset => [`alternate header ${offset}`, body => {
      const tar = zlib.gunzipSync(body); tar[offset] ^= 1; return zlib.gzipSync(tar);
    }]),
    ["extra record", body => zlib.gzipSync(Buffer.concat([zlib.gunzipSync(body), Buffer.alloc(512)]))],
    ["pax record", body => { const tar = zlib.gunzipSync(body); tar[156] = 120; return zlib.gzipSync(tar); }],
    ["hardlink record", body => { const tar = zlib.gunzipSync(body); tar[156] = 49; return zlib.gzipSync(tar); }],
    ["symlink record", body => { const tar = zlib.gunzipSync(body); tar[156] = 50; return zlib.gzipSync(tar); }],
    ["gzip bomb", () => zlib.gzipSync(Buffer.alloc(128 * 1024 * 1024 + 2049))]
  ];
  for (const [label, edit] of edits) {
    const f = fixture(); const asset = f.manifest.products["plugin-kit-ai"].assets[TARGET];
    const file = path.join(f.source, asset.file); const bytes = edit(fs.readFileSync(file));
    write(file, bytes);
    if (label !== "outer pin") Object.assign(asset, c.metadata(bytes));
    f.save(); await assert.rejects(run(f, "plugin-kit-ai"), undefined, label);
    assert.equal(fs.existsSync(f.binaryPath("plugin-kit-ai")), false); assert.deepEqual(locks(f), []);
  }
});

test("selected source is read once then materialized only from the frozen bytes", async () => {
  const f = fixture(); const file = path.join(f.source, f.manifest.products.agentplugins.assets[TARGET].file);
  const originalRead = fs.readFileSync; let reads = 0;
  fs.readFileSync = function (name, ...args) {
    // candidate reader uses a descriptor; identify its current inode.
    if (typeof name === "number" && fs.fstatSync(name).ino === fs.statSync(file).ino) reads++;
    return originalRead.call(this, name, ...args);
  };
  try {
    const result = await run(f, "agentplugins", {}, { afterFreeze() { write(file, "changed after freeze"); } });
    assert.equal(reads, 1); assert.deepEqual(originalRead(result.binaryPath), f.bodies["agentplugins" + TARGET]);
  } finally { fs.readFileSync = originalRead; }
});

test("source changing during the existing descriptor freeze is rejected", async () => {
  const f = fixture(); const file = path.join(f.source, f.manifest.products.agentplugins.assets[TARGET].file);
  const ino = fs.statSync(file).ino; const original = fs.readFileSync;
  fs.readFileSync = function (name, ...args) {
    const bytes = original.call(this, name, ...args);
    if (typeof name === "number" && fs.fstatSync(name).ino === ino) write(file, "changed during freeze");
    return bytes;
  };
  try { await assert.rejects(run(f), /changed while freezing/); }
  finally { fs.readFileSync = original; }
  assert.equal(fs.existsSync(f.binaryPath("agentplugins")), false); assert.deepEqual(locks(f), []);
});

function child(f, product, extra = {}) {
  const marker = path.join(f.root, "acquisitions");
  const code = `const fs=require('node:fs');const p=require(${JSON.stringify(require.resolve("../scripts/private-npm/bootstrap"))});
    p.ensureBinary(${JSON.stringify(product)},${JSON.stringify(f.options(product, extra))}, {
      async afterFreeze(){fs.appendFileSync(${JSON.stringify(marker)},'acquired\\n'); await new Promise(r=>setTimeout(r,100));}
    }).then(x=>process.stdout.write(JSON.stringify(x))).catch(e=>{process.stderr.write(e.message);process.exitCode=1;});`;
  const proc = spawn(process.execPath, ["-e", code], { env: { HOME: f.root, TMPDIR: f.root, PATH: path.dirname(process.execPath) }, stdio: ["ignore", "pipe", "pipe"] });
  return new Promise((resolve, reject) => {
    let out = "", err = "";
    const timeout = setTimeout(() => { proc.kill("SIGKILL"); reject(new Error("fixture process deadline")); }, 10_000);
    proc.stdout.on("data", b => { out += b; }); proc.stderr.on("data", b => { err += b; });
    proc.on("error", reject);
    proc.on("close", status => { clearTimeout(timeout); status === 0 ? resolve(JSON.parse(out)) : reject(new Error(err)); });
  });
}
for (const product of c.PRODUCTS) test(`two independent ${product} consumers acquire once for both cold and corrupt entries`, async () => {
  const f = fixture();
  for (const corrupt of [false, true]) {
    if (corrupt) write(f.binaryPath(product), "corrupt");
    const results = await Promise.all([child(f, product), child(f, product)]);
    assert.equal(results[0].binaryPath, results[1].binaryPath);
    assert.equal(results.filter(r => !r.cacheHit).length, 1);
    assert.deepEqual(fs.readFileSync(results[0].binaryPath), f.bodies[product + TARGET]);
  }
  assert.equal(fs.readFileSync(path.join(f.root, "acquisitions"), "utf8"), "acquired\nacquired\n");
  assert.deepEqual(locks(f), []);
});

test("warm readers lock too; cancelled waiters and stale-looking locks preserve owner; products do not block", async () => {
  const f = fixture(); const cached = await run(f);
  const unlock = await v.acquireLock(cached.binaryPath, { lockRoot: path.join(f.cache, ".locks") });
  const lock = path.join(f.cache, ".locks", locks(f)[0]); const body = fs.readFileSync(lock);
  fs.utimesSync(lock, new Date(0), new Date(0));
  await assert.rejects(run(f, "agentplugins", { lockOptions: { timeoutMs: 20, pollMs: 2 } }), /timed out/);
  const controller = new AbortController();
  const waiting = run(f, "agentplugins", { signal: controller.signal });
  setTimeout(() => controller.abort(), 20);
  await assert.rejects(waiting, /cancelled/); assert.deepEqual(fs.readFileSync(lock), body);
  assert.equal((await run(f, "plugin-kit-ai", { lockOptions: { timeoutMs: 20 } })).cacheHit, false);
  await unlock(); await unlock(); assert.deepEqual(locks(f), []);
  const before = fs.readFileSync(cached.binaryPath);
  const owner = new AbortController(); write(cached.binaryPath, "bad");
  await assert.rejects(run(f, "agentplugins", { signal: owner.signal }, { afterFreeze() { owner.abort(); } }), /cancelled/);
  assert.equal(fs.readFileSync(cached.binaryPath, "utf8"), "bad");
  assert.deepEqual(locks(f), []); assert.equal(before.length, f.bodies["agentplugins" + TARGET].length);
});

test("unsafe cache files and directory ancestors are preserved without repair", async () => {
  for (const kind of ["symlink", "hardlink", "directory", "fifo", "writable", "setuid"]) {
    const f = fixture(); const { binaryPath } = await run(f); fs.unlinkSync(binaryPath);
    const sentinel = path.join(f.root, "sentinel"); write(sentinel, "preserve");
    if (kind === "symlink") fs.symlinkSync(sentinel, binaryPath);
    if (kind === "hardlink") fs.linkSync(sentinel, binaryPath);
    if (kind === "directory") mkdir(binaryPath);
    if (kind === "fifo") assert.equal(spawnSync("mkfifo", [binaryPath]).status, 0);
    if (["writable", "setuid"].includes(kind)) { write(binaryPath, "unsafe"); fs.chmodSync(binaryPath, kind === "writable" ? 0o777 : 0o4755); }
    const before = fs.lstatSync(binaryPath);
    await assert.rejects(run(f), /unsafe/);
    assert.equal(fs.lstatSync(binaryPath).ino, before.ino); assert.equal(fs.readFileSync(sentinel, "utf8"), "preserve");
    assert.deepEqual(locks(f), []);
  }
  for (const location of ["root", "descendant", "ancestor", "symlink ancestor"]) {
    const f = fixture(); await run(f);
    if (location === "root") fs.chmodSync(f.cache, 0o755);
    if (location === "descendant") fs.chmodSync(path.dirname(f.binaryPath("agentplugins")), 0o755);
    if (location === "ancestor") fs.chmodSync(f.root, 0o777);
    if (location === "symlink ancestor") {
      fs.renameSync(f.cache, f.cache + "-real"); fs.symlinkSync(f.cache + "-real", f.cache);
    }
    await assert.rejects(run(f));
  }
});

test("input/cache overlap, source basename and linked package ancestors are rejected", async () => {
  const f = fixture();
  for (const root of [f.cache, f.root, path.join(f.cache, "source"), f.packages.agentplugins,
    path.join(f.root, "candidate with spaces"), "https://github.com/release", f.source + "/../candidate"]) {
    await assert.rejects(run(f, "agentplugins", { candidateRoot: root }));
  }
  fs.renameSync(f.packages.agentplugins, f.packages.agentplugins + "-real");
  fs.symlinkSync(f.packages.agentplugins + "-real", f.packages.agentplugins);
  await assert.rejects(run(f), /symlink/);
});

// Narrow filesystem fault proxy: real bytes/directories except at the specified
// operation. Tests keep named sentinels and inspect every remaining own entry.
function faults(intercept) {
  return new Proxy(fsp, { get(target, key) {
    if (typeof target[key] !== "function") return target[key];
    return async (...args) => intercept(key, args, () => target[key](...args));
  } });
}
const fault = code => Object.assign(new Error(`injected ${code}`), { code });
function handleProxy(handle, intercept) {
  return new Proxy(handle, { get(target, key) {
    const value = Reflect.get(target, key, target);
    return typeof value === "function" ? (...args) => intercept(key, args, () => value.apply(target, args)) : value;
  } });
}

for (const operation of ["write", "close", "chmod", "quarantine", "publish", "verify", "unlink quarantine", "rollback", "unlink staging", "unlock"])
  test(`owned transaction fault: ${operation} is terminal and preserves bounded recovery evidence`, async () => {
    const f = fixture(); const { binaryPath } = await run(f); write(binaryPath, "prior corrupt bytes");
    const sentinel = path.join(path.dirname(binaryPath), "unrelated"); write(sentinel, "preserve");
    let injected = false;
    const io = faults(async (key, args, real) => {
      const file = String(args[0]); const destination = String(args[1]);
      if (key === "open" && file.includes(".agentplugins-staging-")) {
        const handle = await real();
        return handleProxy(handle, async (method, values, call) => {
          if (method === "writeFile" && ["write", "unlink staging"].includes(operation)) {
            await handle.writeFile(Buffer.from("partial")); injected = true; throw fault("ENOSPC");
          }
          if (method === "close" && operation === "close") { await call(); injected = true; throw fault("EIO close"); }
          return call();
        });
      }
      if (key === "chmod" && operation === "chmod") { injected = true; throw fault("EPERM chmod"); }
      if (key === "rename" && destination.includes(".agentplugins-replaced-") && operation === "quarantine") { injected = true; throw fault("EACCES quarantine"); }
      if (key === "rename" && file.includes(".agentplugins-staging-") && ["publish", "rollback"].includes(operation)) { injected = true; throw fault("EIO publish"); }
      if (key === "rename" && file.includes(".agentplugins-replaced-") && operation === "rollback") throw fault("EIO rollback");
      if (key === "rename" && file.includes(".agentplugins-staging-") && operation === "verify") {
        await real(); write(binaryPath, "committed corruption"); injected = true; return;
      }
      if (key === "unlink" && ((operation === "unlink quarantine" && file.includes(".agentplugins-replaced-")) ||
          (operation === "unlink staging" && file.includes(".agentplugins-staging-")) || (operation === "unlock" && file.endsWith(".lock")))) {
        injected = true; throw fault("EACCES cleanup");
      }
      return real();
    });
    await assert.rejects(run(f, "agentplugins", {}, { io }), operation.startsWith("unlink") || ["rollback", "unlock"].includes(operation) ? /uncertainty/ : /injected|verification/);
    assert.equal(injected, true); assert.equal(fs.readFileSync(sentinel, "utf8"), "preserve");
    const entries = fs.readdirSync(path.dirname(binaryPath));
    if (["unlink quarantine", "unlock"].includes(operation)) assert.deepEqual(fs.readFileSync(binaryPath), f.bodies["agentplugins" + TARGET]);
    else if (operation === "rollback") assert.equal(fs.existsSync(binaryPath), false);
    else assert.equal(fs.readFileSync(binaryPath, "utf8"), "prior corrupt bytes");
    assert.equal(entries.some(x => x.startsWith(".agentplugins-staging-")), operation === "unlink staging");
    assert.equal(entries.some(x => x.startsWith(".agentplugins-replaced-")), ["rollback", "unlink quarantine"].includes(operation));
    assert.equal(locks(f).length, operation === "unlock" ? 1 : 0);
  });

test("copy failure, permission denial and ownership mismatch preserve the exact entry", async () => {
  const f = fixture(); const { binaryPath } = await run(f); write(binaryPath, "prior");
  const io = faults((key, args, real) => { if (key === "copyFile") throw fault("ENOSPC copy"); return real(); });
  const file = path.join(f.root, "downloaded"); write(file, f.bodies["agentplugins" + TARGET]);
  await assert.rejects(v.installVerifiedBinary(file, binaryPath, f.release("agentplugins").asset.binary, {
    lockRoot: path.join(f.cache, ".locks"), osName: "linux", strict: true, io
  }), /ENOSPC/);
  assert.equal(fs.readFileSync(binaryPath, "utf8"), "prior");
  assert.deepEqual(fs.readdirSync(path.dirname(binaryPath)), ["agentplugins"]);
  for (const location of [f.cache, binaryPath]) {
    const wrongOwner = faults(async (key, args, real) => {
      const value = await real(); if (key === "lstat" && args[0] === location) value.uid = 98765; return value;
    });
    await assert.rejects(run(f, "agentplugins", {}, { io: wrongOwner }), /owned|unsafe/);
  }
  const denied = faults((key, args, real) => { if (key === "open" && String(args[0]).endsWith(".lock")) throw fault("EACCES"); return real(); });
  await assert.rejects(run(f, "agentplugins", {}, { io: denied }), /EACCES/); assert.deepEqual(locks(f), []);
});

test("unlock only removes owned inode and failed lock write cleans only its own identity", async () => {
  const f = fixture(); await run(f); const lockRoot = path.join(f.cache, ".locks");
  const unlock = await v.acquireLock(f.binaryPath("agentplugins"), { lockRoot });
  const lock = path.join(lockRoot, locks(f)[0]); fs.renameSync(lock, path.join(f.root, "held-lock"));
  write(lock, "unowned sentinel");
  await assert.rejects(unlock(), /identity changed/); await unlock();
  assert.equal(fs.readFileSync(lock, "utf8"), "unowned sentinel"); fs.unlinkSync(lock);
  const io = faults(async (key, args, real) => {
    const value = await real();
    if (key === "open" && String(args[0]).endsWith(".lock")) return handleProxy(value, (method, values, call) => {
      if (method === "writeFile") throw fault("ENOSPC lock"); return call();
    });
    return value;
  });
  await assert.rejects(v.acquireLock(f.binaryPath("agentplugins"), { lockRoot, io }), /ENOSPC/);
  assert.deepEqual(locks(f), []);
  for (const timeoutMs of [Infinity, NaN, -1]) await assert.rejects(v.acquireLock("target", { lockRoot, timeoutMs }), /finite/);
});

test("cancellation during staging and after publication restores prior bytes under the same lock", async () => {
  for (const phase of ["stage", "publish"]) {
    const f = fixture(); const { binaryPath } = await run(f); write(binaryPath, "prior");
    const controller = new AbortController();
    const io = faults(async (key, args, real) => {
      const result = await real();
      if ((phase === "stage" && key === "chmod") ||
          (phase === "publish" && key === "rename" && String(args[0]).includes(".agentplugins-staging-"))) controller.abort();
      return result;
    });
    await assert.rejects(run(f, "agentplugins", { signal: controller.signal }, { io }), /cancelled/);
    assert.equal(fs.readFileSync(binaryPath, "utf8"), "prior");
    assert.deepEqual(fs.readdirSync(path.dirname(binaryPath)), ["agentplugins"]); assert.deepEqual(locks(f), []);
  }
});

test("quarantine collision and newly appeared target preserve unowned sentinels", async () => {
  for (const collision of ["quarantine", "target"]) {
    const f = fixture(); const { binaryPath } = await run(f); write(binaryPath, "prior");
    let sentinel;
    const io = faults(async (key, args, real) => {
      if (collision === "quarantine" && key === "lstat" && String(args[0]).includes(".agentplugins-replaced-")) {
        sentinel = args[0]; if (!fs.existsSync(sentinel)) write(sentinel, "unowned");
      }
      const value = await real();
      if (collision === "target" && key === "rename" && String(args[1]).includes(".agentplugins-replaced-")) {
        sentinel = binaryPath; write(sentinel, "unowned");
      }
      return value;
    });
    await assert.rejects(run(f, "agentplugins", {}, { io }), /collision|appeared/);
    assert.equal(fs.readFileSync(sentinel, "utf8"), "unowned"); assert.deepEqual(locks(f), []);
  }
});

test("private path never consults legacy options, tokens, environment cache or transports", async () => {
  const f = fixture();
  const environment = new Proxy({}, { get() { throw new Error("ambient environment read"); } });
  const result = await run(f, "plugin-kit-ai", { environment, request() { throw new Error("transport called"); },
    repository: "legacy", version: "latest", vendor: f.root });
  assert.deepEqual(fs.readFileSync(result.binaryPath), f.bodies["plugin-kit-ai" + TARGET]);
  await assert.rejects(run(f, "agentplugins", { candidateRoot: undefined }), /local candidate root/);
  assert.equal(fs.existsSync(f.binaryPath("agentplugins")), false);
});

test("staging replacement is preserved when identity-owned cleanup fails closed", async () => {
  const f = fixture(); let sentinel;
  const io = faults(async (key, args, real) => {
    const result = await real();
    if (key === "chmod" && String(args[0]).includes(".agentplugins-staging-")) {
      sentinel = args[0]; fs.renameSync(sentinel, path.join(f.root, "held-staging")); write(sentinel, "unowned");
    }
    return result;
  });
  await assert.rejects(run(f, "agentplugins", {}, { io }), /cleanup\/rollback uncertainty.*identity changed/);
  assert.equal(fs.readFileSync(sentinel, "utf8"), "unowned");
  assert.equal(fs.existsSync(f.binaryPath("agentplugins")), false); assert.deepEqual(locks(f), []);
});

test("metadata symlinks/hardlinks and source ancestor links fail without opening alternate bytes", async () => {
  for (const filename of ["candidate.json", "private-release.json", "package.json"]) for (const link of ["symbolic", "hard"]) {
    const f = fixture(); const file = path.join(f.packages.agentplugins, filename);
    const saved = path.join(f.root, "saved"); fs.renameSync(file, saved);
    if (link === "symbolic") fs.symlinkSync(saved, file);
    else { fs.linkSync(saved, file); }
    await assert.rejects(run(f)); assert.deepEqual(fs.readdirSync(f.cache), []);
  }
  const f = fixture(); const ancestor = path.dirname(f.source);
  fs.renameSync(ancestor, ancestor + "-real"); fs.symlinkSync(ancestor + "-real", ancestor);
  await assert.rejects(run(f), /symlink/); assert.deepEqual(locks(f), []);
});
