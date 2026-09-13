"use strict";

const assert = require("node:assert/strict");
const cp = require("node:child_process");
const test = require("node:test");
const os = require("node:os");
const fs = require("node:fs");
const a = require("../scripts/milestone-a-release-admission");

const source = "a".repeat(40);
const selected = { tag: a.TAG, ref: `refs/tags/${a.TAG}`, source,
  versions: { agentplugins: "0.1.60", "plugin-kit-ai": "2.0.0" } };
const pin = { run_id: 42, run_attempt: 2 };
const run = { id: 42, run_attempt: 2, repository: { full_name: a.REPOSITORY },
  head_repository: { full_name: a.REPOSITORY }, head_sha: source, path: a.WORKFLOW,
  event: "workflow_dispatch", head_branch: a.TAG, status: "completed", conclusion: "success" };
const jobs = () => a.JOBS.map((name, index) => ({ id: 100 + index, name, run_id: 42, run_attempt: 2,
  head_sha: source, head_branch: a.TAG, status: "completed", conclusion: "success" }));

test("admits exactly the accelerated Milestone A run and three-platform job closure", () => {
  const result = a.evidenceContract(structuredClone(run), jobs(), structuredClone(selected), pin);
  assert.deepEqual(result.jobs, a.JOBS);
  assert.equal(result.source, source);
});

const prepareReceipt = () => ({ schema: "milestone-a-e2e-prepare/v1",
  identity: { repository: a.REPOSITORY, commit: source, engine_revision: source, versions: { ...a.E2E_VERSIONS } },
  candidate_head: source, publication: false });
const runReceipts = () => a.PLATFORMS.map(([platform, arch]) => ({ schema: "milestone-a-e2e-run/v1",
  platform, arch, exact_candidate: true, entrypoints: ["agentplugins", "plugin-kit-ai"],
  commands: platform === "linux" ? ["init", "validate", "inspect", "test", "local-add-dry-run"] : ["init", "validate"],
  cleanup: "complete", clean_root_separation: true,
  provenances: { agentplugins: { revision: source }, "plugin-kit-ai": { revision: source } } }));

test("binds behavioral receipts to the source and synthetic 0.1.91 test package without claiming release 0.1.60", () => {
  const result = a.receiptContract(prepareReceipt(), runReceipts(), source);
  assert.deepEqual(result.test_versions, { agentplugins: "0.1.91", "plugin-kit-ai": "2.0.0" });
  assert.deepEqual(result.release_versions, { agentplugins: "0.1.60", "plugin-kit-ai": "2.0.0" });
  assert.equal(result.engine_revision, source);
});

function artifactFixture(attempt) {
  const labels = ["exact-candidate", ...a.PLATFORMS.map(value => value.join("-"))];
  return labels.map((label, index) => ({ id: attempt * 100 + index + 1,
    name: `milestone-a-${label}-${pin.run_id}-${attempt}`, expired: false,
    workflow_run: { id: pin.run_id, head_sha: source }, digest: `sha256:${String(attempt).repeat(64)}` }));
}

function withMockedReceiptProvider(t, artifacts, operation) {
  const promotion = require("../scripts/authoring-promotion");
  const bodies = new Map();
  artifactFixture(2).forEach((item, index) => bodies.set(item.id,
    index === 0 ? prepareReceipt() : runReceipts()[index - 1]));
  t.mock.method(promotion, "acquireArtifact", locator => `/provider/artifact-${locator.artifact_id}.zip`);
  t.mock.method(cp, "spawnSync", (command, args) => {
    if (command === "/usr/bin/gh") return { status: 0, signal: null, stdout: JSON.stringify({ total_count: artifacts.length, artifacts }) };
    if (command === "/usr/bin/python3") {
      const id = Number(/artifact-(\d+)\.zip$/.exec(args[3])?.[1]);
      return { status: 0, signal: null, stdout: JSON.stringify(bodies.get(id)) };
    }
    throw new Error(`unexpected mocked command: ${command}`);
  });
  const scratch = fs.mkdtempSync(`${os.tmpdir()}/milestone-a-provider-fixture-`);
  try { return operation(scratch); } finally { fs.rmSync(scratch, { recursive: true, force: true }); }
}

test("selects attempt 2 receipts while attempt 1 artifacts remain retained", t => {
  const artifacts = [...artifactFixture(1), ...artifactFixture(2)];
  const result = withMockedReceiptProvider(t, artifacts, scratch => a.inspectReceipts(pin, selected, scratch));
  assert.equal(result.source, source);
  assert.deepEqual(result.test_versions, a.E2E_VERSIONS);
});

test("rejects malformed or ambiguous selected-attempt artifact closure", async t => {
  await t.test("malformed selected name leaves the canonical receipt absent", child => {
    const artifacts = [...artifactFixture(1), ...artifactFixture(2)];
    artifacts.at(-1).name += "-malformed";
    assert.throws(() => withMockedReceiptProvider(child, artifacts,
      scratch => a.inspectReceipts(pin, selected, scratch)), /exact Milestone A artifact closure required/);
  });
  await t.test("duplicate selected canonical name is ambiguous", child => {
    const artifacts = [...artifactFixture(1), ...artifactFixture(2)];
    artifacts.push({ ...artifacts.at(-1), id: 999 });
    assert.throws(() => withMockedReceiptProvider(child, artifacts,
      scratch => a.inspectReceipts(pin, selected, scratch)), /exact retained Milestone A artifact required/);
  });
});

test("requires the authentic ordered Linux command receipt", () => {
  const authentic = runReceipts();
  assert.doesNotThrow(() => a.receiptContract(prepareReceipt(), authentic, source));

  const omitted = runReceipts();
  omitted[0].commands.pop();
  assert.throws(() => a.receiptContract(prepareReceipt(), omitted, source), /linux-amd64 receipt/);

  const reordered = runReceipts();
  [reordered[0].commands[3], reordered[0].commands[4]] =
    [reordered[0].commands[4], reordered[0].commands[3]];
  assert.throws(() => a.receiptContract(prepareReceipt(), reordered, source), /linux-amd64 receipt/);
});

for (const [name, mutate] of [
  ["synthetic package presented as 0.1.60", value => { value.prepare.identity.versions.agentplugins = "0.1.60"; }],
  ["engine revision differs", value => { value.prepare.identity.engine_revision = "b".repeat(40); }],
  ["entrypoint omitted", value => { value.runs[1].entrypoints.pop(); }],
  ["Windows runs Linux-only commands", value => { value.runs[1].commands.push("inspect", "test"); }],
  ["launcher provenance differs", value => { value.runs[2].provenances.agentplugins.revision = "b".repeat(40); }],
  ["cleanup incomplete", value => { value.runs[0].cleanup = "pending"; }]
]) test(`rejects receipt contract when ${name}`, () => {
  const value = { prepare: prepareReceipt(), runs: runReceipts() };
  mutate(value);
  assert.throws(() => a.receiptContract(value.prepare, value.runs, source));
});

for (const [name, mutate] of [
  ["wrong source", value => { value.run.head_sha = "b".repeat(40); }],
  ["pull request run", value => { value.run.event = "pull_request"; }],
  ["wrong ref", value => { value.run.head_branch = "main"; }],
  ["failed Linux E2E", value => { value.jobs[1].conclusion = "failure"; }],
  ["missing Windows smoke", value => { value.jobs.splice(2, 1); }],
  ["extra matrix lane", value => { value.jobs.push({ ...value.jobs[0], id: 999, name: "Milestone A / linux-arm64" }); }],
  ["stale agentplugins version", value => { value.selected.versions.agentplugins = "0.1.59"; }],
  ["stale plugin-kit tag", value => { value.selected.versions["plugin-kit-ai"] = "2.0.1"; }]
]) test(`rejects ${name}`, () => {
  const value = { run: structuredClone(run), jobs: jobs(), selected: structuredClone(selected) };
  mutate(value);
  assert.throws(() => a.evidenceContract(value.run, value.jobs, value.selected, pin));
});

test("legacy thirteen-lane admission remains present and separate", () => {
  const promotion = require("../scripts/authoring-promotion");
  assert.equal(promotion.LANES.length, 13);
  assert.throws(() => promotion.requireNativeContracts([]), /missing lanes/);
});
