#!/usr/bin/env node
"use strict";

// Preparation only. B must authenticate real signed promotion inputs before it
// can introduce public staging. This entrypoint never generates qualification.
const fs = require("node:fs");
const path = require("node:path");
const c = require("./dual-authoring-candidate");
const adapter = require("./authoring-release");
const packing = require("./stage-dual-authoring-npm");
const runtime = require("../lib/public-authoring");
const PREFIX = "npm/agentplugins/";
const COMMON = Object.freeze(["lib/verifier.js", "lib/public-authoring.js", "scripts/dual-authoring-candidate.js"]);
const ownFiles = product => ["LICENSE", "README.md", "package.json", `bin/${product}.js`, "lib/platform.js",
  product === "agentplugins" ? "lib/bootstrap.js" : "lib/install.js"];
const ALLOWLIST = Object.freeze([...new Set([...COMMON.map(n => PREFIX + n),
  ...c.PRODUCTS.flatMap(p => ownFiles(p).map(n => `npm/${p}/${n}`)),
  ...["stage-authoring-npm.js", "stage-dual-authoring-npm.js", "stage-dual-authoring-candidate.js",
    "authoring-release.js"].map(n => PREFIX + "scripts/" + n)])]);
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
module.exports = { prepare, packageFiles, ALLOWLIST, COMMON };

if (require.main === module) {
  try {
    if (process.argv.length !== 4 || process.argv[2] !== "--prepare" || !path.isAbsolute(process.argv[3])) throw new Error("usage: stage-authoring-npm.js --prepare <absolute-options.json>");
    process.stdout.write(c.encode(prepare(JSON.parse(c.readFile(process.argv[3], 1024 * 1024)))));
  } catch (e) { process.stderr.write(`public npm preparation: ${e.message}\n`); process.exitCode = 1; }
}
