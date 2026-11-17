import assert from 'node:assert/strict';
import net from 'node:net';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { test } from 'node:test';
import { GoRpcClient } from '../dist/src/client.js';
import { bounded, cleanupClient, fixture, networkErrors, options, outcome, reply, trackTimers, until } from './helpers.mjs';

for (const termination of ['destroy', 'end']) {
test(`remote ${termination} rejects every concurrent request and clears all timers`, async t => {
  const timers = trackTimers(t);
  const f = await fixture(t);
  const calls = Array.from({ length: 4 }, () => outcome(f.client.call('system.ping', {}, options)));
  await until(() => f.requests.length === 4);
  assert.equal(f.connections(), 1);
  const socket = f.client.socket, reader = f.client.rl;
  f.requests[0].socket[termination]();
  networkErrors(await bounded(Promise.all(calls)), /closed/i);
  assert.equal(f.client.pending.size, 0);
  assert.equal(f.client.socket, null);
  assert.equal(f.client.rl, null);
  assert.equal(reader.listenerCount('line'), 0);
  await until(() => socket.listenerCount('close') === 0);
  assert.equal(socket.listenerCount('error'), 0);
  timers.assertEmpty();
});
}

test('readline input error rejects all callers without an unhandled error or duplicate settlement', async t => {
  const timers = trackTimers(t);
  const f = await fixture(t);
  const calls = [outcome(f.client.call('system.ping', {}, options)), outcome(f.client.call('system.ping', {}, options))];
  await until(() => f.requests.length === 2);
  const socket = f.client.socket, reader = f.client.rl;
  reader.emit('error', new Error('fixture input error'));
  socket.emit('error', new Error('later socket error'));
  networkErrors(await bounded(Promise.all(calls)), /input error/);
  await until(() => socket.listenerCount('close') === 0);
  assert.equal(reader.listenerCount('error'), 0);
  assert.equal(f.client.pending.size, 0);
  timers.assertEmpty();
});

for (const id of ['', undefined, null]) {
  test(`anonymous error id=${String(id)} fails the connection, not an arbitrary call`, async t => {
    const timers = trackTimers(t);
    const f = await fixture(t);
    const calls = Array.from({ length: 3 }, () => outcome(f.client.call('system.ping', {}, options)));
    await until(() => f.requests.length === 3);
    const error = { id, status: 'error', error: 'Invalid JSON: uncorrelated frame', details: { transport: true } };
    f.requests[0].socket.write(JSON.stringify(error) + '\n'); // Peer stays open.
    const results = await bounded(Promise.all(calls));
    networkErrors(results, /Invalid JSON/);
    for (const result of results) assert.deepEqual(result.error.details, { transport: true });
    assert.equal(f.client.pending.size, 0);
    assert.equal(f.client.socket, null);
    timers.assertEmpty();
  });
}

test('successful/error responses preserve IDs, payload fields and chunked newline framing', async t => {
  const timers = trackTimers(t);
  const f = await fixture(t, (socket, _, generation, requests) => {
    if (requests.length !== 3) return;
    const [a, b, c] = requests.map(item => item.request);
    assert.equal(generation, 1);
    assert.equal(new Set([a.id, b.id, c.id]).size, 3);
    assert.deepEqual(a.headers, { authorization: 'Bearer valid' });
    assert.equal(a.skipMiddlewares, true);
    assert.deepEqual(a.context, { auth: { actor: 'fixture' } });
    const frames = [
      { id: 'unknown-request', status: 'error', error: 'Ignore unmatched nonempty ID' },
      { id: c.id, status: 'ok', data: null },
      { id: b.id, status: 'error', code: 'denied', error: 'Not allowed', details: { rule: 1 } },
      { id: a.id, status: 'ok', data: ['array', { nested: true }] },
      { id: a.id, status: 'error', error: 'Late duplicate must be ignored' },
    ].map(value => JSON.stringify(value)).join('\n') + '\n\n';
    socket.write(frames.slice(0, 17));
    socket.write(frames.slice(17));
  });
  const a = outcome(f.client.call('system.ping', {}, { ...options, headers: { authorization: 'Bearer valid' }, skipMiddlewares: true, context: { auth: { actor: 'fixture' } } }));
  const b = outcome(f.client.call('system.ping', {}, options));
  const c = outcome(f.client.call('system.ping', {}, options));
  const results = await bounded(Promise.all([a, b, c]));
  assert.deepEqual(results[0].value, ['array', { nested: true }]);
  assert.equal(results[1].error.code, 'denied');
  assert.equal(results[1].error.message, 'Not allowed');
  assert.deepEqual(results[1].error.details, { rule: 1 });
  assert.equal(results[2].value, null);
  assert.equal(f.client.pending.size, 0);
  timers.assertEmpty();
});

test('timeout settles once; late and duplicate replies do not affect subsequent requests', async t => {
  const timers = trackTimers(t);
  const f = await fixture(t);
  const first = outcome(f.client.call('system.ping', {}, { timeoutMs: 30 }));
  await until(() => f.requests.length === 1);
  const timedOut = await bounded(first);
  assert.equal(timedOut.error.code, 'timeout');
  assert.match(timedOut.error.message, /30ms/);
  const second = outcome(f.client.call('system.ping', {}, options));
  await until(() => f.requests.length === 2);
  reply(f.requests[0].socket, f.requests[0].request, 'late');
  reply(f.requests[1].socket, f.requests[1].request, 'current');
  reply(f.requests[1].socket, f.requests[1].request, 'duplicate');
  assert.equal((await bounded(second)).value, 'current');
  assert.equal(f.connections(), 1);
  assert.equal(f.client.pending.size, 0);
  timers.assertEmpty();
});

test('explicit close settles pending calls; reconnection never replays calls or lets old close clear new socket', async t => {
  const timers = trackTimers(t);
  const f = await fixture(t);
  const calls = [outcome(f.client.call('system.ping', {}, options)), outcome(f.client.call('system.ping', {}, options))];
  await until(() => f.requests.length === 2);
  const oldSocket = f.client.socket;
  f.client.close(); f.client.close();
  networkErrors(await bounded(Promise.all(calls)), /closed/i);
  const next = outcome(f.client.call('system.ping', {}, options));
  await until(() => f.requests.length === 3);
  const newSocket = f.client.socket;
  assert.notEqual(newSocket, oldSocket);
  oldSocket.emit('close');
  assert.equal(f.client.socket, newSocket);
  reply(f.requests[2].socket, f.requests[2].request, 'reconnected');
  assert.equal((await bounded(next)).value, 'reconnected');
  assert.deepEqual(f.requests.map(item => item.generation), [1, 1, 2]);
  assert.equal(new Set(f.requests.map(item => item.request.id)).size, 3);
  timers.assertEmpty();
});

test('concurrent connection failures reject together and a later attempt can connect', async t => {
  const timers = trackTimers(t);
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'ipc007-'));
  const socketPath = path.join(dir, 'rpc.sock');
  const client = new GoRpcClient({ socketPath });
  let server;
  const sockets = new Set();
  t.after(async () => {
    cleanupClient(client);
    for (const socket of sockets) socket.destroy();
    if (server) await new Promise(resolve => server.close(resolve));
    fs.rmSync(dir, { recursive: true, force: true });
  });
  const calls = Array.from({ length: 3 }, () => outcome(client.call('system.ping', {}, options)));
  networkErrors(await bounded(Promise.all(calls)), /connect/i);
  timers.assertEmpty();
  server = net.createServer(socket => {
    sockets.add(socket);
    socket.once('data', data => reply(socket, JSON.parse(data.toString()), 'available'));
  });
  await new Promise(resolve => server.listen(socketPath, resolve));
  assert.equal(await bounded(client.call('system.ping', {}, options)), 'available');
  timers.assertEmpty();
});

test('a destroyed socket awaiting its close event cannot strand old pending calls during reconnect', async t => {
  const timers = trackTimers(t);
  const f = await fixture(t);
  const oldCall = outcome(f.client.call('system.ping', {}, options));
  await until(() => f.requests.length === 1);
  f.client.socket.destroy(); // destroyed is synchronous; close is a later event.
  const nextCall = outcome(f.client.call('system.ping', {}, options));
  await until(() => f.requests.length === 2);
  reply(f.requests[1].socket, f.requests[1].request, 'new-generation');
  const [old, next] = await bounded(Promise.all([oldCall, nextCall]));
  networkErrors([old], /closed/i);
  assert.equal(next.value, 'new-generation');
  assert.equal(f.client.pending.size, 0);
  timers.assertEmpty();
});

test('close before connect completes rejects every waiter and removes connect listeners', async t => {
  const timers = trackTimers(t);
  const socket = new net.Socket();
  t.mock.method(net, 'createConnection', () => socket);
  const client = new GoRpcClient();
  t.after(() => { cleanupClient(client); socket.destroy(); });
  const calls = Array.from({ length: 3 }, () => outcome(client.call('system.ping', {}, options)));
  client.close();
  networkErrors(await bounded(Promise.all(calls)), /closed/i);
  assert.equal(socket.listenerCount('connect'), 0);
  timers.assertEmpty();
});

test('connection deadline rejects shared waiters and cleans its timer', async t => {
  const timers = trackTimers(t);
  const socket = new net.Socket();
  t.mock.method(net, 'createConnection', () => socket);
  const client = new GoRpcClient();
  t.after(() => { cleanupClient(client); socket.destroy(); });
  const calls = Array.from({ length: 3 }, () => outcome(client.call('system.ping', {}, options)));
  const deadlines = [...timers.timeouts.values()].filter(timer => timer.ms === 3000);
  assert.ok(deadlines.length > 0);
  for (const timer of deadlines) timer.fire();
  networkErrors(await bounded(Promise.all(calls)), /connection timeout/i);
  timers.assertEmpty();
});

test('syntactically valid non-envelope JSON and unknown IDs retain existing ignored behavior', async t => {
  const timers = trackTimers(t);
  const f = await fixture(t, (socket, request) => {
    socket.write('null\n[]\ntrue\n42\n"text"\n{}\n{"id":3,"status":"ok"}\n' + JSON.stringify({ id: 'unknown', status: 'error', error: 'unmatched' }) + '\n');
    reply(socket, request, 'valid');
  });
  assert.equal(await bounded(f.client.call('system.ping', {}, options)), 'valid');
  assert.equal(f.client.pending.size, 0);
  timers.assertEmpty();
});

test('serialization failure leaves no pending entry/timer and connection remains usable', async t => {
  const timers = trackTimers(t);
  const f = await fixture(t, reply);
  const circular = {}; circular.self = circular;
  const failure = await outcome(f.client.call('system.ping', circular, options));
  assert.ok(failure.error instanceof TypeError);
  assert.equal(f.client.pending.size, 0);
  timers.assertEmpty();
  await bounded(f.client.call('system.ping', {}, options));
  assert.equal(f.requests.length, 1);
  timers.assertEmpty();
});

test('synchronous write failure clears pending/timer and cannot double-settle', async t => {
  const timers = trackTimers(t);
  const f = await fixture(t, reply);
  await f.client.connect();
  const write = t.mock.method(f.client.socket, 'write', () => { throw new Error('fixture write failure'); });
  const result = await outcome(f.client.call('system.ping', {}, options));
  networkErrors([result], /write failed/i);
  assert.equal(f.client.pending.size, 0);
  timers.assertEmpty();
  write.mock.restore();
  await bounded(f.client.call('system.ping', {}, options));
  timers.assertEmpty();
});
