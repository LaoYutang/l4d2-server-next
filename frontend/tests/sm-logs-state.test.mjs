import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import test from 'node:test';
import vm from 'node:vm';

const ts = createRequire(import.meta.url)('typescript');
const component = readFileSync(new URL('../src/views/Logs.vue', import.meta.url), 'utf8');
const script = component.match(/<script setup lang="ts">([\s\S]*?)<\/script>/)?.[1];
assert.ok(script, 'the actual Logs.vue setup script exists');
const code = ts.transpileModule(`${script}
  globalThis.state = {
    selectFile, clearSelectedLog, closeLogStream, loadFileList, togglePause,
    clearDisplay, toggleLogExpanded, handleViewportChange, scrollToBottom,
    selectedFile, logLines, isPaused, isLogExpanded, isMobileView, isViewerFocused,
    logError, listError, isLoading, categories, logContainer, shouldAutoScroll,
  };
`, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } }).outputText;

const deferred = () => {
  let resolve;
  let reject;
  const promise = new Promise((yes, no) => { resolve = yes; reject = no; });
  return { promise, resolve, reject };
};
const file = (name) => ({ name, category: 'L', size: 10, deletable: false, version: 'fixture' });
const listing = (...names) => ({ installed: true, categories: { L: names.map(file), errors: [], other: [] } });

function harness({ mobile = true, getSourceModLogs = async () => listing('A.log', 'B.log') } = {}) {
  const streams = [];
  const errors = [];
  let mount;
  let unmount;
  const media = { matches: mobile, listener: null, addEventListener(_type, listener) { this.listener = listener; }, removeEventListener() { this.listener = null; } };
  const context = {
    exports: {},
    window: { matchMedia: () => media },
    require(id) {
      if (id === 'vue') return {
        ref: (value) => ({ value }), computed: (getter) => ({ get value() { return getter(); } }),
        onMounted(callback) { mount = callback; }, onUnmounted(callback) { unmount = callback; },
        nextTick: (callback) => Promise.resolve().then(callback), watch() {},
      };
      if (id === '../services/api') return { api: {
        getSourceModLogs,
        streamLog(name, onLine, onError) {
          const stream = { name, onLine, onError, closed: false, close() { this.closed = true; } };
          streams.push(stream);
          return stream;
        },
      } };
      if (id === '../stores/auth') return { useAuthStore: () => ({ isAdmin: true }) };
      if (id === 'ant-design-vue') return { message: { error: (text) => errors.push(text) }, Modal: {} };
      if (id === '@ant-design/icons-vue') return {};
      throw new Error(`Unexpected import: ${id}`);
    },
  };
  vm.runInNewContext(code, context, { filename: 'Logs.vue setup' });
  return { state: context.state, streams, errors, media, mount: () => mount(), unmount: () => unmount() };
}

test('selecting a mobile file focuses its reader without changing desktop expansion state', () => {
  const { state, streams } = harness();
  assert.equal(state.isViewerFocused.value, false);
  state.selectFile('A.log');
  streams[0].onLine('content from A');
  assert.equal(state.selectedFile.value, 'A.log');
  assert.equal(state.logLines.value[0], 'content from A');
  assert.equal(state.isViewerFocused.value, true);
  assert.equal(state.isLogExpanded.value, false);
});

test('desktop keeps the list beside selected content and retains explicit expand/collapse', async () => {
  const { state } = harness({ mobile: false });
  state.selectFile('A.log');
  assert.equal(state.isViewerFocused.value, false);
  state.toggleLogExpanded();
  assert.equal(state.isViewerFocused.value, true);
  state.toggleLogExpanded();
  assert.equal(state.isViewerFocused.value, false);
  state.handleViewportChange({ matches: true });
  assert.equal(state.isViewerFocused.value, true);
  assert.equal(state.isLogExpanded.value, false);
  await Promise.resolve();
});

test('switching files closes the previous stream and rejects its late lines and errors', () => {
  const { state, streams, errors } = harness();
  state.selectFile('A.log');
  streams[0].onLine('A initial');
  state.isPaused.value = true;
  state.selectFile('B.log');
  assert.equal(streams[0].closed, true);
  assert.equal(state.isPaused.value, false);
  assert.equal(state.logLines.value.length, 0);
  streams[0].onLine('A arrived late');
  streams[0].onError('A late failure');
  streams[1].onLine('B current');
  assert.deepEqual(Array.from(state.logLines.value), ['B current']);
  assert.deepEqual(errors, []);
  assert.equal(state.logError.value, '');
});

test('retrying the same filename also isolates the old stream session', () => {
  const { state, streams } = harness();
  state.selectFile('A.log');
  streams[0].onError('connection failed');
  assert.equal(state.logError.value, 'connection failed');
  state.selectFile('A.log');
  assert.equal(state.logError.value, '');
  streams[0].onLine('stale retry content');
  streams[0].onError('stale retry error');
  streams[1].onLine('new retry content');
  assert.deepEqual(Array.from(state.logLines.value), ['new retry content']);
  assert.equal(state.logError.value, '');
});

test('returning to the list clears selection, closes the stream and exits expansion', () => {
  const { state, streams } = harness();
  state.selectFile('A.log');
  state.isLogExpanded.value = true;
  state.isPaused.value = true;
  state.logError.value = 'previous error';
  state.clearSelectedLog();
  streams[0].onLine('late after returning');
  assert.equal(streams[0].closed, true);
  assert.equal(state.selectedFile.value, '');
  assert.equal(state.logLines.value.length, 0);
  assert.equal(state.logError.value, '');
  assert.equal(state.isPaused.value, false);
  assert.equal(state.isLogExpanded.value, false);
  assert.equal(state.isViewerFocused.value, false);
});

test('the existing 1000-line limit, pause/drop and clear-display behavior is preserved', () => {
  const { state, streams } = harness();
  state.selectFile('A.log');
  for (let index = 0; index < 1005; index += 1) streams[0].onLine(`line ${index}`);
  assert.equal(state.logLines.value.length, 1000);
  assert.equal(state.logLines.value[0], 'line 5');
  state.togglePause();
  streams[0].onLine('paused content');
  state.togglePause();
  streams[0].onLine('resumed content');
  assert.equal(state.logLines.value.at(-1), 'resumed content');
  assert.equal(state.logLines.value.includes('paused content'), false);
  state.clearDisplay();
  assert.equal(state.logLines.value.length, 0);
  assert.equal(state.selectedFile.value, 'A.log');
  assert.equal(streams[0].closed, false);
});

test('refreshing an existing file preserves its content and does not open another stream', async () => {
  const { state, streams } = harness();
  state.selectFile('A.log');
  streams[0].onLine('still reading');
  await state.loadFileList();
  assert.equal(state.selectedFile.value, 'A.log');
  assert.deepEqual(Array.from(state.logLines.value), ['still reading']);
  assert.equal(streams.length, 1);
  assert.equal(streams[0].closed, false);
});

test('a missing selected file after refresh restores a usable list even when expanded', async () => {
  const { state, streams } = harness({ getSourceModLogs: async () => listing('B.log') });
  state.selectFile('A.log');
  state.isLogExpanded.value = true;
  await state.loadFileList();
  assert.equal(state.selectedFile.value, '');
  assert.equal(state.isLogExpanded.value, false);
  assert.equal(state.isViewerFocused.value, false);
  assert.equal(streams[0].closed, true);
});

test('an older refresh cannot clear the current selection after a newer list completes', async () => {
  const older = deferred();
  const newer = deferred();
  let calls = 0;
  const { state } = harness({ getSourceModLogs: () => (++calls === 1 ? older.promise : newer.promise) });
  state.selectFile('B.log');
  const firstRead = state.loadFileList();
  const secondRead = state.loadFileList();
  newer.resolve(listing('B.log'));
  await secondRead;
  older.resolve(listing('A.log'));
  await firstRead;
  assert.equal(state.selectedFile.value, 'B.log');
  assert.equal(state.categories.value.L[0].name, 'B.log');
  assert.equal(state.isLoading.value, false);
});

test('list failures remain visible and a successful refresh clears that error', async () => {
  let fails = true;
  const { state, errors } = harness({ getSourceModLogs: async () => {
    if (fails) throw new Error('list temporarily unavailable');
    return listing('A.log');
  } });
  await state.loadFileList();
  assert.equal(state.listError.value, 'list temporarily unavailable');
  assert.deepEqual(errors, ['list temporarily unavailable']);
  fails = false;
  await state.loadFileList();
  assert.equal(state.listError.value, '');
  assert.equal(state.categories.value.L[0].name, 'A.log');
});

test('unmount closes the stream and ignores pending lists, late data and errors', async () => {
  const response = deferred();
  const { state, streams, media, mount, unmount, errors } = harness({ getSourceModLogs: () => response.promise });
  mount();
  assert.equal(typeof media.listener, 'function');
  state.selectFile('A.log');
  unmount();
  response.resolve(listing('A.log'));
  await Promise.resolve();
  streams[0].onLine('late after leaving');
  streams[0].onError('late error after leaving');
  assert.equal(streams[0].closed, true);
  assert.equal(state.logLines.value.length, 0);
  assert.equal(state.categories.value.L.length, 0);
  assert.equal(media.listener, null);
  assert.deepEqual(errors, []);
});
