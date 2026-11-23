#!/usr/bin/env python3
"""Drive the ordinary signed Context7 Directory install in the native fixture."""
import argparse
import hashlib
import os
from pathlib import Path
import re
import signal
import stat
import sys
import time
from types import SimpleNamespace

from harness import Screen, check, clean
from native_skills import (LIMIT, NativeSession, choose_targets, failure_screen,
                           hashes, key, preserve, screen, wait_screen)


PLUGIN = "context7"
TARGETS = ("claude", "codex")
LABELS = {"claude": "Claude Code", "codex": "OpenAI Codex"}
QUESTION = "Apply these changes? [y/N]"
ACK = re.compile(rb"(?:^|[\r\n])Changes confirmed\.\r?\n")
ACTIVITY = re.compile(rb"\r\x1b\[2K([^\r\n]*)(?=[\r\n])")
RESULT = re.compile(r"(?:^|[\r\n])Install ([^\r\n]+?) for ([^\r\n:]+): ([^\s]+)[^\r\n]*(?=[\r\n]|$)")
INSTALLATION = re.compile(rf"(?m)^Installation: ([^/\s]+) \({re.escape(PLUGIN)}\)\r?$")


def arguments(installer):
    # The Directory card's saved add path: selection and consent are keyboard input.
    return [str(installer), "add", PLUGIN]


def protected_state(root, env):
    codex, claude, state = (Path(env[name]) for name in
                            ("CODEX_HOME", "CLAUDE_CONFIG_DIR", "AGENTPLUGINS_HOME"))
    for path in (codex, claude, state):
        check(path.is_absolute() and path == path.resolve(strict=True) and path.is_relative_to(root),
              "native plugin profile escaped the selected fixture")
    config = codex / "config.toml"
    check(stat.S_ISREG(config.lstat().st_mode), "Codex fixture config is not a regular file")
    absent = [state / name for name in ("state-v2.json", "uap.toml", "uap.lock")]
    for path in absent:
        check(not os.path.lexists(path), "Directory intent or installation exists before confirmation: " + str(path))
    return {"native_profiles": {str(path): hashes(path) for path in (codex, claude)},
            "codex_config_sha256": hashlib.sha256(config.read_bytes()).hexdigest(),
            "absent": [str(path) for path in absent]}


def review(session):
    view = wait_screen(session, "plugin-review", lambda value: QUESTION in value and
                       sorted(re.findall(r"(?m)^Install ([^\r\n]+?) for ([^\r\n:]+)(?::[^\r\n]*)?\r?$", value)) ==
                       [(PLUGIN, LABELS[target]) for target in TARGETS])
    check(ACK.search(bytes(session.raw)) is None, "Directory changes acknowledged before confirmation")
    return view


def wait_progress(session, after):
    check(0 <= after <= len(session.raw), "invalid plugin confirmation offset")
    deadline = time.monotonic() + session.timeout
    drained, acknowledged = False, False
    while drained or time.monotonic() < deadline:
        received = drained or session.pump(0.1)
        drained = False
        check(len(session.raw) <= LIMIT, "terminal transcript exceeded 1 MiB")
        raw = bytes(session.raw)
        acknowledgments = list(ACK.finditer(raw, after))
        check(len(acknowledgments) <= 1, "duplicate Directory confirmation acknowledgment")
        acknowledgment = acknowledgments[0] if acknowledgments else None
        activity = raw.find(b"Applying changes", after)
        if activity >= 0:
            check(acknowledgment is not None and acknowledgment.end() <= activity,
                  "plugin progress before CLI confirmation acknowledgment")
        if acknowledgment is not None and not acknowledged:
            session.events.append({"event": "confirmation-accepted", "offset": acknowledgment.end()})
            acknowledged = True
        for frame in ACTIVITY.finditer(raw, after):
            visible = clean(frame[1])
            if re.fullmatch(r"\S Applying changes · [0-9]+s", visible) is None:
                continue
            check(acknowledgment is not None and acknowledgment.end() <= frame.start(),
                  "plugin activity frame preceded confirmation acknowledgment")
            check(RESULT.search(clean(raw[after:frame.start()])) is None,
                  "plugin completed before an observed progress frame")
            rendered = Screen(session.screen.rows, session.screen.cols)
            rendered.feed(raw[:frame.end()].replace(b"\x1bM", b"\x1b[1A"))
            check(re.search(r"\S Applying changes · [0-9]+s", rendered.snapshot()) is not None,
                  "plugin progress was not visible in the rendered frame")
            latest = session.screen
            try:
                session.screen = rendered
                session.frame("plugin-progress")
                session.events[-1]["offset"] = frame.end()
            finally:
                session.screen = latest
            return frame.end()
        check(RESULT.search(clean(raw[after:])) is None, "plugin completed without an observed progress frame")
        if session.process.poll() is not None and not received:
            if time.monotonic() < deadline and session.pump(0):
                drained = True
                continue
            raise AssertionError("public CLI exited before plugin-progress")
    raise AssertionError("timeout waiting for terminal screen: plugin-progress; last rendered screen=" +
                         failure_screen(session))


def completed_result(raw, after):
    check(0 <= after <= len(raw), "invalid plugin completion offset")
    text = clean(raw[after:])
    results = RESULT.findall(text)
    check(len(results) == 2 and sorted((plugin, client) for plugin, client, _ in results) ==
          [(PLUGIN, LABELS[target]) for target in TARGETS],
          "Directory result did not cover exactly the plugin and two selected agents")
    installation_ids = INSTALLATION.findall(text)
    check(len(installation_ids) == 1 and len(re.findall(r"(?m)^Installation:", text)) == 1
          and all(outcome == "completed" for _, _, outcome in results),
          "Directory result lost one saved installation or did not complete")
    # These are real operation outcomes, not evidence that MCP/auth is ready.
    return {"installation_id": installation_ids[0],
            "manual_or_next_actions": [line.strip() for line in text.splitlines()
                                       if line.strip().startswith(("Manual:", "Next:"))]}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("installer", "root", "project"):
        parser.add_argument("--" + name, required=True)
    args = parser.parse_args()
    check(sys.platform == "linux", "native plugin keyboard proof is Linux only")
    for name, expected in (("AGENTPLUGINS_NATIVE_DISPOSABLE_HOSTED", "1"),
                           ("GITHUB_ACTIONS", "true"), ("RUNNER_ENVIRONMENT", "github-hosted")):
        check(os.environ.get(name) == expected, "native plugin keyboard proof requires its hosted opt-in")
    root, project, installer = (Path(value).resolve(strict=True)
                                for value in (args.root, args.project, args.installer))
    check(project.parent == root and installer.is_file(), "inexact native plugin fixture")
    evidence = root / "terminal-plugin-ui"
    evidence.mkdir(mode=0o700)
    baseline = protected_state(root, os.environ)
    argv = arguments(installer)
    report = {"schema_version": 1, "status": "failed", "plugin": PLUGIN, "scope": "user", "argv": argv,
              "selected_targets": list(TARGETS), "confirmations": 0, "phase": "selection",
              "unchanged_before_confirmation": False, "progress_observed": False,
              "installation_completed": False, "model_request": False,
              "authentication_qualified": False, "native_readiness_qualified": False}
    session = NativeSession(argv, SimpleNamespace(project=project, env=dict(os.environ)), evidence,
                            timeout=120, rows=35, cols=120)

    def interrupt(_number, _frame):
        raise KeyboardInterrupt("native plugin keyboard proof interrupted")

    signal.signal(signal.SIGTERM, interrupt)
    signal.signal(signal.SIGINT, interrupt)
    try:
        choose_targets(session)
        report["phase"] = "review"
        review(session)
        check(protected_state(root, os.environ) == baseline,
              "native Directory state changed before confirmation")
        report["preconfirmation"] = baseline
        report["unchanged_before_confirmation"] = True
        confirmed_at = len(session.raw)
        key(session, b"y\r")  # The ordinary saved-add confirmation defaults to No.
        report["confirmations"], report["phase"] = 1, "install"
        progress_at = wait_progress(session, confirmed_at)
        report["progress_observed"] = True
        session.finish(0)
        report["result"] = completed_result(session.raw, progress_at)
        screen(session)
        session.frame("plugin-complete")
        report["installation_completed"], report["phase"] = True, "complete"
    except BaseException as error:
        report["error"] = str(error)
        raise
    finally:
        closed = False
        try:
            session.close()
            check(session.owner.returncode == 0, "terminal owner did not exit successfully")
            closed = True
        except BaseException as error:
            report["closure_error"] = repr(error)[:512]
            report["status"] = "failed"
            raise
        finally:
            if closed and report["installation_completed"] and "error" not in report:
                report["status"] = "passed"
            preserve(session, report, root, evidence, closed=closed, name="plugin")
    check(report["status"] == "passed" and report["closure_verified"],
          "native plugin keyboard evidence failed or terminal was not restored and closed")


if __name__ == "__main__":
    main()
