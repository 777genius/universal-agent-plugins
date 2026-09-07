//go:build darwin && arm64 && cgo

package packageview

/*
#include <sys/resource.h>
static int pv_get(void) {
 return getiopolicy_np(IOPOL_TYPE_VFS_MATERIALIZE_DATALESS_FILES, IOPOL_SCOPE_THREAD);
}
static int pv_set(int value) {
 return setiopolicy_np(IOPOL_TYPE_VFS_MATERIALIZE_DATALESS_FILES, IOPOL_SCOPE_THREAD, value);
}
*/
import "C"

const materializationOff = int(C.IOPOL_MATERIALIZE_DATALESS_FILES_OFF)

func materializationGet() (int, error) {
	n := int(C.pv_get())
	if n < 0 {
		return 0, fail("platform_unavailable")
	}
	return n, nil
}
func materializationSet(n int) error {
	if C.pv_set(C.int(n)) != 0 {
		return fail("platform_unavailable")
	}
	return nil
}

const darwinCGO = true
