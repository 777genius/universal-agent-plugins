"""Hermetic unit/inert-transport regressions; these are never native evidence."""
import copy
import errno
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import shutil
import tempfile
import unittest
from unittest import mock

import windsurf_global_skill_audit as proof

ROOT = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location("live_windsurf_unit", ROOT / "scripts/live-install-audit.py")
audit = importlib.util.module_from_spec(spec)
spec.loader.exec_module(audit)
ITEM = "github:obra/the-elements-of-style@05fc4f0d2b97b7c042dd9949ad658568e4a1324e"
UID = "39b5780a-ea06-41f3-9b42-41918272b260"
SKILL = "writing-clearly-and-concisely"
DESCRIPTION = "Inert exact catalog note"
BODY = ("---\nname: " + SKILL + "\ndescription: " + DESCRIPTION + "\n---\nA unit-only fixture.\n").encode()
REFERENCE = b"unit-only owned reference\n"
# Explicit inert immutable source. These literal pins were independently
# composed with packagedigest v1's binary framing, never obtained by asking the
# verifier to bless fixture bytes. They replace production pins only in tests.
SOURCE_FILES = {
    "plugin.json": b'{"name":"elements-of-style","version":"1.0.0"}',
    "skills/writing-clearly-and-concisely/SKILL.md": BODY,
    "skills/writing-clearly-and-concisely/references/note.txt": REFERENCE,
}
UNIT_TREE = "sha256:4cf078c031e51e4bf846c8aa0c7e16e8f139290a2405b56933c378bd5265de57"
UNIT_MANIFEST = "sha256:7f863d62e433cda5dd3cc8e06198a8d526275b498f9f91f699283870a54123f7"


def hashed(body):
    return hashlib.sha256(body).hexdigest()


def packed(value):
    return json.dumps(value, separators=(",", ":"), ensure_ascii=False).encode()


def write(path, body):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_bytes(body)


def atomic(path, body):
    pending = path.with_name(path.name + ".unit-pending")
    pending.write_bytes(body)
    os.replace(pending, path)


def source_digest(files):
    """Inert producer's literal packagesnapshot framing, including directories."""
    dirs = set()
    for name in files:
        parent = Path(name).parent
        while str(parent) != ".":
            dirs.add(parent.as_posix())
            parent = parent.parent
    stream = bytearray()
    for name in sorted(dirs | set(files)):
        if name in dirs:
            stream += b"dir\0" + name.encode() + b"\0false\0" + b"0\0"
        else:
            body = files[name]
            stream += b"file\0" + name.encode() + b"\0false\0" + str(len(body)).encode() + b"\0" + body
    return "sha256:" + hashed(stream)


class InertPublicAndNative:
    """Only transport and resulting fixture bytes are inert; verifier is real."""
    def __init__(self, base, mode="good"):
        self.base, self.mode = base, mode
        self.binary = base / "vendor/bin/devin"
        write(self.binary, b"inert unit transport, never executed\n")
        self.binary.chmod(0o755)
        self.expected_pin = copy.deepcopy(proof.CASES[ITEM])
        if mode != "production-pins-arbitrary-source":
            self.expected_pin.update(tree_digest=UNIT_TREE, manifest_digest=UNIT_MANIFEST)
        if mode == "coherent-subset":
            self.expected_pin["skills"] = ("second-source-skill", "writing-clearly-and-concisely")
            self.expected_pin["tree_digest"] = "sha256:8d6ca1c690b69a25eb40a97939d9fbcc80070dd3b2d303829996b18ff1e0a7db"
        self.pin = copy.deepcopy(self.expected_pin)
        if mode == "coherent-digest-forgery":
            self.pin.update(tree_digest="sha256:" + "a" * 64, manifest_digest="sha256:" + "b" * 64)
        self.repo, self.commit = ITEM[7:].split("@")
        self.name = self.pin["name"]
        self.physical = self.name + "-" + hashed(UID.encode())[:12]
        self.source_key = "github:" + self.repo + ":"
        self.events, self.adds, self.removed = [], 0, False
        self.verifier = None
        self.revision = {"version": self.pin["version"], "resolved_revision": self.commit,
                         "tree_digest": self.pin["tree_digest"], "manifest_digest": self.pin["manifest_digest"]}

    def author_installation(self, env, cwd):
        self.home, self.project, self.data = Path(env["HOME"]), Path(cwd), Path(env["AGENTPLUGINS_HOME"])
        self.managed = self.data / "managed/clients/windsurf" / self.physical
        self.native = self.home / ".config/devin/skills" / SKILL
        self.source_cache = self.data / "store/packages/agentplugins-tree-sha256-v1" / self.pin["tree_digest"][7:] / "package"
        self.binding_id = "client_" + hashed("\0".join((UID, "windsurf", "user", str(self.managed))).encode())[:24]
        for name, body in SOURCE_FILES.items():
            write(self.source_cache / name, body)
        if self.mode == "coherent-subset":
            write(self.source_cache / "skills/second-source-skill/SKILL.md", b"---\nname: second-source-skill\ndescription: Inert second source skill\n---\nSecond.\n")
        projected_body, projected_reference = BODY, REFERENCE
        if self.mode in ("coherent-projection-rewrite", "coherent-source-and-projection-rewrite"):
            projected_body += b"coherent changed projection\n"
            projected_reference += b"coherent changed reference\n"
        if self.mode == "source-cache-bytes":
            write(self.source_cache / "skills" / SKILL / "references/note.txt", REFERENCE + b"changed source\n")
        if self.mode == "coherent-source-and-projection-rewrite":
            write(self.source_cache / "skills" / SKILL / "SKILL.md", projected_body)
            write(self.source_cache / "skills" / SKILL / "references/note.txt", projected_reference)
        if self.mode in ("source-cache-symlink", "source-cache-parent-symlink"):
            source = self.source_cache if self.mode == "source-cache-symlink" else self.source_cache.parent
            outside = self.base / "outside-source-cache"
            source.rename(outside)
            source.symlink_to(outside, target_is_directory=True)
        if self.mode == "source-cache-missing":
            shutil.rmtree(self.source_cache)
        files = {"plugin.json": packed({"name": self.name, "version": self.pin["version"]}),
                 "skills/" + SKILL + "/SKILL.md": projected_body,
                 "skills/" + SKILL + "/references/note.txt": projected_reference}
        for name, body in files.items():
            write(self.managed / name, body)
        write(self.native / "SKILL.md", projected_body)
        write(self.native / "references/note.txt", projected_reference)
        if self.mode == "symlink-leaf":
            outside = self.base / "outside-skill.md"
            write(outside, BODY)
            (self.native / "SKILL.md").unlink()
            (self.native / "SKILL.md").symlink_to(outside)
        if self.mode == "symlink-parent":
            original = self.home / ".config/devin/skills"
            outside = self.base / "outside-skills"
            original.rename(outside)
            original.symlink_to(outside, target_is_directory=True)
        # This literal fixture stream is independent of the verifier's traversal.
        native_digest = "sha256:" + hashed(b"f\0SKILL.md\0-\0" + projected_body + b"\0d\0references\0f\0references/note.txt\0-\0" + projected_reference + b"\0")
        objects = [{"object_id": "package:windsurf:" + self.physical, "kind": "managed_package_directory",
                    "logical_name": self.name, "path": str(self.managed), "managed_digest": source_digest(files), "protection_class": "managed"},
                   {"object_id": "devin_global_skill_directory:" + SKILL, "kind": "devin_global_skill_directory",
                    "logical_name": SKILL, "path": str(self.native), "source_relative": "skills/" + SKILL,
                    "managed_digest": native_digest, "protection_class": "managed"}]
        source = {"source_binding_id": "src_" + hashed((self.source_key + "\0" + self.repo + "\0").encode())[:24],
                  "requested_source": ITEM, "canonical_source": ITEM, "repository": self.repo,
                  "resolved_revision": self.commit, "tree_digest": self.pin["tree_digest"]}
        binding = {"client_binding_id": self.binding_id, "client_id": "windsurf", "scope": "user",
                   "target_locator": str(self.managed), "physical_artifact_id": self.physical,
                   "native_profile_root": str(self.home / ".config/devin"), "materialization": "materialized",
                   "activation": "active", "authentication": "not_required", "policy": "allowed", "verification": "installation_verified",
                   "affected_surfaces": ["windsurf"], "package_revision": copy.deepcopy(self.revision), "native_objects": objects}
        self.state = {"schema_version": 5, "installations": [{"installation_id": UID, "declared_name": self.name,
            "origin_mode": "direct", "source": source, "package": {"loader_kind": "agent_plugins", "format_id": "agent-plugins/1.0.0",
                "schema_uri": "https://plugins.sh/schema/1.0.0.json", "declared_name": self.name, "version": self.pin["version"],
                "manifest_digest": self.pin["manifest_digest"], "inventory": {"skills": [SKILL]}}, "clients": {self.binding_id: binding}}]}
        self.lock = {"lock_version": 1, "manifest_digest": "sha256:" + hashed(packed({"schema_version": 1,
            "dependencies": [{"alias": self.name, "source_key": self.source_key, "selector": {"kind": "commit", "value": self.commit}}],
            "targets": [{"id": "windsurf", "aliases": [self.name]}]})), "packages": [{"version": self.pin["version"],
            "alias": self.name, "source": {"git": {"repository": self.repo, "requested": {"kind": "commit", "value": self.commit}, "commit": self.commit}},
            "source_key": self.source_key, "digest_algorithm": "agentplugins-tree-sha256-v1", "tree_digest": self.pin["tree_digest"],
            "manifest_digest": self.pin["manifest_digest"], "targets": ["windsurf"]}]}
        if self.mode == "wrong-source": source["repository"] = "foreign/repo"
        if self.mode == "wrong-commit": source["resolved_revision"] = "a" * 40
        if self.mode == "wrong-uuid": self.state["installations"][0]["installation_id"] = "39b5780a-ea06-41f3-9b42-41918272b261"
        if self.mode == "wrong-revision": binding["package_revision"]["manifest_digest"] = "sha256:" + "a" * 64
        if self.mode == "wrong-scope": binding["scope"] = "project"
        if self.mode == "wrong-physical": binding["physical_artifact_id"] = "foreign-000000000000"
        if self.mode == "wrong-root": binding["native_profile_root"] = str(self.home / ".devin")
        if self.mode == "wrong-lock": self.lock["packages"][0]["tree_digest"] = "sha256:" + "b" * 64
        if self.mode == "wrong-native-digest": objects[1]["managed_digest"] = "sha256:" + "c" * 64
        if self.mode == "wrong-receipt": objects[1]["object_id"] += "-foreign"
        if self.mode == "prepared": binding.update(activation="prepared", authentication="not_checked")
        if self.mode == "manual": binding.update(activation="manual_activation_required", authentication="not_checked")
        if self.mode in ("prepare-intent", "unknown-intent"): binding["install_intent"] = "prepare" if self.mode == "prepare-intent" else "automatic"
        write(self.data / ("state-v5.json" if self.mode == "state-v5-only" else "state-v2.json"), packed(self.state))
        write(self.data / "uap.lock", packed(self.lock))
        write(self.data / "uap.toml", (f'schema_version = 1\n[dependencies.{self.name}]\ngit = "https://github.com/{self.repo}.git"\ncommit = "{self.commit}"\n[targets]\nwindsurf = ["{self.name}"]\n').encode())
        self.public_node = {"installation_id": UID, "name": self.name, "version": self.pin["version"], "source": self.repo,
            "clients": [{k: copy.deepcopy(binding[k]) for k in ("client_id", "scope", "materialization", "activation", "authentication", "policy", "verification", "package_revision", "affected_surfaces")}], "mixed_version": False}

    def add_document(self, repeated):
        action = {"alias": self.name, "client_id": "windsurf", "action": "unchanged" if repeated else "install",
                  "revision": self.commit, "package_digest": self.pin["tree_digest"], "to_version": self.pin["version"]}
        if repeated:
            action.update(installation_id=UID, binding_id=self.binding_id, from_version=self.pin["version"],
                          from_revision=self.commit, from_digest=self.pin["tree_digest"])
        else:
            action["component_support"] = [{"kind": "mcp_server" if self.mode == "mcp-component" else "skill", "name": SKILL, "support": "prepared"}]
        return {"schema_version": 1, "command": "add", "result": "success", "data": {"actions": [action], "command": "agentplugins add",
            "environment": {"alias": self.name, "intent_change": not repeated, "lock_path": str(self.data / "uap.lock"),
                            "manifest_path": str(self.data / "uap.toml"), "saved": True, "scope": "user", "scope_selection_reason": "explicit", "selected_targets": ["windsurf"]},
            "outcome": "completed", "result": "success", "results": [] if repeated else [{"installation_id": UID, "client_id": "windsurf", "operation": "install", "outcome": "completed"}],
            "schema_version": 1, "scope": "user", "scope_selection_reason": "explicit"}}

    def remove_document(self):
        return {"actions": [{"from_version": self.pin["version"], "from_revision": self.commit, "from_digest": self.pin["tree_digest"],
            "alias": self.name, "client_id": "windsurf", "installation_id": UID, "binding_id": self.binding_id, "action": "remove"}],
            "command": "agentplugins remove", "outcome": "completed", "result": "success",
            "results": [{"installation_id": UID, "client_id": "windsurf", "operation": "remove", "outcome": "completed"}],
            "schema_version": 1, "scope": "user", "scope_selection_reason": "explicit"}

    def call(self, argv, env, cwd, timeout, **kwargs):
        self.events.append(("public", tuple(argv[1:])))
        assert kwargs.get("capture_full") is True
        if argv[1] == "add":
            self.adds += 1
            if self.adds == 1:
                self.author_installation(env, cwd)
            elif self.mode == "repeat-foreign-replacement":
                path = self.home / "foreign-untouched.txt"
                atomic(path, path.read_bytes())
            elif self.mode == "repeat-lock-drift":
                atomic(self.data / "uap.lock", packed({**self.lock, "manifest_digest": "sha256:" + "e" * 64}))
            elif self.mode == "repeat-source-cache-replacement":
                path = self.source_cache / "skills" / SKILL / "SKILL.md"
                atomic(path, path.read_bytes())
            elif self.mode == "repeat-source-cache-bytes":
                write(self.source_cache / "skills" / SKILL / "SKILL.md", BODY + b"changed source\n")
            document = self.add_document(self.adds > 1)
            if self.mode == "repeat-remint" and self.adds > 1:
                document["data"]["actions"][0]["installation_id"] = "39b5780a-ea06-41f3-9b42-41918272b261"
        elif argv[1] == "remove":
            assert argv[2] == UID
            self.removed = True
            if self.mode == "remove-source-cache-deleted":
                shutil.rmtree(self.source_cache)
            if self.mode != "native-residue": shutil.rmtree(self.native)
            if self.mode == "remove-managed-renamed":
                self.managed.rename(self.managed.with_name(self.managed.name + "-retained"))
            else:
                shutil.rmtree(self.managed)
            removed_state = copy.deepcopy(self.state)
            removed_state["installations"][0].update(clients={}, data_retained=True)
            atomic(self.data / "state-v2.json", packed(removed_state))
            atomic(self.data / "uap.toml", b"schema_version = 1\n[dependencies]\n[targets]\n")
            empty = {"lock_version": 1, "manifest_digest": proof.EMPTY_INTENT, "packages": []}
            if self.mode == "remove-nonempty": empty["packages"] = self.lock["packages"]
            if self.mode == "remove-wrong-digest": empty["manifest_digest"] = "sha256:" + "d" * 64
            if self.mode == "remove-deleted-lock": (self.data / "uap.lock").unlink()
            elif self.mode != "remove-stale-lock": atomic(self.data / "uap.lock", packed(empty))
            document = self.remove_document()
        elif argv[1] == "list":
            nodes = [] if self.removed else [copy.deepcopy(self.public_node)]
            if self.mode == "list-remint" and not self.removed:
                nodes[0]["installation_id"] = "39b5780a-ea06-41f3-9b42-41918272b261"
            if self.removed and self.mode.startswith("post-remint-"):
                node = copy.deepcopy(self.public_node)
                node["installation_id"] = "39b5780a-ea06-41f3-9b42-41918272b261"
                if self.mode == "post-remint-name": node["name"] = "different-name"
                if self.mode == "post-remint-source": node.update(name="different-name", source="different/repo")
                nodes = [node]
            document = {"schema_version": 1, "command": "list", "result": "success", "data": {"installations": nodes}}
        else:
            raise AssertionError("unexpected public child")
        body = packed(document).decode()
        return {"exit_code": 0, "timeout": False, "stdout_tail": body[-1800:], "stderr_tail": "", "stdout_full": body, "stderr_full": "", "_json": document}

    def run_probe(self, argv, cwd, env, directory, label, **options):
        phase = label.removeprefix("windsurf-global-")
        self.events.append(("native", phase))
        assert argv == [str(self.binary), "skills", "list", "--json"] and str(cwd) == "/"
        assert options == {"timeout": 10, "limit": 1 << 20, "memory_only": True}
        assert self.verifier is not None
        # Static pinned inventory schema/rows; no verifier method supplies or
        # approves these transport results.
        rows = [
            {"name": "devin-cli", "description": "Look up Devin CLI documentation (skills, extensibility, configuration, commands, models, troubleshooting)", "provider": "Builtin", "base_dir": str(self.binary.parent.parent / "share/devin"), "display_name": "devin-cli", "triggers": ["model"], "warnings": [], "errors": []},
            {"name": "declarative-repo-setup", "description": "Generate and verify a working environment.yaml (Devin snapshot-setup blueprint) for a repo", "provider": "Builtin", "base_dir": "", "display_name": "declarative-repo-setup", "triggers": ["user", "model"], "warnings": [], "errors": []},
            {"name": "upload-secrets", "description": "Securely upload local secrets (dotenv files, env vars, API keys) to the Devin Cloud secrets manager — values never enter the conversation", "provider": "Builtin", "base_dir": "", "display_name": "upload-secrets", "triggers": ["model", "user"], "warnings": [], "errors": []},
            {"name": "foreign-windsurf-global-audit", "description": "Inert foreign Global Skill", "provider": "Devin", "base_dir": str(Path(env["HOME"]) / ".config/devin/skills/foreign-windsurf-global-audit"), "display_name": "foreign-windsurf-global-audit", "triggers": ["user", "model"], "warnings": [], "errors": []},
        ]
        if phase in ("add", "repeat"):
            rows.append({"name": SKILL, "description": DESCRIPTION, "provider": "Devin", "base_dir": str(self.native), "display_name": SKILL, "triggers": ["user", "model"], "warnings": [], "errors": []})
        if phase == "before" and self.mode == "vendor-cache-small":
            write(self.home / ".cache/devin/cli/state.json", b"{}")
            write(self.home / ".config/devin/cli/state.json", b"{}")
            write(self.home / ".local/share/devin/state.json", b"{}")
        if phase == "before" and self.mode == "vendor-cache-foreign":
            write(self.home / ".config/devin/config.json", b'{"foreign":"changed"}')
        if phase == "before" and self.mode in ("before-cache", "before-tmp"):
            path = Path(env["HOME"]) / ".cache/devin/cli/unknown-1012.json" if self.mode == "before-cache" else Path(env["TMPDIR"]) / "unknown-probe-write"
            write(path, b"x" * 674560 if self.mode == "before-cache" else b"unowned")
        if phase == "add":
            if self.mode == "native-missing": rows.pop()
            if self.mode == "native-duplicate": rows.append(copy.deepcopy(rows[-1]))
            if self.mode == "native-path": rows[-1]["base_dir"] = str(self.home / ".devin/skills" / SKILL)
            if self.mode == "native-provider": rows[-1]["provider"] = "Windsurf"
            if self.mode == "native-description": rows[-1]["description"] = "unrelated"
            if self.mode == "native-foreign": rows[3]["description"] = "changed foreign inventory"
            if self.mode == "probe-owned-write": write(self.native / "SKILL.md", BODY + b"changed")
            if self.mode == "probe-state-write": write(self.data / "state-v2.json", packed({**self.state, "unexpected": True}))
            if self.mode == "probe-source-cache-write": write(self.source_cache / "skills" / SKILL / "SKILL.md", BODY + b"changed source\n")
        stdout = packed(rows)
        stderr = b""
        if phase == "add" and self.mode == "duplicate-json": stdout = stdout.replace(b'{"name":', b'{"name":"duplicate","name":', 1)
        if phase == "add" and self.mode == "nonfinite-json": stdout = stdout.replace(b'"warnings":[]', b'"warnings":[NaN]', 1)
        if phase == "add" and self.mode == "native-stderr": stderr = b"unqualified warning"
        record = {"probe": label, "argv": argv, "cwd": str(cwd), "error": None, "exit": 0, "elapsed_seconds": 0.01,
                  "stdout_file": label + ".stdout.log", "stderr_file": label + ".stderr.log",
                  "stdout_bytes": len(stdout), "stderr_bytes": len(stderr), "stdout_sha256": hashed(stdout), "stderr_sha256": hashed(stderr)}
        if phase == "add" and self.mode == "native-exit": record["exit"] = 1
        if phase == "add" and self.mode == "native-truncation": record["stdout_bytes"] += 1
        if phase == "add" and self.mode == "native-label": record["probe"] = "windsurf-global-before"
        if phase == "add" and self.mode == "native-cwd": record["cwd"] = str(self.project)
        return record, stdout, stderr


class WindsurfGlobalSkillAuditTests(unittest.TestCase):
    def run_case(self, mode="good"):
        with tempfile.TemporaryDirectory(prefix="windsurf-unit-") as temporary:
            root = Path(temporary)
            transport = InertPublicAndNative(root, mode)
            self.transport = transport
            cls = audit.WINDSURF_GLOBAL_AUDIT["WindsurfGlobalSkillAudit"]
            def real_factory(*args, **kwargs):
                transport.verifier = cls(*args, **kwargs)
                return transport.verifier
            with mock.patch.dict(cls.__init__.__globals__, {"BINARY_SHA": hashed(transport.binary.read_bytes()),
                    "CASES": {**proof.CASES, ITEM: transport.expected_pin}}), \
                    mock.patch.dict(audit.WINDSURF_GLOBAL_AUDIT, {"WindsurfGlobalSkillAudit": real_factory}), \
                    mock.patch.dict(audit.AGENT_BINARIES, {"devin": str(transport.binary)}, clear=True), \
                    mock.patch.object(audit, "call", side_effect=transport.call), \
                    mock.patch.dict(audit.NATIVE_RESEARCH, {"run_probe": transport.run_probe}):
                result = audit.do_case("/inert/uap", "windsurf", "plugin", ITEM, 26, root / "cases", {ITEM: "elements-of-style"},
                    client_version={"status": "reported", "value": "devin 3000.12.57 (96542e930752)", "exit_code": 0})
            self.assertTrue(transport.verifier.closed)
            return result

    def test_real_verifier_four_phases_and_atomic_empty_agreement(self):
        result = self.run_case()
        evidence = result["windsurf_global_native_proof"]
        self.assertEqual(result["status"], "passed")  # Unit transport result, not native evidence.
        self.assertTrue(result["prepared_components_only"])
        self.assertEqual(result["native_component_candidates"], [])
        self.assertEqual(result["native_identity_files"], [])
        self.assertTrue(result["repeat_verified"] and result["removal_verified"] and result["sentinels_preserved"])
        self.assertEqual([row["phase"] for row in evidence["observations"]], ["before", "add", "repeat", "remove"])
        self.assertTrue(all(row["verified"] and row["custody"]["unchanged"] for row in evidence["observations"]))
        self.assertEqual(evidence["owner"]["uuid"], UID)
        source = evidence["immutable_source_store"]
        self.assertEqual(source["tree_digest"], UNIT_TREE)
        self.assertEqual(source["held_frame"], source["last_verified_frame"])
        self.assertEqual(source["complete_skills"], [SKILL])
        self.assertEqual(source["path"], str(self.transport.source_cache))
        self.assertNotEqual(evidence["original_agreement"]["uap.lock"]["inode"], evidence["removed_agreement"]["lock"]["inode"])
        self.assertEqual(evidence["original_agreement"]["uap.lock"]["device"], evidence["removed_agreement"]["lock"]["device"])
        self.assertEqual(evidence["removed_agreement"]["manifest_digest"], proof.EMPTY_INTENT)
        self.assertFalse(evidence["cache_exception_granted"] or evidence["writer_closure_claimed"])
        self.assertEqual([event for event in self.transport.events if event[0] == "native"], [("native", p) for p in proof.PHASES])

    def test_source_owner_revision_and_receipt_boundaries(self):
        for mode in ("wrong-source", "wrong-commit", "wrong-uuid", "wrong-revision", "wrong-scope", "wrong-physical", "wrong-root", "wrong-lock", "wrong-native-digest", "wrong-receipt", "state-v5-only", "mcp-component", "coherent-digest-forgery", "symlink-leaf", "symlink-parent"):
            with self.subTest(mode=mode), self.assertRaises(ValueError):
                self.run_case(mode)

    def test_actual_cached_source_and_complete_set_are_independent_authorities(self):
        for mode in ("production-pins-arbitrary-source", "source-cache-bytes", "coherent-projection-rewrite",
                     "coherent-source-and-projection-rewrite", "source-cache-symlink", "source-cache-parent-symlink",
                     "source-cache-missing", "coherent-subset"):
            with self.subTest(mode=mode), self.assertRaises(ValueError) as caught:
                self.run_case(mode)
            self.assertFalse(caught.exception.audit_evidence["native_discovery_verified"])
            if mode == "coherent-subset":
                self.assertIn("complete immutable source set", str(caught.exception))
        # The reviewed former counterexample coherently substituted a new Skill
        # in all projected receipts while leaving production source pins intact.
        # Its complete set is now closed independently even under an inert pin.
        with mock.patch.dict(globals(), {"SKILL": "not-in-immutable-source",
                "BODY": BODY.replace(b"writing-clearly-and-concisely", b"not-in-immutable-source")}):
            with self.assertRaisesRegex(ValueError, "complete immutable source set"):
                self.run_case()

    def test_prepared_manual_and_unknown_intent_never_promote(self):
        for mode in ("prepared", "manual", "prepare-intent", "unknown-intent", "list-remint"):
            with self.subTest(mode=mode):
                result = self.run_case(mode)
                self.assertNotEqual(result["status"], "passed")
                self.assertFalse(result["windsurf_global_native_proof"]["native_discovery_verified"])
                self.assertTrue(self.transport.removed)
                self.assertEqual(len(result["windsurf_global_native_proof"]["observations"]), 4)

    def test_full_inventory_and_original_capture_required_with_cleanup(self):
        for mode in ("native-missing", "native-duplicate", "native-path", "native-provider", "native-description", "native-foreign", "duplicate-json", "nonfinite-json", "native-stderr", "native-exit", "native-truncation", "native-label", "native-cwd"):
            with self.subTest(mode=mode):
                result = self.run_case(mode)
                self.assertNotEqual(result["status"], "passed")
                self.assertTrue(self.transport.removed)
                self.assertFalse(result["windsurf_global_native_proof"]["native_discovery_verified"])

    def test_only_bounded_native_devin_cache_may_change_during_a_real_probe(self):
        result = self.run_case("vendor-cache-small")
        self.assertEqual(result["status"], "passed")
        proof = result["windsurf_global_native_proof"]
        self.assertTrue(proof["cache_exception_granted"])
        self.assertEqual(len(proof["native_cache_evidence"]), 1)
        self.assertEqual(proof["native_cache_evidence"][0]["phase"], "before")
        native = proof["observations"]
        self.assertTrue(all(row["verified"] for row in native))
        self.assertFalse(native[0]["custody"]["unchanged"])
        self.assertTrue(native[0]["custody"]["native_cache_only"])
        self.assertTrue(all(row["custody"]["unchanged"] for row in native[1:]))
        failed = self.run_case("vendor-cache-foreign")
        self.assertEqual(failed["status"], "harness-error")
        self.assertFalse(failed["windsurf_global_native_proof"]["native_discovery_verified"])

    def test_probe_cannot_create_cache_tmp_or_mutate_owned_authority(self):
        for mode in ("before-cache", "before-tmp", "probe-owned-write", "probe-state-write", "probe-source-cache-write"):
            with self.subTest(mode=mode), self.assertRaises(ValueError) as caught:
                self.run_case(mode)
            evidence = caught.exception.audit_evidence
            self.assertFalse(evidence["native_discovery_verified"] or evidence["cache_exception_granted"])
            self.assertFalse(evidence["observations"][-1]["custody"]["unchanged"])
            changes = evidence["observations"][-1]["custody"]["changes"]
            self.assertGreater(changes["total"], 0)
            self.assertLessEqual(len(changes["entries"]), 8)
            if mode == "before-cache":
                self.assertTrue(any(entry.get("relative_key") == "home/.cache/devin/cli/unknown-1012.json"
                                    for entry in changes["entries"]))

    def test_frame_diagnostics_bound_names_and_export_no_native_values(self):
        before = {"home/native/" + str(i): {"mode": 1, "inode": 2, "sha256": "private-before-value"}
                  for i in range(10)}
        after = {key: {"mode": 1, "inode": 3, "sha256": "private-after-value"} for key in before}
        changes = proof.frame_changes(before, after)
        self.assertEqual((changes["total"], changes["omitted"], len(changes["entries"])), (10, 2, 8))
        self.assertTrue(all(entry["changed_fields"] == ["inode", "sha256"] for entry in changes["entries"]))
        self.assertNotIn("private-", json.dumps(changes))
        for key, expected in (("/outside/private", None), ("../outside", None),
                              ("home/line\nnext", "home/line?next"), ("a" * 250, "a" * 192)):
            entry = proof.frame_changes({key: (1, 2, 3, 4)}, {key: (1, 2, 4, 5)})["entries"][0]
            self.assertEqual(entry.get("relative_key"), expected)

    def test_repeat_and_remove_never_adopt_replacement_or_residue(self):
        for mode in ("repeat-foreign-replacement", "repeat-lock-drift", "repeat-remint", "native-residue", "remove-nonempty", "remove-wrong-digest", "remove-deleted-lock", "remove-stale-lock",
                     "repeat-source-cache-replacement", "repeat-source-cache-bytes", "remove-source-cache-deleted"):
            with self.subTest(mode=mode), self.assertRaises(ValueError):
                self.run_case(mode)

    def test_renamed_original_managed_package_cannot_qualify_removal(self):
        with self.assertRaisesRegex(ValueError, "original managed package remains linked") as caught:
            self.run_case("remove-managed-renamed")
        self.assertFalse(caught.exception.audit_evidence["native_discovery_verified"])
        self.assertNotIn(("native", "remove"), self.transport.events)
        self.assertTrue(self.transport.verifier.closed)

    def test_later_custody_refusal_clears_prior_finished_evidence(self):
        cls = audit.WINDSURF_GLOBAL_AUDIT["WindsurfGlobalSkillAudit"]
        finish, guard = cls.finish, cls.guard
        for final_check in (finish, guard):
            with self.subTest(final_check=final_check.__name__):
                def finish_then_replace(verifier):
                    self.assertTrue(finish(verifier))
                    self.assertTrue(verifier.evidence()["native_discovery_verified"])
                    foreign = verifier.home / "foreign-untouched.txt"
                    atomic(foreign, foreign.read_bytes())
                    return final_check(verifier)
                # Both checks are real; replace only the fixture between them.
                with mock.patch.object(cls, "finish", finish_then_replace), \
                        self.assertRaises(ValueError) as caught:
                    self.run_case()
                self.assertFalse(caught.exception.audit_evidence["native_discovery_verified"])
                self.assertFalse(self.transport.verifier.evidence()["native_discovery_verified"])
                self.assertTrue(self.transport.verifier.closed)

    def test_rejected_file_hold_closes_duplicate_and_keeps_prior_owner(self):
        with tempfile.TemporaryDirectory(prefix="windsurf-hold-unit-") as temporary:
            root = Path(temporary).resolve(strict=True)
            path = root / "authority.json"
            write(path, b'{"owned":true}\n')
            directory = proof.custody.open_directory(root)
            previous = os.open(path, os.O_RDONLY | os.O_NOFOLLOW)
            duplicates = []
            try:
                body, fact = proof.read_file(directory, path.name)
                verifier = object.__new__(proof.WindsurfGlobalSkillAudit)
                verifier.case, verifier.dirs = root, {root: (directory, proof.facts(os.fstat(directory)))}
                verifier.files, verifier.retired = {path.name: (previous, body, fact)}, []
                duplicate = os.dup
                def replace_after_duplicate(fd):
                    held = duplicate(fd)
                    duplicates.append(held)
                    atomic(path, body)
                    return held
                with mock.patch.object(proof.os, "dup", side_effect=replace_after_duplicate), \
                        self.assertRaisesRegex(ValueError, "member changed during bounded read"):
                    verifier._hold(path.name)
                self.assertEqual(verifier.files[path.name], (previous, body, fact))
                self.assertEqual(verifier.retired, [])
                self.assertEqual(len(duplicates), 1)
                with self.assertRaises(OSError) as caught:
                    os.fstat(duplicates[0])
                self.assertEqual(caught.exception.errno, errno.EBADF)
            finally:
                for fd in [directory, previous, *duplicates]:
                    try:
                        os.close(fd)
                    except OSError:
                        pass

    def test_fresh_target_list_rejects_every_reminted_registration(self):
        for mode in ("post-remint-uuid", "post-remint-name", "post-remint-source"):
            with self.subTest(mode=mode):
                result = self.run_case(mode)
                self.assertNotEqual(result["status"], "passed")
                self.assertFalse(result["removal_verified"])
                self.assertFalse(result["windsurf_global_native_proof"]["native_discovery_verified"])

    def test_closed_source_set_is_exactly_four_immutable_revisions(self):
        self.assertEqual(set(proof.CASES), {
            "github:gemini-cli-extensions/sre@4130b149c0b069ef2028e42b3b9f3f84cce935e6",
            "github:K-Dense-AI/claude-scientific-writer@529b9f73ab4b48925027145800d7b49e96ccfd3c",
            ITEM, "github:workato-devs/recipe-skills@afa1e729a013061c7b328271fadcdf240091cf3c"})
        self.assertNotIn(ITEM.replace("05fc4f0d2b97b7c042dd9949ad658568e4a1324e", "main"), proof.CASES)

    def test_digest_formats_preserve_distinct_order_empty_dirs_and_execute_bits(self):
        with tempfile.TemporaryDirectory(prefix="windsurf-digest-unit-") as temporary:
            root = Path(temporary)
            write(root / "a/z", b"Z")
            write(root / "a-0", b"L")
            (root / "empty").mkdir()
            write(root / "run.sh", b"R")
            (root / "run.sh").chmod(0o755)
            fd = proof.custody.open_directory(root)
            try:
                observed = proof.frame(fd)
                # Literal independently composed streams from the two Go
                # formats: package visits a-0 before a/z; Skill does the reverse.
                self.assertEqual(proof.managed_digest(fd, observed), "sha256:ad4050eb1884bdc6b03bc927c3ef45a2451b39a464fabc5b3f3bd4e030bce9dd")
                self.assertEqual(proof.skill_digest(fd, observed), "sha256:46025ff092066f1d050338658a499f044a616ec755af42231bbd39e95e289b41")
                self.assertEqual(proof.immutable_source_digest(fd, observed), "sha256:b9d377d4f21d2b482e61cca67c8ba2e13080c4b5e27f69183f774b3598b38b21")
                (root / "run.sh").chmod(0o644)
                with self.assertRaises(ValueError):
                    proof.managed_digest(fd, observed)
            finally:
                os.close(fd)

    def test_native_phases_cannot_start_after_missing_before(self):
        with tempfile.TemporaryDirectory(prefix="windsurf-phase-unit-") as temporary:
            base = Path(temporary)
            transport = InertPublicAndNative(base)
            case = base / "case"
            for path in (case, case / "home", case / "project", case / "uap-data", case / "tmp",
                         case / "home/.config", case / "home/.cache", case / "home/.local/share"):
                path.mkdir(mode=0o700, parents=True, exist_ok=True)
            write(case / "home/foreign-untouched.txt", b"foreign profile content never belongs to UAP\n")
            write(case / "home/.windsurf/foreign-neighbor.txt", b"unmanaged native neighbor must survive\n")
            write(case / "project/foreign-untouched.txt", b"foreign content never belongs to UAP\n")
            with mock.patch.object(proof, "BINARY_SHA", hashed(transport.binary.read_bytes())):
                verifier = proof.WindsurfGlobalSkillAudit(case, case / "home", case / "project", ITEM, transport.binary,
                    {"status": "reported", "value": "devin 3000.12.57 (96542e930752)", "exit_code": 0})
            try:
                self.assertFalse(verifier.finish())
                with self.assertRaises(ValueError): verifier.begin_probe("add")
                self.assertFalse(verifier.finish())
            finally:
                verifier.close()

    def test_yaml_metadata_is_strict_and_never_a_name_only_observation(self):
        self.assertEqual(proof.description(BODY, SKILL), DESCRIPTION)
        for body in (BODY.replace(b"description:", b"description: original\ndescription:", 1),
                     BODY.replace(b"description: Inert exact catalog note", b"description: [wrong]"),
                     BODY.replace(b"---\nA unit-only fixture.", b"no closing frontmatter"),
                     BODY.replace(b"description:", b"4: value\ndescription:")):
            with self.subTest(body=body), self.assertRaises(ValueError): proof.description(body, SKILL)
        for body in (b'{"scope":"user","SCOPE":"project"}', b'{"future":' + b"9" * 400 + b"}"):
            with self.subTest(body=body), self.assertRaises(ValueError): proof.strict_json(body)


if __name__ == "__main__":
    unittest.main()
