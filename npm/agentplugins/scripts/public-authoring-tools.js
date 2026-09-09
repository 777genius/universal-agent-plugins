"use strict";
// Source checkout + independently immutable provision are authority, never receipts.
const fs = require("node:fs");
const path = require("node:path");
const assert = require("node:assert/strict");
const c = require("./dual-authoring-candidate");
const { fields: checkedFields, hash } = require("../lib/public-authoring-contract").checks;
const TARGETS = ["linux-amd64", "linux-arm64", "darwin-amd64", "darwin-arm64", "windows-amd64", "windows-arm64"];
const CELLS = TARGETS.flatMap(t => ["kit-node18", "pair-node22", "pair-node24"].map(l => `${t}/${l}`));
const TOOLS = ["node", "python", "git", "gh", "tar"];
const CELL_FIELDS = ["runner", "image", "controller", "npm_node", "shim_node", "npm", "go", "mod_cache", "observer", "installer_policy"];
const ROOT = path.resolve(__dirname, "../../..");
const MANIFEST = path.join(ROOT, ".github/authoring-public-tools.json");
const LIMIT = 1024 * 1024;
function fields(v, names, label) {
  if (v && typeof v === "object" && !Array.isArray(v)) for (const name of names) {
    if (!Object.hasOwn(v, name)) absent(["six controllers", "eighteen cells"].includes(label) ? `${name}:entry` : `${label}:${name}`);
  }
  checkedFields(v, names, label); assert.deepEqual(Object.keys(v), names, "ordered provision fields");
}
function absent(label) { throw new Error(`PUBLIC_PROVISIONING_REQUIRED:${label}`); }
function text(v) { assert.ok(typeof v === "string" && /^[\x21-\x7e]{1,256}$/.test(v), "bounded provision identity"); }
function absolute(v, target) {
  assert.ok(typeof v === "string" && v.length <= 4096 && !/[\x00-\x1f\x7f]/.test(v), "provision path");
  const p = target.startsWith("windows-") ? path.win32 : path.posix;
  assert.ok(p.isAbsolute(v) && p.normalize(v) === v && v !== p.parse(v).root &&
    !v.startsWith("\\\\") && !v.startsWith("//") && !v.endsWith(p.sep), "canonical provision path");
}
function identity(v) { fields(v, ["id", "sha256"], "provision identity"); text(v.id); hash(v.sha256, "identity"); }
function closure(v, target) {
  fields(v, ["root", "files"], "complete provision closure"); absolute(v.root, target);
  assert.ok(Array.isArray(v.files) && v.files.length > 0 && v.files.length <= 4096, "bounded provision closure");
  let last = "";
  for (const row of v.files) {
    fields(row, ["path", "sha256"], "closure file"); hash(row.sha256, "closure file");
    assert.ok(typeof row.path === "string" && /^[A-Za-z0-9_.@+-]+(?:\/[A-Za-z0-9_.@+-]+)*$/.test(row.path) &&
      row.path.length <= 4096 && !row.path.split("/").some(x => x === "." || x === "..") &&
      row.path > last, "ordered unique relative closure files"); last = row.path;
  }
}
function tool(v, target, npm = false) {
  fields(v, npm ? ["path", "version", "sha256", "closure"] : ["path", "version", "sha256"], "provision tool");
  absolute(v.path, target); text(v.version); hash(v.sha256, "tool");
  if (npm) {
    closure(v.closure, target);
    const p = target.startsWith("windows-") ? path.win32 : path.posix;
    assert.ok(v.closure.files.some(f => p.join(v.closure.root, ...f.path.split("/")) === v.path && f.sha256 === v.sha256), "npm CLI in complete closure");
  }
}
function decode(body) {
  assert.ok(body.length > 0 && body.length <= LIMIT, "bounded provision manifest");
  const s = new TextDecoder("utf-8", { fatal: true, ignoreBOM: true }).decode(body);
  let depth = 0, quoted = false, escaped = false;
  for (const ch of s) {
    if (quoted) { if (escaped) escaped = false; else if (ch === "\\") escaped = true; else if (ch === '"') quoted = false; }
    else if (ch === '"') quoted = true;
    else if (ch === "{" || ch === "[") assert.ok(++depth <= 8, "provision depth");
    else if (ch === "}" || ch === "]") depth--;
  }
  const v = JSON.parse(s);
  assert.deepEqual(body, c.encode(v), "canonical provision JSON: duplicates/alternate encoding rejected");
  fields(v, ["schema", "controllers", "cells", "reader"], "provision manifest");
  assert.equal(v.schema, "authoring-public-tools/v1"); assert.equal(v.reader, "linux-amd64");
  fields(v.controllers, TARGETS, "six controllers"); fields(v.cells, CELLS, "eighteen cells");
  for (const target of TARGETS) {
    fields(v.controllers[target], TOOLS, target);
    for (const t of TOOLS) if (v.controllers[target][t] !== null) tool(v.controllers[target][t], target);
  }
  for (const key of CELLS) {
    const row = v.cells[key], target = key.split("/")[0], major = key.match(/node(\d+)$/)[1];
    fields(row, CELL_FIELDS, key); assert.equal(row.controller, target);
    for (const t of ["runner", "image", "observer", "installer_policy"]) if (row[t] !== null) identity(row[t]);
    for (const t of ["npm_node", "shim_node", "npm", "go"]) if (row[t] !== null) {
      tool(row[t], target, t === "npm");
      if (t.endsWith("_node")) assert.match(row[t].version, new RegExp(`^v${major}\\.[0-9]+\\.[0-9]+$`));
    }
    if (row.mod_cache !== null) closure(row.mod_cache, target);
  }
  return v;
}
function freeze(v) { if (v && typeof v === "object") { Object.values(v).forEach(freeze); Object.freeze(v); } return v; }
function readProvisioning() {
  assert.equal(arguments.length, 0, "provision root is trusted source only");
  try { return freeze(decode(c.readFile(MANIFEST, LIMIT))); }
  catch (e) { if (e.code === "ENOENT") absent("linux-amd64:manifest"); throw e; }
}
function checkTool(t, label) {
  if (t === null) absent(label);
  let bytes;
  try { bytes = c.readFile(t.path, 256 * LIMIT); }
  catch (e) { if (e.code === "ENOENT") absent(label); throw e; }
  assert.equal(c.digest(bytes), t.sha256, `source-frozen provision pin mismatch:${label}`);
}
function checkClosure(value, label) {
  if (value === null) absent(label);
  const found = []; let entries = 0, total = 0;
  function walk(dir, relative) {
    c.safeDirectory(dir);
    for (const name of fs.readdirSync(dir).sort()) {
      assert.ok(++entries <= 8192, "closure entry bound");
      const file = path.join(dir, name), rel = relative ? `${relative}/${name}` : name, st = fs.lstatSync(file);
      total += st.isFile() ? st.size : 0; assert.ok(total <= 256 * LIMIT, "closure byte bound");
      if (st.isDirectory()) walk(file, rel);
      else { assert.ok(found.length < 4096, "closure bound"); found.push({ path: rel, sha256: c.digest(c.readFile(file, 256 * LIMIT)) }); }
    }
  }
  try { walk(value.root, ""); } catch (e) { if (e.code === "ENOENT") absent(label); throw e; }
  found.sort((a, b) => a.path < b.path ? -1 : 1);
  assert.deepEqual(found, value.files, `complete source-frozen closure mismatch:${label}`);
}
function requireController(key = "linux-amd64") {
  assert.ok(arguments.length <= 1 && TARGETS.includes(key), "fixed controller key");
  const provision = readProvisioning();
  for (const t of TOOLS) checkTool(provision.controllers[key][t], `${key}:${t}`);
  return provision.controllers[key].node.path;
}
function requireCellTools(key) {
  assert.ok(arguments.length === 1 && CELLS.includes(key), "fixed cell key");
  const provision = readProvisioning(), row = provision.cells[key];
  // Availability checks precede any tool access or future cell scheduling.
  const required = CELL_FIELDS.filter(n => !(["go", "mod_cache"].includes(n) && key !== "linux-amd64/pair-node22") && !(n === "installer_policy" && key.endsWith("kit-node18")));
  for (const name of required) if (row[name] === null) absent(`${key}:${name}`);
  requireController(row.controller);
  for (const name of ["npm_node", "shim_node", "npm", "go"]) if (required.includes(name)) checkTool(row[name], `${key}:${name}`);
  checkClosure(row.npm.closure, `${key}:npm`); if (required.includes("mod_cache")) checkClosure(row.mod_cache, `${key}:mod_cache`);
  return row;
}
module.exports = { readProvisioning, requireController, requireCellTools };
