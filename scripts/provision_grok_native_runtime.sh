#!/usr/bin/env bash
# Shared setup from the reviewed grok-enable-readonly.yml runtime stages.
# This provisions one fresh job-owned runtime; callers retain native qualification.
set -euo pipefail

if [[ $# != 0 || -z "${RUNNER_TEMP:-}" || ! -d "$RUNNER_TEMP" ]]; then
  printf '%s\n' 'Grok runtime provisioning requires RUNNER_TEMP and no arguments' >&2
  exit 1
fi
grok_runtime_parent="$(cd -- "$RUNNER_TEMP" && pwd -P)"
if [[ "$grok_runtime_parent" != "$RUNNER_TEMP" ]]; then
  printf '%s\n' 'Grok runtime provisioning requires a canonical absolute parent' >&2
  exit 1
fi
for grok_runtime_entry in \
  grok-runtime-root grok-runtime-input grok-readonly-evidence \
  bubblewrap-0.13.1 bubblewrap-build bubblewrap-0.13.1.tar.xz \
  grok-runtime-packages.txt grok-bubblewrap-help.txt grok-bubblewrap-build.json; do
  if [[ -e "$RUNNER_TEMP/$grok_runtime_entry" || -L "$RUNNER_TEMP/$grok_runtime_entry" ]]; then
    printf '%s\n' 'Grok runtime provisioning refuses an existing fixed output' >&2
    exit 1
  fi
done

sudo apt-get update
sudo apt-get install --yes --no-install-recommends debootstrap ubuntu-keyring
sudo debootstrap --arch=amd64 --variant=minbase --components=main,universe \
  --force-check-gpg --keyring=/usr/share/keyrings/ubuntu-archive-keyring.gpg \
  --include=python3,git,bubblewrap,ca-certificates,libstdc++6 \
  noble "$RUNNER_TEMP/grok-runtime-root" https://archive.ubuntu.com/ubuntu
sudo chroot "$RUNNER_TEMP/grok-runtime-root" /usr/bin/dpkg-query -W \
  > "$RUNNER_TEMP/grok-runtime-packages.txt"
sudo chroot "$RUNNER_TEMP/grok-runtime-root" /usr/sbin/groupadd --gid="$(id -g)" uapfixture
sudo chroot "$RUNNER_TEMP/grok-runtime-root" /usr/sbin/useradd \
  --uid="$(id -u)" --gid="$(id -g)" --no-create-home --home-dir=/work \
  --shell=/usr/sbin/nologin uapfixture
sudo install -d -m 0755 "$RUNNER_TEMP/grok-runtime-root/opt/uap-input" \
  "$RUNNER_TEMP/grok-runtime-root/work" "$RUNNER_TEMP/grok-runtime-input"
mkdir -m 700 "$RUNNER_TEMP/grok-readonly-evidence"

sudo apt-get install --yes --no-install-recommends \
  build-essential meson ninja-build pkg-config libcap-dev libselinux1-dev curl xz-utils
curl --fail --location --proto '=https' --tlsv1.2 \
  --output "$RUNNER_TEMP/bubblewrap-0.13.1.tar.xz" \
  https://github.com/containers/bubblewrap/releases/download/v0.13.1/bubblewrap-0.13.1.tar.xz
printf '%s  %s\n' \
  0e85c8702248b8dd64144f594147553650cdd0e4c951690345faace79c217c4d \
  "$RUNNER_TEMP/bubblewrap-0.13.1.tar.xz" | sha256sum --check --strict
tar --extract --xz --no-same-owner --no-same-permissions \
  --file "$RUNNER_TEMP/bubblewrap-0.13.1.tar.xz" --directory "$RUNNER_TEMP"
meson setup "$RUNNER_TEMP/bubblewrap-build" "$RUNNER_TEMP/bubblewrap-0.13.1" \
  --buildtype=release -Dselinux=enabled -Dman=disabled -Dtests=false \
  -Dbash_completion=disabled -Dzsh_completion=disabled
meson compile -C "$RUNNER_TEMP/bubblewrap-build"
sudo install -o root -g root -m 0555 "$RUNNER_TEMP/bubblewrap-build/bwrap" \
  "$RUNNER_TEMP/grok-runtime-root/usr/bin/bwrap"
# Metadata startup reads host /proc before argument parsing.
# Callers run native gates inside RootDirectory with MountAPIVFS.
test "$("$RUNNER_TEMP/grok-runtime-root/usr/bin/bwrap" --version)" = 'bubblewrap 0.13.1'
"$RUNNER_TEMP/grok-runtime-root/usr/bin/bwrap" --help \
  > "$RUNNER_TEMP/grok-bubblewrap-help.txt"
grep --fixed-strings -- '--ro-bind-fd FD DEST' "$RUNNER_TEMP/grok-bubblewrap-help.txt"
python3 -I -B - <<'PYTHON' > "$RUNNER_TEMP/grok-bubblewrap-build.json"
import hashlib, json, os, pathlib, stat, subprocess
root = pathlib.Path(os.environ['RUNNER_TEMP'])
source = root / 'bubblewrap-build/bwrap'
installed = root / 'grok-runtime-root/usr/bin/bwrap'
archive = root / 'bubblewrap-0.13.1.tar.xz'
if archive.stat().st_size != 129472 or hashlib.sha256(archive.read_bytes()).hexdigest() != '0e85c8702248b8dd64144f594147553650cdd0e4c951690345faace79c217c4d':
    raise ValueError('pinned bubblewrap archive identity changed')
info = installed.lstat()
digest = hashlib.sha256(installed.read_bytes()).hexdigest()
if not stat.S_ISREG(info.st_mode) or info.st_uid != 0 or info.st_gid != 0 or stat.S_IMODE(info.st_mode) != 0o555 or info.st_nlink != 1:
    raise ValueError('pinned bubblewrap is not an ordinary root-owned read-only executable')
if digest != hashlib.sha256(source.read_bytes()).hexdigest():
    raise ValueError('installed bubblewrap differs from the selected build')
print(json.dumps(dict(upstream='https://github.com/containers/bubblewrap',
    version='0.13.1', archive='bubblewrap-0.13.1.tar.xz',
    archive_sha256='0e85c8702248b8dd64144f594147553650cdd0e4c951690345faace79c217c4d',
    binary_sha256=digest, bytes=info.st_size, mode=oct(info.st_mode), uid=info.st_uid,
    build_packages=subprocess.check_output(['dpkg-query', '-W', 'gcc', 'libc6-dev', 'meson', 'ninja-build', 'pkg-config', 'libcap-dev', 'libselinux1-dev'], text=True),
    configure=['--buildtype=release', '-Dselinux=enabled', '-Dman=disabled', '-Dtests=false',
               '-Dbash_completion=disabled', '-Dzsh_completion=disabled'],
    runtime_package_note='grok-runtime-packages.txt records the distro baseline; /usr/bin/bwrap is this pinned upstream build'),
    sort_keys=True))
PYTHON
