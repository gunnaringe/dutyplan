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
const pb = require('./pb');  // fakes and changed answers, in binary as the page reads them
// A read of the API (Connect: every call is a POST to /dutyplan.v1.<Service>/<Method>): its method, else null.
const readOf = (r) => { const p = new URL(r.url()).pathname; return p.startsWith('/dutyplan.v1.') && /\/(List|Get)[A-Z]/.test(p) ? p.split('/').pop() : null; };

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
    const context = await browser.newContext({ locale: 'en-GB', viewport: { width: 1280, height: 900 } });
    await context.addInitScript(() => { try { if (!localStorage.getItem('tp.lang')) localStorage.setItem('tp.lang', 'en'); } catch (e) { /* ignore */ } });
    await context.grantPermissions(['clipboard-read', 'clipboard-write']);
    const page = await context.newPage();
    const watch = (p) => {
      p.on('response', (r) => { if (r.status() === 404) notFound.push(new URL(r.url()).pathname); });
      p.on('dialog', (d) => { errors.push('a dialog of the browser: ' + d.type() + ' ' + d.message()); d.dismiss().catch(() => {}); });  // the page asks in its own dialogs
      p.on('pageerror', (e) => errors.push('pageerror: ' + e.message));
      p.on('console', (m) => { if (m.type() === 'error') errors.push('console: ' + m.text()); });
    };
    watch(page);
    const ok = (cond, what) => { if (!cond) throw new Error('FAIL: ' + what); };
    const until = (p, fn, arg, what, timeout = 4000) => p.waitForFunction(fn, arg, { timeout }).catch(() => { throw new Error('FAIL: ' + what); });
    const h1is = (p, x) => until(p, (v) => document.querySelector('h1') && document.querySelector('h1').textContent === v, x, 'the heading never became «' + x + '»');
    const msgHas = (p, text) => until(p, (x) => [...document.querySelectorAll('#msg, dialog[open] [data-msg]')].some((m) => m.textContent.includes(x)), text, 'the message never contained «' + text + '»');
    const dialog = page.locator('dialog.modal');
    const closeDialog = async () => { await page.keyboard.press('Escape'); await dialog.waitFor({ state: 'detached' }); };
    // A change in some dialogs draws the page again, and the dialog stays open (the same element, never closed and opened
    // again, so nothing flickers) with new contents: wait for them.
    const reopens = async (action, what) => {
      await page.evaluate(() => document.querySelectorAll('dialog.modal[open]').forEach((d) => { d.dataset.same = '1'; d.firstElementChild.dataset.old = '1'; }));
      await action();
      await until(page, () => document.querySelector('dialog.modal[open][data-same]') && !document.querySelector('dialog.modal [data-old]'), null, what);
      const text = await dialog.innerText();
      ok(!text.includes('[object'), 'a dialog shows a list as «[object …]»: ' + text.slice(0, 200));
    };
    // addRow (a dashed line with a name and «Add»), or a new plan (a field and «Create»)
    const addNamed = async (placeholder, name) => {
      const form = page.locator(`form:is(.addrow, .row):has(input[placeholder="${placeholder}"])`);
      // in the table the add lines are behind a + (the line's key is on both)
      const line = page.locator('tr.adds', { has: form });
      if ((await line.count()) && await line.isHidden()) await page.click(`button.addplus[data-adds="${await line.getAttribute('data-adds')}"]`);
      await form.locator('input').fill(name);
      await form.locator('button').click();
    };

    // --- the front page, organisations, switching ---
    await page.goto(base + '/');
    await page.waitForSelector('h1');
    ok((await page.textContent('h1')) === 'Choose organisation', 'the front page shows the organisation choice');
    ok((await page.getAttribute('html', 'lang')) === 'en', 'the page language is English');
    ok((await page.locator('#actor').count()) === 0, 'there is no name field: no change log, no actor');
    await page.fill('form.row input', 'Sports Club');
    await page.click('form.row button');
    await page.waitForURL('**/o/sports-club');
    await h1is(page, 'Sports Club');
    ok((await page.innerText('main')).includes('No plans yet'), 'a new organisation has no plans');
    await addNamed('New plan, e.g. Autumn 2026', 'Autumn 2026');
    await page.waitForURL('**/o/sports-club/p/autumn-2026');
    await h1is(page, 'Autumn 2026');

    await page.goto(base + '/?switch');
    await page.fill('form.row input', 'Other Org');
    await page.click('form.row button');
    await page.waitForURL('**/o/other-org');
    await page.selectOption('header select', 'sports-club');
    await page.waitForURL('**/o/sports-club');
    await until(page, () => document.body.innerText.includes('Autumn 2026'), null, 'plans follow the organisation');
    let before = errors.length;  // an unknown organisation is a 404 from the API, which is right
    await page.goto(base + '/o/does-not-exist');
    await h1is(page, 'Page not found');
    errors.length = before;
    for (const gone of ['/o/sports-club/setup', '/o/sports-club/p/autumn-2026/me/1']) {  // no setup page and no personal links
      await page.goto(base + gone);
      await h1is(page, 'Page not found');
    }
    errors.length = before;

    // --- the plan page: tasks, details rows, events ---
    const planUrl = '/o/sports-club/p/autumn-2026';
    const cols = () => page.locator('.dp-table thead th[scope=col]:not(.addcol)').count();
    await page.goto(base + planUrl);
    await h1is(page, 'Autumn 2026');
    for (const name of ['Sound', 'Video', 'Café']) {
      await addNamed('Add a task', name);
      await page.waitForSelector(`button[aria-label="Open the task ${name}"]`);
    }
    for (const name of ['Note', 'Theme']) {
      await addNamed('Add a row', name);
      await page.waitForSelector(`input.fname[aria-label="The name of the row ${name}"]`);
    }
    // a new event, every other week until a date
    await page.getByRole('button', { name: 'Add an event' }).click();
    await dialog.locator('input[aria-label="Date"]').fill('2026-08-02');
    await dialog.locator('input[aria-label="Starts"]').fill('11:00');
    ok(await dialog.locator('input[aria-label="The last date"]').isDisabled(), 'no last date until it repeats');
    await dialog.locator('select[aria-label="Repeat"]').selectOption({ label: 'Every other week' });
    await dialog.locator('input[aria-label="The last date"]').fill('2026-08-30');
    await until(page, () => document.querySelector('dialog.modal').innerText.includes('→ 3 events'), null, 'the dialog counts the events');
    await dialog.getByRole('button', { name: 'Add', exact: true }).click();
    await msgHas(page, '3 events added.');
    ok((await cols()) === 3, 'three Sundays, one column each');
    const ev = ['Sun 2 Aug 11:00', 'Sun 16 Aug 11:00', 'Sun 30 Aug 11:00'];
    const head = await page.innerText('.dp-table thead');
    ok(head.includes('Sun 2 Aug') && head.includes('11:00'), 'the header has the date and the time');
    ok(head.includes('Sun 16 Aug') && head.includes('Sun 30 Aug') && !head.includes('Sun 9 Aug'), 'every other week: ' + head.replace(/\s+/g, ' '));

    // places: typed straight in; Enter moves on to the next event
    const places = (task, e) => page.locator(`input[aria-label="Places for ${task} on ${e}"]`);
    // the same number on every event: ⇥ at the end of the row selects it, the bar at the bottom sets them
    await page.click('button[aria-label="Select the row Sound"]');
    await page.fill('.selbar input[type=number]', '1');
    await page.click('.selbar button:has-text("Set")');
    await until(page, () => [...document.querySelectorAll('tr.task-row')].find((r) => r.querySelector('.cellbtn').textContent === 'Sound')
      .querySelectorAll('input[type=number]').length === [...document.querySelectorAll('tr.task-row')].find((r) => r.querySelector('.cellbtn').textContent === 'Sound')
      .querySelectorAll('td.sel').length, null, 'the row is selected');
    for (const e of ev) await until(page, (x) => document.querySelector(`input[aria-label="${x}"]`).value === '1', 'Places for Sound on ' + e, 'the row puts 1 on ' + e);
    await page.keyboard.press('Escape');
    ok(!(await page.locator('dialog.modal input[aria-label^="Places for Sound on every"]').count()), 'no Fill row in the task dialog any more');
    await places('Café', ev[0]).fill('2');
    await places('Café', ev[0]).press('Enter');
    await places('Café', ev[1]).fill('1');
    await places('Café', ev[1]).press('Enter');
    await places('Video', ev[0]).fill('1');
    await places('Video', ev[0]).press('Tab');
    // a place of the draft (its cell), and ⋯ in it, which opens the place's dialog (a click on the cell itself does nothing)
    const draft = (task, e) => page.locator('td.draft', { has: page.locator(`button[aria-label="${task}, ${e}: open the place"]`) });
    const openPlace = (task, e) => page.locator(`button[aria-label="${task}, ${e}: open the place"]`).click();
    await until(page, () => !!document.querySelector('button[aria-label="Video, Sun 2 Aug 11:00: open the place"]'), null, 'a place gets a cell in the draft without a reload');
    ok((await draft('Sound', ev[0]).innerText()).includes('1 open place'), 'an open place before the first draft');
    await page.reload();
    await h1is(page, 'Autumn 2026');
    ok((await places('Café', ev[0]).inputValue()) === '2' && (await places('Video', ev[1]).inputValue()) === '', 'places stay after a reload; empty = not needed');
    ok((await page.locator('td.draft.none').count()) > 0, 'no places: nothing to fill in the draft');

    // ⚡ by a task: whether the planner fills it, as by a person; it is kept
    {
      const zap = () => page.locator('tr.task-row', { has: page.locator('button[aria-label="Open the task Video"]') }).locator('button.auto');
      ok((await zap().getAttribute('aria-pressed')) === 'true', 'the planner fills a new task');
      await zap().click();
      await until(page, () => document.querySelector('tr.task-row:has(button[aria-label="Open the task Video"]) button.auto').getAttribute('aria-pressed') === 'false', null, '⚡ off: only by hand');
      ok((await zap().getAttribute('title')).startsWith('Only by hand'), 'the title says so');
      await page.reload();
      await page.waitForSelector('.dp-table');
      ok((await zap().getAttribute('aria-pressed')) === 'false', 'it is kept');
      await zap().click();
      await until(page, () => document.querySelector('tr.task-row:has(button[aria-label="Open the task Video"]) button.auto').getAttribute('aria-pressed') === 'true', null, '⚡ on again');
      await page.waitForTimeout(300);
    }
    // tasks: new ones go last; ⋮⋮ drags one, and the dialog's «Place in the table» moves one too (for phones)
    const taskOrder = () => page.evaluate(() => [...document.querySelectorAll('tr.task-row .cellbtn')].map((b) => b.firstChild.textContent).join(','));
    ok((await taskOrder()) === 'Sound,Video,Café', 'tasks in the order they were added: ' + await taskOrder());
    const taskRowOf = (name) => page.locator('tr.task-row', { has: page.locator(`button[aria-label="Open the task ${name}"]`) });
    await taskRowOf('Café').locator('.grip').dragTo(taskRowOf('Sound'));
    await until(page, () => [...document.querySelectorAll('tr.task-row .cellbtn')].map((b) => b.firstChild.textContent).join(',') === 'Café,Sound,Video', null, 'dragging ⋮⋮ moves a task to the top');
    ok((await page.evaluate(() => [...document.querySelectorAll('tr.draft-row:not(.foot) th')].map((x) => x.textContent).join(','))) === 'Café,Sound,Video', 'the draft follows the order');
    await page.getByRole('button', { name: 'Open the task Café' }).click();
    await reopens(() => dialog.locator('select[aria-label="Place in the table"]').selectOption({ label: 'After Video' }), 'moving draws the page again');
    await closeDialog();
    ok((await taskOrder()) === 'Sound,Video,Café', 'the dialog moves a task: ' + await taskOrder());
    // task sections: headings like the people's; a task dropped on one goes last in it; the draft shows the heading too
    {
      const tsOrder = () => page.evaluate(() => [...document.querySelectorAll('tr.task-row .cellbtn, tr.task-sect input.fname')]
        .map((b) => (b.tagName === 'INPUT' ? '| ' + b.value : b.firstChild.textContent)).join(','));
      await addNamed('Add a task section', 'Tech');
      await page.waitForSelector('tr.task-sect');
      ok((await page.innerText('tr.task-sect')).includes('Drag tasks here'), 'a new task section is empty');
      await taskRowOf('Sound').locator('.grip').dragTo(page.locator('tr.task-sect'));
      await until(page, () => document.querySelector('tr.task-sect').innerText.includes('1 task'), null, 'a task dropped on its heading goes into it');
      ok((await tsOrder()) === 'Video,Café,| Tech,Sound', 'without a section first, then the section: ' + await tsOrder());
      ok((await page.evaluate(() => [...document.querySelectorAll('tr.draft-row:not(.foot) th, tr.draft-sect th')].map((x) => x.textContent).join(','))) === 'Video,Café,Tech,Sound', 'the draft has the heading too');
      await addNamed('Add a task to Tech', 'Lights');
      await until(page, () => document.querySelector('tr.task-sect').innerText.includes('2 tasks'), null, 'a task added in the section');
      ok((await tsOrder()) === 'Video,Café,| Tech,Sound,Lights', 'last in the section: ' + await tsOrder());
      await taskRowOf('Lights').locator('.grip').dragTo(taskRowOf('Sound'), { targetPosition: { x: 20, y: 3 } });
      await until(page, (want) => [...document.querySelectorAll('tr.task-row .cellbtn')].map((b) => b.firstChild.textContent).join(',') === want, 'Video,Café,Lights,Sound', 'dragged above a task in the section');
      await page.getByRole('button', { name: 'Open the task Video' }).click();
      await reopens(() => dialog.locator('select[aria-label="Section in the table"]').selectOption({ label: 'Tech' }), 'the task dialog opens again');
      await closeDialog();
      ok((await tsOrder()) === 'Café,| Tech,Lights,Sound,Video', 'the dialog puts a task last in a section: ' + await tsOrder());
      await taskRowOf('Video').locator('.grip').dragTo(page.locator('tr.sect', { hasText: 'Tasks' }).first());
      await until(page, (want) => [...document.querySelectorAll('tr.task-row .cellbtn')].map((b) => b.firstChild.textContent).join(',') === want, 'Café,Video,Lights,Sound', 'dropped on the Tasks heading: last of those without a section');
      const name = () => page.locator('tr.task-sect input.fname');
      await name().fill('');
      await name().press('Enter');
      await page.locator('dialog.ask[open]').getByRole('button', { name: 'Delete', exact: true }).click();
      await until(page, () => !document.querySelector('tr.task-sect'), null, 'the task section is gone');
      ok((await taskOrder()) === 'Café,Video,Lights,Sound', 'its tasks come back: ' + await taskOrder());
      await page.getByRole('button', { name: 'Open the task Lights' }).click();
      await dialog.getByRole('button', { name: 'Delete the task' }).click();
      await page.locator('dialog.ask[open]').getByRole('button', { name: 'Delete', exact: true }).click();
      await until(page, () => !document.querySelector('button[aria-label="Open the task Lights"]'), null, 'Lights deleted again');
      await taskRowOf('Café').locator('.grip').dragTo(taskRowOf('Video'), { targetPosition: { x: 20, y: 25 } });
      await until(page, (want) => [...document.querySelectorAll('tr.task-row .cellbtn')].map((b) => b.firstChild.textContent).join(',') === want, 'Video,Café,Sound', 'dragged below');
      await taskRowOf('Sound').locator('.grip').dragTo(taskRowOf('Video'), { targetPosition: { x: 20, y: 3 } });
      await until(page, (want) => [...document.querySelectorAll('tr.task-row .cellbtn')].map((b) => b.firstChild.textContent).join(',') === want, 'Sound,Video,Café', 'the order is back');
    }

    // details rows: a cell is a field; ⋮⋮ moves a row; the name is a field (empty deletes); the pin freezes; values are suggested
    const value = (row, e) => page.locator(`input[aria-label="${row} ${e}"]`);
    const fieldNames = () => page.evaluate(() => [...document.querySelectorAll('tr.field-row input.fname')].map((x) => x.value).join(','));
    const fieldRowOf = (name) => page.locator('tr.field-row', { has: page.locator(`input.fname[aria-label="The name of the row ${name}"]`) });
    await value('Note', ev[0]).fill('Kick-off');
    await value('Note', ev[0]).press('Enter');
    await until(page, (x) => document.activeElement && document.activeElement.getAttribute('aria-label') === x, 'Note ' + ev[1], 'Enter moves on to the next event');
    await value('Note', ev[1]).fill('Away');
    await value('Note', ev[1]).press('Tab');
    await page.waitForTimeout(200);
    ok((await page.locator('tr.field-row button.edit').count()) === 0, 'no ✎ menu on a details row any more');
    await fieldRowOf('Theme').locator('.grip').dragTo(fieldRowOf('Note'));
    await until(page, () => [...document.querySelectorAll('tr.field-row input.fname')].map((x) => x.value).join(',') === 'Theme,Note', null, 'dragging ⋮⋮ moves Theme to the top');
    // the values of a row are suggested; Enter takes the best match (and moves on); another spelling is saved as the one used
    await value('Note', ev[2]).fill('AWAY');
    await value('Note', ev[2]).press('Tab');
    await until(page, (x) => document.querySelector(`input[aria-label="${x}"]`).value === 'Away', 'Note ' + ev[2], 'a text in other letters is saved in the spelling used');
    await value('Note', ev[2]).fill('');
    await value('Note', ev[2]).pressSequentially('ki');
    const sugg = page.locator('tr.field-row td.text ul.elist:not([hidden]) li');
    ok((await sugg.allInnerTexts()).join('|') === 'Kick-off' && (await sugg.first().getAttribute('aria-selected')) === 'true', 'typing suggests the row’s text, picked: ' + (await sugg.allInnerTexts()).join('|'));
    await value('Note', ev[2]).press('Enter');
    await until(page, (x) => document.querySelector(`input[aria-label="${x}"]`).value === 'Kick-off', 'Note ' + ev[2], 'Enter takes the suggestion');
    await page.waitForTimeout(200);
    await page.reload();
    await page.waitForSelector('.dp-table');
    ok((await value('Note', ev[2]).inputValue()) === 'Kick-off', 'and it is saved');
    await value('Note', ev[2]).fill('');
    await value('Note', ev[2]).press('Tab');
    await page.waitForTimeout(200);
    // renamed in place (Enter), and back
    await fieldRowOf('Note').locator('input.fname').fill('Notes');
    await fieldRowOf('Note').locator('input.fname').press('Enter');
    await until(page, () => [...document.querySelectorAll('tr.field-row input.fname')].map((x) => x.value).join(',') === 'Theme,Notes', null, 'the name is changed in place');
    ok((await value('Notes', ev[0]).inputValue()) === 'Kick-off', 'the values stay with a new name');
    await fieldRowOf('Notes').locator('input.fname').fill('Note');
    await fieldRowOf('Notes').locator('input.fname').press('Enter');
    await until(page, () => [...document.querySelectorAll('tr.field-row input.fname')].map((x) => x.value).join(',') === 'Theme,Note', null, 'and back');
    // a name emptied deletes the row, asked in the page's own dialog
    await page.waitForTimeout(300);  // (the rename's redraw can still be on its way, and would take what is typed below with it)
    await addNamed('Add a row', 'Spare');
    await until(page, () => document.querySelectorAll('tr.field-row').length === 3, null, 'a row is added');
    await fieldRowOf('Spare').locator('input.fname').fill('');
    await fieldRowOf('Spare').locator('input.fname').press('Enter');
    await page.locator('dialog.ask[open]').getByRole('button', { name: 'Delete', exact: true }).click();
    await until(page, () => document.querySelectorAll('tr.field-row').length === 2, null, 'an emptied name deletes the row');
    ok((await fieldNames()) === 'Theme,Note', 'the others stay: ' + await fieldNames());
    // the pin freezes a row: it stays under the dates while the table scrolls, also after a reload; and back
    {
      const pin = () => fieldRowOf('Note').locator('button.freeze');
      ok((await pin().getAttribute('aria-pressed')) === 'false', 'a row is not frozen at first');
      await pin().click();
      await until(page, () => !!document.querySelector('tr.field-row.frozen button.freeze[aria-pressed=true]'), null, 'the pin freezes the row');
      await page.reload();
      await page.waitForSelector('tr.field-row.frozen');
      await page.setViewportSize({ width: 1280, height: 330 });  // (the table is short so far: a low window makes it scroll)
      await page.waitForTimeout(150);
      const under = await page.evaluate(() => {
        const box = document.querySelector('.grid-scroll.main');
        box.scrollTop = box.scrollHeight;
        const head = box.querySelector('thead th[scope=col]').getBoundingClientRect().bottom;
        const row = box.querySelector('tr.field-row.frozen td').getBoundingClientRect().top;
        const theme = box.querySelector('tr.field-row:not(.frozen) td').getBoundingClientRect().top;
        box.scrollTop = 0;
        return { head, row, theme };
      });
      await page.setViewportSize({ width: 1280, height: 900 });
      ok(Math.abs(under.row - under.head) < 2 && under.theme < under.head, 'scrolled down, the frozen row stays right under the dates, the other goes: ' + JSON.stringify(under));
      await pin().click();
      await until(page, () => !document.querySelector('tr.field-row.frozen'), null, 'a second click unfreezes it');
    }
    // every other date is shaded, down the table
    ok(await page.evaluate(() => { const tds = document.querySelector('tr.field-row').querySelectorAll('td.text');
      return getComputedStyle(tds[0]).backgroundImage !== 'none' && getComputedStyle(tds[1]).backgroundImage === 'none'; }) === true ||
       await page.evaluate(() => { const tds = document.querySelector('tr.field-row').querySelectorAll('td.text');
      return getComputedStyle(tds[0]).backgroundImage === 'none' && getComputedStyle(tds[1]).backgroundImage !== 'none'; }), 'every other date column is shaded');
    ok((await value('Note', ev[0]).inputValue()) === 'Kick-off', 'a value stays');

    // the event dialog: times and details, the end needs a start
    await page.click(`button[aria-label="Open the event ${ev[2]}"]`);
    await dialog.locator('input[aria-label="Ends"]').fill('12:30');
    await dialog.locator('input[aria-label="Theme"]').fill('Finals');
    await dialog.getByRole('button', { name: 'Save' }).click();
    await dialog.waitFor({ state: 'detached' });
    await until(page, () => document.querySelector('.dp-table thead').innerText.includes('11:00–12:30'), null, 'the end time shows in the header');
    ok((await value('Theme', ev[2]).inputValue()) === 'Finals', 'a details value set in the event dialog');

    // --- people, answers, tasks ---
    for (const name of ['Kari Test', 'Ola Sample', 'Jon Demo']) {
      await addNamed('Add a person', name);
      await page.waitForSelector(`button[aria-label="Open ${name}"]`);
    }
    // people in the order they were added; ⋮⋮ drags one, and sections are headings to place people under
    const peopleOrder = () => page.evaluate(() => [...document.querySelectorAll('tr.person-row .cellbtn, tr.person-sect input.fname')]
      .map((b) => (b.matches('input') ? '| ' + b.value : b.firstChild.textContent)).join(','));
    ok((await peopleOrder()) === 'Kari Test,Ola Sample,Jon Demo', 'people in the order they were added: ' + await peopleOrder());
    const personRowOf = (name) => page.locator('tr.person-row', { has: page.locator(`button[aria-label="Open ${name}"]`) });
    const nameOf = (name) => page.locator(`button[aria-label="Open ${name}"]`);
    await personRowOf('Kari Test').evaluate((e) => e.scrollIntoView({ block: 'center' }));
    await personRowOf('Jon Demo').locator('.grip').dragTo(personRowOf('Kari Test'), { targetPosition: { x: 20, y: 3 } });
    await until(page, () => [...document.querySelectorAll('tr.person-row .cellbtn')].map((b) => b.firstChild.textContent).join(',') === 'Jon Demo,Kari Test,Ola Sample', null, 'dragging ⋮⋮ to the top half of a row puts a person above it');
    // the whole name is the handle (a click still opens); a line shows where it lands; the lower half puts it below
    {
      const box = await nameOf('Jon Demo').boundingBox();
      const to = await personRowOf('Kari Test').boundingBox();
      await page.mouse.move(box.x + 10, box.y + box.height / 2);
      await page.mouse.down();
      await page.mouse.move(box.x + 30, to.y + to.height - 4, { steps: 4 });
      ok(await page.locator('.drop-line').isVisible() && await page.locator('.row-ghost').innerText() === 'Jon Demo', 'a line shows where the row lands, the name follows the pointer');
      const line = await page.locator('.drop-line').boundingBox();
      ok(Math.abs(line.y + line.height / 2 - (to.y + to.height)) < 3, 'the line is under the row: ' + JSON.stringify([line.y, to.y + to.height]));
      await page.mouse.up();
      await until(page, () => [...document.querySelectorAll('tr.person-row .cellbtn')].map((b) => b.firstChild.textContent).join(',') === 'Kari Test,Jon Demo,Ola Sample', null, 'dragging the name to the lower half puts it below');
      ok((await page.locator('dialog[open]').count()) === 0, 'and the drag opens no dialog');
      // Escape lets go without moving anything
      const b2 = await nameOf('Ola Sample').boundingBox();
      await page.mouse.move(b2.x + 10, b2.y + b2.height / 2);
      await page.mouse.down();
      await page.mouse.move(b2.x + 20, to.y + 3, { steps: 4 });
      await page.keyboard.press('Escape');
      await page.mouse.up();
      await page.waitForTimeout(300);
      ok((await peopleOrder()) === 'Kari Test,Jon Demo,Ola Sample' && !(await page.locator('.drop-line').count()), 'Escape lets go: ' + await peopleOrder());
      await nameOf('Ola Sample').click();
      ok((await page.locator('dialog.modal[open]').count()) === 1, 'a plain click on the name still opens it');
      await closeDialog();
    }
    await addNamed('Add a section', 'Not this term');
    await page.waitForSelector('tr.person-sect');
    ok((await page.innerText('tr.person-sect')).includes('Drag people here'), 'a new section is empty');
    await page.locator('tr.person-sect').evaluate((e) => e.scrollIntoView({ block: 'center' }));  // in the middle: near the window's edge a drag scrolls it
    await personRowOf('Ola Sample').locator('.grip').dragTo(page.locator('tr.person-sect'));
    await until(page, () => document.querySelector('tr.person-sect').innerText.includes('1 person'), null, 'a person dropped on a section heading goes into it');
    ok((await peopleOrder()) === 'Kari Test,Jon Demo,| Not this term,Ola Sample', 'without a section first, then the section: ' + await peopleOrder());
    await personRowOf('Kari Test').locator('.grip').dragTo(page.locator('tr.person-sect'));
    await until(page, () => document.querySelector('tr.person-sect').innerText.includes('2 people'), null, 'a second one dropped on the heading');
    ok((await peopleOrder()) === 'Jon Demo,| Not this term,Ola Sample,Kari Test', 'goes last in the section: ' + await peopleOrder());
    await personRowOf('Kari Test').locator('.grip').dragTo(page.locator('tr.sect', { hasText: 'Availability' }));
    await until(page, () => document.querySelector('tr.person-sect').innerText.includes('1 person'), null, 'dropped on the Availability heading');
    ok((await peopleOrder()) === 'Jon Demo,Kari Test,| Not this term,Ola Sample', 'goes last of those without a section: ' + await peopleOrder());
    // the person dialog places someone in a section too (for phones)
    await page.click('button[aria-label="Open Jon Demo"]');
    await reopens(() => dialog.locator('select[aria-label="Section in the table"]').selectOption({ label: 'Not this term' }), 'the person dialog opens again');
    await closeDialog();
    ok((await peopleOrder()) === 'Kari Test,| Not this term,Ola Sample,Jon Demo', 'the dialog puts a person last in the section: ' + await peopleOrder());
    // «Add a person» lines: behind a + at the end of the last name of each list; − closes; the new person lands there
    {
      const plusOf = (key) => page.locator(`button.addplus[data-adds="${key}"]`);
      const sectKey = await page.locator('tr.adds', { has: page.locator('input[placeholder="Add a person to Not this term"]') }).getAttribute('data-adds');
      ok(await page.locator('input[placeholder="Add a person to Not this term"]').isHidden(), 'the add line of a section is hidden at first');
      ok(await personRowOf('Jon Demo').locator(`button.addplus[data-adds="${sectKey}"]`).count() === 1, 'its + is in the last name of the section');
      await plusOf(sectKey).click();
      ok(await page.locator('input[placeholder="Add a person to Not this term"]').isVisible() && (await plusOf(sectKey).innerText()) === '−', '+ shows the line and turns into −');
      ok(await page.evaluate(() => document.activeElement.placeholder) === 'Add a person to Not this term', 'ready to type');
      await plusOf(sectKey).click();
      ok(await page.locator('input[placeholder="Add a person to Not this term"]').isHidden() && (await plusOf(sectKey).innerText()) === '+', '− hides it again');
      await addNamed('Add a person to Not this term', 'Pia Demo');
      await until(page, () => document.querySelectorAll('tr.person-row').length === 4, null, 'the new person is added');
      ok((await peopleOrder()) === 'Kari Test,| Not this term,Ola Sample,Jon Demo,Pia Demo', 'added in the section, last: ' + await peopleOrder());
      ok(await page.locator('input[placeholder="Add a person to Not this term"]').isVisible(), 'the line stays open for the next');
      await page.locator('input[placeholder="Add a person to Not this term"]').press('Escape');
      ok(await page.locator('input[placeholder="Add a person to Not this term"]').isHidden(), 'Escape closes it');
    }
    // a section's name is a field: renamed in place; emptied, the section goes (asked) and its people come back
    {
      const sectName = () => page.locator('tr.person-sect input.fname');
      await sectName().fill('Away');
      await sectName().press('Enter');
      await until(page, () => document.querySelector('tr.person-sect input.fname').value === 'Away', null, 'the section is renamed in place');
      await page.waitForTimeout(300);  // the rename's redraw, so the field emptied next is the new one
      await sectName().fill('');
      await sectName().press('Enter');
      await page.locator('dialog.ask[open]').getByRole('button', { name: 'Delete', exact: true }).click();  // asked in the page's own dialog
      await until(page, () => !document.querySelector('tr.person-sect'), null, 'the section is gone');
      ok((await peopleOrder()) === 'Kari Test,Ola Sample,Jon Demo,Pia Demo', 'its people come back in their order: ' + await peopleOrder());
    }
    const takes = async (person, list) => {
      await page.click(`button[aria-label="Open ${person}"]`);
      // one click per step: can’t → if needed → good
      for (const [task, level] of list) {
        const btn = () => dialog.locator(`button.cyc[aria-label^="${task} for ${person}:"]`);
        ok((await btn().getAttribute('data-level')) === 'none', 'a task starts as can’t: ' + task);
        for (let i = 0; i < { poor: 1, good: 2 }[level]; i++) await reopens(() => btn().click(), 'the person dialog opens again');
        ok((await btn().getAttribute('data-level')) === level, 'the change is in the new dialog: ' + (await btn().getAttribute('aria-label')));
      }
      await closeDialog();
    };
    // quick clicks are not dropped (the save of the first is still under way): two at once go to good, and that is what is saved
    {
      const pia = () => dialog.locator('button.cyc[aria-label^="Sound for Pia Demo:"]');
      const level = async () => { await page.click('button[aria-label="Open Pia Demo"]'); const l = await pia().getAttribute('data-level'); await closeDialog(); return l; };
      await page.click('button[aria-label="Open Pia Demo"]');
      await pia().evaluate((b) => { b.click(); b.click(); });
      ok((await pia().getAttribute('data-level')) === 'good', 'two quick clicks show good at once');
      await closeDialog();
      await page.waitForTimeout(600);
      await page.reload();
      await page.waitForSelector('.dp-table');
      ok((await level()) === 'good', 'and good is saved');
      await page.click('button[aria-label="Open Pia Demo"]');
      await reopens(() => pia().click(), 'the person dialog opens again');
      await closeDialog();
      ok((await level()) === 'none', 'one more click: can’t again');
    }
    await takes('Kari Test', [['Sound', 'good'], ['Video', 'poor']]);
    await takes('Ola Sample', [['Sound', 'good']]);
    await takes('Jon Demo', [['Sound', 'poor'], ['Video', 'good']]);
    ok((await page.getAttribute('button[aria-label="Open Kari Test"]', 'title')) === 'Sound, Video', 'hovering a name shows the tasks the person takes');
    await page.click('button[aria-label="Open Kari Test"]');
    ok((await dialog.locator('input[readonly]').count()) === 0, 'the person dialog has no personal link');
    await closeDialog();

    const answer = (person, e) => page.locator(`button.ans[aria-label^="${person}, ${e}:"]`);
    await answer('Kari Test', ev[0]).click();
    await until(page, (x) => document.querySelector(`button.ans[aria-label^="${x}:"]`).dataset.level === 'good', 'Kari Test, ' + ev[0], 'one click gives ✓');
    await answer('Kari Test', ev[0]).click();
    await until(page, (x) => document.querySelector(`button.ans[aria-label^="${x}:"]`).dataset.level === 'poor', 'Kari Test, ' + ev[0], 'a second click gives ?');
    ok((await page.locator('td.count >> nth=0').getAttribute('aria-label')) === '0 yes, 1 maybe, 0 no', 'the count follows the answers');
    await answer('Jon Demo', ev[1]).click();
    await answer('Jon Demo', ev[1]).click();
    await answer('Jon Demo', ev[1]).click();
    await until(page, (x) => document.querySelector(`button.ans[aria-label^="${x}:"]`).dataset.level === 'never', 'Jon Demo, ' + ev[1], 'three clicks give ✗');
    // On a slow line, quick clicks while the first answer is on its way all count, and reach the server in order (four go
    // round to empty again, as later steps expect).
    {
      await page.route('**/availability', async (route) => { await new Promise((r) => setTimeout(r, 400)); await route.continue(); });
      for (let i = 0; i < 4; i++) await answer('Jon Demo', ev[2]).click();
      await until(page, (x) => document.querySelector(`button.ans[aria-label^="${x}:"]`).dataset.level === 'none', 'Jon Demo, ' + ev[2], 'four quick clicks go round to empty at once');
      await page.waitForTimeout(2000);
      await page.unroute('**/availability');
      const fresh = await context.newPage();  // a new tab reads what the server has (a reload here would show in the next check)
      await fresh.goto(page.url());
      await until(fresh, (x) => (document.querySelector(`button.ans[aria-label^="${x}:"]`) || {}).dataset?.level === 'none', 'Jon Demo, ' + ev[2], 'the server has all four clicks');
      await fresh.close();
    }
    // An answer is shown at once and only sent: nothing is fetched again, so nothing flickers.
    {
      const gets = [];
      const seen = (r) => { const x = readOf(r); if (x) gets.push(x); };
      page.on('request', seen);
      await answer('Ola Sample', ev[0]).click();
      ok((await answer('Ola Sample', ev[0]).getAttribute('data-level')) === 'good', 'the answer shows at once');
      await page.waitForTimeout(400);
      page.off('request', seen);
      ok(gets.length === 0, 'an answer fetches nothing again: ' + gets.join(', '));
      await answer('Ola Sample', ev[0]).click();
      await answer('Ola Sample', ev[0]).click();
      await answer('Ola Sample', ev[0]).click();
      await until(page, (x) => document.querySelector(`button.ans[aria-label^="${x}:"]`).dataset.level === 'none', 'Ola Sample, ' + ev[0], 'four clicks give empty again');
    }
    // ⚡ by the name: whether the planner may pick the person, one click, kept
    {
      const auto = () => page.locator('tr.person-row:has(button[aria-label="Open Jon Demo"]) button.auto');
      ok((await auto().getAttribute('aria-pressed')) === 'true', 'a person can be proposed from the start');
      await auto().click();
      ok((await auto().getAttribute('aria-pressed')) === 'false', 'one click: only by hand');
      await page.reload();
      await auto().waitFor();
      ok((await auto().getAttribute('aria-pressed')) === 'false', 'the choice is kept');
      await auto().click();
      await until(page, () => document.querySelector('tr.person-row:has(button[aria-label="Open Jon Demo"]) button.auto').getAttribute('aria-pressed') === 'true', null, 'and back');
      await page.click('button[aria-label="Open Jon Demo"]');
      ok((await dialog.locator('input[type=checkbox]').count()) === 0, 'the person dialog has no «can be proposed» box any more');
      await closeDialog();
    }
    // A change in a dialog fetches again only what it can have changed: a person's task → their answers and the drafts.
    {
      const gets = [];
      const seen = (r) => { const x = readOf(r); if (x) gets.push(x); };
      await page.click('button[aria-label="Open Kari Test"]');
      const btn = () => dialog.locator('button.cyc[aria-label^="Video for Kari Test:"]');
      const was = await btn().getAttribute('data-level');
      await reopens(() => btn().click(), 'the person dialog is drawn again');  // (what earlier changes left out of date is fetched here)
      page.on('request', seen);
      await reopens(() => btn().click(), 'the person dialog is drawn again');
      page.off('request', seen);
      ok(gets.length > 0 && gets.every((x) => ['ListAnswers', 'ListRuns', 'GetRun'].includes(x)), 'a task level fetches only the answers and the drafts again: ' + gets.join(', '));
      while ((await btn().getAttribute('data-level')) !== was) await reopens(() => btn().click(), 'and back');
      await closeDialog();
    }
    // A tab opened before a deploy: an answer with a field this page does not know is still read
    {
      let type = '';
      const newer = async (route) => {
        const res = await route.fetch();
        type = res.headers()['content-type'];
        // field 999, a varint 1: a field a later server may add
        route.fulfill({ response: res, body: Buffer.concat([await res.body(), Buffer.from([0xb8, 0x3e, 0x01])]) });
      };
      await page.route('**/dutyplan.v1.OrgService/GetOrg', newer);
      await page.reload();
      await page.waitForSelector('button.ans');
      ok(!(await page.locator('#conn').count()), 'an unknown field in an answer is ignored');
      ok(type === 'application/proto', 'the page talks binary protobuf: ' + type);
      await page.unroute('**/dutyplan.v1.OrgService/GetOrg', newer);
    }
    // Signed out (the sign-in in front answers with a redirect to its own site): a bar with «Sign in again», never «Failed to fetch»
    {
      const signedOut = (route) => route.fulfill({ status: 307, headers: { Location: 'https://auth.example.invalid/login' } });
      await page.route('**/dutyplan.v1.**', signedOut);
      const cell = answer('Ola Sample', ev[1]);
      const was = await cell.getAttribute('data-level');
      await cell.click();
      await page.waitForSelector('#conn.signed_out');
      const bar = await page.innerText('#conn');
      ok(bar.includes('signed out') && bar.includes('Sign in again'), 'the bar offers to sign in again: ' + bar);
      ok((await cell.getAttribute('data-level')) === was, 'the click is taken back');
      const body = await page.innerText('body');
      ok(!/failed to fetch|\[object/i.test(body), 'no «Failed to fetch» or «[object» on the page');
      // in a dialog, which covers the bar, the dialog says it too
      await page.unroute('**/dutyplan.v1.**', signedOut);
      await page.click('button[aria-label="Open Kari Test"]');
      await page.route('**/dutyplan.v1.**', signedOut);
      await dialog.locator('button.cyc').first().click();
      await until(page, () => (document.querySelector('dialog[open] [data-msg]') || {}).textContent.includes('Sign in again'), null, 'the dialog offers to sign in again');
      await page.unroute('**/dutyplan.v1.**', signedOut);
      await closeDialog();
      await page.click('#conn button');  // a reload: through the sign-in and back
      await page.waitForSelector('button.ans');
      ok(!(await page.locator('#conn').count()), 'signed in again, the bar is gone');
      ok((await cell.getAttribute('data-level')) === was, 'nothing was saved while signed out');
    }
    // The server out of reach: a bar with «Try again», and the page stays as it was
    {
      const down = (route) => route.abort('connectionrefused');
      await page.route('**/dutyplan.v1.**', down);
      await answer('Ola Sample', ev[1]).click();
      await page.waitForSelector('#conn.offline');
      ok((await page.innerText('#conn')).includes('Try again'), 'the bar offers to try again');
      ok(await page.locator('button.ans').count() > 0, 'the table is still there');
      await page.unroute('**/dutyplan.v1.**', down);
      await page.click('#conn button');
      await until(page, () => !document.querySelector('#conn'), null, 'back again, the bar is gone');
      // the browser logs the refused request it was made to make; that is the test, not a fault of the page
      for (let i = errors.length - 1; i >= 0; i--) if (errors[i].includes('ERR_CONNECTION_REFUSED')) errors.splice(i, 1);
    }
    // ⓘ: the explanation in a dialog, not on the page
    await page.click('button[aria-label="About Availability"]');
    ok((await dialog.innerText()).includes('One row per person'), 'the ⓘ explains the section');
    await closeDialog();
    // Live: a change in another tab shows here without a reload, and the place sideways in the table is kept.
    {
      // another browser (its own context; CSP bypassed only so the test can make the table narrow)
      const otherContext = await browser.newContext({ locale: 'en-GB', viewport: { width: 1280, height: 900 }, bypassCSP: true });
      const other = await otherContext.newPage();
      watch(other);
      await other.goto(page.url());
      await other.locator('button.ans').first().waitFor();
      await other.evaluate(() => { window.notReloaded = true; });
      const grid = other.locator('.grid-scroll.main');
      await other.addStyleTag({ content: '.grid-scroll.main { max-width: 320px; }' });  // narrow, so it scrolls sideways
      await grid.evaluate((g) => { g.scrollLeft = 60; });
      const before = await grid.evaluate((g) => g.scrollLeft);
      ok(before > 0, 'the table scrolls sideways in the test');
      const otherGets = [];
      other.on('request', (r) => { const x = readOf(r); if (x) otherGets.push(x); });
      await answer('Jon Demo', ev[2]).click();
      await until(other, (x) => document.querySelector(`button.ans[aria-label^="${x}:"]`).dataset.level === 'good', 'Jon Demo, ' + ev[2], 'the other tab shows the answer by itself');
      ok(await other.evaluate(() => window.notReloaded === true), 'without a reload');
      ok(otherGets.every((x) => ['ListAnswers', 'ListRuns', 'GetRun'].includes(x)), 'the other tab fetches only the answers and the drafts: ' + otherGets.join(', '));
      ok((await grid.evaluate((g) => g.scrollLeft)) === before, 'and stays where it was sideways');
      await otherContext.close();
    }

    await page.click('button[aria-label="Open the task Sound"]');
    // one click per step: a pair starts as bad → good → can't
    const pair = () => dialog.locator('button.cyc[aria-label^="Sound + Video:"]');
    ok((await pair().getAttribute('data-level')) === 'standard', 'a pair starts as bad');
    await reopens(() => pair().click(), 'the task dialog opens again');
    ok((await pair().getAttribute('aria-label')) === 'Sound + Video: Good', 'one click gives good');
    await reopens(() => pair().click(), 'the task dialog opens again');
    ok((await pair().getAttribute('data-level')) === 'cannot', 'the combination is set to can’t');
    await closeDialog();

    // --- the draft: by hand, make a draft, lock, try again, pin ---
    await openPlace('Café', ev[0]);
    await dialog.locator('input[aria-label="Text"]').fill('Outside guest');
    await dialog.locator('input[aria-label="Places it fills"]').fill('2');
    await reopens(() => dialog.getByRole('button', { name: 'Set it' }).click(), 'the place dialog opens again');
    ok((await dialog.innerText()).includes('Outside guest ×2'), 'the text is in the place');
    await closeDialog();
    ok((await draft('Café', ev[0]).locator('.slot.hand').innerText()) === 'Outside guest ×2', 'set by hand is violet in the draft');
    // several people by hand at once, more than the places
    const sound2places = await places('Sound', ev[2]).inputValue();
    await openPlace('Sound', ev[2]);
    for (const n of ['Kari Test', 'Jon Demo', 'Ola Sample']) await dialog.locator(`input[aria-label="Set ${n} by hand"]`).check();
    await reopens(() => dialog.getByRole('button', { name: 'Add the people ticked' }).click(), 'the place dialog opens again');
    ok((await dialog.locator('input[aria-label="Set Kari Test by hand"]').count()) === 0, 'someone set by hand is not offered again');
    await closeDialog();
    const hand2 = await draft('Sound', ev[2]).locator('.slot.hand').allInnerTexts();
    ok(hand2.length === 3 && Number(sound2places) === 1, 'three by hand in a place for ' + sound2places + ': ' + hand2.join(', '));
    ok((await page.locator('.draft-bar').innerText()).includes('No draft yet'), 'no draft yet');

    await page.getByRole('button', { name: 'Make a draft' }).click();
    const barHas = (x, what) => until(page, (v) => document.querySelector('.draft-bar') && document.querySelector('.draft-bar').innerText.includes(v), x, what, 20000)
      .catch(async (e) => { throw new Error(e.message + ': ' + (await page.innerText('.draft-bar').catch(() => '(no bar)'))); });
    const runIs = (n, what) => until(page, (v) => (document.querySelector('.draft-bar .run-id') || {}).textContent === v, '#' + n, what, 20000);
    await runIs(1, 'the first draft is shown');
    let sound0 = await draft('Sound', ev[0]).innerText();
    ok(/Kari Test|Ola Sample|Jon Demo/.test(sound0), 'the planner filled Sound: ' + sound0);
    const cafe1 = await draft('Café', ev[1]).innerText();
    ok(cafe1.includes('Not filled') && cafe1.includes('Why?'), 'an unfilled place says so: ' + cafe1);
    await openPlace('Café', ev[1]);
    const why = await dialog.innerText();
    ok(why.includes('Not filled:') && why.includes('have not chosen the task'), 'the place dialog explains: ' + why);
    await closeDialog();

    await openPlace('Sound', ev[0]);
    const lockBtn = dialog.getByRole('button', { name: /^Lock / });
    const lockedName = (await lockBtn.getAttribute('aria-label')).slice(5);
    await reopens(() => lockBtn.click(), 'the place dialog opens again');
    ok((await dialog.innerText()).includes('Unlock'), 'a locked name can be unlocked');
    await closeDialog();
    sound0 = await draft('Sound', ev[0]).innerText();
    const lockMark = () => draft('Sound', ev[0]).locator(`.slot.locked button.lockbtn.on[aria-label="Unlock ${lockedName}"]`).count();
    ok(sound0.includes(lockedName) && (await lockMark()) === 1, 'the locked name has a pressed lock: ' + sound0);
    await page.getByRole('button', { name: 'Try again' }).click();
    await runIs(2, 'try again makes a second draft');
    sound0 = await draft('Sound', ev[0]).innerText();
    ok(sound0.includes(lockedName) && (await lockMark()) === 1, 'the locked name is kept: ' + sound0);

    await page.getByRole('button', { name: 'Make it the current plan' }).click();
    await msgHas(page, 'Draft #2 is now the current plan.');
    await until(page, () => !!document.querySelector('.draft-bar button.pin[aria-pressed=true]'), null, 'the pinned draft is the current plan');
    // an answer changes: the draft is out of date
    await answer('Ola Sample', ev[2]).click();
    await until(page, (x) => document.querySelector(`button.ans[aria-label^="${x}:"]`).dataset.level === 'good', 'Ola Sample, ' + ev[2], 'the answer is saved');
    await page.reload();
    await barHas('have changed since this draft was made', 'an out-of-date draft is marked');
    // earlier drafts, under ⚙
    await page.getByRole('button', { name: 'Plan settings' }).click();
    ok((await dialog.innerText()).includes('Draft #1') && (await dialog.innerText()).includes('Current plan'), 'the earlier drafts are listed with the current plan');
    await dialog.getByRole('button', { name: 'Show draft #1' }).click();
    await runIs(1, 'an earlier draft can be shown');
    await barHas('Show the newest', 'and the newest is one click away');
    await page.getByRole('button', { name: 'Show the newest' }).click();
    await runIs(2, 'back to the newest');
    // the distribution, closed with its own Close button (as every dialog is)
    await page.getByRole('button', { name: 'Distribution', exact: true }).click();
    ok((await dialog.innerText()).includes('fair share'), 'the distribution lists the load against the fair share');
    await dialog.locator('.modal-foot').getByRole('button', { name: 'Close', exact: true }).click();
    await dialog.waitFor({ state: 'detached' });

    // --- locking in the draft: the lock by a name, a row selected, Lock all; then Remove unlocked, and back with a pin ---
    {
      const cell = (task, e) => `button[aria-label="${task}, ${e}: open the place"]`;
      const inCell = (sel, what) => until(page, ([c, q]) => { const td = document.querySelector(c)?.closest('td'); return !!td && !!td.querySelector(q); }, [cell('Sound', ev[1]), sel], what);
      await draft('Sound', ev[1]).locator('.nm').first().click();
      ok((await page.locator('dialog[open]').count()) === 0, 'a click on a name in the draft opens no dialog');
      await page.keyboard.press('Escape');  // (it opens the field for a name: see below)
      const one = draft('Sound', ev[1]).locator('button.lockbtn:not(.on)').first();
      const who = (await one.getAttribute('aria-label')).slice(5);
      await one.click();
      await inCell(`button.lockbtn.on[aria-label="Unlock ${who}"]`, 'the lock by a name locks it');
      const soundRow = () => page.evaluate(() => {
        const tr = [...document.querySelectorAll('tr.draft-row')].find((x) => x.querySelector('th').textContent === 'Sound');
        return { on: tr.querySelectorAll('button.lockbtn.on').length, off: tr.querySelectorAll('button.lockbtn:not(.on)').length };
      });
      await page.locator('tr.draft-row', { has: page.locator('th', { hasText: /^Sound$/ }) }).locator('button.rowpick').click();
      ok((await page.locator('.selbar').innerText()).includes('Unlock'), 'a row of the draft selected gives Lock and Unlock');
      await page.locator('.selbar').getByRole('button', { name: 'Unlock', exact: true }).click();
      await until(page, () => !document.querySelector('tr.draft-row button.lockbtn.on'), null, 'Unlock in the bar unlocks the row’s names');
      await page.locator('.selbar').getByRole('button', { name: 'Lock', exact: true }).click();
      await until(page, () => { const tr = [...document.querySelectorAll('tr.draft-row')].find((x) => x.querySelector('th').textContent === 'Sound');
        return !tr.querySelector('button.lockbtn:not(.on)') && !!tr.querySelector('button.lockbtn.on'); }, null, 'Lock in the bar locks every name in the row');
      ok((await soundRow()).off === 0, 'the whole row is locked');
      await page.keyboard.press('Escape');
      // a whole row and a whole date by their own locks
      const rowLock = (task) => page.locator(`tr.draft-row button.lockbtn.group[aria-label$="every name for ${task}"]`);
      ok((await rowLock('Sound').getAttribute('aria-pressed')) === 'true', 'the row lock shows the row is locked');
      await rowLock('Sound').click();
      await until(page, () => !document.querySelector('tr.draft-row:has(button[aria-label="Unlock every name for Sound"])') &&
        ![...document.querySelectorAll('tr.draft-row')].find((x) => x.querySelector('th .nm')?.textContent === 'Sound').querySelector('td button.lockbtn.on'), null, 'the row lock unlocks the row');
      await rowLock('Sound').click();
      await until(page, () => !![...document.querySelectorAll('tr.draft-row')].find((x) => x.querySelector('th .nm')?.textContent === 'Sound').querySelector('td button.lockbtn.on'), null, 'and locks it again');
      ok((await soundRow()).off === 0, 'every name in the row');
      const colLock = page.locator(`tr.draft-locks button[aria-label^="Lock every name on ${ev[0]}"]`);
      await colLock.click();
      await until(page, (c) => !document.querySelector(c), `tr.draft-locks button[aria-label^="Lock every name on ${ev[0]}"]`, 'the date lock locks a date');
      ok((await draft('Video', ev[0]).locator('button.lockbtn:not(.on)').count()) === 0, 'every name on that date is locked');
      // pinned (above) and every name locked: done. «Reopen» unpins and keeps the locks; «We’re done» locks every name, pins the
      // draft and shows it in View
      ok(!(await page.getByRole('button', { name: 'Lock all' }).count()), 'no Lock all any more');
      await barHas('Done', 'a pinned draft with every name locked is done');
      await page.getByRole('button', { name: 'Reopen' }).click();
      await until(page, () => document.querySelector('.draft-bar button.pin[aria-pressed="false"]'), null, 'Reopen unpins it');
      ok(!(await page.locator('tr.draft-row button.lockbtn:not(.on)').count()), 'the names stay locked');
      await rowLock('Video').click();
      await until(page, () => document.querySelector('tr.draft-row button.lockbtn:not(.on)'), null, 'a row unlocked');
      await page.getByRole('button', { name: 'We’re done' }).click();
      await until(page, () => document.querySelector('.mode button[data-mode="show"][aria-pressed="true"]'), null, 'Done shows the plan in View');
      await page.locator('.mode button[data-mode="edit"]').click();
      await barHas('Reopen', 'the bar says done');
      ok(!(await page.locator('tr.draft-row button.lockbtn:not(.on)').count()), 'Done locked every name');
      ok((await page.locator('.draft-bar button.pin').getAttribute('aria-pressed')) === 'true', 'and pinned the draft');
      await page.getByRole('button', { name: 'Reopen' }).click();
      await until(page, () => document.querySelector('.draft-bar button.pin[aria-pressed="false"]'), null, 'reopened again');
      // one unlocked again; Remove unlocked takes only that one, asked in the page's own dialog
      await draft('Sound', ev[1]).locator(`button.lockbtn.on[aria-label="Unlock ${who}"]`).click();
      await inCell(`button.lockbtn:not(.on)[aria-label="Lock ${who}"]`, 'the lock by a name unlocks it');
      await page.getByRole('button', { name: 'Remove unlocked' }).click();
      await page.locator('dialog.ask[open]').getByRole('button', { name: 'Remove', exact: true }).click();
      await barHas('Unlocked names removed', 'Remove unlocked leaves no draft on show');
      ok(!(await draft('Sound', ev[1]).innerText()).includes(who), 'the unlocked name went');
      ok((await draft('Sound', ev[0]).locator('button.lockbtn.on').count()) > 0, 'the locked names stay');
      ok((await draft('Café', ev[0]).locator('.slot.hand').innerText()) === 'Outside guest ×2', 'what is set by hand stays');
      await page.reload();
      await barHas('Unlocked names removed', 'no draft on show after a reload either');
      await page.getByRole('button', { name: 'Plan settings' }).click();
      await dialog.getByRole('button', { name: 'Make draft #2 the current plan' }).click();
      await runIs(2, 'pinning a draft shows it again');
      await closeDialog();
    }

    // --- undo in the draft (button and Ctrl+Z), and the drafts one by one (‹ ›, Show changes) ---
    {
      const unlocked = () => page.locator('tr.draft-row td button.lockbtn:not(.on)').count();
      const undoBtn = () => page.locator('.draft-bar button.undo');
      const before = await unlocked();
      const one = page.locator('tr.draft-row td button.lockbtn:not(.on)').first();
      await one.click();
      await until(page, (n) => document.querySelectorAll('tr.draft-row td button.lockbtn:not(.on)').length === n - 1, before, 'a name locked');
      ok((await undoBtn().getAttribute('title')).startsWith('Undo: locking 1 name'), 'Undo says what it undoes: ' + await undoBtn().getAttribute('title'));
      await undoBtn().click();
      await until(page, (n) => document.querySelectorAll('tr.draft-row td button.lockbtn:not(.on)').length === n, before, 'Undo unlocks it again');
      await page.locator('tr.draft-row button.lockbtn.group[aria-pressed="false"]').first().click();
      await until(page, (n) => document.querySelectorAll('tr.draft-row td button.lockbtn:not(.on)').length < n, before, 'a row locked');
      await page.locator('h1').click();
      await page.keyboard.press('Control+z');
      await until(page, (n) => document.querySelectorAll('tr.draft-row td button.lockbtn:not(.on)').length === n, before, 'Ctrl+Z unlocks the row again');
      // Remove unlocked, undone: the pinned draft is back
      await page.getByRole('button', { name: 'Remove unlocked' }).click();
      await page.locator('dialog.ask[open]').getByRole('button', { name: 'Remove', exact: true }).click();
      await barHas('Unlocked names removed', 'removed');
      await undoBtn().click();
      await runIs(2, 'Undo brings the draft back');
      ok((await page.locator('.draft-bar button.pin').getAttribute('aria-pressed')) === 'true', 'pinned again');
      // ‹ › and Show changes
      await page.getByRole('button', { name: 'Older draft' }).click();
      await runIs(1, '‹ steps to the older draft');
      ok(await page.getByRole('button', { name: 'Older draft' }).isDisabled(), 'the oldest has no older');
      ok(!(await page.getByRole('button', { name: 'Show changes' }).count()), 'nothing to compare the first draft with');
      await page.getByRole('button', { name: 'Newer draft' }).click();
      await runIs(2, '› steps back');
      await page.getByRole('button', { name: 'Show changes' }).click();
      await until(page, () => document.querySelector('.draft-bar button.changes[aria-pressed="true"]'), null, 'Show changes is on');
      const changed = await page.locator('td.draft.changed').count();
      if (changed) ok((await page.locator('td.draft.changed').first().getAttribute('title')).startsWith('In draft #1:'), 'a changed place says what draft #1 had');
      await page.getByRole('button', { name: 'Show changes' }).click();
      await until(page, () => document.querySelector('.draft-bar button.changes[aria-pressed="false"]') && !document.querySelector('td.draft.changed'), null, 'and off');
    }

    // --- «What matters most»: from the draft's bar, Less / Normal / More per priority; the numbers under Advanced; a line in ⚙ ---
    {
      const seg = (n) => dialog.locator('.prio').nth(n).locator('.seg button[aria-pressed="true"]');
      await page.locator('.draft-bar .prio-link').click();
      ok((await seg(0).innerText()) === 'Normal' && (await seg(1).innerText()) === 'Normal' && (await seg(2).innerText()) === 'Normal', 'the defaults are Normal');
      ok((await dialog.locator('ul.rules li').count()) === 5, 'the rules that always hold are listed');
      ok(!(await dialog.locator('input[type=number]').first().isVisible()), 'the numbers are folded away');
      await reopens(() => dialog.locator('.prio').nth(0).getByRole('button', { name: 'More' }).click(), 'the dialog opens again');
      ok((await seg(0).innerText()) === 'More', 'Even distribution: More');
      await dialog.locator('summary').click();
      ok((await dialog.locator('input[aria-label="' + await page.evaluate(() => MESSAGES.en['weight.w_fair.label']) + '"]').inputValue()) === '25', 'the number behind it');
      await closeDialog();
      await page.getByRole('button', { name: 'Plan settings' }).click();
      ok((await dialog.innerText()).includes('Even: More · Wishes: Normal · Combinations: Normal'), 'Plan settings says what matters: ' + await dialog.innerText());
      await dialog.getByRole('button', { name: 'Change' }).click();
      await reopens(() => dialog.locator('.prio').nth(0).getByRole('button', { name: 'Normal' }).click(), 'the dialog opens again');
      ok((await seg(0).innerText()) === 'Normal', 'and back to Normal');
      await closeDialog();
    }

    // --- a name in a cell of the draft: a click opens a field with suggestions; Enter adds; a name of its own; × removes ---
    {
      const td = () => draft('Sound', ev[1]);
      const ed = page.locator('td.draft.editing');  // (the field takes the place of ⋯, which td() goes by)
      const field = () => ed.locator('input.ename');
      const opts = () => ed.locator('ul.elist li').allInnerTexts();
      const chips = () => ed.locator('.echip').allInnerTexts();
      await page.keyboard.down('Control');
      await draft('Sound', ev[0]).locator('.dlines').click();
      await page.keyboard.up('Control');
      ok((await page.locator('td.draft.editing').count()) === 0 && (await page.locator('td.sel').count()) === 1, 'Ctrl+click selects the cell, no field');
      await page.keyboard.press('Escape');
      await td().locator('.dlines').click();
      await until(page, () => document.activeElement && document.activeElement.matches('td.draft.editing input.ename'), null, 'a click on the cell opens the field, ready to type');
      const all = await opts();
      ok(all.length > 0 && all[all.length - 1].includes('Jon Demo') && all[all.length - 1].includes('said no'), 'who said no comes last: ' + all.join(' | '));
      ok(all.some((x) => x.includes('in the draft: locks it')), 'the names the planner put there say so');
      await field().fill('Ingrid Sample');
      ok((await opts()).join('|') === '+ Add «Ingrid Sample» as a name of its own', 'no person by that name: a name of its own: ' + (await opts()).join('|'));
      await field().press('Enter');
      await until(page, () => [...document.querySelectorAll('td.draft.editing .echip.hand')].some((x) => x.textContent.includes('Ingrid Sample')), null, 'Enter adds it');
      await until(page, () => document.activeElement && document.activeElement.matches('td.draft.editing input.ename') && document.activeElement.value === '', null, 'and the field stays open for the next name');
      await field().fill('pia');
      ok((await opts())[0].includes('Pia Demo'), 'typing suggests the person: ' + (await opts())[0]);
      await field().press('Enter');
      await until(page, () => [...document.querySelectorAll('td.draft.editing .echip.hand')].some((x) => x.textContent.includes('Pia Demo')), null, 'a person is added by Enter');
      // a name the planner put there, picked: it is locked (set by hand)
      const planned = ed.locator('ul.elist li', { hasText: 'in the draft: locks it' }).first();
      const plannedName = await planned.locator('.nm').innerText();
      await planned.click();
      await until(page, (n) => [...document.querySelectorAll('td.draft.editing .echip.locked')].some((x) => x.textContent.includes(n)), plannedName, 'a name from the draft picked is locked');
      await page.keyboard.press('Escape');
      await until(page, () => !document.querySelector('td.draft.editing'), null, 'Escape closes the field');
      const hand = await td().locator('.slot.hand').allInnerTexts();
      ok(hand.includes('Ingrid Sample') && hand.includes('Pia Demo'), 'the names are in the cell, violet: ' + hand.join(', '));
      // + opens it too (more names than places are allowed: no warning), and × takes a name set by hand away
      await td().hover();
      await td().locator('button.addname').click();
      await field().waitFor();
      ok((await chips()).length >= 3, 'the field shows the names there: ' + (await chips()).join(', '));
      ok((await ed.locator(`.echip.locked button.x`).count()) === 0, 'a name of the planner’s has no × (its lock is in the cell)');
      for (const n of ['Ingrid Sample', 'Pia Demo']) {
        await ed.locator(`.echip button.x[aria-label="Remove ${n}"]`).click();
        await until(page, (x) => !!document.querySelector('td.draft.editing') && ![...document.querySelectorAll('td.draft.editing .echip.hand')].some((c) => c.textContent.includes(x)), n, '× removes ' + n);
      }
      await page.keyboard.press('Escape');
      ok((await td().locator('.slot.hand').count()) === 0, 'nothing set by hand is left');
      await td().locator(`button.lockbtn.on[aria-label="Unlock ${plannedName}"]`).click();
      await until(page, ([c, n]) => !!document.querySelector(c)?.closest('td').querySelector(`button.lockbtn:not(.on)[aria-label="Lock ${n}"]`),
        [`button[aria-label="Sound, ${ev[1]}: open the place"]`, plannedName], 'and its lock undoes it');
    }

    // --- «Edit | View»: the plan as it stands, without editing; a name found stands out; a card per event on a phone; print ---
    {
      // an earlier draft picked in Edit does not follow into View: View shows the current plan (#2)
      await page.getByRole('button', { name: 'Plan settings' }).click();
      await dialog.getByRole('button', { name: 'Show draft #1' }).click();
      await runIs(1, 'an earlier draft is on show in Edit');
      await page.click('.title-bar .mode button[data-mode=show]');
      await page.waitForSelector('.show-tools');
      ok((await page.locator('.dp-table').count()) === 0 && (await page.locator('.title-bar .settings').count()) === 0, 'View has no table to edit and no settings');
      ok((await page.innerText('.show-tools .source')).startsWith('From draft #2') && (await page.innerText('.show-tools .source')).includes('Current plan'), 'View shows the current plan: ' + await page.innerText('.show-tools .source'));
      const shown = await page.innerText('.show-table');
      ok(shown.includes('Outside guest ×2') && shown.includes('Café') && /\d+ open/.test(shown), 'View has what is set by hand, the tasks and what is open: ' + shown.slice(0, 300));
      ok((await page.locator('.show-table tbody tr').count()) === (await page.locator('.show-cards .show-card').count()), 'a row and a card per event');
      // «Find a name»: a search field; typing shows the names that match, the arrows and Enter pick one
      await page.fill('input.find', 'ari');
      ok((await page.locator('.find-list li[role=option]').allInnerTexts()).join(',') === 'Kari Test', 'typing shows the names that match: ' + (await page.locator('.find-list').innerText()));
      await page.fill('input.find', 'zzz');
      ok((await page.innerText('.find-list')) === 'No name matches', 'and says when none does');
      await page.fill('input.find', 'kar');
      await page.keyboard.press('Enter');
      await until(page, () => document.querySelector('input.find').value === 'Kari Test' && document.querySelector('.find-list').hidden, null, 'Enter picks the name');
      const hits = await page.locator('.show-table .nm.hit').count();
      ok(hits > 0 && (await page.locator('.show-table .nm.hit').first().innerText()) === 'Kari Test', 'the name found stands out');
      ok((await page.innerText('.show-tools .found')) === (hits === 1 ? 'in 1 event' : `in ${hits} events`), 'how many events: ' + await page.innerText('.show-tools .found'));
      const fades = await page.locator('.show-table tbody tr.miss').count();
      const rows = await page.locator('.show-table tbody tr').count();
      ok(fades > 0 && fades + hits === rows, 'the events without the name fade: ' + fades);
      // only the dates the name has a task on; then the dates across, with the name still found
      await page.getByLabel('Only the dates Kari Test has a task').check();
      ok((await page.locator('.show-table tbody tr').count()) === hits && !(await page.locator('.show-table tbody tr.miss').count()), 'only the dates with a task for the name');
      await page.click('.show-tools .across-btn');
      ok((await page.locator('.show-table.across thead th[data-ev]').count()) === hits && (await page.locator('.show-table.across tbody th[scope=row]').allInnerTexts()).includes('Café'), 'the dates across, the tasks down');
      ok((await page.locator('.show-table.across .nm.hit').count()) === hits, 'the name stands out across too');
      await page.reload();
      await page.waitForSelector('.show-tools');
      ok((await page.locator('.show-table.across .nm.hit').count()) === hits && (await page.inputValue('input.find')) === 'Kari Test', 'View, the name, only and across are kept after a reload');
      await page.getByLabel('Only the dates Kari Test has a task').uncheck();
      ok((await page.locator('.show-table.across thead th[data-ev]').count()) === rows && (await page.locator('.show-table.across thead th.miss').count()) === fades, 'all the dates again, those without the name faded');
      await page.click('.show-tools .across-btn');
      await page.waitForSelector('.show-table:not(.across)');
      // a phone: a card per event instead of the table; printed: the table, without the page around it
      await page.setViewportSize({ width: 390, height: 844 });
      ok(await page.locator('.show-cards').isVisible() && !(await page.locator('.show-scroll').isVisible()), 'a phone gets a card per event');
      ok((await page.locator('.show-cards .nm.hit').count()) === hits, 'the name found stands out on the cards too');
      const pw = await page.evaluate(() => [document.documentElement.scrollWidth, document.documentElement.clientWidth]);
      ok(pw[0] <= pw[1], `View on a phone is ${pw[0]} px wide on a ${pw[1]} px screen`);
      await page.emulateMedia({ media: 'print' });
      ok(await page.locator('.show-scroll').isVisible() && !(await page.locator('.show-cards').isVisible()), 'printed: the table, also from a phone');
      ok(!(await page.locator('header').isVisible()) && !(await page.locator('.title-bar .mode').isVisible()) && !(await page.locator('input.find').isVisible()), 'printed: no header, mode or find');
      await page.emulateMedia({ media: 'screen' });
      await page.setViewportSize({ width: 1280, height: 900 });
      await page.fill('input.find', '');
      await page.locator('input.find').blur();
      ok((await page.locator('.show .nm.hit, .show .miss').count()) === 0 && (await page.innerText('.show-tools .found')) === '', 'no name: nothing stands out');
      await page.click('.title-bar .mode button[data-mode=edit]');
      await page.waitForSelector('.dp-table');
      await page.reload();
      await page.waitForSelector('.dp-table');
      ok((await page.getAttribute('.title-bar .mode button[data-mode=edit]', 'aria-pressed')) === 'true', 'Edit is kept after a reload');
    }

    // --- copy an event, delete one ---
    const newDate = () => dialog.locator('input[aria-label="New date"]').first().inputValue();
    const made = () => dialog.locator('ul.copy-dates li').allInnerTexts();
    // the first date suggested has no event yet: from 2 Aug every other week, 16 and 30 Aug are taken, so 13 Sept
    await page.click(`button[aria-label="Copy the event ${ev[0]}"]`);
    ok((await page.getAttribute(`button[aria-label="Copy the event ${ev[0]}"]`, 'title')) === 'Copy to other dates' && (await dialog.locator('input[aria-label="New date"]').inputValue()) === '2026-08-09', 'a week on is free: ' + await newDate());
    await dialog.locator('select[aria-label="Repeat"]').selectOption({ label: 'Every other week' });
    ok((await newDate()) === '2026-09-13', 'the suggestion skips the dates that have an event: ' + await newDate());
    await closeDialog();
    await page.click(`button[aria-label="Copy the event ${ev[2]}"]`);
    ok((await newDate()) === '2026-09-06', 'the copy goes a week later by default');
    await dialog.locator('select[aria-label="Repeat"]').selectOption({ label: 'Every 3 weeks' });
    ok((await newDate()) === '2026-09-20', 'the suggested date follows the rhythm: ' + (await newDate()));
    await dialog.locator('select[aria-label="Repeat"]').selectOption({ label: 'Does not repeat' });
    ok((await newDate()) === '2026-09-06', 'and goes back without a repeat');
    ok((await dialog.innerText()).includes('11:00–12:30') && (await dialog.innerText()).includes('Sound 1') && (await dialog.innerText()).includes('People needed'), 'the dialog shows what comes along');
    ok((await made()).join('|') === 'Sun 6 Sept', 'the dialog lists the new dates: ' + (await made()).join('|'));
    // a date that has an event is marked; the button says how many
    await dialog.getByRole('button', { name: '+ another date' }).click();
    await dialog.locator('input[aria-label="New date"]').nth(1).fill('2026-08-16');
    ok((await made()).join('|') === 'Sun 16 Aug · already has an event|Sun 6 Sept', 'a date with an event is marked: ' + (await made()).join('|'));
    ok((await dialog.locator('ul.copy-dates li.taken').count()) === 1 && (await dialog.locator('button[type=submit]').innerText()) === 'Make 2 copies', 'the button says how many');
    await dialog.locator('input[aria-label="New date"]').nth(1).fill('');
    await dialog.getByRole('button', { name: 'Make 1 copy', exact: true }).click();
    await msgHas(page, '1 copy made.');
    ok((await cols()) === 4, 'the copy is a new column');
    const ev3 = 'Sun 6 Sept 11:00';
    ok((await places('Sound', ev3).inputValue()) === '1' && (await value('Theme', ev3).inputValue()) === 'Finals', 'the places and the details come along');
    ok((await answer('Kari Test', ev3).getAttribute('data-level')) === 'none', 'answers are never copied');
    await page.click(`button[aria-label="Open the event ${ev3}"]`);
    // asked first, in the page's own dialog over the event's: Cancel keeps the event, Delete deletes it
    await dialog.getByRole('button', { name: 'Delete the event' }).click();
    const sure = page.locator('dialog.ask[open]');
    ok((await sure.locator('h2').innerText()) === 'Delete the event' && (await sure.innerText()).includes(ev3.split(' ')[0]), 'the event dialog asks first: ' + await sure.innerText());
    await sure.getByRole('button', { name: 'Cancel' }).click();
    await sure.waitFor({ state: 'detached' });
    ok((await page.locator('dialog.modal[open]').count()) === 1 && (await page.locator('.dp-table thead th[scope=col]:not(.addcol)').count()) === 4, 'Cancel keeps the event and its dialog');
    await dialog.getByRole('button', { name: 'Delete the event' }).click();
    await sure.getByRole('button', { name: 'Delete', exact: true }).click();
    await until(page, () => document.querySelectorAll('.dp-table thead th[scope=col]:not(.addcol)').length === 3, null, 'the event is deleted');

    // --- the organisation: a plan to delete, rename ---
    await page.goto(base + '/o/sports-club');
    await h1is(page, 'Sports Club');
    await addNamed('New plan, e.g. Autumn 2026', 'Spring 2027');
    await page.waitForURL('**/p/spring-2027');
    await page.getByRole('button', { name: 'Plan settings' }).click();
    await dialog.locator('input[aria-label="Name of the plan"]').fill('Spring 2027 (draft)');
    await dialog.getByRole('button', { name: 'Rename' }).click();
    await h1is(page, 'Spring 2027 (draft)');
    ok(page.url().endsWith('/p/spring-2027'), 'a rename keeps the address');
    await page.getByRole('button', { name: 'Plan settings' }).click();
    await dialog.getByRole('button', { name: 'Delete the plan' }).click();
    await page.locator('dialog.ask[open]').getByRole('button', { name: 'Delete', exact: true }).click();
    await page.waitForURL('**/o/sports-club');
    await msgHas(page, 'The plan “Spring 2027 (draft)” was deleted.');
    await page.click('header a[aria-label="Settings"]');
    await h1is(page, 'Settings');
    ok((await page.innerText('main')).includes('cannot be deleted'), 'an organisation with plans cannot be deleted');
    ok(!(await page.locator('header a[aria-label="Members"]').count()), 'no Members without a login');
    {
      const sizes = await page.evaluate(() => [...document.querySelectorAll('header .icon-btn, header .dropdown > summary')].map((e) => Math.round(e.getBoundingClientRect().height)));
      ok(sizes.length >= 2 && sizes.every((x) => x === sizes[0]), 'the header buttons are the same height: ' + sizes);
    }

    // --- selecting like a spreadsheet: drag, the bar, Ctrl+click, a whole row, a whole day, the keys; then put it all back ---
    await page.goto(base + planUrl);
    await page.waitForSelector('.dp-table');
    // --- alignment: one height for the title bar's buttons, one for those in the table; a date that snaps into place is clear of the names ---
    const clearOfNames = () => page.evaluate(() => {
      const box = document.querySelector('.grid-scroll.main');
      return [parseFloat(getComputedStyle(box).scrollPaddingLeft), box.querySelector('th.corner').getBoundingClientRect().width,
        ...[...box.querySelectorAll('tbody tr:not([hidden]) > th[scope=row]')].map((th) => th.getBoundingClientRect().width)].map(Math.round);
    });
    {
      const a = await page.evaluate(() => {
        const hs = (sel) => [...document.querySelectorAll(sel)].filter((e) => e.getClientRects().length).map((e) => Math.round(e.getBoundingClientRect().height));  // (what is shown: the add lines are behind a +)
        return { title: hs('.title-bar > button:not(.info), .title-bar .sideways button'),
          table: hs('.dp-table th.addcol button, .draft-bar .row-inline > button, .draft-bar select, .dp-table thead button.edit'),
          slim: hs('.dp-table tr.adds form.addrow button, .dp-table tr.field-row button.freeze') };  // the add lines and the details rows are slimmer
      });
      ok(a.slim.length >= 2 && a.slim.every((x) => x === a.slim[0]) && a.slim[0] < a.table[0], 'the add lines and details rows have one smaller height: ' + a.slim);
      ok(a.title.length >= 4 && a.title.every((x) => x === a.title[0]), 'the title bar’s buttons are one height: ' + a.title);
      ok(a.table.length >= 4 && a.table.every((x) => x === a.table[0]), 'the buttons in the table are one height: ' + a.table);
      const [pad, ...names] = await clearOfNames();
      ok(names.every((w) => w === names[0] && w <= pad), `the names have one width (${names}) and the dates snap clear of them (${pad})`);
    }
    {
      const who = ['Kari Test', 'Ola Sample'];
      const levelOf = (person, e) => page.evaluate((x) => document.querySelector(`button.ans[aria-label^="${x}:"]`).dataset.level, person + ', ' + e);
      const before = {};
      for (const p of who) for (const e of ev) before[p + '|' + e] = await levelOf(p, e);
      const centre = async (loc) => { const b = await loc.boundingBox(); return [b.x + b.width / 2, b.y + b.height / 2]; };
      await answer(who[0], ev[0]).evaluate((b) => b.scrollIntoView({ block: 'center' }));  // in the middle: near the edge a drag scrolls the window
      const [x0, y0] = await centre(answer(who[0], ev[0]));
      const [x1, y1] = await centre(answer(who[1], ev[1]));
      await page.mouse.move(x0, y0);
      await page.mouse.down();
      await page.mouse.move((x0 + x1) / 2, (y0 + y1) / 2, { steps: 4 });
      await page.mouse.move(x1, y1, { steps: 4 });
      await page.mouse.up();
      ok((await page.locator('td.sel').count()) === 4 && (await page.innerText('.selbar .n')) === '4 cells', 'dragging selects a block of answers');
      ok((await levelOf(who[0], ev[0])) === before[who[0] + '|' + ev[0]], 'the drag did not click the first answer');
      await page.click('.selbar button.ans[data-level=good]');
      await until(page, () => document.querySelectorAll('td.sel button.ans[data-level=good]').length === 4, null, 'the bar sets all four to yes');
      await page.keyboard.press('2');
      await until(page, () => document.querySelectorAll('td.sel button.ans[data-level=poor]').length === 4, null, 'the key 2 sets them to maybe');
      await page.keyboard.down('Control');
      await answer(who[0], ev[0]).click();
      await page.keyboard.up('Control');
      ok((await page.locator('td.sel').count()) === 3, 'Ctrl+click takes one cell out of the selection');
      await page.keyboard.press('Escape');
      ok((await page.locator('td.sel').count()) === 0 && (await page.locator('.selbar').isHidden()), 'Escape lets go');
      await page.locator('tr.person-row', { has: answer(who[1], ev[0]) }).locator('.grip').click();
      ok((await page.locator('td.sel').count()) === 0, '⋮⋮ only drags: a click selects nothing');
      await page.click(`button[aria-label="Select the row ${who[1]}"]`);
      ok((await page.locator('td.sel').count()) === ev.length, '⇥ selects the whole row');
      await page.keyboard.press('Escape');
      await page.locator('td.count').nth(2).click();
      ok((await page.locator('td.sel').count()) === (await page.locator('tr.person-row').count()), 'a click on a count selects everyone that day');
      await page.keyboard.press('Escape');
      // put the answers back the way they were, one click at a time
      const next = { none: 'good', good: 'poor', poor: 'never', never: 'none' };
      for (const p of who) for (const e of ev) {
        for (let i = 0; i < 4 && (await levelOf(p, e)) !== before[p + '|' + e]; i++) {
          const was = await levelOf(p, e);
          await answer(p, e).click();
          await until(page, (x) => document.querySelector(`button.ans[aria-label^="${x[0]}:"]`).dataset.level === x[1], [p + ', ' + e, next[was]], 'one click back');
        }
      }
      // places: a row of a task, set at once, then cleared with Delete, then put back
      const placesWas = [];
      for (const e of ev) placesWas.push(await places('Sound', e).inputValue());
      await page.click('button[aria-label="Select the row Sound"]');
      ok((await page.locator('td.sel input[type=number]').count()) === ev.length, '⇥ selects a task row');
      await page.fill('.selbar input[type=number]', '3');
      await page.click('.selbar button:has-text("Set")');
      await until(page, () => [...document.querySelectorAll('td.sel input[type=number]')].every((i) => i.value === '3'), null, 'Set gives every selected place 3');
      await page.keyboard.press('Delete');
      await until(page, () => [...document.querySelectorAll('td.sel input[type=number]')].every((i) => i.value === ''), null, 'Delete clears them');
      await page.keyboard.press('Escape');
      for (let i = 0; i < ev.length; i++) if (placesWas[i]) { await places('Sound', ev[i]).fill(placesWas[i]); await places('Sound', ev[i]).press('Tab'); }
      await page.waitForTimeout(300);
    }

    // --- folding: a section folds to its heading, and stays folded after a reload ---
    await page.goto(base + planUrl);
    await page.waitForSelector('.dp-table');
    const taskRows = () => page.evaluate(() => [...document.querySelectorAll('.dp-table tr[data-fold~=tasks]')].filter((r) => !r.hidden).length);
    ok((await taskRows()) > 0, 'the tasks are shown');
    await page.click('button.fold[data-key=tasks]');
    ok((await taskRows()) === 0 && (await page.getAttribute('button.fold[data-key=tasks]', 'aria-expanded')) === 'false', 'the tasks fold');
    await page.reload();
    await page.waitForSelector('.dp-table');
    ok((await taskRows()) === 0, 'and stay folded after a reload');
    await page.click('button.fold[data-key=tasks]');
    ok((await taskRows()) > 0, 'and unfold');

    // --- a note on an answer: the corner tab opens it, hovering shows it, empty removes it; the answer stays as it was ---
    {
      const cellOf = (person, e) => page.locator('td.answer', { has: answer(person, e) });
      const levelBefore = await answer('Kari Test', ev[1]).getAttribute('data-level');
      await cellOf('Kari Test', ev[1]).hover();
      await cellOf('Kari Test', ev[1]).locator('.note-tab').click();
      await page.fill('.note-pop textarea', 'Can come after 12');
      await page.click('.note-pop button:has-text("Save")');
      await until(page, () => document.querySelectorAll('td.answer.has-note').length === 1, null, 'saving a note marks the cell');
      ok((await answer('Kari Test', ev[1]).getAttribute('data-level')) === levelBefore, 'the tab does not change the answer');
      await page.reload();
      await page.waitForSelector('.dp-table');
      await page.mouse.move(1, 1);
      await answer('Kari Test', ev[1]).hover();
      await until(page, () => { const p = document.querySelector('.note-pop'); return p && !p.hidden && p.innerText.includes('Can come after 12'); }, null, 'hovering the cell shows the note after a reload');
      await page.mouse.move(1, 1);
      ok(await page.locator('.note-pop').isHidden(), 'and it goes when the mouse leaves');
      await cellOf('Kari Test', ev[1]).hover();
      await cellOf('Kari Test', ev[1]).locator('.note-tab').click();
      await page.click('.note-pop button:has-text("Delete")');
      await until(page, () => !document.querySelector('td.answer.has-note'), null, 'Delete removes the note');
    }

    // --- who is shown: a ✓ per person in a dialog, kept after a reload; the counts still have everyone ---
    {
      const rowOf = (name) => page.locator('tr.person-row', { has: answer(name, ev[0]) });
      const countBefore = await page.locator('td.count').first().getAttribute('aria-label');
      await page.click('button.who');
      await dialog.locator('label.who-item:has-text("Ola Sample") input').uncheck();
      ok(await rowOf('Ola Sample').isHidden(), 'unticking a person hides their row');
      ok((await page.locator('td.count').first().getAttribute('aria-label')) === countBefore, 'the counts still have everyone');
      await closeDialog();
      ok((await page.innerText('button.who')).includes('of'), 'the button says how many are shown: ' + (await page.innerText('button.who')));
      await page.reload();
      await page.waitForSelector('.dp-table');
      ok(await rowOf('Ola Sample').isHidden(), 'and stays hidden after a reload');
      await page.click('button.who');
      await dialog.locator('.who-head button', { hasText: 'All' }).click();
      ok(await rowOf('Ola Sample').isVisible(), 'All shows everyone again');
      // by task: only those who take Video (Kari Test if needed, Jon Demo good; Ola Sample only takes Sound)
      await dialog.locator('label.who-task:has-text("Video") input').check();
      ok(await rowOf('Ola Sample').isHidden() && await rowOf('Kari Test').isVisible() && await rowOf('Jon Demo').isVisible(), 'a task ticked shows only those who take it');
      await closeDialog();
      ok((await page.innerText('button.who')).startsWith('Video · '), 'the button says which task: ' + (await page.innerText('button.who')));
      await page.reload();
      await page.waitForSelector('.dp-table');
      ok(await rowOf('Ola Sample').isHidden() && await rowOf('Kari Test').isVisible(), 'and the task stays ticked after a reload');
      await page.click('button.who');
      await dialog.locator('label.who-task:has-text("Video") input').uncheck();
      ok(await rowOf('Ola Sample').isVisible(), 'no task ticked: everyone again');
      await closeDialog();
      ok((await page.innerText('button.who')) === 'Everyone', 'the button says everyone');
    }

    // --- «Show tasks»: the tasks a person is willing to take, as tags under the name; off by default, kept after a reload ---
    {
      const tagsOf = (name) => page.locator('tr.person-row', { has: answer(name, ev[0]) }).locator('.tags .tag');
      ok((await page.getAttribute('button.tags-btn', 'aria-pressed')) === 'false' && !(await tagsOf('Kari Test').first().isVisible()), 'the tags are hidden at first');
      await page.click('button.tags-btn');
      const kari = await tagsOf('Kari Test').evaluateAll((xs) => xs.map((x) => x.className + ':' + x.textContent).join(','));
      ok(kari === 'tag good:Sound,tag poor:Video', 'Kari Test: Sound green, Video yellow: ' + kari);
      ok(await tagsOf('Kari Test').first().isVisible() && (await tagsOf('Ola Sample').count()) > 0, 'Show tasks shows them for everyone');
      await page.reload();
      await page.waitForSelector('.dp-table');
      ok((await page.getAttribute('button.tags-btn', 'aria-pressed')) === 'true' && await tagsOf('Kari Test').first().isVisible(), 'and keeps them after a reload');
      await page.click('button.tags-btn');
      ok(!(await tagsOf('Kari Test').first().isVisible()), 'a second click hides them');
    }

    // --- a phone: no page is wider than the screen; the table scrolls in its own box with the first column in place ---
    await page.setViewportSize({ width: 390, height: 844 });
    for (const url of ['/o/sports-club', planUrl]) {
      await page.goto(base + url);
      await page.waitForSelector('h1');
      await page.waitForTimeout(150);
      const w = await page.evaluate(() => [document.documentElement.scrollWidth, document.documentElement.clientWidth]);
      ok(w[0] <= w[1], `the page ${url} is ${w[0]} px wide on a ${w[1]} px screen (scrolls sideways)`);
    }
    // the same table as on a wide screen: dates across, people down; the names stay put while the days scroll sideways
    ok((await page.locator('button.ans').count()) === 12, 'on a phone the answers are in the ordinary table (4 people × 3 events)');
    const stuck = await page.evaluate(() => {
      const box = document.querySelector('.grid-scroll.main');
      const th = box.querySelector('tbody th[scope=row]');
      const x0 = th.getBoundingClientRect().left;
      box.scrollLeft = 300;
      return [box.scrollLeft > 0, Math.abs(th.getBoundingClientRect().left - x0) < 1];
    });
    ok(stuck[0] && stuck[1], 'the table scrolls sideways and the row names stay put: ' + stuck);
    {
      const [pad, ...names] = await clearOfNames();
      ok(names.every((w) => w === names[0] && w <= pad), `on a phone too the names have one width (${names}) and the dates snap clear of them (${pad})`);
    }
    const countOf = (e) => page.locator('td.count').nth(ev.indexOf(e)).getAttribute('aria-label');
    const before0 = await answer('Jon Demo', ev[0]).getAttribute('data-level');
    const count0 = await countOf(ev[0]);
    await answer('Jon Demo', ev[0]).click();
    const next0 = { none: 'good', good: 'poor', poor: 'never', never: 'none' }[before0];
    await until(page, ([x, l]) => document.querySelector(`button.ans[aria-label^="${x}:"]`).dataset.level === l, ['Jon Demo, ' + ev[0], next0], 'a tap moves the answer on');
    ok((await countOf(ev[0])) !== count0, 'the count of the event follows: ' + count0 + ' -> ' + (await countOf(ev[0])));
    // the cell into view first: scrolled sideways, it can be under the names, which stay put
    await page.locator('td.answer', { has: answer('Jon Demo', ev[1]) }).evaluate((td) => td.scrollIntoView({ inline: 'center', block: 'center' }));
    await page.locator('td.answer', { has: answer('Jon Demo', ev[1]) }).locator('.note-tab').click({ force: true });
    await until(page, () => { const x = document.querySelector('.note-pop textarea'); return !!x && x.offsetParent !== null; }, null, 'on a phone the corner tab opens the note too');
    const pop = await page.locator('.note-pop').boundingBox();
    ok(pop.x >= 0 && pop.x + pop.width <= 390, 'and the note fits on the screen: ' + JSON.stringify(pop));
    await page.click('.note-pop button:has-text("Cancel")');
    await page.click('button[aria-label="Open Kari Test"]');
    ok((await dialog.locator('h2').innerText()) === 'Kari Test', 'a person’s name at the top opens the person');
    await closeDialog();
    await page.setViewportSize({ width: 1280, height: 900 });

    // --- Norwegian: the switch, what the server writes, the errors ---
    await page.goto(base + '/o/sports-club');
    await h1is(page, 'Sports Club');
    await page.click('header .dropdown.lang summary');
    await page.click('header .drop-item[lang=nb]');
    await page.waitForSelector('h2:has-text("Planer")');
    ok((await page.getAttribute('html', 'lang')) === 'nb', 'the page language becomes Norwegian');
    ok((await page.textContent('header .brand')) === 'Tjenesteplan', 'the brand is Tjenesteplan in Norwegian');
    await page.reload();
    await page.waitForSelector('h2:has-text("Planer")');  // and survives a reload
    await page.goto(base + planUrl);
    await h1is(page, 'Autumn 2026');
    await until(page, () => ((document.querySelector('.draft-bar .run-id') || {}).title || '').startsWith('Utkast #2'), null, 'the draft texts are Norwegian');
    const tbl = await page.innerText('.dp-table');
    ok(tbl.includes('OM HENDELSEN') && tbl.includes('UTKAST TIL FORDELING') && tbl.includes('TILGJENGELIGHET') && tbl.includes('Ikke fylt'), 'the table is Norwegian: ' + tbl.slice(0, 200));
    await page.locator('button[aria-label^="Café, søn. 16. aug."]').click();
    ok((await dialog.innerText()).includes('Ikke fylt:') && (await dialog.innerText()).includes('har ikke valgt oppgaven'), 'the server explains in Norwegian too: ' + (await dialog.innerText()));
    await closeDialog();
    ok((await page.locator('button.ans[aria-label^="Kari Test, søn. 2. aug. 11:00:"]').getAttribute('aria-label')).endsWith(': kanskje'), 'the answers are Norwegian');
    await page.click('.title-bar .mode button[data-mode=show]');
    await page.waitForSelector('.show-tools');
    ok((await page.innerText('.title-bar .mode')).replace(/\s+/g, ' ').trim() === 'Rediger Vis' && (await page.getAttribute('input.find', 'placeholder')) === 'Finn et navn'
      && (await page.innerText('.show-table thead')).includes('Dato') && (await page.innerText('.show-tools .source')).startsWith('Fra utkast #2'), 'View is Norwegian');
    await page.click('.title-bar .mode button[data-mode=edit]');
    await page.waitForSelector('.dp-table');
    before = errors.length;
    await addNamed('Ny oppgave', 'Sound');
    await msgHas(page, 'Oppgave «Sound» finnes allerede');  // an error code from the server, written in Norwegian
    errors.length = before;  // 409 is the right answer here

    // a visitor whose browser is Norwegian gets Norwegian without choosing, and can still switch to English
    const nbContext = await browser.newContext({ locale: 'nb-NO' });
    const nbPage = await nbContext.newPage();
    watch(nbPage);
    await nbPage.goto(base + '/?switch');
    await h1is(nbPage, 'Velg organisasjon');
    ok((await nbPage.getAttribute('html', 'lang')) === 'nb', 'a Norwegian browser gets a Norwegian page');
    ok((await nbPage.innerText('header .dropdown.lang summary')).trim() === 'NB', 'the language button says NB');
    await nbPage.click('header .dropdown.lang summary');
    await nbPage.click('header .drop-item[lang=en]');
    await h1is(nbPage, 'Choose organisation');
    await nbContext.close();

    // With a login (the server says so in Me; the sign-in itself is tested in Go): someone not signed in gets the start
    // page, and «Sign in» goes to the sign-in and back to the same page; someone signed in sees their name and «Sign out».
    {
      const c = await browser.newContext({ locale: 'en-GB' });
      const pg = await c.newPage();
      watch(pg);
      let who = null;
      await pg.route('**/dutyplan.v1.AccountService/Me', (route) => pb.answer(route, { login: true, user: who, signup: 'https://id.example.invalid/signup' }));
      let signInAt = '';
      await pg.route('**/auth/login**', (route) => { signInAt = route.request().url(); return route.fulfill({ body: 'sign-in', contentType: 'text/plain' }); });
      await pg.goto(base + '/o/sample-club?x=1');
      await h1is(pg, 'Plan who does what, fairly');
      ok(await pg.locator('.landing img.logo').count() === 1 && await pg.locator('.landing ol.steps li').count() === 4 && await pg.locator('.landing .points li').count() === 4, 'the start page explains, in steps');
      // the film: in the page's language, the plan as its poster, and a file the browser can read (where it plays H.264)
      const film = () => pg.evaluate(() => { const v = document.querySelector('.landing video.demo'); return v && { src: v.getAttribute('src'), poster: v.getAttribute('poster'), muted: v.muted, h264: v.canPlayType('video/mp4; codecs="avc1.64001f"') }; });
      const f = await film();
      ok(f && f.src === '/demo-en.mp4' && f.poster === '/landing.png' && f.muted, 'the start page has the film: ' + JSON.stringify(f));
      if (f.h264) await until(pg, () => document.querySelector('.landing video.demo').readyState >= 1, null, 'the film loads');
      // About: why it exists, free, and how to reach the maker: e-mail and GitHub issues
      const aboutOf = () => pg.evaluate(() => { const a = document.querySelector('.landing footer.about'); return a && { h: a.querySelector('h2').textContent,
        mail: [...a.querySelectorAll('a[href^="mailto:"]')].map((x) => x.getAttribute('href')), issues: !!a.querySelector('a[href="https://github.com/gunnaringe/dutyplan/issues"]'),
        free: (document.querySelector('.landing .hero .free') || {}).textContent }; });
      const ab = await aboutOf();
      ok(ab && ab.h === 'About Dutyplan' && ab.mail.join() === 'mailto:contact@sermon-studio.app' && ab.issues && ab.free === 'Free, with no ads and no tracking.', 'the start page says who makes it and how to reach them: ' + JSON.stringify(ab));
      // the logo stays small, and nothing is wider than the screen, on a computer and on a phone
      for (const width of [1280, 390]) {
        await pg.setViewportSize({ width, height: 800 });
        await pg.waitForTimeout(100);
        const m = await pg.evaluate(() => { const r = document.querySelector('.landing img.logo').getBoundingClientRect(); return [r.width, r.height, document.documentElement.scrollWidth, document.documentElement.clientWidth]; });
        ok(m[0] <= 80 && m[1] <= 80, `the logo on the start page is ${m[0]}×${m[1]} at ${width} px`);
        ok(m[2] <= m[3], `the start page is ${m[2]} px wide on a ${m[3]} px screen`);
      }
      await pg.setViewportSize({ width: 1280, height: 720 });
      await pg.evaluate(() => localStorage.setItem('tp.lang', 'nb'));
      await pg.reload();
      await pg.waitForSelector('.landing video.demo');
      ok((await film()).src === '/demo-nb.mp4', 'the film in Norwegian on the Norwegian page');
      ok((await aboutOf()).h === 'Om Tjenesteplan', 'the About section in Norwegian');
      ok(await pg.evaluate(async () => { await document.fonts.ready; return document.fonts.check('1.7rem "signature"', 'Gunnar'); }), 'the signature font loads');
      await pg.evaluate(() => localStorage.setItem('tp.lang', 'en'));
      await pg.reload();
      await h1is(pg, 'Plan who does what, fairly');
      ok((await pg.getAttribute('.landing a.button', 'href')) === 'https://id.example.invalid/signup', 'the start page offers to create an account');
      await pg.click('.landing button:has-text("Sign in")');
      await pg.waitForURL('**/auth/login**');
      ok(new URL(signInAt).searchParams.get('next') === '/o/sample-club?x=1', 'back to the same page after signing in: ' + signInAt);
      who = { name: 'Kari Test', email: 'kari@example.org' };
      let signedOut = false;
      await pg.route('**/auth/logout', (route) => { signedOut = route.request().method() === 'POST'; return route.fulfill({ json: { to: '/?bye' } }); });
      await pg.goto(base + '/?switch');
      await h1is(pg, 'Choose organisation');
      ok((await pg.innerText('header .avatar')) === 'KT' && (await pg.getAttribute('header .dropdown.user summary', 'aria-label')) === 'Kari Test', 'the header shows who is signed in, as initials');
      ok(!(await pg.innerText('main')).includes('There is no login'), 'no «no login» warning with a login');
      await pg.click('header .dropdown.user summary');
      await pg.click('header button.signout');
      await pg.waitForURL('**/?bye');
      ok(signedOut, 'signing out posts to /auth/logout and goes where it says');
      await c.close();
    }

    // Sharing (with a login; this server has none, so Me says there is one, and everyone is admin): links to a plan,
    // the members with the plans' links and an invitation, and what a view link or an edit link to a plan shows.
    {
      const c = await browser.newContext({ locale: 'en-GB', permissions: ['clipboard-read', 'clipboard-write'] });
      const pg = await c.newPage();
      watch(pg);
      const dlg = pg.locator('dialog.modal:not(.ask)');
      await pg.route('**/dutyplan.v1.AccountService/Me', (route) => pb.answer(route, { login: true, user: { name: 'Kari Test' }, links: [] }));
      await pg.goto(base + '/o/sports-club');
      ok(!(await pg.locator('button:has-text("Share the organisation")').count()), 'no link to the whole organisation');
      await pg.goto(base + '/o/sports-club/p/autumn-2026');
      await pg.click('button.share');
      await until(pg, () => (document.querySelector('dialog[open] h2') || {}).textContent === 'Share «Autumn 2026»', null, 'the share dialog opens');
      await dlg.locator('input[aria-label="Label"]').fill('For the volunteers');
      ok((await dlg.locator('select[aria-label="What the link allows"] option').allInnerTexts()).join(',') === 'Can view,Can answer,Can edit', 'a link can view, answer or edit');
      await dlg.locator('select[aria-label="What the link allows"]').selectOption('edit');
      await dlg.locator('button:has-text("Make link")').click();
      await until(pg, () => [...document.querySelectorAll('dialog[open] .link-row')].some((li) => li.textContent.includes('For the volunteers')), null, 'the new link is listed');
      const copied = await pg.evaluate(() => navigator.clipboard.readText());
      ok(/^http:\/\/127\.0\.0\.1:\d+\/s\/[\w-]{20,}$/.test(copied), 'the new link is copied: ' + copied);
      ok((await dlg.locator('.link-row .lvl').first().innerText()) === 'Can edit', 'the link says what it allows');
      // no clipboard (an old browser, or not https): the link, selected, in the page's own dialog
      await pg.evaluate(() => { navigator.clipboard.writeText = () => Promise.reject(new Error('no clipboard')); });
      await dlg.first().locator('.link-row button[aria-label="Copy"]').click();
      const byHand = pg.locator('dialog.copy-by-hand[open]');
      await byHand.waitFor();
      ok((await byHand.locator('input').inputValue()) === copied && await byHand.locator('input').evaluate((i) => document.activeElement === i && i.selectionEnd - i.selectionStart === i.value.length), 'without a clipboard the link is shown selected');
      await pg.keyboard.press('Escape');
      await byHand.waitFor({ state: 'detached' });
      await dlg.locator('button[aria-label="Delete the link For the volunteers"]').click();
      await pg.click('dialog.ask button.yes');  // asked first
      await until(pg, () => !document.querySelector('dialog[open] .link-row'), null, 'the deleted link is gone');
      await dlg.locator('button:has-text("Make link")').click();
      await until(pg, () => document.querySelectorAll('dialog[open] .link-row').length === 1, null, 'a view link to the plan');
      await pg.keyboard.press('Escape');
      // the members: the plans' links, and an invitation
      await pg.goto(base + '/o/sports-club');
      await pg.click('header a[aria-label="Members"]');
      await pg.waitForURL('**/o/sports-club/members');
      await until(pg, () => [...document.querySelectorAll('main .link-row')].some((li) => li.textContent.includes('Autumn 2026')), null, 'the plan link is listed on the members page');
      // an invitation is for a person: whom it is for, as its name in the list
      ok((await pg.inputValue('main form.new-link select[aria-label="Who can use it"]')) === 'once', 'an invitation is for one person unless chosen otherwise');
      await pg.fill('main form.new-link input[type=text]', 'the new treasurer');
      ok((await pg.innerText('main form.new-link')).includes('Who are you inviting?'), 'the invitation asks whom it is for');
      await pg.click('main form.new-link button[type=submit]:text-is("Invite")');
      await until(pg, () => [...document.querySelectorAll('main .link-row')].some((li) => li.querySelector('.title').textContent === 'Invitation for the new treasurer' && li.querySelector('.lvl').textContent === 'Becomes a member' && li.querySelector('.uses').textContent === 'one person'), null, 'the invitation is listed with whom it is for, for one person by default');
      ok(await pg.evaluate(() => navigator.clipboard.readText()).then((x) => /\/s\/[\w-]{20,}$/.test(x)), 'the new invitation is copied');
      // signed in on a phone: the header (members, settings, language, initials) and the pages stay inside the screen
      await pg.setViewportSize({ width: 390, height: 844 });
      for (const url of ['/o/sports-club', '/o/sports-club/members', '/o/sports-club/p/autumn-2026']) {
        await pg.goto(base + url);
        await pg.waitForSelector('h1');
        await pg.waitForTimeout(150);
        const w = await pg.evaluate(() => [document.documentElement.scrollWidth, document.documentElement.clientWidth]);
        ok(w[0] <= w[1], `signed in, the page ${url} is ${w[0]} px wide on a ${w[1]} px screen`);
      }
      await pg.setViewportSize({ width: 1280, height: 720 });
      // someone with a view link to the plan: a note, and nothing to share
      const asLink = (extra) => (route) => pb.edit(route, (o) => ({ ...o, role: '', ...extra }));
      await pg.route('**/dutyplan.v1.OrgService/GetOrg', asLink({ level: 'view', plan: 'autumn-2026' }));
      await pg.goto(base + '/o/sports-club/p/autumn-2026');
      await pg.waitForSelector('.show-tools');  // one who may only view starts in View; «Table» is the table, with a note
      ok((await pg.locator('.title-bar .mode button[data-mode=edit]').innerText()) === 'Table', 'a view link has View and Table');
      await pg.click('.title-bar .mode button[data-mode=edit]');
      await pg.waitForSelector('.view-only');
      ok(!(await pg.locator('button.share').count()), 'a view link shows no Share');
      await pg.unroute('**/dutyplan.v1.OrgService/GetOrg');
      // an answer link: «My answers» first; who are you, then every date with yes / maybe / no and a note; kept after a reload
      await pg.route('**/dutyplan.v1.OrgService/GetOrg', asLink({ level: 'answer', plan: 'autumn-2026' }));
      await pg.goto(base + '/o/sports-club/p/autumn-2026');
      await pg.waitForSelector('.me-page');
      ok((await pg.innerText('.me-page h2')) === 'Who are you?' && (await pg.innerText('.title-bar .mode')).replace(/\s+/g, ' ').trim() === 'My answers View', 'an answer link asks who you are, with My answers and View');
      ok(!(await pg.locator('.dp-table, button.share, .title-bar .settings').count()), 'no table, Share or settings for an answer link');
      await pg.click('button.me-pick:has-text("Pia Demo")');
      await until(pg, () => (document.querySelector('.me-head h2') || {}).textContent === 'Pia Demo', null, 'the name chosen is shown');
      const first = pg.locator('.me-ev').first();
      const n = await pg.locator('.me-ev').count();
      ok(n > 1, 'a line per date: ' + n);
      await first.locator('button.me-ans.never').click();
      await until(pg, () => !document.querySelector('#msg.err'), null, 'the answer was refused');
      await first.locator('input.me-note').fill('Away that weekend');
      await first.locator('input.me-note').press('Tab');
      await msgHas(pg, 'Note saved');
      await pg.reload();
      await pg.waitForSelector('.me-list');
      ok((await pg.innerText('.me-head h2')) === 'Pia Demo', 'the name chosen is kept');
      ok((await first.locator('button.me-ans.never').getAttribute('aria-pressed')) === 'true' && (await first.locator('input.me-note').inputValue()) === 'Away that weekend', 'the answer and the note are kept');
      await first.locator('button.me-ans.never').click();  // the same again: not answered
      await until(pg, () => document.querySelector('.me-ev button.me-ans.never').getAttribute('aria-pressed') === 'false', null, 'a second click takes the answer back');
      await first.locator('input.me-note').fill('');
      await first.locator('input.me-note').press('Tab');
      await msgHas(pg, 'Note saved');
      // View has the plan, with the name chosen found
      await pg.click('.title-bar .mode button[data-mode=show]');
      await pg.waitForSelector('.show-tools');
      ok((await pg.inputValue('input.find')) === 'Pia Demo', 'View finds the name chosen');
      await pg.click('.title-bar .mode button[data-mode=me]');
      await pg.waitForSelector('.me-list');
      await pg.getByRole('button', { name: 'Not you? Switch' }).click();
      await until(pg, () => (document.querySelector('.me-page h2') || {}).textContent === 'Who are you?', null, 'Switch asks again');
      // Yes, Maybe, No quickly: the last one is what stays (the writes go one after the other)
      await pg.click('button.me-pick:has-text("Pia Demo")');
      await pg.waitForSelector('.me-list');
      for (const l of ['good', 'poor', 'never', 'never', 'good']) await first.locator(`button.me-ans.${l}`).click();
      await pg.waitForTimeout(400);
      await pg.reload();
      await pg.waitForSelector('.me-list');
      ok((await first.locator('button.me-ans.good').getAttribute('aria-pressed')) === 'true', 'quick clicks end as the last one');
      await first.locator('button.me-ans.good').click();
      // the organisation's page leads an answer link to its plan, and the plan has no way back to it
      await pg.goto(base + '/o/sports-club');
      await pg.waitForURL('**/o/sports-club/p/autumn-2026');
      await pg.waitForSelector('.me-page');
      ok(!(await pg.locator('.title-bar .back').count()), 'no link back to the organisation for a link');
      await pg.unroute('**/dutyplan.v1.OrgService/GetOrg');
      // an edit link to one plan opens that plan, not the organisation, and makes no links
      await pg.route('**/dutyplan.v1.OrgService/GetOrg', asLink({ level: 'edit', plan: 'autumn-2026' }));
      await pg.goto(base + '/o/sports-club');
      await pg.waitForURL('**/o/sports-club/p/autumn-2026');
      ok(!(await pg.locator('button.share').count()), 'no Share for someone with a link');
      await c.close();
    }

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
