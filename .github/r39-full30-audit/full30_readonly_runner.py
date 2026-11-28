"""Read-only three-cohort artifact wrapper; current-source build, not npm control."""
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
HEAD = "fccd72199a1eedc12e3828cf2fdc2f10a2f418f7"
CHECKOUT = "8a1f7d8c87f467e98ca8bfb2ba57fc2a5a16a221"
TREE = "41d82c5eb527f34d16ec2e458194496012bc1c22"
CORE = "c91af38ed9d1950d1026577c6868248f8b2b8425"
MAIN = "ff6c507ddcefd23d41d2f26a6fd0b545ab7b6022"
RUNS = {
    "remaining": (46139632929, ".github/workflows/live-install-remaining-clients.yml"),
    "live": (46139633123, ".github/workflows/live-install-audit.yml"),
}
LANES = {("windsurf", "plugin"), ("codex", "plugin"), ("codex", "skill")}
API_LIMIT, ZIP_LIMIT, LOG_LIMIT = 4 << 20, 256 << 20, 32 << 20
BUILD_COMMAND = "go build -trimpath -ldflags='-X main.version=0.1.115' -o \"$RUNNER_TEMP/agentplugins-audit\" ./cli/cmd/agentplugins"
sys.dont_write_bytecode = True

SOURCE_FILES = {
    "scripts/live-install-audit.py": "ceab34554b6851be09fae9dbc5345fdd753bd0f8",
    "scripts/windsurf_installation_audit.py": "a2aa4976159c1664d916d4471475c6c6430858e7",
    "scripts/windsurf_global_skill_audit.py": "5c6ebe69d5ced9f6f44843171265f07c1988efbb",
    "scripts/codex_runtime_audit.py": "759b35886a434a7f17e2c36d8fd93d6a12bbe45d",
    "scripts/codex_plugin_retention_audit.py": "d4d7a75645bbbda74999b159e4f2d49d51e04004",
    "tests/live-install/fixtures.json": "6c8ed47be814d45629b0dee2b98d58e283486d32",
    ".github/workflows/live-install-audit.yml": "f1cec48991179933910e026195372ab8c5a0d5e8",
    ".github/workflows/live-install-remaining-clients.yml": "8e63129f0d59c139abd57c7bedfe0025f9f75632",
}

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
            and manifest["head"] == HEAD and manifest["checkout"] == CHECKOUT
            and manifest["tree"] == TREE and manifest["core_blob"] == CORE
            and manifest["event"] == "pull_request", "manifest_identity")
    require(isinstance(manifest["runs"], dict) and set(manifest["runs"]) == set(RUNS),
            "manifest_run_families")
    for family, (identifier, _) in RUNS.items():
        fixed = manifest["runs"][family]
        require(fixed["run_id"] == identifier and positive(fixed["run_attempt"]),
                "manifest_run")
    require(isinstance(manifest["lanes"], list) and len(manifest["lanes"]) == 3,
            "manifest_lane_count")
    keys, jobs, artifacts = set(), set(), set()
    for lane in manifest["lanes"]:
        key = (lane["target"], lane["kind"])
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
    require(keys == LANES, "manifest_full_cohorts")


def job_steps(job, target):
    steps = job["steps"]
    require(isinstance(steps, list) and steps
            and all(step.get("status") == "completed"
                    and step.get("conclusion") in ("success", "skipped") for step in steps),
            "job_failed_or_incomplete_step")
    names = [step["name"] for step in steps]
    require(len(names) == len(set(names)), "duplicate_step")
    mandatory = {
        "Set up job",
        "Run actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1",
        "Run actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e",
        "Run actions/setup-node@820762786026740c76f36085b0efc47a31fe5020",
        "Complete job",
    }
    if target == "windsurf":
        mandatory |= {
            "Verify Windsurf source/native custody separately",
            "Refuse inherited real client profile overrides before live cases",
            "Check reviewed Devin publisher admission before Plugin acquisition",
            "Acquire actual public vendor client where a headless Linux build exists",
            "Build CLI under test (not the already-published binary)",
            "Run real acquisitions and report every source and outcome",
            "Preserve exact per-case evidence, even if vendor install or CLI fails",
        }
    else:
        mandatory |= {
            "Run actions/setup-python@ece7cb06caefa5fff74198d8649806c4678c61a1",
            "Install native Codex sandbox prerequisite",
            "Install real native client binary, no login or model call",
            "Build PR source CLI with current signed Registry eligibility version",
            "Attempt all 30 independent pinned sources in isolated profiles",
            "Upload all per-source installation outcomes (even on failures)",
        }
    observed = {step["name"]: step["conclusion"] for step in steps}
    require(all(observed.get(name) == "success" for name in mandatory),
            "mandatory_full_cohort_step_not_success")


def checkout_log(log):
    text = log.decode("utf-8")
    lines, commits = text.splitlines(), []
    for index, line in enumerate(lines[:-1]):
        if "log -1 --format=%H" in line:
            value = re.sub(r"^\d{4}-\d\d-\d\dT\S+Z ", "", lines[index + 1])
            require(re.fullmatch("[0-9a-f]{40}", value), "checkout_log_shape")
            commits.append(value)
    require(commits and set(commits) == {CHECKOUT}, "actual_checkout_log_identity")
    require(BUILD_COMMAND in text, "current_source_cli_build_command_missing")


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
    # One clean PR-head checkout supplies all product bytes as inert data.
    require(git(source, "rev-parse", "HEAD") == HEAD
            and git(source, "rev-parse", "HEAD^{tree}") == TREE
            and git(source, "status", "--porcelain=v1", "--untracked-files=all") == "",
            "source_checkout")
    texts = {}
    for path, expected in SOURCE_FILES.items():
        product_body = (source / path).read_bytes()
        require(len(product_body) <= API_LIMIT and blob_hash(product_body) == expected
                and git(source, "rev-parse", "HEAD:" + path) == expected,
                "canonical_source_blob")
        texts[path] = product_body.decode("utf-8")
    core = Path(args.core).resolve()
    core_body = core.read_bytes()
    require(len(core_body) <= 65536 and blob_hash(core_body) == CORE, "core_custody")
    # Import only the independently reviewed core, never any product source.
    spec = importlib.util.spec_from_file_location("full_cohort_retained_core", core)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    token = os.environ.get("GH_TOKEN", "")
    require(bool(token), "read_token")
    prefix = "/repos/" + REPO
    # Fetch/authenticate each immutable commit object once; no local Git fetch,
    # checkout, index or ref mutation is needed to inspect the synthetic object.
    head = api(prefix + "/git/commits/" + HEAD, token)
    synthetic = api(prefix + "/git/commits/" + CHECKOUT, token)
    require(head["sha"] == HEAD and head["tree"]["sha"] == TREE
            and synthetic["sha"] == CHECKOUT and synthetic["tree"]["sha"] == TREE
            and [parent["sha"] for parent in synthetic["parents"]] == [MAIN, HEAD],
            "authenticated_head_and_synthetic_tree")
    runs = {}
    for family, (identifier, workflow) in RUNS.items():
        run = api(prefix + "/actions/runs/" + str(identifier), token)
        require(run["id"] == identifier and run["head_sha"] == HEAD
                and run["repository"]["full_name"] == REPO
                and run["head_repository"]["full_name"] == REPO
                and run["path"] == workflow
                and run["run_attempt"] == manifest["runs"][family]["run_attempt"]
                and run["event"] == manifest["event"], "run_identity")
        runs[family] = run
    receipts = []
    for lane in manifest["lanes"]:
        target, kind = lane["target"], lane["kind"]
        family = "remaining" if target == "windsurf" else "live"
        run = runs[family]
        job = api(prefix + "/actions/jobs/" + str(lane["job_id"]), token)
        job_name = ("Prepared installation 30 plugin / windsurf (report even if blocked)"
                    if target == "windsurf" else "All 30 " + kind + " / native codex")
        require(job["id"] == lane["job_id"] and job["run_id"] == run["id"]
                and job["head_sha"] == HEAD and job["run_attempt"] == run["run_attempt"]
                and job["name"] == job_name
                and job["status"] == "completed" and job["conclusion"] == "success",
                "full_cohort_job_identity_or_result")
        job_steps(job, target)
        log = redirected(prefix + "/actions/jobs/" + str(lane["job_id"]) + "/logs",
                         token, LOG_LIMIT)
        checkout_log(log)
        artifact_name = ("remaining-client-windsurf-plugin" if target == "windsurf"
                         else "live-install-full-codex-" + kind)
        artifact = api(prefix + "/actions/artifacts/" + str(lane["artifact_id"]), token)
        require(artifact["id"] == lane["artifact_id"] and artifact["expired"] is False
                and artifact["name"] == artifact_name
                and artifact["size_in_bytes"] == lane["archive_size"]
                and artifact["digest"] == "sha256:" + lane["archive_sha256"]
                and artifact["workflow_run"]["id"] == run["id"]
                and artifact["workflow_run"]["head_sha"] == HEAD, "artifact_identity")
        archive = redirected(prefix + "/actions/artifacts/" + str(lane["artifact_id"])
                             + "/zip", token, lane["archive_size"])
        core_manifest = dict(lane, run_id=run["id"], job_id=job["id"], sources=texts)
        receipt = module.audit_full30(archive, {"head": HEAD, "checkout": CHECKOUT,
                                               "tree": TREE},
                                      run["id"], job["id"], core_manifest)
        receipt["artifact_id"] = lane["artifact_id"]
        receipt["decoded_job_log_sha256"] = sha(log)
        receipt["decoded_job_log_bytes"] = len(log)
        receipt["job_steps"] = [{"name": step["name"], "conclusion": step["conclusion"]}
                                for step in job["steps"]]
        receipt["installer_authority"] = {
            "kind": "current PR source build", "version_label": "0.1.115",
            "build_checkout": CHECKOUT, "published_control_used": False}
        receipts.append(receipt)
        del archive, log
    require(git(source, "status", "--porcelain=v1", "--untracked-files=all") == "",
            "source_changed_during_audit")
    result = json.dumps({"audit_kind": "three complete cohorts retained evidence only",
                         "status": "passed", "head": HEAD, "checkout": CHECKOUT,
                         "tree": TREE, "core_blob": CORE, "manifest_sha256": sha(body),
                         "lane_count": len(receipts), "receipts": receipts}, sort_keys=True)
    require(len(result.encode("utf-8")) <= 49152, "receipt_bound")
    print(result, flush=True)


if __name__ == "__main__":
    try:
        main()
    except Exception:
        print('{"status":"failed","audit_kind":"three complete cohorts retained evidence only"}',
              flush=True)
        raise SystemExit(1)
