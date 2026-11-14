"""Inert admission contracts only: no vendor imports, execution or hosted spoofing."""
import json
import os
from pathlib import Path
import runpy
import tempfile
import unittest
from unittest import mock
from types import SimpleNamespace

CASE = runpy.run_path(str(Path(__file__).parent / "e2e/hermes-profile-admission.py"), run_name="hermes_profile_contract")


class ProfileAdmissionContracts(unittest.TestCase):
    def test_real_child_user_bootstrap_is_admitted_only_as_final_user_control(self):
        import sys
        # Exact public DEFAULT_SOUL_MD at the pinned vendor revision; no vendor is imported.
        soul = ("You are Hermes Agent, built by Nous Research. Be direct: match the length of your reply to the weight of "
                "the ask — a one-line question gets a one-line answer, and finished work gets a short report of what "
                "changed, what's verified, and what's left, never a replay of the process. No filler (\"Great question,\" "
                "\"I'd be happy to\"), no restating the request back, no re-summarizing what you already said, no narrating "
                "tool calls the user can see. Plain claims over adjectives; when unsure, say so plainly. Agree because it's "
                "right, not because the user said it. Depth is earned — give it when the user asks for detail, teaches, or "
                "the stakes demand it, not by default.")
        child = """
import os, shutil, sys
from pathlib import Path
home, sibling, effect, soul = Path(sys.argv[1]), Path(sys.argv[2]), sys.argv[3], sys.argv[4]
for name in ('', 'sessions', 'skills'):
    (home / name).chmod(0o700)
for name in ('cron', 'logs', 'logs/curator', 'memories', 'pairing', 'hooks', 'image_cache', 'audio_cache', 'backups', 'backups/config'):
    path = home / name
    path.mkdir(); path.chmod(0o755 if name.startswith('backups') else 0o700)
for name, body, mode in (('SOUL.md', soul.encode('utf-8'), 0o600), ('logs/agent.log', b'', 0o644), ('logs/errors.log', b'', 0o644)):
    path = home / name; path.write_bytes(body); path.chmod(mode)
backup = home / 'backups/config/config.yaml.good.20261113-175542'
shutil.copy2(home / 'config.yaml', backup)
if effect == 'unknown': (home / 'unexpected').write_bytes(b'foreign')
if effect == 'backup': backup.write_bytes(b'x' * 279)
if effect == 'original':
    path = home / 'auth.json'; body = path.read_bytes(); path.rename(home / 'held-auth'); path.write_bytes(body); path.chmod(0o600)
if effect == 'directory':
    path = home / 'sessions'; old = home / 'held-sessions'; path.rename(old); shutil.copytree(old, path); shutil.rmtree(old)
if effect == 'log': (home / 'logs/agent.log').write_bytes(b'not an admitted empty log')
if effect == 'sibling': sibling.write_bytes(b'changed')
if effect == 'alias': backup.unlink(); os.link(home / 'config.yaml', backup)
"""
        for effect in ("valid", "unknown", "backup", "original", "directory", "log", "sibling", "alias"):
            with self.subTest(effect=effect), tempfile.TemporaryDirectory() as temporary:
                parent = Path(temporary).resolve(); outer = parent / "outer"; outer.mkdir()
                home = outer / "user/.hermes"
                for name in ("sessions", "skills/foreign"):
                    (home / name).mkdir(parents=True)
                for name in ("", "sessions", "skills"):
                    (home / name).chmod(0o755)
                nonce = "c" * 64
                config = (f"model: USER_MODEL_{nonce}\nproviders:\n  custom:\n    base_url: http://127.0.0.1:9/USER_PROVIDER_{nonce}\n"
                          "mcp_servers:\n  foreign:\n    command: echo\n    args: [retained]\n")
                for name, body in {"config.yaml": config, "auth.json": "retained auth marker", ".env": "# inert\n",
                                   "sessions/history": "retained history", "skills/foreign/SKILL.md": "retained Skill"}.items():
                    (home / name).write_bytes(body.encode()); (home / name).chmod(0o600)
                for role in ("a", "b"):
                    root = outer / ("project-" + role) / ".uap/hermes-runtime"
                    root.mkdir(parents=True); (root / "config.yaml").write_bytes(b"independent")
                baseline = CASE["inventory"](outer)
                before = baseline
                if effect == "valid":
                    selected = "project-a/.uap/hermes-runtime"
                    CASE["bounded"]([sys.executable, "-I", "-c", "from pathlib import Path; import sys; Path(sys.argv[1]).write_bytes(b'owned bootstrap')", outer / selected / "cache"],
                                    outer, dict(os.environ), parent / "inert-project", timeout=5)
                    before = CASE["inventory"](outer)
                    CASE["profile_effect_boundary"]("config-path-a", "a", baseline, baseline, before, "user/.hermes", selected)
                    with self.assertRaisesRegex(RuntimeError, "outside selected runtime"):
                        CASE["profile_effect_boundary"]("config-path-b", "b", baseline, baseline, before, "user/.hermes", "project-b/.uap/hermes-runtime")
                CASE["bounded"]([sys.executable, "-I", "-c", child, home, root / "config.yaml", effect, soul],
                                outer, dict(os.environ), parent / "inert-user-control", timeout=5)
                if effect == "alias":
                    with self.assertRaisesRegex(RuntimeError, "inventory member refused"):
                        CASE["inventory"](outer)
                    continue
                after = CASE["inventory"](outer)
                boundary = CASE["profile_effect_boundary"]
                args = (baseline, before, after, "user/.hermes", "user/.hermes")
                with self.assertRaisesRegex(RuntimeError, "Project command changed original User"):
                    boundary("config-path-a", "a", *args)
                if effect == "valid":
                    boundary("list-user", "user", *args)
                    with self.assertRaisesRegex(RuntimeError, "original User changed before command"):
                        boundary("list-user", "user", baseline, after, after, "user/.hermes", "user/.hermes")
                    with self.assertRaisesRegex(RuntimeError, "User control changed outside original User"):
                        boundary("unknown-user-action", "user", *args)
                else:
                    with self.assertRaises(RuntimeError):
                        boundary("list-user", "user", *args)

    def test_windows_user_control_lazy_log_pair_joins_original_delta_and_stays_scoped(self):
        import copy
        import hashlib
        # Metadata-only projection of original artifact 14089152690, run
        # 43526178468, Windows job 132234865220, UAP 999be5b1206d.
        # ZIP SHA256 d5ea24f4616fa8b55aecbae5402d9e89a9b1efecd05fd3baca714d89db826860.
        # No fixture paths, permission values, identities or hashes are substituted.
        capture = json.loads(r'''
        {
          "before": {
            "user": {"type":"directory","mode":511,"device":13423725783011504597,"inode":281474976902339},
            "user/AppData": {"type":"directory","mode":511,"device":13423725783011504597,"inode":281474976902345},
            "user/AppData/Local": {"type":"directory","mode":511,"device":13423725783011504597,"inode":281474976902347},
            "user/AppData/Local/hermes": {"type":"directory","mode":511,"device":13423725783011504597,"inode":281474976902351},
            "user/AppData/Local/hermes/.env": {"type":"file","mode":438,"device":13423725783011504597,"inode":281474976902354,"bytes":34,"sha256":"9ec3030a4afa2e0ec4a7abdc05545d3a9061e5f448f8d2391dd6269c2b2457dc"},
            "user/AppData/Local/hermes/auth.json": {"type":"file","mode":438,"device":13423725783011504597,"inode":281474976902353,"bytes":129,"sha256":"6efd71b625c084746c17c16f86223d879bcb2ab337ff3107a3e0e010761260e6"},
            "user/AppData/Local/hermes/config.yaml": {"type":"file","mode":438,"device":13423725783011504597,"inode":281474976902352,"bytes":279,"sha256":"6a9e3de24ef0cb976fbc009b5693d50f8c1e029efa53ddb1546c15148d449c3e"},
            "user/AppData/Local/hermes/sessions": {"type":"directory","mode":511,"device":13423725783011504597,"inode":281474976902355},
            "user/AppData/Local/hermes/sessions/retained-history.txt": {"type":"file","mode":438,"device":13423725783011504597,"inode":281474976902356,"bytes":77,"sha256":"01b8597f031ab252a72baee10e4d7425f0089c3bd817fa9c3112c0bae0ed2980"},
            "user/AppData/Local/hermes/skills": {"type":"directory","mode":511,"device":13423725783011504597,"inode":281474976902357},
            "user/AppData/Local/hermes/skills/foreign": {"type":"directory","mode":511,"device":13423725783011504597,"inode":281474976902358},
            "user/AppData/Local/hermes/skills/foreign/SKILL.md": {"type":"file","mode":438,"device":13423725783011504597,"inode":281474976902359,"bytes":141,"sha256":"bc80d52f2dfcc941591de8f30dc0511735bb782e24ff67aae11d4a07ff810a64"},
            "user/AppData/Roaming": {"type":"directory","mode":511,"device":13423725783011504597,"inode":281474976902346},
            "user/cache": {"type":"directory","mode":511,"device":13423725783011504597,"inode":281474976902341},
            "user/config": {"type":"directory","mode":511,"device":13423725783011504597,"inode":281474976902340},
            "user/data": {"type":"directory","mode":511,"device":13423725783011504597,"inode":281474976902343},
            "user/state": {"type":"directory","mode":511,"device":13423725783011504597,"inode":281474976902342},
            "user/tmp": {"type":"directory","mode":511,"device":13423725783011504597,"inode":281474976902344}
          },
          "delta": {
            "user/AppData/Local/hermes/SOUL.md": {"before":null,"after":{"type":"file","mode":438,"device":13423725783011504597,"inode":281474976902528,"bytes":667,"sha256":"36c1f5a2e92cd1d018311eaf4c8f1e8886672eae78212e033c681d0e3d5d506f"}},
            "user/AppData/Local/hermes/audio_cache": {"before":null,"after":{"type":"directory","mode":511,"device":13423725783011504597,"inode":281474976902527}},
            "user/AppData/Local/hermes/backups": {"before":null,"after":{"type":"directory","mode":511,"device":13423725783011504597,"inode":281474976902529}},
            "user/AppData/Local/hermes/backups/config": {"before":null,"after":{"type":"directory","mode":511,"device":13423725783011504597,"inode":281474976902530}},
            "user/AppData/Local/hermes/backups/config/config.yaml.good.20261114-003350": {"before":null,"after":{"type":"file","mode":438,"device":13423725783011504597,"inode":281474976902531,"bytes":279,"sha256":"6a9e3de24ef0cb976fbc009b5693d50f8c1e029efa53ddb1546c15148d449c3e"}},
            "user/AppData/Local/hermes/cron": {"before":null,"after":{"type":"directory","mode":511,"device":13423725783011504597,"inode":281474976902520}},
            "user/AppData/Local/hermes/hooks": {"before":null,"after":{"type":"directory","mode":511,"device":13423725783011504597,"inode":281474976902525}},
            "user/AppData/Local/hermes/image_cache": {"before":null,"after":{"type":"directory","mode":511,"device":13423725783011504597,"inode":281474976902526}},
            "user/AppData/Local/hermes/logs": {"before":null,"after":{"type":"directory","mode":511,"device":13423725783011504597,"inode":281474976902521}},
            "user/AppData/Local/hermes/logs/curator": {"before":null,"after":{"type":"directory","mode":511,"device":13423725783011504597,"inode":281474976902522}},
            "user/AppData/Local/hermes/memories": {"before":null,"after":{"type":"directory","mode":511,"device":13423725783011504597,"inode":281474976902523}},
            "user/AppData/Local/hermes/pairing": {"before":null,"after":{"type":"directory","mode":511,"device":13423725783011504597,"inode":281474976902524}}
          }
        }
        ''')
        canonical = json.dumps(capture, sort_keys=True, separators=(",", ":")).encode()
        self.assertEqual(hashlib.sha256(canonical).hexdigest(), "a8a525f0ef4b05fcf6d37a7f475850a6bf367c4fb649cf32118579e96c088a89")
        before = capture["before"]
        after = copy.deepcopy(before)
        for name, change in capture["delta"].items():
            self.assertIsNone(change["before"])
            self.assertNotIn(name, before)
            after[name] = change["after"]
        self.assertEqual((len(before), len(after), len(capture["delta"])), (18, 30, 12))
        prefix = "user/AppData/Local/hermes"
        logs = [prefix + "/logs/agent.log", prefix + "/logs/errors.log"]
        backup = next(name for name in after if "/config.yaml.good." in name)
        boundary = CASE["profile_effect_boundary"]
        authority = boundary.__globals__
        with mock.patch.dict(authority, {"os": SimpleNamespace(name="nt")}):
            # The published oracle rejects these exact original bytes before
            # the Windows lazy-open correction. Both native logs are absent.
            boundary("list-user", "user", before, before, after, prefix, prefix)
            eager = copy.deepcopy(after)
            for index, name in enumerate(logs):
                eager[name] = {"type": "file", "mode": 0o666, "device": before[prefix]["device"],
                               "inode": before[prefix]["inode"] + 1000 + index, "bytes": 0,
                               "sha256": hashlib.sha256(b"").hexdigest()}
            boundary("list-user", "user", before, before, eager, prefix, prefix)
            refused = {}
            for index, name in enumerate(logs):
                partial = copy.deepcopy(eager); partial.pop(name)
                refused["only-one-log-" + str(index)] = partial
                nonempty = copy.deepcopy(eager)
                nonempty[name].update(bytes=1, sha256=hashlib.sha256(b"x").hexdigest())
                refused["nonempty-log-" + str(index)] = nonempty
                wrong_mode = copy.deepcopy(eager); wrong_mode[name]["mode"] = 0o644
                refused["wrong-log-mode-" + str(index)] = wrong_mode
            foreign = copy.deepcopy(after); foreign[prefix + "/logs/foreign.log"] = eager[logs[0]]
            refused["foreign-log"] = foreign
            for name in (prefix + "/SOUL.md", prefix + "/logs/curator", backup):
                missing = copy.deepcopy(after); missing.pop(name)
                refused["missing-" + name] = missing
            changed = copy.deepcopy(after); changed[prefix + "/auth.json"]["inode"] += 1000
            refused["original-object-replaced"] = changed
            changed = copy.deepcopy(after); changed[prefix + "/SOUL.md"]["sha256"] = "0" * 64
            refused["soul-content"] = changed
            changed = copy.deepcopy(after); changed[backup]["sha256"] = "0" * 64
            refused["backup-content"] = changed
            changed = copy.deepcopy(after); changed[backup].update(before[prefix + "/config.yaml"])
            refused["backup-alias"] = changed
            changed = copy.deepcopy(after); changed["user/foreign"] = eager[logs[0]]
            refused["outside-native-user"] = changed
            for label, snapshot in refused.items():
                with self.subTest(refusal=label), self.assertRaises(RuntimeError):
                    boundary("list-user", "user", before, before, snapshot, prefix, prefix)
            with self.assertRaisesRegex(RuntimeError, "Project command changed original User"):
                boundary("config-path-a", "a", before, before, after, prefix, "project-a/.uap/hermes-runtime")
            with self.assertRaisesRegex(RuntimeError, "source User control changed"):
                boundary("list-user", "user", before, before, after, "user", prefix, source_capture=True)
            with self.assertRaisesRegex(RuntimeError, "outside original User"):
                boundary("unknown-user-command", "user", before, before, after, prefix, prefix)
        # Explicit mode-only projection for the existing POSIX contract. It is
        # separate from the unmodified original Windows metadata above.
        posix_before, posix_after = copy.deepcopy(before), copy.deepcopy(after)
        secured = {prefix + suffix for suffix in ("", "/sessions", "/skills")}
        for name, record in posix_before.items():
            record["mode"] = 0o755 if record["type"] == "directory" else 0o600
        for name, record in posix_after.items():
            if record["type"] == "directory":
                record["mode"] = 0o700 if name in secured or (name not in before and not name.startswith(prefix + "/backups")) else 0o755
            else:
                record["mode"] = 0o600
        with mock.patch.dict(authority, {"os": SimpleNamespace(name="posix")}):
            with self.assertRaisesRegex(RuntimeError, "unknown/missing bootstrap member"):
                boundary("list-user", "user", posix_before, posix_before, posix_after, prefix, prefix)
            for name in logs:
                posix_after[name] = {**eager[name], "mode": 0o644}
            boundary("list-user", "user", posix_before, posix_before, posix_after, prefix, prefix)

    def test_native_config_readouts_use_exact_retained_real_child_streams(self):
        import sys
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            selected = {"root": str(root / "project-a/.uap/hermes-runtime"), "server": "owned", "entry": {"url": "http://127.0.0.1:19/mcp"}}
            path = str(Path(selected["root"]) / "config.yaml").encode()
            body = json.dumps({"owned": selected["entry"]}).encode()
            cases = [("path", path + b"\n", True), ("path", path + b"\r\n", True), ("mcp", body + b"\n", True),
                     ("path", path.replace(b"project-a", b"project-b") + b"\n", False),
                     ("path", path + b"\nextra\n", False), ("path", path, False), ("path", b" " + path + b"\n", False),
                     ("mcp", b'{"owned":{},"owned":' + json.dumps(selected["entry"]).encode() + b'}', False),
                     ("mcp", b'{"owned":{"url":null,"url":"http://127.0.0.1:19/mcp"}}', False),
                     ("mcp", b'{}', False), ("mcp", body[:-1] + b',"extra":{}}', False),
                     ("mcp", b'{"owned":{"url":true}}', False), ("mcp", b'{"owned":{"url":"http://127.0.0.1:20/mcp"}}', False),
                     ("mcp", body + b'{}', False), ("mcp", b'\xff', False), ("mcp", b'[]', False)]
            for index, (kind, raw, valid) in enumerate(cases):
                with self.subTest(kind=kind, index=index):
                    evidence = root / str(index); evidence.mkdir()
                    output = evidence / ("config-" + kind + "-a")
                    captured = CASE["bounded"]([sys.executable, "-I", "-c", "import sys; sys.stdout.buffer.write(bytes.fromhex(sys.argv[1]))", raw.hex()],
                                               root, dict(os.environ), output, timeout=5)
                    self.assertEqual(captured, raw)
                    CASE["admit_evidence"](evidence)
                    if valid:
                        CASE["config_readout"](kind, captured, selected)
                    else:
                        with self.assertRaises(RuntimeError):
                            CASE["config_readout"](kind, captured, selected)
            CASE["protocol_join"]({"a": [], "b": []}, {"a": [], "b": []}, None, 0)
            with self.assertRaisesRegex(RuntimeError, "probe contacted"):
                CASE["protocol_join"]({"a": [], "b": []}, {"a": ["initialize"], "b": []}, None, 0)

    def test_observer_runtime_preserves_official_bootstrap_and_arguments(self):
        python = str(Path("/payload/python"))
        official = [python, "-I", "-c", "opaque official bootstrap", "--child"]
        make = mock.Mock(return_value=official)
        command = CASE["observer_runtime_command"](make, Path("/payload/source"),
                    Path("/payload/python"), code="observer", args=["--child"])
        make.assert_called_once_with(Path("/payload/source"), python=Path("/payload/python"),
                                     code="observer", args=["--child"])
        self.assertEqual(command, [official[0], "-I", "-B", *official[2:]])
        self.assertEqual(official, [python, "-I", "-c", "opaque official bootstrap", "--child"])
        invalid = (tuple(official), official[1:], ["/foreign/python", *official[1:]],
                   [official[0], "-S", *official[2:]], [*official, "extra"],
                   [*official[:3], None, "--child"], [*official[:4], "changed"])
        for argv in invalid:
            with self.subTest(argv=argv), self.assertRaisesRegex(RuntimeError, "command shape changed"):
                CASE["observer_runtime_command"](mock.Mock(return_value=argv), Path("/payload/source"),
                    "/payload/python", code="observer", args=["--child"])

    def test_observer_bytecode_flag_prevents_real_isolated_python_cache_writes(self):
        import subprocess
        import sys
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            for protected in (False, True):
                directory = root / str(protected)
                directory.mkdir()
                module = directory / "uap_inert_observer_module.py"
                module.write_bytes(b"value = 17\n")
                code = (f"import sys; sys.path.insert(0, {str(directory)!r}); "
                        "import uap_inert_observer_module as probe; "
                        "print(sys.dont_write_bytecode, probe.value)")
                def make(_root, *, python, code, args):
                    return [str(python), "-I", "-c", code, *args]
                command = (CASE["observer_runtime_command"](make, directory, sys.executable, code=code)
                           if protected else make(directory, python=sys.executable, code=code, args=()))
                completed = subprocess.run(command, env={**os.environ, "PYTHONDONTWRITEBYTECODE": "1"},
                                           capture_output=True, check=True, timeout=10)
                self.assertEqual(completed.stdout.strip(), f"{protected} 17".encode())
                self.assertEqual(module.read_bytes(), b"value = 17\n")
                self.assertEqual(any(directory.rglob("*.pyc")), not protected)

    def test_census_is_content_free_disjoint_and_does_not_follow_links(self):
        import time
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            for name in ("user/selected-profile/cache", "user/selected-profile/tools", "user/selected-profile/installs", "project-a", "project-b"):
                (root / name).mkdir(parents=True, exist_ok=True)
                (root / name / "item").write_bytes(b"private")
            os.link(root / "project-a/item", root / "project-a/second")
            if os.name != "nt": (root / "project-b/link").symlink_to(root / "project-a", target_is_directory=True)
            expected = CASE["directory_identity"](root)
            with mock.patch.object(os, "readlink", side_effect=AssertionError("link target read")), mock.patch.object(os, "fdopen", side_effect=AssertionError("content read")), mock.patch.dict(CASE["metadata_census"].__globals__, {"ordinary_bytes": mock.Mock(side_effect=AssertionError("content read"))}):
                value = CASE["metadata_census"](root, expected, time.monotonic() + 5)
            self.assertTrue(value["complete"])
            CASE["validate_census_member"]("bootstrap-census.json", json.dumps(value).encode())
            self.assertEqual(value["logical_regular_bytes"], 42)
            self.assertEqual(value["groups"]["projectA"]["logical_regular_bytes"], 14)
            expected_links = max(os.lstat(root / name).st_nlink for name in ("project-a", "project-a/item", "project-a/second"))
            self.assertEqual(value["groups"]["projectA"]["max_links"], expected_links)
            self.assertEqual(value["groups"]["projectA"]["regular_hardlinked_entries"], 2)
            self.assertEqual(value["entries"], sum(row["entries"] for row in value["groups"].values()))
            self.assertTrue(all(row["entries"] == sum(row["kinds"].values()) for row in value["groups"].values()))
            self.assertNotIn("private", json.dumps(value))

    def test_census_root_replacement_budget_and_saturation_are_incomplete(self):
        import stat
        import time
        with tempfile.TemporaryDirectory() as temporary:
            parent = Path(temporary).resolve()
            root = parent / "root"; root.mkdir()
            original = CASE["directory_identity"](root)
            (root / "file").write_bytes(b"unused")
            partial = CASE["metadata_census"](root, original, time.monotonic() - 1)
            self.assertEqual((partial["status"], partial["reason"], partial["entries"]), ("partial", "time-limit", 0))
            with mock.patch.object(CASE["time"], "monotonic", side_effect=[0, 10]):
                partial = CASE["metadata_census"](root, original, 5)
            self.assertEqual((partial["status"], partial["entries"]), ("partial", 1))
            actual_stat, actual_lstat = os.stat, Path.lstat
            def huge(path, *args, **kwargs):
                # POSIX traverses by dir_fd; Windows' path-based inventory calls Path.lstat.
                if (path == "file" and kwargs.get("dir_fd") is not None) or (
                    os.name == "nt" and isinstance(path, Path) and path == root / "file"
                ):
                    return SimpleNamespace(st_mode=stat.S_IFREG | 0o600, st_nlink=1 << 54, st_size=1 << 54)
                return actual_lstat(path, *args, **kwargs) if os.name == "nt" else actual_stat(path, *args, **kwargs)
            patch = (mock.patch.object(Path, "lstat", autospec=True, side_effect=huge) if os.name == "nt"
                     else mock.patch.object(os, "stat", side_effect=huge))
            with patch:
                value = CASE["metadata_census"](root, original, time.monotonic() + 5)
            self.assertTrue(value["overflow"]); self.assertFalse(value["complete"])
            self.assertEqual(value["logical_regular_bytes"], (1 << 53) - 1)
            root.rename(parent / "original"); root.mkdir()
            with mock.patch.object(os, "scandir", side_effect=AssertionError("foreign root enumerated")):
                refused = CASE["metadata_census"](root, original, time.monotonic() + 5)
            self.assertEqual(refused["status"], "refused")

    def test_census_member_depth_and_exhaustion_are_distinct(self):
        import time
        self.assertEqual(CASE["CENSUS_MEMBERS"], 131072)
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            for index in range(8): (root / str(index)).touch()
            expected = CASE["directory_identity"](root)
            with mock.patch.dict(CASE["metadata_census"].__globals__, {"CENSUS_MEMBERS": 8}):
                value = CASE["metadata_census"](root, expected, time.monotonic() + 5)
                self.assertEqual((value["reason"], value["entries"], value["complete"]), ("member-limit", 8, False))
                CASE["validate_census_member"]("bootstrap-census.json", json.dumps(value).encode())
                value["reason"] = "time-limit"
                CASE["validate_census_member"]("bootstrap-census.json", json.dumps(value).encode())
                value["reason"], value["entries"] = "member-limit", 7
                group = value["groups"]["outer-other"]
                group["entries"] -= 1; group["kinds"]["regular"] -= 1
                with self.assertRaisesRegex(RuntimeError, "census member limit join"):
                    CASE["validate_census_member"]("bootstrap-census.json", json.dumps(value).encode())
            complete = CASE["metadata_census"](root, expected, time.monotonic() + 5)
            self.assertEqual((complete["complete"], complete["entries"]), (True, 9))
            with mock.patch.object(CASE["time"], "monotonic", side_effect=[0] * 9 + [10]):
                expired = CASE["metadata_census"](root, expected, 5)
            self.assertEqual((expired["reason"], expired["entries"], expired["complete"]), ("time-limit", 9, False))
            nested = root
            for _ in range(33): nested = nested / "d"; nested.mkdir()
            deep = CASE["metadata_census"](root, expected, time.monotonic() + 5)
            self.assertEqual((deep["reason"], deep["complete"]), ("depth-limit", False))
            CASE["validate_census_member"]("bootstrap-census.json", json.dumps(deep).encode())

    def test_census_request_requires_settlement_and_closed_identity_before_walk(self):
        import time
        with tempfile.TemporaryDirectory() as temporary:
            parent = Path(temporary).resolve()
            root, evidence = parent / "root", parent / "evidence"
            root.mkdir(); evidence.mkdir()
            expected = CASE["directory_identity"](root)
            with mock.patch.dict(CASE["maybe_census"].__globals__, {"metadata_census": mock.Mock(side_effect=AssertionError("fallback walk"))}):
                CASE["maybe_census"](evidence, root, expected, "c" * 64, True, time.monotonic() + 5, CASE["directory_identity"](evidence))
                self.assertEqual(list(evidence.iterdir()), [])
                CASE["publish_census_request"](evidence, root, "c" * 64, CASE["directory_identity"](evidence))
                CASE["maybe_census"](evidence, root, expected, "c" * 64, False, time.monotonic() + 5, CASE["directory_identity"](evidence))
                value = json.loads((evidence / "bootstrap-census.json").read_text())
                self.assertFalse(value["complete"])
            self.assertTrue({"bootstrap-census.json", "bootstrap-census-request.json"} <= CASE["MEMBERS"])
            self.assertLess((evidence / "bootstrap-census.json").stat().st_size, 16 << 10)
            (evidence / "bootstrap-census.json").unlink()
            with mock.patch.dict(CASE["publish_census_request"].__globals__, {"write_census_member": mock.Mock(side_effect=OSError("cannot publish"))}):
                CASE["publish_census_request"](evidence, root, "c" * 64, CASE["directory_identity"](evidence))
            (evidence / "bootstrap-census-request.json").write_text('{"schema_version":1,"schema_version":1}')
            with mock.patch.dict(CASE["maybe_census"].__globals__, {"metadata_census": mock.Mock(side_effect=AssertionError("invalid request walked"))}):
                CASE["maybe_census"](evidence, root, expected, "c" * 64, True, time.monotonic() + 5, CASE["directory_identity"](evidence))
            self.assertEqual(json.loads((evidence / "bootstrap-census.json").read_text())["reason"], "request")
            with self.assertRaises(RuntimeError): CASE["stage_evidence"](evidence)
            self.assertFalse((evidence / "upload").exists())
            (evidence / "bootstrap-census-request.json").unlink()
            CASE["publish_census_request"](evidence, root, "c" * 64, CASE["directory_identity"](evidence))
            CASE["stage_evidence"](evidence)
            self.assertEqual({path.name for path in (evidence / "upload").iterdir()}, {"bootstrap-census.json", "bootstrap-census-request.json"})

    def test_census_evidence_replacement_never_writes_foreign_parent(self):
        import time
        with tempfile.TemporaryDirectory() as temporary:
            parent = Path(temporary).resolve()
            root, evidence = parent / "root", parent / "evidence"
            root.mkdir(); evidence.mkdir()
            expected = CASE["directory_identity"](evidence)
            evidence.rename(parent / "original-evidence"); evidence.mkdir()
            sentinel = evidence / "foreign"
            sentinel.write_bytes(b"retained")
            with self.assertRaisesRegex(RuntimeError, "original census evidence changed"):
                CASE["maybe_census"](evidence, root, CASE["directory_identity"](root), "c" * 64, True, time.monotonic() + 5, expected)
            self.assertEqual({path.name for path in evidence.iterdir()}, {"foreign"})
            self.assertEqual(sentinel.read_bytes(), b"retained")

    def test_census_publication_refuses_oversize_unknown_and_boolean_documents(self):
        for name in ("bootstrap-census-request.json", "bootstrap-census.json"):
            for body in (b" " * ((16 << 10) + 1), b'{"private":"must not publish"}', b'{"schema_version":true}'):
                with self.subTest(name=name, body=body[:30]), tempfile.TemporaryDirectory() as temporary:
                    evidence = Path(temporary).resolve()
                    (evidence / name).write_bytes(body)
                    with self.assertRaises((RuntimeError, ValueError)):
                        CASE["stage_evidence"](evidence)
                    self.assertFalse((evidence / "upload").exists())

    def test_inventory_refusal_identifies_metadata_without_reading_member(self):
        for mutation in ("oversize", "hardlink", "symlink"):
            with self.subTest(mutation=mutation), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary).resolve()
                member = root / "runtime-member"
                member.write_bytes(b"private contents must never appear in a refusal")
                if mutation == "oversize":
                    with member.open("r+b") as stream:
                        stream.truncate(CASE["LIMIT"] + 1)
                elif mutation == "hardlink":
                    os.link(member, root / "alias")
                else:
                    member.unlink()
                    member.symlink_to(root / "absent")
                with mock.patch.dict(CASE["inventory"].__globals__, {"ordinary_bytes": mock.Mock()}) as patched:
                    with self.assertRaisesRegex(RuntimeError, "inventory member refused:") as caught:
                        CASE["inventory"](root)
                    patched["ordinary_bytes"].assert_not_called()
                message = str(caught.exception)
                details = json.loads(message.split(": ", 1)[1])
                self.assertIn(details["member"], ("runtime-member", "alias"))
                self.assertEqual(set(details), {"member", "member_sha256", "mode", "links", "bytes"})
                self.assertLess(len(message), 800)
                self.assertNotIn(str(root), message)
                self.assertNotIn("private contents", message)

    def test_inventory_refusal_bounds_and_escapes_relative_member(self):
        info = SimpleNamespace(st_mode=0o100600, st_nlink=2, st_size=17)
        for relative in ("line\nname", "x" * 513):
            with self.assertRaises(RuntimeError) as caught:
                CASE["inventory_member"](relative, info)
            message = str(caught.exception)
            self.assertNotIn("\n", message)
            self.assertLess(len(message), 800)
            value = json.loads(message.split(": ", 1)[1])
            self.assertEqual(value["member"], None if len(relative) > 512 else relative)
            self.assertEqual(len(value["member_sha256"]), 64)

    def test_inventory_detects_bytes_modes_directories_and_aliases(self):
        for change in ("bytes", "mode", "directory", "symlink", "hardlink"):
            with self.subTest(change=change), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary).resolve()
                owned = root / "foreign"
                owned.write_bytes(b"unchanged")
                before = CASE["inventory"](root)
                if change == "bytes":
                    owned.write_bytes(b"changed!!")
                elif change == "mode":
                    owned.chmod(0o400)
                elif change == "directory":
                    (root / "new-empty").mkdir()
                elif change == "symlink":
                    (root / "alias").symlink_to(owned)
                else:
                    os.link(owned, root / "alias")
                if change in ("symlink", "hardlink"):
                    with self.assertRaises(RuntimeError):
                        CASE["inventory"](root)
                else:
                    self.assertTrue(CASE["changes"](before, CASE["inventory"](root)))

    def test_inventory_refuses_directory_swap_before_or_during_enumeration(self):
        for boundary in ("before-open", "during-scan"):
            with self.subTest(boundary=boundary), tempfile.TemporaryDirectory() as temporary:
                parent = Path(temporary).resolve()
                root, foreign = parent / "root", parent / "foreign"
                root.mkdir(); foreign.mkdir()
                child = root / "child"
                child.mkdir()
                (foreign / "foreign-directory").mkdir()
                sentinel = foreign / "sentinel"
                sentinel.write_bytes(b"foreign contents")
                member = CASE["inventory_member"]
                scan = os.scandir
                def replace():
                    child.rename(root / "original-child")
                    child.symlink_to(foreign, target_is_directory=True)
                def checked(relative, info):
                    member(relative, info)
                    if boundary == "before-open" and relative == "child": replace()
                def scanning(operand):
                    selected = (Path(operand) == child if not isinstance(operand, int)
                                else os.fstat(operand).st_ino == child.lstat().st_ino)
                    entries = scan(operand)
                    if boundary == "during-scan" and selected:
                        try: replace()
                        except BaseException:
                            entries.close()
                            raise
                    return entries
                with mock.patch.dict(CASE["inventory"].__globals__, {"inventory_member": checked}), mock.patch.object(os, "scandir", side_effect=scanning):
                    # Windows rejects a replaced directory through its typed identity guard.
                    with self.assertRaises((RuntimeError, OSError, ValueError)):
                        CASE["inventory"](root)
                self.assertEqual(sentinel.read_bytes(), b"foreign contents")

    @unittest.skipIf(os.name == "nt", "Windows held directory denies rename instead")
    def test_posix_ordinary_parent_replacement_never_reads_foreign_same_name(self):
        with tempfile.TemporaryDirectory() as temporary:
            parent = Path(temporary).resolve()
            root, foreign = parent / "root", parent / "foreign"
            root.mkdir(); foreign.mkdir()
            child = root / "child"
            child.mkdir()
            (child / "same-name").write_bytes(b"owned")
            secret = foreign / "same-name"
            secret.write_bytes(b"foreign private contents")
            secret_id = (secret.stat().st_dev, secret.stat().st_ino)
            scan, fdopen = os.scandir, os.fdopen
            replaced, read_foreign = [], []
            def scanning(operand):
                entries = scan(operand)
                if isinstance(operand, int) and os.fstat(operand).st_ino == child.lstat().st_ino:
                    child.rename(root / "original-child")
                    foreign.rename(child)
                    replaced.append(True)
                return entries
            def guarded_open(descriptor, *args, **kwargs):
                info = os.fstat(descriptor)
                if (info.st_dev, info.st_ino) == secret_id:
                    read_foreign.append(True)
                    os.close(descriptor)
                    raise AssertionError("foreign bytes would be read")
                return fdopen(descriptor, *args, **kwargs)
            with mock.patch.object(os, "scandir", side_effect=scanning), mock.patch.object(os, "fdopen", side_effect=guarded_open):
                with self.assertRaisesRegex(RuntimeError, "inventory directory changed"):
                    CASE["inventory"](root)
            self.assertEqual(replaced, [True])
            self.assertEqual(read_foreign, [])
            self.assertEqual((child / "same-name").read_bytes(), b"foreign private contents")

    def test_initial_root_replacement_never_reads_foreign_inode(self):
        with tempfile.TemporaryDirectory() as temporary:
            parent = Path(temporary).resolve()
            root, foreign = parent / "root", parent / "foreign"
            root.mkdir(); foreign.mkdir()
            secret = foreign / "same-name"
            secret.write_bytes(b"foreign private contents")
            secret_id = (secret.stat().st_dev, secret.stat().st_ino)
            original, fdopen = CASE["directory_identity"], os.fdopen
            def swap(path):
                identity = original(path)
                root.rename(parent / "original-root")
                foreign.rename(root)
                return identity
            def guarded_open(descriptor, *args, **kwargs):
                info = os.fstat(descriptor)
                if (info.st_dev, info.st_ino) == secret_id:
                    os.close(descriptor)
                    raise AssertionError("foreign root bytes would be read")
                return fdopen(descriptor, *args, **kwargs)
            with mock.patch.dict(CASE["inventory"].__globals__, {"directory_identity": swap}), mock.patch.object(os, "fdopen", side_effect=guarded_open):
                with self.assertRaisesRegex(RuntimeError, "inventory original root changed before open"):
                    CASE["inventory"](root)
            self.assertEqual((root / "same-name").read_bytes(), b"foreign private contents")

    def test_windows_directory_handle_uses_no_reparse_and_no_delete_sharing(self):
        import ctypes
        import sys
        create, close = mock.Mock(return_value=123), mock.Mock(return_value=1)
        transfer = mock.Mock(return_value=17)
        kernel = SimpleNamespace(CreateFileW=create, CloseHandle=close)
        with mock.patch.object(CASE["artifacts"], "WINDOWS", True), mock.patch.object(ctypes, "WinDLL", return_value=kernel, create=True), mock.patch.dict(sys.modules, {"msvcrt": SimpleNamespace(open_osfhandle=transfer)}), mock.patch.object(os, "O_BINARY", 0x8000, create=True):
            self.assertEqual(CASE["open_inventory_directory"](Path("inert-directory")), 17)
            self.assertEqual(create.call_args.args, ("inert-directory", 0x80000000, 3, None, 3, 0x02200000, None))
            self.assertIs(create.restype, ctypes.c_void_p)
            transfer.assert_called_once_with(123, os.O_RDONLY | 0x8000)
            close.assert_not_called()
            transfer.side_effect = OSError("transfer failed")
            with self.assertRaises(OSError): CASE["open_inventory_directory"](Path("inert-directory"))
            close.assert_called_once_with(123)

    def test_windows_held_directory_requires_positive_non_reparse_identity(self):
        import stat
        base = dict(st_dev=4, st_ino=5, st_mode=stat.S_IFDIR | 0o755, st_nlink=1,
                    st_size=0, st_mtime_ns=100, st_birthtime_ns=90,
                    st_file_attributes=16, st_reparse_tag=0)
        with mock.patch.object(CASE["artifacts"], "WINDOWS", True):
            original = SimpleNamespace(**base)
            self.assertTrue(CASE["same_inventory_directory"](original, SimpleNamespace(**dict(base, st_mode=stat.S_IFDIR | 0o700))))
            # Verified on the real Windows runner: path lstat and held fstat
            # can disagree on ordinary-directory attribute flags.
            self.assertTrue(CASE["same_inventory_directory"](
                original, SimpleNamespace(**dict(base, st_file_attributes=32))))
            self.assertFalse(CASE["same_inventory_directory"](original, SimpleNamespace(**dict(base, st_ino=6))))
            for change in ({"st_file_attributes": 16 | 0x400}, {"st_reparse_tag": 1}, {"st_birthtime_ns": None}):
                with self.assertRaises(ValueError): CASE["same_inventory_directory"](original, SimpleNamespace(**dict(base, **change)))

    def test_windows_directory_mismatch_reports_only_field_names(self):
        import stat
        base = dict(st_dev=4, st_ino=5, st_mode=stat.S_IFDIR | 0o755,
                    st_nlink=1, st_size=0, st_mtime_ns=100, st_birthtime_ns=90,
                    st_file_attributes=16, st_reparse_tag=0)
        original = SimpleNamespace(**base)
        with mock.patch.object(CASE["artifacts"], "WINDOWS", True):
            CASE["check_inventory_directory"](
                original, SimpleNamespace(**dict(base, st_mode=stat.S_IFDIR | 0o700)),
                "inventory directory changed at open")
            CASE["check_inventory_directory"](
                original, SimpleNamespace(**dict(base, st_file_attributes=32)),
                "inventory directory changed at open")
            for field, value, label in (
                ("st_dev", 6, "device"), ("st_ino", 6, "inode"),
                ("st_nlink", 2, "links"), ("st_size", 123456789, "bytes"),
                ("st_mtime_ns", 101, "mtime"), ("st_birthtime_ns", 91, "birth"),
            ):
                with self.subTest(field=field), self.assertRaises(RuntimeError) as caught:
                    CASE["check_inventory_directory"](
                        original, SimpleNamespace(**dict(base, **{field: value})),
                        "inventory directory changed at open")
                message = str(caught.exception)
                self.assertIn("Windows metadata: " + label, message)
                self.assertNotIn("123456789", message)
                self.assertNotIn("\\", message)
            for change in ({"st_file_attributes": 16 | 0x400},
                           {"st_reparse_tag": 1}, {"st_birthtime_ns": None}):
                with self.assertRaises(ValueError):
                    CASE["check_inventory_directory"](
                        original, SimpleNamespace(**dict(base, **change)),
                        "inventory directory changed at open")
            with self.assertRaises(RuntimeError):
                CASE["check_inventory_directory"](
                    original, SimpleNamespace(**dict(base, st_mode=stat.S_IFREG | 0o755)),
                    "inventory directory changed at open")

    def test_directory_temporal_diagnostic_keeps_windows_domains_separate(self):
        import stat
        path = Path(__file__).resolve().parent / "private-directory-name"
        base = dict(st_dev=4, st_ino=5, st_mode=stat.S_IFDIR | 0o755, st_nlink=1,
                    st_uid=6, st_gid=7, st_size=0, st_mtime_ns=100, st_ctime_ns=90,
                    st_birthtime_ns=80, st_file_attributes=16, st_reparse_tag=0)
        before = SimpleNamespace(**base)
        held = SimpleNamespace(**dict(base, st_mode=stat.S_IFDIR | 0o700, st_ctime_ns=800, st_file_attributes=32))
        for change in (None, "path", "handle"):
            linked = SimpleNamespace(**dict(base, **({"st_ctime_ns": 123456789} if change == "path" else {})))
            current = SimpleNamespace(**dict(vars(held), **({"st_file_attributes": 123456789} if change == "handle" else {})))
            with self.subTest(change=change), mock.patch.object(CASE["artifacts"], "WINDOWS", True):
                with mock.patch.object(Path, "lstat", return_value=linked), mock.patch.object(os, "fstat", return_value=current):
                    with mock.patch.object(Path, "resolve", return_value=path):
                        if change is None:
                            CASE["verify_inventory_directory"](path, before, 17, held)
                            continue
                        with self.assertRaises(RuntimeError) as caught:
                            CASE["verify_inventory_directory"](path, before, 17, held)
            field = "path:st_ctime_ns" if change == "path" else "handle:st_file_attributes"
            self.assertEqual(str(caught.exception), "inventory directory changed (" + field + ")")

    def test_windows_named_directory_accepts_only_the_observed_ntfs_bit(self):
        import stat
        base = dict(st_dev=4, st_ino=5, st_mode=stat.S_IFDIR | 0o755, st_nlink=1,
                    st_uid=6, st_gid=7, st_size=0, st_mtime_ns=100, st_ctime_ns=90,
                    st_birthtime_ns=80, st_file_attributes=16, st_reparse_tag=0)
        path = mock.Mock()
        path.resolve.return_value = path
        original = SimpleNamespace(**base)
        reported = SimpleNamespace(**dict(base, st_file_attributes=0x10000010))
        verify = CASE["verify_inventory_directory"]
        with mock.patch.object(CASE["artifacts"], "WINDOWS", True):
            for before, after in ((original, reported), (reported, original)):
                path.lstat.return_value = after
                with mock.patch.object(os, "fstat", return_value=original), mock.patch.object(os, "read", side_effect=AssertionError("body read")):
                    verify(path, before, 17, original)
            for change in ({"st_file_attributes": 17}, {"st_file_attributes": 0x80000010},
                           {"st_file_attributes": 0x10000410}, {"st_reparse_tag": 1},
                           {"st_ino": 6}, {"st_uid": 8}, {"st_gid": 8},
                           {"st_mode": stat.S_IFDIR | 0o700}, {"st_ctime_ns": 91}):
                path.lstat.return_value = SimpleNamespace(**dict(base, **change))
                with self.subTest(change=change), mock.patch.object(os, "fstat", return_value=original):
                    with self.assertRaisesRegex(RuntimeError, "inventory directory changed"):
                        verify(path, reported, 17, original)
            path.lstat.return_value = original
            with mock.patch.object(os, "fstat", return_value=reported):
                with self.assertRaisesRegex(RuntimeError, "handle:st_file_attributes"):
                    verify(path, reported, 17, original)
            regular = SimpleNamespace(**dict(base, st_mode=stat.S_IFREG | 0o600, st_file_attributes=32))
            extra = SimpleNamespace(**dict(vars(regular), st_file_attributes=0x10000020))
            self.assertNotEqual(CASE["stable_named_entry"](regular), CASE["stable_named_entry"](extra))
            self.assertNotEqual(CASE["stable_file"](original), CASE["stable_file"](reported))
        with mock.patch.object(CASE["artifacts"], "WINDOWS", False):
            self.assertEqual(CASE["stable_named_entry"](original), CASE["stable_file"](original))


    def test_owned_census_attribute_diagnostic_preserves_refusal_and_io(self):
        import hashlib
        import stat
        base = dict(st_dev=4, st_ino=5, st_mode=stat.S_IFDIR | 0o755, st_nlink=1,
                    st_uid=6, st_gid=7, st_size=0, st_mtime_ns=100, st_ctime_ns=90,
                    st_birthtime_ns=80, st_file_attributes=16, st_reparse_tag=0)
        before, held = SimpleNamespace(**base), SimpleNamespace(**dict(base, st_file_attributes=32))
        path, relative = mock.Mock(), "private/member"
        path.resolve.return_value = path
        cases = [(16, 32, ("cache", relative), False), (0x80000010, 32, ("cache", relative), True),
                 (16, 48, ("payload", relative), True), (48, 64, ("work", "."), True)]
        cases += [(48, 32, context, False) for context in (None, ("private-role", relative),
                  ("cache", "/private"), ("cache", "../private"), ("cache", "x" * 4097), ("cache", "\udcff"))]
        cases += [(value, 32, ("cache", relative), False) for value in (True, -1, 2**32)]
        for left, right, context, diagnostic in cases:
            path.lstat.return_value = SimpleNamespace(**dict(base, st_file_attributes=left))
            opened = SimpleNamespace(**dict(base, st_file_attributes=right))
            path.lstat.reset_mock()
            with self.subTest(left=left, right=right, context=context), mock.patch.object(CASE["artifacts"], "WINDOWS", True):
                with mock.patch.object(os, "fstat", return_value=opened) as fstat, mock.patch.object(os, "stat") as stat_call:
                    with mock.patch.object(os, "read") as read, mock.patch.object(os, "unlink") as unlink, mock.patch.object(Path, "read_bytes") as read_bytes:
                        if (left, right) == (16, 32):
                            CASE["verify_inventory_directory"](path, before, 17, held, diagnostic=context)
                        else:
                            with self.assertRaisesRegex(RuntimeError, "inventory directory changed") as caught:
                                CASE["verify_inventory_directory"](path, before, 17, held, diagnostic=context)
                            message = str(caught.exception)
                            self.assertEqual("attributes:" in message, diagnostic)
                            self.assertNotIn("private", message)
                            if diagnostic:
                                encoded = message.split("attributes:", 1)[1][:-1]
                                self.assertLessEqual(len(encoded.encode("ascii")), 512)
                                self.assertEqual(json.loads(encoded), dict(kind="owned_census_windows_attributes", role=context[0],
                                    member_sha256=hashlib.sha256(context[1].encode()).hexdigest(),
                                    path_before=16, path_after=left, handle_before=32, handle_after=right))
                        path.lstat.assert_called_once_with(); fstat.assert_called_once_with(17)
                        stat_call.assert_not_called(); read.assert_not_called(); unlink.assert_not_called(); read_bytes.assert_not_called()

    def test_directory_temporal_refusal_names_domain_and_fields_without_values(self):
        import stat
        fields = CASE["STABLE_FIELDS"] + (("st_birthtime_ns", "st_file_attributes", "st_reparse_tag") if CASE["artifacts"].WINDOWS else ())
        base = {field: 100 for field in fields}
        base["st_mode"] = stat.S_IFDIR | 0o700
        original = SimpleNamespace(**base)
        path = mock.Mock(name="private-directory-path")
        path.resolve.return_value, path.lstat.return_value = path, original
        with mock.patch.object(os, "fstat", return_value=original) as opened:
            CASE["verify_inventory_directory"](path, original, 17, original)
            for domain in ("path", "handle"):
                for field in fields:
                    value = base[field] ^ 0o100 if field == "st_mode" else 123456789
                    changed = SimpleNamespace(**dict(base, **{field: value}))
                    path.lstat.return_value = changed if domain == "path" else original
                    opened.return_value = changed if domain == "handle" else original
                    path.lstat.reset_mock(); opened.reset_mock()
                    with self.subTest(domain=domain, field=field), self.assertRaises(RuntimeError) as caught:
                        CASE["verify_inventory_directory"](path, original, 17, original)
                    self.assertEqual(str(caught.exception), "inventory directory changed (" + domain + ":" + field + ")")
                    path.lstat.assert_called_once_with()
                    opened.assert_called_once_with(17)
            path.lstat.return_value, opened.return_value = original, original
            path.resolve.return_value = "private-canonical-path"
            opened.reset_mock()
            with self.assertRaises(RuntimeError) as caught:
                CASE["verify_inventory_directory"](path, original, 17, original)
            self.assertEqual(str(caught.exception), "inventory directory changed (canonical)")
            opened.assert_not_called()

    def test_inventory_does_not_eagerly_enumerate_past_membership_bound(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            for index in range(2048):
                (root / f"member-{index:04d}").mkdir()
            original_scandir, observed = os.scandir, [0]

            class CountedScan:
                def __init__(self, stream):
                    self.stream = stream

                def __enter__(self):
                    self.stream.__enter__()
                    return self

                def __exit__(self, *args):
                    return self.stream.__exit__(*args)

                def __iter__(self):
                    return self

                def __next__(self):
                    item = next(self.stream)
                    observed[0] += 1
                    if observed[0] > 2048:
                        raise RuntimeError("scanned beyond membership bound")
                    return item

            with mock.patch.object(os, "scandir", side_effect=lambda path: CountedScan(original_scandir(path))):
                with self.assertRaisesRegex(RuntimeError, "inventory membership bound"):
                    CASE["inventory"](root)
            self.assertEqual(observed[0], 2048)

    def test_protected_original_user_cannot_be_rebaselined(self):
        before = {"auth.json": {"sha256": "old"}, "history": {"sha256": "history"}}
        CASE["protected_unchanged"](before, dict(before), before)
        for name in before:
            changed = {**before, name: {"sha256": "native-new"}}
            with self.assertRaises(RuntimeError):
                CASE["protected_unchanged"](before, changed, before)

    def test_guard_rejects_live_gateway_multiplexer_service_and_external_memory(self):
        clean = {"gateway_running": False, "default_gateway_pid": None, "served_profiles": None,
                 "service_manager": "none", "memory_provider": None}
        CASE["validate_guards"](clean)
        for key, value in (("gateway_running", True), ("default_gateway_pid", 123),
                           ("served_profiles", []), ("service_manager", "s6"), ("memory_provider", "external")):
            with self.subTest(key=key), self.assertRaises(RuntimeError):
                CASE["validate_guards"]({**clean, key: value})
        with self.assertRaises(RuntimeError):
            CASE["validate_guards"]({**clean, "unexpected": False})

    def test_protocol_requires_selected_endpoint_and_zero_sibling_events(self):
        before = {"a": [], "b": []}
        valid = {"a": ["initialize", "notifications/initialized", "tools/list"], "b": []}
        CASE["protocol_join"](before, valid, "a", 1)
        CASE["protocol_join"](valid, valid, None, 0)
        for after in ({"a": ["initialize"], "b": []},
                      {"a": valid["a"], "b": ["initialize"]},
                      {"a": valid["a"] + ["tools/call"], "b": []},
                      {"a": ["tools/list", "initialize"], "b": []}):
            with self.assertRaises(RuntimeError):
                CASE["protocol_join"](before, after, "a", 1)
        with self.assertRaises(RuntimeError):
            CASE["protocol_join"](valid, before, "a", 1)

    def test_custom_root_must_not_be_original_native_home_or_named_profile(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            user = root / "user"
            native = user / ".hermes"
            a, b = root / "project-a" / "runtime", root / "project-b" / "runtime"
            for path in (native, a, b):
                path.mkdir(parents=True)
            CASE["validate_roots"](user, native, [a, b])
            invalid = [native, native / "child", user, root / "profiles" / "named"]
            for path in invalid:
                path.mkdir(parents=True, exist_ok=True)
                with self.subTest(path=path), self.assertRaises(RuntimeError):
                    CASE["validate_roots"](user, native, [path, b])
            with self.assertRaises(RuntimeError):
                CASE["validate_roots"](user, native, [a, a])

    def test_no_user_model_auth_or_history_copy_is_tolerated(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            config = root / "config.yaml"
            config.write_text("mcp_servers: {}\n")
            CASE["no_user_copies"](root, ["USER_MODEL_nonce", "USER_AUTH_nonce", "USER_HISTORY_nonce"])
            for marker in ("USER_MODEL_nonce", "USER_AUTH_nonce", "USER_HISTORY_nonce"):
                config.write_text(marker)
                with self.subTest(marker=marker), self.assertRaises(RuntimeError):
                    CASE["no_user_copies"](root, [marker])

    def test_census_readout_uses_closed_staged_bytes_without_private_context(self):
        import time
        stage = CASE["stage_evidence"]
        authority = stage.__globals__
        with tempfile.TemporaryDirectory() as temporary:
            native, evidence = Path(temporary).resolve() / "native", Path(temporary).resolve() / "evidence"
            native.mkdir(); evidence.mkdir()
            (native / "PRIVATE-NAME").write_bytes(b"PRIVATE-CONTENT")
            identity = CASE["directory_identity"](native)
            census = CASE["metadata_census"](native, identity, time.monotonic() + 5)
            nonce = "f" * 64
            (evidence / "bootstrap-census.json").write_text(json.dumps(census))
            (evidence / "bootstrap-census-request.json").write_text(json.dumps({
                "schema_version": 1, "reason": "initial-inventory-refused", "nonce": nonce,
                "root_identity": list(identity)}))
            (evidence / "profile-admission.json").write_text('{"admission_qualified":false}')
            lines, reads, actual = [], [], CASE["ordinary_bytes"]
            def read(path, *args, **kwargs):
                self.assertFalse((evidence / "upload").exists(), "readout reread after commit")
                reads.append(path.name)
                return actual(path, *args, **kwargs)
            def emit(line, *, flush):
                self.assertTrue(flush)
                self.assertTrue(authority["EVIDENCE_CLOSED"])
                self.assertTrue((evidence / "upload").is_dir())
                lines.append(line)
            with mock.patch.dict(authority, {"ordinary_bytes": read, "print": emit, "EVIDENCE_CLOSED": False}):
                stage(evidence)
                self.assertTrue(authority["EVIDENCE_CLOSED"])
            self.assertEqual(len(lines), 1)
            self.assertEqual(len(reads), 5)
            report = json.loads(lines[0].removeprefix("HERMES_BOOTSTRAP_CENSUS "))
            self.assertEqual(report["kind"], "hermes-bootstrap-census-readout")
            self.assertTrue(report["present"])
            self.assertTrue(report["measurement_only"])
            self.assertFalse(report["admission_qualified"])
            self.assertEqual(report["census"], census)
            for private in (nonce, str(native), str(evidence), "PRIVATE-NAME", "PRIVATE-CONTENT"):
                self.assertNotIn(private, lines[0])

    def test_census_readout_missing_invalid_commit_and_stdout_failures(self):
        stage = CASE["stage_evidence"]
        authority = stage.__globals__
        for mode in ("missing", "invalid", "rename", "stdout-oserror", "stdout-valueerror"):
            with self.subTest(mode=mode), tempfile.TemporaryDirectory() as temporary:
                evidence = Path(temporary).resolve()
                proof = b'{"admission_qualified":false,"error":"original native failure"}'
                (evidence / "profile-admission.json").write_bytes(proof)
                if mode != "missing":
                    census = {"schema_version": 1, "measurement_only": True, "admission_qualified": False,
                              "complete": False, "status": "refused", "reason": "request"}
                    if mode == "invalid": census["nonce"] = "private-not-a-census-field"
                    (evidence / "bootstrap-census.json").write_text(json.dumps(census))
                lines = []
                def emit(line, *, flush):
                    self.assertTrue(authority["EVIDENCE_CLOSED"])
                    self.assertTrue((evidence / "upload").is_dir())
                    lines.append(line)
                    if mode == "stdout-oserror": raise OSError("inert stdout closed")
                    if mode == "stdout-valueerror": raise ValueError("inert stream closed")
                with mock.patch.dict(authority, {"print": emit, "EVIDENCE_CLOSED": False}):
                    if mode == "invalid":
                        with self.assertRaisesRegex(RuntimeError, "closed census schema"):
                            stage(evidence)
                    elif mode == "rename":
                        with mock.patch.object(Path, "rename", side_effect=OSError("inert commit refusal")):
                            with self.assertRaisesRegex(OSError, "inert commit refusal"):
                                stage(evidence)
                    else:
                        stage(evidence)
                    if mode in ("invalid", "rename"):
                        self.assertFalse(authority["EVIDENCE_CLOSED"])
                        self.assertFalse((evidence / "upload").exists())
                        self.assertFalse(list(evidence.glob(".upload-pending-*")))
                        self.assertEqual(lines, [])
                    else:
                        self.assertTrue(authority["EVIDENCE_CLOSED"])
                        self.assertEqual((evidence / "upload/profile-admission.json").read_bytes(), proof)
                        self.assertEqual(len(lines), 1)
                        if mode == "missing":
                            self.assertEqual(json.loads(lines[0].removeprefix("HERMES_BOOTSTRAP_CENSUS ")), {
                                "schema_version": 1, "kind": "hermes-bootstrap-census-readout",
                                "measurement_only": True, "admission_qualified": False, "present": False})
                self.assertEqual((evidence / "profile-admission.json").read_bytes(), proof)

    def test_closed_upload_rejects_unknown_members_and_never_stages_private_tree(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            (root / "auth.json").write_text('{"inert":"must never upload"}')
            with self.assertRaises(RuntimeError):
                CASE["stage_evidence"](root)
            self.assertFalse((root / "upload").exists())
            (root / "auth.json").unlink()
            (root / "profile-admission.json").write_text('{"admission_qualified":false}')
            CASE["stage_evidence"](root)
            self.assertEqual([p.name for p in (root / "upload").iterdir()], ["profile-admission.json"])
            self.assertFalse(json.loads((root / "upload/profile-admission.json").read_text())["admission_qualified"])

    def test_late_second_member_failure_never_publishes_partial_upload(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            (root / "lifecycle.stdout").write_bytes(b"first")
            (root / "lifecycle.stderr").write_bytes(b"second")
            actual, calls = CASE["ordinary_bytes"], []
            def fail_second(path, expected=None):
                calls.append(path.name)
                if len(calls) == 2:
                    raise RuntimeError("late source read failure")
                return actual(path, expected)
            with mock.patch.dict(CASE["stage_evidence"].__globals__, {"ordinary_bytes": fail_second}):
                with self.assertRaisesRegex(RuntimeError, "late source read"):
                    CASE["stage_evidence"](root)
            self.assertEqual(len(calls), 2)
            self.assertFalse((root / "upload").exists())
            self.assertFalse(list(root.glob(".upload-pending-*")))

    def test_reader_rejects_wrong_opened_identity_before_reading(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            wanted, foreign = root / "wanted", root / "foreign"
            wanted.write_bytes(b"wanted")
            foreign.write_bytes(b"foreign")
            descriptor = CASE["artifacts"].open_member(foreign)
            with mock.patch.object(CASE["artifacts"], "open_member", return_value=descriptor):
                with mock.patch.object(os, "fdopen") as read:
                    with self.assertRaisesRegex(RuntimeError, "changed at open"):
                        CASE["ordinary_bytes"](wanted)
                    read.assert_not_called()
            with mock.patch.object(CASE["artifacts"], "open_member") as opened:
                with self.assertRaisesRegex(RuntimeError, "before open"):
                    CASE["ordinary_bytes"](wanted, foreign.stat())
                opened.assert_not_called()
            wanted.write_bytes(b"x" * ((1 << 20) + 1))
            with self.assertRaisesRegex(RuntimeError, "bounded ordinary"):
                CASE["ordinary_bytes"](wanted)

    @unittest.skipUnless(hasattr(os, "mkfifo") and hasattr(os, "O_NONBLOCK"), "POSIX FIFO replacement contract")
    def test_fifo_swap_is_nonblocking_and_refused_before_any_read(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            wanted, fifo = root / "wanted", root / "fifo"
            wanted.write_bytes(b"regular before open")
            os.mkfifo(fifo)
            original_open = os.open
            def swapped_open(path, flags):
                self.assertTrue(flags & os.O_NONBLOCK)
                return original_open(fifo, flags)
            with mock.patch.object(os, "open", side_effect=swapped_open):
                with mock.patch.object(os, "fdopen") as read:
                    with self.assertRaisesRegex(RuntimeError, "changed at open"):
                        CASE["ordinary_bytes"](wanted)
                    read.assert_not_called()

    def test_windows_path_and_fd_domains_keep_separate_complete_stability(self):
        with tempfile.TemporaryDirectory() as temporary:
            path = Path(temporary).resolve() / "ordinary"
            path.write_bytes(b"portable bytes")
            real_lstat, real_fstat, real_open = Path.lstat, os.fstat, os.open
            def win_info(info, held=False, **changes):
                names = ("st_dev", "st_ino", "st_mode", "st_nlink", "st_uid", "st_gid", "st_size", "st_mtime_ns", "st_ctime_ns")
                values = {name: getattr(info, name) for name in names}
                values.update(st_birthtime_ns=1234567, st_file_attributes=0, st_reparse_tag=0)
                if held:
                    values["st_ctime_ns"] += 100000
                    # Windows path and opened-handle APIs can synthesize
                    # execute permission bits differently on the same file.
                    values["st_mode"] ^= 0o111
                values.update(changes)
                return SimpleNamespace(**values)
            for mutation in (None, "path-ctime", "held-ctime", "path-mode", "held-mode",
                             "file-type", "inode", "birth", "reparse"):
                counts = {"path": 0, "held": 0}
                def path_stat(p):
                    counts["path"] += 1
                    value = win_info(real_lstat(p))
                    if mutation == "path-ctime" and counts["path"] > 1:
                        value.st_ctime_ns += 1
                    if mutation == "path-mode" and counts["path"] > 1:
                        value.st_mode ^= 0o200
                    return value
                def held_stat(fd):
                    counts["held"] += 1
                    value = win_info(real_fstat(fd), held=True)
                    if mutation == "held-ctime" and counts["held"] > 1:
                        value.st_ctime_ns += 1
                    if mutation == "held-mode" and counts["held"] > 1:
                        value.st_mode ^= 0o200
                    if mutation == "file-type":
                        value.st_mode = (value.st_mode & 0o7777) | 0o040000
                    if mutation == "inode": value.st_ino += 1
                    if mutation == "birth": value.st_birthtime_ns += 1
                    if mutation == "reparse": value.st_reparse_tag = 1
                    return value
                with self.subTest(mutation=mutation), mock.patch.object(CASE["artifacts"], "WINDOWS", True):
                    # Metadata-domain simulation only. No native Windows success is claimed.
                    with mock.patch.object(Path, "lstat", autospec=True, side_effect=path_stat), mock.patch.object(os, "fstat", side_effect=held_stat):
                        with mock.patch.object(CASE["artifacts"], "open_member", side_effect=lambda p: real_open(p, os.O_RDONLY)):
                            if mutation is None:
                                self.assertEqual(CASE["ordinary_bytes"](path), b"portable bytes")
                            else:
                                with self.assertRaises((RuntimeError, ValueError)):
                                    CASE["ordinary_bytes"](path)

    def test_file_read_access_time_does_not_change_stable_identity(self):
        with tempfile.TemporaryDirectory() as temporary:
            path = Path(temporary).resolve() / "ordinary"
            path.write_bytes(b"read exactly")
            current = path.stat()
            os.utime(path, ns=(1, current.st_mtime_ns))
            before = path.stat()
            self.assertEqual(CASE["ordinary_bytes"](path), b"read exactly")
            self.assertEqual(CASE["stable_file"](before), CASE["stable_file"](path.stat()))

    def test_original_custom_directory_replacement_or_mode_change_refuses(self):
        for mutation in ("replacement", "mode"):
            with self.subTest(mutation=mutation), tempfile.TemporaryDirectory() as temporary:
                parent = Path(temporary).resolve()
                root = parent / "selected"
                root.mkdir(mode=0o700)
                held = {"a": CASE["directory_identity"](root)}
                if mutation == "replacement":
                    root.rename(parent / "original")
                    root.mkdir(mode=0o700)
                else:
                    root.chmod(0o500)
                with self.assertRaisesRegex(RuntimeError, "original custom root directory changed") as caught:
                    CASE["verify_root_identities"]({"a": root}, held)
                message = str(caught.exception)
                self.assertNotIn(str(root), message)
                if mutation == "mode":
                    self.assertIn("(mode)", message)

    def test_exact_config_boundary_rejects_extra_server_wrong_url_and_other_settings(self):
        # These are mapping/byte contracts. The hosted child alone imports the
        # pinned Hermes YAML parser; no substitute parser is admitted here.
        boundary = CASE["config_document_boundary"]
        url = "http://127.0.0.1:12345/mcp"
        valid = {"mcp_servers": {"owned": {"url": url}}}
        body = json.dumps(valid).encode()
        digest = boundary(body, valid, "owned", url)
        boundary(body, valid, "owned", url, authored_sha=digest)
        with self.assertRaisesRegex(RuntimeError, "config bytes changed"):
            boundary(body + b"\n", valid, "owned", url, authored_sha=digest)
        for value in ({"mcp_servers": {"owned": {"url": url + "-changed"}}},
                      {"mcp_servers": {"owned": {"url": url}, "foreign": {"url": url}}},
                      {**valid, "model": "copied"}, {**valid, "auth": {}}, None):
            with self.assertRaises(RuntimeError):
                boundary(json.dumps(value).encode(), value, "owned", url)
        for value in ({}, {"mcp_servers": {}}):
            body = json.dumps(value).encode()
            boundary(body, value, "owned", url, removed=True)
            with self.assertRaises(RuntimeError):
                boundary(body, value, "owned", url, authored_sha=digest)

    def test_removed_config_accepts_only_measured_native_schema_stamp(self):
        # Pinned vendor save_config retains schema version 52 after removing the
        # final server. These bodies match the original native postimage hashes;
        # the original YAML bytes themselves were not uploaded.
        body = b"_config_version: 52\r\n" if os.name == "nt" else b"_config_version: 52\n"
        digest = ("9d2f633e8e0a5553913286ecc1b88d2511a69e545da69760b24135d02db75f3e" if os.name == "nt"
                  else "8bddefe963a867c332591ef2f50ec056944d1a323229b6294a4de7d9718a69cb")
        boundary = CASE["config_document_boundary"]
        value = {"_config_version": 52}
        self.assertEqual(len(body), 21 if os.name == "nt" else 20)
        self.assertEqual(CASE["hashlib"].sha256(body).hexdigest(), digest)
        self.assertEqual(boundary(body, value, "owned", "http://localhost/mcp", removed=True), digest)
        for changed_body, changed_value, removed in (
            (body, value, False), (body + b"\n", value, True),
            (b"_config_version: 51\n", {"_config_version": 51}, True),
            (body, {"_config_version": 52.0}, True),
            (body, {"_config_version": "52"}, True),
            (body, {"_config_version": True}, True),
            (body, {**value, "auth": {}}, True),
            (body, {**value, "mcp_servers": {}}, True),
        ):
            with self.subTest(value=changed_value, body=changed_body, removed=removed), self.assertRaises(RuntimeError):
                boundary(changed_body, changed_value, "owned", "http://localhost/mcp", removed=removed)
        with self.assertRaisesRegex(RuntimeError, "config bytes changed"):
            boundary(body, value, "owned", "http://localhost/mcp", removed=True, authored_sha="0" * 64)

    def test_removed_config_newline_is_exact_for_the_observed_platform(self):
        boundary = CASE["config_document_boundary"]
        bodies = {"posix": b"_config_version: 52\n", "nt": b"_config_version: 52\r\n"}
        # Original b7 Windows artifact 14084285338 records this exact 21-byte hash.
        self.assertEqual(CASE["hashlib"].sha256(bodies["nt"]).hexdigest(),
                         "9d2f633e8e0a5553913286ecc1b88d2511a69e545da69760b24135d02db75f3e")
        for platform, accepted in bodies.items():
            with self.subTest(platform=platform), mock.patch.object(CASE["os"], "name", platform):
                boundary(accepted, {"_config_version": 52}, "owned", "http://localhost/mcp", removed=True)
                for body in (bodies["nt" if platform == "posix" else "posix"], accepted + b"\n",
                             b"_config_version: 52\r", b"_config_version: 52\r\r\n", b"_config_version: 52"):
                    with self.subTest(body=body), self.assertRaisesRegex(RuntimeError, "config stamp"):
                        boundary(body, {"_config_version": 52}, "owned", "http://localhost/mcp", removed=True)

    def test_safe_failure_envelope_keeps_every_qualification_false(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = CASE["safe_failure_evidence"](Path(temporary).resolve())
            proof = json.loads((root / "upload/profile-admission.json").read_text())
            self.assertEqual(proof["status"], "failed")
            for key in ("admission_qualified", "production_project_mcp", "vendor_authentication", "real_model_inference", "tools_call", "source_install_custom_root_qualified"):
                self.assertIs(proof[key], False)


class ObserverFailureDiagnosticContracts(unittest.TestCase):
    nonce, source = "a" * 64, "b" * 40

    def encoded(self, value):
        return json.dumps(value, sort_keys=True, separators=(",", ":")).encode("ascii") + b"\n"

    def record(self):
        code = CASE["public_cleanup_complete"].__code__
        line = next(line for _, _, line in code.co_lines() if line is not None)
        return dict(schema_version=1, measurement_only=True, admission_qualified=False,
                    nonce=self.nonce, source_head=self.source, phase="public", reason="uncaught-observer-exception",
                    exception_kind="RuntimeError", command=None, caller=dict(function=code.co_name, function_line=code.co_firstlineno, line=line))

    def test_child_emits_only_closed_source_joined_failure_and_reraises_same_exception(self):
        import io
        import sys
        seen, original, stream = [], [], io.BytesIO()
        class Probe:
            def __init__(self, *, nonce): self.nonce = nonce
            def __enter__(self): return self
            def __exit__(self, *args): seen.append(self)
        def refuse(probes):
            try:
                CASE["source_refusal_document"](b"{}")
            except RuntimeError as error:
                error.args = ("PRIVATE CONFIG /outside/secret.yaml",)
                original.append(error)
                raise
        with mock.patch.dict(sys.modules, {"pi_project_components_native_e2e": SimpleNamespace(PiHTTPMCPProbe=Probe)}), \
                mock.patch.dict(os.environ, {"HERMES_CALL_NONCE": self.nonce, "HERMES_PROFILE_SOURCE_HEAD": self.source}, clear=True), \
                mock.patch.object(sys, "stdout", SimpleNamespace(buffer=stream)), \
                mock.patch.dict(CASE["child_case"].__globals__, source_capture_mode=lambda: False, observe_profiles=refuse):
            with self.assertRaises(RuntimeError) as caught: CASE["child_case"]()
        self.assertIs(caught.exception, original[0]); self.assertEqual(len(seen), 2)
        body = stream.getvalue(); self.assertLessEqual(len(body), 2048)
        value = CASE["observer_failure_document"](body, self.nonce, self.source)
        self.assertEqual(value["phase"], "profiles"); self.assertEqual(value["caller"]["function"], "source_refusal_document")
        self.assertNotIn(b"PRIVATE", body); self.assertNotIn(b"/outside", body); self.assertNotIn(b".py", body)
        stream.seek(0); stream.truncate()
        with mock.patch.object(sys, "stdout", SimpleNamespace(buffer=stream)), \
                mock.patch.dict(os.environ, {"HERMES_CALL_NONCE": self.nonce, "HERMES_PROFILE_SOURCE_HEAD": self.source}):
            try: CASE["public_json"](b'{"a":0,"a":1}')
            except RuntimeError as error: CASE["emit_observer_failure"](error, "public")
        nested = CASE["observer_failure_document"](stream.getvalue(), self.nonce, self.source)
        self.assertEqual(nested["caller"]["function"], "unique")
        # Even a closed stdout or unknown exception cannot replace the original.
        error = RuntimeError("original")
        with mock.patch.object(sys, "stdout", SimpleNamespace(buffer=mock.Mock(write=mock.Mock(side_effect=OSError("closed"))))), \
                mock.patch.dict(os.environ, {"HERMES_CALL_NONCE": self.nonce, "HERMES_PROFILE_SOURCE_HEAD": self.source}):
            CASE["emit_observer_failure"](error, "public")
        self.assertEqual(str(error), "original")

    def test_command_label_comes_only_from_the_exact_owned_nested_code(self):
        import io
        import sys
        from types import FunctionType
        owner = CASE["observe_public"]
        code = next(value for value in owner.__code__.co_consts if type(value) is type(owner.__code__) and value.co_name == "command")
        cell = (lambda value: lambda: value)(None).__closure__[0]
        command = FunctionType(code, owner.__globals__, closure=tuple(cell for _ in code.co_freevars))
        for label in ("public-repair-a", "PRIVATE /outside/config", None, ["public-add-a"], True):
            stream = io.BytesIO()
            with self.subTest(label=label), mock.patch.object(sys, "stdout", SimpleNamespace(buffer=stream)), \
                    mock.patch.dict(os.environ, {"HERMES_CALL_NONCE": self.nonce, "HERMES_PROFILE_SOURCE_HEAD": self.source}), \
                    mock.patch.dict(owner.__globals__, public_runtime_pins=mock.Mock(side_effect=RuntimeError("private"))):
                try: command(label, "a", [], native=False, inspect=False, expected_failure=False, test=None)
                except RuntimeError as error: CASE["emit_observer_failure"](error, "public")
            value = CASE["observer_failure_document"](stream.getvalue(), self.nonce, self.source)
            self.assertEqual(value["command"], "public-repair-a" if label == "public-repair-a" else None)
            self.assertEqual(value["caller"]["function_line"], code.co_firstlineno)
            self.assertNotIn(b"PRIVATE", stream.getvalue())
        stream = io.BytesIO(); label = "public-add-a"  # Same local name in a foreign frame grants nothing.
        with mock.patch.object(sys, "stdout", SimpleNamespace(buffer=stream)), \
                mock.patch.dict(os.environ, {"HERMES_CALL_NONCE": self.nonce, "HERMES_PROFILE_SOURCE_HEAD": self.source}):
            try: raise RuntimeError(label)
            except RuntimeError as error: CASE["emit_observer_failure"](error, "public")
        self.assertIsNone(CASE["observer_failure_document"](stream.getvalue(), self.nonce, self.source)["command"])

    def test_diagnostic_rejects_noncanonical_unknown_unjoined_and_foreign_callers(self):
        import copy
        value = self.record(); valid = self.encoded(value)
        self.assertEqual(CASE["observer_failure_document"](valid, self.nonce, self.source), value)
        invalid = [valid + valid, valid + b" ", valid.rstrip(), b"[]\n", valid[:-2] + b',"phase":"public"}\n', b"x" * 2049]
        for key, wrong in (("schema_version", True), ("measurement_only", 1), ("admission_qualified", 0),
                           ("nonce", "c" * 64), ("source_head", "c" * 40), ("phase", "unknown"),
                           ("reason", "PRIVATE MESSAGE"), ("exception_kind", "UnboundedExceptionClass"),
                           ("command", "PRIVATE /outside"), ("command", ["public-add-a"]), ("private", "data")):
            changed = copy.deepcopy(value); changed[key] = wrong; invalid.append(self.encoded(changed))
        for key, wrong in (("function", "../outside.py"), ("function", "unknown"), ("function_line", True),
                           ("function_line", 0), ("line", True), ("line", 1000000), ("path", "/private")):
            changed = copy.deepcopy(value); changed["caller"][key] = wrong; invalid.append(self.encoded(changed))
        for body in invalid:
            with self.subTest(body=body[:120]), self.assertRaises((RuntimeError, ValueError, TypeError)):
                CASE["observer_failure_document"](body, self.nonce, self.source)
        value["caller"] = None
        self.assertIsNone(CASE["observer_failure_document"](self.encoded(value), self.nonce, self.source)["caller"])

    def test_retained_descriptor_survives_name_substitution_and_requires_primary_settlement(self):
        import sys
        import subprocess
        value, reader = self.record(), CASE["read_observer_failure"]
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve(); path = root / "lifecycle.stdout"
            with path.open("x+b") as stream:
                descriptor = os.dup(stream.fileno()); original = os.fstat(descriptor)
                try:
                    # An inert Python child writes the original capture descriptor.
                    child = subprocess.Popen([sys.executable, "-I", "-c", "import sys; sys.stdout.buffer.write(bytes.fromhex(sys.argv[1]))", self.encoded(value).hex()], stdout=stream, stderr=subprocess.PIPE)
                    _, error = child.communicate(timeout=5)
                    self.assertEqual(child.returncode, 0); self.assertEqual(error, b"")
                    if os.name != "nt":
                        path.rename(root / "original-stdout"); path.write_bytes(b"private substituted pathname")
                    with mock.patch.object(CASE["os"], "read", side_effect=AssertionError("unsettled read")):
                        self.assertIsNone(reader(descriptor, original, self.nonce, self.source, primary_settled=False))
                    with mock.patch.object(Path, "open", side_effect=AssertionError("must not reopen pathname")), \
                            mock.patch.dict(reader.__globals__, ordinary_bytes=mock.Mock(side_effect=AssertionError("raw read"))):
                        self.assertEqual(reader(descriptor, original, self.nonce, self.source, primary_settled=True), value)
                finally: os.close(descriptor)

    def test_retained_descriptor_omits_malformed_overflow_alias_and_concurrent_change(self):
        reader, value = CASE["read_observer_failure"], self.record()
        for effect in ("empty", "duplicate", "overflow", "nonce", "read-error", "short", "changed", "hardlink"):
            with self.subTest(effect=effect), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary).resolve(); path = root / "stdout"
                with path.open("x+b") as stream:
                    original = os.fstat(stream.fileno()); body = self.encoded(value)
                    if effect == "empty": body = b""
                    if effect == "duplicate": body *= 2
                    if effect == "overflow": body = b"x" * 2049
                    if effect == "nonce": body = self.encoded(dict(value, nonce="c" * 64))
                    stream.write(body); stream.flush()
                    if effect == "hardlink": os.link(path, root / "alias")
                    actual_read = os.read
                    def changed(descriptor, length):
                        if effect == "read-error": raise PermissionError("private")
                        body = actual_read(descriptor, 1 if effect == "short" else length)
                        if effect == "changed": os.ftruncate(descriptor, 0)
                        return body
                    with mock.patch.object(CASE["os"], "read", side_effect=changed):
                        self.assertIsNone(reader(stream.fileno(), original, self.nonce, self.source, primary_settled=True))

    def test_safe_context_never_authorizes_failed_public_cleanup_or_exposes_raw_data(self):
        import copy
        record = self.record()
        context = dict(source_head=self.source, nonce=self.nonce, cleanup_phase="public-family",
                       primary_family_settled=True, observer_exit=1, observer=record)
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve(); home = root / "home"; (home / "public-a").mkdir(parents=True)
            for effect in ("valid", "primary-unsettled-no-record", "unsettled", "source", "extra", "phase", "exit", "caller"):
                changed = copy.deepcopy(context)
                if effect == "primary-unsettled-no-record": changed.update(primary_family_settled=False, observer=None)
                if effect == "unsettled": changed["primary_family_settled"] = False
                if effect == "source": changed["source_head"] = "c" * 40
                if effect == "extra": changed["private"] = "PRIVATE CONFIG"
                if effect == "phase": changed["cleanup_phase"] = "../private"
                if effect == "exit": changed["observer_exit"] = True
                if effect == "caller": changed["observer"]["caller"]["line"] = 999999
                directory = CASE["safe_failure_evidence"](root, "RuntimeError", native_failure=changed)
                raw = (directory / "upload/profile-admission.json").read_bytes(); proof = json.loads(raw)
                self.assertEqual(proof["original_failure_kind"], "RuntimeError")
                self.assertEqual(proof["status"], "failed"); self.assertNotIn(b"PRIVATE", raw)
                self.assertEqual(set(path.name for path in (directory / "upload").iterdir()), {"profile-admission.json"})
                self.assertFalse(CASE["public_cleanup_complete"](home, dict(proof, source={"commit": self.source})))
                for key in ("admission_qualified", "production_project_mcp", "vendor_authentication", "real_model_inference",
                            "tools_call", "source_install_custom_root_qualified", "source_bootstrap_recovery_qualified"):
                    self.assertIs(proof[key], False)
                if effect in ("valid", "primary-unsettled-no-record"): self.assertEqual(proof["native_failure"], changed)
                else: self.assertNotIn("native_failure", proof)


class BundledRuntimeContracts(unittest.TestCase):
    def fixture(self, parent):
        import hashlib
        root, source = parent / "payload", parent / "source"
        for name in ("hermes-agent", "tools/python/bin", "venv/lib/site-packages", "bin"):
            (root / name).mkdir(parents=True)
        (source / "pm").mkdir(parents=True)
        (source / "pm/lock.json").write_bytes(b"inert pinned lock")
        target = "win32-x64" if os.name == "nt" else "linux-x64"
        launcher = "bin/hermes" + (".exe" if os.name == "nt" else "")
        (root / launcher).write_bytes(b"inert launcher; never executed")
        (root / "tools/python/bin/python").write_bytes(b"inert interpreter; never executed")
        selected = {"code": "hermes-agent", "python": "tools/python/bin/python", "site_packages": "venv/lib/site-packages", "environment": "venv", "tools": "tools"}
        manifest = {"schema": 1, "target": target, "ref": CASE["BASE"]["PIN"], "repo": "hermes-agent", "venv": "venv", "store": "tools", "launchers": ["hermes"],
                    "runtime": {"repoDir": "hermes-agent", "toolsDir": "tools", "storePython": selected["python"], "sitePackages": selected["site_packages"], "commands": {"hermes": launcher}}}
        prepared = {"schema": 1, "out": str(root), "source": str(source), "revision": manifest["ref"], "lock": hashlib.sha256(b"inert pinned lock").hexdigest(),
                    "inputs": {**{name: str(root / value) for name, value in selected.items()}, "placement": "contained", "repo": "hermes-agent", "target": target, "ref": manifest["ref"]}}
        (root / "manifest.json").write_text(json.dumps(manifest))
        root.with_name("payload.prepared.json").write_text(json.dumps(prepared))
        return root, source, target, manifest, prepared

    def test_manifest_and_prepared_inputs_bind_actual_native_paths(self):
        with tempfile.TemporaryDirectory() as temporary:
            root, source, target, manifest, prepared = self.fixture(Path(temporary).resolve())
            proof = CASE["payload_contract"](root, source, target)
            self.assertEqual(proof["native_command"], [str(root / manifest["runtime"]["commands"]["hermes"])])
            self.assertIs(proof["source_install_custom_root_qualified"], False)
            for document, key, wrong in ((manifest, "ref", "0" * 40), (manifest, "target", "other"),
                    (prepared, "source", str(root)), (prepared, "lock", "0" * 64),
                    (prepared["inputs"], "python", str(source / "python")),
                    (manifest["runtime"], "storePython", "../source/python"),
                    (manifest["runtime"], "sitePackages", str(source)),
                    (manifest["runtime"]["commands"], "hermes", "bin/other")):
                original = document[key]
                document[key] = wrong
                (root / "manifest.json").write_text(json.dumps(manifest))
                root.with_name("payload.prepared.json").write_text(json.dumps(prepared))
                with self.subTest(key=key), self.assertRaises(RuntimeError):
                    CASE["payload_contract"](root, source, target)
                document[key] = original

    def test_two_internal_regular_aliases_are_closed_before_stream_hashes(self):
        import hashlib
        import time
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve(); (root / "first").write_bytes(b"owned runtime")
            os.link(root / "first", root / "second")
            identity = CASE["directory_identity"](root)
            first = CASE["payload_snapshot"](root, identity, time.monotonic() + 5)
            self.assertEqual(first["logical_regular_bytes"], 2 * len(b"owned runtime"))
            self.assertEqual(first["entries"], 3)
            self.assertEqual(first, CASE["payload_snapshot"](root, identity, time.monotonic() + 5))
            self.assertNotIn("owned runtime", json.dumps(first))

    def test_outside_regular_alias_refuses_without_reading_any_file_body(self):
        import time
        with tempfile.TemporaryDirectory() as temporary:
            parent = Path(temporary).resolve(); root = parent / "payload"; root.mkdir()
            foreign = parent / "private"; foreign.write_bytes(b"FOREIGN-PRIVATE-BYTES")
            os.link(foreign, root / "borrowed")
            identity = CASE["directory_identity"](root)
            with mock.patch.object(os, "read", side_effect=AssertionError("foreign body read")):
                with self.assertRaisesRegex(CASE["PayloadHardlinkRefusal"], "hardlink escapes runtime") as refused:
                    CASE["payload_snapshot"](root, identity, time.monotonic() + 5)
            self.assertEqual(foreign.read_bytes(), b"FOREIGN-PRIVATE-BYTES")
            diagnostic, info = refused.exception.diagnostic, foreign.stat()
            self.assertEqual(set(diagnostic), {"schema_version", "measurement_only", "member", "member_sha256",
                             "hash_encoding", "device", "inode", "links", "captured_aliases", "uid", "gid", "integer_saturated"})
            self.assertEqual((diagnostic["member"], diagnostic["links"], diagnostic["captured_aliases"]), ("borrowed", 2, 1))
            observed = [info.st_dev, info.st_ino, info.st_uid, info.st_gid]
            bounded = [value if type(value) is int and 0 <= value <= (1 << 64) - 1 else None for value in observed]
            self.assertEqual([diagnostic[key] for key in ("device", "inode", "uid", "gid")], bounded)
            self.assertEqual(diagnostic["integer_saturated"], any(value is None for value in bounded))
            evidence = parent / "evidence"; evidence.mkdir()
            proof = dict(status="failed", admission_qualified=False, production_project_mcp=False,
                         tools_call=False, payload_hardlink_refusal=diagnostic)
            CASE["write_json"](evidence / "profile-admission.json", proof)
            with mock.patch.dict(CASE["stage_evidence"].__globals__, {"EVIDENCE_CLOSED": False}):
                CASE["stage_evidence"](evidence)
            published = (evidence / "upload/profile-admission.json").read_text()
            self.assertEqual(json.loads(published), proof)
            self.assertNotIn(str(parent), published); self.assertNotIn("FOREIGN-PRIVATE-BYTES", published)

    def test_payload_alias_diagnostic_bounds_names_integers_and_encoding(self):
        import hashlib
        import time
        original = (4, 5, 0o100600, 2, 7, 100, 101, 8, 9)
        for member in ("x" * 160, "venv/é.py", "venv/" + chr(0xDCFF) + ".py"):
            with self.subTest(member=member), mock.patch.object(os, "read", side_effect=AssertionError("diagnostic read")):
                with self.assertRaisesRegex(CASE["PayloadHardlinkRefusal"], "hardlink escapes runtime") as refused:
                    CASE["payload_hardlink_closure"]({member: {"type": "file", "identity": original}}, time.monotonic() + 5)
                diagnostic = refused.exception.diagnostic
                self.assertEqual(diagnostic["member"], member if len(json.dumps(member, ensure_ascii=True).encode("ascii")) <= 128 else None)
                self.assertEqual(diagnostic["member_sha256"], hashlib.sha256(member.encode("utf-8", "surrogatepass")).hexdigest())
                self.assertEqual(diagnostic["hash_encoding"], "utf-8-surrogatepass")
                self.assertTrue(diagnostic["measurement_only"]); self.assertFalse(diagnostic["integer_saturated"])
                self.assertLessEqual(len(json.dumps(diagnostic, ensure_ascii=True).encode("ascii")), 1024)
        oversized = ((1 << 65), (1 << 65), *original[2:-2], -1, (1 << 65))
        diagnostic = CASE["PayloadHardlinkRefusal"](oversized, ["owned"]).diagnostic
        for key in ("device", "inode", "uid", "gid"):
            self.assertIsNone(diagnostic[key])
        self.assertTrue(diagnostic["integer_saturated"])
        for member in ("/outside", "../outside", "C:/outside", "bad\\\\name"):
            error = CASE["PayloadHardlinkRefusal"](original, [member])
            self.assertIsNone(error.diagnostic)
            self.assertEqual(str(error), "payload hardlink escapes runtime")

    def test_payload_alias_diagnostic_rejects_malformed_containers(self):
        original = (4, 5, 0o100600, 2, 7, 100, 101, 8, 9)
        cases = (({}, ["owned"]), (original[:4], ["owned"]), (list(original), ["owned"]),
                 (original, "owned"), (original, []), (original, ["owned", None]),
                 (original, ["owned"] * (CASE["PAYLOAD_MEMBERS"] + 1)))
        with mock.patch.object(os, "read", side_effect=AssertionError("diagnostic read")):
            for identity, names in cases:
                with self.subTest(identity=type(identity).__name__, names=type(names).__name__):
                    error = CASE["PayloadHardlinkRefusal"](identity, names)
                    self.assertIsNone(error.diagnostic)
                    self.assertEqual(str(error), "payload hardlink escapes runtime")

    def test_payload_alias_diagnostic_saturates_link_count_before_json(self):
        import time
        original = (4, 5, 0o100600, 1 << 20000, 7, 100, 101, 8, 9)
        with mock.patch.object(os, "read", side_effect=AssertionError("diagnostic read")):
            with self.assertRaisesRegex(CASE["PayloadHardlinkRefusal"], "hardlink escapes runtime") as refused:
                CASE["payload_hardlink_closure"]({"owned": {"type": "file", "identity": original}}, time.monotonic() + 5)
            errors = [refused.exception] + [
                CASE["PayloadHardlinkRefusal"]((*original[:3], links, *original[4:]), ["owned"])
                for links in (True, "2")]
        for error in errors:
            self.assertIsNone(error.diagnostic["links"])
            self.assertTrue(error.diagnostic["integer_saturated"])
            self.assertLessEqual(len(json.dumps(error.diagnostic, ensure_ascii=True).encode("ascii")), 1024)
            self.assertLessEqual(len(type(error).__name__ + ": " + str(error)), 300)

    def test_late_outside_alias_is_rejoined_before_first_hash_read(self):
        import time
        function = CASE["payload_hardlink_closure"]
        with tempfile.TemporaryDirectory() as temporary:
            parent = Path(temporary).resolve(); root = parent / "payload"; root.mkdir()
            leaf = root / "runtime"; leaf.write_bytes(b"owned bytes")
            identity = CASE["directory_identity"](root)
            def add_alias(records, deadline):
                function(records, deadline)
                os.link(leaf, parent / "late-foreign")
            with mock.patch.dict(CASE["payload_snapshot"].__globals__, {"payload_hardlink_closure": add_alias}), mock.patch.object(os, "read", side_effect=AssertionError("hash before late-link check")):
                with self.assertRaisesRegex(RuntimeError, "metadata changed before hashing"):
                    CASE["payload_snapshot"](root, identity, time.monotonic() + 5)
            self.assertEqual((parent / "late-foreign").read_bytes(), b"owned bytes")

    def test_same_inode_aliases_must_have_identical_complete_metadata(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve(); first = root / "first"; first.write_bytes(b"owned")
            os.link(first, root / "second")
            before = CASE["stable_file"](first.stat())
            info = first.stat(); os.utime(first, ns=(info.st_atime_ns, info.st_mtime_ns - 60_000_000_000))
            after = CASE["stable_file"]((root / "second").stat())
            self.assertEqual(before[:2], after[:2]); self.assertNotEqual(before, after)
            records = {"first": {"type": "file", "identity": before}, "second": {"type": "file", "identity": after}}
            with mock.patch.object(os, "read", side_effect=AssertionError("body before metadata closure")):
                with self.assertRaisesRegex(RuntimeError, "hardlink metadata differs"):
                    CASE["payload_hardlink_closure"](records, __import__("time").monotonic() + 5)

    def test_complete_metadata_pass_keeps_original_deadline_and_byte_bound(self):
        import time
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve(); (root / "runtime").write_bytes(b"owned")
            identity = CASE["directory_identity"](root)
            with mock.patch.dict(CASE["payload_snapshot"].__globals__, {"PAYLOAD_BYTES": 1}), mock.patch.object(os, "read", side_effect=AssertionError("body before byte bound")):
                with self.assertRaisesRegex(RuntimeError, "payload byte bound"):
                    CASE["payload_snapshot"](root, identity, time.monotonic() + 5)
            with mock.patch.object(os, "read", side_effect=AssertionError("body after deadline")):
                with self.assertRaisesRegex(RuntimeError, "payload scan deadline"):
                    CASE["payload_snapshot"](root, identity, time.monotonic() - 1)

    def test_payload_streams_large_bytes_but_configuration_remains_bounded(self):
        import time
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            binary = root / "runtime"
            binary.write_bytes(b"x" * ((1 << 20) + 1))
            expected = CASE["directory_identity"](root)
            capture = lambda: CASE["payload_snapshot"](root, expected, time.monotonic() + 5)
            original = capture()
            self.assertEqual(original["logical_regular_bytes"], (1 << 20) + 1)
            self.assertEqual(original, capture())
            with self.assertRaisesRegex(RuntimeError, "inventory member refused"):
                CASE["inventory"](root)
            binary.write_bytes(b"y" * ((1 << 20) + 1))
            self.assertNotEqual(original["sha256"], capture()["sha256"])
            for key, value, message in (("PAYLOAD_BYTES", 2, "byte bound"), ("PAYLOAD_MEMBERS", 1, "membership/depth"), ("PAYLOAD_DEPTH", 0, "membership/depth")):
                with mock.patch.dict(CASE["payload_snapshot"].__globals__, {key: value}), self.assertRaisesRegex(RuntimeError, message):
                    capture()
            with self.assertRaisesRegex(RuntimeError, "deadline"):
                CASE["payload_snapshot"](root, expected, time.monotonic() - 1)

    def test_payload_replacement_and_midstream_mutation_are_refused(self):
        import time
        with tempfile.TemporaryDirectory() as temporary:
            parent = Path(temporary).resolve()
            root = parent / "runtime"; root.mkdir()
            path = root / "file"; path.write_bytes(b"one")
            expected, actual_read = CASE["directory_identity"](root), os.read
            original_mtime = path.stat().st_mtime_ns
            def changed(fd, length):
                body = actual_read(fd, length)
                if body:
                    path.write_bytes(b"two")
                    # Windows may expose the same mtime for two immediate writes.
                    # Retain the same-size mutation but make its metadata effect exact.
                    os.utime(path, ns=(path.stat().st_atime_ns, original_mtime + 2_000_000_000))
                    self.assertNotEqual(path.stat().st_mtime_ns, original_mtime)
                return body
            with mock.patch.object(os, "read", side_effect=changed), self.assertRaisesRegex(RuntimeError, "changed during capture"):
                CASE["payload_snapshot"](root, expected, time.monotonic() + 5)
            root.rename(parent / "old"); root.mkdir()
            with mock.patch.object(os, "scandir", side_effect=AssertionError("foreign tree visited")), self.assertRaisesRegex(RuntimeError, "original root changed"):
                CASE["payload_snapshot"](root, expected, time.monotonic() + 5)

    def test_payload_links_resolve_captured_graph_without_target_traversal(self):
        import time
        root = Path(tempfile.gettempdir()).resolve() / "inert-payload"
        records = {".": {"type": "directory"}, "dir": {"type": "directory"}, "file": {"type": "file"},
                   "dir/up": {"type": "link", "target": ".."}, "link": {"type": "link", "target": "dir/up/file"}}
        CASE["payload_links"](root, records, time.monotonic() + 5)
        for target in ("dir/up/../outside", str(root.parent / "outside"), "missing", "link"):
            records["link"]["target"] = target
            with self.subTest(target=target), mock.patch.object(Path, "resolve", side_effect=AssertionError("link traversed")), self.assertRaises(RuntimeError):
                CASE["payload_links"](root, records, time.monotonic() + 5)

    def test_payload_alias_closure_keeps_original_deadline_and_positive_identity(self):
        import time
        original = {"type": "file", "identity": (4, 5, 0o100600, 1, 7, 100, 101, 8, 9)}
        with self.assertRaisesRegex(RuntimeError, "deadline"):
            CASE["payload_hardlink_closure"]({"one": original}, time.monotonic() - 1)
        for index in (0, 1, 3):
            identity = list(original["identity"]); identity[index] = 0
            with self.subTest(index=index), self.assertRaisesRegex(RuntimeError, "identity unavailable"):
                CASE["payload_hardlink_closure"]({"one": {**original, "identity": identity}}, time.monotonic() + 5)

    def test_payload_extended_windows_links_require_local_absolute_drive(self):
        import time
        from pathlib import PureWindowsPath
        records = {".": {"type": "directory"}, "file": {"type": "file"},
                   "link": {"type": "link", "target": r"\\?\C:\payload\file"}}
        for name in ("UNC", "UNC/server", "UNC/server/share", "GLOBALROOT", "GLOBALROOT/Device"):
            records[name] = {"type": "directory"}
        for name in ("UNC/server/share/file", "GLOBALROOT/Device/file"):
            records[name] = {"type": "file"}
        with mock.patch.dict(CASE["payload_links"].__globals__, {"Path": PureWindowsPath}), mock.patch.object(CASE["artifacts"], "WINDOWS", True):
            CASE["payload_links"](PureWindowsPath("C:/payload"), records, time.monotonic() + 5)
            for target in (r"\\?\UNC\server\share\file", r"\\?\GLOBALROOT\Device\file", r"\\?\C:file"):
                records["link"]["target"] = target
                with self.subTest(target=target), self.assertRaisesRegex(RuntimeError, "payload device namespace link"):
                    CASE["payload_links"](PureWindowsPath("C:/payload"), records, time.monotonic() + 5)


    @unittest.skipIf(os.name == "nt", "physical POSIX symlink capture; graph validation runs on every OS")
    def test_payload_records_link_text_and_never_opens_escaping_target(self):
        import time
        with tempfile.TemporaryDirectory() as temporary:
            parent = Path(temporary).resolve()
            root = parent / "runtime"; root.mkdir()
            for name in ("one", "two"): (root / name).write_bytes(b"same")
            link = root / "link"; link.symlink_to("one")
            expected = CASE["directory_identity"](root)
            capture = lambda: CASE["payload_snapshot"](root, expected, time.monotonic() + 5)
            before = capture()
            self.assertEqual((before["links"], before["logical_regular_bytes"]), (1, 8))
            link.unlink(); link.symlink_to("two")
            self.assertNotEqual(before["sha256"], capture()["sha256"])
            (parent / "foreign").write_bytes(b"must not be read")
            link.unlink(); link.symlink_to("../foreign")
            actual_open = os.open
            def guarded(path, *args, **kwargs):
                self.assertNotEqual(Path(path).name, "foreign")
                return actual_open(path, *args, **kwargs)
            with mock.patch.object(os, "open", side_effect=guarded), self.assertRaisesRegex(RuntimeError, "escapes runtime"):
                capture()

    def test_source_default_environment_is_preserved_and_bundle_outputs_are_closed(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            source = {key: str(root / key) for key in ("HERMES_BIN", "UAP_BIN", "RUNNER_TEMP", "HERMES_NATIVE_RUNTIME_COMMAND_FILE")}
            with mock.patch.dict(os.environ, {**source, "PATH": "fixture", "SECRET": "never-forward", "HERMES_PROFILE_PYTHON": "ambient"}, clear=True):
                default = CASE["BASE"]["closed_environment"](root, root, "nonce", "url")
                self.assertEqual({key: default[key] for key in source}, source)
                self.assertNotIn("SECRET", default); self.assertNotIn("HERMES_PROFILE_PYTHON", default)
                installation = {key: str(root / key) for key in ("HERMES_BIN", "RUNNER_TEMP", "HERMES_PROFILE_RUNTIME_ROOT", "HERMES_PROFILE_PYTHON")}
                bundled = CASE["BASE"]["closed_environment"](root, root, "nonce", "url", installation=installation)
                self.assertEqual({key: value for key, value in default.items() if key not in source}, {key: value for key, value in bundled.items() if key not in installation})
                self.assertEqual({key: bundled[key] for key in installation}, installation)
                for invalid in ({**installation, "SECRET": "never-forward"}, {"HERMES_BIN": "missing-other-outputs"}):
                    with self.assertRaisesRegex(RuntimeError, "bundled runtime outputs"):
                        CASE["BASE"]["closed_environment"](root, root, "nonce", "url", installation=invalid)


class PublicLifecycleContracts(unittest.TestCase):
    def test_public_stdio_recipe_keeps_original_probe_authority(self):
        import hashlib
        for suffix in ("", ".exe"):
            with self.subTest(suffix=suffix), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary).resolve() / "original build root"; root.mkdir()
                environment = {}
                for role, path_key, hash_key in (
                    ("uap", "UAP_BIN", "UAP_HERMES_COLD_UAP_SHA256"),
                    ("cold", "UAP_HERMES_COLD_TEST_BIN", "UAP_HERMES_COLD_TEST_SHA256"),
                    ("probe", "UAP_HERMES_MCP_PROBE_BIN", "UAP_HERMES_MCP_PROBE_SHA256")):
                    path = root / (("hermes-mcp-probe" if role == "probe" else role) + suffix)
                    body = role.encode(); path.write_bytes(body); path.chmod(0o700)
                    environment[path_key] = str(path)
                    environment[hash_key] = hashlib.sha256(body).hexdigest()
                with mock.patch.dict(os.environ, environment):
                    pins = CASE["public_runtime_pins"]()
                    for scope in ("a", "b"):
                        project = root.parent / ("project-" + scope)
                        recipe = CASE["public_stdio_recipe"](project, pins["probe"], "original-nonce")
                        self.assertEqual(recipe, {"type": "stdio", "command": "hermes-mcp-probe" + suffix,
                            "args": [], "env": {"PATH": str(root), "UAP_TEST_ROOT": str(project),
                            "UAP_TEST_EVENTS": "${PLUGIN_DATA}/hermes-events.jsonl", "UAP_TEST_NONCE": "original-nonce"}})
                        self.assertEqual(recipe["env"]["PATH"].split(os.pathsep), [str(root)])
                    self.assertEqual(CASE["public_runtime_pins"](pins), pins)
                    probe = Path(pins["probe"]["path"])
                    original = probe.read_bytes(); probe.rename(root / "original-probe")
                    probe.write_bytes(original); probe.chmod(0o700)
                    with mock.patch.dict(CASE["public_runtime_pins"].__globals__,
                            ordinary_bytes=mock.Mock(side_effect=AssertionError("replacement body read"))), \
                            self.assertRaisesRegex(RuntimeError, "original public runtime changed"):
                        CASE["public_runtime_pins"](pins, rehash=False)

    def test_continuity_rejects_individually_valid_but_detached_state_and_data(self):
        import copy
        import shutil
        for effect in ("unchanged", "canonical", "requested", "source-id", "data-owner", "binding-id", "created", "data-directory", "receipt-history", "typed-package"):
            with self.subTest(effect=effect), tempfile.TemporaryDirectory() as temporary:
                outer = Path(temporary).resolve() / "outer"; outer.mkdir()
                project, state_path, state, _, pins = self.fixture(outer)
                state["installations"][0]["package"]["inventory"] = {"mcp_present": True}
                state_path.write_bytes(json.dumps(state).encode())
                _, previous = CASE["public_snapshot"](outer, {"a": project}, pins)
                changed = copy.deepcopy(state)
                installed = changed["installations"][0]
                if effect == "canonical": installed["source"]["canonical_source"] = "different-canonical"
                elif effect == "requested": installed["source"]["requested_source"] = "different-request"
                elif effect == "source-id": installed["source"]["source_binding_id"] = "different-source"
                elif effect == "data-owner":
                    installed["data_receipts"]["data"]["ownership_digest"] = "sha256:" + "0" * 64
                    (previous["a"]["data"] / ".agentplugins-data-owner.json").write_bytes(json.dumps(installed["data_receipts"]["data"]).encode())
                elif effect == "binding-id":
                    binding = installed["clients"].pop("binding")
                    binding["client_binding_id"] = "different-binding"
                    binding["receipts"][0]["client_binding_id"] = "different-binding"
                    installed["clients"]["different-binding"] = binding
                    changed["transaction_receipts"][0]["client_binding_id"] = "different-binding"
                elif effect == "created": installed["created_at"] = "different-creation"
                elif effect == "receipt-history": installed["clients"]["binding"]["receipts"][0]["operation_id"] = "changed-history"
                elif effect == "typed-package": installed["package"]["inventory"]["mcp_present"] = 1
                elif effect == "data-directory":
                    original = previous["a"]["data"]
                    original.rename(outer.parent / "original-data")
                    shutil.copytree(outer.parent / "original-data", original)
                state_path.write_bytes(json.dumps(changed).encode())
                # Both complete package/receipt snapshots remain individually
                # admissible. Only the original cross-operation identity closes
                # this gap; no native executable or capability is faked here.
                _, current = CASE["public_snapshot"](outer, {"a": project}, pins)
                if effect == "unchanged": CASE["public_continuity"](previous["a"], current["a"])
                else:
                    with self.assertRaisesRegex(RuntimeError, "public original"):
                        CASE["public_continuity"](previous["a"], current["a"])

    def test_local_update_can_advance_only_revision_facts_after_snapshot_validation(self):
        import copy
        with tempfile.TemporaryDirectory() as temporary:
            outer = Path(temporary).resolve()
            project, _, _, _, pins = self.fixture(outer)
            _, proofs = CASE["public_snapshot"](outer, {"a": project}, pins)
            original, changed = proofs["a"], copy.deepcopy(proofs["a"])
            changed["installed"]["source"]["tree_digest"] = "sha256:" + "1" * 64
            changed["installed"]["package"].update(version="1.1.0", manifest_digest="sha256:" + "2" * 64)
            changed["binding"]["package_revision"].update(tree_digest="sha256:" + "1" * 64, version="1.1.0", manifest_digest="sha256:" + "2" * 64)
            CASE["public_continuity"](original, changed, advance=True)
            with self.assertRaisesRegex(RuntimeError, "source/package"):
                CASE["public_continuity"](original, changed)
            changed["installed"]["source"]["canonical_source"] = "detached-origin"
            with self.assertRaisesRegex(RuntimeError, "source/package"):
                CASE["public_continuity"](original, changed, advance=True)

    def test_source_capture_keeps_its_original_probe_family_without_public_runtime(self):
        import sys
        for source in (True, False):
            with self.subTest(source=source):
                events, created = [], []
                class Probe:
                    def __init__(self, *, nonce):
                        self.nonce = nonce
                        created.append(self)
                    def __enter__(self):
                        return self
                    def __exit__(self, *args):
                        events.append(("closed", self))
                def raw(probes):
                    self.assertEqual(list(probes.values()), created[:2])
                    events.append(("raw", tuple(probes)))
                def public(probes):
                    self.assertFalse(source)
                    self.assertEqual(list(probes.values()), created[2:])
                    self.assertEqual(events, [("raw", ("a", "b"))])
                    events.append(("public", tuple(probes)))
                with mock.patch.dict(sys.modules, {"pi_project_components_native_e2e": SimpleNamespace(PiHTTPMCPProbe=Probe)}), \
                        mock.patch.dict(os.environ, {"HERMES_CALL_NONCE": "inert-separation-contract"}, clear=True), \
                        mock.patch.dict(CASE["child_case"].__globals__, source_capture_mode=lambda: source,
                                        observe_profiles=raw, observe_public=public):
                    CASE["child_case"]()
                self.assertEqual(len(created), 2 if source else 4)
                self.assertEqual(sum(event[0] == "closed" for event in events), len(created))

    def parent_authority(self, root):
        return dict(version=1,canonical_root=str(root),ancestry=[dict(canonical_path=str(path),scheme='linux-fsuuid-inode-v1',volume_id='12',object_id=str(index+1))
                    for index,path in enumerate(reversed((root,*root.parents)))])

    def fixture(self, root):
        import hashlib
        project, name = root / 'public-a', 'hermes-public-a'
        installation = '00000000-0000-4000-8000-000000000001'
        physical = name + '-' + hashlib.sha256(installation.encode()).hexdigest()[:12]
        target, selected, data = project / '.uap/managed/clients/hermes' / physical, project / '.hermes/uap-session', project / '.uap/plugin-data' / physical
        for path in (target, selected, data): path.mkdir(parents=True)
        selected.chmod(0o700)
        def write(path, body, mode=0o600):
            path.parent.mkdir(parents=True, exist_ok=True); path.write_bytes(body); path.chmod(mode)
        write(target / 'plugin.json', json.dumps({'name': name, 'version': '1.0.0'}).encode())
        write(target / 'skills' / name / 'SKILL.md', b'authored Skill\n')
        launcher = target / ('io.github.777genius.agentplugins/managed-stdio-v1/agentplugins' + ('.exe' if os.name == 'nt' else ''))
        runtime = b'fixed inert UAP bytes; never executed\n' * 32000
        write(launcher, runtime, 0o700)
        digest = hashlib.sha256()
        for path in sorted(target.rglob('*')):
            name_in_package = path.relative_to(target).as_posix()
            kind = 'dir' if path.is_dir() else 'file'; body = b'' if path.is_dir() else path.read_bytes()
            executable = kind == 'file' and os.name != 'nt' and bool(path.stat().st_mode & 0o111)
            digest.update((f'{kind}\0{name_in_package}\0{str(executable).lower()}\0{len(body)}\0').encode()); digest.update(body)
        package_digest = 'sha256:' + digest.hexdigest()
        marker = ('uap-hermes-project-session-v1\n' + str(project) + '\n').encode()
        write(selected / '.uap-owner', marker); write(selected / 'config.yaml', b'mcp_servers: {}\n')
        owner = dict(data_receipt_id='data', physical_backend_id=physical, scope='project', state='owned', locator=str(data), ownership_digest='sha256:'+'e'*64)
        write(data / '.agentplugins-data-owner.json', json.dumps(owner).encode())
        objects = [dict(object_id='package:hermes:' + physical, kind='managed_package_directory', logical_name=name, path=str(target), managed_digest=package_digest, protection_class='managed'),
                   dict(object_id='hermes_project_session_root', kind='hermes_project_session_root', path=str(selected / '.uap-owner'), managed_digest=hashlib.sha256(marker).hexdigest(), protection_class='owned'),
                   dict(kind='hermes_project_skill', object_id='hermes_project_skill:'+name, logical_name=name,
                        source_relative='skills/'+name, path=str(project/'.hermes/skills'/name), protection_class='managed',
                        managed_digest='sha256:'+hashlib.sha256(b'f\0SKILL.md\0-\0authored Skill\n\0').hexdigest())]
        objects += [dict(kind='hermes_project_session_mcp', logical_name=name+'-'+transport, object_id='hermes_project_session_mcp:'+name+'-'+transport,
                        path=str(selected/'config.yaml'), protection_class='managed', managed_digest='sha256:'+'a'*64) for transport in ('http','stdio')]
        source = dict(source_binding_id='source', tree_digest='sha256:'+'d'*64, resolved_revision='')
        package = dict(version='1.0.0', manifest_digest='sha256:'+'f'*64)
        receipt = dict(sequence=1, mutation_type='directory_swap', phase='committed', client_binding_id='binding', active_path=str(target), after_digest=package_digest)
        binding = dict(client_id='hermes', client_binding_id='binding', scope='project', target_locator=str(target), physical_artifact_id=physical,
                       native_profile_root=str(project/'.hermes'), activation='prepared', verification='installation_verified',
                       native_objects=objects, receipts=[receipt], package_revision={**package, **{k:source[k] for k in ('tree_digest','resolved_revision')}}, data_receipt_id='data')
        state = dict(schema_version=5, installations=[dict(installation_id=installation, declared_name=name, clients={'binding':binding}, source=source, package=package, data_receipts={'data':owner})],
                     transaction_receipts=[dict(phase='committed', mutation_type='directory_swap', active_path=str(selected), staging_path=str(selected.parent/'.stage'),backup_path=str(selected.parent/'.backup'),
                                                publication_parent_authority=self.parent_authority(selected.parent),client_binding_id='binding', after_digest='native-root-v1:'+'a'*64)])
        state_path=project/'.uap/state-v2.json';write(state_path,json.dumps(state).encode())
        return project, state_path, state, launcher, {'uap':dict(bytes=len(runtime),sha256=hashlib.sha256(runtime).hexdigest(),identity=list(CASE['stable_file'](launcher.stat())))}

    def test_public_data_locator_requires_the_production_project_namespace(self):
        import copy
        for effect in ("valid", "legacy", "foreign", "sibling", "marker"):
            with self.subTest(effect=effect), tempfile.TemporaryDirectory() as temporary:
                outer = Path(temporary).resolve()
                project, state_path, state, _, _ = self.fixture(outer)
                installed = state["installations"][0]
                binding = installed["clients"]["binding"]
                data = installed["data_receipts"][binding["data_receipt_id"]]
                expected = project / ".uap/plugin-data" / binding["physical_artifact_id"]
                self.assertEqual(data["locator"], str(expected))
                if effect in ("legacy", "foreign", "sibling"):
                    alternate = {
                        "legacy": project / ".uap/data" / binding["physical_artifact_id"],
                        "foreign": outer / "foreign-data",
                        "sibling": outer / "public-b/.uap/plugin-data" / binding["physical_artifact_id"],
                    }[effect]
                    alternate.parent.mkdir(parents=True, exist_ok=True)
                    expected.rename(alternate)
                    data["locator"] = str(alternate)
                    (alternate / ".agentplugins-data-owner.json").write_bytes(json.dumps(data).encode())
                    state_path.write_bytes(json.dumps(state).encode())
                    # Both sources agree on the wrong root; their agreement is
                    # not authority to follow it outside the selected namespace.
                    self.assertEqual(CASE["public_json"]((alternate / ".agentplugins-data-owner.json").read_bytes()), data)
                elif effect == "marker":
                    changed = copy.deepcopy(data)
                    changed["ownership_digest"] = "sha256:" + "0" * 64
                    (expected / ".agentplugins-data-owner.json").write_bytes(json.dumps(changed).encode())
                read = CASE["public_binding"].__globals__["ordinary_bytes"]
                with mock.patch.dict(CASE["public_binding"].__globals__, ordinary_bytes=mock.Mock(wraps=read)):
                    capture = CASE["public_binding"].__globals__["ordinary_bytes"]
                    if effect == "valid":
                        observed = CASE["public_binding"](project, "hermes-public-a")
                        self.assertEqual(observed["data"], expected)
                        self.assertEqual(observed["installed"]["data_receipts"]["data"], data)
                    else:
                        with self.assertRaisesRegex(RuntimeError, "public retained data receipt differs"):
                            CASE["public_binding"](project, "hermes-public-a")
                    if effect in ("legacy", "foreign", "sibling"):
                        self.assertNotIn(alternate / ".agentplugins-data-owner.json",
                                         [call.args[0] for call in capture.call_args_list])

    def test_receipt_owned_large_role_and_whole_package_join_are_both_required(self):
        import copy
        with tempfile.TemporaryDirectory() as temporary:
            outer=Path(temporary).resolve();project,state_path,state,launcher,pins=self.fixture(outer)
            with self.assertRaises(RuntimeError): CASE['inventory'](outer)
            image, proofs=CASE['public_snapshot'](outer,{'a':project},pins)
            self.assertEqual(image[launcher.relative_to(outer).as_posix()]['sha256'],pins['uap']['sha256'])
            self.assertEqual(proofs['a']['digest'],state['installations'][0]['clients']['binding']['receipts'][0]['after_digest'])
            for effect in ('target','package-digest','root','parent','data','pending','attempt-false','attempt-zero','attempt-list','duplicate-mcp','skill-path','skill-digest','source','unknown-large'):
                changed=copy.deepcopy(state);binding=changed['installations'][0]['clients']['binding'];extra=project/'unowned'
                if effect=='target':binding['target_locator']=str(outer/'foreign')
                elif effect=='package-digest':binding['native_objects'][0]['managed_digest']='sha256:'+'0'*64;binding['receipts'][0]['after_digest']='sha256:'+'0'*64
                elif effect=='root':changed['transaction_receipts'][0]['active_path']=str(outer/'foreign')
                elif effect=='parent':changed['transaction_receipts'][0].pop('publication_parent_authority')
                elif effect=='data':changed['installations'][0]['data_receipts']['data']['locator']=str(outer/'foreign')
                elif effect=='pending':binding['pending_native_intent']={}
                elif effect.startswith('attempt-'):binding['native_activation_attempt']={'attempt-false':False,'attempt-zero':0,'attempt-list':[]}[effect]
                elif effect=='duplicate-mcp':binding['native_objects'][-1]=binding['native_objects'][-2]
                elif effect=='skill-path':binding['native_objects'][2]['path']=str(outer/'foreign')
                elif effect=='skill-digest':binding['native_objects'][2]['managed_digest']='sha256:'+'0'*64
                elif effect=='source':binding['package_revision']['tree_digest']='sha256:'+'0'*64
                else:extra.write_bytes(b'x'*(CASE['LIMIT']+1))
                state_path.write_bytes(json.dumps(changed).encode())
                with self.subTest(effect=effect),self.assertRaises(RuntimeError):CASE['public_snapshot'](outer,{'a':project},pins)
                if extra.exists():extra.unlink()
            state_path.write_bytes(json.dumps(state).encode())
            runtime=launcher.read_bytes();launcher.write_bytes(b'X'+runtime[1:])
            with self.assertRaisesRegex(RuntimeError,'pinned runtime bytes differ'):CASE['public_snapshot'](outer,{'a':project},pins)

    def test_runtime_pins_stream_and_refuse_replacement_before_body_read(self):
        import hashlib
        with tempfile.TemporaryDirectory() as temporary:
            root=Path(temporary).resolve();env={}
            for role,path_key,hash_key in (('uap','UAP_BIN','UAP_HERMES_COLD_UAP_SHA256'),('cold','UAP_HERMES_COLD_TEST_BIN','UAP_HERMES_COLD_TEST_SHA256'),('probe','UAP_HERMES_MCP_PROBE_BIN','UAP_HERMES_MCP_PROBE_SHA256')):
                path=root/role;body=(role.encode()+b'x')*(1<<19);path.write_bytes(body)
                env[path_key]=str(path);env[hash_key]=hashlib.sha256(body).hexdigest()
            with mock.patch.dict(os.environ,env):
                pins=CASE['public_runtime_pins']();self.assertEqual(CASE['public_runtime_pins'](pins),pins)
                path=root/'uap';body=path.read_bytes();path.rename(root/'held');path.write_bytes(body)
                reader=CASE['public_runtime_pins'].__globals__
                with mock.patch.dict(reader,ordinary_bytes=mock.Mock(side_effect=AssertionError('replacement body read'))),self.assertRaisesRegex(RuntimeError,'original public runtime changed'):
                    CASE['public_runtime_pins'](pins)

    def test_named_large_pins_keep_alias_length_and_original_ordinary_bounds(self):
        import hashlib
        with tempfile.TemporaryDirectory() as temporary:
            root=Path(temporary).resolve();path=root/'binary';body=b'a'*(CASE['LIMIT']+1);path.write_bytes(body)
            pin=(len(body),hashlib.sha256(body).hexdigest())
            with self.assertRaises(RuntimeError):CASE['ordinary_bytes'](path)
            self.assertEqual(CASE['ordinary_bytes'](path,path.lstat(),pin=pin),pin[1])
            for changed in ((len(body)-1,pin[1]),(len(body),'0'*64),(True,pin[1])):
                with self.subTest(pin=changed),self.assertRaises(RuntimeError):CASE['ordinary_bytes'](path,path.lstat(),pin=changed)
            os.link(path,root/'alias')
            with self.assertRaises(RuntimeError):CASE['ordinary_bytes'](path,path.lstat(),pin=pin)

    def test_public_native_catalog_refuses_unknown_effects_and_unowned_backups(self):
        with tempfile.TemporaryDirectory() as temporary:
            outer=Path(temporary).resolve();root=outer/'public-a/.hermes/uap-session';root.mkdir(parents=True);root.chmod(0o700)
            (root/'.uap-owner').write_bytes(('uap-hermes-project-session-v1\n'+str(root.parent.parent)+'\n').encode());(root/'.uap-owner').chmod(0o600)
            image=CASE['inventory'](outer);CASE['public_profile'](root,image,outer,set())
            foreign=root/'unexpected';foreign.write_bytes(b'not an admitted bootstrap');foreign.chmod(0o600)
            with self.assertRaisesRegex(RuntimeError,'unmeasured'):CASE['public_profile'](root,CASE['inventory'](outer),outer,set())
            foreign.unlink();backup=root/'backups/config/config.yaml.good.20261113-235000';backup.parent.mkdir(parents=True);backup.write_bytes(b'foreign');backup.chmod(0o600)
            with self.assertRaisesRegex(RuntimeError,'unowned bytes'):CASE['public_profile'](root,CASE['inventory'](outer),outer,set())

    def test_removed_binding_keeps_exact_marker_under_the_original_retained_root(self):
        import shutil
        for effect in ('unchanged', 'altered', 'short', 'missing'):
            with self.subTest(effect=effect), tempfile.TemporaryDirectory() as temporary:
                outer=Path(temporary).resolve();project,state_path,state,launcher,pins=self.fixture(outer)
                _,before=CASE['public_snapshot'](outer,{'a':project},pins);proof=before['a'];selected=proof['selected']
                identities=(CASE['directory_identity'](selected),CASE['directory_identity'](selected.parent))
                state['installations'][0]['clients']={};state_path.write_bytes(json.dumps(state).encode())
                shutil.rmtree(proof['target'])
                marker=selected/'.uap-owner';original=marker.read_bytes()
                if effect=='altered':marker.write_bytes(b'X'+original[1:])
                elif effect=='short':marker.write_bytes(original[:-1])
                elif effect=='missing':marker.unlink()
                image,after=CASE['public_snapshot'](outer,{'a':project},pins)
                self.assertEqual(after,{'a':None})
                self.assertEqual(identities,(CASE['directory_identity'](selected),CASE['directory_identity'](selected.parent)))
                self.assertIn(proof['root_receipt'],CASE['public_json'](state_path.read_bytes())['transaction_receipts'])
                if effect=='unchanged':CASE['public_profile'](selected,image,outer,set())
                else:
                    with self.assertRaisesRegex(RuntimeError,'public retained root marker differs'):
                        CASE['public_profile'](selected,image,outer,set())

    def test_stdio_events_require_actual_pair_nonce_fresh_session_and_no_tool_claim(self):
        import copy
        with tempfile.TemporaryDirectory() as temporary:
            path=Path(temporary).resolve()/'events';nonce='c'*64
            rows=[dict(method=method,pid=42,session='42-123',nonce=nonce,timestamp='2026-11-13T23:50:00Z',facts=None) for method in ('initialize','tools/list')]
            def write(value):path.write_bytes(b''.join(json.dumps(row).encode()+b'\n' for row in value))
            write(rows);self.assertEqual(CASE['public_stdio_events'](path,nonce),rows)
            for effect in ('nonce','pid','bool-pid','method','duplicate','incomplete','extra'):
                value=copy.deepcopy(rows)
                if effect=='nonce':value[1]['nonce']='d'*64
                elif effect=='pid':value[1]['pid']=43
                elif effect=='bool-pid':value[0]['pid']=True
                elif effect=='method':value[1]['method']='tools/call'
                elif effect=='duplicate':value+=copy.deepcopy(rows)
                elif effect=='incomplete':value.pop()
                else:value[0]['unknown']=1
                write(value)
                with self.subTest(effect=effect),self.assertRaises(RuntimeError):CASE['public_stdio_events'](path,nonce)

    def cold(self, temporary):
        import base64
        import hashlib
        source=dict(resolved_revision='a'*40,tree_digest='sha256:'+'b'*64)
        first,last=temporary/'original/project',temporary/'cold/project'
        executable=str(temporary/'build/uap'); bundle=str(temporary/'payload/bin/hermes')
        server={'url':'http://127.0.0.1:9/mcp'}
        mcp_digest='sha256:'+hashlib.sha256(b'agentplugins-native-mcp-v1\0hermes-mcp-servers-yaml\0docs\0'+json.dumps(server,sort_keys=True,separators=(',',':')).encode()).hexdigest()
        binding=dict(client_id='hermes',client_binding_id='binding',scope='project',activation='prepared',verification='installation_verified',native_profile_root=str(last/'.hermes'),package_revision=source,
                     native_objects=[dict(kind='hermes_project_session_mcp',object_id='hermes_project_session_mcp:docs',logical_name='docs',path=str(last/'.hermes/uap-session/config.yaml'),protection_class='managed',managed_digest=mcp_digest)])
        receipt=dict(after_digest='native-root-v1:'+'c'*64,active_path=str(last/'.hermes/uap-session'),staging_path=str(last/'.hermes/.stage'),backup_path=str(last/'.hermes/.backup'),
                     publication_parent_authority=self.parent_authority(last/'.hermes'),phase='committed',client_binding_id='binding',mutation_type='directory_swap',sequence=1)
        commands=[]
        for step,project in zip(('original-inspect','cold-inspect','repaired-inspect'),(first,last,last)):
            row=dict(step=step,directory=str(project),argv=[bundle,'config','get','mcp_servers','--json'],
                     public_argv=['run','--target','hermes','--scope','project','--project-root',str(project),'--inspect','--confirm-client-start'],
                     selected_root=str(project/'.hermes/uap-session'),exit_code=0,cleanup_complete=True,no_child_started=False)
            for name,body in (('stdout',json.dumps({'docs':server},separators=(',',':')).encode()+b'\n'),('stderr',b'')):
                row.update({name+'_base64':base64.b64encode(body).decode(),name+'_bytes':len(body),name+'_sha256':hashlib.sha256(body).hexdigest()})
            commands.append(row)
        pins={'uap':dict(path=executable,sha256='d'*64)}
        report=dict(kind='hermes-bundle-cold-restore',entrypoint='public-cobra-in-process',installer_executed=False,source_head='e'*40,installer_sha256='d'*64,bundle_executable=bundle,runner_os={'linux':'linux','darwin':'darwin','win32':'windows'}.get(CASE['sys'].platform),architecture={'x86_64':'amd64','amd64':'amd64','arm64':'arm64','aarch64':'arm64'}.get(CASE['platform'].machine().lower()),original_source=dict(source),restored_source=dict(source),restored_binding=binding,commands=commands,root_publications=[receipt],removed_state=dict(transaction_receipts=[receipt],installations=[dict(clients={})]),activation='prepared',native_tools_call=False,vendor_authentication=False,model_invoked=False)
        return report,pins

    def test_cold_report_requires_three_genuine_cleanup_stream_and_retained_root_joins(self):
        import copy
        temporary=Path(tempfile.gettempdir()).resolve()/'cold-test'
        report,pins=self.cold(temporary)
        def read(value):return CASE['cold_report'](value,pins,Path(report['bundle_executable']),'e'*40,temporary)
        self.assertEqual(read(report),report)
        for row in report['commands']:row['stderr_base64']=None
        self.assertEqual(read(report),report)  # Go's original nil []byte spelling.
        for effect in ('source','pin','entrypoint','executed','cleanup','no-child','stream','nil-nonempty','nil-hash','nil-stdout','count','argv','public-argv','selected-root','missing-root','root','root-type','root-sequence','extra-root','root-removed','active','pending','attempt-false','attempt-zero','attempt-list','flag','unknown','same-environment'):
            value=copy.deepcopy(report)
            if effect=='source':value['restored_source']['resolved_revision']='f'*40
            elif effect=='pin':value['installer_sha256']='f'*64
            elif effect=='entrypoint':value['entrypoint']='compiled-uap-subprocess'
            elif effect=='executed':value['installer_executed']=True
            elif effect=='cleanup':value['commands'][1]['cleanup_complete']=False
            elif effect=='no-child':value['commands'][1]['no_child_started']=True
            elif effect=='stream':value['commands'][1]['stdout_sha256']='f'*64
            elif effect=='nil-nonempty':value['commands'][1]['stderr_bytes']=1
            elif effect=='nil-hash':value['commands'][1]['stderr_sha256']='f'*64
            elif effect=='nil-stdout':value['commands'][1]['stdout_base64']=None
            elif effect=='count':value['commands'].pop()
            elif effect=='argv':value['commands'][1]['argv'][-1]='--yes'
            elif effect=='public-argv':value['commands'][1]['public_argv'][-1]='--yes'
            elif effect=='selected-root':value['commands'][1]['selected_root']=str(temporary/'other')
            elif effect=='missing-root':value['commands'][1].pop('selected_root')
            elif effect=='root':value['root_publications'][0]['active_path']='/foreign'
            elif effect=='root-type':value['root_publications'][0]['mutation_type']='file_write'
            elif effect=='root-sequence':value['root_publications'][0]['sequence']=True
            elif effect=='extra-root':value['root_publications'].append({})
            elif effect=='root-removed':value['removed_state']['transaction_receipts']=[]
            elif effect=='active':value['restored_binding']['activation']='active'
            elif effect=='pending':value['restored_binding']['pending_native_intent']={}
            elif effect.startswith('attempt-'):value['restored_binding']['native_activation_attempt']={'attempt-false':False,'attempt-zero':0,'attempt-list':[]}[effect]
            elif effect=='flag':value['native_tools_call']=True
            elif effect=='same-environment':value['commands'][0]=copy.deepcopy(value['commands'][1]);value['commands'][0]['step']='original-inspect'
            else:value['unknown']=True
            with self.subTest(effect=effect),self.assertRaises(RuntimeError):read(value)

    def test_rehashed_cold_stdout_still_must_match_the_selected_native_mcp_receipt(self):
        import base64
        import copy
        import hashlib
        temporary=Path(tempfile.gettempdir()).resolve()/'cold-test'
        report,pins=self.cold(temporary)
        for effect in ('all-malformed','all-foreign','last-changed','owned-digest','owned-path','duplicate-owned'):
            value=copy.deepcopy(report)
            if effect in ('all-malformed','all-foreign','last-changed'):
                body={'all-malformed':b'not json','all-foreign':b'{"foreign":{}}','last-changed':b'{"docs":{"url":"http://127.0.0.1:10/wrong"}}'}[effect]
                rows=value['commands'] if effect!='last-changed' else value['commands'][-1:]
                for row in rows:row.update(stdout_base64=base64.b64encode(body).decode(),stdout_bytes=len(body),stdout_sha256=hashlib.sha256(body).hexdigest())
            elif effect=='owned-digest':value['restored_binding']['native_objects'][0]['managed_digest']='sha256:'+'0'*64
            elif effect=='owned-path':value['restored_binding']['native_objects'][0]['path']=str(temporary/'foreign/config.yaml')
            else:value['restored_binding']['native_objects']*=2
            with self.subTest(effect=effect),self.assertRaises((RuntimeError,ValueError)):
                CASE['cold_report'](value,pins,Path(report['bundle_executable']),'e'*40,temporary)

    def test_publication_parent_is_a_recorded_closed_role_not_new_cleanup_authority(self):
        import copy
        temporary=Path(tempfile.gettempdir()).resolve()/'cold-test'
        report,pins=self.cold(temporary)
        for effect in ('missing','opaque','bool-version','foreign-root','empty-ancestry','long-ancestry','last-root','bad-id','unknown','before','stage','backup'):
            value=copy.deepcopy(report);receipt=value['root_publications'][0];authority=receipt['publication_parent_authority']
            if effect=='missing':receipt.pop('publication_parent_authority')
            elif effect=='opaque':receipt['publication_parent_authority']='never-an-issued-token'
            elif effect=='bool-version':authority['version']=True
            elif effect=='foreign-root':authority['canonical_root']=str(temporary/'foreign')
            elif effect=='empty-ancestry':authority['ancestry']=[]
            elif effect=='long-ancestry':authority['ancestry']*=257
            elif effect=='last-root':authority['ancestry'][-1]['canonical_path']=str(temporary/'foreign')
            elif effect=='bad-id':authority['ancestry'][0]['object_id']=False
            elif effect=='unknown':authority['unknown']=True
            elif effect=='before':receipt['before_digest']='native-root-v1:'+'f'*64
            elif effect=='stage':receipt['staging_path']=str(temporary/'foreign-stage')
            else:receipt['backup_path']=str(temporary/'foreign-backup')
            with self.subTest(effect=effect),self.assertRaises(RuntimeError):
                CASE['cold_report'](value,pins,Path(report['bundle_executable']),'e'*40,temporary)

    def test_failed_public_family_cannot_authorize_home_cleanup_without_complete_cold_report(self):
        import copy
        with tempfile.TemporaryDirectory() as temporary:
            home=Path(temporary).resolve();proof={'source':{'commit':'a'*40}}
            self.assertTrue(CASE['public_cleanup_complete'](home,proof))
            (home/'public-a').mkdir();marker=home/'public-a/retained';marker.write_bytes(b'never remove unproved native home')
            self.assertFalse(CASE['public_cleanup_complete'](home,proof))
            proof.update(public_project_lifecycle_complete=True,cold_restore={'source':'a'*40,'commands':3,'sha256':'b'*64})
            self.assertTrue(CASE['public_cleanup_complete'](home,proof))
            for effect in ('missing','incomplete','count','bool-count','source','digest','extra'):
                changed=copy.deepcopy(proof)
                if effect=='missing':changed.pop('cold_restore')
                elif effect=='incomplete':changed['public_project_lifecycle_complete']=False
                elif effect=='count':changed['cold_restore']['commands']=2
                elif effect=='bool-count':changed['cold_restore']['commands']=True
                elif effect=='source':changed['cold_restore']['source']='c'*40
                elif effect=='digest':changed['cold_restore']['sha256']='unjoined'
                else:changed['cold_restore']['unknown']=True
                with self.subTest(effect=effect):self.assertFalse(CASE['public_cleanup_complete'](home,changed))
                self.assertEqual(marker.read_bytes(),b'never remove unproved native home')

    def test_compiled_cold_handoff_keeps_one_budget_private_paths_and_requires_executed_report(self):
        # The command callback is inert. This tests the collector boundary;
        # it never qualifies a hosted run, vendor executable or native cleanup.
        effects=['pass','skip','marker','missing-report','cleanup','budget','hosted','sibling','preexisting']
        if os.name!='nt':effects.append('replacement')  # Windows held directories prohibit this rename.
        for effect in effects:
            with self.subTest(effect=effect),tempfile.TemporaryDirectory() as temporary:
                outer=Path(temporary).resolve();private=outer/'private';private.mkdir();evidence=outer/'evidence';evidence.mkdir()
                owned=private/'hermes-cold';report,pins=self.cold(owned);pins['cold']={'path':str(outer/'build/cold')}
                env={'GITHUB_ACTIONS':'true','RUNNER_ENVIRONMENT':'github-hosted','UAP_BIN':pins['uap']['path']}
                if effect=='hosted':env['RUNNER_ENVIRONMENT']='self-hosted'
                if effect=='cleanup':report['commands'][1]['cleanup_complete']=False
                if effect=='preexisting':(private/'unknown').write_bytes(b'foreign')
                marker='HERMES_COLD_SOURCE '+('f' if effect=='marker' else 'e')*40+' '+'d'*64+' (3 native selected commands)\n'
                output=(marker+'--- '+('SKIP' if effect=='skip' else 'PASS')+': TestProjectHermesBundleColdRestoreAndRepair (1.00s)\nPASS\n').encode()
                def command(argv,cwd,child_env,prefix,*,timeout):
                    self.assertEqual(argv,[pins['cold']['path'],'-test.run=^TestProjectHermesBundleColdRestoreAndRepair$','-test.count=1','-test.v','-test.timeout=6m'])
                    self.assertEqual(cwd,CASE['HERE'].parents[1]);self.assertEqual(prefix,evidence/'public-cold')
                    self.assertEqual(timeout,70);self.assertEqual(child_env['UAP_HERMES_COLD_DEADLINE_UNIX_MS'],'2070000')
                    self.assertEqual([child_env[key] for key in ('TMPDIR','TEMP','TMP')],[str(owned)]*3)
                    self.assertTrue(owned.is_dir());self.assertFalse(list(owned.iterdir()))
                    (owned/'retained').write_bytes(b'owned Cold bytes')
                    self.assertEqual(child_env['UAP_HERMES_PROJECT_BUNDLE_E2E'],'1')
                    self.assertEqual(child_env['UAP_HERMES_PROJECT_BUNDLE_EXE'],report['bundle_executable'])
                    self.assertEqual(child_env['GITHUB_ACTIONS'],env['GITHUB_ACTIONS'])
                    if effect!='missing-report':Path(child_env['UAP_HERMES_COLD_REPORT']).write_bytes(json.dumps(report).encode())
                    if effect=='sibling':(private/'unexpected').write_bytes(b'never adopted')
                    if effect=='replacement':owned.rename(outer/'original');owned.mkdir();(owned/'replacement').write_bytes(b'never delete')
                    return output
                callback=mock.Mock(side_effect=command);namespace=CASE['public_cold'].__globals__
                with mock.patch.dict(namespace,public_runtime_pins=mock.Mock(return_value=pins),bounded=callback),mock.patch.object(CASE['time'],'monotonic',return_value=1000),mock.patch.object(CASE['time'],'time',return_value=2000):
                    invoke=lambda:CASE['public_cold'](evidence,private,env,pins,1050 if effect=='budget' else 1080,'e'*40,Path(report['bundle_executable']))
                    if effect=='pass':self.assertEqual(invoke(),report)
                    else:
                        with self.assertRaises((RuntimeError,FileNotFoundError)):invoke()
                if effect in ('budget','hosted','preexisting'):callback.assert_not_called()
                else:callback.assert_called_once()
                if effect=='pass':self.assertFalse(list(private.iterdir()))
                elif effect=='replacement':self.assertEqual((outer/'original/retained').read_bytes(),b'owned Cold bytes');self.assertTrue((owned/'replacement').exists())
                elif effect not in ('budget','hosted','preexisting'):self.assertEqual((owned/'retained').read_bytes(),b'owned Cold bytes')


class SourceRuntimeContracts(unittest.TestCase):
    """Real inert trees only; these receipts never qualify a native source run."""

    def test_source_version_joins_complete_original_readout_and_selected_path(self):
        import hashlib
        mac_source = "/Users/runner/work/_temp/hermes-agent"
        banner = "Hermes Agent v0.23.0 (2026.9.24) · stable · local 2bd378ed1b8d"
        notice = "Update available — run 'hermes update'"
        original = (banner + "\nInstall directory: " + mac_source
                    + "\nInstall method: git\nPython: 3.14.9\nOpenAI SDK: 2.54.0\n" + notice + "\n").encode()
        self.assertEqual(len(original), 217)
        self.assertEqual(hashlib.sha256(original).hexdigest(), "b4f09e15b3940ad8739fa1aa3bc99c802ab162f9112e798edf3557afffaf1911")
        self.assertNotEqual(original.strip(), banner.encode(), "the original disproves v2's one-line oracle")
        with tempfile.TemporaryDirectory() as temporary:
            source = Path(temporary) / "installed/hermes-agent"
            for platform, newline, original_source in (("posix", b"\n", mac_source), ("nt", b"\r\n", r"D:\a\_temp\hermes-agent")):
                captured = original.replace(mac_source.encode(), original_source.encode()).replace(b"\n", newline)
                if platform == "nt":
                    self.assertEqual(len(captured), 209)
                    self.assertEqual(hashlib.sha256(captured).hexdigest(), "da4ae42f21fc02f2f0967394bbfbaaa0f45cad7ec01272c5a9e8f46d8add0691")
                # Only the original runner's source path is substituted for this
                # portable fixture. The original output and hashes stay above.
                self.assertEqual(captured.count(original_source.encode()), 1)
                body = captured.replace(original_source.encode(), str(source).encode())
                with mock.patch.dict(CASE["source_version_readout"].__globals__, {"os": SimpleNamespace(name=platform)}):
                    CASE["source_version_readout"](captured, original_source)
                    CASE["source_version_readout"](body, source)
                    CASE["source_version_readout"](body.removesuffix(notice.encode() + newline), source)
                    refusals = {
                        "banner-only": banner.encode() + newline,
                        "missing-final-newline": body[:-len(newline)],
                        "different-version": body.replace(b"v0.23.0", b"v0.23.1"),
                        "different-commit": body.replace(b"2bd378ed1b8d", b"2bd378ed1b8e"),
                        "different-source": body.replace(str(source).encode(), str(source).encode() + b"-replacement"),
                        "different-method": body.replace(b"Install method: git", b"Install method: pip"),
                        "missing-python": body.replace(b"Python: 3.14.9", b"Python: Not installed"),
                        "incomplete-python": body.replace(b"Python: 3.14.9", b"Python: 3.14"),
                        "oversized-python": body.replace(b"Python: 3.14.9", b"Python: 3.140000.9"),
                        "missing-sdk": body.replace(b"OpenAI SDK: 2.54.0", b"OpenAI SDK: Not installed"),
                        "sdk-suffix": body.replace(b"OpenAI SDK: 2.54.0", b"OpenAI SDK: 2.54.0 unmeasured"),
                        "duplicate-field": body.replace(b"Install method: git", b"Install method: git" + newline + b"Install method: git"),
                        "unobserved-notice": body.replace(notice.encode(), b"Up to date"),
                        "trailing-line": body + b"unmeasured output" + newline,
                        "trailing-empty-line": body + newline,
                        "other-newline": body.replace(newline, b"\n" if platform == "nt" else b"\r\n"),
                    }
                    for name, refused in refusals.items():
                        with self.subTest(platform=platform, refusal=name), self.assertRaisesRegex(RuntimeError, "source.*version|source version"):
                            CASE["source_version_readout"](refused, source)

    def fixture(self, temporary):
        runner = Path(temporary).resolve()
        outer, user = runner / "hermes-native-home", runner / "hermes-native-home/user"
        original = user / ("AppData/Local/hermes" if os.name == "nt" else ".hermes")
        source = original / "hermes-agent"
        for path in (source, original / "tools", original / "installs"):
            path.mkdir(parents=True, exist_ok=True)
        (source / "owned.py").write_bytes(b"inert source")
        (original / "tools/python").write_bytes(b"inert interpreter; never executed")
        (original / "config.yaml").write_bytes(b"original config")
        (original / "auth.json").write_bytes(b"original auth marker")
        (user / "outside-pm.txt").write_bytes(b"complete User coverage")
        for role in ("a", "b"):
            root = outer / ("project-" + role) / ".uap/hermes-runtime"
            root.mkdir(parents=True)
            (root / "config.yaml").write_bytes(b"independent config")
        protected = [(original / name).relative_to(outer).as_posix() for name in ("config.yaml", "auth.json")]
        policy = CASE["source_layout"](outer, user, original, source, protected=protected)
        return runner, outer, user, original, source, policy

    def capture(self, outer, policy):
        import time
        return CASE["payload_snapshot"](outer, CASE["directory_identity"](outer), time.monotonic() + 10, source=policy)

    def uv_cache_fixture(self, temporary):
        runner, outer, _, original, _, policy = self.fixture(temporary)
        baseline = self.capture(outer, policy)
        selected = outer / policy["projects"][0]
        cache = selected / "cache/uv"
        member = cache / "archive-v0/owned/watchfiles/_rust_notify.abi3.so"
        member.parent.mkdir(parents=True)
        member.write_bytes(b"x" * 1160728)
        alias = selected / "installs/generation/venv/watchfiles/_rust_notify.abi3.so"
        alias.parent.mkdir(parents=True)
        os.link(member, alias)
        self.assertEqual(member.stat().st_nlink, 2)
        self.assertEqual(member.stat().st_ino, alias.stat().st_ino)
        return runner, outer, original, policy, cache, member, alias, baseline

    def test_source_uv_cache_role_keeps_complete_capture_and_content_digest(self):
        import hashlib
        import time
        with tempfile.TemporaryDirectory() as temporary:
            _, outer, _, policy, cache, member, alias, baseline = self.uv_cache_fixture(temporary)
            first = self.capture(outer, policy)
            prefix = cache.relative_to(outer).as_posix()
            records, rows = first["_records"], first["ledger"]
            self.assertEqual(rows[prefix]["type"], "runtime-tree")
            self.assertEqual(rows[prefix]["entries"], 5)
            self.assertEqual(rows[prefix]["logical_regular_bytes"], 1160728)
            self.assertEqual(rows[prefix]["links"], 0)
            self.assertEqual(rows[cache.parent.relative_to(outer).as_posix()], records[cache.parent.relative_to(outer).as_posix()])
            for path in (member, alias):
                row = records[path.relative_to(outer).as_posix()]
                self.assertEqual(row["identity"][3:5], (2, 1160728))
                self.assertEqual(row["sha256"], hashlib.sha256(b"x" * 1160728).hexdigest())
            self.assertEqual(sum(row["entries"] if row["type"] == "runtime-tree" else 1 for row in rows.values()), first["entries"])
            self.assertEqual(first["logical_regular_bytes"], sum(row["identity"][4] for row in records.values() if row["type"] == "file"))
            CASE["profile_effect_boundary"]("config-path-a", "a", baseline["ledger"], baseline["ledger"], rows,
                                             "user", policy["projects"][0], source_capture=True)
            sibling = policy["projects"][1]
            self.assertEqual({name: row for name, row in rows.items() if CASE["below"](name, sibling)},
                             {name: row for name, row in baseline["ledger"].items() if CASE["below"](name, sibling)})
            for name in policy["protected"]: self.assertEqual(rows[name], records[name])
            with mock.patch.object(os, "read", side_effect=AssertionError("ledger reread")), \
                    mock.patch.object(os, "open", side_effect=AssertionError("ledger reopen")), \
                    mock.patch.object(os, "scandir", side_effect=AssertionError("ledger rewalk")), \
                    mock.patch.object(Path, "lstat", side_effect=AssertionError("ledger restat")):
                self.assertEqual(CASE["source_ledger"](records, policy, time.monotonic() + 5), rows)
            member.write_bytes(b"y" * 1160728)
            second = self.capture(outer, policy)
            self.assertNotEqual(second["ledger"][prefix]["sha256"], rows[prefix]["sha256"])
            self.assertNotEqual(second["ledger"][policy["projects"][0] + "/installs"]["sha256"], rows[policy["projects"][0] + "/installs"]["sha256"])
            self.assertEqual(second["ledger"]["user"], rows["user"])
            for path in (member, alias):
                self.assertEqual(second["_records"][path.relative_to(outer).as_posix()]["sha256"], hashlib.sha256(b"y" * 1160728).hexdigest())

    def test_source_uv_cache_role_keeps_ordinary_siblings_and_protected_rows(self):
        for relative in ("cache/other/large", "cache/uv-other/large", "ordinary/large", "cache/uv/protected"):
            with self.subTest(relative=relative), tempfile.TemporaryDirectory() as temporary:
                _, outer, _, _, _, policy = self.fixture(temporary)
                member = outer / policy["projects"][0] / relative
                member.parent.mkdir(parents=True)
                member.write_bytes(b"x" * 1160728)
                name = member.relative_to(outer).as_posix()
                if relative == "cache/uv/protected": policy["protected"] = (*policy["protected"], name)
                with self.assertRaisesRegex(RuntimeError, "^source ordinary profile bound$") as refused:
                    self.capture(outer, policy)
                observed = refused.exception.source_payload_refusal
                self.assertEqual(observed["member"], name)
                self.assertEqual(observed["profile_bound"]["predicate"], "file-bytes")
                self.assertEqual(observed["metadata"]["bytes"], 1160728)
                member.write_bytes(b"ordinary retained bytes")
                captured = self.capture(outer, policy)
                self.assertEqual(captured["ledger"][name], captured["_records"][name])

    def test_source_uv_cache_links_keep_all_aliases_and_profile_boundaries(self):
        for other in ("outside", "user-auth", "project-b"):
            with self.subTest(other=other), tempfile.TemporaryDirectory() as temporary:
                runner, outer, original, policy, _, member, _, _ = self.uv_cache_fixture(temporary)
                if other == "user-auth":
                    member.unlink()
                    os.link(original / "auth.json", member)
                else:
                    target = runner / "outside" if other == "outside" else outer / policy["projects"][1] / "linked"
                    os.link(member, target)
                with mock.patch.object(os, "read", side_effect=AssertionError("bytes before cache alias authority")), \
                        self.assertRaisesRegex(RuntimeError, "hardlink crosses|hardlink escapes"):
                    self.capture(outer, policy)

    def test_source_uv_cache_capture_keeps_original_roots_and_membership(self):
        import time
        for change in ("root", "cache-parent", "added", "removed"):
            with self.subTest(change=change), tempfile.TemporaryDirectory() as temporary:
                runner, outer, _, policy, cache, member, _, _ = self.uv_cache_fixture(temporary)
                expected = CASE["directory_identity"](outer)
                original = CASE["payload_hash_files"]
                changed = []
                def move_directory(path):
                    moved = runner / "moved"
                    path.rename(moved); path.mkdir()
                    for child in moved.iterdir(): child.rename(path / child.name)
                def mutate(*args, **kwargs):
                    changed.append(change)
                    if change == "cache-parent": move_directory(cache)
                    elif change == "added": (cache / "unobserved").write_bytes(b"new member")
                    elif change == "removed": member.unlink()
                    return original(*args, **kwargs)
                if change == "root": move_directory(outer)
                with mock.patch.dict(CASE["payload_snapshot"].__globals__, {"payload_hash_files": mutate}), \
                        self.assertRaisesRegex(RuntimeError, "payload|inventory|directory"):
                    CASE["payload_snapshot"](outer, expected, time.monotonic() + 10, source=policy)
                self.assertEqual(changed, [] if change == "root" else [change])

    def test_source_empty_file_keeps_readable_handle_and_both_stat_fences(self):
        import hashlib
        with tempfile.TemporaryDirectory() as temporary:
            _, outer, _, original, _, policy = self.fixture(temporary)
            wanted = original / "installs/empty-ordinary-member"; wanted.write_bytes(b"")
            before, actual_read, actual_stat = wanted.stat(), os.read, os.fstat
            for capture_source in (policy, None):
                with self.subTest(source=capture_source is not None):
                    reads, held = [], []
                    error = PermissionError(13, "inert beyond-EOF byte lock")
                    def observed_stat(fd):
                        info = actual_stat(fd)
                        if (info.st_dev, info.st_ino) == (before.st_dev, before.st_ino): held.append(fd)
                        return info
                    def read(fd, count):
                        info = actual_stat(fd)
                        if (info.st_dev, info.st_ino) == (before.st_dev, before.st_ino):
                            reads.append(count)
                            raise error
                        return actual_read(fd, count)
                    with mock.patch.object(os, "read", side_effect=read), mock.patch.object(os, "fstat", side_effect=observed_stat):
                        if capture_source is None:
                            with self.assertRaises(PermissionError) as refused: self.capture(outer, None)
                            self.assertIs(refused.exception, error); self.assertEqual(reads, [1])
                        else:
                            captured = self.capture(outer, policy)["_records"][wanted.relative_to(outer).as_posix()]
                            self.assertEqual(captured["sha256"], hashlib.sha256(b"").hexdigest())
                            self.assertEqual(captured["identity"], CASE["stable_file"](before))
                            self.assertEqual(reads, []); self.assertEqual(len(held), 2)
                    self.assertTrue(held)
                    with self.assertRaises(OSError): actual_stat(held[-1])

    def test_source_empty_file_changes_keep_original_authority(self):
        for change in ("grow", "replace", "parent"):
            with self.subTest(change=change), tempfile.TemporaryDirectory() as temporary:
                runner, outer, _, original, _, policy = self.fixture(temporary)
                wanted = original / "installs/empty-ordinary-member"; wanted.write_bytes(b"")
                before, actual = wanted.stat(), os.fstat
                changed, denied = [], []
                moved = runner / "original-member"
                def mutate(fd):
                    info = actual(fd)
                    if not changed and (info.st_dev, info.st_ino) == (before.st_dev, before.st_ino):
                        changed.append(fd)
                        if change == "grow": wanted.write_bytes(b"new byte")
                        elif change == "replace":
                            wanted.rename(moved); wanted.write_bytes(b"")
                        else:
                            try: outer.rename(moved)
                            except OSError as error:
                                if os.name != "nt" or getattr(error, "winerror", None) != 32: raise
                                denied.append(error)
                                self.assertFalse(moved.exists())
                                self.assertEqual(CASE["stable_file"](wanted.stat()), CASE["stable_file"](before))
                            else: outer.mkdir()
                    return info
                with mock.patch.object(os, "fstat", side_effect=mutate):
                    if change == "parent" and os.name == "nt":
                        captured = self.capture(outer, policy)
                        self.assertEqual(len(denied), 1)
                        self.assertEqual(captured["_records"][wanted.relative_to(outer).as_posix()]["identity"], CASE["stable_file"](before))
                    else:
                        with self.assertRaisesRegex(RuntimeError, "payload|inventory|directory"):
                            self.capture(outer, policy)
                self.assertEqual(len(changed), 1)
                with self.assertRaises(OSError): actual(changed[0])
                if change == "replace":
                    self.assertEqual(wanted.read_bytes(), b""); self.assertEqual(moved.read_bytes(), b"")
                    self.assertEqual((moved.stat().st_dev, moved.stat().st_ino), (before.st_dev, before.st_ino))

    def test_source_empty_file_still_requires_successful_original_open(self):
        with tempfile.TemporaryDirectory() as temporary:
            _, outer, _, original, _, policy = self.fixture(temporary)
            wanted = original / "installs/empty-ordinary-member"; wanted.write_bytes(b"")
            before, calls = wanted.stat(), []
            error = PermissionError(13, "inert initial open refusal")
            actual_open, actual_member = os.open, CASE["artifacts"].open_member
            def checked(opened):
                def call(path, *args, **kwargs):
                    if Path(path).name == wanted.name:
                        calls.append(path)
                        raise error
                    return opened(path, *args, **kwargs)
                return call
            with mock.patch.object(os, "open", side_effect=checked(actual_open)), \
                    mock.patch.object(CASE["artifacts"], "open_member", side_effect=checked(actual_member)), \
                    self.assertRaises(PermissionError) as refused:
                self.capture(outer, policy)
            self.assertIs(refused.exception, error); self.assertEqual(len(calls), 1)
            self.assertEqual(CASE["stable_file"](wanted.stat()), CASE["stable_file"](before))

    @unittest.skipUnless(os.name == "nt", "actual Windows CRT byte-range lock")
    def test_windows_source_empty_lease_has_no_locked_byte_to_capture(self):
        import hashlib
        import msvcrt
        with tempfile.TemporaryDirectory() as temporary:
            _, outer, _, original, _, policy = self.fixture(temporary)
            lease = original / "installs/generation/.leases" / ("a" * 32)
            lease.parent.mkdir(parents=True)
            for body in (b"", b"x"):
                with self.subTest(bytes=len(body)):
                    owner = os.open(lease, os.O_CREAT | os.O_EXCL | os.O_RDWR | os.O_BINARY, 0o600)
                    try:
                        os.write(owner, body); os.lseek(owner, 0, os.SEEK_SET)
                        msvcrt.locking(owner, msvcrt.LK_NBLCK, 1)
                        reader = CASE["artifacts"].open_member(lease)
                        try:
                            self.assertEqual(os.fstat(reader).st_size, len(body))
                            with self.assertRaises(PermissionError) as refused: os.read(reader, 1)
                            self.assertEqual(refused.exception.errno, 13)
                        finally: os.close(reader)
                        if body:
                            with self.assertRaises(PermissionError): self.capture(outer, policy)
                        else:
                            captured = self.capture(outer, policy)["_records"][lease.relative_to(outer).as_posix()]
                            self.assertEqual(captured["sha256"], hashlib.sha256(b"").hexdigest())
                            self.assertEqual(captured["identity"][4], 0)
                    finally:
                        os.close(owner); lease.unlink()

    def test_source_read_refusal_preserves_original_error_offset_and_closed_fd(self):
        import time
        with tempfile.TemporaryDirectory() as temporary:
            _, outer, _, _, source, policy = self.fixture(temporary)
            wanted = source / "owned.py"; before = wanted.stat()
            for capture_source in (policy, None):
                with self.subTest(source=capture_source is not None):
                    error, reads, original = PermissionError(13, "inert read refusal"), [], os.read
                    def fail_read(fd, count):
                        info = os.fstat(fd)
                        if (info.st_dev, info.st_ino) != (before.st_dev, before.st_ino):
                            return original(fd, count)
                        reads.append(fd)
                        if len(reads) == 1:
                            return original(fd, 3)
                        raise error
                    with mock.patch.object(os, "read", side_effect=fail_read), self.assertRaises(PermissionError) as refused:
                        CASE["payload_snapshot"](outer, CASE["directory_identity"](outer), time.monotonic() + 10, source=capture_source)
                    self.assertIs(refused.exception, error); self.assertEqual(error.errno, 13)
                    self.assertEqual(len(reads), 2)
                    with self.assertRaises(OSError): os.fstat(reads[-1])
                    observed = getattr(error, "source_payload_refusal", None)
                    if capture_source is None:
                        self.assertIsNone(observed)
                    else:
                        self.assertIsNotNone(observed, "original source read failure lacks member metadata")
                        self.assertEqual((observed["member"], observed["bytes_read"], observed["errno"]),
                                         (wanted.relative_to(outer).as_posix(), 3, 13))
                        self.assertEqual(observed["metadata"]["bytes"], before.st_size)

    def test_source_absolute_link_refusal_names_the_intermediate_captured_member(self):
        import time
        with tempfile.TemporaryDirectory() as temporary:
            _, outer, _, _, _, policy = self.fixture(temporary)
            captured = self.capture(outer, policy)["_records"]
            origin, nested = policy["tools"] + "/origin", policy["tools"] + "/nested"
            captured[origin] = {"type": "link", "target": "nested", "identity": captured[policy["tools"] + "/python"]["identity"]}
            captured[nested] = {**captured[origin], "target": str(outer.parent / "PRIVATE-TARGET")}
            with mock.patch.object(os, "read", side_effect=AssertionError("outside body read")), \
                    mock.patch.object(os, "open", side_effect=AssertionError("outside target opened")), \
                    mock.patch.object(Path, "resolve", side_effect=AssertionError("outside target traversed")), \
                    self.assertRaisesRegex(RuntimeError, "^payload link escapes runtime$") as refused:
                CASE["payload_links"](outer, captured, time.monotonic() + 5, source=policy)
            observed = getattr(refused.exception, "source_payload_refusal", None)
            self.assertIsNotNone(observed, "original source link failure lacks member metadata")
            self.assertEqual(observed["member"], nested)
            self.assertEqual(observed["operation"], "absolute-link-target")
            self.assertNotIn("PRIVATE-TARGET", json.dumps(observed))

    def refusal(self, root, member="user/runtime/member", *, identity=None):
        leaf = root / "private-body"; leaf.write_bytes(b"NEVER-PUBLISH-THIS-BODY")
        error = PermissionError(13, "original refusal")
        CASE["source_payload_refusal"](error, member, {"identity": identity or CASE["stable_file"](leaf.stat())}, bytes_read=0)
        return error

    def test_source_ordinary_bound_keeps_exact_first_observation_without_io(self):
        import copy
        import stat
        import time
        policy = dict(user="user", source="user/.hermes/hermes-agent", tools="user/.hermes/tools",
                      installs="user/.hermes/installs", projects=["project-a/.uap/hermes-runtime",
                      "project-b/.uap/hermes-runtime"], protected=())
        for scope, prefix in (("selected-a", policy["projects"][0]), ("selected-b", policy["projects"][1]),
                              ("original-user", "user"), ("fixture-other", "z-fixture")):
            for predicate, count, kind in (("file-bytes", 1, "file"), ("count", 2049, "directory"),
                                            ("count-and-file-bytes", 2049, "file"), ("count", 2049, "link")):
                with self.subTest(scope=scope, predicate=predicate, kind=kind):
                    name = prefix + "/first-refused"
                    mode = {"file": stat.S_IFREG, "directory": stat.S_IFDIR, "link": stat.S_IFLNK}[kind] | 0o600
                    size = CASE["LIMIT"] + 1 if "file-bytes" in predicate else 0
                    item = dict(type=kind, identity=(1, 2, mode, 1, size, 10, 10, 1000, 1000))
                    records = {f"fixture/{index:04}": dict(type="file", identity=(1, 10 + index, stat.S_IFREG | 0o600, 1, 0, 10, 10, 1000, 1000))
                               for index in range(count - 1)}
                    records[name] = item
                    before = copy.deepcopy(records)
                    selected = dict(policy, protected=(name,))
                    with mock.patch.object(os, "read", side_effect=AssertionError("diagnostic read")), \
                            mock.patch.object(os, "open", side_effect=AssertionError("diagnostic open")), \
                            mock.patch.object(os, "scandir", side_effect=AssertionError("diagnostic walk")), \
                            mock.patch.object(Path, "lstat", side_effect=AssertionError("diagnostic restat")), \
                            self.assertRaisesRegex(RuntimeError, "^source ordinary profile bound$") as refused:
                        CASE["source_ledger"](records, selected, time.monotonic() + 5)
                    self.assertEqual(records, before)
                    value = dict(refused.exception.source_payload_refusal, nonce="a" * 64, stage="observer-profile-snapshot")
                    self.assertEqual(value["operation"], "ordinary-profile-bound")
                    self.assertEqual(value["member"], name)
                    self.assertEqual(value["profile_bound"], dict(scope=scope, kind=kind, ordinary_count=count, predicate=predicate))
                    self.assertEqual(value["metadata"]["bytes"], size)
                    self.assertEqual(CASE["source_refusal_document"](json.dumps(value).encode()), value)
        with self.assertRaisesRegex(RuntimeError, "^payload scan deadline$") as expired:
            CASE["source_ledger"](records, selected, time.monotonic() - 1)
        self.assertFalse(hasattr(expired.exception, "source_payload_refusal"))

    def test_source_ordinary_bound_schema_refuses_unjoined_or_false_predicates(self):
        import copy
        import stat
        original = RuntimeError("source ordinary profile bound")
        record = dict(type="file", identity=(1, 2, stat.S_IFREG | 0o600, 1, CASE["LIMIT"] + 1, 10, 10, 1000, 1000))
        bound = dict(scope="selected-a", kind="file", ordinary_count=1, predicate="file-bytes")
        CASE["source_payload_refusal"](original, "project-a/.uap/hermes-runtime/large", record, profile_bound=bound)
        valid = dict(original.source_payload_refusal, nonce="a" * 64, stage="observer-profile-snapshot")
        for change in ("scope", "kind", "count-bool", "count-zero", "count-big", "count-predicate", "size-predicate",
                       "unknown", "unknown-bound", "missing-bound", "read", "target", "errno", "mode", "bytes", "qualified", "oversized"):
            with self.subTest(change=change):
                value = copy.deepcopy(valid); observed = value["profile_bound"]
                if change == "scope": observed["scope"] = "selected-b"
                elif change == "kind": observed["kind"] = "directory"
                elif change == "count-bool": observed["ordinary_count"] = True
                elif change == "count-zero": observed["ordinary_count"] = 0
                elif change == "count-big": observed["ordinary_count"] = CASE["PAYLOAD_MEMBERS"] + 1
                elif change == "count-predicate": observed["predicate"] = "count"
                elif change == "size-predicate": value["metadata"]["bytes"] = CASE["LIMIT"]
                elif change == "unknown": value["private"] = "forbidden"
                elif change == "unknown-bound": observed["private"] = "forbidden"
                elif change == "missing-bound": value.pop("profile_bound")
                elif change == "read": value["bytes_read"] = 0
                elif change == "target": value["target_sha256"] = "b" * 64
                elif change == "errno": value["errno"] = 13
                elif change == "mode": value["metadata"]["mode"] = stat.S_IFDIR | 0o700
                elif change == "bytes": value["metadata"]["bytes"] = CASE["PAYLOAD_BYTES"] + 1
                elif change == "qualified": value["admission_qualified"] = True
                body = b" " * 2049 if change == "oversized" else json.dumps(value).encode()
                with self.assertRaises(RuntimeError): CASE["source_refusal_document"](body)
        for operation in ("file-read", "absolute-link-target"):
            value = dict(valid, operation=operation)
            with self.assertRaisesRegex(RuntimeError, "closed source refusal schema"):
                CASE["source_refusal_document"](json.dumps(value).encode())

    def test_source_ordinary_bound_retains_first_proof_without_replacing_failure(self):
        import stat
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve(); evidence = root / "evidence"; evidence.mkdir()
            expected = CASE["directory_identity"](evidence)
            original = RuntimeError("source ordinary profile bound")
            item = dict(type="file", identity=(1, 2, stat.S_IFREG | 0o600, 1, CASE["LIMIT"] + 1, 10, 10, 1000, 1000))
            bound = dict(scope="selected-a", kind="file", ordinary_count=1, predicate="file-bytes")
            CASE["source_payload_refusal"](original, "project-a/.uap/hermes-runtime/large", item, profile_bound=bound)
            retain = lambda: CASE["retain_source_refusal"](original, evidence, expected, "a" * 64, "observer-profile-snapshot")
            with self.assertRaises(RuntimeError) as refused:
                try: raise original
                except RuntimeError:
                    retain()
                    raise
            self.assertIs(refused.exception, original)
            member = evidence / "source-payload-refusal.json"; first = member.read_bytes()
            bound["scope"] = "selected-b"
            CASE["source_payload_refusal"](original, "project-b/.uap/hermes-runtime/other", item, profile_bound=bound)
            retain(); self.assertEqual(member.read_bytes(), first)
            evidence.rename(root / "retained"); evidence.mkdir()
            with mock.patch.dict(CASE["retain_source_refusal"].__globals__, write_census_member=mock.Mock()) as state:
                retain(); state["write_census_member"].assert_not_called()
            self.assertFalse(list(evidence.iterdir()))
            flags = ("capture_complete", "admission_qualified", "production_project_mcp", "vendor_authentication",
                     "real_model_inference", "tools_call", "source_install_custom_root_qualified", "source_bootstrap_recovery_qualified")
            proof = dict.fromkeys(flags, False); proof.update(nonce="a" * 64, runtime_distribution="source", status="failed")
            CASE["write_json"](root / "retained/profile-admission.json", proof)
            with mock.patch.dict(CASE["stage_evidence"].__globals__, EVIDENCE_CLOSED=False):
                CASE["stage_evidence"](root / "retained")
            self.assertEqual((root / "retained/upload/source-payload-refusal.json").read_bytes(), first)

    def test_source_refusal_has_bounded_names_integers_and_no_additional_reads(self):
        import hashlib
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve(); error = self.refusal(root)
            identity = CASE["stable_file"]((root / "private-body").stat())
            for member in ("user/é.py", "user/" + chr(0xDCFF), "user/" + "x" * 600):
                with mock.patch.object(os, "read", side_effect=AssertionError("diagnostic read")), \
                        mock.patch.object(os, "open", side_effect=AssertionError("diagnostic open")), \
                        mock.patch.object(Path, "lstat", side_effect=AssertionError("diagnostic restat")):
                    CASE["source_payload_refusal"](error, member, {"identity": identity}, bytes_read=0)
                value = dict(error.source_payload_refusal, nonce="a" * 64, stage="parent-before-observer")
                encoded = json.dumps(value).encode()
                self.assertLessEqual(len(encoded), 2048); CASE["source_refusal_document"](encoded)
                self.assertEqual(value["member"], member if len(json.dumps(member).encode()) <= 512 else None)
                self.assertEqual(value["member_sha256"], hashlib.sha256(member.encode("utf-8", "surrogatepass")).hexdigest())
            large = list(identity); large[1] = 1 << 20000
            observed = self.refusal(root, identity=tuple(large)).source_payload_refusal
            self.assertIsNone(observed["metadata"]["inode"]); self.assertTrue(observed["integer_saturated"])
            for member in ("/outside", "../outside", "C:/outside", "bad\\name"):
                self.assertFalse(hasattr(self.refusal(root, member), "source_payload_refusal"))

    def test_source_refusal_publication_keeps_first_error_and_original_directory_authority(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve(); evidence = root / "evidence"; evidence.mkdir()
            error = self.refusal(root); expected = CASE["directory_identity"](evidence)
            retain = lambda: CASE["retain_source_refusal"](error, evidence, expected, "a" * 64, "observer-profile-initial")
            retain(); member = evidence / "source-payload-refusal.json"; first = member.read_bytes()
            error.source_payload_refusal["errno"] = 5
            retain(); self.assertEqual(member.read_bytes(), first)
            member.unlink()
            with mock.patch.dict(CASE["retain_source_refusal"].__globals__, {"write_census_member": mock.Mock(side_effect=OSError("inert writer failed"))}):
                with self.assertRaises(PermissionError) as refused:
                    try: raise error
                    except PermissionError:
                        retain()
                        raise
                self.assertIs(refused.exception, error); self.assertFalse(member.exists())
            evidence.rename(root / "original-evidence"); evidence.mkdir()
            with mock.patch.dict(CASE["retain_source_refusal"].__globals__, {"write_census_member": mock.Mock()}) as scope:
                retain(); scope["write_census_member"].assert_not_called()
            self.assertFalse(member.exists()); self.assertFalse(list((root / "original-evidence").iterdir()))
            with mock.patch.dict(os.environ, {"HERMES_PROFILE_SOURCE_CAPTURE": "1"}):
                inert = CASE["safe_failure_evidence"](root, "process-cleanup-unproved")
            self.assertEqual({path.name for path in (inert / "upload").iterdir()}, {"profile-admission.json"})

    def test_source_refusal_stage_joins_copied_failed_source_proof_and_never_partial_upload(self):
        flags = ("capture_complete", "admission_qualified", "production_project_mcp", "vendor_authentication", "real_model_inference",
                 "tools_call", "source_install_custom_root_qualified", "source_bootstrap_recovery_qualified")
        for change in ("valid", "nonce", "bundled", "success", "unknown", "duplicate", "boolean", "stage", "operation",
                       "metadata-extra", "negative", "offset", "target", "bound", "second-read", *flags):
            with self.subTest(change=change), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary).resolve(); evidence = root / "evidence"; evidence.mkdir()
                error = self.refusal(root)
                CASE["retain_source_refusal"](error, evidence, CASE["directory_identity"](evidence), "a" * 64, "parent-before-observer")
                member = evidence / "source-payload-refusal.json"; value = json.loads(member.read_bytes())
                proof = dict.fromkeys(flags, False)
                proof.update(nonce="a" * 64, runtime_distribution="source", status="failed")
                if change == "nonce": proof["nonce"] = "b" * 64
                if change == "bundled": proof["runtime_distribution"] = "bundled"
                if change == "success": proof["status"] = "passed"
                if change in flags: proof[change] = True
                if change == "unknown": value["private"] = "forbidden"
                if change == "boolean": value["metadata"]["inode"] = True
                if change == "stage": value["stage"] = "unreviewed"
                if change == "operation": value["operation"] = "open"
                if change == "metadata-extra": value["metadata"]["secret"] = "forbidden"
                if change == "negative": value["metadata"]["links"] = -1
                if change == "offset": value["bytes_read"] = value["metadata"]["bytes"] + 1
                if change == "target": value["target_sha256"] = "b" * 64
                body = json.dumps(value).encode()
                if change == "duplicate": body = body[:-1] + b',"nonce":"' + b"a" * 64 + b'"}'
                if change == "bound": body = b" " * 2049
                member.write_bytes(body); CASE["write_json"](evidence / "profile-admission.json", proof)
                actual, reads = CASE["ordinary_bytes"], []
                def read(path, expected=None):
                    reads.append(path.name)
                    if change == "second-read" and reads.count(member.name) == 2:
                        raise RuntimeError("late diagnostic read refused")
                    return actual(path, expected)
                with mock.patch.dict(CASE["stage_evidence"].__globals__, {"ordinary_bytes": read, "EVIDENCE_CLOSED": False}) as scope:
                    if change == "valid":
                        CASE["stage_evidence"](evidence)
                        self.assertEqual((evidence / "upload" / member.name).read_bytes(), body)
                        self.assertEqual(reads.count("profile-admission.json"), 1, "nonce join reopened the proof")
                        self.assertTrue(scope["EVIDENCE_CLOSED"])
                    else:
                        with self.assertRaises(RuntimeError): CASE["stage_evidence"](evidence)
                        self.assertFalse(scope["EVIDENCE_CLOSED"]); self.assertFalse((evidence / "upload").exists())
                        self.assertFalse(list(evidence.glob(".upload-pending-*")))
                    if change == "second-read": self.assertEqual(reads.count(member.name), 2)

    def test_partial_source_refusal_write_retains_original_cause_and_withholds_upload(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve(); evidence = root / "evidence"; evidence.mkdir()
            original = self.refusal(root)
            def partial(path, value, parent=None):
                with path.open("xb") as stream: stream.write(b"{")
                raise OSError("inert partial writer refusal")
            with mock.patch.dict(CASE["retain_source_refusal"].__globals__, write_census_member=partial):
                CASE["retain_source_refusal"](original, evidence, CASE["directory_identity"](evidence), "a" * 64, "observer-profile-initial")
            with self.assertRaisesRegex(RuntimeError, "original Hermes refusal retained") as refused:
                CASE["admit_evidence"](evidence, failure=original)
            self.assertIs(refused.exception.__cause__, original)
            with self.assertRaises(json.JSONDecodeError): CASE["admit_evidence"](evidence)
            self.assertEqual((evidence / "source-payload-refusal.json").read_bytes(), b"{")
            self.assertFalse((evidence / "upload").exists()); self.assertFalse(list(evidence.glob(".upload-pending-*")))

    def test_full_partition_covers_user_outside_pm_and_exact_protected_rows(self):
        with tempfile.TemporaryDirectory() as temporary:
            _, outer, user, original, source, policy = self.fixture(temporary)
            (source / "large-owned").write_bytes(b"x" * ((1 << 20) + 1))
            first = self.capture(outer, policy)
            rows = first["ledger"]
            self.assertEqual(sum(row["entries"] if row["type"] == "runtime-tree" else 1 for row in rows.values()), first["entries"])
            self.assertEqual(rows[policy["protected"][0]], first["_records"][policy["protected"][0]])
            (user / "outside-pm.txt").write_bytes(b"same authority, changed bytes")
            second = self.capture(outer, policy)
            self.assertNotEqual(rows["user"], second["ledger"]["user"])
            with self.assertRaisesRegex(RuntimeError, "Project command changed original User"):
                CASE["profile_effect_boundary"]("config-path-a", "a", rows, rows, second["ledger"], "user", policy["projects"][0], source_capture=True)
            self.assertEqual(rows[policy["source"]], second["ledger"][policy["source"]])
            self.assertEqual((original / "auth.json").read_bytes(), b"original auth marker")

    def test_source_link_graph_rejects_sibling_and_user_even_inside_captured_outer(self):
        import time
        with tempfile.TemporaryDirectory() as temporary:
            _, outer, _, _, _, policy = self.fixture(temporary)
            captured = self.capture(outer, policy)["_records"]
            member = policy["projects"][0] + "/python"
            for target, valid in ((policy["tools"] + "/python", True),
                                  (policy["projects"][0] + "/config.yaml", True),
                                  (policy["projects"][1] + "/config.yaml", False),
                                  (policy["protected"][0], False)):
                records = {**captured, member: {"type": "link", "target": str(outer / target)}}
                with self.subTest(target=target), mock.patch.object(Path, "resolve", side_effect=AssertionError("filesystem link traversed")):
                    if valid:
                        CASE["payload_links"](outer, records, time.monotonic() + 5, source=policy)
                    else:
                        with self.assertRaisesRegex(RuntimeError, "crosses runtime authority"):
                            CASE["payload_links"](outer, records, time.monotonic() + 5, source=policy)
            # A final allowed target cannot launder an intermediate B link.
            bridge = policy["projects"][1] + "/bridge"
            records = {**captured, bridge: {"type": "link", "target": str(outer / policy["tools"] / "python")},
                       member: {"type": "link", "target": str(outer / bridge)}}
            with self.assertRaisesRegex(RuntimeError, "crosses runtime authority"):
                CASE["payload_links"](outer, records, time.monotonic() + 5, source=policy)

    def test_complete_hardlink_set_cannot_join_project_b_or_user_auth(self):
        for other in ("b", "auth", "outside"):
            with self.subTest(other=other), tempfile.TemporaryDirectory() as temporary:
                runner, outer, _, original, _, policy = self.fixture(temporary)
                first = outer / policy["projects"][0] / "shared"
                target = (outer / policy["projects"][1] / "shared" if other == "b" else
                          original / "auth.json" if other == "auth" else runner / "outside")
                if other != "auth": target.write_bytes(b"must not be read")
                os.link(target, first)
                with mock.patch.object(os, "read", side_effect=AssertionError("bytes before alias authority")):
                    with self.assertRaisesRegex(RuntimeError, "hardlink crosses|hardlink escapes"):
                        self.capture(outer, policy)

    def test_windows_source_graph_uses_unique_captured_spelling_for_authority(self):
        import time
        from pathlib import PureWindowsPath
        policy = {"user": "user", "source": "user/native/source", "tools": "user/native/tools",
                  "projects": ["project-a/root", "project-b/root"]}
        records = {name: {"type": "directory"} for name in
                   (".", "user", "user/native", "user/native/source", "user/native/tools", "project-a", "project-a/root")}
        records.update({"user/native/tools/python": {"type": "file"}, "user/native/config.yaml": {"type": "file"},
                        "project-a/root/python": {"type": "link", "target": "C:/PAYLOAD/USER/NATIVE/TOOLS/PYTHON"}})
        with mock.patch.dict(CASE["payload_links"].__globals__, {"Path": PureWindowsPath}):
            CASE["payload_links"](PureWindowsPath("c:/payload"), records, time.monotonic() + 5, source=policy)
            records["project-a/root/python"]["target"] = "C:/PAYLOAD/USER/NATIVE/CONFIG.YAML"
            with self.assertRaisesRegex(RuntimeError, "crosses runtime authority"):
                CASE["payload_links"](PureWindowsPath("c:/payload"), records, time.monotonic() + 5, source=policy)
            records["USER/NATIVE/TOOLS/PYTHON"] = {"type": "file"}
            with self.assertRaisesRegex(RuntimeError, "ambiguous payload path"):
                CASE["payload_links"](PureWindowsPath("c:/payload"), records, time.monotonic() + 5, source=policy)

    def test_project_copy_of_user_sentinel_across_read_chunks_is_refused(self):
        with tempfile.TemporaryDirectory() as temporary:
            _, outer, _, _, _, policy = self.fixture(temporary)
            marker = b"USER_AUTH_" + b"7" * 64
            policy["sentinels"] = (marker,)
            target = outer / policy["projects"][0] / "installs/generation"
            target.mkdir(parents=True)
            (target / "copied").write_bytes(b"x" * ((1 << 20) - 5) + marker)
            with self.assertRaisesRegex(RuntimeError, "User sentinel copied"):
                self.capture(outer, policy)

    def test_source_user_control_has_no_new_baseline_for_unexpected_native_effect(self):
        with tempfile.TemporaryDirectory() as temporary:
            _, outer, user, _, _, policy = self.fixture(temporary)
            before = self.capture(outer, policy)["ledger"]
            CASE["profile_effect_boundary"]("list-user", "user", before, before, before, "user", "user", source_capture=True)
            (user / "unexpected-native-log").write_bytes(b"measured but unqualified")
            after = self.capture(outer, policy)["ledger"]
            self.assertIn("user", CASE["changes"](before, after))
            with self.assertRaisesRegex(RuntimeError, "source User control changed"):
                CASE["profile_effect_boundary"]("list-user", "user", before, before, after, "user", "user", source_capture=True)
            with self.assertRaisesRegex(RuntimeError, "original User changed before command"):
                CASE["profile_effect_boundary"]("list-user", "user", before, after, after, "user", "user", source_capture=True)

    def test_source_fixture_retains_installer_config_and_rejects_foreign_collision(self):
        with tempfile.TemporaryDirectory() as temporary:
            original = Path(temporary).resolve()
            (original / "config.yaml").write_bytes(b"official installer preimage")
            (original / ".env").write_bytes(b"inert installer environment")
            inode = (original / "config.yaml").stat().st_ino
            retained = CASE["prepare_user_fixture"](original, {"config.yaml": "sentinel\n", ".env": "# inert\n"}, source_capture=True)
            self.assertEqual((original / retained["config.yaml"]["member"]).read_bytes(), b"official installer preimage")
            self.assertEqual((original / retained["config.yaml"]["member"]).stat().st_ino, inode)
            self.assertEqual((original / "config.yaml").read_bytes(), b"sentinel\n")
            (original / "auth.json").write_bytes(b"must remain untouched")
            with self.assertRaisesRegex(RuntimeError, "fixture collision"):
                CASE["prepare_user_fixture"](original, {"auth.json": "replacement"}, source_capture=True)
            self.assertEqual((original / "auth.json").read_bytes(), b"must remain untouched")

    def test_source_safe_failure_is_separate_and_all_qualifications_remain_false(self):
        with tempfile.TemporaryDirectory() as temporary, mock.patch.dict(os.environ, {"HERMES_PROFILE_SOURCE_CAPTURE": "1"}):
            directory = CASE["safe_failure_evidence"](Path(temporary).resolve(), "inert-source-refusal")
            self.assertTrue(directory.name.startswith("hermes-profile-source-evidence-"))
            files = list((directory / "upload").iterdir())
            self.assertEqual([path.name for path in files], ["profile-admission.json"])
            proof = json.loads(files[0].read_text())
            self.assertEqual(proof["runtime_distribution"], "source")
            for name in ("admission_qualified", "production_project_mcp", "source_install_custom_root_qualified",
                         "source_bootstrap_recovery_qualified", "vendor_authentication", "real_model_inference", "tools_call"):
                self.assertIs(proof[name], False)

    def installed_fixture(self, temporary):
        import hashlib
        runner, outer, user, original, source, _ = self.fixture(temporary)
        native = user / "bin/hermes"; native.parent.mkdir(); native.write_bytes(b"inert launcher; never executed")
        python = original / "tools/managed-python" / ("python.exe" if os.name == "nt" else "bin/python3")
        python.parent.mkdir(parents=True); python.write_bytes(b"inert store Python; never executed" + b"x" * (1 << 20))
        (original / "tools/facts.json").write_text(json.dumps({"packages": {"python": {"entry": "managed-python"}}}))
        key = hashlib.sha256(str(source).encode()).hexdigest()[:16]
        generation = original / "installs" / key / "environments/selected"
        generation.mkdir(parents=True); (generation / "pyvenv.cfg").write_bytes(b"inert generation")
        facts = generation.parent.parent / "facts.json"
        facts.write_text(json.dumps({"packages": {"venv": {"environment": str(generation)}}}))
        command = runner / "runtime-command.json"
        command.write_text(json.dumps([str(python), "-I", "-c", "inert code; never executed"]))
        env = {"HOME": str(user), "USERPROFILE": str(user), "HERMES_HOME": str(original),
               "HERMES_BIN": str(native), "HERMES_NATIVE_RUNTIME_COMMAND_FILE": str(command)}
        return runner, outer, original, source, facts, env

    def test_source_inputs_join_exact_installer_owner_facts_and_generation(self):
        with tempfile.TemporaryDirectory() as temporary:
            runner, outer, original, source, facts, env = self.installed_fixture(temporary)
            identity = CASE["directory_identity"](outer)
            with mock.patch.dict(os.environ, env):
                inputs = CASE["source_inputs"](runner, source)
                snapshot = CASE["source_runtime_snapshot"](inputs, identity)
                self.assertEqual(snapshot["provenance"]["source"], str(source))
                self.assertEqual(snapshot, CASE["source_runtime_snapshot"](inputs, identity))
                self.assertNotIn("_records", json.dumps(snapshot))
                facts.write_text(json.dumps({"packages": {"venv": {"environment": str(source)}}}))
                with self.assertRaisesRegex(RuntimeError, "generation escaped owner"):
                    CASE["source_runtime_snapshot"](inputs, identity)
            with mock.patch.dict(os.environ, {**env, "HOME": str(runner)}), self.assertRaisesRegex(RuntimeError, "preserved installed User"):
                CASE["source_inputs"](runner, source)
            with mock.patch.dict(os.environ, {**env, "HERMES_BIN": str(runner / "outside")}), self.assertRaisesRegex(RuntimeError, "launcher/interpreter escaped"):
                CASE["source_inputs"](runner, source)
            with mock.patch.dict(os.environ, env):
                inputs = CASE["source_inputs"](runner, source)
                (original / "tools/facts.json").write_text(json.dumps({"packages": {"python": {"entry": "unrecorded"}}}))
                with self.assertRaisesRegex(RuntimeError, "interpreter does not match"):
                    CASE["source_runtime_snapshot"](inputs, identity)

    def test_source_facts_are_rejoined_to_capture_before_decoding(self):
        with tempfile.TemporaryDirectory() as temporary:
            runner, outer, _, source, facts, env = self.installed_fixture(temporary)
            capture = CASE["payload_snapshot"]
            with mock.patch.dict(os.environ, env):
                inputs = CASE["source_inputs"](runner, source)
                identity = CASE["directory_identity"](outer)
                def replace(*args, **kwargs):
                    result = capture(*args, **kwargs)
                    previous = facts.read_bytes(); facts.rename(facts.with_name("retained")); facts.write_bytes(previous)
                    return result
                with mock.patch.dict(CASE["source_runtime_snapshot"].__globals__, {"payload_snapshot": replace}):
                    with self.assertRaisesRegex(RuntimeError, "source input changed after capture"):
                        CASE["source_runtime_snapshot"](inputs, identity)
                outer.rename(runner / "moved"); outer.mkdir()
                with mock.patch.object(os, "scandir", side_effect=AssertionError("replacement visited")):
                    with self.assertRaisesRegex(RuntimeError, "original root changed"):
                        CASE["source_runtime_snapshot"](inputs, identity)

    def test_source_parent_preparation_requires_exact_original_user_relation(self):
        import hashlib
        for effect in ("valid", "outside", "source", "directory", "extra"):
            with self.subTest(effect=effect), tempfile.TemporaryDirectory() as temporary:
                runner, outer, original, source, _, env = self.installed_fixture(temporary)
                (original / "auth.json").unlink()  # The real no-auth installer has no authored auth fixture.
                evidence = runner / "evidence"; evidence.mkdir()
                env.update(RUNNER_TEMP=str(runner), UAP_BIN=str(runner / "unused-uap"), HERMES_NATIVE_SOURCE=str(source))
                with mock.patch.dict(os.environ, env):
                    inputs, records = CASE["source_inputs"](runner, source), {}
                    authority = CASE["directory_identity"](outer)
                    initial = CASE["source_runtime_snapshot"](inputs, authority, retain=records)
                    closed = CASE["BASE"]["closed_environment"]
                    def preparation(*args, **kwargs):
                        result = closed(*args, **kwargs)
                        if effect == "outside": (outer / "user/outside-pm.txt").write_bytes(b"unrelated preparation write")
                        if effect == "source": (source / "owned.py").write_bytes(b"changed source")
                        if effect == "directory":
                            changed = outer / "user"; info = changed.stat()
                            os.utime(changed, ns=(info.st_atime_ns, info.st_mtime_ns + 2_000_000_000))
                            changed.chmod(0o700 if os.name != "nt" else 0o555)
                        if effect == "extra": (original / "undeclared").write_bytes(b"unexpected")
                        return result
                    with mock.patch.dict(CASE["BASE"], {"closed_environment": preparation}):
                        if effect != "valid":
                            with self.assertRaisesRegex(RuntimeError, "preparation changed|preparation replaced"):
                                CASE["prepare_source_user"](inputs, records, evidence, "6" * 64, authority)
                            self.assertFalse((evidence / "source-user-preparation.json").exists())
                            continue
                        child_env, prepared, receipt = CASE["prepare_source_user"](inputs, records, evidence, "6" * 64, authority)
                    self.assertEqual(prepared["immutable"], initial["immutable"])
                    self.assertNotEqual(prepared["user"], initial["user"])
                    self.assertEqual(receipt["original_user"], initial["user"])
                    self.assertEqual(receipt["prepared_user"], prepared["user"])
                    document = evidence / "source-user-preparation.json"
                    digest = hashlib.sha256(document.read_bytes()).hexdigest()
                    self.assertEqual(child_env["HERMES_SOURCE_PREPARATION_SHA256"], digest)
                    read = CASE["read_source_preparation"](evidence, CASE["directory_identity"](evidence), "6" * 64, digest)
                    self.assertEqual(read, json.loads(json.dumps(receipt)))
                    CASE["admit_evidence"](evidence)
                    self.assertEqual(child_env["HOME"], env["HOME"])
                    (outer / "user/outside-pm.txt").write_bytes(b"observer bootstrap effect")
                    after = CASE["source_runtime_snapshot"](inputs, authority)
                    with self.assertRaisesRegex(RuntimeError, "original source User changed"):
                        CASE["require_source_user_baseline"](after["user"], prepared["user"])

    def test_source_preparation_artifact_refuses_raw_or_undeclared_metadata(self):
        with tempfile.TemporaryDirectory() as temporary:
            runner, outer, original, source, _, env = self.installed_fixture(temporary)
            (original / "auth.json").unlink()
            evidence = runner / "evidence"; evidence.mkdir()
            env.update(RUNNER_TEMP=str(runner), UAP_BIN=str(runner / "unused-uap"), HERMES_NATIVE_SOURCE=str(source))
            with mock.patch.dict(os.environ, env):
                inputs, records = CASE["source_inputs"](runner, source), {}
                authority = CASE["directory_identity"](outer)
                CASE["source_runtime_snapshot"](inputs, authority, retain=records)
                CASE["prepare_source_user"](inputs, records, evidence, "7" * 64, authority)
            document = evidence / "source-user-preparation.json"
            original_body = document.read_bytes()
            for effect in ("body", "name", "preimage", "identity", "boolean", "duplicate"):
                with self.subTest(effect=effect):
                    value = json.loads(original_body)
                    if effect == "body": value["installer_preimages"]["config.yaml"]["content"] = "User body must never be exported"
                    if effect == "name": value["declared_directories"].append("user/unmeasured-private-name")
                    if effect == "preimage": value["installer_preimages"]["config.yaml"]["member"] = "outside/config.yaml"
                    if effect == "identity": value["installer_preimages"]["config.yaml"]["retained_identity"][1] += 1
                    if effect == "boolean": value["schema_version"] = True
                    body = json.dumps(value).encode()
                    if effect == "duplicate": body = b'{"nonce":"' + b"8" * 64 + b'",' + body[1:]
                    document.write_bytes(body)
                    with self.assertRaisesRegex(RuntimeError, "source preparation|duplicate census"):
                        CASE["stage_evidence"](evidence)
                    self.assertFalse((evidence / "upload").exists())
            document.write_bytes(original_body)
            CASE["admit_evidence"](evidence)

    def test_source_observer_cannot_choose_a_user_baseline_after_bootstrap(self):
        import hashlib
        import shutil
        import sys
        with tempfile.TemporaryDirectory() as temporary:
            runner, outer, original, source, _, env = self.installed_fixture(temporary)
            for role in ("a", "b"): shutil.rmtree(outer / ("project-" + role))
            (original / "auth.json").unlink()
            evidence = runner / "evidence"; evidence.mkdir()
            nonce = "9" * 64
            env.update(RUNNER_TEMP=str(runner), UAP_BIN=str(runner / "unused-uap"), HERMES_NATIVE_SOURCE=str(source))
            with mock.patch.dict(os.environ, env):
                inputs, records = CASE["source_inputs"](runner, source), {}
                authority = CASE["directory_identity"](outer)
                CASE["source_runtime_snapshot"](inputs, authority, retain=records)
                child, _, _ = CASE["prepare_source_user"](inputs, records, evidence, nonce, authority)
            child.update(HERMES_PROFILE_SOURCE_CAPTURE="1", HERMES_HOME=str(original),
                         HERMES_CENSUS_EVIDENCE_ID=json.dumps(CASE["directory_identity"](evidence)),
                         HERMES_SOURCE_PREPARATION_SHA256=hashlib.sha256((evidence / "source-user-preparation.json").read_bytes()).hexdigest())
            original_config = (original / "config.yaml").read_bytes()
            (outer / "user/outside-pm.txt").write_bytes(b"unexpected observer bootstrap before first command")
            calls = []
            def git_only(argv, *_args, **_kwargs):
                calls.append(argv)
                self.assertEqual(argv[0], "git", "native command started before original User join")
                return b""
            modules = {"hermes_constants": SimpleNamespace(get_default_hermes_root=lambda **_: original),
                       "hermes_yaml": SimpleNamespace(safe_dump=json.dumps, safe_load=json.loads)}
            probes = {role: SimpleNamespace(url="http://127.0.0.1:19/" + role) for role in ("a", "b")}
            with mock.patch.dict(os.environ, child, clear=True), mock.patch.dict(sys.modules, modules), \
                    mock.patch.dict(CASE["observe_profiles"].__globals__, {"bounded": git_only}):
                with self.assertRaisesRegex(RuntimeError, "original source User changed"):
                    CASE["observe_profiles"](probes)
            self.assertEqual(len(calls), 2)
            self.assertEqual((original / "config.yaml").read_bytes(), original_config)

    def test_source_snapshot_rejects_moved_outer_with_original_user_and_leaf(self):
        for boundary in ("payload_snapshot", "source_user_digest"):
            with self.subTest(boundary=boundary), tempfile.TemporaryDirectory() as temporary:
                runner, outer, _, source, facts, env = self.installed_fixture(temporary)
                with mock.patch.dict(os.environ, env):
                    inputs = CASE["source_inputs"](runner, source)
                    expected = CASE["directory_identity"](outer)
                    operation = CASE[boundary]
                    same_leaf = facts.stat()
                    def moved(*args, **kwargs):
                        result = operation(*args, **kwargs)
                        retained = runner / "retained-outer"
                        outer.rename(retained); outer.mkdir(); (retained / "user").rename(outer / "user")
                        self.assertTrue(os.path.samestat(same_leaf, facts.stat()))
                        return result
                    with mock.patch.dict(CASE["source_runtime_snapshot"].__globals__, {boundary: moved}), \
                            mock.patch.dict(CASE["source_runtime_snapshot"].__globals__, {"ordinary_bytes": mock.Mock(side_effect=AssertionError("provenance reread"))}):
                        with self.assertRaisesRegex(RuntimeError, "original census evidence changed"):
                            CASE["source_runtime_snapshot"](inputs, expected)

    def delta(self, before, after, *, deadline=None):
        import time
        summaries = [CASE["source_user_digest"](rows, "user") for rows in (before, after)]
        return CASE["source_user_delta"](before, after, "user", "d" * 64, *summaries,
                                         time.monotonic() + 10 if deadline is None else deadline)

    def test_settled_source_delta_uses_only_captured_user_records_without_adopting_them(self):
        with tempfile.TemporaryDirectory() as temporary:
            _, outer, user, _, source, policy = self.fixture(temporary)
            before = self.capture(outer, policy)["_records"]
            (source / "owned.py").write_bytes(b"changed source bytes must not be published")
            (user / "outside-pm.txt").unlink(); (user / "added").write_bytes(b"added private bytes")
            info = user.stat(); os.utime(user, ns=(info.st_atime_ns, info.st_mtime_ns + 2_000_000_000))
            (outer / policy["projects"][0] / "config.yaml").write_bytes(b"independent Project change")
            after = self.capture(outer, policy)["_records"]
            captured = json.dumps([before, after], sort_keys=True)
            with mock.patch.object(os, "read", side_effect=AssertionError("delta read")), \
                    mock.patch.object(os, "open", side_effect=AssertionError("delta open")), \
                    mock.patch.object(Path, "lstat", side_effect=AssertionError("delta stat")):
                value = self.delta(before, after)
            self.assertEqual(json.dumps([before, after], sort_keys=True), captured)
            rows = {row["member"]: row for row in value["changes"]}
            self.assertEqual(set(rows), {"user", "user/added", "user/outside-pm.txt", policy["source"] + "/owned.py"})
            self.assertEqual((value["total_changes"], value["truncated"]), (4, False))
            self.assertIsNone(rows["user/added"]["before"]); self.assertIsNone(rows["user/outside-pm.txt"]["after"])
            self.assertNotEqual(rows["user"]["before"]["metadata"]["mtime_ns"], rows["user"]["after"]["metadata"]["mtime_ns"])
            self.assertNotIn("private bytes", json.dumps(value)); self.assertNotIn("must not be published", json.dumps(value))
            with self.assertRaisesRegex(RuntimeError, "original source User changed"):
                CASE["require_source_user_baseline"](value["after"], value["before"])
            self.assertEqual(self.delta(after, after)["total_changes"], 0)

    def test_source_delta_retains_prepared_records_only_after_complete_preparation(self):
        for effect in ("valid", "write-refusal", "capture-refusal"):
            with self.subTest(effect=effect), tempfile.TemporaryDirectory() as temporary:
                runner, outer, original, source, _, env = self.installed_fixture(temporary)
                (original / "auth.json").unlink(); evidence = runner / "evidence"; evidence.mkdir()
                env.update(RUNNER_TEMP=str(runner), UAP_BIN=str(runner / "unused-uap"), HERMES_NATIVE_SOURCE=str(source))
                with mock.patch.dict(os.environ, env):
                    inputs, records, retained = CASE["source_inputs"](runner, source), {}, {}
                    authority = CASE["directory_identity"](outer)
                    CASE["source_runtime_snapshot"](inputs, authority, retain=records)
                    capture, write = CASE["source_runtime_snapshot"], CASE["write_json"]
                    def capture_then_refuse(*args, **kwargs):
                        value = capture(*args, **kwargs)
                        if effect == "capture-refusal": raise RuntimeError("complete private map without accepted return")
                        return value
                    def write_or_refuse(*args, **kwargs):
                        if effect == "write-refusal": raise OSError("receipt write refused")
                        return write(*args, **kwargs)
                    with mock.patch.dict(CASE["prepare_source_user"].__globals__, source_runtime_snapshot=capture_then_refuse, write_json=write_or_refuse):
                        if effect == "valid":
                            _, prepared, receipt = CASE["prepare_source_user"](inputs, records, evidence, "d" * 64, authority, retain=retained)
                            self.assertEqual(CASE["source_user_digest"](retained, "user"), prepared["user"])
                            self.assertEqual(prepared["user"], receipt["prepared_user"])
                        else:
                            with self.assertRaises((OSError, RuntimeError)):
                                CASE["prepare_source_user"](inputs, records, evidence, "d" * 64, authority, retain=retained)
                            self.assertEqual(retained, {})

    def test_source_delta_bounds_and_no_incomplete_or_unsettled_publication(self):
        import time
        with tempfile.TemporaryDirectory() as temporary:
            runner, outer, user, _, _, policy = self.fixture(temporary)
            before = self.capture(outer, policy)["_records"]
            for index in range(10): (user / ("added-" + str(index))).write_bytes(b"inert")
            after = self.capture(outer, policy)["_records"]
            value = self.delta(before, after)
            self.assertEqual((value["total_changes"], len(value["changes"]), value["truncated"]), (11, 8, True))
            self.assertLessEqual(len(json.dumps(value).encode()), 16 << 10)
            evidence = runner / "evidence"; evidence.mkdir(); original = PermissionError(13, "retained original")
            args = [evidence, CASE["directory_identity"](evidence), "d" * 64, {"user": value["before"]}, {"user": value["after"]}, before, after, "user", time.monotonic() + 10]
            for effect in ("no-failure", "unsettled", "missing-prepared", "unreturned-after", "partial-after", "expired"):
                call = list(args); failure, settled = original, True
                if effect == "no-failure": failure = None
                if effect == "unsettled": settled = False
                if effect == "missing-prepared": call[3] = None
                if effect == "unreturned-after": call[4] = None
                if effect == "partial-after": call[6] = {}
                if effect == "expired": call[-1] = time.monotonic() - 1
                with self.subTest(effect=effect), mock.patch.dict(CASE["retain_source_user_delta"].__globals__, write_census_member=mock.Mock()) as scope:
                    CASE["retain_source_user_delta"](*call, failure=failure, settled=settled)
                    scope["write_census_member"].assert_not_called()
            with self.assertRaisesRegex(RuntimeError, "deadline"):
                self.delta(before, after, deadline=time.monotonic() - 1)
            def partial(path, value, parent=None):
                with path.open("xb") as stream: stream.write(b"{")
                raise OSError("partial delta writer")
            with mock.patch.dict(CASE["retain_source_user_delta"].__globals__, write_census_member=partial):
                CASE["retain_source_user_delta"](*args, failure=original, settled=True)
            with self.assertRaisesRegex(RuntimeError, "original Hermes refusal retained") as refused:
                CASE["admit_evidence"](evidence, failure=original)
            self.assertIs(refused.exception.__cause__, original)
            self.assertEqual((evidence / "source-user-delta.json").read_bytes(), b"{")
            self.assertFalse((evidence / "upload").exists())

    def test_source_delta_projects_original_windows_identity_and_linux_link_without_target_text(self):
        import hashlib
        # Actual metadata from source2b artifacts14089868523 (Windows) and14091083977 (Linux).
        windows = {"type": "file", "identity": (14296250764777768553, 844424930198189, 33279, 1, 115233,
                   1794618254176314200, 1794618254176314200, 1794618254176314200, 32, 0, 0, 0),
                   "sha256": "ffd5b5c5d96b036614ef5306402007d11908aadb68d491e30bb0dcb26ddef757"}
        value = CASE["source_delta_record"](windows)
        self.assertEqual([value["metadata"][key] for key in ("birthtime_ns", "attributes", "reparse_tag", "uid", "gid")],
                         [1794618254176314200, 32, 0, 0, 0])
        self.assertFalse(value["integer_saturated"])
        large = {**windows, "identity": (windows["identity"][0], 1 << 65, *windows["identity"][2:])}
        value = CASE["source_delta_record"](large)
        self.assertTrue(value["integer_saturated"]); self.assertIsNone(value["metadata"]["inode"])
        changed = {**large, "identity": (large["identity"][0], (1 << 65) + 1, *large["identity"][2:])}
        packet = self.delta({"user/file": large}, {"user/file": changed})
        self.assertEqual(packet["changes"][0]["before"], packet["changes"][0]["after"])
        self.assertEqual(packet["total_changes"], 1)  # The bounded projection cannot retain either wide inode.
        value = CASE["source_delta_record"]({"type": "link", "identity": (66305, 9452821, 41471, 1, 10,
                  1794619614716712820, 1794619614716712820, 1001, 1001), "target": "python3.14"})
        self.assertIsNone(value["sha256"]); self.assertIsNone(value["metadata"]["birthtime_ns"])
        self.assertEqual(value["target_sha256"], hashlib.sha256(b"python3.14").hexdigest())
        self.assertNotIn("python3.14", json.dumps(value))

    def test_source_delta_closed_stage_joins_both_copied_user_summaries(self):
        import copy
        import hashlib
        flags = ("capture_complete", "admission_qualified", "production_project_mcp", "vendor_authentication", "real_model_inference",
                 "tools_call", "source_install_custom_root_qualified", "source_bootstrap_recovery_qualified")
        with tempfile.TemporaryDirectory() as temporary:
            runner, outer, user, _, _, policy = self.fixture(temporary)
            before = self.capture(outer, policy)["_records"]; (user / "added").write_bytes(b"inert")
            packet = self.delta(before, self.capture(outer, policy)["_records"])
            for change in ("valid", "nonce", "before", "after", "bundled", "success", "qualified", "unknown", "boolean", "stat-bool", "stat-bound", "long-member", "truncated", "identical", "duplicate", "extra-row", "bound", "second-read"):
                with self.subTest(change=change):
                    evidence = runner / change; evidence.mkdir(); value = copy.deepcopy(packet)
                    proof = dict.fromkeys(flags, False)
                    proof.update(nonce="d" * 64, runtime_distribution="source", status="failed",
                                 source_prepared_runtime={"user": packet["before"]}, runtime_after={"user": packet["after"]})
                    if change == "nonce": value["nonce"] = "a" * 64
                    if change in ("before", "after"): value[change]["sha256"] = "a" * 64
                    if change == "bundled": proof["runtime_distribution"] = "bundled"
                    if change == "success": proof["status"] = "passed"
                    if change == "qualified": proof["admission_qualified"] = True
                    if change == "unknown": value["secret"] = "forbidden"
                    if change == "boolean": value["total_changes"] = True
                    if change == "stat-bool": value["changes"][0]["after"]["metadata"]["inode"] = True
                    if change == "stat-bound": value["changes"][0]["after"]["metadata"]["ctime_ns"] = 1 << 65
                    if change == "long-member":
                        value["changes"][0]["member"] = "user/" + "x" * 512
                        value["changes"][0]["member_sha256"] = hashlib.sha256(value["changes"][0]["member"].encode()).hexdigest()
                    if change == "truncated": value["truncated"] = True
                    if change == "identical": value["changes"][0]["after"] = copy.deepcopy(value["changes"][0]["before"])
                    if change == "duplicate": value["changes"][1] = value["changes"][0]
                    if change == "extra-row":
                        value["changes"] = [copy.deepcopy(value["changes"][0]) for _ in range(9)]
                        for index, row in enumerate(value["changes"]):
                            row["member"] = "user/member-" + str(index); row["member_sha256"] = hashlib.sha256(row["member"].encode()).hexdigest()
                        value.update(total_changes=9, truncated=False)
                    body = json.dumps(value).encode() if change != "bound" else b" " * ((16 << 10) + 1)
                    member = evidence / "source-user-delta.json"; member.write_bytes(body)
                    CASE["write_json"](evidence / "profile-admission.json", proof)
                    actual, reads = CASE["ordinary_bytes"], []
                    def read(path, expected=None):
                        reads.append(path.name)
                        if change == "second-read" and reads.count(member.name) == 2: raise RuntimeError("late delta read refused")
                        return actual(path, expected)
                    with mock.patch.dict(CASE["stage_evidence"].__globals__, ordinary_bytes=read, EVIDENCE_CLOSED=False) as scope:
                        if change == "valid":
                            CASE["stage_evidence"](evidence)
                            self.assertEqual((evidence / "upload" / member.name).read_bytes(), body)
                            self.assertEqual(reads.count("profile-admission.json"), 1)
                        else:
                            with self.assertRaises(RuntimeError): CASE["stage_evidence"](evidence)
                            self.assertFalse(scope["EVIDENCE_CLOSED"]); self.assertFalse((evidence / "upload").exists())
                            self.assertFalse(list(evidence.glob(".upload-pending-*")))


class SourceObserverContracts(unittest.TestCase):
    def fixture(self, temporary, *, link=False, execute=False):
        import time
        runner, outer, original, source, facts, env = SourceRuntimeContracts().installed_fixture(temporary)
        environment = Path(json.loads(facts.read_bytes())["packages"]["venv"]["environment"])
        interpreter = environment / ("Scripts/python.exe" if os.name == "nt" else "bin/python")
        if execute:
            import venv
            # A stdlib venv only; no pip, vendor import, native Hermes, or hosted spoof.
            venv.EnvBuilder(with_pip=False, symlinks=False).create(environment)
        else:
            interpreter.parent.mkdir()
            if link: interpreter.symlink_to(original / "tools/managed-python/bin/python3")
            else: interpreter.write_bytes(b"inert selected interpreter; never executed")
        api_source = source / "pm/environments.py"; api_source.parent.mkdir()
        suffix = "Scripts/python.exe" if os.name == "nt" else "bin/python"
        api_body = ("def venv_command(project_root, environment, options=()):\n"
                    f"    prefix = [str(environment / {suffix!r}), *options]\n"
                    "    return rewrite(prefix) if rewrite is not None else prefix\n")
        api_source.write_text(api_body)
        scope = {"__name__": "pm.environments", "__file__": str(api_source), "rewrite": None}
        exec(compile(api_body, str(api_source), "exec"), scope)
        (source / "hermes_bootstrap.py").write_text("raise RuntimeError('observer unexpectedly activated Hermes bootstrap')\n")
        with mock.patch.dict(os.environ, env):
            inputs = CASE["source_inputs"](runner, source)
            authority, records = CASE["directory_identity"](outer), {}
            snapshot = CASE["source_runtime_snapshot"](inputs, authority, retain=records)
        return dict(api=scope["venv_command"], inputs=inputs, snapshot=snapshot, records=records, authority=authority,
                    code="assert 'hermes_bootstrap' not in sys.modules; sys.stdout.buffer.write(b'observer-only\\n')", args=("--child",), deadline=time.monotonic() + 10), scope

    def test_exact_selected_interpreter_runs_observer_without_creating_bootstrap_lease(self):
        import subprocess
        with tempfile.TemporaryDirectory() as temporary:
            args, _ = self.fixture(temporary, execute=True)
            before = args["snapshot"]
            argv, proof = CASE["source_observer_runtime_command"](**args)
            self.assertEqual(argv[1:4], ["-I", "-B", "-c"])
            self.assertEqual(argv[-1], "--child")
            self.assertEqual(proof["selected_environment"], before["provenance"]["selected_environment"])
            self.assertIs(proof["admission_qualified"], False)
            done = subprocess.run(argv, stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=5, check=True)
            self.assertEqual(done.stdout, b"observer-only\n"); self.assertEqual(done.stderr, b"")
            self.assertEqual(CASE["source_runtime_snapshot"](args["inputs"], args["authority"]), before)
            self.assertFalse(list(args["inputs"]["original"].rglob(".leases")))
            self.assertEqual(args["inputs"]["runtime_argv"][1:3], ["-I", "-c"])  # Native launcher receipt was not replaced.

    def test_wrong_generation_wrapper_flag_or_api_owner_refuses_before_observer_launch(self):
        import time
        for effect in ("generation", "store", "flag", "wrapper", "payload", "tuple", "element", "api-file", "api-module", "expired"):
            with self.subTest(effect=effect), tempfile.TemporaryDirectory() as temporary:
                args, scope = self.fixture(temporary)
                def rewrite(prefix):
                    if effect == "generation": return [str(Path(prefix[0]).parent.parent.with_name("other") / Path(prefix[0]).parent.name / Path(prefix[0]).name), *prefix[1:]]
                    if effect == "store": return [str(args["inputs"]["python"]), *prefix[1:]]
                    if effect == "flag": return [prefix[0], "-B", "-I"]
                    if effect == "wrapper": return ["wrapper", *prefix]
                    if effect == "payload": return [*prefix, "-S", "_venv_entry.py"]
                    if effect == "tuple": return tuple(prefix)
                    if effect == "element": return [Path(prefix[0]), *prefix[1:]]
                    return prefix
                scope["rewrite"] = rewrite
                if effect == "api-file": scope["__file__"] = str(args["inputs"]["source"] / "foreign.py")
                if effect == "api-module": scope["__name__"] = "foreign.module"
                if effect == "expired": args["deadline"] = time.monotonic() - 1
                with mock.patch.object(CASE["subprocess"], "Popen", side_effect=AssertionError("must not start refused observer")), self.assertRaises(RuntimeError):
                    CASE["source_observer_runtime_command"](**args)

    def test_held_capture_rejects_replaced_generation_ancestor_and_changed_original_bytes(self):
        for effect in ("ancestor", "interpreter", "config", "facts", "api", "digest"):
            with self.subTest(effect=effect), tempfile.TemporaryDirectory() as temporary:
                args, scope = self.fixture(temporary)
                environment = Path(args["snapshot"]["provenance"]["selected_environment"])
                executable = environment / ("Scripts/python.exe" if os.name == "nt" else "bin/python")
                selected = {"interpreter": executable, "config": environment / "pyvenv.cfg", "facts": args["inputs"]["facts"],
                            "api": args["inputs"]["source"] / "pm/environments.py"}
                def change(prefix):
                    if effect == "ancestor":
                        parent = environment.parent; moved = parent.with_name("moved-environments")
                        parent.rename(moved); parent.mkdir(); (moved / environment.name).rename(environment)
                    elif effect != "digest":
                        path = selected[effect]; body = path.read_bytes()
                        path.write_bytes(bytes([body[0] ^ 1]) + body[1:])
                    return prefix
                scope["rewrite"] = change
                if effect == "digest":
                    args["records"][executable.relative_to(args["inputs"]["home"]).as_posix()]["sha256"] = "0" * 64
                with self.assertRaisesRegex(RuntimeError, "source input changed|source captured|pinned runtime bytes"):
                    CASE["source_observer_runtime_command"](**args)

    @unittest.skipIf(os.name == "nt", "POSIX source interpreter symlink shape")
    def test_captured_link_target_is_rejoined_without_following_a_foreign_substitution(self):
        for effect in ("valid", "changed", "outside"):
            with self.subTest(effect=effect), tempfile.TemporaryDirectory() as temporary:
                args, scope = self.fixture(temporary, link=True)
                environment = Path(args["snapshot"]["provenance"]["selected_environment"])
                executable = environment / "bin/python"
                if effect == "valid":
                    _, proof = CASE["source_observer_runtime_command"](**args)
                    self.assertIn(executable.relative_to(args["inputs"]["home"]).as_posix(), proof["captured_members"])
                    self.assertIn(args["inputs"]["python"].relative_to(args["inputs"]["home"]).as_posix(), proof["captured_members"])
                    continue
                foreign = Path(temporary).resolve() / "foreign-python"; foreign.write_bytes(b"must never read")
                if effect == "changed":
                    def replace(prefix):
                        executable.unlink(); executable.symlink_to(foreign)
                        return prefix
                    scope["rewrite"] = replace
                else:
                    # Even a forged captured outside target cannot authorize a filesystem read.
                    args["records"][executable.relative_to(args["inputs"]["home"]).as_posix()]["target"] = str(foreign)
                actual = CASE["ordinary_bytes"]
                def read(path, *positional, **keywords):
                    self.assertNotEqual(Path(path), foreign)
                    return actual(path, *positional, **keywords)
                with mock.patch.dict(CASE["source_observer_runtime_command"].__globals__, ordinary_bytes=read), self.assertRaises(RuntimeError):
                    CASE["source_observer_runtime_command"](**args)

    def test_observer_verification_reuses_parent_authority_and_original_budget(self):
        import time
        with tempfile.TemporaryDirectory() as temporary:
            args, _ = self.fixture(temporary)
            paths, original = [], CASE["ordinary_bytes"]
            def read(path, *positional, **keywords):
                paths.append(Path(path))
                self.assertEqual(keywords["deadline"], args["deadline"])
                if not CASE["artifacts"].WINDOWS: self.assertIsInstance(keywords["parent"], int)
                return original(path, *positional, **keywords)
            with mock.patch.dict(CASE["source_observer_runtime_command"].__globals__, ordinary_bytes=read):
                CASE["source_observer_runtime_command"](**args)
            self.assertEqual(len(paths), 8)
            path = args["inputs"]["facts"]; before = path.stat()
            with mock.patch.object(CASE["os"], "read", side_effect=AssertionError("expired pinned read")), self.assertRaisesRegex(RuntimeError, "deadline"):
                original(path, before, pin=(before.st_size, CASE["hashlib"].sha256(path.read_bytes()).hexdigest()), deadline=time.monotonic() - 1)


class PublicInspectLockContracts(unittest.TestCase):
    # Exact processlock record bytes from the real public Cobra inspect causal
    # replay (Go log SHA256 6551fead920e329e3162e621ab020e55c84d3df20686ad9822b2dd849cbb5915).
    original = b'{"pid":734,"acquired_at":"2000-01-01T00:00:00Z"}\n'
    refreshed = b'{"pid":734,"acquired_at":"2026-11-14T09:42:17.272426987Z"}\n'

    def fixture(self, temporary, scope="a"):
        outer = Path(temporary).resolve() / "outer"
        for name in ("a", "b"):
            project = outer / ("public-" + name)
            for path in (project / ".uap/plugin-data/owned", project / ".hermes/uap-session"):
                path.mkdir(parents=True, mode=0o700)
            (project / ".uap").chmod(0o700)
            (project / ".uap/state-v2.json").write_bytes(b"original state")
            (project / "foreign.txt").write_bytes(b"original foreign")
            lock = project / ".uap/mutation.lock"
            lock.write_bytes(self.original); lock.chmod(0o600)
        project = outer / ("public-" + scope)
        return outer, project, project / ".uap/mutation.lock"

    def command(self, outer, scope, effect):
        import ast
        import time
        # Execute the actual nested command through its delta guard. Inert
        # ports replace only runtime/protocol execution; filesystem captures,
        # the new held authority and the existing allowlist remain real.
        source = Path(CASE["__file__"]).read_bytes()
        owner = next(node for node in ast.parse(source).body if isinstance(node, ast.FunctionDef) and node.name == "observe_public")
        command = next(node for node in owner.body if isinstance(node, ast.FunctionDef) and node.name == "command")
        stop = next(index for index, node in enumerate(command.body) if isinstance(node, ast.Expr)
                    and isinstance(node.value, ast.Call) and any(isinstance(arg, ast.Constant)
                    and arg.value == "public action changed an unselected or unmeasured path" for arg in node.value.args))
        command.body = command.body[:stop + 1] + [ast.Return(value=ast.Name(id="delta", ctx=ast.Load()))]
        command.body[0] = ast.Global(names=command.body[0].names)
        projects = {key: outer / ("public-" + key) for key in ("a", "b")}
        proofs = {key: {"data": path / ".uap/plugin-data/owned"} for key, path in projects.items()}
        current = CASE["inventory"](outer)
        def snapshot(): return CASE["inventory"](outer), proofs
        calls = []
        def bounded(*args, **kwargs):
            calls.append(args[0]); effect(); return b"{}"
        namespace = dict(CASE, current=current, proofs=proofs, snapshot=snapshot, outer=outer,
                         projects=projects, pins={"uap": {"path": "inert-uap"}}, deadline=time.monotonic() + 60,
                         evidence=outer.parent, env={}, born={}, nonce="inert", bounded=bounded,
                         public_runtime_pins=lambda *args, **kwargs: None, public_continuity=lambda *args, **kwargs: None,
                         public_stdio_events=lambda *args: [], probes={key: SimpleNamespace(methods=lambda: []) for key in projects})
        exec(compile(ast.fix_missing_locations(ast.Module(body=[command], type_ignores=[])), CASE["__file__"], "exec"), namespace)
        return namespace["command"], calls

    def test_actual_control_writer_refresh_is_scoped_to_existing_inspect_calls(self):
        for scope in ("a", "b"):
            for action in ("inspect", "native", "wrong-label", "wrong-args", "bad-record", "extra-state", "extra-lock-prefix", "extra-sibling"):
                with self.subTest(scope=scope, action=action), tempfile.TemporaryDirectory() as temporary:
                    outer, project, lock = self.fixture(temporary, scope)
                    original = lock.stat()
                    def effect():
                        lock.write_bytes(self.refreshed)
                        if action == "bad-record": lock.write_bytes(self.refreshed.replace(b"734", b"true"))
                        if action == "extra-state": (project / ".uap/state-v2.json").write_bytes(b"changed state")
                        if action == "extra-lock-prefix": (project / ".uap/mutation.lock-extra").write_bytes(b"unowned sibling")
                        if action == "extra-sibling": (outer / ("public-b" if scope == "a" else "public-a") / "foreign.txt").write_bytes(b"changed foreign")
                    command, calls = self.command(outer, scope, effect)
                    label = "public-run-" + scope if action != "wrong-label" else "public-http-" + scope
                    args = ["run", "--inspect", "--confirm-client-start"] if action != "wrong-args" else ["run"]
                    with mock.patch.dict(os.environ, HERMES_BIN="inert-hermes"):
                        invoke = lambda: command(label, scope, args, inspect=action != "native", native=action == "native")
                        if action == "inspect":
                            delta = invoke()
                            self.assertEqual(set(delta), {lock.relative_to(outer).as_posix()})
                            self.assertTrue(os.path.samestat(original, lock.stat()))
                        else:
                            message = "control role|control lock metadata|unselected or unmeasured"
                            message += "|public control original parent changed" if action == "extra-lock-prefix" else ""
                            with self.assertRaisesRegex(RuntimeError, message):
                                invoke()
                    self.assertEqual(len(calls), 0 if action in {"wrong-label", "wrong-args"} else 1)

    def test_control_record_is_exact_canonical_writer_data(self):
        values = [self.original, self.refreshed,
                  b'{"pid":1,"acquired_at":"2024-02-29T23:59:59.1Z"}\n']
        for body in values: CASE["public_lock_record"](body)
        bad = [b"{}", b"[]", self.original + b"\n", self.original.rstrip(b"\n"),
               self.original.replace(b"734", b"true"), self.original.replace(b"734", b"734.0"),
               self.original.replace(b"734", b"0"), self.original.replace(b"734", b"4294967296"),
               self.original.replace(b'"pid":734,', b'"pid":734,"pid":734,'),
               self.original.replace(b'"pid":734,', b'"pid":734,"extra":0,'),
               b'{"acquired_at":"2000-01-01T00:00:00Z","pid":734}\n',
               self.original.replace(b":734", b": 734"),
               self.original.replace(b"2000-01-01", b"2001-02-29"),
               self.original.replace(b"00:00:00", b"24:00:00"),
               self.original.replace(b"00:00:00", b"00:00:60"),
               self.original.replace(b"Z", b"+00:00"),
               self.original.replace(b"Z", b".0Z"), self.original.replace(b"Z", b".10Z"),
               self.original.replace(b"Z", b".1234567891Z"), b"x" * 257]
        for body in bad:
            with self.subTest(body=body), self.assertRaises((RuntimeError, ValueError)):
                CASE["public_lock_record"](body)

    def test_preimage_replacement_and_aliases_refuse_before_body_read(self):
        for role in ("lock", "parent", "project", "outer", "hardlink", "symlink", "missing", "metadata-row"):
            with self.subTest(role=role), tempfile.TemporaryDirectory() as temporary:
                outer, project, lock = self.fixture(temporary)
                before = CASE["inventory"](outer)
                if role == "missing": lock.unlink()
                elif role == "metadata-row": before[lock.relative_to(outer).as_posix()]["bytes"] = True
                elif role == "hardlink": os.link(lock, outer.parent / "second-lock-name")
                elif role == "symlink":
                    target = outer.parent / "original-lock"; lock.rename(target)
                    try: lock.symlink_to(target)
                    except OSError as error:
                        if os.name != "nt" or getattr(error, "winerror", None) != 1314: raise
                        # An unprivileged Windows host cannot author this link.
                        self.assertFalse(lock.exists()); continue
                else:
                    path = {"lock": lock, "parent": lock.parent, "project": project, "outer": outer}[role]
                    moved = outer.parent / ("original-" + role); path.rename(moved)
                    if role == "lock": path.write_bytes(self.original); path.chmod(0o600)
                    else:
                        path.mkdir(mode=0o700)
                        for child in moved.iterdir(): child.rename(path / child.name)
                with mock.patch.object(os, "fdopen", side_effect=AssertionError("unowned control body read")), \
                        self.assertRaises((RuntimeError, OSError)):
                    with CASE["public_inspect_lock"](outer, project, before, lambda: (CASE["inventory"](outer), {})):
                        self.fail("changed preimage accepted")

    def test_original_handles_refuse_replacement_before_postimage_read(self):
        roles = ("lock", "hardlink", "mode") + (() if os.name == "nt" else ("parent", "project", "outer"))
        for role in roles:
            with self.subTest(role=role), tempfile.TemporaryDirectory() as temporary:
                outer, project, lock = self.fixture(temporary)
                before = CASE["inventory"](outer)
                snapshot = mock.Mock(side_effect=lambda: (CASE["inventory"](outer), {}))
                with CASE["public_inspect_lock"](outer, project, before, snapshot) as finish:
                    if role == "hardlink": os.link(lock, outer.parent / "second-lock-name")
                    elif role == "mode": lock.chmod(0o400)
                    else:
                        path = {"lock": lock, "parent": lock.parent, "project": project, "outer": outer}[role]
                        moved = outer.parent / ("original-" + role); path.rename(moved)
                        if role == "lock": path.write_bytes(self.original); path.chmod(0o600)
                        else:
                            path.mkdir(mode=0o700)
                            for child in moved.iterdir(): child.rename(path / child.name)
                    with mock.patch.object(os, "fdopen", side_effect=AssertionError("replacement postimage read")), \
                            self.assertRaisesRegex(RuntimeError, "original (parent|lock) changed"):
                        finish()
                    snapshot.assert_not_called()
                if role == "mode": lock.chmod(0o600)

    def test_postimage_hash_and_final_named_held_fences_are_mandatory(self):
        for effect in ("hash", "typed-row", "after-read", "after-snapshot", "replacement"):
            with self.subTest(effect=effect), tempfile.TemporaryDirectory() as temporary:
                outer, project, lock = self.fixture(temporary)
                before = CASE["inventory"](outer)
                def snapshot():
                    if effect == "after-read": lock.write_bytes(self.original)
                    image = CASE["inventory"](outer)
                    row = image[lock.relative_to(outer).as_posix()]
                    if effect == "hash": row["sha256"] = "0" * 64
                    elif effect == "typed-row": row["bytes"] = True
                    elif effect == "after-snapshot": lock.write_bytes(self.original)
                    elif effect == "replacement":
                        lock.rename(outer.parent / "original-lock")
                        lock.write_bytes(self.refreshed); lock.chmod(0o600)
                    return image, {}
                with CASE["public_inspect_lock"](outer, project, before, snapshot) as finish:
                    lock.write_bytes(self.refreshed)
                    with self.assertRaisesRegex(RuntimeError, "postimage differs|original lock changed"):
                        finish()

    def test_control_identity_retains_all_nonvolatile_metadata(self):
        with tempfile.TemporaryDirectory() as temporary:
            _, _, lock = self.fixture(temporary)
            info = lock.stat()
            values = {name: getattr(info, name) for name in CASE["STABLE_FIELDS"]}
            values.update(st_birthtime_ns=123456789, st_birthtime=0.123456789,
                          st_file_attributes=32, st_reparse_tag=0)
            original = SimpleNamespace(**values)
            for field in ("st_dev", "st_ino", "st_mode", "st_nlink", "st_uid", "st_gid",
                          "st_birthtime_ns", "st_birthtime", "st_file_attributes", "st_reparse_tag"):
                changed = SimpleNamespace(**{**values, field: values[field] + 1})
                with self.subTest(field=field):
                    try: observed = CASE["public_lock_identity"](changed)
                    except ValueError: continue
                    self.assertNotEqual(CASE["public_lock_identity"](original), observed)

    def test_read_and_child_failure_preserve_the_exception_and_close_original_handles(self):
        for effect in ("read-change", "child-failure"):
            with self.subTest(effect=effect), tempfile.TemporaryDirectory() as temporary:
                outer, project, lock = self.fixture(temporary)
                before = CASE["inventory"](outer)
                observed, actual_stat, actual_open = set(), os.fstat, os.fdopen
                def fstat(fd):
                    observed.add(fd); return actual_stat(fd)
                @CASE["contextmanager"]
                def fdopen(*args, **kwargs):
                    with actual_open(*args, **kwargs) as stream:
                        def read(limit):
                            body = stream.read(limit)
                            lock.write_bytes(self.original)
                            return body
                        yield SimpleNamespace(read=read)
                cause = RuntimeError("inert child failure")
                with mock.patch.object(os, "fstat", side_effect=fstat), self.assertRaises(RuntimeError) as failed:
                    with CASE["public_inspect_lock"](outer, project, before, lambda: self.fail("snapshot after refused capture")) as finish:
                        if effect == "child-failure": raise cause
                        lock.write_bytes(self.refreshed)
                        with mock.patch.object(os, "fdopen", side_effect=fdopen): finish()
                if effect == "child-failure": self.assertIs(failed.exception, cause)
                else: self.assertIn("changed during capture", str(failed.exception))
                self.assertEqual(len(observed), 4)
                for descriptor in observed:
                    with self.assertRaises(OSError): actual_stat(descriptor)



class FailureReadoutContracts(unittest.TestCase):
    nonce, head = "a" * 64, "b" * 40
    flags = ("admission_qualified", "production_project_mcp", "vendor_authentication", "real_model_inference",
             "tools_call", "source_install_custom_root_qualified", "source_bootstrap_recovery_qualified")

    def encoded(self, value):
        return json.dumps(value, sort_keys=True, separators=(",", ":")).encode("ascii")

    def proof(self, distribution):
        value = dict.fromkeys(self.flags, False)
        value.update(status="failed", runtime_distribution=distribution, error="PRIVATE ERROR /outside/config")
        if distribution == "source":
            source = dict(commit=self.head, tree="c" * 40, dirty="")
            value.update(capture_complete=False, nonce=self.nonce, source=source, source_after=dict(source))
        return value

    def source_refusal(self, root):
        import stat
        import time
        target = str(root.parent / "PRIVATE-TARGET")
        item = lambda inode, value: dict(type="link", target=value,
            identity=(1, inode, stat.S_IFLNK | 0o777, 1, len(value), 10, 10, 1000, 1000))
        records = {"user/launcher": item(2, "tools/python"), "user": dict(type="directory"),
                   "user/tools": dict(type="directory"), "user/tools/python": item(3, target)}
        source = dict(user="user", source="user/source", tools="user/tools", installs="user/installs", projects=())
        with mock.patch.object(os, "read", side_effect=AssertionError("target read")), \
                mock.patch.object(os, "open", side_effect=AssertionError("target open")), \
                mock.patch.object(os, "scandir", side_effect=AssertionError("new walk")), \
                mock.patch.object(Path, "lstat", side_effect=AssertionError("new stat")), \
                self.assertRaisesRegex(RuntimeError, "^payload link escapes runtime$") as refused:
            CASE["payload_links"](root, records, time.monotonic() + 10, source=source)
        value = dict(refused.exception.source_payload_refusal, nonce=self.nonce, stage="parent-before-observer")
        self.assertEqual(value["member"], "user/tools/python")
        return value, target

    def native_proof(self):
        proof = self.proof("bundled")
        observer = ObserverFailureDiagnosticContracts().record()
        proof["native_failure"] = dict(source_head=self.head, nonce=self.nonce, cleanup_phase="public-family",
                                       primary_family_settled=True, observer_exit=1, observer=observer)
        return proof

    def test_source_readout_uses_captured_intermediate_link_without_reading_target(self):
        import hashlib
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            refusal, target = self.source_refusal(root)
            body = self.encoded(self.proof("source"))
            with mock.patch.object(os, "read", side_effect=AssertionError("readout read")), \
                    mock.patch.object(os, "open", side_effect=AssertionError("readout open")), \
                    mock.patch.object(os, "scandir", side_effect=AssertionError("readout walk")), \
                    mock.patch.object(Path, "lstat", side_effect=AssertionError("readout stat")):
                line = CASE["admission_failure_readout"](body, refusal)
            report = json.loads(line)
            self.assertEqual(report["source_head"], self.head)
            self.assertEqual(report["nonce_sha256"], hashlib.sha256(self.nonce.encode()).hexdigest())
            self.assertEqual(report["source_refusal"]["stage"], "parent-before-observer")
            self.assertEqual(report["source_refusal"]["operation"], "absolute-link-target")
            self.assertEqual(report["source_refusal"]["member"], "user/tools/python")
            self.assertEqual(report["source_refusal"]["member_sha256"], refusal["member_sha256"])
            self.assertEqual(report["source_refusal"]["target_sha256"], hashlib.sha256(target.encode("utf-8", "surrogatepass")).hexdigest())
            self.assertIsNone(report["native_failure"])
            self.assertTrue(report["measurement_only"])
            self.assertIs(report["admission_qualified"], False); self.assertIs(report["native_qualified"], False)
            self.assertLessEqual(len(line.encode("ascii")) + len("HERMES_ADMISSION_FAILURE ") + 1, 2048)
            for private in (self.nonce, target, str(root), "PRIVATE ERROR"):
                self.assertNotIn(private, line)
            original = RuntimeError("PRIVATE ERROR")
            CASE["source_payload_refusal"](original, "user/" + "x" * 600,
                                          dict(identity=(1, 2, 0o120777, 1, 0, 10, 10, 1000, 1000)), target=target)
            long_member = dict(original.source_payload_refusal, nonce=self.nonce, stage="parent-before-observer")
            long_report = json.loads(CASE["admission_failure_readout"](body, long_member))
            self.assertIsNone(long_report["source_refusal"]["member"])
            self.assertEqual(long_report["source_refusal"]["member_sha256"], long_member["member_sha256"])

    def test_bundled_readout_preserves_unproved_cleanup_and_validated_observer_caller(self):
        import copy
        helper, proof = CASE["admission_failure_readout"], self.native_proof()
        context = proof["native_failure"]
        for mode in ("valid", "absent-observer", "unsettled", "source", "nonce", "caller", "phase", "extra", "exit"):
            with self.subTest(mode=mode):
                changed = copy.deepcopy(proof)
                value = changed["native_failure"]
                if mode == "absent-observer": value.update(primary_family_settled=False, observer=None, observer_exit=None)
                if mode == "unsettled": value["primary_family_settled"] = False
                if mode == "source": value["observer"]["source_head"] = "d" * 40
                if mode == "nonce": value["observer"]["nonce"] = "e" * 64
                if mode == "caller": value["observer"]["caller"]["line"] = 999999
                if mode == "phase": value["cleanup_phase"] = "../PRIVATE"
                if mode == "extra": value["PRIVATE"] = "raw body"
                if mode == "exit": value["observer_exit"] = True
                with mock.patch.object(os, "read", side_effect=AssertionError("readout read")), \
                        mock.patch.object(Path, "read_bytes", side_effect=AssertionError("readout reopen")):
                    line = helper(self.encoded(changed), None)
                if mode not in ("valid", "absent-observer"):
                    self.assertIsNone(line)
                    continue
                report = json.loads(line)
                self.assertEqual(report["native_failure"]["cleanup_phase"], "public-family")
                self.assertEqual(report["native_failure"]["primary_family_settled"], mode == "valid")
                self.assertIs(report["native_qualified"], False); self.assertIs(report["admission_qualified"], False)
                self.assertIsNone(report["source_refusal"])
                if mode == "valid":
                    self.assertEqual(report["native_failure"]["observer"]["caller"], context["observer"]["caller"])
                    self.assertEqual(report["native_failure"]["observer"]["phase"], "public")
                    self.assertEqual(report["native_failure"]["observer"]["exception_kind"], "RuntimeError")
                else:
                    self.assertIsNone(report["native_failure"]["observer"])
                for private in (self.nonce, "PRIVATE", ".py"):
                    self.assertNotIn(private, line)

    def test_failure_readout_omits_unjoined_malformed_and_qualified_documents(self):
        import copy
        helper = CASE["admission_failure_readout"]
        with tempfile.TemporaryDirectory() as temporary:
            refusal, _ = self.source_refusal(Path(temporary).resolve())
            for mode in ("nonce", "source", "tree", "dirty", "changed-source", "extra-source", "success",
                         "wrong-distribution", "missing-capture", "member-hash", "refusal-extra", *self.flags):
                with self.subTest(mode=mode):
                    proof, observed = self.proof("source"), copy.deepcopy(refusal)
                    if mode == "nonce": proof["nonce"] = "d" * 64
                    if mode == "source": proof["source"]["commit"] = "not-a-source"
                    if mode == "tree": proof["source"]["tree"] = True
                    if mode == "dirty": proof["source"]["dirty"] = "PRIVATE"
                    if mode == "changed-source": proof["source_after"]["commit"] = "d" * 40
                    if mode == "extra-source": proof["source"]["PRIVATE"] = "raw"
                    if mode == "success": proof["status"] = "passed"
                    if mode == "wrong-distribution": proof["runtime_distribution"] = "bundled"
                    if mode == "missing-capture": proof.pop("capture_complete")
                    if mode == "member-hash": observed["member_sha256"] = "d" * 64
                    if mode == "refusal-extra": observed["PRIVATE"] = "raw"
                    if mode in self.flags: proof[mode] = True
                    self.assertIsNone(helper(self.encoded(proof), observed))
            body = self.encoded(self.proof("source"))
            duplicate = body[:-1] + b',"status":"failed"}'
            for bad in (None, b"", b"{", duplicate, b" " * (CASE["LIMIT"] + 1)):
                self.assertIsNone(helper(bad, refusal))

    def test_staged_failure_log_follows_commit_and_reuses_existing_reads(self):
        stage, authority = CASE["stage_evidence"], CASE["stage_evidence"].__globals__
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve(); evidence = root / "evidence"; evidence.mkdir()
            refusal, _ = self.source_refusal(root)
            bodies = {"profile-admission.json": self.encoded(self.proof("source")),
                      "source-payload-refusal.json": self.encoded(refusal)}
            for name, body in bodies.items(): (evidence / name).write_bytes(body)
            lines, reads, actual = [], [], CASE["ordinary_bytes"]
            def read(path, *args, **kwargs):
                self.assertFalse((evidence / "upload").exists(), "readout reopened committed evidence")
                reads.append(path.name)
                return actual(path, *args, **kwargs)
            def emit(line, *, flush):
                self.assertTrue(flush); self.assertTrue(authority["EVIDENCE_CLOSED"])
                self.assertTrue((evidence / "upload").is_dir())
                lines.append(line)
            with mock.patch.dict(authority, ordinary_bytes=read, print=emit, EVIDENCE_CLOSED=False):
                stage(evidence)
            self.assertEqual(reads.count("profile-admission.json"), 1)
            self.assertEqual(reads.count("source-payload-refusal.json"), 2)
            self.assertEqual(len(lines), 2)
            self.assertTrue(lines[0].startswith("HERMES_BOOTSTRAP_CENSUS "))
            report = json.loads(lines[1].removeprefix("HERMES_ADMISSION_FAILURE "))
            self.assertEqual(report["source_refusal"]["member_sha256"], refusal["member_sha256"])
            for name, body in bodies.items():
                self.assertEqual((evidence / name).read_bytes(), body)
                self.assertEqual((evidence / "upload" / name).read_bytes(), body)

    def test_settled_observer_context_joins_staged_profile_without_new_artifact_fields(self):
        import copy
        stage, authority = CASE["stage_evidence"], CASE["stage_evidence"].__globals__
        for mode in ("valid", "nonce", "source", "success", "unsettled", "caller", "missing-source", "mixed-context"):
            with self.subTest(mode=mode), tempfile.TemporaryDirectory() as temporary:
                evidence = Path(temporary).resolve()
                proof, context = self.proof("source"), copy.deepcopy(self.native_proof()["native_failure"])
                if mode == "nonce": context["nonce"] = "d" * 64
                if mode == "source": context["source_head"] = "d" * 40
                if mode == "success": proof.update(status="passed", capture_complete=True)
                if mode == "unsettled": context["primary_family_settled"] = False
                if mode == "caller": context["observer"]["caller"]["line"] = -1
                if mode == "missing-source": proof.pop("source")
                if mode == "mixed-context": proof["native_failure"] = context
                body = self.encoded(proof); (evidence / "profile-admission.json").write_bytes(body)
                lines, reads, actual = [], [], CASE["ordinary_bytes"]
                def read(path, *args, **kwargs):
                    self.assertFalse((evidence / "upload").exists())
                    reads.append(path.name)
                    return actual(path, *args, **kwargs)
                def emit(line, *, flush):
                    self.assertTrue(flush); self.assertTrue(authority["EVIDENCE_CLOSED"])
                    self.assertTrue((evidence / "upload").is_dir()); lines.append(line)
                with mock.patch.dict(authority, ordinary_bytes=read, print=emit, EVIDENCE_CLOSED=False):
                    stage(evidence, native_failure=context)
                self.assertEqual(reads, ["profile-admission.json"])
                self.assertEqual((evidence / "upload/profile-admission.json").read_bytes(), body)
                self.assertEqual((evidence / "profile-admission.json").read_bytes(), body)
                self.assertEqual(len(lines), 2 if mode == "valid" else 1)
                if mode == "valid":
                    self.assertNotIn("native_failure", json.loads(body))
                    value = json.loads(lines[1].removeprefix("HERMES_ADMISSION_FAILURE "))
                    self.assertEqual(value["source_head"], self.head)
                    self.assertEqual(value["native_failure"]["observer"]["caller"], context["observer"]["caller"])
                    self.assertEqual(value["native_failure"]["cleanup_phase"], "public-family")
                    self.assertIs(value["native_failure"]["primary_family_settled"], True)
                    self.assertIs(value["native_qualified"], False); self.assertIs(value["admission_qualified"], False)
                    self.assertIsNone(value["source_refusal"])


    def test_publication_failure_and_optional_log_errors_preserve_existing_outcomes(self):
        stage, authority = CASE["stage_evidence"], CASE["stage_evidence"].__globals__
        for mode in ("rename", "bad-join", "success", "malformed-native", "stdout-oserror", "stdout-valueerror"):
            with self.subTest(mode=mode), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary).resolve(); evidence = root / "evidence"; evidence.mkdir()
                proof = self.native_proof()
                if mode in ("rename", "bad-join"):
                    refusal, _ = self.source_refusal(root)
                    (evidence / "source-payload-refusal.json").write_bytes(self.encoded(refusal))
                    proof = self.proof("source")
                    if mode == "bad-join": proof["nonce"] = "d" * 64
                if mode == "success": proof.update(status="passed", capture_complete=True)
                if mode == "malformed-native": proof["native_failure"]["observer"]["caller"]["line"] = -1
                body = self.encoded(proof); (evidence / "profile-admission.json").write_bytes(body)
                lines = []
                def emit(line, *, flush):
                    self.assertTrue(authority["EVIDENCE_CLOSED"])
                    lines.append(line)
                    if mode == "stdout-oserror": raise OSError("PRIVATE")
                    if mode == "stdout-valueerror": raise ValueError("PRIVATE")
                with mock.patch.dict(authority, print=emit, EVIDENCE_CLOSED=False):
                    if mode == "rename":
                        with mock.patch.object(Path, "rename", side_effect=OSError("inert commit refusal")), self.assertRaises(OSError):
                            stage(evidence)
                    elif mode == "bad-join":
                        with self.assertRaisesRegex(RuntimeError, "source refusal proof differs"): stage(evidence)
                    else:
                        original = RuntimeError("original native refusal")
                        with self.assertRaises(RuntimeError) as caught:
                            try: raise original
                            except RuntimeError:
                                stage(evidence)
                                raise
                        self.assertIs(caught.exception, original)
                    if mode in ("rename", "bad-join"):
                        self.assertEqual(lines, []); self.assertFalse(authority["EVIDENCE_CLOSED"])
                        self.assertFalse((evidence / "upload").exists())
                    else:
                        self.assertTrue(authority["EVIDENCE_CLOSED"])
                        self.assertEqual((evidence / "upload/profile-admission.json").read_bytes(), body)
                        self.assertEqual(len(lines), 1 if mode in ("success", "malformed-native") else 2)
                self.assertEqual((evidence / "profile-admission.json").read_bytes(), body)


class PublicBootstrapFailureDetailContracts(unittest.TestCase):
    nonce, source = "a" * 64, "b" * 40

    def fixture(self, *, scope="a", backups=0, unexpected=()):
        import hashlib
        outer = Path("outer")
        root = outer / ("public-" + scope) / ".hermes/uap-session"
        prefix = root.relative_to(outer).as_posix()
        def directory(mode=0o700):
            return dict(type="directory", mode=0o777 if os.name == "nt" else mode, device=1, inode=2)
        def regular(body):
            return dict(type="file", mode=0o666 if os.name == "nt" else 0o600, device=1, inode=3,
                        bytes=len(body), sha256=hashlib.sha256(body).hexdigest())
        marker = ("uap-hermes-project-session-v1\n" + str(root.parent.parent) + "\n").encode()
        config = b"mcp_servers: {}\n"
        rows = {".uap-owner": regular(marker), "config.yaml": regular(config),
                "backups": directory(0o755), "backups/config": directory(0o755)}
        for index in range(backups):
            rows["backups/config/config.yaml.good.20261114-" + f"{index:06d}"] = regular(config)
        for name, kind in unexpected:
            rows[name] = directory() if kind == "directory" else regular(b"PRIVATE BODY")
        image = {prefix: directory(), **{prefix + "/" + name: row for name, row in rows.items()}}
        return root, image, outer, {hashlib.sha256(config).hexdigest()}

    def refused(self, fixture):
        try:
            CASE["public_profile"](*fixture)
        except RuntimeError as error:
            self.assertEqual(str(error), "public native bootstrap member is unmeasured")
            return error
        self.fail("the original membership guard must refuse")

    def emitted(self, fixture, *, label="public-stdio-a", phase="public", mutate=None, foreign=False):
        import io
        import sys
        from types import FunctionType
        owner = CASE["observe_public"]
        code = next(value for value in owner.__code__.co_consts
                    if type(value) is type(owner.__code__) and value.co_name == "command")
        cell = (lambda value: lambda: value)(None).__closure__[0]
        command = FunctionType(code, owner.__globals__, closure=tuple(cell for _ in code.co_freevars))
        details = self.refused(fixture).__dict__.get("_uap_public_bootstrap")
        originals = []
        def refuse(*args, **kwargs):
            if foreign:
                error = RuntimeError("PRIVATE FOREIGN ERROR")
                error.__dict__["_uap_public_bootstrap"] = details
                originals.append(error)
                raise error
            try:
                CASE["public_profile"](*fixture)
            except RuntimeError as error:
                if mutate is not None:
                    mutate(error)
                originals.append(error)
                raise
        stream = io.BytesIO()
        with mock.patch.dict(owner.__globals__, public_runtime_pins=refuse), \
                mock.patch.object(sys, "stdout", SimpleNamespace(buffer=stream)), \
                mock.patch.dict(os.environ, {"HERMES_CALL_NONCE": self.nonce, "HERMES_PROFILE_SOURCE_HEAD": self.source}):
            try:
                command(label, "a", [], native=False, inspect=False, expected_failure=False, test=None)
            except RuntimeError as error:
                self.assertIs(error, originals[0])
                CASE["emit_observer_failure"](error, phase)
            else:
                self.fail("the original observer refusal must escape")
        return stream.getvalue(), originals[0]

    def test_actual_profile_guard_projects_only_captured_counts_and_first_member_hash(self):
        import hashlib
        private = "PRIVATE-NONCE-" + self.nonce
        for scope, backups, unexpected, expected in (
                ("a", 6, (), "backup-count"),
                ("b", 0, (("z-" + private, "directory"), ("a-" + private, "file")), "unmeasured-member"),
                ("a", 6, (("a-" + private, "directory"),), "both")):
            with self.subTest(scope=scope, expected=expected):
                fixture = self.fixture(scope=scope, backups=backups, unexpected=unexpected)
                with mock.patch.object(os, "read", side_effect=AssertionError("diagnostic read")), \
                        mock.patch.object(os, "open", side_effect=AssertionError("diagnostic open")), \
                        mock.patch.object(os, "scandir", side_effect=AssertionError("diagnostic walk")), \
                        mock.patch.object(Path, "lstat", side_effect=AssertionError("diagnostic stat")), \
                        mock.patch.object(Path, "read_bytes", side_effect=AssertionError("diagnostic bytes")):
                    error = self.refused(fixture)
                detail = error.__dict__["_uap_public_bootstrap"]
                self.assertEqual(CASE["public_bootstrap_detail"](detail), detail)
                self.assertEqual((detail["scope"], detail["predicate"]), (scope, expected))
                self.assertEqual((detail["member_count"], detail["backup_count"], detail["unmeasured_count"]),
                                 (4 + backups + len(unexpected), backups, len(unexpected)))
                if unexpected:
                    name, kind = min(unexpected)
                    first = detail["first_unmeasured"]
                    self.assertEqual(first["member_sha256"], hashlib.sha256(
                        b"uap-hermes-public-bootstrap-member-v1\0" + name.encode("utf-8", "surrogatepass")).hexdigest())
                    self.assertEqual(first["type"], kind)
                    self.assertEqual(first["bytes"], len(b"PRIVATE BODY") if kind == "file" else None)
                    self.assertNotIn("sha256", first)  # Never publish a deterministic fingerprint of foreign file contents.
                else:
                    self.assertIsNone(detail["first_unmeasured"])
                encoded = json.dumps(detail, sort_keys=True)
                for raw in (private, self.nonce, "PRIVATE BODY", str(fixture[0]), ".uap-owner"):
                    self.assertNotIn(raw, encoded)
                self.assertNotIn(hashlib.sha256(b"PRIVATE BODY").hexdigest(), encoded)
        self.assertIsNone(CASE["public_profile"](*self.fixture(backups=5)))
        fixture = self.fixture(unexpected=(("unknown", "file"),))
        prefix = fixture[0].relative_to(fixture[2]).as_posix()
        fixture[1][prefix + "/unknown"]["target"] = "/PRIVATE TARGET"
        self.assertNotIn("_uap_public_bootstrap", self.refused(fixture).__dict__)

    def test_closed_detail_rejects_false_counts_unknown_fields_and_unsafe_metadata(self):
        import copy
        valid = self.refused(self.fixture(unexpected=(("unknown", "file"),))).__dict__["_uap_public_bootstrap"]
        invalid = []
        for key, value in (("scope", "user"), ("scope", True), ("predicate", "both"), ("predicate", "unknown"),
                           ("member_count", True), ("member_count", 0), ("member_count", 2048),
                           ("backup_count", -1), ("backup_count", 6), ("unmeasured_count", 0),
                           ("unmeasured_count", True), ("unmeasured_count", 6), ("member", "PRIVATE"),
                           ("first_unmeasured", None)):
            changed = copy.deepcopy(valid); changed[key] = value; invalid.append(changed)
        for key, value in (("member_sha256", "F" * 64), ("member_sha256", self.nonce + "x"),
                           ("type", "link"), ("mode", True), ("mode", 0o10000), ("bytes", -1),
                           ("bytes", CASE["LIMIT"] + 1), ("bytes", True), ("sha256", None),
                           ("sha256", "f" * 64),
                           ("target", "/PRIVATE"), ("member", "PRIVATE")):
            changed = copy.deepcopy(valid); changed["first_unmeasured"][key] = value; invalid.append(changed)
        changed = copy.deepcopy(valid); changed["first_unmeasured"]["type"] = "directory"; invalid.append(changed)
        backup = self.refused(self.fixture(backups=6)).__dict__["_uap_public_bootstrap"]
        changed = copy.deepcopy(backup); changed["first_unmeasured"] = valid["first_unmeasured"]; invalid.append(changed)
        class Hostile:
            def __eq__(self, other): raise AssertionError("foreign equality")
            def __hash__(self): raise AssertionError("foreign hash")
            def __str__(self): raise AssertionError("foreign text")
        changed = copy.deepcopy(valid); changed["first_unmeasured"]["type"] = Hostile(); invalid.append(changed)
        changed = copy.deepcopy(valid); changed["scope"] = Hostile(); invalid.append(changed)
        for index, value in enumerate(invalid):
            with self.subTest(index=index), self.assertRaises(RuntimeError):
                CASE["public_bootstrap_detail"](value)

    def test_owned_caller_command_scope_and_optional_fallback_preserve_original_failure(self):
        import copy
        fixture = self.fixture(unexpected=(("PRIVATE-" + self.nonce, "file"),))
        body, original = self.emitted(fixture)
        record = CASE["observer_failure_document"](body, self.nonce, self.source)
        self.assertEqual(record["public_bootstrap"], original.__dict__["_uap_public_bootstrap"])
        self.assertEqual(record["caller"]["function"], "public_profile")
        self.assertEqual(record["command"], "public-stdio-a")
        self.assertLessEqual(len(body), 2048)
        for label, phase, mutate, foreign in (
                ("public-stdio-b", "public", None, False),
                ("public-stdio-a", "profiles", None, False),
                ("PRIVATE", "public", None, False),
                ("public-stdio-a", "public", lambda error: error.__dict__["_uap_public_bootstrap"].update(predicate="both"), False),
                ("public-stdio-a", "public", lambda error: error.__dict__.update(_uap_public_bootstrap=object()), False),
                ("public-stdio-a", "public", None, True)):
            with self.subTest(label=label, phase=phase, foreign=foreign):
                emitted, error = self.emitted(fixture, label=label, phase=phase, mutate=mutate, foreign=foreign)
                value = CASE["observer_failure_document"](emitted, self.nonce, self.source)
                self.assertNotIn("public_bootstrap", value)
                self.assertEqual(value["exception_kind"], "RuntimeError")
                self.assertEqual(str(error), "PRIVATE FOREIGN ERROR" if foreign else "public native bootstrap member is unmeasured")
                self.assertNotIn(b"PRIVATE", emitted)
        for key, value in (("phase", "profiles"), ("exception_kind", "ValueError"),
                           ("command", "public-stdio-b"), ("command", None), ("caller", None)):
            changed = copy.deepcopy(record); changed[key] = value
            encoded = json.dumps(changed, sort_keys=True, separators=(",", ":")).encode("ascii") + b"\n"
            with self.subTest(key=key), self.assertRaises(RuntimeError):
                CASE["observer_failure_document"](encoded, self.nonce, self.source)
        changed = copy.deepcopy(record)
        changed["caller"] = ObserverFailureDiagnosticContracts().record()["caller"]
        with self.assertRaises(RuntimeError):
            CASE["observer_failure_document"](json.dumps(changed, sort_keys=True, separators=(",", ":")).encode("ascii") + b"\n",
                                              self.nonce, self.source)

    def test_retained_descriptor_and_failure_readout_keep_settlement_identity_and_bounds(self):
        import copy
        body, _ = self.emitted(self.fixture(backups=6, unexpected=(("PRIVATE-" + self.nonce, "file"),)))
        expected = CASE["observer_failure_document"](body, self.nonce, self.source)
        reader = CASE["read_observer_failure"]
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            with (root / "original.stdout").open("x+b") as stream:
                original = os.fstat(stream.fileno())
                stream.write(body); stream.flush()
                with mock.patch.object(os, "read", side_effect=AssertionError("unsettled read")):
                    self.assertIsNone(reader(stream.fileno(), original, self.nonce, self.source, primary_settled=False))
                with mock.patch.object(Path, "open", side_effect=AssertionError("pathname reopen")):
                    self.assertEqual(reader(stream.fileno(), original, self.nonce, self.source, primary_settled=True), expected)
                    self.assertIsNone(reader(stream.fileno(), original, "c" * 64, self.source, primary_settled=True))
                    self.assertIsNone(reader(stream.fileno(), original, self.nonce, "d" * 40, primary_settled=True))
        context = dict(source_head=self.source, nonce=self.nonce, cleanup_phase="public-family",
                       primary_family_settled=True, observer_exit=1, observer=expected)
        helper = CASE["admission_failure_readout"]
        fixtures = FailureReadoutContracts()
        safe = fixtures.proof("bundled"); safe["native_failure"] = context
        settled = fixtures.proof("source"); settled["runtime_distribution"] = "bundled"
        with mock.patch.object(os, "read", side_effect=AssertionError("readout read")), \
                mock.patch.object(Path, "read_bytes", side_effect=AssertionError("readout reopen")):
            lines = (helper(fixtures.encoded(safe), None), helper(fixtures.encoded(settled), None, native_failure=context))
        for line in lines:
            report = json.loads(line)
            self.assertEqual(report["native_failure"]["observer"]["public_bootstrap"], expected["public_bootstrap"])
            self.assertEqual(report["native_failure"]["cleanup_phase"], "public-family")
            self.assertIs(report["admission_qualified"], False); self.assertIs(report["native_qualified"], False)
            self.assertLessEqual(len(line.encode("ascii")) + len("HERMES_ADMISSION_FAILURE ") + 1, 2048)
            self.assertNotIn(self.nonce, line); self.assertNotIn("PRIVATE", line)
        for effect in ("unsettled", "nonce", "source", "success", "malformed"):
            changed = copy.deepcopy(safe)
            if effect == "unsettled": changed["native_failure"]["primary_family_settled"] = False
            if effect == "nonce": changed["native_failure"]["nonce"] = "c" * 64
            if effect == "source": changed["native_failure"]["source_head"] = "d" * 40
            if effect == "success": changed["status"] = "passed"
            if effect == "malformed": changed["native_failure"]["observer"]["public_bootstrap"]["unmeasured_count"] = False
            with self.subTest(effect=effect):
                self.assertIsNone(helper(fixtures.encoded(changed), None))

if __name__ == "__main__":
    unittest.main()
