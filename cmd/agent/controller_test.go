package main

import (
	"testing"
	"time"
)

func TestControllerClampsCommand(t *testing.T) {
	controller := newController()
	controller.set(linearLimit+1, -angularLimit-1)
	linear, angular := controller.command()
	if linear != linearLimit || angular != -angularLimit {
		t.Fatalf("command = (%v, %v), want (%v, %v)", linear, angular, linearLimit, -angularLimit)
	}
}

func TestControllerStopsExpiredCommand(t *testing.T) {
	controller := newController()
	controller.set(0.5, 0.5)
	controller.mu.Lock()
	controller.lastCommand = time.Now().Add(-commandTimeout - time.Millisecond)
	controller.mu.Unlock()
	linear, angular := controller.command()
	if linear != 0 || angular != 0 {
		t.Fatalf("expired command = (%v, %v), want zero", linear, angular)
	}
}

func TestServerHeartbeatTimeout(t *testing.T) {
	heartbeat := newServerHeartbeat()
	if heartbeat.timedOut(time.Now()) {
		t.Fatal("fresh heartbeat timed out")
	}
	heartbeat.lastPing.Store(time.Now().Add(-serverPingTimeout - time.Millisecond).UnixNano())
	if !heartbeat.timedOut(time.Now()) {
		t.Fatal("expired heartbeat did not time out")
	}
}
