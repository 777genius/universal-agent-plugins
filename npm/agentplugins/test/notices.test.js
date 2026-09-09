"use strict";

const assert = require("node:assert/strict");
const { execFileSync } = require("node:child_process");
const crypto = require("node:crypto");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const test = require("node:test");
const { expectedAssets, prepareRelease, verifyRelease } = require("../scripts/release-assets");

const packageRoot = path.resolve(__dirname, "..");
const noticeName = "THIRD_PARTY_NOTICES.txt";
const notices = fs.readFileSync(path.join(packageRoot, noticeName));
const digest = (body) => crypto.createHash("sha256").update(body).digest("hex");

function temporaryRoot(t) {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "agentplugins-notices-"));
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
  return root;
}

test("npm notice copy matches the canonical production notices", (t) => {
  const canonical = path.resolve(packageRoot, "../../cli/plugin-kit-ai", noticeName);
  if (!fs.existsSync(canonical)) return t.skip("detached npm package has no source tree");
  assert.deepEqual(notices, fs.readFileSync(canonical));
});

test("offline npm tarball includes complete notices even with scripts disabled", (t) => {
  const root = temporaryRoot(t);
  const staged = path.join(root, "package");
  fs.cpSync(packageRoot, staged, { recursive: true });
  const result = JSON.parse(execFileSync(process.platform === "win32" ? "npm.cmd" : "npm", [
    "pack", "--offline", "--ignore-scripts", "--json", "--pack-destination", root
  ], { cwd: staged, encoding: "utf8", shell: process.platform === "win32",
    env: { ...process.env, npm_config_cache: path.join(root, "cache") } }));
  assert.ok(result[0].files.some((entry) => entry.path === noticeName));
  const packed = execFileSync("tar", ["-xOf", path.join(root, result[0].filename), `package/${noticeName}`]);
  assert.deepEqual(packed, notices);
});

test("release packaging preserves all six binary bytes and verifies companion notices", (t) => {
  const root = temporaryRoot(t);
  const version = "0.1.99";
  const tag = `agentplugins-v${version}`;
  const commit = "a".repeat(40);
  const bodies = Object.fromEntries(Object.values(expectedAssets(version)).map((file) => [file, Buffer.from(`fixture-${file}\n`)]));
  for (const [file, body] of Object.entries(bodies)) fs.writeFileSync(path.join(root, file), body);
  const manifest = prepareRelease(root, tag, commit);
  assert.equal(manifest.schema_version, 2);
  assert.equal(Object.keys(manifest.assets).length, 6);
  for (const [file, body] of Object.entries(bodies)) assert.deepEqual(fs.readFileSync(path.join(root, file)), body);
  assert.deepEqual(fs.readFileSync(path.join(root, noticeName)), notices);
  assert.deepEqual(verifyRelease(root, tag, commit).notices, [{ file: noticeName, sha256: digest(notices) }]);
  fs.appendFileSync(path.join(root, noticeName), "tampered");
  assert.throws(() => verifyRelease(root, tag, commit), /checksum mismatch/);
  fs.unlinkSync(path.join(root, noticeName));
  assert.throws(() => verifyRelease(root, tag, commit), /checksums.txt/);
  // Historical schema-v2 releases retain their original eight-file contract.
  const checksums = path.join(root, "checksums.txt");
  fs.writeFileSync(checksums, fs.readFileSync(checksums, "utf8").split("\n").filter((line) => !line.endsWith(`  ${noticeName}`)).join("\n"));
  assert.deepEqual(verifyRelease(root, tag, commit).notices, []);
});
