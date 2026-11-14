"""Synthetic provider ZIP tests; no native or release qualification evidence."""
import hashlib
import importlib.util
import io
import os
from pathlib import Path
import stat
import struct
import tempfile
import unittest
from unittest.mock import patch
import warnings
import zipfile
import zlib

spec = importlib.util.spec_from_file_location("authoring_zip", Path(__file__).with_name("read-authoring-evidence-zip.py"))
reader = importlib.util.module_from_spec(spec)
spec.loader.exec_module(reader)
NAMES = sorted(reader.NATIVE_FILES)


def archive(entries=None, compression=zipfile.ZIP_DEFLATED):
    buffer = io.BytesIO()
    with warnings.catch_warnings(), zipfile.ZipFile(buffer, "w", compression) as output:
        warnings.simplefilter("ignore", UserWarning)
        for name, data in entries or [(name, b'{"fixture":true}\n') for name in NAMES]:
            output.writestr(name, data)
    return buffer.getvalue()


class EvidenceZIP(unittest.TestCase):
    def setUp(self):
        # Retain bounded fixtures. No recursive deletion or cleanup-guard bypass.
        self.root = Path(tempfile.mkdtemp(prefix="authoring-evidence-zip-"))
        self.source = self.root / "provider.zip"
        self.output = self.root / "extracted"

    def extract(self, body, digest=None, size=None, names=NAMES, kind="native"):
        self.source.write_bytes(body)
        return reader.extract(self.source, digest or hashlib.sha256(body).hexdigest(),
                              size if size is not None else len(body), self.output, kind, names)

    def reject(self, body, message, **kwargs):
        with self.assertRaisesRegex((ValueError, zipfile.BadZipFile, OSError), message):
            self.extract(body, **kwargs)
        self.assertFalse(os.path.lexists(self.output), "invalid ZIP must cause zero extraction writes")

    def test_complete_native_closure(self):
        result = self.extract(archive())
        self.assertEqual(result["files"], NAMES)
        for name in NAMES:
            self.assertEqual((self.output / name).read_bytes(), b'{"fixture":true}\n')
            self.assertEqual(stat.S_IMODE((self.output / name).stat().st_mode), 0o600)
        with self.assertRaisesRegex(ValueError, "exclusive"):
            self.extract(archive())

    def test_preparation_closure_and_explicit_directories(self):
        names = ["candidate/candidate.json", "candidate-identity.json", "pair-prepared.json", "preparation-run.json"]
        for product in ("agentplugins", "plugin-kit-ai"):
            names += [f"{product}/{name}" for name in ["checksums.txt", "release-manifest.json", *[f"asset-{i}" for i in range(6)]]]
        entries = [(name, b"fixture") for name in names]
        entries += [(name + "/", b"") for name in ("candidate", "agentplugins", "plugin-kit-ai")]
        self.assertEqual(self.extract(archive(entries), names=names, kind="preparation")["files"], sorted(names))

    def test_swapped_checked_and_extracted_zip(self):
        checked = archive()
        swapped = archive([(name, b"different") for name in NAMES])
        self.reject(swapped, "digest/size", digest=hashlib.sha256(checked).hexdigest())

    def test_snapshot_remains_same_when_source_replaced_after_hash(self):
        original = reader.checked_bytes
        checked = archive()
        def replace(*args):
            body = original(*args)
            self.source.write_bytes(b"replaced after immutable snapshot")
            return body
        with patch.object(reader, "checked_bytes", replace):
            self.extract(checked)
        self.assertEqual((self.output / NAMES[0]).read_bytes(), b'{"fixture":true}\n')

    def test_wrong_size(self):
        self.reject(archive(), "size mismatch", size=1)

    def test_paths_duplicates_aliases_and_missing_subjects(self):
        mutations = ["../escape", "/absolute", "C:/escape", "a\\b", "./host.json", "a//b", "a/../b",
                     "host.json.", "host.json ", "NUL.txt", "COM1", "a:stream", "é.json", "a\x00tail",
                     "host.json", "HOST.JSON", "extra.json", "extra/", "a/b/c/d"]
        for name in mutations:
            with self.subTest(name=name):
                self.reject(archive([(n, b"{}") for n in NAMES] + [(name, b"{}")]),
                            "path|alias|duplicate|unexpected|metadata")
        self.reject(archive([(n, b"{}") for n in NAMES[:-1]]), "closure")

    def test_links_special_modes_and_metadata(self):
        for mode in (stat.S_IFLNK | 0o777, stat.S_IFIFO | 0o600, stat.S_IFCHR | 0o600,
                     stat.S_IFSOCK | 0o600, stat.S_IFREG | 0o4600, stat.S_IFDIR | 0o700):
            with self.subTest(mode=mode):
                info = zipfile.ZipInfo(NAMES[0]); info.external_attr = mode << 16
                self.reject(archive([(info, b"{}"), *[(n, b"{}") for n in NAMES[1:]]]), "type")
        for field, value in (("extra", b"\x01\x00\x00\x00"), ("comment", b"alias"), ("external_attr", 0x400)):
            with self.subTest(field=field):
                info = zipfile.ZipInfo(NAMES[0]); setattr(info, field, value)
                self.reject(archive([(info, b"{}"), *[(n, b"{}") for n in NAMES[1:]]]), "metadata|reparse")

    def test_bounded_counts_sizes_ratios_and_encoding(self):
        self.reject(archive([(f"entry-{i}", b"{}") for i in range(65)]), "count")
        self.reject(archive([(n, b"0" * 100000) for n in NAMES]), "ratio")
        with patch.object(reader, "MAX_FILE", 1):
            self.reject(archive(), "size")
        with patch.object(reader, "MAX_TOTAL", 20):
            self.reject(archive(), "total")
        self.reject(archive(compression=zipfile.ZIP_BZIP2), "encoding")
        self.reject(archive([(n, b"") for n in NAMES]), "empty")

    def test_crc_truncation_local_name_and_header_overlap(self):
        body = bytearray(archive(compression=zipfile.ZIP_STORED))
        body[30 + len(NAMES[0])] ^= 1
        self.reject(bytes(body), "CRC")
        self.reject(archive()[:-1], "terminator")
        body = bytearray(archive()); body[30] ^= 1
        self.reject(bytes(body), "name")
        body = bytearray(archive()); offset = body.index(b"PK\x01\x02")
        struct.pack_into("<L", body, offset + 42, 1)
        self.reject(bytes(body), "local header")

    def test_streaming_data_descriptors_and_hidden_local_members(self):
        class Stream(io.BytesIO):
            def seekable(self):
                return False

            def seek(self, *args):
                raise io.UnsupportedOperation("stream")
        stream = Stream()
        with zipfile.ZipFile(stream, "w", zipfile.ZIP_DEFLATED) as output:
            for name in NAMES:
                output.writestr(name, b"fixture")
        body = stream.getvalue()
        self.assertIn(b"PK\x07\x08", body)
        self.extract(body)
        self.output = self.root / "descriptor-rejected"
        corrupt = bytearray(body)
        corrupt[corrupt.index(b"PK\x07\x08") + 4] ^= 1
        self.reject(bytes(corrupt), "descriptor mismatch")
        # An unlisted local payload in a gap must not be hidden by a matching
        # central file inventory; adjust provider digest and offsets coherently.
        ordinary = archive(compression=zipfile.ZIP_STORED)
        offset = ordinary.index(b"PK\x01\x02")
        hidden = b"unlisted local bytes"
        corrupt = bytearray(ordinary[:offset] + hidden + ordinary[offset:])
        struct.pack_into("<L", corrupt, len(corrupt) - 6, offset + len(hidden))
        self.reject(bytes(corrupt), "hidden")

    def test_local_and_central_encoding_must_agree(self):
        body = bytearray(archive())
        struct.pack_into("<H", body, 6, 1)  # local encryption flag only
        self.reject(bytes(body), "local encoding")
        body = bytearray(archive())
        struct.pack_into("<L", body, 18, 0)  # false local compressed size
        self.reject(bytes(body), "local size")

    def test_forged_size_and_crc_cannot_hide_deflate_expansion(self):
        body = bytearray(archive())
        central = body.index(b"PK\x01\x02")
        forged_crc = zlib.crc32(b"{")
        # Both headers consistently claim just the first byte. ZipExtFile alone
        # accepts the truncated member with this forged CRC; intake must not.
        struct.pack_into("<L", body, 14, forged_crc)
        struct.pack_into("<L", body, 22, 1)
        struct.pack_into("<L", body, central + 16, forged_crc)
        struct.pack_into("<L", body, central + 24, 1)
        self.reject(bytes(body), "actual ZIP expansion")

    def test_existing_destination_never_overwrites_sentinel(self):
        self.output.mkdir()
        sentinel = self.output / "host.json"
        sentinel.write_bytes(b"owned preexisting bytes")
        with self.assertRaisesRegex(ValueError, "exclusive"):
            self.extract(archive())
        self.assertEqual(sentinel.read_bytes(), b"owned preexisting bytes")

    def test_archive_and_destination_links(self):
        body = archive(); other = self.root / "other.zip"; other.write_bytes(body)
        self.source.symlink_to(other)
        with self.assertRaisesRegex(ValueError, "unaliased"):
            reader.extract(self.source, hashlib.sha256(body).hexdigest(), len(body), self.output, "native", NAMES)
        self.assertFalse(self.output.exists())
        linked = self.root / "linked.zip"; os.link(other, linked)
        with self.assertRaisesRegex(ValueError, "unaliased"):
            reader.extract(linked, hashlib.sha256(body).hexdigest(), len(body), self.output, "native", NAMES)
        destination = self.root / "alias"; destination.symlink_to(self.root, target_is_directory=True)
        with self.assertRaisesRegex(ValueError, "real directory"):
            reader.safe_directory(destination)


if __name__ == "__main__":
    unittest.main()
