"""Actual UAP install/run lifecycle with pinned Cline, without a model or login."""
import argparse
import base64
import copy
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import time
import threading
import stat
import platform

import native_client_project_artifacts as artifacts

from project_native_research import Budget, acquire, materialize
from project_native_collectors.cline import platform_acquisition
from grok_project_protocol_client import document, require
from grok_project_protocol_e2e import digest, write

NAME, SERVER = "uap-cline-session", "uap_session_probe"
SOURCE = Path(__file__).resolve().parent.parent


def read_owned(path, root, limit=1 << 20):
    path, root = Path(path), Path(root)
    require(path.is_absolute() and path.is_relative_to(root) and path != root and path.resolve() == path
            and root.resolve() == root, "aliased or foreign fixture path")
    before = path.lstat()
    require(stat.S_ISREG(before.st_mode) and before.st_nlink == 1 and before.st_size <= limit,
            "ordinary bounded fixture file required")
    if os.name != "nt":
        require(before.st_uid == os.getuid(), "foreign fixture owner")
    descriptor = artifacts.open_member(path)
    try:
        held = os.fstat(descriptor)
        if artifacts.WINDOWS:
            left, right = artifacts.windows_identity(before), artifacts.windows_identity(held)
            require(stat.S_ISREG(held.st_mode) and left[:2] == right[:2] and left[3:] == right[3:],
                    "Windows file authority differs")
        else:
            require(artifacts.same_opened(before, held), "fixture changed at open")
        chunks, count = [], 0
        while chunk := os.read(descriptor, min(65536, limit + 1 - count)):
            chunks.append(chunk); count += len(chunk)
            require(count <= limit, "fixture grew")
        require(artifacts.identity(os.fstat(descriptor)) == artifacts.identity(held), "held fixture changed")
    finally:
        os.close(descriptor)
    require(artifacts.identity(path.lstat()) == artifacts.identity(before)
            and count == before.st_size and path.resolve() == path, "fixture namespace changed")
    return b"".join(chunks)


def verify_files(identities, budget):
    for path, expected in identities.items():
        budget.check()
        require(hashlib.sha256(read_owned(path, path.parent, 256 << 20)).hexdigest() == expected, "runtime hash differs")


def file_identity(path):
    return artifacts.identity(path.lstat())


def same_record(left, right):
    return json.dumps(left, sort_keys=True, separators=(",", ":"), allow_nan=False) == \
        json.dumps(right, sort_keys=True, separators=(",", ":"), allow_nan=False)


def snapshot(root):
    require(root.resolve() == root and root.is_dir(), "foreign control root changed")
    with os.scandir(root) as entries:
        names = []
        for entry in entries:
            names.append(entry.name)
            require(len(names) <= 1, "foreign control acquired a new entry")
    require(names == ["foreign"], "foreign control membership changed")
    return file_identity(root), file_identity(root / "foreign"), read_owned(root / "foreign", root)


def source_identity():
    git = Path(shutil.which("git")).resolve()
    commit = subprocess.check_output([str(git), "rev-parse", "HEAD"], cwd=SOURCE, timeout=10).decode().strip()
    tree = subprocess.check_output([str(git), "rev-parse", "HEAD^{tree}"], cwd=SOURCE, timeout=10).decode().strip()
    dirty = subprocess.check_output([str(git), "status", "--porcelain", "--untracked-files=all"], cwd=SOURCE, timeout=10)
    require(commit == os.environ.get("EXPECTED_COMMIT") and not dirty, "source is dirty or differs from requested commit")
    return commit, tree


def build_bridge(installer, probe, destination):
    """Compile the fixed test driver with exact source and runtime pins."""
    git, go = [Path(shutil.which(name)).resolve() for name in ("git", "go")]
    commit, tree = source_identity()
    pins = {"sourceCommit": commit, "sourceTree": tree,
            "nativeSHA": platform_acquisition(platform.system(), platform.machine())["binary_sha256"],
            "installerSHA": digest(installer), "probeSHA": digest(probe),
            "fixtureSHA": digest(SOURCE / "repotests/testdata/agentplugins_native_probe/main.go"),
            "gitBinary": str(git), "gitSHA": digest(git), "goBinary": str(go), "goSHA": digest(go)}
    require(all("'" not in value and "\n" not in value for value in pins.values()), "compiler pin quoting refused")
    flags = " ".join("-X 'main." + key + "=" + value + "'" for key, value in pins.items())
    subprocess.run([str(go), "-C", "install/integrationctl/agentplugins", "build", "-trimpath", "-ldflags", flags,
                    "-o", str(destination), "./research/clinesession"],
                   cwd=SOURCE, check=True, timeout=180)


def invoke_bridge(helper, root, action, budget):
    remaining = min(120, budget.check() - 15)
    require(remaining > 5, "journey cleanup reserve exhausted")
    request = {"schema_version": 1, "root": str(root), "action": action,
               "source_commit": os.environ["EXPECTED_COMMIT"],
               "source_tree": os.environ["CLINE_SOURCE_TREE"],
               "deadline_unix_ns": time.time_ns() + int(remaining * 1_000_000_000)}
    child = subprocess.Popen([str(helper)], stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                             stderr=subprocess.PIPE)
    captures, failures = {}, []
    def drain(label, stream, bound):
        try:
            captures[label] = stream.read(bound + 1)
            if len(captures[label]) > bound:
                failures.append("stream-bound"); child.stdin.close()
        except (OSError, ValueError):
            failures.append("stream-refused")
    threads = [threading.Thread(target=drain, args=(label, stream, bound), daemon=True)
               for label, stream, bound in (("out", child.stdout, 2 << 20), ("err", child.stderr, 32768))]
    for thread in threads: thread.start()
    try:
        child.stdin.write(json.dumps(request).encode() + b"\n"); child.stdin.flush()
        child.wait(timeout=remaining + 5)
    finally:
        child.stdin.close()
        if child.poll() is None:
            try: child.wait(timeout=5)
            except subprocess.TimeoutExpired: child.kill(); child.wait(timeout=5)
        cleanup_deadline = min(budget.deadline, time.monotonic() + 5)
        for thread in threads: thread.join(timeout=max(0, cleanup_deadline - time.monotonic()))
    require(child.returncode == 0 and not failures and not any(t.is_alive() for t in threads)
            and captures.get("err") == b"", "bridge settlement refused")
    body = captures["out"]
    require(body.endswith(b"\n") and body.count(b"\n") == 1, "bridge result frame refused")
    require(len(body) <= 2 << 20, "bridge result size")
    result = document(body)
    require(set(result) == {"schema_version", "source_commit", "source_tree", "action", "cleanup_complete", "exit", "stdout", "stderr", "runtime_sha256"}
            and type(result["schema_version"]) is int and result["schema_version"] == 1
            and result["source_commit"] == request["source_commit"] and result["source_tree"] == request["source_tree"]
            and result["action"] == action and result["cleanup_complete"] is True
            and type(result["exit"]) is int, "bridge result identity refused")
    out, err = [base64.b64decode(result[key] or "", validate=True) for key in ("stdout", "stderr")]
    require(len(out) <= 1 << 20 and len(err) <= 32768, "decoded stream bound")
    return result, out, err


def cline_owned_stdio(entry):
    """Unwrap the genuine Cline native transport without accepting a flat or foreign route."""
    require(isinstance(entry, dict) and set(entry) <= {"transport", "disabled", "autoApprove"},
            "Cline owned MCP entry contains unsupported fields")
    if "disabled" in entry:
        require(entry["disabled"] is False, "disabled Cline server must not execute")
    if "autoApprove" in entry:
        require(entry["autoApprove"] == [], "fixture must not grant automatic tool approval")
    transport = entry.get("transport")
    require(isinstance(transport, dict) and set(transport) <=
            {"type", "command", "args", "env", "cwd"} and transport.get("type") == "stdio",
            "Cline native transport is not the selected stdio source")
    require(isinstance(transport.get("command"), str) and transport["command"],
            "Cline native stdio command missing")
    require(isinstance(transport.get("cwd"), str) and transport["cwd"],
            "Cline native stdio cwd missing")
    require(isinstance(transport.get("args", []), list) and
            all(isinstance(value, str) for value in transport.get("args", [])),
            "Cline native stdio arguments invalid")
    env = transport.get("env")
    require(isinstance(env, dict) and all(isinstance(key, str) and isinstance(value, str)
                                           for key, value in env.items()),
            "Cline native stdio environment invalid")
    return transport


def validate_events(result, raw):
    direct = [stage["direct_installed_mcp"] for stage in result["stages"]]
    direct += [result["cold_locked_restore"][key] for key in ("direct_installed_mcp", "repeat_mcp")]
    # Python considers True == 1, so a forged boolean PID could otherwise
    # correlate with a real process in the original MCP event stream.
    require(len(direct) == 5 and all(type(item) is dict and type(item.get("facts")) is dict
            and type(item["facts"].get("pid")) is int and item["facts"]["pid"] > 0
            and type(item["facts"].get("session")) is str and bool(item["facts"]["session"])
            and type(item["facts"].get("nonce")) is str and bool(item["facts"]["nonce"])
            for item in direct), "five typed native session identities required")
    expected = {(item["facts"]["pid"], item["facts"]["session"]): item["facts"] for item in direct}
    require(len(direct) == len(expected) == 5, "five independent fixture sessions required")
    rows = [document(line) for line in raw.splitlines()]
    require(len(rows) == 15, "unexpected or missing probe events")
    grouped = {key: [] for key in expected}
    for row in rows:
        key = row.get("pid"), row.get("session")
        require(type(key[0]) is int and key[0] > 0 and key in expected
                and row.get("nonce") == expected[key]["nonce"], "unowned event session")
        if row.get("method") == "tools/call":
            facts = row.get("facts")
            require(type(facts) is dict and type(facts.get("pid")) is int and facts["pid"] > 0
                    and type(facts.get("session")) is str and bool(facts["session"])
                    and type(facts.get("nonce")) is str and bool(facts["nonce"]),
                    "native call event identity types differ")
        grouped[key].append(row)
    for key, entries in grouped.items():
        require([entry["method"] for entry in entries] == ["initialize", "tools/list", "tools/call"]
                and entries[0]["facts"] is None and entries[1]["facts"] is None
                and entries[2]["facts"] == expected[key], "probe event stream differs from protocol")


class SessionJourney:
    def __init__(self, root, cli, native, probe, helper, evidence, budget, identities):
        self.helper, self.budget, self.settled = helper, budget, True
        self.identities = identities
        self.runtime_before = {path: file_identity(path) for path in identities}
        self.root, self.cli, self.native, self.probe, self.evidence = root, cli, native, probe, evidence
        self.home, self.a, self.b, self.source = [root / x for x in ("home", "a", "b", "source")]
        for path in (self.home, self.a, self.b, self.source):
            path.mkdir(mode=0o700)
        self.nonce = os.urandom(16).hex()
        write(root / "nonce.txt", self.nonce)
        self.events = root / "mcp-events.jsonl"
        self.user_config = self.home / ".cline/data/settings/cline_mcp_settings.json"
        write(self.user_config, {"mcpServers": {"original-user": {
            "command": str(root / "never-run" ), "disabled": True, "autoApprove": []}}})
        write(self.b / "foreign", "untouched")
        self.user_before = read_owned(self.user_config, root)
        self.user_identity = file_identity(self.user_config)
        self.b_before = snapshot(self.b)
        self.warm_origin = self.warm_pin = self.warm_binding = self.marker_before = self.cold_marker = None
        self.data_before = {}

    def data_directory_facts(self, path):
        require(path.is_absolute() and path.is_relative_to(self.root) and path != self.root
                and path.resolve() == path, "original data directory namespace changed")
        info = path.lstat()
        require(stat.S_ISDIR(info.st_mode) and info.st_dev > 0 and info.st_ino > 0,
                "ordinary original data directory required")
        if os.name != "nt":
            require(info.st_uid == os.getuid(), "foreign original data owner")
        facts = info.st_dev, info.st_ino, info.st_mode, info.st_uid, info.st_gid
        if artifacts.WINDOWS:
            artifacts.windows_identity(info)
            facts += (info.st_birthtime_ns, info.st_file_attributes, info.st_reparse_tag)
        return facts

    def remember_data(self, project, installation, binding):
        physical = binding["physical_artifact_id"]
        require(isinstance(physical, str) and physical not in ("", ".", "..") and Path(physical).name == physical,
                "original data physical artifact is not a basename")
        data = Path(installation["data_receipts"][binding["data_receipt_id"]]["locator"])
        require(project in (self.a, self.root / "cold") and data == project / ".uap/plugin-data" / physical
                and data not in self.data_before and len(self.data_before) < 2, "original data capture differs or repeats")
        self.data_before[data] = self.data_directory_facts(data)

    def check_data(self):
        for path, original in self.data_before.items():
            require(self.data_directory_facts(path) == original, "original data directory changed")

    def check(self):
        self.check_data()
        verify_files(self.identities, self.budget)
        require(all(file_identity(path) == before for path, before in self.runtime_before.items()), "runtime replaced")
        require(file_identity(self.user_config) == self.user_identity
                and read_owned(self.user_config, self.root) == self.user_before
                and file_identity(self.user_config) == self.user_identity, "original User MCP changed")
        require(snapshot(self.b) == self.b_before, "Project B changed")
        if self.marker_before is not None:
            require(self.marker_facts() == self.marker_before, "original warm data marker changed")
        if self.cold_marker is not None:
            path, original = self.cold_marker
            require(self.marker_facts(path) == original, "original cold data marker changed")

    def run(self, action, expected=(0,)):
        self.check()
        self.settled = False
        record, out, err = invoke_bridge(self.helper, self.root, action, self.budget)
        self.settled = True
        hashes = record["runtime_sha256"]
        require(set(hashes) == {"native", "installer", "probe", "git", "go", "fixture_source"}
                and all(isinstance(value, str) and len(value) == 64 for value in hashes.values())
                and self.evidence.setdefault("runtime_sha256", hashes) == hashes, "runtime receipt differs")
        self.evidence["commands"].append(dict(record, stdout_base64=base64.b64encode(out).decode(),
                                               stderr_base64=base64.b64encode(err).decode()))
        self.check()
        require(type(record["exit"]) is int
                and record["exit"] in expected, "command failed: " + action)
        return out

    def install(self, action):
        require(document(self.run(action)).get("result") == "success", "UAP action was not successful")

    def author(self, revision):
        write(self.source / "plugin.json", {"$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
                                           "name": NAME, "version": "1.0." + revision})
        binary = self.source / ("bin/probe.exe" if os.name == "nt" else "bin/probe")
        binary.parent.mkdir(exist_ok=True)
        if not binary.exists():
            shutil.copyfile(self.probe, binary)
            binary.chmod(0o700)
        write(self.source / "skills/check/SKILL.md", "---\nname: check\ndescription: Local MCP session test\n---\nNo model.\n")
        write(self.source / "mcp.json", {"$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json",
            "mcpServers": {SERVER: {"type": "stdio", "command": "./bin/probe.exe" if os.name == "nt" else "./bin/probe", "args": [], "env": {
                "UAP_TEST_ROOT": str(self.root), "UAP_TEST_EVENTS": str(self.events),
                "UAP_TEST_NONCE": self.nonce, "UAP_TEST_REVISION": revision}}}})

    def owned(self, project=None, name=NAME):
        project = project or self.a
        state = document(read_owned(project / ".uap/state-v2.json", self.root))
        require(state.get("schema_version") == 5 and len(state.get("installations", [])) == 1, "state differs")
        installation = state["installations"][0]
        require(installation["declared_name"] == name and len(installation["clients"]) == 1, "owned selection differs")
        binding = next(iter(installation["clients"].values()))
        require(binding["client_id"] == "cline" and binding["scope"] == "project"
                and binding["activation"] == "prepared", "startup changed authority or promoted activation")
        active = Path(binding["target_locator"])
        expected = project / ".uap/managed/clients/cline" / binding["physical_artifact_id"]
        require(active == expected and active.resolve() == active, "foreign managed package")
        return installation, binding, active

    def session(self, stage, revision, create=False):
        installation, binding, active = self.owned()
        identity, pin = self.warm_facts(installation, binding, revision)
        require(same_record(identity, self.warm_origin), "original warm installation authority changed")
        require(stage in ("installed", "updated", "repaired"), "unknown warm phase")
        expected_pin = copy.deepcopy(self.warm_pin)
        if stage == "updated":
            require(pin["source"]["tree_digest"] != self.warm_pin["source"]["tree_digest"]
                    and pin["package"]["manifest_digest"] != self.warm_pin["package"]["manifest_digest"],
                    "warm update did not advance its exact revision")
            for section, keys in (("source", ("tree_digest",)), ("package", ("version", "manifest_digest")),
                                  ("binding_revision", ("version", "tree_digest", "manifest_digest"))):
                for key in keys:
                    expected_pin[section][key] = pin[section][key]
        require(same_record(pin, expected_pin), "warm operation changed the accepted source/package authority")
        self.binding_continuity(binding, pin, stage == "updated")
        state = read_owned(self.a / ".uap/state-v2.json", self.root)
        body = self.run("session-inspect")
        rows = document(body)
        config = self.a / ".uap/clients/cline/mcp.json"
        require(isinstance(rows, list) and len(rows) == 1 and rows[0].get("name") == SERVER
                and rows[0].get("path") == str(config) and rows[0].get("transportType") == "stdio"
                and rows[0].get("disabled") is False, "native Cline source selection differs")
        require(read_owned(self.a / ".uap/state-v2.json", self.root) == state, "session changed UAP state")
        selected = cline_owned_stdio(document(read_owned(config, self.root))["mcpServers"][SERVER])
        command = Path(selected["command"])
        require(command.is_absolute() and command.is_relative_to(active) and command.resolve() == command,
                "installed probe command is not owned")
        require(selected["env"]["UAP_TEST_NONCE"] == self.nonce, "installed nonce differs")
        # This fixture omits source cwd, so the normative managed default is
        # the installed plugin root. Exercise exactly that projected directory.
        native_cwd = Path(selected["cwd"])
        require(native_cwd == active and native_cwd.resolve() == active and native_cwd.is_dir(),
                "native Cline MCP cwd differs from the owned plugin root")
        direct = self.direct("direct-create" if create else "direct-session", revision)
        if create:
            require(self.marker_before is None, "warm marker already acknowledged")
            self.marker_before = self.marker_facts()
            require(self.marker_before[1] == ("kept-" + self.nonce).encode(), "initial warm marker differs")
        require(self.marker_facts() == self.marker_before, "original warm data marker changed")
        self.warm_pin = copy.deepcopy(pin)
        self.warm_binding = copy.deepcopy(binding)
        self.evidence["stages"].append({"stage": stage, "binding": binding, "native_inventory": rows,
                                         "package_revision": pin, "direct_installed_mcp": direct})
        return installation, binding, active

    def binding_continuity(self, binding, pin, updated):
        expected, current = copy.deepcopy(self.warm_binding), copy.deepcopy(binding)
        previous_receipts, receipts = expected.pop("receipts"), current.pop("receipts")
        require(isinstance(previous_receipts, list) and isinstance(receipts, list)
                and len(receipts) >= len(previous_receipts)
                and same_record(receipts[:len(previous_receipts)], previous_receipts)
                and all(isinstance(row, dict) and row.get("phase") == "committed" for row in receipts),
                "original committed receipt history changed")
        expected.pop("updated_at", None); current.pop("updated_at", None)
        if updated:
            expected["package_revision"] = pin["binding_revision"]
            before, after = expected.get("native_objects"), current.get("native_objects")
            require(isinstance(before, list) and isinstance(after, list) and len(before) == len(after),
                    "warm native object membership changed")
            for original, observed in zip(before, after):
                require(isinstance(original, dict) and isinstance(observed, dict)
                        and isinstance(observed.get("managed_digest"), str), "warm native object digest missing")
                original["managed_digest"] = observed["managed_digest"]
        require(same_record(current, expected), "original full warm binding authority changed")

    def warm_facts(self, installation, binding, revision):
        self.check_data()
        source, package, pin = installation["source"], installation["package"], binding["package_revision"]
        manifest = read_owned(self.source / "plugin.json", self.root)
        authored = document(manifest)
        require(authored.get("name") == NAME and authored.get("version") == "1.0." + revision
                and source.get("requested_source") == source.get("canonical_source") == str(self.source)
                and bool(source.get("source_binding_id")) and not source.get("repository") and not source.get("package_subpath")
                and package.get("declared_name") == NAME and package.get("version") == pin.get("version") == "1.0." + revision
                and package.get("manifest_digest") == pin.get("manifest_digest") == "sha256:" + hashlib.sha256(manifest).hexdigest()
                and bool(source.get("tree_digest")) and source["tree_digest"] == pin.get("tree_digest")
                and source.get("resolved_revision", "") == pin.get("resolved_revision", ""), "warm source/revision join differs")
        physical, receipt_id = binding["physical_artifact_id"], binding["data_receipt_id"]
        require(isinstance(physical, str) and physical not in ("", ".", "..") and Path(physical).name == physical,
                "warm physical artifact is not a basename")
        data = self.a / ".uap/plugin-data" / physical
        receipts = installation["data_receipts"]
        require(data.is_relative_to(self.a / ".uap/plugin-data") and data.resolve() == data
                and bool(installation.get("installation_id")) and bool(binding.get("client_binding_id"))
                and binding.get("native_profile_root") == str(self.a) and binding.get("verification") == "installation_verified"
                and binding.get("native_activation_attempt", "") == "" and binding.get("pending_native_intent") is None
                and isinstance(receipt_id, str) and set(receipts) == {receipt_id}, "warm data selection differs")
        receipt = receipts[receipt_id]
        require(receipt.get("data_receipt_id") == receipt_id and receipt.get("physical_backend_id") == physical
                and receipt.get("scope") == "project" and receipt.get("state") == "owned"
                and receipt.get("locator") == str(data) and bool(receipt.get("ownership_digest"))
                and same_record(document(read_owned(data / ".agentplugins-data-owner.json", self.root)), receipt),
                "warm original data receipt differs")
        self.check_data()
        identity = {key: installation[key] for key in ("installation_id", "declared_name", "data_receipts")}
        identity["source"] = {key: value for key, value in source.items() if key not in ("resolved_revision", "tree_digest")}
        identity["binding"] = {key: binding.get(key) for key in ("client_binding_id", "client_id", "scope", "target_locator",
            "physical_artifact_id", "native_profile_root", "data_receipt_id", "authentication", "policy")}
        return copy.deepcopy(identity), copy.deepcopy({"source": source, "package": package, "binding_revision": pin})

    def marker_facts(self, path=None):
        if path is None:
            original = self.warm_origin
            receipt = original["data_receipts"][original["binding"]["data_receipt_id"]]
            path = Path(receipt["locator"]) / "native-marker.txt"
        before = file_identity(path)
        body = read_owned(path, self.root, 256)
        require(file_identity(path) == before, "original marker changed during read")
        return before, body

    def verify_retained(self, state):
        require(state.get("schema_version") == 5 and len(state["installations"]) == 1, "retained installation count differs")
        retained = state["installations"][0]
        require(retained["clients"] == {} and retained["data_retained"] is True
                and all(same_record(retained[key], self.warm_origin[key]) for key in ("installation_id", "declared_name", "data_receipts"))
                and same_record(retained["source"], self.warm_pin["source"])
                and same_record(retained["package"], self.warm_pin["package"]),
                "retained original installation/data authority differs")
        require(self.marker_facts() == self.marker_before, "original warm data marker lost")

    def direct(self, action, revision):
        cold = action.startswith("cold-")
        project, server = (self.root / "cold", "uap_public_probe") if cold else (self.a, SERVER)
        config = project / ".uap/clients/cline/mcp.json"
        selected = cline_owned_stdio(document(read_owned(config, self.root))["mcpServers"][server])
        helper, active = Path(selected["command"]), Path(selected["cwd"])
        probe = active / ("main.go" if cold else "bin/probe.exe" if os.name == "nt" else "bin/probe")
        limits = {config: 1 << 20, project / ".uap/state-v2.json": 1 << 20,
                  helper: 256 << 20, probe: 256 << 20}
        def authority():
            return {path: (file_identity(path), hashlib.sha256(read_owned(path, self.root, limit)).hexdigest())
                    for path, limit in limits.items()}
        members = authority()
        expected_probe = self.evidence["runtime_sha256"]["fixture_source"] if cold else digest(self.probe)
        require(members[helper][1] == self.evidence["installer_sha256"] and members[probe][1] == expected_probe,
                "selected direct fixture bytes differ")
        before = read_owned(self.events, self.root, 2 << 20) if self.events.exists() else b""
        direct = document(self.run(action))
        require(authority() == members, "direct fixture members changed")
        require(direct.get("caller") == "direct-installed-stdio-client" and direct.get("native_cline_tools_call") is False
                and direct["facts"]["revision"] == revision and direct["facts"]["nonce"] == self.nonce
                and direct["facts"]["operation"] == ("write" if action.endswith("-create") else "read"),
                "direct protocol identity differs")
        after = read_owned(self.events, self.root, 2 << 20)
        require(after.startswith(before) and after.endswith(b"\n"), "original MCP events changed")
        rows = [document(line) for line in after[len(before):].splitlines()]
        facts = direct["facts"]
        require([row.get("method") for row in rows] == ["initialize", "tools/list", "tools/call"]
                and type(facts.get("pid")) is int and facts["pid"] > 0
                and isinstance(facts.get("session"), str) and bool(facts["session"])
                and all((row.get("pid"), row.get("session"), row.get("nonce")) ==
                        (facts["pid"], facts["session"], self.nonce) for row in rows)
                and rows[0]["facts"] is None and rows[1]["facts"] is None and rows[2]["facts"] == facts,
                "MCP event lifecycle differs")
        return direct

    def execute(self):
        self.run("project-init")
        require(self.run("native-version").strip() == b"3.0.73", "pinned version differs")
        self.author("0")
        self.install("install")
        first, binding, _ = self.owned()
        self.remember_data(self.a, first, binding)
        self.warm_origin, self.warm_pin = self.warm_facts(first, binding, "0")
        self.warm_binding = copy.deepcopy(binding)
        self.evidence["warm_authority"] = copy.deepcopy(self.warm_origin)
        state_path = self.a / ".uap/state-v2.json"
        state = read_owned(state_path, self.root)
        self.run("declined-start", expected=(1,))
        self.run("install-yes-is-not-start-consent", expected=(1,))
        # Read-only diagnostics before the session planner. Keep the original
        # refusal intact; never fake a Detected client or an owned binding.
        self.run("preflight-list")
        self.run("preflight-doctor", expected=(0, 1, 2))
        self.run("preview")
        require(not self.events.exists() and read_owned(state_path, self.root) == state, "declined start had an effect")
        self.install("repeat-install")
        require(read_owned(state_path, self.root) == state, "repeat install changed state")
        first, binding, active = self.session("installed", "0", create=True)
        self.author("1")
        self.install("update")
        updated, current, _ = self.session("updated", "1")
        require(current["client_binding_id"] == binding["client_binding_id"]
                and updated["source"]["tree_digest"] != first["source"]["tree_digest"], "update identity differs")
        quarantine = self.root / "missing-owned-package"
        active.rename(quarantine)
        self.run("missing-package-refusal", expected=(1,))
        self.install("repair")
        repaired, current, _ = self.session("repaired", "1")
        self.install("remove")
        self.run("removed-package-refusal", expected=(1,))
        require(not active.exists(), "owned package survives removal")
        self.verify_retained(document(read_owned(state_path, self.root)))
        original = document(self.run("ordinary-native-user-source"))
        require(len(original) == 1 and original[0]["name"] == "original-user"
                and original[0]["path"] == str(self.user_config), "ordinary startup did not retain User source")
        self.cold_restore()
        self.check()

    def cold_restore(self):
        for name in ("saved", "cold", "cold-home"):
            (self.root / name).mkdir(mode=0o700)
        self.run("saved-init")
        self.install("saved-add")
        agreement = {name: read_owned(self.root / "saved" / name, self.root) for name in ("uap.toml", "uap.lock")}
        locked = document(agreement["uap.lock"])
        require(locked.get("lock_version") == 1 and len(locked["packages"]) == 1, "exact cold lock required")
        pin = locked["packages"][0]
        git = pin["source"]["git"]
        require(git["commit"] == os.environ["EXPECTED_COMMIT"]
                and git["repository"] == "777genius/universal-agent-plugins"
                and git["path"] == "repotests/testdata/agentplugins_native_probe"
                and pin["targets"] == ["cline"], "cold source agreement differs")
        require(not list((self.root / "cold").iterdir()) and not list((self.root / "cold-home").iterdir())
                and not (self.root / "cold-cache").exists(), "cold environment is not empty")
        for name, body in agreement.items():
            with (self.root / "cold" / name).open("xb") as target:
                target.write(body)
        require(set(path.name for path in (self.root / "cold").iterdir()) == set(agreement), "copied more than agreement")
        self.run("cold-init")
        self.install("cold-locked-install")
        installed, binding, active = self.owned(self.root / "cold", "uap-cline-public-probe")
        self.remember_data(self.root / "cold", installed, binding)
        self.cold_binding(pin, installed, binding)
        state = read_owned(self.root / "cold/.uap/state-v2.json", self.root)
        rows = document(self.run("cold-inspect"))
        require(read_owned(self.root / "cold/.uap/state-v2.json", self.root) == state, "cold session changed UAP state")
        config = self.root / "cold/.uap/clients/cline/mcp.json"
        require(len(rows) == 1 and rows[0].get("name") == "uap_public_probe"
                and rows[0].get("path") == str(config) and rows[0].get("transportType") == "stdio"
                and rows[0].get("disabled") is False, "cold native source selection differs")
        # Compile the immutable public stdlib source inside the original deadline.
        # The installed projection remains `go run PLUGIN_ROOT/main.go`.
        require(all(not (self.root / name).exists() for name in ("cold-go-cache", "cold-go-path")),
                "cold compiler cache unexpectedly populated")
        self.run("cold-build")
        direct = self.direct("cold-create", "cold")
        marker = Path(installed["data_receipts"][binding["data_receipt_id"]]["locator"]) / "native-marker.txt"
        self.cold_marker = marker, self.marker_facts(marker)
        require(self.cold_marker[1][1] == ("kept-" + self.nonce).encode(), "initial cold marker differs")
        before = read_owned(self.root / "cold/.uap/state-v2.json", self.root)
        self.install("cold-repeat")
        require(read_owned(self.root / "cold/.uap/state-v2.json", self.root) == before, "cold repeat changed state")
        repeated = self.direct("cold-session", "cold")
        require(direct["facts"]["operation"] == "write" and repeated["facts"]["operation"] == "read"
                and repeated["facts"]["marker"] == direct["facts"]["marker"], "cold marker did not survive repeat")
        require(all(read_owned(self.root / "cold" / name, self.root) == body for name, body in agreement.items()), "cold restore rewrote agreement")
        self.evidence["cold_locked_restore"] = {"separate_public_fixture": True, "agreement_only_transfer": True,
            "agreement_sha256": {name: hashlib.sha256(body).hexdigest() for name, body in agreement.items()},
            "source_pin": pin, "binding": binding, "native_inventory": rows,
            "direct_installed_mcp": direct, "repeat_mcp": repeated}

    def cold_binding(self, pin, installation, binding):
        source, package, revision = installation["source"], installation["package"], binding["package_revision"]
        require(source["resolved_revision"] == pin["source"]["git"]["commit"]
                and source["repository"] == pin["source"]["git"]["repository"]
                and source["package_subpath"] == pin["source"]["git"]["path"]
                and source["tree_digest"] == pin["tree_digest"] == revision["tree_digest"]
                and package["manifest_digest"] == pin["manifest_digest"] == revision["manifest_digest"]
                and revision["resolved_revision"] == source["resolved_revision"], "cold package identity differs")
        receipts = document(read_owned(self.root / "cold/.uap/state-v2.json", self.root))["environment_receipts"]
        matches = [r for r in receipts if r["alias"] == pin["alias"] and r["client_id"] == "cline"]
        require(len(matches) == 1 and matches[0]["phase"] == "committed"
                and matches[0]["installation_id"] == installation["installation_id"]
                and matches[0]["client_binding_id"] == binding["client_binding_id"]
                and matches[0]["source_key"] == pin["source_key"]
                and matches[0]["package_digest"] == pin["tree_digest"], "cold environment receipt differs")
        receipt = installation["data_receipts"][binding["data_receipt_id"]]
        require(receipt["scope"] == "project" and receipt["state"] == "owned"
                and receipt["physical_backend_id"] == binding["physical_artifact_id"]
                and receipt["locator"] == str(self.root / "cold/.uap/plugin-data" / binding["physical_artifact_id"]),
                "cold data receipt differs")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("installer", "probe", "helper", "output"):
        parser.add_argument("--" + name, required=True, type=Path)
    args = parser.parse_args()
    require(os.environ.get("GITHUB_ACTIONS") == "true" and os.environ.get("RUNNER_ENVIRONMENT") == "github-hosted",
            "disposable hosted runner required")
    sha, tree = source_identity()
    os.environ["CLINE_SOURCE_TREE"] = tree
    args.output.mkdir(mode=0o700, parents=True, exist_ok=False)
    result = {"source_commit": sha, "source_tree": tree, "platform": platform.system(), "architecture": platform.machine(),
              "passed": False, "commands": [], "stages": [],
              "model_invoked": False, "vendor_authentication_used": False,
              "native_cline_tools_call": False, "installer_sha256": digest(args.installer),
              "probe_sha256": digest(args.probe), "helper_sha256": digest(args.helper)}
    try:
        budget = Budget(900)
        spec = platform_acquisition(platform.system(), platform.machine())
        receipt, contents = acquire(spec, budget)
        result["acquisition"] = receipt
        root = Path(tempfile.mkdtemp(prefix="uap-cline-session-", dir=os.environ["RUNNER_TEMP"])).resolve()
        try:
            native, identities = materialize(root / "vendor", spec, receipt, contents)
            verify_files(identities, budget)
            (root / "bin").mkdir(mode=0o700)
            for leaf, source in (("agentplugins", args.installer), ("probe", args.probe)):
                destination = root / "bin" / (leaf + (".exe" if os.name == "nt" else ""))
                shutil.copyfile(source, destination)
                destination.chmod(0o700)
                identities[destination] = digest(source)
            identities[args.helper.resolve()] = digest(args.helper)
            (root / "tmp").mkdir(mode=0o700)
            suffix = ".exe" if os.name == "nt" else ""
            journey = SessionJourney(root, root / "bin" / ("agentplugins" + suffix), native,
                                     root / "bin" / ("probe" + suffix), args.helper, result, budget, identities)
            try:
                journey.execute()
                verify_files(identities, budget)
                require(journey.settled, "journey did not settle")
                validate_events(result, read_owned(journey.events, root))
                require(source_identity() == (sha, tree), "source changed during native journey")
            finally:
                if journey.settled and journey.events.exists():
                    raw = read_owned(journey.events, root)
                    result["events_base64"] = base64.b64encode(raw).decode()
                    result["events_sha256"] = hashlib.sha256(raw).hexdigest()
        finally:
            print(json.dumps({"fixtures_retained": True, "scope": "disposable-cline-session"}))
        budget.check()
        result["passed"] = True
    except Exception as error:
        result["error"] = type(error).__name__ + ": " + str(error)
        raise
    finally:
        write(args.output / "evidence.json", result)
        print(json.dumps({"passed": result["passed"], "source_commit": sha}))


if __name__ == "__main__":
    main()
