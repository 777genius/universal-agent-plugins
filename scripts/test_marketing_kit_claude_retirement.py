"""Real filesystem controls for signed four-package Claude cache custody."""
import copy
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


if __name__ == "__main__":
    unittest.main()
