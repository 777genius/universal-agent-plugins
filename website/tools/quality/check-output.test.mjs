import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import { runInNewContext } from "node:vm";

// Execute the validator's actual quickstart assertions without running its
// unrelated dist checks or requiring a production build for these regressions.
const source = readFileSync(new URL("./check-output.mjs", import.meta.url), "utf8");
const start = source.indexOf("for (const claim of", source.indexOf("const quickstart ="));
const end = source.indexOf("const ciIntegration =", start);
assert.ok(start >= 0 && end > start);
const assertions = source.slice(start, end);
const claims = ["Use plugins", "Build plugins", "Preparation", "unreleased",
  "Historical v1 maintenance", "plugin.json", "Supported Node And Python Paths",
  "If You Are Intentionally Starting On Node Or Python", "What You Get", "What To Do Next"];
const highlight = (command) => `<pre><code><span class="line">${command.split(/(?= )/)
  .map(token => `<span style="--shiki-light:#032F62;--shiki-dark:#9ECBFF;">${token}</span>`)
  .join("")}</span></code></pre>`;
const command = "npx universal-agent-plugins add context7";
function check(code, includedClaims = claims) {
  const errors = [];
  const failed = runInNewContext(`${assertions}\nhasError;`, {
    quickstart: includedClaims.map(claim => `<p>${claim}</p>`).join("") + code,
    hasError: false,
    console: { error: message => errors.push(message) },
  });
  assert.equal(failed, errors.length > 0);
  return errors;
}

test("plain and Shiki-highlighted installer commands are accepted", () => {
  assert.deepEqual(check(`<pre><code>${command}</code></pre>`), []);
  assert.deepEqual(check(highlight(command)), []);
});

test("missing, incomplete, metadata-only and split-block commands are rejected", () => {
  for (const code of ["", highlight("npx universal-agent-plugins add"),
    `<pre><code><span data-command="${command}">unrelated</span></code></pre>`,
    highlight("npx universal-agent-plugins") + highlight(" add context7")]) {
    assert.ok(check(code).some(error => error.endsWith(command)));
  }
});

test("highlighting does not bypass old journey rejection or availability claims", () => {
  for (const old of ["npx plugin-kit-ai@latest add notion", highlight("npx plugin-kit-ai@latest add notion"),
    "Recommended Default"]) {
    assert.ok(check(highlight(command) + old).includes("Quickstart still recommends the old v1 first-run journey."));
  }
  for (const claim of claims) {
    assert.ok(check(highlight(command), claims.filter(value => value !== claim)).length > 0, claim);
  }
});
