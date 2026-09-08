"use strict";
const assert = require("node:assert/strict");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const cp = require("node:child_process");
const nodeTest = require("node:test");
const test = (name, fn) => nodeTest(name, { skip: process.env.AGENTPLUGINS_STAGED_TEST_CHILD === "1" }, fn);
const c = require("../scripts/dual-authoring-candidate");
const p = require("../scripts/authoring-promotion");
const ID = { repository: c.REPOSITORY, commit: "a".repeat(40), engine_revision: "a".repeat(40),
  versions: { agentplugins: "0.1.54", "plugin-kit-ai": "2.0.0" } };
const selected = { tag: "agentplugins-v0.1.54", ref: "refs/tags/agentplugins-v0.1.54", source: ID.commit, versions: ID.versions };
const hash = text => c.digest(Buffer.from(text));
const pin = { run_id: 21, run_attempt: 2, artifact_id: 31, artifact_sha256: hash("zip fixture") };
const url = `https://github.com/${c.REPOSITORY}`;

// Text/ustar structural fixtures ONLY. No native compiler or subject execution,
// no valid terminal contract and no authentic signature proof is manufactured.
function fixture() {
  const sandbox = fs.mkdtempSync(path.join(os.tmpdir(), "promotion-"));
  const root = path.join(sandbox, "frozen"), scratch = path.join(sandbox, "provider");
  fs.mkdirSync(root); fs.mkdirSync(scratch); fs.mkdirSync(path.join(root, "candidate"));
  const manifest = { schema: c.SCHEMA, status: "CANDIDATE", identity: structuredClone(ID), asset_scope: "six-platform-pair",
    build: { method: "controlled-git-archive-go-build/v1", go_version: "go1.25.13", go_sha256: hash("go fixture"),
      source_archive_sha256: hash("source fixture"), authoring_mode: "release-cli-contract-v1" }, products: {}, release_eligible: false };
  for (const product of c.PRODUCTS) {
    const assets = {};
    for (const target of c.TARGETS) {
      const binary = Buffer.from(`NOT EXECUTABLE: ${product}/${target}`);
      const name = c.executableName(product, target), file = c.assetName(product, ID.versions[product], target);
      const bytes = product === "plugin-kit-ai" ? c.archive(binary, name) : binary;
      assets[target] = { file, ...c.metadata(bytes), binary: { file: name, ...c.metadata(binary) } };
      if (!fs.existsSync(path.join(root, product))) fs.mkdirSync(path.join(root, product));
      fs.writeFileSync(path.join(root, product, file), bytes);
    }
    manifest.products[product] = { version: ID.versions[product], assets };
  }
  const candidate = c.encode(manifest); fs.writeFileSync(path.join(root, "candidate/candidate.json"), candidate);
  const record = { schema: p.SCHEMA, identity: structuredClone(ID), authoring_mode: "release-cli-contract-v1", asset_scope: "six-platform-pair",
    candidate_sha256: c.digest(candidate), pair_marker_sha256: "", products: {}, qualification: { lanes: [] },
    producer: { workflow: p.WORKFLOW, source: ID.commit, run_id: 41, run_attempt: 3 } };
  const marker = { schema: "authoring-release-pair/v1", status: "CANDIDATE", identity: structuredClone(ID), candidate_sha256: record.candidate_sha256,
    authoring_mode: record.authoring_mode, asset_scope: record.asset_scope, products: {}, release_eligible: false, platform_acceptance: false, attested: false };
  for (const product of c.PRODUCTS) {
    const tag = `${product === "agentplugins" ? "agentplugins-" : ""}v${ID.versions[product]}`;
    const assets = manifest.products[product].assets;
    const projected = { schema_version: 3, status: "CANDIDATE", product, repository: c.REPOSITORY, tag,
      version: ID.versions[product], commit: ID.commit, engine_revision: ID.commit, versions: ID.versions,
      candidate_sha256: record.candidate_sha256, authoring_mode: record.authoring_mode, asset_scope: record.asset_scope,
      assets, release_eligible: false, platform_acceptance: false, attested: false };
    const bytes = c.encode(projected);
    const checks = Buffer.from([...Object.values(assets).map(a => `${a.sha256}  ${a.file}`), `${c.digest(bytes)}  release-manifest.json`].join("\n") + "\n");
    fs.writeFileSync(path.join(root, product, "release-manifest.json"), bytes);
    fs.writeFileSync(path.join(root, product, "checksums.txt"), checks);
    marker.products[product] = { manifest_sha256: c.digest(bytes), checksums_sha256: c.digest(checks) };
    record.products[product] = { tag, ...marker.products[product], assets };
  }
  fs.writeFileSync(path.join(root, "pair-prepared.json"), c.encode(marker)); record.pair_marker_sha256 = c.digest(c.encode(marker));
  record.qualification.lanes = p.LANES.map((lane, i) => {
    const selected = lane === "public-packed-pair" ? c.PRODUCTS.flatMap(x => c.TARGETS.map(t => [x, t])) : [lane.split("/")];
    return { lane, schema: "fixture-terminal/v1", sha256: hash(`TEST ONLY ${i}`), workflow: ".github/workflows/fixture-only.yml",
      source: ID.commit, artifact: { ...pin, artifact_id: 100 + i }, subjects: selected.map(([product, target]) => ({ product, target,
        sha256: record.products[product].assets[target].sha256, binary_sha256: record.products[product].assets[target].binary.sha256 })) };
  });
  const recordFile = path.join(sandbox, "authoring-promotion.json");
  fs.writeFileSync(recordFile, p.encodeRecord(record));
  return { sandbox, root, scratch, record, recordFile, options: { record: recordFile, root, scratch, workflow_sha: ID.commit, preparation: pin, selected } };
}
function expected(f) {
  return { name: "authoring-promotion.json", sha256: c.digest(fs.readFileSync(f.recordFile)), source: ID.commit,
    workflow_sha: "b".repeat(40), ref: "refs/tags/agentplugins-v0.1.54", run_id: 41, run_attempt: 3,
    subjects: [{ name: "authoring-promotion.json", digest: { sha256: c.digest(fs.readFileSync(f.recordFile)) } }] };
}
function verified(e) {
  return [{ verificationResult: { statement: {
    _type: "https://in-toto.io/Statement/v1", subject: structuredClone(e.subjects), predicateType: "https://slsa.dev/provenance/v1",
    predicate: { buildDefinition: { buildType: "https://actions.github.io/buildtypes/workflow/v1",
      externalParameters: { workflow: { ref: e.ref, repository: url, path: p.WORKFLOW } },
      resolvedDependencies: [{ uri: `git+${url}@${e.ref}`, digest: { gitCommit: e.source } }] },
    runDetails: { builder: { id: "https://github.com/actions/runner/github-hosted" }, metadata: { invocationId: `${url}/actions/runs/${e.run_id}/attempts/${e.run_attempt}` } } }
  } } }];
}
// Actual subprocesses execute the production orchestration. Only the test
// process redirects the hard-coded executable to a disposable script. Responses
// are deliberately synthetic, never proof that gh verified a real signature.
function provider(t, f, routes, mutation = null) {
  const script = path.join(f.sandbox, "provider-fixture.js"), log = path.join(f.sandbox, "calls.jsonl");
  const routeFile = path.join(f.sandbox, "routes.json");
  fs.writeFileSync(routeFile, JSON.stringify(routes));
  fs.writeFileSync(script, `const fs = require('node:fs');
const args = process.argv.slice(2);
fs.appendFileSync(${JSON.stringify(log)}, JSON.stringify({args,env:process.env})+'\\n');
const routes=JSON.parse(fs.readFileSync(${JSON.stringify(routeFile)}));
const mutation=${JSON.stringify(mutation)};
if(args[0]==='release' && mutation) {
  const release=Object.values(routes).find(r=>r.body?.tag_name===args[2]).body;
  const pins=mutation.assets[args[2]];
  if(args[1]==='create') { routes['graphql:tag='+args[2]].body.data.repository.release={databaseId:release.id}; release.assets=[]; }
  if(args[1]==='upload' || args[1]==='create') {
    const files=args.slice(3,args.indexOf('--repo'));
    for(const file of files.slice(0,mutation.interrupt ?? files.length)) {
      const name=require('node:path').basename(file), pin=pins.find(a=>a.name===name);
      if(!pin || release.assets.some(a=>a.name===name) || require('node:crypto').createHash('sha256').update(fs.readFileSync(file)).digest('hex')!==pin.digest.slice(7)) throw Error('immutable upload violation');
      release.assets.push(pin);
    }
    if(mutation.moveTag) routes[${JSON.stringify(endpoint('commits/v2.0.0'))}].body.sha='b'.repeat(40);
    if(mutation.replaceID) { release.id+=10000; routes['graphql:tag='+args[2]].body.data.repository.release.databaseId=release.id; routes[${JSON.stringify(endpoint('releases/'))}+release.id]={body:release}; }
  } else if(args[1]==='edit') { if(release.assets.length!==11) throw Error('premature public effect'); release.draft=false; }
  else throw Error('forbidden fixture mutation');
  fs.writeFileSync(${JSON.stringify(routeFile)},JSON.stringify(routes));
  if(mutation.interrupt!==undefined || mutation.failEdit===args[2] && args[1]==='edit') process.exit(9);
  process.exit(0);
}
if(args[0]==='--version'){console.log('gh version ${p.GH_VERSION} (fixture only)');process.exit(0)}
const key=args.includes('graphql')?'graphql:'+args.at(-1):args.includes('verify')?'verify':args.at(-1);
const r=routes[key]; if(!r) {process.stderr.write('fixture unexpected call');process.exit(9)}
if(r.exit) process.exit(r.exit);
if(r.flood) { process.stdout.write('X'.repeat(5*1024*1024)); }
else if(r.binary) process.stdout.write(Buffer.from(r.binary,'base64'));
else process.stdout.write(typeof r.body==='string'?r.body:JSON.stringify(r.body));
`);
  const spawn = cp.spawnSync;
  t.mock.method(cp, "spawnSync", function(executable, args, options) {
    assert.equal(executable, "/usr/bin/gh"); assert.equal(options.shell, false); assert.equal(options.timeout, 30000);
    assert.equal(options.killSignal, "SIGKILL"); assert.equal(options.env.PATH, "/usr/local/bin:/usr/bin:/bin");
    assert.equal(options.env.HOME, f.scratch); assert.equal(options.env.GH_CONFIG_DIR, f.scratch);
    assert.equal(options.env.GH_TOKEN, undefined); assert.equal(options.env.NODE_OPTIONS, undefined);
    return spawn(process.execPath, [script, ...args], options);
  });
  return () => fs.existsSync(log) ? fs.readFileSync(log, "utf8").trim().split("\n").map(JSON.parse) : [];
}
const endpoint = suffix => `repos/${c.REPOSITORY}/${suffix}`;
function artifactRoutes() {
  return {
    [endpoint("actions/runs/21/attempts/2")]: { body: { id: 21, run_attempt: 2, status: "completed", conclusion: "success",
      repository: { full_name: c.REPOSITORY }, head_repository: { full_name: c.REPOSITORY }, head_sha: ID.commit, path: p.WORKFLOW } },
    [endpoint("actions/artifacts/31")]: { body: { id: 31, expired: false, digest: `sha256:${pin.artifact_sha256}`, name: "test-preparation",
      workflow_run: { id: 21, head_sha: ID.commit }, size_in_bytes: Buffer.byteLength("zip fixture") } },
    [endpoint("actions/artifacts/31/zip")]: { binary: Buffer.from("zip fixture").toString("base64") }
  };
}
function releaseRoutes(f, states = ["draft", "draft"]) {
  const routes = {};
  c.PRODUCTS.forEach((product, i) => {
    const tag = f.record.products[product].tag;
    routes[endpoint(`commits/${tag}`)] = { body: { sha: ID.commit } };
    routes[`graphql:tag=${tag}`] = { body: { data: { repository: { release: states[i] === "absent" ? null : { databaseId: 200 + i } } } } };
    const assets = p.releasePins(f.record, product).map((a, j) => {
      const file = a.name === "authoring-promotion.json" ? f.recordFile : a.name === "candidate.json" ? path.join(f.root, "candidate", a.name) :
        a.name === "pair-prepared.json" ? path.join(f.root, a.name) : path.join(f.root, product, a.name);
      const bytes = fs.readFileSync(file), id = 1000 + i * 100 + j;
      routes[endpoint(`releases/assets/${id}`)] = { binary: bytes.toString("base64") };
      return { id, name: a.name, size: bytes.length, digest: `sha256:${a.sha256}`, state: "uploaded" };
    });
    routes[endpoint(`releases/${200 + i}`)] = { body: { id: 200 + i, tag_name: tag, draft: states[i] !== "public", prerelease: false, assets } };
  });
  return routes;
}

test("fixed canonical record ignores construction order but accepts no synthetic qualification", () => {
  const f = fixture(), encoded = p.encodeRecord(f.record);
  const reorder = v => Array.isArray(v) ? v.map(reorder) : v && typeof v === "object" ? Object.fromEntries(Object.entries(v).reverse().map(([k,x]) => [k,reorder(x)])) : v;
  assert.deepEqual(p.encodeRecord(reorder(f.record)), encoded);
  assert.deepEqual(p.decodeRecord(encoded), f.record);
  assert.throws(() => p.admitRecord(encoded, selected), /NATIVE_EVIDENCE_INTEGRATION_REQUIRED.*fixture-terminal/);
  assert.equal(p.frozenSubjects(f.root, f.record).length, 18);
  for (const product of c.PRODUCTS) {
    assert.equal(fs.readdirSync(path.join(f.root, product)).length, 8);
    const m = JSON.parse(fs.readFileSync(path.join(f.root, product, "release-manifest.json")));
    assert.equal(m.attested, false); assert.equal(m.platform_acceptance, false); assert.equal(m.release_eligible, false);
  }
});
for (const [label, mutate] of Object.entries({
  "extra field": r => { r.approved = true; }, "missing lane": r => { r.qualification.lanes.pop(); },
  "duplicate report": r => { r.qualification.lanes[1].sha256 = r.qualification.lanes[0].sha256; },
  "duplicate lane": r => { r.qualification.lanes[1] = r.qualification.lanes[0]; },
  "mixed version": r => { r.identity.versions["plugin-kit-ai"] = "1.2.4"; },
  "missing target": r => { delete r.products.agentplugins.assets["darwin-arm64"]; },
  "subject swap": r => { r.qualification.lanes[0].subjects[0].sha256 = hash("swapped"); },
  "inner swap": r => { r.qualification.lanes[0].subjects[0].binary_sha256 = hash("swapped"); },
  "terminal boolean": r => { r.qualification.terminal = true; },
  "run string": r => { r.producer.run_id = "41"; }, "attempt float": r => { r.producer.run_attempt = 1.5; },
  "huge run": r => { r.producer.run_id = Number.MAX_SAFE_INTEGER + 1; }, "huge asset": r => { r.products.agentplugins.assets["linux-amd64"].size = 2 ** 32; },
  "source branch": r => { r.producer.source = "main"; }, "wrong workflow": r => { r.producer.workflow = ".github/workflows/other.yml"; },
  "evidence source": r => { r.qualification.lanes[0].source = "b".repeat(40); },
  "artifact URL": r => { r.qualification.lanes[0].artifact.url = "https://example.invalid"; }
})) test(`encoder rejects ${label}`, () => { const f = fixture(); mutate(f.record); assert.throws(() => p.encodeRecord(f.record)); });

test("signed-byte decoder rejects duplicates, alternate bytes and bounds", () => {
  const f = fixture(), body = p.encodeRecord(f.record).toString();
  for (const bytes of [Buffer.from(body.replace('"schema":', '"schema": "forged",\n  "schema":')),
    Buffer.from(body + " "), Buffer.from(body.replace('"run_id": 41', '"run_id": 4.1e1')), Buffer.alloc(1024*1024+1), Buffer.from("{bad"), Buffer.from(body.replace("authoring-promotion/v1", "authoring-promotion/v2"))]) {
    assert.throws(() => p.decodeRecord(bytes));
  }
});

test("missing and unsupported native inputs reject before any subprocess or writes", t => {
  const f = fixture(), calls = provider(t, f, {});
  for (const lanes of [[], [{ lane: p.LANES[0], schema: "dual-authoring-public-native/v1" }], f.record.qualification.lanes]) {
    assert.throws(() => p.requireNativeContracts(lanes), /NATIVE_EVIDENCE_INTEGRATION_REQUIRED/);
  }
  assert.throws(() => p.promote(f.options), /NATIVE_EVIDENCE_INTEGRATION_REQUIRED/);
  assert.throws(() => p.promote(f.options, true), /NATIVE_EVIDENCE_INTEGRATION_REQUIRED/);
  assert.deepEqual(calls(), []); assert.deepEqual(fs.readdirSync(f.scratch), []);
});

test("fresh verifier subprocess binds independently selected signer revision and exact statement", t => {
  const f = fixture(), e = expected(f), calls = provider(t, f, { verify: { body: verified(e) } });
  p.verifySubject(f.recordFile, e, f.scratch);
  const args = calls()[1].args;
  for (const [flag, value] of [["--repo", c.REPOSITORY], ["--signer-digest", e.workflow_sha], ["--source-digest", ID.commit],
    ["--source-ref", e.ref], ["--signer-workflow", `github.com/${c.REPOSITORY}/${p.WORKFLOW}`], ["--cert-oidc-issuer", "https://token.actions.githubusercontent.com"]]) {
    assert.equal(args[args.indexOf(flag)+1], value);
  }
  assert(args.includes("--deny-self-hosted-runners"));
});
for (const [label, mutate] of Object.entries({
  "wrong subject": s => { s.subject[0].digest.sha256 = hash("other"); }, "wrong name": s => { s.subject[0].name = "other"; },
  "predicate": s => { s.predicateType = "test/terminal"; }, "statement": s => { s._type = "wrong"; },
  "workflow": s => { s.predicate.buildDefinition.externalParameters.workflow.path = ".github/workflows/other.yml"; },
  "source": s => { s.predicate.buildDefinition.resolvedDependencies[0].digest.gitCommit = "b".repeat(40); },
  "ref": s => { s.predicate.buildDefinition.externalParameters.workflow.ref = "refs/heads/main"; },
  "invocation": s => { s.predicate.runDetails.metadata.invocationId = `${url}/actions/runs/41/attempts/2`; },
  "runner": s => { s.predicate.runDetails.builder.id = "self-hosted"; }
})) test(`successful fixture verifier with ${label} cannot pass binding`, t => {
  const f = fixture(), e = expected(f), response = verified(e); mutate(response[0].verificationResult.statement);
  provider(t, f, { verify: { body: response } }); assert.throws(() => p.verifySubject(f.recordFile, e, f.scratch));
});
for (const response of [{ exit: 1 }, { body: "not JSON" }, { body: [] }, { flood: true }]) test(`verifier process failure ${JSON.stringify(response)}`, t => {
  const f = fixture(); provider(t, f, { verify: response }); assert.throws(() => p.verifySubject(f.recordFile, expected(f), f.scratch));
});
test("verifier timeout stops and never publishes", t => {
  const f = fixture(); let calls = 0;
  t.mock.method(cp, "spawnSync", () => { calls++; return { error: Object.assign(Error("timeout"), { code: "ETIMEDOUT" }), status: null, signal: "SIGKILL" }; });
  assert.throws(() => p.verifySubject(f.recordFile, expected(f), f.scratch), /ETIMEDOUT/); assert.equal(calls, 1);
});
test("subject mutation rejects before invoking verifier", t => {
  const f = fixture(), e = expected(f), calls = provider(t, f, {});
  fs.writeFileSync(f.recordFile, "corruption"); assert.throws(() => p.verifySubject(f.recordFile, e, f.scratch), /changed before/); assert.deepEqual(calls(), []);
});

test("exact provider run attempt and artifact ZIP acquired without extraction", t => {
  const f = fixture(), calls = provider(t, f, artifactRoutes());
  const file = p.acquireArtifact(pin, p.WORKFLOW, ID.commit, f.scratch);
  assert.equal(fs.readFileSync(file, "utf8"), "zip fixture"); assert.equal(calls().length, 6);
  assert.throws(() => p.acquireArtifact(pin, p.WORKFLOW, ID.commit, f.scratch), /already exists/);
});
for (const [label, mutate] of Object.entries({
  "wrong run": r => { r[endpoint("actions/runs/21/attempts/2")].body.id = 22; },
  "stale attempt": r => { r[endpoint("actions/runs/21/attempts/2")].body.run_attempt = 1; },
  "fork": r => { r[endpoint("actions/runs/21/attempts/2")].body.head_repository.full_name = "other/repo"; },
  "workflow": r => { r[endpoint("actions/runs/21/attempts/2")].body.path = ".github/workflows/other.yml"; },
  "source": r => { r[endpoint("actions/runs/21/attempts/2")].body.head_sha = "b".repeat(40); },
  "incomplete": r => { r[endpoint("actions/runs/21/attempts/2")].body.status = "in_progress"; },
  "failure": r => { r[endpoint("actions/runs/21/attempts/2")].body.conclusion = "failure"; },
  "swapped artifact": r => { r[endpoint("actions/artifacts/31")].body.id = 32; },
  "expired": r => { r[endpoint("actions/artifacts/31")].body.expired = true; },
  "digest": r => { r[endpoint("actions/artifacts/31")].body.digest = `sha256:${hash("other")}`; },
  "ZIP bytes": r => { r[endpoint("actions/artifacts/31/zip")].binary = Buffer.from("wrong").toString("base64"); },
  "denial": r => { r[endpoint("actions/runs/21/attempts/2")] = { exit: 1 }; }
})) test(`artifact ${label} rejects without output`, t => {
  const f = fixture(), routes = artifactRoutes(); mutate(routes); provider(t, f, routes);
  assert.throws(() => p.acquireArtifact(pin, p.WORKFLOW, ID.commit, f.scratch)); assert.deepEqual(fs.readdirSync(f.scratch), []);
});

test("modified manifest with recomputed hashes still differs from frozen projection", () => {
  const f = fixture(), file = path.join(f.root, "agentplugins/release-manifest.json");
  const m = JSON.parse(fs.readFileSync(file)); m.attested = true; const body = c.encode(m); fs.writeFileSync(file, body);
  f.record.products.agentplugins.manifest_sha256 = c.digest(body);
  assert.throws(() => p.frozenSubjects(f.root, f.record), /projection bytes/);
});
for (const states of [["absent", "absent"], ["draft", "draft"], ["public", "draft"], ["public", "public"]]) test(`pair observation ${states.join("/")} is read-only`, t => {
  const f = fixture(), calls = provider(t, f, releaseRoutes(f, states));
  const observed = p.inspectPair(f.record, f.scratch);
  assert.deepEqual(observed.states, states); assert.equal(observed.reconciliation_required, states[0] === "public" && states[1] === "draft");
  assert(calls().every(x => x.args[0] === "api"));
});
for (const [label, mutate] of Object.entries({
  "moved tag": r => { r[endpoint("commits/v2.0.0")].body.sha = "b".repeat(40); },
  "prerelease": r => { r[endpoint("releases/201")].body.prerelease = true; },
  "missing second readback": r => { r[endpoint("releases/201")] = { exit: 1 }; },
  "missing public asset": r => { r[endpoint("releases/201")].body.assets.pop(); },
  "duplicate asset": r => { r[endpoint("releases/201")].body.assets[1] = r[endpoint("releases/201")].body.assets[0]; },
  "duplicate asset ID": r => { r[endpoint("releases/201")].body.assets[1].id = 1100; },
  "wrong size type": r => { r[endpoint("releases/201")].body.assets[0].size = "42"; },
  "wrong state": r => { r[endpoint("releases/201")].body.assets[0].state = "starter"; },
  "wrong draft type": r => { r[endpoint("releases/201")].body.draft = "true"; },
  "extra asset": r => { r[endpoint("releases/201")].body.assets.push({ name: "extra" }); },
  "wrong digest": r => { r[endpoint("releases/201")].body.assets[0].digest = `sha256:${hash("other")}`; },
  "changed download": r => { r[endpoint("releases/assets/1100")].binary = Buffer.from("other").toString("base64"); },
  "uncertain not-found": r => { r["graphql:tag=v2.0.0"] = { body: { errors: [{ message: "provider denied" }] } }; }
})) test(`pair ${label} never reports success`, t => {
  const f = fixture(), routes = releaseRoutes(f, ["public", "public"]); mutate(routes); provider(t, f, routes);
  assert.throws(() => p.inspectPair(f.record, f.scratch));
});

test("CLI rejects unsupported contracts with concrete diagnostics, no local outputs", () => {
  const f = fixture(), config = path.join(f.sandbox, "contracts.json");
  fs.writeFileSync(config, JSON.stringify({ lanes: [] }));
  const result = cp.spawnSync(process.execPath, [path.resolve(__dirname, "../scripts/authoring-promotion.js"), "check-contracts", config], {
    env: { PATH: "/usr/local/bin:/usr/bin:/bin" }, cwd: f.sandbox, encoding: "utf8", timeout: 10000 });
  assert.equal(result.status, 1); assert.match(result.stderr, /NATIVE_EVIDENCE_INTEGRATION_REQUIRED.*agentplugins\/darwin-amd64.*public-packed-pair/);
  assert.equal(result.stdout, ""); assert.deepEqual(fs.readdirSync(f.scratch), []);
});

test("pinned attest multi-subject statement binds both projection basename collisions", t => {
  const f = fixture(), e = expected(f);
  e.subjects = [...p.frozenSubjects(f.root, f.record).map(s => ({ name: path.basename(s.file), digest: { sha256: s.sha256 } })), ...e.subjects];
  const response = verified(e); response[0].verificationResult.statement.subject.reverse();
  provider(t, f, { verify: { body: response } });
  assert.equal(e.subjects.length, 19); p.verifySubject(f.recordFile, e, f.scratch);
  const corrupt = verified(e); corrupt[0].verificationResult.statement.subject[0].digest.sha256 = hash("another subject");
  assert.throws(() => p.mapVerifiedOutput(JSON.stringify(corrupt), e), /subject set/);
  corrupt[0].verificationResult.statement.subject.pop();
  assert.throws(() => p.mapVerifiedOutput(JSON.stringify(corrupt), e), /subject count/);
});

// Load unchanged production source into a test-local module and expose ONLY its
// private sequencing seam and exact post-admission main tail. No native adapter is substituted and no production
// caller can inject this recheck. The callback models admission; signatures and
// provider effects still execute their real orchestration against child fixtures.
function sequencing() {
  const file = require.resolve("../scripts/authoring-promotion");
  const Module = require("node:module"), m = { exports: {} };
  const body = fs.readFileSync(file, "utf8");
  // Model only the state AFTER admission; never replace admittedInputs or make
  // the unconditional native gate positive, even inside this private VM.
  const tail = body.slice(body.indexOf("  const observed = inspectPair(state.record, state.o.scratch);", body.indexOf("function main(args)")),
    body.indexOf("\nif (require.main === module)"));
  require("node:vm").runInThisContext(Module.wrap(body.replace(/^#!.*\n/, "") +
    "\nmodule.exports = { promotePair, verifyAll, admissionTail: (state,args) => {\n" + tail + "};"), { filename: file })(m.exports, Module.createRequire(file), m, file, path.dirname(file));
  assert.equal(p.promotePair, undefined);
  return m.exports;
}
for (const count of [0, 10, 11]) test(`exact ${count}/11 draft sequencing and full control`, t => {
  const f = fixture(), routes = releaseRoutes(f), assets = Object.fromEntries(c.PRODUCTS.map((product,i) =>
    [f.record.products[product].tag, routes[endpoint(`releases/${200+i}`)].body.assets]));
  routes[endpoint("releases/200")].body.assets = assets[selected.tag].slice(0,count);
  const e = expected(f), subjects = [...p.frozenSubjects(f.root,f.record), { file: f.recordFile, sha256: e.sha256 }];
  e.workflow_sha = ID.commit; e.subjects = subjects.map(s => ({ name: path.basename(s.file), digest: { sha256: s.sha256 } }));
  routes.verify = { body: verified(e) };
  const calls = provider(t,f,routes,{assets}), seq = sequencing(); let checks = 0;
  const recheck = () => { checks++; fs.appendFileSync(path.join(f.sandbox,"calls.jsonl"), JSON.stringify({admission:checks})+"\n");
    const record = p.validateSelection(fs.readFileSync(f.recordFile),selected), state = { o:f.options,record,subjects };
    p.frozenSubjects(f.root,record); seq.verifyAll(state); return state; };
  const before = p.inspectPair(f.record,f.scratch);
  assert.equal(before.states[0],count===11 ? "draft" : "incomplete-draft");
  assert.equal(before.reconciliation_required,count!==11);
  assert.equal(before.pair[0].missing_assets.length,11-count); assert.equal(before.status,undefined);
  if(count!==11) { assert.throws(()=>seq.promotePair(recheck,false),/reconciliation required/); assert(!calls().some(c=>c.args?.[0]==="release")); }
  assert.deepEqual(seq.promotePair(recheck,count!==11).public_readback,["public","public"]);
  const effects = calls().filter(c=>c.args?.[0]==="release").map(c=>c.args);
  assert.deepEqual(effects.map(a=>a[1]),count===11 ? ["edit","edit"] : ["upload","edit","edit"]);
  if(count!==11) assert.deepEqual(effects[0].slice(3,effects[0].indexOf("--repo")).map(x=>path.basename(x)),assets[selected.tag].slice(count).map(a=>a.name));
  let segment=[];
  for(const call of calls()) {
    if(call.admission) segment=[];
    else if(call.args[0]==="release") {
      assert.equal(segment.filter(a=>a.includes("verify")).length,19);
      for(const product of c.PRODUCTS) assert(segment.some(a=>a.at(-1)===endpoint(`commits/${f.record.products[product].tag}`)));
      for(const id of [200,201]) assert(segment.some(a=>a.at(-1)===endpoint(`releases/${id}`)));
    } else segment.push(call.args);
  }
  assert(checks>=6);
});
for (const mode of ["create0", "create10", "upload", "edit", "moveTag", "replaceID", "admission", "signature"]) test(`partial ${mode} failure preserves immutable state and stops`, t => {
  const f=fixture(), routes=releaseRoutes(f,mode.startsWith("create") ? ["absent","draft"] : ["draft","draft"]);
  const assets=Object.fromEntries(c.PRODUCTS.map((product,i)=>[f.record.products[product].tag,routes[endpoint(`releases/${200+i}`)].body.assets]));
  if(!mode.startsWith("create")) routes[endpoint("releases/200")].body.assets=assets[selected.tag].slice(0,10);
  const mutation={assets,...(mode.startsWith("create") ? {interrupt:Number(mode.slice(6))} : mode==="upload" ? {interrupt:1} : mode==="edit" ? {failEdit:selected.tag} : {[mode]:true})};
  const calls=provider(t,f,routes,mutation), seq=sequencing(); let checks=0;
  const original=fs.readFileSync(f.recordFile);
  assert.throws(()=>seq.promotePair(()=>{
    checks++; if(mode==="admission" && checks===2) p.admitRecord(original,selected);
    const state={o:f.options,record:p.validateSelection(original,selected),subjects:[{file:f.recordFile,sha256:c.digest(original)}]};
    if(mode==="signature" && checks===2) seq.verifyAll(state);
    return state;
  },!mode.startsWith("create")));
  const effects=calls().filter(c=>c.args?.[0]==="release").map(c=>c.args[1]);
  assert.deepEqual(effects,["admission","signature"].includes(mode) ? [] : mode==="edit" ? ["upload","edit"] : [mode.startsWith("create") ? "create" : "upload"]);
  const after=JSON.parse(fs.readFileSync(path.join(f.sandbox,"routes.json")));
  assert.deepEqual(after[endpoint("releases/201")],routes[endpoint("releases/201")]);
  const present=after[endpoint("releases/200")].body.assets;
  assert.deepEqual(present,assets[selected.tag].slice(0,mode.startsWith("create") ? Number(mode.slice(6)) : ["admission","signature"].includes(mode) ? 10 : 11));
  assert.deepEqual(fs.readFileSync(f.recordFile),original); p.frozenSubjects(f.root,f.record);
  if(["create0","create10","upload","edit"].includes(mode)) {
    // A NEW explicit invocation reads the preserved provider state. No retry
    // occurs inside the failed invocation, even if all upload bytes arrived.
    const offset=calls().length, e=expected(f), subjects=[...p.frozenSubjects(f.root,f.record),{file:f.recordFile,sha256:e.sha256}];
    e.workflow_sha=ID.commit; e.subjects=subjects.map(s=>({name:path.basename(s.file),digest:{sha256:s.sha256}}));
    after.verify={body:verified(e)}; t.mock.restoreAll(); const resumed=provider(t,f,after,{assets});
    assert.deepEqual(seq.promotePair(()=>{
      const state={o:f.options,record:p.validateSelection(fs.readFileSync(f.recordFile),selected),subjects};
      p.frozenSubjects(f.root,state.record); seq.verifyAll(state); return state;
    },true).public_readback,["public","public"]);
    const next=resumed().slice(offset).filter(c=>c.args?.[0]==="release").map(c=>c.args[1]);
    assert.deepEqual(next,mode.startsWith("create") ? ["upload","edit","edit"] : mode==="upload" ? ["edit","edit"] : ["edit"]);
  }
});

test("actual preflight and record shells bind dispatch before native acquisition, including resume", t => {
  const f=fixture(), yaml=fs.readFileSync(path.resolve(__dirname,"../../../.github/workflows/agentplugins-release.yml"),"utf8");
  const script=name=>{ const step=yaml.slice(yaml.indexOf(`      - name: ${name}\n`));
    return step.match(/        run: \|\n((?:          .*\n|\n)+)/)[1].split("\n").map(l=>l.slice(10)).join("\n"); };
  const env={PATH:path.dirname(process.execPath)+":/usr/local/bin:/usr/bin:/bin",SOURCE_SHA:ID.commit,WORKFLOW_SHA:ID.commit,
    TAG:selected.tag,WORKFLOW_REF:selected.ref,KIT_VERSION:"2.0.0",GITHUB_REPOSITORY:c.REPOSITORY,PROMOTION_RECORD:fs.readFileSync(f.recordFile,"utf8")};
  const trap=path.join(f.sandbox,"effect-trap.js"), effects=path.join(f.sandbox,"shell-effects");
  fs.writeFileSync(trap, `require('node:child_process').spawnSync=()=>{require('node:fs').appendFileSync(${JSON.stringify(effects)},'effect');throw Error('unexpected process effect')}`);
  env.NODE_OPTIONS=`--require=${trap}`; env.RUNNER_TEMP=f.scratch;
  const cwd=path.resolve(__dirname,"../../..");
  for(const changed of [false,true]) {
    const values={...env,...(changed ? {TAG:"agentplugins-v0.1.55",WORKFLOW_REF:"refs/tags/agentplugins-v0.1.55"} : {})};
    const run=name=>cp.spawnSync("/bin/bash",["-e","-o","pipefail","-s"],{cwd,env:values,input:script(name),encoding:"utf8",timeout:10000});
    assert.equal(run("Validate promotion identity before checkout").status,0);
    for(const name of ["Reject missing native terminal contracts before protected effects","Acquire exact frozen preparation after native admission"]) {
      const result=run(name); assert.equal(result.status,1); assert.equal(result.stdout,"");
      assert.match(result.stderr,changed ? /selected promotion identity/ : /NATIVE_EVIDENCE_INTEGRATION_REQUIRED/);
      if(changed) assert.doesNotMatch(result.stderr,/NATIVE_EVIDENCE_INTEGRATION_REQUIRED/);
    }
  }
  for(const operation of ["admit","admit-reconciliation","promote","reconcile"]) for(const run of ["41","99"]) {
    const config=path.join(f.sandbox,"resume.json");
    fs.writeFileSync(config,JSON.stringify({...f.options,selected:{...selected,tag:"agentplugins-v0.1.55",ref:"refs/tags/agentplugins-v0.1.55",versions:{...ID.versions,agentplugins:"0.1.55"}}}));
    const result=cp.spawnSync(process.execPath,[require.resolve("../scripts/authoring-promotion"),operation,config],
      {cwd,env:{...env,GITHUB_RUN_ID:run,GITHUB_RUN_ATTEMPT:"3"},encoding:"utf8",timeout:10000});
    assert.equal(result.status,1); assert.equal(result.stdout,""); assert.match(result.stderr,/selected promotion identity/);
  }
  assert.equal(fs.existsSync(effects),false);
  const calls=provider(t,f,{});
  for(const mutation of [{tag:"agentplugins-v0.1.55",ref:"refs/tags/agentplugins-v0.1.55",versions:{...ID.versions,agentplugins:"0.1.55"}}, {ref:"refs/heads/main"}, {source:"b".repeat(40)}, {versions:{...ID.versions,"plugin-kit-ai":"2.0.1"}}]) {
    const options={...f.options,selected:{...selected,...mutation}};
    for(const resume of [false,true]) assert.throws(()=>p.promote(options,resume),/selected promotion identity/);
  }
  assert.deepEqual(calls(),[]); assert.deepEqual(fs.readdirSync(f.scratch),[]);
});

for(const defect of ["digest","duplicate","extra","bytes"]) test(`incomplete draft still rejects present ${defect}`,t=>{
  const f=fixture(), routes=releaseRoutes(f), assets=routes[endpoint("releases/200")].body.assets;
  assets.pop();
  if(defect==="digest") assets[0].digest=`sha256:${hash("bad")}`;
  if(defect==="duplicate") assets[1]=assets[0];
  if(defect==="extra") assets.push({name:"unexpected"});
  if(defect==="bytes") routes[endpoint("releases/assets/1000")].binary=Buffer.from("bad").toString("base64");
  const calls=provider(t,f,routes); assert.throws(()=>p.inspectPair(f.record,f.scratch));
  assert(calls().every(c=>c.args[0]==="api"));
});

// Shared offline state for route + sequence tests; all nineteen signatures use
// the real verifier orchestration, with synthetic child-provider responses.
function reconciliationFixture(t, states = ["draft", "draft"], mutation = {}) {
  const f = fixture(), routes = releaseRoutes(f, states), seq = sequencing();
  const assets = Object.fromEntries(c.PRODUCTS.map((product,i) => [f.record.products[product].tag, structuredClone(routes[endpoint(`releases/${200+i}`)].body.assets)]));
  const e = expected(f), subjects = [...p.frozenSubjects(f.root,f.record), {file:f.recordFile, sha256:e.sha256}];
  e.workflow_sha = ID.commit; e.subjects = subjects.map(s => ({name:path.basename(s.file),digest:{sha256:s.sha256}}));
  routes.verify = {body:verified(e)};
  const calls = provider(t,f,routes,{assets,...mutation});
  const change = fn => { const file=path.join(f.sandbox,"routes.json"), next=JSON.parse(fs.readFileSync(file)); fn(next); fs.writeFileSync(file,JSON.stringify(next)); };
  const state = () => ({o:f.options, record:p.validateSelection(fs.readFileSync(f.recordFile),selected), subjects});
  const recheck = () => { const s=state(); p.frozenSubjects(f.root,s.record); seq.verifyAll(s); return s; };
  const writes = () => calls().filter(c => c.args[0] === "release").map(c => c.args);
  return {f,seq,calls,change,state,recheck,writes};
}
for (const failure of ["second-edit", "final-readback"]) test(`completed public pair reconciliation route after ${failure} uncertainty`, t => {
  const b=reconciliationFixture(t,undefined,failure === "second-edit" ? {failEdit:"v2.0.0"} : {});
  assert.throws(() => b.seq.promotePair(() => {
    if (failure === "final-readback" && b.writes().filter(a => a[1] === "edit").length === 2) throw Error("interrupted final readback");
    return b.recheck();
  },false),failure === "second-edit" ? /plugin-kit-ai mutation uncertain/ : /interrupted final readback/);
  assert.deepEqual(b.writes().map(a => a[1]),["edit","edit"]);
  const offset=b.calls().length;
  const admission=b.seq.admissionTail(b.state(),["admit-reconciliation"]);
  assert.equal(admission.sign_required,false); assert.equal(admission.status,"qualified-for-promotion");
  assert.deepEqual(admission.missing_assets,[[],[]]);
  assert.equal(b.calls().slice(offset).filter(c => c.args.includes("verify")).length,19);
  assert.deepEqual(b.seq.promotePair(b.recheck,true).public_readback,["public","public"]);
  assert.equal(b.calls().slice(offset).filter(c => c.args.includes("verify")).length,38);
  assert.equal(b.calls().slice(offset).filter(c => c.args[0] === "release").length,0);
});
for (const invalid of ["absent", "incomplete-public", "signature"]) test(`reconciliation admission rejects ${invalid} without mutation`, t => {
  const b=reconciliationFixture(t,invalid === "absent" ? ["absent","absent"] : ["public","public"]);
  if (invalid === "incomplete-public") b.change(r => r[endpoint("releases/201")].body.assets.pop());
  if (invalid === "signature") b.change(r => r.verify={exit:9});
  assert.throws(() => b.seq.admissionTail(b.state(),["admit-reconciliation"]),
    invalid === "absent" ? /reconciliation requires/ : invalid === "signature" ? /trusted gh failed/ : /incomplete public release/);
  assert.deepEqual(b.writes(),[]);
});
for (const write of ["create", "upload"]) for (const when of ["first", "subsequent"]) for (const transition of ["public", "draft", "absent", "appeared"]) {
  test(`peer ${transition} before ${when} ${write} stops all subsequent mutations`, t => {
    const target=when === "first" ? 0 : 1, peer=1-target;
    const states=["draft","draft"]; states[target]=write === "create" ? "absent" : "draft";
    if (when === "first") states[peer]=transition === "draft" ? "public" : transition === "appeared" ? "absent" : "draft";
    // On subsequent writes the peer is our own verified create or upload.
    // A public->draft case starts with a public peer (no earlier write needed).
    if (when === "subsequent") states[peer]=transition === "draft" ? "public" : write === "create" ? "absent" : "draft";
    const b=reconciliationFixture(t,states); let checks=0, before;
    if (write === "upload") b.change(r => {
      r[endpoint(`releases/${200+target}`)].body.assets.pop();
      if (when === "subsequent" && transition !== "draft") r[endpoint(`releases/${200+peer}`)].body.assets.pop();
    });
    assert.throws(() => b.seq.promotePair(() => {
      if (++checks === (when === "first" ? 2 : 3)) {
        before=b.writes().length;
        b.change(r => {
          const lookup=r[`graphql:tag=${b.f.record.products[c.PRODUCTS[peer]].tag}`].body.data.repository;
          if (transition === "absent") lookup.release=null;
          else if (transition === "appeared") {
            lookup.release={databaseId:300+peer}; r[endpoint(`releases/${300+peer}`)]={body:{...r[endpoint(`releases/${200+peer}`)].body,id:300+peer}};
          } else r[endpoint(`releases/${200+peer}`)].body.draft=transition === "draft";
        });
      }
      return b.recheck();
    },!states.every(s => s === "absent")),/pair changed|release identity changed/);
    assert.equal(b.writes().length,before);
    assert.deepEqual(b.writes().map(a => a[1]),when === "subsequent" && transition !== "draft" ? [write] : []);
  });
}

test("verified own creates advance expected presence without an extra upload", t => {
  const b=reconciliationFixture(t,["absent","absent"]);
  assert.deepEqual(b.seq.promotePair(b.recheck,false).public_readback,["public","public"]);
  assert.deepEqual(b.writes().map(a => a[1]),["create","create","edit","edit"]);
  const before=b.writes().length;
  assert.equal(b.seq.admissionTail(b.state(),["admit-reconciliation"]).sign_required,false);
  assert.deepEqual(b.seq.promotePair(b.recheck,true).public_readback,["public","public"]);
  assert.equal(b.writes().length,before);
});
