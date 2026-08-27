//go:build linux || windows

package process

import (
	"strings"

	psutilnet "github.com/shirou/gopsutil/v4/net"
)

func listeningConnections(connections []psutilnet.ConnectionStat) []Port {
	ports := []Port{}
	for _, connection := range connections {
		if !strings.EqualFold(connection.Status, "LISTEN") || connection.Raddr.Port != 0 {
			continue
		}
		ports = append(ports, Port{Protocol: "tcp", Address: connection.Laddr.IP, Number: int(connection.Laddr.Port)})
	}
	return ports
}
