//go:build linux

package control

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// Keep O_NONBLOCK set so Go can interrupt serial I/O and Linux does not wait
// for a stalled USB transmitter to drain when the last descriptor is closed.
type linuxSerial struct{ *os.File }

func openSerial(name string, baud int) (serialConnection, error) {
	rates := map[int]uint32{9600: unix.B9600, 19200: unix.B19200, 38400: unix.B38400, 57600: unix.B57600, 115200: unix.B115200, 230400: unix.B230400, 460800: unix.B460800, 921600: unix.B921600, 1000000: unix.B1000000}
	rate, ok := rates[baud]
	if !ok {
		return nil, fmt.Errorf("unsupported serial baud %d", baud)
	}
	file, err := os.OpenFile(name, unix.O_RDWR|unix.O_NOCTTY|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	conn, err := file.SyscallConn()
	if err != nil {
		file.Close()
		return nil, err
	}
	var configureErr error
	err = conn.Control(func(fd uintptr) {
		// Do not use File.Fd(): it can put the descriptor back in blocking mode.
		term := &unix.Termios{Iflag: unix.IGNPAR, Cflag: unix.CREAD | unix.CLOCAL | unix.CS8 | rate, Ispeed: rate, Ospeed: rate}
		term.Cc[unix.VMIN] = 1
		configureErr = unix.IoctlSetTermios(int(fd), unix.TCSETS, term)
	})
	if err != nil {
		file.Close()
		return nil, err
	}
	if configureErr != nil {
		file.Close()
		return nil, configureErr
	}
	return &linuxSerial{file}, nil
}
func (p *linuxSerial) Flush() error {
	conn, err := p.SyscallConn()
	if err != nil {
		return err
	}
	var flushErr error
	err = conn.Control(func(fd uintptr) { flushErr = unix.IoctlSetInt(int(fd), unix.TCFLSH, unix.TCIOFLUSH) })
	if err != nil {
		return err
	}
	return flushErr
}
