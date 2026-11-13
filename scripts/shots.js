// The pictures of the documentation (docs/img), made with the real app on invented data, the way the README describes it.
// Not a test of the app (app:ui is that). Needs playwright-core and a Chromium in the cache, like app:ui: PLAYWRIGHT_CORE
// (default ../../scoreboard/node_modules/playwright-core) and CHROME (default: the first chromium-* in ~/.cache/ms-playwright).
// Run: mise run app:shots   (builds first; needs mise run setup for the planner). OUT=dir puts the pictures elsewhere.
const fs = require('fs'), os = require('os'), path = require('path');
const { chromium } = require(process.env.PLAYWRIGHT_CORE || path.join(__dirname, '../../scoreboard/node_modules/playwright-core'));
const { spawn, spawnSync } = require('child_process');

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
const ADMIN = 'Admin';
const TASKS = ['Sound', 'Video', 'Doors', 'Coffee'];
const PEOPLE = ['Kari Test', 'Ola Sample', 'Mona Example', 'Pia Demo', 'Ingrid Sample', 'Njål Example', 'Sunniva Test', 'Jon Demo'];
// what each person takes part in: task -> level (poor | neutral | good)
const CHOICES = {
  'Kari Test': { Sound: 'good', Video: 'neutral' },
  'Ola Sample': { Sound: 'neutral', Doors: 'good' },
  'Mona Example': { Doors: 'good', Coffee: 'good' },
  'Pia Demo': { Video: 'good', Sound: 'poor' },
  'Ingrid Sample': { Video: 'neutral', Coffee: 'good' },
  'Njål Example': { Doors: 'neutral', Sound: 'poor' },
  'Sunniva Test': { Doors: 'neutral', Video: 'good', Coffee: 'neutral' },
  'Jon Demo': { Sound: 'neutral', Coffee: 'good' },
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
  const api = async (method, p, body) => {
    const r = await fetch(base + '/api' + p, { method, headers: { 'Content-Type': 'application/json', 'X-Actor': ADMIN }, body: body === undefined ? undefined : JSON.stringify(body) });
    const text = await r.text();
    if (!r.ok) throw new Error(`${method} ${p}: ${r.status} ${text}`);
    return text ? JSON.parse(text) : null;
  };
  const o = `/orgs/${ORG.slug}`, p = `${o}/plans/${PLAN.slug}`;
  let browser, browserNb;
  try {
    await api('POST', '/orgs', ORG);
    await api('POST', `${o}/tasks/bulk`, { names: TASKS.join('\n') });
    const task = Object.fromEntries((await api('GET', `${o}/tasks`)).map((t) => [t.name, t.id]));
    for (const [a, b, level] of [['Sound', 'Video', 'impractical'], ['Sound', 'Doors', 'cannot'], ['Video', 'Doors', 'cannot']]) {
      await api('PUT', `${o}/combinations`, { a: task[a], b: task[b], level });
    }
    for (const name of PEOPLE) await api('POST', `${o}/people`, { name });
    const person = Object.fromEntries((await api('GET', `${o}/people`)).map((x) => [x.name, x.id]));
    for (const [name, tasks] of Object.entries(CHOICES)) {
      for (const [t, level] of Object.entries(tasks)) await api('PUT', `${o}/people/${person[name]}/tasks/${task[t]}`, { level });
    }
    for (const name of ['Concert', 'Rehearsal']) await api('POST', `${o}/day-types`, { name });
    const dayType = Object.fromEntries((await api('GET', `${o}/day-types`)).map((d) => [d.name, d.id]));
    await api('PUT', `${o}/day-types/${dayType.Concert}/tasks`, { tasks: [['Sound', 1], ['Video', 1], ['Doors', 2], ['Coffee', 1]].map(([t, slots]) => ({ task_id: task[t], slots })) });
    await api('PUT', `${o}/day-types/${dayType.Rehearsal}/tasks`, { tasks: [['Sound', 1], ['Coffee', 1]].map(([t, slots]) => ({ task_id: task[t], slots })) });
    // a group that works together: it can take Coffee, and the planner counts one assignment for each member
    const group = await api('POST', `${o}/groups`, { name: 'Kitchen duo' });
    await api('PUT', `${o}/groups/${group.id}/members`, { people: [person['Mona Example'], person['Jon Demo']] });
    await api('PUT', `${o}/groups/${group.id}/tasks`, { tasks: [{ task_id: task.Coffee, fills: 1, level: 'good' }] });

    await api('POST', `${o}/plans`, { name: PLAN.name });
    await api('POST', `${p}/events/bulk`, { from: '2015-12-05', to: '2015-12-19', weekdays: [6], day_type_id: dayType.Concert, start_time: '19:00', end_time: '21:30' });
    for (const date of ['2015-12-09', '2015-12-16']) {
      await api('POST', `${p}/events`, { date, day_type_id: dayType.Rehearsal, start_time: '18:00', end_time: '20:00' });
    }
    const events = await api('GET', `${p}/events`);
    const byDate = (d, type) => events.find((e) => e.date === d && e.day_type === type);
    const gala = byDate('2015-12-19', 'Concert');
    await api('PATCH', `${p}/events/${gala.id}`, { note: 'Christmas gala', start_time: '18:30', end_time: '22:00', description: 'Doors at 18:00. Load-in from 15:00 through the side entrance.\nThe stage plot is on the notice board by the kitchen.' });
    const first = byDate('2015-12-05', 'Concert');
    await api('PATCH', `${p}/events/${first.id}`, { description: 'Opening night. Two extra chairs for the guest quartet.' });

    // answers: when people cannot or would rather not (event id -> never | poor | good)
    const concerts = events.filter((e) => e.day_type === 'Concert');
    const levels = (who, spec) => api('PUT', `${p}/people/${person[who]}/availability`, { levels: Object.fromEntries(Object.entries(spec).map(([i, l]) => [concerts[i].id, l])) });
    await levels('Kari Test', { 1: 'never', 2: 'poor' });
    await levels('Ola Sample', { 1: 'never' });
    await levels('Pia Demo', { 0: 'poor', 2: 'never' });
    await levels('Ingrid Sample', { 1: 'good' });
    await levels('Njål Example', { 1: 'never' });
    await levels('Sunniva Test', { 2: 'never' });
    await levels('Jon Demo', { 0: 'never' });

    // a locked place (a manual cell), and a selection: only these two can run the video at the gala (and neither can come, so
    // the place stays empty and the plan says why)
    await api('POST', `${p}/cells`, { events: [first.id], task: task.Doors, person: person['Ola Sample'] });
    await api('PUT', `${p}/pools`, { events: [gala.id], task: task.Video, people: [person['Pia Demo'], person['Sunniva Test']], groups: [] });

    // a plan
    const started = await api('POST', `${p}/runs`, { time_limit: 10 });
    for (let i = 0; i < 120; i++) {
      const run = (await api('GET', `${p}/runs/${started.id}`)).run;
      if (run.status === 'done') break;
      if (run.status === 'failed') throw new Error('the planner failed: ' + JSON.stringify(run));
      await new Promise((r) => setTimeout(r, 500));
    }
    // a second, newer run, so that the history has more than one; the newest is the one pinned as the current plan
    const second = await api('POST', `${p}/runs`, { time_limit: 10, seed: 2 });
    for (let i = 0; i < 120; i++) {
      const run = (await api('GET', `${p}/runs/${second.id}`)).run;
      if (run.status === 'done') break;
      if (run.status === 'failed') throw new Error('the planner failed: ' + JSON.stringify(run));
      await new Promise((r) => setTimeout(r, 500));
    }
    await api('PUT', `${p}/runs/${second.id}/pin`);

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
      await c.addInitScript(([l, who, view]) => { try { localStorage.setItem('tp.lang', l); localStorage.setItem('tp.actor', who); localStorage.setItem('tp.planview', view); } catch (e) { /* ignore */ } }, [lang, ADMIN, opts.planview || 'grid']);
      return c;
    };
    const errors = [];
    const open = async (c) => { const pg = await c.newPage(); pg.on('pageerror', (e) => errors.push(e.message)); return pg; };
    const go = async (pg, path, ready = 'main h1') => { await pg.goto(base + path); await pg.waitForSelector(ready, { timeout: 10000 }); await pg.waitForLoadState('networkidle'); };
    const save = async (pg, name, target) => {
      await pg.waitForTimeout(250);
      await (target || pg).screenshot({ path: path.join(OUT, name + '.png') });
    };
    // A section of the page by its heading (the heading text is matched exactly, so «Plan» is not «Plans»).
    const sec = (pg, title) => pg.locator('main section').filter({ has: pg.locator('h2', { hasText: new RegExp('^' + title + '$') }) }).first();
    // From the top edge of one element to the bottom edge of another, over the whole width of the content column.
    const range = async (pg, name, from, to) => {
      await pg.evaluate(() => window.scrollTo(0, 0)); // the boxes are measured from the window, the clip is from the page
      const a = await from.boundingBox(), b = await to.boundingBox();
      await pg.waitForTimeout(250);
      await pg.screenshot({ path: path.join(OUT, name + '.png'), fullPage: true, clip: { x: a.x - 16, y: a.y - 8, width: a.width + 32, height: b.y + b.height - a.y + 16 } });
    };
    const head = async (pg, name, el, height) => {
      await pg.evaluate(() => window.scrollTo(0, 0));
      const a = await el.boundingBox();
      await pg.waitForTimeout(250);
      await pg.screenshot({ path: path.join(OUT, name + '.png'), fullPage: true, clip: { x: a.x - 16, y: a.y - 8, width: a.width + 32, height } });
    };
    // From the very top of the page, so the header and the title are in the picture.
    const fromTop = async (pg, name, to) => {
      await pg.evaluate(() => window.scrollTo(0, 0));
      const b = await to.boundingBox();
      await pg.waitForTimeout(250);
      await pg.screenshot({ path: path.join(OUT, name + '.png'), fullPage: true, clip: { x: 0, y: 0, width: pg.viewportSize().width, height: b.y + b.height + 20 } });
    };
    const planPath = `/o/${ORG.slug}/p/${PLAN.slug}`;

    // The plan, as a grid: the picture at the top of the README.
    const wide = await context({ viewport: { width: 1000, height: 800 } });
    const page = await open(wide);
    await go(page, planPath);
    await save(page, 'hero', sec(page, 'Plan'));

    await go(page, `/o/${ORG.slug}`);
    await fromTop(page, '01-organisation', page.getByText('the planner never picks the person itself'));
    await go(page, `/o/${ORG.slug}/setup`);
    await save(page, '02-day-types', sec(page, 'Day types'));
    await range(page, '03-groups-combinations', sec(page, 'Groups'), sec(page, 'What can be combined on the same day\\?'));

    await go(page, planPath);
    await save(page, '04-events', sec(page, 'Events'));
    await go(page, `${planPath}/me/${person['Kari Test']}`);
    await page.screenshot({ path: path.join(OUT, '05-my-page.png'), fullPage: true, clip: { x: 0, y: 0, width: 1000, height: 1560 } });

    const list = await open(await context({ viewport: { width: 1000, height: 800 }, planview: 'list' }));
    await go(list, planPath);
    await save(list, '06-plan-list', sec(list, 'Plan'));
    await save(list, '07-why', list.locator('li.plan-event', { hasText: 'Not filled' }));
    await save(list, '08-locks-selections', sec(list, 'Manual cells and selections'));
    await sec(list, 'Weighting').locator('summary').first().click();
    await head(list, '09-weighting', sec(list, 'Weighting'), 720);
    const detailsOf = (re) => sec(list, 'Plan').locator('details', { has: list.locator('summary', { hasText: re }) });
    await detailsOf(/^Earlier runs/).locator('summary').click();
    await detailsOf(/^Distribution$/).locator('summary').click();
    await range(list, '10-history-distribution', detailsOf(/^Earlier runs/), detailsOf(/^Distribution$/));

    // On a phone the header takes a lot of room: the pictures start at what they are about.
    const phone = await open(await context({ viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true, planview: 'list' }));
    await go(phone, planPath);
    await phone.locator('.view-toggle').evaluate((e) => e.scrollIntoView());
    await phone.screenshot({ path: path.join(OUT, '11-phone-plan.png') });
    await go(phone, `${planPath}/me/${person['Kari Test']}`);
    await phone.getByRole('heading', { name: 'What can you take part in?' }).evaluate((e) => e.scrollIntoView());
    await phone.screenshot({ path: path.join(OUT, '12-phone-my-page.png') });

    const nb = await open(await context({ viewport: { width: 1000, height: 800 } }, 'nb'));
    await go(nb, planPath);
    await fromTop(nb, '13-norwegian', nb.locator('main section').nth(1));

    if (errors.length) throw new Error('the page failed: ' + errors.join('; '));
    // Smaller files: a text page has few colours. pngquant is optional (the pictures are kept as they are without it).
    if (spawnSync('pngquant', ['--version']).status === 0) {
      for (const f of fs.readdirSync(OUT).filter((n) => n.endsWith('.png'))) {
        spawnSync('pngquant', ['--quality', '65-95', '--strip', '--force', '--ext', '.png', path.join(OUT, f)]);
      }
    }
    console.log('shots: ok, ' + fs.readdirSync(OUT).filter((f) => f.endsWith('.png')).length + ' pictures in ' + OUT);
  } finally {
    if (browserNb) await browserNb.close();
    if (browser) await browser.close();
    srv.kill('SIGTERM');
  }
})().catch((e) => { console.error(e.message); process.exit(1); });
