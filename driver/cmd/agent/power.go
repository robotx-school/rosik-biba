package main

import (
	"fmt"
	"io"
	"strings"
	"time"
)

const (
	powerRequestTTL      = 250 * time.Millisecond
	powerCooldown        = 2 * time.Second
	powerFeedbackTimeout = 2 * time.Second
)

type powerControl struct {
	status      string
	pendingAt   time.Time
	requestedAt time.Time
	deadline    time.Time
}

func (c *controller) refreshPowerLocked(now time.Time) {
	if !c.power.pendingAt.IsZero() && now.Sub(c.power.pendingAt) > powerRequestTTL {
		c.power.pendingAt = time.Time{}
		c.power.status = "expired"
	}
	if (c.power.status == "sent" || c.power.status == "pulsing" || c.power.status == "busy") && !c.power.deadline.IsZero() && now.After(c.power.deadline) {
		c.power.status = "timeout"
	}
}
func (c *controller) requestPowerPulse() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	c.refreshPowerLocked(now)
	if !c.serialHealthyLocked(now) {
		return "offline"
	}
	if !c.power.pendingAt.IsZero() || c.power.status == "sent" || c.power.status == "pulsing" || now.Sub(c.power.requestedAt) < powerCooldown {
		return "busy"
	}
	c.linear = 0
	c.angular = 0
	c.lastCommand = now
	c.power.status = "pending"
	c.power.pendingAt = now
	c.power.requestedAt = now
	return "pending"
}
func (c *controller) cancelPendingPower() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.power.pendingAt.IsZero() {
		c.power.pendingAt = time.Time{}
		c.power.status = "expired"
	}
}

// Called only by the serial owner. Consume before writing: a partial/failed
// pulse write must never be retried, even if its physical outcome is unknown.
func (c *controller) writeSerialTick(w io.Writer) error {
	c.mu.Lock()
	c.refreshPowerLocked(time.Now())
	pulse := !c.power.pendingAt.IsZero()
	if pulse && !c.serialHealthyLocked(time.Now()) {
		c.power.pendingAt = time.Time{}
		c.power.status = "offline"
		pulse = false
	}
	if pulse {
		c.power.pendingAt = time.Time{}
		c.power.status = "sent"
		c.power.deadline = time.Now().Add(powerFeedbackTimeout)
	}
	c.mu.Unlock()
	if pulse {
		const command = "POWER_PULSE\n"
		n, err := io.WriteString(w, command)
		if err == nil && n != len(command) {
			err = io.ErrShortWrite
		}
		if err != nil {
			c.mu.Lock()
			c.power.status = "uncertain"
			c.mu.Unlock()
			return err
		}
	}
	linear, angular := c.command()
	if pulse {
		linear = 0
		angular = 0
	}
	_, err := fmt.Fprintf(w, "%.4f %.4f\n", linear, angular)
	return err
}
func (c *controller) recordFeedback(line string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// Retirement uses this same lock; ignore a line read just before close.
	if !c.serialConnected {
		return
	}
	c.lastFeedback = line
	if strings.HasPrefix(line, "L1:") {
		c.lastReportAt = time.Now()
	}
	switch line {
	case "RELAY READY":
		c.power.status = "ready"
	case "RELAY PULSE_STARTED":
		c.power.status = "pulsing"
		c.power.deadline = time.Now().Add(powerFeedbackTimeout)
	case "RELAY PULSE_DONE":
		c.power.status = "done"
	case "RELAY BUSY":
		c.power.status = "busy"
		c.power.deadline = time.Now().Add(powerFeedbackTimeout)
	}
}
