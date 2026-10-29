package protocol

type Message struct {
	Type    string `json:"type"`
	RobotID string `json:"robotId,omitempty"`
	Token   string `json:"token,omitempty"`

	Linear  float64 `json:"linear,omitempty"`
	Angular float64 `json:"angular,omitempty"`

	SerialConnected bool   `json:"serialConnected,omitempty"`
	LastFeedback    string `json:"lastFeedback,omitempty"`
	PowerStatus     string `json:"powerStatus,omitempty"`

	SDP     string `json:"sdp,omitempty"`
	SDPType string `json:"sdpType,omitempty"`
}
