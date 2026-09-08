#!/usr/bin/env node
"use strict";

const crypto = require("node:crypto");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");

const { detectPlatform, expectedAssetName } = require("../lib/platform");
const { PRODUCER_REPOSITORY, verifyRelease } = require("./release-assets");

const VERSION = /^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$/;
const NPM_PACKAGE_NAME = /^(?:@[a-z0-9][a-z0-9._-]*\/)?[a-z0-9][a-z0-9._-]*$/;
const COMMIT = /^[0-9a-f]{40}$/;
const DIGEST = /^[0-9a-f]{64}$/;
const PLATFORMS = [
  ["darwin", "x64"],
  ["darwin", "arm64"],
  ["linux", "x64"],
  ["linux", "arm64"],
  ["win32", "x64"],
  ["win32", "arm64"]
];
const EVIDENCE_FILES = [
  "AGENTPLUGINS_CLIENT_E2E.md",
  path.join("evidence", "agentplugins-client-e2e-2026-08-30.json")
];
// Pins the source documents, not a rerun or the historical installer commit.
const EVIDENCE_SOURCE = {
  repository: PRODUCER_REPOSITORY,
  commit: "01f02cb51cfe5f664d4d5f52b295c59c7ea03495",
  document_path: "docs/AGENTPLUGINS_CLIENT_E2E.md",
  document_sha256: "0cecbf12a96cf578d12e1cb2f582aa13dca2d9d900ccaefd223f5d7c1030ea65",
  record_path: "docs/evidence/agentplugins-client-e2e-2026-08-30.json",
  record_sha256: "437da1bc7423a85b231be139ff9bfbd7e89c942ef216a61ebde668c08a9c2ee3"
};

function validatePackageMetadata(pkg) {
  if (!pkg || typeof pkg !== "object" || typeof pkg.name !== "string" ||
      !NPM_PACKAGE_NAME.test(pkg.name) || pkg.name.length > 214) {
    throw new Error("staged npm package name is invalid");
  }
  const packageName = pkg.name;
  if (!pkg.bin || pkg.bin.agentplugins !== "bin/agentplugins.js") {
    throw new Error("staged npm package must expose the agentplugins binary");
  }
  return packageName;
}

function requireSafeStagingFile(file, label, allowMissing = false) {
  if (allowMissing && !fs.existsSync(file)) return;
  const stat = fs.lstatSync(file);
  if (!stat.isFile() || stat.isSymbolicLink() || stat.nlink !== 1) {
    throw new Error(`${label} must be a real, unaliased file`);
  }
}

function exactKeys(value, keys) {
  return value && typeof value === "object" && !Array.isArray(value) &&
    Object.keys(value).sort().join(",") === [...keys].sort().join(",");
}

function requireExactKeys(value, keys, label) {
  if (!exactKeys(value, keys)) throw new Error(`client E2E evidence ${label} keys are invalid`);
}

function requireStringArray(value, label) {
  if (!Array.isArray(value) || value.length === 0 || value.some((entry) => typeof entry !== "string" || !entry)) {
    throw new Error(`client E2E evidence ${label} is invalid`);
  }
}

function requireExactArgv(value, expected, label) {
  requireStringArray(value, label);
  if (value.join("\0") !== expected.join("\0")) {
    throw new Error(`client E2E evidence ${label} command identity is invalid`);
  }
}

function validateEvidenceRecord(body) {
  let data;
  try {
    data = JSON.parse(body);
  } catch (error) {
    throw new Error(`client E2E evidence JSON is malformed: ${error.message}`);
  }
  requireExactKeys(data, ["schema_version", "evidence_kind", "recorded_at", "installer", "package", "environment", "transcript", "claim_boundary"], "record");
  if (data.schema_version !== 1 || data.evidence_kind !== "agentplugins_client_lifecycle" ||
      !/^\d{4}-\d{2}-\d{2}$/.test(String(data.recorded_at || ""))) {
    throw new Error("client E2E evidence schema or identity is invalid");
  }
  const installer = data.installer;
  requireExactKeys(installer, ["repository", "commit", "tree", "version", "binary_sha256"], "installer");
  if (installer.repository !== PRODUCER_REPOSITORY || !COMMIT.test(String(installer.commit || "")) ||
      !COMMIT.test(String(installer.tree || "")) || !/^\d+\.\d+\.\d+$/.test(String(installer.version || "")) ||
      !DIGEST.test(String(installer.binary_sha256 || ""))) {
    throw new Error("client E2E installer identity is invalid");
  }

  const pkg = data.package;
  requireExactKeys(pkg, ["selector", "repository", "revision", "name", "version", "tree_digest", "manifest_digest", "acquisition_closure_digest"], "package");
  if (!/^[A-Za-z0-9_.-]+\/[A-Za-z0-9_.-]+$/.test(String(pkg.repository || "")) ||
      !COMMIT.test(String(pkg.revision || "")) || pkg.selector !== `${pkg.repository}@${pkg.revision}` ||
      typeof pkg.name !== "string" || !pkg.name || typeof pkg.version !== "string" || !pkg.version ||
      !/^sha256:[0-9a-f]{64}$/.test(String(pkg.tree_digest || "")) ||
      !/^sha256:[0-9a-f]{64}$/.test(String(pkg.manifest_digest || "")) ||
      !/^sha256:[0-9a-f]{64}$/.test(String(pkg.acquisition_closure_digest || ""))) {
    throw new Error("client E2E package identity is invalid");
  }

  const environment = data.environment;
  requireExactKeys(environment, ["os", "arch", "node", "clients", "isolation"], "environment");
  if (typeof environment.os !== "string" || !environment.os || !["arm64", "amd64"].includes(environment.arch) ||
      !/^\d+\.\d+\.\d+$/.test(String(environment.node || ""))) {
    throw new Error("client E2E environment identity is invalid");
  }
  requireExactKeys(environment.clients, ["claude", "gemini", "opencode", "cline", "windsurf"], "client inventory");
  for (const client of Object.values(environment.clients)) {
    requireExactKeys(client, ["version", "host_surface_detected"], "client inventory entry");
    if ((client.version !== null && (typeof client.version !== "string" || !client.version)) || client.host_surface_detected !== true) {
      throw new Error("client E2E inventory did not detect every claimed host surface");
    }
  }
  const isolationKeys = ["fresh_home", "fresh_xdg_roots", "fresh_claude_config", "fresh_gemini_home", "fresh_cline_data", "fresh_agentplugins_state", "user_project_accessed", "user_client_config_mutated"];
  requireExactKeys(environment.isolation, isolationKeys, "isolation");
  if (isolationKeys.slice(0, 6).some((key) => environment.isolation[key] !== true) ||
      isolationKeys.slice(6).some((key) => environment.isolation[key] !== false)) {
    throw new Error("client E2E isolation or user-project boundary is invalid");
  }

  if (!Array.isArray(data.transcript) || data.transcript.length !== 7) {
    throw new Error("client E2E evidence must contain the exact lifecycle transcript");
  }
  const [add, discovery, doctor, preflight, repair, remove, postRemove] = data.transcript;
  const expectedSteps = ["add", "client_discovery", "doctor", "immutable_update_preflight", "repair", "remove", "post_remove"];
  if (data.transcript.some((entry, index) => !entry || entry.step !== expectedSteps[index])) {
    throw new Error("client E2E evidence lifecycle step identities or order are invalid");
  }
  requireExactKeys(add, ["step", "argv", "exit_code", "result", "status", "acquisition_count", "source_kind", "fetched", "validated", "targets", "shared_identity"], "add step");
  requireExactKeys(add.targets, ["claude", "gemini", "opencode", "cline", "windsurf"], "add targets");
  for (const target of Object.values(add.targets)) {
    requireExactKeys(target, ["outcome", "activation", "verification"], "add target");
    if (target.outcome !== "passed" || target.activation !== "active" || target.verification !== "installation_verified") {
      throw new Error("client E2E add target did not pass");
    }
  }
  requireExactKeys(add.shared_identity, ["installation_count", "physical_artifact_id", "same_tree_digest_for_all_targets", "same_manifest_digest_for_all_targets", "same_closure_digest_for_all_targets"], "shared identity");
  if (add.exit_code !== 0 || add.result !== "success" || add.status !== "completed" || add.acquisition_count !== 1 ||
      add.source_kind !== "github" || add.fetched !== true || add.validated !== true ||
      add.shared_identity.installation_count !== 1 || typeof add.shared_identity.physical_artifact_id !== "string" || !add.shared_identity.physical_artifact_id ||
      ["same_tree_digest_for_all_targets", "same_manifest_digest_for_all_targets", "same_closure_digest_for_all_targets"].some((key) => add.shared_identity[key] !== true)) {
    throw new Error("client E2E add step did not prove one validated shared acquisition");
  }

  requireExactKeys(discovery, ["step", "checks"], "client discovery step");
  requireExactKeys(discovery.checks, ["claude", "gemini_mcp", "gemini_skills", "opencode", "cline", "windsurf"], "client discovery checks");
  const discoveryKeys = {
    claude: ["argv", "exit_code", "plugin_id", "plugin_version", "enabled", "skill_count", "mcp_command"],
    gemini_mcp: ["argv", "exit_code", "server", "transport", "command", "connection"],
    gemini_skills: ["argv", "exit_code", "enabled_skill_count"],
    opencode: ["argv", "exit_code", "server", "type", "command", "cwd_bound_to_managed_package", "plugin_root_bound", "plugin_data_bound"],
    cline: ["config", "transport", "command", "plugin_root_bound", "plugin_data_bound", "projected_skill_count"],
    windsurf: ["config", "command", "plugin_root_bound", "plugin_data_bound", "skills"]
  };
  for (const [key, keys] of Object.entries(discoveryKeys)) {
    requireExactKeys(discovery.checks[key], keys, `${key} discovery check`);
  }
  requireExactArgv(discovery.checks.claude.argv, ["claude", "plugin", "list", "--json"], "claude discovery argv");
  requireExactArgv(discovery.checks.gemini_mcp.argv, ["gemini", "mcp", "list"], "gemini_mcp discovery argv");
  requireExactArgv(discovery.checks.gemini_skills.argv, ["gemini", "skills", "list"], "gemini_skills discovery argv");
  requireExactArgv(discovery.checks.opencode.argv, ["opencode", "debug", "config"], "opencode discovery argv");
  for (const key of ["claude", "gemini_mcp", "gemini_skills", "opencode"]) {
    if (discovery.checks[key].exit_code !== 0) throw new Error(`client E2E discovery check did not pass: ${key}`);
  }
  const nonEmptyCommand = (value) => Array.isArray(value) && value.length > 0 &&
    value.every((entry) => typeof entry === "string" && entry.length > 0);
  const expectedCommand = ["npx", `chrome-devtools-mcp@${data.package.version}`];
  const exactCommand = (value) => nonEmptyCommand(value) && value.join("\0") === expectedCommand.join("\0");
  const { claude, gemini_mcp: geminiMcp, gemini_skills: geminiSkills, opencode, cline, windsurf } = discovery.checks;
  if (claude.plugin_id !== "chrome-devtools@skills-dir" || claude.plugin_version !== data.package.version || claude.enabled !== true || claude.skill_count !== 6 || !exactCommand(claude.mcp_command) ||
      geminiMcp.server !== data.package.name || geminiMcp.transport !== "stdio" || !exactCommand(geminiMcp.command) || geminiMcp.connection !== "disconnected" ||
      geminiSkills.enabled_skill_count !== 6 ||
      opencode.server !== data.package.name || opencode.type !== "local" || !exactCommand(opencode.command) || opencode.cwd_bound_to_managed_package !== true || opencode.plugin_root_bound !== true || opencode.plugin_data_bound !== true ||
      cline.config !== `mcpServers.${data.package.name}` || cline.transport !== "stdio" || !exactCommand(cline.command) || cline.plugin_root_bound !== true || cline.plugin_data_bound !== true || cline.projected_skill_count !== 6 ||
      windsurf.config !== `mcpServers.${data.package.name}` || !exactCommand(windsurf.command) || windsurf.plugin_root_bound !== true || windsurf.plugin_data_bound !== true || windsurf.skills !== "prepared_only") {
    throw new Error("client E2E discovery semantic outcomes are incomplete");
  }

  requireExactKeys(doctor, ["step", "argv", "exit_code", "result", "read_only", "installation_count", "projection_drift_findings", "authentication_not_checked_clients"], "doctor step");
  requireExactKeys(preflight, ["step", "argv", "exit_code", "result", "status", "reason", "succeeded", "failed", "mutated_targets", "postcondition"], "immutable update step");
  requireExactKeys(repair, ["step", "argv", "exit_code", "result", "status", "succeeded", "failed", "targets"], "repair step");
  requireExactKeys(remove, ["step", "argv", "exit_code", "result", "status", "succeeded", "failed", "plugin_data_preserved", "targets"], "remove step");
  requireExactKeys(postRemove, ["step", "checks"], "post-remove step");
  const targets = "claude,gemini,opencode,cline,windsurf";
  requireExactArgv(add.argv, ["agentplugins", "add", data.package.selector, "--target", targets, "--format", "json"], "add argv");
  requireExactArgv(doctor.argv, ["agentplugins", "doctor", "--format", "json"], "doctor argv");
  requireExactArgv(preflight.argv, ["agentplugins", "update", data.package.name, "--target", targets, "--format", "json"], "immutable_update_preflight argv");
  requireExactArgv(repair.argv, ["agentplugins", "repair", data.package.name, "--target", targets, "--format", "json"], "repair argv");
  requireExactArgv(remove.argv, ["agentplugins", "remove", data.package.name, "--target", targets, "--format", "json"], "remove argv");
  requireExactKeys(repair.targets, ["claude", "gemini", "opencode", "cline", "windsurf"], "repair targets");
  requireExactKeys(remove.targets, ["claude", "gemini", "opencode", "cline", "windsurf"], "remove targets");
  requireExactKeys(postRemove.checks, ["agentplugins_installation_count", "claude_plugin_count", "gemini_mcp_count", "gemini_skill_count", "opencode_mcp_count", "cline_mcp_count", "cline_skill_count", "windsurf_mcp_count"], "post-remove checks");
  requireStringArray(doctor.authentication_not_checked_clients, "doctor authentication boundary");
  if (doctor.exit_code !== 0 || doctor.result !== "success" || doctor.read_only !== true || doctor.installation_count !== 1 || doctor.projection_drift_findings !== 0 ||
      doctor.authentication_not_checked_clients.join(",") !== targets ||
      preflight.exit_code !== 1 || preflight.result !== "failure" || preflight.status !== "preflight_failed" || preflight.reason !== "direct full-SHA installations require explicit switch" || preflight.succeeded !== 0 || preflight.failed !== 5 || preflight.mutated_targets !== 0 || preflight.postcondition !== "the exact installation and all five client projections remained installed and unchanged" ||
      repair.exit_code !== 0 || repair.result !== "success" || repair.status !== "completed" || repair.succeeded !== 5 || repair.failed !== 0 ||
      remove.exit_code !== 0 || remove.result !== "success" || remove.status !== "data_retained" || remove.succeeded !== 5 || remove.failed !== 0 || remove.plugin_data_preserved !== true ||
      Object.values(repair.targets).some((value) => value !== "passed") ||
      Object.values(remove.targets).some((value) => value !== "external_completed") ||
      Object.values(postRemove.checks).some((value) => value !== 0)) {
    throw new Error("client E2E evidence contains an unsuccessful lifecycle result");
  }

  const claimKeys = ["lifecycle_e2e", "client_discovery_e2e", "browser_tool_runtime_e2e", "model_turn_e2e", "login_e2e", "oauth_e2e", "windsurf_skill_activation_claimed"];
  requireExactKeys(data.claim_boundary, claimKeys, "claim boundary");
  if (data.claim_boundary.lifecycle_e2e !== true || data.claim_boundary.client_discovery_e2e !== true ||
      claimKeys.slice(2).some((key) => data.claim_boundary[key] !== false)) {
    throw new Error("client E2E claim boundary is invalid");
  }
  return data;
}

function loadEvidence(evidenceRoot) {
  if (!evidenceRoot) {
    throw new Error("release staging requires the checked-in client E2E evidence root");
  }
  if (!path.isAbsolute(evidenceRoot)) {
    throw new Error("client E2E evidence root must be absolute");
  }
  const rootStat = fs.lstatSync(evidenceRoot);
  if (!rootStat.isDirectory() || rootStat.isSymbolicLink()) {
    throw new Error("client E2E evidence root must be a real directory");
  }
  const resolvedRoot = fs.realpathSync(evidenceRoot);
  const bodies = {};
  const digests = {};
  for (const sourceName of EVIDENCE_FILES) {
    const source = path.join(evidenceRoot, sourceName);
    if (!fs.realpathSync(source).startsWith(`${resolvedRoot}${path.sep}`)) {
      throw new Error(`client E2E evidence escapes its exact root: ${sourceName}`);
    }
    const stat = fs.lstatSync(source);
    if (!stat.isFile() || stat.isSymbolicLink() || stat.size <= 0 || stat.nlink !== 1) {
      throw new Error(`client E2E evidence is not a regular non-empty file: ${sourceName}`);
    }
    bodies[sourceName] = fs.readFileSync(source);
    digests[sourceName] = crypto.createHash("sha256").update(bodies[sourceName]).digest("hex");
  }
  const recordName = EVIDENCE_FILES[1];
  const record = validateEvidenceRecord(bodies[recordName].toString("utf8"));
  const markdown = bodies[EVIDENCE_FILES[0]].toString("utf8");
  for (const value of [record.recorded_at, record.installer.commit, record.installer.tree, record.installer.version,
    record.installer.binary_sha256, record.package.selector, record.package.version, record.package.tree_digest,
    record.package.manifest_digest, recordName.split(path.sep).join("/"), digests[recordName]]) {
    if (!markdown.includes(value)) throw new Error("client E2E document does not bind the structured evidence identity and digest");
  }
  if (digests[EVIDENCE_FILES[0]] !== EVIDENCE_SOURCE.document_sha256 ||
      digests[recordName] !== EVIDENCE_SOURCE.record_sha256) {
    throw new Error("client E2E evidence bytes do not match the immutable source locator");
  }
  return {
    bodies,
    metadata: {
      schema_version: record.schema_version,
      kind: record.evidence_kind,
      recorded_at: record.recorded_at,
      document_sha256: digests[EVIDENCE_FILES[0]],
      record_sha256: digests[recordName],
      source: {
        repository: EVIDENCE_SOURCE.repository,
        commit: EVIDENCE_SOURCE.commit,
        document: { path: EVIDENCE_SOURCE.document_path, sha256: digests[EVIDENCE_FILES[0]] },
        record: { path: EVIDENCE_SOURCE.record_path, sha256: digests[recordName] }
      },
      installer: { ...record.installer },
      package: {
        selector: record.package.selector,
        revision: record.package.revision,
        tree_digest: record.package.tree_digest,
        manifest_digest: record.package.manifest_digest
      },
      claim_boundary: { ...record.claim_boundary }
    }
  };
}

function writeEvidence(packageRoot, loaded) {
  const destination = path.join(packageRoot, "test", "evidence-root");
  for (const sourceName of EVIDENCE_FILES) {
    const target = path.join(destination, sourceName);
    fs.mkdirSync(path.dirname(target), { recursive: true });
    fs.writeFileSync(target, loaded.bodies[sourceName], { flag: "wx" });
  }
  return loaded.metadata;
}

function stageEvidence(packageRoot, evidenceRoot) {
  return writeEvidence(packageRoot, loadEvidence(evidenceRoot));
}

function snapshotRelease(assetRoot, release, version, commit, options) {
  const snapshotRoot = fs.mkdtempSync(path.join(os.tmpdir(), "agentplugins-release-"));
  try {
    const names = [
      ...Object.values(release.assets).map((asset) => asset.file),
      "release-manifest.json",
      "checksums.txt"
    ];
    for (const name of names) {
      fs.copyFileSync(path.join(assetRoot, name), path.join(snapshotRoot, name), fs.constants.COPYFILE_EXCL);
    }
    return verifyRelease(snapshotRoot, `agentplugins-v${version}`, commit, options);
  } finally {
    fs.rmSync(snapshotRoot, { recursive: true, force: true });
  }
}

function stage(packageRoot, assetRoot, version, commit, options = {}) {
  if (!VERSION.test(version)) {
    throw new Error(`invalid release version: ${version}`);
  }
  const initialRelease = verifyRelease(assetRoot, `agentplugins-v${version}`, commit, options);
  if (!initialRelease.gate_eligible || initialRelease.manifest_schema !== 2) {
    throw new Error("release staging requires a gate-eligible schema-v2 current producer manifest");
  }
  const pkgPath = path.join(packageRoot, "package.json");
  const packageRootStat = fs.lstatSync(packageRoot);
  if (!packageRootStat.isDirectory() || packageRootStat.isSymbolicLink()) {
    throw new Error("staged npm package root must be a real directory");
  }
  requireSafeStagingFile(pkgPath, "staged npm package.json");
  requireSafeStagingFile(path.join(packageRoot, "assets.json"), "staged npm assets.json", true);
  const pkg = JSON.parse(fs.readFileSync(pkgPath, "utf8"));
  const packageName = validatePackageMetadata(pkg);
  const loadedEvidence = loadEvidence(options.evidenceRoot);
  if (options.afterInitialReleaseVerification) options.afterInitialReleaseVerification();
  const release = snapshotRelease(assetRoot, initialRelease, version, commit, options);
  if (JSON.stringify(release.assets) !== JSON.stringify(initialRelease.assets) ||
      release.manifest_sha256 !== initialRelease.manifest_sha256) {
    throw new Error("release directory changed during staging");
  }
  if (options.afterReleaseSnapshotVerification) options.afterReleaseSnapshotVerification();
  const assets = Object.fromEntries(PLATFORMS.map(([platform, arch]) => {
    const info = detectPlatform(platform, arch);
    const record = release.assets[info.key];
    if (!record || record.file !== expectedAssetName(version, info)) throw new Error(`verified release asset record is missing: ${info.key}`);
    return [info.key, { ...record }];
  }));
  const manifest = {
    schema_version: 2,
    version,
    npm_package: packageName,
    repository: "777genius/plugin-kit-ai",
    tag: `agentplugins-v${version}`,
    producer: {
      repository: release.repository,
      tag: release.tag,
      commit: release.commit,
      release_manifest: {
        schema_version: release.manifest_schema,
        sha256: release.manifest_sha256,
        version: release.version
      }
    },
    client_evidence: loadedEvidence.metadata,
    assets
  };
  pkg.version = version;
  writeEvidence(packageRoot, loadedEvidence);
  fs.writeFileSync(pkgPath, JSON.stringify(pkg, null, 2) + "\n");
  fs.writeFileSync(path.join(packageRoot, "assets.json"), JSON.stringify(manifest, null, 2) + "\n");
  return manifest;
}

function main() {
  const [packageRoot, assetRoot, version, commit, ...args] = process.argv.slice(2);
  if (!packageRoot || !assetRoot || !version || !commit) {
    throw new Error("usage: stage-release.js <package-root> <asset-root> <version> <commit> [allow-legacy-v1] --evidence-root <path>");
  }
  const allowLegacyManifest = args[0] === "allow-legacy-v1";
  const evidenceIndex = allowLegacyManifest ? 1 : 0;
  if (args[evidenceIndex] !== "--evidence-root" || !args[evidenceIndex + 1] || args.length !== evidenceIndex + 2) {
    throw new Error("release staging requires an exact [allow-legacy-v1] --evidence-root <path> argument set");
  }
  const manifest = stage(path.resolve(packageRoot), path.resolve(assetRoot), version, commit, {
    allowLegacyManifest,
    evidenceRoot: args[evidenceIndex + 1]
  });
  process.stdout.write(`Staged agentplugins ${manifest.version} with ${Object.keys(manifest.assets).length} pinned assets\n`);
}

if (require.main === module) {
  try {
    main();
  } catch (error) {
    process.stderr.write(`stage agentplugins npm release: ${error.message}\n`);
    process.exitCode = 1;
  }
}

module.exports = { stage, stageEvidence, validateEvidenceRecord, validatePackageMetadata };
