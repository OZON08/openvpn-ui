// Playwright screenshot capture for docs/images.
// Runs a headless Chromium, logs in with the creds in the env, walks the UI
// and writes PNGs to docs/images (filenames match the README references).
//
// Usage (from docs/screenshots/):
//   npm install
//   UI_URL=http://localhost:8080 UI_USER=admin UI_PASS=secret node capture.mjs
//
// Or drop an .env file next to this script (see .env.example).

import { chromium } from 'playwright';
import { mkdir } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import { dirname, resolve } from 'node:path';
import { readFileSync, existsSync } from 'node:fs';

const __dirname = dirname(fileURLToPath(import.meta.url));
const outDir = resolve(__dirname, '..', 'images');
await mkdir(outDir, { recursive: true });

// Minimal .env loader so this script has no extra deps beyond playwright.
const envFile = resolve(__dirname, '.env');
if (existsSync(envFile)) {
  for (const line of readFileSync(envFile, 'utf8').split('\n')) {
    const m = line.match(/^\s*([A-Z_][A-Z0-9_]*)\s*=\s*(.*)\s*$/);
    if (m && !(m[1] in process.env)) process.env[m[1]] = m[2].replace(/^["']|["']$/g, '');
  }
}

const UI_URL = process.env.UI_URL || 'http://localhost:8080';
const UI_USER = process.env.UI_USER;
const UI_PASS = process.env.UI_PASS;
if (!UI_USER || !UI_PASS) {
  console.error('Set UI_USER and UI_PASS (env or docs/screenshots/.env).');
  process.exit(1);
}

// Each view produces one PNG. `tab` clicks a Monitor tab before shooting.
const views = [
  { file: 'OpenVPN-UI-Home.png',                  path: '/' },
  { file: 'OpenVPN-UI-Certs.png',                 path: '/certificates' },
  { file: 'OpenVPN-UI-Monitor-Sessions.png',      path: '/monitor' },
  { file: 'OpenVPN-UI-Monitor-Users.png',         path: '/monitor', tab: '#tab-users' },
  { file: 'OpenVPN-UI-Monitor-Retention.png',     path: '/monitor', tab: '#tab-retention' },
  { file: 'OpenVPN-UI-Monitor-InfluxDB.png',      path: '/monitor', tab: '#tab-influx' },
  { file: 'OpenVPN-UI-Logs.png',                  path: '/logs' },
  { file: 'OpenVPN-UI-Settings.png',              path: '/settings' },
  { file: 'OpenVPN-UI-Server-config.png',         path: '/ov/config' },
  { file: 'OpenVPN-UI-ClientConf.png',            path: '/ov/clientconfig' },
  { file: 'OpenVPN-UI-EasyRsaVars.png',           path: '/easyrsa/config' },
  { file: 'OpenVPN-UI-Maintenance.png',           path: '/dangerzone' },
  { file: 'OpenVPN-UI-Profile.png',               path: '/profile' },
];

const browser = await chromium.launch();
const context = await browser.newContext({
  viewport: { width: 1400, height: 900 },
  deviceScaleFactor: 2,
});
const page = await context.newPage();

// Login screen first — capture before we submit.
await page.goto(`${UI_URL}/login`);
await page.waitForLoadState('networkidle');
await page.screenshot({ path: resolve(outDir, 'OpenVPN-UI-Login.png') });
console.log('  wrote OpenVPN-UI-Login.png');

await page.fill('input[name="login"]', UI_USER);
await page.fill('input[name="password"]', UI_PASS);
await Promise.all([
  page.waitForURL(url => !url.pathname.startsWith('/login'), { timeout: 10_000 }),
  page.click('button[type="submit"]'),
]);

for (const v of views) {
  await page.goto(`${UI_URL}${v.path}`);
  await page.waitForLoadState('networkidle');
  if (v.tab) {
    await page.click(`a[href="${v.tab}"]`);
    await page.waitForTimeout(350); // tab fade-in
  }
  await page.screenshot({ path: resolve(outDir, v.file), fullPage: true });
  console.log(`  wrote ${v.file}`);
}

await browser.close();
console.log(`\nDone. ${views.length + 1} screenshots in ${outDir}`);
