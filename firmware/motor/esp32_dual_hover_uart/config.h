#pragma once

// Relay contacts emulate the motor controller's momentary power switch.
// Use the same 500ms pulse to enable or disable physical motor power.
static constexpr int POWER_RELAY_PIN = 25;
static constexpr bool POWER_RELAY_ACTIVE_HIGH = true;
static constexpr uint32_t POWER_RELAY_PULSE_MS = 500;
