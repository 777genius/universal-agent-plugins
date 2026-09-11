#!/usr/bin/env node
"use strict";

// Preparation and C1 stage source interfaces. Neither qualifies packs.
// Positive stage execution requires separately accepted artifact/workflow tools.
const fs = require("node:fs");
const path = require("node:path");
const c = require("./dual-authoring-candidate");
const adapter = require("./authoring-release");
const packing = require("./stage-dual-authoring-npm");
const runtime = require("../lib/public-authoring");
const inputs = require("./authoring-native-inputs");
const { projectionBytes } = require("../lib/public-authoring-contract");
const crypto = require("node:crypto");
const { TextDecoder } = require("node:util");
const PREFIX = "npm/agentplugins/";
const COMMON = Object.freeze(["lib/verifier.js", "lib/public-authoring.js", "scripts/dual-authoring-candidate.js"]);
const STAGE_COMMON = Object.freeze([...COMMON, "lib/public-authoring-contract.js", "lib/public-authoring-input.js"]);
const ownFiles = product => ["LICENSE", "README.md", "package.json", `bin/${product}.js`, "lib/platform.js",
  product === "agentplugins" ? "lib/bootstrap.js" : "lib/install.js"];
const ALLOWLIST = Object.freeze([...new Set([...COMMON.map(n => PREFIX + n),
  ...c.PRODUCTS.flatMap(p => ownFiles(p).map(n => `npm/${p}/${n}`)),
  ...["stage-authoring-npm.js", "stage-dual-authoring-npm.js", "stage-dual-authoring-candidate.js",
    "authoring-release.js"].map(n => PREFIX + "scripts/" + n)])]);
// Separate future stage provenance inventory. Never extend the legacy
// preparation wrapper_blobs receipt or ship these producer helpers in a pack.
const STAGE_ALLOWLIST = Object.freeze([...ALLOWLIST,
  ...STAGE_COMMON.filter(n => !COMMON.includes(n)).map(n => PREFIX + n),
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
// Authentication belongs to the separate integrated operations below.
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
const stageClosure = product => [...ownFiles(product), ...STAGE_COMMON, "bin/package.json", "lib/package.json",
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
  for (const name of STAGE_COMMON.filter(n => !COMMON.includes(n))) {
    stageEqual(source[PREFIX + name].mode, "100644", "v2 helper mode");
  }
  stageFields(manifests, c.PRODUCTS, "paired manifest bytes");
  for (const product of c.PRODUCTS) {
    const body = stageBytes(manifests[product], inputs.MAX_INPUT_BYTES);
    const { manifest: expected, checksums } = projectionBytes(input, product);
    if (!body.equals(expected)) throw new Error("stage projection bytes differ from I");
    stageEqual(c.digest(body), input.products[product].manifest_sha256, "selected manifest hash");
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
    for (const name of STAGE_COMMON) files[name] = source[PREFIX + name].bytes;
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
    for (const name of STAGE_COMMON) stageEqual(g[name], wrapper_blobs[PREFIX + name].sha256, "shared runtime");
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

// Integrated source operations below use only fixed external boundaries. The
// checked artifact reader's input-provenance/public-stage kinds and protected
// workflows remain separately required before authentic positive execution.
const promotion = require("./authoring-promotion");
const cp = require("node:child_process");
const { isDeepStrictEqual: stageSame } = require("node:util");
const agreeStage = (a, b, label) => {
  if (!stageSame(a, b)) throw new Error(`C1 stage ${label} changed or mismatched`);
};
function stageLocator(value) {
  stageFields(value, ["run_id", "run_attempt", "artifact_id", "artifact_sha256"], "stage artifact locator");
  return { run_id: stageInteger(value.run_id), run_attempt: stageInteger(value.run_attempt, 1000),
    artifact_id: stageInteger(value.artifact_id), artifact_sha256: stageHash(value.artifact_sha256) };
}
function stageInvocation(value, input) {
  stageFields(value, ["workflow", "source", "ref", "run_id", "run_attempt"], "stage invocation");
  const result = { workflow: STAGE_WORKFLOW, source: input.identity.commit,
    ref: `refs/tags/${input.products.agentplugins.tag}`, run_id: stageInteger(value.run_id),
    run_attempt: stageInteger(value.run_attempt, 1000) };
  agreeStage(value, result, "producer source/ref/workflow");
  if ([input.producer.run_id, input.preparation.artifact.run_id].includes(result.run_id)) {
    throw new Error("separate stage and input/preparation runs required");
  }
  return result;
}
function stageCaller(producer) {
  const expected = { GITHUB_ACTIONS: "true", GITHUB_REPOSITORY: c.REPOSITORY, GITHUB_SHA: producer.source,
    GITHUB_REF: producer.ref, GITHUB_RUN_ID: String(producer.run_id), GITHUB_RUN_ATTEMPT: String(producer.run_attempt),
    GITHUB_WORKFLOW_SHA: producer.source, GITHUB_WORKFLOW_REF: `${c.REPOSITORY}/${STAGE_WORKFLOW}@${producer.ref}` };
  agreeStage(Object.fromEntries(Object.keys(expected).map(k => [k, process.env[k]])), expected, "workflow caller");
  return expected;
}
function stageOptions(value, reading) {
  stageFields(value, ["input", "selected", "workflow_sha", "artifact", "repo", "workParent", "node", "npm",
    ...(reading ? ["stage_sha256"] : ["producer", "output"])], "stage operation options");
  const body = Buffer.from(stageBytes(value.input, inputs.MAX_INPUT_BYTES)), input = inputs.decodeInputs(body);
  stageDescriptors(input, body); // both bounded descriptors before any effects
  // Match the existing readInputs provider boundary without changing pure I/S
  // or descriptor limits. Provider-incompatible versions fail before effects.
  if (Object.values(input.identity.versions).some(v => v.length > 32)) throw new Error("bounded provider versions required");
  stageFields(value.selected, ["tag", "ref", "source", "versions"], "stage selection");
  stageFields(value.selected.versions, c.PRODUCTS, "stage versions");
  const selected = { tag: input.products.agentplugins.tag, ref: `refs/tags/${input.products.agentplugins.tag}`,
    source: input.identity.commit, versions: input.identity.versions };
  agreeStage(value.selected, selected, "selected identity");
  stageEqual(value.workflow_sha, input.identity.commit, "workflow revision F");
  const artifact = stageLocator(value.artifact);
  if (input.producer.run_id === input.preparation.artifact.run_id) throw new Error("separate provenance run required");
  if (!reading) {
    agreeStage([artifact.run_id, artifact.run_attempt], [input.producer.run_id, input.producer.run_attempt], "input attempt");
    if (artifact.artifact_id === input.preparation.artifact.artifact_id) throw new Error("separate input artifact required");
  }
  if (reading && [input.producer.run_id, input.preparation.artifact.run_id].includes(artifact.run_id)) {
    throw new Error("separate completed stage run required");
  }
  const producer = reading ? null : stageInvocation(value.producer, input);
  const stage_sha256 = reading ? stageHash(value.stage_sha256) : null;
  for (const key of ["repo", "workParent"]) c.safeDirectory(value[key]);
  const executing = path.resolve(__dirname, "../../..");
  for (const root of [value.repo, executing]) {
    if (root === value.workParent || root.startsWith(value.workParent + path.sep) || value.workParent.startsWith(root + path.sep)) {
      throw new Error("stage source/scratch roots overlap");
    }
  }
  for (const key of ["node", "npm"]) {
    if (typeof value[key] !== "string" || !path.isAbsolute(value[key]) || path.resolve(value[key]) !== value[key]) {
      throw new Error("absolute trusted stage tool required");
    }
    c.readFile(value[key]);
  }
  if (!reading) {
    stageCaller(producer);
    // Existence is checked separately at reservation, so caller rechecks work
    // after the output has been reserved without weakening initial placement.
    const output = value.output;
    if (typeof output !== "string" || !path.isAbsolute(output) || path.resolve(output) !== output ||
        !/^[A-Za-z0-9][A-Za-z0-9._-]*$/.test(path.basename(output))) throw new Error("safe stage output required");
    c.safeDirectory(path.dirname(output));
    for (const root of [value.repo, executing, value.workParent, path.dirname(value.node), path.dirname(value.npm)]) {
      if (output === root || output.startsWith(root + path.sep) || root.startsWith(output + path.sep)) throw new Error("stage output overlaps input");
    }
  }
  return { body, input, selected, workflow_sha: value.workflow_sha, artifact, producer, stage_sha256,
    repo: value.repo, workParent: value.workParent, node: value.node, npm: value.npm, output: reading ? null : value.output };
}
function stageTools(o, context) {
  const command = (exe, args) => cp.execFileSync(exe, args, { env: context.env, cwd: context.root,
    timeout: 30000, maxBuffer: MAX_STAGE_BYTES }).toString().trim();
  const paths = { node: o.node, npm: o.npm, git: "/usr/bin/git", tar: "/usr/bin/tar", gh: "/usr/bin/gh" };
  const tools = Object.fromEntries(Object.entries(paths).map(([name, file]) => [name, {
    version: (name === "npm" ? command(o.node, [o.npm, "--version"]) : command(file, ["--version"])).split("\n")[0],
    sha256: c.digest(c.readFile(file)) }]));
  if (!tools.gh.version.startsWith(`gh version ${promotion.GH_VERSION} (`)) throw new Error("fixed stage gh provision required");
  return tools;
}
function toolSnapshot(o) {
  // Invocation-local pins supplement S's portable five-tool fields. Include the
  // running interpreter and the fixed checked-reader interpreter; no extra S keys.
  return Object.fromEntries([...new Set([o.node, o.npm, process.execPath, "/usr/bin/git", "/usr/bin/tar", "/usr/bin/gh",
    fs.realpathSync("/usr/bin/python3")])].map(file => [file, c.digest(c.readFile(file))]));
}
function sourcePins(source) {
  return Object.fromEntries(Object.entries(source).map(([n, { bytes, ...pin }]) => [n, pin]));
}
function inputFiles(input) {
  return ["preparation-run.json", "candidate-identity.json", "candidate/candidate.json", "pair-prepared.json",
    ...c.PRODUCTS.flatMap(p => [...c.TARGETS.map(t => `${p}/${input.products[p].assets[t].file}`),
      `${p}/release-manifest.json`, `${p}/checksums.txt`]), inputs.INPUT_FILE];
}
function stageInputSnapshot(root, body) {
  const input = inputs.decodeInputs(body);
  const subjects = inputs.inputSubjects(root, body); // existing projected + receipt reader
  if (subjects.length !== 19) throw new Error("exact nineteen input subjects required");
  const files = Object.fromEntries(inputFiles(input).map(n => [n, c.readFile(path.join(root, n), inputs.MAX_NATIVE_BYTES)]));
  agreeStage(files[inputs.INPUT_FILE], body, "same retained I bytes");
  return { files, subjects: subjects.map(s => ({ file: path.relative(root, s.file), sha256: s.sha256 })) };
}
function manifestsFrom(snapshot) {
  return Object.fromEntries(c.PRODUCTS.map(p => [p, snapshot.files[`${p}/release-manifest.json`]]));
}
function generatedPins(pair) {
  for (const name of [...STAGE_COMMON, inputs.INPUT_FILE]) agreeStage(pair.agentplugins[name], pair["plugin-kit-ai"][name], "shared generated bytes");
  return Object.fromEntries(c.PRODUCTS.map(p => [p,
    Object.fromEntries(Object.entries(pair[p]).map(([n, b]) => [n, c.digest(b)]))]));
}
function retainedPack(root, product, input) {
  const file = `${inputs.PACKAGES[product]}-${input.identity.versions[product]}.tgz`;
  const bytes = c.readFile(path.join(root, file), inputs.MAX_NATIVE_BYTES);
  return { file, ...c.metadata(bytes), integrity: "sha512-" + crypto.createHash("sha512").update(bytes).digest("base64"),
    shasum: crypto.createHash("sha1").update(bytes).digest("hex") };
}
function stageProviders(o, inputArtifact, cwd) {
  promotion.checkInputTags(o.body, cwd);
  return [promotion.inspectArtifact(inputArtifact, inputs.WORKFLOW, o.input.identity.commit, cwd),
    promotion.inspectArtifact(o.input.preparation.artifact, inputs.WORKFLOW, o.input.identity.commit, cwd)];
}
function stageReadInputs(o, artifact, scratch) {
  return inputs.readInputs({ input: o.body, selected: o.selected, workflow_sha: o.workflow_sha, artifact, scratch });
}
function checkGeneratedRoot(root, files) {
  const walk = (dir, prefix = "") => fs.readdirSync(dir).flatMap(n => {
    const relative = prefix + n, file = path.join(dir, n), st = fs.lstatSync(file);
    if (st.isDirectory() && !st.isSymbolicLink()) return walk(file, relative + "/");
    return [relative];
  });
  agreeStage(walk(root).sort(), Object.keys(files).sort(), "generated file inventory");
  for (const [name, body] of Object.entries(files)) {
    agreeStage(c.readFile(path.join(root, name)), body, "generated file bytes");
    stageEqual(fs.lstatSync(path.join(root, name)).mode & 0o777, /^bin\/[^/]+\.js$/.test(name) ? 0o755 : 0o644, "generated mode");
  }
}

/** Produce unsigned S only after authenticated I and two verified packs. Never
 * signs, publishes, qualifies or launches a native input. Failed work is retained.
 * Protected positive execution is unavailable until the fixed reader/workflows
 * and genuine tool/verifier prerequisites have been independently accepted. */
function stagePrepublication(value) {
  const o = stageOptions(value, false);
  c.outputPlacement(o.output, [o.repo, o.workParent, path.resolve(__dirname, "../../.."), path.dirname(o.node), path.dirname(o.npm)]);
  const context = packing.npmContext(o.workParent);
  context.env.PATH = "/usr/local/bin:/usr/bin:/bin";
  const source = packing.blobs(o.repo, o.input.identity.commit, context.env, "stage");
  const toolPins = toolSnapshot(o), tools = stageTools(o, context), callerArgs = [...process.execArgv];
  agreeStage(promotion.inspectStageCaller(o.selected, o.workflow_sha, context.root), o.producer, "provider stage caller");
  const providers = stageProviders(o, o.artifact, context.root);
  const admitted = stageReadInputs(o, o.artifact, context.root);
  const before = stageInputSnapshot(admitted.root, o.body);
  const snapshot = path.join(context.root, "stage-inputs");
  c.outputPlacement(snapshot, [admitted.root, o.repo]);
  fs.mkdirSync(snapshot, { mode: 0o700 });
  for (const [n, b] of Object.entries(before.files)) write(path.join(snapshot, n), b, 0o444);
  agreeStage(stageInputSnapshot(snapshot, o.body), before, "owned input snapshot");
  const pair = pairedPackageFiles(source, manifestsFrom(before), o.body), generated = generatedPins(pair);
  // Authentication may have changed caller/source/tools; no pack until recheck.
  agreeStage(stageOptions(value, false), o, "caller before packing");
  agreeStage(packing.blobs(o.repo, o.input.identity.commit, context.env, "stage"), source, "source before packing");
  agreeStage(toolSnapshot(o), toolPins, "tools before packing");
  fs.mkdirSync(o.output, { mode: 0o700 });
  process.stderr.write("C1_STAGE " + JSON.stringify({ operation: "start", source: o.producer.source, ref: o.producer.ref,
    run_id: o.producer.run_id, run_attempt: o.producer.run_attempt, input_sha256: c.digest(o.body), input_artifact: o.artifact }) + "\n");
  const packs = {};
  for (const product of c.PRODUCTS) {
    const root = path.join(o.output, product); fs.mkdirSync(root, { mode: 0o700 });
    for (const [n, b] of Object.entries(pair[product])) write(path.join(root, n), b, /^bin\/[^/]+\.js$/.test(n) ? 0o755 : 0o644);
    const packed = packing.packPackage(product, pair[product], root, { node: o.node, npm: o.npm,
      output: o.output, identity: o.input.identity }, context);
    const retained = retainedPack(o.output, product, o.input), { shasum, ...legacy } = retained;
    agreeStage(packed, legacy, "pack return versus retained bytes");
    packs[product] = retained; // actual SHA1, without changing v1 pack return/receipts
    process.stderr.write("C1_STAGE " + JSON.stringify({ operation: "pack", product, pack: retained }) + "\n");
  }
  for (const product of c.PRODUCTS) {
    checkGeneratedRoot(path.join(o.output, product), pair[product]);
    packing.verifyPack(path.join(o.output, packs[product].file), pair[product], path.join(context.root, `retained-${product}`), context.env);
  }
  agreeStage(stageProviders(o, o.artifact, context.root), providers, "input providers");
  agreeStage(stageInputSnapshot(admitted.root, o.body), before, "authenticated inputs after packing");
  agreeStage(stageInputSnapshot(snapshot, o.body), before, "snapshot after packing");
  agreeStage(packing.blobs(o.repo, o.input.identity.commit, context.env, "stage"), source, "source after packing");
  agreeStage(toolSnapshot(o), toolPins, "tools after packing");
  agreeStage(process.execArgv, callerArgs, "caller interpreter arguments");
  agreeStage(stageOptions(value, false), o, "caller before completion");
  agreeStage(promotion.inspectStageCaller(o.selected, o.workflow_sha, context.root), o.producer, "provider stage completion");
  for (const product of c.PRODUCTS) {
    checkGeneratedRoot(path.join(o.output, product), pair[product]);
    agreeStage(retainedPack(o.output, product, o.input), packs[product], "retained pair before completion");
  }
  const record = { schema: STAGE_SCHEMA, identity: o.input.identity, authoring_mode: o.input.authoring_mode,
    asset_scope: o.input.asset_scope, candidate_sha256: o.input.candidate_sha256, pair_marker_sha256: o.input.pair_marker_sha256,
    projection_pins: Object.fromEntries(c.PRODUCTS.map(p => [p, { manifest_sha256: o.input.products[p].manifest_sha256,
      checksums_sha256: o.input.products[p].checksums_sha256 }])),
    native_inputs: { sha256: c.digest(o.body), artifact: o.artifact }, wrapper_blobs: sourcePins(source), generated, packs, tools,
    producer: o.producer, assertions: Object.fromEntries(ASSERTIONS.map(n => [n, true])) };
  const completed = decodeStage(encodeStage(record, o.body), o.body);
  packing.completeRecord(o.output, completed);
  process.stderr.write("C1_STAGE " + JSON.stringify({ operation: "completion",
    stage_sha256: c.digest(c.readFile(path.join(o.output, "completion.json"), MAX_STAGE_BYTES)) }) + "\n");
  return completed;
}

/** Authenticate an independently pinned completed S artifact, then retrieve its
 * referenced I through readInputs. The required input Buffer is a comparison
 * pin, never an authentication flag. Check both retained packs without packing.
 * Returns staging evidence only, never qualification or execution permission. */
function retainedContext(value) {
  const o = stageOptions(value, true), context = packing.npmContext(o.workParent);
  context.env.PATH = "/usr/local/bin:/usr/bin:/bin";
  return { o, context, source: packing.blobs(o.repo, o.input.identity.commit, context.env, "stage"), toolPins: toolSnapshot(o) };
}
function recheckSubjects(result) {
  for (const row of result.subjects) pinFile(row.file, row.sha256, 128 * 1024 * 1024);
}
function readStage(value) {
  const { o, context, source, toolPins } = retainedContext(value);
  const before = promotion.inspectArtifact(o.artifact, STAGE_WORKFLOW, o.input.identity.commit, context.root);
  const archive = promotion.acquireArtifact(o.artifact, STAGE_WORKFLOW, o.input.identity.commit, context.root);
  const retained = openRetainedStage(o, context, archive);
  const { record, subjects } = retained;
  const multiset = subjects.map(s => ({ name: path.basename(s.file), digest: { sha256: s.sha256 } }));
  for (const subject of subjects) promotion.verifyStageSubject(subject.file, { name: path.basename(subject.file),
    sha256: subject.sha256, source: record.producer.source, workflow_sha: o.workflow_sha, ref: record.producer.ref,
    run_id: record.producer.run_id, run_attempt: record.producer.run_attempt, subjects: multiset }, context.root);
  const result = validateRetainedStage(value, o, context, source, toolPins, retained);
  agreeStage(promotion.inspectArtifact(o.artifact, STAGE_WORKFLOW, o.input.identity.commit, context.root), before, "completed stage provider");
  agreeStage(packing.blobs(o.repo, o.input.identity.commit, context.env, "stage"), source, "source after signatures");
  agreeStage(toolSnapshot(o), toolPins, "tools after signatures");
  agreeStage(stageOptions(value, true), o, "caller after signatures");
  recheckSubjects(result);
  return result;
}
/** Fixed same-run unsigned acquisition, only for paired_stage_attestation.
 * It cannot weaken the completed reader or accept a caller's staging root. */
function validateUnsignedStage(value) {
  const { o, context, source, toolPins } = retainedContext(value);
  const custody = { artifact: o.artifact, selected: o.selected, workflow_sha: o.workflow_sha, scratch: context.root };
  const before = promotion.inspectCurrentStage(custody);
  const archive = promotion.acquireCurrentStage(custody);
  const result = validateRetainedStage(value, o, context, source, toolPins, openRetainedStage(o, context, archive));
  stageCaller(result.record.producer);
  agreeStage(promotion.inspectCurrentStage(custody), before, "current stage provider");
  recheckSubjects(result);
  return result;
}
// Both entrypoints share precisely the retained byte checks. This private
// function has no completed/authenticated switches or injected verifier.
function openRetainedStage(o, context, archive) {
  const names = ["completion.json", ...c.PRODUCTS.map(p => `${inputs.PACKAGES[p]}-${o.input.identity.versions[p]}.tgz`)];
  const root = promotion.extractArtifact(archive, o.artifact, "public-stage", names, path.join(context.root, "stage"), context.root);
  const body = pinFile(path.join(root, "completion.json"), o.stage_sha256, MAX_STAGE_BYTES), record = decodeStage(body, o.body);
  const invocation = stageInvocation(record.producer, o.input);
  agreeStage([invocation.run_id, invocation.run_attempt], [o.artifact.run_id, o.artifact.run_attempt], "stage artifact attempt");
  if ([record.native_inputs.artifact.artifact_id, o.input.preparation.artifact.artifact_id].includes(o.artifact.artifact_id)) {
    throw new Error("separate stage artifact required");
  }
  const subjects = names.map(name => ({ file: path.join(root, name), sha256: name === "completion.json" ? o.stage_sha256 :
    record.packs[c.PRODUCTS.find(p => record.packs[p].file === name)].sha256 }));
  return { root, record, subjects, body };
}
function validateRetainedStage(value, o, context, source, toolPins, retained) {
  const { root, record, subjects, body } = retained;
  const evidence = promotion.checkStageEvidence(o.artifact, o.selected, o.workflow_sha, record, o.stage_sha256, context.root);
  const providers = stageProviders(o, record.native_inputs.artifact, context.root);
  const admitted = stageReadInputs(o, record.native_inputs.artifact, context.root);
  const snapshot = stageInputSnapshot(admitted.root, o.body);
  const pair = pairedPackageFiles(source, manifestsFrom(snapshot), o.body);
  agreeStage(record.wrapper_blobs, sourcePins(source), "authenticated source closure");
  agreeStage(record.generated, generatedPins(pair), "authenticated generated closures");
  for (const product of c.PRODUCTS) {
    agreeStage(retainedPack(root, product, o.input), record.packs[product], "authenticated retained pack");
    packing.verifyPack(path.join(root, record.packs[product].file), pair[product], path.join(context.root, `read-${product}`), context.env);
  }
  agreeStage(promotion.checkStageEvidence(o.artifact, o.selected, o.workflow_sha, record, o.stage_sha256, context.root), evidence, "stage operation evidence");
  agreeStage(stageProviders(o, record.native_inputs.artifact, context.root), providers, "input providers");
  agreeStage(stageInputSnapshot(admitted.root, o.body), snapshot, "reader inputs");
  agreeStage(packing.blobs(o.repo, o.input.identity.commit, context.env, "stage"), source, "reader source");
  agreeStage(toolSnapshot(o), toolPins, "reader tools");
  agreeStage(stageOptions(value, true), o, "reader caller");
  agreeStage(c.readFile(path.join(root, "completion.json"), MAX_STAGE_BYTES), body, "retained S");
  for (const product of c.PRODUCTS) agreeStage(retainedPack(root, product, o.input), record.packs[product], "reader retained pair");
  return { root, record, subjects };
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
function main(args) {
  if (args.length !== 2) throw new Error("one operation and absolute options file required");
  if (args[0] === "--prepare") {
    if (!path.isAbsolute(args[1])) throw new Error("absolute preparation options required");
    return prepare(JSON.parse(c.readFile(args[1], 1024 * 1024)));
  }
  if (!["--stage-prepublication", "--read-stage", "--validate-unsigned-stage"].includes(args[0])) throw new Error("unknown C1 stage operation");
  const producing = args[0] === "--stage-prepublication";
  const transport = inputs.inputFileOptions(args[1], ["input_file", "selected", "workflow_sha", "artifact", "repo", "workParent", "node", "npm",
    ...(producing ? ["producer", "output"] : ["stage_sha256"])]);
  let result;
  if (producing) {
    const record = stagePrepublication(transport.value), root = transport.value.output;
    const stage_sha256 = c.digest(c.readFile(path.join(root, "completion.json"), MAX_STAGE_BYTES));
    const subjects = [{ file: path.join(root, "completion.json"), sha256: stage_sha256 },
      ...c.PRODUCTS.map(product => ({ file: path.join(root, record.packs[product].file), sha256: record.packs[product].sha256 }))];
    result = { root, record, subjects, stage_sha256 };
  } else result = args[0] === "--read-stage" ? readStage(transport.value) : validateUnsignedStage(transport.value);
  transport.recheck();
  return result;
}
// Public blob verification re-enters this module while the CLI is preparing.
module.exports = { prepare, packageFiles, ALLOWLIST, COMMON,
  encodeStage, decodeStage, pairedPackageFiles, STAGE_ALLOWLIST, stagePrepublication, readStage, validateUnsignedStage, main };

if (require.main === module) {
  try { process.stdout.write(c.encode(main(process.argv.slice(2)))); }
  catch (e) { process.stderr.write(`public npm preparation: ${e.message}\n`); process.exitCode = 1; }
}
