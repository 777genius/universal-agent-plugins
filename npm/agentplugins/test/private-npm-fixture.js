"use strict";
// STRUCTURAL fixture support. Mock Go only reads fixture metadata; it is NEVER
// native build-info evidence. Retain all disposable roots for audit.
const fs = require("node:fs");
const path = require("node:path");
const os = require("node:os");
const cp = require("node:child_process");
const assert = require("node:assert/strict");
const c = require("../scripts/dual-authoring-candidate");
const s = require("../scripts/stage-dual-authoring-npm");
const b = require("../scripts/private-npm/bootstrap");
const mkdir = p => { fs.mkdirSync(p, { recursive: true, mode: 0o700 }); return p; };
const write = (p, bytes, mode = 0o644) => { mkdir(path.dirname(p)); fs.writeFileSync(p, bytes, { mode }); };
const node = process.execPath;
const npm = fs.realpathSync("/usr/local/bin/npm");
const run = (exe, args, env, cwd) => cp.spawnSync(exe, args, { env, cwd, encoding: "utf8", timeout: 45000, maxBuffer: 16 * 1024 * 1024 });
function checked(exe, args, env, cwd) {
  const r = run(exe, args, env, cwd);
  assert.equal(r.status, 0, `${exe} ${args.join(" ")}: ${r.error || ""}\n${r.stdout}\n${r.stderr}`); return r.stdout;
}
function workspace(label) {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), `npm-${label}-`));
  const scratch = mkdir(path.join(root, "scratch"));
  const ctx = s.npmContext(scratch);
  return { root, scratch, env: ctx.env };
}
function sourceFixture(root, env) {
  const repo = mkdir(path.join(root, "source repo"));
  for (const file of s.ALLOWLIST) write(path.join(repo, file), fs.readFileSync(path.resolve(__dirname, "../../..", file)));
  checked("/usr/bin/git", ["init", "--quiet", repo], env);
  checked("/usr/bin/git", ["config", "user.name", "iliya"], env, repo);
  checked("/usr/bin/git", ["config", "user.email", "iliyazelenkog@gmail.com"], env, repo);
  assert.equal(checked("/usr/bin/git", ["config", "user.name"], env, repo).trim(), "iliya");
  assert.equal(checked("/usr/bin/git", ["config", "user.email"], env, repo).trim(), "iliyazelenkog@gmail.com");
  checked("/usr/bin/git", ["add", "."], env, repo);
  checked("/usr/bin/git", ["-c", "core.hooksPath=/dev/null", "commit", "--quiet", "-m", "Disposable structural npm fixture; not intended integration"], env, repo);
  assert.equal(checked("/usr/bin/git", ["show", "-s", "--format=%an <%ae> | %cn <%ce>", "HEAD"], env, repo).trim(),
    "iliya <iliyazelenkog@gmail.com> | iliya <iliyazelenkog@gmail.com>");
  return { repo, commit: checked("/usr/bin/git", ["rev-parse", "HEAD"], env, repo).trim() };
}
function candidate(root, commit, versions = { agentplugins: "0.1.23", "plugin-kit-ai": "2.0.0" }, behavior = "normal") {
  const source = mkdir(path.join(root, "candidate inputs with spaces", "candidate"));
  const go = path.join(mkdir(path.join(root, "tools")), "mock-go");
  write(go, `#!${node}\nconst fs=require('node:fs');const a=process.argv.slice(2);\nif(a[0]==='env') console.log(JSON.stringify({GOVERSION:'go1.25.13',GOHOSTOS:'linux',GOHOSTARCH:'amd64'}));\nelse if(a[1]==='-m')console.log(fs.readFileSync(a[3],'utf8').split('\\n')[1].slice(2));\nelse console.log('go version go1.25.13 linux/amd64 (STRUCTURAL MOCK)');\n`, 0o755);
  const identity = { repository: c.REPOSITORY, commit, engine_revision: commit, versions };
  const manifest = { schema: c.SCHEMA, status: "CANDIDATE", identity, asset_scope: "linux-amd64-pair",
    build: { method: "controlled-git-archive-go-build/v1", go_version: "go1.25.13", go_sha256: c.digest(fs.readFileSync(go)),
      source_archive_sha256: "c".repeat(64), authoring_mode: s.MODE }, products: {}, release_eligible: false };
  for (const product of c.PRODUCTS) {
    const info = { GoVersion: "go1.25.13", Path: `github.com/777genius/plugin-kit-ai/cli/cmd/${product}`,
      Settings: Object.entries({ GOOS: "linux", GOARCH: "amd64", CGO_ENABLED: "0", "-buildmode": "exe", "-compiler": "gc",
        "-ldflags": c.linkerFlags(product, identity, s.MODE) }).map(([Key, Value]) => ({ Key, Value })) };
    const body = Buffer.from(`#!${behavior === "spawn-error" ? "/absent-structural-interpreter" : node}\n//${JSON.stringify(info)}\n` +
      `const fs=require('node:fs');const args=process.argv.slice(2);const p=${JSON.stringify(product)};\n` +
      `if(args[0]==='wait'||args[0]==='stubborn'){process.on('SIGINT',()=>{fs.appendFileSync(args[1]+'.signals','SIGINT\\n');if(args[0]!=='stubborn')setInterval(()=>{if(fs.existsSync(args[1]+'.release'))process.exit(0)},10)});process.on('SIGTERM',()=>{fs.appendFileSync(args[1]+'.signals','SIGTERM\\n');if(args[0]!=='stubborn')setInterval(()=>{if(fs.existsSync(args[1]+'.release'))process.exit(0)},10)});fs.writeFileSync(args[1],String(process.pid));setInterval(()=>{},1000)}\n` +
      `else if(args[0]==='signal')process.kill(process.pid,args[1]);\nelse {console.log(JSON.stringify({kind:'STRUCTURAL mock executable',product:p,version:${JSON.stringify(versions[product])},argv:args,cwd:process.cwd(),env:process.env}));process.exitCode=args[0]==='exit'?Number(args[1]):0;}\n`);
    const file = c.assetName(product, versions[product], "linux-amd64");
    const bytes = product === "plugin-kit-ai" ? c.archive(body, product) : body;
    write(path.join(source, file), bytes, 0o444);
    manifest.products[product] = { version: versions[product], assets: { "linux-amd64": {
      file, ...c.metadata(bytes), binary: { file: product, ...c.metadata(body) }
    } } };
  }
  write(path.join(source, "candidate.json"), c.encode(manifest), 0o444); fs.chmodSync(source, 0o555);
  return { source, go, manifest, identity, manifestDigest: c.digest(c.encode(manifest)) };
}
function fixture(label = "structural", versions, behavior) {
  const f = workspace(label); Object.assign(f, sourceFixture(f.root, f.env));
  Object.assign(f, candidate(f.root, f.commit, versions, behavior));
  f.options = { candidate: true, repo: f.repo, root: f.source, identity: f.identity, manifestDigest: f.manifestDigest,
    assetScope: "linux-amd64-pair", authoringMode: s.MODE, go: f.go, workParent: f.scratch,
    output: path.join(f.root, "packs"), node, npm };
  f.stage = options => s.stagePair(options || f.options);
  f.install = (packs, prefix) => {
    mkdir(prefix); checked(node, [npm, "install", "--prefix", prefix, "--ignore-scripts", "--offline", "--no-audit", "--no-fund", "--package-lock=false", ...packs], f.env, prefix);
    return prefix;
  };
  f.tarball = product => path.join(f.options.output, `${s.PACKAGES[product]}-${f.identity.versions[product]}.tgz`);
  f.cache = mkdir(path.join(f.root, "cache with spaces"));
  f.cwd = mkdir(path.join(f.root, "unrelated project with spaces"));
  f.prefixes = {};
  f.envFor = cache => ({ ...f.env, PATH: "/absent-fixture-path", UAP_PRIVATE_NPM_CANDIDATE: f.source, UAP_PRIVATE_NPM_CACHE: cache || f.cache });
  f.launch = (product, args = [], env = f.envFor()) => run(node, [path.join(f.prefixes[product], "node_modules", s.PACKAGES[product], "bin", product + ".js"), ...args], env, f.cwd);
  f.packageRoot = product => path.join(f.prefixes[product], "node_modules", s.PACKAGES[product]);
  f.binaryPath = (product, cache = f.cache) => b.cachePath(cache, product, "linux-amd64", b.loadRelease(product, f.packageRoot(product), "linux-amd64", s.MODE));
  return f;
}
function installPair(f) {
  f.record = f.stage();
  for (const product of c.PRODUCTS) f.prefixes[product] = f.install([f.tarball(product)], path.join(f.root, product + " detached prefix"));
  return f;
}
const changeJSON = (file, change) => { const value = JSON.parse(fs.readFileSync(file)); change(value); fs.writeFileSync(file, c.encode(value)); };
module.exports = { fs, path, cp, assert, c, s, b, mkdir, write, node, npm, run, checked, workspace, fixture, installPair, changeJSON };
