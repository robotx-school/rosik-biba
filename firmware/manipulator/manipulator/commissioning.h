#pragma once
#include <ctype.h>
#include <stdlib.h>
#include <string.h>

static void processText(char *line) {
  // Tokenize bounded text without String allocation or permissive atoi parsing.
  char *save=nullptr;char *name=strtok_r(line," \t\r",&save);
  if(!name)return;
  for(char *p=name;*p;p++)*p=toupper(*p);
  if(strcmp(name,"PACKET_MODE")==0){packetOnly=true;return;}
  long args[1+4*MAX_SERVOS]={};int count=0;
  while(char *token=strtok_r(nullptr," \t\r",&save)) {
    if(count>=int(sizeof(args)/sizeof(args[0]))){Serial.println("ERROR arguments");return;}
    char *end;args[count]=strtol(token,&end,10);
    if(*end){Serial.println("ERROR integer required");return;}count++;
  }
  uint8_t type=0;
  if(!strcmp(name,"ENABLE_LIFT")) type=0x31;
  else if(!strcmp(name,"DISABLE_LIFT")) type=0x32;
  else if(!strcmp(name,"CALIBRATE_LIFT") || !strcmp(name,"CALLIBRATE_LIFT") || !strcmp(name,"CALIBRATION")) type=0x33;
  else if(!strcmp(name,"STOP_LIFT")) type=0x36;
  else if(!strcmp(name,"HEARTBEAT")) type=0x37;
  if(type) {if(count){Serial.println("ERROR arguments");return;}Serial.printf("RESULT %u\n",liftCommand(type));return;}
  if(!strcmp(name,"SET_LIFT")) {
    if(count!=1 || args[0]<0 || args[0]>60000){Serial.println("ERROR height");return;}
    Serial.printf("RESULT %u\n",liftCommand(0x30,args[0]));return;
  }
  if(!strcmp(name,"GETPOS") || !strcmp(name,"GET_LIFT_STATUS")) {
    uint8_t out[6];getLiftStatus(out);
    int32_t pos=int32_t(uint32_t(out[0])|(uint32_t(out[1])<<8)|(uint32_t(out[2])<<16)|(uint32_t(out[3])<<24));
    Serial.printf("LIFT steps=%ld mm=%.2f flags=%u fault=%u\n",long(pos),-float(pos)/STEPS_PER_MM,out[4],out[5]);return;
  }
  // NVS flash writes may pause tasks. Commission only with the lift stopped.
  if(liftIsMoving()){Serial.println("ERROR stop lift before commissioning");return;}
  if(!strcmp(name,"SAVE") && count==0){Serial.println(saveCalibration()?"OK":"ERROR save");return;}
  if(count<1 || args[0]<1 || args[0]>253){Serial.println("ERROR servo ID");return;}
  uint8_t id=args[0];
  if(!strcmp(name,"SET_CALLIB")) {
    if(count!=4 || args[1]<0 || args[1]>4095 || args[2]<0 || args[2]>4095 || args[3]<0 || args[3]>4095){Serial.println("ERROR calibration");return;}
    ServoCalibration c={uint16_t(args[1]),uint16_t(args[2]),uint16_t(args[3])};
    if(!validCalibration(c)){Serial.println("ERROR calibration");return;}
    servos[id]=c;Serial.println(saveCalibration()?"OK":"ERROR save");
  } else if(!strcmp(name,"GET_CALLIB") && count==1) {
    auto c=servos[id];Serial.printf("min: %u max: %u center: %u valid: %u\n",c.min_pos,c.max_pos,c.center_pos,validCalibration(c));
  } else if((!strcmp(name,"MIN") || !strcmp(name,"MAX") || !strcmp(name,"CENTER")) && count==1) {
    int raw=st.ReadPos(id);
    if(raw<0 || raw>4095){Serial.println("ERROR servo read");return;}
    if(!strcmp(name,"MIN"))servos[id].min_pos=raw;
    else if(!strcmp(name,"MAX"))servos[id].max_pos=raw;
    else servos[id].center_pos=raw;
    Serial.println(saveCalibration()?"OK":"ERROR save");
  } else if((!strcmp(name,"READ") || !strcmp(name,"READ_RAW")) && count==1) {
    int raw=st.ReadPos(id);int pos=!strcmp(name,"READ")?stsToNormalized(servos[id],raw):raw;
    Serial.printf("POS %u %d\n",id,pos);
  } else if((!strcmp(name,"MOVE") || !strcmp(name,"MOVE_RAW")) && count==4) {
    bool rawMode=!strcmp(name,"MOVE_RAW");
    if(args[1]<0 || args[1]>(rawMode?4095:10000) || args[2]<0 || args[2]>4095 || args[3]<0 || args[3]>255){Serial.println("ERROR range");return;}
    int raw=rawMode?args[1]:normalizedToSts(servos[id],args[1]);
    if(raw<0){Serial.println("ERROR calibration missing");return;}
    Serial.println(st.WritePosEx(id,raw,args[2],args[3])!=-1?"OK":"ERROR servo write");
  } else if(!strcmp(name,"PING") && count==1) {
    Serial.println(st.Ping(id)!=-1?"OK":"ERROR ping");
  } else if((!strcmp(name,"ENABLE_TORQUE") || !strcmp(name,"DISABLE_TORQUE")) && count==1) {
    Serial.println(st.EnableTorque(id,!strcmp(name,"ENABLE_TORQUE"))!=-1?"OK":"ERROR torque");
  } else if(!strcmp(name,"SET_SERVO") && count==1) {
    Serial.println(st.writeByte(id,SMS_STS_MODE,0)!=-1?"OK":"ERROR mode");
  } else if(!strcmp(name,"SET_ID") && count==2 && args[1]>=1 && args[1]<=253) {
    st.unLockEprom(id);int result=st.writeByte(id,SMS_STS_ID,args[1]);st.LockEprom(args[1]);
    Serial.println(result!=-1?"OK":"ERROR set ID");
  } else if(!strcmp(name,"READ_LOAD") && count==1) Serial.printf("LOAD %d\n",st.ReadLoad(id));
  else if(!strcmp(name,"READ_VOLTAGE") && count==1) Serial.printf("VOLTAGE %d\n",st.ReadVoltage(id));
  else if(!strcmp(name,"READ_TEMP") && count==1) Serial.printf("TEMP %d\n",st.ReadTemper(id));
  else Serial.println("ERROR unknown command or arguments");
}
