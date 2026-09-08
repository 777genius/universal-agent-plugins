"use strict";

const assert = require("node:assert/strict");
const test = require("node:test");

const { renderFormula, validateManifest } = require("../scripts/homebrew-formula");

const targets = [
  ["darwin-amd64", "darwin", "amd64", ""],
  ["darwin-arm64", "darwin", "arm64", ""],
  ["linux-amd64", "linux", "amd64", ""],
  ["linux-arm64", "linux", "arm64", ""],
  ["windows-amd64", "windows", "amd64", ".exe"],
  ["windows-arm64", "windows", "arm64", ".exe"]
];

function manifest() {
  const version = "1.2.3";
  return {
    schema_version: 2,
    tag: `agentplugins-v${version}`,
    version,
    commit: "1".repeat(40),
    assets: Object.fromEntries(targets.map(([key, osName, archName, suffix], index) => [
      key,
      {
        file: `agentplugins_${version}_${osName}_${archName}${suffix}`,
        sha256: String(index + 1).repeat(64),
        size: 1000 + index
      }
    ]))
  };
}

test("renders a native macOS and Linux formula from exact release identities", () => {
  const formula = renderFormula(manifest());
  for (const [key, osName, archName] of targets.filter(([key]) => !key.startsWith("windows"))) {
    assert.match(formula, new RegExp(`agentplugins_1\\.2\\.3_${osName}_${archName}`), key);
  }
  assert.match(formula, /using: :nounzip/);
  assert.match(formula, /license "Apache-2\.0"/);
  assert.match(formula, /bin\.install asset => "agentplugins"/);
  assert.match(formula, /assert_equal "agentplugins #\{version\}"/);
  assert.doesNotMatch(formula, /windows_(?:amd64|arm64)/);
});

test("fails closed on every release identity used by the formula", () => {
  const mutations = [
    (value) => { value.schema_version = 1; },
    (value) => { value.tag = "v1.2.3"; },
    (value) => { value.version = "01.2.3"; },
    (value) => { value.commit = "main"; },
    (value) => { value.assets["darwin-arm64"].file = "agentplugins"; },
    (value) => { value.assets["linux-amd64"].sha256 = "0"; },
    (value) => { value.assets["linux-arm64"].size = 0; },
    (value) => { value.assets.extra = value.assets["linux-arm64"]; }
  ];
  for (const mutate of mutations) {
    const value = structuredClone(manifest());
    mutate(value);
    assert.throws(() => validateManifest(value));
  }
  assert.throws(() => renderFormula(manifest(), "https://example.com/repo"), /invalid repository/);
});
