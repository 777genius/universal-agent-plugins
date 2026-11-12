"""Actual UAP install/run lifecycle with pinned Cline, without a model or login."""
import argparse
import base64
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile

from native_client_research import run_probe
from project_native_research import Budget, acquire, materialize, verify_files
from project_native_collectors.cline import ACQUISITION
from grok_project_protocol_client import document, exercise, read_owned, require
from grok_project_protocol_e2e import digest, write, snapshot

NAME, SERVER = "uap-cline-session", "uap_session_probe"
SOURCE = Path(__file__).resolve().parent.parent


class SessionJourney:
    def __init__(self, root, cli, native, probe, evidence):
        self.root, self.cli, self.native, self.probe, self.evidence = root, cli, native, probe, evidence
        self.home, self.a, self.b, self.source = [root / x for x in ("home", "a", "b", "source")]
        for path in (self.home, self.a, self.b, self.source):
            path.mkdir(mode=0o700)
        self.nonce = os.urandom(16).hex()
        self.events = root / "mcp-events.jsonl"
        self.user_config = self.home / ".cline/data/settings/cline_mcp_settings.json"
        write(self.user_config, {"mcpServers": {"original-user": {
            "command": "/usr/bin/false", "disabled": True, "autoApprove": []}}})
        write(self.b / "foreign", "untouched")
        self.env = {"HOME": str(self.home), "PATH": str(native.parent) + ":/usr/bin:/bin",
                    "AGENTPLUGINS_HOME": str(root / "cache"), "CLINE_NO_AUTO_UPDATE": "1",
                    "GIT_CONFIG_NOSYSTEM": "1", "GIT_CONFIG_GLOBAL": "/dev/null",
                    "NO_COLOR": "1", "TERM": "dumb", "LANG": "C.UTF-8"}
        self.user_before = read_owned(self.user_config, root)
        self.b_before = snapshot(self.b)

    def check(self):
        require(read_owned(self.user_config, self.root) == self.user_before, "original User MCP changed")
        require(snapshot(self.b) == self.b_before, "Project B changed")

    def run(self, label, argv, expected=(0,), cwd=None):
        self.check()
        record, out, err = run_probe([str(x) for x in argv], cwd or self.a, self.env,
                                    self.root, label, timeout=120, limit=1 << 20, memory_only=True)
        self.evidence["commands"].append(dict(record, stdout_base64=base64.b64encode(out).decode(),
                                               stderr_base64=base64.b64encode(err).decode()))
        self.check()
        require(record["error"] is None and type(record["exit"]) is int
                and record["exit"] in expected, "command failed: " + label)
        return out

    def uap(self, label, args, expected=(0,)):
        return self.run(label, [self.cli, *args, "--target", "cline", "--scope", "project",
                               "--project-root", self.a], expected)

    def install(self, label, verb, selector):
        args = [verb, selector, "--yes", "--format", "json"]
        if verb == "add":
            args.append("--no-save")
        require(document(self.uap(label, args)).get("result") == "success", "UAP action was not successful")

    def author(self, revision):
        write(self.source / "plugin.json", {"$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
                                           "name": NAME, "version": "1.0." + revision})
        binary = self.source / "bin/probe"
        binary.parent.mkdir(exist_ok=True)
        if not binary.exists():
            shutil.copyfile(self.probe, binary)
            binary.chmod(0o700)
        write(self.source / "skills/check/SKILL.md", "---\nname: check\ndescription: Local MCP session test\n---\nNo model.\n")
        write(self.source / "mcp.json", {"$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json",
            "mcpServers": {SERVER: {"type": "stdio", "command": "./bin/probe", "args": [], "env": {
                "UAP_TEST_ROOT": str(self.root), "UAP_TEST_EVENTS": str(self.events),
                "UAP_TEST_NONCE": self.nonce, "UAP_TEST_REVISION": revision}}}})

    def owned(self):
        state = document(read_owned(self.a / ".uap/state-v2.json", self.root))
        require(state.get("schema_version") == 5 and len(state.get("installations", [])) == 1, "state differs")
        installation = state["installations"][0]
        require(installation["declared_name"] == NAME and len(installation["clients"]) == 1, "owned selection differs")
        binding = next(iter(installation["clients"].values()))
        require(binding["client_id"] == "cline" and binding["scope"] == "project"
                and binding["activation"] == "prepared", "startup changed authority or promoted activation")
        active = Path(binding["target_locator"])
        expected = self.a / ".uap/managed/clients/cline" / binding["physical_artifact_id"]
        require(active == expected and active.resolve() == active, "foreign managed package")
        return installation, binding, active

    def session(self, stage, revision, create=False):
        installation, binding, active = self.owned()
        state = read_owned(self.a / ".uap/state-v2.json", self.root)
        body = self.uap(stage + "-native-source", ["run", "--inspect", "--confirm-client-start"])
        rows = document(body)
        config = self.a / ".uap/clients/cline/mcp.json"
        require(isinstance(rows, list) and len(rows) == 1 and rows[0].get("name") == SERVER
                and rows[0].get("path") == str(config) and rows[0].get("transportType") == "stdio"
                and rows[0].get("disabled") is False, "native Cline source selection differs")
        require(read_owned(self.a / ".uap/state-v2.json", self.root) == state, "session changed UAP state")
        selected = document(read_owned(config, self.root))["mcpServers"][SERVER]
        command = Path(selected["command"])
        require(command.is_absolute() and command.is_relative_to(active) and command.resolve() == command,
                "installed probe command is not owned")
        require(selected["env"]["UAP_TEST_NONCE"] == self.nonce, "installed nonce differs")
        direct = exercise(selected, self.env, self.nonce, revision, "kept-" + self.nonce, create)
        self.evidence["stages"].append({"stage": stage, "binding": binding, "native_inventory": rows,
                                         "direct_installed_mcp": direct})
        return installation, binding, active

    def execute(self):
        self.run("project-init", ["/usr/bin/git", "-c", "core.hooksPath=/dev/null", "init", "--quiet"])
        require(self.run("native-version", [self.native, "--version"]).strip() == b"3.0.73", "pinned version differs")
        self.author("0")
        self.install("install", "add", self.source)
        state_path = self.a / ".uap/state-v2.json"
        state = read_owned(state_path, self.root)
        self.uap("declined-start", ["run", "--inspect"], expected=(1,))
        self.uap("install-yes-is-not-start-consent", ["run", "--inspect", "--yes"], expected=(1,))
        # Read-only diagnostics before the session planner. Keep the original
        # refusal intact; never fake a Detected client or an owned binding.
        self.uap("preflight-list", ["list", "--format", "json"])
        self.uap("preflight-doctor", ["doctor", "--format", "json"], expected=(0, 1, 2))
        self.uap("preview", ["run", "--dry-run", "--format", "json"])
        require(not self.events.exists() and read_owned(state_path, self.root) == state, "declined start had an effect")
        self.install("repeat-install", "add", self.source)
        require(read_owned(state_path, self.root) == state, "repeat install changed state")
        first, binding, active = self.session("installed", "0", create=True)
        self.author("1")
        self.install("update", "update", self.source)
        updated, current, _ = self.session("updated", "1")
        require(current["client_binding_id"] == binding["client_binding_id"]
                and updated["source"]["tree_digest"] != first["source"]["tree_digest"], "update identity differs")
        shutil.rmtree(active)
        self.uap("missing-package-refusal", ["run", "--inspect", "--confirm-client-start"], expected=(1,))
        self.install("repair", "repair", updated["installation_id"])
        repaired, current, _ = self.session("repaired", "1")
        self.install("remove", "remove", NAME)
        self.uap("removed-package-refusal", ["run", "--inspect", "--confirm-client-start"], expected=(1,))
        require(not active.exists(), "owned package survives removal")
        state = document(read_owned(state_path, self.root))
        retained = state["installations"][0]
        require(retained["clients"] == {} and retained["data_retained"] is True
                and retained["data_receipts"] == repaired["data_receipts"], "retained data authority differs")
        data = Path(repaired["data_receipts"][current["data_receipt_id"]]["locator"])
        require(read_owned(data / "native-marker.txt", self.root).decode() == "kept-" + self.nonce, "user data lost")
        original = document(self.run("ordinary-native-user-source", [self.native, "--auto-approve", "false", "config", "mcp", "--json"], cwd=self.b))
        require(len(original) == 1 and original[0]["name"] == "original-user"
                and original[0]["path"] == str(self.user_config), "ordinary startup did not retain User source")
        self.check()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("installer", "probe", "output"):
        parser.add_argument("--" + name, required=True, type=Path)
    args = parser.parse_args()
    require(os.environ.get("GITHUB_ACTIONS") == "true" and os.environ.get("RUNNER_ENVIRONMENT") == "github-hosted",
            "disposable hosted runner required")
    sha = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=SOURCE, timeout=10).decode().strip()
    require(sha == os.environ.get("EXPECTED_COMMIT"), "wrong source revision")
    args.output.mkdir(mode=0o700, parents=True, exist_ok=False)
    result = {"source_commit": sha, "passed": False, "commands": [], "stages": [],
              "model_invoked": False, "vendor_authentication_used": False,
              "native_cline_tools_call": False, "installer_sha256": digest(args.installer)}
    try:
        budget = Budget(900)
        receipt, contents = acquire(ACQUISITION, budget)
        result["acquisition"] = receipt
        with tempfile.TemporaryDirectory(prefix="uap-cline-session-", dir=os.environ["RUNNER_TEMP"]) as tmp:
            root = Path(tmp).resolve()
            native, identities = materialize(root / "vendor", ACQUISITION, receipt, contents)
            verify_files(identities, budget)
            journey = SessionJourney(root, args.installer, native, args.probe, result)
            try:
                journey.execute()
                verify_files(identities, budget)
            finally:
                if journey.events.exists():
                    raw = read_owned(journey.events, root)
                    result["events_base64"] = base64.b64encode(raw).decode()
                    result["events_sha256"] = hashlib.sha256(raw).hexdigest()
        result["passed"] = True
    except Exception as error:
        result["error"] = type(error).__name__ + ": " + str(error)
        raise
    finally:
        write(args.output / "evidence.json", result)
        print(json.dumps({"passed": result["passed"], "source_commit": sha}))


if __name__ == "__main__":
    main()
