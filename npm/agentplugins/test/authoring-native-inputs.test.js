"use strict";

// All fixture positives prove structural consistency only, NOT authenticated
// provenance, signing, acquisition, eligibility, acceptance or authorization.
const test = require("node:test");
const assert = require("node:assert/strict");
const c = require("../scripts/dual-authoring-candidate");
const contract = require("../scripts/authoring-native-inputs");
const { encodeInputs, decodeInputs, encodeDescriptor, decodeDescriptor } = contract;
const products = ["agentplugins", "plugin-kit-ai"];
const targets = ["darwin-amd64", "darwin-arm64", "linux-amd64", "linux-arm64", "windows-amd64", "windows-arm64"];
const sha = n => n.toString(16).padStart(64, "0");
const json = value => Buffer.from(JSON.stringify(value, null, 2) + "\n");
const copy = value => structuredClone(value);

function fixture(version = "0.1.99") {
  const value = { schema: "authoring-native-inputs/v1", identity: {
    repository: "777genius/universal-agent-plugins", commit: "a".repeat(40), engine_revision: "a".repeat(40),
    versions: { agentplugins: version, "plugin-kit-ai": "2.0.0" } },
  authoring_mode: "release-cli-contract-v1", asset_scope: "six-platform-pair",
  candidate_sha256: sha(30), pair_marker_sha256: sha(31), products: {},
  preparation: { sha256: sha(32), artifact: { run_id: 101, run_attempt: 2, artifact_id: 301, artifact_sha256: sha(33) } },
  producer: { workflow: ".github/workflows/agentplugins-release.yml", source: "a".repeat(40), run_id: 201, run_attempt: 3 } };
  products.forEach((product, pi) => {
    const v = value.identity.versions[product];
    const p = value.products[product] = { tag: pi ? "v2.0.0" : `agentplugins-v${v}`,
      manifest_sha256: sha(40 + pi), checksums_sha256: sha(50 + pi), assets: {} };
    targets.forEach((target, ti) => {
      const extension = target.startsWith("windows-") ? ".exe" : "";
      const binary = { file: product + extension, sha256: sha(1 + pi * 6 + ti), size: 100 + ti };
      p.assets[target] = { file: `${product}_${v}_${target.replace("-", "_")}${pi ? ".tar.gz" : extension}`,
        sha256: pi ? sha(60 + ti) : binary.sha256, size: pi ? 200 + ti : binary.size, binary };
    });
  });
  return value;
}

function descriptor(input, inputBytes, product) {
  return { schema: "dual-authoring-public-npm/v2", product,
    npm_package: product === "agentplugins" ? "universal-agent-plugins" : "plugin-kit-ai",
    identity: copy(input.identity), authoring_mode: "release-cli-contract-v1", asset_scope: "six-platform-pair",
    candidate_sha256: input.candidate_sha256, release_manifest_sha256: input.products[product].manifest_sha256,
    input_binding: { file: "native-inputs.json", sha256: c.digest(inputBytes) } };
}
function rejectInput(mutate) {
  const value = fixture();
  mutate(value);
  assert.throws(() => encodeInputs(value));
  assert.throws(() => decodeInputs(json(value)));
}
function objectPaths(value, prefix = []) {
  return [prefix, ...Object.entries(value).flatMap(([key, v]) =>
    v && typeof v === "object" ? objectPaths(v, [...prefix, key]) : [])];
}
const at = (value, keys) => keys.reduce((v, key) => v[key], value);
function reversed(value) {
  if (!value || typeof value !== "object") return value;
  return Object.fromEntries(Object.entries(value).reverse().map(([k, v]) => [k, reversed(v)]));
}

test("structural consistency only: I roundtrip and exact fixed inventories/names", () => {
  const f = fixture(), body = encodeInputs(f), decoded = decodeInputs(body);
  assert.deepEqual(decoded, f);
  assert.notStrictEqual(decoded, f);
  assert.deepEqual(Object.keys(decoded), ["schema", "identity", "authoring_mode", "asset_scope", "candidate_sha256",
    "pair_marker_sha256", "products", "preparation", "producer"]);
  assert.deepEqual(Object.keys(decoded.products), products);
  for (const product of products) {
    assert.deepEqual(Object.keys(decoded.products[product].assets), targets);
    for (const target of targets) {
      const a = decoded.products[product].assets[target];
      assert.equal(a.file, c.assetName(product, f.identity.versions[product], target));
      assert.equal(a.binary.file, c.executableName(product, target));
    }
  }
  assert.equal(new Set(products.flatMap(p => targets.map(t => decoded.products[p].assets[t].binary.sha256))).size, 12);
  decoded.producer.run_id++;
  assert.deepEqual(decodeInputs(body), f);
});

test("structural consistency only: constructors impose canonical ordering at every object", () => {
  const f = fixture(), body = json(f);
  assert.deepEqual(encodeInputs(reversed(f)), body);
  assert.deepEqual(encodeInputs(decodeInputs(body)), body);
  assert.throws(() => decodeInputs(json(reversed(f))), /noncanonical/);
  for (const product of products) {
    const d = descriptor(f, body, product);
    assert.deepEqual(encodeDescriptor(reversed(d), body, product), json(d));
    assert.deepEqual(decodeDescriptor(json(d), body, product), d);
    assert.deepEqual(encodeDescriptor(decodeDescriptor(json(d), body, product), body, product), json(d));
    assert.throws(() => decodeDescriptor(json(reversed(d)), body, product), /noncanonical/);
  }
});

test("structural consistency only: missing and extra fields rejected at every I object", () => {
  for (const path of objectPaths(fixture())) {
    for (const key of Object.keys(at(fixture(), path))) {
      rejectInput(f => { delete at(f, path)[key]; });
    }
    rejectInput(f => { at(f, path).unexpected = true; });
    for (const replacement of [null, [], "object", 1, true]) {
      if (path.length) rejectInput(f => { at(f, path.slice(0, -1))[path.at(-1)] = replacement; });
      else assert.throws(() => encodeInputs(replacement));
    }
  }
});

test("structural consistency only: all descriptor inventories are closed on both interfaces", () => {
  const f = fixture(), body = encodeInputs(f), d = descriptor(f, body, "agentplugins");
  const reject = value => {
    assert.throws(() => encodeDescriptor(value, body, "agentplugins"));
    assert.throws(() => decodeDescriptor(json(value), body, "agentplugins"));
  };
  for (const path of objectPaths(d)) {
    for (const key of Object.keys(at(d, path))) {
      const changed = copy(d); delete at(changed, path)[key]; reject(changed);
    }
    const changed = copy(d); at(changed, path).unexpected = false; reject(changed);
  }
  for (const value of [null, [], "object", 1, true]) reject(value);
});

test("structural consistency only: no claim injection, even false, hidden or accessor claims", () => {
  const claims = ["qualification", "signed_subject", "attested", "authenticated", "authenticated_provenance",
    "eligibility", "release_eligible", "acceptance", "platform_acceptance", "assertions", "tarball_sha256",
    "stage_sha256", "execution_sha256", "self_sha256", "artifact", "workflow_sha", "ref"];
  const f = fixture(), body = encodeInputs(f), d = descriptor(f, body, "plugin-kit-ai");
  for (const claim of claims) {
    for (const value of [true, false, null]) {
      rejectInput(x => { x[claim] = value; });
      const injected = { ...d, [claim]: value };
      assert.throws(() => encodeDescriptor(injected, body, "plugin-kit-ai"));
      assert.throws(() => decodeDescriptor(json(injected), body, "plugin-kit-ai"));
    }
  }
  for (const path of objectPaths(f)) {
    let calls = 0;
    const accessor = copy(f), node = at(accessor, path), key = Object.keys(node)[0];
    Object.defineProperty(node, key, { enumerable: true, get() { calls++; return f[key]; } });
    assert.throws(() => encodeInputs(accessor));
    assert.equal(calls, 0);
    const hidden = copy(f); Object.defineProperty(at(hidden, path), "accepted", { value: true });
    assert.throws(() => encodeInputs(hidden));
    const symbolic = copy(f); at(symbolic, path)[Symbol("trusted")] = true;
    assert.throws(() => encodeInputs(symbolic));
    const inherited = copy(f); Object.setPrototypeOf(at(inherited, path), { accepted: true });
    assert.throws(() => encodeInputs(inherited));
  }
});

test("structural consistency only: fixed source, versions, workflows, tags, mode and scope", () => {
  for (const repository of ["777genius/plugin-kit-ai", "fork/universal-agent-plugins", null]) {
    rejectInput(f => { f.identity.repository = repository; });
  }
  for (const commit of ["0".repeat(40), "A".repeat(40), "a".repeat(39), "g".repeat(40), 123,
    ...["\n", "\r", "\r\n"].map(suffix => "a".repeat(40) + suffix)]) {
    rejectInput(f => { f.identity.commit = f.identity.engine_revision = f.producer.source = commit; });
  }
  rejectInput(f => { f.identity.engine_revision = "b".repeat(40); });
  rejectInput(f => { f.producer.source = "b".repeat(40); });
  for (const version of ["2.0.0", "0.1.1-beta", "01.1.1", "0.1.1+build", "v0.1.1", 1, null,
    "0.1.99\n", "0.1.99\r", "0.1.99\r\n"]) {
    rejectInput(f => { f.identity.versions.agentplugins = version; });
  }
  rejectInput(f => { f.identity.versions["plugin-kit-ai"] = "2.0.1"; });
  for (const workflow of [".github/workflows/authoring-frozen-native.yml", ".github/workflows/agentplugins-npm-publish.yml", null]) {
    rejectInput(f => { f.producer.workflow = workflow; });
  }
  rejectInput(f => { f.products.agentplugins.tag = "v0.1.99"; });
  rejectInput(f => { f.products["plugin-kit-ai"].tag = "v2.0.1"; });
  rejectInput(f => { f.authoring_mode = "vertical-slice-v1"; });
  rejectInput(f => { f.asset_scope = "host-pair"; });
  rejectInput(f => { f.schema = "authoring-native-inputs/v2"; });
});

test("structural consistency only: strict hash types and nonzero lowercase digests everywhere", () => {
  const f = fixture();
  for (const path of objectPaths(f)) {
    for (const key of Object.keys(at(f, path)).filter(k => k.endsWith("sha256"))) {
      for (const value of [null, true, 123, "0".repeat(64), "A".repeat(64), "g".repeat(64), "1".repeat(63), "1".repeat(65),
        ...["\n", "\r", "\r\n"].map(suffix => "1".repeat(64) + suffix)]) {
        rejectInput(x => { at(x, path)[key] = value; });
      }
    }
  }
});

test("structural consistency only: positive safe IDs, attempt bounds and native size bounds", () => {
  for (const path of [["preparation", "artifact"], ["producer"]]) {
    for (const key of Object.keys(at(fixture(), path)).filter(k => k === "run_id" || k === "run_attempt" || k === "artifact_id")) {
      for (const value of [0, -1, 1.5, "1", true, null, Number.MAX_SAFE_INTEGER + 1, NaN, Infinity]) {
        rejectInput(f => { at(f, path)[key] = value; });
      }
      for (const value of [1, key === "run_attempt" ? 1000 : Number.MAX_SAFE_INTEGER]) {
        const f = fixture(); at(f, path)[key] = value; assert.deepEqual(decodeInputs(encodeInputs(f)), f);
      }
    }
    rejectInput(f => { at(f, path).run_attempt = 1001; });
  }
  for (const product of products) for (const target of targets) {
    for (const inner of [false, true]) {
      for (const size of [0, -1, 1.5, "1", null, true, contract.MAX_NATIVE_BYTES + 1]) {
        rejectInput(f => { const a = f.products[product].assets[target]; (inner ? a.binary : a).size = size; });
      }
    }
    for (const size of [1, contract.MAX_NATIVE_BYTES]) {
      const f = fixture(), a = f.products[product].assets[target]; a.size = a.binary.size = size;
      assert.deepEqual(decodeInputs(encodeInputs(f)), f);
    }
  }
});

test("structural consistency only: filenames, raw pins and all twelve binary identities", () => {
  for (const product of products) for (const target of targets) {
    for (const file of ["../asset", "asset.exe", "plugin-kit-ai", null]) {
      rejectInput(f => { f.products[product].assets[target].file = file; });
    }
    rejectInput(f => { f.products[product].assets[target].binary.file = "../binary"; });
    rejectInput(f => {
      const a = f.products[product].assets[target];
      const other = product === "agentplugins" ? "plugin-kit-ai" : "agentplugins";
      a.binary.sha256 = f.products[other].assets[target].binary.sha256;
      if (product === "agentplugins") a.sha256 = a.binary.sha256;
    });
    if (product === "agentplugins") {
      rejectInput(f => { f.products[product].assets[target].sha256 = sha(99); });
      rejectInput(f => { f.products[product].assets[target].size++; });
    }
  }
});

test("structural consistency only: v2 requires exact I bytes and selected product on both interfaces", () => {
  const f = fixture(), body = encodeInputs(f);
  for (const product of products) {
    const d = descriptor(f, body, product), encoded = encodeDescriptor(d, body, product);
    for (const selection of [undefined, null, "other", products.find(p => p !== product)]) {
      assert.throws(() => encodeDescriptor(d, body, selection));
      assert.throws(() => decodeDescriptor(encoded, body, selection));
    }
    const changes = [x => x.identity.commit = x.identity.engine_revision = "b".repeat(40),
      x => x.identity.versions.agentplugins = "0.1.98", x => x.candidate_sha256 = sha(99),
      x => x.release_manifest_sha256 = f.products[products.find(p => p !== product)].manifest_sha256,
      x => x.input_binding.sha256 = sha(99), x => x.input_binding.file = "../native-inputs.json",
      x => x.input_binding = null, x => x.npm_package = "wrong", x => x.authoring_mode = "vertical-slice-v1",
      x => x.asset_scope = "host-pair", x => x.schema = "dual-authoring-public-npm/v1"];
    for (const mutate of changes) {
      const changed = copy(d); mutate(changed);
      assert.throws(() => encodeDescriptor(changed, body, product));
      assert.throws(() => decodeDescriptor(json(changed), body, product));
    }
    for (const mutate of [x => x.preparation.artifact.artifact_id++, x => x.producer.run_attempt++,
      x => x.products["plugin-kit-ai"].checksums_sha256 = sha(99), x => x.pair_marker_sha256 = sha(99)]) {
      const changed = copy(f); mutate(changed); const changedBytes = encodeInputs(changed);
      assert.throws(() => encodeDescriptor(d, changedBytes, product));
      assert.throws(() => decodeDescriptor(encoded, changedBytes, product));
    }
    const noncanonical = json(reversed(f)), misleading = copy(d);
    misleading.input_binding.sha256 = c.digest(noncanonical);
    assert.throws(() => encodeDescriptor(misleading, noncanonical, product));
    assert.throws(() => decodeDescriptor(json(misleading), noncanonical, product));
  }
});

test("structural consistency only: bounded canonical UTF-8 bytes, duplicates and spellings", () => {
  const f = fixture(), input = encodeInputs(f), d = descriptor(f, input, "agentplugins");
  for (const [body, decode, maximum] of [[input, decodeInputs, contract.MAX_INPUT_BYTES],
    [json(d), b => decodeDescriptor(b, input, "agentplugins"), contract.MAX_DESCRIPTOR_BYTES]]) {
    const text = body.toString();
    for (const bad of [body.toString(), new Uint8Array(body), null, {}, Buffer.alloc(0), Buffer.alloc(maximum + 1),
      Buffer.concat([Buffer.from([0xef, 0xbb, 0xbf]), body]), Buffer.concat([body, Buffer.from([0xff])]),
      Buffer.from(text.trim()), Buffer.from(text + "\n"), Buffer.from(text + "{}"),
      Buffer.from(text.replace('"schema":', '"schema": null, "schema":')),
      Buffer.from(text.replace('"schema"', '"sch\\u0065ma"')),
      Buffer.from(text.replace('"repository":', '"repository": null, "repository":')),
      Buffer.from(text.replaceAll("\n", "\r\n")), Buffer.from('{"x":'.repeat(7) + '0' + '}'.repeat(7))]) {
      assert.throws(() => decode(bad));
    }
  }
  assert.throws(() => decodeInputs(Buffer.from(input.toString().replace('"run_id": 101', '"run_id": 1.01e2'))));
  assert.throws(() => decodeInputs(Buffer.from(input.toString().replace('"run_id": 101', '"run_id": 101.0'))));
});

test("structural consistency only: constructor and decoder byte limits include exact boundaries", () => {
  const base = encodeInputs(fixture("1.0.0")).length;
  const n = Math.floor((contract.MAX_INPUT_BYTES - base) / 8);
  const near = fixture("1" + "0".repeat(n) + ".0.0");
  // Version appears in identity, tag and six outer filenames. Adjust the run ID
  // digit count to exercise exactly 1 MiB without introducing unknown fields.
  const remaining = contract.MAX_INPUT_BYTES - encodeInputs(near).length;
  near.preparation.artifact.run_id = Number("1" + "0".repeat(2 + remaining));
  const limitBytes = encodeInputs(near);
  assert.equal(limitBytes.length, contract.MAX_INPUT_BYTES);
  assert.deepEqual(decodeInputs(limitBytes), near);
  assert.throws(() => encodeInputs(fixture("1" + "0".repeat(n + 1) + ".0.0")));
  assert.throws(() => encodeInputs(fixture("1".repeat(contract.MAX_INPUT_BYTES + 1))));

  const f = fixture("1.0.0"), body = encodeInputs(f);
  const overhead = json(descriptor(f, body, "agentplugins")).length;
  const large = fixture("1" + "0".repeat(contract.MAX_DESCRIPTOR_BYTES - overhead) + ".0.0");
  const largeBytes = encodeInputs(large), d = descriptor(large, largeBytes, "agentplugins");
  const exact = encodeDescriptor(d, largeBytes, "agentplugins");
  assert.equal(exact.length, contract.MAX_DESCRIPTOR_BYTES);
  assert.deepEqual(decodeDescriptor(exact, largeBytes, "agentplugins"), d);
  const over = fixture(large.identity.versions.agentplugins.replace("1", "10")), overBytes = encodeInputs(over);
  assert.throws(() => encodeDescriptor(descriptor(over, overBytes, "agentplugins"), overBytes, "agentplugins"));
});

test("structural consistency only: four codecs stay pure beside separately named custody operations", () => {
  assert.deepEqual(Object.entries(contract).filter(([, v]) => typeof v === "function").map(([k]) => k),
    ["encodeInputs", "decodeInputs", "encodeDescriptor", "decodeDescriptor", "produceInputs", "readInputs", "inputSubjects"]);
  assert.ok(Object.isFrozen(contract));
  const f = fixture(), snapshot = json(f), input = encodeInputs(f);
  const d = descriptor(f, input, "agentplugins"), before = json(d);
  encodeDescriptor(d, input, "agentplugins"); decodeInputs(input);
  assert.deepEqual(json(f), snapshot); assert.deepEqual(json(d), before); assert.deepEqual(input, snapshot);
});


// C1 tests are orchestration unit evidence only. Provider acquisition and
// signatures are stubbed module operations; these fixtures never authenticate.
const fs = require("node:fs");
const path = require("node:path");
const cp = require("node:child_process");
const promotion = require("../scripts/authoring-promotion");
const release = require("../scripts/authoring-release");
const qualification = require("../scripts/authoring-native-qualification");
const fixtureParent = "/tmp/uap-authoring-d5-c1-provenance-20260909-artifacts/unit-fixtures";

function c1Fixture(t) {
  t.mock.method(cp, "spawnSync", () => assert.fail("C1 tests must never launch a subprocess"));
  fs.mkdirSync(fixtureParent, { recursive: true });
  const sandbox = fs.mkdtempSync(path.join(fixtureParent, "provenance-"));
  const root = path.join(sandbox, "prepared"), provenance = path.join(sandbox, "provenance"), scratch = path.join(sandbox, "scratch");
  for (const dir of [root, provenance, scratch]) fs.mkdirSync(dir);
  const input = fixture(), inner = new Map();
  const write = (rel, bytes) => {
    const file = path.join(root, rel); fs.mkdirSync(path.dirname(file), { recursive: true });
    fs.writeFileSync(file, bytes); return c.metadata(bytes);
  };
  const manifest = { schema: c.SCHEMA, status: "CANDIDATE", identity: input.identity, asset_scope: input.asset_scope,
    build: { method: "controlled-git-archive-go-build/v1", go_version: "go1.25.13", go_sha256: sha(80),
      source_archive_sha256: sha(81), authoring_mode: input.authoring_mode }, products: {}, release_eligible: false };
  for (const product of products) {
    for (const target of targets) {
      const a = input.products[product].assets[target];
      const binary = Buffer.from("NONEXECUTABLE UNIT FIXTURE " + product + "/" + target);
      const outer = product === "agentplugins" ? binary : Buffer.from("OPAQUE OUTER FIXTURE " + target);
      a.binary = { file: a.binary.file, ...c.metadata(binary) };
      Object.assign(a, write(product + "/" + a.file, outer));
      inner.set(c.digest(outer), { file: a.binary.file, binary });
    }
    manifest.products[product] = { version: input.identity.versions[product], assets: input.products[product].assets };
  }
  // Stub the existing unpack operation only. No ZIP/tar internals are tested.
  t.mock.method(c, "unpack", (bytes, name) => {
    const row = inner.get(c.digest(bytes)); assert.ok(row); assert.equal(row.file, name); return row.binary;
  });
  input.candidate_sha256 = write("candidate/candidate.json", c.encode(manifest)).sha256;
  const pins = { identity: input.identity, candidate_sha256: input.candidate_sha256, pair_marker_sha256: "", products: {} };
  for (const product of products) {
    const p = input.products[product];
    const m = { schema_version: 3, status: "CANDIDATE", product, repository: c.REPOSITORY, tag: p.tag,
      version: input.identity.versions[product], commit: input.identity.commit, engine_revision: input.identity.commit,
      versions: input.identity.versions, candidate_sha256: input.candidate_sha256, authoring_mode: input.authoring_mode,
      asset_scope: input.asset_scope, assets: p.assets, release_eligible: false, platform_acceptance: false, attested: false };
    p.manifest_sha256 = write(product + "/release-manifest.json", c.encode(m)).sha256;
    const checks = Buffer.from([...Object.values(p.assets).map(a => a.sha256 + "  " + a.file),
      p.manifest_sha256 + "  release-manifest.json"].join("\n") + "\n");
    p.checksums_sha256 = write(product + "/checksums.txt", checks).sha256;
    pins.products[product] = { manifest_sha256: p.manifest_sha256, checksums_sha256: p.checksums_sha256 };
  }
  const marker = { schema: "authoring-release-pair/v1", status: "CANDIDATE", identity: input.identity,
    candidate_sha256: input.candidate_sha256, authoring_mode: input.authoring_mode, asset_scope: input.asset_scope,
    products: pins.products, release_eligible: false, platform_acceptance: false, attested: false };
  pins.pair_marker_sha256 = input.pair_marker_sha256 = write("pair-prepared.json", c.encode(marker)).sha256;
  const invocation = { repository: c.REPOSITORY, workflow: contract.WORKFLOW, source: input.identity.commit,
    workflow_sha: input.identity.commit, run_id: input.preparation.artifact.run_id, run_attempt: input.preparation.artifact.run_attempt };
  input.preparation.sha256 = qualification.writePreparation(root, pins, invocation).sha256;
  write("candidate-identity.json", c.encode({ identity: input.identity, status: "CANDIDATE",
    manifest_sha256: input.candidate_sha256, output: "/historical/not-opened", local_build_evidence: "/historical/not-opened-either",
    release_eligible: false, platform_acceptance: false, attested: false }));
  const original = release.verifyProjectedPair(root, pins).subjects;
  const files = [...original.map(s => path.relative(root, s.file)), "preparation-run.json", "candidate-identity.json"];
  for (const file of files) {
    fs.mkdirSync(path.dirname(path.join(provenance, file)), { recursive: true });
    fs.copyFileSync(path.join(root, file), path.join(provenance, file));
  }
  const body = encodeInputs(input);
  fs.writeFileSync(path.join(provenance, contract.INPUT_FILE), body);
  const selected = { tag: input.products.agentplugins.tag, ref: "refs/tags/" + input.products.agentplugins.tag,
    source: input.identity.commit, versions: copy(input.identity.versions) };
  const options = { input: body, selected, workflow_sha: input.identity.commit, scratch };
  const artifact = { run_id: input.producer.run_id, run_attempt: input.producer.run_attempt,
    artifact_id: 501, artifact_sha256: sha(90) };
  const reading = { ...options, artifact };
  const calls = [];
  t.mock.method(promotion, "checkInputTags", (bytes, cwd) => {
    calls.push("tags"); assert.deepEqual(bytes, body); assert.equal(cwd, scratch);
  });
  t.mock.method(promotion, "inspectArtifact", (pin, workflow, source, cwd) => {
    calls.push("inspect"); assert.equal(workflow, contract.WORKFLOW); assert.equal(source, input.identity.commit);
    assert.equal(cwd, scratch); assert.ok([artifact.artifact_id, input.preparation.artifact.artifact_id].includes(pin.artifact_id));
    return { fixture_only: copy(pin) };
  });
  t.mock.method(promotion, "acquireInputPreparation", (bytes, cwd) => {
    calls.push("preparation"); assert.equal(cwd, scratch); return promotion.readInputPreparation(root, bytes);
  });
  t.mock.method(promotion, "acquireArtifact", (pin, workflow, source, cwd) => {
    calls.push("acquire"); assert.deepEqual(pin, artifact); assert.equal(workflow, contract.WORKFLOW);
    assert.equal(source, input.identity.commit); return path.join(cwd, "artifact-501.zip");
  });
  t.mock.method(promotion, "extractArtifact", (file, pin, kind, closure, output, cwd) => {
    calls.push("extract"); assert.equal(file, path.join(cwd, "artifact-501.zip")); assert.deepEqual(pin, artifact);
    assert.equal(kind, "input-provenance"); assert.equal(output, path.join(cwd, "frozen"));
    assert.deepEqual([...closure].sort(), [...files, contract.INPUT_FILE].sort()); assert.equal(closure.length, 21);
    // A simulated future interface, NOT the current checked reader.
    return provenance;
  });
  t.mock.method(promotion, "verifySubject", (file, expected, cwd) => {
    calls.push({ file, expected: copy(expected) }); assert.equal(cwd, scratch);
    assert.equal(c.digest(fs.readFileSync(file)), expected.sha256);
    assert.equal(expected.source, input.identity.commit); assert.equal(expected.workflow_sha, input.identity.commit);
    assert.equal(expected.ref, selected.ref); assert.equal(expected.run_id, input.producer.run_id);
    assert.equal(expected.run_attempt, input.producer.run_attempt);
  });
  return { root, provenance, scratch, input, body, options, reading, original, files, pins, calls };
}

test("C1 provenance producer and reader agree with honest simulated custody, never authentic admission", t => {
  const f = c1Fixture(t), produced = contract.produceInputs(f.options);
  assert.deepEqual(produced.input, f.input); assert.equal(produced.subjects.length, 19);
  assert.equal(f.calls.filter(x => typeof x === "object").length, 0, "producer does not verify or sign unsigned I");
  assert.deepEqual(fs.readFileSync(path.join(f.root, contract.INPUT_FILE)), f.body);
  const read = contract.readInputs(f.reading);
  assert.deepEqual(read.input, produced.input);
  const multiset = list => list.map(s => ({ name: path.basename(s.file), digest: { sha256: s.sha256 } }));
  assert.deepEqual(multiset(read.subjects), multiset(produced.subjects));
  const signatures = f.calls.filter(x => typeof x === "object");
  assert.equal(signatures.length, 19);
  for (const call of signatures) assert.deepEqual(call.expected.subjects, multiset(read.subjects));
  for (const name of ["release-manifest.json", "checksums.txt"]) {
    const rows = signatures[0].expected.subjects.filter(s => s.name === name);
    assert.equal(rows.length, 2); assert.notEqual(rows[0].digest.sha256, rows[1].digest.sha256);
  }
  assert.ok(!signatures[0].expected.subjects.some(s => s.name === "authoring-promotion.json"));
  assert.deepEqual(Object.keys(read), ["root", "input", "subjects"]);
});

test("C1 provenance rejects malformed closed options and independent identity disagreements before effects", t => {
  const f = c1Fixture(t);
  const mutations = [o => o.input = null, o => o.input = Buffer.from('{}\n'), o => o.input = Buffer.concat([o.input, Buffer.from('\n')]),
    o => o.verifier = () => true, o => o.success = true, o => o.selected.source = "b".repeat(40),
    o => o.selected.ref = "refs/heads/main", o => o.selected.tag = "v2.0.0", o => o.workflow_sha = "b".repeat(40),
    o => o.selected.versions.agentplugins = "0.1.98", o => o.scratch = "relative"];
  for (const reading of [false, true]) for (const mutate of mutations) {
    const o = { ...f.options, input: Buffer.from(f.body), selected: copy(f.options.selected), ...(reading ? { artifact: copy(f.reading.artifact) } : {}) };
    mutate(o); assert.throws(() => (reading ? contract.readInputs : contract.produceInputs)(o));
  }
  for (const mutate of [o => o.artifact.run_attempt++, o => o.artifact.run_id++, o => o.artifact.artifact_id = 0,
    o => o.artifact.artifact_sha256 = "0".repeat(64), o => o.artifact.artifact_id = f.input.preparation.artifact.artifact_id,
    o => o.artifact.claim = true]) {
    const o = { ...f.reading, artifact: copy(f.reading.artifact) }; mutate(o); assert.throws(() => contract.readInputs(o));
  }
  for (const mutate of [i => i.identity.repository = "fork/repo", i => i.producer.workflow = ".github/workflows/other.yml",
    i => i.producer.source = "b".repeat(40), i => i.products["plugin-kit-ai"].tag = "v2.0.1",
    i => delete i.products.agentplugins.assets["linux-amd64"], i => i.qualification = null,
    i => i.preparation.artifact.run_attempt = 1001, i => i.authoring_mode = "wrong", i => i.asset_scope = "wrong"]) {
    const input = copy(f.input); mutate(input);
    assert.throws(() => contract.produceInputs({ ...f.options, input: json(input) }));
    assert.throws(() => contract.readInputs({ ...f.reading, input: json(input) }));
  }
  assert.deepEqual(f.calls, []); assert.equal(fs.existsSync(path.join(f.root, contract.INPUT_FILE)), false);
  assert.deepEqual(fs.readdirSync(f.scratch), []);
});

test("C1 provenance checked-artifact dependency rejects with no fallback or signature calls", t => {
  const f = c1Fixture(t);
  t.mock.method(promotion, "extractArtifact", (_file, _pin, kind, files) => {
    assert.equal(kind, "input-provenance"); assert.equal(files.length, 21);
    throw Error("simulated current checked interface rejects unsupported kind");
  });
  assert.throws(() => contract.readInputs(f.reading), /unsupported kind/);
  assert.deepEqual(f.calls, ["tags", "inspect", "acquire"]);
});

for (const kind of ["receipt", "attempt", "metadata", "outer", "inner", "I"]) {
  test("C1 provenance rejects " + kind + " disagreement before signatures or I output", t => {
    const f = c1Fixture(t);
    if (kind === "receipt") {
      f.input.preparation.sha256 = sha(97); f.options.input = f.reading.input = encodeInputs(f.input);
      // Identity-only stub: receipt validation itself stays the real reader.
      t.mock.method(promotion, "checkInputTags", () => {});
    } else if (kind === "attempt") {
      // Keep the existing read-only receipt intact; select a disagreeing attempt.
      f.input.preparation.artifact.run_attempt++; f.options.input = encodeInputs(f.input);
      t.mock.method(promotion, "checkInputTags", () => {});
    } else if (kind === "metadata") {
      const file = path.join(f.root, "candidate-identity.json"), metadata = JSON.parse(fs.readFileSync(file));
      metadata.attested = true; fs.writeFileSync(file, c.encode(metadata));
    } else if (kind === "outer") fs.appendFileSync(f.original[3].file, "CHANGED");
    else if (kind === "inner") {
      f.input.products["plugin-kit-ai"].assets["linux-amd64"].binary.sha256 = sha(99);
      f.options.input = encodeInputs(f.input); t.mock.method(promotion, "checkInputTags", () => {});
    } else fs.appendFileSync(path.join(f.provenance, contract.INPUT_FILE), "\n");
    if (kind === "I") assert.throws(() => contract.readInputs(f.reading), /I bytes/);
    else assert.throws(() => contract.produceInputs(f.options));
    assert.equal(fs.existsSync(path.join(f.root, contract.INPUT_FILE)), false);
    assert.equal(f.calls.filter(x => typeof x === "object").length, 0);
  });
}

for (const operation of ["produce", "read"]) for (const changed of ["caller", "source", "subject", "metadata", "provider", "tag"]) {
  test("C1 provenance " + operation + " rejects late " + changed + " changes", t => {
    const f = c1Fixture(t); let tags = 0, inspections = 0;
    const selectedOptions = operation === "read" ? f.reading : f.options;
    t.mock.method(promotion, "checkInputTags", () => {
      if (++tags !== 2) return;
      if (changed === "caller") selectedOptions.input[1] ^= 1;
      if (changed === "source") selectedOptions.workflow_sha = "b".repeat(40);
      if (changed === "subject") fs.appendFileSync(operation === "read" ? path.join(f.provenance, f.files[0]) : f.original[0].file, "changed");
      if (changed === "metadata") {
        const file = path.join(operation === "read" ? f.provenance : f.root, "candidate-identity.json");
        const value = JSON.parse(fs.readFileSync(file)); value.output = "/different-history"; fs.writeFileSync(file, c.encode(value));
      }
      if (changed === "tag") throw Error("simulated moved release tag");
    });
    t.mock.method(promotion, "inspectArtifact", pin => ({ pin: copy(pin), observation: changed === "provider" && ++inspections > (operation === "read" ? 2 : 1) ? "changed" : "same" }));
    assert.throws(() => (operation === "read" ? contract.readInputs : contract.produceInputs)(selectedOptions));
    if (operation === "produce") assert.equal(fs.existsSync(path.join(f.root, contract.INPUT_FILE)), false);
  });
}

test("C1 provenance unsigned verifier rejection never returns reader admission", t => {
  const f = c1Fixture(t); let calls = 0;
  t.mock.method(promotion, "verifySubject", () => { calls++; throw Error("unsigned fixture rejected"); });
  assert.throws(() => contract.readInputs(f.reading), /unsigned fixture rejected/); assert.equal(calls, 1);
});

test("C1 provenance rechecks earlier subjects and I after the last signature operation", t => {
  const f = c1Fixture(t); let calls = 0;
  t.mock.method(promotion, "verifySubject", () => {
    if (++calls === 19) fs.appendFileSync(path.join(f.provenance, f.files[0]), "changed after first verification");
  });
  assert.throws(() => contract.readInputs(f.reading)); assert.equal(calls, 19);
});

test("C1 provenance reader rechecks exact I bytes after all nineteen simulated verifications", t => {
  const f = c1Fixture(t); let calls = 0;
  t.mock.method(promotion, "verifySubject", () => {
    if (++calls === 19) fs.appendFileSync(path.join(f.provenance, contract.INPUT_FILE), "\n");
  });
  assert.throws(() => contract.readInputs(f.reading), /retained I bytes/); assert.equal(calls, 19);
});

test("C1 provenance producer checks unchanged preparation after writing I without returning completion", t => {
  const f = c1Fixture(t), write = fs.writeFileSync;
  t.mock.method(fs, "writeFileSync", (file, bytes, options) => {
    write(file, bytes, options);
    if (file === path.join(f.root, contract.INPUT_FILE)) fs.appendFileSync(f.original[0].file, "late fixture change");
  });
  assert.throws(() => contract.produceInputs(f.options));
  // A failed local candidate is retained; it is never an uploaded completed run.
  assert.equal(f.calls.filter(x => typeof x === "object").length, 0);
});

test("C1 provenance reader compares preparation metadata from independently acquired bytes", t => {
  const f = c1Fixture(t), file = path.join(f.provenance, "candidate-identity.json");
  const metadata = JSON.parse(fs.readFileSync(file)); metadata.output = "/different-historical-location";
  fs.writeFileSync(file, c.encode(metadata));
  assert.throws(() => contract.readInputs(f.reading), /original preparation custody/);
  assert.equal(f.calls.filter(x => typeof x === "object").length, 0);
});

test("C1 provenance enumeration rejects missing original row and changed I without authenticating", t => {
  const f = c1Fixture(t), verify = release.verifyProjectedPair;
  t.mock.method(release, "verifyProjectedPair", (root, pins) => {
    const result = verify(root, pins); return { ...result, subjects: result.subjects.slice(1) };
  });
  assert.throws(() => contract.inputSubjects(f.provenance, f.body), /original subject count/);
  assert.deepEqual(f.calls, []);
});

test("C1 provenance producer completion collision preserves the existing file", t => {
  const f = c1Fixture(t), file = path.join(f.root, contract.INPUT_FILE), previous = Buffer.from("existing output");
  fs.writeFileSync(file, previous);
  assert.throws(() => contract.produceInputs(f.options), /EEXIST/);
  assert.deepEqual(fs.readFileSync(file), previous);
});
