//go:build darwin && arm64

// Narrow libSystem trampoline following x/sys v0.44.0's Darwin convention.
// No syscall, path resolution, inventory or second OS algorithm is here.
#include "textflag.h"

TEXT p1_fgetattrlist_trampoline<>(SB),NOSPLIT,$0-0
 JMP p1_libc_fgetattrlist(SB)
GLOBL ·fgetattrlistAddress(SB), RODATA, $8
DATA ·fgetattrlistAddress(SB)/8, $p1_fgetattrlist_trampoline<>(SB)

TEXT p1_fcntl_trampoline<>(SB),NOSPLIT,$0-0
 JMP p1_libc_fcntl(SB)
GLOBL ·fcntlAddress(SB), RODATA, $8
DATA ·fcntlAddress(SB)/8, $p1_fcntl_trampoline<>(SB)
