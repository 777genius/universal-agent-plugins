"use strict";

// In-memory unit evidence only. No artifact custody, authenticated assertions,
// packing, signatures, native launch, cache, network or qualification is tested.
const test = require("node:test");
const assert = require("node:assert/strict");
const crypto = require("node:crypto");
const c = require("../scripts/dual-authoring-candidate");
const inputs = require("../scripts/authoring-native-inputs");
const stage = require("../scripts/stage-authoring-npm");
const runtime = require("../lib/public-authoring");
const products = ["agentplugins", "plugin-kit-ai"];
const targets = ["darwin-amd64", "darwin-arm64", "linux-amd64", "linux-arm64", "windows-amd64", "windows-arm64"];
const prefix = "npm/agentplugins/";
const sha = n => n.toString(16).padStart(64, "0");
const json = v => Buffer.from(JSON.stringify(v, null, 2) + "\n");
const clone = v => JSON.parse(JSON.stringify(v));
const inventory = p => ["LICENSE", "README.md", "package.json", `bin/${p}.js`, "bin/package.json",
  "lib/package.json", "lib/platform.js", "lib/verifier.js", "lib/public-authoring.js",
  p === "agentplugins" ? "lib/bootstrap.js" : "lib/install.js", "scripts/package.json",
  "scripts/dual-authoring-candidate.js", "public-release.json", "release-manifest.json", "native-inputs.json"].sort();
const assertions = ["authenticated_native_inputs", "exact_preparation_binding", "exact_source_blobs",
  "exact_generated_closures", "exact_pack_entries_modes_bytes", "both_products_complete", "shared_runtime_bytes_equal",
  "pack_once", "inputs_unchanged", "no_native_execution", "no_publication"];

function blob(bytes, mode = "100644") {
  return { bytes, git_blob: crypto.createHash("sha1").update(`blob ${bytes.length}\0`).update(bytes).digest("hex"),
    mode, sha256: c.digest(bytes) };
}
function fixture(version = "0.1.99") {
  const input = { schema: "authoring-native-inputs/v1", identity: {
    repository: "777genius/universal-agent-plugins", commit: "a".repeat(40), engine_revision: "a".repeat(40),
    versions: { agentplugins: version, "plugin-kit-ai": "2.0.0" } },
  authoring_mode: "release-cli-contract-v1", asset_scope: "six-platform-pair",
  candidate_sha256: sha(30), pair_marker_sha256: sha(31), products: {},
  preparation: { sha256: sha(32), artifact: { run_id: 101, run_attempt: 2, artifact_id: 301, artifact_sha256: sha(33) } },
  producer: { workflow: ".github/workflows/agentplugins-release.yml", source: "a".repeat(40), run_id: 201, run_attempt: 3 } };
  const manifests = {};
  products.forEach((product, pi) => {
    const v = input.identity.versions[product];
    const p = input.products[product] = { tag: pi ? "v2.0.0" : `agentplugins-v${v}`,
      manifest_sha256: sha(40 + pi), checksums_sha256: sha(50 + pi), assets: {} };
    targets.forEach((target, ti) => {
      const extension = target.startsWith("windows-") ? ".exe" : "";
      const binary = { file: product + extension, sha256: sha(1 + pi * 6 + ti), size: 100 + ti };
      p.assets[target] = { file: `${product}_${v}_${target.replace("-", "_")}${pi ? ".tar.gz" : extension}`,
        sha256: pi ? sha(60 + ti) : binary.sha256, size: pi ? 200 + ti : binary.size, binary };
    });
    manifests[product] = json({ schema_version: 3, status: "CANDIDATE", product,
      repository: input.identity.repository, tag: p.tag, version: v, commit: input.identity.commit,
      engine_revision: input.identity.engine_revision, versions: clone(input.identity.versions),
      candidate_sha256: input.candidate_sha256, authoring_mode: input.authoring_mode, asset_scope: input.asset_scope,
      assets: clone(p.assets), release_eligible: false, platform_acceptance: false, attested: false });
    p.manifest_sha256 = c.digest(manifests[product]);
    p.checksums_sha256 = c.digest(Buffer.from([...Object.values(p.assets).map(a => `${a.sha256}  ${a.file}`),
      `${p.manifest_sha256}  release-manifest.json`].join("\n") + "\n"));
  });
  const source = Object.fromEntries(stage.STAGE_ALLOWLIST.map(n => [n, blob(Buffer.from(`unit source: ${n}\n`),
    n.includes("/bin/") ? "100755" : "100644")]));
  for (const p of products) {
    source[`npm/${p}/package.json`] = blob(json({ name: inputs.PACKAGES[p], version: "0.0.0-development",
      description: `unit metadata ${p}`, license: "Apache-2.0", homepage: "https://example.invalid/unit",
      repository: { type: "git", url: "https://example.invalid/unit.git" }, keywords: ["preserve", p],
      engines: { node: p === "agentplugins" ? ">=22" : ">=18" }, publishConfig: { access: "public" },
      files: ["old-file"], bin: { [p]: `bin/${p}.js` },
      scripts: p === "agentplugins" ? { test: "node --test" } : { postinstall: "node ./lib/install.js" } }));
  }
  return { input, source, manifests, inputBytes: inputs.encodeInputs(input) };
}
function stageFixture() {
  const f = fixture();
  const pair = stage.pairedPackageFiles(f.source, f.manifests, f.inputBytes);
  const value = { schema: "dual-authoring-public-stage/v1", identity: clone(f.input.identity),
    authoring_mode: f.input.authoring_mode, asset_scope: f.input.asset_scope, candidate_sha256: f.input.candidate_sha256,
    pair_marker_sha256: f.input.pair_marker_sha256,
    projection_pins: Object.fromEntries(products.map(p => [p, {
      manifest_sha256: f.input.products[p].manifest_sha256, checksums_sha256: f.input.products[p].checksums_sha256 }])),
    native_inputs: { sha256: c.digest(f.inputBytes), artifact: { run_id: 201, run_attempt: 3,
      artifact_id: 401, artifact_sha256: sha(71) } },
    wrapper_blobs: Object.fromEntries(Object.entries(f.source).map(([n, { bytes, ...pin }]) => [n, pin])),
    generated: Object.fromEntries(products.map(p => [p, Object.fromEntries(inventory(p).map(n => [n, c.digest(pair[p][n])]))])),
    packs: Object.fromEntries(products.map((p, i) => [p, { file: `${inputs.PACKAGES[p]}-${f.input.identity.versions[p]}.tgz`,
      sha256: sha(80 + i), size: 300 + i, integrity: "sha512-" + Buffer.alloc(64, i + 1).toString("base64"),
      shasum: (90 + i).toString(16).padStart(40, "0") }])),
    tools: Object.fromEntries(["node", "npm", "git", "tar", "gh"].map((n, i) => [n, { version: `unit-${i}.0`, sha256: sha(100 + i) }])),
    producer: { workflow: ".github/workflows/agentplugins-npm-publish.yml", source: f.input.identity.commit,
      ref: `refs/tags/agentplugins-v${f.input.identity.versions.agentplugins}`, run_id: 501, run_attempt: 1 },
    assertions: Object.fromEntries(assertions.map(n => [n, true])) };
  return { ...f, pair, value };
}
function objectPaths(v, prefix = []) {
  return [prefix, ...Object.entries(v).flatMap(([k, child]) =>
    child && typeof child === "object" ? objectPaths(child, [...prefix, k]) : [])];
}
const at = (v, keys) => keys.reduce((obj, key) => obj[key], v);
function reverse(v) {
  return v && typeof v === "object" ? Object.fromEntries(Object.entries(v).reverse().map(([k, x]) => [k, reverse(x)])) : v;
}
function rejectStage(f, mutate) {
  const v = clone(f.value); mutate(v);
  assert.throws(() => stage.encodeStage(v, f.inputBytes));
  assert.throws(() => stage.decodeStage(json(v), f.inputBytes));
}

test("C1 pure pair: exact two-product closure, metadata and identical I/runtime bytes", () => {
  const f = fixture(), pair = stage.pairedPackageFiles(f.source, f.manifests, f.inputBytes);
  assert.deepEqual(Object.keys(pair), products);
  for (const p of products) {
    assert.deepEqual(Object.keys(pair[p]).sort(), inventory(p));
    const pkg = JSON.parse(pair[p]["package.json"]), base = JSON.parse(f.source[`npm/${p}/package.json`].bytes);
    assert.deepEqual(pkg, { ...base, version: f.input.identity.versions[p], private: false, files: inventory(p) });
    const d = inputs.decodeDescriptor(pair[p]["public-release.json"], f.inputBytes, p);
    assert.equal(d.input_binding.sha256, c.digest(f.inputBytes));
    assert.deepEqual(pair[p]["release-manifest.json"], f.manifests[p]);
    assert.deepEqual(pair[p]["native-inputs.json"], f.inputBytes);
    for (const n of ["LICENSE", "README.md", `bin/${p}.js`, "lib/platform.js",
      p === "agentplugins" ? "lib/bootstrap.js" : "lib/install.js"]) {
      assert.deepEqual(pair[p][n], f.source[`npm/${p}/${n}`].bytes);
    }
    for (const dir of ["bin", "lib", "scripts"]) assert.deepEqual(pair[p][`${dir}/package.json`], json({ type: "commonjs" }));
    for (const claim of ["qualification", "signed_subject", "self_sha256", "stage_sha256", "execution_sha256"]) {
      assert.equal(Object.hasOwn(d, claim), false);
      assert.equal(Object.hasOwn(JSON.parse(pair[p]["native-inputs.json"]), claim), false);
    }
  }
  for (const n of [...stage.COMMON, "native-inputs.json"]) {
    assert.deepEqual(pair.agentplugins[n], pair["plugin-kit-ai"][n]);
    assert.notStrictEqual(pair.agentplugins[n], pair["plugin-kit-ai"][n]);
  }
});

test("C1 pure pair: caller bytes and objects unchanged and returned buffers independently owned", () => {
  const f = fixture(), before = json(f);
  const pair = stage.pairedPackageFiles(f.source, f.manifests, f.inputBytes);
  assert.deepEqual(json(f), before);
  for (const p of products) for (const bytes of Object.values(pair[p])) bytes.fill(0);
  assert.deepEqual(json(f), before);
  const fresh = stage.pairedPackageFiles(f.source, f.manifests, f.inputBytes);
  assert.deepEqual(fresh.agentplugins["native-inputs.json"], f.inputBytes);
});

test("C1 pure pair: validate both manifests, selected hashes, assets and checksum pins", () => {
  for (const p of products) {
    for (const mutate of [m => m.product = "wrong", m => m.commit = "b".repeat(40), m => m.version = "2.0.1",
      m => m.assets["linux-amd64"].binary.sha256 = sha(500), m => m.release_eligible = true,
      m => m.qualification = null, m => delete m.attested]) {
      const f = fixture(), m = JSON.parse(f.manifests[p]); mutate(m); f.manifests[p] = json(m);
      f.input.products[p].manifest_sha256 = c.digest(f.manifests[p]);
      assert.throws(() => stage.pairedPackageFiles(f.source, f.manifests, inputs.encodeInputs(f.input)));
    }
    for (const key of ["manifest_sha256", "checksums_sha256"]) {
      const f = fixture(); f.input.products[p][key] = sha(500);
      assert.throws(() => stage.pairedPackageFiles(f.source, f.manifests, inputs.encodeInputs(f.input)), /hash/);
    }
    for (const replacement of [null, "{}", new Uint8Array([123, 125]), Buffer.alloc(0), Buffer.alloc(1024 * 1024 + 1)]) {
      const f = fixture(); f.manifests[p] = replacement;
      assert.throws(() => stage.pairedPackageFiles(f.source, f.manifests, f.inputBytes), /Buffer/);
    }
    const f = fixture(); f.manifests[p] = Buffer.from(f.manifests[p].toString().trim());
    f.input.products[p].manifest_sha256 = c.digest(f.manifests[p]);
    assert.throws(() => stage.pairedPackageFiles(f.source, f.manifests, inputs.encodeInputs(f.input)), /projection/);
  }
  for (const field of ["agentplugins", "plugin-kit-ai", "extra"]) {
    const f = fixture();
    if (field === "extra") f.manifests.extra = Buffer.from("extra"); else delete f.manifests[field];
    assert.throws(() => stage.pairedPackageFiles(f.source, f.manifests, f.inputBytes));
  }
});

test("C1 pure pair: exact source inventory and byte/pin/metadata contracts", () => {
  for (const name of stage.STAGE_ALLOWLIST) {
    const f = fixture(); delete f.source[name];
    assert.throws(() => stage.pairedPackageFiles(f.source, f.manifests, f.inputBytes));
  }
  const path = "npm/plugin-kit-ai/package.json";
  for (const mutate of [s => s.extra = s[path], s => s[path].sha256 = sha(900),
    s => s[path].git_blob = "b".repeat(40), s => s[path].mode = "120000", s => s[path].bytes = "{}",
    s => s[path].bytes = new Uint8Array([1]), s => s[path].bytes = Buffer.alloc(0),
    s => s[path].accepted = true]) {
    const f = fixture(); mutate(f.source);
    assert.throws(() => stage.pairedPackageFiles(f.source, f.manifests, f.inputBytes));
  }
  for (const p of products) for (const mutate of [b => b.name = "other", b => b.engines.node = ">=16",
    b => b.scripts.postinstall = "run something", b => b.bin[p] = "wrong.js", b => b.bin.other = "bin/other.js"]) {
    const f = fixture(), n = `npm/${p}/package.json`, base = JSON.parse(f.source[n].bytes); mutate(base);
    f.source[n] = blob(json(base));
    assert.throws(() => stage.pairedPackageFiles(f.source, f.manifests, f.inputBytes));
  }
  for (const body of [Buffer.from("null\n"), Buffer.from("[]\n"), Buffer.from([0xff]), Buffer.from("{broken")]) {
    const f = fixture(); f.source[path] = blob(body);
    assert.throws(() => stage.pairedPackageFiles(f.source, f.manifests, f.inputBytes));
  }
});

test("C1 pure pair: valid 1 MiB I still fails descriptor overflow before touching source", () => {
  const base = fixture("1.0.0").inputBytes.length;
  const n = Math.floor((inputs.MAX_INPUT_BYTES - base) / 8);
  const f = fixture("1" + "0".repeat(n) + ".0.0");
  const remaining = inputs.MAX_INPUT_BYTES - f.inputBytes.length;
  f.input.preparation.artifact.run_id = Number("1" + "0".repeat(2 + remaining));
  f.inputBytes = inputs.encodeInputs(f.input);
  assert.equal(f.inputBytes.length, inputs.MAX_INPUT_BYTES);
  assert.deepEqual(inputs.decodeInputs(f.inputBytes), f.input);
  let reads = 0;
  const source = new Proxy({}, { ownKeys() { reads++; throw new Error("source touched"); } });
  assert.throws(() => stage.pairedPackageFiles(source, f.manifests, f.inputBytes), /bounded Buffer/);
  assert.equal(reads, 0);
});

test("C1 pure pair: both descriptor boundaries, including a pair where only agent overflows", () => {
  // Agent's npm package spelling makes its descriptor nine bytes larger.
  const f = fixture("1.0.0"), small = stage.pairedPackageFiles(f.source, f.manifests, f.inputBytes);
  const overhead = small.agentplugins["public-release.json"].length;
  const large = fixture("1" + "0".repeat(inputs.MAX_DESCRIPTOR_BYTES - overhead) + ".0.0");
  const d = JSON.parse(small.agentplugins["public-release.json"]);
  d.identity = large.input.identity; d.release_manifest_sha256 = large.input.products.agentplugins.manifest_sha256;
  d.input_binding.sha256 = c.digest(large.inputBytes);
  assert.equal(inputs.encodeDescriptor(d, large.inputBytes, "agentplugins").length, inputs.MAX_DESCRIPTOR_BYTES);
  const kit = JSON.parse(small["plugin-kit-ai"]["public-release.json"]);
  kit.identity = large.input.identity; kit.release_manifest_sha256 = large.input.products["plugin-kit-ai"].manifest_sha256;
  kit.input_binding.sha256 = c.digest(large.inputBytes);
  assert.equal(inputs.encodeDescriptor(kit, large.inputBytes, "plugin-kit-ai").length, inputs.MAX_DESCRIPTOR_BYTES - 9);
  const pair = stage.pairedPackageFiles(large.source, large.manifests, large.inputBytes);
  assert.equal(pair.agentplugins["public-release.json"].length, inputs.MAX_DESCRIPTOR_BYTES);
  assert.equal(pair["plugin-kit-ai"]["public-release.json"].length, inputs.MAX_DESCRIPTOR_BYTES - 9);
  const over = fixture(large.input.identity.versions.agentplugins.replace("1", "10"));
  kit.identity = over.input.identity; kit.release_manifest_sha256 = over.input.products["plugin-kit-ai"].manifest_sha256;
  kit.input_binding.sha256 = c.digest(over.inputBytes);
  assert.equal(inputs.encodeDescriptor(kit, over.inputBytes, "plugin-kit-ai").length, inputs.MAX_DESCRIPTOR_BYTES - 8);
  assert.throws(() => stage.pairedPackageFiles(over.source, over.manifests, over.inputBytes), /bounded Buffer/);
});

test("C1 pure S: canonical two-product roundtrip, explicit inventory and no mutation", () => {
  const f = stageFixture(), before = json(f.value), encoded = stage.encodeStage(f.value, f.inputBytes);
  assert.deepEqual(encoded, before);
  assert.deepEqual(stage.decodeStage(encoded, f.inputBytes), f.value);
  assert.deepEqual(stage.encodeStage(reverse(f.value), f.inputBytes), encoded);
  assert.throws(() => stage.decodeStage(json(reverse(f.value)), f.inputBytes), /noncanonical/);
  assert.deepEqual(Object.keys(f.value), ["schema", "identity", "authoring_mode", "asset_scope", "candidate_sha256",
    "pair_marker_sha256", "projection_pins", "native_inputs", "wrapper_blobs", "generated", "packs", "tools", "producer", "assertions"]);
  assert.deepEqual(Object.keys(f.value.assertions), assertions);
  const decoded = stage.decodeStage(encoded, f.inputBytes); decoded.identity.versions.agentplugins = "9.0.0";
  assert.deepEqual(json(f.value), before);
  assert.deepEqual(stage.decodeStage(encoded, f.inputBytes), f.value);
});

test("C1 pure S: every object rejects missing, unknown, hidden, inherited and accessor fields", () => {
  const f = stageFixture();
  for (const path of objectPaths(f.value)) {
    for (const key of Object.keys(at(f.value, path))) rejectStage(f, v => { delete at(v, path)[key]; });
    rejectStage(f, v => { at(v, path).unexpected = true; });
    for (const value of [null, [], "object", 1, false]) {
      if (path.length) rejectStage(f, v => { at(v, path.slice(0, -1))[path.at(-1)] = value; });
      else assert.throws(() => stage.encodeStage(value, f.inputBytes));
    }
    for (const add of [o => Object.defineProperty(o, "trusted", { value: true }),
      o => o[Symbol("trusted")] = true, o => Object.setPrototypeOf(o, { accepted: true })]) {
      const v = clone(f.value); add(at(v, path)); assert.throws(() => stage.encodeStage(v, f.inputBytes));
    }
    const v = clone(f.value), o = at(v, path), key = Object.keys(o)[0];
    let calls = 0;
    Object.defineProperty(o, key, { enumerable: true, get() { calls++; return true; } });
    assert.throws(() => stage.encodeStage(v, f.inputBytes)); assert.equal(calls, 0);
  }
});

test("C1 pure S: fixed I identity, projection, artifact attempt, workflow/source/ref bindings", () => {
  const f = stageFixture();
  for (const mutate of [v => v.schema = "dual-authoring-public-preparation/v1", v => v.authoring_mode = "vertical-slice-v1",
    v => v.asset_scope = "host-pair", v => v.identity.repository = "fork/repo", v => v.identity.commit = "b".repeat(40),
    v => v.identity.engine_revision = "b".repeat(40), v => v.identity.versions.agentplugins = "0.1.98",
    v => v.identity.versions["plugin-kit-ai"] = "2.0.1", v => v.candidate_sha256 = sha(999), v => v.pair_marker_sha256 = sha(999),
    v => v.native_inputs.sha256 = sha(999), v => v.native_inputs.artifact.run_id++, v => v.native_inputs.artifact.run_attempt++,
    v => v.producer.workflow = inputs.WORKFLOW, v => v.producer.source = "b".repeat(40),
    v => v.producer.ref = "refs/heads/main", v => v.producer.ref = "refs/tags/v2.0.0"]) rejectStage(f, mutate);
  for (const p of products) for (const k of ["manifest_sha256", "checksums_sha256"]) {
    rejectStage(f, v => { v.projection_pins[p][k] = sha(999); });
  }
  for (const mutate of [i => i.preparation.sha256 = sha(999), i => i.preparation.artifact.artifact_id++,
    i => i.products["plugin-kit-ai"].checksums_sha256 = sha(999), i => i.pair_marker_sha256 = sha(999),
    i => i.producer.run_attempt++]) {
    const input = clone(f.input); mutate(input); const bytes = inputs.encodeInputs(input);
    assert.throws(() => stage.encodeStage(f.value, bytes));
    assert.throws(() => stage.decodeStage(json(f.value), bytes));
  }
  for (const bad of [null, f.input, f.inputBytes.toString(), json(reverse(f.input)), Buffer.alloc(0)]) {
    assert.throws(() => stage.encodeStage(f.value, bad));
    assert.throws(() => stage.decodeStage(json(f.value), bad));
    assert.throws(() => stage.pairedPackageFiles(f.source, f.manifests, bad));
  }
});

test("C1 pure S: exact generated set and source/I/manifest/descriptor/shared scope digests", () => {
  const f = stageFixture();
  for (const p of products) {
    for (const n of inventory(p).filter(n => n !== "package.json")) rejectStage(f, v => { v.generated[p][n] = sha(999); });
    for (const n of ["../escape", "qualification.json", "authoring-promotion.json", "completion.json", "extra"]) {
      rejectStage(f, v => { v.generated[p][n] = sha(999); });
    }
    rejectStage(f, v => { v.generated[p]["public-release.json"] = f.value.generated[products.find(x => x !== p)]["public-release.json"]; });
  }
  rejectStage(f, v => { v.wrapper_blobs[prefix + "lib/verifier.js"].sha256 = sha(999); });
  rejectStage(f, v => { v.wrapper_blobs[prefix + "lib/verifier.js"].mode = "120000"; });
});

test("C1 pure S: digest syntax, positive safe IDs, attempts, tarball names and canonical SRI", () => {
  const f = stageFixture();
  const digestPaths = objectPaths(f.value).flatMap(p => Object.keys(at(f.value, p))
    .filter(k => k.endsWith("sha256") || k === "git_blob" || k === "shasum").map(k => [...p, k]));
  for (const path of digestPaths) {
    const n = at(f.value, path).length;
    for (const bad of [null, 1, "0".repeat(n), "A".repeat(n), "f".repeat(n - 1), "f".repeat(n) + "\n"]) {
      rejectStage(f, v => { at(v, path.slice(0, -1))[path.at(-1)] = bad; });
    }
  }
  for (const path of [["producer", "run_id"], ["producer", "run_attempt"], ["native_inputs", "artifact", "artifact_id"],
    ...products.map(p => ["packs", p, "size"])]) {
    for (const bad of [0, -1, 1.5, "1", true, null, Number.MAX_SAFE_INTEGER + 1]) {
      rejectStage(f, v => { at(v, path.slice(0, -1))[path.at(-1)] = bad; });
    }
  }
  rejectStage(f, v => v.producer.run_attempt = 1001);
  for (const p of products) {
    rejectStage(f, v => v.packs[p].size = inputs.MAX_NATIVE_BYTES + 1);
    for (const file of ["../package.tgz", `${p}-9.0.0.tgz`, "package.tar.gz", null]) rejectStage(f, v => v.packs[p].file = file);
    const good = f.value.packs[p].integrity;
    for (const integrity of [null, "sha256-" + good.slice(7), good + "\n", good.slice(0, -1), good + " sha512-other",
      "sha512-" + "A".repeat(85) + "B=="]) rejectStage(f, v => v.packs[p].integrity = integrity);
  }
  for (const path of [["producer", "run_id"], ["native_inputs", "artifact", "artifact_id"]]) {
    const v = clone(f.value); at(v, path.slice(0, -1))[path.at(-1)] = Number.MAX_SAFE_INTEGER;
    assert.deepEqual(stage.decodeStage(stage.encodeStage(v, f.inputBytes), f.inputBytes), v);
  }
  const v = clone(f.value); v.producer.run_attempt = 1000;
  assert.deepEqual(stage.decodeStage(stage.encodeStage(v, f.inputBytes), f.inputBytes), v);
});

test("C1 pure S: closed tools and assertions; syntax does not establish authenticated truth", () => {
  const f = stageFixture();
  for (const name of ["node", "npm", "git", "tar", "gh"]) {
    for (const version of [null, 123, "", " ", "version\n", "a\0b"]) rejectStage(f, v => v.tools[name].version = version);
    rejectStage(f, v => v.tools[name].path = "/usr/bin/tool");
  }
  for (const name of assertions) for (const bad of [false, null, 1, "true"]) rejectStage(f, v => v.assertions[name] = bad);
  for (const name of ["qualification", "attested", "release_eligible", "platform_acceptance", "execution", "self_sha256",
    "artifact", "workflow", "verifier", "authenticatedInputs"]) rejectStage(f, v => v[name] = true);
  // No bytes here establish these unobservable assertions or hash claims. A
  // different well-formed pack digest can pass this codec; only readStage with
  // retained packs/authenticated custody may validate it in subsequent C1.
  const v = clone(f.value); v.packs.agentplugins.sha256 = sha(999);
  assert.deepEqual(stage.decodeStage(stage.encodeStage(v, f.inputBytes), f.inputBytes), v);
  assert.equal(stage.readStage, undefined);
  assert.equal(stage.stagePrepublication, undefined);
});

test("C1 pure S: canonical UTF-8, duplicate keys, nesting, spelling and exact 1 MiB boundary", () => {
  const f = stageFixture(), body = stage.encodeStage(f.value, f.inputBytes), text = body.toString();
  for (const bad of [text, new Uint8Array(body), null, Buffer.alloc(0), Buffer.alloc(1024 * 1024 + 1),
    Buffer.concat([Buffer.from([0xef, 0xbb, 0xbf]), body]), Buffer.concat([body, Buffer.from([0xff])]),
    Buffer.from(text.trim()), Buffer.from(text + "\n"), Buffer.from(text + "{}"),
    Buffer.from(text.replace('"schema":', '"schema": null, "schema":')),
    Buffer.from(text.replace('"schema"', '"sch\\u0065ma"')),
    Buffer.from(text.replace('"run_id": 501', '"run_id": 5.01e2')),
    Buffer.from(text.replaceAll("\n", "\r\n")), Buffer.from('{"x":'.repeat(5) + '0' + '}'.repeat(5))]) {
    assert.throws(() => stage.decodeStage(bad, f.inputBytes));
  }
  const v = clone(f.value);
  v.tools.node.version += "x".repeat(1024 * 1024 - body.length);
  const exact = stage.encodeStage(v, f.inputBytes); assert.equal(exact.length, 1024 * 1024);
  assert.deepEqual(stage.decodeStage(exact, f.inputBytes), v);
  v.tools.node.version += "x";
  assert.throws(() => stage.encodeStage(v, f.inputBytes), /bounded Buffer/);
  assert.throws(() => stage.decodeStage(json(v), f.inputBytes), /bounded Buffer/);
});

test("C1 pure v1 regression: same descriptor, metadata, null/private and all other bytes", () => {
  const f = fixture(), pair = stage.pairedPackageFiles(f.source, f.manifests, f.inputBytes);
  for (const p of products) {
    const v1 = stage.packageFiles(p, f.source, f.manifests[p], { identity: f.input.identity, manifestDigest: f.input.candidate_sha256 });
    assert.deepEqual(Object.keys(v1).sort(), inventory(p).filter(n => n !== "native-inputs.json"));
    assert.deepEqual(v1["public-release.json"], json({ schema: "dual-authoring-public-npm/v1", product: p,
      npm_package: inputs.PACKAGES[p], identity: f.input.identity, authoring_mode: "release-cli-contract-v1",
      asset_scope: "six-platform-pair", candidate_sha256: f.input.candidate_sha256,
      release_manifest_sha256: c.digest(f.manifests[p]), qualification: null }));
    const base = JSON.parse(f.source[`npm/${p}/package.json`].bytes);
    assert.deepEqual(v1["package.json"], json({ ...base, version: f.input.identity.versions[p], private: true,
      files: inventory(p).filter(n => n !== "native-inputs.json") }));
    for (const n of Object.keys(v1).filter(n => !["package.json", "public-release.json"].includes(n))) assert.deepEqual(v1[n], pair[p][n]);
  }
});

test("C1 pure inventories: separate exact stage additions and unchanged legacy exports", () => {
  const own = p => ["LICENSE", "README.md", "package.json", `bin/${p}.js`, "lib/platform.js",
    p === "agentplugins" ? "lib/bootstrap.js" : "lib/install.js"].map(n => `npm/${p}/${n}`);
  assert.deepEqual(stage.COMMON, ["lib/verifier.js", "lib/public-authoring.js", "scripts/dual-authoring-candidate.js"]);
  const legacy = [...stage.COMMON.map(n => prefix + n), ...products.flatMap(own),
    ...["stage-authoring-npm.js", "stage-dual-authoring-npm.js", "stage-dual-authoring-candidate.js", "authoring-release.js"]
      .map(n => prefix + "scripts/" + n)];
  assert.deepEqual(stage.ALLOWLIST, legacy);
  assert.deepEqual(stage.STAGE_ALLOWLIST, [...legacy,
    ...["authoring-native-inputs.js", "authoring-promotion.js", "authoring-native-qualification.js", "platform-proof.js",
      "npm-public-contract.js"].map(n => prefix + "scripts/" + n), "scripts/read-authoring-evidence-zip.py",
    ".github/workflows/agentplugins-release.yml", ".github/workflows/agentplugins-npm-publish.yml"]);
  assert.ok(Object.isFrozen(stage.STAGE_ALLOWLIST));
  assert.deepEqual(Object.keys(stage), ["prepare", "packageFiles", "ALLOWLIST", "COMMON",
    "encodeStage", "decodeStage", "pairedPackageFiles", "STAGE_ALLOWLIST"]);
});

test("C1 pure runtime regression: existing loadRelease rejects v2 and v1/null using only in-memory reads", t => {
  const f = fixture(), pair = stage.pairedPackageFiles(f.source, f.manifests, f.inputBytes);
  let files, reads;
  t.mock.method(c, "safeDirectory", root => root);
  t.mock.method(c, "readFile", file => {
    const name = file.slice("/unit-package/".length); reads.push(name);
    assert.ok(Object.hasOwn(files, name), `unexpected read ${file}`); return files[name];
  });
  for (const p of products) {
    files = pair[p]; reads = [];
    assert.throws(() => runtime.loadRelease(p, "/unit-package", "linux-amd64"), /public release: unexpected or missing fields/);
    assert.deepEqual(reads, ["public-release.json"]);
    // Real metadata contract for v1 is closed; supply its required fixture keys.
    files = stage.packageFiles(p, f.source, f.manifests[p], { identity: f.input.identity, manifestDigest: f.input.candidate_sha256 });
    const pkg = JSON.parse(files["package.json"]); pkg.bugs = { url: "https://example.invalid/unit" };
    if (p === "agentplugins") { pkg.os = ["darwin", "linux", "win32"]; pkg.cpu = ["x64", "arm64"]; }
    files["package.json"] = json(pkg); reads = [];
    assert.throws(() => runtime.loadRelease(p, "/unit-package", "linux-amd64"), /not qualified: preparation package/);
    assert.deepEqual(reads, ["public-release.json", "package.json", "release-manifest.json"]);
  }
});
