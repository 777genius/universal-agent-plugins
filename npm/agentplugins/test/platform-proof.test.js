"use strict";

const assert = require("node:assert/strict");
const crypto = require("node:crypto");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const test = require("node:test");

const script = path.resolve(__dirname, "..", "scripts", "platform-proof.js");
const {
  assertAssetManifest,
  assertContext7Search,
  assertInstalledShimInvocation,
  assertPublicJSONPathFree,
  assertSyntheticInfo,
  frozenReleaseAsset,
  installedShimInvocation,
  lifecycleCommands,
  lifecycleResult,
  npmInvocation,
  parseBootstrapMode,
  parseLifecycle
} = require(script);

test("platform proof binds assets.json producer commit in staged and public modes", () => {
  const version = "1.2.3";
  const commit = "a".repeat(40);
  const manifest = {
    schema_version: 2,
    version,
    npm_package: "universal-agent-plugins",
    repository: "777genius/plugin-kit-ai",
    tag: `agentplugins-v${version}`,
    producer: {
      repository: "777genius/plugin-kit-ai",
      tag: `agentplugins-v${version}`,
      commit
    },
    assets: { "linux-amd64": { file: "agentplugins", size: 1, sha256: "0".repeat(64) } }
  };
  assert.equal(assertAssetManifest(manifest, version, commit, "linux-amd64"), manifest.assets["linux-amd64"]);
  for (const mutate of [
    (value) => { value.producer.commit = "b".repeat(40); },
    (value) => { value.producer.tag = "agentplugins-v9.9.9"; },
    (value) => { value.producer.repository = "lookalike/repository"; },
    (value) => { delete value.producer; }
  ]) {
    const invalid = structuredClone(manifest);
    mutate(invalid);
    assert.throws(() => assertAssetManifest(invalid, version, commit, "linux-amd64"), /expected commit/);
  }
});

test("platform proof invokes the installed npm shim and rejects direct bin bypass", () => {
  const project = path.join(os.tmpdir(), "proof project");
  const posix = installedShimInvocation(project, ["version"], "linux");
  assert.equal(posix.command, path.join(project, "node_modules", ".bin", "agentplugins"));
  assert.equal(posix.shell, false);
  assert.doesNotThrow(() => assertInstalledShimInvocation(posix, project, "linux"));

  const windows = installedShimInvocation(project, ["version"], "win32");
  assert.equal(windows.command, path.join(project, "node_modules", ".bin", "agentplugins.cmd"));
  assert.equal(windows.shell, true);
  assert.doesNotThrow(() => assertInstalledShimInvocation(windows, project, "win32"));

  assert.throws(() => assertInstalledShimInvocation({
    command: process.execPath,
    args: [path.join(project, "node_modules", "universal-agent-plugins", "bin", "agentplugins.js")],
    shell: false
  }, project, process.platform), /must execute the installed npm agentplugins shim/);
});

test("platform proof requires the reviewed upstream context7 distribution in search results", () => {
  const valid = {
    schema_version: 1,
    command: "search",
    result: "success",
    data: {
      results: [
        { product_id: "other", distribution_id: "owner/other" },
        {
          product_id: "context7",
          install_selector: "context7",
          distribution_id: "upstash/context7",
          distribution_kind: "upstream",
          trust_state: "reviewed",
          status: "available"
        }
      ]
    }
  };
  assert.doesNotThrow(() => assertContext7Search(valid));
  for (const invalid of [
    { ...valid, data: { results: [] } },
    { ...valid, data: { results: [{ ...valid.data.results[1], install_selector: "other" }] } },
    { ...valid, data: { results: [{ ...valid.data.results[1], install_selector: "discovery:upstash/context7//plugins/agent-plugins/context7" }] } },
    { ...valid, data: { results: [{ ...valid.data.results[1], distribution_id: "777genius/context7" }] } },
    { ...valid, data: { results: [{ ...valid.data.results[1], distribution_kind: "community_bridge" }] } },
    { ...valid, data: { results: [{ ...valid.data.results[1], trust_state: "discovered" }] } },
    { ...valid, data: { results: [{ ...valid.data.results[1], status: "superseded" }] } },
    { ...valid, data: { results: [{ ...valid.data.results[1], product_id: "other" }] } }
  ]) {
    assert.throws(() => assertContext7Search(invalid), /reviewed upstream context7 distribution/);
  }
});

test("platform proof requires exact synthetic info identity, Cursor version, and active state", () => {
  const valid = {
    schema_version: 1,
    command: "info",
    result: "success",
    data: {
      name: "platform-proof-synthetic",
      version: "1.0.0",
      clients: [{
        client_id: "cursor",
        activation: "active",
        package_revision: { version: "1.0.0" }
      }]
    }
  };
  assert.doesNotThrow(() => assertSyntheticInfo(valid));
  for (const mutate of [
    (value) => { value.data.name = "lookalike"; },
    (value) => { value.data.version = "2.0.0"; },
    (value) => { value.data.clients[0].client_id = "codex"; },
    (value) => { value.data.clients[0].activation = "pending"; },
    (value) => { value.data.clients[0].package_revision.version = "2.0.0"; },
    (value) => { value.data.clients.push({ ...value.data.clients[0] }); }
  ]) {
    const invalid = structuredClone(valid);
    mutate(invalid);
    assert.throws(() => assertSyntheticInfo(invalid), /identity, Cursor target, installed version, and active state/);
  }
});

test("platform proof rejects absolute paths anywhere in public lifecycle JSON", () => {
  const privateRoot = path.join(os.tmpdir(), "agentplugins-private-root");
  assert.doesNotThrow(() => assertPublicJSONPathFree({
    source: "https://agent-plugins.org/releases/demo",
    next_action: "open the prepared plugin in the selected client"
  }, "safe JSON", [privateRoot]));
  assert.throws(() => assertPublicJSONPathFree({
    result: { activation: { next_action: `open ${privateRoot}/plugins/demo` } }
  }, "leaking JSON", [privateRoot]), /absolute local filesystem path/);
  assert.throws(() => assertPublicJSONPathFree({
    next_action: "open C:\\Users\\runner\\agentplugins\\demo"
  }, "Windows leaking JSON", []), /absolute local filesystem path/);
});

test("platform proof rejects lifecycle values other than exact true or false", () => {
  for (const value of ["TRUE", "False", "1", "yes"]) {
    assert.throws(() => parseLifecycle(value), /lifecycle must be exactly true or false/);
  }
  assert.equal(parseLifecycle("true"), true);
  assert.equal(parseLifecycle("false"), false);
});

test("Windows npm invocation executes npm-cli.js without a shell", () => {
  const execPath = "C:\\Program Files\\nodejs\\node.exe";
  const tarball = "C:\\proof workspace\\universal-agent-plugins.tgz";
  const invocation = npmInvocation(["install", "--save-exact", tarball], "win32", execPath);

  assert.equal(invocation.command, execPath);
  assert.deepEqual(invocation.args, [
    "C:\\Program Files\\nodejs\\node_modules\\npm\\bin\\npm-cli.js",
    "install",
    "--save-exact",
    tarball
  ]);
  assert.equal(Object.hasOwn(invocation, "shell"), false);
});

test("platform proof lifecycle commands use the default no-confirmation flow", () => {
  const commands = lifecycleCommands("C:\\proof workspace\\synthetic plugin");

  assert.equal(commands.length, 4);
  assert.deepEqual(commands.map((command) => command[0]), ["add", "add", "update", "remove"]);
  for (const command of commands) {
    assert.equal(command.includes("--yes"), false, `unexpected --yes in ${command.join(" ")}`);
  }
});

test("platform proof reads exact single-target results from direct and batch envelopes", () => {
  const direct = lifecycleResult({
    schema_version: 1,
    command: "add",
    result: "success",
    data: { result: { mutated: true } }
  }, "add", "cursor");
  assert.deepEqual(direct, { mutated: true });

  const batch = lifecycleResult({
    schema_version: 1,
    command: "update",
    result: "success",
    data: {
      batch: true,
      succeeded: 1,
      failed: 0,
      targets: [{ target: "cursor", output: { result: { no_change: true } } }]
    }
  }, "update", "cursor");
  assert.deepEqual(batch, { no_change: true });

  assert.throws(() => lifecycleResult({
    schema_version: 1,
    command: "add",
    result: "success",
    data: { batch: true, result: { mutated: true } }
  }, "add", "cursor"), /ambiguous direct and batch lifecycle results/);

  assert.throws(() => lifecycleResult({
    schema_version: 1,
    command: "remove",
    result: "success",
    data: {
      batch: true,
      succeeded: 1,
      failed: 0,
      targets: [{ target: "codex", output: { result: { mutated: true } } }]
    }
  }, "remove", "cursor"), /exactly one successful cursor lifecycle result/);
});

test("platform proof accepts only explicit bootstrap evidence modes", () => {
  assert.equal(parseBootstrapMode("local_frozen_asset"), "local_frozen_asset");
  assert.equal(parseBootstrapMode("public_release_download"), "public_release_download");
  assert.throws(() => parseBootstrapMode("download"), /bootstrap mode must be exactly/);
});

test("platform proof binds the local binary to the frozen release manifest and npm pin", (t) => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "agentplugins-platform-manifest-"));
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
  const version = "0.1.0";
  const commit = "a".repeat(40);
  const target = "linux-amd64";
  const file = `agentplugins_${version}_linux_amd64`;
  const binary = Buffer.from("exact frozen binary");
  const pinned = {
    file,
    size: binary.length,
    sha256: crypto.createHash("sha256").update(binary).digest("hex")
  };
  fs.writeFileSync(path.join(root, file), binary);
  fs.writeFileSync(path.join(root, "release-manifest.json"), JSON.stringify({
    schema_version: 2,
    tag: `agentplugins-v${version}`,
    version,
    commit,
    assets: { [target]: pinned }
  }));
  assert.equal(frozenReleaseAsset(root, commit, version, target, pinned), path.join(root, file));
  assert.throws(
    () => frozenReleaseAsset(root, "b".repeat(40), version, target, pinned),
    /manifest does not match/
  );
  assert.throws(
    () => frozenReleaseAsset(root, commit, version, target, { ...pinned, sha256: "0".repeat(64) }),
    /manifest does not match/
  );
});
