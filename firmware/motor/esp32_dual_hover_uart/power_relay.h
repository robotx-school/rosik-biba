#pragma once
#include <Arduino.h>
#include <atomic>
#include "config.h"

class PowerRelay {
 public:
  void begin() {
    // Set the inactive latch before enabling the output.
    digitalWrite(POWER_RELAY_PIN,POWER_RELAY_ACTIVE_HIGH?LOW:HIGH);
    pinMode(POWER_RELAY_PIN,OUTPUT);
    active_=false;
  }
  bool pulse(uint32_t now) {
    if(active_.load())return false;
    started_=now;
    digitalWrite(POWER_RELAY_PIN,POWER_RELAY_ACTIVE_HIGH?HIGH:LOW);
    active_.store(true);
    return true;
  }
  bool tick(uint32_t now) {
    if(!active_.load() || uint32_t(now-started_)<POWER_RELAY_PULSE_MS)return false;
    digitalWrite(POWER_RELAY_PIN,POWER_RELAY_ACTIVE_HIGH?LOW:HIGH);
    active_=false;
    return true;
  }
  bool active()const{return active_.load();}
 private:
  std::atomic<bool> active_{false};
  uint32_t started_=0;
};
