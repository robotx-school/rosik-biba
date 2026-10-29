#pragma once
#include <stdint.h>
#define LOW 0
#define HIGH 1
#define OUTPUT 3
static int pinLevels[64];
inline void pinMode(int,int){}
inline void digitalWrite(int pin,int level){pinLevels[pin]=level;}
