//go:build linux

package cli

import (
	"fmt"
	"os"
	"sync"
	"syscall"
	"unsafe"
)

// Reopen stdin as a Go-managed pollable descriptor. A blocking descriptor
// inherited from a terminal cannot otherwise be interrupted by Close.
func InputFile(file *os.File) (*os.File, error) {
	return os.Open(fmt.Sprintf("/proc/self/fd/%d", file.Fd()))
}

// RawTerminal enables editing without echo, retaining output processing.
// The restore closure must run before the process exits.
func RawTerminal(file *os.File) (func(), bool, error) {
	var old syscall.Termios
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, file.Fd(), syscall.TCGETS, uintptr(unsafe.Pointer(&old)))
	if errno != 0 {
		return func() {}, false, nil
	} // Pipe/file: plain line mode.
	raw := old
	raw.Lflag &^= syscall.ICANON | syscall.ECHO | syscall.ISIG
	raw.Iflag &^= syscall.ICRNL | syscall.IXON
	raw.Cc[syscall.VMIN] = 1
	raw.Cc[syscall.VTIME] = 0
	_, _, errno = syscall.Syscall(syscall.SYS_IOCTL, file.Fd(), syscall.TCSETS, uintptr(unsafe.Pointer(&raw)))
	if errno != 0 {
		return nil, false, errno
	}
	var once sync.Once
	return func() {
		once.Do(func() { syscall.Syscall(syscall.SYS_IOCTL, file.Fd(), syscall.TCSETS, uintptr(unsafe.Pointer(&old))) })
	}, true, nil
}
