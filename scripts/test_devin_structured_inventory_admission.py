"""Inert contracts: no native client, network acquisition or hosted spoofing."""
import copy
import hashlib
import json
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


if __name__ == "__main__": unittest.main()
