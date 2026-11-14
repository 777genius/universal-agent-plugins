//go:build linux

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func (c *controller) validatePublisherRuntime(value any, publisher, driver isolationProof) error {
	if c.publisherView == nil {
		return fmt.Errorf("publisher private view missing")
	}
	info, err := os.Stat(canonicalPublisherRoot + "/code")
	if err != nil {
		return err
	}
	identity, err := policyStat(info)
	if err != nil {
		return err
	}
	body, err := publisherDriverCgroup(driver)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(body)
	return validatePublisherRuntimeFacts(value, publisher, c.publisherView.namespace, identity, hex.EncodeToString(digest[:]))
}

func validatePublisherRuntimeFacts(value any, publisher isolationProof, namespace uint64, executable policyIdentity, cgroup string) error {
	m, err := objectKeys(value, strings.Fields("pid mount_namespace network_namespace apparmor_label executable_device executable_inode cgroup_sha256 uid gid capabilities_zero no_new_privileges"), nil)
	if err != nil {
		return err
	}
	if m["apparmor_label"] != "vscode (unconfined)" || m["capabilities_zero"] != true || m["no_new_privileges"] != true || !shaPattern.MatchString(cgroup) || m["cgroup_sha256"] != cgroup {
		return fmt.Errorf("publisher post-exec policy/privilege/cgroup differs")
	}
	want := map[string]string{"pid": strconv.Itoa(publisher.PID), "mount_namespace": strconv.FormatUint(namespace, 10), "network_namespace": strconv.FormatUint(publisher.Namespace, 10), "executable_device": strconv.FormatUint(executable.Device, 10), "executable_inode": strconv.FormatUint(executable.Inode, 10), "uid": strconv.Itoa(publisher.UID), "gid": strconv.Itoa(publisher.GID)}
	for name, expected := range want {
		number, ok := m[name].(json.Number)
		actual, err := number.Int64()
		if !ok || err != nil || actual <= 0 || number.String() != expected {
			return fmt.Errorf("publisher post-exec numeric identity differs")
		}
	}
	return nil
}

func publisherDriverCgroup(driver isolationProof) ([]byte, error) {
	return publisherDriverCgroupUsing(driver, readPublisherProc)
}
func publisherDriverCgroupUsing(driver isolationProof, read func(string, int64, uint32) ([]byte, error)) ([]byte, error) {
	if driver.PID <= 0 || driver.UID <= 0 || driver.UID > 1<<30 {
		return nil, fmt.Errorf("live driver identity refused")
	}
	root := "/proc/" + strconv.Itoa(driver.PID)
	before, err := read(root+"/stat", 8192, uint32(driver.UID))
	if err != nil {
		return nil, err
	}
	born, err := publisherStartTime(before)
	if err != nil {
		return nil, err
	}
	body, err := read(root+"/cgroup", 4096, uint32(driver.UID))
	if err != nil {
		return nil, err
	}
	after, err := read(root+"/stat", 8192, uint32(driver.UID))
	if err != nil {
		return nil, err
	}
	final, err := publisherStartTime(after)
	if err != nil || born != final {
		return nil, fmt.Errorf("live driver process identity changed")
	}
	return body, nil
}
func readPublisherProc(path string, limit int64, uid uint32) ([]byte, error) {
	body, file, _, err := policyRead(path, limit, true, uid)
	if err != nil {
		return nil, err
	}
	return body, file.Close()
}
func publisherProcessStart(root string, uid uint32) (string, error) {
	body, file, _, err := policyRead(root+"/stat", 8192, true, uid)
	if err != nil {
		return "", err
	}
	if err = file.Close(); err != nil {
		return "", err
	}
	return publisherStartTime(body)
}
func publisherStartTime(body []byte) (string, error) {
	end := strings.LastIndexByte(string(body), ')')
	if end < 0 {
		return "", fmt.Errorf("publisher proc stat refused")
	}
	fields := strings.Fields(string(body[end+1:]))
	if len(fields) < 20 {
		return "", fmt.Errorf("publisher proc stat truncated")
	}
	start, err := strconv.ParseUint(fields[19], 10, 64)
	if err != nil || start == 0 || strconv.FormatUint(start, 10) != fields[19] {
		return "", fmt.Errorf("publisher proc start identity refused")
	}
	return fields[19], nil
}
