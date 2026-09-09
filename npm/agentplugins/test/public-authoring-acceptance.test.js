"use strict";
// SYNTHETIC SOURCE CONTROLS ONLY. No npm, native, provider or verifier executes.
const test = require("node:test"), assert = require("node:assert/strict");
const fs = require("node:fs"), path = require("node:path"), os = require("node:os"), crypto = require("node:crypto");
const c = require("../scripts/dual-authoring-candidate"), a = require("../scripts/public-authoring-acceptance");
const s = require("../scripts/stage-authoring-npm"), ic = require("../lib/public-authoring-contract");
const bridge = require("../scripts/packed-installer-bridge");
const repo = path.resolve(__dirname, "../../.."), H = n => c.digest(Buffer.from(String(n)));
const write = (file, value) => fs.writeFileSync(file, Buffer.isBuffer(value) ? value : c.encode(value), { mode: 0o600 });
const hash = file => c.digest(fs.readFileSync(file));
function fixture(t) {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "C3-SYNTHETIC-")); t.diagnostic(`SYNTHETIC ONLY retained: ${root}`);
  const dir = name => { const file = path.join(root, name); fs.mkdirSync(file, { mode: 0o700 }); return file; };
  const inputRoot = dir("inputs"), stageRoot = dir("stage"), journeyRoot = dir("journey"), fixtureRoot = dir("fixtures"), work = dir("admission-scratch"), toolRoot = dir("tools");
  const id = { repository: c.REPOSITORY, commit: "a".repeat(40), engine_revision: "a".repeat(40), versions: { agentplugins: "0.1.99", "plugin-kit-ai": "2.0.0" } };
  const artifact = n => ({ run_id: n, run_attempt: 2, artifact_id: n + 100, artifact_sha256: H(n) });
  const input = { schema: ic.INPUT_SCHEMA, identity: id, authoring_mode: ic.MODE, asset_scope: ic.SCOPE,
    candidate_sha256: H("candidate"), pair_marker_sha256: H("pair"), products: {},
    preparation: { sha256: H("prep"), artifact: artifact(1) },
    producer: { workflow: ic.WORKFLOW, source: id.commit, run_id: 2, run_attempt: 2 } };
  for (const p of c.PRODUCTS) {
    const assets = {};
    for (const target of c.TARGETS) {
      const binary = { file: c.executableName(p, target), sha256: H(p + target), size: 32 };
      assets[target] = { file: c.assetName(p, id.versions[p], target), sha256: p === "agentplugins" ? binary.sha256 : H("outer" + p + target), size: 32, binary };
    }
    input.products[p] = { tag: (p === "agentplugins" ? "agentplugins-v" : "v") + id.versions[p], manifest_sha256: H("manifest"), checksums_sha256: H("checksums"), assets };
    const projection = ic.projectionBytes(input, p);
    input.products[p].manifest_sha256 = c.digest(projection.manifest); input.products[p].checksums_sha256 = c.digest(projection.checksums);
  }
  const inputBytes = ic.encodeInputs(input); write(path.join(inputRoot, ic.INPUT_FILE), inputBytes);
  const source = Object.fromEntries(s.STAGE_ALLOWLIST.map(name => {
    const bytes = fs.readFileSync(path.join(repo, name)), mode = fs.statSync(path.join(repo, name)).mode & 0o111 ? "100755" : "100644";
    return [name, { bytes, mode, sha256: c.digest(bytes), git_blob: crypto.createHash("sha1").update(`blob ${bytes.length}\0`).update(bytes).digest("hex") }];
  }));
  const pair = s.pairedPackageFiles(source, Object.fromEntries(c.PRODUCTS.map(p => [p, ic.projectionBytes(input, p).manifest])), inputBytes);
  const packs = {};
  for (const p of c.PRODUCTS) {
    const body = Buffer.from("SYNTHETIC NOT A TARBALL " + p), file = `${ic.PACKAGES[p]}-${id.versions[p]}.tgz`;
    write(path.join(stageRoot, file), body);
    packs[p] = { file, sha256: c.digest(body), size: body.length, integrity: "sha512-" + crypto.createHash("sha512").update(body).digest("base64"), shasum: crypto.createHash("sha1").update(body).digest("hex") };
  }
  const stage = { schema: "dual-authoring-public-stage/v1", identity: id, authoring_mode: ic.MODE, asset_scope: ic.SCOPE,
    candidate_sha256: input.candidate_sha256, pair_marker_sha256: input.pair_marker_sha256,
    projection_pins: Object.fromEntries(c.PRODUCTS.map(p => [p, { manifest_sha256: input.products[p].manifest_sha256, checksums_sha256: input.products[p].checksums_sha256 }])),
    native_inputs: { sha256: c.digest(inputBytes), artifact: artifact(2) },
    wrapper_blobs: Object.fromEntries(Object.entries(source).map(([k, { bytes, ...pin }]) => [k, pin])),
    generated: Object.fromEntries(c.PRODUCTS.map(p => [p, Object.fromEntries(Object.entries(pair[p]).map(([k, v]) => [k, c.digest(v)]))])),
    packs, tools: Object.fromEntries(["node", "npm", "git", "tar", "gh"].map(k => [k, { version: "synthetic", sha256: H(k) }])),
    producer: { workflow: ".github/workflows/agentplugins-npm-publish.yml", source: id.commit, ref: `refs/tags/${input.products.agentplugins.tag}`, run_id: 3, run_attempt: 2 },
    assertions: Object.fromEntries(["authenticated_native_inputs", "exact_preparation_binding", "exact_source_blobs", "exact_generated_closures", "exact_pack_entries_modes_bytes", "both_products_complete", "shared_runtime_bytes_equal", "pack_once", "inputs_unchanged", "no_native_execution", "no_publication"].map(k => [k, true])) };
  const stageBytes = s.encodeStage(stage, inputBytes); write(path.join(stageRoot, "completion.json"), stageBytes);
  const tools = {};
  for (const name of ["orchestrator_node", "npm_node", "shim_node", "npm", "go"]) {
    const file = name === "orchestrator_node" ? process.execPath : path.join(toolRoot, name);
    if (file !== process.execPath) write(file, Buffer.from("SYNTHETIC TOOL " + name));
    tools[name] = { path: file, sha256: hash(file), version: name.endsWith("_node") ? process.version : "1.0.0" };
  }
  tools.host = { platform: process.platform, arch: process.arch };
  const projects = {};
  for (const p of c.PRODUCTS) {
    const parent = projects[p] = path.join(fixtureRoot, `${p} projects ü`); fs.mkdirSync(parent);
    for (const lane of bridge.LANES) {
      const dir = path.join(parent, lane); fs.mkdirSync(dir); fs.mkdirSync(path.join(dir, "empty"));
      fs.mkdirSync(path.join(dir, "skills/extra-skill"), { recursive: true });
      write(path.join(dir, "plugin.json"), { name: lane }); write(path.join(dir, "skills/extra-skill/SKILL.md"), Buffer.from("SYNTHETIC"));
    }
  }
  const j = { schema: a.SCHEMA, status: "completed", identity: id, authoring_mode: ic.MODE, asset_scope: ic.SCOPE,
    candidate_sha256: stage.candidate_sha256, pair_marker_sha256: stage.pair_marker_sha256, native_inputs: stage.native_inputs,
    stage: { sha256: c.digest(stageBytes), artifact: artifact(3) }, packs, producer: { ...stage.producer, workflow: a.WORKFLOW, run_id: 4 },
    cell: "linux-amd64/pair-node22", tools, command_contract_sha256: H("pending"),
    subjects: Object.fromEntries(c.PRODUCTS.map(p => [p, input.products[p].assets])), projects, evidence: [],
    assertions: Object.fromEntries(["fixed_commands", "pair_parity", "projects_preserved", "npm_lifecycle", "cache_process", "production_installer", "children_reaped"].map(k => [k, true])) };
  const commands = a.commandContract(j.cell);
  j.command_contract_sha256 = c.digest(c.encode(commands));
  const rows = commands.map(row => {
    const args = row.argv.slice(row.product === "agentplugins" && row.author ? 1 : 0);
    const data = { revision: id.commit, engine: "standard-first-slice/1", authoring_schema_version: 1,
      runtime_evidence: { status: "not_evaluated" }, committed: /\/(init|extra-skill)$/.test(row.id),
      toolchain: { status: row.lane === "skill" ? "pass" : "not_evaluated" }, version: id.versions[row.product], product_version: id.versions[row.product] };
    const result = { schema_version: 1, command: args[0] === "--help" ? "author" : `author.${args[0]}${args[0] === "skills" ? `.${args[1]}` : ""}`, result: row.status ? "failure" : "success", data };
    return { product: row.product, id: row.id, argv: row.argv,
      cwd: row.scenario === "projects" ? projects[row.product] : path.join(fixtureRoot, `${row.product} malformed-skill ü`),
      status: row.status, signal: null, stdout: row.id === "product-help" ? "SYNTHETIC help ".repeat(10) : JSON.stringify(result), stderr: "" };
  });
  const evidence = { "commands.json": rows, "projects.json": Object.fromEntries(c.PRODUCTS.map(p => [p, bridge.snapshot(projects[p])])),
    "npm-lifecycle.json": { synthetic: true }, "cache-process.json": { synthetic: true }, "installer.json": { synthetic: true } };
  const save = () => {
    j.evidence = Object.entries(evidence).map(([name, value]) => { const file = path.join(journeyRoot, name); write(file, value); return { path: name, size: fs.statSync(file).size, sha256: hash(file) }; });
    write(path.join(journeyRoot, "public-journey.json"), a.encodeJourney(j, inputBytes, stageBytes));
  };
  save();
  const admission = { schema: "authoring-public-local-inputs/v1", selected: { tag: input.products.agentplugins.tag, ref: j.producer.ref, source: id.commit, versions: id.versions },
    workflow_sha: id.commit, input_file: path.join(inputRoot, ic.INPUT_FILE), stage: j.stage, repo, work_parent: work,
    stage_root: stageRoot, input_root: inputRoot, journey_root: journeyRoot, fixture_root: fixtureRoot, cell: j.cell, tools, producer: j.producer };
  const admissionPath = path.join(root, "admission.json"); write(admissionPath, admission);
  const request = { intake: a.INTAKE, expectedCommit: id.commit, journey: path.join(journeyRoot, "public-journey.json"),
    journeySha256: hash(path.join(journeyRoot, "public-journey.json")), admission: admissionPath, admissionSha256: hash(admissionPath), fixtureRoot };
  const stageResult = { root: stageRoot, record: s.decodeStage(stageBytes, inputBytes), subjects: ["completion.json", ...c.PRODUCTS.map(p => packs[p].file)].map(n => ({ file: path.join(stageRoot, n), sha256: hash(path.join(stageRoot, n)) })) };
  // Explicit opaque interface fixture, not a substitute provenance implementation.
  const inputSubjects = [{ file: path.join(inputRoot, ic.INPUT_FILE), sha256: c.digest(inputBytes) }];
  for (let i = 0; i < 18; i++) { const file = path.join(inputRoot, `synthetic-subject-${i}`); write(file, Buffer.from(`SYNTHETIC ${i}`)); inputSubjects.push({ file, sha256: hash(file) }); }
  const inputResult = { root: inputRoot, input, subjects: inputSubjects };
  return { root, j, inputBytes, stageBytes, evidence, request, admission, stageResult, inputResult, save,
    repin() { save(); request.journeySha256 = hash(request.journey); write(admissionPath, admission); request.admissionSha256 = hash(admissionPath); } };
}
function withReaders(t, f, run) {
  const name = require.resolve("../scripts/authoring-native-inputs"), original = require.cache[name].exports;
  t.mock.method(s, "readStage", options => { assert.equal(options.stage_sha256, f.j.stage.sha256); return f.stageResult; });
  require.cache[name].exports = { ...original, readInputs(options) { assert.deepEqual(options.artifact, f.j.native_inputs.artifact); return f.inputResult; } };
  try { return run(); } finally { require.cache[name].exports = original; t.mock.restoreAll(); }
}
module.exports = { fixture, withReaders };
if (require.main === module) {
  test("C3 unit closed schemas and immutable pair bindings", t => {
    const f = fixture(t), encoded = a.encodeJourney(f.j, f.inputBytes, f.stageBytes);
    assert.deepEqual(a.decodeJourney(encoded, f.inputBytes, f.stageBytes), f.j);
    const reordered = structuredClone(f.j);
    reordered.producer = Object.fromEntries(Object.entries(reordered.producer).reverse());
    assert.deepEqual(a.encodeJourney(reordered, f.inputBytes, f.stageBytes), encoded);
    assert.throws(() => a.decodeJourney(c.encode(reordered), f.inputBytes, f.stageBytes), /fixed J field order/);
    const cases = [j => { j.extra = true; }, j => { delete j.stage; }, j => { j.identity.commit = "b".repeat(40); },
      j => { j.packs["plugin-kit-ai"].sha256 = H("other"); }, j => { j.stage.artifact.run_attempt++; },
      j => { j.native_inputs.artifact.run_attempt++; }, j => { j.producer.workflow = "other"; },
      j => { j.tools.shim_node.version = "v18.1.0"; }, j => { j.subjects.agentplugins["linux-amd64"].binary.size++; },
      j => { j.evidence.extra = true; }, j => { j.evidence[1].path = "arbitrary.json"; }, j => { j.assertions.extra = true; }];
    cases.forEach((mutate, i) => { const j = structuredClone(f.j); mutate(j); assert.throws(() => a.encodeJourney(j, f.inputBytes, f.stageBytes)); t.diagnostic(`closed mutation ${i}`); });
    for (const body of [Buffer.concat([encoded, Buffer.from(" ")]), Buffer.from('{"schema":1,"schema":2}\n'), Buffer.alloc(a.LIMIT + 1), Buffer.from("[".repeat(17) + "]".repeat(17)), encoded.subarray(0, -5)]) assert.throws(() => a.decodeJourney(body, f.inputBytes, f.stageBytes));
  });
  test("C3 unit stage admission precedes npm and native effects", t => {
    const f = fixture(t);
    withReaders(t, f, () => {
      const local = a.readJourneyInputs(f.request); assert.equal(local.projects.length, 10);
      assert.equal(s.readStage.mock.callCount(), 1);
      assert.throws(() => a.readJourney(f.request), /C3b required/);
      const cache = require.cache[require.resolve("../scripts/authoring-native-inputs")], previous = cache.exports;
      cache.exports = { ...previous, readInputs() { throw new Error("I verifier unavailable"); } };
      try { assert.throws(() => a.readJourneyInputs(f.request), /I verifier unavailable/); }
      finally { cache.exports = previous; }
      t.mock.method(s, "readStage", () => { throw new Error("S verifier unavailable"); });
      assert.throws(() => a.readJourneyInputs(f.request), /S verifier unavailable/);
      assert.equal(fs.readdirSync(f.admission.work_parent).length, 0);
    });
  });
  test("C3 unit fixed host runtime and command matrix", t => {
    assert.equal(a.MATRIX_SCHEMA, "public-wrapper-matrix/v1");
    assert.equal(a.matrix.length, 18); assert.equal(a.matrix.reduce((n, r) => n + r.products.length, 0), 30);
    assert.ok(Object.isFrozen(a.matrix) && a.matrix.every(Object.isFrozen));
    for (const row of a.matrix) {
      const cmds = a.commandContract(row.key); assert.equal(cmds.length, row.node === 18 ? 55 : 127);
      assert.equal(cmds.filter(x => x.id.startsWith("installer/")).length, row.node === 18 ? 0 : 18);
      assert.ok(!cmds.some(x => x.argv.includes("--ignore-scripts")));
    }
    for (const key of [null, "linux-amd64/pair-node18", "linux-amd64/pair-node23"]) assert.throws(() => a.commandContract(key));
    t.diagnostic("18 fixed host cells; 30 product runtimes; no execution");
  });
  test("C3 unit journey results parity and preservation", t => {
    const f = fixture(t); withReaders(t, f, () => {
      const local = a.readJourneyInputs(f.request);
      assert.throws(() => a.verifyJourney(local), /C3b required/);
      for (const [name, mutate] of [ ["drop row", x => x.evidence["commands.json"].pop()],
        ["arbitrary argv", x => x.evidence["commands.json"][0].argv.push("--extra")],
        ["cwd", x => { x.evidence["commands.json"][0].cwd = f.root; }],
        ["signal", x => { x.evidence["commands.json"][0].signal = "SIGTERM"; }],
        ["false result", x => { x.evidence["commands.json"][0].stdout = "{}"; }]]) {
        const bad = structuredClone(local); mutate(bad); assert.throws(() => a.verifyJourney(bad), e => !e.message.includes("C3b required")); t.diagnostic(name);
      }
      const file = path.join(f.j.projects.agentplugins, "skill/plugin.json"); fs.chmodSync(file, 0o400);
      assert.throws(() => a.readJourneyInputs(f.request), /original project trees/);
    });
  });
  test("C3 unit production installer evidence is independently required", t => {
    const f = fixture(t); withReaders(t, f, () => {
      const local = a.readJourneyInputs(f.request);
      assert.throws(() => a.verifyJourney(local), /reviewed public installer result validator and whole-descendant observer/);
      assert.throws(() => bridge.publishSeal(f.request, path.join(f.root, "must-not-exist")), /C3b required/);
      assert.equal(fs.existsSync(path.join(f.root, "must-not-exist")), false);
    });
  });
  test("C3 unit terminal closure rejects missing mismatched or stale evidence", t => {
    const f = fixture(t); withReaders(t, f, () => {
      for (const file of [f.request.journey, f.request.admission, f.inputResult.subjects[1].file,
        f.stageResult.subjects[2].file, f.j.tools.go.path, path.join(f.admission.journey_root, "commands.json")]) {
        const bytes = fs.readFileSync(file); fs.appendFileSync(file, "changed"); assert.throws(() => a.readJourneyInputs(f.request)); write(file, bytes); t.diagnostic(path.basename(file));
      }
      t.mock.method(s, "readStage", () => { const bad = structuredClone(f.stageResult); bad.record.producer.run_attempt++; return bad; });
      assert.throws(() => a.readJourneyInputs(f.request), /authenticated S/);
    });
  });
  test("C3 unit same invocation bridge cannot authenticate remote E", t => {
    const f = fixture(t); assert.throws(() => a.readAcceptance(f.request), /completed remote E reader is closed/);
    assert.throws(() => a.request({ ...f.request, expectedCommit: f.request.expectedCommit + "\n" }));
    for (const extra of [{ authenticated: true }, { completed: true }, { allowPublic: true }]) assert.throws(() => a.request({ ...f.request, ...extra }));
    assert.throws(() => a.main(["--produce-journey", f.request.journey]), /only --read-local-inputs/);
  });
  test("C3 unit legacy fixtures cannot qualify authentic acceptance", t => {
    const f = fixture(t);
    for (const intake of ["public-fixture/v1", "public-fixture/v2", "private"]) assert.throws(() => a.request({ ...f.request, intake }));
    for (const schema of ["dual-authoring-public-native/v1", "dual-authoring-public-native/v2", "authoring-public-packed/v1"]) assert.throws(() => a.encodeJourney({ ...f.j, schema }, f.inputBytes, f.stageBytes));
  });
}
