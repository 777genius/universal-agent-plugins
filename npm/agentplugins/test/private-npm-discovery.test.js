"use strict";
const { test, before } = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const cp = require("node:child_process");
let root, env, preload, helper;
before(() => {
  root = fs.mkdtempSync(path.join(os.tmpdir(), "private-npm-discovery-"));
  const dir = name => { const p = path.join(root, name); fs.mkdirSync(p, { recursive: true, mode: 0o700 }); return p; };
  env = { PATH: process.env.PATH, HOME: dir("home"), TMPDIR: dir("tmp"),
    XDG_CONFIG_HOME: dir("config"), XDG_CACHE_HOME: dir("cache"), XDG_DATA_HOME: dir("data"), XDG_STATE_HOME: dir("state"),
    npm_config_userconfig: path.join(root, "user.conf"), npm_config_globalconfig: path.join(root, "global.conf"),
    npm_config_cache: dir("npm-cache"), npm_config_offline: "true", npm_config_ignore_scripts: "true" };
  fs.writeFileSync(env.npm_config_userconfig, ""); fs.writeFileSync(env.npm_config_globalconfig, "");
  // Minimal detached package: default discovery sees ONLY the exact helper.
  dir("test");
  for (const name of ["scripts/dual-authoring-candidate.js", "scripts/stage-dual-authoring-npm.js",
    "scripts/stage-dual-authoring-candidate.js", "scripts/npm-public-contract.js", "scripts/private-npm/bootstrap.js",
    "scripts/private-npm/launcher.js", "lib/verifier.js"]) {
    dir(path.dirname(name));
    fs.copyFileSync(path.join(__dirname, "..", name), path.join(root, name));
  }
  helper = path.join(root, "test", "private-npm-fixture.js");
  fs.copyFileSync(path.join(__dirname, "private-npm-fixture.js"), helper);
  dir("consumers");
  fs.copyFileSync(helper, path.join(root, "consumers", "private-npm-fixture.js"));
  for (const name of ["private-npm-pack.test.js", "private-npm-launcher.test.js"]) {
    // Non-test extension keeps these out of default discovery.
    fs.copyFileSync(path.join(__dirname, name), path.join(root, "consumers", name.replace(".test.js", ".cjs")));
  }
  preload = path.join(root, "absent-npm.cjs");
  // Same narrowly scoped IO fault as the independent review reproducer.
  // Platform overrides are Linux-hosted module fixtures, never native proof.
  fs.writeFileSync(preload, `const fs=require('node:fs');
if(process.env.DISCOVERY_PLATFORM)Object.defineProperty(process,'platform',{value:process.env.DISCOVERY_PLATFORM});
const realpath=fs.realpathSync;fs.realpathSync=function(p,...args){
if(p==='/usr/local/bin/npm'){const e=new Error('ENOENT: simulated absent npm');e.code='ENOENT';throw e;}
return realpath.call(this,p,...args);};\n`);
});
function invoke(args, extra = {}) {
  const r = cp.spawnSync(process.execPath, ["--require", preload, ...args], {
    cwd: root, env: { ...env, ...extra }, encoding: "utf8", timeout: 15000
  });
  assert.equal(r.error, undefined); assert.equal(r.status, 0, r.stdout + r.stderr); return r.stdout;
}
test("STRUCTURAL missing npm: helper import and default discovery are side-effect free", () => {
  invoke(["-e", `require(${JSON.stringify(helper)})`]);
  const out = invoke(["--test", "--test-reporter=tap"]);
  assert.deepEqual([...out.matchAll(/^ok \d+ - (.+)$/gm)].map(match => match[1].replaceAll("\\", "/")),
    ["test/private-npm-fixture.js"]);
  assert.match(out, /^# tests 1$/m);
  assert.match(out, /^# pass 1$/m);
  assert.match(out, /^# skipped 0$/m);
  assert.match(out, /# fail 0/);
});
test("STRUCTURAL missing npm: staged-child and unsupported platform skips precede tool resolution", () => {
  for (const extra of [{ AGENTPLUGINS_STAGED_TEST_CHILD: "1" },
    { DISCOVERY_PLATFORM: "win32" }, { DISCOVERY_PLATFORM: "darwin" }]) {
    const out = invoke(["--test", "--test-reporter=tap", "consumers/private-npm-pack.cjs", "consumers/private-npm-launcher.cjs"], extra);
    assert.match(out, /# fail 0/); assert.match(out, /# skipped 27/);
  }
});
test("STRUCTURAL enabled fixture rejects missing or invalid explicit npm before creating a workspace", () => {
  invoke(["-e", `const assert=require('node:assert/strict');
assert.throws(()=>require(${JSON.stringify(helper)}).fixture(),{code:'ENOENT'});`]);
  const out = invoke(["-e", `const fs=require('node:fs'),assert=require('node:assert/strict');
const f=require(${JSON.stringify(helper)});const before=fs.readdirSync(process.env.TMPDIR);
assert.throws(()=>f.fixture(),{code:'ENOENT'});
assert.throws(()=>f.resolveNpm('relative-npm'),/absolute path/);
assert.throws(()=>f.resolveNpm(process.env.HOME),/regular file/);
// Resolver shape control only: no pretend npm executable is run or provisioned.
assert.equal(f.resolveNpm(${JSON.stringify(helper)}),fs.realpathSync(${JSON.stringify(helper)}));
assert.deepEqual(fs.readdirSync(process.env.TMPDIR),before);console.log('rejected before setup');`],
    { UAP_PRIVATE_NPM_FIXTURE_NPM: path.join(root, "missing", "npm-cli.js") });
  assert.match(out, /rejected before setup/);
});

test("STRUCTURAL Linux IO model: unsupported bridge default discovery skips before fixture setup", {
  skip: process.platform !== "linux" && "Linux module/IO model only"
}, () => {
  const detached = fs.mkdtempSync(path.join(os.tmpdir(), "bridge-discovery-model-"));
  for (const name of ["packed-installer-bridge.test.js", "packed-installer-bridge.js", "dual-authoring-candidate.js"]) {
    fs.copyFileSync(path.join(__dirname, "../scripts", name), path.join(detached, name));
  }
  const guard = path.join(detached, "unsupported.cjs");
  fs.writeFileSync(guard, `const fs=require('node:fs');
Object.defineProperty(process,'platform',{value:process.env.DISCOVERY_PLATFORM});
for(const name of ['mkdtempSync','chmodSync','symlinkSync','linkSync']) {
  fs[name]=()=>{throw new Error('unsupported bridge fixture setup: '+name);};
}
`);
  for (const platform of ["win32", "darwin"]) {
    const r = cp.spawnSync(process.execPath, ["--require", guard, "--test", "--test-reporter=tap"], {
      cwd: detached, env: { ...env, DISCOVERY_PLATFORM: platform }, encoding: "utf8", timeout: 15000
    });
    assert.equal(r.error, undefined);
    assert.equal(r.status, 0, r.stdout + r.stderr);
    assert.match(r.stdout, /^# tests 2$/m);
    assert.match(r.stdout, /^# pass 0$/m);
    assert.match(r.stdout, /^# fail 0$/m);
    assert.match(r.stdout, /^# skipped 2$/m);
    assert.equal((r.stdout.match(/# SKIP Linux POSIX fixture only/g) || []).length, 2);
  }
});
