#!/usr/bin/env node
"use strict";

// Narrow release admission for the accelerated Milestone A cut.  The older
// thirteen-lane/eighteen-cell promotion contract remains implemented by
// authoring-promotion.js and is deliberately not interpreted here.
const cp = require("node:child_process");
const fs = require("node:fs");
const path = require("node:path");
const { isDeepStrictEqual } = require("node:util");
const c = require("./dual-authoring-candidate");

const REPOSITORY = c.REPOSITORY;
const WORKFLOW = ".github/workflows/authoring-milestone-a-e2e.yml";
const RELEASE_WORKFLOW = ".github/workflows/agentplugins-release.yml";
const RECORD_SCHEMA = "milestone-a-promotion/v1";
const RECORD_FILE = "milestone-a-promotion.json";
const TAG = "agentplugins-v0.1.60";
const KIT_TAG = "v2.0.0";
const GH = "/usr/bin/gh";
const GH_VERSION = "2.83.2";
const JOBS = Object.freeze([
  "Exact candidate package",
  "Milestone A / linux-amd64",
  "Milestone A / windows-amd64",
  "Milestone A / darwin-arm64"
]);
const E2E_VERSIONS = Object.freeze({ agentplugins: "0.1.91", "plugin-kit-ai": "2.0.0" });
const PLATFORMS = Object.freeze([
  ["linux", "amd64"], ["windows", "amd64"], ["darwin", "arm64"]
]);
const fail = message => { throw new Error(message); };
const exact = (actual, expected, label) => {
  if (!isDeepStrictEqual(actual, expected)) fail(`${label}: binding mismatch`);
};
const positive = (value, max = Number.MAX_SAFE_INTEGER) => {
  if (!Number.isSafeInteger(value) || value < 1 || value > max) fail("bounded positive integer required");
  return value;
};
const sha = value => {
  if (typeof value !== "string" || !/^[0-9a-f]{40}$/.test(value) || /^0+$/.test(value)) fail("exact nonzero source SHA required");
  return value;
};
const hash = value => {
  if (typeof value !== "string" || !/^[0-9a-f]{64}$/.test(value) || /^0+$/.test(value)) fail("exact nonzero SHA-256 required");
  return value;
};
const artifact = value => {
  c.keys(value, ["run_id", "run_attempt", "artifact_id", "artifact_sha256", "receipt_sha256"], "Milestone A preparation pin");
  return { run_id: positive(value.run_id), run_attempt: positive(value.run_attempt, 1000),
    artifact_id: positive(value.artifact_id), artifact_sha256: hash(value.artifact_sha256),
    receipt_sha256: hash(value.receipt_sha256) };
};
function recordShape(value) {
  c.keys(value, ["schema", "identity", "authoring_mode", "asset_scope", "candidate_sha256",
    "pair_marker_sha256", "products", "preparation", "milestone_a", "signer"], "Milestone A promotion record");
  exact([value.schema, value.authoring_mode, value.asset_scope],
    [RECORD_SCHEMA, "release-cli-contract-v1", "six-platform-pair"], "Milestone A record contract");
  const identity = c.identity(value.identity);
  sha(identity.commit);
  exact(identity, { repository: REPOSITORY, commit: identity.commit, engine_revision: identity.commit,
    versions: { agentplugins: "0.1.60", "plugin-kit-ai": "2.0.0" } }, "Milestone A record identity");
  c.keys(value.products, c.PRODUCTS, "Milestone A products");
  const products = {};
  const binaries = new Set();
  for (const product of c.PRODUCTS) {
    const source = value.products[product];
    c.keys(source, ["tag", "manifest_sha256", "checksums_sha256", "assets"], "Milestone A product");
    exact(source.tag, product === "agentplugins" ? "agentplugins-v0.1.60" : "v2.0.0", "Milestone A product tag");
    c.keys(source.assets, c.TARGETS, "Milestone A product assets");
    const assets = {};
    for (const target of c.TARGETS) {
      const outer = source.assets[target];
      c.keys(outer, ["file", "sha256", "size", "binary"], "Milestone A asset");
      c.keys(outer.binary, ["file", "sha256", "size"], "Milestone A binary");
      exact(outer.file, c.assetName(product, identity.versions[product], target), "Milestone A asset name");
      exact(outer.binary.file, c.executableName(product, target), "Milestone A binary name");
      const assetValue = { file: outer.file, sha256: hash(outer.sha256), size: positive(outer.size, 128 * 1024 * 1024),
        binary: { file: outer.binary.file, sha256: hash(outer.binary.sha256), size: positive(outer.binary.size, 128 * 1024 * 1024) } };
      if (product === "agentplugins") exact([assetValue.sha256, assetValue.size],
        [assetValue.binary.sha256, assetValue.binary.size], "Milestone A raw executable");
      if (binaries.has(assetValue.binary.sha256)) fail("distinct Milestone A binaries required");
      binaries.add(assetValue.binary.sha256);
      assets[target] = assetValue;
    }
    products[product] = { tag: source.tag, manifest_sha256: hash(source.manifest_sha256),
      checksums_sha256: hash(source.checksums_sha256), assets };
  }
  const preparation = artifact(value.preparation);
  c.keys(value.milestone_a, ["workflow", "run_id", "run_attempt"], "Milestone A evidence pin");
  const milestone_a = { workflow: value.milestone_a.workflow, run_id: positive(value.milestone_a.run_id),
    run_attempt: positive(value.milestone_a.run_attempt, 1000) };
  exact(milestone_a.workflow, WORKFLOW, "Milestone A evidence workflow");
  c.keys(value.signer, ["workflow", "source"], "Milestone A signer");
  const signer = { workflow: value.signer.workflow, source: value.signer.source };
  exact(signer, { workflow: RELEASE_WORKFLOW, source: identity.commit }, "Milestone A signer identity");
  return { schema: RECORD_SCHEMA, identity, authoring_mode: value.authoring_mode, asset_scope: value.asset_scope,
    candidate_sha256: hash(value.candidate_sha256), pair_marker_sha256: hash(value.pair_marker_sha256),
    products, preparation, milestone_a, signer };
}
function encodeRecord(value) {
  const body = c.encode(recordShape(value));
  if (body.length > 1024 * 1024) fail("Milestone A promotion record exceeds byte bound");
  return body;
}
function decodeRecord(body) {
  if (!Buffer.isBuffer(body) || body.length === 0 || body.length > 1024 * 1024) fail("bounded Milestone A promotion bytes required");
  const value = JSON.parse(new TextDecoder("utf-8", { fatal: true }).decode(body));
  const record = recordShape(value);
  if (!body.equals(encodeRecord(record))) fail("noncanonical Milestone A promotion record");
  return record;
}
function validateSelection(body, selected) {
  const record = decodeRecord(body);
  exact(selected, { tag: TAG, ref: `refs/tags/${TAG}`, source: record.identity.commit,
    versions: record.identity.versions }, "Milestone A selected identity");
  return record;
}
function gh(args, cwd, maxBuffer = 1024 * 1024) {
  const result = cp.spawnSync(GH, args, { cwd, env: { PATH: "/usr/local/bin:/usr/bin:/bin", GH_TOKEN: process.env.GH_TOKEN },
    encoding: "utf8", timeout: 120000, killSignal: "SIGKILL", maxBuffer, shell: false });
  if (result.error || result.signal || result.status !== 0) fail("trusted gh provider read failed; stop without publication");
  return result.stdout;
}
function api(endpoint, cwd) {
  return JSON.parse(gh(["api", "--hostname", "github.com", "-H", "Accept: application/vnd.github+json",
    "-H", "X-GitHub-Api-Version: 2022-11-28", `repos/${REPOSITORY}/${endpoint}`], cwd));
}
function receiptContract(prepare, runs, source) {
  sha(source);
  exact(prepare?.schema, "milestone-a-e2e-prepare/v1", "Milestone A preparation receipt schema");
  exact(prepare?.identity, { repository: REPOSITORY, commit: source, engine_revision: source,
    versions: E2E_VERSIONS }, "synthetic Milestone A package identity");
  exact(prepare?.candidate_head, source, "Milestone A prepared source");
  exact(prepare?.publication, false, "non-public Milestone A preparation");
  if (!Array.isArray(runs) || runs.length !== PLATFORMS.length) fail("exact three-platform Milestone A receipts required");
  for (let i = 0; i < runs.length; i++) {
    const receipt = runs[i], [platform, arch] = PLATFORMS[i];
    exact([receipt?.schema, receipt?.platform, receipt?.arch, receipt?.exact_candidate, receipt?.entrypoints,
      receipt?.commands, receipt?.cleanup, receipt?.clean_root_separation],
    ["milestone-a-e2e-run/v1", platform, arch, true, ["agentplugins", "plugin-kit-ai"],
      platform === "linux" ? ["init", "validate", "inspect", "test", "local-add-dry-run"] : ["init", "validate"],
      "complete", true], `Milestone A ${platform}-${arch} receipt`);
    for (const product of ["agentplugins", "plugin-kit-ai"])
      exact(receipt?.provenances?.[product]?.revision, source, `${product} ${platform}-${arch} engine revision`);
  }
  return { source, engine_revision: source, test_versions: E2E_VERSIONS,
    release_versions: { agentplugins: "0.1.60", "plugin-kit-ai": "2.0.0" } };
}
const RECEIPT_READER = String.raw`
import json,os,re,stat,sys,zipfile
p=sys.argv[1]; wanted=sys.argv[2]
st=os.lstat(p)
if not stat.S_ISREG(st.st_mode) or st.st_nlink != 1: raise ValueError('regular archive required')
with zipfile.ZipFile(p) as z:
 infos=z.infolist()
 if not 1 <= len(infos) <= 4096: raise ValueError('bounded archive closure required')
 seen=set(); total=0; chosen=[]
 for i in infos:
  n=i.filename.rstrip('/'); parts=n.split('/')
  if not n or len(parts)>8 or any(not re.fullmatch(r'[A-Za-z0-9][A-Za-z0-9._-]{0,150}',x) or x in ('.','..') for x in parts): raise ValueError('unsafe member')
  k=n.lower()
  if k in seen: raise ValueError('duplicate member')
  seen.add(k); mode=i.external_attr >> 16
  if stat.S_IFMT(mode) not in (0,stat.S_IFDIR if i.is_dir() else stat.S_IFREG): raise ValueError('special member')
  if i.file_size > 134217728 or i.file_size > 200*max(1,i.compress_size): raise ValueError('member bound')
  total += i.file_size
  if total > 2147483648: raise ValueError('archive bound')
  if not i.is_dir() and n == wanted: chosen.append(i)
 if len(chosen) != 1: raise ValueError('exact receipt required')
 body=z.read(chosen[0])
 if len(body)>1048576: raise ValueError('receipt bound')
 sys.stdout.buffer.write(body)
`;
function readReceipt(archive, member, cwd) {
  const result = cp.spawnSync("/usr/bin/python3", ["-B", "-c", RECEIPT_READER, archive, member],
    { cwd, env: { PATH: "/usr/bin:/bin", HOME: cwd, PYTHONNOUSERSITE: "1" }, encoding: "utf8",
      timeout: 120000, killSignal: "SIGKILL", maxBuffer: 1024 * 1024, shell: false });
  if (result.error || result.signal || result.status !== 0) fail("checked Milestone A receipt extraction rejected");
  return JSON.parse(result.stdout);
}
function inspectReceipts(pin, selected, cwd) {
  const p = require("./authoring-promotion");
  positive(pin.run_id); positive(pin.run_attempt, 1000); sha(selected.source);
  const response = api(`actions/runs/${pin.run_id}/artifacts?per_page=100`, cwd);
  const names = [`milestone-a-exact-candidate-${pin.run_id}-${pin.run_attempt}`,
    ...PLATFORMS.map(([platform, arch]) => `milestone-a-${platform}-${arch}-${pin.run_id}-${pin.run_attempt}`)];
  // A rerun retains artifacts from earlier attempts under the same run. Require
  // a complete provider page, then close over the four names for the explicitly
  // selected attempt instead of treating valid earlier-attempt artifacts as an
  // ambiguity in that selection.
  if (!Array.isArray(response.artifacts) || response.total_count !== response.artifacts.length ||
      response.artifacts.length > 100) fail("complete bounded Milestone A artifact response required");
  const retainedName = new RegExp(`^milestone-a-(?:exact-candidate|${PLATFORMS.map(value => value.join("-")).join("|")})-${pin.run_id}-([1-9][0-9]{0,3})$`);
  for (const item of response.artifacts) {
    const match = typeof item?.name === "string" && retainedName.exec(item.name);
    if (!match) fail("exact Milestone A artifact closure required");
    positive(Number(match[1]), 1000);
  }
  const receipts = names.map((name, index) => {
    const rows = response.artifacts.filter(item => item.name === name);
    if (rows.length !== 1 || rows[0].expired !== false || rows[0].workflow_run?.id !== pin.run_id ||
        rows[0].workflow_run?.head_sha !== selected.source || !/^sha256:[0-9a-f]{64}$/.test(rows[0].digest))
      fail("exact retained Milestone A artifact required");
    const item = rows[0], locator = { run_id: pin.run_id, run_attempt: pin.run_attempt,
      artifact_id: positive(item.id), artifact_sha256: item.digest.slice(7) };
    const work = fs.mkdtempSync(path.join(cwd, `milestone-a-receipt-${index}-`));
    const archive = p.acquireArtifact(locator, WORKFLOW, selected.source, work);
    return readReceipt(archive, index === 0 ? "evidence/prepare.json" : "run.json", work);
  });
  return receiptContract(receipts[0], receipts.slice(1), selected.source);
}
function evidenceContract(run, jobs, selected, pin) {
  sha(selected.source); positive(pin.run_id); positive(pin.run_attempt, 1000);
  exact(selected, { tag: TAG, ref: `refs/tags/${TAG}`, source: selected.source,
    versions: { agentplugins: "0.1.60", "plugin-kit-ai": "2.0.0" } }, "fixed paired release selection");
  exact([run.id, run.run_attempt, run.repository?.full_name, run.head_repository?.full_name, run.head_sha,
    run.path, run.event, run.head_branch, run.status, run.conclusion],
  [pin.run_id, pin.run_attempt, REPOSITORY, REPOSITORY, selected.source, WORKFLOW,
    "workflow_dispatch", TAG, "completed", "success"], "exact successful Milestone A invocation");
  if (!Array.isArray(jobs) || jobs.length !== JOBS.length) fail("exact Milestone A job closure required");
  const names = jobs.map(job => {
    exact([job.run_id, job.run_attempt, job.head_sha, job.head_branch, job.status, job.conclusion],
      [pin.run_id, pin.run_attempt, selected.source, TAG, "completed", "success"], "successful Milestone A job");
    positive(job.id); return job.name;
  });
  exact(names.sort(), [...JOBS].sort(), "accelerated Milestone A jobs");
  return { workflow: WORKFLOW, source: selected.source, run_id: pin.run_id, run_attempt: pin.run_attempt, jobs: [...JOBS] };
}
function inspectEvidence(pin, selected, cwd) {
  const version = gh(["--version"], cwd);
  if (!version.startsWith(`gh version ${GH_VERSION} (`)) fail(`trusted /usr/bin/gh ${GH_VERSION} required`);
  const run = api(`actions/runs/${positive(pin.run_id)}/attempts/${positive(pin.run_attempt, 1000)}`, cwd);
  const response = api(`actions/runs/${pin.run_id}/attempts/${pin.run_attempt}/jobs?per_page=100`, cwd);
  if (response.total_count !== response.jobs?.length || response.jobs.length > 100) fail("complete bounded job response required");
  const first = evidenceContract(run, response.jobs, selected, pin);
  // Independent provider readback prevents a single stale observation from
  // becoming publication authority.
  const runAgain = api(`actions/runs/${pin.run_id}/attempts/${pin.run_attempt}`, cwd);
  const jobsAgain = api(`actions/runs/${pin.run_id}/attempts/${pin.run_attempt}/jobs?per_page=100`, cwd);
  const second = evidenceContract(runAgain, jobsAgain.jobs, selected, pin);
  exact(second, first, "independent Milestone A readback");
  return first;
}
function admit(input) {
  const p = require("./authoring-promotion");
  const keys = ["record", "scratch", "workflow_sha", "preparation", "selected", "milestone_a"];
  c.keys(input, keys, "Milestone A promotion options");
  c.safeDirectory(input.scratch);
  if (!path.isAbsolute(input.record) || path.basename(input.record) !== RECORD_FILE) fail("absolute Milestone A promotion record required");
  const record = validateSelection(c.readFile(input.record), input.selected);
  exact(input.workflow_sha, record.identity.commit, "release workflow source");
  exact(input.preparation, { run_id: record.preparation.run_id, run_attempt: record.preparation.run_attempt,
    artifact_id: record.preparation.artifact_id, artifact_sha256: record.preparation.artifact_sha256 }, "record preparation selection");
  exact(input.milestone_a, { run_id: record.milestone_a.run_id, run_attempt: record.milestone_a.run_attempt }, "record Milestone A selection");
  const evidence = inspectEvidence(input.milestone_a, input.selected, input.scratch);
  evidence.receipts = inspectReceipts(input.milestone_a, input.selected, input.scratch);
  const prepared = p.acquireMilestonePreparation(input.preparation, record, input.scratch);
  require("./authoring-native-qualification").readPreparation(prepared.root,
    { identity: record.identity, candidate_sha256: record.candidate_sha256, pair_marker_sha256: record.pair_marker_sha256,
      products: Object.fromEntries(c.PRODUCTS.map(name => [name, { manifest_sha256: record.products[name].manifest_sha256,
        checksums_sha256: record.products[name].checksums_sha256 }])) }, prepared.preparation);
  const subjects = p.frozenSubjects(prepared.root, record);
  subjects.push({ file: input.record, sha256: c.digest(encodeRecord(record)) });
  return { o: { ...input, root: prepared.root }, record, subjects, milestone_a: evidence };
}

module.exports = { REPOSITORY, WORKFLOW, RELEASE_WORKFLOW, RECORD_SCHEMA, RECORD_FILE, TAG, KIT_TAG, JOBS, E2E_VERSIONS, PLATFORMS,
  recordShape, encodeRecord, decodeRecord, validateSelection, evidenceContract, receiptContract, inspectEvidence, inspectReceipts, admit };
