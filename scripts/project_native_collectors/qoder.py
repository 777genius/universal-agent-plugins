"""Pinned Qoder archive inventory only; its exact member awaits pin review.

Archive and binary digests are preserved from the existing 1.1.78 collector.
The shared runner must inventory the actual member spelling before a reviewed
amendment selects an entrypoint. No plugin import or session is authorized by
this acquisition leaf.
"""

CLIENT = "qoder"
VERSION = "1.1.78"
AVAILABLE_PHASES = ("acquire",)
ACQUISITION_ONLY = True
ACQUISITION = {
    "url": "https://qoder-ide.oss-accelerate.aliyuncs.com/qodercli/releases/1.1.78/qodercli-linux-x64.tar.gz",
    "integrity": "sha256:5e3213d2d4256c86c5ad2c8bf73fd392f92ba80d0d38f822806542805a48a8ea",
    "max_bytes": 96 << 20,
    "expected_bytes": 78_115_289,
    "format": "tar.gz",
    "entrypoint": None,
    "runtime": "native",
    "binary_sha256": "0da0976e13fe00bc28f725d6fe7c2f0fe9fa9a8c37eeb2ee4e80ad4c0fc58953",
}


def collect(ctx):
    """Refuse all native work until the exact member/phase amendment is reviewed."""
    raise RuntimeError(
        "Qoder is acquisition-only: select the inventoried exact entrypoint "
        "and review the native phase amendment before observation"
    )
