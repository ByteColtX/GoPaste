//go:build !windows

package lansync

import "syscall"

// setSocketOptions prepares the announcement socket: several instances may bind
// the same port, and sending to a subnet broadcast address needs permission.
func setSocketOptions(fd uintptr) error {
	if err := syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1); err != nil {
		return err
	}
	return syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_BROADCAST, 1)
}
