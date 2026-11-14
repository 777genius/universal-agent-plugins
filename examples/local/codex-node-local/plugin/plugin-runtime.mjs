export function continue_() {
  return 0;
}

export class CodexApp {
  onNotify(handler) {
    this.notifyHandler = handler;
    return this;
  }

  run() {
    const hookName = process.argv[2];
    if (hookName !== "notify") {
      process.stderr.write("usage: main.mjs notify <json-payload>\n");
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
