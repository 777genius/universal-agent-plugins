"use strict";
const nodeTest = require("node:test");
const enabled = process.platform === "linux" && process.env.AGENTPLUGINS_STAGED_TEST_CHILD !== "1";
const test = (name, fn) => nodeTest.test(name, { skip: !enabled }, fn);
const before = fn => nodeTest.before(() => { if (enabled) return fn(); });
const { fs, path, assert, c, s, mkdir, node, checked, fixture, installPair, changeJSON } = require("./private-npm-fixture");
let f;
before(() => { f = installPair(fixture()); });
const success = r => { assert.equal(r.status, 0, r.stderr); assert.equal(r.stderr, ""); return JSON.parse(r.stdout); };
const failure = (r, product) => { assert.equal(r.status, 1); assert.equal(r.stdout, ""); assert.match(r.stderr, new RegExp(`^${product} private npm launcher: `)); assert.ok(r.stderr.length < 700); };

test("STRUCTURAL real packs: exact committed closure, independent installation, pins and no lifecycle", () => {
  for (const product of c.PRODUCTS) {
    const root = f.packageRoot(product), pkg = JSON.parse(fs.readFileSync(path.join(root, "package.json")));
    assert.equal(pkg.private, true); assert.equal(pkg.scripts, undefined); assert.equal(pkg.dependencies, undefined);
    assert.equal(pkg.publishConfig, undefined); assert.deepEqual(pkg.bin, { [product]: `bin/${product}.js` });
    assert.deepEqual(pkg.engines, { node: product === "agentplugins" ? ">=22" : ">=18" });
    assert.equal(pkg.repository.url, `git+https://github.com/${c.REPOSITORY}.git`);
    assert.equal(fs.existsSync(path.join(root, "vendor")), false);
    assert.deepEqual(fs.readdirSync(path.join(f.prefixes[product], "node_modules")).filter(n => !n.startsWith(".")), [s.PACKAGES[product]]);
    assert.equal(f.record.packs[product].sha256, c.digest(fs.readFileSync(f.tarball(product))));
    for (const [file, digest] of Object.entries(f.record.generated[product])) assert.equal(c.digest(fs.readFileSync(path.join(root, file))), digest);
    for (const file of s.COMMON) assert.equal(c.digest(fs.readFileSync(path.join(root, file))), f.record.wrapper_blobs["npm/agentplugins/" + file].sha256);
    assert.deepEqual(fs.readFileSync(path.join(root, "candidate.json")), fs.readFileSync(path.join(f.source, "candidate.json")));
  }
  assert.equal(f.record.release_eligible, false); assert.equal(f.record.platform_acceptance, false); assert.equal(f.record.attested, false);
  // Remove checkout access by relocation; only the detached pack remains on the
  // runtime path. Node has no NODE_PATH and each prefix contains just one product.
  fs.renameSync(f.repo, f.repo + "-unavailable");
  for (const product of c.PRODUCTS) assert.equal(success(f.launch(product)).product, product);
  fs.renameSync(f.repo + "-unavailable", f.repo);
});

for (const product of c.PRODUCTS) test(`STRUCTURAL detached ${product}: argv/cwd/env, statuses, cold/warm, stale and corrupt bytes`, () => {
  const args = ["", "two words", "雪🚀", "'quote\"", "$(touch forbidden)", "; & | > *", "--x=value", "--x", "split", "--", "author"];
  const env = { ...f.envFor(), ORDINARY_FIXTURE: "kept", UAP_PRIVATE_NPM_EXTRA: "strip", UAP_CANDIDATE_NATIVE_CONFIG: "strip",
    AGENTPLUGINS_INTERNAL_PROOF_MODE: "strip", AGENTPLUGINS_INTERNAL_PROOF_BINARY: "strip", PLUGIN_KIT_AI_INTERNAL_PROOF_BINARY: "strip" };
  const got = success(f.launch(product, args, env));
  assert.deepEqual(got.argv, args); assert.equal(got.cwd, f.cwd); assert.equal(got.env.ORDINARY_FIXTURE, "kept");
  for (const key of Object.keys(env).filter(k => /UAP_PRIVATE|UAP_CANDIDATE|INTERNAL_PROOF/.test(k))) assert.equal(got.env[key], undefined);
  for (const status of [0, 1, 2, 37]) assert.equal(f.launch(product, ["exit", String(status)]).status, status);
  const binary = f.binaryPath(product), original = fs.readFileSync(binary);
  const stale = path.join(f.cache, "old-v1", f.identity.versions[product], product); mkdir(path.dirname(stale)); fs.writeFileSync(stale, "old v1 sentinel");
  const vendor = path.join(f.packageRoot(product), "vendor", "v1.2.4", product); mkdir(path.dirname(vendor)); fs.writeFileSync(vendor, "old vendor sentinel");
  for (const corrupt of [() => fs.writeFileSync(binary, Buffer.alloc(original.length)), () => fs.writeFileSync(binary, "truncated"),
    () => fs.chmodSync(binary, 0o644), () => fs.writeFileSync(binary, fs.readFileSync(f.binaryPath(c.PRODUCTS.find(p => p !== product))))]) {
    corrupt(); assert.equal(success(f.launch(product)).product, product); assert.deepEqual(fs.readFileSync(binary), original);
    assert.equal(fs.statSync(binary).mode & 0o777, 0o755);
  }
  assert.equal(fs.readFileSync(stale, "utf8"), "old v1 sentinel"); assert.equal(fs.readFileSync(vendor, "utf8"), "old vendor sentinel");
  const separate = mkdir(path.join(f.root, product + " separate cache"));
  assert.equal(success(f.launch(product, [], f.envFor(separate))).product, product);
  fs.renameSync(f.source, f.source + "-unavailable");
  assert.equal(success(f.launch(product)).product, product);
  assert.equal(success(f.launch(product, [], { ...f.envFor(separate), UAP_PRIVATE_NPM_CANDIDATE: undefined })).product, product);
  fs.renameSync(f.source + "-unavailable", f.source);
});

// Every row crosses the real installed fixed bin boundary with a VALID warm
// executable. N1 retains the detailed acquisition/archive/fault implementation.
const mutations = [
  ["package.json", "name", p => { p.name = "wrong"; }],
  ["package.json", "bin path", p => { p.bin[Object.keys(p.bin)[0]] = "../other"; }],
  ["package.json", "bin alias", p => { p.bin.alias = "bin/alias.js"; }],
  ["package.json", "version", p => { p.version = "9.8.7"; }],
  ["package.json", "private", p => { p.private = false; }],
  ["private-release.json", "product", d => { d.product = d.product === "agentplugins" ? "plugin-kit-ai" : "agentplugins"; }],
  ["private-release.json", "npm name", d => { d.npm_package = "wrong"; }],
  ["private-release.json", "other product version", d => { d.identity.versions[d.product === "agentplugins" ? "plugin-kit-ai" : "agentplugins"] = "9.8.7"; }],
  ...[
    ["repository alias", i => { i.repository = "777genius/plugin-kit-ai"; }],
    ["short commit", i => { i.commit = "a"; }], ["uppercase commit", i => { i.commit = "A".repeat(40); }],
    ["wrong commit", i => { i.commit = i.engine_revision = "d".repeat(40); }],
    ["unequal engine", i => { i.engine_revision = "d".repeat(40); }],
    ["extra identity", i => { i.extra = true; }], ["missing identity", i => { delete i.repository; }]
  ].map(([label, fn]) => ["private-release.json", label, d => fn(d.identity)]),
  ["private-release.json", "wrong expected mode", d => { d.authoring_mode = "vertical-slice-v1"; }],
  ["private-release.json", "unknown expected mode", d => { d.authoring_mode = "any"; }],
  ["private-release.json", "missing mode", d => { delete d.authoring_mode; }],
  ["private-release.json", "scope", d => { d.asset_scope = "six-platform-pair"; }],
  ["private-release.json", "unsupported target", d => { d.asset_scope = "linux-386"; }],
  ["private-release.json", "digest", d => { d.candidate_sha256 = "f".repeat(64); }],
  ["private-release.json", "extra descriptor", d => { d.extra = true; }],
  ["candidate.json", "schema v1", m => { m.schema = 1; }], ["candidate.json", "public schema v2", m => { m.schema = 2; }],
  ["candidate.json", "release claim", m => { m.release_eligible = true; }],
  ["candidate.json", "attestation claim", m => { m.attested = true; }],
  ["candidate.json", "missing product", m => { delete m.products["plugin-kit-ai"]; }],
  ["candidate.json", "extra asset", m => { m.products.agentplugins.assets["linux-386"] = {}; }],
  ["candidate.json", "missing asset", m => { delete m.products.agentplugins.assets["linux-amd64"]; }],
  ["candidate.json", "candidate mode", m => { m.build.authoring_mode = "vertical-slice-v1"; }],
  ...[
    ["unsafe name", a => { a.file = "../escape"; }], ["archive filename", a => { a.file = "plugin-kit-ai.tar.gz"; }],
    ["inner filename", a => { a.binary.file = "agentplugins"; }], ["extra pin", a => { a.extra = true; }],
    ...[false, true].flatMap(inner => ["size", "sha256"].flatMap(key => (key === "size" ? [0, -1, 1.5, "1", null, 134217729, Number.MAX_SAFE_INTEGER + 1] : ["A".repeat(64), "short", 1, null, []]).map(value =>
      [`${inner ? "inner" : "outer"} ${key} ${JSON.stringify(value)}`, a => { (inner ? a.binary : a)[key] = value; }])) )
  ].map(([label, fn]) => ["candidate.json", label, m => fn(m.products["plugin-kit-ai"].assets["linux-amd64"])])
];
for (const product of c.PRODUCTS) test(`STRUCTURAL ${product}: full packed warm identity mismatch matrix (${mutations.length} rows plus encoding)`, () => {
  success(f.launch(product)); const binary = f.binaryPath(product), pin = c.digest(fs.readFileSync(binary));
  for (const [name, label, mutate] of mutations) {
    const file = path.join(f.packageRoot(product), name), bytes = fs.readFileSync(file);
    const descriptor = path.join(f.packageRoot(product), "private-release.json"), descriptorBytes = fs.readFileSync(descriptor);
    try {
      changeJSON(file, mutate);
      if (name === "candidate.json") changeJSON(descriptor, d => { d.candidate_sha256 = c.digest(fs.readFileSync(file)); });
      const r = f.launch(product); failure(r, product); assert.equal(c.digest(fs.readFileSync(binary)), pin, label);
    } finally { fs.writeFileSync(file, bytes); fs.writeFileSync(descriptor, descriptorBytes); }
  }
  for (const name of ["package.json", "private-release.json", "candidate.json"]) {
    const file = path.join(f.packageRoot(product), name), bytes = fs.readFileSync(file);
    for (const bad of [Buffer.concat([bytes, Buffer.from(" ")]), Buffer.from(bytes.toString().replace("{", '{"x":1,"x":2,')), Buffer.alloc(1048577, 32), Buffer.from([255]), Buffer.alloc(0)]) {
      fs.writeFileSync(file, bad); failure(f.launch(product), product); fs.writeFileSync(file, bytes);
    }
  }
});

test("STRUCTURAL actual local scripts-disabled upgrade/uninstall/reinstall preserves other product", () => {
  const next = installPair(fixture("upgrade", { agentplugins: "0.1.24", "plugin-kit-ai": "2.0.1" }));
  const prefix = f.install(c.PRODUCTS.map(f.tarball), path.join(f.root, "combined lifecycle prefix"));
  const original = { ...f.prefixes }; for (const p of c.PRODUCTS) f.prefixes[p] = prefix;
  const other = path.join(prefix, "node_modules", "plugin-kit-ai", "candidate.json"), pin = c.digest(fs.readFileSync(other));
  try {
    f.install([next.tarball("agentplugins")], prefix);
    const env = { ...f.envFor(), UAP_PRIVATE_NPM_CANDIDATE: next.source };
    assert.equal(success(f.launch("agentplugins", [], env)).version, "0.1.24");
    assert.equal(c.digest(fs.readFileSync(other)), pin); assert.equal(success(f.launch("plugin-kit-ai")).version, "2.0.0");
    checked(node, [f.options.npm, "uninstall", "--prefix", prefix, "--ignore-scripts", "--offline", "--no-audit", "--no-fund", "universal-agent-plugins"], f.env, prefix);
    assert.equal(fs.existsSync(path.join(prefix, "node_modules", ".bin", "agentplugins")), false);
    assert.equal(success(f.launch("plugin-kit-ai")).version, "2.0.0"); assert.equal(c.digest(fs.readFileSync(other)), pin);
    f.install([f.tarball("agentplugins")], prefix); assert.equal(success(f.launch("agentplugins")).version, "0.1.23");
    // Reciprocal lifecycle uses the same separately staged local upgrade pair.
    // Snapshot the entire preserved package, bin link, and warm executable
    // across each npm operation.
    const tree = root => fs.readdirSync(root).sort().map(name => {
      const file = path.join(root, name), stat = fs.lstatSync(file);
      return [name, stat.mode, stat.isDirectory() ? tree(file) : stat.isSymbolicLink() ? fs.readlinkSync(file) : c.digest(fs.readFileSync(file))];
    });
    const bin = product => path.join(prefix, "node_modules", ".bin", product);
    const agentRoot = f.packageRoot("agentplugins"), agentTree = tree(agentRoot);
    const agentLink = fs.readlinkSync(bin("agentplugins"));
    const agentBinary = f.binaryPath("agentplugins"), agentBytes = fs.readFileSync(agentBinary);
    const agentMode = fs.statSync(agentBinary).mode;
    const preserved = () => {
      assert.deepEqual(tree(agentRoot), agentTree);
      assert.equal(fs.readlinkSync(bin("agentplugins")), agentLink);
      assert.deepEqual(fs.readFileSync(agentBinary), agentBytes);
      assert.equal(fs.statSync(agentBinary).mode, agentMode);
      assert.equal(fs.realpathSync(bin("agentplugins")), path.join(agentRoot, "bin", "agentplugins.js"));
      const got = success(f.launch("agentplugins"));
      assert.equal(got.product, "agentplugins"); assert.equal(got.version, "0.1.23");
      assert.equal(JSON.parse(checked(node, [bin("agentplugins")], f.envFor(), f.cwd)).version, "0.1.23");
    };
    const installed = (version, env) => {
      const root = f.packageRoot("plugin-kit-ai"), pkg = JSON.parse(fs.readFileSync(path.join(root, "package.json")));
      assert.equal(pkg.version, version); assert.equal(pkg.scripts, undefined);
      assert.deepEqual(pkg.bin, { "plugin-kit-ai": "bin/plugin-kit-ai.js" });
      assert.equal(fs.realpathSync(bin("plugin-kit-ai")), path.join(root, "bin", "plugin-kit-ai.js"));
      const got = success(f.launch("plugin-kit-ai", [], env));
      assert.equal(got.product, "plugin-kit-ai"); assert.equal(got.version, version);
      assert.equal(JSON.parse(checked(node, [bin("plugin-kit-ai")], env, f.cwd)).version, version);
      assert.equal(fs.existsSync(path.join(root, "vendor")), false);
      preserved();
    };
    const inputs = [tree(f.source), tree(next.source)];
    f.install([next.tarball("plugin-kit-ai")], prefix);
    installed("2.0.1", env);
    checked(node, [f.options.npm, "uninstall", "--prefix", prefix, "--ignore-scripts", "--offline", "--no-audit", "--no-fund", "plugin-kit-ai"], f.env, prefix);
    assert.equal(fs.existsSync(f.packageRoot("plugin-kit-ai")), false);
    assert.equal(fs.existsSync(bin("plugin-kit-ai")), false); preserved();
    f.install([f.tarball("plugin-kit-ai")], prefix);
    installed("2.0.0", f.envFor());
    assert.deepEqual([tree(f.source), tree(next.source)], inputs);
  } finally { f.prefixes = original; }
  fs.writeFileSync(path.join(f.root, "structural-evidence.json"), c.encode({ kind: "STRUCTURAL real npm packs; mock Go/executables", record: f.record, upgrade: next.record, mutations: mutations.length }));
  console.log(`STRUCTURAL pack artifacts: ${f.root}`);
});

test("STRUCTURAL stager: exact HEAD/blob identity, immutable inputs, reserved output and partial failure", () => {
  const g = fixture("stager-negatives");
  const expect = options => assert.throws(() => g.stage(options));
  expect({ ...g.options, identity: { ...g.identity, commit: "e".repeat(40), engine_revision: "e".repeat(40) } });
  expect({ ...g.options, manifestDigest: "d".repeat(64) }); expect({ ...g.options, authoringMode: "vertical-slice-v1" });
  assert.equal(fs.existsSync(g.options.output), false);
  const runtime = path.join(g.repo, "npm/agentplugins/scripts/private-npm/launcher.js");
  fs.writeFileSync(runtime, "DIRTY MUST NEVER SHIP");
  const record = g.stage();
  assert.equal(record.generated.agentplugins["scripts/private-npm/launcher.js"], record.wrapper_blobs["npm/agentplugins/scripts/private-npm/launcher.js"].sha256);
  const sentinel = path.join(g.options.output, "unowned"); fs.writeFileSync(sentinel, "preserved"); expect(g.options); assert.equal(fs.readFileSync(sentinel, "utf8"), "preserved");
  // A failing npm leaves generated packages and no completion marker.
  const badNpm = path.join(g.root, "tools", "failing-npm.js"); fs.writeFileSync(badNpm, "if(process.argv.includes('--version'))console.log('fixture');else process.exit(7);\n");
  const output = path.join(g.root, "failed-packs"); expect({ ...g.options, output, npm: badNpm });
  assert.equal(fs.existsSync(path.join(output, "agentplugins", "package.json")), true);
  assert.equal(fs.existsSync(path.join(output, "completion.json")), false);
  // A missing committed runtime cannot be supplied by an untracked worktree file.
  checked("/usr/bin/git", ["update-index", "--force-remove", "npm/agentplugins/scripts/private-npm/launcher.js"], g.env, g.repo);
  checked("/usr/bin/git", ["-c", "core.hooksPath=/dev/null", "commit", "--quiet", "-m", "Structural missing closure negative"], g.env, g.repo);
  const commit = checked("/usr/bin/git", ["rev-parse", "HEAD"], g.env, g.repo).trim();
  expect({ ...g.options, output: path.join(g.root, "missing-blob"), identity: { ...g.identity, commit, engine_revision: commit } });
});

test("STRUCTURAL stager rejects repinned wrong build identity before npm or execution", () => {
  for (const wrong of ["main", "version", "revision", "mode", "target", "compiler"]) {
    const g = fixture("buildinfo-" + wrong), m = g.manifest;
    const a = m.products.agentplugins.assets["linux-amd64"], file = path.join(g.source, a.file);
    const lines = fs.readFileSync(file, "utf8").split("\n"), info = JSON.parse(lines[1].slice(2));
    if (wrong === "main") info.Path = info.Path.replace("agentplugins", "plugin-kit-ai");
    else if (wrong === "compiler") info.GoVersion = "go1.0";
    else {
      const key = wrong === "target" ? "GOARCH" : "-ldflags";
      const setting = info.Settings.find(x => x.Key === key);
      setting.Value = wrong === "target" ? "arm64" : wrong === "version" ? setting.Value.replace("0.1.23", "0.1.24") :
        wrong === "revision" ? setting.Value.replace(g.commit, "e".repeat(40)) : setting.Value.replace(s.MODE, "vertical-slice-v1");
    }
    lines[1] = "//" + JSON.stringify(info); const bytes = Buffer.from(lines.join("\n"));
    fs.chmodSync(g.source, 0o700); fs.chmodSync(file, 0o600); fs.writeFileSync(file, bytes); fs.chmodSync(file, 0o444);
    Object.assign(a, c.metadata(bytes)); Object.assign(a.binary, c.metadata(bytes));
    const manifestFile = path.join(g.source, "candidate.json"); fs.chmodSync(manifestFile, 0o600);
    fs.writeFileSync(manifestFile, c.encode(m)); fs.chmodSync(manifestFile, 0o444); fs.chmodSync(g.source, 0o555);
    assert.throws(() => g.stage({ ...g.options, manifestDigest: c.digest(c.encode(m)) }), /embedded Go/);
    assert.equal(fs.existsSync(g.options.output), false);
  }
});

test("STRUCTURAL stager cannot run dirty generator code or use a symlink Git blob", () => {
  const g = fixture("blob-negative");
  const file = path.join(g.repo, "npm/agentplugins/scripts/stage-dual-authoring-npm.js");
  fs.appendFileSync(file, "\n// uncommitted generator\n");
  assert.throws(() => require(file).stagePair(g.options), /executing stager differs/);
  const runtime = path.join(g.repo, "npm/agentplugins/scripts/private-npm/launcher.js");
  fs.unlinkSync(runtime); fs.symlinkSync("bootstrap.js", runtime);
  checked("/usr/bin/git", ["add", "npm/agentplugins/scripts/private-npm/launcher.js"], g.env, g.repo);
  checked("/usr/bin/git", ["-c", "core.hooksPath=/dev/null", "commit", "--quiet", "-m", "Structural symlink closure negative"], g.env, g.repo);
  const commit = checked("/usr/bin/git", ["rev-parse", "HEAD"], g.env, g.repo).trim();
  assert.throws(() => g.stage({ ...g.options, identity: { ...g.identity, commit, engine_revision: commit } }), /regular Git blob/);
  assert.equal(fs.existsSync(g.options.output), false);
});

test("STRUCTURAL completion collision and cleanup failure preserve inspectable evidence", () => {
  for (const fault of ["collision", "unlink"]) {
    const g = fixture("completion-" + fault), link = fs.linkSync, unlink = fs.unlinkSync;
    const marker = path.join(g.options.output, "completion.json");
    const pending = path.join(g.options.output, "completion.pending.json");
    try {
      if (fault === "collision") fs.linkSync = (a, b) => {
        if (b === marker) fs.writeFileSync(marker, "unowned marker", { flag: "wx" });
        return link(a, b);
      };
      else fs.unlinkSync = p => { if (p === pending) throw new Error("injected pending cleanup failure"); return unlink(p); };
      assert.throws(() => g.stage(), fault === "collision" ? /EEXIST/ : /cleanup failure/);
    } finally { fs.linkSync = link; fs.unlinkSync = unlink; }
    assert.equal(fs.existsSync(pending), true);
    for (const p of c.PRODUCTS) assert.equal(fs.existsSync(g.tarball(p)), true);
    if (fault === "collision") assert.equal(fs.readFileSync(marker, "utf8"), "unowned marker");
    else assert.equal(fs.existsSync(marker), false);
  }
});
