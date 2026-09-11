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
    if (executable === "/usr/bin/python3") return spawn(executable, args, options);
    assert.equal(executable, "/usr/bin/gh"); assert.equal(options.shell, false); assert.equal(options.timeout, 30000);
    assert.equal(options.killSignal, "SIGKILL"); assert.equal(options.env.PATH, "/usr/local/bin:/usr/bin:/bin");
    assert.ok(options.env.HOME === f.scratch || options.env.HOME.startsWith(f.scratch + path.sep)); assert.equal(options.env.GH_CONFIG_DIR, options.env.HOME);
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
    for(const name of ["Admit exact native evidence read-only before protected effects","Acquire exact frozen preparation after native admission"]) {
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


// Exercise the actual standard-library reader in a separate bounded process.
// ZIP contents here are synthetic control-flow evidence, never qualification.
function evidenceZIP(files) {
  const result = cp.spawnSync('/usr/bin/python3', ['-B', '-c',
    "import sys,json,base64,zipfile,io; b=io.BytesIO(); z=zipfile.ZipFile(b,'w',zipfile.ZIP_DEFLATED); " +
    "[(z.writestr(n,base64.b64decode(v))) for n,v in json.load(sys.stdin).items()]; z.close(); sys.stdout.buffer.write(b.getvalue())"], {
    input: JSON.stringify(Object.fromEntries(Object.entries(files).map(([name, bytes]) => [name, Buffer.from(bytes).toString('base64')]))),
    env: { PATH: '/usr/bin:/bin', PYTHONNOUSERSITE: '1' }, timeout: 30000, maxBuffer: 64 * 1024 * 1024
  });
  assert.equal(result.status, 0, String(result.stderr)); return result.stdout;
}
function identityMetadata(f) {
  return c.encode({ identity: ID, status: 'CANDIDATE', manifest_sha256: f.record.candidate_sha256,
    output: '/prior-builder/candidate', local_build_evidence: '/prior-builder/evidence',
    release_eligible: false, platform_acceptance: false, attested: false });
}
function preparationZIP(f, mutate = () => {}) {
  const n = require('../scripts/authoring-native-qualification');
  const pins = { identity: ID, candidate_sha256: f.record.candidate_sha256, pair_marker_sha256: f.record.pair_marker_sha256,
    products: Object.fromEntries(c.PRODUCTS.map(product => [product, {
      manifest_sha256: f.record.products[product].manifest_sha256, checksums_sha256: f.record.products[product].checksums_sha256 }])) };
  n.writePreparation(f.root, pins, { repository: c.REPOSITORY, workflow: p.WORKFLOW, source: ID.commit,
    workflow_sha: ID.commit, run_id: 21, run_attempt: 2 });
  const names = ['candidate/candidate.json', 'pair-prepared.json', 'preparation-run.json',
    ...c.PRODUCTS.flatMap(product => fs.readdirSync(path.join(f.root, product)).map(name => `${product}/${name}`))];
  const files = Object.fromEntries(names.map(name => [name, fs.readFileSync(path.join(f.root, name))]));
  files['candidate-identity.json'] = identityMetadata(f);
  mutate(files); return evidenceZIP(files);
}
function zipRoutes(body, artifact, workflow = p.WORKFLOW) {
  return {
    [endpoint(`actions/runs/${artifact.run_id}/attempts/${artifact.run_attempt}`)]: { body: {
      id: artifact.run_id, run_attempt: artifact.run_attempt, status: 'completed', conclusion: 'success',
      repository: { full_name: c.REPOSITORY }, head_repository: { full_name: c.REPOSITORY }, head_sha: ID.commit, path: workflow } },
    [endpoint(`actions/artifacts/${artifact.artifact_id}`)]: { body: {
      id: artifact.artifact_id, expired: false, digest: `sha256:${artifact.artifact_sha256}`, name: 'synthetic-only',
      workflow_run: { id: artifact.run_id, head_sha: ID.commit }, size_in_bytes: body.length } },
    [endpoint(`actions/artifacts/${artifact.artifact_id}/zip`)]: { binary: body.toString('base64') }
  };
}
function registeredNativeRecord(f) {
  for (const lane of f.record.qualification.lanes.filter(x => x.lane !== 'public-packed-pair')) {
    lane.schema = 'authoring-frozen-native/v1'; lane.workflow = '.github/workflows/authoring-frozen-native.yml';
    const index = c.TARGETS.indexOf(lane.lane.split('/')[1]);
    lane.artifact = { run_id: 100 + index, run_attempt: 2, artifact_id: 200 + index, artifact_sha256: hash(`fixture zip ${index}`) };
  }
}
function promoteEvidence(f, preparation) {
  fs.writeFileSync(f.recordFile, p.encodeRecord(f.record));
  return p.promote({ record: f.recordFile, root: f.root, scratch: f.scratch,
    workflow_sha: ID.commit, preparation, selected });
}
function noProtectedCalls(calls) {
  for (const call of calls()) {
    assert.ok(!['release', 'attestation'].includes(call.args[0]), 'rejection must cause zero protected effects');
    assert.ok(!call.args.includes('POST') && !call.args.includes('PATCH') && !call.args.includes('DELETE'));
  }
}

test('N2 independently acquired preparation ZIP closes receipt attempt and eighteen frozen subjects', t => {
  const f = fixture(), bytes = preparationZIP(f), loc = { ...pin, artifact_sha256: c.digest(bytes) };
  const calls = provider(t, f, zipRoutes(bytes, loc));
  const acquired = p.acquirePreparation(loc, f.record, f.scratch);
  assert.equal(acquired.preparation.producer.run_attempt, 2);
  assert.equal(p.frozenSubjects(acquired.root, f.record).length, 18);
  assert.equal(fs.readFileSync(path.join(acquired.root, 'candidate/candidate.json')).equals(fs.readFileSync(path.join(f.root, 'candidate/candidate.json'))), true);
  noProtectedCalls(calls);
});

for (const [name, mutate, error] of [
  ['false preparation metadata claim', files => { const v = JSON.parse(files['candidate-identity.json']); v.attested = true; files['candidate-identity.json'] = c.encode(v); }, /preparation metadata claims/],
  ['foreign preparation metadata identity', files => { const v = JSON.parse(files['candidate-identity.json']); v.identity.commit = 'b'.repeat(40); files['candidate-identity.json'] = c.encode(v); }, /preparation metadata identity/],
  ['stale embedded preparation attempt', files => { const v = JSON.parse(files['preparation-run.json']); v.producer.run_attempt = 1; files['preparation-run.json'] = c.encode(v); }, /eighteen preparation/],
  ['stale embedded preparation source', files => { const v = JSON.parse(files['preparation-run.json']); v.producer.source = 'b'.repeat(40); files['preparation-run.json'] = c.encode(v); }, /eighteen preparation/],
  ['missing receipt subject', files => { const v = JSON.parse(files['preparation-run.json']); v.subjects.pop(); files['preparation-run.json'] = c.encode(v); }, /eighteen preparation/],
  ['missing ZIP subject', files => { delete files['pair-prepared.json']; }, /ZIP extraction rejected/],
  ['unexpected ZIP subject', files => { files['unreviewed.json'] = Buffer.from('{}'); }, /ZIP extraction rejected/],
  ['changed frozen bytes', files => { files['candidate/candidate.json'] = Buffer.from('{}'); }, /candidate/]
]) test(`N2 ${name} has no protected effects`, t => {
  const f = fixture(), body = preparationZIP(f, mutate), loc = { ...pin, artifact_sha256: c.digest(body) };
  registeredNativeRecord(f);
  const calls = provider(t, f, zipRoutes(body, loc));
  assert.throws(() => promoteEvidence(f, loc), error);
  noProtectedCalls(calls);
});

test('N2 checked ZIP cannot be swapped at extraction and unsupported public never authorizes effects', t => {
  const f = fixture(), body = preparationZIP(f), loc = { ...pin, artifact_sha256: c.digest(body) };
  const calls = provider(t, f, zipRoutes(body, loc));
  const file = p.acquireArtifact(loc, p.WORKFLOW, ID.commit, f.scratch);
  const extracted = path.join(f.scratch, 'swapped');
  const names = ['transcripts.json', 'trees.json', 'build-info.json', 'preservation.json',
    'preparation.json', 'host.json', 'scans.json', 'acquisition.json', 'agentplugins-terminal.json', 'plugin-kit-ai-terminal.json'];
  const replacement = evidenceZIP(Object.fromEntries(names.map(name => [name, c.encode({ fixture: true })])));
  fs.chmodSync(file, 0o600); fs.writeFileSync(file, replacement);
  assert.throws(() => p.extractArtifact(file, loc, 'native', names, extracted, f.scratch), /ZIP extraction rejected/);
  assert.equal(fs.existsSync(extracted), false);
  registeredNativeRecord(f);
  const bodyRecord = p.encodeRecord(f.record);
  assert.equal(p.checkNativeContracts(f.record), undefined); // Registered syntax grants no authorization.
  assert.doesNotThrow(() => p.validateSelection(bodyRecord, selected));
  assert.throws(() => p.admitRecord(bodyRecord, selected), /public-packed-pair/);
  assert.throws(() => p.requireNativeContracts(f.record.qualification.lanes.slice(0, 12)), /missing lanes \[public-packed-pair\]/);
  noProtectedCalls(calls);
});

test('N2 native artifact metadata success cannot replace a valid paired terminal', t => {
  const f = fixture(), prepared = preparationZIP(f), loc = { ...pin, artifact_sha256: c.digest(prepared) };
  registeredNativeRecord(f);
  const files = Object.fromEntries(['transcripts.json', 'trees.json', 'build-info.json', 'preservation.json',
    'preparation.json', 'host.json', 'scans.json', 'acquisition.json', 'agentplugins-terminal.json', 'plugin-kit-ai-terminal.json']
    .map(name => [name, c.encode({ fixture: true, status: 'success', name })]));
  const nativeZIP = evidenceZIP(files);
  const lanes = f.record.qualification.lanes.filter(x => x.lane.endsWith('/linux-amd64'));
  for (const lane of lanes) {
    lane.artifact.artifact_sha256 = c.digest(nativeZIP);
    lane.sha256 = c.digest(files[`${lane.lane.split('/')[0]}-terminal.json`]);
  }
  const calls = provider(t, f, { ...zipRoutes(prepared, loc),
    ...zipRoutes(nativeZIP, lanes[0].artifact, '.github/workflows/authoring-frozen-native.yml') });
  assert.throws(() => promoteEvidence(f, loc), /eighteen|preparation|Expected/);
  noProtectedCalls(calls);
  assert.ok(calls().some(x => x.args.at(-1) === endpoint(`actions/artifacts/${lanes[0].artifact.artifact_id}/zip`)), 'native bytes actually acquired');
});

test('N2 provider-bound native semantic mutations reject with zero protected effects', async t => {
  // Reuse executed child fixtures, not a fabricated passing terminal generator.
  // Only the test-local reader substitutes the synthetic scanner archive pin.
  const Module = require('node:module');
  const file = path.join(__dirname, 'authoring-native-qualification.test.js');
  const source = fs.readFileSync(file, 'utf8');
  const instance = new Module(file, module);
  instance.filename = file; instance.paths = Module._nodeModulePaths(__dirname);
  instance._compile(source.slice(0, source.indexOf('test("shared verifier')) +
    '\nmodule.exports = { fixture, subprocessFixtures, internal };', file);
  const fixtureAPI = instance.exports, f = fixtureAPI.fixture();
  f.options.producer.run_id = 43;
  fixtureAPI.subprocessFixtures(t, f);
  await fixtureAPI.internal(true).produce(f.options);
  registeredNativeRecord(f);
  const preparationFiles = ['candidate/candidate.json', 'pair-prepared.json', 'preparation-run.json',
    ...c.PRODUCTS.flatMap(product => fs.readdirSync(path.join(f.root, product)).map(name => `${product}/${name}`))];
  const prepared = evidenceZIP({ ...Object.fromEntries(preparationFiles.map(name => [name, fs.readFileSync(path.join(f.root, name))])),
    'candidate-identity.json': identityMetadata(f) });
  const loc = { ...pin, run_id: 42, run_attempt: 2, artifact_sha256: c.digest(prepared) };
  const original = Object.fromEntries(fs.readdirSync(f.options.output).map(name => [name, fs.readFileSync(path.join(f.options.output, name))]));
  const lanes = f.record.qualification.lanes.filter(x => x.lane.endsWith('/linux-amd64'));
  for (const lane of lanes) lane.artifact.run_id = 43;
  const calls = provider(t, f, {});
  const realReader = require('../scripts/authoring-native-qualification');
  t.mock.method(realReader, 'readTerminals', fixtureAPI.internal(true).readTerminals);
  const routesFile = path.join(f.sandbox, 'routes.json');
  const reseal = (files, changed = null) => {
    if (changed) for (const product of c.PRODUCTS) {
      const name = `${product}-terminal.json`, terminal = JSON.parse(files[name]);
      for (const entry of terminal.evidence) if (files[entry.file]) Object.assign(entry, c.metadata(files[entry.file]));
      files[name] = c.encode(terminal);
    }
    const bytes = evidenceZIP(files);
    for (const lane of lanes) {
      lane.artifact.artifact_sha256 = c.digest(bytes);
      lane.sha256 = c.digest(files[`${lane.lane.split('/')[0]}-terminal.json`]);
    }
    return { ...zipRoutes(prepared, loc), ...zipRoutes(bytes, lanes[0].artifact, '.github/workflows/authoring-frozen-native.yml') };
  };
  const cases = [
    ['embedded stale native source', files => { const x = JSON.parse(files['agentplugins-terminal.json']); x.producer.source = 'b'.repeat(40); files['agentplugins-terminal.json'] = c.encode(x); }, /source|producer|Expected/],
    ['embedded stale native attempt', files => { const x = JSON.parse(files['agentplugins-terminal.json']); x.producer.run_attempt = 1; files['agentplugins-terminal.json'] = c.encode(x); }, /producer|run_attempt|Expected/],
    ['missing signed peer subject', files => { const x = JSON.parse(files['agentplugins-terminal.json']); delete x.peer_subject; files['agentplugins-terminal.json'] = c.encode(x); }, /closed native terminal/],
    ['wrong peer binary', files => { const x = JSON.parse(files['agentplugins-terminal.json']); x.peer_subject.binary.sha256 = 'b'.repeat(64); files['agentplugins-terminal.json'] = c.encode(x); }, /peer_subject|Expected/],
    ['success-shaped runtime claim', files => { const x = JSON.parse(files['agentplugins-terminal.json']); x.assertions.runtime = 'pass'; files['agentplugins-terminal.json'] = c.encode(x); }, /runtime|Expected/],
    ['omitted mandatory command', files => { const x = JSON.parse(files['transcripts.json']); x.agentplugins.pop(); files['transcripts.json'] = c.encode(x); }, /command|Expected/],
    ['rehashed grouped reconciliation omission', files => {
      const x = JSON.parse(files['transcripts.json']);
      const row = x.installer.find(r => r.id === 'info');
      assert.ok(row, 'mandatory info command present');
      const stdout = JSON.parse(row.stdout), client = stdout.data.clients[0];
      delete client.receipt_reconciled; delete client.native_discovery_reconciled; delete client.native_identity_state;
      row.stdout = JSON.stringify(stdout); files['transcripts.json'] = c.encode(x);
    }, /public info client matches checked registration/]
  ];
  for (const [label, mutate, error] of cases) await t.test(label, () => {
    const files = Object.fromEntries(Object.entries(original).map(([name, bytes]) => [name, Buffer.from(bytes)]));
    mutate(files); fs.writeFileSync(routesFile, JSON.stringify(reseal(files, true)));
    assert.throws(() => promoteEvidence(f, loc), error);
    noProtectedCalls(calls);
  });
  // A coherent accepted Linux pair reaches the next required target, whose
  // missing provider route fails. Linux success does not silently shrink lanes.
  fs.writeFileSync(routesFile, JSON.stringify(reseal(original)));
  assert.throws(() => promoteEvidence(f, loc), /trusted gh failed/);
  const next = c.TARGETS.find(target => target !== 'linux-amd64');
  const missing = f.record.qualification.lanes.find(lane => lane.lane === `agentplugins/${next}`).artifact;
  assert.ok(calls().some(call => call.args.at(-1) === endpoint(`actions/runs/${missing.run_id}/attempts/${missing.run_attempt}`)));
  noProtectedCalls(calls);
});


// C1 interface-only tests. Load a private test copy to stub existing module
// operations below the public wrappers. No production injection API is added,
// and no subprocess, artifact acquisition or signature is executed.
function c1PromotionInterface(t) {
  t.mock.method(cp, "spawnSync", () => assert.fail("C1 interface test cannot spawn"));
  const filename = require.resolve("../scripts/authoring-promotion");
  const Module = require("node:module"), local = new Module(filename, module);
  local.filename = filename; local.paths = module.paths;
  local._compile(fs.readFileSync(filename, "utf8") + String.raw`
const c1Calls = [];
const c1Responses = new Map();
acquireArtifact = (pin, workflow, source, cwd) => {
  c1Calls.push({operation:"acquire",pin,workflow,source,cwd});
  return path.join(cwd,"artifact-"+pin.artifact_id+".zip");
};
extractArtifact = (file,pin,kind,files,output,cwd) => {
  c1Calls.push({operation:"extract",file,pin,kind,files,output,cwd}); return output;
};
readPreparationBinding = (root,pin,record,receiptSha256) => {
  c1Calls.push({operation:"read",root,pin,record,receiptSha256});
  return {root,preparation:{sha256:receiptSha256 ?? "Q-computed-receipt",producer:invocationFor(pin,WORKFLOW,record.identity.commit)}};
};
cliVersion = cwd => c1Calls.push({operation:"version",cwd});
api = (endpoint,cwd) => { c1Calls.push({operation:"api",endpoint,cwd}); return c1Responses.get(endpoint) ?? {sha:"a".repeat(40)}; };
module.exports.c1Calls = c1Calls;
module.exports.c1Responses = c1Responses;
`, filename);
  const scratch = fs.mkdtempSync(path.join(os.tmpdir(),"q-interface-"));
  const input = { schema:"authoring-native-inputs/v1",identity:structuredClone(ID),authoring_mode:"release-cli-contract-v1",
    asset_scope:"six-platform-pair",candidate_sha256:hash("candidate"),pair_marker_sha256:hash("pair"),products:{},
    preparation:{sha256:hash("receipt"),artifact:structuredClone(pin)},
    producer:{workflow:p.WORKFLOW,source:ID.commit,run_id:41,run_attempt:3} };
  for (const product of c.PRODUCTS) {
    const assets = {};
    for (const target of c.TARGETS) {
      const binary = {file:c.executableName(product,target),sha256:hash(product+target),size:10};
      assets[target] = {file:c.assetName(product,ID.versions[product],target),
        sha256:product === "agentplugins" ? binary.sha256 : hash("outer"+target),size:10,binary};
    }
    input.products[product] = {tag:(product === "agentplugins" ? "agentplugins-v" : "v")+ID.versions[product],
      manifest_sha256:hash(product+"manifest"),checksums_sha256:hash(product+"checksums"),assets};
  }
  const {preparation:_prep,...common} = input;
  const record = {...common,schema:p.SCHEMA,qualification:{lanes:p.LANES.map((lane,i) => {
    const pairs = lane === "public-packed-pair" ? c.PRODUCTS.flatMap(product => c.TARGETS.map(target => [product,target])) : [lane.split("/")];
    return {lane,schema:"fixture-terminal/v1",sha256:hash("terminal"+i),workflow:".github/workflows/fixture-only.yml",
      source:ID.commit,artifact:{...pin,artifact_id:100+i},subjects:pairs.map(([product,target]) => ({product,target,
        sha256:input.products[product].assets[target].sha256,binary_sha256:input.products[product].assets[target].binary.sha256}))};
  })}};
  return {adapter:local.exports,scratch,input,record,body:require("../scripts/authoring-native-inputs").encodeInputs(input)};
}

test("C1 provenance preparation adapters share exact existing twenty-entry same-byte interface", t => {
  const f = c1PromotionInterface(t), a = f.adapter;
  const result = a.acquireInputPreparation(f.body,f.scratch);
  assert.deepEqual(a.c1Calls.map(c => c.operation),["acquire","extract","read"]);
  const [acquired,extracted,read] = a.c1Calls;
  assert.deepEqual(acquired.pin,f.input.preparation.artifact);
  assert.equal(acquired.workflow,p.WORKFLOW); assert.equal(acquired.source,ID.commit);
  assert.equal(extracted.file,path.join(acquired.cwd,"artifact-31.zip"));
  assert.equal(extracted.cwd,acquired.cwd); assert.equal(extracted.kind,"preparation");
  assert.equal(extracted.files.length,20); assert.equal(new Set(extracted.files).size,20);
  assert.ok(extracted.files.includes("candidate/candidate.json"));
  assert.ok(!extracted.files.includes("native-inputs.json")); assert.ok(!extracted.files.includes("authoring-promotion.json"));
  assert.equal(read.root,extracted.output); assert.deepEqual(read.record,f.input);
  assert.equal(read.receiptSha256,f.input.preparation.sha256);
  assert.deepEqual(Object.keys(result),["root","preparation"]);
  assert.deepEqual(result.preparation,{sha256:f.input.preparation.sha256,producer:{repository:c.REPOSITORY,
    workflow:p.WORKFLOW,source:ID.commit,workflow_sha:ID.commit,run_id:pin.run_id,run_attempt:pin.run_attempt}});
});

test("C1 provenance Q wrapper preserves full record validation, bytes and receipt return contract", t => {
  const f = c1PromotionInterface(t), before = p.encodeRecord(f.record), a = f.adapter;
  const result = a.acquirePreparation(pin,f.record,f.scratch);
  const read = a.c1Calls.at(-1);
  assert.deepEqual(read.record,p.decodeRecord(before)); assert.equal(read.receiptSha256,undefined);
  assert.deepEqual(p.encodeRecord(f.record),before);
  assert.deepEqual(result,{root:read.root,preparation:{sha256:"Q-computed-receipt",producer:{repository:c.REPOSITORY,
    workflow:p.WORKFLOW,source:ID.commit,workflow_sha:ID.commit,run_id:pin.run_id,run_attempt:pin.run_attempt}}});
  a.c1Calls.length = 0;
  assert.throws(() => a.acquirePreparation(pin,f.input,f.scratch));
  const missing = structuredClone(f.record); missing.qualification.lanes.pop();
  assert.throws(() => a.acquirePreparation(pin,missing,f.scratch),/missing required lanes/);
  assert.deepEqual(a.c1Calls,[]);
  assert.equal(p.LANES.length,13);
  assert.throws(() => p.requireNativeContracts(f.record.qualification.lanes),/NATIVE_EVIDENCE_INTEGRATION_REQUIRED/);
  assert.throws(() => p.requireNativeContracts(f.record.qualification.lanes.slice(0,12)),/public-packed-pair/);
  for (const product of c.PRODUCTS) {
    assert.ok(p.releasePins(f.record,product).some(row => row.name === "authoring-promotion.json"));
    assert.ok(!p.releasePins(f.record,product).some(row => row.name === "native-inputs.json"));
  }
});

test("C1 provenance fixed tag adapter reuses both existing derived release-tag endpoints", t => {
  const f = c1PromotionInterface(t); f.adapter.checkInputTags(f.body,f.scratch);
  assert.deepEqual(f.adapter.c1Calls,[{operation:"version",cwd:f.scratch},
    {operation:"api",endpoint:"commits/agentplugins-v0.1.54",cwd:f.scratch},
    {operation:"api",endpoint:"commits/v2.0.0",cwd:f.scratch}]);
});

test("C1 provenance malformed preparation adapter input rejects before any intake operation", t => {
  const f = c1PromotionInterface(t);
  for (const bytes of [null,Buffer.from("{}\n"),Buffer.concat([f.body,Buffer.from("\n")])]) {
    assert.throws(() => f.adapter.acquireInputPreparation(bytes,f.scratch));
    assert.throws(() => f.adapter.readInputPreparation(f.scratch,bytes));
    assert.throws(() => f.adapter.checkInputTags(bytes,f.scratch));
  }
  assert.deepEqual(f.adapter.c1Calls,[]); assert.deepEqual(fs.readdirSync(f.scratch),[]);
});

test("C1 provenance fixed completed-attempt inspector rejects foreign or stale provider metadata at interface level", t => {
  const f = c1PromotionInterface(t), a = f.adapter;
  const run = { id:pin.run_id,run_attempt:pin.run_attempt,status:"completed",conclusion:"success",
    repository:{full_name:c.REPOSITORY},head_repository:{full_name:c.REPOSITORY},head_sha:ID.commit,path:p.WORKFLOW };
  const item = { id:pin.artifact_id,expired:false,digest:"sha256:"+pin.artifact_sha256,
    workflow_run:{id:pin.run_id,head_sha:ID.commit},name:"fixture-only",size_in_bytes:123 };
  const select = (r,i) => {
    a.c1Responses.set("actions/runs/21/attempts/2",r);
    a.c1Responses.set("actions/artifacts/31",i); a.c1Calls.length = 0;
  };
  select(run,item);
  assert.deepEqual(a.inspectArtifact(pin,p.WORKFLOW,ID.commit,f.scratch),{run,item});
  assert.deepEqual(a.c1Calls.map(c => c.endpoint),["actions/runs/21/attempts/2","actions/artifacts/31"]);
  for (const mutate of [r => r.id++,r => r.run_attempt++,r => r.status = "in_progress",r => r.conclusion = "failure",
    r => r.repository.full_name = "fork/repo",r => r.head_repository.full_name = "fork/repo",
    r => r.head_sha = "b".repeat(40),r => r.path = ".github/workflows/other.yml"]) {
    const changed = structuredClone(run); mutate(changed); select(changed,item);
    assert.throws(() => a.inspectArtifact(pin,p.WORKFLOW,ID.commit,f.scratch),/exact successful/);
    assert.equal(a.c1Calls.length,1);
  }
  for (const mutate of [i => i.id++,i => i.expired = true,i => i.digest = "sha256:"+hash("different"),
    i => i.workflow_run.id++,i => i.workflow_run.head_sha = "b".repeat(40),i => i.size_in_bytes = 0]) {
    const changed = structuredClone(item); mutate(changed); select(run,changed);
    assert.throws(() => a.inspectArtifact(pin,p.WORKFLOW,ID.commit,f.scratch));
    assert.ok(a.c1Calls.every(c => c.operation === "api"));
  }
  assert.deepEqual(fs.readdirSync(f.scratch),[]);
});

test("C1 provenance fixed tag adapter rejects a moved second product tag", t => {
  const f = c1PromotionInterface(t);
  f.adapter.c1Responses.set("commits/v2.0.0",{sha:"b".repeat(40)});
  assert.throws(() => f.adapter.checkInputTags(f.body,f.scratch),/moved release tag/);
  assert.ok(f.adapter.c1Calls.every(c => ["version","api"].includes(c.operation)));
});

// New fixed-stage adapter tests mock the existing process interface IN MEMORY.
// No verifier execution or authentic signature compatibility is claimed.
test("C1 stage integration fixed npm signer uses existing verification interface with exact three subjects", t => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "c1-stage-signer-"));
  const rows = ["completion.json", "universal-agent-plugins-0.1.54.tgz", "plugin-kit-ai-2.0.0.tgz"].map(name => {
    const file = path.join(root, name), body = Buffer.from(`unsigned interface fixture ${name}`);
    fs.writeFileSync(file, body); return { name, file, digest: { sha256: c.digest(body) } };
  });
  let workflow = ".github/workflows/agentplugins-npm-publish.yml", calls = [], active;
  t.mock.method(cp, "spawnSync", (exe, args, options) => {
    assert.equal(exe, "/usr/bin/gh"); assert.equal(options.env.PATH, "/usr/local/bin:/usr/bin:/bin");
    calls.push(args);
    if (args[0] === "--version") return { status: 0, stdout: `gh version ${p.GH_VERSION} (interface fixture)\n` };
    assert.deepEqual(args.slice(0, 3), ["attestation", "verify", active.file]);
    assert.equal(args[args.indexOf("--signer-workflow") + 1], `github.com/${c.REPOSITORY}/.github/workflows/agentplugins-npm-publish.yml`);
    assert.equal(args[args.indexOf("--signer-digest") + 1], ID.commit);
    assert.equal(args[args.indexOf("--source-digest") + 1], ID.commit);
    assert.equal(args[args.indexOf("--source-ref") + 1], selected.ref);
    const statement = verified(active.expected); // existing output fixture shape only
    statement[0].verificationResult.statement.predicate.buildDefinition.externalParameters.workflow.path = workflow;
    return { status: 0, stdout: JSON.stringify(statement) };
  });
  for (const row of rows) {
    active = { file: row.file, expected: { name: row.name, sha256: row.digest.sha256, source: ID.commit,
      workflow_sha: ID.commit, ref: selected.ref, run_id: 501, run_attempt: 4,
      subjects: rows.map(({ name, digest }) => ({ name, digest })) } };
    assert.equal(p.verifyStageSubject(active.file, active.expected, root)._type, "https://in-toto.io/Statement/v1");
  }
  assert.equal(calls.filter(a => a[0] === "attestation").length, 3);
  workflow = p.WORKFLOW;
  assert.throws(() => p.verifyStageSubject(active.file, active.expected, root), /verified workflow/);
  for (const mutate of [e => e.workflow_sha = "b".repeat(40), e => e.subjects.pop(),
    e => e.subjects[0].name = "authoring-promotion.json", e => e.workflow = p.WORKFLOW,
    e => e.ref = "refs/heads/main", e => e.sha256 = hash("different bytes")]) {
    const expected = structuredClone(active.expected); mutate(expected); const before = calls.length;
    assert.throws(() => p.verifyStageSubject(active.file, expected, root)); assert.equal(calls.length, before);
  }
});

// New fixed workflow interfaces only. Provider/log bytes below are explicitly
// mocked orchestration evidence, never genuine custody or N2 acceptance.
function c1WorkflowProvider(t) {
  const scratch = fs.mkdtempSync(path.join(os.tmpdir(), 'c1-workflow-provider-'));
  const artifact = {run_id: 701, run_attempt: 3, artifact_id: 801, artifact_sha256: hash('opaque checked byte fixture')};
  const workflow = '.github/workflows/agentplugins-npm-publish.yml';
  const fields = {GITHUB_ACTIONS: 'true', GITHUB_EVENT_NAME: 'workflow_dispatch', GITHUB_REPOSITORY: c.REPOSITORY,
    GITHUB_SHA: selected.source, GITHUB_WORKFLOW_SHA: selected.source, GITHUB_REF: selected.ref,
    GITHUB_WORKFLOW_REF: `${c.REPOSITORY}/${workflow}@${selected.ref}`, GITHUB_RUN_ID: '701', GITHUB_RUN_ATTEMPT: '3',
    GITHUB_JOB: 'paired_stage_attestation'};
  for (const [key, value] of Object.entries(fields)) {
    const prior = process.env[key]; process.env[key] = value;
    t.after(() => {if (prior === undefined) delete process.env[key]; else process.env[key] = prior;});
  }
  const run = {id: 701, run_attempt: 3, status: 'in_progress', conclusion: null, event: 'workflow_dispatch',
    repository: {full_name: c.REPOSITORY}, head_repository: {full_name: c.REPOSITORY}, head_sha: selected.source,
    path: workflow, head_branch: selected.tag};
  const job = {id: 901, run_id: 701, run_attempt: 3, head_sha: selected.source, head_branch: selected.tag,
    name: 'paired_stage', status: 'completed', conclusion: 'success', started_at: '2026-09-09T01:00:00Z',
    completed_at: '2026-09-09T01:10:00Z', steps: ['C1 preflight', 'C1 checkout', 'C1 setup', 'C1 stage', 'C1 upload', 'C1 upload evidence']
      .map((name, i) => ({name, number: i + 2, status: 'completed', conclusion: 'success'}))};
  const signing = {...job, id: 902, name: 'paired_stage_attestation', status: 'in_progress', conclusion: null};
  const jobs = {total_count: 2, jobs: [job, signing]};
  const item = {id: 801, expired: false, digest: `sha256:${artifact.artifact_sha256}`,
    workflow_run: {id: 701, head_sha: selected.source}, size_in_bytes: Buffer.byteLength('opaque checked byte fixture'),
    name: `authoring-public-stage-${selected.source}-701-3`, created_at: '2026-09-09T01:05:00Z'};
  const records = [
    {operation: 'start', source: selected.source, ref: selected.ref, run_id: 701, run_attempt: 3,
      input_sha256: hash('I'), input_artifact: {run_id: 601, run_attempt: 2, artifact_id: 701, artifact_sha256: hash('I zip')}},
    {operation: 'pack', product: 'agentplugins', pack: {fixture_only: 'agent'}},
    {operation: 'pack', product: 'plugin-kit-ai', pack: {fixture_only: 'kit'}},
    {operation: 'completion', stage_sha256: hash('S')},
    {operation: 'upload', artifact_id: 801, artifact_sha256: artifact.artifact_sha256, stage_sha256: hash('S')}];
  const calls = [];
  t.mock.method(cp, 'spawnSync', (exe, args, options) => {
    assert.equal(exe, '/usr/bin/gh'); calls.push(args);
    let stdout;
    const endpoint = args.at(-1);
    if (args[0] === '--version') stdout = `gh version ${p.GH_VERSION} (mocked)\n`;
    else if (endpoint.endsWith('/zip')) stdout = Buffer.from('opaque checked byte fixture');
    else if (endpoint.endsWith('/logs')) stdout = records.map(r => `2026-09-09T01:06:00.000Z C1_STAGE ${JSON.stringify(r)}\n`).join('');
    else if (endpoint.endsWith('/jobs?per_page=100')) stdout = JSON.stringify(jobs);
    else if (endpoint.includes('/attempts/')) stdout = JSON.stringify(run);
    else if (endpoint.includes('/commits/')) stdout = JSON.stringify({sha: selected.source});
    else if (endpoint.endsWith('/artifacts/801')) stdout = JSON.stringify(item);
    else assert.fail(`unexpected provider request ${endpoint}`);
    return {status: 0, stdout};
  });
  return {scratch, artifact, workflow, run, job, signing, jobs, item, records, calls,
    options: {artifact, selected, workflow_sha: selected.source, scratch}};
}
test('C1 workflow current unsigned custody downloads exact same checked bytes; completed reader stays closed', t => {
  const f = c1WorkflowProvider(t);
  assert.throws(() => p.inspectArtifact(f.artifact, f.workflow, selected.source, f.scratch), /successful workflow/);
  const file = p.acquireCurrentStage(f.options);
  assert.deepEqual(fs.readFileSync(file), Buffer.from('opaque checked byte fixture'));
  assert.equal(f.calls.filter(args => args.at(-1).endsWith('/zip')).length, 1);
  assert.throws(() => p.acquireCurrentStage(f.options), /already exists/);
  assert.equal(f.calls.filter(args => args.at(-1).endsWith('/zip')).length, 1);
});
for (const defect of ['failed', 'cancelled', 'skipped', 'incomplete', 'old attempt', 'foreign run', 'foreign ref',
  'unknown caller', 'ambiguous producer', 'artifact ID', 'artifact name', 'artifact digest', 'expired', 'upload time',
  'missing log', 'duplicate pack', 'reordered packs', 'wrong upload', 'wrong completion', 'extra step', 'step failed', 'partial jobs']) {
  test(`C1 workflow fixed current-stage rejects ${defect} before download`, t => {
    const f = c1WorkflowProvider(t);
    if (['failed', 'cancelled', 'skipped'].includes(defect)) f.job.conclusion = defect;
    if (defect === 'incomplete') f.job.status = 'in_progress';
    if (defect === 'old attempt') f.run.run_attempt--;
    if (defect === 'foreign run') f.options.artifact = {...f.artifact, run_id: 700};
    if (defect === 'foreign ref') f.run.head_branch = 'main';
    if (defect === 'unknown caller') process.env.GITHUB_JOB = 'publish';
    if (defect === 'ambiguous producer') {f.jobs.jobs.push({...f.job, id: 903}); f.jobs.total_count++;}
    if (defect === 'artifact ID') f.item.id++;
    if (defect === 'artifact name') f.item.name += '-other';
    if (defect === 'artifact digest') f.item.digest = `sha256:${hash('other')}`;
    if (defect === 'expired') f.item.expired = true;
    if (defect === 'upload time') f.item.created_at = '2026-09-10T00:00:00Z';
    if (defect === 'missing log') f.records.pop();
    if (defect === 'duplicate pack') f.records.splice(2, 0, f.records[1]);
    if (defect === 'reordered packs') [f.records[1], f.records[2]] = [f.records[2], f.records[1]];
    if (defect === 'wrong upload') f.records[4].artifact_id++;
    if (defect === 'wrong completion') f.records[4].stage_sha256 = hash('different');
    if (defect === 'extra step') f.job.steps.push({name: 'npm publish', number: 20});
    if (defect === 'step failed') f.job.steps[3].conclusion = 'failure';
    if (defect === 'partial jobs') f.jobs.total_count++;
    assert.throws(() => p.acquireCurrentStage(f.options));
    assert.equal(f.calls.filter(args => args.at(-1).endsWith('/zip')).length, 0);
    assert.deepEqual(fs.readdirSync(f.scratch), []);
  });
}
test('C1 workflow current-stage cannot accept completed toggle, caller root or verifier', t => {
  const f = c1WorkflowProvider(t);
  for (const key of ['allow_incomplete', 'authenticated', 'root', 'verify', 'producer']) {
    assert.throws(() => p.acquireCurrentStage({...f.options, [key]: true}));
  }
  assert.equal(f.calls.length, 0);
});
test('C1 workflow provenance signer independently requires live provider caller and successful admission', t => {
  const f = c1WorkflowProvider(t);
  process.env.GITHUB_JOB = 'paired_input_attestation';
  process.env.GITHUB_WORKFLOW_REF = `${c.REPOSITORY}/${p.WORKFLOW}@${selected.ref}`;
  f.run.path = p.WORKFLOW; f.job.name = 'paired_input_admission'; f.signing.name = 'paired_input_attestation';
  const caller = p.inspectInputCaller(selected, selected.source, f.scratch);
  assert.deepEqual(caller, {workflow: p.WORKFLOW, source: selected.source, run_id: 701, run_attempt: 3});
  f.job.conclusion = 'failure';
  assert.throws(() => p.inspectInputCaller(selected, selected.source, f.scratch));
  assert.equal(f.calls.filter(args => args.at(-1).endsWith('/zip')).length, 0);
});
