package control

import (
	"context"
	"io"
	"net"
	"testing"
	"time"
)

func TestFirmwareRejectionIsReturnedWithoutReplay(t *testing.T) {
	client, robot := net.Pipe()
	defer robot.Close()
	d := newTestDevice(client)
	done := make(chan struct{})
	go func() {
		defer close(done)
		p, err := ReadPacket(robot)
		if err != nil {
			return
		}
		_ = WritePacket(robot, Packet{Type: Ack, Data: []byte{p.Type, 3}})
	}()
	if _, err := d.Execute(context.Background(), Command{Action: "calibrate"}); err == nil {
		t.Fatal("firmware rejection ignored")
	}
	<-done
}
func TestStalledReplyTimesOut(t *testing.T) {
	client, robot := net.Pipe()
	defer robot.Close()
	d := newTestDevice(client)
	go func() { _, _ = ReadPacket(robot) }()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, err := d.Execute(ctx, Command{Action: "enable"}); err == nil {
		t.Fatal("expected timeout")
	}
	if time.Since(start) > time.Second {
		t.Fatal("transaction hung")
	}
	if d.port != nil {
		t.Fatal("uncertain serial session retained")
	}
}
func TestInvalidFeedbackCannotBecomeAPose(t *testing.T) {
	p := Packet{Type: GetPoses, Data: []byte{1}}
	if _, err := decodeResult(p, Packet{Type: SendPoses, Data: []byte{255, 255}}); err == nil {
		t.Fatal("failed servo read was accepted")
	}
}
func newTestDevice(port io.ReadWriteCloser) *Device {
	d := NewDevice("unused", 115200)
	d.port = port
	return d
}

// USB drivers can block even Close while draining an unresponsive device.
// Requests must return on time and quarantine that device until cleanup ends.
func TestDeadlineReturnsWhileDriverCloseIsBlocked(t *testing.T) {
	client, robot := net.Pipe()
	defer robot.Close()
	release := make(chan struct{})
	port := &blockedClosePort{Conn: client, release: release}
	d := newTestDevice(port)
	go func() { _, _ = ReadPacket(robot) }()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := d.Execute(ctx, Command{Action: "enable"}); done <- err }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("missing deadline error")
		}
	case <-time.After(200 * time.Millisecond):
		close(release)
		<-done
		t.Fatal("request blocked on USB driver close")
	}
	_, err := d.Execute(context.Background(), Command{Action: "status"})
	if err == nil {
		t.Fatal("reopened a serial port while prior close is pending")
	}
	close(release)
}

type blockedClosePort struct {
	net.Conn
	release <-chan struct{}
}

func (p *blockedClosePort) Close() error { <-p.release; return p.Conn.Close() }
