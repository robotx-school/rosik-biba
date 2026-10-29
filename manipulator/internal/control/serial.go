package control

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"
)

type serialConnection interface {
	io.ReadWriteCloser
	Flush() error
}

// Device allows one in-flight serial request, including the reply. Failed
// commands are never retried: their physical outcome may be unknown.
type Device struct {
	name    string
	baud    int
	gate    chan struct{}
	port    io.ReadWriteCloser
	closing <-chan struct{}
}

func NewDevice(name string, baud int) *Device {
	return &Device{name: name, baud: baud, gate: make(chan struct{}, 1)}
}

// retire closes outside the request goroutine: Linux USB drivers can block
// inside close(2) while draining a failed device. Quarantine it until done.
func (d *Device) retire(port io.ReadWriteCloser) {
	d.port = nil
	done := make(chan struct{})
	d.closing = done
	go func() { defer close(done); _ = port.Close() }()
}
func (d *Device) Close() error {
	d.gate <- struct{}{}
	defer func() { <-d.gate }()
	if d.port != nil {
		d.retire(d.port)
	}
	if d.closing == nil {
		return nil
	}
	select {
	case <-d.closing:
		return nil
	case <-time.After(3 * time.Second):
		return errors.New("USB serial driver close is still pending")
	}
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
	if d.closing != nil {
		select {
		case <-d.closing:
			d.closing = nil
		default:
			return Result{}, errors.New("serial port is still closing; reconnect the device if this persists")
		}
	}
	if d.port == nil {
		p, err := openSerial(d.name, d.baud)
		if err != nil {
			return Result{}, fmt.Errorf("open manipulator serial: %w", err)
		}
		d.port = p
		if deadlinePort, ok := p.(interface{ SetDeadline(time.Time) error }); ok {
			deadline, _ := ctx.Deadline()
			if err := deadlinePort.SetDeadline(deadline); err != nil {
				d.retire(p)
				return Result{}, err
			}
		}
		// Opening common ESP32 USB bridges resets the chip. Never send a movement
		// before startup has completed; no automatic calibration or target replay.
		timer := time.NewTimer(1500 * time.Millisecond)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			d.retire(d.port)
			return Result{}, ctx.Err()
		}
		if err := p.Flush(); err != nil {
			d.retire(d.port)
			return Result{}, err
		}
		if _, err := io.WriteString(p, "PACKET_MODE\n"); err != nil {
			d.retire(d.port)
			return Result{}, err
		}
	}
	port := d.port
	if deadlinePort, ok := port.(interface{ SetDeadline(time.Time) error }); ok {
		deadline, _ := ctx.Deadline()
		if err := deadlinePort.SetDeadline(deadline); err != nil {
			d.retire(port)
			return Result{}, err
		}
	}
	// Interrupt pollable I/O with a deadline, not Close: the driver's close
	// may itself stall. Join the callback before reusing a successful session.
	interrupted := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		defer close(interrupted)
		if deadlinePort, ok := port.(interface{ SetDeadline(time.Time) error }); ok {
			_ = deadlinePort.SetDeadline(time.Now())
		} else {
			_ = port.Close()
		}
	})
	defer func() {
		if !stop() {
			<-interrupted
		}
	}()
	fail := func(err error) (Result, error) {
		d.retire(port)
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
