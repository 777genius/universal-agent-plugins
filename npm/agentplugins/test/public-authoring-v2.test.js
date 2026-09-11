"use strict";

// Source/unit/interface evidence only: harmless bytes, deterministic fs and
// external-engine outcomes. No archive, native process, transport or custody proof.
const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const crypto = require("node:crypto");
const c = require("../scripts/dual-authoring-candidate");
const codec = require("../lib/public-authoring-contract");
const snapshots = require("../lib/public-authoring-input");
const runtime = require("../lib/public-authoring");
const v = require("../lib/verifier");
const stage = require("../scripts/stage-authoring-npm");
const realpath = fs.realpathSync;
require("../lib/platform");
const ROOT = path.resolve(__dirname, "../../..");
const source = Object.fromEntries(stage.STAGE_ALLOWLIST.map(name => {
  const bytes = fs.readFileSync(path.join(ROOT, name));
  return [name, { bytes, mode: (fs.lstatSync(path.join(ROOT, name)).mode & 0o111) ? "100755" : "100644",
    sha256: c.digest(bytes), git_blob: crypto.createHash("sha1").update(`blob ${bytes.length}\0`).update(bytes).digest("hex") }];
}));
const clone = value => structuredClone(value);
const hash = n => String(n).padStart(64, "0");
const LOCATOR = "UAP_PUBLIC_AUTHORING_ASSET_FILE";
const native = (product, target) => Buffer.from(`harmless ${product} ${target}\n`);
function fixture() {
  const input = { schema: codec.INPUT_SCHEMA, identity: { repository: c.REPOSITORY, commit: "a".repeat(40),
    engine_revision: "a".repeat(40), versions: { agentplugins: "0.1.99", "plugin-kit-ai": "2.0.0" } },
  authoring_mode: codec.MODE, asset_scope: codec.SCOPE, candidate_sha256: hash(30), pair_marker_sha256: hash(31), products: {},
  preparation: { sha256: hash(32), artifact: { run_id: 101, run_attempt: 2, artifact_id: 301, artifact_sha256: hash(33) } },
  producer: { workflow: codec.WORKFLOW, source: "a".repeat(40), run_id: 201, run_attempt: 3 } };
  const manifests = {}, outers = {};
  for (const product of c.PRODUCTS) {
    const assets = {}, version = input.identity.versions[product];
    for (const target of c.TARGETS) {
      const binary = native(product, target), outer = product === "agentplugins" ? binary : Buffer.concat([Buffer.from("mock outer\n"), binary]);
      assets[target] = { file: c.assetName(product, version, target), ...c.metadata(outer),
        binary: { file: c.executableName(product, target), ...c.metadata(binary) } };
      outers[assets[target].file] = outer;
    }
    input.products[product] = { tag: (product === "agentplugins" ? "agentplugins-v" : "v") + version,
      manifest_sha256: hash(40), checksums_sha256: hash(41), assets };
    // Independent literal oracle: no call to the shared projection constructor.
    manifests[product] = Buffer.from(JSON.stringify({ schema_version: 3, status: "CANDIDATE", product,
      repository: "777genius/universal-agent-plugins", tag: input.products[product].tag, version,
      commit: "a".repeat(40), engine_revision: "a".repeat(40), versions: { agentplugins: "0.1.99", "plugin-kit-ai": "2.0.0" },
      candidate_sha256: hash(30), authoring_mode: "release-cli-contract-v1", asset_scope: "six-platform-pair", assets,
      release_eligible: false, platform_acceptance: false, attested: false }, null, 2) + "\n");
    const p = input.products[product]; p.manifest_sha256 = c.digest(manifests[product]);
    p.checksums_sha256 = c.digest(Buffer.from(c.TARGETS.map(t => `${assets[t].sha256}  ${assets[t].file}\n`).join("") + `${p.manifest_sha256}  release-manifest.json\n`));
  }
  const inputBytes = codec.encodeInputs(input), pair = stage.pairedPackageFiles(source, manifests, inputBytes);
  return { input, inputBytes, manifests, pair, outers };
}

function memory(t, files, root = "/unit-package") {
  const nodes = new Map(), handles = new Map(), calls = [], owner = process.geteuid();
  let serial = 1, fd = 200;
  function put(name, body, overrides = {}) {
    const dir = body === null;
    const node = { body, dev: 1, ino: serial++, mode: dir ? 0o40700 : 0o100644, uid: owner, gid: owner,
      nlink: 1, size: dir ? 0 : body.length, mtimeMs: 1, ctimeMs: 1, type: dir ? "directory" : "file", ...overrides };
    nodes.set(name, node); return node;
  }
  function parents(name) {
    let current = path.dirname(name);
    while (!nodes.has(current)) { put(current, null, { mode: 0o40755 }); if (current === "/") break; current = path.dirname(current); }
  }
  for (const [name, bytes] of Object.entries(files)) { const full = path.join(root, name); parents(full); put(full, Buffer.from(bytes)); }
  const fault = { action: () => {} };
  const stat = node => ({ ...node, isFile: () => node.type === "file", isDirectory: () => node.type === "directory", isSymbolicLink: () => node.type === "link" });
  function get(name) { const node = nodes.get(name); if (!node) throw Object.assign(new Error(`missing ${name}`), { code: "ENOENT" }); return node; }
  for (const [name, fn] of Object.entries({
    lstatSync: name => stat(get(name)), realpathSync: name => { if (name.startsWith(ROOT + "/npm/")) return realpath(name); get(name); return name; },
    openSync: (name, flags) => { assert.equal(flags, fs.constants.O_RDONLY | fs.constants.O_NOFOLLOW); const id = fd++; handles.set(id, get(name)); return id; },
    fstatSync: id => stat(handles.get(id)),
    readSync: (id, buffer, offset, length, position) => {
      assert.ok(length <= 65536); return handles.get(id).body.copy(buffer, offset, position, position + length);
    },
    closeSync: id => { assert.ok(handles.has(id), "descriptor closed exactly once"); handles.delete(id); }
  })) t.mock.method(fs, name, (...args) => { calls.push([name, ...args]); fault.action(name, args); return fn(...args); });
  return { nodes, handles, calls, fault, put, parents, root };
}

function engine(t, f, product, warm = false) {
  const calls = [], asset = f.input.products[product].assets["linux-amd64"];
  let valid = warm;
  const fault = { action: () => {} };
  const invoke = name => { calls.push(name); fault.action(name); };
  const stat = { dev: 2, ino: 2, uid: process.geteuid(), mode: 0o40700, nlink: 1,
    isDirectory: () => true, isSymbolicLink: () => false, isFile: () => true };
  const io = Object.fromEntries(["mkdir", "lstat", "mkdtemp", "unlink", "rmdir"].map(name => [name, async () => {
    invoke(name); return name === "lstat" ? { ...stat } : name === "mkdtemp" ? "/unit-cache/public-authoring-v2/.download-unit" : undefined;
  }]));
  t.mock.method(v, "privateDirectory", async () => invoke("directory"));
  t.mock.method(v, "acquireLock", async () => { invoke("lock"); return async () => invoke("unlock"); });
  t.mock.method(v, "strictCachedBinary", async (file, pin) => { invoke("strict"); assert.deepEqual(pin, asset.binary); return valid; });
  t.mock.method(v, "commitVerifiedBinary", async (bytes, file, pin) => {
    invoke("commit"); assert.deepEqual(bytes, native(product, "linux-amd64")); assert.deepEqual(pin, asset.binary); valid = true;
  });
  t.mock.method(v, "downloadFile", async (url, file, pin, options) => {
    invoke("download"); assert.equal(url, `https://github.com/${c.REPOSITORY}/releases/download/${f.input.products[product].tag}/${asset.file}`);
    assert.deepEqual(pin, asset); assert.deepEqual(Object.keys(options).sort(), ["onOpen", "request", "signal"]);
  });
  t.mock.method(c, "readFile", () => { invoke("read-download"); return Buffer.from(f.outers[asset.file]); });
  t.mock.method(c, "unpack", (bytes, name) => { invoke("unpack"); assert.deepEqual(bytes, f.outers[asset.file]); assert.equal(name, asset.binary.file); return native(product, "linux-amd64"); });
  return { calls, fault, stat, options: { packageRoot: "/unit-package", cacheRoot: "/unit-cache", platform: "linux", arch: "x64", io, environment: {} } };
}
function local(m, f, product) {
  const a = f.input.products[product].assets["linux-amd64"], parent = "/custody space ü", file = `${parent}/${a.file}`;
  m.put(parent, null); m.put(file, Buffer.from(f.outers[a.file])); return file;
}
function noEffects(e) { assert.deepEqual(e.calls, []); }

if (require.main === module) {
  test("C2 schema both products and six targets", t => {
    const f = fixture();
    for (const p of c.PRODUCTS) {
      const oracle = codec.projectionBytes(f.input, p);
      assert.deepEqual(oracle.manifest, f.manifests[p]);
      assert.equal(c.digest(oracle.checksums), f.input.products[p].checksums_sha256);
      const m = memory(t, f.pair[p]);
      for (const target of c.TARGETS) {
        t.diagnostic(`schema ${p}/${target}`);
        const r = runtime.loadRelease(p, m.root, target);
        assert.deepEqual(r.asset, f.input.products[p].assets[target]); assert.equal(r.descriptor.identity.commit, f.input.identity.commit);
        assert.equal(m.handles.size, 0);
      }
      t.mock.restoreAll();
    }
  });

  test("C2 schema closed metadata and immutable bindings", async t => {
    const f = fixture();
    // The 13 existing structural tables cover every nested key/type/canonical
    // codec case; here exercise their integration and all package-only fields.
    for (const p of c.PRODUCTS) for (const field of ["name", "version", "bin", "engines", "scripts", "private", "files", ...(p === "agentplugins" ? ["os", "cpu"] : [])]) {
      t.diagnostic(`package mismatch ${p}/${field}`);
      const files = { ...f.pair[p] }, pkg = JSON.parse(files["package.json"]); pkg[field] = "wrong"; files["package.json"] = c.encode(pkg);
      const m = memory(t, files), e = engine(t, f, p);
      await assert.rejects(runtime.ensureBinary(p, e.options)); noEffects(e); assert.equal(m.handles.size, 0); t.mock.restoreAll();
    }
    for (const name of ["native-inputs.json", "public-release.json", "release-manifest.json", "package.json"]) {
      const m = memory(t, f.pair.agentplugins), e = engine(t, f, "agentplugins", true);
      e.fault.action = phase => { if (phase === "strict") m.nodes.get(`${m.root}/${name}`).ctimeMs++; };
      await assert.rejects(runtime.ensureBinary("agentplugins", e.options), /changed/); assert.equal(m.handles.size, 0); t.mock.restoreAll();
    }
    for (const p of c.PRODUCTS) {
      const input = clone(f.input); input.products[p].checksums_sha256 = hash(99);
      const body = codec.encodeInputs(input), files = { ...f.pair.agentplugins, "native-inputs.json": body };
      const d = JSON.parse(files["public-release.json"]); d.input_binding.sha256 = c.digest(body); files["public-release.json"] = codec.encodeDescriptor(d, body, "agentplugins");
      const m = memory(t, files), e = engine(t, f, "agentplugins");
      await assert.rejects(runtime.ensureBinary("agentplugins", e.options), /projection/); noEffects(e); assert.equal(m.handles.size, 0); t.mock.restoreAll();
    }
  });

  test("C2 locator validates before cold and warm effects", async t => {
    const f = fixture();
    const bad = [undefined, null, "", " ", 17, "relative", "file:///custody/asset", "~/asset", "/missing", "/a\0b"];
    for (const warm of [false, true]) {
      for (const value of bad) {
        t.diagnostic(`locator warm=${warm} value=${JSON.stringify(value)}`);
        const m = memory(t, f.pair.agentplugins), e = engine(t, f, "agentplugins", warm); e.options.environment[LOCATOR] = value;
        await assert.rejects(runtime.ensureBinary("agentplugins", e.options)); noEffects(e); assert.equal(m.handles.size, 0); t.mock.restoreAll();
      }
      for (const change of [
        (m, file) => file.replace("/custody space ü/", "/custody space ü/./"),
        (m, file) => file.replace("/custody space ü/", "/custody space ü/../custody space ü/"),
        (m, file) => { m.nodes.delete(file); return file; },
        ...["link", "directory", "fifo", "socket", "device"].map(type => (m, file) => { m.nodes.get(file).type = type; return file; }),
        ...[{ nlink: 2 }, { uid: process.geteuid() + 1 }, { mode: 0o100666 }, { mode: 0o104644 }, { size: 0 }].map(x => (m, file) => { Object.assign(m.nodes.get(file), x); return file; }),
        ...[{ mode: 0o40755 }, { mode: 0o41700 }, { uid: process.geteuid() + 1 }, { type: "link" }].map(x => (m, file) => { Object.assign(m.nodes.get(path.dirname(file)), x); return file; }),
        (m, file, e) => { e.options.cacheRoot = "/"; return file; },
        (m, file, e) => { e.options.cacheRoot = path.dirname(file); return file; },
        (m, file, e) => { e.options.cacheRoot = `${path.dirname(file)}/output`; return file; },
        (m, file, e) => { e.options.cacheRoot = m.root; return file; },
        (m, file) => { const dest = `${m.root}/${path.basename(file)}`; m.nodes.set(dest, m.nodes.get(file)); return dest; }
      ]) {
        const m = memory(t, f.pair.agentplugins), e = engine(t, f, "agentplugins", warm), file = local(m, f, "agentplugins");
        e.options.environment[LOCATOR] = change(m, file, e);
        await assert.rejects(runtime.ensureBinary("agentplugins", e.options)); noEffects(e); assert.equal(m.handles.size, 0); t.mock.restoreAll();
      }
    }
    for (const warm of [false, true]) {
      const m = memory(t, f.pair.agentplugins), e = engine(t, f, "agentplugins", warm), file = local(m, f, "agentplugins");
      const old = fs.realpathSync;
      t.mock.method(fs, "realpathSync", name => name === path.dirname(file) ? "/alias" : old(name));
      e.options.environment[LOCATOR] = file;
      await assert.rejects(runtime.ensureBinary("agentplugins", e.options), /ancestor/); noEffects(e); assert.equal(m.handles.size, 0); t.mock.restoreAll();
    }

  });

  test("C2 snapshot bounds identity and closure", t => {
    const body = Buffer.from("bounded fixture");
    for (const delta of [0, 1, -1]) {
      const m = memory(t, { file: body });
      if (delta === 0) { const s = snapshots.snapshotPublicFile(`${m.root}/file`, { kind: "metadata", maximum: body.length }); assert.deepEqual(s.bytes, body); s.bytes.fill(0); s.recheck(); s.close(); s.close(); }
      else assert.throws(() => snapshots.snapshotPublicFile(`${m.root}/file`, { kind: "metadata", maximum: body.length + delta, exactSize: body.length + delta }));
      assert.equal(m.handles.size, 0); t.mock.restoreAll();
    }
    for (const field of ["ino", "dev", "mode", "uid", "gid", "size", "nlink", "mtimeMs", "ctimeMs", "content", "ancestor", "replacement", "missing"]) {
      t.diagnostic(`snapshot recheck ${field}`);
      const m = memory(t, { file: body }), file = `${m.root}/file`, s = snapshots.snapshotPublicFile(file, { kind: "metadata", maximum: 100 });
      const node = m.nodes.get(file);
      if (field === "content") node.body = Buffer.alloc(body.length, 120);
      else if (field === "ancestor") m.nodes.get(m.root).ino++;
      else if (field === "replacement") m.put(file, body);
      else if (field === "missing") m.nodes.delete(file);
      else node[field]++;
      assert.throws(() => s.recheck()); s.close(); assert.equal(m.handles.size, 0); t.mock.restoreAll();
    }
    for (const fault of ["openSync", "readSync", "fstatSync", "lstatSync", "closeSync", "growth", "short"]) {
      t.diagnostic(`snapshot fault ${fault}`);
      const m = memory(t, { file: body }); let fired = false;
      m.fault.action = (name, args) => {
        if (!fired && name === (fault === "growth" || fault === "short" ? "readSync" : fault)) {
          fired = true;
          if (fault === "growth" || fault === "short") m.nodes.get(`${m.root}/file`).body = fault === "growth" ? Buffer.concat([body, body]) : body.subarray(1);
          else if (fault === "closeSync") { m.handles.delete(args[0]); throw new Error("mock close failure"); }
          else throw new Error(`mock ${fault}`);
        }
      };
      assert.throws(() => { const s = snapshots.snapshotPublicFile(`${m.root}/file`, { kind: "metadata", maximum: 100 }); s.close(); });
      assert.equal(fired, true); assert.equal(m.handles.size, 0); t.mock.restoreAll();
    }
    for (const phase of ["initial", "recheck"]) {
      const m = memory(t, { file: body }), controller = new AbortController();
      if (phase === "initial") controller.abort();
      let held;
      assert.throws(() => {
        held = snapshots.snapshotPublicFile(`${m.root}/file`, { kind: "metadata", maximum: 100, signal: controller.signal });
        controller.abort(); held.recheck();
      }, /cancelled/);
      if (held) held.close(); assert.equal(m.handles.size, 0); t.mock.restoreAll();
    }

  });

  test("C2 local acquisition uses the shared checked engine", async t => {
    const f = fixture();
    for (const p of c.PRODUCTS) for (const warm of [false, true]) {
      t.diagnostic(`local ${p} warm=${warm}`);
      const m = memory(t, f.pair[p]), e = engine(t, f, p, warm), file = local(m, f, p);
      e.options.environment[LOCATOR] = file;
      const result = await runtime.ensureBinary(p, e.options);
      assert.equal(result.cacheHit, warm); assert.notEqual(result.binaryPath, file); assert.equal(e.calls.includes("download"), false);
      assert.equal(e.calls.filter(n => n === "unpack").length, p === "plugin-kit-ai" ? 1 : 0);
      assert.equal(e.calls.includes("commit"), !warm); assert.equal(m.handles.size, 0); t.mock.restoreAll();
    }
    for (const failure of ["outer", "inner", "unpack"]) for (const warm of [false, true]) {
      t.diagnostic(`local pin failure ${failure} warm=${warm}`);
      const p = "plugin-kit-ai", m = memory(t, f.pair[p]), e = engine(t, f, p, warm), file = local(m, f, p);
      e.options.environment[LOCATOR] = file;
      if (failure === "outer") m.nodes.get(file).body = Buffer.alloc(m.nodes.get(file).size, 120);
      else t.mock.method(c, "unpack", () => { if (failure === "unpack") throw new Error("mock unpack"); return Buffer.from("wrong"); });
      await assert.rejects(runtime.ensureBinary(p, e.options)); noEffects(e); assert.equal(m.handles.size, 0); t.mock.restoreAll();
    }
  });

  test("C2 anonymous acquisition keeps canonical transport", async t => {
    const f = fixture();
    for (const p of c.PRODUCTS) {
      const m = memory(t, f.pair[p]), e = engine(t, f, p);
      e.options.environment = { PLUGIN_KIT_AI_REPOSITORY: "wrong", PLUGIN_KIT_AI_VERSION: "wrong", UAP_PUBLIC_AUTHORING_VERIFIED: "true" };
      const result = await runtime.ensureBinary(p, e.options);
      assert.equal(result.repository, c.REPOSITORY); assert.ok(e.calls.indexOf("download") < e.calls.indexOf("commit")); assert.equal(m.handles.size, 0); t.mock.restoreAll();
    }
  });

  test("C2 v2 cache isolation and lock ordering", async t => {
    const f = fixture(), paths = new Set();
    for (const p of c.PRODUCTS) for (const target of c.TARGETS) {
      const m = memory(t, f.pair[p]), r = runtime.loadRelease(p, m.root, target);
      const pathname = runtime.cachePath("/cache", p, target, r); paths.add(pathname);
      assert.ok(pathname.startsWith("/cache/public-authoring-v2/"));
      assert.notEqual(pathname, runtime.cachePath("/cache", p, target, { ...r, descriptor: { ...r.descriptor, schema: runtime.SCHEMA } }));
      for (const changed of [{ ...r, version: "3.0.0" }, { ...r, descriptor: { ...r.descriptor, identity: { ...r.descriptor.identity, commit: "b".repeat(40) } } },
        { ...r, asset: { ...r.asset, binary: { ...r.asset.binary, sha256: hash(99) } } }]) assert.notEqual(pathname, runtime.cachePath("/cache", p, target, changed));
      t.mock.restoreAll();
    }
    assert.equal(paths.size, 12);
    let expected;
    for (const supplied of [false, true]) for (const warm of [false, true]) {
      const m = memory(t, f.pair.agentplugins), e = engine(t, f, "agentplugins", warm);
      if (supplied) e.options.environment[LOCATOR] = local(m, f, "agentplugins");
      const r = await runtime.ensureBinary("agentplugins", e.options);
      if (expected) assert.equal(r.binaryPath, expected); expected = r.binaryPath;
      assert.ok(e.calls.indexOf("lock") < e.calls.indexOf("strict")); assert.equal(e.calls.at(-1), "unlock");
      assert.equal(e.calls.filter(n => n === "strict").length, 2); assert.equal(m.handles.size, 0); t.mock.restoreAll();
    }
    for (const problem of ["timeout", "unsafe", "directory replacement", "final binary"]) {
      t.diagnostic(`cache fault ${problem}`);
      const m = memory(t, f.pair.agentplugins), e = engine(t, f, "agentplugins", true);
      let strict = 0;
      e.fault.action = phase => {
        if (problem === "timeout" && phase === "lock") throw Object.assign(new Error("mock lock timeout"), { code: "ETIMEDOUT" });
        if (problem === "unsafe" && phase === "strict") throw new Error("mock unsafe cached object");
        if (problem === "directory replacement" && phase === "strict") e.stat.ino++;
      };
      if (problem === "final binary") t.mock.method(v, "strictCachedBinary", async () => ++strict === 1);
      await assert.rejects(runtime.ensureBinary("agentplugins", e.options), /timeout|unsafe|replaced|final public/);
      assert.equal(e.calls.filter(n => n === "lock").length, 1); assert.equal(e.calls.includes("commit"), false);
      assert.equal(m.handles.size, 0); t.mock.restoreAll();
    }

  });

  test("C2 failures cancel and preserve caller state", async t => {
    const f = fixture();
    for (const phase of ["mkdir", "directory", "lock", "strict", "download", "commit", "unlock", "cancel", "precommit", "final", "close"]) {
      t.diagnostic(`acquisition failure ${phase}`);
      const m = memory(t, f.pair.agentplugins), e = engine(t, f, "agentplugins"), controller = new AbortController(); e.options.signal = controller.signal;
      const original = Buffer.from(m.nodes.get(`${m.root}/package.json`).body); let fired = false;
      if (phase === "close") m.fault.action = (name, args) => { if (name === "closeSync" && e.calls.includes("commit")) { m.handles.delete(args[0]); throw new Error("mock close failure"); } };
      else e.fault.action = name => {
        if (phase === "cancel" && name === "lock") controller.abort();
        if (phase === "precommit" && name === "read-download") m.nodes.get(`${m.root}/package.json`).ino++;
        if (phase === "final" && name === "commit") m.nodes.get(`${m.root}/package.json`).ctimeMs++;
        if (name === phase && !fired) { fired = true; throw new Error(`mock ${phase}`); }
      };
      await assert.rejects(runtime.ensureBinary("agentplugins", e.options));
      assert.deepEqual(m.nodes.get(`${m.root}/package.json`).body, original); assert.equal(m.handles.size, 0); t.mock.restoreAll();
    }
    for (const warm of [false, true]) for (const checkpoint of warm ? ["unpack", "lock", "strict"] : ["unpack", "lock", "strict", "commit"]) {
      for (const field of ["ino", "uid", "mode", "nlink", "mtimeMs", "ctimeMs", "content", "parent"]) {
        t.diagnostic(`custody recheck warm=${warm} checkpoint=${checkpoint} field=${field}`);
        const p = "plugin-kit-ai", m = memory(t, f.pair[p]), e = engine(t, f, p, warm), file = local(m, f, p);
        e.options.environment[LOCATOR] = file; let changed = false;
        e.fault.action = phase => {
          if (phase !== checkpoint || changed) return; changed = true;
          if (field === "content") m.nodes.get(file).body = Buffer.alloc(m.nodes.get(file).size, 120);
          else if (field === "parent") m.nodes.get(path.dirname(file)).ino++;
          else m.nodes.get(file)[field]++;
        };
        await assert.rejects(runtime.ensureBinary(p, e.options), /changed|unsafe|regular|custody/);
        assert.equal(changed, true); assert.equal(m.handles.size, 0);
        if (checkpoint === "unpack") assert.deepEqual(e.calls, ["unpack"]);
        t.mock.restoreAll();
      }
    }
    for (const checkpoint of ["lock", "download", "commit"]) {
      const m = memory(t, f.pair.agentplugins), e = engine(t, f, "agentplugins");
      const primary = new Error(`mock primary ${checkpoint}`), cleanup = new Error("mock unlock uncertainty");
      e.fault.action = phase => { if (phase === checkpoint) throw primary; if (phase === "unlock") throw cleanup; };
      let caught;
      try { await runtime.ensureBinary("agentplugins", e.options); } catch (error) { caught = error; }
      assert.ok(caught);
      if (checkpoint === "lock") assert.strictEqual(caught, primary);
      else { assert.ok(caught.message.includes(primary.message)); assert.ok(caught.message.includes(cleanup.message)); }
      assert.equal(m.handles.size, 0); t.mock.restoreAll();
    }

  });

  test("C2 terminal cancellation rejects after successful awaited unlock", async t => {
    const f = fixture();
    for (const p of c.PRODUCTS) for (const warm of [false, true]) for (const supplied of [false, true]) {
      t.diagnostic(`terminal cancel ${p} warm=${warm} local=${supplied}`);
      const m = memory(t, f.pair[p]), e = engine(t, f, p, warm), controller = new AbortController();
      e.options.signal = controller.signal;
      if (supplied) e.options.environment[LOCATOR] = local(m, f, p);
      t.mock.method(v, "acquireLock", async () => {
        e.calls.push("lock");
        return async () => { await Promise.resolve(); assert.ok(m.handles.size >= 4); e.calls.push("unlock"); controller.abort(); };
      });
      await assert.rejects(runtime.ensureBinary(p, e.options), /cancelled/);
      assert.equal(controller.signal.aborted, true); assert.equal(e.calls.at(-1), "unlock");
      assert.equal(e.calls.includes("commit"), !warm); assert.equal(m.handles.size, 0);
      // A late rejection may leave the verified cache entry for a fresh operation.
      e.options.signal = undefined;
      e.fault.action = () => {};
      t.mock.method(v, "acquireLock", async () => async () => {});
      assert.equal((await runtime.ensureBinary(p, e.options)).cacheHit, true);
      assert.equal(m.handles.size, 0); t.mock.restoreAll();
    }
  });

  test("C2 terminal retained inputs and placement reject after successful awaited unlock", async t => {
    const f = fixture();
    for (const p of c.PRODUCTS) for (const warm of [false, true]) for (const field of ["package.json", "native-inputs.json", "public-release.json", "release-manifest.json", "locator", "placement"]) {
      t.diagnostic(`terminal replacement ${p} warm=${warm} field=${field}`);
      const m = memory(t, f.pair[p]), e = engine(t, f, p, warm), file = local(m, f, p);
      e.options.environment[LOCATOR] = file;
      if (field === "placement") m.put("/unit-cache", null);
      let changed = false;
      t.mock.method(v, "acquireLock", async () => {
        e.calls.push("lock");
        return async () => {
          await Promise.resolve(); assert.equal(m.handles.size, 5); e.calls.push("unlock");
          if (field === "placement") m.put("/unit-cache", null, { type: "link" });
          else { const name = field === "locator" ? file : `${m.root}/${field}`; m.put(name, Buffer.from(m.nodes.get(name).body)); }
          changed = true;
        };
      });
      await assert.rejects(runtime.ensureBinary(p, e.options), /changed|unsafe ancestor/);
      assert.equal(changed, true); assert.equal(e.calls.at(-1), "unlock");
      assert.equal(e.calls.includes("commit"), !warm); assert.equal(m.handles.size, 0); t.mock.restoreAll();
    }
  });

  test("C2 terminal primary and snapshot close errors aggregate", async t => {
    const f = fixture(), m = memory(t, f.pair.agentplugins), e = engine(t, f, "agentplugins"), controller = new AbortController();
    e.options.signal = controller.signal;
    e.fault.action = phase => { if (phase === "unlock") controller.abort(); };
    m.fault.action = (name, args) => {
      if (name === "closeSync" && e.calls.includes("unlock")) { m.handles.delete(args[0]); throw new Error("terminal close uncertainty"); }
    };
    await assert.rejects(runtime.ensureBinary("agentplugins", e.options), error => {
      assert.match(error.message, /cancelled/); assert.match(error.message, /terminal close uncertainty/); return true;
    });
    assert.equal(m.handles.size, 0);
  });

  test("C2 v1 null and legacy contracts remain unchanged", async t => {
    const f = fixture();
    for (const p of c.PRODUCTS) {
      const files = stage.packageFiles(p, source, f.manifests[p], { identity: f.input.identity, manifestDigest: f.input.candidate_sha256 });
      assert.equal(Object.keys(files).length, 14); assert.equal(JSON.parse(files["package.json"]).private, true);
      for (const locator of ["/anything", ""]) {
        const m = memory(t, files), e = engine(t, f, p);
        t.mock.method(c, "readFile", file => files[path.basename(file)]);
        e.options.environment[LOCATOR] = locator;
        await assert.rejects(runtime.ensureBinary(p, e.options), /not qualified/); noEffects(e); assert.equal(m.handles.size, 0); t.mock.restoreAll();
      }
    }
  });

  test("C2 child environment removes the locator", async () => {
    await require("./public-authoring-v2-pack.test").launchers(fixture());
    assert.deepEqual(runtime.childEnvironment({ [LOCATOR]: "/untrusted", NODE_OPTIONS: "preload", UAP_PUBLIC_AUTHORING_TEST: "true",
      UAP_PRIVATE_NPM_ROOT: "private", AGENTPLUGINS_INTERNAL_PROOF_BINARY: "private", PLUGIN_KIT_AI_VERSION: "wrong",
      PLUGIN_KIT_AI_REPOSITORY: "wrong", PLUGIN_KIT_AI_RELEASE_BASE_URL: "wrong", KEEP: "yes", PATH: "/ordinary" }), { KEEP: "yes", PATH: "/ordinary" });
  });
}
module.exports = { fixture, memory, engine, source, local, LOCATOR, native };
