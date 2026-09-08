"use strict";

// Package consistency, not a signature verifier. The future protected public
// stager must authenticate the signed pair before emitting a binding. Preparation
// emits null. No runtime environment/CLI option can qualify a preparation pack.
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

function loadRelease(product, packageRoot, target) {
  if (!Object.hasOwn(PACKAGES, product) || !c.TARGETS.includes(target)) throw new Error("unsupported public product/target");
  c.safeDirectory(packageRoot);
  const { value: d, bytes: descriptorBytes } = json(path.join(packageRoot, "public-release.json"));
  c.keys(d, ["schema", "product", "npm_package", "identity", "authoring_mode", "asset_scope",
    "candidate_sha256", "release_manifest_sha256", "qualification"], "public release");
  c.identity(d.identity);
  if (d.schema !== SCHEMA || d.product !== product || d.npm_package !== PACKAGES[product] ||
      /^0{40}$/.test(d.identity.commit) || d.identity.versions["plugin-kit-ai"] !== "2.0.0" ||
      d.authoring_mode !== MODE || d.asset_scope !== SCOPE || !hash(d.candidate_sha256) || !hash(d.release_manifest_sha256)) {
    throw new Error("public release identity mismatch");
  }
  const { value: pkg } = json(path.join(packageRoot, "package.json"));
  c.keys(pkg, ["name", "version", "description", "license", "homepage", "repository", "bugs", "keywords",
    "engines", "publishConfig", "files", "bin", "scripts", "private", ...(product === "agentplugins" ? ["os", "cpu"] : [])], "public package");
  const closure = ["LICENSE", "README.md", "package.json", `bin/${product}.js`, "bin/package.json", "lib/package.json",
    "lib/platform.js", "lib/verifier.js", "lib/public-authoring.js", "scripts/package.json", "scripts/dual-authoring-candidate.js",
    product === "agentplugins" ? "lib/bootstrap.js" : "lib/install.js", "public-release.json", "release-manifest.json"];
  if (typeof pkg.private !== "boolean" || !Array.isArray(pkg.files) || !equal(pkg.files, closure.sort())) {
    throw new Error("public package closure mismatch");
  }
  c.keys(pkg.bin, [product], "public npm bin");
  c.keys(pkg.engines, ["node"], "public npm engines");
  const scripts = product === "agentplugins" ? { test: "node --test" } : { postinstall: "node ./lib/install.js" };
  if (pkg.name !== PACKAGES[product] || pkg.version !== d.identity.versions[product] ||
      pkg.bin[product] !== `bin/${product}.js` || pkg.engines.node !== (product === "agentplugins" ? ">=22" : ">=18") ||
      !equal(pkg.scripts, scripts)) throw new Error("public npm package binding mismatch");
  const { value: m, bytes } = json(path.join(packageRoot, "release-manifest.json"));
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
  qualification(d); // Always before effects, including warm cache lookup.
  return { descriptor: d, manifest: m, asset: m.assets[target], version: pkg.version,
    binding: c.digest(descriptorBytes), tag: m.tag };
}

function cachePath(root, product, target, release) {
  return path.join(root, "public-authoring-v1", MODE, release.descriptor.identity.commit,
    release.descriptor.candidate_sha256, product, release.version, target, release.asset.binary.sha256, release.asset.binary.file);
}

// Create only missing directories; never chmod an existing historical cache.
// PR167 validates every ancestor before the next child can be created.
async function namespace(root, io) {
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
  const owned = path.join(root, "public-authoring-v1");
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
  const release = loadRelease(product, packageRoot, target);
  // No historical repository/version/cache override is consulted on this path.
  const root = options.cacheRoot || path.join(os.homedir(), ".cache", "universal-agent-plugins");
  if (inside(root, packageRoot) || inside(packageRoot, root)) throw new Error("public cache overlaps package");
  const io = options.io || fsp;
  const owned = await namespace(root, io);
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
      temporary = await io.mkdtemp(path.join(owned, ".download-"));
      temporaryStat = await io.lstat(temporary);
      await v.privateDirectory(owned, temporary, io);
      const file = path.join(temporary, "asset");
      await v.downloadFile(`https://github.com/${c.REPOSITORY}/releases/download/${release.tag}/${release.asset.file}`,
        file, release.asset, { request: options.request, signal: options.signal, onOpen: stat => { downloadStat = stat; } });
      v.cancelled(options.signal);
      const bytes = c.readFile(file);
      if (bytes.length !== release.asset.size || c.digest(bytes) !== release.asset.sha256) throw new Error("outer public asset mismatch");
      const binary = product === "plugin-kit-ai" ? c.unpack(bytes, release.asset.binary.file) : bytes;
      if (binary.length !== release.asset.binary.size || c.digest(binary) !== release.asset.binary.sha256) throw new Error("inner public binary mismatch");
      await unchangedDirectories();
      await v.commitVerifiedBinary(binary, binaryPath, release.asset.binary, settings);
    }
    v.cancelled(options.signal);
    // A concurrent metadata edit cannot turn this operation into a warm bypass.
    if (loadRelease(product, packageRoot, target).binding !== release.binding) throw new Error("public release changed during acquisition");
    await unchangedDirectories();
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
