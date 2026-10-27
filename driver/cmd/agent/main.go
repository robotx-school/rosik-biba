package main

import (
	"errors"
	"log/slog"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"bibika-server/internal/ice"
	"bibika-server/internal/protocol"
	"golang.org/x/net/websocket"
)

const (
	defaultSerialPort = "/dev/ttyUSB1"
	defaultBaud       = 115200
	linearLimit       = 1.5
	angularLimit      = 2.0
	serverPingTimeout = 3 * time.Second
)

type config struct {
	serverURL  string
	robotID    string
	token      string
	serialPort string
	baud       int
	video      videoConfig
	iceConfig  ice.Config
}

type videoConfig struct {
	device string
	width  int
	height int
	fps    int
}

type agentClient struct {
	config config
	ctrl   *controller
	camera *cameraPublisher
}

type serverHeartbeat struct {
	lastPing atomic.Int64
}

func newServerHeartbeat() *serverHeartbeat {
	heartbeat := &serverHeartbeat{}
	heartbeat.mark()
	return heartbeat
}

func (h *serverHeartbeat) mark() {
	h.lastPing.Store(time.Now().UnixNano())
}

func (h *serverHeartbeat) timedOut(now time.Time) bool {
	lastPing := h.lastPing.Load()
	return lastPing == 0 || now.Sub(time.Unix(0, lastPing)) > serverPingTimeout
}

func main() {
	cfg, err := loadConfig()
	if err != nil {
		slog.Error("invalid agent configuration", "error", err)
		os.Exit(1)
	}
	ctrl := newController()
	go ctrl.serialLoop(cfg.serialPort, cfg.baud)
	client := &agentClient{config: cfg, ctrl: ctrl, camera: newCameraPublisher(cfg)}

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)
	go client.run()
	<-shutdown
	ctrl.stop()
	client.camera.close()
}

func (a *agentClient) run() {
	for {
		if err := a.connect(); err != nil {
			slog.Warn("server connection lost", "error", err)
		}
		time.Sleep(time.Second)
	}
}

func (a *agentClient) connect() error {
	endpoint, err := url.Parse(a.config.serverURL)
	if err != nil {
		return err
	}
	ws, err := websocket.Dial(a.config.serverURL, "", endpoint.Scheme+"://"+endpoint.Host)
	if err != nil {
		return err
	}
	defer ws.Close()
	peer := &lockedSocket{conn: ws}
	if err := peer.send(protocol.Message{Type: "hello", RobotID: a.config.robotID, Token: a.config.token}); err != nil {
		return err
	}
	if err := a.camera.start(peer); err != nil {
		return err
	}
	defer a.camera.close()
	heartbeat := newServerHeartbeat()
	watchdogDone := make(chan struct{})
	defer close(watchdogDone)
	go watchServerHeartbeat(ws, heartbeat, watchdogDone)

	statusDone := make(chan struct{})
	go func() {
		defer close(statusDone)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for range ticker.C {
			status := a.ctrl.status()
			if err := peer.send(protocol.Message{Type: "status", SerialConnected: status.serialConnected, LastFeedback: status.lastFeedback}); err != nil {
				return
			}
		}
	}()

	for {
		var message protocol.Message
		if err := websocket.JSON.Receive(ws, &message); err != nil {
			_ = ws.Close()
			<-statusDone
			return err
		}
		switch message.Type {
		case "ping":
			heartbeat.mark()
		case "control":
			a.ctrl.set(message.Linear, message.Angular)
		case "camera_publish_answer":
			if err := a.camera.setAnswer(message); err != nil {
				return err
			}
		}
	}
}

func watchServerHeartbeat(ws *websocket.Conn, heartbeat *serverHeartbeat, done <-chan struct{}) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case now := <-ticker.C:
			if heartbeat.timedOut(now) {
				slog.Warn("server heartbeat timed out; reconnecting")
				_ = ws.Close()
				return
			}
		}
	}
}

type lockedSocket struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func (s *lockedSocket) send(message protocol.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return websocket.JSON.Send(s.conn, message)
}

func loadConfig() (config, error) {
	serverURL := os.Getenv("BIBIKA_SERVER_URL")
	token := os.Getenv("BIBIKA_AGENT_TOKEN")
	if serverURL == "" || token == "" {
		return config{}, errors.New("BIBIKA_SERVER_URL and BIBIKA_AGENT_TOKEN are required")
	}
	cfg := config{
		serverURL:  serverURL,
		robotID:    env("BIBIKA_ROBOT_ID", "default"),
		token:      token,
		serialPort: env("BIBIKA_SERIAL", defaultSerialPort),
		baud:       envInt("BIBIKA_BAUD", defaultBaud),
		video: videoConfig{
			device: env("BIBIKA_VIDEO", "/dev/video0"),
			width:  envInt("BIBIKA_VIDEO_WIDTH", 1280),
			height: envInt("BIBIKA_VIDEO_HEIGHT", 720),
			fps:    envInt("BIBIKA_VIDEO_FPS", 30),
		},
		iceConfig: ice.Load(),
	}
	if cfg.baud <= 0 || cfg.video.width <= 0 || cfg.video.height <= 0 || cfg.video.fps <= 0 {
		return config{}, errors.New("serial baud and video dimensions/FPS must be positive")
	}
	return cfg, nil
}

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func envInt(name string, fallback int) int {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
