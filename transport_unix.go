//go:build !windows

package krypticdev

import (
	"bufio"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

func socketPath() string {
	if override := os.Getenv("KRYPTIC_SOCKET_PATH"); override != "" {
		return override
	}
	if runtime.GOOS == "linux" {
		if runtimeDir := os.Getenv("XDG_RUNTIME_DIR"); runtimeDir != "" {
			return filepath.Join(runtimeDir, "kryptic-daemon.sock")
		}
	}
	return "/tmp/kryptic-daemon.sock"
}

// roundTrip writes one NDJSON request line to the daemon socket and reads the
// one response line back.
func roundTrip(line []byte, timeout time.Duration) ([]byte, error) {
	connection, err := net.DialTimeout("unix", socketPath(), timeout)
	if err != nil {
		return nil, err
	}
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(timeout))

	if _, err := connection.Write(line); err != nil {
		return nil, err
	}

	return bufio.NewReader(connection).ReadBytes('\n')
}
