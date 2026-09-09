"use strict";

const assert = require("node:assert/strict");
const childProcess = require("node:child_process");
const crypto = require("node:crypto");
const fs = require("node:fs");
const fsp = require("node:fs/promises");
const os = require("node:os");
const path = require("node:path");
const test = require("node:test");

const { detectPlatform, expectedAssetName } = require("../lib/platform");
const { prepareRelease, verifyRelease } = require("../scripts/release-assets");
const { stage, stageEvidence, validateEvidenceRecord, validatePackageMetadata } = require("../scripts/stage-release");

const COMMIT = "a".repeat(40);
const HISTORICAL_COMMIT = "5630ccd92aa91c8ac8cafb37eea8752fd82edce0";
const HISTORICAL_TREE = "cf13cbe2f64ae09d93ad34bfc6047fe99d5ca845";

test("checked-in release evidence matches the pinned snapshot and preserves historical claims", async (t) => {
  const root = await fsp.mkdtemp(path.join(os.tmpdir(), "agentplugins-current-evidence-"));
  t.after(() => fsp.rm(root, { recursive: true, force: true }));
  const packageVersion = JSON.parse(await fsp.readFile(path.resolve(__dirname, "../package.json"), "utf8")).version;
  const evidenceRoot = packageVersion === "0.0.0-development"
    ? path.resolve(__dirname, "../../../docs")
    : path.join(__dirname, "evidence-root");
  const metadata = stageEvidence(root, evidenceRoot);
  assert.equal(metadata.source.commit, "01f02cb51cfe5f664d4d5f52b295c59c7ea03495");
  assert.equal(metadata.installer.commit, HISTORICAL_COMMIT);
  assert.equal(metadata.installer.version, "0.1.22");
  assert.equal(metadata.claim_boundary.model_turn_e2e, false);
  assert.equal(metadata.claim_boundary.oauth_e2e, false);
  const document = path.join(root, "test/evidence-root/AGENTPLUGINS_CLIENT_E2E.md");
  await fsp.appendFile(document, "\nUnreviewed change\n");
  assert.throws(() => stageEvidence(root, path.dirname(document)), /immutable source locator/);
});

async function fixtureEvidence(root) {
  const evidenceRoot = path.join(root, "evidence");
  await fsp.mkdir(path.join(evidenceRoot, "evidence"), { recursive: true });
  const record = {
    schema_version: 1,
    evidence_kind: "agentplugins_client_lifecycle",
    recorded_at: "2026-08-30",
    installer: {
      repository: "777genius/plugin-kit-ai",
      commit: HISTORICAL_COMMIT,
      tree: HISTORICAL_TREE,
      version: "0.1.22",
      binary_sha256: "8f417cea031d42b07badbe1b2a37dcd53deb2e5804f99d668f733309ecb4022b"
    },
    package: {
      selector: "ChromeDevTools/chrome-devtools-mcp@cb39d1d835c3baa3eff87501cd8c1de020604789",
      repository: "ChromeDevTools/chrome-devtools-mcp",
      revision: "cb39d1d835c3baa3eff87501cd8c1de020604789",
      name: "chrome-devtools",
      version: "1.8.0",
      tree_digest: "sha256:3bd47ccd3f990a6fdd8d3e2fa3dac48ac460a9043e0ccf0c5e14522fb4c472ea",
      manifest_digest: "sha256:b34a4dcd71cd536a7f5a3a51d76d53ae5af3d0ce0f18783e71c7f01da865b867",
      acquisition_closure_digest: "sha256:d06d41ea4cca87f5731aac72cd9c1bc46e280fd9d84107ed1dcbbbc6e05e02e9"
    },
    environment: {
      os: "macOS 15.6.1",
      arch: "arm64",
      node: "24.18.0",
      clients: {
        claude: { version: "2.1.205", host_surface_detected: true },
        gemini: { version: "0.36.0", host_surface_detected: true },
        opencode: { version: "1.18.4", host_surface_detected: true },
        cline: { version: null, host_surface_detected: true },
        windsurf: { version: null, host_surface_detected: true }
      },
      isolation: {
        fresh_home: true, fresh_xdg_roots: true, fresh_claude_config: true,
        fresh_gemini_home: true, fresh_cline_data: true, fresh_agentplugins_state: true,
        user_project_accessed: false, user_client_config_mutated: false
      }
    },
    transcript: [
      {
        step: "add", argv: ["agentplugins", "add"], exit_code: 0, result: "success", status: "completed",
        acquisition_count: 1, source_kind: "github", fetched: true, validated: true,
        targets: Object.fromEntries(["claude", "gemini", "opencode", "cline", "windsurf"].map((name) => [name,
          { outcome: "passed", activation: "active", verification: "installation_verified" }])),
        shared_identity: { installation_count: 1, physical_artifact_id: "chrome-devtools-f7f684ed8c79", same_tree_digest_for_all_targets: true,
          same_manifest_digest_for_all_targets: true, same_closure_digest_for_all_targets: true }
      },
      {
        step: "client_discovery",
        checks: {
          claude: { argv: ["claude", "plugin", "list", "--json"], exit_code: 0, plugin_id: "chrome-devtools@skills-dir", plugin_version: "1.8.0",
            enabled: true, skill_count: 6, mcp_command: ["npx", "chrome-devtools-mcp@1.8.0"] },
          gemini_mcp: { argv: ["gemini", "mcp", "list"], exit_code: 0, server: "chrome-devtools", transport: "stdio",
            command: ["npx", "chrome-devtools-mcp@1.8.0"], connection: "disconnected" },
          gemini_skills: { argv: ["gemini", "skills", "list"], exit_code: 0, enabled_skill_count: 6 },
          opencode: { argv: ["opencode", "debug", "config"], exit_code: 0, server: "chrome-devtools", type: "local",
            command: ["npx", "chrome-devtools-mcp@1.8.0"], cwd_bound_to_managed_package: true, plugin_root_bound: true, plugin_data_bound: true },
          cline: { config: "mcpServers.chrome-devtools", transport: "stdio", command: ["npx", "chrome-devtools-mcp@1.8.0"], plugin_root_bound: true,
            plugin_data_bound: true, projected_skill_count: 6 },
          windsurf: { config: "mcpServers.chrome-devtools", command: ["npx", "chrome-devtools-mcp@1.8.0"], plugin_root_bound: true,
            plugin_data_bound: true, skills: "prepared_only" }
        }
      },
      { step: "doctor", argv: ["agentplugins", "doctor"], exit_code: 0, result: "success", read_only: true,
        installation_count: 1, projection_drift_findings: 0, authentication_not_checked_clients: [] },
      { step: "immutable_update_preflight", argv: ["agentplugins", "update"], exit_code: 1, result: "failure",
        status: "preflight_failed", reason: "direct full-SHA installations require explicit switch", succeeded: 0, failed: 5, mutated_targets: 0,
        postcondition: "the exact installation and all five client projections remained installed and unchanged" },
      { step: "repair", argv: ["agentplugins", "repair"], exit_code: 0, result: "success", status: "completed",
        succeeded: 5, failed: 0, targets: Object.fromEntries(["claude", "gemini", "opencode", "cline", "windsurf"].map((name) => [name, "passed"])) },
      { step: "remove", argv: ["agentplugins", "remove"], exit_code: 0, result: "success", status: "data_retained",
        succeeded: 5, failed: 0, plugin_data_preserved: true,
        targets: Object.fromEntries(["claude", "gemini", "opencode", "cline", "windsurf"].map((name) => [name, "external_completed"])) },
      { step: "post_remove", checks: Object.fromEntries(["agentplugins_installation_count", "claude_plugin_count", "gemini_mcp_count",
        "gemini_skill_count", "opencode_mcp_count", "cline_mcp_count", "cline_skill_count", "windsurf_mcp_count"].map((name) => [name, 0])) }
    ],
    claim_boundary: { lifecycle_e2e: true, client_discovery_e2e: true, browser_tool_runtime_e2e: false,
      model_turn_e2e: false, login_e2e: false, oauth_e2e: false, windsurf_skill_activation_claimed: false }
  };
  const targets = "claude,gemini,opencode,cline,windsurf";
  record.transcript[0].argv = ["agentplugins", "add", record.package.selector, "--target", targets, "--format", "json"];
  record.transcript[2].argv = ["agentplugins", "doctor", "--format", "json"];
  record.transcript[2].authentication_not_checked_clients = targets.split(",");
  record.transcript[3].argv = ["agentplugins", "update", record.package.name, "--target", targets, "--format", "json"];
  record.transcript[4].argv = ["agentplugins", "repair", record.package.name, "--target", targets, "--format", "json"];
  record.transcript[5].argv = ["agentplugins", "remove", record.package.name, "--target", targets, "--format", "json"];
  const recordFixture = path.join(__dirname, "fixtures/historical-evidence/evidence/agentplugins-client-e2e-2026-08-30.json");
  assert.deepEqual(record, JSON.parse(await fsp.readFile(recordFixture, "utf8")));
  const recordBody = await fsp.readFile(recordFixture);
  const recordDigest = crypto.createHash("sha256").update(recordBody).digest("hex");
  await fsp.writeFile(path.join(evidenceRoot, "evidence/agentplugins-client-e2e-2026-08-30.json"), recordBody);
  assert.equal(recordDigest, "437da1bc7423a85b231be139ff9bfbd7e89c942ef216a61ebde668c08a9c2ee3");
  await fsp.copyFile(
    path.join(__dirname, "fixtures/historical-evidence/AGENTPLUGINS_CLIENT_E2E.md"),
    path.join(evidenceRoot, "AGENTPLUGINS_CLIENT_E2E.md")
  );
  return evidenceRoot;
}

function childNodeEnvironment(extra = {}) {
  const environment = {};
  for (const [key, value] of Object.entries(process.env)) {
    if (key === "NODE_UNIQUE_ID" || key === "NODE_CHANNEL_FD" ||
        key === "NODE_CHANNEL_SERIALIZATION_MODE" || key.startsWith("NODE_TEST_")) continue;
    environment[key] = value;
  }
  return { ...environment, ...extra };
}

test("release staging embeds every exact platform asset hash", async (t) => {
  const root = await fsp.mkdtemp(path.join(os.tmpdir(), "agentplugins-stage-"));
  t.after(() => fsp.rm(root, { recursive: true, force: true }));
  const packageRoot = path.join(root, "package");
  const assetsRoot = path.join(root, "assets");
  await fsp.mkdir(packageRoot);
  await fsp.mkdir(assetsRoot);
  await fsp.writeFile(path.join(packageRoot, "package.json"), JSON.stringify({
    name: "universal-agent-plugins",
    version: "0.0.0-development",
    bin: { agentplugins: "bin/agentplugins.js" }
  }));
  const version = "0.1.0";
  for (const [platform, arch] of [["darwin", "x64"], ["darwin", "arm64"], ["linux", "x64"], ["linux", "arm64"], ["win32", "x64"], ["win32", "arm64"]]) {
    const info = detectPlatform(platform, arch);
    await fsp.writeFile(path.join(assetsRoot, expectedAssetName(version, info)), `${info.key}\n`);
  }
  prepareRelease(assetsRoot, `agentplugins-v${version}`, COMMIT);
  const release = verifyRelease(assetsRoot, `agentplugins-v${version}`, COMMIT);
  const evidenceRoot = await fixtureEvidence(root);
  const manifest = stage(packageRoot, assetsRoot, version, COMMIT, { evidenceRoot });
  assert.equal(manifest.version, version);
  assert.deepEqual(
    await fsp.readFile(path.join(packageRoot, "THIRD_PARTY_NOTICES.txt")),
    await fsp.readFile(path.join(assetsRoot, "THIRD_PARTY_NOTICES.txt"))
  );
  assert.equal(manifest.npm_package, "universal-agent-plugins");
  assert.equal(manifest.producer.repository, "777genius/plugin-kit-ai");
  assert.equal(manifest.producer.commit, COMMIT);
  assert.equal(manifest.producer.release_manifest.sha256, release.manifest_sha256);
  assert.equal(manifest.client_evidence.installer.commit, HISTORICAL_COMMIT);
  assert.deepEqual(manifest.client_evidence.source, {
    repository: "777genius/plugin-kit-ai",
    commit: "01f02cb51cfe5f664d4d5f52b295c59c7ea03495",
    document: {
      path: "docs/AGENTPLUGINS_CLIENT_E2E.md",
      sha256: manifest.client_evidence.document_sha256
    },
    record: {
      path: "docs/evidence/agentplugins-client-e2e-2026-08-30.json",
      sha256: manifest.client_evidence.record_sha256
    }
  });
  assert.notEqual(manifest.client_evidence.installer.commit, manifest.producer.commit);
  assert.notEqual(manifest.client_evidence.installer.version, manifest.version);
  assert.equal(Object.keys(manifest.assets).length, 6);
  for (const asset of Object.values(manifest.assets)) {
    assert.match(asset.sha256, /^[0-9a-f]{64}$/);
    assert.ok(asset.size > 0);
  }
  const pkg = JSON.parse(await fsp.readFile(path.join(packageRoot, "package.json"), "utf8"));
  assert.equal(pkg.version, version);
  assert.equal(
    await fsp.readFile(path.join(packageRoot, "test/evidence-root/AGENTPLUGINS_CLIENT_E2E.md"), "utf8"),
    await fsp.readFile(path.join(evidenceRoot, "AGENTPLUGINS_CLIENT_E2E.md"), "utf8")
  );
  // A rechecksummed companion from a different source must not reach npm.
  const noticeFile = path.join(assetsRoot, "THIRD_PARTY_NOTICES.txt");
  await fsp.writeFile(noticeFile, "different source notices\n");
  const noticeHash = crypto.createHash("sha256").update(await fsp.readFile(noticeFile)).digest("hex");
  const checksumsFile = path.join(assetsRoot, "checksums.txt");
  await fsp.writeFile(checksumsFile, (await fsp.readFile(checksumsFile, "utf8")).replace(
    /^[0-9a-f]{64}  THIRD_PARTY_NOTICES\.txt$/m, `${noticeHash}  THIRD_PARTY_NOTICES.txt`
  ));
  assert.throws(() => stage(packageRoot, assetsRoot, version, COMMIT, { evidenceRoot }), /packaged notices do not match/);
});

test("historical schema-v2 staging rejects missing notices before mutations or packing", async (t) => {
  const root = await fsp.mkdtemp(path.join(os.tmpdir(), "agentplugins-stage-missing-notices-"));
  t.after(() => fsp.rm(root, { recursive: true, force: true }));
  const packageRoot = path.join(root, "package");
  const assetsRoot = path.join(root, "assets");
  await fsp.mkdir(packageRoot);
  await fsp.mkdir(assetsRoot);
  await fsp.writeFile(path.join(packageRoot, "package.json"), JSON.stringify({
    name: "universal-agent-plugins", version: "0.0.0-development",
    bin: { agentplugins: "bin/agentplugins.js" }, files: ["THIRD_PARTY_NOTICES.txt"]
  }));
  await fsp.writeFile(path.join(packageRoot, "assets.json"), "existing asset metadata\n");
  await fsp.writeFile(path.join(packageRoot, "THIRD_PARTY_NOTICES.txt"), "stale checkout notices\n");
  await fsp.mkdir(path.join(packageRoot, "test/evidence-root"), { recursive: true });
  await fsp.writeFile(path.join(packageRoot, "test/evidence-root/keep.txt"), "existing evidence\n");
  const version = "0.1.12";
  for (const [platform, arch] of [["darwin", "x64"], ["darwin", "arm64"], ["linux", "x64"], ["linux", "arm64"], ["win32", "x64"], ["win32", "arm64"]]) {
    const info = detectPlatform(platform, arch);
    await fsp.writeFile(path.join(assetsRoot, expectedAssetName(version, info)), `${info.key}\n`);
  }
  prepareRelease(assetsRoot, `agentplugins-v${version}`, COMMIT);
  await fsp.unlink(path.join(assetsRoot, "THIRD_PARTY_NOTICES.txt"));
  const checksumsPath = path.join(assetsRoot, "checksums.txt");
  await fsp.writeFile(checksumsPath, (await fsp.readFile(checksumsPath, "utf8"))
    .split("\n").filter((line) => !line.endsWith("  THIRD_PARTY_NOTICES.txt")).join("\n"));
  const historical = verifyRelease(assetsRoot, `agentplugins-v${version}`, COMMIT);
  assert.equal(historical.manifest_schema, 2);
  assert.equal(historical.gate_eligible, true);
  assert.deepEqual(historical.notices, []);
  const evidenceRoot = await fixtureEvidence(root);
  const snapshot = () => fs.readdirSync(root, { recursive: true }).sort().map((name) => {
    const file = path.join(root, name);
    return [name, fs.statSync(file).isDirectory() ? null : fs.readFileSync(file)];
  });
  const before = snapshot();
  const pack = t.mock.fn();
  const afterInitialReleaseVerification = t.mock.fn();
  assert.throws(() => {
    stage(packageRoot, assetsRoot, version, COMMIT, { evidenceRoot, afterInitialReleaseVerification });
    pack();
  }, /release staging requires verified companion THIRD_PARTY_NOTICES\.txt/);
  assert.equal(pack.mock.callCount(), 0);
  assert.equal(afterInitialReleaseVerification.mock.callCount(), 0);
  assert.deepEqual(snapshot(), before);
});

test("staged package tests are hermetic to repository layout and caller cwd", {
  skip: process.env.AGENTPLUGINS_STAGED_TEST_CHILD === "1"
}, async (t) => {
  const root = await fsp.mkdtemp(path.join(os.tmpdir(), "agentplugins-hermetic-stage-"));
  t.after(() => fsp.rm(root, { recursive: true, force: true }));
  const packageRoot = path.join(root, "detached-layout", "package");
  const assetsRoot = path.join(root, "release-assets");
  const callerRoot = path.join(root, "unrelated-caller");
  const npmCache = path.join(root, "npm-cache");
  await fsp.cp(path.resolve(__dirname, ".."), packageRoot, { recursive: true });
  await fsp.rm(path.join(packageRoot, "test", "evidence-root"), { recursive: true, force: true });
  await fsp.mkdir(assetsRoot);
  await fsp.mkdir(callerRoot);
  const version = "0.1.22";
  for (const [platform, arch] of [["darwin", "x64"], ["darwin", "arm64"], ["linux", "x64"], ["linux", "arm64"], ["win32", "x64"], ["win32", "arm64"]]) {
    const info = detectPlatform(platform, arch);
    await fsp.writeFile(path.join(assetsRoot, expectedAssetName(version, info)), `${info.key}\n`);
  }
  prepareRelease(assetsRoot, `agentplugins-v${version}`, COMMIT);
  const evidenceRoot = await fixtureEvidence(root);
  stage(packageRoot, assetsRoot, version, COMMIT, { evidenceRoot });

  const result = childProcess.spawnSync("npm", ["--prefix", packageRoot, "test"], {
    cwd: callerRoot,
    encoding: "utf8",
    env: childNodeEnvironment({
      AGENTPLUGINS_STAGED_TEST_CHILD: "1",
      AGENTPLUGINS_DETACHED_ASSERT_ROOT: packageRoot,
      NPM_CONFIG_CACHE: npmCache
    })
  });
  assert.equal(result.status, 0, `${result.stdout}\n${result.stderr}`);
  assert.match(result.stdout, /[ℹ#] tests [1-9][0-9]*/);
  assert.match(result.stdout, /[ℹ#] pass [1-9][0-9]*/);
  assert.match(result.stdout, /detached package execution sentinel/);

  const packed = childProcess.spawnSync("npm", ["pack", "--ignore-scripts", "--json"], {
    cwd: packageRoot,
    encoding: "utf8",
    env: childNodeEnvironment({ NPM_CONFIG_CACHE: npmCache })
  });
  assert.equal(packed.status, 0, `${packed.stdout}\n${packed.stderr}`);
  const packResult = JSON.parse(packed.stdout);
  assert.equal(packResult.length, 1);
  const packedFiles = packResult[0].files.map(({ path: filename }) => filename);
  assert.ok(packedFiles.includes("README.md"));
  assert.ok(packedFiles.includes("assets.json"));
  assert.ok(packedFiles.includes("THIRD_PARTY_NOTICES.txt"));
  const packedNotices = childProcess.execFileSync("tar", ["-xOf",
    path.join(packageRoot, packResult[0].filename), "package/THIRD_PARTY_NOTICES.txt"]);
  assert.deepEqual(packedNotices, await fsp.readFile(path.join(assetsRoot, "THIRD_PARTY_NOTICES.txt")));
  assert.equal(packedFiles.some((filename) => filename.startsWith("test/") || filename.includes("evidence-root")), false);
});

test("release staging rejects malformed and internally mismatched historical evidence", async (t) => {
  const root = await fsp.mkdtemp(path.join(os.tmpdir(), "agentplugins-bad-evidence-"));
  t.after(() => fsp.rm(root, { recursive: true, force: true }));
  const packageRoot = path.join(root, "package");
  const assetsRoot = path.join(root, "assets");
  await fsp.mkdir(packageRoot);
  await fsp.mkdir(assetsRoot);
  await fsp.writeFile(path.join(packageRoot, "package.json"), JSON.stringify({
    name: "universal-agent-plugins", version: "0.0.0-development",
    bin: { agentplugins: "bin/agentplugins.js" }
  }));
  const version = "0.1.9";
  for (const [platform, arch] of [["darwin", "x64"], ["darwin", "arm64"], ["linux", "x64"], ["linux", "arm64"], ["win32", "x64"], ["win32", "arm64"]]) {
    const info = detectPlatform(platform, arch);
    await fsp.writeFile(path.join(assetsRoot, expectedAssetName(version, info)), `${info.key}\n`);
  }
  prepareRelease(assetsRoot, `agentplugins-v${version}`, COMMIT);
  const evidenceRoot = await fixtureEvidence(root);
  const recordPath = path.join(evidenceRoot, "evidence/agentplugins-client-e2e-2026-08-30.json");
  await fsp.writeFile(recordPath, "dummy\n");
  assert.throws(() => stage(packageRoot, assetsRoot, version, COMMIT, { evidenceRoot }), /JSON is malformed/);

  await fsp.rm(path.join(packageRoot, "test"), { recursive: true, force: true });
  const repairedEvidence = await fixtureEvidence(root);
  const record = JSON.parse(await fsp.readFile(recordPath, "utf8"));
  record.package.selector = `ChromeDevTools/chrome-devtools-mcp@${"9".repeat(40)}`;
  await fsp.writeFile(recordPath, JSON.stringify(record));
  assert.throws(() => stage(packageRoot, assetsRoot, version, COMMIT, { evidenceRoot: repairedEvidence }), /package identity is invalid/);

  await fixtureEvidence(root);
  const extraKeyRecord = JSON.parse(await fsp.readFile(recordPath, "utf8"));
  extraKeyRecord.unexpected = true;
  await fsp.writeFile(recordPath, JSON.stringify(extraKeyRecord));
  assert.throws(() => stage(packageRoot, assetsRoot, version, COMMIT, { evidenceRoot }), /record keys are invalid/);

  await fixtureEvidence(root);
  const markdownPath = path.join(evidenceRoot, "AGENTPLUGINS_CLIENT_E2E.md");
  const markdown = await fsp.readFile(markdownPath, "utf8");
  await fsp.writeFile(markdownPath, markdown.replace(
    "437da1bc7423a85b231be139ff9bfbd7e89c942ef216a61ebde668c08a9c2ee3",
    "0".repeat(64)
  ));
  assert.throws(() => stage(packageRoot, assetsRoot, version, COMMIT, { evidenceRoot }), /does not bind/);
});

test("historical client discovery and boundary claims reject every semantic bypass", async (t) => {
  const root = await fsp.mkdtemp(path.join(os.tmpdir(), "agentplugins-evidence-semantics-"));
  t.after(() => fsp.rm(root, { recursive: true, force: true }));
  const evidenceRoot = await fixtureEvidence(root);
  const original = JSON.parse(await fsp.readFile(
    path.join(evidenceRoot, "evidence/agentplugins-client-e2e-2026-08-30.json"), "utf8"
  ));
  const cases = [
    ["claude enabled", (r) => { r.transcript[1].checks.claude.enabled = false; }],
    ["claude skill count", (r) => { r.transcript[1].checks.claude.skill_count = 1; }],
    ["claude command", (r) => { r.transcript[1].checks.claude.mcp_command = ["wrong"]; }],
    ["Gemini connection", (r) => { r.transcript[1].checks.gemini_mcp.connection = "connected"; }],
    ["Gemini command", (r) => { r.transcript[1].checks.gemini_mcp.command = ["wrong"]; }],
    ["Gemini skill count", (r) => { r.transcript[1].checks.gemini_skills.enabled_skill_count = 1; }],
    ["OpenCode cwd binding", (r) => { r.transcript[1].checks.opencode.cwd_bound_to_managed_package = false; }],
    ["OpenCode root binding", (r) => { r.transcript[1].checks.opencode.plugin_root_bound = false; }],
    ["OpenCode data binding", (r) => { r.transcript[1].checks.opencode.plugin_data_bound = false; }],
    ["OpenCode command", (r) => { r.transcript[1].checks.opencode.command = ["wrong"]; }],
    ["Cline root binding", (r) => { r.transcript[1].checks.cline.plugin_root_bound = false; }],
    ["Cline data binding", (r) => { r.transcript[1].checks.cline.plugin_data_bound = false; }],
    ["Cline skill count", (r) => { r.transcript[1].checks.cline.projected_skill_count = 1; }],
    ["Cline command", (r) => { r.transcript[1].checks.cline.command = ["wrong"]; }],
    ["Windsurf root binding", (r) => { r.transcript[1].checks.windsurf.plugin_root_bound = false; }],
    ["Windsurf data binding", (r) => { r.transcript[1].checks.windsurf.plugin_data_bound = false; }],
    ["Windsurf activation boundary", (r) => { r.transcript[1].checks.windsurf.skills = "active"; }],
    ["Windsurf command", (r) => { r.transcript[1].checks.windsurf.command = ["wrong"]; }],
    ["discovery argv", (r) => { r.transcript[1].checks.claude.argv = ["wrong"]; }],
    ["doctor argv", (r) => { r.transcript[2].argv = ["agentplugins", "doctor"]; }],
    ["doctor authentication boundary", (r) => { r.transcript[2].authentication_not_checked_clients = []; }],
    ["preflight argv", (r) => { r.transcript[3].argv = ["agentplugins", "update"]; }],
    ["preflight reason", (r) => { r.transcript[3].reason = "wrong but non-empty"; }],
    ["preflight failed count", (r) => { r.transcript[3].failed = 0; }],
    ["preflight postcondition", (r) => { r.transcript[3].postcondition = "wrong but non-empty"; }]
  ];
  for (const [label, mutate] of cases) {
    const record = structuredClone(original);
    mutate(record);
    assert.throws(() => validateEvidenceRecord(JSON.stringify(record)), /client E2E evidence|client E2E discovery/, label);
  }
});

test("identical release bytes at different commits produce distinct fail-closed metadata", async (t) => {
  const root = await fsp.mkdtemp(path.join(os.tmpdir(), "agentplugins-commit-bound-"));
  t.after(() => fsp.rm(root, { recursive: true, force: true }));
  const version = "0.1.10";
  const results = [];
  for (const commit of ["1".repeat(40), "2".repeat(40)]) {
    const caseRoot = path.join(root, commit[0]);
    const packageRoot = path.join(caseRoot, "package");
    const assetsRoot = path.join(caseRoot, "assets");
    await fsp.mkdir(packageRoot, { recursive: true });
    await fsp.mkdir(assetsRoot);
    await fsp.writeFile(path.join(packageRoot, "package.json"), JSON.stringify({
      name: "universal-agent-plugins", version: "0.0.0-development",
      bin: { agentplugins: "bin/agentplugins.js" }
    }));
    for (const [platform, arch] of [["darwin", "x64"], ["darwin", "arm64"], ["linux", "x64"], ["linux", "arm64"], ["win32", "x64"], ["win32", "arm64"]]) {
      const info = detectPlatform(platform, arch);
      await fsp.writeFile(path.join(assetsRoot, expectedAssetName(version, info)), `${info.key}\n`);
    }
    prepareRelease(assetsRoot, `agentplugins-v${version}`, commit);
    const evidenceRoot = await fixtureEvidence(caseRoot);
    results.push(stage(packageRoot, assetsRoot, version, commit, { evidenceRoot }));
  }
  assert.notEqual(results[0].producer.commit, results[1].producer.commit);
  assert.notEqual(results[0].producer.release_manifest.sha256, results[1].producer.release_manifest.sha256);
  assert.equal(results[0].client_evidence.record_sha256, results[1].client_evidence.record_sha256);
});

test("release verification fails closed on checksum, size, and manifest mismatch", async (t) => {
  const root = await fsp.mkdtemp(path.join(os.tmpdir(), "agentplugins-release-assets-"));
  t.after(() => fsp.rm(root, { recursive: true, force: true }));
  const version = "0.1.4";
  for (const [platform, arch] of [["darwin", "x64"], ["darwin", "arm64"], ["linux", "x64"], ["linux", "arm64"], ["win32", "x64"], ["win32", "arm64"]]) {
    const info = detectPlatform(platform, arch);
    await fsp.writeFile(path.join(root, expectedAssetName(version, info)), `${info.key}\n`);
  }
  prepareRelease(root, `agentplugins-v${version}`, COMMIT);
  assert.equal(verifyRelease(root, `agentplugins-v${version}`, COMMIT).gate_eligible, true);

  await fsp.writeFile(path.join(root, "ambiguous-extra-asset"), "unexpected");
  assert.throws(() => verifyRelease(root, `agentplugins-v${version}`, COMMIT), /must contain exactly/);
  await fsp.rm(path.join(root, "ambiguous-extra-asset"));

  const manifestPath = path.join(root, "release-manifest.json");
  const manifest = JSON.parse(await fsp.readFile(manifestPath, "utf8"));
  manifest.assets["linux-amd64"].size += 1;
  await fsp.writeFile(manifestPath, JSON.stringify(manifest));
  const checksumsPath = path.join(root, "checksums.txt");
  const manifestDigest = crypto.createHash("sha256").update(await fsp.readFile(manifestPath)).digest("hex");
  const checksums = (await fsp.readFile(checksumsPath, "utf8")).replace(
    /^[0-9a-f]{64}  release-manifest\.json$/m,
    `${manifestDigest}  release-manifest.json`
  );
  await fsp.writeFile(checksumsPath, checksums);
  assert.throws(() => verifyRelease(root, `agentplugins-v${version}`, COMMIT), /asset metadata mismatch/);

  prepareRelease(root, `agentplugins-v${version}`, COMMIT);
  await fsp.appendFile(path.join(root, `agentplugins_${version}_linux_amd64`), "changed");
  assert.throws(() => verifyRelease(root, `agentplugins-v${version}`, COMMIT), /checksum mismatch/);

  prepareRelease(root, `agentplugins-v${version}`, COMMIT);
  const wrongIdentity = JSON.parse(await fsp.readFile(manifestPath, "utf8"));
  wrongIdentity.commit = "b".repeat(40);
  await fsp.writeFile(manifestPath, JSON.stringify(wrongIdentity));
  assert.throws(() => verifyRelease(root, `agentplugins-v${version}`, COMMIT), /identity does not match/);
});

test("release staging rejects hardlinked inputs and concurrent release replacement", async (t) => {
  const root = await fsp.mkdtemp(path.join(os.tmpdir(), "agentplugins-stage-attacks-"));
  t.after(() => fsp.rm(root, { recursive: true, force: true }));
  const makeCase = async (name) => {
    const caseRoot = path.join(root, name);
    const packageRoot = path.join(caseRoot, "package");
    const assetsRoot = path.join(caseRoot, "assets");
    await fsp.mkdir(packageRoot, { recursive: true });
    await fsp.mkdir(assetsRoot);
    await fsp.writeFile(path.join(packageRoot, "package.json"), JSON.stringify({
      name: "universal-agent-plugins", version: "0.0.0-development", bin: { agentplugins: "bin/agentplugins.js" }
    }));
    const version = "0.1.11";
    for (const [platform, arch] of [["darwin", "x64"], ["darwin", "arm64"], ["linux", "x64"], ["linux", "arm64"], ["win32", "x64"], ["win32", "arm64"]]) {
      const info = detectPlatform(platform, arch);
      await fsp.writeFile(path.join(assetsRoot, expectedAssetName(version, info)), `${info.key}\n`);
    }
    prepareRelease(assetsRoot, `agentplugins-v${version}`, COMMIT);
    return { packageRoot, assetsRoot, version, evidenceRoot: await fixtureEvidence(caseRoot) };
  };

  const hardlinkCase = await makeCase("hardlink");
  const source = path.join(hardlinkCase.assetsRoot, `agentplugins_${hardlinkCase.version}_linux_amd64`);
  await fsp.link(source, path.join(root, "external-hardlink"));
  assert.throws(() => stage(hardlinkCase.packageRoot, hardlinkCase.assetsRoot, hardlinkCase.version, COMMIT,
    { evidenceRoot: hardlinkCase.evidenceRoot }), /unaliased file/);
  assert.equal(fs.existsSync(path.join(hardlinkCase.packageRoot, "assets.json")), false);

  const mutationCase = await makeCase("mutation");
  assert.throws(() => stage(mutationCase.packageRoot, mutationCase.assetsRoot, mutationCase.version, COMMIT, {
    evidenceRoot: mutationCase.evidenceRoot,
    afterInitialReleaseVerification() {
      const target = path.join(mutationCase.assetsRoot, `agentplugins_${mutationCase.version}_linux_amd64`);
      const replacement = path.join(mutationCase.assetsRoot, "replacement-in-flight");
      fs.writeFileSync(replacement, "concurrent replacement");
      fs.renameSync(replacement, target);
    }
  }), /checksum mismatch/);
  assert.equal(fs.existsSync(path.join(mutationCase.packageRoot, "assets.json")), false);
  assert.equal(JSON.parse(await fsp.readFile(path.join(mutationCase.packageRoot, "package.json"))).version, "0.0.0-development");

  const postCheckCase = await makeCase("post-check-replacement");
  const validated = verifyRelease(postCheckCase.assetsRoot, `agentplugins-v${postCheckCase.version}`, COMMIT);
  const staged = stage(postCheckCase.packageRoot, postCheckCase.assetsRoot, postCheckCase.version, COMMIT, {
    evidenceRoot: postCheckCase.evidenceRoot,
    afterReleaseSnapshotVerification() {
      for (const [platform, arch] of [["darwin", "x64"], ["darwin", "arm64"], ["linux", "x64"],
        ["linux", "arm64"], ["win32", "x64"], ["win32", "arm64"]]) {
        const info = detectPlatform(platform, arch);
        fs.writeFileSync(path.join(postCheckCase.assetsRoot, expectedAssetName(postCheckCase.version, info)), `replacement-${info.key}\n`);
      }
      prepareRelease(postCheckCase.assetsRoot, `agentplugins-v${postCheckCase.version}`, COMMIT);
    }
  });
  const replaced = verifyRelease(postCheckCase.assetsRoot, `agentplugins-v${postCheckCase.version}`, COMMIT);
  assert.notEqual(replaced.manifest_sha256, validated.manifest_sha256);
  assert.equal(staged.producer.release_manifest.sha256, validated.manifest_sha256);
  assert.deepEqual(staged.assets, validated.assets);
});

test("legacy manifests are audit-only and never gate eligible", async (t) => {
  const root = await fsp.mkdtemp(path.join(os.tmpdir(), "agentplugins-legacy-assets-"));
  t.after(() => fsp.rm(root, { recursive: true, force: true }));
  const version = "0.1.4";
  for (const [platform, arch] of [["darwin", "x64"], ["darwin", "arm64"], ["linux", "x64"], ["linux", "arm64"], ["win32", "x64"], ["win32", "arm64"]]) {
    const info = detectPlatform(platform, arch);
    await fsp.writeFile(path.join(root, expectedAssetName(version, info)), `${info.key}\n`);
  }
  const { assets } = prepareRelease(root, `agentplugins-v${version}`, COMMIT);
  await fsp.rm(path.join(root, "THIRD_PARTY_NOTICES.txt"));
  await fsp.writeFile(path.join(root, "release-manifest.json"), JSON.stringify({
    schema_version: 1,
    tag: `agentplugins-v${version}`,
    commit: COMMIT
  }) + "\n");
  const names = [...Object.values(assets).map((asset) => asset.file), "release-manifest.json"];
  const checksums = names.map((name) => {
    const body = fs.readFileSync(path.join(root, name));
    return `${crypto.createHash("sha256").update(body).digest("hex")}  ${name}`;
  }).join("\n") + "\n";
  await fsp.writeFile(path.join(root, "checksums.txt"), checksums);
  assert.throws(() => verifyRelease(root, `agentplugins-v${version}`, COMMIT), /schema or version/);
  const audited = verifyRelease(root, `agentplugins-v${version}`, COMMIT, { allowLegacyManifest: true });
  assert.equal(audited.manifest_schema, 1);
  assert.equal(audited.gate_eligible, false);

  const stagingRoot = await fsp.mkdtemp(path.join(os.tmpdir(), "agentplugins-legacy-stage-"));
  t.after(() => fsp.rm(stagingRoot, { recursive: true, force: true }));
  const packageRoot = path.join(stagingRoot, "package");
  await fsp.mkdir(packageRoot);
  await fsp.writeFile(path.join(packageRoot, "package.json"), JSON.stringify({
    name: "universal-agent-plugins", version: "0.0.0-development", bin: { agentplugins: "bin/agentplugins.js" }
  }));
  const evidenceRoot = await fixtureEvidence(stagingRoot);
  assert.throws(() => stage(packageRoot, root, version, COMMIT, { allowLegacyManifest: true, evidenceRoot }), /gate-eligible schema-v2/);
});

test("npm distribution name is independent from the agentplugins binary name", () => {
  for (const name of [
    "agentplugins",
    "universal-agent-plugins",
    "agentplugins-cli",
    "@ilyazelenko/agentplugins",
    "@777genius/agentplugins"
  ]) {
    assert.equal(validatePackageMetadata({
      name,
      bin: { agentplugins: "bin/agentplugins.js" }
    }), name);
  }
});

test("release staging rejects unsafe package metadata", () => {
  assert.throws(() => stageEvidence("package", "docs"), /evidence root must be absolute/);
  for (const name of ["AgentPlugins", " agentplugins ", 123, null]) {
    assert.throws(
      () => validatePackageMetadata({ name, bin: { agentplugins: "bin/agentplugins.js" } }),
      /package name is invalid/
    );
  }
  assert.throws(
    () => validatePackageMetadata({ name: "agentplugins-cli", bin: { other: "bin/agentplugins.js" } }),
    /must expose the agentplugins binary/
  );
});
