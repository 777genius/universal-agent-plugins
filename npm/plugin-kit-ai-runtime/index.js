import fs from "node:fs";

export const CLAUDE_STABLE_HOOKS = [
  "Stop",
  "PreToolUse",
  "UserPromptSubmit"
];

export const CLAUDE_EXTENDED_HOOKS = [
  "Stop",
  "PreToolUse",
  "UserPromptSubmit",
  "SessionStart",
  "SessionEnd",
  "Notification",
  "PostToolUse",
  "PostToolUseFailure",
  "PermissionRequest",
  "SubagentStart",
  "SubagentStop",
  "PreCompact",
  "Setup",
  "TeammateIdle",
  "TaskCompleted",
  "ConfigChange",
  "WorktreeCreate",
  "WorktreeRemove"
];

export function allow() {
  return {};
}

export function continue_() {
  return 0;
}

export class ClaudeApp {
  constructor(options) {
    this.allowedHooks = new Set(options.allowedHooks);
    this.usage = options.usage;
    this.handlers = new Map();
  }

  on(hookName, handler) {
    this.handlers.set(hookName, handler);
    return this;
  }

  onStop(handler) {
    return this.on("Stop", handler);
  }

  onPreToolUse(handler) {
    return this.on("PreToolUse", handler);
  }

  onUserPromptSubmit(handler) {
    return this.on("UserPromptSubmit", handler);
  }

  run() {
    const hookName = process.argv[2];
    if (!hookName || !this.allowedHooks.has(hookName)) {
      process.stderr.write(`usage: ${this.usage}\n`);
      return 1;
    }
    const handler = this.handlers.get(hookName);
    if (!handler) {
      process.stderr.write(`no handler registered for ${hookName}\n`);
      return 1;
    }
    const event = JSON.parse(fs.readFileSync(0, "utf8"));
    const response = handler(event) ?? allow();
    if (Object.keys(response).length === 0) {
      process.stdout.write("{}");
    } else {
      process.stdout.write(JSON.stringify(response));
    }
    return 0;
  }
}

export class CodexApp {
  onNotify(handler) {
    this.notifyHandler = handler;
    return this;
  }

  run() {
    const hookName = process.argv[2];
    if (hookName !== "notify") {
      process.stderr.write("usage: main.js notify <json-payload>\n");
      return 1;
    }
    const payload = process.argv[3];
    if (!payload) {
      process.stderr.write("missing notify payload\n");
      return 1;
    }
    if (!this.notifyHandler) {
      process.stderr.write("no handler registered for notify\n");
      return 1;
    }
    const event = JSON.parse(payload);
    return this.notifyHandler(event) ?? continue_();
  }
}
