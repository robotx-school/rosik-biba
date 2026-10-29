# Motor power relay implementation

User request: add one 500ms relay pulse for motor power enable/disable and a frontend button. GPIO25 chosen as a configurable free motor ESP32 pin; active-high by default. The hardware switch toggles actual power, so UI must not claim confirmed on/off state.

1. Add nonblocking firmware relay timing and exact POWER_PULSE serial command. Ignore duplicates while pulsing, zero wheel commands during the pulse, and default output inactive at boot. Add native timing tests (500ms, duplicate request, clock wrap).
2. Extend driver protocol and existing WebSocket control relay with power_pulse requests and power_result status. Preserve camera code and network settings.
3. Add one-shot, fresh-only agent requests through the existing exclusive serial writer. Reject offline/busy input, never retry a pulse after failed write or reconnection, and parse firmware feedback. Test emitted-once, expiration, reconnect clearing and status timeout.
4. Add a driver power button with connection/busy gating and visible pulse feedback. Preserve camera functions and ordinary joystick/gamepad behavior.
5. Document GPIO, polarity, pulse semantics and protocol. Run driver tests/build, host relay tests, ESP32 compile and focused review. No deployment, flashing or hardware operation.

Existing uncommitted manipulator serial timeout fix, deployment notes and AGENTS.md belong to earlier work and must be retained.
