#!/usr/bin/env node
"use strict";

// No downloads, installs, package metadata, or user configuration changes.
const { bootstrapProgress } = require("../lib/bootstrap-progress");
if (!process.stderr.isTTY) {
  process.stderr.write("Run this preview in an interactive terminal.\n");
  process.exitCode = 1;
} else {
  process.stderr.write("Startup UI preview — simulated network, no installation\n");
  const progress = bootstrapProgress(process.stderr, globalThis, process.env, process.argv.slice(2));
  let step = 0;
  const timer = setInterval(() => {
    step++;
    if (step >= 5 && step <= 25) progress.update({ received: (step - 5) * 1048576, total: 20 * 1048576 });
    if (step === 30) {
      clearInterval(timer);
      progress.stop();
      process.stderr.write("Preview finished. Nothing was installed.\n");
    }
  }, 250);
  process.once("SIGINT", () => {
    clearInterval(timer);
    progress.stop();
    process.stderr.write("Preview cancelled.\n");
    process.exitCode = 130;
  });
}
