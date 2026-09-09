"use strict";

// C3a: closed local input custody and structural J contract. No producer, E
// authentication, installer policy, archive engine or execution override lives here.
const fs = require("node:fs");
const path = require("node:path");
const assert = require("node:assert/strict");
const c = require("./dual-authoring-candidate");
const contract = require("../lib/public-authoring-contract");
const { fields, hash, positive, fixed } = contract.checks;
const LIMIT = 1024 * 1024, TRANSCRIPT_LIMIT = 16 * LIMIT;
const SCHEMA = "authoring-public-journey/v1";
const MATRIX_SCHEMA = "public-wrapper-matrix/v1";
const INTAKE = "public-authenticated/v1";
const WORKFLOW = ".github/workflows/authoring-public-packed.yml";
const REQUEST = ["intake", "expectedCommit", "journey", "journeySha256", "admission", "admissionSha256", "fixtureRoot"];
const J_FIELDS = ["schema", "status", "identity", "authoring_mode", "asset_scope", "candidate_sha256", "pair_marker_sha256",
  "native_inputs", "stage", "packs", "producer", "cell", "tools", "command_contract_sha256", "subjects", "projects", "evidence", "assertions"];
const LANES = Object.freeze(["skill", "mcp-remote", "mcp-stdio", "hybrid-remote", "hybrid-stdio"]);
const EVIDENCE = Object.freeze(["commands.json", "projects.json", "npm-lifecycle.json", "cache-process.json", "installer.json"]);
const ASSERTIONS = Object.freeze(["fixed_commands", "pair_parity", "projects_preserved", "npm_lifecycle", "cache_process", "production_installer", "children_reaped"]);
const MISSING = "C3b required: reviewed public installer result validator and whole-descendant observer; full npm/cache/process/parity finalization is not implemented; J execution admission is closed";
const matrix = Object.freeze(["linux-amd64", "linux-arm64", "darwin-amd64", "darwin-arm64", "windows-amd64", "windows-arm64"].flatMap(target =>
  [18, 22, 24].map(node => Object.freeze({ key: `${target}/${node === 18 ? "kit" : "pair"}-node${node}`, target, node,
    products: Object.freeze(node === 18 ? ["plugin-kit-ai"] : [...c.PRODUCTS]) }))));
const agree = (a, b, label) => assert.deepEqual(a, b, `C3 ${label}`);
function cell(key) { const found = matrix.find(row => row.key === key); assert.ok(found, "fixed C3 cell required"); return found; }
function absolute(value) {
  assert.ok(typeof value === "string" && value.length <= 4096 && !/[\x00-\x1f\x7f]/.test(value) &&
    path.isAbsolute(value) && path.normalize(value) === value && value !== path.parse(value).root, "canonical absolute C3 path");
  return value;
}
function disjoint(roots) {
  roots.forEach(absolute);
  roots.forEach((a, i) => roots.slice(i + 1).forEach(b => {
    a = a.toLowerCase(); b = b.toLowerCase();
    assert.ok(a !== b && !a.startsWith(b + path.sep) && !b.startsWith(a + path.sep), "C3 overlapping roots");
  }));
}
function pin(file, sha256, maximum = LIMIT) {
  hash(sha256, "file"); const body = c.readFile(absolute(file), maximum);
  agree(c.digest(body), sha256, "retained file pin"); return body;
}
function bounded(body, maximum) {
  assert.ok(Buffer.isBuffer(body) && body.length > 0 && body.length <= maximum, "bounded canonical C3 bytes");
  // Fixed schemas are shallow. Bound nesting before parse, including arrays.
  const text = new TextDecoder("utf-8", { fatal: true, ignoreBOM: true }).decode(body);
  let depth = 0, quoted = false, escaped = false;
  for (const ch of text) {
    if (quoted) { if (escaped) escaped = false; else if (ch === "\\") escaped = true; else if (ch === '"') quoted = false; }
    else if (ch === '"') quoted = true;
    else if (ch === "{" || ch === "[") assert.ok(++depth <= 16, "C3 depth limit");
    else if (ch === "}" || ch === "]") depth--;
  }
  const value = JSON.parse(text);
  agree(body, c.encode(value), "canonical JSON (no duplicate keys or alternate spelling)"); return value;
}
function fileJSON(file, maximum = LIMIT) { return bounded(c.readFile(absolute(file), maximum), maximum); }
function locator(value) {
  fields(value, ["sha256", "artifact"], "C3 locator"); hash(value.sha256, "locator");
  fields(value.artifact, ["run_id", "run_attempt", "artifact_id", "artifact_sha256"], "C3 artifact");
  for (const k of ["run_id", "artifact_id"]) positive(value.artifact[k], Number.MAX_SAFE_INTEGER, k);
  positive(value.artifact.run_attempt, 1000, "attempt"); hash(value.artifact.artifact_sha256, "artifact");
}
function producer(value, input) {
  fields(value, ["workflow", "source", "ref", "run_id", "run_attempt"], "C3 producer");
  fixed(value.workflow, WORKFLOW, "public workflow"); fixed(value.source, input.identity.commit, "source F");
  fixed(value.ref, `refs/tags/${input.products.agentplugins.tag}`, "public ref");
  positive(value.run_id, Number.MAX_SAFE_INTEGER, "public run"); positive(value.run_attempt, 1000, "public attempt");
}
function tools(value, selected) {
  fields(value, ["orchestrator_node", "npm_node", "shim_node", "npm", "go", "host"], "C3 tools");
  const [os, cpu] = selected.target.split("-");
  agree(value.host, { platform: os === "windows" ? "win32" : os, arch: cpu === "amd64" ? "x64" : "arm64" }, "host matrix");
  for (const key of ["orchestrator_node", "npm_node", "shim_node", "npm", "go"]) {
    const t = value[key];
    if (key === "go" && selected.key !== "linux-amd64/pair-node22") { agree(t, null, "Go only in bridge cell"); continue; }
    fields(t, ["path", "sha256", "version"], "C3 tool pin"); absolute(t.path); hash(t.sha256, key);
    assert.ok(typeof t.version === "string" && t.version.length < 128 && /^[\x21-\x7e]+$/.test(t.version), "bounded tool version");
    if (key.endsWith("_node")) assert.match(t.version, /^v[1-9][0-9]*\.[0-9]+\.[0-9]+$/);
    if (["npm_node", "shim_node"].includes(key)) assert.ok(t.version.startsWith(`v${selected.node}.`), "actual selected Node major");
  }
}

/** Fixed core command/result inventory, never executable caller argv. C3b must
 * also implement the separately required npm/cache/process evidence validators. */
function commandContract(key) {
  const selected = cell(key), rows = [];
  for (const product of selected.products) {
    const add = (id, args, status = 0, author = true, lane = null, scenario = "projects") => rows.push({ product, id,
      argv: [...(author && product === "agentplugins" ? ["author"] : []), ...args], status, author, lane, scenario });
    add("product-version", ["version", "--format=json"], 0, false);
    add("product-help", ["--help"], 0, false);
    add("engine-version", ["version", "--format=json"]); add("author-help", ["--help", "--format=json"]);
    add("capabilities", ["capabilities", "--format=json"]);
    for (const lane of LANES) {
      // Existing bridge constructor, loaded only on invocation (no require cycle).
      const init = [...require("./packed-installer-bridge").publicInit(lane), "--format=json"];
      add(`${lane}/init`, init, 0, true, lane);
      add(`${lane}/extra-skill`, ["skills", "init", "extra-skill", lane, "--description", "Use for extra documentation requests", "--format=json"], 0, true, lane);
      for (const verb of ["skills validate", "validate", "inspect", "test", "compat", "doctor"]) {
        add(`${lane}/${verb.replace(" ", "-")}`, [...verb.split(" "), lane,
          ...(verb === "compat" ? ["--target", "claude,codex"] : []), "--format=json"], verb === "doctor" && lane !== "skill" ? 1 : 0, true, lane);
      }
      add(`${lane}/existing`, init, 1, true, lane);
    }
    add("malformed-skill", ["validate", "skill", "--format=json"], 1, true, "skill", "malformed-skill");
    add("invalid-flag", ["init", "invalid-destination", "--force", "--format=json"], 2);
    add("missing-template-input", ["init", "missing-destination", "--template", "mcp-stdio", "--format=json"], 2);
    add("installer-flag", ["validate", "skill", "--scope=user", "--format=json"], 2, true, "skill");
    if (product === "plugin-kit-ai") add("retired-v1", ["update", "--all", "--format=json"], 2, false);
    if (product === "agentplugins") for (const lane of ["skill", "mcp-remote", "hybrid-stdio"]) {
      for (const verb of ["dry-run", "add", "info", "update", "remove", "list"]) {
        const argv = verb === "list" ? ["list"] : [verb === "dry-run" ? "add" : verb, lane, "--target=codex"];
        add(`installer/${lane}/${verb}`, [...argv, ...(verb === "dry-run" ? ["--dry-run"] : []), "--format=json"], 0, false, lane);
      }
    }
  }
  return rows;
}
function journey(value, inputBytes, stageBytes) {
  const input = contract.decodeInputs(inputBytes);
  const stage = require("./stage-authoring-npm").decodeStage(stageBytes, inputBytes);
  fields(value, J_FIELDS, "C3 J"); fixed(value.schema, SCHEMA, "J schema"); fixed(value.status, "completed", "J status syntax");
  for (const name of ["identity", "authoring_mode", "asset_scope", "candidate_sha256", "pair_marker_sha256", "native_inputs", "packs"]) agree(value[name], stage[name], name);
  locator(value.stage); agree(value.stage.sha256, c.digest(stageBytes), "exact S bytes");
  agree([value.stage.artifact.run_id, value.stage.artifact.run_attempt], [stage.producer.run_id, stage.producer.run_attempt], "S attempt");
  assert.ok(![stage.native_inputs.artifact.artifact_id, input.preparation.artifact.artifact_id].includes(value.stage.artifact.artifact_id), "separate stage artifact");
  producer(value.producer, input);
  assert.ok(![stage.producer.run_id, input.producer.run_id, input.preparation.artifact.run_id].includes(value.producer.run_id), "separate public invocation");
  const selected = cell(value.cell); tools(value.tools, selected);
  agree(value.command_contract_sha256, c.digest(c.encode(commandContract(value.cell))), "fixed core command contract");
  agree(value.subjects, Object.fromEntries(c.PRODUCTS.map(p => [p, input.products[p].assets])), "all twelve outer/inner native subject pins");
  fields(value.projects, selected.products, "C3 projects");
  for (const p of selected.products) absolute(value.projects[p]);
  assert.ok(Array.isArray(value.evidence) && value.evidence.length === EVIDENCE.length, "fixed evidence table");
  agree(Reflect.ownKeys(value.evidence), [...EVIDENCE.map((_, i) => String(i)), "length"], "plain evidence array fields");
  value.evidence.forEach((row, i) => {
    fields(row, ["path", "size", "sha256"], "C3 evidence row"); fixed(row.path, EVIDENCE[i], "fixed evidence filename");
    positive(row.size, i === 0 ? TRANSCRIPT_LIMIT : LIMIT, "evidence size"); hash(row.sha256, "evidence");
  });
  fields(value.assertions, ASSERTIONS, "C3 assertion syntax");
  for (const key of ASSERTIONS) fixed(value.assertions[key], true, "assertion syntax only");
  // Reconstruct every object in fixed order, rather than preserving caller key
  // insertion order. Structural syntax never recomputes successful assertions.
  const orderedLocator = v => ({ sha256: v.sha256, artifact: Object.fromEntries(
    ["run_id", "run_attempt", "artifact_id", "artifact_sha256"].map(k => [k, v.artifact[k]])) });
  const normalized = { ...value,
    ...Object.fromEntries(["identity", "authoring_mode", "asset_scope", "candidate_sha256", "pair_marker_sha256", "native_inputs", "packs"].map(k => [k, stage[k]])),
    stage: orderedLocator(value.stage),
    producer: Object.fromEntries(["workflow", "source", "ref", "run_id", "run_attempt"].map(k => [k, value.producer[k]])),
    tools: Object.fromEntries(["orchestrator_node", "npm_node", "shim_node", "npm", "go", "host"].map(k => [k,
      value.tools[k] === null ? null : Object.fromEntries((k === "host" ? ["platform", "arch"] : ["path", "sha256", "version"]).map(n => [n, value.tools[k][n]]))])),
    subjects: Object.fromEntries(c.PRODUCTS.map(p => [p, input.products[p].assets])),
    projects: Object.fromEntries(selected.products.map(p => [p, value.projects[p]])),
    evidence: value.evidence.map(row => ({ path: row.path, size: row.size, sha256: row.sha256 })),
    assertions: Object.fromEntries(ASSERTIONS.map(k => [k, value.assertions[k]])) };
  return Object.fromEntries(J_FIELDS.map(key => [key, normalized[key]]));
}
function encodeJourney(value, input, stage) {
  const body = c.encode(journey(value, input, stage)); assert.ok(body.length <= LIMIT, "J size limit"); return body;
}
function decodeJourney(body, input, stage) {
  const value = journey(bounded(body, LIMIT), input, stage);
  agree(body, c.encode(value), "fixed J field order"); return value;
}
function request(value) {
  fields(value, REQUEST, "C3 bridge request"); fixed(value.intake, INTAKE, "authentic intake");
  assert.ok(typeof value.expectedCommit === "string" && value.expectedCommit.length === 40 && /^[0-9a-f]{40}$/.test(value.expectedCommit) && !/^0+$/.test(value.expectedCommit), "exact source F");
  for (const key of ["journey", "admission", "fixtureRoot"]) absolute(value[key]);
  for (const key of ["journeySha256", "admissionSha256"]) hash(value[key], key);
  assert.ok(c.encode(value).length <= LIMIT, "request size limit"); return value;
}

/** Input custody only: genuinely call completed S and I readers. The local
 * receipt is comparison data, never its own authentication or completed E. */
function readJourneyInputs(value) {
  const r = request(value), receiptBytes = pin(r.admission, r.admissionSha256);
  const a = bounded(receiptBytes, LIMIT);
  fields(a, ["schema", "selected", "workflow_sha", "input_file", "stage", "repo", "work_parent", "stage_root", "input_root",
    "journey_root", "fixture_root", "cell", "tools", "producer"], "C3 local admission");
  fixed(a.schema, "authoring-public-local-inputs/v1", "local input schema"); locator(a.stage);
  fields(a.selected, ["tag", "ref", "source", "versions"], "selection");
  for (const key of ["repo", "work_parent", "stage_root", "input_root", "journey_root", "fixture_root"]) c.safeDirectory(absolute(a[key]));
  const roots = [a.repo, a.work_parent, a.stage_root, a.input_root, a.journey_root, a.fixture_root]; disjoint(roots);
  for (const root of roots) disjoint([root, r.admission]);
  agree(a.repo, path.resolve(__dirname, "../../.."), "executing source checkout");
  agree(r.journey, path.join(a.journey_root, "public-journey.json"), "fixed local J filename");
  agree(r.fixtureRoot, a.fixture_root, "original fixture root");
  agree(a.input_file, path.join(a.input_root, contract.INPUT_FILE), "retained original I filename");
  const inputBytes = c.readFile(a.input_file, LIMIT), input = contract.decodeInputs(inputBytes);
  agree(a.selected, { tag: input.products.agentplugins.tag, ref: `refs/tags/${input.products.agentplugins.tag}`,
    source: input.identity.commit, versions: input.identity.versions }, "selection from I");
  agree(a.workflow_sha, r.expectedCommit, "requested F"); agree(a.workflow_sha, input.identity.commit, "I source F");
  const stageBytes = pin(path.join(a.stage_root, "completion.json"), a.stage.sha256);
  const body = pin(r.journey, r.journeySha256), j = decodeJourney(body, inputBytes, stageBytes);
  agree(j.stage, a.stage, "stage locator"); agree(j.cell, a.cell, "local cell"); agree(j.tools, a.tools, "local tools");
  agree(j.producer, a.producer, "same public invocation pins");
  agree(j.tools.host, { platform: process.platform, arch: process.arch }, "actual reader host");
  agree(j.tools.orchestrator_node, { path: process.execPath, sha256: c.digest(c.readFile(process.execPath)), version: process.version }, "executing Node");
  const toolPins = Object.entries(j.tools).filter(([k, v]) => k !== "host" && v !== null).map(([, v]) => v);
  for (const t of toolPins) { pin(t.path, t.sha256, contract.MAX_NATIVE_BYTES); for (const root of roots) disjoint([root, t.path]); }
  const stages = require("./stage-authoring-npm"), inputs = require("./authoring-native-inputs");
  assert.equal(typeof stages.readStage, "function", "missing completed stage verifier");
  assert.equal(typeof inputs.readInputs, "function", "missing completed input verifier");
  const admittedStage = stages.readStage({ input: inputBytes, selected: a.selected, workflow_sha: a.workflow_sha,
    artifact: a.stage.artifact, stage_sha256: a.stage.sha256, repo: a.repo, workParent: a.work_parent,
    node: a.tools.orchestrator_node.path, npm: a.tools.npm.path });
  agree(admittedStage.record, stages.decodeStage(stageBytes, inputBytes), "authenticated S record");
  const admittedInputs = inputs.readInputs({ input: inputBytes, selected: a.selected, workflow_sha: a.workflow_sha,
    artifact: j.native_inputs.artifact, scratch: a.work_parent });
  agree(admittedInputs.input, input, "authenticated I record");
  // Compare retained original custody to independently acquired authenticated bytes.
  const retained = [];
  for (const [admitted, root, count] of [[admittedStage, a.stage_root, 3], [admittedInputs, a.input_root, 19]]) {
    assert.equal(admitted.subjects.length, count, "original subject multiset count");
    for (const row of admitted.subjects) {
      const relative = path.relative(admitted.root, row.file);
      assert.ok(relative && !relative.startsWith("..") && !path.isAbsolute(relative), "checked subject containment");
      const file = path.join(root, relative);
      pin(row.file, row.sha256, contract.MAX_NATIVE_BYTES); pin(file, row.sha256, contract.MAX_NATIVE_BYTES);
      retained.push({ file, sha256: row.sha256 });
    }
  }
  // These local snapshots are stable across re-admission; freshly extracted
  // authentication scratch is not part of the local project's seal identity.
  const bridge = require("./packed-installer-bridge"), projects = [];
  for (const product of cell(j.cell).products) {
    const parent = path.join(a.fixture_root, `${product} projects ü`);
    agree(j.projects[product], parent, "original public project parent");
    agree(fs.readdirSync(parent).sort(), [...LANES].sort(), "all and only five original projects");
    for (const lane of LANES) {
      const source = path.join(parent, lane); c.safeDirectory(source);
      c.readFile(path.join(source, "plugin.json"), LIMIT); c.readFile(path.join(source, "skills/extra-skill/SKILL.md"), LIMIT);
      projects.push({ product, lane, source });
    }
  }
  const evidence = {};
  for (const row of j.evidence) {
    const bytes = pin(path.join(a.journey_root, row.path), row.sha256, row.path === "commands.json" ? TRANSCRIPT_LIMIT : LIMIT);
    agree(bytes.length, row.size, "evidence size"); evidence[row.path] = bounded(bytes, row.path === "commands.json" ? TRANSCRIPT_LIMIT : LIMIT);
  }
  const snapshots = [a.stage_root, a.input_root, a.journey_root, a.fixture_root].map(root => bridge.snapshot(root));
  agree(evidence["projects.json"], Object.fromEntries(cell(j.cell).products.map(p => [p, bridge.snapshot(j.projects[p])])), "original project trees and modes");
  for (const row of retained) pin(row.file, row.sha256, contract.MAX_NATIVE_BYTES);
  for (const t of toolPins) pin(t.path, t.sha256, contract.MAX_NATIVE_BYTES);
  agree(pin(r.admission, r.admissionSha256), receiptBytes, "late admission bytes"); agree(pin(r.journey, r.journeySha256), body, "late J bytes");
  return { record: j, evidence, identity: j.identity, repo: a.repo, candidate_sha256: j.candidate_sha256,
    packs: j.packs, projects, snapshots, protected_paths: [a.work_parent, r.admission, ...toolPins.map(t => t.path)] };
}
function verifyJourney(local) {
  const { record: j, evidence } = local, expected = commandContract(j.cell), rows = evidence["commands.json"];
  assert.ok(Array.isArray(rows) && rows.length === expected.length, "exact ordered C3 core command rows");
  rows.forEach((row, i) => {
    const want = expected[i]; fields(row, ["product", "id", "argv", "cwd", "status", "signal", "stdout", "stderr"], "C3 command result");
    for (const key of ["product", "id", "argv", "status"]) agree(row[key], want[key], `command ${key}`);
    const parent = j.projects[want.product];
    const cwd = want.scenario === "projects" ? parent : path.join(path.dirname(parent), `${want.product} malformed-skill ü`);
    agree(row.cwd, cwd, "fixed cwd"); agree(row.signal, null, "complete command (no signal)"); agree(row.stderr, "", "clean stderr");
    assert.ok(typeof row.stdout === "string" && Buffer.byteLength(row.stdout) <= LIMIT, "bounded stdout");
    if (want.id === "product-help") { assert.ok(row.stdout.length > 50, "product help"); return; }
    const v = JSON.parse(row.stdout); agree(v.schema_version, 1, "result schema");
    agree(v.result, want.status === 0 ? "success" : "failure", "result status");
    assert.ok(v.data && typeof v.data === "object" && !Array.isArray(v.data), "result data");
    if (want.author) {
      const args = want.argv.slice(want.product === "agentplugins" ? 1 : 0);
      agree(v.command, args[0] === "--help" ? "author" : `author.${args[0]}${args[0] === "skills" ? `.${args[1]}` : ""}`, "author operation");
      agree(v.data.revision, j.identity.commit, "engine F"); agree(v.data.engine, "standard-first-slice/1", "engine");
      agree(v.data.authoring_schema_version, 1, "author schema"); agree(v.data.runtime_evidence?.status, "not_evaluated", "offline runtime boundary");
      if (want.lane) agree(v.data.committed, /\/(init|extra-skill)$/.test(want.id), "mutation boundary");
      if (want.id.endsWith("/doctor")) agree(v.data.toolchain?.status, want.lane === "skill" ? "pass" : "not_evaluated", "doctor boundary");
    } else if (want.id === "product-version") agree(v.data[want.product === "agentplugins" ? "version" : "product_version"], j.identity.versions[want.product], "product version");
  });
  // A complete structural transcript still cannot attest process observation,
  // installer assessment, npm postinstall, parity or final child quiescence.
  throw new Error(MISSING);
}
function readJourney(value) { const local = readJourneyInputs(value); verifyJourney(local); }
function readAcceptance() { throw new Error("C3b required: completed remote E reader is closed; local J and fixture success are not E"); }
function main(args) {
  assert.ok(args.length === 2 && args[0] === "--read-local-inputs", "C3a supports only --read-local-inputs REQUEST; public execution and E are closed");
  const requestValue = fileJSON(args[1]), result = readJourneyInputs(requestValue);
  return { scope: "authenticated-input-custody-only", cell: result.record.cell, source: result.identity.commit,
    journey_sha256: requestValue.journeySha256, release_eligible: false, platform_acceptance: false, attested: false };
}
// No producer or completed-E CLI can return a success-shaped placeholder.
module.exports = { matrix, commandContract, encodeJourney, decodeJourney, readJourneyInputs, verifyJourney, readJourney,
  readAcceptance, request, fileJSON, disjoint, main, LIMIT, SCHEMA, MATRIX_SCHEMA, INTAKE, WORKFLOW, MISSING };
if (require.main === module) {
  try { process.stdout.write(c.encode(main(process.argv.slice(2)))); }
  catch (error) { process.stderr.write(`C3 local inputs: ${error.message}\n`); process.exitCode = 1; }
}
