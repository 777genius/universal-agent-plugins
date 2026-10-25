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
