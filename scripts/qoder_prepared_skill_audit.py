"""Exact owned Qoder Project Skill installation; never session readiness."""
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import sys
import time
import uuid

ACCEPTANCE = "qoder-prepared-skills-installation"
FACTS = {"materialization": "materialized", "activation": "prepared",
         "authentication": "not_checked", "policy": "allowed",
         "verification": "installation_verified"}


def require(value):
    if not value:
        raise ValueError("Qoder Prepared Project Skill proof is unresolved")


class QoderPreparedSkillAudit:
    """Hold the original native namespace and selected resource directories."""

    def __init__(self, home, project, data, item, api, audits):
        require(sys.platform == "linux" and hasattr(os, "geteuid")
                and hasattr(os, "O_NOFOLLOW") and hasattr(os, "O_DIRECTORY"))
        self.home, self.project, self.data = map(Path, (home, project, data))
        self.case, self.item, self.api = self.project.parent, item, api
        self.custody, self.audits = api["CLAUDE_RETIREMENT_AUDIT"], audits
        self.fds, self.anchors, self.owner = {}, {}, None
        self.removing = False
        self.evidence = {"schema_version": 1, "qualification": "owned-project-file-lifecycle",
                         "native_ready": False, "checkpoints": {}}
        require(self.home.parent == self.data.parent == self.case)
        self.native_root = self.project / ".qoder"
        self.profile = self.home / ".qoder"
        # Existing foreign fixtures create both roots. This observer creates no
        # Qoder/UAP/config/trust/receipt files and never imports the native view.
        for path in (self.case, self.home, self.project, self.data, self.native_root,
                     self.native_root / "skills", self.profile):
            self.anchor(path)
        for key, path in (("native", self.native_root), ("home", self.profile)):
            self.fds[key] = self.custody["open_directory"](path)
            audits.callback(os.close, self.fds[key])
        self.before = {key: self.snapshot(key) for key in ("native", "home")}
        self.evidence["before"] = self.facts(self.before)
        require(not (self.project / ".uap").exists() and not (self.project / ".uap").is_symlink())
        self.guard()

    def checked(self, method, *args):
        require(method in {"guard", "installed", "checkpoint", "begin_remove", "removed"})
        try:
            return getattr(self, method)(*args)
        except (OSError, ValueError, KeyError, TypeError) as error:
            self.evidence["refusal"] = {"stage": method, "type": type(error).__name__}
            error.qoder_prepared_skill_proof = self.evidence
            raise

    def stamp(self, path):
        value = path.lstat()
        require(stat.S_ISDIR(value.st_mode) and path.resolve(strict=True) == path
                and value.st_uid == os.geteuid())
        return value.st_mode, value.st_dev, value.st_ino, value.st_uid, value.st_gid

    def anchor(self, path):
        parent = path
        while parent != self.case.parent:
            self.anchors.setdefault(parent, self.stamp(parent))
            require(parent.is_relative_to(self.case))
            parent = parent.parent

    def guard(self):
        selected = ({Path(self.owner["active"]), Path(self.owner["native_path"])}
                    if self.removing and self.owner is not None else set())
        require(all(self.stamp(path) == expected for path, expected in self.anchors.items() if path not in selected))

    def snapshot(self, key):
        path = {"native": self.native_root, "home": self.profile}.get(key)
        if path is not None:
            self.custody["named_directory"](path, self.fds[key])
        snapshot = self.custody["tree"](self.fds[key])[0]
        require(all(row[7 if row[0] == "file" else 4] == os.geteuid() for row in snapshot.values()))
        return snapshot

    def read(self, path):
        return json.loads(self.bytes(path).decode("utf-8"),
                          object_pairs_hook=self.api["_unique_json_object"],
                          parse_constant=lambda _: require(False))

    def bytes(self, path):
        self.guard()
        started = time.monotonic()
        require(path.is_relative_to(self.case) and path != self.case)
        fds = []
        try:
            fd = self.custody["open_directory"](self.case)
            fds.append(fd)
            pairs = []
            parts = path.relative_to(self.case).parts
            for name in parts[:-1]:
                before = os.stat(name, dir_fd=fd, follow_symlinks=False)
                require(stat.S_ISDIR(before.st_mode) and before.st_uid == os.geteuid())
                child = os.open(name, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW, dir_fd=fd)
                fds.append(child)
                require(os.path.samestat(before, os.fstat(child)))
                pairs.append((fd, name, before))
                fd = child
            name = parts[-1]
            before = os.stat(name, dir_fd=fd, follow_symlinks=False)
            require(stat.S_ISREG(before.st_mode) and before.st_uid == os.geteuid()
                    and before.st_nlink == 1 and 0 <= before.st_size <= 64 << 20)
            child = os.open(name, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK, dir_fd=fd)
            fds.append(child)
            identity = self.custody["_FILES"]["identity"]
            require(identity(before) == identity(os.fstat(child)))
            chunks, size = [], 0
            while True:
                require(time.monotonic() - started < 5)
                block = os.read(child, min(65536, before.st_size - size + 1))
                if not block:
                    break
                chunks.append(block)
                size += len(block)
                require(size <= before.st_size)
            require(size == before.st_size and identity(before) == identity(os.fstat(child))
                    and identity(before) == identity(os.stat(name, dir_fd=fd, follow_symlinks=False)))
            require(all(os.path.samestat(before, os.stat(name, dir_fd=parent, follow_symlinks=False))
                        for parent, name, before in pairs))
            self.guard()
            return b"".join(chunks)
        finally:
            for fd in reversed(fds):
                os.close(fd)

    def skill_digest(self, path, snapshot):
        # clients/shared/digest.go DigestSkillDirectory: lexical WalkDir order,
        # explicit kind/name/executable markers and complete original bytes.
        digest = hashlib.sha256()
        for relative, row in snapshot.items():
            if not relative:
                continue
            digest.update((b"f" if row[0] == "file" else b"d") + b"\0" + relative.encode() + b"\0")
            if row[0] == "file":
                body = self.bytes(path / relative)
                require(len(body) == row[5] and hashlib.sha256(body).hexdigest() == row[-1])
                digest.update((b"x" if row[1] & 0o111 else b"-") + b"\0" + body + b"\0")
        return "sha256:" + digest.hexdigest()

    @staticmethod
    def public(document):
        require(type(document) is dict and type(document.get("schema_version")) is int
                and document["schema_version"] == 1 and document.get("command") == "list"
                and document.get("result") == "success")
        rows = document["data"]["installations"]
        require(type(rows) is list)
        return rows

    def add(self, document, owner):
        require(type(document) is dict and type(document.get("schema_version")) is int
                and document["schema_version"] == 1 and document.get("command") == "skills add"
                and document.get("result") == "success" and document.get("status") == "succeeded"
                and document.get("scope") == "project"
                and document.get("scope_selection_reason") == "explicit"
                and document.get("readiness") == "needs_action")
        actions = document.get("next_actions")
        require(type(actions) is list and len(actions) == 1
                and actions[0].get("code") == "verify_native"
                and actions[0].get("actor") == "native_client"
                and set(actions[0]) == {"code", "actor", "message"}
                and actions[0]["message"] == "Start a fresh selected client session and verify that the installed Skill is discovered."
                and document.get("repairInstructions") == [])
        items = document["items"]
        require(type(items) is list and len(items) == 1 and items[0]["alias"] == self.item["name"]
                and items[0]["targetId"] == "qoder" and items[0]["destinationPath"] == owner["active"]
                and items[0]["status"] in ("succeeded", "unchanged"))

    def inspect(self, listed):
        self.guard()
        require(len(self.public(listed)) == 1)
        matches = self.api["matching_installations"](listed, self.item["name"], "qoder", "skill", self.item)
        require(len(matches) == 1 and self.api["locked_skill_is_pinned"](self.project, self.item, "qoder"))
        public = matches[0]
        iid = public["installation_id"]
        require(type(iid) is str and str(uuid.UUID(iid)) == iid and len(public["clients"]) == 1)
        state = self.read(self.project / ".uap/state-v2.json")
        require(type(state.get("schema_version")) is int and state["schema_version"] == 5)
        records = [row for row in state["installations"] if row.get("clients")]
        require(len(records) == 1 and records[0]["installation_id"] == iid)
        record = records[0]
        require(type(record["clients"]) is dict and len(record["clients"]) == 1)
        binding_id, binding = next(iter(record["clients"].items()))
        source_key = f'github-skill:{self.item["repo"]}:{self.item["name"]}:skills/{self.item["folder"]}'
        source_hash = hashlib.sha256(source_key.encode()).hexdigest()
        declared = "skill-" + source_hash[:56]
        physical = declared[:51].rstrip(".-") + "-" + hashlib.sha256(iid.encode()).hexdigest()[:12]
        active = self.project / ".uap/managed/clients/qoder" / physical
        require(record["origin_mode"] == "direct" and record["declared_name"] == public["name"] == declared
                and binding["client_binding_id"] == binding_id == "client_" + hashlib.sha256(
                    "\0".join((iid, "qoder", "project", str(active))).encode()).hexdigest()[:24]
                and binding["physical_artifact_id"] == physical and binding["target_locator"] == str(active)
                and binding["native_profile_root"] == str(active.parent)
                and binding["affected_surfaces"] == public["clients"][0]["affected_surfaces"] == ["qoder"])
        for row in (binding, public["clients"][0]):
            require(row["client_id"] == "qoder" and row["scope"] == "project"
                    and all(type(row.get(key)) is str and row[key] == value for key, value in FACTS.items()))
        # read_public.go publicPackageRevision deliberately excludes private
        # Skill provenance. Raw normalized Skills expose exactly these3 pins.
        require(public["clients"][0]["package_revision"] == self.api["public_skill_revision"](binding["package_revision"]))
        pin = [p for p in self.read(self.project / "uap.lock")["packages"] if p.get("alias") == self.item["name"]]
        require(len(pin) == 1)
        pin = pin[0]
        source, package, revision = record["source"], record["package"], binding["package_revision"]
        require(source["stable_source_key"] == source_key and source["source_binding_id"] == "src_" + source_hash[:24]
                and source["repository"] == self.item["repo"] and source["package_subpath"] == "skills/" + self.item["folder"]
                and source["resolved_revision"] == revision["resolved_revision"] == self.item["ref"]
                and source["tree_digest"] == revision["tree_digest"] == pin["tree_digest"]
                and package["manifest_digest"] == revision["manifest_digest"] == pin["manifest_digest"]
                and package["declared_name"] == declared and package["format_id"] == "agent-plugins/1.0.0"
                and package["inventory"]["skills"] == [self.item["name"]] and not package["inventory"].get("mcp_present", False))
        skill = revision["skill"]
        require(skill["name"] == self.item["name"] and skill["requested_path"] == "skills/" + self.item["folder"]
                and skill["resolved_path"] == "skills/" + self.item["folder"]
                and skill["normalizer"] == "agent-skill-package/v1"
                and skill["raw_tree_digest"] == pin["source"]["git"]["raw_tree_digest"])
        require(all(type(value) is str and re.fullmatch(r"sha256:[0-9a-f]{64}", value)
                    for value in (revision["tree_digest"], revision["manifest_digest"], skill["raw_tree_digest"])))
        objects = binding["native_objects"]
        require(type(objects) is list and len(objects) == 2)
        managed = [row for row in objects if row.get("kind") == "managed_package_directory"]
        native = [row for row in objects if row.get("kind") == "qoder_project_skill_directory"]
        require(len(managed) == len(native) == 1)
        managed_digest = managed[0]["managed_digest"]
        require(re.fullmatch(r"sha256:[0-9a-f]{64}", managed_digest) is not None)
        require(managed[0] == {"object_id": "package:qoder:" + physical,
                "kind": "managed_package_directory", "logical_name": declared, "path": str(active),
                "managed_digest": managed_digest, "protection_class": "managed"})
        native_path = self.native_root / "skills" / self.item["name"]
        require(native[0] == {"object_id": "qoder_project_skill_directory:" + self.item["name"],
                "kind": "qoder_project_skill_directory", "logical_name": self.item["name"],
                "path": str(native_path), "source_relative": "skills/" + self.item["name"],
                "managed_digest": native[0]["managed_digest"], "protection_class": "managed"})
        receipts = binding["receipts"]
        require(type(receipts) is list and len(receipts) == 1)
        receipt = receipts[0]
        require(receipt["phase"] == "committed" and receipt["client_binding_id"] == binding_id
                and receipt["active_path"] == str(active) and receipt["after_digest"] == managed_digest
                and type(receipt["operation_id"]) is str and bool(receipt["operation_id"])
                and type(receipt["sequence"]) is int and receipt["sequence"] > 0
                and receipt["mutation_type"] == "directory_swap")
        authority = receipt["environment_authority"]
        require(all(authority.get(key) == value for key, value in {
            "environment_id": "project-default", "client_id": "qoder", "expected_action": "install",
            "alias": self.item["name"], "source_key": source_key}.items())
            and all(authority[key] == value for key, value in {
                "installation_id": iid, "client_binding_id": binding_id}.items() if key in authority))
        environment = state["environment_receipts"]
        require(type(environment) is list and len(environment) == 1)
        env = environment[0]
        require(env["phase"] == "committed" and env["environment_id"] == "project-default"
                and env["alias"] == self.item["name"] and env["client_id"] == "qoder"
                and env["installation_id"] == iid and env["client_binding_id"] == binding_id
                and env["source_key"] == source_key and env["package_digest"] == revision["tree_digest"]
                and env["desired_action"] == "install" and type(env["receipt_id"]) is str and bool(env["receipt_id"]))
        require("receipt_id" not in authority or authority["receipt_id"] == env["receipt_id"])
        return {"installation_id": iid, "binding_id": binding_id, "physical": physical,
                "active": str(active), "native_path": str(native_path), "native_receipt": native[0],
                "source": source, "revision": revision, "package": package,
                "binding": binding, "environment_receipt": env}

    def installed(self, listed, added):
        require(self.owner is None)
        owner = self.inspect(listed)
        self.add(added, owner)
        active, native = Path(owner["active"]), Path(owner["native_path"])
        for path in (active, native):
            self.anchor(path)
        for key, path in (("active", active), ("source", active / "skills" / self.item["name"]), ("selected", native)):
            self.fds[key] = self.custody["open_directory"](path)
            self.audits.callback(os.close, self.fds[key])
        self.trees = {key: self.snapshot(key) for key in ("active", "source", "selected")}
        require(self.custody["content"](self.trees["source"]) == self.custody["content"](self.trees["selected"]))
        require(self.skill_digest(active / "skills" / self.item["name"], self.trees["source"])
                == self.skill_digest(native, self.trees["selected"]) == owner["native_receipt"]["managed_digest"])
        body = self.bytes(native / "SKILL.md")
        observed_blob = hashlib.sha1(b"blob " + str(len(body)).encode() + b"\0" + body).hexdigest()
        require(observed_blob == self.item["skill_md_git_blob_sha1"])
        self.evidence["skill_git_blob_sha1"] = observed_blob
        self.evidence["source_resource_snapshot"] = self.trees["source"]
        self.evidence["managed_package_snapshot"] = self.trees["active"]
        self.evidence["digest_scope"] = "native Skill digest independently recomputed; managed package receipt joined with original full-byte custody"
        self.owner = owner
        self.installed_trees = {key: self.snapshot(key) for key in ("native", "home")}
        prefix = "skills/" + self.item["name"]
        actual = self.installed_trees["native"]
        require({key: value for key, value in actual.items() if key != prefix and not key.startswith(prefix + "/")} == self.before["native"])
        require(self.installed_trees["home"] == self.before["home"])
        self.state = self.bytes(self.project / ".uap/state-v2.json")
        self.evidence["owner"] = owner
        self.checkpoint("after-add", listed, added)

    def checkpoint(self, label, listed, added):
        require(self.owner is not None and label in ("after-add", "repeat-add"))
        self.guard()
        self.add(added, self.owner)
        require(self.inspect(listed) == self.owner and self.bytes(self.project / ".uap/state-v2.json") == self.state)
        for key, path in (("active", Path(self.owner["active"])),
                          ("source", Path(self.owner["active"]) / "skills" / self.item["name"]),
                          ("selected", Path(self.owner["native_path"]))):
            self.custody["named_directory"](path, self.fds[key])
            require(self.snapshot(key) == self.trees[key])
        snapshots = {key: self.snapshot(key) for key in ("native", "home")}
        require(snapshots == self.installed_trees)
        self.evidence["checkpoints"][label] = self.facts(snapshots)

    def facts(self, snapshots):
        return {key: {"entries": len(snapshot), "snapshot": snapshot, "snapshot_sha256": hashlib.sha256(
            json.dumps(snapshot, sort_keys=True).encode()).hexdigest()} for key, snapshot in snapshots.items()}

    def begin_remove(self):
        require(self.owner is not None and not self.removing)
        self.guard()
        self.removing = True

    def removed(self, listed, removal):
        require(self.removing and self.owner is not None and set(self.evidence["checkpoints"]) == {"after-add", "repeat-add"})
        require(type(removal) is dict and type(removal.get("schema_version")) is int
                and removal["schema_version"] == 1 and removal.get("command") == "agentplugins remove"
                and removal.get("result") == "success" and removal.get("outcome") == "completed"
                and removal.get("scope") == "project" and removal.get("scope_selection_reason") == "explicit"
                and removal.get("actions") == [{"from_revision": self.owner["revision"]["resolved_revision"],
                    "from_digest": self.owner["revision"]["tree_digest"], "alias": self.item["name"],
                    "client_id": "qoder", "installation_id": self.owner["installation_id"],
                    "binding_id": self.owner["binding_id"], "action": "remove"}]
                and removal.get("results") == [{"installation_id": self.owner["installation_id"],
                    "client_id": "qoder", "operation": "remove", "outcome": "completed"}])
        self.evidence["public_removal"] = removal
        # These selected roots are supposed to disappear. Preserve all original
        # ancestor anchors while checking their original held descriptors.
        for path in (Path(self.owner["active"]), Path(self.owner["native_path"])):
            require(not path.exists() and not path.is_symlink())
            self.anchors.pop(path, None)
        self.guard()
        require(self.public(listed) == [])
        state = self.read(self.project / ".uap/state-v2.json")
        require(type(state.get("schema_version")) is int and state["schema_version"] == 5
                and not any(row.get("clients") for row in state["installations"])
                and not state.get("environment_receipts", []))
        require(not self.api["locked_skill_is_pinned"](self.project, self.item, "qoder"))
        snapshots = {key: self.snapshot(key) for key in ("native", "home")}
        require(snapshots == self.before)
        require(self.snapshot("active") == {"": self.snapshot("active")[""]}
                and self.snapshot("selected") == {"": self.snapshot("selected")[""]})
        self.evidence["checkpoints"]["after-remove"] = self.facts(snapshots)
        self.evidence["status"] = "prepared-lifecycle-verified"
        return True


def summary(report, fixtures, acceptance):
    """A complete exact pinned installation lane, with native PASS unchanged."""
    result = {"installation_passed": 0, "installation_prepared_lifecycle_passed": 0,
              "installation_verified_negative_coverage": 0, "installation_acceptance_passed": False}
    if (acceptance != ACCEPTANCE or (report.get("target"), report.get("kind"), report.get("platform"),
        report.get("architecture"), report.get("canonical_client_id")) != ("qoder", "skill", "linux", "x86_64", "qoder")
        or report.get("client_version") != {"status": "reported", "value": "1.1.81", "exit_code": 0}
        or report.get("cli_version") != {"status": "reported", "value": "agentplugins 0.1.115", "exit_code": 0}
        or any(type(report.get(key)) is not int or report[key] != value for key, value in {
            "fixture_count": 30, "cases_attempted": 30, "passed": 0, "prepared_lifecycle_passed": 30,
            "verified_negative_coverage": 0}.items())
        or report.get("full_coverage") is not True or report.get("source_selection_complete") is not True):
        return result
    rows, sources = report.get("results"), fixtures.get("skills")
    if type(rows) is not list or len(rows) != 30 or type(sources) is not list or len(sources) != 30:
        return result
    for index, (row, item) in enumerate(zip(rows, sources)):
        if (type(row) is not dict or type(row.get("index")) is not int or row["index"] != index
            or (row.get("target"), row.get("kind"), row.get("identity"), row.get("source")) !=
                ("qoder", "skill", item["name"], item["source"])
            or row.get("status") != "prepared-lifecycle-passed"
            or row.get("public_skill_ready") != {"after-add": False, "repeat-add": False}
            or any(row.get(key) is not True for key in ("listed_identity", "lock_exact_source_pinned", "skill_bytes_match",
                "repeat_verified", "repeat_lock_unchanged", "repeat_native_unchanged", "removal_verified",
                "post_list_schema_valid", "sentinels_preserved"))
            or any(row.get(key) is not False for key in ("failed_install_registered", "failed_install_residue",
                "post_remove_identity_registered", "post_lock_pinned"))
            or row.get("post_removal_skill") is not None or row.get("post_removal_leftovers") != []
            or row.get("post_skill_tree_delta") != [] or row.get("native_residuals") != []):
            return result
        proof = row.get("qoder_prepared_skill_proof")
        if (type(proof) is not dict or proof.get("schema_version") != 1 or type(proof["schema_version"]) is not int
            or proof.get("status") != "prepared-lifecycle-verified" or proof.get("native_ready") is not False
            or proof.get("qualification") != "owned-project-file-lifecycle"
            or set(proof.get("checkpoints", {})) != {"after-add", "repeat-add", "after-remove"}):
            return result
        images = [proof.get("before"), *(proof["checkpoints"][phase] for phase in ("after-add", "repeat-add", "after-remove"))]
        for image in images:
            if type(image) is not dict or set(image) != {"native", "home"}:
                return result
            for captured in image.values():
                if (type(captured) is not dict or type(captured.get("snapshot")) is not dict
                    or not 1 <= len(captured["snapshot"]) <= 10000
                    or type(captured.get("entries")) is not int or captured["entries"] != len(captured["snapshot"])
                    or captured.get("snapshot_sha256") != hashlib.sha256(json.dumps(
                        captured["snapshot"], sort_keys=True).encode()).hexdigest()):
                    return result
        if (images[1] != images[2] or images[0] != images[3]
            or proof.get("skill_git_blob_sha1") != item["skill_md_git_blob_sha1"]):
            return result
        owner = proof.get("owner", {})
        iid = owner.get("installation_id")
        if (type(iid) is not str or row.get("matched_installation_ids") != [iid]
            or row.get("repeat_installation_ids") != [iid]
            or owner.get("source", {}).get("resolved_revision") != item["ref"]
            or owner.get("native_path", "").split("/project/", 1)[-1] != ".qoder/skills/" + item["name"]
            or row.get("skill_path") != ".qoder/skills/" + item["name"] + "/SKILL.md"):
            return result
        for phase in ("after-add", "repeat-add", "after-remove"):
            observation = row.get("public_target_states", {}).get(phase, {})
            expected = [] if phase == "after-remove" else [FACTS]
            if (observation.get("status") != "observed" or observation.get("invalid_rows") != 0
                or observation.get("truncated") is not False or observation.get("states") != expected
                or observation.get("selected_installations") != len(expected)
                or observation.get("selected_bindings") != len(expected)):
                return result
        for phase in ("install", "list", "repeat_add", "repeat_list", "cleanup", "post_remove_list"):
            command = row.get(phase)
            if (type(command) is not dict or type(command.get("exit_code")) is not int
                or command["exit_code"] != 0 or command.get("timeout") is not False):
                return result
            try:
                if type(json.loads(command["stdout_tail"])) is not dict:
                    return result
            except (ValueError, KeyError, TypeError):
                return result
        result["installation_passed"] += 1
        result["installation_prepared_lifecycle_passed"] += 1
    result["installation_acceptance_passed"] = result["installation_passed"] == 30
    return result
