"use strict";

// Own stderr until the native process starts; never repaint over its prompts.
function bootstrapProgress(stream = process.stderr, timers = globalThis, environment = process.env, args = []) {
  const flags = args.includes("--") ? args.slice(0, args.indexOf("--")) : args;
  let mode = "auto", format = "human";
  for (let i = 0; i < flags.length; i++) {
    if (flags[i] === "--no-color") mode = "never";
    else if (flags[i].startsWith("--color=")) mode = flags[i].slice(8);
    else if (flags[i] === "--color") mode = flags[++i];
    else if (flags[i].startsWith("--format=")) format = flags[i].slice(9);
    else if (flags[i] === "--format") format = flags[++i];
  }
  if (!stream.isTTY || format === "json") return { update() {}, status() {}, stop() {} };
  const live = environment.TERM !== "dumb" && !flags.includes("--plain");
  const color = live && mode !== "never" && (mode === "always" || !environment.NO_COLOR);
  // Match terminaltheme: Label=cyan, Success=green, Muted=bright black.
  const paint = (code, text) => color ? `\x1b[${code}m${text}\x1b[0m` : text;
  const clip = (segments, columns) => {
    let remaining = columns - 1;
    return segments.map(([code, text]) => {
      const visible = Array.from(text).slice(0, Math.max(0, remaining)).join("");
      remaining -= Array.from(visible).length;
      return visible ? paint(code, visible) : "";
    }).join("");
  };
  let stage = "Preparing native CLI (first run may download from GitHub)";
  let download;
  let frame = 0;
  let stopped = false;
  const started = Date.now();
  const frames = ["⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"];
  const render = () => {
    const elapsed = Math.floor((Date.now() - started) / 1000);
    const columns = Math.max(1, stream.columns || 80);
    const segments = [["36", `${frames[frame++ % frames.length]} `]];
    if (download) {
      const percent = Math.floor(download.received / download.total * 100);
      const cells = columns >= 90 ? 20 : 12;
      const count = Math.floor(percent / 100 * cells);
      segments.push(["1;36", "Downloading CLI "]);
      if (columns >= 65) segments.push(["32", "━".repeat(count)], ["90", "─".repeat(cells - count)], ["0", " "]);
      segments.push(["1;36", `${percent}%`], ["90", `  ${(download.received / 1048576).toFixed(1)} / ${(download.total / 1048576).toFixed(1)} MB`]);
    } else {
      segments.push(["1;36", stage]);
    }
    segments.push(["90", ` · ${elapsed}s`]);
    stream.write(`\r\x1b[2K${clip(segments, columns)}`);
  };
  if (live) render();
  else stream.write(`agentplugins: ${stage}\n`);
  const timer = timers.setInterval(() => {
    if (stopped) return;
    if (live) render();
    else if (++frame % 50 === 0) stream.write(`agentplugins: ${stage}\n`);
  }, 100);
  timer.unref?.();
  return {
    status(message) {
      if (stopped) return;
      stage = message;
      download = undefined;
    },
    update({ received, total }) {
      if (stopped || !Number.isFinite(received) || !Number.isFinite(total) || total <= 0) return;
      download = received < total ? { received: Math.max(0, received), total } : undefined;
      stage = download ? "Downloading native CLI from GitHub" : "Download complete; verifying native CLI";
    },
    stop() {
      if (stopped) return;
      stopped = true;
      timers.clearInterval(timer);
      if (live) stream.write("\r\x1b[2K");
    }
  };
}

module.exports = { bootstrapProgress };
