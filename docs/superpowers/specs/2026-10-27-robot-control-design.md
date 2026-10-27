# Robot control integration

## Intent and agreed constraints

Use two ESP32s on the robot: the existing hoverboard motor bridge and a combined
servo/stepper controller. Run the existing website on the robot Raspberry Pi,
reachable through the tailnet. Rewrite the Python manipulator controller in Go.
Preserve the driver website and camera stream implementation and behavior.
The requested relocation of their files is permitted; changing those features is not.

## Approaches

1. Host on the RPi, with independent serial devices for motors and manipulator.
   The user selected hosting directly on the RPi. Keep the existing driver server
   and agent running on that same machine, connected through loopback.
2. Retain a separate hosted server and outbound RPi agent. This matches the
   existing deployment but adds a network hop and was not selected.

Following the user's site/camera constraint, the proposed manipulator receiver
runs as a separate Go process on the RPi. This avoids changing the existing
server, agent, frontend, and camera code.

## Repository layout

- `driver/`: all existing project application files, Go module, templates,
  deployment files, environment files, existing artifacts and ancillary files.
  Preserve relative paths within this directory and preserve file contents.
- `manipulator/`: independent Go module containing serial packet handling,
  an RPi receiver command, and an operator controller command replacing Python.
- `firmware/motor/esp32_dual_hover_uart/`: copy of the existing motor sketch
  from `~/hoverboard-modify`; do not modify that external checkout.
- `firmware/manipulator/`: combined ESP32 servo and lift sketch and configuration.
- `docs/`: architecture, wiring, protocol, and run instructions.
- root README: entry point linking the components and their build instructions.
- repository/environment metadata stays at the root.

## Data flow

Driver browser -> tailnet -> existing RPi website -> existing local robot agent
-> motor ESP32 USB serial. Existing camera signaling and video path are preserved.

Operator positions -> Go controller -> authenticated HTTP over tailnet
-> Go RPi manipulator receiver -> manipulator ESP32 USB serial
-> FEETECH STS bus and step/direction lift driver.

The Go controller accepts position batches from JSON input. Also provide serial
reading of the existing master controller's position packets so an operator can
forward measured poses. Preserve the upstream normalized 0..10000 position scale and firmware
calibration conversion, including encoder wrap. Complete upstream sources
retrieved during implementation established this mapping.

## Manipulator protocol and firmware

Preserve the Python controller's binary framing: `?$`, one type byte, unsigned
little-endian 16-bit payload length, then payload. Retain its servo position read
and write operations and lift set/enable/disable/calibrate commands. Validate
lengths, IDs, ranges, and finite input values before issuing hardware commands.
Serialize serial transactions and bound reads, network requests, and packet sizes.
Report errors to the caller; never report a write as confirmed physical motion.

Use the supplied robot wiring: ESP32 GPIO17 RX / GPIO16 TX at 1,000,000 baud, with
115,200 baud USB serial. Move the GyverStepper lift logic onto this ESP32;
it must no longer require the writer firmware's separate Arduino lift connection.
Use configurable named GPIO settings: upper switch GPIO33, lower switch GPIO32,
enable GPIO27, direction GPIO12, step GPIO13. Both switches use INPUT_PULLUP
and connect to GND when pressed. Both remain active low. These assignments supersede the original proposed GPIO defaults.

Retain 50 steps/mm, 8,000 steps/s maximum, 2,000 steps/s squared acceleration,
and the upper endstop as the lift zero. Positive commanded mm moves down from
that zero. Implement nonblocking serial parsing so stepper ticking continues
while packets arrive. Check directional endstops continuously; bound calibration
travel and duration and stop lift motion on communications timeout. Do not home
or start movement automatically on boot or reconnect. Require calibration before
absolute lift positioning. Preserve servo calibration support from upstream
where available; keep any missing upstream files explicitly documented.

## Deployment and validation

Document separate stable serial paths for each robot ESP32, loopback settings
for the existing driver server/agent, application listen address and token settings for the
manipulator, firmware dependencies, Arduino CLI commands, and the optional
`127.0.0.1:2080` proxy. Do not flash hardware or start motion as part of this work.
Build Go commands and compile ESP32 firmware where dependencies are available;
report unavailable toolchains/dependencies and distinguish compilation from
hardware validation. Preserve existing tests when relocating the project.

## Implementation choices proposed for approval

A separate manipulator HTTP service, JSON operator input plus optional master
serial input, and configurable lift GPIO defaults are implementation proposals.
They have not yet been confirmed as existing hardware or operator behavior.

User approved implementation and owns all network/tailnet/local network configuration. Do not change network setup or existing deployment settings.
