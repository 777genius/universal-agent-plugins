"""Platform helpers for the plugin-kit-ai Python wrapper."""

from __future__ import annotations

import platform
from dataclasses import dataclass


@dataclass(frozen=True)
class PlatformInfo:
    os_name: str
    arch_name: str
    binary_name: str


def detect_platform() -> PlatformInfo:
    system = platform.system().lower()
    machine = platform.machine().lower()

    if system == "darwin":
        os_name = "darwin"
    elif system == "linux":
        os_name = "linux"
    elif system == "windows":
        os_name = "windows"
    else:
        raise RuntimeError(f"unsupported OS {platform.system()}")

    if machine in {"x86_64", "amd64"}:
        arch_name = "amd64"
    elif machine in {"arm64", "aarch64"}:
        arch_name = "arm64"
    else:
        raise RuntimeError(f"unsupported architecture {platform.machine()}")

    binary_name = "plugin-kit-ai.exe" if os_name == "windows" else "plugin-kit-ai"
    return PlatformInfo(os_name=os_name, arch_name=arch_name, binary_name=binary_name)


def asset_name_for_version(version: str, platform_info: PlatformInfo) -> str:
    return f"plugin-kit-ai_{version}_{platform_info.os_name}_{platform_info.arch_name}.tar.gz"
