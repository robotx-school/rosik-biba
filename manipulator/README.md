# Go manipulator control

Independent of the existing driver site and camera. The Go serial library replaces Python `ServoController`: batch pose read/write, six-button reads on an upstream master reader, and lift enable/disable/home/position. The controller supports JSON lines or polling a physical master reader.

## Build

```sh
go build -o bin/receiver ./cmd/receiver
go build -o bin/controller ./cmd/controller
```

## Raspberry Pi receiver

Set `MANIPULATOR_TOKEN` to the same nonempty secret on both machines. Supply your own listen address; no network configuration is performed.

```sh
export MANIPULATOR_TOKEN='your-shared-secret'
./bin/receiver -listen YOUR_RPI_ADDRESS:8070 -serial /dev/serial/by-id/YOUR_MANIPULATOR_ESP32
```

The default bind is `127.0.0.1:8070`. `-baud` defaults to 115200. Never point this process at the motor ESP32. Serial connects lazily on the first command. `/health` reports the process running, not a tested serial connection.

## Operator: JSON lines

```sh
export MANIPULATOR_TOKEN='your-shared-secret'
./bin/controller -url http://YOUR_RPI_ADDRESS:8070
```

Type one JSON object per line. The process sends heartbeats while waiting for input:

```json
{"action":"read","ids":[1,2]}
{"action":"poses","ids":[1,2],"positions":[5000,5000],"speed":400,"accel":50}
{"action":"calibrate"}
{"action":"status"}
{"action":"lift","mm":100}
{"action":"stop"}
{"action":"disable"}
```

Home is asynchronous: wait until `status` shows `calibrated: true` and `calibrating: false` before commanding height. Homing enables the lift; `enable` alone does not home it. Positions are calibrated **0..10000**, not raw encoder ticks. IDs are unique 1..253, at most 12 per batch. Speed is 0..4095 and acceleration 0..255; zero speed uses the servo firmware's own interpretation, so supply an explicit speed for routine moves. Height is positive downward from the upper switch; firmware enforces `MAX_TRAVEL_MM` (default 600mm). `stop` retains lift holding power; `disable` removes it and invalidates lift calibration.

For an input file or pipe, EOF normally exits and stops the lift. Use `-hold` to maintain heartbeats for a bounded duration after EOF, for example:

```sh
printf '%s\n' '{"action":"calibrate"}' | ./bin/controller -url http://YOUR_RPI_ADDRESS:8070 -hold 20s
```

A failed input or connection ends forwarding; the firmware lift watchdog stops after the lease expires. A new controller session does not replay old targets or automatically home. Ctrl-C requests a lift stop. Servo targets continue normal position holding.

## Operator: physical master manipulator

Use the existing upstream reader firmware and its own NVS servo calibration on the operator-side ESP32, connected to the master servo bus:

```sh
./bin/controller -url http://YOUR_RPI_ADDRESS:8070 \
  -serial /dev/serial/by-id/YOUR_MASTER_READER \
  -ids 1,2,3,4,5,6,7,8,9,10,11,12 -speed 400 -accel 50 -interval 50ms
```

Read failures and invalid feedback stop forwarding rather than sending an invalid pose. There is no movement retry or queued pose replay. Polling waits for each read and HTTP response; `-interval` is a requested minimum period, not a guaranteed frame rate. Configure and validate both master and robot calibration first. The program forwards already normalized positions directly; encoder wrap conversion stays in the firmware.

## HTTP API

`POST /command`, `Authorization: Bearer <MANIPULATOR_TOKEN>`, one command object. Successful responses contain `status: accepted` for an ACK or `status: feedback` plus `positions` or `lift`. Body limit 4096 bytes; unknown fields, duplicate IDs, out-of-range values, missing height and extra JSON objects are rejected. Serial errors return HTTP 503; invalid commands return 400; authentication errors return 401. No CORS or browser integration is added.

`heartbeat`, `enable`, `disable`, `calibrate`, `stop` and `status` take no extra fields. `buttons` is available through `control.Device.Execute` for a master reader, but the robot ESP32 has no master buttons and rejects that request. For sustained lift motion, API clients must send `heartbeat` at least every 200ms and stop sending when the operator disconnects.

See [protocol and timing](../docs/architecture.md) and [firmware commissioning](../firmware/README.md).
