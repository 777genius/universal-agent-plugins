#!/usr/bin/env python3
"""Drive one ordinary Skills keyboard install with the hosted native clients."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import signal
import sys
import time
from types import SimpleNamespace

from harness import Session, Screen, check, clean, hashes


SKILLS = ("docx", "frontend-design")
TARGETS = ("claude", "codex")
LIMIT = 1 << 20


def arguments(installer, project, source):
    return [str(installer), "skills", "add", source, "--no-ui", "--scope", "project",
            "--project-root", str(project)]


def compact(text):
    # Review boxes and physical wrapping are presentation, not path authority.
    return re.sub(r"[\s│┃╭╮╰╯┌┐└┘─━]", "", text)


def screen(session):
    check(len(session.raw) <= LIMIT, "terminal transcript exceeded 1 MiB")
    # Same evidence-renderer normalization as selection_matrix.Keyboard; the
    # actual PTY bytes and input remain untouched.
    rendered = Screen(session.screen.rows, session.screen.cols)
    rendered.feed(bytes(session.raw).replace(b"\x1bM", b"\x1b[1A"))
    session.screen = rendered
    return rendered.snapshot()


def wait_screen(session, label, predicate):
    deadline = time.monotonic() + session.timeout
    while time.monotonic() < deadline:
        session.pump(0.1)
        view = screen(session)
        if predicate(view):
            session.frame(label)
            return view
        check(session.process.poll() is None, "public CLI exited before " + label)
    raise AssertionError("timeout waiting for terminal screen: " + label)


def wait_install_progress(session, after):
    # groupProgressBoard writes two newline-terminated agent rows per frame.
    # Inspect those boundaries in byte order, even when several frames and the
    # final result arrive in one PTY read. Never count a pre-confirmation frame.
    check(0 <= after <= len(session.raw), "invalid confirmation transcript offset")
    rendered = Screen(session.screen.rows, session.screen.cols)
    offset, rendered_to, rows = after, 0, {}
    deadline = time.monotonic() + session.timeout
    while time.monotonic() < deadline:
        received = session.pump(0.1)
        check(len(session.raw) <= LIMIT, "terminal transcript exceeded 1 MiB")
        raw = bytes(session.raw)
        while (end := raw.find(b"\n", offset)) >= 0:
            end += 1
            rendered.feed(raw[rendered_to:end].replace(b"\x1bM", b"\x1b[1A"))
            chunk = raw[offset:end]
            if b"\x1b[2A" in chunk:  # Start of the next live two-row board.
                rows.clear()
            line = clean(chunk).strip()
            offset = rendered_to = end
            check("Skills installation verified." not in line,
                  "terminal verified before an observed post-confirmation progress frame")
            row = re.fullmatch(r"(Claude Code|OpenAI Codex)\s+[·●✓!×] [0-2]/2 [^\r\n]+", line)
            if row is None:
                rows.clear()
                continue
            if row[1] in rows:
                rows.clear()
            rows[row[1]] = line
            if len(rows) != 2:
                continue
            active = re.search(r"\b[01]/2 (preparing|installing)\b", "\n".join(rows.values()))
            rows.clear()
            view = rendered.snapshot()
            if active and "Claude Code" in view and "OpenAI Codex" in view and \
                    re.search(r"\b[01]/2 (preparing|installing)\b", view) is not None and \
                    "Skills installation verified." not in view:
                latest = session.screen
                try:
                    session.screen = rendered
                    session.frame("install-progress")
                    session.events[-1]["offset"] = end
                finally:
                    session.screen = latest
                return end
        check(session.process.poll() is None or received, "public CLI exited before install-progress")
    raise AssertionError("timeout waiting for terminal screen: install-progress")


def key(session, value):
    session.events.append({"event": "raw-key", "hex": value.hex(), "offset": len(session.raw)})
    session.send(value)


def skill_rows(view):
    return re.findall(r"\[([• ])\]\s+\d+\.\s+([^\n]+)", view)


def target_rows(view):
    return re.findall(r"\[([• ])\][^\n]*?\(([a-z0-9-]+)\)", view)


def choose_skills(session):
    wait_screen(session, "skill-menu", lambda v: "Choose Skills to install" in v and "enter submit" in v)
    previous = ""
    for name in SKILLS:
        query = "skills/" + name
        key(session, b"/" + b"\x7f" * len(previous) + query.encode() + b"\r")
        expected = name + " · " + query
        wait_screen(session, "filter-" + name,
                    lambda v: skill_rows(v) == [(" ", expected)] and "enter submit" in v)
        key(session, b" ")
        wait_screen(session, "selected-" + name, lambda v: skill_rows(v) == [("•", expected)])
        previous = query
    key(session, b"\r")


def choose_targets(session):
    view = wait_screen(session, "target-menu",
                       lambda v: "Choose targets" in v and "enter submit" in v and len(target_rows(v)) == 2)
    rows = target_rows(view)
    check(sorted(rows) == [("•", target) for target in TARGETS], "unexpected native target defaults")
    # Exercise the ordinary target toggles without changing the final selection.
    for index, (_, target) in enumerate(rows):
        key(session, b" ")
        wait_screen(session, target + "-off", lambda v: (" ", target) in target_rows(v))
        key(session, b" ")
        wait_screen(session, target + "-on", lambda v: target_rows(v) == rows)
        if index + 1 < len(rows):
            key(session, b"\x1b[B")
    key(session, b"\r")


def unchanged_before_confirmation(root, project, env, before):
    baseline = json.loads((root / "native-project-baseline.json").read_text())
    profile = json.loads((Path(env["CLAUDE_CONFIG_DIR"]) / ".claude.json").read_text())
    check(profile["projects"].get(str(project), {}).get("hasTrustDialogAccepted") is not True,
          "selected Claude project was trusted before confirmation")
    check(profile["projects"][baseline["other_project"]] == baseline["foreign_trust"],
          "foreign Claude trust changed before confirmation")
    settings = json.loads((project / ".claude/settings.json").read_text())
    check(settings == {"enabledPlugins": {"foreign-project-fixture@skills-dir": False}},
          "Claude project settings changed before confirmation")
    config = (Path(env["CODEX_HOME"]) / "config.toml").read_text()
    check(config == baseline["codex_config"], "Codex config changed before confirmation")
    for directory, expected in before.items():
        check(hashes(Path(directory)) == expected, "native Skill directory changed before confirmation: " + directory)
    absent = [project / "uap.toml", project / "uap.lock", project / ".uap/state-v2.json"]
    absent += [project / ".agents/skills" / name for name in SKILLS]
    for path in absent:
        check(not os.path.lexists(path), "selected installation changed before confirmation: " + str(path))
    return {"absent": [str(path) for path in absent],
            "codex_config_sha256": hashlib.sha256(config.encode()).hexdigest(),
            "selected_claude_trust": False, "foreign_claude_trust_unchanged": True,
            "claude_project_settings_unchanged": True, "native_skill_directories": before}


def confirm_review(session, project):
    view = wait_screen(session, "review-summary", lambda v: "Install these Skills?" in v and "d show paths" in v)
    check("2Skills·2agents·4bindings" in compact(view), "review lost the selected binding count")
    for name in SKILLS:
        check("install" + name in compact(view), "review omitted selected Skill " + name)
    key(session, b"\x1b[F")  # End: read the persistence warning at the end of the summary.
    warning = 'Installation may trust the selected project "' + str(project) + '" in Claude Code.'
    wait_screen(session, "review-consent", lambda v: compact(warning) in compact(v) and
                "Thispreferenceremainsafterremoval." in compact(v))
    key(session, b"d")
    wait_screen(session, "review-paths", lambda v: "d hide paths" in v and compact(str(project)) in compact(v))


def verified_result(raw):
    text = clean(raw)
    marker = "Skills installation succeeded"
    check(marker in text, "terminal installation did not succeed")
    result = text[text.rfind(marker):]
    check("Skills installation verified." in result, "terminal installation did not report native readiness")
    check("Claude Code" in result and "OpenAI Codex" in result, "terminal result omitted a selected agent")
    for name in SKILLS:
        check(len(re.findall(r"(?m)^\s*✓ " + re.escape(name) + r" — installed\s*$", result)) == 2,
              "terminal result did not install both copies of " + name)
    for refused in ("setup required", "not verified", "Next steps:", "sign-in required"):
        check(refused not in result, "terminal result still requires action: " + refused)


def preserve(session, report, root, evidence):
    # The native collector copies top-level JSON/log files only. Keep the
    # actual transcript and rendered frames there, including on failure.
    raw = bytes(session.raw)
    report["transcript"] = {"bytes": len(raw), "sha256": hashlib.sha256(raw).hexdigest(),
                            "truncated": len(raw) > LIMIT}
    (root / "native-terminal-skills.log").write_bytes(raw[-LIMIT:])
    events = json.loads((evidence / "events.json").read_text())
    report["terminal"] = events
    restoration = events.get("restoration") or {}
    report["closure_verified"] = (events["exit"] == 0 and not events["forced_cleanup"] and
                                  all(restoration.get(key) is True for key in
                                      ("termios_equal", "cursor_restored", "next_line_read", "echo")))
    if not report["closure_verified"] or len(raw) > LIMIT:
        report["status"] = "failed"
    report["frames"] = [{"name": path.stem, "text": path.read_text(),
                         "svg": path.with_suffix(".svg").read_text()}
                        for path in sorted(evidence.glob("*.frame.txt"))]
    body = json.dumps(report, indent=2) + "\n"
    check(len(body.encode()) <= LIMIT, "terminal structured evidence exceeded 1 MiB")
    (root / "native-terminal-skills.json").write_text(body)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("installer", "root", "project", "source"):
        parser.add_argument("--" + name, required=True)
    args = parser.parse_args()
    check(sys.platform == "linux", "native Skills keyboard proof is Linux only")
    for name, expected in (("AGENTPLUGINS_NATIVE_DISPOSABLE_HOSTED", "1"),
                           ("GITHUB_ACTIONS", "true"), ("RUNNER_ENVIRONMENT", "github-hosted")):
        check(os.environ.get(name) == expected, "native Skills keyboard proof requires its hosted opt-in")
    root, project, installer = (Path(value).resolve(strict=True) for value in (args.root, args.project, args.installer))
    check(project.parent == root and installer.is_file(), "inexact native Skills fixture")
    evidence = root / "terminal-ui"
    evidence.mkdir(mode=0o700)
    argv = arguments(installer, project, args.source)
    report = {"schema_version": 1, "status": "failed", "argv": argv, "source": args.source,
              "selected_skills": list(SKILLS), "selected_targets": list(TARGETS), "confirmations": 0,
              "unchanged_before_confirmation": False, "progress_observed": False,
              "verified_result": False, "phase": "selection"}
    before = {str(project / directory): hashes(project / directory)
              for directory in (".agents/skills", ".claude/skills")}
    session = Session(argv, SimpleNamespace(project=project, env=dict(os.environ)), evidence,
                      timeout=120, rows=35, cols=120)

    def interrupt(_number, _frame):
        raise KeyboardInterrupt("native Skills keyboard proof interrupted")

    signal.signal(signal.SIGTERM, interrupt)
    signal.signal(signal.SIGINT, interrupt)
    try:
        choose_skills(session)
        choose_targets(session)
        report["phase"] = "review"
        confirm_review(session, project)
        report["preconfirmation"] = unchanged_before_confirmation(root, project, os.environ, before)
        report["unchanged_before_confirmation"] = True
        confirmed_at = len(session.raw)
        key(session, b"\r")
        report["confirmations"], report["phase"] = 1, "install"
        progress_at = wait_install_progress(session, confirmed_at)
        report["progress_observed"] = True
        session.wait(r"Skills installation verified\.", "complete", after=progress_at)
        session.finish(0)
        verified_result(session.raw)
        screen(session)
        session.frame("verified-result")
        report["verified_result"], report["phase"], report["status"] = True, "complete", "passed"
    except BaseException as error:
        report["error"] = str(error)
        raise
    finally:
        try:
            session.close()
        finally:
            preserve(session, report, root, evidence)
    check(report["closure_verified"], "native Skills keyboard terminal was not restored and closed")


if __name__ == "__main__":
    main()
