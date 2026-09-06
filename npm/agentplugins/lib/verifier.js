"use strict";

// Metadata-neutral primitives. Release selection stays in each explicit facade.
const crypto = require("node:crypto");
const fs = require("node:fs");
const fsp = require("node:fs/promises");
const https = require("node:https");
const path = require("node:path");
const MAX_REDIRECTS = 5;
const DOWNLOAD_TIMEOUT_MS = 30_000;
const LOCK_TIMEOUT_MS = 30_000;

async function sha256File(file) {
  const hash = crypto.createHash("sha256");
  const stream = fs.createReadStream(file);
  for await (const chunk of stream) {
    hash.update(chunk);
  }
  return hash.digest("hex");
}

async function validCachedBinary(file, expectedHash) {
  try {
    const stat = await fsp.lstat(file);
    if (!stat.isFile() || stat.isSymbolicLink()) {
      return false;
    }
    return (await sha256File(file)) === expectedHash;
  } catch (error) {
    if (error && error.code === "ENOENT") {
      return false;
    }
    throw error;
  }
}

function validateDownloadURL(value) {
  const parsed = new URL(value);
  if (parsed.protocol !== "https:" || parsed.port) {
    throw new Error("binary download and every redirect must use an approved GitHub HTTPS host");
  }
  if (parsed.username || parsed.password) {
    throw new Error("binary download URL cannot contain credentials");
  }
  const pathAndQuery = `${parsed.pathname}${parsed.search}`;
  switch (parsed.hostname) {
    case "github.com":
      return { hostname: "github.com", path: pathAndQuery, url: parsed };
    case "release-assets.githubusercontent.com":
      return { hostname: "release-assets.githubusercontent.com", path: pathAndQuery, url: parsed };
    default:
      throw new Error("binary download and every redirect must use an approved GitHub HTTPS host");
  }
}

function requestApprovedTarget(target, requestOptions) {
  const options = {
    ...requestOptions,
    method: "GET",
    path: target.path,
    port: 443,
    protocol: "https:"
  };
  switch (target.hostname) {
    case "github.com":
      return https.get({ ...options, hostname: "github.com" });
    case "release-assets.githubusercontent.com":
      return https.get({ ...options, hostname: "release-assets.githubusercontent.com" });
    default:
      throw new Error("binary download host was not validated");
  }
}

async function downloadFile(value, destination, expected, options = {}, redirects = MAX_REDIRECTS) {
  const target = validateDownloadURL(value);
  await new Promise((resolve, reject) => {
    const requestOptions = {
      headers: {
        Accept: "application/octet-stream",
        "User-Agent": "agentplugins-npm-bootstrap"
      }
    };
    const request = typeof options.request === "function"
      ? options.request(target.url, requestOptions)
      : requestApprovedTarget(target, requestOptions);
    request.setTimeout(DOWNLOAD_TIMEOUT_MS, () => request.destroy(new Error("binary download timed out")));
    let streamFailure;
    request.once("error", error => streamFailure ? streamFailure(error) : reject(error));
    request.once("response", (response) => {
      if ([301, 302, 303, 307, 308].includes(response.statusCode) && response.headers.location) {
        response.resume();
        if (redirects <= 0) {
          reject(new Error("too many binary download redirects"));
          return;
        }
        try {
          const next = new URL(response.headers.location, target.url).toString();
          downloadFile(next, destination, expected, options, redirects - 1).then(resolve, reject);
        } catch (error) { reject(error); }
        return;
      }
      if (response.statusCode < 200 || response.statusCode > 299) {
        response.resume();
        reject(new Error(`binary download failed with HTTP ${response.statusCode}`));
        return;
      }
      const length = response.headers["content-length"];
      if (length !== undefined && (!/^[1-9][0-9]*$/.test(String(length)) || Number(length) !== expected.size)) {
        response.resume();
        reject(new Error("binary download size does not match embedded metadata"));
        return;
      }
      const output = fs.createWriteStream(destination, { flags: "wx", mode: 0o600 });
      const hash = crypto.createHash("sha256");
      let size = 0;
      let settled = false;
      let verified = false;
      let pendingError;
      const fail = (error) => {
        if (settled || pendingError) return;
        pendingError = error;
        response.destroy();
        output.destroy();
      };
      streamFailure = fail;
      let ended = false;
      response.once("end", () => { ended = true; });
      response.once("close", () => { if (!ended) fail(new Error("binary download response closed before completion")); });
      output.once("error", fail);
      response.once("error", fail);
      response.on("data", (chunk) => {
        size += chunk.length;
        if (size > expected.size) {
          fail(new Error("binary download exceeded embedded size"));
          return;
        }
        hash.update(chunk);
      });
      response.pipe(output);
      output.once("finish", () => {
        if (settled || pendingError) return;
        if (size !== expected.size || hash.digest("hex") !== expected.sha256) {
          fail(new Error("binary download failed embedded SHA-256 verification"));
          return;
        }
        verified = true;
      });
      output.once("close", () => {
        if (settled) return;
        settled = true;
        if (pendingError) {
          reject(pendingError);
          return;
        }
        if (!verified) {
          reject(new Error("binary download closed before verification completed"));
          return;
        }
        resolve();
      });
    });
  });
}

function cancelled(signal) {
  if (signal && signal.aborted) throw new Error("binary acquisition cancelled");
}

function delay(milliseconds, signal) {
  return new Promise((resolve, reject) => {
    const abort = () => { clearTimeout(timer); signal?.removeEventListener("abort", abort); reject(new Error("binary acquisition cancelled")); };
    const timer = setTimeout(() => { signal?.removeEventListener("abort", abort); resolve(); }, milliseconds);
    signal?.addEventListener("abort", abort, { once: true });
    if (signal?.aborted) abort();
  });
}

const sameFile = (a, b) => a.dev === b.dev && a.ino === b.ino;
const uid = () => typeof process.geteuid === "function" ? process.geteuid() : null;
function ownedFile(stat) {
  if (!stat.isFile() || stat.nlink !== 1 || (uid() !== null && stat.uid !== uid()) ||
      (process.platform !== "win32" && (stat.mode & 0o7022))) {
    throw new Error("unsafe binary cache file; refusing to move or replace it");
  }
}

// Every component is real. Shared ancestors may be root-owned; a writable
// ancestor requires sticky protection. The root and all descendants are 0700.
async function privateDirectory(root, directory = root, io = fsp) {
  if (typeof root !== "string" || !path.isAbsolute(root) || path.resolve(root) !== root ||
      root.includes("\0") || root.split(/[\\/]/).includes("..") ||
      !(directory === root || directory.startsWith(root + path.sep))) throw new Error("unsafe private cache root");
  let current = path.parse(directory).root;
  for (const part of directory.slice(current.length).split(path.sep).filter(Boolean)) {
    current = path.join(current, part);
    const below = current !== root && current.startsWith(root + path.sep);
    if (below) {
      try { await io.mkdir(current, { mode: 0o700 }); }
      catch (error) { if (error.code !== "EEXIST") throw error; }
    }
    const stat = await io.lstat(current);
    const privatePart = below || current === root;
    if (!stat.isDirectory() || stat.isSymbolicLink() ||
        (uid() !== null && stat.uid !== uid() && (privatePart || stat.uid !== 0)) ||
        (process.platform !== "win32" && (privatePart ? (stat.mode & 0o7777) !== 0o700 :
          ((stat.mode & 0o022) !== 0 && (stat.mode & 0o1000) === 0)))) {
      throw new Error("private cache requires owned mode-0700 real directories and safe ancestors");
    }
  }
}

async function strictCachedBinary(file, pin, options = {}) {
  const io = options.io || fsp;
  let before;
  try { before = await io.lstat(file); }
  catch (error) { if (error.code === "ENOENT") return false; throw error; }
  ownedFile(before);
  if (before.size !== pin.size) return false;
  if (options.osName !== "windows" && (before.mode & 0o777) !== 0o755) return false;
  const handle = await io.open(file, fs.constants.O_RDONLY | (fs.constants.O_NOFOLLOW || 0));
  try {
    const opened = await handle.stat();
    ownedFile(opened);
    if (!sameFile(before, opened)) throw new Error("binary cache file changed while opening");
    const hash = crypto.createHash("sha256");
    const buffer = Buffer.alloc(Math.min(64 * 1024, pin.size));
    let size = 0;
    while (size <= pin.size) {
      cancelled(options.signal);
      const { bytesRead } = await handle.read(buffer, 0, Math.min(buffer.length, pin.size + 1 - size), null);
      if (!bytesRead) break;
      size += bytesRead;
      hash.update(buffer.subarray(0, bytesRead));
    }
    const after = await handle.stat();
    const named = await io.lstat(file);
    ownedFile(after); ownedFile(named);
    if (!sameFile(before, named) || after.size !== before.size || after.mode !== before.mode ||
        after.mtimeMs !== before.mtimeMs || after.ctimeMs !== before.ctimeMs) throw new Error("binary cache file changed while verifying");
    return size === pin.size && hash.digest("hex") === pin.sha256;
  } finally { await handle.close(); }
}

async function removeOwned(file, identity, io) {
  let named;
  try { named = await io.lstat(file); }
  catch (error) { if (error.code === "ENOENT") return; throw error; }
  if (!sameFile(named, identity) || !named.isFile() || named.nlink !== 1) {
    throw new Error("owned cleanup identity changed; preserved named object");
  }
  await io.unlink(file);
}

function failures(primary, cleanup) {
  if (!cleanup.length) return primary;
  return new Error(`${primary ? primary.message + "; " : ""}cleanup/rollback uncertainty: ${cleanup.map(e => e.message).join("; ")}`);
}

async function acquireLock(target, options = {}) {
  const io = options.io || fsp;
  const { lockRoot, signal } = options;
  const timeoutMs = options.timeoutMs ?? LOCK_TIMEOUT_MS;
  const pollMs = options.pollMs ?? 50;
  if (!Number.isFinite(timeoutMs) || timeoutMs < 0 || timeoutMs > 300_000 ||
      !Number.isFinite(pollMs) || pollMs <= 0 || pollMs > 1000) throw new Error("invalid finite lock wait");
  cancelled(signal);
  await io.mkdir(lockRoot, { recursive: true, mode: 0o700 });
  const rootStat = await io.lstat(lockRoot);
  if (!rootStat.isDirectory() || rootStat.isSymbolicLink() ||
      (uid() !== null && rootStat.uid !== uid()) ||
      (process.platform !== "win32" && (rootStat.mode & 0o777) !== 0o700)) {
    throw new Error("agentplugins cache lock root must be a user-owned mode-0700 real directory");
  }
  const lockPath = path.join(lockRoot, crypto.createHash("sha256").update(target).digest("hex") + ".lock");
  const started = Date.now();
  while (true) {
    cancelled(signal);
    let handle;
    try { handle = await io.open(lockPath, "wx", 0o600); }
    catch (error) {
      if (error.code !== "EEXIST") throw error;
      if (Date.now() - started >= timeoutMs) {
        throw new Error(`timed out waiting for the agentplugins binary cache lock at ${lockPath}; remove it only after confirming no agentplugins or npm process is running`);
      }
      await delay(Math.min(pollMs, Math.max(1, timeoutMs - (Date.now() - started))), signal);
      continue;
    }
    let identity, released = false;
    const release = async () => {
      if (released) return;
      released = true;
      const errors = [];
      // Inode identity is held open through comparison/removal, preventing reuse.
      if (identity) await removeOwned(lockPath, identity, io).catch(e => errors.push(e));
      await handle.close().catch(e => errors.push(e));
      if (errors.length) throw failures(null, errors);
    };
    try {
      identity = await handle.stat();
      await handle.writeFile(JSON.stringify({ pid: process.pid, nonce: crypto.randomBytes(16).toString("hex") }) + "\n");
      cancelled(signal);
      return release;
    } catch (error) {
      const errors = [];
      if (!identity) errors.push(new Error("lock identity unavailable; retained lock"));
      await release().catch(e => errors.push(e));
      throw failures(error, errors);
    }
  }
}

// Caller holding the target lock uses commitVerifiedBinary directly. Both
// facades use this same stage/quarantine/publish algorithm; strict validation is
// private-only. Two renames have an absence interval, not crash durability.
async function commitVerifiedBinary(input, binaryPath, pin, options = {}) {
  const io = options.io || fsp;
  const valid = file => options.strict ? strictCachedBinary(file, pin, options) : validCachedBinary(file, pin.sha256);
  cancelled(options.signal);
  if (await valid(binaryPath)) return binaryPath;
  let previous;
  try {
    previous = await io.lstat(binaryPath);
    if (options.strict) ownedFile(previous);
    else if (!previous.isFile() || previous.isSymbolicLink()) {
      throw new Error("binary cache target exists but is not a regular file; refusing to move or replace it");
    }
  } catch (error) { if (error.code !== "ENOENT") throw error; }
  const parent = path.dirname(binaryPath);
  await io.mkdir(parent, { recursive: true, mode: 0o700 });
  const suffix = `${process.pid}-${crypto.randomBytes(6).toString("hex")}`;
  const staging = path.join(parent, `.agentplugins-staging-${suffix}`);
  const quarantine = path.join(parent, `.agentplugins-replaced-${suffix}`);
  let staged, replaced = false, published = false, primary;
  const errors = [];
  try {
    // Reserve before writing/copying so partial-write cleanup has an identity.
    const handle = await io.open(staging, "wx", 0o600);
    try {
      staged = await handle.stat();
      if (Buffer.isBuffer(input)) await handle.writeFile(input);
    } catch (error) {
      if (!staged) errors.push(new Error("staging identity unavailable; retained staging file"));
      throw error;
    } finally { await handle.close().catch(e => { errors.push(e); }); }
    if (errors.length) throw new Error("staging close failed");
    if (!Buffer.isBuffer(input)) await io.copyFile(input, staging);
    if (options.osName !== "windows") await io.chmod(staging, 0o755);
    if (!(await valid(staging))) throw new Error("staged binary failed repeated SHA-256 verification");
    cancelled(options.signal);
    if (previous) {
      // Never overwrite a colliding quarantine or a changed target.
      try { await io.lstat(quarantine); throw new Error("quarantine collision"); }
      catch (error) { if (error.code !== "ENOENT") throw error; }
      const named = await io.lstat(binaryPath);
      if (!sameFile(named, previous)) throw new Error("binary cache target changed before commit");
      await io.rename(binaryPath, quarantine);
      replaced = true;
    }
    try { await io.lstat(binaryPath); throw new Error("binary cache target appeared before publication"); }
    catch (error) { if (error.code !== "ENOENT") throw error; }
    await io.rename(staging, binaryPath);
    published = true;
    if (!(await valid(binaryPath))) throw new Error("committed binary failed repeated SHA-256 verification");
    cancelled(options.signal);
  } catch (error) { primary = error; }
  if (primary && published) {
    await removeOwned(binaryPath, staged, io).then(() => { published = false; }).catch(e => errors.push(e));
  }
  if (primary && replaced && !published) {
    try {
      let absent = false;
      try { await io.lstat(binaryPath); } catch (e) { if (e.code !== "ENOENT") throw e; absent = true; }
      if (!absent) throw new Error("rollback target occupied; retained quarantine");
      const named = await io.lstat(quarantine);
      if (!sameFile(named, previous)) throw new Error("rollback identity changed; retained quarantine");
      await io.rename(quarantine, binaryPath);
      replaced = false;
    } catch (e) { errors.push(e); }
  }
  if (!primary && replaced) await removeOwned(quarantine, previous, io).catch(e => errors.push(e));
  if (staged && !published) await removeOwned(staging, staged, io).catch(e => errors.push(e));
  if (primary || errors.length) throw failures(primary, errors);
  return binaryPath;
}

async function installVerifiedBinary(input, binaryPath, pin, options = {}) {
  const unlock = await acquireLock(binaryPath, options);
  let result, primary;
  try { result = await commitVerifiedBinary(input, binaryPath, pin, options); }
  catch (error) { primary = error; }
  const errors = [];
  await unlock().catch(e => errors.push(e));
  if (primary || errors.length) throw failures(primary, errors);
  return result;
}

module.exports = {
  sha256File, validCachedBinary, downloadFile, acquireLock, installVerifiedBinary,
  commitVerifiedBinary, strictCachedBinary, privateDirectory, cancelled, failures
};
