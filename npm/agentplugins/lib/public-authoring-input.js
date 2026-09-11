"use strict";

// Bounded observations in an owned, quiescent namespace. These path checks do
// not exclude hostile same-UID writers, mount replacement or every host race.
const fs = require("node:fs");
const path = require("node:path");
const crypto = require("node:crypto");
const digest = bytes => crypto.createHash("sha256").update(bytes).digest("hex");
const fail = message => { throw new Error(`public snapshot: ${message}`); };
const directoryKeys = ["dev", "ino", "mode", "uid", "gid"];
const fileKeys = [...directoryKeys, "size", "nlink", "mtimeMs", "ctimeMs"];
const same = (a, b, keys) => keys.every(key => a[key] === b[key]);
const within = (a, b) => { const r = path.relative(b, a); return r === "" || (!r.startsWith(`..${path.sep}`) && r !== ".." && !path.isAbsolute(r)); };
const overlaps = (a, b) => within(a, b) || within(b, a);

function canonical(file) {
  if (typeof file !== "string" || !file || file.includes("\0") || !path.isAbsolute(file) ||
      path.resolve(file) !== file || file.split(/[\\/]/).some(p => p === "." || p === "..")) fail("canonical absolute path required");
  return file;
}
function cancelled(signal) {
  if (signal && signal.aborted) { const error = new Error("public snapshot cancelled"); error.code = "ABORT_ERR"; throw error; }
}
function uid() {
  if (typeof process.geteuid !== "function" || !fs.constants.O_NOFOLLOW) fail("host ownership/no-follow support required");
  return process.geteuid();
}
function directory(name, owner) {
  const stat = fs.lstatSync(name);
  if (!stat.isDirectory() || stat.isSymbolicLink() || fs.realpathSync(name) !== name ||
      (stat.uid !== 0 && stat.uid !== owner) || ((stat.mode & 0o022) && !(stat.mode & 0o1000))) fail("unsafe ancestor");
  return stat;
}
function ancestors(name, owner, missing = false) {
  canonical(name);
  let current = path.parse(name).root;
  const pins = [[current, directory(current, owner)]];
  for (const part of name.slice(current.length).split(path.sep).filter(Boolean)) {
    current = path.join(current, part);
    try { pins.push([current, directory(current, owner)]); }
    catch (error) { if (missing && error.code === "ENOENT") break; throw error; }
  }
  return pins;
}
function recheckDirectories(pins, owner) {
  for (const [name, pin] of pins) if (!same(directory(name, owner), pin, directoryKeys)) fail("ancestor changed");
}

// Validate the entire configured cache boundary without creating it. Existing
// ancestor aliases are rejected even if the final cache directory is absent.
function publicPlacement(packageRoot, cacheRoot) {
  canonical(packageRoot); canonical(cacheRoot);
  if (overlaps(packageRoot, cacheRoot)) fail("cache overlaps package");
  const owner = uid(), packages = ancestors(packageRoot, owner), cache = ancestors(cacheRoot, owner, true);
  const p = packages[packages.length - 1], q = cache[cache.length - 1];
  if (q[0] === cacheRoot && p[1].dev === q[1].dev && p[1].ino === q[1].ino) fail("cache aliases package");
  return { recheck() { recheckDirectories(packages, owner); recheckDirectories(cache, owner); } };
}

function snapshotPublicFile(file, { kind, maximum, exactSize, protectedRoots = [], signal } = {}) {
  cancelled(signal); canonical(file);
  if (!["metadata", "local asset"].includes(kind) || !Number.isSafeInteger(maximum) || maximum <= 0 ||
      maximum > (kind === "metadata" ? 1024 * 1024 : 128 * 1024 * 1024) ||
      (exactSize !== undefined && (!Number.isSafeInteger(exactSize) || exactSize <= 0 || exactSize > maximum))) fail("fixed bounded policy required");
  const owner = uid(), parent = path.dirname(file), parents = ancestors(parent, owner);
  const extra = [];
  if (kind === "local asset") {
    const direct = parents[parents.length - 1][1];
    if (direct.uid !== owner || (direct.mode & 0o7777) !== 0o700) fail("owned private custody parent required");
    for (const root of protectedRoots) {
      canonical(root);
      if (overlaps(parent, root)) fail("custody overlaps protected root");
      const pins = ancestors(root, owner, true), last = pins[pins.length - 1];
      if (last[0] === root && direct.dev === last[1].dev && direct.ino === last[1].ino) fail("custody aliases protected root");
      extra.push(...pins);
    }
  }
  const before = fs.lstatSync(file);
  const regular = stat => stat.isFile() && !stat.isSymbolicLink() && stat.nlink === 1 && stat.size > 0 && stat.size <= maximum &&
    (exactSize === undefined || stat.size === exactSize) &&
    (kind !== "local asset" || (stat.uid === owner && !(stat.mode & 0o7022)));
  if (!regular(before)) fail("regular nonempty bounded single-link file required");
  let fd, closed = false;
  function close() { if (fd !== undefined && !closed) { closed = true; fs.closeSync(fd); } }
  function identities() {
    cancelled(signal);
    if (closed) fail("closed descriptor");
    recheckDirectories([...parents, ...extra], owner);
    const named = fs.lstatSync(file), opened = fs.fstatSync(fd);
    if (!regular(named) || !regular(opened) || !same(before, named, fileKeys) || !same(before, opened, fileKeys)) fail("file identity or metadata changed");
  }
  function read() {
    // Allocate only the admitted initial size plus one overflow detection byte;
    // positional reads also make repeated rechecks independent of fd offsets.
    const body = Buffer.alloc(before.size + 1);
    let offset = 0;
    while (offset < body.length) {
      cancelled(signal);
      const count = fs.readSync(fd, body, offset, Math.min(64 * 1024, body.length - offset), offset);
      if (!Number.isSafeInteger(count) || count < 0 || count > Math.min(64 * 1024, body.length - offset)) fail("invalid bounded read result");
      if (count === 0) break;
      offset += count;
    }
    if (offset !== before.size) fail("short read or growth");
    return body.subarray(0, offset);
  }
  try {
    fd = fs.openSync(file, fs.constants.O_RDONLY | fs.constants.O_NOFOLLOW);
    identities();
    const bytes = read();
    identities();
    const pin = digest(bytes);
    return { get bytes() { return Buffer.from(bytes); }, close,
      recheck() { identities(); const current = read(); identities(); if (digest(current) !== pin) fail("file contents changed"); } };
  } catch (primary) {
    try { close(); } catch (error) { throw new AggregateError([primary, error], "public snapshot read and close failed"); }
    throw primary;
  }
}

module.exports = { snapshotPublicFile, publicPlacement };
