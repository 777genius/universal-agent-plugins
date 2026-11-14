import { ClaudeApp, allow } from "./plugin-runtime.js";

const app = new ClaudeApp({
  allowedHooks: ["Stop", "PreToolUse", "UserPromptSubmit"],
  usage: "main.ts <Stop|PreToolUse|UserPromptSubmit>",
})
  .onStop((event) => {
    void event;
    return allow();
  })
  .onPreToolUse((event) => {
    void event;
    return allow();
  })
  .onUserPromptSubmit((event) => {
    void event;
    return allow();
  });

process.exit(app.run());
