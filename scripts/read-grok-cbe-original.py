#!/usr/bin/env python3
"""Read one immutable Grok failure artifact; never execute artifact content."""
import hashlib
import io
import json
import os
from pathlib import Path
import re
import resource
import sys
import tempfile
import time
import zipfile
import struct

REPOSITORY = "777genius/universal-agent-plugins"
REPOSITORY_ID = 1193031149
BRANCH = "diagnostics/grok-cbe-original-readout-20261115"
SOURCE = "cbe62f2c7cbdcc75e31cc5e91112b4b98952e794"
SOURCE_TREE = "35e29197e9a2b0d8956def68a3cfe3ea79a5c025"
RUN = 43683143775
JOB = 132717895675
ARTIFACT = 14151208260
ARTIFACT_NAME = "grok-readonly-" + SOURCE
ARCHIVE_BYTES = 3402
ARCHIVE_SHA256 = "6c8ddbedd51b72432dea74fd529dfa4864731efb5ef9fbc3ae1f1af6a29172ce"
READER_BLOB = "cdde17a7664fd1569318609bdf7ba57cfba2a365"
PRODUCER_BYTES = 6322
PRODUCER_SHA256 = "4e81e8a6b35f9534fbbdea5b4acbd2664f34a9a3395f8eaa77bb94bd6f79df4e"
FILES = {
    "grok-readonly-evidence/evidence.json",
    "grok-readonly-evidence/runtime-metadata.json",
    "grok-runtime-build.json",
    "grok-runtime-packages.txt",
}
LABELS = ("version", "git", "native-lock", "readonly-production-inventory")


def require(condition):
    if not condition:
        raise ValueError("fixed original evidence refused")


def primitives():
    # This source is in the already reviewed parent, not in the downloaded ZIP.
    path = Path(__file__).with_name("read-hermes-2b-originals.py")
    body = path.read_bytes()
    require(hashlib.sha1(b"blob " + str(len(body)).encode() + b"\0" + body).hexdigest() == READER_BLOB)
    namespace = {"__name__": "fixed_grok_original_helpers", "__file__": str(path)}
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
            and run.get("head_branch") == "feat/grok-enable-recovery-from-878-20261115")
    archive = root / "original.zip"
    helpers["api_file"](endpoint + "/zip", archive, deadline)
    require(time.monotonic() < deadline)
    return checked(archive, ARCHIVE_SHA256, ARCHIVE_BYTES)


def documents(body, helpers, expansion, deadline):
    require(len(body) >= 22 and body[-22:-18] == b"PK\x05\x06")
    disk, first, disk_count, count, size, offset, comment = struct.unpack_from("<4H2LH", body, len(body) - 18)
    require(disk == first == comment == 0 and disk_count == count and 4 <= count <= 5
            and size <= 64 << 10 and offset + size == len(body) - 22)
    with zipfile.ZipFile(io.BytesIO(body)) as archive:
        require(len(archive.infolist()) == count)
        names, starts = helpers["zip_index"](body, archive, {"files": 4})
        require({name for name, info in names.items() if not info.is_dir()} == FILES
                and all(not info.is_dir() or name == "grok-readonly-evidence/" for name, info in names.items()))
        result = []
        for name, maximum in (("grok-readonly-evidence/evidence.json", 64 << 10),
                              ("grok-runtime-build.json", 16 << 10)):
            require(time.monotonic() < deadline)
            data = helpers["selected_member"](body, archive, names[name], starts[name], maximum, expansion)
            result.append(helpers["json_document"](data))
    require(time.monotonic() < deadline)
    return result


def build_source(value):
    require(type(value) is dict and set(value) == {
        "source_head", "source_tree", "go_version", "build_environment", "build_flags", "copied_inputs"})
    require(value["source_head"] == SOURCE and value["source_tree"] == SOURCE_TREE
            and value["go_version"] == "go version go1.26.11 linux/amd64")
    require(value["build_environment"] == {
        "GOENV": "off", "GOFLAGS": "", "GOEXPERIMENT": "", "GOTOOLCHAIN": "local",
        "GOOS": "linux", "GOARCH": "amd64", "GOAMD64": "v1", "CGO_ENABLED": "0",
        "GOWORK": "/home/runner/work/universal-agent-plugins/universal-agent-plugins/go.work"})
    require(value["build_flags"] == {name: ["test", "-tags=grokreadonlynative", "-c"]
                                    for name in ("grok-readonly-test", "grok-runtime-metadata-test")})
    copied = value["copied_inputs"]
    require(type(copied) is dict and set(copied) == {
        "grok-readonly-test", "grok-runtime-metadata-test", "grok-enable-readonly.py"})
    for name, row in copied.items():
        require(type(row) is dict and set(row) == {"sha256", "bytes", "mode", "uid"}
                and type(row["sha256"]) is str and re.fullmatch("[0-9a-f]{64}", row["sha256"])
                and row["sha256"] != "0" * 64 and type(row["bytes"]) is int and 0 < row["bytes"] <= 64 << 20
                and type(row["uid"]) is int and row["uid"] == 0
                and row["mode"] == ("0o100444" if name.endswith(".py") else "0o100555"))
    producer = copied["grok-enable-readonly.py"]
    require(producer["bytes"] == PRODUCER_BYTES and producer["sha256"] == PRODUCER_SHA256)


def command_argv(root):
    binary, home, project = root + "/grok", root + "/home", root + "/project"
    return (
        [binary, "--version"],
        ["/usr/bin/git", "-c", "core.hooksPath=/dev/null", "init", "--quiet"],
        [binary, "mcp", "disable", "uap_inert_lock"],
        ["/opt/uap-input/grok-readonly-test", "-test.run=^TestEnablementReadonlyNativeInventory$",
         "-test.count=1", "-test.v", "-test.timeout=40s", "-grok-readonly-home=" + home,
         "-grok-readonly-project=" + project, "-grok-readonly-native=" + binary,
         "-grok-readonly-path=" + project + "/.grok/plugins/uap-inert-readonly",
         "-grok-readonly-name=uap-inert-readonly", "-grok-readonly-version=1.0.0"],
    )


def safe_text(value, maximum):
    require(type(value) is str and len(value.encode("utf-8")) <= maximum
            and all(32 <= ord(char) <= 126 or char in "\n\r\t" for char in value))
    require(re.search(r"(?i)://|authorization\s*:|(?:password|token|secret|api[_-]?key)\s*[=:]|-----BEGIN", value) is None)
    for name in ("GH_TOKEN", "GITHUB_TOKEN"):
        token = os.environ.get(name, "")
        require(not token or token not in value)
    return value


def failure(value):
    require(type(value) is dict and set(value) == {
        "passed", "commands", "model_invoked", "authentication_used", "error"}
        and value["passed"] is False and value["model_invoked"] is False
        and value["authentication_used"] is False)
    error = safe_text(value["error"], 2048)
    require(bool(error))
    rows = value["commands"]
    require(type(rows) is list and len(rows) <= 4)
    expected = None
    if rows:
        first = rows[0]
        require(type(first) is dict and type(first.get("argv")) is list and len(first["argv"]) == 2
                and type(first["argv"][0]) is str)
        matched = re.fullmatch(r"(/work/grok-readonly-[a-z0-9_]{8})/grok", first["argv"][0])
        require(matched is not None)
        expected = command_argv(matched.group(1))
    for index, row in enumerate(rows):
        require(type(row) is dict and set(row) == {"label", "argv", "exit", "stdout", "stderr"}
                and row["label"] == LABELS[index] and row["argv"] == expected[index]
                and type(row["exit"]) is int and -255 <= row["exit"] <= 255
                and (index == len(rows) - 1 or row["exit"] == 0)
                and type(row["stdout"]) is str and type(row["stderr"]) is str
                and len(row["stdout"].encode()) + len(row["stderr"].encode()) <= 1 << 20)
    if len(rows) > 1:
        require(rows[0]["stdout"] == "grok 1.0.70 (f50904eaee6c)\n")
    last = None
    if rows:
        row = rows[-1]
        if row["exit"] != 0:
            require(error == "ValueError: native gate failed: " + row["label"])
        last = dict(label=row["label"], exit=row["exit"],
                    stdout=safe_text(row["stdout"], 4096), stderr=safe_text(row["stderr"], 4096))
    return dict(source=SOURCE, source_tree=SOURCE_TREE, run=RUN, job=JOB, artifact=ARTIFACT,
                archive_sha256=ARCHIVE_SHA256, passed=False, model_invoked=False,
                authentication_used=False, error=error, commands=len(rows), last_command=last)


def main():
    require(len(sys.argv) == 1 and os.environ.get("GITHUB_REPOSITORY") == REPOSITORY
            and os.environ.get("GITHUB_REF") == "refs/heads/" + BRANCH
            and os.environ.get("GITHUB_EVENT_NAME") == "push")
    os.umask(0o077)
    resource.setrlimit(resource.RLIMIT_FSIZE, (1 << 20, 1 << 20))
    deadline = time.monotonic() + 180
    helpers, checked, expansion = primitives()
    with tempfile.TemporaryDirectory(prefix="grok-cbe-original-", dir=os.environ["RUNNER_TEMP"]) as temporary:
        body = download(Path(temporary), deadline, helpers, checked)
        evidence, build = documents(body, helpers, expansion, deadline)
        build_source(build)
        result = failure(evidence)
        require(time.monotonic() < deadline)
        print("GROK_CBE_ORIGINAL_READOUT=" + json.dumps(result, ensure_ascii=True, sort_keys=True), flush=True)


if __name__ == "__main__":
    try:
        main()
    except Exception:
        print("GROK_CBE_ORIGINAL_READOUT_REFUSED", file=sys.stderr)
        sys.exit(1)
