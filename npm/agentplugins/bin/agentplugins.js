#!/usr/bin/env node
"use strict";

const path = require("node:path");
const { spawn } = require("node:child_process");

const { ensureBinary, formatBootstrapError } = require("../lib/bootstrap");

async function main() {
  let resolved;
  const progress = require("../lib/bootstrap-progress").bootstrapProgress(process.stderr, globalThis, process.env, process.argv.slice(2));
  const controller = new AbortController();
  let interrupted;
  const onInterrupt = () => cancelBootstrap("SIGINT");
  const onTerminate = () => cancelBootstrap("SIGTERM");
  const releaseSignals = () => {
    process.removeListener("SIGINT", onInterrupt);
    process.removeListener("SIGTERM", onTerminate);
  };
  function exitWithSignal(signal, force = false) {
    process.exitCode = signal === "SIGINT" ? 130 : 143;
    // Windows process.kill emulates a forced termination, not signal delivery.
    if (process.platform === "win32") {
      if (force) process.exit(process.exitCode);
    } else process.kill(process.pid, signal);
  }
  function cancelBootstrap(signal) {
    if (interrupted) {
      // A second signal is an explicit forced exit; never trap it in cleanup.
      releaseSignals();
      exitWithSignal(signal, true);
      return;
    }
    interrupted = signal;
    controller.abort();
  }
  process.on("SIGINT", onInterrupt);
  process.on("SIGTERM", onTerminate);
  try {
    resolved = await ensureBinary({ packageRoot: path.resolve(__dirname, ".."), onProgress: progress.update, onStatus: progress.status, signal: controller.signal });
  } catch (error) {
    progress.stop();
    process.stderr.write(interrupted ? `agentplugins npm launcher: ${error.message}\n` : formatBootstrapError(error) + "\n");
    process.exitCode = 1;
    return;
  } finally {
    // The acquisition promise settles only after its owned files and lock close.
    // Remove only our handlers before restoring the signal or native handoff.
    releaseSignals();
    progress.stop();
    if (interrupted) exitWithSignal(interrupted);
  }
  if (interrupted) return;
  const childEnvironment = resolved.publicAuthoring
    ? require("../lib/public-authoring").childEnvironment() : { ...process.env };
  delete childEnvironment.AGENTPLUGINS_INTERNAL_PROOF_MODE;
  delete childEnvironment.AGENTPLUGINS_INTERNAL_PROOF_BINARY;
  const child = spawn(resolved.binaryPath, process.argv.slice(2), { stdio: "inherit", env: childEnvironment });
  child.once("error", (error) => {
    process.stderr.write(`agentplugins npm launcher: ${error.message}\n`);
    process.exitCode = 1;
  });
  child.once("exit", (code, signal) => {
    if (signal) {
      process.kill(process.pid, signal);
      return;
    }
    process.exitCode = code === null ? 1 : code;
  });
}

main();
