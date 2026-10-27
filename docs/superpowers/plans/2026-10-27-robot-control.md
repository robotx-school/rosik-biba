# Robot Control Implementation Plan

> Implement inline using superpowers:executing-plans. User approved implementation.

**Goal:** Organize the existing driver unchanged and implement independent Go manipulator control and combined ESP32 firmware.
**Architecture:** Existing driver stays intact in driver/. An independent manipulator Go module owns operator forwarding and robot serial access. Servo and lift run on the second ESP32.
**Tech Stack:** Go, tarm/serial, ESP32 Arduino, SCServo, GyverStepper.
**Spec:** docs/superpowers/specs/2026-10-27-robot-control-design.md

## Global Constraints

- Preserve existing site/camera code byte for byte; no network, routing, tailnet, firewall, or deployment configuration changes.
- Keep external hoverboard checkout unchanged; copy its sketch.
- Preserve binary servo protocol and upstream calibration conversion, including encoder wrap.
- No flashing or physical movement. Build/compile and use isolated simulated protocol tests.
- Work in this checkout on a new branch; no commits or publishing required.

## Review Focus

- Fragmented, oversized, zero-length and malformed packets cannot hang or trigger motion.
- Concurrent HTTP requests serialize access; timeout/reconnect never replays stale movement.
- Lift endstops, calibration travel/time limits, and missing communication stop stepping.
- Normalized calibrated poses use 0..10000; invalid feedback never becomes a move.
- Original driver file bytes and relative layout remain identical after relocation.

### Task 1: Repository organization

- [x] Capture original file digests (excluding metadata/docs), move application/deployment/artifacts into driver/, preserve relative paths and bytes.
- [x] Copy motor sketch into firmware/motor/esp32_dual_hover_uart/.
- [x] Confirm file digests after relocation; preserve ignore rules for secrets/artifacts.

### Task 2: Go manipulator packet and serial library

Files: manipulator/go.mod, internal/control/{packet,serial,command}.go and focused tests.
Interfaces: Packet{Type byte, Data []byte}; ReadPacket(io.Reader); WritePacket(io.Writer, Packet); Command.Validate(); Device.Execute(Command).
- [x] Pin upstream wire examples and malformed length/partial input cases in tests; implement bounded framing.
- [x] Add command validation and packet encoding for pose read/write, buttons, lift set/enable/disable/calibrate/status/stop/heartbeat.
- [x] Add serial open/close and serialized bounded transactions, resynchronization and explicit error results.
- [x] Run focused meaningful tests and build.

### Task 3: Receiver and operator commands

Files: manipulator/cmd/{receiver,controller}/main.go and internal/control/http.go.
- [x] Implement authenticated POST /command and GET /health; bounded strict JSON, request/read/write timeouts, serialized device access.
- [x] Implement controller JSON-lines forwarding and optional master serial polling for positions/buttons, bounded HTTP, cancellation, no replay.
- [x] Supply operator flags for IDs, speed, acceleration, polling rate, token, URL, serial and baud.
- [x] Exercise validation/authentication and partial/error feedback with simulated serial and HTTP; build commands.

### Task 4: Combined ESP32 firmware

Files: firmware/manipulator/manipulator/{manipulator.ino,config.h,calibration.h,lift.h,protocol.h}; upstream attribution and operator reader reference.
- [x] Retrieve pinned complete upstream sources and original lift sketch; retain license and calibration reference.
- [x] Implement calibrated STS conversion and persistent calibration with compatible NVS layout; preserve commissioning text commands.
- [x] Replace separate-Arduino lift UART with local GyverStepper tick loop, configurable GPIOs, endstop checks and bounded homing.
- [x] Implement bounded nonblocking packet parser, exact payload checks, zero-length commands, command ACK/error, heartbeat watchdog and lift status.
- [x] Compile merged sketch and motor sketch using installed ESP32 core and local library dependencies. Do not upload.

### Task 5: Documentation and final review

Files: root README.md; manipulator/README.md; firmware/README.md; docs/architecture.md.
- [x] Document components, builds, flags, JSON examples, binary protocol, physical wiring defaults, calibration and startup behavior.
- [x] Correct design assumptions using retrieved upstream evidence (positions are calibrated normalized 0..10000, not raw ticks).
- [x] Run Go build/tests, firmware compile, driver digest comparison and git diff checks.
- [x] Obtain independent read-only whole-change review, address material findings, report verified limitations.
