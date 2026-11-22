"""Inert public transports and real filesystem receipts, never native proof."""
import base64
import copy
import hashlib
from io import StringIO
import json
import os
from pathlib import Path
import platform
import re
from runpy import run_path
import shutil
import sys
from tempfile import TemporaryDirectory
import unittest
from unittest.mock import patch
import uuid

import pi_qoder_installation_audit as installation

ROOT = Path(__file__).resolve().parents[1]
CLI = "/unit/agentplugins"
PHASES = ("install", "list", "repeat_add", "repeat_list", "cleanup", "post_remove_list")
MANUAL = ("Reload resources or restart Pi to discover user Skills and MCP; installed files "
          "do not prove an authenticated connection or model tool call.")


def digest(body):
    return hashlib.sha256(body).hexdigest()


def packed(value):
    return json.dumps(value, ensure_ascii=False, separators=(",", ":"), allow_nan=False).encode()


def write(path, body):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_bytes(body)
    path.chmod(0o600)


def atomic(path, body):
    pending = path.with_name(path.name + ".unit-pending")
    write(pending, body)
    os.replace(pending, path)


def members(files):
    directories = set()
    for name in files:
        directories.update(p.as_posix() for p in Path(name).parents if p.as_posix() != ".")
    return directories


def package_digest(files):
    """Independent fixture producer for packagesnapshot's fixed framing."""
    directories, stream = members(files), bytearray()
    for name in sorted(directories | set(files)):
        body = files.get(name, b"")
        stream += ("dir" if name in directories else "file").encode() + b"\0" + name.encode()
        stream += b"\0false\0" + str(len(body)).encode() + b"\0" + body
    return "sha256:" + digest(stream)


def skill_digest(files, *, trust=False):
    """Skill WalkDir and Pi trust formats are different production contracts."""
    directories, stream = members(files), bytearray()
    for name in sorted(directories | set(files), key=lambda p: tuple(p.split("/"))):
        stream += ("d" if name in directories else "f").encode() + b"\0" + name.encode() + b"\0"
        if name in files:
            stream += (digest(files[name]).encode() if trust else b"-\0" + files[name]) + b"\0"
    return "sha256:" + digest(stream)


def skill_body(name):
    return ("---\nname: " + name + "\ndescription: Inert installation receipt fixture\n---\n"
            "This file is never submitted to a native client.\n").encode()


def hook_bytes():
    source = (ROOT / "install/integrationctl/agentplugins/clients/pi/project_trust_hook.go").read_text()
    marker = json.loads(re.search(r"const piTrustHookMarker = (\"[^\n]+\")", source)[1])
    body = re.search(r"const piTrustHookSource = piTrustHookMarker \+ `([\s\S]*)`", source)[1]
    value = (marker + body).encode()
    if len(value) != 9078 or digest(value) != "eb4fa44cb880e1df05e42356f7e4bef9efef4abad82495cdfc87867b76b90a73":
        raise AssertionError("inert fixture must retain the exact production hook; it is never executed")
    return value


class InertLifecycle:
    """Author synthetic UAP outputs; all admission and custody methods are real."""
    def __init__(self, audit, fixtures, target, kind, index, *, acceptance=None):
        self.audit, self.fixtures = audit, copy.deepcopy(fixtures)
        self.target, self.kind, self.index = target, kind, index
        self.acceptance = acceptance
        self.clean_pi = acceptance == "pi-clean-native-skills"
        self.item = self.fixtures["skills" if kind == "skill" else "plugin_aliases"][index]
        self.name = self.item["name"] if kind == "skill" else self.fixtures["plugin_declared_names"].get(self.item, self.item)
        self.source = self.item["source"] if kind == "skill" else self.item
        self.scope = "project" if kind == "skill" else "user"
        if kind == "skill":
            body = skill_body(self.name)
            self.item["skill_md_git_blob_sha1"] = hashlib.sha1(b"blob " + str(len(body)).encode() + b"\0" + body).hexdigest()
        self.pins = audit["effective_signed_release_pins"](self.fixtures, target)
        self.alias_sources = {self.name: self.source}
        if kind == "plugin" and self.name == "marketing-kit":
            for alias, distribution in audit["MARKETING_KIT_DISTRIBUTIONS"].items():
                if alias == self.name:
                    continue
                anchor = audit["MARKETING_KIT_SIGNED_CLOSURE"][distribution]
                self.pins[alias] = {"distribution": distribution, "sequence": anchor["release_sequence"],
                    "repository": anchor["repository"], "path": anchor["package_path"],
                    "revision": anchor["resolved_revision"], "tree_digest": anchor["tree_digest"],
                    "manifest_digest": anchor["manifest_digest"]}
                self.alias_sources[alias] = alias
        self.calls, self.outcomes, self.owners = [], {}, {}
        self.cli = CLI
        self.removed = False

    def bind(self, env, project):
        self.env, self.project = env, Path(project)
        self.home, self.data, self.case = Path(env["HOME"]), Path(env["AGENTPLUGINS_HOME"]), Path(project).parent
        self.lock_root = self.project if self.kind == "skill" else self.data
        self.state_root = self.project / ".uap" if self.kind == "skill" else self.data
        self.profile = ((self.project / ".pi" if self.kind == "skill" else self.home / ".pi/agent")
                        if self.target == "pi" else self.project / ".uap/managed/clients/qoder")
        self.native_skills = (self.project / ("." + self.target) if self.kind == "skill" else self.profile) / "skills"

    def prepare(self, root, *, admitted_pins=None, before_observer=None):
        case = root / (self.kind + "-" + str(self.index))
        for relative in ("home", "project", "uap-data", "tmp"):
            (case / relative).mkdir(parents=True, mode=0o700)
        env = self.audit["isolated_client_env"](case / "home", case, case / "uap-data")
        self.bind(env, case / "project")
        for path in (self.home / "foreign-untouched.txt", self.project / "foreign-untouched.txt",
                     self.home / ("." + self.target) / "foreign-neighbor.txt"):
            write(path, b"inert independent foreign content\n")
        if self.clean_pi:
            self.native_skills.mkdir(parents=True, mode=0o700)
            (self.profile / "extensions").mkdir(mode=0o700)
        elif self.kind == "skill":
            write(self.native_skills / "foreign-neighbor/SKILL.md", skill_body("foreign-neighbor"))
        if before_observer:
            before_observer(self)
        return installation.PiQoderInstallationAudit(CLI, env, self.project, case, self.audit,
            target=self.target, kind=self.kind, item=self.item, index=self.index,
            fixtures=self.fixtures, signed_release_pins=self.pins if admitted_pins is None else admitted_pins,
            acceptance=self.acceptance)

    def selected_source(self, alias):
        source = self.alias_sources[alias]
        if self.kind == "skill":
            repo, path, commit = self.item["repo"], "skills/" + self.item["folder"], self.item["ref"]
            return repo, path, commit, "github-skill:" + repo.lower() + ":" + self.name + ":" + path
        if source.startswith("github:"):
            repo, commit = source[7:].rsplit("@", 1)
            return repo.lower(), "", commit, "github:" + repo.lower() + ":"
        pin = self.pins[source]
        return pin["repository"], pin["path"], pin["revision"], "directory:" + pin["distribution"].lower()

    def author(self):
        records, locked, all_servers = [], [], {}
        for alias in self.alias_sources:
            repo, path, commit, key = self.selected_source(alias)
            uid = str(uuid.uuid5(uuid.NAMESPACE_URL, "inert/" + self.target + "/" + self.kind + "/" + alias))
            name = "skill-" + digest(key.encode())[:56] if self.kind == "skill" else alias
            physical = name[:51].rstrip(".-") + "-" + digest(uid.encode())[:12]
            active = self.profile / ("extensions" if self.target == "pi" else "") / physical
            binding_id = "client_" + digest("\0".join((uid, self.target, self.scope, str(active))).encode())[:24]
            components = ([("skill", self.name)] if self.kind == "skill" else
                          [("skill", "campaign-planning")] if alias == "marketing-kit" else [("mcp_server", alias)])
            if alias == "uizze-ui-slop":
                components = [("skill", name) for name in ("anti-ui-slop", "image-to-ui", "ui-design", "ui-radar", "uizze-ui")]
            decisions = [{"kind": kind, "name": name, "support": "native"} for kind, name in components]
            if alias == "uizze-ui-slop":
                decisions.append({"kind": "extension", "name": "com.github.awesome-copilot", "support": "unsupported",
                                  "reason": "extension_semantics_not_established"})
            files = {"plugin.json": packed({"$schema": "https://plugins.sh/schema/1.0.0.json", "name": name}) + b"\n"}
            objects, native_dirs = [], []
            for kind, logical in components:
                receipt_kind = self.target + "_" + self.scope + ("_skill_directory" if kind == "skill" else "_mcp_server")
                obj = {"object_id": receipt_kind + ":" + logical, "kind": receipt_kind,
                       "logical_name": logical, "protection_class": "managed"}
                if kind == "skill":
                    content = {"SKILL.md": skill_body(logical), "references/note.txt": b"inert unchanged reference\n"}
                    native = self.native_skills / logical
                    for leaf, body in content.items():
                        files["skills/" + logical + "/" + leaf] = body
                        write(native / leaf, body)
                    obj.update(path=str(native), source_relative="skills/" + logical, managed_digest=skill_digest(content))
                    native_dirs.append(native)
                else:
                    native = {"url": "https://unit.invalid/" + logical}
                    files["mcp.json"] = packed({"mcpServers": {logical: {"type": "streamable-http", **native}}})
                    all_servers[logical] = native
                    framed = b"agentplugins-native-mcp-v1\0mcpServers\0" + logical.encode() + b"\0" + packed(native)
                    obj.update(path=str(self.profile / "mcp.json"), managed_digest="sha256:" + digest(framed))
                objects.append(obj)
            for leaf, body in files.items():
                write(active / leaf, body)
            managed = package_digest(files)
            objects.insert(0, {"object_id": "package:" + self.target + ":" + physical, "kind": "managed_package_directory",
                "logical_name": name, "path": str(active), "managed_digest": managed, "protection_class": "managed"})
            signed = self.kind == "plugin" and not self.alias_sources[alias].startswith("github:")
            pin = self.pins[self.alias_sources[alias]] if signed else None
            revision = {"resolved_revision": commit, "tree_digest": pin["tree_digest"] if pin else "sha256:" + digest(b"inert immutable tree/" + key.encode()),
                        "manifest_digest": pin["manifest_digest"] if pin else "sha256:" + digest(files["plugin.json"])}
            if signed:
                revision.update(distribution_id=pin["distribution"], release_sequence=pin["sequence"])
            is_active = self.clean_pi or self.kind == "plugin" and alias == self.name and self.index in (13, 22, 25, 29)
            state = {"materialization": "materialized", "activation": "active" if is_active else "prepared",
                "authentication": "not_required" if is_active else "not_checked", "policy": "allowed", "verification": "installation_verified"}
            binding = {"client_binding_id": binding_id, "client_id": self.target, "scope": self.scope,
                "target_locator": str(active), "physical_artifact_id": physical, "native_profile_root": str(self.profile),
                **state, "package_revision": revision, "affected_surfaces": [self.target], "native_objects": objects}
            record = {"installation_id": uid, "declared_name": name,
                "source": {"repository": repo, "package_subpath": path, "resolved_revision": commit, "tree_digest": revision["tree_digest"]},
                "package": {"declared_name": name, "manifest_digest": revision["manifest_digest"],
                    "inventory": {field: [c["name"] for c in decisions if c["kind"] == kind] for kind, field in
                                  (("skill", "skills"), ("mcp_server", "mcp_servers"), ("extension", "extensions"))}},
                "clients": {binding_id: binding}}
            node = {"installation_id": uid, "name": name, "source": repo + ("/" + path if path else ""),
                "mixed_version": False, "clients": [{"client_id": self.target, "scope": self.scope, **state,
                    "package_revision": revision, "affected_surfaces": [self.target]}]}
            selector = {"kind": "release", "value": str(pin["sequence"])} if signed else {"kind": "commit", "value": commit}
            source = {"repository": repo, "path": path, "commit": commit, "requested": selector}
            if signed:
                source.update(distribution=pin["distribution"], distribution_id=pin["distribution"], release_sequence=pin["sequence"])
                record["directory"] = {"distribution_id": pin["distribution"], "desired_release_sequence": pin["sequence"]}
                node["directory"] = {"recorded_distribution": pin["distribution"], "recorded_release_sequence": pin["sequence"],
                    "recorded_revision": commit, "product_id": alias}
            elif self.kind == "skill":
                source.update(kind="skill", skill=self.name, resolved_skill_path=path, normalizer="agent-skill-package/v1",
                              raw_tree_digest="sha256:" + digest(b"inert raw selection/" + key.encode()))
            package = {"alias": alias, "source_key": key, "source": {"registry" if signed else "git": source},
                "digest_algorithm": "agentplugins-tree-sha256-v1", "tree_digest": revision["tree_digest"],
                "manifest_digest": revision["manifest_digest"], "targets": [self.target]}
            if alias != self.name:
                package["required_by"] = [{"alias": self.name, "selector": selector}]
            locked.append(package)
            records.append(record)
            self.owners[alias] = {"uuid": uid, "name": name, "physical": physical, "active": active, "binding_id": binding_id,
                "components": components, "decisions": decisions, "record": record, "node": node, "revision": revision,
                "files": files, "native_dirs": native_dirs}
        if all_servers:
            write(self.profile / "mcp.json", packed({"mcpServers": all_servers}))
            write(self.profile / "mcp.json.agentplugins.lock", b"")
        if self.target == "pi" and self.kind == "skill":
            owner = self.owners[self.name]
            native_files = {p: body for p, body in {"SKILL.md": skill_body(self.name), "references/note.txt": b"inert unchanged reference\n"}.items()}
            hook = self.home / ".pi/agent/extensions/agentplugins-project-trust"
            if self.clean_pi:
                for directory in (self.home / ".pi/agent", hook.parent, hook):
                    directory.mkdir(mode=0o700)
            write(hook / "index.ts", hook_bytes())
            registry = {"schema_version": 1, "projects": {str(self.project): {owner["physical"]: {
                "extensions": {owner["physical"]: skill_digest(owner["files"], trust=True)},
                "skills": {self.name: skill_digest(native_files, trust=True)}, "mcp_servers": {}}}}}
            write(hook / "projects.json", packed(registry))
            for leaf in ("index.ts", "projects.json"):
                write(hook / (leaf + ".agentplugins.lock"), b"")
            if self.clean_pi:
                for name in ("auth.json", "models-store.json"):
                    write(self.home / ".pi/agent" / name, b"{}")
        self.state = {"schema_version": 5, "installations": records}
        self.lock = {"lock_version": 1, "manifest_digest": self.manifest(), "packages": locked}
        self.save_state()
        write(self.lock_root / "uap.lock", packed(self.lock))

    def manifest(self):
        repo, path, commit, key = self.selected_source(self.name)
        if self.kind == "skill" or self.source.startswith("github:"):
            source = {"git": "https://github.com/" + repo + ".git", "commit": commit}
            if self.kind == "skill":
                source.update(kind="skill", skill=self.name, path=path)
            selector = {"kind": "commit", "value": commit}
        else:
            pin = self.pins[self.source]
            source = {"registry": "uap", "distribution": pin["distribution"], "release": pin["sequence"]}
            selector = {"kind": "release", "value": str(pin["sequence"])}
        text = "schema_version = 1\n[dependencies." + json.dumps(self.name) + "]\n"
        text += "".join(k + " = " + json.dumps(v) + "\n" for k, v in source.items())
        text += "[targets]\n" + self.target + " = [" + json.dumps(self.name) + "]\n"
        write(self.lock_root / "uap.toml", text.encode())
        dep = {"alias": self.name, "source_key": key}
        if path and self.kind == "skill":
            dep["path"] = path
        dep["selector"] = selector
        return "sha256:" + digest(packed({"schema_version": 1, "dependencies": [dep],
            "targets": [{"id": self.target, "aliases": [self.name]}]}))

    def save_state(self):
        write(self.state_root / "state-v2.json", packed(self.state))

    def remove(self):
        for owner in self.owners.values():
            shutil.rmtree(owner["active"])
            for path in owner["native_dirs"]:
                shutil.rmtree(path)
        if (self.profile / "mcp.json").exists():
            atomic(self.profile / "mcp.json", packed({"mcpServers": {}}))
        if self.target == "pi" and self.kind == "skill":
            for leaf in ("index.ts", "projects.json"):
                (self.home / ".pi/agent/extensions/agentplugins-project-trust" / leaf).unlink()
        empty = "sha256:" + digest(b'{"schema_version":1,"dependencies":null,"targets":null}')
        atomic(self.lock_root / "uap.lock", packed({"lock_version": 1, "manifest_digest": empty, "packages": []}))
        atomic(self.lock_root / "uap.toml", b"schema_version = 1\n")
        self.state = {"schema_version": 5, "installations": []}
        self.save_state()
        self.removed = True

    def document(self, phase):
        if phase in ("list", "repeat_list", "post_remove_list"):
            return {"schema_version": 1, "command": "list", "result": "success",
                "data": {"installations": [] if phase == "post_remove_list" else [copy.deepcopy(o["node"]) for o in self.owners.values()]}}
        repeated, removed = phase == "repeat_add", phase == "cleanup"
        if self.kind == "skill" and not removed:
            owner = self.owners[self.name]
            candidate = "sha256:" + digest("\0".join((self.item["repo"], self.item["ref"], "skills/" + self.item["folder"])).encode())
            return {"schema_version": 1, "command": "skills add", "result": "success", "status": "succeeded",
                "scope": "project", "scope_selection_reason": "explicit", "repairInstructions": [],
                "readiness": "ready" if self.clean_pi else "needs_action",
                "items": [{"alias": self.name, "candidateId": candidate, "destinationPath": str(owner["active"]),
                           "targetId": self.target, "status": "unchanged" if repeated else "succeeded"}],
                "next_actions": [] if self.clean_pi else [{"code": "verify_native", "actor": "native_client", "message": "Verify discovery in a fresh selected client session."}]}
        actions, results = [], []
        for alias, owner in self.owners.items():
            revision = owner["revision"]
            action = {"alias": alias, "client_id": self.target, "action": "remove" if removed else "unchanged" if repeated else "install"}
            if removed or repeated:
                action.update(installation_id=owner["uuid"], binding_id=owner["binding_id"],
                              from_revision=revision["resolved_revision"], from_digest=revision["tree_digest"])
            if not removed:
                action.update(revision=revision["resolved_revision"], package_digest=revision["tree_digest"])
            if not (repeated or removed):
                action["component_support"] = copy.deepcopy(owner["decisions"])
            actions.append(action)
            if not repeated:
                result = {"installation_id": owner["uuid"], "client_id": self.target,
                          "operation": "remove" if removed else "install", "outcome": "completed"}
                if not removed and next(iter(owner["record"]["clients"].values()))["activation"] == "prepared":
                    result["manual_actions"] = [MANUAL]
                results.append(result)
        data = {"schema_version": 1, "command": "agentplugins " + ("remove" if removed else "add"),
            "result": "success", "outcome": "completed", "scope": self.scope, "scope_selection_reason": "explicit",
            "actions": actions, "results": results}
        if removed:
            return data
        data["environment"] = {"alias": self.name, "intent_change": not repeated, "saved": True, "scope": self.scope,
            "scope_selection_reason": "explicit", "selected_targets": [self.target],
            "lock_path": str(self.lock_root / "uap.lock"), "manifest_path": str(self.lock_root / "uap.toml")}
        return {"schema_version": 1, "command": "add", "result": "success", "data": data}

    def argv(self, phase):
        scope = ["--scope", "project", "--project-root", str(self.project)] if self.kind == "skill" else ["--scope", "user"]
        if phase in ("install", "repeat_add"):
            return ([self.cli, "skills", "add", self.source, "--skill", self.name, "--target", self.target, *scope, "--no-ui", "--yes", "--format", "json"]
                    if self.kind == "skill" else [self.cli, "add", self.source, "--target", self.target, *scope, "--format", "json"])
        if phase == "cleanup":
            return [self.cli, "remove", self.owners[self.name]["uuid"], "--target", self.target, *scope, "--format", "json"]
        return [self.cli, "list", "--target", self.target, *scope, "--format", "json"]

    def call(self, argv, env, cwd, timeout, **options):
        """Replace only the child transport; use real do_case/observer methods."""
        self.cli = argv[0]
        phase = PHASES[len(self.calls)]
        if not self.calls:
            self.bind(env, cwd)
            self.author()
        if phase == "cleanup":
            self.remove()
        if argv != self.argv(phase):
            raise AssertionError(("unexpected public argv", phase, argv, self.argv(phase)))
        self.calls.append(list(argv))
        result = self.outcome(self.document(phase))
        self.outcomes[phase] = copy.deepcopy(result)
        return result

    def outcome(self, document, *, code=0, stderr=""):
        stdout = packed(document).decode()
        return {"exit_code": code, "timeout": False, "stdout_full": stdout, "stderr_full": stderr,
            "stdout_tail": stdout[-1800:], "stderr_tail": stderr[-1800:], "_json": copy.deepcopy(document)}

    def refusal(self):
        # Production stager creates these exact empty parents before Pi rejects
        # this source's extension-discovery override and removes its staging.
        self.profile.mkdir(parents=True, mode=0o700)
        (self.profile / "extensions").mkdir(mode=0o700)
        reason = "pi package manifest overrides selected extension discovery"
        document = {"schema_version": 1, "command": "add", "result": "failure", "data": {
            "schema_version": 1, "command": "agentplugins add", "result": "failure", "outcome": "partial",
            "scope": "user", "scope_selection_reason": "explicit",
            "actions": [{"alias": self.name, "client_id": "pi", "action": "install",
                "component_support": [{"kind": "skill", "name": "writing-clearly-and-concisely", "support": "native"}],
                "revision": self.source.rsplit("@", 1)[1],
                "package_digest": "sha256:32fc89e62a200be082357d2cd5111284aa974b79678835cd60875de8fbba5958"}],
            "bindings": [{"alias": self.name, "client_id": "pi", "status": "incomplete", "code": "FAILED"}],
            "environment": {"alias": self.name, "saved": True, "intent_change": True, "scope": "user",
                "scope_selection_reason": "explicit", "selected_targets": ["pi"],
                "lock_path": str(self.lock_root / "uap.lock"), "manifest_path": str(self.lock_root / "uap.toml")},
            "results": [{"installation_id": "77193358-d6de-4c97-ab38-b9afe999342d", "operation": "install",
                         "outcome": "incomplete", "reason": reason}]}}
        stderr = "agentplugins: dependency sync stopped; inspect binding results and next actions: 1 of 1 bindings failed: " + reason + "\n"
        return self.outcome(document, code=1, stderr=stderr)

    def step(self, observer, phase, mutate=None):
        observer.guard()
        if phase == "install":
            self.author()
        elif phase == "cleanup":
            self.remove()
        outcome = self.outcome(self.document(phase))
        if mutate:
            mutate(self, outcome)
        argv = self.argv(phase)
        observer.observe(argv, outcome)
        self.outcomes[phase] = copy.deepcopy(outcome)
        observer.guard()

    def metadata(self, observer, mutate=None):
        """One inert form of main's existing source-document query, not a probe."""
        observer.guard()
        argv = [self.cli, "skills", "list", self.source, "--path", "skills/" + self.item["folder"],
                "--scope", "project", "--project-root", str(self.project), "--format", "json"]
        document = {"schema_version": 1, "command": "skills list", "result": "success", "data": {
            "scope": "project", "scope_selection_reason": "explicit", "source": self.item["repo"],
            "revision": self.item["ref"], "source_path": "skills/" + self.item["folder"], "complete": False,
            "diagnostics": [{"code": "exact_skill_selection", "severity": "warning",
                "message": "Catalog scan is incomplete; explicitly scan a source folder to discover nested Skills"}],
            "candidates": [{"name": self.name, "path": "skills/" + self.item["folder"], "status": "available",
                "description": "Inert installation receipt fixture", "diagnostic_codes": []}]}}
        outcome = self.outcome(document)
        if mutate:
            mutate(self, argv, outcome)
        observer.observe(argv, outcome)
        observer.guard()

    def row(self):
        uid = self.owners[self.name]["uuid"]
        row = {"index": self.index, "target": self.target, "kind": self.kind, "source": self.source, "identity": self.name,
            "status": ("passed" if self.clean_pi else "prepared-lifecycle-passed" if self.acceptance in installation.PROJECT_ACCEPTANCES
                       else "incomplete-native-proof" if self.kind == "skill" else "package-installed-native-unqualified"),
            **{key: True for key in ("listed_identity", "repeat_verified", "repeat_native_unchanged", "repeat_lock_unchanged",
                "removal_verified", "post_list_schema_valid", "sentinels_preserved")},
            **{key: False for key in ("repeat_native_runtime_only", "failed_install_registered", "failed_install_residue", "post_remove_identity_registered")},
            "native_residuals": [], "post_removal_leftovers": [], "matched_installation_ids": [uid], "repeat_installation_ids": [uid],
            **copy.deepcopy(self.outcomes)}
        if self.kind == "skill":
            row.update({key: True for key in ("lock_written", "lock_contains_identity", "lock_exact_source_pinned", "skill_bytes_match")})
            row.update(repeat_project_unchanged=None, post_removal_skill=None, post_lock_pinned=False,
                       public_skill_ready={"after-add": self.clean_pi, "repeat-add": self.clean_pi})
        return row


@unittest.skipUnless(sys.platform == "linux" and platform.machine() == "x86_64", "Explicit Linux x86_64 installation routes")
class PiQoderInstallationAuditTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.audit = run_path(str(ROOT / "scripts/live-install-audit.py"))
        cls.fixtures = json.loads((ROOT / "tests/live-install/fixtures.json").read_bytes())

    def exercise(self, target, kind, index, *, phase=None, mutate=None, row_mutation=None):
        with TemporaryDirectory(prefix="pi-qoder-installation-unit-") as temporary:
            fixture = InertLifecycle(self.audit, self.fixtures, target, kind, index)
            observer = fixture.prepare(Path(temporary).resolve())
            try:
                for current in PHASES:
                    fixture.step(observer, current, mutate if current == phase else None)
                row = fixture.row()
                if row_mutation:
                    row_mutation(row)
                row["pi_qoder_installation_audit"] = observer.finish(row)
                return row
            finally:
                observer.close()

    def exercise_main_project(self, acceptance, index=0, *, phase=None, mutate=None, row_mutation=None,
                              metadata_mutation=None, skip_metadata=False):
        target, kind = installation.PROJECT_ACCEPTANCES[acceptance]
        with TemporaryDirectory(prefix="pi-qoder-main-cohort-unit-") as temporary:
            fixture = InertLifecycle(self.audit, self.fixtures, target, kind, index, acceptance=acceptance)
            observer = fixture.prepare(Path(temporary).resolve())
            try:
                for current in PHASES:
                    fixture.step(observer, current, mutate if current == phase else None)
                    if current == "list" and fixture.clean_pi and not skip_metadata:
                        fixture.metadata(observer, metadata_mutation)
                row = fixture.row()
                if row_mutation:
                    row_mutation(row)
                row["pi_qoder_installation_audit"] = observer.finish(row)
                return row, fixture.fixtures
            finally:
                observer.close()

    def test_three_typed_routes_preserve_prepared_status_and_original_owner_cleanup(self):
        for target, kind, index in (("pi", "skill", 0), ("qoder", "skill", 0), ("pi", "plugin", 1)):
            with self.subTest(target=target, kind=kind):
                row = self.exercise(target, kind, index)
                proof = row["pi_qoder_installation_audit"]
                self.assertEqual(proof["status"], "verified", proof["errors"])
                self.assertTrue(proof["installation_qualified"])
                self.assertFalse(proof["exact_refusal_verified"])
                self.assertEqual([o["phase"] for o in proof["observations"]], list(PHASES))
                self.assertEqual(proof["observations"][4]["argv"][2], row["matched_installation_ids"][0])
                self.assertTrue(all(proof[key] is False for key in installation.QUALIFICATION))
                self.assertEqual(row["status"], "incomplete-native-proof" if kind == "skill" else "package-installed-native-unqualified")
                if target == "pi" and kind == "skill":
                    self.assertEqual(len(proof["coordination_files"]), 2)
                    self.assertTrue(all(fact["bytes"] == 0 for fact in proof["coordination_files"].values()))

    def test_plugin_closure_retains_prepared_children_and_omits_only_unsupported_extension(self):
        row = self.exercise("pi", "plugin", 13)
        proof = row["pi_qoder_installation_audit"]
        self.assertEqual(proof["status"], "verified", proof["errors"])
        self.assertEqual(set(proof["owners"]), {"marketing-kit", "notion", "hubspot-crm", "playwright"})
        self.assertEqual(proof["selected"]["marketing-kit"]["clients"][0]["activation"], "active")
        self.assertTrue(all(proof["selected"][alias]["clients"][0]["activation"] == "prepared"
                            for alias in ("notion", "hubspot-crm", "playwright")))
        def omit_child(fixture, outcome):
            fixture.state["installations"] = [r for r in fixture.state["installations"] if r["declared_name"] != "notion"]
            fixture.save_state()
        with self.assertRaisesRegex(ValueError, "saved owner closure differs"):
            self.exercise("pi", "plugin", 13, phase="install", mutate=omit_child)
        sanitized = self.exercise("pi", "plugin", 28)
        self.assertTrue(sanitized["pi_qoder_installation_audit"]["installation_qualified"])
        self.assertEqual(sanitized["status"], "package-installed-native-unqualified")
        def restore_unsupported(fixture, outcome):
            owner = fixture.owners[fixture.name]
            manifest = json.loads(owner["files"]["plugin.json"])
            manifest["extensions"] = {"com.github.awesome-copilot": {"skills": ["skills"]}}
            owner["files"]["plugin.json"] = packed(manifest)
            write(owner["active"] / "plugin.json", owner["files"]["plugin.json"])
            binding = next(iter(fixture.state["installations"][0]["clients"].values()))
            binding["native_objects"][0]["managed_digest"] = package_digest(owner["files"])
            fixture.save_state()
        with self.assertRaisesRegex(ValueError, "unsupported extension survived"):
            self.exercise("pi", "plugin", 28, phase="install", mutate=restore_unsupported)

    def test_source_uuid_revision_projection_and_foreign_changes_are_rejected(self):
        def change_source(fixture, outcome):
            fixture.state["installations"][0]["source"]["resolved_revision"] = "0" * 40
            fixture.save_state()
        def change_uuid(fixture, outcome):
            fixture.state["installations"][0]["installation_id"] = "55555555-5555-4555-8555-555555555555"
            fixture.save_state()
        def change_receipt(fixture, outcome):
            binding = next(iter(fixture.state["installations"][0]["clients"].values()))
            binding["native_objects"][0]["managed_digest"] = "sha256:" + "f" * 64
            fixture.save_state()
        def change_native(fixture, outcome):
            write(fixture.native_skills / fixture.name / "SKILL.md", b"different projected bytes\n")
        def change_foreign(fixture, outcome):
            write(fixture.project / "foreign-untouched.txt", b"foreign mutation\n")
        def create_mcp(fixture, outcome):
            write(fixture.project / ".mcp.json", b'{"mcpServers":{}}')
        def alias_mcp(fixture, outcome):
            (fixture.project / ".mcp.json").symlink_to(fixture.home / "foreign-untouched.txt")
        for mutate in (change_source, change_uuid, change_receipt, change_native, change_foreign, create_mcp, alias_mcp):
            with self.subTest(mutation=mutate.__name__), self.assertRaises(ValueError):
                self.exercise("qoder", "skill", 0, phase="install", mutate=mutate)
        def change_mcp(fixture, outcome):
            write(fixture.profile / "mcp.json", packed({"mcpServers": {"context7": {"url": "https://unit.invalid/changed"}}}))
        def change_hook(fixture, outcome):
            path = fixture.home / ".pi/agent/extensions/agentplugins-project-trust/index.ts"
            write(path, path.read_bytes() + b"// changed owned hook\n")
        for kind, index, mutate in (("plugin", 1, change_mcp), ("skill", 0, change_hook)):
            with self.subTest(mutation=mutate.__name__), self.assertRaises(ValueError):
                self.exercise("pi", kind, index, phase="install", mutate=mutate)

    def test_repeat_remint_replacement_and_removed_residue_are_rejected(self):
        def replace_native(fixture, outcome):
            path = fixture.native_skills / fixture.name / "SKILL.md"
            atomic(path, path.read_bytes())
        def remint_public(fixture, outcome):
            document = json.loads(outcome["stdout_full"])
            document["data"]["installations"][0]["installation_id"] = "55555555-5555-4555-8555-555555555555"
            outcome.update(fixture.outcome(document))
        def residue(fixture, outcome):
            write(fixture.native_skills / fixture.name / "SKILL.md", skill_body(fixture.name))
        for phase, mutate in (("repeat_add", replace_native), ("repeat_list", remint_public), ("cleanup", residue)):
            with self.subTest(phase=phase), self.assertRaises(ValueError):
                self.exercise("qoder", "skill", 0, phase=phase, mutate=mutate)

    def test_malformed_full_public_envelopes_and_missing_phase_never_qualify(self):
        mutations = {
            "missing full stdout": lambda f, o: o.pop("stdout_full"),
            "tail cannot replace full": lambda f, o: o.update(stdout_full=o["stdout_full"][-30:]),
            "duplicate field": lambda f, o: o.update(stdout_full=o["stdout_full"].replace('"schema_version":1', '"schema_version":1,"schema_version":1', 1)),
            "bool schema": lambda f, o: o.update(stdout_full=o["stdout_full"].replace('"schema_version":1', '"schema_version":true', 1)),
            "nonfinite": lambda f, o: o.update(stdout_full=o["stdout_full"][:-1] + ',"bad":NaN}'),
            "bool exit": lambda f, o: o.update(exit_code=False),
            "stderr": lambda f, o: o.update(stderr_full="unexpected\n"),
            "timeout": lambda f, o: o.update(timeout=True),
        }
        for name, mutate in mutations.items():
            with self.subTest(name=name), self.assertRaises(installation.PiQoderInstallationError) as caught:
                self.exercise("qoder", "skill", 0, phase="install", mutate=mutate)
            evidence = caught.exception.audit_evidence["pi_qoder_installation_audit"]
            self.assertFalse(evidence["installation_qualified"])
            if name == "duplicate field":
                self.assertEqual(evidence["observations"][-1]["admission"], "unqualified")
                body = base64.b64decode(evidence["observations"][-1]["stdout_base64"])
                self.assertIn(b'"schema_version":1,"schema_version":1', body)
                self.assertEqual(digest(body), evidence["observations"][-1]["stdout_sha256"])
        row = self.exercise("qoder", "skill", 0, row_mutation=lambda r: r.update(repeat_add=None))
        self.assertFalse(row["pi_qoder_installation_audit"]["installation_qualified"])
        with TemporaryDirectory() as temporary:
            fixture = InertLifecycle(self.audit, self.fixtures, "qoder", "skill", 0)
            observer = fixture.prepare(Path(temporary).resolve())
            try:
                for phase in PHASES[:-1]:
                    fixture.step(observer, phase)
                proof = observer.finish(fixture.row())
                self.assertFalse(proof["installation_qualified"])
                self.assertEqual(len(proof["observations"]), 5)
            finally:
                observer.close()

    def test_context7_uses_effective_pi_signed_pin_not_global_or_coherent_wrong_release(self):
        row = self.exercise("pi", "plugin", 1)
        proof = row["pi_qoder_installation_audit"]
        self.assertEqual(row["identity"], "context7")
        self.assertEqual(proof["owners"]["context7"]["revision"]["distribution_id"], "777genius/context7")
        self.assertEqual(proof["owners"]["context7"]["revision"]["release_sequence"], 7)
        for mode in ("global", "coherent wrong pin"):
            with self.subTest(mode=mode), TemporaryDirectory() as temporary:
                fixture = InertLifecycle(self.audit, self.fixtures, "pi", "plugin", 1)
                admitted = copy.deepcopy(self.fixtures["signed_release_pins"] if mode == "global" else fixture.pins)
                if mode != "global":
                    admitted["context7"].update(revision="f" * 40, sequence=8, tree_digest="sha256:" + "e" * 64)
                observer = fixture.prepare(Path(temporary).resolve(), admitted_pins=admitted)
                try:
                    with self.assertRaisesRegex(ValueError, "portable source intent differs"):
                        fixture.step(observer, "install")
                finally:
                    observer.close()
    def test_only_exact_pi_refusal_with_empty_target_and_foreign_custody_is_accepted(self):
        for changed in (False, True):
            with self.subTest(changed=changed), TemporaryDirectory() as temporary:
                fixture = InertLifecycle(self.audit, self.fixtures, "pi", "plugin", 26)
                observer = fixture.prepare(Path(temporary).resolve())
                try:
                    failed, listed = fixture.refusal(), fixture.outcome(fixture.document("post_remove_list"))
                    if changed:
                        failed["stdout_full"] = failed["stdout_full"].replace("overrides selected extension discovery", "unavailable")
                        with self.assertRaises(installation.PiQoderInstallationError):
                            observer.observe(fixture.argv("install"), failed)
                        continue
                    observer.observe(fixture.argv("install"), failed)
                    observer.observe(fixture.argv("list"), listed)
                    row = {"index": 26, "target": "pi", "kind": "plugin", "source": fixture.source,
                        "identity": fixture.name, "status": "documented-or-source-unsupported", "install": failed, "list": listed,
                        "native_file_delta": 0, "native_identity_files": [], "native_residuals": [], "sentinels_preserved": True,
                        **{key: False for key in ("listed_identity", "failed_install_registered", "failed_install_residue", "post_remove_identity_registered")}}
                    proof = observer.finish(row)
                    self.assertEqual(proof["exact_refusal_verified"], not changed, proof["errors"])
                    self.assertFalse(proof["installation_qualified"])
                    self.assertTrue(all(proof[key] is False for key in installation.QUALIFICATION))
                finally:
                    observer.close()
        with TemporaryDirectory() as temporary:
            fixture = InertLifecycle(self.audit, self.fixtures, "pi", "plugin", 26)
            observer = fixture.prepare(Path(temporary).resolve())
            try:
                failed = fixture.refusal()
                write(fixture.profile / "extensions/unowned-leftover.txt", b"not part of the exact empty-parent refusal\n")
                with self.assertRaises(installation.PiQoderInstallationError):
                    observer.observe(fixture.argv("install"), failed)
            finally:
                observer.close()

    def test_real_do_case_captures_six_full_envelopes_and_keeps_native_status_unqualified(self):
        modes = (("pi", "skill", 0, "pi-skills-installation"),
                 ("qoder", "skill", 0, "qoder-skills-installation"),
                 ("pi", "plugin", 1, "pi-installation"))
        for target, kind, index, mode in modes:
            with self.subTest(route=mode), TemporaryDirectory() as temporary:
                fixture = InertLifecycle(self.audit, self.fixtures, target, kind, index)
                globals_ = self.audit["do_case"].__globals__
                with patch.dict(globals_, {"call": fixture.call, "AGENT_BINARIES": {}}), \
                        patch("subprocess.Popen", side_effect=AssertionError("no child is allowed in this inert test")) as child:
                    row = self.audit["do_case"](CLI, target, kind, fixture.item, index, Path(temporary).resolve(),
                        fixture.fixtures["plugin_declared_names"], fixture.pins,
                        installation_audit=mode, installation_fixtures=fixture.fixtures)
                child.assert_not_called()
                proof = row["pi_qoder_installation_audit"]
                self.assertEqual(proof["status"], "verified", {"errors": proof["errors"], "row": row})
                self.assertEqual(len(fixture.calls), 6)
                self.assertEqual(row["status"], "incomplete-native-proof" if kind == "skill" else "package-installed-native-unqualified")
                self.assertFalse(any(proof[key] for key in installation.QUALIFICATION))
                self.assertEqual(row["cleanup"]["exit_code"], 0)
                self.assertEqual(fixture.calls[4][2], row["matched_installation_ids"][0])
                for phase, capture in zip(PHASES, proof["observations"]):
                    expected = fixture.outcomes[phase]["stdout_full"].encode()
                    self.assertEqual(capture["stdout_sha256"], digest(expected))
                    self.assertEqual(capture["stdout_bytes"], len(expected))

    def test_real_main_accepts_complete_installation_receipts_without_creating_native_pass(self):
        with TemporaryDirectory(prefix="pi-qoder-main-unit-") as temporary:
            root = Path(temporary).resolve()
            fixtures = copy.deepcopy(self.fixtures)
            for item in fixtures["skills"]:
                body = skill_body(item["name"])
                item["skill_md_git_blob_sha1"] = hashlib.sha1(b"blob " + str(len(body)).encode() + b"\0" + body).hexdigest()
            fixture_path = root / "inert-fixtures.json"
            write(fixture_path, packed(fixtures))
            cli, vendor = root / "agentplugins", root / "qoder"
            write(cli, b"inert public command transport, never executed\n")
            write(vendor, b"not a vendor executable, native diagnostic must refuse\n")
            cases = {}
            corrupt = False
            failed_bytes = None

            def transport(argv, env, cwd, timeout, **options):
                nonlocal failed_bytes
                if argv == [str(cli), "version"] or argv == [str(vendor), "--version"]:
                    version = "agentplugins 0.1.115" if argv[0] == str(cli) else "1.1.81"
                    return {"exit_code": 0, "timeout": False, "stdout_tail": version, "stderr_tail": ""}
                index = int(Path(cwd).parent.name.rsplit("-", 1)[1])
                if index not in cases:
                    cases[index] = InertLifecycle(self.audit, fixtures, "qoder", "skill", index)
                result = cases[index].call(argv, env, cwd, timeout, **options)
                if corrupt and index == 0 and len(cases[index].calls) == 1:
                    failed_bytes = result["stdout_full"].replace('"schema_version":1', '"schema_version":1,"schema_version":1', 1)
                    result["stdout_full"] = failed_bytes
                return result

            arguments = ["audit", "--cli", str(cli), "--target", "qoder", "--kind", "skill",
                         "--limit", "30", "--report-dir", str(root / "report"), "--acceptance", "qoder-skills-installation"]
            globals_ = self.audit["main"].__globals__
            with patch.dict(globals_, {"FIXTURES": fixture_path, "AGENT_BINARIES": {"qoder": str(vendor)}, "call": transport}), \
                    patch.dict(os.environ, {"AUDIT_VENDOR_OUTCOME": "success"}), \
                    patch.object(sys, "argv", arguments), patch("sys.stdout", new=StringIO()), \
                    patch("subprocess.Popen", side_effect=AssertionError("no child is allowed in this inert test")) as child:
                code = self.audit["main"]()
            child.assert_not_called()
            report = json.loads((root / "report/qoder-skill.json").read_bytes())
            self.assertEqual(code, 0, [(r["index"], r.get("error"), r.get("pi_qoder_installation_audit", {}).get("errors")) for r in report["results"]])
            self.assertEqual(report["installation_passed"], 30)
            self.assertEqual(report["installation_verified_negative_coverage"], 0)
            self.assertTrue(report["installation_acceptance_passed"])
            self.assertEqual(report["passed"], 0)
            self.assertEqual(report["verified_negative_coverage"], 0)
            self.assertEqual({r["status"] for r in report["results"]}, {"incomplete-native-proof"})
            self.assertTrue(all(len(f.calls) == 6 for f in cases.values()))
            self.assertTrue(all(report[key] is False for key in installation.QUALIFICATION))
            for name, mutate in (
                    ("missing receipt", lambda r: r["results"][0].pop("pi_qoder_installation_audit")),
                    ("missing phase", lambda r: r["results"][0]["pi_qoder_installation_audit"]["observations"].pop()),
                    ("changed source", lambda r: r["results"][0].update(source="https://unit.invalid/foreign")),
                    ("failed removal", lambda r: r["results"][0].update(removal_verified=False)),
                    ("native relabel", lambda r: r.update(passed=30)),
                    ("native claim", lambda r: r["results"][0]["pi_qoder_installation_audit"].update(native_discovery_qualified=True)),
                    ("bool count", lambda r: r.update(cases_attempted=True))):
                with self.subTest(mutation=name):
                    changed = copy.deepcopy(report)
                    mutate(changed)
                    result = installation.summary(changed, fixtures, "qoder-skills-installation", signed_release_pins=fixtures["signed_release_pins"])
                    self.assertFalse(result["installation_acceptance_passed"])
            # Repeat only the same inert public transport with one malformed
            # complete first envelope. Actual main must retain the typed error
            # capture, never turn it into a successful installation row.
            corrupt = True
            cases.clear()
            arguments[arguments.index("--report-dir") + 1] = str(root / "error-report")
            with patch.dict(globals_, {"FIXTURES": fixture_path, "AGENT_BINARIES": {"qoder": str(vendor)}, "call": transport}), \
                    patch.dict(os.environ, {"AUDIT_VENDOR_OUTCOME": "success"}), \
                    patch.object(sys, "argv", arguments), patch("sys.stdout", new=StringIO()), \
                    patch("subprocess.Popen", side_effect=AssertionError("no child is allowed in this inert test")) as child:
                self.assertEqual(self.audit["main"](), 1)
            child.assert_not_called()
            failed_report = json.loads((root / "error-report/qoder-skill.json").read_bytes())
            failed_row = failed_report["results"][0]
            self.assertEqual(failed_row["status"], "harness-error")
            proof = failed_row["pi_qoder_installation_audit"]
            self.assertFalse(proof["installation_qualified"])
            self.assertFalse(failed_report["installation_acceptance_passed"])
            self.assertEqual(failed_report["passed"], 0)
            capture = proof["observations"][0]
            self.assertEqual(capture["admission"], "unqualified")
            self.assertEqual(base64.b64decode(capture["stdout_base64"]), failed_bytes.encode())
            self.assertEqual(capture["stdout_sha256"], digest(failed_bytes.encode()))


    def test_main_project_modes_keep_one_six_phase_lifecycle_and_installation_only_proof(self):
        for acceptance in installation.PROJECT_ACCEPTANCES:
            with self.subTest(acceptance=acceptance):
                row, _ = self.exercise_main_project(acceptance)
                proof = row["pi_qoder_installation_audit"]
                clean = acceptance == "pi-clean-native-skills"
                self.assertEqual(proof["status"], "verified", proof["errors"])
                self.assertTrue(proof["installation_qualified"])
                self.assertEqual(proof["acceptance"], acceptance)
                self.assertEqual(row["status"], "passed" if clean else "prepared-lifecycle-passed")
                self.assertEqual([o["phase"] for o in proof["observations"]], list(PHASES))
                self.assertEqual(proof["observations"][4]["argv"][2], row["matched_installation_ids"][0])
                self.assertTrue(all(proof[key] is False for key in installation.QUALIFICATION))
                if clean:
                    self.assertEqual(proof["source_metadata"]["phase"], "source_metadata")
                    self.assertEqual(proof["source_metadata"]["admission"], "verified")
                    self.assertEqual(len(proof["runtime_stores"]), 2)
                    self.assertTrue(all(f["bytes"] == 2 and f["sha256"] == digest(b"{}")
                                        for f in proof["runtime_stores"].values()))
                    self.assertEqual(len(proof["foreign_files"]), 3)
                    self.assertFalse(any("/skills/foreign-neighbor/SKILL.md" in path for path in proof["foreign_files"]))
                else:
                    self.assertIsNone(proof["source_metadata"])
                    self.assertEqual(proof["runtime_stores"], {})

    def test_main_project_ready_contract_is_closed_and_legacy_modes_are_unchanged(self):
        for acceptance in installation.PROJECT_ACCEPTANCES:
            def mismatch(fixture, outcome):
                document = fixture.document("install")
                document["readiness"] = "needs_action" if fixture.clean_pi else "ready"
                outcome.update(fixture.outcome(document))
            with self.subTest(acceptance=acceptance), self.assertRaises(ValueError):
                self.exercise_main_project(acceptance, phase="install", mutate=mismatch)
            row, _ = self.exercise_main_project(acceptance, row_mutation=lambda r: r.update(status="incomplete-native-proof"))
            self.assertFalse(row["pi_qoder_installation_audit"]["installation_qualified"])
        def prepared_state(fixture, _outcome):
            binding = next(iter(fixture.state["installations"][0]["clients"].values()))
            binding.update(activation="prepared", authentication="not_checked")
            fixture.save_state()
        with self.assertRaises(ValueError):
            self.exercise_main_project("pi-clean-native-skills", phase="install", mutate=prepared_state)
        self.assertEqual({key: installation.ROUTES[key] for key in (
            "pi-installation", "pi-skills-installation", "qoder-skills-installation")},
            {"pi-installation": ("pi", "plugin"), "pi-skills-installation": ("pi", "skill"),
             "qoder-skills-installation": ("qoder", "skill")})

    def test_clean_pi_runtime_stores_require_controlled_appearance_and_frozen_identity(self):
        def preexisting(kind):
            def create(fixture):
                path = (fixture.native_skills / "foreign-neighbor/SKILL.md" if kind == "foreign" else
                        fixture.home / ".pi/agent/auth.json")
                write(path, b"{}")
            return create
        for kind in ("foreign", "runtime"):
            with self.subTest(preexisting=kind), TemporaryDirectory() as temporary:
                fixture = InertLifecycle(self.audit, self.fixtures, "pi", "skill", 0,
                                         acceptance="pi-clean-native-skills")
                with self.assertRaises(ValueError):
                    fixture.prepare(Path(temporary).resolve(), before_observer=preexisting(kind))
        for phase, fault in (("install", "nonempty"), ("install", "mode"), ("install", "symlink"),
                             ("install", "cache"), ("repeat_list", "replace"), ("cleanup", "deleted")):
            def mutate(fixture, _outcome, fault=fault):
                path = fixture.home / ".pi/agent/auth.json"
                if fault == "nonempty": write(path, b'{"provider":"inert"}')
                elif fault == "mode": path.chmod(0o644)
                elif fault == "symlink":
                    path.unlink()
                    path.symlink_to("models-store.json")
                elif fault == "cache": write(path.parent / "unowned-cache", b"inert")
                elif fault == "replace": atomic(path, b"{}")
                else: path.unlink()
            with self.subTest(phase=phase, fault=fault), self.assertRaises((ValueError, OSError)):
                self.exercise_main_project("pi-clean-native-skills", phase=phase, mutate=mutate)
        def cold_store(fixture, _outcome):
            write(fixture.home / ".pi/agent/auth.json", b"{}")
            write(fixture.home / ".pi/agent/models-store.json", b"{}")
        with self.assertRaises(ValueError):
            self.exercise_main_project("pi-foreign-prepared-skills-installation", phase="install", mutate=cold_store)

    def test_clean_pi_metadata_is_one_exact_read_only_capture_not_a_seventh_lifecycle_phase(self):
        for fault in ("argv", "revision", "schema-bool", "complete", "candidate", "duplicate-json",
                      "stderr", "tmp-write", "same-byte-foreign-replacement"):
            def mutate(fixture, argv, outcome, fault=fault):
                document = copy.deepcopy(outcome["_json"])
                if fault == "argv": argv[3] = "github:other/source@" + "a" * 40
                elif fault == "revision": document["data"]["revision"] = "a" * 40
                elif fault == "schema-bool": document["schema_version"] = True
                elif fault == "complete": document["data"]["complete"] = True
                elif fault == "candidate": document["data"]["candidates"][0]["name"] = "other-skill"
                elif fault == "duplicate-json":
                    outcome["stdout_full"] = outcome["stdout_full"].replace('{"schema_version":1', '{"schema_version":1,"schema_version":1', 1)
                    return
                elif fault == "stderr":
                    outcome["stderr_full"] = "inert diagnostic\n"
                    return
                elif fault == "tmp-write": write(fixture.case / "tmp/unowned", b"inert")
                elif fault == "same-byte-foreign-replacement":
                    path = fixture.home / "foreign-untouched.txt"
                    atomic(path, path.read_bytes())
                outcome.update(fixture.outcome(document))
            with self.subTest(fault=fault), self.assertRaises(ValueError):
                self.exercise_main_project("pi-clean-native-skills", metadata_mutation=mutate)
        row, _ = self.exercise_main_project("pi-clean-native-skills", skip_metadata=True)
        self.assertFalse(row["pi_qoder_installation_audit"]["installation_qualified"])
        for early in (True, False):
            with self.subTest(early=early), TemporaryDirectory() as temporary:
                fixture = InertLifecycle(self.audit, self.fixtures, "pi", "skill", 0,
                                         acceptance="pi-clean-native-skills")
                observer = fixture.prepare(Path(temporary).resolve())
                try:
                    fixture.step(observer, "install")
                    if not early:
                        fixture.step(observer, "list")
                        fixture.metadata(observer)
                    with self.assertRaises(ValueError): fixture.metadata(observer)
                    self.assertEqual(len(observer.observations), 1 if early else 2)
                    self.assertTrue(observer.errors)
                finally:
                    observer.close()

    def test_main_project_summaries_join_thirty_real_receipts_and_keep_native_verdict_separate(self):
        for acceptance, (target, kind) in installation.PROJECT_ACCEPTANCES.items():
            rows, fixtures = [], copy.deepcopy(self.fixtures)
            for index in range(30):
                row, authored = self.exercise_main_project(acceptance, index)
                self.assertEqual(row["pi_qoder_installation_audit"]["status"], "verified")
                rows.append(row)
                fixtures["skills"][index] = authored["skills"][index]
            clean = acceptance == "pi-clean-native-skills"
            report = {"acceptance": acceptance, "target": target, "kind": kind, "platform": "linux", "architecture": "x86_64",
                "client_version": {"status": "reported", "value": "1.5.0" if target == "pi" else "1.1.81", "exit_code": 0},
                "cli_version": {"status": "reported", "value": "agentplugins 0.1.115", "exit_code": 0},
                "fixture_count": 30, "cases_attempted": 30, "source_selection_complete": True,
                "full_coverage": True, "passed": 30 if clean else 0, "prepared_lifecycle_passed": 0 if clean else 30,
                "verified_negative_coverage": 0, "results": rows}
            pins = self.audit["effective_signed_release_pins"](fixtures, target)
            result = installation.summary(report, fixtures, acceptance, signed_release_pins=pins)
            self.assertTrue(result["installation_acceptance_passed"], acceptance)
            self.assertEqual(result["installation_passed"], 30)
            self.assertTrue(all(result[key] is False for key in installation.QUALIFICATION))
            for mutation in ("removal", "acceptance", "report-acceptance", "native-count", "status", "missing-phase"):
                changed = copy.deepcopy(report)
                row = changed["results"][0]
                if mutation == "removal": row["removal_verified"] = False
                elif mutation == "acceptance": row["pi_qoder_installation_audit"]["acceptance"] = "pi-skills-installation"
                elif mutation == "report-acceptance": changed["acceptance"] = "pi-skills-installation"
                elif mutation == "native-count": changed["passed"] = 0 if clean else 30
                elif mutation == "status": row["status"] = "incomplete-native-proof"
                else: row["pi_qoder_installation_audit"]["observations"].pop()
                with self.subTest(acceptance=acceptance, mutation=mutation):
                    self.assertFalse(installation.summary(changed, fixtures, acceptance,
                                                         signed_release_pins=pins)["installation_acceptance_passed"])


if __name__ == "__main__":
    unittest.main()
