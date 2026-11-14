//go:build linux

package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

func validateDisplayReady(lock []byte, lockInfo, socketInfo os.FileInfo, proof isolationProof) error {
	pid, err := strconv.Atoi(strings.TrimSpace(string(lock)))
	if err != nil || pid <= 0 || pid != proof.PID {
		return fmt.Errorf("new display lock PID differs")
	}
	lockUID, err := statUID(lockInfo)
	if err != nil || int64(lockUID) != int64(proof.UID) || !regular(lockInfo, 64) {
		return fmt.Errorf("display lock owner/type differs")
	}
	socketUID, err := statUID(socketInfo)
	st, ok := socketInfo.Sys().(*syscall.Stat_t)
	if err != nil || int64(socketUID) != int64(proof.UID) || !ok || st.Nlink != 1 || socketInfo.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("new display socket identity differs")
	}
	return nil
}

func (c *controller) retainReadyDisplay(lock []byte, lockInfo, socketInfo os.FileInfo, proof isolationProof) error {
	if e := validateDisplayReady(lock, lockInfo, socketInfo, proof); e != nil {
		return e
	}
	after, ae := os.Lstat("/tmp/.X11-unix/X93")
	if ae != nil || !sameFile(socketInfo, after) {
		return fmt.Errorf("display socket changed during readiness")
	}
	assets, holdErr := holdDisplayAssets("/tmp/.X93-lock", "/tmp/.X11-unix/X93", lockInfo, socketInfo)
	if holdErr != nil {
		return holdErr
	}
	c.displayAssets = assets
	return nil
}
