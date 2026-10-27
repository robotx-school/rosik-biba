#include <Arduino.h>

static constexpr uint32_t USB_BAUD = 115200;
static constexpr uint32_t HOVER_BAUD = 115200;

static constexpr int BOARD_LEFT_RX_PIN = 16;
static constexpr int BOARD_LEFT_TX_PIN = 17;
static constexpr int BOARD_RIGHT_RX_PIN = 26;
static constexpr int BOARD_RIGHT_TX_PIN = 27;

static constexpr float WHEEL_BASE_WIDTH_M = 0.42f;
static constexpr float WHEEL_RADIUS_M = 0.256f;
static constexpr float TARGET_RPM_MIN = -30.0f;
static constexpr float TARGET_RPM_MAX = 30.0f;
// Feedback reports each controller's two motors separately. The left motors
// can be configured independently; both right motors share one direction.
static constexpr int8_t LEFT_MOTOR1_DIRECTION = 1;
static constexpr int8_t LEFT_MOTOR2_DIRECTION = -1;
static constexpr int8_t RIGHT_MOTOR1_DIRECTION = -1;
static constexpr int8_t RIGHT_MOTOR2_DIRECTION = 1;

static constexpr uint16_t START_FRAME = 0xABCD;
static constexpr uint32_t SEND_INTERVAL_MS = 50;
static constexpr uint32_t REPORT_INTERVAL_MS = 100;
static constexpr uint32_t COMMAND_TIMEOUT_MS = 500;
static constexpr float RAD_S_TO_RPM = 60.0f / (2.0f * PI);

HardwareSerial HoverLeft(2);
HardwareSerial HoverRight(1);

struct __attribute__((packed)) SerialCommand {
  uint16_t start;
  int16_t steer;
  int16_t speed;
  uint16_t checksum;
};

struct __attribute__((packed)) SerialFeedback {
  uint16_t start;
  int16_t cmd1;
  int16_t cmd2;
  int16_t speedR_meas;
  int16_t speedL_meas;
  int16_t batVoltage;
  int16_t boardTemp;
  uint16_t cmdLed;
  uint16_t checksum;
};

struct FeedbackParser {
  SerialFeedback feedback = {};
  SerialFeedback pending = {};
  uint8_t *cursor = reinterpret_cast<uint8_t *>(&pending);
  uint8_t index = 0;
  uint8_t previous = 0;
  bool hasNew = false;
};

static float targetLeftRpm = 0.0f;
static float targetRightRpm = 0.0f;
static String inputLine;
static uint32_t lastCommandMs = 0;
static uint32_t lastSendMs = 0;
static uint32_t lastReportMs = 0;
static FeedbackParser feedbackLeft;
static FeedbackParser feedbackRight;

static float clampTargetRpm(float value) {
  if (value < TARGET_RPM_MIN) return TARGET_RPM_MIN;
  if (value > TARGET_RPM_MAX) return TARGET_RPM_MAX;
  return value;
}

static uint16_t commandChecksum(const SerialCommand &command) {
  return static_cast<uint16_t>(command.start ^ command.steer ^ command.speed);
}

static uint16_t feedbackChecksum(const SerialFeedback &feedback) {
  return static_cast<uint16_t>(
      feedback.start ^ feedback.cmd1 ^ feedback.cmd2 ^ feedback.speedR_meas ^
      feedback.speedL_meas ^ feedback.batVoltage ^ feedback.boardTemp ^ feedback.cmdLed);
}

static void sendCommand(HardwareSerial &port, float motor1TargetRpm,
                        float motor2TargetRpm) {
  int16_t motor1Speed = static_cast<int16_t>(lroundf(clampTargetRpm(motor1TargetRpm)));
  int16_t motor2Speed = static_cast<int16_t>(lroundf(clampTargetRpm(motor2TargetRpm)));

  SerialCommand command;
  command.start = START_FRAME;
  // The board combines speed and steer as motor1 = speed + steer,
  // motor2 = speed - steer. Encode independently reversed wheel targets.
  command.speed = static_cast<int16_t>((motor1Speed + motor2Speed) / 2);
  command.steer = static_cast<int16_t>((motor1Speed - motor2Speed) / 2);
  command.checksum = commandChecksum(command);
  port.write(reinterpret_cast<const uint8_t *>(&command), sizeof(command));
}

static void parseFeedback(HardwareSerial &port, FeedbackParser &parser) {
  while (port.available() > 0) {
    uint8_t incoming = static_cast<uint8_t>(port.read());
    uint16_t frame = (static_cast<uint16_t>(incoming) << 8) | parser.previous;

    if (frame == START_FRAME) {
      parser.pending = {};
      parser.cursor = reinterpret_cast<uint8_t *>(&parser.pending);
      parser.cursor[0] = parser.previous;
      parser.cursor[1] = incoming;
      parser.index = 2;
    } else if (parser.index >= 2 && parser.index < sizeof(SerialFeedback)) {
      parser.cursor[parser.index++] = incoming;
    }

    if (parser.index == sizeof(SerialFeedback)) {
      if (parser.pending.start == START_FRAME &&
          parser.pending.checksum == feedbackChecksum(parser.pending)) {
        parser.feedback = parser.pending;
        parser.hasNew = true;
      }
      parser.index = 0;
    }

    parser.previous = incoming;
  }
}

static void parseUsbCommandLine(String line) {
  line.trim();
  if (line.length() == 0) return;

  char buffer[64];
  line.toCharArray(buffer, sizeof(buffer));

  char *linearToken = strtok(buffer, ", ;\t");
  char *angularToken = strtok(nullptr, ", ;\t");
  if (linearToken == nullptr || angularToken == nullptr) {
    Serial.println("ERR expected: <linear_m_s> <angular_rad_s>");
    return;
  }

  float linear = atof(linearToken);
  float angular = atof(angularToken);

  float leftWheelMps = linear - (angular * WHEEL_BASE_WIDTH_M * 0.5f);
  float rightWheelMps = linear + (angular * WHEEL_BASE_WIDTH_M * 0.5f);

  targetLeftRpm = clampTargetRpm((leftWheelMps / WHEEL_RADIUS_M) * RAD_S_TO_RPM);
  targetRightRpm = clampTargetRpm((rightWheelMps / WHEEL_RADIUS_M) * RAD_S_TO_RPM);
  lastCommandMs = millis();

  Serial.print("OK tgtL:");
  Serial.print(targetLeftRpm, 2);
  Serial.print(" tgtR:");
  Serial.println(targetRightRpm, 2);
}

static void readUsbCommands() {
  while (Serial.available() > 0) {
    char c = static_cast<char>(Serial.read());
    if (c == '\r') continue;
    if (c == '\n') {
      parseUsbCommandLine(inputLine);
      inputLine = "";
      continue;
    }
    if (inputLine.length() < 63) inputLine += c;
  }
}

static int16_t leftMotor1Rpm() {
  return static_cast<int16_t>(feedbackLeft.feedback.speedL_meas * LEFT_MOTOR1_DIRECTION);
}

static int16_t leftMotor2Rpm() {
  return static_cast<int16_t>(feedbackLeft.feedback.speedR_meas * LEFT_MOTOR2_DIRECTION);
}

static int16_t rightMotor1Rpm() {
  return static_cast<int16_t>(feedbackRight.feedback.speedL_meas * RIGHT_MOTOR1_DIRECTION);
}

static int16_t rightMotor2Rpm() {
  return static_cast<int16_t>(feedbackRight.feedback.speedR_meas * RIGHT_MOTOR2_DIRECTION);
}

void setup() {
  Serial.begin(USB_BAUD);
  HoverLeft.begin(HOVER_BAUD, SERIAL_8N1, BOARD_LEFT_RX_PIN, BOARD_LEFT_TX_PIN);
  HoverRight.begin(HOVER_BAUD, SERIAL_8N1, BOARD_RIGHT_RX_PIN, BOARD_RIGHT_TX_PIN);
  lastCommandMs = millis();

  Serial.println("ESP32 dual hoverboard UART bridge ready");
  Serial.println("Send: <linear_m_s> <angular_rad_s>");
}

void loop() {
  uint32_t now = millis();

  readUsbCommands();
  parseFeedback(HoverLeft, feedbackLeft);
  parseFeedback(HoverRight, feedbackRight);

  if (now - lastCommandMs > COMMAND_TIMEOUT_MS) {
    targetLeftRpm = 0.0f;
    targetRightRpm = 0.0f;
  }

  if (now - lastSendMs >= SEND_INTERVAL_MS) {
    sendCommand(HoverLeft, targetLeftRpm * LEFT_MOTOR1_DIRECTION,
                targetLeftRpm * LEFT_MOTOR2_DIRECTION);
    sendCommand(HoverRight, targetRightRpm * RIGHT_MOTOR1_DIRECTION,
                targetRightRpm * RIGHT_MOTOR2_DIRECTION);
    lastSendMs = now;
  }

  if (now - lastReportMs >= REPORT_INTERVAL_MS) {
    Serial.print("L1:");
    Serial.print(leftMotor1Rpm());
    Serial.print(" L2:");
    Serial.print(leftMotor2Rpm());
    Serial.print(" R1:");
    Serial.print(rightMotor1Rpm());
    Serial.print(" R2:");
    Serial.print(rightMotor2Rpm());
    Serial.print(" cmdL:");
    Serial.print(targetLeftRpm, 2);
    Serial.print(" cmdR:");
    Serial.println(targetRightRpm, 2);
    lastReportMs = now;
  }
}
