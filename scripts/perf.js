// How quick the plan page feels, measured on a big invented plan: the time from a click (or a typed value) until the page
// shows it, until everything has settled (no request out, no change to the page for QUIET ms), the requests it took and the
// time the page's main thread was blocked (long tasks). The server is local; every API call is delayed by DELAY ms (default
// 80) to stand in for the trip to the real server. Not a test (nothing fails); it prints a table to compare before and after.
// Run: mise run app:perf   (PEOPLE=200 EVENTS=60 for the stress case, DELAY=0 without the delay, RUNS=5 repeats per action)
// Needs playwright-core and a Chromium like app:ui, and mise run setup for the planner (a draft is on show).
const fs = require('fs'), os = require('os'), path = require('path');
const { chromium } = require(process.env.PLAYWRIGHT_CORE || path.join(__dirname, '../../scoreboard/node_modules/playwright-core'));
const { spawn } = require('child_process');

const QUIET = 1500;  // ms without a request or a change before an action counts as settled (longer than redrawSoon)
const N_PEOPLE = +(process.env.PEOPLE || 50), N_EVENTS = +(process.env.EVENTS || 40), DELAY = +(process.env.DELAY ?? 80), RUNS = +(process.env.RUNS || 5);
const TASKS = ['Sound', 'Video', 'Doors', 'Coffee', 'Lights', 'Stream'];
const SLOTS = { Sound: 1, Video: 1, Doors: 2, Coffee: 1, Lights: 1, Stream: 1 };
const people = Array.from({ length: N_PEOPLE }, (_, i) => 'Person ' + String(i + 1).padStart(3, '0'));  // invented: no names at all
let seed = 7;
const rnd = () => (seed = (seed * 1103515245 + 12345) % 2147483648) / 2147483648;  // the same data every run
const chromeDir = () => {
  const root = path.join(os.homedir(), '.cache/ms-playwright');
  const d = fs.readdirSync(root).filter((n) => /^chromium-\d+$/.test(n)).sort().pop();
  return path.join(root, d, 'chrome-linux64', 'chrome');
};
let step = '';
const addDays = (d, n) => new Date(Date.parse(d + 'T00:00:00Z') + n * 864e5).toISOString().slice(0, 10);

(async () => {
  if (!process.env.DUTYPLAN_SOLVER) throw new Error('DUTYPLAN_SOLVER is not set (run via mise run app:perf)');
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'dp-perf-'));
  const srv = spawn(process.env.BIN || path.join(__dirname, '../bin/dutyplan'),  // BIN: another build, to compare
    ['--addr', '127.0.0.1:0', '--db', path.join(dir, 'perf.db')], { stdio: ['ignore', 'pipe', 'pipe'] });
  let base = '';
  await new Promise((res, rej) => {
    srv.stdout.on('data', (d) => { const m = /http:\/\/\S+/.exec(String(d)); if (m && !base) { base = m[0]; res(); } });
    setTimeout(() => rej(new Error('server start timeout')), 5000);
  });
  const o = { org: 'perf-hall', plan: 'big-plan' };
  const api = async (proc, body = {}) => {
    const r = await fetch(`${base}/dutyplan.v1.${proc}`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
    const text = await r.text();
    if (!r.ok) throw new Error(`${proc}: ${r.status} ${text}`);
    return JSON.parse(text);
  };
  let browser;
  try {
    // ---- the plan ----
    await api('AccountService/CreateOrg', { name: 'Perf Hall' });
    await api('PlanService/CreatePlan', { org: o.org, name: 'Big plan' });
    for (const name of TASKS) await api('SetupService/CreateTask', { ...o, name });
    const task = Object.fromEntries((await api('SetupService/ListTasks', o)).tasks.map((t) => [t.name, t.id]));
    for (const name of people) await api('SetupService/CreatePerson', { ...o, name });
    const person = (await api('SetupService/ListPeople', o)).people.map((p) => p.id);
    for (const [i, p] of person.entries()) {
      for (const k of [i % TASKS.length, (i * 7 + 3) % TASKS.length]) await api('PlanService/SetPersonTask', { ...o, person: p, task: task[TASKS[k]], level: rnd() < 0.8 ? 'good' : 'poor' }).catch(() => {});
    }
    const start = '2026-01-04';
    const { events } = await api('PlanService/AddEvents', { ...o, date: start, until: addDays(start, 7 * (N_EVENTS - 1)) });
    const places = [];
    for (const e of events) for (const t of TASKS) places.push({ event: e.id, task: task[t], slots: SLOTS[t] });
    await api('PlanService/SetPlacesMany', { ...o, places });
    const answers = [];
    for (const p of person) for (const e of events) if (rnd() < 0.7) answers.push({ person: p, event: e.id, level: ['good', 'good', 'poor', 'never'][Math.floor(rnd() * 4)] });
    for (let i = 0; i < answers.length; i += 5000) await api('PlanService/SetAnswers', { ...o, answers: answers.slice(i, i + 5000) });
    const { field } = await api('PlanService/AddField', { ...o, name: 'Note' });
    for (const e of events.slice(0, 10)) await api('PlanService/SetFieldValue', { ...o, event: e.id, field: field.id, value: 'Theme ' + e.id });
    await api('DraftService/StartRun', { ...o, time_limit: 10 });
    for (let i = 0; ; i++) {
      const { runs } = await api('DraftService/ListRuns', o);
      if (runs[0] && runs[0].status !== 'running' && runs[0].status !== 'queued') { if (runs[0].status !== 'done') throw new Error('the draft: ' + runs[0].status + ' ' + runs[0].error); break; }
      if (i > 120) throw new Error('the draft never finished');
      await new Promise((r) => setTimeout(r, 1000));
    }

    // ---- the server: each read the plan page makes, timed here (JSON, no delay; the page reads binary) ----
    if (process.env.SERVER !== '0') {
      const { runs } = await api('DraftService/ListRuns', o);
      const reads = [['PlanService/GetPlan', o], ['PlanService/ListEvents', o], ['SetupService/ListTasks', o], ['SetupService/ListPeople', o], ['SetupService/ListCombinations', o],
        ['PlanService/ListFields', o], ['PlanService/ListAnswers', o], ['PlanService/ListPlaces', o], ['DraftService/ListCells', o], ['DraftService/ListPools', o],
        ['DraftService/GetWeights', o], ['DraftService/ListRuns', o], ['SetupService/ListSections', o], ['DraftService/GetRun', { ...o, id: runs[0].id }]];
      const server = [];
      for (const [proc, body] of reads) {
        const ms = [];
        for (let k = 0; k < 5; k++) { const t = performance.now(); const r = await fetch(`${base}/dutyplan.v1.${proc}`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) }); const b = await r.arrayBuffer(); ms.push([performance.now() - t, b.byteLength]); }
        ms.sort((x, y) => x[0] - y[0]);
        server.push({ read: proc.split('/')[1], ms: +ms[2][0].toFixed(1), kB: +(ms[2][1] / 1024).toFixed(1) });
      }
      console.table(server);
    }

    // ---- the page ----
    browser = await chromium.launch({ executablePath: process.env.CHROME || chromeDir(), args: ['--no-sandbox'] });
    const ctx = await browser.newContext({ viewport: { width: 1600, height: 1000 }, locale: 'en-GB', timezoneId: 'UTC' });
    await ctx.addInitScript(() => {
      // the time of the last change to the page, and the time the main thread was blocked
      window.__perf = { lastMutation: 0, blocked: 0 };
      // shown: the first change to the page after which __cond (the action's check) holds
      new MutationObserver(() => {
        const t = performance.now();
        window.__perf.lastMutation = t;
        if (!window.__perf.tableAt && document.querySelector('.dp-table button.ans')) window.__perf.tableAt = t;  // a page load: the table is there
        if (window.__cond && window.__perf.shownAt == null) { try { if (window.__cond(window.__arg)) window.__perf.shownAt = t; } catch (e) { /* not yet */ } }
      }).observe(document, { subtree: true, childList: true, attributes: true, characterData: true });
      new PerformanceObserver((l) => { for (const e of l.getEntries()) window.__perf.blocked += e.duration; }).observe({ type: 'longtask', buffered: false });
    });
    const page = await ctx.newPage();
    const errors = [];
    page.on('pageerror', (e) => errors.push(e.message));
    page.on('console', (m) => { if (m.text().startsWith('RENDER')) console.log(step + ': ' + m.text()); });
    let out = 0, lastResponse = 0, requests = 0;
    const now = () => page.evaluate(() => performance.now());
    await page.route('**/dutyplan.v1.**', async (route) => { if (DELAY) await new Promise((r) => setTimeout(r, DELAY)); route.continue(); });
    page.on('request', (r) => { if (r.url().includes('/dutyplan.v1.')) { out++; requests++; } });
    const done = async (r) => { if (r.url().includes('/dutyplan.v1.')) { out--; lastResponse = await now().catch(() => 0); } };
    page.on('requestfinished', done);
    page.on('requestfailed', done);
    // settled: nothing out, and no change to the page for QUIET ms
    const settled = async () => {
      for (;;) {
        await page.waitForTimeout(50);
        const p = await page.evaluate(() => [performance.now(), window.__perf.lastMutation]);
        if (out === 0 && p[0] - Math.max(p[1], lastResponse) > QUIET) return Math.max(p[1], lastResponse);
      }
    };
    const results = [];
    const metrics = await page.context().newCDPSession(page);
    await metrics.send('Performance.enable');
    const m = async () => Object.fromEntries((await metrics.send('Performance.getMetrics')).metrics.map((x) => [x.name, x.value]));
    // measure: act() starts the action in the page and returns its start (performance.now()); shown is a function run in the
    // page that is true once the page shows the result.
    const measure = async (name, act, shown, arg) => {
      step = name;
      const rows = [];
      const profile = process.env.PROFILE && name.includes(process.env.PROFILE);
      const cdp = profile ? await page.context().newCDPSession(page) : null;
      if (cdp) { await cdp.send('Profiler.enable'); await cdp.send('Profiler.setSamplingInterval', { interval: 200 }); }
      for (let i = 0; i < RUNS; i++) {
        await settled();
        if (cdp && i === RUNS - 1) await cdp.send('Profiler.start');
        await metrics.send('HeapProfiler.collectGarbage');  // (before the count starts: it blocks too)
        const m0 = await m();
        requests = 0;
        await page.evaluate(([cond, a]) => { window.__perf.blocked = 0; window.__perf.shownAt = null; window.__cond = cond ? eval('(' + cond + ')') : null; window.__arg = a; },
          [shown && shown !== 'load' ? shown.toString() : null, arg ? arg(i) : null]);
        const t0 = await act(i);
        const at = shown === 'load' ? 'tableAt' : 'shownAt';
        if (shown) await page.waitForFunction((k) => window.__perf[k] != null, at, { timeout: 10000 });
        const vis = shown ? await page.evaluate((k) => window.__perf[k], at) : null;
        const end = await settled();
        if (cdp && i === RUNS - 1) {
          const { profile: pr } = await cdp.send('Profiler.stop');
          const self = new Map(), dt = pr.timeDeltas || [];
          const byId = new Map(pr.nodes.map((n) => [n.id, n]));
          pr.samples.forEach((id, k) => { const n = byId.get(id); const f = n.callFrame; const key = (f.functionName || '(anon)') + ' ' + f.url.split('/').pop().split('?')[0] + ':' + (f.lineNumber + 1) + ':' + (f.columnNumber + 1); self.set(key, (self.get(key) || 0) + (dt[k] || 0) / 1000); });
          // inclusive: the time in a function and in what it calls (once per sample, also when it recurses)
          const parent = new Map(); for (const n of pr.nodes) for (const c of n.children || []) parent.set(c, n.id);
          const incl = new Map();
          pr.samples.forEach((id, k) => { const seen = new Set(); for (let x = id; x; x = parent.get(x)) { const f = byId.get(x).callFrame; if (!f.url.includes('app.js')) continue; const key = (f.functionName || '(anon)') + ':' + (f.lineNumber + 1); if (seen.has(key)) continue; seen.add(key); incl.set(key, (incl.get(key) || 0) + (dt[k] || 0) / 1000); } });
          console.log('inclusive (app.js):\n' + [...incl].sort((a, b) => b[1] - a[1]).slice(0, 30).map(([k, v]) => '  ' + v.toFixed(1).padStart(7) + ' ms  ' + k).join('\n'));
          console.log('profile of ' + name + ':\n' + [...self].sort((a, b) => b[1] - a[1]).slice(0, 25).map(([k, v]) => '  ' + v.toFixed(1).padStart(7) + ' ms  ' + k).join('\n'));
        }
        const blocked = await page.evaluate(() => window.__perf.blocked);  // (before the collection below, which blocks too)
        await metrics.send('HeapProfiler.collectGarbage');
        const m1 = await m();
        const d = (k) => (m1[k] - (name === 'open the plan' ? 0 : m0[k])) * 1000;
        rows.push({ shown: vis == null ? null : vis - t0, settled: end - t0, requests, blocked,
          script: d('ScriptDuration'), style: d('RecalcStyleDuration'), layout: d('LayoutDuration'), elements: m1.Nodes });
      }
      const med = (k) => { const v = rows.map((r) => r[k]).filter((x) => x != null).sort((a, b) => a - b); return v.length ? Math.round(v[Math.floor(v.length / 2)]) : '–'; };
      results.push({ action: name, shown: med('shown'), settled: med('settled'), requests: med('requests'), blocked: med('blocked'), script: med('script'), style: med('style'), layout: med('layout'), nodes: med('elements') });
    };
    const url = `${base}/o/${o.org}/p/${o.plan}`;

    await measure('open the plan', async () => { await page.goto(url, { waitUntil: 'commit' }); return 0; },  // performance.now() starts at the navigation
      'load');
    // an answer: the k-th person's cell on the k-th event
    await measure('click an answer', (i) => page.evaluate((i) => { window.__at = i * 41 + 3; const b = document.querySelectorAll('.dp-table button.ans')[window.__at]; window.__was = b.dataset.level; const t = performance.now(); b.click(); return t; }, i),
      () => { const b = document.querySelectorAll('.dp-table button.ans')[window.__at]; return b && b.dataset.level !== window.__was && performance.now(); });  // (the label says the level, so by position)
    await measure('type places', (i) => page.evaluate((i) => { const x = document.querySelectorAll('.dp-table input[aria-label^="Places for Sound on"]')[i + 1]; const t = performance.now(); x.value = String(2 + (i % 2)); x.dispatchEvent(new Event('input', { bubbles: true })); x.dispatchEvent(new Event('change', { bubbles: true })); return t; }, i));
    await measure('type a details value', (i) => page.evaluate((i) => { const x = document.querySelectorAll('.dp-table tr.field-row td.text input')[i + 12]; const t = performance.now(); x.value = 'New ' + i; x.dispatchEvent(new Event('input', { bubbles: true })); x.dispatchEvent(new Event('change', { bubbles: true })); return t; }, i));
    await measure('lock a draft name', (i) => page.evaluate((i) => { window.__at = i * 3; const b = document.querySelectorAll('.dp-table button.lockbtn:not(.group)')[window.__at]; window.__was = b.getAttribute('aria-pressed'); const t = performance.now(); b.click(); return t; }, i),
      () => { const b = document.querySelectorAll('.dp-table button.lockbtn:not(.group)')[window.__at]; return b && b.getAttribute('aria-pressed') !== window.__was && performance.now(); });
    await measure('add a person', (i) => page.evaluate((i) => {
      const input = [...document.querySelectorAll('input[placeholder="Add a person"]')].pop();
      input.value = 'Extra ' + String(i).padStart(2, '0'); const t = performance.now(); input.form.requestSubmit(); return t;
    }, i), (name) => [...document.querySelectorAll('.dp-table th[scope=row]')].some((th) => th.textContent.includes(name)) && performance.now(), (i) => 'Extra ' + String(i).padStart(2, '0'));
    await measure('open a person', (i) => page.evaluate((i) => { const b = document.querySelectorAll('.dp-table tr.person-row th button.cellbtn')[i]; const t = performance.now(); b.click(); return t; }, i),
      () => document.querySelector('dialog.modal[open]') && performance.now());
    await page.keyboard.press('Escape');
    if (errors.length) console.log('page errors: ' + errors.join(' | '));
    console.log(`perf: ${N_PEOPLE} people × ${N_EVENTS} events × ${TASKS.length} tasks, ${answers.length} answers, delay ${DELAY} ms, median of ${RUNS} (ms)`);
    console.table(results);
  } finally {
    if (browser) await browser.close();
    srv.kill();
    fs.rmSync(dir, { recursive: true, force: true });
  }
})().catch((e) => { console.error('perf: ' + (step ? step + ': ' : '') + e.message); process.exit(1); });
