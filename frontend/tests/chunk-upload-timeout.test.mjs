import assert from 'node:assert/strict';
import { getEventListeners } from 'node:events';
import { readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import test from 'node:test';
import vm from 'node:vm';

const ts = createRequire(import.meta.url)('typescript');
const source = readFileSync(new URL('../src/services/api.ts', import.meta.url), 'utf8');
const compiled = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
}).outputText;
const MiB = 1024 * 1024;
const WINDOW = 30_000;

// Drive the actual API method with a monotonic clock and XHR events. No real
// network, wall-clock waits, browser globals, or additional test framework.
class Clock {
  now = 0;
  nextId = 1;
  timers = new Map();

  setTimeout = (callback, delay) => {
    const id = this.nextId++;
    this.timers.set(id, { at: this.now + delay, callback });
    return id;
  };

  clearTimeout = (id) => { this.timers.delete(id); };

  nextDue(target) {
    return [...this.timers].filter(([, timer]) => timer.at <= target)
      .sort((a, b) => a[1].at - b[1].at || a[0] - b[0])[0];
  }

  advance(ms) {
    const target = this.now + ms;
    for (let next; (next = this.nextDue(target));) {
      const [id, timer] = next;
      this.now = timer.at;
      this.timers.delete(id);
      timer.callback();
    }
    this.now = target;
  }

  // Model a background tab whose callbacks run late, without pretending that
  // overdue callbacks actually ran at their requested deadlines.
  delay(ms) { this.now += ms; }

  runDelayedTimers() {
    for (let next; (next = this.nextDue(this.now));) {
      const [id, timer] = next;
      this.timers.delete(id);
      timer.callback();
    }
  }
}

class Target {
  listeners = new Map();

  addEventListener(type, listener) {
    if (!this.listeners.has(type)) this.listeners.set(type, new Set());
    this.listeners.get(type).add(listener);
  }

  removeEventListener(type, listener) { this.listeners.get(type)?.delete(listener); }

  emit(type, fields = {}) {
    for (const listener of [...(this.listeners.get(type) || [])]) {
      listener.call(this, { type, target: this, ...fields });
    }
  }

  get listenerCount() {
    return [...this.listeners.values()].reduce((sum, values) => sum + values.size, 0);
  }
}

function harness(options = {}) {
  const clock = new Clock();
  const requests = [];
  class XHR extends Target {
    static DONE = 4;
    upload = new Target();
    status = 0;
    responseText = '';
    readyState = 0;
    abortCalls = 0;
    sent = false;
    headers = {};

    constructor() { super(); requests.push(this); }

    open(method, url) {
      if (options.openThrows) throw new Error('open failed');
      this.method = method;
      this.url = url;
      this.readyState = 1;
    }

    setRequestHeader(name, value) { this.headers[name] = value; }

    send(body) {
      if (options.sendThrows) throw new Error('send failed');
      this.sent = true;
      this.body = body;
    }

    abort() {
      this.abortCalls++;
      this.readyState = 0;
      if (options.abortEmits !== false) this.emit('abort');
    }

    progress(loaded, total = 5 * MiB + 512, lengthComputable = true) {
      this.upload.emit('progress', { loaded, total, lengthComputable });
    }

    uploadComplete(total = 5 * MiB + 512) {
      this.progress(total, total);
      this.upload.emit('load');
      this.upload.emit('loadend');
    }

    respond(status = 200, responseText = 'OK') {
      this.status = status;
      this.responseText = responseText;
      this.readyState = 4;
      this.emit('load');
      this.emit('loadend');
    }
  }
  const context = {
    exports: {},
    require(id) { assert.equal(id, '../stores/auth'); return {}; },
    XMLHttpRequest: XHR,
    FormData,
    File,
    Blob,
    Error,
    performance: { now: () => clock.now },
    setTimeout: clock.setTimeout,
    clearTimeout: clock.clearTimeout,
  };
  vm.runInNewContext(compiled, context, { filename: 'api.ts' });
  const { api } = context.exports;
  api.getCredential = () => 'test-credential';
  api.handleResponseError = (status) => {
    if (status === 401) throw new Error('认证失效，请重新登录');
    if (status === 403) throw new Error('没有权限执行此操作');
  };
  return {
    api, clock, requests,
    start({ size = 5 * MiB, index = 0, uploadId = 'upload-task', signal } = {}) {
      const promise = api.uploadChunkWithSignal(uploadId, index, new Blob([new Uint8Array(size)]), signal);
      const outcome = { kind: 'pending' };
      // Attach the rejection handler immediately, including for synchronous
      // setup failure and already-aborted signals.
      promise.then(
        () => { outcome.kind = 'success'; },
        (error) => { outcome.kind = 'error'; outcome.error = error; },
      );
      return { promise, outcome, xhr: requests.at(-1) };
    },
    assertClean(signal) {
      assert.equal(clock.timers.size, 0, 'all watchdog timers were cleared');
      for (const xhr of requests) {
        assert.equal(xhr.listenerCount, 0, 'main XHR handlers were removed');
        assert.equal(xhr.upload.listenerCount, 0, 'upload handlers were removed');
      }
      if (signal) assert.equal(getEventListeners(signal, 'abort').length, 0, 'abort listener was removed');
    },
  };
}

async function pending(upload) {
  await Promise.resolve();
  assert.equal(upload.outcome.kind, 'pending');
}

// Fetch response decoding and the upload queue cross several Promise turns.
// Yield one actual event-loop turn without advancing the simulated clock.
const flushAsync = () => new Promise((resolve) => setImmediate(resolve));

function observe(promise) {
  const outcome = { kind: 'pending' };
  promise.then(
    (value) => { outcome.kind = 'success'; outcome.value = value; },
    (error) => { outcome.kind = 'error'; outcome.error = error; },
  );
  return { promise, outcome };
}

test('a slow 5 MiB upload meeting every window can run for over 30 seconds', async () => {
  const state = harness();
  const upload = state.start();
  assert.equal(upload.xhr.url, '/upload/chunk');
  assert.equal(upload.xhr.method, 'POST');
  assert.equal(upload.xhr.headers.Authorization, 'Bearer test-credential');
  assert.equal(upload.xhr.body.get('uploadId'), 'upload-task');
  assert.equal(upload.xhr.body.get('chunkIndex'), '0');
  assert.equal(upload.xhr.body.get('chunk').size, 5 * MiB);
  for (let window = 1; window <= 4; window++) {
    state.clock.advance(WINDOW - 1);
    upload.xhr.progress(window * MiB);
    state.clock.advance(1);
    await pending(upload);
    assert.equal(upload.xhr.abortCalls, 0);
  }
  state.clock.advance(20_000);
  upload.xhr.uploadComplete();
  await pending(upload);
  state.clock.advance(20_000);
  upload.xhr.respond();
  await upload.promise;
  state.assertClean();
});

test('exactly 1 MiB meets a window, but one byte below times out', async () => {
  for (const bytes of [MiB, MiB - 1]) {
    const state = harness();
    const upload = state.start();
    state.clock.advance(WINDOW - 1);
    upload.xhr.progress(bytes);
    state.clock.advance(1);
    if (bytes === MiB) {
      await pending(upload);
      upload.xhr.uploadComplete();
      upload.xhr.respond();
      await upload.promise;
    } else {
      await assert.rejects(upload.promise, /上传超时/);
      assert.equal(upload.xhr.abortCalls, 1);
    }
    state.assertClean();
  }
});

test('no progress and trickled progress both fail the fixed first window', async () => {
  for (const trickle of [false, true]) {
    const state = harness();
    const upload = state.start();
    for (let i = 1; i <= 5; i++) {
      state.clock.advance(5_000);
      if (trickle) upload.xhr.progress(i * 1024);
    }
    state.clock.advance(5_000);
    await assert.rejects(upload.promise, /上传超时/);
    assert.equal(state.clock.now, WINDOW, 'progress did not restart the observation deadline');
    state.assertClean();
  }
});

test('each window uses new transmitted bytes rather than cumulative credit', async () => {
  const state = harness();
  const upload = state.start();
  state.clock.advance(WINDOW - 1);
  upload.xhr.progress(2 * MiB);
  state.clock.advance(1);
  await pending(upload);
  state.clock.advance(WINDOW);
  await assert.rejects(upload.promise, /上传超时/);
  state.assertClean();
});

test('unknown upload total still counts actual monotonic progress bytes', async () => {
  const state = harness();
  const upload = state.start();
  state.clock.advance(WINDOW - 1);
  upload.xhr.progress(MiB, 0, false);
  upload.xhr.progress(MiB - 100, 0, false);
  upload.xhr.progress(MiB, 0, false);
  state.clock.advance(1);
  await pending(upload);
  upload.xhr.uploadComplete();
  upload.xhr.respond();
  await upload.promise;
  state.assertClean();
});

test('a final chunk smaller than 1 MiB gets a separate response wait after sending', async () => {
  const state = harness();
  const upload = state.start({ size: MiB / 2, index: 4 });
  state.clock.advance(10_000);
  upload.xhr.uploadComplete(MiB / 2 + 512);
  state.clock.advance(25_000);
  await pending(upload);
  upload.xhr.respond();
  await upload.promise;
  assert.equal(upload.xhr.abortCalls, 0);
  state.assertClean();
});

test('the remaining fraction of a large chunk can finish without sending a whole MiB', async () => {
  const state = harness();
  const upload = state.start();
  state.clock.advance(WINDOW - 1);
  upload.xhr.progress(4.5 * MiB);
  state.clock.advance(1);
  state.clock.advance(25_000);
  upload.xhr.uploadComplete();
  await pending(upload);
  upload.xhr.respond();
  await upload.promise;
  state.assertClean();
});

test('full reported payload size alone is not request-body completion', async () => {
  const state = harness();
  const upload = state.start();
  state.clock.advance(WINDOW - 1);
  upload.xhr.progress(5 * MiB, 5 * MiB + 512);
  state.clock.advance(1);
  await pending(upload);
  state.clock.advance(WINDOW);
  await assert.rejects(upload.promise, /上传超时/);
  state.assertClean();
});

test('body completion never counts as server confirmation, even with a 200 status visible', async () => {
  const state = harness();
  const upload = state.start({ size: 512 });
  state.clock.advance(5_000);
  upload.xhr.uploadComplete(1024);
  upload.xhr.status = 200;
  upload.xhr.readyState = 2;
  state.clock.advance(WINDOW - 1);
  await pending(upload);
  state.clock.advance(1);
  await assert.rejects(upload.promise, /服务端确认.*超时/);
  state.assertClean();
});

test('response deadline begins at body completion, including uploads longer than 30 seconds', async () => {
  const state = harness();
  const upload = state.start();
  for (let i = 1; i <= 2; i++) {
    state.clock.advance(WINDOW - 1);
    upload.xhr.progress(i * MiB);
    state.clock.advance(1);
  }
  state.clock.advance(5_000);
  upload.xhr.uploadComplete();
  state.clock.advance(WINDOW - 1);
  await pending(upload);
  state.clock.advance(1);
  await assert.rejects(upload.promise, /服务端确认.*超时/);
  state.assertClean();
});

test('late timer uses actual elapsed time and reanchors the next observation', async () => {
  const state = harness();
  const upload = state.start();
  state.clock.delay(60_000);
  upload.xhr.progress(2 * MiB);
  state.clock.runDelayedTimers();
  await pending(upload);
  state.clock.advance(WINDOW - 1);
  upload.xhr.progress(3 * MiB);
  state.clock.advance(1);
  await pending(upload);
  upload.xhr.uploadComplete();
  upload.xhr.respond();
  await upload.promise;
  state.assertClean();
});

test('late timers do not treat a long interval as only 30 seconds', async () => {
  const state = harness();
  const upload = state.start();
  state.clock.delay(60_000);
  upload.xhr.progress(2 * MiB - 1);
  state.clock.runDelayedTimers();
  await assert.rejects(upload.promise, /上传超时/);
  state.assertClean();
});

test('late timers never invent progress for an inactive request', async () => {
  const state = harness();
  const upload = state.start();
  state.clock.delay(90_000);
  state.clock.runDelayedTimers();
  await assert.rejects(upload.promise, /上传超时/);
  state.assertClean();
});

for (const phase of ['before_send', 'sending', 'response']) {
  test(`user cancellation during ${phase} rejects and clears all listeners and timers`, async () => {
    const state = harness({ abortEmits: phase !== 'before_send' });
    const controller = new AbortController();
    if (phase === 'before_send') controller.abort();
    const upload = state.start({ signal: controller.signal });
    if (phase === 'response') upload.xhr.uploadComplete();
    if (phase !== 'before_send') controller.abort();
    await assert.rejects(upload.promise, /上传已取消/);
    if (phase === 'before_send') assert.equal(upload.xhr.sent, false);
    state.assertClean(controller.signal);
    upload.xhr.respond();
    await Promise.resolve();
    assert.equal(upload.outcome.kind, 'error', 'late response cannot overwrite cancellation');
  });
}

test('cancellation settles even if the transport does not emit an abort event', async () => {
  const state = harness({ abortEmits: false });
  const controller = new AbortController();
  const upload = state.start({ signal: controller.signal });
  controller.abort();
  await assert.rejects(upload.promise, /上传已取消/);
  state.assertClean(controller.signal);
});

for (const [event, message] of [['error', /网络/], ['abort', /中断|超时|取消/], ['timeout', /超时/]]) {
  test(`transport ${event} cannot resolve success and clears resources`, async () => {
    const state = harness();
    const upload = state.start();
    upload.xhr.emit(event);
    await assert.rejects(upload.promise, message);
    state.assertClean();
  });
}

for (const status of [400, 401, 403, 500]) {
  test(`HTTP ${status} after a complete upload remains a failure`, async () => {
    const state = harness();
    const upload = state.start();
    upload.xhr.uploadComplete();
    upload.xhr.respond(status, 'server rejected the chunk');
    await assert.rejects(upload.promise, /server rejected|认证失效|没有权限/);
    state.assertClean();
  });
}

test('a timed-out request cannot alter its retry or another concurrent request', async () => {
  const state = harness();
  const first = state.start({ index: 0 });
  const concurrent = state.start({ index: 1 });
  state.clock.advance(WINDOW - 1);
  concurrent.xhr.progress(MiB);
  state.clock.advance(1);
  await assert.rejects(first.promise, /上传超时/);
  await pending(concurrent);
  const retry = state.start({ index: 0 });
  first.xhr.uploadComplete();
  first.xhr.respond();
  await Promise.resolve();
  assert.equal(first.outcome.kind, 'error');
  await pending(retry);
  state.clock.advance(20_000);
  concurrent.xhr.uploadComplete();
  concurrent.xhr.respond();
  retry.xhr.uploadComplete();
  retry.xhr.respond();
  await Promise.all([concurrent.promise, retry.promise]);
  state.assertClean();
});

for (const failure of ['openThrows', 'sendThrows']) {
  test(`synchronous ${failure} cleans up without a later timeout`, async () => {
    const state = harness({ [failure]: true });
    const controller = new AbortController();
    const upload = state.start({ signal: controller.signal });
    await assert.rejects(upload.promise, /failed/);
    state.assertClean(controller.signal);
    const cleanupAbortCalls = upload.xhr.abortCalls;
    state.clock.advance(3 * WINDOW);
    assert.equal(upload.xhr.abortCalls, cleanupAbortCalls, 'no delayed timer aborts the cleaned transport');
  });
}

test('observed body completion wins over an overdue upload timer and still requires confirmation', async () => {
  const state = harness();
  const upload = state.start();
  // A suspended browser cannot reconstruct throughput in historical 30-second
  // windows. If completion is observed first, only confirmation remains pending.
  state.clock.delay(3 * WINDOW);
  upload.xhr.uploadComplete();
  state.clock.runDelayedTimers();
  await pending(upload);
  assert.equal(upload.xhr.abortCalls, 0);
  state.clock.advance(WINDOW - 1);
  await pending(upload);
  upload.xhr.respond();
  await upload.promise;
  state.assertClean();
});

test('uploadMap preserves a timed-out upload ID and resumeUpload sends only missing chunks before merge', async () => {
  const state = harness();
  const calls = [];
  let confirmMerge;
  state.api.post = async (path, data) => {
    calls.push({ path, data });
    if (path === '/upload/init') return Response.json({ uploadId: 'retained-upload' });
    if (path === '/upload/status') return Response.json({ uploadedChunks: [0] });
    if (path === '/upload/merge') return new Promise((resolve) => { confirmMerge = resolve; });
    assert.fail(`unexpected endpoint: ${path}`);
  };
  const file = new File([new Uint8Array(5 * MiB + 1)], 'map.zip');
  const initialized = [];
  const upload = observe(state.api.uploadMap(file, undefined, undefined, (id) => initialized.push(id)));
  await flushAsync();
  assert.equal(state.requests.length, 2);
  assert.equal(state.requests[0].body.get('chunkIndex'), '0');
  assert.equal(state.requests[1].body.get('chunkIndex'), '1');
  state.clock.advance(5_000);
  state.requests[0].uploadComplete();
  state.requests[0].respond();
  await flushAsync();
  state.clock.advance(WINDOW - 5_000);
  const result = await upload.promise;
  assert.equal(result.success, false);
  assert.equal(result.uploadId, 'retained-upload');
  assert.match(result.error, /上传超时/);
  assert.deepEqual(initialized, ['retained-upload']);
  assert.deepEqual(calls.map(({ path }) => path), ['/upload/init']);
  state.assertClean();

  const resumed = observe(state.api.resumeUpload(result.uploadId, file));
  await flushAsync();
  assert.equal(state.requests.length, 3, 'resume sends exactly one missing chunk');
  const retry = state.requests[2];
  assert.equal(retry.body.get('uploadId'), 'retained-upload');
  assert.equal(retry.body.get('chunkIndex'), '1');
  assert.equal(retry.body.get('chunk').size, 1);
  // A late response from the timed-out attempt cannot complete its retry.
  state.requests[1].uploadComplete(513);
  state.requests[1].respond();
  await pending(resumed);
  retry.uploadComplete(513);
  await pending(resumed);
  assert.equal(calls.some(({ path }) => path === '/upload/merge'), false);
  retry.respond();
  await flushAsync();
  assert.deepEqual(calls.map(({ path }) => path), ['/upload/init', '/upload/status', '/upload/merge']);
  assert.equal(calls[1].data.uploadId, 'retained-upload');
  assert.equal(calls[2].data.uploadId, 'retained-upload');
  await pending(resumed);
  confirmMerge(new Response('上传成功'));
  await resumed.promise;
  state.assertClean();
});

test('cancellation during initialization cleans the returned upload ID without sending chunks', async () => {
  const state = harness();
  const controller = new AbortController();
  const calls = [];
  let finishInit;
  state.api.post = async (path, data) => {
    calls.push({ path, data });
    if (path === '/upload/init') return new Promise((resolve) => { finishInit = resolve; });
    if (path === '/upload/cancel') return new Response('已取消');
    assert.fail(`unexpected endpoint: ${path}`);
  };
  const file = new File([new Uint8Array(512)], 'map.zip');
  const initialized = [];
  const upload = observe(state.api.uploadMap(file, undefined, controller.signal, (id) => initialized.push(id)));
  controller.abort();
  assert.equal(state.requests.length, 0);
  finishInit(Response.json({ uploadId: 'cancel-during-init' }));
  await assert.rejects(upload.promise, /上传已取消/);
  assert.equal(state.requests.length, 0);
  assert.deepEqual(initialized, []);
  assert.deepEqual(calls.map(({ path }) => path), ['/upload/init', '/upload/cancel']);
  assert.equal(calls[1].data.uploadId, 'cancel-during-init');
  state.assertClean(controller.signal);
});

test('active cancellation stops the queued chunk and a same-name retry uses its own upload ID', async () => {
  const state = harness();
  const controller = new AbortController();
  const calls = [];
  let initCount = 0;
  state.api.post = async (path, data) => {
    calls.push({ path, data });
    if (path === '/upload/init') return Response.json({ uploadId: ++initCount === 1 ? 'cancelled-upload' : 'retry-upload' });
    if (path === '/upload/cancel') return new Response('已取消');
    if (path === '/upload/merge') return new Response('上传成功');
    assert.fail(`unexpected endpoint: ${path}`);
  };
  const file = new File([new Uint8Array(15 * MiB + 1)], 'same-name.zip');
  const initialized = [];
  const cancelled = observe(state.api.uploadMap(file, undefined, controller.signal, (id) => initialized.push(id)));
  await flushAsync();
  assert.equal(state.requests.length, 3, 'the existing queue starts three requests concurrently');
  controller.abort();
  await assert.rejects(cancelled.promise, /上传已取消/);
  assert.equal(state.requests.length, 3, 'the fourth queued chunk never starts');
  state.assertClean(controller.signal);
  // The panel separately invokes cancellation after aborting active requests.
  await state.api.cancelUpload(initialized[0]);
  const retried = observe(state.api.uploadMap(file, undefined, undefined, (id) => initialized.push(id)));
  await flushAsync();
  assert.deepEqual(initialized, ['cancelled-upload', 'retry-upload']);
  assert.equal(state.requests.length, 6);
  for (const stale of state.requests.slice(0, 3)) {
    stale.uploadComplete();
    stale.respond();
  }
  await pending(retried);
  assert.equal(state.requests.length, 6, 'late cancelled events cannot drain the retry queue');
  const completed = new Set();
  for (let round = 0; round < 10 && completed.size < 4; round++) {
    for (const xhr of state.requests.slice(3)) {
      if (completed.has(xhr)) continue;
      assert.equal(xhr.body.get('uploadId'), 'retry-upload');
      xhr.uploadComplete(xhr.body.get('chunk').size + 512);
      xhr.respond();
      completed.add(xhr);
    }
    await flushAsync();
  }
  assert.equal(completed.size, 4, 'the queued fourth request starts after a retry slot becomes available');
  assert.equal((await retried.promise).success, true);
  assert.deepEqual(state.requests.slice(3).map((xhr) => xhr.body.get('chunkIndex')), ['0', '1', '2', '3']);
  const cancellations = calls.filter(({ path }) => path === '/upload/cancel');
  assert.equal(cancellations.length, 1);
  assert.equal(cancellations[0].data.uploadId, 'cancelled-upload');
  const merges = calls.filter(({ path }) => path === '/upload/merge');
  assert.equal(merges.length, 1);
  assert.equal(merges[0].data.uploadId, 'retry-upload');
  state.assertClean(controller.signal);
});
