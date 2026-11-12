import assert from 'node:assert/strict';
import { createRequire } from 'node:module';
import test from 'node:test';

// Use an isolated built app containing L20261112.log (200+ lines, including
// SM-MOBILE-END and a long line), errors_20261112.log (SM-MOBILE-SWITCH), and
// empty.log. This regression only reads logs; it never calls delete/cleanup.
const baseURL = process.env.SM_LOGS_BASE_URL;
const password = process.env.SM_LOGS_PASSWORD;

test('selected SourceMod logs remain readable across mobile and desktop layouts', {
  skip: (!baseURL || !password) && 'Set SM_LOGS_BASE_URL and SM_LOGS_PASSWORD for the isolated browser fixture',
}, async (t) => {
  const { chromium } = createRequire(import.meta.url)(process.env.PLAYWRIGHT_MODULE || 'playwright');
  const browser = await chromium.launch({
    executablePath: process.env.CHROMIUM_PATH || '/usr/bin/chromium',
    headless: true, args: ['--no-sandbox', '--disable-dev-shm-usage'],
  });
  t.after(() => browser.close());

  for (const [width, height, theme] of [[1440, 900, 'light'], [390, 844, 'light'], [320, 740, 'light'], [768, 900, 'light'], [390, 844, 'dark']]) {
    await t.test(`${width}px ${theme}: content, navigation, scroll, empty and failed streams`, async () => {
      const context = await browser.newContext({ viewport: { width, height }, isMobile: width < 1024, hasTouch: width < 1024 });
      try {
        await context.addInitScript(({ password, theme }) => {
          localStorage.setItem('server_password', password);
          localStorage.setItem('server_role', 'admin');
          localStorage.setItem('l4d2_manager_theme', theme);
        }, { password, theme });
        const page = await context.newPage();
        const unexpectedErrors = [];
        page.on('pageerror', (error) => unexpectedErrors.push(error.message));
        await page.goto(`${baseURL}/#/logs`);
        const select = async (name) => {
          if (width < 1024 && await page.getByRole('button', { name: '返回列表', exact: true }).isVisible()) {
            await page.getByRole('button', { name: '返回列表', exact: true }).click();
          }
          await page.getByText(name, { exact: true }).click();
        };
        const fits = async () => {
          const size = await page.evaluate(() => {
            const main = document.querySelector('.main-content');
            return { viewport: innerWidth, document: document.documentElement.scrollWidth, width: main.clientWidth, scroll: main.scrollWidth };
          });
          assert.ok(size.document <= size.viewport + 1 && size.scroll <= size.width + 1, JSON.stringify(size));
        };
        const readable = async () => {
          const output = page.locator('.log-output');
          await output.waitFor();
          // Wait for the focus/sidebar transition before measuring the reader.
          await page.waitForTimeout(350);
          const rect = await output.evaluate((el) => {
            const r = el.getBoundingClientRect();
            return { top: r.top, bottom: r.bottom, height: r.height, viewport: innerHeight };
          });
          assert.ok(rect.height >= 200 && rect.top >= 0 && rect.bottom <= rect.viewport + 1, JSON.stringify(rect));
          await fits();
          if (width < 1024) {
            const target = await page.getByRole('button', { name: '返回列表', exact: true }).boundingBox();
            assert.ok(target.height >= 40 && target.y >= 0 && target.y + target.height <= height);
          }
        };

        // Ant's reset gives alerts margin:0; spacing belongs to the sidebar's
        // flex gap so a failed refresh cannot attach feedback to the file list.
        await page.getByText('L20261112.log', { exact: true }).waitFor();
        await page.route('**/logs/list', (route) => route.fulfill({ status: 503, body: '隔离测试：日志列表暂时不可用' }));
        await page.getByRole('button', { name: '刷新列表', exact: true }).click();
        await page.locator('.log-list-error').waitFor();
        const gap = await page.locator('.log-sidebar').evaluate((el) => {
          const error = el.querySelector('.log-list-error').getBoundingClientRect();
          const list = el.querySelector('.log-file-list').getBoundingClientRect();
          return list.top - error.bottom;
        });
        assert.ok(gap >= 11.5, `list feedback gap=${gap}`);
        await fits();
        await page.unroute('**/logs/list');
        await page.getByRole('button', { name: '刷新列表', exact: true }).click();
        await page.locator('.log-list-error').waitFor({ state: 'detached' });

        await select('L20261112.log');
        await page.locator('.log-output').getByText(/SM-MOBILE-END/).waitFor();
        await readable();
        const scrolling = await page.locator('.log-output').evaluate((el) => {
          el.scrollTop = 0;
          const vertical = el.scrollHeight > el.clientHeight && el.scrollTop === 0;
          el.scrollLeft = 200;
          const horizontal = el.scrollLeft > 0;
          el.scrollTop = el.scrollHeight;
          return { vertical, horizontal, atBottom: el.scrollTop + el.clientHeight >= el.scrollHeight - 1 };
        });
        assert.deepEqual(scrolling, { vertical: true, horizontal: true, atBottom: true });
        await select('errors_20261112.log');
        await page.locator('.log-output').getByText(/SM-MOBILE-SWITCH/).waitFor();
        assert.equal(await page.locator('.log-output').getByText(/SM-MOBILE-END/).count(), 0);
        await readable();

        await select('empty.log');
        await page.getByText('当前日志为空，等待新内容...', { exact: true }).waitFor();
        await readable();

        let failStream = true;
        await page.route('**/logs/stream?file=errors_20261112.log', (route) => failStream
          ? route.fulfill({ status: 503, body: '隔离测试：日志服务暂时不可用' }) : route.continue());
        await select('errors_20261112.log');
        await page.locator('.log-stream-error').getByText('日志读取失败', { exact: true }).waitFor();
        await fits();
        failStream = false;
        await page.getByRole('button', { name: /重\s*试/ }).click();
        await page.locator('.log-output').getByText(/SM-MOBILE-SWITCH/).waitFor();
        assert.equal(await page.locator('.log-stream-error').count(), 0);
        await readable();

        if (width < 1024) await page.getByRole('button', { name: '返回列表', exact: true }).click();
        await page.getByRole('button', { name: '刷新列表', exact: true }).click();
        await select('L20261112.log');
        await page.locator('.log-output').getByText(/SM-MOBILE-END/).waitFor();
        await readable();
        await page.reload();
        await page.getByText('L20261112.log', { exact: true }).waitFor();
        assert.equal(await page.locator('.log-output').getByText(/SM-MOBILE-END/).count(), 0);
        await select('errors_20261112.log');
        await page.locator('.log-output').getByText(/SM-MOBILE-SWITCH/).waitFor();
        await readable();
        assert.deepEqual(unexpectedErrors, []);
      } finally { await context.close(); }
    });
  }

  await t.test('568×320 short landscape keeps files and reader reachable by scrolling', async () => {
    const context = await browser.newContext({ viewport: { width: 568, height: 320 }, isMobile: true, hasTouch: true });
    try {
      await context.addInitScript((password) => {
        localStorage.setItem('server_password', password);
        localStorage.setItem('server_role', 'admin');
      }, password);
      const page = await context.newPage();
      await page.goto(`${baseURL}/#/logs`);
      // A row clipped beneath the list header used to remain unclickable even
      // when Playwright attempted to scroll it into view.
      await page.getByText('L20261112.log', { exact: true }).click({ timeout: 5000 });
      await page.locator('.log-output').getByText(/SM-MOBILE-END/).waitFor();
      await page.waitForTimeout(350);
      await page.locator('.log-output').scrollIntoViewIfNeeded();
      const reader = await page.locator('.log-output').evaluate((el) => {
        const r = el.getBoundingClientRect();
        return { height: r.height, visibleHeight: Math.min(r.bottom, innerHeight) - Math.max(r.top, 64) };
      });
      assert.ok(reader.height >= 100 && reader.visibleHeight >= 80, JSON.stringify(reader));
      await page.getByRole('button', { name: '返回列表', exact: true }).click();
      await page.getByText('errors_20261112.log', { exact: true }).click({ timeout: 5000 });
      await page.locator('.log-output').getByText(/SM-MOBILE-SWITCH/).waitFor();
      const width = await page.evaluate(() => ({ viewport: innerWidth, page: document.documentElement.scrollWidth }));
      assert.ok(width.page <= width.viewport + 1);
    } finally { await context.close(); }
  });
});
