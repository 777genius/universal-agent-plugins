"""Optional dedicated Kiro CI key; never a credential discovery mechanism."""
import base64
import json
import os


def private_forms(value):
    if not value:
        return ()
    raw = value.encode("ascii")
    forms = [raw, json.dumps(value)[1:-1].encode("ascii")]
    for encode in (base64.b64encode, base64.urlsafe_b64encode):
        encoded = encode(raw)
        forms.extend((encoded, encoded.rstrip(b"=")))
    return tuple(dict.fromkeys(forms))


def snapshot(client, environment=None):
    if client != "kiro":
        return ""
    value = (os.environ if environment is None else environment).get("KIRO_API_KEY", "")
    if not isinstance(value, str) or value and (
            not 8 <= len(value) <= 4096 or any(not 33 <= ord(char) <= 126 for char in value)
            or any(len(form) > 4096 for form in private_forms(value))):
        raise ValueError("Kiro CI key refused by private capture bounds")
    return value


def register(ctx, value):
    ctx.kiro_api_key = value
    for form in private_forms(value):
        ctx.register_secret(form)


def reject_private(bodies, value):
    forms = private_forms(value)
    if any(form in name.encode() or form in body for name, body in bodies.items() for form in forms):
        raise ValueError("private Kiro credential refused from native evidence")
