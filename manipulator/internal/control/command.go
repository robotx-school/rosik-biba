package control

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// Positions are calibrated normalized values (0..10000), not encoder ticks.
type Command struct {
	Action    string `json:"action"`
	IDs       []int  `json:"ids,omitempty"`
	Positions []int  `json:"positions,omitempty"`
	Speed     int    `json:"speed,omitempty"`
	Accel     int    `json:"accel,omitempty"`
	MM        *int   `json:"mm,omitempty"`
}

type Result struct {
	Status    string      `json:"status"`
	Positions []int       `json:"positions,omitempty"`
	Buttons   []bool      `json:"buttons,omitempty"`
	Lift      *LiftStatus `json:"lift,omitempty"`
}

type LiftStatus struct {
	Steps       int32 `json:"steps"` // signed steps from the upper zero; negative is down
	Calibrated  bool  `json:"calibrated"`
	Calibrating bool  `json:"calibrating"`
	Enabled     bool  `json:"enabled"`
	Moving      bool  `json:"moving"`
	Top         bool  `json:"top"`
	Bottom      bool  `json:"bottom"`
	Fault       int   `json:"fault"`
}

func validateIDs(ids []int) error {
	if len(ids) < 1 || len(ids) > 12 {
		return errors.New("require 1..12 servo IDs")
	}
	seen := map[int]bool{}
	for _, id := range ids {
		if id < 1 || id > 253 || seen[id] {
			return errors.New("servo IDs must be unique and within 1..253")
		}
		seen[id] = true
	}
	return nil
}

func (c Command) Packet() (Packet, error) {
	p := Packet{}
	// Reject unrelated fields rather than silently accepting misspelled intent.
	if c.Action != "poses" && (len(c.Positions) > 0 || c.Speed != 0 || c.Accel != 0) {
		return p, errors.New("pose fields require action poses")
	}
	if c.Action != "poses" && c.Action != "read" && len(c.IDs) > 0 {
		return p, errors.New("IDs require poses or read")
	}
	if c.Action != "lift" && c.MM != nil {
		return p, errors.New("mm requires action lift")
	}
	switch c.Action {
	case "poses":
		if err := validateIDs(c.IDs); err != nil {
			return p, err
		}
		if len(c.IDs) != len(c.Positions) {
			return p, errors.New("IDs and positions must have the same length")
		}
		if c.Speed < 0 || c.Speed > 4095 || c.Accel < 0 || c.Accel > 255 {
			return p, errors.New("speed must be 0..4095 and accel 0..255")
		}
		p.Type = SetPoses
		p.Data = make([]byte, 4+3*len(c.IDs))
		binary.LittleEndian.PutUint16(p.Data, uint16(c.Speed))
		binary.LittleEndian.PutUint16(p.Data[2:], uint16(c.Accel))
		for i, id := range c.IDs {
			pos := c.Positions[i]
			if pos < 0 || pos > 10000 {
				return Packet{}, errors.New("positions must be 0..10000")
			}
			p.Data[4+3*i] = byte(id)
			binary.LittleEndian.PutUint16(p.Data[5+3*i:], uint16(pos))
		}
	case "read":
		if err := validateIDs(c.IDs); err != nil {
			return p, err
		}
		p.Type = GetPoses
		for _, id := range c.IDs {
			p.Data = append(p.Data, byte(id))
		}
	case "buttons":
		p = Packet{Type: GetButtons, Data: []byte{'d'}} // upstream reader expects a nonempty request
	case "lift":
		if c.MM == nil || *c.MM < 0 || *c.MM > 60000 {
			return p, errors.New("mm is required and must be 0..60000; firmware also enforces its configured travel limit")
		}
		p.Type = SetLift
		p.Data = make([]byte, 2)
		binary.LittleEndian.PutUint16(p.Data, uint16(*c.MM))
	case "enable":
		p.Type = EnableLift
	case "disable":
		p.Type = DisableLift
	case "calibrate":
		p.Type = CalibrateLift
	case "status":
		p.Type = GetLift
	case "stop":
		p.Type = StopLift
	case "heartbeat":
		p.Type = Heartbeat
	default:
		return p, fmt.Errorf("unknown action %q", c.Action)
	}
	return p, nil
}

// FirmwareError is a valid reply rejecting a command, not a broken serial link.
type FirmwareError struct{ Command, Code byte }

func (e *FirmwareError) Error() string {
	return fmt.Sprintf("firmware rejected command 0x%02x: error %d", e.Command, e.Code)
}

func decodeResult(request, reply Packet) (Result, error) {
	result := Result{Status: "accepted"}
	if reply.Type == Ack {
		if len(reply.Data) != 2 || reply.Data[0] != request.Type {
			return Result{}, errors.New("unexpected acknowledgement")
		}
		if reply.Data[1] != 0 {
			return Result{}, &FirmwareError{Command: request.Type, Code: reply.Data[1]}
		}
		if request.Type == GetPoses || request.Type == GetButtons || request.Type == GetLift {
			return Result{}, errors.New("missing requested feedback")
		}
		return result, nil
	}
	switch {
	case request.Type == GetPoses && reply.Type == SendPoses:
		if len(reply.Data) != 2*len(request.Data) {
			return Result{}, errors.New("incorrect pose feedback length")
		}
		for i := 0; i < len(reply.Data); i += 2 {
			pos := int(binary.LittleEndian.Uint16(reply.Data[i:]))
			if pos > 10000 {
				return Result{}, errors.New("servo read failed or calibration is invalid")
			}
			result.Positions = append(result.Positions, pos)
		}
	case request.Type == GetButtons && reply.Type == SendButtons:
		if len(reply.Data) != 6 {
			return Result{}, errors.New("incorrect button feedback length")
		}
		for _, b := range reply.Data {
			if b > 1 {
				return Result{}, errors.New("invalid button state")
			}
			result.Buttons = append(result.Buttons, b != 0)
		}
	case request.Type == GetLift && reply.Type == SendLift:
		if len(reply.Data) != 6 {
			return Result{}, errors.New("incorrect lift feedback length")
		}
		f := reply.Data[4]
		result.Lift = &LiftStatus{Steps: int32(binary.LittleEndian.Uint32(reply.Data)), Calibrated: f&1 != 0, Calibrating: f&2 != 0, Enabled: f&4 != 0, Moving: f&8 != 0, Top: f&16 != 0, Bottom: f&32 != 0, Fault: int(reply.Data[5])}
	default:
		return Result{}, fmt.Errorf("unexpected response 0x%02x for 0x%02x", reply.Type, request.Type)
	}
	result.Status = "feedback"
	return result, nil
}
