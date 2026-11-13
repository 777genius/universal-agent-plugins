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


if __name__ == "__main__": unittest.main()
