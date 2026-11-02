#!/usr/bin/env python3
"""Offline research fixtures, not a UAP runtime implementation or E2E test.

Run with Python plus npm/pip already installed. Only inert locally generated
packages are used. All package/config/cache writes are inside temporary roots.
The npm metadata server binds loopback and is shut down before exit.
"""
from __future__ import annotations

import base64
import gzip
import hashlib
import io
import json
import os
from pathlib import Path
import shutil
import subprocess
import tarfile
import tempfile
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import sys
import zipfile


def run(args: list[str], cwd: Path, env: dict[str, str], *, success: bool = True) -> subprocess.CompletedProcess[str]:
    result = subprocess.run(args, cwd=cwd, env=env, capture_output=True, text=True, timeout=30, check=False)
    if (result.returncode == 0) != success:
        raise AssertionError(f"Unexpected exit {result.returncode}: {args!r}\n{result.stdout}\n{result.stderr}")
    return result


def wheel(root: Path, name: str, version: str, dependency: str = "") -> None:
    dist = f"{name}-{version}.dist-info"
    metadata = f"Metadata-Version: 2.1\nName: {name}\nVersion: {version}\n"
    if dependency:
        metadata += f"Requires-Dist: {dependency}\n"
    with zipfile.ZipFile(root / f"{name}-{version}-py3-none-any.whl", "w") as out:
        out.writestr(zipfile.ZipInfo(f"{dist}/METADATA", (1980, 1, 1, 0, 0, 0)), metadata + "\n")
        out.writestr(zipfile.ZipInfo(f"{dist}/WHEEL", (1980, 1, 1, 0, 0, 0)), "Wheel-Version: 1.0\nGenerator: local-research\nRoot-Is-Purelib: true\nTag: py3-none-any\n")
        out.writestr(zipfile.ZipInfo(f"{dist}/RECORD", (1980, 1, 1, 0, 0, 0)), "")


def pip_spike(root: Path, env: dict[str, str]) -> dict:
    first, later = root / "day1", root / "day2"
    first.mkdir()
    later.mkdir()
    for folder in (first, later):
        wheel(folder, "uap_research_parent", "1.0.0", "uap-research-leaf>=1,<2")
        wheel(folder, "uap_research_leaf", "1.0.0")
    wheel(later, "uap_research_leaf", "1.1.0")
    wheel(later, "uap_research_leaf", "2.0.0")
    parent1 = (first / "uap_research_parent-1.0.0-py3-none-any.whl").read_bytes()
    # Reuse identical parent bytes; the only relevant change is available leaves.
    (later / "uap_research_parent-1.0.0-py3-none-any.whl").write_bytes(parent1)
    results = {}
    for label, folder, leaf in (("first", first, None), ("fresh_later", later, None), ("full_pins", later, "1.0.0")):
        report = root / f"{label}.json"
        args = [sys.executable, "-m", "pip", "--isolated", "install", "--dry-run", "--ignore-installed", "--no-index", "--no-cache-dir", "--disable-pip-version-check", "--find-links", str(folder), "--report", str(report), "uap-research-parent==1.0.0"]
        if leaf:
            args.append("uap-research-leaf==" + leaf)
        run(args, root, env)
        packages = json.loads(report.read_text())["install"]
        results[label] = {p["metadata"]["name"]: p["metadata"]["version"] for p in packages}
    assert results["first"]["uap_research_leaf"] == "1.0.0"
    assert results["fresh_later"]["uap_research_leaf"] == "1.1.0"
    assert results["full_pins"]["uap_research_leaf"] == "1.0.0"
    bad = run([sys.executable, "-m", "pip", "--isolated", "install", "--dry-run", "--ignore-installed", "--no-index", "--no-cache-dir", "--disable-pip-version-check", "--find-links", str(later), "uap-research-parent==1.0.0", "uap-research-leaf==2.0.0"], root, env, success=False)
    assert "ResolutionImpossible" in bad.stderr, bad.stderr
    return {"tool": run([sys.executable, "-m", "pip", "--version"], root, env).stdout.strip(), "parent_sha256": hashlib.sha256(parent1).hexdigest(), "selections": results, "incompatible_pins_refused": True, "installed_packages": False}


def archive(manifest: dict) -> bytes:
    body = json.dumps(manifest, sort_keys=True).encode()
    stream = io.BytesIO()
    with tarfile.open(fileobj=stream, mode="w") as out:
        info = tarfile.TarInfo("package/package.json")
        info.size, info.mode, info.mtime = len(body), 0o644, 0
        out.addfile(info, io.BytesIO(body))
    return gzip.compress(stream.getvalue(), mtime=0)


def npm_spike(root: Path, env: dict[str, str]) -> dict:
    npm = shutil.which("npm")
    if npm is None:
        raise RuntimeError("npm is required; this spike does not install tools")
    manifests = {
        ("uap-research-parent", "1.0.0"): {"name": "uap-research-parent", "version": "1.0.0", "dependencies": {"uap-research-leaf": "^1.0.0"}},
        ("uap-research-leaf", "1.0.0"): {"name": "uap-research-leaf", "version": "1.0.0"},
        ("uap-research-leaf", "1.1.0"): {"name": "uap-research-leaf", "version": "1.1.0"},
    }
    tarballs = {key: archive(manifest) for key, manifest in manifests.items()}
    phase = {"later": False}

    class Registry(BaseHTTPRequestHandler):
        def log_message(self, *_args: object) -> None:
            pass

        def do_GET(self) -> None:
            name = self.path.split("/")[1]
            versions = [v for (n, v) in manifests if n == name and (phase["later"] or v != "1.1.0")]
            if not versions:
                self.send_error(404)
                return
            if "/-/" in self.path:
                selected = next((v for v in versions if self.path == f"/{name}/-/{name}-{v}.tgz"), None)
                if selected is None:
                    self.send_error(404)
                    return
                payload, kind = tarballs[(name, selected)], "application/octet-stream"
            else:
                records = {}
                for version in versions:
                    blob = tarballs[(name, version)]
                    records[version] = {**manifests[(name, version)], "dist": {"tarball": f"http://127.0.0.1:{self.server.server_port}/{name}/-/{name}-{version}.tgz", "integrity": "sha512-" + base64.b64encode(hashlib.sha512(blob).digest()).decode()}}
                payload = json.dumps({"name": name, "dist-tags": {"latest": max(versions)}, "versions": records}).encode()
                kind = "application/json"
            self.send_response(200)
            self.send_header("Content-Type", kind)
            self.send_header("Content-Length", str(len(payload)))
            self.end_headers()
            self.wfile.write(payload)

    server = ThreadingHTTPServer(("127.0.0.1", 0), Registry)
    worker = threading.Thread(target=server.serve_forever, daemon=True)
    worker.start()
    try:
        for file in ("user.npmrc", "global.npmrc"):
            (root / file).write_text("")
        private = {**env, "NPM_CONFIG_USERCONFIG": str(root / "user.npmrc"), "NPM_CONFIG_GLOBALCONFIG": str(root / "global.npmrc"), "NPM_CONFIG_CACHE": str(root / "cache")}
        flags = ["--ignore-scripts", "--no-audit", "--no-fund", "--no-update-notifier", "--fetch-retries=0", f"--registry=http://127.0.0.1:{server.server_port}"]
        package = {"name": "local-uap-research", "version": "1.0.0", "private": True, "dependencies": {"uap-research-parent": "1.0.0"}}
        selected = {}
        locks = {}
        for label in ("first", "fresh_later"):
            folder = root / label
            folder.mkdir()
            (folder / "package.json").write_text(json.dumps(package))
            phase["later"] = label == "fresh_later"
            run([npm, "install", "--package-lock-only", *flags], folder, private)
            locks[label] = (folder / "package-lock.json").read_bytes()
            selected[label] = json.loads(locks[label])["packages"]["node_modules/uap-research-leaf"]["version"]
        assert selected == {"first": "1.0.0", "fresh_later": "1.1.0"}, selected
        pinned = root / "locked_replay"
        pinned.mkdir()
        (pinned / "package.json").write_text(json.dumps(package))
        (pinned / "package-lock.json").write_bytes(locks["first"])
        run([npm, "ci", *flags], pinned, private)
        selected["locked_replay"] = json.loads((pinned / "node_modules/uap-research-leaf/package.json").read_text())["version"]
        assert selected["locked_replay"] == "1.0.0"
        assert (pinned / "package-lock.json").read_bytes() == locks["first"]
        package["dependencies"]["uap-research-leaf"] = "1.1.0"
        (pinned / "package.json").write_text(json.dumps(package))
        bad = run([npm, "ci", *flags], pinned, private, success=False)
        assert "lock" in bad.stderr.lower() and "sync" in bad.stderr.lower(), bad.stderr
        return {"tool": "npm " + run([npm, "--version"], root, private).stdout.strip(), "parent_sha256": hashlib.sha256(tarballs[("uap-research-parent", "1.0.0")]).hexdigest(), "selections": selected, "lock_bytes_unchanged": True, "manifest_mismatch_refused": True}
    finally:
        server.shutdown()
        server.server_close()
        worker.join(timeout=5)
        if worker.is_alive():
            raise RuntimeError("local registry did not shut down")


def main() -> None:
    with tempfile.TemporaryDirectory(prefix="uap-runtime-research-") as temporary:
        root = Path(temporary)
        (root / "home").mkdir()
        env = {key: value for key, value in os.environ.items() if not key.lower().startswith(("npm_config_", "pip_")) and key not in {"NODE_OPTIONS", "PYTHONPATH", "PYTHONHOME"}}
        env.update({"HOME": str(root / "home"), "PYTHONDONTWRITEBYTECODE": "1"})
        (root / "pip").mkdir()
        (root / "npm").mkdir()
        result = {"scope": "inert local packages; real package-manager resolution; not UAP/native-client E2E", "python": sys.version.split()[0], "pip": pip_spike(root / "pip", env), "npm": npm_spike(root / "npm", env)}
    result["temporary_roots_removed"] = True
    print(json.dumps(result, indent=2, sort_keys=True))


if __name__ == "__main__":
    main()
