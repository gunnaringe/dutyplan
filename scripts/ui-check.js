// Nettlesersjekk av frontenden mot en midlertidig server. Krever playwright-core og en Chromium i hurtigbufferen:
// PLAYWRIGHT_CORE (standard ../../scoreboard/node_modules/playwright-core) og CHROME (standard første chromium-* i ~/.cache/ms-playwright).
// Kjør: mise run app:ui   (bygger først). Skriver én linje ved suksess.
const fs = require('fs'), os = require('os'), path = require('path');
const { chromium } = require(process.env.PLAYWRIGHT_CORE || path.join(__dirname, '../../scoreboard/node_modules/playwright-core'));
const chromeDir = () => {
  const root = path.join(os.homedir(), '.cache/ms-playwright');
  const d = fs.readdirSync(root).filter((n) => /^chromium-\d+$/.test(n)).sort().pop();
  return path.join(root, d, 'chrome-linux64', 'chrome');
};
const { spawn } = require('child_process');

(async () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'tp-ui-'));
  const srv = spawn(path.join(__dirname, '../bin/tjenesteplan'), ['--addr', '127.0.0.1:0', '--db', path.join(dir, 'ui.db')], { stdio: ['ignore', 'pipe', 'pipe'] });
  let base = '';
  await new Promise((res, rej) => {
    srv.stdout.on('data', (d) => { const m = /http:\/\/\S+/.exec(String(d)); if (m && !base) { base = m[0]; res(); } });
    setTimeout(() => rej(new Error('server start timeout')), 5000);
  });
  const errors = [];
  const notFound = [];
  let browser;
  try {
    browser = await chromium.launch({ executablePath: process.env.CHROME || chromeDir(), args: ['--no-sandbox'] });
    const page = await browser.newPage();
    page.on('response', (r) => { if (r.status() === 404) notFound.push(new URL(r.url()).pathname); });
    page.on('pageerror', (e) => errors.push('pageerror: ' + e.message));
    page.on('console', (m) => { if (m.type() === 'error') errors.push('console: ' + m.text()); });
    const ok = (cond, what) => { if (!cond) throw new Error('FEIL: ' + what); };
    const h1is = (t) => page.waitForFunction((x) => document.querySelector('h1') && document.querySelector('h1').textContent === x, t, { timeout: 4000 }).catch(() => { throw new Error('FEIL: overskriften ble aldri «' + t + '»'); });

    await page.goto(base + '/');
    await page.waitForSelector('h1');
    ok((await page.textContent('h1')) === 'Velg organisasjon', 'forsiden viser organisasjonsvalg');

    // uten navn: skriving avvises med en forklaring
    await page.fill('form.row input', 'Menigheten');
    await page.click('form.row button');
    ok((await page.textContent('#msg')).includes('Skriv hvem du er'), 'avvist uten navn');

    await page.fill('#actor', 'Ola Nordmann');
    await page.dispatchEvent('#actor', 'change');
    await page.click('form.row button');
    await page.waitForURL('**/o/menigheten');
    await h1is('Menigheten');

    await page.fill('form.row input[placeholder^="Ny plan"]', 'Høst 2026');
    await page.click('text=Opprett plan');
    await page.waitForURL('**/o/menigheten/p/host-2026');
    await h1is('Høst 2026');

    await page.click('text=← Menigheten');
    await page.waitForSelector('h2:has-text("Personer")');
    await page.fill('form.row input[placeholder="Ny person"]', 'Kari Test');
    await page.click('text=Legg til');
    await page.waitForSelector('td:has-text("Kari Test")');
    await page.uncheck('input[aria-label="Kan foreslås: Kari Test"]');
    await page.waitForFunction(() => document.body.innerText.includes('person.update'));
    const logText = await page.innerText('table.log');
    ok(logText.includes('Ola Nordmann') && logText.includes('proposable true -> false'), 'endringsloggen viser hvem og hva: ' + logText);

    // bytte organisasjon: ny organisasjon, deretter tilbake via valgfeltet
    await page.goto(base + '/?bytt');
    await page.fill('form.row input', 'Annen org');
    await page.click('form.row button');
    await page.waitForURL('**/o/annen-org');
    await h1is('Annen org');
    ok((await page.innerText('main')).includes('Ingen planer ennå'), 'ny organisasjon er tom');
    await page.selectOption('header select', 'menigheten');
    await page.waitForURL('**/o/menigheten');
    await h1is('Menigheten');
    ok((await page.innerText('main')).includes('Høst 2026'), 'planene følger organisasjonen');
    const before = errors.length;  // ukjent organisasjon gir 404 fra API-et, og det er riktig
    await page.goto(base + '/o/finnes-ikke');
    await h1is('Fant ikke siden');
    errors.length = before;

    // --- fase 2: oppsett og datoer ---
    await page.goto(base + '/o/menigheten/oppsett');
    await h1is('Oppsett');
    await page.fill('input[placeholder="Ny oppgave"]', 'Lyd');
    await page.getByRole('button', { name: 'Legg til', exact: true }).click();
    await page.waitForSelector('td:has-text("Lyd")');
    await page.fill('textarea[aria-label^="Flere oppgaver"]', 'Bilde\nKafé\nLyd');
    await page.getByRole('button', { name: 'Legg til alle' }).click();
    await page.waitForSelector('td:has-text("Kafé")');
    await page.fill('input[placeholder^="Ny dagtype"]', 'Gudstjeneste');
    await page.getByRole('button', { name: 'Opprett', exact: true }).click();
    await page.waitForSelector('input[aria-label="Plasser for Lyd på Gudstjeneste"]');
    await page.fill('input[aria-label="Plasser for Lyd på Gudstjeneste"]', '1');
    await page.fill('input[aria-label="Plasser for Kafé på Gudstjeneste"]', '3');
    await page.click('text=Lagre oppgaver');
    await page.selectOption('select[aria-label="Bilde og Lyd"]', 'kan_ikke');
    await page.reload();
    await h1is('Oppsett');
    await page.waitForSelector('input[aria-label="Plasser for Kafé på Gudstjeneste"]');
    ok((await page.inputValue('input[aria-label="Plasser for Kafé på Gudstjeneste"]')) === '3', 'plasser ligger igjen etter omlasting');
    ok((await page.inputValue('select[aria-label="Bilde og Lyd"]')) === 'kan_ikke', 'kombinasjonen ligger igjen');

    await page.goto(base + '/o/menigheten/p/host-2026');
    await h1is('Høst 2026');
    await page.fill('input[aria-label="Fra"]', '2026-08-01');
    await page.fill('input[aria-label="Til"]', '2026-08-31');
    await page.getByRole('button', { name: 'Legg til alle' }).click();
    await page.waitForFunction(() => document.querySelectorAll('select[aria-label^="Dagtype 2026"]').length === 5);
    ok((await page.textContent('#msg')).includes('5 nye hendelser'), 'fem søndager i august 2026');
    // en ekstra hendelse samme dag er bare en hendelse til
    await page.fill('input[aria-label="Dato"]', '2026-08-09');
    await page.fill('input[aria-label="Merknad"]', 'kveld');
    await page.click('text=Legg til hendelse');
    await page.waitForFunction(() => document.querySelectorAll('select[aria-label^="Dagtype 2026-08-09"]').length === 2);
    await page.click('button[aria-label^="Slett hendelsen 2026-08-30"]');
    await page.waitForFunction(() => document.querySelectorAll('select[aria-label^="Dagtype 2026"]').length === 5);

    ok(errors.length === 0, 'konsollfeil: ' + errors.join(' | ') + ' 404 på: ' + notFound.join(', '));
    console.log('ui-check: ok');
  } catch (e) {
    console.log(String(e.message || e), errors.length ? '\n' + errors.join('\n') : '');
    process.exitCode = 1;
  } finally {
    if (browser) await browser.close();
    srv.kill('SIGTERM');
    fs.rmSync(dir, { recursive: true, force: true });
  }
})();
