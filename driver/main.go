package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"bibika-server/internal/ice"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"golang.org/x/net/websocket"
)

const defaultListenAddr = ":8067"

type serverConfig struct {
	listenAddr     string
	agentToken     string
	defaultRobotID string
	publicIP       string
	iceUDPPort     int
	iceConfig      ice.Config
}

func main() {
	cfg, err := loadServerConfig()
	if err != nil {
		slog.Error("invalid server configuration", "error", err)
		os.Exit(1)
	}

	webrtcAPI, mediaSocket, err := newWebRTCAPI(cfg.publicIP, cfg.iceUDPPort)
	if err != nil {
		slog.Error("configure WebRTC media socket", "error", err)
		os.Exit(1)
	}
	defer mediaSocket.Close()
	hub := newHub(cfg.agentToken, cfg.defaultRobotID, webrtcAPI, cfg.iceConfig)
	renderer, err := newTemplateRenderer()
	if err != nil {
		slog.Error("parse templates", "error", err)
		os.Exit(1)
	}

	e := echo.New()
	e.HideBanner = true
	e.Use(middleware.Recover())
	e.GET("/", func(c echo.Context) error {
		c.Response().Header().Set(echo.HeaderContentType, echo.MIMETextHTMLCharsetUTF8)
		c.Response().WriteHeader(http.StatusOK)
		return renderer.render(c.Response(), "index.html", indexPageData{LinearLimit: 1.5, AngularLimit: 2.0, RobotID: cfg.defaultRobotID, ICEServers: cfg.iceConfig.Servers, RelayOnly: cfg.iceConfig.Relay})
	})
	e.GET("/ws/control", echo.WrapHandler(websocket.Handler(hub.handleControlWS)))
	e.GET("/ws/camera", echo.WrapHandler(websocket.Handler(hub.handleCameraWS)))
	e.GET("/agent/connect", echo.WrapHandler(websocket.Handler(hub.handleAgentWS)))
	e.GET("/ping", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	go func() {
		slog.Info("server listening", "addr", cfg.listenAddr)
		if err := e.Start(cfg.listenAddr); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server stopped unexpectedly", "error", err)
		}
	}()

	shutdownSignal := make(chan os.Signal, 1)
	signal.Notify(shutdownSignal, syscall.SIGINT, syscall.SIGTERM)
	<-shutdownSignal

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := e.Shutdown(ctx); err != nil {
		slog.Error("server shutdown failed", "error", err)
	}
}

func loadServerConfig() (serverConfig, error) {
	token := os.Getenv("BIBIKA_AGENT_TOKEN")
	if token == "" {
		return serverConfig{}, errors.New("BIBIKA_AGENT_TOKEN is required")
	}
	cfg := serverConfig{
		listenAddr:     env("BIBIKA_LISTEN", defaultListenAddr),
		agentToken:     token,
		defaultRobotID: env("BIBIKA_ROBOT_ID", "default"),
		publicIP:       os.Getenv("BIBIKA_PUBLIC_IP"),
		iceUDPPort:     envInt("BIBIKA_ICE_UDP_PORT", 31415),
		iceConfig:      ice.Load(),
	}
	if cfg.iceUDPPort < 1 || cfg.iceUDPPort > 65535 {
		return serverConfig{}, errors.New("invalid WebRTC UDP port")
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
