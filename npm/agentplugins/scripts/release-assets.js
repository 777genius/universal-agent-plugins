#!/usr/bin/env node
"use strict";

const crypto = require("node:crypto");
const fs = require("node:fs");
const path = require("node:path");

const VERSION = /^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$/;
const COMMIT = /^[0-9a-f]{40}$/;
const DIGEST = /^[0-9a-f]{64}$/;
const PRODUCER_REPOSITORY = "777genius/plugin-kit-ai";
const TARGETS = [
  ["darwin-amd64", "darwin", "amd64", ""],
  ["darwin-arm64", "darwin", "arm64", ""],
  ["linux-amd64", "linux", "amd64", ""],
  ["linux-arm64", "linux", "arm64", ""],
  ["windows-amd64", "windows", "amd64", ".exe"],
  ["windows-arm64", "windows", "arm64", ".exe"]
];

function sha256(file) {
  return crypto.createHash("sha256").update(fs.readFileSync(file)).digest("hex");
}

function regularUnaliasedFile(file, label) {
  const stat = fs.lstatSync(file);
  if (!stat.isFile() || stat.isSymbolicLink() || stat.size <= 0 || stat.nlink !== 1) {
    throw new Error(`${label} is not a regular, non-empty, unaliased file`);
  }
  return stat;
}

function validateReleaseRoot(assetRoot) {
  const stat = fs.lstatSync(assetRoot);
  if (!stat.isDirectory() || stat.isSymbolicLink()) {
    throw new Error("release root must be a real directory");
  }
}

function releaseIdentity(tag, commit) {
  if (!/^agentplugins-v\d+\.\d+\.\d+$/.test(tag)) {
    throw new Error(`invalid stable release tag: ${tag}`);
  }
  const version = tag.slice("agentplugins-v".length);
  if (!VERSION.test(version) || !COMMIT.test(commit)) {
    throw new Error("release version or commit is invalid");
  }
  return { tag, version, commit };
}

function expectedAssets(version) {
  return Object.fromEntries(TARGETS.map(([key, osName, archName, suffix]) => [
    key,
    `agentplugins_${version}_${osName}_${archName}${suffix}`
  ]));
}

function assetMetadata(assetRoot, version) {
  const assets = {};
  const identities = new Set();
  for (const [key, file] of Object.entries(expectedAssets(version))) {
    const filePath = path.join(assetRoot, file);
    const stat = regularUnaliasedFile(filePath, `release asset ${file}`);
    const identity = `${stat.dev}:${stat.ino}`;
    if (identities.has(identity)) throw new Error(`release asset is aliased: ${file}`);
    identities.add(identity);
    assets[key] = { file, sha256: sha256(filePath), size: stat.size };
  }
  return assets;
}

function writeChecksums(assetRoot, assets) {
  const names = [...Object.values(assets).map((asset) => asset.file), "release-manifest.json"];
  const body = names.map((file) => `${sha256(path.join(assetRoot, file))}  ${file}`).join("\n") + "\n";
  fs.writeFileSync(path.join(assetRoot, "checksums.txt"), body);
}

function prepareRelease(assetRoot, tag, commit) {
  const identity = releaseIdentity(tag, commit);
  const manifest = {
    schema_version: 2,
    ...identity,
    assets: assetMetadata(assetRoot, identity.version)
  };
  fs.writeFileSync(path.join(assetRoot, "release-manifest.json"), JSON.stringify(manifest, null, 2) + "\n");
  writeChecksums(assetRoot, manifest.assets);
  return manifest;
}

function parseChecksums(assetRoot) {
  const body = fs.readFileSync(path.join(assetRoot, "checksums.txt"), "utf8");
  const entries = new Map();
  for (const line of body.split(/\r?\n/)) {
    if (!line) continue;
    const match = line.match(/^([0-9a-f]{64})  ([A-Za-z0-9_.-]+)$/);
    if (!match || entries.has(match[2])) {
      throw new Error("checksums.txt contains an invalid or duplicate entry");
    }
    entries.set(match[2], match[1]);
  }
  return entries;
}

function verifyChecksums(assetRoot, expectedNames) {
  const entries = parseChecksums(assetRoot);
  if (entries.size !== expectedNames.length || expectedNames.some((name) => !entries.has(name))) {
    throw new Error("checksums.txt does not name exactly the six binaries and release manifest");
  }
  for (const name of expectedNames) {
    if (entries.get(name) !== sha256(path.join(assetRoot, name))) {
      throw new Error(`checksum mismatch: ${name}`);
    }
  }
}

function verifyRelease(assetRoot, tag, commit, options = {}) {
  validateReleaseRoot(assetRoot);
  const identity = releaseIdentity(tag, commit);
  const manifestPath = path.join(assetRoot, "release-manifest.json");
  regularUnaliasedFile(manifestPath, "release manifest");
  regularUnaliasedFile(path.join(assetRoot, "checksums.txt"), "release checksums");
  const manifest = JSON.parse(fs.readFileSync(manifestPath, "utf8"));
  if (manifest.tag !== identity.tag || manifest.commit !== identity.commit) {
    throw new Error("release manifest identity does not match the exact tag and commit");
  }
  const computed = assetMetadata(assetRoot, identity.version);
  const names = [...Object.values(computed).map((asset) => asset.file), "release-manifest.json"];
  verifyChecksums(assetRoot, names);
  const expectedFiles = [...names, "checksums.txt"].sort();
  const actualFiles = fs.readdirSync(assetRoot).sort();
  if (actualFiles.join("\n") !== expectedFiles.join("\n")) {
    throw new Error("release directory must contain exactly the six binaries, checksums, and manifest");
  }

  if (manifest.schema_version === 1 && options.allowLegacyManifest === true) {
    if (Object.keys(manifest).sort().join(",") !== "commit,schema_version,tag") {
      throw new Error("legacy release manifest has unexpected fields");
    }
    return {
      repository: PRODUCER_REPOSITORY,
      ...identity,
      assets: computed,
      manifest_schema: 1,
      manifest_sha256: sha256(manifestPath),
      gate_eligible: false
    };
  }
  if (manifest.schema_version !== 2 || manifest.version !== identity.version) {
    throw new Error("release manifest schema or version does not match");
  }
  if (Object.keys(manifest).sort().join(",") !== "assets,commit,schema_version,tag,version") {
    throw new Error("release manifest has unexpected or missing fields");
  }
  const expectedKeys = Object.keys(computed).sort();
  if (!manifest.assets || Object.keys(manifest.assets).sort().join(",") !== expectedKeys.join(",")) {
    throw new Error("release manifest must contain exactly six target assets");
  }
  for (const key of expectedKeys) {
    const actual = manifest.assets[key];
    const expected = computed[key];
    if (!actual || Object.keys(actual).sort().join(",") !== "file,sha256,size" ||
        actual.file !== expected.file || actual.sha256 !== expected.sha256 || actual.size !== expected.size ||
        !DIGEST.test(String(actual.sha256)) || !Number.isSafeInteger(actual.size) || actual.size <= 0) {
      throw new Error(`release manifest asset metadata mismatch: ${key}`);
    }
  }
  return {
    repository: PRODUCER_REPOSITORY,
    ...identity,
    assets: computed,
    manifest_schema: 2,
    manifest_sha256: sha256(manifestPath),
    gate_eligible: true
  };
}

function main() {
  const [command, rootArg, tag, commit, policy] = process.argv.slice(2);
  if (!command || !rootArg || !tag || !commit) {
    throw new Error("usage: release-assets.js <prepare|verify> <asset-root> <tag> <commit> [allow-legacy-v1]");
  }
  const assetRoot = path.resolve(rootArg);
  const result = command === "prepare"
    ? prepareRelease(assetRoot, tag, commit)
    : command === "verify"
      ? verifyRelease(assetRoot, tag, commit, { allowLegacyManifest: policy === "allow-legacy-v1" })
      : (() => { throw new Error(`unknown command: ${command}`); })();
  process.stdout.write(JSON.stringify(result, null, 2) + "\n");
}

if (require.main === module) {
  try {
    main();
  } catch (error) {
    process.stderr.write(`agentplugins release assets: ${error.message}\n`);
    process.exitCode = 1;
  }
}

module.exports = { PRODUCER_REPOSITORY, expectedAssets, prepareRelease, verifyRelease };
