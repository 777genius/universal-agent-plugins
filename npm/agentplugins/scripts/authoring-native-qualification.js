#!/usr/bin/env node
"use strict";

// N1 observes one frozen pair. This is not provider admission or qualification
// signing. No promotion caller accepts these local records in this checkpoint.
const fs = require("node:fs");
const path = require("node:path");
const os = require("node:os");
const cp = require("node:child_process");
const crypto = require("node:crypto");
const zlib = require("node:zlib");
const assert = require("node:assert/strict");
const c = require("./dual-authoring-candidate");
const { verifyProjectedPair } = require("./authoring-release");
const { buildInfo } = require("./stage-dual-authoring-candidate");
const { lifecycleResult } = require("./platform-proof");
const SCHEMA = "authoring-frozen-native/v1";
const WORKFLOW = ".github/workflows/authoring-frozen-native.yml";
const PREPARATION = ".github/workflows/agentplugins-release.yml";
const TARGET = "linux-amd64";
const MODE = "release-cli-contract-v1";
const LIMIT = 1024 * 1024;
const CASES = Object.freeze(["skill", "mcp-remote", "mcp-stdio", "hybrid-remote", "hybrid-stdio"]);
const LEAVES = Object.freeze(["author.capabilities", "author.compat", "author.doctor", "author.init", "author.inspect",
  "author.skills.init", "author.skills.validate", "author.test", "author.validate"]);
const FILES = Object.freeze(["transcripts.json", "trees.json", "build-info.json", "preservation.json", "preparation.json", "host.json", "scans.json", "acquisition.json"]);
// Fixed identities from conformance/profile.go, specregistry/registry.go,
// readiness.Engine and domain.ClientDefinitions at this reviewed source.
const SCHEMAS = [
  { id: "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json", digest: "sha256:0a4aad95ce337878ad38802ebf0daa3fde76abe3f65400c86bcbb1ec0b3ab883" },
  { id: "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json", digest: "sha256:6539175bfcdf43085855183e86da40ea94b166547a72b47ae9a0a390516d3acb" }
];
const PROFILES = [
  { id: "agent-plugins/1.0.0", revision: "ff8ab5e392cc87bd88d87c060815a87490e51003", digest: "sha256:97a658b7dca3ce1b4c2266b95da300fa51d9dc4ade59d73168e5f9104272da18" },
  { id: "agent-skills/2026-09-06", revision: "69ef37e9424c0a7ea9dd2293b559e43ec8176379", digest: "sha256:b9079c0c10b7930e8c6a20ff2bc10cda2a3343c55185120e3f1116a1a529b220" },
  ...SCHEMAS.map(x => ({ ...x, revision: "1.0.0" })),
  { id: "author-document-bounds/v1", revision: "1", digest: "sha256:4b8ab8fd50481ccd1a0b777dcbbfa06cf89516a5ea61ce09d56d6dd6a2c43004" }
];
function capabilities() {
  const rows = [
    ["chatgpt", "compatibility_projection", "projected", "unsupported", "unsupported"],
    ["claude", "compatibility_projection", "projected", "projected", "unsupported", "automatic"],
    ["cline", "native", "native", "native", "unsupported", "automatic"],
    ["codex", "compatibility_projection", "projected", "projected", "unsupported"],
    ["copilot", "native", "native", "native", "native"], ["cursor", "native", "native", "native", "native"],
    ["gemini", "native", "native", "native", "unsupported"], ["kiro", "native", "native", "native", "unsupported"],
    ["opencode", "prepared_package", "prepared", "prepared", "unsupported", "automatic"],
    ["vscode", "prepared_package", "prepared", "prepared", "prepared"],
    ["windsurf", "prepared_package", "prepared", "prepared", "prepared"]
  ];
  return { schemas: SCHEMAS, profiles: PROFILES, commands: LEAVES,
    evidence_limits: ["static_only", "no_path_lookup", "no_executable_version_probe", "no_runtime_or_oauth_evidence", "native_files_metadata_only"],
    clients: rows.map(([client_id, package_mode, skill_support, mcp, extension_support, activation_mode = "manual"]) => ({
      client_id, package_mode, activation_mode, scopes: ["user"], skill_support,
      mcp_transports: { stdio: mcp, "streamable-http": mcp, sse: mcp },
      app_support: client_id === "chatgpt" ? "projected" : "unsupported", extension_support })) };
}
const POLICY = { id: "agent-plugin-install", version: 2,
  digest: "sha256:9cf869e299d847d7078aeca01f5a182fcdb0144bbf513c83290459991c79e037" };
const BROKEN = "---\nname: [\n---\nBroken\n";
const TOP = ["schema", "lane", "identity", "candidate_sha256", "pair_marker_sha256", "projection_pins", "subject",
  "peer_subject", "preparation", "producer", "host", "tools", "assertions", "evidence"];
const fail = message => { throw new Error(message); };
const exact = (a, b, label) => assert.deepEqual(a, b, label);
function sha(s, length = 64) {
  assert.equal(typeof s, "string");
  assert.match(s, new RegExp(`^[0-9a-f]{${length}}$`));
  assert.ok(!/^0+$/.test(s)); return s;
}
function positive(n) { assert.ok(Number.isSafeInteger(n) && n > 0 && n <= Number.MAX_SAFE_INTEGER); return n; }
// Tokenize only to reject duplicate object keys and excessive nesting; JSON.parse
// still owns grammar validation. This accepts Go's whitespace/field order without
// accepting duplicate keys or ambiguous UTF-8 from a successful child process.
function jsonDocument(text) {
  assert.equal(typeof text, "string"); assert.ok(Buffer.byteLength(text) <= LIMIT);
  const tokens = text.match(/"(?:[^"\\\x00-\x1f]|\\.)*"|[{}\[\]:,]|true|false|null|-?(?:0|[1-9]\d*)(?:\.\d+)?(?:[eE][+-]?\d+)?/g) || [];
  const stack = []; let expectKey = false;
  for (let i = 0; i < tokens.length; i++) {
    const token = tokens[i];
    if (token === "{" || token === "[") {
      stack.push(token === "{" ? new Set() : null); assert.ok(stack.length <= 64); expectKey = token === "{";
    } else if (token === "}" || token === "]") { stack.pop(); expectKey = false; }
    else if (token === ",") expectKey = stack[stack.length - 1] !== null;
    else if (expectKey && token.startsWith('"')) {
      const key = JSON.parse(token), keys = stack[stack.length - 1];
      assert.ok(keys && !keys.has(key), "duplicate JSON key"); keys.add(key); expectKey = false;
    }
  }
  return JSON.parse(text);
}
function canonical(bytes, limit = LIMIT) {
  assert.ok(Buffer.isBuffer(bytes) && bytes.length > 0 && bytes.length <= limit, "bounded JSON required");
  const v = JSON.parse(bytes.toString("utf8"));
  exact(bytes, c.encode(v), "canonical UTF-8 JSON, no duplicate keys"); return v;
}
function invocation(v, workflow, source) {
  c.keys(v, ["repository", "workflow", "source", "workflow_sha", "run_id", "run_attempt"], "invocation");
  exact([v.repository, v.workflow, v.source, v.workflow_sha], [c.REPOSITORY, workflow, source, source]);
  sha(source, 40); positive(v.run_id); positive(v.run_attempt); assert.ok(v.run_attempt <= 1000);
  return v;
}
function subject(manifest, product) {
  return { product, target: TARGET, ...manifest.products[product].assets[TARGET] };
}
function preparationRecord(root, pins, producer) {
  const verified = verifyProjectedPair(root, pins);
  invocation(producer, PREPARATION, pins.identity.commit);
  return { schema: "authoring-preparation-run/v1", identity: pins.identity, producer,
    subjects: verified.subjects.map(s => ({ file: path.relative(root, s.file).split(path.sep).join("/"),
      ...c.metadata(c.readFile(s.file)) })) };
}
function writePreparation(root, pins, producer) {
  const v = preparationRecord(root, pins, producer);
  const file = path.join(root, "preparation-run.json");
  fs.writeFileSync(file, c.encode(v), { flag: "wx", mode: 0o444 });
  return { file, ...c.metadata(c.readFile(file)) };
}
function readPreparation(root, pins, expected) {
  c.keys(expected, ["sha256", "producer"], "preparation pin"); sha(expected.sha256);
  const bytes = c.readFile(path.join(root, "preparation-run.json"), LIMIT);
  exact(c.digest(bytes), expected.sha256);
  const value = canonical(bytes);
  exact(value, preparationRecord(root, pins, expected.producer), "all eighteen preparation input subjects");
  return value;
}
// Only retained owned trees are read; bounds include empty files/directories and
// modes. No recursive cleanup, special files, aliases or silent exclusions.
function tree(root) {
  c.safeDirectory(root);
  const result = []; let total = 0;
  function walk(rel) {
    assert.ok(result.length < 4096, "tree entry bound");
    const file = rel === "." ? root : path.join(root, rel), st = fs.lstatSync(file);
    assert.equal(st.mode & 0o7000, 0);
    assert.ok(!st.isSymbolicLink() && (st.isDirectory() || st.isFile() && st.nlink === 1));
    const item = { path: rel, mode: st.mode & 0o777, kind: st.isDirectory() ? "directory" : "file" };
    if (st.isFile()) {
      total += st.size; assert.ok(total <= 16 * LIMIT, "tree byte bound");
      // c.readFile intentionally rejects empty candidate assets. An empty
      // generated file is still part of the tree identity.
      const bytes = st.size ? c.readFile(file, 16 * LIMIT) : fs.readFileSync(file, { flag: fs.constants.O_RDONLY | fs.constants.O_NOFOLLOW });
      Object.assign(item, c.metadata(bytes));
    }
    result.push(item);
    if (st.isDirectory()) for (const name of fs.readdirSync(file).sort()) {
      assert.ok(!/[\\\x00-\x1f]/.test(name)); walk(rel === "." ? name : `${rel}/${name}`);
    }
  }
  walk("."); return result;
}
function inputCustody(root, subjects) {
  return [...subjects.map(x => x.file), path.join(root, "preparation-run.json")].map(file => {
    const st = fs.lstatSync(file);
    return { file: path.relative(root, file), mode: st.mode, dev: st.dev, ino: st.ino,
      links: st.nlink, size: st.size, mtime: st.mtimeMs, ctime: st.ctimeMs };
  });
}
function context(root) {
  fs.mkdirSync(root, { mode: 0o700 });
  for (const name of ["home", "tmp", "state", "config", "cache", "data", "empty-bin", "projects"])
    fs.mkdirSync(path.join(root, name), { mode: 0o700 });
  // Credential-free allowlist. Even workflow tokens and caller proxy settings
  // are discarded. PATH contains neither product nor ambient toolchains.
  return { root, env: { PATH: path.join(root, "empty-bin"), HOME: path.join(root, "home"),
    TMPDIR: path.join(root, "tmp"), XDG_CONFIG_HOME: path.join(root, "config"),
    XDG_CACHE_HOME: path.join(root, "cache"), XDG_DATA_HOME: path.join(root, "data"),
    XDG_STATE_HOME: path.join(root, "state"), AGENTPLUGINS_HOME: path.join(root, "state"),
    GOENV: "off", GOTOOLCHAIN: "local", GOPROXY: "off", GOSUMDB: "off", GOVCS: "*:off",
    GOMAXPROCS: "2", LC_ALL: "C", TZ: "UTC", NO_COLOR: "1" } };
}
// Linux owned process group supervision. Escaped descendants/whole-OS network
// observation are NOT proved by kill(0) or PATH traps; see observationGate.
function subprocess(file, argv, ctx, signal, installer = false) {
  return new Promise((resolve, reject) => {
    if (signal?.aborted) return reject(new Error("cancelled before process"));
    const child = cp.spawn(file, argv, { cwd: path.join(ctx.root, "projects"), env: ctx.env,
      shell: false, detached: true, stdio: ["ignore", "pipe", "pipe"] });
    let stdout = [], stderr = [], bytes = 0, problem, closed = false, settled = false, stopped = false, grace;
    const finish = error => {
      if (settled) return; settled = true;
      clearTimeout(timer); clearTimeout(grace); signal?.removeEventListener("abort", cancel);
      if (error) reject(new Error(error));
    };
    const uncertain = () => {
      let detail = "";
      try { child.stdout.destroy?.(); child.stderr.destroy?.(); child.unref?.(); }
      catch (e) { detail = `; stream detach failed: ${e.code || "unknown"}`; }
      finish(`${problem}; cleanup uncertain: child close not observed${detail}`);
    };
    const stop = reason => {
      if (stopped || settled) return;
      stopped = true; problem ||= reason;
      if (child.pid && !closed) {
        try { process.kill(-child.pid, "SIGKILL"); }
        catch (e) { if (e.code !== "ESRCH") { problem += `; owned-child cleanup denied: ${e.code}`; uncertain(); return; } }
      }
      // One cleanup attempt only. A missing close (including inherited pipes)
      // must not hold failure diagnostics hostage. Unref is not cleanup proof.
      grace = setTimeout(uncertain, 1000);
    };
    const timer = setTimeout(() => stop("timeout"), installer ? 120000 : 15000);
    const cancel = () => stop("cancelled"); signal?.addEventListener("abort", cancel, { once: true });
    for (const [stream, chunks] of [[child.stdout, stdout], [child.stderr, stderr]]) stream.on("data", b => {
      if (settled) return;
      bytes += b.length;
      if (bytes > LIMIT) stop("output flood"); else chunks.push(b);
    });
    child.on("error", e => stop(`subprocess failed: ${e.code}`));
    child.on("close", (status, sig) => {
      closed = true;
      if (settled) return; // Never retry denied cleanup after a late close.
      if (child.pid) {
        try {
          process.kill(-child.pid, 0); problem ||= "owned descendants survived";
          if (!stopped) {
            stopped = true;
            try { process.kill(-child.pid, "SIGKILL"); }
            catch (e) { if (e.code !== "ESRCH") problem += `; owned-child cleanup denied: ${e.code}`; }
          }
        } catch (e) { if (e.code !== "ESRCH") problem ||= `cleanup uncertain: ${e.code}`; }
      }
      if (problem || sig) return finish(problem || `signal ${sig}`);
      try {
        const decoder = new TextDecoder("utf-8", { fatal: true });
        const result = { status, stdout: decoder.decode(Buffer.concat(stdout)), stderr: decoder.decode(Buffer.concat(stderr)) };
        finish(); resolve(result);
      } catch { finish("invalid UTF-8 subprocess output"); }
    });
  });
}

function initArgs(lane) {
  const args = ["init", lane, "--template", lane.startsWith("hybrid-") ? "hybrid" : lane];
  if (lane.startsWith("hybrid-")) args.push("--mcp-template", lane.endsWith("remote") ? "mcp-remote" : "mcp-stdio");
  if (lane.endsWith("remote")) args.push("--url", "https://docs.example.com/mcp");
  if (lane.endsWith("stdio")) args.push("--runtime", "node");
  return args;
}
// Closed inventory is owned by producer and reader, never supplied by callers.
function commands(product) {
  const rows = [];
  const add = (id, args, status = 0, author = true, lane = null) => rows.push({ id, args:
    [...(author && product === "agentplugins" ? ["author"] : []), ...args, "--format=json"], status, author, lane });
  add("product-version", ["version"], 0, false);
  rows.push({ id: "product-help", args: ["--help"], status: 0, author: false, lane: null });
  add("engine-version", ["version"]); add("author-help", ["--help"]); add("capabilities", ["capabilities"]);
  for (const lane of CASES) {
    add(`${lane}/init`, initArgs(lane), 0, true, lane);
    add(`${lane}/extra-skill`, ["skills", "init", "extra-skill", lane, "--description", "Use for extra documentation requests"], 0, true, lane);
    for (const verb of ["skills validate", "validate", "inspect", "test", "compat", "doctor"]) {
      const args = [...verb.split(" "), lane];
      if (verb === "compat") args.push("--target", "claude,codex");
      add(`${lane}/${verb.replace(" ", "-")}`, args, verb === "doctor" && lane !== "skill" ? 1 : 0, true, lane);
    }
    add(`${lane}/existing`, initArgs(lane), 1, true, lane);
  }
  add("malformed-skill", ["validate", "skill"], 1, true, "skill");
  add("invalid-flag", ["init", "invalid-destination", "--force"], 2);
  add("missing-template-input", ["init", "missing-destination", "--template", "mcp-stdio"], 2);
  add("installer-flag", ["validate", "skill", "--scope=user"], 2, true, "skill");
  if (product === "plugin-kit-ai") add("retired-v1", ["update", "--all"], 2, false);
  return rows;
}
function envelope(row, expected) {
  exact(row.status, expected.status, `${expected.id} exit`); exact(row.stderr, "", `${expected.id} stderr`);
  if (expected.id === "product-help") {
    assert.ok(row.stdout.length > 50 && row.stdout.length < LIMIT); return null;
  }
  const v = jsonDocument(row.stdout); // one complete document; trailing prompts fail
  exact(v.schema_version, 1); exact(v.result, expected.status === 0 ? "success" : "failure");
  assert.ok(v.data && !Array.isArray(v.data)); return v;
}
function authorResult(row, spec, product, identity) {
  const r = envelope(row, spec);
  if (!r) return;
  if (!spec.author) {
    if (spec.id === "product-version") {
      exact(r.command, product === "agentplugins" ? "version" : "author.version", "product version command identifier");
      exact(r.data[product === "agentplugins" ? "version" : "product_version"], identity.versions[product]);
    }
    if (spec.id === "retired-v1") exact(r.data.error?.code, "v1_operation_unavailable");
    return;
  }
  const args = spec.args.slice(product === "agentplugins" ? 1 : 0);
  const operation = args[0] === "--help" ? "author" : `author.${args[0]}${args[0] === "skills" ? `.${args[1]}` : ""}`;
  exact(r.command, operation, "author command identifier");
  const d = r.data;
  exact(d.revision, identity.commit); exact(d.engine, "standard-first-slice/1");
  exact(d.authoring_schema_version, 1); exact(d.engine_version, d.engine);
  exact(d.runtime_evidence.status, "not_evaluated");
  if (spec.id === "author-help" || spec.id === "capabilities") exact(d.commands, LEAVES);
  if (spec.id === "engine-version") { exact(d.product, product); exact(d.product_version, identity.versions[product]); return; }
  if (spec.id === "capabilities") { exact(d.capabilities, capabilities(), "fixed capabilities inventory"); return; }
  if (spec.id === "author-help") { assert.ok(d.help.use.startsWith(product === "agentplugins" ? "agentplugins author" : "plugin-kit-ai")); return; }
  const mutation = /\/(init|extra-skill)$/.test(spec.id);
  if (mutation) { exact(d.committed, true); exact(d.effects.committed, true); }
  else exact(d.committed, false);
  if (!spec.lane || /\/(existing)$/.test(spec.id) || spec.id === "installer-flag") return;
  assert.match(d.identity.tree_digest, /^sha256:[0-9a-f]{64}$/);
  exact(d.identity.read_profile, "packageview-local-linux-v1");
  exact(d.profiles, PROFILES, "exact conformance profiles");
  exact(d.schema_ids, (spec.lane === "skill" ? [SCHEMAS[0].id] : SCHEMAS.map(x => x.id).sort()), "accepted package schema inventory");
  exact(d.loadability.status, "pass");
  exact(d.normative_conformance.status, spec.id === "malformed-skill" ? "fail" : "pass");
  exact(d.authoring_readiness.status, spec.id === "malformed-skill" ? "fail" : "pass");
  exact(d.release_policy.status, "not_evaluated");
  if (spec.id === "malformed-skill") {
    assert.ok(d.components.some(x => x.type === "skill" && x.status === "pass"), "valid Skills survive malformed peer");
    assert.ok(d.findings.some(x => x.severity === "error"));
  }
  if (spec.id.endsWith("/doctor")) exact(d.toolchain.status, spec.lane === "skill" ? "pass" : "not_evaluated");
  if (spec.id.endsWith("/compat")) {
    exact(d.clients.map(x => x.client_id).sort(), ["claude", "codex"]);
    exact(d.compatibility.status, "pass");
  }
  if (spec.id.endsWith("/inspect")) {
    exact(d.inspection.name, spec.lane);
    exact(d.inspection.schema, "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json");
    const types = d.inspection.components.map(x => x.type).sort();
    const skills = spec.lane === "skill" || spec.lane.startsWith("hybrid-") ? 2 : 1;
    exact(types, [...Array(skills).fill("skill"), ...(spec.lane === "skill" ? [] : [spec.lane.endsWith("remote") ? "mcp_streamable-http" : "mcp_stdio"])].sort());
  }
}
function normalized(row) {
  const r = jsonDocument(row.stdout);
  delete r.data.product; delete r.data.product_version;
  if (r.data.help) r.data.help.use = r.data.help.use.replace(/^agentplugins author|^plugin-kit-ai/, "<author>");
  return { status: row.status, stderr: row.stderr, result: r };
}
function treeShape(entries) {
  assert.ok(Array.isArray(entries) && entries.length > 0 && entries.length <= 4096);
  const names = new Set(); let bytes = 0;
  for (const v of entries) {
    c.keys(v, v.kind === "directory" ? ["path", "mode", "kind"] : ["path", "mode", "kind", "sha256", "size"], "tree entry");
    assert.ok(typeof v.path === "string" && v.path.length <= 512 && !names.has(v.path));
    assert.ok(v.path === "." || /^(?!\.\.?($|\/))[A-Za-z0-9._-]+(?:\/[A-Za-z0-9._-]+)*$/.test(v.path));
    assert.ok(v.path === "." || v.path.split("/").every(x => x !== "." && x !== "..")); names.add(v.path);
    assert.ok(Number.isInteger(v.mode) && v.mode >= 0 && v.mode <= 511);
    assert.ok(v.kind === "file" || v.kind === "directory");
    if (v.kind === "file") { sha(v.sha256); assert.ok(Number.isSafeInteger(v.size) && v.size >= 0); bytes += v.size; }
  }
  assert.ok(entries[0]?.path === "." && entries[0].kind === "directory" && bytes <= 16 * LIMIT);
  for (const v of entries) if (v.path !== ".") {
    const parent = path.posix.dirname(v.path);
    assert.ok(entries.some(x => x.path === parent && x.kind === "directory"), "tree parent closure");
  }
}
// Base64 preserves exact bounded bytes, including empty/non-UTF8 generated
// files. Entries remain no-link inventories; no evidence path is opened here.
function capturedBytes(entries, documents) {
  c.keys(documents, entries.filter(x => x.kind === "file").map(x => x.path), "referenced byte closure");
  const result = Object.create(null); let total = 0;
  for (const item of entries.filter(x => x.kind === "file")) {
    const value = documents[item.path]; assert.equal(typeof value, "string");
    assert.ok(value.length <= 24 * LIMIT);
    const bytes = Buffer.from(value, "base64"); exact(bytes.toString("base64"), value);
    exact(c.metadata(bytes), { size: item.size, sha256: item.sha256 }, "referenced bytes");
    total += bytes.length; assert.ok(total <= 16 * LIMIT); result[item.path] = bytes;
  }
  return result;
}
// Exact packagedigest/snapshot.go v1 framing for this fixed no-link journey.
// Hash content, not per-file digest text, and exclude the synthetic root entry.
function packageIdentity(project) {
  treeShape(project.files); const bytes = capturedBytes(project.files, project.documents);
  const h = crypto.createHash("sha256");
  const length = n => { const b = Buffer.alloc(8); b.writeBigUInt64BE(BigInt(n)); h.update(b); };
  const frame = text => { const b = Buffer.from(text); length(b.length); h.update(b); };
  frame("agentplugins.package-tree\0sha256\0v1");
  for (const item of project.files.filter(x => x.path !== ".").sort((a, b) => a.path < b.path ? -1 : 1)) {
    const body = item.kind === "file" ? bytes[item.path] : Buffer.alloc(0);
    for (const text of ["entry", item.path, item.kind, item.kind === "directory" ? "040000" : item.mode & 0o111 ? "100755" : "100644", ""]) frame(text);
    length(body.length); h.update(body);
  }
  return { tree_digest: `sha256:${h.digest("hex")}`, manifest_digest: `sha256:${c.digest(bytes["plugin.json"])}` };
}
function projectEvidence(root, lane) {
  const files = tree(root);
  const manifest = JSON.parse(c.readFile(path.join(root, "plugin.json"), LIMIT));
  const mcp = lane === "skill" ? null : JSON.parse(c.readFile(path.join(root, "mcp.json"), LIMIT));
  const lock = lane.endsWith("stdio") ? JSON.parse(c.readFile(path.join(root, "package-lock.json"), LIMIT)) : null;
  const pkg = lock ? JSON.parse(c.readFile(path.join(root, "package.json"), LIMIT)) : null;
  const documents = Object.fromEntries(files.filter(x => x.kind === "file").map(x =>
    [x.path, (x.size ? c.readFile(path.join(root, x.path), 16 * LIMIT) : Buffer.alloc(0)).toString("base64")]));
  exact(tree(root), files, "project bytes and modes stable during capture");
  return { files, manifest, mcp, lock, package: pkg, documents };
}
function checkProject(value, lane) {
  c.keys(value, ["files", "manifest", "mcp", "lock", "package", "documents"], "project evidence");
  treeShape(value.files); assert.ok(value.files.length > 3);
  const documents = { "plugin.json": value.manifest, ...(value.mcp ? { "mcp.json": value.mcp } : {}),
    ...(value.lock ? { "package.json": value.package, "package-lock.json": value.lock } : {}) };
  const captured = capturedBytes(value.files, value.documents);
  for (const [name, parsed] of Object.entries(documents)) {
    exact(jsonDocument(captured[name].toString("utf8")), parsed);
    const pin = value.files.find(x => x.path === name); assert.ok(pin);
    exact([pin.sha256, pin.size], [c.digest(captured[name]), captured[name].length]);
  }
  const names = value.files.map(x => x.path);
  exact(new Set(names).size, names.length);
  for (const name of names) assert.ok(!/(^|\/)(plugin\.yaml|hooks|\.codex-plugin|\.mcp\.json|\.app\.json|node_modules)(\/|$)/.test(name));
  for (const name of ["plugin.json", "README.md", ".gitignore", "skills/extra-skill/SKILL.md"]) assert.ok(names.includes(name), name);
  exact(value.manifest.$schema, "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json");
  exact(value.manifest.name, lane); exact(value.manifest.version, "0.1.0");
  assert.equal(value.manifest.author, undefined); assert.equal(value.manifest.license, undefined);
  if (lane === "skill") exact(value.mcp, null);
  else {
    exact(value.mcp.$schema, "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json");
    const servers = Object.values(value.mcp.mcpServers); exact(servers.length, 1);
    exact(servers[0].type, lane.endsWith("remote") ? "streamable-http" : "stdio");
  }
  if (lane.endsWith("stdio")) {
    const version = value.package.dependencies["@modelcontextprotocol/sdk"];
    exact(version, "1.30.0");
    exact(value.lock.packages["node_modules/@modelcontextprotocol/sdk"].version, version);
    exact(value.lock.packages["node_modules/@modelcontextprotocol/sdk"].integrity,
      "sha512-xKd8OIzlqNzcqcNumGAa6g+PW2kjD5vrpcKOnfldAUPP3j7lnqMPwlTXQm8gF+UwH72z0lqaRbjr9hqGz0eITA==");
  } else { exact(value.lock, null); exact(value.package, null); }
}
function installationCommands() {
  return [
    ...["skill", "mcp-remote", "hybrid-stdio"].map(lane => ({ id: `dry-run/${lane}`, args: ["add", `<source>/${lane}`, "--target=codex", "--dry-run", "--format=json"] })),
    { id: "add", args: ["add", "<source>/skill", "--target=codex", "--format=json"] },
    { id: "info", args: ["info", "skill", "--target=codex", "--format=json"] },
    { id: "update", args: ["update", "skill", "--target=codex", "--format=json"] },
    { id: "remove", args: ["remove", "skill", "--target=codex", "--format=json"] },
    { id: "list", args: ["list", "--format=json"] }
  ];
}
function scanEvidence(r, source, project) {
  const data = r.data.security ? r.data : r.data.targets?.[0]?.output;
  const security = data?.security;
  assert.ok(security, "production scan evidence");
  c.keys(security, ["schema_version", "subject", "scanner", "policy", "outcome", "counts", "scanned_files", "report_digest", "evidence_source",
    ...(Object.hasOwn(security, "findings") ? ["findings"] : [])], "security assessment");
  exact(security.schema_version, 1, "fixed production security schema");
  exact(security.scanner, { id: "lintai", version: "0.1.3" }); exact(security.policy, POLICY, "fixed production security policy");
  exact(security.evidence_source, source); exact(security.outcome, "no_blocking_findings");
  // This journey already requires no warnings or blocking findings. In this
  // outcome the production counts and published finding projection are empty.
  exact(security.counts, { blocking: 0, warnings: 0, total: 0 }, "security counts match outcome");
  exact(security.findings ?? [], [], "security findings match counts"); positive(security.scanned_files);
  const subject = packageIdentity(project);
  exact(security.subject, subject, "independently captured package security subject");
  exact({ tree_digest: data.tree_digest, manifest_digest: data.manifest_digest }, subject);
  assert.match(security.report_digest, /^sha256:[0-9a-f]{64}$/);
  return security;
}
function acquisition(state, bodies) {
  const root = { path: ".", mode: 448, kind: "directory" };
  assert.ok(Array.isArray(state.acquisition)); treeShape([root, ...state.acquisition]);
  for (const item of state.acquisition) {
    const directory = /^(?:security(?:\/(?:lintai|assessments))?|security\/lintai\/0\.1\.3(?:\/linux-amd64(?:-musl)?)?)$/.test(item.path);
    const file = /^(?:security\/lintai\/0\.1\.3\/linux-amd64(?:-musl)?\/lintai|security\/assessments\/[0-9a-f]{64}\.json|(?:directory|discovery)-v1-cache\.json)$/.test(item.path);
    assert.ok(directory || file, "fixed acquisition paths"); exact(item.kind, directory ? "directory" : "file");
  }
  return capturedBytes(state.acquisition, Object.fromEntries(state.acquisition.filter(x => x.kind === "file")
    .map(x => [x.path, bodies[x.sha256]])));
}
function acquisitionClosure(rows, bodies) {
  const files = rows.flatMap(row => [row.before, row.after].flatMap(state => state.acquisition.filter(x => x.kind === "file")));
  const pins = [...new Map(files.map(x => [x.sha256, x])).values()];
  c.keys(bodies, pins.map(x => x.sha256), "closed acquisition byte subjects");
  let total = 0;
  for (const item of pins) { total += item.size; assert.ok(total <= 16 * LIMIT, "total acquisition byte bound"); }
  for (const row of rows) for (const state of [row.before, row.after]) acquisition(state, bodies);
}
// Fixed ReleaseScanner release.go pins, never supplied by an evidence caller.
// HTTP bodies are observer evidence, not files invented in the scanner cache.
const SCANNER_RELEASES = Object.freeze({
  "linux-amd64": { name: "lintai-v0.1.3-x86_64-unknown-linux-gnu.tar.gz", sha256: "2b3d176db752433b904a4b42375543ff398f4841d22e48f7d4f23ded925b72da" },
  "linux-amd64-musl": { name: "lintai-v0.1.3-x86_64-unknown-linux-musl.tar.gz", sha256: "3da60f749c61e2caca029a44a9ce422d570aef8c57f82ce51c411c8cec12f61b" }
});
function scannerArchive(scan, executable) {
  const asset = SCANNER_RELEASES[scan.executable.path.split("/")[3]];
  assert.ok(asset, "fixed scanner platform");
  c.keys(scan.archive, ["url", "bytes"], "observed release archive");
  exact(scan.archive.url, `https://github.com/777genius/lintai/releases/download/v0.1.3/${asset.name}`, "fixed scanner release URL");
  assert.equal(typeof scan.archive.bytes, "string");
  assert.ok(scan.archive.bytes.length <= 44 * LIMIT, "scanner archive byte bound");
  const body = Buffer.from(scan.archive.bytes, "base64");
  exact(body.toString("base64"), scan.archive.bytes);
  assert.ok(body.length > 0 && body.length <= 32 * LIMIT, "production scanner archive bound");
  exact(c.digest(body), asset.sha256, "independently pinned scanner release archive");
  // Bounded in-memory tar inspection. No extraction to disk or execution. This
  // N1 profile accepts ordinary tar files/directories; unknown extensions fail
  // closed pending inspection of the genuine pinned archive.
  const tar = zlib.gunzipSync(body, { maxOutputLength: 64 * LIMIT });
  let offset = 0, binary, count = 0;
  const octal = b => { const v = b.toString("ascii").replace(/\0.*$/, "").trim(); assert.match(v, /^[0-7]+$/); return parseInt(v, 8); };
  const names = new Set();
  while (offset + 512 <= tar.length) {
    const header = tar.subarray(offset, offset + 512);
    if (header.every(x => x === 0)) break;
    assert.ok(++count <= 4096, "scanner archive entry bound");
    const sum = header.reduce((n, x, i) => n + (i >= 148 && i < 156 ? 32 : x), 0);
    exact(octal(header.subarray(148, 156)), sum, "scanner tar checksum");
    const text = (a, b) => header.subarray(a, b).toString("utf8").replace(/\0.*$/, "");
    const prefix = text(345, 500), name = (prefix ? prefix + "/" : "") + text(0, 100);
    assert.ok(name && !name.startsWith("/") && !/[\\\x00-\x1f]/.test(name) && !name.split("/").includes(".."), "safe scanner archive name");
    assert.ok(!names.has(name), "unique scanner archive entry"); names.add(name);
    const type = header[156], size = octal(header.subarray(124, 136));
    assert.ok([0, 48, 53].includes(type), "ordinary scanner archive entries only");
    assert.ok(size <= 32 * LIMIT && (type !== 53 || size === 0));
    const end = offset + 512 + size; assert.ok(end <= tar.length, "complete scanner archive entry");
    if (path.posix.basename(name) === "lintai" && type !== 53) {
      assert.equal(binary, undefined, "one release scanner executable"); binary = tar.subarray(offset + 512, end);
    }
    offset = offset + 512 + Math.ceil(size / 512) * 512;
  }
  assert.ok(tar.length - offset >= 1024 && tar.subarray(offset).every(x => x === 0), "complete scanner tar terminator");
  assert.ok(binary?.length, "release archive contains lintai");
  exact(binary, executable, "scanner executable is the pinned archive member");
}
function scanReplay(rows, projects, scans, bodies) {
  assert.ok(Array.isArray(scans)); exact(scans.length, 3, "three observed fresh scans and report bytes");
  const fresh = new Map(); let executable;
  for (let i = 0; i < 4; i++) {
    const row = rows[i], lane = i < 3 ? row.id.slice(8) : "skill";
    const assessment = scanEvidence(jsonDocument(row.stdout), i < 3 ? "local_scan" : "cache", projects[lane]);
    const key = c.digest(Buffer.from([assessment.subject.tree_digest, assessment.subject.manifest_digest,
      "lintai", "0.1.3", POLICY.id, String(POLICY.version), POLICY.digest].join("\0")));
    const file = `security/assessments/${key}.json`, before = acquisition(row.before, bodies), after = acquisition(row.after, bodies);
    const cached = { ...assessment }; delete cached.evidence_source;
    assert.ok(after[file], "required assessment cache bytes"); exact(jsonDocument(after[file].toString("utf8")), cached);
    if (i < 3) {
      assert.equal(before[file], undefined, "fresh scan must precede cache creation");
      const scan = scans[i]; c.keys(scan, ["id", "args", "subject", "executable", "archive", "report"], "observed scanner call");
      exact([scan.id, scan.args, scan.subject], [row.id, ["scan-agent-plugin", `<source>/${lane}`], assessment.subject]);
      c.keys(scan.executable, ["path", "sha256"], "observed scanner executable");
      assert.match(scan.executable.path, /^security\/lintai\/0\.1\.3\/linux-amd64(?:-musl)?\/lintai$/);
      assert.ok(after[scan.executable.path], "required scanner acquisition bytes");
      exact(c.digest(after[scan.executable.path]), sha(scan.executable.sha256));
      scannerArchive(scan, after[scan.executable.path]);
      const item = row.after.acquisition.find(x => x.path === scan.executable.path); exact(item.mode, 0o700);
      if (executable) exact(scan.executable, executable, "same acquired scanner"); else executable = scan.executable;
      assert.equal(typeof scan.report, "string");
      const report = jsonDocument(scan.report);
      exact(`sha256:${c.digest(Buffer.from(scan.report))}`, assessment.report_digest, "exact scanner report bytes");
      exact(report.schema_version, 1); exact(report.tool, { name: "lintai", version: "0.1.3" });
      exact([report.policy.id, report.policy.version], [POLICY.id, POLICY.version], "scanner report policy");
      assert.ok(Array.isArray(report.policy.presets) && report.policy.presets.every(x => typeof x === "string"));
      exact(report.stats.scanned_files, assessment.scanned_files, "scanner report file counts");
      assert.ok(Number.isSafeInteger(report.stats.skipped_files) && report.stats.skipped_files >= 0);
      exact(report.findings, [], "scanner report findings"); exact(report.runtime_errors ?? [], []);
      assert.ok(Array.isArray(report.diagnostics ?? []));
      fresh.set(lane, { assessment: cached, bytes: after[file] });
    } else {
      const preceding = fresh.get(lane); assert.ok(preceding, "preceding genuine scan required");
      exact(cached, preceding.assessment, "cache must reuse exact preceding scan");
      exact(before[file], preceding.bytes); exact(after[file], preceding.bytes);
      exact(c.digest(before[executable.path]), executable.sha256);
      exact(before[executable.path], after[executable.path]);
    }
  }
}
function stateDocument(state) {
  const item = state.state.find(x => x.path === "state-v2.json");
  if (!item) { exact(state.state_document, null); return { installations: [] }; }
  assert.equal(typeof state.state_document, "string");
  exact(c.metadata(Buffer.from(state.state_document)), { sha256: item.sha256, size: item.size });
  const document = jsonDocument(state.state_document); exact(document.schema_version, 4);
  assert.ok(Array.isArray(document.installations)); return document;
}
// Only capture knows the actual operation root. Check exact equality before
// replacing it with this fixed evidence token; replay never accepts a target
// expectation supplied by a caller or a different root inside the evidence.
const INSTALLER_ROOT = "<installer-state>";
function normalizeInstaller(rows, root) {
  const normalizeClient = (client, id) => {
    const physical = `skill-${c.digest(Buffer.from(id)).slice(0, 12)}`;
    exact(client.physical_artifact_id, physical);
    exact(client.target_locator, `${root}/managed/clients/codex/${physical}`, "registration targets operation-owned installer root");
    const binding = locator => "client_" + c.digest(Buffer.from([id, "codex", "user", locator].join("\0"))).slice(0, 24);
    exact(client.client_binding_id, binding(client.target_locator));
    client.target_locator = `${INSTALLER_ROOT}/managed/clients/codex/${physical}`;
    client.client_binding_id = binding(client.target_locator);
  };
  for (const row of rows) {
    for (const state of [row.before, row.after]) {
      exact(state.root, root); state.root = INSTALLER_ROOT;
      if (!state.state_document) continue;
      const document = stateDocument(state);
      for (const registration of document.installations) {
        const clients = Object.values(registration.clients); exact(clients.length, 1);
        exact(Object.keys(registration.clients), [clients[0].client_binding_id]);
        normalizeClient(clients[0], registration.installation_id);
        registration.clients = { [clients[0].client_binding_id]: clients[0] };
      }
      state.state_document = c.encode(document).toString("utf8");
      Object.assign(state.state.find(x => x.path === "state-v2.json"), c.metadata(Buffer.from(state.state_document)));
    }
  }
}
function publicInfoClient(value, client) {
  // read.go: publicInstallationView/publicPackageRevision expose a projection,
  // never the private binding, locator, physical child or catalog evidence.
  // This assertion is shared by per-command capture and terminal replay.
  const expected = {};
  for (const key of ["client_id", "scope", "materialization", "activation", "authentication", "policy", "verification"]) {
    assert.equal(typeof client[key], "string", `registered public info ${key}`);
    expected[key] = client[key];
  }
  const revision = client.package_revision, exposed = {};
  for (const key of ["version", "distribution_id", "release_sequence"])
    if (revision[key]) exposed[key] = revision[key];
  // Go strings.TrimSpace uses Unicode White_Space (unlike JS trim's BOM rule).
  const resolved = (revision.resolved_revision || "").replace(/^\p{White_Space}+|\p{White_Space}+$/gu, "");
  if (/^[0-9a-f]{40}$/.test(resolved)) exposed.resolved_revision = resolved;
  for (const key of ["tree_digest", "manifest_digest"]) exposed[key] = revision[key];
  expected.package_revision = exposed;
  if (client.affected_surfaces?.length) expected.affected_surfaces = client.affected_surfaces.slice().sort();
  // read_reconciliation.go returns indeterminate with false reconciliation for
  // this isolated Codex config, with no installed/versioned native client.
  // Optional observations cannot invent successful native discovery evidence.
  if (["receipt_reconciled", "native_discovery_reconciled", "native_identity_state"].some(key => Object.hasOwn(value, key)))
    Object.assign(expected, { receipt_reconciled: false, native_discovery_reconciled: false, native_identity_state: "indeterminate" });
  exact(value, expected, "public info client matches checked registration and isolated lifecycle observations");
}
function installedIdentity(state, project) {
  const document = stateDocument(state); exact(document.installations.length, 1);
  const registration = document.installations[0], subject = packageIdentity(project);
  exact(registration.declared_name, "skill"); exact(registration.package.declared_name, "skill");
  exact(registration.package.version, "0.1.0"); exact(registration.source.tree_digest, subject.tree_digest);
  exact(registration.package.manifest_digest, subject.manifest_digest);
  assert.match(registration.installation_id, /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/);
  const clients = Object.values(registration.clients); exact(clients.length, 1);
  const client = clients[0]; exact([client.client_id, client.scope, client.materialization], ["codex", "user", "materialized"]);
  const physical = `skill-${c.digest(Buffer.from(registration.installation_id)).slice(0, 12)}`;
  exact(client.physical_artifact_id, physical);
  const projection = `managed/clients/codex/${physical}`;
  exact(client.target_locator, `${state.root}/${projection}`, "registration targets operation-owned installer root");
  const binding = "client_" + c.digest(Buffer.from([registration.installation_id, "codex", "user", client.target_locator].join("\0"))).slice(0, 24);
  exact(Object.keys(registration.clients), [binding]); exact(client.client_binding_id, binding);
  exact([client.package_revision.version, client.package_revision.tree_digest, client.package_revision.manifest_digest],
    ["0.1.0", subject.tree_digest, subject.manifest_digest]);
  assert.ok(state.state.some(x => x.path === projection && x.kind === "directory"), "owned client projection exists");
  // Skills are copied without changes into the Codex compatibility projection.
  for (const skill of project.files.filter(x => /^skills\/[^/]+\/SKILL.md$/.test(x.path))) {
    const copied = state.state.find(x => x.path === `${projection}/${skill.path}`);
    assert.ok(copied && copied.kind === "file"); exact([copied.sha256, copied.size], [skill.sha256, skill.size]);
  }
  const documents = capturedBytes(state.state.filter(x => /\/(?:\.codex-plugin\/plugin|\.agents\/plugins\/marketplace)\.json$/.test(x.path)), state.projection_documents);
  const read = leaf => {
    const bytes = documents[`${projection}/${leaf}`]; assert.ok(bytes, `mandatory Codex projection ${leaf}`);
    return jsonDocument(bytes.toString("utf8"));
  };
  const manifest = { name: "skill", version: "0.1.0", skills: "./skills/" };
  for (const key of ["description", "homepage", "repository", "license", "author", "keywords"])
    if (project.manifest[key] !== undefined) manifest[key] = project.manifest[key];
  exact(read(".codex-plugin/plugin.json"), manifest, "Codex plugin identity and component references");
  exact(read(".agents/plugins/marketplace.json"), {
    name: "agentplugins-" + c.digest(Buffer.from(physical)).slice(0, 12),
    plugins: [{ name: "skill", source: { source: "local", path: "./" },
      policy: { installation: "AVAILABLE", authentication: "ON_INSTALL" }, category: "Productivity" }]
  }, "Codex managed marketplace identity and reference");
  return { registration, client, projection };
}
function installed(row, spec, projects) {
  const r = envelope(row, { ...spec, status: 0 }), verb = spec.args[0]; exact(r.command, verb);
  if (spec.id.startsWith("dry-run/")) {
    exact(r.data.dry_run, true);
    scanEvidence(r, "local_scan", projects[spec.id.slice(8)]);
    const lane = spec.id.slice(8), result = lifecycleResult(r, "add", "codex"), plan = result.plan;
    exact([plan.client_id, plan.scope, plan.status], ["codex", "user", "manual_activation_required"]);
    const skills = projects[lane].files.filter(x => /^skills\/[^/]+\/SKILL.md$/.test(x.path)).map(x => `skill:${x.path.split("/")[1]}`);
    const servers = Object.keys(projects[lane].mcp?.mcpServers || {}).map(x => `mcp_server:${x}`);
    exact(plan.components.map(x => `${x.kind}:${x.name}`).sort(), [...skills, ...servers].sort());
    assert.ok(plan.components.every(x => x.support === "projected"), "Codex compatibility projection support");
    exact(row.before.client, row.after.client); exact(row.before.state, row.after.state);
    exact(row.before.state_document_source, row.after.state_document_source, "dry-run raw state bytes preserved");
    return;
  }
  if (["add", "update", "remove"].includes(verb)) {
    const result = lifecycleResult(r, verb, "codex");
    exact(result.mutated, verb !== "update");
    const identity = installedIdentity(verb === "add" ? row.after : row.before, projects.skill);
    exact(result.installation_id, identity.registration.installation_id, "lifecycle installed identity");
    if (verb === "update") { exact(row.before.state_document_source, row.after.state_document_source, "update raw state bytes preserved"); exact(result.no_change, true); exact(row.before.state, row.after.state); exact(row.before.client, row.after.client); }
    else assert.notDeepEqual(row.before.state, row.after.state, "real lifecycle state mutation");
    if (verb === "add") {
      exact(result.activation.authentication, "not_checked");
      assert.notEqual(result.activation.authentication_attested, true);
      exact(row.before.client, row.after.client, "preexisting client configuration preservation");
      exact(stateDocument(row.before).installations, [], "new installation starts unregistered");
      assert.ok(!row.before.state.some(x => x.path === identity.projection || x.path.startsWith(identity.projection + "/")), "new owned projection");
      assert.ok(row.after.state.some(x => x.path === "state-v2.json"), "production lifecycle state");
      scanEvidence(r, "cache", projects.skill); // The preceding fresh dry-run scanned these identical bytes.
    }
    if (verb === "remove") {
      assert.ok(!row.after.state.some(x => x.path === identity.projection || x.path.startsWith(identity.projection + "/")), "remove owned projection");
      exact(stateDocument(row.after).installations, [], "remove registration");
      assert.ok(!row.after.state.some(x => x.path.startsWith("managed/clients/codex/") && x.kind === "file"), "no remaining Codex projection files");
      exact(row.before.client, row.after.client, "remove preserves client configuration");
    }
  } else if (verb === "info") {
    exact(row.before, row.after, "info is read only");
    const identity = installedIdentity(row.before, projects.skill);
    exact(r.data.installation_id, identity.registration.installation_id, "info installed identity");
    exact(r.data.name, "skill"); exact(r.data.version, "0.1.0");
    exact(r.data.clients.length, 1);
    publicInfoClient(r.data.clients[0], identity.client);
  } else {
    exact(row.before, row.after, "list is read only");
    exact(r.data.installations, []); exact(stateDocument(row.after).installations, []);
  }
}
// No supported, reviewed whole-descendant observer has been provisioned for
// this contract. Never retry the denied ptrace flow, or treat a caller boolean,
// control trap, local file, or an empty syscall list as host observation.
function observationGate() {
  fail("WHOLE_OS_OBSERVATION_UNAVAILABLE: supported authoring process/network and owned-descendant observation remains a separate execution prerequisite; no passing terminal");
}
function commandContinuity(rows, projects) {
  const sort = xs => xs.slice().sort((a, b) => a.path < b.path ? -1 : a.path > b.path ? 1 : 0);
  let expected = [{ path: ".", mode: 448, kind: "directory" }];
  const broken = [
    { path: "skill/skills/broken", mode: 448, kind: "directory" },
    { path: "skill/skills/broken/SKILL.md", mode: 384, kind: "file", ...c.metadata(Buffer.from(BROKEN)) }
  ];
  for (const row of rows) {
    if (row.id === "malformed-skill") expected = sort([...expected, ...broken]);
    exact(row.before, expected, `command continuity before ${row.id}`);
    if (/\/(init|extra-skill)$/.test(row.id)) {
      const [lane, operation] = row.id.split("/");
      let files = projects[lane].files;
      if (operation === "init") files = files.filter(x => !/^skills\/extra-skill(?:\/|$)/.test(x.path) &&
        !(x.path === "skills" && lane.startsWith("mcp-")));
      const added = files.map(x => ({ ...x, path: x.path === "." ? lane : `${lane}/${x.path}` }));
      const keep = expected.filter(x => x.path !== lane && !x.path.startsWith(lane + "/"));
      if (operation === "init") assert.ok(!expected.some(x => x.path === lane));
      else for (const old of expected.filter(x => x.path === lane || x.path.startsWith(lane + "/")))
        exact(added.find(x => x.path === old.path), old, "extra Skill preserves all existing bytes and modes");
      expected = sort([...keep, ...added]);
      assert.notDeepEqual(row.before, expected, "declared authoring mutation has effects");
    }
    exact(row.after, expected, `command effects and final project tree ${row.id}`);
    if (row.id !== "product-help") {
      const data = jsonDocument(row.stdout).data;
      if (data.identity) {
        const lane = row.id === "malformed-skill" ? "skill" : row.id.split("/")[0];
        const files = row.after.filter(x => x.path === lane || x.path.startsWith(lane + "/"))
          .map(x => ({ ...x, path: x.path === lane ? "." : x.path.slice(lane.length + 1) }));
        const documents = Object.fromEntries(files.filter(x => x.kind === "file").map(x => [x.path,
          x.path === "skills/broken/SKILL.md" ? Buffer.from(BROKEN).toString("base64") : projects[lane].documents[x.path]]));
        exact(data.identity.tree_digest, packageIdentity({ files, documents }).tree_digest, "author result describes observed command tree");
      }
    }
    if (row.id === "malformed-skill") expected = expected.filter(x => !broken.some(b => b.path === x.path));
  }
  exact(expected, sort([{ path: ".", mode: 448, kind: "directory" }, ...CASES.flatMap(lane =>
    projects[lane].files.map(x => ({ ...x, path: x.path === "." ? lane : `${lane}/${x.path}` })))]), "final command tree closure");
}
function custodyCoverage(entries, preparation) {
  const subjects = [...preparation.subjects, { file: "preparation-run.json", ...c.metadata(c.encode(preparation)) }];
  assert.ok(Array.isArray(entries)); exact(entries.length, 19, "eighteen inputs plus preparation custody");
  exact(entries.map(x => x.file), subjects.map(x => x.file));
  const inodes = new Set();
  entries.forEach((v, i) => {
    c.keys(v, ["file", "mode", "dev", "ino", "links", "size", "mtime", "ctime"], "input custody fields");
    exact(v.size, subjects[i].size); exact(v.links, 1);
    for (const key of ["mode", "dev", "ino"]) assert.ok(Number.isSafeInteger(v[key]) && v[key] >= 0);
    assert.ok(v.mode <= 0o177777); exact(v.mode & 0o170000, 0o100000); exact(v.mode & 0o7000, 0);
    for (const key of ["mtime", "ctime"]) assert.ok(Number.isFinite(v[key]) && v[key] > 0);
    const identity = `${v.dev}:${v.ino}`; assert.ok(!inodes.has(identity)); inodes.add(identity);
  });
}
function verifyJourney(e, pins) {
  c.keys(e, FILES, "evidence bundle");
  const transcript = e["transcripts.json"], projects = e["trees.json"];
  c.keys(transcript, [...c.PRODUCTS, "installer"], "transcripts"); c.keys(projects, c.PRODUCTS, "trees");
  for (const p of c.PRODUCTS) {
    const specs = commands(p), rows = transcript[p]; exact(rows.length, specs.length, "complete command inventory");
    for (let i = 0; i < specs.length; i++) {
      const row = rows[i], spec = specs[i];
      c.keys(row, ["id", "args", "status", "stdout", "stderr", "before", "after"], "command transcript");
      treeShape(row.before); treeShape(row.after);
      exact([row.id, row.args], [spec.id, spec.args]); authorResult(row, spec, p, pins.identity);
      if (!/\/(init|extra-skill)$/.test(spec.id)) exact(row.before, row.after, "read/rejection preservation");
    }
    c.keys(projects[p], CASES, "five templates");
    for (const lane of CASES) checkProject(projects[p][lane], lane);
    commandContinuity(rows, projects[p]);
  }
  for (let i = 0; i < commands("agentplugins").length; i++) {
    const spec = commands("agentplugins")[i];
    if (spec.author) exact(normalized(transcript.agentplugins[i]), normalized(transcript["plugin-kit-ai"][i]), `parity ${spec.id}`);
  }
  exact(projects.agentplugins, projects["plugin-kit-ai"], "pair tree/mode parity");
  const specs = installationCommands(); exact(transcript.installer.length, specs.length);
  for (let i = 0; i < specs.length; i++) {
    const row = transcript.installer[i];
    c.keys(row, ["id", "args", "status", "stdout", "stderr", "before", "after"], "installer transcript");
    for (const state of [row.before, row.after]) {
      c.keys(state, ["root", "client", "state", "acquisition", "state_document", "state_document_source", "projection_documents"], "separate installer state and acquisition");
      exact(state.root, INSTALLER_ROOT, "fixed operation-owned installer root token");
      if (state.state_document === null) exact(state.state_document_source, null);
      else {
        c.keys(state.state_document_source, ["size", "sha256"], "raw state document byte pin");
        sha(state.state_document_source.sha256); positive(state.state_document_source.size);
        assert.ok(state.state_document_source.size <= LIMIT);
      }
      treeShape(state.client); treeShape(state.state);
      capturedBytes(state.state.filter(x => /\/(?:\.codex-plugin\/plugin|\.agents\/plugins\/marketplace)\.json$/.test(x.path)), state.projection_documents);
      acquisition(state, e["acquisition.json"]); stateDocument(state);
    }
    if (i) exact(row.before, transcript.installer[i - 1].after, "installer state continuity");
    else { exact(row.before.acquisition, []); exact(stateDocument(row.before).installations, []); }
    // Every original client file/directory survives every command, including modes.
    for (const original of transcript.installer[0].before.client)
      exact(row.after.client.find(x => x.path === original.path), original, "preexisting client preservation");
    exact([row.id, row.args], [specs[i].id, specs[i].args]); installed(row, specs[i], projects.agentplugins);
  }
  acquisitionClosure(transcript.installer, e["acquisition.json"]);
  scanReplay(transcript.installer, projects.agentplugins, e["scans.json"], e["acquisition.json"]);
  const preserve = e["preservation.json"];
  c.keys(preserve, ["inputs_before", "inputs_after", "projects_before", "projects_after", "author_homes_before", "author_homes_after", "custody_before", "custody_after"], "preservation");
  for (const name of ["inputs", "projects", "author_homes", "custody"]) exact(preserve[`${name}_before`], preserve[`${name}_after`]);
  custodyCoverage(preserve.custody_before, e["preparation.json"]);
  custodyCoverage(preserve.custody_after, e["preparation.json"]);
  exact(preserve.projects_before, projects);
  exact(preserve.inputs_before, e["preparation.json"].subjects, "preserved eighteen independently pinned subjects");
  c.keys(preserve.author_homes_before, c.PRODUCTS, "isolated author homes");
  for (const p of c.PRODUCTS) {
    c.keys(preserve.author_homes_before[p], ["home", "state", "config", "cache", "data", "tmp"], "author isolation");
    for (const entries of Object.values(preserve.author_homes_before[p])) exact(entries, [{ path: ".", mode: 448, kind: "directory" }]);
  }
  return { inventory: "paired-linux-journey/1", commands: Object.fromEntries(c.PRODUCTS.map(p => [p, commands(p).map(x => x.id)])),
    installer: specs.map(x => x.id), parity: "both-products", preservation: "inputs-projects-client-state", runtime: "not_evaluated" };
}
function terminal(p, pins, manifest, options, evidence, tools) {
  return { schema: SCHEMA, lane: `${p}/${TARGET}`, identity: pins.identity, candidate_sha256: pins.candidate_sha256,
    pair_marker_sha256: pins.pair_marker_sha256, projection_pins: pins.products, subject: subject(manifest, p),
    peer_subject: subject(manifest, c.PRODUCTS.find(x => x !== p)), preparation: options.preparation,
    producer: options.producer, host: evidence["host.json"], tools, assertions: verifyJourney(evidence, pins),
    evidence: FILES.map(file => ({ file, ...c.metadata(c.encode(evidence[file])) })) };
}
function readTerminals(root, inputRoot, pins, expected) {
  c.safeDirectory(root); c.keys(expected, ["producer", "preparation", "tools"], "terminal expectations");
  invocation(expected.producer, WORKFLOW, pins.identity.commit);
  const prepared = readPreparation(inputRoot, pins, expected.preparation);
  const { manifest } = verifyProjectedPair(inputRoot, pins);
  const names = [...FILES, ...c.PRODUCTS.map(p => `${p}-terminal.json`)];
  exact(fs.readdirSync(root).sort(), names.sort(), "exact paired artifact closure");
  const evidence = Object.fromEntries(FILES.map(file => [file, canonical(c.readFile(path.join(root, file), 32 * LIMIT), 32 * LIMIT)]));
  exact(evidence["preparation.json"], prepared);
  const build = evidence["build-info.json"]; c.keys(build, c.PRODUCTS, "selected build info");
  for (const p of c.PRODUCTS) buildInfo(build[p], p, TARGET, pins.identity, MODE);
  const host = evidence["host.json"];
  exact(host, { platform: "linux", architecture: "x64", machine: "x86_64", target: TARGET,
    observation: "whole-descendant-authoring-no-process-network/1" });
  c.keys(expected.tools, ["go", "node"], "host tools");
  for (const [name, version] of [["go", "go1.25.13"], ["node", "v22.21.1"]]) {
    c.keys(expected.tools[name], ["sha256", "version"], "tool pin"); sha(expected.tools[name].sha256); exact(expected.tools[name].version, version);
  }
  const result = [];
  for (const p of c.PRODUCTS) {
    const value = canonical(c.readFile(path.join(root, `${p}-terminal.json`), LIMIT));
    c.keys(value, TOP, "closed native terminal");
    exact(value, terminal(p, pins, manifest, expected, evidence, expected.tools)); result.push(value);
  }
  // This only verifies local observation encoding. Provider completed attempt,
  // signer revision, authenticated artifact custody and all 13 P lanes are N2+.
  return result;
}
async function produce(options, signal) {
  c.keys(options, ["root", "pins", "preparation", "producer", "go", "go_sha256", "output", "work"], "native options");
  const { root, pins } = options;
  invocation(options.producer, WORKFLOW, pins.identity.commit);
  exact([process.platform, process.arch, os.machine()], ["linux", "x64", "x86_64"], "actual Linux amd64 host");
  exact(process.version, "v22.21.1"); sha(options.go_sha256);
  assert.ok(path.isAbsolute(options.go)); exact(c.digest(c.readFile(options.go)), options.go_sha256);
  // Host Go has its own pin; a different-platform builder executable need not hash identically.
  const frozen = verifyProjectedPair(root, pins), prep = readPreparation(root, pins, options.preparation);
  c.outputPlacement(options.output, [root, path.dirname(options.go), path.resolve(__dirname, "../../..")]);
  c.outputPlacement(options.work, [root, path.dirname(options.go), path.resolve(__dirname, "../../..")]);
  assert.ok(!options.output.startsWith(options.work + "/") && !options.work.startsWith(options.output + "/") && options.work !== options.output);
  fs.mkdirSync(options.output, { mode: 0o700 }); fs.mkdirSync(options.work, { mode: 0o700 });
  const e = {}, rows = {}, projects = {}, binaries = {}, build = {}, contexts = {};
  const acquisitionBodies = Object.create(null); let acquisitionTotal = 0;
  const frozenBefore = frozen.subjects.map(s => ({ file: path.relative(root, s.file), ...c.metadata(c.readFile(s.file)) }));
  const custodyBefore = inputCustody(root, frozen.subjects);
  const ownedTerminals = [];
  try {
    const tool = context(path.join(options.work, "host-tool"));
    const go = await subprocess(options.go, ["env", "-json", "GOVERSION", "GOHOSTOS", "GOHOSTARCH"], tool, signal);
    exact(go.status, 0); exact(go.stderr, "");
    exact(JSON.parse(go.stdout), { GOVERSION: "go1.25.13", GOHOSTOS: "linux", GOHOSTARCH: "amd64" });
    for (const p of c.PRODUCTS) {
      contexts[p] = context(path.join(options.work, p));
      const a = frozen.manifest.products[p].assets[TARGET], bytes = c.readFile(path.join(root, p, a.file));
      const binary = p === "plugin-kit-ai" ? c.unpack(bytes, a.binary.file) : bytes;
      binaries[p] = path.join(contexts[p].root, p); fs.writeFileSync(binaries[p], binary, { flag: "wx", mode: 0o500 });
      exact(c.metadata(c.readFile(binaries[p])), { sha256: a.binary.sha256, size: a.binary.size });
      const r = await subprocess(options.go, ["version", "-m", "-json", binaries[p]], tool, signal);
      exact(r.status, 0); exact(r.stderr, ""); build[p] = JSON.parse(r.stdout); buildInfo(build[p], p, TARGET, pins.identity, MODE);
    }
    const protectedHomes = () => Object.fromEntries(c.PRODUCTS.map(p => [p, Object.fromEntries(["home", "state", "config", "cache", "data", "tmp"].map(n => [n, tree(path.join(contexts[p].root, n))]))]));
    const homesBefore = protectedHomes();
    for (const p of c.PRODUCTS) {
      rows[p] = []; projects[p] = {}; const ctx = contexts[p], projectRoot = path.join(ctx.root, "projects");
      for (const spec of commands(p)) {
        const malformed = path.join(projectRoot, "skill/skills/broken");
        if (spec.id === "malformed-skill") {
          fs.mkdirSync(malformed, { mode: 0o700 }); fs.writeFileSync(path.join(malformed, "SKILL.md"), BROKEN, { flag: "wx", mode: 0o600 });
        }
        const before = tree(projectRoot), r = await subprocess(binaries[p], spec.args, ctx, signal);
        const row = { id: spec.id, args: spec.args, ...r, before, after: tree(projectRoot) };
        rows[p].push(row); authorResult(row, spec, p, pins.identity);
        if (!/\/(init|extra-skill)$/.test(spec.id)) exact(row.before, row.after);
        if (spec.id === "malformed-skill") { fs.unlinkSync(path.join(malformed, "SKILL.md")); fs.rmdirSync(malformed); }
      }
      for (const lane of CASES) projects[p][lane] = projectEvidence(path.join(projectRoot, lane), lane);
    }
    const homesAfter = protectedHomes(); exact(homesBefore, homesAfter, "authoring homes/cache/client/state preservation");
    const install = context(path.join(options.work, "installer"));
    const client = path.join(install.env.HOME, ".codex"); fs.mkdirSync(client, { mode: 0o700 });
    fs.writeFileSync(path.join(client, "preservation-marker"), "owned client content\n", { flag: "wx", mode: 0o600 });
    const markerBefore = c.metadata(c.readFile(path.join(client, "preservation-marker")));
    fs.writeFileSync(path.join(client, "config.toml"), "# owned native journey client root\n", { flag: "wx", mode: 0o600 });
    // Real supported detector must recognize this isolated config root. No fake
    // executable, injected detector, scanner seed, auth or activation flag.
    rows.installer = [];
    const state = () => {
      const all = tree(install.env.AGENTPLUGINS_HOME);
      // Genuine feed/scanner acquisition is retained separately. These are the
      // production cache names, not wildcard state exclusions or seeded inputs.
      const isAcquisition = x => /^(security(?:\/|$)|directory-v1-cache\.json$|discovery-v1-cache\.json$)/.test(x.path);
      const acquisition = all.filter(isAcquisition);
      const stateFile = path.join(install.env.AGENTPLUGINS_HOME, "state-v2.json");
      for (const item of acquisition.filter(x => x.kind === "file")) {
        const bytes = c.readFile(path.join(install.env.AGENTPLUGINS_HOME, item.path), 16 * LIMIT);
        exact(c.metadata(bytes), { sha256: item.sha256, size: item.size }, "acquisition snapshot bytes");
        if (!Object.hasOwn(acquisitionBodies, item.sha256)) { acquisitionTotal += bytes.length; assert.ok(acquisitionTotal <= 16 * LIMIT, "total acquisition byte bound"); }
        acquisitionBodies[item.sha256] = bytes.toString("base64");
      }
      const projection_documents = Object.fromEntries(all.filter(x => /\/(?:\.codex-plugin\/plugin|\.agents\/plugins\/marketplace)\.json$/.test(x.path))
        .map(x => [x.path, c.readFile(path.join(install.env.AGENTPLUGINS_HOME, x.path), LIMIT).toString("base64")]));
      // The root comes from the fixed context used for this very subprocess,
      // never registration text or a readTerminals expected-target parameter.
      // Retain the actual byte pin as well as the normalized document pin so
      // path normalization cannot conceal info/update/list byte mutation.
      const rawState = all.find(x => x.path === "state-v2.json");
      return { state_document_source: rawState ? { size: rawState.size, sha256: rawState.sha256 } : null,
        root: install.env.AGENTPLUGINS_HOME, client: tree(client), state: all.filter(x => !isAcquisition(x)), acquisition, projection_documents,
        state_document: all.some(x => x.path === "state-v2.json") ? c.readFile(stateFile, LIMIT).toString("utf8") : null };
    };
    for (const spec of installationCommands()) {
      const args = spec.args.map(a => a.replace("<source>", path.join(contexts.agentplugins.root, "projects")));
      const before = state(), r = await subprocess(binaries.agentplugins, args, install, signal, true);
      const row = { id: spec.id, args: spec.args, ...r, before, after: state() };
      rows.installer.push(row); installed(row, spec, projects.agentplugins);
    }
    exact(c.metadata(c.readFile(path.join(client, "preservation-marker"))), markerBefore, "client sentinel preserved through installer");
    const afterProjects = Object.fromEntries(c.PRODUCTS.map(p => [p, Object.fromEntries(CASES.map(lane =>
      [lane, projectEvidence(path.join(contexts[p].root, "projects", lane), lane)]))]));
    const after = verifyProjectedPair(root, pins); readPreparation(root, pins, options.preparation);
    exact(c.digest(c.readFile(options.go)), options.go_sha256);
    for (const p of c.PRODUCTS) {
      exact(c.digest(c.readFile(binaries[p])), frozen.manifest.products[p].assets[TARGET].binary.sha256);
      exact(fs.lstatSync(binaries[p]).mode & 0o777, 0o500, "selected executable mode preserved");
    }
    normalizeInstaller(rows.installer, install.env.AGENTPLUGINS_HOME);
    e["transcripts.json"] = rows; e["trees.json"] = projects; e["build-info.json"] = build; e["preparation.json"] = prep;
    e["preservation.json"] = { inputs_before: frozenBefore,
      inputs_after: after.subjects.map(s => ({ file: path.relative(root, s.file), ...c.metadata(c.readFile(s.file)) })),
      projects_before: projects, projects_after: afterProjects, author_homes_before: homesBefore, author_homes_after: homesAfter,
      custody_before: custodyBefore, custody_after: inputCustody(root, after.subjects) };
    e["acquisition.json"] = acquisitionBodies;
    e["scans.json"] = observationGate(); // Unavailable capability is a failure diagnostic, never a pass stub.
    // ReleaseScanner keeps neither its raw report stdout nor acquisition HTTP
    // bytes. No scan record can be inferred from a cached assessment. A future
    // reviewed observer must close that custody; production has no positive seam.
    e["host.json"] = { platform: process.platform, architecture: process.arch, machine: os.machine(), target: TARGET,
      observation: "whole-descendant-authoring-no-process-network/1" };
    verifyJourney(e, pins);
    const tools = { go: { sha256: options.go_sha256, version: "go1.25.13" }, node: { sha256: c.digest(c.readFile(process.execPath)), version: process.version } };
    for (const file of FILES) fs.writeFileSync(path.join(options.output, file), c.encode(e[file]), { flag: "wx", mode: 0o400 });
    if (signal?.aborted) fail("cancelled before terminal");
    for (const p of c.PRODUCTS) {
      const file = path.join(options.output, `${p}-terminal.json`);
      const fd = fs.openSync(file, "wx", 0o400); ownedTerminals.push(file);
      try { fs.writeFileSync(fd, c.encode(terminal(p, pins, frozen.manifest, options, e, tools))); }
      finally { fs.closeSync(fd); }
    }
    return readTerminals(options.output, root, pins, { producer: options.producer, preparation: options.preparation, tools });
  } catch (error) {
    for (const file of ownedTerminals) fs.unlinkSync(file);
    fs.writeFileSync(path.join(options.output, "failure-transcripts.json"), c.encode({
      schema: "authoring-frozen-native-failure-transcripts/v1",
      invocations: Object.fromEntries(Object.entries(rows).map(([p, r]) => [p, r.slice(-10).map(x => ({
        id: x.id, args: x.args, status: x.status, stdout: x.stdout.slice(0, 65536), stderr: x.stderr.slice(0, 65536) }))]))
    }), { flag: "wx", mode: 0o600 });
    fs.writeFileSync(path.join(options.output, "diagnostic.json"), c.encode({ schema: "authoring-frozen-native-failure/v1",
      error: String(error.message).slice(0, 2000), commands: Object.fromEntries(Object.entries(rows).map(([p, r]) => [p, r.map(x => ({ id: x.id, status: x.status }))])),
      native_acceptance: false }), { flag: "wx", mode: 0o600 });
    throw error;
  }
}
if (require.main === module) {
  const controller = new AbortController();
  for (const signal of ["SIGINT", "SIGTERM"]) process.once(signal, () => controller.abort());
  Promise.resolve().then(() => {
    assert.equal(process.argv.length, 3, "one exact configuration file required");
    return produce(canonical(c.readFile(process.argv[2], LIMIT)), controller.signal);
  }).catch(error => { process.stderr.write(String(error.message).slice(0, 2000) + "\n"); process.exitCode = 1; });
}
module.exports = { writePreparation, readPreparation, produce, readTerminals };
