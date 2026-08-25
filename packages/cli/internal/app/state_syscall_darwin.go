//go:build darwin

package app

// SYS_openat from the Darwin system call table. The standard syscall package
// exposes Syscall6 but not this constant on Darwin.
const sysOpenAt = 463
