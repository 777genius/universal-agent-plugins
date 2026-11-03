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
    "codex": ("codex",), "copilot": ("copilot",), "cline": ("cline",),
    "claude": ("claude",), "gemini": ("gemini",),
    "opencode": ("opencode",), "pi": ("pi",),
    "cursor": ("cursor-agent", "cursor"), "vscode": ("code",),
    "kiro": ("kiro-cli", "kiro"), "grok": ("grok",),
    "kimi": ("kimi",), "qoder": ("qoder", "qodercli"),
    "windsurf": ("devin", "windsurf"),
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
    direct = doc["plugin_declared_names"]
    exact_git = [v for v in names if v.startswith("github:")]
    if set(direct) != set(exact_git + ["777genius/chrome-devtools-bridge"]):
        raise ValueError("All external manifest identities must be supplied and no stale alias mappings")
    for plugin in exact_git:
        if not re.fullmatch(r"github:[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+@[a-f0-9]{40}", plugin):
            raise ValueError("Direct plugin source is not a pinned root package")
        if not re.fullmatch(r"[a-z][a-z0-9-]*", direct[plugin]):
            raise ValueError("Invalid direct package declared identity")
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
                           text=True, errors="replace", stdin=subprocess.DEVNULL,
                           timeout=timeout, check=False)
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


def attributable_native_changes(native_roots, changes, identity):
    """A changed unrelated config cannot claim the plugin was projected.

    Evidence must come from the managed name in the changed path or content,
    cross-checked with the exact verified CLI installation record.
    """
    found = []
    for root in native_roots:
        if not root.is_dir() or root.is_symlink():
            continue
        for relpath, _digest in changes:
            prefix = root.name + "/"
            if not relpath.startswith(prefix):
                continue
            leaf = root / relpath[len(prefix):]
            if leaf.is_symlink() or not leaf.is_file():
                continue
            try:
                if leaf.stat().st_size > 1024 * 1024:
                    continue
                owned_name = identity.casefold() in relpath.casefold()
                owned_content = identity.casefold() in leaf.read_text(
                    encoding="utf-8", errors="replace").casefold()
                if owned_name or owned_content:
                    found.append(relpath)
            except OSError:
                continue
    return sorted(set(found))


def source_target_incompatibility(diagnostic):
    """A signed package with no eligible adapter target is unsupported, not broken."""
    return "release does not support complete target set" in diagnostic.lower()


def native_identity_residuals(native_roots, identity, observed_paths, target):
    """After remove, reject any remaining named native file/config entry.

    Codex may retain its downloaded source cache after its native plugin and
    marketplace are removed; that cache is not active client configuration.
    """
    leftover = []
    for root in native_roots:
        prefix = root.name + "/"
        for observed in observed_paths:
            if not observed.startswith(prefix):
                continue
            relative = observed[len(prefix):]
            if target == "codex" and relative.startswith("plugins/cache/"):
                continue
            file = root / relative
            if file.is_symlink():
                leftover.append(observed + ": symlink")
                continue
            if not file.is_file():
                continue
            try:
                if identity.casefold() in relative.casefold():
                    leftover.append(observed)
                elif file.stat().st_size <= 1024 * 1024 and identity.casefold() in file.read_text(
                        encoding="utf-8", errors="replace").casefold():
                    leftover.append(observed)
            except OSError:
                leftover.append(observed + ": unreadable")
    return sorted(set(leftover))


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
            (client.get("verification") == "installation_verified" or
             (kind == "plugin" and client.get("materialization") == "materialized" and
              client.get("verification") == "package_validated" and
              client.get("activation") == "manual_activation_required"))
            for client in clients):
            continue
        source = node.get("source", "")
        if kind == "skill":
            accepted = source == expected_source
        elif item == "777genius/chrome-devtools-bridge":
            accepted = node.get("name") == identity and isinstance(source, str)
        elif isinstance(item, str) and item.startswith("github:"):
            expected_repo = item.split("@", 1)[0][len("github:"):].casefold()
            accepted = (node.get("name") == identity and isinstance(source, str)
                        and expected_repo in source.casefold())
        else:
            accepted = node.get("name") == identity
        if accepted and node.get("installation_id"):
            # An exact-SHA Git selector must resolve to that SHA, not merely to
            # a repository with a matching display name. Validate the native
            # binding's recorded immutable commit, not CLI stdout prose.
            if kind == "plugin" and isinstance(item, str) and item.startswith("github:"):
                commit = item.rsplit("@", 1)[-1]
                if not any(
                    isinstance(client, dict) and client.get("client_id") == target
                    and client.get("package_revision", {}).get("resolved_revision") == commit
                    for client in clients
                ):
                    continue
            # A catalogue name is not enough to prove the selected signed
            # distribution: tie reviewed aliases to the signed directory
            # product and ensure immutable source revision/digests exist.
            if kind == "plugin" and isinstance(item, str) and not item.startswith("github:"):
                directory = node.get("directory", {})
                if not isinstance(directory, dict) or (
                    (item == "777genius/chrome-devtools-bridge" and
                     directory.get("recorded_distribution") != item) or
                    (item != "777genius/chrome-devtools-bridge" and
                     directory.get("product_id") != item)):
                    continue
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
    temp = case_root / "tmp"
    temp.mkdir(exist_ok=True)
    roots = {
        "CODEX_HOME": home / ".codex",
        "CLAUDE_CONFIG_DIR": home / ".claude",
        "GEMINI_CLI_HOME": home,  # Gemini appends /.gemini itself
        "QODER_CONFIG_DIR": home / ".qoder",
        "PI_CODING_AGENT_DIR": home / ".pi/agent",
        "KIMI_CODE_HOME": home / ".kimi-code",
        "KIMI_CODE_CACHE_DIR": home / ".cache/kimi-code",
        "GROK_HOME": home / ".grok",
        "CLINE_DATA_DIR": home / ".cline/data",
        "CLINE_MCP_SETTINGS_PATH": home / ".cline/data/settings/cline_mcp_settings.json",
    }
    if any(not path.is_relative_to(home) for path in roots.values()):
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


def do_case(cli, target, kind, item, index, runroot, plugin_declared_names):
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
    # A foreign file lives adjacent to installed native user artifacts.
    # Its content and position are independent from UAP-managed receipts.
    native_foreign = (native_roots[0] / "foreign-neighbor.txt") if target == "codex" else None
    if native_foreign:
        native_foreign.parent.mkdir(parents=True, exist_ok=True)
        native_foreign.write_text("unmanaged native neighbor must survive\n")
    # A pre-existing, unrelated project Skill is an independently owned neighbor.
    project_skill_roots = PROJECT_SKILL_ROOTS.get(target, [])
    project_foreign = ((project / project_skill_roots[0] / "foreign-neighbor" / "SKILL.md")
                       if kind == "skill" and project_skill_roots else None)
    if project_foreign:
        project_foreign.parent.mkdir(parents=True, exist_ok=True)
        project_foreign.write_text("---\nname: foreign-neighbor\ndescription: unrelated\n---\n")
    # Presence of an actual binary comes from the workflow; a config directory is
    # only an empty isolated profile and is never counted as agent detection.
    if target in ("codex", "claude", "opencode", "gemini", "copilot", "pi"):
        native_roots[0].mkdir(parents=True, exist_ok=True)
    env = isolated_client_env(home, case_root, data)
    before = snapshot(native_roots)
    if kind == "plugin":
        identity = plugin_declared_names.get(item, item)
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
    native_attribution = attributable_native_changes(native_roots, changed, identity)
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
    prepared_binding = bool(listed_identity and kind == "plugin" and any(
        client.get("client_id") == target and client.get("verification") == "package_validated"
        and client.get("activation") == "manual_activation_required"
        for client in matching[0].get("clients", []) if isinstance(client, dict)))
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
                           user_sentinel.read_text() == "foreign profile content never belongs to UAP\n" and
                           (not native_foreign or native_foreign.read_text() ==
                            "unmanaged native neighbor must survive\n") and
                           (not project_foreign or project_foreign.read_text() ==
                            "---\nname: foreign-neighbor\ndescription: unrelated\n---\n"))
    post_removal_skill = skill_file_present(project, target, identity) if kind == "skill" and cleanup else None
    post_list = (call([cli, "list", "--target", target, *scope, "--format", "json"],
                      env, project, 30) if cleanup and cleanup["exit_code"] == 0 else None)
    post_list_json = post_list.pop("_json", None) if post_list else None
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
    post_lock_pinned = bool(kind == "skill" and (project / "uap.lock").exists()
                            and locked_skill_is_pinned(project, item, target))
    native_residuals = (native_identity_residuals(native_roots, identity, native_attribution, target)
                        if cleanup and cleanup["exit_code"] == 0 and kind == "plugin" else [])
    removal_verified = bool(
        cleanup and cleanup["exit_code"] == 0 and post_list and post_list["exit_code"] == 0
        and post_list_schema_valid and old_ids.isdisjoint(remaining_ids)
        and (kind != "plugin" or not native_residuals)
        and (kind != "skill" or (post_removal_skill is None and not post_lock_pinned))
    )
    diagnostics = (output["stdout_tail"] + "\n" + output["stderr_tail"]).lower()
    if target == "chatgpt" and kind == "plugin" and output["exit_code"] == 0:
        status = "manual-setup-prepared"
    elif installed and removal_verified and sentinels_preserved:
        status = ("package-installed-native-unqualified" if kind == "plugin" and not native_attribution
                  else "prepared-lifecycle-passed" if prepared_binding else "passed")
    elif installed and cleanup and "external removal requires explicit attestation" in (
            cleanup.get("stderr_tail", "") + cleanup.get("stdout_tail", "")).lower():
        status = "external-native-uninstall-required"
    elif installed:
        status = "cleanup-failed"
    elif "manual_activation_required" in diagnostics and "cgroup" in diagnostics:
        status = "host-containment-unavailable"
    elif "not detected" in diagnostics or "not available" in diagnostics or "target_unavailable" in diagnostics:
        status = "target-unavailable"
    elif ("unsupported" in diagnostics or "not supported" in diagnostics
          or source_target_incompatibility(diagnostics)):
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
            "native_file_delta": len(changed),
            "native_identity_files": native_attribution, "skill_path": skill_path,
            "lock_written": bool(locked), "lock_contains_identity": bool(lock_contains),
            "lock_exact_source_pinned": lock_pinned, "skill_bytes_match": bool(skill_bytes_match),
            "listed_identity": listed_identity, "prepared_binding": prepared_binding,
            "matched_installation_ids": sorted(old_ids),
            "native_uninstall": native_uninstall, "cleanup": cleanup, "post_remove_list": post_list,
            "post_removal_skill": post_removal_skill, "post_lock_pinned": post_lock_pinned,
            "removal_verified": removal_verified, "native_residuals": native_residuals,
            "post_list_schema_valid": post_list_schema_valid,
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
              "source_selection_complete": args.limit == 30,
              "full_coverage": False, "results": []}
    report_file = report_dir / f"{args.target}-{args.kind}.json"
    def save():
        report_file.write_text(json.dumps(report, indent=2, ensure_ascii=False) + "\n")
    eligible_binaries = EXECUTABLE.get(args.target, ())
    if eligible_binaries and not any(shutil.which(name) for name in eligible_binaries):
        report["blocker"] = (
            "Genuine client executable not installed; tried " + ", ".join(eligible_binaries)
        )
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
                result = do_case(cli, args.target, args.kind, item, i, runroot,
                                 doc["plugin_declared_names"])
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
    report["prepared_lifecycle_passed"] = sum(
        r["status"] == "prepared-lifecycle-passed" for r in report["results"])
    report["full_coverage"] = args.limit == 30 and report["cases_attempted"] == 30
    save()
    print(f'{args.target} {args.kind}: {passed}/{args.limit} passed; attempted={report["cases_attempted"]}')
    return 0 if passed == args.limit else 1


if __name__ == "__main__":
    sys.exit(main())
