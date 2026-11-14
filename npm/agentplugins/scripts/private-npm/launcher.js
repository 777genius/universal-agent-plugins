"use strict";

const { spawn } = require("node:child_process");
const { ensureBinary } = require("./bootstrap");
const PRODUCTS = ["agentplugins", "plugin-kit-ai"];
const MODE = "release-cli-contract-v1";
const SHUTDOWN_MS = 1500;

function childEnvironment(env) {
  return Object.fromEntries(Object.entries(env).filter(([key]) =>
    !/^(UAP_PRIVATE_NPM_|UAP_CANDIDATE_|AGENTPLUGINS_INTERNAL_PROOF_|PLUGIN_KIT_AI_INTERNAL_PROOF_)/i.test(key)));
}

function target() {
  const os = { linux: "linux", darwin: "darwin", win32: "windows" }[process.platform];
  const arch = { x64: "amd64", arm64: "arm64" }[process.arch];
  if (!os || !arch) throw new Error("unsupported private platform");
  return `${os}-${arch}`;
}

// Internal seams allow deterministic race tests; shims never accept hooks.
async function supervise(product, packageRoot, hooks = {}) {
  if (!PRODUCTS.includes(product)) throw new Error("unknown fixed private product");
  const host = hooks.host || process;
  const controller = new AbortController();
  let child, requested, forwarded = false, closed = false, timer, failure;
  const forward = () => {
    if (!requested || !child?.pid || closed || forwarded) return;
    forwarded = true;
    try { child.kill(requested); } catch (error) { failure ||= error; }
    // No process groups: the offline author contract starts no grandchildren.
    timer = setTimeout(() => {
      if (!closed) try { child.kill("SIGKILL"); } catch (error) { failure ||= error; }
    }, SHUTDOWN_MS);
  };
  const cancel = signal => {
    requested ||= signal;
    controller.abort();
    forward();
  };
  const onInt = () => cancel("SIGINT"), onTerm = () => cancel("SIGTERM");
  host.on("SIGINT", onInt); host.on("SIGTERM", onTerm);
  try {
    const resolved = await (hooks.acquire || ensureBinary)(product, {
      packageRoot, cacheRoot: host.env.UAP_PRIVATE_NPM_CACHE,
      candidateRoot: host.env.UAP_PRIVATE_NPM_CANDIDATE, target: target(),
      expectedMode: MODE, signal: controller.signal
    });
    if (requested) return { signal: requested };
    const outcome = await new Promise(resolve => {
      try {
        child = (hooks.spawn || spawn)(resolved.binaryPath, host.argv.slice(2), {
          cwd: host.cwd(), shell: false, stdio: "inherit", env: childEnvironment(host.env)
        });
      } catch (error) { failure = error; resolve({ code: 1 }); return; }
      // 'close' follows 'exit'/'error' and stdio closure. Never finish at exit.
      child.once("error", error => { failure ||= error; });
      child.once("spawn", forward);
      child.once("close", (code, signal) => {
        closed = true;
        resolve(signal ? { signal } : { code: Number.isInteger(code) ? code : 1 });
      });
      forward(); // Covers cancellation inside spawn, before its event.
    });
    if (failure) throw failure;
    return outcome;
  } catch (error) {
    // Acquisition has awaited its owned lock/stage cleanup before rejection.
    // Report cleanup uncertainty even when a parent signal caused cancellation.
    const message = String(error?.message || "launcher failure").replace(/[\x00-\x1f\x7f]/g, " ").slice(0, 512);
    host.stderr.write(`${product} private npm launcher: ${message}\n`);
    return requested && !child ? { signal: requested } : { code: 1 };
  } finally {
    clearTimeout(timer);
    host.removeListener("SIGINT", onInt); host.removeListener("SIGTERM", onTerm);
  }
}

async function main(product, packageRoot) {
  const result = await supervise(product, packageRoot);
  if (result.signal) {
    // Handlers are gone; POSIX reflection must not enter our forwarding path.
    try { process.kill(process.pid, result.signal); }
    catch { process.exitCode = 1; }
  } else process.exitCode = result.code;
}

module.exports = { main, supervise, childEnvironment, SHUTDOWN_MS };
