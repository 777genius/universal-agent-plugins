"use strict";

const fs = require("node:fs");
const path = require("node:path");

// Schema v3 is an explicit projection of one sealed native pair. Historical
// consumers continue to require v2; consistency is never publication approval.
const c = require("./dual-authoring-candidate");
const candidateProducer = require("./stage-dual-authoring-candidate");
const AUTHORING_MODE = "release-cli-contract-v1";
const AUTHORING_SCOPE = "six-platform-pair";
const FALSE_CLAIMS = { release_eligible: false, platform_acceptance: false, attested: false };

function authoringOptions(options, preparing) {
  c.keys(options, ["candidate", "root", "identity", "manifestDigest", "go", "workParent",
    "assetScope", "authoringMode", "outputs", "pairMarker"], "authoring release options");
  c.identity(options.identity);
  c.keys(options.outputs, c.PRODUCTS, "product outputs");
  if (options.candidate !== true || options.authoringMode !== AUTHORING_MODE ||
      options.assetScope !== AUTHORING_SCOPE || options.identity.versions["plugin-kit-ai"] !== "2.0.0" ||
      /^0{40}$/.test(options.identity.commit)) throw new Error("explicit first-cut release pair required");
  if (typeof options.go !== "string" || !path.isAbsolute(options.go)) throw new Error("absolute trusted Go tool required");
  const protectedRoots = [options.root, options.workParent, path.dirname(options.go), path.resolve(__dirname, "../../..")];
  protectedRoots.forEach(c.safeDirectory);
  const roots = c.PRODUCTS.map((product) => options.outputs[product]);
  const marker = options.pairMarker;
  if (typeof marker !== "string" || !path.isAbsolute(marker) || path.resolve(marker) !== marker ||
      !/^[A-Za-z0-9][A-Za-z0-9._-]*$/.test(path.basename(marker))) throw new Error("unsafe pair marker path");
  c.safeDirectory(path.dirname(marker));
  // Case-fold too: these outputs must remain disjoint on every target filesystem.
  const overlaps = (a, b) => {
    a = a.toLowerCase(); b = b.toLowerCase();
    return a === b || a.startsWith(b.endsWith(path.sep) ? b : b + path.sep) ||
      b.startsWith(a.endsWith(path.sep) ? a : a + path.sep);
  };
  for (const root of roots) {
    if (preparing) c.outputPlacement(root, protectedRoots);
    else c.safeDirectory(root);
    if (protectedRoots.some((input) => overlaps(root, input)) || overlaps(root, marker)) {
      throw new Error("projection overlaps input, scratch, tool, repository or marker");
    }
  }
  if (overlaps(roots[0], roots[1]) || protectedRoots.some((root) => overlaps(marker, root))) {
    throw new Error("authoring outputs overlap");
  }
  if (preparing) {
    try { fs.lstatSync(marker); }
    catch (error) { if (error.code === "ENOENT") return; throw error; }
    throw new Error("pair marker already exists");
  }
}

function verifiedAuthoringCandidate(options) {
  candidateProducer.verifyCandidate(Object.fromEntries([
    "candidate", "root", "identity", "manifestDigest", "go", "workParent", "assetScope", "authoringMode"
  ].map((key) => [key, options[key]])));
  return c.frozenCandidate(options.root, options.identity, options.manifestDigest, AUTHORING_SCOPE, AUTHORING_MODE);
}

function productManifest(frozen, product) {
  const id = frozen.manifest.identity;
  return { schema_version: 3, status: "CANDIDATE", product, repository: id.repository,
    tag: product === "agentplugins" ? `agentplugins-v${id.versions[product]}` : `v${id.versions[product]}`,
    version: id.versions[product], commit: id.commit, engine_revision: id.engine_revision,
    versions: id.versions, candidate_sha256: frozen.manifest_sha256,
    authoring_mode: AUTHORING_MODE, asset_scope: AUTHORING_SCOPE,
    assets: frozen.manifest.products[product].assets, ...FALSE_CLAIMS };
}

function projectionChecksums(manifest, body) {
  return Buffer.from([...Object.values(manifest.assets).map((asset) => `${asset.sha256}  ${asset.file}`),
    `${c.digest(body)}  release-manifest.json`].join("\n") + "\n");
}

function checkProjections(options, frozen) {
  const products = {};
  for (const product of c.PRODUCTS) {
    const root = options.outputs[product];
    c.safeDirectory(root);
    const manifest = productManifest(frozen, product);
    const body = c.encode(manifest);
    if (!c.readFile(path.join(root, "release-manifest.json"), 1024 * 1024).equals(body) ||
        !c.readFile(path.join(root, "checksums.txt"), 64 * 1024).equals(projectionChecksums(manifest, body))) {
      throw new Error("projection manifest or checksums differ from pinned candidate");
    }
    for (const asset of Object.values(manifest.assets)) {
      const bytes = c.readFile(path.join(root, asset.file));
      if (bytes.length !== asset.size || c.digest(bytes) !== asset.sha256) throw new Error("projected asset corruption");
      // Same exact archive digest as the independently inspected frozen input,
      // including its inner binary identity; no rearchive or subject execution.
    }
    const expected = [...Object.values(manifest.assets).map((asset) => asset.file), "release-manifest.json", "checksums.txt"];
    if (fs.readdirSync(root).sort().join("\n") !== expected.sort().join("\n")) throw new Error("extra or missing projection files");
    products[product] = { manifest_sha256: c.digest(body), checksums_sha256: c.digest(projectionChecksums(manifest, body)) };
  }
  return { schema: "authoring-release-pair/v1", status: "CANDIDATE", identity: frozen.manifest.identity,
    candidate_sha256: frozen.manifest_sha256, authoring_mode: AUTHORING_MODE,
    asset_scope: AUTHORING_SCOPE, products, ...FALSE_CLAIMS };
}

function prepareAuthoringRelease(input) {
  const options = structuredClone(input);
  authoringOptions(options, true); // Fail before scratch or output side effects.
  const frozen = verifiedAuthoringCandidate(options);
  authoringOptions(options, true);
  // Reserve both absent roots before any copy. Partial bytes are retained, but
  // only the separate final pair marker signals complete preparation.
  for (const product of c.PRODUCTS) fs.mkdirSync(options.outputs[product], { mode: 0o700 });
  let markerOwned = false;
  try {
    for (const product of c.PRODUCTS) {
      const root = options.outputs[product];
      const manifest = productManifest(frozen, product);
      for (const asset of Object.values(manifest.assets)) {
        const bytes = c.readFile(path.join(options.root, asset.file));
        if (c.digest(bytes) !== asset.sha256 || bytes.length !== asset.size) throw new Error("candidate changed before projection");
        fs.writeFileSync(path.join(root, asset.file), bytes, { flag: "wx", mode: 0o444 });
      }
      const body = c.encode(manifest);
      fs.writeFileSync(path.join(root, "release-manifest.json"), body, { flag: "wx", mode: 0o444 });
      fs.writeFileSync(path.join(root, "checksums.txt"), projectionChecksums(manifest, body), { flag: "wx", mode: 0o444 });
    }
    const pair = checkProjections(options, frozen);
    const fd = fs.openSync(options.pairMarker, "wx", 0o444);
    markerOwned = true;
    try { fs.writeFileSync(fd, c.encode(pair)); }
    catch (error) {
      try { fs.closeSync(fd); } catch (closeError) { error.closeError = closeError; }
      throw error;
    }
    fs.closeSync(fd);
    return pair;
  } catch (error) {
    if (markerOwned) {
      try { fs.unlinkSync(options.pairMarker); }
      catch (cleanupError) { throw new AggregateError([error, cleanupError], "pair marker cleanup failed", { cause: error }); }
    }
    throw error;
  }
}

function verifyAuthoringRelease(input) {
  const options = structuredClone(input);
  authoringOptions(options, false);
  const frozen = verifiedAuthoringCandidate(options);
  const pair = checkProjections(options, frozen);
  if (!c.readFile(options.pairMarker, 1024 * 1024).equals(c.encode(pair))) throw new Error("pair success marker mismatch");
  return { ...pair, consistency_verified: true };
}

module.exports = { prepareAuthoringRelease, verifyAuthoringRelease };
