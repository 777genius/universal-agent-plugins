"use strict";
if (process.env.AGENTPLUGINS_STAGED_TEST_CHILD === "1") {
  require("node:test")("source-checkout-only suite", { skip: "requires the complete repository source tree" }, () => {});
} else {

// In-memory package closure and actual JavaScript launcher interfaces only.
const test = require("node:test");
const assert = require("node:assert/strict");
const vm = require("node:vm");
const path = require("node:path");
const fs = require("node:fs");
const cp = require("node:child_process");
const { EventEmitter } = require("node:events");
const c = require("../scripts/dual-authoring-candidate");
const codec = require("../lib/public-authoring-contract");
const facade = require("../scripts/authoring-native-inputs");
const stage = require("../scripts/stage-authoring-npm");
const packing = require("../scripts/stage-dual-authoring-npm");
const { fixture, source, LOCATOR } = require("./public-authoring-v2.test");
const root = path.resolve(__dirname, "../../..");
const inventory = p => ["LICENSE", "README.md", "package.json", `bin/${p}.js`, "bin/package.json", "lib/package.json",
  "lib/platform.js", "lib/verifier.js", "lib/public-authoring.js", "lib/public-authoring-contract.js", "lib/public-authoring-input.js",
  p === "agentplugins" ? "lib/bootstrap.js" : "lib/install.js", "scripts/package.json", "scripts/dual-authoring-candidate.js",
  "public-release.json", "release-manifest.json", "native-inputs.json"].sort();

function loader(files, overrides = {}, processValue = process) {
  const cache = new Map(), loaded = [], builtins = new Set(["fs", "fs/promises", "os", "path", "crypto", "util", "child_process", "https", "http", "zlib", "stream", "stream/promises", "url"]);
  function load(name, main = false) {
    if (cache.has(name)) return cache.get(name).exports;
    assert.ok(Object.hasOwn(files, name), `require escaped the fixed package closure: ${name}`);
    const module = { exports: {} }; cache.set(name, module); loaded.push(name);
    function localRequire(request) {
      const builtin = request.replace(/^node:/, "");
      if (!request.startsWith(".")) {
        assert.ok(builtins.has(builtin), `unapproved external module ${request}`);
        return Object.hasOwn(overrides, builtin) ? overrides[builtin] : require(`node:${builtin}`);
      }
      let target = path.posix.normalize(path.posix.join(path.posix.dirname(name), request));
      if (!path.posix.extname(target)) target += ".js";
      return load(target);
    }
    localRequire.main = main ? module : null;
    if (name.endsWith(".json")) module.exports = JSON.parse(files[name]);
    else {
      const code = files[name].toString().replace(/^#![^\n]*\n/, "");
      const fn = vm.runInNewContext(`(function(require,module,exports,__filename,__dirname){${code}\n})`,
        { Buffer, process: processValue, console, setTimeout, clearTimeout, URL, AbortController }, { filename: name });
      fn(localRequire, module, module.exports, `/packed/${name}`, `/packed/${path.posix.dirname(name)}`);
    }
    return module.exports;
  }
  return { load, loaded };
}

async function launchers(f) {
  for (const p of c.PRODUCTS) for (const failed of [false, true]) for (const postinstall of p === "plugin-kit-ai" ? [false, true] : [false]) {
    const calls = [], child = new EventEmitter();
    const env = { KEEP: "yes", [LOCATOR]: "/custody/untrusted", NODE_OPTIONS: "preload", UAP_PRIVATE_NPM_ROOT: "private", AGENTPLUGINS_INTERNAL_PROOF_BINARY: "private" };
    const proc = { env, argv: ["node", "launcher", "argument with spaces", "--format", "json"], pid: 12,
      platform: "linux", arch: "x64", cwd: () => "/caller", stderr: { write: text => calls.push(["stderr", text]) },
      exit: code => { proc.exitCode = code; }, kill: (pid, signal) => calls.push(["kill", pid, signal]) };
    const fakeFS = { ...fs, lstatSync: () => ({ isFile: () => true }),
      readFileSync: file => { assert.equal(file, "/packed/package.json"); return f.pair[p]["package.json"]; } };
    const l = loader(f.pair[p], { fs: fakeFS, child_process: { spawn: (file, args, options) => {
      calls.push(["spawn", file, args, options]); return child;
    } } }, proc);
    const api = l.load("lib/public-authoring.js");
    api.ensureBinary = async (product, options) => {
      calls.push(["runtime", product, options]); assert.equal(product, p); assert.equal(options.packageRoot, "/packed");
      if (failed) throw new Error("synthetic runtime rejection");
      return { binaryPath: "/checked/cache/binary", publicAuthoring: true };
    };
    l.load(postinstall ? "lib/install.js" : `bin/${p}.js`, postinstall);
    await new Promise(resolve => setImmediate(resolve));
    assert.equal(calls.filter(c => c[0] === "runtime").length, 1);
    const spawned = calls.find(c => c[0] === "spawn");
    if (failed || postinstall) assert.equal(spawned, undefined);
    else {
      assert.equal(spawned[1], "/checked/cache/binary"); assert.deepEqual([...spawned[2]], proc.argv.slice(2));
      assert.equal(spawned[3].cwd, undefined); assert.equal(spawned[3].stdio, "inherit");
      assert.deepEqual({ ...spawned[3].env }, { KEEP: "yes" });
      const exit = child.listeners("exit")[0];
      child.emit("exit", 7, null); assert.equal(proc.exitCode, 7);
      exit(null, "SIGTERM"); assert.deepEqual(calls.at(-1), ["kill", 12, "SIGTERM"]);
      child.emit("error", new Error("mock spawn")); assert.equal(proc.exitCode, 1);
    }
    if (failed) assert.equal(proc.exitCode, 1);
  }
}

if (require.main === module) {
test("C2 closure both public packages remain self contained", async () => {
  const f = fixture();
  for (const p of c.PRODUCTS) {
    const files = f.pair[p]; assert.deepEqual(Object.keys(files).sort(), inventory(p)); assert.equal(Object.keys(files).length, 17);
    const pkg = JSON.parse(files["package.json"]), base = JSON.parse(source[`npm/${p}/package.json`].bytes);
    assert.deepEqual(pkg, { ...base, version: f.input.identity.versions[p], private: false, files: inventory(p) });
    for (const dir of ["bin", "lib", "scripts"]) assert.deepEqual(JSON.parse(files[`${dir}/package.json`]), { type: "commonjs" });
    const modes = Object.fromEntries(Object.keys(files).map(n => [n, n === `bin/${p}.js` ? 0o755 : 0o644]));
    assert.equal(Object.values(modes).filter(n => n === 0o755).length, 1);
    const l = loader(files);
    for (const n of ["lib/public-authoring.js", "lib/public-authoring-contract.js", "lib/public-authoring-input.js", p === "agentplugins" ? "lib/bootstrap.js" : "lib/install.js"]) l.load(n);
    assert.ok(l.loaded.includes("scripts/dual-authoring-candidate.js"));
    assert.equal(l.loaded.some(n => /promotion|native-inputs\.js|provider|workflow/.test(n)), false);
    const missing = { ...files }; delete missing["lib/public-authoring-contract.js"];
    assert.throws(() => loader(missing).load("lib/public-authoring-contract.js"), /closure/);
  }
  for (const n of ["native-inputs.json", "lib/public-authoring-contract.js", "lib/public-authoring-input.js", "lib/public-authoring.js"]) {
    assert.deepEqual(f.pair.agentplugins[n], f.pair["plugin-kit-ai"][n]); assert.notStrictEqual(f.pair.agentplugins[n], f.pair["plugin-kit-ai"][n]);
  }
  await launchers(f);
});

test("C2 closure executing helpers are pinned at F", t => {
  const f = fixture(), commit = f.input.identity.commit;
  for (const problem of [null, "HEAD", "missing", "blob", "mode", "checkout", "executing"]) {
    const helper = "npm/agentplugins/lib/public-authoring-contract.js";
    const byBlob = new Map(Object.values(source).map(pin => [pin.git_blob, pin.bytes]));
    t.mock.method(c, "safeDirectory", value => value);
    t.mock.method(cp, "execFileSync", (exe, args) => {
      assert.equal(exe, "/usr/bin/git");
      if (args[0] === "rev-parse") return Buffer.from(problem === "HEAD" ? "b".repeat(40) : commit);
      if (args[0] === "ls-tree") {
        const name = args.at(-1), pin = source[name];
        if (name === helper && problem === "missing") return Buffer.alloc(0);
        return Buffer.from(`${name === helper && problem === "mode" ? "120000" : pin.mode} blob ${pin.git_blob}\t${name}\0`);
      }
      assert.equal(args[0], "cat-file");
      if (problem === "blob" && args[2] === source[helper].git_blob) return Buffer.from("drift");
      return byBlob.get(args[2]);
    });
    t.mock.method(c, "readFile", file => {
      const name = file.startsWith("/checkout/") ? file.slice(10) : path.relative(root, file);
      if (name === helper && ((problem === "checkout" && file.startsWith("/checkout/")) || (problem === "executing" && !file.startsWith("/checkout/")))) return Buffer.from("drift");
      return source[name].bytes;
    });
    t.mock.method(fs, "lstatSync", file => ({ mode: source[file.startsWith("/checkout/") ? file.slice(10) : path.relative(root, file)]?.mode === "100755" ? 0o755 : 0o644 }));
    if (problem) assert.throws(() => packing.blobs("/checkout", commit, {}, "stage"));
    else {
      assert.deepEqual(Object.keys(packing.blobs("/checkout", commit, {}, "stage")), stage.STAGE_ALLOWLIST);
      assert.deepEqual(Object.keys(packing.blobs("/checkout", commit, {}, "public")), stage.ALLOWLIST);
    }
    t.mock.restoreAll();
  }
  for (const helper of ["lib/public-authoring-contract.js", "lib/public-authoring-input.js"]) {
    const name = "npm/agentplugins/" + helper;
    for (const mutate of [s => { delete s[name]; }, s => { s.extra = s[name]; }, s => { s[name] = { ...s[name], sha256: "f".repeat(64) }; }, s => { s[name] = { ...s[name], mode: "100755" }; },
      s => { s[name] = { ...s[name], bytes: Buffer.from("changed") }; }]) {
      const altered = { ...source }; mutate(altered); assert.throws(() => stage.pairedPackageFiles(altered, f.manifests, f.inputBytes));
    }
  }
});

test("C2 closure producer codecs and v1 bytes stay compatible", () => {
  const f = fixture();
  for (const name of ["encodeInputs", "decodeInputs", "encodeDescriptor", "decodeDescriptor"]) assert.strictEqual(facade[name], codec[name]);
  assert.equal(Object.hasOwn(facade, "checks"), false);
  for (const name of ["produceInputs", "readInputs", "inputSubjects"]) assert.equal(Object.hasOwn(codec, name), false);
  assert.equal(codec.MAX_INPUT_BYTES, 1024 * 1024); assert.equal(codec.MAX_DESCRIPTOR_BYTES, 64 * 1024);
  const before = codec.encodeInputs(f.input); assert.deepEqual(codec.decodeInputs(before), f.input); assert.deepEqual(codec.encodeInputs(f.input), before);
  for (const p of c.PRODUCTS) {
    const v1 = stage.packageFiles(p, source, f.manifests[p], { identity: f.input.identity, manifestDigest: f.input.candidate_sha256 });
    assert.deepEqual(Object.keys(v1).sort(), inventory(p).filter(n => !["native-inputs.json", "lib/public-authoring-contract.js", "lib/public-authoring-input.js"].includes(n)));
    const l = loader(v1); l.load("lib/public-authoring.js");
    assert.equal(l.loaded.some(n => /public-authoring-(contract|input)/.test(n)), false);
    assert.equal(JSON.parse(v1["public-release.json"]).qualification, null);
    for (const name of Object.keys(v1).filter(n => !["public-release.json", "package.json"].includes(n))) assert.deepEqual(v1[name], f.pair[p][name]);
    const descriptor = f.pair[p]["public-release.json"];
    assert.deepEqual(codec.encodeDescriptor(codec.decodeDescriptor(descriptor, before, p), before, p), descriptor);
    for (const malformed of [Buffer.alloc(0), Buffer.concat([descriptor, Buffer.from(" ")]), Buffer.from("{}")]) {
      let first, second;
      try { facade.decodeDescriptor(malformed, before, p); } catch (error) { first = error.message; }
      try { codec.decodeDescriptor(malformed, before, p); } catch (error) { second = error.message; }
      assert.ok(first); assert.equal(first, second);
    }
  }
});

}
module.exports = { loader, launchers };
}
