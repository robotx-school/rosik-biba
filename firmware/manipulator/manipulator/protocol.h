#pragma once
#include "calibration.h"
#include "lift.h"

static uint16_t get16(const uint8_t *p){return uint16_t(p[0])|(uint16_t(p[1])<<8);}
static void sendPacket(uint8_t type,const uint8_t *data,uint16_t size) {
  uint8_t header[5]={'?','$',type,uint8_t(size),uint8_t(size>>8)};
  Serial.write(header,sizeof(header));if(size) Serial.write(data,size);
}
static void ack(uint8_t command,uint8_t error=0) {uint8_t data[2]={command,error};sendPacket(0x7e,data,2);}
static bool packetOnly=false;
static bool validID(uint8_t id){return id>=1 && id<=253;}
// Error codes: 1 malformed/unknown; 2 range; 3 not ready; 4 endstop conflict;
// 9 servo calibration missing; 10 servo read failed. Lift faults 5..8 are status-only.
static void dispatchPacket(uint8_t type,const uint8_t *data,uint16_t size) {
  if(type==0x10) {
    if(size<7 || (size-4)%3 || (size-4)/3>MAX_SERVOS){ack(type,1);return;}
    uint16_t speed=get16(data),accel=get16(data+2);
    if(speed>4095 || accel>255){ack(type,2);return;}
    const int count=(size-4)/3;
    uint8_t ids[MAX_SERVOS],accs[MAX_SERVOS];
    int16_t positions[MAX_SERVOS];uint16_t speeds[MAX_SERVOS];
    for(int i=0;i<count;i++) {
      ids[i]=data[4+3*i];uint16_t pos=get16(data+5+3*i);
      if(!validID(ids[i]) || pos>10000){ack(type,2);return;}
      for(int j=0;j<i;j++) if(ids[j]==ids[i]){ack(type,2);return;}
      int raw=normalizedToSts(servos[ids[i]],pos);
      if(raw<0){ack(type,9);return;}
      positions[i]=raw;speeds[i]=speed;accs[i]=accel;
    }
    // Validate the entire batch before transmitting any target.
    st.SyncWritePosEx(ids,count,positions,speeds,accs);
    touchControl();ack(type);
  } else if(type==0x11) {
    if(size<1 || size>MAX_SERVOS){ack(type,1);return;}
    uint8_t response[MAX_SERVOS*2];
    for(int i=0;i<size;i++) {
      if(!validID(data[i])){ack(type,2);return;}
      if(!validCalibration(servos[data[i]])){ack(type,9);return;}
    }
    for(int i=0;i<size;i++) {
      int pos=stsToNormalized(servos[data[i]],st.ReadPos(data[i]));
      if(pos<0){ack(type,10);return;}
      response[2*i]=uint8_t(pos);response[2*i+1]=uint8_t(pos>>8);
    }
    sendPacket(0x12,response,size*2);
  } else if(type==0x30) {
    if(size!=2){ack(type,1);return;}
    ack(type,liftCommand(type,get16(data)));
  } else if(type==0x31 || type==0x32 || type==0x33 || type==0x36 || type==0x37) {
    if(size!=0){ack(type,1);return;}
    ack(type,liftCommand(type));
  } else if(type==0x34) {
    if(size!=0){ack(type,1);return;}
    uint8_t response[6];getLiftStatus(response);sendPacket(0x35,response,sizeof(response));
  } else if(type=='H' && size==0) {
    packetOnly=false;ack(type);
  } else {ack(type,1);}
}
