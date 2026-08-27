package process

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"testing"
	"time"
)

func TestProcessHelper(t *testing.T) {
	if os.Getenv("ARE_PROCESS_HELPER") != "1" {
		return
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		os.Exit(2)
	}
	fmt.Println(listener.Addr().(*net.TCPAddr).Port)
	_, _ = bufio.NewReader(os.Stdin).ReadByte()
	_ = listener.Close()
	os.Exit(0)
}

func TestNativeAttributionAndPortOwnership(t *testing.T) {
	root := t.TempDir()
	command := exec.Command(os.Args[0], "-test.run=^TestProcessHelper$")
	command.Dir = root
	command.Env = append(os.Environ(), "ARE_PROCESS_HELPER=1")
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdin, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = stdin.Close()
		_ = command.Wait()
	})
	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	wantPort, err := strconv.Atoi(line[:len(line)-1])
	if err != nil {
		t.Fatal(err)
	}

	observer := NewNativeObserver([]string{root})
	deadline := time.Now().Add(5 * time.Second)
	for {
		got, limitations := observer.Resolve(context.Background(), Hints{})
		for _, evidence := range got {
			if evidence.Identity.PID != command.Process.Pid {
				continue
			}
			for _, port := range evidence.Ports {
				if port.Number == wantPort && port.Address == "127.0.0.1" {
					return
				}
			}
		}
		if time.Now().After(deadline) {
			baseline, baselineErr := observer.Baseline(context.Background())
			var helperMetadata []Metadata
			for _, metadata := range baseline.Processes {
				if metadata.Identity.PID == command.Process.Pid {
					helperMetadata = append(helperMetadata, metadata)
				}
			}
			t.Fatalf("helper pid=%d port=%d not attributed; metadata=%#v baseline_err=%v got=%#v limitations=%#v", command.Process.Pid, wantPort, helperMetadata, baselineErr, got, limitations)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
