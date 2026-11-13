"""Inert real-file/consumer contracts; never vendor or model execution."""
import base64
from contextlib import contextmanager
import copy
import hashlib
import json
import os
from pathlib import Path
import tempfile
import shutil
from types import SimpleNamespace
import unittest
from unittest import mock

import devin_global_lifecycle_admission as case


class GlobalLifecycleContracts(unittest.TestCase):
    def context(self, root):
        return SimpleNamespace(root=root, original_output_identity=case.base.root_identity(root),
                               budget=SimpleNamespace(check=lambda: 10), argv=[str(root / "no-native")], summary={})


    def build_receipt(self):
        value = {"schema_version": 1, "source_commit": "a" * 40, "source_tree": "b" * 40,
                 "environment": "original-hosted-build; GOTOOLCHAIN=local", "commands": [], "streams": {}}
        for role, path in (("go", "/hosted/go"), ("installer", "/owned/agentplugins")):
            value[role] = dict(path=path, bytes=8, sha256="c" * 64, uid=os.getuid(), gid=os.getgid(), mode=0o755)
        for label, args in (("build-go-version", ["version"]),
                            ("build-uap", ["build", "-o", "/owned/agentplugins", "./cli/cmd/agentplugins"])):
            row = dict(probe=label, argv=["/hosted/go", *args], cwd=str(case.base.shared.SOURCE),
                       exit=0, error=None, elapsed_seconds=0.01)
            for kind in ("stdout", "stderr"):
                body = b"go version go1.26.11 linux/amd64\n" if (label, kind) == ("build-go-version", "stdout") else b""
                name = label + "." + kind + ".log"
                row.update({kind + "_file": name, kind + "_bytes": len(body), kind + "_sha256": case.sha(body)})
                value["streams"][name] = base64.b64encode(body).decode()
            value["commands"].append(row)
        return value

    def completed_summary(self):
        first = ("source-commit", "source-tree", "source-clean", "git-init-a", "git-init-b", "git-init-c") + case.base.METADATA_PROBES
        tail = ("uap-add", *case.NATIVE[:4], "uap-doctor-installed", "uap-repair", *case.NATIVE[4:8],
                "uap-doctor-repaired", "uap-remove", *case.NATIVE[8:], "source-final-clean")
        return {"commands": [{"probe": name, "exit": 0, "error": None} for name in first + tail],
                "command_effects": [dict(label=name, changes={}, outer_changes={}) for name in case.base.METADATA_PROBES + tail[:-1]],
                "global_lifecycle_capture_complete": True, "metadata_capture_complete": True, "ordinary_metadata_capture_complete": True,
                "metadata_recipe": "ordinary-global", "metadata_route": case.ordinary.OrdinaryEffects.metadata_route,
                "native_json_schema": case.base.METADATA_SCHEMA, "exact_source_admission": case.base.METADATA_SCHEMA,
                "admission_qualified": False, "qualification": dict.fromkeys(case.base.shared.QUALIFICATION_CASES, False),
                "native_global_steps": list(case.NATIVE), "public_global_steps": list(case.PUBLIC),
                "probe_methods": dict.fromkeys(("a", "b", "user", "local", "legacy"), []),
                "accepted_connections": dict.fromkeys(("a", "b", "user", "local", "legacy"), 0),
                "source_commit": "a" * 40, "source_tree": "b" * 40, "global_build_receipt": self.build_receipt()}

    @contextmanager
    def installed_state(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            ctx = case.base.shared.Context(root, {}, SimpleNamespace(check=lambda: 10))
            ctx.original_output_identity, ctx.argv = case.base.root_identity(root), [str(root / "no-vendor")]
            probes = {name: SimpleNamespace(url="http://127.0.0.1:1234/mcp", accepted_connections=[],
                      methods=lambda: [], call_records=lambda: ([], 0)) for name in ("a", "b", "user", "local", "legacy")}
            source = case.author_source(ctx, probes)
            expected, _, _ = case.base.fixture(ctx, {name: probe.url for name, probe in probes.items()}, "n" * 64)
            journey = object.__new__(case.Journey)
            journey.ctx, journey.home, journey.source = ctx, ctx.home / "uap-state", source
            journey.profile, journey.probes, journey.expected = Path(expected["user"]["root"]), probes, expected
            journey.source_bodies, journey.source_sha = ctx.global_source_bodies, case.source_digest(ctx.global_source_bodies)
            journey.identifier = journey.binding = journey.installation_basis = journey.binding_basis = journey.data_authority = None
            journey.data_receipt, journey.package_members, journey.removed = None, set(), False
            journey.public_labels, journey.native_labels, journey.state_history, journey.log_births = [], [], [], {}
            identifier = "12345678-1234-4234-8234-123456789abc"
            physical, active, data = journey.identity_paths(identifier)
            binding_id = "client_" + case.sha("\0".join((identifier, "windsurf", "user", str(active))).encode())[:24]
            data_id = "data_" + case.sha("\0".join((identifier, physical, "user")).encode())[:24]
            marker = dict(data_receipt_id=data_id, physical_backend_id=physical, scope="user", locator=str(data),
                          ownership_digest="sha256:" + case.sha(("agentplugins-plugin-data-v1\0" + data_id + "\0" + str(data)).encode()), state="owned")
            def write(path, body, mode=0o644):
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_bytes(body); path.chmod(mode)
            projected = {case.SERVERS[0]: dict(type="stdio", command=str(active / case.NS / "agentplugins"),
                            args=["--internal-stdio-v1", str(active), str(data), str(active), "plugin", "--", "python3", str(active / "inert.py")],
                            env={"PLUGIN_ROOT": str(active), "PLUGIN_DATA": str(data)}),
                         case.SERVERS[1]: dict(type="streamable-http", url=probes["user"].url)}
            for name, body in journey.source_bodies.items():
                write(active / name, json.dumps({"$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json",
                                                "mcpServers": projected}).encode() if name == "mcp.json" else body)
            helper = b"inert-cli"
            journey.cli_pin = (len(helper), case.sha(helper))
            write(active / case.NS / "agentplugins", helper, 0o700)
            write(active / case.NS / "metadata.json", json.dumps(dict(protocol=1, sha256=case.sha(helper), cliVersion="0.1.0-development")).encode(), 0o600)
            entries = copy.deepcopy(expected["user"]["mcp"])
            for name in case.SERVERS:
                entries[name] = dict(projected[name]); entries[name].pop("type")
            entries[case.SERVERS[1]]["transport"] = "http"
            write(journey.profile / "mcp_config.json", json.dumps({"mcpServers": entries}).encode())
            skill = journey.source_bodies["skills/" + case.SKILL + "/SKILL.md"]
            write(journey.profile / "skills" / case.SKILL / "SKILL.md", skill)
            write(data / ".agentplugins-data-owner.json", json.dumps(marker).encode(), 0o600)
            write(journey.home / "mutation.lock", b'{"pid":1,"acquired_at":"2026-11-13T00:00:00Z"}', 0o600)
            manifest = "sha256:" + case.sha(journey.source_bodies["plugin.json"])
            journey.security = dict(schema_version=1, subject=dict(tree_digest=journey.source_sha, manifest_digest=manifest),
                                    scanner=dict(id="lintai", version="0.1.3"), policy=case.POLICY,
                                    outcome="no_blocking_findings", counts=dict(blocking=0, warnings=0, total=0),
                                    scanned_files=4, report_digest="sha256:" + "e" * 64)
            journey.assessment_path = case.security_role(journey.security)
            write(journey.home / journey.assessment_path, json.dumps(journey.security).encode(), 0o600)
            inventory = case.base.metadata_inventory(root, ctx.budget)
            _, digest = case.capture_package(ctx, active, inventory, journey.cli_pin)
            objects = [dict(object_id="package:windsurf:" + physical, kind="managed_package_directory",
                            logical_name=case.NAME, path=str(active), managed_digest=digest, protection_class="managed"),
                       dict(object_id="devin_global_skill_directory:" + case.SKILL, kind="devin_global_skill_directory",
                            logical_name=case.SKILL, path=str(journey.profile / "skills" / case.SKILL), source_relative="skills/" + case.SKILL,
                            managed_digest="sha256:" + case.sha(b"f\0SKILL.md\0-\0" + skill + b"\0"), protection_class="managed")]
            objects += [dict(object_id="devin_global_mcp_server:" + name, kind="devin_global_mcp_server", logical_name=name,
                             path=str(journey.profile / "mcp_config.json"), managed_digest=case.entry_digest(name, entries[name]),
                             protection_class="managed") for name in case.SERVERS]
            binding = dict(client_binding_id=binding_id, client_id="windsurf", scope="user", physical_artifact_id=physical,
                           target_locator=str(active), native_profile_root=str(journey.profile), data_receipt_id=data_id,
                           materialization="materialized", activation="prepared", authentication="not_checked", policy="allowed",
                           verification="installation_verified", package_revision=dict(version="1.0.0", tree_digest=journey.source_sha, manifest_digest=manifest),
                           native_objects=objects, receipts=[dict(operation_id="operation-1", phase="committed", sequence=1,
                           mutation_type="directory_swap",
                           client_binding_id=binding_id, active_path=str(active), after_digest=digest)])
            item = dict(installation_id=identifier, declared_name=case.NAME, clients={binding_id: binding}, data_receipts={data_id: marker},
                        source=dict(source_binding_id="src_" + case.sha((str(source) + "\0\0").encode())[:24],
                                    requested_source=str(source), canonical_source=str(source), resolved_revision="", tree_digest=journey.source_sha),
                        package=dict(loader_kind="agent_plugins", format_id="agent-plugins/1.0.0",
                                     schema_uri="https://agent-plugins.org/schemas/1.0.0/plugin.schema.json", declared_name=case.NAME,
                                     version="1.0.0", manifest_digest=manifest,
                                     inventory=dict(mcp_present=True, mcp_enabled=True, mcp_servers=sorted(case.SERVERS), skills=[case.SKILL])))
            value = dict(schema_version=5, installations=[item])
            path = journey.home / "state-v2.json"
            write(path, json.dumps(value).encode(), 0o600)
            inventory = case.base.metadata_inventory(root, ctx.budget)
            journey.state(inventory, identifier=identifier)
            journey.verified_inventory = inventory
            journey.authorities = {part.relative_to(root).as_posix(): case.base.root_identity(part)
                                   for part in (journey.home, journey.profile, source)}
            journey.effects = SimpleNamespace(baseline=inventory, config_path="home/config/devin/config.json", verify=mock.Mock())
            yield journey, value, path, active, data

    def test_large_role_streaming_hash_does_not_widen_ordinary_limit(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve(); ctx = self.context(root)
            path = root / "large"; path.write_bytes(b"x" * ((1 << 20) + 1))
            path.chmod(0o700)
            inventory = case.base.metadata_inventory(root, ctx.budget)
            expected = hashlib.sha256(path.read_bytes()).hexdigest()
            with self.assertRaisesRegex(ValueError, "ordinary file authority"):
                case.regular(ctx, "large", inventory)
            self.assertEqual(case.regular(ctx, "large", inventory, large=(path.stat().st_size, expected)), expected)
            with self.assertRaisesRegex(ValueError, "large role differs"):
                case.regular(ctx, "large", inventory, large=(path.stat().st_size, "0" * 64))
            original = case.base.os.read
            sizes = []
            def read(fd, amount):
                sizes.append(amount); return original(fd, amount)
            with mock.patch.object(case.base.os, "read", side_effect=read):
                case.regular(ctx, "large", inventory, large=(path.stat().st_size, expected))
            self.assertLessEqual(max(sizes), 65536)
            with mock.patch.object(os, "read", side_effect=AssertionError("wrong-sized role read bytes")):
                with self.assertRaisesRegex(ValueError, "large role metadata"):
                    case.regular(ctx, "large", inventory, large=(path.stat().st_size + 1, expected))
            path.chmod(0o755)
            with mock.patch.object(os, "read", side_effect=AssertionError("wrong-mode role read bytes")):
                with self.assertRaisesRegex(ValueError, "large role metadata"):
                    case.regular(ctx, "large", case.base.metadata_inventory(root, ctx.budget), large=(path.stat().st_size, expected))

    def test_large_role_alias_owner_and_identity_refuse_before_read(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve(); ctx = self.context(root)
            path = root / "member"; path.write_bytes(b"owned")
            inventory = case.base.metadata_inventory(root, ctx.budget)
            for change in ("owner", "links", "inode"):
                value = copy.deepcopy(inventory)
                if change in ("owner", "links"):
                    row = list(value["member"]); row[4 if change == "owner" else 3] += 1; value["member"] = tuple(row)
                else:
                    other = root / "other"; other.write_bytes(b"foreign"); other.replace(path)
                with mock.patch.object(os, "read", side_effect=AssertionError("foreign bytes read")):
                    with self.assertRaises(ValueError): case.regular(ctx, "member", value, large=(5, case.sha(b"owned")))

    def test_public_consumer_uses_real_argv_and_aborts_failed_command_without_adoption(self):
        journey = object.__new__(case.Journey)
        journey.installer, journey.home = Path("/owned/agentplugins"), Path("/owned/home/uap-state")
        journey.cli_pin = (12, '0' * 64)
        journey.ctx = SimpleNamespace(env={"HOME": "/original", "XDG_CONFIG_HOME": "/original/config", "PATH": "/usr/bin"},
                                     argv=["/owned/client/bin/devin"], projects={"user": Path("/owned/outside")}, budget=object())
        journey.frame = mock.Mock(return_value=({}, {}))
        journey.state = mock.Mock(side_effect=AssertionError("failed command adopted state"))
        journey.ctx.command = mock.Mock(return_value=({"exit": 1}, b"{}", b"native refusal"))
        with mock.patch.object(case, 'installer_pin', return_value=journey.cli_pin):
            with self.assertRaisesRegex(ValueError, "public Global lifecycle refused"):
                journey.public("uap-add", ["add", "/owned/source", "--no-save"])
        args = journey.ctx.command.call_args.args
        self.assertEqual(args[1], ["/owned/agentplugins", "add", "/owned/source", "--no-save", "--yes",
                                   "--target", "windsurf", "--scope", "user", "--format", "json"])
        self.assertEqual(args[3]["HOME"], "/original")
        self.assertEqual(args[3]["XDG_CONFIG_HOME"], "/original/config")
        self.assertEqual(args[3]["PATH"], "/owned/client/bin:/usr/bin")
        journey.state.assert_not_called()

    def test_native_fence_rejects_user_foreign_and_prior_log_changes(self):
        journey = object.__new__(case.Journey)
        journey.effects = mock.Mock(); journey.effects.baseline = {}; journey.effects.config_path = "config"
        journey.log_births = {}
        for name in ("home/config/devin/mcp_config.json", "home/uap-state/state-v2.json",
                     "project-b/foreign", "home/data/devin/cli/logs/devin_20261113-180000_12.log"):
            with self.subTest(name=name), self.assertRaises(ValueError):
                journey.verify_native_delta({name: (1,)}, {name: (2,)}, {name: {"sha256": "before"}},
                                            {name: {"sha256": "after"}}, "mcp-installed-a")
        journey.effects.verify.assert_not_called()

    def test_exact_sequence_completes_only_research_with_false_qualifications(self):
        value = self.completed_summary()
        case.completion(value)
        for change in ("missing", "error", "qualification", "bool", "effects", "schema", "source", "endpoint", "ordinary"):
            wrong = copy.deepcopy(value)
            if change == "missing": wrong["commands"].pop()
            elif change == "error": wrong["commands"][0]["error"] = "timeout"
            elif change == "qualification": wrong["qualification"]["mcp-tools-call"] = True
            elif change == "bool": wrong["commands"][8]["exit"] = False
            elif change == "effects": wrong["command_effects"].pop(13)
            elif change == "schema": wrong["native_json_schema"] = "unreviewed"
            elif change == "source": wrong["source_tree"] = "d" * 40
            elif change == "endpoint": wrong["accepted_connections"]["a"] = False
            else: wrong["ordinary_metadata_capture_complete"] = False
            with self.subTest(change=change), self.assertRaises(ValueError): case.completion(wrong)

    def test_actual_owned_skill_quarantine_preserves_inode_bytes_and_refuses_existing_target(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve(); ctx = self.context(root)
            ctx.private, ctx.summary = root / "private", {}
            ctx.private.mkdir()
            profile = root / "home/config/devin"
            selected = profile / "skills" / case.SKILL
            selected.mkdir(parents=True)
            body = b"---\nname: owned\n---\nprivate source\n"
            (selected / "SKILL.md").write_bytes(body)
            original = case.base.root_identity(selected)
            case.quarantine_skill(ctx, profile, body)
            moved = ctx.private / "deliberately-quarantined-skill"
            self.assertEqual(case.base.root_identity(moved), original)
            self.assertEqual((moved / "SKILL.md").read_bytes(), body)
            selected.mkdir(); (selected / "SKILL.md").write_bytes(body)
            with self.assertRaisesRegex(ValueError, "quarantine preexists"):
                case.quarantine_skill(ctx, profile, body)
            self.assertEqual((moved / "SKILL.md").read_bytes(), body)

    def test_closed_uap_home_refuses_unknown_member_before_reading_it(self):
        with self.installed_state() as (journey, _value, _path, _active, _data):
            journey.closed_home(journey.verified_inventory)
            unknown = journey.home / "unknown-private-token"
            unknown.write_bytes(b"foreign sentinel")
            with mock.patch.object(os, "read", side_effect=AssertionError("any body was read")) as read:
                with self.assertRaisesRegex(ValueError, "unknown UAP state member"):
                    journey.frame()
                read.assert_not_called()
            self.assertEqual(unknown.read_bytes(), b"foreign sentinel")

    def test_native_skill_extra_member_is_refused_before_any_postpublic_read(self):
        with self.installed_state() as (journey, _value, _path, _active, _data):
            unknown = journey.profile / "skills" / case.SKILL / "foreign-private-token"
            unknown.write_bytes(b"foreign sentinel")
            inventory = case.base.metadata_inventory(journey.ctx.root, journey.ctx.budget)
            with mock.patch.object(os, "read", side_effect=AssertionError("any postpublic body was read")) as read:
                with self.assertRaisesRegex(ValueError, "Global native Skill closed membership"):
                    journey.state(inventory)
                read.assert_not_called()
            self.assertEqual(unknown.read_bytes(), b"foreign sentinel")

    def test_native_skill_metadata_and_removal_refuse_before_reads(self):
        for change in ("directory", "mode", "size", "removed"):
            with self.subTest(change=change), self.installed_state() as (journey, _value, _path, active, _data):
                selected = journey.profile / "skills" / case.SKILL
                if change == "directory": (selected / "unknown").mkdir()
                elif change == "mode": (selected / "SKILL.md").chmod(0o600)
                elif change == "size": (selected / "SKILL.md").write_bytes(b"wrong size")
                elif change == "removed": shutil.rmtree(active)
                inventory = case.base.metadata_inventory(journey.ctx.root, journey.ctx.budget)
                with mock.patch.object(os, "read", side_effect=AssertionError("postpublic body read")) as read:
                    with self.assertRaisesRegex(ValueError, "Global native Skill"):
                        journey.state(inventory, removed=change == "removed")
                    read.assert_not_called()

    def test_declared_skill_quarantine_allows_frame_but_not_repaired_state(self):
        with self.installed_state() as (journey, _value, _path, _active, _data):
            journey.quarantine()
            journey.frame()
            with mock.patch.object(os, "read", side_effect=AssertionError("repaired body read")) as read:
                with self.assertRaisesRegex(ValueError, "Global native Skill closed membership"):
                    journey.state(case.base.metadata_inventory(journey.ctx.root, journey.ctx.budget))
                read.assert_not_called()

    def test_aggregate_budget_is_admitted_before_first_body(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve(); ctx = self.context(root)
            for index in range(17):
                with (root / str(index)).open("wb") as output: output.truncate(1 << 20)
            with mock.patch.object(os, "read", side_effect=AssertionError("over-budget body read")) as read:
                with self.assertRaisesRegex(ValueError, "Global ordinary aggregate"):
                    case.capture(ctx, (1, "0" * 64))
                read.assert_not_called()
            (root / "16").unlink()
            _, frame = case.capture(ctx, (1, "0" * 64))
            self.assertEqual(sum(len(row["identity"]) == 9 for row in frame.values()), 16)

    def test_scan_cache_role_is_exact_subject_policy_and_typed_counts(self):
        value = {"schema_version": 1, "subject": {"tree_digest": "sha256:" + "a" * 64,
                 "manifest_digest": "sha256:" + "b" * 64}, "scanner": {"id": "lintai", "version": "0.1.3"},
                 "policy": case.POLICY, "counts": {"blocking": 0, "warnings": 0, "total": 0},
                 "outcome": "no_blocking_findings", "scanned_files": 4, "report_digest": "sha256:" + "c" * 64}
        path = case.security_role(value)
        self.assertRegex(path, r"^security/assessments/[0-9a-f]{64}\.json$")
        changed = copy.deepcopy(value); changed["subject"]["tree_digest"] = "sha256:" + "d" * 64
        self.assertNotEqual(case.security_role(changed), path)
        for field in ("schema_version", "blocking", "scanner", "extra"):
            wrong = copy.deepcopy(value)
            if field == "schema_version": wrong[field] = True
            elif field == "blocking": wrong["counts"][field] = False
            elif field == "scanner": wrong[field]["version"] = "0.1.2"
            else: wrong["extra"] = "unbound"
            with self.assertRaises(ValueError): case.security_role(wrong)

    def test_actual_native_consumer_checks_selected_user_source_before_next_command(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            ctx = case.base.shared.Context(root, {}, SimpleNamespace(check=lambda: 10))
            ctx.original_output_identity = case.base.root_identity(root)
            ctx.argv = [str(root / "no-vendor")]
            probes = {name: SimpleNamespace(url='http://127.0.0.1:1234/mcp', accepted_connections=[],
                       methods=lambda: [], call_records=lambda: ([], 0)) for name in ('a', 'b', 'user', 'local', 'legacy')}
            expected, _, _ = case.base.fixture(ctx, {name: probe.url for name, probe in probes.items()}, "n" * 64)
            profile = Path(expected['user']['root'])
            entries = dict(expected['user']['mcp'])
            entries.update({case.SERVERS[0]: {'command': 'python3', 'args': []},
                            case.SERVERS[1]: {'url': probes['user'].url}})
            (profile / 'mcp_config.json').write_text(json.dumps({'mcpServers': entries}))
            wanted = {name: (entry, profile / 'mcp_config.json') for name, entry in entries.items()}
            wanted.update({name: (entry, Path(expected['a']['root']) / 'mcp_config.json') for name, entry in expected['a']['mcp'].items()})
            wanted['same-name'] = ({'url': expected['local']['url'], 'disabled': True}, Path(expected['local']['path']))
            wanted['legacy-foreign'] = ({'url': expected['legacy_url'], 'disabled': True}, ctx.home / '.codeium/windsurf/mcp_config.json')
            rows = [dict(name=name, enabled=not entry.get('disabled', False), disabled_reason='disabled' if entry.get('disabled') else None,
                         transport='stdio' if 'command' in entry else 'http', command=entry.get('command'),
                         args=entry.get('args', []), url=entry.get('url'), disabled_tools=entry.get('disabledTools', []), source=str(source))
                    for name, (entry, source) in wanted.items()]
            for wrong in (False, True):
                journey = object.__new__(case.Journey)
                journey.ctx, journey.probes, journey.expected, journey.profile = ctx, probes, expected, profile
                journey.cli_pin, journey.helper, journey.nonce = (1, '0' * 64), None, "n" * 64
                journey.authorities = {profile.relative_to(root).as_posix(): case.base.root_identity(profile)}
                journey.log_births, journey.native_labels = {}, []
                journey.frame = lambda inventory=None: case.capture(ctx, journey.cli_pin, authorities=journey.authorities, inventory=inventory)
                journey.effects = SimpleNamespace(baseline=case.base.metadata_inventory(root, ctx.budget),
                                                   config_path='home/config/devin/config.json', verify=mock.Mock())
                emitted = copy.deepcopy(rows)
                if wrong:
                    next(row for row in emitted if row['name'] == case.SERVERS[0])['source'] = str(ctx.projects['b'] / '.devin/mcp_config.json')
                ctx.probe = mock.Mock(side_effect=[({'exit': 0}, json.dumps(emitted).encode(), b''), StopIteration('second command reached')])
                with self.assertRaises(ValueError if wrong else StopIteration):
                    journey.native('installed')
                self.assertEqual(ctx.probe.call_count, 1 if wrong else 2)
                self.assertEqual(journey.native_labels, [] if wrong else ['mcp-installed-a'])
                self.assertNotIn('global_lifecycle_capture_complete', ctx.summary)

    def test_global_recipe_reaches_host_guard_without_fixture_io(self):
        with mock.patch.object(case.base.shared.native, 'require_hosted', side_effect=RuntimeError('host gate')) as gate:
            with mock.patch.object(Path, 'mkdir', side_effect=AssertionError('fixture created')):
                with self.assertRaisesRegex(RuntimeError, 'host gate'):
                    case.base.run(Path('/unused'), collector=lambda *args: None, recipe='ordinary-global')
        gate.assert_called_once()

    def test_execute_forwards_removed_state_to_public_consumer(self):
        journey = object.__new__(case.Journey)
        journey.source, journey.identifier = Path('/source'), 'installed-id'
        journey.ctx = SimpleNamespace(summary={})
        journey.log_births, journey.state_history = {}, []
        journey.public_labels, journey.native_labels = [], []
        journey.source_bodies = {'skills/' + case.SKILL + '/SKILL.md': b'inert'}
        journey.profile = Path('/profile')
        calls = []
        def public(label, argv, **kwargs):
            calls.append((label, kwargs)); journey.public_labels.append(label)
        def native(phase):
            journey.native_labels.extend(kind + '-' + phase + '-' + scope
                                         for scope in ('a', 'b') for kind in ('mcp', 'skills'))
        journey.public, journey.native = public, native
        with mock.patch.object(journey, "quarantine"):
            journey.execute()
        self.assertEqual(calls[-1], ('uap-remove', {'removed': True}))

    def test_native_replacement_refuses_before_any_post_command_body(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve(); ctx = self.context(root)
            path = root / 'user-config'; path.write_bytes(b'original')
            before = case.base.metadata_inventory(root, ctx.budget)
            journey = object.__new__(case.Journey)
            journey.ctx, journey.native_labels = ctx, []
            journey.frame = mock.Mock(return_value=(before, {}))
            def probe(*args, **kwargs):
                other = root / 'private'; other.write_bytes(b'foreign secret')
                other.replace(path)
                return {'exit': 0}, b'[]', b''
            ctx.probe = probe
            with mock.patch.object(os, 'read', side_effect=AssertionError('foreign body read')):
                with self.assertRaisesRegex(ValueError, 'nonauxiliary authority'):
                    journey.native('installed')
            journey.frame.assert_called_once_with()

    def test_original_uap_root_replacement_refuses_before_body_capture(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve(); ctx = self.context(root)
            state = root / "home/uap-state"; state.mkdir(parents=True)
            original = case.base.root_identity(state)
            state.rename(root / "moved-state")
            state.mkdir(); (state / "foreign").write_bytes(b"foreign private body")
            with mock.patch.object(os, 'read', side_effect=AssertionError('foreign bytes read')):
                with self.assertRaisesRegex(ValueError, 'original directory authority changed'):
                    case.capture(ctx, (1, '0' * 64), authorities={'home/uap-state': original})

    def test_successful_public_command_rejoins_original_roots_before_state_bytes(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve(); ctx = self.context(root)
            state = root / 'home/uap-state'; state.mkdir(parents=True)
            (state / 'state-v2.json').write_text('{"schema_version":5,"installations":[]}')
            journey = object.__new__(case.Journey)
            journey.ctx, journey.home, journey.installer = ctx, state, root / 'agentplugins'
            journey.identifier = 'owned-id'
            journey.cli_pin = (1, '0' * 64)
            journey.authorities = {'home/uap-state': case.base.root_identity(state)}
            before = case.base.metadata_inventory(root, ctx.budget)
            journey.frame = mock.Mock(return_value=(before, {}))
            ctx.env, ctx.projects = {'PATH': '/usr/bin'}, {'user': root}
            def command(*args):
                state.rename(root / 'retained-original-state')
                state.mkdir()
                (state / 'state-v2.json').write_bytes(b'foreign private state')
                return {'exit': 0}, b'{"schema_version":1,"command":"doctor","result":"success","data":{}}', b''
            ctx.command = command
            with mock.patch.object(case, 'installer_pin', return_value=journey.cli_pin):
                with mock.patch.object(os, 'read', side_effect=AssertionError('foreign state bytes read')):
                    with self.assertRaisesRegex(ValueError, 'original directory authority changed'):
                        journey.public('uap-doctor-installed', ['doctor', 'owned-id'])

    def test_successful_public_source_leaf_swap_refuses_before_any_body(self):
        for label, operation in (("uap-repair", "repair"), ("uap-doctor-installed", "doctor")):
            with self.subTest(label=label), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary).resolve(); ctx = self.context(root)
                state = root / "home/uap-state"; state.mkdir(parents=True)
                source = root / "component-source"; source.mkdir()
                member = source / "plugin.json"; member.write_bytes(b"original source")
                journey = object.__new__(case.Journey)
                journey.ctx, journey.home, journey.profile, journey.installer = ctx, state, root / "profile", root / "agentplugins"
                journey.identifier = "owned-id"
                journey.cli_pin = (1, "0" * 64)
                journey.authorities = {path.relative_to(root).as_posix(): case.base.root_identity(path) for path in (state, source)}
                before = case.base.metadata_inventory(root, ctx.budget)
                journey.frame = mock.Mock(return_value=(before, {}))
                journey.state = lambda inventory, **kwargs: case.regular(ctx, "component-source/plugin.json", inventory)
                ctx.env, ctx.projects = {"PATH": "/usr/bin"}, {"user": root}
                def command(*args):
                    replacement = root / "replacement"; replacement.write_bytes(b"foreign private source")
                    replacement.replace(member)
                    return {"exit": 0}, json.dumps(dict(schema_version=1, command=operation, result="success", data={})).encode(), b""
                ctx.command = command
                with mock.patch.object(case, "installer_pin", return_value=journey.cli_pin):
                    with mock.patch.object(os, "read", side_effect=AssertionError("foreign source bytes read")) as read:
                        with self.assertRaisesRegex(ValueError, "public Global changed foreign file"):
                            journey.public(label, [operation, "owned-id"])
                        read.assert_not_called()



    def test_package_digest_uses_independent_literal_framing_and_default_reader(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve(); ctx = self.context(root)
            active = root / "package"; (active / "lib").mkdir(parents=True)
            for name, body, mode in (("a", b"A", 0o644), ("lib/b", b"BC", 0o644), ("run", b"#!", 0o700)):
                path = active / name; path.write_bytes(body); path.chmod(mode)
            inventory = case.base.metadata_inventory(root, ctx.budget)
            literal = (b"file\0a\0false\0" + b"1\0A" + b"dir\0lib\0false\0" + b"0\0"
                       + b"file\0lib/b\0false\0" + b"2\0BC" + b"file\0run\0true\0" + b"2\0#!")
            bodies, digest = case.capture_package(ctx, active, inventory, (1, "0" * 64))
            self.assertEqual(bodies, {"a": b"A", "lib/b": b"BC", "run": b"#!"})
            self.assertEqual(digest, "sha256:" + hashlib.sha256(literal).hexdigest())
            self.assertEqual(case.base.read_bootstrap_file(ctx, "package/a", inventory["package/a"], inventory), b"A")
            with mock.patch.object(os, "read", side_effect=AssertionError("invalid sink read bytes")):
                with self.assertRaisesRegex(ValueError, "SHA256 state"):
                    case.base.read_bootstrap_file(ctx, "package/a", inventory["package/a"], inventory, package_digest=hashlib.sha1())
            read = os.read
            def replace_after_read(fd, count):
                body = read(fd, count)
                if body:
                    info = (active / "a").stat()
                    os.utime(active / "a", ns=(info.st_atime_ns, info.st_mtime_ns + 2000000000))
                return body
            with mock.patch.object(os, "read", side_effect=replace_after_read):
                with self.assertRaisesRegex(ValueError, "bootstrap file changed"):
                    case.capture_package(ctx, active, inventory, (1, "0" * 64))
            self.assertNotIn("global_lifecycle_capture_complete", ctx.summary)

    def test_actual_state_consumer_joins_all_digests_and_keeps_retained_installation(self):
        with self.installed_state() as (journey, value, path, active, data):
            item = value["installations"][0]
            binding = next(iter(item["clients"].values()))
            for change in ("native", "package", "source", "revision", "data"):
                wrong = copy.deepcopy(value); selected = wrong["installations"][0]
                target = next(iter(selected["clients"].values()))
                if change == "native": target["native_objects"][1]["managed_digest"] = "sha256:" + "0" * 64
                elif change == "package":
                    target["native_objects"][0]["managed_digest"] = target["receipts"][0]["after_digest"] = "sha256:" + "0" * 64
                elif change == "source": selected["source"]["tree_digest"] = "sha256:" + "0" * 64
                elif change == "revision": target["package_revision"]["manifest_digest"] = "sha256:" + "0" * 64
                else: selected["data_receipts"][binding["data_receipt_id"]]["locator"] = str(data.parent / "foreign")
                path.write_text(json.dumps(wrong))
                with self.subTest(change=change), self.assertRaises(ValueError):
                    journey.state(case.base.metadata_inventory(journey.ctx.root, journey.ctx.budget))
            original = data.stat(); marker = data / ".agentplugins-data-owner.json"
            marker_before = marker.stat(); marker_body = marker.read_bytes()
            shutil.rmtree(active)
            shutil.rmtree(journey.profile / "skills" / case.SKILL)
            retained = copy.deepcopy(value)
            retained["installations"][0].update(clients={}, data_retained=True)
            path.write_text(json.dumps(retained))
            self.assertEqual(journey.state(case.base.metadata_inventory(journey.ctx.root, journey.ctx.budget), removed=True), retained)
            self.assertTrue(os.path.samestat(original, data.stat()))
            self.assertEqual((marker.stat().st_ino, marker.stat().st_mode, marker.stat().st_mtime_ns, marker.read_bytes()),
                             (marker_before.st_ino, marker_before.st_mode, marker_before.st_mtime_ns, marker_body))
            for wrong in (dict(schema_version=5, installations=[]), value):
                path.write_text(json.dumps(wrong))
                with self.assertRaises(ValueError):
                    journey.state(case.base.metadata_inventory(journey.ctx.root, journey.ctx.budget), removed=True)

    def test_pending_native_intent_and_authentication_attestation_are_typed(self):
        with self.installed_state() as (journey, value, path, _active, _data):
            for field, invalid in (("pending_native_intent", {}), ("pending_native_intent", False),
                                   ("pending_native_intent", 0), ("pending_native_intent", []),
                                   ("authentication_attested", True), ("authentication_attested", 0),
                                   ("authentication_attested", None)):
                wrong = copy.deepcopy(value)
                next(iter(wrong["installations"][0]["clients"].values()))[field] = invalid
                path.write_text(json.dumps(wrong))
                with self.subTest(field=field, invalid=invalid), self.assertRaisesRegex(ValueError, "Global binding authority"):
                    journey.state(case.base.metadata_inventory(journey.ctx.root, journey.ctx.budget))

    def test_committed_receipts_preserve_consecutive_directory_swap_history(self):
        with self.installed_state() as (journey, value, path, _active, _data):
            binding = next(iter(value["installations"][0]["clients"].values()))
            original = copy.deepcopy(binding["receipts"][0])
            second = dict(original, operation_id="operation-2", sequence=2)
            for receipts in ([dict(original, mutation_type="entry_write")], [dict(original, sequence=True)],
                             [original, original], [original, dict(second, sequence=3)]):
                binding["receipts"] = receipts; path.write_text(json.dumps(value))
                with self.subTest(receipts=receipts), self.assertRaisesRegex(ValueError, "Global committed public receipts"):
                    journey.state(case.base.metadata_inventory(journey.ctx.root, journey.ctx.budget))
            binding["receipts"] = [original, second]; path.write_text(json.dumps(value))
            journey.state(case.base.metadata_inventory(journey.ctx.root, journey.ctx.budget))
            for receipts in ([original], [dict(original, operation_id="rewritten"), second], [second]):
                binding["receipts"] = receipts; path.write_text(json.dumps(value))
                with self.subTest(receipts=receipts), self.assertRaisesRegex(ValueError, "Global committed public receipts"):
                    journey.state(case.base.metadata_inventory(journey.ctx.root, journey.ctx.budget))

    def test_public_grouped_repair_requires_its_second_committed_receipt(self):
        for append in (False, True):
            with self.subTest(append=append), self.installed_state() as (journey, value, path, _active, _data):
                journey.installer = journey.ctx.private / "hosted-agentplugins"
                journey.public_labels = ["uap-add", "uap-doctor-installed"]
                journey.ctx.env = {"PATH": os.defpath}
                def command(*_args):
                    if append:
                        binding = next(iter(value["installations"][0]["clients"].values()))
                        binding["receipts"].append(dict(binding["receipts"][0], operation_id="operation-2", sequence=2))
                        path.write_text(json.dumps(value))
                    return {"exit": 0, "error": None}, json.dumps(dict(schema_version=1, command="repair", result="success", data={})).encode(), b""
                journey.ctx.command = command
                with mock.patch.object(case, "installer_pin", return_value=journey.cli_pin):
                    if append:
                        journey.public("uap-repair", ["repair", journey.identifier])
                        self.assertEqual(journey.public_labels[-1], "uap-repair")
                    else:
                        with self.assertRaisesRegex(ValueError, "Global public receipt sequence"):
                            journey.public("uap-repair", ["repair", journey.identifier])
                        self.assertEqual(journey.ctx.summary.get("command_effects", []), [])

    def test_coherent_unauthored_projection_digests_do_not_create_ownership(self):
        for change in ("disabled", "approval", "cwd", "headers", "type", "schema"):
            with self.subTest(change=change), self.installed_state() as (journey, value, path, active, _data):
                projection = active / "mcp.json"
                document = json.loads(projection.read_bytes())
                selected = document["mcpServers"]
                if change == "disabled": selected[case.SERVERS[0]]["disabled"] = True
                elif change == "approval": selected[case.SERVERS[0]]["autoApprove"] = ["unreviewed"]
                elif change == "cwd": selected[case.SERVERS[0]]["cwd"] = str(active)
                elif change == "headers": selected[case.SERVERS[1]]["headers"] = {"Authorization": "foreign"}
                elif change == "type": selected[case.SERVERS[1]]["type"] = "sse"
                else: document.pop("$schema")
                projection.write_text(json.dumps(document))
                native_path = journey.profile / "mcp_config.json"
                native = json.loads(native_path.read_bytes())
                for name in case.SERVERS:
                    entry = dict(selected[name]); entry.pop("type")
                    if name == case.SERVERS[1]: entry["transport"] = "http"
                    native["mcpServers"][name] = entry
                native_path.write_text(json.dumps(native))
                inventory = case.base.metadata_inventory(journey.ctx.root, journey.ctx.budget)
                _, digest = case.capture_package(journey.ctx, active, inventory, journey.cli_pin)
                binding = next(iter(value["installations"][0]["clients"].values()))
                binding["receipts"][-1]["after_digest"] = digest
                for obj in binding["native_objects"]:
                    if obj["kind"] == "managed_package_directory": obj["managed_digest"] = digest
                    elif obj["kind"] == "devin_global_mcp_server":
                        obj["managed_digest"] = case.entry_digest(obj["logical_name"], native["mcpServers"][obj["logical_name"]])
                path.write_text(json.dumps(value))
                journey.installation_basis = journey.binding_basis = journey.data_authority = journey.binding = None
                with self.assertRaisesRegex(ValueError, "Global projected (transport|document settings)"):
                    journey.state(case.base.metadata_inventory(journey.ctx.root, journey.ctx.budget))
                self.assertEqual((journey.source / "mcp.json").read_bytes(), journey.source_bodies["mcp.json"])

    def test_canonical_devin_digest_is_distinct_from_raw_json_and_skill_hash(self):
        entry = {"url": "https://example.invalid/?a=1&b=2", "headers": {"z": "\u2028", "a": "<owned>"}}
        literal = b'{"headers":{"a":"\\u003cowned\\u003e","z":"\\u2028"},"url":"https://example.invalid/?a=1\\u0026b=2"}'
        expected = hashlib.sha256(b"agentplugins-native-mcp-v1\0devin-mcpServers\0owned\0" + literal).hexdigest()
        self.assertEqual(case.entry_digest("owned", entry), "sha256:" + expected)
        self.assertNotEqual(case.entry_digest("other", entry), "sha256:" + expected)

    def test_doctor_accepts_only_the_truthful_prepared_result(self):
        identifier = "owned-id"
        data = dict(read_only=True, installation_count=1, open_operation_count=0,
                    installation=dict(installation_id=identifier, name=case.NAME, clients=[dict(client_id="windsurf", scope="user",
                        activation="prepared", materialization="materialized", authentication="not_checked", policy="allowed",
                        verification="installation_verified")]),
                    findings=[dict(status=status, code=code, installation_id=identifier, installation_name=case.NAME, client_id="windsurf")
                              for status, code in (("degraded", "activation_pending"), ("unknown", "authentication_not_checked"),
                                                   ("degraded", "native_projection_changed"))])
        case.doctor_result(data, identifier)
        for change in ("write", "healthy", "active", "count", "missing"):
            wrong = copy.deepcopy(data)
            if change == "write": wrong["read_only"] = False
            elif change == "healthy": wrong["findings"][2].update(status="healthy", code="no_degradation_detected")
            elif change == "active": wrong["installation"]["clients"][0]["activation"] = "active"
            elif change == "count": wrong["open_operation_count"] = False
            else: wrong["findings"].pop()
            with self.subTest(change=change), self.assertRaises(ValueError): case.doctor_result(wrong, identifier)

    def test_global_completion_requires_two_original_build_streams(self):
        value = self.completed_summary()
        for change in ("bool", "source", "bytes", "version", "command", "extra", "owner"):
            wrong = copy.deepcopy(value)
            receipt = wrong["global_build_receipt"]
            if change == "bool": receipt["commands"][0]["exit"] = False
            elif change == "source": receipt["source_tree"] = "d" * 40
            elif change == "bytes": receipt["commands"][0]["stdout_bytes"] += 1
            elif change == "version": receipt["streams"]["build-go-version.stdout.log"] = base64.b64encode(b"go version go1.26.10 linux/amd64\n").decode()
            elif change == "command": receipt["commands"][1]["argv"].append("-tags=unreviewed")
            elif change == "extra": receipt["commands"].append(receipt["commands"][0])
            else: receipt["installer"]["uid"] += 1
            with self.subTest(change=change), self.assertRaises(ValueError): case.completion(wrong)

    def test_ordinary_global_gate_preserves_actual_argv_and_first_migration_gate(self):
        ctx = SimpleNamespace(summary=dict(metadata_recipe="ordinary-global", metadata_route=case.ordinary.OrdinaryEffects.metadata_route,
                              metadata_capture_complete=True, commands=[], command_effects=[{} for _ in case.base.METADATA_PROBES]),
                              argv=["/owned/devin"], projects={scope: Path("/owned/" + scope) for scope in ("a", "b", "user")},
                              root=Path("/owned"), budget=object())
        expected = [(label, args, "a") for label, args in case.base.HELP]
        expected += [(kind + "-" + label, [kind, "list", "--json"], scope)
                     for label, scope in (("a", "a"), ("b", "b"), ("user", "user"), ("a-repeat", "a")) for kind in ("mcp", "skills")]
        ctx.summary["commands"] = [dict(probe=label, argv=[*ctx.argv, *args], cwd=str(ctx.projects[scope])) for label, args, scope in expected]
        effects = SimpleNamespace(verify=mock.Mock(), step=13, after={"observed": True})
        with mock.patch.object(case.base, "metadata_inventory", return_value={}):
            case.complete_ordinary_gate(ctx, effects)
            for step, after in ((12, effects.after), (13, None)):
                effects.step, effects.after = step, after
                with self.assertRaisesRegex(ValueError, "gate incomplete"): case.complete_ordinary_gate(ctx, effects)
        ctx.summary["commands"][-1]["argv"].insert(1, "--config")
        with self.assertRaisesRegex(ValueError, "argv or scope"):
            case.ordinary.require_ordinary_commands(ctx, recipe="ordinary-global")
        with self.assertRaisesRegex(ValueError, "recipe or sequence"):
            case.ordinary.require_ordinary_commands(ctx)

    def test_readout_extends_only_the_fixed_global_catalog(self):
        value = self.completed_summary()
        value.update(collection_complete=True, cleanup_complete=True)
        state = dict(phase="source-final-clean", reason="metadata-complete", deadline=float(case.base.time.monotonic() + 10))
        readout = case.base.closed_readout(value, state)
        self.assertEqual((readout["commands"], len(readout["effects"])), (37, 30))
        value.update(collection_complete=False, metadata_recipe="ordinary")
        state.update(phase="mcp-a", reason="capture-incomplete")
        with self.assertRaisesRegex(ValueError, "readout counts"): case.base.closed_readout(value, state)



    def test_owned_skill_row_is_closed_in_the_actual_native_consumer(self):
        profile = Path("/owned/home/config/devin")
        for change in ("valid", "name", "description", "provider", "triggers", "warnings", "errors", "extra", "sibling"):
            journey = object.__new__(case.Journey)
            ctx = SimpleNamespace(root=Path("/owned"), projects={"a": Path("/owned/a"), "b": Path("/owned/b")},
                                  summary={}, budget=object())
            journey.ctx, journey.profile, journey.expected, journey.nonce = ctx, profile, {"user": {"mcp": {}}}, "n" * 64
            journey.probes, journey.log_births, journey.native_labels = {}, {}, []
            journey.frame = mock.Mock(return_value=({}, {}))
            journey.read = lambda *_args: json.dumps({"mcpServers": {name: {} for name in case.SERVERS}}).encode()
            row = case.owned_skill_row(ctx, profile, "a")
            if change == "sibling": row["base_dir"] = str(profile / "skills/foreign")
            elif change == "extra": row["unreviewed"] = True
            elif change != "valid": row[change] = "different"
            ctx.probe = mock.Mock(side_effect=[({"exit": 0}, json.dumps([{"name": name} for name in case.SERVERS]).encode(), b""),
                ({"exit": 0}, json.dumps([row]).encode(), b""), StopIteration("next scope")])
            with mock.patch.object(case.base, "metadata_inventory", return_value={}):
                with mock.patch.object(case.base, "verify_metadata_sources"), mock.patch.object(case.base, "verify_skill_metadata_sources"):
                    with mock.patch.object(case.base, "zero_effects"):
                        with self.subTest(change=change), self.assertRaises(StopIteration if change == "valid" else ValueError):
                            journey.native("installed")
            self.assertEqual(ctx.probe.call_count, 3 if change == "valid" else 2)
            self.assertEqual(journey.native_labels, ["mcp-installed-a", "skills-installed-a"] if change == "valid" else ["mcp-installed-a"])

    def test_build_commands_are_separate_bounded_original_receipts(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            go, installer = root / "go", root / "installer"
            go.write_bytes(b"inert-go"); go.chmod(0o755)
            ctx = SimpleNamespace(summary=dict(source_commit="a" * 40, source_tree="b" * 40, commands=[]),
                                  budget=SimpleNamespace(check=lambda: 20), private=root, identities={},
                                  env={"HOME": "/native-profile"}, process_cleanup_proved=True)
            def run(argv, cwd, environment, _output, label, **kwargs):
                self.assertEqual(environment.get("HOME"), os.environ.get("HOME"))
                self.assertEqual(environment["GOTOOLCHAIN"], "local")
                self.assertEqual((kwargs["timeout"], kwargs["limit"], kwargs["memory_only"]),
                                 (10 if label == "build-go-version" else 20, 1 << 20, True))
                receipt = self.build_receipt()
                index = 0 if label == "build-go-version" else 1
                record = receipt["commands"][index]; record["argv"], record["cwd"] = argv, str(cwd)
                bodies = [base64.b64decode(receipt["streams"][label + "." + kind + ".log"]) for kind in ("stdout", "stderr")]
                if label == "build-uap": installer.write_bytes(b"inert-built-cli"); installer.chmod(0o755)
                return record, *bodies
            with mock.patch.object(case.shutil, "which", return_value=str(go)):
                with mock.patch.object(case.base.shared, "run_probe", side_effect=run) as execute:
                    pin = case.build_installer(ctx, installer)
                    self.assertEqual(pin, (15, hashlib.sha256(b"inert-built-cli").hexdigest()))
                    self.assertEqual(execute.call_count, 2)
                    self.assertEqual(ctx.summary["commands"], [])
                    self.assertTrue(ctx.process_cleanup_proved)
                    with self.assertRaisesRegex(ValueError, "fresh installer output"): case.build_installer(ctx, installer)
                    self.assertEqual(execute.call_count, 2)



if __name__ == "__main__":
    unittest.main()
