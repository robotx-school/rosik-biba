package main

import "bibika-server/internal/protocol"

type agentMessage = protocol.Message

type robotStatus struct {
	Online          bool   `json:"online"`
	SerialConnected bool   `json:"serialConnected"`
	LastFeedback    string `json:"lastFeedback"`
	PowerStatus     string `json:"powerStatus"`
}
