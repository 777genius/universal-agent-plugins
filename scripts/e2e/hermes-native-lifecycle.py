#!/usr/bin/env python3
"""One real Hermes/UAP lifecycle for Linux, macOS and native Windows."""
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile

HELPER = Path(__file__).with_name("hermes-native.py")


def require(condition, message):
    if not condition:
        raise RuntimeError(message)


class Lifecycle:
    def __init__(self, root):
        self.root = root
        self.profile = Path(os.environ["HERMES_HOME"])
        self.evidence = Path(os.environ["HERMES_NATIVE_EVIDENCE"])
        self.uap = os.environ["UAP_BIN"]
        self.hermes = [os.environ["HERMES_BIN"]]
        runtime_file = os.environ.get("HERMES_NATIVE_RUNTIME_COMMAND_FILE")
        if runtime_file:
            self.hermes = json.loads(Path(runtime_file).read_text(encoding="utf-8-sig"))
            require(isinstance(self.hermes, list) and len(self.hermes) >= 4
                    and all(isinstance(arg, str) for arg in self.hermes),
                    "official Hermes runtime command is not an argv array")
        self.user_source = root / "global-src"
        self.project_source = root / "project-src"
        self.project_a = root / "project"
        self.project_b = root / "project-b"
        self.project_c = root / "project-c"
        self.config = self.profile / "config.yaml"
        self.native_home = None

    def capture(self, command, name, cwd=None, expected_failure=False, env=None):
        output = self.evidence / name
        error = self.evidence / (name + ".stderr.txt")
        with output.open("wb") as stdout, error.open("wb") as stderr:
            # The outer watchdog owns the complete process tree. Killing a
            # launcher here could orphan its children before that cleanup.
            result = subprocess.run([str(arg) for arg in command], cwd=cwd or self.root,
                                    env=env, stdout=stdout, stderr=stderr,
                                    check=False)
        if (result.returncode != 0) != expected_failure:
            raise RuntimeError(f"{name}: unexpected exit {result.returncode}\n"
                               + output.read_text(encoding="utf-8", errors="replace")
                               + error.read_text(encoding="utf-8", errors="replace"))
        return output

    def run_uap(self, stage, *args):
        return self.capture([self.uap, *args, "--target", "hermes", "--yes",
                             "--format", "json"], stage + "-uap.json")

    def observe(self, stage, cwd, marker, project_count, trusted=False):
        # Observe the profile a bare `hermes` resolves, never an assumed file.
        env = dict(os.environ, TERMINAL_CWD=str(cwd), HERMES_HOME=str(self.native_home))
        output = self.capture([sys.executable, HELPER, "observe", marker, str(project_count),
                               "trusted" if trusted else "untrusted"],
                              stage + "-skills.json", cwd, env=env)
        print(output.read_text(encoding="utf-8"), end="", flush=True)

    def resolved_home(self, stage):
        output = self.capture([*self.hermes, "config", "path"], stage + "-config-path.txt")
        return Path(output.read_text(encoding="utf-8").strip()).parent.resolve()

    def hermes_json(self, stage, key):
        output = self.capture([*self.hermes, "config", "get", "--json", key], stage + "-" + key + ".json")
        return json.loads(output.read_text(encoding="utf-8"))

    def trusted_roots(self, stage):
        value = self.hermes_json(stage, "skills.trusted_project_dirs")
        value = [] if value is None else [value] if isinstance(value, str) else value
        require(isinstance(value, list) and all(isinstance(item, str) for item in value),
                "native Hermes trust list is not a list of paths")
        return {Path(item).resolve() for item in value}

    def require_trust_report(self, output, granted):
        text = output.read_text(encoding="utf-8")
        if granted:
            require("Hermes now trusts the Git repository root" in text,
                    "confirmed Project install did not report verified Hermes trust")
        else:
            require("UAP did not change Hermes project trust" in text
                    and "hermes skills trust " in text,
                    "Project install did not report the exact manual Hermes trust command")

    def probe_mcp(self, stage):
        self.capture([*self.hermes, "mcp", "test", "uap-native-test-server"],
                     stage + "-mcp.txt")

    def skill(self, scope, root=None):
        base = root if root is not None else self.profile
        return base / "skills" / ("native-smoke-" + scope) / "SKILL.md"

    def project_skill(self, project):
        return self.skill("project", project / ".hermes")

    def prepare(self):
        self.profile.mkdir(parents=True, exist_ok=True)
        for project in (self.project_a, self.project_b):
            project.mkdir()
            subprocess.run(["git", "-C", str(project), "init", "-q"],
                           check=True, timeout=30)
        for source, scope in ((self.user_source, "user"), (self.project_source, "project")):
            skill = self.skill(scope, source)
            skill.parent.mkdir(parents=True)
            (source / "plugin.json").write_text(json.dumps({
                "$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
                "name": "hermes-uap-native-" + scope, "version": "1.0.0",
                "description": "Disposable Hermes native " + scope + " smoke",
            }) + "\n", encoding="utf-8")
            skill.write_text("---\nname: native-smoke-" + scope
                             + "\ndescription: Validates scoped native Skills discovery.\n"
                             + "---\nNo model or external tool call is required.\n"
                             + "UAP_NATIVE_" + scope.upper() + "_V1\n", encoding="utf-8")
        (self.user_source / "mcp.json").write_text(json.dumps({
            "$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json",
            "mcpServers": {"uap-native-test-server": {
                "type": "streamable-http", "url": os.environ["HERMES_NATIVE_MCP_URL"],
            }},
        }) + "\n", encoding="utf-8")
        foreign = self.skill("foreign")
        foreign.parent.mkdir(parents=True, exist_ok=True)
        foreign.write_text("---\nname: native-smoke-foreign\n"
                           "description: Retained foreign Skill in a disposable profile.\n"
                           "---\nDo not change this foreign Skill.\n", encoding="utf-8")
        shutil.copyfile(foreign, self.root / "foreign-skill.md")
        self.config.write_text("model: offline-native-smoke\nmcp_servers:\n"
                               "  foreign:\n    command: echo\n    args: [foreign]\n",
                               encoding="utf-8")
        self.config.chmod(0o600)
        self.capture([*self.hermes, "--version"], "native-version.txt")
        self.native_home = self.resolved_home("native")
        require(self.native_home == self.profile.resolve(),
                "bare Hermes resolves another profile than the selected HERMES_HOME")

    def advance(self, source, scope):
        manifest = source / "plugin.json"
        data = json.loads(manifest.read_text(encoding="utf-8"))
        data["version"] = "1.1.0"
        manifest.write_text(json.dumps(data) + "\n", encoding="utf-8")
        skill = self.skill(scope, source)
        prefix = "UAP_NATIVE_" + scope.upper()
        skill.write_text(skill.read_text(encoding="utf-8").replace(prefix + "_V1", prefix + "_V2"),
                         encoding="utf-8")

    def same_bytes(self, expected, actual, message):
        require(expected.read_bytes() == actual.read_bytes(), message)

    def remove_skill(self, skill):
        # Refuse unexpected additional contents rather than deleting a whole tree.
        skill.unlink()
        skill.parent.rmdir()

    def user_lifecycle(self):
        self.run_uap("user-add", "add", self.user_source, "--scope", "user", "--no-save")
        self.observe("user-add", self.root, "UAP_NATIVE_USER_V1", 0)
        self.probe_mcp("user-add")
        listing = self.capture([*self.hermes, "skills", "list", "--source", "local", "--enabled-only"],
                               "user-cli-skills.txt")
        require("native-smoke-user" in listing.read_text(encoding="utf-8"),
                "native CLI did not list the installed User Skill")
        listing = self.capture([*self.hermes, "mcp", "list"], "user-cli-mcp.txt")
        require("uap-native-test-server" in listing.read_text(encoding="utf-8"),
                "native CLI did not list the installed User MCP")
        self.run_uap("user-repeat", "add", self.user_source, "--scope", "user", "--no-save")
        self.observe("user-repeat", self.root, "UAP_NATIVE_USER_V1", 0)
        self.probe_mcp("user-repeat")
        self.advance(self.user_source, "user")
        self.run_uap("user-update", "update", "hermes-uap-native-user", "--scope", "user")
        self.observe("user-update", self.root, "UAP_NATIVE_USER_V2", 0)
        self.probe_mcp("user-update")
        self.remove_skill(self.skill("user"))
        self.observe("user-missing", self.root, "absent", 0)
        self.run_uap("user-repair", "repair", "hermes-uap-native-user", "--scope", "user")
        self.observe("user-repair", self.root, "UAP_NATIVE_USER_V2", 0)
        self.probe_mcp("user-repair")

    def project_lifecycle(self):
        servers = self.hermes_json("project-before", "mcp_servers")
        require(not self.trusted_roots("project-before"), "disposable profile already trusts a project")
        for label, project in (("a", self.project_a), ("b", self.project_b)):
            output = self.run_uap("project-" + label + "-add", "add", self.project_source,
                                  "--scope", "project", "--project-root", project, "--no-save")
            self.require_trust_report(output, granted=True)
        output = self.run_uap("project-a-repeat", "add", self.project_source, "--scope", "project",
                              "--project-root", self.project_a, "--no-save")
        self.require_trust_report(output, granted=True)
        for label, project in (("a", self.project_a), ("b", self.project_b)):
            self.observe("project-" + label + "-add", project, "UAP_NATIVE_USER_V2", 1, True)
        b_snapshot = self.root / "project-b-skill.md"
        shutil.copyfile(self.project_skill(self.project_b), b_snapshot)
        self.advance(self.project_source, "project")
        self.run_uap("project-a-update", "update", "hermes-uap-native-project", "--scope",
                     "project", "--project-root", self.project_a)
        self.same_bytes(self.skill("project", self.project_source), self.project_skill(self.project_a),
                        "Project update did not install the current source revision")
        self.same_bytes(b_snapshot, self.project_skill(self.project_b), "Project A update changed B")
        self.remove_skill(self.project_skill(self.project_a))
        self.observe("project-a-missing", self.project_a, "UAP_NATIVE_USER_V2", 0, True)
        self.run_uap("project-a-repair", "repair", "hermes-uap-native-project", "--scope",
                     "project", "--project-root", self.project_a)
        self.observe("project-a-repair", self.project_a, "UAP_NATIVE_USER_V2", 1, True)
        self.same_bytes(self.skill("project", self.project_source), self.project_skill(self.project_a),
                        "Project repair did not restore the recorded current revision")
        self.same_bytes(b_snapshot, self.project_skill(self.project_b), "Project A repair changed B")
        self.refuse_project_mcp()
        self.run_uap("project-a-remove", "remove", "hermes-uap-native-project", "--scope",
                     "project", "--project-root", self.project_a)
        # Trust is the user's persistent preference; removal never revokes it.
        self.observe("project-a-remove", self.project_a, "UAP_NATIVE_USER_V2", 0, True)
        self.observe("project-b-retained", self.project_b, "UAP_NATIVE_USER_V2", 1, True)
        self.same_bytes(b_snapshot, self.project_skill(self.project_b), "Project A removal changed B")
        self.run_uap("project-b-remove", "remove", "hermes-uap-native-project", "--scope",
                     "project", "--project-root", self.project_b)
        self.observe("project-b-remove", self.project_b, "UAP_NATIVE_USER_V2", 0, True)
        self.unconfirmed_and_redirected_trust()
        require(self.trusted_roots("project-after") == {self.project_a.resolve(), self.project_b.resolve()},
                "Project operations trusted anything but the exact confirmed Git roots")
        require(self.hermes_json("project-after", "mcp_servers") == servers,
                "Project operations changed User MCP configuration")

    def unconfirmed_and_redirected_trust(self):
        project = self.project_c
        project.mkdir()
        subprocess.run(["git", "-C", str(project), "init", "-q"], check=True, timeout=30)
        before = self.config.read_bytes()
        self.run_uap("project-c-dry-run", "add", self.project_source, "--scope", "project",
                     "--project-root", project, "--no-save", "--dry-run")
        require(not (project / ".hermes").exists() and before == self.config.read_bytes()
                and project.resolve() not in self.trusted_roots("project-c-dry-run"),
                "a dry run installed files or changed Hermes trust")
        # A sticky active profile redirects a bare `hermes` away from the
        # selected HERMES_HOME. UAP must refuse instead of writing either file.
        work = self.profile / "profiles" / "work"
        work.mkdir(parents=True)
        (work / "config.yaml").write_text("model: offline-native-smoke\n", encoding="utf-8")
        marker = self.profile / "active_profile"
        marker.write_text("work\n", encoding="utf-8")
        try:
            require(self.resolved_home("project-c-sticky") == work.resolve(),
                    "sticky active profile did not redirect a bare Hermes")
            files = {path: path.read_bytes() for path in (self.config, work / "config.yaml")}
            output = self.run_uap("project-c-sticky-add", "add", self.project_source, "--scope", "project",
                                  "--project-root", project, "--no-save")
            self.require_trust_report(output, granted=False)
            require(all(path.read_bytes() == body for path, body in files.items()),
                    "UAP wrote Hermes trust despite a redirected active profile")
            require(project.resolve() not in self.trusted_roots("project-c-sticky"),
                    "redirected active profile trusts an unverified project")
        finally:
            marker.unlink()
        require(project.resolve() not in self.trusted_roots("project-c-selected"),
                "selected profile trusts a project after the grant fell back")
        self.run_uap("project-c-remove", "remove", "hermes-uap-native-project", "--scope",
                     "project", "--project-root", project)

    def refuse_project_mcp(self):
        before = self.config.read_bytes()
        output = self.capture([self.uap, "add", self.user_source, "--target", "hermes",
                               "--scope", "project", "--project-root", self.project_a,
                               "--no-save", "--yes", "--format", "json"],
                              "project-mcp-refusal.json", expected_failure=True)
        subprocess.run([sys.executable, HELPER, "refusal", output], check=True, timeout=30)
        require(before == self.config.read_bytes(), "Project MCP refusal changed User YAML")

    def remove_user(self):
        self.run_uap("user-remove", "remove", "hermes-uap-native-user", "--scope", "user")
        self.observe("user-remove", self.root, "absent", 0)
        require("uap-native-test-server" not in self.config.read_text(encoding="utf-8"),
                "removed User MCP remains in native YAML")
        listing = self.capture([*self.hermes, "mcp", "list"], "removed-mcp-list.txt")
        require("uap-native-test-server" not in listing.read_text(encoding="utf-8"),
                "removed User MCP remains natively listed")
        self.same_bytes(self.root / "foreign-skill.md", self.skill("foreign"),
                        "User lifecycle changed the foreign Skill")


def main():
    for key in ("HERMES_BIN", "UAP_BIN", "RUNNER_TEMP", "HERMES_HOME",
                "HERMES_NATIVE_MCP_URL", "HERMES_NATIVE_EVIDENCE"):
        require(bool(os.environ.get(key)), "Missing " + key + "; run through hermes-native.py")
    with tempfile.TemporaryDirectory(prefix="hermes-uap-native-", dir=os.environ["RUNNER_TEMP"]) as tmp:
        suite = Lifecycle(Path(tmp))
        suite.prepare()
        suite.user_lifecycle()
        suite.project_lifecycle()
        suite.remove_user()
    print("Hermes native User lifecycle, Project A/B isolation and trust boundary passed.", flush=True)


if __name__ == "__main__":
    main()
