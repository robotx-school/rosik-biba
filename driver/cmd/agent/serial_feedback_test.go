package main

import (
	"bufio"
	"io"
	"os"
	"testing"
	"time"
)

type idleThenFeedback struct{ calls int }

func (r *idleThenFeedback) Read(p []byte) (int, error) {
	r.calls++
	switch r.calls {
	case 1:
		return 0, io.EOF
	case 2:
		return copy(p, "RELAY PULSE_DONE\n"), nil
	default:
		return 0, os.ErrClosed
	}
}
func TestSerialFeedbackContinuesAfterIdleTimeout(t *testing.T) {
	scanner := bufio.NewScanner(serialFeedbackReader{Reader: &idleThenFeedback{}})
	if !scanner.Scan() || scanner.Text() != "RELAY PULSE_DONE" {
		t.Fatal("idle timeout discarded subsequent relay feedback")
	}
	if scanner.Scan() {
		t.Fatal("unexpected extra feedback")
	}
}

type slowSerialClose struct{ release <-chan struct{} }

func (p slowSerialClose) Close() error { <-p.release; return nil }
func TestRetireSerialImmediatelyMarksOfflineAndDropsPendingPower(t *testing.T) {
	c := newController()
	c.setSerial(true)
	c.requestPowerPulse()
	release := make(chan struct{})
	readerDone := make(chan struct{})
	readerStop := make(chan struct{})
	closed := c.retireSerial(slowSerialClose{release: release}, readerDone, readerStop)
	if c.status().serialConnected || c.requestPowerPulse() != "offline" {
		t.Fatal("blocked close left relay available")
	}
	select {
	case <-closed:
		t.Fatal("close completed before driver cleanup")
	default:
	}
	close(release)
	close(readerDone)
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("cleanup did not finish")
	}
}

type eofSerialReader struct{}

func (eofSerialReader) Read([]byte) (int, error) { return 0, io.EOF }
func TestSerialFeedbackExitsWhenRetiredPortKeepsReturningEOF(t *testing.T) {
	stop := make(chan struct{})
	done := make(chan struct{})
	scanner := bufio.NewScanner(serialFeedbackReader{Reader: eofSerialReader{}, stop: stop})
	go func() { scanner.Scan(); close(done) }()
	close(stop)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("retired EOF reader did not exit")
	}
}
