"use strict";
// Root-controlled opt-in only. No build, provisioning or structural executable
// enters this suite. Config pins a completed pair from the FINAL integrated SHA.
const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const cp = require("node:child_process");
const crypto = require("node:crypto");
const c = require("../scripts/dual-authoring-candidate");
const producer = require("../scripts/stage-dual-authoring-candidate");
const s = require("../scripts/stage-dual-authoring-npm");
const b = require("../scripts/private-npm/bootstrap");
const config = process.env.UAP_PRIVATE_NPM_NATIVE_CONFIG;
const mkdir = p => { fs.mkdirSync(p, { mode: 0o700 }); return p; };
const read = p => c.readFile(p, 1024 * 1024);
function tree(root, relative = "") {
  return fs.readdirSync(path.join(root, relative)).sort().flatMap(n => {
    const name = path.join(relative, n), file = path.join(root, name), stat = fs.lstatSync(file);
    assert.equal(stat.isSymbolicLink(), false);
    return stat.isDirectory() ? tree(root, name) : [{ path: name, mode: stat.mode & 0o777, sha256: c.digest(fs.readFileSync(file)) }];
  });
}

test("NATIVE opt-in: exact two Linux tarballs, five accepted template lanes and complete release journeys", { skip: !config }, () => {
  assert.equal(process.platform, "linux"); assert.equal(process.arch, "x64");
  const cfg = JSON.parse(read(config));
  c.keys(cfg, ["stage", "completionDigest", "evidenceOutput"], "native npm config");
  const o = cfg.stage; assert.equal(o.authoringMode, s.MODE); assert.equal(o.assetScope, "linux-amd64-pair");
  c.outputPlacement(cfg.evidenceOutput, [o.repo, o.root, o.output, o.workParent]);
  const context = s.npmContext(o.workParent), env = context.env;
  const run = (exe, argv, cwd = context.root, extraEnv = {}) => cp.spawnSync(exe, argv, {
    env: { ...env, ...extraEnv }, cwd, encoding: "utf8", timeout: 30000, maxBuffer: 8 * 1024 * 1024
  });
  const checked = (exe, argv, cwd) => { const r = run(exe, argv, cwd); assert.equal(r.status, 0, `${r.error || ""}\n${r.stderr}`); return r.stdout; };
  const source = s.blobs(o.repo, o.identity.commit, env); // also checks executing stager identity
  const completionBytes = read(path.join(o.output, "completion.json"));
  assert.equal(c.digest(completionBytes), cfg.completionDigest);
  const record = JSON.parse(completionBytes); assert.deepEqual(record.identity, o.identity);
  assert.equal(record.candidate_sha256, o.manifestDigest); assert.equal(record.authoring_mode, s.MODE);
  for (const claim of ["release_eligible", "platform_acceptance", "attested"]) assert.equal(record[claim], false);
  producer.verifyCandidate({ candidate: true, root: o.root, identity: o.identity, manifestDigest: o.manifestDigest,
    assetScope: o.assetScope, authoringMode: o.authoringMode, go: o.go, workParent: o.workParent });
  const frozen = c.frozenCandidate(o.root, o.identity, o.manifestDigest, o.assetScope, o.authoringMode);
  const candidateBefore = tree(o.root), candidateMode = fs.statSync(o.root).mode;
  const wrappers = {}, prefixes = {}, caches = {}, projects = {}, reports = {}, trees = {}, invocations = [];
  mkdir(cfg.evidenceOutput);
  for (const product of c.PRODUCTS) {
    const files = s.packageFiles(product, source, read(path.join(o.root, "candidate.json")), o);
    const pin = record.packs[product], tarball = path.join(o.output, pin.file), bytes = c.readFile(tarball);
    assert.equal(c.digest(bytes), pin.sha256); assert.equal(bytes.length, pin.size);
    assert.equal("sha512-" + crypto.createHash("sha512").update(bytes).digest("base64"), pin.integrity);
    assert.deepEqual(record.generated[product], Object.fromEntries(Object.entries(files).map(([n, bytes]) => [n, c.digest(bytes)])));
    s.verifyPack(tarball, files, path.join(context.root, product + "-extract"), env);
    const prefix = mkdir(path.join(context.root, product + " isolated prefix")); prefixes[product] = prefix;
    checked(o.node, [o.npm, "install", "--prefix", prefix, "--ignore-scripts", "--offline", "--no-audit", "--no-fund", "--package-lock=false", tarball], prefix);
    wrappers[product] = path.join(prefix, "node_modules", s.PACKAGES[product], "bin", product + ".js");
    caches[product] = mkdir(path.join(context.root, product + " cache"));
    projects[product] = mkdir(path.join(context.root, product + " disposable projects"));
    reports[product] = []; trees[product] = [];
    assert.deepEqual(fs.readdirSync(path.join(prefix, "node_modules")).filter(n => !n.startsWith(".")), [s.PACKAGES[product]]);
  }
  function invoke(product, argv, code = 0, json = true) {
    const result = run(o.node, [wrappers[product], ...argv], projects[product], {
      PATH: "/absent-native-fixture-path", UAP_PRIVATE_NPM_CACHE: caches[product], UAP_PRIVATE_NPM_CANDIDATE: o.root,
      BASH_COMP_DEBUG_FILE: path.join(context.root, "forbidden-completion-debug")
    });
    invocations.push({ product, argv, status: result.status, signal: result.signal, stdout: result.stdout, stderr: result.stderr });
    // Retain partial evidence after every actual process, including a failed oracle.
    fs.writeFileSync(path.join(cfg.evidenceOutput, "invocations.json"), c.encode(invocations));
    assert.equal(result.status, code, `${product} ${argv.join(" ")}: ${result.stdout}\n${result.stderr}`);
    assert.equal(result.stderr, ""); assert.equal(result.stdout.includes("credential-fixture"), false);
    assert.equal(fs.existsSync(path.join(context.root, "forbidden-completion-debug")), false);
    return json ? JSON.parse(result.stdout) : result.stdout;
  }
  function author(product, args, code = 0, shared = true) {
    const r = invoke(product, [...(product === "agentplugins" ? ["author"] : []), ...args, "--format=json"], code);
    assert.equal(r.schema_version, 1); assert.equal(r.result, code === 0 ? "success" : "failure");
    assert.ok(r.command.startsWith("author")); const d = r.data;
    assert.equal(d.revision, o.identity.commit); assert.equal(d.engine, "standard-first-slice/1");
    assert.equal(d.engine_version, "standard-first-slice/1"); assert.equal(d.authoring_schema_version, 1);
    if (d.product) {
      assert.equal(d.product, product); assert.equal(d.product_version, o.identity.versions[product]);
    }
    // Only accepted presentation identities differ. All policies, diagnostics,
    // engine fields and tree/manifest digests remain exact comparison inputs.
    const compare = structuredClone(r);
    if (compare.data.product) { delete compare.data.product; delete compare.data.product_version; }
    if (compare.data.help) compare.data.help.use = compare.data.help.use.replace(/^agentplugins author|^plugin-kit-ai/, "<author>");
    if (shared) reports[product].push(compare);
    if (code === 2) assert.deepEqual(d.effects, { attempted: false, committed: false });
    return d;
  }
  for (const product of c.PRODUCTS) {
    const version = invoke(product, ["version", "--format=json"]);
    assert.equal(version.data[product === "agentplugins" ? "version" : "product_version"], o.identity.versions[product]);
    const rootHelp = invoke(product, ["--help"], 0, false); assert.ok(rootHelp.length > 0);
    author(product, ["version"]); author(product, ["capabilities"]);
    for (const command of [[], ["init"], ["validate"], ["inspect"], ["compat"], ["doctor"], ["test"], ["skills"], ["skills", "init"], ["skills", "validate"], ["version"]]) {
      author(product, [...command, "--help"]);
    }
    for (const lane of ["skill", "mcp-remote", "mcp-stdio", "hybrid-remote", "hybrid-stdio"]) {
      const template = lane.startsWith("hybrid") ? "hybrid" : lane;
      const extra = lane.endsWith("remote") ? ["--url=https://docs.example.com/mcp"] : lane.endsWith("stdio") ? ["--runtime=node"] : [];
      if (template === "hybrid") extra.push("--mcp-template=mcp-" + lane.split("-")[1]);
      assert.equal(author(product, ["init", lane, `--template=${template}`, ...extra]).committed, true);
      const project = path.join(projects[product], lane);
      for (const command of ["validate", "inspect", "test"]) {
        const d = author(product, [command, project]); assert.equal(d.runtime_evidence.status, "not_evaluated"); assert.ok(d.identity.tree_digest);
      }
      author(product, ["compat", project, "--target=claude,codex"]);
      author(product, ["doctor", project], lane === "skill" ? 0 : 1);
      author(product, ["skills", "validate", project]);
      assert.equal(author(product, ["skills", "init", "extra-skill", project, "--description=Disposable fixture."]).committed, true);
      author(product, ["skills", "validate", project]);
      const before = tree(project);
      author(product, ["init", lane, "--template=skill"], 1);
      assert.deepEqual(tree(project), before);
      trees[product].push(before);
    }
    for (const args of [["unknown"], ["version", "extra"], ["init", "--scope=user", "--help"], ["init", "--force=false", "--help"],
      ["test", ".", "--handshake"], ["init", "--template=mcp-remote"], ["skills", "init", "--description=version"], ["capabilities", "--unknown=credential-fixture"]]) author(product, args, 2, false);
    // Policy/I/O failure remains distinct from argument/v1 errors.
    author(product, ["validate", "absent-fixture"], 1, false);
    for (const protocol of ["__complete", "__completeNoDesc"]) {
      const prefix = product === "agentplugins" ? ["author"] : [];
      assert.equal(invoke(product, [protocol, ...prefix, "init", "--description", "--=x"], 0, false), ":0\n");
      invoke(product, [protocol, ...prefix, "--unknown=credential-fixture", ""], 2, false);
    }
    for (const shell of ["bash", "zsh", "fish", "powershell"]) assert.ok(invoke(product, ["completion", shell], 0, false).length > 0);
    const packageRoot = path.dirname(path.dirname(wrappers[product]));
    const release = b.loadRelease(product, packageRoot, "linux-amd64", s.MODE);
    assert.equal(c.digest(c.readFile(b.cachePath(caches[product], product, "linux-amd64", release))), frozen.manifest.products[product].assets["linux-amd64"].binary.sha256);
  }
  assert.deepEqual(reports.agentplugins, reports["plugin-kit-ai"]); assert.deepEqual(trees.agentplugins, trees["plugin-kit-ai"]);
  // Reuse B's exact committed inventory as data; do not maintain a second list.
  const inventory = checked("/usr/bin/git", ["show", `${o.identity.commit}:cli/plugin-kit-ai/cmd/plugin-kit-ai/release_compat.go`], o.repo);
  const rows = [...inventory.matchAll(/\{"([^"]+)", "([^"]*)", (true|false)\}/g)]; assert.ok(rows.length >= 50);
  for (const [, command, flags, retained] of rows) if (retained === "false") {
    const tails = [[], ["--help"], ["--unknown=credential-fixture", "--help"]];
    for (const flag of flags.split(" ").filter(Boolean)) {
      const [spec, short] = flag.split("/"), boolean = spec.endsWith("!"), name = spec.replace(/!$/, "");
      const value = boolean ? "false" : "credential-fixture";
      tails.push([`--${name}=${value}`], boolean ? [`--${name}`] : [`--${name}`, value]);
      if (short) tails.push([`-${short}=${value}`]);
    }
    for (const tail of tails) {
      const r = invoke("plugin-kit-ai", ["--format=json", ...command.split(" "), ...tail], 2);
      assert.equal(r.result, "failure"); assert.deepEqual(r.data.effects, { attempted: false, committed: false });
      assert.equal(r.data.normative_conformance.status, "not_evaluated");
    }
  }
  // Replay A's committed parser matrix directly, keeping its expected statuses
  // and operation IDs. This is packed-process proof of the same accepted data.
  const parserSource = checked("/usr/bin/git", ["show", `${o.identity.commit}:cli/plugin-kit-ai/internal/authoring/commands/public_contract_test.go`], o.repo);
  const parserBlock = parserSource.split("func TestPublicParserAndHelpParity")[1].split("for _, tc := range cases")[0];
  const parserRows = [...parserBlock.matchAll(/\{(?:\[\]string\{([^}]*)\}|nil), "(author[^"]*)", ([012])\}/g)];
  assert.ok(parserRows.length >= 30);
  for (const [, tokens, operation, status] of parserRows) for (const product of c.PRODUCTS) {
    const args = tokens ? JSON.parse("[" + tokens + "]") : [];
    const r = invoke(product, [...(product === "agentplugins" ? ["author"] : []), "--format=json", ...args], Number(status));
    assert.equal(r.command, operation); assert.equal(r.schema_version, 1);
    assert.equal(r.data.revision, o.identity.commit);
    assert.deepEqual(r.data.effects, { attempted: false, committed: false });
    assert.equal(JSON.stringify(r).includes("unknown-secret"), false);
    assert.equal(JSON.stringify(r).includes("invalid-secret"), false);
    for (const policy of ["normative_conformance", "authoring_readiness", "host_safety", "release_policy", "loadability", "compatibility", "runtime_evidence", "toolchain"]) {
      assert.equal(r.data[policy].status, "not_evaluated");
    }
  }
  // B's retained-command legacy-flag exemptions come from its accepted test.
  const compatSource = checked("/usr/bin/git", ["show", `${o.identity.commit}:cli/plugin-kit-ai/cmd/plugin-kit-ai/release_compat_test.go`], o.repo);
  const mapBody = compatSource.split("retained := map[string]string{")[1].split("}")[0];
  const retainedFlags = Object.fromEntries([...mapBody.matchAll(/"([^"]+)": "([^"]*)"/g)].map(m => [m[1], m[2].split(" ")]));
  for (const [, command, flags, retained] of rows) if (retained === "true") {
    for (const spec of flags.split(" ").filter(Boolean)) {
      const [long, short] = spec.split("/"), name = long.replace(/!$/, "");
      if (retainedFlags[command]?.includes(name)) continue;
      const value = long.endsWith("!") ? "false" : "credential-fixture";
      for (const tail of [[`--${name}=${value}`], ...(short ? [[`-${short}=${value}`]] : [])]) {
        const r = invoke("plugin-kit-ai", [...command.split(" "), "--format=json", ...tail], 2);
        assert.deepEqual(r.data.effects, { attempted: false, committed: false });
      }
    }
  }
  for (const spelling of ["1", "t", "T", "TRUE", "true", "True", "0", "f", "F", "FALSE", "false", "False"]) {
    const truth = ["1", "t", "T", "TRUE", "true", "True"].includes(spelling);
    const r = invoke("plugin-kit-ai", ["update", `--all=${spelling}`, `--dry-run=${spelling}`, "--format=json"], 2);
    assert.equal(r.data.error.action.includes("agentplugins update --all"), truth);
    assert.equal(r.data.error.action.includes("Preserve plan intent"), truth);
    invoke("plugin-kit-ai", ["init", `--force=${spelling}`, "--help", "--format=json"], 2);
  }
  assert.deepEqual(tree(o.root), candidateBefore); assert.equal(fs.statSync(o.root).mode, candidateMode);
  fs.writeFileSync(path.join(cfg.evidenceOutput, "native-completion.json"), c.encode({
    kind: "actual-linux-private-npm-pair", identity: o.identity, candidate_sha256: o.manifestDigest,
    completion_sha256: cfg.completionDigest, packs: record.packs, tools: record.tools,
    inventory_sha256: c.digest(Buffer.from(inventory)), invocations: invocations.length, projects,
    release_eligible: false, platform_acceptance: false, attested: false,
    installer_planner: "separate existing injected-detector/scanner source fixture required; no live installer action"
  }));
});
