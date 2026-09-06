"use strict";

const assert = require("node:assert/strict");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const cp = require("node:child_process");
const test = require("node:test");
const c = require("../scripts/dual-authoring-candidate");
const producer = require("../scripts/stage-dual-authoring-candidate");

// Structural controlled-builder fixtures only: Go env/build/inspection are
// stubbed. Git snapshot, exclusive creation, writes and cleanup are real.
// Actual Linux bytes and native journeys are separate opt-in evidence.
function fixture(t, mode) {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "candidate-marker-"));
  const repo = path.resolve(__dirname, "../../..");
  const workParent = path.join(root, "work");
  const modCache = path.join(root, "modules");
  fs.mkdirSync(workParent); fs.mkdirSync(modCache);
  const context = producer.privateContext(workParent);
  const commit = cp.execFileSync("/usr/bin/git", ["rev-parse", "HEAD"], { cwd: repo, env: context.env, encoding: "utf8" }).trim();
  const identity = { repository: c.REPOSITORY, commit, engine_revision: commit,
    versions: { agentplugins: "0.1.23", "plugin-kit-ai": "2.0.0" } };
  const go = path.join(root, "structural-go");
  fs.writeFileSync(go, "structural tool fixture, never executed");
  const output = path.join(root, "candidate");
  const marker = path.join(output, "candidate.json");
  const options = { candidate: true, repo, workParent, modCache, go, output, identity, assetScope: "linux-amd64-pair" };
  if (mode) {
    options.authoringMode = mode;
    // Structural source-capability response, like the fake compiler below;
    // this is not a real source or native candidate proof.
    const read = c.readFile;
    t.mock.method(c, "readFile", function(file, ...rest) {
      if (file.endsWith("/source/cli/plugin-kit-ai/internal/authoring/commands/commands.go"))
        return Buffer.from('const ReleaseMode = "release-cli-contract-v1"\n');
      return read(file, ...rest);
    });
  }
  // Keep the trusted tool directory disjoint from candidate output.
  const tools = path.join(root, "tools"); fs.mkdirSync(tools);
  fs.renameSync(go, path.join(tools, "go")); options.go = path.join(tools, "go");
  const exec = cp.execFileSync;
  t.mock.method(cp, "execFileSync", function (command, args, opts) {
    if (command !== options.go) return exec.apply(this, arguments);
    if (args[0] === "env") return JSON.stringify({ GOVERSION: "go1.25.13", GOHOSTOS: "linux", GOHOSTARCH: "amd64" });
    if (args[0] === "build") {
      const product = args.at(-1).split("/").at(-1);
      assert.equal(args[args.indexOf("-ldflags") + 1], c.linkerFlags(product, identity, mode));
      fs.writeFileSync(args[args.indexOf("-o") + 1], `STRUCTURAL ONLY: ${product}`);
      return Buffer.alloc(0);
    }
    assert.equal(args[0], "version");
    const product = fs.readFileSync(args.at(-1), "utf8").replace("STRUCTURAL ONLY: ", "");
    assert.ok(c.PRODUCTS.includes(product));
    return JSON.stringify({ GoVersion: "go1.25.13", Path: `github.com/777genius/plugin-kit-ai/cli/cmd/${product}`,
      Settings: Object.entries({ GOOS: "linux", GOARCH: "amd64", CGO_ENABLED: "0", "-buildmode": "exe", "-compiler": "gc",
        "-ldflags": c.linkerFlags(product, identity, mode) }).map(([Key, Value]) => ({ Key, Value })) });
  });
  return { options, marker, output };
}

const faults = ["marker-chmod", "partial-write", "complete-write", "close", "write-and-close",
  "directory-chmod", "directory-chmod-after-change", "exclusive-collision", "cleanup-unlink", "cleanup-after-partial-write", "cleanup-permission-retry", "cleanup-permission-failure"];
for (const fault of faults) test(`owned candidate marker failure: ${fault}`, {
  skip: process.env.AGENTPLUGINS_STAGED_TEST_CHILD === "1"
}, (t) => {
  const { options, marker, output } = fixture(t);
  const original = Object.fromEntries(["openSync", "writeFileSync", "closeSync", "chmodSync", "unlinkSync"].map(name => [name, fs[name]]));
  const primary = Object.assign(new Error(`injected ${fault}`), { code: "EIO" });
  const cleanup = Object.assign(new Error("injected marker cleanup failure"), { code: "EACCES" });
  const closeError = Object.assign(new Error("injected secondary close failure"), { code: "EIO" });
  let markerFd, injected = false, written = false, closeInjected = false, unlinkCalls = 0, restored = false;
  const unrelated = Buffer.from("unrelated marker must survive");
  const collide = () => {
    const fd = original.openSync(marker, "wx", 0o640);
    try { original.writeFileSync(fd, unrelated); } finally { original.closeSync(fd); }
    injected = true;
  };
  t.mock.method(fs, "openSync", function (file, flags) {
    if (file === marker && flags === "wx" && fault === "exclusive-collision" && !injected) collide();
    const fd = original.openSync.apply(this, arguments);
    if (file === marker && flags === "wx") markerFd = fd;
    return fd;
  });
  t.mock.method(fs, "writeFileSync", function (file, body, opts) {
    if (file === marker || (markerFd !== undefined && file === markerFd)) {
      // The path branch also reproduces the pre-fix writeFileSync(..., 'wx').
      if (fault === "exclusive-collision" && !injected) collide();
      if (["partial-write", "write-and-close", "cleanup-after-partial-write"].includes(fault)) {
        original.writeFileSync(file, body.subarray(0, 31), opts);
        assert.equal(fs.statSync(marker).size, 31);
        injected = true; throw primary;
      }
      const result = original.writeFileSync.apply(this, arguments);
      c.manifestShape(JSON.parse(fs.readFileSync(marker)), options.identity, options.assetScope);
      written = true;
      if (fault === "complete-write") { injected = true; throw primary; }
      return result;
    }
    return original.writeFileSync.apply(this, arguments);
  });
  t.mock.method(fs, "closeSync", function (fd) {
    const result = original.closeSync.apply(this, arguments);
    if (fd === markerFd && !closeInjected && ["close", "write-and-close"].includes(fault)) {
      // Close for real before throwing; do not leak the fixture descriptor.
      closeInjected = true; injected = true;
      throw fault === "close" ? primary : closeError;
    }
    return result;
  });
  t.mock.method(fs, "chmodSync", function (file, mode) {
    if (file === marker && ["marker-chmod", "cleanup-unlink", "cleanup-permission-failure"].includes(fault)) {
      assert.equal(written, true); injected = true; throw primary;
    }
    if (file === output && mode === 0o555 && fault.startsWith("directory-chmod")) {
      assert.equal(written, true);
      if (fault === "directory-chmod-after-change") original.chmodSync(file, mode);
      injected = true; throw primary;
    }
    if (file === output && mode === 0o555 && fault === "cleanup-permission-retry") {
      original.chmodSync(file, mode); injected = true; throw primary;
    }
    if (file === output && mode === 0o700) restored = true;
    return original.chmodSync.apply(this, arguments);
  });
  t.mock.method(fs, "unlinkSync", function (file) {
    if (file === marker) {
      unlinkCalls++;
      if (["cleanup-unlink", "cleanup-after-partial-write"].includes(fault)) throw Object.assign(cleanup, { code: "EIO" });
      if (fault === "cleanup-permission-failure" || (fault === "cleanup-permission-retry" && unlinkCalls === 1)) throw cleanup;
    }
    return original.unlinkSync.apply(this, arguments);
  });
  let failure;
  try { producer.stageCandidate(options); } catch (error) { failure = error; }
  assert.equal(injected, true, "must reach the requested finalization fault");
  if (fault === "exclusive-collision") {
    assert.equal(failure.code, "EEXIST");
    assert.deepEqual(fs.readFileSync(marker), unrelated);
    assert.equal(fs.statSync(marker).mode & 0o777, 0o640);
    assert.equal(unlinkCalls, 0);
  } else if (["cleanup-unlink", "cleanup-after-partial-write", "cleanup-permission-failure"].includes(fault)) {
    assert.ok(failure instanceof AggregateError);
    assert.equal(failure.cause, primary);
    assert.deepEqual(failure.errors, [primary, cleanup]);
    assert.match(failure.message, /marker cleanup failed/);
    assert.equal(fs.existsSync(marker), true, "failed cleanup is reported, not hidden");
  } else {
    assert.equal(failure, primary, "successful cleanup preserves the original error object/code/message");
    assert.equal(fs.existsSync(marker), false);
    assert.equal(fs.readdirSync(output).length, 2, "retain partial assets for inspection");
    if (fault === "write-and-close") assert.equal(failure.closeError, closeError);
    if (fault === "cleanup-permission-retry") { assert.equal(unlinkCalls, 2); assert.equal(restored, true); }
  }
});

test("successful structural candidate still finalizes and verifies", {
  skip: process.env.AGENTPLUGINS_STAGED_TEST_CHILD === "1"
}, (t) => {
  const { options, marker, output } = fixture(t);
  const result = producer.stageCandidate(options);
  assert.equal(fs.statSync(marker).mode & 0o777, 0o444);
  assert.equal(fs.statSync(output).mode & 0o777, 0o555);
  assert.equal(producer.verifyCandidate({ candidate: true, root: output, identity: options.identity,
    manifestDigest: result.manifest_sha256, go: options.go, workParent: options.workParent,
    assetScope: options.assetScope }).consistency_verified, true);
});


test("release mode producer and verifier require matching explicit intent", {
  // Like the sibling producer fixtures, this requires the repository Git snapshot.
  skip: process.env.AGENTPLUGINS_STAGED_TEST_CHILD === "1"
}, (t) => {
  const mode = "release-cli-contract-v1";
  const { options, output, marker } = fixture(t, mode);
  const staged = producer.stageCandidate(options);
  assert.equal(JSON.parse(fs.readFileSync(marker)).build.authoring_mode, mode);
  const verify = { candidate: true, root: output, identity: options.identity,
    manifestDigest: staged.manifest_sha256, go: options.go, workParent: options.workParent, assetScope: options.assetScope };
  assert.throws(() => producer.verifyCandidate(verify), /build description/);
  assert.equal(producer.verifyCandidate({ ...verify, authoringMode: mode }).consistency_verified, true);
});
