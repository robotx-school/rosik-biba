#!/usr/bin/env bash
set -euo pipefail
base="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
build_dir="$(mktemp -d /tmp/rosik-relay-tests.XXXXXX)"
trap 'rm -rf -- "$build_dir"' EXIT
"${CXX:-c++}" -std=c++11 -Wall -Wextra -Werror -I "$base/stubs" "$base/power_relay_test.cpp" -o "$build_dir/power_relay_test"
"$build_dir/power_relay_test"
