"""Prepare a disposable profile through the pinned native configuration writer.

The harness never creates the native init lock. This is fixture preparation,
before public lifecycle assertions; it does not qualify authentication or MCP.
"""
import base64
from contextlib import contextmanager, ExitStack
import copy
import hashlib
import json
import os
import stat
import tomllib

from devin_structured_inventory_admission import metadata_inventory
from grok_acp_bootstrap import PINNED, PREFIX
from grok_acp_inventory_admission import (LOCK_NAME, directory_id, lock_bootstrap_metadata,
    lock_config_bytes, lock_help, stable)
from grok_project_protocol_client import read_owned, require
from project_native_collectors.grok import ACQUISITION
from project_native_research import Budget, verify_files

CONFIG = PREFIX + "config.toml"
LOCK = PREFIX + ".config-init.lock"
WITNESS = "native-profile-witness.py"
MARKER = "native-profile-unexpected-witness"
WITNESS_BODY = (b"import pathlib,sys\npathlib.Path(sys.argv[1]).write_bytes(b'unexpected')\n"
                b"raise SystemExit(93)\n")


def stdout_for_help(row):
    require(type(row.get("exit")) is int and row["exit"] == 0 and row.get("error") is None,
            "native profile help capture failed")
    bodies = {}
    for name in ("stdout", "stderr"):
        body = base64.b64decode(row[name + "_base64"], validate=True)
        require(len(body) <= 1 << 20 and type(row[name + "_bytes"]) is int
                and row[name + "_bytes"] == len(body)
                and row[name + "_sha256"] == hashlib.sha256(body).hexdigest(),
                "native profile help stream receipt differs")
        bodies[name] = body
    require(bodies["stderr"] == b"", "native profile help stderr is not empty")
    # lock_help consumes one stream. Empty, separately retained stderr makes
    # this exact stdout projection legitimate; no stream concatenation occurs.
    return dict(exit=0, error=None, merged_base64=row["stdout_base64"])


@contextmanager
def profile_descriptors(root, inventory):
    with ExitStack() as stack:
        descriptors, links = [], []
        parent = os.open(root, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_CLOEXEC)
        stack.callback(os.close, parent)
        descriptors.append((parent, inventory["."]))
        root_fd = parent
        for name, relative in (("home", "home"), (".grok", "home/.grok")):
            child = os.open(name, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_CLOEXEC,
                            dir_fd=parent)
            stack.callback(os.close, child)
            descriptors.append((child, inventory[relative]))
            links.append((parent, name, inventory[relative]))
            parent = child
        def check():
            require(tuple(directory_id(root.lstat()).values()) == tuple(inventory["."]),
                    "native profile root was replaced")
            for descriptor, expected in descriptors:
                require(tuple(directory_id(os.fstat(descriptor)).values()) == tuple(expected),
                        "native profile held directory differs")
            for descriptor, name, expected in links:
                require(tuple(directory_id(os.stat(name, dir_fd=descriptor, follow_symlinks=False)).values())
                        == tuple(expected), "native profile named directory differs")
        check()
        yield root_fd, parent, check
        check()


def held_bytes(parent, name, descriptor, expected, limit):
    require(stable(os.fstat(descriptor)) == tuple(expected)
            == stable(os.stat(name, dir_fd=parent, follow_symlinks=False))
            and stat.S_ISREG(expected[2]) and expected[3] == 1
            and 0 <= expected[6] <= limit, "native profile file authority differs")
    body = os.read(descriptor, limit + 1)
    require(len(body) == expected[6] and stable(os.fstat(descriptor)) == tuple(expected)
            == stable(os.stat(name, dir_fd=parent, follow_symlinks=False)),
            "native profile file changed during read")
    return body


def check_native_lock(journey):
    receipt = journey.native_profile
    if receipt is None:
        return
    with profile_descriptors(journey.root, receipt) as (_, parent, check):
        descriptor = os.open(".config-init.lock", os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK | os.O_CLOEXEC,
                             dir_fd=parent)
        try:
            require(held_bytes(parent, ".config-init.lock", descriptor, receipt[LOCK], 0) == b"",
                    "native profile lock is not empty")
            check()
        finally:
            os.close(descriptor)
    require(not os.path.lexists(journey.root / MARKER), "native profile witness executed")


def remove_fixture(journey, after, config):
    with profile_descriptors(journey.root, after) as (root_fd, profile_fd, check), ExitStack() as stack:
        files = []
        for parent, name, relative, body in ((profile_fd, "config.toml", CONFIG, config),
                                            (root_fd, WITNESS, WITNESS, WITNESS_BODY)):
            descriptor = os.open(name, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK | os.O_CLOEXEC,
                                 dir_fd=parent)
            stack.callback(os.close, descriptor)
            require(held_bytes(parent, name, descriptor, after[relative], 16384) == body,
                    "temporary native profile fixture bytes differ")
            files.append((parent, name, descriptor, after[relative]))
        check()
        for parent, name, descriptor, expected in files:
            require(stable(os.fstat(descriptor)) == tuple(expected)
                    == stable(os.stat(name, dir_fd=parent, follow_symlinks=False)),
                    "temporary native profile fixture replaced before removal")
        for parent, name, descriptor, expected in files:
            check()
            require(stable(os.fstat(descriptor)) == tuple(expected)
                    == stable(os.stat(name, dir_fd=parent, follow_symlinks=False)),
                    "temporary native profile fixture replaced during removal")
            os.unlink(name, dir_fd=parent)


def admit_writer(journey, before, after, authored, budget):
    require(set(before) <= set(after), "native profile original member missing")
    for name, original in before.items():
        current = after[name]
        if name == CONFIG:
            require(tuple(current[i] for i in (0, 2, 3, 4, 5))
                    == tuple(original[i] for i in (0, 2, 3, 4, 5))
                    and current[3] == 1 and 0 < current[6] <= 16384,
                    "native profile configuration authority changed")
        else:
            require(current == original, "native profile original metadata changed")
    lock = after.get(LOCK)
    require(LOCK not in before and lock is not None and len(lock) == 9
            and lock[0] == before["."][0] and lock[2] == stat.S_IFREG | 0o644
            and lock[3] == 1 and lock[4:6] == before["."][3:] and lock[6] == 0,
            "native writer did not create the expected empty lock")
    # The complete originals stay in evidence. Only the independently admitted
    # new lock is projected out for the existing exact 27-doc/log startup guard.
    docs = lock_bootstrap_metadata(before, {name: value for name, value in after.items() if name != LOCK}, None)
    for name, pin in docs.items():
        budget.check()
        require(hashlib.sha256(lock_config_bytes(journey.root, before, after, name)).hexdigest() == pin[1],
                "native profile startup documentation differs")
    body = lock_config_bytes(journey.root, before, after)
    expected = copy.deepcopy(authored)
    expected["disabled_mcp_servers"] = [LOCK_NAME]
    expected["mcp_servers"][LOCK_NAME]["enabled"] = False
    require(json.dumps(tomllib.loads(body.decode("utf-8")), sort_keys=True, allow_nan=False)
            == json.dumps(expected, sort_keys=True, allow_nan=False), "native profile writer effect differs")
    require(not os.path.lexists(journey.root / MARKER), "native profile witness executed")
    require(metadata_inventory(journey.root, budget) == after, "native profile settled metadata changed")
    return body


def prepare_native_profile(journey):
    require(journey.native_profile is None, "native profile preparation repeated")
    proof = dict(complete=False, native_lock_created=False)
    journey.evidence["native_profile_preparation"] = proof
    profile = journey.home / ".grok"
    require(not os.path.lexists(journey.root / MARKER), "native profile marker collision")
    profile.mkdir(mode=0o700, exist_ok=True)
    require(profile.resolve() == profile and not list(profile.iterdir()), "native profile is not fresh")
    args = ["-I", str(journey.root / WITNESS), str(journey.root / MARKER)]
    authored = dict(disabled_mcp_servers=[], plugins=dict(enabled=[], disabled=[]),
                    mcp_servers={LOCK_NAME: dict(command="/usr/bin/python3", args=args, enabled=True)})
    text = ('disabled_mcp_servers = []\n[plugins]\nenabled = []\ndisabled = []\n'
            '[mcp_servers.' + LOCK_NAME + ']\ncommand = "/usr/bin/python3"\nargs = '
            + json.dumps(args) + '\nenabled = true\n').encode()
    for path, body in ((journey.root / CONFIG, text), (journey.root / WITNESS, WITNESS_BODY)):
        with path.open("xb") as stream:
            stream.write(body)
        path.chmod(0o600)
    with Budget(30) as budget:
        verify_files({journey.grok: ACQUISITION["binary_sha256"]}, budget)
        before = metadata_inventory(journey.root, budget)
        proof.update(before=before, authored_config_base64=base64.b64encode(text).decode())
        commands = journey.evidence["commands"]
        versions = [index for index, row in enumerate(commands)
                    if row.get("argv") == [str(journey.grok), "--version"]]
        require(len(versions) == 1, "native profile version receipt missing or ambiguous")
        indices = [versions[0]]
        for label, argv in (("native-profile-mcp-help", [journey.grok, "mcp", "--help"]),
                            ("native-profile-disable-help", [journey.grok, "mcp", "disable", "--help"])):
            journey.run(label, argv, timeout=5)
            indices.append(len(commands) - 1)
        lock_help([stdout_for_help(commands[index]) for index in indices])
        require(base64.b64decode(commands[indices[0]]["stdout_base64"], validate=True)
                == b"grok 1.0.70 (f50904eaee6c)\n", "native profile version differs")
        prewriter = metadata_inventory(journey.root, budget)
        names = {PREFIX + name for name in PINNED if name.startswith("docs/user-guide/")}
        names.update(PREFIX + name for name in ("docs", "docs/user-guide", "logs", "logs/unified.jsonl"))
        require(all(prewriter.get(name) == value for name, value in before.items())
                and set(prewriter) - set(before) <= names and LOCK not in prewriter
                and not os.path.lexists(journey.root / LOCK)
                and read_owned(journey.root / CONFIG, journey.root) == text,
                "native profile changed before writer or lock already exists")
        proof.update(before_writer=prewriter, help_command_indices=indices)
        journey.run("native-profile-disable-witness", [journey.grok, "mcp", "disable", LOCK_NAME], timeout=5)
        after = metadata_inventory(journey.root, budget)
        proof.update(after=after, writer_command_index=len(commands) - 1)
        body = admit_writer(journey, before, after, authored, budget)
        proof["disabled_config_base64"] = base64.b64encode(body).decode()
        journey.native_profile = {name: after[name] for name in (".", "home", "home/.grok", LOCK)}
        check_native_lock(journey)
        remove_fixture(journey, after, body)
        require(metadata_inventory(journey.root, budget)
                == {name: value for name, value in after.items() if name not in (CONFIG, WITNESS)},
                "temporary native profile removal changed other members")
        check_native_lock(journey)
        verify_files({journey.grok: ACQUISITION["binary_sha256"]}, budget)
        proof.update(complete=True, native_lock_created=True, temporary_fixture_removed=True)
