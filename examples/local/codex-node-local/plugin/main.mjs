import { CodexApp, continue_ } from "./plugin-runtime.mjs";

const app = new CodexApp().onNotify((event) => {
  void event;
  return continue_();
});

process.exit(app.run());
