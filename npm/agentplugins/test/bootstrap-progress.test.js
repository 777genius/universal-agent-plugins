"use strict";
const assert = require("node:assert/strict");
const test = require("node:test");
const { bootstrapProgress } = require("../lib/bootstrap-progress");

function terminal(columns = 80, environment = {}) {
  const lines = [];
  let tick, cleared = false;
  const timer = { unref() {} };
  const stream = { isTTY: true, columns, write: line => lines.push(line) };
  const timers = {
    setInterval(callback, delay) { assert.equal(delay, 100); tick = callback; return timer; },
    clearInterval(value) { assert.equal(value, timer); cleared = true; }
  };
  return { lines, stream, timers, environment, tick() { if (!cleared) tick(); }, get cleared() { return cleared; } };
}

test("cold start renders immediately, animates in place, shows bytes and clears before handoff", () => {
  const ui = terminal();
  const progress = bootstrapProgress(ui.stream, ui.timers, ui.environment);
  assert.match(ui.lines[0], /Preparing native CLI/);
  assert.match(ui.lines[0], /\x1b\[1;36m/, "uses the shared cyan label palette");
  ui.tick();
  assert.notEqual(ui.lines[0], ui.lines[1], "spinner advances without waiting for network callbacks");
  progress.update({ received: 1048576, total: 4194304 });
  ui.tick();
  const visible = ui.lines.at(-1).replace(/\x1b\[[0-9;]*m/g, "");
  assert.match(visible, /Downloading CLI ━━━───────── 25%  1.0 \/ 4.0 MB/);
  assert.match(ui.lines.at(-1), /\x1b\[32m/, "download fill uses semantic success green");
  progress.update({ received: 4194304, total: 4194304 });
  ui.tick();
  assert.match(ui.lines.at(-1), /verifying native CLI/);
  progress.stop();
  assert.equal(ui.lines.at(-1), "\r\x1b[2K");
  assert.equal(ui.cleared, true);
  const count = ui.lines.length;
  progress.stop();
  ui.tick();
  assert.equal(ui.lines.length, count);
  assert.ok(ui.lines.every(line => !line.includes("\n")), "live output never scrolls the terminal");
});

test("narrow terminal clips output and does not wrap; resizes are read each frame", () => {
  const ui = terminal(30);
  const progress = bootstrapProgress(ui.stream, ui.timers, {});
  progress.update({ received: 50, total: 100 });
  ui.tick();
  assert.ok(Array.from(ui.lines.at(-1).replace(/\x1b\[[0-9;]*[Km]/g, "").replace(/\r/g, "")).length < 30);
  ui.stream.columns = 100;
  ui.tick();
  assert.match(ui.lines.at(-1), /50%/);
  progress.stop();
});

test("dumb terminal uses plain status and no escapes", () => {
  const ui = terminal();
  const progress = bootstrapProgress(ui.stream, ui.timers, { TERM: "dumb" });
  for (let i = 0; i < 50; i++) ui.tick();
  progress.stop();
  assert.equal(ui.lines.length, 2);
  assert.ok(ui.lines.every(line => line.endsWith("\n") && !line.includes("\x1b")));
});

test("non-TTY acquisition never writes output or creates a timer", () => {
  const progress = bootstrapProgress({ isTTY: false, write() { assert.fail("unexpected output"); } }, {
    setInterval() { assert.fail("unexpected timer"); }
  });
  progress.update({ received: 1, total: 2 });
  progress.stop();
});

test("NO_COLOR preserves animation without color sequences", () => {
  const ui = terminal();
  const progress = bootstrapProgress(ui.stream, ui.timers, { NO_COLOR: "1" });
  progress.update({ received: 50, total: 100 });
  ui.tick();
  progress.stop();
  assert.ok(ui.lines.every(line => !/\x1b\[[0-9;]*m/.test(line)));
  assert.match(ui.lines[1], /50%/);
});

test("bootstrap respects native color/plain/JSON presentation flags", () => {
  for (const args of [["--no-color"], ["--color=never"], ["--color", "never"]]) {
    const ui = terminal();
    const progress = bootstrapProgress(ui.stream, ui.timers, {}, args);
    progress.stop();
    assert.ok(ui.lines.every(line => !/\x1b\[[0-9;]*m/.test(line)));
  }
  const ui = terminal();
  const progress = bootstrapProgress(ui.stream, ui.timers, {}, ["--plain"]);
  progress.stop();
  assert.ok(ui.lines.every(line => !line.includes("\x1b")));
  for (const args of [["--format=json"], ["--format", "json"]]) {
    const quiet = bootstrapProgress({ isTTY: true, write() { assert.fail("JSON must stay quiet"); } }, {
      setInterval() { assert.fail("JSON must not animate"); }
    }, {}, args);
    quiet.stop();
  }
  const forced = terminal();
  bootstrapProgress(forced.stream, forced.timers, { NO_COLOR: "1" }, ["--color=always"]).stop();
  assert.match(forced.lines[0], /\x1b\[36m/);
});
