package main

import (
	"bufio"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/tarm/serial"
)

const (
	serialRate     = 20 * time.Millisecond
	commandTimeout = 250 * time.Millisecond
)

type controllerStatus struct {
	serialConnected bool
	lastFeedback    string
}

type controller struct {
	mu              sync.Mutex
	linear          float64
	angular         float64
	lastCommand     time.Time
	serialConnected bool
	lastFeedback    string
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
	return controllerStatus{serialConnected: c.serialConnected, lastFeedback: c.lastFeedback}
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
		go c.readSerial(port, done)

		ticker := time.NewTicker(serialRate)
		for range ticker.C {
			linear, angular := c.command()
			if _, err := fmt.Fprintf(port, "%.4f %.4f\n", linear, angular); err != nil {
				slog.Warn("serial write failed", "error", err)
				break
			}
		}
		ticker.Stop()
		_ = port.Close()
		<-done
		c.setSerial(false)
		time.Sleep(500 * time.Millisecond)
	}
}

func (c *controller) setSerial(connected bool) {
	c.mu.Lock()
	c.serialConnected = connected
	c.mu.Unlock()
}

func (c *controller) readSerial(port *serial.Port, done chan<- struct{}) {
	defer close(done)
	scanner := bufio.NewScanner(port)
	scanner.Buffer(make([]byte, 1024), 256*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		c.mu.Lock()
		c.lastFeedback = line
		c.mu.Unlock()
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
