"""Real filesystem controls for signed four-package Claude cache custody."""
import copy
import ast
from contextlib import ExitStack, contextmanager
import hashlib
import json
import os
from pathlib import Path
import runpy
import shutil
import sys
from tempfile import TemporaryDirectory
import unittest
from unittest.mock import patch

import marketing_kit_claude_retirement as retirement

from marketing_kit_claude_retirement import (SignedClosureRetirement, observe_project_checkpoint,
                                           project_checkpoint_summary)

ROOT = Path(__file__).resolve().parents[1]


@unittest.skipUnless(sys.platform == "linux", "Linux signed cache custody")
class SignedClosureRetirementContracts(unittest.TestCase):
    def fixture_state(self, root):
        audit = runpy.run_path(str(ROOT / "scripts/live-install-audit.py"))
        home, project = root / "home", root / "project"
        profile = home / ".claude"
        profile.mkdir(parents=True)
        (project / ".uap").mkdir(parents=True)
        (profile / "foreign-neighbor.txt").write_text("unmanaged native neighbor\n")
        pins = copy.deepcopy(audit["MARKETING_KIT_SIGNED_CLOSURE"])
        rows, records, owners, actions, results = [], [], [], [], []
        installed = {"version": 2, "plugins": {"foreign@vendor": [{"scope": "user", "foreign": True}]}}
        markets = {"vendor": {"source": {"source": "github", "repo": "foreign/vendor"}, "foreign": 1}}
        for index, (alias, distribution) in enumerate(audit["MARKETING_KIT_DISTRIBUTIONS"].items()):
            pin = pins[distribution]
            physical, installation_id, binding_id = f"physical-{index}", f"installation-{index}", f"binding-{index}"
            active = project / ".claude/skills" / physical
            bundle = active / ".agentplugins-plugin"
            payload = bundle / ".agentplugins-runtime" / "plugin.json"
            payload.parent.mkdir(parents=True)
            payload.write_text(json.dumps({"name": alias, "version": pin["package_version"]}))
            if alias == "playwright":
                # The ordinary scanner must retain its large-file diagnostic;
                # the custody reader hashes the complete bounded binary.
                (payload.parent / "runtime.bin").write_bytes(b"x" * ((1 << 20) + 1))
            marketplace = "agentplugins-" + hashlib.sha256((physical + "\0" + str(project)).encode()).hexdigest()[:12]
            cache = profile / "plugins/cache" / marketplace / alias / pin["package_version"]
            cache.parent.mkdir(parents=True)
            shutil.copytree(bundle, cache)
            revision = {"distribution_id": distribution, "version": pin["package_version"],
                        **{key: pin[key] for key in ("release_sequence", "resolved_revision", "tree_digest", "manifest_digest")}}
            # Exact signed local components qualify active installation setup;
            # the two remote MCP packages keep authentication unchecked.
            authentication = "not_required" if alias in ("marketing-kit", "playwright") else "not_checked"
            client = {"client_id": "claude", "scope": "project", "activation": "active",
                      "materialization": "materialized", "authentication": authentication,
                      "policy": "allowed", "verification": "installation_verified", "package_revision": revision}
            public = {"installation_id": installation_id, "name": alias,
                      "source": pin["repository"] + "/" + pin["package_path"], "clients": [copy.deepcopy(client)],
                      "directory": {"recorded_distribution": distribution,
                                    "recorded_revision": pin["resolved_revision"],
                                    "recorded_release_sequence": pin["release_sequence"]}}
            receipt = {"kind": "claude_project_plugin_cache", "logical_name": alias,
                       "object_id": "claude-project-cache:" + alias + "@" + marketplace,
                       "path": str(bundle), "source_relative": cache.relative_to(profile).as_posix(),
                       "protection_class": "managed", "managed_digest": "sha256:" + str(index + 1) * 64}
            binding = {**client, "client_binding_id": binding_id, "physical_artifact_id": physical,
                       "target_locator": str(active), "native_objects": [receipt],
                       "receipts": [{"phase": "committed", "client_binding_id": binding_id,
                                     "active_path": str(active), "operation_id": f"operation-{index}"}]}
            record = {"installation_id": installation_id, "declared_name": alias, "origin_mode": "directory",
                      "source": {"repository": pin["repository"], "package_subpath": pin["package_path"],
                                 "resolved_revision": pin["resolved_revision"], "tree_digest": pin["tree_digest"]},
                      "package": {"declared_name": alias, "version": pin["package_version"],
                                  "manifest_digest": pin["manifest_digest"]},
                      "directory": {"distribution_id": distribution, "desired_release_sequence": pin["release_sequence"]},
                      "clients": {binding_id: binding}}
            key = alias + "@" + marketplace
            installed["plugins"][key] = [{"scope": "project", "projectPath": str(project), "installPath": str(cache)}]
            markets[marketplace] = {"source": {"source": "directory", "path": str(active)}, "installLocation": str(active)}
            actions.append({"alias": alias, "action": "remove", "client_id": "claude",
                            "installation_id": installation_id, "binding_id": binding_id})
            results.append({"operation": "remove", "client_id": "claude", "installation_id": installation_id, "outcome": "completed"})
            rows.append(public); records.append(record)
            owners.append({"cache": cache, "active": active, "key": key, "market": marketplace})
        state = project / ".uap/state-v2.json"
        state.write_text(json.dumps({"schema_version": 5, "installations": records}))
        (profile / "plugins/installed_plugins.json").write_text(json.dumps(installed))
        (profile / "plugins/known_marketplaces.json").write_text(json.dumps(markets))
        return {"root": root, "home": home, "project": project, "profile": profile, "audit": audit,
                "pins": pins, "public": rows, "owners": owners, "state": state,
                "cleanup": {"result": "success", "outcome": "completed", "scope": "project",
                            "actions": actions, "results": results}}

    @contextmanager
    def fixture(self):
        with TemporaryDirectory() as directory, ExitStack() as resources:
            f = self.fixture_state(Path(directory).resolve())
            f["resources"] = resources
            yield f

    def capture(self, f, **kwargs):
        subject = SignedClosureRetirement(f["root"], f["home"], f["project"], f["public"], f["pins"], f["audit"], **kwargs)
        f["resources"].callback(subject.close)
        return subject

    def test_fixture_closes_capture_when_case_exits_without_finish(self):
        subject = None
        try:
            with self.fixture() as f:
                subject = self.capture(f)
                descriptors = tuple(subject.fds)
                self.assertTrue(descriptors)
            self.assertEqual(subject.fds, [])
            for descriptor in descriptors:
                with self.assertRaises(OSError):
                    os.fstat(descriptor)
        finally:
            if subject is not None:
                subject.close()

    def test_fixture_closes_capture_when_assertion_interrupts_case(self):
        subject = None
        try:
            with self.assertRaisesRegex(AssertionError, "early fixture exit"):
                with self.fixture() as f:
                    subject = self.capture(f)
                    descriptors = tuple(subject.fds)
                    self.assertTrue(descriptors)
                    raise AssertionError("early fixture exit")
            self.assertEqual(subject.fds, [])
            for descriptor in descriptors:
                with self.assertRaises(OSError):
                    os.fstat(descriptor)
        finally:
            if subject is not None:
                subject.close()

    def retire(self, f, marker=True):
        for index, owner in enumerate(f["owners"]):
            quarantine = owner["cache"].parent / (".agentplugins-cache-retire-" + f"{index + 1:032x}")
            quarantine.mkdir(mode=0o700)
            owner["moved"] = quarantine / "retired"
            owner["cache"].rename(owner["moved"])
            if marker:
                (owner["moved"] / ".orphaned_at").write_text("1794880808341")
            shutil.rmtree(owner["active"])
        installed_path = f["profile"] / "plugins/installed_plugins.json"
        installed = json.loads(installed_path.read_text())
        for owner in f["owners"]:
            del installed["plugins"][owner["key"]]
        installed_path.write_text(json.dumps(installed))
        markets_path = f["profile"] / "plugins/known_marketplaces.json"
        markets = json.loads(markets_path.read_text())
        for owner in f["owners"]:
            del markets[owner["market"]]
        markets_path.write_text(json.dumps(markets))

    def scan(self, f):
        audit, profile = f["audit"], f["profile"]
        observed = audit["snapshot"]([profile])
        identities = set(audit["MARKETING_KIT_DISTRIBUTIONS"])
        identities.update(distribution.rsplit("/", 1)[-1] for distribution in f["pins"])
        result = set()
        for alias in identities:
            result.update(audit["native_identity_residuals"]([profile], alias, observed, "claude"))
        for base, directories, _ in os.walk(profile, followlinks=False):
            for name in directories:
                if name in identities:
                    result.add(".claude/" + (Path(base) / name).relative_to(profile).as_posix() + ": directory")
        return result


    @contextmanager
    def user_diagnostic_fixture(self):
        with self.fixture() as f:
            path, _, document = self.author_passive_usage(f, temporary=False)
            document["foreign"]["private"] = "PRIVATE_VALUE"
            path.write_text(json.dumps(document))
            subject = self.capture(f)
            self.retire(f)
            subject.finish(f["cleanup"], self.scan(f), lambda: self.scan(f), keep_open=True)
            yield f, subject, path

    def user_authority_fixture(self, f, subject):
        records = json.loads(f["state"].read_text())["installations"]
        public = copy.deepcopy(f["public"])
        data = f["root"] / "data"
        data.mkdir()
        for record, view in zip(records, public):
            old_key, binding = next(iter(record["clients"].items()))
            alias = record["declared_name"]
            installation_id, binding_id = "user-" + record["installation_id"], "user-" + old_key
            active = f["profile"] / "skills" / ("user-" + binding["physical_artifact_id"])
            active.mkdir(parents=True)
            (active / "SKILL.md").write_text("owned User fixture\n")
            record["installation_id"] = view["installation_id"] = installation_id
            binding.update(scope="user", client_binding_id=binding_id,
                physical_artifact_id=active.name, target_locator=str(active), native_objects=[],
                receipts=[{"phase": "committed", "client_binding_id": binding_id,
                           "active_path": str(active), "operation_id": "user-op-" + alias}])
            record["clients"] = {binding_id: binding}
            view["clients"][0]["scope"] = "user"
        (data / "state-v2.json").write_text(json.dumps({"schema_version": 5, "installations": records}))
        authority = subject.user_initialize_authority(data, public)
        native = [{key: owner[key] for key in ("id", "version", "scope", "enabled", "installPath")}
                  for owner in authority["owners"].values()]
        receipt = {"result": "success", "outcome": "completed", "scope": "user", "actions": [], "results": []}
        for alias, owner in authority["owners"].items():
            receipt["actions"].append({"alias": alias, "action": "remove", "client_id": "claude",
                "installation_id": owner["installation_id"], "binding_id": owner["binding_id"]})
            receipt["results"].append({"operation": "remove", "client_id": "claude",
                "installation_id": owner["installation_id"], "outcome": "completed"})
        return authority, native, receipt

    def stage_user_writer(self, subject, authority, path, native, mutation=None, *, removed=False):
        clock = 1795750000000
        document = json.loads(path.read_bytes())
        if not removed:
            document["pluginUsage"].update({owner["id"]: {"usageCount": 0,
                "lastUsedAt": clock + 1, "lastUsedNumStartups": 0} for owner in authority["owners"].values()})
        if mutation is not None:
            mutation(document)
        def writer():
            replacement = path.with_name("user-initialize-replacement")
            replacement.write_text(json.dumps(document)); replacement.chmod(0o600)
            replacement.replace(path)
            return b"native capture held bytes", {"exit": 0, "error": None, "stderr_bytes": 0}
        with patch.object(retirement.time, "time_ns", side_effect=[clock * 1_000_000, (clock + 2) * 1_000_000]):
            subject.stage_user_initialize(authority, writer, native, removed=removed)
        return {"scope": "user", "home": str(subject.home), "project": str(subject.project),
                "native_rows": copy.deepcopy(native)}

    def main_user_initialize(self, subject, authority, action, native, sink):
        # Exercise the actual nested main seam without module startup or clients.
        source = getattr(self, "user_initialize_main_source", None)
        if source is None:
            source = (ROOT / "scripts/marketing-kit-project-release-e2e.py").read_text()
        main = ast.parse(source)
        owner = next(node for node in ast.walk(main) if isinstance(node, ast.FunctionDef)
                     and node.name == "claude_before_remove")
        initialize = next(node for node in ast.walk(owner) if isinstance(node, ast.FunctionDef)
                          and node.name == "initialize")
        namespace = {"authority": authority, "claude_a_retirement": subject,
                     "claude_retirement": retirement.__dict__,
                     "report": {"claude_user_initialize_diagnostics": sink}}
        exec(compile(ast.Module(body=[initialize], type_ignores=[]), "<actual-main-initialize>", "exec"), namespace)
        return namespace["initialize"](action, native)

    def diagnostic_user_writer(self, path, clock, *, checkpoint_timestamp=False, invalid_count=False):
        document = json.loads(path.read_bytes())
        stamp = clock[0] - 5 if checkpoint_timestamp else clock[0] + 1
        for key in retirement._DIAGNOSTIC_USER_KEYS:
            document["pluginUsage"][key] = {"usageCount": 1 if invalid_count else 0,
                "lastUsedAt": stamp, "lastUsedNumStartups": 0}
        replacement = path.with_name("diagnostic-initialize-replacement")
        replacement.write_text(json.dumps(document)); replacement.chmod(0o600); replacement.replace(path)
        clock[0] += 5
        return b"held native capture", {"exit": 0, "error": None, "stderr_bytes": 0}

    def test_main_user_observer_excludes_stage_and_observer_checkpoint_timestamps(self):
        with self.user_diagnostic_fixture() as (f, subject, path):
            authority, native, _ = self.user_authority_fixture(f, subject)
            sink, clock = [], [1795750000000]
            checkpoint = subject.checkpoint
            calls = []
            def checked():
                calls.append("checkpoint")
                checkpoint()
                clock[0] += 10
            with patch.object(subject, "checkpoint", side_effect=checked), \
                    patch.object(retirement.time, "time_ns", side_effect=lambda: clock[0] * 1_000_000):
                with self.assertRaises(ValueError):
                    self.main_user_initialize(subject, authority,
                        lambda: self.diagnostic_user_writer(path, clock, checkpoint_timestamp=True), native, sink)
            self.assertEqual(calls, ["checkpoint", "checkpoint"])
            self.assertEqual(len(sink), 1)
            self.assertTrue(sink[0]["clock_window_valid"])
            for row in sink[0]["after"]["fixed_fixture_keys"].values():
                self.assertFalse(row["timestamp_in_window"])
            self.assertIsNone(authority["pending"])

    def test_main_user_observer_records_successful_capture_before_stage_validation_refuses(self):
        with self.user_diagnostic_fixture() as (f, subject, path):
            authority, native, _ = self.user_authority_fixture(f, subject)
            sink, clock = [], [1795750000000]
            old = subject.passive_usage[0]
            offsets = {row["fd"]: os.lseek(row["fd"], 0, os.SEEK_CUR) for row in subject.passive_usage}
            with patch.object(retirement.time, "time_ns", side_effect=lambda: clock[0] * 1_000_000):
                with self.assertRaises(ValueError):
                    self.main_user_initialize(subject, authority,
                        lambda: self.diagnostic_user_writer(path, clock, invalid_count=True), native, sink)
            self.assertTrue(sink[0]["native_capture_returned"])
            self.assertTrue(sink[0]["pre_initialize_checkpoint_passed"])
            self.assertFalse(sink[0]["initialization_qualified"])
            self.assertIs(subject.passive_usage[0], old)
            self.assertIsNone(authority["pending"])
            self.assertEqual({fd: os.lseek(fd, 0, os.SEEK_CUR) for fd in offsets}, offsets)

    def test_main_user_observer_never_runs_capture_when_either_precheckpoint_refuses(self):
        for refused in (1, 2):
            with self.subTest(checkpoint=refused), self.user_diagnostic_fixture() as (f, subject, path):
                authority, native, _ = self.user_authority_fixture(f, subject)
                checkpoint = subject.checkpoint
                sink, calls, captures = [], [], []
                def checked():
                    calls.append("checkpoint")
                    if len(calls) == refused:
                        raise ValueError("injected precheckpoint refusal")
                    checkpoint()
                with patch.object(subject, "checkpoint", side_effect=checked):
                    with self.assertRaisesRegex(ValueError, "injected precheckpoint"):
                        self.main_user_initialize(subject, authority, lambda: captures.append(True), native, sink)
                self.assertEqual(captures, [])
                self.assertIsNone(authority["pending"])
                if refused == 1:
                    self.assertEqual(sink, [])
                else:
                    self.assertEqual(len(sink), 1)
                    self.assertFalse(sink[0]["pre_initialize_checkpoint_passed"])
                    self.assertFalse(sink[0]["native_capture_returned"])
                    self.assertFalse(sink[0]["clock_window_valid"])

    def test_user_initialize_missing_passive_baseline_refuses_before_capture_or_observer(self):
        for baseline in ("absent", "no-owned-usage"):
            with self.subTest(baseline=baseline):
                with self.fixture() as f:
                    path = f["profile"] / ".claude.json"
                    if baseline == "no-owned-usage":
                        path.write_text(json.dumps({"firstStartVersion": "2.1.263", "pluginUsage": {},
                                                    "foreign": {"preserve": "original"}}))
                        path.chmod(0o600)
                    original = path.read_bytes() if path.exists() else None
                    subject = self.capture(f)
                    self.retire(f)
                    subject.finish(f["cleanup"], self.scan(f), lambda: self.scan(f), keep_open=True)
                    self.assertEqual(subject.passive_usage, [])
                    self.assertIsNone(subject.usage_contract)
                    authority, native, _ = self.user_authority_fixture(f, subject)
                    descriptors = tuple(subject.fds)
                    offsets = {fd: os.lseek(fd, 0, os.SEEK_CUR) for fd in descriptors}
                    captures, observations = [], []
                    with self.assertRaisesRegex(ValueError,
                            r"^Claude User initialization requires a held passive \.claude\.json baseline$"):
                        subject.stage_user_initialize(authority, lambda: captures.append(True), native,
                            observer=lambda capture: observations.append(True))
                    self.assertEqual(captures, [])
                    self.assertEqual(observations, [])
                    self.assertIsNone(authority["pending"])
                    self.assertEqual(authority["phase"], "before")
                    self.assertIsNone(subject.user_usage)
                    self.assertEqual(subject.detached_usage, [])
                    self.assertEqual(tuple(subject.fds), descriptors)
                    self.assertEqual({fd: os.lseek(fd, 0, os.SEEK_CUR) for fd in descriptors}, offsets)
                    self.assertEqual(path.read_bytes() if path.exists() else None, original)
                    subject.checkpoint()
                self.assertEqual(subject.fds, [])
                for fd in descriptors:
                    with self.assertRaises(OSError):
                        os.fstat(fd)

    def test_user_initialize_receipt_bound_full_lifecycle_retains_original_descriptors(self):
        with self.user_diagnostic_fixture() as (f, subject, path):
            authority, native, receipt = self.user_authority_fixture(f, subject)
            old = subject.passive_usage[0]
            offsets = {fd: os.lseek(fd, 0, os.SEEK_CUR) for fd in subject.fds}
            proof = self.stage_user_writer(subject, authority, path, native)
            self.assertIs(subject.passive_usage[0], old)  # Pending isn't authority.
            self.assertEqual(subject.detached_usage, [])
            subject.commit_user_initialize(authority, proof)
            subject.checkpoint()
            self.assertEqual(authority["phase"], "initialized")
            self.assertEqual(os.fstat(old["fd"]).st_nlink, 0)
            self.assertEqual(len(subject.detached_usage), 1)
            original_contract = copy.deepcopy(subject.usage_contract)
            subject.user_remove_receipt(authority, receipt)
            after = self.stage_user_writer(subject, authority, path, [], removed=True)
            subject.commit_user_initialize(authority, after)
            subject.checkpoint()
            self.assertEqual(authority["phase"], "finished")
            self.assertEqual(len(subject.detached_usage), 2)
            self.assertEqual(subject.usage_contract, original_contract)
            provenance = subject.passive_usage_proof()["user_initialize_transition"]
            self.assertTrue(provenance["exact_four_user_remove_receipts_qualified"])
            self.assertEqual(provenance["receipt_bound_user_key_count"], 4)
            self.assertFalse(provenance["full_native_removal_qualified"])
            self.assertEqual({fd: os.lseek(fd, 0, os.SEEK_CUR) for fd in offsets}, offsets)
            descriptors = tuple(subject.fds)
            subject.close()
            for fd in descriptors:
                with self.assertRaises(OSError):
                    os.fstat(fd)

    def test_user_initialize_refuses_foreign_or_original_usage_and_invalid_new_rows(self):
        mutations = {
            "foreign": lambda d: d["foreign"].update(private="changed"),
            "project-row": lambda d: d["pluginUsage"].pop(next(iter(d["pluginUsage"]))),
            "missing-user": lambda d: d["pluginUsage"].pop("notion@skills-dir"),
            "extra-user": lambda d: d["pluginUsage"].update({"foreign@skills-dir": {}}),
            "boolean-count": lambda d: d["pluginUsage"]["notion@skills-dir"].update(usageCount=False),
            "startups": lambda d: d["pluginUsage"]["notion@skills-dir"].update(lastUsedNumStartups=1),
            "timestamp": lambda d: d["pluginUsage"]["notion@skills-dir"].update(lastUsedAt=1795750000100),
            "shape": lambda d: d["pluginUsage"]["notion@skills-dir"].update(extra=0),
        }
        for name, mutation in mutations.items():
            with self.subTest(name=name), self.user_diagnostic_fixture() as (f, subject, path):
                authority, native, _ = self.user_authority_fixture(f, subject)
                old = subject.passive_usage[0]
                with self.assertRaises(ValueError):
                    self.stage_user_writer(subject, authority, path, native, mutation)
                self.assertIs(subject.passive_usage[0], old)
                self.assertIsNone(authority["pending"])

    def test_user_initialize_native_binding_and_complete_remove_receipts_are_exact(self):
        for mutation in ("version", "scope", "enabled", "installPath", "missing", "duplicate"):
            with self.subTest(native=mutation), self.user_diagnostic_fixture() as (f, subject, path):
                authority, native, _ = self.user_authority_fixture(f, subject)
                if mutation == "missing": native.pop()
                elif mutation == "duplicate": native[1] = copy.deepcopy(native[0])
                else: native[0][mutation] = False if mutation == "enabled" else "foreign"
                called = []
                with self.assertRaises(ValueError):
                    subject.stage_user_initialize(authority, lambda: called.append(True), native)
                self.assertEqual(called, [])
        for mutation in ("missing", "extra", "duplicate", "binding", "installation", "scope", "outcome"):
            with self.subTest(receipt=mutation), self.user_diagnostic_fixture() as (f, subject, path):
                authority, native, receipt = self.user_authority_fixture(f, subject)
                proof = self.stage_user_writer(subject, authority, path, native)
                subject.commit_user_initialize(authority, proof)
                if mutation == "missing": receipt["actions"].pop(); receipt["results"].pop()
                elif mutation == "extra": receipt["actions"].append(copy.deepcopy(receipt["actions"][0])); receipt["results"].append(copy.deepcopy(receipt["results"][0]))
                elif mutation == "duplicate": receipt["actions"][1] = copy.deepcopy(receipt["actions"][0])
                elif mutation == "binding": receipt["actions"][0]["binding_id"] = "foreign"
                elif mutation == "installation": receipt["results"][0]["installation_id"] = "foreign"
                elif mutation == "scope": receipt["scope"] = "project"
                else: receipt["results"][0]["outcome"] = "unchanged"
                with self.assertRaises(ValueError):
                    subject.user_remove_receipt(authority, receipt)
                self.assertEqual(authority["phase"], "initialized")

    def test_user_initialize_pending_commit_checks_original_history_and_cache_anchors(self):
        for mutation in ("original-fd", "following-path", "cache", "registry", "native-proof"):
            with self.subTest(mutation=mutation), self.user_diagnostic_fixture() as (f, subject, path):
                authority, native, _ = self.user_authority_fixture(f, subject)
                old = subject.passive_usage[0]
                proof = self.stage_user_writer(subject, authority, path, native)
                if mutation == "original-fd":
                    with open('/proc/self/fd/' + str(old["fd"]), 'r+b') as stream:
                        stream.seek(0); stream.write(b' ')
                elif mutation == "following-path":
                    replacement = path.with_name("later-replacement")
                    shutil.copyfile(path, replacement); replacement.chmod(0o600); replacement.replace(path)
                elif mutation == "cache": (f["owners"][0]["moved"] / "foreign").write_text("changed")
                elif mutation == "registry": (f["profile"] / "plugins/known_marketplaces.json").write_text("{}")
                else: proof["native_rows"][0]["version"] = "foreign"
                with self.assertRaises(ValueError):
                    subject.commit_user_initialize(authority, proof)
                self.assertIs(subject.passive_usage[0], old)

    def test_user_after_remove_initializer_refuses_any_unobserved_semantic_delta(self):
        for mutation in (lambda d: d["pluginUsage"].pop("notion@skills-dir"),
                         lambda d: d["foreign"].update(private="changed")):
            with self.user_diagnostic_fixture() as (f, subject, path):
                authority, native, receipt = self.user_authority_fixture(f, subject)
                proof = self.stage_user_writer(subject, authority, path, native)
                subject.commit_user_initialize(authority, proof)
                subject.user_remove_receipt(authority, receipt)
                with self.assertRaises(ValueError):
                    self.stage_user_writer(subject, authority, path, [], mutation, removed=True)

    def test_user_initialized_original_fd_and_old_temporary_custody_remain_authoritative(self):
        for mutation in ("old-main", "old-temporary", "new-temporary", "project-row", "foreign"):
            with self.subTest(mutation=mutation), self.fixture() as f:
                path, _, _ = self.author_passive_usage(f, temporary=True)
                subject = self.capture(f)
                self.retire(f)
                subject.finish(f["cleanup"], self.scan(f), lambda: self.scan(f), keep_open=True)
                authority, native, _ = self.user_authority_fixture(f, subject)
                old = subject.passive_usage[0]
                proof = self.stage_user_writer(subject, authority, path, native)
                subject.commit_user_initialize(authority, proof)
                descriptors = tuple(subject.fds)
                if mutation == "old-main":
                    with open('/proc/self/fd/' + str(old["fd"]), 'r+b') as stream:
                        stream.write(b' ')
                elif mutation == "old-temporary": subject.passive_usage[1]["path"].write_text("{}")
                elif mutation == "new-temporary":
                    pending = path.with_name(".claude.json.tmp.123.0123456789ab")
                    shutil.copyfile(path, pending); pending.chmod(0o600)
                else:
                    document = json.loads(path.read_bytes())
                    if mutation == "foreign": document["foreign"]["private"] = "changed"
                    else: document["pluginUsage"].pop(next(iter(subject.usage_contract["rows"])))
                    path.write_text(json.dumps(document))
                with self.assertRaises(ValueError): subject.checkpoint()
                self.assertEqual(subject.fds, [])
                for fd in descriptors:
                    with self.assertRaises(OSError): os.fstat(fd)

    def test_user_initialize_refuses_reentry_and_new_temporaries_before_commit(self):
        for mutation in ("reentry", "temporary"):
            with self.subTest(mutation=mutation), self.user_diagnostic_fixture() as (f, subject, path):
                authority, native, _ = self.user_authority_fixture(f, subject)
                proof = self.stage_user_writer(subject, authority, path, native)
                if mutation == "temporary":
                    pending = path.with_name(".claude.json.tmp.123.0123456789ab")
                    shutil.copyfile(path, pending); pending.chmod(0o600)
                    with self.assertRaises(ValueError): subject.commit_user_initialize(authority, proof)
                    self.assertIsNone(subject.user_usage)
                else:
                    subject.commit_user_initialize(authority, proof)
                    with self.assertRaises(ValueError): subject.commit_user_initialize(authority, proof)
                    with self.assertRaises(ValueError): subject.stage_user_initialize(authority, lambda: self.fail("reentered"), native)

    def test_user_initialize_diagnostic_observes_without_adopting_bytes(self):
        keys = tuple(alias + "@skills-dir" for alias in ("marketing-kit", "notion", "hubspot-crm", "playwright"))
        clock = 1795750000000
        for mode in ("atomic", "inplace", "same-json"):
            with self.subTest(mode=mode), self.user_diagnostic_fixture() as (f, subject, path):
                descriptors = tuple(subject.fds)
                passive = copy.deepcopy(subject.passive_usage)
                contract = copy.deepcopy(subject.usage_contract)
                allowed = set(subject.retired_allowed)
                offsets = {row["fd"]: os.lseek(row["fd"], 0, os.SEEK_CUR)
                           for row in subject.passive_usage}
                original = path.read_bytes()
                document = json.loads(original)
                if mode != "same-json":
                    for key in keys:
                        document["pluginUsage"][key] = {"usageCount": 0, "lastUsedAt": clock + 1,
                                                       "lastUsedNumStartups": 0}
                result = object()
                def writer():
                    body = original if mode == "same-json" else json.dumps(document).encode()
                    if mode == "inplace":
                        path.write_bytes(body)
                    else:
                        replacement = path.with_name("diagnostic-pending")
                        replacement.write_bytes(body); replacement.chmod(0o600)
                        replacement.replace(path)
                    return result
                sink = []
                with patch.object(retirement.time, "time_ns", side_effect=[(clock + 1) * 1_000_000, (clock + 5) * 1_000_000]):
                    self.assertIs(retirement.observe_user_initialize_diagnostic(subject, writer, sink), result)
                self.assertEqual(tuple(subject.fds), descriptors)
                self.assertEqual(subject.passive_usage, passive)
                self.assertEqual(subject.usage_contract, contract)
                self.assertEqual(subject.retired_allowed, allowed)
                self.assertFalse(subject.finished)
                self.assertEqual({fd: os.lseek(fd, 0, os.SEEK_CUR) for fd in offsets}, offsets)
                row = sink[0]
                self.assertTrue(row["diagnostic_only"])
                self.assertEqual(row["ownership"], "ownership_unqualified")
                self.assertTrue(row["pre_initialize_checkpoint_passed"])
                self.assertTrue(row["native_capture_returned"])
                self.assertFalse(row["initialization_qualified"])
                self.assertTrue(row["foreign_json_equal"])
                self.assertEqual(row["same_named_inode"], mode == "inplace")
                self.assertEqual(row["old_fd_bytes_equal"], mode != "inplace")
                self.assertEqual(row["after"]["retirement_fd_link_count"], 1 if mode == "inplace" else 0)
                self.assertEqual(row["full_json_equal"], mode == "same-json")
                if mode != "same-json":
                    for key in keys:
                        observed = row["after"]["fixed_fixture_keys"][key]
                        for flag in ("present", "closed_row_shape", "usage_count_exact_zero",
                                     "startups_exact_zero", "timestamp_in_window"):
                            self.assertTrue(observed[flag], (key, flag))
                serialized = json.dumps(row)
                self.assertNotIn("PRIVATE_VALUE", serialized)
                self.assertNotIn(str(f["root"]), serialized)
                for owner in f["owners"]:
                    self.assertNotIn(owner["key"], serialized)
                with self.assertRaises(ValueError):
                    subject.checkpoint()  # Every changed image remains refused.

    def test_user_initialize_diagnostic_hashes_unknown_data_and_bad_rows(self):
        clock = 1795750000000
        key = "marketing-kit@skills-dir"
        for mutation in ("foreign", "unknown-key", "extra-field", "positive", "boolean", "bad-time"):
            with self.subTest(mutation=mutation), self.user_diagnostic_fixture() as (f, subject, path):
                document = json.loads(path.read_text())
                document["pluginUsage"][key] = {"usageCount": 0, "lastUsedAt": clock + 1,
                                               "lastUsedNumStartups": 0}
                if mutation == "foreign": document["PRIVATE_NAME"] = "PRIVATE_VALUE"
                elif mutation == "unknown-key": document["pluginUsage"]["PRIVATE_NAME"] = "PRIVATE_VALUE"
                elif mutation == "extra-field": document["pluginUsage"][key]["PRIVATE_NAME"] = "PRIVATE_VALUE"
                elif mutation == "positive": document["pluginUsage"][key]["usageCount"] = 1
                elif mutation == "boolean": document["pluginUsage"][key]["lastUsedNumStartups"] = False
                elif mutation == "bad-time": document["pluginUsage"][key]["lastUsedAt"] = True
                sink = []
                with patch.object(retirement.time, "time_ns", side_effect=[(clock + 1) * 1_000_000, (clock + 5) * 1_000_000]):
                    retirement.observe_user_initialize_diagnostic(subject, lambda: path.write_text(json.dumps(document)), sink)
                row = sink[0]
                if mutation in ("foreign", "unknown-key"):
                    self.assertFalse(row["foreign_json_equal"])
                observed = row["after"]["fixed_fixture_keys"][key]
                if mutation == "extra-field": self.assertFalse(observed["closed_row_shape"])
                elif mutation == "positive": self.assertFalse(observed["usage_count_exact_zero"])
                elif mutation == "boolean": self.assertFalse(observed["startups_exact_zero"])
                elif mutation == "bad-time": self.assertFalse(observed["timestamp_in_window"])
                serialized = json.dumps(row)
                self.assertNotIn("PRIVATE_NAME", serialized)
                self.assertNotIn("PRIVATE_VALUE", serialized)
                with self.assertRaises(ValueError):
                    subject.checkpoint()

    def test_user_initialize_diagnostic_failure_never_masks_writer_or_precheck(self):
        for mutation in ("writer", "reader", "projection", "precheck"):
            with self.subTest(mutation=mutation), self.user_diagnostic_fixture() as (f, subject, path):
                marker = RuntimeError("PRIVATE_VALUE")
                calls, sink = [], []
                def writer():
                    calls.append(True)
                    document = json.loads(path.read_text())
                    document["foreign"]["changed"] = "PRIVATE_VALUE"
                    path.write_text(json.dumps(document))
                    raise marker
                if mutation == "precheck":
                    path.write_text(path.read_text() + " ")
                if mutation == "reader":
                    original = retirement._diagnostic_named_image
                    def unavailable(*args, **kwargs):
                        if calls and "held_fd" not in kwargs:
                            raise OSError("PRIVATE_NAME PRIVATE_VALUE")
                        return original(*args, **kwargs)
                    context = patch.object(retirement, "_diagnostic_named_image", side_effect=unavailable)
                elif mutation == "projection":
                    context = patch.object(retirement, "_diagnostic_digest", side_effect=ValueError("PRIVATE_NAME"))
                else:
                    context = patch.dict({}, {})
                with context:
                    with self.assertRaises(ValueError if mutation == "precheck" else RuntimeError) as raised:
                        retirement.observe_user_initialize_diagnostic(subject, writer, sink)
                if mutation != "precheck":
                    self.assertIs(raised.exception, marker)
                    self.assertEqual(calls, [True])
                    with self.assertRaises(ValueError): subject.checkpoint()
                else:
                    self.assertEqual(calls, [])
                    self.assertFalse(sink[0]["pre_initialize_checkpoint_passed"])
                    self.assertFalse(sink[0]["native_capture_returned"])
                if mutation == "reader": self.assertEqual(sink[0]["after"]["status"], "unavailable")
                if mutation == "projection": self.assertEqual(sink[0]["status"], "unavailable")
                self.assertNotIn("PRIVATE_NAME", json.dumps(sink))
                self.assertNotIn("PRIVATE_VALUE", json.dumps(sink))

    def test_named_diagnostic_survives_missing_or_unreadable_old_descriptor(self):
        for missing in (False, True):
            with self.subTest(missing=missing), self.fixture() as f:
                if not missing:
                    path, _, _ = self.author_passive_usage(f, temporary=False)
                else:
                    path = f["profile"] / ".claude.json"
                subject = self.capture(f)
                self.retire(f)
                subject.finish(f["cleanup"], self.scan(f), lambda: self.scan(f), keep_open=True)
                old_fds = {row["fd"] for row in subject.passive_usage}
                original = retirement._diagnostic_image
                def read(fd, audit):
                    if fd in old_fds:
                        raise OSError("PRIVATE unavailable old descriptor")
                    return original(fd, audit)
                def writer():
                    path.write_text('{"pluginUsage":{},"firstStartVersion":"2.1.263"}')
                    path.chmod(0o600)
                sink = []
                capture = retirement._diagnostic_capture
                def unavailable_old_fd(subject):
                    # Only diagnostic reads fail; the preceding authority
                    # checkpoint still exercises the real shared same-FD reader.
                    with patch.object(retirement, "_diagnostic_image", side_effect=read):
                        return capture(subject)
                with patch.object(retirement, "_diagnostic_capture", side_effect=unavailable_old_fd):
                    retirement.observe_user_initialize_diagnostic(subject, writer, sink)
                row = sink[0]
                self.assertEqual(row["after"]["status"], "observed")
                self.assertTrue(row["after"]["first_start_version_matches"])
                for key in ("retirement_fd_raw_sha256", "retirement_fd_link_count",
                            "named_joins_retirement_fd", "retirement_image_equal"):
                    self.assertIsNone(row["after"][key], key)
                self.assertIsNone(row["old_fd_same_inode"])
                self.assertIsNone(row["old_fd_bytes_equal"])
                self.assertNotIn("PRIVATE", json.dumps(row))
                if not missing:
                    with self.assertRaises(ValueError):
                        subject.checkpoint()
                else:
                    self.assertEqual(subject.passive_usage, [])

    def test_named_diagnostic_uses_its_fd_and_refuses_foreign_body_input(self):
        with self.fixture() as f:
            path, _, _ = self.author_passive_usage(f, temporary=False)
            expected = path.read_bytes()
            forged = expected.replace(b'"2.1.263"', b'"9.9.999"')
            self.assertEqual(len(expected), len(forged))
            shadow, parked = f["home"] / "shadow-profile", f["home"] / "parked-profile"
            shadow.mkdir()
            (shadow / path.name).write_bytes(forged)
            (shadow / path.name).chmod(0o600)
            identity = retirement._CUSTODY["_FILES"]["identity"]
            original_identity = identity(path.lstat())
            fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW)
            try:
                f["profile"].rename(parked)
                shadow.rename(f["profile"])
                try:
                    body, error = f["audit"]["copilot_settings_read"](path, f["root"])
                finally:
                    f["profile"].rename(shadow)
                    parked.rename(f["profile"])
                self.assertIsNone(error)
                self.assertEqual(body.encode(), forged)
                self.assertEqual(identity(os.fstat(fd)), original_identity)
                self.assertEqual(identity(path.lstat()), original_identity)
                image = retirement._diagnostic_named_image(path, f["root"], f["audit"])
                self.assertEqual(image["sha256"], hashlib.sha256(expected).hexdigest())
                self.assertNotEqual(image["sha256"], hashlib.sha256(forged).hexdigest())
                with self.assertRaises(TypeError):
                    retirement._diagnostic_image(fd, f["audit"], forged)
            finally:
                os.close(fd)

    def test_named_and_temporary_diagnostics_refuse_real_ancestor_aba_without_foreign_reads(self):
        for temporary in (False, True):
            with self.subTest(temporary=temporary), self.fixture() as f:
                main, pending, _ = self.author_passive_usage(f)
                subject = self.capture(f)
                path = pending if temporary else main
                expected = path.read_bytes()
                shadow, parked = f["home"] / "shadow-profile", f["home"] / "parked-profile"
                shadow.mkdir()
                foreign = expected.replace(b'"2.1.263"', b'"9.9.999"')
                (shadow / path.name).write_bytes(foreign)
                (shadow / path.name).chmod(0o600)
                second = pending.with_name(".claude.json.tmp.139.710615a62269")
                if temporary:
                    second.write_bytes(expected); second.chmod(0o600)
                original_open, original_pread = os.open, os.pread
                opened, read_bodies, redirected = [], [], []
                home_identity = (f["home"].stat().st_dev, f["home"].stat().st_ino)
                shadow_identity = (shadow.stat().st_dev, shadow.stat().st_ino)
                descriptors = tuple(subject.fds)
                offsets = {fd: os.lseek(fd, 0, os.SEEK_CUR) for fd in descriptors}
                def open_aba(name, *args, **kwargs):
                    parent = kwargs.get("dir_fd")
                    if (name == f["profile"].name and args[0] & os.O_DIRECTORY
                            and parent is not None
                            and (os.fstat(parent).st_dev, os.fstat(parent).st_ino) == home_identity):
                        f["profile"].rename(parked)
                        shadow.rename(f["profile"])
                        try:
                            fd = original_open(name, *args, **kwargs)
                            info = os.fstat(fd)
                            redirected.append((info.st_dev, info.st_ino))
                        finally:
                            f["profile"].rename(shadow)
                            parked.rename(f["profile"])
                    else:
                        fd = original_open(name, *args, **kwargs)
                    opened.append(fd)
                    return fd
                def pread(fd, length, offset):
                    body = original_pread(fd, length, offset)
                    read_bodies.append(body)
                    return body
                with (
                    patch.object(os, "open", side_effect=open_aba),
                    patch.object(os, "pread", side_effect=pread),
                ):
                    if temporary:
                        with self.assertRaisesRegex(ValueError, "passive usage temporary bound") as caught:
                            subject.require_temporary_bound([pending, second], "capture")
                        detail = caught.exception.claude_temporary_bound_diagnostic
                        self.assertTrue(all(row["status"] == "unavailable" for row in detail["members"]))
                        self.assertNotIn(hashlib.sha256(foreign).hexdigest(), json.dumps(detail))
                    else:
                        with self.assertRaises(ValueError):
                            retirement._diagnostic_named_image(path, f["root"], f["audit"])
                self.assertEqual(redirected, [shadow_identity] * (2 if temporary else 1))
                self.assertEqual(read_bodies, [])
                self.assertTrue(opened)
                self.assertEqual(tuple(subject.fds), descriptors)
                self.assertEqual({fd: os.lseek(fd, 0, os.SEEK_CUR) for fd in descriptors}, offsets)
                for fd in opened:
                    with self.assertRaises(OSError):
                        os.fstat(fd)

    def test_user_diagnostic_unknown_baseline_is_null_and_checkpoint_time_is_excluded(self):
        clock = 1795750000000
        key = "marketing-kit@skills-dir"
        for failed_before in (False, True):
            with self.subTest(failed_before=failed_before), self.user_diagnostic_fixture() as (f, subject, path):
                original_read = retirement._diagnostic_named_image
                reads = []
                def read(*args, **kwargs):
                    if "held_fd" in kwargs:
                        return original_read(*args, **kwargs)
                    reads.append(True)
                    if failed_before and len(reads) == 1:
                        raise ValueError("PRIVATE baseline failure")
                    return original_read(*args, **kwargs)
                checkpoint = subject.checkpoint
                def checked():
                    checkpoint()
                    document = json.loads(path.read_text())
                    document["pluginUsage"][key] = {"usageCount": 0, "lastUsedAt": clock + 1,
                                                    "lastUsedNumStartups": 0}
                    path.write_text(json.dumps(document))
                sink = []
                with (
                    patch.object(retirement, "_diagnostic_named_image", side_effect=read),
                    patch.object(subject, "checkpoint", side_effect=checked),
                    patch.object(retirement.time, "time_ns", side_effect=[
                        (clock + 3) * 1_000_000, (clock + 5) * 1_000_000]),
                ):
                    retirement.observe_user_initialize_diagnostic(subject, lambda: None, sink)
                row = sink[0]["after"]["fixed_fixture_keys"][key]
                self.assertTrue(row["present"])
                self.assertIsNone(row["row_unchanged"])
                self.assertFalse(row["timestamp_in_window"])
                self.assertTrue(sink[0]["clock_window_valid"])
                self.assertNotIn("PRIVATE", json.dumps(sink))

    def test_user_diagnostic_has_no_initialize_window_when_checkpoint_refuses(self):
        with self.user_diagnostic_fixture() as (f, subject, path):
            marker, sink = ValueError("PRIVATE original checkpoint failure"), []
            with (
                patch.object(subject, "checkpoint", side_effect=marker),
                patch.object(retirement.time, "time_ns", return_value=2000000),
            ):
                with self.assertRaises(ValueError) as caught:
                    retirement.observe_user_initialize_diagnostic(
                        subject, lambda: self.fail("action must not run"), sink)
            self.assertIs(caught.exception, marker)
            self.assertFalse(sink[0]["pre_initialize_checkpoint_passed"])
            self.assertFalse(sink[0]["native_capture_returned"])
            self.assertFalse(sink[0]["clock_window_valid"])
            for phase in ("before", "after"):
                self.assertEqual(sink[0][phase]["status"], "observed")
                for row in sink[0][phase]["fixed_fixture_keys"].values():
                    self.assertIsNone(row["timestamp_in_window"])
            self.assertNotIn("PRIVATE", json.dumps(sink))

    def test_user_diagnostic_clock_flag_uses_only_the_actual_action_interval(self):
        clock = 1795750000000
        key = "marketing-kit@skills-dir"
        for before_ms, action_ms, end_ms, valid in ((0, 3, 2, False), (0, 2, 2, True), (3, 1, 2, True)):
            with self.subTest(action_ms=action_ms, end_ms=end_ms, before_ms=before_ms), \
                    self.user_diagnostic_fixture() as (f, subject, path):
                current = [clock + before_ms]
                checkpoint = subject.checkpoint
                def checked():
                    checkpoint()
                    current[0] = clock + action_ms
                def action():
                    document = json.loads(path.read_text())
                    document["pluginUsage"][key] = {"usageCount": 0, "lastUsedAt": clock + end_ms,
                                                    "lastUsedNumStartups": 0}
                    path.write_text(json.dumps(document))
                    current[0] = clock + end_ms
                sink = []
                with patch.object(subject, "checkpoint", side_effect=checked), \
                        patch.object(retirement.time, "time_ns", side_effect=lambda: current[0] * 1_000_000):
                    retirement.observe_user_initialize_diagnostic(subject, action, sink)
                self.assertIs(sink[0]["clock_window_valid"], valid)
                self.assertIs(sink[0]["after"]["fixed_fixture_keys"][key]["timestamp_in_window"], valid)
                self.assertTrue(sink[0]["pre_initialize_checkpoint_passed"])
                self.assertTrue(sink[0]["native_capture_returned"])
                self.assertFalse(sink[0]["initialization_qualified"])

    def test_user_row_comparison_distinguishes_equal_changed_and_missing(self):
        with self.fixture() as f:
            path, _, document = self.author_passive_usage(f, temporary=False)
            key = "marketing-kit@skills-dir"
            document["pluginUsage"][key] = {"usageCount": 0, "lastUsedAt": 1795750000000,
                                            "lastUsedNumStartups": 0}
            path.write_text(json.dumps(document))
            image = retirement._diagnostic_named_image(path, f["root"], f["audit"])
            for previous, expected in ((None, None), ({"pluginUsage": {}}, None), (document, True)):
                row = retirement._diagnostic_document(image, 0, 10 ** 13, previous)["fixed_fixture_keys"][key]
                self.assertIs(row["row_unchanged"], expected)
            changed = copy.deepcopy(document)
            changed["pluginUsage"][key]["usageCount"] = 1
            row = retirement._diagnostic_document(image, 0, 10 ** 13, changed)["fixed_fixture_keys"][key]
            self.assertIs(row["row_unchanged"], False)

    def test_main_retains_complete_large_temporary_diagnostic_for_capture_and_finish(self):
        from io import StringIO
        runner = runpy.run_path(str(ROOT / "scripts/marketing-kit-project-release-e2e.py"))
        main = runner["main"]
        for phase in ("capture", "finish"):
            with self.subTest(phase=phase), self.fixture() as f:
                _, pending, _ = self.author_passive_usage(f)
                subject = self.capture(f) if phase == "finish" else None
                for index in range(10):
                    extra = pending.with_name(f".claude.json.tmp.{200 + index}.710615a62269")
                    extra.write_bytes(pending.read_bytes()); extra.chmod(0o600)
                with self.assertRaisesRegex(ValueError, "passive usage temporary bound: ") as caught:
                    if subject is None:
                        self.capture(f)
                    else:
                        self.retire(f)
                        subject.finish(f["cleanup"], self.scan(f), lambda: self.scan(f))
                error = caught.exception
                detail = error.claude_temporary_bound_diagnostic
                self.assertGreater(len(str(error)), 2500)
                self.assertEqual(json.loads(str(error).split(": ", 1)[1]), detail)
                report_path = f["root"] / "diagnostic-report.json"
                with (
                    patch.dict(main.__globals__, {
                        "RemovalDiscovery": lambda *_: None,
                        "run_case": lambda *_: (_ for _ in ()).throw(error),
                    }),
                    patch.object(sys, "argv", ["runner", "--target", "claude",
                        "--cli-version", "0.1.116", "--report", str(report_path)]),
                    patch.object(sys, "stdout", StringIO()),
                    patch.object(sys, "stderr", StringIO()),
                ):
                    self.assertEqual(main(), 1)
                report = json.loads(report_path.read_text())
                self.assertEqual(report["status"], "failed")
                self.assertEqual(report["failure"], str(error)[:2500])
                self.assertEqual(report["claude_temporary_bound_diagnostic"], detail)
                self.assertEqual(detail["phase"], phase)
                self.assertEqual((detail["temporary_count"], len(detail["members"]), detail["omitted_count"]),
                                 (11, 8, 3))
                for private in (str(f["root"]), pending.name, "PRIVATE"):
                    self.assertNotIn(private, json.dumps(report))

    def author_passive_usage(self, f, temporary=True):
        usage = {owner["key"]: {"usageCount": 0, "lastUsedAt": 1795729883121,
                                 "lastUsedNumStartups": 0} for owner in f["owners"]}
        document = {"firstStartVersion": "2.1.263", "foreign": {"preserve": True}, "pluginUsage": usage}
        path = f["profile"] / ".claude.json"
        path.write_text(json.dumps(document)); path.chmod(0o600)
        pending = f["profile"] / ".claude.json.tmp.138.710615a62268"
        if temporary:
            partial = copy.deepcopy(document)
            del partial["pluginUsage"][f["owners"][0]["key"]]
            pending.write_text(json.dumps(partial)); pending.chmod(0o600)
        return path, pending, document

    def test_passive_native_usage_is_receipt_bound_and_remains_in_raw_scanner(self):
        for temporary in (False, True):
            with self.subTest(temporary=temporary), self.fixture() as f:
                path, pending, _ = self.author_passive_usage(f, temporary)
                before = {p: (p.read_bytes(), p.stat()) for p in (path, pending) if p.exists()}
                subject = self.capture(f)
                self.retire(f)
                raw = self.scan(f)
                self.assertIn(".claude/.claude.json", raw)
                proof = subject.finish(f["cleanup"], raw, lambda: self.scan(f), keep_open=True)
                self.assertEqual(proof["passive_plugin_usage"]["receipt_bound_key_count"], 4)
                self.assertEqual(len(proof["passive_plugin_usage"]["files"]), 1 + temporary)
                self.assertFalse(proof["full_native_removal_qualified"])
                subject.verify_retained(f["home"], f["root"], raw, lambda: self.scan(f))
                for p, (body, held) in before.items():
                    self.assertEqual(p.read_bytes(), body)
                    self.assertTrue(os.path.samestat(p.stat(), held))
                self.assertEqual(raw, self.scan(f))

    def test_passive_usage_rejects_unknown_records_config_and_temporary_files(self):
        mutations = ("missing_owner", "wrong_version", "positive_count", "boolean_count", "unknown_usage_field",
                     "other_market", "outside_usage", "temporary_foreign", "temporary_boolean",
                     "temporary_symlink", "temporary_hardlink", "main_hardlink", "second_temporary", "unsafe_mode")
        for mutation in mutations:
            with self.subTest(mutation=mutation), self.fixture() as f:
                path, pending, document = self.author_passive_usage(f)
                key = f["owners"][0]["key"]
                if mutation == "missing_owner": del document["pluginUsage"][key]
                elif mutation == "wrong_version": document["firstStartVersion"] = "2.1.264"
                elif mutation == "positive_count": document["pluginUsage"][key]["usageCount"] = 1
                elif mutation == "boolean_count": document["pluginUsage"][key]["usageCount"] = False
                elif mutation == "unknown_usage_field": document["pluginUsage"][key]["active"] = True
                elif mutation == "other_market": document["pluginUsage"]["marketing-kit@agentplugins-0123456789ab"] = {}
                elif mutation == "outside_usage": document["activePlugin"] = key
                elif mutation == "temporary_foreign":
                    value = json.loads(pending.read_text()); value["foreign"]["preserve"] = False
                    pending.write_text(json.dumps(value))
                elif mutation == "temporary_boolean":
                    value = json.loads(pending.read_text()); next(iter(value["pluginUsage"].values()))["usageCount"] = False
                    pending.write_text(json.dumps(value))
                elif mutation == "temporary_symlink": pending.unlink(); pending.symlink_to(path.name)
                elif mutation == "temporary_hardlink": os.link(pending, pending.with_name("foreign-hardlink"))
                elif mutation == "main_hardlink": os.link(path, path.with_name("foreign-hardlink"))
                elif mutation == "second_temporary":
                    another = pending.with_name(".claude.json.tmp.139.710615a62268")
                    another.write_bytes(pending.read_bytes()); another.chmod(0o600)
                elif mutation == "unsafe_mode": path.chmod(0o644)
                path.write_text(json.dumps(document))
                with self.assertRaises(ValueError): self.capture(f)

    def test_passive_usage_refuses_changed_or_replaced_bytes_and_late_temporary(self):
        for mutation in ("bytes", "replacement", "mode", "temporary_bytes", "late_temporary"):
            with self.subTest(mutation=mutation), self.fixture() as f:
                path, pending, document = self.author_passive_usage(f, mutation != "late_temporary")
                subject = self.capture(f)
                self.retire(f)
                if mutation == "bytes":
                    document["foreign"]["preserve"] = False; path.write_text(json.dumps(document))
                elif mutation == "replacement":
                    replacement = path.with_name("replacement"); replacement.write_bytes(path.read_bytes())
                    replacement.chmod(0o600); replacement.replace(path)
                elif mutation == "mode": path.chmod(0o644)
                else:
                    pending.write_text(json.dumps(document)); pending.chmod(0o600)
                raw = self.scan(f)
                with self.assertRaises(ValueError): subject.finish(f["cleanup"], raw, lambda: self.scan(f))
                self.assertEqual(subject.fds, [])

    def test_multiple_temporary_diagnostic_preserves_capture_and_finish_refusals(self):
        for phase in ("capture", "finish"):
            with self.subTest(phase=phase), self.fixture() as f:
                _, pending, _ = self.author_passive_usage(f)
                subject = self.capture(f) if phase == "finish" else None
                second = pending.with_name(".claude.json.tmp.139.710615a62269")
                second.write_bytes(pending.read_bytes()); second.chmod(0o600)
                if phase == "finish":
                    third = pending.with_name(".claude.json.tmp.140.710615a62270")
                    third.write_bytes(pending.read_bytes()); third.chmod(0o600)
                with self.assertRaisesRegex(ValueError, "passive usage temporary bound: ") as raised:
                    if subject is None:
                        self.capture(f)
                    else:
                        self.retire(f)
                        subject.finish(f["cleanup"], self.scan(f), lambda: self.scan(f))
                detail = json.loads(str(raised.exception).split(": ", 1)[1])
                self.assertEqual((detail["phase"], detail["temporary_count"], detail["limit"]), (phase, 3 if phase == "finish" else 2, 2 if phase == "finish" else 1))
                self.assertEqual(detail["prebound_temporary_count"], 1 if phase == "finish" else 0)
                self.assertEqual(detail["observed_prebound_temporary_count"], 1 if phase == "finish" else 0)
                self.assertEqual(detail["new_temporary_count"], 2)
                self.assertTrue(detail["diagnostic_only"])
                self.assertEqual(detail["ownership"], "ownership_unqualified")
                for row in detail["members"]:
                    self.assertEqual(row["status"], "observed")
                    for flag in ("regular_0600_owner_single_link", "named_held_identity_equal",
                                 "same_original_owner", "original_rows_unchanged", "foreign_json_equal",
                                 "nonempty_original_subset"):
                        self.assertTrue(row[flag], flag)
                    self.assertEqual(row["selected_original_row_count"], 3)
                    self.assertEqual(row["bytes"], pending.stat().st_size)
                if subject is not None:
                    self.assertEqual(subject.fds, [])

    def test_temporary_diagnostic_is_private_bounded_and_never_changes_authority(self):
        for mutation in ("foreign", "symlink", "changed-old", "truncation", "reader"):
            with self.subTest(mutation=mutation), self.fixture() as f:
                _, pending, _ = self.author_passive_usage(f)
                subject = self.capture(f)
                second = pending.with_name(".claude.json.tmp.139.710615a62269")
                second.write_bytes(pending.read_bytes()); second.chmod(0o600)
                if mutation == "foreign":
                    document = json.loads(second.read_text()); document["PRIVATE_NAME"] = "PRIVATE_VALUE"
                    second.write_text(json.dumps(document))
                elif mutation == "symlink":
                    second.unlink(); second.symlink_to(pending.name)
                elif mutation == "changed-old":
                    pending.write_text(pending.read_text() + " ")
                third = pending.with_name(".claude.json.tmp.140.710615a62270")
                third.write_bytes(second.read_bytes()); third.chmod(0o600)
                paths = [pending, second, third]
                if mutation == "truncation":
                    for index in range(9):
                        extra = pending.with_name(f".claude.json.tmp.{200 + index}.710615a62269")
                        extra.write_bytes(pending.read_bytes()); extra.chmod(0o600); paths.append(extra)
                passive = copy.deepcopy(subject.passive_usage)
                contract = copy.deepcopy(subject.usage_contract)
                descriptors = tuple(subject.fds)
                offsets = {fd: os.lseek(fd, 0, os.SEEK_CUR) for fd in descriptors}
                opened, open_attempts = [], []
                original_open = os.open
                def tracked_open(path, *args, **kwargs):
                    open_attempts.append((path, kwargs.get("dir_fd")))
                    fd = original_open(path, *args, **kwargs); opened.append(fd); return fd
                image_context = (patch.object(retirement, "_diagnostic_image", side_effect=ValueError("PRIVATE_VALUE"))
                                 if mutation == "reader" else patch.dict({}, {}))
                with image_context, patch.object(os, "open", side_effect=tracked_open):
                    with self.assertRaisesRegex(ValueError, "passive usage temporary bound: ") as raised:
                        subject.require_temporary_bound(paths, "finish")
                detail = json.loads(str(raised.exception).split(": ", 1)[1])
                self.assertEqual(detail["temporary_count"], len(paths))
                self.assertEqual(len(detail["members"]), min(8, len(paths)))
                self.assertEqual(detail["omitted_count"], max(0, len(paths) - 8))
                by_hash = {row["name_sha256"]: row for row in detail["members"]}
                if mutation == "foreign":
                    self.assertFalse(by_hash[retirement._diagnostic_digest(second.name)]["foreign_json_equal"])
                elif mutation == "symlink":
                    self.assertFalse(by_hash[retirement._diagnostic_digest(second.name)]["regular_0600_owner_single_link"])
                elif mutation == "changed-old":
                    self.assertFalse(by_hash[retirement._diagnostic_digest(pending.name)]["old_temporary_image_unchanged"])
                elif mutation == "reader":
                    self.assertTrue(all(row["status"] == "unavailable" for row in detail["members"]))
                self.assertEqual(tuple(subject.fds), descriptors)
                self.assertEqual(subject.passive_usage, passive)
                self.assertEqual(subject.usage_contract, contract)
                self.assertIsNone(subject.retired_allowed)
                self.assertFalse(subject.finished)
                self.assertEqual({fd: os.lseek(fd, 0, os.SEEK_CUR) for fd in descriptors}, offsets)
                self.assertTrue(opened)
                self.assertTrue(any(parent is not None for _, parent in open_attempts))
                for fd in opened:
                    with self.assertRaises(OSError): os.fstat(fd)
                for private in ("PRIVATE_NAME", "PRIVATE_VALUE", str(f["root"]), pending.name, second.name):
                    self.assertNotIn(private, str(raised.exception))

    def test_temporary_diagnostic_never_reads_an_ancestor_redirect(self):
        with self.fixture() as f, TemporaryDirectory() as outside:
            _, pending, document = self.author_passive_usage(f)
            subject = self.capture(f)
            descriptors = tuple(subject.fds)
            passive = copy.deepcopy(subject.passive_usage)
            offsets = {fd: os.lseek(fd, 0, os.SEEK_CUR) for fd in descriptors}
            second = pending.with_name(".claude.json.tmp.139.710615a62269")
            document["PRIVATE_NAME"] = "PRIVATE_VALUE"
            for name in (pending.name, second.name):
                candidate = Path(outside) / name
                candidate.write_text(json.dumps(document)); candidate.chmod(0o600)
            f["profile"].rename(f["home"] / "original-profile")
            f["profile"].symlink_to(outside, target_is_directory=True)
            opened, open_attempts = [], []
            original_open = os.open
            def tracked_open(path, *args, **kwargs):
                open_attempts.append((path, kwargs.get("dir_fd")))
                fd = original_open(path, *args, **kwargs); opened.append(fd); return fd
            with patch.object(os, "open", side_effect=tracked_open), \
                    patch.object(retirement, "_diagnostic_image", wraps=retirement._diagnostic_image) as image, \
                    patch.object(os, "fdopen", wraps=os.fdopen) as stream, \
                    patch.object(os, "pread", wraps=os.pread) as pread:
                with self.assertRaisesRegex(ValueError, "passive usage temporary bound: ") as raised:
                    subject.require_temporary_bound([pending, second], "capture")
            detail = json.loads(str(raised.exception).split(": ", 1)[1])
            self.assertEqual(detail["temporary_count"], 2)
            self.assertTrue(all(row["status"] == "unavailable" for row in detail["members"]))
            image.assert_not_called(); stream.assert_not_called(); pread.assert_not_called()
            self.assertTrue(opened)
            self.assertTrue(any(parent is not None for _, parent in open_attempts))
            for fd in opened:
                with self.assertRaises(OSError): os.fstat(fd)
            self.assertEqual(tuple(subject.fds), descriptors)
            self.assertEqual(subject.passive_usage, passive)
            self.assertEqual({fd: os.lseek(fd, 0, os.SEEK_CUR) for fd in descriptors}, offsets)
            for private in ("PRIVATE_NAME", "PRIVATE_VALUE", outside, str(f["root"])):
                self.assertNotIn(private, str(raised.exception))

    def test_finish_never_refreshes_old_temporary_custody_during_reopen(self):
        for window in ("before-reopen", "after-reopened-read"):
            with self.subTest(window=window), self.fixture() as f:
                _, old, _ = self.author_passive_usage(f)
                subject = self.capture(f)
                passive = copy.deepcopy(subject.passive_usage)
                descriptors = tuple(subject.fds)
                body = old.read_bytes()
                self.retire(f)
                original = subject.held_passive_document
                opened, raced = [], []
                def replace_old():
                    replacement = old.with_name("replacement")
                    replacement.write_bytes(body); replacement.chmod(0o600); replacement.replace(old)
                    raced.append(True)
                def interleave(path):
                    if path == old and window == "before-reopen": replace_old()
                    document, held = original(path)
                    opened.append(held["fd"])
                    if path == old and window == "after-reopened-read": replace_old()
                    return document, held
                with patch.object(subject, "held_passive_document", side_effect=interleave):
                    with self.assertRaises(ValueError):
                        subject.finish(f["cleanup"], self.scan(f), lambda: self.scan(f))
                self.assertEqual(raced, [True])
                self.assertEqual(old.read_bytes(), body)
                self.assertEqual(subject.passive_usage, passive)
                self.assertIsNone(subject.retired_allowed)
                self.assertEqual(subject.fds, [])
                for fd in (*descriptors, *opened):
                    with self.assertRaises(OSError): os.fstat(fd)

    def test_post_remove_temporary_custody_refusal_has_only_bounded_metadata(self):
        for mutation in ("mode", "empty", "identity"):
            with self.subTest(mutation=mutation), self.fixture() as f:
                _, pending, document = self.author_passive_usage(f, temporary=False)
                subject = self.capture(f)
                self.retire(f)
                del document["pluginUsage"][f["owners"][0]["key"]]
                pending.write_text(json.dumps(document)); pending.chmod(0o600)
                if mutation == "mode": pending.chmod(0o644)
                elif mutation == "empty": pending.write_bytes(b"")
                named_image = retirement._diagnostic_named_image
                descriptor_image = retirement._diagnostic_image
                metadata_observations = []
                def image(fd, audit):
                    # Count only the actual initial descriptor-metadata guard,
                    # rather than imposing the retired pathname reader's counts.
                    original_fstat, original_euid = os.fstat, os.geteuid
                    stats, owners = [], []
                    def fstat(opened):
                        stats.append(opened)
                        return original_fstat(opened)
                    def euid():
                        owners.append(True)
                        return original_euid()
                    with patch.object(os, "fstat", side_effect=fstat), patch.object(
                            os, "geteuid", side_effect=euid):
                        try:
                            return descriptor_image(fd, audit)
                        finally:
                            metadata_observations.append((stats, owners))
                            self.assertEqual(stats, [fd])
                            self.assertEqual(owners, [True])
                def named(path, case, audit, *, held_fd=None):
                    if path != pending:
                        return named_image(path, case, audit, held_fd=held_fd)
                    self.assertIsNotNone(held_fd)
                    if mutation == "identity":
                        replacement = pending.with_name("private-replacement")
                        replacement.write_bytes(pending.read_bytes()); replacement.chmod(0o600)
                        replacement.replace(pending)
                    original_fstat = os.fstat
                    held_observations = []
                    def fstat(opened):
                        if opened == held_fd:
                            held_observations.append(opened)
                        return original_fstat(opened)
                    # Neither an admission refusal nor its annotation may read
                    # file content. Failed I/O is never retried by diagnostics.
                    with patch.object(os, "pread", side_effect=AssertionError("unexpected content read")):
                        if mutation == "identity":
                            with patch.object(os, "fstat", side_effect=fstat):
                                try:
                                    return named_image(path, case, audit, held_fd=held_fd)
                                finally:
                                    self.assertEqual(held_observations, [held_fd])
                        with patch.object(retirement, "_diagnostic_image", side_effect=image):
                            return named_image(path, case, audit, held_fd=held_fd)
                raw = self.scan(f)
                with patch.object(retirement, "_diagnostic_named_image", side_effect=named), self.assertRaisesRegex(
                        ValueError, "Claude cache retirement authority is unresolved") as raised:
                    subject.finish(f["cleanup"], raw, lambda: self.scan(f))
                diagnostic = raised.exception.passive_file_custody_diagnostics
                reason = {"mode": "mode", "empty": "size", "identity": "identity"}[mutation]
                self.assertTrue(diagnostic["reasons"][reason])
                self.assertEqual(sum(value is True for value in diagnostic["reasons"].values()), 1)
                self.assertEqual(diagnostic["phase"],
                                 "held-descriptor" if mutation == "identity" else "descriptor-metadata")
                self.assertEqual(len(metadata_observations), int(mutation != "identity"))
                for facts in diagnostic["observations"].values():
                    self.assertEqual(set(facts), {"type", "mode", "nlink", "uid", "gid", "size"})
                serialized = str(raised.exception)
                self.assertLess(len(serialized), 1200)
                for private in (str(f["root"]), pending.name, "pluginUsage", "lastUsedAt", "foreign"):
                    self.assertNotIn(private, serialized)
                self.assertEqual(subject.fds, [])

    def test_passive_named_metadata_refusal_preserves_lazy_observations(self):
        for mutation in ("hardlink", "member_identity", "named_identity"):
            with self.subTest(mutation=mutation), self.fixture() as f:
                path, _, _ = self.author_passive_usage(f, temporary=False)
                subject = self.capture(f)
                held_fd = subject.passive_usage[0]["fd"]
                replacement = path.with_name("private-replacement")
                replacement.write_bytes(path.read_bytes()); replacement.chmod(0o600)
                if mutation == "hardlink":
                    os.link(path, path.with_name("private-hardlink"))
                original_member = f["audit"]["WINDSURF_GLOBAL_AUDIT"]["member"]
                @contextmanager
                def member(root_fd, relative):
                    if mutation == "member_identity":
                        replacement.replace(path)
                    with original_member(root_fd, relative) as fd:
                        if mutation == "named_identity":
                            original_fstat = os.fstat
                            def replace_after_observation(opened):
                                info = original_fstat(opened)
                                if opened == fd:
                                    replacement.replace(path)
                                return info
                            with patch.object(os, "fstat", side_effect=replace_after_observation):
                                yield fd
                        else:
                            yield fd
                original_lstat = Path.lstat
                named_stats = []
                def lstat(candidate, *args, **kwargs):
                    if candidate == path:
                        named_stats.append(candidate)
                    return original_lstat(candidate, *args, **kwargs)
                try:
                    with patch.dict(f["audit"]["WINDSURF_GLOBAL_AUDIT"], {"member": member}), patch.object(
                            Path, "lstat", lstat), patch.object(
                            os, "pread", side_effect=AssertionError("unexpected content read")):
                        with self.assertRaises(ValueError) as raised:
                            subject.passive_image(path, held_fd)
                    diagnostic = raised.exception.passive_file_custody_diagnostics
                    self.assertEqual(diagnostic["phase"], "named-member")
                    captured = diagnostic["observations"]
                    self.assertEqual(captured["member_fstat"] is None, mutation == "hardlink")
                    self.assertEqual(captured["named_lstat"] is None, mutation != "named_identity")
                    self.assertEqual(len(named_stats), 2 if mutation == "named_identity" else 1)
                    self.assertEqual(diagnostic["reasons"]["link_count"], mutation == "hardlink")
                    self.assertEqual(diagnostic["reasons"]["identity"], mutation != "hardlink")
                finally:
                    subject.close()

    def test_passive_metadata_io_failures_keep_original_exception(self):
        for phase in ("pathname", "descriptor", "held-descriptor"):
            with self.subTest(phase=phase), self.fixture() as f:
                path, _, _ = self.author_passive_usage(f, temporary=False)
                subject = self.capture(f)
                held_fd = subject.passive_usage[0]["fd"]
                failure = OSError("private fixture I/O failure")
                original_fstat = os.fstat
                def fstat(fd):
                    if fd == held_fd:
                        raise failure
                    return original_fstat(fd)
                try:
                    with patch.object(os, "pread", side_effect=AssertionError("unexpected content read")):
                        with self.assertRaises(OSError) as raised:
                            if phase == "pathname":
                                with patch.object(Path, "lstat", side_effect=failure):
                                    subject.passive_image(path, held_fd)
                            elif phase == "descriptor":
                                with patch.object(os, "fstat", side_effect=fstat):
                                    retirement._diagnostic_image(held_fd, f["audit"])
                            else:
                                with patch.object(os, "fstat", side_effect=fstat):
                                    subject.passive_image(path, held_fd)
                    self.assertIs(raised.exception, failure)
                    self.assertEqual(failure.args, ("private fixture I/O failure",))
                    self.assertFalse(hasattr(failure, "passive_file_custody_diagnostics"))
                finally:
                    subject.close()

    def test_only_valid_remove_admits_usage_deletion_and_one_native_temporary_once(self):
        for transition in ("in_place", "atomic", "new_temporary", "wrong_receipt"):
            with self.subTest(transition=transition), self.fixture() as f:
                path, pending, document = self.author_passive_usage(f, temporary=False)
                subject = self.capture(f)
                self.retire(f)
                del document["pluginUsage"][f["owners"][0]["key"]]
                if transition in ("in_place", "wrong_receipt"):
                    path.write_text(json.dumps(document))
                elif transition == "atomic":
                    pending.write_text(json.dumps(document)); pending.chmod(0o600); pending.replace(path)
                else:
                    pending.write_text(json.dumps(document)); pending.chmod(0o600)
                raw = self.scan(f)
                if transition == "wrong_receipt":
                    f["cleanup"]["results"][0]["installation_id"] = "unrelated-installation"
                    with self.assertRaises(ValueError): subject.finish(f["cleanup"], raw, lambda: self.scan(f))
                    self.assertEqual(subject.fds, [])
                    continue
                subject.finish(f["cleanup"], raw, lambda: self.scan(f), keep_open=True)
                subject.verify_retained(f["home"], f["root"], raw, lambda: self.scan(f))
                # A matching-looking usage temporary created after the one
                # authorized transition must never acquire retained authority.
                late = pending.with_name(".claude.json.tmp.139.710615a62269")
                late.write_text(json.dumps(document)); late.chmod(0o600)
                with self.assertRaises(ValueError):
                    subject.verify_retained(f["home"], f["root"], self.scan(f), lambda: self.scan(f))
                self.assertEqual(subject.fds, [])

    def test_finish_admits_two_strict_temporaries_with_original_custody(self):
        for old_count, selected_count in ((0, 3), (0, 1), (1, 2)):
            with self.subTest(old_count=old_count, selected_count=selected_count), self.fixture() as f:
                _, pending, document = self.author_passive_usage(f, temporary=bool(old_count))
                subject = self.capture(f)
                self.retire(f)
                for owner in f["owners"][:4 - selected_count]:
                    del document["pluginUsage"][owner["key"]]
                names = [pending, pending.with_name(".claude.json.tmp.139.710615a62269")]
                for path in names[old_count:]:
                    path.write_text(json.dumps(document)); path.chmod(0o600)
                before = {path: (path.read_bytes(), path.stat()) for path in names}
                original = tuple(subject.passive_usage)
                offsets = {row["fd"]: os.lseek(row["fd"], 0, os.SEEK_CUR) for row in original}
                raw = self.scan(f)
                proof = subject.finish(f["cleanup"], raw, lambda: self.scan(f), keep_open=True)
                self.assertEqual(len(proof["passive_plugin_usage"]["files"]), 3)
                self.assertEqual({fd: os.lseek(fd, 0, os.SEEK_CUR) for fd in offsets}, offsets)
                self.assertFalse(proof["full_native_removal_qualified"])
                subject.verify_retained(f["home"], f["root"], raw, lambda: self.scan(f))
                self.assertEqual(raw, self.scan(f))
                for path, (body, info) in before.items():
                    self.assertEqual(path.read_bytes(), body)
                    self.assertTrue(os.path.samestat(path.stat(), info))
                names[-1].write_text(json.dumps(dict(document, changed_after_finish=True)))
                with self.assertRaises(ValueError): subject.checkpoint()
                self.assertEqual(subject.fds, [])

    def test_two_new_temporaries_never_bypass_member_or_remove_authority(self):
        for mutation in ("third", "foreign", "changed-row", "boolean-row", "empty", "all-four",
                         "mode", "hardlink", "symlink", "owner", "receipt", "registry", "cache"):
            with self.subTest(mutation=mutation), self.fixture() as f:
                main, pending, document = self.author_passive_usage(f, temporary=False)
                subject = self.capture(f)
                original = copy.deepcopy(subject.passive_usage)
                descriptors = tuple(subject.fds)
                self.retire(f)
                del document["pluginUsage"][f["owners"][0]["key"]]
                second = pending.with_name(".claude.json.tmp.139.710615a62269")
                for path in (pending, second):
                    path.write_text(json.dumps(document)); path.chmod(0o600)
                changed = copy.deepcopy(document)
                key = next(iter(changed["pluginUsage"]))
                if mutation == "third":
                    third = pending.with_name(".claude.json.tmp.140.710615a62270")
                    third.write_bytes(pending.read_bytes()); third.chmod(0o600)
                elif mutation == "foreign": changed["foreign"]["preserve"] = False
                elif mutation == "changed-row": changed["pluginUsage"][key]["usageCount"] = 1
                elif mutation == "boolean-row": changed["pluginUsage"][key]["usageCount"] = False
                elif mutation == "empty": changed["pluginUsage"] = {}
                elif mutation == "all-four": changed = json.loads(main.read_text())
                elif mutation == "mode": second.chmod(0o644)
                elif mutation == "hardlink": os.link(second, second.with_name("foreign-hardlink"))
                elif mutation == "symlink": second.unlink(); second.symlink_to(pending.name)
                elif mutation == "receipt": f["cleanup"]["results"][0]["installation_id"] = "foreign"
                elif mutation == "registry":
                    registry = f["profile"] / "plugins/installed_plugins.json"
                    value = json.loads(registry.read_text()); value["foreign"] = True
                    registry.write_text(json.dumps(value))
                elif mutation == "cache":
                    retired = next(f["owners"][0]["cache"].parent.glob(".agentplugins-cache-retire-*/retired"))
                    (retired / "unowned").write_text("foreign")
                if mutation in ("foreign", "changed-row", "boolean-row", "empty", "all-four"):
                    second.write_text(json.dumps(changed))
                named_image = retirement._diagnostic_named_image
                def foreign_group(path, *args, **kwargs):
                    image = named_image(path, *args, **kwargs)
                    if mutation == "owner" and path == second:
                        image = dict(image, gid=image["gid"] + 1)
                    return image
                with patch.object(retirement, "_diagnostic_named_image", side_effect=foreign_group):
                    with self.assertRaises(ValueError):
                        subject.finish(f["cleanup"], self.scan(f), lambda: self.scan(f))
                self.assertEqual(subject.passive_usage, original)
                self.assertIsNone(subject.retired_allowed)
                self.assertEqual(subject.fds, [])
                for fd in descriptors:
                    with self.assertRaises(OSError): os.fstat(fd)

    def test_passive_authority_uses_original_fd_despite_path_reader_aba(self):
        for operation in ("image", "document"):
            with self.subTest(operation=operation), self.fixture() as f:
                path, _, document = self.author_passive_usage(f, temporary=False)
                subject = self.capture(f)
                fd = subject.passive_usage[0]["fd"]
                expected = path.read_bytes()
                foreign = expected.replace(b'"2.1.263"', b'"9.9.999"')
                shadow, parked = f["home"] / "shadow-profile", f["home"] / "parked-profile"
                shadow.mkdir(); (shadow / path.name).write_bytes(foreign)
                (shadow / path.name).chmod(0o600)
                reader = f["audit"]["copilot_settings_read"]
                def pathname_aba(candidate, case):
                    f["profile"].rename(parked); shadow.rename(f["profile"])
                    try: return reader(candidate, case)
                    finally:
                        f["profile"].rename(shadow); parked.rename(f["profile"])
                # Actual reader returns foreign bytes while the original FD and
                # restored pathname retain their original complete identity.
                identity = retirement._CUSTODY["_FILES"]["identity"]
                before = identity(os.fstat(fd))
                body, error = pathname_aba(path, f["root"])
                self.assertIsNone(error); self.assertEqual(body.encode(), foreign)
                self.assertEqual(identity(os.fstat(fd)), before)
                self.assertEqual(identity(path.lstat()), before)
                offset = os.lseek(fd, 7, os.SEEK_SET)
                with patch.dict(f["audit"], {"copilot_settings_read": pathname_aba}):
                    if operation == "image": image = subject.passive_image(path, fd)
                    else:
                        observed, held = subject.held_passive_document(path)
                        self.assertEqual(observed, document)
                        image = held["image"]
                self.assertEqual(image[-1], hashlib.sha256(expected).hexdigest())
                self.assertNotEqual(image[-1], hashlib.sha256(foreign).hexdigest())
                self.assertEqual(os.lseek(fd, 0, os.SEEK_CUR), offset)

    def test_passive_authority_refuses_member_walk_aba_without_foreign_reads(self):
        for operation in ("image", "document"):
            with self.subTest(operation=operation), self.fixture() as f:
                path, _, _ = self.author_passive_usage(f, temporary=False)
                subject = self.capture(f)
                held = subject.passive_usage[0]
                expected = path.read_bytes()
                foreign = expected.replace(b'"2.1.263"', b'"9.9.999"')
                shadow, parked = f["home"] / "shadow-profile", f["home"] / "parked-profile"
                shadow.mkdir(); (shadow / path.name).write_bytes(foreign)
                (shadow / path.name).chmod(0o600)
                original_open, original_pread = os.open, os.pread
                opened, reads, redirected = [], [], []
                home_identity = (f["home"].stat().st_dev, f["home"].stat().st_ino)
                shadow_identity = (shadow.stat().st_dev, shadow.stat().st_ino)
                original_descriptors = tuple(subject.fds)
                offsets = {fd: os.lseek(fd, 0, os.SEEK_CUR) for fd in original_descriptors}
                def open_aba(name, *args, **kwargs):
                    parent = kwargs.get("dir_fd")
                    if (name == f["profile"].name and args[0] & os.O_DIRECTORY
                            and parent is not None
                            and (os.fstat(parent).st_dev, os.fstat(parent).st_ino) == home_identity):
                        f["profile"].rename(parked); shadow.rename(f["profile"])
                        try:
                            fd = original_open(name, *args, **kwargs)
                            info = os.fstat(fd)
                            redirected.append((info.st_dev, info.st_ino))
                        finally:
                            f["profile"].rename(shadow); parked.rename(f["profile"])
                    else: fd = original_open(name, *args, **kwargs)
                    opened.append(fd)
                    return fd
                def pread(fd, length, offset):
                    body = original_pread(fd, length, offset); reads.append(body)
                    return body
                with patch.object(os, "open", side_effect=open_aba), patch.object(os, "pread", side_effect=pread):
                    with self.assertRaises(ValueError):
                        if operation == "image": subject.passive_image(path, held["fd"])
                        else: subject.held_passive_document(path)
                self.assertEqual(redirected, [shadow_identity])
                self.assertEqual(reads, [])
                self.assertTrue(opened)
                retained = set(subject.fds)
                self.assertEqual(len(retained - set(original_descriptors)), 1 if operation == "document" else 0)
                for fd in set(opened) - retained:
                    with self.assertRaises(OSError): os.fstat(fd)
                for fd in retained:
                    os.fstat(fd)
                self.assertEqual({fd: os.lseek(fd, 0, os.SEEK_CUR) for fd in original_descriptors}, offsets)
                subject.close()
                for fd in set(opened) | retained:
                    with self.assertRaises(OSError): os.fstat(fd)

    def test_passive_usage_rejects_same_inode_rewrites_without_owned_deletion(self):
        for mutation in ("same_bytes", "whitespace_only"):
            with self.subTest(mutation=mutation), self.fixture() as f:
                path, _, document = self.author_passive_usage(f, temporary=False)
                subject = self.capture(f)
                self.retire(f)
                before = path.stat()
                if mutation == "same_bytes": path.write_bytes(path.read_bytes())
                else: path.write_text(json.dumps(document, indent=2))
                self.assertTrue(os.path.samestat(before, path.stat()))
                raw = self.scan(f)
                with self.assertRaises(ValueError): subject.finish(f["cleanup"], raw, lambda: self.scan(f))
                self.assertEqual(subject.fds, [])

    @unittest.skipUnless(shutil.which("claude"), "pinned native Claude unavailable")
    def test_real_native_four_plugin_initialize_retains_only_bound_passive_usage(self):
        with self.fixture() as f:
            executable = str(Path(shutil.which("claude")).resolve())
            data = f["root"] / "data"; data.mkdir()
            env = f["audit"]["isolated_client_env"](f["home"], f["root"], data)
            env.update(DISABLE_AUTOUPDATER="1", CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC="1")
            probe = runpy.run_path(str(ROOT / "scripts/native_client_research.py"))["run_probe"]
            def native(*arguments, input_bytes=None):
                record, stdout, stderr = probe([executable, *arguments], f["project"], env,
                    f["root"], "pinned-native-passive-usage", timeout=60, memory_only=True, input_bytes=input_bytes)
                self.assertEqual((record["exit"], record["error"], stderr), (0, None, b""))
                return stdout
            self.assertEqual(native("--version").strip(), b"2.1.263 (Claude Code)")
            (f["profile"] / "plugins/installed_plugins.json").write_text('{"version":2,"plugins":{}}')
            (f["profile"] / "plugins/known_marketplaces.json").write_text('{}')
            for owner, row in zip(f["owners"], f["public"]):
                bundle = owner["active"] / ".agentplugins-plugin"
                (bundle / ".claude-plugin").mkdir()
                (bundle / ".claude-plugin/plugin.json").write_text(json.dumps({
                    "name": row["name"], "version": f["pins"][row["directory"]["recorded_distribution"]]["package_version"]}))
                marketplace = owner["active"] / ".claude-plugin"; marketplace.mkdir()
                (marketplace / "marketplace.json").write_text(json.dumps({"name": owner["market"],
                    "owner": {"name": "Agent Plugins CLI"},
                    "plugins": [{"name": row["name"], "source": "./.agentplugins-plugin"}]}))
                shutil.rmtree(owner["cache"])
                native("plugin", "marketplace", "add", str(owner["active"]), "--scope", "project")
                native("plugin", "install", owner["key"], "--scope", "project")
            native("-p", "--input-format", "stream-json", "--output-format", "stream-json", "--verbose",
                   "--permission-mode", "dontAsk", "--strict-mcp-config", "--mcp-config", '{"mcpServers":{}}',
                   input_bytes=b'{"type":"control_request","request_id":"native-discovery","request":{"subtype":"initialize"}}\n')
            subject = self.capture(f)
            for owner in f["owners"]:
                native("plugin", "uninstall", owner["key"], "--scope", "project", "--keep-data")
                native("plugin", "marketplace", "remove", owner["market"], "--scope", "project")
            for index, owner in enumerate(f["owners"]):
                quarantine = owner["cache"].parent / (".agentplugins-cache-retire-" + f"{index + 1:032x}")
                quarantine.mkdir(mode=0o700)
                owner["cache"].rename(quarantine / "retired")
                shutil.rmtree(owner["active"])
            self.assertEqual(json.loads(native("plugin", "list", "--json")), [])
            raw = self.scan(f)
            self.assertIn(".claude/.claude.json", raw)
            runner = runpy.run_path(str(ROOT / "scripts/marketing-kit-project-release-e2e.py"))
            proof = runner["check_removed_native_components"]("claude", f["home"], f["root"],
                "genuine native usage", claude_retirement=subject, cleanup=f["cleanup"])
            self.assertEqual(proof["raw_native_residuals"], sorted(raw))
            self.assertEqual(proof["passive_plugin_usage"]["receipt_bound_key_count"], 4)
            self.assertFalse(proof["full_native_removal_qualified"])

    def test_unowned_residual_refusal_is_bounded_sanitized_and_closes_custody(self):
        with self.fixture() as f:
            subject = self.capture(f)
            self.retire(f)
            raw = self.scan(f)
            for name in ["private-secret-\n" + str(index) for index in range(24)] + ["mcp-needs-auth-cache.json"]:
                (f["profile"] / name).write_text('{"name":"marketing-kit"}')
            residuals = self.scan(f)
            self.assertEqual(len(residuals - raw), 25)
            with self.assertRaisesRegex(ValueError, "unowned native residuals: ") as raised:
                subject.finish(f["cleanup"], residuals, lambda: self.scan(f))
            message = str(raised.exception)
            detail = json.loads(message.split(": ", 1)[1])
            self.assertEqual(detail["unexpected_residual_count"], 25)
            self.assertEqual(len(detail["items"]), 20)
            self.assertEqual(detail["omitted_key_count"], 5)
            self.assertEqual(detail["items"][0]["key"], ".claude/mcp-needs-auth-cache.json")
            self.assertNotIn("private-secret", message)
            self.assertEqual(subject.fds, [])
            self.assertEqual(residuals, self.scan(f))

    def test_unowned_residual_diagnostics_preserve_complete_observations_without_prefixes(self):
        with self.fixture() as f:
            subject = self.capture(f)
            self.retire(f)
            unsafe = ("visible: private-name", "visible: private-name\nline", "visible: unreadable extra",
                      "visible: unreadable", "visible: symlink", "visible: directory", "visible: unscanned-too-large")
            for name in unsafe:
                (f["profile"] / name).write_text('{"name":"marketing-kit"}')
            unsafe_link = "visible: private-link"
            (f["profile"] / unsafe_link).symlink_to("missing-target")
            (f["profile"] / "large.bin").write_bytes(b"x" * ((1 << 20) + 1))
            (f["profile"] / "marketing-kit").mkdir()
            (f["profile"] / "safe.json").write_text('{"name":"marketing-kit"}')
            residuals = self.scan(f)
            with self.assertRaisesRegex(ValueError, "unowned native residuals: ") as raised:
                subject.finish(f["cleanup"], residuals, lambda: self.scan(f))
            detail = json.loads(str(raised.exception).split(": ", 1)[1])
            items = {row["key_sha256"]: row for row in detail["items"]}
            hash_only = [".claude/" + name for name in unsafe] + [
                ".claude/" + unsafe_link + ": symlink", ".claude/large.bin: unscanned-too-large",
                ".claude/marketing-kit: directory"]
            for observation in hash_only:
                digest = hashlib.sha256(observation.encode()).hexdigest()
                self.assertIn(digest, items)
                self.assertNotIn("key", items[digest])
            readable = {row["key"] for row in detail["items"] if "key" in row}
            self.assertEqual(readable, {".claude/safe.json"})
            self.assertEqual(detail["unexpected_residual_count"], len(hash_only) + 1)
            self.assertEqual(subject.fds, [])
            self.assertEqual(residuals, self.scan(f))

    def test_exact_four_cache_custody_optional_marker_and_raw_findings_preserved(self):
        for marker in (False, True):
            with self.subTest(marker=marker), self.fixture() as f:
                subject = self.capture(f)
                self.retire(f, marker)
                residuals = self.scan(f)
                self.assertTrue(any(value.endswith(": unscanned-too-large") for value in residuals))
                proof = subject.finish(f["cleanup"], residuals, lambda: self.scan(f))
                self.assertEqual(proof["status"], "verified-signed-closure-cache-retirement-at-checkpoint")
                self.assertFalse(proof["writer_closure_claimed"])
                self.assertTrue(proof["retired_bytes_present"])
                for flag in ("physical_absence_qualified", "writer_closure_qualified", "full_native_removal_qualified"):
                    self.assertIs(proof[flag], False)
                self.assertEqual(proof["raw_native_residuals"], sorted(residuals))
                self.assertEqual(len(proof["members"]), 4)
                self.assertEqual(residuals, self.scan(f))
                self.assertEqual(subject.fds, [])
                self.assertTrue(all(owner["moved"].is_dir() for owner in f["owners"]))

    def test_installed_active_or_prepared_preserves_exact_readiness_without_auth_promotion(self):
        for activation in ("active", "prepared"):
            with self.subTest(activation=activation), self.fixture() as f:
                state = json.loads(f["state"].read_text())
                expected = []
                for record, public in zip(state["installations"], f["public"]):
                    binding = next(iter(record["clients"].values()))
                    binding["activation"] = public["clients"][0]["activation"] = activation
                    authentication = ("not_required" if activation == "active"
                                      and record["declared_name"] in ("marketing-kit", "playwright") else "not_checked")
                    binding["authentication"] = public["clients"][0]["authentication"] = authentication
                    expected.append({key: binding[key] for key in ("client_id", "scope", "activation",
                        "materialization", "verification", "authentication", "policy")})
                f["state"].write_text(json.dumps(state))
                subject = self.capture(f)
                self.retire(f)
                proof = subject.finish(f["cleanup"], self.scan(f), lambda: self.scan(f))
                self.assertEqual([member["binding_at_capture"] for member in proof["members"]], expected)
                self.assertEqual([row["authentication"] for row in expected],
                                 ["not_required" if activation == "active"
                                  and alias in ("marketing-kit", "playwright") else "not_checked"
                                  for alias in f["audit"]["MARKETING_KIT_DISTRIBUTIONS"]])
                self.assertFalse(proof["writer_closure_claimed"])

    def test_exact_project_authentication_and_raw_public_agreement_required(self):
        for alias in ("marketing-kit", "notion", "hubspot-crm", "playwright"):
            expected = "not_required" if alias in ("marketing-kit", "playwright") else "not_checked"
            for wrong in ("auth_pending", "not_checked" if expected == "not_required" else "not_required",
                          "failed", "authenticated", "pending", "unknown", "", None, "missing"):
                for location in ("both", "raw", "public"):
                    with self.subTest(alias=alias, wrong=wrong, location=location), self.fixture() as f:
                        state = json.loads(f["state"].read_text())
                        record = next(row for row in state["installations"] if row["declared_name"] == alias)
                        public = next(row for row in f["public"] if row["name"] == alias)
                        binding = next(iter(record["clients"].values()))
                        for label, row in (("raw", binding), ("public", public["clients"][0])):
                            if location not in ("both", label): continue
                            if wrong == "missing": row.pop("authentication")
                            else: row["authentication"] = wrong
                        f["state"].write_text(json.dumps(state))
                        with self.assertRaisesRegex(ValueError, "authentication"):
                            self.capture(f)

    def test_active_local_qualification_requires_absent_catalog_and_prepared_never_promotes(self):
        for alias in ("marketing-kit", "playwright"):
            for catalog in ("absent", "null"):
                for activation in ("active", "prepared"):
                    expected = "not_required" if activation == "active" else "not_checked"
                    for authentication in ("not_checked", "not_required"):
                        with (self.subTest(alias=alias, catalog=catalog, activation=activation,
                                           authentication=authentication), self.fixture() as f):
                            state = json.loads(f["state"].read_text())
                            index = next(i for i, row in enumerate(state["installations"])
                                         if row["declared_name"] == alias)
                            binding = next(iter(state["installations"][index]["clients"].values()))
                            for row in (binding, f["public"][index]["clients"][0]):
                                row.update(activation=activation, authentication=authentication)
                            if catalog == "null":
                                binding["package_revision"]["catalog_evidence"] = None
                            f["state"].write_text(json.dumps(state))
                            if authentication != expected:
                                with self.assertRaisesRegex(ValueError, "authentication.*expected=" + expected):
                                    self.capture(f)
                            else:
                                subject = self.capture(f)
                                self.assertEqual(subject.owners[index]["binding_at_capture"]["authentication"], expected)
                                subject.close()

    def test_pinned_project_checkpoint_refuses_explicit_unknown_and_malformed_catalog(self):
        explicit_unknown = {"schema_version": 1, "catalog_version": "directory-snapshot-1",
                            "compatibility": {"claude": {"authentication": "unknown",
                                                         "package": "projected", "verification": "schema_only"}}}
        for index in range(4):
            for catalog in (explicit_unknown, {}, [], "private-secret", False):
                for authentication in ("not_checked", "not_required"):
                    with (self.subTest(index=index, catalog=catalog, authentication=authentication),
                          self.fixture() as f):
                        state = json.loads(f["state"].read_text())
                        binding = next(iter(state["installations"][index]["clients"].values()))
                        binding["package_revision"]["catalog_evidence"] = catalog
                        binding["authentication"] = f["public"][index]["clients"][0]["authentication"] = authentication
                        f["state"].write_text(json.dumps(state))
                        with self.assertRaisesRegex(ValueError, "authentication catalog evidence.*expected=absent_or_null") as failure:
                            self.capture(f)
                        self.assertNotIn("secret", str(failure.exception))

    def test_remote_authentication_never_inherits_active_local_qualification(self):
        for alias in ("notion", "hubspot-crm"):
            for activation in ("active", "prepared"):
                with self.subTest(alias=alias, activation=activation), self.fixture() as f:
                    state = json.loads(f["state"].read_text())
                    index = next(i for i, row in enumerate(state["installations"]) if row["declared_name"] == alias)
                    binding = next(iter(state["installations"][index]["clients"].values()))
                    for row in (binding, f["public"][index]["clients"][0]):
                        row.update(activation=activation, authentication="not_required")
                    f["state"].write_text(json.dumps(state))
                    with self.assertRaisesRegex(ValueError, alias + ".*authentication.*expected=not_checked"):
                        self.capture(f)

    def test_authentication_refusal_identifies_recorded_and_public_states_without_arbitrary_data(self):
        for raw, public, expected in (
                ("not_checked", "not_checked", "recorded=not_checked, public=not_checked"),
                ("not_checked", "auth_pending", "recorded=not_checked, public=auth_pending"),
                ("foreign-secret-value", {"untrusted": "secret"}, "recorded=invalid, public=invalid")):
            with self.subTest(raw=raw), self.fixture() as f:
                state = json.loads(f["state"].read_text())
                record = next(row for row in state["installations"] if row["declared_name"] == "marketing-kit")
                view = next(row for row in f["public"] if row["name"] == "marketing-kit")
                next(iter(record["clients"].values()))["authentication"] = raw
                view["clients"][0]["authentication"] = public
                f["state"].write_text(json.dumps(state))
                with self.assertRaises(ValueError) as failure:
                    self.capture(f)
                self.assertIn(expected, str(failure.exception))
                self.assertIn("expected=not_required", str(failure.exception))
                self.assertNotIn("secret", str(failure.exception))

    def test_first_authentication_refusal_retains_all_four_exact_checkpoint_observations(self):
        with self.fixture() as f:
            state = json.loads(f["state"].read_text())
            digest = "sha256:" + "a" * 64
            for record, public in zip(state["installations"], f["public"]):
                binding = next(iter(record["clients"].values()))
                binding["authentication"] = public["clients"][0]["authentication"] = "not_required"
                record["directory"]["snapshot_digest"] = digest
                binding["package_revision"]["catalog_evidence"] = {
                    "schema_version": 1, "catalog_version": "directory-snapshot-1", "digest": digest,
                    "compatibility": {"claude": {"authentication": "unknown", "package": "projected", "verification": "schema_only"}}}
            f["state"].write_text(json.dumps(state))
            checkpoints = []
            with self.assertRaisesRegex(ValueError, "marketing-kit.*authentication"):
                self.capture(f, checkpoint_diagnostics=checkpoints)
            self.assertEqual(len(checkpoints), 1)
            observation = checkpoints[0]
            self.assertEqual(observation["status"], "observed")
            self.assertEqual(set(observation["members"]), set(f["audit"]["MARKETING_KIT_DISTRIBUTIONS"]))
            for member in observation["members"].values():
                self.assertTrue(member["same_installation_id"])
                self.assertEqual(member["recorded"], member["public"])
                self.assertEqual(member["recorded"]["authentication"], "not_required")
                self.assertEqual(member["recorded_catalog_evidence"]["authentication"], "unknown")
                self.assertTrue(member["recorded_catalog_evidence"]["digest_matches_recorded_snapshot"])
                self.assertEqual(member["public_catalog_evidence"], "not_exposed_by_public_schema")
            self.assertTrue(all(owner["cache"].is_dir() for owner in f["owners"]))

    def test_checkpoint_projection_redacts_malicious_fields_and_does_not_guess_ambiguous_binding(self):
        with self.fixture() as f:
            state = json.loads(f["state"].read_text())
            record, public = state["installations"][0], f["public"][0]
            raw = next(iter(record["clients"].values()))
            for binding in (raw, public["clients"][0]):
                binding.update(authentication="private-secret", activation={"secret": "value"},
                               verification=["secret"], policy="private-secret")
                binding["package_revision"]["catalog_evidence"] = {
                    "schema_version": True, "catalog_version": "private-secret", "digest": "private-secret",
                    "repository": "private-secret", "compatibility": {"claude": {
                        "authentication": "private-secret", "package": {"secret": True}, "verification": "private-secret"}}}
            observation = project_checkpoint_summary(state, f["public"], f["audit"])
            member = observation["members"]["marketing-kit"]
            self.assertEqual(set(member["recorded"].values()), {"invalid"})
            self.assertEqual(member["recorded_catalog_evidence"]["schema_version"], "invalid")
            self.assertEqual(member["recorded_catalog_evidence"]["digest_shape"], "invalid")
            self.assertNotIn("secret", json.dumps(observation))
            self.assertNotIn(str(f["root"]), json.dumps(observation))
            record["clients"]["another"] = copy.deepcopy(raw)
            observation = project_checkpoint_summary(state, f["public"], f["audit"])
            self.assertEqual(set(observation["members"]["marketing-kit"]["recorded"].values()), {"missing"})
            self.assertEqual(observation["members"]["marketing-kit"]["recorded_binding_selection"], "multiple")

    def test_checkpoint_safe_read_failure_is_unavailable_and_does_not_replace_strict_error(self):
        with self.fixture() as f:
            audit = dict(f["audit"], copilot_settings_read=lambda *_: (None, "private-secret"))
            self.assertEqual(observe_project_checkpoint(f["root"], f["project"], f["public"], audit), {"status": "unavailable"})
            checkpoints = []
            original = ValueError("original strict refusal")
            with patch.object(SignedClosureRetirement, "document", side_effect=original):
                with self.assertRaises(ValueError) as refusal:
                    self.capture(f, checkpoint_diagnostics=checkpoints)
            self.assertIs(refusal.exception, original)
            self.assertEqual(checkpoints, [{"status": "unavailable"}])

    def test_checkpoint_normalizes_only_actual_domain_enums_and_catalog_presence(self):
        with self.fixture() as f:
            state = json.loads(f["state"].read_text())
            binding = next(iter(state["installations"][0]["clients"].values()))
            public = f["public"][0]["clients"][0]
            enums = {"authentication": ("not_checked", "not_required", "auth_pending", "authenticated", "failed"),
                     "activation": ("not_required", "prepared", "manual_activation_required", "active", "failed"),
                     "verification": ("not_run", "package_validated", "installation_verified", "runtime_verified", "failed"),
                     "policy": ("allowed", "blocked", "approval_required")}
            for key, values in enums.items():
                for value in values:
                    binding[key] = public[key] = value
                    member = project_checkpoint_summary(state, f["public"], f["audit"])["members"]["marketing-kit"]
                    self.assertEqual(member["recorded"][key], value)
                    self.assertEqual(member["public"][key], value)
            for key, value in (("activation", "degraded"), ("verification", "not_checked"), ("policy", "not_checked")):
                binding[key] = value
                self.assertEqual(project_checkpoint_summary(state, f["public"], f["audit"])["members"]["marketing-kit"]["recorded"][key], "invalid")
            for record, value in zip(state["installations"][1:], (None, [], {"schema_version": 1,
                    "catalog_version": "directory-snapshot-1", "compatibility": {}})):
                next(iter(record["clients"].values()))["package_revision"]["catalog_evidence"] = value
            members = project_checkpoint_summary(state, f["public"], f["audit"])["members"]
            self.assertEqual([member["recorded_catalog_evidence"]["presence"] for member in members.values()],
                             ["absent", "null", "invalid", "object"])
            self.assertEqual(members["playwright"]["recorded_catalog_evidence"]["claude_compatibility"], "absent")

    def test_checkpoint_sink_keeps_successful_retirement_and_fd_custody_unchanged(self):
        with self.fixture() as f:
            checkpoints = []
            subject = self.capture(f, checkpoint_diagnostics=checkpoints)
            self.assertEqual(checkpoints, [observe_project_checkpoint(f["root"], f["project"], f["public"], f["audit"])])
            held = tuple(subject.fds)
            self.assertTrue(held)
            self.retire(f)
            proof = subject.finish(f["cleanup"], self.scan(f), lambda: self.scan(f))
            self.assertEqual(len(proof["members"]), 4)
            self.assertEqual(subject.fds, [])
            for fd in held:
                with self.assertRaises(OSError): os.fstat(fd)

    def test_other_readiness_fields_still_fail_closed(self):
        for key, values in {"activation": ("failed", "manual_activation_required", "not_required", "degraded", "unknown", ""),
                            "materialization": ("absent",), "verification": ("failed", "package_validated", "runtime_verified"),
                            "policy": ("blocked",), "scope": ("user",), "client_id": ("codex",)}.items():
            for value in values:
                with self.subTest(key=key, value=value), self.fixture() as f:
                    state = json.loads(f["state"].read_text())
                    binding = next(iter(state["installations"][0]["clients"].values()))
                    binding[key] = f["public"][0]["clients"][0][key] = value
                    f["state"].write_text(json.dumps(state))
                    with self.assertRaisesRegex(ValueError, key):
                        self.capture(f)

    def test_signed_state_public_receipt_namespace_and_bundle_mutations_refused(self):
        for mutation in ("source", "digest", "public", "scope", "receipt", "namespace", "bundle", "hardlink", "symlink"):
            with self.subTest(mutation=mutation), self.fixture() as f:
                state = json.loads(f["state"].read_text())
                record = state["installations"][0]
                binding = next(iter(record["clients"].values()))
                if mutation == "source": record["source"]["repository"] = "foreign/repo"
                elif mutation == "digest": record["source"]["tree_digest"] = "sha256:" + "0" * 64
                elif mutation == "public": f["public"][0]["directory"]["recorded_release_sequence"] = 99
                elif mutation == "scope": binding["scope"] = "user"
                elif mutation == "receipt": binding["receipts"][0]["phase"] = "pending"
                elif mutation == "namespace": binding["native_objects"][0]["source_relative"] = "plugins/cache/foreign/marketing-kit/0.1.0"
                else:
                    payload = f["owners"][0]["cache"] / ".agentplugins-runtime/plugin.json"
                    if mutation == "bundle": payload.write_text("foreign")
                    elif mutation == "hardlink": os.link(payload, payload.with_name("hardlink"))
                    elif mutation == "symlink": payload.with_name("link").symlink_to(payload)
                f["state"].write_text(json.dumps(state))
                with self.assertRaises((ValueError, OSError)):
                    self.capture(f)

    def test_removal_receipts_must_name_every_prior_signed_owner(self):
        for mutation in ("missing", "duplicate", "wrong_id", "wrong_binding", "wrong_client", "wrong_scope", "unchanged", "pending"):
            with self.subTest(mutation=mutation), self.fixture() as f:
                subject = self.capture(f)
                self.retire(f)
                cleanup = f["cleanup"]
                if mutation == "missing": cleanup["actions"].pop()
                elif mutation == "duplicate": cleanup["actions"][0] = copy.deepcopy(cleanup["actions"][1])
                elif mutation == "wrong_id": cleanup["results"][0]["installation_id"] = "foreign"
                elif mutation == "wrong_binding": cleanup["actions"][0]["binding_id"] = "foreign"
                elif mutation == "wrong_client": cleanup["results"][0]["client_id"] = "codex"
                elif mutation == "wrong_scope": cleanup["scope"] = "user"
                elif mutation == "unchanged": cleanup["actions"][0]["action"] = "unchanged"
                elif mutation == "pending": cleanup["results"][0]["outcome"] = "pending"
                with self.assertRaises(ValueError):
                    subject.finish(cleanup, self.scan(f), lambda: self.scan(f))
                self.assertEqual(subject.fds, [])

    def test_foreign_native_registry_rows_and_unowned_residuals_never_qualify(self):
        for mutation in ("installed_foreign", "market_foreign", "active_registration", "extra_residual", "extra_quarantine", "marker", "retired_bytes", "retired_replaced", "parent_replaced", "project_replaced"):
            with self.subTest(mutation=mutation), self.fixture() as f:
                subject = self.capture(f)
                self.retire(f)
                owner = f["owners"][0]
                if mutation in ("installed_foreign", "active_registration"):
                    path = f["profile"] / "plugins/installed_plugins.json"
                    doc = json.loads(path.read_text())
                    if mutation == "installed_foreign": doc["plugins"]["foreign@vendor"][0]["foreign"] = False
                    else: doc["plugins"][owner["key"]] = [{"scope": "project"}]
                    path.write_text(json.dumps(doc))
                elif mutation == "market_foreign":
                    path = f["profile"] / "plugins/known_marketplaces.json"
                    doc = json.loads(path.read_text()); doc["vendor"]["foreign"] = True
                    path.write_text(json.dumps(doc))
                elif mutation == "extra_residual": (f["profile"] / "marketing-kit.json").write_text("{}")
                elif mutation == "extra_quarantine":
                    (owner["cache"].parent / (".agentplugins-cache-retire-" + "f" * 32)).mkdir(mode=0o700)
                elif mutation == "marker": (owner["moved"] / ".orphaned_at").write_text("not-a-timestamp")
                elif mutation == "retired_bytes": (owner["moved"] / ".agentplugins-runtime/plugin.json").write_text("foreign")
                elif mutation == "retired_replaced":
                    old = owner["moved"].with_name("original"); owner["moved"].rename(old)
                    shutil.copytree(old, owner["moved"])
                elif mutation == "parent_replaced":
                    parent = owner["cache"].parent; old = parent.with_name("original"); parent.rename(old)
                    shutil.copytree(old, parent)
                elif mutation == "project_replaced":
                    f["project"].rename(f["project"].with_name("original-project"))
                    f["project"].mkdir()
                with self.assertRaises((ValueError, OSError)):
                    subject.finish(f["cleanup"], self.scan(f), lambda: self.scan(f))
                self.assertEqual(subject.fds, [])

    def test_rescan_cannot_mutate_retained_tree_or_registry_after_first_check(self):
        for mutation in ("tree", "registry", "active_cache", "quarantine_extra", "parent_empty_sibling"):
            with self.subTest(mutation=mutation), self.fixture() as f:
                subject = self.capture(f)
                self.retire(f)
                residuals = self.scan(f)
                calls = []
                def rescan():
                    calls.append(True)
                    observed = self.scan(f)
                    owner = f["owners"][0]
                    if mutation == "tree": (owner["moved"] / "extra.txt").write_text("foreign")
                    elif mutation == "registry":
                        path = f["profile"] / "plugins/known_marketplaces.json"
                        doc = json.loads(path.read_text()); doc["vendor"]["foreign"] = 2
                        path.write_text(json.dumps(doc))
                    elif mutation == "active_cache": owner["cache"].mkdir()
                    elif mutation == "parent_empty_sibling": (owner["cache"].parent / "unrelated-empty").mkdir(exist_ok=True)
                    else: (owner["moved"].parent / "extra").mkdir()
                    return observed
                with self.assertRaises(ValueError):
                    subject.finish(f["cleanup"], residuals, rescan)
                self.assertTrue(calls)
                self.assertEqual(subject.fds, [])

    def test_close_is_safe_when_command_refuses_before_finish(self):
        with self.fixture() as f:
            subject = self.capture(f)
            subject.close(); subject.close()
            self.assertEqual(subject.fds, [])
            with self.assertRaises(ValueError):
                subject.finish(f["cleanup"], {"foreign"}, lambda: {"foreign"})

    def test_project_identity_cannot_reset_between_member_captures(self):
        original = SignedClosureRetirement.capture
        with self.fixture() as f:
            seen = []
            def capture(subject, *args):
                owner = original(subject, *args)
                seen.append(subject)
                if len(seen) == 1:
                    old = f["project"].with_name("original-project")
                    f["project"].rename(old)
                    shutil.copytree(old, f["project"])
                return owner
            with patch.object(SignedClosureRetirement, "capture", capture):
                with self.assertRaises(ValueError):
                    self.capture(f)
            self.assertTrue(seen)
            self.assertEqual(seen[0].fds, [])

    def test_marketing_entry_keeps_strict_default_and_profile_scope_binding(self):
        runner = runpy.run_path(str(ROOT / "scripts/marketing-kit-project-release-e2e.py"))
        check = runner["check_removed_native_components"]
        with self.fixture() as f:
            subject = self.capture(f)
            self.retire(f)
            with self.assertRaisesRegex(AssertionError, "native Marketing Kit remnants"):
                check("claude", f["home"], f["root"], "strict default")
            with self.assertRaisesRegex(AssertionError, "another profile"):
                check("claude", f["home"], f["root"], "wrong scope",
                      project_scope=True, claude_retirement=subject, cleanup=f["cleanup"])
            raw = runner["native_component_residuals"]("claude", f["home"], f["root"], "raw")
            proof = check("claude", f["home"], f["root"], "guarded checkpoint",
                          claude_retirement=subject, cleanup=f["cleanup"])
            self.assertEqual(proof["matched_residual_count"], len(raw))
            self.assertEqual(raw, runner["native_component_residuals"](
                "claude", f["home"], f["root"], "still raw"))
            self.assertEqual(subject.fds, [])

    def test_project_retirement_survives_user_cycle_only_with_continuously_held_authority(self):
        runner = runpy.run_path(str(ROOT / "scripts/marketing-kit-project-release-e2e.py"))
        for retained in (False, True):
            with self.subTest(retained=retained), self.fixture() as f:
                subject = self.capture(f)
                try:
                    original_fds = tuple(subject.fds)
                    self.retire(f)
                    subject.finish(f["cleanup"], self.scan(f), lambda: self.scan(f), keep_open=retained)
                    # Actual filesystem representation of the independent User
                    # @skills-dir lifecycle; no native process is simulated.
                    user = f["profile"] / "skills/marketing-kit"
                    user.mkdir(parents=True)
                    (user / "SKILL.md").write_text("User-owned marketing-kit fixture")
                    if retained:
                        subject.checkpoint()  # User files coexist; no global absence claim.
                        self.assertTrue(set(original_fds) <= set(subject.fds))
                    shutil.rmtree(user)
                    with self.assertRaisesRegex(AssertionError, "native Marketing Kit remnants"):
                        runner["check_removed_native_components"](
                            "claude", f["home"], f["root"], "old final User A default")
                    if retained:
                        proof = runner["check_removed_native_components"](
                            "claude", f["home"], f["root"], "final User A",
                            retained_claude_retirement=subject)
                        self.assertEqual(proof["scope"], "project")
                        self.assertFalse(proof["writer_closure_claimed"])
                        self.assertEqual(proof["matched_residual_count"], len(self.scan(f)))
                    else:
                        self.assertEqual(subject.fds, [])
                        with self.assertRaises(ValueError):
                            subject.verify_retained(f["home"], f["root"], self.scan(f), lambda: self.scan(f))
                finally:
                    subject.close()

    def test_retained_checkpoint_rejects_late_user_residue_and_retired_mutations(self):
        changes = ("user-residue", "tree", "metadata", "marker", "quarantine", "parent",
                   "registry", "active-resurrection", "cache-resurrection", "empty", "rescan")
        for change in changes:
            with self.subTest(change=change), self.fixture() as f:
                subject = self.capture(f)
                self.retire(f)
                subject.finish(f["cleanup"], self.scan(f), lambda: self.scan(f), keep_open=True)
                held = tuple(subject.fds)
                owner = f["owners"][0]
                if change == "user-residue":
                    path = f["profile"] / "skills/marketing-kit/SKILL.md"
                    path.parent.mkdir(parents=True); path.write_text("must not be allowed by Project custody")
                elif change in ("tree", "metadata"):
                    path = owner["moved"] / ".agentplugins-runtime/plugin.json"
                    if change == "tree": path.write_text("changed retired bytes")
                    else: path.chmod(0o600)
                elif change == "marker":
                    (owner["moved"] / ".orphaned_at").write_text("1794880808342")
                elif change == "quarantine":
                    (owner["moved"].parent / "extra.txt").write_text("unexpected sibling")
                elif change == "parent":
                    parent = owner["cache"].parent
                    saved = parent.with_name(parent.name + "-saved")
                    parent.rename(saved); shutil.copytree(saved, parent)
                elif change == "registry":
                    path = f["profile"] / "plugins/installed_plugins.json"
                    data = json.loads(path.read_text()); data["plugins"][owner["key"]] = []
                    path.write_text(json.dumps(data))
                elif change == "active-resurrection":
                    owner["active"].mkdir(parents=True)
                elif change == "cache-resurrection":
                    owner["cache"].mkdir()
                raw = self.scan(f)
                if change == "empty": raw = set()
                rescan = (lambda: raw | {".claude/unowned"}) if change == "rescan" else lambda: self.scan(f)
                with self.assertRaises(ValueError):
                    subject.verify_retained(f["home"], f["root"], raw, rescan)
                self.assertEqual(subject.fds, [])
                for fd in held:
                    with self.assertRaises(OSError): os.fstat(fd)

    def test_retained_parent_entries_preserve_exact_accepted_siblings(self):
        for marker in (False, True):
            for change in (None, "empty-sibling", "nested-empty-sibling", "replaced-sibling", "removed-sibling"):
                with self.subTest(marker=marker, change=change), self.fixture() as f:
                    parent = f["owners"][0]["cache"].parent
                    sibling = parent / "unrelated-version"
                    sibling.mkdir()
                    subject = self.capture(f)
                    try:
                        self.retire(f, marker=marker)
                        subject.finish(f["cleanup"], self.scan(f), lambda: self.scan(f), keep_open=True)
                        held, original = tuple(subject.fds), self.scan(f)
                        if change == "empty-sibling": (parent / "unrelated-empty").mkdir()
                        elif change == "nested-empty-sibling": (parent / "new-version/empty").mkdir(parents=True)
                        elif change in ("replaced-sibling", "removed-sibling"):
                            sibling.rename(f["root"] / "original-unrelated-version")
                            if change == "replaced-sibling": sibling.mkdir()
                        # These namespace mutations leave the identity scanner
                        # unchanged; rejection must come from held-parent custody.
                        self.assertEqual(self.scan(f), original)
                        if change is None:
                            proof = subject.verify_retained(f["home"], f["root"], original, lambda: self.scan(f))
                            self.assertEqual(proof["status"],
                                "verified-unchanged-signed-project-cache-retirement-at-user-removal")
                            for flag in ("physical_absence_qualified", "writer_closure_qualified", "full_native_removal_qualified"):
                                self.assertIs(proof[flag], False)
                            self.assertTrue(set(held) <= set(subject.fds))
                        else:
                            with self.assertRaises(ValueError):
                                subject.verify_retained(f["home"], f["root"], original, lambda: self.scan(f))
                            self.assertEqual(subject.fds, [])
                            for fd in held:
                                with self.assertRaises(OSError): os.fstat(fd)
                    finally:
                        subject.close()

    def test_retained_authority_cannot_be_unfinished_closed_reused_or_wrong_profile(self):
        for change in ("unfinished", "closed", "wrong-home", "wrong-case", "twice", "invalid-receipt"):
            with self.subTest(change=change), self.fixture() as f:
                subject = self.capture(f)
                held = tuple(subject.fds)
                self.assertIsNone(subject.retired_allowed)
                if change == "unfinished":
                    with self.assertRaises(ValueError): subject.checkpoint()
                else:
                    self.retire(f)
                    cleanup = copy.deepcopy(f["cleanup"])
                    if change == "invalid-receipt": cleanup["results"][0]["outcome"] = "pending"
                    if change == "invalid-receipt":
                        with self.assertRaises(ValueError):
                            subject.finish(cleanup, self.scan(f), lambda: self.scan(f), keep_open=True)
                        self.assertIsNone(subject.retired_allowed)
                    else:
                        subject.finish(cleanup, self.scan(f), lambda: self.scan(f), keep_open=True)
                        held = tuple(subject.fds)
                        if change == "closed": subject.close()
                        with self.assertRaises(ValueError):
                            if change == "twice":
                                subject.finish(cleanup, self.scan(f), lambda: self.scan(f), keep_open=True)
                            else:
                                subject.verify_retained(f["home"] / "wrong" if change == "wrong-home" else f["home"],
                                    f["root"] / "wrong" if change == "wrong-case" else f["root"],
                                    self.scan(f), lambda: self.scan(f))
                self.assertEqual(subject.fds, [])
                for fd in held:
                    with self.assertRaises(OSError): os.fstat(fd)

    def test_execute_captures_verified_four_member_receipt_for_real_retirement(self):
        runner = runpy.run_path(str(ROOT / "scripts/marketing-kit-project-release-e2e.py"))
        execute = runner["execute"]
        with self.fixture() as f:
            subject = self.capture(f)
            captured = []
            def invoke(argv, env, project, timeout):
                self.assertEqual(argv, ["/released/agentplugins", "remove", "marketing-kit",
                    "--scope", "project", "--project-root", str(f["project"]), "--format", "json"])
                self.assertEqual(project, f["project"])
                self.retire(f)
                return {"exit_code": 0, "timeout": False, "_json": copy.deepcopy(f["cleanup"])}
            try:
                with patch.dict(execute.__globals__, {"invoke": invoke}):
                    step = execute("/released/agentplugins", {}, f["project"], "claude", "remove B",
                        "remove", "marketing-kit", "--scope", "project", "--project-root", str(f["project"]),
                        capture=captured.append)
                self.assertEqual(step, {"stage": "remove B", "exit_code": 0, "result": "passed"})
                self.assertEqual(captured, [f["cleanup"]])
                proof = runner["check_removed_native_components"](
                    "claude", f["home"], f["root"], "remove B native HOME",
                    claude_retirement=subject, cleanup=captured[0])
                self.assertEqual(len(proof["members"]), 4)
                self.assertNotIn("actions", proof)
                self.assertEqual(subject.fds, [])
            finally:
                subject.close()

    def test_execute_refusal_never_captures_receipt_and_caller_closes_custody(self):
        runner = runpy.run_path(str(ROOT / "scripts/marketing-kit-project-release-e2e.py"))
        execute = runner["execute"]
        for failure in ("process", "pending", "partial_receipts"):
            with self.subTest(failure=failure), self.fixture() as f:
                subject = self.capture(f)
                captured = []
                result = {"exit_code": 0, "timeout": False, "_json": copy.deepcopy(f["cleanup"])}
                if failure == "process": result["exit_code"] = 1
                elif failure == "pending": result["_json"]["outcome"] = "pending"
                else: result["_json"]["results"].pop()
                try:
                    with patch.dict(execute.__globals__, {"invoke": lambda *args: result}):
                        with self.assertRaisesRegex(AssertionError, "CLI did not succeed"):
                            execute("/released/agentplugins", {}, f["project"], "claude", "remove B",
                                "remove", "marketing-kit", "--scope", "project", "--project-root", str(f["project"]),
                                capture=captured.append)
                    self.assertEqual(captured, [])
                finally:
                    subject.close()
                self.assertEqual(subject.fds, [])
                self.assertTrue(all(owner["cache"].is_dir() for owner in f["owners"]))

    def test_fresh_prepared_authentication_is_preserved_without_promotion(self):
        with self.fixture() as f:
            state = json.loads(f["state"].read_text())
            for record, public in zip(state["installations"], f["public"]):
                binding = next(iter(record["clients"].values()))
                binding["activation"] = public["clients"][0]["activation"] = "prepared"
                binding["authentication"] = public["clients"][0]["authentication"] = "not_checked"
            f["state"].write_text(json.dumps(state))
            subject = self.capture(f)
            self.retire(f)
            proof = subject.finish(f["cleanup"], self.scan(f), lambda: self.scan(f))
            self.assertEqual([row["binding_at_capture"]["authentication"] for row in proof["members"]],
                             ["not_checked"] * 4)
            self.assertEqual([row["binding_at_capture"]["activation"] for row in proof["members"]],
                             ["prepared"] * 4)
            self.assertFalse(proof["writer_closure_claimed"])

    def test_main_attaches_bounded_A_and_B_diagnostics_only_to_failure_report(self):
        from test_live_install_audit_env import IsolatedClientEnvironmentTest
        fixture = IsolatedClientEnvironmentTest()
        code, report, _ = fixture._exercise_marketing_kit_scope_report("claude", project_remove_error="remove B")
        self.assertEqual(code, 1)
        self.assertIn("injected Project removal failure", report["failure"])
        self.assertEqual(set(report["claude_project_checkpoint_diagnostics"]),
                         {"project_a_after_repair", "project_b_before_remove"})
        code, report, _ = fixture._exercise_marketing_kit_scope_report("claude")
        self.assertEqual(code, 0, report.get("failure"))
        self.assertNotIn("claude_project_checkpoint_diagnostics", report)

if __name__ == "__main__":
    unittest.main()
