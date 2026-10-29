# ESP32 firmware

## Robot boards

1. `motor/esp32_dual_hover_uart/`: imported from `~/hoverboard-modify`, extended with a momentary power relay. USB motion serial remains `linear_m_s angular_rad_s\n` at 115200 baud. Its original UART pins, wheel directions and watchdog remain intact. The external `~/hoverboard-modify` checkout is not modified.
2. `manipulator/manipulator/`: combined FEETECH STS bus and local GyverStepper lift. No separate Arduino lift board or `Serial2` lift connection is needed.

The sketches build for `esp32:esp32:esp32` with ESP32 Arduino core 3.3.12. Select the correct FQBN if your boards differ. This work does not flash either board.

## Build

With Arduino CLI, Git and the ESP32 core installed:

```sh
./firmware/build.sh
# Optional, if dependency downloads require your existing proxy:
HTTPS_PROXY=http://127.0.0.1:2080 HTTP_PROXY=http://127.0.0.1:2080 ./firmware/build.sh
```

The script fetches pinned SCServo 1.0.2 and GyverStepper 2.7.3 revisions into ignored `firmware/.deps/` and compiles both sketches. It does not install global libraries or modify networking. `FQBN=... ./firmware/build.sh` selects another ESP32 board. The output binaries remain under `.deps/build/`.

## Motor power relay

The driver website's **Toggle motor power** button requests one 500ms pulse on the motor ESP32. Both enable and disable use the same pulse, emulating the existing momentary power switch. Actual on/off state is not sensed; pulse completion must not be interpreted as confirmation of motor power state.

The GPIO defaults to **25**, active high. Configure `POWER_RELAY_PIN` and `POWER_RELAY_ACTIVE_HIGH` in `motor/esp32_dual_hover_uart/config.h` for your relay module. Connect the GPIO to the relay module's logic input and its contacts across the existing power switch circuit. The GPIO drives the module input, not a bare relay coil.

The output starts inactive and never pulses automatically on boot or reconnect. An independent ESP timer task releases the relay after 500ms even if USB logging blocks. Duplicate requests during a pulse are ignored without extending it. Wheel targets are zeroed during the pulse; the UART watchdog continues running.

Serial command: `POWER_PULSE\n` at 115200 baud, exact uppercase token. Firmware feedback is `RELAY READY`, `RELAY PULSE_STARTED`, `RELAY PULSE_DONE`, or `RELAY BUSY`. The Go agent emits each accepted request once, discards expired requests, limits repeated requests, and never retries a failed pulse write. Missing completion feedback is reported as unconfirmed.

Use matching updated driver server, robot agent and motor firmware for the button. Compilation does not update deployed programs or flash the ESP32.

## Manipulator wiring

Edit [config.h](manipulator/manipulator/config.h) to match the actual robot before flashing. These ESP32 GPIO numbers match the supplied robot wiring.

| Signal | ESP32 GPIO |
| --- | --- |
| STS bus adapter RX | GPIO17, UART1 RX |
| STS bus adapter TX | GPIO16, UART1 TX |
| Lift STEP | GPIO13 |
| Lift DIR | GPIO12 |
| Lift ENABLE | GPIO27, active low |
| Upper limit | GPIO33, internal pullup, active low |
| Lower limit | GPIO32, internal pullup, active low |

Both limit switch inputs use internal pullups. Connect each switch between its GPIO and GND; no external pullup is required. Both limits are active low.

Use the existing suitable STS bus adapter and motor driver; servo power does not come from an ESP32 GPIO. Verify the adapter's TX/RX labeling against its documentation and the upstream connection (which explicitly connects matching RX/TX labels). Share the appropriate signal ground. Configure direction so positive step count moves toward the upper switch; `LIFT_REVERSE` changes this. Match `ENABLE_ACTIVE_LOW` and `LIMIT_ACTIVE_LEVEL` to your electronics.

The original motion constants are retained: 50 steps/mm, 8000 steps/s max and 2000 steps/s² acceleration. Homing uses half speed, a maximum travel of `MAX_TRAVEL_MM + 50`, and a 20s timeout. `MAX_TRAVEL_MM` defaults to 600; set the actual measured usable travel. Adjust the homing timeout if your travel/step scale requires it. Both switches pressed is a fault. Endstop stops and watchdog stops brake immediately and retain holding power. Disabling removes holding power and invalidates homing.

## Servo calibration

The robot uses upstream calibrated positions from 0 to 10000. Raw STS encoder ticks are 0..4095. Conversion preserves normal, reversed and encoder-wrapping arcs using min/max/center. It is deliberately performed in firmware, independently for each robot/master servo. The upstream table is kept in `upstream/servo-control/servo_callibration.txt` as a reference, not automatically applied to unknown hardware.

Existing NVS `servos` namespace / `servos` key is loaded with the original 256-record `{u16 min, u16 max, u16 center}` layout. The older `poses` namespace twelve-servo arrays are also migrated in memory. Missing/degenerate calibration makes normalized servo movement fail instead of generating an invalid map/division. Flash normally without erasing NVS to preserve existing calibration.

With the Go receiver stopped, open the robot USB serial at 115200. Text commands are accepted after reset:

```text
PING 1
READ_RAW 1
GET_CALLIB 1
SET_CALLIB 1 773 3470 2715
GET_CALLIB 1
READ 1
```

`SET_CALLIB` is `ID MIN MAX CENTER` in raw ticks. The numeric example is servo 1 in the upstream robot table; only apply it if it matches your mechanism. `MIN ID`, `MAX ID` and `CENTER ID` capture the current encoder position and save it; `SAVE` explicitly saves all records. Partial calibration may be saved while commissioning, but normal moves remain rejected until it is valid. `MOVE ID NORMALIZED_POSITION SPEED ACCEL`, `MOVE_RAW ID RAW_POSITION SPEED ACCEL`, torque enable/disable, `SET_ID OLD NEW`, `SET_SERVO ID`, and voltage/load/temperature reads are supported. Raw moves are manual commissioning operations, so validate their travel yourself.

Text lift commands: `ENABLE_LIFT`, `DISABLE_LIFT`, `CALIBRATE_LIFT` (also upstream misspelling `CALLIBRATE_LIFT`), `SET_LIFT MM`, `STOP_LIFT`, `GET_LIFT_STATUS`, `HEARTBEAT`. Text homing/moves need heartbeats too. Use the Go controller for sustained lift operation; it sends them automatically. Text commissioning that might touch flash/servos is blocked during lift motion.

Go switches to `PACKET_MODE` on open. To return to text without rebooting, send the empty binary `H` command (`3f 24 48 00 00`). USB packets are also recognized directly in text mode. See [wire protocol](../docs/architecture.md).

## Sources

- Servo controller/reader, Python protocol and calibration logic: [kakoy-to-chuvak/rosok-biba-servo-control](https://github.com/kakoy-to-chuvak/rosok-biba-servo-control/tree/7aea53b25808d8cff093cc2452ad245be38a0de3), MIT; original files and license in `upstream/servo-control/`.
- Lift behavior: [Ivan-Linuxoid/INTENSIV_ROBOTX_2026_FILES — 003_stepperTest](https://github.com/Ivan-Linuxoid/INTENSIV_ROBOTX_2026_FILES/blob/main/BERITE_KOD_DLYA_NITENSIVA/003_stepperTest/003_stepperTest.ino); pinned raw source URL in `upstream/stepper/SOURCE.txt`.
- Library dependencies: [SCServo](https://github.com/workloads/scservo/tree/268e338776fc7c2cd2de1ede9d03f6ffcd509946), [GyverStepper](https://github.com/GyverLibs/GyverStepper/tree/e9d0a68833605e87a082a9367877434d6bb8f271).

The combined implementation fixes upstream empty-payload command handling, the extra pose-record count, malformed packet lengths, the lift calibration spelling mismatch, and blocking serial input. Endstops/watchdog now continue ticking independently of servo requests. The unmodified upstream reference sketches are not the combined build targets.

## Checks without hardware

```sh
./firmware/manipulator/tests/run.sh
./firmware/motor/tests/run.sh
(cd driver && node frontend_power_test.cjs)
(cd manipulator && go test -race ./...)
(cd driver && go test ./...)
```

The C++ checks exercise calibration mathematics and the production firmware parser/lift state machine against simulated GPIO, time and a stepper. They do not validate electrical pulse timing, load, real switch wiring or actual servo responses. The Go checks cover Python wire compatibility, invalid inputs, authentication, rejected commands and bounded stalled serial reads.
