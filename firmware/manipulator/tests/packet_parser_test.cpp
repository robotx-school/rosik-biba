#include "../manipulator/manipulator.ino"
#include <assert.h>
#include <stdio.h>

static void bytes(std::initializer_list<uint8_t> data){for(auto b:data)Serial.input.push_back(b);readUSB();}
static void expectAck(uint8_t type,uint8_t code){
 assert(Serial.output.size()==7);
 assert(Serial.output[0]=='?' && Serial.output[1]=='$' && Serial.output[2]==0x7e);
 assert(Serial.output[3]==2 && Serial.output[4]==0);
 assert(Serial.output[5]==type && Serial.output[6]==code);Serial.output.clear();
}
int main(){
 pinLevels[LIMIT_TOP]=HIGH;pinLevels[LIMIT_BOTTOM]=HIGH;setup();
 bytes({'?'});assert(Serial.output.empty());
 bytes({'?','$',0x31,0});assert(Serial.output.empty());
 bytes({0});expectAck(0x31,0);assert(liftEnabled && !liftIsMoving());
 servos[1]={773,3470,2715};servos[2]={754,3130,1551};
 bytes({'?','$',0x10,7,0,0x90,1,50,0,1,0x88,0x13});
 expectAck(0x10,0);assert(st.syncWrites==1 && st.lastCount==1);
 bytes({'?','$',0x10,10,0,0x90,1,50,0,1,0x88,0x13,2,0x88,0x13});
 expectAck(0x10,0);assert(st.syncWrites==2 && st.lastCount==2);
 bytes({'?','$',0x10,6,0,0,0,0,0,1,0});
 expectAck(0x10,1);assert(st.syncWrites==2);
 bytes({'?','$',0x10,10,0,0x90,1,50,0,1,0x88,0x13,1,0x88,0x13});
 expectAck(0x10,2);assert(st.syncWrites==2); // duplicate rejects whole batch
 bytes({'?','$',0x10,7,0,0x90,1,50,0,3,0x88,0x13});
 expectAck(0x10,9);assert(st.syncWrites==2); // no NVS calibration
 bytes({'?','$',0x32,0,0});expectAck(0x32,0);assert(!liftEnabled);
 // An embedded valid packet inside an oversized payload cannot enable a motor.
 bytes({'?','$',0x31,1,1});expectAck(0x31,1);
 const uint8_t embedded[]={'?','$',0x31,0,0};
 for(int i=0;i<257;i++)Serial.input.push_back(i<5?embedded[i]:0);
 while(Serial.available())readUSB();
 assert(!liftEnabled && Serial.output.empty());
 bytes({'?','$',0x30,2,0,1}); // interrupted height frame
 fakeMillis+=PACKET_TIMEOUT_MS+1;readUSB();
 bytes({'?','$',0x36,0,0});expectAck(0x36,0);
 bytes({'?','$',0x33,0,0});expectAck(0x33,0);assert(calibrating);
 puts("Production firmware parser: fragmented, empty, oversized, timed-out and invalid batches passed");
}
