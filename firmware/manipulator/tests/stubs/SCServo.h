#pragma once
#include "Arduino.h"
#define SMS_STS_MODE 33
#define SMS_STS_ID 5
class SMS_STS {
 public:
  FakeSerial *pSerial=nullptr;
  int IOTimeOut=100;
  int syncWrites=0,lastCount=0;
  void SyncWritePosEx(uint8_t*,uint8_t count,int16_t*,uint16_t*,uint8_t*){syncWrites++;lastCount=count;}
  int ReadPos(uint8_t){return 2000;}
  int WritePosEx(uint8_t,int,uint16_t,uint8_t){return 1;}
  int Ping(uint8_t){return 1;}
  int EnableTorque(uint8_t,int){return 1;}
  int writeByte(uint8_t,int,int){return 1;}
  void unLockEprom(uint8_t){}
  void LockEprom(uint8_t){}
  int ReadLoad(uint8_t){return 0;}
  int ReadVoltage(uint8_t){return 0;}
  int ReadTemper(uint8_t){return 0;}
};
