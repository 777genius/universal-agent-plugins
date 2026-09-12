"use strict";
if (process.env.AGENTPLUGINS_STAGED_TEST_CHILD === "1") {
  require("node:test")("source-checkout-only suite", { skip: "requires the complete repository source tree" }, () => {});
} else {
// SYNTHETIC provision only. Harmless bytes are hashed, never executed.
const test = require("node:test"), assert = require("node:assert/strict");
const fs = require("node:fs"), path = require("node:path"), os = require("node:os"), vm = require("node:vm");
const { createRequire } = require("node:module");
const source = path.resolve(__dirname, "../scripts/public-authoring-tools.js"), local = createRequire(source);
const c = local("./dual-authoring-candidate"), shipped = local("./public-authoring-tools");
const base = shipped.readProvisioning(), manifestPath = path.resolve(__dirname, "../../../.github/authoring-public-tools.json");
function fixture(body = c.encode(base), read = c.readFile, filesystem = fs) {
  const context = { module: { exports: {} }, Buffer, TextDecoder, __dirname: path.dirname(source),
    require: id => id === "./dual-authoring-candidate" ? { ...c, readFile: (f, cap) => f === manifestPath ? body : read(f, cap) } : id === "node:fs" ? filesystem : local(id) };
  vm.runInThisContext("(function(require,module,__dirname){" + fs.readFileSync(source, "utf8") + "\n})", { filename: source })(context.require, context.module, context.__dirname); return context.module.exports;
}
const fresh = () => JSON.parse(c.encode(base));
function pin(file) { return { path: file, version: "v22.21.1", sha256: c.digest(fs.readFileSync(file)) }; }
function prepared() {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "C3b-tools-SYNTHETIC-"));
  const file = path.join(root, "node"); fs.writeFileSync(file, "SYNTHETIC TOOL\n");
  const value = fresh(); for (const key of Object.keys(value.controllers["linux-amd64"])) value.controllers["linux-amd64"][key] = pin(file);
  return { root, file, value };
}
test("C3b tools frozen six controllers eighteen cells and unavailable provision", () => {
  assert.equal(Object.keys(base.controllers).length, 6); assert.equal(Object.keys(base.cells).length, 18);
  assert.equal(base.reader, "linux-amd64"); assert.ok(Object.isFrozen(base.cells));
  assert.throws(() => shipped.requireController(), /PUBLIC_PROVISIONING_REQUIRED:linux-amd64:node/);
  for (const key of Object.keys(base.cells)) assert.throws(() => shipped.requireCellTools(key), new RegExp(`PUBLIC_PROVISIONING_REQUIRED:${key}:runner`));
  for (const fn of [() => shipped.readProvisioning("/receipt"), () => shipped.requireController("unknown"),
    () => shipped.requireController("linux-amd64", base), () => shipped.requireCellTools("unknown")]) assert.throws(fn);
});
test("C3b tools literal canonical fixture and closed malformed table", () => {
  const literal = '{\n  "path": "/synthetic/node",\n  "version": "v22.21.1",\n  "sha256": "' + c.digest(Buffer.from("SYNTHETIC TOOL\n")) + '"\n}';
  const value = fresh(); value.controllers["linux-amd64"].node = JSON.parse(literal);
  assert.equal(JSON.stringify(fixture(c.encode(value)).readProvisioning().controllers["linux-amd64"].node, null, 2), literal);
  const missing = fresh(); delete missing.cells["linux-arm64/kit-node18"];
  assert.throws(() => fixture(c.encode(missing)).readProvisioning(), /PUBLIC_PROVISIONING_REQUIRED:linux-arm64\/kit-node18:entry/);
  const mutations = [v => delete v.cells["linux-arm64/kit-node18"], v => v.controllers.extra = {},
    v => delete v.controllers["linux-amd64"].tar, v => v.cells["linux-amd64/kit-node18"].extra = null,
    v => v.reader = "windows-amd64", v => v.cells["linux-amd64/kit-node18"].controller = "linux-arm64",
    v => v.controllers["linux-amd64"].node.sha256 = "0".repeat(64), v => v.controllers["linux-amd64"].node.path = "/synthetic/../node",
    v => v.controllers["linux-amd64"].node.version = "x".repeat(257),
    v => v.controllers["linux-amd64"].node.extra = true];
  for (const mutate of mutations) { const bad = JSON.parse(c.encode(value)); mutate(bad); assert.throws(() => fixture(c.encode(bad)).readProvisioning()); }
  for (const body of [Buffer.from(JSON.stringify(value)), Buffer.from(c.encode(value).toString().replace('  "schema":', '  "reader": "linux-amd64",\n  "schema":')),
    Buffer.from('[['.repeat(10)), Buffer.alloc(1024 * 1024 + 1, 32), Buffer.from([255]), Buffer.from('null\n')]) {
    assert.throws(() => fixture(body).readProvisioning());
  }
});
test("C3b tools controller pins missing tools links and late replacement", () => {
  const { file, value, root } = prepared(); const api = fixture(c.encode(value));
  assert.equal(api.requireController(), file);
  fs.writeFileSync(file, "CHANGED TOOL\n"); assert.throws(() => api.requireController(), /pin mismatch/);
  fs.writeFileSync(file, "SYNTHETIC TOOL\n");
  const link = path.join(root, "link"); fs.symlinkSync(file, link);
  value.controllers["linux-amd64"].node.path = link; assert.throws(() => fixture(c.encode(value)).requireController());
  value.controllers["linux-amd64"].node.path = path.join(root, "missing");
  assert.throws(() => fixture(c.encode(value)).requireController(), /PUBLIC_PROVISIONING_REQUIRED:linux-amd64:node/);
  value.controllers["linux-amd64"].node = pin(file); value.controllers["linux-amd64"].gh = null;
  assert.throws(() => fixture(c.encode(value)).requireController(), /PUBLIC_PROVISIONING_REQUIRED:linux-amd64:gh/);
  fs.linkSync(file, path.join(root, "hardlink")); assert.throws(() => api.requireController());
});
test("C3b tools complete npm closure and cell capabilities", () => {
  const { file, value, root } = prepared(), key = "linux-amd64/pair-node22", row = value.cells[key];
  const npmRoot = path.join(root, "npm"); fs.mkdirSync(npmRoot); const cli = path.join(npmRoot, "npm-cli.js"); fs.writeFileSync(cli, "SYNTHETIC NPM\n");
  const files = [{ path: "npm-cli.js", sha256: pin(cli).sha256 }];
  for (const name of ["runner", "image", "observer", "installer_policy"]) row[name] = { id: "synthetic-only", sha256: pin(file).sha256 };
  for (const name of ["npm_node", "shim_node", "go"]) row[name] = pin(file);
  row.npm = { ...pin(cli), closure: { root: npmRoot, files } }; row.mod_cache = { root: npmRoot, files };
  assert.equal(fixture(c.encode(value)).requireCellTools(key).npm.path, cli);
  for (const name of ["image", "npm_node", "shim_node", "npm", "go", "mod_cache", "observer", "installer_policy"]) {
    const bad = JSON.parse(c.encode(value)); bad.cells[key][name] = null;
    assert.throws(() => fixture(c.encode(bad)).requireCellTools(key), new RegExp(`PUBLIC_PROVISIONING_REQUIRED:${key}:${name}`));
  }
  const empty = path.join(npmRoot, "empty.js"); fs.writeFileSync(empty, "");
  assert.throws(() => fixture(c.encode(value)).requireCellTools(key), /closure mismatch/);
  files.unshift({ path: "empty.js", sha256: pin(empty).sha256 });
  assert.equal(fixture(c.encode(value)).requireCellTools(key).mod_cache.files[0].sha256, c.digest(Buffer.alloc(0)));
  // Empty members remain exhaustive and cannot be aliased or change mid-read.
  const hard = path.join(root, "empty-hardlink"); fs.linkSync(empty, hard);
  assert.throws(() => fixture(c.encode(value)).requireCellTools(key), /unaliased/);
  // Retain fixtures without cleanup: use a fresh empty member after the alias case.
  const nextRoot = path.join(root, "next-npm"); fs.mkdirSync(nextRoot);
  fs.writeFileSync(path.join(nextRoot, "empty.js"), ""); fs.writeFileSync(path.join(nextRoot, "npm-cli.js"), "SYNTHETIC NPM\n");
  row.npm.path = path.join(nextRoot, "npm-cli.js"); row.npm.closure.root = nextRoot; row.mod_cache.root = nextRoot;
  const target = path.join(nextRoot, "empty.js");
  let changed = false;
  const filesystem = { ...fs, readSync(fd, ...args) {
    if (!changed) { changed = true; fs.writeFileSync(target, "x"); }
    return fs.readSync(fd, ...args);
  } };
  assert.throws(() => fixture(c.encode(value), c.readFile, filesystem).requireCellTools(key), /closure file changed/);
  fs.writeFileSync(target, "");
  const aliasRoot = path.join(root, "linked-npm"); fs.symlinkSync(nextRoot, aliasRoot);
  const linked = JSON.parse(c.encode(value)); linked.cells[key].mod_cache.root = aliasRoot;
  assert.throws(() => fixture(c.encode(linked)).requireCellTools(key), /symlink/);
  const replacedDescriptor = { ...fs, fstatSync(fd, options) {
    const st = fs.fstatSync(fd, options); return { ...st, ino: st.ino + 1n };
  } };
  assert.throws(() => fixture(c.encode(value), c.readFile, replacedDescriptor).requireCellTools(key), /closure file changed/);
  const emptyTool = JSON.parse(c.encode(value)); emptyTool.controllers["linux-amd64"].node = pin(target);
  assert.throws(() => fixture(c.encode(emptyTool)).requireController(), /nonempty/);
  assert.throws(() => fixture(Buffer.alloc(0)).readProvisioning());
  const bad = JSON.parse(c.encode(value)); bad.cells[key].npm.closure.files.push(files[0]);
  assert.throws(() => fixture(c.encode(bad)).readProvisioning(), /ordered unique/);
  fs.writeFileSync(path.join(row.npm.closure.root, "unlisted.js"), "UNLISTED\n");
  assert.throws(() => fixture(c.encode(value)).requireCellTools(key), /complete source-frozen closure mismatch/);
});
test("C3b tools manifest absence and source root cannot be substituted", () => {
  const error = Object.assign(new Error("missing"), { code: "ENOENT" });
  const context = { module: { exports: {} }, Buffer, TextDecoder, __dirname: path.dirname(source),
    require: id => id === "./dual-authoring-candidate" ? { ...c, readFile: f => { assert.equal(f, manifestPath); throw error; } } : local(id) };
  vm.runInThisContext("(function(require,module,__dirname){" + fs.readFileSync(source, "utf8") + "\n})")(context.require, context.module, context.__dirname);
  assert.throws(() => context.module.exports.readProvisioning(), /PUBLIC_PROVISIONING_REQUIRED:linux-amd64:manifest/);
  assert.throws(() => context.module.exports.readProvisioning(base), /trusted source only/);
});
test("C3b tools JS Python agree on identical canonical fixture bytes", () => {
  const { spawnSync } = require("node:child_process");
  const { root, value } = prepared(), bodies = [c.encode(base), c.encode(value)];
  for (const mutate of [v => v.reader = "darwin-amd64", v => delete v.cells["windows-arm64/pair-node24"],
    v => v.controllers["linux-amd64"].node.extra = true, v => v.controllers["linux-amd64"].node.sha256 = "0".repeat(64),
    v => v.controllers["linux-amd64"].node.path = "/synthetic/../node", v => v.controllers["linux-amd64"].node.path = "/synthetic/node/",
    v => v.controllers["linux-amd64"].node = { version: "v22.21.1", path: "/synthetic/node", sha256: v.controllers["linux-amd64"].node.sha256 }]) {
    const bad = JSON.parse(c.encode(value)); mutate(bad); bodies.push(c.encode(bad));
  }
  bodies.push(Buffer.from(JSON.stringify(value)), Buffer.from(c.encode(value).toString().replace('  "schema":', '  "reader": "linux-amd64",\n  "schema":')),
    Buffer.from('['.repeat(9)), Buffer.alloc(1024 * 1024 + 1, 32), Buffer.from([255]));
  const expected = bodies.map(body => { try { fixture(body).readProvisioning(); return true; } catch { return false; } });
  assert.deepEqual(expected, [true, true, ...Array(bodies.length - 2).fill(false)]);
  // Identical canonical bytes at 4095/4096/4097 code points, including supplementary Unicode.
  for (const [suffix, accepted] of [["", true], ["a", true], ["ab", false]]) {
    const unicode = JSON.parse(c.encode(value));
    unicode.controllers["linux-amd64"].node.path = "/" + "😀".repeat(4094) + suffix;
    const body = c.encode(unicode); bodies.push(body); expected.push(accepted);
    if (accepted) assert.doesNotThrow(() => fixture(body).readProvisioning());
    else assert.throws(() => fixture(body).readProvisioning(), /provision path/);
  }
  bodies.forEach((body, i) => fs.writeFileSync(path.join(root, `${i}.json`), body));
  const code = `import importlib.util,json,pathlib,sys
spec=importlib.util.spec_from_file_location('provision',sys.argv[1]); p=importlib.util.module_from_spec(spec); spec.loader.exec_module(p)
root=pathlib.Path(sys.argv[2]); (root/'.github').mkdir(); p.__file__=str(root/'scripts/check-packed-ci.py'); results=[]
for i in range(int(sys.argv[3])):
 (root/'.github/authoring-public-tools.json').write_bytes((root/(str(i)+'.json')).read_bytes())
 try: p.read_provisioning(); results.append(True)
 except (ValueError,TypeError): results.append(False)
print(json.dumps(results))
`;
  const argv = ["-B", "-c", code, path.resolve(__dirname, "../../../scripts/check-packed-ci.py"), root, String(bodies.length)];
  const env = Object.fromEntries(["HOME", "TMPDIR", "TMP", "TEMP", "XDG_CACHE_HOME"].map(k => [k, process.env[k]]));
  Object.assign(env, { PATH: "/usr/local/bin:/usr/bin:/bin", LANG: "C.UTF-8", LC_ALL: "C.UTF-8" });
  const result = spawnSync("/usr/bin/python3", argv, { env, cwd: root, encoding: "utf8" });
  fs.writeFileSync(path.join(root, "agreement-receipt.json"), JSON.stringify({ argv: ["/usr/bin/python3", ...argv], env, cwd: root,
    exit: result.status, stdout: result.stdout, stderr: result.stderr, fixtures: bodies.map(c.digest), expected }, null, 2) + "\n");
  assert.equal(result.status, 0, result.stderr); assert.equal(result.stderr, ""); assert.deepEqual(JSON.parse(result.stdout), expected);
});
}
