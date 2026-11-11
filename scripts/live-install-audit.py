#!/usr/bin/env python3
"""Live, non-mocked UAP install/remove evidence. Runs only on disposable profiles."""
import argparse
import hashlib
import json
import os
import platform
from pathlib import Path
import re
import shutil
import stat
import subprocess
import sys
import tempfile
import tomllib
from datetime import datetime, timezone
from urllib.parse import urlsplit, urlunsplit

ROOT = Path(__file__).resolve().parents[1]
FIXTURES = ROOT / "tests/live-install/fixtures.json"

# One immutable authority for the signed Marketing Kit and its dependencies.
MARKETING_KIT_DISTRIBUTIONS = {
    "marketing-kit": "777genius/marketing-kit",
    "notion": "777genius/notion",
    "hubspot-crm": "777genius/hubspot-crm",
    "playwright": "777genius/playwright-bridge",
}
MARKETING_KIT_SIGNED_CLOSURE = {
    "777genius/marketing-kit": {
        "release_sequence": 1,
        "package_version": "0.1.0",
        "repository": "777genius/universal-agent-plugins-registry",
        "package_path": "plugins/marketing-kit",
        "resolved_revision": "c5bcedd4ecf12585a8a50401852e57a44891ebec",
        "tree_digest": "sha256:8918c0f0222bd735fd0eef3cacf90b5a9351fb213483d1558a65b9911624e68a",
        "manifest_digest": "sha256:40b18a6b33c6392134aff7beafb9a91eaeecf46ce765e0e888cdc743dd284ea2"
    },
    "777genius/notion": {
        "release_sequence": 1,
        "package_version": "0.1.0",
        "repository": "777genius/universal-agent-plugins",
        "package_path": "plugins/notion",
        "resolved_revision": "2ddbb99dd190c1792b79904f9875e6322bccd243",
        "tree_digest": "sha256:860fa6dd1830d0da2b8fed38195fa6ad6307082ac12d708118e5eb1c5d10f9b7",
        "manifest_digest": "sha256:bbab30a4f062218d7a5fce2f6c818680d3c7adfaebdbaa3e523e15942a2f57e5"
    },
    "777genius/hubspot-crm": {
        "release_sequence": 1,
        "package_version": "0.1.0",
        "repository": "777genius/universal-agent-plugins",
        "package_path": "plugins/hubspot-crm",
        "resolved_revision": "2ddbb99dd190c1792b79904f9875e6322bccd243",
        "tree_digest": "sha256:09a0b151c384235c3d7b3929398695d24cd7b07c75993a0a73f1a218198ededd",
        "manifest_digest": "sha256:20334e86a1b599d2759e04bfaef37dc2ddb7f6b0f3994b5d66d4eca12fbec941"
    },
    "777genius/playwright-bridge": {
        "release_sequence": 2,
        "package_version": "0.0.80",
        "repository": "777genius/universal-agent-plugins-registry",
        "package_path": "plugins/playwright",
        "resolved_revision": "d9c8d778200d51ae941d86bb69af23534cd1c8f7",
        "tree_digest": "sha256:ff3c74bb5760fea47003130f3984d48234892330325a7cad35a7bc973acc9cf6",
        "manifest_digest": "sha256:8419b5c9e67894c2312048c4fe30e3ea920d00101d02641230183dd7ab4d3b31"
    }
}

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
    "codex": ("codex",), "copilot": ("copilot",), "cline": ("cline",),
    "claude": ("claude",), "gemini": ("gemini",),
    "opencode": ("opencode",), "pi": ("pi",),
    "cursor": ("cursor-agent", "cursor"), "vscode": ("code",),
    "kiro": ("kiro-cli", "kiro"), "grok": ("grok",),
    "kimi": ("kimi",), "qoder": ("qoder", "qodercli"),
    "windsurf": ("devin", "windsurf"),
}

# Bind native client commands before running any untrusted package lifecycle.
# In particular, Codex removal attestation must not resolve a new PATH entry.
AGENT_BINARIES = {
    name: str(Path(path).resolve())
    for candidates in EXECUTABLE.values() for name in candidates
    if (path := shutil.which(name))
}
# Git is a host test prerequisite, not a client plugin or an acquired package.
# Resolve it before executing any third-party installation source.
GIT_EXECUTABLE = str(Path(path).resolve()) if (path := shutil.which("git")) else None


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
    direct = doc["plugin_declared_names"]
    exact_git = [v for v in names if v.startswith("github:")]
    if set(direct) != set(exact_git + ["777genius/chrome-devtools-bridge", "777genius/github-bridge"]):
        raise ValueError("All external manifest identities must be supplied and no stale alias mappings")
    pins = doc.get("signed_release_pins")
    signed_aliases = set(names) - set(exact_git)
    if not isinstance(pins, dict) or set(pins) != signed_aliases:
        raise ValueError("Missing frozen Registry release pins for signed aliases")
    for alias, pin in pins.items():
        if not isinstance(pin, dict) or set(pin) != {
                "distribution", "repository", "path", "revision",
                "sequence", "tree_digest", "manifest_digest"}:
            raise ValueError(f"Invalid frozen signed release pin: {alias}")
        if (not all(isinstance(pin[key], str) and pin[key]
                    for key in ("distribution", "repository", "path"))
                or type(pin["sequence"]) is not int or pin["sequence"] < 1
                or not isinstance(pin["revision"], str)
                or not re.fullmatch(r"[0-9a-f]{40}", pin["revision"])
                or any(not isinstance(pin[key], str) or
                       re.fullmatch(r"sha256:[0-9a-f]{64}", pin[key]) is None
                       for key in ("tree_digest", "manifest_digest"))):
            raise ValueError(f"Malformed frozen signed release pin: {alias}")
    for plugin in exact_git:
        if not re.fullmatch(r"github:[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+@[a-f0-9]{40}", plugin):
            raise ValueError("Direct plugin source is not a pinned root package")
        if not re.fullmatch(r"[a-z][a-z0-9-]*", direct[plugin]):
            raise ValueError("Invalid direct package declared identity")
    for alias in ("777genius/chrome-devtools-bridge", "777genius/github-bridge"):
        if not re.fullmatch(r"[a-z][a-z0-9-]*", direct[alias]):
            raise ValueError("Invalid namespaced distribution manifest identity")
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


def call(argv, env, cwd, timeout=105, capture_files=False, capture_full=False):
    try:
        if capture_files or capture_full:
            # A Windows native client may spawn a child which inherits stdout
            # or stderr. Pipes stay open after the UAP process exits, causing
            # subprocess.run(capture_output=True) to time out after a real add.
            # Capture in private regular files so the parent exit is observed.
            scratch = Path(env.get("TEMP", ""))
            if not scratch.is_absolute() or not scratch.is_dir() or scratch.is_symlink():
                raise OSError("private output capture root unavailable")
            with (tempfile.TemporaryFile(mode="w+b", dir=scratch) as stdout_file,
                  tempfile.TemporaryFile(mode="w+b", dir=scratch) as stderr_file):
                p = subprocess.run(argv, cwd=cwd, env=env, stdout=stdout_file,
                                   stderr=stderr_file, stdin=subprocess.DEVNULL,
                                   timeout=timeout, check=False)
                stdout_file.seek(0)
                stderr_file.seek(0)
                limit = 1 << 20
                output, errors = stdout_file.read(limit + 1), stderr_file.read(limit + 1)
            if len(output) > limit or len(errors) > limit:
                return {"exit_code": None, "stdout_tail": "",
                        "stderr_tail": "bounded native output capture exceeded",
                        "timeout": False, "_json": None}
            decoding = "strict" if capture_full else "replace"
            stdout, stderr = output.decode("utf-8", errors=decoding), errors.decode("utf-8", errors=decoding)
        else:
            p = subprocess.run(argv, cwd=cwd, env=env, capture_output=True,
                               text=True, errors="replace", stdin=subprocess.DEVNULL,
                               timeout=timeout, check=False)
            stdout, stderr = p.stdout, p.stderr
        try:
            parsed_json = json.loads(stdout)
        except ValueError:
            parsed_json = None
        if (capture_files and not capture_full and p.returncode == 0
                and (not isinstance(parsed_json, dict)
                     or parsed_json.get("result") != "success")):
            # An inherited child may append to the file after UAP exits.
            # Never promote a truncated/corrupt add response to native PASS.
            return {"exit_code": None, "stdout_tail": stdout[-1800:],
                    "stderr_tail": "native add success JSON unavailable",
                    "timeout": False, "_json": None}
        complete = {"stdout_full": stdout, "stderr_full": stderr} if capture_full else {}
        return {"exit_code": p.returncode, "stdout_tail": stdout[-1800:],
                "stderr_tail": stderr[-1800:], "timeout": False,
                "_json": parsed_json, **complete}
    except UnicodeError:
        return {"exit_code": None, "stdout_tail": "", "stderr_tail": "native output capture was not UTF-8",
                "timeout": False, "_json": None}
    except subprocess.TimeoutExpired as e:
        return {"exit_code": None, "stdout_tail": "", "stderr_tail": str(e)[-1800:], "timeout": True}
    except OSError as e:
        return {"exit_code": None, "stdout_tail": "", "stderr_tail": str(e)[-1800:], "timeout": False}


def install_case_timeout(target, kind, item):
    # The frozen Firebase Windows runtime can finish durable registration
    # while a cold native runtime setup still holds the original add process.
    # Stay bounded: a timeout or any surviving registration still FAILS E2E.
    if (sys.platform == "win32" and kind == "plugin"
            and item == "firebase"):
        return 180
    return 105


def version_evidence(argv, env, cwd):
    """Record a short, observable version; a successful path alone is not proof."""
    observed = call(argv, env, cwd, 25)
    value = (observed["stdout_tail"] or observed["stderr_tail"]).strip()
    if observed["timeout"]:
        return {"status": "timeout", "exit_code": observed["exit_code"]}
    if observed["exit_code"] != 0:
        return {"status": "probe-failed", "exit_code": observed["exit_code"]}
    if not value:
        return {"status": "empty-output", "exit_code": 0}
    return {"status": "reported", "value": value[:200], "exit_code": 0}


def snapshot(root):
    """Record native entries without following symlinks or hiding read errors."""
    found = {}
    for target_root in root:
        if target_root.is_symlink():
            found[f"{target_root.name}/.native-root"] = "symlink"
            continue
        if not target_root.is_dir():
            continue

        def walk_error(error):
            try:
                relative = Path(error.filename).relative_to(target_root)
                found[f"{target_root.name}/{relative}/.walk-error"] = "unreadable"
            except (TypeError, ValueError):
                found[f"{target_root.name}/.walk-error"] = "unreadable"

        for base, dirs, files in os.walk(target_root, followlinks=False, onerror=walk_error):
            for name in dirs[:]:
                path = Path(base) / name
                if path.is_symlink():
                    found[f"{target_root.name}/{path.relative_to(target_root)}"] = "symlink"
                    dirs.remove(name)
            for name in files:
                path = Path(base) / name
                relative = str(path.relative_to(target_root))
                try:
                    if path.is_symlink():
                        digest = "symlink"
                    elif not path.is_file():
                        digest = "non-regular"
                    else:
                        # Never reuse a constant digest for a large native artifact:
                        # repeat-add and remove can rewrite files above 4 MiB.
                        sha = hashlib.sha256()
                        with path.open("rb") as stream:
                            for chunk in iter(lambda: stream.read(1024 * 1024), b""):
                                sha.update(chunk)
                        digest = sha.hexdigest()
                except OSError:
                    digest = "unreadable"
                found[f"{target_root.name}/{relative}"] = digest
    return found


def exact_native_identity(text, identity):
    """Recognize an identity token in native data, never a URL substring."""
    expected = identity.casefold()

    def contains(value):
        if isinstance(value, str):
            return value.casefold() == expected
        if isinstance(value, dict):
            return any(str(key).casefold() == expected or contains(child)
                       for key, child in value.items())
        if isinstance(value, list):
            return any(contains(child) for child in value)
        return False

    try:
        return contains(json.loads(text))
    except ValueError:
        escaped = re.escape(identity)
        # TOML/YAML-style native registrations: exact keys or identity fields.
        # An unrelated URL such as https://github.com cannot qualify "github".
        return bool(
            re.search(r"""(?mi)^\s*['"]?""" + escaped + r"""['"]?\s*[:=]""", text)
            or re.search(
                r"""(?mi)^\s*(?:name|id|plugin|plugin_id|identity)\s*[:=]\s*"""
                r"""['"]?""" + escaped + r"""['"]?\s*(?:$|[,#}])""", text)
        )


def codex_owned_native_config(path, identity, case_root):
    """Codex stores exact plugin@marketplace keys, not bare plugin names."""
    try:
        state = tomllib.loads(path.read_text(encoding="utf-8"))
        plugins = state.get("plugins", {})
        markets = state.get("marketplaces", {})
        if not isinstance(plugins, dict) or not isinstance(markets, dict):
            return False
        for spec in plugins:
            if not isinstance(spec, str) or not spec.startswith(identity + "@"):
                continue
            marketplace = spec[len(identity) + 1:]
            if not re.fullmatch(r"agentplugins-[a-f0-9]{12}", marketplace):
                continue
            entry = markets.get(marketplace)
            if not isinstance(entry, dict) or entry.get("source_type", "local") != "local":
                continue
            source = entry.get("source")
            if not isinstance(source, str) or not Path(source).is_absolute():
                continue
            if Path(source).resolve().is_relative_to(case_root.resolve()):
                return True
    except (OSError, TypeError, ValueError):
        return False
    return False


def native_components_from_add(parsed):
    """Read exact natively supported component identities from the released CLI.

    Metadata is a candidate list only; each name still needs independent,
    changed on-disk native evidence inside the disposable client profile.
    """
    if not isinstance(parsed, dict) or not isinstance(parsed.get("data"), dict):
        return ()
    actions = parsed["data"].get("actions")
    if not isinstance(actions, list):
        return ()
    names = set()
    for action in actions:
        if not isinstance(action, dict) or not isinstance(action.get("component_support"), list):
            continue
        for component in action["component_support"]:
            if not isinstance(component, dict) or component.get("support") != "native":
                continue
            name = component.get("name")
            if isinstance(name, str) and re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9_.-]{0,127}", name):
                names.add(name)
    return tuple(sorted(names))


def prepared_only_add(parsed):
    """Never call a prepared-only package a native installation."""
    if not isinstance(parsed, dict) or not isinstance(parsed.get("data"), dict):
        return False
    actions = parsed["data"].get("actions")
    if not isinstance(actions, list) or not actions:
        return False
    found = False
    for action in actions:
        if not isinstance(action, dict) or not isinstance(action.get("component_support"), list):
            return False
        for component in action["component_support"]:
            if not isinstance(component, dict) or component.get("support") != "prepared":
                return False
            found = True
    return found


def attributable_native_changes(native_roots, changes, identity, target, component_names=()):
    """Credit only identity-attributed active native state, not cached downloads."""
    found = []
    identities = tuple(dict.fromkeys((identity, *component_names)))
    normalized = {name.casefold() for name in identities}
    for root in native_roots:
        if not root.is_dir() or root.is_symlink():
            continue
        for relpath, _digest in changes:
            prefix = root.name + "/"
            if not relpath.startswith(prefix):
                continue
            relative = relpath[len(prefix):]
            if passive_native_path(target, relative):
                continue
            leaf = root / relative
            if leaf.is_symlink() or not leaf.is_file():
                continue
            try:
                # A complete name component (or file stem) is an attributable
                # managed native path; arbitrary substring matches are not.
                owned_path = any(
                    component.casefold() in normalized
                    or Path(component).stem.casefold() in normalized
                    for component in Path(relative).parts
                )
                # Larger files without an attributable path stay unqualified.
                owned_content = False
                if not owned_path and leaf.stat().st_size <= 1024 * 1024:
                    if target == "codex" and relative == "config.toml":
                        owned_content = codex_owned_native_config(
                            leaf, identity, root.parent.parent)
                    else:
                        content = leaf.read_text(encoding="utf-8", errors="replace")
                        owned_content = any(exact_native_identity(content, name)
                                            for name in identities)
                if owned_path or owned_content:
                    found.append(relpath)
            except OSError:
                continue
    return sorted(set(found))


def source_target_incompatibility(diagnostic, target=None):
    """Recognize narrowly known client/source-policy refusals, never PASS."""
    diagnostic = diagnostic.casefold()
    return ("release does not support complete target set" in diagnostic
            or (target == "pi" and
                "pi package manifest overrides selected extension discovery" in diagnostic))


def is_cli_refusal(output):
    """Diagnostic words cannot turn a successful add or timeout into a refusal."""
    return type(output.get("exit_code")) is int and output["exit_code"] != 0


def host_containment_refusal(diagnostic):
    """Selected native process custody was explicitly unavailable, not a bad source."""
    diagnostic = diagnostic.casefold()
    return ("manual_activation_required" in diagnostic and
            ("cgroup" in diagnostic or
             "safe duplex process containment unavailable" in diagnostic))


def passive_native_path(target, relative):
    """Downloaded caches and run logs are not active native registrations."""
    normalized = "/".join(Path(relative).parts)
    return ((target == "codex" and normalized.startswith("plugins/cache/"))
            or (target == "qoder" and normalized.startswith(("plugins/cache/", "logs/runs/")))
            # Official Grok user-guide Markdown is passive vendor documentation,
            # not a managed plugin registration. No other paths are excluded.
            or (target == "grok" and normalized in (
                "docs/user-guide/05-configuration.md",
                "docs/user-guide/16-subagents.md")))


def qoder_passive_latest_log_link(root, file):
    """Qualify only Qoder's real, contained run-log pointer, never a plugin link."""
    logs = root / "logs"
    if file != logs / "latest" or root.is_symlink() or logs.is_symlink():
        return False
    try:
        relative = file.readlink()
        if relative.is_absolute() or not path_inside_case(logs, root):
            return False
        destination = Path(os.path.normpath(str(logs / relative)))
        if not any(destination.is_relative_to(logs / leaf)
                   for leaf in ("runs", "sessions")):
            return False
        return (path_inside_case(destination, logs)
                and (destination.is_file() or destination.is_dir()))
    except (OSError, RuntimeError, ValueError):
        return False


def qoder_native_plugin_state(home, env, cwd, identity, expected_version=None):
    """Read real Qoder 1.1 native JSON; never qualify from a package cache."""
    profile = home / ".qoder"
    executable = next((AGENT_BINARIES[name] for name in EXECUTABLE["qoder"]
                       if AGENT_BINARIES.get(name)), None)
    # Both native profile selectors must remain inside the disposable HOME.
    # The executable was pinned before any untrusted source installation.
    if (not executable or not Path(executable).is_absolute()
            or not profile.is_dir() or profile.is_symlink()
            or not path_inside_case(profile, home)
            or env.get("QODER_CONFIG_DIR") != str(profile.resolve(strict=True))):
        return {"status": "unqualified-prerequisites"}
    observed = call([executable, "--config-dir", str(profile),
                     "plugins", "list", "--json"], env, cwd, 30)
    entries = observed.get("_json")
    if observed.get("exit_code") != 0 or not isinstance(entries, list) or len(entries) > 1024:
        return {"status": "unqualified-list"}
    seen = set()
    claimed = []
    for entry in entries:
        if not isinstance(entry, dict) or not isinstance(entry.get("id"), str):
            return {"status": "unqualified-schema"}
        native_id = entry["id"]
        if (native_id.count("@") != 1 or not all(native_id.split("@", 1))
                or native_id in seen
                or ("version" in entry and not isinstance(entry["version"], str))):
            return {"status": "unqualified-schema"}
        seen.add(native_id)
        if native_id.split("@", 1)[0] == identity:
            claimed.append(entry)
    if not claimed:
        return {"status": "absent"}
    if (len(claimed) == 1 and claimed[0]["id"] == identity + "@local"
            and isinstance(expected_version, str) and expected_version
            and claimed[0].get("version") == expected_version):
        return {"status": "registered"}
    return {"status": "unqualified-identity-or-version"}


def copilot_json_document(text):
    def unique(pairs):
        if len({key for key, _ in pairs}) != len(pairs):
            raise ValueError("ambiguous Copilot JSON object")
        return dict(pairs)
    def reject_constant(_):
        raise ValueError("invalid Copilot JSON constant")
    return json.loads(text, object_pairs_hook=unique, parse_constant=reject_constant)

COPILOT_BUILTIN_JSON = [{"name": "browser-use", "marketplace": "", "version": "0.0.4",
                         "enabled": False, "source": "builtin"}]
COPILOT_BUILTIN_TEXT = "Built-in plugins:\n  • browser-use (v0.0.4) [disabled]\n"
COPILOT_EMPTY_TEXT = "No plugins installed.\n\nUse 'copilot plugin install <source>' to install a plugin.\n"


def copilot_native_entries_state(entries, owners):
    """Check the complete observed .105 live JSON identity and source schema."""
    if not isinstance(entries, list) or len(entries) != len(owners) + 1:
        raise AssertionError("Copilot native plugin listing malformed or incomplete")
    found, builtin = set(), []
    for entry in entries:
        if (not isinstance(entry, dict) or not isinstance(entry.get("name"), str)
                or type(entry.get("enabled")) is not bool or not isinstance(entry.get("source"), str)
                or (entry.get("version") is not None and not isinstance(entry["version"], str))):
            raise AssertionError("Copilot native plugin entry malformed")
        name = entry["name"]
        if name == "browser-use":
            builtin.append(entry)
            continue
        if (name not in owners or name in found or entry.get("marketplace") != owners[name]["marketplace"]
                or entry["enabled"] is not True
                or entry.get("version") != owners[name]["version"]
                or entry["source"] != "live" or entry.get("installedFrom") != owners[name]["target_locator"]):
            raise AssertionError("Copilot native managed identity is foreign or duplicated")
        found.add(name)
    if builtin != COPILOT_BUILTIN_JSON or found != set(owners):
        raise AssertionError("Copilot native closure or observed builtin baseline changed")
    return {"status": "installed" if owners else "absent", "managed_count": len(found)}


def copilot_native_text_state(text, owners):
    """Consume complete .105 live listing, including each exact physical path."""
    document = text.replace("\r\n", "\n")
    if not owners:
        if document != COPILOT_EMPTY_TEXT + COPILOT_BUILTIN_TEXT:
            raise AssertionError("Copilot native empty text differs from observed baseline")
        return
    if not document.startswith(COPILOT_BUILTIN_TEXT):
        raise AssertionError("Copilot native builtin prefix changed")
    document = document[len(COPILOT_BUILTIN_TEXT):].removesuffix("\n")
    lines = document.split("\n")
    if (not lines or lines[0] != "Live Plugins (loaded from a local marketplace directory, never copied):"
            or len(lines) != 1 + 2 * len(owners)):
        raise AssertionError("Copilot native full live listing incomplete")
    found = set()
    for index in range(1, len(lines), 2):
        entry = re.fullmatch(r"  • ([A-Za-z0-9][A-Za-z0-9._-]*)@(agentplugins-[0-9a-f]{12}) \(v([^() ]+)\) \(enabled\)", lines[index])
        if (entry is None or entry[1] not in owners or entry[1] in found
                or entry[2] != owners[entry[1]]["marketplace"] or entry[3] != owners[entry[1]]["version"]
                or lines[index + 1] != "      from " + owners[entry[1]]["target_locator"]):
            raise AssertionError("Copilot native physical identity/version/source differs")
        found.add(entry[1])
    if found != set(owners):
        raise AssertionError("Copilot native physical closure incomplete")

def marketing_kit_signed_pins(fixture_pins):
    """Extend the reviewed 30-source catalogue only with its signed kit child."""
    if not isinstance(fixture_pins, dict):
        return None
    pins = {}
    for alias, distribution in MARKETING_KIT_DISTRIBUTIONS.items():
        anchor = MARKETING_KIT_SIGNED_CLOSURE[distribution]
        pin = {"distribution": distribution, "repository": anchor["repository"],
               "path": anchor["package_path"], "revision": anchor["resolved_revision"],
               "sequence": anchor["release_sequence"], "tree_digest": anchor["tree_digest"],
               "manifest_digest": anchor["manifest_digest"]}
        # HubSpot is a kit dependency, not a 31st top-level catalogue case.
        # Never silently replace a changed or missing reviewed catalogue pin.
        if alias != "hubspot-crm" or alias in fixture_pins:
            supplied = fixture_pins.get(alias)
            if (not isinstance(supplied, dict) or type(supplied.get("sequence")) is not int
                    or supplied != pin):
                return None
        pins[alias] = pin
    return pins


def copilot_signed_kit_owners(list_json, pins, data, case_root):
    """Qualify all four signed User bindings before trusting native siblings."""
    aliases = tuple(MARKETING_KIT_DISTRIBUTIONS)
    pins = marketing_kit_signed_pins(pins)
    if pins is None:
        return None
    text, status = copilot_settings_read(data / "state-v2.json", case_root)
    if status is not None:
        return None
    try:
        state = json.loads(text, object_pairs_hook=_unique_json_object)
        records = state["installations"]
        receipts = state["environment_receipts"]
        public_records = list_json["data"]["installations"]
        if (not isinstance(public_records, list) or len(public_records) != 4
                or not isinstance(records, list) or len(records) != 4
                or not isinstance(receipts, list) or len(receipts) != 4):
            return None
        owners, receipt_keys, physical_ids, binding_ids = {}, set(), set(), set()
        for alias in aliases:
            pin = pins[alias]
            matches = matching_installations(list_json, alias, "copilot", "plugin", alias, pins)
            if len(matches) != 1:
                return None
            public = matches[0]
            public_clients = public["clients"]
            if (not isinstance(public_clients, list) or len(public_clients) != 1
                    or public_clients[0].get("client_id") != "copilot"):
                return None
            version = MARKETING_KIT_SIGNED_CLOSURE[pin["distribution"]]["package_version"]
            public_revision = public_clients[0]["package_revision"]
            if (public_revision.get("version") != version
                    or public_revision.get("distribution_id") != pin["distribution"]
                    or type(public_revision.get("release_sequence")) is not int
                    or public_revision["release_sequence"] != pin["sequence"]):
                return None
            selected = [v for v in records if v["directory"].get("distribution_id") == pin["distribution"]]
            if len(selected) != 1:
                return None
            record = selected[0]
            directory, source, package = (record[k] for k in ("directory", "source", "package"))
            if (record["installation_id"] != public["installation_id"]
                    or record.get("origin_mode") != "directory"
                    or type(directory.get("desired_release_sequence")) is not int
                    or directory.get("desired_release_sequence") != pin["sequence"]
                    or package.get("version") != version
                    or package.get("manifest_digest") != pin["manifest_digest"]
                    or any(source.get(key) != pin[field] for key, field in (
                        ("repository", "repository"), ("package_subpath", "path"),
                        ("resolved_revision", "revision"), ("tree_digest", "tree_digest")))):
                return None
            bindings = record["clients"]
            if not isinstance(bindings, dict) or len(bindings) != 1:
                return None
            binding_id, binding = next(iter(bindings.items()))
            physical = binding.get("physical_artifact_id")
            target = Path(binding.get("target_locator", ""))
            revision = binding.get("package_revision", {})
            if (not isinstance(binding_id, str) or not binding_id or binding_id in binding_ids
                    or binding.get("client_binding_id") != binding_id
                    or binding.get("client_id") != "copilot" or binding.get("scope") != "user"
                    or binding.get("materialization") != "materialized"
                    or binding.get("verification") != "installation_verified"
                    or binding.get("affected_surfaces") != ["copilot", "vscode"]
                    or not isinstance(physical, str) or not physical.strip()
                    or physical != physical.strip() or physical in physical_ids
                    or physical in (".", "..") or Path(physical).name != physical
                    or not target.is_absolute() or not target.is_dir()
                    or not path_inside_case(target, case_root)
                    or type(revision.get("release_sequence")) is not int
                    or any(revision.get(key) != expected for key, expected in (
                        ("distribution_id", pin["distribution"]),
                        ("release_sequence", pin["sequence"]),
                        ("version", version), ("resolved_revision", pin["revision"]),
                        ("tree_digest", pin["tree_digest"]),
                        ("manifest_digest", pin["manifest_digest"])))):
                return None
            binding_receipts = binding.get("receipts")
            if (not isinstance(binding_receipts, list) or not binding_receipts
                    or any(not isinstance(v, dict) or v.get("phase") != "committed"
                           or v.get("client_binding_id") != binding_id
                           or v.get("active_path") != str(target)
                           or not isinstance(v.get("operation_id"), str) or not v["operation_id"]
                           for v in binding_receipts)
                    or len({v["operation_id"] for v in binding_receipts}) != len(binding_receipts)):
                return None
            marketplace = "agentplugins-" + hashlib.sha256(physical.encode()).hexdigest()[:12]
            manifest_text, manifest_status = copilot_settings_read(
                target / ".github/plugin/marketplace.json", case_root)
            if manifest_status is not None:
                return None
            manifest = json.loads(manifest_text, object_pairs_hook=_unique_json_object)
            if (not isinstance(manifest, dict) or manifest.get("name") != marketplace
                    or not isinstance(manifest.get("plugins"), list)
                    or len(manifest["plugins"]) != 1
                    or any(manifest["plugins"][0].get(key) != value for key, value in (
                        ("name", alias), ("version", version), ("source", ".")))):
                return None
            owners[alias] = {"marketplace": marketplace, "version": version,
                             "target_locator": str(target)}
            receipt_keys.add((alias, "copilot", public["installation_id"],
                              binding_id, "directory:" + pin["distribution"], pin["tree_digest"]))
            physical_ids.add(physical)
            binding_ids.add(binding_id)
        observed = {tuple(item.get(key) for key in (
            "alias", "client_id", "installation_id", "client_binding_id",
            "source_key", "package_digest")) for item in receipts}
        if (observed != receipt_keys or len(observed) != 4
                or len({v.get("receipt_id") for v in receipts}) != 4
                or any(v.get("phase") != "committed" or v.get("environment_id") != "user-default"
                       or not isinstance(v.get("receipt_id"), str) or not v["receipt_id"]
                       for v in receipts)):
            return None
        return owners
    except (KeyError, IndexError, TypeError, ValueError, AttributeError):
        return None


def copilot_signed_kit_settings(home, case_root, owners, enabled):
    """Require exactly four physical preferences and only owned marketplaces."""
    if type(enabled) is not bool or not isinstance(owners, dict) or set(owners) != {
            "marketing-kit", "notion", "hubspot-crm", "playwright"}:
        return False
    text, status = copilot_settings_read(home / ".copilot/settings.json", case_root)
    if status is not None:
        return False
    try:
        document = copilot_json_document(text)
        expected = {name + "@" + owner["marketplace"] for name, owner in owners.items()}
        preferences = document.get("enabledPlugins") if isinstance(document, dict) else None
        if (not isinstance(document, dict)
                or set(document) not in ({"enabledPlugins"},
                                        {"enabledPlugins", "extraKnownMarketplaces"})
                or not isinstance(preferences, dict) or set(preferences) != expected
                or any(value is not enabled for value in preferences.values())):
            return False
        marketplaces = document.get("extraKnownMarketplaces", {})
        if not isinstance(marketplaces, dict):
            return False
        if not enabled:
            return marketplaces == {}
        # Copilot .105 retains owned directory registrations while installed.
        # Complete native JSON and TEXT independently require all four plugins.
        expected_sources = {
            owner["marketplace"]: {"source": {"source": "directory",
                                               "path": owner["target_locator"]}}
            for owner in owners.values()}
        return all(name in expected_sources and value == expected_sources[name]
                   for name, value in marketplaces.items())
    except (TypeError, ValueError, KeyError, AttributeError):
        return False


def copilot_native_list_read_unchanged(profile, home, before):
    """Reject native read mutations except at most two bounded new process logs."""
    after = snapshot([profile])
    unsafe = ("symlink", "unreadable", "non-regular")
    if any(value in unsafe for value in (*before.values(), *after.values())):
        return False
    added = [name for name in after if name not in before]
    if len(added) > 2:
        return False
    prefix = profile.name + "/" + "logs" + os.sep
    for name in added:
        if not name.startswith(prefix):
            return False
        path = profile / name[len(profile.name) + 1:]
        try:
            metadata = path.lstat()
        except OSError:
            return False
        if (path.parent != profile / "logs"
                or re.fullmatch(r"process-[0-9]+-[0-9]+\.log", path.name) is None
                or not stat.S_ISREG(metadata.st_mode) or metadata.st_nlink != 1
                or metadata.st_size > (1 << 20) or not path_inside_case(path, home)):
            return False
        after.pop(name)
    return after == before


def copilot_native_plugin_state(home, env, cwd, identity, expected_version=None,
                                owners=None, owned_call=None):
    """Read the original profile; signed kits require complete JSON and TEXT."""
    profile = home / ".copilot"
    executable = AGENT_BINARIES.get("copilot")
    if (not executable or not Path(executable).is_absolute()
            or not profile.is_dir() or profile.is_symlink()
            or not path_inside_case(profile, home)):
        return {"status": "unqualified-prerequisites"}
    native_env = dict(env, COPILOT_HOME=str(profile))
    if owners is not None:
        if (not isinstance(owners, dict)
                or (owners and set(owners) != set(MARKETING_KIT_DISTRIBUTIONS))):
            return {"status": "unqualified-signed-closure"}
        before_native_query = snapshot([profile])
        try:
            responses = []
            invoke_owned = call if owned_call is None else owned_call
            for suffix in (["--json"], []):
                if (not profile.is_dir() or profile.is_symlink()
                        or not path_inside_case(profile, home)):
                    return {"status": "unqualified-prerequisites"}
                result = invoke_owned([executable, "plugin", "list", *suffix],
                                      native_env, cwd, 30, capture_full=True)
                if (type(result.get("exit_code")) is not int or result["exit_code"] != 0
                        or result.get("timeout") is not False or result.get("stderr_full") != ""
                        or not isinstance(result.get("stdout_full"), str)
                        or len(result["stdout_full"].encode("utf-8")) > 1 << 20):
                    return {"status": "unqualified-list"}
                responses.append(result["stdout_full"])
            try:
                state = copilot_native_entries_state(copilot_json_document(responses[0]), owners)
                copilot_native_text_state(responses[1], owners)
                return state
            except (AssertionError, ValueError, TypeError, KeyError, AttributeError):
                return {"status": "unqualified-signed-closure"}
        finally:
            if not copilot_native_list_read_unchanged(profile, home, before_native_query):
                raise AssertionError("Copilot native list modified protected profile")
    observed = call([executable, "plugin", "list", "--json"], native_env, cwd, 30)
    entries = observed.get("_json")
    if observed.get("exit_code") != 0 or not isinstance(entries, list) or len(entries) > 1024:
        return {"status": "unqualified-list"}
    found = 0
    for entry in entries:
        if (not isinstance(entry, dict)
                or not isinstance(entry.get("name"), str)
                or not isinstance(entry.get("enabled"), bool)
                or not isinstance(entry.get("source"), str)
                or ("version" in entry and entry["version"] is not None
                    and not isinstance(entry["version"], str))
                or ("marketplace" in entry and entry["marketplace"] is not None
                    and not isinstance(entry["marketplace"], str))):
            return {"status": "unqualified-schema"}
        market = entry.get("marketplace")
        managed = (isinstance(market, str)
                   and re.fullmatch(r"agentplugins-[0-9a-f]{12}", market))
        if entry["name"] != identity:
            # A managed marketplace under a reminted name is not native absence.
            if managed:
                return {"status": "unqualified-reminted-managed"}
            continue
        if not managed:
            return {"status": "unqualified-foreign-name"}
        if (entry["enabled"] is not True
                or (entry.get("version") is not None
                    and entry["version"] != expected_version)):
            return {"status": "unqualified-activation-or-version"}
        found += 1
    if found == 0:
        return {"status": "absent"}
    if found == 1 and isinstance(expected_version, str) and expected_version:
        return {"status": "installed"}
    return {"status": "unqualified-identity"}


def copilot_disabled_preference_only(before, after_add, after_remove,
                                    native_before, native_after, residuals):
    """Accept no active registration, without deleting a vendor's disabled preference."""
    if (not isinstance(before, dict) or before.get("status") != "absent"
            or not isinstance(after_add, dict) or after_add.get("status") != "parsed"
            or not isinstance(after_remove, dict) or after_remove.get("status") != "parsed"
            or not isinstance(native_before, dict) or native_before.get("status") != "installed"
            or not isinstance(native_after, dict) or native_after.get("status") != "absent"
            or residuals != [".copilot/settings.json"]
            or after_remove.get("allowed_sections") != ["enabledPlugins"]):
        return False
    expected_after = {"exact_key": 0, "qualified_key": 1, "reference_key": 0,
                      "exact_value": 0, "qualified_value": 0, "reference_value": 0}
    return (after_add.get("enabled_plugin_value_counts") ==
            {"true": 1, "false": 0, "non_boolean": 0}
            and after_remove.get("enabled_plugin_value_counts") ==
            {"true": 0, "false": 1, "non_boolean": 0}
            and after_remove.get("match_counts") == expected_after)


def native_identity_residuals(native_roots, identity, observed_paths, target,
                              component_names=()):
    """After remove, reject any remaining named native file/config entry.

    Codex may retain its downloaded source cache after its native plugin and
    marketplace are removed; that cache is not active client configuration.
    """
    leftover = []
    identities = tuple(dict.fromkeys((identity, *component_names)))
    normalized = {name.casefold() for name in identities}
    for root in native_roots:
        prefix = root.name + "/"
        if root.is_symlink():
            leftover.append(root.name + ": symlink")
            continue
        for observed in observed_paths:
            if not observed.startswith(prefix):
                continue
            relative = observed[len(prefix):]
            # Synthetic os.walk error markers have no on-disk file to lstat.
            # Retain the error as negative proof instead of treating it as gone.
            if relative == ".walk-error" or relative.endswith("/.walk-error"):
                leftover.append(observed + ": unreadable-walk")
                continue
            file = root / relative
            try:
                mode = file.lstat().st_mode
                if stat.S_ISLNK(mode):
                    if (target != "qoder" or "/".join(Path(relative).parts) != "logs/latest"
                            or not qoder_passive_latest_log_link(root, file)):
                        leftover.append(observed + ": symlink")
                elif not stat.S_ISREG(mode):
                    leftover.append(observed + ": non-regular")
                elif passive_native_path(target, relative):
                    continue
                elif any(
                        part.casefold() in normalized
                        or Path(part).stem.casefold() in normalized
                        or any(re.fullmatch(
                            re.escape(name) + r"@agentplugins-[0-9a-f]{12}",
                            Path(part).stem, re.IGNORECASE) for name in identities)
                        for part in Path(relative).parts):
                    leftover.append(observed)
                elif file.stat().st_size > 1024 * 1024:
                    # Unread content must never count as clean native state.
                    leftover.append(observed + ": unscanned-too-large")
                else:
                    contents = file.read_text(encoding="utf-8", errors="replace")
                    qualified = any(re.search(
                        r"(?<![A-Za-z0-9_./:-])" + re.escape(name)
                        + r"@agentplugins-[0-9a-f]{12}(?![A-Za-z0-9_./-])",
                        contents, re.IGNORECASE) for name in identities)
                    if qualified or any(exact_native_identity(contents, name)
                                        for name in identities):
                        leftover.append(observed)
            except FileNotFoundError:
                continue
            except OSError:
                leftover.append(observed + ": unreadable")
    return sorted(set(leftover))


def path_inside_case(path, case_root):
    """Reject intermediate symlinks as well as escaping or unreadable roots."""
    try:
        if case_root.is_symlink() or not path.is_relative_to(case_root):
            return False
        if not path.resolve().is_relative_to(case_root.resolve(strict=True)):
            return False
        cursor = path
        while cursor != case_root:
            if cursor.is_symlink():
                return False
            cursor = cursor.parent
        return True
    except (OSError, RuntimeError, ValueError):
        return False


def skill_removal_leftovers(project, target, skill_name, installed_path=None):
    """Reject surviving Skills and substituted intermediate project roots."""
    leftovers = []
    case_root = project.parent
    for relative_root in PROJECT_SKILL_ROOTS.get(target, []):
        base = project / relative_root
        named = base / skill_name
        if not path_inside_case(base, case_root) or not path_inside_case(named, case_root):
            leftovers.append(str(base.relative_to(project)) + ": unsafe ancestor")
        elif named.exists() or named.is_symlink():
            leftovers.append(str(named.relative_to(project)))
    if installed_path:
        path = project / installed_path
        if not path_inside_case(path, case_root):
            leftovers.append(str(installed_path) + ": unsafe ancestor")
        elif path.exists() or path.is_symlink():
            leftovers.append(str(path.relative_to(project)))
    return sorted(set(leftovers))


def skill_file_present(project, target, skill_name):
    case_root = project.parent
    for relative_root in PROJECT_SKILL_ROOTS.get(target, []):
        base = project / relative_root
        if not path_inside_case(base, case_root) or not base.is_dir():
            continue
        for f in base.rglob("SKILL.md"):
            if not path_inside_case(f, case_root) or not f.is_file():
                continue
            try:
                head = f.read_text(encoding="utf-8", errors="replace")[:3000]
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
            and re.fullmatch(r"sha256:[0-9a-f]{64}", package.get("tree_digest", "")) is not None
            and re.fullmatch(r"sha256:[0-9a-f]{64}", package.get("manifest_digest", "")) is not None
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


def has_target_registration(response, identity, target):
    """Inspect failed add's list without requiring a verified/active binding."""
    if not isinstance(response, dict):
        return False
    data = response.get("data")
    installations = data.get("installations") if isinstance(data, dict) else None
    if not isinstance(installations, list):
        return False
    return any(
        isinstance(node, dict) and node.get("name") == identity
        and isinstance(node.get("clients"), list)
        and any(isinstance(client, dict) and client.get("client_id") == target
                for client in node["clients"])
        for node in installations
    )


def recorded_signed_release_matches(directory, revision):
    """Bind a verified client's package to the recorded signed release."""
    if not isinstance(directory, dict) or not isinstance(revision, dict):
        return False
    distribution = directory.get("recorded_distribution")
    commit = directory.get("recorded_revision")
    sequence = directory.get("recorded_release_sequence")
    if (not isinstance(distribution, str) or not distribution
            or not isinstance(commit, str) or not re.fullmatch(r"[0-9a-f]{40}", commit)
            or type(sequence) is not int or sequence < 1):
        return False
    return bool(
        revision.get("distribution_id") == distribution
        and revision.get("resolved_revision") == commit
        and revision.get("release_sequence") == sequence
        and all(isinstance(revision.get(key), str)
                and re.fullmatch(r"sha256:[0-9a-f]{64}", revision[key])
                for key in ("tree_digest", "manifest_digest"))
    )


def signed_release_public_observation(response, identity, target):
    """Only immutable public provenance fields, never raw list/account content."""
    if not isinstance(response, dict) or not isinstance(response.get("data"), dict):
        return []
    nodes = response["data"].get("installations", [])
    if not isinstance(nodes, list):
        return []
    def public_string(value, pattern):
        return value if isinstance(value, str) and re.fullmatch(pattern, value) else None

    # Signed source paths include owner/repo/subdirectories, not just distribution IDs.
    selector = r"[A-Za-z0-9_.-]+(?:/[A-Za-z0-9_.-]+){1,8}"
    sha = r"[0-9a-f]{40}"
    digest = r"sha256:[0-9a-f]{64}"
    evidence = []
    for node in nodes:
        if not isinstance(node, dict) or node.get("name") != identity:
            continue
        directory = node.get("directory")
        directory = directory if isinstance(directory, dict) else {}
        clients = node.get("clients")
        if not isinstance(clients, list):
            continue
        for client in clients:
            if not isinstance(client, dict) or client.get("client_id") != target:
                continue
            revision = client.get("package_revision")
            revision = revision if isinstance(revision, dict) else {}
            evidence.append({
                "source": public_string(node.get("source"), selector),
                "recorded_distribution": public_string(
                    directory.get("recorded_distribution"), selector),
                "recorded_revision": public_string(directory.get("recorded_revision"), sha),
                "recorded_release_sequence": directory.get("recorded_release_sequence")
                if type(directory.get("recorded_release_sequence")) is int else None,
                "resolved_revision": public_string(revision.get("resolved_revision"), sha),
                "release_sequence": revision.get("release_sequence")
                if type(revision.get("release_sequence")) is int else None,
                "tree_digest": public_string(revision.get("tree_digest"), digest),
                "manifest_digest": public_string(revision.get("manifest_digest"), digest),
            })
    return evidence[:3]


def matching_installations(response, identity, target, kind, item, signed_release_pins=None):
    """Verify the frozen Registry release, never only live self-consistency."""
    signed_release_pins = signed_release_pins or {}
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
        if not isinstance(clients, list):
            continue
        verified_clients = [
            client for client in clients
            if isinstance(client, dict) and client.get("client_id") == target and
            (client.get("verification") == "installation_verified" or
             (kind == "plugin" and client.get("materialization") == "materialized" and
              client.get("verification") == "package_validated" and
              client.get("activation") == "manual_activation_required"))
        ]
        if not verified_clients:
            continue
        source = node.get("source", "")
        if kind == "skill":
            accepted = source == expected_source
        elif item in ("777genius/chrome-devtools-bridge", "777genius/github-bridge"):
            accepted = node.get("name") == identity and isinstance(source, str)
        elif isinstance(item, str) and item.startswith("github:"):
            expected_repo = item.split("@", 1)[0][len("github:"):].casefold()
            recorded_repo = (re.sub(r"^(?:github:|https://github\.com/|git@github\.com:)",
                                    "", source, flags=re.IGNORECASE)
                             .split("@", 1)[0].removesuffix(".git").rstrip("/")
                             if isinstance(source, str) else "")
            accepted = node.get("name") == identity and recorded_repo.casefold() == expected_repo
        else:
            accepted = node.get("name") == identity
        if accepted and node.get("installation_id"):
            # An exact-SHA Git selector must resolve to that SHA, not merely to
            # a repository with a matching display name. Validate the native
            # binding's recorded immutable commit, not CLI stdout prose.
            if kind == "plugin" and isinstance(item, str) and item.startswith("github:"):
                commit = item.rsplit("@", 1)[-1]
                if not any(
                    isinstance(client.get("package_revision"), dict)
                    and client["package_revision"].get("resolved_revision") == commit
                    for client in verified_clients
                ):
                    continue
            # A catalogue name is not enough to prove the selected signed
            # distribution: tie reviewed aliases to the signed directory
            # product and ensure immutable source revision/digests exist.
            if kind == "plugin" and isinstance(item, str) and not item.startswith("github:"):
                pin = signed_release_pins.get(item)
                directory = node.get("directory", {})
                # The published signed snapshot must provide an immutable SHA.
                # Even a self-consistent native receipt cannot replace that pin.
                if not isinstance(pin, dict) or not isinstance(pin.get("revision"), str):
                    continue
                # CLI `list.source` serializes the canonical repository/subpath;
                # the signed Directory distribution is an independent authority.
                # Never compare those two different source-identifying fields.
                expected_public_source = (
                    pin["repository"].strip("/") + "/" + pin["path"].strip("/"))
                if (source != expected_public_source or
                        not isinstance(directory, dict) or
                        directory.get("recorded_distribution") != pin["distribution"] or
                        directory.get("recorded_revision") != pin["revision"] or
                        directory.get("recorded_release_sequence") != pin["sequence"] or
                        (item != "777genius/chrome-devtools-bridge" and
                         directory.get("product_id") !=
                         ("github" if item == "777genius/github-bridge" else item))):
                    continue
                if not any(
                        recorded_signed_release_matches(
                            directory, client.get("package_revision"))
                        and client["package_revision"].get("tree_digest") == pin["tree_digest"]
                        and client["package_revision"].get("manifest_digest") == pin["manifest_digest"]
                        for client in verified_clients):
                    continue
            result.append(node)
    return result



def _unique_json_object(pairs):
    """Reject ambiguous duplicate keys in local audit receipts."""
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError("duplicate JSON field")
        result[key] = value
    return result


def grok_prepared_skill_installations(list_json, item, skill_path, project, add_result, *, repeated=False):
    """Qualify exact Grok project Skill preparation without claiming native activation."""
    if (not isinstance(list_json, dict) or not isinstance(item, dict)
            or not isinstance(skill_path, str) or not isinstance(add_result, dict)):
        return []
    data = list_json.get("data")
    entries = data.get("installations") if isinstance(data, dict) else None
    if (not isinstance(entries, list) or len(entries) != 1
            or not isinstance(entries[0], dict)):
        return []
    node = entries[0]
    parts = Path(skill_path).parts
    if (len(parts) != 6 or parts[:2] != (".grok", "plugins")
            or parts[3:] != ("skills", item.get("name"), "SKILL.md")
            or re.fullmatch(r"skill-[a-f0-9]{45}-[a-f0-9]{12}", parts[2]) is None):
        return []
    root, plugins = project / ".grok", project / ".grok" / "plugins"
    bundle = plugins / parts[2]
    skill = bundle / "skills" / item["name"] / "SKILL.md"
    if (any(p.is_symlink() for p in (project, root, plugins, bundle,
            bundle / "skills", skill.parent, skill))
            or not plugins.is_dir() or not bundle.is_dir() or not skill.is_file()
            or not bundle.resolve().is_relative_to(project.resolve())):
        return []
    try:
        receipt = json.loads(add_result.get("stdout_tail", ""), object_pairs_hook=_unique_json_object)
        if (not isinstance(receipt, dict) or receipt.get("status") != "succeeded"
                or not isinstance(receipt.get("items"), list) or len(receipt["items"]) != 1):
            return []
        selected = receipt["items"][0]
        if (not isinstance(selected, dict)
                or selected.get("alias") != item["name"]
                or selected.get("targetId") != "grok"
                or selected.get("status") != ("unchanged" if repeated else "succeeded")
                or selected.get("destinationPath") != str(bundle)
                or not isinstance(selected.get("candidateId"), str)
                or re.fullmatch(r"sha256:[0-9a-f]{64}", selected["candidateId"]) is None):
            return []
    except (TypeError, ValueError, KeyError, IndexError):
        return []
    name = node.get("name")
    if (node.get("source") != f'{item["repo"]}/skills/{item["folder"]}'
            or not isinstance(name, str) or re.fullmatch(r"skill-[a-f0-9]{56}", name) is None
            or not parts[2].startswith(name[:51] + "-")):
        return []
    uid = node.get("installation_id")
    if (not isinstance(uid, str)
            or re.fullmatch(r"[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}", uid) is None):
        return []
    clients = node.get("clients")
    if (not isinstance(clients, list) or len(clients) != 1
            or not isinstance(clients[0], dict)):
        return []
    client = clients[0]
    if any(client.get(k) != v for k, v in (
            ("client_id", "grok"), ("scope", "project"),
            ("materialization", "materialized"), ("verification", "package_validated"),
            ("activation", "prepared"), ("policy", "allowed"))):
        return []
    revision = client.get("package_revision")
    if not isinstance(revision, dict) or revision.get("resolved_revision") != item.get("ref"):
        return []
    lock = project / "uap.lock"
    if lock.is_symlink() or not lock.is_file():
        return []
    try:
        packages = json.loads(lock.read_text(encoding="utf-8"),
                              object_pairs_hook=_unique_json_object)["packages"]
        matches = [p for p in packages if isinstance(p, dict) and p.get("alias") == item["name"]]
    except (OSError, TypeError, KeyError, ValueError):
        return []
    if len(matches) != 1:
        return []
    for key in ("tree_digest", "manifest_digest"):
        value = revision.get(key)
        if (not isinstance(value, str) or re.fullmatch(r"sha256:[a-f0-9]{64}", value) is None
                or matches[0].get(key) != value):
            return []
    return [node]


def grok_prepared_source_survives(response, item, prior_name, prior_uid):
    """Fail closed on retained or reminted Grok source after removal."""
    data = response.get("data") if isinstance(response, dict) else None
    nodes = data.get("installations") if isinstance(data, dict) else None
    if not isinstance(nodes, list) or not all(isinstance(n, dict) for n in nodes):
        return True
    source = f'{item["repo"]}/skills/{item["folder"]}'
    return any(n.get("source") == source or n.get("installation_id") == prior_uid
               or n.get("name") == prior_name
               or (isinstance(n.get("name"), str) and n["name"].startswith(prior_name[:51]))
               for n in nodes)

def codex_marketplace_source_in_case(source, case_root):
    """Prove physical containment, including Windows extended-length path aliases.

    Compare real directory identity; never permit a lexical prefix or an
    unresolved source to authorize native plugin removal.
    """
    if not source.is_absolute():
        return False
    try:
        root = case_root.resolve(strict=True)
        selected = source.resolve(strict=True)
        if selected.is_relative_to(root):
            return True
        # Go/Codex may write \\?\-prefixed Windows paths into TOML while
        # Python's temporary root is spelled as a normal drive path.
        return any(os.path.samefile(parent, root)
                   for parent in (selected, *selected.parents))
    except (OSError, ValueError, RuntimeError):
        return False


def uninstall_native_codex_plugin(identity, home, case_root, env, cwd):
    """Perform the documented *actual* Codex uninstall before attestation.

    This never touches an ambient account: the config and managed marketplace
    must both be contained within this case's disposable profile.
    """
    codex_bin = AGENT_BINARIES.get("codex")
    if not codex_bin or not Path(codex_bin).is_absolute() or not Path(codex_bin).is_file():
        return {"completed": False, "reason": "pinned Codex executable unavailable"}
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
        if not codex_marketplace_source_in_case(source, case_root):
            return {"completed": False, "reason": "marketplace source outside disposable case"}
    except (OSError, TypeError, ValueError):
        return {"completed": False, "reason": "cannot parse owned Codex config"}
    isolated = dict(env, CODEX_HOME=str(home / ".codex"))
    remove_plugin = call([codex_bin, "plugin", "remove", spec, "--json"], isolated, cwd, 40)
    remove_plugin.pop("_json", None)
    remove_market = call([codex_bin, "plugin", "marketplace", "remove", marketplace, "--json"],
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


# These native-client overrides supersede HOME/XDG in real UAP client adapters.
# Strip the few additional explicit-config knobs before creating one case's
# exact roots. Never allow a caller's existing AI client profile into this E2E.
PROFILE_ENV_KEYS = {
    "CODEX_HOME", "CLAUDE_CONFIG_DIR", "CLAUDE_CODE_CONFIG_DIR",
    "GEMINI_CLI_HOME", "QODER_CONFIG_DIR", "PI_CODING_AGENT_DIR",
    "KIMI_CODE_HOME", "KIMI_CODE_CACHE_DIR", "GROK_HOME",
    "CLINE_DATA_DIR", "CLINE_MCP_SETTINGS_PATH",
    "OPENCODE_CONFIG", "OPENCODE_CONFIG_DIR", "OPENCODE_DATA_DIR",
    "CURSOR_USER_DATA_DIR",
}



# A third-party package may launch processes during a live installation.
# The audit requires public network access, not the runner's account tokens.
# An explicit allowlist is safer than blacklisting secret-name families.
INHERITED_RUNTIME_ENV_KEYS = frozenset({
    "PATH", "PATHEXT", "SystemRoot", "SYSTEMROOT", "WINDIR", "COMSPEC",
    "OS", "USER", "LOGNAME",
    "LANG", "LANGUAGE", "LC_ALL", "LC_CTYPE", "LC_MESSAGES", "TZ", "TERM",
    "HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY",
    "http_proxy", "https_proxy", "all_proxy", "no_proxy",
    "SSL_CERT_FILE", "SSL_CERT_DIR", "CURL_CA_BUNDLE",
    "REQUESTS_CA_BUNDLE", "NODE_EXTRA_CA_CERTS", "GIT_SSL_CAINFO",
})

def isolated_client_env(home, case_root, data):
    """Every native client authority root must belong to this disposable case."""
    home, case_root, data = (p.resolve(strict=True) for p in (home, case_root, data))
    if not home.is_relative_to(case_root) or not data.is_relative_to(case_root):
        raise ValueError("audit profile paths are not inside the disposable case")
    # Native profile overrides, Git config injection and host SSH helpers must
    # not escape the case even if a maintainer runs this script locally.
    env = {key: value for key, value in os.environ.items()
           if key in INHERITED_RUNTIME_ENV_KEYS}
    # Public proxy routing is useful, but embedded proxy credentials must not
    # reach untrusted package install subprocesses.
    for key in ("HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY",
                "http_proxy", "https_proxy", "all_proxy"):
        if key not in env or "@" not in env[key]:
            continue
        try:
            parsed = urlsplit(env[key])
            if "@" in parsed.netloc:
                env[key] = urlunsplit(parsed._replace(netloc=parsed.netloc.rsplit("@", 1)[1]))
            else:
                env.pop(key)
        except ValueError:
            env.pop(key)
    temp = case_root / "tmp"
    temp.mkdir(exist_ok=True)
    if not temp.resolve().is_relative_to(case_root):
        raise ValueError("child temp root would escape disposable case")
    roots = {
        "CODEX_HOME": home / ".codex",
        "CLAUDE_CONFIG_DIR": home / ".claude",
        "GEMINI_CLI_HOME": home,  # Gemini appends /.gemini itself
        "QODER_CONFIG_DIR": home / ".qoder",
        "PI_CODING_AGENT_DIR": home / ".pi/agent",
        "KIMI_CODE_HOME": home / ".kimi-code",
        "KIMI_CODE_CACHE_DIR": home / ".cache/kimi-code",
        # Grok uses disposable HOME/.grok by default. Setting GROK_HOME
        # makes a normal profile look custom and disqualifies project Skills.
        "CLINE_DATA_DIR": home / ".cline/data",
        "CLINE_MCP_SETTINGS_PATH": home / ".cline/data/settings/cline_mcp_settings.json",
    }
    if any(not path.resolve().is_relative_to(home) for path in roots.values()):
        raise ValueError("native profile would escape disposable home")
    env.update({
        "HOME": str(home), "USERPROFILE": str(home),
        "XDG_CONFIG_HOME": str(home / ".config"),
        "XDG_CACHE_HOME": str(home / ".cache"),
        "XDG_DATA_HOME": str(home / ".local/share"),
        "XDG_STATE_HOME": str(home / ".local/state"),
        "APPDATA": str(home / "AppData/Roaming"),
        "LOCALAPPDATA": str(home / "AppData/Local"),
        "AGENTPLUGINS_HOME": str(data),
        "TMPDIR": str(temp), "TEMP": str(temp), "TMP": str(temp),
        "GIT_CONFIG_NOSYSTEM": "1",
        "GIT_CONFIG_GLOBAL": str(case_root / "empty.gitconfig"),
        "GIT_TERMINAL_PROMPT": "0", "GCM_INTERACTIVE": "never",
        "KIMI_CODE_NO_AUTO_UPDATE": "1",
        "CI": "true", "DO_NOT_TRACK": "1",
        **{key: str(path) for key, path in roots.items()},
    })
    return env


def preserved_regular_text(path, expected, case_root):
    """Require a real in-case foreign sentinel, not a symlinked impersonation."""
    if path is None:
        return True
    try:
        return (path_inside_case(path, case_root) and path.is_file()
                and path.read_text(encoding="utf-8") == expected)
    except (OSError, UnicodeError):
        return False



def copilot_settings_windows_name(value):
    """Normalize only Win32 drive/UNC prefixes, never resolve a candidate path."""
    import ntpath
    value = str(value).replace("/", "\\")
    if value.startswith("\\\\?\\UNC\\"):
        value = "\\\\" + value[8:]
    elif value.startswith("\\\\?\\"):
        value = value[4:]
    value = ntpath.normpath(value)
    # Preserve component case, Unicode and short-name aliases exactly. Only a
    # drive letter is normalized; do not invent filesystem equivalence keys.
    return value[0].upper() + value[1:] if len(value) >= 2 and value[1] == ":" else value


def copilot_settings_windows_descriptor(path, initial, expected):
    """Reuse the owned Win32 handle seam and verify its final name before bytes."""
    import ctypes
    from ctypes import wintypes
    import msvcrt
    from runpy import run_path
    helpers = run_path(str(ROOT / "scripts/native_client_project_artifacts.py"))
    descriptor = helpers["open_member"](path)
    try:
        if not helpers["same_opened"](initial, os.fstat(descriptor)):
            raise ValueError("Windows settings identity changed")
        kernel = ctypes.WinDLL("kernel32", use_last_error=True)
        query = kernel.GetFinalPathNameByHandleW
        query.argtypes = [wintypes.HANDLE, wintypes.LPWSTR, wintypes.DWORD, wintypes.DWORD]
        query.restype = wintypes.DWORD
        buffer = ctypes.create_unicode_buffer(4101)
        size = query(msvcrt.get_osfhandle(descriptor), buffer, len(buffer), 0)
        if (size == 0 or size >= len(buffer)
                or copilot_settings_windows_name(buffer.value) != copilot_settings_windows_name(expected)):
            raise ValueError("Windows settings handle left canonical case path")
        return descriptor
    except BaseException:
        os.close(descriptor)
        raise


def copilot_settings_read(path, case_root):
    """Read bounded owned bytes through descriptors, never a replaced symlink."""
    descriptors = []
    try:
        # Resolve only the trusted root, before inspecting candidate metadata.
        # Windows compares the actual held-handle name with this lexical path.
        relative = path.relative_to(case_root)
        canonical_root = case_root.resolve(strict=True)
        root_stat = canonical_root.lstat()
        if not stat.S_ISDIR(root_stat.st_mode):
            return None, "unsafe-path"
        expected = canonical_root / relative
        initial = path.lstat()
        if not stat.S_ISREG(initial.st_mode) or initial.st_size > 1024 * 1024:
            return None, "not-regular-or-too-large"
        if initial.st_nlink != 1 or not path_inside_case(path, case_root):
            return None, "unsafe-path"
        flags = (os.O_RDONLY | getattr(os, "O_NOFOLLOW", 0)
                 | getattr(os, "O_NONBLOCK", 0) | getattr(os, "O_CLOEXEC", 0))
        # POSIX walks verified directory descriptors. A renamed ancestor cannot
        # redirect the final open, and no candidate component is resolved.
        if (os.open in os.supports_dir_fd and hasattr(os, "O_DIRECTORY")
                and hasattr(os, "O_NOFOLLOW")):
            parent = os.open(canonical_root, flags | os.O_DIRECTORY)
            descriptors.append(parent)
            opened_root = os.fstat(parent)
            if (not stat.S_ISDIR(root_stat.st_mode) or not stat.S_ISDIR(opened_root.st_mode)
                    or (root_stat.st_dev, root_stat.st_ino) != (opened_root.st_dev, opened_root.st_ino)):
                return None, "unsafe-path"
            for name in relative.parts[:-1]:
                parent = os.open(name, flags | os.O_DIRECTORY, dir_fd=parent)
                descriptors.append(parent)
            descriptor = os.open(path.name, flags, dir_fd=parent)
        elif os.name == "nt":
            descriptor = copilot_settings_windows_descriptor(path, initial, expected)
        else:
            return None, "unsafe-path"  # No pathname-only read fallback.
        descriptors.append(descriptor)
        opened = os.fstat(descriptor)
        current = path.lstat()
        current_root = canonical_root.lstat()
        identity = (initial.st_dev, initial.st_ino)
        if (not stat.S_ISREG(opened.st_mode) or not stat.S_ISREG(current.st_mode)
                or not stat.S_ISDIR(current_root.st_mode)
                or (root_stat.st_dev, root_stat.st_ino) != (current_root.st_dev, current_root.st_ino)
                or opened.st_nlink != 1 or current.st_nlink != 1
                or identity != (opened.st_dev, opened.st_ino)
                or identity != (current.st_dev, current.st_ino)
                or not path_inside_case(path, case_root)):
            return None, "unsafe-path"
        if opened.st_size > 1024 * 1024:
            return None, "not-regular-or-too-large"
        with os.fdopen(os.dup(descriptor), "rb") as stream:
            body = stream.read(1024 * 1024 + 1)
        if len(body) > 1024 * 1024:
            return None, "not-regular-or-too-large"
        return body.decode("utf-8"), None
    except FileNotFoundError:
        return None, "absent"
    except (OSError, RuntimeError, UnicodeError, ValueError):
        return None, "unsafe-path"
    finally:
        for descriptor in reversed(descriptors):
            os.close(descriptor)


def copilot_settings_sanitized_shape(home, case_root, identity):
    """Observe *only* token shape, never export raw client settings or secrets."""
    path = home / ".copilot/settings.json"
    if not path_inside_case(path, case_root) or path.is_symlink():
        return {"status": "unsafe-path"}
    if not path.exists():
        return {"status": "absent"}
    try:
        text, status = copilot_settings_read(path, case_root)
        if status is not None:
            return {"status": status}
        doc = json.loads(text)
    except (OSError, UnicodeError, ValueError):
        return {"status": "unreadable-or-invalid-json"}
    if not isinstance(doc, (dict, list)):
        return {"status": "unexpected-json-root"}
    names = ("exact_key", "qualified_key", "reference_key",
             "exact_value", "qualified_value", "reference_value")
    counts = dict.fromkeys(names, 0)
    qualified = re.compile(re.escape(identity) + r"@agentplugins-[0-9a-f]{12}", re.I)
    allowed_sections = {"plugins", "installedPlugins", "enabledPlugins",
                        "marketplaces", "pluginMarketplaces"}
    sections = sorted(set(doc) & allowed_sections) if isinstance(doc, dict) else []
    # Native enablement evidence is separate from a mere matching key. Export
    # aggregate booleans only, never a key, provider value or raw settings JSON.
    enabled_values = {"true": 0, "false": 0, "non_boolean": 0}
    enabled = doc.get("enabledPlugins") if isinstance(doc, dict) else None
    if isinstance(enabled, dict):
        for key, value in enabled.items():
            if isinstance(key, str) and qualified.fullmatch(key):
                bucket = ("true" if value is True else
                          "false" if value is False else "non_boolean")
                enabled_values[bucket] += 1
    def match(value, role):
        if value.casefold() == identity.casefold():
            counts["exact_" + role] += 1
        elif qualified.fullmatch(value):
            counts["qualified_" + role] += 1
        elif identity.casefold() in value.casefold():
            counts["reference_" + role] += 1
    stack = [(doc, 0)]
    nodes = 0
    depth_truncated = False
    while stack and nodes < 10000:
        value, depth = stack.pop()
        nodes += 1
        if depth >= 32:
            depth_truncated = True
            continue
        if isinstance(value, dict):
            for key, child in value.items():
                match(key, "key")
                stack.append((child, depth + 1))
        elif isinstance(value, list):
            stack.extend((child, depth + 1) for child in value)
        elif isinstance(value, str):
            match(value, "value")
    return {"status": "truncated" if stack or depth_truncated else "parsed",
            "allowed_sections": sections, "match_counts": counts,
            "enabled_plugin_value_counts": enabled_values}


def initialize_project_skill_git_root(project, env, case_root):
    """Create an isolated real Git worktree for native project Skill discovery."""
    if (not GIT_EXECUTABLE or not Path(GIT_EXECUTABLE).is_file()
            or not project.is_dir() or project.is_symlink()
            or not project.resolve().is_relative_to(case_root.resolve())):
        raise RuntimeError("genuine isolated project Git prerequisite unavailable")
    # Git init follows pre-existing .git symlinks and writes to their targets.
    # Refuse an existing marker before invoking any external Git process.
    marker = project / ".git"
    if marker.exists() or marker.is_symlink():
        raise RuntimeError("disposable project Git metadata must be initially absent")
    # Git for Windows has no POSIX /dev/null. Disable hooks with an empty,
    # case-owned directory on every OS, never through host Git config.
    disabled_hooks = case_root / "empty-git-hooks"
    if disabled_hooks.is_symlink():
        raise RuntimeError("disposable Git hooks directory is redirected")
    disabled_hooks.mkdir(exist_ok=True)
    if (not disabled_hooks.is_dir() or disabled_hooks.is_symlink()
            or not disabled_hooks.resolve().is_relative_to(case_root.resolve())):
        raise RuntimeError("disposable Git hooks directory escaped its case")
    result = call([GIT_EXECUTABLE, "-c", f"core.hooksPath={disabled_hooks.resolve()}",
                   "init", "--quiet"], env, project, 30)
    if (result["exit_code"] != 0 or not marker.is_dir() or marker.is_symlink()
            or not marker.resolve().is_relative_to(project.resolve())):
        raise RuntimeError("genuine isolated project Git init failed")


def do_case(cli, target, kind, item, index, runroot, plugin_declared_names,
            signed_release_pins=None):
    case_root = runroot / f"{kind}-{index:02d}"
    home = case_root / "home"
    project = case_root / "project"
    data = case_root / "uap-data"
    # A previous case can plant a symlink at the *case root* itself. Resolving
    # that link and using its destination as the trust anchor is not containment.
    if runroot.is_symlink() or runroot.parent.is_symlink() or not runroot.parent.is_dir():
        raise ValueError("audit run root is redirected or unavailable")
    runroot.mkdir(exist_ok=True)
    if runroot.is_symlink() or not runroot.is_dir():
        raise ValueError("audit run root is redirected")
    canonical_runroot = runroot.resolve(strict=True)
    if case_root.is_symlink():
        raise ValueError("audit case root is redirected")
    case_root.mkdir(exist_ok=True)
    if (runroot.is_symlink() or case_root.is_symlink() or not case_root.is_dir()
            or case_root.resolve(strict=True).parent != canonical_runroot):
        raise ValueError("audit case root escaped disposable run root")
    canonical_case = case_root.resolve(strict=True)
    for path in (home, project, data, home / ".config", home / ".cache", home / ".local/share"):
        # Never create a descendant through an already redirected parent.
        if (path.is_symlink() or
                not path.parent.resolve().is_relative_to(canonical_case)):
            raise ValueError("nested audit directory escapes disposable case")
        path.mkdir(parents=True, exist_ok=True)
        if path.is_symlink() or not path.resolve().is_relative_to(canonical_case):
            raise ValueError("nested audit directory escapes disposable case")
    native_roots = [home / v for v in NATIVE_ROOTS[target]]
    if any(not root.resolve().is_relative_to(case_root.resolve()) for root in native_roots):
        raise ValueError("native root escapes disposable case")
    sentinel = project / "foreign-untouched.txt"
    sentinel.write_text("foreign content never belongs to UAP\n")
    user_sentinel = home / "foreign-untouched.txt"
    user_sentinel.write_text("foreign profile content never belongs to UAP\n")
    # A foreign file lives adjacent to installed native user artifacts.
    # Its content and position are independent from UAP-managed receipts.
    native_foreign = (native_roots[0] / "foreign-neighbor.txt") if native_roots else None
    if native_foreign:
        native_foreign.parent.mkdir(parents=True, exist_ok=True)
        native_foreign.write_text("unmanaged native neighbor must survive\n")
    # A pre-existing, unrelated project Skill is an independently owned neighbor.
    project_skill_roots = PROJECT_SKILL_ROOTS.get(target, [])
    project_foreign = None
    if kind == "skill" and project_skill_roots:
        # Qoder's UAP-managed bundle is not an unmanaged native Skill location.
        # Grok's plugins/ children require complete plugin manifests. A
        # foreign standalone Skill belongs to its separate native skills/ root.
        foreign_root = (".qoder/skills" if target == "qoder" else
                        ".grok/skills" if target == "grok" else
                        project_skill_roots[0])
        project_foreign = project / foreign_root / "foreign-neighbor" / "SKILL.md"
    if project_foreign:
        project_foreign.parent.mkdir(parents=True, exist_ok=True)
        project_foreign.write_text("---\nname: foreign-neighbor\ndescription: unrelated\n---\n")
    # An installed Skill must leave the complete project Skill tree unchanged
    # after remove, not just its old SKILL.md pathname.
    before_skill_tree = (snapshot([project / name for name in project_skill_roots])
                         if kind == "skill" else {})
    # Presence of an actual binary comes from the workflow; a config directory is
    # only an empty isolated profile and is never counted as agent detection.
    if target in ("codex", "claude", "opencode", "gemini", "copilot", "pi"):
        native_roots[0].mkdir(parents=True, exist_ok=True)
    env = isolated_client_env(home, case_root, data)
    def assert_case_unchanged():
        # Native clients and acquired packages execute as subprocesses.
        # Re-anchor before and after each call; otherwise an ancestor symlink
        # can make os.walk/read_text silently inspect a foreign profile.
        try:
            if (runroot.is_symlink() or case_root.is_symlink()
                    or runroot.resolve(strict=True) != canonical_runroot
                    or case_root.resolve(strict=True) != canonical_case):
                raise ValueError("audit case root changed after external execution")
            for path in (home, project, data):
                if not path.is_dir() or not path_inside_case(path, case_root):
                    raise ValueError("audit case profile/project root was redirected")
            for path in (*native_roots,
                         *(project / rel for rel in project_skill_roots)):
                if not path_inside_case(path, case_root):
                    raise ValueError("audit case native/project ancestor was redirected")
        except (OSError, RuntimeError) as error:
            raise ValueError("audit case root became unavailable") from error

    def audited_call(*args, **kwargs):
        assert_case_unchanged()
        outcome = call(*args, **kwargs)
        assert_case_unchanged()
        return outcome

    if kind == "skill" and target in ("kimi", "grok"):
        # Grok inspect reports projectRoot from Git discovery. Never weaken its
        # strict native project identity contract to accept a missing root.
        initialize_project_skill_git_root(project, env, case_root)
    before = snapshot(native_roots)
    if kind == "plugin":
        identity = plugin_declared_names.get(item, item)
        args = ["add", item, "--target", target, "--scope", "user", "--format", "json"]
        # VS Code's unchanged user adapter only accepts an imperative add.
        if target == "vscode":
            args.append("--no-save")
        scope = ["--scope", "user"]
    else:
        identity = item["name"]
        args = ["skills", "add", item["source"], "--skill", item["name"],
                "--target", target, "--scope", "project", "--project-root",
                str(project), "--no-ui", "--yes", "--format", "json"]
        scope = ["--scope", "project", "--project-root", str(project)]
    copilot_before = (copilot_settings_sanitized_shape(home, case_root, identity)
                      if kind == "plugin" and target == "copilot" else None)
    add_timeout = install_case_timeout(target, kind, item)
    # Keep the original strict add/list/repeat/remove evidence; only avoid
    # inherited Windows Firebase child pipe handles masking parent exit.
    file_capture = (sys.platform == "win32" and kind == "plugin" and item == "firebase")
    capture_option = {"capture_files": True} if file_capture else {}
    output = audited_call([cli, *args], env, project, add_timeout, **capture_option)
    added_json = output.pop("_json", None)
    native_components = (native_components_from_add(added_json)
                         if kind == "plugin" else ())
    prepared_components_only = (prepared_only_add(added_json)
                                if kind == "plugin" else False)
    copilot_after_add = (copilot_settings_sanitized_shape(home, case_root, identity)
                         if copilot_before is not None else None)
    changed = set(snapshot(native_roots).items()) - set(before.items())
    native_attribution = attributable_native_changes(
        native_roots, changed, identity, target, native_components)
    skill_path = skill_file_present(project, target, identity) if kind == "skill" else None
    locked = (project / "uap.toml").is_file() and (project / "uap.lock").is_file()
    lock_contains = locked and identity in (project / "uap.lock").read_text(encoding="utf-8")
    lock_pinned = bool(kind == "skill" and locked and locked_skill_is_pinned(project, item, target))
    skill_bytes_match = (kind == "skill" and matches_pinned_skill_bytes(
        project, skill_path, item["skill_md_git_blob_sha1"]))
    listed = (audited_call([cli, "list", "--target", target, *scope, "--format", "json"],
                   env, project, 30))
    listed_json = listed.pop("_json", None) if listed else None
    matching = matching_installations(listed_json, identity, target, kind, item,
                                     signed_release_pins)
    grok_prepared = (grok_prepared_skill_installations(
        listed_json, item, skill_path, project, output)
        if target == "grok" and kind == "skill" and not matching
        and listed["exit_code"] == 0 else [])
    if grok_prepared:
        matching = grok_prepared
    signed_observation = (
        signed_release_public_observation(listed_json, identity, target)
        if kind == "plugin" and isinstance(item, str) and not item.startswith("github:")
        else [])
    listed_identity = len(matching) == 1
    failed_install_registered = has_target_registration(listed_json, identity, target) if (
        output["exit_code"] != 0) else False
    failed_install_residue = bool(output["exit_code"] != 0 and (
        failed_install_registered or native_attribution or skill_path))
    prepared_binding = bool(listed_identity and (
        grok_prepared or (kind == "plugin" and any(
            client.get("client_id") == target and client.get("verification") == "package_validated"
            and client.get("activation") == "manual_activation_required"
            for client in matching[0].get("clients", []) if isinstance(client, dict)))))
    # Name and file presence alone are insufficient. A true Skill pass also
    # proves the immutable source in the portable lock AND exact upstream bytes.
    installed = bool(
        output["exit_code"] == 0 and listed["exit_code"] == 0 and listed_identity and
        (kind == "plugin" or
         (bool(skill_path) and lock_pinned and skill_bytes_match))
    )
    # BUG-016: a single successful add/remove cannot attest idempotence.
    # Repeat the same source and target only after the first registered add;
    # keep native, managed and foreign state inside the same disposable case.
    repeat_add = None
    repeat_list = None
    repeat_identity_ids = []
    repeat_native_unchanged = None
    repeat_lock_unchanged = None
    repeat_project_unchanged = None
    repeat_verified = False
    if installed:
        before_repeat_native = snapshot(native_roots)
        before_repeat_project = (snapshot([project / root for root in project_skill_roots])
                                 if grok_prepared else None)
        lock_root = project if kind == "skill" else data
        lock_paths = (lock_root / "uap.toml", lock_root / "uap.lock")
        def lock_state():
            return tuple(
                (path.is_file() and not path.is_symlink(),
                 path.read_bytes() if path.is_file() and not path.is_symlink() else None)
                for path in lock_paths
            )
        before_repeat_lock = lock_state()
        repeat_add = audited_call([cli, *args], env, project, add_timeout,
                                  **capture_option)
        repeat_add.pop("_json", None)
        repeat_list = audited_call([cli, "list", "--target", target, *scope, "--format", "json"],
                           env, project, 30)
        repeat_list_json = repeat_list.pop("_json", None)
        repeat_matches = (grok_prepared_skill_installations(
            repeat_list_json, item, skill_path, project, repeat_add, repeated=True)
            if grok_prepared else matching_installations(
                repeat_list_json, identity, target, kind, item, signed_release_pins))
        repeat_identity_ids = sorted(
            entry["installation_id"] for entry in repeat_matches)
        repeat_native_unchanged = snapshot(native_roots) == before_repeat_native
        repeat_project_unchanged = (snapshot([project / root for root in project_skill_roots])
                                    == before_repeat_project if grok_prepared else None)
        repeat_lock_unchanged = lock_state() == before_repeat_lock
        repeat_verified = bool(
            repeat_add["exit_code"] == 0 and repeat_list["exit_code"] == 0
            and len(repeat_matches) == 1
            and repeat_identity_ids == sorted(entry["installation_id"] for entry in matching)
            and repeat_native_unchanged and repeat_lock_unchanged
            and (not grok_prepared or repeat_project_unchanged)
        )
    # BUG-039: independent native ID+version proof before UAP removes it.
    qoder_native_before_remove = None
    if target == "qoder" and kind == "plugin" and installed and repeat_verified:
        qoder_native_before_remove = qoder_native_plugin_state(
            home, env, project, identity, matching[0].get("version"))
        assert_case_unchanged()
    copilot_native_before_remove = None
    kit_owners = None
    if target == "copilot" and kind == "plugin" and installed and repeat_verified:
        if item == "marketing-kit":
            kit_owners = copilot_signed_kit_owners(listed_json, signed_release_pins or {},
                                                   data, case_root)
        if item == "marketing-kit" and (kit_owners is None
                or not copilot_signed_kit_settings(home, case_root, kit_owners, True)):
            copilot_native_before_remove = {"status": "unqualified-signed-closure"}
        else:
            copilot_native_before_remove = copilot_native_plugin_state(
                home, env, project, identity, matching[0].get("version"),
                owners=kit_owners, owned_call=audited_call)
        assert_case_unchanged()
    cleanup = None
    native_uninstall = None
    if installed:
        # Codex explicitly requires external uninstall before UAP can settle a
        # user-scope plugin removal. We execute and verify the real native
        # plugin + marketplace commands, never attest on a guessed state.
        if kind == "plugin" and target == "codex":
            native_uninstall = uninstall_native_codex_plugin(
                identity, home, case_root, env, project)
            assert_case_unchanged()
        attestation = (["--external-uninstalled"] if
                       native_uninstall and native_uninstall["completed"] else [])
        # Grok project Skills have generated bundle names; never select by raw alias.
        selector = matching[0]["installation_id"] if grok_prepared else identity
        cleanup = audited_call([cli, "remove", selector, "--target", target, *scope,
                        *attestation, "--format", "json"], env, project, 60)
        cleanup.pop("_json", None)
    qoder_native_after_remove = None
    if target == "qoder" and kind == "plugin" and cleanup and cleanup["exit_code"] == 0:
        qoder_native_after_remove = qoder_native_plugin_state(
            home, env, project, identity)
        assert_case_unchanged()
    sentinels_preserved = (
        preserved_regular_text(sentinel, "foreign content never belongs to UAP\n", case_root)
        and preserved_regular_text(user_sentinel, "foreign profile content never belongs to UAP\n", case_root)
        and preserved_regular_text(native_foreign, "unmanaged native neighbor must survive\n", case_root)
        and preserved_regular_text(
            project_foreign, "---\nname: foreign-neighbor\ndescription: unrelated\n---\n", case_root)
    )
    post_skill_tree_delta = []
    if kind == "skill" and cleanup:
        after_skill_tree = snapshot([project / name for name in project_skill_roots])
        post_skill_tree_delta = sorted(
            path for path in before_skill_tree.keys() | after_skill_tree.keys()
            if before_skill_tree.get(path) != after_skill_tree.get(path))
    post_removal_skill = skill_file_present(project, target, identity) if kind == "skill" and cleanup else None
    post_removal_leftovers = (
        skill_removal_leftovers(project, target, identity, skill_path)
        if kind == "skill" and cleanup else [])
    post_list = (audited_call([cli, "list", "--target", target, *scope, "--format", "json"],
                      env, project, 30) if cleanup and cleanup["exit_code"] == 0 else None)
    post_list_json = post_list.pop("_json", None) if post_list else None
    copilot_native_after_remove = None
    if target == "copilot" and kind == "plugin" and cleanup and cleanup["exit_code"] == 0:
        copilot_native_after_remove = copilot_native_plugin_state(
            home, env, project, identity, owners={} if item == "marketing-kit" else None,
            owned_call=audited_call)
        assert_case_unchanged()
    copilot_after_remove = (copilot_settings_sanitized_shape(home, case_root, identity)
                            if copilot_before is not None and cleanup else None)
    remaining_ids = set()
    post_list_schema_valid = False
    if isinstance(post_list_json, dict):
        try:
            entries = post_list_json["data"]["installations"]
            if isinstance(entries, list) and all(isinstance(v, dict) for v in entries):
                post_list_schema_valid = True
                remaining_ids = {entry.get("installation_id") for entry in entries}
        except (KeyError, TypeError):
            pass
    old_ids = {entry["installation_id"] for entry in matching}
    # A successful remove must not re-register this identity under a new ID.
    post_remove_identity_registered = bool(
        post_list_schema_valid and (
            has_target_registration(post_list_json, identity, target)
            or (grok_prepared and grok_prepared_source_survives(
                post_list_json, item, matching[0]["name"], matching[0]["installation_id"]))))
    post_lock_pinned = bool(kind == "skill" and (project / "uap.lock").exists()
                            and locked_skill_is_pinned(project, item, target))
    unsafe_paths = [path for path, digest in changed
                    if digest in ("symlink", "unreadable", "non-regular")]
    # The remove step may create new native registrations after the add snapshot.
    # Rescan the disposable roots; preserve earlier observed unsafe paths too.
    native_residuals = (
        native_identity_residuals(
            native_roots, identity,
            sorted(set(snapshot(native_roots)) | set(native_attribution) | set(unsafe_paths)),
            target, native_components)
        if cleanup and cleanup["exit_code"] == 0 else []
    )
    # Preserve truthful disk-state evidence without equating passive cached
    # source/log files to an active Qoder/Codex plugin registration.
    passive_retained = []
    qoder_passive_log_link_verified = None
    if target == "qoder" and kind == "plugin" and cleanup:
        qoder_passive_log_link_verified = qoder_passive_latest_log_link(
            native_roots[0], native_roots[0] / "logs/latest")
    if cleanup and cleanup["exit_code"] == 0:
        for root in native_roots:
            prefix = root.name + "/"
            for observed in native_attribution:
                if not observed.startswith(prefix):
                    continue
                relative = observed[len(prefix):]
                path = root / relative
                if passive_native_path(target, relative) and path.is_file() and not path.is_symlink():
                    passive_retained.append(observed)
    copilot_disabled_setting_retained = bool(
        target == "copilot" and kind == "plugin"
        and copilot_disabled_preference_only(
            copilot_before, copilot_after_add, copilot_after_remove,
            copilot_native_before_remove, copilot_native_after_remove,
            native_residuals)
        and (item != "marketing-kit" or copilot_signed_kit_settings(
            home, case_root, kit_owners, False)))
    removal_verified = bool(
        cleanup and cleanup["exit_code"] == 0 and post_list and post_list["exit_code"] == 0
        and post_list_schema_valid and old_ids.isdisjoint(remaining_ids)
        and (target != "qoder" or kind != "plugin"
             or (qoder_native_after_remove is not None
                 and qoder_native_after_remove["status"] == "absent"))
        and (target != "copilot" or kind != "plugin" or item != "marketing-kit"
             or (copilot_native_after_remove is not None
                 and copilot_native_after_remove["status"] == "absent"))
        and not post_remove_identity_registered
        and (not native_residuals or copilot_disabled_setting_retained)
        and (kind != "skill" or (post_removal_skill is None
                                  and not post_removal_leftovers and not post_lock_pinned
                                  and not post_skill_tree_delta))
    )
    diagnostics = (output["stdout_tail"] + "\n" + output["stderr_tail"]).lower()
    install_refused = is_cli_refusal(output)
    if installed and removal_verified and sentinels_preserved and not repeat_verified:
        status = "repeat-add-unqualified"
    elif target == "chatgpt" and kind == "plugin" and installed and removal_verified and sentinels_preserved and repeat_verified:
        status = "manual-setup-prepared"
    elif installed and removal_verified and sentinels_preserved and repeat_verified:
        status = ("prepared-lifecycle-passed" if (
                       kind == "plugin" and prepared_components_only)
                  else "package-installed-native-unqualified" if (
                       kind == "plugin" and (
                           (target == "qoder" and
                            (qoder_native_before_remove is None or
                             qoder_native_before_remove["status"] != "registered"))
                           or (target == "copilot" and
                               (copilot_native_before_remove is None or
                                copilot_native_before_remove["status"] != "installed"))
                           or (target == "codex" and not (
                               native_uninstall and native_uninstall.get("completed")))
                           or (target not in ("codex", "qoder", "copilot")
                               and not native_attribution)))
                  else "prepared-lifecycle-passed" if prepared_binding else "passed")
    elif installed and cleanup and (
            "external removal requires explicit attestation" in
            (cleanup.get("stderr_tail", "") + cleanup.get("stdout_tail", "")).lower()
            or (target == "vscode" and kind == "plugin" and prepared_binding
                and repeat_verified and cleanup.get("exit_code") not in (None, 0)
                and "removal requires a manual client step for [vscode]" in
                cleanup.get("stderr_tail", "").lower()
                and "--external-uninstalled" in
                cleanup.get("stderr_tail", "").lower())):
        # A prepared VS Code package is not an activated native plugin.
        # Do not counterfeit the external-uninstalled attestation.
        status = "external-native-uninstall-required"
    elif installed:
        status = "cleanup-failed"
    elif failed_install_residue:
        status = "failed-install-residue"
    elif install_refused and host_containment_refusal(diagnostics):
        status = "host-containment-unavailable"
    elif install_refused and ("not detected" in diagnostics or "not available" in diagnostics
                             or "target_unavailable" in diagnostics
                             or (target == "windsurf" and kind == "plugin"
                                 and "clientconfigroot must be an explicit absolute clean path"
                                 in diagnostics)):
        status = "target-unavailable"
    elif install_refused and ("unsupported" in diagnostics or "not supported" in diagnostics
                              or source_target_incompatibility(diagnostics, target)):
        status = "documented-or-source-unsupported"
    elif install_refused and "minimum installer version" in diagnostics:
        status = "test-build-version-policy"
    elif install_refused and "distribution is suspended" in diagnostics:
        status = "expected-suspended-policy"
    elif (install_refused and target == "chatgpt" and kind == "plugin"
          and listed["exit_code"] == 0 and not failed_install_registered
          and not changed and sentinels_preserved
          and "group apply preflight failed; no target was changed" in output["stderr_tail"].lower()
          and "native identity ownership is indeterminate; refusing mutation"
          in output["stderr_tail"].lower()):
        # The native ownership preflight refused before mutating the disposable
        # profile. Never promote this or a source-unsupported case to native PASS.
        status = "native-ownership-unverified"
    elif output["exit_code"] == 0:
        status = "incomplete-native-proof"
    else:
        status = "install-failed"
    return {"kind": kind, "index": index, "identity": identity, "target": target,
            "source": item if kind == "plugin" else item["source"],
            "status": status, "install": output, "list": listed,
            "native_file_delta": len(changed),
            "native_identity_files": native_attribution,
            "native_component_candidates": list(native_components),
            "prepared_components_only": prepared_components_only,
            "skill_path": skill_path,
            "lock_written": bool(locked), "lock_contains_identity": bool(lock_contains),
            "lock_exact_source_pinned": lock_pinned, "skill_bytes_match": bool(skill_bytes_match),
            "listed_identity": listed_identity, "prepared_binding": prepared_binding,
            "signed_release_observation": signed_observation,
            "repeat_add": repeat_add, "repeat_list": repeat_list,
            "repeat_installation_ids": repeat_identity_ids,
            "repeat_native_unchanged": repeat_native_unchanged,
            "repeat_lock_unchanged": repeat_lock_unchanged,
            "repeat_project_unchanged": repeat_project_unchanged,
            "repeat_verified": repeat_verified,
            "failed_install_registered": failed_install_registered,
            "failed_install_residue": failed_install_residue,
            "matched_installation_ids": sorted(old_ids),
            "native_uninstall": native_uninstall, "cleanup": cleanup, "post_remove_list": post_list,
            "copilot_settings_proof": ({"before": copilot_before,
                                        "after_add": copilot_after_add,
                                        "after_remove": copilot_after_remove}
                                       if copilot_before is not None else None),
            "post_removal_skill": post_removal_skill,
            "post_removal_leftovers": post_removal_leftovers,
            "post_skill_tree_delta": post_skill_tree_delta,
            "post_lock_pinned": post_lock_pinned,
            "removal_verified": removal_verified, "native_residuals": native_residuals,
            "retained_passive_native_files": sorted(set(passive_retained)),
            "qoder_passive_log_link_verified": qoder_passive_log_link_verified,
            "qoder_native_after_repeat": qoder_native_before_remove,
            "qoder_native_after_remove": qoder_native_after_remove,
            "copilot_native_after_repeat": copilot_native_before_remove,
            "copilot_native_after_remove": copilot_native_after_remove,
            "copilot_disabled_setting_retained": copilot_disabled_setting_retained,
            "post_list_schema_valid": post_list_schema_valid,
            "post_remove_identity_registered": post_remove_identity_registered,
            "sentinels_preserved": sentinels_preserved}


def expected_negative_coverage(row, cli_version=None):
    """Accept only verified *non-native* outcomes, never relabel them PASS.

    The published release has frozen unsupported target combinations, and
    OpenCode can complete a prepared package lifecycle without proving native
    activation. Fail closed on side effects, missing cleanup, or new sources.
    """
    if not row.get("sentinels_preserved") or row.get("failed_install_residue"):
        return False
    target = row.get("target")
    if row.get("kind") != "plugin":
        return False

    # A completed OpenCode package lifecycle can be non-native even when
    # prepared Skills are materialized under the isolated native profile.
    # The entire identity must be removed; a surviving file is never accepted.
    native_files = row.get("native_identity_files")
    if (target == "opencode"
            and isinstance(native_files, list)
            and all(isinstance(path, str) and path.startswith("opencode/")
                    and ".." not in Path(path).parts for path in native_files)
            and row.get("native_component_candidates") == []
            and row.get("listed_identity") is True
            and row.get("repeat_verified") is True
            and row.get("removal_verified") is True
            and row.get("post_remove_identity_registered") is False
            and row.get("native_residuals") == []
            and isinstance(row.get("install"), dict)
            and row["install"].get("exit_code") == 0
            and isinstance(row.get("cleanup"), dict)
            and row["cleanup"].get("exit_code") == 0):
        if (row.get("status") == "prepared-lifecycle-passed"
                and row.get("prepared_components_only") is True):
            return True
        # Only these immutable sources in the verified 0.1.95 / 0.1.101 / 0.1.102
        # catalog have a clean lifecycle but no attributable native component.
        # An unknown or newly unqualified source must still fail this audit.
        non_native_direct_sources = {
            "github:itential/builder-skills@2ccdf77358b018ad4df125422a067fe84e63de1d",
            "github:GoogleCloudPlatform/google-cloud-storage-plugin@b346e39698fcdfdc711ca3ee35e41c8dbaa1aa2b",
            "github:cometchat/cometchat-skills@8652f1ea41d12e5058123db1e4e6babef9c918f1",
            "github:uizze/uizze@fc401e9e654cbafe427b0c98142fc8bccd0c4e7e",
        }
        if (row.get("status") == "package-installed-native-unqualified"
                and cli_version in ("agentplugins 0.1.95", "agentplugins 0.1.101", "agentplugins 0.1.102")
                and row.get("prepared_components_only") is False
                and native_files == []
                and row.get("source") in non_native_direct_sources):
            return True

    if (row.get("status") != "documented-or-source-unsupported"
            or row.get("native_file_delta") != 0
            or row.get("native_identity_files")
            or row.get("listed_identity")
            or row.get("failed_install_registered")
            or not isinstance(row.get("install"), dict)
            or type(row["install"].get("exit_code")) is not int
            or row["install"]["exit_code"] == 0):
        return False
    diagnostic = row["install"].get("stderr_tail", "")
    if not isinstance(diagnostic, str):
        return False
    if (row.get("source") == "marketing-kit"
            and target in ("cline", "copilot", "kimi", "qoder", "grok")
            and "release does not support complete target set" in diagnostic
            and ("missing " + target) in diagnostic):
        return True
    # Pi's generic plugin route cannot install these frozen sources when the
    # signed release explicitly lacks pi or its own manifest prevents native
    # extension discovery. Refusals are negative, never a native installation.
    return bool(target == "pi"
                and (("release does not support complete target set" in diagnostic
                      and "missing pi" in diagnostic)
                     or "pi package manifest overrides selected extension discovery"
                        in diagnostic))


def unexecuted_source_rows(selected, kind, target, status, declared_names):
    """Record exact fixture slots on a refused bootstrap, never native PASS."""
    if status not in ("client-unavailable", "documented-unsupported"):
        raise ValueError("only non-execution status permitted")
    return [{"target": target, "kind": kind, "index": index,
             "source": item if kind == "plugin" else item["source"],
             "identity": declared_names.get(item, item) if kind == "plugin" else item["name"],
             "status": status}
            for index, item in enumerate(selected)]


def vendor_bootstrap_attested():
    """A failed vendor bootstrap cannot qualify an unrelated runner binary."""
    return os.environ.get("AUDIT_VENDOR_OUTCOME") == "success"


def published_audit_temp_parent():
    """Keep GitHub Windows deep fixtures inside the real short runner scratch.

    The published 0.1.101 CLI has a separately documented long-path bug.
    This qualifies an ordinary bounded Windows project, not MAX_PATH support.
    """
    if sys.platform != "win32" or os.environ.get("GITHUB_ACTIONS") != "true":
        return None
    candidate = os.environ.get("RUNNER_TEMP", "")
    parent = Path(candidate)
    if not candidate or not parent.is_absolute() or parent.is_symlink() or not parent.is_dir():
        raise RuntimeError("Windows GitHub runner scratch is unavailable or redirected")
    return str(parent)


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
    source_sha = os.environ.get("GITHUB_SHA", "")
    pr_head_sha = os.environ.get("AUDIT_PR_HEAD_SHA", "")
    workflow_run = os.environ.get("GITHUB_RUN_ID", "")
    report = {"checked_at": datetime.now(timezone.utc).isoformat(),
              "target": args.target, "kind": args.kind, "cli": args.cli,
              "github_sha": source_sha if re.fullmatch(r"[a-f0-9]{40}", source_sha) else None,
              "pr_head_sha": pr_head_sha if re.fullmatch(r"[a-f0-9]{40}", pr_head_sha) else None,
              "github_run_id": workflow_run if workflow_run.isdigit() else None,
              "fixture_sha256": hashlib.sha256(FIXTURES.read_bytes()).hexdigest(),
              "platform": sys.platform, "architecture": platform.machine(),
              "client_binary": None, "client_version": {"status": "not-probed"},
              "cli_version": {"status": "not-probed"},
              "fixture_count": len(cases), "cases_attempted": 0,
              "source_selection_complete": args.limit == 30,
              "full_coverage": False, "results": []}
    report_file = report_dir / f"{args.target}-{args.kind}.json"
    def save():
        report_file.write_text(json.dumps(report, indent=2, ensure_ascii=False) + "\n")
    eligible_binaries = EXECUTABLE.get(args.target, ())
    if not vendor_bootstrap_attested():
        report["blocker"] = "Vendor bootstrap was not verified; no native-client PASS is permitted"
        report["results"] = unexecuted_source_rows(
            selected, args.kind, args.target, "client-unavailable",
            doc["plugin_declared_names"])
        save()
        print(report["blocker"], file=sys.stderr)
        return 2
    if eligible_binaries and not any(AGENT_BINARIES.get(name) for name in eligible_binaries):
        report["blocker"] = (
            "Genuine client executable not installed; tried " + ", ".join(eligible_binaries)
        )
        report["results"] = unexecuted_source_rows(
            selected, args.kind, args.target, "client-unavailable",
            doc["plugin_declared_names"])
        save()
        print(report["blocker"], file=sys.stderr)
        return 2
    if args.target == "chatgpt" and args.kind == "skill":
        report["blocker"] = "ChatGPT has no native project skill filesystem adapter"
        report["results"] = unexecuted_source_rows(
            selected, args.kind, args.target, "documented-unsupported",
            doc["plugin_declared_names"])
        save()
        return 2
    cli = str(Path(args.cli).resolve(strict=True))
    # Real Windows runner scratch avoids accidental >260-character internal
    # preview paths for deep immutable Skills (tracked separately in PR #780).
    # This does not convert a refused installation into native PASS.
    temp_parent = published_audit_temp_parent()
    report["temp_parent_kind"] = ("github-windows-runner" if temp_parent else "python-default")
    with tempfile.TemporaryDirectory(prefix="uap-live-30x30-", dir=temp_parent) as temp:
        runroot = Path(temp)
        probe_case = runroot / "version-probe"
        probe_home, probe_data = probe_case / "home", probe_case / "uap-data"
        probe_home.mkdir(parents=True)
        probe_data.mkdir()
        probe_env = isolated_client_env(probe_home, probe_case, probe_data)
        report["cli_version"] = version_evidence([cli, "version"], probe_env, probe_case)
        if eligible_binaries:
            native = next(AGENT_BINARIES[name] for name in eligible_binaries
                          if AGENT_BINARIES.get(name))
            report["client_binary"] = native
            report["client_version"] = version_evidence([native, "--version"], probe_env, probe_case)
        save()
        for i, item in enumerate(selected):
            try:
                result = do_case(cli, args.target, args.kind, item, i, runroot,
                                 doc["plugin_declared_names"],
                                 doc["signed_release_pins"])
            except Exception as e:
                result = {"kind": args.kind, "target": args.target, "index": i,
                          "source": item if args.kind == "plugin" else item["source"],
                          "identity": (doc["plugin_declared_names"].get(item, item)
                                       if args.kind == "plugin" else item["name"]),
                          "status": "harness-error", "error": repr(e)}
            report["results"].append(result)
            report["cases_attempted"] += 1
            save()
            line = {"target": args.target, "kind": args.kind, "index": i,
                    "status": result["status"]}
            if result["status"] == "incomplete-native-proof" and args.kind == "plugin":
                line["observed_signed_release"] = result.get("signed_release_observation", [])
                line["expected_signed_release"] = doc["signed_release_pins"].get(item)
            print(json.dumps(line), flush=True)
    passed = sum(r["status"] == "passed" for r in report["results"])
    report["passed"] = passed
    report["prepared_lifecycle_passed"] = sum(
        r["status"] == "prepared-lifecycle-passed" for r in report["results"])
    accepted_negatives = [
        r for r in report["results"]
        if expected_negative_coverage(r, report["cli_version"].get("value"))
    ]
    report["verified_negative_coverage"] = len(accepted_negatives)
    report["full_coverage"] = args.limit == 30 and report["cases_attempted"] == 30
    save()
    print(f'{args.target} {args.kind}: {passed}/{args.limit} native PASS; '
          f'verified_negative={len(accepted_negatives)}; attempted={report["cases_attempted"]}')
    return 0 if (report["cases_attempted"] == args.limit
                 and passed + len(accepted_negatives) == args.limit) else 1


if __name__ == "__main__":
    sys.exit(main())
