//go:build linux

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

const canonicalPublisherRoot = "/usr/share/code"

type publisherViewMount struct {
	target     string
	id, parent uint64
	underlying os.FileInfo
}
type trustedPublisherView struct {
	parent                   *publisherParent
	original                 *os.File
	identity                 policyIdentity
	originalMount, namespace uint64
	originalNamespace        uint64
	acquired                 []map[string]any
	topology                 []publisherMountRow
	mounts                   []publisherViewMount
	lower, upper, work       string
	released                 bool
	active                   bool
	uncertainMount           bool
	layers                   []*trustedScannerNode
}

func publisherNamespace() (uint64, error) {
	info, err := os.Stat("/proc/self/ns/mnt")
	if err != nil {
		return 0, err
	}
	identity, err := policyStat(info)
	return identity.Inode, err
}

func publisherMountTable() ([]byte, error) {
	body, file, _, err := policyRead("/proc/self/mountinfo", publisherMountInfoLimit, true, 0)
	if err != nil {
		return nil, err
	}
	return body, file.Close()
}

func admittedPublisherPolicy(s hostPolicySnapshot) error {
	want := map[string]string{
		"code":   stockCodeProfileSHA,
		"abi":    "e510bb8f6788b45e48de2f859a6f94a7b8416cbac5a1051814cfce925fa911bd",
		"global": "2c1d72013b53bf103fb8abe2199ce5775f8493df1acd3ef497046156c09fdf7c",
		"alias":  "e7b8b278db3f7aca74e0176f22cbd3adc3c89212cc9e66a90a2787b3ceee8030",
	}
	if s.Grammar != "known-ubuntu-vscode-userns" || !s.KnownProfileMatch || s.LoadedMode != "unconfined" || s.Enabled == nil || !*s.Enabled || s.CodeAbsent == nil || !*s.CodeAbsent || s.Files["local_code"].State != "absent" || s.Entries["disable_code"] != "absent" || s.Entries["force_complain_code"] != "absent" {
		return fmt.Errorf("stock publisher policy prerequisite refused")
	}
	for name, digest := range want {
		if s.Files[name].State != "present" || s.Files[name].SHA256 != digest {
			return fmt.Errorf("stock publisher include identity refused")
		}
	}
	for name, value := range map[string]int64{"apparmor_userns": 1, "apparmor_unconfined": 0, "userns_clone": 1} {
		if s.Sysctls[name] == nil || *s.Sysctls[name] != value {
			return fmt.Errorf("stock publisher sysctl prerequisite refused")
		}
	}
	if s.Sysctls["max_user_namespaces"] == nil || *s.Sysctls["max_user_namespaces"] <= 0 {
		return fmt.Errorf("publisher user namespace budget refused")
	}
	return nil
}

func (c *controller) preparePublisherView(policy *hostPolicyMeasurement) error {
	if policy == nil || policy.reason != "none" {
		return fmt.Errorf("publisher view requires observed host policy")
	}
	if err := admittedPublisherPolicy(policy.before); err != nil {
		return err
	}
	ns, err := publisherNamespace()
	if err != nil || c.request.Input.OriginalMountNamespace == 0 || ns == c.request.Input.OriginalMountNamespace {
		return fmt.Errorf("private publisher namespace refused")
	}
	v := &trustedPublisherView{namespace: ns, originalNamespace: c.request.Input.OriginalMountNamespace}
	c.publisherView = v // partial acquisition never permits Base deletion without release.
	v.parent, err = holdPublisherParent(c.request.Input.OriginalMountNamespace, ns, c.request.Input.UID)
	if err != nil {
		return err
	}
	if err = v.holdOriginal(); err != nil {
		return err
	}
	body, err := publisherMountTable()
	if err != nil {
		return err
	}
	v.topology, err = publisherOriginalTopology(body, v.originalMount)
	if err != nil {
		return err
	}
	base := filepath.Join(c.request.Base, "publisher-view")
	if err = os.Mkdir(base, 0700); err != nil {
		return err
	}
	v.lower, v.upper, v.work = filepath.Join(base, "lower"), filepath.Join(base, "upper"), filepath.Join(base, "work")
	for _, path := range []string{v.lower, v.upper, v.work} {
		if err = os.Mkdir(path, 0755); err != nil {
			return err
		}
		if err = v.holdLayer(path); err != nil {
			return err
		}
	}
	if err = v.admitLayers(); err != nil {
		return err
	}
	if err = v.bindLower(); err != nil {
		return err
	}
	if err = v.mountOverlay(); err != nil {
		return err
	}
	source := filepath.Join(c.request.Base, "publisher", publisherPrefix)
	if err = c.checkPublisher(); err != nil {
		return err
	}
	return v.bindPublisher(source)
}

func (v *trustedPublisherView) holdOriginal() error {
	info, err := anchoredInfo("/usr/share")
	if err != nil {
		return err
	}
	identity, err := policyStat(info)
	if err != nil || !info.IsDir() || identity.UID != 0 || identity.GID != 0 || info.Mode().Perm() != 0777 {
		return fmt.Errorf("original publisher parent identity refused")
	}
	fd, err := unix.Open("/usr/share", unix.O_PATH|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	v.original = os.NewFile(uintptr(fd), "/usr/share")
	v.identity = identity
	v.originalMount, err = scannerMountID("/usr/share")
	if err != nil {
		return err
	}
	return v.verifyOriginal(false)
}

func (v *trustedPublisherView) originalPath() string {
	return "/proc/self/fd/" + strconv.FormatUint(uint64(v.original.Fd()), 10)
}
func (v *trustedPublisherView) verifyOriginal(covered bool) error {
	if v.original == nil {
		return fmt.Errorf("held original publisher parent missing")
	}
	info, err := v.original.Stat()
	if err != nil {
		return err
	}
	identity, err := policyStat(info)
	if err != nil || identity != v.identity {
		return fmt.Errorf("original publisher parent changed")
	}
	path := v.originalPath()
	if !covered {
		path = "/usr/share"
	}
	linked, err := os.Stat(path)
	if err != nil || !os.SameFile(info, linked) {
		return fmt.Errorf("original publisher parent path changed")
	}
	var st unix.Statx_t
	if err = unix.Statx(int(v.original.Fd()), "", unix.AT_EMPTY_PATH, unix.STATX_MNT_ID, &st); err != nil || st.Mask&unix.STATX_MNT_ID == 0 || st.Mnt_id != v.originalMount {
		return fmt.Errorf("original publisher mount changed")
	}
	if _, err = os.Lstat(filepath.Join(path, "code")); !os.IsNotExist(err) {
		return fmt.Errorf("foreign publisher leaf collision")
	}
	return nil
}

func (v *trustedPublisherView) admitLayers() error {
	var device uint64
	for _, path := range []string{v.lower, v.upper, v.work} {
		info, err := exactDirectory(path, 0)
		if err != nil || info.Mode().Perm() != 0755 {
			return fmt.Errorf("publisher layer ownership refused")
		}
		identity, err := policyStat(info)
		if err != nil || (device != 0 && identity.Device != device) {
			return fmt.Errorf("publisher layer filesystem differs")
		}
		device = identity.Device
		entries, err := os.ReadDir(path)
		if err != nil || len(entries) != 0 {
			return fmt.Errorf("fresh publisher layer required")
		}
	}
	var fs unix.Statfs_t
	if err := unix.Statfs(v.upper, &fs); err != nil {
		return err
	}
	// Refuse overlay-on-overlay and filesystems without the documented local
	// upper xattr/d_type semantics. Genuine hosted admission must establish ext4.
	if fs.Type != unix.EXT4_SUPER_MAGIC {
		return fmt.Errorf("publisher upper filesystem not admitted ext4")
	}
	return nil
}

func (v *trustedPublisherView) retainMount(target string, parent uint64, underlying os.FileInfo) error {
	v.uncertainMount = true
	id, err := scannerMountID(target)
	if err != nil || id == 0 || id == parent {
		return fmt.Errorf("publisher new mount identity refused")
	}
	v.mounts = append(v.mounts, publisherViewMount{target, id, parent, underlying})
	kind := "lower"
	if target == "/usr/share" {
		kind = "overlay"
	}
	if target == canonicalPublisherRoot {
		kind = "publisher"
	}
	v.acquired = append(v.acquired, map[string]any{"kind": kind, "mount_id": id, "parent_mount_id": parent})
	v.uncertainMount = false
	return nil
}
func (v *trustedPublisherView) bindLower() error {
	if err := v.verifyParent(); err != nil {
		return err
	}
	parent, err := scannerMountID(v.lower)
	if err != nil {
		return err
	}
	underlying, err := anchoredInfo(v.lower)
	if err != nil {
		return err
	}
	if err = unix.Mount(v.originalPath(), v.lower, "", unix.MS_BIND, ""); err != nil {
		return err
	}
	if err = v.retainMount(v.lower, parent, underlying); err != nil {
		return err
	}
	if err = v.verifyParent(); err != nil {
		return err
	}
	if err = unix.Mount("", v.lower, "", unix.MS_BIND|unix.MS_REMOUNT|unix.MS_RDONLY|unix.MS_NOATIME|unix.MS_NOSUID|unix.MS_NODEV, ""); err != nil {
		return err
	}
	return publisherReadOnly(v.lower)
}
func publisherReadOnly(path string) error {
	var fs unix.Statfs_t
	if err := unix.Statfs(path, &fs); err != nil {
		return err
	}
	required := int64(unix.ST_RDONLY | unix.ST_NOSUID | unix.ST_NODEV | unix.ST_NOATIME)
	if fs.Flags&required != required {
		return fmt.Errorf("publisher mount is writable")
	}
	return nil
}
func (v *trustedPublisherView) mountOverlay() error {
	if err := v.verifyParent(); err != nil {
		return err
	}
	if err := v.verifyOriginal(false); err != nil {
		return err
	}
	options := "lowerdir=" + v.lower + ",upperdir=" + v.upper + ",workdir=" + v.work + ",index=off,redirect_dir=off,metacopy=off,xino=off"
	if err := unix.Mount("overlay", "/usr/share", "overlay", unix.MS_NODEV|unix.MS_NOSUID, options); err != nil {
		return err
	}
	if err := v.retainMount("/usr/share", v.originalMount, v.originalInfo()); err != nil {
		return err
	}
	info, err := exactDirectory("/usr/share", 0)
	upper, ue := exactDirectory(v.upper, 0)
	if err != nil || ue != nil || info.Mode().Perm() != 0755 || upper.Mode().Perm() != 0755 || os.SameFile(info, upper) || os.SameFile(info, v.originalInfo()) {
		return fmt.Errorf("upper-backed publisher parent refused")
	}
	return v.verifyOriginal(true)
}
func (v *trustedPublisherView) originalInfo() os.FileInfo { info, _ := v.original.Stat(); return info }
func (v *trustedPublisherView) bindPublisher(source string) error {
	if err := v.verifyParent(); err != nil {
		return err
	}
	if _, err := os.Lstat(canonicalPublisherRoot); !os.IsNotExist(err) {
		return fmt.Errorf("fresh canonical publisher leaf required")
	}
	if err := os.Mkdir(canonicalPublisherRoot, 0755); err != nil {
		return err
	}
	parent, err := scannerMountID(canonicalPublisherRoot)
	if err != nil {
		return err
	}
	underlying, err := anchoredInfo(canonicalPublisherRoot)
	if err != nil {
		return err
	}
	if err = v.holdLayer(source); err != nil {
		return err
	}
	if err = unix.Mount(source, canonicalPublisherRoot, "", unix.MS_BIND, ""); err != nil {
		return err
	}
	if err = v.retainMount(canonicalPublisherRoot, parent, underlying); err != nil {
		return err
	}
	if err = v.verifyParent(); err != nil {
		return err
	}
	if err = unix.Mount("", canonicalPublisherRoot, "", unix.MS_BIND|unix.MS_REMOUNT|unix.MS_RDONLY|unix.MS_NOSUID|unix.MS_NODEV|unix.MS_NOATIME, ""); err != nil {
		return err
	}
	if err = publisherReadOnly(canonicalPublisherRoot); err != nil {
		return err
	}
	a, err := os.Stat(source)
	b, be := os.Stat(canonicalPublisherRoot)
	if err != nil || be != nil || !os.SameFile(a, b) {
		return fmt.Errorf("canonical publisher source binding differs")
	}
	if err = v.verifyOriginal(true); err != nil {
		return err
	}
	v.active = true
	return v.verifyActive()
}

func (c *controller) closePublisherView() error {
	v := c.publisherView
	if v == nil {
		return nil
	}
	if err := v.verifyLayers(); err != nil {
		return err
	}
	if v.uncertainMount {
		return fmt.Errorf("publisher unmeasured mount release refused")
	}
	for _, record := range c.children {
		if !record.TreeReaped {
			return fmt.Errorf("publisher view release requires proven owned-tree cleanup")
		}
	}
	for len(v.mounts) > 0 {
		if err := v.releaseLastMount(); err != nil {
			return err
		}
		v.mounts = v.mounts[:len(v.mounts)-1]
	}
	if v.original == nil {
		v.released = true
		return nil
	}
	if err := v.verifyOriginal(false); err != nil {
		return err
	}
	body, err := publisherMountTable()
	if err != nil {
		return err
	}
	after, err := publisherOriginalTopology(body, v.originalMount)
	if err != nil || !reflect.DeepEqual(v.topology, after) {
		return fmt.Errorf("original mount topology changed")
	}
	v.released = true
	for _, n := range v.layers {
		if err := n.file.Close(); err != nil {
			return err
		}
	}
	if v.parent != nil {
		if err := v.parent.file.Close(); err != nil {
			return err
		}
	}
	return v.original.Close()
}

func (v *trustedPublisherView) summary() map[string]any {
	parentPID, parentSHA := 0, ""
	if v.parent != nil {
		parentPID, parentSHA = v.parent.pid, v.parent.executableSHA
	}
	return map[string]any{"sudo_parent_pid": parentPID, "sudo_parent_executable_sha256": parentSHA, "mount_namespace": v.namespace, "original_mount_namespace": v.originalNamespace, "original_share_identity": v.identity, "acquired_mounts": v.acquired, "original_mount_id": v.originalMount, "released": v.released, "mount_identity_uncertain": v.uncertainMount, "remaining_mounts": len(v.mounts), "label_required": "vscode (unconfined)", "scope": "private-publisher-view-v1"}
}

func (v *trustedPublisherView) holdLayer(path string) error {
	info, err := anchoredInfo(path)
	if err != nil {
		return err
	}
	uid, err := statUID(info)
	if err != nil || uid != 0 || !info.IsDir() || info.Mode().Perm()&022 != 0 {
		return fmt.Errorf("publisher held layer refused")
	}
	fd, err := unix.Open(path, unix.O_PATH|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	node := &trustedScannerNode{path: path, file: os.NewFile(uintptr(fd), path), info: info}
	if err = node.verify(); err != nil {
		_ = node.file.Close()
		return err
	}
	v.layers = append(v.layers, node)
	return nil
}
func (v *trustedPublisherView) verifyLayers() error {
	for _, n := range v.layers {
		// Lower is temporarily a mount target; compare its held underlying inode
		// against the restored path only after its exact mount has been released.
		covered := false
		for _, m := range v.mounts {
			if m.target == n.path {
				covered = true
			}
		}
		if !covered {
			if err := n.verify(); err != nil {
				return err
			}
		}
	}
	return nil
}

func (v *trustedPublisherView) verifyActive() error {
	ns, err := publisherNamespace()
	if err != nil || ns != v.namespace || !v.active || len(v.mounts) != 3 {
		return fmt.Errorf("publisher active namespace/mount count differs")
	}
	if err = v.verifyOriginal(true); err != nil {
		return err
	}
	if err = v.verifyLayers(); err != nil {
		return err
	}
	for _, m := range v.mounts {
		id, err := scannerMountID(m.target)
		if err != nil || id != m.id {
			return fmt.Errorf("publisher active mount identity differs")
		}
	}
	body, err := publisherMountTable()
	if err != nil {
		return err
	}
	rows, err := publisherMountRows(body)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if strings.HasPrefix(row.target, "/usr/share/") && (row.target != canonicalPublisherRoot || row.id != v.mounts[2].id) {
			return fmt.Errorf("publisher descendant mount changed")
		}
	}
	if err = publisherReadOnly(v.lower); err != nil {
		return err
	}
	return publisherReadOnly(canonicalPublisherRoot)
}

func (v *trustedPublisherView) releaseLastMount() error {
	mount := v.mounts[len(v.mounts)-1]
	id, err := scannerMountID(mount.target)
	if err != nil || id != mount.id {
		return fmt.Errorf("publisher release mount identity differs")
	}
	if err = v.verifyOriginal(len(v.mounts) >= 2); err != nil {
		return err
	}
	if err = unix.Unmount(mount.target, unix.UMOUNT_NOFOLLOW); err != nil {
		return fmt.Errorf("publisher exact mount release failed: %w", err)
	}
	after, err := scannerMountID(mount.target)
	if err != nil || after != mount.parent {
		return fmt.Errorf("publisher underlying mount not restored")
	}
	restored, err := anchoredInfo(mount.target)
	if err != nil || !scannerNodeIdentity(mount.underlying, restored) {
		return fmt.Errorf("publisher underlying inode not restored")
	}
	return nil
}

func (v *trustedPublisherView) verifyParent() error {
	ns, err := publisherNamespace()
	if err != nil || ns != v.namespace || ns == v.originalNamespace {
		return fmt.Errorf("publisher current mount namespace changed")
	}
	return v.parent.verify()
}
