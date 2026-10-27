#pragma once
#include <stdint.h>
#define LOW 0
#define HIGH 1
#define INPUT 0
#define INPUT_PULLUP 2
#define OUTPUT 3
static uint32_t fakeMillis=0;
static int pinLevels[64];
static uint32_t millis(){return fakeMillis;}
static void pinMode(int,int){}
static int digitalRead(int pin){return pinLevels[pin];}
static void digitalWrite(int pin,int level){pinLevels[pin]=level;}
using portMUX_TYPE=int;
#define portMUX_INITIALIZER_UNLOCKED 0
#define portENTER_CRITICAL(p) ((void)(p))
#define portEXIT_CRITICAL(p) ((void)(p))
#include <stddef.h>
#include <deque>
#include <vector>
#define SERIAL_8N1 0
inline void delay(int){}
class FakeSerial {
 public:
  std::deque<uint8_t> input;
  std::vector<uint8_t> output;
  template<typename... Args> void begin(Args...){}
  int available(){return input.size();}
  int read(){if(input.empty())return -1;int b=input.front();input.pop_front();return b;}
  size_t write(const uint8_t *p,size_t n){output.insert(output.end(),p,p+n);return n;}
  template<typename T> void println(T){}
  template<typename... Args> void printf(const char*,Args...){}
};
static FakeSerial Serial,Serial1;
