"use strict";

const path = require("node:path");

const OS = {
  darwin: "darwin",
  linux: "linux",
  win32: "windows"
};

const ARCH = {
  x64: "amd64",
  arm64: "arm64"
};

function detectPlatform(platform = process.platform, arch = process.arch) {
  const osName = OS[platform];
  const archName = ARCH[arch];
  if (!osName) {
    throw new Error(`unsupported operating system: ${platform}`);
  }
  if (!archName) {
    throw new Error(`unsupported CPU architecture: ${arch}`);
  }
  return {
    key: `${osName}-${archName}`,
    osName,
    archName,
    binaryName: osName === "windows" ? "agentplugins.exe" : "agentplugins"
  };
}

function expectedAssetName(version, platformInfo) {
  const suffix = platformInfo.osName === "windows" ? ".exe" : "";
  return `agentplugins_${version}_${platformInfo.osName}_${platformInfo.archName}${suffix}`;
}

function cacheRoot(environment = process.env, platform = process.platform, home = require("node:os").homedir()) {
  if (String(environment.AGENTPLUGINS_CACHE_DIR || "").trim()) {
    return path.resolve(String(environment.AGENTPLUGINS_CACHE_DIR).trim());
  }
  if (platform === "win32") {
    const local = String(environment.LOCALAPPDATA || "").trim();
    return path.join(local || path.join(home, "AppData", "Local"), "agentplugins", "Cache");
  }
  const xdg = String(environment.XDG_CACHE_HOME || "").trim();
  if (xdg) {
    return path.join(path.resolve(xdg), "agentplugins");
  }
  if (platform === "darwin") {
    return path.join(home, "Library", "Caches", "agentplugins");
  }
  return path.join(home, ".cache", "agentplugins");
}

module.exports = { cacheRoot, detectPlatform, expectedAssetName };
