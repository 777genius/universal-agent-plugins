export type JSONMap = Record<string, unknown>;
export type CodexHandler = (event: JSONMap) => number | void;

export function continue_(): number {
  return 0;
}

export class CodexApp {
  private notifyHandler?: CodexHandler;

  onNotify(handler: CodexHandler): this {
    this.notifyHandler = handler;
    return this;
  }

  run(): number {
    const hookName = process.argv[2];
    if (hookName !== "notify") {
      process.stderr.write("usage: main.ts notify <json-payload>\n");
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
    const event = JSON.parse(payload) as JSONMap;
    return this.notifyHandler(event) ?? continue_();
  }
}
