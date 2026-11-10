#!/usr/bin/env bash
# The Python lifecycle is shared by native Linux, macOS and Windows lanes.
set -euo pipefail
script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
exec python3 "$script_dir/hermes-native.py" run "$script_dir/hermes-native-lifecycle.py"
