"use strict";

// Codecs and subject enumeration are structural only. The bounded producer
// prepares unsigned I; only readInputs invokes the fixed signature boundary.
// Neither operation grants qualification, publication or execution permission.
const c = require("./dual-authoring-candidate");
const fs = require("node:fs");
const path = require("node:path");
const { isDeepStrictEqual: equal } = require("node:util");

const { encodeInputs, decodeInputs, encodeDescriptor, decodeDescriptor, INPUT_SCHEMA, DESCRIPTOR_SCHEMA, INPUT_FILE, MODE, SCOPE, WORKFLOW, PACKAGES, MAX_INPUT_BYTES, MAX_DESCRIPTOR_BYTES, MAX_NATIVE_BYTES, checks: { fail, fixed, hash, positive, fields } } = require("../lib/public-authoring-contract");

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
  p.checkPreparationRef(prepPin, o.selected, o.scratch);
  p.checkPreparationRef(o.artifact, o.selected, o.scratch);
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
  p.checkPreparationRef(prepPin, o.selected, o.scratch);
  p.checkPreparationRef(o.artifact, o.selected, o.scratch);
  agree(operationOptions(value, true), o, "caller inputs changed during authentication");
  return { root, input: o.input, subjects };
}

/** Derive I only from the checked original preparation and independently bound
 * current provenance caller. Both fixed jobs repeat this operation. */
function produceInputsFromPreparation(value) {
  fields(value, ["selected", "workflow_sha", "preparation", "repo", "scratch"], "preparation producer options");
  const p = require("./authoring-promotion"), packing = require("./stage-dual-authoring-npm");
  const selected = p.workflowSelection(value.selected, value.workflow_sha);
  const pin = artifact(value.preparation);
  for (const name of ["repo", "scratch"]) c.safeDirectory(value[name]);
  const executing = path.resolve(__dirname, "../../..");
  for (const root of [value.repo, executing]) {
    if (root === value.scratch || root.startsWith(value.scratch + path.sep) || value.scratch.startsWith(root + path.sep)) {
      fail("provenance source/scratch overlap");
    }
  }
  const env = { PATH: "/usr/local/bin:/usr/bin:/bin", HOME: value.scratch, LC_ALL: "C.UTF-8" };
  const toolFiles = [process.execPath, "/usr/bin/git", "/usr/bin/gh", fs.realpathSync("/usr/bin/python3")];
  const tools = () => toolFiles.map(file => ({ file, sha256: c.digest(c.readFile(file)) }));
  const toolPins = tools(), callerArgs = [...process.execArgv];
  const source = packing.blobs(value.repo, selected.source, env, "stage");
  const caller = p.inspectInputCaller(selected, value.workflow_sha, value.scratch);
  if (caller.run_id === pin.run_id) fail("separate original preparation required");
  const before = p.inspectArtifact(pin, WORKFLOW, selected.source, value.scratch);
  p.checkPreparationRef(pin, selected, value.scratch);
  const work = fs.mkdtempSync(path.join(value.scratch, "derive-inputs-"));
  const archive = p.acquireArtifact(pin, WORKFLOW, selected.source, work);
  const files = ["preparation-run.json", "candidate-identity.json", "candidate/candidate.json", "pair-prepared.json",
    ...c.PRODUCTS.flatMap(product => [...c.TARGETS.map(target =>
      `${product}/${c.assetName(product, selected.versions[product], target)}`),
    `${product}/release-manifest.json`, `${product}/checksums.txt`])];
  const root = p.extractArtifact(archive, pin, "preparation", files, path.join(work, "frozen"), work);
  const snapshot = () => files.map(file => ({ file, sha256: c.digest(c.readFile(path.join(root, file), MAX_NATIVE_BYTES)) }));
  const originals = snapshot();
  const candidateBytes = c.readFile(path.join(root, "candidate/candidate.json"), MAX_INPUT_BYTES);
  const candidate = JSON.parse(candidateBytes);
  const id = c.identity({ repository: c.REPOSITORY, commit: selected.source, engine_revision: selected.source, versions: selected.versions });
  const tentative = { schema: INPUT_SCHEMA, identity: id, authoring_mode: MODE, asset_scope: SCOPE,
    candidate_sha256: c.digest(candidateBytes), pair_marker_sha256: c.digest(c.readFile(path.join(root, "pair-prepared.json"), MAX_INPUT_BYTES)),
    products: Object.fromEntries(c.PRODUCTS.map(product => [product, {
      tag: (product === "agentplugins" ? "agentplugins-v" : "v") + selected.versions[product],
      manifest_sha256: c.digest(c.readFile(path.join(root, product, "release-manifest.json"), MAX_INPUT_BYTES)),
      checksums_sha256: c.digest(c.readFile(path.join(root, product, "checksums.txt"), MAX_INPUT_BYTES)),
      assets: candidate.products?.[product]?.assets }])),
    preparation: { sha256: c.digest(c.readFile(path.join(root, "preparation-run.json"), MAX_INPUT_BYTES)), artifact: pin },
    producer: caller };
  const body = encodeInputs(tentative);
  preparationSnapshot(root, body); // candidate, projections, metadata and original receipt
  const result = produceInputs({ input: body, selected, workflow_sha: value.workflow_sha, scratch: value.scratch });
  const retained = [...inputSubjects(result.root, body), ...["preparation-run.json", "candidate-identity.json"].map(file =>
    ({ file: path.join(result.root, file), sha256: c.digest(c.readFile(path.join(result.root, file), MAX_INPUT_BYTES)) }))];
  agree(retained.filter(row => path.basename(row.file) !== INPUT_FILE).map(row =>
    ({ file: path.relative(result.root, row.file), sha256: row.sha256 })).sort((a, b) => a.file.localeCompare(b.file)),
  [...originals].sort((a, b) => a.file.localeCompare(b.file)), "derived versus revalidated original payload");
  agree(snapshot(), originals, "derivation root changed");
  agree(p.inspectArtifact(pin, WORKFLOW, selected.source, value.scratch), before, "original provider changed");
  p.checkPreparationRef(pin, selected, value.scratch);
  agree(p.inspectInputCaller(selected, value.workflow_sha, value.scratch), caller, "current provenance caller");
  agree(packing.blobs(value.repo, selected.source, env, "stage"), source, "provenance source changed");
  agree(p.workflowSelection(value.selected, value.workflow_sha), selected, "provenance selection changed");
  agree(tools(), toolPins, "provenance tools changed");
  agree(process.execArgv, callerArgs, "provenance interpreter arguments");
  return result;
}

// Fixed file transport for the two C1 modules. Options remain comparison pins;
// reading a file never turns its I bytes into authenticated input provenance.
function inputFileOptions(file, names) {
  if (typeof file !== "string" || !path.isAbsolute(file) || path.resolve(file) !== file || /[\x00-\x1f]/.test(file)) {
    fail("normalized absolute options file required");
  }
  const raw = c.readFile(file, MAX_INPUT_BYTES);
  const value = JSON.parse(new TextDecoder("utf-8", { fatal: true }).decode(raw));
  fields(value, names, "CLI options");
  // Canonical options prevent duplicate keys and ambiguous transport spelling.
  if (!raw.equals(c.encode(value))) fail("canonical options required");
  let input, inputFile;
  if (names.includes("input_file")) {
    inputFile = value.input_file;
    if (typeof inputFile !== "string" || !path.isAbsolute(inputFile) || path.resolve(inputFile) !== inputFile ||
        /[\x00-\x1f]/.test(inputFile) || inputFile === file) fail("normalized distinct input_file required");
    input = c.readFile(inputFile, MAX_INPUT_BYTES);
    decodeInputs(input);
    delete value.input_file;
    value.input = input;
  }
  const files = [file, ...(inputFile ? [inputFile] : [])];
  for (const root of [path.resolve(__dirname, "../../.."), value.repo,
    ...[value.node, value.npm].filter(v => typeof v === "string").map(v => path.dirname(v))].filter(Boolean)) {
    for (const candidate of files) if (candidate === root || candidate.startsWith(root + path.sep)) fail("CLI transport overlaps source/tools");
  }
  return { value, recheck() {
    agree(c.readFile(file, MAX_INPUT_BYTES), raw, "CLI options bytes");
    if (inputFile) agree(c.readFile(inputFile, MAX_INPUT_BYTES), input, "CLI comparison I bytes");
  } };
}

function main(args) {
  if (args.length !== 2 || !["--produce-inputs", "--read-inputs"].includes(args[0])) {
    fail("usage: authoring-native-inputs.js --produce-inputs|--read-inputs <absolute-options.json>");
  }
  const producing = args[0] === "--produce-inputs";
  const transport = inputFileOptions(args[1], producing ?
    ["selected", "workflow_sha", "preparation", "repo", "scratch"] :
    ["input_file", "selected", "workflow_sha", "scratch", "artifact"]);
  const result = producing ? produceInputsFromPreparation(transport.value) : readInputs(transport.value);
  transport.recheck();
  return result;
}

module.exports = Object.freeze({ encodeInputs, decodeInputs, encodeDescriptor, decodeDescriptor,
  produceInputs, readInputs, inputSubjects, produceInputsFromPreparation, inputFileOptions, main,
  INPUT_SCHEMA, DESCRIPTOR_SCHEMA, INPUT_FILE, MODE, SCOPE, WORKFLOW, PACKAGES,
  MAX_INPUT_BYTES, MAX_DESCRIPTOR_BYTES, MAX_NATIVE_BYTES });

if (require.main === module) {
  try { process.stdout.write(c.encode(main(process.argv.slice(2)))); }
  catch (error) { process.stderr.write(`C1 inputs: ${error.message}\n`); process.exitCode = 1; }
}
