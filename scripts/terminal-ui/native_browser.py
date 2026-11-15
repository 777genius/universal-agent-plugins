#!/usr/bin/env python3
"""Own the public CLI's PTY while Playwright drives its private browser URL."""
import json
import os
from pathlib import Path
import signal
import sys
import time
from types import SimpleNamespace

from harness import Session, check


def main():
    check(sys.platform == "linux", "browser/native proof is Linux only")
    for key, value in (("AGENTPLUGINS_NATIVE_BROWSER_LIVE", "1"),
                       ("AGENTPLUGINS_NATIVE_DISPOSABLE_HOSTED", "1"),
                       ("GITHUB_ACTIONS", "true"), ("RUNNER_ENVIRONMENT", "github-hosted")):
        check(os.environ.get(key) == value, "browser/native proof requires its hosted opt-in")
    root = Path(os.environ["AGENTPLUGINS_NATIVE_BROWSER_ROOT"]).resolve(strict=True)
    project = Path(os.environ["AGENTPLUGINS_NATIVE_BROWSER_PROJECT"]).resolve(strict=True)
    installer = Path(os.environ["AGENTPLUGINS_NATIVE_BROWSER_INSTALLER"]).resolve(strict=True)
    check(project.parent == root and installer.is_file(), "inexact browser/native fixture")
    private = root / "browser-private"
    private.mkdir(mode=0o700)
    opener = private / "xdg-open"
    ready, done = private / "ready.json", private / "done"
    # Only browser launch is redirected. Real Codex, Claude, Git, scanner and
    # installer commands keep their ordinary executable identities and runners.
    opener.write_text("#!/bin/sh\nprintf '%s' \"$1\" > \"$UAP_BROWSER_PRIVATE_URL\"\n")
    opener.chmod(0o700)
    env = dict(os.environ)
    env["PATH"] = str(private) + os.pathsep + env["PATH"]
    env["UAP_BROWSER_PRIVATE_URL"] = str(private / "url")
    evidence = root / "browser-terminal"
    evidence.mkdir(mode=0o700)
    session = Session([str(installer), "skills", "add",
                       env["AGENTPLUGINS_NATIVE_BROWSER_SKILL_SOURCE"], "--ui", "--scope", "user"],
                      SimpleNamespace(project=project, env=env), evidence, timeout=30, rows=35, cols=120)

    def interrupt(_number, _frame):
        raise KeyboardInterrupt("browser/native driver interrupted")

    signal.signal(signal.SIGTERM, interrupt)
    signal.signal(signal.SIGINT, interrupt)
    verified = False
    try:
        deadline = time.monotonic() + 240
        published = False
        while time.monotonic() < deadline:
            session.pump(0.05)
            check(session.process.poll() is None, "public CLI exited before browser completion")
            url_path = private / "url"
            if not published and url_path.exists():
                url = url_path.read_text()
                check(url.startswith("http://127.0.0.1:") and "#capability=" in url, "invalid private picker URL")
                # Atomic readiness publication avoids partial JSON observation.
                pending = private / "ready.pending"
                pending.write_text(json.dumps({"url": url}))
                pending.chmod(0o600)
                pending.replace(ready)
                published = True
            if done.exists():
                verified = done.read_text().strip() == "verify"
                break
        check(published, "public CLI did not open its browser picker")
        check(done.exists(), "browser/native fixture deadline exceeded")
        # The public --ui command intentionally waits while the tab is open.
        # End this owned interactive session explicitly and require graceful exit.
        session.send(b"\x03")
        session.finish(1)
        check(verified, "browser did not verify the installation")
    finally:
        session.close()
        ready.unlink(missing_ok=True)
        (private / "url").unlink(missing_ok=True)
    check(session.process.returncode == 1, "public CLI did not finish its session gracefully")


if __name__ == "__main__":
    main()
