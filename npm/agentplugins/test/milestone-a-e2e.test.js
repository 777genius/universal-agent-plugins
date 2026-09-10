"use strict";
const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const harness = require("../scripts/milestone-a-e2e");

const repo = path.resolve(__dirname, "../../..");
const source = fs.readFileSync(path.join(repo, "npm/agentplugins/scripts/milestone-a-e2e.js"), "utf8");
const workflow = fs.readFileSync(path.join(repo, ".github/workflows/authoring-milestone-a-e2e.yml"), "utf8");

test("harness contract fixes exact candidate, entrypoints, order, isolation, and cleanup", () => {
  assert.deepEqual(harness.COMMANDS, ["init", "validate", "inspect", "test", "local-add-dry-run"]);
  assert.deepEqual(harness.TARGETS, { linux: ["amd64"], windows: ["amd64"], darwin: ["arm64"] });
  for (const token of ["checkout is not the expected exact candidate", "six-platform-pair",
    'PRODUCTS = ["agentplugins", "plugin-kit-ai"]', '`project-${product}`', '`client-${product}`',
    '"add", ".", "--target=codex", "--dry-run", "--format=json"',
    'registry_fallback: false', 'for (const name of ["work", "home", "tmp", "cache", "config"])']) assert.match(source, new RegExp(escape(token)));
  assert.doesNotMatch(source, /npm (view|install).*latest|https?:\/\/registry|npx|npm_config_registry/);
  assert.match(source, /timeout: 120000/);
});

test("workflow is secretless, pinned, bounded, PR/manual, and failure-preserving", () => {
  assert.match(workflow, /pull_request:/); assert.match(workflow, /workflow_dispatch:/);
  assert.match(workflow, /permissions:\n  contents: read/);
  assert.doesNotMatch(workflow, /secrets\.|permissions:\s*write|publish|npm-token|id-token/);
  assert.match(workflow, /ubuntu-24\.04[\s\S]*linux, arch: amd64/);
  assert.match(workflow, /windows-2022[\s\S]*windows, arch: amd64/);
  assert.match(workflow, /macos-14[\s\S]*darwin, arch: arm64/);
  assert.match(workflow, /if: always\(\)[\s\S]*actions\/upload-artifact@[0-9a-f]{40}/);
  assert.doesNotMatch(workflow, /uses:\s*[^\n]+@(?![0-9a-f]{40}(?:\s|$))/);
  for (const value of [...workflow.matchAll(/timeout-minutes:\s*(\d+)/g)].map(m => Number(m[1]))) assert.ok(value <= 20);
  assert.match(workflow, /NODE_OPTIONS: --max-old-space-size=384/);
});

test("tree digest is deterministic and rejects links", () => {
  fs.mkdirSync(os.tmpdir(), { recursive: true });
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "milestone-a-tree-"));
  try {
    fs.mkdirSync(path.join(root, "b")); fs.writeFileSync(path.join(root, "b", "z"), "z");
    fs.writeFileSync(path.join(root, "a"), "a");
    assert.deepEqual(harness.tree(root).map(row => row[0]), ["a", "b/z"]);
    fs.symlinkSync(path.join(root, "a"), path.join(root, "link"));
    assert.throws(() => harness.tree(root), /non-regular/);
  } finally { fs.rmSync(root, { recursive: true, force: true }); }
});

function escape(value) { return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&"); }
