#!/usr/bin/env node
"use strict";

const fs = require("node:fs");
const path = require("node:path");

const VERSION = /^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$/;
const COMMIT = /^[0-9a-f]{40}$/;
const DIGEST = /^[0-9a-f]{64}$/;
const REPOSITORY = /^[A-Za-z0-9][A-Za-z0-9-]*\/[A-Za-z0-9][A-Za-z0-9._-]*$/;
const TARGETS = [
  ["darwin-amd64", "darwin", "amd64", ""],
  ["darwin-arm64", "darwin", "arm64", ""],
  ["linux-amd64", "linux", "amd64", ""],
  ["linux-arm64", "linux", "arm64", ""],
  ["windows-amd64", "windows", "amd64", ".exe"],
  ["windows-arm64", "windows", "arm64", ".exe"]
];

function exactKeys(value, expected, label) {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    throw new Error(`${label} must be an object`);
  }
  const actual = Object.keys(value).sort();
  const wanted = [...expected].sort();
  if (actual.join("\n") !== wanted.join("\n")) {
    throw new Error(`${label} has unexpected or missing fields`);
  }
}

function validateManifest(manifest) {
  exactKeys(manifest, ["schema_version", "tag", "version", "commit", "assets"], "release manifest");
  if (manifest.schema_version !== 2 || !VERSION.test(String(manifest.version))) {
    throw new Error("release manifest schema or version is invalid");
  }
  if (manifest.tag !== `agentplugins-v${manifest.version}` || !COMMIT.test(String(manifest.commit))) {
    throw new Error("release manifest tag or commit is invalid");
  }

  const expectedKeys = TARGETS.map(([key]) => key);
  exactKeys(manifest.assets, expectedKeys, "release manifest assets");
  for (const [key, osName, archName, suffix] of TARGETS) {
    const asset = manifest.assets[key];
    exactKeys(asset, ["file", "sha256", "size"], `release asset ${key}`);
    const expectedFile = `agentplugins_${manifest.version}_${osName}_${archName}${suffix}`;
    if (asset.file !== expectedFile || !DIGEST.test(String(asset.sha256)) ||
        !Number.isSafeInteger(asset.size) || asset.size <= 0) {
      throw new Error(`release asset metadata is invalid: ${key}`);
    }
  }
  return manifest;
}

function assetBlock(repository, manifest, key, indent) {
  const asset = manifest.assets[key];
  const url = `https://github.com/${repository}/releases/download/${manifest.tag}/${asset.file}`;
  return [
    `${indent}url "${url}", using: :nounzip`,
    `${indent}sha256 "${asset.sha256}"`
  ].join("\n");
}

function renderFormula(manifestInput, repository = "777genius/universal-agent-plugins") {
  if (!REPOSITORY.test(repository)) throw new Error(`invalid repository: ${repository}`);
  const manifest = validateManifest(manifestInput);
  return `class Agentplugins < Formula
  desc "Universal installer and lifecycle manager for Agent Plugins 1.0"
  homepage "https://github.com/${repository}"
  version "${manifest.version}"
  license "Apache-2.0"

  on_macos do
    if Hardware::CPU.arm?
${assetBlock(repository, manifest, "darwin-arm64", "      ")}
    else
${assetBlock(repository, manifest, "darwin-amd64", "      ")}
    end
  end

  on_linux do
    if Hardware::CPU.arm?
${assetBlock(repository, manifest, "linux-arm64", "      ")}
    else
${assetBlock(repository, manifest, "linux-amd64", "      ")}
    end
  end

  def install
    asset = Dir["agentplugins_*"].fetch(0)
    bin.install asset => "agentplugins"
    chmod 0755, bin/"agentplugins"
  end

  test do
    assert_equal "agentplugins #{version}", shell_output("#{bin}/agentplugins version").strip
  end
end
`;
}

function main() {
  const [manifestPath, outputPath, repository = "777genius/universal-agent-plugins"] = process.argv.slice(2);
  if (!manifestPath || !outputPath) {
    throw new Error("usage: homebrew-formula.js <release-manifest.json> <output.rb> [owner/repo]");
  }
  const manifest = JSON.parse(fs.readFileSync(path.resolve(manifestPath), "utf8"));
  const output = path.resolve(outputPath);
  fs.mkdirSync(path.dirname(output), { recursive: true });
  fs.writeFileSync(output, renderFormula(manifest, repository), { mode: 0o644 });
}

if (require.main === module) {
  try {
    main();
  } catch (error) {
    process.stderr.write(`agentplugins Homebrew formula: ${error.message}\n`);
    process.exitCode = 1;
  }
}

module.exports = { renderFormula, validateManifest };
