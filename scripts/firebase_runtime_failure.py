"""Bounded, failure-only facts from an owned Firebase fixture; never qualification."""

import hashlib
import json
import os
from pathlib import Path
import re
from runpy import run_path
import stat
import time

_FILES = run_path(str(Path(__file__).with_name("native_client_project_artifacts.py")))


def _unique(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError("duplicate key")
        result[key] = value
    return result


def collect(case):
    """Return all fixed facts or a refusal; no raw log, URL, path or body output."""
    refused = {"status": "refused-unqualified", "qualified": False, "facts": []}
    refusal_phase = "case-root"
    held, directories = [], {}
    deadline = time.monotonic() + 5
    total = 0

    def directory(path):
        if time.monotonic() > deadline:
            raise ValueError("deadline")
        for member in (*reversed(path.parents), path):
            info = member.lstat()
            if (not stat.S_ISDIR(info.st_mode) or stat.S_ISLNK(info.st_mode)
                    or getattr(info, "st_file_attributes", 0) & 0x400):
                raise ValueError("redirected directory")
            key = (info.st_dev, info.st_ino, info.st_mode)
            if member in directories and directories[member] != key:
                raise ValueError("directory replaced")
            directories[member] = key

    def entries(path, limit=16):
        directory(path)
        result = []
        with os.scandir(path) as scan:
            for entry in scan:
                result.append(Path(entry.path))
                if len(result) > limit:
                    raise ValueError("entry bound")
        return sorted(result)

    def read(path, limit=1 << 20):
        nonlocal total
        directory(path.parent)
        before = path.lstat()
        if (not stat.S_ISREG(before.st_mode) or before.st_nlink != 1
                or getattr(before, "st_file_attributes", 0) & 0x400
                or not 0 < before.st_size <= limit or total + before.st_size > 4 << 20):
            raise ValueError("file bound")
        fd = _FILES["open_member"](path)
        held.append((path, fd, _FILES["identity"](before)))
        if not _FILES["same_opened"](before, os.fstat(fd)):
            raise ValueError("opened identity")
        directory(path.parent)
        body = bytearray()
        while len(body) <= before.st_size:
            if time.monotonic() > deadline:
                raise ValueError("deadline")
            block = os.read(fd, min(65536, before.st_size + 1 - len(body)))
            if not block:
                break
            body.extend(block)
        if len(body) != before.st_size:
            raise ValueError("changed size")
        total += len(body)
        return bytes(body)

    def file_fact(path, log=False):
        try:
            if log and path.lstat().st_size > 1 << 20:
                directory(path.parent)
                before = path.lstat()
                if (not stat.S_ISREG(before.st_mode) or before.st_nlink != 1
                        or getattr(before, "st_file_attributes", 0) & 0x400):
                    raise ValueError("log identity")
                fd = _FILES["open_member"](path)
                held.append((path, fd, _FILES["identity"](before)))
                if not _FILES["same_opened"](before, os.fstat(fd)):
                    raise ValueError("log opened identity")
                directory(path.parent)
                return {"present": True, "bytes": before.st_size, "uninspected_large": True}
            body = read(path)
        except FileNotFoundError:
            return {"present": False}
        return {"present": True, "bytes": len(body), "sha256": hashlib.sha256(body).hexdigest()}

    def dir_fact(path):
        try:
            directory(path)
        except FileNotFoundError:
            return False
        return True

    def staged_backend(path, skills, data):
        manifest = json.loads(read(path / ".claude-plugin/plugin.json", 16384), object_pairs_hook=_unique)
        servers = json.loads(read(path / ".mcp.json", 65536), object_pairs_hook=_unique)
        if not isinstance(manifest, dict) or manifest.get("name") != "firebase":
            raise ValueError("staging-manifest")
        if not isinstance(servers, dict) or len(servers) != 1:
            raise ValueError("staging-server")
        server = next(iter(servers.values()))
        env = server.get("env") if isinstance(server, dict) else None
        if not isinstance(env, dict) or not isinstance(env.get("PLUGIN_DATA"), str):
            raise ValueError("staging-env")
        backend = Path(env["PLUGIN_DATA"])
        if not backend.is_absolute() or backend.parent != data:
            # Classify only an owned root's spelling, not the untrusted
            # vendor path. This cannot change the fail-closed verdict.
            try:
                directory(data)
                physical_parent = str(data.resolve(strict=True))
            except (OSError, ValueError):
                physical_parent = None
            refused["staging_parent_probe"] = {
                "absolute": backend.is_absolute(),
                "lexical_parent": backend.parent == data,
                "physical_parent": physical_parent is not None
                                   and str(backend.parent) == physical_parent,
            }
            raise ValueError("staging-data-parent")
        if not re.fullmatch(r"firebase-[a-f0-9]{12}", backend.name):
            raise ValueError("staging-data-identity")
        if env["PLUGIN_DATA"] != str(data / backend.name):
            raise ValueError("staging-data-canonical")
        if env.get("PLUGIN_ROOT") != str(skills / backend.name / ".agentplugins-runtime"):
            raise ValueError("staging-root")
        directory(backend)
        return backend.name

    try:
        case = Path(case)
        if not case.is_absolute() or case.name != "plugin-06" or ".." in case.parts:
            raise ValueError("case authority")
        directory(case)
        refusal_phase = "native-projection"
        source_facts = []
        entrypoint = None
        source_configs = {}
        sources = {}
        skills = case / "home/.claude/skills"
        data = case / "uap-data/plugin-data"
        if dir_fact(skills):
            for skill in entries(skills):
                if not re.fullmatch(r"firebase-[a-f0-9]{12}", skill.name):
                    raise ValueError("foreign projection")
                sources[skill.name] = (skill, "active")
        if dir_fact(skills.parent):
            for staged in entries(skills.parent):
                if not staged.name.startswith(".agentplugins-staging-"):
                    continue
                if not re.fullmatch(r"\.agentplugins-staging-[a-f0-9]{16}", staged.name):
                    raise ValueError("unknown staging path")
                refusal_phase = "staging-binding"
                backend = staged_backend(staged, skills, data)
                if backend in sources:
                    raise ValueError("ambiguous source")
                sources[backend] = (staged, "staging")
        refusal_phase = "source-envelope"
        for backend, (skill, location) in sources.items():
            source = skill / ".agentplugins-runtime/io.github.777genius.agentplugins/runtime"
            config_body = read(source / "runtime.json", 16384)
            config = json.loads(config_body, object_pairs_hook=_unique)
            if not isinstance(config, dict):
                raise ValueError("config shape")
            candidate = config.get("entrypoint")
            if (set(config) != {"schema_version", "package", "version", "entrypoint",
                                "package_lock_sha256", "omit_optional"}
                    or config.get("schema_version") != 1 or type(config.get("omit_optional")) is not bool
                    or not isinstance(config.get("package"), str)
                    or not re.fullmatch(r"(?:@[a-z0-9._-]+/)?[a-z0-9._-]+", config["package"])
                    or not isinstance(config.get("version"), str)
                    or not re.fullmatch(r"[0-9]+\.[0-9]+\.[0-9]+(?:-[A-Za-z0-9.-]+)?", config["version"])
                    or not isinstance(config.get("package_lock_sha256"), str)
                    or not re.fullmatch(r"sha256:[a-f0-9]{64}", config["package_lock_sha256"])
                    or not isinstance(candidate, str) or len(candidate) > 256
                    or not re.fullmatch(r"node_modules/(?:[A-Za-z0-9_.@-]+/)+[A-Za-z0-9_.-]+", candidate)
                    or ".." in Path(candidate).parts):
                raise ValueError("entrypoint")
            if entrypoint is not None and entrypoint != candidate:
                raise ValueError("ambiguous source")
            entrypoint = candidate
            source_configs[backend] = config
            source_package = read(source / "package.json", 65536)
            source_lock = file_fact(source / "package-lock.json")
            source_doc = json.loads(source_package, object_pairs_hook=_unique)
            if (not isinstance(source_doc, dict) or source_doc.get("dependencies") != {config["package"]: config["version"]}
                    or "sha256:" + source_lock.get("sha256", "") != config["package_lock_sha256"]):
                raise ValueError("source identity")
            source_facts.append({"location": location, "config_sha256": hashlib.sha256(config_body).hexdigest(),
                                 "package": {"bytes": len(source_package), "sha256": hashlib.sha256(source_package).hexdigest()},
                                 "lock": source_lock})
        facts = []
        refusal_phase = "owned-data"
        if dir_fact(data):
            for backend in entries(data):
                refusal_phase = "owned-data"
                directory(backend)
                if backend.name not in source_configs:
                    raise ValueError("unselected backend")
                config = source_configs[backend.name]
                owner = json.loads(read(backend / ".agentplugins-data-owner.json", 16384), object_pairs_hook=_unique)
                if not isinstance(owner, dict):
                    raise ValueError("owner shape")
                receipt = owner.get("data_receipt_id")
                allowed = {"data_receipt_id", "physical_backend_id", "scope", "locator",
                           "ownership_digest", "state", "created_at", "updated_at"}
                receipt_bounded = isinstance(receipt, str) and bool(
                    re.fullmatch(r"data_[a-f0-9]{24}", receipt))
                lexical_locator = str(backend)
                lexical_digest = ("sha256:" + hashlib.sha256(
                    ("agentplugins-plugin-data-v1\0" + receipt + "\0"
                     + lexical_locator).encode()).hexdigest()) if receipt_bounded else None
                if (not set(owner) <= allowed or not receipt_bounded
                        or owner.get("state") != "owned"
                        or owner.get("locator") != lexical_locator
                        or owner.get("scope") != "user"
                        or owner.get("physical_backend_id") != backend.name
                        or owner.get("ownership_digest") != lexical_digest):
                    # The directory and all its ancestors have already been
                    # checked without following reparse points. Resolved spelling
                    # is diagnostic only and never admits or rewrites ownership.
                    try:
                        physical_locator = str(backend.resolve(strict=True))
                    except (OSError, ValueError):
                        physical_locator = None
                    physical_digest = ("sha256:" + hashlib.sha256(
                        ("agentplugins-plugin-data-v1\0" + receipt + "\0"
                         + physical_locator).encode()).hexdigest()
                        if receipt_bounded and physical_locator is not None else None)
                    refused["owned_data_probe"] = {
                        "receipt_bounded": receipt_bounded,
                        "owner_fields_bounded": set(owner) <= allowed,
                        "state_owned": owner.get("state") == "owned",
                        "scope_user": owner.get("scope") == "user",
                        "backend_identity": owner.get("physical_backend_id") == backend.name,
                        "locator_lexical": owner.get("locator") == lexical_locator,
                        "locator_physical": (physical_locator is not None
                                             and owner.get("locator") == physical_locator),
                        "digest_lexical": owner.get("ownership_digest") == lexical_digest,
                        "digest_physical": (physical_digest is not None
                                            and owner.get("ownership_digest") == physical_digest),
                    }
                    raise ValueError("data authority")
                refusal_phase = "locked-npm-runtime"
                runtime = backend / "npm-runtime"
                row = {"runtime_present": dir_fact(runtime), "staging": [], "targets": [], "locks": 0}
                if row["runtime_present"]:
                    for target in entries(runtime):
                        directory(target)
                        if re.fullmatch(r"[a-f0-9]{64}\.lock(?:\.retired-[a-f0-9]+)?", target.name):
                            row["locks"] += 1
                            continue
                        stage = bool(re.fullmatch(r"\.tmp-[a-f0-9]{64}-[A-Za-z0-9]+", target.name))
                        if not stage and not re.fullmatch(r"[a-f0-9]{64}", target.name):
                            raise ValueError("unknown runtime entry")
                        package = read(target / "package.json", 65536)
                        doc = json.loads(package, object_pairs_hook=_unique)
                        if not isinstance(doc, dict):
                            raise ValueError("package shape")
                        dependencies = doc.get("dependencies", {})
                        lock = file_fact(target / "package-lock.json")
                        if (dependencies != {config["package"]: config["version"]}
                                or "sha256:" + lock.get("sha256", "") != config["package_lock_sha256"]):
                            raise ValueError("package identity")
                        item = {"package_sha256": hashlib.sha256(package).hexdigest(),
                                "dependencies": dependencies, "lock": lock,
                                "marker": file_fact(target / ".agentplugins-runtime.json"),
                                "entrypoint": file_fact(target / entrypoint) if entrypoint else None,
                                "node_modules": dir_fact(target / "node_modules"),
                                "bin": dir_fact(target / "node_modules/.bin")}
                        row["staging" if stage else "targets"].append(item)
                refusal_phase = "npm-cache"
                logs = backend / "npm-cache/_logs"
                row["npm_logs"] = {"present": dir_fact(logs), "count": 0}
                if row["npm_logs"]["present"]:
                    candidates = entries(logs, 64)
                    if any(not re.fullmatch(r"[0-9T_Z.-]+-debug-[0-9]+\.log", p.name) for p in candidates):
                        raise ValueError("log name")
                    row["npm_logs"]["count"] = len(candidates)
                    if candidates:
                        row["npm_logs"]["latest"] = file_fact(candidates[-1], log=True)
                facts.append(row)
        refusal_phase = "held-custody"
        for path in directories:
            directory(path)
        for path, fd, expected in held:
            if (_FILES["identity"](path.lstat()) != expected
                    or not _FILES["same_opened"](path.lstat(), os.fstat(fd))):
                raise ValueError("file changed")
        if time.monotonic() > deadline:
            raise ValueError("deadline")
        return {"status": "observed-unqualified", "qualified": False,
                "writer_closure_claimed": False, "source": source_facts, "facts": facts}
    except (OSError, ValueError, TypeError, RuntimeError, KeyError) as error:
        # Fixed identifiers only: no exception strings, paths or npm log content.
        # These are refusal locations, never product attribution or qualification.
        refused["refusal_phase"] = refusal_phase
        # Only enumerated, source-owned categories; never include exception text,
        # captured paths, native config values or npm log contents.
        categories = {
            "staging-binding": {"staging-manifest", "staging-server", "staging-env",
                                "staging-data-parent", "staging-data-identity",
                                "staging-data-canonical", "staging-root"},
            "owned-data": {"unselected backend", "owner shape", "data authority",
                           "redirected directory", "directory replaced",
                           "entry bound", "file bound", "opened identity",
                           "changed size", "duplicate key", "deadline"},
        }
        if refusal_phase in categories:
            detail = str(error) if isinstance(error, ValueError) else ""
            refused["refusal_detail"] = (
                detail if detail in categories[refusal_phase] else "unclassified")
        refused["refusal_class"] = (
            "missing" if isinstance(error, FileNotFoundError)
            else "deadline" if isinstance(error, TimeoutError) or (
                isinstance(error, ValueError) and str(error) == "deadline")
            else "invalid-state")
        return refused
    finally:
        for _, fd, _ in reversed(held):
            os.close(fd)
