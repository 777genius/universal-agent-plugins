#!/usr/bin/env node
"use strict";

// Opt-in offline producer. No published wrapper, workflow, or default build calls
// this module. Compile only the two known mains; never run package scripts,
// generators, downloaded executables, or binaries supplied by a caller.
const fs = require("node:fs");
const path = require("node:path");
const cp = require("node:child_process");
const c = require("./dual-authoring-candidate");

const SOURCE_PATHS = ["go.mod", "go.work", "go.work.sum", "cli", "install", "sdk"];
const GO_VERSION = "go1.25.13";

function run(command, args, options = {}) {
  return cp.execFileSync(command, args, { timeout: 10 * 60 * 1000, maxBuffer: 64 * 1024 * 1024, ...options });
}

function optIn(options, fields) {
  c.keys(options, ["candidate", ...fields], "options");
  if (options.candidate !== true) throw new Error("explicit candidate opt-in required");
  c.identity(options.identity);
}

function privateContext(workParent) {
  c.safeDirectory(workParent);
  const root = fs.mkdtempSync(path.join(workParent, "dual-authoring-"));
  fs.chmodSync(root, 0o700);
  for (const name of ["home", "tmp", "cache", "config", "data", "state", "bin", "source"]) {
    fs.mkdirSync(path.join(root, name), { mode: 0o700 });
  }
  // Start from an allowlist; inherited GOFLAGS, toolchain, profiles, credentials,
  // proxy settings, NODE_OPTIONS and Git config cannot change this build.
  const env = {
    PATH: "/usr/bin:/bin", HOME: path.join(root, "home"), USERPROFILE: path.join(root, "home"),
    TMPDIR: path.join(root, "tmp"), TMP: path.join(root, "tmp"), TEMP: path.join(root, "tmp"),
    XDG_CONFIG_HOME: path.join(root, "config"), XDG_CACHE_HOME: path.join(root, "cache"),
    XDG_DATA_HOME: path.join(root, "data"), XDG_STATE_HOME: path.join(root, "state"),
    APPDATA: path.join(root, "config"), LOCALAPPDATA: path.join(root, "data"),
    GIT_CONFIG_NOSYSTEM: "1", GIT_CONFIG_GLOBAL: "/dev/null", GIT_TERMINAL_PROMPT: "0",
    GIT_NO_REPLACE_OBJECTS: "1", GIT_NO_LAZY_FETCH: "1",
    GIT_CONFIG_COUNT: "1", GIT_CONFIG_KEY_0: "protocol.allow", GIT_CONFIG_VALUE_0: "never",
    GOENV: "off", GOTOOLCHAIN: "local", GOPROXY: "off", GOSUMDB: "off", GOVCS: "*:off",
    GOMAXPROCS: "2", GOCACHE: path.join(root, "cache"), CGO_ENABLED: "0", LC_ALL: "C", TZ: "UTC"
  };
  return { root, env };
}

function toolchain(go, context) {
  if (typeof go !== "string" || !path.isAbsolute(go)) throw new Error("trusted Go tool path must be absolute");
  const hash = c.digest(c.readFile(go));
  const info = JSON.parse(run(go, ["env", "-json", "GOVERSION", "GOHOSTOS", "GOHOSTARCH"], { env: context.env }));
  if (info.GOVERSION !== GO_VERSION || info.GOHOSTOS !== "linux" || info.GOHOSTARCH !== "amd64") {
    throw new Error("this bounded candidate producer/verifier requires trusted Go 1.25.13 on Linux amd64");
  }
  return hash;
}

function buildInfo(info, product, target, id) {
  if (!info || info.GoVersion !== GO_VERSION || info.Path !== `github.com/777genius/plugin-kit-ai/cli/cmd/${product}` ||
      !Array.isArray(info.Settings)) throw new Error("embedded Go product/toolchain identity mismatch");
  const settings = new Map();
  for (const setting of info.Settings) {
    if (settings.has(setting.Key)) throw new Error("duplicate Go build setting");
    settings.set(setting.Key, setting.Value);
  }
  const [os, arch] = target.split("-");
  const required = {
    GOOS: os, GOARCH: arch, CGO_ENABLED: "0", "-buildmode": "exe", "-compiler": "gc",
    "-ldflags": c.linkerFlags(product, id)
  };
  for (const [key, value] of Object.entries(required)) {
    if (settings.get(key) !== value) throw new Error(`embedded Go build setting mismatch: ${product}/${target}/${key}`);
  }
}

function inspectBinary(go, file, product, target, id, env) {
  // `go version` reads build info; it never starts the subject executable.
  buildInfo(JSON.parse(run(go, ["version", "-m", "-json", file], { env })), product, target, id);
}

function sourceSnapshot(repo, commit, context) {
  c.safeDirectory(repo);
  const options = { cwd: repo, env: context.env };
  const head = run("/usr/bin/git", ["rev-parse", "HEAD"], options).toString().trim();
  if (head !== commit) throw new Error("declared source commit must equal checkout HEAD");
  const tree = run("/usr/bin/git", ["ls-tree", "-r", "-z", commit, "--", ...SOURCE_PATHS], options).toString();
  for (const entry of tree.split("\0").filter(Boolean)) {
    const match = /^(100644|100755) blob [0-9a-f]{40}\t(.+)$/.exec(entry);
    if (!match || /[\x00-\x1f\x7f]/.test(match[2]) || match[2].split("/").some((part) => !part || part === "." || part === "..") ||
        match[2].startsWith("/") || match[2].includes("\\")) throw new Error("unsafe frozen source tree entry");
  }
  const snapshot = path.join(context.root, "source.tar");
  run("/usr/bin/git", ["-c", "core.attributesFile=/dev/null", "archive", "--format=tar", "--output", snapshot, commit, "--", ...SOURCE_PATHS], options);
  const body = c.readFile(snapshot);
  run("/usr/bin/tar", ["-xf", snapshot, "-C", path.join(context.root, "source"), "--no-same-owner"], { env: context.env });
  // Git archive applies export attributes. Check each extracted tracked blob
  // against the exact Git object, so local attributes/export-subst cannot alter
  // compiled source. Missing/export-ignored inputs fail instead of weakening proof.
  for (const entry of tree.split("\0").filter(Boolean)) {
    const [description, name] = entry.split("\t");
    const object = description.split(" ")[2];
    const file = path.join(context.root, "source", name);
    const bytes = fs.readFileSync(file); // tree above forbids all links
    const actual = require("node:crypto").createHash("sha1")
      .update(`blob ${bytes.length}\0`).update(bytes).digest("hex");
    if (actual !== object) throw new Error("source snapshot does not match exact Git blob");
  }
  return c.digest(body);
}

function writeExclusive(file, body, mode = 0o444) {
  fs.writeFileSync(file, body, { flag: "wx", mode });
  fs.chmodSync(file, mode);
}

function stageCandidate(options) {
  optIn(options, ["repo", "output", "workParent", "go", "modCache", "identity", "assetScope"]);
  const targets = c.scopeTargets(options.assetScope);
  const id = structuredClone(options.identity);
  c.safeDirectory(options.modCache);
  c.safeDirectory(options.workParent);
  c.outputPlacement(options.output, [options.repo, options.modCache, options.workParent, path.dirname(options.go)]);
  const context = privateContext(options.workParent);
  const goHash = toolchain(options.go, context);
  const sourceHash = sourceSnapshot(options.repo, id.commit, context);
  const env = { ...context.env, GOMODCACHE: options.modCache, GOWORK: path.join(context.root, "source", "go.work") };
  const manifest = {
    schema: c.SCHEMA, status: "CANDIDATE", identity: id, asset_scope: options.assetScope,
    build: { method: "controlled-git-archive-go-build/v1", go_version: GO_VERSION, go_sha256: goHash,
      source_archive_sha256: sourceHash, authoring_mode: "vertical-slice-v1" },
    products: {}, release_eligible: false
  };
  // Reserve an absent destination exclusively. Partial output has no candidate
  // marker. Existing output is never reused, replaced, or recursively removed.
  c.outputPlacement(options.output, [options.repo, options.modCache, options.workParent, path.dirname(options.go)]);
  fs.mkdirSync(options.output, { mode: 0o700 });
  const log = [];
  const marker = path.join(options.output, "candidate.json");
  let markerOwned = false;
  try {
    for (const product of c.PRODUCTS) {
      const assets = {};
      manifest.products[product] = { version: id.versions[product], assets };
      for (const target of targets) {
        const [os, arch] = target.split("-");
        const binaryPath = path.join(context.root, "bin", `${product}-${target}`);
        // No trimpath: Go intentionally omits -ldflags from build info with
        // trimpath. Preserve the exact embedded version/engine linker settings
        // for byte inspection. Reproducibility is a later, separate gate.
        const args = ["build", "-p", "2", "-buildvcs=false", "-mod=readonly", "-ldflags", c.linkerFlags(product, id),
          "-o", binaryPath, `./cli/plugin-kit-ai/cmd/${product}`];
        run(options.go, args, { cwd: path.join(context.root, "source"), env: { ...env, GOOS: os, GOARCH: arch } });
        inspectBinary(options.go, binaryPath, product, target, id, env);
        const binary = c.readFile(binaryPath);
        const file = c.assetName(product, id.versions[product], target);
        const binaryName = c.executableName(product, target);
        const bytes = product === "plugin-kit-ai" ? c.archive(binary, binaryName) : binary;
        assets[target] = { file, ...c.metadata(bytes), binary: { file: binaryName, ...c.metadata(binary) } };
        writeExclusive(path.join(options.output, file), bytes);
        log.push({ product, target, argv: [options.go, ...args], binary_sha256: c.digest(binary) });
        // A private transcript is evidence of this local invocation, not a
        // portable attestation. It is outside the closed candidate file set.
        fs.writeFileSync(path.join(context.root, "build-log.json"), c.encode(log));
      }
    }
    c.manifestShape(manifest, id, options.assetScope);
    const body = c.encode(manifest);
    const manifestHash = c.digest(body);
    // Validate the complete set in a separate private root before publishing the
    // final marker. Copies are independent bytes, never hardlinks.
    const checkRoot = path.join(context.root, "check");
    fs.mkdirSync(checkRoot, { mode: 0o700 });
    for (const file of fs.readdirSync(options.output)) writeExclusive(path.join(checkRoot, file), c.readFile(path.join(options.output, file)));
    writeExclusive(path.join(checkRoot, "candidate.json"), body);
    verifyCandidate({ candidate: true, root: checkRoot, identity: id, manifestDigest: manifestHash,
      go: options.go, workParent: options.workParent, assetScope: options.assetScope });
    // Recheck final output bytes immediately before committing the marker.
    for (const product of c.PRODUCTS) for (const asset of Object.values(manifest.products[product].assets)) {
      if (c.digest(c.readFile(path.join(options.output, asset.file))) !== asset.sha256) throw new Error("staged bytes changed");
    }
    if (fs.readdirSync(options.output).length !== targets.length * 2) throw new Error("unexpected partial output entry");
    // Own the marker as soon as exclusive creation succeeds, before any write
    // or finalization can throw. A failed open never authorizes its removal.
    const markerFd = fs.openSync(marker, "wx", 0o444);
    markerOwned = true;
    try {
      fs.writeFileSync(markerFd, body);
    } catch (error) {
      // Preserve the write failure even if closing also fails; retain that
      // secondary diagnostic without retrying a possibly closed descriptor.
      try { fs.closeSync(markerFd); } catch (closeError) { error.closeError = closeError; }
      throw error;
    }
    fs.closeSync(markerFd);
    fs.chmodSync(marker, 0o444);
    fs.chmodSync(options.output, 0o555);
    return { status: "CANDIDATE", manifest_sha256: manifestHash, output: options.output,
      local_build_evidence: context.root, release_eligible: false, platform_acceptance: false, attested: false };
  } catch (error) {
    // Leave inspectable partial bytes but no success marker, even if final chmod
    // failed. Do not delete caller files or erase the failed build's evidence.
    if (markerOwned) {
      try {
        try { fs.unlinkSync(marker); }
        catch (cleanupError) {
          if (!["EACCES", "EPERM"].includes(cleanupError.code)) throw cleanupError;
          // Directory finalization may have changed permissions before throwing.
          // Only this invocation's reserved output is made writable for cleanup.
          fs.chmodSync(options.output, 0o700);
          fs.unlinkSync(marker);
        }
      } catch (cleanupError) {
        throw new AggregateError([error, cleanupError],
          `${error.message}; candidate marker cleanup failed: ${cleanupError.message}`, { cause: error });
      }
    }
    throw error;
  }
}

function verifyCandidate(options) {
  optIn(options, ["root", "identity", "manifestDigest", "go", "workParent", "assetScope"]);
  c.safeDirectory(options.workParent);
  if (options.workParent === options.root || options.workParent.startsWith(options.root + path.sep)) {
    throw new Error("verification scratch must be outside candidate output");
  }
  const frozen = c.frozenCandidate(options.root, options.identity, options.manifestDigest, options.assetScope);
  const context = privateContext(options.workParent);
  const goHash = toolchain(options.go, context);
  if (goHash !== frozen.manifest.build.go_sha256) throw new Error("trusted Go tool digest mismatch");
  for (const { product, target, binary } of frozen.binaries) {
    const file = path.join(context.root, "bin", `${product}-${target}`);
    writeExclusive(file, binary); // no executable bit; no execution during verify
    inspectBinary(options.go, file, product, target, options.identity, context.env);
  }
  return { status: "CANDIDATE", manifest_sha256: frozen.manifest_sha256,
    consistency_verified: true, release_eligible: false, platform_acceptance: false, attested: false };
}

function main(argv) {
  const [command, opt, config] = argv;
  if (argv.length !== 3 || opt !== "--candidate" || !["stage", "verify"].includes(command)) {
    throw new Error("usage: stage-dual-authoring-candidate.js <stage|verify> --candidate <absolute-config.json>");
  }
  if (!path.isAbsolute(config)) throw new Error("config path must be absolute");
  const options = JSON.parse(c.readFile(config, 1024 * 1024));
  return command === "stage" ? stageCandidate(options) : verifyCandidate(options);
}

if (require.main === module) {
  try { process.stdout.write(JSON.stringify(main(process.argv.slice(2)), null, 2) + "\n"); }
  catch (error) { process.stderr.write(`dual authoring candidate: ${error.message}\n`); process.exitCode = 1; }
}

module.exports = { stageCandidate, verifyCandidate, buildInfo, sourceSnapshot, privateContext, main };
