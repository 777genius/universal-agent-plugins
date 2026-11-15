#!/usr/bin/env node
"use strict";

const path = require("node:path");
const { spawn } = require("node:child_process");

const { ensureBinary, formatBootstrapError } = require("../lib/bootstrap");

async function main() {
  let resolved;
  const progress = require("../lib/bootstrap-progress").bootstrapProgress(process.stderr, globalThis, process.env, process.argv.slice(2));
  try {
    resolved = await ensureBinary({ packageRoot: path.resolve(__dirname, ".."), onProgress: progress.update, onStatus: progress.status });
  } catch (error) {
    progress.stop();
    process.stderr.write(formatBootstrapError(error) + "\n");
    process.exitCode = 1;
    return;
  } finally {
    progress.stop();
  }
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
