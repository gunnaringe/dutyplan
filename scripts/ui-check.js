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
  if (!process.env.TJENESTEPLAN_SOLVER) throw new Error('TJENESTEPLAN_SOLVER mangler (kjør via mise run app:ui)');
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
    page.on('dialog', (d) => d.accept());  // bekreftelser (sletting m.m.) godtas
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
    const logText = await page.innerText('ul.log');
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
    await page.locator('form:has(input[placeholder^="Ny dagtype"]) button').click();
    await page.waitForSelector('input[aria-label="Plasser for Lyd på Gudstjeneste"]');
    await page.fill('input[aria-label="Plasser for Lyd på Gudstjeneste"]', '1');
    await page.fill('input[aria-label="Plasser for Kafé på Gudstjeneste"]', '3');
    await page.locator('.card:has(input[aria-label="Navn på dagtype"]) button:has-text("Lagre oppgaver")').click();
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

    // --- fase 3: min side ---
    await page.goto(base + '/o/menigheten/p/host-2026');
    await h1is('Høst 2026');
    await page.selectOption('select[aria-label="Velg navnet ditt"]', { label: 'Kari Test' });
    await page.click('text=Åpne min side');
    await h1is('Kari Test');
    await page.click('button[aria-label="Legg til Lyd"]');
    await page.waitForSelector('[aria-label="Hvor gjerne: Lyd"]');
    await page.click('[aria-label="Hvor gjerne: Lyd"] button[data-level="godt"]');
    await page.waitForSelector('[aria-label="Hvor gjerne: Lyd"] button[data-level="godt"][aria-pressed="true"]');
    await page.click('.avail-row:first-child button[data-level="ikke"]');
    await page.waitForSelector('.avail-row:first-child button[data-level="ikke"][aria-pressed="true"]');
    ok((await page.innerText('main')).includes('1 passer ikke'), 'oppsummeringen teller svaret');
    await page.reload();
    await h1is('Kari Test');
    await page.waitForSelector('.avail-row:first-child button[data-level="ikke"][aria-pressed="true"]');
    await page.waitForSelector('[aria-label="Hvor gjerne: Lyd"] button[data-level="godt"][aria-pressed="true"]');
    await page.click('.row-inline button:has-text("Godt")');  // alle på godt; ett svar overskrives, bekreftelsen godtas
    await page.waitForFunction(() => document.querySelectorAll('.avail-row button[data-level="godt"][aria-pressed="true"]').length === document.querySelectorAll('.avail-row').length);
    await page.click('.avail-row:first-child button[data-level="godt"]');  // trykk på valgt svar igjen fjerner det
    await page.waitForSelector('.avail-row:first-child button[aria-pressed="true"]', { state: 'detached' });

    // --- fase 4: planlegg ---
    const kariUrl = page.url().replace(base, '');
    await page.goto(base + '/o/menigheten');
    await page.waitForSelector('input[aria-label="Kan foreslås: Kari Test"]');
    await page.check('input[aria-label="Kan foreslås: Kari Test"]');  // slått av tidligere i testen
    await page.waitForFunction(() => document.querySelector('input[aria-label="Kan foreslås: Kari Test"]').checked && document.body.innerText.includes('true -> false') && document.body.innerText.includes('false -> true'));
    // en gruppe: Ola Test er med i «Koret», som planleggeren kan foreslå til Kafé (1 av 3 plasser)
    await page.goto(base + '/o/menigheten');
    await page.fill('form.row input[placeholder="Ny person"]', 'Ola Test');
    await page.click('text=Legg til');
    await page.waitForSelector('td:has-text("Ola Test")');
    await page.goto(base + '/o/menigheten/oppsett');
    await h1is('Oppsett');
    await page.fill('input[placeholder^="Ny gruppe"]', 'Koret');
    await page.locator('form:has(input[placeholder^="Ny gruppe"]) button').click();
    await page.waitForSelector('input[aria-label="Navn på gruppen"]');
    await page.check('input[aria-label="Ola Test i Koret"]');
    // hver lagring tegner siden på nytt: vent til den er ferdig før neste felt fylles ut
    const saved = async (suffix, button) => {
      await Promise.all([page.waitForResponse((r) => r.url().endsWith(suffix) && r.request().method() === 'PUT'), page.locator('.card:has(input[aria-label="Navn på gruppen"]) button:has-text("' + button + '")').click()]);
      await page.waitForLoadState('networkidle');
    };
    await saved('/members', 'Lagre medlemmer');
    await page.fill('input[aria-label="Plasser Koret fyller på Kafé"]', '1');
    await saved('/tasks', 'Lagre oppgaver');
    await page.reload();
    await page.waitForSelector('input[aria-label="Plasser Koret fyller på Kafé"]');
    ok((await page.inputValue('input[aria-label="Plasser Koret fyller på Kafé"]')) === '1', 'gruppens oppgave ligger igjen etter omlasting');
    ok(await page.isChecked('input[aria-label="Ola Test i Koret"]'), 'gruppens medlem ligger igjen etter omlasting');
    ok(await page.isChecked('input[aria-label="Kan foreslås: Koret"]'), 'en ny gruppe kan foreslås');
    await page.goto(base + '/o/menigheten/p/host-2026');
    await h1is('Høst 2026');
    ok((await page.innerText('main')).includes('Ingen plan er laget ennå'), 'planen er tom før første kjøring');
    // manuelle ruter og utvalg, for flere hendelser om gangen: en gjest på Lyd første søndag, og Kari alene i utvalget for Lyd 16. august
    await page.click('summary:has-text("Ny manuell rute")');
    const addForm = page.locator('form:has(input[placeholder^="Tekst"])');
    await addForm.locator('summary:has-text("Hendelser")').click();
    await addForm.locator('input[aria-label="2026-08-02 Gudstjeneste"]').check();
    await addForm.locator('select[aria-label="Oppgave for ruten"]').selectOption({ label: 'Lyd' });
    await addForm.locator('select[aria-label="Hva settes inn"]').selectOption('text');
    await addForm.locator('input[aria-label="Tekst"]').fill('Gjest utenfra');
    await addForm.locator('button[type=submit]').click();
    await page.waitForSelector('.rule:has-text("Lyd: Gjest utenfra")');
    await page.click('summary:has-text("Nytt utvalg")');
    const poolForm = page.locator('form:has(select[aria-label="Oppgave for utvalget"])');
    await poolForm.locator('summary:has-text("Hendelser")').click();
    await poolForm.locator('input[aria-label="2026-08-16 Gudstjeneste"]').check();
    await poolForm.locator('select[aria-label="Oppgave for utvalget"]').selectOption({ label: 'Lyd' });
    await poolForm.locator('input[aria-label="Utvalg: Kari Test"]').check();
    await poolForm.locator('button[type=submit]').click();
    await page.waitForSelector('.rule:has-text("Utvalg Lyd: Kari Test")');
    await page.click('summary:has-text("Ny manuell rute")');
    await page.locator('form:has(input[placeholder^="Tekst"]) button[type=submit]').click();  // ingen hendelse valgt: forklaring, ikke feil
    await page.waitForFunction(() => document.getElementById('msg').textContent.includes('Velg minst én hendelse'));
    await page.getByRole('button', { name: 'Planlegg', exact: true }).click();
    await page.waitForSelector('.plan-event', { timeout: 20000 });
    let panel = await page.innerText('.plan-panel');
    ok(panel.includes('Kari Test'), 'Kari fikk Lyd: ' + panel);
    ok(panel.includes('Kontrollert uavhengig: ingen brudd'), 'kontrollen ble kjørt: ' + panel);
    ok(panel.includes('Koret (Ola Test)'), 'gruppen er foreslått på Kafé, med medlemmene: ' + panel);
    ok(panel.includes('Mangler 2') && panel.includes('har ikke valgt oppgaven'), 'plasser som ikke kan fylles, forklares: ' + panel);
    ok(panel.includes('Gjest utenfra'), 'den manuelle gjesten står på Lyd: ' + panel);
    ok((await page.locator('.plan-event').nth(0).innerText()).includes('Gjest utenfra') && !(await page.locator('.plan-event').nth(0).innerText()).includes('Kari Test'), 'gjesten fyller plassen første søndag, så Kari får ikke Lyd da');
    ok((await page.locator('.plan-event').count()) === 5, 'en rad per hendelse');
    await page.getByRole('button', { name: 'Prøv igjen' }).click();
    await page.waitForSelector('.plan-panel p:has-text("Planlegger")', { timeout: 3000 }).catch(() => {});  // kan være ferdig før vi rekker å se den
    await page.waitForSelector('button:has-text("Prøv igjen"):not([disabled])', { timeout: 20000 });
    ok((await page.locator('.plan-event').count()) === 5, 'ny plan etter «Prøv igjen»');
    // planen blir utdatert når et svar endres
    await page.goto(base + kariUrl);
    await h1is('Kari Test');
    await page.click('.avail-row:nth-child(2) button[data-level="darlig"]');
    await page.waitForSelector('.avail-row:nth-child(2) button[data-level="darlig"][aria-pressed="true"]');
    await page.goto(base + '/o/menigheten/p/host-2026');
    await page.waitForSelector('.plan-panel .warn');
    ok((await page.innerText('.plan-panel')).includes('endret siden denne planen ble laget'), 'utdatert plan merkes');
    await page.getByRole('button', { name: 'Prøv igjen' }).click();
    await page.waitForSelector('.plan-panel .warn', { state: 'detached', timeout: 20000 });

    // «Lås» gjør et forslag til en manuell rute (og planen blir utdatert); «Fjern» tar den bort igjen
    await page.locator('button[aria-label^="Lås Kari Test"]').first().click();
    await page.waitForSelector('.rule:has-text("Lyd: Kari Test")');
    await page.waitForSelector('.plan-panel .warn');
    await page.click('button[aria-label^="Fjern ruten Lyd Gjest utenfra"]');
    await page.waitForSelector('.rule:has-text("Gjest utenfra")', { state: 'detached' });

    // ingen side skal bli bredere enn en mobilskjerm
    await page.setViewportSize({ width: 390, height: 844 });
    for (const url of ['/o/menigheten', '/o/menigheten/oppsett', '/o/menigheten/p/host-2026', kariUrl]) {
      await page.goto(base + url);
      await page.waitForSelector('h1');
      await page.waitForTimeout(150);
      const w = await page.evaluate(() => [document.documentElement.scrollWidth, document.documentElement.clientWidth]);
      ok(w[0] <= w[1], `siden ${url} er ${w[0]} px bred på en ${w[1]} px skjerm (scroller sideveis)`);
    }

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
