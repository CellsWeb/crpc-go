import assert from 'node:assert/strict';
import net from 'node:net';
import { Duplex } from 'node:stream';
import { test } from 'node:test';
import { GoRpcClient } from '../dist/src/client.js';
import { bounded, cleanupClient, fixture, networkErrors, options, outcome, reply, trackTimers, until } from './helpers.mjs';

for (const line of ['{bad-json}', 'not JSON', '{"id":"truncated"', '{"data":NaN}']) {
  test(`corrupted response ${line} promptly fails all pending calls while peer stays open`, async t => {
    const timers = trackTimers(t);
    t.mock.method(console, 'error', () => {}); // Silence the baseline log-and-ignore behavior.
    const f = await fixture(t);
    const calls = Array.from({ length: 4 }, () => outcome(f.client.call('system.ping', {}, options)));
    await until(() => f.requests.length === 4);
    const socket = f.client.socket, reader = f.client.rl;
    f.requests[0].socket.write(line + '\n');
    networkErrors(await bounded(Promise.all(calls)), /IPC stream corruption.*unparsable response/i);
    assert.equal(f.client.pending.size, 0);
    assert.equal(f.client.socket, null);
    assert.equal(f.client.rl, null);
    assert.equal(f.client.connecting, null);
    assert.equal(f.client.disconnect, null);
    assert.equal(socket.destroyed, true);
    assert.equal(reader.listenerCount('line'), 0);
    await until(() => socket.listenerCount('close') === 0);
    assert.equal(socket.listenerCount('error'), 0);
    assert.equal(reader.listenerCount('error'), 0);
    timers.assertEmpty();
  });
}

test('corruption wins over coalesced late replies; stale events cannot affect a reconnected caller; no replay', async t => {
  const timers = trackTimers(t);
  t.mock.method(console, 'error', () => {});
  const f = await fixture(t);
  const calls = Array.from({ length: 3 }, () => outcome(f.client.call('system.ping', {}, options)));
  await until(() => f.requests.length === 3);
  const socket = f.client.socket, reader = f.client.rl;
  const staleLine = reader.listeners('line')[0];
  const late = JSON.stringify({ id: f.requests[0].request.id, status: 'ok', data: 'late' });
  f.requests[0].socket.write('{broken}\n' + late + '\n');
  const results = await bounded(Promise.all(calls));
  networkErrors(results, /stream corruption/);
  assert.ok(results.every(result => result.error === results[0].error), 'connection error is shared, not arbitrarily correlated');
  timers.assertEmpty();
  const next = outcome(f.client.call('system.ping', {}, options));
  await until(() => f.requests.length === 4);
  const currentSocket = f.client.socket;
  staleLine('{old-corruption}'); // Simulate an already queued callback from the old generation.
  reader.emit('line', late);
  socket.emit('close');
  assert.equal(f.client.socket, currentSocket);
  assert.equal(f.client.pending.size, 1);
  reply(f.requests[3].socket, f.requests[3].request, 'reconnected');
  assert.equal((await bounded(next)).value, 'reconnected');
  assert.deepEqual(f.requests.map(item => item.generation), [1, 1, 1, 2]);
  assert.equal(new Set(f.requests.map(item => item.request.id)).size, 4);
  assert.equal(f.client.pending.size, 0);
  timers.assertEmpty();
});

test('truncated response at EOF rejects promptly and cleans the connection', async t => {
  const timers = trackTimers(t);
  t.mock.method(console, 'error', () => {});
  const f = await fixture(t);
  const calls = [outcome(f.client.call('system.ping', {}, options)), outcome(f.client.call('system.ping', {}, options))];
  await until(() => f.requests.length === 2);
  f.requests[0].socket.end('{"id":'); // No trailing newline; readline flushes the incomplete line at EOF.
  networkErrors(await bounded(Promise.all(calls)), /stream corruption/);
  assert.equal(f.client.pending.size, 0);
  assert.equal(f.client.socket, null);
  timers.assertEmpty();
});

test('partial valid response is buffered until completed; whitespace and CRLF remain supported', async t => {
  const timers = trackTimers(t);
  const f = await fixture(t);
  const call = outcome(f.client.call('system.ping', {}, options));
  await until(() => f.requests.length === 1);
  const { socket, request } = f.requests[0];
  socket.write(' \r\n\n{"id":');
  await new Promise(resolve => setTimeout(resolve, 20));
  assert.equal(f.client.pending.size, 1);
  assert.equal(f.client.socket.destroyed, false);
  socket.write(JSON.stringify(request.id) + ',"status":"ok","data":{"split":true}}\r\n');
  assert.deepEqual((await bounded(call)).value, { split: true });
  assert.equal(f.client.pending.size, 0);
  timers.assertEmpty();
});

test('mocked socket: invalid JSON retires connection and removes readline/socket listeners', async t => {
  const timers = trackTimers(t);
  t.mock.method(console, 'error', () => {});
  // A connected socket stand-in: an unconnected net.Socket adds its own
  // internal connect listener when readline resumes it, unlike a real peer.
  const socket = new Duplex({ read() {}, write(_chunk, _encoding, callback) { callback(); } });
  t.mock.method(net, 'createConnection', () => socket);
  const write = t.mock.method(socket, 'write', () => true);
  const client = new GoRpcClient();
  t.after(() => { cleanupClient(client); socket.destroy(); });
  const calls = Array.from({ length: 3 }, () => outcome(client.call('system.ping', {}, options)));
  socket.emit('connect');
  await until(() => client.pending.size === 3);
  const reader = client.rl;
  socket.emit('data', Buffer.from('{corrupted}\n'));
  networkErrors(await bounded(Promise.all(calls)), /stream corruption/);
  assert.equal(write.mock.callCount(), 3);
  assert.equal(client.pending.size, 0);
  assert.equal(client.socket, null);
  assert.equal(socket.destroyed, true);
  await until(() => socket.listenerCount('close') === 0);
  assert.equal(socket.listenerCount('connect'), 0);
  assert.equal(socket.listenerCount('error'), 0);
  assert.equal(reader.listenerCount('line'), 0);
  assert.equal(reader.listenerCount('error'), 0);
  timers.assertEmpty();
});
