// Browser check of the frontend against a temporary server, in English first and then in Norwegian. Needs playwright-core and a
// Chromium in the cache: PLAYWRIGHT_CORE (default ../../scoreboard/node_modules/playwright-core) and CHROME (default: the first
// chromium-* in ~/.cache/ms-playwright). Run: mise run app:ui   (builds first). Prints one line on success.
const fs = require('fs'), os = require('os'), path = require('path');
const { chromium } = require(process.env.PLAYWRIGHT_CORE || path.join(__dirname, '../../scoreboard/node_modules/playwright-core'));
const chromeDir = () => {
  const root = path.join(os.homedir(), '.cache/ms-playwright');
  const d = fs.readdirSync(root).filter((n) => /^chromium-\d+$/.test(n)).sort().pop();
  return path.join(root, d, 'chrome-linux64', 'chrome');
};
const { spawn } = require('child_process');

(async () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'dp-ui-'));
  if (!process.env.DUTYPLAN_SOLVER) throw new Error('DUTYPLAN_SOLVER is not set (run via mise run app:ui)');
  const srv = spawn(path.join(__dirname, '../bin/dutyplan'), ['--addr', '127.0.0.1:0', '--db', path.join(dir, 'ui.db')], { stdio: ['ignore', 'pipe', 'pipe'] });
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
    // The language is pinned to English until the check switches it itself.
    const context = await browser.newContext({ locale: 'en-GB' });
    await context.addInitScript(() => { try { if (!localStorage.getItem('tp.lang')) localStorage.setItem('tp.lang', 'en'); } catch (e) { /* ignore */ } });
    const page = await context.newPage();
    const watch = (p) => {
      p.on('response', (r) => { if (r.status() === 404) notFound.push(new URL(r.url()).pathname); });
      p.on('dialog', (d) => d.accept());  // confirmations (deleting and so on) are accepted
      p.on('pageerror', (e) => errors.push('pageerror: ' + e.message));
      p.on('console', (m) => { if (m.type() === 'error') errors.push('console: ' + m.text()); });
    };
    watch(page);
    const ok = (cond, what) => { if (!cond) throw new Error('FAIL: ' + what); };
    const h1is = (p, t) => p.waitForFunction((x) => document.querySelector('h1') && document.querySelector('h1').textContent === x, t, { timeout: 4000 }).catch(() => { throw new Error('FAIL: the heading never became «' + t + '»'); });
    const msgHas = (p, text) => p.waitForFunction((x) => document.getElementById('msg').textContent.includes(x), text, { timeout: 4000 }).catch(() => { throw new Error('FAIL: the message never contained «' + text + '»'); });
    const add = (p) => p.getByRole('button', { name: 'Add', exact: true }).click();

    await page.goto(base + '/');
    await page.waitForSelector('h1');
    await page.evaluate(() => localStorage.setItem('tp.planview', 'list'));  // the grid is the default on wide screens; most checks below look for the list
    ok((await page.textContent('h1')) === 'Choose organisation', 'the front page shows the organisation choice');
    ok((await page.getAttribute('html', 'lang')) === 'en', 'the page language is English');

    // without a name: writing is refused with an explanation
    await page.fill('form.row input', 'Sports Club');
    await page.click('form.row button');
    ok((await page.textContent('#msg')).includes('Type who you are'), 'refused without a name');

    await page.fill('#actor', 'Ola Nordmann');
    await page.dispatchEvent('#actor', 'change');
    await page.click('form.row button');
    await page.waitForURL('**/o/sports-club');
    await h1is(page, 'Sports Club');

    await page.fill('form.row input[placeholder^="New plan"]', 'Autumn 2026');
    await page.click('text=Create plan');
    await page.waitForURL('**/o/sports-club/p/autumn-2026');
    await h1is(page, 'Autumn 2026');

    await page.click('text=← Sports Club');
    await page.waitForSelector('h2:has-text("People")');
    await page.fill('form.row input[placeholder="New person"]', 'Kari Test');
    await add(page);
    await page.waitForSelector('td:has-text("Kari Test")');
    await page.uncheck('input[aria-label="Can be proposed: Kari Test"]');
    await page.waitForFunction(() => document.body.innerText.includes('person.update'));
    const logText = await page.innerText('ul.log');
    ok(logText.includes('Ola Nordmann') && logText.includes('proposable true -> false'), 'the change log shows who and what: ' + logText);

    // switching organisation: a new organisation, then back through the selector
    await page.goto(base + '/?switch');
    await page.fill('form.row input', 'Other Org');
    await page.click('form.row button');
    await page.waitForURL('**/o/other-org');
    await h1is(page, 'Other Org');
    ok((await page.innerText('main')).includes('No plans yet'), 'a new organisation is empty');
    await page.selectOption('header select:not(#lang)', 'sports-club');
    await page.waitForURL('**/o/sports-club');
    await h1is(page, 'Sports Club');
    ok((await page.innerText('main')).includes('Autumn 2026'), 'plans follow the organisation');
    const before = errors.length;  // an unknown organisation is a 404 from the API, which is right
    await page.goto(base + '/o/does-not-exist');
    await h1is(page, 'Page not found');
    errors.length = before;

    // --- phase 2: setup and dates ---
    await page.goto(base + '/o/sports-club/setup');
    await h1is(page, 'Setup');
    await page.fill('input[placeholder="New task"]', 'Sound');
    await add(page);
    await page.waitForSelector('td:has-text("Sound")');
    await page.fill('textarea[aria-label^="Several tasks"]', 'Video\nCafé\nSound');
    await page.getByRole('button', { name: 'Add all' }).click();
    await page.waitForSelector('td:has-text("Café")');
    await page.fill('input[placeholder^="New day type"]', 'Match day');
    await page.locator('form:has(input[placeholder^="New day type"]) button').click();
    await page.waitForSelector('input[aria-label="Places for Sound on Match day"]');
    await page.fill('input[aria-label="Places for Sound on Match day"]', '1');
    await page.fill('input[aria-label="Places for Café on Match day"]', '3');
    await page.locator('.card:has(input[aria-label="Day type name"]) button:has-text("Save tasks")').click();
    await page.selectOption('select[aria-label="Sound + Video"]', 'cannot');
    await page.reload();
    await h1is(page, 'Setup');
    await page.waitForSelector('input[aria-label="Places for Café on Match day"]');
    ok((await page.inputValue('input[aria-label="Places for Café on Match day"]')) === '3', 'places stay after a reload');
    ok((await page.inputValue('select[aria-label="Sound + Video"]')) === 'cannot', 'the combination stays');

    await page.goto(base + '/o/sports-club/p/autumn-2026');
    await h1is(page, 'Autumn 2026');
    await page.fill('input[aria-label="From"]', '2026-08-01');
    await page.fill('input[aria-label="To"]', '2026-08-31');
    await page.getByRole('button', { name: 'Add all' }).click();
    await page.waitForFunction(() => document.querySelectorAll('select[aria-label^="Day type 2026"]').length === 5);
    ok((await page.textContent('#msg')).includes('New events: 5'), 'five Sundays in August 2026');
    // an extra event on the same day is just another event
    await page.fill('input[aria-label="Date"]', '2026-08-09');
    await page.fill('input[aria-label="Note"]', 'evening');
    await page.click('text=Add event');
    await page.waitForFunction(() => document.querySelectorAll('select[aria-label^="Day type 2026-08-09"]').length === 2);
    await page.click('button[aria-label^="Delete the event 2026-08-30"]');
    await page.waitForFunction(() => document.querySelectorAll('select[aria-label^="Day type 2026"]').length === 5);

    // --- phase 3: my page ---
    await page.goto(base + '/o/sports-club/p/autumn-2026');
    await h1is(page, 'Autumn 2026');
    await page.selectOption('select[aria-label="Pick your name"]', { label: 'Kari Test' });
    await page.click('text=Open my page');
    await h1is(page, 'Kari Test');
    await page.click('button[aria-label="Add Sound"]');
    await page.waitForSelector('[aria-label="How willingly: Sound"]');
    await page.click('[aria-label="How willingly: Sound"] button[data-level="good"]');
    await page.waitForSelector('[aria-label="How willingly: Sound"] button[data-level="good"][aria-pressed="true"]');
    await page.click('.avail-row:first-child button[data-level="never"]');
    await page.waitForSelector('.avail-row:first-child button[data-level="never"][aria-pressed="true"]');
    ok((await page.innerText('main')).includes('1 do not suit'), 'the summary counts the answer');
    await page.reload();
    await h1is(page, 'Kari Test');
    await page.waitForSelector('.avail-row:first-child button[data-level="never"][aria-pressed="true"]');
    await page.waitForSelector('[aria-label="How willingly: Sound"] button[data-level="good"][aria-pressed="true"]');
    await page.click('.row-inline button:has-text("Good")');  // everything to good; one answer is overwritten, the confirmation is accepted
    await page.waitForFunction(() => document.querySelectorAll('.avail-row button[data-level="good"][aria-pressed="true"]').length === document.querySelectorAll('.avail-row').length);
    await page.click('.avail-row:first-child button[data-level="good"]');  // pressing the chosen answer again removes it
    await page.waitForSelector('.avail-row:first-child button[aria-pressed="true"]', { state: 'detached' });

    // --- phase 4: plan ---
    const kariUrl = page.url().replace(base, '');
    ok(/\/o\/sports-club\/p\/autumn-2026\/me\/\d+$/.test(kariUrl), 'the personal page has an English address: ' + kariUrl);
    await page.goto(base + '/o/sports-club');
    await page.waitForSelector('input[aria-label="Can be proposed: Kari Test"]');
    await page.check('input[aria-label="Can be proposed: Kari Test"]');  // switched off earlier in the check
    await page.waitForFunction(() => document.querySelector('input[aria-label="Can be proposed: Kari Test"]').checked && document.body.innerText.includes('true -> false') && document.body.innerText.includes('false -> true'));
    // a group: Ola Test is in «Team A», which the planner may propose for Café (1 of 3 places)
    await page.fill('form.row input[placeholder="New person"]', 'Ola Test');
    await add(page);
    await page.waitForSelector('td:has-text("Ola Test")');
    await page.goto(base + '/o/sports-club/setup');
    await h1is(page, 'Setup');
    await page.fill('input[placeholder^="New group"]', 'Team A');
    await page.locator('form:has(input[placeholder^="New group"]) button').click();
    await page.waitForSelector('input[aria-label="Group name"]');
    await page.check('input[aria-label="Ola Test in Team A"]');
    // every save redraws the page: wait until it is done before the next field is filled in
    const saved = async (suffix, button) => {
      await Promise.all([page.waitForResponse((r) => r.url().endsWith(suffix) && r.request().method() === 'PUT'), page.locator('.card:has(input[aria-label="Group name"]) button:has-text("' + button + '")').click()]);
      await page.waitForLoadState('networkidle');
    };
    await saved('/members', 'Save members');
    await page.fill('input[aria-label="Places Team A fills on Café"]', '1');
    await saved('/tasks', 'Save tasks');
    await page.reload();
    await page.waitForSelector('input[aria-label="Places Team A fills on Café"]');
    ok((await page.inputValue('input[aria-label="Places Team A fills on Café"]')) === '1', 'the group task stays after a reload');
    ok(await page.isChecked('input[aria-label="Ola Test in Team A"]'), 'the group member stays after a reload');
    ok(await page.isChecked('input[aria-label="Can be proposed: Team A"]'), 'a new group can be proposed');
    await page.goto(base + '/o/sports-club/p/autumn-2026');
    await h1is(page, 'Autumn 2026');
    ok((await page.innerText('main')).includes('No plan has been made yet'), 'the plan is empty before the first run');
    // manual cells and selections, for several events at a time: a guest on Sound the first Sunday, and Kari alone in the selection for Sound on 16 August
    await page.click('summary:has-text("New manual cell")');
    const addForm = page.locator('form:has(input[placeholder^="Text"])');
    await addForm.locator('summary:has-text("Events")').click();
    await addForm.locator('input[aria-label="2026-08-02 Match day"]').check();
    await addForm.locator('select[aria-label="Task for the cell"]').selectOption({ label: 'Sound' });
    await addForm.locator('select[aria-label="What to put in"]').selectOption('text');
    await addForm.locator('input[aria-label="Text"]').fill('Outside guest');
    await addForm.locator('button[type=submit]').click();
    await page.waitForSelector('.rule:has-text("Sound: Outside guest")');
    await page.click('summary:has-text("New selection")');
    const poolForm = page.locator('form:has(select[aria-label="Task for the selection"])');
    await poolForm.locator('summary:has-text("Events")').click();
    await poolForm.locator('input[aria-label="2026-08-16 Match day"]').check();
    await poolForm.locator('select[aria-label="Task for the selection"]').selectOption({ label: 'Sound' });
    await poolForm.locator('input[aria-label="Selection: Kari Test"]').check();
    await poolForm.locator('button[type=submit]').click();
    await page.waitForSelector('.rule:has-text("Selection Sound: Kari Test")');
    await page.click('summary:has-text("New manual cell")');
    await page.locator('form:has(input[placeholder^="Text"]) button[type=submit]').click();  // no event chosen: an explanation, not an error
    await msgHas(page, 'Choose at least one event');
    await page.getByRole('button', { name: 'Plan', exact: true }).click();
    await page.waitForSelector('.plan-event', { timeout: 20000 });
    let panel = await page.innerText('.plan-panel');
    ok(panel.includes('Kari Test'), 'Kari got Sound: ' + panel);
    ok(panel.includes('Checked independently: no rule violations'), 'the check ran: ' + panel);
    ok(panel.includes('Team A (Ola Test)'), 'the group is proposed for Café, with its members: ' + panel);
    ok(panel.includes('Missing 2') && panel.includes('have not chosen the task'), 'places that cannot be filled are explained: ' + panel);
    ok(panel.includes('Outside guest'), 'the manual guest is on Sound: ' + panel);
    ok((await page.locator('.plan-event').nth(0).innerText()).includes('Outside guest') && !(await page.locator('.plan-event').nth(0).innerText()).includes('Kari Test'), 'the guest fills the place the first Sunday, so Kari does not get Sound then');
    ok((await page.locator('.plan-event').count()) === 5, 'one row per event');
    await page.getByRole('button', { name: 'Try again' }).click();
    await page.waitForSelector('.plan-panel p:has-text("Planning")', { timeout: 3000 }).catch(() => {});  // it may be done before we get to see it
    await page.waitForSelector('button:has-text("Try again"):not([disabled])', { timeout: 20000 });
    ok((await page.locator('.plan-event').count()) === 5, 'a new plan after «Try again»');
    // the plan goes out of date when an answer changes
    await page.goto(base + kariUrl);
    await h1is(page, 'Kari Test');
    await page.click('.avail-row:nth-child(2) button[data-level="poor"]');
    await page.waitForSelector('.avail-row:nth-child(2) button[data-level="poor"][aria-pressed="true"]');
    await page.goto(base + '/o/sports-club/p/autumn-2026');
    await page.waitForSelector('.plan-panel .warn');
    ok((await page.innerText('.plan-panel')).includes('have changed since this plan was made'), 'an out-of-date plan is marked');
    await page.getByRole('button', { name: 'Try again' }).click();
    await page.waitForSelector('.plan-panel .warn', { state: 'detached', timeout: 20000 });

    // a group that a manual cell names cannot be deleted, and the message says so (not «already exists»)
    await page.goto(base + '/o/sports-club/p/autumn-2026');
    await page.click('summary:has-text("New manual cell")');
    const grpForm = page.locator('form:has(input[placeholder^="Text"])');
    await grpForm.locator('summary:has-text("Events")').click();
    await grpForm.locator('input[aria-label="2026-08-23 Match day"]').check();
    await grpForm.locator('select[aria-label="What to put in"]').selectOption('group');
    await grpForm.locator('button[type=submit]').click();
    await page.waitForSelector('.rule:has-text("Team A")');
    await page.goto(base + '/o/sports-club/setup');
    await page.waitForSelector('input[aria-label="Group name"]');
    const beforeConflict = errors.length;  // 409 is the right answer here, and the browser logs it as an error
    await page.locator('.card:has(input[aria-label="Group name"]) button.danger').click();  // the confirmation is accepted
    await msgHas(page, 'is in use and cannot be deleted');
    errors.length = beforeConflict;
    ok(!(await page.textContent('#msg')).includes('already exists'), 'in use is not «already exists»: ' + (await page.textContent('#msg')));
    await page.goto(base + '/o/sports-club/p/autumn-2026');
    await page.click('button[aria-label^="Remove the cell"][aria-label*="Team A"]');
    await page.waitForSelector('.rule:has-text("Team A")', { state: 'detached' });
    // «Lock» turns a proposal into a manual cell (and the plan goes out of date); «Remove» takes it away again
    await page.locator('button[aria-label^="Lock Kari Test"]').first().click();
    await page.waitForSelector('.rule:has-text("Sound: Kari Test")');
    await page.waitForSelector('.plan-panel .warn');
    await page.click('button[aria-label^="Remove the cell Sound Outside guest"]');
    await page.waitForSelector('.rule:has-text("Outside guest")', { state: 'detached' });

    // weighting: changed per plan, stays, makes the plan out of date, and can be reset to the default
    await page.goto(base + '/o/sports-club/p/autumn-2026');
    await h1is(page, 'Autumn 2026');
    await page.click('summary:has-text("Show and change weighting")');
    const perTask = page.locator('input[aria-label="Equal share of each task"]');
    ok((await perTask.inputValue()) === '0', 'by default fairness per task is off');
    await perTask.fill('5');
    await page.getByRole('button', { name: 'Save weighting' }).click();
    await msgHas(page, 'Weighting saved');
    await page.waitForSelector('summary:has-text("1 changed")');
    ok((await page.innerText('.plan-panel')).includes('weights'), 'a different weighting makes the plan out of date');
    await page.reload();
    await page.click('summary:has-text("1 changed")');
    ok((await page.locator('input[aria-label="Equal share of each task"]').inputValue()) === '5', 'the weighting stays after a reload');
    await page.locator('input[aria-label="Equal total load"]').fill('500');  // outside the limit: the server refuses
    await page.getByRole('button', { name: 'Save weighting' }).click();
    const beforeBad = errors.length;
    await msgHas(page, 'must be between 0 and 100');
    errors.length = beforeBad;  // 400 is the right answer here, and the browser logs it as an error
    ok((await page.textContent('#msg')).includes('“Equal total load”'), 'the error names the weight by its label: ' + (await page.textContent('#msg')));
    await page.getByRole('button', { name: 'Back to default' }).click();
    await msgHas(page, 'reset to the default');
    await page.click('summary:has-text("Show and change weighting")');
    ok((await page.locator('input[aria-label="Equal share of each task"]').inputValue()) === '0', 'back to the default');

    // the grid (events across, tasks down) and «my assignments»
    await page.goto(base + '/o/sports-club/p/autumn-2026');
    await h1is(page, 'Autumn 2026');
    await page.waitForSelector('.plan-event');
    await page.getByRole('button', { name: 'Grid', exact: true }).click();
    await page.waitForSelector('table.plan-grid');
    ok((await page.locator('table.plan-grid thead th[scope=col]').count()) === 5, 'one column per event');
    ok((await page.locator('table.plan-grid tbody tr').count()) >= 2, 'one row per task');
    ok((await page.innerText('table.plan-grid')).includes('Team A (Ola Test)'), 'the group is in the grid');
    ok((await page.locator('table.plan-grid button[aria-label^="Lock"]').count()) >= 1, 'proposals can be locked in the grid too');
    await page.reload();
    await page.waitForSelector('table.plan-grid');  // the choice stays
    await page.getByRole('button', { name: 'List', exact: true }).click();
    await page.waitForSelector('.plan-event');
    await page.selectOption('select[aria-label="Pick your name"]', { label: 'Ola Test' });
    await page.click('text=Open my page');
    await h1is(page, 'Ola Test');
    await page.waitForSelector('.mine-row');
    ok((await page.innerText('.mine')).includes('Café (with Team A)'), 'an assignment through a group names the group: ' + (await page.innerText('main')));
    await page.goto(base + kariUrl);
    await h1is(page, 'Kari Test');
    await page.waitForSelector('.mine-row');
    ok((await page.innerText('.mine')).includes('Sound'), 'Kari has Sound in «my assignments»');
    await page.evaluate(() => localStorage.setItem('tp.planview', 'grid'));  // the width of the grid is checked below

    const urls = ['/o/sports-club', '/o/sports-club/setup', '/o/sports-club/p/autumn-2026', kariUrl];
    const noSideways = async (lang) => {
      for (const url of urls) {
        await page.goto(base + url);
        await page.waitForSelector('h1');
        await page.waitForTimeout(150);
        const w = await page.evaluate(() => [document.documentElement.scrollWidth, document.documentElement.clientWidth]);
        ok(w[0] <= w[1], `[${lang}] the page ${url} is ${w[0]} px wide on a ${w[1]} px screen (scrolls sideways)`);
      }
    };
    // no page may be wider than a phone screen
    await page.setViewportSize({ width: 390, height: 844 });
    await noSideways('en');

    // --- Norwegian: the switch, what the server writes, the errors, and the same width check ---
    await page.setViewportSize({ width: 1280, height: 900 });
    await page.evaluate(() => localStorage.setItem('tp.planview', 'list'));  // the explanations are read from the list
    await page.goto(base + '/o/sports-club');
    await h1is(page, 'Sports Club');
    await page.selectOption('#lang', 'nb');
    await page.waitForSelector('h2:has-text("Planer")');
    ok((await page.getAttribute('html', 'lang')) === 'nb', 'the page language becomes Norwegian');
    ok((await page.textContent('header .brand')) === 'Tjenesteplan', 'the brand is Tjenesteplan in Norwegian');
    ok((await page.evaluate(() => localStorage.getItem('tp.lang'))) === 'nb', 'the choice is kept');
    await page.reload();
    await page.waitForSelector('h2:has-text("Planer")');  // and survives a reload
    await page.goto(base + '/o/sports-club/p/autumn-2026');
    await h1is(page, 'Autumn 2026');
    await page.waitForSelector('.plan-event');
    panel = await page.innerText('.plan-panel');
    ok(panel.includes('Mangler 2') && panel.includes('har ikke valgt oppgaven'), 'the server explains in Norwegian too: ' + panel);
    ok(panel.includes('Kontrollert uavhengig'), 'the run texts are Norwegian: ' + panel);
    await page.goto(base + '/o/sports-club/setup');
    await h1is(page, 'Oppsett');
    await page.fill('input[placeholder="Ny oppgave"]', 'Sound');
    const beforeDup = errors.length;
    await page.getByRole('button', { name: 'Legg til', exact: true }).click();
    await msgHas(page, 'Oppgave «Sound» finnes allerede');  // an error code from the server, written in Norwegian
    errors.length = beforeDup;  // 409 is the right answer here
    await page.goto(base + kariUrl);
    await h1is(page, 'Kari Test');
    await page.waitForSelector('.mine-row');
    ok((await page.innerText('main')).includes('Mine oppdrag') && (await page.innerText('main')).includes('Når kan du?'), 'my page is Norwegian');
    await page.evaluate(() => localStorage.setItem('tp.planview', 'grid'));
    await page.setViewportSize({ width: 390, height: 844 });
    await noSideways('nb');

    // a visitor whose browser is Norwegian gets Norwegian without choosing, and can still switch to English
    const nbContext = await browser.newContext({ locale: 'nb-NO' });
    const nbPage = await nbContext.newPage();
    watch(nbPage);
    await nbPage.goto(base + '/?switch');
    await h1is(nbPage, 'Velg organisasjon');
    ok((await nbPage.getAttribute('html', 'lang')) === 'nb', 'a Norwegian browser gets a Norwegian page');
    await nbPage.selectOption('#lang', 'en');
    await h1is(nbPage, 'Choose organisation');
    await nbContext.close();

    ok(errors.length === 0, 'console errors: ' + errors.join(' | ') + ' 404 on: ' + notFound.join(', '));
    console.log('ui-check: ok');
  } catch (e) {
    const at = String(e.stack || '').split('\n').find((l) => l.includes('ui-check.js')) || '';
    console.log(String(e.message || e), at.trim(), errors.length ? '\n' + errors.join('\n') : '');
    process.exitCode = 1;
  } finally {
    if (browser) await browser.close();
    srv.kill('SIGTERM');
    fs.rmSync(dir, { recursive: true, force: true });
  }
})();
