package main

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"bibika-server/internal/protocol"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
)

const readBufSize = 1500

type cameraPublisher struct {
	cfg config
	mu  sync.Mutex

	peer   *webrtc.PeerConnection
	track  *webrtc.TrackLocalStaticSample
	cancel context.CancelFunc
}

func newCameraPublisher(cfg config) *cameraPublisher {
	return &cameraPublisher{cfg: cfg}
}

func (p *cameraPublisher) start(socket *lockedSocket) error {
	p.close()
	peerConfig := webrtc.Configuration{ICEServers: p.cfg.iceConfig.Servers}
	if p.cfg.iceConfig.Relay {
		peerConfig.ICETransportPolicy = webrtc.ICETransportPolicyRelay
	}
	peer, err := webrtc.NewPeerConnection(peerConfig)
	if err != nil {
		return err
	}
	peer.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		if state == webrtc.PeerConnectionStateDisconnected || state == webrtc.PeerConnectionStateFailed {
			// Keep the control WebSocket alive when media fails. A camera/network
			// issue must not interrupt robot control; the publisher will continue
			// retrying camera capture while the agent remains connected.
			slog.Warn("camera media connection lost", "state", state.String())
		}
	})
	track, err := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8, ClockRate: 90000}, "camera", "bibika")
	if err != nil {
		_ = peer.Close()
		return err
	}
	sender, err := peer.AddTrack(track)
	if err != nil {
		_ = peer.Close()
		return err
	}
	go drainRTCP(sender)
	offer, err := peer.CreateOffer(nil)
	if err != nil {
		_ = peer.Close()
		return err
	}
	gathered := webrtc.GatheringCompletePromise(peer)
	if err := peer.SetLocalDescription(offer); err != nil {
		_ = peer.Close()
		return err
	}
	<-gathered
	p.mu.Lock()
	p.peer = peer
	p.track = track
	p.mu.Unlock()
	return socket.send(protocol.Message{Type: "camera_publish_offer", SDP: peer.LocalDescription().SDP, SDPType: "offer"})
}

func (p *cameraPublisher) setAnswer(message protocol.Message) error {
	if message.SDPType != "answer" || message.SDP == "" {
		return errors.New("invalid camera publisher answer")
	}
	p.mu.Lock()
	peer := p.peer
	track := p.track
	p.mu.Unlock()
	if peer == nil || track == nil {
		return errors.New("camera publisher is not initialized")
	}
	if err := peer.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeAnswer, SDP: message.SDP}); err != nil {
		return err
	}
	p.mu.Lock()
	if p.cancel == nil {
		ctx, cancel := context.WithCancel(context.Background())
		p.cancel = cancel
		go p.run(ctx, track)
	}
	p.mu.Unlock()
	return nil
}

func (p *cameraPublisher) close() {
	p.mu.Lock()
	if p.cancel != nil {
		p.cancel()
		p.cancel = nil
	}
	if p.peer != nil {
		_ = p.peer.Close()
		p.peer = nil
		p.track = nil
	}
	p.mu.Unlock()
}

func (p *cameraPublisher) run(ctx context.Context, track *webrtc.TrackLocalStaticSample) {
	for ctx.Err() == nil {
		if err := publishCamera(ctx, p.cfg.video, track); err != nil && ctx.Err() == nil {
			slog.Warn("camera publisher stopped", "error", err)
			select {
			case <-ctx.Done():
			case <-time.After(time.Second):
			}
		}
	}
}

func publishCamera(ctx context.Context, cfg videoConfig, track *webrtc.TrackLocalStaticSample) error {
	cmd, stdout, err := startVP8Camera(ctx, cfg)
	if err != nil {
		return err
	}
	defer func() {
		_ = stdout.Close()
		_ = cmd.Wait()
	}()
	reader := bufio.NewReaderSize(stdout, readBufSize*4)
	if err := readIVFHeader(reader); err != nil {
		return err
	}
	frameDuration := time.Second / time.Duration(cfg.fps)
	for ctx.Err() == nil {
		frame, err := readIVFFrame(reader)
		if err != nil {
			return err
		}
		if err := track.WriteSample(media.Sample{Data: frame, Duration: frameDuration}); err != nil {
			return err
		}
	}
	return nil
}

func drainRTCP(sender *webrtc.RTPSender) {
	buffer := make([]byte, readBufSize)
	for {
		if _, _, err := sender.Read(buffer); err != nil {
			return
		}
	}
}

func startVP8Camera(ctx context.Context, cfg videoConfig) (*exec.Cmd, io.ReadCloser, error) {
	args := []string{
		"-hide_banner", "-loglevel", "warning",
		"-f", "v4l2", "-input_format", "mjpeg",
		"-video_size", fmt.Sprintf("%dx%d", cfg.width, cfg.height),
		"-framerate", strconv.Itoa(cfg.fps), "-i", cfg.device,
		"-an", "-c:v", "libvpx", "-deadline", "realtime", "-cpu-used", "8",
		"-b:v", "1200k", "-maxrate", "1200k", "-bufsize", "600k",
		"-g", strconv.Itoa(cfg.fps), "-error-resilient", "1", "-f", "ivf", "-",
	}
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}
	return cmd, stdout, nil
}

func readIVFHeader(r io.Reader) error {
	header := make([]byte, 32)
	if _, err := io.ReadFull(r, header); err != nil {
		return err
	}
	if string(header[:4]) != "DKIF" {
		return errors.New("invalid IVF signature")
	}
	return nil
}

func readIVFFrame(r io.Reader) ([]byte, error) {
	header := make([]byte, 12)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, err
	}
	size := binary.LittleEndian.Uint32(header[:4])
	if size == 0 || size > 4*1024*1024 {
		return nil, fmt.Errorf("invalid IVF frame size %d", size)
	}
	frame := make([]byte, size)
	_, err := io.ReadFull(r, frame)
	return frame, err
}
