#define HUMAN_MODE 0
#define PACKET_MODE 1

byte serial_mode = HUMAN_MODE;

void handlePacketData(uint8_t *buffer, uint8_t data_type, uint16_t data_len);

void handleSerial() {
    if ( Serial.available() ) {
        if ( serial_mode == HUMAN_MODE ) {
            String cmd = Serial.readStringUntil('\n');
            cmd.trim();
            if (cmd.length() > 0) {
                processCommand(cmd);
            }
        } else {
            handlePacket();
        }
    }
}

// =============================================================
//                                                ПАРСЕР ПАКЕТОВ
// =============================================================
enum PACKET_STATE {
    PKT_WAIT_HEADER,
    PKT_WAIT_TYPE,
    PKT_WAIT_LEN,
    PKT_WAIT_LEN_HIGH,   // добавить
    PKT_WAIT_DATA,
};

#define PACKET_SET_POSES 0x10
#define PACKET_GET_POSES 0x11
#define PACKET_SEND_POSES 0x12

#define PACKET_GET_BUTTONS 0x20
#define PACKET_SEND_BUTTONS 0x21
#define PACKET_HUMAN_MODE ((uint8_t)'H')

const uint8_t PACKET_HEADER[] = {'?', '$'};

void handlePacket() {
    static uint8_t packet_type = 0;
    static uint16_t packet_len = 0;
    static enum PACKET_STATE packet_state = PKT_WAIT_HEADER;
    static uint8_t header_state = 0;          // явная инициализация
    static uint8_t buffer[256];
    static uint16_t data_pos = 0;

    while (Serial.available()) {
        uint8_t b = Serial.read();

        switch (packet_state) {
            case PKT_WAIT_HEADER:
                if (b == PACKET_HEADER[header_state]) {
                    header_state++;
                } else {
                    header_state = 0;
                }
                if (header_state >= sizeof(PACKET_HEADER)) {
                    header_state = 0;
                    packet_state = PKT_WAIT_TYPE;
                }
                break;

            case PKT_WAIT_TYPE:
                packet_type = b;
                if ( packet_type == 'H' ) {
                    serial_mode = HUMAN_MODE;
                }
                packet_state = PKT_WAIT_LEN;
                break;

            case PKT_WAIT_LEN:
                packet_len = b;
                packet_state = PKT_WAIT_LEN_HIGH;
                break;

            case PKT_WAIT_LEN_HIGH:
                packet_len |= (b << 8);
                if (packet_len > 0 && packet_len <= sizeof(buffer)) {
                    data_pos = 0;
                    packet_state = PKT_WAIT_DATA;
                } else {
                    packet_state = PKT_WAIT_HEADER;
                }
                break;

            case PKT_WAIT_DATA:
                buffer[data_pos++] = b;
                if (data_pos == packet_len) {
                    handlePacketData(buffer, packet_type, packet_len);
                    packet_state = PKT_WAIT_HEADER;
                }
                break;
        }
    }
}

void handlePacketData(uint8_t *buffer, uint8_t data_type, uint16_t data_len) {
    uint8_t send_buffer[256];
    uint16_t offset = 0;

    if (data_type == PACKET_GET_POSES) {
        // Заголовок ответа
        send_buffer[offset++] = PACKET_HEADER[0];
        send_buffer[offset++] = PACKET_HEADER[1];
        send_buffer[offset++] = PACKET_SEND_POSES;
        uint16_t resp_len = data_len * 2;
        send_buffer[offset++] = resp_len & 0xFF;
        send_buffer[offset++] = (resp_len >> 8) & 0xFF;

        // Для каждого ID из запроса читаем позицию
        for (int i = 0; i < data_len; i++) {
            uint8_t id = buffer[i];
            int sts_pos = st.ReadPos(id);
            uint16_t pos = convertSts2Pos(id, sts_pos);
            send_buffer[offset++] = pos & 0xFF;
            send_buffer[offset++] = (pos >> 8) & 0xFF;
        }
        Serial.write(send_buffer, offset);
    } else if (data_type == PACKET_SET_POSES) {
        int off = 0;
        uint16_t speed = buffer[off] | (buffer[off+1] << 8); off += 2;
        uint16_t accel = buffer[off] | (buffer[off+1] << 8); off += 2;
        for (int i = 0; i < data_len / 3; i++) {
            uint8_t id = buffer[off++];
            uint16_t pos = buffer[off] | (buffer[off+1] << 8); off += 2;
            st.WritePosEx(id, pos, speed, (uint8_t)accel);
        }
    } else if ( data_type == PACKET_GET_BUTTONS ) {
        send_buffer[offset++] = PACKET_HEADER[0];
        send_buffer[offset++] = PACKET_HEADER[1];
        send_buffer[offset++] = PACKET_SEND_BUTTONS;
        uint16_t resp_len = 6;
        send_buffer[offset++] = resp_len & 0xFF;
        send_buffer[offset++] = (resp_len >> 8) & 0xFF;

        send_buffer[offset++] = !digitalRead(BUTTON_UP);
        send_buffer[offset++] = !digitalRead(BUTTON_DOWN);
        send_buffer[offset++] = !digitalRead(BUTTON_LEFT);
        send_buffer[offset++] = !digitalRead(BUTTON_RIGHT);
        send_buffer[offset++] = !digitalRead(BUTTON_LIFT_UP);
        send_buffer[offset++] = !digitalRead(BUTTON_LIFT_DOWN);
        Serial.write(send_buffer, offset);
    }
}

// =============================================================
//                                                ПАРСЕР КОМАНД
// =============================================================

void processCommand(String command) {
    // Разделяем на имя команды и аргументы (после первого пробела)
    int firstSpaceIndex = command.indexOf(' ');
    String cmdName = command;
    String args = "";

    if (firstSpaceIndex > 0) {
        cmdName = command.substring(0, firstSpaceIndex);
        args        = command.substring(firstSpaceIndex + 1);
        args.trim();
    }

    cmdName.toUpperCase(); // игнорируем регистр

    // 1. Поиск сервоприводов
    if      (cmdName == "PING")             { handlePing(args); }
    else if (cmdName == "SCAN_ID")          { handleScanID(args); }

    // 2. Настройка ID
    else if (cmdName == "SET_ID")           { handleSetID(args); }

    // 3. Режимы работы
    else if (cmdName == "SET_SERVO")        { handleSetServo(args); }

    // 4a. Управление в позиционном режиме
    else if (cmdName == "MOVE")             { handleMove(args); }
    else if (cmdName == "MOVE_ALL")         { }
    else if (cmdName == "MOVE_RAW")         { handleMoveRaw(args); }
    else if (cmdName == "SYNC_MOVE")        { handleSyncMove(args); }
    else if (cmdName == "READ")             { handleReadPos(args); }
    else if (cmdName == "READ_RAW")         { handleReadRaw(args); }
    else if (cmdName == "SCAN_POS")         { handleScanPos(args); }

    // 5. Усилие и телеметрия
    else if (cmdName == "ENABLE_TORQUE")    { handleEnableTorque(args); }
    else if (cmdName == "DISABLE_TORQUE")   { handleDisableTorque(args); }
    else if (cmdName == "READ_LOAD")        { handleReadLoad(args); }
    else if (cmdName == "READ_VOLTAGE")     { handleReadVoltage(args); }
    else if (cmdName == "READ_TEMP")        { handleReadTemp(args); }
    
    // Калибровка
    else if (cmdName == "CENTER")           { handleSetCenter(args); }
    else if (cmdName == "MIN")              { handleSetMin(args); }
    else if (cmdName == "MAX")              { handleSetMax(args); }
    else if (cmdName == "SAVE")             { savePrefernces(); }
    else if (cmdName == "GET_CALLIB")       { handleGetCallib(args); }
    else if (cmdName == "SET_CALLIB")       { handleSetCallib(args); }

    else if (cmdName == "GET_BUTTONS")      { handleGetButtons(args); }

    else if (cmdName == "PACKET_MODE")      { serial_mode = PACKET_MODE; }

    else {
        Serial.println("ERROR: Unknown command");
    }
}