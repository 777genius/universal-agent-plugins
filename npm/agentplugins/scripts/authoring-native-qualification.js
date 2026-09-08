#!/usr/bin/env node
"use strict";

// N1 observes one frozen pair. This is not provider admission or qualification
// signing. No promotion caller accepts these local records in this checkpoint.
const fs = require("node:fs");
const path = require("node:path");
const os = require("node:os");
const cp = require("node:child_process");
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
const FILES = Object.freeze(["transcripts.json", "trees.json", "build-info.json", "preservation.json", "preparation.json", "host.json"]);
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
    let stdout = [], stderr = [], bytes = 0, problem, closed = false;
    const stop = reason => {
      problem ||= reason;
      if (child.pid && !closed) {
        try { process.kill(-child.pid, "SIGKILL"); }
        catch (e) { if (e.code !== "ESRCH") problem = `owned-child cleanup denied: ${e.code}`; }
      }
    };
    const timer = setTimeout(() => stop("timeout"), installer ? 120000 : 15000);
    const cancel = () => stop("cancelled"); signal?.addEventListener("abort", cancel, { once: true });
    for (const [stream, chunks] of [[child.stdout, stdout], [child.stderr, stderr]]) stream.on("data", b => {
      bytes += b.length;
      if (bytes > LIMIT) stop("output flood"); else chunks.push(b);
    });
    child.on("error", e => { problem = `subprocess failed: ${e.code}`; });
    child.on("close", (status, sig) => {
      closed = true; clearTimeout(timer); signal?.removeEventListener("abort", cancel);
      if (child.pid) {
        try { process.kill(-child.pid, 0); problem ||= "owned descendants survived"; process.kill(-child.pid, "SIGKILL"); }
        catch (e) { if (e.code !== "ESRCH") problem ||= `cleanup uncertain: ${e.code}`; }
      }
      if (problem || sig) return reject(new Error(problem || `signal ${sig}`));
      try {
        const decoder = new TextDecoder("utf-8", { fatal: true });
        resolve({ status, stdout: decoder.decode(Buffer.concat(stdout)), stderr: decoder.decode(Buffer.concat(stderr)) });
      } catch { reject(new Error("invalid UTF-8 subprocess output")); }
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
  if (spec.id === "capabilities") { assert.ok(d.capabilities); return; }
  if (spec.id === "author-help") { assert.ok(d.help.use.startsWith(product === "agentplugins" ? "agentplugins author" : "plugin-kit-ai")); return; }
  const mutation = /\/(init|extra-skill)$/.test(spec.id);
  if (mutation) { exact(d.committed, true); exact(d.effects.committed, true); }
  else exact(d.committed, false);
  if (!spec.lane || /\/(existing)$/.test(spec.id) || spec.id === "installer-flag") return;
  assert.match(d.identity.tree_digest, /^sha256:[0-9a-f]{64}$/);
  exact(d.identity.read_profile, "packageview-local-linux-v1");
  assert.ok(Array.isArray(d.profiles) && d.profiles.length > 0, "embedded Skills profile");
  for (const profile of d.profiles) { assert.ok(profile.id && profile.revision); assert.match(profile.digest, /^sha256:[0-9a-f]{64}$/); }
  assert.ok(d.profiles.some(p => p.id === "agent-skills/2026-09-06" &&
    p.revision === "69ef37e9424c0a7ea9dd2293b559e43ec8176379" &&
    p.digest === "sha256:b9079c0c10b7930e8c6a20ff2bc10cda2a3343c55185120e3f1116a1a529b220"), "pinned Skills rules");
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
    assert.ok(!v.path.split("/").includes("..")); names.add(v.path);
    assert.ok(Number.isInteger(v.mode) && v.mode >= 0 && v.mode <= 511);
    assert.ok(v.kind === "file" || v.kind === "directory");
    if (v.kind === "file") { sha(v.sha256); assert.ok(Number.isSafeInteger(v.size) && v.size >= 0); bytes += v.size; }
  }
  assert.ok(names.has(".") && bytes <= 16 * LIMIT);
  for (const v of entries) if (v.path !== ".") {
    const parent = path.posix.dirname(v.path);
    assert.ok(entries.some(x => x.path === parent && x.kind === "directory"), "tree parent closure");
  }
}
function projectEvidence(root, lane) {
  const files = tree(root);
  const manifest = JSON.parse(c.readFile(path.join(root, "plugin.json"), LIMIT));
  const mcp = lane === "skill" ? null : JSON.parse(c.readFile(path.join(root, "mcp.json"), LIMIT));
  const lock = lane.endsWith("stdio") ? JSON.parse(c.readFile(path.join(root, "package-lock.json"), LIMIT)) : null;
  const pkg = lock ? JSON.parse(c.readFile(path.join(root, "package.json"), LIMIT)) : null;
  const documents = Object.fromEntries(["plugin.json", ...(mcp ? ["mcp.json"] : []), ...(lock ? ["package.json", "package-lock.json"] : [])]
    .map(n => [n, c.readFile(path.join(root, n), LIMIT).toString("utf8")]));
  return { files, manifest, mcp, lock, package: pkg, documents };
}
function checkProject(value, lane) {
  c.keys(value, ["files", "manifest", "mcp", "lock", "package", "documents"], "project evidence");
  treeShape(value.files); assert.ok(value.files.length > 3);
  const documents = { "plugin.json": value.manifest, ...(value.mcp ? { "mcp.json": value.mcp } : {}),
    ...(value.lock ? { "package.json": value.package, "package-lock.json": value.lock } : {}) };
  c.keys(value.documents, Object.keys(documents), "exact project documents");
  for (const [name, parsed] of Object.entries(documents)) {
    exact(jsonDocument(value.documents[name]), parsed);
    const pin = value.files.find(x => x.path === name); assert.ok(pin);
    exact([pin.sha256, pin.size], [c.digest(Buffer.from(value.documents[name])), Buffer.byteLength(value.documents[name])]);
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
function scanEvidence(r, source) {
  const data = r.data.security ? r.data : r.data.targets?.[0]?.output;
  const security = data?.security;
  assert.ok(security && security.scanned_files > 0, "production scan evidence");
  exact(security.scanner, { id: "lintai", version: "0.1.3" });
  exact(security.evidence_source, source); exact(security.outcome, "no_blocking_findings");
  exact(security.subject, { tree_digest: data.tree_digest, manifest_digest: data.manifest_digest });
  assert.match(security.report_digest, /^sha256:[0-9a-f]{64}$/);
}
function installed(row, spec, projects) {
  const r = envelope(row, { ...spec, status: 0 }), verb = spec.args[0]; exact(r.command, verb);
  if (spec.id.startsWith("dry-run/")) {
    exact(r.data.dry_run, true);
    scanEvidence(r, "local_scan");
    const lane = spec.id.slice(8), result = lifecycleResult(r, "add", "codex"), plan = result.plan;
    exact([plan.client_id, plan.scope, plan.status], ["codex", "user", "manual_activation_required"]);
    const skills = projects[lane].files.filter(x => /^skills\/[^/]+\/SKILL.md$/.test(x.path)).map(x => `skill:${x.path.split("/")[1]}`);
    const servers = Object.keys(projects[lane].mcp?.mcpServers || {}).map(x => `mcp_server:${x}`);
    exact(plan.components.map(x => `${x.kind}:${x.name}`).sort(), [...skills, ...servers].sort());
    assert.ok(plan.components.every(x => x.support && x.support !== "unsupported"));
    exact(row.before.client, row.after.client); exact(row.before.state, row.after.state);
    return;
  }
  if (["add", "update", "remove"].includes(verb)) {
    const result = lifecycleResult(r, verb, "codex");
    exact(result.mutated, verb !== "update");
    if (verb === "update") { exact(result.no_change, true); exact(row.before.state, row.after.state); exact(row.before.client, row.after.client); }
    else assert.notDeepEqual(row.before.state, row.after.state, "real lifecycle state mutation");
    if (verb === "add") {
      exact(result.activation.authentication, "not_checked");
      assert.notEqual(result.activation.authentication_attested, true);
      assert.notDeepEqual(row.before.client, row.after.client, "real client projection");
      assert.ok(row.after.state.some(x => x.path === "state-v2.json"), "production lifecycle state");
      scanEvidence(r, "cache"); // The preceding fresh dry-run scanned these identical bytes.
    }
  } else if (verb === "info") {
    exact(r.data.name, "skill"); exact(r.data.version, "0.1.0");
    exact(r.data.clients.length, 1); exact(r.data.clients[0].client_id, "codex");
    exact(r.data.clients[0].package_revision.version, "0.1.0");
  } else exact(r.data.installations, []);
}
// No supported, reviewed whole-descendant observer has been provisioned for
// this contract. Never retry the denied ptrace flow, or treat a caller boolean,
// control trap, local file, or an empty syscall list as host observation.
function observationGate() {
  fail("WHOLE_OS_OBSERVATION_UNAVAILABLE: supported authoring process/network and owned-descendant observation remains a separate execution prerequisite; no passing terminal");
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
      c.keys(state, ["client", "state", "acquisition"], "separate installer state and acquisition");
      treeShape(state.client); treeShape(state.state);
      assert.ok(Array.isArray(state.acquisition) && state.acquisition.length <= 4096);
    }
    exact([row.id, row.args], [specs[i].id, specs[i].args]); installed(row, specs[i], projects.agentplugins);
  }
  const preserve = e["preservation.json"];
  c.keys(preserve, ["inputs_before", "inputs_after", "projects_before", "projects_after", "author_homes_before", "author_homes_after", "custody_before", "custody_after"], "preservation");
  for (const name of ["inputs", "projects", "author_homes", "custody"]) exact(preserve[`${name}_before`], preserve[`${name}_after`]);
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
          fs.mkdirSync(malformed, { mode: 0o700 }); fs.writeFileSync(path.join(malformed, "SKILL.md"), "---\nname: [\n---\nBroken\n", { flag: "wx" });
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
      return { client: tree(client), state: all.filter(x => !isAcquisition(x)), acquisition: all.filter(isAcquisition) };
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
    e["transcripts.json"] = rows; e["trees.json"] = projects; e["build-info.json"] = build; e["preparation.json"] = prep;
    e["preservation.json"] = { inputs_before: frozenBefore,
      inputs_after: after.subjects.map(s => ({ file: path.relative(root, s.file), ...c.metadata(c.readFile(s.file)) })),
      projects_before: projects, projects_after: afterProjects, author_homes_before: homesBefore, author_homes_after: homesAfter,
      custody_before: custodyBefore, custody_after: inputCustody(root, after.subjects) };
    verifyJourney({ ...e, "host.json": {} }, pins);
    observationGate(); // Unavailable capability is a failure diagnostic, never a pass stub.
    e["host.json"] = { platform: process.platform, architecture: process.arch, machine: os.machine(), target: TARGET,
      observation: "whole-descendant-authoring-no-process-network/1" };
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
