# Implementation record

Plan: 2026-10-27-robot-control.md

- User approved implementation, owns network/tailnet configuration, and requires existing site/camera preserved.
- Work stays in the shared checkout on feat/manipulator-integration; no commits, uploads or network changes.
- Task 1 complete: all 49 original driver files relocated with matching SHA-256 digests. Original site/camera/deployment bytes unchanged.
- Task 2 complete: bounded framing, calibrated 0..10000 poses, serial transactions and command validation implemented. Focused protocol/error/timeout tests pass.
- Task 3 complete: independent authenticated receiver and operator JSON/master-serial controller implemented. Go build and race tests pass.
- Task 4 complete: local stepper replaces Arduino UART lift; merged firmware and copied motor firmware compile with ESP32 core 3.3.12, pinned libraries. Host calibration/parser/lift-state tests pass.
- Task 5 complete: setup, wiring, protocol and operation documented. Independent read-only review found no critical/important failures.
- Review limitation: SCServo SyncWritePosEx returns void; an ACK means acceptance/transmission attempt, not confirmed servo execution. This is explicit in the API and documentation; position queries are available to obtain feedback.
- Evidence: /tmp/rosik-driver-before.json, /tmp/rosik-build-script.log. Physical hardware has not been flashed or exercised.
