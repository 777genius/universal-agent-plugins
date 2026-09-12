"use strict";
if (process.env.AGENTPLUGINS_STAGED_TEST_CHILD === "1") {
  require("node:test")("source-checkout-only suite", { skip: "requires the complete repository source tree" }, () => {});
} else {
// SYNTHETIC SOURCE CONTROLS ONLY. No npm, native, provider or verifier executes.
const test = require("node:test"), assert = require("node:assert/strict");
const fs = require("node:fs"), path = require("node:path"), os = require("node:os"), crypto = require("node:crypto");
const c = require("../scripts/dual-authoring-candidate"), a = require("../scripts/public-authoring-acceptance");
const s = require("../scripts/stage-authoring-npm"), ic = require("../lib/public-authoring-contract");
const bridge = require("../scripts/packed-installer-bridge");
const repo = path.resolve(__dirname, "../../.."), H = n => c.digest(Buffer.from(String(n)));
const write = (file, value) => fs.writeFileSync(file, Buffer.isBuffer(value) ? value : c.encode(value), { mode: 0o600 });
const hash = file => c.digest(fs.readFileSync(file));
function fixture(t, key = 'linux-amd64/pair-node22') {
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
      const dir = path.join(parent, lane); fs.mkdirSync(dir, { mode: 0o755 });
      for (const [name, bytes] of Object.entries(a.generatedFiles(lane))) {
        const file = path.join(dir, name); fs.mkdirSync(path.dirname(file), { recursive: true, mode: 0o755 });
        fs.writeFileSync(file, bytes, { mode: 0o644 });
      }
    }
  }
  const selectedCell = a.matrix.find(row => row.key === key), windows = key.startsWith('windows-');
  const namespace = windows ? path.win32.join('Z:\\SYNTHETIC-retained', path.basename(root)) : root;
  const recorded = file => windows ? path.win32.join(namespace, ...path.relative(root, file).split(path.sep)) : file;
  for (const name of ['npm_node', 'shim_node']) tools[name].version = `v${selectedCell.node}.21.1`;
  tools.host = { platform: windows ? 'win32' : selectedCell.target.split('-')[0], arch: selectedCell.target.endsWith('amd64') ? 'x64' : 'arm64' };
  if (key !== 'linux-amd64/pair-node22') tools.go = null;
  if (windows) for (const [name, tool] of Object.entries(tools)) if (tool && name !== 'host')
    tool.path = path.win32.join('Z:\\SYNTHETIC-tools', selectedCell.target, name + '.exe');
  const j = { schema: a.SCHEMA, status: "completed", identity: id, authoring_mode: ic.MODE, asset_scope: ic.SCOPE,
    candidate_sha256: stage.candidate_sha256, pair_marker_sha256: stage.pair_marker_sha256, native_inputs: stage.native_inputs,
    stage: { sha256: c.digest(stageBytes), artifact: artifact(3) }, packs, producer: { ...stage.producer, workflow: a.WORKFLOW, run_id: 4 },
    cell: key, tools, command_contract_sha256: H("pending"),
    subjects: Object.fromEntries(c.PRODUCTS.map(p => [p, input.products[p].assets])),
    projects: Object.fromEntries(selectedCell.products.map(p => [p, recorded(projects[p])])), evidence: [],
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
  const evidence = { "commands.json": rows, "projects.json": Object.fromEntries(selectedCell.products.map(p => {
    const snapshot = bridge.snapshot(projects[p]); snapshot.root = recorded(snapshot.root);
    if (windows) for (const entry of snapshot.entries) entry.mode = entry.kind === 'directory' ? 0o777 : 0o666;
    snapshot.sha256 = c.digest(c.encode(snapshot.entries)); return [p, snapshot];
  })),
    "npm-lifecycle.json": { synthetic: true }, "cache-process.json": { synthetic: true }, "installer.json": { synthetic: true } };
  const save = () => {
    j.evidence = Object.entries(evidence).map(([name, value]) => { const file = path.join(journeyRoot, name); for (const [relative, bytes] of Object.entries(a.evidenceFiles(name, value))) { const target = path.join(journeyRoot, relative); fs.mkdirSync(path.dirname(target), { recursive: true }); write(target, bytes); } return { path: name, size: fs.statSync(file).size, sha256: hash(file) }; });
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
  return { root, namespace, j, inputBytes, stageBytes, evidence, request, admission, stageResult, inputResult, save,
    repin() { save(); request.journeySha256 = hash(request.journey); write(admissionPath, admission); request.admissionSha256 = hash(admissionPath); } };
}
function withReaders(t, f, run) {
  const name = require.resolve("../scripts/authoring-native-inputs"), original = require.cache[name].exports;
  t.mock.method(s, "readStage", options => { assert.equal(options.stage_sha256, f.j.stage.sha256); return f.stageResult; });
  require.cache[name].exports = { ...original, readInputs(options) { assert.deepEqual(options.artifact, f.j.native_inputs.artifact); return f.inputResult; } };
  try { return run(); } finally { require.cache[name].exports = original; t.mock.restoreAll(); }
}
// Every value below is explicitly SYNTHETIC. No pack, native process, scanner,
// custody or observer implementation is executed by these semantic fixtures.
function semanticFixture(f) {
  const j = f.j, commands = a.commandContract(j.cell), roots = a.rootsFor(f.namespace, j.cell);
  const hostPath = j.cell.startsWith('windows-') ? path.win32 : path.posix;
  const snapshots = f.evidence['projects.json'];
  const assessment = status => ({ status, finding_ids: [] });
  const rows = commands.map(w => {
    const productVersion = w.id === 'product-version' && w.product === 'agentplugins';
    const args = w.argv.slice(w.product === 'agentplugins' && w.author ? 1 : 0);
    const operation = w.id === 'retired-v1' ? 'author' : args[0] === '--help' ? 'author' : `author.${args[0]}${args[0] === 'skills' ? '.' + args[1] : ''}`;
    const mutation = ['author.init', 'author.skills.init'].includes(operation), committed = /\/(init|extra-skill)$/.test(w.id);
    const project = w.lane && !w.id.endsWith('/existing') && w.id !== 'installer-flag' && !w.id.startsWith('installer/');
    const malformed = w.id === 'malformed-skill';
    const data = Object.fromEntries(['compatibility', 'toolchain', 'loadability', 'normative_conformance', 'host_safety', 'authoring_readiness', 'release_policy', 'runtime_evidence'].map(k => [k, assessment('not_evaluated')]));
    Object.assign(data, { schema: 'agentplugins-authoring-report/v1', engine: 'standard-first-slice/1', revision: j.identity.commit, command: operation,
      mode: mutation ? 'local_mutation' : 'read', identity: { scope_algorithm: '', read_profile: '', tree_exclusions: null },
      coverage: { components_requested: !!project, skills_enumerated: !!project, inventory_complete: !!project, tree_complete: !!project,
        plugin: project ? 'pass' : 'not_evaluated', mcp: project && w.lane !== 'skill' ? 'pass' : 'not_evaluated', skills: project ? 'pass' : 'not_evaluated', filesystem: project ? 'pass' : 'not_evaluated', facts_complete: !!project },
      profiles: project ? a.PROFILES : [], schema_ids: project ? a.PROFILES.slice(w.lane === 'skill' ? 2 : 2, w.lane === 'skill' ? 3 : 4).map(x => x.id).sort() : [],
      findings: [], components: project ? a.componentFacts(w.lane, !w.id.endsWith('/init'), malformed) : [], checks: [], committed,
      affected_paths: committed ? ['plugin.json'] : [], authoring_schema_version: 1, engine_version: 'standard-first-slice/1',
      requested: { operation, mode: mutation ? 'local_mutation' : 'read' }, effects: { attempted: !!project, committed }, next_actions: [] });
    if (project) {
      const manifest = snapshots[w.product].entries.find(x => x.path === `${w.lane}/plugin.json`);
      data.identity = { scope_algorithm: 'agentplugins-captured-input-sha256-v1', scope_digest: 'sha256:' + H(w.lane), tree_algorithm: 'agentplugins-tree-sha256-v1',
        tree_digest: a.generatedTreeIdentity(snapshots[w.product].entries, w.lane, j.cell), manifest_digest: 'sha256:' + manifest.sha256,
        read_profile: `packageview-local-${a.matrix.find(c => c.key === j.cell).target.split('-')[0]}-v1`, tree_exclusions: ['root .git', 'root non-directory .plugin-kit-ai.lock'] };
      for (const k of ['loadability', 'normative_conformance', 'host_safety', 'authoring_readiness']) data[k] = assessment(malformed && ['normative_conformance', 'authoring_readiness'].includes(k) ? 'fail' : 'pass');
      data.inspection = { name: w.lane, version: '0.1.0', schema: a.PROFILES[2].id, components: data.components.map(x =>
        ({ id: x.id, type: x.type, name: x.id === 'sha256:' + H('skill:extra-skill') ? 'extra-skill' : w.lane })) };
    }
    if (w.id.endsWith('/doctor')) data.toolchain = assessment(w.lane === 'skill' ? 'pass' : 'not_evaluated');
    if (w.id.endsWith('/compat')) {
      data.compatibility = assessment('pass'); data.clients = ['claude', 'codex'].map((client_id, i) => {
        const counts = { skill: 0, mcp_server: 0 };
        return { client_id, capabilities: a.clientFacts().find(c => c.client_id === client_id), components: data.components.map(x => ({ kind: x.type === 'skill' ? 'skill' : 'mcp_server' }))
          .sort((a, b) => a.kind.localeCompare(b.kind)).map(x => ({ ...x, index: ++counts[x.kind], support: 'projected' })),
          limitations: ['static_adapter_support_only', 'installation_not_checked', 'authentication_not_checked', 'runtime_not_checked', 'client_version_not_checked', 'catalog_publication_not_checked', ...(i ? ['manual_activation_required'] : [])] };
      });
    }
    if (w.id.endsWith('/test')) data.checks = [['portable_configuration', 'pass'], ['package_hygiene', 'pass'], ['static_skills', 'pass'], ['static_mcp', w.lane === 'skill' ? 'not_evaluated' : 'pass'], ['runtime', 'not_evaluated']].map(([id, status]) => ({ id, ...assessment(status) }));
    if (['author-help', 'capabilities', 'engine-version', 'product-version'].includes(w.id)) data.commands = a.SURFACE;
    if (w.id === 'capabilities') data.capabilities = { schemas: a.PROFILES.slice(2, 4).map(({ id, digest }) => ({ id, digest })), profiles: a.PROFILES,
      clients: a.clientFacts(), commands: a.SURFACE, evidence_limits: ['static_only', 'no_path_lookup', 'no_executable_version_probe', 'no_runtime_or_oauth_evidence', 'native_files_metadata_only'] };
    if (w.id === 'engine-version' || w.product === 'plugin-kit-ai' && w.id === 'product-version') Object.assign(data, { product: w.product, product_version: j.identity.versions[w.product] });
    const result = { schema_version: 1, command: productVersion ? 'version' : operation, result: w.status ? 'failure' : 'success', data: productVersion ? { version: j.identity.versions.agentplugins } : data };
    return { product: w.product, id: w.id, argv: w.argv, cwd: w.scenario === 'projects' ? j.projects[w.product] : hostPath.join(hostPath.dirname(j.projects[w.product]), `${w.product} malformed-skill ü`),
      status: w.status, signal: null, stdout: w.id === 'product-help' ? `SYNTHETIC ${w.product} help `.repeat(10) : JSON.stringify(result), stderr: '' };
  });
  f.evidence['commands.json'] = rows;
  const inventory = a.scenarioContract(j.cell), prefixes = new Map(), caches = new Map(), selected = a.matrix.find(x => x.key === j.cell);
  const empty = () => Object.fromEntries(selected.products.map(p => [p, null]));
  const asset = (p, name) => ({ path: a.expectedCachePath(j, roots, { cache: name, prefix: 'unused', product: p, kind: 'cold' }, p), ...Object.fromEntries(['sha256', 'size'].map(k => [k, j.subjects[p][selected.target].binary[k]])), mode: j.cell.startsWith('windows-') ? 0o666 : 0o755 });
  const pkg = (p, prefix) => ({ tree: H(p + 'tree'), shims: (j.cell.startsWith('windows-') ? ['posix', 'cmd', 'powershell'] : ['posix']).map(kind => ({ kind,
    path: hostPath.join(roots.npm, prefix, 'prefix', ...(j.cell.startsWith('windows-') ? [] : ['bin']), p + ({ posix: '', cmd: '.cmd', powershell: '.ps1' }[kind])),
    sha256: H(p + 'shim'), mode: j.cell.startsWith('windows-') ? 0o666 : 0o777, target: j.cell.startsWith('windows-') ? null : `../lib/node_modules/${ic.PACKAGES[p]}/bin/${p}.js` })) });
  let projectState = H('initial-projects'); const lastMutation = inventory.cache.filter(s => s.command !== null && /\/(init|extra-skill)$/.test(commands[s.command].id)).at(-1).id;
  let next = 0; const groups = new Map();
  const ref = () => ({ path: 'sidecars/synthetic-observation.json', size: 10, sha256: H('SYNTHETIC\n') });
  function observation(s) {
    if (!prefixes.has(s.prefix)) prefixes.set(s.prefix, empty()); if (!caches.has(s.cache)) caches.set(s.cache, empty());
    const before = { projects: projectState, prefix: structuredClone(prefixes.get(s.prefix)), cache: structuredClone(caches.get(s.cache)), client: H('client'), state: H('state'), inputs: H('inputs') };
    const after = structuredClone(before), p = s.product;
    if (s.command !== null && /\/(init|extra-skill)$/.test(commands[s.command].id)) projectState = s.id === lastMutation ? c.digest(c.encode(snapshots)) : H(s.id);
    after.projects = projectState;
    const npm = ['install', 'reinstall', 'uninstall'].includes(s.kind);
    let acquisitions = 0, commits = 0, launches = npm ? 0 : 1;
    if (['install', 'reinstall'].includes(s.kind)) after.prefix[p] = pkg(p, s.prefix);
    if (s.kind === 'uninstall') after.prefix[p] = null;
    if (s.kind.startsWith('invalid-') || s.kind === 'waiter-cancel') launches = 0;
    else if (s.kind !== 'uninstall' && (p === 'plugin-kit-ai' || !npm)) {
      if (!before.cache[p] || s.kind === 'repair') acquisitions = commits = 1;
      after.cache[p] = asset(p, s.cache);
    }
    if (s.kind === 'concurrent-cold' && !s.id.endsWith('-0')) acquisitions = commits = 0;
    if (s.kind === 'repair') before.cache[p] = { ...before.cache[p], sha256: H('C3 intentional owned cache corruption\n'), size: Buffer.byteLength('C3 intentional owned cache corruption\n') };
    if (s.kind === 'concurrent-cold' || s.kind === 'waiter-cancel') before.cache[p] = null;
    const planned = a.plannedInvocation(j, s, roots), core = s.command === null ? null : rows[s.command];
    const stdout = core ? core.stdout : npm || s.kind === 'literal-argv' || s.event || s.kind.startsWith('invalid-') ? 'SYNTHETIC process output' : rows.find(r => r.product === p && r.id === 'product-version').stdout;
    const row = { id: s.id, command: s.command, argv: planned.argv, cwd: planned.cwd, env: planned.env,
      executable: { path: planned.argv[0], sha256: npm ? j.tools.npm_node.sha256 : H(p + 'shim') }, runtime: npm ? j.tools.npm_node : j.tools.shim_node,
      stdout: { size: Buffer.byteLength(stdout), sha256: H(stdout) }, stderr: { size: 0, sha256: H('') }, status: core ? core.status : s.kind.startsWith('invalid-') ? 1 : 0, signal: null,
      before, after, observation: ref(), events: !npm ? ['shim', ...(launches ? ['native'] : [])] : [],
      acquisitions, commits, downloads: 0, native_launches: launches, postinstall: null, interval: [next, next + 10],
      literal: s.kind === 'literal-argv' ? { root: hostPath.join(planned.cwd, 'literal project ü'), description: a.LITERAL_DESCRIPTION, manifest_sha256: H('synthetic literal manifest') } : null };
    if (s.group) { if (!groups.has(s.group)) groups.set(s.group, next); row.interval = [groups.get(s.group), groups.get(s.group) + 10]; } next += 20;
    if (s.kind === 'waiter-cancel') row.events.push('cache-waiter');
    if (s.kind === 'waiter-owner') row.events.splice(1, 0, 'lock-owner');
    if (s.kind === 'repair') row.events.splice(1, 0, 'repair-before-launch');
    if (s.kind.startsWith('invalid-')) row.events.push('locator-rejected');
    if (s.event) { row.events.push('cancel-delivered'); row.status = ({ SIGINT: 130, SIGTERM: 143, CTRL_C_EVENT: 130, TerminateProcess: 1 })[s.event]; }
    row.events.push('reaped');
    if (['install', 'reinstall'].includes(s.kind) && p === 'plugin-kit-ai') row.postinstall = { argv: [j.tools.npm_node.path, './lib/install.js'], runtime: j.tools.npm_node, acquisitions, commits, observation: ref() };
    prefixes.set(s.prefix, after.prefix); caches.set(s.cache, after.cache); return row;
  }
  const npm = inventory.npm.map(observation), cache = inventory.cache.map(observation), installer = inventory.installer.map(observation);
  // Both peer starts were cold in an overlapping group, with disjoint namespaces.
  const peers = cache.filter(r => r.id.endsWith('/peer-overlap'));
  for (const row of peers) for (const p of selected.products.filter(x => x !== row.id.split('/')[0])) { row.before.cache[p] = null; row.after.cache[p] = asset(p, 'peer-overlap'); }
  f.evidence['npm-lifecycle.json'] = { schema: 'authoring-public-npm-lifecycle/v1', cell: j.cell, rows: npm };
  f.evidence['cache-process.json'] = { schema: 'authoring-public-cache-process/v1', cell: j.cell, rows: cache, finalization: {
    rows: [...inventory.npm, ...inventory.cache, ...inventory.installer].map(s => s.id), descendants: [], locks: [], late_errors: [], observation: ref() } };
  f.evidence['installer.json'] = { schema: 'authoring-public-installer/v1', cell: j.cell, rows: installer, assessment: selected.node === 18 ? null : ref(), readbacks: inventory.installer.map(ref) };
  const sidecars = path.join(f.admission.journey_root, 'sidecars'); fs.mkdirSync(sidecars, { recursive: true }); fs.writeFileSync(path.join(sidecars, 'synthetic-observation.json'), 'SYNTHETIC\n');
  f.repin(); return f;
}
function withFacades(t, run) {
  const exists = fs.existsSync, Module = require('node:module'), load = Module._load;
  const api = {
    'public-authoring-custody': { readPublicInputs() { throw new Error('SYNTHETIC custody not provided'); } },
    'public-process-observation': { openPublicObservation() { throw new Error('SYNTHETIC session not provided'); }, verifyPublicObservation({ rows, finalization }) { return { rows, finalization }; } },
    'public-installer-evidence': { requirePublicInstaller() { return undefined; }, verifyPublicInstaller({ assessment, readbacks }) { return { assessment, readbacks }; } }
  };
  fs.existsSync = file => Object.keys(api).some(n => file === path.join(repo, 'npm/agentplugins/scripts', n + '.js')) || exists(file);
  Module._load = function(name, ...args) {
    const key = Object.keys(api).find(n => name === path.join(repo, 'npm/agentplugins/scripts', n + '.js'));
    return key ? api[key] : load.call(this, name, ...args);
  };
  const restore = () => { fs.existsSync = exists; Module._load = load; };
  try { const result = run(api); if (result && typeof result.then === 'function') return result.finally(restore); restore(); return result; } catch (e) { restore(); throw e; }
}

// All eighteen semantic records are synthetic; only existing public owner APIs
// are substituted. The actual aggregate reader/encoder/filesystem code runs.
function aggregateFixture(t, run, phase = 'assemble') {
  const artifacts = new Map(), modes = new Map(), journeys = [], manifest = { controllers: {}, cells: {} };
  let first, designated;
  for (const { key, target } of a.matrix) {
    const f = semanticFixture(fixture(t, key)), j = f.j;
    if (!first) first = { j, root: f.root, admission: f.admission, inputBytes: f.inputBytes, stageBytes: f.stageBytes,
      inputResult: f.inputResult, stageResult: f.stageResult };
    assert.deepEqual(f.inputBytes, first.inputBytes); assert.deepEqual(f.stageBytes, first.stageBytes);
    const artifact = { run_id: 4, run_attempt: 2, artifact_id: 1000 + journeys.length, artifact_sha256: H('SYNTHETIC archive ' + key) };
    journeys.push({ cell: key, sha256: f.request.journeySha256, artifact });
    artifacts.set(artifact.artifact_id, { root: f.admission.journey_root, artifact });
    modes.set(artifact.artifact_id, 'produce');
    manifest.controllers[target] = { node: j.tools.orchestrator_node, git: { path: '/SYNTHETIC/git' }, python: { path: '/SYNTHETIC/python' } };
    manifest.cells[key] = { ...j.tools, runner: {}, image: {}, observer: {}, installer_policy: {} };
    if (key === 'linux-amd64/pair-node22') designated = { j, root: f.admission.journey_root, located: journeys.at(-1) };
  }
  const attempt = (mode, run_id) => {
    const producer = { ...first.j.producer, run_id };
    return { producer, status: 'completed', conclusion: 'success', jobs: a.PUBLIC_JOBS[mode].map((name, i) => ({
      id: run_id * 100 + i, name, run_id, run_attempt: producer.run_attempt, source: producer.source,
      ref: producer.ref, status: 'completed', conclusion: 'success' })) };
  };
  for (const value of artifacts.values()) value.attempt = attempt('produce', 4);
  for (const name of a.BRIDGE_FILES) {
    const file = path.join(designated.root, 'bridge', name); fs.mkdirSync(path.dirname(file), { recursive: true });
    write(file, Buffer.from('SYNTHETIC bridge transport ' + name));
  }
  const bridgeLocator = { sha256: hash(path.join(designated.root, 'bridge/summary.json')), artifact: designated.located.artifact };
  const inputSubjects = first.inputResult.subjects.slice(0, 7);
  for (const product of c.PRODUCTS) for (const target of c.TARGETS) {
    const file = path.join(first.inputResult.root, first.j.subjects[product][target].file);
    write(file, Buffer.from((product === 'agentplugins' ? '' : 'outer') + product + target));
    inputSubjects.push({ file, sha256: hash(file) });
  }
  first.inputResult.subjects = inputSubjects;
  const request = { schema: 'authoring-public-assemble/v1', selected: first.admission.selected, workflow_sha: first.j.identity.commit,
    input_file: first.admission.input_file, stage: first.j.stage, repo, work_parent: first.admission.work_parent,
    output: path.join(first.root, 'SYNTHETIC-R2'), producer: attempt('assemble', 5).producer, journeys, bridge: bridgeLocator };
  const provision = require('../scripts/public-authoring-tools'), promotion = require('../scripts/authoring-promotion'), cp = require('node:child_process');
  t.mock.method(provision, 'requireController', () => process.execPath);
  t.mock.method(provision, 'readProvisioning', () => manifest);
  t.mock.method(cp, 'execFileSync', (file, args) => {
    assert.equal(file, '/SYNTHETIC/git');
    if (args[0] === 'rev-parse') return Buffer.from(request.workflow_sha);
    if (args[0] === 'status') return Buffer.alloc(0);
    assert.deepEqual(args, ['ls-files', '-z']); return Buffer.from('npm/agentplugins/scripts/public-authoring-acceptance.js\0');
  });
  t.mock.method(cp, 'spawnSync', (file, args) => {
    assert.equal(file, '/SYNTHETIC/python'); assert.deepEqual(args.slice(0, 3), ['-B', path.join(repo, 'scripts/check-packed-ci.py'), '--completed-bridge']);
    assert.equal(args[3], path.join(designated.root, 'bridge')); assert.equal(args[4], request.workflow_sha);
    return { status: 0, signal: null, stderr: Buffer.alloc(0), stdout: c.encode({ identity: designated.j.identity,
      public_inputs: { journey_sha256: designated.located.sha256, producer: designated.j.producer } }) };
  });
  t.mock.method(promotion, 'inspectPublicCaller', (selected, sha, mode) => {
    assert.deepEqual(selected, request.selected); assert.equal(sha, request.workflow_sha); assert.equal(mode, phase);
    return attempt(mode, mode === 'attest' ? 6 : 5).producer;
  });
  t.mock.method(promotion, 'inspectPublicAttempt', (artifact, selected, mode) => {
    assert.equal(mode, modes.get(artifact.artifact_id)); assert.deepEqual(selected, request.selected);
    assert.deepEqual(artifact, artifacts.get(artifact.artifact_id).artifact); return artifacts.get(artifact.artifact_id).attempt;
  });
  const seedBundle = () => {
    const root = path.join(first.root, 'SYNTHETIC-original-R2'), files = [];
    for (const loc of journeys) {
      const retained = artifacts.get(loc.artifact.artifact_id).root;
      for (const name of ['public-journey.json', 'commands.json', 'projects.json', 'npm-lifecycle.json', 'cache-process.json', 'installer.json',
        ...fs.readdirSync(path.join(retained, 'sidecars')).map(name => 'sidecars/' + name)])
        files.push({ name: `journeys/${loc.cell}/${name}`, original: path.join(retained, name) });
    }
    files.push(...a.BRIDGE_FILES.map(name => ({ name: 'bridge/' + name, original: path.join(designated.root, 'bridge', name) })));
    const e = { schema: a.ACCEPTANCE_SCHEMA, lane: 'public-packed-pair',
      ...Object.fromEntries(['identity', 'authoring_mode', 'asset_scope', 'candidate_sha256', 'pair_marker_sha256', 'native_inputs', 'stage', 'packs'].map(k => [k, first.j[k]])),
      producer: request.producer, matrix: { schema: a.MATRIX_SCHEMA, cells: a.matrix.map(row => row.key) }, journeys,
      bridge: bridgeLocator, evidence: { path: a.INDEX_FILE, size: 1, sha256: H('pending index') }, assertions: first.j.assertions };
    const index = { schema: a.INDEX_SCHEMA, matrix: e.matrix, journeys, bridge: bridgeLocator,
      files: files.map(({ name, original }) => ({ path: name, size: fs.statSync(original).size, sha256: hash(original) }))
        .sort((a, b) => a.path < b.path ? -1 : 1) };
    const indexBytes = a.encodeAcceptanceIndex(index, e);
    e.evidence = { path: a.INDEX_FILE, size: indexBytes.length, sha256: c.digest(indexBytes) };
    const bytes = a.encodeAcceptance(e, first.inputBytes, first.stageBytes);
    fs.mkdirSync(root);
    for (const { name, original } of files) {
      const file = path.join(root, name); fs.mkdirSync(path.dirname(file), { recursive: true }); fs.copyFileSync(original, file);
    }
    write(path.join(root, a.INDEX_FILE), indexBytes); write(path.join(root, a.ACCEPTANCE_FILE), bytes);
    const locators = {};
    for (const [mode, run_id, artifact_id] of [['assemble', 5, 2000], ['attest', 6, 2001]]) {
      const artifact = { run_id, run_attempt: 2, artifact_id, artifact_sha256: H('SYNTHETIC bundle transport ' + mode) };
      const retained = mode === 'assemble' ? root : path.join(first.root, 'SYNTHETIC-original-R3');
      if (mode === 'attest') {
        fs.mkdirSync(retained); write(path.join(retained, a.ATTESTATION_LINK), locators.assemble);
        fs.cpSync(root, path.join(retained, 'evidence'), { recursive: true });
      }
      artifacts.set(artifact_id, { root: retained, artifact, attempt: attempt(mode, run_id) }); modes.set(artifact_id, mode);
      locators[mode] = { sha256: c.digest(bytes), artifact };
    }
    return { e, index, root, bytes, assembly: locators.assemble, acceptance: locators.attest };
  };
  return withFacades(t, api => {
    api['public-authoring-custody'].readPublicInputs = () => ({ input: first.inputResult, stage: first.stageResult });
    api['public-authoring-custody'].readPublicArtifact = ({ kind, locator }) => {
      assert.equal(kind, { produce: 'public-journeys', assemble: 'public-assembly', attest: 'public-evidence' }[modes.get(locator.artifact.artifact_id)]);
      return artifacts.get(locator.artifact.artifact_id);
    };
    return run({ request, first, artifacts, manifest, api, seedBundle });
  });
}
function substitutedAssemblyLink(t, late) {
  return aggregateFixture(t, ({ request, seedBundle, artifacts }) => {
    const original = seedBundle(), promotion = require('../scripts/authoring-promotion'); let signatures = 0;
    const replace = () => {
      const value = structuredClone(original.assembly); value.artifact.run_attempt++;
      write(path.join(artifacts.get(original.acceptance.artifact.artifact_id).root, a.ATTESTATION_LINK), value);
    };
    if (!late) replace();
    t.mock.method(promotion, 'verifyPublicSubject', () => { if (++signatures === 1 && late) replace(); });
    const r = { schema: 'authoring-public-read/v1', ...Object.fromEntries(['selected', 'workflow_sha', 'input_file', 'stage', 'repo', 'work_parent'].map(k => [k, request[k]])),
      assembly: original.assembly, acceptance: original.acceptance };
    assert.throws(() => a.readAcceptance(r), /R3 original R2 locator/);
    assert.equal(signatures, late ? 2 : 0);
  }, 'read');
}

module.exports = { fixture, withReaders };
if (require.main === module) {
  test('C3 unit R3 original locator substitution rejects before signatures', t => substitutedAssemblyLink(t, false));
  test('C3 unit R3 late original locator change cannot return acceptance', t => substitutedAssemblyLink(t, true));
  test('C3 unit attestation inputs re-admit original aggregate without signing', t => aggregateFixture(t, ({ request, first, seedBundle }) => {
    const original = seedBundle();
    const r = { schema: 'authoring-public-attest/v1', ...Object.fromEntries(['selected', 'workflow_sha', 'input_file', 'stage', 'repo', 'work_parent'].map(k => [k, request[k]])),
      assembly: original.assembly, output: path.join(first.root, 'SYNTHETIC-R3-intake') };
    const result = a.attestAcceptanceInputs(r);
    assert.deepEqual(result.assembly, original.assembly); assert.equal(result.sha256, original.assembly.sha256);
    assert.deepEqual(fs.readFileSync(path.join(result.root, 'evidence', a.ACCEPTANCE_FILE)), original.bytes);
    assert.deepEqual(a.readAcceptanceClosure(path.join(result.root, 'evidence'), original.e), original.index);
    assert.deepEqual(JSON.parse(fs.readFileSync(path.join(result.root, a.ATTESTATION_LINK))), original.assembly);
    assert.deepEqual(fs.readdirSync(result.root).sort(), [a.ATTESTATION_LINK, 'evidence']);
    assert.deepEqual(result.subjects.map(row => path.basename(row.file)), [a.ACCEPTANCE_FILE, a.INDEX_FILE]);
  }, 'attest'));
  test('C3 unit completed aggregate reader binds original R2 and both R3 subjects', t => aggregateFixture(t, ({ request, seedBundle }) => {
    const original = seedBundle(), promotion = require('../scripts/authoring-promotion'); let signatures = 0;
    t.mock.method(promotion, 'verifyPublicSubject', (file, expected) => {
      signatures++; assert.equal(hash(file), expected.sha256);
      assert.equal(expected.run_id, 6); assert.equal(expected.run_attempt, 2);
      assert.equal(expected.source, request.workflow_sha); assert.equal(expected.workflow_sha, request.workflow_sha);
      assert.equal(expected.ref, request.selected.ref);
      assert.deepEqual(expected.subjects, [{ name: a.ACCEPTANCE_FILE, digest: { sha256: original.assembly.sha256 } },
        { name: a.INDEX_FILE, digest: { sha256: original.e.evidence.sha256 } }]);
      assert.equal(expected.name, path.basename(file));
    });
    const r = { schema: 'authoring-public-read/v1', ...Object.fromEntries(['selected', 'workflow_sha', 'input_file', 'stage', 'repo', 'work_parent'].map(k => [k, request[k]])),
      assembly: original.assembly, acceptance: original.acceptance };
    const result = a.readAcceptance(r);
    assert.equal(signatures, 2); assert.deepEqual(result.record, original.e);
    assert.deepEqual(result.assembly, original.assembly); assert.deepEqual(result.acceptance, original.acceptance);
  }, 'read'));
  test('C3 unit assembly replays eighteen cells through fixed owner interfaces', t => aggregateFixture(t, ({ request, first, api }) => {
    const assembled = a.assembleAcceptance(request);
    assert.equal(assembled.root, request.output);
    const bytes = fs.readFileSync(path.join(assembled.root, a.ACCEPTANCE_FILE));
    assert.equal(c.digest(bytes), assembled.sha256);
    const e = a.decodeAcceptance(bytes, first.inputBytes, first.stageBytes);
    assert.deepEqual(e.journeys, request.journeys); assert.deepEqual(e.bridge, request.bridge);
    assert.deepEqual(e.producer, request.producer); assert.equal(a.readAcceptanceClosure(assembled.root, e).journeys.length, 18);
    assert.deepEqual(assembled.subjects.map(row => path.basename(row.file)), [a.ACCEPTANCE_FILE, a.INDEX_FILE]);
    const custody = api['public-authoring-custody'], originalArtifact = custody.readPublicArtifact;
    custody.readPublicArtifact = value => {
      const retained = originalArtifact(value);
      return { ...retained, artifact: { ...retained.artifact, run_attempt: 99 } };
    };
    const denied = { ...request, output: path.join(first.root, 'SYNTHETIC-rejected-R2') };
    assert.throws(() => a.assembleAcceptance(denied), /exact public artifact custody/);
    assert.equal(fs.existsSync(denied.output), false);
    const badStage = structuredClone(first.stageResult.record); badStage.packs['plugin-kit-ai'].size++;
    custody.readPublicInputs = () => ({ input: first.inputResult, stage: { ...first.stageResult, record: badStage } });
    let artifactEffects = 0; custody.readPublicArtifact = () => { artifactEffects++; throw Error('unexpected artifact acquisition'); };
    assert.throws(() => a.assembleAcceptance(denied), /authenticated S/);
    assert.equal(artifactEffects, 0); assert.equal(fs.existsSync(denied.output), false);
  }));
  for (const { key } of a.matrix) test(`C3 unit completed cell ${key}`, t => {
    const f = semanticFixture(fixture(t, key)), j = f.j;
    const manifest = { cells: { [key]: { ...j.tools, runner: {}, image: {}, observer: {}, installer_policy: {} } },
      controllers: { [key.split('/')[0]]: { node: j.tools.orchestrator_node } } };
    const attempt = { producer: j.producer, status: 'completed', conclusion: 'success',
      jobs: a.PUBLIC_JOBS.produce.map((name, i) => ({ id: i + 1, name, run_id: j.producer.run_id,
        run_attempt: j.producer.run_attempt, source: j.producer.source, ref: j.producer.ref, status: 'completed', conclusion: 'success' })) };
    const located = { sha256: f.request.journeySha256, artifact: { run_id: j.producer.run_id,
      run_attempt: j.producer.run_attempt, artifact_id: 500, artifact_sha256: H('SYNTHETIC transport') } };
    let remoteOpens = 0;
    for (const name of ['lstatSync', 'statSync', 'realpathSync', 'readFileSync', 'openSync', 'readdirSync']) {
      const original = fs[name];
      t.mock.method(fs, name, function(file, ...args) {
        if (typeof file === 'string' && (file.startsWith('Z:\\') || Object.values(j.projects).some(root => file === root || file.startsWith(root + path.sep)))) {
          remoteOpens++; throw Error('original producer project/tool namespace opened');
        }
        return original.call(this, file, ...args);
      });
    }
    withFacades(t, () => assert.deepEqual(a.retainedJourney(f.admission.journey_root, located, f.inputBytes,
      f.stageBytes, manifest, attempt, f.admission.selected).record, j));
    assert.equal(remoteOpens, 0);
  });
  test('C3 unit completed J replays retained bytes without opening original roots', t => {
    const f = semanticFixture(fixture(t)), j = f.j;
    const retained = path.join(f.root, 'retained-copy');
    fs.cpSync(f.admission.journey_root, retained, { recursive: true });
    const row = { ...j.tools, runner: {}, image: {}, observer: {}, installer_policy: {} };
    const manifest = { cells: { [j.cell]: row }, controllers: { 'linux-amd64': { node: j.tools.orchestrator_node } } };
    const located = { sha256: f.request.journeySha256, artifact: { run_id: j.producer.run_id,
      run_attempt: j.producer.run_attempt, artifact_id: 500, artifact_sha256: H('SYNTHETIC retained transport') } };
    const attempt = { producer: j.producer, status: 'completed', conclusion: 'success',
      jobs: a.PUBLIC_JOBS.produce.map((name, i) => ({ id: i + 1, name, run_id: j.producer.run_id,
        run_attempt: j.producer.run_attempt, source: j.producer.source, ref: j.producer.ref, status: 'completed', conclusion: 'success' })) };
    const check = () => a.retainedJourney(retained, located, f.inputBytes, f.stageBytes, manifest, attempt, f.admission.selected);
    let forbidden = 0;
    for (const name of ['lstatSync', 'statSync', 'realpathSync', 'readFileSync', 'openSync', 'readdirSync']) {
      const original = fs[name];
      t.mock.method(fs, name, function(file, ...args) {
        if (typeof file === 'string' && file.startsWith(f.root + path.sep) && file !== retained && !file.startsWith(retained + path.sep)) {
          forbidden++; throw Error('original remote namespace opened');
        }
        return original.call(this, file, ...args);
      });
    }
    withFacades(t, api => {
      assert.deepEqual(check().record, j);
      const original = attempt.jobs[0].run_attempt;
      attempt.jobs[0].run_attempt++;
      assert.throws(check, /public job run_attempt/); attempt.jobs[0].run_attempt = original;
      const verify = api['public-process-observation'].verifyPublicObservation;
      api['public-process-observation'].verifyPublicObservation = () => true;
      assert.throws(check, /never boolean success/);
      api['public-process-observation'].verifyPublicObservation = verify;
      fs.appendFileSync(path.join(retained, 'sidecars/synthetic-observation.json'), 'changed');
      assert.throws(check, /pin|size/);
    });
    assert.equal(forbidden, 0);
  });
  test('C3 unit E syntax binds all cells and distinct assembly attempt', t => {
    const f = fixture(t), j = f.j;
    const journeys = a.matrix.map((row, i) => ({ cell: row.key, sha256: H(`J-${i}`),
      artifact: { run_id: 4, run_attempt: 2, artifact_id: 200 + i, artifact_sha256: H(`archive-${i}`) } }));
    const e = { schema: a.ACCEPTANCE_SCHEMA, lane: 'public-packed-pair',
      ...Object.fromEntries(['identity', 'authoring_mode', 'asset_scope', 'candidate_sha256', 'pair_marker_sha256', 'native_inputs', 'stage', 'packs'].map(k => [k, j[k]])),
      producer: { ...j.producer, run_id: 5 }, matrix: { schema: a.MATRIX_SCHEMA, cells: a.matrix.map(row => row.key) }, journeys,
      bridge: { sha256: H('bridge'), artifact: journeys[1].artifact },
      evidence: { path: a.INDEX_FILE, size: 100, sha256: H('index') }, assertions: j.assertions };
    const encoded = a.encodeAcceptance(e, f.inputBytes, f.stageBytes);
    assert.deepEqual(a.decodeAcceptance(encoded, f.inputBytes, f.stageBytes), e);
    const reversed = Object.fromEntries(Object.entries(e).reverse());
    assert.deepEqual(a.encodeAcceptance(reversed, f.inputBytes, f.stageBytes), encoded);
    assert.throws(() => a.decodeAcceptance(c.encode(reversed), f.inputBytes, f.stageBytes), /fixed E field order/);
    const cases = [
      ['missing cell', x => x.journeys.pop()],
      ['duplicate cell', x => { x.journeys[2] = x.journeys[1]; }],
      ['wrong R1 attempt', x => { x.journeys[17].artifact.run_attempt++; }],
      ['wrong R1 run', x => { x.journeys[17].artifact.run_id++; }],
      ['R2 self-completion cycle', x => { x.producer.run_id = 4; }],
      ['R2 input cycle', x => { x.producer.run_id = 2; }],
      ['wrong workflow', x => { x.producer.workflow = '.github/workflows/agentplugins-release.yml'; }],
      ['wrong F', x => { x.producer.source = 'b'.repeat(40); }],
      ['wrong ref', x => { x.producer.ref = 'refs/heads/master'; }],
      ['wrong stage attempt', x => { x.stage.artifact.run_attempt++; }],
      ['wrong bridge cell', x => { x.bridge.artifact = x.journeys[2].artifact; }],
      ['bridge substituted J', x => { x.bridge.sha256 = x.journeys[1].sha256; }],
      ['duplicate J digest', x => { x.journeys[2].sha256 = x.journeys[1].sha256; }],
      ['duplicate archive digest', x => { x.journeys[2].artifact.artifact_sha256 = x.journeys[1].artifact.artifact_sha256; }],
      ['duplicate artifact ID', x => { x.journeys[2].artifact.artifact_id = x.journeys[1].artifact.artifact_id; }],
      ['wrong second pack SRI', x => { x.packs['plugin-kit-ai'].integrity = 'sha512-' + 'A'.repeat(88); }],
      ['wrong I digest', x => { x.native_inputs.sha256 = H('other I'); }],
      ['wrong matrix', x => x.matrix.cells.reverse()],
      ['unknown assertion', x => { x.assertions.fixture_accepted = true; }],
      ['false assertion', x => { x.assertions.children_reaped = false; }],
      ['self locator', x => { x.artifact = x.bridge.artifact; }],
      ['wrong index filename', x => { x.evidence.path = a.ACCEPTANCE_FILE; }],
      ['oversized index', x => { x.evidence.size = a.LIMIT + 1; }]
    ];
    for (const [name, mutate] of cases) {
      const bad = structuredClone(e); mutate(bad);
      assert.throws(() => a.encodeAcceptance(bad, f.inputBytes, f.stageBytes), name); t.diagnostic(name);
    }
    for (const body of [Buffer.concat([encoded, Buffer.from(' ')]), Buffer.from('{"schema":1,"schema":2}\n'), Buffer.alloc(a.LIMIT + 1),
      Buffer.from('['.repeat(17) + ']'.repeat(17)), encoded.subarray(0, -5)]) assert.throws(() => a.decodeAcceptance(body, f.inputBytes, f.stageBytes));
    // A canonical E fixture remains unable to cross completed custody admission.
    assert.throws(() => a.readAcceptance(e), /PUBLIC_PROVISIONING_REQUIRED/);
    const selected = f.admission.selected;
    const attempt = (mode, run_id) => {
      const producer = { ...j.producer, run_id };
      return { producer, status: 'completed', conclusion: 'success', jobs: a.PUBLIC_JOBS[mode].map((name, i) => ({
        id: run_id * 100 + i, name, run_id, run_attempt: producer.run_attempt, source: producer.source,
        ref: producer.ref, status: 'completed', conclusion: 'success' })) };
    };
    const attempts = { produce: attempt('produce', 4), assemble: attempt('assemble', 5), attest: attempt('attest', 6) };
    const assembly = { sha256: c.digest(encoded), artifact: { run_id: 5, run_attempt: 2, artifact_id: 300, artifact_sha256: H('R2') } };
    const signed = { sha256: c.digest(encoded), artifact: { run_id: 6, run_attempt: 2, artifact_id: 301, artifact_sha256: H('R3') } };
    assert.deepEqual(a.acceptanceGraph(e, attempts, selected, assembly, signed),
      Object.fromEntries(Object.entries(attempts).map(([k, v]) => [k, v.producer])));
    for (const [name, mutate] of [
      ['R1 incomplete', x => { x.produce.status = 'in_progress'; }],
      ['R2 wrong attempt', x => { x.assemble.producer.run_attempt++; }],
      ['R3 wrong attempt', x => { x.attest.producer.run_attempt++; }],
      ['missing bridge cell', x => { x.produce.jobs.splice(2, 1); }],
      ['duplicate job', x => { x.produce.jobs[3] = x.produce.jobs[2]; }],
      ['skipped cell', x => { x.produce.jobs[3].conclusion = 'skipped'; }],
      ['wrong job F', x => { x.attest.jobs[1].source = 'b'.repeat(40); }],
      ['R3 self-completion', x => { x.attest.producer.run_id = 5; }]
    ]) {
      const bad = structuredClone(attempts); mutate(bad);
      assert.throws(() => a.acceptanceGraph(e, bad, selected, assembly, signed), name);
    }
    assert.throws(() => a.acceptanceGraph(e, attempts, selected, assembly, { ...signed, sha256: H('changed E') }), /unchanged R2 E/);
    const index = { schema: a.INDEX_SCHEMA, matrix: e.matrix, journeys: e.journeys, bridge: e.bridge,
      files: e.journeys.flatMap(j => ['public-journey.json', 'commands.json', 'projects.json', 'npm-lifecycle.json', 'cache-process.json', 'installer.json'].map(name => ({
        path: `journeys/${j.cell}/${name}`, size: 1, sha256: name === 'public-journey.json' ? j.sha256 : H(j.cell + name) }))) };
    index.files.push(...a.BRIDGE_FILES.map(name => ({ path: 'bridge/' + name, size: 1,
      sha256: name === 'summary.json' ? e.bridge.sha256 : H('bridge/' + name) })));
    index.files.sort((a, b) => a.path < b.path ? -1 : 1);
    const indexBytes = a.encodeAcceptanceIndex(index, e);
    assert.deepEqual(a.decodeAcceptanceIndex(indexBytes, e), index);
    for (const [name, mutate] of [
      ['missing cell record', x => x.files.pop()],
      ['missing bridge', x => x.files.shift()],
      ['duplicate member', x => x.files.push(x.files[0])],
      ['closure traversal', x => { x.files[0].path = 'bridge/../summary.json'; }],
      ['index self cycle', x => { x.files[0].path = a.INDEX_FILE; }],
      ['unknown cell', x => { x.files[1].path = 'journeys/linux-other/kit-node18/public-journey.json'; }],
      ['wrong J digest', x => { x.files.find(r => r.path.endsWith('/public-journey.json')).sha256 = H('other J'); }],
      ['wrong bridge digest', x => { x.files.find(r => r.path === 'bridge/summary.json').sha256 = H('other bridge'); }],
      ['aggregate overflow', x => { x.files.forEach(r => { r.size = 16 * a.LIMIT; }); }]
    ]) {
      const bad = structuredClone(index); mutate(bad); assert.throws(() => a.encodeAcceptanceIndex(bad, e), name);
    }
    // Exercise the actual retained filesystem reader, including empty directory
    // additions. These bytes are only transport fixtures, never completed E.
    const closure = path.join(f.root, 'SYNTHETIC-closure'); fs.mkdirSync(closure);
    for (const row of index.files) {
      const i = e.journeys.findIndex(j => row.path === `journeys/${j.cell}/public-journey.json`);
      const bytes = Buffer.from(i >= 0 ? `J-${i}` : row.path === 'bridge/summary.json' ? 'bridge' : row.path);
      row.size = bytes.length; row.sha256 = c.digest(bytes);
      const file = path.join(closure, row.path); fs.mkdirSync(path.dirname(file), { recursive: true }); write(file, bytes);
    }
    const retainedIndex = a.encodeAcceptanceIndex(index, e);
    e.evidence = { path: a.INDEX_FILE, size: retainedIndex.length, sha256: c.digest(retainedIndex) };
    write(path.join(closure, a.INDEX_FILE), retainedIndex);
    write(path.join(closure, a.ACCEPTANCE_FILE), a.encodeAcceptance(e, f.inputBytes, f.stageBytes));
    assert.deepEqual(a.readAcceptanceClosure(closure, e), index);
    fs.mkdirSync(path.join(closure, 'extra-empty'));
    assert.throws(() => a.readAcceptanceClosure(closure, e), /unindexed retained directory/);
    fs.renameSync(path.join(closure, 'extra-empty'), path.join(f.root, 'retained-extra-empty'));
    const member = path.join(closure, index.files[0].path), displaced = path.join(f.root, 'retained-missing-member');
    fs.renameSync(member, displaced);
    assert.throws(() => a.readAcceptanceClosure(closure, e), /exhaustive retained closure/);
    fs.renameSync(displaced, member);
    assert.deepEqual(a.readAcceptanceClosure(closure, e), index);
    fs.appendFileSync(member, 'changed');
    assert.throws(() => a.readAcceptanceClosure(closure, e), /closure digest/);
  });
  test('C3 unit historical tools use remote namespace without local filesystem access', t => {
    const f = fixture(t), j = structuredClone(f.j); j.cell = 'windows-arm64/pair-node24';
    j.tools.host = { platform: 'win32', arch: 'arm64' }; j.tools.go = null;
    for (const key of ['orchestrator_node', 'npm_node', 'shim_node', 'npm']) {
      j.tools[key].path = `Z:\\retained remote ü\\${key}.exe`;
      if (key.endsWith('_node')) j.tools[key].version = key === 'orchestrator_node' ? 'v22.21.1' : 'v24.1.0';
    }
    const row = { ...structuredClone(j.tools), runner: {}, image: {}, observer: {}, installer_policy: {} };
    const manifest = { cells: { [j.cell]: row }, controllers: { 'windows-arm64': { node: j.tools.orchestrator_node } } };
    t.mock.method(fs, 'lstatSync', () => { throw new Error('remote path opened'); });
    t.mock.method(fs, 'readFileSync', () => { throw new Error('remote path opened'); });
    assert.deepEqual(a.historicalTools(j, manifest), j.tools);
    for (const key of ['npm_node', 'shim_node', 'npm']) {
      const bad = structuredClone(manifest); bad.cells[j.cell][key].sha256 = H('substituted');
      assert.throws(() => a.historicalTools(j, bad), /historical source-frozen/);
    }
    row.observer = null; assert.throws(() => a.historicalTools(j, manifest), /PUBLIC_PROVISIONING_REQUIRED/);
  });
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
      semanticFixture(f); const local = a.readJourneyInputs(f.request); assert.equal(local.projects.length, 10);
      assert.equal(s.readStage.mock.callCount(), 1);
      assert.throws(() => a.readJourney(f.request), /PUBLIC_FACADE_REQUIRED/);
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
      semanticFixture(f); const local = a.readJourneyInputs(f.request);
      assert.throws(() => a.verifyJourney(local), /PUBLIC_FACADE_REQUIRED/);
      assert.deepEqual(a.verifyResults(f.j, f.evidence), { fixed_commands: true, pair_parity: true, projects_preserved: true });
      const resultCases = [
        ['schema', d => { d.schema = 'other'; }], ['profile', d => { d.profiles[0].digest = 'sha256:' + H('wrong'); }],
        ['read profile', d => { d.identity.read_profile = 'other-host'; }], ['component name', d => { d.inspection.components[0].name = 'wrong'; }],
        ['component type', d => { d.inspection.components[0].type = 'wrong'; }],
        ['coverage', d => { d.coverage.filesystem = 'not_evaluated'; }],
        ['component omitted', d => d.components.pop()], ['readiness', d => { d.authoring_readiness.status = 'not_evaluated'; }],
        ['runtime claim', d => { d.runtime_evidence.status = 'pass'; }], ['release claim', d => { d.release_policy.status = 'pass'; }],
        ['diagnostic parity', d => { d.next_actions.push({ code: 'different', message: 'different' }); }],
        ['unknown nested', d => { d.identity.extra = true; }]
      ];
      for (const [name, mutate] of resultCases) {
        const evidence = structuredClone(f.evidence), row = evidence['commands.json'].find(r => r.product === 'agentplugins' && r.id === 'skill/inspect');
        const value = JSON.parse(row.stdout); mutate(value.data); row.stdout = JSON.stringify(value);
        assert.throws(() => a.verifyResults(f.j, evidence), name); t.diagnostic(name);
      }
      assert.throws(() => a.outputJSON('{"a":1,"a":2}'), /duplicate/);
      withFacades(t, () => assert.equal(a.verifyJourney(local), local));

      for (const [name, mutate] of [ ["drop row", x => x.evidence["commands.json"].pop()],
        ["arbitrary argv", x => x.evidence["commands.json"][0].argv.push("--extra")],
        ["cwd", x => { x.evidence["commands.json"][0].cwd = f.root; }],
        ["signal", x => { x.evidence["commands.json"][0].signal = "SIGTERM"; }],
        ["false result", x => { x.evidence["commands.json"][0].stdout = "{}"; }]]) {
        const bad = structuredClone(local); mutate(bad); assert.throws(() => a.verifyJourney(bad), e => !e.message.includes("PUBLIC_FACADE_REQUIRED")); t.diagnostic(name);
      }
      const file = path.join(f.j.projects.agentplugins, "skill/plugin.json"); fs.chmodSync(file, 0o400);
      assert.throws(() => a.readJourneyInputs(f.request), /original project trees/);
    });
  });
  test('C3 unit fixed generated closure and captured identity', t => {
    const f = semanticFixture(fixture(t));
    assert.ok(a.verifyResults(f.j, f.evidence).projects_preserved);
    const extraRoot = structuredClone({ 'commands.json': f.evidence['commands.json'], 'projects.json': f.evidence['projects.json'] });
    for (const snapshot of Object.values(extraRoot['projects.json'])) {
      snapshot.entries.push({ path: 'unexpected', kind: 'file', mode: 0o644, size: 0, sha256: H('') });
      snapshot.sha256 = c.digest(c.encode(snapshot.entries));
    }
    assert.throws(() => a.verifyResults(f.j, extraRoot), /only generated lane entries/);
    // Independently rendered scaffold fixtures and ordinary v1 framing pins.
    const golden = {"skill": "sha256:383668d78b12a5ad868c895183a6d86b3441c6fedc833371250eecd8e40d9754", "mcp-remote": "sha256:2aeaa18441a1e66f8b31c9beee438f88205c9dd24a4d73dd3fc24ebfce50c005", "mcp-stdio": "sha256:39908919898e31f6bd170623b8f70581aa5604f21910c334466d1fc527028e39", "hybrid-remote": "sha256:66fa98b22663011eb77e8ffbe4b73f056eb94b2706fe9f9a58244d32e0935bdb", "hybrid-stdio": "sha256:1ebd460c3872e672a6378226265f89d861160f29932a0b15ae8e376a3c576f6b"};
    for (const lane of bridge.LANES) {
      const entries = f.evidence['projects.json'].agentplugins.entries;
      for (const cell of a.matrix) {
        const hostEntries = structuredClone(entries);
        if (cell.target.startsWith('windows-')) for (const e of hostEntries) e.mode = e.kind === 'directory' ? 0o777 : 0o666;
        assert.equal(a.generatedTreeIdentity(hostEntries, lane, cell.key), golden[lane]);
      }
      for (const entry of entries.filter(e => e.path.startsWith(lane + '/'))) {
        for (const mutation of ['missing', 'mode', ...(entry.kind === 'file' ? ['content'] : [])]) {
          const bad = structuredClone({ 'commands.json': f.evidence['commands.json'], 'projects.json': f.evidence['projects.json'] });
          for (const snapshot of Object.values(bad['projects.json'])) {
            const e = snapshot.entries.find(e => e.path === entry.path);
            if (mutation === 'missing') snapshot.entries = snapshot.entries.filter(x => x.path !== e.path);
            else if (mutation === 'mode') e.mode ^= 0o100;
            else e.sha256 = H('symmetric changed content');
            snapshot.sha256 = c.digest(c.encode(snapshot.entries));
          }
          for (const snapshot of Object.values(bad['projects.json']))
            assert.throws(() => a.generatedTreeIdentity(snapshot.entries, lane, f.j.cell), e => e.message.length < 1024, `${lane} ${mutation} ${entry.path}`);
          if (entry.path === lane + '/plugin.json') assert.throws(() => a.verifyResults(f.j, bad), e => e.message.length < 1024);
        }
      }
      for (const kind of ['file', 'directory']) {
        const bad = structuredClone({ 'commands.json': f.evidence['commands.json'], 'projects.json': f.evidence['projects.json'] });
        for (const snapshot of Object.values(bad['projects.json'])) {
          snapshot.entries.push({ path: lane + '/unexpected-empty', mode: kind === 'file' ? 0o644 : 0o755, kind,
            ...(kind === 'file' ? { size: 0, sha256: H('') } : {}) });
          snapshot.sha256 = c.digest(c.encode(snapshot.entries));
        }
        assert.throws(() => a.verifyResults(f.j, bad), e => e.message.length < 1024);
      }
      const bad = structuredClone({ 'commands.json': f.evidence['commands.json'], 'projects.json': f.evidence['projects.json'] });
      for (const row of bad['commands.json'].filter(r => r.id.startsWith(lane + '/'))) {
        const v = JSON.parse(row.stdout);
        if (v.data.identity.tree_digest) v.data.identity.tree_digest = 'sha256:' + H('unbound identical claim');
        row.stdout = JSON.stringify(v);
      }
      assert.throws(() => a.verifyResults(f.j, bad), /captured generated tree identity/);
    }
  });
  test("C3 unit production installer evidence is independently required", t => {
    const f = fixture(t); withReaders(t, f, () => {
      semanticFixture(f); const local = a.readJourneyInputs(f.request);
      assert.throws(() => a.verifyJourney(local), /PUBLIC_FACADE_REQUIRED/);
      assert.throws(() => bridge.publishSeal(f.request, path.join(f.root, "must-not-exist")), /PUBLIC_FACADE_REQUIRED/);
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
    const f = fixture(t); assert.throws(() => a.readAcceptance(f.request), /PUBLIC_PROVISIONING_REQUIRED/);
    assert.throws(() => a.request({ ...f.request, expectedCommit: f.request.expectedCommit + "\n" }));
    for (const extra of [{ authenticated: true }, { completed: true }, { allowPublic: true }]) assert.throws(() => a.request({ ...f.request, ...extra }));
    const before = fs.readdirSync(f.root); let effects = 0;
    t.mock.method(require('node:child_process'), 'spawnSync', () => { effects++; throw Error('unexpected effect'); });
    for (const mode of ['--assemble', '--attest-inputs', '--read'])
      assert.throws(() => a.main([mode, f.request.journey]), /PUBLIC_PROVISIONING_REQUIRED/);
    assert.equal(effects, 0); assert.deepEqual(fs.readdirSync(f.root), before);
  });
  test("C3 unit legacy fixtures cannot qualify authentic acceptance", t => {
    const f = fixture(t);
    for (const intake of ["public-fixture/v1", "public-fixture/v2", "private"]) assert.throws(() => a.request({ ...f.request, intake }));
    for (const schema of ["dual-authoring-public-native/v1", "dual-authoring-public-native/v2", "authoring-public-packed/v1"]) assert.throws(() => a.encodeJourney({ ...f.j, schema }, f.inputBytes, f.stageBytes));
  });
  test('C3 unit npm shims postinstall and peer lifecycle contract', async t => {
    const f = semanticFixture(fixture(t)), j = f.j, roots = a.rootsFor(f.root, j.cell), original = f.evidence['npm-lifecycle.json'];
    assert.deepEqual(a.verifyNpmLifecycle(j, original, roots), { npm_lifecycle: true });
    const cases = [
      ['omitted npm row', x => x.rows.pop()], ['wrong argv', x => x.rows[0].argv.push('--ignore-scripts')],
      ['wrong cwd', x => { x.rows[0].cwd = f.root; }], ['extra env', x => { x.rows[0].env.NODE_OPTIONS = '--require evil'; }],
      ['wrong npm runtime', x => { x.rows[0].runtime = structuredClone(j.tools.orchestrator_node); x.rows[0].runtime.version = 'v18.0.0'; }],
      ['missing postinstall', x => { x.rows.find(r => r.postinstall).postinstall = null; }],
      ['postinstall wrong node', x => { x.rows.find(r => r.postinstall).postinstall.runtime.version = 'v18.0.0'; }],
      ['retained removed shim', x => { const r = x.rows.find(r => r.id.endsWith('/uninstall')); r.after.prefix.agentplugins = r.before.prefix.agentplugins; }],
      ['peer damaged', x => { const r = x.rows.find(r => r.id.includes('/shared-agentplugins/uninstall')); r.after.prefix['plugin-kit-ai'].tree = H('changed'); }],
      ['reinstall changed bytes', x => { const r = x.rows.find(r => r.id.endsWith('/reinstall')); r.after.prefix.agentplugins.tree = H('changed'); }],
      ['native missing', x => { x.rows.find(r => r.id.endsWith('/probe')).native_launches = 0; }]
    ];
    for (const [name, mutate] of cases) { const bad = structuredClone(original); mutate(bad); assert.throws(() => a.verifyNpmLifecycle(j, bad, roots), name); t.diagnostic(name); }
    for (const cell of a.matrix) {
      const contract = a.scenarioContract(cell.key); assert.ok(Object.isFrozen(contract) && Object.isFrozen(contract.npm));
      assert.equal(contract.npm.length, cell.node === 18 ? 5 : 34); assert.equal(contract.installer.length, cell.node === 18 ? 0 : 18);
      const fake = structuredClone(j); fake.cell = cell.key;
      if (cell.target.startsWith('windows-')) {
        fake.projects = Object.fromEntries(cell.products.map(p => [p, `C:\\C3\\projects\\${p} projects ü`]));
        for (const k of ['npm_node', 'shim_node', 'npm']) fake.tools[k].path = `C:\\tools\\${k}.exe`;
        const r = a.rootsFor('C:\\C3', cell.key), literal = contract.cache.find(s => s.kind === 'literal-argv');
        const invocation = a.plannedInvocation(fake, literal, r);
        assert.match(invocation.argv[0], /powershell\.exe$/); assert.ok(invocation.argv.at(-1).includes('.ps1'));
        assert.ok(invocation.argv.at(-1).includes("''single''")); assert.ok(invocation.argv.at(-1).includes('$(literal)'));
        const version = contract.cache.find(s => s.command !== null && a.commandContract(cell.key)[s.command].id === 'product-version');
        assert.ok(a.plannedInvocation(fake, version, r).argv.at(-1).includes('.cmd'));
      } else {
        const installation = a.plannedInvocation(fake, contract.npm[0], roots);
        assert.deepEqual(installation.argv.slice(2, -1), ['install', '--global', '--prefix', path.join(roots.npm, contract.npm[0].prefix, 'prefix'), '--offline', '--ignore-scripts=false', '--foreground-scripts', '--no-audit', '--no-fund']);
        const literal = a.plannedInvocation(fake, contract.cache.find(s => s.kind === 'literal-argv'), roots);
        assert.ok(literal.argv.includes(a.LITERAL_DESCRIPTION)); assert.ok(!literal.argv[0].endsWith('.js'));
      }
    }
    // The real producer must close before any output/npm/native effect when its
    // source-frozen provision is absent; no input chooses a callback or command.
    const tools = require('../scripts/public-authoring-tools'); let checks = 0;
    t.mock.method(tools, 'requireController', () => { checks++; throw new Error('PUBLIC_PROVISIONING_REQUIRED:synthetic'); });
    const before = fs.readdirSync(f.root);
    await assert.rejects(a.produceJourney({ command: ['arbitrary'], success() {} }), /PUBLIC_PROVISIONING_REQUIRED/);
    assert.equal(checks, 1); assert.deepEqual(fs.readdirSync(f.root), before); t.mock.restoreAll();
    assert.throws(() => a.requireFacades(j.cell), /public-authoring-custody.js#readPublicInputs.*public-process-observation.js#openPublicObservation.*public-installer-evidence.js#requirePublicInstaller/);
    // Real producer control flow, with opaque synthetic owner interfaces. No
    // npm/native/tool fixture is executed and no synthetic J can be emitted.
    const request = { schema: 'authoring-public-produce/v1', selected: f.admission.selected, workflow_sha: j.identity.commit,
      stage: j.stage, input_file: f.admission.input_file, repo, work_parent: f.admission.work_parent,
      output: path.join(f.root, 'producer-control'), cell: j.cell, tools: j.tools, producer: j.producer };
    const frozen = { ...j.tools, npm: { ...j.tools.npm, closure: { root: path.dirname(j.tools.npm.path) } }, mod_cache: null };
    const history = [];
    t.mock.method(tools, 'requireController', () => process.execPath);
    t.mock.method(tools, 'requireCellTools', () => frozen);
    t.mock.method(tools, 'readProvisioning', () => ({ controllers: { 'linux-amd64': { git: j.tools.go } } }));
    t.mock.method(require('node:child_process'), 'execFileSync', (file, args) => {
      assert.equal(file, j.tools.go.path); history.push('source:' + args[0]);
      if (args[0] === 'rev-parse') return Buffer.from(j.identity.commit);
      if (args[0] === 'status') return Buffer.alloc(0);
      assert.deepEqual(args, ['ls-files', '-z']); return Buffer.from('npm/agentplugins/scripts/public-authoring-acceptance.js\0');
    });
    await withFacades(t, async api => {
      api['public-authoring-custody'].readPublicInputs = () => { history.push('custody'); throw new Error('SYNTHETIC custody denial'); };
      await assert.rejects(a.produceJourney(request), /custody denial/);
      assert.equal(fs.existsSync(request.output), false);
      assert.ok(history.indexOf('source:ls-files') < history.indexOf('custody'));
      const subjects = [f.inputResult.subjects[0], ...f.inputResult.subjects.slice(1, 7)];
      for (const product of c.PRODUCTS) for (const target of c.TARGETS) {
        const file = path.join(f.inputResult.root, j.subjects[product][target].file);
        write(file, Buffer.from((product === 'agentplugins' ? '' : 'outer') + product + target));
        subjects.push({ file, sha256: hash(file) });
      }
      api['public-authoring-custody'].readPublicInputs = () => ({ stage: f.stageResult, input: { ...f.inputResult, subjects } });
      api['public-installer-evidence'].requirePublicInstaller = () => { throw new Error('SYNTHETIC installer unavailable'); };
      await assert.rejects(a.produceJourney(request), /installer unavailable/);
      assert.equal(fs.existsSync(request.output), false);
      api['public-installer-evidence'].requirePublicInstaller = () => history.push('installer-ready');
      let runs = 0, finishes = 0;
      api['public-process-observation'].openPublicObservation = ({ roots }) => {
        history.push('observation-open');
        assert.equal(fs.readFileSync(path.join(roots.npm, 'alone-agentplugins/user.npmrc'), 'utf8'), '');
        return {
          run(invocation) { runs++; assert.deepEqual(invocation, a.plannedInvocation(j, a.scenarioContract(j.cell).npm[0], roots)); throw new Error('SYNTHETIC primary run failure'); },
          cancel() { assert.fail('no cancellation scenario reached'); },
          finish() { finishes++; throw new Error('SYNTHETIC late finalization failure'); }
        };
      };
      await assert.rejects(a.produceJourney(request), /journey incomplete/);
      assert.equal(runs, 1); assert.equal(finishes, 1);
      assert.ok(history.indexOf('installer-ready') < history.indexOf('observation-open'));
      const failure = JSON.parse(fs.readFileSync(path.join(request.output, 'evidence/failure.json')));
      assert.match(failure.primary, /C3 fixed scenario failed/); assert.match(failure.primary, /primary run failure/); assert.match(failure.finalization, /late finalization failure/);
      assert.equal(fs.existsSync(path.join(request.output, 'evidence/public-journey.json')), false);
      for (const failFinish of [false, true]) {
        request.output = path.join(f.root, `malformed-observer-${failFinish}`);
        let finished = 0, ran = 0;
        api['public-process-observation'].openPublicObservation = () => ({
          run() { ran++; },
          finish() { finished++; if (failFinish) throw new Error('SYNTHETIC malformed finalizer failure'); }
        });
        await assert.rejects(a.produceJourney(request), /journey incomplete/);
        assert.equal(finished, 1); assert.equal(ran, 0);
        const receipt = JSON.parse(fs.readFileSync(path.join(request.output, 'evidence/failure.json')));
        assert.match(receipt.primary, /session.cancel/);
        if (failFinish) assert.match(receipt.finalization, /malformed finalizer failure/);
        else assert.equal(receipt.finalization, null);
        assert.equal(fs.existsSync(path.join(request.output, 'evidence/public-journey.json')), false);
      }
      for (const session of [null, { run() {}, cancel() {} }]) {
        request.output = path.join(f.root, `no-finalizer-${session === null}`);
        api['public-process-observation'].openPublicObservation = () => session;
        await assert.rejects(a.produceJourney(request), /journey incomplete/);
        const receipt = JSON.parse(fs.readFileSync(path.join(request.output, 'evidence/failure.json')));
        assert.match(receipt.primary, /session.finish/); assert.equal(receipt.finalization, null);
        assert.equal(fs.existsSync(path.join(request.output, 'evidence/public-journey.json')), false);
      }
      request.output = path.join(f.root, 'observer-open-failure');
      api['public-process-observation'].openPublicObservation = () => { throw new Error('SYNTHETIC observation unavailable'); };
      await assert.rejects(a.produceJourney(request), /observation unavailable/);
      assert.match(JSON.parse(fs.readFileSync(path.join(request.output, 'evidence/failure.json'))).primary, /observation unavailable/);
      assert.equal(fs.existsSync(path.join(request.output, 'evidence/public-journey.json')), false);
    });
    t.mock.restoreAll();

  });
  test('C3 unit cache process failure and cancellation ordering', t => {
    const f = semanticFixture(fixture(t)), j = f.j, roots = a.rootsFor(f.root, j.cell), original = f.evidence['cache-process.json'];
    const verify = value => a.verifyCacheProcess(j, value, roots, f.evidence['commands.json']);
    assert.deepEqual(verify(original), { cache_process: true, children_reaped: true });
    // Transport regression: full long-path observations exceed a 1MiB record,
    // without dropping rows or relaxing record/output/shard/aggregate limits.
    const transport = a.evidenceFiles('cache-process.json', original);
    assert.ok(transport['cache-process.json'].length <= a.LIMIT);
    assert.deepEqual(a.expandEvidence('cache-process.json', JSON.parse(transport['cache-process.json']), f.admission.journey_root), original);
    const padded = { ...original, rows: original.rows.map(r => ({ ...r, diagnostic: 'x'.repeat(130000) })) };
    const large = a.evidenceFiles('cache-process.json', padded);
    assert.ok(Object.keys(large).filter(n => n.startsWith('sidecars/')).length > 1);
    for (const [name, bytes] of Object.entries(large)) assert.ok(bytes.length <= (name.startsWith('sidecars/') ? 16 * a.LIMIT : a.LIMIT));
    const shardRoot = path.join(f.root, 'transport'); fs.mkdirSync(path.join(shardRoot, 'sidecars'), { recursive: true });
    for (const [name, bytes] of Object.entries(large)) write(path.join(shardRoot, name), bytes);
    assert.ok(require('node:util').isDeepStrictEqual(a.expandEvidence('cache-process.json', JSON.parse(large['cache-process.json']), shardRoot), padded));
    assert.throws(() => a.evidenceFiles('cache-process.json', { ...original, rows: [{ data: 'x'.repeat(a.LIMIT) }] }), /1MiB process record/);
    const index = JSON.parse(transport['cache-process.json']);
    assert.throws(() => a.expandEvidence('cache-process.json', index, f.admission.journey_root, { size: 128 * a.LIMIT }), /128MiB/);
    const badIndex = structuredClone(index); badIndex.rows.shards[0].sha256 = H('changed');
    assert.throws(() => a.expandEvidence('cache-process.json', badIndex, f.admission.journey_root), /pin/);
    const find = (x, kind) => x.rows.find(r => r.id.endsWith('/' + kind));
    const cases = [
      ['warm reacquisition', x => { find(x, 'warm').acquisitions++; }],
      ['warm wrong bytes', x => { find(x, 'warm').after.cache.agentplugins.sha256 = H('wrong'); }],
      ['invalid cold launches', x => { find(x, 'invalid-cold').native_launches = 1; }],
      ['invalid warm fallback', x => { find(x, 'invalid-warm').downloads = 1; }],
      ['uncorrupted repair input', x => { find(x, 'repair').before.cache.agentplugins = structuredClone(find(x, 'repair').after.cache.agentplugins); }],
      ['unchecked repair', x => { const r = find(x, 'repair'); r.events = r.events.filter(x => x !== 'repair-before-launch'); }],
      ['repair wrong mode', x => { find(x, 'repair').after.cache.agentplugins.mode = 0o644; }],
      ['nonoverlapping concurrency', x => { find(x, 'concurrent-cold-3').interval = [100000, 100001]; }],
      ['two concurrent commits', x => { find(x, 'concurrent-cold-2').commits++; }],
      ['missing fourth request', x => { x.rows.splice(x.rows.findIndex(r => r.id.endsWith('/concurrent-cold-3')), 1); }],
      ['signal before boundary', x => { find(x, 'SIGINT').events = ['cancel-delivered', 'reaped']; }],
      ['cancel before native', x => { find(x, 'SIGINT').events = ['shim', 'cancel-delivered', 'native', 'reaped']; }],
      ['early reap', x => { find(x, 'SIGINT').events = ['shim', 'reaped', 'native', 'cancel-delivered']; }],
      ['waiter native launch', x => { find(x, 'waiter-cancel').native_launches = 1; }],
      ['wrong cancel exit', x => { find(x, 'SIGTERM').status = 0; }],
      ['not a cache waiter', x => { const r = find(x, 'waiter-cancel'); r.events = r.events.filter(x => x !== 'cache-waiter'); }],
      ['leaked descendant', x => x.finalization.descendants.push(42)], ['owned lock leak', x => x.finalization.locks.push('lock')],
      ['late observer error', x => x.finalization.late_errors.push('denied')], ['omitted final row', x => x.finalization.rows.pop()],
      ['oversized observation', x => { x.rows[0].observation.size = 16 * a.LIMIT + 1; }],
      ['literal effect missing', x => { find(x, 'literal-argv').literal = null; }],
      ['literal expansion', x => { find(x, 'literal-argv').literal.description = 'expanded shell values'; }],
      ['literal wrong cwd effect', x => { find(x, 'literal-argv').literal.root = f.root; }],
      ['direct-bin substitution', x => { x.rows[0].argv = [j.tools.shim_node.path, 'bin/agentplugins.js']; }]
    ];
    for (const [name, mutate] of cases) { const bad = structuredClone(original); mutate(bad); assert.throws(() => verify(bad), name); t.diagnostic(name); }
    withFacades(t, api => {
      assert.equal(a.verifyJourney({ record: j, evidence: f.evidence }).record, j);
      api['public-process-observation'].verifyPublicObservation = () => true;
      assert.throws(() => a.verifyJourney({ record: j, evidence: f.evidence }), /never boolean success/);
    });
  });

}
}
