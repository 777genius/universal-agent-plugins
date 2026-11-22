"""Exact Devin Global installation observations with separate native proof.

The caller supplies its validated fixture selections and existing source matcher.
Complete public JSON and native images stay in memory. Reports contain bounded
hashes, counts and checked public states, never native profile contents.
"""
import copy
import hashlib
import json
from pathlib import Path
import re
import stat
import sys
from uuid import UUID

from windsurf_global_skill_audit import CASES as GLOBAL_SKILL_CASES

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


def native_reference(row, source, identity, index):
    """Join the separate closed native proof; installation never creates one."""
    require(index in ACTIVE_SLOTS and identity == ACTIVE_SLOTS[index]
            and source in GLOBAL_SKILL_CASES)
    pin = GLOBAL_SKILL_CASES[source]
    proof = row.get("windsurf_global_native_proof")
    require(type(proof) is dict and proof.get("scenario") == "four-immutable-global-pure-skills"
            and proof.get("source") == source and proof.get("native_discovery_verified") is True
            and proof.get("cache_exception_granted") is False and proof.get("writer_closure_claimed") is False
            and proof.get("reasons") == []
            and encoded(proof.get("independent_source_pin"), 1 << 20) == encoded(pin, 1 << 20))
    owner, stored = proof.get("owner"), proof.get("immutable_source_store")
    require(type(owner) is dict and owner.get("name") == identity
            and owner.get("native_lifecycle_qualified") is True
            and type(owner.get("uuid")) is str and str(UUID(owner["uuid"])) == owner["uuid"]
            and owner.get("revision") == {"version": pin["version"], "resolved_revision": source.rsplit("@", 1)[1],
                "tree_digest": pin["tree_digest"], "manifest_digest": pin["manifest_digest"]}
            and row.get("matched_installation_ids") == row.get("repeat_installation_ids") == [owner["uuid"]]
            and type(stored) is dict and stored.get("tree_digest") == pin["tree_digest"]
            and stored.get("complete_skills") == list(pin["skills"])
            and type(stored.get("held_frame")) is dict and stored["held_frame"] == stored.get("last_verified_frame"))
    observations, transitions = proof.get("observations"), proof.get("public_transitions")
    require(type(observations) is list and all(type(v) is dict for v in observations)
            and [v.get("phase") for v in observations]
            == ["before", "add", "repeat", "remove"]
            and all(v.get("verified") is True and type(v.get("custody")) is dict
                    and v["custody"].get("unchanged") is True
                    and v["custody"].get("before") == v["custody"].get("after") for v in observations)
            and type(transitions) is list and all(type(v) is dict for v in transitions)
            and [v.get("phase") for v in transitions]
            == ["add", "repeat", "remove"] and all(v.get("verified") is True for v in transitions))
    return {"source": source, "installation_id": owner["uuid"], "sha256": digest(proof)}


class WindsurfInstallationAudit:
    """Observe each actual public call before the harness consumes `_json`."""

    def __init__(self, cli, env, project, case_root, audit, *, native_runtime=None):
        require(sys.platform == "linux" and type(cli) is str and Path(cli).is_absolute())
        self.cli, self.audit, self.native_runtime = cli, audit, native_runtime
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
        if native_runtime is None:
            if not self.global_root.exists() and not self.global_root.is_symlink():
                self.global_root.mkdir(mode=0o700)
            self.foreign = self.global_root / "foreign-neighbor.txt"
            with self.foreign.open("xb") as stream:
                stream.write(b"unmanaged Devin Global neighbor must survive\n")
        else:
            # Reuse only the exact native observer's authored, still pre-child
            # profile. Adding another file would invalidate its frozen HOME.
            native_class = audit.get("WINDSURF_GLOBAL_AUDIT", {}).get("WindsurfGlobalSkillAudit")
            require(type(native_class) is type and type(native_runtime) is native_class)
            require(native_runtime.case == self.case and native_runtime.home == self.home
                    and native_runtime.project == self.project and native_runtime.data == self.data
                    and native_runtime.profile == self.global_root and native_runtime.item in GLOBAL_SKILL_CASES
                    and encoded(native_runtime.pin, 1 << 20) == encoded(GLOBAL_SKILL_CASES[native_runtime.item], 1 << 20)
                    and native_runtime.closed is False and native_runtime.finished is False
                    and native_runtime.removing is False and native_runtime.owner is None
                    and native_runtime.probe_frame is None and native_runtime.bound_list == set()
                    and native_runtime.observations == native_runtime.transitions == native_runtime.reasons == [])
            self.foreign = native_runtime.skills / "foreign-windsurf-global-audit/SKILL.md"
        self.anchors[self.global_root] = ordinary(self.global_root)
        self.native_roots = (self.home / ".windsurf", self.home / ".devin", self.global_root)
        self.observations = []
        self.initial = self.image()
        self.project_initial = self.audit["scoped_snapshot"]((self.project,))

    def guard(self):
        require(all(ordinary(path) == identity for path, identity in self.anchors.items()))
        for root in self.native_roots:
            require(self.audit["path_inside_case"](root, self.case))
        if self.native_runtime is not None:
            self.native_runtime.guard()

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
        self.observations.append((list(argv), copy.deepcopy(outcome), image))

    def finish(self, row, item, index, fixtures):
        source, identity = selected(fixtures, index)
        require(item == source and row.get("index") == index
                and (row.get("target"), row.get("kind"), row.get("source"), row.get("identity"))
                == ("windsurf", "plugin", source, identity))
        native_proof = self.native_runtime.evidence() if self.native_runtime is not None else None
        reference = None
        if row.get("status") == "passed":
            require(self.native_runtime is not None and self.native_runtime.finished is True)
            reference = native_reference(row, source, identity, index)
        if self.native_runtime is not None:
            require(index in ACTIVE_SLOTS and self.native_runtime.item == source)
            self.same_native_evidence(row.get("windsurf_global_native_proof"), native_proof)
        self.guard()
        require(self.image() == (self.observations[-1][2] if self.observations else self.initial))
        require(self.audit["scoped_snapshot"]((self.project,)) == self.project_initial)
        scope = ["--target", "windsurf", "--scope", "user", "--format", "json"]
        add, listed = [self.cli, "add", source, *scope], [self.cli, "list", *scope]
        negative = index == 13 or index in COMPONENT_REFUSALS
        remove_identity = identity
        if self.native_runtime is not None:
            owner = self.native_runtime.owner
            require(type(owner) is dict and row.get("matched_installation_ids") == [owner.get("uuid")])
            remove_identity = owner["uuid"]
        expected = [add, listed] if negative else [add, listed, add, listed,
                    [self.cli, "remove", remove_identity, *scope], listed]
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
        if self.native_runtime is not None:
            # Never call finish() twice or rewrite the row. Read-only guards may
            # advance their diagnostic count, but cannot alter any proof fact.
            self.same_native_evidence(native_proof, self.native_runtime.evidence())
        return {"status": "verified-installation-negative" if negative else "verified-installation-lifecycle",
                **({"preserved_native_proof": reference} if reference is not None else {}),
                "scope": "user", "canonical_client_id": "windsurf", "fixture_index": index,
                "source": source, "identity": identity, "public_state": state,
                "native_runtime_qualified": False, "writer_closure_claimed": False,
                "native_roots": [".windsurf", ".devin", ".config/devin"],
                "project_unchanged": True, "original_native_preserved": True,
                "initial_native_image": image_fact(self.initial),
                "observations": [{"argv": argv, "json_sha256": digest(outcome.get("_json"), 1 << 20),
                                  "native_image": image_fact(image)}
                                 for argv, outcome, image in self.observations]}

    @staticmethod
    def same_native_evidence(before, after):
        require(type(before) is dict and type(after) is dict)
        prior, current = before.get("successful_guards"), after.get("successful_guards")
        require(type(prior) is int and type(current) is int and 0 < prior <= current
                and encoded({k: v for k, v in before.items() if k != "successful_guards"}, 4 << 20)
                == encoded({k: v for k, v in after.items() if k != "successful_guards"}, 4 << 20))

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

    def positive(self, row, source, identity, index, fixtures):
        state = ACTIVE_SKILLS if index in ACTIVE_SLOTS else PREPARED
        if source.startswith("github:"):
            require(re.fullmatch(r"github:[^\s@]+@[0-9a-f]{40}", source))
        if index in ACTIVE_SLOTS:
            require(identity == ACTIVE_SLOTS[index] and source.startswith("github:"))
        require((row.get("status") == "prepared-lifecycle-passed"
                 or (row.get("status") == "passed" and self.native_runtime is not None and index in ACTIVE_SLOTS))
                and row.get("prepared_components_only") is True
                and all(row.get(key) is True for key in (
                    "listed_identity", "repeat_verified", "repeat_native_unchanged", "repeat_lock_unchanged",
                    "removal_verified", "post_list_schema_valid"))
                and row.get("post_removal_leftovers") == [])
        require(self.observations[0][2] == self.observations[1][2]
                == self.observations[2][2] == self.observations[3][2]
                and self.observations[4][2] == self.observations[5][2] == self.initial)
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
        return dict(state)


def summary(report, fixtures, *, fixture_sha256):
    """Accept installation coverage and retain the existing four-case native gate."""
    result = {"installation_passed": 0, "installation_prepared_lifecycle_passed": 0,
              "qualification_scope": "owned Devin Global installation lifecycle and exact refusals",
              "installation_active_skill_lifecycle_passed": 0,
              "installation_verified_negative_coverage": 0, "installation_acceptance_passed": False,
              "retained_native_acceptance_passed": False,
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
                and type(report.get("passed")) is int and 0 <= report["passed"] <= len(ACTIVE_SLOTS)
                and type(report["cli_version"].get("exit_code")) is int
                and type(report["client_version"].get("exit_code")) is int
                and type(report.get("results")) is list
                and len(report["results"]) == 30)
        preserved_native = 0
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
            frames = [entry.get("native_image") for entry in proof["observations"] if type(entry) is dict]
            require(len(frames) == (2 if negative else 6))
            for frame in [initial, *frames]:
                require(type(frame) is dict and set(frame) == {"sha256", "entries"}
                        and type(frame["sha256"]) is str and re.fullmatch(r"[0-9a-f]{64}", frame["sha256"])
                        and type(frame["entries"]) is int and frame["entries"] > 0)
            require(all(type(entry.get("json_sha256")) is str
                        and re.fullmatch(r"[0-9a-f]{64}", entry["json_sha256"])
                        for entry in proof["observations"]))
            if row.get("status") == "passed":
                require(proof.get("preserved_native_proof") == native_reference(row, source, identity, index)
                        and row["windsurf_global_native_proof"]["binary"]["path"] == report["client_binary"])
                preserved_native += 1
            else:
                require("preserved_native_proof" not in proof)
            if negative:
                require(frames == [initial, initial]
                        and row.get("status") == ("documented-or-source-unsupported" if index == 13 else "install-failed"))
            else:
                require(frames[0] == frames[1] == frames[2] == frames[3] and frames[4] == frames[5] == initial
                        and row.get("status") in ("prepared-lifecycle-passed", "passed")
                        and row.get("prepared_components_only") is True
                        and all(row.get(key) is True for key in (
                            "listed_identity", "repeat_verified", "repeat_native_unchanged", "repeat_lock_unchanged",
                            "removal_verified", "post_list_schema_valid"))
                        and row.get("post_removal_leftovers") == [])
            result["installation_verified_negative_coverage" if negative else "installation_passed"] += 1
            if not negative:
                result["installation_active_skill_lifecycle_passed" if index in ACTIVE_SLOTS
                       else "installation_prepared_lifecycle_passed"] += 1
        require(report["passed"] == preserved_native)
        require((result["installation_passed"], result["installation_verified_negative_coverage"],
                 result["installation_prepared_lifecycle_passed"], result["installation_active_skill_lifecycle_passed"])
                == (25, 5, 21, 4))
        result["installation_acceptance_passed"] = True
        result["retained_native_acceptance_passed"] = preserved_native == len(ACTIVE_SLOTS)
    except (ValueError, TypeError, KeyError, RecursionError):
        pass
    return result
