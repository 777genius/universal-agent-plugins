#!/usr/bin/env node
"use strict";

// PRIVATE PREPARATION contract, deliberately unrelated to assets.json schema v2.
// Checksums and Go build info are consistency checks, NOT attestation or platform
// acceptance. Trust starts at the controlled builder and an independently saved
// manifest digest. Never execute a supplied asset to discover its identity.
const crypto = require("node:crypto");
const fs = require("node:fs");
const path = require("node:path");
const zlib = require("node:zlib");

const REPOSITORY = "777genius/universal-agent-plugins";
const SCHEMA = "dual-authoring-candidate/v1";
const PRODUCTS = Object.freeze(["agentplugins", "plugin-kit-ai"]);
const TARGETS = Object.freeze([
  "darwin-amd64", "darwin-arm64", "linux-amd64", "linux-arm64", "windows-amd64", "windows-arm64"
]);
const COMMANDS = "github.com/777genius/plugin-kit-ai/cli/internal/authoring/commands";
const MAX_BINARY = 128 * 1024 * 1024;
const digest = (body) => crypto.createHash("sha256").update(body).digest("hex");
const encode = (value) => Buffer.from(JSON.stringify(value, null, 2) + "\n");

function keys(value, expected, label) {
  if (!value || typeof value !== "object" || Array.isArray(value) ||
      Object.keys(value).sort().join(",") !== [...expected].sort().join(",")) {
    throw new Error(`${label}: unexpected or missing fields`);
  }
}

function identity(value) {
  keys(value, ["repository", "commit", "engine_revision", "versions"], "identity");
  keys(value.versions, PRODUCTS, "product versions");
  if (value.repository !== REPOSITORY || typeof value.commit !== "string" ||
      !/^[0-9a-f]{40}$/.test(value.commit) || value.engine_revision !== value.commit) {
    throw new Error("candidate repository or exact source/engine revision is invalid");
  }
  for (const version of Object.values(value.versions)) {
    if (typeof version !== "string" || !/^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$/.test(version)) {
      throw new Error("candidate requires exact product versions");
    }
  }
  if (value.versions.agentplugins === value.versions["plugin-kit-ai"]) {
    throw new Error("candidate product versions must be distinct; engine identity is separate");
  }
  return value;
}

function assetName(product, version, target) {
  if (!PRODUCTS.includes(product) || !TARGETS.includes(target)) throw new Error("unknown product or target");
  const suffix = product === "plugin-kit-ai" ? ".tar.gz" : target.startsWith("windows-") ? ".exe" : "";
  return `${product}_${version}_${target.replace("-", "_")}${suffix}`;
}

function executableName(product, target) {
  return product + (target.startsWith("windows-") ? ".exe" : "");
}

function authoringMode(value = "vertical-slice-v1") {
  if (!["vertical-slice-v1", "release-cli-contract-v1"].includes(value)) throw new Error("unknown private authoring mode");
  return value;
}

function linkerFlags(product, id, mode) {
  return `-X main.version=${id.versions[product]} -X ${COMMANDS}.Enabled=${authoringMode(mode)} -X ${COMMANDS}.Revision=${id.commit}`;
}

// Walk every component, including ancestors, before resolving paths. This is a
// private, quiescent local namespace contract, not protection from a hostile
// same-UID process or privileged mount replacement. Never normalize '..' away.
function safeDirectory(value) {
  if (typeof value !== "string" || !path.isAbsolute(value) || value.includes("\0") ||
      value.split(/[\\/]/).includes("..") || path.resolve(value) !== value) {
    throw new Error("directory must be an absolute normalized safe path");
  }
  let current = path.parse(value).root;
  for (const component of value.slice(current.length).split(path.sep).filter(Boolean)) {
    current = path.join(current, component);
    const stat = fs.lstatSync(current);
    if (!stat.isDirectory() || stat.isSymbolicLink()) throw new Error("directory contains a symlink or non-directory");
  }
  return value;
}

function inside(a, b) { return a === b || a.startsWith(b + path.sep); }

function outputPlacement(output, protectedRoots) {
  if (typeof output !== "string" || !path.isAbsolute(output) || path.resolve(output) !== output ||
      output.split(/[\\/]/).includes("..") || !/^[A-Za-z0-9][A-Za-z0-9._-]*$/.test(path.basename(output))) {
    throw new Error("unsafe candidate output path");
  }
  safeDirectory(path.dirname(output));
  for (const root of protectedRoots) {
    safeDirectory(root);
    if (inside(output, root) || inside(root, output)) throw new Error("candidate output overlaps protected input");
  }
  try {
    fs.lstatSync(output);
  } catch (error) {
    if (error.code === "ENOENT") return;
    throw error;
  }
  throw new Error("candidate output already exists");
}

// O_NOFOLLOW plus descriptor identity/link-count checks freeze the read bytes.
// Ancestor checks rely on the private/quiescent namespace boundary above.
function readFile(file, maximum = MAX_BINARY) {
  safeDirectory(path.dirname(file));
  const before = fs.lstatSync(file);
  if (!before.isFile() || before.nlink !== 1 || before.size <= 0 || before.size > maximum) {
    throw new Error("candidate file must be regular, nonempty, bounded and unaliased");
  }
  const fd = fs.openSync(file, fs.constants.O_RDONLY | (fs.constants.O_NOFOLLOW || 0));
  try {
    const opened = fs.fstatSync(fd);
    if (opened.dev !== before.dev || opened.ino !== before.ino || opened.nlink !== 1) throw new Error("candidate file changed");
    const body = fs.readFileSync(fd);
    const after = fs.fstatSync(fd);
    const named = fs.lstatSync(file);
    if (body.length !== before.size || after.size !== before.size || after.mtimeMs !== before.mtimeMs ||
        after.ctimeMs !== before.ctimeMs || after.nlink !== 1 || named.dev !== before.dev || named.ino !== before.ino) {
      throw new Error("candidate file changed while freezing bytes");
    }
    return body;
  } finally { fs.closeSync(fd); }
}

// A deliberately tiny deterministic ustar subset: exactly one ordinary product
// executable, no paths, extension records, links, trailing files or extraction.
// Public plugin-kit archive filenames remain the existing six tar.gz names.
function archive(binary, name) {
  if (!/^(agentplugins|plugin-kit-ai)(\.exe)?$/.test(name) || !binary.length || binary.length > MAX_BINARY) {
    throw new Error("invalid candidate archive input");
  }
  const header = Buffer.alloc(512);
  const field = (offset, length, value) => header.write(value, offset, length, "ascii");
  field(0, 100, name);
  field(100, 8, "0000755\0");
  field(108, 8, "0000000\0");
  field(116, 8, "0000000\0");
  field(124, 12, binary.length.toString(8).padStart(11, "0") + "\0");
  field(136, 12, "00000000000\0");
  header.fill(32, 148, 156);
  field(156, 1, "0");
  field(257, 6, "ustar\0");
  field(263, 2, "00");
  const sum = header.reduce((a, b) => a + b, 0);
  field(148, 8, sum.toString(8).padStart(6, "0") + "\0 ");
  return zlib.gzipSync(Buffer.concat([header, binary, Buffer.alloc((512 - binary.length % 512) % 512 + 1024)]), { level: 9 });
}

function unpack(body, name) {
  const tar = zlib.gunzipSync(body, { maxOutputLength: MAX_BINARY + 2048 });
  if (tar.length < 1536) throw new Error("invalid candidate archive");
  const sizeText = tar.subarray(124, 136).toString("ascii");
  if (!/^[0-7]{11}\0$/.test(sizeText)) throw new Error("invalid archive size");
  const size = parseInt(sizeText, 8);
  if (size <= 0 || size > MAX_BINARY || 512 + size > tar.length) throw new Error("invalid archive binary size");
  const binary = tar.subarray(512, 512 + size);
  // Compare the uncompressed canonical record too, rejecting alternate headers,
  // extra members, unsafe names and padding. No archive is ever extracted to disk.
  if (!tar.equals(zlib.gunzipSync(archive(binary, name)))) throw new Error("noncanonical or unsafe candidate archive");
  return binary;
}

function metadata(body) { return { sha256: digest(body), size: body.length }; }

// Two bounded preparation scopes. A Linux pair supports this checkpoint's real
// offline journey without pretending missing target assets passed a release gate.
// The six-platform scope is closed; neither scope changes published v2 assets.
function scopeTargets(scope) {
  if (scope === "linux-amd64-pair") return ["linux-amd64"];
  if (scope === "six-platform-pair") return TARGETS;
  throw new Error("unknown candidate asset scope");
}

function checkMetadata(value, body, label) {
  keys(value, ["sha256", "size"], label);
  if (typeof value.sha256 !== "string" || !/^[0-9a-f]{64}$/.test(value.sha256) ||
      !Number.isSafeInteger(value.size) || value.size <= 0 ||
      value.sha256 !== digest(body) || value.size !== body.length) throw new Error(`${label}: digest or size mismatch`);
}

function manifestShape(manifest, expected, expectedScope, expectedMode) {
  const mode = authoringMode(expectedMode);
  identity(expected);
  const targets = scopeTargets(expectedScope);
  keys(manifest, ["schema", "status", "identity", "asset_scope", "build", "products", "release_eligible"], "candidate");
  identity(manifest.identity);
  if (manifest.schema !== SCHEMA || manifest.asset_scope !== expectedScope || manifest.status !== "CANDIDATE" || manifest.release_eligible !== false ||
      manifest.identity.commit !== expected.commit || manifest.identity.repository !== expected.repository ||
      PRODUCTS.some((p) => manifest.identity.versions[p] !== expected.versions[p])) throw new Error("candidate identity/schema mismatch");
  keys(manifest.build, ["method", "go_version", "go_sha256", "source_archive_sha256", "authoring_mode"], "build");
  if (manifest.build.method !== "controlled-git-archive-go-build/v1" || manifest.build.go_version !== "go1.25.13" ||
      manifest.build.authoring_mode !== mode ||
      typeof manifest.build.go_sha256 !== "string" || typeof manifest.build.source_archive_sha256 !== "string" ||
      !/^[0-9a-f]{64}$/.test(manifest.build.go_sha256) || !/^[0-9a-f]{64}$/.test(manifest.build.source_archive_sha256)) {
    throw new Error("candidate controlled build description is invalid");
  }
  keys(manifest.products, PRODUCTS, "products");
  for (const product of PRODUCTS) {
    const value = manifest.products[product];
    keys(value, ["version", "assets"], product);
    if (value.version !== expected.versions[product]) throw new Error("wrong product version");
    keys(value.assets, targets, `${product} assets`);
    for (const target of targets) {
      const asset = value.assets[target];
      keys(asset, ["file", "sha256", "size", "binary"], "asset");
      keys(asset.binary, ["file", "sha256", "size"], "binary");
      if (asset.file !== assetName(product, value.version, target) || asset.binary.file !== executableName(product, target)) {
        throw new Error("wrong product asset or binary name");
      }
    }
  }
}

// This function validates STRUCTURE/BYTES ONLY. The public verifier additionally
// checks embedded product/build settings with the trusted Go tool. Neither is a
// platform gate, and neither establishes provenance from untrusted metadata.
function frozenCandidate(root, expected, manifestDigest, expectedScope, expectedMode) {
  safeDirectory(root);
  if (typeof manifestDigest !== "string" || !/^[0-9a-f]{64}$/.test(manifestDigest)) throw new Error("independent manifest digest is required");
  const body = readFile(path.join(root, "candidate.json"), 1024 * 1024);
  if (digest(body) !== manifestDigest) throw new Error("candidate manifest digest mismatch");
  const manifest = JSON.parse(body);
  // Require canonical encoding: duplicate JSON keys and ambiguous encodings fail.
  if (!body.equals(encode(manifest))) throw new Error("noncanonical candidate JSON");
  manifestShape(manifest, expected, expectedScope, expectedMode);
  const expectedFiles = ["candidate.json"];
  const binaries = [];
  const seen = new Set();
  for (const product of PRODUCTS) {
    for (const target of scopeTargets(expectedScope)) {
      const asset = manifest.products[product].assets[target];
      expectedFiles.push(asset.file);
      const bytes = readFile(path.join(root, asset.file));
      checkMetadata({ sha256: asset.sha256, size: asset.size }, bytes, "asset");
      const binary = product === "plugin-kit-ai" ? unpack(bytes, asset.binary.file) : bytes;
      checkMetadata({ sha256: asset.binary.sha256, size: asset.binary.size }, binary, "binary");
      if (seen.has(asset.binary.sha256)) throw new Error("duplicate executable bytes across products/targets");
      seen.add(asset.binary.sha256);
      binaries.push({ product, target, binary });
    }
  }
  if (fs.readdirSync(root).sort().join("\n") !== expectedFiles.sort().join("\n")) throw new Error("candidate contains extra or missing files");
  return { manifest, binaries, manifest_sha256: manifestDigest };
}

module.exports = {
  REPOSITORY, SCHEMA, PRODUCTS, TARGETS, COMMANDS, digest, encode, keys, identity,
  assetName, executableName, authoringMode, linkerFlags, safeDirectory, outputPlacement, readFile,
  archive, unpack, metadata, scopeTargets, manifestShape, frozenCandidate
};
