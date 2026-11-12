import assert from 'node:assert/strict';
import { createRequire } from 'node:module';
import test from 'node:test';

// Run against a built app preview with MAP_LAYOUT_BASE_URL=http://127.0.0.1:PORT.
// Playwright is optional for the ordinary Node unit suite. PLAYWRIGHT_MODULE can
// point at an existing installation in a managed browser-test environment.
const baseURL = process.env.MAP_LAYOUT_BASE_URL;
const longName = `${'无空格的超长中文地图名称'.repeat(16)}_${'campaign'.repeat(18)}.vpk`;
const longError = `无法保存文件：/data/${'directory_without_any_spaces_'.repeat(15)}/地图配置.json`;
const config = {
  enabled: false, retention_days: 3, check_time: '03:00',
  timezone: 'Asia/Shanghai (UTC+08:00)', next_check_at: '2026-11-14T03:00:00+08:00',
};

test('actual Ant/Vue layout keeps feedback separated and long content inside narrow screens', {
  skip: !baseURL && 'Set MAP_LAYOUT_BASE_URL to run the browser layout regression',
}, async (t) => {
  const { chromium } = createRequire(import.meta.url)(process.env.PLAYWRIGHT_MODULE || 'playwright');
  const browser = await chromium.launch({
    executablePath: process.env.CHROMIUM_PATH || '/usr/bin/chromium', headless: true,
    args: ['--no-sandbox', '--disable-dev-shm-usage'],
  });
  t.after(() => browser.close());

  for (const width of [320, 768]) {
    await t.test(`${width}px success, errors and protected-map results`, async () => {
      const context = await browser.newContext({ viewport: { width, height: 1000 } });
      try {
        await context.addInitScript(() => {
          localStorage.setItem('server_password', 'browser-layout-fixture');
          localStorage.setItem('server_role', 'admin');
        });
        const page = await context.newPage();
        let loadFails = false;
        let saveFails = false;
        // Every POST is intercepted: these layout checks cannot change a server,
        // save settings, or remove a real map even if pointed at a running app.
        await page.route('**/*', async (route) => {
          const request = route.request();
          if (request.method() !== 'POST') return route.continue();
          const path = new URL(request.url()).pathname;
          if (path === '/maps/cleanup/config') {
            return route.fulfill(loadFails ? { status: 500, body: longError } : { json: config });
          }
          if (path === '/config/map-cleanup') {
            return route.fulfill(saveFails ? { status: 500, body: longError } : { json: { ...config, enabled: true } });
          }
          if (path === '/maps/list') {
            return route.fulfill({ json: [{
              name: longName, id: 'fixture-id', revision: 'fixture-revision', locked: true,
              uploaded_at: null, size: 1000, size_label: '1000 B', missing: false,
            }] });
          }
          if (path === '/maps/summary') return route.fulfill({ json: { items: {} } });
          if (path === '/maps/remove-batch') {
            return route.fulfill({ json: {
              deleted: [], skipped_locked: [longName], skipped_unknown_time: [],
              skipped_changed: [], pending_cleanup: [], failed: [],
            } });
          }
          return route.fulfill({ json: {
            enabled: false, history_enabled: false, version: 'layout-fixture',
            limit_percent: 90, default_percent: 90, min_percent: 80, max_percent: 99,
          } });
        });

        const geometry = (element) => {
          const rect = element.getBoundingClientRect();
          return { top: rect.top, bottom: rect.bottom, left: rect.left, right: rect.right, width: rect.width, height: rect.height };
        };
        const assertPageFits = async () => {
          const sizes = await page.evaluate(() => ({
            viewport: innerWidth, document: document.documentElement.scrollWidth,
            content: document.querySelector('.main-content').scrollWidth,
            contentWidth: document.querySelector('.main-content').clientWidth,
          }));
          assert.ok(sizes.document <= sizes.viewport + 1, JSON.stringify(sizes));
          assert.ok(sizes.content <= sizes.contentWidth + 1, JSON.stringify(sizes));
        };
        const assertSettingFeedback = async (text) => {
          const alert = page.locator('.map-cleanup-setting .ant-alert').filter({ hasText: text });
          await alert.waitFor();
          const alertBox = await alert.evaluate(geometry);
          const buttonBox = await page.getByRole('button', { name: '保存清理设置', exact: true }).evaluate(geometry);
          assert.ok(buttonBox.top - alertBox.bottom >= 11.5, `feedback/button gap=${buttonBox.top - alertBox.bottom}`);
          assert.ok(buttonBox.height >= 39.5, `save target height=${buttonBox.height}`);
          const overflow = await alert.evaluate((element) => element.scrollWidth > element.clientWidth + 1);
          assert.equal(overflow, false, 'long feedback must wrap inside its alert');
          const action = alert.locator('.ant-alert-action button');
          if (await action.count()) assert.ok((await action.evaluate(geometry)).height >= 39.5);
          await assertPageFits();
        };

        await page.goto(`${baseURL}/#/system`);
        await page.getByRole('switch', { name: '启用地图自动清理' }).click();
        await page.getByRole('button', { name: '保存清理设置', exact: true }).click();
        await assertSettingFeedback('配置已保存');

        saveFails = true;
        await page.getByRole('button', { name: '保存清理设置', exact: true }).click();
        await assertSettingFeedback('保存失败');
        if (width === 320) {
          const positions = await page.locator('.map-cleanup-setting .ant-alert').filter({ hasText: '保存失败' }).evaluate((element) => {
            const content = element.querySelector('.ant-alert-content').getBoundingClientRect();
            const action = element.querySelector('.ant-alert-action').getBoundingClientRect();
            const button = element.querySelector('.ant-alert-action button').getBoundingClientRect();
            return { contentBottom: content.bottom, actionTop: action.top, buttonHeight: button.height };
          });
          assert.ok(positions.actionTop - positions.contentBottom >= 11.5);
          assert.ok(positions.buttonHeight >= 39.5);
        }

        loadFails = true;
        await page.reload();
        await assertSettingFeedback('加载地图清理配置失败');

        await page.goto(`${baseURL}/#/maps`);
        await page.locator('.ant-table-thead input[type="checkbox"]').check();
        await page.getByRole('button', { name: /删除选中/ }).click();
        await page.locator('.ant-modal-confirm').getByRole('button', { name: '确认删除', exact: true }).click();
        const result = page.locator('.map-removal-result');
        await result.waitFor();
        await result.locator('summary').click();
        const resultBox = await result.evaluate(geometry);
        const toolbarBox = await page.locator('.maps-toolbar').evaluate(geometry);
        assert.ok(toolbarBox.top - resultBox.bottom >= 15.5, `result/toolbar gap=${toolbarBox.top - resultBox.bottom}`);
        assert.equal(await result.evaluate((element) => element.scrollWidth > element.clientWidth + 1), false);
        const closeBox = await result.locator('.ant-alert-close-icon').evaluate(geometry);
        assert.ok(closeBox.width >= 39.5 && closeBox.height >= 39.5, `close target=${JSON.stringify(closeBox)}`);
        const toolbarTargets = await page.locator('.maps-toolbar button').evaluateAll((buttons) => buttons
          .filter((button) => button.getBoundingClientRect().width > 0)
          .map((button) => ({ height: button.getBoundingClientRect().height, left: button.getBoundingClientRect().left, right: button.getBoundingClientRect().right })));
        for (const target of toolbarTargets) {
          assert.ok(target.height >= 39.5, `toolbar target height=${target.height}`);
          assert.ok(target.left >= -1 && target.right <= width + 1, JSON.stringify(target));
        }
        await assertPageFits();
      } finally {
        await context.close();
      }
    });
  }
});
