package main

import (
	"bufio"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/tarm/serial"
)

const (
	serialRate      = 20 * time.Millisecond
	commandTimeout  = 250 * time.Millisecond
	feedbackTimeout = 500 * time.Millisecond
)

type controllerStatus struct {
	powerStatus     string
	serialConnected bool
	lastFeedback    string
}

type controller struct {
	power           powerControl
	mu              sync.Mutex
	linear          float64
	angular         float64
	lastCommand     time.Time
	serialConnected bool
	lastFeedback    string
	lastReportAt    time.Time
	feedbackHealthy bool
}

func newController() *controller {
	return &controller{lastCommand: time.Now()}
}

func (c *controller) set(linear, angular float64) {
	c.mu.Lock()
	c.linear = clamp(linear, -linearLimit, linearLimit)
	c.angular = clamp(angular, -angularLimit, angularLimit)
	c.lastCommand = time.Now()
	c.mu.Unlock()
}

func (c *controller) stop() {
	c.set(0, 0)
}

func (c *controller) command() (float64, float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if time.Since(c.lastCommand) > commandTimeout {
		return 0, 0
	}
	return c.linear, c.angular
}

func (c *controller) status() controllerStatus {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.refreshPowerLocked(time.Now())
	healthy := c.serialHealthyLocked(time.Now())
	if healthy != c.feedbackHealthy {
		c.feedbackHealthy = healthy
		if healthy {
			slog.Info("motor ESP32 feedback received")
		} else {
			slog.Warn("motor ESP32 feedback stale or unavailable")
		}
	}
	powerStatus := c.power.status
	if powerStatus == "" {
		powerStatus = "unknown"
	}
	return controllerStatus{serialConnected: healthy, lastFeedback: c.lastFeedback, powerStatus: powerStatus}
}

func (c *controller) serialHealthyLocked(now time.Time) bool {
	return c.serialConnected && !c.lastReportAt.IsZero() && now.Sub(c.lastReportAt) <= feedbackTimeout
}

func (c *controller) serialLoop(portName string, baud int) {
	for {
		port, err := serial.OpenPort(&serial.Config{Name: portName, Baud: baud, ReadTimeout: 20 * time.Millisecond})
		if err != nil {
			c.setSerial(false)
			slog.Warn("serial open failed", "port", portName, "error", err)
			time.Sleep(time.Second)
			continue
		}

		slog.Info("serial connected", "port", portName, "baud", baud)
		c.setSerial(true)
		done := make(chan struct{})
		readerStop := make(chan struct{})
		go c.readSerial(port, done, readerStop)

		ticker := time.NewTicker(serialRate)
	serialWriteLoop:
		for {
			select {
			case <-done:
				slog.Warn("serial feedback reader stopped; reopening port", "port", portName)
				break serialWriteLoop
			case <-ticker.C:
				if err := c.writeSerialTick(port); err != nil {
					slog.Warn("serial write failed", "error", err)
					break serialWriteLoop
				}
			}
		}
		ticker.Stop()
		closed := c.retireSerial(port, done, readerStop)
		select {
		case <-closed:
		case <-time.After(time.Second):
			slog.Warn("serial device offline; USB driver cleanup is still pending", "port", portName)
			<-closed
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func (c *controller) setSerial(connected bool) {
	c.mu.Lock()
	if connected != c.serialConnected {
		c.lastReportAt = time.Time{}
		c.lastFeedback = ""
		c.power.pendingAt = time.Time{}
		c.power.deadline = time.Time{}
		if connected {
			c.power.status = "unknown"
		} else {
			c.power.status = "offline"
		}
	}
	c.serialConnected = connected
	c.mu.Unlock()
}

func (c *controller) readSerial(port *serial.Port, done chan<- struct{}, stop <-chan struct{}) {
	defer close(done)
	scanner := bufio.NewScanner(serialFeedbackReader{Reader: port, stop: stop})
	scanner.Buffer(make([]byte, 1024), 256*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		c.recordFeedback(line)
	}
	if err := scanner.Err(); err != nil {
		select {
		case <-stop:
		default:
			slog.Warn("serial feedback read failed", "error", err)
		}
	}
}

func clamp(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
