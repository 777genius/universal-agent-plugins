#!/usr/bin/env node
"use strict";

const path = require("path");
const { spawn } = require("child_process");

const { ensureInstalled, formatInstallError } = require("../lib/install");

async function main() {
  let installResult;
  try {
    installResult = await ensureInstalled({ packageRoot: path.resolve(__dirname, ".."), quiet: true });
  } catch (err) {
    process.stderr.write(formatInstallError(err) + "\n");
    process.exit(1);
    return;
  }

  const child = spawn(installResult.installedBinary, process.argv.slice(2), {
    stdio: "inherit",
    env: installResult.publicAuthoring ? require("../lib/public-authoring").childEnvironment() : process.env
  });
  child.on("error", (err) => {
    process.stderr.write(`plugin-kit-ai npm launcher: ${err.message}\n`);
    process.exit(1);
  });
  child.on("exit", (code, signal) => {
    if (signal) {
      process.kill(process.pid, signal);
      return;
    }
    process.exit(code === null ? 1 : code);
  });
}

main();
