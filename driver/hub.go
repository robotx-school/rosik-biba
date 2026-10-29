package main

import (
	"errors"
	"log/slog"
	"net"
	"sync"

	"bibika-server/internal/ice"
	"time"

	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
	"golang.org/x/net/websocket"
)

const agentHeartbeatInterval = time.Second
const agentStatusTimeout = 500 * time.Millisecond

type socketPeer struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func (p *socketPeer) send(message agentMessage) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return websocket.JSON.Send(p.conn, message)
}

type agentSession struct {
	*socketPeer
	robotID    string
	status     robotStatus
	lastStatus time.Time
	camera     *webrtc.PeerConnection
}

type controlSession struct {
	*socketPeer
	robotID string
}

type cameraSession struct {
	*socketPeer
	robotID string
}

type cameraSource struct {
	mu             sync.Mutex
	tracks         map[*webrtc.TrackLocalStaticRTP]struct{}
	cachedKeyframe []*rtp.Packet
	cacheTimestamp uint32
	cachingFrame   bool
}

func newCameraSource() *cameraSource {
	return &cameraSource{tracks: make(map[*webrtc.TrackLocalStaticRTP]struct{})}
}

func (s *cameraSource) add(track *webrtc.TrackLocalStaticRTP) {
	s.mu.Lock()
	s.tracks[track] = struct{}{}
	cachedKeyframe := append([]*rtp.Packet(nil), s.cachedKeyframe...)
	s.mu.Unlock()
	for _, packet := range cachedKeyframe {
		if err := track.WriteRTP(packet); err != nil {
			slog.Debug("cached camera keyframe relay failed", "error", err)
			return
		}
	}
}

func (s *cameraSource) remove(track *webrtc.TrackLocalStaticRTP) {
	s.mu.Lock()
	delete(s.tracks, track)
	s.mu.Unlock()
}

func (s *cameraSource) clearCache() {
	s.mu.Lock()
	s.cachedKeyframe = nil
	s.cachingFrame = false
	s.mu.Unlock()
}

func (s *cameraSource) broadcast(packet *rtp.Packet) {
	s.mu.Lock()
	s.cacheKeyframe(packet)
	tracks := make([]*webrtc.TrackLocalStaticRTP, 0, len(s.tracks))
	for track := range s.tracks {
		tracks = append(tracks, track)
	}
	s.mu.Unlock()

	for _, track := range tracks {
		if err := track.WriteRTP(packet); err != nil {
			slog.Debug("camera packet relay failed", "error", err)
		}
	}
}

func (s *cameraSource) cacheKeyframe(packet *rtp.Packet) {
	if isVP8KeyframeStart(packet.Payload) {
		s.cachedKeyframe = []*rtp.Packet{cloneRTPPacket(packet)}
		s.cacheTimestamp = packet.Timestamp
		s.cachingFrame = true
		return
	}
	if !s.cachingFrame {
		return
	}
	if packet.Timestamp != s.cacheTimestamp {
		s.cachingFrame = false
		return
	}
	s.cachedKeyframe = append(s.cachedKeyframe, cloneRTPPacket(packet))
}

func cloneRTPPacket(packet *rtp.Packet) *rtp.Packet {
	clone := *packet
	clone.Payload = append([]byte(nil), packet.Payload...)
	clone.Header.CSRC = append([]uint32(nil), packet.Header.CSRC...)
	return &clone
}

func isVP8KeyframeStart(payload []byte) bool {
	if len(payload) < 2 || payload[0]&0x10 == 0 || payload[0]&0x0F != 0 {
		return false
	}
	offset := 1
	if payload[0]&0x80 != 0 {
		if len(payload) <= offset {
			return false
		}
		extension := payload[offset]
		offset++
		if extension&0x80 != 0 {
			if len(payload) <= offset {
				return false
			}
			if payload[offset]&0x80 != 0 {
				offset += 2
			} else {
				offset++
			}
		}
		if extension&0x40 != 0 {
			offset++
		}
		if extension&0x20 != 0 {
			offset++
		}
	}
	return len(payload) > offset && payload[offset]&0x01 == 0
}

type hub struct {
	mu             sync.Mutex
	agentToken     string
	defaultRobotID string
	webrtcAPI      *webrtc.API
	peerConfig     webrtc.Configuration
	agents         map[string]*agentSession
	controllers    map[*controlSession]struct{}
	cameras        map[string]*cameraSource
	viewers        map[*cameraSession]struct{}
}

func newHub(agentToken, defaultRobotID string, webrtcAPI *webrtc.API, iceConfig ice.Config) *hub {
	peerConfig := webrtc.Configuration{ICEServers: iceConfig.Servers}
	if iceConfig.Relay {
		peerConfig.ICETransportPolicy = webrtc.ICETransportPolicyRelay
	}
	return &hub{
		agentToken:     agentToken,
		defaultRobotID: defaultRobotID,
		webrtcAPI:      webrtcAPI,
		peerConfig:     peerConfig,
		agents:         make(map[string]*agentSession),
		controllers:    make(map[*controlSession]struct{}),
		cameras:        make(map[string]*cameraSource),
		viewers:        make(map[*cameraSession]struct{}),
	}
}

func newWebRTCAPI(publicIP string, port int) (*webrtc.API, *net.UDPConn, error) {
	mediaSocket, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4zero, Port: port})
	if err != nil {
		return nil, nil, err
	}
	settings := webrtc.SettingEngine{}
	settings.SetICEUDPMux(webrtc.NewICEUDPMux(nil, mediaSocket))
	if publicIP != "" {
		settings.SetNAT1To1IPs([]string{publicIP}, webrtc.ICECandidateTypeHost)
	}
	return webrtc.NewAPI(webrtc.WithSettingEngine(settings)), mediaSocket, nil
}

func (h *hub) handleAgentWS(ws *websocket.Conn) {
	defer ws.Close()
	var hello agentMessage
	if err := websocket.JSON.Receive(ws, &hello); err != nil || hello.Type != "hello" || hello.Token != h.agentToken || hello.RobotID == "" {
		slog.Warn("rejected agent connection", "error", err)
		return
	}

	session := &agentSession{socketPeer: &socketPeer{conn: ws}, robotID: hello.RobotID, status: robotStatus{Online: true}}
	h.registerAgent(session)
	defer h.unregisterAgent(session)
	heartbeatDone := make(chan struct{})
	defer close(heartbeatDone)
	go h.sendAgentHeartbeats(session, heartbeatDone)
	go h.watchAgentStatus(session, heartbeatDone)

	for {
		var message agentMessage
		if err := websocket.JSON.Receive(ws, &message); err != nil {
			return
		}
		switch message.Type {
		case "status", "heartbeat":
			h.updateStatus(session, message)
		case "power_result":
			h.broadcastPowerResult(session.robotID, message.PowerStatus)
		case "camera_publish_offer":
			if err := h.acceptCameraPublisher(session, message); err != nil {
				slog.Warn("camera publisher setup failed", "robot_id", session.robotID, "error", err)
			}
		}
	}
}

func (h *hub) sendAgentHeartbeats(session *agentSession, done <-chan struct{}) {
	ticker := time.NewTicker(agentHeartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			if err := session.send(agentMessage{Type: "ping"}); err != nil {
				_ = session.conn.Close()
				return
			}
		}
	}
}

func (h *hub) registerAgent(session *agentSession) {
	h.mu.Lock()
	previous := h.agents[session.robotID]
	h.agents[session.robotID] = session
	h.mu.Unlock()
	if previous != nil && previous != session {
		_ = previous.conn.Close()
	}
	h.broadcastStatus(session.robotID, session.status)
	slog.Info("robot agent connected", "robot_id", session.robotID)
}

func (h *hub) unregisterAgent(session *agentSession) {
	h.mu.Lock()
	isCurrent := h.agents[session.robotID] == session
	if isCurrent {
		delete(h.agents, session.robotID)
	}
	h.mu.Unlock()
	if !isCurrent {
		return
	}
	if session.camera != nil {
		_ = session.camera.Close()
	}
	h.clearCameraCache(session.robotID)
	h.notifyCameraReset(session.robotID)
	h.broadcastStatus(session.robotID, robotStatus{})
	slog.Info("robot agent disconnected", "robot_id", session.robotID)
}

func (h *hub) clearCameraCache(robotID string) {
	h.mu.Lock()
	source := h.cameras[robotID]
	h.mu.Unlock()
	if source != nil {
		source.clearCache()
	}
}

func (h *hub) addViewer(session *cameraSession) {
	h.mu.Lock()
	h.viewers[session] = struct{}{}
	h.mu.Unlock()
}

func (h *hub) removeViewer(session *cameraSession) {
	h.mu.Lock()
	delete(h.viewers, session)
	h.mu.Unlock()
}

func (h *hub) notifyCameraReset(robotID string) {
	h.mu.Lock()
	viewers := make([]*cameraSession, 0, len(h.viewers))
	for viewer := range h.viewers {
		if viewer.robotID == robotID {
			viewers = append(viewers, viewer)
		}
	}
	h.mu.Unlock()
	for _, viewer := range viewers {
		if err := viewer.send(agentMessage{Type: "camera_reset"}); err != nil {
			_ = viewer.conn.Close()
		}
	}
}

func (h *hub) updateStatus(session *agentSession, message agentMessage) {
	status := robotStatus{Online: true, SerialConnected: message.SerialConnected, LastFeedback: message.LastFeedback, PowerStatus: message.PowerStatus}
	h.mu.Lock()
	if h.agents[session.robotID] != session {
		h.mu.Unlock()
		return
	}
	session.status = status
	session.lastStatus = time.Now()
	h.mu.Unlock()
	h.broadcastStatus(session.robotID, status)
}

func (h *hub) watchAgentStatus(session *agentSession, done <-chan struct{}) {
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			h.mu.Lock()
			stale := h.agents[session.robotID] == session && session.status.SerialConnected && !session.lastStatus.IsZero() && time.Since(session.lastStatus) >= agentStatusTimeout
			if stale {
				session.status.SerialConnected = false
				session.status.PowerStatus = "offline"
			}
			status := session.status
			h.mu.Unlock()
			if stale {
				h.broadcastStatus(session.robotID, status)
			}
		}
	}
}

func (h *hub) broadcastStatus(robotID string, status robotStatus) {
	h.mu.Lock()
	controllers := make([]*controlSession, 0, len(h.controllers))
	for controller := range h.controllers {
		if controller.robotID == robotID {
			controllers = append(controllers, controller)
		}
	}
	h.mu.Unlock()
	for _, controller := range controllers {
		if err := controller.send(agentMessage{Type: "status", RobotID: robotID, SerialConnected: status.SerialConnected, LastFeedback: status.LastFeedback, PowerStatus: status.PowerStatus}); err != nil {
			_ = controller.conn.Close()
		}
	}
}

func (h *hub) handleControlWS(ws *websocket.Conn) {
	defer ws.Close()
	robotID := ws.Request().URL.Query().Get("robotId")
	if robotID == "" {
		robotID = h.defaultRobotID
	}
	session := &controlSession{socketPeer: &socketPeer{conn: ws}, robotID: robotID}
	h.mu.Lock()
	h.controllers[session] = struct{}{}
	agent := h.agents[robotID]
	var initialStatus robotStatus
	if agent != nil {
		initialStatus = agent.status
	}
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		delete(h.controllers, session)
		h.mu.Unlock()
		h.sendStop(robotID)
	}()

	if agent != nil {
		_ = session.send(agentMessage{Type: "status", RobotID: robotID, SerialConnected: initialStatus.SerialConnected, LastFeedback: initialStatus.LastFeedback, PowerStatus: initialStatus.PowerStatus})
	} else {
		_ = session.send(agentMessage{Type: "status", RobotID: robotID})
	}

	for {
		var command agentMessage
		if err := websocket.JSON.Receive(ws, &command); err != nil {
			return
		}
		if command.Type != "control" && command.Type != "power_pulse" {
			continue
		}
		h.mu.Lock()
		agent = h.agents[robotID]
		h.mu.Unlock()
		if agent != nil {
			forward := agentMessage{Type: command.Type}
			if command.Type == "control" {
				forward.Linear = command.Linear
				forward.Angular = command.Angular
			}
			if err := agent.send(forward); err != nil {
				_ = agent.conn.Close()
			}
		} else if command.Type == "power_pulse" {
			_ = session.send(agentMessage{Type: "power_result", PowerStatus: "offline"})
		}
	}
}

func (h *hub) sendStop(robotID string) {
	h.mu.Lock()
	agent := h.agents[robotID]
	h.mu.Unlock()
	if agent != nil {
		_ = agent.send(agentMessage{Type: "control"})
	}
}

func (h *hub) acceptCameraPublisher(session *agentSession, message agentMessage) error {
	if message.SDP == "" || message.SDPType != "offer" {
		return errors.New("invalid publisher offer")
	}
	if session.camera != nil {
		_ = session.camera.Close()
	}
	h.clearCameraCache(session.robotID)
	h.notifyCameraReset(session.robotID)
	peer, err := h.webrtcAPI.NewPeerConnection(h.peerConfig)
	if err != nil {
		return err
	}
	session.camera = peer
	peer.OnTrack(func(remote *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
		go h.relayCamera(session.robotID, remote)
	})
	if err := peer.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: message.SDP}); err != nil {
		_ = peer.Close()
		return err
	}
	answer, err := peer.CreateAnswer(nil)
	if err != nil {
		_ = peer.Close()
		return err
	}
	gathered := webrtc.GatheringCompletePromise(peer)
	if err := peer.SetLocalDescription(answer); err != nil {
		_ = peer.Close()
		return err
	}
	<-gathered
	return session.send(agentMessage{Type: "camera_publish_answer", SDP: peer.LocalDescription().SDP, SDPType: "answer"})
}

func (h *hub) relayCamera(robotID string, remote *webrtc.TrackRemote) {
	h.mu.Lock()
	source := h.cameras[robotID]
	if source == nil {
		source = newCameraSource()
		h.cameras[robotID] = source
	}
	h.mu.Unlock()
	for {
		packet, _, err := remote.ReadRTP()
		if err != nil {
			return
		}
		source.broadcast(packet)
	}
}

func (h *hub) handleCameraWS(ws *websocket.Conn) {
	defer ws.Close()
	robotID := ws.Request().URL.Query().Get("robotId")
	if robotID == "" {
		robotID = h.defaultRobotID
	}
	session := &cameraSession{socketPeer: &socketPeer{conn: ws}, robotID: robotID}
	var offer agentMessage
	if err := websocket.JSON.Receive(ws, &offer); err != nil || offer.Type != "camera_offer" || offer.SDPType != "offer" {
		return
	}

	peer, err := h.webrtcAPI.NewPeerConnection(h.peerConfig)
	if err != nil {
		return
	}
	defer peer.Close()
	h.mu.Lock()
	source := h.cameras[robotID]
	if source == nil {
		source = newCameraSource()
		h.cameras[robotID] = source
	}
	h.mu.Unlock()
	track, err := webrtc.NewTrackLocalStaticRTP(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8, ClockRate: 90000}, "camera", robotID)
	if err != nil {
		return
	}
	if _, err := peer.AddTrack(track); err != nil {
		return
	}
	source.add(track)
	defer source.remove(track)

	if err := peer.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: offer.SDP}); err != nil {
		return
	}
	answer, err := peer.CreateAnswer(nil)
	if err != nil {
		return
	}
	gathered := webrtc.GatheringCompletePromise(peer)
	if err := peer.SetLocalDescription(answer); err != nil {
		return
	}
	<-gathered
	if err := session.send(agentMessage{Type: "camera_answer", SDP: peer.LocalDescription().SDP, SDPType: "answer"}); err != nil {
		return
	}
	h.addViewer(session)
	defer h.removeViewer(session)

	buffer := make([]byte, 1)
	_, _ = ws.Read(buffer)
}
