"use strict";

// Codecs and subject enumeration are structural only. The bounded producer
// prepares unsigned I; only readInputs invokes the fixed signature boundary.
// Neither operation grants qualification, publication or execution permission.
const c = require("./dual-authoring-candidate");
const fs = require("node:fs");
const path = require("node:path");
const { TextDecoder, isDeepStrictEqual: equal } = require("node:util");

const INPUT_SCHEMA = "authoring-native-inputs/v1";
const DESCRIPTOR_SCHEMA = "dual-authoring-public-npm/v2";
const INPUT_FILE = "native-inputs.json";
const MODE = "release-cli-contract-v1";
const SCOPE = "six-platform-pair";
const WORKFLOW = ".github/workflows/agentplugins-release.yml";
const MAX_INPUT_BYTES = 1024 * 1024;
const MAX_DESCRIPTOR_BYTES = 64 * 1024;
const MAX_NATIVE_BYTES = 128 * 1024 * 1024;
const PACKAGES = Object.freeze({ agentplugins: "universal-agent-plugins", "plugin-kit-ai": "plugin-kit-ai" });

function fail(label) { throw new Error(`structural consistency only: ${label}`); }
function fixed(value, expected, label) {
  if (value !== expected) fail(`${label} mismatch`);
  return value;
}
function hash(value, label) {
  if (typeof value !== "string" || value.length !== 64 || !/^[0-9a-f]{64}$/.test(value) || /^0+$/.test(value)) fail(`${label} SHA256`);
  return value;
}
function positive(value, maximum, label) {
  if (!Number.isSafeInteger(value) || value <= 0 || value > maximum) fail(`${label} positive bounded integer`);
  return value;
}

// Only ordinary own enumerable data fields. Do not silently discard symbols,
// hidden claims or custom prototypes, or invoke accessors/toJSON while encoding.
function fields(value, names, label) {
  if (!value || typeof value !== "object" ||
      ![Object.prototype, null].includes(Object.getPrototypeOf(value))) fail(`${label} data object`);
  const own = Reflect.ownKeys(value);
  if (own.length !== names.length || own.some(key => typeof key !== "string" || !names.includes(key))) {
    fail(`${label} unexpected or missing fields`);
  }
  for (const key of own) {
    const d = Object.getOwnPropertyDescriptor(value, key);
    if (!d.enumerable || !("value" in d)) fail(`${label} own enumerable data fields`);
  }
  c.keys(value, names, label);
}

function identity(value) {
  fields(value, ["repository", "commit", "engine_revision", "versions"], "identity");
  fields(value.versions, c.PRODUCTS, "versions");
  // Bound strings before calling the existing candidate version validator.
  for (const product of c.PRODUCTS) {
    if (typeof value.versions[product] !== "string" || value.versions[product].length > MAX_INPUT_BYTES ||
        /[\r\n]/.test(value.versions[product])) {
      fail("version string limit/type");
    }
  }
  // Candidate's $-anchored regex also matches before a final newline; this
  // contract requires exactly forty source characters and stable versions.
  if (typeof value.commit !== "string" || value.commit.length !== 40) fail("exact source revision length/type");
  c.identity(value);
  if (/^0+$/.test(value.commit)) fail("nonzero source revision required");
  fixed(value.versions["plugin-kit-ai"], "2.0.0", "kit version");
  return { repository: value.repository, commit: value.commit, engine_revision: value.engine_revision,
    versions: { agentplugins: value.versions.agentplugins, "plugin-kit-ai": value.versions["plugin-kit-ai"] } };
}

function pin(value, file, label) {
  fields(value, ["file", "sha256", "size"], label);
  return { file: fixed(value.file, file, `${label} file`), sha256: hash(value.sha256, label),
    size: positive(value.size, MAX_NATIVE_BYTES, `${label} size`) };
}

function inputs(value) {
  fields(value, ["schema", "identity", "authoring_mode", "asset_scope", "candidate_sha256",
    "pair_marker_sha256", "products", "preparation", "producer"], "inputs");
  fixed(value.schema, INPUT_SCHEMA, "input schema");
  const id = identity(value.identity);
  fixed(value.authoring_mode, MODE, "authoring mode");
  fixed(value.asset_scope, SCOPE, "asset scope");
  fields(value.products, c.PRODUCTS, "products");
  const products = {}, binaries = new Set();
  for (const product of c.PRODUCTS) {
    const p = value.products[product], assets = {};
    fields(p, ["tag", "manifest_sha256", "checksums_sha256", "assets"], "product");
    const tag = (product === "agentplugins" ? "agentplugins-v" : "v") + id.versions[product];
    fixed(p.tag, tag, "product tag");
    fields(p.assets, c.TARGETS, "assets");
    for (const target of c.TARGETS) {
      const a = p.assets[target];
      fields(a, ["file", "sha256", "size", "binary"], "asset");
      const file = c.assetName(product, id.versions[product], target);
      const binary = pin(a.binary, c.executableName(product, target), "binary");
      const outer = pin({ file: a.file, sha256: a.sha256, size: a.size }, file, "asset");
      if (product === "agentplugins" && (outer.sha256 !== binary.sha256 || outer.size !== binary.size)) {
        fail("raw agent outer/inner pins disagree");
      }
      if (binaries.has(binary.sha256)) fail("twelve distinct binary hashes required");
      binaries.add(binary.sha256);
      assets[target] = { file: outer.file, sha256: outer.sha256, size: outer.size, binary };
    }
    products[product] = { tag, manifest_sha256: hash(p.manifest_sha256, "manifest"),
      checksums_sha256: hash(p.checksums_sha256, "checksums"), assets };
  }
  const prep = value.preparation, producer = value.producer;
  fields(prep, ["sha256", "artifact"], "preparation");
  fields(prep.artifact, ["run_id", "run_attempt", "artifact_id", "artifact_sha256"], "preparation artifact");
  const a = prep.artifact;
  fields(producer, ["workflow", "source", "run_id", "run_attempt"], "producer");
  return { schema: INPUT_SCHEMA, identity: id, authoring_mode: MODE, asset_scope: SCOPE,
    candidate_sha256: hash(value.candidate_sha256, "candidate"), pair_marker_sha256: hash(value.pair_marker_sha256, "pair marker"),
    products, preparation: { sha256: hash(prep.sha256, "preparation"), artifact: {
      run_id: positive(a.run_id, Number.MAX_SAFE_INTEGER, "preparation run"),
      run_attempt: positive(a.run_attempt, 1000, "preparation attempt"),
      artifact_id: positive(a.artifact_id, Number.MAX_SAFE_INTEGER, "preparation artifact ID"),
      artifact_sha256: hash(a.artifact_sha256, "preparation artifact") } },
    producer: { workflow: fixed(producer.workflow, WORKFLOW, "producer workflow"),
      source: fixed(producer.source, id.commit, "producer source"),
      run_id: positive(producer.run_id, Number.MAX_SAFE_INTEGER, "producer run"),
      run_attempt: positive(producer.run_attempt, 1000, "producer attempt") } };
}

function bytes(value, maximum) {
  if (!Buffer.isBuffer(value) || value.length === 0 || value.length > maximum) fail("nonempty bounded Buffer required");
  return value;
}
function encoded(value, maximum) {
  return bytes(c.encode(value), maximum);
}
function parsed(body, maximum) {
  bytes(body, maximum);
  const text = new TextDecoder("utf-8", { fatal: true, ignoreBOM: true }).decode(body);
  // Both fixed contracts have objects only, at most six levels deep. Bound
  // nesting before JSON.parse; canonical re-encoding rejects duplicate keys,
  // escapes, alternate number spellings, whitespace, BOM and trailing data.
  let depth = 0, quoted = false, escaped = false;
  for (const ch of text) {
    if (quoted) {
      if (escaped) escaped = false;
      else if (ch === "\\") escaped = true;
      else if (ch === '"') quoted = false;
    } else if (ch === '"') quoted = true;
    else if (ch === "[") fail("arrays are outside the fixed contracts");
    else if (ch === "{" && ++depth > 6) fail("object depth limit");
    else if (ch === "}") depth--;
  }
  return JSON.parse(text);
}

/** Encode a complete I object in fixed order. Structural consistency only. */
function encodeInputs(value) { return encoded(inputs(value), MAX_INPUT_BYTES); }

/** Decode canonical I bytes to a fresh data object. Structural consistency only. */
function decodeInputs(body) {
  const value = inputs(parsed(body, MAX_INPUT_BYTES));
  if (!body.equals(encoded(value, MAX_INPUT_BYTES))) fail("noncanonical input bytes");
  return value;
}

function descriptor(value, inputBytes, product) {
  if (!c.PRODUCTS.includes(product)) fail("explicit selected product required");
  const input = decodeInputs(inputBytes);
  fields(value, ["schema", "product", "npm_package", "identity", "authoring_mode", "asset_scope",
    "candidate_sha256", "release_manifest_sha256", "input_binding"], "descriptor");
  fixed(value.schema, DESCRIPTOR_SCHEMA, "descriptor schema");
  fixed(value.product, product, "selected product");
  fixed(value.npm_package, PACKAGES[product], "npm package");
  const id = identity(value.identity);
  if (!c.encode(id).equals(c.encode(input.identity))) fail("descriptor/input identity mismatch");
  fields(value.input_binding, ["file", "sha256"], "input binding");
  return { schema: DESCRIPTOR_SCHEMA, product, npm_package: PACKAGES[product], identity: id,
    authoring_mode: fixed(value.authoring_mode, MODE, "descriptor mode"),
    asset_scope: fixed(value.asset_scope, SCOPE, "descriptor scope"),
    candidate_sha256: fixed(value.candidate_sha256, input.candidate_sha256, "descriptor candidate"),
    release_manifest_sha256: fixed(value.release_manifest_sha256, input.products[product].manifest_sha256, "descriptor manifest"),
    input_binding: { file: fixed(value.input_binding.file, INPUT_FILE, "input binding file"),
      sha256: fixed(value.input_binding.sha256, c.digest(inputBytes), "exact input bytes digest") } };
}

/** Encode complete v2 data against exact canonical I bytes and a required product.
 * Structural consistency only; this does not generate or qualify a package. */
function encodeDescriptor(value, inputBytes, product) {
  return encoded(descriptor(value, inputBytes, product), MAX_DESCRIPTOR_BYTES);
}

/** Decode v2 with the same required I bytes/product. Structural consistency only.
 * No authentication, signing, acquisition, eligibility or acceptance is implied. */
function decodeDescriptor(body, inputBytes, product) {
  const value = descriptor(parsed(body, MAX_DESCRIPTOR_BYTES), inputBytes, product);
  if (!body.equals(encoded(value, MAX_DESCRIPTOR_BYTES))) fail("noncanonical descriptor bytes");
  return value;
}

const agree = (a, b, label) => { if (!equal(a, b)) throw new Error(`C1 provenance ${label} mismatch`); };
function artifact(value) {
  fields(value, ["run_id", "run_attempt", "artifact_id", "artifact_sha256"], "input artifact");
  return { run_id: positive(value.run_id, Number.MAX_SAFE_INTEGER, "input run"),
    run_attempt: positive(value.run_attempt, 1000, "input attempt"),
    artifact_id: positive(value.artifact_id, Number.MAX_SAFE_INTEGER, "input artifact ID"),
    artifact_sha256: hash(value.artifact_sha256, "input artifact") };
}
function operationOptions(value, reading) {
  fields(value, ["input", "selected", "workflow_sha", "scratch", ...(reading ? ["artifact"] : [])], "provenance options");
  const input = decodeInputs(value.input), body = Buffer.from(value.input);
  fields(value.selected, ["tag", "ref", "source", "versions"], "selected inputs");
  fields(value.selected.versions, c.PRODUCTS, "selected versions");
  const selected = { tag: input.products.agentplugins.tag, ref: `refs/tags/${input.products.agentplugins.tag}`,
    source: input.identity.commit, versions: input.identity.versions };
  agree(value.selected, selected, "selected source/ref/versions");
  fixed(value.workflow_sha, input.identity.commit, "integrated workflow source F");
  if (Object.values(input.identity.versions).some(v => v.length > 32)) fail("bounded provider versions required");
  if (input.producer.run_id === input.preparation.artifact.run_id) fail("separate provenance and preparation runs required");
  const pin = reading ? artifact(value.artifact) : null;
  if (reading) {
    agree([pin.run_id, pin.run_attempt], [input.producer.run_id, input.producer.run_attempt], "I artifact producer attempt");
    if (pin.artifact_id === input.preparation.artifact.artifact_id) fail("I and preparation artifacts must differ");
  }
  c.safeDirectory(value.scratch);
  return { body, input, selected, workflow_sha: value.workflow_sha, scratch: value.scratch, artifact: pin };
}
function projectionPins(input) {
  return { identity: input.identity, candidate_sha256: input.candidate_sha256, pair_marker_sha256: input.pair_marker_sha256,
    products: Object.fromEntries(c.PRODUCTS.map(p => [p, {
      manifest_sha256: input.products[p].manifest_sha256, checksums_sha256: input.products[p].checksums_sha256 }])) };
}
function preparationSnapshot(root, body) {
  const input = decodeInputs(body);
  const verified = require("./authoring-release").verifyProjectedPair(root, projectionPins(input));
  agree(verified.subjects.length, 18, "original subject count");
  for (const p of c.PRODUCTS) agree(verified.manifest.products[p].assets, input.products[p].assets, "outer/inner pins");
  const prepared = require("./authoring-promotion").readInputPreparation(root, body);
  return { subjects: verified.subjects, preparation: prepared.preparation,
    metadata_sha256: c.digest(c.readFile(path.join(root, "candidate-identity.json"), MAX_INPUT_BYTES)) };
}

/** Structural enumeration of exactly 18 original subjects plus exact I bytes.
 * Keep rows, including both manifest/checksum basenames; this is NOT admission. */
function inputSubjects(root, inputBytes) {
  decodeInputs(inputBytes);
  const snapshot = preparationSnapshot(root, inputBytes);
  const file = path.join(root, INPUT_FILE);
  agree(c.readFile(file, MAX_INPUT_BYTES), inputBytes, "retained I bytes");
  return [...snapshot.subjects, { file, sha256: c.digest(inputBytes) }];
}

/** Acquire the exact completed preparation and prepare unsigned I for the
 * future protected provenance job. No signatures, OIDC or upload are performed.
 * Its returned nineteen rows are signing candidates, never authenticated proof. */
function produceInputs(value) {
  const o = operationOptions(value, false), p = require("./authoring-promotion");
  const pin = o.input.preparation.artifact;
  p.checkInputTags(o.body, o.scratch);
  const before = p.inspectArtifact(pin, WORKFLOW, o.input.identity.commit, o.scratch);
  const prepared = p.acquireInputPreparation(o.body, o.scratch);
  const snapshot = preparationSnapshot(prepared.root, o.body);
  p.checkInputTags(o.body, o.scratch);
  agree(p.inspectArtifact(pin, WORKFLOW, o.input.identity.commit, o.scratch), before, "preparation provider changed");
  agree(preparationSnapshot(prepared.root, o.body), snapshot, "preparation changed before I completion");
  agree(operationOptions(value, false), o, "caller inputs changed before I completion");
  const file = path.join(prepared.root, INPUT_FILE);
  fs.writeFileSync(file, o.body, { flag: "wx", mode: 0o444 });
  const subjects = inputSubjects(prepared.root, o.body);
  agree(preparationSnapshot(prepared.root, o.body), snapshot, "preparation changed at I completion");
  agree(operationOptions(value, false), o, "caller inputs changed at I completion");
  return { root: prepared.root, input: o.input, subjects };
}

/** Source wiring for a completed provenance artifact. Positive integrated use
 * is UNAVAILABLE: the existing checked reader supports native/preparation only.
 * A separately accepted 21-entry input-provenance interface is required. Never
 * substitute preparation kind, append files, or fall back to another extractor. */
function readInputs(value) {
  const o = operationOptions(value, true), p = require("./authoring-promotion");
  p.checkInputTags(o.body, o.scratch);
  const before = p.inspectArtifact(o.artifact, WORKFLOW, o.input.identity.commit, o.scratch);
  const work = fs.mkdtempSync(path.join(o.scratch, "input-provenance-"));
  const file = p.acquireArtifact(o.artifact, WORKFLOW, o.input.identity.commit, work);
  const files = ["preparation-run.json", "candidate-identity.json", "candidate/candidate.json", "pair-prepared.json",
    ...c.PRODUCTS.flatMap(p => [...c.TARGETS.map(t => `${p}/${o.input.products[p].assets[t].file}`),
      `${p}/release-manifest.json`, `${p}/checksums.txt`]), INPUT_FILE];
  const root = p.extractArtifact(file, o.artifact, "input-provenance", files, path.join(work, "frozen"), work);
  agree(c.readFile(path.join(root, INPUT_FILE), MAX_INPUT_BYTES), o.body, "independently pinned I bytes");
  const snapshot = preparationSnapshot(root, o.body);
  const prepPin = o.input.preparation.artifact;
  const prepBefore = p.inspectArtifact(prepPin, WORKFLOW, o.input.identity.commit, o.scratch);
  const prepared = p.acquireInputPreparation(o.body, o.scratch);
  const original = preparationSnapshot(prepared.root, o.body);
  const relative = (s, base) => ({ ...s,
    subjects: s.subjects.map(row => ({ file: path.relative(base, row.file), sha256: row.sha256 })) });
  agree(relative(snapshot, root), relative(original, prepared.root), "original preparation custody");
  const subjects = inputSubjects(root, o.body);
  const multiset = subjects.map(s => ({ name: path.basename(s.file), digest: { sha256: s.sha256 } }));
  for (const subject of subjects) p.verifySubject(subject.file, {
    name: path.basename(subject.file), sha256: subject.sha256, source: o.input.identity.commit,
    workflow_sha: o.workflow_sha, ref: o.selected.ref, run_id: o.input.producer.run_id,
    run_attempt: o.input.producer.run_attempt, subjects: multiset
  }, o.scratch);
  p.checkInputTags(o.body, o.scratch);
  agree(p.inspectArtifact(o.artifact, WORKFLOW, o.input.identity.commit, o.scratch), before, "I provider changed");
  agree(p.inspectArtifact(prepPin, WORKFLOW, o.input.identity.commit, o.scratch), prepBefore, "preparation provider changed");
  agree(preparationSnapshot(prepared.root, o.body), original, "original preparation changed");
  agree(preparationSnapshot(root, o.body), snapshot, "provenance preparation changed");
  agree(inputSubjects(root, o.body), subjects, "provenance subjects changed");
  agree(operationOptions(value, true), o, "caller inputs changed during authentication");
  return { root, input: o.input, subjects };
}

module.exports = Object.freeze({ encodeInputs, decodeInputs, encodeDescriptor, decodeDescriptor,
  produceInputs, readInputs, inputSubjects,
  INPUT_SCHEMA, DESCRIPTOR_SCHEMA, INPUT_FILE, MODE, SCOPE, WORKFLOW, PACKAGES,
  MAX_INPUT_BYTES, MAX_DESCRIPTOR_BYTES, MAX_NATIVE_BYTES });
