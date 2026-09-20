// Linux test harness only. Fail closed if confinement cannot be installed.
// The initial exec uses the exact pre-exec pathname pointer; every subsequent
// exec in the Go image is killed. This is a test trap, not a hostile-code sandbox.
#define _GNU_SOURCE
#include <errno.h>
#include <fcntl.h>
#include <linux/audit.h>
#include <linux/filter.h>
#include <linux/landlock.h>
#include <linux/seccomp.h>
#include <sched.h>
#include <stddef.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <sys/prctl.h>
#include <sys/syscall.h>
#include <unistd.h>

#define RO (LANDLOCK_ACCESS_FS_READ_FILE | LANDLOCK_ACCESS_FS_READ_DIR)
#define RW (RO | LANDLOCK_ACCESS_FS_WRITE_FILE | LANDLOCK_ACCESS_FS_REMOVE_DIR | LANDLOCK_ACCESS_FS_REMOVE_FILE | LANDLOCK_ACCESS_FS_MAKE_DIR | LANDLOCK_ACCESS_FS_MAKE_REG | LANDLOCK_ACCESS_FS_MAKE_SYM | LANDLOCK_ACCESS_FS_REFER | LANDLOCK_ACCESS_FS_TRUNCATE)
static void die(const char *what) { perror(what); exit(125); }
static void allow(int rules, const char *path, uint64_t access) {
 int fd = open(path, O_PATH | O_CLOEXEC);
 if (fd < 0) die("guard path");
 struct landlock_path_beneath_attr rule = {.allowed_access=access,.parent_fd=fd};
 if (syscall(SYS_landlock_add_rule,rules,LANDLOCK_RULE_PATH_BENEATH,&rule,0)) die("guard rule");
 close(fd);
}
#define KILL SECCOMP_RET_KILL_PROCESS
#define DENY(n) BPF_JUMP(BPF_JMP|BPF_JEQ|BPF_K,n,0,1), BPF_STMT(BPF_RET|BPF_K,KILL)
int main(int argc,char **argv) {
 // guard binary scratch project-or-dash read|write -- command args...
 if (argc<7) return 125;
 if (prctl(PR_SET_NO_NEW_PRIVS,1,0,0,0)) die("guard no-new-privileges");
 struct landlock_ruleset_attr attr = {.handled_access_fs=RW | LANDLOCK_ACCESS_FS_EXECUTE};
 int rules=syscall(SYS_landlock_create_ruleset,&attr,sizeof(attr),0);
 if(rules<0) die("guard ruleset");
 allow(rules,argv[1],LANDLOCK_ACCESS_FS_READ_FILE|LANDLOCK_ACCESS_FS_EXECUTE);
 allow(rules,argv[2],RW);
 if(argv[3][0]!='-') allow(rules,argv[3],argv[4][0]=='w'?RW:RO);
 allow(rules,"/dev/null",LANDLOCK_ACCESS_FS_READ_FILE|LANDLOCK_ACCESS_FS_WRITE_FILE);
 // Native cgo binaries may need the system ELF interpreter and libraries.
 allow(rules,"/lib",RO|LANDLOCK_ACCESS_FS_EXECUTE);
 allow(rules,"/lib64",RO|LANDLOCK_ACCESS_FS_EXECUTE);
 allow(rules,"/etc/ld.so.cache",LANDLOCK_ACCESS_FS_READ_FILE);
 allow(rules,"/proc",RO);
 allow(rules,"/sys",RO);
 if(syscall(SYS_landlock_restrict_self,rules,0)) die("guard restrict");
 close(rules);
 uintptr_t executable=(uintptr_t)argv[1];
 struct sock_filter filter[]={
  BPF_STMT(BPF_LD|BPF_W|BPF_ABS,offsetof(struct seccomp_data,arch)),
#if defined(__x86_64__)
  BPF_JUMP(BPF_JMP|BPF_JEQ|BPF_K,AUDIT_ARCH_X86_64,1,0),
#elif defined(__aarch64__)
  BPF_JUMP(BPF_JMP|BPF_JEQ|BPF_K,AUDIT_ARCH_AARCH64,1,0),
#else
#error Unsupported guard architecture
#endif
  BPF_STMT(BPF_RET|BPF_K,KILL),
  BPF_STMT(BPF_LD|BPF_W|BPF_ABS,offsetof(struct seccomp_data,nr)),
  DENY(SYS_socket),DENY(SYS_socketpair),DENY(SYS_connect),DENY(SYS_bind),DENY(SYS_listen),
  DENY(SYS_sendto),DENY(SYS_sendmsg),DENY(SYS_recvfrom),DENY(SYS_recvmsg),DENY(SYS_execveat),
#ifdef SYS_fork
  DENY(SYS_fork),DENY(SYS_vfork),
#endif
  BPF_JUMP(BPF_JMP|BPF_JEQ|BPF_K,SYS_clone,0,4),
  BPF_STMT(BPF_LD|BPF_W|BPF_ABS,offsetof(struct seccomp_data,args[0])),
  BPF_JUMP(BPF_JMP|BPF_JSET|BPF_K,CLONE_THREAD,1,0),
  BPF_STMT(BPF_RET|BPF_K,KILL),
  BPF_STMT(BPF_RET|BPF_K,SECCOMP_RET_ALLOW),
  // Go uses clone for threads. No clone3 flags pointer inspection is attempted.
  BPF_JUMP(BPF_JMP|BPF_JEQ|BPF_K,SYS_clone3,0,1),
  BPF_STMT(BPF_RET|BPF_K,SECCOMP_RET_ERRNO|ENOSYS),
  BPF_JUMP(BPF_JMP|BPF_JEQ|BPF_K,SYS_execve,0,6),
  BPF_STMT(BPF_LD|BPF_W|BPF_ABS,offsetof(struct seccomp_data,args[0])),
  BPF_JUMP(BPF_JMP|BPF_JEQ|BPF_K,(uint32_t)executable,0,3),
  BPF_STMT(BPF_LD|BPF_W|BPF_ABS,offsetof(struct seccomp_data,args[0])+4),
  BPF_JUMP(BPF_JMP|BPF_JEQ|BPF_K,(uint32_t)(executable>>32),0,1),
  BPF_STMT(BPF_RET|BPF_K,SECCOMP_RET_ALLOW),
  BPF_STMT(BPF_RET|BPF_K,KILL),
  BPF_STMT(BPF_RET|BPF_K,SECCOMP_RET_ALLOW),
 };
 struct sock_fprog program={.len=sizeof(filter)/sizeof(filter[0]),.filter=filter};
 if(prctl(PR_SET_SECCOMP,SECCOMP_MODE_FILTER,&program)) die("guard seccomp");
 argv[5]=argv[1];
 execv(argv[1],argv+5);
 die("guard exec");
}
