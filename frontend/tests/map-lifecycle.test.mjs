import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import test from 'node:test';
import vm from 'node:vm';

const ts = createRequire(import.meta.url)('typescript');
const source = (path) => readFileSync(new URL(path, import.meta.url), 'utf8');
const compile = (code) => ts.transpileModule(code, {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
}).outputText;
const plain = (value) => JSON.parse(JSON.stringify(value));
const deferred = () => {
  let resolve;
  let reject;
  const promise = new Promise((yes, no) => { resolve = yes; reject = no; });
  return { promise, resolve, reject };
};
const item = (name, locked = false, revision = 'v1') => ({
  name, locked, revision, id: name, uploaded_at: '2026-11-12T04:00:00Z', size: 1024,
  size_label: '1.00KB', missing: false,
});
const result = (overrides = {}) => ({
  deleted: [], skipped_locked: [], skipped_unknown_time: [], skipped_changed: [], pending_cleanup: [], failed: [], ...overrides,
});
const utilityContext = { exports: {} };
vm.runInNewContext(compile(source('../src/utils/mapLifecycle.ts')), utilityContext);
const utilities = utilityContext.exports;

function lifecycle(api) {
  let dispose;
  const context = {
    exports: {},
    require(id) {
      if (id === 'vue') return {
        ref: (value) => ({ value }),
        onBeforeUnmount(callback) { dispose = callback; },
      };
      if (id === '../services/api') return { api };
      if (id === '../utils/mapLifecycle') return utilities;
      throw new Error(`Unexpected import ${id}`);
    },
  };
  vm.runInNewContext(compile(source('../src/composables/useMapLifecycle.ts')), context);
  return { state: context.exports.useMapLifecycle(), dispose: () => dispose() };
}

test('older list responses cannot replace a newer list or prune its selection', async () => {
  const first = deferred();
  const second = deferred();
  let calls = 0;
  const { state } = lifecycle({ getMapLifecycleList: () => (++calls === 1 ? first.promise : second.promise) });
  state.selectedRowKeys.value = ['new.vpk'];
  const oldRead = state.loadMaps();
  const newRead = state.loadMaps();
  second.resolve([item('new.vpk')]);
  await newRead;
  first.resolve([item('old.vpk')]);
  await oldRead;
  assert.equal(state.maps.value[0].name, 'new.vpk');
  assert.deepEqual(plain(state.selectedRowKeys.value), ['new.vpk']);
});

test('lock sends the displayed revision and invalidates an earlier list request', async () => {
  const oldRead = deferred();
  const latestRead = deferred();
  const calls = [];
  let reads = 0;
  const { state } = lifecycle({
    getMapLifecycleList: () => (++reads === 1 ? oldRead.promise : latestRead.promise),
    setMapLocked: async (...args) => { calls.push(args); return item('a.vpk', true, 'v2'); },
  });
  const pendingRead = state.loadMaps();
  const locking = state.setMapLocked(item('a.vpk'), true);
  await Promise.resolve();
  latestRead.resolve([item('a.vpk', true, 'v2')]);
  await locking;
  oldRead.resolve([item('a.vpk', false)]);
  await pendingRead;
  assert.deepEqual(plain(calls), [['a.vpk', true, 'v1']]);
  assert.equal(state.maps.value[0].locked, true);
  assert.equal(state.maps.value[0].revision, 'v2');
});

test('rejected lock is never optimistically applied and reloads the server state', async () => {
  const { state } = lifecycle({
    getMapLifecycleList: async () => [item('a.vpk')],
    setMapLocked: async () => { throw new Error('地图已变化'); },
  });
  state.maps.value = [item('a.vpk')];
  await assert.rejects(state.setMapLocked(item('a.vpk'), true), /地图已变化/);
  assert.equal(state.maps.value[0].locked, false);
  assert.equal(state.lockingMaps.value['a.vpk'], false);
});

test('mixed removal counts protected items separately and retains their selection', async () => {
  const calls = [];
  const { state } = lifecycle({
    getMapLifecycleList: async () => [item('keep.vpk', true), item('failed.vpk')],
    deleteMaps: async (...args) => {
      calls.push(args);
      return result({ deleted: ['delete.vpk'], skipped_locked: ['keep.vpk'], failed: [{ name: 'failed.vpk', error: 'permission denied' }] });
    },
  });
  state.selectedRowKeys.value = ['delete.vpk', 'keep.vpk', 'failed.vpk'];
  await state.removeMaps([...state.selectedRowKeys.value], { 'delete.vpk': 'v1', 'keep.vpk': 'v2' });
  assert.deepEqual(plain(calls), [[['delete.vpk', 'keep.vpk', 'failed.vpk'], { 'delete.vpk': 'v1', 'keep.vpk': 'v2' }]]);
  assert.deepEqual(plain(state.selectedRowKeys.value), ['keep.vpk', 'failed.vpk']);
  assert.equal(state.removalFeedback.value.type, 'warning');
  assert.equal(state.removalFeedback.value.title, '已删除 1 个，保留 1 个，失败 1 个');
});

test('all locked items produce a retained result, never a deletion success', async () => {
  const { state } = lifecycle({
    getMapLifecycleList: async () => [item('keep.vpk', true)],
    clearMaps: async () => result({ skipped_locked: ['keep.vpk'] }),
  });
  await state.removeMaps();
  assert.equal(state.removalFeedback.value.type, 'warning');
  assert.equal(state.removalFeedback.value.title, '已删除 0 个，保留 1 个');
  assert.match(state.removalFeedback.value.description, /没有地图被删除/);
  assert.equal(state.maps.value[0].locked, true);
});

test('network uncertainty refreshes the list without claiming success or clearing retained selection', async () => {
  let refreshed = false;
  const { state } = lifecycle({
    getMapLifecycleList: async () => { refreshed = true; return [item('keep.vpk')]; },
    deleteMaps: async () => { throw new Error('network interrupted'); },
  });
  state.selectedRowKeys.value = ['keep.vpk'];
  await state.removeMaps(['keep.vpk']);
  assert.equal(refreshed, true);
  assert.equal(state.removalFeedback.value.type, 'warning');
  assert.match(state.removalFeedback.value.title, /结果未确认/);
  assert.deepEqual(plain(state.selectedRowKeys.value), ['keep.vpk']);
  assert.equal(state.removing.value, false);
});

test('committed deletions awaiting cleanup are removed from the list but remain a visible warning', async () => {
  const { state } = lifecycle({
    getMapLifecycleList: async () => [],
    deleteMaps: async () => result({ deleted: ['pending.vpk'], pending_cleanup: ['pending.vpk'] }),
  });
  state.maps.value = [item('pending.vpk')];
  state.selectedRowKeys.value = ['pending.vpk'];
  await state.removeMaps(['pending.vpk']);
  assert.equal(state.maps.value.length, 0);
  assert.equal(state.selectedRowKeys.value.length, 0);
  assert.equal(state.removalFeedback.value.type, 'warning');
  assert.equal(state.removalFeedback.value.title, '已删除 1 个，其中待恢复 1 个');
  assert.match(state.removalFeedback.value.description, /后续清理尚未完成/);
  assert.match(utilities.mapDeleteDetails(state.removalFeedback.value.result)[0].reason, /清理待恢复/);
});

test('disposed components ignore late list responses', async () => {
  const pending = deferred();
  const { state, dispose } = lifecycle({ getMapLifecycleList: () => pending.promise });
  const read = state.loadMaps();
  dispose();
  pending.resolve([item('late.vpk')]);
  await read;
  assert.equal(state.maps.value.length, 0);
});

test('confirmation freezes both selected names and revisions before the user clicks OK', async () => {
  const component = source('../src/views/Maps.vue');
  const script = component.match(/<script setup lang="ts">([\s\S]*?)<\/script>/)[1];
  const ast = ts.createSourceFile('Maps.ts', script, ts.ScriptTarget.Latest, true, ts.ScriptKind.TS);
  const statement = ast.statements.find((node) => ts.isVariableStatement(node) &&
    node.declarationList.declarations.some((node) => node.name.getText(ast) === 'confirmDeleteMaps'));
  assert.ok(statement);
  let confirmation;
  const calls = [];
  const context = {
    removing: { value: false },
    lifecycleMaps: { value: [item('a.vpk', false, 'confirmed-version')] },
    Modal: { confirm(options) { confirmation = options; } },
    ExclamationCircleOutlined: {},
    h() {},
    removeMaps: async (...args) => calls.push(args),
  };
  vm.runInNewContext(compile(`${statement.getText(ast)}\nglobalThis.confirm = confirmDeleteMaps;`), context);
  const selected = ['a.vpk'];
  context.confirm(selected);
  selected.push('unexpected.vpk');
  context.lifecycleMaps.value = [item('a.vpk', false, 'later-version')];
  await confirmation.onOk();
  assert.deepEqual(plain(calls), [[['a.vpk'], { 'a.vpk': 'confirmed-version' }]]);
});

test('old map summaries cannot overwrite a replaced map or clear its newer loading state', async () => {
  const script = source('../src/views/Maps.vue').match(/<script setup lang="ts">([\s\S]*?)<\/script>/)[1];
  const ast = ts.createSourceFile('Maps.ts', script, ts.ScriptTarget.Latest, true, ts.ScriptKind.TS);
  const statement = ast.statements.find((node) => ts.isVariableStatement(node) &&
    node.declarationList.declarations.some((node) => node.name.getText(ast) === 'loadMapSummaries'));
  assert.ok(statement);
  const oldResponse = deferred();
  const newResponse = deferred();
  let calls = 0;
  const context = {
    summaryRequests: new Map(), summaryRequestId: 0, summaryDisposed: false,
    lifecycleMaps: { value: [item('a.vpk', false, 'old')] },
    mapSummaries: { value: {} }, mapSummaryLoading: { value: {} },
    api: { getMapSummaries: () => (++calls === 1 ? oldResponse.promise : newResponse.promise) },
  };
  vm.runInNewContext(compile(`${statement.getText(ast)}\nglobalThis.load = loadMapSummaries;`), context);
  const oldRead = context.load(['a.vpk']);
  context.lifecycleMaps.value = [item('a.vpk', false, 'new')];
  const newRead = context.load(['a.vpk']);
  oldResponse.resolve({ 'a.vpk': { title: 'Old campaign' } });
  await oldRead;
  assert.equal(context.mapSummaryLoading.value['a.vpk'], true);
  assert.equal(context.mapSummaries.value['a.vpk'], undefined);
  newResponse.resolve({ 'a.vpk': { title: 'New campaign' } });
  await newRead;
  assert.equal(context.mapSummaries.value['a.vpk'].title, 'New campaign');
  assert.equal(context.mapSummaryLoading.value['a.vpk'], false);
});

test('real API sends structured lifecycle and removal requests', async () => {
  const context = { exports: {}, require: () => ({}) };
  vm.runInNewContext(compile(source('../src/services/api.ts')), context);
  const { api } = context.exports;
  const calls = [];
  api.post = async (path) => { calls.push({ path }); return Response.json([item('a.vpk')]); };
  api.postJson = async (path, body) => { calls.push({ path, body }); return Response.json(result()); };
  await api.getMapLifecycleList();
  await api.setMapLocked('a.vpk', true, 'revision-1');
  await api.deleteMaps(['a.vpk'], { 'a.vpk': 'revision-1' });
  assert.deepEqual(plain(calls), [
    { path: '/maps/list' },
    { path: '/maps/lock', body: { map: 'a.vpk', locked: true, expected_revision: 'revision-1' } },
    { path: '/maps/remove-batch', body: { maps: ['a.vpk'], revisions: { 'a.vpk': 'revision-1' } } },
  ]);
});

function cleanupSetting(api) {
  const script = source('../src/components/settings/MapAutoCleanupSetting.vue')
    .match(/<script setup lang="ts">([\s\S]*?)<\/script>/)[1];
  const context = {
    exports: {},
    defineProps: () => ({ active: false }),
    withDefaults: (props) => props,
    require(id) {
      if (id === 'vue') return {
        ref: (value) => ({ value }), computed: (getter) => ({ get value() { return getter(); } }),
        onBeforeUnmount() {}, watch() {},
      };
      if (id === 'ant-design-vue') return { message: { success() {} } };
      if (id === '@ant-design/icons-vue') return {};
      if (id === '../../services/api') return { api };
      throw new Error(`Unexpected import ${id}`);
    },
  };
  vm.runInNewContext(compile(`${script}\nglobalThis.state = { load, save, enabled, retentionDays, config, loadError, saveError, saved, nextCheck };`), context);
  return context.state;
}
const config = { enabled: false, retention_days: 3, check_time: '03:00', timezone: 'Asia/Shanghai (UTC+08:00)', next_check_at: '2026-11-13T03:00:00+08:00' };

test('failed settings load cannot save the initial default over the server config', async () => {
  let saves = 0;
  const state = cleanupSetting({
    getMapCleanupConfig: async () => { throw new Error('unavailable'); },
    setMapCleanupConfig: async () => { saves += 1; return config; },
  });
  await state.load();
  await state.save();
  assert.equal(saves, 0);
  assert.match(state.loadError.value, /unavailable/);
});

test('settings reject invalid retention days and show the exact server-local next check', async () => {
  const calls = [];
  const state = cleanupSetting({
    getMapCleanupConfig: async () => config,
    setMapCleanupConfig: async (target) => { calls.push(target); return { ...config, ...target }; },
  });
  await state.load();
  assert.equal(state.nextCheck.value, '2026-11-13 03:00:00 UTC+08:00');
  for (const invalid of [null, 0, -1, 1.5, 3651, NaN]) {
    state.retentionDays.value = invalid;
    await state.save();
  }
  assert.equal(calls.length, 0);
  state.retentionDays.value = 7;
  state.enabled.value = true;
  await state.save();
  assert.deepEqual(plain(calls), [{ enabled: true, retention_days: 7 }]);
  assert.equal(state.saved.value, true);
});

test('re-reading after an uncertain save clears its error and shows the server truth', async () => {
  const state = cleanupSetting({
    getMapCleanupConfig: async () => config,
    setMapCleanupConfig: async () => { throw new Error('network interrupted'); },
  });
  await state.load();
  state.enabled.value = true;
  await state.save();
  assert.equal(state.config.value.enabled, false);
  assert.equal(state.saved.value, false);
  assert.match(state.saveError.value, /network interrupted/);
  await state.load();
  assert.equal(state.enabled.value, false);
  assert.equal(state.saveError.value, '');
});

test('historical/invalid times are explicit and deletion details retain every protected reason', () => {
  assert.match(utilities.formatMapUploadTime(null), /历史地图/);
  assert.match(utilities.formatMapUploadTime('invalid'), /异常/);
  assert.equal(utilities.mapDeleteDetails(result({ skipped_locked: ['locked.vpk'], skipped_changed: ['changed.vpk'], skipped_unknown_time: ['old.vpk'], failed: [{ name: 'failed.vpk', error: 'disk failure' }] })).length, 4);
});
