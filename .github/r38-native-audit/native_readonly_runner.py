"""Read-only six-lane native artifact acquisition; imports only a pinned audit core."""
import argparse
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import urllib.error
import urllib.parse
import urllib.request

REPO = "777genius/universal-agent-plugins"
API = "https://api.github.com"
HEAD = "c93d3b9803a3dd036ac6f05b3787a3b6b51200a0"
TREE = "d2b10c5ef27715cf5fe8bcea02ee0b7ab33492e6"
CORE = "1ccc6d2c9b2ab7b901fcb028b329e4e4694c922e"
WORKFLOW_PATH = ".github/workflows/agentplugins-native-clients.yml"
SOURCE_FILES = {
    "scripts/run-native-client-matrix.py": "6036bbdfd219e15a9819995fa1f57ff0608530ab",
    WORKFLOW_PATH: "37112c3561d51f298c10368dd4530e0db0b7dca3",
}
LANES = {(client, target) for client in ("claude", "codex")
         for target in ("darwin-arm64", "windows-amd64", "linux-amd64")}
API_LIMIT, ZIP_LIMIT, LOG_LIMIT = 4 << 20, 256 << 20, 32 << 20
sys.dont_write_bytecode = True


def require(ok, reason):
    if not ok:
        raise ValueError(reason)


def sha(body):
    return hashlib.sha256(body).hexdigest()


def pairs(values):
    result = {}
    for key, value in values:
        require(key not in result, "duplicate_json_key")
        result[key] = value
    return result


def decode(body):
    return json.loads(body.decode("utf-8"), object_pairs_hook=pairs,
                      parse_constant=lambda _: require(False, "nonfinite_json"))


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, request, fp, code, msg, headers, newurl):
        return None


OPENER = urllib.request.build_opener(NoRedirect())


def response(request, limit):
    try:
        reply = OPENER.open(request, timeout=45)
    except urllib.error.HTTPError as error:
        reply = error
    try:
        status, headers = reply.code, reply.headers
        body = reply.read(limit + 1) if status == 200 else b""
        require(len(body) <= limit, "response_bound")
        return status, headers, body
    finally:
        reply.close()


def api_request(path, token):
    require(path.startswith("/repos/" + REPO + "/")
            and "?" not in path and "#" not in path, "api_path")
    return urllib.request.Request(API + path, headers={
        "Authorization": "Bearer " + token, "Accept": "application/vnd.github+json",
        "X-GitHub-Api-Version": "2022-11-28",
        "User-Agent": "uap-native-retained-byte-audit",
    })


def api(path, token):
    status, _, body = response(api_request(path, token), API_LIMIT)
    require(status == 200, "api_status")
    return decode(body)


def signed_url(url):
    parsed = urllib.parse.urlsplit(url)
    host = (parsed.hostname or "").lower()
    require(parsed.scheme == "https" and parsed.port in (None, 443)
            and parsed.username is None and parsed.password is None
            and not parsed.fragment and not any(ord(c) < 32 for c in url),
            "signed_url")
    require(any(host.endswith(suffix) for suffix in (
        ".blob.core.windows.net", ".githubusercontent.com",
        ".actions.githubusercontent.com", ".amazonaws.com")), "signed_host")


def redirected(path, token, limit):
    status, headers, _ = response(api_request(path, token), 0)
    require(status == 302, "api_download_redirect")
    location = headers.get("Location", "")
    for _ in range(4):
        signed_url(location)
        # Fresh tokenless requests: no API Authorization or cookies forwarded.
        request = urllib.request.Request(location, headers={
            "User-Agent": "uap-native-retained-byte-audit"})
        status, headers, body = response(request, limit)
        if status == 200:
            return body
        require(status in (301, 302, 303, 307, 308), "download_status")
        location = urllib.parse.urljoin(location, headers.get("Location", ""))
    raise ValueError("redirect_limit")


def git(source, *args):
    completed = subprocess.run(["git", "-C", str(source), *args], check=False,
                               stdout=subprocess.PIPE, stderr=subprocess.DEVNULL,
                               timeout=20)
    require(completed.returncode == 0 and len(completed.stdout) <= API_LIMIT,
            "source_git")
    return completed.stdout.decode("utf-8").strip()


def blob_hash(body):
    return hashlib.sha1(b"blob " + str(len(body)).encode() + b"\0" + body).hexdigest()


def positive(value):
    return type(value) is int and value > 0


def manifest_checks(manifest):
    require(manifest["schema_version"] == 1 and manifest["repo"] == REPO
            and manifest["head"] == HEAD and manifest["checkout"] == HEAD
            and manifest["tree"] == TREE and manifest["core_blob"] == CORE,
            "manifest_identity")
    require(manifest["workflow_path"] == WORKFLOW_PATH
            and manifest["event"] == "pull_request"
            and manifest["run_id"] == 46134103452
            and positive(manifest["run_attempt"]), "manifest_run")
    require(isinstance(manifest["lanes"], list) and len(manifest["lanes"]) == 6,
            "manifest_lane_count")
    keys, jobs, artifacts = set(), set(), set()
    for lane in manifest["lanes"]:
        key = (lane["client"], lane["target"])
        require(key in LANES and key not in keys, "manifest_lane_identity")
        keys.add(key)
        require(positive(lane["job_id"]) and positive(lane["artifact_id"])
                and lane["job_id"] not in jobs and lane["artifact_id"] not in artifacts,
                "manifest_lane_ids")
        jobs.add(lane["job_id"])
        artifacts.add(lane["artifact_id"])
        require(type(lane["archive_size"]) is int
                and 0 < lane["archive_size"] <= ZIP_LIMIT
                and isinstance(lane["archive_sha256"], str)
                and re.fullmatch("[0-9a-f]{64}", lane["archive_sha256"]),
                "manifest_archive")
    require(keys == LANES, "manifest_six_lane_coverage")


def job_steps(job, client, target):
    steps = job["steps"]
    require(isinstance(steps, list) and steps
            and all(s.get("status") == "completed"
                    and s.get("conclusion") in ("success", "skipped") for s in steps),
            "job_failed_or_incomplete_step")
    names = [s["name"] for s in steps]
    require(len(names) == len(set(names)), "duplicate_step")
    mandatory = {
        "Set up job",
        "Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1",
        "Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e",
        "Verify native matrix safety and evidence gates",
        "Build exact source and execute pinned native client in disposable fixture",
        "Upload native evidence including failed stages",
        "Complete job",
    }
    if client == "codex" and target == "linux-amd64":
        mandatory |= {
            "Set up Node for browser Skills qualification",
            "Bundle the real picker before building the public CLI",
            "Prepare the native Codex Linux sandbox",
            "Preserve the actual browser to native Skills screenshots",
        }
    observed = {s["name"]: s["conclusion"] for s in steps}
    require(all(observed.get(name) == "success" for name in mandatory),
            "mandatory_step_not_success")


def checkout_log(log):
    lines = log.decode("utf-8").splitlines()
    commits = []
    for index, line in enumerate(lines[:-1]):
        if "log -1 --format=%H" in line:
            value = re.sub(r"^\d{4}-\d\d-\d\dT\S+Z ", "", lines[index + 1])
            require(re.fullmatch("[0-9a-f]{40}", value), "checkout_log_shape")
            commits.append(value)
    require(commits and set(commits) == {HEAD}, "checkout_log_identity")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--manifest", required=True)
    parser.add_argument("--manifest-sha256", required=True)
    parser.add_argument("--source", required=True)
    parser.add_argument("--core", required=True)
    args = parser.parse_args()
    body = Path(args.manifest).read_bytes()
    require(len(body) <= 32768 and sha(body) == args.manifest_sha256,
            "manifest_custody")
    manifest = decode(body)
    manifest_checks(manifest)
    source = Path(args.source).resolve()
    require(git(source, "rev-parse", "HEAD") == HEAD
            and git(source, "rev-parse", "HEAD^{tree}") == TREE
            and git(source, "status", "--porcelain=v1", "--untracked-files=all") == "",
            "source_checkout")
    texts = {}
    for path, expected in SOURCE_FILES.items():
        source_body = (source / path).read_bytes()
        require(len(source_body) <= API_LIMIT and blob_hash(source_body) == expected
                and git(source, "rev-parse", "HEAD:" + path) == expected,
                "canonical_source_blob")
        texts[path] = source_body.decode("utf-8")
    core = Path(args.core).resolve()
    core_body = core.read_bytes()
    require(len(core_body) <= 65536 and blob_hash(core_body) == CORE, "core_custody")
    # Import only the immutable independent audit core, never product source.
    spec = importlib.util.spec_from_file_location("native_retained_audit_core", core)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    token = os.environ.get("GH_TOKEN", "")
    require(bool(token), "read_token")
    prefix = "/repos/" + REPO
    run = api(prefix + "/actions/runs/" + str(manifest["run_id"]), token)
    require(run["id"] == manifest["run_id"] and run["head_sha"] == HEAD
            and run["repository"]["full_name"] == REPO
            and run["head_repository"]["full_name"] == REPO
            and run["path"] == WORKFLOW_PATH
            and run["run_attempt"] == manifest["run_attempt"]
            and run["event"] == manifest["event"], "run_identity")
    commit = api(prefix + "/git/commits/" + HEAD, token)
    require(commit["sha"] == HEAD and commit["tree"]["sha"] == TREE,
            "authenticated_source_tree")
    receipts = []
    for lane in manifest["lanes"]:
        client, target = lane["client"], lane["target"]
        job = api(prefix + "/actions/jobs/" + str(lane["job_id"]), token)
        require(job["id"] == lane["job_id"] and job["run_id"] == run["id"]
                and job["head_sha"] == HEAD and job["run_attempt"] == run["run_attempt"]
                and job["name"] == client + " runtime (" + target + ")"
                and job["status"] == "completed" and job["conclusion"] == "success",
                "job_identity_or_result")
        job_steps(job, client, target)
        log = redirected(prefix + "/actions/jobs/" + str(lane["job_id"]) + "/logs",
                         token, LOG_LIMIT)
        checkout_log(log)
        artifact = api(prefix + "/actions/artifacts/" + str(lane["artifact_id"]), token)
        require(artifact["id"] == lane["artifact_id"] and artifact["expired"] is False
                and artifact["name"] == "native-client-" + client + "-" + target
                and artifact["size_in_bytes"] == lane["archive_size"]
                and artifact["digest"] == "sha256:" + lane["archive_sha256"]
                and artifact["workflow_run"]["id"] == run["id"]
                and artifact["workflow_run"]["head_sha"] == HEAD,
                "artifact_identity")
        archive = redirected(prefix + "/actions/artifacts/" + str(lane["artifact_id"])
                             + "/zip", token, lane["archive_size"])
        core_manifest = dict(lane, run_id=run["id"], job_id=job["id"],
                             runner_source=texts["scripts/run-native-client-matrix.py"],
                             workflow_source=texts[WORKFLOW_PATH])
        receipt = module.audit_native(archive, {"commit": HEAD, "tree": TREE},
                                      run["id"], job["id"], core_manifest)
        receipt["artifact_id"] = lane["artifact_id"]
        receipt["decoded_job_log_sha256"] = sha(log)
        receipt["decoded_job_log_bytes"] = len(log)
        receipt["job_steps"] = [{"name": s["name"], "conclusion": s["conclusion"]}
                                for s in job["steps"]]
        receipts.append(receipt)
        del archive, log
    require(git(source, "status", "--porcelain=v1", "--untracked-files=all") == "",
            "source_changed_during_audit")
    result = json.dumps({"audit_kind": "six native collection retained-byte audits only",
                         "status": "passed", "source": {"commit": HEAD, "tree": TREE},
                         "core_blob": CORE, "manifest_sha256": sha(body),
                         "lane_count": len(receipts), "receipts": receipts},
                        sort_keys=True)
    require(len(result.encode("utf-8")) <= 49152, "receipt_bound")
    print(result, flush=True)


if __name__ == "__main__":
    try:
        main()
    except Exception:
        # No tokens, URLs, response bodies, paths or protocol contents in failures.
        print('{"status":"failed","audit_kind":"six native collection retained-byte audits only"}',
              flush=True)
        raise SystemExit(1)
