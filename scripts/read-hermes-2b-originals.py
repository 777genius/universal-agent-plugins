#!/usr/bin/env python3
"""Read exactly two retained failure streams; never run or extract artifact code."""
import hashlib
import io
import json
import os
from pathlib import Path
import re
import resource
import stat
import struct
import subprocess
import sys
import tempfile
import time
import zipfile

REPOSITORY = "777genius/universal-agent-plugins"
REPOSITORY_ID = 1193031149
BRANCH = "diagnostics/hermes-2b-original-readout-20261115"
SOURCE = "2b373a525bd401eaa5aa7580a43dfe1ed0a482be"
RUN = 43682181871
HELPER_BLOB = "aadd5107cbda79947b3a47437c95f3a7f3962645"
EMPTY = hashlib.sha256(b"").hexdigest()
SPECS = (
    {
        "label": "linux-project-inspect", "artifact": 14150304332, "job": 132714978943,
        "name": "hermes-project-install-Linux-X64-43682181871-1",
        "size": 10263642, "sha256": "72d69883855a062e6b5b9ea7096f8edac7812f3cbbf96036f521c23c68156b0a",
        "files": 783, "stream": "stderr", "bytes": 170,
        "stream_sha256": "48e0cc2e3e1be1de94c1798b610a6d2e3f2709ef9add14ba93dbfbd84a097b89",
        "argv": ["/home/runner/work/_temp/hermes-uap", "run", "--target", "hermes", "--scope", "project",
                 "--project-root", "/home/runner/work/_temp/hermes-project-install-zityro11/fixture/project-a",
                 "--inspect", "--confirm-client-start"],
        "cwd": "/home/runner/work/_temp/hermes-project-install-zityro11/fixture/project-a",
        "timeout": (1, 60),
    },
    {
        "label": "macos-cold-test", "artifact": 14151506433, "job": 132714978621,
        "name": "hermes-project-install-macOS-ARM64-43682181871-1",
        "size": 9834637, "sha256": "5c9f3e52bc30249f5514f77143ee748a82069c67ea3294abfb166357f235eed9",
        "files": 839, "stream": "stdout", "bytes": 225,
        "stream_sha256": "d92f70a937207d6ed0eb973bedfb19959982187a7fda48d27ef2569f6fdb60db",
        "argv": ["/Users/runner/work/_temp/hermes-cold",
                 "-test.run=^TestProjectHermesBundleColdRestoreAndRepair$",
                 "-test.v", "-test.count=1", "-test.timeout=6m"],
        "cwd": "/Users/runner/work/universal-agent-plugins/universal-agent-plugins",
        "timeout": (390, 390),
    },
)


def require(condition):
    if not condition:
        raise ValueError("fixed original evidence refused")


def json_document(body):
    def pairs(items):
        result = {}
        for key, value in items:
            require(key not in result)
            result[key] = value
        return result
    return json.loads(body.decode("utf-8"), object_pairs_hook=pairs,
                      parse_constant=lambda _: require(False))


def borrowed_reader():
    # Load only these already reviewed repository bytes, never any ZIP member.
    path = Path(__file__).with_name("read-authoring-evidence-zip.py")
    body = path.read_bytes()
    require(hashlib.sha1(b"blob " + str(len(body)).encode() + b"\0" + body).hexdigest() == HELPER_BLOB)
    namespace = {"__name__": "fixed_original_zip_helpers"}
    exec(compile(body, str(path), "exec"), namespace)
    return namespace["checked_bytes"], namespace["check_expansion"]


def api_file(endpoint, destination, deadline):
    require(time.monotonic() < deadline)
    environment = dict(os.environ)
    environment.pop("GH_DEBUG", None)
    environment.update(GH_HOST="github.com", GH_PROMPT_DISABLED="1")
    with destination.open("xb") as output, destination.with_suffix(destination.suffix + ".errors").open("xb") as errors:
        result = subprocess.run(
            ["gh", "api", "--hostname", "github.com", "-H", "X-GitHub-Api-Version: 2022-11-28",
             "-H", "Accept: application/vnd.github+json", endpoint],
            stdout=output, stderr=errors, env=environment,
            timeout=min(60, deadline - time.monotonic()), check=False)
    require(result.returncode == 0 and time.monotonic() < deadline)


def artifact_bytes(spec, root, deadline, checked_bytes):
    endpoint = f"repos/{REPOSITORY}/actions/artifacts/{spec['artifact']}"
    metadata_path = root / (spec["label"] + ".json")
    api_file(endpoint, metadata_path, deadline)
    with metadata_path.open("rb") as handle:
        metadata_body = handle.read((64 << 10) + 1)
    require(len(metadata_body) <= 64 << 10)
    metadata = json_document(metadata_body)
    require(type(metadata) is dict)
    require(metadata.get("id") == spec["artifact"] and metadata.get("name") == spec["name"]
            and metadata.get("size_in_bytes") == spec["size"] and metadata.get("expired") is False
            and metadata.get("digest") == "sha256:" + spec["sha256"])
    run = metadata.get("workflow_run")
    require(type(run) is dict and run.get("id") == RUN and run.get("head_sha") == SOURCE
            and run.get("repository_id") == run.get("head_repository_id") == REPOSITORY_ID
            and run.get("head_branch") == "feat/hermes-project-install-from-878-20261115")
    archive_path = root / (spec["label"] + ".zip")
    api_file(endpoint + "/zip", archive_path, deadline)
    return checked_bytes(archive_path, spec["sha256"], spec["size"])


def zip_index(body, archive, spec):
    # Allocation was bounded by the ordinary EOCD before ZipFile construction.
    infos = archive.infolist()
    require(len(infos) <= 2048)
    names, intervals, starts = {}, [], {}
    total = 0
    cd_offset = struct.unpack_from("<L", body, len(body) - 6)[0]
    for info in infos:
        name = info.filename
        parts = name.rstrip("/").split("/")
        require(info.orig_filename == name and 0 < len(name.encode("utf-8")) <= 1024
                and len(parts) <= 32 and all(part not in ("", ".", "..") for part in parts)
                and not any(char in name for char in ("\0", "\\", ":"))
                and name not in names and len(info.extra) <= 4096 and len(info.comment) <= 4096)
        require(info.flag_bits & ~0x808 == 0 and info.compress_type in (zipfile.ZIP_STORED, zipfile.ZIP_DEFLATED))
        mode = info.external_attr >> 16
        require(stat.S_IFMT(mode) in (0, stat.S_IFDIR if info.is_dir() else stat.S_IFREG)
                and not mode & 0o7000 and not info.external_attr & 0x400)
        require(0 <= info.file_size <= 64 << 20 and 0 <= info.compress_size <= len(body))
        total += info.file_size
        require(total <= 256 << 20 and (not info.is_dir() or info.file_size == 0))
        offset = info.header_offset
        require(0 <= offset <= cd_offset - 30 and body[offset:offset + 4] == b"PK\x03\x04")
        flags, method = struct.unpack_from("<HH", body, offset + 6)
        crc, compressed, expanded = struct.unpack_from("<LLL", body, offset + 14)
        name_size, extra_size = struct.unpack_from("<HH", body, offset + 26)
        require((flags, method) == (info.flag_bits, info.compress_type)
                and name_size <= 1024 and extra_size <= 4096)
        require((crc, compressed, expanded) == (info.CRC, info.compress_size, info.file_size)
                or flags & 8 and (crc, compressed, expanded) == (0, 0, 0))
        start = offset + 30 + name_size + extra_size
        require(start <= cd_offset and
                body[offset + 30:offset + 30 + name_size].decode("utf-8" if flags & 0x800 else "cp437") == name)
        end = start + info.compress_size
        require(end <= cd_offset)
        if flags & 8:
            if body[end:end + 4] == b"PK\x07\x08":
                end += 4
            require(end + 12 <= cd_offset and
                    struct.unpack_from("<LLL", body, end) == (info.CRC, info.compress_size, info.file_size))
            end += 12
        names[name], starts[name] = info, start
        intervals.append((offset, end))
    intervals.sort()
    require(intervals and intervals[0][0] == 0 and intervals[-1][1] == cd_offset
            and all(left[1] == right[0] for left, right in zip(intervals, intervals[1:])))
    require(sum(not info.is_dir() for info in infos) == spec["files"])
    return names, starts


def selected_member(body, archive, info, start, maximum, expansion):
    require(not info.is_dir() and info.file_size <= maximum)
    expansion(body, start, info)
    with archive.open(info) as member:
        result = member.read(maximum + 1)
    require(len(result) == info.file_size and len(result) <= maximum)
    return result


def joined_stream(spec, body, expansion, deadline):
    require(len(body) >= 22 and body[-22:-18] == b"PK\x05\x06")
    disk, start_disk, count_disk, count, cd_size, cd_offset, comment = struct.unpack_from("<4H2LH", body, len(body) - 18)
    require(disk == start_disk == comment == 0 and count_disk == count and 0 < count <= 2048
            and cd_size <= 2 << 20 and cd_offset + cd_size == len(body) - 22)
    with zipfile.ZipFile(io.BytesIO(body)) as archive:
        require(len(archive.infolist()) == count)
        names, starts = zip_index(body, archive, spec)
        pattern = r"hermes-project-supervision-[a-z0-9_]{8}/native\." + spec["stream"]
        candidates = [info for name, info in names.items()
                      if re.fullmatch(pattern, name) and info.file_size == spec["bytes"]]
        require(0 < len(candidates) <= 32)
        matches = []
        for info in candidates:
            require(time.monotonic() < deadline)
            data = selected_member(body, archive, info, starts[info.filename], spec["bytes"], expansion)
            if hashlib.sha256(data).hexdigest() == spec["stream_sha256"]:
                matches.append((info.filename.rsplit("/", 1)[0], data))
        require(len(matches) == 1)
        prefix, data = matches[0]
        documents = {}
        for leaf in ("record.json", "request.json"):
            name = prefix + "/" + leaf
            require(name in names and time.monotonic() < deadline)
            documents[leaf] = json_document(selected_member(body, archive, names[name], starts[name], 16 << 10, expansion))
        record, request = documents["record.json"], documents["request.json"]
        require(type(request) is dict and set(request) == {"argv", "directory", "timeout_seconds", "limit"}
                and request["argv"] == spec["argv"] and request["directory"] == spec["cwd"]
                and type(request["timeout_seconds"]) is int
                and spec["timeout"][0] <= request["timeout_seconds"] <= spec["timeout"][1]
                and type(request["limit"]) is int and request["limit"] == 1 << 20)
        require(type(record) is dict and set(record) == {
            "argv", "cwd", "exit_code", "cleanup_complete", "no_child_started", "error", "streams"}
            and record["argv"] == request["argv"] and record["cwd"] == request["directory"]
            and type(record["exit_code"]) is int and record["exit_code"] == 1
            and record["cleanup_complete"] is True and record["no_child_started"] is False
            and record["error"] is None)
        expected = {name: {"bytes": spec["bytes"] if name == spec["stream"] else 0,
                          "sha256": spec["stream_sha256"] if name == spec["stream"] else EMPTY}
                    for name in ("stdout", "stderr")}
        streams = record["streams"]
        require(type(streams) is dict and set(streams) == {"stdout", "stderr"}
                and all(type(row) is dict and set(row) == {"bytes", "sha256"}
                        and type(row["bytes"]) is int and type(row["sha256"]) is str
                        for row in streams.values()) and streams == expected)
        other = "stderr" if spec["stream"] == "stdout" else "stdout"
        empty = prefix + "/native." + other
        require(empty in names and names[empty].file_size == 0 and names[empty].CRC == 0)
        require(selected_member(body, archive, names[empty], starts[empty], 0, expansion) == b"")
    require(time.monotonic() < deadline)
    message = data.decode("utf-8")
    require(all(ord(char) >= 32 or char in "\n\r\t" for char in message))
    require(re.search(r"(?i)://|authorization\s*:|(?:password|token|secret|api[_-]?key)\s*[=:]", message) is None)
    for name in ("GH_TOKEN", "GITHUB_TOKEN"):
        token = os.environ.get(name, "")
        require(not token or token not in message)
    return {"source": SOURCE, "run": RUN, "job": spec["job"], "artifact": spec["artifact"],
            "archive_sha256": spec["sha256"], "stage": spec["label"], "exit_code": 1,
            "cleanup_complete": True, "stream": spec["stream"], "bytes": len(data),
            "sha256": spec["stream_sha256"], "message": message}


def main():
    require(len(sys.argv) == 1 and os.environ.get("GITHUB_REPOSITORY") == REPOSITORY
            and os.environ.get("GITHUB_REF") == "refs/heads/" + BRANCH
            and os.environ.get("GITHUB_EVENT_NAME") == "push")
    os.umask(0o077)
    # Bounds include each API response and stderr file; neither is echoed.
    resource.setrlimit(resource.RLIMIT_FSIZE, (16 << 20, 16 << 20))
    deadline = time.monotonic() + 300
    checked_bytes, expansion = borrowed_reader()
    failed = False
    with tempfile.TemporaryDirectory(prefix="hermes-2b-readout-", dir=os.environ["RUNNER_TEMP"]) as directory:
        for spec in SPECS:
            try:
                body = artifact_bytes(spec, Path(directory), deadline, checked_bytes)
                result = joined_stream(spec, body, expansion, deadline)
                print("HERMES_ORIGINAL_READOUT " + json.dumps(result, ensure_ascii=True, sort_keys=True), flush=True)
            except Exception:
                failed = True
                print("HERMES_ORIGINAL_READOUT_REFUSED " + spec["label"], file=sys.stderr, flush=True)
    require(not failed)


if __name__ == "__main__":
    try:
        main()
    except Exception:
        print("HERMES_ORIGINAL_READOUT_REFUSED", file=sys.stderr)
        sys.exit(1)
