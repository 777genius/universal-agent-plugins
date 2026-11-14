#!/usr/bin/env python3
from plugin_runtime import CodexApp, continue_

app = CodexApp()


@app.on_notify
def on_notify(event):
    _ = event
    return continue_()


if __name__ == "__main__":
    raise SystemExit(app.run())
