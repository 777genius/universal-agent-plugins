"use strict";
const assert = require("node:assert/strict");
const { EventEmitter } = require("node:events");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");

const packageRoot = path.join(process.env.UAP_GLOBAL_NPM_ROOT, "universal-agent-plugins");
const pkg = JSON.parse(fs.readFileSync(path.join(packageRoot, "package.json"), "utf8"));
assert.equal(pkg.version, "0.1.102", "must test the exact public npm package");
const { ensureBinary } = require(path.join(packageRoot, "lib/bootstrap.js"));
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

(async () => {
  try {
    await abortOnce();
    await abortOnce(); // Verify reacquisition of the same cache after cleanup.
    console.log("Published npm 0.1.102 abort/cache-lock cleanup: PASS (2 attempts)");
  } finally {
    fs.rmSync(cacheRoot, { recursive: true, force: true });
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
