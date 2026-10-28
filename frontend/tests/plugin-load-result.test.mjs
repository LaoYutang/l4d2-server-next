import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import test from 'node:test';
import vm from 'node:vm';

const ts = createRequire(import.meta.url)('typescript');

// Execute the real API methods and the two SFC handlers with browser effects
// replaced by spies. No Vue mount, server connection, or extra test framework.
const apiSource = readFileSync(new URL('../src/services/api.ts', import.meta.url), 'utf8');
const component = readFileSync(new URL('../src/views/Plugins.vue', import.meta.url), 'utf8');
const script = component.match(/<script setup lang="ts">([\s\S]*?)<\/script>/)?.[1];
assert.ok(script, 'Plugins.vue script setup was found');
const scriptAst = ts.createSourceFile('Plugins.ts', script, ts.ScriptTarget.Latest, true, ts.ScriptKind.TS);
const handlerNames = ['hotTogglePlugin', 'loadPluginImmediately'];
const handlers = scriptAst.statements.filter((statement) =>
  ts.isVariableStatement(statement) && statement.declarationList.declarations.some((declaration) =>
    ts.isIdentifier(declaration.name) && handlerNames.includes(declaration.name.text),
  ),
);
assert.equal(handlers.length, handlerNames.length, 'both actual component handlers were found');

function compile(source) {
  return ts.transpileModule(source, {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
  }).outputText;
}

function harness(response) {
  const apiContext = {
    exports: {},
    require(id) {
      assert.equal(id, '../stores/auth');
      return {};
    },
  };
  vm.runInNewContext(compile(apiSource), apiContext, { filename: 'api.ts' });
  const { api, ApiRequestError } = apiContext.exports;
  const requests = [];
  api.post = async (path, data) => {
    requests.push({ path, name: data.name });
    return response.clone();
  };
  const messages = [];
  let confirmation;
  const context = {
    api,
    ApiRequestError,
    AModal: { confirm(options) { confirmation = options; } },
    message: {
      loading() { return () => {}; },
      success(text) { messages.push({ kind: 'success', text }); },
      warning(text) { messages.push({ kind: 'warning', text }); },
      error(text) { messages.push({ kind: 'error', text }); },
    },
    fetchPlugins() {},
  };
  vm.runInNewContext(compile(
    handlers.map((statement) => statement.getText(scriptAst)).join('\n') +
    '\nglobalThis.actions = { hotTogglePlugin, loadPluginImmediately };',
  ), context, { filename: 'Plugins.vue handlers' });
  return {
    requests,
    messages,
    async run(handler) {
      context.actions[handler]({ name: 'Example', status: 'disabled' });
      assert.ok(confirmation, 'user confirmation precedes the operation');
      await confirmation.onOk();
    },
  };
}

for (const [handler, path] of [
  ['hotTogglePlugin', '/plugins/enable-and-load'],
  ['loadPluginImmediately', '/plugins/load'],
]) {
  for (const [code, status, text] of [
    ['plugin_result_unknown', 502, '插件加载结果未确认，已保留部署文件'],
    ['plugin_operation_busy', 409, '插件操作正在进行，请稍后重试'],
  ]) {
    test(`${handler}: ${code} shows a warning, never success or failure`, async () => {
      const state = harness(Response.json({ error: code, message: text }, { status }));
      await state.run(handler);
      assert.deepEqual(state.requests, [{ path, name: 'Example' }]);
      assert.deepEqual(state.messages, [{ kind: 'warning', text }]);
    });
  }

  test(`${handler}: confirmed success shows success`, async () => {
    const state = harness(Response.json({ message: '插件加载成功' }));
    await state.run(handler);
    assert.equal(state.messages.length, 1);
    assert.equal(state.messages[0].kind, 'success');
  });

  test(`${handler}: ordinary backend errors retain the failure message`, async () => {
    const state = harness(new Response('目标插件无法加载', { status: 500 }));
    await state.run(handler);
    assert.equal(state.messages.length, 1);
    assert.equal(state.messages[0].kind, 'error');
    assert.match(state.messages[0].text, /失败: 目标插件无法加载$/);
  });
}
