//go:build windows

package krypticdev

import (
	"bufio"
	"errors"
	"net"
	"os"
	"strings"
	"time"
)

func socketPath() string {
	if override := os.Getenv("KRYPTIC_SOCKET_PATH"); override != "" {
		return override
	}
	return `\\.\pipe\kryptic-daemon`
}

// roundTrip writes one NDJSON request line to the daemon and reads the one
// response line back.
//
// The daemon serves a byte-mode named pipe, so a plain file handle works — no
// win32 bindings and no dependency for consumers. The timeout covers connecting
// (the pipe can briefly report "busy" between served clients); the read then
// blocks until the daemon replies, which it does immediately or not at all —
// matching the .NET client's semantics. A KRYPTIC_SOCKET_PATH override that is
// not a pipe path (tests use unix sockets) is dialed as a unix socket, which
// Windows supports natively.
func roundTrip(line []byte, timeout time.Duration) ([]byte, error) {
	path := socketPath()
	if !strings.HasPrefix(path, `\\.\pipe\`) {
		return roundTripUnix(path, line, timeout)
	}

	deadline := time.Now().Add(timeout)
	var pipe *os.File
	for {
		var err error
		pipe, err = os.OpenFile(path, os.O_RDWR, 0)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			return nil, errors.New("timed out connecting to the daemon pipe")
		}
		time.Sleep(50 * time.Millisecond)
	}
	defer pipe.Close()

	if _, err := pipe.Write(line); err != nil {
		return nil, err
	}

	return bufio.NewReader(pipe).ReadBytes('\n')
}

func roundTripUnix(path string, line []byte, timeout time.Duration) ([]byte, error) {
	connection, err := net.DialTimeout("unix", path, timeout)
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
