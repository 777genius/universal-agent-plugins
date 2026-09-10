#!/usr/bin/env node
"use strict";

// Milestone A's bounded, offline exact-candidate producer and consumer.  This
// intentionally composes the existing sealed native/npm assembly rather than
// introducing another package format.
const fs = require("node:fs");
const path = require("node:path");
const cp = require("node:child_process");
const crypto = require("node:crypto");
const producer = require("./stage-dual-authoring-candidate");
const packer = require("./stage-dual-authoring-npm");

const PRODUCTS = ["agentplugins", "plugin-kit-ai"];
const TARGETS = { linux: ["amd64"], windows: ["amd64"], darwin: ["arm64"] };
const COMMANDS = ["init", "validate", "inspect", "test", "local-add-dry-run"];

function fail(message) { throw new Error(message); }
function sha(bytes) { return crypto.createHash("sha256").update(bytes).digest("hex"); }
function absolute(name, value) {
  if (!value || !path.isAbsolute(value) || path.resolve(value) !== value) fail(`${name} must be absolute`);
  return value;
}
function cleanRoot(root) {
  absolute("root", root);
  if (fs.existsSync(root)) fail("Milestone A root must be new");
  fs.mkdirSync(root, { recursive: false, mode: 0o700 });
  for (const name of ["evidence", "work", "home", "tmp", "cache", "config"])
    fs.mkdirSync(path.join(root, name), { mode: 0o700 });
}
function baseEnv(root) {
  return { ...process.env, HOME: path.join(root, "home"), USERPROFILE: path.join(root, "home"),
    TMPDIR: path.join(root, "tmp"), TMP: path.join(root, "tmp"), TEMP: path.join(root, "tmp"),
    XDG_CONFIG_HOME: path.join(root, "config"), XDG_CACHE_HOME: path.join(root, "cache"),
    npm_config_cache: path.join(root, "cache", "npm"), npm_config_offline: "true",
    npm_config_audit: "false", npm_config_fund: "false", npm_config_update_notifier: "false",
    NODE_OPTIONS: "--max-old-space-size=384", GIT_TERMINAL_PROMPT: "0" };
}
function run(exe, args, options = {}) {
  const result = cp.spawnSync(exe, args, { encoding: "utf8", timeout: 120000,
    windowsHide: true, maxBuffer: 16 * 1024 * 1024, ...options });
  if (result.error) throw result.error;
  if (result.signal) fail(`${path.basename(exe)} terminated by ${result.signal}`);
  return result;
}
function requireSuccess(result, label) {
  if (result.status !== 0) fail(`${label} exited ${result.status}: ${result.stderr || result.stdout}`);
  return result;
}
function jsonContract(result, label) {
  requireSuccess(result, label);
  let value; try { value = JSON.parse(result.stdout); } catch { fail(`${label} did not return JSON`); }
  if (value.schema_version !== 1 || value.result !== "success" || typeof value.command !== "string")
    fail(`${label} returned an invalid result contract`);
  return value;
}
function tree(root) {
  const rows = [];
  function visit(dir, prefix = "") {
    for (const ent of fs.readdirSync(dir, { withFileTypes: true }).sort((a, b) => a.name.localeCompare(b.name))) {
      const rel = prefix ? `${prefix}/${ent.name}` : ent.name, file = path.join(dir, ent.name);
      if (ent.isDirectory()) visit(file, rel);
      else if (ent.isFile()) rows.push([rel, sha(fs.readFileSync(file))]);
      else fail(`generated tree contains non-regular entry: ${rel}`);
    }
  }
  visit(root); return rows;
}
function prepare(config) {
  const root = absolute("root", config.root), repo = absolute("repo", config.repo);
  cleanRoot(root);
  const head = requireSuccess(run("git", ["rev-parse", "HEAD"], { cwd: repo }), "git head").stdout.trim();
  if (head !== config.expectedHead) fail("checkout is not the expected exact candidate");
  const identity = { repository: "777genius/universal-agent-plugins", commit: head,
    versions: { agentplugins: "2.0.0", "plugin-kit-ai": "2.0.0" } };
  const common = { candidate: true, repo, identity, assetScope: "six-platform-pair",
    authoringMode: "release-cli-contract-v1", go: absolute("go", config.go), workParent: path.join(root, "work") };
  const candidate = path.join(root, "candidate");
  const built = producer.stageCandidate({ ...common, output: candidate, modCache: absolute("modCache", config.modCache) });
  const packages = path.join(root, "packages");
  const packed = packer.stagePair({ ...common, root: candidate, manifestDigest: built.manifest_sha256,
    output: packages, node: absolute("node", config.node), npm: absolute("npm", config.npm) });
  const receipt = { schema: "milestone-a-e2e-prepare/v1", candidate_head: head,
    candidate_sha256: built.manifest_sha256, asset_scope: "six-platform-pair",
    packages: packed.packs, registry_fallback: false, publication: false };
  fs.writeFileSync(path.join(root, "evidence", "prepare.json"), JSON.stringify(receipt, null, 2) + "\n");
  return receipt;
}
function consume(config) {
  const root = absolute("root", config.root), input = absolute("input", config.input);
  cleanRoot(root);
  const platform = config.platform, arch = config.arch;
  if (!TARGETS[platform]?.includes(arch)) fail("unsupported Milestone A platform lane");
  const env = baseEnv(root), node = process.execPath, npm = process.platform === "win32" ? "npm.cmd" : "npm";
  const installed = {}, reports = {}, trees = {};
  let primary;
  try {
    for (const product of PRODUCTS) {
      const prefix = path.join(root, "work", `install-${product}`);
      fs.mkdirSync(prefix);
      const packageName = product === "agentplugins" ? "universal-agent-plugins-2.0.0.tgz" : "plugin-kit-ai-2.0.0.tgz";
      requireSuccess(run(npm, ["install", "--ignore-scripts", "--offline", "--no-package-lock", "--prefix", prefix,
        path.join(input, "packages", packageName)], { env }), `install ${product}`);
      const packageDir = path.join(prefix, "node_modules", product === "agentplugins" ? "universal-agent-plugins" : product);
      installed[product] = path.join(packageDir, "bin", `${product}.js`);
    }
    for (const product of PRODUCTS) {
      const project = path.join(root, "work", `project-${product}`), client = path.join(root, "work", `client-${product}`);
      fs.mkdirSync(client);
      const argv = (args) => [installed[product], ...(product === "agentplugins" ? ["author"] : []), ...args];
      const commandEnv = { ...env, UAP_PRIVATE_NPM_CANDIDATE: path.join(input, "candidate"),
        UAP_PRIVATE_NPM_CACHE: path.join(root, "cache", product) };
      reports[product] = [];
      reports[product].push(jsonContract(run(node, argv(["init", project, "--template=skill", "--name=milestone-a-fixture",
        "--description=Disposable Milestone A fixture.", "--format=json"]), { env: commandEnv }), `${product} init`));
      for (const command of ["validate", ...(platform === "linux" ? ["inspect", "test"] : [])])
        reports[product].push(jsonContract(run(node, argv([command, project, "--format=json"]), { env: commandEnv }), `${product} ${command}`));
      if (platform === "linux") {
        const add = run(node, [installed.agentplugins, "add", ".", "--target=codex", "--dry-run", "--format=json"],
          { cwd: project, env: { ...commandEnv, HOME: client, USERPROFILE: client } });
        reports[product].push(jsonContract(add, `${product} local add --dry-run`));
      }
      trees[product] = tree(project);
    }
    if (platform === "linux" && JSON.stringify(trees.agentplugins) !== JSON.stringify(trees["plugin-kit-ai"]))
      fail("entrypoints generated different trees; no provenance exception was required or applied");
    const receipt = { schema: "milestone-a-e2e-run/v1", platform, arch, exact_candidate: true,
      entrypoints: PRODUCTS, clean_root_separation: true, commands: platform === "linux" ? COMMANDS : ["launcher-smoke", "init", "validate"],
      fixture_target: "codex", registry_fallback: false, reports, trees, cleanup: "pending" };
    fs.writeFileSync(path.join(root, "evidence", "run.json"), JSON.stringify(receipt, null, 2) + "\n");
    return receipt;
  } catch (error) {
    primary = error;
    fs.writeFileSync(path.join(root, "evidence", "failure.json"), JSON.stringify({
      schema: "milestone-a-e2e-failure/v1", platform, arch, message: String(error.message), cleanup: "pending"
    }, null, 2) + "\n");
    throw error;
  } finally {
    for (const name of ["work", "home", "tmp", "cache", "config"])
      fs.rmSync(path.join(root, name), { recursive: true, force: true });
    const file = path.join(root, "evidence", primary ? "failure.json" : "run.json");
    if (fs.existsSync(file)) { const r = JSON.parse(fs.readFileSync(file)); r.cleanup = "complete";
      fs.writeFileSync(file, JSON.stringify(r, null, 2) + "\n"); }
  }
}
function main(argv) {
  if (argv.length !== 2 || !["prepare", "run"].includes(argv[0])) fail("usage: milestone-a-e2e.js <prepare|run> <config.json>");
  const config = JSON.parse(fs.readFileSync(absolute("config", argv[1])));
  return argv[0] === "prepare" ? prepare(config) : consume(config);
}
if (require.main === module) try { process.stdout.write(JSON.stringify(main(process.argv.slice(2))) + "\n"); }
catch (error) { process.stderr.write(`Milestone A E2E: ${error.message}\n`); process.exitCode = 1; }
module.exports = { prepare, consume, main, tree, COMMANDS, TARGETS };
