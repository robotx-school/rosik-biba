#include "../manipulator/calibration_math.h"
#include <assert.h>
#include <stdio.h>

int main() {
  ServoCalibration examples[]={
    {773,3470,2715}, {3198,1002,2575}, {318,20,2188},
    {1888,282,3051}, {2000,3890,1058}, {4061,3756,1865}
  };
  for(const auto &c:examples) {
    assert(validCalibration(c));
    assert(normalizedToSts(c,0)==c.min_pos);
    assert(normalizedToSts(c,10000)==c.max_pos);
    assert(stsToNormalized(c,c.min_pos)==0);
    assert(stsToNormalized(c,c.max_pos)==10000);
    for(int raw=0;raw<4096;raw++) {
      int pos=stsToNormalized(c,raw);
      assert(pos>=0 && pos<=10000);
    }
    for(int pos=0;pos<=10000;pos+=100) {
      int raw=normalizedToSts(c,pos);
      assert(raw>=0 && raw<4096);
      int roundtrip=stsToNormalized(c,raw);
      assert(roundtrip>=pos-40 && roundtrip<=pos+40);
    }
  }
  assert(!validCalibration({0,0,0}));
  assert(normalizedToSts({0,0,0},5000)==-1);
  assert(stsToNormalized(examples[0],-1)==-1);
  assert(normalizedToSts(examples[0],10001)==-1);
  puts("Calibration endpoints, inverted and wrapped arcs passed");
}
