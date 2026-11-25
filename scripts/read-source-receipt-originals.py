#!/usr/bin/env python3
"""Read fixed original receipts. This grants no native qualification."""
import hashlib, io, json, os, resource, stat, struct, subprocess, sys, tempfile, time, zipfile
from pathlib import Path
REPO = "777genius/universal-agent-plugins"
REPO_ID = 1193031149
SOURCE_BRANCH = "fix/release-source-closure-audit-20261122"
BRANCH = "diagnostics/source-receipt-originals-20261125"
BASE = "2c7d48c0440c7e596dd9ab8453551b147713fb8a"
HELPER_BLOB = "aadd5107cbda79947b3a47437c95f3a7f3962645"
# Five immutable original artifacts; decoded size ceilings are operational resources, not measured expanded sizes or production contracts.
SPECS = [
 {"artifact":15038185848,"name":"source-antigravity-darwin-arm64-45613167116-1","size":92037,"sha256":"da478af0262f3c03a601b496c1abb53ad4aca69c83a97818b4f1d4a475ab4a66","run":45613167116,"head":"25cfea7122ee4af83f6414506285f26ba26e30d4","platform":"darwin-arm64","client":"antigravity","whole_run_completed":True},
 {"artifact":15033812233,"name":"source-pi-darwin-arm64-45613167116-1","size":91817,"sha256":"66514200919c7c0993b94e102cdc257feac73ef7cb643906303ae799f4e7fb85","run":45613167116,"head":"25cfea7122ee4af83f6414506285f26ba26e30d4","platform":"darwin-arm64","client":"pi","whole_run_completed":True},
 {"artifact":15033531008,"name":"source-pi-linux-amd64-45613167116-1","size":91818,"sha256":"feaed1213d403c85f723198006fa6a4d79659d35fcbb6c540d40e07dc580a4a7","run":45613167116,"head":"25cfea7122ee4af83f6414506285f26ba26e30d4","platform":"linux-amd64","client":"pi","whole_run_completed":True},
 {"artifact":15032623696,"name":"source-hermes-linux-amd64-45613167116-1","size":626,"sha256":"353e3881b92ca7b1a1c4e300a94c4cd62b0783d6dc9b987233c190c1494faf24","run":45613167116,"head":"25cfea7122ee4af83f6414506285f26ba26e30d4","platform":"linux-amd64","client":"hermes","whole_run_completed":True},
 {"artifact":15048365349,"name":"source-antigravity-windows-amd64-45633193552-1","size":92161,"sha256":"dff0fcb236929fb363fda6156e96cd73610e37a57b76da9c35f58ecdd75c9730","run":45633193552,"head":"2c7d48c0440c7e596dd9ab8453551b147713fb8a","platform":"windows-amd64","client":"antigravity","whole_run_completed":False,"job":139317383388,"job_name":"Public .116 / antigravity / windows-amd64"}
]
ARCHIVE_BOUND = 92161
RECEIPT_BOUND = 32 << 20
TOTAL_BOUND = 128 << 20

def require(ok):
    if not ok: raise ValueError("fixed source receipt readout refused")

def document(body):
    def pairs(rows):
        result = {}
        for key, value in rows:
            require(key not in result)
            result[key] = value
        return result
    return json.loads(body.decode("utf-8"), object_pairs_hook=pairs,
                      parse_constant=lambda _: require(False))

def reader():
    path = Path("scripts/read-authoring-evidence-zip.py")
    body = path.read_bytes()
    require(hashlib.sha1(b"blob " + str(len(body)).encode() + b"\0" + body).hexdigest() == HELPER_BLOB)
    namespace = {"__name__": "source_receipt_zip_helpers"}
    exec(compile(body, str(path), "exec"), namespace)
    return namespace["checked_bytes"], namespace["check_expansion"]

def api(endpoint, destination, deadline):
    require(time.monotonic() < deadline)
    environment = dict(os.environ)
    environment.pop("GH_DEBUG", None)
    environment.update(GH_HOST="github.com", GH_PROMPT_DISABLED="1")
    with destination.open("xb") as output, destination.with_suffix(destination.suffix + ".errors").open("xb") as errors:
        result = subprocess.run(["gh", "api", "--hostname", "github.com",
            "-H", "X-GitHub-Api-Version: 2022-11-28", "-H", "Accept: application/vnd.github+json",
            endpoint], stdout=output, stderr=errors, env=environment,
            timeout=min(60, deadline-time.monotonic()), check=False)
    require(result.returncode == 0 and time.monotonic() < deadline)

def metadata(endpoint, path, deadline):
    api(endpoint, path, deadline)
    with path.open("rb") as stream: body = stream.read((64 << 10) + 1)
    require(len(body) <= 64 << 10)
    result = document(body)
    require(type(result) is dict)
    return result

def zip_index(body, archive):
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
                and name_size <= 1024 and extra_size == 0)
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
    return names, starts

def selected_member(body, archive, info, start, maximum, expansion):
    require(not info.is_dir() and info.file_size <= maximum)
    expansion(body, start, info)
    with archive.open(info) as member:
        result = member.read(maximum + 1)
    require(len(result) == info.file_size and len(result) <= maximum)
    return result

def receipt_bytes(body, spec, expansion):
    require(len(body) >= 22 and body[-22:-18] == b"PK\x05\x06")
    disk, start_disk, disk_count, count, cd_size, cd_offset, comment = struct.unpack_from("<4H2LH", body, len(body)-18)
    require(disk == start_disk == comment == 0 and disk_count == count and 0 < count <= 128
            and cd_size <= 64 << 10 and cd_offset + cd_size == len(body)-22)
    with zipfile.ZipFile(io.BytesIO(body)) as archive:
        require(len(archive.infolist()) == count)
        names, starts = zip_index(body, archive)
        files = {name for name, info in names.items() if not info.is_dir()}
        scopes = ("user",) if spec["client"] == "hermes" else ("user", "project")
        triples = {f"verified-public-policy/{channel}-marketing-kit-{scope}-{stage}":
                   {f"verified-public-policy/{channel}-marketing-kit-{scope}-{stage}/{leaf}"
                    for leaf in ("snapshot.json", "envelope.json", "policy.json")}
                   for channel in ("binary", "npm") for scope in scopes
                   for stage in ("add", "update", "repair", "remove")}
        require("receipt.json" in files and files <= {"receipt.json"}.union(*triples.values()))
        for leaves in triples.values(): require(not files & leaves or leaves <= files)
        directories = {str(Path(name).parent) for name in files} - {"."}
        directories |= {str(Path(name).parent) for name in directories} - {"."}
        require(all(name.rstrip("/") in directories for name, info in names.items() if info.is_dir()))
        require(sum(info.file_size for info in names.values()) <= TOTAL_BOUND)
        for name, info in names.items():
            require(not info.extra and not info.comment)
            expansion(body, starts[name], info)
            with archive.open(info) as stream:
                actual = 0
                while chunk := stream.read(65536):
                    actual += len(chunk)
                    require(actual <= info.file_size)
                require(actual == info.file_size)
        return selected_member(body, archive, names["receipt.json"], starts["receipt.json"], RECEIPT_BOUND, expansion)

def read_one(spec, root, deadline, checked_bytes, expansion, phase):
    phase[0] = "metadata"
    endpoint = f"repos/{REPO}/actions/artifacts/{spec['artifact']}"
    meta = metadata(endpoint, root / f"{spec['artifact']}.json", deadline)
    require(meta.get("id") == spec["artifact"] and meta.get("name") == spec["name"]
            and meta.get("size_in_bytes") == spec["size"] and 0 < spec["size"] <= ARCHIVE_BOUND
            and meta.get("digest") == "sha256:" + spec["sha256"] and meta.get("expired") is False)
    run = meta.get("workflow_run")
    require(type(run) is dict and run.get("id") == spec["run"] and run.get("head_sha") == spec["head"]
            and run.get("repository_id") == run.get("head_repository_id") == REPO_ID
            and run.get("head_branch") == SOURCE_BRANCH)
    detail = metadata(f"repos/{REPO}/actions/runs/{spec['run']}",
                      root / f"run-{spec['artifact']}.json", deadline)
    require(detail.get("id") == spec["run"] and detail.get("head_sha") == spec["head"]
            and detail.get("head_branch") == SOURCE_BRANCH and detail.get("event") == "workflow_dispatch"
            and detail.get("run_attempt") == 1
            and detail.get("path") == ".github/workflows/released-source-audit.yml")
    if spec["whole_run_completed"]:
        require(detail.get("status") == "completed")
    else:
        job = metadata(f"repos/{REPO}/actions/jobs/{spec['job']}",
                       root / f"job-{spec['artifact']}.json", deadline)
        require(job.get("id") == spec["job"] and job.get("run_id") == spec["run"]
                and job.get("head_sha") == spec["head"] and job.get("head_branch") == SOURCE_BRANCH
                and job.get("run_attempt") == 1 and job.get("status") == "completed"
                and job.get("conclusion") == "failure" and job.get("name") == spec["job_name"]
                and job.get("workflow_name") == "Released exact-source lifecycle qualification")
    phase[0] = "download"
    archive = root / f"{spec['artifact']}.zip"
    api(endpoint + "/zip", archive, deadline)
    phase[0] = "zip"
    raw = receipt_bytes(checked_bytes(archive, spec["sha256"], spec["size"]), spec, expansion)
    phase[0] = "header"
    receipt = document(raw)
    require(type(receipt) is dict and receipt.get("schema_version") == 1
            and receipt.get("kind") == "released-exact-source-lifecycle"
            and receipt.get("workflow_sha") == receipt.get("harness_sha") == spec["head"]
            and receipt.get("run_id") == str(spec["run"]) and receipt.get("run_attempt") == "1"
            and receipt.get("platform") == spec["platform"] and type(receipt.get("cases")) is list)
    require(all(type(row) is dict and row.get("client") == spec["client"] for row in receipt["cases"]))
    encoded = json.dumps(receipt, ensure_ascii=True, sort_keys=True, separators=(",", ":"))
    for key in ("GH_TOKEN", "GITHUB_TOKEN"):
        token = os.environ.get(key, "")
        require(not token or token not in raw.decode("utf-8") and token not in encoded)
    identity = {"artifact": spec["artifact"], "run": spec["run"], "head": spec["head"],
                "archive_sha256": spec["sha256"], "receipt_bytes": len(raw),
                "receipt_sha256": hashlib.sha256(raw).hexdigest(), "native_qualification_claimed": False}
    phase[0] = "emit"
    print("SOURCE_RECEIPT_IDENTITY " + json.dumps(identity, sort_keys=True), flush=True)
    chunks = [encoded[i:i+12000] for i in range(0, len(encoded), 12000)]
    for index, chunk in enumerate(chunks):
        print("SOURCE_RECEIPT_CHUNK " + json.dumps({"artifact": spec["artifact"],
            "index": index, "total": len(chunks), "json_text": chunk}, ensure_ascii=True), flush=True)

def main():
    require(len(sys.argv) == 1 and os.environ.get("GITHUB_REPOSITORY") == REPO
            and os.environ.get("GITHUB_REF") == "refs/heads/" + BRANCH
            and os.environ.get("GITHUB_EVENT_NAME") == "push")
    require(len(SPECS) == 5 and len({s["artifact"] for s in SPECS}) == 5
            and all(type(v) is int and 0 < v <= 128 << 20
                    for v in (ARCHIVE_BOUND, RECEIPT_BOUND, TOTAL_BOUND)))
    os.umask(0o077)
    resource.setrlimit(resource.RLIMIT_FSIZE, (ARCHIVE_BOUND + (1 << 20),) * 2)
    deadline = time.monotonic() + 300
    checked_bytes, expansion = reader()
    failed = False
    with tempfile.TemporaryDirectory(prefix="source-receipt-readout-", dir=os.environ["RUNNER_TEMP"]) as temp:
        for spec in SPECS:
            phase = ["metadata"]
            try: read_one(spec, Path(temp), deadline, checked_bytes, expansion, phase)
            except Exception:
                failed = True
                print("SOURCE_RECEIPT_READOUT_REFUSED " + str(spec["artifact"]) + " " + phase[0], file=sys.stderr, flush=True)
    require(not failed)

if __name__ == "__main__":
    try: main()
    except Exception:
        print("SOURCE_RECEIPT_READOUT_REFUSED", file=sys.stderr)
        sys.exit(1)
