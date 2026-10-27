#include <GyverStepper.h>

GStepper<STEPPER2WIRE> stepper1(800, A3, A2, A1); // конфигурация шагового двигателя

#define LIMIT_BOTTOM 4 // D4 — пин нижнего концевика
#define LIMIT_TOP    5  // D5 — пин верхнего концевика
const int STEPS_PER_MM = 50; // шагов на 1 мм

bool calibrating = false; // флаг калибровки
int calibrationOffset = 0; // смещение калибровки (шаги)

int originalSpeed; // для сохранения исходной скорости

void setup() {
  Serial.begin(115200);
  Serial.println(F("Команды: getpos, enable, disable, calibration, число (мм), 0 (в нулевую позицию)"));
  
  pinMode(LIMIT_BOTTOM, INPUT_PULLUP); // инициализация концевиков
  pinMode(LIMIT_TOP, INPUT_PULLUP);

  stepper1.setRunMode(FOLLOW_POS); // режим следования за позицией

  originalSpeed = 8000;
  stepper1.setMaxSpeed(originalSpeed); // макс. скорость
  stepper1.setAcceleration(2000); // ускорение

  // Драйвер изначально отключён
  stepper1.autoPower(true); // разрешить автоотключение
  stepper1.disable();
}

void loop() {
  if (Serial.available() > 0) {
    char input[32] = {0}; // буфер для команды
    Serial.readBytesUntil('\n', input, sizeof(input) - 1);
    input[sizeof(input) - 1] = 0; // null-terminate

    // Приведение строки к нижнему регистру
    for (char *p = input; *p; p++) *p = tolower(*p);

    if (strcmp(input, "getpos") == 0) { // получить текущую позицию
      int curSteps = stepper1.getCurrent();
      float curMm = (curSteps - calibrationOffset) / (float)STEPS_PER_MM;
      Serial.print(F("Позиция: "));
      Serial.print(curSteps - calibrationOffset);
      Serial.print(F(" шагов ("));
      Serial.print(curMm, 2);
      Serial.println(F(") мм"));
      return;
    }

    if (strcmp(input, "calibration") == 0) { // калибровка
      calibrating = true;
      stepper1.setMaxSpeed(originalSpeed / 2); // устанавливаем половину
      stepper1.autoPower(false); // запретить автоотключение
      stepper1.enable(); // включить драйвер
      Serial.println(F("Калибровка: движение к верхнему концевику (половинная скорость)..."));
      stepper1.setTarget(100000, ABSOLUTE); // движение вверх
      return;
    }

    if (strcmp(input, "enable") == 0) { // включить драйвер (опционально)
      stepper1.autoPower(false);
      stepper1.enable();
      Serial.println(F("Драйвер включён"));
      return;
    }

    if (strcmp(input, "disable") == 0) { // отключить драйвер
      stepper1.autoPower(true);
      stepper1.disable();
      Serial.println(F("Драйвер отключён"));
      return;
    }

    int mm = atoi(input); // обработка числовой команды (мм)
    mm = -mm; // ИНВЕРТИРУЕМ ЗНАЧЕНИЕ
    if (mm != 0) {
      int absoluteSteps = mm * STEPS_PER_MM;
      int target = calibrationOffset + absoluteSteps;

      bool topPressed = digitalRead(LIMIT_TOP) == LOW;
      bool bottomPressed = digitalRead(LIMIT_BOTTOM) == LOW;

      // Проверка на столкновение с концевиками
      if (absoluteSteps > 0 && topPressed) {
        Serial.println(F("Запрещено: верхний концевик"));
        return;
      }
      if (absoluteSteps < 0 && bottomPressed) {
        Serial.println(F("Запрещено: нижний концевик"));
        return;
      }

      stepper1.setTarget(target, ABSOLUTE);
      Serial.print(F("Движение на "));
      Serial.print(mm);
      Serial.println(F(" мм"));
    } else { // команда "0"
      if (stepper1.getCurrent() == calibrationOffset) {
        Serial.println(F("Уже в нулевой позиции"));
      } else {
        stepper1.setTarget(calibrationOffset, ABSOLUTE);
        Serial.println(F("Движение в нулевую позицию (0 мм)"));
      }
    }
  }

  // Обработка калибровки
  if (calibrating) {
    bool topPressed = digitalRead(LIMIT_TOP) == LOW; // проверка верхнего концевика

    if (topPressed && stepper1.getState() == 0) { // концевик сработал, двигатель остановился
      calibrationOffset = stepper1.getCurrent(); // текущая позиция = верхний концевик → новая 0
      calibrating = false;
      stepper1.setMaxSpeed(originalSpeed); // восстанавливаем исходную скорость
      stepper1.setAcceleration(2000);
      stepper1.autoPower(false);
      stepper1.enable();
      Serial.println(F("Калибровка завершена (0 = позиция верхнего концевика)"));
    }
  }

  // Защита от превышения концевиков во время движения
  if (stepper1.getState() != 0) {
    bool movingPositive = stepper1.getTarget() > stepper1.getCurrent();
    bool topPressed = digitalRead(LIMIT_TOP) == LOW;
    bool bottomPressed = digitalRead(LIMIT_BOTTOM) == LOW;

    if (movingPositive && topPressed) { // движение вверх, сработал верхний
      stepper1.stop();
      stepper1.setTarget(stepper1.getCurrent(), ABSOLUTE);
      Serial.println(F("Стоп: верхний концевик"));
    } else if (!movingPositive && bottomPressed) { // движение вниз, сработал нижний
      stepper1.stop();
      stepper1.setTarget(stepper1.getCurrent(), ABSOLUTE);
      Serial.println(F("Стоп: нижний концевик"));
    }
  }

  stepper1.tick(); // обновление шагового драйвера
}
