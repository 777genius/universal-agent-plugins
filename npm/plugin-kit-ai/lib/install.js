"use strict";

const crypto = require("crypto");
const fs = require("fs");
const http = require("http");
const https = require("https");
const os = require("os");
const path = require("path");
const zlib = require("zlib");

const { assetNameForVersion, detectPlatform } = require("./platform");

const defaultRepository = "777genius/plugin-kit-ai";
const placeholderVersion = "0.0.0-development";

function normalizeTag(raw) {
  const value = String(raw || "").trim();
  if (!value || value === "latest") {
    return "";
  }
  return value.startsWith("v") ? value : `v${value}`;
}

function deriveReleaseBase(apiBase, override) {
  if (override && String(override).trim()) {
    return String(override).trim().replace(/\/$/, "");
  }
  const trimmed = String(apiBase || "https://api.github.com").trim().replace(/\/$/, "");
  if (trimmed === "https://api.github.com" || trimmed === "http://api.github.com") {
    return "https://github.com";
  }
  return trimmed.replace(/\/api\/v3$/, "").replace(/\/api$/, "");
}

function readPackageVersion(packageRoot) {
  const pkgPath = path.join(packageRoot, "package.json");
  const pkg = JSON.parse(fs.readFileSync(pkgPath, "utf8"));
  return String(pkg.version || "").trim();
}

function resolveRequestedTag(packageRoot) {
  const envVersion = normalizeTag(process.env.PLUGIN_KIT_AI_VERSION || "");
  if (envVersion) {
    return envVersion;
  }
  const packageVersion = readPackageVersion(packageRoot);
  if (packageVersion && packageVersion !== placeholderVersion) {
    return normalizeTag(packageVersion);
  }
  return "";
}

function headers(acceptJson) {
  const out = {};
  if (process.env.GITHUB_TOKEN) {
    out.Authorization = `Bearer ${process.env.GITHUB_TOKEN}`;
  }
  if (acceptJson) {
    out.Accept = "application/vnd.github+json";
  }
  return out;
}

function fetchBuffer(url, extraHeaders, redirects = 5) {
  return new Promise((resolve, reject) => {
    const target = new URL(url);
    const client = target.protocol === "https:" ? https : http;
    const req = client.get(target, { headers: extraHeaders }, (res) => {
      if ([301, 302, 303, 307, 308].includes(res.statusCode) && res.headers.location) {
        if (redirects <= 0) {
          reject(new Error(`too many redirects for ${url}`));
          return;
        }
        res.resume();
        const nextURL = new URL(res.headers.location, target).toString();
        fetchBuffer(nextURL, extraHeaders, redirects - 1).then(resolve, reject);
        return;
      }
      if (res.statusCode < 200 || res.statusCode > 299) {
        const chunks = [];
        res.on("data", (chunk) => chunks.push(chunk));
        res.on("end", () => {
          reject(new Error(`request failed for ${url}: HTTP ${res.statusCode} ${Buffer.concat(chunks).toString("utf8").trim()}`));
        });
        return;
      }
      const chunks = [];
      res.on("data", (chunk) => chunks.push(chunk));
      res.on("end", () => resolve(Buffer.concat(chunks)));
    });
    req.on("error", reject);
  });
}

async function fetchText(url, acceptJson) {
  const buffer = await fetchBuffer(url, headers(acceptJson));
  return buffer.toString("utf8");
}

async function latestTag(apiBase, repository) {
  const cleanBase = String(apiBase || "https://api.github.com").trim().replace(/\/$/, "");
  const body = await fetchText(`${cleanBase}/repos/${repository}/releases/latest`, true);
  const payload = JSON.parse(body);
  if (!payload.tag_name) {
    throw new Error(`could not resolve latest release tag from ${cleanBase}`);
  }
  return normalizeTag(payload.tag_name);
}

function parseChecksums(text) {
  const out = new Map();
  for (const rawLine of String(text || "").split(/\r?\n/)) {
    const line = rawLine.trim();
    if (!line) {
      continue;
    }
    const fields = line.split(/\s+/);
    if (fields.length < 2) {
      throw new Error(`invalid checksums.txt line "${line}"`);
    }
    const sum = fields[0].trim();
    const name = fields[fields.length - 1].replace(/^\*/, "").trim();
    out.set(name, sum);
  }
  return out;
}

function sha256(buffer) {
  return crypto.createHash("sha256").update(buffer).digest("hex");
}

function readString(block, start, end) {
  const value = block.subarray(start, end).toString("utf8");
  return value.replace(/\0.*$/, "").trim();
}

function parseOctal(value) {
  const trimmed = String(value || "").replace(/\0/g, "").trim();
  if (!trimmed) {
    return 0;
  }
  return parseInt(trimmed, 8);
}

function extractBinaryFromTarGz(archiveBuffer, wantedName) {
  const tarBuffer = zlib.gunzipSync(archiveBuffer);
  let offset = 0;
  while (offset + 512 <= tarBuffer.length) {
    const header = tarBuffer.subarray(offset, offset + 512);
    if (header.every((byte) => byte === 0)) {
      break;
    }
    const name = readString(header, 0, 100);
    const prefix = readString(header, 345, 500);
    const fullName = prefix ? `${prefix}/${name}` : name;
    const size = parseOctal(readString(header, 124, 136));
    const typeFlag = header[156] === 0 ? "0" : String.fromCharCode(header[156]);
    const dataStart = offset + 512;
    const dataEnd = dataStart + size;
    if ((typeFlag === "0" || typeFlag === "") && path.basename(fullName) === wantedName) {
      return tarBuffer.subarray(dataStart, dataEnd);
    }
    offset = dataStart + Math.ceil(size / 512) * 512;
  }
  throw new Error(`archive does not contain ${wantedName} at archive root`);
}

async function ensureInstalled(options = {}) {
  const packageRoot = options.packageRoot || path.resolve(__dirname, "..");
  const packageVersion = readPackageVersion(packageRoot);
  const major = /^(0|[1-9]\d*)\./.exec(packageVersion);
  let descriptor = false;
  try { fs.lstatSync(path.join(packageRoot, "public-release.json")); descriptor = true; }
  catch (error) { if (error.code !== "ENOENT") throw error; }
  if (descriptor || (major && Number(major[1]) >= 2)) {
    try {
      if (packageVersion !== "2.0.0") throw new Error("unsupported public plugin-kit-ai version");
      if (!descriptor) throw new Error("plugin-kit-ai major 2 requires public-release.json");
      const result = await require("./public-authoring").ensureBinary("plugin-kit-ai", { ...options, packageRoot });
      return { ...result, installedBinary: result.binaryPath };
    } catch (error) { error.publicAuthoring = true; throw error; }
  }
  const rejectLegacyMajor = tag => {
    if (/^v?(?:[2-9]|[1-9]\d+)\./.test(tag)) throw new Error("legacy wrapper cannot acquire plugin-kit-ai major 2 or later");
  };
  rejectLegacyMajor(normalizeTag(process.env.PLUGIN_KIT_AI_VERSION));
  const repository = process.env.PLUGIN_KIT_AI_REPOSITORY || defaultRepository;
  const apiBase = process.env.GITHUB_API_BASE || "https://api.github.com";
  const releaseBase = deriveReleaseBase(apiBase, process.env.PLUGIN_KIT_AI_RELEASE_BASE_URL);
  const platformInfo = detectPlatform();
  let tag = resolveRequestedTag(packageRoot);
  if (!tag) {
    tag = await latestTag(apiBase, repository);
  }
  rejectLegacyMajor(tag);
  const version = tag.replace(/^v/, "");
  const assetName = assetNameForVersion(version, platformInfo);
  const vendorDir = path.join(packageRoot, "vendor", tag);
  const installedBinary = path.join(vendorDir, platformInfo.binaryName);
  if (fs.existsSync(installedBinary)) {
    return { tag, version, assetName, installedBinary, repository };
  }

  const downloadBase = `${releaseBase}/${repository}/releases/download/${tag}`;
  const checksums = parseChecksums(await fetchText(`${downloadBase}/checksums.txt`, false));
  if (!checksums.has(assetName)) {
    throw new Error(`checksums.txt missing asset ${assetName}`);
  }

  const archive = await fetchBuffer(`${downloadBase}/${assetName}`, headers(false));
  const expectedSum = checksums.get(assetName);
  const actualSum = sha256(archive);
  if (actualSum !== expectedSum) {
    throw new Error(`checksum mismatch for ${assetName}`);
  }

  fs.mkdirSync(vendorDir, { recursive: true });
  const binary = extractBinaryFromTarGz(archive, platformInfo.binaryName);
  fs.writeFileSync(installedBinary, binary);
  if (platformInfo.osName !== "windows") {
    fs.chmodSync(installedBinary, 0o755);
  }

  if (!options.quiet) {
    process.stdout.write(
      [
        "Installed plugin-kit-ai npm wrapper binary",
        `Version: ${tag}`,
        `Repository: ${repository}`,
        `Asset: ${assetName}`,
        `Installed path: ${installedBinary}`,
        "Checksum: verified via checksums.txt"
      ].join(os.EOL) + os.EOL
    );
  }

  return { tag, version, assetName, installedBinary, repository };
}

function formatInstallError(err) {
  if (err.publicAuthoring) return `plugin-kit-ai npm bootstrap: ${err.message}`;
  return [
    `plugin-kit-ai npm bootstrap: ${err.message}`,
    "Fallbacks:",
    "- Homebrew: brew install 777genius/homebrew-plugin-kit-ai/plugin-kit-ai",
    "- Verified script: curl -fsSL https://raw.githubusercontent.com/777genius/plugin-kit-ai/main/scripts/install.sh | sh"
  ].join(os.EOL);
}

async function main() {
  try {
    await ensureInstalled({ packageRoot: path.resolve(__dirname, ".."), quiet: false });
  } catch (err) {
    process.stderr.write(formatInstallError(err) + os.EOL);
    process.exit(1);
  }
}

if (require.main === module) {
  main();
}

module.exports = {
  ensureInstalled,
  formatInstallError,
  latestTag,
  normalizeTag,
  parseChecksums,
  resolveRequestedTag
};
