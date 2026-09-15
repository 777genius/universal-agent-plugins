import assert from "node:assert/strict";
import { test } from "node:test";
import { quickstartClaims, quickstartErrors } from "./check-output.mjs";

const highlight = (command) => `<pre><code><span class="line">${command.split(/(?= )/).map((token) => `<span>${token}</span>`).join("")}</span></code></pre>`;
const command = "npx universal-agent-plugins add context7";
const page = (code, claims = quickstartClaims) => `<main>${claims.map((claim) => `<p>${claim}</p>`).join("")}${code}</main>`;

test("plain and highlighted installer commands satisfy the quickstart contract", () => {
  assert.deepEqual(quickstartErrors(page(`<pre><code>${command}</code></pre>`)), []);
  assert.deepEqual(quickstartErrors(page(highlight(command))), []);
});

test("missing claims and split or incomplete commands fail", () => {
  for (const code of ["", highlight("npx universal-agent-plugins add"), highlight("npx universal-agent-plugins") + highlight(" add context7")]) assert.ok(quickstartErrors(page(code)).length > 0);
  for (const claim of quickstartClaims) assert.ok(quickstartErrors(page(highlight(command), quickstartClaims.filter((item) => item !== claim))).length > 0, claim);
});

test("retired authoring copy is rejected even inside highlighted code", () => {
  for (const retired of ["Milestone A", "plugin-kit-ai init demo", "plugin.yaml", "/legacy/v1/"]) assert.ok(quickstartErrors(page(highlight(command) + `<p>${retired}</p>`)).some((error) => error.includes("retired authoring copy")));
});
