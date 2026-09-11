"use strict";

// C3 local custody, fixed producer and semantic evidence contracts. Completed E
// admission belongs to step 3; installer/observation/custody engines stay external.
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
const MISSING = "C3b required: PUBLIC_FACADE_REQUIRED:public-authoring-custody.js#readPublicInputs,public-process-observation.js#openPublicObservation,public-process-observation.js#verifyPublicObservation,public-installer-evidence.js#requirePublicInstaller,public-installer-evidence.js#verifyPublicInstaller";
const matrix = Object.freeze(["linux-amd64", "linux-arm64", "darwin-amd64", "darwin-arm64", "windows-amd64", "windows-arm64"].flatMap(target =>
  [18, 22, 24].map(node => Object.freeze({ key: `${target}/${node === 18 ? "kit" : "pair"}-node${node}`, target, node,
    products: Object.freeze(node === 18 ? ["plugin-kit-ai"] : [...c.PRODUCTS]) }))));
// Compare without asking node:assert to render an unbounded object/Buffer diff.
// Rejected transcript-sized values must remain cheap to report under memory limits.
const agree = (a, b, label) => assert.ok(require('node:util').isDeepStrictEqual(a, b), `C3 ${label}`);
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
    fields(t, ["path", "sha256", "version"], "C3 tool pin"); hostAbsolute(t.path, selected.key); hash(t.sha256, key);
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
  for (const p of selected.products) hostAbsolute(value.projects[p], selected.key);
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
  const evidence = {}, budget = { size: j.evidence.reduce((n, row) => n + row.size, 0) };
  for (const row of j.evidence) {
    const bytes = pin(path.join(a.journey_root, row.path), row.sha256, row.path === "commands.json" ? TRANSCRIPT_LIMIT : LIMIT);
    agree(bytes.length, row.size, "evidence size"); evidence[row.path] = expandEvidence(row.path, bounded(bytes, row.path === "commands.json" ? TRANSCRIPT_LIMIT : LIMIT), a.journey_root, budget);
  }
  const snapshots = [a.stage_root, a.input_root, a.journey_root, a.fixture_root].map(root => bridge.snapshot(root));
  agree(evidence["projects.json"], Object.fromEntries(cell(j.cell).products.map(p => [p, bridge.snapshot(j.projects[p])])), "original project trees and modes");
  for (const row of retained) pin(row.file, row.sha256, contract.MAX_NATIVE_BYTES);
  for (const t of toolPins) pin(t.path, t.sha256, contract.MAX_NATIVE_BYTES);
  agree(pin(r.admission, r.admissionSha256), receiptBytes, "late admission bytes"); agree(pin(r.journey, r.journeySha256), body, "late J bytes");
  return { record: j, evidence, identity: j.identity, repo: a.repo, candidate_sha256: j.candidate_sha256,
    packs: j.packs, projects, snapshots, protected_paths: [a.work_parent, r.admission, ...toolPins.map(t => t.path)] };
}
// C3b step 2. These pure contracts are replayed against authenticated observations;
// neither a caller assertion nor a synthetic unit fixture authenticates execution.
const AGGREGATE_LIMIT = 128 * LIMIT;
const PRODUCE_FIELDS = ['schema', 'selected', 'workflow_sha', 'stage', 'input_file', 'repo', 'work_parent', 'output', 'cell', 'tools', 'producer'];
const FACADES = Object.freeze({
  'public-authoring-custody': ['readPublicInputs'],
  'public-process-observation': ['openPublicObservation', 'verifyPublicObservation'],
  'public-installer-evidence': ['requirePublicInstaller', 'verifyPublicInstaller']
});
function requireFacades(key) {
  const result = {}, missing = [];
  for (const [name, exports] of Object.entries(FACADES)) {
    if (name === 'public-installer-evidence' && cell(key).node === 18) continue;
    const file = path.join(__dirname, name + '.js');
    if (!fs.existsSync(file)) { missing.push(...exports.map(e => `${name}.js#${e}`)); continue; }
    const api = require(file);
    for (const e of exports) if (typeof api[e] !== 'function') missing.push(`${name}.js#${e}`);
    result[name] = api;
  }
  assert.equal(missing.length, 0, `C3b required: PUBLIC_FACADE_REQUIRED:${missing.join(',')}`);
  return result;
}
const hostPath = key => key.startsWith('windows-') ? path.win32 : path.posix;
function hostAbsolute(value, key) {
  const p = hostPath(key);
  assert.ok(typeof value === 'string' && value.length <= 4096 && !/[\x00-\x1f\x7f]/.test(value) && p.isAbsolute(value) &&
    p.normalize(value) === value && value !== p.parse(value).root && !value.startsWith('\\\\'), 'canonical recorded host path');
  return value;
}
function freeze(value) { if (value && typeof value === 'object') { Object.values(value).forEach(freeze); Object.freeze(value); } return value; }
function list(value, count, label) {
  assert.ok(Array.isArray(value) && value.length === count, label);
  agree(Reflect.ownKeys(value), [...value.map((_, i) => String(i)), 'length'], `${label} plain array`);
}
function nonnegative(value, maximum, label) { assert.ok(Number.isSafeInteger(value) && value >= 0 && value <= maximum, label); }
function textValue(value, maximum = LIMIT) { assert.ok(typeof value === 'string' && Buffer.byteLength(value) <= maximum && !value.includes('\0'), 'bounded text'); }
function digestID(value) { assert.match(value, /^sha256:[0-9a-f]{64}$/); hash(value.slice(7), 'digest identity'); }
function sidecar(value) {
  fields(value, ['path', 'size', 'sha256'], 'observation sidecar');
  assert.match(value.path, /^sidecars\/[a-z0-9][a-z0-9._-]{0,150}$/);
  positive(value.size, TRANSCRIPT_LIMIT, 'sidecar size'); hash(value.sha256, 'sidecar');
}
/** Supplementary rows have a separate identity/count; core55/core127 never change. */
function scenarioContract(key) {
  const selected = cell(key), npm = [], cache = [], installer = [];
  const add = (into, product, kind, prefix, cacheName, suffix = kind, group = null, command = null, event = null) =>
    into.push({ id: `${product}/${prefix}/${suffix}`, product, kind, prefix, cache: cacheName, group, command, event });
  for (const p of selected.products) {
    const prefix = `alone-${p}`;
    for (const k of ['install', 'probe', 'uninstall', 'reinstall', 'probe-reinstalled']) add(npm, p, k, prefix, prefix);
  }
  if (selected.products.length === 2) for (const order of [selected.products, [...selected.products].reverse()]) {
    const prefix = `shared-${order[0]}`;
    for (const p of order) { add(npm, p, 'install', prefix, prefix); add(npm, p, 'probe', prefix, prefix); }
    for (const p of order) {
      const peer = order.find(x => x !== p);
      add(npm, p, 'uninstall', prefix, prefix);
      add(npm, peer, 'probe-peer', prefix, prefix, `peer-after-${p}`);
      add(npm, p, 'reinstall', prefix, prefix);
      add(npm, p, 'probe-reinstalled', prefix, prefix);
    }
  }
  for (const p of selected.products) {
    const prefix = `alone-${p}`;
    for (const k of ['cold', 'warm', 'repair', 'invalid-cold', 'invalid-warm'])
      add(cache, p, k, prefix, k === 'invalid-cold' ? `invalid-${p}` : `serial-${p}`);
    for (let i = 0; i < 4; i++) add(cache, p, 'concurrent-cold', prefix, `concurrent-${p}`, `concurrent-cold-${i}`, `cold-${p}`);
    for (let i = 0; i < 2; i++) add(cache, p, 'concurrent-warm', prefix, `concurrent-${p}`, `concurrent-warm-${i}`);
    add(cache, p, 'literal-argv', prefix, `serial-${p}`);
    if (key.startsWith('windows-')) {
      add(cache, p, 'cancel', prefix, `serial-${p}`, 'console-cancel', null, null, 'CTRL_C_EVENT');
      add(cache, p, 'cancel', prefix, `serial-${p}`, 'process-cancel', null, null, 'TerminateProcess');
    } else for (const event of ['SIGINT', 'SIGTERM']) add(cache, p, 'cancel', prefix, `serial-${p}`, event, null, null, event);
    add(cache, p, 'waiter-owner', prefix, `waiter-${p}`, 'waiter-owner', `waiter-${p}`);
    add(cache, p, 'waiter-cancel', prefix, `waiter-${p}`, 'waiter-cancel', `waiter-${p}`, null,
      key.startsWith('windows-') ? 'CTRL_C_EVENT' : 'SIGINT');
  }
  if (selected.products.length === 2) for (const p of selected.products)
    add(cache, p, 'peer-overlap', `alone-${p}`, 'peer-overlap', 'peer-overlap', 'peer-overlap');
  commandContract(key).forEach((r, i) => add(r.id.startsWith('installer/') ? installer : cache, r.product, 'core',
    `alone-${r.product}`, `alone-${r.product}`, `core-${r.id}`, null, i));
  return freeze({ npm, cache, installer });
}
function rootsFor(output, key) {
  const p = hostPath(key); hostAbsolute(output, key);
  return Object.fromEntries(['input', 'stage', 'admission', 'projects', 'npm', 'cache', 'client', 'state', 'evidence', 'scenarios']
    .map(n => [n, p.join(output, n)]));
}
function scopePaths(j, scenario, roots) {
  const p = hostPath(j.cell), name = scenario.cache;
  return { prefix: p.join(roots.npm, scenario.prefix, 'prefix'), home: p.join(roots.cache, name),
    cwd: scenario.kind === 'core' ? commandCwd(j, commandContract(j.cell)[scenario.command]) :
      p.join(roots.scenarios, scenario.product, scenario.kind === 'literal-argv' ? "cwd spaces ü 'quotes' $literal ; &" : scenario.prefix),
    userconfig: p.join(roots.npm, scenario.prefix, 'user.npmrc'), globalconfig: p.join(roots.npm, scenario.prefix, 'global.npmrc'),
    npmCache: p.join(roots.npm, scenario.prefix, 'npm-cache') };
}
function commandCwd(j, want) {
  const p = hostPath(j.cell), parent = j.projects[want.product];
  return want.scenario === 'projects' ? parent : p.join(p.dirname(parent), `${want.product} malformed-skill ü`);
}
const LITERAL_DESCRIPTION = 'Use spaces ü "double" \'single\' $HOME $(literal) `literal` ; & | < > %PATH% !literal!';
function plannedInvocation(j, scenario, roots) {
  const p = hostPath(j.cell), s = scopePaths(j, scenario, roots), windows = j.cell.startsWith('windows-');
  const asset = p.join(roots.input, j.subjects[scenario.product][cell(j.cell).target].file);
  const env = { HOME: s.home, TMPDIR: p.join(s.home, 'tmp'),
    PATH: p.dirname(j.tools[['install', 'reinstall', 'uninstall'].includes(scenario.kind) ? 'npm_node' : 'shim_node'].path), LANG: 'C.UTF-8', LC_ALL: 'C.UTF-8',
    NPM_CONFIG_USERCONFIG: s.userconfig, NPM_CONFIG_GLOBALCONFIG: s.globalconfig, NPM_CONFIG_CACHE: s.npmCache,
    NPM_CONFIG_OFFLINE: 'true', NPM_CONFIG_AUDIT: 'false', NPM_CONFIG_FUND: 'false',
    UAP_PUBLIC_AUTHORING_ASSET_FILE: scenario.kind.startsWith('invalid-') ? p.join(roots.input, 'absent-invalid-locator') : asset,
    CODEX_HOME: p.join(roots.client, 'codex'), XDG_CONFIG_HOME: roots.client, XDG_STATE_HOME: roots.state,
    XDG_DATA_HOME: p.join(roots.state, 'data'), XDG_CACHE_HOME: p.join(s.home, '.cache') };
  if (windows) {
    // Frozen OS destinations, never inherited COMSPEC/PowerShell or caller shell.
    env.USERPROFILE = s.home; env.TEMP = env.TMP = p.join(s.home, 'tmp');
    env.SystemRoot = 'C:\\Windows'; env.ComSpec = 'C:\\Windows\\System32\\cmd.exe';
    env.PATHEXT = '.COM;.EXE;.BAT;.CMD'; env.LOCALAPPDATA = p.join(s.home, 'AppData', 'Local');
  }
  let argv;
  if (['install', 'reinstall', 'uninstall'].includes(scenario.kind)) {
    const uninstall = scenario.kind === 'uninstall';
    argv = [j.tools.npm_node.path, j.tools.npm.path, uninstall ? 'uninstall' : 'install', '--global', '--prefix', s.prefix,
      '--offline', '--ignore-scripts=false', ...(!uninstall ? ['--foreground-scripts'] : []), '--no-audit', '--no-fund',
      uninstall ? contract.PACKAGES[scenario.product] : p.join(roots.stage, j.packs[scenario.product].file)];
  } else {
    let args = scenario.kind === 'core' ? [...commandContract(j.cell)[scenario.command].argv] :
      scenario.kind === 'literal-argv' ? [...(scenario.product === 'agentplugins' ? ['author'] : []), 'init', 'literal project ü',
        '--name', 'literal-project', '--template=skill', '--description', LITERAL_DESCRIPTION, '--format=json'] : ['version', '--format=json'];
    if (scenario.kind === 'core') {
      const core = commandContract(j.cell)[scenario.command];
      if (core.id.startsWith('installer/') && /\/(dry-run|add)$/.test(core.id)) args[1] = p.join(j.projects[core.product], core.lane);
    }
    const shim = p.join(s.prefix, ...(windows ? [] : ['bin']), scenario.product);
    // PowerShell's call operator with independently single-quoted array elements
    // preserves metacharacters. .cmd is exercised separately on core version rows.
    if (windows && scenario.kind === 'core' && commandContract(j.cell)[scenario.command].id === 'product-version') {
      const quote = v => { assert.ok(!/["%\r\n!]/.test(v), 'cmd fixed version tokens'); return '"' + v + '"'; };
      argv = [env.ComSpec, '/d', '/s', '/c', '"' + [shim + '.cmd', ...args].map(quote).join(' ') + '"'];
    } else if (windows) {
      const quote = v => "'" + v.replaceAll("'", "''") + "'";
      argv = ['C:\\Windows\\System32\\WindowsPowerShell\\v1.0\\powershell.exe', '-NoLogo', '-NoProfile', '-NonInteractive',
        '-Command', '& ' + [shim + '.ps1', ...args].map(quote).join(' ') + '; exit $LASTEXITCODE'];
    } else argv = [shim, ...args];
  }
  return { id: scenario.id, argv, cwd: s.cwd, env };
}
function tree(value, key, expectedRoot) {
  fields(value, ['root', 'sha256', 'entries'], 'tree snapshot'); hostAbsolute(value.root, key);
  if (expectedRoot !== undefined) agree(value.root, expectedRoot, 'snapshot root');
  assert.ok(Array.isArray(value.entries) && value.entries.length > 0 && value.entries.length <= 8192, 'tree entries bound');
  hash(value.sha256, 'tree'); agree(value.sha256, c.digest(c.encode(value.entries)), 'tree digest');
  const seen = new Set();
  for (const e of value.entries) {
    fields(e, e.kind === 'directory' ? ['path', 'mode', 'kind'] : ['path', 'mode', 'kind', 'size', 'sha256'], 'project tree entry');
    assert.ok(e.path === '.' || typeof e.path === 'string' && e.path.length <= 4096 && !e.path.startsWith('/') &&
      !e.path.includes('\\') && !e.path.split('/').some(x => !x || x === '.' || x === '..'), 'relative project entry');
    assert.ok(!seen.has(e.path.toLowerCase()), 'unique tree path'); seen.add(e.path.toLowerCase());
    assert.ok(['file', 'directory'].includes(e.kind), 'project regular file/directory');
    nonnegative(e.mode, 0o777, 'exact host mode');
    if (e.kind === 'file') { nonnegative(e.size, LIMIT, 'file bound'); hash(e.sha256, 'file'); }
    assert.ok(!/(^|\/)(hooks|plugin\.yaml)(\/|$)/.test(e.path), 'no generated legacy manifest/root hooks');
  }
  agree(value.entries[0].path, '.', 'tree root entry');
  for (const e of value.entries.slice(1)) {
    const parent = path.posix.dirname(e.path); assert.ok(value.entries.some(x => x.path === parent && x.kind === 'directory'), 'explicit parent including empty directories');
  }
  return value;
}
const CORRUPTION_BYTES = Buffer.from('C3 intentional owned cache corruption\n');
function binary(value, j, product, corrupted = false) {
  if (value === null) return;
  fields(value, ['path', 'sha256', 'size', 'mode'], 'observed binary'); hostAbsolute(value.path, j.cell);
  const expected = corrupted ? { sha256: c.digest(CORRUPTION_BYTES), size: CORRUPTION_BYTES.length } : j.subjects[product][cell(j.cell).target].binary;
  agree(value.sha256, expected.sha256, 'I inner binary bytes'); agree(value.size, expected.size, 'I inner binary size');
  agree(value.mode, j.cell.startsWith('windows-') ? 0o666 : 0o755, 'native executable host mode');
}
function observedState(value, j, corruptedProduct = null) {
  fields(value, ['projects', 'prefix', 'cache', 'client', 'state', 'inputs'], 'observed state');
  for (const k of ['projects', 'client', 'state', 'inputs']) hash(value[k], k);
  fields(value.prefix, cell(j.cell).products, 'prefix products'); fields(value.cache, cell(j.cell).products, 'cache products');
  for (const product of cell(j.cell).products) {
    const pkg = value.prefix[product];
    if (pkg !== null) {
      fields(pkg, ['tree', 'shims'], 'installed package'); hash(pkg.tree, 'package tree');
      const kinds = j.cell.startsWith('windows-') ? ['posix', 'cmd', 'powershell'] : ['posix']; list(pkg.shims, kinds.length, 'actual npm shim inventory');
      pkg.shims.forEach((shim, i) => {
        fields(shim, ['kind', 'path', 'sha256', 'mode', 'target'], 'npm shim'); agree(shim.kind, kinds[i], 'shim kind');
        hostAbsolute(shim.path, j.cell); hash(shim.sha256, 'shim'); nonnegative(shim.mode, 0o777, 'shim mode');
        if (j.cell.startsWith('windows-')) { agree(shim.target, null, 'Windows regular shim'); agree(shim.mode, 0o666, 'Windows shim mode'); }
        else { textValue(shim.target, 4096); assert.ok(shim.target.endsWith(`/bin/${product}.js`), 'npm link to product bin'); agree(shim.mode, 0o777, 'POSIX npm symlink'); }
      });
    }
    binary(value.cache[product], j, product, product === corruptedProduct);
  }
}
function expectedCachePath(j, roots, scenario, product) {
  const p = hostPath(j.cell), native = j.subjects[product][cell(j.cell).target].binary;
  return p.join(scopePaths(j, scenario, roots).home, '.cache', 'universal-agent-plugins', 'public-authoring-v2', contract.MODE,
    j.identity.commit, j.candidate_sha256, product, j.identity.versions[product], cell(j.cell).target, native.sha256, native.file);
}
function verifyObservedRow(row, scenario, j, roots, commands) {
  fields(row, ['id', 'command', 'argv', 'cwd', 'env', 'executable', 'runtime', 'stdout', 'stderr', 'status', 'signal',
    'before', 'after', 'observation', 'events', 'acquisitions', 'commits', 'downloads', 'native_launches', 'postinstall', 'interval', 'literal'], 'observed process row');
  agree(row.id, scenario.id, 'fixed scenario ID'); agree(row.command, scenario.command, 'core reference');
  const want = plannedInvocation(j, scenario, roots);
  for (const k of ['argv', 'cwd', 'env']) agree(row[k], want[k], `actual ${k}`);
  fields(row.executable, ['path', 'sha256'], 'observed executable'); agree(row.executable.path, want.argv[0], 'actual executable'); hash(row.executable.sha256, 'executable');
  const npm = ['install', 'reinstall', 'uninstall'].includes(scenario.kind);
  agree(row.runtime, npm ? j.tools.npm_node : j.tools.shim_node, 'observed runtime, not controller');
  if (npm) agree(row.executable.sha256, j.tools.npm_node.sha256, 'npm executing runtime hash');
  else if (!j.cell.startsWith('windows-')) {
    const pkg = row.before.prefix[scenario.product]; assert.ok(pkg, 'installed public shim required');
    agree(row.executable.sha256, pkg.shims[0].sha256, 'observed installed shim bytes');
  }
  for (const k of ['stdout', 'stderr']) { fields(row[k], ['size', 'sha256'], 'pinned output'); nonnegative(row[k].size, LIMIT, 'output size'); hash(row[k].sha256, 'output hash'); }
  if (scenario.kind === 'literal-argv') {
    fields(row.literal, ['root', 'description', 'manifest_sha256'], 'literal argv filesystem effect');
    agree(row.literal.root, hostPath(j.cell).join(want.cwd, 'literal project ü'), 'actual cwd-relative destination');
    agree(row.literal.description, LITERAL_DESCRIPTION, 'literal shell metacharacters preserved'); hash(row.literal.manifest_sha256, 'literal manifest');
  } else agree(row.literal, null, 'no invented literal effect');
  sidecar(row.observation); observedState(row.before, j, scenario.kind === 'repair' ? scenario.product : null); observedState(row.after, j);
  for (const state of [row.before, row.after]) for (const product of cell(j.cell).products) {
    if (state.cache[product]) agree(state.cache[product].path, expectedCachePath(j, roots, scenario, product), 'exact owned product cache path');
    if (state.prefix[product]) state.prefix[product].shims.forEach(shim => {
      const p = hostPath(j.cell), windows = j.cell.startsWith('windows-');
      agree(shim.path, p.join(scopePaths(j, scenario, roots).prefix, ...(windows ? [] : ['bin']), product + ({ posix: '', cmd: '.cmd', powershell: '.ps1' }[shim.kind])), 'exact installed npm shim path');
    });
  }
  for (const k of ['acquisitions', 'commits', 'downloads', 'native_launches']) nonnegative(row[k], 32, 'process effect count');
  list(row.interval, 2, 'observed monotonic interval'); row.interval.forEach(x => nonnegative(x, Number.MAX_SAFE_INTEGER, 'monotonic time'));
  assert.ok(row.interval[1] >= row.interval[0] && row.interval[1] - row.interval[0] <= 120000, 'fixed 120s scenario deadline');
  assert.ok(Array.isArray(row.events) && row.events.length <= 32, 'bounded observed boundaries');
  row.events.forEach(x => assert.ok(['shim', 'native', 'cache-waiter', 'lock-owner', 'reaped', 'cancel-delivered', 'repair-before-launch', 'locator-rejected'].includes(x), 'fixed boundary'));
  agree(new Set(row.events).size, row.events.length, 'unique boundaries'); agree(row.events.at(-1), 'reaped', 'descendants reaped after all observed boundaries');
  if (!npm) { agree(row.events[0], 'shim', 'shim is the first execution boundary');
    if (row.events.includes('native')) assert.ok(row.events.indexOf('shim') < row.events.indexOf('native'), 'shim before native');
  }
  agree(row.downloads, 0, 'no native download/fallback');
  for (const k of ['inputs', 'projects', 'client', 'state']) {
    const mutation = scenario.kind === 'core' && /\/(init|extra-skill)$/.test(commandContract(j.cell)[scenario.command].id);
    const installer = scenario.command !== null && commandContract(j.cell)[scenario.command].id.startsWith('installer/');
    if (!(k === 'projects' && mutation) && !(installer && ['client', 'state'].includes(k))) agree(row.after[k], row.before[k], `preserved ${k}`);
  }
  const canceled = ['cancel', 'waiter-cancel'].includes(scenario.kind);
  const status = scenario.command !== null ? commandContract(j.cell)[scenario.command].status : scenario.kind.startsWith('invalid-') ? 1 : 0;
  if (canceled) {
    assert.ok(row.events.includes('cancel-delivered') && row.events.includes(scenario.kind === 'waiter-cancel' ? 'cache-waiter' : 'native'), 'reached genuine cancellation boundary');
    const boundary = scenario.kind === 'waiter-cancel' ? 'cache-waiter' : 'native';
    assert.ok(row.events.indexOf(boundary) < row.events.indexOf('cancel-delivered'), 'reached boundary before cancellation delivery');
    agree(row.native_launches, scenario.kind === 'waiter-cancel' ? 0 : 1, 'cancelled waiter never launches native');
    if (scenario.kind === 'waiter-cancel') agree([row.acquisitions, row.commits], [0, 0], 'cancelled waiter never acquires or commits');
    const codes = { SIGINT: 130, SIGTERM: 143, CTRL_C_EVENT: 130, TerminateProcess: 1 };
    agree(row.status, codes[scenario.event], 'fixed cancellation exit mapping'); agree(row.signal, null, 'wrapper maps cancellation exit');
  } else { agree(row.status, status, 'fixed exit'); agree(row.signal, null, 'no unexpected signal'); }
  if (!npm && !canceled && !scenario.kind.startsWith('invalid-')) {
    assert.ok(row.events.includes('shim') && row.events.includes('native'), 'real shim/native boundaries');
    agree(row.native_launches, 1, 'one native process'); assert.ok(row.after.cache[scenario.product], 'native ran from checked cache');
    if (scenario.kind === 'core') { agree(row.after.cache, row.before.cache, 'core warm cache preserved'); agree([row.acquisitions, row.commits], [0, 0], 'core warm no acquisition/commit'); }
  }
  if (scenario.command !== null) {
    const core = commands[scenario.command];
    for (const [k, value] of Object.entries({ cwd: row.cwd, status: row.status, signal: row.signal })) agree(core[k], value, `core observed ${k}`);
    for (const k of ['stdout', 'stderr']) agree(row[k], { size: Buffer.byteLength(core[k]), sha256: c.digest(Buffer.from(core[k])) }, 'core exact observed output pin');
  }
  return row;
}
function recordRows(value, schema, j, expected, roots, commands) {
  fixed(value.schema, schema, 'scenario schema'); agree(value.cell, j.cell, 'scenario cell'); list(value.rows, expected.length, 'complete ordered scenario rows');
  value.rows.forEach((r, i) => verifyObservedRow(r, expected[i], j, roots, commands)); return value.rows;
}
function verifyNpmLifecycle(j, value, roots, commands = []) {
  fields(value, ['schema', 'cell', 'rows'], 'npm lifecycle');
  const expected = scenarioContract(j.cell).npm, rows = recordRows(value, 'authoring-public-npm-lifecycle/v1', j, expected, roots, commands);
  const previous = new Map(), originals = new Map();
  rows.forEach((r, i) => {
    const s = expected[i], product = s.product, peer = cell(j.cell).products.find(p => p !== product);
    if (previous.has(s.prefix)) agree(r.before, previous.get(s.prefix), 'continuous prefix lifecycle history');
    else { agree(Object.values(r.before.prefix), cell(j.cell).products.map(() => null), 'fresh prefix'); agree(Object.values(r.before.cache), cell(j.cell).products.map(() => null), 'cold prefix cache'); }
    previous.set(s.prefix, r.after);
    if (peer) { agree(r.after.prefix[peer], r.before.prefix[peer], 'peer package/shims/modes unchanged'); agree(r.after.cache[peer], r.before.cache[peer], 'peer cache/binary unchanged'); }
    const key = `${s.prefix}/${product}`;
    if (['install', 'reinstall'].includes(s.kind)) {
      agree(r.before.prefix[product], null, 'install into missing package'); assert.ok(r.after.prefix[product], 'installed package/shims');
      if (s.kind === 'install') originals.set(key, r.after.prefix[product]); else agree(r.after.prefix[product], originals.get(key), 'same tarball reinstall bytes/modes');
      if (product === 'plugin-kit-ai') {
        fields(r.postinstall, ['argv', 'runtime', 'acquisitions', 'commits', 'observation'], 'real kit postinstall');
        agree(r.postinstall.argv, [j.tools.npm_node.path, './lib/install.js'], 'genuine npm postinstall command');
        agree(r.postinstall.runtime, j.tools.npm_node, 'selected kit postinstall Node'); sidecar(r.postinstall.observation);
        const cold = r.before.cache[product] === null;
        agree(r.postinstall.acquisitions, cold ? 1 : 0, 'cold/warm postinstall acquisition'); agree(r.postinstall.commits, cold ? 1 : 0, 'postinstall cache commit');
        assert.ok(r.after.cache[product], 'postinstall checked cache');
      } else agree(r.postinstall, null, 'agent no install script');
    } else {
      agree(r.postinstall, null, 'no hidden postinstall');
      if (s.kind === 'uninstall') { assert.ok(r.before.prefix[product], 'installed before removal'); agree(r.after.prefix[product], null, 'removed package and all shims absent'); agree(r.after.cache, r.before.cache, 'uninstall preserves native cache'); }
      else { agree(r.after.prefix, r.before.prefix, 'probe read-only package'); assert.ok(r.events.includes('native') && r.native_launches === 1, 'peer/standalone real native command'); }
    }
  });
  return { npm_lifecycle: true };
}
function verifyCacheProcess(j, value, roots, commands) {
  fields(value, ['schema', 'cell', 'rows', 'finalization'], 'cache/process');
  const expected = scenarioContract(j.cell).cache, rows = recordRows(value, 'authoring-public-cache-process/v1', j, expected, roots, commands);
  const groups = new Map(), last = new Map();
  rows.forEach((r, i) => {
    const s = expected[i], p = s.product;
    agree(r.postinstall, null, 'shim has no npm postinstall');
    agree(r.after.prefix, r.before.prefix, 'shim preserves installed trees');
    for (const peer of cell(j.cell).products.filter(x => x !== p)) {
      if (s.kind === 'peer-overlap') { agree(r.before.cache[peer], null, 'both peer namespaces start cold'); if (r.after.cache[peer]) binary(r.after.cache[peer], j, peer); }
      else agree(r.after.cache[peer], r.before.cache[peer], 'cache namespace independence');
    }
    if (s.kind.startsWith('invalid-')) {
      agree(r.native_launches, 0, 'invalid locator never launches native'); agree(r.acquisitions, 0, 'invalid locator never acquires'); agree(r.commits, 0, 'invalid locator never commits');
      agree(r.after.cache, r.before.cache, 'invalid locator does not fallback even warm'); assert.ok(r.events.includes('locator-rejected'), 'locator rejection observed');
      if (s.kind === 'invalid-cold') agree(r.before.cache[p], null, 'invalid genuinely cold'); else assert.ok(r.before.cache[p], 'invalid genuinely warm');
    } else if (['cold', 'warm', 'repair', 'concurrent-cold', 'concurrent-warm', 'peer-overlap', 'waiter-owner'].includes(s.kind)) {
      assert.ok(r.after.cache[p], 'checked cache result');
      if (['cold', 'peer-overlap'].includes(s.kind)) { agree(r.before.cache[p], null, 'fresh cold acquisition'); agree(r.acquisitions, 1, 'one cold acquisition'); agree(r.commits, 1, 'one checked commit'); }
      if (['warm', 'concurrent-warm'].includes(s.kind)) { assert.ok(r.before.cache[p], 'warm cache present'); agree(r.after.cache, r.before.cache, 'warm exact cache identity'); agree([r.acquisitions, r.commits], [0, 0], 'warm zero acquisition/commit'); }
      if (s.kind === 'repair') { assert.ok(r.before.cache[p], 'observed fixed corruption before repair'); assert.ok(r.events.includes('repair-before-launch') && r.events.indexOf('repair-before-launch') < r.events.indexOf('native'), 'owned corruption repaired before any execution'); agree([r.acquisitions, r.commits], [1, 1], 'checked repair acquisition/commit'); }
      agree(r.native_launches, 1, 'one actual native command');
    }
    if (s.group) { if (!groups.has(s.group)) groups.set(s.group, []); groups.get(s.group).push(r); }
    if (!s.group && s.kind !== 'core') {
      if (last.has(s.cache)) {
        const before = s.kind === 'repair' ? { ...r.before.cache, [p]: r.after.cache[p] } : r.before.cache;
        agree(before, last.get(s.cache), 'continuous serial cache history around fixed owned corruption');
      }
      last.set(s.cache, r.after.cache);
    }
  });
  for (const [id, group] of groups) {
    assert.ok(Math.max(...group.map(r => r.interval[0])) < Math.min(...group.map(r => r.interval[1])), `real overlapping requests:${id}`);
    if (id.startsWith('cold-')) {
      list(group, 4, 'four simultaneous cold requests');
      group.forEach(r => agree(Object.values(r.before.cache), cell(j.cell).products.map(() => null), 'all four start from the same cold state')); agree(group.reduce((n, r) => n + r.acquisitions, 0), 1, 'one concurrent acquisition');
      agree(group.reduce((n, r) => n + r.commits, 0), 1, 'one concurrent commit');
      group.forEach(r => agree(r.after.cache, group[0].after.cache, 'one valid resulting cache'));
    }
    if (id === 'peer-overlap') { list(group, 2, 'both overlapping peer requests'); agree(group.map(r => r.id.split('/')[0]), cell(j.cell).products, 'distinct overlapping products'); }
    if (id.startsWith('waiter-')) assert.ok(group.some(r => r.events.includes('lock-owner')) && group.some(r => r.events.includes('cache-waiter')), 'genuine owner/waiter overlap');
  }
  fields(value.finalization, ['rows', 'descendants', 'locks', 'late_errors', 'observation'], 'descendant finalization');
  agree(value.finalization.rows, [...scenarioContract(j.cell).npm, ...expected, ...scenarioContract(j.cell).installer].map(s => s.id), 'finalization complete process inventory');
  for (const k of ['descendants', 'locks', 'late_errors']) agree(value.finalization[k], [], `no remaining ${k}`);
  sidecar(value.finalization.observation);
  return { cache_process: true, children_reaped: true };
}
const PLUGIN_SCHEMA = 'https://agent-plugins.org/schemas/1.0.0/plugin.schema.json';
const MCP_SCHEMA = 'https://agent-plugins.org/schemas/1.0.0/mcp.schema.json';
const PROFILES = freeze([
  ['agent-plugins/1.0.0', 'ff8ab5e392cc87bd88d87c060815a87490e51003', '97a658b7dca3ce1b4c2266b95da300fa51d9dc4ade59d73168e5f9104272da18'],
  ['agent-skills/2026-09-06', '69ef37e9424c0a7ea9dd2293b559e43ec8176379', 'b9079c0c10b7930e8c6a20ff2bc10cda2a3343c55185120e3f1116a1a529b220'],
  [PLUGIN_SCHEMA, '1.0.0', '0a4aad95ce337878ad38802ebf0daa3fde76abe3f65400c86bcbb1ec0b3ab883'],
  [MCP_SCHEMA, '1.0.0', '6539175bfcdf43085855183e86da40ea94b166547a72b47ae9a0a390516d3acb'],
  ['author-document-bounds/v1', '1', '4b8ab8fd50481ccd1a0b777dcbbfa06cf89516a5ea61ce09d56d6dd6a2c43004']
].map(([id, revision, digest]) => ({ id, revision, digest: 'sha256:' + digest })));
const SURFACE = freeze(['capabilities', 'compat', 'doctor', 'init', 'inspect', 'skills.init', 'skills.validate', 'test', 'validate', 'version'].map(x => 'author.' + x));
function clientFacts() {
  return [
    ['chatgpt', 'compatibility_projection', 'manual', 'projected', 'unsupported', 'projected', 'unsupported'],
    ['claude', 'compatibility_projection', 'automatic', 'projected', 'projected', 'unsupported', 'unsupported'],
    ['cline', 'native', 'automatic', 'native', 'native', 'unsupported', 'unsupported'],
    ['codex', 'compatibility_projection', 'manual', 'projected', 'projected', 'unsupported', 'unsupported'],
    ['copilot', 'native', 'manual', 'native', 'native', 'unsupported', 'native'],
    ['cursor', 'native', 'manual', 'native', 'native', 'unsupported', 'native'],
    ['gemini', 'native', 'manual', 'native', 'native', 'unsupported', 'unsupported'],
    ['kiro', 'native', 'manual', 'native', 'native', 'unsupported', 'unsupported'],
    ['opencode', 'prepared_package', 'automatic', 'prepared', 'prepared', 'unsupported', 'unsupported'],
    ['vscode', 'prepared_package', 'manual', 'prepared', 'prepared', 'unsupported', 'prepared'],
    ['windsurf', 'prepared_package', 'manual', 'prepared', 'prepared', 'unsupported', 'prepared']
  ].map(([client_id, package_mode, activation_mode, skill_support, mcp, app_support, extension_support]) =>
    ({ client_id, package_mode, activation_mode, scopes: ['user'], skill_support, mcp_transports: { stdio: mcp, 'streamable-http': mcp, sse: mcp }, app_support, extension_support }));
}
function optionalFields(v, required, optional, label) {
  assert.ok(v && typeof v === 'object', label);
  fields(v, [...required, ...optional.filter(k => Object.hasOwn(v, k))], label);
}
function outputJSON(text) {
  textValue(text); assert.ok(text.length > 0, 'one JSON output');
  // Tokenize only to detect duplicate object keys/depth. JSON.parse owns syntax.
  const stack = []; let match;
  const tokens = /"(?:[^"\\\x00-\x1f]|\\(?:["\\/bfnrt]|u[0-9a-fA-F]{4}))*"|[{}\[\]]/g;
  while ((match = tokens.exec(text))) {
    const token = match[0];
    if (token === '{' || token === '[') { stack.push(token === '{' ? new Set() : null); assert.ok(stack.length <= 16, 'output depth'); }
    else if (token === '}' || token === ']') stack.pop();
    else if (/^\s*:/.test(text.slice(tokens.lastIndex))) {
      const keys = stack.at(-1), key = JSON.parse(token); assert.ok(keys && !keys.has(key), 'duplicate JSON result key'); keys.add(key);
    }
  }
  return JSON.parse(text);
}
function componentFacts(lane, extra = true, malformed = false) {
  const rows = [], skill = lane === 'skill' || lane.startsWith('hybrid-');
  const add = (type, name, requirements = [], status = 'pass') => rows.push({ id: 'sha256:' + c.digest(Buffer.from(`${type === 'skill' ? 'skill' : 'mcp'}:${name}`)), type, status, requirements });
  if (skill) add('skill', lane);
  if (extra) add('skill', 'extra-skill', [], malformed ? 'fail' : 'pass');
  if (lane !== 'skill') add(lane.endsWith('stdio') ? 'mcp_stdio' : 'mcp_streamable-http', lane,
    lane.endsWith('stdio') ? ['executable_unresolved', 'executable_path'] : ['remote_endpoint_uncontacted']);
  return rows.sort((a, b) => a.id.localeCompare(b.id));
}
const ASSESSMENTS = ['compatibility', 'toolchain', 'loadability', 'normative_conformance', 'host_safety', 'authoring_readiness', 'release_policy', 'runtime_evidence'];
function authorResult(v, want, j) {
  const d = v.data, version = want.id === 'engine-version' || want.id === 'product-version';
  const optional = ['help', 'root', 'inspection', 'commands', 'withheld_path_ids', 'error', 'clients', 'capabilities', 'doctor_checks', 'product', 'product_version'];
  optionalFields(d, [...ASSESSMENTS, 'schema', 'engine', 'revision', 'command', 'mode', 'identity', 'coverage', 'profiles', 'schema_ids', 'findings',
    'components', 'checks', 'committed', 'affected_paths', 'authoring_schema_version', 'engine_version', 'requested', 'effects', 'next_actions'], optional, 'author result fields');
  agree(d.schema, 'agentplugins-authoring-report/v1', 'report schema'); agree(d.engine, 'standard-first-slice/1', 'engine');
  agree(d.engine_version, d.engine, 'engine version'); agree(d.revision, j.identity.commit, 'engine F'); agree(d.authoring_schema_version, 1, 'author schema');
  const args = want.argv.slice(want.product === 'agentplugins' ? 1 : 0);
  const op = want.id === 'retired-v1' ? 'author' : args[0] === '--help' ? 'author' : `author.${args[0]}${args[0] === 'skills' ? '.' + args[1] : ''}`;
  agree(v.command, op, 'operation'); agree(d.command, op, 'report operation');
  const mutation = op === 'author.init' || op === 'author.skills.init';
  agree(d.mode, mutation ? 'local_mutation' : 'read', 'operation mode'); agree(d.requested, { operation: op, mode: d.mode }, 'requested operation');
  const committed = /\/(init|extra-skill)$/.test(want.id);
  agree(d.committed, committed, 'commit boundary'); fields(d.effects, ['attempted', 'committed'], 'effects');
  agree(d.effects.committed, committed, 'public effects'); assert.equal(typeof d.effects.attempted, 'boolean');
  assert.ok(Array.isArray(d.findings) && d.findings.length <= 256, 'bounded findings');
  const ids = new Set();
  for (const f of d.findings) {
    optionalFields(f, ['id', 'code', 'layer', 'rule', 'severity'], ['location', 'item_id'], 'finding');
    digestID(f.id); assert.ok(!ids.has(f.id), 'unique diagnostics'); ids.add(f.id);
    Object.values(f).forEach(x => textValue(x, 4096)); assert.ok(['error', 'warning', 'info'].includes(f.severity), 'diagnostic severity');
  }
  for (const name of ASSESSMENTS) {
    fields(d[name], ['status', 'finding_ids'], 'separate policy assessment');
    assert.ok(['pass', 'fail', 'not_evaluated'].includes(d[name].status), 'policy state');
    assert.ok(Array.isArray(d[name].finding_ids) && d[name].finding_ids.every(id => ids.has(id)), 'assessment references real findings');
    agree([...new Set(d[name].finding_ids)].sort(), d[name].finding_ids, 'canonical finding references');
  }
  agree(d.runtime_evidence, { status: 'not_evaluated', finding_ids: [] }, 'offline runtime not evaluated');
  agree(d.release_policy, { status: 'not_evaluated', finding_ids: [] }, 'release policy not inferred');
  fields(d.coverage, ['components_requested', 'skills_enumerated', 'inventory_complete', 'tree_complete', 'plugin', 'mcp', 'skills', 'filesystem', 'facts_complete'], 'coverage');
  for (const k of ['components_requested', 'skills_enumerated', 'inventory_complete', 'tree_complete', 'facts_complete']) assert.equal(typeof d.coverage[k], 'boolean');
  for (const k of ['plugin', 'mcp', 'skills', 'filesystem']) assert.ok(['pass', 'fail', 'not_evaluated'].includes(d.coverage[k]), 'coverage state');
  optionalFields(d.identity, ['scope_algorithm', 'read_profile', 'tree_exclusions'], ['scope_digest', 'tree_algorithm', 'tree_digest', 'manifest_digest'], 'project identity');
  const project = want.lane && !want.id.endsWith('/existing') && want.id !== 'installer-flag';
  if (project) {
    agree(d.effects.attempted, true, 'entered actual project operation');
    agree(d.identity.scope_algorithm, 'agentplugins-captured-input-sha256-v1', 'scope algorithm');
    agree(d.identity.tree_algorithm, 'agentplugins-tree-sha256-v1', 'tree algorithm');
    agree(d.identity.read_profile, `packageview-local-${cell(j.cell).target.split('-')[0]}-v1`, 'recorded host read profile');
    agree(d.identity.tree_exclusions, ['root .git', 'root non-directory .plugin-kit-ai.lock'], 'exact tree exclusions');
    for (const k of ['scope_digest', 'tree_digest', 'manifest_digest']) digestID(d.identity[k]);
    agree(d.profiles, PROFILES, 'embedded profiles');
    agree(d.schema_ids, (want.lane === 'skill' ? [PLUGIN_SCHEMA] : [MCP_SCHEMA, PLUGIN_SCHEMA]).sort(), 'exact schema inventory');
    for (const k of ['components_requested', 'skills_enumerated', 'inventory_complete', 'tree_complete', 'facts_complete']) agree(d.coverage[k], true, 'complete captured facts');
    const malformed = want.id === 'malformed-skill';
    for (const k of ['plugin', 'filesystem']) agree(d.coverage[k], 'pass', 'complete core/filesystem coverage');
    agree(d.coverage.mcp, want.lane === 'skill' ? 'not_evaluated' : 'pass', 'MCP coverage');
    agree(d.coverage.skills, 'pass', 'complete Skills capture separate from conformance');
    agree(d.loadability.status, 'pass', 'valid Skill sibling remains loadable');
    agree(d.normative_conformance.status, malformed ? 'fail' : 'pass', 'normative conformance');
    agree(d.host_safety.status, 'pass', 'host safety separate');
    agree(d.authoring_readiness.status, malformed ? 'fail' : 'pass', 'authoring readiness');
    agree(d.components, componentFacts(want.lane, !want.id.endsWith('/init'), malformed), 'exact Skill/MCP components and boundary');
    fields(d.inspection, ['name', 'version', 'schema', 'components'], 'inspection');
    agree([d.inspection.name, d.inspection.version, d.inspection.schema], [want.lane, '0.1.0', PLUGIN_SCHEMA], 'package identity');
    const names = d.inspection.components.map(x => {
      optionalFields(x, ['id', 'type'], ['name', 'namespace', 'executable', 'executable_kind'], 'display component');
      digestID(x.id); Object.values(x).forEach(v => textValue(v, 256)); return [x.id, x.type, x.name];
    });
    agree(names, d.components.map(x => [x.id, x.type, x.id === 'sha256:' + c.digest(Buffer.from('skill:extra-skill')) ? 'extra-skill' : want.lane]), 'component display names');
  } else {
    agree(d.profiles, [], 'no invented project profiles'); agree(d.schema_ids, [], 'no invented project schemas');
    agree(d.components, [], 'no project components');
  }
  if (d.doctor_checks) { assert.ok(Array.isArray(d.doctor_checks) && d.doctor_checks.length <= 256); d.doctor_checks.forEach(x => { optionalFields(x, ['id', 'status', 'action'], ['item_id'], 'doctor check'); Object.values(x).forEach(v => textValue(v, 8192)); assert.ok(['pass', 'fail', 'not_evaluated'].includes(x.status)); }); }
  if (d.withheld_path_ids) { assert.ok(Array.isArray(d.withheld_path_ids) && d.withheld_path_ids.length <= 256); d.withheld_path_ids.forEach(digestID); }
  if (d.root !== undefined) { textValue(d.root, 4096); assert.fail('absolute/root disclosure was not requested'); }
  if (want.id.endsWith('/doctor')) agree(d.toolchain.status, want.lane === 'skill' ? 'pass' : 'not_evaluated', 'doctor offline boundary');
  else agree(d.toolchain.status, 'not_evaluated', 'no implicit toolchain proof');
  if (want.id.endsWith('/compat')) {
    agree(d.compatibility.status, 'pass', 'static compatibility'); list(d.clients, 2, 'two explicit clients');
    d.clients.forEach((client, i) => {
      fields(client, ['client_id', 'capabilities', 'components', 'limitations'], 'compat client'); agree(client.client_id, ['claude', 'codex'][i], 'client order');
      agree(client.capabilities, clientFacts().find(x => x.client_id === client.client_id), 'client capability facts');
      const counts = { skill: 0, mcp_server: 0 };
      const components = d.components.map(x => ({ kind: x.type === 'skill' ? 'skill' : 'mcp_server', index: 0, support: 'projected' }))
        .sort((a, b) => a.kind.localeCompare(b.kind)).map(x => ({ ...x, index: ++counts[x.kind] }));
      agree(client.components.map(({ kind, index, support }) => ({ kind, index, support })), components, 'compat exact components');
      client.components.forEach(x => optionalFields(x, ['kind', 'index', 'support'], ['limitations'], 'compat component'));
      agree(client.limitations, ['static_adapter_support_only', 'installation_not_checked', 'authentication_not_checked', 'runtime_not_checked', 'client_version_not_checked', 'catalog_publication_not_checked', ...(i === 1 ? ['manual_activation_required'] : [])], 'compat evidence limits');
    });
  } else agree(d.compatibility.status, 'not_evaluated', 'no implicit compatibility');
  if (want.id === 'capabilities') {
    fields(d.capabilities, ['schemas', 'profiles', 'clients', 'commands', 'evidence_limits'], 'capability inventory');
    agree(d.capabilities.schemas, PROFILES.slice(2, 4).map(({ id, digest }) => ({ id, digest })), 'embedded schema pins');
    agree(d.capabilities.profiles, PROFILES, 'capability profiles'); agree(d.capabilities.clients, clientFacts(), 'complete client inventory');
    agree(d.capabilities.commands, SURFACE, 'complete capability commands');
    agree(d.capabilities.evidence_limits, ['static_only', 'no_path_lookup', 'no_executable_version_probe', 'no_runtime_or_oauth_evidence', 'native_files_metadata_only'], 'capability limits');
  }
  if (['author-help', 'capabilities', 'engine-version', 'product-version'].includes(want.id)) agree(d.commands, SURFACE, 'implemented command surface');
  if (version) agree([d.product, d.product_version], [want.product, j.identity.versions[want.product]], 'kit product version');
  if (d.help) { fields(d.help, ['use', 'flags', 'guidance'], 'help'); textValue(d.help.use, 4096); textValue(d.help.guidance, 8192); assert.ok(Array.isArray(d.help.flags) && d.help.flags.every(x => typeof x === 'string'), 'help flags'); }
  if (d.error) { fields(d.error, ['code', 'action'], 'operation error'); textValue(d.error.code, 256); textValue(d.error.action, 8192); }
  for (const k of ['affected_paths', 'next_actions', 'checks']) assert.ok(Array.isArray(d[k]) && d[k].length <= 256, 'bounded result lists');
  d.affected_paths.forEach(x => { textValue(x, 4096); assert.ok(!x.startsWith('/') && !x.split('/').includes('..'), 'relative affected path'); });
  if (!committed) agree(d.affected_paths, [], 'read/failed operation no affected files'); else assert.ok(d.affected_paths.length > 0, 'committed actual paths');
  d.next_actions.forEach(x => { optionalFields(x, ['code', 'message'], ['operation'], 'next action'); Object.values(x).forEach(v => textValue(v, 8192)); });
  d.checks.forEach(x => { fields(x, ['id', 'status', 'finding_ids'], 'static check'); textValue(x.id, 256); assert.ok(['pass', 'fail', 'not_evaluated'].includes(x.status)); assert.ok(x.finding_ids.every(id => ids.has(id))); });
  if (want.id.endsWith('/test')) agree(d.checks.map(x => [x.id, x.status]), [['portable_configuration', 'pass'], ['package_hygiene', 'pass'], ['static_skills', 'pass'], ['static_mcp', want.lane === 'skill' ? 'not_evaluated' : 'pass'], ['runtime', 'not_evaluated']], 'complete static checks');
}
function normalizeResult(v) {
  const copy = structuredClone(v);
  // Only product/version fields and displayed invocation prefix are allowed to differ.
  delete copy.data.product; delete copy.data.product_version;
  if (copy.data.help) copy.data.help.use = copy.data.help.use.replace(/^(plugin-kit-ai|agentplugins author)(?= |$)/, 'AUTHOR');
  return copy;
}
// Scenario records stay below 1MiB; ordered process rows use bounded transcript
// sidecars. Long supported workspace paths must not inflate the record envelope.
function evidenceFiles(name, value) {
  const files = {};
  if (['npm-lifecycle.json', 'cache-process.json', 'installer.json'].includes(name) && Array.isArray(value.rows)) {
    const refs = []; let shard = [], size = 3;
    const flush = () => {
      if (!shard.length) return;
      const file = `sidecars/${name.slice(0, -5)}-rows-${refs.length}.json`, bytes = c.encode(shard);
      assert.ok(bytes.length <= TRANSCRIPT_LIMIT, '16MiB transcript shard'); files[file] = bytes;
      refs.push({ path: file, size: bytes.length, sha256: c.digest(bytes) }); shard = []; size = 3;
    };
    for (const row of value.rows) {
      const bytes = c.encode(row); assert.ok(bytes.length <= LIMIT, '1MiB process record');
      if (size + bytes.length + 1 > TRANSCRIPT_LIMIT) flush();
      shard.push(row); size += bytes.length + 1;
    }
    flush(); value = { ...value, rows: { shards: refs } };
  }
  const bytes = c.encode(value);
  assert.ok(bytes.length <= (name === 'commands.json' ? TRANSCRIPT_LIMIT : LIMIT), 'bounded evidence file');
  files[name] = bytes; return files;
}
function expandEvidence(name, value, root, budget = { size: 0 }) {
  if (!['npm-lifecycle.json', 'cache-process.json', 'installer.json'].includes(name) || value.rows === undefined || Array.isArray(value.rows)) return value;
  fields(value.rows, ['shards'], 'ordered row transcript index');
  assert.ok(Array.isArray(value.rows.shards) && value.rows.shards.length <= 128, 'bounded shard inventory');
  const rows = [];
  value.rows.shards.forEach((ref, i) => {
    sidecar(ref); agree(ref.path, `sidecars/${name.slice(0, -5)}-rows-${i}.json`, 'fixed row shard name');
    budget.size += ref.size; assert.ok(budget.size <= AGGREGATE_LIMIT, '128MiB transcript aggregate');
    const bytes = pin(path.join(root, ref.path), ref.sha256, TRANSCRIPT_LIMIT); agree(bytes.length, ref.size, 'row shard size');
    const shard = bounded(bytes, TRANSCRIPT_LIMIT); assert.ok(Array.isArray(shard) && shard.length > 0, 'nonempty row shard');
    for (const row of shard) { assert.ok(c.encode(row).length <= LIMIT, '1MiB process record'); rows.push(row); }
  });
  const expanded = { ...value, rows };
  agree(c.encode(value), evidenceFiles(name, expanded)[name], 'canonical row shard partition'); return expanded;
}
// Fixed publicInit lanes only: exact scaffold bytes, not a configurable template engine.
function generatedFiles(lane) {
  assert.ok(LANES.includes(lane), 'fixed generated lane');
  const hybrid = lane.startsWith('hybrid-'), stdio = lane.endsWith('stdio'), remote = lane.endsWith('remote');
  const description = hybrid ? 'An Agent Plugins package with a Skill and an MCP server.' : lane === 'skill' ? 'A Skill for documentation and task guidance.' : remote ? 'An Agent Plugins package with a remote MCP server.' : 'An Agent Plugins package with a local Node MCP server.';
  const json = value => JSON.stringify(value, null, 2) + '\n';
  const files = {
    'plugin.json': json({ $schema: PROFILES[2].id, description, name: lane, version: '0.1.0' }),
    '.gitignore': 'node_modules/\n.DS_Store\n',
    'README.md': `# ${lane}\n\n${description}\n\nThis package uses Agent Plugins 1.0: \`plugin.json\`, with portable components in \`skills/\` and/or \`mcp.json\`.\n` +
      (stdio ? '\nThe stdio server requires Node >=22 and the official MCP SDK pinned in package-lock.json. Dependency installation and runtime execution are separate, explicit author actions. Creation performs neither; runtime behavior has not been tested.\n' : remote ? '\nThe remote MCP URL is configuration only. Creation does not contact the endpoint or verify authentication or runtime behavior.\n' : ''),
    'skills/extra-skill/SKILL.md': '---\nname: "extra-skill"\ndescription: "Use for extra documentation requests"\n---\n\n# extra-skill\n\nUse for extra documentation requests\n'
  };
  if (hybrid || lane === 'skill') files[`skills/${lane}/SKILL.md`] = `---\nname: ${lane}\ndescription: ${JSON.stringify(description)}\n---\n\n# ${lane}\n\n${description}\n\nUse this skill when the request matches its description. Clarify missing requirements before taking action and report the result.\n`;
  if (remote || stdio) files['mcp.json'] = json({ $schema: PROFILES[3].id, mcpServers: { [lane]: stdio ? { args: ['${PLUGIN_ROOT}/src/server.mjs'], command: 'node', type: 'stdio' } : { type: 'streamable-http', url: 'https://docs.example.com/mcp' } } });
  if (stdio) {
    // Existing source-frozen embedded npm fixtures; never install or resolve them.
    for (const name of ['package.json', 'package-lock.json']) {
      const bytes = c.readFile(path.resolve(__dirname, '../../../cli/plugin-kit-ai/internal/authoring/scaffold/templates', name), LIMIT).toString('utf8');
      const needle = name === 'package.json' ? '"name":"agent-plugin-template"' : '"name": "agent-plugin-template"';
      agree(bytes.split(needle).length - 1, name === 'package.json' ? 1 : 2, 'fixed embedded root names');
      files[name] = bytes.split(needle).join(needle.replace('agent-plugin-template', lane));
    }
    files['src/server.mjs'] = `import { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js';\nimport { StdioServerTransport } from '@modelcontextprotocol/sdk/server/stdio.js';\n\nconst server = new McpServer({ name: "${lane}", version: '0.1.0' });\nserver.registerTool('hello', { description: 'Return a greeting', inputSchema: {} }, async () => ({\n  content: [{ type: 'text', text: "Hello from ${lane}!" }],\n}));\nawait server.connect(new StdioServerTransport());\n`;
  }
  return Object.fromEntries(Object.entries(files).map(([name, body]) => [name, Buffer.from(body)]));
}
function generatedTreeIdentity(entries, lane, key) {
  const files = generatedFiles(lane), directories = new Set(['.']);
  for (const name of Object.keys(files)) for (let dir = path.posix.dirname(name); dir !== '.'; dir = path.posix.dirname(dir)) directories.add(dir);
  const actual = entries.filter(e => e.path === lane || e.path.startsWith(lane + '/')).map(e => ({ ...e, path: e.path === lane ? '.' : e.path.slice(lane.length + 1) }));
  agree(actual.map(e => e.path).sort(), [...directories, ...Object.keys(files)].sort(), 'fixed generated file/directory closure');
  const windows = cell(key).target.startsWith('windows-');
  for (const e of actual) {
    const directory = directories.has(e.path);
    agree(e.kind, directory ? 'directory' : 'file', 'generated entry type');
    // Node's Windows stat reports writable files 0666 and directories 0777;
    // packageview uses that host read profile, including file execute bits.
    agree(e.mode, windows ? directory ? 0o777 : 0o666 : directory ? 0o755 : 0o644, 'generated host mode');
    if (!directory) { agree(e.size, files[e.path].length, 'generated file size'); agree(e.sha256, c.digest(files[e.path]), 'generated file content'); }
  }
  // Ordinary agentplugins-tree-sha256-v1 framing (DigestCaptured). Root is
  // excluded; directories, including empty ones, are entries. Bytes below are
  // bound to captured size/hash above; claimed report digests are never inputs.
  const chunks = [], length = n => { const b = Buffer.alloc(8); b.writeBigUInt64BE(BigInt(n)); return b; };
  const frame = value => { const b = Buffer.from(value); chunks.push(length(b.length), b); };
  frame('agentplugins.package-tree\0sha256\0v1');
  for (const e of actual.filter(e => e.path !== '.').sort((a, b) => a.path < b.path ? -1 : a.path > b.path ? 1 : 0)) {
    const body = e.kind === 'file' ? files[e.path] : Buffer.alloc(0);
    for (const field of ['entry', e.path, e.kind, e.kind === 'directory' ? '040000' : e.mode & 0o111 ? '100755' : '100644', '']) frame(field);
    chunks.push(length(body.length), body);
  }
  return 'sha256:' + c.digest(Buffer.concat(chunks));
}
function verifyResults(j, evidence) {
  for (const name of EVIDENCE) if (Object.hasOwn(evidence, name)) evidenceFiles(name, evidence[name]);
  const expected = commandContract(j.cell), rows = evidence['commands.json']; list(rows, expected.length, 'exact ordered C3 core command rows');
  const payloads = new Map(), identityByProject = new Map();
  rows.forEach((row, i) => {
    const want = expected[i]; fields(row, ['product', 'id', 'argv', 'cwd', 'status', 'signal', 'stdout', 'stderr'], 'C3 command result');
    for (const k of ['product', 'id', 'argv', 'status']) agree(row[k], want[k], `command ${k}`);
    agree(row.cwd, commandCwd(j, want), 'fixed cwd'); agree(row.signal, null, 'complete command'); agree(row.stderr, '', 'clean stderr'); textValue(row.stdout);
    if (want.id === 'product-help') { assert.ok(row.stdout.includes(want.product) && row.stdout.length > 50, 'real product help'); return; }
    const v = outputJSON(row.stdout); fields(v, ['schema_version', 'command', 'result', 'data'], 'one output envelope');
    agree(v.schema_version, 1, 'JSON schema'); agree(v.result, want.status === 0 ? 'success' : 'failure', 'result status');
    if (want.id.startsWith('installer/')) return; // Checked by the mandatory fixed public installer facade below.
    if (want.author || want.product === 'plugin-kit-ai') authorResult(v, want, j);
    else { agree(v.command, 'version', 'agent product version operation'); assert.ok(v.data && typeof v.data === 'object'); agree(v.data.version, j.identity.versions.agentplugins, 'agent product version'); }
    if (want.lane && !/\/(init|existing)$/.test(want.id) && !['malformed-skill', 'installer-flag'].includes(want.id)) {
      const key = `${want.product}/${want.lane}`;
      if (identityByProject.has(key)) agree(v.data.identity, identityByProject.get(key), 'unchanged read-only project identity');
      else identityByProject.set(key, v.data.identity);
    }
    payloads.set(`${want.product}/${want.id}`, v);
  });
  fields(evidence['projects.json'], cell(j.cell).products, 'final canonical projects');
  for (const p of cell(j.cell).products) {
    const snapshot = tree(evidence['projects.json'][p], j.cell, j.projects[p]);
    assert.ok(snapshot.entries.every(e => e.path === '.' || LANES.some(lane => e.path === lane || e.path.startsWith(lane + '/'))), 'only generated lane entries');
    agree(snapshot.entries.filter(e => e.kind === 'directory' && e.path !== '.' && !e.path.includes('/')).map(e => e.path).sort(), [...LANES].sort(), 'exact five canonical projects');
    for (const lane of LANES) {
      const capturedDigest = generatedTreeIdentity(snapshot.entries, lane, j.cell);
      const identity = identityByProject.get(`${p}/${lane}`); assert.ok(identity, 'project read identity present');
      const manifest = snapshot.entries.find(e => e.path === `${lane}/plugin.json`);
      agree(identity.tree_digest, capturedDigest, 'captured generated tree identity');
      agree(identity.manifest_digest, 'sha256:' + manifest.sha256, 'observed manifest identity');
    }
  }
  if (cell(j.cell).products.length === 2) {
    for (const want of expected.filter(r => r.product === 'agentplugins' && r.author))
      agree(normalizeResult(payloads.get(`agentplugins/${want.id}`)), normalizeResult(payloads.get(`plugin-kit-ai/${want.id}`)), 'pair JSON parity including diagnostics/readiness/digests');
    agree(evidence['projects.json'].agentplugins.entries, evidence['projects.json']['plugin-kit-ai'].entries, 'pair generated trees bytes/modes/empty directories');
  }
  return { fixed_commands: true, pair_parity: true, projects_preserved: true };
}
function verifyInstaller(j, value, roots, commands, observation, facade) {
  fields(value, ['schema', 'cell', 'rows', 'assessment', 'readbacks'], 'installer evidence');
  recordRows(value, 'authoring-public-installer/v1', j, scenarioContract(j.cell).installer, roots, commands);
  if (cell(j.cell).node === 18) { agree(value.assessment, null, 'kit has no installer assessment'); agree(value.readbacks, [], 'kit has no installer readbacks'); return { production_installer: true }; }
  // Assessment/readback internals belong exclusively to the reviewed owner facade.
  // They are bounded opaque sidecars here, not caller booleans or copied security logic.
  sidecar(value.assessment); list(value.readbacks, 18, 'all eighteen installer readbacks'); value.readbacks.forEach(sidecar);
  const checked = facade.verifyPublicInstaller({ cell: j.cell, identity: j.identity, subjects: j.subjects,
    commands: value.rows, assessment: value.assessment, readbacks: value.readbacks, observation });
  agree(checked, { assessment: value.assessment, readbacks: value.readbacks }, 'checked installer evidence, never boolean success');
  return { production_installer: true };
}
function journeyRoots(j) {
  const p = hostPath(j.cell), output = p.dirname(p.dirname(j.projects[cell(j.cell).products[0]]));
  return rootsFor(output, j.cell);
}
function verifyJourney(local) {
  const { record: j, evidence } = local, roots = journeyRoots(j);
  const result = verifyResults(j, evidence);
  Object.assign(result, verifyNpmLifecycle(j, evidence['npm-lifecycle.json'], roots, evidence['commands.json']),
    verifyCacheProcess(j, evidence['cache-process.json'], roots, evidence['commands.json']));
  const api = requireFacades(j.cell), all = [...evidence['npm-lifecycle.json'].rows, ...evidence['cache-process.json'].rows, ...evidence['installer.json'].rows];
  // All core effects share the same original project roots. Bind observed history
  // to the retained final snapshots, including every read and installer row.
  const coreRows = all.filter(r => r.command !== null).sort((a, b) => {
    const order = [...scenarioContract(j.cell).cache, ...scenarioContract(j.cell).installer].map(s => s.command);
    return order.indexOf(a.command) - order.indexOf(b.command);
  });
  let projectState;
  for (const r of coreRows) {
    if (projectState !== undefined) agree(r.before.projects, projectState, 'continuous original project history');
    const w = commandContract(j.cell)[r.command];
    if (/\/(init|extra-skill)$/.test(w.id)) assert.notEqual(r.before.projects, r.after.projects, 'actual authoring mutation changed tree');
    projectState = r.after.projects;
  }
  agree(projectState, c.digest(c.encode(evidence['projects.json'])), 'final original project snapshots');
  for (const scenario of [...scenarioContract(j.cell).npm, ...scenarioContract(j.cell).cache]) {
    if (['install', 'uninstall', 'reinstall', 'literal-argv', 'cancel', 'waiter-cancel', 'invalid-cold', 'invalid-warm', 'core'].includes(scenario.kind)) continue;
    const row = all.find(r => r.id === scenario.id), version = evidence['commands.json'].find(r => r.product === scenario.product && r.id === 'product-version');
    agree(row.stdout, { size: Buffer.byteLength(version.stdout), sha256: c.digest(Buffer.from(version.stdout)) }, 'supplementary native version result');
    agree(row.stderr, { size: 0, sha256: c.digest(Buffer.alloc(0)) }, 'supplementary clean stderr');
  }
  const finalization = evidence['cache-process.json'].finalization;
  const observed = api['public-process-observation'].verifyPublicObservation({ cell: j.cell, tools: j.tools, subjects: j.subjects, rows: all, finalization });
  agree(observed, { rows: all, finalization }, 'checked observation evidence, never boolean success');
  Object.assign(result, verifyInstaller(j, evidence['installer.json'], roots, evidence['commands.json'], finalization, api['public-installer-evidence']));
  agree(Object.fromEntries(ASSERTIONS.map(k => [k, result[k]])), j.assertions, 'all seven assertions recomputed');
  return local;
}
function sourceSeal(repo, source, provision) {
  const git = provision.controllers[cell(provision.key).target].git.path;
  const run = args => require('node:child_process').execFileSync(git, args, { cwd: repo, env: { PATH: path.dirname(git), LANG: 'C', GIT_CONFIG_NOSYSTEM: '1', GIT_CONFIG_GLOBAL: process.platform === 'win32' ? 'NUL' : '/dev/null' }, maxBuffer: TRANSCRIPT_LIMIT }).toString();
  agree(run(['rev-parse', 'HEAD']).trim(), source, 'exact executing source F'); agree(run(['status', '--porcelain=v1', '--untracked-files=all']), '', 'clean exact source');
  return run(['ls-files', '-z']).split('\0').filter(Boolean).map(name => {
    const file = path.join(repo, name), st = fs.lstatSync(file);
    assert.ok(st.isFile() || st.isSymbolicLink(), 'source file/link');
    return { name, mode: st.mode, sha256: c.digest(st.isSymbolicLink() ? Buffer.from(fs.readlinkSync(file)) : fs.readFileSync(file)) };
  });
}
function admitProducerInputs(r, api) {
  const inputBytes = c.readFile(r.input_file, LIMIT), input = contract.decodeInputs(inputBytes);
  agree(r.selected, { tag: input.products.agentplugins.tag, ref: `refs/tags/${input.products.agentplugins.tag}`, source: input.identity.commit, versions: input.identity.versions }, 'producer selected I');
  agree(r.workflow_sha, input.identity.commit, 'producer F'); producer(r.producer, input);
  const admitted = api.readPublicInputs({ input: inputBytes, selected: r.selected, workflow_sha: r.workflow_sha, stage: r.stage,
    repo: r.repo, work_parent: r.work_parent, tools: r.tools });
  fields(admitted, ['stage', 'input'], 'authenticated cross-host intake');
  agree(admitted.input.input, input, 'authenticated I');
  const stageBytes = pin(path.join(admitted.stage.root, 'completion.json'), r.stage.sha256), stages = require('./stage-authoring-npm');
  const stage = stages.decodeStage(stageBytes, inputBytes); agree(admitted.stage.record, stage, 'authenticated S');
  agree(stage.native_inputs.sha256, c.digest(inputBytes), 'S binds original I');
  agree([stage.producer.run_id, stage.producer.run_attempt], [r.stage.artifact.run_id, r.stage.artifact.run_attempt], 'exact stage attempt');
  assert.ok(![stage.producer.run_id, input.producer.run_id, input.preparation.artifact.run_id].includes(r.producer.run_id), 'separate public producer');
  const retained = [];
  for (const [admission, count, kind] of [[admitted.stage, 3, 'stage'], [admitted.input, 19, 'input']]) {
    c.safeDirectory(admission.root); list(admission.subjects, count, 'original subject multiset');
    const seen = new Set();
    for (const row of admission.subjects) {
      const relative = path.relative(admission.root, row.file);
      assert.ok(relative && !relative.startsWith('..') && !path.isAbsolute(relative) && !seen.has(relative), 'distinct contained original subject'); seen.add(relative);
      const bytes = pin(row.file, row.sha256, contract.MAX_NATIVE_BYTES);
      retained.push({ kind, relative, file: row.file, sha256: row.sha256, mode: fs.statSync(row.file).mode & 0o777, bytes });
    }
  }
  assert.ok(retained.some(x => x.kind === 'input' && x.relative === contract.INPUT_FILE && x.bytes.equals(inputBytes)), 'original I subject');
  for (const product of c.PRODUCTS) {
    agree(Object.keys(stage.generated[product]).length, 17, 'both authenticated exact seventeen-entry packs');
    const pack = retained.find(x => x.kind === 'stage' && x.relative === stage.packs[product].file);
    assert.ok(pack, 'both original pack subjects'); agree(pack.sha256, stage.packs[product].sha256, 'pack SHA256'); agree(pack.bytes.length, stage.packs[product].size, 'pack size');
    const crypto = require('node:crypto');
    agree('sha512-' + crypto.createHash('sha512').update(pack.bytes).digest('base64'), stage.packs[product].integrity, 'pack SRI');
    agree(crypto.createHash('sha1').update(pack.bytes).digest('hex'), stage.packs[product].shasum, 'pack SHA1');
    for (const target of c.TARGETS) assert.ok(retained.some(x => x.kind === 'input' && x.relative === input.products[product].assets[target].file &&
      x.sha256 === input.products[product].assets[target].sha256), 'all twelve original native outer subjects');
  }
  return { inputBytes, input, stageBytes, stage, retained, admitted };
}
function actualState(j, roots, scenario) {
  const bridge = require('./packed-installer-bridge'), scope = scopePaths(j, scenario, roots), p = hostPath(j.cell);
  const snap = root => bridge.snapshot(root, true).sha256, prefix = {}, cache = {};
  for (const product of cell(j.cell).products) {
    const packageRoot = p.join(scope.prefix, ...(j.cell.startsWith('windows-') ? [] : ['lib']), 'node_modules', contract.PACKAGES[product]);
    const kinds = j.cell.startsWith('windows-') ? [['posix', ''], ['cmd', '.cmd'], ['powershell', '.ps1']] : [['posix', '']];
    const shims = kinds.map(([kind, suffix]) => p.join(scope.prefix, ...(j.cell.startsWith('windows-') ? [] : ['bin']), product + suffix));
    if (!fs.existsSync(packageRoot)) { assert.ok(shims.every(file => { try { fs.lstatSync(file); return false; } catch (e) { if (e.code === 'ENOENT') return true; throw e; } }), 'removed all shims, including dangling links'); prefix[product] = null; }
    else prefix[product] = { tree: snap(packageRoot), shims: kinds.map(([kind], i) => {
      const file = shims[i], st = fs.lstatSync(file), target = st.isSymbolicLink() ? fs.readlinkSync(file) : null;
      const resolved = fs.realpathSync(file); assert.ok(resolved.startsWith(packageRoot + p.sep) || resolved === file, 'contained npm shim');
      return { kind, path: file, sha256: c.digest(fs.readFileSync(file)), mode: st.mode & 0o777, target };
    }) };
    const release = { descriptor: { schema: contract.DESCRIPTOR_SCHEMA, identity: j.identity, candidate_sha256: j.candidate_sha256 },
      version: j.identity.versions[product], asset: j.subjects[product][cell(j.cell).target] };
    const file = require('../lib/public-authoring').cachePath(p.join(scope.home, '.cache', 'universal-agent-plugins'), product, cell(j.cell).target, release);
    cache[product] = fs.existsSync(file) ? { path: file, sha256: c.digest(c.readFile(file, contract.MAX_NATIVE_BYTES)), size: fs.statSync(file).size, mode: fs.statSync(file).mode & 0o777 } : null;
  }
  return { projects: c.digest(c.encode(Object.fromEntries(cell(j.cell).products.map(p => [p, bridge.snapshot(j.projects[p])])))), prefix, cache, client: snap(roots.client), state: snap(roots.state),
    inputs: c.digest(c.encode([snap(roots.input), snap(roots.stage)])) };
}
function verifySidecars(evidence, root) {
  const rows = [...evidence['npm-lifecycle.json'].rows, ...evidence['cache-process.json'].rows, ...evidence['installer.json'].rows];
  const refs = rows.flatMap(r => [r.observation, ...(r.postinstall ? [r.postinstall.observation] : [])]);
  refs.push(evidence['cache-process.json'].finalization.observation);
  if (evidence['installer.json'].assessment) refs.push(evidence['installer.json'].assessment, ...evidence['installer.json'].readbacks);
  const names = new Map(); let total = 0;
  for (const name of EVIDENCE) for (const [file, bytes] of Object.entries(evidenceFiles(name, evidence[name]))) {
    if (file !== name) refs.push({ path: file, size: bytes.length, sha256: c.digest(bytes) });
    else total += bytes.length;
  }
  for (const ref of refs) {
    sidecar(ref);
    if (names.has(ref.path)) { agree(names.get(ref.path), ref, 'same sidecar pin'); continue; }
    names.set(ref.path, ref); total += ref.size; assert.ok(total <= AGGREGATE_LIMIT, '128MiB complete evidence closure');
    const body = pin(path.join(root, ref.path), ref.sha256, TRANSCRIPT_LIMIT); agree(body.length, ref.size, 'sidecar size');
  }
  assert.ok(total <= AGGREGATE_LIMIT, '128MiB complete evidence closure');
  agree(fs.readdirSync(path.join(root, 'sidecars')).sort(), [...names.keys()].map(n => n.slice('sidecars/'.length)).sort(), 'exhaustive sidecar closure');
}
function failureText(error) {
  if (!error) return null;
  const pending = [error], seen = new Set(); let text = '', count = 0;
  while (pending.length && text.length < 120000 && count++ < 64) {
    const current = pending.shift(); if (seen.has(current)) continue; seen.add(current);
    text += String(current && current.stack || current).slice(0, 8192) + '\n';
    if (current && Array.isArray(current.errors)) pending.push(...current.errors.slice(0, 16));
    if (current && current.cause) pending.push(current.cause);
  }
  return text.slice(0, 120000) + (pending.length ? '\n[additional failure details bounded; journey failed]\n' : '');
}
async function produceJourney(value) {
  // Provision authority is checked before any untrusted record or output effect.
  const provisioning = require('./public-authoring-tools');
  const controller = provisioning.requireController(process.platform === 'win32' ? `windows-${process.arch === 'x64' ? 'amd64' : 'arm64'}` : `${process.platform}-${process.arch === 'x64' ? 'amd64' : 'arm64'}`);
  agree(controller, process.execPath, 'independently provisioned executing controller');
  fields(value, PRODUCE_FIELDS, 'closed producer request'); const r = value;
  fixed(r.schema, 'authoring-public-produce/v1', 'producer schema'); cell(r.cell); tools(r.tools, cell(r.cell)); locator(r.stage);
  fields(r.selected, ['tag', 'ref', 'source', 'versions'], 'selected source');
  for (const k of ['input_file', 'repo', 'work_parent', 'output']) absolute(r[k]);
  agree(r.repo, path.resolve(__dirname, '../../..'), 'executing checkout');
  agree(r.tools.host, { platform: process.platform, arch: process.arch }, 'actual native host');
  const provision = provisioning.requireCellTools(r.cell), manifest = provisioning.readProvisioning();
  agree(r.tools.orchestrator_node, { path: controller, version: process.version, sha256: c.digest(c.readFile(controller)) }, 'controller comparison');
  for (const k of ['npm_node', 'shim_node', 'npm', 'go']) agree(r.tools[k], provision[k] === null ? null : Object.fromEntries(['path', 'sha256', 'version'].map(n => [n, provision[k][n]])), 'source-frozen cell tools');
  const api = requireFacades(r.cell), source = sourceSeal(r.repo, r.workflow_sha, { ...manifest, key: r.cell });
  c.safeDirectory(r.work_parent); c.safeDirectory(path.dirname(r.output)); assert.ok(!fs.existsSync(r.output), 'new owned output');
  disjoint([r.output, r.work_parent, r.repo, path.dirname(r.input_file)]);
  const intake = admitProducerInputs(r, api['public-authoring-custody']), roots = rootsFor(r.output, r.cell);
  disjoint([r.output, intake.admitted.input.root, intake.admitted.stage.root]);
  for (const t of Object.values(r.tools).filter(t => t && t.path)) disjoint([r.output, t.path]);
  disjoint([r.output, provision.npm.closure.root]); if (provision.mod_cache) disjoint([r.output, provision.mod_cache.root]);
  const j = { schema: SCHEMA, status: 'completed', ...Object.fromEntries(['identity', 'authoring_mode', 'asset_scope', 'candidate_sha256', 'pair_marker_sha256', 'native_inputs', 'packs'].map(k => [k, intake.stage[k]])),
    stage: r.stage, producer: r.producer, cell: r.cell, tools: r.tools, command_contract_sha256: c.digest(c.encode(commandContract(r.cell))),
    subjects: Object.fromEntries(c.PRODUCTS.map(p => [p, intake.input.products[p].assets])),
    projects: Object.fromEntries(cell(r.cell).products.map(p => [p, path.join(roots.projects, `${p} projects ü`)])), evidence: [], assertions: Object.fromEntries(ASSERTIONS.map(k => [k, true])) };
  // Installer availability is required before making output, npm or native effects.
  if (api['public-installer-evidence']) api['public-installer-evidence'].requirePublicInstaller({ cell: r.cell, tools: r.tools, roots });
  fs.mkdirSync(r.output, { mode: 0o700 });
  try {
    for (const root of Object.values(roots)) fs.mkdirSync(root, { mode: 0o700 });
    for (const retained of intake.retained) {
      const file = path.join(roots[retained.kind], retained.relative); fs.mkdirSync(path.dirname(file), { recursive: true, mode: 0o700 });
      fs.writeFileSync(file, retained.bytes, { flag: 'wx', mode: retained.mode }); fs.chmodSync(file, retained.mode);
    }
    Object.values(j.projects).forEach(dir => fs.mkdirSync(dir, { mode: 0o700 }));
    const scenarios = scenarioContract(r.cell), all = [...scenarios.npm, ...scenarios.cache, ...scenarios.installer];
    for (const scenario of all) {
      const s = scopePaths(j, scenario, roots);
      for (const dir of [s.prefix, s.home, path.join(s.home, 'tmp'), s.npmCache, s.cwd]) fs.mkdirSync(dir, { recursive: true, mode: 0o700 });
      for (const file of [s.userconfig, s.globalconfig]) if (!fs.existsSync(file)) fs.writeFileSync(file, '', { flag: 'wx', mode: 0o600 });
    }
    const observed = new Map(), commands = new Array(commandContract(r.cell).length); let primary, terminal, finalError;
    const recheck = () => {
      provisioning.requireCellTools(r.cell); agree(sourceSeal(r.repo, r.workflow_sha, { ...manifest, key: r.cell }), source, 'source preserved');
      intake.retained.forEach(x => { for (const file of [x.file, path.join(roots[x.kind], x.relative)]) {
        pin(file, x.sha256, contract.MAX_NATIVE_BYTES); agree(fs.statSync(file).mode & 0o777, x.mode, 'original and copied custody modes');
      } });
    };
    async function run(s, before) {
      if (s.kind === 'repair') {
        const b = before.cache[s.product]; assert.ok(b, 'owned exact repair target'); binary(b, j, s.product);
        assert.ok(b.path.startsWith(scopePaths(j, s, roots).home + path.sep), 'owned corruption target only');
        fs.writeFileSync(b.path, CORRUPTION_BYTES, { flag: 'w' });
        before = actualState(j, roots, s); binary(before.cache[s.product], j, s.product, true);
      }
      if (s.kind === 'core' && commandContract(j.cell)[s.command].id === 'malformed-skill') {
        const destination = path.join(scopePaths(j, s, roots).cwd, 'skill');
        fs.cpSync(path.join(j.projects[s.product], 'skill'), destination, { recursive: true, errorOnExist: true, force: false });
        fs.writeFileSync(path.join(destination, 'skills/extra-skill/SKILL.md'), '---\nname: [invalid\n---\n', { flag: 'w' });
      }
      const pending = Promise.resolve(session.run(plannedInvocation(j, s, roots))).then(value => ({ value }), error => ({ error }));
      let cancellationError;
      if (s.event) { try { await session.cancel({ id: s.id, event: s.event }); } catch (error) { cancellationError = error; } }
      const outcome = await pending;
      if (cancellationError || outcome.error) throw new AggregateError([cancellationError, outcome.error].filter(Boolean), 'observed command/cancellation failed');
      const returned = outcome.value; fields(returned, ['row', 'stdout', 'stderr'], 'observed run result');
      const row = returned.row; for (const k of ['stdout', 'stderr']) { textValue(returned[k]); agree(row[k], { size: Buffer.byteLength(returned[k]), sha256: c.digest(Buffer.from(returned[k])) }, 'observed raw output'); }
      agree(row.before, before, 'observed before state matches original live roots'); agree(row.after, actualState(j, roots, s), 'observed after state matches original live roots');
      if (s.kind === 'literal-argv') {
        const root = path.join(scopePaths(j, s, roots).cwd, 'literal project ü'), bytes = c.readFile(path.join(root, 'plugin.json'), LIMIT);
        const manifest = outputJSON(bytes.toString('utf8'));
        agree(manifest.description, LITERAL_DESCRIPTION, 'literal argv bytes/cwd effects through real shim');
        agree(row.literal, { root, description: manifest.description, manifest_sha256: c.digest(bytes) }, 'observed literal effect matches original bytes');
      }
      if (s.command !== null) {
        const core = commandContract(j.cell)[s.command]; commands[s.command] = { product: core.product, id: core.id, argv: core.argv, cwd: row.cwd,
          status: row.status, signal: row.signal, stdout: returned.stdout, stderr: returned.stderr };
      }
      verifyObservedRow(row, s, j, roots, commands); observed.set(s.id, row);
    }
    const session = await api['public-process-observation'].openPublicObservation({ cell: r.cell, tools: r.tools, roots });
    try {
      for (const method of ['finish', 'run', 'cancel']) assert.equal(typeof session?.[method], 'function', `PUBLIC_FACADE_REQUIRED:public-process-observation.js#session.${method}`);
      for (let i = 0; i < all.length;) {
        const s = all[i], group = [s]; i++;
        if (s.group) while (i < all.length && all[i].group === s.group) group.push(all[i++]);
        recheck();
        const before = group.map(s => actualState(j, roots, s));
        // Admit the entire group before launching any member; no source scan or
        // synchronous cache walk serializes its four actual requests.
        const outcomes = await Promise.allSettled(group.map((s, i) => run(s, before[i])));
        const errors = outcomes.filter(x => x.status === 'rejected').map(x => x.reason);
        try { recheck(); } catch (error) { errors.push(error); }
        if (errors.length) throw new AggregateError(errors, 'C3 fixed scenario failed');
      }
    } catch (e) { primary = e; }
    finally { try { if (typeof session?.finish === 'function') terminal = await session.finish(); } catch (e) { finalError = e; } }
    if (primary || finalError) {
      fs.writeFileSync(path.join(roots.evidence, 'failure.json'), c.encode({ primary: failureText(primary), finalization: failureText(finalError) }), { flag: 'wx', mode: 0o600 });
      throw new AggregateError([primary, finalError].filter(Boolean), 'C3 journey incomplete; no J');
    }
    fields(terminal, ['finalization', 'assessment', 'readbacks'], 'observation terminal evidence');
    const evidence = { 'commands.json': commands,
      'projects.json': Object.fromEntries(cell(r.cell).products.map(p => [p, require('./packed-installer-bridge').snapshot(j.projects[p])])),
      'npm-lifecycle.json': { schema: 'authoring-public-npm-lifecycle/v1', cell: r.cell, rows: scenarios.npm.map(s => observed.get(s.id)) },
      'cache-process.json': { schema: 'authoring-public-cache-process/v1', cell: r.cell, rows: scenarios.cache.map(s => observed.get(s.id)), finalization: terminal.finalization },
      'installer.json': { schema: 'authoring-public-installer/v1', cell: r.cell, rows: scenarios.installer.map(s => observed.get(s.id)), assessment: terminal.assessment, readbacks: terminal.readbacks } };
    verifyJourney({ record: j, evidence }); recheck();
    const late = admitProducerInputs(r, api['public-authoring-custody']); agree(late.stageBytes, intake.stageBytes, 'late authenticated S'); agree(late.inputBytes, intake.inputBytes, 'late authenticated I');
    agree(late.retained.map(({ kind, relative, sha256, mode }) => ({ kind, relative, sha256, mode })), intake.retained.map(({ kind, relative, sha256, mode }) => ({ kind, relative, sha256, mode })), 'late complete custody multiset'); recheck();
    for (const name of EVIDENCE) {
      const files = evidenceFiles(name, evidence[name]);
      for (const [file, bytes] of Object.entries(files)) fs.writeFileSync(path.join(roots.evidence, file), bytes, { flag: 'wx', mode: 0o600 });
      const bytes = files[name]; j.evidence.push({ path: name, size: bytes.length, sha256: c.digest(bytes) });
    }
    verifySidecars(evidence, roots.evidence); recheck();
    const bytes = encodeJourney(j, intake.inputBytes, intake.stageBytes);
    const admission = { schema: 'authoring-public-local-inputs/v1', selected: r.selected, workflow_sha: r.workflow_sha, input_file: path.join(roots.input, contract.INPUT_FILE), stage: r.stage,
      repo: r.repo, work_parent: r.work_parent, stage_root: roots.stage, input_root: roots.input, journey_root: roots.evidence, fixture_root: roots.projects, cell: r.cell, tools: r.tools, producer: r.producer };
    const admissionFile = path.join(roots.admission, 'local-inputs.json'), admissionBytes = c.encode(admission);
    fs.writeFileSync(admissionFile, admissionBytes, { flag: 'wx', mode: 0o600 }); recheck();
    fs.writeFileSync(path.join(roots.evidence, 'public-journey.json'), bytes, { flag: 'wx', mode: 0o600 });
    return { record: j, request: { intake: INTAKE, expectedCommit: r.workflow_sha, journey: path.join(roots.evidence, 'public-journey.json'),
      journeySha256: c.digest(bytes), admission: admissionFile, admissionSha256: c.digest(admissionBytes), fixtureRoot: roots.projects } };
  } catch (error) {
    const diagnostic = path.join(fs.existsSync(roots.evidence) ? roots.evidence : r.output, 'failure.json');
    if (!fs.existsSync(diagnostic)) {
      try { fs.writeFileSync(diagnostic, c.encode({ primary: failureText(error), finalization: null }), { flag: 'wx', mode: 0o600 }); }
      catch (diagnosticError) { throw new AggregateError([error, diagnosticError], 'C3 incomplete; failure receipt could not be written'); }
    }
    throw error;
  }
}

function readJourney(value) {
  const r = request(value), admission = bounded(pin(r.admission, r.admissionSha256), LIMIT);
  requireFacades(admission.cell);
  const provision = require('./public-authoring-tools'), selected = cell(admission.cell);
  agree(provision.requireController(selected.target), process.execPath, 'trusted local controller'); provision.requireCellTools(selected.key);
  const frozen = { ...provision.readProvisioning(), key: selected.key }, before = sourceSeal(admission.repo, r.expectedCommit, frozen);
  const local = readJourneyInputs(value); verifyJourney(local); verifySidecars(local.evidence, path.dirname(value.journey));
  provision.requireCellTools(selected.key); agree(sourceSeal(admission.repo, r.expectedCommit, frozen), before, 'late source closure'); return local;
}
function readAcceptance() { throw new Error("C3b required: completed remote E reader is closed; local J and fixture success are not E"); }
function main(args) {
  if (args.length === 2 && args[0] === "--produce-journey") return produceJourney(fileJSON(args[1]));
  assert.ok(args.length === 2 && args[0] === "--read-local-inputs", "C3a supports only --read-local-inputs REQUEST; public execution and E are closed");
  const requestValue = fileJSON(args[1]), result = readJourneyInputs(requestValue);
  return { scope: "authenticated-input-custody-only", cell: result.record.cell, source: result.identity.commit,
    journey_sha256: requestValue.journeySha256, release_eligible: false, platform_acceptance: false, attested: false };
}
// No producer or completed-E CLI can return a success-shaped placeholder.
module.exports = { generatedFiles, generatedTreeIdentity, evidenceFiles, expandEvidence, scenarioContract, plannedInvocation, rootsFor, requireFacades, verifyNpmLifecycle, verifyCacheProcess, verifyResults, produceJourney,
  expectedCachePath, PROFILES, SURFACE, clientFacts, componentFacts, LITERAL_DESCRIPTION, outputJSON, matrix, commandContract, encodeJourney, decodeJourney, readJourneyInputs, verifyJourney, readJourney,
  readAcceptance, request, fileJSON, disjoint, main, LIMIT, SCHEMA, MATRIX_SCHEMA, INTAKE, WORKFLOW, MISSING };
if (require.main === module) {
  Promise.resolve().then(() => main(process.argv.slice(2))).then(result => process.stdout.write(c.encode(result))).catch(error => { process.stderr.write(`C3 public journey: ${error.message}\n`); process.exitCode = 1; });
}
