//go:build linux

package control

import (
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestLinuxSerialReadDeadlineAndCloseWithoutPeerReply(t *testing.T) {
	fd, err := unix.Open("/dev/ptmx", unix.O_RDWR|unix.O_NOCTTY|unix.O_NONBLOCK, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer unix.Close(fd)
	if err := unix.IoctlSetPointerInt(fd, unix.TIOCSPTLCK, 0); err != nil {
		t.Fatal(err)
	}
	n, err := unix.IoctlGetInt(fd, unix.TIOCGPTN)
	if err != nil {
		t.Fatal(err)
	}
	port, err := openSerial(fmt.Sprintf("/dev/pts/%d", n), 115200)
	if err != nil {
		t.Fatal(err)
	}
	defer port.Close()
	deadlinePort, ok := port.(interface{ SetReadDeadline(time.Time) error })
	if !ok {
		t.Fatal("serial port does not expose a read deadline")
	}
	if err := deadlinePort.SetReadDeadline(time.Now().Add(40 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	if _, err := port.Write([]byte("PACKET_MODE\n?$\x34\x00\x00")); err != nil {
		t.Fatal(err)
	}
	var b [1]byte
	start := time.Now()
	_, err = port.Read(b[:])
	if !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatalf("want deadline exceeded, got %v", err)
	}
	if time.Since(start) > time.Second {
		t.Fatal("serial read deadline was not honored")
	}
	closed := make(chan error, 1)
	go func() { closed <- port.Close() }()
	select {
	case err := <-closed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("close blocked on unanswered output")
	}
}
