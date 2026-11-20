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
    // A change in some dialogs draws the page again and opens the same dialog on top: wait for the new one.
    const reopens = async (action, what) => {
      await page.evaluate(() => document.querySelectorAll('dialog.modal').forEach((d) => { d.dataset.old = '1'; }));
      await action();
      await page.waitForSelector('dialog.modal[open]:not([data-old])', { timeout: 4000 }).catch(() => { throw new Error('FAIL: ' + what); });
      await until(page, () => !document.querySelector('dialog.modal[data-old]'), null, what);
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
    // a new event, every week until a date
    await page.getByRole('button', { name: 'Add an event' }).click();
    await dialog.locator('input[aria-label="Date"]').fill('2026-08-02');
    await dialog.locator('input[aria-label="Starts"]').fill('11:00');
    await dialog.locator('input[aria-label="Every week until"]').check();
    await dialog.locator('input[aria-label="The last date"]').fill('2026-08-16');
    await until(page, () => document.querySelector('dialog.modal').innerText.includes('→ 3 events'), null, 'the dialog counts the events');
    await dialog.getByRole('button', { name: 'Add', exact: true }).click();
    await msgHas(page, '3 events added.');
    ok((await cols()) === 3, 'three Sundays, one column each');
    const ev = ['Sun 2 Aug 11:00', 'Sun 9 Aug 11:00', 'Sun 16 Aug 11:00'];
    ok((await page.innerText('.dp-table thead')).includes('Sun 2 Aug') && (await page.innerText('.dp-table thead')).includes('11:00'), 'the header has the date and the time');

    // places: typed straight in; Enter moves on to the next event
    const places = (task, e) => page.locator(`input[aria-label="Places for ${task} on ${e}"]`);
    for (const e of ev) {
      await places('Sound', e).fill('1');
      await places('Sound', e).press('Enter');
    }
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
    const takes = async (person, list) => {
      await page.click(`button[aria-label="Open ${person}"]`);
      for (const [task, level] of list) {
        await reopens(() => dialog.locator(`select[aria-label="${task} for ${person}"]`).selectOption(level), 'the person dialog opens again');
        ok((await dialog.locator(`select[aria-label="${task} for ${person}"]`).inputValue()) === level, 'the change is in the new dialog');
      }
      await closeDialog();
    };
    await takes('Kari Test', [['Sound', 'good'], ['Video', 'neutral']]);
    await takes('Ola Sample', [['Sound', 'good']]);
    await takes('Jon Demo', [['Sound', 'neutral'], ['Video', 'good']]);
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

    await page.click('button[aria-label="Open the task Sound"]');
    await reopens(() => dialog.locator('select[aria-label="Sound + Video"]').selectOption('cannot'), 'the task dialog opens again');
    ok((await dialog.locator('select[aria-label="Sound + Video"]').inputValue()) === 'cannot', 'the combination is set');
    await closeDialog();

    // --- the draft: by hand, make a draft, lock, try again, pin ---
    await draft('Café', ev[0]).click();
    await dialog.locator('select[aria-label="What"]').selectOption('text');
    await dialog.locator('input[aria-label="Text"]').fill('Outside guest');
    await dialog.locator('input[aria-label="Places it fills"]').fill('2');
    await reopens(() => dialog.getByRole('button', { name: 'Set it' }).click(), 'the place dialog opens again');
    ok((await dialog.innerText()).includes('Outside guest ×2'), 'the text is in the place');
    await closeDialog();
    ok((await draft('Café', ev[0]).locator('.slot.hand').innerText()) === 'Outside guest ×2', 'set by hand is violet in the draft');
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
    await page.getByRole('button', { name: '⚙ Plan settings' }).click();
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
    ok((await dialog.locator('input[aria-label="New date"]').inputValue()) === '2026-08-23', 'the copy goes a week later by default');
    ok((await dialog.innerText()).includes('11:00–12:30') && (await dialog.innerText()).includes('Sound 1'), 'the dialog shows what comes along');
    await dialog.getByRole('button', { name: 'Copy', exact: true }).click();
    await msgHas(page, '1 copy made.');
    ok((await cols()) === 4, 'the copy is a new column');
    const ev3 = 'Sun 23 Aug 11:00';
    ok((await places('Sound', ev3).inputValue()) === '1' && (await value('Theme', ev3).inputValue()) === 'Finals', 'the places and the details come along');
    ok((await answer('Kari Test', ev3).getAttribute('data-level')) === 'none', 'answers are never copied');
    await page.click(`button[aria-label="Open the event ${ev3}"]`);
    await dialog.getByRole('button', { name: 'Delete the event' }).click();
    await until(page, () => document.querySelectorAll('.dp-table thead th[scope=col]:not(.addcol)').length === 3, null, 'the event is deleted');

    // --- the organisation: groups, a plan to delete, rename ---
    await page.goto(base + '/o/sports-club');
    await h1is(page, 'Sports Club');
    await addNamed('New group', 'Choir');
    await page.click('button[aria-label="Open the group Choir"]');
    await dialog.locator('input[aria-label="Kari Test is in Choir"]').check();
    await dialog.locator('input[aria-label="Ola Sample is in Choir"]').check();
    await reopens(() => dialog.getByRole('button', { name: 'Save the members' }).click(), 'the group dialog opens again');
    ok(await dialog.locator('input[aria-label="Kari Test is in Choir"]').isChecked(), 'the members are saved');
    await dialog.locator('input[aria-label="Places Choir fills in Sound"]').fill('1');
    await reopens(() => dialog.getByRole('button', { name: 'Save', exact: true }).click(), 'the group dialog opens again');
    ok((await dialog.locator('input[aria-label="Places Choir fills in Sound"]').inputValue()) === '1', 'the group’s task is saved');
    await closeDialog();
    const grp = await page.innerText('.grp');
    ok(grp.includes('Kari Test, Ola Sample') && grp.includes('May be proposed for Sound (fills 1)'), 'the group line: ' + grp);

    await addNamed('New plan, e.g. Autumn 2026', 'Spring 2027');
    await page.waitForURL('**/p/spring-2027');
    await page.getByRole('button', { name: '⚙ Plan settings' }).click();
    await dialog.locator('input[aria-label="Name of the plan"]').fill('Spring 2027 (draft)');
    await dialog.getByRole('button', { name: 'Rename' }).click();
    await h1is(page, 'Spring 2027 (draft)');
    ok(page.url().endsWith('/p/spring-2027'), 'a rename keeps the address');
    await page.getByRole('button', { name: '⚙ Plan settings' }).click();
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
    const stuck = await page.evaluate(() => {
      const box = document.querySelector('.grid-scroll');
      const th = document.querySelector('button[aria-label="Open Kari Test"]').closest('th');
      const x0 = th.getBoundingClientRect().left;
      box.scrollLeft = 300;
      return [box.scrollLeft > 0, Math.abs(th.getBoundingClientRect().left - x0) < 1];
    });
    ok(stuck[0] && stuck[1], 'the table scrolls sideways and the names stay put: ' + stuck);
    await page.setViewportSize({ width: 1280, height: 900 });

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
    await page.locator('button[aria-label^="Café, søn. 9. aug."]').click();
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
