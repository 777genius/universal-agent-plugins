"use strict";

const assert = require("node:assert/strict");
const crypto = require("node:crypto");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const test = require("node:test");

const {
  validateAuditSignatures,
  validateDownloadedTarball,
  validatePackJSON,
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
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
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
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
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
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
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
