//go:build darwin && !cgo

package process

import (
	"context"
	"errors"

	psutilprocess "github.com/shirou/gopsutil/v4/process"
)

func processOwnedByCurrentUser(context.Context, *psutilprocess.Process) (bool, error) {
	return false, errors.New("macOS native process observation requires cgo")
}

func nativeListeningPorts(context.Context, Identity) ([]Port, error) {
	return nil, errors.New("macOS native port observation requires cgo")
}

func nativeHoldsAnyPath(context.Context, *psutilprocess.Process, map[string]struct{}) (bool, error) {
	return false, errors.New("macOS native process observation requires cgo")
}
