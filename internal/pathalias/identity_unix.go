//go:build darwin || linux

package pathalias

import (
	"fmt"
	"os"
	"syscall"
)

func stableIdentity(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return "", fmt.Errorf("native root identity unavailable")
	}
	return fmt.Sprintf("%d:%d:%d", stat.Dev, stat.Ino, info.Mode()&os.ModeType), nil
}
