package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"rosik-manipulator/internal/control"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:8070", "HTTP listen address; set the address appropriate for your network")
	port := flag.String("serial", "", "manipulator ESP32 serial path (required)")
	baud := flag.Int("baud", 115200, "USB serial baud")
	flag.Parse()
	token := os.Getenv("MANIPULATOR_TOKEN")
	if *port == "" || *baud <= 0 || token == "" {
		log.Fatal("-serial, positive -baud, and MANIPULATOR_TOKEN are required")
	}
	device := control.NewDevice(*port, *baud)
	defer device.Close()
	server := &http.Server{Addr: *listen, Handler: control.NewHandler(token, device), ReadHeaderTimeout: 2 * time.Second, ReadTimeout: 4 * time.Second, WriteTimeout: 5 * time.Second, IdleTimeout: 15 * time.Second, MaxHeaderBytes: 8192}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()
	log.Printf("manipulator receiver listening on %s; serial %s", *listen, *port)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
