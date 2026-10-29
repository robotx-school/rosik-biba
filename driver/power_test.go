package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"bibika-server/internal/ice"
	"github.com/pion/webrtc/v4"
	"golang.org/x/net/websocket"
)

func TestPowerPulseTraversesControlAndAgentWebSockets(t *testing.T) {
	h := newHub("secret", "robot", webrtc.NewAPI(), ice.Config{})
	mux := http.NewServeMux()
	mux.Handle("/agent", websocket.Handler(h.handleAgentWS))
	mux.Handle("/control", websocket.Handler(h.handleControlWS))
	server := httptest.NewServer(mux)
	defer server.Close()
	dial := func(path string) *websocket.Conn {
		t.Helper()
		ws, err := websocket.Dial("ws"+strings.TrimPrefix(server.URL, "http")+path, "", server.URL)
		if err != nil {
			t.Fatal(err)
		}
		ws.SetDeadline(time.Now().Add(2 * time.Second))
		return ws
	}
	agent := dial("/agent")
	defer agent.Close()
	if err := websocket.JSON.Send(agent, agentMessage{Type: "hello", Token: "secret", RobotID: "robot"}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for {
		h.mu.Lock()
		registered := h.agents["robot"] != nil
		h.mu.Unlock()
		if registered {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("agent not registered")
		}
		time.Sleep(time.Millisecond)
	}
	driver := dial("/control")
	defer driver.Close()
	var reply agentMessage
	if err := websocket.JSON.Receive(driver, &reply); err != nil {
		t.Fatal(err)
	}
	if err := websocket.JSON.Send(driver, agentMessage{Type: "power_pulse", Linear: 1.0}); err != nil {
		t.Fatal(err)
	}
	if err := websocket.JSON.Receive(agent, &reply); err != nil {
		t.Fatal(err)
	}
	if reply.Type != "power_pulse" || reply.Linear != 0 || reply.Angular != 0 {
		t.Fatalf("forwarded %+v", reply)
	}
	if err := websocket.JSON.Send(agent, agentMessage{Type: "power_result", PowerStatus: "pending"}); err != nil {
		t.Fatal(err)
	}
	if err := websocket.JSON.Receive(driver, &reply); err != nil {
		t.Fatal(err)
	}
	if reply.Type != "power_result" || reply.PowerStatus != "pending" {
		t.Fatalf("result %+v", reply)
	}
}
