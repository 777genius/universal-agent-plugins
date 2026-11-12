"""Pinned native API admission research; never product or activation acceptance.

The only mutation experiment enables an inert, test-authored Grok Skill inside
fresh disposable HOME/projects. No MCP process, provider or real profile is used.
"""
import argparse
import base64
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import subprocess
import tempfile

CLIENTS = ("cline", "grok")
NAME = "uap-automation-probe"
SOURCE = Path(__file__).resolve().parent.parent


def require(ok, message):
    if not ok:
        raise ValueError(message)


def put(path, body):
    path.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
    with path.open("x", encoding="utf-8") as stream:
        stream.write(body if isinstance(body, str) else json.dumps(body, sort_keys=True))
    path.chmod(0o600)


def snapshot(root):
    """Capture ordinary test config bytes, not aliases or an unbounded cache."""
    out = {}
    for path in sorted(root.rglob("*")):
        meta = path.lstat()
        require(not path.is_symlink(), "native probe created an alias")
        if path.is_dir():
            continue
        require(stat.S_ISREG(meta.st_mode) and meta.st_size <= 1 << 20, "config capture bound")
        raw = path.read_bytes()
        out[str(path.relative_to(root))] = {
            "sha256": hashlib.sha256(raw).hexdigest(), "mode": stat.S_IMODE(meta.st_mode),
            "bytes": len(raw), "base64": base64.b64encode(raw).decode(),
        }
        require(len(out) <= 256, "config inventory bound")
    return out


def exact_plugin_id(body, path):
    """Accept only an explicit scoped native ID joined to one exact path."""
    def pairs(items):
        result = {}
        for key, value in items:
            require(key not in result, "duplicate native JSON key")
            result[key] = value
        return result
    value = json.loads(body, object_pairs_hook=pairs)
    rows = value if isinstance(value, list) else value.get("plugins") if isinstance(value, dict) else None
    require(isinstance(rows, list), "unqualified native list envelope")
    matches = [row for row in rows if isinstance(row, dict) and row.get("path") == str(path)]
    require(len(matches) == 1, "exact native plugin path missing or duplicated")
    selected = matches[0]
    ident = selected.get("id")
    require(selected.get("name") == NAME and selected.get("scope") == "project"
            and isinstance(ident, str) and re.fullmatch(r"project/[A-Za-z0-9_-]{4,128}/" + NAME, ident),
            "no qualified project-native full ID")
    return ident


class Probe:
    def __init__(self, root, binary, evidence):
        self.root, self.binary, self.evidence = root, binary, evidence
        self.home, self.a, self.b = [root / part for part in ("home", "project-a", "project-b")]
        for path in (self.home, self.a, self.b, root / "tmp"):
            path.mkdir(mode=0o700)
        self.env = {"HOME": str(self.home), "PATH": str(binary.parent) + ":/usr/bin:/bin",
                    "LANG": "C.UTF-8", "LC_ALL": "C.UTF-8", "NO_COLOR": "1", "TERM": "dumb",
                    "TMPDIR": str(root / "tmp"), "GIT_CONFIG_NOSYSTEM": "1",
                    "GIT_CONFIG_GLOBAL": "/dev/null", "GIT_TERMINAL_PROMPT": "0",
                    "GROK_DISABLE_AUTOUPDATER": "1", "CLINE_NO_AUTO_UPDATE": "1"}
        for part in ("CONFIG", "DATA", "CACHE", "STATE"):
            directory = root / ("xdg-" + part.lower())
            directory.mkdir(mode=0o700)
            self.env["XDG_" + part + "_HOME"] = str(directory)
        for project in (self.a, self.b):
            self.run("git-" + project.name, ["/usr/bin/git", "-c", "core.hooksPath=/dev/null", "init", "--quiet"], project)

    def run(self, label, args, cwd=None, overrides=None):
        from native_client_research import run_probe
        record, stdout, stderr = run_probe([str(item) for item in args], cwd or self.a,
            dict(self.env, **(overrides or {})), self.root, label, timeout=30,
            limit=1 << 20, memory_only=True)
        self.evidence["commands"].append(dict(record,
            stdout_base64=base64.b64encode(stdout).decode(),
            stderr_base64=base64.b64encode(stderr).decode()))
        require(record["error"] is None, "native capture failed: " + label)
        return record["exit"], stdout

    def grok(self):
        for label, args in (("version", ["--version"]), ("plugin-help", ["plugin", "--help"]),
                            ("enable-help", ["plugin", "enable", "--help"]),
                            ("list-help", ["plugin", "list", "--help"])):
            self.run(label, [self.binary, *args])
        put(self.home / ".grok/config.toml",
            '[marketplace]\ndefault_skills_installs_purged = true\n[features]\nturn_summary = false\n')
        roots = [self.home / ".grok/plugins/user-probe", self.a / ".grok/plugins/project-probe",
                 self.b / ".grok/plugins/project-probe"]
        for index, root in enumerate(roots):
            put(root / ".grok-plugin/plugin.json", {"name": NAME, "version": "1.0.0", "skills": "./skills/"})
            put(root / ("skills/probe-" + str(index) + "/SKILL.md"),
                "---\nname: probe-" + str(index) + "\ndescription: Inert scope identity fixture\n---\nText only.\n")
        code, listing = self.run("list-before-a", [self.binary, "plugin", "list", "--json"])
        self.run("list-before-b", [self.binary, "plugin", "list", "--json"], self.b)
        self.run("inspect-before-a", [self.binary, "inspect", "--json"])
        before = {"user": snapshot(self.home / ".grok"), "a": snapshot(self.a / ".grok"),
                  "b": snapshot(self.b / ".grok")}
        self.evidence["before"] = before
        # A name-only mutator is deliberately NEVER called. Absence of a stable
        # full native ID is evidence of a missing admission prerequisite.
        try:
            require(code == 0, "native plugin list failed")
            ident = exact_plugin_id(listing, roots[1])
        except (ValueError, TypeError) as error:
            self.evidence["enable_admission"] = {"admitted": False, "reason": str(error)}
        else:
            exit_code, _ = self.run("enable-exact-project-id", [self.binary, "plugin", "enable", ident])
            self.evidence["enable_admission"] = {"admitted": True, "id": ident, "exit": exit_code}
            self.run("list-after-a", [self.binary, "plugin", "list", "--json"])
            self.run("list-after-b", [self.binary, "plugin", "list", "--json"], self.b)
            self.run("inspect-after-a", [self.binary, "inspect", "--json"])
        self.evidence["after"] = {"user": snapshot(self.home / ".grok"),
            "a": snapshot(self.a / ".grok"), "b": snapshot(self.b / ".grok")}
        require(not (self.home / ".grok/trusted_folders.toml").exists(), "native experiment changed trust")
        require(self.evidence["after"]["a"] == before["a"] and self.evidence["after"]["b"] == before["b"],
                "native enable changed project package bytes")

    def cline(self):
        for label, args in (("version", ["--version"]), ("help", ["--help"]),
                            ("mcp-help", ["mcp", "--help"]),
                            ("mcp-list-help", ["mcp", "list", "--help"])):
            self.run(label, [self.binary, "--auto-approve", "false", *args])
        # Disabled entries prove source selection without executing an MCP server.
        user = self.home / ".cline/data/settings/cline_mcp_settings.json"
        project = self.a / ".uap/clients/cline/mcp.json"
        for path, name in ((user, "user-sentinel"), (project, "project-sentinel")):
            put(path, {"mcpServers": {name: {"command": "/usr/bin/false", "disabled": True,
                "autoApprove": [], "args": []}}})
        self.run("mcp-default", [self.binary, "--auto-approve", "false", "mcp", "list"])
        self.run("mcp-selected-a", [self.binary, "--auto-approve", "false", "mcp", "list"],
                 overrides={"CLINE_MCP_SETTINGS_PATH": str(project)})
        self.run("mcp-default-b", [self.binary, "--auto-approve", "false", "mcp", "list"], self.b)
        self.evidence["selected_environment"] = {"CLINE_MCP_SETTINGS_PATH": str(project),
                                                "CLINE_NO_AUTO_UPDATE": "1"}
        self.evidence["configuration"] = {"user": snapshot(user.parent), "project": snapshot(project.parent)}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--client", required=True, choices=CLIENTS)
    parser.add_argument("--output", required=True, type=Path)
    args = parser.parse_args()
    require(os.environ.get("GITHUB_ACTIONS") == "true"
            and os.environ.get("RUNNER_ENVIRONMENT") == "github-hosted", "disposable hosted runner required")
    expected = os.environ.get("EXPECTED_COMMIT", "")
    require(re.fullmatch(r"[a-f0-9]{40}", expected), "exact source SHA required")
    actual = subprocess.check_output(["/usr/bin/git", "rev-parse", "HEAD"], cwd=SOURCE, timeout=10).decode().strip()
    require(actual == expected, "checkout differs from reviewed head")
    tree = subprocess.check_output(["/usr/bin/git", "rev-parse", "HEAD^{tree}"], cwd=SOURCE, timeout=10).decode().strip()
    args.output.mkdir(mode=0o700, parents=True, exist_ok=False)
    evidence = {"schema_version": 1, "client": args.client, "source_commit": actual,
                "source_tree": tree, "commands": [], "research_completed": False,
                "qualified_product_e2e": False, "provider_authentication": False, "model_invoked": False}
    try:
        from project_native_research import Budget, acquire, materialize, verify_files
        if args.client == "grok":
            from project_native_collectors.grok import ACQUISITION
        else:
            from project_native_collectors.cline import ACQUISITION
        budget = Budget(600)
        receipt, contents = acquire(ACQUISITION, budget)
        evidence["acquisition"] = receipt
        with tempfile.TemporaryDirectory(prefix="uap-native-api-", dir=os.environ["RUNNER_TEMP"]) as temporary:
            root = Path(temporary).resolve()
            (root / "bin").mkdir(mode=0o700)
            binary, identities = materialize(root / "bin", ACQUISITION, receipt, contents)
            probe = Probe(root, binary, evidence)
            getattr(probe, args.client)()
            verify_files(identities, budget)
        evidence["research_completed"] = True
    except Exception as error:
        evidence["error"] = type(error).__name__ + ": " + str(error)
        raise
    finally:
        put(args.output / "native-api.json", evidence)
        print(json.dumps({"research_completed": evidence["research_completed"],
                          "qualified_product_e2e": False, "client": args.client}))


if __name__ == "__main__":
    main()
