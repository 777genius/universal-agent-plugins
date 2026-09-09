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

test("structural consistency only: only four pure codec operations, no effectful or trust API", () => {
  assert.deepEqual(Object.entries(contract).filter(([, v]) => typeof v === "function").map(([k]) => k),
    ["encodeInputs", "decodeInputs", "encodeDescriptor", "decodeDescriptor"]);
  assert.ok(Object.isFrozen(contract));
  const f = fixture(), snapshot = json(f), input = encodeInputs(f);
  const d = descriptor(f, input, "agentplugins"), before = json(d);
  encodeDescriptor(d, input, "agentplugins"); decodeInputs(input);
  assert.deepEqual(json(f), snapshot); assert.deepEqual(json(d), before); assert.deepEqual(input, snapshot);
});
