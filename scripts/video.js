// The short film on the start page (web/demo-<lang>.mp4): the real app on invented data, a plan made from nothing to a
// draft, with a pointer and a caption for each step. Not a test (app:ui is that). Needs what app:shots needs, plus ffmpeg
// (the mise task brings it). Run: mise run app:video [en|nb]   (both languages without an argument). Run it again when
// the page changes visibly in what the film shows: adding events, places, answers, the draft, a place set by hand.
const fs = require('fs'), os = require('os'), path = require('path'), vm = require('vm');
const { chromium } = require(process.env.PLAYWRIGHT_CORE || path.join(__dirname, '../../scoreboard/node_modules/playwright-core'));
const { spawn, spawnSync } = require('child_process');

let failed = null; // the page, for a picture of where it stopped
const SPEED = Number(process.env.SPEED || 1.4); // played faster than it was done: the clicks in real time feel slow
const W = 1024, H = 576; // about the size it is shown at on the start page, so the text stays readable
const chromeDir = () => {
  const root = path.join(os.homedir(), '.cache/ms-playwright');
  const d = fs.readdirSync(root).filter((n) => /^chromium-\d+$/.test(n)).sort().pop();
  return path.join(root, d, 'chrome-linux64', 'chrome');
};
// the page's own texts, to find buttons by what they say in either language
const sandbox = { navigator: { languages: ['en'] }, localStorage: { getItem: () => null, setItem() {} }, document: { documentElement: {} } };
vm.runInNewContext(fs.readFileSync(path.join(__dirname, '../web/i18n.js'), 'utf8') + '\nthis.M = MESSAGES;', sandbox);

// Every name is invented (the same people as in the README pictures).
const PEOPLE = ['Kari Test', 'Ola Sample', 'Mona Example', 'Pia Demo', 'Ingrid Sample', 'Njål Example', 'Sunniva Test', 'Jon Demo'];
const DATA = {
  en: { org: 'Maple Hall', plan: 'December 2015', tasks: ['Sound', 'Video', 'Doors', 'Coffee'] },
  nb: { org: 'Lønnehuset', plan: 'Desember 2015', tasks: ['Lyd', 'Video', 'Dør', 'Kaffe'] },
};
// what each person takes part in, by task number: 0 Sound, 1 Video, 2 Doors, 3 Coffee
const CHOICES = [[0, 1], [0, 2], [2, 3], [1, 0], [1, 3], [2, 0], [2, 1, 3], [0, 3]];
const SAY = {
  en: {
    start: 'A new plan: the tasks and the people are there, the dates are not',
    events: 'Add the dates: one concert, every week',
    places: 'Type how many each task needs',
    fill: 'Or the same number on every date at once',
    answers: 'Everyone says when they can, on their own phone',
    click: 'Or click for them: can, if needed, can’t',
    draft: 'Make a draft: a fair proposal in seconds',
    hand: 'Set someone by hand',
    again: 'Try again: the next draft keeps what you set',
    end: 'Dutyplan: free and open source',
  },
  nb: {
    start: 'En ny plan: oppgavene og folkene finnes, datoene ikke',
    events: 'Legg inn datoene: én konsert, hver uke',
    places: 'Skriv hvor mange hver oppgave trenger',
    fill: 'Eller samme tall på alle datoene på en gang',
    answers: 'Alle sier når de kan, på sin egen telefon',
    click: 'Eller klikk for dem: kan, ved behov, kan ikke',
    draft: 'Lag et utkast: et rettferdig forslag på sekunder',
    hand: 'Sett noen for hånd',
    again: 'Prøv igjen: neste utkast beholder det du har satt',
    end: 'Tjenesteplan: gratis og åpen kildekode',
  },
};

// A pointer and a caption drawn into the page (a recorded page has no mouse pointer). Both are popovers, so they are in
// the top layer above the page; when a dialog opens they are shown again to stay above it.
function overlay() {
  const css = (e, s) => { e.style.cssText = s; return e; };
  const make = () => {
    const pointer = css(document.createElement('div'), 'position:fixed;inset:auto;left:0;top:0;margin:0;padding:0;border:0;background:none;overflow:visible;pointer-events:none;width:26px;height:26px;transition:transform .08s');
    pointer.popover = 'manual';
    pointer.innerHTML = '<svg width="26" height="26" viewBox="0 0 26 26"><path d="M3 2 L3 21 L8.5 16 L12 24 L15.5 22.5 L12 15 L19.5 15 Z" fill="#111" stroke="#fff" stroke-width="1.6" stroke-linejoin="round"/></svg>';
    const ring = css(document.createElement('div'), 'position:absolute;left:-14px;top:-14px;width:28px;height:28px;border-radius:50%;background:rgba(31,111,74,.35);transform:scale(0);transition:transform .25s,opacity .35s;opacity:0');
    pointer.prepend(ring);
    const caption = css(document.createElement('div'), 'position:fixed;inset:auto 0 28px 0;margin:0 auto;width:max-content;max-width:90vw;padding:.6rem 1.2rem;border:0;border-radius:999px;background:rgba(20,28,24,.88);color:#fff;font:600 22px/1.3 system-ui,sans-serif;pointer-events:none;transition:opacity .3s;opacity:0');
    caption.popover = 'manual';
    document.body.append(pointer, caption);
    pointer.showPopover();
    caption.showPopover();
    let top = null;
    const raise = () => {
      const d = [...document.querySelectorAll('dialog[open]')].pop() || null;
      if (d === top) return;
      top = d;
      for (const e of [caption, pointer]) { e.hidePopover(); e.showPopover(); }
    };
    document.addEventListener('mousemove', (ev) => { pointer.style.transform = `translate(${ev.clientX - 3}px, ${ev.clientY - 2}px)`; raise(); }, true);
    document.addEventListener('mousedown', () => { raise(); ring.style.transition = 'none'; ring.style.transform = 'scale(.3)'; ring.style.opacity = '1'; ring.getBoundingClientRect(); ring.style.transition = ''; ring.style.transform = 'scale(1.6)'; ring.style.opacity = '0'; }, true);
    new MutationObserver(raise).observe(document.body, { subtree: true, attributes: true, attributeFilter: ['open'], childList: true });
    window.__say = (text) => { raise(); caption.style.opacity = text ? '1' : '0'; if (text) caption.textContent = text; };
  };
  if (document.body) make(); else document.addEventListener('DOMContentLoaded', make);
}

async function film(lang, out, base, srv) {
  const L = sandbox.M[lang], D = DATA[lang], say = SAY[lang];
  const api = async (method, p, body) => {
    const r = await fetch(base + '/api' + p, { method, headers: { 'Content-Type': 'application/json' }, body: body === undefined ? undefined : JSON.stringify(body) });
    const text = await r.text();
    if (!r.ok) throw new Error(`${method} ${p}: ${r.status} ${text}`);
    return text ? JSON.parse(text) : null;
  };
  // ---- the data before the film: an organisation with tasks and people, and an empty plan ----
  const org = await api('POST', '/orgs', { name: D.org });
  const o = `/orgs/${org.slug}`;
  for (const name of D.tasks) await api('POST', `${o}/tasks`, { name });
  const task = (await api('GET', `${o}/tasks`)).map((t) => t.id);
  await api('PUT', `${o}/combinations`, { a: task[0], b: task[2], level: 'cannot' });
  await api('PUT', `${o}/combinations`, { a: task[1], b: task[2], level: 'cannot' });
  for (const name of PEOPLE) await api('POST', `${o}/people`, { name });
  const person = Object.fromEntries((await api('GET', `${o}/people`)).map((x) => [x.name, x.id]));
  for (const [i, tasks] of CHOICES.entries()) {
    for (const [k, t] of tasks.entries()) await api('PUT', `${o}/people/${person[PEOPLE[i]]}/tasks/${task[t]}`, { level: k === 0 ? 'good' : 'poor' });
  }
  const plan = await api('POST', `${o}/plans`, { name: D.plan });
  const p = `${o}/plans/${plan.slug}`;

  // ---- the film ----
  const browser = await chromium.launch({ executablePath: process.env.CHROME || chromeDir(), args: ['--no-sandbox', lang === 'nb' ? '--lang=nb' : '--lang=en-GB'] });
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'dp-video-'));
  const ctx = await browser.newContext({
    viewport: { width: W, height: H }, deviceScaleFactor: 1, locale: lang === 'nb' ? 'nb-NO' : 'en-GB', timezoneId: 'UTC', bypassCSP: true,
    colorScheme: 'light',
  });
  await ctx.addInitScript((l) => { try { localStorage.setItem('tp.lang', l); } catch (e) { /* ignore */ } }, lang);
  await ctx.addInitScript(overlay);
  const page = await ctx.newPage();
  failed = page;
  const t0 = Date.now();
  const at = () => (Date.now() - t0) / 1000;
  const errors = [];
  page.on('pageerror', (e) => errors.push(e.message));
  const wait = (ms) => page.waitForTimeout(ms);
  const caption = async (text, ms = 1400) => { await page.evaluate((x) => window.__say(x), text); await wait(ms * 1.2); };
  let pos = { x: W / 2, y: H / 2 };
  const glide = async (x, y, ms = 550) => {
    const from = pos, n = Math.max(8, Math.round(ms / 16));
    for (let i = 1; i <= n; i++) {
      const k = i / n, e = k < 0.5 ? 2 * k * k : 1 - Math.pow(-2 * k + 2, 2) / 2;
      await page.mouse.move(from.x + (x - from.x) * e, from.y + (y - from.y) * e);
      await wait(16);
    }
    pos = { x, y };
  };
  // to an element (scrolled into view first, smoothly), and a click on it
  const point = async (loc, ms) => {
    await loc.waitFor();
    // in view and not under the table's sticky header or the caption: what is at its middle is the element itself
    const seen = (e) => { const r = e.getBoundingClientRect(); const at = document.elementFromPoint(r.left + Math.min(r.width / 2, 40), r.top + r.height / 2); return r.bottom <= innerHeight - 90 && !!at && e.contains(at); };
    if (!(await loc.evaluate(seen))) {
      await loc.evaluate((e) => e.scrollIntoView({ block: 'center', behavior: 'smooth' }));
      await wait(700);
    }
    const b = await loc.boundingBox();
    await glide(b.x + Math.min(b.width / 2, 40), b.y + b.height / 2, ms);
  };
  const click = async (loc, ms) => { await point(loc, ms); await wait(120); await page.mouse.down(); await wait(60); await page.mouse.up(); await wait(250); };
  const dialog = page.locator('dialog.modal');
  const closeDialog = async () => { await click(dialog.getByRole('button', { name: L['common.close'], exact: true }).last()); await dialog.waitFor({ state: 'detached' }); await wait(300); };
  const keep = []; // the parts of the recording that make the film (the planner's wait is cut out)
  let from = 0;

  await page.goto(`${base}/o/${org.slug}/p/${plan.slug}`);
  await page.waitForSelector('.dp-table');
  await page.waitForLoadState('networkidle');
  await page.mouse.move(pos.x, pos.y);
  // the recording: a frame from Chromium each time the screen changes, with its time
  const frames = [];
  const cdp = await ctx.newCDPSession(page);
  cdp.on('Page.screencastFrame', (f) => {
    frames.push({ t: f.metadata.timestamp - t0 / 1000, data: f.data });
    cdp.send('Page.screencastFrameAck', { sessionId: f.sessionId }).catch(() => {});
  });
  await cdp.send('Page.startScreencast', { format: 'jpeg', quality: 92, maxWidth: W, maxHeight: H });
  await wait(300);
  from = at();
  await caption(say.start, 1800);

  // the dates: one event, every week, four in all
  await caption(say.events, 200);
  await click(page.getByRole('button', { name: L['table.add_event'] }));
  await dialog.waitFor();
  await wait(400);
  const field = (k) => dialog.getByLabel(L[k], { exact: true });
  await click(field('event.date'));
  await field('event.date').fill('2015-12-05');
  await wait(300);
  await click(field('event.starts'));
  await field('event.starts').fill('19:00');
  await wait(300);
  await click(dialog.locator(`select[aria-label="${L['copy.repeat']}"]`));
  await dialog.locator(`select[aria-label="${L['copy.repeat']}"]`).selectOption({ label: L['copy.every_1'] });
  await wait(400);
  await click(field('copy.until'));
  await field('copy.until').fill('2015-12-26');
  await field('copy.until').evaluate((e) => e.blur());
  await wait(700);
  await click(dialog.getByRole('button', { name: L['common.add'], exact: true }));
  await page.waitForSelector('.dp-table thead th:nth-child(5)');
  await wait(600);

  // places: typed in, Enter goes on to the next date; then Fill row for a whole task
  await caption(say.places, 200);
  const row = (name) => page.locator('tr.task-row', { has: page.locator('th', { hasText: name }) });
  await click(row(D.tasks[0]).locator('input[type=number]').first());
  for (let i = 0; i < 4; i++) { await page.keyboard.type('1', { delay: 120 }); await wait(150); await page.keyboard.press('Enter'); await wait(200); }
  await click(row(D.tasks[2]).locator('input[type=number]').first());
  for (const n of ['2', '2', '2', '3']) { await page.keyboard.type(n, { delay: 120 }); await wait(150); await page.keyboard.press('Enter'); await wait(200); }
  await page.keyboard.press('Escape');
  await caption(say.fill, 200);
  for (const t of [D.tasks[1]]) {
    await click(page.getByRole('button', { name: L['table.task_aria'].replace('{name}', t), exact: true }));
    await dialog.waitFor();
    const fill = dialog.getByLabel(L['task.fill_aria'].replace('{name}', t), { exact: true });
    await click(fill);
    await page.keyboard.type('1', { delay: 120 });
    await wait(300);
    await click(dialog.getByRole('button', { name: L['task.fill'], exact: true }));
    await wait(400);
    await closeDialog();
  }
  await wait(300);

  // answers: the others answer from their phones (through the API, as their page would), and show up at once
  const events = await api('GET', `${p}/events`);
  await page.locator('tr.person-row').first().evaluate((e) => e.scrollIntoView({ block: 'center', behavior: 'smooth' }));
  await wait(600);
  await caption(say.answers, 300);
  const LEVELS = ['good', 'good', 'poor', 'never', 'good', '', 'good', 'never', 'good', 'poor', 'good', 'good'];
  for (const [i, name] of PEOPLE.slice(1).entries()) {
    const levels = Object.fromEntries(events.map((ev, j) => [ev.id, LEVELS[(i * 5 + j * 3) % LEVELS.length]]));
    await api('PUT', `${p}/people/${person[name]}/availability`, { levels });
    await wait(260);
  }
  await wait(500);
  await caption(say.click, 200);
  const answer = (name, j) => page.locator(`tr.person-row[data-person="${person[name]}"] button.ans`).nth(j);
  for (const [j, clicks] of [[0, 1], [2, 3], [3, 2]]) {
    for (let c = 0; c < clicks; c++) { await click(answer(PEOPLE[0], j), c ? 150 : 400); await wait(150); }
  }
  await wait(500);

  // the draft
  await page.evaluate(() => window.scrollTo({ top: 0, behavior: 'smooth' }));
  await wait(600);
  await caption(say.draft, 200);
  await click(page.getByRole('button', { name: L['run.plan'], exact: true }));
  await wait(600);
  keep.push([from, at()]);
  await page.waitForSelector('.draft-bar .run-id', { timeout: 60000 });
  await page.waitForSelector('tr.draft-row button.draftbtn .slot:not(.open)');
  from = at() - 0.2;
  await page.locator('tr.draft-row').first().evaluate((e) => e.scrollIntoView({ block: 'center', behavior: 'smooth' }));
  await wait(1600);

  // a place by hand: one more on the doors on the last date, then a new draft that keeps it
  await caption(say.hand, 300);
  const cell = page.locator('tr.draft-row', { has: page.locator('th', { hasText: D.tasks[0] }) }).locator('button.draftbtn').nth(3);
  await click(cell);
  await dialog.waitFor();
  await wait(700);
  const box = dialog.locator('input[type=checkbox]').first();
  await click(box);
  await box.check();
  await wait(400);
  await click(dialog.getByRole('button', { name: L['place.add_people'], exact: true }));
  await wait(600);
  await closeDialog();
  await wait(600);
  await caption(say.again, 200);
  await click(page.getByRole('button', { name: L['run.retry'], exact: true }));
  await wait(500);
  keep.push([from, at()]);
  await page.waitForSelector('.draft-bar .run-id >> text=#2', { timeout: 60000 });
  from = at() - 0.2;
  await wait(2200);
  await glide(W - 60, H - 140, 500);
  await caption(say.end, 2400);
  keep.push([from, at()]);

  await cdp.send('Page.stopScreencast');
  await browser.close();
  if (errors.length) throw new Error('the page failed: ' + errors.join('; '));

  // ---- the file: the kept parts after each other, H.264 for every browser, small ----
  // Each frame stands until the next one; a part starts with the frame on screen at its start.
  const list = [];
  let n = 0;
  for (const [a, b] of keep) {
    const inside = frames.filter((f) => f.t > a && f.t < b);
    const before = frames.filter((f) => f.t <= a).pop();
    const shown = (before ? [{ ...before, t: a }] : []).concat(inside);
    shown.forEach((f, i) => {
      const file = path.join(dir, `f${String(n++).padStart(5, '0')}.jpg`);
      fs.writeFileSync(file, Buffer.from(f.data, 'base64'));
      list.push(`file '${file}'`, `duration ${(((i + 1 < shown.length ? shown[i + 1].t : b) - f.t) / SPEED).toFixed(3)}`);
    });
  }
  list.push(list[list.length - 2]); // the concat format needs the last file once more
  fs.writeFileSync(path.join(dir, 'list.txt'), list.join('\n') + '\n');
  const ff = spawnSync(process.env.FFMPEG || 'ffmpeg', ['-y', '-loglevel', 'error', '-f', 'concat', '-safe', '0', '-i', path.join(dir, 'list.txt'),
    '-vf', `fps=20,scale=${W}:${H}`, '-c:v', 'libx264', '-preset', 'veryslow', '-tune', 'animation', '-crf', process.env.CRF || '32',
    '-pix_fmt', 'yuv420p', '-movflags', '+faststart', '-an', out], { stdio: 'inherit' });
  if (ff.status !== 0) throw new Error('ffmpeg failed');
  fs.rmSync(dir, { recursive: true, force: true });
  return { seconds: keep.reduce((s, [a, b]) => s + b - a, 0) / SPEED, bytes: fs.statSync(out).size };
}

(async () => {
  if (!process.env.DUTYPLAN_SOLVER) throw new Error('DUTYPLAN_SOLVER is not set (run via mise run app:video)');
  const langs = process.argv.slice(2).length ? process.argv.slice(2) : ['en', 'nb'];
  for (const lang of langs) {
    if (!DATA[lang]) throw new Error('no such language: ' + lang);
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'dp-video-db-'));
    const srv = spawn(path.join(__dirname, '../bin/dutyplan'), ['--addr', '127.0.0.1:0', '--db', path.join(dir, 'video.db')], { stdio: ['ignore', 'pipe', 'pipe'] });
    try {
      let base = '';
      await new Promise((res, rej) => {
        srv.stdout.on('data', (d) => { const m = /http:\/\/\S+/.exec(String(d)); if (m && !base) { base = m[0]; res(); } });
        setTimeout(() => rej(new Error('server start timeout')), 5000);
      });
      const out = path.join(process.env.OUT || path.join(__dirname, '../web'), `demo-${lang}.mp4`);
      const r = await film(lang, out, base, srv);
      console.log(`video: ${out}, ${r.seconds.toFixed(1)} s, ${Math.round(r.bytes / 1024)} KB`);
    } finally {
      srv.kill('SIGTERM');
      fs.rmSync(dir, { recursive: true, force: true });
    }
  }
})().catch(async (e) => {
  console.error('video: ' + e.message);
  if (failed) await failed.screenshot({ path: path.join(os.tmpdir(), 'dp-video-failed.png') }).then(() => console.error('video: the page then: ' + path.join(os.tmpdir(), 'dp-video-failed.png'))).catch(() => {});
  process.exit(1);
});
