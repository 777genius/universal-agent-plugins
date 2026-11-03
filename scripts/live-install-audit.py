#!/usr/bin/env python3
"""Live, non-mocked UAP install/remove evidence. Runs only on disposable profiles."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tempfile
from datetime import datetime, timezone

ROOT = Path(__file__).resolve().parents[1]
FIXTURES = ROOT / "tests/live-install/fixtures.json"
NATIVE_ROOTS = {
    "codex": [".codex", ".agents"], "chatgpt": [],
    "cursor": [".cursor"], "copilot": [".copilot", ".config/copilot"],
    "vscode": [".vscode", ".config/Code"], "kiro": [".kiro"],
    "claude": [".claude"], "gemini": [".gemini"],
    "opencode": [".config/opencode", ".opencode"], "cline": [".cline"],
    "windsurf": [".windsurf", ".devin"], "grok": [".grok"],
    "kimi": [".kimi-code", ".kimi"], "qoder": [".qoder"],
    "pi": [".pi"],
}
PROJECT_SKILL_ROOTS = {
    "codex": [".agents/skills"], "cursor": [".cursor/skills"],
    "copilot": [".github/skills"], "vscode": [".github/skills"],
    "kiro": [".kiro/skills"], "claude": [".claude/skills"],
    "gemini": [".gemini/skills"], "opencode": [".opencode/skills"],
    "cline": [".cline/skills"], "windsurf": [".devin/skills"],
    "grok": [".grok/plugins"], "kimi": [".kimi-code/skills"],
    "qoder": [".uap/managed/clients/qoder"], "pi": [".pi/skills"],
}
EXECUTABLE = {
    "codex": "codex", "copilot": "copilot", "claude": "claude",
    "gemini": "gemini", "opencode": "opencode", "pi": "pi",
    "kiro": "kiro", "grok": "grok", "kimi": "kimi",
    "qoder": "qoder", "windsurf": "devin",
}


def validate_fixtures(doc):
    names = doc["plugin_aliases"]
    skills = doc["skills"]
    targets = doc["targets"]
    if len(names) != 30 or len(set(names)) != 30:
        raise ValueError("Must have exactly 30 different registry aliases")
    if len(skills) != 30 or len(set((v["repo"], v["name"]) for v in skills)) != 30:
        raise ValueError("Must have exactly 30 distinct Skills")
    if set(targets) != set(NATIVE_ROOTS):
        raise ValueError("All 15 advertised adapters must be represented")
    for skill in skills:
        if not re.fullmatch(r"[0-9a-f]{40}", skill["ref"]):
            raise ValueError("Skill lacks a complete immutable commit")
        canonical = f'https://github.com/{skill["repo"]}/tree/{skill["ref"]}/skills/{skill["folder"]}'
        if skill["source"] != canonical or not re.fullmatch(r"[a-z0-9]+(?:-[a-z0-9]+)*", skill["name"]):
            raise ValueError("Invalid Skill identity/source")
    if len(set(s["repo"] for s in skills)) < 3:
        raise ValueError("Insufficiently diverse publishers")


def call(argv, env, cwd, timeout=105):
    try:
        p = subprocess.run(argv, cwd=cwd, env=env, capture_output=True,
                           text=True, errors="replace", timeout=timeout, check=False)
        return {"exit_code": p.returncode, "stdout_tail": p.stdout[-1800:],
                "stderr_tail": p.stderr[-1800:], "timeout": False}
    except subprocess.TimeoutExpired as e:
        return {"exit_code": None, "stdout_tail": "", "stderr_tail": str(e)[-1800:], "timeout": True}
    except OSError as e:
        return {"exit_code": None, "stdout_tail": "", "stderr_tail": str(e)[-1800:], "timeout": False}


def snapshot(root):
    """Only regular files under native profile roots; ignore ephemeral UAP cache."""
    found = {}
    for target_root in root:
        if not target_root.is_dir() or target_root.is_symlink():
            continue
        for base, dirs, files in os.walk(target_root, followlinks=False):
            dirs[:] = [v for v in dirs if not (Path(base) / v).is_symlink()]
            for name in files:
                p = Path(base) / name
                if p.is_symlink() or not p.is_file():
                    continue
                relative = str(p.relative_to(target_root))
                try:
                    st = p.stat()
                    if st.st_size > 4 * 1024 * 1024:
                        digest = "too-large"
                    else:
                        digest = hashlib.sha256(p.read_bytes()).hexdigest()
                    found[f"{target_root.name}/{relative}"] = digest
                except OSError:
                    pass
    return found


def skill_file_present(project, target, skill_name):
    for relative_root in PROJECT_SKILL_ROOTS.get(target, []):
        base = project / relative_root
        if not base.exists() or base.is_symlink():
            continue
        for f in base.rglob("SKILL.md"):
            if f.is_symlink() or not f.is_file():
                continue
            try:
                head = f.read_text(encoding="utf-8")[:3000]
            except OSError:
                continue
            if re.search(r"(?m)^name:\s*['\"]?" + re.escape(skill_name) + r"['\"]?\s*$", head):
                return str(f.relative_to(project))
    return None


def do_case(cli, target, kind, item, index, runroot):
    case_root = runroot / f"{kind}-{index:02d}"
    home = case_root / "home"
    project = case_root / "project"
    data = case_root / "uap-data"
    for path in (home, project, data, home / ".config", home / ".cache", home / ".local/share"):
        path.mkdir(parents=True, exist_ok=True)
    sentinel = project / "foreign-untouched.txt"
    sentinel.write_text("foreign content never belongs to UAP\n")
    user_sentinel = home / "foreign-untouched.txt"
    user_sentinel.write_text("foreign profile content never belongs to UAP\n")
    native_roots = [home / v for v in NATIVE_ROOTS[target]]
    # Presence of an actual binary comes from the workflow; a config directory is
    # only an empty isolated profile and is never counted as agent detection.
    if target in ("codex", "claude", "opencode", "gemini", "copilot", "pi"):
        native_roots[0].mkdir(parents=True, exist_ok=True)
    env = dict(os.environ)
    env.update(HOME=str(home), USERPROFILE=str(home),
               XDG_CONFIG_HOME=str(home / ".config"),
               XDG_CACHE_HOME=str(home / ".cache"),
               XDG_DATA_HOME=str(home / ".local/share"),
               AGENTPLUGINS_HOME=str(data),
               GIT_CONFIG_NOSYSTEM="1", GIT_CONFIG_GLOBAL=str(case_root / "empty.gitconfig"),
               GIT_TERMINAL_PROMPT="0", GCM_INTERACTIVE="never", CI="true",
               DO_NOT_TRACK="1")
    before = snapshot(native_roots)
    if kind == "plugin":
        identity = item
        args = ["add", item, "--target", target, "--scope", "user", "--format", "json"]
        scope = ["--scope", "user"]
    else:
        identity = item["name"]
        args = ["skills", "add", item["source"], "--skill", item["name"],
                "--target", target, "--scope", "project", "--project-root",
                str(project), "--no-ui", "--yes", "--format", "json"]
        scope = ["--scope", "project", "--project-root", str(project)]
    output = call([cli, *args], env, project)
    changed = set(snapshot(native_roots).items()) - set(before.items())
    skill_path = skill_file_present(project, target, identity) if kind == "skill" else None
    locked = (project / "uap.toml").is_file() and (project / "uap.lock").is_file()
    lock_contains = locked and identity in (project / "uap.lock").read_text(encoding="utf-8")
    listed = call([cli, "list", "--target", target, *scope, "--format", "json"], env, project, 30) if output["exit_code"] == 0 else None
    listed_identity = bool(listed and listed["exit_code"] == 0 and identity in listed["stdout_tail"])
    # For a project Skill, require source-bound persisted state and native SKILL.md;
    # for a user plugin require a real native profile delta plus managed list.
    installed = (output["exit_code"] == 0 and
                 ((kind == "skill" and bool(skill_path) and lock_contains) or
                  (kind == "plugin" and bool(changed) and listed_identity)))
    cleanup = None
    if installed:
        cleanup = call([cli, "remove", identity, "--target", target, *scope,
                        "--format", "json"], env, project, 60)
    sentinels_preserved = (sentinel.read_text() == "foreign content never belongs to UAP\n" and
                           user_sentinel.read_text() == "foreign profile content never belongs to UAP\n")
    diagnostics = (output["stdout_tail"] + "\n" + output["stderr_tail"]).lower()
    if installed and cleanup and cleanup["exit_code"] == 0 and sentinels_preserved:
        status = "passed"
    elif installed:
        status = "cleanup-failed"
    elif "not detected" in diagnostics or "not available" in diagnostics or "target_unavailable" in diagnostics:
        status = "target-unavailable"
    elif "unsupported" in diagnostics or "not supported" in diagnostics:
        status = "documented-or-source-unsupported"
    elif output["exit_code"] == 0:
        status = "incomplete-native-proof"
    else:
        status = "install-failed"
    return {"kind": kind, "index": index, "identity": identity, "target": target,
            "source": item if kind == "plugin" else item["source"],
            "status": status, "install": output, "list": listed,
            "native_file_delta": len(changed), "skill_path": skill_path,
            "lock_written": bool(locked), "lock_contains_identity": bool(lock_contains),
            "listed_identity": listed_identity, "cleanup": cleanup,
            "sentinels_preserved": sentinels_preserved}


def main():
    p = argparse.ArgumentParser()
    p.add_argument("--cli", required=True)
    p.add_argument("--kind", choices=("plugin", "skill"), required=True)
    p.add_argument("--target", required=True)
    p.add_argument("--limit", type=int, default=30)
    p.add_argument("--report-dir", required=True)
    args = p.parse_args()
    doc = json.loads(FIXTURES.read_text())
    validate_fixtures(doc)
    if args.target not in doc["targets"] or not 1 <= args.limit <= 30:
        p.error("unknown target or invalid limit")
    report_dir = Path(args.report_dir)
    report_dir.mkdir(parents=True, exist_ok=True)
    cases = doc["plugin_aliases"] if args.kind == "plugin" else doc["skills"]
    selected = cases[:args.limit]
    report = {"checked_at": datetime.now(timezone.utc).isoformat(),
              "target": args.target, "kind": args.kind, "cli": args.cli,
              "fixture_count": len(cases), "cases_attempted": 0,
              "full_coverage": args.limit == 30, "results": []}
    report_file = report_dir / f"{args.target}-{args.kind}.json"
    def save():
        report_file.write_text(json.dumps(report, indent=2, ensure_ascii=False) + "\n")
    expected_binary = EXECUTABLE.get(args.target)
    if expected_binary and shutil.which(expected_binary) is None:
        report["blocker"] = f"Genuine {expected_binary} client executable is not installed"
        report["results"] = [{"target": args.target, "kind": args.kind,
                              "status": "client-unavailable", "identity": v if args.kind == "plugin" else v["name"]}
                             for v in selected]
        save()
        print(report["blocker"], file=sys.stderr)
        return 2
    if args.target == "chatgpt" and args.kind == "skill":
        report["blocker"] = "ChatGPT has no native project skill filesystem adapter"
        report["results"] = [{"status": "documented-unsupported", "identity": v["name"]} for v in selected]
        save()
        return 2
    cli = str(Path(args.cli).resolve(strict=True))
    with tempfile.TemporaryDirectory(prefix="uap-live-30x30-") as temp:
        runroot = Path(temp)
        for i, item in enumerate(selected):
            try:
                result = do_case(cli, args.target, args.kind, item, i, runroot)
            except Exception as e:
                result = {"kind": args.kind, "target": args.target, "index": i,
                          "status": "harness-error", "error": repr(e)}
            report["results"].append(result)
            report["cases_attempted"] += 1
            save()
            print(json.dumps({"target": args.target, "kind": args.kind, "index": i,
                              "status": result["status"]}), flush=True)
    passed = sum(r["status"] == "passed" for r in report["results"])
    report["passed"] = passed
    save()
    print(f'{args.target} {args.kind}: {passed}/{args.limit} passed; full={args.limit == 30}')
    return 0 if passed == args.limit else 1


if __name__ == "__main__":
    sys.exit(main())
