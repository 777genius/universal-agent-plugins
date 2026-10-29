"""Inert Gemini cycle raw-byte/privacy boundaries; no native execution."""
import base64
import hashlib
import io
import json
import re

from project_native_runtime import strict_json
_ACTUAL_KEY=re.compile(rb"uap-fixture-[0-9a-f]{48}")

def complete_numeric_json(raw, budget):
    if len(raw) > budget[0]:
        return True
    budget[0] -= len(raw)
    def reject_constant(_):
        raise json.JSONDecodeError("nonstandard JSON constant", "", 0)
    try:
        json.loads(raw.decode("utf-8", errors="strict"), parse_int=lambda _: 0,
                   parse_float=lambda _: 0, parse_constant=reject_constant)
    except json.JSONDecodeError:
        return False
    except (UnicodeError, RecursionError, ValueError):
        return True
    return True


def private(body):
    if len(body) > 1 << 20 or b"uap-gemini-private-" in body or _ACTUAL_KEY.search(body):
        return True
    budget = [4 * len(body), 16384]
    def encoded(raw, depth):
        if depth > 64 or len(raw) > budget[0]:
            return True
        budget[0] -= len(raw)
        try:
            value = strict_json(raw)
        except UnicodeError:
            return True
        except RecursionError:
            return True
        except ValueError as error:
            if str(error) == "nonfinite JSON":
                if complete_numeric_json(raw, budget):
                    return True
            elif not isinstance(error, json.JSONDecodeError):
                return True
            if b"\n" not in raw:
                return False
            for line in io.BytesIO(raw):
                if len(line) > budget[0]:
                    return True
                budget[0] -= len(line)
                try:
                    value = strict_json(line)
                except (UnicodeError, RecursionError):
                    return True
                except ValueError as line_error:
                    if str(line_error) == "nonfinite JSON":
                        if complete_numeric_json(line, budget):
                            return True
                    elif not isinstance(line_error, json.JSONDecodeError):
                        return True
                    continue
                if visit(value, depth):
                    return True
            return False
        return visit(value, depth)
    def visit(value, depth, document_depth=0):
        budget[1] -= 1
        # Mirror the preserved Go strict decoder's per-document structure cap;
        # depth64 separately bounds traversal across nested JSON strings.
        if depth > 64 or document_depth > 32 or budget[1] < 0:
            return True
        if type(value) is str:
            try:
                raw = value.encode("utf-8", errors="strict")
            except UnicodeError:
                return True
            if b"uap-gemini-private-" in raw or _ACTUAL_KEY.search(raw):
                return True
            return encoded(raw, depth + 1)
        if type(value) is dict:
            try:
                invalid_key = any(len(key.encode("utf-8", errors="strict")) > 4096 for key in value)
            except UnicodeError:
                return True
            if invalid_key:
                return True
            return any(visit(key, depth + 1, document_depth + 1) or visit(item, depth + 1, document_depth + 1) for key, item in value.items())
        if type(value) is list:
            return any(visit(item, depth + 1, document_depth + 1) for item in value)
        return False
    return encoded(body, 0)


def stream(row):
    if (type(row) is not dict or set(row) != {"bytes", "sha256", "body"}
            or type(row["bytes"]) is not int or not 0 <= row["bytes"] <= 1 << 20
            or not isinstance(row["body"], str) or len(row["body"]) > (1 << 20) * 4 // 3 + 4
            or not isinstance(row["sha256"], str) or re.fullmatch(r"[0-9a-f]{64}", row["sha256"]) is None):
        raise ValueError("closed Gemini cycle raw stream receipt required")
    try:
        body = base64.b64decode(row["body"], validate=True)
    except ValueError as error:
        raise ValueError("Gemini cycle raw base64 differs") from error
    if len(body) != row["bytes"] or hashlib.sha256(body).hexdigest() != row["sha256"] or private(body):
        raise ValueError("Gemini cycle raw bytes/hash/private canary differs")
    return body


def rows(body):
    if len(body) > 1 << 20 or (body and not body.endswith(b"\n")) or private(body):
        raise ValueError("bounded complete Gemini cycle aggregate required")
    rows = []
    for line in io.BytesIO(body):
        row = strict_json(line)
        if (len(rows) >= 128 or type(row) is not dict or set(row) != {"stage", "value"}
                or not isinstance(row["stage"], str) or type(row["value"]) is not dict):
            raise ValueError("closed Gemini cycle stage record required")
        rows.append(row)
    return rows
