import assert from "node:assert/strict";
import fs from "node:fs/promises";
import { createRequire } from "node:module";
import { fileURLToPath, pathToFileURL } from "node:url";
import { test } from "node:test";
import { currentSourceBody, currentSourceErrors, documentContent, homeErrors, quickstartClaims, quickstartErrors } from "./check-output.mjs";

const highlight = (command) => `<pre><code><span class="line">${command.split(/(?= )/).map((token) => `<span>${token}</span>`).join("")}</span></code></pre>`;
const command = "npx universal-agent-plugins add context7";
const sourceBoundary = "The current npm and native installer release does not include <code>agentplugins author</code>. Current source implements scaffolding and explicit MCP runtime checks.";
const setupLink = '<a href="../build/#build-from-source">source-build setup</a>';
const page = (code) => `<main>${quickstartClaims.map((claim) => `<p>${claim}</p>`).join("")}<p>${sourceBoundary}</p>${setupLink}${code}</main>`;

test("plain and highlighted installer commands satisfy the quickstart contract", () => {
  assert.deepEqual(quickstartErrors(page(`<pre><code>${command}</code></pre>`)), []);
  assert.deepEqual(quickstartErrors(page(highlight(command))), []);
});

test("parsed document text preserves entities and highlighted code without treating attributes as content", () => {
  const html = '<main><p title="<code>attribute command</code>">Use <strong>plugins</strong> &amp; &lt;script&gt;</p>' +
    '<pre><code><span>agentplugins install</span> --locked --scope user</code></pre>' +
    '<a href="../build/&#35;build-from-source">Build</a></main>';
  assert.deepEqual(documentContent(html), {
    text: "Use plugins & <script> agentplugins install --locked --scope user Build",
    commands: ["agentplugins install --locked --scope user"],
    links: ["../build/#build-from-source"],
    parseErrors: []
  });
});

test("comments and non-rendered containers cannot supply current claims, commands or links", () => {
  const hidden = '<p>Agent Plugins authoring is available</p><code>hidden command</code><a href="../build/">Hidden</a>';
  const html = `<script>"<main>${hidden}</main>"</script><main><p>Public</p><!-- ${hidden} -->` +
    `<script>${hidden}</script><style>${hidden}</style><template>${hidden}</template>` +
    `<noscript>${hidden}</noscript><div hidden>${hidden}</div></main>`;
  assert.deepEqual(documentContent(html), { text: "Public", commands: [], links: [], parseErrors: [] });
  for (const tag of ["script", "style", "template", "noscript"]) {
    assert.ok(quickstartErrors(page(`<${tag}>${highlight(command)}</${tag}>`))
      .some((error) => error.includes("released installer command")), tag);
  }
  assert.ok(quickstartErrors(page(`<p title='<code>${command}</code>'>No command</p>`))
    .some((error) => error.includes("released installer command")));
});

test("malformed HTML cannot silently pass an output assertion", () => {
  const invalid = page(highlight(command)).replace("</main>", "<p>");
  assert.ok(quickstartErrors(invalid).some((error) => error.includes("invalid HTML")));
});

test("missing claims and split or incomplete commands fail", () => {
  for (const code of ["", highlight("npx universal-agent-plugins add"), highlight("npx universal-agent-plugins") + highlight(" add context7")]) assert.ok(quickstartErrors(page(code)).length > 0);
  for (const claim of quickstartClaims.filter((item) => !command.includes(item))) {
    assert.ok(quickstartErrors(page(highlight(command)).replaceAll(claim, "")).length > 0, claim);
  }
});

test("current installer exclusion and linked source setup cannot be replaced by old release claims", () => {
  const valid = page(highlight(command));
  assert.ok(quickstartErrors(valid.replace("does not include", "includes")).some((error) => error.includes("excludes agentplugins author")));
  assert.ok(quickstartErrors(valid.replace("../build/#build-from-source", "../build/")).some((error) => error.includes("source-build setup")));
  for (const stale of ["Agent Plugins authoring is available", "public-channel E2E are verified"]) {
    assert.ok(quickstartErrors(valid.replace("</main>", `<p>${stale}</p></main>`)).some((error) => error.includes("public-channel authoring availability")));
    assert.deepEqual(quickstartErrors(valid.replace("</main>", `<!-- ${stale} --></main>`)), []);
  }
  const hiddenBoundary = valid.replace(`<p>${sourceBoundary}</p>`, `<!-- ${sourceBoundary} -->`);
  assert.ok(quickstartErrors(hiddenBoundary).some((error) => error.includes("excludes agentplugins author")));
  const hiddenInstall = valid.replace(highlight(command), `<!-- ${highlight(command)} -->`);
  assert.ok(quickstartErrors(hiddenInstall).some((error) => error.includes("released installer command")));
});

test("actual rendered home and quickstart preserve the released installer and source-build boundary", async () => {
  const website = new URL("../../", import.meta.url);
  const require = createRequire(new URL("package.json", website));
  const { createMarkdownRenderer } = await import(pathToFileURL(process.env.VITEPRESS_TEST_MODULE || require.resolve("vitepress")).href);
  const md = await createMarkdownRenderer(fileURLToPath(website));
  const render = async (relative) => `<main>${await md.renderAsync(await fs.readFile(new URL(`source/en/${relative}`, website), "utf8"))}</main>`;
  const home = await render("index.md");
  const quickstart = await render("guide/quickstart.md");
  assert.deepEqual(homeErrors(home), []);
  assert.deepEqual(quickstartErrors(quickstart), []);
  // The 0.1.94 project-selection minimum remains valid, but cannot satisfy
  // the separate current-release assertion after the installer advances.
  assert.ok(homeErrors(home.replaceAll("CLI 0.1.95", "CLI 0.1.94"))
    .some((error) => error.includes("current installer guidance: CLI 0.1.95")));
  assert.ok(quickstartErrors(quickstart.replaceAll("CLI 0.1.95", "CLI 0.1.94"))
    .some((error) => error.includes("current installer/source guidance: CLI 0.1.95")));
  assert.ok(homeErrors(home.replace("does not include", "includes")).some((error) => error.includes("excludes agentplugins author")));
  assert.ok(homeErrors(home.replace("npx", "not-a-command")).some((error) => error.includes("released installer command")));
});

test("retired authoring copy is rejected even inside highlighted code", () => {
  for (const retired of ["Milestone A", "plugin-kit-ai init demo", "plugin.yaml", "/legacy/v1/"]) assert.ok(quickstartErrors(page(highlight(command) + `<p>${retired}</p>`)).some((error) => error.includes("retired authoring copy")));
});

test("embedded historical locale snapshots are excluded from current-source scanning", () => {
  const source = `current copy\n<!-- locale-historical-source:start\nplugin-kit-ai generate .\nlocale-historical-source:end -->`;
  assert.equal(currentSourceBody(source), "current copy\n");
  assert.deepEqual(currentSourceErrors(source, "ru/guide/quickstart.md"), []);
  assert.deepEqual(currentSourceErrors("plugin-kit-ai generate .", "ru/guide/quickstart.md"), [
    "Current public source contains retired authoring copy: ru/guide/quickstart.md"
  ]);
});
