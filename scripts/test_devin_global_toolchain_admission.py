"""Inert archive and real private-file authority contracts; never run Go."""
from contextlib import contextmanager
import copy
import gzip
import hashlib
import io
import json
import os
from pathlib import Path
import stat
import tarfile
import tempfile
from types import SimpleNamespace
import unittest
from unittest import mock

import devin_global_toolchain as case
import devin_structured_inventory_admission as base


def archive(members):
    output, rows = io.BytesIO(), []
    with tarfile.open(fileobj=output, mode="w", format=tarfile.PAX_FORMAT) as tree:
        for name, kind, mode, body, headers in members:
            member = tarfile.TarInfo(name)
            member.type, member.mode, member.pax_headers = kind, mode, headers
            member.size = len(body) if kind == tarfile.REGTYPE else 0
            if kind in (tarfile.LNKTYPE, tarfile.SYMTYPE): member.linkname = "../foreign"
            tree.addfile(member, io.BytesIO(body) if kind == tarfile.REGTYPE else None)
            row = dict(name=name.rstrip("/"), kind="directory" if kind == tarfile.DIRTYPE else "file",
                       mode=mode, bytes=member.size)
            if kind != tarfile.DIRTYPE: row["sha256"] = hashlib.sha256(body).hexdigest()
            rows.append(row)
    body = gzip.compress(output.getvalue(), mtime=0)
    pins = dict(ARCHIVE_BYTES=len(body), ARCHIVE_SHA256=hashlib.sha256(body).hexdigest(),
                MEMBERS=len(rows), DIRECTORIES=sum(row["kind"] == "directory" for row in rows),
                PAYLOAD=sum(row["bytes"] for row in rows), CATALOG_SHA256=case.catalog_digest(rows),
                GO_BYTES=8, GO_SHA256=hashlib.sha256(b"inert-go").hexdigest())
    return body, pins


MEMBERS = [("go", tarfile.DIRTYPE, 0o755, b"", {}),
           ("go/bin", tarfile.DIRTYPE, 0o755, b"", {}),
           ("go/bin/go", tarfile.REGTYPE, 0o755, b"inert-go", {}),
           ("go/src", tarfile.DIRTYPE, 0o755, b"", {}),
           ("go/src/source.go", tarfile.REGTYPE, 0o644, b"inert-source", {})]


class PrivateToolchainContracts(unittest.TestCase):
    @contextmanager
    def fixture(self, members=None):
        body, pins = archive(MEMBERS if members is None else members)
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve() / "owned"
            root.mkdir(mode=0o700)
            ctx = SimpleNamespace(root=root, original_output_identity=base.root_identity(root),
                budget=SimpleNamespace(check=lambda: 30), global_build_cleanup_proved=True,
                process_cleanup_proved=True)
            with mock.patch.multiple(case, **pins), mock.patch.object(case, "download", side_effect=lambda _budget: io.BytesIO(body)):
                yield ctx, body, pins

    def test_complete_private_distribution_and_exact_cleanup_leave_shared_input_unchanged(self):
        with self.fixture() as (ctx, _, _):
            foreign = ctx.root.parent / "shared-go"
            foreign.write_bytes(b"existing-host-go"); foreign.chmod(0o777)
            original = foreign.stat()
            descriptors = set(os.listdir("/proc/self/fd"))
            with case.prepared(ctx) as compiler:
                self.assertFalse(ctx.global_build_cleanup_proved)
                self.assertEqual(compiler.go.read_bytes(), b"inert-go")
                self.assertEqual(compiler.go.stat().st_mode, stat.S_IFREG | 0o755)
                self.assertEqual((compiler.path / "go/src/source.go").stat().st_mode, stat.S_IFREG | 0o644)
                self.assertEqual(len(compiler.entries), len(MEMBERS) + 1)
                for _ in range(4): compiler.verify()
                receipt = compiler.receipt
            self.assertTrue(ctx.global_build_cleanup_proved)
            self.assertFalse(compiler.path.exists())
            case.validate_receipt(receipt, dict(path=str(compiler.go), bytes=8, sha256=case.GO_SHA256,
                                              uid=os.getuid(), gid=os.getgid()))
            self.assertEqual(set(os.listdir("/proc/self/fd")), descriptors)
            self.assertEqual(foreign.stat(), original)
            self.assertEqual(foreign.read_bytes(), b"existing-host-go")

    def test_missing_or_nonboolean_initial_cleanup_authority_never_creates_stage(self):
        for value in (None, False, 0, 1, "true"):
            with self.subTest(value=value), self.fixture() as (ctx, _, _):
                if value is None: del ctx.global_build_cleanup_proved
                else: ctx.global_build_cleanup_proved = value
                with self.assertRaisesRegex(ValueError, "private build root authority"), case.prepared(ctx): pass
                self.assertFalse((ctx.root / case.NAME).exists())

    def test_restrictive_umask_changes_only_new_owned_distribution_modes(self):
        with self.fixture() as (ctx, _, _):
            original = os.umask(0o077)
            try:
                with case.prepared(ctx) as compiler:
                    self.assertEqual(compiler.path.stat().st_mode, stat.S_IFDIR | 0o700)
                    self.assertEqual((compiler.path / "go/bin").stat().st_mode, stat.S_IFDIR | 0o755)
                    self.assertEqual(compiler.go.stat().st_mode, stat.S_IFREG | 0o755)
            finally:
                os.umask(original)
            self.assertTrue(ctx.global_build_cleanup_proved)

    def test_creation_exception_restores_proof_only_for_original_absent_name(self):
        for effect in ("before", "after", "foreign"):
            with self.subTest(effect=effect), self.fixture() as (ctx, _, _):
                mkdir = os.mkdir
                if effect == "foreign":
                    (ctx.root / case.NAME).mkdir(); (ctx.root / case.NAME / "foreign").write_bytes(b"retain")
                def fail(name, *args, **kwargs):
                    if name != case.NAME: return mkdir(name, *args, **kwargs)
                    self.assertFalse(ctx.global_build_cleanup_proved)
                    if effect == "after": mkdir(name, *args, **kwargs)
                    raise OSError("creation fault")
                with mock.patch.object(case.os, "mkdir", side_effect=fail), self.assertRaises(OSError), case.prepared(ctx): pass
                self.assertIs(ctx.global_build_cleanup_proved, effect == "before")
                self.assertEqual((ctx.root / case.NAME).exists(), effect != "before")
                if effect == "foreign": self.assertEqual((ctx.root / case.NAME / "foreign").read_bytes(), b"retain")

    def test_download_and_partial_write_failure_retain_unknown_stage_without_descriptor_leaks(self):
        for effect in ("download", "write"):
            with self.subTest(effect=effect), self.fixture() as (ctx, _, _):
                before = set(os.listdir("/proc/self/fd")); write = os.write
                def partial(fd, body):
                    write(fd, body[:2]); raise OSError("partial write fault")
                target = mock.patch.object(case, "download", side_effect=OSError("download fault")) if effect == "download" else mock.patch.object(case.os, "write", side_effect=partial)
                with target, self.assertRaises(OSError), case.prepared(ctx): pass
                self.assertFalse(ctx.global_build_cleanup_proved)
                self.assertTrue((ctx.root / case.NAME).is_dir())
                self.assertEqual(set(os.listdir("/proc/self/fd")), before)
                if effect == "write": self.assertEqual((ctx.root / case.NAME / case.ARCHIVE).stat().st_size, 2)

    def test_wrong_archive_pin_or_length_is_refused_before_tar_parser(self):
        for field, value in (("ARCHIVE_SHA256", "0" * 64), ("ARCHIVE_BYTES", 1)):
            with self.subTest(field=field), self.fixture() as (ctx, _, _):
                with mock.patch.object(case, field, value), mock.patch.object(case.tarfile, "open") as parse:
                    with self.assertRaises(ValueError), case.prepared(ctx): pass
                parse.assert_not_called()
                self.assertFalse(ctx.global_build_cleanup_proved)

    def test_closed_tar_format_refuses_aliases_unknown_headers_counts_and_catalog(self):
        bad = [("../escape", tarfile.REGTYPE, 0o644, b"x", {}),
               ("go/link", tarfile.SYMTYPE, 0o755, b"", {}),
               ("go/link", tarfile.LNKTYPE, 0o755, b"", {}),
               ("go/fifo", tarfile.FIFOTYPE, 0o644, b"", {}),
               ("go/sparse", tarfile.GNUTYPE_SPARSE, 0o644, b"", {}),
               ("go/unknown", tarfile.REGTYPE, 0o644, b"x", {"comment": "foreign"}),
               ("go/link-path", tarfile.REGTYPE, 0o644, b"x", {"linkpath": "foreign"}),
               ("go/src/source.go", tarfile.REGTYPE, 0o644, b"duplicate", {}),
               ("go/set-id", tarfile.REGTYPE, 0o4755, b"x", {})]
        for extra in bad:
            with self.subTest(extra=extra[0:3]), self.fixture(MEMBERS + [extra]) as (ctx, _, _):
                with self.assertRaises(ValueError), case.prepared(ctx): pass
                self.assertFalse(ctx.global_build_cleanup_proved)
                self.assertFalse((ctx.root / "escape").exists())
        for field, value in (("MEMBERS", 1), ("CATALOG_SHA256", "0" * 64), ("PAYLOAD", 1), ("EXPANDED", 1024)):
            with self.subTest(field=field), self.fixture() as (ctx, _, _):
                with mock.patch.object(case, field, value), self.assertRaises(ValueError), case.prepared(ctx): pass
                self.assertFalse(ctx.global_build_cleanup_proved)

    def test_only_two_original_singleton_pax_path_headers_are_admitted(self):
        prefix = [("go/test", tarfile.DIRTYPE, 0o755, b"", {}),
                  ("go/test/fixedbugs", tarfile.DIRTYPE, 0o755, b"", {}),
                  ("go/test/fixedbugs/issue27836.dir", tarfile.DIRTYPE, 0o755, b"", {})]
        for name in sorted(case.PAX_PATHS):
            for effect in ("exact", "extra", "different-path"):
                headers = {"path": name}
                if effect == "extra": headers["mtime"] = "0"
                elif effect == "different-path": headers["path"] = name + "-different"
                with self.subTest(name=name, effect=effect), self.fixture(MEMBERS + prefix + [(name, tarfile.REGTYPE, 0o644, b"inert", headers)]) as (ctx, _, _):
                    if effect == "exact":
                        with case.prepared(ctx) as compiler: self.assertEqual((compiler.path / name).read_bytes(), b"inert")
                        self.assertTrue(ctx.global_build_cleanup_proved)
                    else:
                        with self.assertRaises(ValueError), case.prepared(ctx): pass
                        self.assertFalse(ctx.global_build_cleanup_proved)

    def test_complete_tree_or_parent_drift_refuses_cleanup_and_preserves_foreign_bytes(self):
        for effect in ("outer", "stage", "directory", "leaf", "bytes", "mode", "hardlink", "unknown", "missing"):
            with self.subTest(effect=effect), self.fixture() as (ctx, _, _):
                before = set(os.listdir("/proc/self/fd")); retained = None; entered = False
                with self.assertRaises((ValueError, OSError)):
                    with case.prepared(ctx) as compiler:
                        entered = True
                        target = {"outer": ctx.root, "stage": compiler.path, "directory": compiler.path / "go/bin", "leaf": compiler.go}.get(effect)
                        if target:
                            moved = target.with_name(target.name + "-original"); target.rename(moved)
                            if effect == "leaf": target.write_bytes(b"FOREIGN"); retained = target
                            else: target.mkdir(mode=0o700); retained = target / "foreign"; retained.write_bytes(b"FOREIGN")
                        elif effect == "bytes": compiler.go.write_bytes(b"changed")
                        elif effect == "mode": compiler.go.chmod(0o777)
                        elif effect == "hardlink": os.link(compiler.go, compiler.path / "foreign-link")
                        elif effect == "unknown": retained = compiler.path / "foreign"; retained.write_bytes(b"FOREIGN")
                        elif effect == "missing": compiler.go.unlink()
                        compiler.verify()
                self.assertTrue(entered)
                self.assertFalse(ctx.global_build_cleanup_proved)
                self.assertEqual(set(os.listdir("/proc/self/fd")), before)
                if retained is not None: self.assertEqual(retained.read_bytes(), b"FOREIGN")

    def test_stage_substitution_after_full_verification_refuses_before_unlink(self):
        with self.fixture() as (ctx, _, _):
            with self.assertRaises(ValueError):
                with case.prepared(ctx) as compiler:
                    walk = compiler.walk
                    def replace(parent, prefix="", *, remove=False):
                        if remove and not prefix:
                            compiler.path.rename(compiler.path.with_name("original-stage"))
                            compiler.path.mkdir(); (compiler.path / "foreign").write_bytes(b"FOREIGN")
                        return walk(parent, prefix, remove=remove)
                    compiler.walk = replace
            self.assertFalse(ctx.global_build_cleanup_proved)
            self.assertEqual((compiler.path / "foreign").read_bytes(), b"FOREIGN")
            self.assertEqual((compiler.path.with_name("original-stage") / "go/bin/go").read_bytes(), b"inert-go")

    def test_unproved_process_or_expired_budget_keeps_all_toolchain_members(self):
        for effect in ("process", "deadline"):
            with self.subTest(effect=effect), self.fixture() as (ctx, _, _):
                with self.assertRaises((ValueError, TimeoutError)):
                    with case.prepared(ctx) as compiler:
                        if effect == "process": ctx.process_cleanup_proved = False
                        else: ctx.budget.check = mock.Mock(side_effect=TimeoutError("expired"))
                self.assertFalse(ctx.global_build_cleanup_proved)
                self.assertTrue((compiler.path / "go/src/source.go").is_file())

    def test_unknown_or_ill_typed_receipt_cannot_attest_full_tree_or_cleanup(self):
        with self.fixture() as (ctx, _, _):
            with case.prepared(ctx) as compiler:
                for _ in range(4): compiler.verify()
                receipt = compiler.receipt
            executable = dict(path=str(compiler.go), bytes=8, sha256=case.GO_SHA256, uid=os.getuid(), gid=os.getgid())
            case.validate_receipt(receipt, executable)
            for field, value in (("cleanup_proved", 1), ("members", True), ("url", "https://foreign.invalid/go.tar.gz"),
                    ("catalog_sha256", "0" * 64), ("root", "/owned/../build-toolchain"), ("verifications", 5),
                    ("original_root", [1, 2, stat.S_IFDIR | 0o700, True, os.getgid()]), ("extra", "foreign")):
                wrong = copy.deepcopy(receipt); wrong[field] = value
                with self.subTest(field=field), self.assertRaises(ValueError): case.validate_receipt(wrong, executable)
            for field in receipt:
                wrong = copy.deepcopy(receipt); del wrong[field]
                with self.subTest(missing=field), self.assertRaises(ValueError): case.validate_receipt(wrong, executable)

    def test_shared_global_failure_fences_every_owned_deletion_and_raw_publication(self):
        for proof in (None, False, 0, 1, "true", True):
            with self.subTest(proof=proof), tempfile.TemporaryDirectory() as temporary:
                output = Path(temporary).resolve() / "capture"
                def source(ctx):
                    self.assertIs(ctx.global_build_cleanup_proved, True)
                    (ctx.private / "retained").write_bytes(b"owned-private-bytes")
                    if proof is None: del ctx.global_build_cleanup_proved
                    else: ctx.global_build_cleanup_proved = proof
                    raise ValueError("inert source refusal")
                with mock.patch.object(base.shared.native, "require_hosted"), \
                     mock.patch.object(base.AdmissionContext, "source_and_projects", source), \
                     mock.patch.object(base, "publish_with_readout") as publish:
                    self.assertEqual(base.run(output, collector=lambda *_: None, recipe="ordinary-global"), 1)
                summary = publish.call_args.args[0]
                self.assertIs(summary["cleanup_complete"], proof is True)
                self.assertEqual((output / "private/retained").exists(), proof is not True)
                if proof is not True:
                    self.assertIsNone(publish.call_args.args[1])
                    self.assertEqual(summary["commands"], [])
                    self.assertNotIn("global_build_receipt", summary)
                    self.assertNotIn("owned-private-bytes", json.dumps(summary))

    def test_real_private_stage_joins_two_build_captures_and_expires_before_return(self):
        import devin_global_lifecycle_admission as lifecycle
        for effect in ("success", "exit", "capture", "version", "source-drift"):
            with self.subTest(effect=effect), self.fixture() as (ctx, _, _):
                installer = ctx.root.parent / "installer"
                ctx.private = ctx.root / "private"; ctx.private.mkdir(mode=0o700)
                ctx.summary = dict(source_commit="a" * 40, source_tree="b" * 40, commands=[])
                ctx.identities = {}
                calls = []
                def run(argv, cwd, environment, output, label, **kwargs):
                    calls.append(label)
                    self.assertFalse(ctx.process_cleanup_proved)
                    self.assertFalse(ctx.global_build_cleanup_proved)
                    self.assertEqual(environment["GOROOT"], str(ctx.root / case.NAME / "go"))
                    self.assertEqual(environment["PATH"], str(ctx.root / case.NAME / "go/bin") + ":/usr/bin:/bin")
                    self.assertEqual((environment["GOENV"], environment["GOTOOLCHAIN"]), ("off", "local"))
                    self.assertEqual(kwargs, dict(timeout=10 if label == "build-go-version" else 30,
                                                 limit=1 << 20, memory_only=True))
                    if effect == "capture": raise OSError("inert unsettled capture")
                    row = dict(probe=label, argv=argv, cwd=str(cwd), exit=1 if effect == "exit" else 0,
                               error=None, elapsed_seconds=0.01)
                    out = b"go version go1.26.11 linux/amd64\n" if label == "build-go-version" else b""
                    if effect == "version": out = b"different Go version\n"
                    if effect == "source-drift": (ctx.root / case.NAME / "go/src/source.go").write_bytes(b"foreign edit")
                    if label == "build-uap": installer.write_bytes(b"inert-built-cli"); installer.chmod(0o755)
                    for kind, body in (("stdout", out), ("stderr", b"")):
                        row.update({kind + "_file": label + "." + kind + ".log", kind + "_bytes": len(body),
                                    kind + "_sha256": hashlib.sha256(body).hexdigest()})
                    return row, out, b""
                with mock.patch.object(base.shared, "run_probe", side_effect=run), \
                     mock.patch.object(lifecycle.shutil, "which", side_effect=AssertionError("shared compiler selected")):
                    if effect == "success":
                        self.assertEqual(lifecycle.build_installer(ctx, installer), (15, lifecycle.sha(b"inert-built-cli")))
                        lifecycle.validate_build_receipt(ctx.summary["global_build_receipt"], ctx.summary)
                        self.assertEqual(ctx.identities, {installer: lifecycle.sha(b"inert-built-cli")})
                    else:
                        with self.assertRaises((OSError, ValueError)): lifecycle.build_installer(ctx, installer)
                self.assertEqual(calls, ["build-go-version", "build-uap"] if effect == "success" else ["build-go-version"])
                settled = effect not in ("capture", "source-drift")
                self.assertIs(ctx.global_build_cleanup_proved, settled)
                self.assertEqual((ctx.root / case.NAME).exists(), not settled)
                self.assertEqual(ctx.summary["commands"], [])

    def test_fixed_https_acquisition_has_no_proxy_or_foreign_redirect(self):
        response = mock.MagicMock(url=case.URL)
        opener = mock.Mock(); opener.open.return_value = response
        with mock.patch.object(case.urllib.request, "build_opener", return_value=opener) as build:
            self.assertIs(case.download(SimpleNamespace(check=lambda: 12)), response)
        opener.open.assert_called_once_with(case.URL, timeout=12)
        proxy, redirect = build.call_args.args
        self.assertEqual(proxy.proxies, {})
        for target in ("http://dl.google.com/go/go1.26.11.linux-amd64.tar.gz", "https://foreign.invalid/go.tar.gz"):
            with self.subTest(target=target), self.assertRaisesRegex(ValueError, "redirect"):
                redirect.redirect_request(None, None, 302, None, None, target)
        response.url = "https://foreign.invalid/go.tar.gz"
        with mock.patch.object(case.urllib.request, "build_opener", return_value=opener):
            with self.assertRaisesRegex(ValueError, "origin"): case.download(SimpleNamespace(check=lambda: 12))
        response.close.assert_called_once()


if __name__ == "__main__": unittest.main()
