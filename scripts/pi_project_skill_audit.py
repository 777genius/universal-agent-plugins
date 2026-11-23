"""Exact Pi foreign-trust refusal and clean native Project Skill cohorts."""
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import sys
import time
import uuid
import base64

FOREIGN_ACCEPTANCE = "pi-foreign-prepared-skills-installation"
NATIVE_ACCEPTANCE = "pi-clean-native-skills"
FACTS = {"materialization": "materialized", "activation": "prepared",
         "authentication": "not_checked", "policy": "allowed",
         "verification": "installation_verified"}


def require(value):
    if not value:
        raise ValueError("Pi Project Skill proof is unresolved")


def pinned_version(value, expected):
    return (type(value) is dict and value == {"status": "reported", "value": expected, "exit_code": 0}
            and type(value.get("exit_code")) is int)


class PiProjectSkillAudit:
    """Hold the original native namespace and selected resource directories."""

    def __init__(self, home, project, data, item, api, audits, mode, env, client_version, cli, run_command):
        require(sys.platform == "linux" and hasattr(os, "geteuid")
                and hasattr(os, "O_NOFOLLOW") and hasattr(os, "O_DIRECTORY"))
        require(mode in ("foreign-prepared", "clean-native")
                and pinned_version(client_version, "1.5.0"))
        self.mode, self.env, self.cli, self.run_command = mode, env, cli, run_command
        self.binary = api["AGENT_BINARIES"].get("pi")
        require(type(self.binary) is str and Path(self.binary).is_absolute())
        self.home, self.project, self.data = map(Path, (home, project, data))
        self.case, self.item, self.api = self.project.parent, item, api
        self.custody, self.audits = api["CLAUDE_RETIREMENT_AUDIT"], audits
        self.fds, self.anchors, self.owner = {}, {}, None
        self.removing = False
        self.evidence = {"schema_version": 1, "qualification": mode, "native_ready": False,
                         "producer_attribution": False, "writer_closure": False,
                         "vendor_authentication": False, "model_calls": False,
                         "checkpoints": {}, "native_catalogs": {}}
        require(self.home.parent == self.data.parent == self.case)
        self.native_root = self.project / ".pi"
        self.profile = self.home / ".pi"
        self.hook = self.profile / "agent/extensions/agentplugins-project-trust"
        self.foreign = self.native_root / "skills/foreign-neighbor/SKILL.md"
        require(self.foreign.exists() == (mode == "foreign-prepared"))
        # Both fixture cohorts create these empty authority roots before capture.
        # This observer never creates native trust, receipts or managed files.
        for path in (self.case, self.home, self.project, self.data, self.native_root,
                     self.native_root / "skills", self.native_root / "extensions", self.profile):
            self.anchor(path)
        for key, path in (("native", self.native_root), ("home", self.profile)):
            self.fds[key] = self.custody["open_directory"](path)
            audits.callback(os.close, self.fds[key])
        self.before = {key: self.snapshot(key) for key in ("native", "home")}
        # Both are new fixture cohorts. Never adopt a pre-existing saved
        # decision, hook owner, native package, receipt or runtime store.
        expected_native = {"", "extensions", "skills"}
        if mode == "foreign-prepared":
            expected_native |= {"skills/foreign-neighbor", "skills/foreign-neighbor/SKILL.md"}
        require(set(self.before["native"]) == expected_native
                and set(self.before["home"]) == {"", "foreign-neighbor.txt"}
                and self.bytes(self.profile / "foreign-neighbor.txt") == b"unmanaged native neighbor must survive\n")
        self.evidence["before"] = self.facts(self.before)
        require(not (self.project / ".uap").exists() and not (self.project / ".uap").is_symlink())
        self.guard()

    def checked(self, method, *args):
        require(method in {"guard", "installed", "checkpoint", "begin_remove", "removed"})
        try:
            return getattr(self, method)(*args)
        except (OSError, ValueError, KeyError, TypeError) as error:
            self.evidence["refusal"] = {"stage": method, "type": type(error).__name__}
            error.pi_project_skill_proof = self.evidence
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
                and document.get("readiness") == ("needs_action" if self.mode == "foreign-prepared" else "ready"))
        actions = document.get("next_actions")
        if self.mode == "foreign-prepared":
            require(actions == [{"code": "verify_native", "actor": "native_client",
                "message": "Start a fresh selected client session and verify that the installed Skill is discovered."}])
        else:
            require(actions in (None, []))
        require(document.get("repairInstructions") == [])
        items = document["items"]
        require(type(items) is list and len(items) == 1 and items[0]["alias"] == self.item["name"]
                and items[0]["targetId"] == "pi" and items[0]["destinationPath"] == owner["active"]
                and items[0]["status"] in ("succeeded", "unchanged"))

    def inspect(self, listed):
        self.guard()
        require(len(self.public(listed)) == 1)
        matches = self.api["matching_installations"](listed, self.item["name"], "pi", "skill", self.item)
        require(len(matches) == 1 and self.api["locked_skill_is_pinned"](self.project, self.item, "pi"))
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
        active = self.native_root / "extensions" / physical
        require(record["origin_mode"] == "direct" and record["declared_name"] == public["name"] == declared
                and binding["client_binding_id"] == binding_id == "client_" + hashlib.sha256(
                    "\0".join((iid, "pi", "project", str(active))).encode()).hexdigest()[:24]
                and binding["physical_artifact_id"] == physical and binding["target_locator"] == str(active)
                and binding["native_profile_root"] == str(self.native_root)
                and binding["affected_surfaces"] == public["clients"][0]["affected_surfaces"] == ["pi"])
        for row in (binding, public["clients"][0]):
            require(row["client_id"] == "pi" and row["scope"] == "project"
                    and all(type(row.get(key)) is str and row[key] == value for key, value in self.expected_facts().items()))
        revision = binding["package_revision"]
        # read_public.go intentionally omits private Skill/WebSkill/CatalogEvidence.
        require(self.custody["same_json"](self.api["public_skill_revision"](revision), public["clients"][0]["package_revision"]))
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
        native = [row for row in objects if row.get("kind") == "pi_project_skill_directory"]
        require(len(managed) == len(native) == 1)
        managed_digest = managed[0]["managed_digest"]
        require(re.fullmatch(r"sha256:[0-9a-f]{64}", managed_digest) is not None)
        require(managed[0] == {"object_id": "package:pi:" + physical,
                "kind": "managed_package_directory", "logical_name": declared, "path": str(active),
                "managed_digest": managed_digest, "protection_class": "managed"})
        native_path = self.native_root / "skills" / self.item["name"]
        require(native[0] == {"object_id": "pi_project_skill_directory:" + self.item["name"],
                "kind": "pi_project_skill_directory", "logical_name": self.item["name"],
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
            "environment_id": "project-default", "client_id": "pi", "expected_action": "install",
            "alias": self.item["name"], "source_key": source_key}.items())
            and all(authority[key] == value for key, value in {
                "installation_id": iid, "client_binding_id": binding_id}.items() if key in authority))
        environment = state["environment_receipts"]
        require(type(environment) is list and len(environment) == 1)
        env = environment[0]
        require(env["phase"] == "committed" and env["environment_id"] == "project-default"
                and env["alias"] == self.item["name"] and env["client_id"] == "pi"
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
        self.owner, self.listed = owner, listed
        self.installed_trees = {key: self.snapshot(key) for key in ("native", "home")}
        self.trust_check()
        self.namespace_check(self.installed_trees)
        self.state = self.bytes(self.project / ".uap/state-v2.json")
        self.evidence["owner"] = owner
        if self.mode == "clean-native":
            self.source_metadata()
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
        self.trust_check()
        self.catalog(label, removed=False)
        require({key: self.snapshot(key) for key in ("native", "home")} == snapshots)
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
                    "client_id": "pi", "installation_id": self.owner["installation_id"],
                    "binding_id": self.owner["binding_id"], "action": "remove"}]
                and removal.get("results") == [{"installation_id": self.owner["installation_id"],
                    "client_id": "pi", "operation": "remove", "outcome": "completed"}])
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
        require(not self.api["locked_skill_is_pinned"](self.project, self.item, "pi"))
        snapshots = {key: self.snapshot(key) for key in ("native", "home")}
        self.namespace_check(snapshots, removed=True)
        self.catalog("after-remove", removed=True)
        require({key: self.snapshot(key) for key in ("native", "home")} == snapshots)
        require(self.snapshot("active") == {"": self.snapshot("active")[""]}
                and self.snapshot("selected") == {"": self.snapshot("selected")[""]})
        self.evidence["checkpoints"]["after-remove"] = self.facts(snapshots)
        self.evidence["native_ready"] = self.mode == "clean-native"
        self.evidence["status"] = ("prepared-lifecycle-verified" if self.mode == "foreign-prepared" else "native-lifecycle-verified")
        return True

    def expected_facts(self):
        return {**FACTS, **({"activation": "active", "authentication": "not_required"}
                           if self.mode == "clean-native" else {})}

    def trust_digest(self, path, image):
        digest = hashlib.sha256()
        for name, row in image.items():
            if not name:
                continue
            digest.update((b"d" if row[0] == "dir" else b"f") + b"\0" + name.encode() + b"\0")
            if row[0] == "file":
                body = self.bytes(path / name)
                require(hashlib.sha256(body).hexdigest() == row[-1])
                digest.update(row[-1].encode() + b"\0")
        return "sha256:" + digest.hexdigest()

    def trust_check(self):
        self.guard()
        require(not any(p.exists() or p.is_symlink() for p in
                        (self.profile / "agent/trust.json", self.profile / "agent/settings.json")))
        record = self.read(self.hook / "projects.json")
        owner = {"extensions": {self.owner["physical"]: self.trust_digest(
                    Path(self.owner["active"]), self.trees["active"])},
                 "skills": {self.item["name"]: self.trust_digest(
                    Path(self.owner["native_path"]), self.trees["selected"])}, "mcp_servers": {}}
        require(self.custody["same_json"](record, {"schema_version": 1,
                "projects": {str(self.project): {self.owner["physical"]: owner}}}))
        body = self.bytes(self.hook / "index.ts")
        # Exact 92344 project_trust_hook.go literal; never accept a marker alone.
        require(len(body) == 9078 and hashlib.sha256(body).hexdigest() ==
                "eb4fa44cb880e1df05e42356f7e4bef9efef4abad82495cdfc87867b76b90a73")
        require(self.foreign.exists() == (self.mode == "foreign-prepared"))
        if self.mode == "foreign-prepared":
            require(self.bytes(self.foreign) == b"---\nname: foreign-neighbor\ndescription: unrelated\n---\n")
        self.evidence["trust"] = {"record": record,
            "record_sha256": hashlib.sha256(self.bytes(self.hook / "projects.json")).hexdigest(),
            "hook_bytes": len(body), "hook_sha256": hashlib.sha256(body).hexdigest(),
            "saved_trust_absent": True, "settings_override_absent": True,
            "foreign_blocker": ".pi/skills/foreign-neighbor is not managed by UAP" if self.mode == "foreign-prepared" else None,
            "blocker_evidence": "exact source contract plus held namespace; not an archived hook callback"}
        self.guard()

    def namespace_check(self, images, removed=False):
        prefixes = ("extensions/" + self.owner["physical"], "skills/" + self.item["name"])
        native = images["native"]
        foreign = {key: value for key, value in native.items()
                   if not any(key == prefix or key.startswith(prefix + "/") for prefix in prefixes)}
        require(foreign == self.before["native"])
        home = images["home"]
        prefix = "agent/extensions/agentplugins-project-trust/"
        directories = {"agent", "agent/extensions", prefix[:-1]}
        files = {prefix + name for name in ("index.ts", "projects.json", "index.ts.agentplugins.lock", "projects.json.agentplugins.lock")}
        # Official Pi1.5.0 offline startup creates only these empty JSON stores
        # (AuthStorage and FileModelsStore both use FileAuthStorageBackend).
        runtime_files = {"agent/auth.json", "agent/models-store.json"} if self.mode == "clean-native" else set()
        files |= runtime_files
        require(set(home) == set(self.before["home"]) | directories | (files -
                ({prefix + "index.ts", prefix + "projects.json"} if removed else set())))
        require(all(home[key] == value for key, value in self.before["home"].items()))
        for key in directories:
            row = home[key]
            require(row[0] == "dir" and row[1] == stat.S_IFDIR | 0o700 and row[4] == os.geteuid())
        for key in files & set(home):
            row = home[key]
            require(row[0] == "file" and row[1] == stat.S_IFREG | 0o600
                    and row[4] == 1 and row[7] == os.geteuid())
            if key in runtime_files:
                require(row[5] == 2 and row[-1] == hashlib.sha256(b"{}").hexdigest()
                        and self.bytes(self.profile / key) == b"{}")
            if key.endswith(".agentplugins.lock"):
                require(row[5] == 0 and row[-1] == hashlib.sha256(b"").hexdigest())
        self.evidence["native_profile_stores"] = {key: home[key] for key in sorted(runtime_files)}
        if removed:
            before = self.installed_trees["home"]
            require(all(home[key] == before[key] for key in home))
            require(not any(p.exists() or p.is_symlink() for p in
                            (self.hook / "index.ts", self.hook / "projects.json")))
        else:
            for path in (self.profile / "agent", self.hook.parent, self.hook):
                self.anchor(path)

    def source_metadata(self):
        """Run every post-read custody check even when command/parsing refuses."""
        self.guard()
        paths = (self.project / ".uap/state-v2.json", self.project / "uap.lock", self.project / "uap.toml",
                 self.project / "foreign-untouched.txt", self.home / "foreign-untouched.txt")
        identity = self.custody["_FILES"]["identity"]
        documents = {path: (self.bytes(path), identity(path.lstat())) for path in paths}
        images = {key: self.snapshot(key) for key in ("native", "home", "active", "source", "selected")}
        captured = {"qualification": "exact-selected-source-document; nested Skills not enumerated"}
        self.evidence["source_metadata"] = captured
        primary = None
        try:
            self._source_metadata(captured)
        except BaseException as error:
            primary = error
            captured["refusal"] = type(error).__name__
            raise
        finally:
            try:
                self.guard()
                require({key: self.snapshot(key) for key in images} == images
                        and all(self.bytes(path) == body and identity(path.lstat()) == stamp
                                for path, (body, stamp) in documents.items())
                        and self.inspect(self.listed) == self.owner)
                captured["owned_resources_unchanged"] = True
                self.guard()
            except BaseException as error:
                captured.update(owned_resources_unchanged=False, custody_refusal=type(error).__name__)
                if primary is None:
                    raise
                primary.add_note("Pi source metadata post-command custody also refused: " + type(error).__name__)

    def _source_metadata(self, captured):
        argv = [self.cli, "skills", "list", self.item["source"],
                "--path", "skills/" + self.item["folder"], "--scope", "project",
                "--project-root", str(self.project), "--format", "json"]
        captured.update(argv=argv, cwd=str(self.project))
        output = self.run_command(argv, self.env, self.project, 150, capture_full=True)
        raw = output.pop("_json", None)
        stdout, stderr = output.pop("stdout_full", None), output.pop("stderr_full", None)
        captured.update(argv=argv, cwd=str(self.project), result=output,
                        qualification="exact-selected-source-document; nested Skills not enumerated")
        require(type(stdout) is str and type(stderr) is str)
        for label, body in (("stdout", stdout.encode()), ("stderr", stderr.encode())):
            require(len(body) <= 1 << 20)
            captured[label + "_base64"] = base64.b64encode(body).decode()
            captured[label + "_bytes"] = len(body)
            captured[label + "_sha256"] = hashlib.sha256(body).hexdigest()
        require(type(output.get("exit_code")) is int and output["exit_code"] == 0
                and output.get("timeout") is False and stderr == "")
        document, description = source_description(captured, self.item, self.project, self.cli)
        require(self.custody["same_json"](raw, document))
        captured.update(document=document, skill_git_blob_sha1=self.evidence["skill_git_blob_sha1"])
        self.description = description

    def catalog(self, phase, removed):
        if self.mode != "clean-native":
            return
        self.guard()
        nonce = "uap-skills-" + uuid.uuid4().hex
        payload = (json.dumps({"id": nonce, "type": "get_commands"}, separators=(",", ":")) + "\n").encode()
        native_env = {key: self.env[key] for key in ("PATH", "HOME", "USERPROFILE", "SYSTEMROOT", "WINDIR", "TEMP", "TMP", "TMPDIR", "LANG", "LC_ALL", "XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_STATE_HOME", "XDG_DATA_HOME") if key in self.env}
        native_env.update(PI_CODING_AGENT_DIR=str(self.profile / "agent"), PI_OFFLINE="1")
        argv = [self.binary, "--mode", "rpc", "--no-session", "--offline"]
        def response_frame(body):
            value = json.loads(body.decode("utf-8"), object_pairs_hook=self.api["_unique_json_object"],
                               parse_constant=lambda _: require(False))
            require(type(value) is dict and value.get("id") == nonce and value.get("type") == "response"
                    and value.get("command") == "get_commands" and value.get("success") is True)
            return True
        record, stdout, stderr = self.api["COPILOT_CATALOG_PROBE"](
            argv, self.project, native_env, self.case, "pi-" + phase,
            timeout=30, limit=1 << 20, memory_only=True, input_bytes=payload, input_complete=response_frame)
        captured = {**record, "stdin_base64": base64.b64encode(payload).decode(),
            "stdout_base64": base64.b64encode(stdout).decode(), "stderr_base64": base64.b64encode(stderr).decode(),
            "requested_scope": "project", "expected_selected_present": not removed,
            "vendor_authentication": False, "model_calls": False}
        self.evidence["native_catalogs"][phase] = captured
        require(record.get("error") is None and type(record.get("exit")) is int and record["exit"] == 0
                and record.get("stdin_written") is True and record.get("stdin_response_confirmed") is True
                and record.get("argv") == argv
                and record.get("cwd") == str(self.project)
                and record.get("stdin_bytes") == len(payload)
                and record.get("stdin_sha256") == hashlib.sha256(payload).hexdigest()
                and stderr == b"" and 0 < len(stdout) <= 1 << 20)
        for label, body in (("stdout", stdout), ("stderr", stderr)):
            require(record.get(label + "_bytes") == len(body) and
                    record.get(label + "_sha256") == hashlib.sha256(body).hexdigest())
        require(stdout.endswith(b"\n") and len(stdout.splitlines()) == 1)
        response = json.loads(stdout.decode("utf-8"), object_pairs_hook=self.api["_unique_json_object"],
                              parse_constant=lambda _: require(False))
        require(type(response) is dict and response.get("id") == nonce and response.get("type") == "response"
                and response.get("command") == "get_commands" and response.get("success") is True)
        commands = response["data"]["commands"]
        require(type(commands) is list and len(commands) <= 512
                and all(type(row) is dict and type(row.get("name")) is str and row["name"] for row in commands))
        selected = [row for row in commands if row["name"].casefold() == ("skill:" + self.item["name"]).casefold()]
        require(len(selected) == (0 if removed else 1))
        if not removed:
            row = selected[0]
            require(row["name"] == "skill:" + self.item["name"] and row.get("source") == "skill"
                    and type(row.get("description")) is str and row["description"] == self.description)
            source = row["sourceInfo"]
            require(type(source) is dict)
            require(all(source.get(key) == value for key, value in {
                "path": str(Path(self.owner["native_path"]) / "SKILL.md"), "source": "auto",
                "scope": "project", "origin": "top-level", "baseDir": str(self.native_root)}.items())
                and "changelogPath" not in source)
            require(self.evidence.setdefault("observed_description", row["description"]) == row["description"])
        captured["selected_present"] = not removed
        self.guard()

def source_description(capture, item, project, cli):
    """One exact selected document, not a complete repository scan."""
    require(type(capture) is dict and capture.get("argv") == [cli, "skills", "list", item["source"],
        "--path", "skills/" + item["folder"], "--scope", "project", "--project-root", str(project), "--format", "json"]
        and capture.get("cwd") == str(project)
        and capture.get("qualification") == "exact-selected-source-document; nested Skills not enumerated")
    output = capture["result"]
    require(type(output.get("exit_code")) is int and output["exit_code"] == 0 and output.get("timeout") is False)
    streams = {}
    for label in ("stdout", "stderr"):
        body = base64.b64decode(capture[label + "_base64"], validate=True)
        require(len(body) <= 1 << 20 and type(capture.get(label + "_bytes")) is int
                and capture[label + "_bytes"] == len(body)
                and capture.get(label + "_sha256") == hashlib.sha256(body).hexdigest())
        streams[label] = body
    require(streams["stderr"] == b"")
    def pairs(values):
        document = {}
        for key, value in values:
            require(key not in document)
            document[key] = value
        return document
    document = json.loads(streams["stdout"].decode("utf-8"), object_pairs_hook=pairs, parse_constant=lambda _: require(False))
    require(type(document) is dict and type(document.get("schema_version")) is int and document["schema_version"] == 1
            and document.get("command") == "skills list" and document.get("result") == "success")
    data = document["data"]
    require(type(data) is dict and data.get("scope") == "project" and data.get("scope_selection_reason") == "explicit"
            and data.get("source") == item["repo"] and data.get("revision") == item["ref"]
            and data.get("source_path") == "skills/" + item["folder"] and data.get("complete") is False
            and data.get("diagnostics") == [{"code": "exact_skill_selection", "severity": "warning",
                "message": "Catalog scan is incomplete; explicitly scan a source folder to discover nested Skills"}])
    candidates = data["candidates"]
    require(type(candidates) is list and len(candidates) == 1 and type(candidates[0]) is dict)
    candidate = candidates[0]
    require(candidate.get("name") == item["name"] and candidate.get("path") == "skills/" + item["folder"]
            and candidate.get("status") == "available" and candidate.get("diagnostic_codes") in (None, [])
            and type(candidate.get("description")) is str and bool(candidate["description"]))
    return document, candidate["description"]


def summary(report, fixtures, acceptance):
    """Two disjoint full-source cohorts; Prepared never contributes native PASS."""
    result = {"pi_cohort_verified": 0, "pi_cohort_acceptance_passed": False}
    mode = {FOREIGN_ACCEPTANCE: "foreign-prepared", NATIVE_ACCEPTANCE: "clean-native"}.get(acceptance)
    if (mode is None or report.get("pi_skill_cohort") != mode
        or (report.get("target"), report.get("kind"), report.get("platform"), report.get("architecture"), report.get("canonical_client_id")) != ("pi", "skill", "linux", "x86_64", "pi")
        or not pinned_version(report.get("client_version"), "1.5.0")
        or not pinned_version(report.get("cli_version"), "agentplugins 0.1.115")
        or any(type(report.get(key)) is not int or report[key] != value for key, value in {
            "fixture_count": 30, "cases_attempted": 30, "passed": 30 if mode == "clean-native" else 0,
            "prepared_lifecycle_passed": 0 if mode == "clean-native" else 30,
            "verified_negative_coverage": 0}.items())
        or report.get("full_coverage") is not True or report.get("source_selection_complete") is not True):
        return result
    rows, sources = report.get("results"), fixtures.get("skills")
    if type(rows) is not list or len(rows) != 30 or type(sources) is not list or len(sources) != 30:
        return result
    seen = set()
    for index, (row, item) in enumerate(zip(rows, sources)):
        if (type(row) is not dict or type(row.get("index")) is not int or row["index"] != index
            or (row.get("target"), row.get("kind"), row.get("identity"), row.get("source")) != ("pi", "skill", item["name"], item["source"])
            or row.get("status") != ("passed" if mode == "clean-native" else "prepared-lifecycle-passed")
            or type(row.get("public_skill_ready")) is not dict
            or set(row["public_skill_ready"]) != {"after-add", "repeat-add"}
            or any(value is not (mode == "clean-native") for value in row["public_skill_ready"].values())
            or any(row.get(key) is not True for key in ("listed_identity", "lock_exact_source_pinned", "skill_bytes_match", "repeat_verified", "repeat_lock_unchanged", "repeat_native_unchanged", "removal_verified", "post_list_schema_valid", "sentinels_preserved"))
            or any(row.get(key) is not False for key in ("failed_install_registered", "failed_install_residue", "post_remove_identity_registered", "post_lock_pinned"))
            or row.get("post_removal_skill") is not None or row.get("post_removal_leftovers") != []
            or row.get("post_skill_tree_delta") != [] or row.get("native_residuals") != []):
            return result
        proof = row.get("pi_project_skill_proof")
        if (type(proof) is not dict or type(proof.get("schema_version")) is not int or proof["schema_version"] != 1
            or proof.get("qualification") != mode or proof.get("native_ready") is not (mode == "clean-native")
            or proof.get("status") != ("native-lifecycle-verified" if mode == "clean-native" else "prepared-lifecycle-verified")
            or any(proof.get(key) is not False for key in ("producer_attribution", "writer_closure", "vendor_authentication", "model_calls"))
            or set(proof.get("checkpoints", {})) != {"after-add", "repeat-add", "after-remove"}
            or proof.get("skill_git_blob_sha1") != item["skill_md_git_blob_sha1"]):
            return result
        images = [proof.get("before"), *(proof["checkpoints"][phase] for phase in ("after-add", "repeat-add", "after-remove"))]
        for image in images:
            if type(image) is not dict or set(image) != {"native", "home"}:
                return result
            for captured in image.values():
                if (type(captured) is not dict or type(captured.get("snapshot")) is not dict
                    or type(captured.get("entries")) is not int or captured["entries"] != len(captured["snapshot"])
                    or not 1 <= captured["entries"] <= 10000
                    or captured.get("snapshot_sha256") != hashlib.sha256(json.dumps(captured["snapshot"], sort_keys=True).encode()).hexdigest()):
                    return result
        if images[1] != images[2]:
            return result
        owner, trust = proof.get("owner", {}), proof.get("trust", {})
        iid = owner.get("installation_id")
        if (type(iid) is not str or iid in seen or row.get("matched_installation_ids") != [iid]
            or row.get("repeat_installation_ids") != [iid]
            or row.get("skill_path") != ".pi/skills/" + item["name"] + "/SKILL.md"
            or owner.get("source", {}).get("resolved_revision") != item["ref"]
            or trust.get("saved_trust_absent") is not True or trust.get("settings_override_absent") is not True
            or trust.get("foreign_blocker") != (".pi/skills/foreign-neighbor is not managed by UAP" if mode == "foreign-prepared" else None)):
            return result
        seen.add(iid)
        facts = {**FACTS, **({"activation": "active", "authentication": "not_required"} if mode == "clean-native" else {})}
        observations = row.get("public_target_states")
        if type(observations) is not dict:
            return result
        for phase in ("after-add", "repeat-add", "after-remove"):
            observation = observations.get(phase)
            expected = [] if phase == "after-remove" else [facts]
            if (type(observation) is not dict or observation.get("status") != "observed"
                or observation.get("truncated") is not False or observation.get("states") != expected
                or any(type(observation.get(key)) is not int or observation[key] != value for key, value in {
                    "invalid_rows": 0, "selected_installations": len(expected), "selected_bindings": len(expected)}.items())):
                return result
        metadata = proof.get("source_metadata")
        if mode == "clean-native":
            try:
                document, description = source_description(metadata, item, Path(owner["active"]).parent.parent.parent, report["cli"])
                if (metadata.get("owned_resources_unchanged") is not True
                    or metadata.get("skill_git_blob_sha1") != item["skill_md_git_blob_sha1"]
                    or proof.get("observed_description") != description or metadata.get("document") != document):
                    return result
            except (ValueError, KeyError, TypeError, UnicodeError):
                return result
        elif metadata is not None:
            return result
        catalogs = proof.get("native_catalogs")
        if type(catalogs) is not dict or set(catalogs) != ({"after-add", "repeat-add", "after-remove"} if mode == "clean-native" else set()):
            return result
        for phase, capture in catalogs.items():
            if (type(capture) is not dict or capture.get("error") is not None
                or type(capture.get("exit")) is not int or capture["exit"] != 0
                or capture.get("stdin_written") is not True or capture.get("stdin_response_confirmed") is not True
                or capture.get("selected_present") is not (phase != "after-remove")):
                return result
            for stream in ("stdin", "stdout", "stderr"):
                try:
                    body = base64.b64decode(capture[stream + "_base64"], validate=True)
                except (KeyError, ValueError, TypeError):
                    return result
                if (type(capture.get(stream + "_bytes")) is not int
                    or capture[stream + "_bytes"] != len(body)
                    or capture.get(stream + "_sha256") != hashlib.sha256(body).hexdigest()):
                    return result
        for phase in ("install", "list", "repeat_add", "repeat_list", "cleanup", "post_remove_list"):
            command = row.get(phase)
            if type(command) is not dict or type(command.get("exit_code")) is not int or command["exit_code"] != 0 or command.get("timeout") is not False:
                return result
            try:
                if type(json.loads(command["stdout_tail"])) is not dict:
                    return result
            except (ValueError, KeyError, TypeError):
                return result
        result["pi_cohort_verified"] += 1
    result["pi_cohort_acceptance_passed"] = result["pi_cohort_verified"] == 30
    return result
