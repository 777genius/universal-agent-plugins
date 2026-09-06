"use strict";
const nodeTest = require("node:test");
const enabled = process.platform === "linux" && process.env.AGENTPLUGINS_STAGED_TEST_CHILD !== "1";
const test = (name, fn) => nodeTest.test(name, { skip: !enabled }, fn);
const before = fn => nodeTest.before(() => { if (enabled) return fn(); });
const { EventEmitter } = require("node:events");
const { fs, path, cp, assert, c, s, b, mkdir, node, fixture, installPair } = require("./private-npm-fixture");
const v = require("../lib/verifier");
const { supervise, SHUTDOWN_MS } = require("../scripts/private-npm/launcher");
let f;
before(() => { f = installPair(fixture("launcher")); });
const delay = ms => new Promise(resolve => setTimeout(resolve, ms));
async function until(predicate) {
  const deadline = Date.now() + 8000;
  while (!predicate()) { if (Date.now() > deadline) throw new Error("fixture readiness deadline"); await delay(10); }
}
function wrapper(product, args = [], env = f.envFor(), preload) {
  const child = cp.spawn(node, [...(preload ? ["--require", preload] : []), path.join(f.packageRoot(product), "bin", product + ".js"), ...args], {
    env, cwd: f.cwd, stdio: ["ignore", "pipe", "pipe"]
  });
  let stdout = "", stderr = ""; child.stdout.on("data", b => { stdout += b; }); child.stderr.on("data", b => { stderr += b; });
  const done = new Promise((resolve, reject) => {
    const timer = setTimeout(() => { child.kill("SIGKILL"); reject(new Error("wrapper exceeded fixture deadline")); }, 10000);
    child.once("error", reject);
    child.once("close", (code, signal) => { clearTimeout(timer); resolve({ code, signal, stdout, stderr }); });
  });
  return { child, done };
}
function observer(root, product, log) {
  const preload = path.join(root, product + "-observer.js");
  const asset = path.join(f.source, f.manifest.products[product].assets["linux-amd64"].file);
  fs.writeFileSync(preload, `const fs=require('node:fs');const open=fs.openSync;fs.openSync=function(p,...args){const fd=open.call(this,p,...args);if(p===${JSON.stringify(asset)})fs.appendFileSync(${JSON.stringify(log)},process.pid+'\\n');return fd};\n`);
  return preload;
}
for (const product of c.PRODUCTS) test(`POSIX STRUCTURAL independent ${product} processes: exactly one cold/corrupt acquisition, warm has none`, async () => {
  const root = mkdir(path.join(f.root, product + " concurrency"));
  const cache = mkdir(path.join(root, "cache")), log = path.join(root, "reads");
  const preload = observer(root, product, log);
  for (const phase of ["cold", "corrupt", "warm"]) {
    if (phase === "corrupt") fs.writeFileSync(f.binaryPath(product, cache), "owned corrupt bytes");
    if (phase === "warm") fs.renameSync(f.source, f.source + "-unavailable");
    const count = fs.existsSync(log) ? fs.readFileSync(log, "utf8").trim().split("\n").length : 0;
    try {
      const processes = [wrapper(product, [], f.envFor(cache), preload), wrapper(product, [], f.envFor(cache), preload)];
      assert.notEqual(processes[0].child.pid, processes[1].child.pid);
      const results = await Promise.all(processes.map(p => p.done));
      for (const r of results) { assert.equal(r.code, 0, r.stderr); assert.equal(JSON.parse(r.stdout).product, product); }
      const after = fs.readFileSync(log, "utf8").trim().split("\n").length;
      assert.equal(after - count, phase === "warm" ? 0 : 1, "selected source open count across processes");
      assert.equal(c.digest(fs.readFileSync(f.binaryPath(product, cache))), f.manifest.products[product].assets["linux-amd64"].binary.sha256);
      assert.deepEqual(fs.readdirSync(path.join(cache, ".locks")), []);
    } finally { if (phase === "warm") fs.renameSync(f.source + "-unavailable", f.source); }
  }
});

for (const product of c.PRODUCTS) for (const signal of ["SIGINT", "SIGTERM"]) test(`POSIX STRUCTURAL ${product}: parent ${signal}, child ${signal}, stubborn cleanup`, async () => {
  const direct = await wrapper(product, ["signal", signal]).done;
  assert.equal(direct.signal, signal); assert.equal(direct.stdout, ""); assert.equal(direct.stderr, "");
  for (const mode of ["wait", "stubborn"]) {
    const ready = path.join(f.root, `${product}-${signal}-${mode}`);
    const p = wrapper(product, [mode, ready]); let pid;
    try {
      await until(() => fs.existsSync(ready)); pid = Number(fs.readFileSync(ready, "utf8"));
      const start = Date.now(); p.child.kill(signal);
      await until(() => fs.existsSync(ready + ".signals"));
      p.child.kill(signal); await delay(40);
      // Hold the cooperative child alive across BOTH requests; a signal sent
      // after wrapper teardown is an external termination, not forwarding.
      if (mode === "wait") fs.writeFileSync(ready + ".release", "finish");
      const result = await p.done;
      assert.equal(result.stdout, ""); assert.equal(result.stderr, "");
      if (mode === "stubborn") { assert.equal(result.signal, "SIGKILL"); assert.ok(Date.now() - start >= SHUTDOWN_MS - 100); }
      else assert.equal(result.code, 0);
      assert.ok(Date.now() - start < SHUTDOWN_MS + 3000);
      assert.equal(fs.readFileSync(ready + ".signals", "utf8"), signal + "\n");
      assert.throws(() => process.kill(pid, 0), { code: "ESRCH" }, "actual child is gone");
    } finally {
      if (pid) try { process.kill(pid, "SIGKILL"); } catch {}
      try { p.child.kill("SIGKILL"); } catch {}
    }
  }
});

for (const warm of [false, true]) for (const signal of ["SIGINT", "SIGTERM"]) test(`POSIX STRUCTURAL parent ${signal} during ${warm ? "warm" : "cold"} lock wait preserves unowned lock and other product`, async () => {
  assert.equal(f.launch("agentplugins").status, 0);
  const cache = warm ? f.cache : mkdir(path.join(f.root, "cold-wait-" + signal));
  const binary = f.binaryPath("agentplugins", cache), lockRoot = path.join(cache, ".locks");
  if (!warm) mkdir(lockRoot);
  const unlock = await v.acquireLock(binary, { lockRoot });
  const names = fs.readdirSync(lockRoot), pins = names.map(n => fs.readFileSync(path.join(lockRoot, n)));
  const ready = path.join(f.root, `lock-wait-${warm}-${signal}`), preload = ready + ".js";
  fs.writeFileSync(preload, `const fs=require('node:fs');const open=fs.promises.open;fs.promises.open=async function(p,...a){try{return await open.call(this,p,...a)}catch(e){if(String(p).endsWith('.lock')&&e.code==='EEXIST')fs.writeFileSync(${JSON.stringify(ready)},'waiting');throw e}};`);
  const p = wrapper("agentplugins", [], f.envFor(cache), preload);
  try {
    // Observe a live waiter: it cannot finish while the same warm target is held.
    await until(() => fs.existsSync(ready)); assert.equal(p.child.exitCode, null);
    assert.equal(f.launch("plugin-kit-ai").status, 0);
    p.child.kill(signal); const result = await p.done;
    assert.equal(result.signal, signal); assert.equal(result.stdout, "");
    assert.deepEqual(fs.readdirSync(lockRoot), names);
    names.forEach((n, i) => assert.deepEqual(fs.readFileSync(path.join(lockRoot, n)), pins[i]));
  } finally { await unlock(); p.child.kill("SIGKILL"); }
});

test("STRUCTURAL detached real spawn error is bounded product stderr only", () => {
  const g = installPair(fixture("spawn-error", undefined, "spawn-error"));
  for (const product of c.PRODUCTS) {
    const r = g.launch(product); assert.equal(r.status, 1); assert.equal(r.stdout, "");
    assert.match(r.stderr, new RegExp(`^${product} private npm launcher: `)); assert.ok(r.stderr.length < 700);
    assert.deepEqual(fs.readdirSync(path.join(g.cache, ".locks")), []);
  }
});

// Deterministic event ordering supplements, never replaces, detached processes.
function host() {
  const h = new EventEmitter(); h.env = {}; h.argv = ["node", "shim", "", "literal;value"];
  h.cwd = () => "/structural fixture"; h.errors = ""; h.stderr = { write(s) { h.errors += s; } }; return h;
}
for (const race of ["acquisition", "spawn", "exit", "error-close", "throw", "cleanup-uncertainty"]) test(`STRUCTURAL deterministic launcher ${race} race waits for cleanup/close`, async () => {
  const h = host(), child = new EventEmitter(); child.pid = 123; const kills = [];
  child.kill = signal => { kills.push(signal); return true; };
  let completed = false, released = false, close;
  const result = supervise("agentplugins", "/fixture", {
    host: h,
    async acquire(product, options) {
      assert.equal(options.expectedMode, s.MODE);
      if (race === "acquisition" || race === "cleanup-uncertainty") {
        h.emit("SIGINT"); assert.equal(options.signal.aborted, true); await delay(5); released = true;
        throw new Error(race === "cleanup-uncertainty" ? "owned cleanup uncertainty" : "cancelled");
      }
      return { binaryPath: "/verified" };
    },
    spawn(file, argv, options) {
      assert.equal(file, "/verified"); assert.deepEqual(argv, h.argv.slice(2)); assert.equal(options.shell, false);
      assert.equal(options.stdio, "inherit"); assert.equal(options.cwd, h.cwd());
      if (race === "throw") throw new Error("synchronous spawn failure\n" + "x".repeat(1000));
      if (race === "spawn") h.emit("SIGTERM");
      close = () => child.emit("close", 37, null);
      queueMicrotask(() => {
        child.emit("spawn");
        if (race === "error-close") child.emit("error", new Error("spawn failure"));
        child.emit("exit", 37, null);
        if (race === "exit") { h.emit("SIGINT"); h.emit("SIGTERM"); }
      });
      return child;
    }
  }).then(r => { completed = true; return r; });
  await delay(20);
  if (close) { assert.equal(completed, false, "exit is not stdio close"); close(); }
  const r = await result;
  if (["acquisition", "cleanup-uncertainty"].includes(race)) { assert.equal(released, true); assert.equal(r.signal, "SIGINT"); }
  else assert.equal(r.code, ["throw", "error-close"].includes(race) ? 1 : 37);
  if (race === "cleanup-uncertainty") assert.match(h.errors, /cleanup uncertainty/);
  assert.ok(h.errors.length < 600); assert.ok(kills.length <= 1);
  assert.equal(h.listenerCount("SIGINT"), 0); assert.equal(h.listenerCount("SIGTERM"), 0);
});
