"use strict";

// Explicit PRIVATE N1 adapter. No launcher, environment selection or public
// fallback. Identity is trusted only through an independently verified pack.
const fs = require("node:fs");
const fsp = require("node:fs/promises");
const path = require("node:path");
const c = require("../dual-authoring-candidate");
const v = require("../../lib/verifier");
const SCHEMA = "dual-authoring-npm/v1";
const MAX_JSON = 1024 * 1024;
const MAX_BINARY = 128 * 1024 * 1024; // Same bound as the candidate reader/unpack.
const PACKAGES = Object.freeze({ agentplugins: "universal-agent-plugins", "plugin-kit-ai": "plugin-kit-ai" });
const inside = (a, b) => a === b || a.startsWith(b + path.sep);
const hash = value => typeof value === "string" && /^[0-9a-f]{64}$/.test(value);

function json(file) {
  const bytes = c.readFile(file, MAX_JSON);
  const value = JSON.parse(bytes);
  if (!bytes.equals(c.encode(value))) throw new Error("noncanonical private JSON");
  return { bytes, value };
}

function pin(value) {
  if (!hash(value.sha256) || !Number.isSafeInteger(value.size) || value.size <= 0 || value.size > MAX_BINARY) {
    throw new Error("invalid bounded private binary/asset pin");
  }
}

function loadRelease(product, packageRoot, target) {
  if (!Object.hasOwn(PACKAGES, product)) throw new Error("unknown fixed private product");
  c.safeDirectory(packageRoot);
  const { value: descriptor } = json(path.join(packageRoot, "private-release.json"));
  c.keys(descriptor, ["schema", "product", "npm_package", "identity", "asset_scope", "authoring_mode", "candidate_sha256"], "private release");
  c.identity(descriptor.identity);
  if (descriptor.schema !== SCHEMA || descriptor.product !== product || descriptor.npm_package !== PACKAGES[product] ||
      !hash(descriptor.candidate_sha256)) throw new Error("private release binding is invalid");
  // B owns future producer/validator modes. Never accept an either-mode check.
  if (descriptor.authoring_mode !== "vertical-slice-v1") throw new Error("unsupported explicit expected candidate mode");
  if (!c.scopeTargets(descriptor.asset_scope).includes(target)) throw new Error("unsupported private target for expected scope");
  const { value: pkg } = json(path.join(packageRoot, "package.json"));
  c.keys(pkg.bin, [product], "private npm bin");
  if (pkg.name !== PACKAGES[product] || pkg.version !== descriptor.identity.versions[product] ||
      pkg.private !== true || pkg.bin[product] !== `bin/${product}.js`) throw new Error("private npm package binding is invalid");
  const { bytes, value: manifest } = json(path.join(packageRoot, "candidate.json"));
  if (c.digest(bytes) !== descriptor.candidate_sha256) throw new Error("candidate manifest digest mismatch");
  c.manifestShape(manifest, descriptor.identity, descriptor.asset_scope);
  if (manifest.build.authoring_mode !== descriptor.authoring_mode) throw new Error("candidate mode does not match expected mode");
  const seen = new Set();
  for (const p of c.PRODUCTS) for (const asset of Object.values(manifest.products[p].assets)) {
    pin(asset); pin(asset.binary);
    if (p === "agentplugins" && (asset.sha256 !== asset.binary.sha256 || asset.size !== asset.binary.size)) {
      throw new Error("raw asset and binary pins must agree");
    }
    if (seen.has(asset.binary.sha256)) throw new Error("duplicate executable bytes across products/targets");
    seen.add(asset.binary.sha256);
  }
  return { descriptor, manifest, bytes, asset: manifest.products[product].assets[target], version: pkg.version };
}

function sourcePlacement(root, packageRoot, cacheRoot) {
  if (typeof root !== "string" || !path.isAbsolute(root) || path.resolve(root) !== root ||
      root.split(/[\\/]/).includes("..") || root.includes("\0") ||
      !/^[A-Za-z0-9][A-Za-z0-9._-]*$/.test(path.basename(root))) throw new Error("unsafe local candidate root");
  for (const other of [packageRoot, cacheRoot]) {
    if (inside(root, other) || inside(other, root)) throw new Error("private roots overlap");
  }
}

function freezeSelected(root, release) {
  c.safeDirectory(root);
  const names = ["candidate.json", ...c.PRODUCTS.flatMap(p => Object.values(release.manifest.products[p].assets).map(a => a.file))];
  if (fs.readdirSync(root).sort().join("\n") !== names.sort().join("\n")) throw new Error("candidate contains extra or missing files");
  for (const name of ["", "candidate.json", release.asset.file]) {
    const stat = fs.lstatSync(path.join(root, name));
    if (process.platform !== "win32" && (stat.mode & 0o222)) throw new Error("candidate inputs must be immutable");
  }
  const sourceManifest = c.readFile(path.join(root, "candidate.json"), MAX_JSON);
  if (!sourceManifest.equals(release.bytes)) throw new Error("source candidate manifest changed");
  const bytes = c.readFile(path.join(root, release.asset.file), MAX_BINARY);
  c.keys(release.asset.binary, ["file", "sha256", "size"], "binary");
  if (bytes.length !== release.asset.size || c.digest(bytes) !== release.asset.sha256) throw new Error("compressed/raw asset digest or size mismatch");
  const binary = release.descriptor.product === "plugin-kit-ai" ? c.unpack(bytes, release.asset.binary.file) : bytes;
  if (binary.length !== release.asset.binary.size || c.digest(binary) !== release.asset.binary.sha256) throw new Error("inner binary digest or size mismatch");
  return binary;
}

function cachePath(root, product, target, release) {
  return path.join(root, "dual-authoring-npm-v1", release.descriptor.authoring_mode,
    release.descriptor.candidate_sha256, product, release.version, target, release.asset.binary.sha256, release.asset.binary.file);
}

// hooks are internal fault/observation seams for offline structural tests. They
// are never populated from JSON/environment and cannot replace acquisition.
async function ensureBinary(product, options = {}, hooks = {}) {
  const { packageRoot, cacheRoot, candidateRoot, signal } = options;
  v.cancelled(signal);
  const release = loadRelease(product, packageRoot, options.target);
  c.safeDirectory(cacheRoot);
  if (inside(cacheRoot, packageRoot) || inside(packageRoot, cacheRoot)) throw new Error("private cache overlaps package input");
  // Validate a supplied locator even on warm hits, without opening its source.
  if (candidateRoot !== undefined) sourcePlacement(candidateRoot, packageRoot, cacheRoot);
  const io = hooks.io || fsp;
  await v.privateDirectory(cacheRoot, cacheRoot, io);
  const binaryPath = cachePath(cacheRoot, product, options.target, release);
  await v.privateDirectory(cacheRoot, path.dirname(binaryPath), io);
  const lockRoot = path.join(cacheRoot, ".locks");
  await v.privateDirectory(cacheRoot, lockRoot, io);
  const settings = { io, signal, strict: true, osName: options.target.split("-")[0] };
  const unlock = await v.acquireLock(binaryPath, { ...options.lockOptions, io, signal, lockRoot });
  let result, primary;
  try {
    v.cancelled(signal);
    // Warm consumers use the identical lock, including validation under it.
    await v.privateDirectory(cacheRoot, path.dirname(binaryPath), io);
    const hit = await v.strictCachedBinary(binaryPath, release.asset.binary, settings);
    if (!hit) {
      sourcePlacement(candidateRoot, packageRoot, cacheRoot);
      const binary = freezeSelected(candidateRoot, release);
      await hooks.afterFreeze?.();
      v.cancelled(signal);
      await v.commitVerifiedBinary(binary, binaryPath, release.asset.binary, settings);
    }
    v.cancelled(signal);
    result = { binaryPath, version: release.version, cacheHit: hit, product, candidate_sha256: release.descriptor.candidate_sha256 };
  } catch (error) { primary = error; }
  const cleanup = [];
  await unlock().catch(e => cleanup.push(e));
  if (primary || cleanup.length) throw v.failures(primary, cleanup);
  return result;
}

module.exports = { SCHEMA, loadRelease, ensureBinary, cachePath };
