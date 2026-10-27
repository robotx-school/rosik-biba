package main

import (
	"testing"

	"github.com/pion/rtp"
)

func TestCameraSourceCachesLatestVP8Keyframe(t *testing.T) {
	source := newCameraSource()
	keyframeStart := &rtp.Packet{Header: rtp.Header{Timestamp: 10}, Payload: []byte{0x10, 0x00}}
	keyframeContinuation := &rtp.Packet{Header: rtp.Header{Timestamp: 10}, Payload: []byte{0x00, 0xFF}}
	nextFrame := &rtp.Packet{Header: rtp.Header{Timestamp: 11}, Payload: []byte{0x10, 0x01}}

	source.cacheKeyframe(keyframeStart)
	source.cacheKeyframe(keyframeContinuation)
	source.cacheKeyframe(nextFrame)

	if len(source.cachedKeyframe) != 2 {
		t.Fatalf("cached packets = %d, want 2", len(source.cachedKeyframe))
	}
	if source.cachedKeyframe[0].Payload[1] != 0 {
		t.Fatal("cached frame is not a VP8 keyframe")
	}
}

func TestVP8KeyframeStart(t *testing.T) {
	if !isVP8KeyframeStart([]byte{0x10, 0x00}) {
		t.Fatal("keyframe was not detected")
	}
	if isVP8KeyframeStart([]byte{0x10, 0x01}) {
		t.Fatal("interframe was incorrectly detected as a keyframe")
	}
}
