//go:build windows

package fsobserve

import (
	"fmt"
	"os"
	"syscall"
)

func objectIdentity(path string, info os.FileInfo) (string, error) {
	pathPtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return "", err
	}
	handle, err := syscall.CreateFile(
		pathPtr,
		0,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE,
		nil,
		syscall.OPEN_EXISTING,
		syscall.FILE_FLAG_BACKUP_SEMANTICS|syscall.FILE_FLAG_OPEN_REPARSE_POINT,
		0,
	)
	if err != nil {
		return "", err
	}
	defer syscall.CloseHandle(handle)
	var fileInfo syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(handle, &fileInfo); err != nil {
		return "", err
	}
	return fmt.Sprintf("%d:%d:%d:%d", fileInfo.VolumeSerialNumber, fileInfo.FileIndexHigh, fileInfo.FileIndexLow, info.Mode()&os.ModeType), nil
}
