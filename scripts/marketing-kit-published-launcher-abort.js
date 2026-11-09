"use strict";
const assert = require("node:assert/strict");
const { EventEmitter } = require("node:events");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");

const npmRoot = process.env.UAP_GLOBAL_NPM_ROOT;
assert.ok(npmRoot, "UAP_GLOBAL_NPM_ROOT must point to the global npm root");
const packageRoot = path.join(npmRoot, "universal-agent-plugins");
const pkg = JSON.parse(fs.readFileSync(path.join(packageRoot, "package.json"), "utf8"));
assert.equal(pkg.version, "0.1.102", "must test the exact public npm package");
const { ensureBinary, acquireLock } = require(path.join(packageRoot, "lib/bootstrap.js"));
const cacheRoot = fs.mkdtempSync(path.join(os.tmpdir(), "uap-published-abort-"));

async function abortOnce() {
  const controller = new AbortController();
  let requests = 0;
  let destroyed = 0;
  await assert.rejects(ensureBinary({
    packageRoot,
    cacheRoot,
    signal: controller.signal,
    request: () => {
      requests += 1;
      const request = new EventEmitter();
      request.setTimeout = () => request;
      request.destroy = error => {
        destroyed += 1;
        queueMicrotask(() => request.emit("error", error));
        return request;
      };
      setTimeout(() => controller.abort(), 100);
      return request;
    }
  }), /cancel|abort/i, "stalled published acquisition must reject on AbortSignal");
  assert.equal(requests, 1, "must exercise the published downloader");
  assert.equal(destroyed, 1, "abort must destroy the pending request");
  assert.equal(controller.signal.aborted, true);

  function visit(dir) {
    for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
      assert.ok(!entry.name.endsWith(".lock") && !entry.name.startsWith(".download-"),
        `published cache retains acquisition state: ${entry.name}`);
      const candidate = path.join(dir, entry.name);
      if (entry.isDirectory()) visit(candidate);
      else assert.fail(`aborted acquisition retained a cache file: ${candidate}`);
    }
  }
  visit(cacheRoot);
}


async function verifyLockWaitCancellation() {
  // Aborting an HTTP download happens before the binary cache lock is taken.
  // Exercise the actual published lock primitive separately rather than
  // incorrectly treating downloader cancellation as lock-cleanup evidence.
  const lockRoot = path.join(cacheRoot, ".locks");
  const target = path.join(cacheRoot, "locked-binary");
  const options = { lockRoot, pollMs: 10, timeoutMs: 2_000 };
  const releaseOwner = await acquireLock(target, options);
  let timer;
  try {
    // A lock *count* alone could pass if a cancelled waiter destroyed the
    // owner's file and created a replacement. Preserve its exact identity.
    const ownerNames = fs.readdirSync(lockRoot);
    assert.equal(ownerNames.length, 1, "owner must hold exactly one published cache lock");
    assert.ok(ownerNames[0].endsWith(".lock"), "expected the published lock filename");
    const ownerPath = path.join(lockRoot, ownerNames[0]);
    const ownerStat = fs.lstatSync(ownerPath, { bigint: true });
    assert.ok(ownerStat.isFile() && !ownerStat.isSymbolicLink(),
      "published owner lock must be a regular non-symlink file");
    const ownerBytes = fs.readFileSync(ownerPath);
    const controller = new AbortController();
    const waiter = acquireLock(target, { ...options, signal: controller.signal });
    timer = setTimeout(() => controller.abort(), 50);
    await assert.rejects(waiter, /cancel/i, "published lock waiter must honor AbortSignal");
    assert.deepEqual(fs.readdirSync(lockRoot), ownerNames,
      "cancelled waiter must not replace the owner's lock or leave orphan files");
    const surviving = fs.lstatSync(ownerPath, { bigint: true });
    assert.equal(surviving.dev, ownerStat.dev, "cancelled waiter replaced owner's lock device");
    assert.equal(surviving.ino, ownerStat.ino, "cancelled waiter replaced owner's lock inode");
    assert.deepEqual(fs.readFileSync(ownerPath), ownerBytes,
      "cancelled waiter modified owner's lock nonce or receipt");
  } finally {
    clearTimeout(timer);
    await releaseOwner();
  }
  assert.deepEqual(fs.readdirSync(lockRoot), [],
    "owner release must remove its own cache lock without leftovers");
  const releaseAgain = await acquireLock(target, options);
  await releaseAgain();
  assert.deepEqual(fs.readdirSync(lockRoot), [],
    "released published lock must be reacquirable and removable");
}

(async () => {
  try {
    await abortOnce();
    await abortOnce(); // Two real published downloader aborts against a stalled request.
    await verifyLockWaitCancellation();
    console.log("Published npm 0.1.102 abort/lock cleanup: PASS (2 downloads; lock wait/reacquire)");
  } finally {
    fs.rmSync(cacheRoot, { recursive: true, force: true });
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
