//go:build windows

package go_steamworks_pure

import (
	"golang.org/x/sys/windows"
)

func loadSystemLib(path string) (uintptr, error) {
	return windows.NewLazyDLL(path).Handle(), nil
}

func closeLibc(libc uintptr) {
}
