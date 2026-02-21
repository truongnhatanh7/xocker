package sync

import (
	"fmt"
	"io"
	"os"
	"time"

	"golang.org/x/sys/unix"
)

func CreateSocketPair() (*os.File, *os.File, error) {
	fds, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM, 0)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create socketpair: %w", err)
	}

	parentConn := os.NewFile(uintptr(fds[0]), "parent-sync")
	childConn := os.NewFile(uintptr(fds[1]), "child-sync")

	if parentConn == nil || childConn == nil {
		return nil, nil, fmt.Errorf("failed to create os.File from socketpair fds")
	}

	return parentConn, childConn, nil
}

func SignalReady(conn *os.File, config string) error {
	if conn == nil {
		return fmt.Errorf("connection is nil")
	}

	message := []byte("READY\n" + config)
	n, err := conn.Write(message)
	if err != nil {
		return fmt.Errorf("failed to write ready signal: %w", err)
	}

	if n != len(message) {
		return fmt.Errorf("incomplete write: wrote %d bytes, expected %d", n, len(message))
	}

	return nil
}

func WaitForReady(conn *os.File, timeout time.Duration) (string, error) {
	if conn == nil {
		return "", fmt.Errorf("connection is nil")
	}

	type result struct {
		config string
		err    error
	}
	resultChan := make(chan result, 1)

	go func() {
		buf := make([]byte, 6)
		n, err := io.ReadFull(conn, buf)
		if err != nil {
			resultChan <- result{"", fmt.Errorf("failed to read ready signal: %w", err)}
			return
		}

		if n != 6 || string(buf) != "READY\n" {
			resultChan <- result{"", fmt.Errorf("received invalid signal: %q (expected \"READY\\n\")", string(buf))}
			return
		}

		configBuf := make([]byte, 1024)
		n, err = conn.Read(configBuf)
		if err != nil && err != io.EOF {
			resultChan <- result{"", fmt.Errorf("failed to read config data: %w", err)}
			return
		}

		resultChan <- result{string(configBuf[:n]), nil}
	}()

	select {
	case res := <-resultChan:
		return res.config, res.err
	case <-time.After(timeout):
		return "", fmt.Errorf("timeout waiting for ready signal after %v", timeout)
	}
}
