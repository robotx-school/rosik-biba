#pragma once

// Robot ESP32 servo and lift wiring supplied by the operator.
static constexpr int SERVO_RX = 17;
static constexpr int SERVO_TX = 16;
static constexpr int LIFT_STEP = 13;
static constexpr int LIFT_DIR = 12;
static constexpr int LIFT_ENABLE = 27;
static constexpr int LIMIT_TOP = 33;
static constexpr int LIMIT_BOTTOM = 32;
static constexpr bool LIFT_REVERSE = false;
static constexpr bool ENABLE_ACTIVE_LOW = true;
static constexpr int LIMIT_ACTIVE_LEVEL = LOW; // switches to GND; internal pullups on GPIO33 and GPIO32
static constexpr int STEPS_PER_MM = 50;
static constexpr int MAX_TRAVEL_MM = 600; // set to measured usable travel
static constexpr int HOMING_TRAVEL_MM = MAX_TRAVEL_MM + 50;
static constexpr uint32_t HOMING_TIMEOUT_MS = 20000;
static constexpr uint32_t CONTROL_TIMEOUT_MS = 750;
static constexpr uint32_t PACKET_TIMEOUT_MS = 100;
static constexpr int MAX_SPEED = 8000;
static constexpr int ACCELERATION = 2000;
static constexpr int SERVO_TIMEOUT_MS = 10;
static constexpr int MAX_SERVOS = 12;
static constexpr int MAX_PACKET = 256;
static_assert(MAX_TRAVEL_MM > 0 && MAX_TRAVEL_MM <= 60000, "invalid travel");
