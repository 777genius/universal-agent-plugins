"""Inert contracts: no native client, network acquisition or hosted spoofing."""
import copy
import hashlib
import json
import os
from pathlib import Path
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import Mock, patch

import devin_structured_inventory_admission as case


class DevinAdmissionContracts(unittest.TestCase):
    def test_unique_bounded_native_json(self):
        self.assertEqual(case.strict_json(b'[{"name":"owned","enabled":false}]'), [{"name": "owned", "enabled": False}])
        for body in (b'{"a":1,"a":2}', b'{} {}', b'{"a":NaN}', b'{"x":1e9999}', b'{"x":-1e9999}', b'null', b'x' * ((1 << 20) + 1), b'[' * 18 + b'0' + b']' * 18):
            with self.subTest(body=body[:40]), self.assertRaises(ValueError):
                case.strict_json(body)

    def test_archive_requires_reviewed_binary_not_old_pin_or_executable_companion(self):
        files = [{"name": "bin/devin", "kind": "file", "bytes": 213734376, "executable": True, "sha256": case.BINARY}]
        files += [{"name": "companion-" + str(i), "kind": "file", "bytes": 0, "executable": False} for i in range(233)]
        files[1]["bytes"] = 214280841 - files[0]["bytes"]
        files += [{"name": "dir-" + str(i), "kind": "directory", "bytes": 0, "executable": True} for i in range(16)]
        receipt = {"pin_verified": True, "sha256": case.ARCHIVE, "bytes": 72925933, "members": files}
        case.admitted_archive(receipt)
        for change in ("old-pin", "wrong-binary", "special", "executable", "duplicate-count"):
            bad = copy.deepcopy(receipt)
            if change == "old-pin": bad["sha256"] = "0" * 64
            elif change == "wrong-binary": bad["members"][0]["sha256"] = "0" * 64
            elif change == "special": bad["members"][1]["kind"] = "link"
            elif change == "executable": bad["members"][1]["executable"] = True
            else: bad["members"].pop()
            with self.subTest(change=change), self.assertRaises(ValueError): case.admitted_archive(bad)

    def test_connection_guard_includes_non_rpc_connections_and_stdio_start(self):
        probe = SimpleNamespace(methods=lambda: (), call_records=lambda: ([], 0), accepted_connections=[])
        with tempfile.TemporaryDirectory() as temporary:
            marker = Path(temporary) / "started"
            case.zero_effects({"owned": probe}, marker)
            probe.accepted_connections.append(("127.0.0.1", 1234))
            with self.assertRaises(ValueError): case.zero_effects({"owned": probe}, marker)
            probe.accepted_connections.clear()
            marker.write_text("unexpected")
            with self.assertRaises(ValueError): case.zero_effects({"owned": probe}, marker)

    def test_probe_observer_preserves_existing_handler_and_sets_connection_deadline(self):
        connection = Mock()
        address = ("127.0.0.1", 1234)
        server = SimpleNamespace(get_request=lambda: (connection, address))
        probe = case.observe_connections(SimpleNamespace(server=server))
        self.assertEqual(probe.server.get_request(), (connection, address))
        connection.settimeout.assert_called_once_with(2)
        self.assertEqual(probe.accepted_connections, [address])

    def test_runner_exception_keeps_process_cleanup_fence_closed(self):
        ctx = object.__new__(case.AdmissionContext)
        ctx.summary = {}
        with patch.object(case.shared.Context, "command", side_effect=RuntimeError("cleanup failed")):
            with self.assertRaisesRegex(RuntimeError, "cleanup failed"):
                ctx.command("native-list", [])
        self.assertFalse(ctx.process_cleanup_proved)
        self.assertEqual(ctx.summary["in_flight_command"], "native-list")
        ctx = object.__new__(case.AdmissionContext)
        ctx.summary = {}
        negative = ({"exit": 1}, b"", b"authentication refused")
        with patch.object(case.shared.Context, "command", return_value=negative):
            self.assertEqual(ctx.command("native-list", []), negative)
        self.assertTrue(ctx.process_cleanup_proved)
        self.assertNotIn("in_flight_command", ctx.summary)

    def test_outer_inventory_detects_new_runtime_companion_and_unknown_directory(self):
        with tempfile.TemporaryDirectory() as temporary, case.shared.Budget(10) as budget:
            root = Path(temporary).resolve()
            runtime = root / "client"
            runtime.mkdir()
            (runtime / "devin").write_bytes(b"inert original runtime")
            before = case.metadata_inventory(root, budget)
            companion = runtime / "new-companion"
            companion.write_bytes(b"unexpected")
            self.assertNotEqual(before, case.metadata_inventory(root, budget))
            companion.unlink()
            unknown = root / "unknown-native-directory"
            unknown.mkdir()
            self.assertNotEqual(before, case.metadata_inventory(root, budget))
            unknown.rmdir()
            (runtime / "alias").symlink_to(runtime / "devin")
            with self.assertRaisesRegex(ValueError, "alias/special"):
                case.metadata_inventory(root, budget)

    def test_fixture_preserves_original_profile_legacy_and_collision_sources(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            home = root / "home"; home.mkdir()
            projects = {name: root / ("project-" + name) for name in ("a", "b", "user")}
            for path in projects.values(): path.mkdir()
            ctx = SimpleNamespace(root=root, home=home, projects=projects, env={"XDG_CONFIG_HOME": str(home / "config")})
            urls = {name: "http://127.0.0.1:" + str(1000 + i) + "/mcp" for i, name in enumerate(("a", "b", "user", "local", "legacy"))}
            expected, startup, marker = case.fixture(ctx, urls, "f" * 64)
            self.assertFalse(startup.exists())
            self.assertIn("unexpected native start", marker.read_text())
            for name in ("a", "b", "user"):
                entry = expected[name]
                body = (Path(entry["root"]) / "mcp_config.json").read_bytes()
                self.assertEqual(hashlib.sha256(body).hexdigest(), entry["mcp_config_sha256"])
                self.assertEqual(json.loads(body)["mcpServers"]["same-name"]["url"], urls[name])
                self.assertFalse(json.loads(body)["mcpServers"]["same-name"]["disabled"])
            self.assertEqual(json.loads(Path(expected["local"]["path"]).read_text())["mcpServers"]["same-name"]["url"], urls["local"])
            self.assertEqual(json.loads((home / ".codeium/windsurf/mcp_config.json").read_text())["mcpServers"]["legacy-foreign"], {"url": urls["legacy"], "disabled": True})
            self.assertIn("inert_legacy_sentinel", (home / ".codeium/windsurf/config.json").read_text())
            self.assertIn("legacy-foreign", (home / ".codeium/windsurf/skills/legacy-foreign/SKILL.md").read_text())




class KnownFileDiagnosticContracts(unittest.TestCase):
    def fixture(self, root):
        for relative in case.KnownFileDiagnostics.PATHS.values():
            path = root / relative
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(b'{"read_config_from":{"cloud":false}}\n' if relative.endswith('config.json') else b'# owned Git template\n')
        return root / case.KnownFileDiagnostics.PATHS['user-config'], root / case.KnownFileDiagnostics.PATHS['git-a-exclude']

    def hold(self, root):
        baseline = {}
        for label, relative in case.KnownFileDiagnostics.PATHS.items():
            body = (root / relative).read_bytes()
            key = "home/config/devin/config.json" if label == "user-config" else "a/.git/info/exclude"
            baseline[key] = {"kind": "file", "size": len(body), "sha256": hashlib.sha256(body).hexdigest()}
        return case.KnownFileDiagnostics(root, case.root_identity(root), baseline)

    def test_original_root_and_fixture_join_precede_admission(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary) / 'owned'
            config, _ = self.fixture(root)
            original = case.root_identity(root)
            root.rename(root.with_name('old'))
            root.mkdir()
            (root / 'foreign').write_text('preserve')
            with patch.object(case.os, 'read', side_effect=AssertionError('foreign read')), self.assertRaises(ValueError):
                case.KnownFileDiagnostics(root, original, {})
            self.assertEqual((root / 'foreign').read_text(), 'preserve')
            config, _ = self.fixture(root)
            config.write_bytes(b'{"unexpected":true}')
            with self.assertRaises(ValueError): self.hold(root)
            config, exclude = self.fixture(root)
            baseline = {
                key: {'kind': 'file', 'size': path.stat().st_size, 'sha256': hashlib.sha256(path.read_bytes()).hexdigest()}
                for key, path in [('home/config/devin/config.json', config), ('a/.git/info/exclude', exclude)]}
            exclude.write_bytes(b'foreign replacement')
            with self.assertRaises(ValueError):
                case.KnownFileDiagnostics(root, case.root_identity(root), baseline)

    def test_replaced_file_is_observed_without_renewing_original_or_leaking_values(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            config, exclude = self.fixture(root)
            held = self.hold(root)
            try:
                original = copy.deepcopy(held.before)
                replacement = config.with_suffix('.replacement')
                replacement.write_bytes(b'{"read_config_from":{"cloud":false},"setup":true,"token":"PRIVATE","count":123,"bad-key":"PRIVATE"}')
                replacement.replace(config)
                exclude.write_bytes(exclude.read_bytes() + b'.devin/cache/\n/private/SECRET\n')
                result = held.finish()
                self.assertEqual(result['files']['user-config']['before'], original['user-config'])
                self.assertNotEqual(result['files']['user-config']['after']['identity'], original['user-config']['identity'])
                text = json.dumps(result)
                self.assertNotIn('PRIVATE', text)
                self.assertNotIn('SECRET', text)
                self.assertNotIn('bad-key', text)
                self.assertIn('"setup", "value": true', text)
                self.assertNotIn('"value": 123', text)
                self.assertIn(b'PRIVATE', config.read_bytes())
            finally:
                held.close()

    def test_missing_large_alias_hardlink_malformed_and_parent_replacement_refuse(self):
        for change in ('missing', 'large', 'alias', 'hardlink', 'malformed', 'parent', 'root'):
            with self.subTest(change=change), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary) / 'owned'
                config, exclude = self.fixture(root)
                held = self.hold(root)
                try:
                    if change == 'missing': config.unlink()
                    elif change == 'large': config.write_bytes(b'x' * 4097)
                    elif change == 'alias':
                        config.unlink()
                        config.symlink_to(exclude)
                    elif change == 'hardlink':
                        import os
                        os.link(config, config.with_suffix('.alias'))
                    elif change == 'malformed': config.write_bytes(b'{"setup":true,"setup":false}')
                    else:
                        target = config.parent if change == 'parent' else root
                        target.rename(target.with_name(target.name + '-old'))
                        target.mkdir()
                        (target / 'foreign').write_text('preserve')
                    with self.assertRaises((ValueError, OSError)):
                        held.finish()
                    if change in ('parent', 'root'):
                        self.assertEqual((target / 'foreign').read_text(), 'preserve')
                finally:
                    held.close()

    def test_mid_read_identity_race_refuses(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            config, _exclude = self.fixture(root)
            held = self.hold(root)
            original_read = case.os.read
            def racing_read(fd, limit):
                result = original_read(fd, limit)
                config.write_bytes(b'{"changed":true}')
                return result
            try:
                with patch.object(case.os, 'read', side_effect=racing_read), self.assertRaises(ValueError):
                    held.finish()
            finally:
                held.close()


class ProspectiveBootstrapContracts(unittest.TestCase):
    def fixture(self, root):
        root = Path(root).resolve()
        for name in ("home/cache", "home/config/devin", "home/data", "private"):
            (root / name).mkdir(parents=True, exist_ok=True)
        config = root / case.METADATA_CONFIG
        config.write_bytes(b'{"read_config_from":{"cloud":false}}\n'); config.chmod(0o600)
        user = root / "home/config/devin/config.json"
        user.write_bytes(config.read_bytes()); user.chmod(0o644)
        ctx = SimpleNamespace(root=root, original_output_identity=case.root_identity(root),
                              budget=SimpleNamespace(check=lambda: None))
        baseline = case.metadata_inventory(root, ctx.budget)
        return ctx, case.BootstrapEffects(ctx, baseline), user

    def create_aux(self, ctx):
        for name in sorted(case.BOOTSTRAP_DIRS, key=lambda item: len(Path(item).parts)):
            (ctx.root / name).mkdir(mode=0o755, exist_ok=True)
        for name, body in ((case.INSTALLATION_ID, b"12345678-1234-1234-1234-123456789abc"),
                           (case.MAINTENANCE, b"1234\n"),
                           ("home/data/devin/cli/logs/devin_20261113-093816_2273.log", b"inert bounded log")):
            (ctx.root / name).write_bytes(body); (ctx.root / name).chmod(0o644)

    def test_prospective_creation_and_version_do_not_renew_original_user_baseline(self):
        with tempfile.TemporaryDirectory() as root:
            ctx, effects, user = self.fixture(root)
            original = copy.deepcopy(effects.baseline)
            effects.verify(case.metadata_inventory(ctx.root, ctx.budget))
            self.create_aux(ctx)
            (ctx.root / case.METADATA_CONFIG).write_bytes(b'{"version":1,"read_config_from":{"cloud":false}}')
            effects.verify(case.metadata_inventory(ctx.root, ctx.budget))
            self.assertEqual(effects.baseline, original)
            self.assertEqual(user.read_bytes(), b'{"read_config_from":{"cloud":false}}\n')
            self.assertTrue(effects.version_seen)
            self.assertNotIn("inert bounded log", json.dumps(effects.proofs))
            user.write_bytes(b'{"version":1}')
            with self.assertRaisesRegex(ValueError, "protected original"):
                effects.verify(case.metadata_inventory(ctx.root, ctx.budget))

    def test_unknown_settings_paths_and_unmeasured_formats_are_refused(self):
        changes = ("permission", "numeric-cloud", "oversize-config", "unknown", "oauth", "uuid", "maintenance", "date", "fixed")
        for change in changes:
            with self.subTest(change=change), tempfile.TemporaryDirectory() as root:
                ctx, effects, _ = self.fixture(root); self.create_aux(ctx)
                leaf = ctx.root / case.METADATA_CONFIG
                if change == "permission": leaf.write_bytes(b'{"read_config_from":{"cloud":false},"permissions":{}}')
                elif change == "numeric-cloud": leaf.write_bytes(b'{"read_config_from":{"cloud":0}}')
                elif change == "oversize-config": leaf.write_bytes(b" " * 256 + b'{}')
                elif change == "unknown": (ctx.root / "home/cache/devin/unknown").write_bytes(b"unknown")
                elif change == "oauth": (ctx.root / "home/data/devin/mcp/oauth/token").write_bytes(b"secret")
                elif change == "uuid": (ctx.root / case.INSTALLATION_ID).write_bytes(b"not-measured")
                elif change == "maintenance": (ctx.root / case.MAINTENANCE).write_bytes(b"1234")
                elif change == "date": (ctx.root / "home/data/devin/cli/logs/devin_20261313-093816_2273.log").write_bytes(b"log")
                else: (ctx.root / "home/cache/devin/telemetry_state.json").write_bytes(b"unreviewed")
                with self.assertRaises(ValueError): effects.verify(case.metadata_inventory(ctx.root, ctx.budget))

    def test_auxiliary_aliases_modes_and_file_bounds_are_not_adopted(self):
        for change in ("hardlink", "symlink", "mode", "size"):
            with self.subTest(change=change), tempfile.TemporaryDirectory() as root:
                ctx, effects, user = self.fixture(root); self.create_aux(ctx)
                leaf = ctx.root / case.MAINTENANCE
                if change == "hardlink": leaf.unlink(); os.link(user, leaf)
                elif change == "symlink": leaf.unlink(); leaf.symlink_to(user)
                elif change == "mode": leaf.chmod(0o600)
                else: leaf.write_bytes(b"x" * 16385)
                with self.assertRaises(ValueError): effects.verify(case.metadata_inventory(ctx.root, ctx.budget))

    def test_continuity_and_no_version_regression_are_additional_constraints(self):
        for change in ("uuid", "version"):
            with self.subTest(change=change), tempfile.TemporaryDirectory() as root:
                ctx, effects, _ = self.fixture(root); self.create_aux(ctx)
                config = ctx.root / case.METADATA_CONFIG
                config.write_bytes(b'{"read_config_from":{"cloud":false},"version":1}')
                effects.verify(case.metadata_inventory(ctx.root, ctx.budget))
                if change == "uuid": (ctx.root / case.INSTALLATION_ID).write_bytes(b"87654321-1234-1234-1234-123456789abc")
                else: config.write_bytes(b'{"read_config_from":{"cloud":false}}')
                with self.assertRaises(ValueError): effects.verify(case.metadata_inventory(ctx.root, ctx.budget))

    def test_log_birth_count_and_original_snapshot_remain_bounded(self):
        with tempfile.TemporaryDirectory() as root:
            ctx, effects, _ = self.fixture(root)
            before = case.metadata_inventory(ctx.root, ctx.budget)
            self.create_aux(ctx)
            after = case.metadata_inventory(ctx.root, ctx.budget)
            effects.verify_command(before, after)
            (ctx.root / "home/data/devin/cli/logs/devin_20261113-093817_2273.log").write_bytes(b"second")
            with self.assertRaisesRegex(ValueError, "multiple logs"):
                effects.verify_command(before, case.metadata_inventory(ctx.root, ctx.budget))
            baseline = {"home/data": {"kind": "directory", "mode": "0o755", "size": 10},
                        "home/config/devin/config.json": {"kind": "file", "size": 37, "sha256": "original"}}
            changed = copy.deepcopy(baseline); changed["home/data"]["size"] = 20
            effects.verify_snapshot(baseline, changed)
            changed["home/config/devin/config.json"]["sha256"] = "rewritten"
            with self.assertRaises(ValueError): effects.verify_snapshot(baseline, changed)

    def test_original_parent_swap_refuses_before_foreign_bytes_are_read(self):
        with tempfile.TemporaryDirectory() as root:
            ctx, effects, _ = self.fixture(root)
            current = case.metadata_inventory(ctx.root, ctx.budget)
            private = ctx.root / "private"; private.rename(ctx.root / "old-private")
            private.mkdir(); foreign = private / "metadata-config.json"; foreign.write_bytes(b"foreign")
            real_read = case.os.read
            def guarded(fd, count):
                self.assertNotEqual(os.fstat(fd).st_ino, foreign.stat().st_ino)
                return real_read(fd, count)
            with patch.object(case.os, "read", side_effect=guarded), self.assertRaises(ValueError):
                case.read_bootstrap_file(ctx, case.METADATA_CONFIG, current[case.METADATA_CONFIG], current)
            self.assertEqual(foreign.read_bytes(), b"foreign")

    def test_mcp_source_binding_rejects_config_rebase_and_local_override_loss(self):
        with tempfile.TemporaryDirectory() as root:
            root = Path(root); home = root / "home"; home.mkdir()
            projects = {name: root / ("project-" + name) for name in ("a", "b", "user")}
            for path in projects.values(): path.mkdir()
            ctx = SimpleNamespace(root=root, home=home, projects=projects, env={"XDG_CONFIG_HOME": str(home / "config")})
            expected, _, _ = case.fixture(ctx, {name: "http://127.0.0.1:1234/" + name for name in ("a", "b", "user", "local", "legacy")}, "a" * 64)
            for scope in ("a", "b", "user"):
                wanted = {name: (entry, Path(expected["user"]["root"]) / "mcp_config.json") for name, entry in expected["user"]["mcp"].items()}
                if scope != "user": wanted.update({name: (entry, Path(expected[scope]["root"]) / "mcp_config.json") for name, entry in expected[scope]["mcp"].items()})
                if scope == "a": wanted["same-name"] = ({"url": expected["local"]["url"], "disabled": True}, Path(expected["local"]["path"]))
                wanted["legacy-foreign"] = ({"url": expected["legacy_url"], "disabled": True}, home / ".codeium/windsurf/mcp_config.json")
                rows = [{"name": name, "enabled": not entry.get("disabled", False), "disabled_reason": "disabled" if entry.get("disabled") else None,
                         "transport": "stdio" if "command" in entry else "http", "source": str(source), "url": entry.get("url"),
                         "command": entry.get("command"), "args": entry.get("args", []), "disabled_tools": entry.get("disabledTools", [])}
                        for name, (entry, source) in wanted.items()]
                case.verify_metadata_sources(json.dumps(rows).encode(), ctx, expected, scope)
                bad = copy.deepcopy(rows); bad[0]["source"] = str(root / "private/metadata-config.json")
                with self.assertRaises(ValueError): case.verify_metadata_sources(json.dumps(bad).encode(), ctx, expected, scope)
                if scope == "a":
                    bad = copy.deepcopy(rows)
                    next(row for row in bad if row["name"] == "same-name")["enabled"] = True
                    with self.assertRaises(ValueError): case.verify_metadata_sources(json.dumps(bad).encode(), ctx, expected, scope)


class SkillsMetadataContracts(unittest.TestCase):
    def fixture(self):
        root = Path('/owned'); ctx = SimpleNamespace(root=root, home=root / 'home', projects={key: root / ('project-' + key) for key in ('a', 'b')})
        expected = {key: {'skill': str(root / ('home/config/devin' if key == 'user' else 'project-' + key + '/.devin') / 'skills/same-skill/SKILL.md')} for key in ('a', 'b', 'user')}
        def row(name, description, provider, base, display, triggers):
            return dict(name=name, description=description, triggers=triggers, provider=provider, base_dir=base, display_name=display, warnings=[], errors=[])
        rows = [row('legacy-foreign', 'Preserve legacy Windsurf skill.', 'Windsurf', '/owned/home/.codeium/windsurf/skills/legacy-foreign', 'legacy-foreign', ['user', 'model'])]
        for source in ('user', 'a'):
            rows.append(row('/owned/project-a:same-skill', 'Inert ' + source + ' ' + 'f' * 64, 'Devin', str(Path(expected[source]['skill']).parent), 'same-skill', ['user', 'model']))
        rows += [
            row('devin-cli', 'Look up Devin CLI documentation (skills, extensibility, configuration, commands, models, troubleshooting)', 'Builtin', '/owned/client/share/devin', 'devin-cli', ['model']),
            row('declarative-repo-setup', 'Generate and verify a working environment.yaml (Devin snapshot-setup blueprint) for a repo', 'Builtin', '', 'declarative-repo-setup', ['model', 'user']),
            row('upload-secrets', 'Securely upload local secrets (dotenv files, env vars, API keys) to the Devin Cloud secrets manager — values never enter the conversation', 'Builtin', '', 'upload-secrets', ['model', 'user'])]
        return ctx, expected, rows

    def test_same_native_name_is_admitted_only_with_distinct_original_sources(self):
        ctx, expected, rows = self.fixture()
        self.assertEqual(rows[1]['name'], rows[2]['name'])
        case.verify_skill_metadata_sources(json.dumps(rows).encode(), ctx, expected, 'a', 'f' * 64)
        user = copy.deepcopy(rows); del user[2]; user[1]['name'] = 'same-skill'
        case.verify_skill_metadata_sources(json.dumps(user).encode(), ctx, expected, 'user', 'f' * 64)
        b = copy.deepcopy(rows)
        for i in (1, 2): b[i]['name'] = '/owned/project-b:same-skill'
        b[2].update(base_dir=str(Path(expected['b']['skill']).parent), description='Inert b ' + 'f' * 64)
        case.verify_skill_metadata_sources(json.dumps(b).encode(), ctx, expected, 'b', 'f' * 64)

    def test_foreign_duplicate_missing_legacy_builtin_and_changed_semantics_refuse(self):
        ctx, expected, original = self.fixture()
        for change in ('sibling', 'duplicate', 'missing', 'legacy', 'builtin-path', 'builtin-extra', 'nonce', 'name', 'triggers', 'warning', 'errors', 'extra-field', 'wrong-type'):
            rows = copy.deepcopy(original)
            if change == 'sibling': rows[2]['base_dir'] = str(Path(expected['b']['skill']).parent)
            elif change == 'duplicate': rows[2] = copy.deepcopy(rows[1])
            elif change == 'missing': rows.pop(2)
            elif change == 'legacy': rows[0]['provider'] = 'Devin'
            elif change == 'builtin-path': rows[3]['base_dir'] = '/foreign/share/devin'
            elif change == 'builtin-extra': rows.append(copy.deepcopy(rows[-1]))
            elif change == 'nonce': rows[2]['description'] = 'Inert a stale-nonce'
            elif change == 'name': rows[2]['name'] = 'same-skill'
            elif change == 'triggers': rows[2]['triggers'] = ['user']
            elif change == 'warning': rows[2]['warnings'] = ['native warning']
            elif change == 'errors': rows[2]['errors'] = {}
            elif change == 'extra-field': rows[2]['unexpected'] = True
            else: rows[2]['base_dir'] = []
            with self.subTest(change=change), self.assertRaises(ValueError):
                case.verify_skill_metadata_sources(json.dumps(rows).encode(), ctx, expected, 'a', 'f' * 64)


class ClosedDevinReadoutContracts(unittest.TestCase):
    def fixture(self):
        summary = dict(collection_complete=False, metadata_capture_complete=False, admission_qualified=False,
                       cleanup_complete=True, qualification=dict.fromkeys(case.shared.QUALIFICATION_CASES, False),
                       source_commit='a' * 40, source_tree='b' * 40,
                       error='ValueError: PRIVATE_CONFIG_UUID', commands=[dict(exit=1, error=None)],
                       command_effects=[dict(label='mcp-a', changes={'PRIVATE_CONFIG': {'before': None, 'after': {'private': 'PRIVATE_UUID'}}},
                                             outer_changes={'PRIVATE_PATH': {'before': [1], 'after': [2]}})])
        state = dict(phase='mcp-a', reason='bootstrap-path-refused', deadline=case.time.monotonic() + 5)
        return summary, state

    def test_fixed_projection_has_no_private_values_and_never_qualifies(self):
        summary, state = self.fixture()
        value = case.closed_readout(summary, state)
        self.assertEqual(value, dict(schema_version=1, phase='mcp-a', reason='bootstrap-path-refused',
                                    collection_complete=False, metadata_capture_complete=False, cleanup_complete=True,
                                    commands=1, failed_commands=1,
                                    effects=[dict(phase='mcp-a', fixture=dict(added=1, removed=0, changed=0),
                                                  outer=dict(added=0, removed=0, changed=1))]))
        for word in ('PRIVATE', 'UUID', 'source_commit', 'config', 'error'):
            self.assertNotIn(word, json.dumps(value))
        self.assertEqual(case.readout_reason(ValueError('unknown PRIVATE_CONFIG')), 'unclassified-refusal')
        self.assertEqual(case.readout_reason(ValueError('unknown native bootstrap path')), 'bootstrap-path-refused')

    def complete_fixture(self):
        summary, state = self.fixture()
        summary.pop('error')
        summary.update(collection_complete=True, metadata_capture_complete=True,
                       native_json_schema=case.METADATA_SCHEMA, exact_source_admission=case.METADATA_SCHEMA,
                       probe_methods=dict.fromkeys(('a', 'b', 'user', 'local', 'legacy'), []),
                       accepted_connections=dict.fromkeys(('a', 'b', 'user', 'local', 'legacy'), 0))
        names = ('source-commit', 'source-tree', 'source-clean', 'git-init-a', 'git-init-b', 'git-init-c') + case.METADATA_PROBES + ('source-final-clean',)
        summary['commands'] = [dict(probe=name, exit=0, error=None) for name in names]
        summary['command_effects'] = [dict(label=name, changes={}, outer_changes={}) for name in case.METADATA_PROBES]
        state.update(phase='source-final-clean', reason='metadata-complete')
        return summary, state

    def test_completion_requires_original_order_effects_and_zero_endpoint_activity(self):
        summary, state = self.complete_fixture()
        self.assertTrue(case.closed_readout(summary, state)['collection_complete'])
        for change in ('missing', 'order', 'bool-exit', 'error', 'effect', 'endpoint', 'methods', 'schema', 'cleanup', 'artifact', 'reason'):
            summary, state = self.complete_fixture()
            if change == 'missing': summary['commands'].pop()
            elif change == 'order': summary['commands'][0], summary['commands'][1] = summary['commands'][1], summary['commands'][0]
            elif change == 'bool-exit': summary['commands'][0]['exit'] = False
            elif change == 'error': summary['error'] = 'late failure'
            elif change == 'effect': summary['command_effects'].pop()
            elif change == 'endpoint': summary['accepted_connections']['legacy'] = 1
            elif change == 'methods': summary['probe_methods']['user'] = ['initialize']
            elif change == 'schema': summary['native_json_schema'] = 'unreviewed'
            elif change == 'cleanup': summary['cleanup_complete'] = False
            elif change == 'artifact': summary['artifact_admission_failed'] = True
            else: state['reason'] = 'capture-incomplete'
            with self.subTest(change=change), self.assertRaises(ValueError): case.closed_readout(summary, state)

    def test_successful_metadata_publication_preserves_false_runtime_and_failed_join_revokes_success(self):
        import zipfile
        for corrupt in (False, True):
            with self.subTest(corrupt=corrupt), tempfile.TemporaryDirectory() as temporary:
                summary, state = self.complete_fixture(); captures = {}
                for command in summary['commands']:
                    for stream in ('stdout', 'stderr'):
                        name = command['probe'] + '.' + stream + '.log'; captures[name] = b''
                        command.update({stream + '_file': name, stream + '_bytes': 0, stream + '_sha256': hashlib.sha256(b'').hexdigest()})
                if corrupt: summary['commands'][-1]['stdout_sha256'] = '0' * 64
                root = Path(temporary).resolve()
                with patch.object(case.os, 'write'):
                    case.publish_with_readout(summary, SimpleNamespace(captures=captures, secrets=[]), root, case.root_identity(root), state)
                with zipfile.ZipFile(root / 'upload/research.zip') as archive:
                    self.assertIsNone(archive.testzip()); committed = json.loads(archive.read('research.json'))
                self.assertIs(committed['collection_complete'], not corrupt)
                self.assertTrue(all(value is False for value in committed['qualification'].values()))
                if not corrupt: self.assertFalse(committed['admission_qualified'])

    def test_unknown_types_flags_bounds_and_deadline_refuse(self):
        for change in ('phase', 'reason', 'metadata', 'qualification', 'admission', 'cleanup', 'artifact', 'commands', 'missing-exit', 'effects', 'deadline'):
            with self.subTest(change=change):
                summary, state = self.fixture()
                if change in ('phase', 'reason'): state[change] = 'PRIVATE'
                elif change == 'metadata': summary['metadata_capture_complete'] = 0
                elif change == 'qualification': summary['qualification'][case.shared.QUALIFICATION_CASES[0]] = 0
                elif change == 'admission': summary['admission_qualified'] = True
                elif change == 'cleanup': summary['cleanup_complete'] = False
                elif change == 'artifact': summary['artifact_admission_failed'] = True
                elif change == 'commands': summary['commands'] *= 61
                elif change == 'missing-exit': del summary['commands'][0]['exit']
                elif change == 'effects': summary['command_effects'][0]['label'] = 'PRIVATE_PATH'
                else: state['deadline'] = case.time.monotonic() - 1
                with self.assertRaises(ValueError): case.closed_readout(summary, state)

    def test_actual_publisher_commits_before_finite_output_and_preserves_negative(self):
        import zipfile
        summary, state = self.fixture()
        summary['commands'] = []
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            original = case.root_identity(root)
            emitted = []
            def write(fd, body):
                self.assertEqual(fd, 1)
                with zipfile.ZipFile(root / 'upload/research.zip') as archive:
                    self.assertIsNone(archive.testzip())
                    committed = json.loads(archive.read('research.json'))
                    self.assertEqual(committed, summary)
                    self.assertFalse(committed['collection_complete'])
                emitted.append(body)
                return len(body)
            with patch.object(case.os, 'write', side_effect=write):
                case.publish_with_readout(summary, None, root, original, state)
            self.assertEqual(len(emitted), 1)
            self.assertNotIn(b'PRIVATE', emitted[0])
            self.assertEqual(json.loads(emitted[0].split(b'=', 1)[1])['phase'], 'mcp-a')
            self.assertFalse(summary['collection_complete'])

    def test_readout_failure_or_stdout_error_preserves_actual_committed_archive(self):
        import zipfile
        for failure in ('deadline', 'stdout-os', 'stdout-value', 'internal-admission'):
            with self.subTest(failure=failure), tempfile.TemporaryDirectory() as temporary:
                summary, state = self.fixture(); summary['commands'] = []
                if failure == 'deadline': state['deadline'] = case.time.monotonic() - 1
                if failure == 'internal-admission': summary['commands'] = [{}]
                root = Path(temporary).resolve(); emitted = []
                def write(_fd, body):
                    if failure == 'stdout-os': raise OSError('PRIVATE_STDOUT')
                    if failure == 'stdout-value': raise ValueError('PRIVATE_STDOUT')
                    emitted.append(body); return len(body)
                with patch.object(case.os, 'write', side_effect=write):
                    case.publish_with_readout(summary, None, root, case.root_identity(root), state)
                with zipfile.ZipFile(root / 'upload/research.zip') as archive:
                    self.assertIsNone(archive.testzip())
                    committed = json.loads(archive.read('research.json'))
                    self.assertFalse(committed['collection_complete'])
                if not failure.startswith('stdout'):
                    self.assertEqual(emitted, [b'DEVIN_CLOSED_READOUT={"schema_version":1,"reason":"readout-refused"}\n'])

    def test_failed_publication_emits_nothing_and_root_replacement_never_leaks(self):
        summary, state = self.fixture()
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            with patch.object(case.shared, 'publish', side_effect=OSError('PRIVATE_PUBLISH')), patch.object(case.os, 'write') as output:
                with self.assertRaises(OSError):
                    case.publish_with_readout(summary, None, root, case.root_identity(root), state)
                output.assert_not_called()
            original = case.root_identity(root)
            def replace(*_args):
                root.rename(root.with_name(root.name + '-original'))
                root.mkdir(); (root / 'foreign').write_bytes(b'retained')
            try:
                with patch.object(case.shared, 'publish', side_effect=replace), patch.object(case.os, 'write') as output:
                    case.publish_with_readout(summary, None, root, original, state)
                self.assertEqual(output.call_args.args[1], b'DEVIN_CLOSED_READOUT={"schema_version":1,"reason":"readout-refused"}\n')
                self.assertEqual((root / 'foreign').read_bytes(), b'retained')
            finally:
                import shutil
                shutil.rmtree(root.with_name(root.name + '-original'))

class FixedDefinitionCacheContracts(unittest.TestCase):
    def fixture(self, root):
        ctx, effects, user = ProspectiveBootstrapContracts.fixture(self, root)
        ProspectiveBootstrapContracts.create_aux(self, ctx)
        cache = ctx.root / case.DEFINITIONS_CACHE
        body = (b"PRIVATE_CACHE_BYTES" * (case.DEFINITIONS_BYTES // 19 + 2))[:case.DEFINITIONS_BYTES]
        cache.write_bytes(body); cache.chmod(0o600)
        # Authored inert bytes exercise the real reader; never a native cache fixture.
        fixed = dict(case.BOOTSTRAP_FIXED)
        fixed[case.DEFINITIONS_CACHE] = (len(body), hashlib.sha256(body).hexdigest())
        return ctx, effects, user, cache, body, fixed

    def test_separate_fixed_allowance_reads_full_short_chunks_and_only_records_hash(self):
        self.assertEqual(case.BOOTSTRAP_FIXED[case.DEFINITIONS_CACHE],
                         (674340, "8626d6fd4ce3381b2eb32b6c60b7ad752d88c73d8a4fb848e3576bff113393f1"))
        with tempfile.TemporaryDirectory() as root:
            ctx, effects, user, cache, body, fixed = self.fixture(root)
            baseline = copy.deepcopy(effects.baseline)
            real_read, requests = case.os.read, []
            def short_read(fd, count):
                if os.fstat(fd).st_ino == cache.stat().st_ino:
                    self.assertLessEqual(count, case.DEFINITIONS_BYTES + 1)
                    requests.append(count)
                    return real_read(fd, min(count, 32768))
                return real_read(fd, count)
            with patch.object(case, "BOOTSTRAP_FIXED", fixed), patch.object(case.os, "read", side_effect=short_read):
                effects.verify(case.metadata_inventory(ctx.root, ctx.budget))
                first = copy.deepcopy(effects.definitions_identity)
                effects.verify(case.metadata_inventory(ctx.root, ctx.budget))
            self.assertGreater(len(requests), 2)
            self.assertEqual(effects.definitions_identity, first)
            self.assertEqual(effects.baseline, baseline)
            self.assertEqual(user.read_bytes(), b'{"read_config_from":{"cloud":false}}\n')
            self.assertEqual(effects.proofs[-1][case.DEFINITIONS_CACHE],
                             {"bytes": 674340, "sha256": hashlib.sha256(body).hexdigest()})
            bodies = case.shared.evidence({"commands": [], "proofs": effects.proofs}, {}, [])
            self.assertNotIn(b"PRIVATE_CACHE_BYTES", bodies["research.json"])

    def test_real_pin_size_and_mode_refuse_without_generic_larger_file_admission(self):
        for change in ("hash", "size", "mode", "link", "other-name", "other-file"):
            with self.subTest(change=change), tempfile.TemporaryDirectory() as root:
                ctx, effects, _, cache, body, fixed = self.fixture(root)
                if change == "size": cache.write_bytes(body + b"x")
                elif change == "mode": cache.chmod(0o644)
                elif change == "link": os.link(cache, cache.with_name("hardlinked.bin"))
                elif change == "other-name": cache.rename(cache.with_name("unleash_definitions.unreviewed.bin"))
                elif change == "other-file":
                    (ctx.root / case.MAINTENANCE).write_bytes(b"x" * 16385)
                if change == "hash":
                    with self.assertRaisesRegex(ValueError, "fixed bytes"):
                        effects.verify(case.metadata_inventory(ctx.root, ctx.budget))
                else:
                    with patch.object(case, "BOOTSTRAP_FIXED", fixed), self.assertRaises(ValueError):
                        effects.verify(case.metadata_inventory(ctx.root, ctx.budget))

    def test_first_cache_and_parent_identities_never_refresh_or_read_replacements(self):
        for change in ("leaf", "parent", "removed", "mtime"):
            with self.subTest(change=change), tempfile.TemporaryDirectory() as root:
                ctx, effects, _, cache, body, fixed = self.fixture(root)
                os.utime(cache, ns=(1_000_000_000, 1_000_000_000))
                with patch.object(case, "BOOTSTRAP_FIXED", fixed):
                    effects.verify(case.metadata_inventory(ctx.root, ctx.budget))
                    original = copy.deepcopy(effects.definitions_identity)
                    if change == "leaf":
                        cache.rename(cache.with_name("original.bin")); cache.write_bytes(body); cache.chmod(0o600)
                    elif change == "parent":
                        cache.parent.rename(cache.parent.with_name("original-cli")); cache.parent.mkdir(mode=0o755)
                        cache.write_bytes(body); cache.chmod(0o600)
                        (cache.parent / "foreign").write_bytes(b"retained")
                    elif change == "removed": cache.unlink()
                    else: os.utime(cache, ns=(2_000_000_000, 2_000_000_000))
                    with patch.object(case.os, "read", side_effect=AssertionError("replacement bytes must not be read")):
                        with self.assertRaisesRegex(ValueError, "original identity"):
                            effects.verify(case.metadata_inventory(ctx.root, ctx.budget))
                    self.assertEqual(effects.definitions_identity, original)
                    if change == "parent": self.assertEqual((cache.parent / "foreign").read_bytes(), b"retained")

    def test_preexisting_cache_is_not_seeded_into_original_baseline(self):
        with tempfile.TemporaryDirectory() as root:
            ctx, _, _, _, _, _ = self.fixture(root)
            with self.assertRaisesRegex(ValueError, "preexists original baseline"):
                case.BootstrapEffects(ctx, case.metadata_inventory(ctx.root, ctx.budget))


if __name__ == "__main__": unittest.main()
