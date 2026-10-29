# Bibika Robot Relay

The project runs as two separate Go binaries:

```text
Browser UI -- HTTPS/WSS --> Public server <-- outbound WSS -- Robot agent
                                  ^
                                  | WebRTC video relay
                                  +-- Agent camera publisher and browser viewers
```

The robot agent is the only process that accesses the serial device and camera. It makes an outbound connection to the public server, so the server never opens a connection to the robot.

## Build

```sh
go build -o bibika-server .
go build -o bibika-agent ./cmd/agent
```

## Run the public server

```sh
export BIBIKA_AGENT_TOKEN='replace-with-a-long-random-secret'
export BIBIKA_LISTEN=':8067'             # optional
export BIBIKA_ROBOT_ID='default'          # optional
./bibika-server
```

Serve the public endpoint over HTTPS in production. The server must be publicly reachable over TCP for HTTPS/WSS and UDP for WebRTC media.

WebRTC media is bound to UDP `31415`. Forward both TCP and UDP `31415` from the public address to the server host.

### TURN relay

STUN discovers addresses but never relays video. For video to work when the robot is on a different or restrictive network, run a publicly reachable TURN server and set these variables on both the server and robot agent:

```sh
export BIBIKA_ICE_URLS='turn:turn.example.com:3478?transport=udp'
export BIBIKA_ICE_USERNAME='bibika'
export BIBIKA_ICE_CREDENTIAL='replace-with-a-long-random-secret'
export BIBIKA_ICE_TRANSPORT_POLICY='relay'
```

`relay` forces both WebRTC legs through TURN: robot → TURN → server → TURN → browser. The TURN host needs TCP/UDP 3478 and its configured UDP relay range open to the Internet. It must use a public address or DNS name; `192.168.0.66` can only work for devices on that LAN.

## Run the robot agent

```sh
export BIBIKA_SERVER_URL='wss://robot.example.com/agent/connect'
export BIBIKA_AGENT_TOKEN='replace-with-the-same-secret'
export BIBIKA_ROBOT_ID='default'
export BIBIKA_SERIAL='/dev/ttyUSB1'       # optional
export BIBIKA_BAUD='115200'               # optional
export BIBIKA_VIDEO='/dev/video0'         # optional
export BIBIKA_VIDEO_WIDTH='1280'          # optional
export BIBIKA_VIDEO_HEIGHT='720'          # optional
export BIBIKA_VIDEO_FPS='30'              # optional
./bibika-agent
```

The agent reconnects automatically after a network failure. It clamps all motion values locally and sends a zero command when it has not received a fresh server command for 250 ms.

## Protocol

The agent first sends an authenticated `hello` message over its outbound WebSocket. The server then forwards browser control messages to that connection and broadcasts agent status to browser control sessions.

Camera media is not sent through the control WebSocket. The agent publishes a VP8 WebRTC track to the server, and the server relays RTP packets to each browser WebRTC viewer.

## Operational Requirements

- Install `ffmpeg` with `libvpx` support on the robot.
- Grant the agent account access to the serial and V4L2 devices.
- Use the same non-empty `BIBIKA_AGENT_TOKEN` on both processes.
- Put the public server behind TLS; the browser automatically selects `wss://` when loaded through HTTPS.
- The current UI has no user authentication. Do not expose it to untrusted networks until browser authentication and per-robot authorization are added.


## Motor power relay

The **Toggle motor power** button sends `{ "type": "power_pulse" }` over the existing control WebSocket. The server forwards it to the robot agent, which writes one `POWER_PULSE` serial line. The motor ESP32 pulses configurable GPIO25 active high for 500ms, then releases it. The same physical pulse toggles enable or disable; neither the browser nor ESP32 knows the actual power state.

The button is disabled while disconnected or awaiting a pulse/cooldown. It displays firmware confirmation and reports missing/uncertain outcomes without retrying. Driver status includes `powerStatus`; immediate request results use `power_result`. Reloading or reconnecting never requests a pulse. Existing motor firmware without relay support will not confirm a pulse.

Build the updated server and agent and compile the motor firmware before deployment. See [relay wiring and polarity](../firmware/README.md#motor-power-relay). No relay is operated by the automated checks:

```sh
go test -race ./...
node frontend_power_test.cjs
../firmware/motor/tests/run.sh
```

On serial failure, the agent immediately marks the device offline and clears pending pulses. It quarantines the port until driver cleanup finishes; reconnect USB if kernel cleanup remains stuck. Idle serial timeouts do not stop the feedback reader, and retired readers cannot overwrite offline relay status.
