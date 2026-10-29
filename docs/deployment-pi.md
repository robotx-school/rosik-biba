# Raspberry Pi deployment

Deployed to `pi@192.168.0.66` (ARM64 Ubuntu 24.04) on 2026-10-27.

## Active processes

- Existing `bibika-server.service`: original `/home/pi/bibika-server/bibika-agent` binary, existing server endpoint, tokens and camera encoding retained. Only the camera device is overridden to the connected camera's stable `by-id` path.
- `bibika_manip.service`: the Go receiver replaces the old Python server through `/etc/systemd/system/bibika_manip.service.d/20-go-receiver.conf`. The base Python unit and script are retained for rollback. Service is enabled at boot.
- New release: `/home/pi/rosik-biba/current` points to `releases/2f4368a-serialfix`. Both Go manipulator programs, the existing driver binaries, complete source archive and hash manifest are installed there. Driver binaries in this release are staged; the running agent retains its original binary.

## Connected devices

| Function | Device | Persistent configured path |
| --- | --- | --- |
| Motors (confirmed by operator) | `/dev/ttyUSB1` | `/dev/serial/by-path/platform-xhci-hcd.0-usb-0:2:1.0-port0` |
| Manipulator | `/dev/ttyUSB0` | `/dev/serial/by-path/platform-xhci-hcd.1-usb-0:2:1.0-port0` |
| Camera | `/dev/video0` | `/dev/v4l/by-id/usb-4K_USB_Camera_4K_USB_Camera_01.00.00-video-index0` |

Both ESP32 USB bridges report the same serial number `0001`, so their shared `by-id` alias cannot distinguish them. The serial paths depend on retaining these USB socket assignments. The camera override is `/etc/systemd/system/bibika-server.service.d/20-camera-device.conf`. Network, tailnet and routing configuration were not changed.

## Manipulator endpoint and operator

The receiver listens at `http://192.168.0.66:8070` (application bind `0.0.0.0:8070`). Its token and device settings are in `/home/pi/rosik-biba/config/manipulator.env`, mode 0600. A copy for the local operator is in ignored `manipulator/.env`, also mode 0600. Do not commit these environment files.

From this checkout:

```sh
set -a
. manipulator/.env
set +a
manipulator/bin/controller -url http://192.168.0.66:8070
```

The local controller binary is built in ignored `manipulator/bin/`. You can substitute your own reachable address without changing the services' routing or tailnet setup.

## Verification and remaining hardware work

The HTTP health endpoint returns 200 and the new service is active/enabled. The camera publisher is running against the corrected camera device. Existing agent executable, environment file and base systemd unit were hash-checked against pre-deployment backups and remain identical.

A read-only lift status request did **not** receive an ESP32 reply. It now returns HTTP 503 after the configured three-second deadline. No firmware was flashed, lift homing started or movement commands sent. End-to-end manipulator operation is not yet verified; check the ESP32 connection and whether it runs the merged firmware before operation.

Hardware testing exposed CP210x kernel `close(2)` blocking while a serial reply was absent. The source includes a deployment fix: Linux uses nonblocking serial I/O with OS deadlines, cancels reads using deadlines, and closes failed devices asynchronously. Until that kernel close finishes, the port is quarantined and further commands fail instead of reopening it or replaying targets. Regression tests cover a driver whose Close blocks, plus real PTY timeout handling. This patch is recorded in the release source archive and manifest in addition to base commit `2f4368a`.

## Backups and service inspection

Pre-deployment configuration backup: `/home/pi/rosik-biba/backups/20261027-184007`. The path is also recorded in `/home/pi/rosik-biba/last-backup`.

```sh
systemctl status bibika_manip.service bibika-server.service
journalctl -u bibika_manip.service -n 30 --no-pager
curl http://127.0.0.1:8070/health
```

Rollback uses the retained original units and backups: remove only the new named drop-ins and restore the prior release link as needed, then reload systemd and restart the affected service. The legacy Python manipulator automatically enables the lift on startup, so account for that behavior before choosing to run it again.

## Motor power button deployment

The UI server and Pi agent were updated on 2026-10-27 for the relay button. The live UI at `http://100.228.47.107:31415/` serves **Toggle motor power** below the joystick. The server remains in the existing local Docker container `rosik-biba-server-1`, with the same environment, port bindings and network mode; the updated image is tagged `rosik-biba-server:latest`. The previous image is `rosik-biba-server:pre-power-relay-20261027`, with its binary/configuration fingerprints backed up in ignored `driver/dist/relay-backups/20261027-relay/`.

The Pi runs the updated `/home/pi/bibika-server/bibika-agent` through the existing service and camera override. Its previous agent binary and configuration are backed up in `/home/pi/rosik-biba/backups/relay-20261027-192200/`. Camera publisher operation and agent reconnection were verified after restart. Network/tailnet settings were not changed.

Following explicit user authorization, the motor ESP32 on `/dev/ttyUSB1` was flashed with GPIO25 active high for a 500ms pulse. Firmware artifacts, manifest, programming log and successful status are at `/home/pi/rosik-biba/releases/power-relay/firmware/motor/`. Esptool verified all four written images. The original affected flash range `0x00000000..0x0005ffff` is backed up at `/home/pi/rosik-biba/backups/motor-before-relay-20261027/flash.bin`, with its hash and covered range recorded beside it. Other flash sectors were not erased or written. Transfers at 921600 and 460800 baud failed before programming; backup and programming succeeded at 115200 baud. The updated agent is online, its motor serial connection is confirmed, and the camera publisher is running. No relay pulse or movement test was performed.
