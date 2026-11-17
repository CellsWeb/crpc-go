import assert from 'node:assert/strict';
import net from 'node:net';
import readline from 'node:readline';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { GoRpcClient, RpcError } from '../dist/src/client.js';

export const options = { timeoutMs: 10_000 };
export const outcome = promise => promise.then(value => ({ value }), error => ({ error }));
export async function bounded(promise, ms = 1000) {
  let timer;
  try {
    return await Promise.race([promise, new Promise((_, reject) => { timer = setTimeout(() => reject(new Error('Client did not settle promptly')), ms); })]);
  } finally { clearTimeout(timer); }
}
export async function until(predicate) {
  const end = Date.now() + 1000;
  while (!predicate()) {
    if (Date.now() >= end) throw new Error('Condition did not become true');
    await new Promise(resolve => setTimeout(resolve, 5));
  }
}
export function networkErrors(results, message) {
  for (const result of results) {
    assert.ok(result.error instanceof RpcError, String(result.error));
    assert.equal(result.error.code, 'network_error');
    if (message) assert.match(result.error.message, message);
  }
}
export function trackTimers(t) {
  const timeouts = new Map(), intervals = new Set();
  const realSetTimeout = globalThis.setTimeout, realClearTimeout = globalThis.clearTimeout;
  const realSetInterval = globalThis.setInterval, realClearInterval = globalThis.clearInterval;
  t.mock.method(globalThis, 'setTimeout', (callback, ms, ...args) => {
    let timer;
    const fire = () => { realClearTimeout(timer); timeouts.delete(timer); callback(...args); };
    timer = realSetTimeout(fire, ms);
    if (ms === 3000 || ms === options.timeoutMs) timeouts.set(timer, { ms, fire });
    return timer;
  });
  t.mock.method(globalThis, 'clearTimeout', timer => { timeouts.delete(timer); realClearTimeout(timer); });
  t.mock.method(globalThis, 'setInterval', (callback, ms, ...args) => {
    const timer = realSetInterval(callback, ms, ...args); intervals.add(timer); return timer;
  });
  t.mock.method(globalThis, 'clearInterval', timer => { intervals.delete(timer); realClearInterval(timer); });
  t.after(() => {
    for (const timer of timeouts.keys()) realClearTimeout(timer);
    for (const timer of intervals) realClearInterval(timer);
  });
  return { timeouts, intervals, assertEmpty() { assert.equal(timeouts.size, 0, 'uncleared request/connect timers'); assert.equal(intervals.size, 0, 'uncleared connection polling intervals'); } };
}
export function cleanupClient(client) {
  client.close();
  // Baseline lacks teardown rejection. Ensure failing red tests cannot leave
  // 10-second timers behind; callers already have outcome handlers attached.
  for (const pending of client.pending.values()) { clearTimeout(pending.timer); pending.reject(new Error('Test cleanup')); }
  client.pending.clear();
}
export async function fixture(t, onRequest) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'ipc007-'));
  const socketPath = path.join(dir, 'rpc.sock');
  const sockets = new Set(), requests = [];
  let connections = 0;
  const server = net.createServer(socket => {
    const generation = ++connections;
    sockets.add(socket);
    const lines = readline.createInterface({ input: socket });
    socket.on('error', () => {});
    socket.on('close', () => { sockets.delete(socket); lines.close(); });
    lines.on('line', line => {
      const request = JSON.parse(line);
      requests.push({ request, socket, generation });
      onRequest?.(socket, request, generation, requests);
    });
  });
  await new Promise((resolve, reject) => { server.once('error', reject); server.listen(socketPath, resolve); });
  const client = new GoRpcClient({ socketPath });
  t.after(async () => {
    cleanupClient(client);
    for (const socket of sockets) socket.destroy();
    await new Promise(resolve => server.close(resolve));
    fs.rmSync(dir, { recursive: true, force: true });
  });
  return { client, requests, server, socketPath, connections: () => connections };
}
export const reply = (socket, request, data) => socket.write(JSON.stringify({ id: request.id, status: 'ok', data }) + '\n');
