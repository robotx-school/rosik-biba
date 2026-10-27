#pragma once
#include <Preferences.h>
#include "calibration_math.h"

static ServoCalibration servos[256] = {};
static Preferences preferences;
static void loadCalibration() {
  preferences.begin("servos",true);
  if(preferences.getBytesLength("servos")==sizeof(servos)) {
    preferences.getBytes("servos",servos,sizeof(servos));
    preferences.end();return;
  }
  preferences.end();
  // Migrate the older twelve-servo NVS layout without changing its mapping.
  uint16_t mins[12]={}, maxs[12]={}, centers[12]={};
  preferences.begin("poses",true);
  if(preferences.getBytesLength("min")==sizeof(mins) &&
     preferences.getBytesLength("max")==sizeof(maxs) &&
     preferences.getBytesLength("center")==sizeof(centers)) {
    preferences.getBytes("min",mins,sizeof(mins));
    preferences.getBytes("max",maxs,sizeof(maxs));
    preferences.getBytes("center",centers,sizeof(centers));
    for(int i=0;i<12;i++) servos[i+1]={mins[i],maxs[i],centers[i]};
  }
  preferences.end();
}
static bool saveCalibration() {
  if(!preferences.begin("servos",false)) return false;
  bool ok=preferences.putBytes("servos",servos,sizeof(servos))==sizeof(servos);
  preferences.end();return ok;
}
