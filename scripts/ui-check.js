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
    const context = await browser.newContext({ locale: 'en-GB', viewport: { width: 1280, height: 900 } });
    await context.addInitScript(() => { try { if (!localStorage.getItem('tp.lang')) localStorage.setItem('tp.lang', 'en'); } catch (e) { /* ignore */ } });
    await context.grantPermissions(['clipboard-read', 'clipboard-write']);
    const page = await context.newPage();
    const watch = (p) => {
      p.on('response', (r) => { if (r.status() === 404) notFound.push(new URL(r.url()).pathname); });
      p.on('dialog', (d) => d.accept());  // confirmations (deleting and so on) are accepted
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
    };
    // addRow: a dashed line with a name and «Add»
    const addNamed = async (placeholder, name) => {
      const form = page.locator(`form.addrow:has(input[placeholder="${placeholder}"])`);
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
    await page.selectOption('header select:not(#lang)', 'sports-club');
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
      await addNamed('Add a row, e.g. Theme', name);
      await page.waitForSelector(`button[aria-label="Change the row ${name}"]`);
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
    // Fill row (in the task's dialog): the same number on every event
    await page.getByRole('button', { name: 'Open the task Sound' }).click();
    await reopens(async () => {
      await dialog.locator('input[aria-label="Places for Sound on every event"]').fill('1');
      await dialog.getByRole('button', { name: 'Fill row' }).click();
    }, 'Fill row draws the page again');
    await closeDialog();
    for (const e of ev) ok((await places('Sound', e).inputValue()) === '1', 'Fill row puts 1 on ' + e);
    await places('Café', ev[0]).fill('2');
    await places('Café', ev[0]).press('Enter');
    await places('Café', ev[1]).fill('1');
    await places('Café', ev[1]).press('Enter');
    await places('Video', ev[0]).fill('1');
    await places('Video', ev[0]).press('Tab');
    const draft = (task, e) => page.locator(`button[aria-label="${task}, ${e}: open the place"]`);
    await until(page, () => !!document.querySelector('button[aria-label="Video, Sun 2 Aug 11:00: open the place"]'), null, 'a place gets a cell in the draft without a reload');
    ok((await draft('Sound', ev[0]).innerText()).includes('1 open place'), 'an open place before the first draft');
    await page.reload();
    await h1is(page, 'Autumn 2026');
    ok((await places('Café', ev[0]).inputValue()) === '2' && (await places('Video', ev[1]).inputValue()) === '', 'places stay after a reload; empty = not needed');
    ok((await page.locator('td.draft.none').count()) > 0, 'no places: nothing to fill in the draft');

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

    // details rows: a cell is a field; ✎ moves a row
    const value = (row, e) => page.locator(`input[aria-label="${row} ${e}"]`);
    await value('Note', ev[0]).fill('Kick-off');
    await value('Note', ev[0]).press('Enter');
    await until(page, (x) => document.activeElement && document.activeElement.getAttribute('aria-label') === x, 'Note ' + ev[1], 'Enter moves on to the next event');
    await value('Note', ev[1]).fill('Away');
    await value('Note', ev[1]).press('Tab');
    await page.waitForTimeout(200);
    await page.click('button[aria-label="Change the row Theme"]');
    await dialog.locator('select[aria-label="Where"]').selectOption({ label: 'At the top' });
    await dialog.getByRole('button', { name: 'Save' }).click();
    await dialog.waitFor({ state: 'detached' });
    await until(page, () => [...document.querySelectorAll('tr.field-row .nm')].map((x) => x.textContent).join(',') === 'Theme,Note', null, 'Theme is moved to the top');
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
    const peopleOrder = () => page.evaluate(() => [...document.querySelectorAll('tr.person-row .cellbtn, tr.person-sect .nm')]
      .map((b) => (b.classList.contains('nm') ? '| ' + b.textContent : b.firstChild.textContent)).join(','));
    ok((await peopleOrder()) === 'Kari Test,Ola Sample,Jon Demo', 'people in the order they were added: ' + await peopleOrder());
    const personRowOf = (name) => page.locator('tr.person-row', { has: page.locator(`button[aria-label="Open ${name}"]`) });
    await personRowOf('Jon Demo').locator('.grip').dragTo(personRowOf('Kari Test'));
    await until(page, () => [...document.querySelectorAll('tr.person-row .cellbtn')].map((b) => b.firstChild.textContent).join(',') === 'Jon Demo,Kari Test,Ola Sample', null, 'dragging ⋮⋮ moves a person to the top');
    await addNamed('Add a section', 'Not this term');
    await page.waitForSelector('tr.person-sect');
    ok((await page.innerText('tr.person-sect')).includes('Drag people here'), 'a new section is empty');
    await personRowOf('Ola Sample').locator('.grip').dragTo(page.locator('tr.person-sect'));
    await until(page, () => document.querySelector('tr.person-sect').innerText.includes('1 person'), null, 'a person dropped on a section heading goes into it');
    ok((await peopleOrder()) === 'Jon Demo,Kari Test,| Not this term,Ola Sample', 'without a section first, then the section: ' + await peopleOrder());
    // the person dialog places someone in a section too (for phones)
    await page.click('button[aria-label="Open Jon Demo"]');
    await reopens(() => dialog.locator('select[aria-label="Section in the table"]').selectOption({ label: 'Not this term' }), 'the person dialog opens again');
    await closeDialog();
    ok((await peopleOrder()) === 'Kari Test,| Not this term,Ola Sample,Jon Demo', 'the dialog puts a person last in the section: ' + await peopleOrder());
    // deleting the section puts its people back, without a section
    await page.click('button[aria-label="Change the section Not this term"]');
    await dialog.getByRole('button', { name: 'Delete the section' }).click();
    await until(page, () => !document.querySelector('tr.person-sect'), null, 'the section is gone');
    ok((await peopleOrder()) === 'Kari Test,Ola Sample,Jon Demo', 'its people come back in their order: ' + await peopleOrder());
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
    await takes('Kari Test', [['Sound', 'good'], ['Video', 'poor']]);
    await takes('Ola Sample', [['Sound', 'good']]);
    await takes('Jon Demo', [['Sound', 'poor'], ['Video', 'good']]);
    ok((await page.innerText('button[aria-label="Open Kari Test"]')).includes('Sound, Video'), 'the row shows the tasks a person takes');
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
    // An answer is shown at once and only sent: nothing is fetched again, so nothing flickers.
    {
      const gets = [];
      const seen = (r) => { if (r.method() === 'GET' && new URL(r.url()).pathname.startsWith('/api/')) gets.push(new URL(r.url()).pathname); };
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
      const seen = (r) => { if (r.method() === 'GET' && new URL(r.url()).pathname.startsWith('/api/')) gets.push(new URL(r.url()).pathname.split('/').slice(-1)[0]); };
      await page.click('button[aria-label="Open Kari Test"]');
      const btn = () => dialog.locator('button.cyc[aria-label^="Video for Kari Test:"]');
      const was = await btn().getAttribute('data-level');
      await reopens(() => btn().click(), 'the person dialog is drawn again');  // (what earlier changes left out of date is fetched here)
      page.on('request', seen);
      await reopens(() => btn().click(), 'the person dialog is drawn again');
      page.off('request', seen);
      ok(gets.length > 0 && gets.every((x) => ['answers', 'runs'].includes(x) || /^\d+$/.test(x)), 'a task level fetches only the answers and the drafts again: ' + gets.join(', '));
      while ((await btn().getAttribute('data-level')) !== was) await reopens(() => btn().click(), 'and back');
      await closeDialog();
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
      other.on('request', (r) => { if (r.method() === 'GET' && new URL(r.url()).pathname.startsWith('/api/')) otherGets.push(new URL(r.url()).pathname.split('/').slice(-1)[0]); });
      await answer('Jon Demo', ev[2]).click();
      await until(other, (x) => document.querySelector(`button.ans[aria-label^="${x}:"]`).dataset.level === 'good', 'Jon Demo, ' + ev[2], 'the other tab shows the answer by itself');
      ok(await other.evaluate(() => window.notReloaded === true), 'without a reload');
      ok(otherGets.every((x) => ['answers', 'runs'].includes(x) || /^\d+$/.test(x)), 'the other tab fetches only the answers and the drafts: ' + otherGets.join(', '));
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
    await draft('Café', ev[0]).click();
    await dialog.locator('input[aria-label="Text"]').fill('Outside guest');
    await dialog.locator('input[aria-label="Places it fills"]').fill('2');
    await reopens(() => dialog.getByRole('button', { name: 'Set it' }).click(), 'the place dialog opens again');
    ok((await dialog.innerText()).includes('Outside guest ×2'), 'the text is in the place');
    await closeDialog();
    ok((await draft('Café', ev[0]).locator('.slot.hand').innerText()) === 'Outside guest ×2', 'set by hand is violet in the draft');
    // several people by hand at once, more than the places
    const sound2places = await places('Sound', ev[2]).inputValue();
    await draft('Sound', ev[2]).click();
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
    await barHas('Draft #1', 'the first draft is shown');
    let sound0 = await draft('Sound', ev[0]).innerText();
    ok(/Kari Test|Ola Sample|Jon Demo/.test(sound0), 'the planner filled Sound: ' + sound0);
    const cafe1 = await draft('Café', ev[1]).innerText();
    ok(cafe1.includes('Not filled') && cafe1.includes('Why?'), 'an unfilled place says so: ' + cafe1);
    await draft('Café', ev[1]).click();
    const why = await dialog.innerText();
    ok(why.includes('Not filled:') && why.includes('have not chosen the task'), 'the place dialog explains: ' + why);
    await closeDialog();

    await draft('Sound', ev[0]).click();
    const lockBtn = dialog.getByRole('button', { name: /^Lock / });
    const lockedName = (await lockBtn.getAttribute('aria-label')).slice(5);
    await reopens(() => lockBtn.click(), 'the place dialog opens again');
    ok((await dialog.innerText()).includes('Unlock'), 'a locked name can be unlocked');
    await closeDialog();
    sound0 = await draft('Sound', ev[0]).innerText();
    ok(sound0.includes(lockedName) && sound0.includes('locked'), 'the locked name has the label: ' + sound0);
    await page.getByRole('button', { name: 'Try again' }).click();
    await barHas('Draft #2', 'try again makes a second draft');
    sound0 = await draft('Sound', ev[0]).innerText();
    ok(sound0.includes(lockedName) && sound0.includes('locked'), 'the locked name is kept: ' + sound0);

    await page.getByRole('button', { name: 'Make it the current plan' }).click();
    await msgHas(page, 'Draft #2 is now the current plan.');
    await barHas('✓ Current plan', 'the pinned draft is the current plan');
    // an answer changes: the draft is out of date
    await answer('Ola Sample', ev[2]).click();
    await until(page, (x) => document.querySelector(`button.ans[aria-label^="${x}:"]`).dataset.level === 'good', 'Ola Sample, ' + ev[2], 'the answer is saved');
    await page.reload();
    await barHas('have changed since this draft was made', 'an out-of-date draft is marked');
    // earlier drafts, under ⚙
    await page.getByRole('button', { name: 'Plan settings' }).click();
    ok((await dialog.innerText()).includes('Draft #1') && (await dialog.innerText()).includes('Current plan'), 'the earlier drafts are listed with the current plan');
    await dialog.getByRole('button', { name: 'Show draft #1' }).click();
    await barHas('Draft #1', 'an earlier draft can be shown');
    await barHas('Show the newest', 'and the newest is one click away');
    await page.getByRole('button', { name: 'Show the newest' }).click();
    await barHas('Draft #2', 'back to the newest');
    // the distribution, closed with its own Close button (as every dialog is)
    await page.getByRole('button', { name: 'Distribution', exact: true }).click();
    ok((await dialog.innerText()).includes('fair share'), 'the distribution lists the load against the fair share');
    await dialog.locator('.modal-foot').getByRole('button', { name: 'Close', exact: true }).click();
    await dialog.waitFor({ state: 'detached' });

    // --- copy an event, delete one ---
    await page.click(`button[aria-label="Copy the event ${ev[2]}"]`);
    const newDate = () => dialog.locator('input[aria-label="New date"]').inputValue();
    ok((await newDate()) === '2026-09-06', 'the copy goes a week later by default');
    await dialog.locator('select[aria-label="Repeat"]').selectOption({ label: 'Every 3 weeks' });
    ok((await newDate()) === '2026-09-20', 'the suggested date follows the rhythm: ' + (await newDate()));
    await dialog.locator('select[aria-label="Repeat"]').selectOption({ label: 'Does not repeat' });
    ok((await newDate()) === '2026-09-06', 'and goes back without a repeat');
    ok((await dialog.innerText()).includes('11:00–12:30') && (await dialog.innerText()).includes('Sound 1'), 'the dialog shows what comes along');
    await dialog.getByRole('button', { name: 'Copy', exact: true }).click();
    await msgHas(page, '1 copy made.');
    ok((await cols()) === 4, 'the copy is a new column');
    const ev3 = 'Sun 6 Sept 11:00';
    ok((await places('Sound', ev3).inputValue()) === '1' && (await value('Theme', ev3).inputValue()) === 'Finals', 'the places and the details come along');
    ok((await answer('Kari Test', ev3).getAttribute('data-level')) === 'none', 'answers are never copied');
    await page.click(`button[aria-label="Open the event ${ev3}"]`);
    await dialog.getByRole('button', { name: 'Delete the event' }).click();
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
    await page.waitForURL('**/o/sports-club');
    await msgHas(page, 'The plan “Spring 2027 (draft)” was deleted.');
    await page.getByRole('button', { name: 'Rename or delete the organisation' }).click();
    ok((await dialog.innerText()).includes('cannot be deleted'), 'an organisation with plans cannot be deleted');
    await closeDialog();

    // --- a phone: no page is wider than the screen; the table scrolls in its own box with the first column in place ---
    await page.setViewportSize({ width: 390, height: 844 });
    for (const url of ['/o/sports-club', planUrl]) {
      await page.goto(base + url);
      await page.waitForSelector('h1');
      await page.waitForTimeout(150);
      const w = await page.evaluate(() => [document.documentElement.scrollWidth, document.documentElement.clientWidth]);
      ok(w[0] <= w[1], `the page ${url} is ${w[0]} px wide on a ${w[1]} px screen (scrolls sideways)`);
    }
    // the availability is turned (events down, people across) above the rest; the answers are only there
    await page.waitForSelector('.dp-table.turned');
    ok((await page.locator('.dp-table.turned button.ans').count()) === 9 && (await page.locator('button.ans').count()) === 9, 'on a phone the answers are in the turned table only');
    ok((await page.locator('.dp-table.turned th[scope=row]').first().evaluate((e) => getComputedStyle(e).position)) === 'sticky', 'the event column stays put');
    const stuck = await page.evaluate(() => {
      const box = document.querySelectorAll('.grid-scroll')[1];
      const th = box.querySelector('tbody th[scope=row]');
      const x0 = th.getBoundingClientRect().left;
      box.scrollLeft = 300;
      return [box.scrollLeft > 0, Math.abs(th.getBoundingClientRect().left - x0) < 1];
    });
    ok(stuck[0] && stuck[1], 'the rest of the table scrolls sideways and the row names stay put: ' + stuck);
    const gotAt = (person, e) => answer(person, e).locator('xpath=..').locator('.got').innerText().catch(() => '');
    ok((await gotAt(lockedName, ev[0])).includes('Sound'), 'under the answer: the task the person has in the draft (the locked name)');
    const countOf = (e) => page.locator('.dp-table.turned tbody tr', { has: answer('Jon Demo', e) }).locator('.count').getAttribute('aria-label');
    const before0 = await answer('Jon Demo', ev[0]).getAttribute('data-level');
    const count0 = await countOf(ev[0]);
    await answer('Jon Demo', ev[0]).click();
    const next0 = { none: 'good', good: 'poor', poor: 'never', never: 'none' }[before0];
    await until(page, ([x, l]) => document.querySelector(`button.ans[aria-label^="${x}:"]`).dataset.level === l, ['Jon Demo, ' + ev[0], next0], 'a tap moves the answer on');
    ok((await countOf(ev[0])) !== count0, 'the count of the event follows: ' + count0 + ' -> ' + (await countOf(ev[0])));
    await page.click('button[aria-label="Open Kari Test"]');
    ok((await dialog.locator('h2').innerText()) === 'Kari Test', 'a person’s name at the top opens the person');
    await closeDialog();
    await page.setViewportSize({ width: 1280, height: 900 });
    await until(page, () => !document.querySelector('.dp-table.turned') && document.querySelectorAll('.dp-table button.ans').length === 9, null, 'a wide screen gets the table with people as rows again');

    // --- Norwegian: the switch, what the server writes, the errors ---
    await page.goto(base + '/o/sports-club');
    await h1is(page, 'Sports Club');
    await page.selectOption('#lang', 'nb');
    await page.waitForSelector('h2:has-text("Planer")');
    ok((await page.getAttribute('html', 'lang')) === 'nb', 'the page language becomes Norwegian');
    ok((await page.textContent('header .brand')) === 'Tjenesteplan', 'the brand is Tjenesteplan in Norwegian');
    await page.reload();
    await page.waitForSelector('h2:has-text("Planer")');  // and survives a reload
    await page.goto(base + planUrl);
    await h1is(page, 'Autumn 2026');
    await barHas('Utkast #2', 'the draft texts are Norwegian');
    const tbl = await page.innerText('.dp-table');
    ok(tbl.includes('OM HENDELSEN') && tbl.includes('UTKAST TIL FORDELING') && tbl.includes('TILGJENGELIGHET') && tbl.includes('Ikke fylt'), 'the table is Norwegian: ' + tbl.slice(0, 200));
    await page.locator('button[aria-label^="Café, søn. 16. aug."]').click();
    ok((await dialog.innerText()).includes('Ikke fylt:') && (await dialog.innerText()).includes('har ikke valgt oppgaven'), 'the server explains in Norwegian too: ' + (await dialog.innerText()));
    await closeDialog();
    ok((await page.locator('button.ans[aria-label^="Kari Test, søn. 2. aug. 11:00:"]').getAttribute('aria-label')).endsWith(': kanskje'), 'the answers are Norwegian');
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
