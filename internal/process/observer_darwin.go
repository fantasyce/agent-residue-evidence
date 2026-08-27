//go:build darwin && cgo

package process

/*
#cgo LDFLAGS: -lproc
#include <arpa/inet.h>
#include <errno.h>
#include <libproc.h>
#include <netinet/in.h>
#include <stdlib.h>
#include <string.h>
#include <sys/proc_info.h>

struct are_port {
	char address[INET6_ADDRSTRLEN];
	int number;
};

static int are_listening_ports(pid_t pid, struct are_port *ports, int capacity) {
	int bytes = proc_pidinfo(pid, PROC_PIDLISTFDS, 0, NULL, 0);
	if (bytes <= 0) return -errno;
	struct proc_fdinfo *fds = malloc((size_t)bytes);
	if (fds == NULL) return -ENOMEM;
	bytes = proc_pidinfo(pid, PROC_PIDLISTFDS, 0, fds, bytes);
	if (bytes <= 0) { int saved = errno; free(fds); return -saved; }
	int count = bytes / (int)sizeof(struct proc_fdinfo);
	int written = 0;
	for (int i = 0; i < count && written < capacity; i++) {
		if (fds[i].proc_fdtype != PROX_FDTYPE_SOCKET) continue;
		struct socket_fdinfo socket_info;
		int got = proc_pidfdinfo(pid, fds[i].proc_fd, PROC_PIDFDSOCKETINFO, &socket_info, sizeof(socket_info));
		if (got != sizeof(socket_info)) continue;
		if (socket_info.psi.soi_kind != SOCKINFO_TCP) continue;
		struct tcp_sockinfo *tcp = &socket_info.psi.soi_proto.pri_tcp;
		if (tcp->tcpsi_state != TSI_S_LISTEN) continue;
		struct in_sockinfo *in = &tcp->tcpsi_ini;
		const void *address = NULL;
		int family = AF_UNSPEC;
		if (in->insi_vflag & INI_IPV4) {
			family = AF_INET;
			address = &in->insi_laddr.ina_46.i46a_addr4;
		} else if (in->insi_vflag & INI_IPV6) {
			family = AF_INET6;
			address = &in->insi_laddr.ina_6;
		}
		if (address == NULL || inet_ntop(family, address, ports[written].address, sizeof(ports[written].address)) == NULL) continue;
		ports[written].number = ntohs((uint16_t)in->insi_lport);
		written++;
	}
	free(fds);
	return written;
}
*/
import "C"

import (
	"context"
	"fmt"
	"os"
	"unsafe"

	psutilprocess "github.com/shirou/gopsutil/v4/process"
)

func processOwnedByCurrentUser(ctx context.Context, process *psutilprocess.Process) (bool, error) {
	uids, err := process.UidsWithContext(ctx)
	if err != nil || len(uids) == 0 {
		return false, err
	}
	return uids[0] == uint32(os.Getuid()), nil
}

func nativeListeningPorts(ctx context.Context, identity Identity) ([]Port, error) {
	if err := processStillMatches(ctx, identity); err != nil {
		return nil, err
	}
	const capacity = 4096
	buffer := make([]C.struct_are_port, capacity)
	count := int(C.are_listening_ports(C.pid_t(identity.PID), &buffer[0], capacity))
	if count < 0 {
		return nil, fmt.Errorf("proc_pidinfo failed with errno %d", -count)
	}
	ports := make([]Port, 0, count)
	for index := 0; index < count; index++ {
		address := C.GoString((*C.char)(unsafe.Pointer(&buffer[index].address[0])))
		ports = append(ports, Port{Protocol: "tcp", Address: address, Number: int(buffer[index].number)})
	}
	return ports, nil
}

func nativeHoldsAnyPath(context.Context, *psutilprocess.Process, map[string]struct{}) (bool, error) {
	return false, nil
}
