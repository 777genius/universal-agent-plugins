"""Real filesystem and raw-envelope controls for the explicit installation lane."""
import copy
import hashlib
import json
import os
import shutil
from pathlib import Path
from io import StringIO
from runpy import run_path
import stat
import subprocess
import sys
from tempfile import TemporaryDirectory
import unittest
from unittest.mock import patch
from types import SimpleNamespace

import windsurf_installation_audit as installation

from windsurf_installation_audit import (ACTIVE_SLOTS, ACTIVE_SKILLS, CLIENT_VERSION, CLI_VERSION,
    COMPONENT_REFUSALS, PREPARED, WindsurfInstallationAudit, selected, summary,
    image_drift_diagnostic, require_same_image)

ROOT = Path(__file__).resolve().parents[1]
VERIFY_PRODUCER = installation.verify_projection_producer


def source_pin(files):
    """Independent inert producer's packagedigest framing, never a receipt."""
    directories = set()
    for name in files:
        directories.update(parent.as_posix() for parent in Path(name).parents if str(parent) != ".")
    stream = bytearray()
    def field(value):
        body = value.encode()
        stream.extend(len(body).to_bytes(8, "big") + body)
    field("agentplugins.package-tree\0sha256\0v1")
    for name in sorted(directories | set(files)):
        directory = name in directories
        for value in ("entry", name, "directory" if directory else "file", "040000" if directory else "100644", ""):
            field(value)
        body = b"" if directory else files[name]
        stream.extend(len(body).to_bytes(8, "big") + body)
    return {"tree_digest": "sha256:" + hashlib.sha256(stream).hexdigest(),
            "manifest_digest": "sha256:" + hashlib.sha256(files["plugin.json"]).hexdigest()}


def delivery_pin(root):
    """Independent producer framing includes executable launcher mode."""
    stream = bytearray()
    for path in sorted(root.rglob("*")):
        name = path.relative_to(root).as_posix()
        file = path.is_file()
        body = path.read_bytes() if file else b""
        execute = file and bool(path.stat().st_mode & 0o111)
        stream.extend(f'{"file" if file else "dir"}\0{name}\0{str(execute).lower()}\0{len(body)}\0'.encode() + body)
    return "sha256:" + hashlib.sha256(stream).hexdigest()


@unittest.skipUnless(sys.platform == "linux", "Exact Linux Devin Global installation audit")
class WindsurfInstallationAuditTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.audit = run_path(str(ROOT / "scripts/live-install-audit.py"))
        cls.fixture_bytes = (ROOT / "tests/live-install/fixtures.json").read_bytes()
        cls.fixtures = json.loads(cls.fixture_bytes)
        cls.fixture_hash = hashlib.sha256(cls.fixture_bytes).hexdigest()

    def setUp(self):
        # Replace production pins only in this inert suite. Positives are real
        # canonical Go projections from these independently framed source trees.
        self.fixtures = copy.deepcopy(type(self).fixtures)
        self.sources, self.pins = {}, {}
        for index in range(30):
            source, identity = selected(self.fixtures, index)
            files = {"plugin.json": json.dumps({"$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json", "name": identity, "version": "1.0.0"}, separators=(",", ":")).encode(),
                     "README.md": b"Independent source bytes must be delivered.\n",
                     "hooks/inert.json": b"{}"}
            skills = ("unit-skill",) if index in ACTIVE_SLOTS else ()
            if skills:
                files["skills/unit-skill/SKILL.md"] = b"---\nname: unit-skill\ndescription: Inert independent Skill\n---\nUnit bytes.\n"
            else:
                files["mcp.json"] = json.dumps({"$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json", "mcpServers": {"unit-server": {
                    "type": "streamable-http", "url": "https://unit.invalid/" + identity}}}, separators=(",", ":")).encode()
            pin = {**source_pin(files), "name": identity, "version": "1.0.0", "skills": skills}
            self.sources[source], self.pins[source] = files, pin
            if not source.startswith("github:"):
                self.fixtures["signed_release_pins"][source].update(source_pin(files))
        self.addCleanup(patch.stopall)
        self.native_pins = {selected(self.fixtures, index)[0]: self.pins[selected(self.fixtures, index)[0]]
                            for index in ACTIVE_SLOTS}
        self.prepared_git_pins = {source: self.pins[source] for source in installation.PREPARED_GIT_PINS}
        patch.object(installation, "GLOBAL_SKILL_CASES", self.native_pins).start()
        patch.object(installation, "PREPARED_GIT_PINS", self.prepared_git_pins).start()
        # The transport is deliberately inert; all byte/digest/projection code
        # remains real. Producer-binary provenance has separate controls below.
        patch.object(installation, "verify_projection_producer", return_value=None).start()
        observer_globals = self.audit["WINDSURF_INSTALLATION"]["WindsurfInstallationAudit"].__init__.__globals__
        patch.dict(observer_globals, {"GLOBAL_SKILL_CASES": self.native_pins,
            "PREPARED_GIT_PINS": self.prepared_git_pins,
            "verify_projection_producer": lambda cli: None,
            "projection_helper": installation.projection_helper}).start()
        temporary = TemporaryDirectory(prefix="windsurf-unit-fixtures-")
        self.addCleanup(temporary.cleanup)
        fixture_path = Path(temporary.name) / "fixtures.json"
        self.fixture_bytes = json.dumps(self.fixtures).encode()
        self.fixture_hash = hashlib.sha256(self.fixture_bytes).hexdigest()
        fixture_path.write_bytes(self.fixture_bytes)
        patch.dict(self.audit["main"].__globals__, {"FIXTURES": fixture_path}).start()

    def fixture(self, root, cli="/fixture/agentplugins", *, saved=None):
        case = root / "plugin-case"
        for relative in ("home/.config", "home/.windsurf", "project", "uap-data"):
            (case / relative).mkdir(parents=True)
        for name, body in (saved or {}).items():
            (case / "uap-data" / name).write_bytes(body)
        env = {"HOME": str(case / "home"), "XDG_CONFIG_HOME": str(case / "home/.config"),
               "AGENTPLUGINS_HOME": str(case / "uap-data")}
        observer = WindsurfInstallationAudit(cli, env, case / "project", case, self.audit)
        return observer, env

    def author_runtime(self, observer):
        """Independent minimal owned runtime; no npm command or native execution."""
        identity, uid = "unit-runtime", "39b5780a-ea06-41f3-9b42-41918272b260"
        physical = identity + "-" + hashlib.sha256(uid.encode()).hexdigest()[:12]
        lock = b'{"lockfileVersion":3,"requires":true,"packages":{"":{"dependencies":{"unit":"1.0.0"}},"node_modules/unit":{"version":"1.0.0"}}}'
        package = b'{"private":true,"dependencies":{"unit":"1.0.0"}}'
        lock_digest = "sha256:" + hashlib.sha256(lock).hexdigest()
        config = {"schema_version": 1, "package": "unit", "version": "1.0.0",
                  "entrypoint": "node_modules/unit/bin.js", "package_lock_sha256": lock_digest,
                  "omit_optional": False}
        prefix = "io.github.777genius.agentplugins/runtime/"
        files = {"plugin.json": b'{"name":"unit-runtime","version":"1.0.0"}',
                 prefix + "runtime.json": json.dumps(config).encode(),
                 prefix + "package.json": package, prefix + "package-lock.json": lock,
                 prefix + "launcher.mjs": b"// inert\n"}
        pin = source_pin(files)
        source = observer.data / "store/packages/agentplugins-tree-sha256-v1" / pin["tree_digest"][7:] / "package"
        for name, body in files.items():
            path = source / name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(body)
        data = observer.data / "plugin-data" / physical
        runtime = data / "npm-runtime" / lock_digest[7:]
        (runtime / "node_modules/.bin").mkdir(parents=True)
        (runtime / "node_modules/unit").mkdir()
        (runtime / "node_modules/unit/bin.js").write_bytes(b"console.log('inert');\n")
        (runtime / "node_modules/.bin/unit").symlink_to("../unit/bin.js")
        for name, body in (("package.json", package), ("package-lock.json", lock)):
            (runtime / name).write_bytes(body)
        marker = {"schema_version": 1, "lock_digest": lock_digest, "package": "unit", "version": "1.0.0",
                  "entrypoint": config["entrypoint"], "omit_optional": False}
        (runtime / ".agentplugins-runtime.json").write_text(json.dumps(marker))
        receipt_id = "data_" + hashlib.sha256("\0".join((uid, physical, "user")).encode()).hexdigest()[:24]
        receipt = {"data_receipt_id": receipt_id, "physical_backend_id": physical, "scope": "user", "state": "owned",
                   "locator": str(data), "ownership_digest": "sha256:" + hashlib.sha256(
                       ("agentplugins-plugin-data-v1\0" + receipt_id + "\0" + str(data)).encode()).hexdigest()}
        (data / ".agentplugins-data-owner.json").write_text(json.dumps(receipt))
        state = {"schema_version": 5, "installations": [{"installation_id": uid, "declared_name": identity,
                 "clients": {}, "data_retained": True, "source": {"tree_digest": pin["tree_digest"]}, "data_receipts": {receipt_id: receipt}}]}
        (observer.data / "state-v2.json").write_text(json.dumps(state))
        return runtime, state

    def test_owned_locked_npm_bin_links_have_full_bytes_and_identity_custody(self):
        with TemporaryDirectory() as temporary:
            observer, _ = self.fixture(Path(temporary).resolve())
            runtime, state = self.author_runtime(observer)
            before = observer.saved_image()
            self.assertTrue(before["runtime_authority"])
            link = next(row for row in before["runtime_authority"].values() if row["kind"] == "link")
            self.assertEqual(link["target"], "../unit/bin.js")
            self.assertEqual(before, observer.saved_image())
            binary = runtime / "node_modules/unit/bin.js"
            body, info = binary.read_bytes(), binary.stat()
            binary.write_bytes(body.replace(b"inert", b"other"))
            os.utime(binary, ns=(info.st_atime_ns, info.st_mtime_ns))
            self.assertNotEqual(before["runtime_authority"], observer.saved_image()["runtime_authority"])

    def test_runtime_authority_keeps_root_and_locked_input_identity(self):
        for mutation in (".agentplugins-runtime.json", "package.json", "package-lock.json", "root-mode"):
            with self.subTest(mutation=mutation), TemporaryDirectory() as temporary:
                observer, _ = self.fixture(Path(temporary).resolve())
                runtime, _ = self.author_runtime(observer)
                before = observer.saved_image()
                self.assertEqual(before["runtime_authority"], observer.saved_image()["runtime_authority"])
                if mutation == "root-mode":
                    runtime.chmod(stat.S_IMODE(runtime.stat().st_mode) ^ 0o010)
                else:
                    path = runtime / mutation
                    body, info = path.read_bytes(), path.stat()
                    path.rename(observer.case / "original-runtime-input")
                    path.write_bytes(body)
                    path.chmod(stat.S_IMODE(info.st_mode))
                    os.utime(path, ns=(info.st_atime_ns, info.st_mtime_ns))
                    self.assertNotEqual(path.stat().st_ino, info.st_ino)
                    self.assertEqual(path.read_bytes(), body)
                after = observer.saved_image()
                self.assertEqual(before["ancillary"], after["ancillary"])
                self.assertNotEqual(before["runtime_authority"], after["runtime_authority"])

    def test_retained_runtime_cannot_rebind_original_source_or_data_receipt(self):
        for mutation in ("unchanged", "coherent-source", "source-binding", "package-binding",
                         "coherent-receipt", "source-replacement"):
            with self.subTest(mutation=mutation), TemporaryDirectory() as temporary:
                observer, _ = self.fixture(Path(temporary).resolve())
                runtime, state = self.author_runtime(observer)
                record = state["installations"][0]
                receipt_id, receipt = next(iter(record["data_receipts"].items()))
                physical = receipt["physical_backend_id"]
                source_digest = record["source"]["tree_digest"]
                source = observer.data / "store/packages/agentplugins-tree-sha256-v1" / source_digest[7:] / "package"
                active = observer.data / "managed/clients/windsurf" / physical
                active.mkdir(parents=True); (active / "plugin.json").write_bytes((source / "plugin.json").read_bytes())
                record["package"] = {"manifest_digest": "sha256:" + "a" * 64}
                record["clients"] = {"bound": {"client_id": "windsurf", "physical_artifact_id": physical,
                    "data_receipt_id": receipt_id, "target_locator": str(active),
                    "package_revision": {"tree_digest": source_digest}}}
                record.pop("data_retained")
                state_path = observer.data / "state-v2.json"
                state_path.write_text(json.dumps(state))
                original = observer.saved_image()
                observer.saved_observations.append(original)
                shutil.rmtree(active)
                record.update(clients={}, data_retained=True)
                if mutation == "coherent-source":
                    files = {path.relative_to(source).as_posix(): path.read_bytes()
                             for path in source.rglob("*") if path.is_file()}
                    files["README.md"] = b"Different self-consistent source with the same runtime lock.\n"
                    replacement_digest = source_pin(files)["tree_digest"]
                    replacement = source.parent.parent / replacement_digest[7:] / "package"
                    for name, body in files.items():
                        path = replacement / name; path.parent.mkdir(parents=True, exist_ok=True); path.write_bytes(body)
                    record["source"]["tree_digest"] = replacement_digest
                elif mutation == "source-binding": record["source"]["repository"] = "unrelated/source"
                elif mutation == "package-binding": record["package"]["manifest_digest"] = "sha256:" + "b" * 64
                elif mutation == "coherent-receipt":
                    receipt["created_at"] = "2026-11-26T00:00:00Z"
                    (runtime.parent.parent / ".agentplugins-data-owner.json").write_text(json.dumps(receipt))
                elif mutation == "source-replacement":
                    held = source.with_name("previous-package"); source.rename(held); shutil.copytree(held, source)
                state_path.write_text(json.dumps(state))
                if mutation == "unchanged":
                    retained = observer.saved_image()
                    self.assertEqual(retained["runtime_authority"], original["runtime_authority"])
                else:
                    with self.assertRaises(ValueError): observer.saved_image()

    def test_runtime_reader_preserves_all_original_custody_entry_exclusions(self):
        for excluded in (".git", ".uap-integrations.lock", ".plugin-kit-ai.lock"):
            with self.subTest(excluded=excluded), TemporaryDirectory() as temporary:
                observer, _ = self.fixture(Path(temporary).resolve())
                runtime, _ = self.author_runtime(observer)
                path = runtime / "node_modules/unit" / excluded
                if excluded == ".git":
                    path.mkdir()
                else:
                    path.write_bytes(b"excluded authority never becomes npm custody")
                with self.assertRaises(ValueError):
                    observer.saved_image()

    def test_runtime_links_require_exact_receipt_marker_source_lock_and_physical_targets(self):
        for mutation in ("no-receipt", "owner-marker", "runtime-marker", "typed-marker", "runtime-lock", "source-lock",
                         "absolute", "escape", "chain", "foreign-bin", "outside-bin"):
            with self.subTest(mutation=mutation), TemporaryDirectory() as temporary:
                observer, _ = self.fixture(Path(temporary).resolve())
                runtime, state = self.author_runtime(observer)
                link = runtime / "node_modules/.bin/unit"
                if mutation == "no-receipt":
                    state["installations"][0]["data_receipts"] = {}
                    (observer.data / "state-v2.json").write_text(json.dumps(state))
                elif mutation == "owner-marker": (runtime.parent.parent / ".agentplugins-data-owner.json").write_text("{}")
                elif mutation == "runtime-marker": (runtime / ".agentplugins-runtime.json").write_text("{}")
                elif mutation == "typed-marker":
                    path = runtime / ".agentplugins-runtime.json"
                    marker = json.loads(path.read_text()); marker["schema_version"] = True
                    path.write_text(json.dumps(marker))
                elif mutation == "runtime-lock": (runtime / "package-lock.json").write_bytes(b"{}")
                elif mutation == "source-lock":
                    next((observer.data / "store").rglob("package-lock.json")).write_bytes(b"{}")
                elif mutation == "foreign-bin":
                    (observer.data / "unowned/.bin").mkdir(parents=True)
                    (observer.data / "unowned/.bin/unit").symlink_to("../../" + str(link.relative_to(observer.data)))
                elif mutation == "outside-bin": (runtime / "node_modules/unit/alias").symlink_to("bin.js")
                else:
                    link.unlink()
                    if mutation == "absolute": link.symlink_to(runtime / "node_modules/unit/bin.js")
                    if mutation == "escape": link.symlink_to("../../package.json")
                    if mutation == "chain":
                        (runtime / "node_modules/.bin/other").symlink_to("../unit/bin.js")
                        link.symlink_to("other")
                with self.assertRaises((ValueError, OSError)):
                    observer.saved_image()

    def row(self, index):
        source, identity = selected(self.fixtures, index)
        negative = index == 13 or index in COMPONENT_REFUSALS
        return {"index": index, "source": source, "identity": identity, "target": "windsurf", "kind": "plugin",
                "status": "documented-or-source-unsupported" if index == 13 else
                          "install-failed" if negative else "prepared-lifecycle-passed",
                "sentinels_preserved": True, "failed_install_registered": False,
                "failed_install_residue": False, "post_remove_identity_registered": False,
                "native_residuals": [], "listed_identity": not negative, "native_file_delta": 0,
                "native_identity_files": [], "native_component_candidates": [],
                "matched_installation_ids": [] if negative else ["fixture-installation"],
                "repeat_installation_ids": [] if negative else ["fixture-installation"],
                "repeat_verified": not negative, "removal_verified": not negative,
                "repeat_add": None, "repeat_list": None, "cleanup": None, "post_remove_list": None,
                "post_removal_leftovers": [], "repeat_native_unchanged": True,
                "repeat_lock_unchanged": True, "post_list_schema_valid": True,
                "prepared_components_only": not negative}

    def raw_list(self, index):
        source, identity = selected(self.fixtures, index)
        state = ACTIVE_SKILLS if index in ACTIVE_SLOTS else PREPARED
        client = {**state, "client_id": "windsurf", "scope": "user"}
        node = {"installation_id": "fixture-installation", "name": identity,
                "mixed_version": False, "clients": [client]}
        if source.startswith("github:"):
            node["source"] = source[len("github:"):].rsplit("@", 1)[0]
            client["package_revision"] = {"resolved_revision": source.rsplit("@", 1)[1],
                **{key: self.pins[source][key] for key in ("tree_digest", "manifest_digest")}}
        else:
            pin = self.fixtures["signed_release_pins"][source]
            node["source"] = pin["repository"] + "/" + pin["path"]
            node["directory"] = {"recorded_distribution": pin["distribution"],
                "recorded_revision": pin["revision"], "recorded_release_sequence": pin["sequence"],
                "product_id": "github" if source == "777genius/github-bridge" else source}
            client["package_revision"] = {"distribution_id": pin["distribution"],
                "resolved_revision": pin["revision"], "release_sequence": pin["sequence"],
                "tree_digest": pin["tree_digest"], "manifest_digest": pin["manifest_digest"]}
        return {"schema_version": 1, "command": "list", "result": "success",
                "data": {"installations": [node]}}

    def outcome(self, raw=None, *, code=0, stderr=""):
        return {"exit_code": code, "timeout": False, "_json": raw,
                "stdout_tail": "" if raw is None else json.dumps(raw)[-1800:],
                "stderr_tail": stderr[-1800:], "stderr_full": stderr}

    def sequence(self, observer, row):
        index, source, identity = row["index"], row["source"], row["identity"]
        scope = ["--target", "windsurf", "--scope", "user", "--format", "json"]
        add = [observer.cli, "add", source, *scope]
        listed = [observer.cli, "list", *scope]
        if index == 13 or index in COMPONENT_REFUSALS:
            diagnostic = ('agentplugins: no eligible directory release for "marketing-kit": '
                          '777genius/marketing-kit: release does not support complete target set; missing windsurf\n'
                          if index == 13 else f"agentplugins: preflight {identity}/windsurf: "
                          "devin Global supports only selected Skills and MCP components\n")
            return [(add, self.outcome(code=1, stderr=diagnostic)),
                    (listed, self.outcome({"schema_version": 1, "command": "list", "result": "success",
                                          "data": {"installations": []}}))]
        def mutation(operation, action, initial=False):
            entry = {"action": action, "alias": identity, "client_id": "windsurf",
                     "installation_id": "fixture-installation"}
            revision = self.raw_list(index)["data"]["installations"][0]["clients"][0]["package_revision"]
            if operation == "add":
                entry.update(package_digest=revision["tree_digest"], revision=revision["resolved_revision"])
            if initial:
                entry["component_support"] = [{"kind": "skill" if index in ACTIVE_SLOTS else "mcp_server",
                                                "name": identity, "support": "prepared"}]
            result = {"operation": "install" if operation == "add" else "remove",
                      "client_id": "windsurf", "installation_id": "fixture-installation", "outcome": "completed"}
            return self.outcome({"schema_version": 1, "command": "agentplugins " + operation,
                "scope": "user", "scope_selection_reason": "explicit", "result": "success", "outcome": "completed",
                "actions": [entry], "results": [] if action == "unchanged" else [result]})
        return [(add, mutation("add", "install", True)), (listed, self.outcome(self.raw_list(index))),
                (add, mutation("add", "unchanged")), (listed, self.outcome(self.raw_list(index))),
                ([observer.cli, "remove", identity, *scope], mutation("remove", "remove")),
                (listed, self.outcome({"schema_version": 1, "command": "list", "result": "success",
                                      "data": {"installations": []}}))]

    def author_saved(self, observer, row, position):
        """Independent inert producer: saved intent, binding and managed receipt."""
        if row["index"] == 13 or row["index"] in COMPONENT_REFUSALS:
            return
        data = observer.data
        if position == 0:
            node = self.raw_list(row["index"])["data"]["installations"][0]
            client = node["clients"][0]
            identity, source = row["identity"], row["source"]
            physical = identity[:51].rstrip(".-") + "-" + hashlib.sha256(node["installation_id"].encode()).hexdigest()[:12]
            active = data / "managed/clients/windsurf" / physical
            source_root = data / "store/packages/agentplugins-tree-sha256-v1" / client["package_revision"]["tree_digest"][7:] / "package"
            for name, body in self.sources[source].items():
                path = source_root / name
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_bytes(body)
            data_path, data_receipt = "", None
            if any(server.get("type") == "stdio" for server in json.loads(self.sources[source].get("mcp.json", b"{}")).get("mcpServers", {}).values()):
                data_path = str(data / "plugin-data" / physical)
                Path(data_path).mkdir(parents=True)
                receipt_id = "data_" + hashlib.sha256("\0".join((node["installation_id"], physical, "user")).encode()).hexdigest()[:24]
                data_receipt = {"data_receipt_id": receipt_id, "physical_backend_id": physical, "scope": "user",
                    "locator": data_path, "state": "owned", "ownership_digest": "sha256:" + hashlib.sha256(
                        ("agentplugins-plugin-data-v1\0" + receipt_id + "\0" + data_path).encode()).hexdigest()}
                (Path(data_path) / ".agentplugins-data-owner.json").write_text(json.dumps(data_receipt))
            with TemporaryDirectory(prefix="windsurf-unit-projection-") as temporary:
                projected = installation.canonical_projection({"source_root": str(source_root),
                    **{key: self.pins[source][key] for key in ("tree_digest", "manifest_digest")},
                    "physical_artifact_id": physical, "active_path": str(active), "data_path": data_path,
                    "cli": getattr(observer, "cli", "/fixture/agentplugins"), "cli_version": CLI_VERSION.removeprefix("agentplugins "),
                    "work_root": str(Path(temporary).resolve())})
                shutil.copytree(projected["staging_path"], active)
            binding_id = "client_" + hashlib.sha256("\0".join((node["installation_id"], "windsurf", "user", str(active))).encode()).hexdigest()[:24]
            revision = client["package_revision"]
            binding = {**copy.deepcopy(client), "client_binding_id": binding_id,
                "physical_artifact_id": physical, "target_locator": str(active),
                "native_profile_root": str(observer.global_root),
                "native_objects": [{"kind": "managed_package_directory", "object_id": "package:windsurf:" + physical,
                    "logical_name": identity, "path": str(active), "managed_digest": delivery_pin(active), "protection_class": "managed"}]}
            if data_receipt is not None:
                binding["data_receipt_id"] = data_receipt["data_receipt_id"]
            if source.startswith("github:"):
                repo, commit = source[7:].rsplit("@", 1); repo = repo.lower(); subpath = ""
                dep = f'git = "https://github.com/{repo}.git"\ncommit = "{commit}"\n'
                locked_source = {"git": {"repository": repo, "commit": commit, "requested": {"kind": "commit", "value": commit}}}
            else:
                release = self.fixtures["signed_release_pins"][source]
                repo, subpath, commit = release["repository"], release["path"], release["revision"]
                dep = f'registry = "uap"\ndistribution = "{release["distribution"]}"\nrelease = "{release["sequence"]}"\n'
                locked_source = {"registry": {"distribution_id": release["distribution"],
                    "release_sequence": release["sequence"], "repository": repo, "path": subpath, "commit": commit}}
            source_key = "github:" + repo + ":" if source.startswith("github:") else "directory:" + release["distribution"].lower()
            selector = {"kind": "commit", "value": commit} if source.startswith("github:") else {"kind": "release", "value": str(release["sequence"])}
            intent = {"schema_version": 1, "dependencies": [{"alias": identity, "source_key": source_key,
                "selector": selector}], "targets": [{"id": "windsurf", "aliases": [identity]}]}
            manifest_digest = "sha256:" + hashlib.sha256(json.dumps(intent, separators=(",", ":")).encode()).hexdigest()
            state = {"schema_version": 5, "installations": [{"installation_id": node["installation_id"], "declared_name": identity,
                "source": {"repository": repo, "package_subpath": subpath, "resolved_revision": commit, "tree_digest": revision["tree_digest"]},
                "package": {"manifest_digest": revision["manifest_digest"]}, "clients": {binding_id: binding}}]}
            if data_receipt is not None:
                state["installations"][0]["data_receipts"] = {data_receipt["data_receipt_id"]: data_receipt}
            (data / "state-v2.json").write_text(json.dumps(state))
            (data / "uap.toml").write_text(f'schema_version = 1\n[dependencies.{json.dumps(identity)}]\n' + dep
                + f'[targets]\nwindsurf = [{json.dumps(identity)}]\n')
            (data / "uap.lock").write_text(json.dumps({"lock_version": 1, "manifest_digest": manifest_digest, "packages": [{"alias": identity, "source_key": source_key,
                "source": locked_source, "targets": ["windsurf"], "digest_algorithm": "agentplugins-tree-sha256-v1",
                **{key: revision[key] for key in ("tree_digest", "manifest_digest")}}]}))
        elif position == 4:
            state = json.loads((data / "state-v2.json").read_text())
            for record in state["installations"]:
                for binding in record["clients"].values(): shutil.rmtree(binding["target_locator"])
            (data / "state-v2.json").write_text('{"schema_version":5,"installations":[]}')
            (data / "uap.toml").write_text('schema_version = 1\n[dependencies]\n[targets]\n')
            (data / "uap.lock").write_text('{"lock_version":1,"packages":[]}')

    def authored_sequence(self, observer, row, sequence):
        for position, entry in enumerate(sequence):
            self.author_saved(observer, row, position)
            yield entry

    def finish(self, observer, row, sequence=None):
        for argv, outcome in self.authored_sequence(observer, row, sequence or self.sequence(observer, row)):
            observer.observe(argv, outcome)
        return observer.finish(row, row["source"], row["index"], self.fixtures)

    def native_lifecycle(self, mutation=None, *, transport_mode="good", constructor_stage="fresh"):
        # Reuse the existing transport fixture, never its verdict: all native
        # source/receipt/inventory/custody methods below are the real verifier.
        import test_windsurf_global_skill_audit as native_fixture
        files = dict(native_fixture.SOURCE_FILES)
        files["plugin.json"] = json.dumps({"$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
            "name": "elements-of-style", "version": "1.0.0"}, separators=(",", ":")).encode()
        unit_pin = source_pin(files)
        substituted = patch.dict(vars(native_fixture), {"SOURCE_FILES": files,
            "UNIT_TREE": unit_pin["tree_digest"], "UNIT_MANIFEST": unit_pin["manifest_digest"]})
        substituted.start()
        self.addCleanup(substituted.stop)
        class CanonicalTransport(native_fixture.InertPublicAndNative):
            def author_installation(self, env, cwd):
                super().author_installation(env, cwd)
                native_fixture.write(self.managed / "plugin.json", files["plugin.json"])
                binding = self.state["installations"][0]["clients"][self.binding_id]
                binding["native_objects"][0]["managed_digest"] = native_fixture.source_digest(files)
                native_fixture.write(self.data / "state-v2.json", native_fixture.packed(self.state))
        live = native_fixture.audit
        with TemporaryDirectory(prefix="windsurf-installation-unit-") as temporary:
            root = Path(temporary).resolve()
            transport = CanonicalTransport(root, transport_mode)
            cls = live.WINDSURF_GLOBAL_AUDIT["WindsurfGlobalSkillAudit"]
            globals_copy = {**vars(live), "WINDSURF_GLOBAL_AUDIT": {
                **live.WINDSURF_GLOBAL_AUDIT, "WindsurfGlobalSkillAudit": cls}}
            holder = {}

            def real_factory(case, home, project, item, executable, version):
                runtime = cls(case, home, project, item, executable, version)
                transport.verifier = runtime
                try:
                    if constructor_stage == "after-probe-start":
                        runtime.begin_probe("before")
                    env = {"HOME": str(home), "XDG_CONFIG_HOME": str(home / ".config"),
                           "AGENTPLUGINS_HOME": str(case / "uap-data")}
                    before = native_fixture.proof.frame(runtime.dirs[runtime.home][0])
                    observer = WindsurfInstallationAudit("/fixture/agentplugins", env, project, case,
                                                         globals_copy, native_runtime=runtime)
                    self.assertEqual(before, native_fixture.proof.frame(runtime.dirs[runtime.home][0]))
                    self.assertFalse((runtime.profile / "foreign-neighbor.txt").exists())
                    holder["observer"] = observer
                    transition = runtime.after_public_transition
                    def observe_transition(phase, raw, outcome, argv):
                        transition(phase, raw, outcome, argv)
                        observer.observe(argv, outcome)
                    runtime.after_public_transition = observe_transition
                    return runtime
                except BaseException:
                    runtime.close()
                    raise

            def public_call(argv, *args, **kwargs):
                result = transport.call(argv, *args, **kwargs)
                if argv[1] == "list":
                    holder["observer"].observe(argv, result)
                return result

            original_case = live._do_case
            def completed_case(*args, **kwargs):
                row = original_case(*args, **kwargs)
                if mutation is not None:
                    mutation(row, transport)
                prior = copy.deepcopy(row)
                row["windsurf_installation_audit"] = holder["observer"].finish(
                    row, native_fixture.ITEM, 26, self.fixtures)
                self.assertEqual({k: v for k, v in row.items() if k != "windsurf_installation_audit"}, prior)
                self.assertEqual(row["windsurf_installation_audit"], holder["observer"].finish(
                    row, native_fixture.ITEM, 26, self.fixtures))
                return row

            pins = {**self.native_pins, native_fixture.ITEM: transport.expected_pin}
            with patch.dict(cls.__init__.__globals__, {
                    "BINARY_SHA": native_fixture.hashed(transport.binary.read_bytes()), "CASES": pins}),                     patch.object(installation, "GLOBAL_SKILL_CASES", pins),                     patch.dict(live.WINDSURF_GLOBAL_AUDIT, {"WindsurfGlobalSkillAudit": real_factory}),                     patch.dict(live.AGENT_BINARIES, {"devin": str(transport.binary)}, clear=True),                     patch.object(live, "call", side_effect=public_call),                     patch.dict(live.NATIVE_RESEARCH, {"run_probe": transport.run_probe}),                     patch.object(live, "_do_case", side_effect=completed_case):
                result = live.do_case("/fixture/agentplugins", "windsurf", "plugin", native_fixture.ITEM, 26,
                    root / "cases", {native_fixture.ITEM: "elements-of-style"}, client_version={
                        "status": "reported", "value": CLIENT_VERSION, "exit_code": 0})
            self.assertTrue(transport.verifier.closed)
            return result, pins

    def test_prepared_git_source_keeps_the_native_map_closed_to_four_cases(self):
        source, identity = selected(self.fixtures, 21)
        self.assertEqual(identity, "sanity")
        native_sources = {selected(self.fixtures, index)[0] for index in ACTIVE_SLOTS}
        self.assertEqual(set(installation.GLOBAL_SKILL_CASES), native_sources)
        self.assertEqual(len(native_sources), 4)
        self.assertNotIn(source, installation.GLOBAL_SKILL_CASES)
        self.assertEqual(set(installation.PREPARED_GIT_PINS), {source})
        with TemporaryDirectory() as temporary:
            observer, _ = self.fixture(Path(temporary).resolve())
            row = self.row(21)
            proof = self.finish(observer, row)
            self.assertEqual(proof["source_projection"]["tree_digest"], self.pins[source]["tree_digest"])
            self.assertEqual(proof["public_state"], PREPARED)
            self.assertFalse(proof["native_runtime_qualified"])
            self.assertNotIn("preserved_native_proof", proof)
        with self.assertRaises(ValueError):
            installation.independent_source_pin(source.rsplit("@", 1)[0] + "@" + "0" * 40, self.fixtures)

    def test_prepared_git_projection_rejects_a_different_independent_pin(self):
        source, _ = selected(self.fixtures, 21)
        with TemporaryDirectory() as temporary:
            observer, _ = self.fixture(Path(temporary).resolve())
            row = self.row(21)
            for argv, outcome in self.authored_sequence(observer, row, self.sequence(observer, row)):
                observer.observe(argv, outcome)
            bad = {source: {**self.prepared_git_pins[source], "tree_digest": "sha256:" + "0" * 64}}
            with patch.object(installation, "PREPARED_GIT_PINS", bad), self.assertRaises(ValueError):
                observer.finish(row, source, 21, self.fixtures)

    def test_all_exact_slots_preserve_prepared_active_and_five_negatives(self):
        proofs = []
        for index in range(30):
            with self.subTest(index=index), TemporaryDirectory() as temporary:
                observer, _ = self.fixture(Path(temporary).resolve())
                row = self.row(index)
                proof = self.finish(observer, row)
                row["windsurf_installation_audit"] = proof
                proofs.append(row)
                self.assertFalse(proof["native_runtime_qualified"])
                self.assertNotIn("_json", json.dumps(proof))
                self.assertNotIn("unmanaged Devin", json.dumps(proof))
                if index in ACTIVE_SLOTS:
                    self.assertEqual(proof["public_state"], ACTIVE_SKILLS)
        report = self.report(proofs)
        result = summary(report, self.fixtures, fixture_sha256=self.fixture_hash)
        self.assertTrue(result["installation_acceptance_passed"])
        self.assertEqual((result["installation_passed"], result["installation_verified_negative_coverage"],
                          result["installation_prepared_lifecycle_passed"], result["installation_active_skill_lifecycle_passed"]),
                         (25, 5, 21, 4))
        self.assertFalse(any(result[k] for k in ("native_runtime_qualified", "authentication_qualified",
                                               "model_qualified", "mcp_tools_qualified")))
        self.assertEqual(report["passed"], 0)
        self.assertFalse(result["retained_native_acceptance_passed"])

        native_row, pins = self.native_lifecycle()
        self.assertEqual(native_row["status"], "passed")  # Inert proof, not a native run.
        self.assertEqual(native_row["windsurf_installation_audit"]["observations"][4]["argv"][2],
                         native_row["matched_installation_ids"][0])
        self.assertFalse(native_row["windsurf_installation_audit"]["native_runtime_qualified"])
        combined = copy.deepcopy(report)
        combined["results"][26] = native_row
        combined["passed"] = 1
        combined["client_binary"] = native_row["windsurf_global_native_proof"]["binary"]["path"]
        with patch.object(installation, "GLOBAL_SKILL_CASES", pins):
            joined = summary(combined, self.fixtures, fixture_sha256=self.fixture_hash)
            self.assertTrue(joined["installation_acceptance_passed"])
            self.assertFalse(joined["retained_native_acceptance_passed"])
            self.assertFalse(joined["native_runtime_qualified"])
            for corruption in ("lost-proof", "flag-only", "phase", "source-pin", "uuid", "digest", "count"):
                with self.subTest(summary_corruption=corruption):
                    bad = copy.deepcopy(combined)
                    row = bad["results"][26]
                    if corruption == "lost-proof": row.pop("windsurf_global_native_proof")
                    elif corruption == "flag-only": row["windsurf_global_native_proof"] = {"native_discovery_verified": True}
                    elif corruption == "phase": row["windsurf_global_native_proof"]["observations"].pop()
                    elif corruption == "source-pin": row["windsurf_global_native_proof"]["independent_source_pin"]["tree_digest"] = "sha256:" + "a" * 64
                    elif corruption == "uuid": row["matched_installation_ids"] = ["39b5780a-ea06-41f3-9b42-41918272b261"]
                    elif corruption == "digest": row["windsurf_installation_audit"]["preserved_native_proof"]["sha256"] = "a" * 64
                    else: bad["passed"] = 0
                    verdict = summary(bad, self.fixtures, fixture_sha256=self.fixture_hash)
                    self.assertFalse(verdict["installation_acceptance_passed"])
                    self.assertFalse(verdict["retained_native_acceptance_passed"])

        for corruption in ("lost-proof", "flag-only", "phase", "source-pin", "uuid", "forged-stream", "stale-finished", "foreign-change"):
            with self.subTest(live_corruption=corruption), self.assertRaises(ValueError):
                def corrupt(row, transport):
                    if corruption == "lost-proof": row.pop("windsurf_global_native_proof")
                    elif corruption == "flag-only": row["windsurf_global_native_proof"] = {"native_discovery_verified": True}
                    elif corruption == "phase": row["windsurf_global_native_proof"]["observations"].pop()
                    elif corruption == "source-pin": row["windsurf_global_native_proof"]["independent_source_pin"]["tree_digest"] = "sha256:" + "a" * 64
                    elif corruption == "uuid": row["matched_installation_ids"] = ["39b5780a-ea06-41f3-9b42-41918272b261"]
                    elif corruption == "forged-stream": row["windsurf_global_native_proof"]["observations"][1]["stdout"]["base64"] = "e30="
                    elif corruption == "stale-finished": transport.verifier._failure("inert late custody loss")
                    else: (transport.home / "foreign-untouched.txt").write_bytes(b"changed foreign")
                self.native_lifecycle(corrupt)
        with self.assertRaises(ValueError):
            self.native_lifecycle(constructor_stage="after-probe-start")
        for index in (0, 26):
            with self.subTest(unwitnessed_passed=index), TemporaryDirectory() as temporary, self.assertRaises(ValueError):
                observer, _ = self.fixture(Path(temporary).resolve())
                row = self.row(index)
                row["status"] = "passed"
                row["windsurf_global_native_proof"] = {"native_discovery_verified": True}
                self.finish(observer, row)
        cached_native, _ = self.native_lifecycle(transport_mode="vendor-cache-small")
        self.assertEqual(cached_native["status"], "passed")
        self.assertTrue(cached_native["windsurf_global_native_proof"]["cache_exception_granted"])
        self.assertEqual(cached_native["windsurf_installation_audit"]["native_post_remove_state"], "pristine")

        public_cached, _ = self.native_lifecycle(transport_mode="public-both-cache")
        self.assertEqual(public_cached["status"], "passed")
        self.assertEqual([event["phase"] for event in public_cached["windsurf_global_native_proof"]["native_cache_evidence"]],
                         ["public-add", "public-repeat"])
        self.assertIn("preserved_native_proof", public_cached["windsurf_installation_audit"])

        failed_native, _ = self.native_lifecycle(transport_mode="native-missing")
        self.assertEqual(failed_native["status"], "prepared-lifecycle-passed")
        self.assertNotIn("preserved_native_proof", failed_native["windsurf_installation_audit"])
        failed_report = copy.deepcopy(report)
        failed_report["results"][26] = failed_native
        with patch.object(installation, "GLOBAL_SKILL_CASES", pins):
            verdict = summary(failed_report, self.fixtures, fixture_sha256=self.fixture_hash)
        self.assertTrue(verdict["installation_acceptance_passed"])
        self.assertFalse(verdict["retained_native_acceptance_passed"])

    def report(self, rows):
        return {"target": "windsurf", "kind": "plugin", "platform": "linux", "architecture": "x86_64",
                "cli": "/fixture/agentplugins", "client_binary": "/fixture/vendor/bin/devin",
                "cli_version": {"status": "reported", "value": CLI_VERSION, "exit_code": 0},
                "client_version": {"status": "reported", "value": CLIENT_VERSION, "exit_code": 0},
                "fixture_sha256": self.fixture_hash, "fixture_count": 30, "cases_attempted": 30,
                "full_coverage": True, "source_selection_complete": True, "passed": 0, "results": rows}

    def test_old_report_without_actual_global_custody_never_qualifies(self):
        self.assertFalse(summary(self.report([self.row(i) for i in range(30)]), self.fixtures,
                                 fixture_sha256=self.fixture_hash)["installation_acceptance_passed"])

    def test_actual_global_and_legacy_roots_are_observed(self):
        with TemporaryDirectory() as temporary:
            observer, env = self.fixture(Path(temporary).resolve())
            self.assertEqual(observer.global_root, Path(env["XDG_CONFIG_HOME"]) / "devin")
            self.assertEqual(observer.native_roots, (Path(env["HOME"]) / ".windsurf",
                Path(env["HOME"]) / ".devin", Path(env["XDG_CONFIG_HOME"]) / "devin"))
            row = self.row(0)
            sequence = self.sequence(observer, row)
            owned = observer.global_root / "owned-fixture.json"
            owned.write_text('{"name":"atlassian"}')
            self.author_saved(observer, row, 0)
            for argv, outcome in sequence[:4]:
                observer.observe(argv, outcome)
            owned.unlink()
            self.author_saved(observer, row, 4)
            for argv, outcome in sequence[4:]:
                observer.observe(argv, outcome)
            proof = observer.finish(row, row["source"], 0, self.fixtures)
            self.assertNotEqual(proof["initial_native_image"], proof["observations"][0]["native_image"])
            self.assertEqual(proof["initial_native_image"], proof["observations"][-1]["native_image"])

    def test_profile_environment_aliases_and_occupied_sentinel_refused(self):
        for mutation in ("HOME", "XDG_CONFIG_HOME", "AGENTPLUGINS_HOME", "global-link", "occupied"):
            with self.subTest(mutation=mutation), TemporaryDirectory() as temporary:
                root = Path(temporary).resolve()
                observer, env = self.fixture(root)
                if mutation == "global-link":
                    saved = observer.global_root.with_name("saved")
                    observer.global_root.rename(saved)
                    observer.global_root.symlink_to(saved, target_is_directory=True)
                elif mutation != "occupied":
                    env[mutation] += "-wrong"
                with self.assertRaises((ValueError, FileExistsError)):
                    WindsurfInstallationAudit(observer.cli, env, observer.project, observer.case, self.audit)

    def test_fresh_internal_profile_symlink_and_raced_substitution_never_redirect_sentinel(self):
        for raced in (False, True):
            with self.subTest(raced=raced), TemporaryDirectory() as temporary:
                root = Path(temporary).resolve()
                observer, env = self.fixture(root)
                # Fresh construction, with no occupied sentinel to mask the
                # pre-write directory refusal seen in the original regression.
                shutil.rmtree(observer.global_root)
                sibling = observer.xdg / "sibling"
                sibling.mkdir()
                original_open = os.open
                if not raced:
                    observer.global_root.symlink_to(sibling, target_is_directory=True)
                else:
                    observer.global_root.mkdir()
                def substitute(path, flags, *args, **kwargs):
                    if raced and path == "foreign-neighbor.txt":
                        observer.global_root.rename(observer.xdg / "original")
                        observer.global_root.symlink_to(sibling, target_is_directory=True)
                    return original_open(path, flags, *args, **kwargs)
                with patch.object(installation.os, "open", side_effect=substitute), self.assertRaises(ValueError):
                    WindsurfInstallationAudit(observer.cli, env, observer.project, observer.case, self.audit)
                self.assertEqual(list(sibling.iterdir()), [])

    def test_profile_directory_creation_uses_verified_xdg_descriptor_during_substitution(self):
        with TemporaryDirectory() as temporary:
            observer, env = self.fixture(Path(temporary).resolve())
            shutil.rmtree(observer.global_root)
            sibling = observer.home / "unmanaged-sibling"
            sibling.mkdir()
            original_mkdir = os.mkdir
            def substitute(path, mode=0o777, *, dir_fd=None):
                if path == "devin":
                    self.assertIsNotNone(dir_fd)
                    observer.xdg.rename(observer.home / "original-config")
                    observer.xdg.symlink_to(sibling, target_is_directory=True)
                return original_mkdir(path, mode=mode, dir_fd=dir_fd)
            with patch.object(installation.os, "mkdir", side_effect=substitute), self.assertRaises(ValueError):
                WindsurfInstallationAudit(observer.cli, env, observer.project, observer.case, self.audit)
            self.assertEqual(list(sibling.iterdir()), [])
            self.assertFalse((sibling / "devin").exists())

    def test_exact_negatives_refuse_saved_intent_and_hidden_binding_residue(self):
        for index in (13, *COMPONENT_REFUSALS):
            for mutation in ("dependency", "binding", "lock", "managed", "foreign-data"):
                with self.subTest(index=index, mutation=mutation), TemporaryDirectory() as temporary:
                    observer, _ = self.fixture(Path(temporary).resolve())
                    row = self.row(index)
                    sequence = self.sequence(observer, row)
                    # A valid exact refusal with an empty public list remains
                    # insufficient if the failed add changed private ownership.
                    if mutation == "dependency":
                        (observer.data / "uap.toml").write_text('schema_version=1\n[dependencies]\nresidue="1.0.0"\n')
                    elif mutation == "binding":
                        self.author_saved(observer, self.row(0), 0)
                    elif mutation == "lock":
                        (observer.data / "uap.lock").write_text('{"lock_version":1,"packages":[{"alias":"residue"}]}')
                    elif mutation == "foreign-data":
                        (observer.data / "unowned.json").write_text('{}')
                    else:
                        path = observer.data / "managed/hidden/package.json"
                        path.parent.mkdir(parents=True); path.write_text('{}')
                    with self.assertRaises(ValueError): self.finish(observer, row, sequence)
        with TemporaryDirectory() as temporary:
            observer, _ = self.fixture(Path(temporary).resolve())
            cache = observer.data / "store/cache/inert.json"
            cache.parent.mkdir(parents=True); cache.write_text('{}')
            (observer.data / "mutation.lock").write_bytes(b"")
            proof = self.finish(observer, self.row(13))
            self.assertEqual(proof["status"], "verified-installation-negative")

    def test_exact_negatives_allow_only_producer_cache_names_and_bounded_coordination(self):
        for index in (13, *COMPONENT_REFUSALS):
            with self.subTest(index=index), TemporaryDirectory() as temporary:
                observer, _ = self.fixture(Path(temporary).resolve())
                for name in ("directory-v1-cache.json", "discovery-v1-cache.json"):
                    (observer.data / name).write_text('{"schema_version":1,"sequence":1,"snapshot":"e30=","envelope":"e30="}')
                    (observer.data / (name + ".lock")).write_bytes(b"")
                self.assertEqual(self.finish(observer, self.row(index))["status"], "verified-installation-negative")
        for mutation in ("wrong-name", "link", "fifo"):
            with self.subTest(mutation=mutation), TemporaryDirectory() as temporary:
                observer, _ = self.fixture(Path(temporary).resolve())
                path = observer.data / "directory-v1-cache.json"
                if mutation == "wrong-name": path.with_name("directory-v1-cache.json.retained").write_text('{}')
                elif mutation == "link": path.symlink_to(observer.foreign)
                else: os.mkfifo(path)
                with self.assertRaises(ValueError): self.finish(observer, self.row(13))

    def test_exact_negatives_preserve_full_saved_authority_bytes_existence_and_metadata(self):
        empty = {"uap.toml": b'schema_version=1\n[dependencies]\n[targets]\n',
                 "uap.lock": json.dumps({"lock_version": 1, "manifest_digest": "sha256:" + hashlib.sha256(
                     b'{"schema_version":1,"dependencies":[],"targets":[]}').hexdigest(), "packages": []}).encode(),
                 "state-v2.json": b'{"schema_version":5,"installations":[]}'}
        for index in (13, *COMPONENT_REFUSALS):
            with self.subTest(index=index, mutation="untouched"), TemporaryDirectory() as temporary:
                observer, _ = self.fixture(Path(temporary).resolve(), saved=empty)
                self.assertEqual(self.finish(observer, self.row(index))["status"], "verified-installation-negative")
            for mutation in ("new-lock", "new-state", "new-manifest", "rewrite", "same-bytes-replace", "mode", "removed", "transient"):
                with self.subTest(index=index, mutation=mutation), TemporaryDirectory() as temporary:
                    observer, _ = self.fixture(Path(temporary).resolve(), saved=empty if not mutation.startswith("new-") else None)
                    row = self.row(index)
                    sequence = self.sequence(observer, row)
                    manifest = observer.data / "uap.toml"
                    if mutation == "new-lock":
                        (observer.data / "uap.lock").write_text('{"lock_version":1,"manifest_digest":"corrupt","packages":[]}')
                    elif mutation == "new-state":
                        (observer.data / "state-v2.json").write_text('{"schema_version":5,"installations":[],"unexpected":"retained"}')
                    elif mutation == "new-manifest": manifest.write_bytes(empty["uap.toml"])
                    elif mutation in ("rewrite", "transient"): manifest.write_bytes(b'unknown="injected"\n' + empty["uap.toml"])
                    elif mutation == "same-bytes-replace":
                        pending = manifest.with_name("replacement")
                        pending.write_bytes(manifest.read_bytes()); pending.replace(manifest)
                    elif mutation == "mode": manifest.chmod(0o400)
                    elif mutation == "removed": manifest.unlink()
                    observer.observe(*sequence[0])
                    if mutation == "transient": manifest.write_bytes(empty["uap.toml"])
                    observer.observe(*sequence[1])
                    with self.assertRaises(ValueError): observer.finish(row, row["source"], index, self.fixtures)

    def test_negative_cannot_erase_private_residue_before_empty_list(self):
        with TemporaryDirectory() as temporary:
            observer, _ = self.fixture(Path(temporary).resolve())
            row = self.row(13)
            sequence = self.sequence(observer, row)
            manifest = observer.data / "uap.toml"
            manifest.write_text('schema_version=1\n[dependencies]\nresidue="1.0.0"\n')
            observer.observe(*sequence[0])
            manifest.unlink()
            observer.observe(*sequence[1])
            with self.assertRaises(ValueError): observer.finish(row, row["source"], row["index"], self.fixtures)

    def forged_sequence(self, observer, row, mutation):
        for position, entry in enumerate(self.authored_sequence(observer, row, self.sequence(observer, row))):
            if position == 0:
                state_path = observer.data / "state-v2.json"
                state = json.loads(state_path.read_bytes())
                binding = next(iter(state["installations"][0]["clients"].values()))
                active = Path(binding["target_locator"])
                if mutation == "arbitrary-package":
                    shutil.rmtree(active)
                    active.mkdir()
                    (active / "plugin.json").write_text('{"name":"arbitrary","version":"0.0.0"}')
                    (active / "mcp.json").write_text('{"mcpServers":{}}')
                elif mutation == "mcp-drop":
                    (active / "mcp.json").write_text('{"mcpServers":{}}')
                elif mutation == "mcp-drift":
                    document = json.loads((active / "mcp.json").read_bytes())
                    document["mcpServers"]["unit-server"]["url"] = "https://substituted.invalid/"
                    (active / "mcp.json").write_text(json.dumps(document))
                elif mutation == "skill-drift":
                    (active / "skills/unit-skill/SKILL.md").write_text("Self-consistent unrelated Skill bytes.\n")
                elif mutation == "command-drift":
                    path = next((active / "skills").glob("*/SKILL.md"))
                    path.write_text("Self-consistent unrelated rendered command.\n")
                elif mutation == "stdio-drift":
                    document = json.loads((active / "mcp.json").read_bytes())
                    document["mcpServers"]["unit-server"]["args"][0] = "/unrelated/plugin-root"
                    (active / "mcp.json").write_text(json.dumps(document))
                else:
                    (active / "README.md").write_text("Self-consistent unrelated asset bytes.\n")
                binding["native_objects"][0]["managed_digest"] = delivery_pin(active)
                state_path.write_text(json.dumps(state))
            yield entry

    def test_self_consistent_wrong_delivery_receipt_cannot_replace_independent_source_projection(self):
        for index, mutation in ((0, "arbitrary-package"), (0, "mcp-drop"), (0, "mcp-drift"),
                                (0, "asset-drift"), (26, "skill-drift")):
            with self.subTest(index=index, mutation=mutation), TemporaryDirectory() as temporary:
                observer, _ = self.fixture(Path(temporary).resolve())
                row = self.row(index)
                for argv, outcome in self.forged_sequence(observer, row, mutation):
                    observer.observe(argv, outcome)
                saved = observer.saved_observations[0]
                binding = next(iter(saved["documents"]["state-v2.json"]["installations"][0]["clients"].values()))
                receipt = binding["native_objects"][0]
                delivery = next(iter(saved["deliveries"].values()))
                self.assertEqual(receipt["managed_digest"], delivery["digest"])
                self.assertEqual(saved["sources"][next(iter(saved["sources"]))]["digest"], self.pins[row["source"]]["tree_digest"])
                with self.assertRaises(ValueError): observer.finish(row, row["source"], index, self.fixtures)

    def test_canonical_stdio_data_cwd_and_rendered_command_projection_and_wrong_bytes(self):
        source = self.fixtures["plugin_aliases"][0]
        files = self.sources[source]
        files["mcp.json"] = json.dumps({"$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json", "mcpServers": {
            "unit-server": {"type": "stdio", "command": "node", "args": ["${PLUGIN_ROOT}/server.js", "${PLUGIN_DATA}/state"],
                "cwd": "${PLUGIN_DATA}", "env": {"UNIT_DATA": "${PLUGIN_DATA}"}}}}, separators=(",", ":")).encode()
        files["io.github.777genius.agentplugins/commands/echo.md"] = b"---\ndescription: Inert echo command\ndisable-model-invocation: false\n---\nEcho inert source bytes.\n"
        self.pins[source].update(source_pin(files))
        self.fixtures["signed_release_pins"][source].update(source_pin(files))
        for mutation in (None, "stdio-drift", "command-drift"):
            with self.subTest(mutation=mutation), TemporaryDirectory() as temporary:
                root = Path(temporary).resolve()
                cli = root / "agentplugins"
                cli.write_bytes(b"#!/bin/sh\nexit 0\n")
                cli.chmod(0o700)
                observer, _ = self.fixture(root, str(cli))
                row = self.row(0)
                iterator = self.forged_sequence(observer, row, mutation) if mutation else self.authored_sequence(observer, row, self.sequence(observer, row))
                for position, (argv, outcome) in enumerate(iterator):
                    if position == 0:
                        state = json.loads((observer.data / "state-v2.json").read_bytes())
                        binding = next(iter(state["installations"][0]["clients"].values()))
                        active = Path(binding["target_locator"])
                        self.assertTrue(list((active / "skills").glob("*/SKILL.md")))
                        config = json.loads((active / "mcp.json").read_bytes())["mcpServers"]["unit-server"]
                        if mutation is None:
                            data = state["installations"][0]["data_receipts"][binding["data_receipt_id"]]["locator"]
                            self.assertEqual(config["env"]["UNIT_DATA"], data)
                            self.assertIn(data, config["args"])
                            self.assertTrue(config["command"].startswith(str(active) + "/"))
                    observer.observe(argv, outcome)
                if mutation:
                    with self.assertRaises(ValueError): observer.finish(row, source, 0, self.fixtures)
                else:
                    proof = observer.finish(row, source, 0, self.fixtures)
                    self.assertEqual(proof["source_projection"]["tree_digest"], self.pins[source]["tree_digest"])
                    self.assertFalse(proof["native_runtime_qualified"])

    def test_producer_vcs_and_changed_checkout_cannot_authorize_another_binary(self):
        revision = "a" * 40
        base = ("fixture: go1.26.11\n\tpath\tgithub.com/777genius/plugin-kit-ai/cli/cmd/agentplugins\n"
                "\tbuild\tvcs.revision=" + revision + "\n\tbuild\tvcs.modified=false\n")
        for mutation in (None, "revision", "missing-vcs", "dirty-binary", "dirty-checkout", "untracked"):
            with self.subTest(mutation=mutation):
                built = base
                if mutation == "revision": built = built.replace(revision, "b" * 40)
                elif mutation == "missing-vcs": built = "fixture: go1.26.11\n"
                elif mutation == "dirty-binary": built = built.replace("modified=false", "modified=true")
                def command(argv, **kwargs):
                    if argv[:3] == ["go", "version", "-m"]: return SimpleNamespace(stdout=built)
                    if argv[:3] == ["git", "rev-parse", "HEAD"]: return SimpleNamespace(stdout=revision + "\n")
                    if argv[:2] == ["git", "diff"] and mutation == "dirty-checkout":
                        raise subprocess.CalledProcessError(1, argv)
                    return SimpleNamespace(stdout=b"foreign.go\n" if mutation == "untracked" else b"")
                with patch.object(installation, "producer_file", return_value={"sha256": "inert"}), \
                        patch.object(installation.subprocess, "run", side_effect=command):
                    if mutation is None:
                        self.assertEqual(VERIFY_PRODUCER("/fixture/cli")["revision"], revision)
                    else:
                        with self.assertRaises((ValueError, subprocess.CalledProcessError)):
                            VERIFY_PRODUCER("/fixture/cli")

    def test_cached_projection_helper_bytes_identity_and_child_time_changes_refuse(self):
        for mutation in ("bytes", "replace", "mode", "during-child"):
            with self.subTest(mutation=mutation), TemporaryDirectory() as temporary:
                binary = Path(temporary) / "oracle"
                binary.write_bytes(b"inert executable bytes\n")
                binary.chmod(0o700)
                original = installation.producer_file(str(binary))
                if mutation == "bytes": binary.write_bytes(b"substituted executable bytes\n")
                elif mutation == "replace":
                    pending = binary.with_name("replacement")
                    pending.write_bytes(binary.read_bytes()); pending.chmod(0o700); pending.replace(binary)
                elif mutation == "mode": binary.chmod(0o500)
                def child(*args, **kwargs):
                    binary.write_bytes(b"changed during oracle child\n")
                    return SimpleNamespace(stdout=b"{}")
                with patch.object(installation, "projection_helper", return_value=(str(binary), original)), \
                        patch.object(installation.subprocess, "run", side_effect=child) as invoked:
                    with self.assertRaises(ValueError): installation.canonical_projection({})
                    self.assertEqual(invoked.call_count, int(mutation == "during-child"))

    def test_original_cli_identity_must_survive_until_source_projection(self):
        with TemporaryDirectory() as temporary:
            observer, _ = self.fixture(Path(temporary).resolve())
            row = self.row(0)
            for argv, outcome in self.authored_sequence(observer, row, self.sequence(observer, row)):
                observer.observe(argv, outcome)
            observer.producer = {"revision": "original", "executable": {"sha256": "original"}}
            with patch.object(installation, "verify_projection_producer", return_value={"revision": "original", "executable": {"sha256": "substituted"}}):
                with self.assertRaises(ValueError): observer.finish(row, row["source"], 0, self.fixtures)

    def test_saved_binding_delivery_lock_repeat_and_removal_are_required(self):
        for mutation in ("no-saved", "manifest-missing", "lock-missing", "state-missing", "wrong-source",
                         "wrong-revision", "wrong-binding", "wrong-state", "wrong-lock", "wrong-lock-intent",
                         "receipt-digest", "wrong-locator", "missing-managed", "managed-bytes",
                         "repeat-binding", "repeat-delivery", "retained-binding", "retained-package", "retained-intent"):
            with self.subTest(mutation=mutation), TemporaryDirectory() as temporary:
                observer, _ = self.fixture(Path(temporary).resolve())
                row = self.row(0)
                sequence = self.sequence(observer, row)
                def corrupt_sequence():
                    for position, entry in enumerate(sequence):
                        if mutation != "no-saved":
                            retained_state = None
                            if position == 4 and mutation in ("retained-binding", "retained-package"):
                                retained_state = (observer.data / "state-v2.json").read_text()
                                retained_path = Path(next(iter(json.loads(retained_state)["installations"][0]["clients"].values()))["target_locator"])
                            if position == 4 and mutation == "state-missing":
                                (observer.data / "state-v2.json").write_text(saved_state)
                            self.author_saved(observer, row, position)
                            if retained_state is not None:
                                if mutation == "retained-binding":
                                    (observer.data / "state-v2.json").write_text(retained_state)
                                else:
                                    retained_path.mkdir(); (retained_path / "retained.json").write_text('{}')
                            if position == (2 if mutation.startswith("repeat-") else 0):
                                state_path = observer.data / "state-v2.json"
                                state = json.loads(state_path.read_text())
                                record = state["installations"][0]
                                binding = next(iter(record["clients"].values()))
                                saved_state = json.dumps(state)
                                if mutation.endswith("missing") and mutation != "missing-managed":
                                    (observer.data / {"manifest-missing": "uap.toml", "lock-missing": "uap.lock", "state-missing": "state-v2.json"}[mutation]).unlink()
                                elif mutation == "wrong-source": record["source"]["repository"] = "foreign/source"
                                elif mutation == "wrong-revision": binding["package_revision"]["tree_digest"] = "sha256:" + "0" * 64
                                elif mutation in ("wrong-binding", "repeat-binding"):
                                    binding["client_binding_id"] = "foreign"
                                elif mutation == "wrong-state": binding["activation"] = "active"
                                elif mutation in ("wrong-lock", "wrong-lock-intent"):
                                    lock_path = observer.data / "uap.lock"
                                    locked = json.loads(lock_path.read_text())
                                    if mutation == "wrong-lock": locked["packages"][0]["tree_digest"] = "sha256:" + "0" * 64
                                    else: locked["manifest_digest"] = "sha256:" + "0" * 64
                                    lock_path.write_text(json.dumps(locked))
                                elif mutation == "receipt-digest": binding["native_objects"][0]["managed_digest"] = "sha256:" + "0" * 64
                                elif mutation == "wrong-locator": binding["target_locator"] += "-foreign"
                                elif mutation == "missing-managed": shutil.rmtree(binding["target_locator"])
                                elif mutation in ("managed-bytes", "repeat-delivery"):
                                    (Path(binding["target_locator"]) / "mcp.json").write_text('{"changed":true}')
                                if mutation not in ("state-missing",): state_path.write_text(json.dumps(state))
                            if position == 4 and mutation == "retained-intent":
                                (observer.data / "uap.toml").write_text('schema_version=1\n[dependencies]\nresidue="1.0.0"\n')
                        yield entry
                with self.assertRaises(ValueError):
                    for argv, outcome in corrupt_sequence(): observer.observe(argv, outcome)
                    observer.finish(row, row["source"], row["index"], self.fixtures)

    def test_original_global_foreign_bytes_and_root_identity_are_pinned(self):
        for mutation in ("foreign", "global", "home", "project", "xdg", "data", "case"):
            with self.subTest(mutation=mutation), TemporaryDirectory() as temporary:
                observer, _ = self.fixture(Path(temporary).resolve())
                if mutation == "foreign":
                    observer.foreign.write_text("changed foreign")
                else:
                    path = getattr(observer, "global_root" if mutation == "global" else mutation)
                    saved = path.with_name(path.name + "-old")
                    path.rename(saved)
                    path.mkdir()
                with self.assertRaises((ValueError, AssertionError, FileNotFoundError)):
                    observer.observe(*self.sequence(observer, self.row(0))[0])

    def test_links_special_files_and_project_writes_refuse(self):
        for mutation in ("symlink", "fifo", "project"):
            with self.subTest(mutation=mutation), TemporaryDirectory() as temporary:
                observer, _ = self.fixture(Path(temporary).resolve())
                if mutation == "symlink":
                    (observer.global_root / "redirect").symlink_to(observer.foreign)
                elif mutation == "fifo":
                    os.mkfifo(observer.global_root / "pipe")
                else:
                    (observer.project / "unexpected").write_text("changed")
                with self.assertRaises((ValueError, AssertionError)):
                    observer.observe(*self.sequence(observer, self.row(0))[0])

    def test_raw_envelope_missing_wrong_scope_state_source_or_ids_refuse(self):
        for mutation in ("missing-json", "tail-only", "wrong-scope", "extra-client", "extra-row", "wrong-source",
                         "wrong-sha", "wrong-digest", "policy", "authentication", "verification", "activation",
                         "rekey", "remove-id", "remove-outcome", "add-id", "add-revision", "add-digest", "add-missing-revision", "add-missing-digest", "add-result", "result-type", "implicit-scope", "nan", "large", "timeout", "exit", "stderr"):
            with self.subTest(mutation=mutation), TemporaryDirectory() as temporary:
                observer, _ = self.fixture(Path(temporary).resolve())
                row = self.row(0)
                sequence = self.sequence(observer, row)
                outcome = sequence[1][1]
                node = outcome["_json"]["data"]["installations"][0]
                client = node["clients"][0]
                if mutation in ("missing-json", "tail-only"):
                    outcome.pop("_json")
                elif mutation == "wrong-scope": client["scope"] = "project"
                elif mutation == "extra-client": node["clients"].append(copy.deepcopy(client))
                elif mutation == "extra-row": outcome["_json"]["data"]["installations"].append(copy.deepcopy(node))
                elif mutation == "wrong-source": node["source"] += "-other"
                elif mutation == "wrong-sha": client["package_revision"]["resolved_revision"] = "0" * 40
                elif mutation == "wrong-digest": client["package_revision"]["tree_digest"] = "sha256:" + "0" * 64
                elif mutation in ("policy", "authentication", "verification", "activation"): client[mutation] = "invalid"
                elif mutation == "rekey": sequence[3][1]["_json"]["data"]["installations"][0]["installation_id"] = "new"
                elif mutation == "remove-id": sequence[4][1]["_json"]["results"][0]["installation_id"] = "foreign"
                elif mutation == "remove-outcome": sequence[4][1]["_json"]["outcome"] = "failed"
                elif mutation == "add-id": sequence[0][1]["_json"]["actions"][0]["installation_id"] = "foreign-installation"
                elif mutation == "add-revision": sequence[0][1]["_json"]["actions"][0]["revision"] = "0" * 40
                elif mutation == "add-digest": sequence[0][1]["_json"]["actions"][0]["package_digest"] = "sha256:" + "0" * 64
                elif mutation == "add-missing-revision": sequence[0][1]["_json"]["actions"][0].pop("revision")
                elif mutation == "add-missing-digest": sequence[0][1]["_json"]["actions"][0].pop("package_digest")
                elif mutation == "add-result": sequence[0][1]["_json"]["results"] = []
                elif mutation == "result-type": sequence[4][1]["_json"]["results"] = [42]
                elif mutation == "implicit-scope": sequence[0][1]["_json"]["scope_selection_reason"] = "default"
                elif mutation == "nan": outcome["_json"]["extra"] = float("nan")
                elif mutation == "large": outcome["_json"]["extra"] = "x" * (1 << 20)
                elif mutation == "timeout": outcome["timeout"] = True
                elif mutation == "exit": outcome["exit_code"] = 1
                elif mutation == "stderr": outcome["stderr_full"] = "hidden warning"
                with self.assertRaises((ValueError, AssertionError)):
                    self.finish(observer, row, sequence)

    def test_exact_actual_argv_and_command_order_required(self):
        for mutation in ("scope", "other-cli", "extra", "missing", "reorder"):
            with self.subTest(mutation=mutation), TemporaryDirectory() as temporary:
                observer, _ = self.fixture(Path(temporary).resolve())
                row = self.row(0)
                sequence = self.sequence(observer, row)
                if mutation == "scope": sequence[0][0][sequence[0][0].index("user")] = "project"
                elif mutation == "other-cli": sequence[0][0][0] = "/other/cli"
                elif mutation == "extra": sequence[0][0].append("--external-uninstalled")
                elif mutation == "missing": sequence.pop()
                else: sequence[0], sequence[1] = sequence[1], sequence[0]
                with self.assertRaises(ValueError): self.finish(observer, row, sequence)

    def test_negative_exact_complete_stderr_empty_inventory_and_all_images(self):
        for index in (13, 23, 24, 27, 28):
            for mutation in ("hidden-banner", "substring", "timeout", "exit", "registered", "global-write", "legacy-write"):
                with self.subTest(index=index, mutation=mutation), TemporaryDirectory() as temporary:
                    observer, _ = self.fixture(Path(temporary).resolve())
                    row = self.row(index)
                    sequence = self.sequence(observer, row)
                    if mutation == "hidden-banner": sequence[0][1]["stderr_full"] = "x" * 3000 + sequence[0][1]["stderr_tail"]
                    elif mutation == "substring": sequence[0][1]["stderr_full"] += "unexpected\n"
                    elif mutation == "timeout": sequence[0][1]["timeout"] = True
                    elif mutation == "exit": sequence[0][1]["exit_code"] = 2
                    elif mutation == "registered": sequence[1][1]["_json"]["data"]["installations"] = [{}]
                    else:
                        path = observer.global_root if mutation == "global-write" else observer.home / ".windsurf"
                        (path / "unexpected").write_text("native mutation")
                    with self.assertRaises(ValueError): self.finish(observer, row, sequence)

    def test_retained_empty_devin_writer_state_refuses_foreign_or_active_bytes(self):
        for skill_parent in (False, True):
            for corruption in (None, "foreign", "active-server", "nonempty-lock",
                               "nonempty-skill", "wrong-mode"):
                with self.subTest(skill_parent=skill_parent, corruption=corruption), TemporaryDirectory() as temporary:
                    observer, _ = self.fixture(Path(temporary).resolve())
                    row = self.row(0)
                    sequence = self.sequence(observer, row)
                    if skill_parent:
                        source = row["source"]
                        self.sources[source]["skills/unit-skill/SKILL.md"] = (
                            b"---\nname: unit-skill\ndescription: Inert mixed Skill\n---\nUnit bytes.\n")
                        pin = source_pin(self.sources[source])
                        self.pins[source].update(pin)
                        self.fixtures["signed_release_pins"][source].update(pin)
                        sequence = self.sequence(observer, row)
                        sequence[0][1]["_json"]["actions"][0]["component_support"].append(
                            {"kind": "skill", "name": "unit-skill", "support": "prepared"})
                    for position, (argv, outcome) in enumerate(self.authored_sequence(observer, row, sequence)):
                        if position == 0:
                            config = observer.global_root / "mcp_config.json"
                            config.write_bytes(b'{"mcpServers":{}}\n')
                            config.chmod(0o600)
                            lock = observer.global_root / "mcp_config.json.agentplugins.lock"
                            lock.write_bytes(b"")
                            lock.chmod(0o600)
                            if skill_parent:
                                (observer.global_root / "skills").mkdir(mode=0o700)
                        if position == 4:
                            if corruption == "foreign":
                                (observer.global_root / "alien-mcp.json").write_bytes(b"{}")
                            elif corruption == "active-server":
                                config.write_bytes(b'{"mcpServers":{"foreign":{"command":"other"}}}')
                            elif corruption == "nonempty-lock":
                                lock.write_bytes(b"not-empty")
                            elif corruption == "nonempty-skill":
                                skills = observer.global_root / "skills"
                                skills.mkdir(exist_ok=True)
                                (skills / "foreign").write_bytes(b"foreign")
                            elif corruption == "wrong-mode":
                                lock.chmod(0o644)
                        observer.observe(argv, outcome)
                    if corruption is None:
                        proof = observer.finish(row, row["source"], 0, self.fixtures)
                        self.assertEqual(proof["native_post_remove_state"], "empty-writer-coordination")
                        self.assertEqual(proof["public_state"], PREPARED)
                        self.assertNotEqual(proof["observations"][4]["native_image"],
                                            proof["initial_native_image"])
                    else:
                        with self.assertRaises(ValueError):
                            observer.finish(row, row["source"], 0, self.fixtures)

    def test_repeat_and_late_native_mutations_refuse(self):
        for when in ("repeat", "late"):
            with self.subTest(when=when), TemporaryDirectory() as temporary:
                observer, _ = self.fixture(Path(temporary).resolve())
                row = self.row(0)
                sequence = self.sequence(observer, row)
                for position, (argv, outcome) in enumerate(self.authored_sequence(observer, row, sequence)):
                    if when == "repeat" and position == 2:
                        (observer.global_root / "unexpected").write_text("changed during repeat")
                    observer.observe(argv, outcome)
                if when == "late":
                    (observer.global_root / "late").write_text("post-observation write")
                with self.assertRaises(ValueError) as caught:
                    observer.finish(row, row["source"], 0, self.fixtures)
                if when == "repeat":
                    changes = caught.exception.windsurf_installation_diagnostics["comparisons"]
                    self.assertEqual(changes[1]["phase"], "after-repeat-add")
                    self.assertGreater(changes[1]["total"], 0)
                    expected_key = hashlib.sha256(b"devin/unexpected").hexdigest()
                    self.assertTrue(any(entry["key_sha256"] == expected_key for entry in changes[1]["entries"]))
                    self.assertTrue(all("relative_key" not in entry for entry in changes[1]["entries"]))

    def test_unattributed_global_leftover_cannot_hide_behind_empty_identity_scanner(self):
        with TemporaryDirectory() as temporary:
            observer, _ = self.fixture(Path(temporary).resolve())
            row = self.row(0)
            sequence = self.sequence(observer, row)
            for position, (argv, outcome) in enumerate(self.authored_sequence(observer, row, sequence)):
                if position == 4:
                    (observer.global_root / "unrelated-new-file").write_text("must not survive cleanup")
                observer.observe(argv, outcome)
            self.assertEqual(row["native_residuals"], [])
            self.assertEqual(observer.observations[4][2], observer.observations[5][2])
            with self.assertRaises(ValueError): observer.finish(row, row["source"], 0, self.fixtures)

    def test_lifecycle_flags_never_override_failed_observations(self):
        for key, value in (("repeat_verified", False), ("removal_verified", False), ("sentinels_preserved", False),
                           ("failed_install_residue", True), ("native_residuals", ["unowned"]),
                           ("post_list_schema_valid", False), ("prepared_components_only", False)):
            with self.subTest(key=key), TemporaryDirectory() as temporary:
                observer, _ = self.fixture(Path(temporary).resolve())
                row = self.row(0)
                row[key] = value
                with self.assertRaises(ValueError): self.finish(observer, row)

    def completed_report(self, cli="/fixture/agentplugins"):
        rows = []
        for index in range(30):
            with TemporaryDirectory() as temporary:
                observer, _ = self.fixture(Path(temporary).resolve(), cli)
                row = self.row(index)
                row["windsurf_installation_audit"] = self.finish(observer, row)
                rows.append(row)
        return self.report(rows)

    def test_summary_refuses_tampered_row_custody_scope_and_counts(self):
        original = self.completed_report()
        self.assertTrue(summary(original, self.fixtures, fixture_sha256=self.fixture_hash)["installation_acceptance_passed"])
        for mutation in ("missing-proof", "missing-saved-proof", "source", "index-bool", "residue", "sentinel", "repeat", "global-omitted",
                         "image-count", "negative-image", "state-promotion", "writer-claim", "model-native-count",
                         "missing-row", "duplicate-row", "cli-bool"):
            with self.subTest(mutation=mutation):
                report = copy.deepcopy(original)
                row = report["results"][0]
                proof = row["windsurf_installation_audit"]
                if mutation == "missing-proof": del row["windsurf_installation_audit"]
                elif mutation == "missing-saved-proof": proof.pop("initial_saved_authority_sha256")
                elif mutation == "source": row["source"] = "other-source"
                elif mutation == "index-bool": row["index"] = False
                elif mutation == "residue": row["native_residuals"] = ["unowned"]
                elif mutation == "sentinel": row["sentinels_preserved"] = False
                elif mutation == "repeat": row["repeat_verified"] = False
                elif mutation == "global-omitted": proof["native_roots"].pop()
                elif mutation == "image-count": proof["observations"][0]["native_image"]["entries"] = False
                elif mutation == "negative-image":
                    report["results"][13]["windsurf_installation_audit"]["observations"][0]["native_image"]["sha256"] = "0" * 64
                elif mutation == "state-promotion": proof["public_state"] = dict(ACTIVE_SKILLS)
                elif mutation == "writer-claim": proof["writer_closure_claimed"] = True
                elif mutation == "model-native-count": report["passed"] = 25
                elif mutation == "missing-row": report["results"].pop()
                elif mutation == "duplicate-row": report["results"][1] = copy.deepcopy(row)
                elif mutation == "cli-bool": report["cli_version"]["exit_code"] = False
                self.assertFalse(summary(report, self.fixtures, fixture_sha256=self.fixture_hash)["installation_acceptance_passed"])

    def test_full_parsed_observations_survive_caller_consumption_without_output_export(self):
        with TemporaryDirectory() as temporary:
            observer, _ = self.fixture(Path(temporary).resolve())
            row = self.row(0)
            for argv, outcome in self.authored_sequence(observer, row, self.sequence(observer, row)):
                observer.observe(argv, outcome)
                outcome.pop("_json")
                outcome.pop("stderr_full")
            proof = observer.finish(row, row["source"], 0, self.fixtures)
            self.assertEqual(proof["status"], "verified-installation-lifecycle")
            self.assertNotIn("installations", json.dumps(proof))

    def test_observed_active_and_prepared_tuples_cannot_substitute(self):
        for index in (0, *ACTIVE_SLOTS):
            with self.subTest(index=index), TemporaryDirectory() as temporary:
                observer, _ = self.fixture(Path(temporary).resolve())
                row = self.row(index)
                sequence = self.sequence(observer, row)
                for position in (1, 3):
                    client = sequence[position][1]["_json"]["data"]["installations"][0]["clients"][0]
                    client.update(PREPARED if index in ACTIVE_SLOTS else ACTIVE_SKILLS)
                with self.assertRaises(ValueError): self.finish(observer, row, sequence)

    def integrated_native_case(self, cli, root, index, mode="good"):
        """Real combined route with complete 16/17/1/8 admitted sets and inert I/O."""
        import test_windsurf_global_skill_audit as native_fixture
        item, identity = selected(self.fixtures, index)
        pin = copy.deepcopy(native_fixture.proof.CASES[item])
        names = pin["skills"]
        bodies = {name: (f"---\nname: {name}\ndescription: {native_fixture.DESCRIPTION}\n---\nA unit-only fixture.\n").encode()
                  for name in names}
        files = {"plugin.json": native_fixture.packed({"$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
            "name": identity, "version": pin["version"]})}
        for name in names:
            files.update({f"skills/{name}/SKILL.md": bodies[name],
                          f"skills/{name}/references/note.txt": native_fixture.REFERENCE})
        for name in pin.get("rejected_skills", {}):
            files[f"skills/{name}/SKILL.md"] = (f"---\nname: {name}\ndescription: Rejected immutable source Skill\n"
                "metadata:\n  openclaw:\n    emoji: test\n---\nInert rejected Skill.\n").encode()
        if "source_skills" in pin:
            files["commands/scientific-writer-init.md"] = b"Plain upstream command, not a portable command Skill.\n"
        pin.update(source_pin(files))

        class CompleteTransport(native_fixture.InertPublicAndNative):
            def __init__(self):
                super().__init__(root, mode)
                self.repo = self.repo.lower()
                self.source_key = "github:" + self.repo + ":"

            def author_installation(self, env, cwd):
                super().author_installation(env, cwd)
                delivered = {relative: body for relative, body in files.items()
                             if not any(relative.startswith("skills/" + name + "/")
                                        for name in pin.get("rejected_skills", {}))}
                for relative, body in delivered.items():
                    native_fixture.write(self.managed / relative, body)
                installed = self.state["installations"][0]
                installed["package"]["inventory"]["skills"] = list(names)
                source = "github:" + self.repo + "@" + self.commit
                installed["source"].update(requested_source=source, canonical_source=source)
                objects = installed["clients"][self.binding_id]["native_objects"]
                objects[0]["managed_digest"] = native_fixture.source_digest(delivered)
                for name in names[1:]:
                    directory = self.home / ".config/devin/skills" / name
                    native_fixture.write(directory / "SKILL.md", bodies[name])
                    native_fixture.write(directory / "references/note.txt", native_fixture.REFERENCE)
                    stream = (b"f\0SKILL.md\0-\0" + bodies[name] + b"\0d\0references\0f\0references/note.txt\0-\0"
                              + native_fixture.REFERENCE + b"\0")
                    objects.append({"object_id": "devin_global_skill_directory:" + name,
                        "kind": "devin_global_skill_directory", "logical_name": name,
                        "path": str(directory), "source_relative": "skills/" + name,
                        "managed_digest": "sha256:" + native_fixture.hashed(stream), "protection_class": "managed"})
                native_fixture.write(self.data / "state-v2.json", native_fixture.packed(self.state))

            def add_document(self, repeated):
                raw = super().add_document(repeated)
                if not repeated:
                    raw["data"]["actions"][0]["component_support"] = [
                        {"kind": "skill", "name": name, "support": "prepared"} for name in names]
                return raw

            def call(self, argv, *args, **kwargs):
                if argv[1] == "remove":
                    for name in names[1:]:
                        native_fixture.shutil.rmtree(self.home / ".config/devin/skills" / name)
                return super().call(argv, *args, **kwargs)

            def run_probe(self, argv, cwd, env, directory, label, **options):
                record, stdout, stderr = super().run_probe(argv, cwd, env, directory, label, **options)
                if label in ("windsurf-global-add", "windsurf-global-repeat"):
                    rows = json.loads(stdout)
                    rows.extend({"name": name, "description": native_fixture.DESCRIPTION, "provider": "Devin",
                        "base_dir": str(self.home / ".config/devin/skills" / name), "display_name": name,
                        "triggers": ["user", "model"], "warnings": [], "errors": []} for name in names[1:])
                    stdout = native_fixture.packed(rows)
                    record.update(stdout_bytes=len(stdout), stdout_sha256=native_fixture.hashed(stdout))
                return record, stdout, stderr

        live = self.audit["do_case"].__globals__
        cls = live["WINDSURF_GLOBAL_AUDIT"]["WindsurfGlobalSkillAudit"]
        original_init = cls.__init__
        installation_globals = live["WINDSURF_INSTALLATION"]["summary"].__globals__
        with patch.dict(vars(native_fixture), {"ITEM": item, "SKILL": names[0], "BODY": bodies[names[0]],
                "SOURCE_FILES": files, "UNIT_TREE": pin["tree_digest"], "UNIT_MANIFEST": pin["manifest_digest"]}):
            transport = CompleteTransport()
            def retain_instance(instance, *args, **kwargs):
                original_init(instance, *args, **kwargs)
                transport.verifier = instance
            pins = {**self.native_pins, item: pin}
            with patch.dict(original_init.__globals__, {"CASES": pins,
                    "BINARY_SHA": native_fixture.hashed(transport.binary.read_bytes())}), \
                    patch.dict(installation_globals, {"GLOBAL_SKILL_CASES": pins}), \
                    patch.object(cls, "__init__", retain_instance), \
                    patch.dict(live, {"call": transport.call, "AGENT_BINARIES": {"devin": str(transport.binary)}}), \
                    patch.dict(live["NATIVE_RESEARCH"], {"run_probe": transport.run_probe}):
                row = self.audit["do_case"](cli, "windsurf", "plugin", item, index, root / "cases",
                    self.fixtures["plugin_declared_names"], self.fixtures["signed_release_pins"],
                    client_version={"status": "reported", "value": CLIENT_VERSION, "exit_code": 0},
                    installation_audit="windsurf-global-installation", installation_fixtures=self.fixtures)
            self.assertTrue(transport.verifier.closed)
            self.assertEqual(len([event for event in transport.events if event[0] == "public"]), 6)
            self.assertEqual([event for event in transport.events if event[0] == "native"],
                             [("native", phase) for phase in ("before", "add", "repeat", "remove")])
            self.assertEqual(row["windsurf_global_native_proof"]["immutable_source_store"]["complete_skills"], list(names))
            self.assertFalse((transport.home / ".config/devin/foreign-neighbor.txt").exists())
            return row, {item: pin}

    def test_real_do_case_observes_wrapped_envelopes_before_consuming_them(self):
        # Only public subprocess responses are substituted. Run the real case
        # setup, native roots, source matcher, repeat/removal and final observer.
        cli = str(Path(sys.executable).resolve())
        for index, count in ((0, 6), (22, 6), (13, 2), (23, 2)):
            with self.subTest(index=index), TemporaryDirectory() as temporary:
                expected_row = self.row(index)
                if index in ACTIVE_SLOTS:
                    row, _ = self.integrated_native_case(cli, Path(temporary).resolve(), index)
                    self.assertEqual(row["status"], "passed")  # Inert transport only.
                    proof = row["windsurf_installation_audit"]
                    self.assertEqual(proof["public_state"], ACTIVE_SKILLS)
                    self.assertEqual(len(proof["observations"]), count)
                    self.assertEqual(proof["observations"][4]["argv"][2], row["matched_installation_ids"][0])
                    self.assertTrue(row["windsurf_global_native_proof"]["native_discovery_verified"])
                    self.assertFalse(proof["native_runtime_qualified"])
                    for position, operation in enumerate(("install", "list", "repeat_add", "repeat_list", "cleanup", "post_remove_list")):
                        self.assertNotIn("_json", row[operation])
                        # Existing native callers retain complete streams; the
                        # installation observer hashes the same parsed JSON.
                        self.assertEqual(installation.digest(json.loads(row[operation]["stdout_full"]), 1 << 20),
                                         proof["observations"][position]["json_sha256"])
                        self.assertEqual(row[operation]["stderr_full"], "")
                    continue
                sequence = self.sequence(SimpleNamespace(cli=cli), expected_row)
                if count == 6:
                    # Actual add/repeat output uses the public outer envelope;
                    # bare mutation envelopes cannot drive prepared_only_add.
                    for position in (0, 2):
                        outcome = sequence[position][1]
                        outcome["_json"] = {"schema_version": 1, "command": "add", "result": "success",
                                            "data": outcome["_json"]}
                        outcome["stdout_tail"] = json.dumps(outcome["_json"])[-1800:]
                calls = []
                def public_call(argv, env, project, *args, **kwargs):
                    expected, outcome = sequence[len(calls)]
                    self.assertEqual(argv, expected)
                    self.assertIs(kwargs.get("capture_full"), True)
                    self.assertEqual((Path(env["XDG_CONFIG_HOME"]) / "devin/foreign-neighbor.txt").read_bytes(),
                                     b"unmanaged Devin Global neighbor must survive\n")
                    self.assertEqual(project, Path(temporary).resolve() / f"plugin-{index:02d}/project")
                    self.author_saved(SimpleNamespace(data=Path(env["AGENTPLUGINS_HOME"]),
                        global_root=Path(env["XDG_CONFIG_HOME"]) / "devin"), expected_row, len(calls))
                    calls.append(list(argv))
                    return copy.deepcopy(outcome)
                with patch.dict(self.audit["do_case"].__globals__, {"call": public_call}):
                    row = self.audit["do_case"](cli, "windsurf", "plugin", expected_row["source"], index,
                        Path(temporary).resolve(), self.fixtures["plugin_declared_names"],
                        self.fixtures["signed_release_pins"], installation_audit="windsurf-global-installation",
                        installation_fixtures=self.fixtures)
                self.assertEqual(len(calls), count)
                self.assertEqual(row["status"], expected_row["status"])
                proof = row["windsurf_installation_audit"]
                self.assertEqual(proof["public_state"], None if count == 2 else ACTIVE_SKILLS if index == 22 else PREPARED)
                self.assertFalse(proof["native_runtime_qualified"])
                for operation in ("install", "list", "repeat_add", "repeat_list", "cleanup", "post_remove_list"):
                    if row[operation] is not None:
                        self.assertTrue({"_json", "stdout_full", "stderr_full"}.isdisjoint(row[operation]))

    def test_real_do_case_rejects_wrong_route_before_any_public_call(self):
        with TemporaryDirectory() as temporary, patch.dict(self.audit["do_case"].__globals__, {
                "call": lambda *args, **kwargs: self.fail("wrong route executed a public command")}):
            with self.assertRaisesRegex(ValueError, "does not match"):
                self.audit["do_case"](str(Path(sys.executable).resolve()), "cursor", "plugin",
                    self.fixtures["plugin_aliases"][0], 0, Path(temporary).resolve(),
                    self.fixtures["plugin_declared_names"], self.fixtures["signed_release_pins"],
                    installation_audit="windsurf-global-installation", installation_fixtures=self.fixtures)

    def first_main_case_with_data_transition(self, mutation=None):
        """Actual main/case/observer; only version and public child I/O are inert."""
        class StopAfterFirstCase(BaseException):
            pass

        cli = str(Path(sys.executable).resolve())
        live = self.audit["main"].__globals__
        sequence = self.sequence(SimpleNamespace(cli=cli), self.row(0))
        for position in (0, 2):
            outcome = sequence[position][1]
            outcome["_json"] = {"schema_version": 1, "command": "add", "result": "success",
                                "data": outcome["_json"]}
            outcome["stdout_tail"] = json.dumps(outcome["_json"])[-1800:]
        calls, identities, transitions = [], [], []
        def identity(path):
            info = path.lstat()
            return info.st_mode, info.st_dev, info.st_ino, info.st_uid, info.st_gid

        def version(argv, env, cwd):
            if argv[1] == "version" and mutation in ("existing-mode", "symlink"):
                data = Path(cwd).parent / "plugin-00/uap-data"
                data.parent.mkdir()
                if mutation == "existing-mode":
                    data.mkdir(mode=0o755)
                    data.chmod(0o755)
                else:
                    foreign = Path(cwd).parent / "foreign-data"
                    foreign.mkdir()
                    data.symlink_to(foreign, target_is_directory=True)
            return {"status": "reported", "value": CLI_VERSION if argv[1] == "version" else CLIENT_VERSION,
                    "exit_code": 0}

        def public_call(argv, env, project, *args, **kwargs):
            if project.parent.name != "plugin-00":
                self.assertEqual(project.parent.name, "plugin-01")
                # The first row is already saved by the real main. This scoped
                # test does not run or qualify the remaining 29 native cases.
                raise StopAfterFirstCase()
            expected, outcome = sequence[len(calls)]
            self.assertEqual(argv, expected)
            self.assertIs(kwargs.get("capture_full"), True)
            data = Path(env["AGENTPLUGINS_HOME"])
            identities.append(identity(data))
            self.author_saved(SimpleNamespace(data=data, global_root=Path(env["XDG_CONFIG_HOME"]) / "devin"),
                              self.row(0), len(calls))
            calls.append(list(argv))
            if len(calls) == 1:
                if mutation == "mode":
                    data.chmod(0o755)
                elif mutation == "inode":
                    data.rename(data.with_name("saved-data"))
                    data.mkdir(mode=0o700)
                else:
                    # The real state and mutation-lock writers protect this
                    # parent before returning the first public add envelope.
                    data.chmod(0o700)
                transitions.append(identity(data))
            return copy.deepcopy(outcome)

        with TemporaryDirectory() as temporary:
            output = Path(temporary).resolve() / "report"
            oracle = installation.projection_helper(None)[0]
            original_popen = self.audit["subprocess"].Popen
            def oracle_only(argv, *args, **kwargs):
                self.assertEqual(argv, [oracle], "native/public child forbidden in this inert data-mode regression")
                return original_popen(argv, *args, **kwargs)
            previous_umask = os.umask(0o022)
            try:
                with patch.dict(live, {"call": public_call, "version_evidence": version,
                        "vendor_bootstrap_attested": lambda: True, "AGENT_BINARIES": {"devin": cli}}), \
                        patch.object(self.audit["subprocess"], "Popen", side_effect=oracle_only), \
                        patch.object(self.audit["platform"], "machine", return_value="x86_64"), \
                        patch.object(sys, "stdout", StringIO()), \
                        patch.object(sys, "argv", ["audit", "--cli", cli, "--kind", "plugin", "--target", "windsurf",
                            "--limit", "30", "--acceptance", "windsurf-global-installation", "--report-dir", str(output)]):
                    with self.assertRaises(StopAfterFirstCase):
                        self.audit["main"]()
            finally:
                os.umask(previous_umask)
            report = json.loads((output / "windsurf-plugin.json").read_text())
        self.assertEqual(report["cases_attempted"], 1)
        self.assertEqual(len(report["results"]), 1)
        self.assertFalse(report["full_coverage"])
        self.assertFalse(report["installation_acceptance_passed"])
        self.assertFalse(report["retained_native_acceptance_passed"])
        return report["results"][0], calls, identities, transitions

    def test_real_main_authors_private_data_before_first_public_add(self):
        row, calls, identities, transitions = self.first_main_case_with_data_transition()
        self.assertEqual(row["status"], "prepared-lifecycle-passed",
                         f"first data identities: {identities}; after chmod: {transitions}; {row.get('error')}")
        self.assertEqual(len(calls), 6)
        self.assertEqual(stat.S_IMODE(identities[0][0]), 0o700)
        self.assertTrue(all(value == identities[0] for value in identities + transitions))
        self.assertTrue(row["repeat_verified"] and row["removal_verified"] and row["sentinels_preserved"])
        proof = row["windsurf_installation_audit"]
        self.assertEqual(len(proof["observations"]), 6)
        self.assertEqual(proof["public_state"], PREPARED)
        self.assertFalse(proof["native_runtime_qualified"])

    def test_real_main_keeps_data_mode_inode_and_symlink_refusals(self):
        for mutation in ("mode", "inode", "existing-mode", "symlink"):
            with self.subTest(mutation=mutation):
                row, calls, identities, transitions = self.first_main_case_with_data_transition(mutation)
                self.assertEqual(row["status"], "harness-error")
                self.assertNotIn("windsurf_installation_audit", row)
                if mutation == "symlink":
                    self.assertEqual(calls, [])
                    self.assertIn("nested audit directory escapes disposable case", row["error"])
                else:
                    self.assertEqual(len(calls), 1)
                    self.assertNotEqual(identities[0], transitions[0])
                    self.assertIn("Devin Global installation evidence is unresolved", row["error"])

    def test_real_main_installation_exit_retains_native_gate_and_counts(self):
        cli = str(Path(sys.executable).resolve())
        ordinary = self.completed_report(cli)["results"]
        live = self.audit["main"].__globals__
        observer_globals = live["WINDSURF_INSTALLATION"]["summary"].__globals__
        with TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            qualified, pins = {}, dict(observer_globals["GLOBAL_SKILL_CASES"])
            for index in ACTIVE_SLOTS:
                row, selected_pin = self.integrated_native_case(cli, root, index)
                self.assertEqual(row["status"], "passed")  # Inert transport only.
                qualified[index] = row
                pins.update(selected_pin)
            failed, _ = self.integrated_native_case(cli, root / "failed-native", 26, "native-missing")
            self.assertEqual(failed["status"], "prepared-lifecycle-passed")
            binary = qualified[26]["windsurf_global_native_proof"]["binary"]["path"]
            cases = (
                ("no-native", (), None, True, False, 1),
                ("one-native", (26,), None, True, False, 1),
                ("all-four", tuple(ACTIVE_SLOTS), None, True, True, 0),
                ("failed-native", tuple(ACTIVE_SLOTS), "failed-native", True, False, 1),
                ("missing-installation", tuple(ACTIVE_SLOTS), "missing-installation", False, False, 1),
                ("missing-native", tuple(ACTIVE_SLOTS), "missing-native", False, False, 1),
                ("forged-native", tuple(ACTIVE_SLOTS), "forged-native", False, False, 1),
            )
            for name, admitted, mutation, installed, retained, expected_exit in cases:
                with self.subTest(case=name):
                    selected_rows = copy.deepcopy(ordinary)
                    for index in admitted:
                        selected_rows[index] = copy.deepcopy(qualified[index])
                    if mutation == "failed-native": selected_rows[26] = copy.deepcopy(failed)
                    elif mutation == "missing-installation": selected_rows[0].pop("windsurf_installation_audit")
                    elif mutation == "missing-native": selected_rows[26].pop("windsurf_global_native_proof")
                    elif mutation == "forged-native": selected_rows[26]["windsurf_global_native_proof"] = {"native_discovery_verified": True}
                    def do_case(*args, **kwargs):
                        self.assertEqual(kwargs.get("installation_audit"), "windsurf-global-installation")
                        self.assertEqual(kwargs.get("installation_fixtures"), self.fixtures)
                        self.assertEqual(args[3], self.fixtures["plugin_aliases"][args[4]])
                        return copy.deepcopy(selected_rows[args[4]])
                    def version(argv, *args):
                        return {"status": "reported", "value": CLI_VERSION if argv[1] == "version" else CLIENT_VERSION,
                                "exit_code": 0}
                    output = root / ("report-" + name)
                    selected_pins = dict(self.native_pins)
                    for index in admitted:
                        source = self.fixtures["plugin_aliases"][index]
                        selected_pins[source] = pins[source]
                    with patch.dict(live, {"do_case": do_case, "vendor_bootstrap_attested": lambda: True,
                            "AGENT_BINARIES": {"devin": binary}, "version_evidence": version}), \
                            patch.dict(observer_globals, {"GLOBAL_SKILL_CASES": selected_pins}), \
                            patch.object(sys, "platform", "linux"), \
                            patch.object(self.audit["platform"], "machine", return_value="x86_64"), \
                            patch.object(sys, "stdout", StringIO()), \
                            patch.object(sys, "argv", ["audit", "--cli", cli, "--kind", "plugin", "--target", "windsurf",
                                "--limit", "30", "--acceptance", "windsurf-global-installation", "--report-dir", str(output)]):
                        code = self.audit["main"]()
                    report = json.loads((output / "windsurf-plugin.json").read_text())
                    self.assertEqual(code, expected_exit)
                    self.assertEqual(report["passed"], len(admitted) - (mutation == "failed-native"))
                    self.assertEqual(report["verified_negative_coverage"], 0)
                    self.assertEqual(report["cases_attempted"], 30)
                    self.assertIs(report["installation_acceptance_passed"], installed)
                    self.assertIs(report["retained_native_acceptance_passed"], retained)
                    self.assertFalse(any(report[key] for key in ("native_runtime_qualified", "authentication_qualified",
                                                                "model_qualified", "mcp_tools_qualified")))
                    if installed:
                        self.assertEqual((report["installation_passed"], report["installation_verified_negative_coverage"]), (25, 5))
                    self.assertEqual([row["status"] for row in report["results"]],
                                     [row["status"] for row in selected_rows])

    def test_summary_requires_exact_versions_complete_selection_and_new_proof(self):
        base = self.completed_report()
        self.assertTrue(summary(base, self.fixtures, fixture_sha256=self.fixture_hash)["installation_acceptance_passed"])
        missing = copy.deepcopy(base)
        missing["results"][0].pop("windsurf_installation_audit")
        self.assertFalse(summary(missing, self.fixtures, fixture_sha256=self.fixture_hash)["installation_acceptance_passed"])
        for key, value in (("platform", "darwin"), ("architecture", "arm64"), ("target", "cursor"),
                           ("kind", "skill"), ("cases_attempted", 29), ("full_coverage", False),
                           ("fixture_sha256", "0" * 64), ("passed", 25),
                           ("client_binary", "/fixture/windsurf"),
                           ("client_version", {"status": "reported", "value": "devin 3000.12.58", "exit_code": 0})):
            with self.subTest(key=key):
                report = copy.deepcopy(base)
                report[key] = value
                self.assertFalse(summary(report, self.fixtures, fixture_sha256=self.fixture_hash)["installation_acceptance_passed"])

    def neutral_frames(self, observer):
        for name, body in (("mcp_config.json", b'{"mcpServers":{}}\n'),
                           ("mcp_config.json.agentplugins.lock", b"")):
            path = observer.global_root / name
            with path.open("xb") as stream:
                stream.write(body)
            path.chmod(0o600)
        (observer.global_root / "skills").mkdir(mode=0o700)
        frames = []
        for _ in range(6):
            image = observer.image()
            observer.infrastructure_frames.append(observer._infrastructure_frame(image))
            frames.append(image)
        return frames

    def test_drift_diagnostic_bounds_unknown_names_and_retains_all_failures(self):
        before = {
            "devin/secret-personal-old-name": "PRIVATE-OLD-CONTENT",
            "devin/secret-personal-changed-name": "PRIVATE-BEFORE",
        }
        after = {
            "devin/secret-personal-changed-name": "PRIVATE-AFTER",
            **{f"devin/secret-personal-new-{i:02d}": "PRIVATE-NEW" for i in range(20)},
        }
        diagnostic = image_drift_diagnostic(before, after)
        self.assertEqual(diagnostic["changed_entries"], 22)
        self.assertEqual(len(diagnostic["entries"]), 8)
        self.assertEqual(diagnostic["omitted_entries"], 14)
        exported = json.dumps(diagnostic)
        self.assertNotIn("secret-personal", exported)
        self.assertNotIn("PRIVATE", exported)
        self.assertTrue(all(entry["role"] == "unknown"
                            for entry in diagnostic["entries"]))
        for old, new in (
                ({"unknown": "old"}, {}),
                ({}, {"unknown": "new"}),
                ({"unknown": "old"}, {"unknown": "new"})):
            with self.subTest(before=old, after=new):
                with self.assertRaisesRegex(ValueError, "native-image drift"):
                    require_same_image(old, new, "remove/initial")

    def test_drift_diagnostic_cannot_admit_preseeded_document_inode_changes(self):
        before = {
            "devin/mcp_config.json": "same-document-digest",
            "metadata:2:mcp_config.json": (0o100600, 11, 22, 33),
        }
        after = {
            "devin/mcp_config.json": "same-document-digest",
            "metadata:2:mcp_config.json": (0o100600, 11, 23, 34),
        }
        diagnostic = image_drift_diagnostic(before, after)
        self.assertEqual(diagnostic["changed_entries"], 1)
        self.assertEqual(diagnostic["entries"][0]["role"], "mcp_document")
        self.assertEqual(diagnostic["entries"][0]["operation"], "changed")
        with self.assertRaisesRegex(ValueError, "metadata_changed=1"):
            require_same_image(before, after, "remove/initial")
        require_same_image(before, copy.deepcopy(before), "unchanged")

    def test_drift_diagnostic_identifies_known_infrastructure_without_values(self):
        after = {
            "devin/mcp_config.json": "PRIVATE-DOCUMENT-DIGEST",
            "metadata:2:mcp_config.json": (0o100600, 11, 22, 33),
            "devin/mcp_config.json.agentplugins.lock": "PRIVATE-LOCK-DIGEST",
            "metadata:2:mcp_config.json.agentplugins.lock": (0o100600, 11, 23, 34),
            "directory:2:skills": "present",
            "metadata:2:skills": (0o40700, 11, 24),
            "devin/config.json": "PRIVATE-NATIVE-CONFIG-DIGEST",
            "metadata:2:config.json": (0o100600, 11, 25, 35),
        }
        diagnostic = image_drift_diagnostic({}, after)
        self.assertIs(diagnostic["diagnostic_only"], True)
        self.assertEqual(diagnostic["changed_entries"], 8)
        self.assertEqual(diagnostic["omitted_entries"], 0)
        self.assertEqual(
            sorted(entry["role"] for entry in diagnostic["entries"]),
            sorted(["mcp_document", "writer_lock", "skills_parent", "native_config"] * 2),
        )
        for entry in diagnostic["entries"]:
            self.assertEqual(entry["operation"], "added")
            self.assertIs(entry["before_present"], False)
            self.assertIs(entry["after_present"], True)
            self.assertRegex(entry["key_sha256"], r"^[0-9a-f]{64}$")
        exported = json.dumps(diagnostic)
        self.assertNotIn("PRIVATE", exported)
        self.assertNotIn("mcp_config.json", exported)
        self.assertNotIn("metadata:", exported)

    def test_image_drift_diagnostic_is_fail_closed_and_never_leaks_names(self):
        with TemporaryDirectory() as temporary:
            observer, _ = self.fixture(Path(temporary).resolve())
            row = self.row(0)
            sequence = self.sequence(observer, row)
            secret = observer.global_root / "secret-personal-value-not-for-logs.txt"
            for index, (argv, outcome) in enumerate(self.authored_sequence(observer, row, sequence)):
                if index == 1:
                    secret.write_text("PRIVATE-TOKEN-MUST-NOT-LEAK")
                if index == 4:
                    secret.unlink()
                observer.observe(argv, outcome)
            with self.assertRaisesRegex(ValueError, "native-image drift") as context:
                observer.finish(row, row["source"], 0, self.fixtures)
            diagnostic = str(context.exception)
            self.assertIn("add/list", diagnostic)
            self.assertIn("file_added=1", diagnostic)
            self.assertNotIn("secret-personal", diagnostic)
            self.assertNotIn("PRIVATE-TOKEN", diagnostic)

    def test_empty_replacement_cannot_renew_stable_retained_skills_parent(self):
        with TemporaryDirectory() as temporary:
            observer, _ = self.fixture(Path(temporary).resolve())
            frames = self.neutral_frames(observer)
            skills = observer.global_root / "skills"
            skills.rename(observer.case / "old-skills")
            skills.mkdir(mode=0o700)
            for position in (4, 5):
                frames[position] = observer.image()
                observer.infrastructure_frames[position] = observer._infrastructure_frame(frames[position])
            self.assertEqual(frames[4], frames[5])
            self.assertEqual(list(skills.iterdir()), [])
            self.assertEqual(observer.infrastructure_frames[0][1][0], 0o40700)
            self.assertEqual(observer.infrastructure_frames[5][1][0], 0o40700)
            self.assertNotEqual(observer.infrastructure_frames[0][1][2],
                                observer.infrastructure_frames[5][1][2])
            with self.assertRaises(ValueError):
                observer._expected_removed_image(frames, mcp=True, skills=True)

    def test_neutral_document_shape_is_strict_and_whitespace_independent(self):
        valid = (b'{"mcpServers":{}}', b'{\n "mcpServers": {}\n}\n')
        invalid = (b'{}', b'[]', b'{"mcpServers":[]}', b'{"mcpServers":{"foreign":{}}}',
                   b'{"mcpServers":{},"foreign":false}', b'{"mcpServers":{},"mcpServers":{}}',
                   b'{"mcpServers":{},"foreign":NaN}', b'{"mcpServers":{}} trailing', b' ' * 4097,
                   b'\xff', b'\xef\xbb\xbf{"mcpServers":{}}') + tuple(
                       '{"mcpServers":{}}'.encode(encoding)
                       for encoding in ("utf-16", "utf-16-le", "utf-16-be",
                                        "utf-32", "utf-32-le", "utf-32-be"))
        for body in (*valid, *invalid):
            with self.subTest(body=body[:64]), TemporaryDirectory() as temporary:
                observer, _ = self.fixture(Path(temporary).resolve())
                frames = self.neutral_frames(observer)
                (observer.global_root / "mcp_config.json").write_bytes(body)
                frames[4] = observer.image()
                frames[5] = observer.image()
                if body in valid:
                    expected, _ = observer._expected_removed_image(frames, mcp=True, skills=True)
                    self.assertEqual(expected, frames[5])
                else:
                    with self.assertRaises(ValueError):
                        observer._expected_removed_image(frames, mcp=True, skills=True)

    def test_optional_capture_never_swallows_original_or_outer_guard_failures(self):
        with TemporaryDirectory() as temporary:
            observer, _ = self.fixture(Path(temporary).resolve())
            image = observer.image()
            for guards in ((ValueError("before guard"),), (None, ValueError("after guard"))):
                with self.subTest(guards=len(guards)), \
                        patch.object(observer, "guard", side_effect=guards), \
                        patch.object(observer, "_lock_frame", side_effect=ValueError("optional capture")):
                    with self.assertRaisesRegex(ValueError, "guard"):
                        observer._infrastructure_frame(image)
            (observer.global_root / "redirect").symlink_to(observer.foreign)
            with patch.object(observer, "_infrastructure_frame", side_effect=AssertionError("capture must not run")) as capture:
                with self.assertRaises(ValueError):
                    observer.observe(*self.sequence(observer, self.row(0))[0])
                capture.assert_not_called()

    def test_optional_retention_capture_cannot_reject_exact_original_restoration(self):
        for body in (b"not neutral", b"x" * 4097):
            with self.subTest(size=len(body)), TemporaryDirectory() as temporary:
                observer, _ = self.fixture(Path(temporary).resolve())
                row = self.row(0)
                lock = observer.global_root / "mcp_config.json.agentplugins.lock"
                with lock.open("xb") as stream:
                    stream.write(body)
                lock.chmod(0o600)
                for position, (argv, outcome) in enumerate(self.authored_sequence(observer, row, self.sequence(observer, row))):
                    if position == 4:
                        lock.unlink()
                    observer.observe(argv, outcome)
                proof = observer.finish(row, row["source"], 0, self.fixtures)
                self.assertEqual(proof["terminal_mode"], "restored-original-image")
                self.assertEqual(proof["retained_infrastructure"], [])
                self.assertEqual(proof["terminal_native_image"], proof["initial_native_image"])
                if len(body) > 4096:
                    self.assertIsNone(observer.infrastructure_frames[0])

    def test_retained_infrastructure_rejects_foreign_links_modes_and_replaced_authority(self):
        for mutation in ("mode", "hardlink", "symlink", "replaced-lock", "nonempty-lock",
                         "nonempty-skills", "replaced-skills", "foreign", "no-mcp", "no-skills"):
            with self.subTest(mutation=mutation), TemporaryDirectory() as temporary:
                observer, _ = self.fixture(Path(temporary).resolve())
                frames = self.neutral_frames(observer)
                lock = observer.global_root / "mcp_config.json.agentplugins.lock"
                skills = observer.global_root / "skills"
                if mutation == "mode":
                    lock.chmod(0o644)
                elif mutation == "hardlink":
                    os.link(lock, observer.case / "lock-peer")
                elif mutation == "symlink":
                    lock.unlink()
                    lock.symlink_to(observer.foreign)
                elif mutation == "replaced-lock":
                    lock.rename(observer.case / "old-lock")
                    with lock.open("xb"):
                        pass
                    lock.chmod(0o600)
                elif mutation == "nonempty-lock":
                    lock.write_bytes(b"foreign")
                elif mutation == "nonempty-skills":
                    (skills / "foreign").write_bytes(b"foreign")
                elif mutation == "replaced-skills":
                    skills.rename(observer.case / "old-skills")
                    skills.mkdir(mode=0o700)
                elif mutation == "foreign":
                    observer.foreign.write_bytes(b"changed foreign")
                with self.assertRaises(ValueError):
                    for position in (4, 5):
                        frames[position] = observer.image()
                        observer.infrastructure_frames[position] = observer._infrastructure_frame(frames[position])
                    observer._expected_removed_image(frames,
                        mcp=mutation != "no-mcp", skills=mutation != "no-skills")

    def test_retained_neutral_infrastructure_keeps_original_image_and_exact_final_image(self):
        with TemporaryDirectory() as temporary:
            observer, _ = self.fixture(Path(temporary).resolve())
            initial = copy.deepcopy(observer.initial)
            frames = self.neutral_frames(observer)
            expected, roles = observer._expected_removed_image(frames, mcp=True, skills=True)
            self.assertEqual(expected, frames[5])
            self.assertEqual(observer.initial, initial)
            self.assertEqual(roles, ["mcp_document", "skills_parent", "writer_lock"])
            self.assertNotEqual(expected, initial)
            self.assertEqual(observer.foreign.read_bytes(), b"unmanaged Devin Global neighbor must survive\n")
            (observer.global_root / "unknown").write_text("foreign")
            with self.assertRaisesRegex(ValueError, "native-image drift"):
                observer._expected_removed_image(frames, mcp=True, skills=True)

    def test_same_bytes_replacement_cannot_renew_stable_retained_lock(self):
        with TemporaryDirectory() as temporary:
            observer, _ = self.fixture(Path(temporary).resolve())
            frames = self.neutral_frames(observer)
            lock = observer.global_root / "mcp_config.json.agentplugins.lock"
            lock.rename(observer.case / "old-lock")
            with lock.open("xb"):
                pass
            lock.chmod(0o600)
            for position in (4, 5):
                frames[position] = observer.image()
                observer.infrastructure_frames[position] = observer._infrastructure_frame(frames[position])
            self.assertEqual(frames[4], frames[5])
            self.assertNotEqual(observer.infrastructure_frames[0][0][0][2],
                                observer.infrastructure_frames[5][0][0][2])
            with self.assertRaises(ValueError):
                observer._expected_removed_image(frames, mcp=True, skills=True)

    def test_same_invalid_retained_lock_never_gains_optional_proof(self):
        for body in (b"not neutral", b"x" * 4097):
            with self.subTest(size=len(body)), TemporaryDirectory() as temporary:
                observer, _ = self.fixture(Path(temporary).resolve())
                row = self.row(0)
                for name, content in (("mcp_config.json", b'{"mcpServers":{}}'),
                                      ("mcp_config.json.agentplugins.lock", body)):
                    path = observer.global_root / name
                    with path.open("xb") as stream:
                        stream.write(content)
                    path.chmod(0o600)
                for argv, outcome in self.authored_sequence(observer, row, self.sequence(observer, row)):
                    observer.observe(argv, outcome)
                with self.assertRaises(ValueError):
                    observer.finish(row, row["source"], 0, self.fixtures)

    def test_summary_requires_exact_terminal_retention_proof(self):
        report = self.completed_report()
        with TemporaryDirectory() as temporary:
            observer, _ = self.fixture(Path(temporary).resolve())
            row = self.row(0)
            for name, body in (("mcp_config.json", b'{"mcpServers":{}}\n'),
                               ("mcp_config.json.agentplugins.lock", b"")):
                path = observer.global_root / name
                with path.open("xb") as stream:
                    stream.write(body)
                path.chmod(0o600)
            row["windsurf_installation_audit"] = self.finish(observer, row)
            report["results"][0] = row
        proof = report["results"][0]["windsurf_installation_audit"]
        self.assertEqual(proof["terminal_mode"], "retained-neutral-infrastructure")
        self.assertEqual(proof["retained_infrastructure"], ["mcp_document", "writer_lock"])
        self.assertTrue(summary(report, self.fixtures, fixture_sha256=self.fixture_hash)["installation_acceptance_passed"])
        for field, value in (("terminal_mode", "restored-original-image"),
                             ("retained_infrastructure", []),
                             ("retained_infrastructure", ["writer_lock"]),
                             ("retained_infrastructure", ["mcp_document", "writer_lock", "writer_lock"]),
                             ("terminal_native_image", proof["initial_native_image"])):
            with self.subTest(field=field, value=value):
                changed = copy.deepcopy(report)
                changed["results"][0]["windsurf_installation_audit"][field] = value
                self.assertFalse(summary(changed, self.fixtures, fixture_sha256=self.fixture_hash)["installation_acceptance_passed"])

    def test_terminal_original_image_remains_valid_without_retained_infrastructure(self):
        with TemporaryDirectory() as temporary:
            observer, _ = self.fixture(Path(temporary).resolve())
            frames = [observer.image() for _ in range(6)]
            observer.infrastructure_frames = [(None, None)] * 6
            with patch.object(observer, "_neutral_file", side_effect=AssertionError("unneeded neutral read")):
                expected, roles = observer._expected_removed_image(frames, mcp=True, skills=True)
            self.assertEqual(expected, observer.initial)
            self.assertEqual(roles, [])
            (observer.global_root / "late").write_text("foreign")
            with self.assertRaisesRegex(ValueError, "native-image drift"):
                observer._expected_removed_image(frames, mcp=True, skills=True)


if __name__ == "__main__":
    unittest.main()
