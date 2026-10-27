
// =============================================================
//                        1) Поиск сервоприводов: PING и SCAN_ID
// =============================================================

// PING <id> — одиночная проверка, есть ли серво с таким ID
void handlePing(const String &args) {
    int id = args.toInt();
    if (id < 0 || id > 253) {
        Serial.println("ERROR: Invalid ID");
        return;
    }

    int pingRes = st.Ping(id);
    Serial.println(pingRes != -1 ? "OK" : "ERROR: Ping fail");
}

// SCAN_ID [from] [to] — поиск всех сервоприводов на шине
//
// Примеры:
//        - SCAN_ID                                        -> сканировать 0..253
//        - SCAN_ID 100                        -> сканировать 100..253
//        - SCAN_ID 90 110         -> сканировать 90..110
//
void handleScanID(const String &args) {
    int from = 0;
    int to = 253;

    if (args.length()) {
        int sp = args.indexOf(' ');
        if (sp < 0) {
            from = args.toInt();
        } else {
            from = args.substring(0, sp).toInt();
            to = args.substring(sp + 1).toInt();
        }
    }

    if (from < 0) from = 0;
    if (to > 253) to = 253;
    if (to < from) {
        int t = from;
        from = to;
        to = t;
    }

    int foundCount = 0;

    Serial.print("SCAN_ID from ");
    Serial.print(from);
    Serial.print(" to ");
    Serial.println(to);

    for (int id = from; id <= to; id++) {
        delayMicroseconds(300);
        int resp = st.Ping(id);
        if (resp != -1) {
            Serial.print("FOUND ID ");
            Serial.println(resp);
            foundCount++;
            delayMicroseconds(200);
        }
    }

    Serial.print("FOUND_TOTAL ");
    Serial.println(foundCount);
}

// =============================================================
//                        2) Настройка ID: SET_ID <oldId> <newId>
// =============================================================

void handleSetID(const String &args) {
    int spaceIdx = args.indexOf(' ');
    if (spaceIdx < 0) {
        Serial.println("ERROR: SET_ID usage: SET_ID <oldId> <newId>");
        return;
    }

    int oldId = args.substring(0, spaceIdx).toInt();
    int newId = args.substring(spaceIdx + 1).toInt();

    if (oldId < 0 || oldId > 253 || newId < 0 || newId > 253) {
        Serial.println("ERROR: Invalid ID range");
        return;
    }

    st.unLockEprom((uint8_t)oldId);
    bool writeOk = st.writeByte((uint8_t)oldId, SMS_STS_ID, (uint8_t)newId);
    st.LockEprom((uint8_t)newId);

    Serial.println(writeOk ? "OK" : "ERROR: write ID fail");
}

// =============================================================
//                        3) Режимы работы: SET_SERVO / SET_WHEEL / READ_MODE
// =============================================================

// SET_SERVO <id> — позиционный режим (MODE = 0)
void handleSetServo(const String &args) {
    int id = args.toInt();
    if (id < 0 || id > 253) {
        Serial.println("ERROR: invalid ID");
        return;
    }
    int r = st.writeByte((uint8_t)id, SMS_STS_MODE, 0);
    if (r != -1) {
        st.EnableTorque((uint8_t)id, 1);
        Serial.println("OK");
    } else {
        Serial.println("ERROR");
    }
}


// =============================================================
//                        4a) Позиционный режим: MOVE / SYNC_MOVE / READ_POS
// =============================================================

// MOVE <id> <position> <speed> <acc>
void handleMove(const String &args) {
    int sp1 = args.indexOf(' ');
    int sp2 = args.indexOf(' ', sp1 + 1);
    int sp3 = args.indexOf(' ', sp2 + 1);

    if (sp1 < 0 || sp2 < 0 || sp3 < 0) {
        Serial.println("ERROR: MOVE usage: MOVE <id> <pos> <speed> <acc>");
        return;
    }

    int id = args.substring(0, sp1).toInt();
    int pos = args.substring(sp1 + 1, sp2).toInt();
    int speed = args.substring(sp2 + 1, sp3).toInt();
    int acc = args.substring(sp3 + 1).toInt();

    if (id < 0 || id > 253) {
        Serial.println("ERROR: invalid ID");
        return;
    }

    if (pos < 0) pos = 0;
    if (pos > 4095) pos = 4095;
    if (speed < 0) speed = 0;
    if (speed > 4095) speed = 4095;
    if (acc < 0) acc = 0;
    if (acc > 255) acc = 255;

    st.WritePosEx((uint8_t)id, convertPos2Sts(id, pos), (uint16_t)speed, (uint8_t)acc);
    Serial.println("OK");
}

// MOVE <id> <position> <speed> <acc>
void handleMoveRaw(const String &args) {
    int sp1 = args.indexOf(' ');
    int sp2 = args.indexOf(' ', sp1 + 1);
    int sp3 = args.indexOf(' ', sp2 + 1);

    if (sp1 < 0 || sp2 < 0 || sp3 < 0) {
        Serial.println("ERROR: MOVE usage: MOVE <id> <pos> <speed> <acc>");
        return;
    }

    int id = args.substring(0, sp1).toInt();
    int pos = args.substring(sp1 + 1, sp2).toInt();
    int speed = args.substring(sp2 + 1, sp3).toInt();
    int acc = args.substring(sp3 + 1).toInt();

    if (id < 0 || id > 253) {
        Serial.println("ERROR: invalid ID");
        return;
    }

    if (pos < 0) pos = 0;
    if (pos > 4095) pos = 4095;
    if (speed < 0) speed = 0;
    if (speed > 4095) speed = 4095;
    if (acc < 0) acc = 0;
    if (acc > 255) acc = 255;

    st.WritePosEx((uint8_t)id, pos, (uint16_t)speed, (uint8_t)acc);
    Serial.println("OK");
}

// SYNC_MOVE <n> <id pos speed acc> ...
// Пример:
//         SYNC_MOVE 2        100 2048 400 50        101 1024 400 50
//
void handleSyncMove(const String &args) {
    int sp1 = args.indexOf(' ');
    if (sp1 < 0) {
        Serial.println("ERROR: SYNC_MOVE usage");
        return;
    }

    int n = args.substring(0, sp1).toInt();
    if (n < 1 || n > 12) {
        Serial.println("ERROR: n out of range");
        return;
    }

    uint8_t IDs[12];
    int16_t Positions[12];
    uint16_t Speeds[12];
    uint8_t Accs[12];

    String remain = args.substring(sp1 + 1);
    remain.trim();

    for (int i = 0; i < n; i++) {
        int spA = remain.indexOf(' ');
        int spB = remain.indexOf(' ', spA + 1);
        int spC = remain.indexOf(' ', spB + 1);

        if (spA < 0 || spB < 0 || spC < 0) {
            Serial.println("ERROR: not enough arguments for SYNC_MOVE");
            return;
        }

        String idStr = remain.substring(0, spA);
        String posStr = remain.substring(spA + 1, spB);
        String spdStr = remain.substring(spB + 1, spC);

        int spNext = remain.indexOf(' ', spC + 1);
        String accStr;
        if (spNext >= 0) {
            accStr = remain.substring(spC + 1, spNext);
            remain = remain.substring(spNext + 1);
        } else {
            accStr = remain.substring(spC + 1);
            remain = "";
        }
        remain.trim();

        int id = idStr.toInt();
        int pos = posStr.toInt();
        int speed = spdStr.toInt();
        int acc = accStr.toInt();

        if (pos < 0) pos = 0;
        if (pos > 4095) pos = 4095;
        if (speed < 0) speed = 0;
        if (speed > 4095) speed = 4095;
        if (acc < 0) acc = 0;
        if (acc > 255) acc = 255;

        IDs[i] = (uint8_t)id;
        Positions[i] = (int16_t)convertPos2Sts(id, pos);
        Speeds[i] = (uint16_t)speed;
        Accs[i] = (uint8_t)acc;
    }

    st.SyncWritePosEx(IDs, n, Positions, Speeds, Accs);
    Serial.println("OK");
}

// READ <id>
void handleReadPos(const String &args) {
    int id = args.toInt();
    if (id < 0 || id > 253) {
        Serial.println("ERROR: invalid ID");
        return;
    }
    int pos = st.ReadPos(id);
    if (pos != -1) {
        Serial.print("POS ");
        Serial.print(id);
        Serial.print(" ");
        Serial.println(convertSts2Pos(id, pos));
    } else {
        Serial.println("ERROR: read pos fail");
    }
}

// READ_RAW <id>
void handleReadRaw(const String &args) {
    int id = args.toInt();
    if (id < 0 || id > 253) {
        Serial.println("ERROR: invalid ID");
        return;
    }
    int pos = st.ReadPos(id);
    if (pos != -1) {
        Serial.print("POS ");
        Serial.print(id);
        Serial.print(" ");
        Serial.println(pos);
    } else {
        Serial.println("ERROR: read pos fail");
    }
}

// SCAN_POS [from] [to] — читаем позиции всех ответивших в диапазоне
void handleScanPos(const String &args) {
    int from = 0;
    int to = 253;

    if (args.length()) {
        int sp = args.indexOf(' ');
        if (sp < 0) {
            from = args.toInt();
        } else {
            from = args.substring(0, sp).toInt();
            to = args.substring(sp + 1).toInt();
        }
    }

    if (from < 0) from = 0;
    if (to > 253) to = 253;
    if (to < from) {
        int t = from;
        from = to;
        to = t;
    }

    for (int id = from; id <= to; id++) {
        int pos = st.ReadPos(id);
        if (pos >= 0) {
            Serial.print("ID ");
            Serial.print(id);
            Serial.print(" POS ");
            Serial.println(convertSts2Pos(id, pos));
        }
        delayMicroseconds(300);
    }
    Serial.println("DONE");
}

// =============================================================
//                        5) Усилие и телеметрия: torque, load, voltage, temp
// =============================================================

// ENABLE_TORQUE <id>
void handleEnableTorque(const String &args) {
    int id = args.toInt();
    if (id < 0 || id > 253) {
        Serial.println("ERROR: Invalid ID");
        return;
    }
    int res = st.EnableTorque((uint8_t)id, 1);
    Serial.println(res != -1 ? "OK" : "ERROR: enable torque fail");
}

// DISABLE_TORQUE <id>
void handleDisableTorque(const String &args) {
    int id = args.toInt();
    if (id < 0 || id > 253) {
        Serial.println("ERROR: Invalid ID");
        return;
    }
    int res = st.EnableTorque((uint8_t)id, 0);
    Serial.println(res != -1 ? "OK" : "ERROR: disable torque fail");
}

// READ_LOAD <id>
void handleReadLoad(const String &args) {
    int id = args.toInt();
    if (id < 0 || id > 253) {
        Serial.println("ERROR: invalid ID");
        return;
    }
    int load = st.ReadLoad(id);
    if (load != -1) {
        Serial.print("LOAD ");
        Serial.print(id);
        Serial.print(" ");
        Serial.println(load);
    } else {
        Serial.println("ERROR: read load fail");
    }
}

// READ_VOLTAGE <id>
void handleReadVoltage(const String &args) {
    int id = args.toInt();
    if (id < 0 || id > 253) {
        Serial.println("ERROR: invalid ID");
        return;
    }
    int volt = st.ReadVoltage(id);
    if (volt != -1) {
        Serial.print("VOLTAGE ");
        Serial.println(volt);
    } else {
        Serial.println("ERROR: read voltage fail");
    }
}

// READ_TEMP <id>
void handleReadTemp(const String &args) {
    int id = args.toInt();
    if (id < 0 || id > 253) {
        Serial.println("ERROR: invalid ID");
        return;
    }
    int temp = st.ReadTemper(id);
    if (temp != -1) {
        Serial.print("TEMP ");
        Serial.println(temp);
    } else {
        Serial.println("ERROR: read temperature fail");
    }
}

void handleSetLift(const String &args) {
    int height = args.toInt();
    Serial2.println(height);
    Serial.printf("Move lift to %i mm\n", height);
    // Дичь по сериал
}










