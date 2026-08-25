//go:build linux

package app

import "syscall"

const sysOpenAt = syscall.SYS_OPENAT
