"""Qualify eleven pinned OpenCode SDK artifacts, never a node_modules subtree.

OpenCode v1.18.34 (aec0b9a6d8898f68f923aaf08b7306d931fd9d76)
packages/opencode/src/config/config.ts installs @opencode-ai/plugin@1.18.34.
packages/core/src/npm.ts uses Arborist binLinks=true and ignoreScripts=true.
The selected bytes below were independently matched to SHA512-verified official
registry.npmjs.org archives after a genuine empty-profile native bootstrap.
This is only a removal-scan proof; it grants no native discovery or readiness.
"""
from contextlib import contextmanager, ExitStack
import hashlib
import json
import os
from pathlib import Path
import stat

VERSION = "1.18.34"
MAX_BYTES = 4 * 1024 * 1024
ROOT_PACKAGE = (65, "03acb0f9a0c48470d1b64ff6c036c5788be9f27e186c0a556a0b1f6731548b80")

# Exact official package.json bytes, including SDK identity and version.
PACKAGE_PINS = {
    "@opencode-ai/plugin": (
        "1.18.34", 1786, "a210307c7fdbf726aebd5ccee74eaf0cca2b76118a8f558e1b877e2d7f3b8e60"),
    "effect": (
        "4.0.0-beta.83", 3208, "527b918a3a636a90e5416c1fb3729ecf4d8bb2da21d05410839738255af57921"),
    "msgpackr-extract": (
        "3.0.7", 2535, "2ef1cc5d0bd9c092ff9080877cbd1f1bc5bd72484665ff5829d69f8cc8a8d949"),
    "node-gyp-build-optional-packages": (
        "5.2.2", 897, "939e18b5652b4cd288a9ff263ddd9be1101b0503903794b9f8feb2603461e1a4"),
    "uuid": (
        "14.0.3", 3611, "da6bb61e393eb364b83daab9e897e88acbf4c32e7d1d2af20597cbeff866b429"),
    "which": (
        "2.0.2", 1043, "43010039ed5e89f7186960be682b3cb5cda5ab6cdfb06cbfd4f081cf0e7b4260"),
    "yaml": (
        "2.9.1", 3254, "1c6441703d8204a23ded0d37ddf57c3b69d821dc392f20852d1f605bb9b8861c"),
}
# Four complete vendor bundles; no other large file is admitted.
BUNDLES = {
    "node_modules/effect/dist/unstable/httpapi/internal/httpApiScalar.js": (
        "effect", 3249516,
        "d6f24c3c5996aee46fc7a0a1fb13aca0c07b24a9402ad482b9544b6c4f9ca8ea"),
    "node_modules/effect/dist/unstable/httpapi/internal/httpApiSwagger.js": (
        "effect", 1984871,
        "82cf8b08f4e568362da63c221ad6382b28c62a2ec58ace321dc5ae977e197f80"),
    "node_modules/effect/src/unstable/httpapi/internal/httpApiScalar.ts": (
        "effect", 3249481,
        "c6536937a36c108eacd5728c919e0283b150af0a65856d80ec271ea22483bfd3"),
    "node_modules/effect/src/unstable/httpapi/internal/httpApiSwagger.ts": (
        "effect", 1984841,
        "2241ab419a9111209a06b4fdcc14f1eb022f5b4d67cf1840fc5ed1bd84358589"),
}
# Seven exact link texts and separately opened, contained regular targets.
BIN_LINKS = {
    "node_modules/.bin/download-msgpackr-prebuilds": (
        "msgpackr-extract",
        "../msgpackr-extract/bin/download-prebuilds.js",
        "node_modules/msgpackr-extract/bin/download-prebuilds.js",
        336, "e5d9fb733fb23a14f4a3e09f0618051b9393a419b0f77e2b15766a8cf8beb22c"),
    "node_modules/.bin/node-gyp-build-optional-packages": (
        "node-gyp-build-optional-packages",
        "../node-gyp-build-optional-packages/bin.js",
        "node_modules/node-gyp-build-optional-packages/bin.js",
        2219, "b6c3ee58c7199854c80d2a6a7a67292ec6c9be9b6de2125c8f8370afc3c6bff4"),
    "node_modules/.bin/node-gyp-build-optional-packages-optional": (
        "node-gyp-build-optional-packages",
        "../node-gyp-build-optional-packages/optional.js",
        "node_modules/node-gyp-build-optional-packages/optional.js",
        143, "e0b3a3a04166e6ecf1020cb31c0c4a54432c16d6d88714bd4de2214cf67dec81"),
    "node_modules/.bin/node-gyp-build-optional-packages-test": (
        "node-gyp-build-optional-packages",
        "../node-gyp-build-optional-packages/build-test.js",
        "node_modules/node-gyp-build-optional-packages/build-test.js",
        398, "34c19ff8b6675d6d27c63a7df44d77a442805eeea8756d1c89e0264f4a3028f6"),
    "node_modules/.bin/node-which": (
        "which",
        "../which/bin/node-which",
        "node_modules/which/bin/node-which",
        985, "055f0ac4eed1a1591d033d59462972968bf3483b4cc07e163589569c0fb999f0"),
    "node_modules/.bin/uuid": (
        "uuid",
        "../uuid/dist-node/bin/uuid",
        "node_modules/uuid/dist-node/bin/uuid",
        45, "a9dc67d05c3b08842280064e7e65fab9de3f4257b1dfd88e6e51d5afd967bdf4"),
    "node_modules/.bin/yaml": (
        "yaml",
        "../yaml/bin.mjs",
        "node_modules/yaml/bin.mjs",
        310, "faebbf70669988b5be13130907adf7b788145960448eab08e92e66189e72f448"),
}


def candidate(relative):
    return relative in BUNDLES or relative in BIN_LINKS


def supported():
    # No pathname-only fallback on Windows or hosts without descriptor custody.
    return (os.name == "posix" and os.open in os.supports_dir_fd
            and os.stat in os.supports_dir_fd and os.readlink in os.supports_dir_fd
            and os.stat in os.supports_follow_symlinks
            and all(hasattr(os, flag) for flag in
                    ("O_DIRECTORY", "O_NOFOLLOW", "O_CLOEXEC", "O_NONBLOCK")))


def _identity(info):
    result = (info.st_mode, info.st_dev, info.st_ino)
    if not stat.S_ISDIR(info.st_mode):
        result += (info.st_nlink, info.st_size, info.st_mtime_ns, info.st_ctime_ns)
    return result


@contextmanager
def _owned_objects(root, pins, link):
    """Hold each ancestor and exact leaf until identity/content checks finish."""
    flags = os.O_RDONLY | os.O_NOFOLLOW | os.O_CLOEXEC | os.O_NONBLOCK
    with ExitStack() as opened:
        directories = {}
        held = []

        def directory(path):
            if path in directories:
                return directories[path]
            parent = directory(path.parent) if path != Path("/") else None
            name = path.name if parent is not None else "/"
            initial = os.stat(name, dir_fd=parent, follow_symlinks=False)
            if not stat.S_ISDIR(initial.st_mode):
                raise ValueError("OpenCode bootstrap parent is not a directory")
            descriptor = os.open(name, flags | os.O_DIRECTORY, dir_fd=parent)
            opened.callback(os.close, descriptor)
            if _identity(os.fstat(descriptor)) != _identity(initial):
                raise ValueError("OpenCode bootstrap parent changed")
            directories[path] = descriptor
            held.append((parent, name, descriptor, initial))
            return descriptor

        bodies = {}
        for relative, (size, digest) in pins.items():
            path = root / relative
            parent = directory(path.parent)
            initial = os.stat(path.name, dir_fd=parent, follow_symlinks=False)
            if (not stat.S_ISREG(initial.st_mode) or initial.st_nlink != 1
                    or not 0 <= size <= MAX_BYTES or initial.st_size != size):
                raise ValueError("OpenCode bootstrap file kind or size changed")
            descriptor = os.open(path.name, flags, dir_fd=parent)
            opened.callback(os.close, descriptor)
            if _identity(os.fstat(descriptor)) != _identity(initial):
                raise ValueError("OpenCode bootstrap file changed before read")
            with os.fdopen(os.dup(descriptor), "rb") as stream:
                body = stream.read(size + 1)
            if len(body) != size or hashlib.sha256(body).hexdigest() != digest:
                raise ValueError("OpenCode bootstrap bytes differ from pinned source")
            bodies[relative] = body.decode("utf-8")
            held.append((parent, path.name, descriptor, initial))
        if link is not None:
            relative, expected = link
            path = root / relative
            parent = directory(path.parent)
            initial = os.stat(path.name, dir_fd=parent, follow_symlinks=False)
            if (not stat.S_ISLNK(initial.st_mode) or initial.st_nlink != 1
                    or initial.st_size > 4096
                    or os.readlink(path.name, dir_fd=parent) != expected):
                raise ValueError("OpenCode bootstrap link text or kind changed")
            held.append((parent, path.name, None, initial))
        yield bodies
        for parent, name, descriptor, initial in held:
            if (_identity(os.stat(name, dir_fd=parent, follow_symlinks=False)) != _identity(initial)
                    or (descriptor is not None
                        and _identity(os.fstat(descriptor)) != _identity(initial))):
                raise ValueError("OpenCode bootstrap object or ancestor changed during read")


def qualify(root, relative, client_version, path_has_identity, content_has_identity):
    """Return bounded proof only after native pins, all bytes and ownership match."""
    if (not candidate(relative) or not supported() or not root.is_absolute()
            or root != Path(os.path.abspath(root))
            or not isinstance(client_version, dict)
            or client_version.get("status") != "reported"
            or client_version.get("value") != VERSION):
        return None
    link = None
    if relative in BIN_LINKS:
        owner, text, target, size, digest = BIN_LINKS[relative]
        link = (relative, text)
    else:
        owner, size, digest = BUNDLES[relative]
        target = relative
    package_paths = {name: "node_modules/" + name + "/package.json"
                     for name in ("@opencode-ai/plugin", owner)}
    pins = {"package.json": ROOT_PACKAGE, target: (size, digest)}
    pins.update({path: PACKAGE_PINS[name][1:] for name, path in package_paths.items()})
    if any(path_has_identity(path) for path in (*pins, relative)):
        return None
    try:
        with _owned_objects(root, pins, link) as bodies:
            if (json.loads(bodies["package.json"]) != {"dependencies": {"@opencode-ai/plugin": VERSION}}
                    or any(content_has_identity(body) for body in bodies.values())):
                return None
            for name, path in package_paths.items():
                manifest = json.loads(bodies[path])
                if (not isinstance(manifest, dict) or manifest.get("name") != name
                        or manifest.get("version") != PACKAGE_PINS[name][0]):
                    return None
        return {"path": relative, "native_version": VERSION, "regular_target": target,
                "bytes": size, "sha256": digest, "link_text": link[1] if link else None,
                "sdk_manifest_sha256": PACKAGE_PINS["@opencode-ai/plugin"][2],
                "owner_manifest_sha256": PACKAGE_PINS[owner][2],
                "source_bytes_fully_read": True, "symlink_followed": False,
                "atomic_snapshot_claimed": False, "writer_closure_claimed": False}
    except (OSError, RuntimeError, UnicodeError, ValueError):
        return None
