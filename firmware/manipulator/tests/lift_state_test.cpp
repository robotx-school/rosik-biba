// Simulated pins/time and stepper: exercises production lift state transitions,
// not electrical pulse timing or the real motor driver.
#include "../manipulator/lift.h"
#include <assert.h>
#include <stdio.h>

static void switches(bool top,bool bottom) {
 pinLevels[LIMIT_TOP]=top?LOW:HIGH;pinLevels[LIMIT_BOTTOM]=bottom?LOW:HIGH;
}
int main(){
 switches(false,false);setupLift();
 assert(!calibrated && !liftEnabled && !liftIsMoving());
 assert(liftCommand(0x30,10)==3); // no absolute moves before homing
 assert(liftCommand(0x31)==0 && liftEnabled && !liftIsMoving());
 assert(liftCommand(0x33)==0 && calibrating && liftIsMoving());
 switches(true,false);liftTimerTick(nullptr);
 assert(calibrated && !calibrating && !liftIsMoving() && lift.getCurrent()==0);
 assert(liftCommand(0x30,MAX_TRAVEL_MM+1)==2);
 assert(liftCommand(0x30,10)==0); // move away from a pressed top switch
 liftTimerTick(nullptr);assert(lift.getCurrent()==-1);
 switches(false,true);liftTimerTick(nullptr);
 assert(!liftIsMoving() && liftFault==8 && liftEnabled);
 assert(liftCommand(0x30,20)==4); // bottom blocks further downward motion
 assert(liftCommand(0x30,0)==0);  // moving away from bottom is allowed
 switches(false,false);fakeMillis+=CONTROL_TIMEOUT_MS+1;liftTimerTick(nullptr);
 assert(!liftIsMoving() && liftFault==5 && liftEnabled);
 assert(liftCommand(0x31)==0 && !liftIsMoving()); // enable cannot resume stale target
 assert(liftCommand(0x32)==0 && !calibrated && !liftEnabled);
 assert(liftCommand(0x33)==0);
 fakeMillis+=HOMING_TIMEOUT_MS+1;touchControl();liftTimerTick(nullptr);
 assert(!calibrating && !calibrated && !liftIsMoving() && liftFault==6);
 switches(true,true);assert(liftCommand(0x33)==4);
 switches(false,false);assert(liftCommand(0x33)==0);
 lift.setCurrent(lift.getTarget());liftTimerTick(nullptr);
 assert(!calibrating && !calibrated && liftFault==6); // no switch within travel
 uint8_t status[6];getLiftStatus(status);assert(status[5]==6);
 puts("Lift homing, travel, directional endstops, watchdog and no-resume checks passed");
}
