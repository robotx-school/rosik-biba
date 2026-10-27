package control

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/tarm/serial"
)

// Device allows one in-flight serial request, including the reply. Failed
// commands are never retried: their physical outcome may be unknown.
type Device struct {
	name string
	baud int
	gate chan struct{}
	port io.ReadWriteCloser
}

func NewDevice(name string, baud int) *Device {
	return &Device{name: name, baud: baud, gate: make(chan struct{}, 1)}
}
func (d *Device) Close() error {
	d.gate <- struct{}{}
	defer func() { <-d.gate }()
	if d.port == nil {
		return nil
	}
	err := d.port.Close()
	d.port = nil
	return err
}
func (d *Device) Execute(ctx context.Context, c Command) (Result, error) {
	packet, err := c.Packet()
	if err != nil {
		return Result{}, err
	}
	// Bound the whole operation even for callers without a deadline.
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	queueTimer := time.NewTimer(150 * time.Millisecond)
	defer queueTimer.Stop()
	select {
	case <-queueTimer.C:
		return Result{}, errors.New("serial busy; command was not queued")
	case d.gate <- struct{}{}:
	case <-ctx.Done():
		return Result{}, ctx.Err()
	}
	defer func() { <-d.gate }()
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if d.port == nil {
		p, err := serial.OpenPort(&serial.Config{Name: d.name, Baud: d.baud, ReadTimeout: 50 * time.Millisecond})
		if err != nil {
			return Result{}, fmt.Errorf("open manipulator serial: %w", err)
		}
		d.port = p
		// Opening common ESP32 USB bridges resets the chip. Never send a movement
		// before startup has completed; no automatic calibration or target replay.
		timer := time.NewTimer(1500 * time.Millisecond)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			_ = d.port.Close()
			d.port = nil
			return Result{}, ctx.Err()
		}
		if err := p.Flush(); err != nil {
			_ = d.port.Close()
			d.port = nil
			return Result{}, err
		}
		if _, err := io.WriteString(p, "PACKET_MODE\n"); err != nil {
			_ = d.port.Close()
			d.port = nil
			return Result{}, err
		}
	}
	port := d.port
	// Closing a stuck port releases reads/writes when the request expires.
	stop := context.AfterFunc(ctx, func() { _ = port.Close() })
	defer stop()
	fail := func(err error) (Result, error) {
		_ = port.Close()
		d.port = nil
		if ctx.Err() != nil {
			return Result{}, ctx.Err()
		}
		return Result{}, err
	}
	if err := WritePacket(port, packet); err != nil {
		return fail(err)
	}
	reply, err := ReadPacket(contextReader{ctx: ctx, r: port})
	if err != nil {
		return fail(err)
	}
	result, err := decodeResult(packet, reply)
	if err != nil {
		var rejected *FirmwareError
		if errors.As(err, &rejected) {
			return Result{}, err
		}
		return fail(err)
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	return result, nil
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	for {
		if err := r.ctx.Err(); err != nil {
			return 0, err
		}
		n, err := r.r.Read(p)
		if n > 0 {
			return n, err
		}
		// tarm/serial returns EOF when its termios read timer expires.
		if err != nil && !errors.Is(err, io.EOF) {
			return 0, err
		}
		select {
		case <-r.ctx.Done():
			return 0, r.ctx.Err()
		case <-time.After(time.Millisecond):
		}
	}
}
