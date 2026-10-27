package control

import (
	"encoding/binary"
	"errors"
	"io"
)

const MaxPayload = 256
const (
	SetPoses      byte = 0x10
	GetPoses      byte = 0x11
	SendPoses     byte = 0x12
	GetButtons    byte = 0x20
	SendButtons   byte = 0x21
	SetLift       byte = 0x30
	EnableLift    byte = 0x31
	DisableLift   byte = 0x32
	CalibrateLift byte = 0x33
	GetLift       byte = 0x34
	SendLift      byte = 0x35
	StopLift      byte = 0x36
	Heartbeat     byte = 0x37
	Ack           byte = 0x7e
)

type Packet struct {
	Type byte
	Data []byte
}

// ReadPacket skips startup text and finds the next bounded binary frame.
// The caller must supply a reader with a deadline for physical serial devices.
func ReadPacket(r io.Reader) (Packet, error) {
	var b [1]byte
	matched := false
	for {
		if _, err := io.ReadFull(r, b[:]); err != nil {
			return Packet{}, err
		}
		if matched && b[0] == '$' {
			break
		}
		matched = b[0] == '?'
	}
	var h [3]byte
	if _, err := io.ReadFull(r, h[:]); err != nil {
		return Packet{}, err
	}
	n := int(binary.LittleEndian.Uint16(h[1:]))
	if n > MaxPayload {
		return Packet{}, errors.New("packet payload exceeds 256 bytes")
	}
	p := Packet{Type: h[0], Data: make([]byte, n)}
	_, err := io.ReadFull(r, p.Data)
	return p, err
}

func WritePacket(w io.Writer, p Packet) error {
	if len(p.Data) > MaxPayload {
		return errors.New("packet payload exceeds 256 bytes")
	}
	b := make([]byte, 5+len(p.Data))
	copy(b, "?$")
	b[2] = p.Type
	binary.LittleEndian.PutUint16(b[3:5], uint16(len(p.Data)))
	copy(b[5:], p.Data)
	n, err := w.Write(b)
	if err == nil && n != len(b) {
		err = io.ErrShortWrite
	}
	return err
}
