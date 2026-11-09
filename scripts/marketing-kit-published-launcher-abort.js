"use strict";
const assert = require("node:assert/strict");
const { EventEmitter } = require("node:events");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");

const packageRoot = path.join(process.env.UAP_GLOBAL_NPM_ROOT, "universal-agent-plugins");
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
    const controller = new AbortController();
    const waiter = acquireLock(target, { ...options, signal: controller.signal });
    timer = setTimeout(() => controller.abort(), 50);
    await assert.rejects(waiter, /cancel/i, "published lock waiter must honor AbortSignal");
    assert.equal(fs.readdirSync(lockRoot).filter(name => name.endsWith(".lock")).length, 1,
      "cancelled waiter must not remove the original owner's lock");
  } finally {
    clearTimeout(timer);
    await releaseOwner();
  }
  assert.equal(fs.readdirSync(lockRoot).filter(name => name.endsWith(".lock")).length, 0,
    "owner release must remove its own cache lock");
  const releaseAgain = await acquireLock(target, options);
  await releaseAgain();
  assert.equal(fs.readdirSync(lockRoot).filter(name => name.endsWith(".lock")).length, 0,
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
