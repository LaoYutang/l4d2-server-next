import assert from 'node:assert/strict';
import { createRequire } from 'node:module';
import test from 'node:test';

// Point SELF_SERVICE_BASE_URL at a built frontend preview. All API calls are stubbed.
const baseURL = process.env.SELF_SERVICE_BASE_URL;

test('admin selects the new grant type and a visitor keeps the type actually issued', {
  skip: !baseURL && 'Set SELF_SERVICE_BASE_URL to run the self-service browser regression',
}, async (t) => {
  const { chromium } = createRequire(import.meta.url)(process.env.PLAYWRIGHT_MODULE || 'playwright');
  const browser = await chromium.launch({
    executablePath: process.env.CHROMIUM_PATH || '/usr/bin/chromium',
    args: ['--no-sandbox', '--disable-dev-shm-usage'],
  });
  t.after(() => browser.close());

  const admin = await browser.newPage();
  await admin.addInitScript(() => {
    localStorage.setItem('server_password', 'self-service-test-admin');
    localStorage.setItem('server_role', 'admin');
  });
  const visitor = await browser.newPage();
  let accessType = 'temporary';
  let issued = false;
  const fixture = async (route) => {
    const request = route.request();
    if (request.method() === 'GET') return route.continue();
    switch (new URL(request.url()).pathname) {
      case '/auth-codes/list':
        return route.fulfill({ json: {
          items: [], page: 1, page_size: 20, total: 0, cleanup_count: 0,
          counts: { all: 0, active: 0, expired: 0, revoked: 0 }, server_time: new Date().toISOString(),
        } });
      case '/config/self-service':
        accessType = request.postDataJSON().access_type;
        return route.fulfill({ json: { status: 'ok' } });
      case '/self-service/status':
        return route.fulfill({ json: {
          enabled: true, access_type: accessType, in_cooldown: issued,
          remaining_seconds: issued ? 3600 : 0, last_generated_time: '',
        } });
      case '/self-service/generate':
        issued = true;
        return route.fulfill({ json: {
          code: 'fixture-self-service-code', access_type: accessType,
          expires_at: new Date(Date.now() + 3_600_000).toISOString(),
        } });
      default:
        return route.fulfill({ status: 404 });
    }
  };
  await admin.route('**/*', fixture);
  await visitor.route('**/*', fixture);
  await Promise.all([admin.goto(`${baseURL}/#/auth-codes`), visitor.goto(`${baseURL}/#/login`)]);

  const newGrant = (label) => visitor.getByText(`新领取的授权码为「${label}」，有效期 1 小时。`, { exact: true });
  const selectGrant = async (label, type) => {
    const saved = admin.waitForResponse('**/config/self-service');
    await admin.getByRole('radio', { name: label, exact: true }).click();
    assert.deepEqual((await saved).request().postDataJSON(), { access_type: type });
  };
  await newGrant('临时权限').waitFor();
  await selectGrant('仅地图上传与下载', 'map_upload_only');

  // The visitor opened the page before the admin's change. The issuance response
  // is authoritative even when the visitor's last public status was different.
  const grantResponse = visitor.waitForResponse('**/self-service/generate*');
  await visitor.getByRole('button', { name: '获取授权码', exact: true }).click();
  const request = (await grantResponse).request();
  assert.equal(request.method(), 'POST');
  assert.equal(request.postData(), null);
  assert.equal(new URL(request.url()).search, '');
  const receipt = visitor.getByText('已领取「仅地图上传与下载」授权码', { exact: true });
  await receipt.waitFor();

  const refresh = visitor.getByRole('button', { name: '刷新领取状态' });
  await refresh.click();
  await newGrant('仅地图上传与下载').waitFor();
  await selectGrant('临时权限', 'temporary');
  await refresh.click();
  await newGrant('临时权限').waitFor();
  assert.ok(await receipt.isVisible());
  assert.ok(await visitor.getByText('fixture-self-service-code', { exact: true }).isVisible());
});
