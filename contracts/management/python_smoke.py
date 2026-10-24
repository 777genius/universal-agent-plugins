#!/usr/bin/env python3
"""Qualification of the public process/HTTP protocol using the real Go engine.

Run only against disposable profiles. This is not a maintained Python SDK.
Example: python contracts/management/python_smoke.py --executable /path/agentplugins \
  --config /tmp/fixture/startup.json --source 777genius/universal-agent-plugins@PINNED_COMMIT//repotests/testdata/agentplugins_commands_fixture --kind skill
"""
import argparse
from contextlib import suppress
import json
from pathlib import Path
import queue
import subprocess
import threading
import time
import urllib.error
import urllib.parse
import urllib.request
import uuid


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--executable", required=True)
    parser.add_argument("--config", required=True)
    parser.add_argument("--source", required=True)
    parser.add_argument("--kind", choices=("plugin", "skill"), default="plugin")
    parser.add_argument("--target")
    args = parser.parse_args()
    executable = Path(args.executable)
    if not executable.is_absolute():
        raise ValueError("Use an absolute, pinned executable path")
    configuration = json.loads(Path(args.config).read_text(encoding="utf-8"))
    encoded = (json.dumps(configuration) + "\n").encode()
    if len(encoded) > 65536:
        raise ValueError("Startup configuration exceeds 64 KiB")
    process = subprocess.Popen([str(executable), "manage", "serve"], stdin=subprocess.PIPE,
                               stdout=subprocess.PIPE, stderr=subprocess.DEVNULL)
    try:
        process.stdin.write(encoded)
        process.stdin.flush()
        ready = queue.Queue(maxsize=1)
        threading.Thread(target=lambda: ready.put(process.stdout.readline(65537)), daemon=True).start()
        line = ready.get(timeout=30)
        if len(line) > 65536 or not line.endswith(b"\n"):
            raise RuntimeError("Missing or oversized engine bootstrap")
        bootstrap = json.loads(line)
        endpoint = urllib.parse.urlsplit(bootstrap["url"])
        if (bootstrap["apiVersion"] != 1 or endpoint.scheme != "http" or
                endpoint.hostname != "127.0.0.1" or not endpoint.port or endpoint.path or
                endpoint.query or endpoint.fragment or endpoint.username or endpoint.password or
                bootstrap["origin"] != bootstrap["url"]):
            raise RuntimeError("Unsupported or non-local bootstrap")
        if not isinstance(bootstrap.get("token"), str) or len(bootstrap["token"]) != 64:
            raise RuntimeError("Invalid bootstrap capability")
        class NoRedirect(urllib.request.HTTPRedirectHandler):
            def redirect_request(self, req, fp, code, msg, headers, newurl):
                return None

        opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), NoRedirect())

        def request(method, path, payload=None):
            body = None if payload is None else json.dumps(payload).encode()
            req = urllib.request.Request(bootstrap["url"] + path, data=body, method=method,
                                         headers={"Authorization": "Bearer " + bootstrap["token"],
                                                  "Origin": bootstrap["origin"],
                                                  "Content-Type": "application/json"})
            try:
                with opener.open(req, timeout=30) as response:
                    data = response.read(4 * 1024 * 1024 + 1)
            except urllib.error.HTTPError as error:
                detail = json.loads(error.read(65536))
                raise RuntimeError("Management refusal: " + str(detail.get("code"))) from None
            if len(data) > 4 * 1024 * 1024:
                raise RuntimeError("Oversized management response")
            return json.loads(data)

        session = request("GET", "/api/v1/session")
        if session["contextId"] != bootstrap["contextId"] or session["apiVersion"] != 1:
            raise RuntimeError("Authenticated session mismatch")
        initial = request("GET", "/api/v1/environment")
        if initial["entries"] or initial.get("pendingOperations"):
            raise RuntimeError("Smoke requires a fresh disposable environment")
        target = args.target or configuration["targets"][0]["id"]
        if args.kind == "skill":
            catalog = request("POST", "/api/v1/skills/catalog", {"source": args.source})
            candidates = [entry for entry in catalog["candidates"] if entry["available"]]
            if not candidates:
                raise RuntimeError("Fixture has no available Skill")
            candidate = candidates[0]["id"]
            preview = request("GET", "/api/v1/skills/catalog/" + urllib.parse.quote(catalog["catalogId"], safe="") +
                              "/candidates/" + urllib.parse.quote(candidate, safe="") + "/preview")
            if not preview["markdown"]:
                raise RuntimeError("Fixture preview is empty")
            intent = {"action": "install_skills", "catalogId": catalog["catalogId"],
                      "candidateIds": [candidate], "targetIds": [target]}
        else:
            intent = {"action": "install", "source": args.source, "alias": "python-smoke",
                      "targetIds": [target]}
        plan = request("POST", "/api/v1/plans", intent)
        if not plan["confirmable"]:
            raise RuntimeError("Fixture plan was not confirmable")
        request_id = str(uuid.uuid4())
        apply_path = "/api/v1/plans/" + urllib.parse.quote(plan["planId"], safe="") + "/apply"
        accepted = request("POST", apply_path, {"requestId": request_id})
        repeated = request("POST", apply_path, {"requestId": request_id})
        if repeated["operationId"] != accepted["operationId"]:
            raise RuntimeError("Accepted identity changed on repeat")
        deadline = time.monotonic() + 60
        operation = accepted
        while operation["state"] in ("queued", "running") and time.monotonic() < deadline:
            time.sleep(0.1)
            operation = request("GET", "/api/v1/operations/" + urllib.parse.quote(accepted["operationId"], safe=""))
        if operation["state"] != "succeeded":
            raise RuntimeError("Fixture operation did not succeed: " + str(operation["state"]))
        observed = request("GET", "/api/v1/environment")
        if not observed["entries"]:
            raise RuntimeError("Successful installation has no observed owned entry")
        print(json.dumps({"apiVersion": session["apiVersion"], "engineVersion": session["engineVersion"],
                          "contextId": session["contextId"], "operationId": operation["operationId"],
                          "state": operation["state"], "observedEntries": len(observed["entries"])}))
    finally:
        with suppress(BrokenPipeError):
            process.stdin.close()
        try:
            process.wait(timeout=5)
        except subprocess.TimeoutExpired:
            process.terminate()
            try:
                process.wait(timeout=1)
            except subprocess.TimeoutExpired:
                process.kill()
                process.wait(timeout=3)
        process.stdout.close()


if __name__ == "__main__":
    main()
