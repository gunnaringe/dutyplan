// The pictures of the documentation (docs/img), made with the real app on invented data, the way the README describes it.
// Not a test of the app (app:ui is that). Needs playwright-core and a Chromium in the cache, like app:ui: PLAYWRIGHT_CORE
// (default ../../scoreboard/node_modules/playwright-core) and CHROME (default: the first chromium-* in ~/.cache/ms-playwright).
// Run: mise run app:shots   (builds first; needs mise run setup for the planner). OUT=dir puts the pictures elsewhere.
const fs = require('fs'), os = require('os'), path = require('path');
const { chromium } = require(process.env.PLAYWRIGHT_CORE || path.join(__dirname, '../../scoreboard/node_modules/playwright-core'));
const { spawn, spawnSync } = require('child_process');
const pb = require('./pb');  // a changed answer, in binary as the page reads it

let step = ''; // the picture being made, for the error message
const OUT = process.env.OUT || path.join(__dirname, '../docs/img');
const chromeDir = () => {
  const root = path.join(os.homedir(), '.cache/ms-playwright');
  const d = fs.readdirSync(root).filter((n) => /^chromium-\d+$/.test(n)).sort().pop();
  return path.join(root, d, 'chrome-linux64', 'chrome');
};

// The demo: a community hall with concerts on Saturdays and rehearsals on Wednesdays. Every name is invented.
const ORG = { name: 'Maple Hall', slug: 'maple-hall' };
const PLAN = { name: 'December 2015', slug: 'december-2015' };
const TODAY = '2015-10-21'; // what the pictures show as the day the plans were made (see the timestamps below)
const TASKS = ['Sound', 'Video', 'Doors', 'Coffee'];
const PEOPLE = ['Kari Test', 'Ola Sample', 'Mona Example', 'Pia Demo', 'Ingrid Sample', 'Njål Example', 'Sunniva Test', 'Jon Demo'];
// what each person takes part in: task -> level (poor | good)
const CHOICES = {
  'Kari Test': { Sound: 'good', Video: 'good' },
  'Ola Sample': { Sound: 'good', Doors: 'good' },
  'Mona Example': { Doors: 'good', Coffee: 'good' },
  'Pia Demo': { Video: 'good', Sound: 'poor' },
  'Ingrid Sample': { Video: 'good', Coffee: 'good' },
  'Njål Example': { Doors: 'good', Sound: 'poor' },
  'Sunniva Test': { Doors: 'good', Video: 'good', Coffee: 'good' },
  'Jon Demo': { Sound: 'good', Coffee: 'good' },
};

(async () => {
  if (!process.env.DUTYPLAN_SOLVER) throw new Error('DUTYPLAN_SOLVER is not set (run via mise run app:shots)');
  fs.mkdirSync(OUT, { recursive: true });
  for (const f of fs.readdirSync(OUT)) if (f.endsWith('.png')) fs.unlinkSync(path.join(OUT, f)); // the set is made whole each time
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'dp-shots-'));
  const srv = spawn(path.join(__dirname, '../bin/dutyplan'), ['--addr', '127.0.0.1:0', '--db', path.join(dir, 'shots.db')], { stdio: ['ignore', 'pipe', 'pipe'] });
  let base = '';
  await new Promise((res, rej) => {
    srv.stdout.on('data', (d) => { const m = /http:\/\/\S+/.exec(String(d)); if (m && !base) { base = m[0]; res(); } });
    setTimeout(() => rej(new Error('server start timeout')), 5000);
  });

  // ---- the data, through the API like any client ----
  // a Connect call: POST /dutyplan.v1.<Service>/<Method> with the organisation and plan in the body
  const api = async (proc, body = {}) => {
    const r = await fetch(`${base}/dutyplan.v1.${proc}`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ org: ORG.slug, ...body }) });
    const text = await r.text();
    if (!r.ok) throw new Error(`${proc}: ${r.status} ${text}`);
    return JSON.parse(text);
  };
  const plan = PLAN.slug;
  let browser, browserNb;
  try {
    await api('AccountService/CreateOrg', { org: undefined, name: ORG.name });
    // the plan, with its own tasks and people: concerts every Saturday and rehearsals every Wednesday, each event with its own places
    await api('PlanService/CreatePlan', { name: PLAN.name });
    for (const name of TASKS) await api('SetupService/CreateTask', { plan, name });
    const task = Object.fromEntries((await api('SetupService/ListTasks', { plan })).tasks.map((t) => [t.name, t.id]));
    for (const [a, b, level] of [['Sound', 'Doors', 'cannot'], ['Video', 'Doors', 'cannot']]) {
      await api('SetupService/SetCombination', { plan, a: task[a], b: task[b], level });
    }
    for (const name of PEOPLE) await api('SetupService/CreatePerson', { plan, name });
    const person = Object.fromEntries((await api('SetupService/ListPeople', { plan })).people.map((x) => [x.name, x.id]));
    // a section: people placed under their own heading in Availability (only placement)
    const guests = (await api('SetupService/CreateSection', { plan, name: 'Guests' })).section;
    for (const name of ['Njål Example', 'Sunniva Test']) await api('SetupService/MovePerson', { plan, id: person[name], section: guests.id, position: 99 });
    await api('PlanService/CreatePlan', { name: 'Spring 2016', from: plan });  // a second plan, with copies of this one's people and tasks
    // the tasks each person takes, in this plan
    for (const [name, tasks] of Object.entries(CHOICES)) {
      for (const [t, level] of Object.entries(tasks)) await api('PlanService/SetPersonTask', { plan, person: person[name], task: task[t], level });
    }
    // (as on the page: one event, its places typed in, then copied every week)
    const weekly = async (date, nextWeek, until, times, places) => {
      const [first] = (await api('PlanService/AddEvents', { plan, date, ...times })).events;
      for (const [t, slots] of Object.entries(places)) await api('PlanService/SetPlaces', { plan, event: first.id, task: task[t], slots });
      await api('PlanService/CopyEvent', { plan, id: first.id, dates: [nextWeek], until, times: true, places: true });
    };
    await weekly('2015-12-05', '2015-12-12', '2015-12-19', { start_time: '19:00', end_time: '21:30' }, { Sound: 1, Video: 1, Doors: 2, Coffee: 1 });
    await weekly('2015-12-09', '2015-12-16', '2015-12-16', { start_time: '18:00', end_time: '20:00' }, { Sound: 1, Coffee: 1 });
    const events = (await api('PlanService/ListEvents', { plan })).events;
    const on = (d) => events.find((e) => e.date === d);
    const concerts = ['2015-12-05', '2015-12-12', '2015-12-19'].map(on), rehearsals = ['2015-12-09', '2015-12-16'].map(on);
    const gala = concerts[2];
    await api('PlanService/UpdateEvent', { plan, id: gala.id, start_time: '18:30', end_time: '22:00' });
    await api('PlanService/SetPlaces', { plan, event: gala.id, task: task.Doors, slots: 3 });

    // the plan's own details rows: text for people, which the planner never reads
    const note = (await api('PlanService/AddField', { plan, name: 'Note' })).field;
    const theme = (await api('PlanService/AddField', { plan, name: 'Theme' })).field;
    for (const [ev, value] of [[concerts[0], 'Concert'], [rehearsals[0], 'Rehearsal'], [concerts[1], 'Concert'], [rehearsals[1], 'Rehearsal'], [gala, 'Christmas gala']]) {
      await api('PlanService/SetFieldValue', { plan, event: ev.id, field: note.id, value });
    }
    for (const [i, value] of [[0, 'Brass night'], [1, 'Strings'], [2, 'Christmas']]) await api('PlanService/SetFieldValue', { plan, event: concerts[i].id, field: theme.id, value });

    // answers: when people can, would rather not, or cannot (event id -> good | poor | never)
    const answer = (who, spec) => api('PlanService/SetAvailability', { plan, person: person[who], levels: Object.fromEntries(spec.map(([ev, l]) => [ev.id, l])) });
    await answer('Kari Test', [[concerts[0], 'good'], [rehearsals[0], 'never'], [concerts[1], 'never'], [gala, 'poor']]);
    await answer('Ola Sample', [[concerts[0], 'good'], [rehearsals[0], 'never'], [concerts[1], 'never'], [gala, 'good']]);
    await answer('Mona Example', [[concerts[0], 'good'], [concerts[1], 'good'], [gala, 'good']]);
    await answer('Pia Demo', [[concerts[0], 'poor'], [rehearsals[0], 'good'], [gala, 'never']]);
    await answer('Ingrid Sample', [[concerts[1], 'good'], [rehearsals[1], 'good']]);
    await answer('Njål Example', [[rehearsals[0], 'poor'], [concerts[1], 'never'], [gala, 'good']]);
    await answer('Sunniva Test', [[concerts[0], 'good'], [concerts[1], 'good'], [gala, 'never']]);
    await answer('Jon Demo', [[concerts[0], 'never'], [rehearsals[0], 'never'], [rehearsals[1], 'good']]);

    // set by hand: a person and a text at the doors; and a selection: only these two can run the video at the gala (and neither
    // can come, so the place stays empty and the draft says why)
    await api('DraftService/AddCell', { plan, event: concerts[0].id, task: task.Doors, person: person['Ola Sample'] });
    await api('DraftService/AddCell', { plan, event: gala.id, task: task.Doors, label: 'Stewards', fills: 2 });
    await api('DraftService/SetPool', { plan, event: gala.id, task: task.Video, people: [person['Pia Demo'], person['Sunniva Test']] });

    const draft = async () => {
      const started = await api('DraftService/StartRun', { plan, time_limit: 10 });
      for (let i = 0; i < 120; i++) {
        const run = (await api('DraftService/GetRun', { plan, id: started.id })).run;
        if (run.status === 'done') return run;
        if (run.status === 'failed') throw new Error('the planner failed: ' + JSON.stringify(run));
        await new Promise((r) => setTimeout(r, 500));
      }
      throw new Error('the planner took too long');
    };
    // a first draft; then one of its names is locked and the draft is tried again, and the newer one is pinned as the current plan
    await draft();
    await api('DraftService/AddCell', { plan, event: concerts[0].id, task: task.Sound, person: person['Kari Test'], locked: true });
    const second = await draft();
    await api('DraftService/PinRun', { plan, id: second.id });

    // The server stamps what it writes with the real time. The pictures should not change with the day they are made, so the
    // stamps in the temporary database are set to a fixed day (the page shows what is stored, in UTC below).
    const { DatabaseSync } = require('node:sqlite');
    const db = new DatabaseSync(path.join(dir, 'shots.db'), { timeout: 5000 });
    const day = TODAY + ' 10:00:00';
    for (const { t, c } of db.prepare(`SELECT m.name AS t, p.name AS c FROM sqlite_master m, pragma_table_info(m.name) p WHERE m.type = 'table' AND p.name IN ('created_at', 'at', 'started_at', 'finished_at')`).all()) {
      db.prepare(`UPDATE "${t}" SET "${c}" = strftime('%Y-%m-%dT%H:%M:%SZ', ?, '+' || (rowid * 5) || ' seconds') WHERE "${c}" IS NOT NULL`).run(day);
    }
    db.prepare(`UPDATE plan_run SET started_at = strftime('%Y-%m-%dT%H:%M:%SZ', ?, '+' || (30 + id * 2) || ' minutes')`).run(day);
    db.prepare(`UPDATE plan_run SET finished_at = strftime('%Y-%m-%dT%H:%M:%SZ', started_at, '+12 seconds') WHERE finished_at IS NOT NULL`).run();
    db.close();

    // ---- the pictures ----
    const launch = (lang) => chromium.launch({ executablePath: process.env.CHROME || chromeDir(), args: ['--no-sandbox', lang === 'nb' ? '--lang=nb' : '--lang=en-GB'] });
    browser = await launch('en');
    browserNb = await launch('nb');
    const context = async (opts, lang = 'en') => {
      const c = await (lang === 'nb' ? browserNb : browser).newContext({ locale: lang === 'nb' ? 'nb-NO' : 'en-GB', deviceScaleFactor: 2, timezoneId: 'UTC', ...opts });
      await c.addInitScript((l) => { try { localStorage.setItem('tp.lang', l); } catch (e) { /* ignore */ } }, lang);
      return c;
    };
    const errors = [];
    const open = async (c) => { const pg = await c.newPage(); pg.on('pageerror', (e) => errors.push(e.message)); return pg; };
    const go = async (pg, path, ready = 'main h1') => { await pg.goto(base + path); await pg.waitForSelector(ready, { timeout: 10000 }); await pg.waitForLoadState('networkidle'); };
    const save = async (pg, name, target) => {
      await pg.waitForTimeout(250);
      await (target || pg).screenshot({ path: path.join(OUT, name + '.png') });
    };
    // From the top edge of one element to the bottom edge of another, over the width of both.
    const range = async (pg, name, from, to) => {
      await pg.evaluate(() => window.scrollTo(0, 0)); // the boxes are measured from the window, the clip is from the page
      const a = await from.boundingBox(), b = await to.boundingBox();
      const x = Math.min(a.x, b.x), right = Math.max(a.x + a.width, b.x + b.width);
      await pg.waitForTimeout(250);
      await pg.screenshot({ path: path.join(OUT, name + '.png'), fullPage: true, clip: { x: x - 16, y: a.y - 8, width: right - x + 32, height: b.y + b.height - a.y + 16 } });
    };
    // From the very top of the page, so the header and the title are in the picture.
    const fromTop = async (pg, name, to) => {
      await pg.evaluate(() => window.scrollTo(0, 0));
      const b = await to.boundingBox();
      await pg.waitForTimeout(250);
      await pg.screenshot({ path: path.join(OUT, name + '.png'), fullPage: true, clip: { x: 0, y: 0, width: pg.viewportSize().width, height: b.y + b.height + 20 } });
    };
    const planPath = `/o/${ORG.slug}/p/${PLAN.slug}`;
    const dialog = (pg) => pg.locator('dialog.modal');
    const closeDialog = async (pg) => { await pg.keyboard.press('Escape'); await dialog(pg).waitFor({ state: 'detached' }); };
    const shoot = async (pg, name, button) => { step = name; await button.click(); await dialog(pg).waitFor(); await save(pg, name, dialog(pg)); await closeDialog(pg); };

    // The plan page: the whole table, with the current plan in it. The picture at the top of the README.
    const wide = await context({ viewport: { width: 1280, height: 900 } });
    const page = await open(wide);
    await go(page, planPath, '.dp-table');
    await range(page, 'hero', page.locator('main h1'), page.locator('.grid-scroll'));

    // The organisation and its plans.
    await go(page, `/o/${ORG.slug}`);
    await fromTop(page, '01-organisation', page.locator('.plans'));

    // The sections of the table, one by one.
    await go(page, planPath, '.dp-table');
    const sects = page.locator('.dp-table tr.sect');
    const draftHead = page.locator('.dp-table tr.sect.draft');
    await range(page, '03-details-places', sects.nth(0), draftHead.locator('xpath=preceding-sibling::tr[not(@hidden)][1]'));
    const availability = sects.nth(3);
    await range(page, '04-draft', draftHead, availability.locator('xpath=preceding-sibling::tr[not(@hidden)][1]'));
    await range(page, '05-availability', availability, page.locator('.dp-table tbody tr:not([hidden])').last());
    // A name typed into a cell of the draft: the suggestions (yes first, no last) and a name of its own.
    step = '19-cell-names';
    await page.locator('td.draft', { has: page.getByRole('button', { name: /^Sound, Sat 12 Dec .*: open the place$/ }) }).locator('.dlines').click();
    await page.locator('td.draft.editing input.ename').pressSequentially('S');
    await range(page, '19-cell-names', draftHead, page.locator('td.draft.editing ul.elist'));
    await page.keyboard.press('Escape');

    // The dialogs behind the names.
    await shoot(page, '06-task', page.getByRole('button', { name: 'Open the task Doors' }));
    await shoot(page, '07-event', page.getByRole('button', { name: /^Open the event Sat 19 Dec/ }));
    await page.getByRole('button', { name: /^Copy the event Sat 5 Dec/ }).click();
    await dialog(page).waitFor();
    await dialog(page).getByRole('combobox', { name: 'Repeat' }).selectOption({ label: 'Every other week' });
    await dialog(page).getByLabel('New date').fill('2015-12-26');
    await dialog(page).getByLabel('The last date').fill('2016-01-30');
    await dialog(page).getByLabel('The last date').evaluate((e) => e.blur());
    await save(page, '08-copy', dialog(page));
    await closeDialog(page);
    await page.getByRole('button', { name: 'Open Kari Test' }).click();
    await dialog(page).waitFor();
    await save(page, '09-person', dialog(page));
    await closeDialog(page);
    await shoot(page, '10-place', page.getByRole('button', { name: /^Video, Sat 19 Dec .*: open the place$/ }));
    step = '12-distribution';
    await page.locator('.draft-bar .more > summary').click();  // behind More ⋯
    await page.getByRole('button', { name: 'Distribution', exact: true }).click();
    await dialog(page).waitFor();
    await save(page, '12-distribution', dialog(page));
    await closeDialog(page);

    // «Edit | View»: the plan as it stands, with a name found; on a phone a card per event (the choice is kept in that browser,
    // so back to Edit after).
    step = '16-view';
    await page.click('.title-bar .mode button[data-mode=show]');
    await page.waitForSelector('.show-tools');
    await page.fill('input.find', 'Kari');
    await page.keyboard.press('Enter');
    await page.locator('input.find').blur();
    await range(page, '16-view', page.locator('main h1'), page.locator('.show-table tbody tr').last());
    step = '22-export';
    await page.click('.show-tools .export > summary');
    await range(page, '22-export', page.locator('main h1'), page.locator('.show-table tbody tr').nth(3));
    await page.keyboard.press('Escape');
    await page.click('.title-bar .mode button[data-mode=edit]');
    await page.waitForSelector('.dp-table');

    // ⚙: the name, what matters (a line) and the earlier drafts, in a window tall enough for all of it.
    const tall = await open(await context({ viewport: { width: 1280, height: 2000 } }));
    await go(tall, planPath, '.dp-table');
    await tall.getByRole('button', { name: /Plan settings/ }).click();
    await dialog(tall).waitFor();
    await save(tall, '11-settings', dialog(tall));
    await closeDialog(tall);
    // «What matters», from the draft's bar: the three priorities and the rules that always hold (the numbers folded away)
    await tall.locator('.prio-link').click();
    await dialog(tall).waitFor();
    await save(tall, '20-what-matters', dialog(tall));
    await closeDialog(tall);
    // the organisation's page: its plans, a new plan copied from another, and ⋯ Duplicate with or without the filled-in table
    const orgPage = await open(await context({ viewport: { width: 1100, height: 640 } }));
    await go(orgPage, '/o/' + ORG.slug);
    await orgPage.click(`details.plan-menu:has(summary[aria-label="More for ${PLAN.name}"]) > summary`);
    await orgPage.getByRole('button', { name: 'Duplicate', exact: true }).click();
    await dialog(orgPage).waitFor();
    await save(orgPage, '21-duplicate');
    await closeDialog(orgPage);

    // On a phone: the same table, narrower, scrolling sideways under the names.
    const phone = await open(await context({ viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true }));
    await go(phone, planPath, '.dp-table');
    await phone.screenshot({ path: path.join(OUT, '14-phone.png') });
    step = '17-view-phone';
    await phone.click('.title-bar .mode button[data-mode=show]');
    await phone.waitForSelector('.show-cards');
    await phone.fill('input.find', 'Kari');
    await phone.keyboard.press('Enter');
    await phone.locator('input.find').blur();
    await phone.screenshot({ path: path.join(OUT, '17-view-phone.png') });

    // «My answers», what an answer link opens (this server has no login: the organisation is made to say level answer)
    step = '18-my-answers';
    const answering = await open(await context({ viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true }));
    await answering.route('**/dutyplan.v1.OrgService/GetOrg', (route) => pb.edit(route, (o) => ({ ...o, role: '', level: 'answer', plan: PLAN.slug })));
    await go(answering, planPath, '.me-page');
    await answering.click('button.me-pick:has-text("Kari Test")');
    await answering.waitForSelector('.me-list');
    await answering.screenshot({ path: path.join(OUT, '18-my-answers.png') });

    const nb = await open(await context({ viewport: { width: 1280, height: 900 } }, 'nb'));
    await go(nb, planPath, '.dp-table');
    await range(nb, '15-norwegian', nb.locator('main h1'), nb.locator('.grid-scroll'));

    if (errors.length) throw new Error('the page failed: ' + errors.join('; '));
    // Smaller files: a text page has few colours. pngquant is optional (the pictures are kept as they are without it).
    if (spawnSync('pngquant', ['--version']).status === 0) {
      for (const f of fs.readdirSync(OUT).filter((n) => n.endsWith('.png'))) {
        spawnSync('pngquant', ['--quality', '65-95', '--strip', '--force', '--ext', '.png', path.join(OUT, f)]);
      }
    }
    fs.copyFileSync(path.join(OUT, 'hero.png'), path.join(__dirname, '../web/landing.png')); // the picture on the start page for people not signed in
    console.log('shots: ok, ' + fs.readdirSync(OUT).filter((f) => f.endsWith('.png')).length + ' pictures in ' + OUT);
  } finally {
    if (browserNb) await browserNb.close();
    if (browser) await browser.close();
    srv.kill('SIGTERM');
  }
})().catch((e) => { console.error((step ? step + ': ' : '') + e.message); process.exit(1); });
