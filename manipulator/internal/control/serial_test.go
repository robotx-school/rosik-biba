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
