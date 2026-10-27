# Rosik Biba robot control

Two robot ESP32s connect independently to the Raspberry Pi:

- **[driver/](driver/README.md)** — the existing driving website, Go server/agent, camera stream and deployment files. Relocated together without changing their contents. Run its original commands from `driver/`.
- **[manipulator/](manipulator/README.md)** — Go operator controller and RPi manipulator receiver, replacing the Python serial controller and forwarding calibrated poses and lift commands.
- **[firmware/](firmware/README.md)** — original motor bridge and combined servo/stepper ESP32 firmware, wiring defaults, commissioning and build instructions.
- **[docs/architecture.md](docs/architecture.md)** — component and protocol reference.

The website and manipulator receiver run as separate processes on the RPi. You supply the network addresses and manage the tailnet/local network. This repository does not configure networking.

## Build Go programs

```sh
(cd driver && go build -o /tmp/bibika-server . && go build -o /tmp/bibika-agent ./cmd/agent)
(cd manipulator && go build -o /tmp/manipulator-receiver ./cmd/receiver && go build -o /tmp/manipulator-controller ./cmd/controller)
```

The motor and manipulator must use different serial devices. Prefer stable `/dev/serial/by-id/...` paths.

The implementation was compiled for ESP32 Dev Module. Physical pin assignments, limit switch polarity, usable lift travel and servo calibration must match your hardware before use. No firmware has been uploaded and no robot movement has been performed.
