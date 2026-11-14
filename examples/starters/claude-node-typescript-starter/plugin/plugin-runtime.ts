import fs from "node:fs";

export type JSONMap = Record<string, unknown>;
export type ClaudeHandler = (event: JSONMap) => JSONMap | void;

export function allow(): Record<string, never> {
  return {};
}

export class ClaudeApp {
  private readonly allowedHooks: Set<string>;
  private readonly usage: string;
  private readonly handlers = new Map<string, ClaudeHandler>();

  constructor(options: { allowedHooks: string[]; usage: string }) {
    this.allowedHooks = new Set(options.allowedHooks);
    this.usage = options.usage;
  }

  on(hookName: string, handler: ClaudeHandler): this {
    this.handlers.set(hookName, handler);
    return this;
  }

  onStop(handler: ClaudeHandler): this {
    return this.on("Stop", handler);
  }

  onPreToolUse(handler: ClaudeHandler): this {
    return this.on("PreToolUse", handler);
  }

  onUserPromptSubmit(handler: ClaudeHandler): this {
    return this.on("UserPromptSubmit", handler);
  }

  run(): number {
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
    const event = JSON.parse(fs.readFileSync(0, "utf8")) as JSONMap;
    const response = handler(event) ?? allow();
    if (Object.keys(response).length === 0) {
      process.stdout.write("{}");
    } else {
      process.stdout.write(JSON.stringify(response));
    }
    return 0;
  }
}
