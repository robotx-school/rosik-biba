#pragma once
#include <GyverStepper.h>
#include <esp_timer.h>
#include "config.h"

static GStepper<STEPPER2WIRE> lift(800,LIFT_STEP,LIFT_DIR,LIFT_ENABLE);
static portMUX_TYPE liftMux = portMUX_INITIALIZER_UNLOCKED;
static esp_timer_handle_t liftTimer;
static bool calibrated=false, calibrating=false, liftEnabled=false;
static uint32_t homingStarted=0, lastControl=0;
static uint8_t liftFault=0;
static bool topPressed(){return digitalRead(LIMIT_TOP)==LIMIT_ACTIVE_LEVEL;}
static bool bottomPressed(){return digitalRead(LIMIT_BOTTOM)==LIMIT_ACTIVE_LEVEL;}
// All calls ending in Locked run with liftMux held, including the timer tick.
static void haltLiftLocked(uint8_t fault=0) {
  lift.brake(); // immediate stop: deceleration must not pass an endstop
  if(calibrating) calibrated=false;
  calibrating=false;
  lift.setMaxSpeed(MAX_SPEED);
  liftFault=fault;
  // Keep holding power: disabling a vertical lift can let it fall.
}
static void liftTimerTick(void*) {
  portENTER_CRITICAL(&liftMux);
  const uint32_t now=millis();
  bool top=topPressed(),bottom=bottomPressed();
  if(top && bottom) {
    calibrated=false;haltLiftLocked(4);
  } else if(lift.getState()) {
    const bool up=lift.getTarget()>lift.getCurrent();
    if(now-lastControl>CONTROL_TIMEOUT_MS) {
      haltLiftLocked(5);
    } else if(calibrating && now-homingStarted>HOMING_TIMEOUT_MS) {
      haltLiftLocked(6);
    } else if(up && top) {
      bool homing=calibrating;
      haltLiftLocked();
      lift.setCurrent(0);
      calibrated=true;
      if(!homing) liftFault=7; // upper endstop reached during ordinary motion
    } else if(!up && bottom) {
      haltLiftLocked(8);
    } else {
      lift.tick();
      if(calibrating && !lift.getState()) {calibrated=false;haltLiftLocked(6);}
    }
  }
  portEXIT_CRITICAL(&liftMux);
}
static void setupLift() {
  // Both limit switch inputs use internal pullups; switches connect to GND.
  pinMode(LIMIT_TOP,INPUT_PULLUP);pinMode(LIMIT_BOTTOM,INPUT_PULLUP);
  lift.reverse(LIFT_REVERSE);
  lift.invertEn(!ENABLE_ACTIVE_LOW);
  lift.setRunMode(FOLLOW_POS);lift.setMaxSpeed(MAX_SPEED);
  lift.setAcceleration(ACCELERATION);lift.autoPower(false);
  lift.brake();lift.disable();
  esp_timer_create_args_t args={};args.callback=liftTimerTick;args.name="lift-tick";
  ESP_ERROR_CHECK(esp_timer_create(&args,&liftTimer));
  // A separate timer task keeps stepping/endstop/watchdog checks running while
  // the servo UART waits for replies. No serial/NVS work occurs in this task.
  ESP_ERROR_CHECK(esp_timer_start_periodic(liftTimer,50));
}
static uint8_t liftCommand(uint8_t type,uint16_t mm=0) {
  portENTER_CRITICAL(&liftMux);
  uint8_t error=0;
  if(type==0x32) {
    haltLiftLocked();lift.disable();liftEnabled=false;calibrated=false;
  } else if(type==0x36) {
    haltLiftLocked();
  } else if(type==0x37) {
    lastControl=millis();
  } else if(topPressed() && bottomPressed()) {
    calibrated=false;haltLiftLocked(4);error=4;
  } else if(type==0x31) {
    // Do not call GStepper::enable here: it resumes a stale target.
    digitalWrite(LIFT_ENABLE,ENABLE_ACTIVE_LOW?LOW:HIGH);
    liftEnabled=true;
  } else if(type==0x33) {
    haltLiftLocked();calibrated=false;liftEnabled=true;lastControl=millis();
    digitalWrite(LIFT_ENABLE,ENABLE_ACTIVE_LOW?LOW:HIGH);
    if(topPressed()) {lift.setCurrent(0);calibrated=true;}
    else {
      calibrating=true;homingStarted=millis();lift.setMaxSpeed(MAX_SPEED/2);
      lift.setTarget(lift.getCurrent()+int32_t(HOMING_TRAVEL_MM)*STEPS_PER_MM,ABSOLUTE);
    }
  } else if(type==0x30) {
    if(!liftEnabled || !calibrated || calibrating) error=3;
    else if(mm>MAX_TRAVEL_MM) error=2;
    else {
      int32_t target=-int32_t(mm)*STEPS_PER_MM;
      bool up=target>lift.getCurrent(),down=target<lift.getCurrent();
      if((up && topPressed()) || (down && bottomPressed())) error=4;
      else {liftFault=0;lastControl=millis();lift.setTarget(target,ABSOLUTE);}
    }
  }
  portEXIT_CRITICAL(&liftMux);
  return error;
}
static void touchControl(){portENTER_CRITICAL(&liftMux);lastControl=millis();portEXIT_CRITICAL(&liftMux);}
static bool liftIsMoving(){portENTER_CRITICAL(&liftMux);bool moving=lift.getState();portEXIT_CRITICAL(&liftMux);return moving;}
static void getLiftStatus(uint8_t out[6]) {
  portENTER_CRITICAL(&liftMux);
  uint32_t pos=uint32_t(lift.getCurrent());
  for(int i=0;i<4;i++) out[i]=uint8_t(pos>>(8*i));
  out[4]=(calibrated?1:0)|(calibrating?2:0)|(liftEnabled?4:0)|
    (lift.getState()?8:0)|(topPressed()?16:0)|(bottomPressed()?32:0);
  out[5]=liftFault;
  portEXIT_CRITICAL(&liftMux);
}
