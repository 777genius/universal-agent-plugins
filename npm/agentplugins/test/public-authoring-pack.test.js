"use strict";
if (process.env.AGENTPLUGINS_STAGED_TEST_CHILD === "1") {
  require("node:test")("source-checkout-only suite", { skip: "requires the complete repository source tree" }, () => {});
} else {

const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const os = require("node:os");
const cp = require("node:child_process");
const test = require("node:test");
const c = require("../scripts/dual-authoring-candidate");
const packing = require("../scripts/stage-dual-authoring-npm");
const stager = require("../scripts/stage-authoring-npm");
const { fixture, REPO } = require("./public-authoring.test");
const NODE = process.execPath;
const NPM = path.resolve(path.dirname(NODE), "../lib/node_modules/npm/bin/npm-cli.js");

// External, transport-only preload. Runtime/metadata/cache/bin modules remain
// the real packed code. The native child cannot inherit this preload.
function preload(file, bodies, log, offline = false) {
  fs.writeFileSync(file, `"use strict";
const fs = require("node:fs"), https = require("node:https");
const { EventEmitter } = require("node:events");
const { PassThrough } = require("node:stream");
const bodies = ${JSON.stringify(Object.fromEntries(Object.entries(bodies).map(([n, b]) => [n, b.toString("base64")]))) };
https.get = options => {
  const req = new EventEmitter();
  req.setTimeout = () => req;
  req.destroy = e => req.emit("error", e);
  process.nextTick(() => {
    const url = "https://" + options.hostname + options.path;
    fs.appendFileSync(${JSON.stringify(log)}, url + "\\n");
    if (${offline} || options.hostname !== "github.com" || !bodies[url]) return req.emit("error", new Error("fixture denied transport"));
    const res = new PassThrough(); res.statusCode = 200; res.headers = {};
    req.emit("response", res); res.end(Buffer.from(bodies[url], "base64"));
  }); return req;
};
`);
}
function run(exe, args, env, cwd) {
  return cp.spawnSync(exe, args, { env, cwd, encoding: "utf8", timeout: 30000, maxBuffer: 8 * 1024 * 1024 });
}
// Driver-only intent guard; this does not observe networking in native children.
const PREFLIGHT_STDERR = "agentplugins: --scope project is not supported by the current client adapters; the public CLI supports user scope only\n";
function nativeObservation(product, exe, bin, argv, env, cwd, spawn = run) {
  const bridge = require("../scripts/packed-installer-bridge");
  const rejected = ["add", path.join(cwd, "skill"), "--target=codex", "--scope=project", "--dry-run", "--format=json"];
  const same = args => JSON.stringify(args) === JSON.stringify(argv);
  if (product === "agentplugins") {
    const allowed = [["version", "--format=json"], ["--help"], ["add", "--help", "--format=json"], rejected];
    const author = [["version"], ["--help"]];
    for (const lane of bridge.LANES) {
      const source = path.join(cwd, lane);
      author.push(bridge.publicInit(lane), ["skills", "init", "extra-skill", source, "--description=Disposable fixture."],
        ["skills", "validate", source], ...["validate", "inspect", "test"].map(n => [n, source]));
    }
    allowed.push(...author.map(args => ["author", ...args, "--format=json"]));
    assert.ok(allowed.some(same), "offline native observation denied before spawn: production-security-inputs-not-offline");
  }
  const result = spawn(exe, [bin, ...argv], env, cwd);
  assert.equal(result.signal, null);
  assert.equal(result.stderr, product === "agentplugins" && same(rejected) ? PREFLIGHT_STDERR : "");
  return result;
}
if (require.main === module) test("offline native guard rejects original valid add before spawn effects", () => {
  let calls = 0;
  const source = "/disposable/agentplugins projects ü";
  const argv = ["add", path.join(source, "skill"), "--target=codex", "--dry-run", "--format=json"];
  for (const args of [argv, ["install", ...argv.slice(1)], ["--target=codex", ...argv],
    [...argv, "--help"], [...argv, "--scope=project"], ["author", "publish", "--format=json"]]) {
    assert.throws(() => nativeObservation("agentplugins", NODE, "/unused/bin", args, {}, source,
      () => { calls++; throw new Error("spawn effect"); }), /denied before spawn/);
  }
  assert.equal(calls, 0);
});
if (require.main === module) test("offline native guard allows only designated installer results", () => {
  const cwd = "/disposable/agentplugins projects ü", bin = "/unused/bin";
  for (const argv of [["add", "--help", "--format=json"],
    ["add", path.join(cwd, "skill"), "--target=codex", "--scope=project", "--dry-run", "--format=json"],
    ["author", "test", path.join(cwd, "skill"), "--format=json"]]) {
    const stderr = argv.includes("--scope=project") ? PREFLIGHT_STDERR : "";
    const invoke = result => nativeObservation("agentplugins", NODE, bin, argv, {}, cwd, (exe, args) => {
      assert.equal(exe, NODE); assert.deepEqual(args, [bin, ...argv]); return result;
    });
    assert.equal(invoke({ signal: null, stderr }).stderr, stderr);
    assert.throws(() => invoke({ signal: null, stderr: stderr + "unexpected output" }));
    assert.throws(() => invoke({ signal: "SIGTERM", stderr }));
  }
});
function ok(result) { assert.equal(result.status, 0, result.stderr || result.stdout); return result.stdout; }
function packageBytes(root) {
  const files = {};
  function walk(dir) {
    for (const name of fs.readdirSync(dir)) {
      const file = path.join(dir, name), relative = path.relative(root, file);
      if (fs.lstatSync(file).isDirectory()) walk(file);
      else files[relative] = fs.readFileSync(file);
    }
  }
  walk(root); return files;
}
function environment(context, prefix, home) {
  fs.mkdirSync(home, { recursive: true, mode: 0o700 });
  return { ...context.env, HOME: home, USERPROFILE: home, PATH: `${path.dirname(NODE)}:/usr/local/bin:/usr/bin:/bin`,
    npm_config_prefix: prefix };
}

function mainDiffersFromHead(head) {
  const name = "npm/agentplugins/scripts/stage-authoring-npm.js";
  return !fs.readFileSync(path.join(REPO, name)).equals(
    cp.execFileSync("/usr/bin/git", ["show", `${head}:${name}`], { cwd: REPO }));
}

if (require.main === module) {
  test("fresh --prepare CLI reaches the real public blob closure and source guards", () => {
    const parent = fs.mkdtempSync(path.join(os.tmpdir(), "public-cli-"));
    const head = cp.execFileSync("/usr/bin/git", ["rev-parse", "HEAD"], { cwd: REPO, encoding: "utf8" }).trim();
    const root = path.join(parent, "candidate"), workParent = path.join(parent, "work");
    const projections = path.join(parent, "projections"), outputs = {}, projectionPins = {};
    for (const dir of [root, workParent, projections]) fs.mkdirSync(dir);
    const body = Buffer.from("synthetic CLI boundary fixture\n"), digest = c.digest(body);
    for (const p of c.PRODUCTS) {
      outputs[p] = path.join(projections, p); fs.mkdirSync(outputs[p]);
      for (const name of ["release-manifest.json", "checksums.txt"]) fs.writeFileSync(path.join(outputs[p], name), body);
      projectionPins[p] = { manifest_sha256: digest, checksums_sha256: digest };
    }
    const pairMarker = path.join(projections, "pair.json");
    fs.writeFileSync(pairMarker, body); fs.writeFileSync(path.join(root, "candidate.json"), body);
    const options = { candidate: { root, workParent, outputs, pairMarker, manifestDigest: digest,
      authoringMode: "release-cli-contract-v1", assetScope: "six-platform-pair",
      identity: { repository: c.REPOSITORY, commit: head, engine_revision: head,
        versions: { agentplugins: "0.1.99", "plugin-kit-ai": "2.0.0" } } },
      repo: REPO, node: NODE, npm: NPM, output: path.join(parent, "output"), projectionPins, pairMarkerDigest: digest };
    const optionsFile = path.join(parent, "options.json"), loader = path.join(parent, "boundary.cjs");
    fs.writeFileSync(optionsFile, c.encode(options));
    // Only expensive candidate verification is stubbed. Do not preload the main:
    // the fresh process must execute its require.main branch before re-entry.
    // Stop after the real blobs call, before any projection copy or npm packing.
    fs.writeFileSync(loader, `"use strict";
const assert = require("node:assert/strict");
const adapter = require(${JSON.stringify(require.resolve("../scripts/authoring-release"))});
const packing = require(${JSON.stringify(require.resolve("../scripts/stage-dual-authoring-npm"))});
assert.equal(require.cache[${JSON.stringify(require.resolve("../scripts/stage-authoring-npm"))}], undefined);
let verified = false;
adapter.verifyAuthoringRelease = () => { verified = true; };
const blobs = packing.blobs;
packing.blobs = (...args) => {
  assert.equal(verified, true);
  assert.equal(args[3], "public");
  const source = blobs(...args);
  assert.ok(source["npm/agentplugins/scripts/stage-authoring-npm.js"]);
  throw new Error("fixture: public closure verified; stopped before packing");
};
`);
    const expected = mainDiffersFromHead(head)
      ? "executing stager differs from committed source: scripts/stage-authoring-npm.js"
      : "fixture: public closure verified; stopped before packing";
    const env = { PATH: "/usr/local/bin:/usr/bin:/bin", HOME: parent, GIT_CONFIG_NOSYSTEM: "1", GIT_CONFIG_GLOBAL: "/dev/null" };
    const invoke = () => run(NODE, ["--require", loader, path.join(REPO, "npm/agentplugins/scripts/stage-authoring-npm.js"), "--prepare", optionsFile], env, parent);
    const result = invoke();
    assert.equal(result.status, 1);
    assert.equal(result.stdout, "");
    assert.equal(result.stderr, `public npm preparation: ${expected}\n`);
    assert.equal(fs.existsSync(options.output), false);
    options.candidate.identity.commit = options.candidate.identity.engine_revision = "0".repeat(40);
    fs.writeFileSync(optionsFile, c.encode(options));
    const wrongHead = invoke();
    assert.equal(wrongHead.status, 1);
    assert.equal(wrongHead.stderr, "public npm preparation: expected source must equal checkout HEAD\n");
    assert.equal(fs.existsSync(options.output), false);
  });
  test("both real public packs install independently and together; public bins and kit postinstall use shared acquisition", () => {
    const parent = fs.mkdtempSync(path.join(os.tmpdir(), "public pack ü "));
    const context = packing.npmContext(parent);
    context.env.PATH = `${path.dirname(NODE)}:/usr/local/bin:/usr/bin:/bin`;
    const fixtures = {}, packs = {}, bodies = {};
    for (const p of c.PRODUCTS) {
      const f = fixtures[p] = fixture(p, (product, target) => Buffer.from(`#!${NODE}\n// ${product} ${target}\n` +
        'if (process.argv[2] === "signal") process.kill(process.pid, "SIGTERM");\n' +
        'else if (process.argv[2] === "exit") process.exit(23);\n' +
        'else console.log(JSON.stringify({argv:process.argv.slice(2),cwd:process.cwd(),preload:process.env.NODE_OPTIONS||null,proof:process.env.AGENTPLUGINS_INTERNAL_PROOF_BINARY||null}));\n'), parent);
      f.qualify();
      const files = packageBytes(f.packageRoot);
      const result = packing.packPackage(p, files, f.packageRoot, { node: NODE, npm: NPM, output: parent, identity: f.identity },
        { ...context, root: fs.mkdtempSync(path.join(parent, "verify-")) });
      packs[p] = path.join(parent, result.file);
      for (const [n, b] of Object.entries(f.bodies)) bodies[`https://github.com/${c.REPOSITORY}/releases/download/${f.manifest.tag}/${n}`] = b;
    }
    for (const n of stager.COMMON) assert.deepEqual(fs.readFileSync(path.join(fixtures.agentplugins.packageRoot, n)), fs.readFileSync(path.join(fixtures["plugin-kit-ai"].packageRoot, n)));
    const loader = path.join(parent, "transport.cjs"), log = path.join(parent, "transport.log");
    preload(loader, bodies, log);
    const prefixes = [path.join(parent, "agent only"), path.join(parent, "kit only"), path.join(parent, "shared ü")];
    for (const [i, prefix] of prefixes.entries()) {
      const products = i === 0 ? ["agentplugins"] : i === 1 ? ["plugin-kit-ai"] : c.PRODUCTS;
      const home = path.join(parent, `home-${i}`), project = path.join(parent, `project-${i}`);
      fs.mkdirSync(project); fs.writeFileSync(path.join(project, "unchanged"), "sentinel");
      const env = environment(context, prefix, home);
      for (const p of products) ok(run(NODE, [NPM, "install", "--global", "--prefix", prefix, "--offline", "--ignore-scripts", "--no-audit", "--no-fund", packs[p]], env, project));
      const childEnv = { ...env, NODE_OPTIONS: `--require=${JSON.stringify(loader)}`, AGENTPLUGINS_INTERNAL_PROOF_BINARY: "/nonexistent", PLUGIN_KIT_AI_VERSION: "v9.0.0" };
      for (const p of products) {
        const installed = path.join(prefix, "lib/node_modules", p === "agentplugins" ? "universal-agent-plugins" : p);
        if (p === "plugin-kit-ai") {
          const post = run(NODE, [path.join(installed, "lib/install.js")], childEnv, project);
          assert.equal(ok(post), "", "postinstall shared path is quiet");
        }
        const bin = path.join(installed, `bin/${p}.js`), args = ["space arg", "ü", "$(touch forbidden)", "a;b", "--format", "json"];
        const result = JSON.parse(ok(run(NODE, [bin, ...args], childEnv, project)));
        assert.deepEqual(result, { argv: args, cwd: project, preload: null, proof: null });
        assert.equal(run(NODE, [bin, "exit"], childEnv, project).status, 23);
        assert.equal(run(NODE, [bin, "signal"], childEnv, project).signal, "SIGTERM");
        // Exercise npm's actual installed shim too, not just a source bin.
        assert.deepEqual(JSON.parse(ok(run(path.join(prefix, "bin", p), args, childEnv, project))).argv, args);
      }
      const lines = fs.readFileSync(log, "utf8").trim().split("\n");
      for (const line of lines) assert.ok(bodies[line]);
      const offlineLoader = path.join(parent, `offline-${i}.cjs`); preload(offlineLoader, {}, log, true);
      for (const p of products) ok(run(path.join(prefix, "bin", p), ["warm"], { ...childEnv, NODE_OPTIONS: `--require=${JSON.stringify(offlineLoader)}` }, project));
      assert.equal(fs.readFileSync(path.join(project, "unchanged"), "utf8"), "sentinel");
      assert.deepEqual(fs.readdirSync(project), ["unchanged"]);
      if (i === 2) {
        const before = fs.readFileSync(path.join(prefix, "bin/plugin-kit-ai"));
        ok(run(NODE, [NPM, "uninstall", "--global", "--prefix", prefix, "--offline", "--ignore-scripts", "universal-agent-plugins"], env, project));
        assert.deepEqual(fs.readFileSync(path.join(prefix, "bin/plugin-kit-ai")), before);
        ok(run(path.join(prefix, "bin/plugin-kit-ai"), ["peer"], { ...childEnv, NODE_OPTIONS: `--require=${JSON.stringify(offlineLoader)}` }, project));
        ok(run(NODE, [NPM, "install", "--global", "--prefix", prefix, "--offline", "--ignore-scripts", packs.agentplugins], env, project));
        ok(run(path.join(prefix, "bin/agentplugins"), ["reinstalled"], { ...childEnv, NODE_OPTIONS: `--require=${JSON.stringify(offlineLoader)}` }, project));
      }
    }
  });
  test("actual public bins reject preparation, unsupported versions and legacy v2 override before effects", () => {
    for (const p of c.PRODUCTS) {
      const f = fixture(p);
      const env = { PATH: `${path.dirname(NODE)}:/usr/local/bin:/usr/bin:/bin`, HOME: f.root,
        UAP_PUBLIC_AUTHORING_VERIFIED: "true", PLUGIN_KIT_AI_VERSION: "v1.2.4" };
      const result = run(NODE, [path.join(f.packageRoot, `bin/${p}.js`), "--verified", "--format", "json"], env, f.root);
      assert.equal(result.status, 1); assert.equal(result.stdout, ""); assert.match(result.stderr, /not qualified/);
      assert.equal(result.stderr.includes("Fallbacks:"), false);
      assert.equal(fs.existsSync(path.join(f.root, ".cache")), false);
      if (p === "plugin-kit-ai") {
        fs.unlinkSync(path.join(f.packageRoot, "public-release.json"));
        assert.match(run(NODE, [path.join(f.packageRoot, "lib/install.js")], env, f.root).stderr, /requires public-release/);
        const pkgFile = path.join(f.packageRoot, "package.json"), pkg = JSON.parse(fs.readFileSync(pkgFile));
        pkg.version = "1.2.4"; fs.writeFileSync(pkgFile, c.encode(pkg));
        assert.match(run(NODE, [path.join(f.packageRoot, "lib/install.js")], { ...env, PLUGIN_KIT_AI_VERSION: "v2.0.0" }, f.root).stderr, /legacy wrapper cannot/);
      }
    }
  });
  test("public preparation cannot accept a CLI qualification boolean or substitute working source blobs", () => {
    assert.throws(() => stager.prepare({ verified: true }), /unexpected or missing fields/);
    const head = cp.execFileSync("/usr/bin/git", ["rev-parse", "HEAD"], { cwd: REPO, encoding: "utf8" }).trim();
    const root = fs.mkdtempSync(path.join(os.tmpdir(), "public blob-"));
    const context = packing.npmContext(root);
    // This gate is deliberately postcommit-sensitive. On a committed successor
    // the real closure loads; in the writer's dirty tree it must fail closed.
    const tracked = cp.spawnSync("/usr/bin/git", ["cat-file", "-e", `${head}:npm/agentplugins/scripts/stage-authoring-npm.js`], { cwd: REPO });
    if (tracked.status !== 0) assert.throws(() => packing.blobs(REPO, head, context.env, "public"), /missing|differs/);
    else if (mainDiffersFromHead(head)) assert.throws(() => packing.blobs(REPO, head, context.env, "public"),
      { message: "executing stager differs from committed source: scripts/stage-authoring-npm.js" });
    else assert.deepEqual(Object.keys(packing.blobs(REPO, head, context.env, "public")).sort(), [...stager.ALLOWLIST].sort());
    assert.throws(() => packing.blobs(REPO, "0".repeat(40), context.env, "public"), /HEAD/);
    assert.throws(() => packing.blobs(REPO, head, context.env, "arbitrary"), /fixed/);
  });
}
module.exports = { preload, run, ok, packageBytes, environment, nativeObservation, PREFLIGHT_STDERR };

if (require.main === module) test("shared completion helper preserves a collision and rolls back an uncertain marker", () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "public completion-"));
  fs.writeFileSync(path.join(root, "completion.json"), "unrelated");
  assert.throws(() => packing.completeRecord(root, { fixture: true }), /EEXIST/);
  assert.equal(fs.readFileSync(path.join(root, "completion.json"), "utf8"), "unrelated");
  const second = fs.mkdtempSync(path.join(os.tmpdir(), "public completion-"));
  const unlink = fs.unlinkSync;
  fs.unlinkSync = file => {
    if (file === path.join(second, "completion.pending.json")) throw new Error("injected marker cleanup failure");
    return unlink(file);
  };
  try { assert.throws(() => packing.completeRecord(second, { fixture: true }), /marker cleanup failure/); }
  finally { fs.unlinkSync = unlink; }
  assert.equal(fs.existsSync(path.join(second, "completion.json")), false);
  assert.equal(fs.readFileSync(path.join(second, "completion.pending.json"), "utf8"), c.encode({ fixture: true }).toString());
});
}
