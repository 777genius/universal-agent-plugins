#!/usr/bin/env bash
# Ubuntu 24.04 scoped remediation documented by Codex:
# https://developers.openai.com/codex/concepts/sandboxing#prerequisites
# Keep global user-namespace policy and native stderr/readiness gates unchanged.
set -euo pipefail

if [[ $(uname -s) != Linux || $EUID == 0 ]]; then
  echo 'Codex sandbox setup requires an unprivileged Linux runner' >&2
  exit 1
fi
# All automatic consumers of this helper select ubuntu-24.04.
. /etc/os-release
if [[ $ID != ubuntu || $VERSION_ID != 24.04 ]]; then
  echo 'Codex sandbox setup requires the reviewed Ubuntu 24.04 profile layout' >&2
  exit 1
fi

read_policy() {
  if [[ -r $1 ]]; then cat "$1"; else printf 'unavailable\n'; fi
}
policy_paths=(
  /proc/sys/kernel/apparmor_restrict_unprivileged_userns
  /proc/sys/kernel/unprivileged_userns_clone
  /proc/sys/user/max_user_namespaces
)
policy_before=()
for path in "${policy_paths[@]}"; do
  policy_before+=("$(read_policy "$path")")
  printf 'before %s=%s\n' "$path" "${policy_before[-1]}"
done
printf 'probe_uid=%s kernel=%s\n' "$EUID" "$(uname -r)"
printf 'apparmor_enabled=%s\n' "$(read_policy /sys/module/apparmor/parameters/enabled)"
printf 'policy_reader_profile=%s\n' "$(read_policy /proc/self/attr/current)"

sudo apt-get update
sudo apt-get install --yes --no-install-recommends bubblewrap apparmor-profiles apparmor-utils
bwrap=$(command -v bwrap)
if [[ $(readlink -f "$bwrap") != /usr/bin/bwrap ]]; then
  echo 'Refusing a bwrap executable outside the distro profile attachment' >&2
  exit 1
fi
"$bwrap" --version
dpkg-query -W -f='${Package}=${Version}\n' bubblewrap apparmor apparmor-profiles apparmor-utils
sha256sum /usr/bin/bwrap

if [[ ${policy_before[0]} == 1 ]]; then
  source_profile=/usr/share/apparmor/extra-profiles/bwrap-userns-restrict
  active_profile=/etc/apparmor.d/bwrap-userns-restrict
  [[ -f $source_profile && ! -L $source_profile ]]
  if [[ -e $active_profile || -L $active_profile ]]; then
    if [[ ! -f $active_profile || -L $active_profile ]] || ! cmp -s "$source_profile" "$active_profile"; then
      echo 'Refusing to replace a different existing bwrap AppArmor profile' >&2
      exit 1
    fi
  else
    sudo install -m 0644 "$source_profile" "$active_profile"
  fi
  # Load only this packaged application profile; never disable global policy.
  sudo apparmor_parser -r "$active_profile"
  sha256sum "$source_profile" "$active_profile"
  sudo apparmor_parser -N "$active_profile"
fi

for i in "${!policy_paths[@]}"; do
  after=$(read_policy "${policy_paths[$i]}")
  printf 'after %s=%s\n' "${policy_paths[$i]}" "$after"
  if [[ $after != "${policy_before[$i]}" ]]; then
    echo 'Global user-namespace policy changed during prerequisite installation' >&2
    exit 1
  fi
done
# Exact argv used by Codex 0.153.4 and 0.162.0 sandboxing/src/bwrap.rs.
# This runs as the original runner user, without sudo, Codex, credentials or a model.
printf 'probe: %q --unshare-user --unshare-net --ro-bind / / /bin/true\n' "$bwrap"
probe_status=0
"$bwrap" --unshare-user --unshare-net --ro-bind / / /bin/true || probe_status=$?
printf 'probe_exit=%s\n' "$probe_status"
exit "$probe_status"
