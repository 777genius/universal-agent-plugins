"use strict";

const assert = require("node:assert/strict");
const cp = require("node:child_process");
const test = require("node:test");
const os = require("node:os");
const fs = require("node:fs");
const path = require("node:path");
const a = require("../scripts/milestone-a-release-admission");
const candidate = require("../scripts/dual-authoring-candidate");
const promotion = require("../scripts/authoring-promotion");
const workflowPath = path.resolve(__dirname, "../../../.github/workflows/agentplugins-release.yml");

const source = "a".repeat(40);
const selected = { tag: a.TAG, ref: `refs/tags/${a.TAG}`, source,
  versions: { agentplugins: "0.1.60", "plugin-kit-ai": "2.0.0" } };
const pin = { run_id: 42, run_attempt: 2 };
const run = { id: 42, run_attempt: 2, repository: { full_name: a.REPOSITORY },
  head_repository: { full_name: a.REPOSITORY }, head_sha: source, path: a.WORKFLOW,
  event: "workflow_dispatch", head_branch: a.TAG, status: "completed", conclusion: "success" };
const jobs = () => a.JOBS.map((name, index) => ({ id: 100 + index, name, run_id: 42, run_attempt: 2,
  head_sha: source, head_branch: a.TAG, status: "completed", conclusion: "success" }));

function promotionRecord() {
  let binary = 1;
  const products = Object.fromEntries(candidate.PRODUCTS.map(product => [product, {
    tag: product === "agentplugins" ? a.TAG : a.KIT_TAG,
    manifest_sha256: product === "agentplugins" ? "d".repeat(64) : "e".repeat(64),
    checksums_sha256: product === "agentplugins" ? "f".repeat(64) : "1".repeat(64),
    assets: Object.fromEntries(candidate.TARGETS.map(target => {
      const digest = (++binary).toString(16).repeat(64);
      const size = 1000 + binary;
      const binaryPin = { file: candidate.executableName(product, target), sha256: digest, size };
      return [target, { file: candidate.assetName(product, selected.versions[product], target),
        sha256: digest,
        size: product === "agentplugins" ? size : size + 20, binary: binaryPin }];
    }))
  }]));
  return { schema: a.RECORD_SCHEMA,
    identity: { repository: a.REPOSITORY, commit: source, engine_revision: source, versions: selected.versions },
    authoring_mode: "release-cli-contract-v1", asset_scope: "six-platform-pair",
    candidate_sha256: "a".repeat(64), pair_marker_sha256: "b".repeat(64), products,
    preparation: { run_id: 41, run_attempt: 1, artifact_id: 91,
      artifact_sha256: "c".repeat(64), receipt_sha256: "2".repeat(64) },
    milestone_a: { workflow: a.WORKFLOW, run_id: pin.run_id, run_attempt: pin.run_attempt },
    signer: { workflow: a.RELEASE_WORKFLOW, source } };
}

test("uses a canonical Milestone A record without legacy qualification lanes", () => {
  const record = promotionRecord();
  const body = a.encodeRecord(record);
  assert.deepEqual(a.decodeRecord(body), record);
  assert.deepEqual(a.validateSelection(body, selected), record);
  assert.equal(promotion.promotionRecord(record).name, a.RECORD_FILE);
  assert.equal(promotion.releasePins(record, "agentplugins").at(-1).name, a.RECORD_FILE);
  assert.equal(Object.hasOwn(record, "qualification"), false);
});

test("rejects legacy or invented qualification content on the Milestone A route", () => {
  const record = promotionRecord();
  record.qualification = { lanes: [] };
  assert.throws(() => a.encodeRecord(record), /Milestone A promotion record/);
  const legacy = { ...record, schema: promotion.SCHEMA };
  delete legacy.qualification;
  assert.throws(() => a.encodeRecord(legacy));
});

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

test("Milestone A admission and promotion embedded Node programs parse", t => {
  if (!fs.existsSync(workflowPath)) return t.skip("detached npm package has no source workflow");
  const workflow = fs.readFileSync(workflowPath, "utf8");
  for (const name of ["Independently admit preparation and authenticated Milestone A evidence",
    "Reacquire and re-admit the exact frozen pair and Milestone A run"]) {
    const start = workflow.indexOf(`      - name: ${name}\n`);
    assert.notEqual(start, -1, `missing workflow step ${name}`);
    const block = workflow.slice(start).match(/          node <<'NODE'\n([\s\S]*?)\n          NODE/);
    assert.ok(block, `missing embedded Node program for ${name}`);
    assert.doesNotThrow(() => new Function(block[1].split("\n").map(line => line.slice(10)).join("\n")), name);
  }
});
