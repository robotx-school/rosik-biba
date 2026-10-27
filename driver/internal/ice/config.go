package ice

import (
	"os"
	"strings"

	"github.com/pion/webrtc/v4"
)

// Config is the common ICE configuration used by the server and robot agent.
// BIBIKA_ICE_URLS accepts a comma-separated STUN and/or TURN URL list.
type Config struct {
	Servers []webrtc.ICEServer
	Relay   bool
}

func Load() Config {
	urls := splitURLs(os.Getenv("BIBIKA_ICE_URLS"))
	if len(urls) == 0 {
		return Config{}
	}
	return Config{
		Servers: []webrtc.ICEServer{{
			URLs:       urls,
			Username:   os.Getenv("BIBIKA_ICE_USERNAME"),
			Credential: os.Getenv("BIBIKA_ICE_CREDENTIAL"),
		}},
		Relay: strings.EqualFold(os.Getenv("BIBIKA_ICE_TRANSPORT_POLICY"), "relay"),
	}
}

func splitURLs(raw string) []string {
	parts := strings.Split(raw, ",")
	urls := make([]string, 0, len(parts))
	for _, part := range parts {
		if url := strings.TrimSpace(part); url != "" {
			urls = append(urls, url)
		}
	}
	return urls
}
