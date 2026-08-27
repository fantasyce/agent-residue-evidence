//go:build windows

package process

import (
	"context"
	"os/user"
	"strings"

	psutilnet "github.com/shirou/gopsutil/v4/net"
	psutilprocess "github.com/shirou/gopsutil/v4/process"
)

func processOwnedByCurrentUser(ctx context.Context, process *psutilprocess.Process) (bool, error) {
	current, err := user.Current()
	if err != nil {
		return false, err
	}
	owner, err := process.UsernameWithContext(ctx)
	if err != nil {
		return false, err
	}
	return strings.EqualFold(owner, current.Username), nil
}

func nativeListeningPorts(ctx context.Context, identity Identity) ([]Port, error) {
	if err := processStillMatches(ctx, identity); err != nil {
		return nil, err
	}
	connections, err := psutilnet.ConnectionsPidWithContext(ctx, "tcp", int32(identity.PID))
	if err != nil {
		return nil, err
	}
	return listeningConnections(connections), nil
}

func nativeHoldsAnyPath(ctx context.Context, process *psutilprocess.Process, paths map[string]struct{}) (bool, error) {
	files, err := process.OpenFilesWithContext(ctx)
	if err != nil {
		return false, err
	}
	for _, file := range files {
		if _, matches := paths[file.Path]; matches {
			return true, nil
		}
	}
	return false, nil
}
