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
    await page.waitForSelector('input[aria-label="Name of Kari Test"]');
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
    await page.waitForSelector('input[aria-label="Name of Sound"]');
    await page.fill('textarea[aria-label^="Several tasks"]', 'Video\nCafé\nSound');
    await page.getByRole('button', { name: 'Add all' }).click();
    await page.waitForSelector('input[aria-label="Name of Café"]');
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

    // --- the plan page: one table ---
    const planUrl = '/o/sports-club/p/autumn-2026';
    const cols = (p) => p.locator('.dp-table thead th[scope=col]:not(.addcol)').count();
    const dialog = page.locator('dialog.modal');
    const closeDialog = async () => { await page.keyboard.press('Escape'); await dialog.waitFor({ state: 'detached' }); };
    await page.goto(base + planUrl);
    await h1is(page, 'Autumn 2026');
    await page.click('summary:has-text("Add several events")');
    await page.fill('input[aria-label="From"]', '2026-08-01');
    await page.fill('input[aria-label="To"]', '2026-08-31');
    await page.getByRole('button', { name: 'Add all' }).click();
    await msgHas(page, 'New events: 5');
    ok((await cols(page)) === 5, 'five Sundays in August 2026, one column each');
    // an extra event on the same day is just another event: «+» at the end of the date row
    await page.getByRole('button', { name: 'Add an event' }).click();
    await dialog.locator('input[aria-label="Date"]').fill('2026-08-09');
    await dialog.locator('input[aria-label="Note"]').fill('evening');
    await dialog.getByRole('button', { name: 'Add event' }).click();
    await page.waitForFunction(() => document.querySelectorAll('.dp-table thead th[scope=col]:not(.addcol)').length === 6);
    ok((await page.locator('button[aria-label="Open the event 2026-08-09 Match day"]').count()) === 2, 'two events on 9 August');
    await page.click('button[aria-label="Open the event 2026-08-30 Match day"]');
    await dialog.getByRole('button', { name: 'Delete the event' }).click();  // the confirmation is accepted
    await page.waitForFunction(() => document.querySelectorAll('.dp-table thead th[scope=col]:not(.addcol)').length === 5);

    // answers: a click moves the cell on (blank → yes → maybe → no → blank), and the count above the event follows
    const cell = (who, label) => page.locator(`button.ans[aria-label^="${who}, ${label}:"]`);
    const kari1 = cell('Kari Test', '2026-08-02 Match day');
    const level = (loc) => loc.getAttribute('data-level');
    const clickTo = async (loc, want) => { await loc.click(); await page.waitForFunction(([sel, w]) => document.querySelector(sel).dataset.level === w, [`button.ans[aria-label^="Kari Test, 2026-08-02 Match day:"]`, want]); };
    ok((await level(kari1)) === 'none', 'a new person has not answered');
    await clickTo(kari1, 'good');
    ok((await kari1.innerText()) === '✓' && (await kari1.getAttribute('aria-label')).endsWith(': yes'), 'one click is yes');
    await clickTo(kari1, 'poor');
    await clickTo(kari1, 'never');
    ok((await kari1.innerText()) === '✗', 'three clicks are no');
    ok((await page.locator('.dp-table td.count').first().innerText()).includes('✗ 1'), 'the count above the event follows: ' + (await page.locator('.dp-table td.count').first().innerText()));
    await page.reload();
    await page.waitForSelector('.dp-table');
    ok((await level(kari1)) === 'never', 'the answer stays after a reload');
    await clickTo(kari1, 'none');
    await clickTo(kari1, 'good');
    await page.reload();
    await page.waitForSelector('.dp-table');
    ok((await level(kari1)) === 'good', 'and its next state too');

    // the person: what they take part in, from the dialog behind the name
    ok((await page.locator('tr:has(button[aria-label="Open Kari Test"]) .sub.prompt').innerText()) === 'choose tasks', 'a person without tasks is asked to choose');
    await page.click('button[aria-label="Open Kari Test"]');
    await dialog.locator('select[aria-label="Sound for Kari Test"]').selectOption('good');
    // the page is drawn again with the new data (the chosen task under the name), and the dialog opens again on top
    const kariSub = page.locator('tr:has(button[aria-label="Open Kari Test"]) .sub').first();
    await page.waitForFunction(() => { return [...document.querySelectorAll('tr[data-person]')].some((tr) => tr.querySelector('button[aria-label="Open Kari Test"]') && tr.querySelector('.sub').textContent === 'Sound'); });
    await dialog.waitFor();
    ok((await dialog.locator('select[aria-label="Sound for Kari Test"]').inputValue()) === 'good', 'the dialog opens again with the new data');
    await closeDialog();
    ok((await kariSub.getAttribute('class')) === 'sub', 'no longer asked to choose');
    await page.reload();
    await page.click('button[aria-label="Open Kari Test"]');
    ok((await dialog.locator('select[aria-label="Sound for Kari Test"]').inputValue()) === 'good', 'the task choice stays after a reload');
    await closeDialog();

    // places on one event: faded from the day type, bold when the event has its own, and back when emptied
    const cafe1 = page.locator('input[aria-label="Places for Café on 2026-08-02 Match day"]');
    ok((await cafe1.inputValue()) === '' && (await cafe1.getAttribute('placeholder')) === '3', 'the day type’s places show faded');
    await cafe1.fill('2');
    await cafe1.blur();
    await page.waitForFunction(() => document.querySelector('input[aria-label="Places for Café on 2026-08-02 Match day"]').classList.contains('own'));
    await page.reload();
    await page.waitForSelector('.dp-table');
    ok((await cafe1.inputValue()) === '2' && (await cafe1.getAttribute('class')) === 'own', 'the event’s own places stay after a reload');
    await cafe1.fill('3');  // the day type’s own number: back to the day type
    await cafe1.blur();
    await page.waitForFunction(() => !document.querySelector('input[aria-label="Places for Café on 2026-08-02 Match day"]').classList.contains('own'));
    await page.reload();
    await page.waitForSelector('.dp-table');
    ok((await cafe1.inputValue()) === '', 'typing the day type’s number goes back to it');
    await cafe1.fill('0');
    await cafe1.blur();
    await page.waitForFunction(() => document.querySelector('input[aria-label="Places for Café on 2026-08-02 Match day"]').classList.contains('own'));
    await cafe1.fill('');
    await cafe1.blur();
    await page.waitForFunction(() => !document.querySelector('input[aria-label="Places for Café on 2026-08-02 Match day"]').classList.contains('own'));
    await page.reload();
    await page.waitForSelector('.dp-table');
    ok((await cafe1.inputValue()) === '', 'emptying the field goes back to the day type');

    // an info row of the plan: add, a value, rename and delete
    await page.fill('input[placeholder="New info row, e.g. Theme"]', 'Theme');
    await page.press('input[placeholder="New info row, e.g. Theme"]', 'Enter');
    await page.waitForSelector('button[aria-label="Open the info row Theme"]');
    await page.fill('input[aria-label="Theme 2026-08-02 Match day"]', 'Opening');
    await Promise.all([page.waitForResponse((r) => r.url().includes('/fields/') && r.request().method() === 'PUT'), page.locator('input[aria-label="Theme 2026-08-02 Match day"]').blur()]);
    await page.reload();
    ok((await page.inputValue('input[aria-label="Theme 2026-08-02 Match day"]')) === 'Opening', 'the value stays after a reload');
    await page.click('button[aria-label="Open the info row Theme"]');
    await dialog.locator('input[aria-label="Name of the info row"]').fill('Motto');
    await dialog.getByRole('button', { name: 'Rename' }).click();
    await page.waitForSelector('button[aria-label="Open the info row Motto"]');
    await page.reload();
    ok((await page.inputValue('input[aria-label="Motto 2026-08-02 Match day"]')) === 'Opening', 'a renamed row keeps its values');
    await page.click('button[aria-label="Open the info row Motto"]');
    await dialog.getByRole('button', { name: 'Delete the info row' }).click();  // the confirmation is accepted
    await page.waitForSelector('button[aria-label="Open the info row Motto"]', { state: 'detached' });
    await page.reload();
    ok((await page.locator('button[aria-label="Open the info row Motto"]').count()) === 0, 'the row is gone after a reload');

    // a task: «cannot be combined with anything» sets every pair, and unticking sets them back to standard
    const comboApi = () => page.evaluate(() => fetch('/api/orgs/sports-club/combinations').then((r) => r.json()));
    await page.click('button[aria-label="Open the task Café"]');
    await dialog.locator('input[aria-label="Cannot be combined with anything"]').check();
    await page.waitForFunction(() => { const b = document.querySelector('dialog.modal input[aria-label="Cannot be combined with anything"]'); return b && b.checked && document.querySelector('dialog.modal select[aria-label="Café + Sound"]').value === 'cannot'; });
    await closeDialog();
    await page.reload();
    await page.click('button[aria-label="Open the task Café"]');
    ok(await dialog.locator('input[aria-label="Cannot be combined with anything"]').isChecked(), 'it stays after a reload');
    ok((await comboApi()).filter((c) => c.level === 'cannot').length === 3, 'Café cannot be combined with Sound or Video, and Sound + Video stays: ' + JSON.stringify(await comboApi()));
    await dialog.locator('input[aria-label="Cannot be combined with anything"]').uncheck();  // the confirmation is accepted
    await page.waitForFunction(() => { const s = document.querySelector('dialog.modal select[aria-label="Café + Sound"]'); return s && s.value === 'standard'; });
    await closeDialog();
    ok((await comboApi()).length === 1, 'back to standard; Sound + Video is untouched: ' + JSON.stringify(await comboApi()));

    // a personal link: the table with the person's row marked, and the name filled in at the top when it is empty
    const kariId = await page.locator('tr:has(button[aria-label="Open Kari Test"])').getAttribute('data-person');
    const kariUrl = `${planUrl}/me/${kariId}`;
    await page.evaluate(() => localStorage.setItem('tp.actor', ''));
    await page.goto(base + kariUrl);
    await page.waitForSelector('tr.me');
    ok((await page.locator('tr.me button.cellbtn').getAttribute('aria-label')) === 'Open Kari Test', 'the row of the person is marked');
    ok((await page.inputValue('#actor')) === 'Kari Test', 'the name is filled in');
    await page.fill('#actor', 'Ola Nordmann');
    await page.dispatchEvent('#actor', 'change');
    const beforeBad404 = errors.length;
    await page.goto(base + planUrl + '/me/99999');
    await h1is(page, 'Page not found');
    errors.length = beforeBad404;
    ok((await page.evaluate(() => localStorage.getItem('tp.org'))) === 'sports-club', 'an unknown person does not forget the organisation');

    // --- the plan ---
    await page.goto(base + '/o/sports-club');
    await page.waitForSelector('input[aria-label="Can be proposed: Kari Test"]');
    await page.check('input[aria-label="Can be proposed: Kari Test"]');  // switched off earlier in the check
    await page.waitForFunction(() => document.querySelector('input[aria-label="Can be proposed: Kari Test"]').checked && document.body.innerText.includes('true -> false') && document.body.innerText.includes('false -> true'));
    // a group: Ola Test is in «Team A», which the planner may propose for Café (1 of 3 places)
    await page.fill('form.row input[placeholder="New person"]', 'Ola Test');
    await add(page);
    await page.waitForSelector('input[aria-label="Name of Ola Test"]');
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
    await page.goto(base + planUrl);
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
    await page.waitForSelector('.plan-cell', { timeout: 20000 });
    // the explanations are behind «Why?», so read the text of the table, not only what is visible
    const tableText = () => page.locator('.dp-table').evaluate((e) => e.textContent);
    let text = await tableText();
    const bar = () => page.innerText('.plan-bar');
    ok(text.includes('Kari Test') && (await page.locator('tr:has(button[aria-label="Open Kari Test"]) .tag.task').count()) >= 1, 'Kari got Sound, and her row says so: ' + text);
    ok((await bar()).includes('Checked independently: no rule violations'), 'the check ran: ' + (await bar()));
    ok(text.includes('Team A (Ola Test)'), 'the group is proposed for Café, with its members: ' + text);
    ok((await page.locator('tr:has(button[aria-label="Open Ola Test"]) .tag.task', { hasText: 'Café (with Team A)' }).count()) >= 1, 'work through a group is in the member’s row');
    ok(text.includes('Missing 2') && text.includes('have not chosen the task'), 'places that cannot be filled are explained: ' + text);
    const sound1 = await page.locator('td.places:has(input[aria-label="Places for Sound on 2026-08-02 Match day"])').innerText();
    ok(sound1.includes('Outside guest') && !sound1.includes('Kari Test'), 'the guest fills the place the first Sunday, so Kari does not get Sound then: ' + sound1);
    await page.getByRole('button', { name: 'Try again' }).click();
    await page.waitForSelector('.plan-bar p:has-text("Planning")', { timeout: 3000 }).catch(() => {});  // it may be done before we get to see it
    await page.waitForSelector('button:has-text("Try again"):not([disabled])', { timeout: 20000 });
    await page.waitForSelector('.plan-cell');
    // the plan goes out of date when an answer changes
    await cell('Kari Test', '2026-08-09 Match day').first().click();
    await page.waitForFunction(() => document.querySelector('button.ans[aria-label^="Kari Test, 2026-08-09 Match day:"]').dataset.level === 'good');
    await page.reload();
    await page.waitForSelector('.plan-bar .warn');
    ok((await bar()).includes('have changed since this plan was made'), 'an out-of-date plan is marked');
    await page.getByRole('button', { name: 'Try again' }).click();
    await page.waitForSelector('.plan-bar .warn', { state: 'detached', timeout: 20000 });

    // «Show plan» takes the plan out of the table and back, and the choice stays
    await page.waitForSelector('.plan-cell');
    await page.getByRole('button', { name: 'Show plan' }).click();
    await page.waitForSelector('.plan-cell', { state: 'detached' });
    await page.reload();
    await page.waitForSelector('.dp-table');
    ok((await page.locator('.plan-cell').count()) === 0 && (await page.getAttribute('button.toggle', 'aria-pressed')) === 'false', 'the choice stays after a reload');
    await page.getByRole('button', { name: 'Show plan' }).click();
    await page.waitForSelector('.plan-cell');

    // history and pin: an earlier run can be shown and made the current plan, which the page then opens with, and which a
    // person's assignments follow
    const assignments = () => page.evaluate((id) => fetch(`/api/orgs/sports-club/plans/autumn-2026/people/${id}/assignments`).then((r) => r.json()), kariId);
    await page.click('summary:has-text("Earlier runs")');
    ok((await page.locator('.run-row').count()) >= 3, 'every run is in the history');
    ok(!(await bar()).includes('not the newest'), 'without a pinned run the newest is shown');
    await page.locator('.run-row button:has-text("Show")').last().click();
    await page.waitForSelector('.plan-bar :text("not the newest")');
    await page.getByRole('button', { name: 'Use as the current plan' }).click();
    await msgHas(page, 'is now the current plan');
    await page.waitForSelector('.plan-bar :text("not the newest")');  // still the older run, now with the pin
    await page.goto(base + planUrl);
    await page.waitForSelector('.plan-bar .tag.lock');
    ok((await bar()).includes('not the newest') && (await bar()).includes('pinned'), 'the page opens with the pinned run: ' + (await bar()));
    ok((await assignments()).run.pinned === true, 'a person’s assignments follow the pinned run');
    await page.getByRole('button', { name: 'Unpin' }).click();
    await msgHas(page, 'Unpinned');
    ok((await assignments()).run.pinned === false, 'unpinned, they follow the newest');
    await page.goto(base + planUrl);
    await page.waitForSelector('.plan-cell');
    ok(!(await bar()).includes('not the newest'), 'and the page opens with the newest again');

    // a group that a manual cell names cannot be deleted, and the message says so (not «already exists»)
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
    await page.goto(base + planUrl);
    await page.click('button[aria-label^="Remove the cell"][aria-label*="Team A"]');
    await page.waitForSelector('.rule:has-text("Team A")', { state: 'detached' });
    // «Lock» turns a proposal into a manual cell (and the plan goes out of date); «Remove» takes it away again
    await page.locator('button[aria-label^="Lock Kari Test"]').first().click();
    await page.waitForSelector('.rule:has-text("Sound: Kari Test")');
    await page.waitForSelector('.plan-bar .warn');
    await page.click('button[aria-label^="Remove the cell Sound Outside guest"]');
    await page.waitForSelector('.rule:has-text("Outside guest")', { state: 'detached' });

    // weighting: changed per plan, stays, makes the plan out of date, and can be reset to the default
    await page.goto(base + planUrl);
    await h1is(page, 'Autumn 2026');
    await page.click('summary:has-text("Show and change weighting")');
    const perTask = page.locator('input[aria-label="Equal share of each task"]');
    ok((await perTask.inputValue()) === '0', 'by default fairness per task is off');
    await perTask.fill('5');
    await page.getByRole('button', { name: 'Save weighting' }).click();
    await msgHas(page, 'Weighting saved');
    await page.waitForSelector('summary:has-text("1 changed")');
    ok((await bar()).includes('weights'), 'a different weighting makes the plan out of date');
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

    const urls = ['/o/sports-club', '/o/sports-club/setup', planUrl, kariUrl];
    const noSideways = async (lang) => {
      for (const url of urls) {
        await page.goto(base + url);
        await page.waitForSelector('h1');
        await page.waitForTimeout(150);
        const w = await page.evaluate(() => [document.documentElement.scrollWidth, document.documentElement.clientWidth]);
        ok(w[0] <= w[1], `[${lang}] the page ${url} is ${w[0]} px wide on a ${w[1]} px screen (scrolls sideways)`);
      }
    };
    // no page may be wider than a phone screen (the table scrolls in its own box)
    await page.setViewportSize({ width: 390, height: 844 });
    await noSideways('en');

    // --- rename and delete what you created ---
    await page.setViewportSize({ width: 1280, height: 900 });
    await page.goto(base + '/o/sports-club');
    await page.waitForSelector('input[aria-label="Name of Kari Test"]');
    await page.fill('input[aria-label="Name of Kari Test"]', 'Kari Renamed');
    await page.locator('input[aria-label="Name of Kari Test"]').blur();  // leaving the field saves it
    await page.waitForSelector('input[aria-label="Name of Kari Renamed"]');
    await page.goto(base + kariUrl);  // the personal link still works, with the new name
    await page.waitForSelector('tr.me button[aria-label="Open Kari Renamed"]');
    await page.goto(base + '/o/sports-club');
    await page.fill('input[aria-label="Name of Kari Renamed"]', 'Kari Test');
    await page.locator('input[aria-label="Name of Kari Renamed"]').blur();  // leaving the field saves it
    await page.waitForSelector('input[aria-label="Name of Kari Test"]');
    await page.goto(base + '/o/sports-club/setup');
    await page.fill('input[aria-label="Name of Video"]', 'Camera');
    await page.locator('input[aria-label="Name of Video"]').blur();  // leaving the field saves it
    await page.waitForSelector('input[aria-label="Name of Camera"]');
    await page.fill('input[aria-label="Name of Camera"]', 'Sound');  // a name that is taken
    const beforeTaken = errors.length;
    await page.locator('input[aria-label="Name of Camera"]').blur();  // leaving the field saves it
    await msgHas(page, 'Task “Sound” already exists');
    errors.length = beforeTaken;  // 409 is the right answer here
    ok((await page.inputValue('input[aria-label="Name of Camera"]')) === 'Camera', 'a refused rename puts the old name back');

    await page.goto(base + '/o/sports-club');
    await page.click('summary:has-text("Rename or delete")');
    await page.fill('input[aria-label="Name of the organisation"]', 'Sports Club 2');
    await page.getByRole('button', { name: 'Rename', exact: true }).click();
    await h1is(page, 'Sports Club 2');
    ok(page.url().endsWith('/o/sports-club'), 'the address stays when the organisation is renamed');
    await page.click('summary:has-text("Rename or delete")');
    await page.fill('input[aria-label="Name of the organisation"]', 'Sports Club');
    await page.getByRole('button', { name: 'Rename', exact: true }).click();
    await h1is(page, 'Sports Club');
    await page.click('summary:has-text("Rename or delete")');
    ok((await page.innerText('main')).includes('cannot be deleted'), 'an organisation with plans cannot be deleted: the form is not offered');
    ok((await page.locator('button:has-text("Delete the organisation")').count()) === 0, 'no delete button while there are plans');

    await page.goto(base + planUrl);
    await h1is(page, 'Autumn 2026');
    await page.click('summary:has-text("Rename or delete")');
    await page.fill('input[aria-label="Name of the plan"]', 'Fall 2026');
    await page.getByRole('button', { name: 'Rename', exact: true }).click();
    await h1is(page, 'Fall 2026');
    ok(page.url().endsWith('/p/autumn-2026'), 'the address stays when the plan is renamed');
    await page.goto(base + kariUrl);  // a personal link in this plan still works
    await page.waitForSelector('tr.me');
    await page.goto(base + planUrl);
    await page.click('summary:has-text("Rename or delete")');
    await page.fill('input[aria-label="Name of the plan"]', 'Autumn 2026');
    await page.getByRole('button', { name: 'Rename', exact: true }).click();
    await h1is(page, 'Autumn 2026');

    // a plan to throw away, then an organisation to throw away
    await page.goto(base + '/o/sports-club');
    await page.fill('form.row input[placeholder^="New plan"]', 'Scratch');
    await page.click('text=Create plan');
    await h1is(page, 'Scratch');
    await page.click('summary:has-text("Rename or delete")');
    await page.getByRole('button', { name: 'Delete the plan' }).click();  // the confirmation is accepted
    await page.waitForURL('**/o/sports-club');
    await h1is(page, 'Sports Club');
    await msgHas(page, 'The plan “Scratch” was deleted');
    ok(!(await page.innerText('main')).includes('Scratch') || (await page.locator('ul.plain a:has-text("Scratch")').count()) === 0, 'the deleted plan is not listed');
    ok((await page.innerText('ul.log')).includes('plan.delete'), 'the deletion is in the change log');

    await page.goto(base + '/?switch');
    await page.fill('form.row input', 'Temp Org');
    await page.click('form.row button');
    await page.waitForURL('**/o/temp-org');
    await page.fill('form.row input[placeholder^="New plan"]', 'Temp plan');
    await page.click('text=Create plan');
    await h1is(page, 'Temp plan');
    await page.click('text=← Temp Org');
    await h1is(page, 'Temp Org');
    await page.click('summary:has-text("Rename or delete")');
    ok((await page.locator('button:has-text("Delete the organisation")').count()) === 0, 'an organisation with a plan cannot be deleted');
    await page.click('ul.plain a:has-text("Temp plan")');
    await h1is(page, 'Temp plan');
    await page.click('summary:has-text("Rename or delete")');
    await page.getByRole('button', { name: 'Delete the plan' }).click();
    await page.waitForURL('**/o/temp-org');
    await page.click('summary:has-text("Rename or delete")');
    const deleteOrg = page.getByRole('button', { name: 'Delete the organisation' });
    ok(await deleteOrg.isDisabled(), 'the delete button waits for the name to be typed');
    await page.fill('input[aria-label="Type the name of the organisation to confirm"]', 'Temp');
    ok(await deleteOrg.isDisabled(), 'a part of the name is not enough');
    await page.fill('input[aria-label="Type the name of the organisation to confirm"]', 'Temp Org');
    ok(await deleteOrg.isEnabled(), 'the whole name enables it');
    await deleteOrg.click();
    await h1is(page, 'Choose organisation');
    await msgHas(page, 'The organisation “Temp Org” was deleted');
    ok(!(await page.innerText('main')).includes('Temp Org'), 'the deleted organisation is not listed');
    await page.evaluate(() => localStorage.setItem('tp.org', 'sports-club'));

    // --- a time of day and a description on an event, in its dialog ---
    await page.goto(base + planUrl);
    await h1is(page, 'Autumn 2026');
    await page.click('button[aria-label="Open the event 2026-08-02 Match day"]');
    await dialog.locator('input[aria-label="Start time"]').fill('11:00');
    await dialog.locator('input[aria-label="End time"]').fill('10:00');  // before the start: refused, and the dialog says why
    const beforeEnd = errors.length;
    await dialog.getByRole('button', { name: 'Save' }).click();
    await page.waitForFunction(() => document.querySelector('dialog.modal [data-msg]').textContent.includes('The end time must be after the start time'));
    errors.length = beforeEnd;  // 400 is the right answer here
    await dialog.locator('input[aria-label="End time"]').fill('12:30');
    await dialog.locator('textarea[aria-label="Description"]').fill('Doors open at 10:30.\nBring the banner.');
    await dialog.getByRole('button', { name: 'Save' }).click();
    await page.waitForSelector('button[aria-label="Open the event 2026-08-02 11:00 Match day"]');  // the labels carry the time now
    await page.reload();
    const head1 = await page.locator('button[aria-label="Open the event 2026-08-02 11:00 Match day"]').innerText();
    ok(head1.includes('11:00–12:30') && head1.includes('description'), 'the date row shows the time and that there is a description: ' + head1);
    await page.click('button[aria-label="Open the event 2026-08-02 11:00 Match day"]');
    ok((await dialog.locator('textarea[aria-label="Description"]').inputValue()) === 'Doors open at 10:30.\nBring the banner.', 'the description stays after a reload');
    await dialog.locator('input[aria-label="Start time"]').fill('');  // clearing the start time clears the end time too
    await dialog.getByRole('button', { name: 'Save' }).click();
    await page.waitForSelector('button[aria-label="Open the event 2026-08-02 Match day"]');
    await page.click('button[aria-label="Open the event 2026-08-02 Match day"]');
    ok((await dialog.locator('input[aria-label="End time"]').inputValue()) === '', 'clearing the start clears the end');
    await closeDialog();

    // --- Norwegian: the switch, what the server writes, the errors, and the same width check ---
    await page.setViewportSize({ width: 1280, height: 900 });
    await page.goto(base + '/o/sports-club');
    await h1is(page, 'Sports Club');
    await page.selectOption('#lang', 'nb');
    await page.waitForSelector('h2:has-text("Planer")');
    ok((await page.getAttribute('html', 'lang')) === 'nb', 'the page language becomes Norwegian');
    ok((await page.textContent('header .brand')) === 'Tjenesteplan', 'the brand is Tjenesteplan in Norwegian');
    ok((await page.evaluate(() => localStorage.getItem('tp.lang'))) === 'nb', 'the choice is kept');
    await page.reload();
    await page.waitForSelector('h2:has-text("Planer")');  // and survives a reload
    await page.goto(base + planUrl);
    await h1is(page, 'Autumn 2026');
    await page.waitForSelector('.plan-cell');
    text = await tableText();
    ok(text.includes('Mangler 2') && text.includes('har ikke valgt oppgaven'), 'the server explains in Norwegian too: ' + text);
    ok((await bar()).includes('Kontrollert uavhengig') && (await bar()).includes('Bruk som gjeldende plan'), 'the run texts are Norwegian: ' + (await bar()));
    ok((await page.locator('button.ans[aria-label^="Kari Test, 2026-08-09 Match day:"]').first().getAttribute('aria-label')).endsWith(': ja'), 'the answers are Norwegian');
    await page.goto(base + '/o/sports-club/setup');
    await h1is(page, 'Oppsett');
    await page.fill('input[placeholder="Ny oppgave"]', 'Sound');
    const beforeDup = errors.length;
    await page.getByRole('button', { name: 'Legg til', exact: true }).click();
    await msgHas(page, 'Oppgave «Sound» finnes allerede');  // an error code from the server, written in Norwegian
    errors.length = beforeDup;  // 409 is the right answer here
    await page.goto(base + kariUrl);
    await page.waitForSelector('tr.me');
    ok((await page.innerText('.dp-table')).includes('Dato') && (await page.innerText('.dp-table')).includes('PERSONER'), 'the table is Norwegian: ' + (await page.innerText('.dp-table')).slice(0, 80));
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
