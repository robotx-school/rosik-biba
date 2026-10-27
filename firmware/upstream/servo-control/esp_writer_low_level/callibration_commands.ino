uint16_t min_poses[12];
uint16_t max_poses[12];
uint16_t center_poses[12];

struct SERVO {
    uint16_t min_pos;
    uint16_t max_pos;
    uint16_t center_pos;
};

SERVO servos[256];

void printCallibration(uint8_t id)  {
    SERVO cur = servos[id];
    Serial.printf("max: %i | min: %i | center: %i\n", cur.max_pos, cur.min_pos, cur.center_pos);
}

void handleGetCallib(const String &args) {
    int id = args.toInt();
    if ( id < 0 || id > 255 ) {
        Serial.println("Wrong id");
        return;
    }
    printCallibration(id);
}

void handleSetCallib(const String &args) {
    const char *str = args.c_str();

    char *token = strtok((char*)str, " ");
    if ( token == NULL ) {
        return;
    }
    int id = atoi(token);
    if ( id < 0 || id > 255 ) {
        return;
    }

    token = strtok(NULL, " ");
    if ( token == NULL ) {
        return;
    }
    uint16_t min_pos = atoi(token);

    token = strtok(NULL, " ");
    if ( token == NULL ) {
        return;
    }
    uint16_t max_pos = atoi(token);
    
    token = strtok(NULL, " ");
    if ( token == NULL ) {
        return;
    }
    uint16_t center_pos = atoi(token);

    servos[id].max_pos = max_pos;
    servos[id].min_pos = min_pos;
    servos[id].center_pos = center_pos;

    Serial.printf("Callibration for servo %i set\n", id);
    savePrefernces();
}

void loadPreferences() {
    preferences.begin("poses", true);
    preferences.getBytes("max", max_poses, 24);
    preferences.getBytes("min", min_poses, 24);
    preferences.getBytes("center", center_poses, 24);
    preferences.end();
    preferences.begin("servos", true);
    // Если данные ещё не сохранены, size вернёт 0 – тогда нужно инициализировать дефолтными значениями
    size_t sz = preferences.getBytesLength("servos");
    if (sz == sizeof(servos)) {
        preferences.getBytes("servos", servos, sizeof(servos));
    } else {
        Serial.println("Load previous");
        // Инициализация значениями по умолчанию
        for (int i = 1 ; i < 13; i++) {
            servos[i].min_pos = min_poses[i-1];
            servos[i].max_pos = max_poses[i-1];
            servos[i].center_pos = center_poses[i-1];
        }
    }
    preferences.end();
    Serial.println("Settings loaded");
}

void savePrefernces() {
    preferences.begin("servos", false);
    preferences.putBytes("servos", servos, sizeof(servos));
    preferences.end();
}
 
void handleSetCenter(String &args) {
    uint8_t id = args.toInt();

    int pos = st.ReadPos(id);
    if ( pos == -1 ) {
        Serial.printf("Failed to read pos");
        return;
    }

    servos[id].center_pos = pos;
    Serial.printf("Center pos for servo %i set to %i\n", id, pos);
    savePrefernces();
}

void handleSetMin(String &args) {
    uint8_t id = args.toInt();

    int pos = st.ReadPos(id);
    if ( pos == -1 ) {
        Serial.printf("Failed to read pos");
        return;
    }

    servos[id].min_pos = pos;
    Serial.printf("Min pos for servo %i set to %i\n", id, pos);
    savePrefernces();
}

void handleSetMax(String &args) {
    uint8_t id = args.toInt();

    int pos = st.ReadPos(id);
    if ( pos == -1 ) {
        Serial.printf("Failed to read pos");
        return;
    }

    servos[id].max_pos = pos;
    Serial.printf("Max pos for servo %i set to %i\n", id, pos);
    savePrefernces();
}


uint16_t _constrain(uint16_t pos, uint16_t p1, uint16_t p2) {
    if ( p1 < p2 ) {
        return constrain(pos, p1, p2);
    }
    return constrain(pos, p2, p1);
}

uint16_t _unconstarin(uint16_t pos, uint16_t p1, uint16_t p2) {
    if ( ( pos > p1 && pos < p2 ) ||
         ( pos < p1 && pos > p2 ) ) {
        if ( abs(pos-p1) < abs(pos-p2) ) {
            return p1;
        } else {
            return p2;
        }
    }

    return pos;
}

uint16_t convertSts2Pos(int id, int sts_pos) {
    if  ( sts_pos < 0 || sts_pos > 4095 ) {
        return uint16_t(-1);
    }

    uint16_t min = servos[id].min_pos;
    uint16_t max = servos[id].max_pos;
    uint16_t center = servos[id].center_pos;

    // Если центральная позиция между min и max
    if ( ( center > min && center < max ) ||
         ( center < min && center > max ) ) {
        sts_pos = _constrain(sts_pos, min, max);
        return map(sts_pos, min, max, 0, 10000);
    }

    // Если центральная позиция не между min и max
    if ( min < max ) {
        sts_pos = _unconstarin(sts_pos, min, max);
        if ( sts_pos <= min ) {
            sts_pos += 4096;
        }
        min += 4096;
    } else {
        sts_pos = _unconstarin(sts_pos, max, min);
        if ( sts_pos <= max ) {
            sts_pos += 4096;
        }
        max += 4096;
    }
    return map(sts_pos, min, max, 0, 10000);
}

uint16_t convertPos2Sts(int id, uint16_t pos) {
    pos = constrain(pos, 0, 10000);
    uint16_t min = servos[id].min_pos;
    uint16_t max = servos[id].max_pos;
    uint16_t center = servos[id].center_pos;

    // Если центральная позиция между min и max
    if ( ( center > min && center < max ) ||
         ( center < min && center > max ) ) {
        pos = map(pos, 0, 10000, min, max);
        return pos;
    }

    // Если центральная позиция не между min и max
    if ( min < max ) {
        return map(pos, 0, 10000, min + 4096, max) & 0xFFF;
    } else {
        return map(pos, 0, 10000, min, max + 4096) & 0xFFF;
    }
}