"use strict";
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");
const test = require("node:test");

test("launcher clears the live line before reporting bootstrap errors", async () => {
  const events = [];
  const entry = path.resolve(__dirname, "../bin/agentplugins.js");
  const processStub = { argv: ["node", entry, "skills", "add", "acme/skills"], env: {},
    stderr: { write: line => events.push(line) } };
  await vm.runInNewContext(fs.readFileSync(entry, "utf8"), {
    __dirname: path.dirname(entry), process: processStub,
    require(name) {
      if (name === "../lib/bootstrap") return {
        ensureBinary: async () => { throw new Error("network unavailable"); },
        formatBootstrapError: error => error.message
      };
      if (name === "../lib/bootstrap-progress") return {
        bootstrapProgress: () => ({ update() {}, stop() { events.push("clear"); } })
      };
      if (name === "node:child_process") return { spawn() { assert.fail("no native process on error"); } };
      return require(name);
    }
  });
  assert.deepEqual(events, ["clear", "network unavailable\n", "clear"]);
  assert.equal(processStub.exitCode, 1);
});
