package control

import (
	"bytes"
	"io"
	"testing"
)

func TestPythonWireCompatibility(t *testing.T) {
	p, err := (Command{Action: "poses", IDs: []int{1, 12}, Positions: []int{5000, 10000}, Speed: 400, Accel: 50}).Packet()
	if err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	if err := WritePacket(&b, p); err != nil {
		t.Fatal(err)
	}
	want := []byte{'?', '$', 0x10, 10, 0, 0x90, 1, 50, 0, 1, 0x88, 0x13, 12, 0x10, 0x27}
	if !bytes.Equal(b.Bytes(), want) {
		t.Fatalf("got %x want %x", b.Bytes(), want)
	}
}
func TestFramingResyncAndEmptyCommand(t *testing.T) {
	r := bytes.NewReader([]byte{'b', 'o', 'o', 't', '?', '?', '$', 0x31, 0, 0})
	p, err := ReadPacket(r)
	if err != nil || p.Type != 0x31 || len(p.Data) != 0 {
		t.Fatalf("%+v %v", p, err)
	}
}
func TestMalformedAndTruncatedPackets(t *testing.T) {
	for _, raw := range [][]byte{{'?', '$', 1, 1, 1}, {'?', '$', 1, 2, 0, 1}} {
		if _, err := ReadPacket(bytes.NewReader(raw)); err == nil {
			t.Fatal("accepted malformed packet")
		}
	}
	var b bytes.Buffer
	if err := WritePacket(&b, Packet{Data: make([]byte, MaxPayload+1)}); err == nil {
		t.Fatal("accepted oversized write")
	}
	if err := WritePacket(shortWriter{}, Packet{Type: 0x31}); err != io.ErrShortWrite {
		t.Fatalf("short write: %v", err)
	}
}

type shortWriter struct{}

func (shortWriter) Write(p []byte) (int, error) { return 0, nil }
func TestCommandRejectsUnsafeInput(t *testing.T) {
	cases := []Command{
		{Action: "poses", IDs: []int{1}, Positions: []int{65535}},
		{Action: "poses", IDs: []int{1, 1}, Positions: []int{5, 5}},
		{Action: "poses", IDs: []int{254}, Positions: []int{5}},
		{Action: "poses", IDs: []int{1}, Positions: []int{5}, Accel: 256},
		{Action: "poses", IDs: []int{1}, Positions: []int{5}, Speed: 4096},
		{Action: "lift", MM: intPtr(-1)}, {Action: "lift"},
		{Action: "read", IDs: []int{}}, {Action: "unknown"},
	}
	for _, c := range cases {
		if _, err := c.Packet(); err == nil {
			t.Fatalf("accepted %+v", c)
		}
	}
}
func intPtr(i int) *int { return &i }
