"use strict";

const fs = require("node:fs");
const path = require("node:path");
const { isDeepStrictEqual: equal } = require("node:util");
const fail = message => { throw new Error(message); };
const exact = (a, b, label) => { if (!equal(a, b)) fail(`${label}: binding mismatch`); };

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

function verifyProjectedPair(root, pins) {
  c.keys(pins, ["identity", "candidate_sha256", "pair_marker_sha256", "products"], "projected pair pins");
  c.identity(pins.identity);
  if (/^0{40}$/.test(pins.identity.commit) || pins.identity.versions["plugin-kit-ai"] !== "2.0.0") fail("first-cut identity required");
  c.keys(pins.products, c.PRODUCTS, "projection pins");
  const record = pins;
  const seen = new Set();
  c.safeDirectory(root);
  const candidateFile = path.join(root, "candidate", "candidate.json");
  const body = c.readFile(candidateFile, 1024 * 1024);
  exact(c.digest(body), record.candidate_sha256, "candidate digest");
  const manifest = JSON.parse(body);
  if (!body.equals(c.encode(manifest))) fail("noncanonical candidate");
  c.manifestShape(manifest, record.identity, AUTHORING_SCOPE, AUTHORING_MODE);
  const result = [{ file: candidateFile, sha256: record.candidate_sha256 }];
  const products = {};
  for (const p of c.PRODUCTS) {
    const projection = path.join(root, p); c.safeDirectory(projection);
    c.keys(pins.products[p], ["manifest_sha256", "checksums_sha256"], "product projection pins");
    const m = { schema_version: 3, status: "CANDIDATE", product: p, repository: c.REPOSITORY,
      tag: productManifest({ manifest, manifest_sha256: pins.candidate_sha256 }, p).tag, version: record.identity.versions[p], commit: record.identity.commit,
      engine_revision: record.identity.commit, versions: record.identity.versions, candidate_sha256: record.candidate_sha256,
      authoring_mode: AUTHORING_MODE, asset_scope: AUTHORING_SCOPE, assets: manifest.products[p].assets,
      release_eligible: false, platform_acceptance: false, attested: false };
    const mBody = c.encode(m);
    const checks = Buffer.from([...Object.values(m.assets).map(a => `${a.sha256}  ${a.file}`), `${c.digest(mBody)}  release-manifest.json`].join("\n") + "\n");
    for (const [name, bytes, hash] of [["release-manifest.json", mBody, record.products[p].manifest_sha256], ["checksums.txt", checks, record.products[p].checksums_sha256]]) {
      const file = path.join(projection, name);
      exact(c.readFile(file, 1024 * 1024), bytes, "projection bytes"); exact(c.digest(bytes), hash, "independent projection pin");
      result.push({ file, sha256: hash });
    }
    for (const a of Object.values(m.assets)) {
      const file = path.join(projection, a.file); const bytes = c.readFile(file);
      exact(c.metadata(bytes), { sha256: a.sha256, size: a.size }, "asset bytes");
      const binary = p === "plugin-kit-ai" ? c.unpack(bytes, a.binary.file) : bytes;
      exact(c.metadata(binary), { sha256: a.binary.sha256, size: a.binary.size }, "inner binary bytes");
      if (seen.has(a.binary.sha256)) fail("duplicate product/target binary");
      seen.add(a.binary.sha256);
      result.push({ file, sha256: a.sha256 });
    }
    exact(fs.readdirSync(projection).sort(), [...Object.values(m.assets).map(a => a.file), "release-manifest.json", "checksums.txt"].sort(), "projection closure");
    products[p] = { manifest_sha256: c.digest(mBody), checksums_sha256: c.digest(checks) };
  }
  const marker = { schema: "authoring-release-pair/v1", status: "CANDIDATE", identity: record.identity,
    candidate_sha256: record.candidate_sha256, authoring_mode: AUTHORING_MODE, asset_scope: AUTHORING_SCOPE, products,
    release_eligible: false, platform_acceptance: false, attested: false };
  const markerFile = path.join(root, "pair-prepared.json");
  exact(c.readFile(markerFile, 1024 * 1024), c.encode(marker), "pair marker");
  exact(c.digest(c.encode(marker)), record.pair_marker_sha256, "pair marker pin");
  result.push({ file: markerFile, sha256: record.pair_marker_sha256 });
  return { manifest, subjects: result }; // Eighteen unchanged frozen inputs.
}

module.exports = { prepareAuthoringRelease, verifyAuthoringRelease, verifyProjectedPair };
