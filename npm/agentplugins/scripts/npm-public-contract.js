#!/usr/bin/env node
"use strict";

const crypto = require("node:crypto");
const fs = require("node:fs");
const path = require("node:path");
const { isDeepStrictEqual } = require("node:util");

const PACKAGE_NAME = "universal-agent-plugins";
const UAP_REPOSITORY = "https://github.com/777genius/universal-agent-plugins";
const UAP_WORKFLOW = ".github/workflows/agentplugins-npm-publish.yml";
const SLSA_PREDICATE = "https://slsa.dev/provenance/v1";
const VERSION = /^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$/;
const INTEGRITY = /^sha512-([A-Za-z0-9+/]{86}==)$/;
const SHASUM = /^[0-9a-f]{40}$/;
const COMMIT = /^[0-9a-f]{40}$/;

function fail(message) {
  throw new Error(message);
}

function exactObject(actual, expected, label) {
  if (!isDeepStrictEqual(actual, expected)) {
    fail(`${label} does not match the package contract`);
  }
}

function exactKeys(actual, expected, label) {
  if (!actual || typeof actual !== "object" || Array.isArray(actual) ||
      !isDeepStrictEqual(Object.keys(actual).sort(), expected.slice().sort())) {
    fail(`${label} does not have the exact package contract fields`);
  }
}

function validateExpected(version, integrity, shasum) {
  if (!VERSION.test(version)) fail("version must be an exact stable semantic version");
  const match = integrity.match(INTEGRITY);
  if (!match || Buffer.from(match[1], "base64").length !== 64) {
    fail("integrity must be an exact SHA-512 SRI value");
  }
  if (!SHASUM.test(shasum)) fail("shasum must be an exact lowercase SHA-1 value");
}

function validatePackJSON(value, version) {
  let record;
  if (Array.isArray(value)) {
    if (value.length !== 1) fail("npm pack JSON must contain exactly one record");
    [record] = value;
  } else if (value && typeof value === "object") {
    const keys = Object.keys(value);
    if (keys.length !== 1 || keys[0] !== PACKAGE_NAME) {
      fail("npm pack JSON must contain exactly one package-named record");
    }
    record = value[PACKAGE_NAME];
  } else {
    fail("npm pack JSON must contain exactly one record");
  }
  if (!record || record.name !== PACKAGE_NAME || record.version !== version ||
      record.filename !== `${PACKAGE_NAME}-${version}.tgz`) {
    fail("npm pack JSON package identity does not match the release");
  }
  validateExpected(version, record.integrity, record.shasum);
  return record;
}

function validatePublicMetadata(metadata, version, integrity, shasum) {
  validateExpected(version, integrity, shasum);
  if (Array.isArray(metadata)) {
    if (metadata.length !== 1) {
      fail("public npm metadata must contain exactly one release record");
    }
    [metadata] = metadata;
  }
  if (!metadata || metadata.name !== PACKAGE_NAME || metadata.version !== version) {
    fail("public npm name/version does not match the package contract");
  }
  exactObject(metadata.repository, {
    type: "git",
    url: "git+https://github.com/777genius/universal-agent-plugins.git",
    directory: "npm/agentplugins"
  }, "public npm repository");
  if (metadata.homepage !== "https://777genius.github.io/universal-agent-plugins/") {
    fail("public npm homepage does not match the UAP Pages contract");
  }
  exactObject(metadata.engines, { node: ">=22" }, "public npm engines");
  exactObject(metadata.bin, { agentplugins: "bin/agentplugins.js" }, "public npm bin");
  exactObject(metadata.scripts, { test: "node --test" }, "public npm scripts/lifecycle hooks");

  const expectedTarball = `https://registry.npmjs.org/${PACKAGE_NAME}/-/${PACKAGE_NAME}-${version}.tgz`;
  const expectedAttestation = `https://registry.npmjs.org/-/npm/v1/attestations/${PACKAGE_NAME}@${version}`;
  if (!metadata.dist || metadata.dist.tarball !== expectedTarball ||
      metadata.dist.integrity !== integrity || metadata.dist.shasum !== shasum) {
    fail("public npm dist identity does not match the staged tarball");
  }
  exactObject(metadata.dist.attestations, {
    url: expectedAttestation,
    provenance: { predicateType: "https://slsa.dev/provenance/v1" }
  }, "public npm attestation URL and provenance predicate");
  if (typeof metadata._npmUser === "string") {
    // The public npm registry currently flattens the publisher object to this
    // exact display string. OIDC/workflow identity is verified by the signed
    // provenance checks below, not by fields the registry does not expose.
    if (metadata._npmUser !== "GitHub Actions <npm-oidc-no-reply@github.com>") {
      fail("public npm publisher identity is not GitHub Actions");
    }
  } else {
    exactKeys(metadata._npmUser, ["name", "email", "trustedPublisher"], "public npm publisher identity");
    if (metadata._npmUser.name !== "GitHub Actions" ||
        metadata._npmUser.email !== "npm-oidc-no-reply@github.com") {
      fail("public npm publisher identity is not GitHub Actions");
    }
    const publisher = metadata._npmUser?.trustedPublisher;
    exactKeys(publisher, ["id", "oidcConfigId"], "public npm trusted publisher");
    if (publisher.id !== "github" ||
        !/^oidc:[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/.test(publisher.oidcConfigId)) {
      fail("public npm trusted publisher is not an exact GitHub Actions OIDC identity");
    }
  }
  return metadata;
}

function decodeBase64JSON(encoded, label) {
  if (typeof encoded !== "string" || !/^[A-Za-z0-9+/]+={0,2}$/.test(encoded) || encoded.length % 4 !== 0) {
    fail(`${label} is not canonical base64`);
  }
  const decoded = Buffer.from(encoded, "base64");
  if (decoded.toString("base64") !== encoded) fail(`${label} is not canonical base64`);
  try {
    return JSON.parse(decoded.toString("utf8"));
  } catch (error) {
    fail(`${label} is not JSON: ${error.message}`);
  }
}

function validateSLSAAttestation(response, version, integrity, uapTag, uapCommit) {
  validateExpected(version, integrity, "0".repeat(40));
  if (uapTag !== `agentplugins-v${version}`) fail("UAP tag does not match the package version");
  if (!COMMIT.test(uapCommit)) fail("UAP commit must be an exact lowercase commit");
  if (!response || !Array.isArray(response.attestations)) fail("npm attestation response is invalid");
  const provenance = response.attestations.filter((item) => item?.predicateType === SLSA_PREDICATE);
  if (provenance.length !== 1) fail("npm response must contain exactly one SLSA provenance attestation");
  const attestation = provenance[0];
  if (attestation.bundle?.mediaType !== "application/vnd.dev.sigstore.bundle.v0.3+json") {
    fail("npm SLSA provenance is not a Sigstore bundle v0.3");
  }
  const envelope = attestation.bundle.dsseEnvelope;
  if (!envelope || envelope.payloadType !== "application/vnd.in-toto+json" ||
      !Array.isArray(envelope.signatures) || envelope.signatures.length !== 1 ||
      typeof envelope.signatures[0]?.sig !== "string" || envelope.signatures[0].sig.length === 0) {
    fail("npm SLSA provenance does not contain one signed in-toto DSSE payload");
  }
  const statement = decodeBase64JSON(envelope.payload, "npm SLSA DSSE payload");
  const integrityBytes = Buffer.from(integrity.match(INTEGRITY)[1], "base64");
  exactObject(statement.subject, [{
    name: `pkg:npm/${PACKAGE_NAME}@${version}`,
    digest: { sha512: integrityBytes.toString("hex") }
  }], "npm SLSA subject and staged SHA-512");
  if (statement._type !== "https://in-toto.io/Statement/v1" ||
      statement.predicateType !== SLSA_PREDICATE) {
    fail("npm SLSA statement type or predicate does not match the contract");
  }
  const build = statement.predicate?.buildDefinition;
  if (build?.buildType !== "https://slsa-framework.github.io/github-actions-buildtypes/workflow/v1") {
    fail("npm SLSA provenance is not a GitHub Actions workflow build");
  }
  exactObject(build.externalParameters?.workflow, {
    ref: `refs/tags/${uapTag}`,
    repository: UAP_REPOSITORY,
    path: UAP_WORKFLOW
  }, "npm SLSA GitHub Actions workflow identity");
  exactObject(build.resolvedDependencies, [{
    uri: `git+${UAP_REPOSITORY}@refs/tags/${uapTag}`,
    digest: { gitCommit: uapCommit }
  }], "npm SLSA UAP tag and commit dependency");
  if (statement.predicate?.runDetails?.builder?.id !==
      "https://github.com/actions/runner/github-hosted") {
    fail("npm SLSA publisher did not use a GitHub-hosted Actions runner");
  }
  const invocation = statement.predicate?.runDetails?.metadata?.invocationId;
  if (typeof invocation !== "string" ||
      !invocation.startsWith(`${UAP_REPOSITORY}/actions/runs/`)) {
    fail("npm SLSA invocation is not an exact UAP GitHub Actions run");
  }
  return statement;
}

function validateAuditSignatures(audit, response, version) {
  if (!VERSION.test(version) || !audit || !Array.isArray(audit.invalid) || audit.invalid.length !== 0 ||
      !Array.isArray(audit.missing) || audit.missing.length !== 0 ||
      !Array.isArray(audit.verified) || audit.verified.length !== 1) {
    fail("npm audit signatures did not cryptographically verify exactly one package");
  }
  const verified = audit.verified[0];
  const expectedAttestation = `https://registry.npmjs.org/-/npm/v1/attestations/${PACKAGE_NAME}@${version}`;
  if (verified.name !== PACKAGE_NAME || verified.version !== version ||
      verified.location !== `node_modules/${PACKAGE_NAME}` ||
      verified.registry !== "https://registry.npmjs.org/") {
    fail("npm audit signatures package identity does not match the public release");
  }
  exactObject(verified.attestations, {
    url: expectedAttestation,
    provenance: { predicateType: SLSA_PREDICATE }
  }, "npm audit signatures attestation identity");
  exactObject(verified.attestationBundles, response?.attestations,
    "npm audit signatures cryptographically verified bundles");
  return verified;
}

function validateDownloadedTarball(packJSON, tarballRoot, version, integrity, shasum) {
  validateExpected(version, integrity, shasum);
  const record = validatePackJSON(packJSON, version);
  if (record.integrity !== integrity || record.shasum !== shasum) {
    fail("downloaded public npm pack identity does not match the staged tarball");
  }
  const root = path.resolve(tarballRoot);
  const tarball = path.join(root, record.filename);
  const stat = fs.lstatSync(tarball);
  if (!stat.isFile() || stat.isSymbolicLink()) fail("downloaded npm tarball is not a regular file");
  const body = fs.readFileSync(tarball);
  const actualIntegrity = `sha512-${crypto.createHash("sha512").update(body).digest("base64")}`;
  const actualShasum = crypto.createHash("sha1").update(body).digest("hex");
  if (actualIntegrity !== integrity || actualShasum !== shasum) {
    fail("downloaded public npm tarball bytes do not match the staged integrity and shasum");
  }
  return record;
}

function readJSON(filename) {
  return JSON.parse(fs.readFileSync(filename, "utf8"));
}

function main() {
  const [command, ...args] = process.argv.slice(2);
  if (command === "stage-outputs" && args.length === 3) {
    const [packFile, outputFile, version] = args;
    const record = validatePackJSON(readJSON(packFile), version);
    fs.appendFileSync(outputFile, `tarball_integrity=${record.integrity}\ntarball_shasum=${record.shasum}\n`);
  } else if (command === "metadata" && args.length === 4) {
    validatePublicMetadata(readJSON(args[0]), args[1], args[2], args[3]);
  } else if (command === "attestation" && args.length === 5) {
    validateSLSAAttestation(readJSON(args[0]), args[1], args[2], args[3], args[4]);
  } else if (command === "audit" && args.length === 3) {
    validateAuditSignatures(readJSON(args[0]), readJSON(args[1]), args[2]);
  } else if (command === "download" && args.length === 5) {
    validateDownloadedTarball(readJSON(args[0]), args[1], args[2], args[3], args[4]);
  } else {
    fail("usage: npm-public-contract.js stage-outputs <pack-json> <github-output> <version> | metadata <metadata-json> <version> <integrity> <shasum> | attestation <attestation-json> <version> <integrity> <uap-tag> <uap-commit> | audit <npm-audit-json> <attestation-json> <version> | download <pack-json> <tarball-root> <version> <integrity> <shasum>");
  }
}

if (require.main === module) {
  try {
    main();
  } catch (error) {
    process.stderr.write(`npm public contract: ${error.message}\n`);
    process.exitCode = 1;
  }
}

module.exports = {
  validateDownloadedTarball,
  validateAuditSignatures,
  validatePackJSON,
  validatePublicMetadata,
  validateSLSAAttestation
};
