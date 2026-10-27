// Combined FEETECH STS servo controller and local step/direction lift.
// See ../../README.md and ../../upstream/ for source attribution.
#include <Arduino.h>
#include <SCServo.h>
#include "config.h"

static SMS_STS st;
#include "calibration.h"
#include "lift.h"
#include "protocol.h"
#include "commissioning.h"

static uint8_t frame[MAX_PACKET];
static uint8_t parseState=0, frameType=0;
static uint16_t frameSize=0, framePos=0;
static uint32_t lastByte=0;
static char textLine[256];
static size_t textSize=0;
static bool textOverflow=false;

static void readUSB() {
  if(parseState && millis()-lastByte>PACKET_TIMEOUT_MS){parseState=0;framePos=0;}
  // Limit work per loop; the independent lift timer continues throughout.
  for(int budget=0;budget<128 && Serial.available();budget++) {
    uint8_t b=Serial.read();lastByte=millis();
    if(parseState==0) {
      if(b=='?'){parseState=1;textSize=0;textOverflow=false;}
      else if(!packetOnly) {
        if(b=='\n') {
          if(!textOverflow){textLine[textSize]=0;processText(textLine);}
          textSize=0;textOverflow=false;
        } else if(b!='\r') {
          if(textSize+1<sizeof(textLine))textLine[textSize++]=char(b);
          else textOverflow=true;
        }
      }
    } else if(parseState==1) {
      parseState=(b=='$')?2:((b=='?')?1:0);
    } else if(parseState==2) {frameType=b;parseState=3;}
    else if(parseState==3) {frameSize=b;parseState=4;}
    else if(parseState==4) {
      frameSize|=uint16_t(b)<<8;framePos=0;
      if(frameSize>MAX_PACKET){ack(frameType,1);parseState=6;}
      else if(frameSize==0){parseState=0;dispatchPacket(frameType,frame,0);}
      else parseState=5;
    } else if(parseState==5) {
      frame[framePos++]=b;
      if(framePos==frameSize){parseState=0;dispatchPacket(frameType,frame,frameSize);}
    } else if(parseState==6) {
      // Discard oversized frames; never interpret their payload as commands.
      if(++framePos==frameSize)parseState=0;
    }
  }
}
void setup() {
  Serial.begin(115200);
  Serial1.begin(1000000,SERIAL_8N1,SERVO_RX,SERVO_TX);
  st.pSerial=&Serial1;st.IOTimeOut=SERVO_TIMEOUT_MS;
  loadCalibration();setupLift();
  Serial.println("Manipulator ready; lift disabled and unhomed. PACKET_MODE or commissioning commands.");
}
void loop(){readUSB();delay(1);}
