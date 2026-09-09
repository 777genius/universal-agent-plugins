#!/usr/bin/env node
"use strict";

// PRIVATE, offline packaging of one controlled candidate. Never builds a main.
const fs = require("node:fs");
const path = require("node:path");
const cp = require("node:child_process");
const crypto = require("node:crypto");
const c = require("./dual-authoring-candidate");
const producer = require("./stage-dual-authoring-candidate");
const { validateProductPackJSON } = require("./npm-public-contract");
const MODE = "release-cli-contract-v1";
const PACKAGES = { agentplugins: "universal-agent-plugins", "plugin-kit-ai": "plugin-kit-ai" };
const COMMON = ["lib/verifier.js", "scripts/dual-authoring-candidate.js",
  "scripts/private-npm/bootstrap.js", "scripts/private-npm/launcher.js"];
const PREFIX = "npm/agentplugins/";
const ALLOWLIST = Object.freeze([...COMMON.map(n => PREFIX + n),
  PREFIX + "scripts/stage-dual-authoring-npm.js", PREFIX + "scripts/stage-dual-authoring-candidate.js",
  PREFIX + "scripts/private-npm/README.md",
  ...["agentplugins", "plugin-kit-ai"].flatMap(p => ["package.json", "LICENSE", "README.md"].map(n => `npm/${p}/${n}`))]);
const run = (exe, argv, env, cwd) => cp.execFileSync(exe, argv, {
  env, cwd, timeout: 120000, maxBuffer: 64 * 1024 * 1024
});
const write = (file, bytes, mode = 0o644) => {
  fs.mkdirSync(path.dirname(file), { recursive: true, mode: 0o700 });
  fs.writeFileSync(file, bytes, { flag: "wx", mode });
  fs.chmodSync(file, mode);
};

function npmContext(workParent) {
  const context = producer.privateContext(workParent);
  const user = path.join(context.root, "npm-user.conf"), global = path.join(context.root, "npm-global.conf");
  fs.writeFileSync(user, "", { flag: "wx", mode: 0o600 });
  fs.writeFileSync(global, "", { flag: "wx", mode: 0o600 });
  Object.assign(context.env, { npm_config_userconfig: user, npm_config_globalconfig: global,
    npm_config_cache: path.join(context.root, "cache", "npm"), npm_config_offline: "true",
    npm_config_ignore_scripts: "true", npm_config_audit: "false", npm_config_fund: "false",
    npm_config_update_notifier: "false" });
  return context;
}

function blobs(repo, commit, env, closure = "private") {
  if (!["private", "public", "stage"].includes(closure)) throw new Error("unknown fixed npm closure");
  const allowlist = closure === "private" ? ALLOWLIST : require("./stage-authoring-npm")[closure === "stage" ? "STAGE_ALLOWLIST" : "ALLOWLIST"];
  c.safeDirectory(repo);
  if (run("/usr/bin/git", ["rev-parse", "HEAD"], env, repo).toString().trim() !== commit) {
    throw new Error("expected source must equal checkout HEAD");
  }
  const result = {};
  // Check the newly executing helper at the same commit without extending the
  // historical private/public preparation wrapper_blobs receipt inventories.
  const packHelper = PREFIX + "scripts/npm-public-contract.js";
  for (const name of [...new Set([...allowlist, packHelper])]) {
    const entry = run("/usr/bin/git", ["ls-tree", "-z", commit, "--", name], env, repo).toString();
    const match = /^(100644|100755) blob ([0-9a-f]{40})\t([^\0]+)\0$/.exec(entry);
    if (!match || match[3] !== name) throw new Error(`required regular Git blob missing: ${name}`);
    const bytes = run("/usr/bin/git", ["cat-file", "blob", match[2]], env, repo);
    result[name] = { bytes, git_blob: match[2], mode: match[1], sha256: c.digest(bytes) };
  }
  // The code doing verification/generation must itself be this committed code.
  // A dirty caller may not manufacture an exact-source claim using old blobs.
  for (const name of ["scripts/npm-public-contract.js", "scripts/stage-dual-authoring-npm.js", "scripts/stage-dual-authoring-candidate.js",
    "scripts/dual-authoring-candidate.js", ...(closure === "public" ?
      ["scripts/stage-authoring-npm.js", "scripts/authoring-release.js", "lib/public-authoring.js"] : [])]) {
    if (!c.readFile(path.resolve(__dirname, "..", name)).equals(result[PREFIX + name].bytes)) {
      throw new Error(`executing stager differs from committed source: ${name}`);
    }
  }
  if (closure === "stage") {
    // Every listed checkout byte AND every executing-tree byte must be F. Keep
    // legacy preparation inventories/checks unchanged; stage has its own set.
    const executing = path.resolve(__dirname, "../../..");
    for (const root of new Set([repo, executing])) for (const name of allowlist) {
      const file = path.join(root, name), pin = result[name];
      if (!c.readFile(file).equals(pin.bytes) ||
          (fs.lstatSync(file).mode & 0o777) !== (pin.mode === "100755" ? 0o755 : 0o644)) {
        throw new Error(`stage source differs from committed F: ${name}`);
      }
    }
  }
  return Object.fromEntries(allowlist.map(name => [name, result[name]]));
}

function packageFiles(product, source, manifestBytes, options) {
  const base = JSON.parse(source[`npm/${product}/package.json`].bytes);
  const files = Object.fromEntries(COMMON.map(n => [n, source[PREFIX + n].bytes]));
  files["LICENSE"] = source[`npm/${product}/LICENSE`].bytes;
  files["README.md"] = source[PREFIX + "scripts/private-npm/README.md"].bytes;
  // Local CommonJS scopes let our bounded loader diagnose malformed root JSON
  // before Node's package-scope parser can abort outside the launcher.
  for (const dir of ["bin", "scripts", "lib"]) files[`${dir}/package.json`] = c.encode({ type: "commonjs" });
  files["candidate.json"] = manifestBytes;
  files["private-release.json"] = c.encode({ schema: "dual-authoring-npm/v1", product,
    npm_package: PACKAGES[product], identity: options.identity, asset_scope: options.assetScope,
    authoring_mode: options.authoringMode, candidate_sha256: options.manifestDigest });
  files[`bin/${product}.js`] = Buffer.from(`#!/usr/bin/env node\n"use strict";\nrequire("../scripts/private-npm/launcher").main(${JSON.stringify(product)}, require("node:path").resolve(__dirname, ".."));\n`);
  files["package.json"] = c.encode({ name: PACKAGES[product], version: options.identity.versions[product],
    private: true, description: `Private controlled candidate wrapper for ${product}`,
    license: base.license, engines: base.engines,
    repository: { type: "git", url: `git+https://github.com/${c.REPOSITORY}.git` },
    files: [...Object.keys(files)].sort(), bin: { [product]: `bin/${product}.js` } });
  return files;
}

function verifyPack(tarball, files, destination, env) {
  // Existing system tar handles npm's archive format; no new archive parser.
  const entries = run("/usr/bin/tar", ["-tzf", tarball], env).toString().trimEnd().split("\n");
  const expected = Object.keys(files).map(n => "package/" + n).sort();
  if (JSON.stringify([...entries].sort()) !== JSON.stringify(expected)) throw new Error("npm tar entries differ from exact closure");
  const verbose = run("/usr/bin/tar", ["-tvzf", tarball], env).toString().trimEnd().split("\n");
  if (verbose.length !== expected.length || verbose.some((line, i) =>
    !line.startsWith(/^package\/bin\/[^/]+\.js$/.test(entries[i]) ? "-rwxr-xr-x " : "-rw-r--r-- "))) {
    throw new Error("npm pack contains nonregular entries or unexpected modes");
  }
  fs.mkdirSync(destination, { mode: 0o700 });
  run("/usr/bin/tar", ["-xzf", tarball, "-C", destination, "--no-same-owner", "--same-permissions"], env);
  for (const [name, bytes] of Object.entries(files)) {
    if (!c.readFile(path.join(destination, "package", name)).equals(bytes)) throw new Error(`packed bytes changed: ${name}`);
    const mode = fs.statSync(path.join(destination, "package", name)).mode & 0o777;
    if (mode !== (/^bin\/[^/]+\.js$/.test(name) ? 0o755 : 0o644)) throw new Error(`packed file mode changed: ${name}`);
  }
}


// One exact pack algorithm for both fixed closures; private defaults are intact.
function packPackage(product, files, root, options, context) {
    if (!c.PRODUCTS.includes(product)) throw new Error("unknown fixed npm product");
    const result = JSON.parse(run(options.node, [options.npm, "pack", "--ignore-scripts", "--offline", "--json",
      "--pack-destination", options.output], context.env, root));
    const filename = `${PACKAGES[product]}-${options.identity.versions[product]}.tgz`;
    const record = validateProductPackJSON(result, product, options.identity.versions[product]);
    const tarball = path.join(options.output, filename), bytes = c.readFile(tarball);
    const integrity = "sha512-" + crypto.createHash("sha512").update(bytes).digest("base64");
    const shasum = crypto.createHash("sha1").update(bytes).digest("hex");
    if (record.integrity !== integrity) throw new Error("npm integrity differs from actual pack");
    if (record.shasum !== shasum) throw new Error("npm shasum differs from actual pack");
    verifyPack(tarball, files, path.join(context.root, product), context.env);
    return { file: filename, ...c.metadata(bytes), integrity };
}

function stagePair(options) {
  c.keys(options, ["candidate", "repo", "root", "identity", "manifestDigest", "assetScope", "authoringMode",
    "go", "workParent", "output", "node", "npm"], "private npm options");
  if (options.candidate !== true || options.authoringMode !== MODE) throw new Error("explicit release candidate mode required");
  c.identity(options.identity); c.scopeTargets(options.assetScope);
  for (const key of ["repo", "root", "workParent"]) c.safeDirectory(options[key]);
  const roots = [options.repo, options.root, options.workParent];
  for (let i = 0; i < roots.length; i++) for (let j = i + 1; j < roots.length; j++) {
    if (roots[i] === roots[j] || roots[i].startsWith(roots[j] + path.sep) || roots[j].startsWith(roots[i] + path.sep)) {
      throw new Error("private source/candidate/scratch roots overlap");
    }
  }
  for (const key of ["go", "node", "npm"]) {
    if (!path.isAbsolute(options[key])) throw new Error("tool path must be absolute");
    c.readFile(options[key]);
  }
  c.outputPlacement(options.output, [...roots, ...["go", "node", "npm"].map(k => path.dirname(options[k]))]);
  const context = npmContext(options.workParent);
  const source = blobs(options.repo, options.identity.commit, context.env);
  const manifestBytes = c.readFile(path.join(options.root, "candidate.json"), 1024 * 1024);
  // Refuse mutable inputs, without chmod or any write to the candidate.
  for (const name of ["", ...fs.readdirSync(options.root)]) {
    if (fs.lstatSync(path.join(options.root, name)).mode & 0o222) throw new Error("candidate must be sealed");
  }
  producer.verifyCandidate({ candidate: true, root: options.root, identity: options.identity,
    manifestDigest: options.manifestDigest, assetScope: options.assetScope, authoringMode: options.authoringMode,
    go: options.go, workParent: options.workParent });
  const tools = Object.fromEntries(["node", "npm", "go"].map(k => [k, {
    path: options[k], sha256: c.digest(c.readFile(options[k]))
  }]));
  tools.node.version = run(options.node, ["--version"], context.env).toString().trim();
  tools.npm.version = run(options.node, [options.npm, "--version"], context.env).toString().trim();
  tools.go.version = run(options.go, ["version"], context.env).toString().trim();
  for (const name of ["git", "tar"]) tools[name] = { path: `/usr/bin/${name}`,
    sha256: c.digest(c.readFile(`/usr/bin/${name}`)), version: run(`/usr/bin/${name}`, ["--version"], context.env).toString().split("\n")[0] };
  tools.stager_node = { path: process.execPath, version: process.version, sha256: c.digest(c.readFile(process.execPath)) };
  fs.mkdirSync(options.output, { mode: 0o700 }); // exclusive reservation, retain failures
  const packs = {}, generated = {}, packFiles = {};
  for (const product of c.PRODUCTS) {
    const root = path.join(options.output, product); fs.mkdirSync(root, { mode: 0o700 });
    const files = packageFiles(product, source, manifestBytes, options); packFiles[product] = files;
    generated[product] = Object.fromEntries(Object.entries(files).map(([n, b]) => [n, c.digest(b)]));
    for (const [name, bytes] of Object.entries(files)) write(path.join(root, name), bytes, /^bin\/[^/]+\.js$/.test(name) ? 0o755 : 0o644);
    packs[product] = packPackage(product, files, root, options, context);
  }
  for (const name of [...COMMON, "candidate.json"]) {
    if (!packFiles.agentplugins[name].equals(packFiles["plugin-kit-ai"][name])) throw new Error("shared pack bytes differ");
  }
  // Freeze completion only after both actual packs verify, and recheck inputs.
  c.frozenCandidate(options.root, options.identity, options.manifestDigest, options.assetScope, options.authoringMode);
  if (!manifestBytes.equals(c.readFile(path.join(options.root, "candidate.json")))) throw new Error("candidate changed while packing");
  for (const pack of Object.values(packs)) if (c.digest(c.readFile(path.join(options.output, pack.file))) !== pack.sha256) throw new Error("pack changed before completion");
  const record = { schema: "dual-authoring-npm-completion/v1", status: "CANDIDATE", identity: options.identity,
    candidate_sha256: options.manifestDigest, asset_scope: options.assetScope, authoring_mode: MODE,
    wrapper_blobs: Object.fromEntries(Object.entries(source).map(([n, { bytes, ...pin }]) => [n, pin])),
    generated, packs, tools, evidence: context.root, release_eligible: false, platform_acceptance: false, attested: false };
  completeRecord(options.output, record);
  return record;
}

function completeRecord(output, record) {
  // Complete the bytes before exclusive publication. Never remove a collision.
  const temporary = path.join(output, "completion.pending.json");
  write(temporary, c.encode(record), 0o444);
  const marker = path.join(output, "completion.json");
  fs.linkSync(temporary, marker);
  try { fs.unlinkSync(temporary); } catch (error) {
    try {
      const owned = fs.lstatSync(temporary), named = fs.lstatSync(marker);
      if (owned.ino !== named.ino || owned.dev !== named.dev) throw new Error("completion cleanup identity changed");
      fs.unlinkSync(marker);
    } catch (cleanup) { throw new AggregateError([error, cleanup], "completion cleanup uncertainty"); }
    throw error;
  }
}

if (require.main === module) {
  try {
    if (process.argv.length !== 4 || process.argv[2] !== "--candidate" || !path.isAbsolute(process.argv[3])) throw new Error("usage: stage-dual-authoring-npm.js --candidate <absolute-config.json>");
    process.stdout.write(c.encode(stagePair(JSON.parse(c.readFile(process.argv[3], 1024 * 1024)))));
  } catch (error) { process.stderr.write(`private npm pair: ${error.message}\n`); process.exitCode = 1; }
}
module.exports = { completeRecord, packPackage, stagePair, verifyPack, npmContext, blobs, packageFiles, ALLOWLIST, COMMON, MODE, PACKAGES };
