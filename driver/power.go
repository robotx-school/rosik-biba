package main

// Relay pulse acknowledgements share the existing driver control connection.
// They describe the pulse only; actual motor power is not sensed.
func (h *hub) broadcastPowerResult(robotID, state string) {
	h.mu.Lock()
	sessions := make([]*controlSession, 0, len(h.controllers))
	for session := range h.controllers {
		if session.robotID == robotID {
			sessions = append(sessions, session)
		}
	}
	h.mu.Unlock()
	for _, session := range sessions {
		if err := session.send(agentMessage{Type: "power_result", PowerStatus: state}); err != nil {
			_ = session.conn.Close()
		}
	}
}
