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
import tomllib
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
        raise ValueError("Must have exactly 30 distinct plugin sources")
    if len(skills) != 30 or len(set((v["repo"], v["name"]) for v in skills)) != 30:
        raise ValueError("Must have exactly 30 distinct Skills")
    if set(targets) != set(NATIVE_ROOTS):
        raise ValueError("All 15 advertised adapters must be represented")
    for skill in skills:
        if not re.fullmatch(r"[0-9a-f]{40}", skill["ref"]):
            raise ValueError("Skill lacks a complete immutable commit")
        canonical = f'https://github.com/{skill["repo"]}/tree/{skill["ref"]}/skills/{skill["folder"]}'
        if (skill["source"] != canonical or
                not re.fullmatch(r"[a-z0-9]+(?:-[a-z0-9]+)*", skill["name"]) or
                not re.fullmatch(r"[0-9a-f]{40}", skill.get("skill_md_git_blob_sha1", ""))):
            raise ValueError("Invalid Skill identity/source or pinned blob")
    if len(set(s["repo"] for s in skills)) < 3:
        raise ValueError("Insufficiently diverse publishers")


def call(argv, env, cwd, timeout=105):
    try:
        p = subprocess.run(argv, cwd=cwd, env=env, capture_output=True,
                           text=True, errors="replace", timeout=timeout, check=False)
        try:
            parsed_json = json.loads(p.stdout)
        except ValueError:
            parsed_json = None
        return {"exit_code": p.returncode, "stdout_tail": p.stdout[-1800:],
                "stderr_tail": p.stderr[-1800:], "timeout": False,
                "_json": parsed_json}
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


def locked_skill_is_pinned(project, item, target):
    """Require the exact source, commit, selected folder and target in the real lock."""
    try:
        lock = json.loads((project / "uap.lock").read_text(encoding="utf-8"))
        selected = [p for p in lock["packages"] if p.get("alias") == item["name"]]
        if len(selected) != 1:
            return False
        package = selected[0]
        source = package["source"]["git"]
        location = "skills/" + item["folder"]
        return bool(
            source.get("kind") == "skill" and source.get("skill") == item["name"]
            and source.get("repository", "").casefold() == item["repo"].casefold()
            and source.get("commit") == item["ref"]
            and (source.get("resolved_skill_path") == location or source.get("path") == location)
            and target in package["targets"]
            and package.get("tree_digest", "").startswith("sha256:")
            and package.get("manifest_digest", "").startswith("sha256:")
        )
    except (OSError, ValueError, KeyError, TypeError):
        return False


def matches_pinned_skill_bytes(project, skill_path, blob_sha1):
    if skill_path is None:
        return False
    try:
        data = (project / skill_path).read_bytes()
        git_blob = b"blob " + str(len(data)).encode("ascii") + b"\0" + data
        return hashlib.sha1(git_blob).hexdigest() == blob_sha1
    except OSError:
        return False


def matching_installations(response, identity, target, kind, item):
    """No substring matching: inspect the entire parsed list response."""
    if not isinstance(response, dict):
        return []
    try:
        installations = response["data"]["installations"]
        if not isinstance(installations, list):
            return []
    except (KeyError, TypeError):
        return []
    expected_source = (item if kind == "plugin" else
                       f'{item["repo"]}/skills/{item["folder"]}')
    result = []
    for node in installations:
        if not isinstance(node, dict):
            continue
        clients = node.get("clients", [])
        if not isinstance(clients, list) or not any(
            isinstance(client, dict) and client.get("client_id") == target and
            client.get("verification") == "installation_verified"
            for client in clients):
            continue
        source = node.get("source", "")
        if kind == "skill":
            accepted = source == expected_source
        elif item == "777genius/chrome-devtools-bridge":
            accepted = node.get("name") == identity and isinstance(source, str)
        elif isinstance(item, str) and item.startswith("github:sanity-io/"):
            accepted = node.get("name") == "sanity" and "sanity-io/agent-toolkit" in source
        else:
            accepted = node.get("name") == identity
        if accepted and node.get("installation_id"):
            result.append(node)
    return result


def uninstall_native_codex_plugin(identity, home, case_root, env, cwd):
    """Perform the documented *actual* Codex uninstall before attestation.

    This never touches an ambient account: the config and managed marketplace
    must both be contained within this case's disposable profile.
    """
    config = home / ".codex/config.toml"
    if not config.is_file() or config.is_symlink():
        return {"completed": False, "reason": "managed Codex native config absent"}
    try:
        state = tomllib.loads(config.read_text(encoding="utf-8"))
        plugins = state.get("plugins", {})
        candidates = [name for name in plugins
                      if isinstance(name, str) and name.startswith(identity + "@")]
        if len(candidates) != 1:
            return {"completed": False, "reason": "cannot identify one managed plugin"}
        spec = candidates[0]
        marketplace = spec.split("@", 1)[1]
        entry = state.get("marketplaces", {}).get(marketplace)
        if not isinstance(entry, dict) or entry.get("source_type", "local") != "local":
            return {"completed": False, "reason": "managed local marketplace cannot be verified"}
        source = Path(entry.get("source", ""))
        if not source.is_absolute() or not source.resolve().is_relative_to(case_root.resolve()):
            return {"completed": False, "reason": "marketplace source outside disposable case"}
    except (OSError, TypeError, ValueError):
        return {"completed": False, "reason": "cannot parse owned Codex config"}
    isolated = dict(env, CODEX_HOME=str(home / ".codex"))
    remove_plugin = call(["codex", "plugin", "remove", spec, "--json"], isolated, cwd, 40)
    remove_plugin.pop("_json", None)
    remove_market = call(["codex", "plugin", "marketplace", "remove", marketplace, "--json"],
                         isolated, cwd, 40) if remove_plugin["exit_code"] == 0 else None
    if remove_market:
        remove_market.pop("_json", None)
    try:
        after = tomllib.loads(config.read_text(encoding="utf-8")) if config.exists() else {}
        no_plugin = spec not in after.get("plugins", {})
        no_market = marketplace not in after.get("marketplaces", {})
    except (OSError, ValueError, TypeError):
        no_plugin = no_market = False
    return {"completed": bool(remove_plugin["exit_code"] == 0 and no_plugin and no_market),
            "plugin_remove": remove_plugin, "marketplace_remove": remove_market,
            "native_plugin_absent": no_plugin, "native_marketplace_absent": no_market}


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
        identity = ("chrome-devtools" if item == "777genius/chrome-devtools-bridge"
                    else "sanity" if item.startswith("github:sanity-io/agent-toolkit@")
                    else item)
        args = ["add", item, "--target", target, "--scope", "user", "--format", "json"]
        scope = ["--scope", "user"]
    else:
        identity = item["name"]
        args = ["skills", "add", item["source"], "--skill", item["name"],
                "--target", target, "--scope", "project", "--project-root",
                str(project), "--no-ui", "--yes", "--format", "json"]
        scope = ["--scope", "project", "--project-root", str(project)]
    output = call([cli, *args], env, project)
    output.pop("_json", None)
    changed = set(snapshot(native_roots).items()) - set(before.items())
    skill_path = skill_file_present(project, target, identity) if kind == "skill" else None
    locked = (project / "uap.toml").is_file() and (project / "uap.lock").is_file()
    lock_contains = locked and identity in (project / "uap.lock").read_text(encoding="utf-8")
    lock_pinned = bool(kind == "skill" and locked and locked_skill_is_pinned(project, item, target))
    skill_bytes_match = (kind == "skill" and matches_pinned_skill_bytes(
        project, skill_path, item["skill_md_git_blob_sha1"]))
    listed = (call([cli, "list", "--target", target, *scope, "--format", "json"],
                   env, project, 30) if output["exit_code"] == 0 else None)
    listed_json = listed.pop("_json", None) if listed else None
    matching = matching_installations(listed_json, identity, target, kind, item)
    listed_identity = len(matching) == 1
    # Name and file presence alone are insufficient. A true Skill pass also
    # proves the immutable source in the portable lock AND exact upstream bytes.
    installed = bool(
        output["exit_code"] == 0 and listed_identity and
        (kind == "plugin" or
         (bool(skill_path) and lock_pinned and skill_bytes_match))
    )
    cleanup = None
    native_uninstall = None
    if installed:
        # Codex explicitly requires external uninstall before UAP can settle a
        # user-scope plugin removal. We execute and verify the real native
        # plugin + marketplace commands, never attest on a guessed state.
        if kind == "plugin" and target == "codex":
            native_uninstall = uninstall_native_codex_plugin(
                identity, home, case_root, env, project)
        attestation = (["--external-uninstalled"] if
                       native_uninstall and native_uninstall["completed"] else [])
        cleanup = call([cli, "remove", identity, "--target", target, *scope,
                        *attestation, "--format", "json"], env, project, 60)
        cleanup.pop("_json", None)
    sentinels_preserved = (sentinel.read_text() == "foreign content never belongs to UAP\n" and
                           user_sentinel.read_text() == "foreign profile content never belongs to UAP\n")
    post_removal_skill = skill_file_present(project, target, identity) if kind == "skill" and cleanup else None
    post_list = (call([cli, "list", "--target", target, *scope, "--format", "json"],
                      env, project, 30) if cleanup and cleanup["exit_code"] == 0 else None)
    post_list_json = post_list.pop("_json", None) if post_list else None
    remaining_ids = set()
    if isinstance(post_list_json, dict):
        try:
            remaining_ids = {
                entry.get("installation_id")
                for entry in post_list_json["data"]["installations"]
                if isinstance(entry, dict)
            }
        except (KeyError, TypeError):
            pass
    old_ids = {entry["installation_id"] for entry in matching}
    post_lock_pinned = bool(kind == "skill" and (project / "uap.lock").exists()
                            and locked_skill_is_pinned(project, item, target))
    removal_verified = bool(
        cleanup and cleanup["exit_code"] == 0 and post_list and post_list["exit_code"] == 0
        and isinstance(post_list_json, dict) and old_ids.isdisjoint(remaining_ids)
        and (kind != "skill" or (post_removal_skill is None and not post_lock_pinned))
    )
    diagnostics = (output["stdout_tail"] + "\n" + output["stderr_tail"]).lower()
    if target == "chatgpt" and kind == "plugin" and output["exit_code"] == 0:
        status = "manual-setup-prepared"
    elif installed and removal_verified and sentinels_preserved:
        status = "passed" if kind == "skill" or changed else "package-installed-native-unqualified"
    elif installed:
        status = "cleanup-failed"
    elif "not detected" in diagnostics or "not available" in diagnostics or "target_unavailable" in diagnostics:
        status = "target-unavailable"
    elif "unsupported" in diagnostics or "not supported" in diagnostics:
        status = "documented-or-source-unsupported"
    elif "minimum installer version" in diagnostics:
        status = "test-build-version-policy"
    elif "distribution is suspended" in diagnostics:
        status = "expected-suspended-policy"
    elif output["exit_code"] == 0:
        status = "incomplete-native-proof"
    else:
        status = "install-failed"
    return {"kind": kind, "index": index, "identity": identity, "target": target,
            "source": item if kind == "plugin" else item["source"],
            "status": status, "install": output, "list": listed,
            "native_file_delta": len(changed), "skill_path": skill_path,
            "lock_written": bool(locked), "lock_contains_identity": bool(lock_contains),
            "lock_exact_source_pinned": lock_pinned, "skill_bytes_match": bool(skill_bytes_match),
            "listed_identity": listed_identity, "matched_installation_ids": sorted(old_ids),
            "native_uninstall": native_uninstall, "cleanup": cleanup, "post_remove_list": post_list,
            "post_removal_skill": post_removal_skill, "post_lock_pinned": post_lock_pinned,
            "removal_verified": removal_verified, "sentinels_preserved": sentinels_preserved}


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
