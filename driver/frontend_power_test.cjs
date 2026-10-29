// Run with node frontend_power_test.cjs. Simulated DOM/WebSockets; no network or hardware.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
let script = fs.readFileSync(__dirname + '/templates/index.html', 'utf8').match(/<script>([\s\S]*?)<\/script>/)[1]
  .replace('{{ .LinearLimit }}', '1.5').replace('{{ .AngularLimit }}', '2')
  .replace('{{ .RobotID }}', '"test"').replace('{{ .ICEServersJSON }}', '[]')
  .replace("{{ if .RelayOnly }}'relay'{{ else }}'all'{{ end }}", "'all'");
const elements = new Map(), sockets = [], timers = new Map();
let timerID = 0;
function element(id) {
  if (!elements.has(id)) elements.set(id, {
    disabled: id === 'power_button', style: {}, textContent: '', className: '',
    offsetWidth: id === 'joystick_handle' ? 76 : 220, handlers: {},
    addEventListener(type, fn) { this.handlers[type] = fn; },
    getBoundingClientRect() { return { left: 0, top: 0 }; }
  });
  return elements.get(id);
}
class Socket {
  static OPEN = 1; static CLOSING = 2;
  constructor(url) { this.url = url; this.readyState = 0; this.sent = []; sockets.push(this); }
  send(data) { assert.equal(this.readyState, 1); this.sent.push(JSON.parse(data)); }
  close() { this.readyState = 3; this.onclose?.(); }
}
vm.runInNewContext(script, {
  document: { getElementById: element, body: { classList: { toggle() {}, remove() {} } } },
  window: { addEventListener() {} }, navigator: { getGamepads: () => [] },
  location: { protocol: 'http:', host: 'test' }, WebSocket: Socket,
  requestAnimationFrame() {}, console,
  setTimeout(fn, ms) { const id = ++timerID; timers.set(id, { fn, ms }); return id; },
  clearTimeout(id) { timers.delete(id); }, setInterval() { return ++timerID; }, clearInterval() {}
});
const button = element('power_button'), status = element('power_status');
const control = sockets[0];
function receive(ws, data) { ws.onmessage({ data: JSON.stringify(data) }); }
function pulseCount() { return sockets.flatMap(s => s.sent).filter(m => m.type === 'power_pulse').length; }
assert.equal(button.disabled, true);
control.readyState = 1; control.onopen();
assert.equal(pulseCount(), 0, 'opening the UI must not toggle power');
receive(control, { type: 'status', serialConnected: true, powerStatus: 'ready' });
assert.equal(button.disabled, false);
button.handlers.click();
assert.equal(pulseCount(), 1); assert.equal(button.disabled, true);
button.handlers.click(); assert.equal(pulseCount(), 1, 'duplicate clicks must not repeat a pulse');
receive(control, { type: 'power_result', powerStatus: 'pending' });
assert.match(element('serial_status').textContent, /Online/, 'power ACK must not mark serial offline');
receive(control, { type: 'status', serialConnected: true, powerStatus: 'done' });
assert.match(status.textContent, /on\/off not sensed/);
assert.equal(button.disabled, true, 'cooldown remains after completion');
[...timers.values()].find(t => t.ms === 3000).fn();
assert.equal(button.disabled, false); assert.equal(pulseCount(), 1);
control.close(); assert.equal(button.disabled, true);
button.handlers.click(); assert.equal(pulseCount(), 1);
[...timers.values()].find(t => t.ms === 800).fn();
const reconnected = sockets.at(-1); reconnected.readyState = 1; reconnected.onopen();
assert.equal(pulseCount(), 1, 'reconnection must not replay a power pulse');
console.log('Frontend relay button: offline gating, single click, cooldown, ACK and no reconnect replay passed');
