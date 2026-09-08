"use strict";

// Dedicated lane: missing configuration is a failure, never a skipped acceptance.
// Run only after the coordinator commits/integrates A and freezes native assets
// from THAT exact source. Synthetic package binding proves fixture execution,
// not authenticated native promotion, npm signatures or public eligibility.
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const crypto = require("node:crypto");
const test = require("node:test");
const c = require("../scripts/dual-authoring-candidate");
const bridge = require("../scripts/packed-installer-bridge");
const adapter = require("../scripts/authoring-release");
const packing = require("../scripts/stage-dual-authoring-npm");
const stager = require("../scripts/stage-authoring-npm");
const { preload, run, ok, environment, nativeObservation } = require("./public-authoring-pack.test");

function tree(root) {
  const result = {};
  function walk(dir) {
    for (const name of fs.readdirSync(dir).sort()) {
      const file = path.join(dir, name), stat = fs.lstatSync(file);
      assert.ok(!stat.isSymbolicLink(), "project aliases forbidden");
      if (stat.isDirectory()) walk(file);
      else result[path.relative(root, file)] = c.digest(c.readFile(file));
    }
  }
  walk(root); return result;
}

test("PUBLIC NATIVE: exact integrated packs, both actual bins, static parity and lifecycle", t => {
  const config = process.env.UAP_PUBLIC_AUTHORING_NATIVE_CONFIG;
  assert.ok(config && path.isAbsolute(config), "required UAP_PUBLIC_AUTHORING_NATIVE_CONFIG: exact integrated assets remain a coordinator gate");
  const cfg = JSON.parse(c.readFile(config, 1024 * 1024));
  c.keys(cfg, ["prepare", "completionDigest", "evidenceOutput"], "public native fixture config");
  const o = cfg.prepare, candidate = o.candidate;
  const platform = require("../lib/platform").detectPlatform();
  const target = platform.key;
  assert.equal(target, "linux-amd64", "public packed intake is the bounded Linux lane");
  c.outputPlacement(cfg.evidenceOutput, [o.repo, o.output, candidate.root, candidate.workParent]);
  const context = packing.npmContext(candidate.workParent);
  const env = { ...context.env, PATH: `${path.dirname(o.node)}:/usr/local/bin:/usr/bin:/bin` };
  const source = packing.blobs(o.repo, candidate.identity.commit, env, "public");
  adapter.verifyAuthoringRelease(candidate);
  const completion = c.readFile(path.join(o.output, "completion.json"));
  assert.equal(c.digest(completion), cfg.completionDigest);
  const record = JSON.parse(completion);
  assert.deepEqual(record.identity, candidate.identity);
  assert.equal(record.candidate_sha256, candidate.manifestDigest);
  assert.equal(record.pair_marker_sha256, o.pairMarkerDigest);
  assert.equal(c.digest(c.readFile(candidate.pairMarker)), o.pairMarkerDigest);
  assert.deepEqual(record.projection_pins, o.projectionPins);
  assert.equal(record.qualification, null);
  for (const claim of ["release_eligible", "platform_acceptance", "attested"]) assert.equal(record[claim], false);
  assert.deepEqual(record.wrapper_blobs, Object.fromEntries(Object.entries(source).map(([n, { bytes, ...pin }]) => [n, pin])));
  fs.mkdirSync(cfg.evidenceOutput, { mode: 0o700 });
  const bodies = {}, tarballs = {}, packs = {}, binaries = {}, installations = [], invocations = [], reports = {}, trees = {};
  const integrity = bytes => "sha512-" + crypto.createHash("sha512").update(bytes).digest("base64");
  for (const p of c.PRODUCTS) {
    const manifestBytes = c.readFile(path.join(candidate.outputs[p], "release-manifest.json"));
    assert.equal(c.digest(manifestBytes), o.projectionPins[p].manifest_sha256);
    assert.equal(c.digest(c.readFile(path.join(candidate.outputs[p], "checksums.txt"))), o.projectionPins[p].checksums_sha256);
    const files = stager.packageFiles(p, source, manifestBytes, candidate);
    assert.deepEqual(record.generated[p], Object.fromEntries(Object.entries(files).map(([n, b]) => [n, c.digest(b)])));
    const pin = record.packs[p], tarball = path.join(o.output, pin.file), bytes = c.readFile(tarball);
    assert.equal(c.digest(bytes), pin.sha256); assert.equal(bytes.length, pin.size); assert.equal(integrity(bytes), pin.integrity);
    const extract = path.join(context.root, `preparation-${p}`);
    packing.verifyPack(tarball, files, extract, env);
    const root = path.join(extract, "package");
    // Prove unchanged ordinary preparation rejects, then assemble separately
    // labelled synthetic packs. No source_sha rewriting or altered public code.
    const denied = run(o.node, [path.join(root, `bin/${p}.js`), "version"], env, context.root);
    assert.equal(denied.status, 1); assert.match(denied.stderr, /not qualified/); assert.equal(denied.stdout, "");
    const d = JSON.parse(files["public-release.json"]);
    d.qualification = { identity: candidate.identity, candidate_sha256: candidate.manifestDigest,
      manifest_sha256: Object.fromEntries(c.PRODUCTS.map(p => [p, o.projectionPins[p].manifest_sha256])),
      signed_subject: { sha256: c.digest(Buffer.from("synthetic fixture only")),
        workflow: `${c.REPOSITORY}/.github/workflows/fixture-only.yml`, source: candidate.identity.commit } };
    files["public-release.json"] = c.encode(d);
    fs.writeFileSync(path.join(root, "public-release.json"), files["public-release.json"]);
    const output = path.join(cfg.evidenceOutput, p); fs.mkdirSync(output);
    const packed = packing.packPackage(p, files, root, { ...o, output, identity: candidate.identity },
      { env, root: fs.mkdtempSync(path.join(context.root, "synthetic-pack-")) });
    tarballs[p] = path.join(output, packed.file);
    packs[p] = { ...packed, file: tarballs[p] };
    const m = JSON.parse(manifestBytes), a = m.assets[target];
    const body = c.readFile(path.join(candidate.outputs[p], a.file));
    assert.equal(c.digest(body), a.sha256);
    bodies[`https://github.com/${c.REPOSITORY}/releases/download/${m.tag}/${a.file}`] = body;
  }
  const loader = path.join(context.root, "transport.cjs"), log = path.join(cfg.evidenceOutput, "downloads.log");
  preload(loader, bodies, log);
  function install(p, prefix, env) {
    const argv = [o.npm, "install", "--global", "--prefix", prefix, "--offline", "--ignore-scripts", "--no-audit", "--no-fund", tarballs[p]];
    const result = run(o.node, argv, env, context.root);
    installations.push({ product: p, argv, status: result.status, signal: result.signal, pack_sha256: c.digest(c.readFile(tarballs[p])) });
    ok(result);
  }
  // Separate prefixes and homes prove neither native product needs its peer.
  for (const p of c.PRODUCTS) {
    const prefix = path.join(context.root, `${p} independent prefix`);
    const independent = environment({ ...context, env }, prefix, path.join(context.root, `${p} independent home`));
    install(p, prefix, independent);
    const packageRoot = path.join(prefix, "lib/node_modules", p === "agentplugins" ? "universal-agent-plugins" : p);
    const controlled = { ...independent, NODE_OPTIONS: `--require=${JSON.stringify(loader)}`, PATH: "/absent-peer-binary-path" };
    if (p === "plugin-kit-ai") assert.equal(ok(run(o.node, [path.join(packageRoot, "lib/install.js")], controlled, context.root)), "");
    const version = JSON.parse(ok(run(o.node, [path.join(packageRoot, `bin/${p}.js`), "version", "--format=json"], controlled, context.root)));
    assert.equal(version.data[p === "agentplugins" ? "version" : "product_version"], candidate.identity.versions[p]);
    assert.deepEqual(fs.readdirSync(path.join(prefix, "lib/node_modules")), [p === "agentplugins" ? "universal-agent-plugins" : p]);
  }
  const prefix = path.join(context.root, "shared prefix ü"), home = path.join(context.root, "consumer home ü");
  const consumer = environment({ ...context, env }, prefix, home);
  for (const p of c.PRODUCTS) install(p, prefix, consumer);
  const client = path.join(home, ".codex"), installer = path.join(context.root, "installer-state");
  fs.mkdirSync(client); fs.mkdirSync(installer);
  fs.writeFileSync(path.join(client, "config.toml"), "# isolated synthetic target\n");
  const transportEnv = { ...consumer, CODEX_HOME: client, AGENTPLUGINS_HOME: installer,
    NODE_OPTIONS: `--require=${JSON.stringify(loader)}` };
  const bin = p => path.join(prefix, "lib/node_modules", p === "agentplugins" ? "universal-agent-plugins" : p, `bin/${p}.js`);
  const projects = Object.fromEntries(c.PRODUCTS.map(p => {
    const root = path.join(context.root, `${p} projects ü`); fs.mkdirSync(root); return [p, root];
  }));
  function invoke(p, argv, status = 0, json = true) {
    const r = nativeObservation(p, o.node, bin(p), argv, { ...transportEnv, PATH: "/absent-peer-binary-path" }, projects[p]);
    invocations.push({ product: p, argv, status: r.status, signal: r.signal, stdout: r.stdout, stderr: r.stderr });
    fs.writeFileSync(path.join(cfg.evidenceOutput, "invocations.json"), c.encode(invocations));
    assert.equal(r.status, status, r.stdout + r.stderr);
    return json ? JSON.parse(r.stdout) : r.stdout;
  }
  function author(p, argv, status = 0, compare = true) {
    const r = invoke(p, [...(p === "agentplugins" ? ["author"] : []), ...argv, "--format=json"], status);
    assert.equal(r.schema_version, 1); assert.equal(r.data.revision, candidate.identity.commit);
    assert.equal(r.data.engine, "standard-first-slice/1");
    assert.equal(r.result, status === 0 ? "success" : "failure");
    const normalized = JSON.parse(JSON.stringify(r).split(projects[p]).join("<project>"));
    delete normalized.data.product; delete normalized.data.product_version;
    if (normalized.data.help) normalized.data.help.use = normalized.data.help.use.replace(/^agentplugins author|^plugin-kit-ai/, "<author>");
    if (compare) reports[p].push(normalized);
    return r.data;
  }
  ok(run(o.node, [path.resolve(bin("plugin-kit-ai"), "../../lib/install.js")], transportEnv, context.root));
  for (const p of c.PRODUCTS) {
    reports[p] = []; trees[p] = [];
    const version = invoke(p, ["version", "--format=json"]);
    assert.equal(version.data[p === "agentplugins" ? "version" : "product_version"], candidate.identity.versions[p]);
    assert.ok(invoke(p, ["--help"], 0, false).length);
    author(p, ["version"]);
    const help = author(p, ["--help"]);
    for (const name of ["dev", "bootstrap", "normalize", "import", "export", "publish"]) {
      assert.ok(!JSON.stringify(help).includes(`"${name}"`), `deferred ${name} absent`);
    }
    for (const lane of bridge.LANES) {
      assert.equal(author(p, bridge.publicInit(lane)).committed, true);
      const project = path.join(projects[p], lane);
      assert.equal(author(p, ["skills", "init", "extra-skill", project, "--description=Disposable fixture."]).committed, true);
      author(p, ["skills", "validate", project]);
      for (const command of ["validate", "inspect", "test"]) {
        const d = author(p, [command, project]);
        assert.equal(d.runtime_evidence.status, "not_evaluated"); assert.ok(d.identity.tree_digest);
      }
      trees[p].push(tree(project));
    }
  }
  assert.deepEqual(fs.readdirSync(installer), [], "authoring never creates installer state");
  assert.equal(fs.readFileSync(path.join(client, "config.toml"), "utf8"), "# isolated synthetic target\n");
  assert.deepEqual(reports.agentplugins, reports["plugin-kit-ai"]);
  assert.deepEqual(trees.agentplugins, trees["plugin-kit-ai"]);
  const before = tree(projects["plugin-kit-ai"]);
  const retired = invoke("plugin-kit-ai", ["update", "--all", "--format=json"], 2);
  assert.equal(retired.data.error.code, "v1_operation_unavailable");
  assert.deepEqual(tree(projects["plugin-kit-ai"]), before);
  // Genuine executable visibility/preflight only; valid production add remains required.
  const preservedRoots = [...Object.values(projects), client, installer];
  const preserved = preservedRoots.map(root => bridge.snapshot(root));
  const help = invoke("agentplugins", ["add", "--help", "--format=json"]);
  bridge.installerHelp(help);
  assert.deepEqual(preservedRoots.map(root => bridge.snapshot(root)), preserved);
  assert.equal(invoke("agentplugins", ["add", path.join(projects.agentplugins, "skill"), "--target=codex",
    "--scope=project", "--dry-run", "--format=json"], 1, false), "");
  assert.deepEqual(preservedRoots.map(root => bridge.snapshot(root)), preserved);
  const installer_boundary = bridge.installerBoundary(projects.agentplugins);
  const runtime = require("../lib/public-authoring");
  for (const p of c.PRODUCTS) {
    const packageRoot = path.resolve(bin(p), "../..");
    const release = runtime.loadRelease(p, packageRoot, target);
    const cached = runtime.cachePath(path.join(home, ".cache", "universal-agent-plugins"), p, target, release);
    fs.writeFileSync(cached, "corrupt fixture cache");
    ok(run(o.node, [bin(p), "version", "--format=json"], transportEnv, projects[p]));
    assert.equal(c.digest(c.readFile(cached)), release.asset.binary.sha256);
    binaries[p] = { path: cached, sha256: release.asset.binary.sha256, size: c.readFile(cached).length };
  }
  const offline = path.join(context.root, "offline.cjs"); preload(offline, {}, log, true);
  const offlineEnv = { ...transportEnv, NODE_OPTIONS: `--require=${JSON.stringify(offline)}` };
  for (const p of c.PRODUCTS) ok(run(o.node, [bin(p), "version", "--format=json"], offlineEnv, projects[p]));
  const peerBytes = fs.readFileSync(bin("plugin-kit-ai"));
  ok(run(o.node, [o.npm, "uninstall", "--global", "--prefix", prefix, "--offline", "--ignore-scripts", "universal-agent-plugins"], consumer, context.root));
  assert.deepEqual(fs.readFileSync(bin("plugin-kit-ai")), peerBytes);
  ok(run(o.node, [bin("plugin-kit-ai"), "version"], offlineEnv, projects["plugin-kit-ai"]));
  install("agentplugins", prefix, consumer);
  ok(run(o.node, [bin("agentplugins"), "version"], offlineEnv, projects.agentplugins));
  assert.deepEqual(tree(projects["plugin-kit-ai"]), before);
  assert.equal(fs.readFileSync(log, "utf8").trim().split("\n").length, 6, "independent/shared cold and corruption recovery downloads per product; warm is offline");
  fs.writeFileSync(path.join(cfg.evidenceOutput, "result.json"), c.encode({ source: candidate.identity.commit,
    fixture_acquisition_execution: true, signed_promotion: false, public_eligible: false, runtime_evidence: "not_evaluated" }), { flag: "wx" });
  for (const p of c.PRODUCTS) assert.deepEqual(bridge.LANES.map(lane => tree(path.join(projects[p], lane))), trees[p], "all generated projects preserved through lifecycle");
  const terminal = { schema: "dual-authoring-public-native/v2", installer_boundary, status: "completed",
    identity: candidate.identity, candidate_sha256: candidate.manifestDigest, completion_sha256: cfg.completionDigest,
    config_sha256: c.digest(c.readFile(config)), pair_marker_sha256: o.pairMarkerDigest, projection_pins: o.projectionPins,
    fixtureRoot: context.root, target, packs, binaries, installations,
    tools: { ...record.tools, go: { path: candidate.go, sha256: c.digest(c.readFile(candidate.go)) },
      producer_node: { path: process.execPath, sha256: c.digest(c.readFile(process.execPath)), version: process.version } },
    invocations: invocations.length, invocations_sha256: c.digest(c.readFile(path.join(cfg.evidenceOutput, "invocations.json"))),
    downloads_sha256: c.digest(c.readFile(log)), result_sha256: c.digest(c.readFile(path.join(cfg.evidenceOutput, "result.json"))),
    projects, trees: Object.fromEntries(c.PRODUCTS.map(p => [p, bridge.snapshot(projects[p])])),
    fixture_acquisition_execution: true, qualification: null, signed_promotion: false, public_eligible: false,
    release_eligible: false, platform_acceptance: false, attested: false, runtime_evidence: "not_evaluated" };
  // Validate the full contract before exclusive terminal publication. No success
  // marker exists if a lane, pin, or lifecycle assertion above failed.
  bridge.publicEvidence(cfg, terminal, config, "public-fixture/v2");
  const terminalPath = path.join(cfg.evidenceOutput, "public-native-completion.json");
  fs.writeFileSync(terminalPath, c.encode(terminal), { flag: "wx", mode: 0o600 });
  t.diagnostic("public native completion: " + JSON.stringify({ file: terminalPath, sha256: c.digest(c.readFile(terminalPath)), source: candidate.identity.commit }));
});
