#!/usr/bin/env python3
"""Read only two immutable Hermes originals; never execute a native client."""
import argparse
import hashlib
import io
import json
from pathlib import Path
import re
import runpy
import stat
import struct
import zipfile
import zlib

check_expansion = runpy.run_path(str(Path(__file__).with_name('read-authoring-evidence-zip.py')),
                               run_name='hermes_original_zip_checks')['check_expansion']

CASES = {
    "mac-b79": (14046328482, 9945, "ed0d57c5bea2e29991d7ff59d752d62209ce7842088bad2291d880a22f3af877",
                "43439520227", "b79c2ac0b0bf2021b3697bb691108df24438c01d", "6e334b74554872dbb44aaab42847f63e739cd1d6", "macOS"),
    "win-0a1": (14052063757, 1077, "1be40362fd5f15c4e71519e2d73eb0dcbc102a92add9453a44496a9e59c01c55",
                "43449616323", "0a1ecf16d41708b5c3cfc4add1e3cf8f0c6c96d8", "19689eca4603a7b2d423e3fcbe6cff647395e24a", "Windows"),
}
PIN = "2bd378ed1b8d6bf2192cb26e0a55ff3091e6bd19"
PIN_TREE = "675b4cea2d2aef80460f07235b0e27e6c6922743"
COMMANDS = ({prefix + scope for prefix in ("git-", "guard-", "list-", "test-", "remove-mcp-", "removed-list-", "removed-test-")
             for scope in ("a", "b")} | {"test-a-restart", "list-user", "version-a"})
MEMBERS = ({name + suffix for name in COMMANDS for suffix in (".stdout", ".stderr", ".status.json")}
           | {"original-user-baseline.json", "original-user-final.json", "source-guards.json", "profile-endpoints.json",
              "profile-command-deltas.json", "bootstrap-census-request.json", "bootstrap-census.json",
              "profile-admission.json", "lifecycle.stdout", "lifecycle.stderr", "owned-start-gate"})
FLAGS = ("capture_complete", "admission_qualified", "production_project_mcp", "vendor_authentication",
         "real_model_inference", "tools_call", "source_install_custom_root_qualified")
ERRORS = ("RuntimeError", "ValueError", "AssertionError", "FileNotFoundError", "PermissionError",
          "PayloadHardlinkRefusal", "TimeoutExpired", "JSONDecodeError")
GUARDS = ("native profile observation failed", "native profile MCP ledger differs",
          "shared metadata effects require independent admission", "OS User home changed",
          "payload hardlink escapes runtime", "immutable bundled runtime changed",
          "original User settings/data copied into custom root", "file changed during bounded capture",
          "inventory membership bound", "inventory byte bound", "bounded ordinary single-link file required",
          "file changed before open", "inventory root replaced")
FRAME = re.compile(r'^  File "(?:[^"\r\n]*[/\\])?(hermes-profile-admission\.py|hermes-noauth-tool-call\.py)", line ([1-9][0-9]{0,4}), in [A-Za-z_][A-Za-z_0-9]*$')

def require(ok):
    if not ok:
        raise ValueError("Hermes original archive refused")

def unique(pairs):
    result = {}
    for key, value in pairs:
        require(key not in result)
        result[key] = value
    return result

def document(body):
    require(len(body) <= 1 << 20)
    return json.loads(body.decode("utf-8"), object_pairs_hook=unique,
                      parse_constant=lambda _: require(False))

def unpack(body, expected_size, expected_hash):
    require(type(body) is bytes and len(body) == expected_size and hashlib.sha256(body).hexdigest() == expected_hash)
    # No comment/ZIP64; allocation authority is checked before ZipFile constructs entries.
    require(len(body) >= 22 and body[-22:-18] == b"PK\x05\x06")
    disk, start_disk, on_disk, count, central_size, central_offset, comment = struct.unpack("<4H2IH", body[-18:])
    require(disk == start_disk == comment == 0 and on_disk == count and 0 < count <= len(MEMBERS)
            and count != 65535 and central_size <= 16384 and central_offset + central_size == len(body) - 22)
    result, total, intervals = {}, 0, []
    with zipfile.ZipFile(io.BytesIO(body)) as archive:
        require(len(archive.infolist()) == count and not archive.comment)
        for info in archive.infolist():
            require(info.filename in MEMBERS and info.filename not in result and not info.is_dir()
                    and not info.flag_bits & 1 and info.compress_type in (zipfile.ZIP_STORED, zipfile.ZIP_DEFLATED)
                    and len(info.extra) <= 4096 and info.file_size <= 1 << 20 and info.compress_size <= len(body))
            offset = 0
            while offset < len(info.extra):
                require(offset + 4 <= len(info.extra))
                kind, size = struct.unpack_from("<HH", info.extra, offset)
                offset += 4
                require(kind != 1 and offset + size <= len(info.extra))
                offset += size
            mode = info.external_attr >> 16
            require(not stat.S_ISLNK(mode) and (not stat.S_IFMT(mode) or stat.S_ISREG(mode)))
            offset = info.header_offset
            require(0 <= offset and offset + 30 <= central_offset and body[offset:offset + 4] == b"PK\x03\x04")
            flags, method = struct.unpack_from("<HH", body, offset + 6)
            crc, compressed, expanded = struct.unpack_from("<III", body, offset + 14)
            require((flags, method) == (info.flag_bits, info.compress_type)
                    and ((crc, compressed, expanded) == (info.CRC, info.compress_size, info.file_size)
                         or flags & 8 and (crc, compressed, expanded) == (0, 0, 0)))
            name_size, extra_size = struct.unpack_from("<HH", body, offset + 26)
            require(extra_size == 0 and body[offset + 30:offset + 30 + name_size] == info.filename.encode("utf-8"))
            start = offset + 30 + name_size
            end = start + info.compress_size
            require(end <= central_offset)
            check_expansion(body, start, info)
            if flags & 8:
                if body[end:end + 4] == b"PK\x07\x08":
                    end += 4
                require(end + 12 <= central_offset and struct.unpack_from("<III", body, end)
                        == (info.CRC, info.compress_size, info.file_size))
                end += 12
            intervals.append((offset, end))
            total += info.file_size
            require(total <= 16 << 20)
            with archive.open(info) as source:
                data = source.read((1 << 20) + 1)
                require(len(data) == info.file_size and not source.read(1))
            result[info.filename] = data  # ZipExtFile verifies CRC on full bounded read.
    intervals.sort()
    require(intervals[0][0] == 0 and intervals[-1][1] == central_offset
            and all(a[1] == b[0] for a, b in zip(intervals, intervals[1:])))
    return result

def alias(value):
    require(type(value) is dict and set(value) == {"schema_version", "measurement_only", "member", "member_sha256",
            "hash_encoding", "device", "inode", "links", "captured_aliases", "uid", "gid", "integer_saturated"})
    require(value["schema_version"] == 1 and type(value["schema_version"]) is int
            and value["measurement_only"] is True and type(value["integer_saturated"]) is bool
            and value["hash_encoding"] == "utf-8-surrogatepass"
            and type(value["member_sha256"]) is str and re.fullmatch("[0-9a-f]{64}", value["member_sha256"]))
    member = value["member"]
    require(member is None or (type(member) is str and member and ":" not in member and "\\" not in member
            and all(part not in ("", ".", "..") for part in member.split("/"))
            and len(json.dumps(member, ensure_ascii=True).encode("ascii")) <= 128
            and hashlib.sha256(member.encode("utf-8", "surrogatepass")).hexdigest() == value["member_sha256"]))
    numbers = ("device", "inode", "links", "captured_aliases", "uid", "gid")
    require(all(value[key] is None or type(value[key]) is int and 0 <= value[key] < 1 << 64 for key in numbers)
            and value["integer_saturated"] is any(value[key] is None for key in numbers)
            and len(json.dumps(value, ensure_ascii=True).encode("ascii")) <= 1024)
    return {key: value[key] for key in ("member", "member_sha256", *numbers, "integer_saturated")}

def project(files, case):
    _, _, _, run, head, tree, os_name = case
    require("profile-admission.json" in files)
    proof = document(files["profile-admission.json"])
    require(type(proof) is dict and proof.get("source") == proof.get("source_after")
            == {"commit": head, "tree": tree, "dirty": ""}
            and proof.get("hermes_source") == proof.get("hermes_source_after")
            == {"commit": PIN, "tree": PIN_TREE, "dirty": ""}
            and proof.get("run_id") == run and proof.get("run_attempt") == "1"
            and proof.get("runner_os") == os_name and proof.get("status") == "failed"
            and proof.get("cleanup") == "completed" and proof.get("runtime_distribution") == "bundled"
            and all(proof.get(key) is False for key in FLAGS))
    streams = proof.get("streams")
    require(type(streams) is dict and set(streams) == set(files) - {"profile-admission.json"})
    for name, record in streams.items():
        require(type(record) is dict and set(record) == {"bytes", "sha256"}
                and type(record["bytes"]) is int and record["bytes"] == len(files[name])
                and record["sha256"] == hashlib.sha256(files[name]).hexdigest())
    runtime_present = "runtime_before" in proof or "runtime_after" in proof
    runtime_stable = "runtime_before" in proof and "runtime_after" in proof and proof["runtime_before"] == proof["runtime_after"]
    error = proof.get("error")
    require(type(error) is str and len(error) <= 400)
    error_class = next((name for name in ERRORS if error.startswith(name + ": ")), "unknown")
    guard = next((index for index, text in enumerate(GUARDS) if error.endswith(": " + text)), None)
    frames, stream_error_classes, stream_guards = [], set(), set()
    for name in ("lifecycle.stderr", *(command + ".stderr" for command in sorted(COMMANDS))):
        raw = files.get(name)
        if raw is None:
            continue
        lines = raw.decode("utf-8", errors="strict").splitlines()
        require(len(lines) <= 32768)
        for line in lines:
            for kind in ERRORS:
                if line.startswith(kind + ": "):
                    stream_error_classes.add(kind)
                    for index, text in enumerate(GUARDS):
                        if line == kind + ": " + text:
                            stream_guards.add(index)
                    if line.startswith(kind + ": inventory member refused: "):
                        stream_guards.add("inventory-member-refused")
            match = FRAME.fullmatch(line)
            if match:
                frames.append({"file": match[1], "line": int(match[2])})
                require(len(frames) <= 64)
    known = [name for name in COMMANDS if name + ".status.json" in files]
    # The original command-delta ledger is the only chronological command authority.
    last = None
    if "profile-command-deltas.json" in files:
        ledger = document(files["profile-command-deltas.json"])
        rows = ledger.get("commands") if type(ledger) is dict else None
        require(type(rows) is list and len(rows) <= len(COMMANDS))
        for row in rows:
            require(type(row) is dict and row.get("label") in COMMANDS)
        if rows:
            last = rows[-1]["label"]
    result = {"kind": "hermes-fixed-original-readout", "source": head, "run": run,
              "failed": True, "native_qualified": False, "error_class": error_class,
              "guard_index": guard, "traceback_frames": frames, "last_command": last,
              "known_status_count": len(known), "stream_joins": len(streams),
              "stream_error_classes": sorted(stream_error_classes),
              "stream_guards": sorted(stream_guards, key=str), "runtime_present": runtime_present,
              "runtime_stable": runtime_stable, "runtime_error_present": "runtime_error" in proof}
    if "payload_hardlink_refusal" in proof:
        require(os_name == "Windows" and "runtime_before" not in proof and proof.get("native_family_elapsed_seconds") is None)
        result["payload_hardlink_refusal"] = alias(proof["payload_hardlink_refusal"])
    return result

def audit(body, name):
    case = CASES[name]
    files = unpack(body, case[1], case[2])
    result = project(files, case)
    result.update(artifact_id=case[0], archive_bytes=case[1], archive_sha256=case[2],
                  member_count=len(files), crc_verified=True)
    return result

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("case", choices=CASES)
    parser.add_argument("archive", type=Path)
    args = parser.parse_args()
    try:
        with args.archive.open("rb") as source:
            body = source.read(CASES[args.case][1] + 1)
        result = audit(body, args.case)
    except (ValueError, OSError, UnicodeError, KeyError, TypeError, zipfile.BadZipFile, RuntimeError, struct.error, zlib.error):
        result = {"kind": "hermes-fixed-original-readout", "case": args.case, "refused": True, "native_qualified": False}
    print(json.dumps(result, sort_keys=True, separators=(",", ":")))
    return 1 if result.get("refused") is True else 0

if __name__ == "__main__":
    raise SystemExit(main())
