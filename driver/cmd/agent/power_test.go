package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestPowerPulseIsWrittenOnceAndNeverReplayed(t *testing.T) {
	c := newController()
	c.setSerial(true)
	if got := c.requestPowerPulse(); got != "pending" {
		t.Fatalf("request=%q", got)
	}
	if got := c.requestPowerPulse(); got != "busy" {
		t.Fatalf("duplicate=%q", got)
	}
	var output bytes.Buffer
	if err := c.writeSerialTick(&output); err != nil {
		t.Fatal(err)
	}
	if err := c.writeSerialTick(&output); err != nil {
		t.Fatal(err)
	}
	if strings.Count(output.String(), "POWER_PULSE\n") != 1 {
		t.Fatalf("serial writes %q", output.String())
	}
	c.setSerial(false)
	c.setSerial(true)
	output.Reset()
	if err := c.writeSerialTick(&output); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), "POWER_PULSE") {
		t.Fatal("pulse replayed on reconnect")
	}
}
func TestPowerPulseOfflineAndExpiredRequestsAreNotSent(t *testing.T) {
	c := newController()
	if c.requestPowerPulse() != "offline" {
		t.Fatal("offline pulse accepted")
	}
	c.setSerial(true)
	c.requestPowerPulse()
	c.mu.Lock()
	c.power.pendingAt = time.Now().Add(-powerRequestTTL - time.Millisecond)
	c.mu.Unlock()
	var output bytes.Buffer
	if err := c.writeSerialTick(&output); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), "POWER_PULSE") {
		t.Fatal("expired pulse sent")
	}
	if c.status().powerStatus != "expired" {
		t.Fatal("expiration not reported")
	}
}
func TestPowerPulseWriteFailureIsNotRetried(t *testing.T) {
	c := newController()
	c.setSerial(true)
	c.requestPowerPulse()
	if err := c.writeSerialTick(failedRelayWriter{}); err == nil {
		t.Fatal("expected write failure")
	}
	var output bytes.Buffer
	if err := c.writeSerialTick(&output); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), "POWER_PULSE") {
		t.Fatal("uncertain pulse retried")
	}
	if c.status().powerStatus != "uncertain" {
		t.Fatal("uncertain outcome not reported")
	}
}

type failedRelayWriter struct{}

func (failedRelayWriter) Write([]byte) (int, error) { return 0, errors.New("disconnected") }
func TestPowerPulseTracksFirmwareFeedbackAndTimeout(t *testing.T) {
	c := newController()
	c.setSerial(true)
	c.requestPowerPulse()
	var output bytes.Buffer
	c.writeSerialTick(&output)
	c.recordFeedback("RELAY PULSE_STARTED")
	if c.status().powerStatus != "pulsing" {
		t.Fatal("missing active feedback")
	}
	c.recordFeedback("RELAY PULSE_DONE")
	if c.status().powerStatus != "done" {
		t.Fatal("missing completion feedback")
	}
	c.mu.Lock()
	c.power.status = "sent"
	c.power.deadline = time.Now().Add(-time.Second)
	c.mu.Unlock()
	if c.status().powerStatus != "timeout" {
		t.Fatal("missing confirmation timeout")
	}
}

func TestPendingPowerPulseIsCancelledOnDisconnect(t *testing.T) {
	for _, disconnect := range []func(*controller){func(c *controller) { c.cancelPendingPower() }, func(c *controller) { c.setSerial(false); c.setSerial(true) }} {
		c := newController()
		c.setSerial(true)
		c.requestPowerPulse()
		disconnect(c)
		var output bytes.Buffer
		if err := c.writeSerialTick(&output); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(output.String(), "POWER_PULSE") {
			t.Fatal("pending pulse survived disconnect")
		}
	}
}

func TestRetiredSerialFeedbackCannotOverwriteOfflineStatus(t *testing.T) {
	c := newController()
	c.setSerial(true)
	c.setSerial(false)
	c.recordFeedback("RELAY PULSE_DONE")
	if got := c.status().powerStatus; got != "offline" {
		t.Fatalf("late feedback changed offline state to %q", got)
	}
}
