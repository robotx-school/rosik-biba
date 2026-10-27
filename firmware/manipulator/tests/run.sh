#!/usr/bin/env bash
set -euo pipefail
base="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
build_dir="$(mktemp -d /tmp/rosik-firmware-tests.XXXXXX)"
trap 'rm -rf -- "$build_dir"' EXIT
for test_name in calibration_test lift_state_test packet_parser_test; do
  "${CXX:-c++}" -std=c++11 -Wall -Wextra -Werror -I "$base/stubs" "$base/$test_name.cpp" -o "$build_dir/$test_name"
  "$build_dir/$test_name"
done
