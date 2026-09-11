"use strict";

const assert = require("node:assert/strict");
const crypto = require("node:crypto");
const cp = require("node:child_process");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const test = require("node:test");

const {
  validateAuditSignatures,
  validateDownloadedTarball,
  validatePackJSON,
  validateProductPackJSON,
  validatePublicMetadata,
  validateSLSAAttestation
} = require("../scripts/npm-public-contract");

function fixture(body = Buffer.from("exact tarball bytes")) {
  const version = "1.2.3";
  const integrity = `sha512-${crypto.createHash("sha512").update(body).digest("base64")}`;
  const shasum = crypto.createHash("sha1").update(body).digest("hex");
  const pack = [{
    name: "universal-agent-plugins",
    version,
    filename: `universal-agent-plugins-${version}.tgz`,
    integrity,
    shasum
  }];
  const metadata = {
    name: "universal-agent-plugins",
    version,
    homepage: "https://777genius.github.io/universal-agent-plugins/",
    repository: {
      type: "git",
      url: "git+https://github.com/777genius/universal-agent-plugins.git",
      directory: "npm/agentplugins"
    },
    engines: { node: ">=22" },
    bin: { agentplugins: "bin/agentplugins.js" },
    scripts: { test: "node --test" },
    _npmUser: {
      name: "GitHub Actions",
      email: "npm-oidc-no-reply@github.com",
      trustedPublisher: { id: "github", oidcConfigId: "oidc:001caef4-dbce-4b2d-a25d-1d6ee59b68ac" }
    },
    dist: {
      integrity,
      shasum,
      tarball: `https://registry.npmjs.org/universal-agent-plugins/-/universal-agent-plugins-${version}.tgz`,
      attestations: {
        url: `https://registry.npmjs.org/-/npm/v1/attestations/universal-agent-plugins@${version}`,
        provenance: { predicateType: "https://slsa.dev/provenance/v1" }
      }
    }
  };
  const uapTag = `agentplugins-v${version}`;
  const uapCommit = "a".repeat(40);
  const statement = {
    _type: "https://in-toto.io/Statement/v1",
    subject: [{
      name: `pkg:npm/universal-agent-plugins@${version}`,
      digest: { sha512: crypto.createHash("sha512").update(body).digest("hex") }
    }],
    predicateType: "https://slsa.dev/provenance/v1",
    predicate: {
      buildDefinition: {
        buildType: "https://slsa-framework.github.io/github-actions-buildtypes/workflow/v1",
        externalParameters: {
          workflow: {
            ref: `refs/tags/${uapTag}`,
            repository: "https://github.com/777genius/universal-agent-plugins",
            path: ".github/workflows/agentplugins-npm-publish.yml"
          }
        },
        resolvedDependencies: [{
          uri: `git+https://github.com/777genius/universal-agent-plugins@refs/tags/${uapTag}`,
          digest: { gitCommit: uapCommit }
        }]
      },
      runDetails: {
        builder: { id: "https://github.com/actions/runner/github-hosted" },
        metadata: {
          invocationId: "https://github.com/777genius/universal-agent-plugins/actions/runs/123/attempts/1"
        }
      }
    }
  };
  const attestations = {
    attestations: [{ predicateType: "https://github.com/npm/attestation/tree/main/specs/publish/v0.1" }, {
      predicateType: "https://slsa.dev/provenance/v1",
      bundle: {
        mediaType: "application/vnd.dev.sigstore.bundle.v0.3+json",
        dsseEnvelope: {
          payload: Buffer.from(JSON.stringify(statement)).toString("base64"),
          payloadType: "application/vnd.in-toto+json",
          signatures: [{ sig: "signed", keyid: "" }]
        }
      }
    }]
  };
  return { attestations, body, integrity, metadata, pack, shasum, statement, uapCommit, uapTag, version };
}

test("public npm metadata and downloaded pack bind the staged package identity", (t) => {
  const value = fixture();
  assert.equal(validatePackJSON(value.pack, value.version), value.pack[0]);
  assert.equal(validatePublicMetadata(value.metadata, value.version, value.integrity, value.shasum), value.metadata);
  assert.equal(
    validatePublicMetadata([value.metadata], value.version, value.integrity, value.shasum),
    value.metadata
  );
  const publicStringPublisher = JSON.parse(JSON.stringify(value.metadata));
  publicStringPublisher._npmUser = "GitHub Actions <npm-oidc-no-reply@github.com>";
  assert.equal(
    validatePublicMetadata(publicStringPublisher, value.version, value.integrity, value.shasum),
    publicStringPublisher
  );
  assert.throws(
    () => validatePublicMetadata([], value.version, value.integrity, value.shasum),
    /exactly one release record/
  );
  assert.throws(
    () => validatePublicMetadata([value.metadata, value.metadata], value.version, value.integrity, value.shasum),
    /exactly one release record/
  );
  assert.throws(
    () => validatePublicMetadata({ ...publicStringPublisher, _npmUser: "Unknown <unknown@example.com>" }, value.version, value.integrity, value.shasum),
    /publisher identity is not GitHub Actions/
  );
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "npm-public-contract-"));
  t.after(() => cp.execFileSync("rm", ["-r", "--", root]));
  fs.writeFileSync(path.join(root, value.pack[0].filename), value.body);
  assert.equal(
    validateDownloadedTarball(value.pack, root, value.version, value.integrity, value.shasum),
    value.pack[0]
  );
});

test("npm 12 object-shaped pack JSON preserves the exact package identity", (t) => {
  const value = fixture();
  const npm12 = { "universal-agent-plugins": structuredClone(value.pack[0]) };
  assert.equal(validatePackJSON(npm12, value.version), npm12["universal-agent-plugins"]);
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "npm-public-contract-npm12-"));
  t.after(() => cp.execFileSync("rm", ["-r", "--", root]));
  fs.writeFileSync(path.join(root, value.pack[0].filename), value.body);
  assert.equal(
    validateDownloadedTarball(npm12, root, value.version, value.integrity, value.shasum),
    npm12["universal-agent-plugins"]
  );
  assert.throws(
    () => validatePackJSON({ ...npm12, lookalike: structuredClone(value.pack[0]) }, value.version),
    /exactly one package-named record/
  );
  assert.throws(
    () => validatePackJSON({ lookalike: structuredClone(value.pack[0]) }, value.version),
    /exactly one package-named record/
  );
});

test("public npm SLSA DSSE binds staged digest and exact UAP workflow tag commit", () => {
  const value = fixture();
  assert.deepEqual(
    validateSLSAAttestation(
      value.attestations, value.version, value.integrity, value.uapTag, value.uapCommit
    ),
    value.statement
  );
});

test("npm audit signatures output binds cryptographic verification to the fetched bundles", () => {
  const value = fixture();
  const audit = {
    invalid: [],
    missing: [],
    verified: [{
      name: "universal-agent-plugins",
      version: value.version,
      location: "node_modules/universal-agent-plugins",
      registry: "https://registry.npmjs.org/",
      attestations: {
        url: `https://registry.npmjs.org/-/npm/v1/attestations/universal-agent-plugins@${value.version}`,
        provenance: { predicateType: "https://slsa.dev/provenance/v1" }
      },
      attestationBundles: value.attestations.attestations
    }]
  };
  assert.equal(validateAuditSignatures(audit, value.attestations, value.version), audit.verified[0]);
  for (const mutate of [
    (x) => { x.invalid.push({ name: "universal-agent-plugins" }); },
    (x) => { x.verified[0].name = "lookalike"; },
    (x) => { x.verified[0].attestationBundles[1].bundle.dsseEnvelope.payload = "e30="; }
  ]) {
    const invalid = structuredClone(audit);
    mutate(invalid);
    assert.throws(() => validateAuditSignatures(invalid, value.attestations, value.version));
  }
});

test("public npm SLSA DSSE fails closed for every reviewed provenance binding", () => {
  const value = fixture();
  const mutations = [
    (x) => { x.attestations[1].predicateType = "https://example.invalid/predicate"; },
    (x) => { x.attestations.push(structuredClone(x.attestations[1])); },
    (x) => { x.attestations[1].bundle.mediaType = "application/json"; },
    (x) => { x.attestations[1].bundle.dsseEnvelope.payloadType = "application/json"; },
    (x) => { x.attestations[1].bundle.dsseEnvelope.signatures = []; },
    (x) => mutatePayload(x, (statement) => { statement.subject[0].digest.sha512 = "0".repeat(128); }),
    (x) => mutatePayload(x, (statement) => { statement.subject[0].name = "pkg:npm/lookalike@1.2.3"; }),
    (x) => mutatePayload(x, (statement) => { statement.predicateType = "https://example.invalid/predicate"; }),
    (x) => mutatePayload(x, (statement) => {
      statement.predicate.buildDefinition.externalParameters.workflow.repository =
        "https://github.com/lookalike/repository";
    }),
    (x) => mutatePayload(x, (statement) => {
      statement.predicate.buildDefinition.externalParameters.workflow.path = "/.github/workflows/lookalike.yml";
    }),
    (x) => mutatePayload(x, (statement) => {
      statement.predicate.buildDefinition.externalParameters.workflow.path =
        "/.github/workflows/agentplugins-npm-publish.yml";
    }),
    (x) => mutatePayload(x, (statement) => {
      statement.predicate.buildDefinition.externalParameters.workflow.ref = "refs/heads/main";
    }),
    (x) => mutatePayload(x, (statement) => {
      statement.predicate.buildDefinition.resolvedDependencies[0].digest.gitCommit = "b".repeat(40);
    }),
    (x) => mutatePayload(x, (statement) => {
      statement.predicate.runDetails.builder.id = "https://example.invalid/runner";
    }),
    (x) => mutatePayload(x, (statement) => {
      statement.predicate.runDetails.metadata.invocationId =
        "https://github.com/lookalike/repository/actions/runs/123/attempts/1";
    })
  ];
  for (const mutate of mutations) {
    const invalid = structuredClone(value.attestations);
    mutate(invalid);
    assert.throws(() => validateSLSAAttestation(
      invalid, value.version, value.integrity, value.uapTag, value.uapCommit
    ));
  }
});

function mutatePayload(attestations, mutate) {
  const envelope = attestations.attestations[1].bundle.dsseEnvelope;
  const statement = JSON.parse(Buffer.from(envelope.payload, "base64").toString("utf8"));
  mutate(statement);
  envelope.payload = Buffer.from(JSON.stringify(statement)).toString("base64");
}

test("public npm metadata fails closed for every reviewed identity field", () => {
  const value = fixture();
  const mutations = [
    (x) => { x.name = "lookalike"; },
    (x) => { x.version = "1.2.4"; },
    (x) => { x.repository.url = "git+https://github.com/lookalike/repository.git"; },
    (x) => { x.homepage = "https://example.invalid/"; },
    (x) => { x.dist.tarball = "https://example.invalid/package.tgz"; },
    (x) => { x.dist.integrity = `sha512-${Buffer.alloc(64).toString("base64")}`; },
    (x) => { x.dist.shasum = "0".repeat(40); },
    (x) => { x.dist.attestations.url = "https://example.invalid/attestation"; },
    (x) => { x.dist.attestations.provenance.predicateType = "https://example.invalid/predicate"; },
    (x) => { x._npmUser.name = "npm user"; },
    (x) => { x._npmUser.trustedPublisher.id = "other"; },
    (x) => { x.engines.node = ">=22.23.2"; },
    (x) => { x.bin.agentplugins = "bin/lookalike.js"; },
    (x) => { x.scripts.postinstall = "node install.js"; }
  ];
  for (const mutate of mutations) {
    const invalid = structuredClone(value.metadata);
    mutate(invalid);
    assert.throws(() => validatePublicMetadata(invalid, value.version, value.integrity, value.shasum));
  }
});

test("downloaded public npm bytes and pack JSON reject staged digest mismatches", (t) => {
  const value = fixture();
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "npm-public-contract-negative-"));
  t.after(() => cp.execFileSync("rm", ["-r", "--", root]));
  fs.writeFileSync(path.join(root, value.pack[0].filename), Buffer.from("tampered"));
  assert.throws(
    () => validateDownloadedTarball(value.pack, root, value.version, value.integrity, value.shasum),
    /bytes do not match/
  );
  const wrongPack = structuredClone(value.pack);
  wrongPack[0].shasum = "0".repeat(40);
  assert.throws(
    () => validateDownloadedTarball(wrongPack, root, value.version, value.integrity, value.shasum),
    /pack identity/
  );
});

const products = { agentplugins: "universal-agent-plugins", "plugin-kit-ai": "plugin-kit-ai" };
const responseForms = {
  array: record => [record],
  object: record => ({ [record.name]: record })
};
function productRecord(product) {
  const record = fixture().pack[0];
  record.name = products[product];
  record.filename = `${record.name}-${record.version}.tgz`;
  return record;
}

for (const product of Object.keys(products)) for (const [form, wrap] of Object.entries(responseForms)) {
  test(`C1 fixed pack ${product} ${form}: identity, shape and digest syntax`, () => {
    const record = productRecord(product);
    // npm carries additional informational fields; these are not a new schema.
    record.files = [{ path: "package.json", size: 123, mode: 420 }];
    assert.equal(validateProductPackJSON(wrap(record), product, record.version), record);
    if (product === "agentplugins") assert.equal(validatePackJSON(wrap(record), record.version), record);
    else assert.throws(() => validatePackJSON(wrap(record), record.version));
    for (const mutation of [
      x => { x.name = "lookalike"; },
      x => { x.version = "1.2.4"; },
      x => { x.filename = "../" + x.filename; },
      x => { x.filename = x.filename.replace("1.2.3", "1.2.4"); },
      x => { x.integrity = undefined; },
      x => { x.integrity = 42; },
      x => { x.integrity = "sha256-" + "a".repeat(86) + "=="; },
      x => { x.integrity += "\n"; },
      x => { x.integrity = "sha512-" + "A".repeat(85) + "B=="; },
      x => { x.shasum = undefined; },
      x => { x.shasum = 42; },
      x => { x.shasum = "A".repeat(40); },
      x => { x.shasum = "a".repeat(39); },
      x => { x.shasum += "\n"; }
    ]) {
      const bad = structuredClone(record); mutation(bad);
      assert.throws(() => validateProductPackJSON(wrap(bad), product, record.version));
    }
    for (const value of [null, false, "pack", [], [record, record], [null], {},
      { lookalike: record }, { [record.name]: record, extra: record },
      { [record.name]: [record] }]) {
      assert.throws(() => validateProductPackJSON(value, product, record.version));
    }
    for (const version of [null, 123, "01.2.3", "1.2.3-beta.1", "1.2.3+build", "1.2.3\n"]) {
      assert.throws(() => validateProductPackJSON(wrap(record), product, version));
    }
    for (const unknown of ["universal-agent-plugins", "other", "toString", null]) {
      assert.throws(() => validateProductPackJSON(wrap(record), unknown, record.version), /unknown fixed npm product/);
    }
  });
}

test("C1 blob checks bind the helper while preserving both historical receipt inventories", t => {
  const packing = require("../scripts/stage-dual-authoring-npm");
  const publicPacking = require("../scripts/stage-authoring-npm");
  const c = require("../scripts/dual-authoring-candidate");
  const repo = path.resolve(__dirname, "../../..");
  const helper = "npm/agentplugins/scripts/npm-public-contract.js";
  const commit = "a".repeat(40);
  const readFile = c.readFile;
  let fault;
  const requested = [];
  // Simulated committed-source interface, not an accepted Git successor.
  t.mock.method(cp, "execFileSync", (exe, args) => {
    assert.equal(exe, "/usr/bin/git");
    if (args[0] === "rev-parse") return Buffer.from(commit + "\n");
    if (args[0] === "ls-tree") {
      const name = args.at(-1); requested.push(name);
      if (fault === "missing" && name === helper) return Buffer.from("");
      const bytes = fs.readFileSync(path.join(repo, name));
      const hash = crypto.createHash("sha1").update(`blob ${bytes.length}\0`).update(bytes).digest("hex");
      bodies.set(hash, bytes);
      return Buffer.from(`100644 blob ${hash}\t${name}\0`);
    }
    assert.equal(args[0], "cat-file"); return bodies.get(args[2]);
  });
  const bodies = new Map();
  t.mock.method(c, "readFile", (file, ...rest) => fault === "dirty" && file === path.join(repo, helper) ?
    Buffer.from("changed executing helper") : readFile(file, ...rest));
  for (const [closure, allowlist] of [["private", packing.ALLOWLIST], ["public", publicPacking.ALLOWLIST]]) {
    assert.equal(allowlist.includes(helper), false);
    const source = packing.blobs(repo, commit, {}, closure);
    assert.deepEqual(Object.keys(source), [...allowlist]);
    assert.ok(requested.includes(helper));
    for (const name of allowlist) assert.deepEqual(source[name].bytes, fs.readFileSync(path.join(repo, name)));
    for (fault of ["missing", "dirty"]) {
      assert.throws(() => packing.blobs(repo, commit, {}, closure), /required regular Git blob missing|executing stager differs/);
    }
    fault = undefined;
    assert.throws(() => packing.blobs(repo, "b".repeat(40), {}, closure), /checkout HEAD/);
  }
});

// Explicit offline tool provision only. Retain small fixtures under TMPDIR for
// evidence; never import the native-building private-npm fixture suite.
test("C1 packPackage real offline packs: both forms/products, same receipts, rejection before completion", {
  skip: !process.env.UAP_C1_PACK_NPM
}, t => {
  const packing = require("../scripts/stage-dual-authoring-npm");
  const scratch = fs.mkdtempSync(path.join(os.tmpdir(), "c1-pack-consumer-"));
  const npm = process.env.UAP_C1_PACK_NPM;
  assert.ok(path.isAbsolute(npm));
  const execFile = cp.execFileSync;
  const actualRecords = {}, productBytes = {};
  for (const product of Object.keys(products)) for (const [form, wrap] of Object.entries(responseForms)) {
    const base = path.join(scratch, `${product}-${form}`); fs.mkdirSync(base);
    const context = packing.npmContext(base);
    const root = path.join(base, "package"); fs.mkdirSync(root);
    const output = path.join(base, "output"); fs.mkdirSync(output);
    const files = { "package.json": Buffer.from(JSON.stringify({
      name: products[product], version: "1.2.3", private: true,
      scripts: { prepack: "exit 91", prepare: "exit 92", postpack: "exit 93", postinstall: "exit 94" }
    }) + "\n"), "README.md": Buffer.from("Offline fixed pack consumer fixture.\n") };
    for (const [name, body] of Object.entries(files)) fs.writeFileSync(path.join(root, name), body, { mode: 0o644 });
    let calls = 0;
    const mock = t.mock.method(cp, "execFileSync", (exe, args, options) => {
      if (exe !== process.execPath || args[0] !== npm) return execFile(exe, args, options);
      calls++;
      assert.deepEqual(args.slice(1), ["pack", "--ignore-scripts", "--offline", "--json", "--pack-destination", output]);
      const response = JSON.parse(execFile(exe, args, options));
      const record = validateProductPackJSON(response, product, "1.2.3");
      actualRecords[product] = record;
      return Buffer.from(JSON.stringify(wrap(record)));
    });
    const options = { node: process.execPath, npm, output, identity: { versions: { [product]: "1.2.3" } } };
    const receipt = packing.packPackage(product, files, root, options, context);
    mock.mock.restore();
    assert.equal(calls, 1);
    const body = fs.readFileSync(path.join(output, receipt.file));
    assert.deepEqual(receipt, { file: `${products[product]}-1.2.3.tgz`,
      sha256: crypto.createHash("sha256").update(body).digest("hex"), size: body.length,
      integrity: "sha512-" + crypto.createHash("sha512").update(body).digest("base64") });
    if (productBytes[product]) assert.deepEqual(body, productBytes[product]);
    productBytes[product] = body;
    assert.equal(actualRecords[product].shasum, crypto.createHash("sha1").update(body).digest("hex"));
    assert.deepEqual(fs.readFileSync(path.join(context.root, product, "package/README.md")), files["README.md"]);
    // Same bytes and real response, with one field corrupted. No extraction or
    // downstream completion is allowed even when the other digest is correct.
    for (const field of ["name", "version", "filename", "integrity", "shasum"]) {
      const bad = { ...actualRecords[product], [field]: field === "integrity" ?
        "sha512-" + Buffer.alloc(64).toString("base64") : field === "shasum" ? "0".repeat(40) : "wrong" };
      let tarCalls = 0;
      const rejection = t.mock.method(cp, "execFileSync", (exe, args) => {
        if (exe === process.execPath && args[0] === npm) return Buffer.from(JSON.stringify(wrap(bad)));
        tarCalls++; throw new Error("unexpected downstream tool");
      });
      const marker = path.join(output, "completion.json");
      assert.throws(() => {
        const pack = packing.packPackage(product, files, root, options, context);
        packing.completeRecord(output, { pack });
      }, /package identity|package-named record|differs from actual pack/);
      rejection.mock.restore();
      assert.equal(tarCalls, 0); assert.equal(fs.existsSync(marker), false);
    }
    fs.writeFileSync(path.join(output, receipt.file), Buffer.from("changed tarball bytes"));
    const changed = t.mock.method(cp, "execFileSync", (exe, args) => {
      assert.equal(exe, process.execPath); assert.equal(args[0], npm);
      return Buffer.from(JSON.stringify(wrap(actualRecords[product])));
    });
    assert.throws(() => packing.packPackage(product, files, root, options, context), /integrity differs from actual pack/);
    changed.mock.restore();
  }
});

test("C1 packPackage rejects unknown products before invoking npm", t => {
  const packing = require("../scripts/stage-dual-authoring-npm");
  const run = t.mock.method(cp, "execFileSync", () => { throw new Error("unexpected tool"); });
  assert.throws(() => packing.packPackage("toString", {}, "", {}, {}), /unknown fixed npm product/);
  assert.equal(run.mock.callCount(), 0);
});
