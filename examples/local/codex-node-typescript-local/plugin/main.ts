import { CodexApp, continue_ } from "./plugin-runtime.js";

const app = new CodexApp().onNotify((event) => {
  void event;
  return continue_();
});

process.exit(app.run());
