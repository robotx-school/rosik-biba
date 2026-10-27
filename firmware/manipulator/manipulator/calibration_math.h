#pragma once
#include <stdint.h>

// Field order/size matches upstream Preferences namespace servos/key servos.
struct ServoCalibration { uint16_t min_pos, max_pos, center_pos; };
static_assert(sizeof(ServoCalibration) == 6, "NVS calibration layout changed");
static bool validCalibration(const ServoCalibration &c) {
  return c.min_pos < 4096 && c.max_pos < 4096 && c.center_pos < 4096 &&
    c.min_pos != c.max_pos && c.center_pos != c.min_pos && c.center_pos != c.max_pos;
}
static bool centerBetween(const ServoCalibration &c) {
  return (c.center_pos > c.min_pos && c.center_pos < c.max_pos) ||
    (c.center_pos < c.min_pos && c.center_pos > c.max_pos);
}
static int32_t mapPosition(int32_t v, int32_t a, int32_t b, int32_t x, int32_t y) {
  return (v-a)*(y-x)/(b-a)+x;
}
static int normalizedToSts(const ServoCalibration &c, uint16_t pos) {
  if (!validCalibration(c) || pos > 10000) return -1;
  int32_t lo=c.min_pos, hi=c.max_pos;
  if (!centerBetween(c)) { if (lo < hi) lo+=4096; else hi+=4096; }
  return mapPosition(pos,0,10000,lo,hi)&0xfff;
}
static int stsToNormalized(const ServoCalibration &c, int raw) {
  if (!validCalibration(c) || raw<0 || raw>4095) return -1;
  int32_t lo=c.min_pos, hi=c.max_pos;
  int low=lo<hi?lo:hi, high=lo>hi?lo:hi;
  if (centerBetween(c)) {
    if(raw<low) raw=low;
    if(raw>high) raw=high;
  } else {
    // Exclude the forbidden encoder arc, projecting to its nearest endpoint.
    if(raw>low && raw<high) raw=(raw-low < high-raw)?low:high;
    if (lo<hi) {if(raw<=lo) raw+=4096;lo+=4096;}
    else {if(raw<=hi) raw+=4096;hi+=4096;}
  }
  return mapPosition(raw,lo,hi,0,10000);
}
