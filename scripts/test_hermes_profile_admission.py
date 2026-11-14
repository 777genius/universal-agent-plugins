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


if __name__ == "__main__":
    unittest.main()
