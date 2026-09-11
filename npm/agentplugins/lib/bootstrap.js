"use strict";

const crypto = require("node:crypto");
const fs = require("node:fs");
const fsp = require("node:fs/promises");
const os = require("node:os");
const path = require("node:path");

const { cacheRoot, detectPlatform, expectedAssetName } = require("./platform");

const verifier = require("./verifier");
const { sha256File, validCachedBinary, downloadFile } = verifier;

function acquireLock(target, options = {}) {
  const lockRoot = options.lockRoot || path.join(cacheRoot(
    options.environment || process.env, options.platform || process.platform,
    options.home || os.homedir()
  ), ".locks");
  return verifier.acquireLock(target, { ...options, lockRoot });
}

function installVerifiedBinary(downloaded, binaryPath, release, platformInfo, lockRoot) {
  return verifier.installVerifiedBinary(downloaded, binaryPath, release.asset, {
    osName: platformInfo.osName, lockRoot
  });
}

const VERSION = /^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$/;
const DIGEST = /^[0-9a-f]{64}$/;
const PROOF_MODE = "local-frozen-release-asset-v1";
const PRODUCER_REPOSITORY = "777genius/plugin-kit-ai";
const HISTORICAL_EVIDENCE_COMMIT = "4b25a45e1574bab7a4f49e48905a3b3b2647e917";
const HISTORICAL_EVIDENCE_DOCUMENT_SHA256 = "df6769bf430a337f116cd9df75bcc3ea26df166a016eacf9bc9fbc6cfbf9b100";
const HISTORICAL_EVIDENCE_RECORD_SHA256 = "437da1bc7423a85b231be139ff9bfbd7e89c942ef216a61ebde668c08a9c2ee3";
const COMMIT = /^[0-9a-f]{40}$/;

function exactKeys(value, keys) {
  return value && typeof value === "object" && !Array.isArray(value) &&
    Object.keys(value).sort().join(",") === [...keys].sort().join(",");
}

function loadRelease(packageRoot, platformInfo) {
  const pkg = JSON.parse(fs.readFileSync(path.join(packageRoot, "package.json"), "utf8"));
  const manifest = JSON.parse(fs.readFileSync(path.join(packageRoot, "assets.json"), "utf8"));
  const version = String(pkg.version || "").trim();
  if (!VERSION.test(version) || version === "0.0.0-development") {
    throw new Error("this development npm package has no released binary; use an exact published version");
  }
  if (manifest.schema_version !== 2 || manifest.version !== version) {
    throw new Error("npm version and embedded binary manifest do not match");
  }
  if (manifest.npm_package !== pkg.name) {
    throw new Error("npm package name and embedded binary manifest do not match");
  }
  if (!/^[A-Za-z0-9_.-]+\/[A-Za-z0-9_.-]+$/.test(String(manifest.repository || ""))) {
    throw new Error("embedded binary repository is invalid");
  }
  if (manifest.tag !== `agentplugins-v${version}`) {
    throw new Error("embedded release tag does not match npm version");
  }
  const producer = manifest.producer;
  const releaseManifest = producer && producer.release_manifest;
  if (!exactKeys(producer, ["repository", "tag", "commit", "release_manifest"]) ||
      producer.repository !== PRODUCER_REPOSITORY || producer.repository !== manifest.repository ||
      producer.tag !== manifest.tag || !COMMIT.test(String(producer.commit || "")) ||
      !exactKeys(releaseManifest, ["schema_version", "sha256", "version"]) ||
      releaseManifest.schema_version !== 2 || releaseManifest.version !== version ||
      !DIGEST.test(String(releaseManifest.sha256 || ""))) {
    throw new Error("embedded producer release identity is invalid or incomplete");
  }
  const evidence = manifest.client_evidence;
  const installer = evidence && evidence.installer;
  const evidencePackage = evidence && evidence.package;
  const source = evidence && evidence.source;
  const claims = evidence && evidence.claim_boundary;
  const claimKeys = ["lifecycle_e2e", "client_discovery_e2e", "browser_tool_runtime_e2e", "model_turn_e2e", "login_e2e", "oauth_e2e", "windsurf_skill_activation_claimed"];
  if (!exactKeys(evidence, ["schema_version", "kind", "recorded_at", "document_sha256", "record_sha256", "source", "installer", "package", "claim_boundary"]) ||
      evidence.schema_version !== 1 || evidence.kind !== "agentplugins_client_lifecycle" ||
      !/^\d{4}-\d{2}-\d{2}$/.test(String(evidence.recorded_at || "")) ||
      evidence.document_sha256 !== HISTORICAL_EVIDENCE_DOCUMENT_SHA256 || evidence.record_sha256 !== HISTORICAL_EVIDENCE_RECORD_SHA256 ||
      !exactKeys(source, ["repository", "commit", "document", "record"]) || source.repository !== PRODUCER_REPOSITORY || source.commit !== HISTORICAL_EVIDENCE_COMMIT ||
      !exactKeys(source.document, ["path", "sha256"]) || source.document.path !== "docs/AGENTPLUGINS_CLIENT_E2E.md" || source.document.sha256 !== evidence.document_sha256 ||
      !exactKeys(source.record, ["path", "sha256"]) || source.record.path !== "docs/evidence/agentplugins-client-e2e-2026-08-30.json" || source.record.sha256 !== evidence.record_sha256 ||
      !exactKeys(installer, ["repository", "commit", "tree", "version", "binary_sha256"]) ||
      installer.repository !== PRODUCER_REPOSITORY || !COMMIT.test(String(installer.commit || "")) ||
      !COMMIT.test(String(installer.tree || "")) || !/^\d+\.\d+\.\d+$/.test(String(installer.version || "")) ||
      !DIGEST.test(String(installer.binary_sha256 || "")) ||
      !exactKeys(evidencePackage, ["selector", "revision", "tree_digest", "manifest_digest"]) ||
      !COMMIT.test(String(evidencePackage.revision || "")) ||
      !/^[A-Za-z0-9_.-]+\/[A-Za-z0-9_.-]+@[0-9a-f]{40}$/.test(String(evidencePackage.selector || "")) ||
      !String(evidencePackage.selector).endsWith(`@${evidencePackage.revision}`) ||
      !/^sha256:[0-9a-f]{64}$/.test(String(evidencePackage.tree_digest || "")) ||
      !/^sha256:[0-9a-f]{64}$/.test(String(evidencePackage.manifest_digest || "")) ||
      !exactKeys(claims, claimKeys) || claims.lifecycle_e2e !== true || claims.client_discovery_e2e !== true ||
      claimKeys.slice(2).some((key) => claims[key] !== false)) {
    throw new Error("embedded historical client evidence identity is invalid or incomplete");
  }
  const asset = manifest.assets && manifest.assets[platformInfo.key];
  if (!asset || asset.file !== expectedAssetName(version, platformInfo) || !DIGEST.test(String(asset.sha256 || "")) || !Number.isSafeInteger(asset.size) || asset.size <= 0) {
    throw new Error(`npm package does not contain a valid binary pin for ${platformInfo.key}`);
  }
  if (path.basename(asset.file) !== asset.file) {
    throw new Error("embedded binary asset name is unsafe");
  }
  return { asset, manifest, version };
}

function localProofAsset(environment = {}) {
  const mode = String(environment.AGENTPLUGINS_INTERNAL_PROOF_MODE || "").trim();
  const file = String(environment.AGENTPLUGINS_INTERNAL_PROOF_BINARY || "").trim();
  if (!mode && !file) return "";
  if (mode !== PROOF_MODE || !file || !path.isAbsolute(file)) {
    throw new Error("internal proof bootstrap requires an absolute frozen release asset and exact proof mode");
  }
  return file;
}

async function verifyLocalProofAsset(file, expected) {
  const stat = await fsp.lstat(file);
  if (!stat.isFile() || stat.isSymbolicLink() || stat.size !== expected.size ||
      await sha256File(file) !== expected.sha256) {
    throw new Error("internal proof release asset does not match embedded size and SHA-256 metadata");
  }
}

async function ensureBinary(options = {}) {
  const packageRoot = options.packageRoot || path.resolve(__dirname, "..");
  // lstat detects dangling descriptors too; malformed new metadata never falls back.
  try {
    fs.lstatSync(path.join(packageRoot, "public-release.json"));
    return require("./public-authoring").ensureBinary("agentplugins", { ...options, packageRoot });
  } catch (error) { if (error.code !== "ENOENT") throw error; }
  const platformInfo = detectPlatform(options.platform, options.arch);
  const release = loadRelease(packageRoot, platformInfo);
  const root = options.cacheRoot || cacheRoot(options.environment, options.platform);
  const lockRoot = path.join(root, ".locks");
  const binaryPath = path.join(root, release.version, platformInfo.key, platformInfo.binaryName);
  if (await validCachedBinary(binaryPath, release.asset.sha256)) {
    return { binaryPath, version: release.version, cacheHit: true };
  }
  const proofAsset = localProofAsset(options.environment || process.env);
  if (proofAsset) {
    try {
      if (path.basename(proofAsset) !== release.asset.file) {
        throw new Error("internal proof release asset filename does not match embedded metadata");
      }
      await verifyLocalProofAsset(proofAsset, release.asset);
      await installVerifiedBinary(proofAsset, binaryPath, release, platformInfo, lockRoot);
      return { binaryPath, version: release.version, cacheHit: false, source: "local_frozen_asset" };
    } catch (error) {
      const cold = !(await validCachedBinary(binaryPath, release.asset.sha256));
      const suffix = cold ? " No client or plugin files were changed." : "";
      throw new Error(`unable to obtain agentplugins ${release.version}: ${error.message}.${suffix}`);
    }
  }
  const releaseBase = `https://github.com/${release.manifest.repository}/releases/download/${release.manifest.tag}`;
  const url = `${String(releaseBase).replace(/\/$/, "")}/${encodeURIComponent(release.asset.file)}`;
  const temporary = path.join(os.tmpdir(), `agentplugins-download-${process.pid}-${crypto.randomBytes(8).toString("hex")}`);
  try {
    await downloadFile(url, temporary, release.asset, options);
    await installVerifiedBinary(temporary, binaryPath, release, platformInfo, lockRoot);
    return { binaryPath, version: release.version, cacheHit: false };
  } catch (error) {
    const cold = !(await validCachedBinary(binaryPath, release.asset.sha256));
    const suffix = cold ? " No client or plugin files were changed." : "";
    throw new Error(`unable to obtain agentplugins ${release.version}: ${error.message}.${suffix}`);
  } finally {
    await fsp.rm(temporary, { force: true });
  }
}

function formatBootstrapError(error, version = "this exact version") {
  return [
    `agentplugins npm launcher: ${error.message}`,
    `The launcher never falls back to latest. Check network access or install ${version} from the matching GitHub release.`
  ].join(os.EOL);
}

module.exports = {
  PROOF_MODE,
  acquireLock,
  downloadFile,
  ensureBinary,
  formatBootstrapError,
  loadRelease,
  localProofAsset,
  sha256File,
  validCachedBinary
};
