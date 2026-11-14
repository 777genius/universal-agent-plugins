"use strict";

const osMap = {
  darwin: "darwin",
  linux: "linux",
  win32: "windows"
};

const archMap = {
  x64: "amd64",
  arm64: "arm64"
};

function detectPlatform(platform = process.platform, arch = process.arch) {
  const osName = osMap[platform];
  if (!osName) {
    throw new Error(`unsupported OS ${platform}`);
  }
  const archName = archMap[arch];
  if (!archName) {
    throw new Error(`unsupported architecture ${arch}`);
  }
  return {
    osName,
    archName,
    binaryName: osName === "windows" ? "plugin-kit-ai.exe" : "plugin-kit-ai"
  };
}

function assetNameForVersion(version, platformInfo) {
  return `plugin-kit-ai_${version}_${platformInfo.osName}_${platformInfo.archName}.tar.gz`;
}

module.exports = {
  detectPlatform,
  assetNameForVersion
};
