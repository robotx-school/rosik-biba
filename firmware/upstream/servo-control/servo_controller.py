import serial
import time
import struct



class ServoController:
    PACKET_HEADER = b'?$'
    PACKET_SET_POSES = 0x10
    PACKET_GET_POSES = 0x11
    PACKET_SEND_POSES = 0x12

    PACKET_GET_BUTTONS = 0x20
    PACKET_SEND_BUTTONS = 0x21

    PACKET_SET_LIFT = 0x30
    PACKET_ENABLE_LIFT = 0x31
    PACKET_DISABLE_LIFT = 0x32  
    PACKET_CALLIBRATE_LIFT = 0x33
    PACKET_HUMAN_MODE = ord('H')

    def __init__(self):
        self.serial = None
        self.error = ""
        self.last_error = ""
        self._state = 'WAIT_HEADER'
        self._header_pos = 0
        self._packet_type = 0
        self._packet_len = 0
        self._packet_data = bytearray()
        self._lock = threading.Lock()

    def connect(self, port: str, timeout: float = 0.05) -> bool:
        try:
            self.serial = serial.Serial(port=port, baudrate=DEFAULT_BAUD, timeout=timeout)
            self.serial.reset_input_buffer()
            self.serial.reset_output_buffer()
            time.sleep(0.1)
            self.serial.write(b'PACKET_MODE\n')
            time.sleep(0.1)
            self.serial.reset_input_buffer()
            self.last_error = ""
            return True
        except serial.SerialException as e:
            self.error = str(e)
            self.serial = None
            return False

    def is_connected(self) -> bool:
        return self.serial is not None and self.serial.is_open

    def _write_packet(self, packet_type: int, data: bytes = bytes()) -> bool:
        if not self.is_connected():
            return False
        with self._lock:
            try:
                packet = self.PACKET_HEADER
                packet += struct.pack("<B", packet_type)
                packet += struct.pack("<H", len(data))
                packet += data
                self.serial.write(packet)
                return True
            except serial.SerialException as e:
                self.last_error = str(e)
                return False

    def _read_packet(self, timeout: float = 0.01):
        if not self.is_connected():
            return None
        end_time = time.time() + timeout
        while time.time() < end_time:
            try:
                b = self.serial.read(1)
                if not b:
                    continue
                byte = b[0]
                if self._state == 'WAIT_HEADER':
                    if byte == self.PACKET_HEADER[0]:
                        self._header_pos = 1
                        self._state = 'WAIT_HEADER2'
                    else:
                        self._header_pos = 0
                elif self._state == 'WAIT_HEADER2':
                    if byte == self.PACKET_HEADER[1]:
                        self._state = 'WAIT_TYPE'
                    else:
                        self._state = 'WAIT_HEADER'
                        self._header_pos = 0
                elif self._state == 'WAIT_TYPE':
                    self._packet_type = byte
                    self._state = 'WAIT_LEN_LOW'
                elif self._state == 'WAIT_LEN_LOW':
                    self._packet_len = byte
                    self._state = 'WAIT_LEN_HIGH'
                elif self._state == 'WAIT_LEN_HIGH':
                    self._packet_len |= (byte << 8)
                    if 0 < self._packet_len < 512:
                        self._packet_data = bytearray()
                        self._state = 'WAIT_DATA'
                    else:
                        self._state = 'WAIT_HEADER'
                        self._header_pos = 0
                elif self._state == 'WAIT_DATA':
                    self._packet_data.append(byte)
                    if len(self._packet_data) == self._packet_len:
                        result = (self._packet_type, bytes(self._packet_data))
                        self._state = 'WAIT_HEADER'
                        self._header_pos = 0
                        return result
            except serial.SerialException as e:
                self.last_error = str(e)
                return None
        return None

    def read_pos_batch(self, servo_ids: list[int]) -> list[int] | None:
        data = bytes(servo_ids)
        if not self._write_packet(self.PACKET_GET_POSES, data):
            return None
        packet = self._read_packet()
        while packet is None:
            packet = self._read_packet()
        if packet and packet[0] == self.PACKET_SEND_POSES:
            data = packet[1]
            if len(data) >= len(servo_ids) * 2:
                positions = []
                for i in range(len(servo_ids)):
                    pos = struct.unpack("<H", data[i*2:(i+1)*2])[0]
                    positions.append(pos)
                return positions
        return None

    def write_pos_batch(self, ids: list[int], poses: list[int], speed: int, accel: int):
        if len(poses) != len(ids):
            return
        
        data = struct.pack("<HH", speed, accel)
        for id, pos in zip(ids, poses):
            data += struct.pack("<BH", id, pos)
        self._write_packet(self.PACKET_SET_POSES, data)
    
    def read_buttons(self) -> list[bool]:
        self._write_packet(self.PACKET_GET_BUTTONS, b'd')

        packet = self._read_packet()
        while packet is None:
            # print("no packet")
            packet = self._read_packet()

        if packet[0] == self.PACKET_SEND_BUTTONS:
            data = packet[1]
            if len(data) != 6:
                    return None
            return struct.unpack("<BBBBBB", data)  
        
    def set_lift(self, height):
        height = min(60000, max(0, height))
        self._write_packet(self.PACKET_SET_LIFT, struct.pack("<H", height))

    def enable_lift(self):
        self._write_packet(self.PACKET_ENABLE_LIFT)

    def callibrate_lift(self):
        self._write_packet(self.PACKET_CALLIBRATE_LIFT)
    
    def disable_lift(self):
        self._write_packet(self.PACKET_DISABLE_LIFT)




def send_pos_tcp(positions, speed, accel):
    data = struct.pack("<HH", speed, accel)
    for i, pos in enumerate(positions):
        servo_id = i + 1
        data += struct.pack("<B", servo_id)
        data += struct.pack("<H", pos)
    server.send_data(0x20, data)
    start = time.perf_counter()
    while True:
        if server.has_msg():
            type, tyme, msg = server.get_msg()
            if type == 0x40:
                return
        if time.perf_counter() - start > 1.0:
            print("timeout")
            return
        time.sleep(0.001)