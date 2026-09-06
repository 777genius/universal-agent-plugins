import assert from "node:assert/strict";
import fs from "node:fs/promises";
import path from "node:path";
import os from "node:os";
import { fileURLToPath } from "node:url";
import { createHash } from "node:crypto";
import { run } from "../lib/process.mjs";

const repository = fileURLToPath(new URL("../../../", import.meta.url));
for (const [key, value] of Object.entries({ GOPROXY: "off", GOSUMDB: "off", GOENV: "off", GOTOOLCHAIN: "local", GOMAXPROCS: "2" })) {
  if (process.env[key] !== value) throw new Error(`Required offline adapter integration needs ${key}=${value}; no acceptance tests skipped`);
}
for (const key of ["HOME", "TMPDIR", "GOCACHE", "GOMODCACHE", "DOCS_TOOLS_ROOT"])
  if (!path.isAbsolute(process.env[key] || "")) throw new Error(`Required integration needs private absolute ${key} (GOMODCACHE may be an existing read-only cache)`);
if (process.env.GOFLAGS) throw new Error("Required integration needs empty GOFLAGS");
assert.match(await run("go", ["version"]), / go1\.25\.13 /);
const directory = await fs.mkdtemp(path.join(os.tmpdir(), "docs-consumer-integration-"));
console.log(`Integration evidence: ${directory}`);
const source = path.join(directory, "source");
const site = path.join(directory, "site");
// Local clones only; no installation, fetch or caller-worktree mutation.
for (const destination of [source, site])
  await run("git", ["clone", "--quiet", "--no-hardlinks", "--no-checkout", repository, destination]);
// Read the checked-in source pin after setting explicit output/source locations.
process.env.DOCS_AUTHORING_CHECKOUT = source;
process.env.DOCS_SITE_OUTPUT_ROOT = path.join(site, "website");
const { acceptedAuthoringSHA, requireAuthoringSource } = await import("../lib/source-contract.mjs");
process.env.DOCS_AUTHORING_SOURCE_SHA = acceptedAuthoringSHA;
for (const cwd of [source, site]) await run("git", ["checkout", "--quiet", "--detach", acceptedAuthoringSHA], { cwd });
await requireAuthoringSource();
const output = path.join(directory, "adapter-output");
await run("go", ["run", "-p", "2", "./cli/plugin-kit-ai/tools/authoring-docs", "--source-sha", acceptedAuthoringSHA,
  "--checkout", source, "--out-dir", output], { cwd: source, env: { GOWORK: path.join(source, "go.work") } });
const envelope = JSON.parse(await fs.readFile(path.join(output, "prepared-authoring-v2/manifest.json"), "utf8"));
for (const pin of envelope.sources) {
  const digest = createHash("sha256").update(await fs.readFile(path.join(source, pin.path))).digest("hex");
  assert.equal(digest, pin.sha256, pin.path);
}
const log = await run(process.execPath, ["--test", "website/tools/quality/locale-preparation.test.mjs", "website/tools/lib/frontmatter.test.mjs", "website/tools/lib/site-consumer.test.mjs"], {
  cwd: repository, env: { DOCS_TEST_ADAPTER_OUTPUT: output, DOCS_TEST_ARTIFACTS: directory }
});
console.log(log);
await fs.writeFile(path.join(directory, "tests.log"), log);
const { extractCLI } = await import("../extractors/cli.mjs");
const { extractPlatformData } = await import("../extractors/platform.mjs");
const { assembleBundles } = await import("../generate.mjs");
async function assemble() { await assembleBundles([await extractCLI(), await extractPlatformData()]); }
async function snapshot(root, prefix = "") {
  const result = {};
  for (const item of (await fs.readdir(root, { withFileTypes: true })).sort((a,b) => a.name.localeCompare(b.name))) {
    const name = path.join(prefix, item.name), file = path.join(root, item.name);
    if (item.isDirectory()) Object.assign(result, await snapshot(file, name));
    else result[name] = createHash("sha256").update(await fs.readFile(file)).digest("hex");
  }
  return result;
}
await assemble();
const first = await snapshot(path.join(site, "website/generated"));
const runtime = await snapshot(path.join(site, "website/.site"));
await requireAuthoringSource();
// A generated baseline commit is downstream from the immutable adapter pin.
await run("git", ["add", "website/generated"], { cwd: site });
await run("git", ["-c", "user.name=Docs fixture", "-c", "user.email=docs-fixture@example.invalid", "-c", "commit.gpgsign=false",
  "commit", "--quiet", "-m", "Disposable generated baseline"], { cwd: site });
assert.notEqual((await run("git", ["rev-parse", "HEAD"], { cwd: site })).trim(), acceptedAuthoringSHA);
await assemble();
assert.deepEqual(await snapshot(path.join(site, "website/generated")), first);
assert.deepEqual(await snapshot(path.join(site, "website/.site")), runtime);
await requireAuthoringSource();
await run(process.execPath, [path.join(site, "website/tools/quality/check-drift.mjs")]);
const support = path.join(site, "website/generated/en/reference/target-support.md");
await fs.appendFile(support, "\nintentional drift probe\n");
await assert.rejects(run(process.execPath, [path.join(site, "website/tools/quality/check-drift.mjs")]));
await assemble();
assert.deepEqual(await snapshot(path.join(site, "website/generated")), first);
const aliases = JSON.parse(await fs.readFile(path.join(site, "website/generated/registries/redirects.json"), "utf8"));
assert.equal(aliases["/reference/target-support"], "/en/reference/target-support");
// Fail before output writes if callers select their writable site as provenance.
process.env.DOCS_AUTHORING_CHECKOUT = site;
await assert.rejects(requireAuthoringSource(), /separate/);
process.env.DOCS_AUTHORING_CHECKOUT = source;
const dirtyFile = path.join(source, envelope.sources[0].path);
await fs.appendFile(dirtyFile, "\n");
await assert.rejects(requireAuthoringSource(), /tracked changes/);
await assert.rejects(run("go", ["run", "-p", "2", "./cli/plugin-kit-ai/tools/authoring-docs", "--source-sha", acceptedAuthoringSHA,
  "--checkout", source, "--out-dir", path.join(directory, "dirty-rejected")], { cwd: source, env: { GOWORK: path.join(source, "go.work") } }));
await run("git", ["restore", "--", envelope.sources[0].path], { cwd: source });
await requireAuthoringSource();
await fs.writeFile(path.join(directory, "assembly-evidence.json"), JSON.stringify({
  sourceSHA: acceptedAuthoringSHA, sourcePins: envelope.sources, generatedHashes: first, runtimeHashes: runtime,
  repeatability: "three real assemblies; equal bytes; source clean; committed output baseline stable; intentional drift rejected",
  scope: "Actual CLI adapter + complete historical CLI/platform bundles through production assembly. SDK/runtime extractors and full site/browser are separate gates."
}, null, 2) + "\n");
console.log("Required adapter and real assembly integration passed; no skipped acceptance checks.");
