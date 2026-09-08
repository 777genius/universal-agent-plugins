#!/usr/bin/env node
"use strict";

// P's fixed encoding and protected process boundary. Structural validity never
// authorizes effects. No accepted all-target native evidence issuer exists yet.
const fs = require("node:fs");
const path = require("node:path");
const cp = require("node:child_process");
const { isDeepStrictEqual: equal } = require("node:util");
const c = require("./dual-authoring-candidate");
const REPOSITORY = c.REPOSITORY;
const WORKFLOW = ".github/workflows/agentplugins-release.yml";
const URL = `https://github.com/${REPOSITORY}`;
const SIGNER = `github.com/${REPOSITORY}/${WORKFLOW}`;
const GH = "/usr/bin/gh";
const GH_VERSION = "2.83.2";
const MODE = "release-cli-contract-v1";
const SCOPE = "six-platform-pair";
const SCHEMA = "authoring-promotion/v1";
const SLSA = "https://slsa.dev/provenance/v1";
const LIMIT = 1024 * 1024;
const LANES = Object.freeze([...c.PRODUCTS.flatMap(p => c.TARGETS.map(t => `${p}/${t}`)), "public-packed-pair"]);
const fail = message => { throw new Error(message); };
const exact = (a, b, label) => { if (!equal(a, b)) fail(`${label}: binding mismatch`); };
const sha = (v, n = 64) => {
  if (typeof v !== "string" || !new RegExp(`^[0-9a-f]{${n}}$`).test(v) || /^0+$/.test(v)) fail(`exact nonzero SHA-${n === 40 ? "1 source" : "256"} required`);
  return v;
};
const integer = (v, max = Number.MAX_SAFE_INTEGER) => {
  if (!Number.isSafeInteger(v) || v < 1 || v > max) fail("bounded positive integer required");
  return v;
};
const tag = (id, p) => `${p === "agentplugins" ? "agentplugins-" : ""}v${id.versions[p]}`;
function identity(v) {
  c.identity(v); sha(v.commit, 40);
  if (v.versions["plugin-kit-ai"] !== "2.0.0" || Object.values(v.versions).some(x => x.length > 32)) fail("first-cut paired versions required");
  return { repository: REPOSITORY, commit: v.commit, engine_revision: v.commit,
    versions: { agentplugins: v.versions.agentplugins, "plugin-kit-ai": "2.0.0" } };
}
// Fixed field ordering, even when input objects were constructed in another order.
function asset(v, id, product, target) {
  c.keys(v, ["file", "sha256", "size", "binary"], "asset");
  c.keys(v.binary, ["file", "sha256", "size"], "binary");
  exact(v.file, c.assetName(product, id.versions[product], target), "asset name");
  exact(v.binary.file, c.executableName(product, target), "binary name");
  const result = { file: v.file, sha256: sha(v.sha256), size: integer(v.size, 128 * LIMIT),
    binary: { file: v.binary.file, sha256: sha(v.binary.sha256), size: integer(v.binary.size, 128 * LIMIT) } };
  if (product === "agentplugins") exact([result.sha256, result.size], [result.binary.sha256, result.binary.size], "raw executable");
  return result;
}
function producer(v, source) {
  c.keys(v, ["workflow", "source", "run_id", "run_attempt"], "producer");
  exact(v.workflow, WORKFLOW, "producer workflow"); exact(v.source, source, "producer source");
  return { workflow: WORKFLOW, source: sha(v.source, 40), run_id: integer(v.run_id), run_attempt: integer(v.run_attempt, 1000) };
}
function locator(v) {
  c.keys(v, ["run_id", "run_attempt", "artifact_id", "artifact_sha256"], "artifact locator");
  return { run_id: integer(v.run_id), run_attempt: integer(v.run_attempt, 1000),
    artifact_id: integer(v.artifact_id), artifact_sha256: sha(v.artifact_sha256) };
}
function lane(v, name, products, id) {
  c.keys(v, ["lane", "schema", "sha256", "workflow", "source", "artifact", "subjects"], "terminal evidence binding");
  exact(v.lane, name, "required lane order");
  // schema is an identifier only, not an adapter registration or trust claim.
  if (typeof v.schema !== "string" || !/^[a-z][a-z0-9-]{0,79}\/v[1-9][0-9]?$/.test(v.schema)) fail("bounded evidence schema identifier required");
  if (typeof v.workflow !== "string" || !/^\.github\/workflows\/[a-z0-9-]{1,80}\.yml$/.test(v.workflow)) fail("canonical evidence workflow path required");
  exact(v.source, id.commit, "evidence source");
  const selected = name === "public-packed-pair" ? c.PRODUCTS.flatMap(p => c.TARGETS.map(t => [p, t])) : [name.split("/")];
  const subjects = selected.map(([p, t]) => ({ product: p, target: t,
    sha256: products[p].assets[t].sha256, binary_sha256: products[p].assets[t].binary.sha256 }));
  exact(v.subjects, subjects, "frozen lane subjects");
  return { lane: name, schema: v.schema, sha256: sha(v.sha256), workflow: v.workflow,
    source: id.commit, artifact: locator(v.artifact), subjects };
}
function recordShape(v) {
  c.keys(v, ["schema", "identity", "authoring_mode", "asset_scope", "candidate_sha256", "pair_marker_sha256", "products", "qualification", "producer"], "promotion");
  exact([v.schema, v.authoring_mode, v.asset_scope], [SCHEMA, MODE, SCOPE], "promotion schema/mode/scope");
  const id = identity(v.identity);
  c.keys(v.products, c.PRODUCTS, "products");
  const products = {};
  const binaries = new Set();
  for (const p of c.PRODUCTS) {
    const value = v.products[p];
    c.keys(value, ["tag", "manifest_sha256", "checksums_sha256", "assets"], "product");
    exact(value.tag, tag(id, p), "product tag"); c.keys(value.assets, c.TARGETS, "six targets");
    products[p] = { tag: value.tag, manifest_sha256: sha(value.manifest_sha256), checksums_sha256: sha(value.checksums_sha256), assets: {} };
    for (const t of c.TARGETS) {
      const pin = asset(value.assets[t], id, p, t);
      if (binaries.has(pin.binary.sha256)) fail("duplicate product/target binary");
      binaries.add(pin.binary.sha256); products[p].assets[t] = pin;
    }
  }
  c.keys(v.qualification, ["lanes"], "qualification");
  if (!Array.isArray(v.qualification.lanes) || v.qualification.lanes.length !== LANES.length) fail(`missing required lanes: ${LANES.join(", ")}`);
  const lanes = v.qualification.lanes.map((x, i) => lane(x, LANES[i], products, id));
  if (new Set(lanes.map(x => x.sha256)).size !== LANES.length) fail("duplicate terminal report subject");
  return { schema: SCHEMA, identity: id, authoring_mode: MODE, asset_scope: SCOPE,
    candidate_sha256: sha(v.candidate_sha256), pair_marker_sha256: sha(v.pair_marker_sha256), products,
    qualification: { lanes }, producer: producer(v.producer, id.commit) };
}
function encodeRecord(value) {
  const body = c.encode(recordShape(value));
  if (body.length > LIMIT) fail("promotion record exceeds byte bound");
  return body;
}
// Canonical bytes reject duplicate keys, extra whitespace, noncanonical UTF-8,
// key order and number spellings. Objects must not be accepted as signed bytes.
function decodeRecord(body) {
  if (!Buffer.isBuffer(body) || body.length === 0 || body.length > LIMIT) fail("bounded promotion bytes required");
  const value = JSON.parse(body.toString("utf8"));
  if (!body.equals(encodeRecord(value))) fail("noncanonical promotion record");
  return recordShape(value);
}
function requireNativeContracts(lanes) {
  if (!Array.isArray(lanes) || lanes.length > LANES.length) fail("bounded terminal lane array required");
  const seen = new Set();
  for (const value of lanes) {
    if (!LANES.includes(value?.lane) || seen.has(value.lane)) fail("unknown or duplicate terminal lane");
    seen.add(value.lane);
  }
  const missing = LANES.filter(x => !seen.has(x));
  // Deliberately NO caller-supplied adapter, issuer, success boolean or policy.
  // Native owners must deliver reviewed terminal schemas AND their producer
  // source asserting the concrete native/installer and public packed gates.
  const unsupported = lanes.map(x => `${x.lane}:${String(x.schema).slice(0, 100)}`);
  fail(`NATIVE_EVIDENCE_INTEGRATION_REQUIRED: missing lanes [${missing.join(", ")}]; unsupported contracts [${unsupported.join(", ")}]. ` +
    "No accepted frozen-pair all-target terminal producer/schema is integrated. dual-authoring-public-native/v1 is Linux fixture evidence with false release claims; private-packed or SLSA build success cannot qualify this pair. Signing and promotion are disabled.");
}
function admitRecord(body) {
  const record = decodeRecord(body);
  requireNativeContracts(record.qualification.lanes);
  return record; // Unreachable until real native terminal adapters are reviewed.
}

// No executable/issuer configuration input. Offline tests replace spawnSync in
// their own process, never through production flags or inherited environment.
function gh(args, cwd, maximum = 4 * LIMIT, encoding = "utf8") {
  c.safeDirectory(cwd);
  const env = { PATH: "/usr/local/bin:/usr/bin:/bin", HOME: cwd, GH_CONFIG_DIR: cwd,
    GH_HOST: "github.com", GH_PROMPT_DISABLED: "1", GH_PAGER: "cat", GH_NO_UPDATE_NOTIFIER: "1" };
  // Only the supported workflow token context is forwarded; no auth-store reads.
  if (process.env.GITHUB_ACTIONS === "true" && process.env.GITHUB_REPOSITORY === REPOSITORY && process.env.GH_TOKEN) env.GH_TOKEN = process.env.GH_TOKEN;
  const result = cp.spawnSync(GH, args, { cwd, env, encoding, timeout: 30000,
    killSignal: "SIGKILL", maxBuffer: maximum, shell: false });
  if (result.error || result.signal || result.status !== 0) fail(`trusted gh failed (${result.error?.code || result.signal || result.status}); provider denial or uncertain state: stop, do not retry or reroute`);
  return result.stdout;
}
function cliVersion(cwd) {
  if (!gh(["--version"], cwd).startsWith(`gh version ${GH_VERSION} (`)) fail(`trusted /usr/bin/gh ${GH_VERSION} required`);
}
function api(endpoint, cwd) {
  return JSON.parse(gh(["api", "--hostname", "github.com", "-H", "Accept: application/vnd.github+json",
    "-H", "X-GitHub-Api-Version: 2022-11-28", `repos/${REPOSITORY}/${endpoint}`], cwd));
}
// The provider attempt endpoint is mandatory: latest run state cannot substitute
// for an independently selected attempt. Metadata alone does not admit evidence.
function inspectArtifact(pin, workflow, source, cwd) {
  const loc = locator(pin); sha(source, 40);
  if (typeof workflow !== "string" || !/^\.github\/workflows\/[a-z0-9-]{1,80}\.yml$/.test(workflow)) fail("approved workflow required");
  const run = api(`actions/runs/${loc.run_id}/attempts/${loc.run_attempt}`, cwd);
  if (run.id !== loc.run_id || run.run_attempt !== loc.run_attempt || run.status !== "completed" || run.conclusion !== "success" ||
      run.repository?.full_name !== REPOSITORY || run.head_repository?.full_name !== REPOSITORY ||
      run.head_sha !== source || run.path !== workflow) fail("exact successful workflow/source/run/attempt required");
  const item = api(`actions/artifacts/${loc.artifact_id}`, cwd);
  if (item.id !== loc.artifact_id || item.expired !== false || item.digest !== `sha256:${loc.artifact_sha256}` ||
      item.workflow_run?.id !== loc.run_id || item.workflow_run?.head_sha !== source ||
      typeof item.name !== "string" || item.name.length > 128) fail("artifact identity/digest/run mismatch");
  integer(item.size_in_bytes, 2 * 1024 * LIMIT);
  // Artifact metadata lacks a run_attempt field. The accepted producer adapter
  // must additionally bind its bytes to this attempt; names never supply that.
  return { run, item };
}
function acquireArtifact(pin, workflow, source, cwd) {
  cliVersion(cwd);
  const before = inspectArtifact(pin, workflow, source, cwd);
  const file = path.join(cwd, `artifact-${pin.artifact_id}.zip`);
  if (fs.existsSync(file)) fail("artifact destination already exists");
  // gh api follows the provider's supported artifact ZIP redirect. No free URL.
  // Binary output is bounded in memory, never extracted or executed here.
  const zip = gh(["api", "--hostname", "github.com", `repos/${REPOSITORY}/actions/artifacts/${pin.artifact_id}/zip`], cwd, 2 * 1024 * LIMIT, null);
  if (zip.length !== before.item.size_in_bytes || c.digest(zip) !== pin.artifact_sha256) fail("artifact ZIP digest/size mismatch");
  exact(inspectArtifact(pin, workflow, source, cwd), before, "artifact changed during acquisition");
  fs.writeFileSync(file, zip, { flag: "wx", mode: 0o400 });
  return file;
}

// Mapping for fresh `gh attestation verify --format json` results. This function
// is structural; only verifySubject calls the cryptographic boundary. B must
// never promote supplied JSON into authenticated proof by calling this mapper.
function mapVerifiedOutput(output, expected) {
  if (typeof output !== "string" || Buffer.byteLength(output) > 4 * LIMIT) fail("bounded verifier output required");
  const results = JSON.parse(output);
  if (!Array.isArray(results) || results.length !== 1) fail("one verified attestation required");
  const statement = results[0]?.verificationResult?.statement;
  if (statement?._type !== "https://in-toto.io/Statement/v1" || statement.predicateType !== SLSA) fail("verified in-toto/SLSA type mismatch");
  // actions/attest signs the complete subject-path set in one statement. Both
  // products legitimately have a release-manifest.json/checksums.txt basename.
  // Compare the entire independently pinned multiset, never merely find one hash.
  const subjects = expected.subjects;
  if (!Array.isArray(subjects) || subjects.length < 1 || subjects.length > 19) fail("bounded expected subject set required");
  const normalized = subjects.map(s => {
    c.keys(s, ["name", "digest"], "subject"); c.keys(s.digest, ["sha256"], "subject digest"); sha(s.digest.sha256);
    if (typeof s.name !== "string" || !/^[A-Za-z0-9][A-Za-z0-9._-]{0,150}$/.test(s.name)) fail("subject basename required");
    return { name: s.name, digest: { sha256: s.digest.sha256 } };
  });
  const order = list => [...list].sort((a,b) => `${a.name}:${a.digest.sha256}`.localeCompare(`${b.name}:${b.digest.sha256}`, "en"));
  if (new Set(normalized.map(s => `${s.name}:${s.digest.sha256}`)).size !== normalized.length ||
      !normalized.some(s => s.name === expected.name && s.digest.sha256 === expected.sha256)) fail("duplicate or missing selected subject");
  if (!Array.isArray(statement.subject) || statement.subject.length !== normalized.length) fail("verified subject count mismatch");
  exact(order(statement.subject), order(normalized), "verified subject set");
  const build = statement.predicate?.buildDefinition;
  if (build?.buildType !== "https://actions.github.io/buildtypes/workflow/v1") fail("verified Actions build type mismatch");
  exact(build.externalParameters?.workflow, { ref: expected.ref, repository: URL, path: WORKFLOW }, "verified workflow");
  exact(build.resolvedDependencies, [{ uri: `git+${URL}@${expected.ref}`, digest: { gitCommit: expected.source } }], "verified source");
  const run = statement.predicate?.runDetails;
  exact(run?.metadata?.invocationId, `${URL}/actions/runs/${expected.run_id}/attempts/${expected.run_attempt}`, "verified invocation");
  exact(run?.builder?.id, "https://github.com/actions/runner/github-hosted", "verified runner");
  return statement;
}
function verifySubject(file, expected, cwd) {
  c.keys(expected, ["name", "sha256", "source", "workflow_sha", "ref", "run_id", "run_attempt", "subjects"], "verification expectations");
  sha(expected.sha256); sha(expected.source, 40); sha(expected.workflow_sha, 40);
  integer(expected.run_id); integer(expected.run_attempt, 1000);
  if (expected.name !== path.basename(file) || !/^refs\/tags\/agentplugins-v(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$/.test(expected.ref)) fail("exact subject basename and agent tag ref required");
  if (c.digest(c.readFile(file)) !== expected.sha256) fail("subject changed before signature verification");
  cliVersion(cwd);
  const output = gh(["attestation", "verify", file, "--repo", REPOSITORY,
    "--signer-workflow", SIGNER, "--signer-digest", expected.workflow_sha,
    "--source-digest", expected.source, "--source-ref", expected.ref,
    "--cert-oidc-issuer", "https://token.actions.githubusercontent.com",
    "--deny-self-hosted-runners", "--predicate-type", SLSA, "--format", "json"], cwd);
  const statement = mapVerifiedOutput(output, expected);
  if (c.digest(c.readFile(file)) !== expected.sha256) fail("subject changed after signature verification");
  return statement;
}

// Projection roots keep their exact eight-file contract. Proof metadata sits
// beside them. Reconstruct the expected manifests/marker from pinned candidate
// bytes; this never builds or launches native subjects, even for Go build info.
function frozenSubjects(root, record) {
  c.safeDirectory(root);
  const candidateFile = path.join(root, "candidate", "candidate.json");
  const body = c.readFile(candidateFile, LIMIT);
  exact(c.digest(body), record.candidate_sha256, "candidate digest");
  const manifest = JSON.parse(body);
  if (!body.equals(c.encode(manifest))) fail("noncanonical candidate");
  c.manifestShape(manifest, record.identity, SCOPE, MODE);
  const result = [{ file: candidateFile, sha256: record.candidate_sha256 }];
  const products = {};
  for (const p of c.PRODUCTS) {
    const projection = path.join(root, p); c.safeDirectory(projection);
    exact(manifest.products[p].assets, record.products[p].assets, "candidate product pins");
    const m = { schema_version: 3, status: "CANDIDATE", product: p, repository: REPOSITORY,
      tag: tag(record.identity, p), version: record.identity.versions[p], commit: record.identity.commit,
      engine_revision: record.identity.commit, versions: record.identity.versions, candidate_sha256: record.candidate_sha256,
      authoring_mode: MODE, asset_scope: SCOPE, assets: manifest.products[p].assets,
      release_eligible: false, platform_acceptance: false, attested: false };
    const mBody = c.encode(m);
    const checks = Buffer.from([...Object.values(m.assets).map(a => `${a.sha256}  ${a.file}`), `${c.digest(mBody)}  release-manifest.json`].join("\n") + "\n");
    for (const [name, bytes, hash] of [["release-manifest.json", mBody, record.products[p].manifest_sha256], ["checksums.txt", checks, record.products[p].checksums_sha256]]) {
      const file = path.join(projection, name);
      exact(c.readFile(file, LIMIT), bytes, "projection bytes"); exact(c.digest(bytes), hash, "independent projection pin");
      result.push({ file, sha256: hash });
    }
    for (const a of Object.values(m.assets)) {
      const file = path.join(projection, a.file); const bytes = c.readFile(file);
      exact(c.metadata(bytes), { sha256: a.sha256, size: a.size }, "asset bytes");
      const binary = p === "plugin-kit-ai" ? c.unpack(bytes, a.binary.file) : bytes;
      exact(c.metadata(binary), { sha256: a.binary.sha256, size: a.binary.size }, "inner binary bytes");
      result.push({ file, sha256: a.sha256 });
    }
    exact(fs.readdirSync(projection).sort(), [...Object.values(m.assets).map(a => a.file), "release-manifest.json", "checksums.txt"].sort(), "projection closure");
    products[p] = { manifest_sha256: c.digest(mBody), checksums_sha256: c.digest(checks) };
  }
  const marker = { schema: "authoring-release-pair/v1", status: "CANDIDATE", identity: record.identity,
    candidate_sha256: record.candidate_sha256, authoring_mode: MODE, asset_scope: SCOPE, products,
    release_eligible: false, platform_acceptance: false, attested: false };
  const markerFile = path.join(root, "pair-prepared.json");
  exact(c.readFile(markerFile, LIMIT), c.encode(marker), "pair marker");
  exact(c.digest(c.encode(marker)), record.pair_marker_sha256, "pair marker pin");
  result.push({ file: markerFile, sha256: record.pair_marker_sha256 });
  return result; // 18 unchanged input subjects, promotion record is the 19th.
}
function releasePins(record, p) {
  return [...Object.values(record.products[p].assets).map(a => ({ name: a.file, sha256: a.sha256, size: a.size })),
    { name: "release-manifest.json", sha256: record.products[p].manifest_sha256 },
    { name: "checksums.txt", sha256: record.products[p].checksums_sha256 },
    { name: "candidate.json", sha256: record.candidate_sha256 },
    { name: "pair-prepared.json", sha256: record.pair_marker_sha256 },
    { name: "authoring-promotion.json", sha256: c.digest(encodeRecord(record)) }];
}
function checkTag(record, p, cwd) {
  // The commit endpoint peels annotated tags too; tag names are derived, never URLs.
  const result = api(`commits/${tag(record.identity, p)}`, cwd);
  exact(result.sha, record.identity.commit, "moved release tag");
}
function inspectRelease(record, p, cwd) {
  checkTag(record, p, cwd);
  // GraphQL distinguishes definitive absence from errors without treating every
  // HTTP failure as not-found. No credentials or server error text is logged.
  const query = 'query($owner:String!,$name:String!,$tag:String!){repository(owner:$owner,name:$name){release(tagName:$tag){databaseId}}}';
  const response = JSON.parse(gh(["api", "graphql", "-f", `query=${query}`, "-f", "owner=777genius", "-f", "name=universal-agent-plugins", "-f", `tag=${tag(record.identity, p)}`], cwd));
  if (response.errors || !response.data?.repository || !Object.hasOwn(response.data.repository, "release")) fail("uncertain release lookup");
  if (response.data.repository.release === null) return null;
  const id = integer(response.data.repository.release.databaseId);
  const release = api(`releases/${id}`, cwd);
  if (release.id !== id || release.tag_name !== tag(record.identity, p) || typeof release.draft !== "boolean" || release.prerelease !== false) fail("release identity/state mismatch");
  const expected = releasePins(record, p);
  if (!Array.isArray(release.assets) || release.assets.length !== expected.length) fail("release asset allowlist mismatch");
  const seen = new Set();
  for (const a of release.assets) {
    const pin = expected.find(x => x.name === a.name);
    if (!pin || seen.has(a.name) || a.state !== "uploaded" || a.digest !== `sha256:${pin.sha256}`) fail("release asset bytes/digest mismatch");
    integer(a.id); integer(a.size, 128 * LIMIT); if (pin.size !== undefined) exact(a.size, pin.size, "release size");
    const bytes = gh(["api", "--hostname", "github.com", "-H", "Accept: application/octet-stream",
      `repos/${REPOSITORY}/releases/assets/${a.id}`], cwd, 128 * LIMIT, null);
    if (bytes.length !== a.size || c.digest(bytes) !== pin.sha256) fail("downloaded release asset bytes mismatch");
    seen.add(a.name);
  }
  return release;
}
function inspectPair(record, cwd) {
  const pair = c.PRODUCTS.map(p => inspectRelease(record, p, cwd));
  // This describes observed state only; it never authorizes a write.
  const states = pair.map(r => r === null ? "absent" : r.draft ? "draft" : "public");
  return { pair, states, reconciliation_required: states.includes("public") && !states.every(s => s === "public") };
}

function options(v) {
  c.keys(v, ["record", "root", "scratch", "workflow_sha", "preparation"], "promotion options");
  for (const name of ["root", "scratch"]) c.safeDirectory(v[name]);
  if (v.root === v.scratch || v.root.startsWith(v.scratch + path.sep) || v.scratch.startsWith(v.root + path.sep)) fail("scratch and inputs must be disjoint");
  if (typeof v.record !== "string" || !path.isAbsolute(v.record) || path.basename(v.record) !== "authoring-promotion.json") fail("absolute authoring-promotion.json required");
  sha(v.workflow_sha, 40); locator(v.preparation);
  return v;
}
function admittedInputs(input) {
  const o = options(input);
  const record = admitRecord(c.readFile(o.record, LIMIT)); // Before gh, output or attestation.
  exact(o.workflow_sha, record.identity.commit, "integrated workflow source");
  cliVersion(o.scratch);
  inspectArtifact(o.preparation, WORKFLOW, record.identity.commit, o.scratch);
  // Integration must acquire/validate the accepted native terminal artifacts
  // here, including actual asserted gates and exact attempt/subject bindings.
  // requireNativeContracts remains unconditional until that implementation lands.
  const subjects = frozenSubjects(o.root, record);
  subjects.push({ file: o.record, sha256: c.digest(encodeRecord(record)) });
  return { o, record, subjects };
}
function verifyAll(state) {
  for (const subject of state.subjects) verifySubject(subject.file, {
    name: path.basename(subject.file), sha256: subject.sha256, source: state.record.identity.commit,
    workflow_sha: state.o.workflow_sha, ref: `refs/tags/${tag(state.record.identity, "agentplugins")}`,
    run_id: state.record.producer.run_id, run_attempt: state.record.producer.run_attempt,
    subjects: state.subjects.map(s => ({ name: path.basename(s.file), digest: { sha256: s.sha256 } }))
  }, state.o.scratch);
}
function promote(input, reconciliation = false) {
  let state = admittedInputs(input);
  verifyAll(state);
  let observed = inspectPair(state.record, state.o.scratch);
  if (observed.reconciliation_required && !reconciliation) fail("PARTIAL_NATIVE_PROMOTION: exact pair reconciliation required; no automatic second publication");
  if (observed.states.every(s => s === "public")) return { status: "qualified-for-promotion", public_readback: observed.states };
  if (reconciliation && !observed.reconciliation_required) fail("explicit reconciliation requires an observed partial pair");
  // Draft creation is possible only after real admission and all 19 signatures.
  for (const p of c.PRODUCTS) {
    state = admittedInputs(input); verifyAll(state);
    const existing = inspectRelease(state.record, p, state.o.scratch);
    if (existing === null) {
      const projection = path.join(state.o.root, p);
      const files = releasePins(state.record, p).map(pin => {
        if (pin.name === "authoring-promotion.json") return state.o.record;
        if (pin.name === "candidate.json") return path.join(state.o.root, "candidate", pin.name);
        if (pin.name === "pair-prepared.json") return path.join(state.o.root, pin.name);
        return path.join(projection, pin.name);
      });
      for (const product of c.PRODUCTS) checkTag(state.record, product, state.o.scratch);
      gh(["release", "create", tag(state.record.identity, p), ...files, "--repo", REPOSITORY, "--verify-tag", "--target", state.record.identity.commit,
        "--draft", "--title", tag(state.record.identity, p), "--notes", "Qualified frozen authoring pair; publication is reconciled separately."], state.o.scratch);
    } else if (!existing.draft && !reconciliation) fail("release changed during draft preparation; reconcile exact pair");
  }
  for (const p of c.PRODUCTS) {
    state = admittedInputs(input); verifyAll(state);
    observed = inspectPair(state.record, state.o.scratch);
    const index = c.PRODUCTS.indexOf(p);
    if (reconciliation && observed.states[index] === "public") continue;
    if (reconciliation) {
      if (observed.states[index] !== "draft" || observed.states[1 - index] !== "public") fail("partial pair changed before reconciliation");
    } else exact(observed.states, index === 0 ? ["draft", "draft"] : ["public", "draft"], "pair changed before publication");
    // No retry on uncertain response. The next invocation stops on partial state.
    for (const product of c.PRODUCTS) checkTag(state.record, product, state.o.scratch);
    try { gh(["release", "edit", tag(state.record.identity, p), "--repo", REPOSITORY, "--draft=false"], state.o.scratch); }
    catch { fail(`PARTIAL_NATIVE_PROMOTION: ${p} mutation uncertain; inspect both exact tags/assets before any further action`); }
  }
  state = admittedInputs(input); verifyAll(state); observed = inspectPair(state.record, state.o.scratch);
  exact(observed.states, ["public", "public"], "both public readbacks required");
  return { status: "qualified-for-promotion", public_readback: observed.states };
}
function main(args) {
  if (args.length !== 2 || !["admit", "admit-reconciliation", "promote", "reconcile", "check-contracts"].includes(args[0]) || !path.isAbsolute(args[1])) fail("usage: authoring-promotion.js <check-contracts|admit|admit-reconciliation|promote|reconcile> <absolute-config.json>");
  const value = JSON.parse(c.readFile(args[1], LIMIT));
  if (args[0] === "check-contracts") { c.keys(value, ["lanes"], "terminal contracts"); requireNativeContracts(value.lanes); }
  if (args[0] === "promote" || args[0] === "reconcile") return promote(value, args[0] === "reconcile");
  const state = admittedInputs(value);
  const observed = inspectPair(state.record, state.o.scratch);
  if (observed.reconciliation_required && args[0] !== "admit-reconciliation") fail("PARTIAL_NATIVE_PROMOTION: reconcile before signing");
  if (args[0] === "admit-reconciliation" && !observed.reconciliation_required) fail("reconciliation requires partial public pair");
  // Resuming an exact record keeps the original signing invocation. A later run
  // reuses/verifies those signatures instead of adding a different invocation.
  const signRequired = String(state.record.producer.run_id) === process.env.GITHUB_RUN_ID &&
    String(state.record.producer.run_attempt) === process.env.GITHUB_RUN_ATTEMPT;
  if (!signRequired) verifyAll(state);
  return { status: "qualified-for-promotion", subjects: state.subjects, sign_required: signRequired };
}
if (require.main === module) {
  try { process.stdout.write(JSON.stringify(main(process.argv.slice(2))) + "\n"); }
  catch (error) { process.stderr.write(`authoring promotion: ${error.message}\n`); process.exitCode = 1; }
}
module.exports = { SCHEMA, WORKFLOW, GH_VERSION, LANES, encodeRecord, decodeRecord, admitRecord, requireNativeContracts,
  inspectArtifact, acquireArtifact, mapVerifiedOutput, verifySubject, frozenSubjects, releasePins, inspectPair, promote };
