"use strict";

const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const cp = require("node:child_process");
const test = require("node:test");
const c = require("../scripts/dual-authoring-candidate");
const producer = require("../scripts/stage-dual-authoring-candidate");

// Explicit opt-in ONLY for the output of a locally trusted controlled build.
// Never point this at downloaded candidates. Unlike verifyCandidate, this test
// executes the two already built Linux entrypoints in disposable fixture roots.
// Structural fixtures in the sibling test never enter this execution path.
const config = process.env.UAP_CANDIDATE_NATIVE_CONFIG;
const enabled = !!config && process.env.AGENTPLUGINS_STAGED_TEST_CHILD !== "1";

test("actual controlled Linux pair: offline verification, engine reports, frozen journeys and negative proof", {
  skip: !enabled
}, () => {
  const options = JSON.parse(c.readFile(config, 1024 * 1024));
  assert.equal(options.assetScope, "linux-amd64-pair");
  // Bind the candidate to the intended source checkout, independently of its
  // manifest. An explicit source also permits replay from a detached test copy.
  const sourceRepo = process.env.UAP_CANDIDATE_NATIVE_SOURCE_REPO || path.resolve(__dirname, "../../..");
  c.safeDirectory(sourceRepo);
  const sourceContext = producer.privateContext(options.workParent);
  const sourceHead = cp.execFileSync("/usr/bin/git", ["rev-parse", "HEAD"], {
    cwd: sourceRepo, env: sourceContext.env, encoding: "utf8"
  }).trim();
  assert.equal(options.identity.commit, sourceHead, "candidate revision must equal intended source HEAD");
  assert.deepEqual(producer.verifyCandidate(options), {
    status: "CANDIDATE", manifest_sha256: options.manifestDigest,
    consistency_verified: true, release_eligible: false, platform_acceptance: false, attested: false
  });
  const frozen = c.frozenCandidate(options.root, options.identity, options.manifestDigest, options.assetScope, options.authoringMode);
  const release = options.authoringMode === "release-cli-contract-v1";
  const before = new Map(fs.readdirSync(options.root).map((name) => [name, c.digest(c.readFile(path.join(options.root, name)))]));
  assert.equal(fs.statSync(options.root).mode & 0o222, 0);
  const contexts = c.PRODUCTS.map(() => producer.privateContext(options.workParent));
  const reports = [];
  const trees = [];
  for (let i = 0; i < frozen.binaries.length; i++) {
    const { product, binary } = frozen.binaries[i];
    const context = contexts[i];
    const executable = path.join(context.root, "bin", product);
    fs.writeFileSync(executable, binary, { flag: "wx", mode: 0o700 });
    assert.equal(c.digest(c.readFile(executable)), frozen.manifest.products[product].assets["linux-amd64"].binary.sha256);
    const productReports = [];
    const productTrees = [];
    const invokeCode = (expectedCode, ...args) => {
      const argv = [...(product === "agentplugins" ? ["author"] : []), ...args, "--format=json"];
      const result = cp.spawnSync(executable, argv, {
        cwd: context.root, env: { ...context.env, PATH: path.join(context.root, "bin") },
        timeout: 30000, encoding: "utf8"
      });
      assert.equal(result.status, expectedCode, `${product}: ${result.stdout}\n${result.stderr}`);
      assert.equal(result.stderr, "");
      let report = JSON.parse(result.stdout);
      if (release) {
        assert.equal(report.schema_version, 1); assert.equal(report.result, expectedCode === 0 ? "success" : "failure");
        assert.ok(report.command.startsWith("author")); report = report.data;
        assert.equal(report.authoring_schema_version, 1);
        assert.equal(report.engine_version, "standard-first-slice/1");
        if (report.help) report.help.use = report.help.use.replace(/^agentplugins author|^plugin-kit-ai/, "<author>");
        if (report.product) {
          assert.equal(report.product, product); assert.equal(report.product_version, options.identity.versions[product]);
          delete report.product; delete report.product_version;
        }
      }
      assert.equal(report.revision, options.identity.commit);
      assert.equal(report.engine, "standard-first-slice/1");
      assert.equal(result.stdout.includes(context.root), false);
      productReports.push({ argv, report });
      return report;
    };
    const invoke = (...args) => invokeCode(0, ...args);
    if (release) {
      invoke("version"); invoke("--help"); invoke("help", "skills", "init");
      const rootArgs = ["version", "--format=json"];
      const rootResult = cp.spawnSync(executable, rootArgs, { cwd: context.root, env: context.env, encoding: "utf8", timeout: 30000 });
      assert.equal(rootResult.status, 0); const v = JSON.parse(rootResult.stdout);
      assert.equal(v.data[product === "agentplugins" ? "version" : "product_version"], options.identity.versions[product]);
      for (const args of product === "agentplugins" ? [["author", "init", "--scope=user", "--help"], ["author", "unknown"]] :
        [["init", "--force=false", "--help"], ["skills", "ls", "--json"], ["bundle", "fetch", "--github-token=credential-fixture"], ["integrations", "sync", "--dry-run=false"]]) {
        const r = cp.spawnSync(executable, [...args, "--format=json"], { cwd: context.root, env: context.env, encoding: "utf8", timeout: 30000 });
        assert.equal(r.status, 2); assert.equal(r.stderr, ""); assert.equal(r.stdout.includes("credential-fixture"), false);
        const e = JSON.parse(r.stdout); assert.equal(e.result, "failure"); assert.deepEqual(e.data.effects, { attempted: false, committed: false });
      }
    }
    invoke("capabilities");
    for (const lane of (release ? ["skill", "mcp-remote", "mcp-stdio", "hybrid-remote", "hybrid-stdio"] : ["skill", "mcp-remote"])) {
      const template = lane.startsWith("hybrid") ? "hybrid" : lane;
      const project = path.join(context.root, lane);
      const extra = lane.endsWith("remote") ? ["--url=https://docs.example.com/mcp"] : lane.endsWith("stdio") ? ["--runtime=node"] : [];
      if (template === "hybrid") extra.push("--mcp-template=mcp-" + lane.split("-")[1]);
      const initialized = release ? invoke("init", lane, `--template=${template}`, ...extra) : invoke("init", project, `--template=${template}`, "--name=demo", "--description=Disposable candidate fixture.", ...extra);
      assert.equal(initialized.committed, true);
      for (const command of ["validate", "inspect", "test"]) {
        const report = invoke(command, project);
        assert.equal(report.runtime_evidence.status, "not_evaluated");
        assert.ok(report.identity.tree_digest);
      }
      if (release) {
        invoke("compat", project, "--target=claude,codex"); invokeCode(lane === "skill" ? 0 : 1, "doctor", project);
        invoke("skills", "validate", project);
        assert.equal(invoke("skills", "init", "extra-skill", project, "--description=Disposable fixture.").committed, true);
        invoke("skills", "validate", project);
      }
      const tree = [];
      const walk = (relative) => {
        for (const name of fs.readdirSync(path.join(project, relative)).sort()) {
          const child = path.join(relative, name);
          const file = path.join(project, child);
          const stat = fs.lstatSync(file);
          assert.equal(stat.isSymbolicLink(), false);
          if (stat.isDirectory()) walk(child);
          else tree.push({ path: child, mode: stat.mode & 0o777, sha256: c.digest(c.readFile(file)) });
        }
      };
      walk(""); productTrees.push(tree);
    }
    reports.push(productReports); trees.push(productTrees);
  }
  assert.deepEqual(reports[0].map((r) => r.report), reports[1].map((r) => r.report));
  assert.deepEqual(trees[0], trees[1]);
  for (const [name, hash] of before) {
    assert.equal(c.digest(c.readFile(path.join(options.root, name))), hash);
    assert.equal(fs.statSync(path.join(options.root, name)).mode & 0o222, 0);
  }
  // Swap the two *real* mains and repin all hashes. Structure still looks valid;
  // embedded Go main identity must fail. Verification never executes either file.
  const badRoot = path.join(contexts[0].root, "swapped"); fs.mkdirSync(badRoot);
  const bad = structuredClone(frozen.manifest);
  for (let i = 0; i < c.PRODUCTS.length; i++) {
    const product = c.PRODUCTS[i];
    const asset = bad.products[product].assets["linux-amd64"];
    const wrongBinary = frozen.binaries[1 - i].binary;
    const body = product === "plugin-kit-ai" ? c.archive(wrongBinary, asset.binary.file) : wrongBinary;
    Object.assign(asset, c.metadata(body)); Object.assign(asset.binary, c.metadata(wrongBinary));
    fs.writeFileSync(path.join(badRoot, asset.file), body);
  }
  const badBody = c.encode(bad); fs.writeFileSync(path.join(badRoot, "candidate.json"), badBody);
  const badOptions = { ...options, root: badRoot, manifestDigest: c.digest(badBody) };
  assert.equal(c.frozenCandidate(badRoot, options.identity, badOptions.manifestDigest, options.assetScope, options.authoringMode).binaries.length, 2);
  assert.throws(() => producer.verifyCandidate(badOptions), /embedded Go product/);
  // Repinned real bytes must still reject changed version, engine or known mode.
  for (const change of ["version", "revision", "mode"]) {
    const root = path.join(contexts[0].root, "repinned-" + change); fs.mkdirSync(root);
    const m = structuredClone(frozen.manifest);
    if (change === "version") { m.identity.versions["plugin-kit-ai"] = "2.0.91"; m.products["plugin-kit-ai"].version = "2.0.91"; }
    if (change === "revision") m.identity.commit = m.identity.engine_revision = "b".repeat(40);
    if (change === "mode") m.build.authoring_mode = release ? "vertical-slice-v1" : "release-cli-contract-v1";
    for (const product of c.PRODUCTS) {
      const asset = m.products[product].assets["linux-amd64"];
      const original = frozen.manifest.products[product].assets["linux-amd64"].file;
      asset.file = c.assetName(product, m.identity.versions[product], "linux-amd64");
      fs.writeFileSync(path.join(root, asset.file), c.readFile(path.join(options.root, original)));
    }
    const body = c.encode(m); fs.writeFileSync(path.join(root, "candidate.json"), body);
    const repinned = { ...options, root, identity: m.identity, manifestDigest: c.digest(body), authoringMode: m.build.authoring_mode };
    c.frozenCandidate(root, repinned.identity, repinned.manifestDigest, options.assetScope, repinned.authoringMode);
    assert.throws(() => producer.verifyCandidate(repinned), /embedded Go build setting mismatch/);
  }
  // A build failure cannot produce a success marker or overwrite existing output.
  const emptyModules = path.join(contexts[0].root, "empty-modules"); fs.mkdirSync(emptyModules);
  const failureRoot = fs.mkdtempSync(path.join(path.dirname(options.workParent), "candidate-failure-"));
  const failedOutput = path.join(failureRoot, "failed-build");
  const stageOptions = { candidate: true, repo: sourceRepo, output: failedOutput,
    modCache: emptyModules, workParent: options.workParent, go: options.go, identity: options.identity, assetScope: options.assetScope, ...(options.authoringMode ? { authoringMode: options.authoringMode } : {}) };
  const wrongCommit = (sourceHead[0] === "0" ? "1" : "0") + sourceHead.slice(1);
  assert.throws(() => producer.stageCandidate({ ...stageOptions,
    identity: { ...options.identity, commit: wrongCommit, engine_revision: wrongCommit }
  }), /declared source commit must equal checkout HEAD/);
  assert.equal(fs.existsSync(failedOutput), false);
  assert.throws(() => producer.stageCandidate(stageOptions), /Command failed/);
  assert.equal(fs.statSync(failedOutput).isDirectory(), true);
  assert.equal(fs.existsSync(path.join(failedOutput, "candidate.json")), false);
  // Existing unrelated bytes survive rejection before any compiler invocation.
  const unrelated = path.join(failureRoot, "unrelated"); fs.mkdirSync(unrelated);
  fs.writeFileSync(path.join(unrelated, "sentinel"), "preserve");
  assert.throws(() => producer.stageCandidate({ ...stageOptions, output: unrelated }), /already exists|overlaps/);
  assert.equal(fs.readFileSync(path.join(unrelated, "sentinel"), "utf8"), "preserve");
  fs.writeFileSync(path.join(contexts[0].root, "native-journey.json"), c.encode({
    kind: "local-linux-candidate-journey", identity: options.identity, manifest_sha256: options.manifestDigest,
    reports, trees, platform_acceptance: false, attested: false
  }), { flag: "wx", mode: 0o444 });
  process.stdout.write(`native journey evidence: ${path.join(contexts[0].root, "native-journey.json")}\n`);
});
