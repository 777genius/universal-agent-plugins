"""Isolated read-only Pi paired-artifact wrapper; bounded JSON output only."""
import argparse
import hashlib
import importlib.util
import io
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import urllib.error
import urllib.parse
import urllib.request
import zipfile

REPO = "777genius/universal-agent-plugins"
API = "https://api.github.com"
CORE_BLOB = "8d146d211fdc20c6f0be3e610a16d1fb2d89e528"
API_LIMIT = 4 * 1024 * 1024
ZIP_LIMIT = 8 * 1024 * 1024
sys.dont_write_bytecode = True


def require(ok, label):
    if not ok:
        raise ValueError(label)


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, request, fp, code, msg, headers, newurl):
        return None


OPENER = urllib.request.build_opener(NoRedirect())


def bounded_response(request, limit):
    """Return status/headers/body; errors never escape with response details."""
    try:
        response = OPENER.open(request, timeout=45)
    except urllib.error.HTTPError as error:
        response = error
    try:
        status = response.code
        headers = response.headers
        body = response.read(limit + 1) if status == 200 else b""
        require(len(body) <= limit, "response_bound")
        return status, headers, body
    finally:
        response.close()


def api(path, token):
    require(path.startswith("/repos/" + REPO + "/")
            and "?" not in path and "#" not in path, "api_path")
    request = urllib.request.Request(API + path, headers={
        "Authorization": "Bearer " + token,
        "Accept": "application/vnd.github+json",
        "X-GitHub-Api-Version": "2022-11-28",
        "User-Agent": "uap-isolated-artifact-audit",
    })
    status, headers, body = bounded_response(request, API_LIMIT)
    require(status == 200, "api_status")
    return json.loads(body)


def signed_url(url):
    parsed = urllib.parse.urlsplit(url)
    host = (parsed.hostname or "").lower()
    require(parsed.scheme == "https" and parsed.port in (None, 443)
            and parsed.username is None and parsed.password is None
            and not parsed.fragment, "download_url")
    require(any(host.endswith(suffix) for suffix in (
        ".blob.core.windows.net", ".githubusercontent.com",
        ".actions.githubusercontent.com", ".amazonaws.com")),
        "download_host")


def download(artifact_id, token, size):
    path = "/repos/" + REPO + "/actions/artifacts/" + str(artifact_id) + "/zip"
    request = urllib.request.Request(API + path, headers={
        "Authorization": "Bearer " + token,
        "Accept": "application/vnd.github+json",
        "X-GitHub-Api-Version": "2022-11-28",
        "User-Agent": "uap-isolated-artifact-audit",
    })
    status, headers, unused_body = bounded_response(request, 0)
    require(status == 302, "artifact_redirect")
    location = headers.get("Location", "")
    for unused_index in range(4):
        signed_url(location)
        # A fresh Request has no Authorization, cookies, or API headers.
        request = urllib.request.Request(location, headers={
            "User-Agent": "uap-isolated-artifact-audit",
        })
        status, headers, body = bounded_response(request, size)
        if status == 200:
            require(len(body) == size, "artifact_size")
            return body
        require(status in (301, 302, 303, 307, 308), "download_status")
        location = urllib.parse.urljoin(location, headers.get("Location", ""))
    raise ValueError("download_redirect_limit")


def git(source, *args):
    result = subprocess.run(["git", "-C", str(source), *args],
                            stdout=subprocess.PIPE, stderr=subprocess.DEVNULL,
                            timeout=20, check=False)
    require(result.returncode == 0 and len(result.stdout) <= API_LIMIT, "source_git")
    return result.stdout.decode("utf-8").strip()


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--manifest", required=True)
    parser.add_argument("--manifest-sha256", required=True)
    parser.add_argument("--source", required=True)
    parser.add_argument("--core", required=True)
    args = parser.parse_args()
    manifest_body = Path(args.manifest).read_bytes()
    require(len(manifest_body) <= 16384, "manifest_bound")
    require(hashlib.sha256(manifest_body).hexdigest() == args.manifest_sha256,
            "manifest_digest")
    manifest = json.loads(manifest_body)
    require(manifest["repo"] == REPO, "manifest_repo")
    for field in ("head", "checkout", "tree"):
        require(re.fullmatch("[0-9a-f]{40}", manifest[field]), "manifest_identity")
    require(manifest["checkout"] == manifest["head"], "direct_source_checkout")
    require(set(manifest["artifacts"]) == {"trust", "components"}, "artifact_kinds")
    for fixed in manifest["artifacts"].values():
        require(type(fixed["artifact_id"]) is int and fixed["artifact_id"] > 0,
                "manifest_artifact_id")
        require(re.fullmatch("[0-9a-f]{64}", fixed["sha256"]), "manifest_zip_digest")
        require(type(fixed["size"]) is int and 0 < fixed["size"] <= ZIP_LIMIT,
                "manifest_zip_size")
    for field in ("run_id", "job_id"):
        require(type(manifest[field]) is int and manifest[field] > 0,
                "manifest_id")
    source = Path(args.source).resolve()
    require(git(source, "rev-parse", "HEAD") == manifest["head"], "source_head")
    require(git(source, "rev-parse", "HEAD^{tree}") == manifest["tree"], "source_tree")
    require(git(source, "status", "--porcelain=v1", "--untracked-files=normal") == "",
            "source_dirty")
    require(git(source, "rev-parse", manifest["checkout"] + "^{tree}")
            == manifest["tree"], "checkout_tree")
    core = Path(args.core).resolve()
    core_body = core.read_bytes()
    require(len(core_body) <= 65536, "core_bound")
    object_body = b"blob " + str(len(core_body)).encode() + b"\0" + core_body
    require(hashlib.sha1(object_body).hexdigest() == CORE_BLOB, "core_blob")
    token = os.environ.get("GH_TOKEN", "")
    require(bool(token), "read_token")
    prefix = "/repos/" + REPO
    run = api(prefix + "/actions/runs/" + str(manifest["run_id"]), token)
    job = api(prefix + "/actions/jobs/" + str(manifest["job_id"]), token)
    require(run["id"] == manifest["run_id"] and run["head_sha"] == manifest["head"]
            and run["repository"]["full_name"] == REPO
            and run["head_repository"]["full_name"] == REPO
            and run["path"] == ".github/workflows/pi-project-native.yml"
            and run["run_attempt"] == manifest["run_attempt"]
            and run["event"] == manifest["event"], "run_identity")
    require(job["id"] == manifest["job_id"] and job["run_id"] == run["id"]
            and job["head_sha"] == manifest["head"]
            and job["run_attempt"] == manifest["run_attempt"]
            and job["name"] == manifest["job_name"], "job_identity")
    require(job["status"] == "completed" and job["conclusion"] == "success",
            "job_result")
    blobs = {}
    member_names = {}
    for kind in ("trust", "components"):
        fixed = manifest["artifacts"][kind]
        artifact = api(prefix + "/actions/artifacts/" + str(fixed["artifact_id"]), token)
        require(artifact["id"] == fixed["artifact_id"] and artifact["expired"] is False
                and artifact["name"] == fixed["artifact_name"]
                and artifact["size_in_bytes"] == fixed["size"]
                and artifact["digest"] == "sha256:" + fixed["sha256"]
                and artifact["workflow_run"]["id"] == run["id"]
                and artifact["workflow_run"]["head_sha"] == manifest["head"],
                "artifact_identity")
        blob = download(fixed["artifact_id"], token, fixed["size"])
        require(hashlib.sha256(blob).hexdigest() == fixed["sha256"], "archive_digest")
        blobs[kind] = blob
        with zipfile.ZipFile(io.BytesIO(blob)) as archive:
            names = archive.namelist()
            require(len(names) <= 100 and len(names) == len(set(names))
                    and all(name not in ("", ".", "..")
                            and "/" not in name and "\\" not in name
                            and "\x00" not in name and ":" not in name
                            for name in names), "zip_exact_flat_names")
            member_names[kind] = sorted(names)
    # Only the immutable reviewed auditor is imported; product source is read as data.
    spec = importlib.util.spec_from_file_location("isolated_pi_audit_core", core)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    receipt = module.audit_pi(blobs, source, run, job, manifest)
    receipt["audit_kind"] = "independent paired-artifact check only"
    receipt["zip_member_names"] = member_names
    receipt["manifest_sha256"] = args.manifest_sha256
    result = json.dumps(receipt, sort_keys=True)
    require(len(result) <= 8192, "receipt_bound")
    print(result, flush=True)


if __name__ == "__main__":
    try:
        main()
    except Exception:
        # Never print exception text, signed URLs, tokens, or response bodies.
        print('{"status":"fail","audit_kind":"independent paired-artifact check only"}',
              flush=True)
        raise SystemExit(1)
