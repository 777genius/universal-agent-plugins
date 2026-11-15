#!/usr/bin/env python3
"""Read one fixed Devin failure artifact; never extract or execute its content."""
import hashlib
import io
import json
import math
import os
from pathlib import Path
import re
import resource
import struct
import sys
import tempfile
import time
import zipfile

REPOSITORY = "777genius/universal-agent-plugins"
REPOSITORY_ID = 1193031149
BRANCH = "diagnostics/devin-843-original-readout-20261115"
SOURCE = "843ffaea3ed7b5aa2beafb205d48b27586e42f69"
SOURCE_TREE = "faf21a276c567c9b1c3b13fb3c2a11d5f3cfd379"
RUN = 43680575924
JOB = 132741363291
ARTIFACT = 14153842913
ARTIFACT_NAME = "devin-global-lifecycle-43680575924-1"
ARCHIVE_BYTES = 56792
ARCHIVE_SHA256 = "0374906b0abc5293fcea54ed05d1212840ad12c54accdc6bcf6be6d348338405"
READER_BLOB = "cdde17a7664fd1569318609bdf7ba57cfba2a365"
ROOT = "/home/runner/work/_temp/devin-global-lifecycle"
SOURCE_DIRECTORY = "/home/runner/work/universal-agent-plugins/universal-agent-plugins"
BINARY = "3f3fb99f472d92f7852be3775ac2b66b5d60baf26f3441b02d54c56b160bc30d"
PRELUDE_SHA256 = "4271c11033b288f7bb8fd13d37d0a7231f751e84ce201b34850bfec485037eeb"
SOURCE_LABELS = ("source-commit", "source-tree", "source-clean", "git-init-a", "git-init-b", "git-init-c")
METADATA_LABELS = ("version", "help", "mcp-help", "mcp-list-help", "skills-list-help",
                   "mcp-a", "skills-a", "mcp-b", "skills-b", "mcp-user", "skills-user",
                   "mcp-a-repeat", "skills-a-repeat")
INSTALLED_LABELS = ("uap-add", "mcp-installed-a", "skills-installed-a", "mcp-installed-b", "skills-installed-b")
FAILED_LABEL = "mcp-original-config-installed-a"
LABELS = SOURCE_LABELS + METADATA_LABELS + INSTALLED_LABELS + (FAILED_LABEL,)
QUALIFICATIONS = {"project-lifecycle", "native-restart-removal", "mcp-initialize", "mcp-tools-list", "mcp-tools-call"}


def require(condition):
    if not condition:
        raise ValueError("fixed original evidence refused")


def primitives():
    # These are reviewed repository bytes, never content from the artifact.
    path = Path(__file__).with_name("read-hermes-2b-originals.py")
    body = path.read_bytes()
    require(hashlib.sha1(b"blob " + str(len(body)).encode() + b"\0" + body).hexdigest() == READER_BLOB)
    namespace = {"__name__": "fixed_devin_original_helpers", "__file__": str(path)}
    exec(compile(body, str(path), "exec"), namespace)
    checked, expansion = namespace["borrowed_reader"]()
    return namespace, checked, expansion


def download(root, deadline, helpers, checked):
    endpoint = f"repos/{REPOSITORY}/actions/artifacts/{ARTIFACT}"
    metadata_path = root / "artifact.json"
    helpers["api_file"](endpoint, metadata_path, deadline)
    with metadata_path.open("rb") as handle:
        body = handle.read((64 << 10) + 1)
    require(len(body) <= 64 << 10)
    metadata = helpers["json_document"](body)
    require(type(metadata) is dict and metadata.get("id") == ARTIFACT
            and metadata.get("name") == ARTIFACT_NAME and metadata.get("size_in_bytes") == ARCHIVE_BYTES
            and metadata.get("digest") == "sha256:" + ARCHIVE_SHA256 and metadata.get("expired") is False)
    run = metadata.get("workflow_run")
    require(type(run) is dict and run.get("id") == RUN and run.get("head_sha") == SOURCE
            and run.get("repository_id") == run.get("head_repository_id") == REPOSITORY_ID
            and run.get("head_branch") == "feat/native-install-automation-six-clients-20261112")
    archive = root / "original.zip"
    helpers["api_file"](endpoint + "/zip", archive, deadline)
    require(time.monotonic() < deadline)
    return checked(archive, ARCHIVE_SHA256, ARCHIVE_BYTES)


def members(body, expected, helpers, expansion, deadline):
    require(len(body) >= 22 and body[-22:-18] == b"PK\x05\x06")
    disk, first, disk_count, count, size, offset, comment = struct.unpack_from("<4H2LH", body, len(body) - 18)
    require(disk == first == comment == 0 and disk_count == count == len(expected)
            and size <= 64 << 10 and offset + size == len(body) - 22)
    with zipfile.ZipFile(io.BytesIO(body)) as archive:
        require(len(archive.infolist()) == count)
        names, starts = helpers["zip_index"](body, archive, {"files": len(expected)})
        require(set(names) == set(expected) and all(not info.is_dir() for info in names.values())
                and sum(info.file_size for info in names.values()) <= 16 << 20)
        result = {}
        for name, maximum in expected.items():
            require(time.monotonic() < deadline)
            result[name] = helpers["selected_member"](body, archive, names[name], starts[name], maximum, expansion)
    require(time.monotonic() < deadline)
    return result


def safe_text(value, maximum):
    require(type(value) is str and len(value.encode("utf-8")) <= maximum
            and all(32 <= ord(char) <= 126 or char in "\n\r\t" for char in value))
    require(re.search(r"(?i)://|authorization\s*:|(?:password|token|secret|api[_-]?key)\s*[=:]|-----BEGIN", value) is None)
    for name in ("GH_TOKEN", "GITHUB_TOKEN"):
        token = os.environ.get(name, "")
        require(not token or token not in value)
    return value


def original_request(row, helpers):
    argv = row["argv"]
    require(len(argv) >= 2 and type(argv[-2]) is str and type(argv[-1]) is str
            and len(argv[-2].encode("utf-8")) == 2764
            and hashlib.sha256(argv[-2].encode("utf-8")).hexdigest() == PRELUDE_SHA256)
    raw = argv[-1].encode("ascii")
    require(len(raw) <= 16384)
    request = helpers["json_document"](raw)
    require(type(request) is dict and set(request) == {
        "uid", "gid", "cwd", "home", "config", "argv", "environment", "files", "directories", "logs"}
        and json.dumps(request, sort_keys=True, separators=(",", ":")).encode("ascii") == raw)
    require(type(request["uid"]) is int and 0 < request["uid"] < 1 << 32
            and type(request["gid"]) is int and 0 < request["gid"] < 1 << 32)
    native, home, cwd = ROOT + "/client/bin/devin", ROOT + "/home", ROOT + "/project-a"
    config, logs = home + "/config/devin/config.json", home + "/data/devin/cli/logs"
    require(request["cwd"] == row["cwd"] == cwd and request["home"] == home
            and request["config"] == config and request["logs"] == logs
            and request["argv"] == [native, "--config", config, "mcp", "list", "--json"])
    environment = request["environment"]
    require(type(environment) is dict and set(environment) == {
        "HOME", "XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_CACHE_HOME", "XDG_STATE_HOME", "TMPDIR", "PATH"}
        and environment["HOME"] == home and environment["TMPDIR"] == ROOT + "/tmp"
        and all(environment["XDG_" + role.upper() + "_HOME"] == home + "/" + role
                for role in ("config", "data", "cache", "state"))
        and type(environment["PATH"]) is str and 0 < len(environment["PATH"].encode()) <= 8192
        and "\0" not in environment["PATH"])
    for field, paths in (("files", {native, config}), ("directories", {home, cwd, logs})):
        values = request[field]
        require(type(values) is dict and set(values) == paths)
        for identity in values.values():
            require(type(identity) is list and len(identity) == 9
                    and all(type(item) is int and 0 <= item < 1 << 64 for item in identity)
                    and identity[4:6] == [request["uid"], request["gid"]]
                    and identity[2] & 0o170000 == (0o100000 if field == "files" else 0o040000)
                    and (field != "files" or identity[3] == 1))
    expected = ["/usr/bin/bwrap", "--unshare-user", "--uid", str(request["uid"]), "--gid", str(request["gid"]),
                "--unshare-pid", "--unshare-net", "--unshare-ipc", "--unshare-uts", "--disable-userns",
                "--die-with-parent", "--new-session", "--cap-drop", "ALL", "--tmpfs", "/"]
    for path in ("/usr", "/bin", "/lib", "/lib64", ROOT):
        expected.extend(("--ro-bind", path, path))
    expected.extend(("--proc", "/proc", "--remount-ro", "/proc", "--dev", "/dev"))
    for name in ("null", "zero", "full", "random", "urandom", "tty", "pts"):
        expected.extend(("--remount-ro", "/dev/" + name))
    expected.extend(("--remount-ro", "/dev", "--remount-ro", "/", "--chdir", cwd, "--",
                     "/usr/bin/python3", "-I", "-B", "-c", argv[-2], argv[-1]))
    require(argv == expected)
    return b"uap-devin-readonly-viability-v1:" + hashlib.sha256(raw).hexdigest().encode("ascii") + b"\n"


def failure(value, bodies, helpers):
    require(type(value) is dict and value.get("client") == "windsurf" and value.get("phase") == "observe"
            and value.get("candidate") == "devin-global-lifecycle-3000.12.57"
            and value.get("metadata_recipe") == "ordinary-global"
            and value.get("source_commit") == SOURCE and value.get("source_tree") == SOURCE_TREE
            and value.get("collection_complete") is False and value.get("admission_qualified") is False
            and value.get("metadata_capture_complete") is True and value.get("ordinary_metadata_capture_complete") is True
            and value.get("cleanup_complete") is True and value.get("artifact_admission_failed", False) is False
            and value.get("global_lifecycle_capture_complete", False) is False and "in_flight_command" not in value)
    qualification = value.get("qualification")
    require(type(qualification) is dict and set(qualification) == QUALIFICATIONS
            and all(part is False for part in qualification.values()))
    for field, empty in (("probe_methods", []), ("accepted_connections", 0)):
        values = value.get(field)
        require(type(values) is dict and set(values) == {"a", "b", "user", "local", "legacy"}
                and all(type(part) is type(empty) and part == empty for part in values.values()))
    error = safe_text(value.get("error"), 512)
    require(bool(error))
    runtime = value.get("runtime_files")
    require(type(runtime) is list and 0 < len(runtime) <= 4096)
    native = [item for item in runtime if type(item) is dict and item.get("path") == ROOT + "/client/bin/devin"]
    require(len(native) == 1 and set(native[0]) == {"path", "sha256"} and native[0]["sha256"] == BINARY)
    rows = value.get("commands")
    require(type(rows) is list and len(rows) == len(LABELS) == 25)
    keys = {"probe", "argv", "cwd", "error", "exit", "elapsed_seconds",
            "stdout_file", "stderr_file", "stdout_bytes", "stderr_bytes", "stdout_sha256", "stderr_sha256"}
    for index, (row, label) in enumerate(zip(rows, LABELS)):
        require(type(row) is dict and set(row) == keys and row["probe"] == label and row["error"] is None
                and type(row["exit"]) is int and -255 <= row["exit"] <= 255
                and (row["exit"] != 0 if index == 24 else row["exit"] == 0)
                and type(row["elapsed_seconds"]) in (int, float) and math.isfinite(row["elapsed_seconds"])
                and 0 <= row["elapsed_seconds"] <= 910
                and type(row["cwd"]) is str and type(row["argv"]) is list and 1 <= len(row["argv"]) <= 96
                and all(type(arg) is str and "\0" not in arg and len(arg.encode()) <= 24 << 10 for arg in row["argv"]))
        for stream in ("stdout", "stderr"):
            name = label + "." + stream + ".log"
            data = bodies[name]
            require(row[stream + "_file"] == name and type(row[stream + "_bytes"]) is int
                    and row[stream + "_bytes"] == len(data) and len(data) <= 1 << 20
                    and row[stream + "_sha256"] == hashlib.sha256(data).hexdigest())
    for row, arguments, key in zip(rows[:3], (["rev-parse", "HEAD"], ["rev-parse", "HEAD^{tree}"],
                                             ["status", "--porcelain", "--untracked-files=normal"]),
                                  (SOURCE, SOURCE_TREE, "")):
        require(row["argv"] == ["/usr/bin/git", *arguments] and row["cwd"] == SOURCE_DIRECTORY
                and bodies[row["stdout_file"]] == ((key + "\n").encode() if key else b"")
                and bodies[row["stderr_file"]] == b"")
    version = rows[6]
    require(version["argv"] == [ROOT + "/client/bin/devin", "--version"]
            and version["cwd"] == ROOT + "/project-a" and bodies[version["stderr_file"]] == b""
            and re.fullmatch(rb"devin 3000\.12\.57 \([0-9a-f]{7,40}\)\n", bodies[version["stdout_file"]]))
    effects = value.get("command_effects")
    require(type(effects) is list and len(effects) == 18
            and [item.get("label") if type(item) is dict else None for item in effects]
            == list(METADATA_LABELS + INSTALLED_LABELS))
    last = rows[-1]
    header = original_request(last, helpers)
    streams = {}
    for kind in ("stdout", "stderr"):
        data = bodies[last[kind + "_file"]]
        streams[kind] = dict(bytes=len(data), sha256=hashlib.sha256(data).hexdigest(),
                             text=safe_text(data.decode("utf-8", "strict"), 4096))
    return dict(source=SOURCE, source_tree=SOURCE_TREE, run=RUN, job=JOB, artifact=ARTIFACT,
                archive_sha256=ARCHIVE_SHA256, collection_complete=False, admission_qualified=False,
                qualification=qualification, cleanup_complete=True, commands=25, failed_commands=1,
                error=error, failed_command=dict(probe=FAILED_LABEL, exit=last["exit"], error=None,
                    original_argv_joined=True, prelude_header_seen=bodies[last["stdout_file"]].startswith(header),
                    **streams))


def main():
    require(len(sys.argv) == 1 and os.environ.get("GITHUB_REPOSITORY") == REPOSITORY
            and os.environ.get("GITHUB_REF") == "refs/heads/" + BRANCH
            and os.environ.get("GITHUB_EVENT_NAME") == "push")
    os.umask(0o077)
    resource.setrlimit(resource.RLIMIT_FSIZE, (1 << 20, 1 << 20))
    deadline = time.monotonic() + 180
    helpers, checked, expansion = primitives()
    with tempfile.TemporaryDirectory(prefix="devin-843-original-", dir=os.environ["RUNNER_TEMP"]) as temporary:
        outer = download(Path(temporary), deadline, helpers, checked)
        inner = members(outer, {"research.zip": 16 << 20}, helpers, expansion, deadline)["research.zip"]
        expected = {"research.json": 1 << 20}
        expected.update({label + "." + stream + ".log": 1 << 20 for label in LABELS for stream in ("stdout", "stderr")})
        bodies = members(inner, expected, helpers, expansion, deadline)
        result = failure(helpers["json_document"](bodies["research.json"]), bodies, helpers)
        require(time.monotonic() < deadline)
        print("DEVIN_843_ORIGINAL_READOUT=" + json.dumps(result, ensure_ascii=True, sort_keys=True), flush=True)


if __name__ == "__main__":
    try:
        main()
    except Exception:
        print("DEVIN_843_ORIGINAL_READOUT_REFUSED", file=sys.stderr)
        sys.exit(1)
