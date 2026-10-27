#pragma once
#include "Arduino.h"
static constexpr int STEPPER2WIRE=0,FOLLOW_POS=0,ABSOLUTE=0;
template<int Type> class GStepper {
 public:
  GStepper(int,int,int,int){}
  void reverse(bool){}
  void invertEn(bool){}
  void setRunMode(int){}
  void setMaxSpeed(int){}
  void setAcceleration(int){}
  void autoPower(bool){}
  void brake(){moving=false;}
  void disable(){}
  void setCurrent(int32_t pos){current=pos;}
  int32_t getCurrent(){return current;}
  int32_t getTarget(){return target;}
  bool getState(){return moving;}
  void setTarget(int32_t pos,int){target=pos;moving=target!=current;}
  void tick(){if(current==target)moving=false;else current+=target>current?1:-1;}
 private:
  int32_t current=0,target=0;
  bool moving=false;
};
