"""Exact Devin Global installation observations; never a native runtime gate.

The caller supplies its validated fixture selections and existing source matcher.
Complete public JSON and native images stay in memory. Reports contain bounded
hashes, counts and checked public states, never native profile contents.
"""
import copy
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import sys

ACCEPTANCE = "windsurf-global-installation"
PREPARED = {"materialization": "materialized", "activation": "prepared",
            "authentication": "not_checked", "policy": "allowed",
            "verification": "installation_verified"}
ACTIVE_SKILLS = {**PREPARED, "activation": "active", "authentication": "not_required"}
ACTIVE_SLOTS = {22: "sre-extension", 25: "claude-scientific-writer",
                26: "elements-of-style", 29: "recipe-skills"}
COMPONENT_REFUSALS = {23: "itential-builder", 24: "google-cloud-storage",
                     27: "cometchat-skills", 28: "uizze-ui-slop"}
CLIENT_VERSION = "devin 3000.12.57 (96542e930752)"
CLI_VERSION = "agentplugins 0.1.115"


def require(condition):
    if not condition:
        raise ValueError("Devin Global installation evidence is unresolved")


def encoded(value, limit):
    body = json.dumps(value, sort_keys=True, separators=(",", ":"), allow_nan=False).encode()
    require(len(body) <= limit)
    return body


def digest(value, limit=4 << 20):
    return hashlib.sha256(encoded(value, limit)).hexdigest()


def image_drift_counts(before, after):
    """Bounded counts by key class; never print native paths or values."""
    def category(key):
        if key.startswith("metadata:"):
            return "metadata"
        if key.startswith("directory:"):
            return "directory"
        if key.startswith("root-kind:"):
            return "root"
        if key.startswith(("link:", "root-link:")):
            return "link"
        return "file"

    differences = {}
    for status, entries in (
            ("added", set(after) - set(before)),
            ("removed", set(before) - set(after)),
            ("changed", {key for key in set(before) & set(after) if before[key] != after[key]})):
        for key in entries:
            label = category(key) + "_" + status
            differences[label] = differences.get(label, 0) + 1
    return ",".join(f"{key}={differences[key]}" for key in sorted(differences)) or "none"


def image_drift_diagnostic(before, after):
    """Bounded nonauthorizing roles; unknown native names and values stay private."""
    known = {
        "devin/mcp_config.json": "mcp_document",
        "metadata:2:mcp_config.json": "mcp_document",
        "devin/mcp_config.json.agentplugins.lock": "writer_lock",
        "metadata:2:mcp_config.json.agentplugins.lock": "writer_lock",
        "directory:2:skills": "skills_parent",
        "metadata:2:skills": "skills_parent",
        "devin/config.json": "native_config",
        "metadata:2:config.json": "native_config",
    }
    changed = sorted(
        key for key in set(before) | set(after)
        if (key in before) != (key in after)
        or key in before and before[key] != after[key]
    )
    entries = []
    for key in changed[:8]:
        entries.append({
            "role": known.get(key, "unknown"),
            "key_sha256": hashlib.sha256(
                key.encode("utf-8", "surrogatepass")
            ).hexdigest(),
            "operation": (
                "added" if key not in before
                else "removed" if key not in after
                else "changed"
            ),
            "before_present": key in before,
            "after_present": key in after,
        })
    return {
        "diagnostic_only": True,
        "changed_entries": len(changed),
        "omitted_entries": max(0, len(changed) - len(entries)),
        "entries": entries,
    }


def require_same_image(before, after, phase):
    if before != after:
        diagnostic = encoded(image_drift_diagnostic(before, after), 4096).decode()
        raise ValueError(
            "Devin Global native-image drift " + phase + ": "
            + image_drift_counts(before, after)
            + "; diagnostic=" + diagnostic
        )


def image_fact(image):
    return {"sha256": digest(image), "entries": len(image)}


def ordinary(path):
    info = path.lstat()
    require(stat.S_ISDIR(info.st_mode) and path.resolve(strict=True) == path)
    return info.st_mode, info.st_dev, info.st_ino, info.st_uid, info.st_gid


def selected(fixtures, index):
    require(type(index) is int and 0 <= index < 30)
    sources = fixtures["plugin_aliases"]
    require(type(sources) is list and len(sources) == 30)
    source = sources[index]
    require(type(source) is str)
    return source, fixtures["plugin_declared_names"].get(source, source)


def document(outcome, operation):
    require(type(outcome) is dict and type(outcome.get("exit_code")) is int
            and outcome["exit_code"] == 0 and outcome.get("timeout") is False
            and outcome.get("stderr_full") == "")
    raw = outcome.get("_json")
    require(type(raw) is dict and type(raw.get("schema_version")) is int
            and raw["schema_version"] == 1 and raw.get("result") == "success")
    encoded(raw, 1 << 20)
    require(raw.get("command") in (operation, "agentplugins " + operation))
    if operation == "list":
        payload = raw.get("data")
        require(type(payload) is dict and type(payload.get("installations")) is list)
    else:
        payload = raw.get("data", raw)
        require(type(payload) is dict and payload.get("command") == "agentplugins " + operation
                and payload.get("result") == "success" and payload.get("outcome") == "completed"
                and payload.get("scope") == "user" and payload.get("scope_selection_reason") == "explicit")
    return raw, payload


class WindsurfInstallationAudit:
    """Observe each actual public call before the harness consumes `_json`."""

    def __init__(self, cli, env, project, case_root, audit):
        require(sys.platform == "linux" and type(cli) is str and Path(cli).is_absolute())
        self.cli, self.audit = cli, audit
        self.case, self.project = Path(case_root), Path(project)
        self.home, self.data = self.case / "home", self.case / "uap-data"
        self.xdg = self.home / ".config"
        require(self.case.resolve(strict=True) == self.case and self.project == self.case / "project"
                and env.get("HOME") == str(self.home)
                and env.get("XDG_CONFIG_HOME") == str(self.xdg)
                and env.get("AGENTPLUGINS_HOME") == str(self.data))
        self.anchors = {p: ordinary(p) for p in
                        (self.case, self.home, self.xdg, self.project, self.data)}
        self.global_root = self.xdg / "devin"
        if not self.global_root.exists() and not self.global_root.is_symlink():
            self.global_root.mkdir(mode=0o700)
        self.anchors[self.global_root] = ordinary(self.global_root)
        self.native_roots = (self.home / ".windsurf", self.home / ".devin", self.global_root)
        self.foreign = self.global_root / "foreign-neighbor.txt"
        with self.foreign.open("xb") as stream:
            stream.write(b"unmanaged Devin Global neighbor must survive\n")
        self.observations = []
        self.infrastructure_frames = []
        self.initial = self.image()
        self.terminal_image = self.initial
        self.retained_infrastructure = []
        self.project_initial = self.audit["scoped_snapshot"]((self.project,))

    def guard(self):
        require(all(ordinary(path) == identity for path, identity in self.anchors.items()))
        for root in self.native_roots:
            require(self.audit["path_inside_case"](root, self.case))

    def image(self):
        self.guard()
        image = self.audit["scoped_snapshot"](self.native_roots)
        require(type(image) is dict and all(type(key) is str for key in image)
                and not any(key.startswith(("link:", "root-link:")) for key in image)
                and not any(value in ("unreadable", "symlink", "non-regular")
                            for value in image.values()))
        encoded(image, 4 << 20)
        self.guard()
        return image

    def observe(self, argv, outcome):
        """Consume complete existing parsed JSON, without rerunning any command."""
        require(type(argv) is list and argv and argv[0] == self.cli and len(self.observations) < 6
                and all(type(arg) is str and "\0" not in arg for arg in argv))
        require(type(outcome) is dict)
        encoded(outcome.get("_json"), 1 << 20)
        image = self.image()
        require(self.audit["scoped_snapshot"]((self.project,)) == self.project_initial)
        # Existing foreign files and directory identities remain authoritative.
        require(all(image.get(key) == value for key, value in self.initial.items()))
        self.infrastructure_frames.append(self._infrastructure_frame(image))
        self.observations.append((list(argv), copy.deepcopy(outcome), image))

    def finish(self, row, item, index, fixtures):
        source, identity = selected(fixtures, index)
        require(item == source and row.get("index") == index
                and (row.get("target"), row.get("kind"), row.get("source"), row.get("identity"))
                == ("windsurf", "plugin", source, identity))
        self.guard()
        require(self.image() == (self.observations[-1][2] if self.observations else self.initial))
        require(self.audit["scoped_snapshot"]((self.project,)) == self.project_initial)
        scope = ["--target", "windsurf", "--scope", "user", "--format", "json"]
        add, listed = [self.cli, "add", source, *scope], [self.cli, "list", *scope]
        negative = index == 13 or index in COMPONENT_REFUSALS
        expected = [add, listed] if negative else [add, listed, add, listed,
                    [self.cli, "remove", identity, *scope], listed]
        require([entry[0] for entry in self.observations] == expected)
        require(row.get("sentinels_preserved") is True
                and all(row.get(key) is False for key in (
                    "failed_install_registered", "failed_install_residue", "post_remove_identity_registered"))
                and row.get("native_residuals") == [])
        if negative:
            self.negative(row, source, identity, index)
            state = None
        else:
            state = self.positive(row, source, identity, index, fixtures)
        return {"status": "verified-installation-negative" if negative else "verified-installation-lifecycle",
                "scope": "user", "canonical_client_id": "windsurf", "fixture_index": index,
                "source": source, "identity": identity, "public_state": state,
                "native_runtime_qualified": False, "writer_closure_claimed": False,
                "native_roots": [".windsurf", ".devin", ".config/devin"],
                "project_unchanged": True, "original_native_preserved": True,
                "initial_native_image": image_fact(self.initial),
                "terminal_mode": ("retained-neutral-infrastructure" if self.retained_infrastructure
                                  else "restored-original-image"),
                "retained_infrastructure": list(self.retained_infrastructure),
                "terminal_native_image": image_fact(self.terminal_image),
                "observations": [{"argv": argv, "json_sha256": digest(outcome.get("_json"), 1 << 20),
                                  "native_image": image_fact(image)}
                                 for argv, outcome, image in self.observations]}

    def negative(self, row, source, identity, index):
        require(all(image == self.initial for _, _, image in self.observations))
        first, second = (entry[1] for entry in self.observations)
        if index == 13:
            require(source == identity == "marketing-kit"
                    and row.get("status") == "documented-or-source-unsupported")
            diagnostic = ('agentplugins: no eligible directory release for "marketing-kit": '
                          '777genius/marketing-kit: release does not support complete target set; missing windsurf\n')
        else:
            require(identity == COMPONENT_REFUSALS[index]
                    and re.fullmatch(r"github:[^\s@]+@[0-9a-f]{40}", source)
                    and row.get("status") == "install-failed")
            diagnostic = (f"agentplugins: preflight {identity}/windsurf: "
                          "devin Global supports only selected Skills and MCP components\n")
        require(type(first.get("exit_code")) is int and first["exit_code"] == 1
                and first.get("timeout") is False and first.get("stdout_tail") == ""
                and first.get("stderr_full") == diagnostic and first.get("_json") is None
                and row.get("listed_identity") is False
                and type(row.get("native_file_delta")) is int and row["native_file_delta"] == 0
                and all(row.get(key) == [] for key in (
                    "native_identity_files", "native_component_candidates", "matched_installation_ids",
                    "repeat_installation_ids", "post_removal_leftovers"))
                and all(row.get(key) is False for key in ("repeat_verified", "removal_verified"))
                and all(key in row and row[key] is None for key in
                        ("repeat_add", "repeat_list", "cleanup", "post_remove_list")))
        _, payload = document(second, "list")
        require(payload["installations"] == [])

    def _lock_frame(self, image):
        name = "mcp_config.json.agentplugins.lock"
        key = "devin/" + name
        if key not in image or key in self.initial:
            return None
        path = self.global_root / name
        native = self.audit["NATIVE_ARTIFACTS"]
        before = path.lstat()
        require(stat.S_ISREG(before.st_mode) and before.st_nlink == 1
                and 0 <= before.st_size <= 4096)
        descriptor = native["open_member"](path)
        try:
            opened = os.fstat(descriptor)
            require(native["same_opened"](before, opened)
                    and (opened.st_uid, opened.st_gid) == (before.st_uid, before.st_gid))
            body = os.read(descriptor, before.st_size + 1)
            require(len(body) == before.st_size)
            final_held = os.fstat(descriptor)
            final_named = path.lstat()
            require(native["identity"](opened) == native["identity"](final_held)
                    == native["identity"](final_named)
                    and (before.st_uid, before.st_gid) == (opened.st_uid, opened.st_gid)
                    == (final_held.st_uid, final_held.st_gid)
                    == (final_named.st_uid, final_named.st_gid))
        finally:
            os.close(descriptor)
        identity = (before.st_mode, before.st_dev, before.st_ino,
                    before.st_uid, before.st_gid, before.st_nlink, before.st_mtime_ns)
        require(image.get(key) == hashlib.sha256(body).hexdigest()
                and image.get("metadata:2:" + name)
                == (before.st_mode, before.st_dev, before.st_ino, before.st_mtime_ns))
        return identity, body

    def _infrastructure_frame(self, image):
        self.guard()
        try:
            lock = self._lock_frame(image)
            skill = None
            key = "directory:2:skills"
            if key in image and key not in self.initial:
                skill = ordinary(self.global_root / "skills")
                require(image.get("metadata:2:skills") == skill[:3])
            facts = (lock, skill)
        except Exception:
            # Extra retention proof is optional for exact restoration.
            # An unproven frame cannot authorize retained infrastructure.
            facts = None
        self.guard()
        return facts

    def _neutral_file(self, name, frame):
        require(name in ("mcp_config.json", "mcp_config.json.agentplugins.lock"))
        self.guard()
        path = self.global_root / name
        require(self.audit["path_inside_case"](path, self.case))
        native = self.audit["NATIVE_ARTIFACTS"]
        before = path.lstat()
        require(stat.S_ISREG(before.st_mode) and before.st_mode == 0o100600
                and before.st_nlink == 1 and 0 <= before.st_size <= 4096
                and (before.st_uid, before.st_gid) == self.anchors[self.global_root][3:])
        descriptor = native["open_member"](path)
        try:
            opened = os.fstat(descriptor)
            require(native["same_opened"](before, opened)
                    and (opened.st_uid, opened.st_gid) == (before.st_uid, before.st_gid))
            body = os.read(descriptor, before.st_size + 1)
            require(len(body) == before.st_size)
            final_held = os.fstat(descriptor)
            final_named = path.lstat()
            require(native["identity"](opened) == native["identity"](final_held)
                    == native["identity"](final_named)
                    and (before.st_uid, before.st_gid) == (opened.st_uid, opened.st_gid)
                    == (final_held.st_uid, final_held.st_gid)
                    == (final_named.st_uid, final_named.st_gid))
        finally:
            os.close(descriptor)
        if name == "mcp_config.json":
            value = json.loads(body.decode("utf-8"), object_pairs_hook=self.audit["_unique_json_object"],
                               parse_constant=self.audit["_invalid_json_constant"])
            require(type(value) is dict and set(value) == {"mcpServers"}
                    and type(value["mcpServers"]) is dict and not value["mcpServers"])
        else:
            require(body == b"")
        file_key, metadata_key = "devin/" + name, "metadata:2:" + name
        metadata = (before.st_mode, before.st_dev, before.st_ino, before.st_mtime_ns)
        require(frame.get(file_key) == hashlib.sha256(body).hexdigest()
                and frame.get(metadata_key) == metadata)
        self.guard()
        return {file_key: frame[file_key], metadata_key: metadata}

    def _expected_removed_image(self, frames, *, mcp, skills):
        require(type(mcp) is bool and type(skills) is bool and len(frames) == 6)
        require_same_image(frames[4], frames[5], "remove/list")
        if frames[4] == self.initial:
            require_same_image(self.initial, self.image(), "final/original")
            self.guard()
            return dict(self.initial), []
        require(len(self.infrastructure_frames) == 6
                and all(type(fact) is tuple and len(fact) == 2
                        for fact in self.infrastructure_frames))
        expected, roles = dict(self.initial), []
        if mcp:
            for name, role in (("mcp_config.json", "mcp_document"),
                               ("mcp_config.json.agentplugins.lock", "writer_lock")):
                keys = ("devin/" + name, "metadata:2:" + name)
                require(all(key not in self.initial for key in keys))
                require(all(key in frame for frame in frames for key in keys))
                additions = self._neutral_file(name, frames[5])
                if role == "writer_lock":
                    require(all(all(frame[key] == additions[key] for key in keys) for frame in frames))
                    locked = self.infrastructure_frames[5][0]
                    require(locked is not None and locked[1] == b""
                            and locked[0][0] == 0o100600
                            and locked[0][3:6] == (*self.anchors[self.global_root][3:], 1)
                            and all(fact[0] == locked for fact in self.infrastructure_frames))
                expected.update(additions)
                roles.append(role)
        if skills:
            keys = ("directory:2:skills", "metadata:2:skills")
            require(all(key not in self.initial for key in keys))
            path = self.global_root / "skills"
            identity = ordinary(path)
            require(identity[0] == 0o40700
                    and identity[3:] == self.anchors[self.global_root][3:]
                    and all(fact[1] == identity for fact in self.infrastructure_frames))
            with os.scandir(path) as entries:
                require(next(entries, None) is None)
            require(ordinary(path) == identity)
            additions = {keys[0]: "present", keys[1]: identity[:3]}
            require(all(all(frame.get(key) == value for key, value in additions.items())
                        for frame in frames))
            expected.update(additions)
            roles.append("skills_parent")
        self.guard()
        require_same_image(expected, frames[4], "remove/neutral-profile")
        require_same_image(expected, frames[5], "post-remove/neutral-profile")
        require_same_image(frames[5], self.image(), "final/observed")
        return expected, sorted(roles)

    def positive(self, row, source, identity, index, fixtures):
        state = ACTIVE_SKILLS if index in ACTIVE_SLOTS else PREPARED
        if source.startswith("github:"):
            require(re.fullmatch(r"github:[^\s@]+@[0-9a-f]{40}", source))
        if index in ACTIVE_SLOTS:
            require(identity == ACTIVE_SLOTS[index] and source.startswith("github:"))
        require(row.get("status") == "prepared-lifecycle-passed"
                and row.get("prepared_components_only") is True
                and all(row.get(key) is True for key in (
                    "listed_identity", "repeat_verified", "repeat_native_unchanged", "repeat_lock_unchanged",
                    "removal_verified", "post_list_schema_valid"))
                and row.get("post_removal_leftovers") == [])
        frames = [entry[2] for entry in self.observations]
        for phase, before, after in (
                ("add/list", frames[0], frames[1]),
                ("add/repeat-add", frames[0], frames[2]),
                ("add/repeat-list", frames[0], frames[3]),
                ("remove/list", frames[4], frames[5])):
            require_same_image(before, after, phase)
        documents = [document(outcome, operation) for (_, outcome, _), operation in
                     zip(self.observations, ("add", "list", "add", "list", "remove", "list"))]
        prior = None
        for position in (1, 3):
            raw, payload = documents[position]
            entries = payload["installations"]
            matched = self.audit["matching_installations"](
                raw, identity, "windsurf", "plugin", source, fixtures["signed_release_pins"])
            require(type(entries) is list and len(entries) == 1 and matched == entries)
            node = entries[0]
            clients = node.get("clients")
            require(type(clients) is list and len(clients) == 1 and type(clients[0]) is dict
                    and clients[0].get("client_id") == "windsurf" and clients[0].get("scope") == "user"
                    and all(clients[0].get(key) == value for key, value in state.items()))
            require(type(node.get("installation_id")) is str and bool(node["installation_id"])
                    and node.get("mixed_version") is False)
            require(prior is None or node == prior)
            prior = node
        installation = prior["installation_id"]
        require(row.get("matched_installation_ids") == row.get("repeat_installation_ids") == [installation]
                and documents[5][1]["installations"] == [])
        for position, action, operation in ((0, "install", "install"), (2, "unchanged", "install"),
                                            (4, "remove", "remove")):
            payload = documents[position][1]
            actions, results = payload.get("actions"), payload.get("results")
            require(type(actions) is list and len(actions) == 1 and type(actions[0]) is dict
                    and actions[0].get("alias") == identity and actions[0].get("client_id") == "windsurf"
                    and actions[0].get("action") == action and type(results) is list)
            if position == 0:
                components = actions[0].get("component_support")
                require(type(components) is list and bool(components)
                        and all(type(component) is dict and component.get("support") == "prepared"
                                for component in components))
            if position != 0:
                require(actions[0].get("installation_id") == installation)
            if position == 2 and not results:
                continue
            require(len(results) == 1 and type(results[0]) is dict
                    and results[0].get("client_id") == "windsurf"
                    and results[0].get("installation_id") == installation
                    and results[0].get("operation") == operation
                    and results[0].get("outcome") in (("completed", "unchanged") if position == 2 else ("completed",)))
        components = documents[0][1]["actions"][0]["component_support"]
        require(all(component.get("kind") in ("skill", "mcp_server") for component in components))
        kinds = {component["kind"] for component in components}
        self.terminal_image, self.retained_infrastructure = self._expected_removed_image(
            frames, mcp="mcp_server" in kinds, skills="skill" in kinds)
        return dict(state)


def summary(report, fixtures, *, fixture_sha256):
    """Accept one explicitly selected installation audit; native counts stay zero."""
    result = {"installation_passed": 0, "installation_prepared_lifecycle_passed": 0,
              "qualification_scope": "owned Devin Global lifecycle with exact restoration or verified retained neutral infrastructure, and exact refusals",
              "installation_active_skill_lifecycle_passed": 0,
              "installation_verified_negative_coverage": 0, "installation_acceptance_passed": False,
              "native_runtime_qualified": False, "authentication_qualified": False,
              "model_qualified": False, "mcp_tools_qualified": False}
    try:
        require((report.get("target"), report.get("kind"), report.get("platform"), report.get("architecture"))
                == ("windsurf", "plugin", "linux", "x86_64")
                and report.get("cli_version") == {"status": "reported", "value": CLI_VERSION, "exit_code": 0}
                and report.get("client_version") == {"status": "reported", "value": CLIENT_VERSION, "exit_code": 0}
                and type(report.get("client_binary")) is str
                and Path(report["client_binary"]).is_absolute() and Path(report["client_binary"]).name == "devin"
                and type(report.get("cli")) is str and Path(report["cli"]).is_absolute()
                and re.fullmatch(r"[0-9a-f]{64}", fixture_sha256)
                and report.get("fixture_sha256") == fixture_sha256
                and report.get("source_selection_complete") is True and report.get("full_coverage") is True
                and type(report.get("fixture_count")) is int and report["fixture_count"] == 30
                and type(report.get("cases_attempted")) is int and report["cases_attempted"] == 30
                and type(report.get("passed")) is int and report["passed"] == 0
                and type(report["cli_version"].get("exit_code")) is int
                and type(report["client_version"].get("exit_code")) is int
                and type(report.get("results")) is list
                and len(report["results"]) == 30)
        for index, row in enumerate(report["results"]):
            require(type(row) is dict)
            source, identity = selected(fixtures, index)
            proof = row.get("windsurf_installation_audit")
            negative = index == 13 or index in COMPONENT_REFUSALS
            require(type(row.get("index")) is int and row["index"] == index
                    and (row.get("target"), row.get("kind"), row.get("source"), row.get("identity"))
                    == ("windsurf", "plugin", source, identity)
                    and type(proof) is dict and type(proof.get("fixture_index")) is int
                    and proof["fixture_index"] == index
                    and (proof.get("source"), proof.get("identity"), proof.get("canonical_client_id"), proof.get("scope"))
                    == (source, identity, "windsurf", "user")
                    and proof.get("native_runtime_qualified") is False
                    and proof.get("writer_closure_claimed") is False
                    and proof.get("native_roots") == [".windsurf", ".devin", ".config/devin"]
                    and proof.get("project_unchanged") is True
                    and proof.get("original_native_preserved") is True
                    and proof.get("status") == ("verified-installation-negative" if negative else "verified-installation-lifecycle")
                    and proof.get("public_state") == (None if negative else ACTIVE_SKILLS if index in ACTIVE_SLOTS else PREPARED)
                    and type(proof.get("observations")) is list
                    and len(proof["observations"]) == (2 if negative else 6))
            require(row.get("sentinels_preserved") is True
                    and row.get("native_residuals") == []
                    and all(row.get(key) is False for key in (
                        "failed_install_registered", "failed_install_residue", "post_remove_identity_registered")))
            initial = proof.get("initial_native_image")
            terminal = proof.get("terminal_native_image")
            roles = proof.get("retained_infrastructure")
            mode = proof.get("terminal_mode")
            require(type(roles) is list and roles in (
                [], ["mcp_document", "writer_lock"], ["skills_parent"],
                ["mcp_document", "skills_parent", "writer_lock"]))
            frames = [entry.get("native_image") for entry in proof["observations"] if type(entry) is dict]
            require(len(frames) == (2 if negative else 6))
            for frame in [initial, terminal, *frames]:
                require(type(frame) is dict and set(frame) == {"sha256", "entries"}
                        and type(frame["sha256"]) is str and re.fullmatch(r"[0-9a-f]{64}", frame["sha256"])
                        and type(frame["entries"]) is int and frame["entries"] > 0)
            require(all(type(entry.get("json_sha256")) is str
                        and re.fullmatch(r"[0-9a-f]{64}", entry["json_sha256"])
                        for entry in proof["observations"]))
            if roles:
                require(mode == "retained-neutral-infrastructure" and not negative
                        and terminal != initial
                        and terminal["entries"] == initial["entries"] + 2 * len(roles)
                        and (index not in ACTIVE_SLOTS or roles == ["skills_parent"]))
            else:
                require(mode == "restored-original-image" and terminal == initial)
            if negative:
                require(frames == [initial, initial]
                        and row.get("status") == ("documented-or-source-unsupported" if index == 13 else "install-failed"))
            else:
                require(frames[0] == frames[1] == frames[2] == frames[3]
                        and frames[4] == frames[5] == terminal
                        and row.get("status") == "prepared-lifecycle-passed"
                        and row.get("prepared_components_only") is True
                        and all(row.get(key) is True for key in (
                            "listed_identity", "repeat_verified", "repeat_native_unchanged", "repeat_lock_unchanged",
                            "removal_verified", "post_list_schema_valid"))
                        and row.get("post_removal_leftovers") == [])
            result["installation_verified_negative_coverage" if negative else "installation_passed"] += 1
            if not negative:
                result["installation_active_skill_lifecycle_passed" if index in ACTIVE_SLOTS
                       else "installation_prepared_lifecycle_passed"] += 1
        require((result["installation_passed"], result["installation_verified_negative_coverage"],
                 result["installation_prepared_lifecycle_passed"], result["installation_active_skill_lifecycle_passed"])
                == (25, 5, 21, 4))
        result["installation_acceptance_passed"] = True
    except (ValueError, TypeError, KeyError, RecursionError):
        pass
    return result
