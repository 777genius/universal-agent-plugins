"use strict";
const assert = require("node:assert/strict");
const { EventEmitter } = require("node:events");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");
const test = require("node:test");

function launcher(overrides = {}) {
  const events = [], kills = [], exits = [], spawned = [];
  const entry = path.resolve(__dirname, "../bin/agentplugins.js");
  const child = new EventEmitter();
  const processStub = Object.assign(new EventEmitter(), {
    argv: ["node", entry, "skills", "add", "acme/skills"], env: {}, pid: 123, platform: "linux",
    stderr: { write: line => events.push(line) },
    kill: (pid, signal) => kills.push([pid, signal]),
    exit: code => exits.push(code)
  });
  let acquisition;
  return {
    events, kills, exits, spawned, child, process: processStub,
    get acquisition() { return acquisition; },
    run() {
      return vm.runInNewContext(fs.readFileSync(entry, "utf8"), {
        __dirname: path.dirname(entry), process: processStub, AbortController,
        require(name) {
          if (name === "../lib/bootstrap") return {
            ensureBinary: options => {
              acquisition = options;
              return overrides.ensureBinary ? overrides.ensureBinary(options) :
                Promise.resolve({ binaryPath: "/fixture/native" });
            },
            formatBootstrapError: error => error.message
          };
          if (name === "../lib/bootstrap-progress") return {
            bootstrapProgress: () => ({ update() {}, status() {}, stop() { events.push("clear"); } })
          };
          if (name === "node:child_process") return {
            spawn(...args) { spawned.push(args); overrides.onSpawn?.(); return child; }
          };
          return require(name);
        }
      });
    }
  };
}

test("launcher clears the live line before reporting bootstrap errors", async () => {
  const fixture = launcher({ ensureBinary: async () => { throw new Error("network unavailable"); } });
  await fixture.run();
  assert.deepEqual(fixture.events, ["clear", "network unavailable\n", "clear"]);
  assert.equal(fixture.process.exitCode, 1);
  assert.equal(fixture.spawned.length, 0);
  assert.equal(fixture.process.listenerCount("SIGINT"), 0);
  assert.equal(fixture.process.listenerCount("SIGTERM"), 0);
});

test("bootstrap cancellation waits for acquisition cleanup and never launches native", async () => {
  for (const signal of ["SIGINT", "SIGTERM"]) {
    let finish;
    const fixture = launcher({ ensureBinary: () => new Promise((resolve, reject) => { finish = reject; }) });
    const completed = fixture.run();
    fixture.process.emit(signal);
    assert.equal(fixture.acquisition.signal.aborted, true);
    assert.deepEqual(fixture.kills, [], "must await the acquisition owner's cleanup");
    finish(new Error("binary acquisition cancelled"));
    await completed;
    assert.deepEqual(fixture.kills, [[123, signal]]);
    assert.equal(fixture.process.exitCode, signal === "SIGINT" ? 130 : 143);
    assert.equal(fixture.spawned.length, 0);
    assert.equal(fixture.process.listenerCount("SIGINT"), 0);
    assert.equal(fixture.process.listenerCount("SIGTERM"), 0);
  }
});

test("cancellation during a successful cache check still prevents native handoff", async () => {
  let finish;
  const fixture = launcher({ ensureBinary: () => new Promise(resolve => { finish = resolve; }) });
  const completed = fixture.run();
  fixture.process.emit("SIGINT");
  finish({ binaryPath: "/fixture/native", cacheHit: true });
  await completed;
  assert.equal(fixture.spawned.length, 0);
  assert.deepEqual(fixture.kills, [[123, "SIGINT"]]);
  assert.equal(fixture.process.exitCode, 130);
});

test("a second signal can force exit while bootstrap cleanup is pending", () => {
  const fixture = launcher({ ensureBinary: () => new Promise(() => {}) });
  void fixture.run();
  fixture.process.emit("SIGINT");
  fixture.process.emit("SIGTERM");
  assert.deepEqual(fixture.kills, [[123, "SIGTERM"]]);
  assert.equal(fixture.process.exitCode, 143);
  assert.equal(fixture.process.listenerCount("SIGINT"), 0);
  assert.equal(fixture.process.listenerCount("SIGTERM"), 0);
  assert.equal(fixture.spawned.length, 0);
});

test("native handoff removes only bootstrap handlers and preserves child results", async () => {
  for (const signal of [null, "SIGTERM"]) {
    const retained = () => {};
    const fixture = launcher({ onSpawn() {
      assert.deepEqual(fixture.process.listeners("SIGINT"), [retained]);
      assert.equal(fixture.process.listenerCount("SIGTERM"), 0);
    } });
    fixture.process.on("SIGINT", retained);
    await fixture.run();
    assert.equal(fixture.spawned.length, 1);
    assert.equal(fixture.spawned[0][2].stdio, "inherit");
    fixture.child.emit("exit", signal ? null : 23, signal);
    if (signal) assert.deepEqual(fixture.kills, [[123, signal]]);
    else assert.equal(fixture.process.exitCode, 23);
  }
});

test("Windows bootstrap cancellation retains conventional exit codes after cleanup", async () => {
  let finish;
  const fixture = launcher({ ensureBinary: () => new Promise((resolve, reject) => { finish = reject; }) });
  fixture.process.platform = "win32";
  const completed = fixture.run();
  fixture.process.emit("SIGINT");
  finish(new Error("binary acquisition cancelled"));
  await completed;
  assert.equal(fixture.process.exitCode, 130);
  assert.deepEqual(fixture.kills, []);
  assert.deepEqual(fixture.exits, []);
  assert.equal(fixture.spawned.length, 0);
  const forced = launcher({ ensureBinary: () => new Promise(() => {}) });
  forced.process.platform = "win32";
  void forced.run();
  forced.process.emit("SIGINT");
  forced.process.emit("SIGTERM");
  assert.deepEqual(forced.exits, [143]);
  assert.equal(forced.process.listenerCount("SIGINT"), 0);
  assert.equal(forced.process.listenerCount("SIGTERM"), 0);
});
