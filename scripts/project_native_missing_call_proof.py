"""Strict *server-side* evidence checks for pending Cursor/Qoder MCP calls.

An accepted receipt means the disposable stdio server saw one call; it does
NOT establish which agent launched it. The caller must independently bind
the real pinned native process, UAP installation, project selection, and
legitimate model authorization before claiming selected-client E2E.
"""
import argparse
from datetime import datetime
import json
from pathlib import Path
import re

MAX_BYTES = 1 << 20
MAX_ROWS = 128
SESSION = re.compile(r"[1-9][0-9]*-[1-9][0-9]*\Z")
NONCE = re.compile(r"[0-9a-f]{32,64}\Z")
METHODS = frozenset({"initialize", "tools/list", "tools/call"})


def _unique(pairs):
    value = {}
    for key, item in pairs:
        if key in value:
            raise ValueError("duplicate JSON key")
        value[key] = item
    return value


def _timestamp(value):
    if not isinstance(value, str) or len(value) > 40 or not value.endswith("Z"):
        raise ValueError("non-UTC event timestamp")
    try:
        moment = datetime.fromisoformat(value[:-1] + "+00:00")
    except ValueError as exc:
        raise ValueError("invalid event timestamp") from exc
    if moment.utcoffset().total_seconds() != 0:
        raise ValueError("event timestamp offset differs")
    return moment


def read_events(body):
    if not isinstance(body, bytes) or len(body) > MAX_BYTES or (body and not body.endswith(b"\n")):
        raise ValueError("unbounded/incomplete event log")
    lines = body.splitlines()
    if len(lines) > MAX_ROWS:
        raise ValueError("excessive events")
    result = []
    for line in lines:
        if not line or len(line) > 8192:
            raise ValueError("invalid event line size")
        try:
            value = json.loads(line.decode("utf-8", "strict"), object_pairs_hook=_unique,
                               parse_constant=lambda _: (_ for _ in ()).throw(ValueError("nonfinite JSON")))
        except (UnicodeError, json.JSONDecodeError) as exc:
            raise ValueError("invalid event JSON") from exc
        if not isinstance(value, dict) or set(value) != {
                "method", "pid", "session", "nonce", "timestamp", "facts"}:
            raise ValueError("unexpected event schema")
        if type(value["method"]) is not str or value["method"] not in METHODS or type(value["pid"]) is not int or value["pid"] <= 0:
            raise ValueError("unexpected method or server PID")
        if type(value["session"]) is not str or not SESSION.fullmatch(value["session"]):
            raise ValueError("invalid server session")
        if int(value["session"].split("-")[0]) != value["pid"]:
            raise ValueError("session/server PID mismatch")
        _timestamp(value["timestamp"])
        if value["method"] == "tools/call":
            facts = value["facts"]
            if not isinstance(facts, dict) or type(facts.get("pid")) is not int or facts["pid"] != value["pid"] or facts.get("session") != value["session"] or facts.get("nonce") != value["nonce"]:
                raise ValueError("uncorrelated tool result")
        elif value["facts"] is not None:
            raise ValueError("non-call method carried tool result")
        result.append(value)
    return result


def require_server_call(events, *, nonce, started, ended):
    """Return one correlated call receipt, NEVER a native-client PASS by itself."""
    if not isinstance(nonce, str) or not NONCE.fullmatch(nonce):
        raise ValueError("fresh cryptographic hex nonce required")
    if (not isinstance(started, datetime) or not isinstance(ended, datetime)
            or started.tzinfo is None or ended.tzinfo is None or not started < ended
            or (ended - started).total_seconds() > 600):
        raise ValueError("bounded independently measured run window required")
    sessions = {}
    calls = []
    previous = started
    for row in read_events(events):
        moment = _timestamp(row["timestamp"])
        if row["nonce"] != nonce or not started <= moment <= ended or moment < previous:
            raise ValueError("stale/foreign/out-of-order nonce receipt")
        previous = moment
        name, method = row["session"], row["method"]
        if method == "initialize":
            if name in sessions:
                raise ValueError("reinitialized session")
            sessions[name] = {"pid": row["pid"], "listed": False, "called": False}
        else:
            state = sessions.get(name)
            if not state or state["pid"] != row["pid"]:
                raise ValueError("MCP method before initialization")
            if method == "tools/list":
                if state["called"]:
                    raise ValueError("tools/list after call")
                state["listed"] = True
            else:
                if not state["listed"] or state["called"]:
                    raise ValueError("tool call before list or duplicate")
                state["called"] = True
                calls.append(row)
    if len(calls) != 1:
        raise ValueError("exactly one server-received tools/call required")
    return {"session": calls[0]["session"], "server_pid": calls[0]["pid"],
            "nonce": nonce, "method": "tools/call", "sessions_observed": len(sessions)}


def fresh_native_model_events(before, after, *, nonce):
    """Separate read-only prior inventories from the genuine selected model turn.

    A pre-model call is never qualified. Original bytes must be an exact
    prefix, and the model suffix must itself contain a fresh session with
    initialize/list/call. The full unmodified log is retained for audit.
    """
    if type(nonce) is not str or not NONCE.fullmatch(nonce):
        raise ValueError("invalid event nonce")
    previous, complete = read_events(before), read_events(after)
    if (any(row["nonce"] != nonce or row["method"] == "tools/call" for row in previous)
            or not after.startswith(before) or len(complete) < len(previous)):
        raise ValueError("stale/changed or pre-model MCP call evidence")
    return after[len(before):]


def require_unchanged_after_negative(before, after):
    """Prove no new events in a separate *already executed* native negative case.

    The caller still must prove that the selected client actually ran after
    project removal / from project B. Empty logs without such a run prove nothing.
    """
    read_events(before)
    read_events(after)
    if after != before:
        raise ValueError("removed/outside Project unexpectedly reached fixture server")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--events", type=Path, required=True)
    parser.add_argument("--nonce", required=True)
    parser.add_argument("--started", required=True, help="UTC ISO-8601 start")
    parser.add_argument("--ended", required=True, help="UTC ISO-8601 end")
    args = parser.parse_args()
    # Do not print facts, command output, profile content or credentials.
    require_server_call(args.events.read_bytes(), nonce=args.nonce,
                        started=_timestamp(args.started), ended=_timestamp(args.ended))
    print("server-side fresh MCP call receipt: valid; native agent identity still requires separate proof")


if __name__ == "__main__":
    main()
