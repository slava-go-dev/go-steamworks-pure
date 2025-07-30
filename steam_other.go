//go:build !windows

package go_steamworks_pure

import "github.com/ebitengine/purego"

func loadSystemLib(path string) (uintptr, error) {
	return purego.Dlopen(path, purego.RTLD_NOW|purego.RTLD_GLOBAL)
}

func closeLibc(libc uintptr) {
	_ = purego.Dlclose(libc)
}
