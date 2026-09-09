"use strict";

// Package consistency, not a signature verifier. The future protected public
// stager must authenticate the signed pair before emitting a binding. Preparation
// emits null. No runtime environment/CLI option can qualify a preparation pack.
const fs = require("node:fs");
const fsp = require("node:fs/promises");
const os = require("node:os");
const path = require("node:path");
const c = require("../scripts/dual-authoring-candidate");
const v = require("./verifier");
const SCHEMA = "dual-authoring-public-npm/v1";
const MODE = "release-cli-contract-v1";
const SCOPE = "six-platform-pair";
const PACKAGES = Object.freeze({ agentplugins: "universal-agent-plugins", "plugin-kit-ai": "plugin-kit-ai" });
const hash = x => typeof x === "string" && /^[0-9a-f]{64}$/.test(x);
const inside = (a, b) => a === b || a.startsWith(b + path.sep);
const equal = (a, b) => c.encode(a).equals(c.encode(b));

function json(file) {
  const bytes = c.readFile(file, 1024 * 1024);
  const value = JSON.parse(bytes);
  if (!bytes.equals(c.encode(value))) throw new Error("noncanonical public JSON");
  return { bytes, value };
}
function pin(value) {
  if (!hash(value.sha256) || !Number.isSafeInteger(value.size) || value.size <= 0 || value.size > 128 * 1024 * 1024) {
    throw new Error("invalid bounded public asset/binary pin");
  }
}

function qualification(descriptor) {
  const q = descriptor.qualification;
  if (q === null) throw new Error("public authoring is not qualified: preparation package");
  // This is the normalized package binding specified for authenticated staging,
  // NOT an encoding/parser for the still-unavailable signed promotion record.
  // B owns verification of exact native subjects, checksums, manifests, candidate,
  // workflow/source, immutable public tags and all required qualification lanes.
  c.keys(q, ["identity", "candidate_sha256", "manifest_sha256", "signed_subject"], "qualification binding");
  c.keys(q.manifest_sha256, c.PRODUCTS, "qualified pair manifests");
  c.keys(q.signed_subject, ["sha256", "workflow", "source"], "signed qualification subject");
  if (!equal(q.identity, descriptor.identity) || q.candidate_sha256 !== descriptor.candidate_sha256 ||
      !Object.values(q.manifest_sha256).every(hash) ||
      q.manifest_sha256[descriptor.product] !== descriptor.release_manifest_sha256 ||
      !hash(q.signed_subject.sha256) || q.signed_subject.source !== descriptor.identity.commit ||
      typeof q.signed_subject.workflow !== "string" ||
      !/^777genius\/universal-agent-plugins\/\.github\/workflows\/[a-z0-9-]+\.yml$/.test(q.signed_subject.workflow)) {
    throw new Error("public qualification binding mismatch");
  }
}

function validateRelease(product, packageRoot, target, read = json, inputBytes) {
  if (!Object.hasOwn(PACKAGES, product) || !c.TARGETS.includes(target)) throw new Error("unsupported public product/target");
  c.safeDirectory(packageRoot);
  const { value: d, bytes: descriptorBytes } = read(path.join(packageRoot, "public-release.json"));
  const v2 = inputBytes !== undefined;
  const contract = v2 ? require("./public-authoring-contract") : null;
  if (v2) contract.decodeDescriptor(descriptorBytes, inputBytes, product);
  c.keys(d, ["schema", "product", "npm_package", "identity", "authoring_mode", "asset_scope",
    "candidate_sha256", "release_manifest_sha256", v2 ? "input_binding" : "qualification"], "public release");
  c.identity(d.identity);
  if (d.schema !== (v2 ? contract.DESCRIPTOR_SCHEMA : SCHEMA) || d.product !== product || d.npm_package !== PACKAGES[product] ||
      /^0{40}$/.test(d.identity.commit) || d.identity.versions["plugin-kit-ai"] !== "2.0.0" ||
      d.authoring_mode !== MODE || d.asset_scope !== SCOPE || !hash(d.candidate_sha256) || !hash(d.release_manifest_sha256)) {
    throw new Error("public release identity mismatch");
  }
  const { value: pkg } = read(path.join(packageRoot, "package.json"));
  c.keys(pkg, ["name", "version", "description", "license", "homepage", "repository", "bugs", "keywords",
    "engines", "publishConfig", "files", "bin", "scripts", "private", ...(product === "agentplugins" ? ["os", "cpu"] : [])], "public package");
  const closure = ["LICENSE", "README.md", "package.json", `bin/${product}.js`, "bin/package.json", "lib/package.json",
    "lib/platform.js", "lib/verifier.js", "lib/public-authoring.js", "scripts/package.json", "scripts/dual-authoring-candidate.js",
    product === "agentplugins" ? "lib/bootstrap.js" : "lib/install.js", "public-release.json", "release-manifest.json"];
  if (v2) closure.push("native-inputs.json", "lib/public-authoring-contract.js", "lib/public-authoring-input.js");
  if (v2 && (pkg.private !== false || (product === "agentplugins" &&
      (!equal(pkg.os, ["darwin", "linux", "win32"]) || !equal(pkg.cpu, ["x64", "arm64"]))))) {
    throw new Error("public v2 package private/OS/CPU mismatch");
  }
  if (typeof pkg.private !== "boolean" || !Array.isArray(pkg.files) || !equal(pkg.files, closure.sort())) {
    throw new Error("public package closure mismatch");
  }
  c.keys(pkg.bin, [product], "public npm bin");
  c.keys(pkg.engines, ["node"], "public npm engines");
  const scripts = product === "agentplugins" ? { test: "node --test" } : { postinstall: "node ./lib/install.js" };
  if (pkg.name !== PACKAGES[product] || pkg.version !== d.identity.versions[product] ||
      pkg.bin[product] !== `bin/${product}.js` || pkg.engines.node !== (product === "agentplugins" ? ">=22" : ">=18") ||
      !equal(pkg.scripts, scripts)) throw new Error("public npm package binding mismatch");
  const { value: m, bytes } = read(path.join(packageRoot, "release-manifest.json"));
  c.keys(m, ["schema_version", "status", "product", "repository", "tag", "version", "commit", "engine_revision",
    "versions", "candidate_sha256", "authoring_mode", "asset_scope", "assets", "release_eligible", "platform_acceptance", "attested"], "producer projection");
  if (c.digest(bytes) !== d.release_manifest_sha256 || m.schema_version !== 3 || m.status !== "CANDIDATE" ||
      m.product !== product || m.repository !== c.REPOSITORY || m.version !== pkg.version ||
      m.tag !== (product === "agentplugins" ? `agentplugins-v${pkg.version}` : `v${pkg.version}`) ||
      m.commit !== d.identity.commit || m.engine_revision !== d.identity.engine_revision || !equal(m.versions, d.identity.versions) ||
      m.candidate_sha256 !== d.candidate_sha256 || m.authoring_mode !== MODE || m.asset_scope !== SCOPE ||
      m.release_eligible !== false || m.platform_acceptance !== false || m.attested !== false) {
    throw new Error("public producer projection mismatch");
  }
  c.keys(m.assets, c.TARGETS, "six public targets");
  const seen = new Set();
  for (const key of c.TARGETS) {
    const a = m.assets[key];
    c.keys(a, ["file", "sha256", "size", "binary"], "public asset");
    c.keys(a.binary, ["file", "sha256", "size"], "public binary");
    pin(a); pin(a.binary);
    if (a.file !== c.assetName(product, pkg.version, key) || a.binary.file !== c.executableName(product, key) ||
        (product === "agentplugins" && (a.sha256 !== a.binary.sha256 || a.size !== a.binary.size)) || seen.has(a.binary.sha256)) {
      throw new Error("public asset filename/raw pin/uniqueness mismatch");
    }
    seen.add(a.binary.sha256);
  }
  if (v2) {
    const input = contract.decodeInputs(inputBytes);
    for (const peer of c.PRODUCTS) {
      const projection = contract.projectionBytes(input, peer), pins = input.products[peer];
      if (c.digest(projection.manifest) !== pins.manifest_sha256 || c.digest(projection.checksums) !== pins.checksums_sha256 ||
          (peer === product && !bytes.equals(projection.manifest))) throw new Error("public pair projection mismatch");
    }
  } else qualification(d); // Always before effects, including warm cache lookup.
  return { descriptor: d, manifest: m, asset: m.assets[target], version: pkg.version,
    binding: c.digest(descriptorBytes), tag: m.tag };
}

// Bounded schema dispatch lives in the existing shipped runtime so v1 packages
// do not import either new helper. Actual v1 validation retains its old reader.
function descriptorSchema(file) {
  const named = fs.lstatSync(file);
  if (!named.isFile() || named.isSymbolicLink() || named.nlink !== 1 || named.size <= 0 || named.size > 1024 * 1024) {
    throw new Error("invalid bounded public descriptor");
  }
  const fd = fs.openSync(file, fs.constants.O_RDONLY | (fs.constants.O_NOFOLLOW || 0));
  let primary, schema;
  try {
    const bytes = Buffer.alloc(named.size + 1);
    let offset = 0, count;
    while (offset < bytes.length) {
      const length = Math.min(65536, bytes.length - offset);
      count = fs.readSync(fd, bytes, offset, length, offset);
      if (!Number.isSafeInteger(count) || count < 0 || count > length) throw new Error("invalid bounded descriptor read");
      if (count === 0) break;
      offset += count;
    }
    const opened = fs.fstatSync(fd);
    if (offset !== named.size || ["dev", "ino", "size", "mode", "nlink", "uid", "gid", "mtimeMs", "ctimeMs"].some(k => named[k] !== opened[k])) {
      throw new Error("public descriptor changed");
    }
    schema = JSON.parse(bytes.subarray(0, offset)).schema;
  } catch (error) { primary = error; }
  try { fs.closeSync(fd); } catch (error) { throw v.failures(primary, [error]); }
  if (primary) throw primary;
  return schema;
}

function snapshotRelease(product, packageRoot, target, signal) {
  if (!Object.hasOwn(PACKAGES, product) || !c.TARGETS.includes(target)) throw new Error("unsupported public product/target");
  c.safeDirectory(packageRoot);
  const schema = descriptorSchema(path.join(packageRoot, "public-release.json"));
  if (schema === SCHEMA) {
    const release = validateRelease(product, packageRoot, target);
    return { release, close() {}, recheck() {
      if (validateRelease(product, packageRoot, target).binding !== release.binding) throw new Error("public release changed during acquisition");
    } };
  }
  if (schema !== "dual-authoring-public-npm/v2") throw new Error("unsupported public release schema");
  const { snapshotPublicFile } = require("./public-authoring-input");
  const contract = require("./public-authoring-contract"), snapshots = new Map();
  function close() {
    const errors = [];
    for (const snapshot of snapshots.values()) try { snapshot.close(); } catch (error) { errors.push(error); }
    if (errors.length) throw v.failures(null, errors);
  }
  try {
    for (const [name, maximum] of [["public-release.json", contract.MAX_DESCRIPTOR_BYTES], [contract.INPUT_FILE, contract.MAX_INPUT_BYTES],
      ["release-manifest.json", 1024 * 1024], ["package.json", 1024 * 1024]]) {
      snapshots.set(path.join(packageRoot, name), snapshotPublicFile(path.join(packageRoot, name), { kind: "metadata", maximum, signal }));
    }
    const read = file => {
      const bytes = snapshots.get(file).bytes, value = JSON.parse(bytes);
      if (!bytes.equals(c.encode(value))) throw new Error("noncanonical public JSON");
      return { bytes, value };
    };
    const release = validateRelease(product, packageRoot, target, read, snapshots.get(path.join(packageRoot, contract.INPUT_FILE)).bytes);
    const recheck = () => { for (const snapshot of snapshots.values()) snapshot.recheck(); };
    recheck();
    return { release, close, recheck };
  } catch (primary) {
    try { close(); } catch (error) { throw v.failures(primary, [error]); }
    throw primary;
  }
}

function loadRelease(product, packageRoot, target) {
  const snapshot = snapshotRelease(product, packageRoot, target);
  snapshot.close();
  return snapshot.release;
}
const releaseNamespace = release => release.descriptor.schema === SCHEMA ? "public-authoring-v1" : "public-authoring-v2";

function cachePath(root, product, target, release) {
  return path.join(root, releaseNamespace(release), MODE, release.descriptor.identity.commit,
    release.descriptor.candidate_sha256, product, release.version, target, release.asset.binary.sha256, release.asset.binary.file);
}

// Create only missing directories; never chmod an existing historical cache.
// PR167 validates every ancestor before the next child can be created.
async function namespace(root, io, name = "public-authoring-v1") {
  if (typeof root !== "string" || !path.isAbsolute(root) || path.resolve(root) !== root ||
      root.includes("\0") || root.split(/[\\/]/).includes("..")) throw new Error("unsafe public cache root");
  let current = path.parse(root).root;
  for (const part of root.slice(current.length).split(path.sep).filter(Boolean)) {
    current = path.join(current, part);
    try { await io.mkdir(current, { mode: 0o700 }); }
    catch (e) { if (e.code !== "EEXIST") throw e; }
    const stat = await io.lstat(current);
    const uid = typeof process.geteuid === "function" ? process.geteuid() : null;
    if (!stat.isDirectory() || stat.isSymbolicLink() ||
        (uid !== null && stat.uid !== uid && stat.uid !== 0) ||
        (process.platform !== "win32" && (stat.mode & 0o022) && !(stat.mode & 0o1000))) {
      throw new Error("public cache requires safe ancestors");
    }
  }
  const owned = path.join(root, name);
  try { await io.mkdir(owned, { mode: 0o700 }); }
  catch (e) { if (e.code !== "EEXIST") throw e; }
  await v.privateDirectory(owned, owned, io);
  return owned;
}

function childEnvironment(environment = process.env) {
  return Object.fromEntries(Object.entries(environment).filter(([key]) =>
    !/^(AGENTPLUGINS_INTERNAL_PROOF_|UAP_PRIVATE_NPM_|UAP_PUBLIC_AUTHORING_|PLUGIN_KIT_AI_(VERSION|REPOSITORY|RELEASE_BASE_URL)$|NODE_OPTIONS$)/.test(key)));
}

async function ensureBinary(product, options = {}) {
  const platform = require("./platform").detectPlatform(options.platform, options.arch);
  const target = `${platform.osName}-${platform.archName}`;
  if (options.target !== undefined && options.target !== target) throw new Error("unexpected public native target");
  const packageRoot = options.packageRoot || path.resolve(__dirname, "..");
  v.cancelled(options.signal);
  const snapshot = snapshotRelease(product, packageRoot, target, options.signal);
  let assetSnapshot, primary, result;
  try {
    const release = snapshot.release;
    const root = options.cacheRoot || path.join(os.homedir(), ".cache", "universal-agent-plugins");
    let placement, localBinary;
    if (release.descriptor.schema !== SCHEMA) {
      const input = require("./public-authoring-input");
      placement = input.publicPlacement(packageRoot, root);
      const environment = options.environment || process.env;
      if (Object.hasOwn(environment, "UAP_PUBLIC_AUTHORING_ASSET_FILE")) {
        const file = environment.UAP_PUBLIC_AUTHORING_ASSET_FILE;
        if (typeof file !== "string" || !file || path.basename(file) !== release.asset.file) throw new Error("invalid public asset locator basename");
        assetSnapshot = input.snapshotPublicFile(file, { kind: "local asset", maximum: release.asset.size,
          exactSize: release.asset.size, protectedRoots: [packageRoot, root], signal: options.signal });
        localBinary = checkedBinary(product, assetSnapshot.bytes, release.asset);
      }
    }
    const recheck = () => { snapshot.recheck(); if (placement) placement.recheck(); if (assetSnapshot) assetSnapshot.recheck(); };
    if (release.descriptor.schema !== SCHEMA) recheck();
    result = await acquireBinary(product, options, platform, target, packageRoot, release, localBinary, recheck);
  } catch (error) { primary = error; }
  const errors = [];
  for (const held of [assetSnapshot, snapshot]) if (held) try { held.close(); } catch (error) { errors.push(error); }
  if (primary || errors.length) throw v.failures(primary, errors);
  return result;
}

function checkedBinary(product, bytes, asset) {
  if (bytes.length !== asset.size || c.digest(bytes) !== asset.sha256) throw new Error("outer public asset mismatch");
  const binary = product === "plugin-kit-ai" ? c.unpack(bytes, asset.binary.file) : bytes;
  if (!Buffer.isBuffer(binary) || binary.length !== asset.binary.size || c.digest(binary) !== asset.binary.sha256) throw new Error("inner public binary mismatch");
  return Buffer.from(binary);
}

async function acquireBinary(product, options, platform, target, packageRoot, release, localBinary, recheck) {
  // No historical repository/version/cache override is consulted on this path.
  const root = options.cacheRoot || path.join(os.homedir(), ".cache", "universal-agent-plugins");
  if (inside(root, packageRoot) || inside(packageRoot, root)) throw new Error("public cache overlaps package");
  const io = options.io || fsp;
  const owned = await namespace(root, io, releaseNamespace(release));
  const binaryPath = cachePath(root, product, target, release);
  await v.privateDirectory(owned, path.dirname(binaryPath), io);
  const lockRoot = path.join(owned, ".locks");
  await v.privateDirectory(owned, lockRoot, io);
  const settings = { io, signal: options.signal, strict: true, osName: platform.osName };
  const directoryPins = await Promise.all([owned, path.dirname(binaryPath)].map(async name => [name, await io.lstat(name)]));
  const unchangedDirectories = async () => {
    await v.privateDirectory(owned, path.dirname(binaryPath), io);
    for (const [name, pin] of directoryPins) {
      const named = await io.lstat(name);
      if (named.dev !== pin.dev || named.ino !== pin.ino) throw new Error("public cache directory replaced");
    }
  };
  const unlock = await v.acquireLock(binaryPath, { ...options.lockOptions, io, signal: options.signal, lockRoot });
  let primary, result, temporary, temporaryStat, downloadStat;
  const errors = [];
  try {
    await unchangedDirectories();
    const hit = await v.strictCachedBinary(binaryPath, release.asset.binary, settings);
    if (!hit) {
      let binary = localBinary;
      if (!binary) {
        temporary = await io.mkdtemp(path.join(owned, ".download-"));
        temporaryStat = await io.lstat(temporary);
        await v.privateDirectory(owned, temporary, io);
        const file = path.join(temporary, "asset");
        await v.downloadFile(`https://github.com/${c.REPOSITORY}/releases/download/${release.tag}/${release.asset.file}`,
          file, release.asset, { request: options.request, signal: options.signal, onOpen: stat => { downloadStat = stat; } });
        v.cancelled(options.signal);
        const bytes = c.readFile(file);
        binary = checkedBinary(product, bytes, release.asset);
      }
      if (release.descriptor.schema !== SCHEMA) recheck();
      await unchangedDirectories();
      await v.commitVerifiedBinary(binary, binaryPath, release.asset.binary, settings);
    }
    v.cancelled(options.signal);
    // A concurrent metadata edit cannot turn this operation into a warm bypass.
    recheck();
    await unchangedDirectories();
    if (release.descriptor.schema !== SCHEMA && !await v.strictCachedBinary(binaryPath, release.asset.binary, settings)) {
      throw new Error("final public binary verification failed");
    }
    if (release.descriptor.schema !== SCHEMA) recheck();
    result = { binaryPath, version: release.version, tag: release.tag, product, cacheHit: hit,
      publicAuthoring: true, repository: c.REPOSITORY };
  } catch (e) { primary = e; }
  if (temporary) {
    try {
      const named = await io.lstat(temporary);
      if (named.ino !== temporaryStat.ino || named.dev !== temporaryStat.dev || !named.isDirectory()) throw new Error("download directory replaced");
      if (downloadStat) {
        const file = path.join(temporary, "asset"), namedFile = await io.lstat(file);
        if (!namedFile.isFile() || namedFile.nlink !== 1 || namedFile.ino !== downloadStat.ino || namedFile.dev !== downloadStat.dev) throw new Error("download cleanup identity changed");
        await io.unlink(file);
      }
      await io.rmdir(temporary);
    } catch (e) { errors.push(e); }
  }
  await unlock().catch(e => errors.push(e));
  if (primary || errors.length) throw v.failures(primary, errors);
  return result;
}
module.exports = { SCHEMA, MODE, SCOPE, PACKAGES, loadRelease, cachePath, ensureBinary, childEnvironment };
