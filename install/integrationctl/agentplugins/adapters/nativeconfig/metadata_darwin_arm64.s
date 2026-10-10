//go:build darwin && arm64

#include "textflag.h"

TEXT plain_getattr_trampoline<>(SB),NOSPLIT,$0-0
	JMP plain_libc_fgetattrlist(SB)
GLOBL ·plainGetattrAddress(SB), RODATA, $8
DATA ·plainGetattrAddress(SB)/8, $plain_getattr_trampoline<>(SB)

TEXT plain_list_trampoline<>(SB),NOSPLIT,$0-0
	JMP plain_libc_flistxattr(SB)
GLOBL ·plainListAddress(SB), RODATA, $8
DATA ·plainListAddress(SB)/8, $plain_list_trampoline<>(SB)

TEXT plain_get_trampoline<>(SB),NOSPLIT,$0-0
	JMP plain_libc_fgetxattr(SB)
GLOBL ·plainGetAddress(SB), RODATA, $8
DATA ·plainGetAddress(SB)/8, $plain_get_trampoline<>(SB)
