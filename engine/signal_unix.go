//go:build !windows

package engine

import "syscall"

var sigWINCH = syscall.SIGWINCH
