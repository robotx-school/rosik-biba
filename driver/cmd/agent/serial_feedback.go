package main

import (
	"errors"
	"io"
	"time"
)

// The serial library represents its idle read timer as EOF. Do not terminate
// the feedback scanner at an idle gap; a closed/unplugged port returns an error
// or is retired by the single serial writer after its next failed write.
type serialFeedbackReader struct {
	io.Reader
	stop <-chan struct{}
}

func (r serialFeedbackReader) Read(p []byte) (int, error) {
	for {
		select {
		case <-r.stop:
			return 0, io.ErrClosedPipe
		default:
		}
		n, err := r.Reader.Read(p)
		if n > 0 || (err != nil && !errors.Is(err, io.EOF)) {
			return n, err
		}
		select {
		case <-r.stop:
			return 0, io.ErrClosedPipe
		case <-time.After(5 * time.Millisecond):
		}
	}
}

// Keep the device offline and quarantined while a USB driver's close is stuck.
// Only one cleanup runs; the serial loop cannot reopen or replay queued pulses.
func (c *controller) retireSerial(port io.Closer, readerDone <-chan struct{}, readerStop chan struct{}) <-chan struct{} {
	c.setSerial(false)
	close(readerStop)
	closed := make(chan struct{})
	go func() { defer close(closed); _ = port.Close(); <-readerDone }()
	return closed
}
