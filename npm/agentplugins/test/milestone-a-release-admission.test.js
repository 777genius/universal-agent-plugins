"use strict";

const assert = require("node:assert/strict");
const test = require("node:test");
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
