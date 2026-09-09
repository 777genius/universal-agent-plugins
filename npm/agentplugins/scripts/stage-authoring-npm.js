#!/usr/bin/env node
"use strict";

// CLI preparation only, plus pure C1 S/final-format generation contracts.
// Authenticated staging is still absent. This entrypoint never qualifies packs.
const fs = require("node:fs");
const path = require("node:path");
const c = require("./dual-authoring-candidate");
const adapter = require("./authoring-release");
const packing = require("./stage-dual-authoring-npm");
const runtime = require("../lib/public-authoring");
const inputs = require("./authoring-native-inputs");
const crypto = require("node:crypto");
const { TextDecoder } = require("node:util");
const PREFIX = "npm/agentplugins/";
const COMMON = Object.freeze(["lib/verifier.js", "lib/public-authoring.js", "scripts/dual-authoring-candidate.js"]);
const ownFiles = product => ["LICENSE", "README.md", "package.json", `bin/${product}.js`, "lib/platform.js",
  product === "agentplugins" ? "lib/bootstrap.js" : "lib/install.js"];
const ALLOWLIST = Object.freeze([...new Set([...COMMON.map(n => PREFIX + n),
  ...c.PRODUCTS.flatMap(p => ownFiles(p).map(n => `npm/${p}/${n}`)),
  ...["stage-authoring-npm.js", "stage-dual-authoring-npm.js", "stage-dual-authoring-candidate.js",
    "authoring-release.js"].map(n => PREFIX + "scripts/" + n)])]);
// Separate future stage provenance inventory. Never extend the legacy
// preparation wrapper_blobs receipt or ship these producer helpers in a pack.
const STAGE_ALLOWLIST = Object.freeze([...ALLOWLIST,
  ...["authoring-native-inputs.js", "authoring-promotion.js", "authoring-native-qualification.js",
    "platform-proof.js", "npm-public-contract.js"].map(n => PREFIX + "scripts/" + n),
  "scripts/read-authoring-evidence-zip.py", ".github/workflows/agentplugins-release.yml",
  ".github/workflows/agentplugins-npm-publish.yml"]);
const STAGE_SCHEMA = "dual-authoring-public-stage/v1";
const STAGE_WORKFLOW = ".github/workflows/agentplugins-npm-publish.yml";
const MAX_STAGE_BYTES = 1024 * 1024;
const ASSERTIONS = Object.freeze(["authenticated_native_inputs", "exact_preparation_binding", "exact_source_blobs",
  "exact_generated_closures", "exact_pack_entries_modes_bytes", "both_products_complete", "shared_runtime_bytes_equal",
  "pack_once", "inputs_unchanged", "no_native_execution", "no_publication"]);
const write = (file, bytes, mode = 0o644) => {
  fs.mkdirSync(path.dirname(file), { recursive: true, mode: 0o700 });
  fs.writeFileSync(file, bytes, { flag: "wx", mode });
  fs.chmodSync(file, mode);
};

function packageFiles(product, source, manifestBytes, candidate) {
  if (!c.PRODUCTS.includes(product)) throw new Error("unknown fixed public package");
  const base = JSON.parse(source[`npm/${product}/package.json`].bytes);
  const files = Object.fromEntries(ownFiles(product).filter(n => n !== "package.json").map(n => [n, source[`npm/${product}/${n}`].bytes]));
  for (const name of COMMON) files[name] = source[PREFIX + name].bytes;
  for (const dir of ["bin", "lib", "scripts"]) files[`${dir}/package.json`] = c.encode({ type: "commonjs" });
  files["release-manifest.json"] = manifestBytes;
  files["public-release.json"] = c.encode({ schema: runtime.SCHEMA, product, npm_package: runtime.PACKAGES[product],
    identity: candidate.identity, authoring_mode: runtime.MODE, asset_scope: runtime.SCOPE,
    candidate_sha256: candidate.manifestDigest, release_manifest_sha256: c.digest(manifestBytes), qualification: null });
  // Preserve scripts and engines exactly, including kit postinstall. These are
  // private preparation packs of the real public launcher, never private shims.
  files["package.json"] = c.encode({ ...base, version: candidate.identity.versions[product], private: true,
    files: [...Object.keys(files), "package.json"].sort() });
  return files;
}

// These fixed pure contracts establish byte/shape consistency ONLY. In
// particular, parsing S's assertions cannot establish that any assertion is true.
// No authenticated producer/reader, custody adapter or staging CLI exists here.
function stageFields(value, names, label) {
  if (!value || typeof value !== "object" || ![Object.prototype, null].includes(Object.getPrototypeOf(value))) {
    throw new Error(`${label}: ordinary data object required`);
  }
  const own = Reflect.ownKeys(value);
  if (own.length !== names.length || own.some(k => typeof k !== "string" || !names.includes(k))) {
    throw new Error(`${label}: unexpected or missing fields`);
  }
  for (const key of own) {
    const d = Object.getOwnPropertyDescriptor(value, key);
    if (!d.enumerable || !("value" in d)) throw new Error(`${label}: enumerable data fields required`);
  }
  c.keys(value, names, label);
}
function stageEqual(value, expected, label) {
  if (value !== expected) throw new Error(`${label}: stage binding mismatch`);
  return value;
}
function stageHash(value, length = 64) {
  if (typeof value !== "string" || value.length !== length || !/^[0-9a-f]+$/.test(value) || /^0+$/.test(value)) {
    throw new Error("stage nonzero lowercase digest required");
  }
  return value;
}
function stageInteger(value, maximum = Number.MAX_SAFE_INTEGER) {
  if (!Number.isSafeInteger(value) || value <= 0 || value > maximum) throw new Error("stage positive bounded integer required");
  return value;
}
function stageBytes(value, maximum) {
  if (!Buffer.isBuffer(value) || value.length === 0 || value.length > maximum) throw new Error("stage nonempty bounded Buffer required");
  return value;
}
const stageClosure = product => [...ownFiles(product), ...COMMON, "bin/package.json", "lib/package.json",
  "scripts/package.json", "public-release.json", "release-manifest.json", inputs.INPUT_FILE].sort();

function stageDescriptors(input, inputBytes) {
  // Preflight BOTH 64 KiB limits, even when I itself fits its 1 MiB contract.
  // Stable versions keep the accepted I policy; no artificial version ceiling.
  return Object.fromEntries(c.PRODUCTS.map(product => [product, inputs.encodeDescriptor({
    schema: inputs.DESCRIPTOR_SCHEMA, product, npm_package: inputs.PACKAGES[product], identity: input.identity,
    authoring_mode: input.authoring_mode, asset_scope: input.asset_scope, candidate_sha256: input.candidate_sha256,
    release_manifest_sha256: input.products[product].manifest_sha256,
    input_binding: { file: inputs.INPUT_FILE, sha256: c.digest(inputBytes) }
  }, inputBytes, product)]));
}

function stageBlob(value, withBytes) {
  stageFields(value, ["git_blob", "mode", "sha256", ...(withBytes ? ["bytes"] : [])], "stage blob");
  const pin = { git_blob: stageHash(value.git_blob, 40), mode: value.mode, sha256: stageHash(value.sha256) };
  if (!["100644", "100755"].includes(pin.mode)) throw new Error("stage regular Git mode required");
  if (withBytes) {
    const body = stageBytes(value.bytes, inputs.MAX_NATIVE_BYTES);
    stageEqual(c.digest(body), pin.sha256, "source SHA256");
    const blob = crypto.createHash("sha1").update(`blob ${body.length}\0`).update(body).digest("hex");
    stageEqual(blob, pin.git_blob, "source Git blob");
  }
  return pin;
}

/** Pure final-format pair constructor. Supplied blobs and canonical projections
 * are byte contracts, NOT authenticated source/provenance. The integrated C1
 * producer must authenticate I/all eighteen inputs and committed F first, then
 * reuse packPackage once per product and completeRecord after revalidation. */
function pairedPackageFiles(source, manifests, inputBytes) {
  const input = inputs.decodeInputs(inputBytes);
  const descriptors = stageDescriptors(input, inputBytes);
  stageFields(source, STAGE_ALLOWLIST, "stage source closure");
  for (const name of STAGE_ALLOWLIST) stageBlob(source[name], true);
  stageFields(manifests, c.PRODUCTS, "paired manifest bytes");
  for (const product of c.PRODUCTS) {
    const body = stageBytes(manifests[product], inputs.MAX_INPUT_BYTES), id = input.identity;
    // Exact existing authoring-release productManifest encoding, using the
    // already validated I assets. No candidate rebuild, archive or second reader.
    const expected = c.encode({ schema_version: 3, status: "CANDIDATE", product, repository: id.repository,
      tag: input.products[product].tag, version: id.versions[product], commit: id.commit, engine_revision: id.engine_revision,
      versions: id.versions, candidate_sha256: input.candidate_sha256, authoring_mode: input.authoring_mode,
      asset_scope: input.asset_scope, assets: input.products[product].assets,
      release_eligible: false, platform_acceptance: false, attested: false });
    if (!body.equals(expected)) throw new Error("stage projection bytes differ from I");
    stageEqual(c.digest(body), input.products[product].manifest_sha256, "selected manifest hash");
    const checksums = Buffer.from([...Object.values(input.products[product].assets).map(a => `${a.sha256}  ${a.file}`),
      `${c.digest(body)}  release-manifest.json`].join("\n") + "\n");
    stageEqual(c.digest(checksums), input.products[product].checksums_sha256, "projection checksums hash");
    const baseBytes = source[`npm/${product}/package.json`].bytes;
    const base = JSON.parse(new TextDecoder("utf-8", { fatal: true, ignoreBOM: true }).decode(baseBytes));
    if (!base || Array.isArray(base) || typeof base !== "object") throw new Error("source package object required");
    stageEqual(base.name, inputs.PACKAGES[product], "source package name");
    c.keys(base.bin, [product], "source bin");
    stageEqual(base.bin[product], `bin/${product}.js`, "source bin");
    c.keys(base.engines, ["node"], "source engines");
    stageEqual(base.engines.node, product === "agentplugins" ? ">=22" : ">=18", "source Node support");
    const scripts = product === "agentplugins" ? { test: "node --test" } : { postinstall: "node ./lib/install.js" };
    c.keys(base.scripts, Object.keys(scripts), "source scripts");
    for (const name of Object.keys(scripts)) stageEqual(base.scripts[name], scripts[name], "source script");
  }
  const pair = {};
  for (const product of c.PRODUCTS) {
    const files = packageFiles(product, source, manifests[product], {
      identity: input.identity, manifestDigest: input.candidate_sha256 });
    files["public-release.json"] = descriptors[product];
    files[inputs.INPUT_FILE] = inputBytes;
    files["package.json"] = c.encode({ ...JSON.parse(files["package.json"]), private: false, files: stageClosure(product) });
    // Own the returned buffers, including each product's I and shared runtime.
    pair[product] = Object.fromEntries(Object.entries(files).map(([name, body]) => [name, Buffer.from(body)]));
  }
  return pair;
}

function stageRecord(value, inputBytes) {
  const input = inputs.decodeInputs(inputBytes);
  stageFields(value, ["schema", "identity", "authoring_mode", "asset_scope", "candidate_sha256", "pair_marker_sha256",
    "projection_pins", "native_inputs", "wrapper_blobs", "generated", "packs", "tools", "producer", "assertions"], "stage");
  // Reuse the accepted descriptor identity validation (including closed nested
  // fields) instead of relaxing candidate identity or introducing a new policy.
  const descriptors = stageDescriptors({ ...input, identity: value.identity }, inputBytes);
  stageEqual(value.schema, STAGE_SCHEMA, "schema");
  stageEqual(value.authoring_mode, input.authoring_mode, "mode");
  stageEqual(value.asset_scope, input.asset_scope, "scope");
  stageEqual(value.candidate_sha256, input.candidate_sha256, "candidate");
  stageEqual(value.pair_marker_sha256, input.pair_marker_sha256, "pair marker");
  stageFields(value.projection_pins, c.PRODUCTS, "projection pins");
  const projection_pins = {};
  for (const product of c.PRODUCTS) {
    const pin = value.projection_pins[product];
    stageFields(pin, ["manifest_sha256", "checksums_sha256"], "projection pin");
    projection_pins[product] = {
      manifest_sha256: stageEqual(pin.manifest_sha256, input.products[product].manifest_sha256, "manifest"),
      checksums_sha256: stageEqual(pin.checksums_sha256, input.products[product].checksums_sha256, "checksums") };
  }
  const native = value.native_inputs;
  stageFields(native, ["sha256", "artifact"], "native inputs");
  stageFields(native.artifact, ["run_id", "run_attempt", "artifact_id", "artifact_sha256"], "input artifact");
  const a = native.artifact;
  const native_inputs = { sha256: stageEqual(native.sha256, c.digest(inputBytes), "I bytes"), artifact: {
    run_id: stageEqual(a.run_id, input.producer.run_id, "I producer run"),
    run_attempt: stageEqual(a.run_attempt, input.producer.run_attempt, "I producer attempt"),
    artifact_id: stageInteger(a.artifact_id), artifact_sha256: stageHash(a.artifact_sha256) } };
  stageFields(value.wrapper_blobs, STAGE_ALLOWLIST, "stage wrapper blobs");
  const wrapper_blobs = Object.fromEntries(STAGE_ALLOWLIST.map(n => [n, stageBlob(value.wrapper_blobs[n], false)]));
  stageFields(value.generated, c.PRODUCTS, "generated pair");
  const generated = {};
  for (const product of c.PRODUCTS) {
    const g = value.generated[product];
    stageFields(g, stageClosure(product), "generated closure");
    generated[product] = Object.fromEntries(stageClosure(product).map(n => [n, stageHash(g[n])]));
    for (const name of ownFiles(product).filter(n => n !== "package.json")) {
      stageEqual(g[name], wrapper_blobs[`npm/${product}/${name}`].sha256, "generated source file");
    }
    for (const name of COMMON) stageEqual(g[name], wrapper_blobs[PREFIX + name].sha256, "shared runtime");
    for (const dir of ["bin", "lib", "scripts"]) {
      stageEqual(g[`${dir}/package.json`], c.digest(c.encode({ type: "commonjs" })), "CommonJS scope");
    }
    stageEqual(g[inputs.INPUT_FILE], native_inputs.sha256, "generated I");
    stageEqual(g["release-manifest.json"], projection_pins[product].manifest_sha256, "generated manifest");
    stageEqual(g["public-release.json"], c.digest(descriptors[product]), "generated descriptor");
  }
  stageFields(value.packs, c.PRODUCTS, "stage packs");
  const packs = {};
  for (const product of c.PRODUCTS) {
    const p = value.packs[product];
    stageFields(p, ["file", "sha256", "size", "integrity", "shasum"], "pack");
    if (typeof p.integrity !== "string" || !/^sha512-[A-Za-z0-9+/]{86}==$/.test(p.integrity) ||
        p.integrity.length !== 95 || Buffer.from(p.integrity.slice(7), "base64").toString("base64") !== p.integrity.slice(7)) {
      throw new Error("canonical SHA512 SRI required");
    }
    packs[product] = { file: stageEqual(p.file, `${inputs.PACKAGES[product]}-${input.identity.versions[product]}.tgz`, "tarball name"),
      sha256: stageHash(p.sha256), size: stageInteger(p.size, inputs.MAX_NATIVE_BYTES),
      integrity: p.integrity, shasum: stageHash(p.shasum, 40) };
  }
  stageFields(value.tools, ["node", "npm", "git", "tar", "gh"], "stage tools");
  const tools = {};
  for (const name of ["node", "npm", "git", "tar", "gh"]) {
    const t = value.tools[name];
    stageFields(t, ["version", "sha256"], "tool");
    if (typeof t.version !== "string" || !t.version.length || t.version.length > MAX_STAGE_BYTES ||
        t.version.trim() !== t.version || /[\x00-\x1f\x7f]/.test(t.version)) throw new Error("stage tool version string required");
    tools[name] = { version: t.version, sha256: stageHash(t.sha256) };
  }
  const p = value.producer;
  stageFields(p, ["workflow", "source", "ref", "run_id", "run_attempt"], "stage producer");
  const producer = { workflow: stageEqual(p.workflow, STAGE_WORKFLOW, "stage workflow"),
    source: stageEqual(p.source, input.identity.commit, "stage source"),
    ref: stageEqual(p.ref, `refs/tags/${input.products.agentplugins.tag}`, "stage ref"),
    run_id: stageInteger(p.run_id), run_attempt: stageInteger(p.run_attempt, 1000) };
  stageFields(value.assertions, ASSERTIONS, "stage assertions");
  const assertions = Object.fromEntries(ASSERTIONS.map(n => [n, stageEqual(value.assertions[n], true, "assertion syntax")]));
  return { schema: STAGE_SCHEMA, identity: input.identity, authoring_mode: input.authoring_mode, asset_scope: input.asset_scope,
    candidate_sha256: input.candidate_sha256, pair_marker_sha256: input.pair_marker_sha256, projection_pins,
    native_inputs, wrapper_blobs, generated, packs, tools, producer, assertions };
}

/** Canonical S codec only. Does not attest/assert truth or authorize effects. */
function encodeStage(value, inputBytes) {
  return stageBytes(c.encode(stageRecord(value, inputBytes)), MAX_STAGE_BYTES);
}

/** Canonical S codec only, NOT authenticated readStage. Retained tarballs,
 * source at F, tool provision, signatures and operation evidence are external. */
function decodeStage(body, inputBytes) {
  stageBytes(body, MAX_STAGE_BYTES);
  const text = new TextDecoder("utf-8", { fatal: true, ignoreBOM: true }).decode(body);
  let depth = 0, quoted = false, escaped = false;
  for (const ch of text) {
    if (quoted) {
      if (escaped) escaped = false;
      else if (ch === "\\") escaped = true;
      else if (ch === '"') quoted = false;
    } else if (ch === '"') quoted = true;
    else if (ch === "[") throw new Error("stage arrays are outside the fixed contract");
    else if (ch === "{" && ++depth > 4) throw new Error("stage object depth limit");
    else if (ch === "}") depth--;
  }
  const value = stageRecord(JSON.parse(text), inputBytes);
  if (!body.equals(stageBytes(c.encode(value), MAX_STAGE_BYTES))) throw new Error("noncanonical stage bytes");
  return value;
}

function pinFile(file, expected, maximum = 1024 * 1024) {
  if (typeof expected !== "string" || !/^[0-9a-f]{64}$/.test(expected)) throw new Error("independent preparation digest required");
  const bytes = c.readFile(file, maximum);
  if (c.digest(bytes) !== expected) throw new Error("preparation input digest mismatch");
  return bytes;
}

function pins(options) {
  const { candidate, projectionPins, pairMarkerDigest } = options;
  c.keys(projectionPins, c.PRODUCTS, "projection pins");
  const bodies = {};
  for (const p of c.PRODUCTS) {
    c.keys(projectionPins[p], ["manifest_sha256", "checksums_sha256"], "projection digests");
    bodies[p] = pinFile(path.join(candidate.outputs[p], "release-manifest.json"), projectionPins[p].manifest_sha256);
    pinFile(path.join(candidate.outputs[p], "checksums.txt"), projectionPins[p].checksums_sha256, 64 * 1024);
  }
  pinFile(candidate.pairMarker, pairMarkerDigest);
  pinFile(path.join(candidate.root, "candidate.json"), candidate.manifestDigest);
  return bodies;
}

function prepare(options) {
  c.keys(options, ["candidate", "repo", "node", "npm", "output", "projectionPins", "pairMarkerDigest"], "public preparation options");
  const candidate = options.candidate;
  if (candidate.authoringMode !== runtime.MODE || candidate.assetScope !== runtime.SCOPE) throw new Error("public preparation requires full release pair");
  c.identity(candidate.identity);
  const roots = [options.repo, candidate.root, candidate.workParent, ...Object.values(candidate.outputs), path.dirname(candidate.pairMarker)];
  roots.forEach(c.safeDirectory);
  for (const key of ["node", "npm"]) {
    if (!path.isAbsolute(options[key])) throw new Error("absolute trusted tool required");
    c.readFile(options[key]);
  }
  c.outputPlacement(options.output, [...roots, path.dirname(options.node), path.dirname(options.npm)]);
  const before = pins(options);
  // Requires real Go build-info verification of the pinned frozen candidate and
  // exact projections, without executing assets or claiming platform acceptance.
  adapter.verifyAuthoringRelease(candidate);
  const context = packing.npmContext(candidate.workParent);
  context.env.PATH = "/usr/local/bin:/usr/bin:/bin"; // mandatory guarded tools
  const source = packing.blobs(options.repo, candidate.identity.commit, context.env, "public");
  fs.mkdirSync(options.output, { mode: 0o700 });
  const snapshot = path.join(options.output, "projection-snapshot");
  fs.mkdirSync(snapshot, { mode: 0o700 });
  const outputs = {};
  for (const p of c.PRODUCTS) {
    const dest = outputs[p] = path.join(snapshot, p);
    fs.mkdirSync(dest, { mode: 0o700 });
    for (const name of fs.readdirSync(candidate.outputs[p])) write(path.join(dest, name), c.readFile(path.join(candidate.outputs[p], name)), 0o444);
  }
  const pairMarker = path.join(snapshot, "pair.json");
  write(pairMarker, pinFile(candidate.pairMarker, options.pairMarkerDigest), 0o444);
  const copied = { ...candidate, outputs, pairMarker };
  adapter.verifyAuthoringRelease(copied);
  pins({ ...options, candidate: copied });
  const packs = {}, generated = {}, closures = {};
  for (const p of c.PRODUCTS) {
    const files = closures[p] = packageFiles(p, source, before[p], candidate);
    const root = path.join(options.output, p);
    fs.mkdirSync(root, { mode: 0o700 });
    for (const [name, bytes] of Object.entries(files)) write(path.join(root, name), bytes, name === `bin/${p}.js` ? 0o755 : 0o644);
    generated[p] = Object.fromEntries(Object.entries(files).map(([n, b]) => [n, c.digest(b)]));
    packs[p] = packing.packPackage(p, files, root, { ...options, identity: candidate.identity }, context);
  }
  for (const name of COMMON) if (!closures.agentplugins[name].equals(closures["plugin-kit-ai"][name])) throw new Error("public shared closure differs");
  const after = pins(options);
  for (const p of c.PRODUCTS) {
    if (!before[p].equals(after[p])) throw new Error("projection changed during packing");
    pinFile(path.join(options.output, packs[p].file), packs[p].sha256, 128 * 1024 * 1024);
  }
  adapter.verifyAuthoringRelease(copied);
  const record = { schema: "dual-authoring-public-preparation/v1", identity: candidate.identity,
    candidate_sha256: candidate.manifestDigest, projection_pins: options.projectionPins, pair_marker_sha256: options.pairMarkerDigest,
    wrapper_blobs: Object.fromEntries(Object.entries(source).map(([n, { bytes, ...pin }]) => [n, pin])),
    generated, packs, tools: Object.fromEntries(["node", "npm"].map(k => [k, { path: options[k], sha256: c.digest(c.readFile(options[k])) }])),
    qualification: null, release_eligible: false, platform_acceptance: false, attested: false };
  packing.blobs(options.repo, candidate.identity.commit, context.env, "public");
  packing.completeRecord(options.output, record);
  return record;
}
// Public blob verification re-enters this module while the CLI is preparing.
module.exports = { prepare, packageFiles, ALLOWLIST, COMMON,
  encodeStage, decodeStage, pairedPackageFiles, STAGE_ALLOWLIST };

if (require.main === module) {
  try {
    if (process.argv.length !== 4 || process.argv[2] !== "--prepare" || !path.isAbsolute(process.argv[3])) throw new Error("usage: stage-authoring-npm.js --prepare <absolute-options.json>");
    process.stdout.write(c.encode(prepare(JSON.parse(c.readFile(process.argv[3], 1024 * 1024)))));
  } catch (e) { process.stderr.write(`public npm preparation: ${e.message}\n`); process.exitCode = 1; }
}
