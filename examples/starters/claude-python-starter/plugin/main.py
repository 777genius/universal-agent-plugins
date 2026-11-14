#!/usr/bin/env python3
from plugin_runtime import ClaudeApp, allow

app = ClaudeApp(
    allowed_hooks=["Stop", "PreToolUse", "UserPromptSubmit"],
    usage="main.py <Stop|PreToolUse|UserPromptSubmit>",
)


@app.on_stop
def on_stop(event):
    _ = event
    return allow()


@app.on_pre_tool_use
def on_pre_tool_use(event):
    _ = event
    return allow()


@app.on_user_prompt_submit
def on_user_prompt_submit(event):
    _ = event
    return allow()


if __name__ == "__main__":
    raise SystemExit(app.run())
