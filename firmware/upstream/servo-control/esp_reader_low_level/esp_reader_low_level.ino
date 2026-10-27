/************************************************************
 * FEETECH STS: текстовый контроллер по UART1
 * ---------------------------------------------------------
 * Возможности:
 *
 * 1) Найти сервоприводы на шине
 *        - PING <id>
 *        - SCAN_ID [from] [to]
 *
 * 2) Задать / изменить ID

 *        - SET_ID <oldId> <newId>
 *
 * 4) Управление
 *            - MOVE <id> <pos> <speed> <acc>
 *            - SYNC_MOVE <n> <id1> <pos1> <speed1> <acc1> ...
 *            - READ <id> (считывает откалиброваную позицию)
 *            - READ_RAW <id> (считывает позицию с энкодеров)
 *            - SCAN_POS [from] [to]
 *
 * 5) Управлять усилием и читать параметры
 *        - ENABLE_TORQUE <id>
 *        - DISABLE_TORQUE <id>
 *        - READ_LOAD <id>
 *        - READ_VOLTAGE <id>
 *        - READ_TEMP <id>
 *
 * 6) Калибровка
 *        - CENTER <id> (Устанавливает спеднюю позицию для сервы)
 *        - MAX <id>    (Устанавливает максимальную позицию для сервы)
 *        - MIN <id>    (Устанавливает минимальную позицию для сервы)
 *
 * ---------------------------------------------------------
 * Аппаратные требования (ESP32):
 *    - Шина сервоприводов на UART1 (Serial1)
 *    - Скорость: 1_000_000 бод (SERVO_BAUD)
 *    - Пины: 
 *                 GPIO18 = RX1 → RX платы-переходника,
 *                 GPIO19 = TX1 → TX платы-переходника
 *        ВАЖНО - Тут нет ошибки! Именно так!
 *
 * Библиотека:
 *    - SCServo (класс SMS_STS)
 *
 * Обмен с ПК:
 *    - USB-Serial (Serial) @ 115200 бод
 *    - Одна команда на строку: <COMMAND> [arg1] [arg2] ...
 *        Пример: "MOVE 100 2048 400 50\n"
 *    - Регистр команд не важен (set_id, Set_Id, SET_ID — одинаково).
 *
 * Внимание:
 *    - В рабочей прошивке лучше использовать бинарный протокол и
 *        избегать класса String из-за динамической памяти.
 ************************************************************/

 // Прототипы функций из других файлов
void loadPreferences();
void SetupClient();
void HandleClient();
void handleSerial();
void handlePacket();
void processCommand(String command);


#include <SCServo.h>
#include <Preferences.h>

// Пины UART1 (ESP32) для шины сервоприводов
#define S_RXD 18
#define S_TXD 19

// Пины кнопок
#define BUTTON_UP    32
#define BUTTON_DOWN  25
#define BUTTON_LEFT  33
#define BUTTON_RIGHT 26

#define BUTTON_LIFT_UP   14
#define BUTTON_LIFT_DOWN 27

// Скорость UART для STS-сервоприводов
#define SERVO_BAUD 1000000


// Глобальный объект библиотеки
SMS_STS st;
Preferences preferences;

// ------------------------- SETUP / LOOP -----------------------

void setup() {
    // Глобальный serial
    Serial.begin(115200);
    delay(500);

    pinMode(BUTTON_UP, INPUT_PULLUP);
    pinMode(BUTTON_DOWN, INPUT_PULLUP);
    pinMode(BUTTON_LEFT, INPUT_PULLUP);
    pinMode(BUTTON_RIGHT, INPUT_PULLUP);
    pinMode(BUTTON_LIFT_UP, INPUT_PULLUP);
    pinMode(BUTTON_LIFT_DOWN, INPUT_PULLUP);

    // serial сервоприводов
    Serial1.begin(SERVO_BAUD, SERIAL_8N1, S_RXD, S_TXD);
    st.pSerial = &Serial1;

    loadPreferences();    
}

void loop() {
    handleSerial();
}

