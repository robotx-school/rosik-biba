package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"rosik-manipulator/internal/control"
)

type sender struct {
	url, token string
	client     *http.Client
	mu         sync.Mutex
}

func (s *sender) send(ctx context.Context, c control.Command) (control.Result, error) {
	if _, err := c.Packet(); err != nil {
		return control.Result{}, err
	}
	b, err := json.Marshal(c)
	if err != nil {
		return control.Result{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.url+"/command", bytes.NewReader(b))
	if err != nil {
		return control.Result{}, err
	}
	req.Header.Set("Authorization", "Bearer "+s.token)
	req.Header.Set("Content-Type", "application/json")
	res, err := s.client.Do(req)
	if err != nil {
		return control.Result{}, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 8193))
	if err != nil {
		return control.Result{}, err
	}
	if len(body) > 8192 {
		return control.Result{}, errors.New("oversized receiver response")
	}
	if res.StatusCode != 200 {
		return control.Result{}, fmt.Errorf("receiver %s: %s", res.Status, body)
	}
	var result control.Result
	if err := json.Unmarshal(body, &result); err != nil {
		return result, err
	}
	return result, nil
}
func main() {
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}
func run() error {
	endpoint := flag.String("url", "", "receiver base URL (required)")
	port := flag.String("serial", "", "optional operator master reader ESP32 serial path; otherwise read JSON lines from stdin")
	baud := flag.Int("baud", 115200, "master USB serial baud")
	idsFlag := flag.String("ids", "1,2,3,4,5,6,7,8,9,10,11,12", "master servo IDs")
	speed := flag.Int("speed", 400, "servo speed for master forwarding (0..4095)")
	accel := flag.Int("accel", 50, "servo acceleration for master forwarding (0..255)")
	interval := flag.Duration("interval", 50*time.Millisecond, "master polling period (minimum 20ms)")
	hold := flag.Duration("hold", 0, "keep sending lift heartbeats this long after stdin EOF; 0 exits immediately")
	flag.Parse()
	token := os.Getenv("MANIPULATOR_TOKEN")
	u, err := url.Parse(*endpoint)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.RawQuery != "" || u.Fragment != "" || u.User != nil || token == "" {
		return errors.New("valid -url and MANIPULATOR_TOKEN are required")
	}
	if *interval < 20*time.Millisecond || *baud <= 0 || *hold < 0 {
		return errors.New("invalid interval, baud or hold duration")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	s := &sender{url: strings.TrimRight(*endpoint, "/"), token: token, client: &http.Client{Timeout: 4 * time.Second}}
	// Initialize the robot serial connection before starting the watchdog lease.
	if _, err := s.send(ctx, control.Command{Action: "heartbeat"}); err != nil {
		return err
	}
	failures := make(chan error, 1)
	go func() {
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := s.send(ctx, control.Command{Action: "heartbeat"}); err != nil {
					select {
					case failures <- err:
					default:
					}
					cancel()
					return
				}
			}
		}
	}()
	done := make(chan error, 1)
	go func() {
		if *port != "" {
			done <- forwardMaster(ctx, s, *port, *baud, *idsFlag, *speed, *accel, *interval)
			return
		}
		done <- forwardJSON(ctx, s)
	}()
	select {
	case err := <-done:
		if err != nil {
			return err
		}
		if *hold > 0 {
			timer := time.NewTimer(*hold)
			defer timer.Stop()
			select {
			case <-timer.C:
			case <-ctx.Done():
			}
		}
	case err := <-failures:
		return err
	case <-ctx.Done():
		select {
		case err := <-failures:
			return err
		default:
		}
	}
	// Lift stops; servo targets retain their normal holding behavior.
	cancel()
	stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
	defer stopCancel()
	_, _ = s.send(stopCtx, control.Command{Action: "stop"})
	return nil
}
func forwardJSON(ctx context.Context, s *sender) error {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 4096), 4096)
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) == "" {
			continue
		}
		decoder := json.NewDecoder(bytes.NewReader(scanner.Bytes()))
		decoder.DisallowUnknownFields()
		var c control.Command
		if err := decoder.Decode(&c); err != nil {
			return err
		}
		var extra any
		if err := decoder.Decode(&extra); err != io.EOF {
			return errors.New("expected one command per line")
		}
		result, err := s.send(ctx, c)
		if err != nil {
			return err
		}
		if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
			return err
		}
	}
	return scanner.Err()
}
func forwardMaster(ctx context.Context, s *sender, port string, baud int, idsText string, speed, accel int, interval time.Duration) error {
	var ids []int
	for _, part := range strings.Split(idsText, ",") {
		id, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil {
			return err
		}
		ids = append(ids, id)
	}
	probe := control.Command{Action: "poses", IDs: ids, Positions: make([]int, len(ids)), Speed: speed, Accel: accel}
	if _, err := probe.Packet(); err != nil {
		return err
	}
	master := control.NewDevice(port, baud)
	defer master.Close()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
		result, err := master.Execute(ctx, control.Command{Action: "read", IDs: ids})
		if err != nil {
			return fmt.Errorf("master feedback: %w", err)
		}
		probe.Positions = result.Positions
		if _, err := s.send(ctx, probe); err != nil {
			return err
		}
	}
}
