# Repository Guidelines

## Project Structure & Module Organization

- `driver/`: independent Go module containing the driving website, embedded HTML in `templates/`, WebRTC camera relay, and robot agent in `cmd/agent/`. Deployment files remain alongside this application.
- `manipulator/`: independent Go module with `cmd/receiver/` for the Raspberry Pi, `cmd/controller/` for operator input, and serial/HTTP logic in `internal/control/`.
- `firmware/motor/`: hoverboard ESP32 bridge. `firmware/manipulator/manipulator/`: combined servo and stepper firmware; hardware assignments live in `config.h`.
- `firmware/manipulator/tests/`: host C++ checks and hardware stubs. `firmware/upstream/`: original references and licenses; preserve source attribution.
- `docs/`: architecture, protocol, deployment notes, and design records.

## Build, Test, and Development Commands

Run Go commands inside the appropriate module; there is no root Go module.

```sh
(cd driver && go build ./... && go test ./...)
(cd manipulator && go build ./... && go test -race ./... && go vet ./...)
./firmware/manipulator/tests/run.sh
./firmware/build.sh
```

The firmware test script runs simulated calibration, parser, and lift checks. The build script downloads pinned libraries into ignored `firmware/.deps/` and compiles both ESP32 sketches using Arduino CLI.

For local receiver development:

```sh
cd manipulator
go run ./cmd/receiver -serial /dev/YOUR_MANIPULATOR -listen 127.0.0.1:8070
```

Set `MANIPULATOR_TOKEN` first. See each module's README for other launch commands.

## Coding Style & Naming Conventions

Format Go with `gofmt`; use idiomatic Go identifiers and lowercase package names. Keep platform code behind build tags, as in `serial_linux.go`. Follow surrounding Arduino/C++ formatting and use descriptive hardware constants. Preserve binary packet layouts and normalized servo positions (`0..10000`).

## Testing Guidelines

Use Go's `testing` package with `*_test.go` files and `TestBehavior` names. Firmware checks use C++ assertions and `*_test.cpp` files. Cover malformed packets, deadlines, calibration wrap, endstops, and watchdog behavior when changing those paths. No numerical coverage threshold is configured. Simulation and compilation do not establish hardware correctness.

## Commit & Pull Request Guidelines

History uses Conventional Commit subjects such as `feat: integrate Go manipulator control and combined ESP32 firmware`. Use concise imperative subjects and an appropriate type, such as `feat:` or `fix:`. PRs should describe behavior changes, affected components, validation, and hardware limitations. Link relevant issues; include screenshots for visible UI changes.

## Configuration & Hardware Boundaries

Keep tokens in ignored `.env` files; exclude binaries and downloaded dependencies. Preserve driver/camera behavior and network settings unless explicitly requested. Deployment, firmware flashing, homing, and movement are distinct actions: obtain explicit scope before operating hardware. Never replay uncertain movement commands automatically.
