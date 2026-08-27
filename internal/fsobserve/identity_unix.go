//go:build darwin || linux

package fsobserve

import (
	"fmt"
	"os"
	"syscall"
)

func objectIdentity(_ string, info os.FileInfo) (string, error) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return "", fmt.Errorf("native file identity unavailable")
	}
	return fmt.Sprintf("%d:%d:%d", stat.Dev, stat.Ino, info.Mode()&os.ModeType), nil
}
