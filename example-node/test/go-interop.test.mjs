import assert from 'node:assert/strict';
import { spawn, execFileSync } from 'node:child_process';
import { once } from 'node:events';
import fs from 'node:fs';
import net from 'node:net';
import os from 'node:os';
import path from 'node:path';
import readline from 'node:readline';
import { fileURLToPath } from 'node:url';
import { before, after, test } from 'node:test';
import { GoRpcClient } from '../dist/src/client.js';
import { bounded, cleanupClient, networkErrors, options, outcome, trackTimers, until } from './helpers.mjs';

const repo = fileURLToPath(new URL('../../', import.meta.url));
let binaryDir, binary;
before(() => {
  binaryDir = fs.mkdtempSync(path.join(os.tmpdir(), 'ipc007-bin-'));
  binary = path.join(binaryDir, 'daemon');
  const args = ['build', '-o', binary];
  if (process.env.CRPC_IPC_TEST_RACE === '1') args.push('-race');
  args.push('./example-node/test/testdata');
  execFileSync('go', args, { cwd: repo, env: { ...process.env, GOWORK: 'off' }, stdio: 'pipe' });
});
after(() => { if (binaryDir) fs.rmSync(binaryDir, { recursive: true, force: true }); });

async function daemonAt(t, socketPath) {
  const process = spawn(binary, [socketPath], { env: { ...globalThis.process.env, GORACE: 'atexit_sleep_ms=0' }, stdio: ['ignore', 'pipe', 'pipe'] });
  let stdout = '', stderr = '', stopped = false;
  process.stdout.on('data', chunk => { stdout += chunk; });
  process.stderr.on('data', chunk => { stderr += chunk; });
  const exited = new Promise(resolve => {
    process.once('close', resolve); // Wait for stdout/stderr to drain as well.
    process.once('error', error => { stderr += String(error); resolve(); });
  });
  async function stop(signal = 'SIGTERM') {
    if (!stopped) { stopped = true; process.kill(signal); }
    await bounded(exited, 3000);
    assert.ok(!stderr.includes('DATA RACE'), stderr);
  }
  t.after(() => stop());
  const end = Date.now() + 3000;
  while (true) {
    const ready = await new Promise(resolve => {
      const socket = net.createConnection(socketPath);
      socket.once('connect', () => { socket.destroy(); resolve(true); });
      socket.once('error', () => { socket.destroy(); resolve(false); });
    });
    if (ready) break;
    if (Date.now() > end) throw new Error('Go daemon did not listen: ' + stderr);
    await new Promise(resolve => setTimeout(resolve, 10));
  }
  return { stop, stdout: () => stdout };
}
async function live(t) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'ipc007-go-'));
  const socketPath = path.join(dir, 'rpc.sock');
  // Register directory cleanup before daemon startup; daemon cleanup runs first
  // explicitly below so no live daemon/socket is removed underneath another test.
  const daemon = await daemonAt(t, socketPath);
  const client = new GoRpcClient({ socketPath });
  t.after(async () => { cleanupClient(client); await daemon.stop(); fs.rmSync(dir, { recursive: true, force: true }); });
  return { daemon, client, socketPath };
}

test('actual Go bridge: concurrent arbitrary JSON values, normal errors and authorization', async t => {
  const timers = trackTimers(t);
  const f = await live(t);
  const values = [{ object: 1 }, [[1, 2], [null, true]], 'string', 1.25, true, false, null];
  const calls = values.map(value => f.client.call('system.ping', value, options));
  // Go's existing BridgeResponse data,omitempty omits a nil result. Preserve
  // that legacy undefined result; an explicitly present data:null stays null.
  assert.deepEqual(await bounded(Promise.all(calls)), [...values.slice(0, -1), undefined]);
  assert.deepEqual(await f.client.call('public.hello', { name: 'Node' }, options), { greeting: 'Hello, Node!' });
  const denied = await outcome(f.client.call('user.getProfile', { id: 'fixture' }, options));
  assert.equal(denied.error.code, 'rpc_error'); // Existing Go bridge has no code field.
  assert.match(denied.error.message, /Denied/);
  assert.deepEqual(await f.client.call('user.getProfile', { id: 'fixture' }, { ...options, headers: { authorization: 'Bearer valid' } }), { id: 'fixture' });
  assert.equal(f.client.pending.size, 0);
  timers.assertEmpty();
});

test('actual Go anonymous malformed-frame error promptly rejects all active calls and reconnects', async t => {
  const timers = trackTimers(t);
  const f = await live(t);
  const socket = await f.client.connect();
  socket.write('{invalid}\n');
  const calls = Array.from({ length: 3 }, () => outcome(f.client.call('system.ping', {}, options)));
  networkErrors(await bounded(Promise.all(calls)), /Invalid JSON/);
  assert.equal(f.client.pending.size, 0);
  timers.assertEmpty();
  assert.deepEqual(await bounded(f.client.call('public.hello', { name: 'Reconnect' }, options)), { greeting: 'Hello, Reconnect!' });
  assert.notEqual(f.client.socket, socket);
  timers.assertEmpty();
});

test('actual Go oversized newline frame rejects concurrent calls without timeout and permits reconnect', async t => {
  const timers = trackTimers(t);
  const f = await live(t);
  const calls = [outcome(f.client.call('system.ping', { message: 'x'.repeat(3 * 1024 * 1024) }, options)), outcome(f.client.call('system.ping', {}, options)), outcome(f.client.call('system.ping', {}, options))];
  networkErrors(await bounded(Promise.all(calls), 2000));
  assert.equal(f.client.pending.size, 0);
  timers.assertEmpty();
  assert.deepEqual(await f.client.call('system.ping', { message: 'after-limit' }, options), { message: 'after-limit' });
  timers.assertEmpty();
});

test('actual Go daemon termination rejects concurrent callers and reconnects after daemon restart', async t => {
  const timers = trackTimers(t);
  const f = await live(t);
  const calls = [outcome(f.client.call('system.ping', { message: 'hold' }, options)), outcome(f.client.call('system.ping', {}, options)), outcome(f.client.call('system.ping', {}, options))];
  await until(() => f.daemon.stdout().includes('HOLD') && f.client.pending.size === 3);
  await f.daemon.stop('SIGKILL');
  networkErrors(await bounded(Promise.all(calls)), /closed|socket/i);
  timers.assertEmpty();
  const restarted = await daemonAt(t, f.socketPath);
  assert.deepEqual(await f.client.call('system.ping', { message: 'restarted' }, options), { message: 'restarted' });
  timers.assertEmpty();
  await restarted.stop();
});

test('Go protocol evidence: malformed JSON is anonymous/recoverable; newline and length frames remain compatible', async t => {
  const f = await live(t);
  const socket = net.createConnection(f.socketPath);
  socket.on('error', () => {});
  await once(socket, 'connect');
  const lines = readline.createInterface({ input: socket });
  t.after(() => { lines.close(); socket.destroy(); });
  const exchange = async wire => {
    const response = once(lines, 'line');
    socket.write(wire, () => {});
    return JSON.parse((await bounded(response))[0]);
  };
  const malformed = await exchange('{invalid}\n');
  assert.equal(malformed.id, '');
  assert.equal(malformed.status, 'error');
  assert.match(malformed.error, /Invalid JSON/);
  const lineRequest = { id: 'line', function: 'system.ping', data: [[null, true], { value: 1 }] };
  assert.deepEqual(await exchange(JSON.stringify(lineRequest) + '\n'), { id: 'line', status: 'ok', data: lineRequest.data });
  const lengthRequest = { id: 'length', function: 'system.ping', data: { text: '✓', nested: [1, false] } };
  const payload = Buffer.from(JSON.stringify(lengthRequest));
  const header = Buffer.alloc(4); header.writeUInt32BE(payload.length);
  assert.deepEqual(await exchange(Buffer.concat([header, payload])), { id: 'length', status: 'ok', data: lengthRequest.data });
  const oversized = Buffer.alloc(4); oversized.writeUInt32BE(2 * 1024 * 1024 + 1);
  const error = await exchange(oversized);
  assert.equal(error.id, '');
  assert.equal(error.status, 'error');
  assert.match(error.error, /exceeds limit/);
  await bounded(once(socket, 'close'));
});

for (const corruption of ['invalid-json-line', 'truncated-eof']) {
  test(`actual Go response through fault-injecting proxy: ${corruption} fails callers and reconnects without replay`, async t => {
    const timers = trackTimers(t);
    t.mock.method(console, 'error', () => {});
    const f = await live(t);
    const proxyPath = path.join(path.dirname(f.socketPath), 'proxy.sock');
    const peers = new Set(), requests = [], responses = [];
    let generation = 0;
    const proxy = net.createServer(downstream => {
      const current = ++generation;
      const upstream = net.createConnection(f.socketPath);
      const input = readline.createInterface({ input: downstream });
      const output = readline.createInterface({ input: upstream });
      peers.add(downstream); peers.add(upstream);
      for (const socket of [downstream, upstream]) socket.on('error', () => {});
      input.on('line', line => requests.push({ generation: current, request: JSON.parse(line) }));
      downstream.pipe(upstream);
      let corrupted = false;
      output.on('line', line => {
        const response = JSON.parse(line); // Actual unchanged Go bridge must emit valid JSON.
        responses.push(response);
        if (current === 1) {
          if (corrupted) return;
          corrupted = true;
          if (corruption === 'truncated-eof') downstream.end(line.slice(0, -1));
          else downstream.write('{corrupted-response}\n' + line + '\n');
        } else downstream.write(line + '\n');
      });
      downstream.once('close', () => { input.close(); output.close(); upstream.destroy(); });
      upstream.once('close', () => { output.close(); downstream.end(); });
    });
    await new Promise((resolve, reject) => { proxy.once('error', reject); proxy.listen(proxyPath, resolve); });
    const client = new GoRpcClient({ socketPath: proxyPath });
    t.after(async () => {
      cleanupClient(client);
      for (const peer of peers) peer.destroy();
      await new Promise(resolve => proxy.close(resolve));
    });
    const calls = Array.from({ length: 3 }, (_, index) => outcome(client.call('system.ping', { index }, options)));
    networkErrors(await bounded(Promise.all(calls)), /IPC stream corruption.*unparsable response/i);
    assert.equal(client.pending.size, 0);
    assert.equal(client.socket, null);
    assert.equal(client.rl, null);
    timers.assertEmpty();
    assert.equal(responses[0].status, 'ok');
    assert.deepEqual(responses[0].data, { index: 0 });
    assert.deepEqual(await bounded(client.call('public.hello', { name: 'After corruption' }, options)), { greeting: 'Hello, After corruption!' });
    await until(() => requests.length === 4);
    assert.deepEqual(requests.map(item => item.generation), [1, 1, 1, 2]);
    assert.equal(new Set(requests.map(item => item.request.id)).size, 4);
    assert.equal(client.pending.size, 0);
    timers.assertEmpty();
  });
}
