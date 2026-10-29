#include "../esp32_dual_hover_uart/power_relay.h"
#include <assert.h>
#include <stdio.h>

int main(){
 PowerRelay relay;
 relay.begin();assert(!relay.active() && pinLevels[POWER_RELAY_PIN]==LOW);
 assert(relay.pulse(100));assert(relay.active() && pinLevels[POWER_RELAY_PIN]==HIGH);
 assert(!relay.tick(599));assert(!relay.pulse(599)); // a duplicate must not extend the pulse
 assert(relay.tick(600));assert(!relay.active() && pinLevels[POWER_RELAY_PIN]==LOW);
 assert(!relay.tick(601)); // completion is emitted once
 assert(relay.pulse(1000));assert(relay.tick(1500)); // same pulse for either physical toggle direction
 assert(relay.pulse(UINT32_MAX-200));assert(!relay.tick(298));assert(relay.tick(299));
 puts("Relay boot inactivity, 500ms pulse, duplicate suppression and clock wrap passed");
}
