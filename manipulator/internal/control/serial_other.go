//go:build !linux

package control

import (
	"github.com/tarm/serial"
	"time"
)

func openSerial(name string, baud int) (serialConnection, error) {
	return serial.OpenPort(&serial.Config{Name: name, Baud: baud, ReadTimeout: 50 * time.Millisecond})
}
