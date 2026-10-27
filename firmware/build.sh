#!/usr/bin/env bash
set -euo pipefail
base="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
deps="$base/.deps"
mkdir -p "$deps"
fetch() {
  local name="$1" repo="$2" revision="$3"
  local target="$deps/$name"
  if [[ ! -d "$target" ]]; then
    git init -q "$target"
    git -C "$target" remote add origin "$repo"
    git -C "$target" fetch -q --depth 1 origin "$revision"
    git -C "$target" checkout -q --detach FETCH_HEAD
  fi
  if [[ "$(git -C "$target" rev-parse HEAD)" != "$revision" ]]; then
    echo "Unexpected dependency revision in $target; inspect it before rebuilding" >&2
    exit 1
  fi
}
fetch SCServo https://github.com/workloads/scservo.git 268e338776fc7c2cd2de1ede9d03f6ffcd509946
fetch GyverStepper https://github.com/GyverLibs/GyverStepper.git e9d0a68833605e87a082a9367877434d6bb8f271
board="${FQBN:-esp32:esp32:esp32}"
arduino-cli compile --fqbn "$board" --library "$deps/SCServo" --library "$deps/GyverStepper" --build-path "$deps/build/manipulator" "$base/manipulator/manipulator"
arduino-cli compile --fqbn "$board" --build-path "$deps/build/motor" "$base/motor/esp32_dual_hover_uart"
