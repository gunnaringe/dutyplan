import { Account, call, changedBy, confirmed, Draft, errorText, forget, get, Plan, Setup, write } from './api';
import { LEVEL_ICON, h, icon, locale, mark, say, store, type Kid, type Page } from './core';
import { live, render } from './main';
import { ask, shareDialog, viewOnly } from './org';
import { ME, dropdown, go, link, linkBtn, me } from './shell';
import { addRow, closeBtn, enterMovesOn, field, foot, infoBtn, keyed, modal, reopened } from './ui';
import { MAX_EVERY, addDays, clock, enc, fmtDate, fmtShortDate, fmtTime, repeatDates } from './util';

// ---- the plan page: one table ----------------------------------------------------------------------------------------
// Events across. From the top: the event details (rows the plan defines), the tasks with the places each event needs, the
// draft allocation (who got what, and what is set by hand), and availability (everyone's answer per event, set with a click:
// blank → yes → maybe → no → blank; blank = not answered, which the planner counts as neutral). The names of events, rows,
// tasks and people open a dialog with the rest. Tasks, combinations and people belong to the organisation, not the plan.

export const ANSWER_NEXT = { '': 'good', good: 'poor', poor: 'never', never: '' };
export const answerMark = (l) => (l ? icon(LEVEL_ICON[l]) : '');
// One click moves a setting on, the same way as the answers. A person's task: not → bad → good (not = no row).
// Two tasks on the same event: can't → bad → good (bad = no row: a pair that is not set is impractical for the planner).
export const TASK_NEXT = { '': 'poor', poor: 'good', good: '' };
export const COMBO_NEXT = { cannot: 'standard', standard: 'practical', practical: 'cannot' };
// cycle: a button that shows a level as an icon (red thumb down, yellow ?, green thumb up) and moves it on with a click. kind
// 'task' or 'combo' picks the texts. The button shows the new level at once; onNext(next) saves it and says whether it went
// through (if not, the button goes back).
export function cycle(kind, level, aria, onNext) {
  const text = (l) => t(kind === 'task' ? ({ '': 'pick.not', poor: 'pick.poor', good: 'pick.good' })[l] : ({ cannot: 'combo.cannot', standard: 'combo.bad', practical: 'combo.good' })[l]);
  const b = h('button', { type: 'button', class: 'cyc' });
  const paint = (l) => {
    b.dataset.level = l || 'none';
    b.title = text(l);
    b.setAttribute('aria-label', `${aria}: ${text(l)}`);
    b.replaceChildren(icon(LEVEL_ICON[l]));
  };
  // Quick clicks are not dropped: each moves the shown level on at once; one save runs at a time, and when it is done the
  // level shown last is saved (the steps in between never are). A refused save goes back to what was saved.
  let shown = level, busy = false;
  b.addEventListener('click', async () => {
    shown = kind === 'task' ? TASK_NEXT[shown] : COMBO_NEXT[shown];
    paint(shown);
    if (busy) return;
    busy = true;
    while (shown !== level) {
      const want = shown;
      if (await onNext(want)) level = want; else { shown = level; paint(level); }
    }
    busy = false;
  });
  paint(level);
  return b;
}

// Weighting: what the planner puts weight on, for this plan. The numbers are points in the planner's sum; the defaults are a good
// starting point. Only what differs from the default is stored, so a changed default reaches plans that never chose anything.
export const WEIGHT_GROUP = {
  w_fair: 'fair', w_fair_range: 'fair', w_task_fair: 'fair', w_task_range: 'fair',
  'w_task.good': 'wishes', 'w_task.poor': 'wishes', 'w_date.good': 'wishes', 'w_date.poor': 'wishes',
  'w_combo.practical': 'combos', 'w_combo.impractical': 'combos',
};
export const weightLabel = (key) => t('weight.' + key + '.label');
export const weightGroup = (key) => t('weight.group.' + (WEIGHT_GROUP[key] || 'fair'));

export function weightsForm(at: { org: string; plan: string }, wr, saved) {
  const inputs = wr.specs.map((spec) => {
    const cur = spec.key in wr.values ? wr.values[spec.key] : spec.default;
    const input = h('input', { type: 'number', min: String(spec.min), max: String(spec.max), step: 'any', value: String(cur), 'aria-label': weightLabel(spec.key) });
    return { spec, input, changed: spec.key in wr.values };
  });
  const groups: string[] = [...new Set<string>(wr.specs.map((x) => weightGroup(x.key)))];
  const blocks = groups.map((g) => h('div', { class: 'weight-group' }, h('h4', {}, g),
    inputs.filter(({ spec }) => weightGroup(spec.key) === g).map(({ spec, input, changed }) => {
      const def = spec.default;
      return h('div', { class: 'weight-row' + (changed ? ' changed' : '') },
        h('label', {}, h('span', { class: 'wlabel' }, weightLabel(spec.key), changed ? h('span', { class: 'tag' }, t('weights.changed')) : ''), input),
        h('div', { class: 'small muted' }, t('weights.default', { value: def })));
    })));
  const save = (values, text) => async () => { if (await write(() => call(Draft.setWeights, { ...at, values }))) saved(text); };
  const nChanged = Object.keys(wr.values || {}).length;
  return h('details', {}, h('summary', {}, nChanged ? t('weights.show_changed', { n: nChanged }) : t('weights.show')),
    h('div', { class: 'row-inline' }, infoBtn(t('weights.title'), [t('weights.hint'), ...wr.specs.map((x) => `${weightLabel(x.key)}: ${t('weight.' + x.key + '.hint')}`)])),
    h('div', { class: 'stack' }, blocks,
      h('div', { class: 'row-inline' },
        h('button', { type: 'button', onclick: () => {
          const values = {};
          for (const { spec, input } of inputs) {
            if (input.value.trim() !== '') values[spec.key] = Number(input.value);
          }
          return save(values, t('weights.saved'))();
        } }, t('weights.save')),
        h('button', { type: 'button', class: 'ghost', onclick: save({}, t('weights.reset_done')) }, t('weights.reset')))));
}

// «What matters» (2026-11-23, the user: "Make the settings page for the plan have simple settings for what is important. Setting
// the numbers manually could be hidden as a advanced setting."): three choices, each Less / Normal / More, that set a group of
// weights to fixed values (Normal = the defaults); the numbers stay under «Advanced». Page only: the same PUT weights.
export const PRIORITIES = [
  { key: 'fair', levels: { less: { w_fair: 4, w_fair_range: 4 }, normal: { w_fair: 10, w_fair_range: 10 }, more: { w_fair: 25, w_fair_range: 25 } } },
  { key: 'wishes', levels: { less: { 'w_task.good': 1, 'w_task.poor': -2, 'w_date.good': 1, 'w_date.poor': -3 },
    normal: { 'w_task.good': 2, 'w_task.poor': -4, 'w_date.good': 3, 'w_date.poor': -6 },
    more: { 'w_task.good': 5, 'w_task.poor': -10, 'w_date.good': 8, 'w_date.poor': -15 } } },
  { key: 'combos', levels: { less: { 'w_combo.practical': 1, 'w_combo.impractical': -2 }, normal: { 'w_combo.practical': 2, 'w_combo.impractical': -5 },
    more: { 'w_combo.practical': 5, 'w_combo.impractical': -12 } } },
];
export const weightNow = (wr, k) => (k in (wr.values || {}) ? wr.values[k] : (wr.specs.find((x) => x.key === k) || {}).default);
// the level a priority is at now, or '' when its numbers were set by hand to something else
export const priorityLevel = (wr, p) => Object.keys(p.levels).find((l) => Object.entries(p.levels[l]).every(([k, v]) => weightNow(wr, k) === v)) || '';
export const prioritySummary = (wr) => PRIORITIES.map((p) => t('prio.' + p.key + '.short') + ': ' + t('prio.level.' + (priorityLevel(wr, p) || 'custom'))).join(' · ');

export function prioritiesForm(at: { org: string; plan: string }, wr, saved) {
  const rows = PRIORITIES.map((p) => {
    const now = priorityLevel(wr, p);
    return h('div', { class: 'prio' }, h('div', {}, h('div', { class: 'plabel' }, t('prio.' + p.key)), h('div', { class: 'small muted' }, t('prio.' + p.key + '.hint'))),
      h('div', { class: 'seg', role: 'group', 'aria-label': t('prio.' + p.key) }, ['less', 'normal', 'more'].map((l) => h('button', { type: 'button', 'aria-pressed': String(now === l),
        onclick: async () => {
          if (now === l) return;
          if (await write(() => call(Draft.setWeights, { ...at, values: { ...(wr.values || {}), ...p.levels[l] } }))) saved(t('prio.saved', { what: t('prio.' + p.key), level: t('prio.level.' + l) }));
        } }, t('prio.level.' + l)))),
      now ? '' : h('div', { class: 'small muted' }, t('prio.custom_hint')));
  });
  return h('div', { class: 'stack' }, rows);
}

// What the page keeps between redraws of the same plan: the draft picked under the earlier drafts.
// A place in a grid of the table: the row (a person, a task) and the event.
export type At = { row: number; ev: number };
// An undo step: what it undoes (for the button's title) and the opposite request(s) of the change.
export type Undo = { label: string; undo: () => Promise<unknown> };
// What a row of the table takes when a row is dropped on it (dropOn), and what a ⋮⋮ handle drags (handleFor).
export type Drop = { accept: (kind: string, item: any) => boolean; act: (item: any, after: boolean, kind?: string) => unknown; readonly into?: boolean };
declare global {
  interface HTMLElement { _drop?: Drop; _drag?: { kind: string; item: any } }
}
export const planState: {
  key: string;
  runId: number | null;
  aside: boolean;
  undo: Undo[];
  sel?: { keys: string[]; anchor: At } | null;  // a selection of the draft, kept over a redraw
  adds: Set<string>;  // the add lines that are open
  edit?: string | null;  // the draft cell being edited (its key)
  editInput?: HTMLInputElement;
  undoLast?: () => Promise<void>;
} = { key: '', runId: null, aside: false, undo: [], adds: new Set() };
// Ctrl+Z (⌘Z) outside a text field undoes the newest change in the draft (planState.undoLast, set by the plan page).
document.addEventListener('keydown', (e) => {
  if (!(e.ctrlKey || e.metaKey) || e.shiftKey || e.altKey || e.key.toLowerCase() !== 'z' || !planState.undoLast) return;
  if (!document.querySelector('.draft-bar')) return;  // only on a plan's table
  const target = e.target as Element;
  if (target.closest && target.closest('input, textarea, select, [contenteditable]')) return;
  e.preventDefault();
  planState.undoLast();
});

// Sideways through the events: ◀ ▶ move a screenful, «Next» goes to the first event from today. Over the dates the wheel scrolls
// sideways too. grid is the table's scroll box.
export function sideways(grid, events) {
  if (!grid) return '';
  const firstCol = () => (grid.querySelector('tbody th[scope=row]') || { offsetWidth: 0 }).offsetWidth;
  const by = (dir) => grid.scrollBy({ left: dir * Math.max(120, grid.clientWidth - firstCol() - 40), behavior: 'smooth' });
  const toNext = () => {
    const today = new Date().toISOString().slice(0, 10);
    const i = events.findIndex((ev) => ev.date >= today);
    const th = grid.querySelectorAll('thead th[scope=col]')[i < 0 ? events.length - 1 : i];
    if (th) grid.scrollTo({ left: th.offsetLeft - firstCol(), behavior: 'smooth' });
  };
  grid.addEventListener('wheel', (e) => {
    if (!e.target.closest('thead') || Math.abs(e.deltaX) > Math.abs(e.deltaY)) return;
    e.preventDefault();
    grid.scrollLeft += e.deltaY;
  });
  grid.addEventListener('keydown', (e) => {
    if (e.target !== grid) return;
    if (e.key === 'ArrowRight' || e.key === 'ArrowLeft') { e.preventDefault(); by(e.key === 'ArrowRight' ? 1 : -1); }
  });
  grid.tabIndex = 0;
  grid.setAttribute('aria-label', t('table.scroll_aria'));
  return h('span', { class: 'sideways' },
    h('button', { type: 'button', class: 'ghost icon', title: t('table.earlier'), 'aria-label': t('table.earlier'), onclick: () => by(-1) }, icon('left')),
    h('button', { type: 'button', class: 'ghost icon', title: t('table.next_aria'), 'aria-label': t('table.next_aria'), onclick: toNext }, icon('today')),
    h('button', { type: 'button', class: 'ghost icon', title: t('table.later'), 'aria-label': t('table.later'), onclick: () => by(1) }, icon('right')));
}

// With a mouse, the table can be dragged sideways from anywhere that is not a control or a cell that selects.
export function dragScroll(grid) {
  let start: { x: number; left: number } | null = null;
  let moved = false;
  grid.addEventListener('pointerdown', (e) => {
    if (e.pointerType !== 'mouse' || e.button !== 0 || e.target.closest('button, input, select, textarea, a, label, .grip, td[data-cell], th[scope=row]:has(.grip)')) return;
    start = { x: e.clientX, left: grid.scrollLeft };
    moved = false;
  });
  grid.addEventListener('pointermove', (e) => {
    if (!start) return;
    const dx = e.clientX - start.x;
    if (!moved && Math.abs(dx) < 5) return;
    if (!moved) grid.setPointerCapture(e.pointerId);
    moved = true;
    grid.classList.add('dragging');
    grid.scrollLeft = start.left - dx;
  });
  const end = () => { start = null; grid.classList.remove('dragging'); };
  grid.addEventListener('pointerup', end);
  grid.addEventListener('pointercancel', end);
  grid.addEventListener('click', (e) => { if (moved) { e.stopPropagation(); e.preventDefault(); moved = false; } }, true);
}

export async function pagePlan(r, org, people) {
  const key = r.org + '/' + r.plan;
  if (planState.key !== key) Object.assign(planState, { key, runId: null, aside: false, sel: null, adds: new Set(), undo: [] });
  const [plan, events, tasks, combos, fields, answers, places, cells, pools, weights, health, runs, sections, taskSections] = await Promise.all([
    get(Plan.getPlan, { org: r.org, plan: r.plan }, 'plan'), get(Plan.listEvents, { org: r.org, plan: r.plan }, 'events'), get(Setup.listTasks, { org: r.org }, 'tasks'), get(Setup.listCombinations, { org: r.org }, 'combinations'),
    get(Plan.listFields, { org: r.org, plan: r.plan }), get(Plan.listAnswers, { org: r.org, plan: r.plan }), get(Plan.listPlaces, { org: r.org, plan: r.plan }, 'places'), get(Draft.listCells, { org: r.org, plan: r.plan }, 'cells'),
    get(Draft.listPools, { org: r.org, plan: r.plan }, 'pools'), get(Draft.getWeights, { org: r.org, plan: r.plan }), get(Account.health, {}), get(Draft.listRuns, { org: r.org, plan: r.plan }, 'runs'), get(Setup.listSections, { org: r.org }, 'sections'), get(Setup.listTaskSections, { org: r.org }, 'sections')]);

  // The draft on show: the one picked under the earlier drafts, else the current plan (pinned), else the newest; none
  // when «Remove unlocked» set the drafts aside (also when another tab did it).
  if (plan.draftAside && !planState.aside) planState.runId = null;
  planState.aside = plan.draftAside;
  const pinned = runs.find((x) => x.pinned);
  const shownId = runs.some((x) => x.id === planState.runId) ? planState.runId : plan.draftAside ? null : pinned ? pinned.id : runs.length ? runs[0].id : null;
  // «Edit | View» (see showPage): View always shows the current plan, else the newest finished draft, not one picked in Edit
  const modeKey = 'mode:' + key;
  // an answer link has «My answers» and View; the others Edit (Table) and View
  const modes = org.level === 'answer' ? ['me', 'show'] : ['edit', 'show'];
  const stored = store.get(modeKey);
  const mode = modes.includes(stored) ? stored : org.level === 'answer' ? 'me' : org.level === 'view' ? 'show' : 'edit';
  const viewId = mode === 'edit' ? shownId : plan.draftAside ? null : pinned ? pinned.id : (runs.find((x) => x.status === 'done') || {}).id;
  const view = viewId ? await get(Draft.getRun, { org: r.org, plan: r.plan, id: viewId }) : null;
  const done = !!view && view.run.status === 'done';

  const label = (ev) => fmtShortDate(ev.date) + (ev.startTime ? ' ' + ev.startTime : '');
  const nameOf = (list, id) => (list.find((x) => x.id === id) || { name: '?' }).name;
  const at = (ev, tk) => ev.id + '/' + tk.id;
  const ans = {};  // person -> event -> never | poor | good ('' or missing = not answered)
  answers.events.forEach((a) => { (ans[a.personId] ||= {})[a.eventId] = a.level; });
  const notes = {};  // person -> event -> the note on that answer
  (answers.notes || []).forEach((n) => { (notes[n.personId] ||= {})[n.eventId] = n.note; });
  const chosen = {};  // person -> the tasks they take part in, with how willingly
  answers.tasks.forEach((a) => { (chosen[a.personId] ||= []).push(a); });
  const placesAt = {};  // 'event/task' -> places (missing = not needed)
  places.forEach((p) => { placesAt[p.eventId + '/' + p.taskId] = p.slots; });
  const fieldValue = {};  // 'event/field' -> text
  fields.values.forEach((v) => { fieldValue[v.eventId + '/' + v.fieldId] = v.value; });
  const cellsAt = {};  // 'event/task' -> the cells set by hand and the locked names, as they are now
  cells.forEach((c) => { (cellsAt[c.eventId + '/' + c.taskId] ||= []).push(c); });
  const poolAt = {};  // 'event/task' -> whom the planner may pick
  pools.forEach((p) => { poolAt[p.eventId + '/' + p.taskId] = p; });
  const planCell = {};  // 'event/task' -> the cell of the draft on show
  if (done) for (const ev of view.events) for (const c of ev.cells) planCell[ev.id + '/' + c.taskId] = c;
  // The drafts one by one (2026-11-23, the user: "show the different drafts"): ‹ › in the draft's bar step to an older or newer
  // draft, and «Show changes» marks the planner's names that are new since the finished draft before it (the old names in the
  // cell's title). Kept per plan in this browser.
  const changesKey = 'changes:' + key;
  const shownAt = done ? runs.findIndex((x) => x.id === view.run.id) : -1;
  const olderId = shownAt < 0 ? null : (runs.slice(shownAt + 1).find((x) => x.status === 'done') || {}).id;
  const comparing = mode === 'edit' && !!olderId && store.get(changesKey) === '1';
  const before = {};  // 'event/task' -> the planner's names there in the draft before
  if (comparing) for (const ev of (await get(Draft.getRun, { org: r.org, plan: r.plan, id: olderId })).events) for (const c of ev.cells) before[ev.id + '/' + c.taskId] = c.people;

  // What a place in the draft holds now: the cells set by hand and the locked names (always as they are now), then the
  // planner's names in the draft on show that are not among them (also one that was locked then and is unlocked now), then
  // what is still open. A name that was set by hand when the draft was made but has been removed since is not shown.
  const slotsAt = (ev, tk) => {
    const k = at(ev, tk);
    const n = placesAt[k] || 0;
    const items = (cellsAt[k] || []).map((c) => ({ cell: c, kind: c.personId ? 'person' : 'text', id: c.personId || 0,
      name: c.personId ? nameOf(people, c.personId) : c.label, fills: c.fills || 1, hand: !c.locked, locked: c.locked }));
    const rc = planCell[k];
    if (rc) {
      const has = (kind, id) => items.some((x) => x.kind === kind && x.id === id);
      rc.people.filter((p) => (!p.locked || p.kept) && !has('person', p.id)).forEach((p) => items.push({ kind: 'person', id: p.id, name: p.name, fills: 1, weak: p.weak }));
    }
    const filled = items.reduce((s, x) => s + x.fills, 0);
    return { n, items, open: Math.max(0, n - filled), why: rc && rc.missing > 0 ? rc.explanation || '' : '' };
  };
  const itemText = (x) => x.name + (x.fills > 1 ? ' ×' + x.fills : '');
  const slot = (x) => h('span', { class: 'slot' + (x.hand ? ' hand' : '') + (x.locked ? ' locked' : '') },
    x.locked ? mark('lock') : x.hand ? mark('hand') : '', itemText(x), x.weak ? h('span', { class: 'chip poor' }, t('draft.maybe')) : '');
  // Locking (2026-11-22, the user: "La en planlagt celle som inneholder navn ha en lås-knapp … Det bør kanskje være enklere å
  // låse plan?"): the planner's names of places, to lock (those not locked yet) or to unlock (those locked), in one request.
  const lockItems = (pairs, on) => pairs.flatMap(([ev, tk]) => slotsAt(ev, tk).items
    .filter((x) => x.kind === 'person' && (on ? !x.cell : x.locked)).map((x) => ({ event: ev.id, task: tk.id, person: x.id })));
  const setLocks = async (list, on) => {
    if (!list.length) { say(t(on ? 'sel.nothing_to_lock' : 'sel.nothing_to_unlock')); return; }
    if (!(await write(() => call(Draft.setLocks, { org: r.org, plan: r.plan, locks: list, locked: on })))) return;
    undoable(tn(on ? 'undo.lock' : 'undo.unlock', list.length), () => call(Draft.setLocks, { org: r.org, plan: r.plan, locks: list, locked: !on }));
    if (sel && sel.grid === 'draft') planState.sel = { keys: [...sel.keys], anchor: sel.anchor };  // a selection of the draft stays
    render();
  };
  const everyPlace = () => events.flatMap((ev) => tasks.map((tk) => [ev, tk]));
  // Undo (2026-11-23, the user: "Add a easier way to regret options"): what this tab changed in the draft, newest last (at most
  // 30, forgotten with another plan or a reload). Each entry undoes itself with the opposite request(s); not a change log.
  const undoable = (label, undo) => { planState.undo.push({ label, undo }); if (planState.undo.length > 30) planState.undo.shift(); };
  const undoLast = planState.undoLast = async () => {
    const u = planState.undo.pop();
    if (!u) return;
    if (await write(u.undo)) { await render(); say(t('undo.done', { what: u.label })); } else render();
  };
  const cellBack = (c) => ({ event: c.eventId, task: c.taskId, ...(c.personId ? { person: c.personId } : { label: c.label }), fills: c.fills, locked: c.locked });
  // A lock for a whole row (a task) or column (an event) of the draft (2026-11-23, the user: "Let it be possible to lock a column
  // or a row"): pressed when every planner name in it is locked; a press locks the rest, or unlocks them all.
  const groupLock = (pairs, lockAria, unlockAria) => {
    const toLock = lockItems(pairs, true), toUnlock = lockItems(pairs, false);
    if (!toLock.length && !toUnlock.length) return '';
    const on = !toLock.length;
    const name = on ? unlockAria : lockAria;
    return h('button', { type: 'button', class: 'lockbtn group' + (on ? ' on' : ''), 'aria-pressed': String(on), title: name, 'aria-label': name,
      onclick: () => setLocks(on ? toUnlock : toLock, !on) }, icon(on ? 'lock' : 'unlock'));
  };

  // ---- «Edit | View» (2026-11-23, the user: "Kanskje også en visning som er en penere visning uten redigering eller
  // tilgjengelighet? … Ikke egne lenker for nå, bare to moduser for samme data."): the same page, the same data. View shows what
  // each place holds now (as the draft shows it: the current plan, else the newest finished draft), with the event details, and
  // nothing to change. Remembered per plan in this browser; one who may only view starts in View.
  const modeSwitch = h('div', { class: 'mode', role: 'group', 'aria-label': t('mode.aria') },
    modes.map((m) => h('button', { type: 'button', 'aria-pressed': String(mode === m), 'data-mode': m,
      onclick: () => { if (m !== mode) { store.set(modeKey, m); render(); } } }, t(m === 'edit' && org.level === 'view' ? 'mode.table' : 'mode.' + m))));
  const shareBtn = () => (ME && ME.login && org.role ? h('button', { type: 'button', class: 'ghost share', onclick: () => shareDialog(r.org, org, plan) }, icon('share'), h('span', { class: 'lbl' }, ' ' + t('share.open'))) : '');
  // back to the organisation; not for a link, whose organisation page leads straight back here
  const back = () => (org.plan ? '' : link(`/o/${enc(r.org)}`, '‹ ' + org.name, { class: 'back small', 'aria-label': t('common.back', { name: org.name }) }));
  // only tasks some event needs or has a name in
  const showTasksOf = (ev) => tasks.filter((tk) => placesAt[at(ev, tk)] || cellsAt[at(ev, tk)]);
  if (mode === 'show') return showPage();
  if (mode === 'me') return mePage();
  // «My answers» (2026-11-22, the user: "Vi trenger også en visning for én person. Full planlegging av alt blir litt mye for en frivillig å
  // forholde seg til, så gjerne en visning der de kan fylle ut når de selv kan. Foreløpig uten noen bruker-management, så samme lenke."):
  // what an answer link opens. «Who are you?» picks a name from the list (kept in this browser, «Not you? Switch»); then the person's
  // own tasks and every date down the page with yes / maybe / no and a note. The same link for everyone: anyone may answer for anyone.
  function mePage() {
    const meKey = 'me:' + key;
    const me = people.find((p) => String(p.id) === store.get(meKey));
    const title = h('div', { class: 'title-bar' }, h('h1', {}, plan.name), infoBtn(plan.name, [t('me.info')]), modeSwitch);
    if (!me) {
      const picks = [...people].sort((a, b) => a.name.localeCompare(b.name, locale())).map((p) => h('button', { type: 'button', class: 'ghost me-pick', 'data-name': p.name.toLowerCase(),
        onclick: () => { store.set(meKey, String(p.id)); render(); } }, p.name));
      const find = people.length > 8 ? h('input', { type: 'search', class: 'wide', placeholder: t('table.filter'), 'aria-label': t('table.filter'), oninput: (e) => {
        const q = e.target.value.trim().toLowerCase();
        picks.forEach((b) => { b.hidden = !!q && !b.dataset.name!.includes(q); });
      } }) : '';
      return h('div', { class: 'plan-page me-page' }, title, h('h2', {}, t('me.who')), h('p', { class: 'muted small' }, t('me.who_hint')),
        people.length ? [find, h('div', { class: 'me-picks' }, picks)] : h('p', { class: 'muted' }, t('me.nobody')));
    }
    const mine = events.flatMap((ev) => showTasksOf(ev).filter((tk) => slotsAt(ev, tk).items.some((x) => x.kind === 'person' && x.id === me.id)).map((tk) => [ev, tk] as const));
    const level = (ev) => (ans[me.id] || {})[ev.id] || '';
    let saving = Promise.resolve();  // one write after the other: Yes then No quickly must end as No
    const answerRow = (ev) => {
      const btns = ['good', 'poor', 'never'].map((l) => h('button', { type: 'button', class: 'me-ans ' + l, 'data-level': l, 'aria-pressed': String(level(ev) === l),
        'aria-label': t('me.answer_aria', { label: label(ev), answer: t('me.' + l) }) }, icon(LEVEL_ICON[l]), h('span', {}, t('me.' + l))));
      const paint = () => btns.forEach((b) => b.setAttribute('aria-pressed', String(level(ev) === b.dataset.level)));
      btns.forEach((b) => b.addEventListener('click', async () => {
        const was = level(ev);
        const next = was === b.dataset.level ? '' : b.dataset.level!;  // the same again: not answered
        (ans[me.id] ||= {})[ev.id] = next;
        paint();
        saving = saving.then(async () => {
          if (!(await write(() => call(Plan.setAvailability, { org: r.org, plan: r.plan, person: me.id, levels: { [ev.id]: next } })))) { ans[me.id][ev.id] = was; paint(); }
        });
      }));
      let saved = (notes[me.id] || {})[ev.id] || '';
      const note = h('input', { type: 'text', class: 'me-note', maxlength: '300', value: saved, placeholder: t('note.placeholder'), 'aria-label': t('note.aria', { name: me.name, label: label(ev) }) });
      note.addEventListener('change', async () => {
        const value = note.value.trim();
        if (value === saved) return;
        saving = saving.then(async () => {
          if (await write(() => call(Plan.setNote, { org: r.org, plan: r.plan, event: ev.id, person: me.id, note: value }))) { saved = value; (notes[me.id] ||= {})[ev.id] = value; say(t('me.note_saved')); }
        });
      });
      const tasksHere = mine.filter(([e]) => e.id === ev.id).map(([, tk]) => tk.name);
      const details = fields.fields.map((f) => fieldValue[ev.id + '/' + f.id]).filter(Boolean).join(' · ');
      return h('li', { class: 'me-ev' + (tasksHere.length ? ' busy' : ''), 'data-event': String(ev.id) },
        h('div', { class: 'me-when' }, h('span', { class: 'd' }, fmtDate(ev.date)), clock(ev) ? h('span', { class: 'tm' }, clock(ev)) : '',
          tasksHere.length ? h('span', { class: 'me-task' }, t('me.you_have', { tasks: tasksHere.join(', ') })) : '',
          details ? h('span', { class: 'small muted' }, details) : ''),
        h('div', { class: 'me-btns', role: 'group', 'aria-label': label(ev) }, btns), note);
    };
    return h('div', { class: 'plan-page me-page' }, title,
      h('div', { class: 'me-head' }, h('h2', {}, me.name), linkBtn(t('me.switch'), () => { store.set(meKey, ''); render(); })),
      h('section', { class: 'me-mine' }, h('h3', {}, t('me.mine')),
        mine.length ? h('ul', { class: 'plain' }, mine.map(([ev, tk]) => h('li', {}, h('b', {}, fmtShortDate(ev.date)), clock(ev) ? ' ' + clock(ev) : '', ' · ', tk.name)))
          : h('p', { class: 'muted small' }, done ? t('me.no_tasks') : t('me.no_plan'))),
      h('h3', {}, t('me.when')),
      events.length ? h('ul', { class: 'plain me-list' }, events.map(answerRow)) : h('p', { class: 'muted small' }, t('show.no_events')));
  }

  function showPage() {
    // only tasks some event needs or has a name in, and details some event has
    const showTasks = tasks.filter((tk) => events.some((ev) => placesAt[at(ev, tk)] || cellsAt[at(ev, tk)]));
    const showFields = fields.fields.filter((f) => events.some((ev) => fieldValue[ev.id + '/' + f.id]));
    const has = (ev, tk) => !!(placesAt[at(ev, tk)] || cellsAt[at(ev, tk)]);
    const value = (ev, f) => fieldValue[ev.id + '/' + f.id] || '';
    const names = (ev, tk) => {
      const s = slotsAt(ev, tk);
      return h('span', { class: 'names' }, s.items.map((x) => h('span', { class: 'nm', 'data-person': x.kind === 'person' ? String(x.id) : null }, itemText(x))),
        s.open ? h('span', { class: 'open' }, tn('show.open', s.open)) : '');
    };
    const hasTask = (ev, id) => showTasks.some((tk) => has(ev, tk) && slotsAt(ev, tk).items.some((x) => x.kind === 'person' && x.id === id));
    const when = (ev) => [h('span', { class: 'd' }, fmtShortDate(ev.date)), clock(ev) ? h('span', { class: 'tm' }, clock(ev)) : ''];
    // Remembered per plan in this browser: the name found, whether the dates go across (2026-11-23, the user: "Add option to show
    // the table with dates on the x axis"), and whether only the dates the name has a task on are shown ("only include dates where
    // the selected person has a task").
    const findKey = 'find:' + key, acrossKey = 'across:' + key, onlyKey = 'only:' + key;
    let across = store.get(acrossKey) === '1';
    let only = store.get(onlyKey) === '1';
    let found = 0;
    // events down, tasks across; or the dates across, the details and the tasks down. Every cell of an event says which (data-ev).
    const down = (evs) => h('table', { class: 'show-table' },
      h('thead', {}, h('tr', {}, h('th', { scope: 'col' }, t('show.date')), showFields.map((f) => h('th', { scope: 'col' }, f.name)),
        showTasks.map((tk) => h('th', { scope: 'col' }, tk.name)))),
      h('tbody', {}, evs.map((ev) => h('tr', { 'data-ev': String(ev.id) },
        h('th', { scope: 'row' }, when(ev)),
        showFields.map((f) => h('td', { class: 'detail' }, value(ev, f))),
        showTasks.map((tk) => h('td', {}, has(ev, tk) ? names(ev, tk) : ''))))));
    const acrossTable = (evs) => h('table', { class: 'show-table across' },
      h('thead', {}, h('tr', {}, h('th', { scope: 'col' }, ''), evs.map((ev) => h('th', { scope: 'col', 'data-ev': String(ev.id) }, when(ev))))),
      h('tbody', {},
        showFields.map((f) => h('tr', {}, h('th', { scope: 'row' }, f.name), evs.map((ev) => h('td', { class: 'detail', 'data-ev': String(ev.id) }, value(ev, f))))),
        showTasks.map((tk) => h('tr', {}, h('th', { scope: 'row' }, tk.name), evs.map((ev) => h('td', { 'data-ev': String(ev.id) }, has(ev, tk) ? names(ev, tk) : ''))))));
    const cards = (evs) => h('div', { class: 'show-cards' }, evs.map((ev) => h('section', { class: 'show-card', 'data-ev': String(ev.id) },
      h('h2', {}, fmtDate(ev.date), clock(ev) ? h('span', { class: 'tm' }, clock(ev)) : ''),
      showFields.filter((f) => value(ev, f)).map((f) => h('p', { class: 'detail' }, h('span', { class: 'lbl' }, f.name), ' ', value(ev, f))),
      h('dl', {}, showTasks.filter((tk) => has(ev, tk)).map((tk) => h('div', {}, h('dt', {}, tk.name), h('dd', {}, names(ev, tk))))))));
    const body = h('div', { class: 'show-body' });
    const count = h('span', { class: 'small found', 'aria-live': 'polite' });
    const onlyBox = h('input', { type: 'checkbox', checked: only, onchange: (e) => { only = e.target.checked; store.set(onlyKey, only ? '1' : ''); draw(); } });
    const onlyLabel = h('span', {});
    const onlyField = h('label', { class: 'check only' }, onlyBox, onlyLabel);
    const acrossBtn = h('button', { type: 'button', class: 'ghost across-btn', 'aria-pressed': String(across), title: t('show.across_hint'),
      onclick: () => { across = !across; store.set(acrossKey, across ? '1' : ''); acrossBtn.setAttribute('aria-pressed', String(across)); draw(); } },
    icon('check'), ' ' + t('show.across'));
    // the name found stands out, the events without it fade (or are left out, with «only»)
    function draw() {
      const evs = found && only ? events.filter((ev) => hasTask(ev, found)) : events;
      body.replaceChildren(h('div', { class: 'show-scroll' }, (across ? acrossTable : down)(evs)), cards(evs));
      body.querySelectorAll<HTMLElement>('.nm[data-person]').forEach((x) => x.classList.toggle('hit', !!found && x.dataset.person === String(found)));
      const hits = new Set(events.filter((ev) => found && hasTask(ev, found)).map((ev) => String(ev.id)));
      body.querySelectorAll<HTMLElement>('[data-ev]').forEach((x) => x.classList.toggle('miss', !!found && !hits.has(x.dataset.ev!)));
      count.textContent = found ? tn('show.found', hits.size) : '';
      const p = people.find((x) => x.id === found);
      onlyField.hidden = !p;
      onlyLabel.textContent = p ? ' ' + t('show.only', { name: p.name }) : '';
    }
    // «Find a name»: a search field with the names that match under it (arrows and Enter, or a click); empty = nobody
    const sorted = [...people].sort((a, b) => a.name.localeCompare(b.name, locale()));
    const input = h('input', { type: 'search', class: 'find', placeholder: t('show.find'), 'aria-label': t('show.find'), autocomplete: 'off',
      role: 'combobox', 'aria-autocomplete': 'list', 'aria-expanded': 'false', 'aria-controls': 'find-list' });
    const list = h('ul', { class: 'find-list', id: 'find-list', role: 'listbox', hidden: true });
    let active = -1;
    const options = () => [...list.querySelectorAll('li[role=option]')];
    const close = () => { list.hidden = true; input.setAttribute('aria-expanded', 'false'); active = -1; };
    const pick = (p) => {
      found = p ? p.id : 0;
      input.value = p ? p.name : '';
      store.set(findKey, found ? String(found) : '');
      close();
      draw();
    };
    const activate = (i) => {
      active = i;
      options().forEach((o, j) => o.setAttribute('aria-selected', String(j === i)));
      if (options()[i]) { options()[i].scrollIntoView({ block: 'nearest' }); input.setAttribute('aria-activedescendant', options()[i].id); }
    };
    const open = () => {
      const q = input.value.trim().toLowerCase();
      const hits = sorted.filter((p) => p.name.toLowerCase().includes(q))
        .sort((a, b) => Number(!a.name.toLowerCase().startsWith(q)) - Number(!b.name.toLowerCase().startsWith(q)));
      list.replaceChildren(...(hits.length ? hits.map((p, i) => h('li', { role: 'option', id: 'find-' + i, 'aria-selected': 'false',
        onmousedown: (e) => { e.preventDefault(); pick(p); } }, p.name)) : [h('li', { class: 'none' }, t('show.find_none'))]));
      list.hidden = false;
      input.setAttribute('aria-expanded', 'true');
      if (q && hits.length) activate(0); else active = -1;
    };
    input.addEventListener('focus', () => { input.select(); open(); });
    input.addEventListener('input', () => { if (!input.value.trim()) { pick(null); input.focus(); } open(); });
    input.addEventListener('blur', () => {
      close();
      const p = people.find((x) => x.id === found);
      input.value = p ? p.name : '';  // what is typed but not picked is let go
    });
    input.addEventListener('keydown', (e) => {
      const n = options().length;
      if (e.key === 'ArrowDown') { e.preventDefault(); if (list.hidden) open(); activate(Math.min(n - 1, active + 1)); }
      else if (e.key === 'ArrowUp') { e.preventDefault(); activate(Math.max(0, active - 1)); }
      else if (e.key === 'Enter') { e.preventDefault(); const o = options()[active]; if (o) pick(sorted.find((p) => p.name === o.textContent)); }
      else if (e.key === 'Escape') { if (!list.hidden) { e.preventDefault(); close(); } }
    });
    const source = done ? t('show.source', { id: view.run.id, time: fmtTime(view.run.finishedAt || view.run.startedAt) }) + (view.run.pinned ? ' · ' + t('run.current') : '')
      : t('show.no_draft');
    const root = h('div', { class: 'plan-page show' },
      h('div', { class: 'title-bar' }, back(), h('h1', {}, plan.name), infoBtn(plan.name, [t('show.info')]), modeSwitch, shareBtn()),
      events.length ? h('div', { class: 'show-tools' }, h('div', { class: 'findbox' }, input, list), count, onlyField, acrossBtn,
        h('span', { class: 'small muted source' }, source),
        h('button', { type: 'button', class: 'ghost print', onclick: () => window.print() }, t('show.print'))) : h('p', { class: 'muted small' }, t('show.no_events')),
      events.length ? body : '');
    const was = Number(store.get(findKey) || (org.level === 'answer' ? store.get('me:' + key) : ''));  // an answer link: the name chosen
    const p = people.find((x) => x.id === was);
    if (p) { found = p.id; input.value = p.name; }
    draw();
    return root;
  }

  // ---- the draft: make one or try again, and what the draft on show is
  const newest = runs[0];
  const running = !!newest && newest.status === 'running';
  const pin = async (id, on) => {
    const was = (runs.find((x) => x.pinned) || {}).id;
    if (!(await write(() => call(on ? Draft.pinRun : Draft.unpinRun, { org: r.org, plan: r.plan, id })))) return;
    undoable(t(on ? 'undo.pin' : 'undo.unpin', { id }), () => (was ? call(Draft.pinRun, { org: r.org, plan: r.plan, id: was }) : call(Draft.unpinRun, { org: r.org, plan: r.plan, id })));
    planState.runId = id;
    await render();
    say(on ? t('run.pin_done', { id }) : t('run.unpin_done'));
  };
  // ---- folding: a section of the table, or of the people, folds to its heading; remembered per plan in this browser
  const foldKey = 'fold:' + r.org + '/' + r.plan;
  const folded = new Set((() => { try { return JSON.parse(store.get(foldKey) || '[]'); } catch (e) { return []; } })());
  const folds = (key, rows) => [rows].flat(Infinity).filter((x): x is HTMLElement => x instanceof Element)
    .map((tr) => { tr.dataset.fold = ((tr.dataset.fold || '') + ' ' + key).trim(); return tr; });
  const applyFolds = (table) => {
    table.querySelectorAll('tr[data-fold]').forEach((tr: HTMLElement) => {
      tr.hidden = tr.dataset.filtered === '1' || (tr.dataset.adds && !planState.adds.has(tr.dataset.adds)) || (tr.dataset.person && !shown(Number(tr.dataset.person))) || tr.dataset.fold!.split(' ').some((k) => folded.has(k));
    });
    table.querySelectorAll('button.fold').forEach((b) => b.setAttribute('aria-expanded', String(!folded.has(b.dataset.key))));
  };
  const foldBtn = (key, title) => h('button', { type: 'button', class: 'fold', 'data-key': key, 'aria-expanded': String(!folded.has(key)),
    title: t('table.fold'), 'aria-label': t('table.fold_aria', { name: title }), onclick: (e) => {
      if (folded.has(key)) folded.delete(key); else folded.add(key);
      store.set(foldKey, JSON.stringify([...folded]));
      applyFolds(e.currentTarget.closest('table'));
    } }, icon('caret'));
  // Who is shown (2026-11-22, the user: "La det være mulig å filtrere hvem du ønsker å vise i plan. Dette skal være en avkryssing, så
  // du kan velge et fritt utvalg."): a ✓ per person, kept in this browser per plan. Only what the table shows: the counts, the
  // planner and the draft still have everyone. The hidden ids are kept, so someone new is shown.
  const hideKey = 'hide:' + r.org + '/' + r.plan;
  const hidden = new Set((() => { try { return JSON.parse(store.get(hideKey) || '[]'); } catch { return []; } })()
    .filter((id) => people.some((p) => p.id === id)));
  // … and by task (2026-11-23, the user: "be a filter that also can filter on tasks"): with tasks ticked, only the people who take one
  // of them (good or if needed) are shown. Kept per plan in this browser, like the ticks on people.
  const taskKey = 'whotasks:' + r.org + '/' + r.plan;
  const byTask = new Set((() => { try { return JSON.parse(store.get(taskKey) || '[]'); } catch { return []; } })()
    .filter((id) => tasks.some((tk) => tk.id === id && tk.planned)));
  const takes = (p) => (chosen[p.id] || []).some((x) => byTask.has(x.taskId) && (x.level === 'good' || x.level === 'poor'));
  const shown = (id) => { const p = people.find((x) => x.id === id); return !p || (!hidden.has(id) && (!byTask.size || takes(p))); };
  const filtering = () => hidden.size > 0 || byTask.size > 0;
  const whoLabel = () => {
    const n = people.filter((p) => shown(p.id)).length;
    const of = t('who.shown', { shown: n, all: people.length });
    if (!byTask.size) return hidden.size ? of : t('who.all');
    return tasks.filter((tk) => byTask.has(tk.id)).map((tk) => tk.name).join(', ') + ' · ' + of;
  };
  const whoBtn = h('button', { type: 'button', class: 'who' + (filtering() ? ' on' : ''), title: t('who.label'), onclick: () => whoDialog() },
    icon('filter'), h('span', { class: 'lbl' }, whoLabel()));
  const whoSave = () => {
    store.set(hideKey, JSON.stringify([...hidden]));
    store.set(taskKey, JSON.stringify([...byTask]));
    whoBtn.querySelector('.lbl')!.textContent = whoLabel();
    whoBtn.classList.toggle('on', filtering());
    applyFolds(table.querySelector('table'));
  };
  const whoDialog = () => {
    const boxes = people.map((p) => h('input', { type: 'checkbox', checked: !hidden.has(p.id), onchange: (e) => {
      if (e.target.checked) hidden.delete(p.id); else hidden.add(p.id);
      whoSave();
    } }));
    const items = people.map((p, i) => h('label', { class: 'who-item', 'data-name': p.name.toLowerCase() }, boxes[i], h('span', {}, p.name)));
    const find = h('input', { type: 'search', class: 'wide', placeholder: t('table.filter'), 'aria-label': t('table.filter'), oninput: (e) => {
      const q = e.target.value.trim().toLowerCase();
      items.forEach((it) => { it.hidden = !!q && !it.dataset.name.includes(q); });
    } });
    // All / None act on the names the search shows, so «find "an", None» hides just those.
    const all = (show) => () => {
      people.forEach((p, i) => { if (!items[i].hidden) { if (show) hidden.delete(p.id); else hidden.add(p.id); boxes[i].checked = show; } });
      whoSave();
    };
    const planned = tasks.filter((tk) => tk.planned);
    const taskChips = planned.map((tk) => h('label', { class: 'chip who-task' }, h('input', { type: 'checkbox', checked: byTask.has(tk.id), onchange: (e) => {
      if (e.target.checked) byTask.add(tk.id); else byTask.delete(tk.id);
      whoSave();
    } }), ' ' + tk.name));
    modal(t('who.label'), { info: [t('who.info')] },
      planned.length ? [h('h3', {}, t('who.tasks')), h('p', { class: 'muted small' }, t('who.tasks_hint')), h('div', { class: 'who-tasks' }, taskChips), h('h3', {}, t('who.people'))] : '',
      h('div', { class: 'who-head' }, find,
        h('button', { type: 'button', class: 'ghost', onclick: all(true) }, t('who.select_all')),
        h('button', { type: 'button', class: 'ghost', onclick: all(false) }, t('who.select_none'))),
      h('div', { class: 'who-list' }, items), foot('', closeBtn()));
  };
  const tagsBtn = h('button', { type: 'button', class: 'tags-btn', 'aria-pressed': String(store.get('tags') === '1'), title: t('tags.hint'),
    onclick: (e) => {
      const on = store.get('tags') !== '1';
      store.set('tags', on ? '1' : '');
      e.currentTarget.setAttribute('aria-pressed', String(on));
      e.currentTarget.closest('table').classList.toggle('show-tags', on);
    } }, icon('check'), h('span', { class: 'lbl' }, t('tags.show')));
  // With many people: a filter on the names (only the rows; the sections stay). A selection keeps to the rows shown.
  let filterText = '';
  const filterBox = () => h('input', { type: 'search', class: 'filter', value: filterText, placeholder: t('table.filter'), 'aria-label': t('table.filter'),
    oninput: (e) => {
      filterText = e.target.value.trim().toLowerCase();
      const table = e.target.closest('table');
      table.querySelectorAll('tr.person-row').forEach((tr) => { tr.dataset.filtered = filterText && !tr.dataset.name.includes(filterText) ? '1' : ''; });
      applyFolds(table);
    } });

  const bar = h('div', { class: 'draft-bar' });
  let barMore: HTMLElement[] = [];  // what goes behind More ⋯ (filled with the bar)
  const poll = async () => {
    if (!bar.isConnected) return;  // the page has been replaced
    try {
      const latest = await call(Draft.getRun, { org: r.org, plan: r.plan, id: 0 });
      if (latest.run.status === 'running') setTimeout(poll, live && live.es.readyState === 1 ? 5000 : 1000);  // the stream says when it ends
      else { planState.runId = latest.run.id; forget(changedBy('RunFinished'), r.org); render(); }
    } catch (e) { say(errorText(e), 'err'); }
  };
  {
    const limits = [[10, '10 s'], [20, '20 s'], [60, '60 s']];
    const limitSel = h('select', { 'aria-label': t('run.limit_aria') }, limits.map(([v, text]) => h('option', { value: String(v), selected: String(v) === (store.get('limit') || '20') }, text)));
    limitSel.addEventListener('change', () => store.set('limit', limitSel.value));
    const start = async () => {
      let id;
      if (await write(async () => { id = (await call(Draft.startRun, { org: r.org, plan: r.plan, timeLimit: parseInt(limitSel.value, 10) })).id; })) { planState.runId = id; render(); }
    };
    const nothing = !events.length || !people.length || !Object.values(placesAt).some(Boolean);  // no event, person or place: nothing to plan
    // The bar in three groups (2026-11-23, the user: "Make the draft plan top Row with the buttons and such easier and prettier"):
    // which draft is on show (‹ #n ›, Show changes, the current-plan pill), what to do with it (We’re done, Try again with its
    // time, Undo), and the rest (the compromises as a chip, Adjust the planner, More ⋯ with what is seldom used).
    const which: HTMLElement[] = [], line: HTMLElement[] = [], extra: HTMLElement[] = [], more: HTMLElement[] = [];
    const retry = h('span', { class: 'split' }, h('button', { type: 'button', class: view ? 'retry' : 'retry first', disabled: !health.planner || running || nothing,
      title: nothing && !running ? t('run.nothing') : null, onclick: start }, running ? t('run.planning') : view ? t('run.retry') : t('run.plan')), limitSel);
    const notes: HTMLElement[] = [];
    if (!health.planner) line.push(h('span', { class: 'small' }, t('run.no_planner')));
    if (running) { line.push(h('span', { role: 'status' }, t('run.running', { n: newest.timeLimit }))); setTimeout(poll, live && live.es.readyState === 1 ? 5000 : 1000); }
    if (!view) { if (!running) line.push(h('span', { class: 'small' }, plan.draftAside && runs.length ? t('run.aside') : t('run.none_yet'))); }
    else if (view.run.status === 'failed') notes.push(h('p', { class: 'err', role: 'alert' }, t('run.failed', { error: view.run.error || t('run.unknown_error') })));
    else if (done) {
      const run = view.run;
      const about = t('run.label', { id: run.id, time: fmtTime(run.finishedAt || run.startedAt) }) + ' · ' + (run.solverStatus === 'OPTIMAL' ? t('run.optimal') : t('run.feasible'));
      const step = (i, aria, ic) => h('button', { type: 'button', class: 'ghost icon step', disabled: !runs[i], 'aria-label': aria, title: aria,
        onclick: () => { planState.runId = runs[i].id; render(); } }, icon(ic));
      which.push(h('span', { class: 'steps' }, step(shownAt + 1, t('run.older'), 'left'),
        h('span', { class: 'small run-id', title: about }, '#' + run.id, run.solverStatus === 'OPTIMAL' ? '' : h('span', { class: 'warn' }, ' ⚠')),
        step(shownAt - 1, t('run.newer'), 'right')));
      if (olderId) which.push(h('button', { type: 'button', class: 'ghost sm changes' + (comparing ? ' on' : ''), 'aria-pressed': String(comparing),
        title: t('run.changes_hint', { id: olderId }), onclick: () => { store.set(changesKey, comparing ? '' : '1'); render(); } }, t('run.changes')));
      which.push(h('button', { type: 'button', class: 'ghost sm pin' + (run.pinned ? ' on' : ''), 'aria-pressed': String(!!run.pinned),
        title: run.pinned ? t('run.current') + ' · ' + t('run.unpin') : t('run.pin'), 'aria-label': run.pinned ? t('run.unpin') : t('run.pin'),
        onclick: () => pin(run.id, !run.pinned) }, icon('pin'), ' ' + (run.pinned ? t('run.current') : t('run.make_current'))));
      if (run.id !== newest.id) which.push(linkBtn(t('run.show_newest'), () => { planState.runId = newest.id; render(); }));
      if (org.level !== 'view') {
        // «We’re done» (2026-11-23, the user: "Maybe remove the lock all? … Maybe a more visible "we are done" button?"): locks every
        // name of the draft on show, makes it the current plan and shows it in View. Done = pinned with nothing left to lock;
        // «Reopen» unpins it and keeps the locks. «Remove unlocked»: only what is set by hand and locked is left.
        const toLock = lockItems(everyPlace(), true);
        if (run.pinned && !toLock.length) {
          line.push(h('span', { class: 'done-mark', role: 'status' }, icon('check'), ' ' + t('draft.is_done')));
          line.push(h('button', { type: 'button', class: 'ghost sm', title: t('draft.reopen_hint'), onclick: async () => {
            if (await write(() => call(Draft.unpinRun, { org: r.org, plan: r.plan, id: run.id }))) {
              undoable(t('undo.reopen'), () => call(Draft.pinRun, { org: r.org, plan: r.plan, id: run.id }));
              planState.runId = run.id; await render(); say(t('draft.reopened'));
            }
          } }, t('draft.reopen')));
        } else {
          line.push(h('button', { type: 'button', class: 'done', disabled: running, title: t('draft.done_hint'), onclick: async () => {
            const was = (runs.find((x) => x.pinned) || {}).id;
            if (toLock.length && !(await write(() => call(Draft.setLocks, { org: r.org, plan: r.plan, locks: toLock, locked: true })))) return;
            if (!run.pinned && !(await write(() => call(Draft.pinRun, { org: r.org, plan: r.plan, id: run.id })))) return;
            undoable(t('undo.done_mark'), async () => {
              if (toLock.length) await call(Draft.setLocks, { org: r.org, plan: r.plan, locks: toLock, locked: false });
              if (!run.pinned) await (was ? call(Draft.pinRun, { org: r.org, plan: r.plan, id: was }) : call(Draft.unpinRun, { org: r.org, plan: r.plan, id: run.id }));
            });
            planState.runId = run.id;
            store.set(modeKey, 'show');
            await render();
            say(t('draft.done_said'));
          } }, icon('check'), ' ' + t('draft.done')));
        }
        more.push(h('button', { type: 'button', class: 'drop-item aside', disabled: running, title: t('draft.aside_hint'), onclick: async () => {
          if (!(await ask(t('draft.aside'), t('draft.aside_confirm'), t('draft.aside_yes')))) return;
          const was = (runs.find((x) => x.pinned) || {}).id;
          if (await write(() => call(Draft.setDraftAside, { org: r.org, plan: r.plan }))) {
            undoable(t('undo.aside'), () => (was ? call(Draft.pinRun, { org: r.org, plan: r.plan, id: was }) : call(Draft.clearDraftAside, { org: r.org, plan: r.plan })));
            planState.runId = null; await render(); say(t('draft.aside_done'));
          }
        } }, t('draft.aside')));
      }
      if (run.stale) notes.push(h('p', { class: 'warn small', role: 'status' }, t('run.stale')));
      if (run.checked && run.violations.length) notes.push(h('div', { class: 'err', role: 'alert' }, h('p', {}, t('run.violations')), h('ul', {}, run.violations.map((v) => h('li', {}, v)))));
      if (view.warnings.length) notes.push(h('ul', { class: 'warn small' }, view.warnings.map((w) => h('li', {}, w))));
    }
    if (org.level !== 'view' && planState.undo.length) {
      const last = planState.undo[planState.undo.length - 1];
      extra.push(h('button', { type: 'button', class: 'ghost sm undo', title: t('undo.title', { what: last.label }) + ' (Ctrl+Z)', 'aria-label': t('undo.title', { what: last.label }), onclick: undoLast },
        icon('undo'), ' ' + t('undo.btn')));
    }
    // what the squares mean is in the ⓘ, not on the bar
    const key = (m, text) => h('span', { class: 'key' }, m, h('span', {}, text));
    line.push(retry);
    bar.replaceChildren(h('div', { class: 'row-inline' }, which.length ? h('span', { class: 'grp which' }, which) : '', h('span', { class: 'grp acts' }, line),
      h('span', { class: 'grp extra' }, extra)), ...notes);
    barMore = more;
    bar.prepend(h('span', { class: 'bar-title' }, foldBtn('draft', t('draft.title')), t('draft.title'),
      infoBtn(t('draft.title'), [t('draft.info'), key(mark('hand'), t('draft.key_hand_info')), key(mark('lock'), t('draft.key_locked_info'))])));
  }

  // ---- dialogs
  function eventDialog(ev) {
    let dlg;
    const start = h('input', { type: 'time', value: ev.startTime, 'aria-label': t('event.starts') });
    const end = h('input', { type: 'time', value: ev.endTime, 'aria-label': t('event.ends') });
    const values = fields.fields.map((f) => ({ f, input: h('input', { type: 'text', maxlength: '200', value: fieldValue[ev.id + '/' + f.id] || '', 'aria-label': f.name }) }));
    dlg = modal(fmtDate(ev.date), { info: [t('event.planner_ignores')] },
      h('form', { class: 'stack', onsubmit: async (e) => {
        e.preventDefault();
        const body: Record<string, string> = {};
        const endWanted = start.value ? end.value : '';  // an end time needs a start time
        if (start.value !== ev.startTime) body.startTime = start.value;
        if (endWanted !== ev.endTime) body.endTime = endWanted;
        const todo = values.filter(({ f, input }) => input.value.trim() !== (fieldValue[ev.id + '/' + f.id] || ''));
        if (!Object.keys(body).length && !todo.length) { dlg.close(); return; }
        if (await write(async () => {
          if (Object.keys(body).length) await call(Plan.updateEvent, { org: r.org, plan: r.plan, id: ev.id, ...body }, 'event');
          for (const { f, input } of todo) await call(Plan.setFieldValue, { org: r.org, plan: r.plan, event: ev.id, field: f.id, value: input.value });
        })) { dlg.close(); render(); }
      } },
        h('div', { class: 'row-inline' }, field(t('event.starts'), start), field(t('event.ends'), end)),
        values.map(({ f, input }) => field(f.name, input)),
        foot(h('button', { type: 'button', class: 'danger', onclick: async () => {
          if (await write(async () => { await confirmed(t('event.delete'), t('event.delete_confirm', { date: fmtDate(ev.date) }), t('common.delete')); await call(Plan.deleteEvent, { org: r.org, plan: r.plan, id: ev.id }); })) { dlg.close(); render(); }
        } }, t('event.delete')),
        h('button', { type: 'button', class: 'ghost', onclick: () => { dlg.close(); copyDialog(ev); } }, icon('copy'), ' ' + t('event.copy')),
        h('button', { type: 'submit' }, t('common.save')))));
  }

  // «Repeat»: does not repeat, every week, every other week … every 8 weeks, until a date, and how many events that gives.
  // onEvery is told when the choice changes (the dialogs move a date they suggested into the new rhythm).
  function repeat(dates, onEvery, preview?) {
    const everyText = (n) => (n === 1 ? t('copy.every_1') : n === 2 ? t('copy.every_2') : t('copy.every_n', { n }));
    const sel = h('select', { 'aria-label': t('copy.repeat') }, h('option', { value: '0' }, t('copy.once')),
      Array.from({ length: MAX_EVERY }, (_, i) => h('option', { value: String(i + 1) }, everyText(i + 1))));
    const until = h('input', { type: 'date', disabled: true, 'aria-label': t('copy.until') });
    const count = h('span', { class: 'small muted', 'aria-live': 'polite' });
    const every = () => Number(sel.value);
    let untilField;
    const update = () => {
      until.disabled = !every();
      untilField.hidden = !every();
      until.required = !!every();
      const all = repeatDates(dates(), until.value, every()).sort();
      const n = all.length;
      count.textContent = n && !preview ? '→ ' + tn('copy.count', n) : '';  // (a preview lists the dates instead)
      if (preview) preview(all);
    };
    sel.addEventListener('change', () => { if (onEvery) onEvery(every()); update(); });
    until.addEventListener('input', update);
    return {
      el: h('div', { class: 'row-inline repeat' }, field(t('copy.repeat'), sel), untilField = field(t('copy.until'), until), count),
      until: () => (every() ? until.value : ''), every, update,
    };
  }

  // A copy of an event on one or more dates. What comes along is chosen; answers and the draft are never copied.
  // (2026-11-23, the user: "Improve the copy button for events"): the first date suggested is the first one in the rhythm with no
  // event yet; the dates it makes are listed, a date that already has an event marked (it gets a second one); the button says how many.
  function copyDialog(ev) {
    let dlg;
    const inputs: HTMLInputElement[] = [];
    const list = h('div', { class: 'dates' });
    const dates = () => inputs.map((i) => i.value).filter(Boolean);
    const taken = new Set(events.map((x) => x.date));
    const free = (d, step) => { for (let i = 0; i < 60 && taken.has(d); i++) d = addDays(d, step); return d; };
    // The first new date is a suggestion one step after the event, in the rhythm chosen, until it is changed by hand.
    let suggested = true;
    const shown = h('ul', { class: 'copy-dates', 'aria-label': t('copy.made') });
    const submit = h('button', { type: 'submit' }, t('copy.button'));
    const preview = (all) => {
      const max = 14;
      shown.replaceChildren(...all.slice(0, max).map((d) => h('li', { class: taken.has(d) ? 'taken' : '', title: taken.has(d) ? t('copy.taken') : '' },
        fmtShortDate(d), taken.has(d) ? h('span', { class: 'small' }, ' · ' + t('copy.taken_short')) : '')),
        all.length > max ? h('li', { class: 'more' }, t('copy.more', { n: all.length - max })) : '');
      shown.hidden = !all.length;
      submit.textContent = all.length ? tn('copy.make', all.length) : t('copy.button');
      submit.disabled = !all.length;
    };
    const week = repeat(dates, (n) => { if (suggested) inputs[0].value = free(addDays(ev.date, 7 * Math.max(n, 1)), 7 * Math.max(n, 1)); }, preview);
    const addDate = (v) => {
      const i = h('input', { type: 'date', value: v || '', 'aria-label': t('copy.date') });
      i.addEventListener('input', week.update);
      inputs.push(i);
      list.append(i);
      return i;
    };
    addDate(free(addDays(ev.date, 7), 7)).addEventListener('input', () => { suggested = false; });
    const check = (on, text, detail?) => {
      const box = h('input', { type: 'checkbox', checked: on, 'aria-label': text });
      return { box, el: h('label', { class: 'check' }, box, ' ' + text, detail ? h('span', { class: 'muted small' }, ' ' + detail) : '') };
    };
    const details = fields.fields.filter((f) => fieldValue[ev.id + '/' + f.id]).map((f) => f.name);
    const placeText = tasks.filter((tk) => placesAt[at(ev, tk)]).map((tk) => `${tk.name} ${placesAt[at(ev, tk)]}`).join(', ');
    const times = check(true, t('copy.times'), clock(ev) || t('copy.none'));
    const det = check(true, t('copy.details'), details.join(', ') || t('copy.none'));
    const pl = check(true, t('copy.places'), placeText || t('copy.none'));
    const hand = check(false, t('copy.hand'));
    dlg = modal(t('copy.title_of', { label: fmtShortDate(ev.date) }), { info: [t('copy.never')] },
      h('form', { class: 'stack', onsubmit: async (e) => {
        e.preventDefault();
        let made;
        if (await write(async () => {
          made = await call(Plan.copyEvent, { org: r.org, plan: r.plan, id: ev.id, dates: dates(), until: week.until(), every: week.every(), times: times.box.checked, details: det.box.checked, places: pl.box.checked, hand: hand.box.checked }, 'events');
        })) { dlg.close(); await render(); say(tn('copy.done', made.length)); }
      } },
        field(t('copy.date'), list),
        h('div', {}, linkBtn('+ ' + t('copy.another_date'), () => { addDate('').focus(); week.update(); })),
        week.el,
        shown,
        h('h3', {}, t('copy.comes_along')),
        times.el, det.el, pl.el, hand.el,
        foot('', submit)));
    week.update();
  }

  // A new event: empty, or a copy of one there is; optionally repeated every week or every few weeks until a date.
  function newEventDialog() {
    let dlg;
    const last = events[events.length - 1];
    const date = h('input', { type: 'date', required: true, value: last ? addDays(last.date, 7) : '', 'aria-label': t('event.date') });
    const from = h('select', { 'aria-label': t('event.start_from') }, h('option', { value: '' }, t('event.empty')),
      [...events].reverse().map((ev) => h('option', { value: String(ev.id), selected: !!last && ev.id === last.id }, t('event.copy_of', { label: label(ev) }))));
    const start = h('input', { type: 'time', 'aria-label': t('event.starts') });
    const end = h('input', { type: 'time', 'aria-label': t('event.ends') });
    // The date is a suggestion one step after the last event, in the rhythm chosen, until it is changed by hand.
    let suggested = !!last;
    const week = repeat(() => [date.value], (n) => { if (suggested) date.value = addDays(last.date, 7 * Math.max(n, 1)); });
    date.addEventListener('input', () => { suggested = false; week.update(); });
    const timesHint = h('p', { class: 'muted small' });
    const hint = () => { timesHint.textContent = from.value ? t('event.times_from_copy') : ''; timesHint.hidden = !from.value; };
    from.addEventListener('change', hint);
    dlg = modal(t('event.new'),
      h('form', { class: 'stack', onsubmit: async (e) => {
        e.preventDefault();
        let made;
        const times = { startTime: start.value, endTime: start.value ? end.value : '' };
        if (await write(async () => {
          made = from.value
            ? await call(Plan.copyEvent, { org: r.org, plan: r.plan, id: Number(from.value), dates: [date.value], until: week.until(), every: week.every(), times: !start.value, ...times, details: true, places: true, hand: false }, 'events')
            : await call(Plan.addEvents, { org: r.org, plan: r.plan, date: date.value, until: week.until(), every: week.every(), ...times }, 'events');
        })) { dlg.close(); await render(); say(tn('event.added', made.length)); }
      } },
        h('div', { class: 'row-inline' }, field(t('event.date'), date), events.length ? field(t('event.start_from'), from) : ''),
        h('div', { class: 'row-inline' }, field(t('event.starts'), start), field(t('event.ends'), end)),
        timesHint,
        week.el,
        foot('', h('button', { type: 'submit' }, t('common.add')))));
    hint();
    week.update();
    date.focus();
  }



  function taskDialog(tk) {
    let dlg;
    const changed = () => render();
    const name = h('input', { type: 'text', value: tk.name, maxlength: '80', 'aria-label': t('common.name') });
    const others = tasks.filter((x) => x.id !== tk.id);
    const levelWith = (o) => (combos.find((c) => c.a === Math.min(tk.id, o.id) && c.b === Math.max(tk.id, o.id)) || { level: 'standard' }).level;
    const alone = others.length > 0 && others.every((o) => levelWith(o) === 'cannot');
    const aloneBox = h('input', { type: 'checkbox', checked: alone, disabled: !others.length, 'aria-label': t('task.alone') });
    aloneBox.addEventListener('change', async () => {
      const on = aloneBox.checked;
      // unticking sets every pair with this task back to standard, also ones set by hand: ask first
      if (await write(async () => { if (!on) await confirmed(t('task.alone'), t('task.alone_off_confirm', { name: tk.name }), t('task.alone_off_yes')); await call(Setup.setTaskCombinations, { org: r.org, task: tk.id, level: on ? 'cannot' : 'standard' }); })) changed();
      else aloneBox.checked = !on;
    });
    const pairRows = others.map((o) => h('tr', {}, h('td', {}, o.name), h('td', {}, cycle('combo', levelWith(o), `${tk.name} + ${o.name}`, async (next) => {
      const ok = await write(() => call(Setup.setCombination, { org: r.org, a: tk.id, b: o.id, level: next }));
      if (ok) changed();
      return ok;
    }))));
    // Where the task stands in the table, among the tasks of its section, and which section (also for those who cannot drag,
    // such as on a phone).
    const mates = tasksIn(tk.section).filter((x) => x.id !== tk.id);
    const now = tasksIn(tk.section).findIndex((x) => x.id === tk.id);
    const where = h('select', { 'aria-label': t('task.where') }, h('option', { value: '0', selected: now === 0 }, t('field.at_top')),
      mates.map((x, i) => h('option', { value: String(i + 1), selected: now === i + 1 }, t('field.after', { name: x.name }))));
    where.addEventListener('change', async () => {
      if (await write(() => call(Setup.moveTask, { org: r.org, id: tk.id, position: parseInt(where.value, 10) }))) changed(); else where.value = String(now);
    });
    const sectionSel = h('select', { 'aria-label': t('task.section') }, h('option', { value: '0', selected: !tk.section }, t('task.no_section')),
      taskSections.map((x) => h('option', { value: String(x.id), selected: tk.section === x.id }, x.name)));
    sectionSel.addEventListener('change', async () => {
      if (await write(() => call(Setup.moveTask, { org: r.org, id: tk.id, section: parseInt(sectionSel.value, 10), position: tasks.length }))) changed(); else sectionSel.value = String(tk.section);
    });
    dlg = keyed(modal(tk.name, { info: [t('task.org_wide', { org: org.name }), t('task.auto_hint'), t('task.combine_hint')] },
      h('form', { class: 'row', onsubmit: async (e) => {
        e.preventDefault();
        if (name.value.trim() !== tk.name && await write(() => call(Setup.updateTask, { org: r.org, id: tk.id, name: name.value }, 'task'))) changed();
      } }, name, h('button', { type: 'submit' }, t('common.rename'))),
      mates.length ? field(t('task.where'), where) : '',
      taskSections.length ? field(t('task.section'), sectionSel) : '',
      h('h3', {}, t('task.combine')),
      h('label', { class: 'check' }, aloneBox, ' ', h('strong', {}, t('task.alone'))),
      pairRows.length ? h('table', { class: 'mini' }, h('tbody', {}, pairRows)) : '',
      foot(h('button', { type: 'button', class: 'danger', onclick: async () => {
        if (await write(async () => { await confirmed(t('task.delete'), t('task.delete_confirm', { name: tk.name }), t('common.delete')); await call(Setup.deleteTask, { org: r.org, id: tk.id }); })) { dlg.close(); render(); }
      } }, t('task.delete')), closeBtn())), 'task', tk.id);
  }

  function personDialog(p) {
    const changed = () => render();
    const name = h('input', { type: 'text', value: p.name, maxlength: '80', 'aria-label': t('common.name') });
    // the section the person is placed in (last in it); only where they stand in the table
    const sectionSel = h('select', { 'aria-label': t('person.section') }, h('option', { value: '0', selected: !p.section }, t('person.no_section')),
      sections.map((x) => h('option', { value: String(x.id), selected: p.section === x.id }, x.name)));
    sectionSel.addEventListener('change', async () => {
      if (await write(() => call(Setup.movePerson, { org: r.org, id: p.id, section: parseInt(sectionSel.value, 10), position: people.length }))) changed(); else sectionSel.value = String(p.section);
    });
    const levelOf = (tk) => ((chosen[p.id] || []).find((x) => x.taskId === tk.id) || { level: '' }).level;
    const taskRows = tasks.filter((tk) => tk.planned).map((tk) => {
      const cur = levelOf(tk);
      return h('tr', {}, h('td', { class: cur ? '' : 'muted' }, tk.name), h('td', {}, cycle('task', cur, t('person.task_aria', { name: p.name, task: tk.name }), async (next) => {
        const ok = await write(() => (next ? call(Setup.setPersonTask, { org: r.org, person: p.id, task: tk.id, level: next }) : call(Setup.removePersonTask, { org: r.org, person: p.id, task: tk.id })));
        if (ok) changed();
        return ok;
      })));
    });
    keyed(modal(p.name, { info: [t('person.tasks_hint'), t('person.auto_hint'), t('person.org_wide', { org: org.name })] },
      h('form', { class: 'row', onsubmit: async (e) => {
        e.preventDefault();
        if (name.value.trim() !== p.name && await write(() => call(Setup.updatePerson, { org: r.org, id: p.id, name: name.value }, 'person'))) changed();
      } }, name, h('button', { type: 'submit' }, t('common.rename'))),
      h('h3', {}, t('person.tasks')),
      taskRows.length ? h('table', { class: 'mini' }, h('tbody', {}, taskRows)) : h('p', { class: 'muted' }, t('person.no_tasks')),
      sections.length ? field(t('person.section'), sectionSel) : '',
      foot('', closeBtn())), 'person', p.id);
  }

  // A place in the draft: what is set by hand (violet), the planner's names (lock one to keep it), and whom the planner
  // may pick there.
  function placeDialog(ev, tk) {
    const k = at(ev, tk);
    const changed = () => render();
    const act = (fn) => async () => { if (await write(fn)) changed(); };
    const s = slotsAt(ev, tk);
    const rows = s.items.map((x) => h('tr', {},
      h('td', {}, slot(x)),
      h('td', { class: 'small muted' }, x.hand ? t('place.by_hand') : t('place.from_planner')),
      h('td', { class: 'right' }, x.cell
        ? linkBtn(x.locked ? t('place.unlock') : t('common.remove'), act(() => call(Draft.deleteCell, { org: r.org, plan: r.plan, id: x.cell.id })), t(x.locked ? 'place.unlock_aria' : 'place.remove_aria', { name: x.name }))
        : h('button', { type: 'button', class: 'ghost sm', 'aria-label': t('place.lock_aria', { name: x.name }),
          onclick: act(() => call(Draft.addCell, { org: r.org, plan: r.plan, event: ev.id, task: tk.id, person: x.id, locked: true }, 'cell')) }, t('place.lock')))));
    // By hand: tick any number of people (also more than the places: the place then shows as fuller than needed), or set a
    // text that fills places.
    const inCell = new Set(s.items.filter((x) => x.cell && x.kind === 'person').map((x) => x.id));
    const pick = people.filter((p) => !inCell.has(p.id)).map((p) => ({ p, box: h('input', { type: 'checkbox', 'aria-label': t('place.person_aria', { name: p.name }) }) }));
    const peopleForm = pick.length ? h('form', { class: 'stack', onsubmit: async (e) => {
      e.preventDefault();
      const ticked = pick.filter((x) => x.box.checked);
      if (!ticked.length) return;
      if (await write(async () => { for (const { p } of ticked) await call(Draft.addCell, { org: r.org, plan: r.plan, event: ev.id, task: tk.id, person: p.id }, 'cell'); })) changed();
    } }, h('div', { class: 'chips' }, pick.map(({ p, box }) => h('label', { class: 'chip' }, box, ' ' + p.name))),
      h('div', { class: 'row-inline' }, h('button', { type: 'submit' }, t('place.add_people')))) : '';
    const textIn = h('input', { type: 'text', maxlength: '80', placeholder: t('place.text_placeholder'), 'aria-label': t('common.text') });
    const fills = h('input', { type: 'number', min: '1', max: '50', value: '1', 'aria-label': t('place.fills') });
    const setForm = h('form', { class: 'row-inline', onsubmit: async (e) => {
      e.preventDefault();
      const body = { event: ev.id, task: tk.id, label: textIn.value, fills: parseInt(fills.value, 10) || 1 };
      if (await write(() => call(Draft.addCell, { org: r.org, plan: r.plan, ...body }, 'cell'))) changed();
    } }, textIn, fills, h('button', { type: 'submit', 'aria-label': t('place.add') }, '+'));

    let poolPart: Kid = '';
    if (tk.planned) {
      const pool = poolAt[k] || { people: [] };
      const takes = (p) => (chosen[p.id] || []).some((x) => x.taskId === tk.id && x.level !== 'never');
      const pBoxes = people.filter((p) => takes(p) || pool.people.includes(p.id)).map((p) => ({ p, box: h('input', { type: 'checkbox', checked: pool.people.includes(p.id), 'aria-label': t('place.pool_person_aria', { name: p.name }) }) }));
      // one line, «The planner can pick: anyone who takes Sound · Change»; Change opens the ticks
      const who = pool.people.length ? pool.people.map((id) => nameOf(people, id)).join(', ') : t('place.pool_all', { task: tk.name });
      const ticks = h('div', { class: 'stack pool-ticks', hidden: true },
        pool.people.length ? h('p', { class: 'muted small' }, t('place.pool_set')) : '',
        pBoxes.length ? [h('div', { class: 'chips' }, pBoxes.map(({ p, box }) => h('label', { class: 'chip' }, box, ' ' + p.name))),
          h('div', { class: 'row-inline' }, h('button', { type: 'button', class: 'ghost sm', onclick: act(() => call(Draft.setPool, { org: r.org, plan: r.plan, event: ev.id, task: tk.id, people: pBoxes.filter((x) => x.box.checked).map((x) => x.p.id) })) }, t('place.pool_save')))]
          : h('p', { class: 'muted small' }, t('place.pool_nobody', { task: tk.name })));
      poolPart = [h('p', { class: 'pool-line small' + (pool.people.length ? ' set' : '') }, pool.people.length ? h('span', { class: 'mark pool', 'aria-hidden': 'true' }, icon('filter')) : '',
        t('place.pool_line', { who }), ' · ', linkBtn(t('place.pool_change'), () => { ticks.hidden = !ticks.hidden; }, t('place.pool_change_aria'))), ticks];
    }
    const hands = s.items.filter((x) => x.hand);
    keyed(modal(`${tk.name} · ${fmtShortDate(ev.date)}`, { info: [t('place.add_people_hint'), tk.planned ? t('place.pool_anyone', { task: tk.name }) : t('place.not_planned'), t('place.lock_hint')] },
      h('p', { class: 'muted small' }, tn('place.places', s.n)),
      done && s.open && s.why ? h('p', { class: 'warn small' }, t('place.why', { why: s.why })) : '',
      rows.length ? h('table', { class: 'mini' }, h('tbody', {}, rows)) : h('p', { class: 'muted small' }, t('place.nothing')),
      h('h3', {}, t('place.set_by_hand')), peopleForm, setForm,
      poolPart,
      foot(hands.length ? h('button', { type: 'button', class: 'danger', onclick: act(async () => {
        await confirmed(t('place.clear'), t('place.clear_confirm', { task: tk.name, date: fmtShortDate(ev.date) }), t('place.clear_yes'));
        for (const x of hands) await call(Draft.deleteCell, { org: r.org, plan: r.plan, id: x.cell.id });
      }) }, t('place.clear')) : '', closeBtn())), 'place', k);
  }

  // ⚙: the plan's name, the weighting, the earlier drafts and the pin, delete.
  // «What matters»: the three priorities, the rules the planner always keeps, and the numbers under «Advanced»
  function prioritiesDialog() {
    keyed(modal(t('weights.title'), { info: [t('weights.hint')] },
      prioritiesForm({ org: r.org, plan: r.plan }, weights, (text) => { render().then(() => say(text)); }),
      h('h3', {}, t('prio.always')),
      h('ul', { class: 'small rules' }, ['takes', 'no', 'combo', 'pool', 'kept'].map((k) => h('li', {}, t('prio.rule.' + k)))),
      weightsForm({ org: r.org, plan: r.plan }, weights, (text) => { render().then(() => say(text)); }),
      foot('', closeBtn())), 'priorities', 0);
  }

  function settingsDialog() {
    let dlg;
    const name = h('input', { type: 'text', value: plan.name, maxlength: '80', 'aria-label': t('settings.name') });
    const runRows = runs.map((x) => {
      const onShow = !!view && x.id === view.run.id;
      return h('tr', { class: onShow ? 'shown' : false },
        h('td', {}, t('run.label', { id: x.id, time: fmtTime(x.finishedAt || x.startedAt) })),
        h('td', {}, x.status !== 'done' ? h('span', { class: 'tag' }, t('run.status.' + x.status)) : '', x.pinned ? h('span', { class: 'chip pin' }, t('run.current')) : ''),
        h('td', { class: 'right' }, onShow ? h('span', { class: 'small muted' }, t('run.on_show'))
          : linkBtn(t('run.show'), () => { planState.runId = x.id; dlg.close(); render(); }, t('run.show_aria', { id: x.id })),
        x.status === 'done' && !x.pinned ? [' · ', linkBtn(t('run.pin_short'), () => { dlg.close(); pin(x.id, true); }, t('run.pin_aria', { id: x.id }))] : ''));
    });
    dlg = keyed(modal(plan.name,
      h('form', { class: 'row', onsubmit: async (e) => {
        e.preventDefault();
        if (name.value.trim() !== plan.name && await write(() => call(Plan.renamePlan, { org: r.org, plan: r.plan, name: name.value }, 'plan'))) { dlg.close(); await render(); say(t('settings.renamed')); }
      } }, name, h('button', { type: 'submit' }, t('common.rename'))),
      h('h3', {}, t('weights.title')),
      h('p', { class: 'small' }, prioritySummary(weights), ' · ', linkBtn(t('prio.change'), () => { dlg.close(); prioritiesDialog(); })),
      h('h3', {}, t('run.history')),
      runs.length ? h('table', { class: 'mini' }, h('tbody', {}, runRows)) : h('p', { class: 'muted small' }, t('run.no_history')),
      foot(h('button', { type: 'button', class: 'danger', onclick: async () => {
        if (await write(async () => { await confirmed(t('settings.delete'), t('settings.delete_confirm', { name: plan.name }), t('common.delete')); await call(Plan.deletePlan, { org: r.org, plan: r.plan }); })) { dlg.close(); go(`/o/${enc(r.org)}`); say(t('settings.deleted', { name: plan.name })); }
      } }, t('settings.delete')), closeBtn())), 'settings', 0);
  }

  const listDialog = (title, items) => modal(title, h('ul', { class: 'small' }, items.map((x) => h('li', {}, x))), foot('', closeBtn()));

  // ---- the table
  const cols = events.length;
  const filler = () => h('td', { class: 'addcol' });
  // ⇥ at the end of a row: selects the whole row, as dragging over it would (Shift stretches, Ctrl/⌘ adds or takes away).
  const rowPick = (grid, id, name) => h('td', { class: 'addcol' }, h('button', { type: 'button', class: 'rowpick', title: t('sel.row'),
    'aria-label': t('sel.row_aria', { name }), onclick: (e) => selectLine(grid, e, (c) => c.row === id) }, icon('rowsel')));
  const sect = (key, title, info) => h('tr', { class: 'sect' }, h('th', { scope: 'row' }, h('div', { class: 'rl' }, foldBtn(key, title), title, info ? infoBtn(title, info) : '')),
    h('td', { colspan: String(cols + 1) }));
  const addLine = (form) => h('tr', { class: 'adds' }, h('th', { scope: 'row' }, form), h('td', { colspan: String(cols + 1) }));
  // The add lines are behind a small + at the end of the last name cell of their list (2026-11-23, the user: "Hide the add people
  // icon under a small plus symbol at the end of the last cells, where clicking it makes it a minus and collapse it"); an empty
  // list has it on its heading. Open, the + is a − and the line shows, ready to type; it stays open after adding (planState.adds,
  // for this plan while the page is open), Escape or − closes it. lines: the add lines it opens (the last list of people also
  // opens «Add a section»).
  const adds = (key, rows, host, lines) => {
    const open = planState.adds.has(key);
    const trs = lines.map((form) => {
      const tr = addLine(form);
      tr.hidden = !open;
      tr.classList.add('folded-add');
      tr.dataset.adds = key;
      const input = form.querySelector('input');
      input.addEventListener('keydown', (e) => { if (e.key === 'Escape') { e.preventDefault(); e.stopPropagation(); toggle(); } });
      return tr;
    });
    const what = lines[0].querySelector('input').placeholder;
    const plus = h('button', { type: 'button', class: 'addplus', 'data-adds': key, 'aria-expanded': String(open), title: open ? t('adds.close') : what,
      'aria-label': open ? t('adds.close') : what }, open ? '−' : '+');
    const toggle = () => {
      if (planState.adds.has(key)) planState.adds.delete(key); else planState.adds.add(key);
      const now = planState.adds.has(key);
      applyFolds(plus.closest('table'));
      plus.textContent = now ? '−' : '+';
      plus.title = now ? t('adds.close') : what;
      plus.setAttribute('aria-label', plus.title);
      plus.setAttribute('aria-expanded', String(now));
      if (now) trs[0].querySelector('input').focus(); else plus.focus();
    };
    plus.addEventListener('click', toggle);
    const last = rows.length ? rows[rows.length - 1] : host;
    const th = last.querySelector('th');
    th.classList.add('has-addplus');
    th.append(plus);
    if (open) setTimeout(() => { if (document.activeElement === document.body) trs[0].querySelector('input').focus({ preventScroll: true }); }, 0);
    return [...rows, ...trs];
  };

  const head = h('tr', {}, h('th', { scope: 'row', class: 'corner' }, t('table.date')),
    events.map((ev) => h('th', { scope: 'col' }, h('div', { class: 'rl' },
      h('button', { type: 'button', class: 'cellbtn', 'aria-label': t('table.event_aria', { label: label(ev) }), onclick: () => eventDialog(ev) }, fmtShortDate(ev.date)),
      h('button', { type: 'button', class: 'edit copy', title: t('copy.title'), 'aria-label': t('copy.aria', { label: label(ev) }), onclick: () => copyDialog(ev) }, icon('copy'))),
      clock(ev) ? h('span', { class: 'sub' }, clock(ev)) : '')),
    h('th', { scope: 'col', class: 'addcol' }, h('button', { type: 'button', class: 'ghost', 'aria-label': t('table.add_event'), title: t('table.add_event'), onclick: newEventDialog }, events.length ? '+' : '+ ' + t('table.add_event'))));  // with no event yet it says what it does

  // A text in a cell is saved when the field is left, without redrawing the page (so the next field keeps the focus).
  // A text field in the table. With values (a function: the texts used in the row so far, most used first) it suggests them
  // as you type, as the field for a name in the draft does: the best match that starts with what is typed is picked, so Enter
  // takes it (and moves on, as Enter always does); ↓ ↑ choose another, Escape or a click elsewhere closes the list. A text that
  // is one of them in other letters (case) is saved in the spelling already used, so the row stays consistent.
  const textCell = (cur, placeholder, aria, save, values) => {
    let value = cur;
    const input = h('input', { type: 'text', value, maxlength: '200', placeholder, 'aria-label': aria, autocomplete: 'off' });
    const td = h('td', { class: 'text' }, input);
    if (values) suggest(input, td, values);
    enterMovesOn(input);
    input.addEventListener('change', async () => {
      let v = input.value.trim();
      const same = values && values().find((x) => x.toLowerCase() === v.toLowerCase());
      if (same) v = same;
      if (v === value) { input.value = value; return; }
      if (await write(() => save(v))) value = v;
      input.value = value;
    });
    return td;
  };
  let suggestN = 0;
  const suggest = (input, td, values) => {
    const id = 'sugg-' + ++suggestN;
    const list = h('ul', { class: 'elist sugg', id, role: 'listbox', hidden: true });
    input.setAttribute('role', 'combobox');
    input.setAttribute('aria-autocomplete', 'list');
    input.setAttribute('aria-controls', id);
    input.setAttribute('aria-expanded', 'false');
    td.append(list);
    let hits: string[] = [], active = -1;
    const show = (on) => { list.hidden = !on; input.setAttribute('aria-expanded', String(on)); td.classList.toggle('suggesting', on); };
    const activate = (i) => {
      active = i;
      [...list.children].forEach((li, j) => li.setAttribute('aria-selected', String(j === i)));
      if (list.children[i]) { list.children[i].scrollIntoView({ block: 'nearest' }); input.setAttribute('aria-activedescendant', list.children[i].id); }
      else input.removeAttribute('aria-activedescendant');
    };
    const take = (v) => { input.value = v; show(false); };
    const fill = (all) => {
      const q = input.value.trim().toLowerCase();
      const known = values();
      hits = known.filter((v) => v.toLowerCase() !== q && (all || q) && v.toLowerCase().includes(q))
        .sort((a, b) => Number(!a.toLowerCase().startsWith(q)) - Number(!b.toLowerCase().startsWith(q))).slice(0, 8);
      list.replaceChildren(...hits.map((v, i) => h('li', { role: 'option', id: id + '-' + i, onmousedown: (e) => { e.preventDefault(); take(v); input.dispatchEvent(new Event('change')); } }, v)));
      show(hits.length > 0);
      // the best match is picked only when it starts with what is typed and what is typed is not a text of its own already used
      activate(q && hits.length && hits[0].toLowerCase().startsWith(q) && !known.some((v) => v.toLowerCase() === q) ? 0 : -1);
    };
    input.addEventListener('input', () => fill(false));
    // before Enter moves on (capture): Enter takes the picked text; Escape only closes the list
    input.addEventListener('keydown', (e) => {
      if (e.key === 'ArrowDown') { e.preventDefault(); if (list.hidden) fill(true); else activate(Math.min(hits.length - 1, active + 1)); }
      else if (e.key === 'ArrowUp' && !list.hidden) { e.preventDefault(); activate(Math.max(0, active - 1)); }
      else if (e.key === 'Enter' && !list.hidden && hits[active] !== undefined) take(hits[active]);
      else if (e.key === 'Escape' && !list.hidden) { e.preventDefault(); e.stopPropagation(); show(false); }
    }, true);
    input.addEventListener('blur', () => show(false));
  };
  // The texts of a details row, most used first (for its suggestions).
  const rowValues = (f) => () => {
    const n = new Map();
    for (const ev of events) { const v = fieldValue[ev.id + '/' + f.id]; if (v) n.set(v, (n.get(v) || 0) + 1); }
    return [...n.keys()].sort((a, b) => n.get(b) - n.get(a) || a.localeCompare(b, locale()));
  };
  // ⋮⋮ drags a row (a details row, a task, a person, a section). dropOn makes a row take what accept(kind, item) allows;
  // act(item) saves the move. dragRow is the usual case: a row of a kind takes the others of that kind at its position.
  // Moving rows (2026-11-23, the user: "Let the drag and drop of people easier"): with a mouse the whole name cell is the handle
  // (a click still opens; the drag starts after a few pixels), on a touch screen ⋮⋮. A line shows where the row lands, above or
  // below the row under the pointer by its half; on a heading (a section) it goes last in it. Near the edges the table scrolls on.
  // handleFor marks a row's ⋮⋮ (what is dragged); dropOn makes a row take what accept(kind, item) allows, act(item, after) saves.
  const grip = (title) => h('span', { class: 'grip', title, 'aria-hidden': 'true' }, '⋮⋮');
  const handleFor = (handle, kind, item) => { handle._drag = { kind, item }; };
  const dropOn = (row, accept, act, into = false) => { row._drop = { accept, act, into }; return row; };
  // A row of a list (details rows, tasks, sections): it takes the others of its kind, above or below it.
  const dragRow = (row, handle, kind, item, list, move) => {
    handleFor(handle, kind, item);
    return dropOn(row, (k, d) => k === kind && d.id !== item.id, (d, after) => {
      const others = list.filter((x) => x.id !== d.id);
      return move(d, others.findIndex((x) => x.id === item.id) + (after ? 1 : 0));
    });
  };
  let rowDrag: any = null;  // { kind, item, row, x, y, started, target: { tr, after } }
  const dropMarks = () => document.querySelectorAll('tr.drop').forEach((x) => x.classList.remove('drop'));
  const rowDragAt = (x, y) => {
    const d = rowDrag;
    d.ghost.style.left = x + 12 + 'px';
    d.ghost.style.top = y + 8 + 'px';
    dropMarks();
    d.line.hidden = true;
    d.target = null;
    const tr = document.elementFromPoint(x, y)?.closest('tr');
    if (!tr || tr === d.row || !tr._drop || !tr._drop.accept(d.kind, d.item)) return;
    const r = tr.getBoundingClientRect();
    const after = tr._drop.into || y > r.top + r.height / 2;
    d.target = { tr, after };
    if (tr._drop.into) { tr.classList.add('drop'); return; }
    const box = tr.closest('.grid-scroll')!.getBoundingClientRect();
    Object.assign(d.line.style, { left: box.left + 'px', width: Math.min(box.width, r.width) + 'px', top: (after ? r.bottom : r.top) - 1.5 + 'px' });
    d.line.hidden = false;
  };
  const rowDragScroll = () => {
    const d = rowDrag;
    if (!d || !d.started) return;
    const box = d.row.closest('.grid-scroll');
    const r = box.getBoundingClientRect();
    const head = box.querySelector('thead');
    const top = Math.max(r.top, 0) + (head ? head.offsetHeight : 0);
    const dy = d.y < top + 30 ? -16 : d.y > Math.min(r.bottom, innerHeight) - 30 ? 16 : 0;
    if (dy) { box.scrollTop += dy; rowDragAt(d.x, d.y); }
  };
  const rowDragMove = (e) => {
    const d = rowDrag;
    if (!d) return;
    d.x = e.clientX;
    d.y = e.clientY;
    if (!d.started) {
      if (Math.hypot(d.x - d.x0, d.y - d.y0) < 5) return;
      d.started = true;
      document.body.classList.add('row-dragging');
      d.row.classList.add('dragged');
      if (document.activeElement && (document.activeElement as HTMLElement).blur) (document.activeElement as HTMLElement).blur();
      d.ghost = document.body.appendChild(h('div', { class: 'row-ghost' }, d.item.name));
      d.line = document.body.appendChild(h('div', { class: 'drop-line', hidden: true }));
      d.timer = setInterval(rowDragScroll, 40);
    }
    e.preventDefault();
    rowDragAt(d.x, d.y);
  };
  const rowDragEnd = async (e) => {
    const d = rowDrag;
    rowDrag = null;
    removeEventListener('pointermove', rowDragMove);
    removeEventListener('pointerup', rowDragEnd);
    removeEventListener('pointercancel', rowDragEnd);
    removeEventListener('keydown', rowDragKey, true);
    if (!d || !d.started) return;
    clearInterval(d.timer);
    d.ghost.remove();
    d.line.remove();
    dropMarks();
    d.row.classList.remove('dragged');
    document.body.classList.remove('row-dragging');
    // the click that ends a drag opens nothing (and is let go at once if none comes)
    const swallow = (c) => { c.stopPropagation(); c.preventDefault(); };
    document.addEventListener('click', swallow, { capture: true, once: true });
    setTimeout(() => document.removeEventListener('click', swallow, { capture: true }), 0);
    if (e.type === 'pointerup' && d.target && await write(() => d.target.tr._drop.act(d.item, d.target.after, d.kind))) render();
  };
  const rowDragKey = (e) => { if (e.key === 'Escape' && rowDrag) { e.preventDefault(); e.stopPropagation(); rowDrag.target = null; rowDragEnd({ type: 'pointercancel' }); } };
  const onRowDown = (e) => {
    if (e.button !== 0 || rowDrag) return;
    const th = e.target.closest('th[scope=row]');
    const g = th && th.querySelector('.grip');
    if (!g || !g._drag) return;
    const onGrip = !!e.target.closest('.grip');
    if (e.pointerType !== 'mouse' && !onGrip) return;  // on a touch screen only ⋮⋮, so the table still scrolls
    if (!onGrip && e.target.closest('input, select, textarea, button.fold, button.auto, button.freeze, button.edit, button.addplus, .tags')) return;
    if (onGrip) e.preventDefault();
    rowDrag = { ...g._drag, row: th.closest('tr'), x0: e.clientX, y0: e.clientY, x: e.clientX, y: e.clientY, started: false };
    addEventListener('pointermove', rowDragMove, { passive: false });
    addEventListener('pointerup', rowDragEnd);
    addEventListener('pointercancel', rowDragEnd);
    addEventListener('keydown', rowDragKey, true);
  };
  // A heading's name as a field of its own (a details row, a section): Enter saves, Escape takes it back, empty deletes (asked).
  const nameField = (cur, aria, hint, rename, remove) => {
    const name = h('input', { type: 'text', class: 'fname', value: cur, maxlength: '80', 'aria-label': aria, title: hint });
    name.addEventListener('keydown', (e) => {
      if (e.key === 'Enter') { e.preventDefault(); name.blur(); }
      else if (e.key === 'Escape') { name.value = cur; name.blur(); }
    });
    name.addEventListener('change', async () => {
      const v = name.value.trim();
      if (v === cur) { name.value = cur; return; }
      if (await write(() => (v ? rename(v) : remove()))) render(); else name.value = cur;
    });
    return name;
  };
  // Details rows (2026-11-23, the user: "Remove the edit meny of about. Renaming can be done directly"): the name is a field
  // (emptied: the row is deleted, asked first), ⋮⋮ drags the row to another place, the pin freezes it ("similar to having a frozen
  // Row in excel": it stays under the dates while the table scrolls, for everyone), and the values suggest the texts of the row.
  const fieldRow = (f, i) => {
    const g = grip(t('field.drag'));
    const name = nameField(f.name, t('field.name_aria', { name: f.name }), t('field.name_hint'),
      (v) => call(Plan.renameField, { org: r.org, plan: r.plan, id: f.id, name: v }, 'field'),
      async () => { await confirmed(t('field.delete'), t('field.delete_confirm', { name: f.name }), t('common.delete')); return call(Plan.deleteField, { org: r.org, plan: r.plan, id: f.id }); });
    const pinName = t(f.frozen ? 'field.unfreeze' : 'field.freeze', { name: f.name });
    const pin = h('button', { type: 'button', class: 'freeze' + (f.frozen ? ' on' : ''), 'aria-pressed': String(!!f.frozen), title: pinName, 'aria-label': pinName,
      onclick: async () => { if (await write(() => call(Plan.freezeField, { org: r.org, plan: r.plan, id: f.id, frozen: !f.frozen }))) render(); } }, icon('pin'));
    const row = h('tr', { class: 'field-row' + (f.frozen ? ' frozen' : '') }, h('th', { scope: 'row' }, h('div', { class: 'rl' }, g, name, pin)),
    events.map((ev) => textCell(fieldValue[ev.id + '/' + f.id] || '', t('table.add_value', { name: f.name.toLowerCase() }), t('table.value_aria', { name: f.name, label: label(ev) }),
      async (v) => { await call(Plan.setFieldValue, { org: r.org, plan: r.plan, event: ev.id, field: f.id, value: v }); fieldValue[ev.id + '/' + f.id] = v.trim(); }, rowValues(f))), filler());
    return dragRow(row, g, 'field', f, fields.fields, (d, pos) => call(Plan.moveField, { org: r.org, plan: r.plan, id: d.id, position: pos }));
  };

  // Names straight in the cell (2026-11-23, the user: "Let it be possible to click a planning cell to easily add names. Show
  // autocompletions. Adding a name that is not a person makes it a custom name. Add a plus to make it possible to add more names. We allow
  // adding more people then we have spots." … "No need to warn if adding more people then required … it is a consious choice"): a plain
  // click on a draft cell (or its +) opens a field with suggestions. Those who said yes and take the task come first, those who said no
  // last; one the planner put there is locked when picked; text that is no one becomes a name of its own. After each change the page is
  // drawn again and the field opens again in the same cell, for the next name. × takes away a name set by hand. Escape or a click
  // outside closes it.
  const levelRank = { good: 0, poor: 1, '': 2, never: 3 };
  const taskRank = { good: 0, poor: 1, '': 2, never: 3 };
  const editCell = (ev, tk, td, typed = '') => {
    const k = at(ev, tk);
    planState.edit = k;
    const s = slotsAt(ev, tk);
    // what is typed while the page is drawn again goes into the new field (editInput: the field of the cell open now)
    const again = async (action) => { input.value = ''; if (await write(action)) { planState.edit = k; await render(); } };
    const chips = s.items.map((x) => h('span', { class: 'echip' + (x.hand ? ' hand' : '') + (x.locked ? ' locked' : '') },
      x.locked ? mark('lock') : '', h('span', {}, itemText(x)),
      x.hand ? h('button', { type: 'button', class: 'x', 'aria-label': t('place.remove_aria', { name: x.name }), title: t('common.remove'),
        onclick: () => again(async () => { await call(Draft.deleteCell, { org: r.org, plan: r.plan, id: x.cell.id });
          undoable(t('undo.remove', { name: x.name }), () => call(Draft.addCell, { org: r.org, plan: r.plan, ...(cellBack(x.cell)) }, 'cell')); }) }, '×') : ''));
    const taken = new Set(s.items.filter((x) => x.cell && x.kind === 'person').map((x) => x.id));
    const fromPlanner = new Set(s.items.filter((x) => !x.cell && x.kind === 'person').map((x) => x.id));
    const taskLevel = (p) => ((chosen[p.id] || []).find((x) => x.taskId === tk.id) || {}).level || '';
    const level = (p) => (ans[p.id] || {})[ev.id] || '';
    const candidates = people.filter((p) => !taken.has(p.id))
      .sort((a, b) => levelRank[level(a)] - levelRank[level(b)] || taskRank[taskLevel(a)] - taskRank[taskLevel(b)] || a.name.localeCompare(b.name, locale()));
    const input = h('input', { type: 'text', class: 'ename', maxlength: '80', placeholder: t('cell.placeholder'), autocomplete: 'off', role: 'combobox',
      'aria-label': t('cell.add_aria', { task: tk.name, label: label(ev) }), 'aria-autocomplete': 'list', 'aria-expanded': 'true', 'aria-controls': 'cell-list' });
    const list = h('ul', { class: 'elist', id: 'cell-list', role: 'listbox' });
    let options: { el: HTMLLIElement; add: () => unknown }[] = [];
    let active = -1;
    const activate = (i) => {
      active = i;
      options.forEach((o, j) => o.el.setAttribute('aria-selected', String(j === i)));
      if (options[i]) { options[i].el.scrollIntoView({ block: 'nearest' }); input.setAttribute('aria-activedescendant', options[i].el.id); }
    };
    const added = (name, body) => again(async () => { const c = await call(Draft.addCell, { org: r.org, plan: r.plan, ...body }, 'cell');
      undoable(t('undo.add', { name }), () => call(Draft.deleteCell, { org: r.org, plan: r.plan, id: c.id })); });
    const addPerson = (p) => added(p.name, { event: ev.id, task: tk.id, person: p.id, ...(fromPlanner.has(p.id) ? { locked: true } : {}) });
    const addText = (text) => added(text, { event: ev.id, task: tk.id, label: text, fills: 1 });
    const fill = () => {
      const q = input.value.trim().toLowerCase();
      const hits = candidates.filter((p) => p.name.toLowerCase().includes(q))
        .sort((a, b) => (q ? Number(!a.name.toLowerCase().startsWith(q)) - Number(!b.name.toLowerCase().startsWith(q)) : 0)).slice(0, 12);
      options = hits.map((p, i) => {
        const l = level(p), tl = taskLevel(p);
        const el = h('li', { role: 'option', id: 'cell-opt-' + i, class: l === 'never' ? 'no' : '', onmousedown: (e) => { e.preventDefault(); addPerson(p); } },
          h('span', { class: 'ans ' + (l || 'none') }, l ? icon(LEVEL_ICON[l]) : ''), h('span', { class: 'nm' }, p.name),
          h('span', { class: 'why small' }, fromPlanner.has(p.id) ? t('cell.in_draft') : l === 'never' ? t('cell.said_no') : tl && tl !== 'never' ? tk.name + ': ' + t('pick.' + tl) : ''));
        return { el, add: () => addPerson(p) };
      });
      const exact = people.some((p) => p.name.toLowerCase() === q);
      if (q && !exact) {
        const text = input.value.trim();
        const el = h('li', { role: 'option', id: 'cell-opt-custom', class: 'custom', onmousedown: (e) => { e.preventDefault(); addText(text); } }, '+ ' + t('cell.custom', { text }));
        options.push({ el, add: () => addText(text) });
      }
      list.replaceChildren(...options.map((o) => o.el));
      list.hidden = !options.length;
      activate(q && options.length ? 0 : -1);  // what is typed: the best match is picked by Enter (the name of its own when nothing matches)
    };
    input.addEventListener('input', fill);
    input.addEventListener('keydown', (e) => {
      if (e.key === 'ArrowDown') { e.preventDefault(); activate(Math.min(options.length - 1, active + 1)); }
      else if (e.key === 'ArrowUp') { e.preventDefault(); activate(Math.max(0, active - 1)); }
      else if (e.key === 'Enter') { e.preventDefault(); if (options[active]) options[active].add(); }
      else if (e.key === 'Escape') { e.preventDefault(); e.stopPropagation(); close(); }
    });
    const addBtn = h('button', { type: 'button', class: 'eadd', title: t('cell.add'), 'aria-label': t('cell.add'),
      onmousedown: (e) => e.preventDefault(), onclick: () => { if (options[active]) options[active].add(); else input.focus(); } }, '+');
    const box = h('div', { class: 'cell-edit' }, h('div', { class: 'echips' }, chips), h('div', { class: 'erow' }, input, addBtn), list);
    const close = () => { if (planState.edit === k) planState.edit = null; if (box.isConnected) putDraftCell(ev, tk); };
    box.addEventListener('focusout', (e) => { if (!box.contains(e.relatedTarget as Node | null)) setTimeout(() => { if (box.isConnected && !box.contains(document.activeElement)) close(); }, 0); });
    td.classList.add('editing');
    td.replaceChildren(box);
    planState.editInput = input;
    input.value = typed;
    fill();
    input.focus();
  };
  // The draft's cells, so a change of places can redraw the cell below without redrawing the page.
  const draftTd = {};
  // A place in the draft: the names, each of the planner's with a lock button (locked: kept by the next draft), and ⋯ for
  // the place's dialog, + (or a plain click on the cell) for a name (editCell); dragging over cells selects them (Lock / Unlock in the bar).
  const draftCell = (ev, tk) => {
    const s = slotsAt(ev, tk);
    const edit = org.level !== 'view';
    const plus = () => h('button', { type: 'button', class: 'addname', title: t('cell.add'), 'aria-label': t('cell.add_aria', { task: tk.name, label: label(ev) }),
      onclick: (e) => editCell(ev, tk, e.currentTarget.closest('td')) }, '+');
    const clickable = (td) => {
      if (edit) td.addEventListener('click', (e) => { if (!e.ctrlKey && !e.metaKey && !e.shiftKey && !e.target.closest('button, input, .cell-edit')) editCell(ev, tk, td); });
      return td;
    };
    if (!s.n && !s.items.length) return clickable(h('td', { class: 'draft none' }, h('div', { class: 'dcell' }, h('div', { class: 'dlines' }, h('span', { class: 'muted small', 'aria-hidden': 'true' }, '–')),
      edit ? h('div', { class: 'dside' }, plus()) : '')));
    const open = () => placeDialog(ev, tk);
    const was = comparing ? before[at(ev, tk)] || [] : null;
    const isNew = (x) => was && x.kind === 'person' && !x.hand && !was.some((p) => p.id === x.id);
    const line = (x) => { const el = line0(x); if (isNew(x)) el.classList.add('new'); return el; };
    const line0 = (x) => {
      if (!edit || x.hand || x.kind !== 'person') return slot(x);
      const name = t(x.locked ? 'place.unlock_aria' : 'place.lock_aria', { name: x.name });
      return h('span', { class: 'slot' + (x.locked ? ' locked' : '') },
        h('button', { type: 'button', class: 'lockbtn' + (x.locked ? ' on' : ''), 'aria-pressed': String(!!x.locked), title: name, 'aria-label': name,
          onclick: () => setLocks([{ event: ev.id, task: tk.id, person: x.id }], !x.locked) }, icon(x.locked ? 'lock' : 'unlock')),
        h('span', { class: 'nm' }, itemText(x)), x.weak ? h('span', { class: 'chip poor' }, t('draft.maybe')) : '');
    };
    const lines = s.items.map(line);
    if (s.open && done) for (let i = 0; i < s.open; i++) lines.push(h('span', { class: 'slot empty' }, t('draft.not_filled'), s.why ? [' · ', h('button', { type: 'button', class: 'why link-button', onclick: open }, t('draft.why'))] : ''));
    else if (s.open) lines.push(h('span', { class: 'slot open' }, tn('draft.open', s.open)));
    const pool = poolAt[at(ev, tk)];
    const td = h('td', { class: 'draft' }, h('div', { class: 'dcell' }, h('div', { class: 'dlines' }, lines),
      h('div', { class: 'dside' },
        h('button', { type: 'button', class: 'dmenu', title: t('draft.open_place'), 'aria-label': t('draft.place_aria', { task: tk.name, label: label(ev) }), onclick: open }, icon('more')),
        edit ? plus() : '',
        pool && pool.people.length ? h('span', { class: 'mark pool', role: 'img', title: t('draft.pool_mark'), 'aria-label': t('draft.pool_mark') }, icon('filter')) : '')));
    if (was && (s.items.some(isNew) || was.some((p) => !s.items.some((x) => x.kind === 'person' && x.id === p.id)))) {
      td.classList.add('changed');
      td.title = t('draft.before', { id: olderId, names: was.map((p) => p.name).join(', ') || '–' });
    }
    if (done) reg('draft', tk.id, ev, td, () => putDraftCell(ev, tk));
    return clickable(td);
  };
  const putDraftCell = (ev, tk) => { const td = draftCell(ev, tk); if (draftTd[at(ev, tk)]) draftTd[at(ev, tk)].replaceWith(td); draftTd[at(ev, tk)] = td; return td; };

  // ---- selecting cells like in a spreadsheet (2026-11-22, the user: "drag and drop, and unselect selected cells + mark a
  // whole Row"): drag over cells, Shift+click to stretch, Ctrl/⌘+click to add or take away one, ⇥ at the end of a row for
  // the whole row (⋮⋮ only drags), a click on an event's count for everyone that day. Two grids: places (tasks × events) and answers (people ×
  // events); a selection is in one of them. A bar at the bottom sets every selected cell at once (one request); Escape
  // or ✕ lets go. Hidden rows (folded, filtered) are never selected. Dragging needs a mouse; a touch scrolls the table.
  const grids = { places: { rows: [], cells: new Map() }, answers: { rows: [], cells: new Map() }, draft: { rows: [], cells: new Map() } };
  const evIndex = new Map<number, number>(events.map((ev, i) => [ev.id, i]));
  let sel: { grid: string; keys: Set<string>; anchor: At; base: Set<string> } | null = null;  // keys: 'row/event'
  const selbar = h('div', { class: 'selbar', hidden: true, role: 'region', 'aria-label': t('sel.bar') });
  const cellKey = (row, ev) => row + '/' + ev;
  const reg = (grid, row, ev, td, paint) => {
    if (org.level === 'view') return;
    const g = grids[grid];
    if (!g.rows.includes(row)) g.rows.push(row);
    g.cells.set(cellKey(row, ev.id), { td, row, ev, paint });
    td.dataset.cell = grid + '|' + row + '|' + ev.id;
  };
  const rowShown = (grid, row) => { const c = grids[grid].cells.get(cellKey(row, events[0] && events[0].id)); return !c || !c.td.closest('tr').hidden; };
  const paintSel = () => {
    document.querySelectorAll('td.sel').forEach((td) => td.classList.remove('sel'));
    if (sel) sel.keys.forEach((k) => { const c = grids[sel!.grid].cells.get(k); if (c) c.td.classList.add('sel'); });
    drawBar();
  };
  const unselect = () => { sel = null; paintSel(); };
  const rect = (grid, a, b) => {
    const rows = grids[grid].rows;
    const [r0, r1] = [rows.indexOf(a.row), rows.indexOf(b.row)].sort((x, y) => x - y);
    const [c0, c1] = [evIndex.get(a.ev)!, evIndex.get(b.ev)!].sort((x, y) => x - y);
    const keys = new Set<string>();
    for (let i = r0; i <= r1; i++) if (rowShown(grid, rows[i])) for (let j = c0; j <= c1; j++) keys.add(cellKey(rows[i], events[j].id));
    return keys;
  };
  const selectRect = (grid, anchor, to, add) => {
    const keys = rect(grid, anchor, to);
    if (add && sel && sel.grid === grid) sel.base.forEach((k) => keys.add(k));
    sel = { grid, keys, anchor, base: add && sel && sel.grid === grid ? sel.base : new Set() };
    paintSel();
  };
  const toggleCell = (grid, row, ev) => {
    if (!sel || sel.grid !== grid) sel = { grid, keys: new Set(), anchor: { row, ev }, base: new Set() };
    const k = cellKey(row, ev);
    if (sel.keys.has(k)) sel.keys.delete(k); else sel.keys.add(k);
    sel.anchor = { row, ev };
    sel.base = new Set(sel.keys);
    if (!sel.keys.size) sel = null;
    paintSel();
  };
  // A whole row (⋮⋮), or a whole event column (its count): Shift stretches from the last one, Ctrl/⌘ adds or takes away.
  const selectLine = (grid, e, pick) => {
    const keys = new Set([...grids[grid].cells.keys()].filter((k) => pick(grids[grid].cells.get(k)) && rowShown(grid, grids[grid].cells.get(k).row)));
    if (!keys.size) return;
    const first = grids[grid].cells.get([...keys][0]);
    if ((e.ctrlKey || e.metaKey) && sel && sel.grid === grid) {
      const all = [...keys].every((k) => sel!.keys.has(k));
      keys.forEach((k) => (all ? sel!.keys.delete(k) : sel!.keys.add(k)));
      sel.base = new Set(sel.keys);
      if (!sel.keys.size) sel = null;
    } else if (e.shiftKey && sel && sel.grid === grid) {
      const last = grids[grid].cells.get([...keys].pop());
      sel.keys = rect(grid, sel.anchor, { row: last.row, ev: events[events.length - 1].id });
      rect(grid, sel.anchor, { row: first.row, ev: events[0].id }).forEach((k) => sel!.keys.add(k));
      sel.base = new Set(sel.keys);
    } else {
      sel = { grid, keys, anchor: { row: first.row, ev: first.ev.id }, base: new Set(keys) };
    }
    paintSel();
  };
  // Dragging: from the cell pressed to the cell under the pointer; near an edge the table scrolls on.
  let drag: { grid: string; from: At; moved: boolean; x: number; y: number; add: boolean } | null = null;
  const cellAt = (x, y) => {
    const td = document.elementFromPoint(x, y)?.closest<HTMLElement>('td[data-cell]');
    if (!td) return null;
    const [grid, row, ev] = td.dataset.cell!.split('|');
    return { grid, row: Number(row), ev: Number(ev) };
  };
  const dragFollow = () => {
    if (!drag) return;
    const c = cellAt(drag.x, drag.y);
    if (!c || c.grid !== drag.grid) return;
    if (!drag.moved && c.row === drag.from.row && c.ev === drag.from.ev) return;
    if (!drag.moved) { drag.moved = true; document.body.classList.add('selecting'); if (document.activeElement && (document.activeElement as HTMLElement).blur) (document.activeElement as HTMLElement).blur(); }
    selectRect(drag.grid, drag.from, c, drag.add);
  };
  let scroller: ReturnType<typeof setInterval> | null = null;
  const autoScroll = () => {
    if (!drag || !drag.moved) return;
    const box = document.querySelector('.grid-scroll.main');
    if (!box) return;
    const r = box.getBoundingClientRect();
    const edge = 40, step = 24;
    const dx = drag.x > r.right - edge ? step : drag.x < r.left + 160 + edge ? -step : 0;
    const dy = drag.y > Math.min(r.bottom, innerHeight) - edge ? step : drag.y < Math.max(r.top, 0) + 80 ? -step : 0;
    if (dx) box.scrollLeft += dx;
    if (dy) box.scrollTop += dy;
    if (dx || dy) dragFollow();
  };
  const onMove = (e) => { if (drag) { drag.x = e.clientX; drag.y = e.clientY; dragFollow(); } };
  const onUp = () => {
    if (!drag) return;
    const moved = drag.moved;
    drag = null;
    clearInterval(scroller!);
    document.body.classList.remove('selecting');
    if (moved) document.addEventListener('click', (e) => { e.stopPropagation(); e.preventDefault(); }, { capture: true, once: true });
  };
  const onCellDown = (e) => {
    if (e.pointerType !== 'mouse' || e.button !== 0) return;
    const td = e.target.closest('td[data-cell]');
    if (!td || e.target.closest('.note-tab')) return;
    const [grid, row, ev] = td.dataset.cell.split('|').map((x, i) => (i ? Number(x) : x));
    if (e.ctrlKey || e.metaKey) { e.preventDefault(); toggleCell(grid, row, ev); swallowClick(); return; }
    if (e.shiftKey && sel && sel.grid === grid) { e.preventDefault(); selectRect(grid, sel.anchor, { row, ev }, false); swallowClick(); return; }
    if (sel) unselect();
    drag = { grid, from: { row, ev }, moved: false, x: e.clientX, y: e.clientY, add: false };
    clearInterval(scroller!);
    scroller = setInterval(autoScroll, 40);
  };
  const swallowClick = () => document.addEventListener('click', (e) => { e.stopPropagation(); e.preventDefault(); }, { capture: true, once: true });
  const onKey = (e) => {
    if (!sel || !selbar.isConnected) return;
    if (document.querySelector('dialog[open]')) return;
    const typing = e.target.closest && e.target.closest('input, textarea, select');
    if (e.key === 'Escape') { unselect(); return; }
    if (typing && !e.target.closest('.selbar')) return;
    if (sel.grid === 'draft') return;  // the draft's bar has only Lock and Unlock
    if (e.key === 'Delete' || e.key === 'Backspace') { e.preventDefault(); applySel(sel.grid === 'answers' ? '' : 0); return; }
    if (sel.grid === 'answers' && ['1', '2', '3'].includes(e.key) && !typing) { e.preventDefault(); applySel({ 1: 'good', 2: 'poor', 3: 'never' }[e.key]); }
    if (sel.grid === 'places' && /^[0-9]$/.test(e.key) && !typing) { e.preventDefault(); const n = selbar.querySelector('input'); if (n) { n.focus(); n.value = e.key; } }
  };
  document.addEventListener('pointermove', onMove);
  document.addEventListener('pointerup', onUp);
  document.addEventListener('keydown', onKey);
  // the listeners go with the page: a new page has its own
  const offSel = () => { document.removeEventListener('pointermove', onMove); document.removeEventListener('pointerup', onUp); document.removeEventListener('keydown', onKey); };
  const watchGone = setInterval(() => { if (!selbar.isConnected && document.querySelector('#main > *')) { offSel(); clearInterval(watchGone); } }, 2000);

  // Setting the selection: shown at once, one request, taken back if the server refuses.
  const applySel = async (value) => {
    if (!sel) return;
    const g = grids[sel.grid];
    const cells = [...sel.keys].map((k) => g.cells.get(k)).filter(Boolean);
    if (sel.grid === 'draft') {
      const on = value === 'lock';
      await setLocks(lockItems(cells.map((c) => [c.ev, tasks.find((tk) => tk.id === c.row)]).filter((x) => x[1]), on), on);
      return;
    }
    if (sel.grid === 'answers') {
      const was = cells.map((c) => (ans[c.row] || {})[c.ev.id] || '');
      const items = cells.filter((c, i) => was[i] !== value).map((c) => ({ person: c.row, event: c.ev.id, level: value }));
      if (!items.length) return;
      const paint = () => { cells.forEach((c) => c.paint()); new Set(cells.map((c) => c.ev.id)).forEach(recount); };
      cells.forEach((c) => { (ans[c.row] ||= {})[c.ev.id] = value; });
      paint();
      if (!(await write(() => call(Plan.setAnswers, { org: r.org, plan: r.plan, answers: items })))) { cells.forEach((c, i) => { ans[c.row][c.ev.id] = was[i]; }); paint(); }
    } else {
      const n = Number(value) || 0;
      const was = cells.map((c) => placesAt[at(c.ev, { id: c.row })] || 0);
      const items = cells.filter((c, i) => was[i] !== n).map((c) => ({ event: c.ev.id, task: c.row, slots: n }));
      if (!items.length) return;
      const set = (c, v) => { const k = at(c.ev, { id: c.row }); if (v) placesAt[k] = v; else delete placesAt[k]; };
      const paint = () => cells.forEach((c) => { c.paint(); putDraftCell(c.ev, tasks.find((tk) => tk.id === c.row)); });
      cells.forEach((c) => set(c, n));
      paint();
      if (!(await write(() => call(Plan.setPlacesMany, { org: r.org, plan: r.plan, places: items })))) { cells.forEach((c, i) => set(c, was[i])); paint(); }
    }
  };
  const drawBar = () => {
    selbar.hidden = !sel;
    if (!sel) { selbar.replaceChildren(); return; }
    const n = sel.keys.size;
    const close = h('button', { type: 'button', class: 'ghost icon', title: t('sel.unselect'), 'aria-label': t('sel.unselect'), onclick: unselect }, icon('x'));
    const count = h('span', { class: 'n' }, tn('sel.count', n));
    if (sel.grid === 'answers') {
      const lv = (level, key) => h('button', { type: 'button', class: 'ans', 'data-level': level, title: t('answer.' + level) + ' (' + key + ')', 'aria-label': t('sel.set_answer', { answer: t('answer.' + level) }),
        onclick: () => applySel(level) }, answerMark(level));
      selbar.replaceChildren(count, lv('good', '1'), lv('poor', '2'), lv('never', '3'),
        h('button', { type: 'button', class: 'ghost sm', title: t('sel.clear') + ' (Delete)', onclick: () => applySel('') }, t('sel.clear')), close);
    } else if (sel.grid === 'draft') {
      selbar.replaceChildren(count, h('button', { type: 'button', class: 'sm', onclick: () => applySel('lock') }, icon('lock'), ' ' + t('sel.lock')),
        h('button', { type: 'button', class: 'ghost sm', onclick: () => applySel('unlock') }, icon('unlock'), ' ' + t('sel.unlock')), close);
    } else {
      const num = h('input', { type: 'number', min: '0', max: '50', inputmode: 'numeric', 'aria-label': t('sel.places_aria'), placeholder: '#' });
      const form = h('form', { class: 'row-inline', onsubmit: (e) => { e.preventDefault(); applySel(parseInt(num.value || '0', 10)); } },
        num, h('button', { type: 'submit', class: 'sm' }, t('sel.set')));
      selbar.replaceChildren(count, form, h('button', { type: 'button', class: 'ghost sm', title: t('sel.clear') + ' (Delete)', onclick: () => applySel(0) }, t('sel.clear')), close);
    }
  };

  // Places: how many each event needs, typed straight in; empty = not needed. Only a real change is sent.
  const placesCell = (tk, ev) => {
    const k = at(ev, tk);
    const input = enterMovesOn(h('input', { type: 'number', min: '0', max: '50', inputmode: 'numeric', 'aria-label': t('table.places_aria', { task: tk.name, label: label(ev) }) }));
    const paint = () => { const n = placesAt[k] || 0; input.value = n ? String(n) : ''; input.classList.toggle('set', n > 0); };
    input.addEventListener('change', async () => {
      const raw = input.value.trim();
      const n = raw === '' ? 0 : parseInt(raw, 10);
      const cur = placesAt[k] || 0;
      if (Number.isNaN(n) || n === cur) { paint(); return; }
      if (await write(() => (n === 0 ? call(Plan.resetPlaces, { org: r.org, plan: r.plan, event: ev.id, task: tk.id }) : call(Plan.setPlaces, { org: r.org, plan: r.plan, event: ev.id, task: tk.id, slots: n })))) { if (n) placesAt[k] = n; else delete placesAt[k]; putDraftCell(ev, tk); }
      paint();
    });
    paint();
    const td = h('td', { class: 'places' }, input);
    reg('places', tk.id, ev, td, paint);
    return td;
  };
  // Tasks: ⋮⋮ drags a task to another row (the organisation's order, the same in every plan); the name opens it.
  const taskRow = (tk, i) => {
    const g = grip(t('task.drag'));
    // ⚡: whether the planner fills the task (off: only what is set by hand); the draft and the titles change with it
    const zap = zapBtn(tk, 'planned', (on) => t(on ? 'task.auto_on' : 'task.auto_off', { name: tk.name }),
      t('task.planned_aria', { name: tk.name }), (on) => call(Setup.updateTask, { org: r.org, id: tk.id, planned: on }), () => render());
    const row = h('tr', { class: 'task-row' }, h('th', { scope: 'row' }, h('div', { class: 'rl' }, g,
      h('button', { type: 'button', class: 'cellbtn', 'aria-label': t('table.task_aria', { name: tk.name }), onclick: () => taskDialog(tk) }, tk.name), zap)),
    events.map((ev) => placesCell(tk, ev)), rowPick('places', tk.id, tk.name));
    handleFor(g, 'task', tk);
    return dropOn(row, (k, d) => k === 'task' && d.id !== tk.id, (d, after) => {
      const others = tasksIn(tk.section).filter((x) => x.id !== d.id);
      return moveTask(d, tk.section, others.findIndex((x) => x.id === tk.id) + (after ? 1 : 0));
    });
  };

  // Availability: one click on a cell moves the answer on; the count above each event follows.
  // The count is a bar (2026-11-23, the user: "make the summed thumbs up, down, question mark be a simple cake diagram or something
  // similar. Show the real numbers on hover … useful when just having a quick glance"; a bar, with those not answered): green,
  // yellow, red and grey in proportion, the numbers in a tip on hover (at once, not the browser's slow title) and for screen readers.
  const countCells = {};
  const tallyTip = h('div', { class: 'tally-tip', hidden: true, role: 'tooltip' });
  const showTip = (td) => {
    tallyTip.textContent = td.dataset.tip || '';
    tallyTip.hidden = false;
    const b = td.getBoundingClientRect();
    tallyTip.style.left = Math.max(8, Math.min(b.left + b.width / 2 - tallyTip.offsetWidth / 2, innerWidth - tallyTip.offsetWidth - 8)) + 'px';
    tallyTip.style.top = (b.top - tallyTip.offsetHeight - 6 > 8 ? b.top - tallyTip.offsetHeight - 6 : b.bottom + 6) + 'px';
  };
  const countCell = (ev) => {
    const td = h('td', { class: 'count small', title: t('sel.column_hint'), onclick: (e) => selectLine('answers', e, (c) => c.ev.id === ev.id) });
    td.addEventListener('pointerenter', () => showTip(td));
    td.addEventListener('pointerleave', () => { tallyTip.hidden = true; });
    return (countCells[ev.id] = td);
  };
  const recount = (evId) => {
    const c = { good: 0, poor: 0, never: 0, none: 0 };
    people.forEach((p) => { const l = (ans[p.id] || {})[evId]; c[l || 'none']++; });
    const seg = (k) => { if (!c[k]) return ''; const s = h('span', { class: 'seg ' + k }); s.style.flexGrow = String(c[k]); return s; };
    const td = countCells[evId];
    td.replaceChildren(h('div', { class: 'tally' }, ['good', 'poor', 'never', 'none'].map(seg)));
    td.dataset.tip = t('table.count', c);
    td.setAttribute('aria-label', td.dataset.tip);
    if (!tallyTip.hidden && td.matches(':hover')) showTip(td);
  };
  // A note on an answer (2026-11-22, the user: "notat/kommentar i hver celle … vises som en flik i hjørnet når du klikker eller
  // på hover"): a tab in the cell's corner. Hovering a cell with a note shows it; a click on the tab opens it, to write in it
  // (or only to read it, with a view link). One popover for the whole page.
  const notePop = h('div', { class: 'note-pop', hidden: true, role: 'dialog' });
  let noteOpen: { td: HTMLElement; edit: boolean } | null = null;  // the popover on show
  const placePop = (td) => {
    const r = td.getBoundingClientRect();
    notePop.hidden = false;
    const w = notePop.offsetWidth, hgt = notePop.offsetHeight;
    const left = Math.max(8, Math.min(r.right - 8, innerWidth - w - 8));
    const top = r.top - hgt - 6 > 8 ? r.top - hgt - 6 : Math.min(r.bottom + 6, innerHeight - hgt - 8);
    notePop.style.left = left + 'px';
    notePop.style.top = top + 'px';
  };
  const closeNote = () => { notePop.hidden = true; notePop.replaceChildren(); noteOpen = null; };
  const peekNote = (td, p, ev) => {
    if (noteOpen && noteOpen.edit) return;
    const text = (notes[p.id] || {})[ev.id];
    if (!text) return;
    notePop.replaceChildren(h('div', { class: 'note-who small muted' }, p.name + ' · ' + label(ev)), h('div', { class: 'note-text' }, text));
    notePop.classList.remove('editing');
    noteOpen = { td, edit: false };
    placePop(td);
  };
  const openNote = (td, p, ev, repaint) => {
    const text = (notes[p.id] || {})[ev.id] || '';
    const head = h('div', { class: 'note-who small muted' }, p.name + ' · ' + label(ev));
    if (org.level === 'view') { if (text) peekNote(td, p, ev); return; }
    const area = h('textarea', { rows: '3', maxlength: '300', placeholder: t('note.placeholder'), 'aria-label': t('note.aria', { name: p.name, label: label(ev) }) });
    area.value = text;
    const save = async (value) => {
      const was = text;
      value = value.trim();
      closeNote();
      if (value === was) return;
      if (value) (notes[p.id] ||= {})[ev.id] = value; else if (notes[p.id]) delete notes[p.id][ev.id];
      repaint();
      if (!(await write(() => call(Plan.setNote, { org: r.org, plan: r.plan, event: ev.id, person: p.id, note: value })))) {
        if (was) (notes[p.id] ||= {})[ev.id] = was; else if (notes[p.id]) delete notes[p.id][ev.id];
        repaint();
      }
    };
    area.addEventListener('keydown', (e) => {
      if (e.key === 'Escape') { e.preventDefault(); e.stopPropagation(); closeNote(); }
      if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) { e.preventDefault(); save(area.value); }
    });
    notePop.replaceChildren(head, area, h('div', { class: 'row-inline' },
      h('button', { type: 'button', class: 'sm', onclick: () => save(area.value) }, t('common.save')),
      text ? h('button', { type: 'button', class: 'ghost sm', onclick: () => save('') }, t('note.delete')) : '',
      h('button', { type: 'button', class: 'ghost sm', onclick: closeNote }, t('note.cancel'))));
    notePop.classList.add('editing');
    noteOpen = { td, edit: true };
    placePop(td);
    area.focus();
  };
  const onNoteAway = (e) => {
    if (!noteOpen || !noteOpen.edit) return;
    if (!notePop.isConnected) { document.removeEventListener('pointerdown', onNoteAway, true); return; }
    if (!notePop.contains(e.target) && !e.target.closest('.note-tab')) closeNote();
  };
  document.addEventListener('pointerdown', onNoteAway, true);
  const noteTab = (td, p, ev, repaint) => {
    const tab = h('button', { type: 'button', class: 'note-tab', onclick: (e) => {
      e.stopPropagation();
      if (noteOpen && noteOpen.td === td && noteOpen.edit) closeNote(); else openNote(td, p, ev, repaint);
    } });
    td.addEventListener('mouseenter', () => peekNote(td, p, ev));
    td.addEventListener('mouseleave', () => { if (noteOpen && !noteOpen.edit && noteOpen.td === td) closeNote(); });
    return tab;
  };
  const answerCell = (p, ev) => {
    const b = h('button', { type: 'button', class: 'ans' });
    let saving = Promise.resolve();  // the clicks' writes, one after the other: every click counts, in the order made
    const paint = () => {
      const l = (ans[p.id] || {})[ev.id] || '';
      b.replaceChildren(answerMark(l));
      b.dataset.level = l || 'none';
      b.setAttribute('aria-label', t('table.answer_aria', { name: p.name, label: label(ev), answer: t('answer.' + (l || 'none')) }));
    };
    b.addEventListener('click', () => {
      const was = (ans[p.id] || {})[ev.id] || '';
      const next = ANSWER_NEXT[was];
      (ans[p.id] ||= {})[ev.id] = next;  // shown at once, taken back if the server refuses
      paint();
      recount(ev.id);
      saving = saving.then(async () => {
        if (!(await write(() => call(Plan.setAvailability, { org: r.org, plan: r.plan, person: p.id, levels: { [ev.id]: next } })))) { ans[p.id][ev.id] = was; paint(); recount(ev.id); }
      });
    });
    paint();
    const td = h('td', { class: 'answer' }, b);
    const paintNote = () => {
      const text = (notes[p.id] || {})[ev.id] || '';
      td.classList.toggle('has-note', !!text);
      tab.title = text || t('note.add');
      tab.setAttribute('aria-label', text ? t('note.open_aria', { name: p.name, label: label(ev) }) : t('note.add_aria', { name: p.name, label: label(ev) }));
    };
    const tab = noteTab(td, p, ev, paintNote);
    if (org.level !== 'view' || (notes[p.id] || {})[ev.id]) td.append(tab);
    paintNote();
    reg('answers', p.id, ev, td, paint);
    return td;
  };
  // People: ⋮⋮ drags a person to another row, also into another section (or onto its heading: the top of it). Sections are
  // headings with their people below, the organisation's own; those without a section come first. Only placement.
  const inSection = (id) => people.filter((p) => p.section === id);
  const tasksIn = (id) => tasks.filter((x) => x.section === id);
  // A heading of a task section (2026-11-23, the user: "Make it possible to have some sections for the tasks as well. As for
  // people, sections is just a UI concept for now."): as a section of people. ⋮⋮ drags it among the sections, the name is a
  // field (empty deletes it, asked; its tasks go back), a task dropped on it goes last in it; the same headings are in the draft.
  const taskSectRow = (x) => {
    const g = grip(t('section.drag'));
    const n = tasksIn(x.id).length;
    const row = h('tr', { class: 'task-sect' }, h('th', { scope: 'row' }, h('div', { class: 'rl' }, g, foldBtn('ts' + x.id, x.name),
      nameField(x.name, t('section.name_aria', { name: x.name }), t('tsect.name_hint'),
        (v) => call(Setup.renameTaskSection, { org: r.org, id: x.id, name: v }, 'section'),
        async () => { await confirmed(t('section.delete'), t('tsect.delete_confirm', { name: x.name }), t('common.delete')); return call(Setup.deleteTaskSection, { org: r.org, id: x.id }); }))),
    h('td', { colspan: String(cols + 1) }, h('span', { class: 'small' }, n ? tn('tsect.tasks', n) : t('tsect.empty'))));
    dragRow(row, g, 'tsect', x, taskSections, (d, pos) => call(Setup.moveTaskSection, { org: r.org, id: d.id, position: pos }));
    const asSection = row._drop!;  // dragRow() has just set it
    row._drop = {
      accept: (k, d) => k === 'task' || asSection.accept(k, d),
      act: (d, after, kind) => (kind === 'task' ? moveTask(d, x.id, tasksIn(x.id).filter((o) => o.id !== d.id).length) : asSection.act(d, after)),
      get into() { return rowDrag && rowDrag.kind === 'task'; },
    };
    return row;
  };
  const movePerson = (d, section, position) => call(Setup.movePerson, { org: r.org, id: d.id, section, position });
  const moveTask = (d, section, position) => call(Setup.moveTask, { org: r.org, id: d.id, section, position });
  // ⚡ by a name: whether the planner may pick the person, or fills the task (one click; set by hand works either way).
  const zapBtn = (obj, key, title, aria, save: (on: boolean) => Promise<unknown>, after?) => {
    const b = h('button', { type: 'button', class: 'auto', 'aria-label': aria });
    const paint = () => { b.setAttribute('aria-pressed', String(!!obj[key])); b.title = title(!!obj[key]); };
    b.append(icon('auto'));
    let busy = false;
    b.addEventListener('click', async () => {
      if (busy) return;
      busy = true;
      obj[key] = !obj[key];
      paint();
      if (!(await write(() => save(obj[key])))) { obj[key] = !obj[key]; paint(); } else if (after) after();
      busy = false;
    });
    paint();
    return b;
  };
  const autoBtn = (p) => zapBtn(p, 'proposable', (on) => t(on ? 'person.auto_on' : 'person.auto_off', { name: p.name }),
    t('person.proposable_aria', { name: p.name }), (on) => call(Setup.updatePerson, { org: r.org, id: p.id, proposable: on }));
  // The tasks a person is willing to take, as tags under the name: green = good, yellow = if needed (2026-11-22, the user: "tags på
  // personer som viser gul/grønn tag for oppgaver de er villige til å ta? Kanskje gjøre det mulig å ekspande for å vise for alle?").
  // Always there, shown for everyone with «Show tasks» in the Availability heading (off by default, kept in this browser).
  const tagsOf = (p) => tasks.filter((tk) => tk.planned).map((tk) => [tk, ((chosen[p.id] || []).find((x) => x.taskId === tk.id) || {}).level])
    .filter(([, l]) => l === 'good' || l === 'poor')
    .map(([tk, l]) => h('span', { class: 'tag ' + l, title: tk.name + ': ' + t('pick.' + l) }, tk.name));
  const personRow = (p) => {
    const mine = (chosen[p.id] || []).filter((x) => x.level !== 'never' && tasks.some((tk) => tk.id === x.taskId && tk.planned)).map((x) => nameOf(tasks, x.taskId));
    const tags = tagsOf(p);
    const g = grip(t('person.drag'));
    handleFor(g, 'person', p);
    const row = h('tr', { class: 'person-row', 'data-name': p.name.toLowerCase(), 'data-person': String(p.id) },
      h('th', { scope: 'row' }, h('div', { class: 'rl' }, g, h('button', { type: 'button', class: 'cellbtn' + (mine.length ? '' : ' no-tasks'),
        title: mine.length ? mine.join(', ') : t('table.choose_tasks'), 'aria-label': t('table.person_aria', { name: p.name }), onclick: () => personDialog(p) },
        p.name), autoBtn(p)), tags.length ? h('div', { class: 'tags' }, tags) : ''),
      events.map((ev) => answerCell(p, ev)), rowPick('answers', p.id, p.name));
    return dropOn(row, (k, d) => k === 'person' && d.id !== p.id, (d, after) => {
      const others = inSection(p.section).filter((x) => x.id !== d.id);
      return movePerson(d, p.section, others.findIndex((x) => x.id === p.id) + (after ? 1 : 0));
    });
  };
  const sectionRow = (x, i) => {
    const g = grip(t('section.drag'));
    const n = inSection(x.id).length;
    const row = h('tr', { class: 'person-sect' }, h('th', { scope: 'row' }, h('div', { class: 'rl' }, g, foldBtn('s' + x.id, x.name),
      nameField(x.name, t('section.name_aria', { name: x.name }), t('section.name_hint'),
        (v) => call(Setup.renameSection, { org: r.org, id: x.id, name: v }, 'section'),
        async () => { await confirmed(t('section.delete'), t('section.delete_confirm', { name: x.name }), t('common.delete')); return call(Setup.deleteSection, { org: r.org, id: x.id }); }))),
    h('td', { colspan: String(cols + 1) }, h('span', { class: 'small' }, n ? tn('section.people', n) : t('section.empty'))));
    // a section is dragged among the sections (a line above or below); a person dropped on it goes last in it
    dragRow(row, g, 'section', x, sections, (d, pos) => call(Setup.moveSection, { org: r.org, id: d.id, position: pos }));
    const asSection = row._drop!;  // dragRow() has just set it
    row._drop = {
      accept: (k, d) => k === 'person' || asSection.accept(k, d),
      act: (d, after, kind) => (kind === 'person' ? movePerson(d, x.id, inSection(x.id).filter((o) => o.id !== d.id).length) : asSection.act(d, after)),
      get into() { return rowDrag && rowDrag.kind === 'person'; },
    };
    return row;
  };

  // The draft's extras, in its bar: the compromises as a chip; Adjust the planner (it was «What matters most», 2026-11-23, the user:
  // "a bit hard to understand. Maybe this should be a button with a logo and some better name?"), with a dot when it is not the
  // defaults; More ⋯ with the distribution, the earlier drafts and Remove unlocked.
  {
    const ext = bar.querySelector('.grp.extra')!;
    if (done && view.weak.length) ext.prepend(h('button', { type: 'button', class: 'chip warn-chip', onclick: () => listDialog(tn('draft.compromises', view.weak.length), view.weak) },
      tn('draft.compromises', view.weak.length)));
    if (org.level !== 'view') {
      const tuned = PRIORITIES.some((p) => priorityLevel(weights, p) !== 'normal');
      ext.append(h('button', { type: 'button', class: 'ghost sm prio-link' + (tuned ? ' tuned' : ''), title: prioritySummary(weights), onclick: prioritiesDialog },
        icon('sliders'), ' ' + t('weights.title')));
    }
    const items = [
      done && view.load.length ? h('button', { type: 'button', class: 'drop-item', onclick: () => listDialog(t('draft.distribution'), view.load.map((l) => t('run.load_line', { name: l.name, count: l.count, target: l.target.toLocaleString(locale()) }))) }, t('draft.distribution')) : '',
      done && runs.length > 1 ? h('button', { type: 'button', class: 'drop-item', onclick: settingsDialog }, t('draft.earlier')) : '',
      ...barMore].filter(Boolean) as HTMLElement[];
    items.forEach((b) => b.addEventListener('click', () => { const d = b.closest('details'); if (d) d.open = false; }));
    if (items.length) ext.append(dropdown('more', [icon('more'), h('span', { class: 'lbl' }, ' ' + t('draft.more'))], t('draft.more'), ...items));
  }

  // «Add a person» where the person lands: after those without a section, and at the end of each section
  const addPerson = (x?) => addRow(x ? t('table.new_person_in', { section: x.name }) : t('table.new_person'),
    (name) => call(Setup.createPerson, { org: r.org, ...(x ? { name, section: x.id } : { name }) }, 'person'));
  const addSection = () => addRow(t('table.new_section'), (name) => call(Setup.createSection, { org: r.org, name }, 'section'));
  const detailsHead = sect('details', t('table.details'), [t('table.details_hint')]);
  const tasksHead = dropOn(sect('tasks', t('table.tasks'), [t('table.tasks_hint')]), (k) => k === 'task',
    (d) => moveTask(d, 0, tasksIn(0).filter((o) => o.id !== d.id).length), true);
  const addTask = (x?) => addRow(x ? t('table.new_task_in', { section: x.name }) : t('table.new_task'),
    (name) => call(Setup.createTask, { org: r.org, ...(x ? { name, section: x.id } : { name }) }, 'task'));
  const addTaskSection = () => addRow(t('tsect.new'), (name) => call(Setup.createTaskSection, { org: r.org, name }, 'section'));
  let availHead;
  const table = h('div', { class: 'grid-scroll main' }, h('table', { class: 'dp-table' + (store.get('tags') === '1' ? ' show-tags' : '') },
    h('thead', {}, head),
    h('tbody', {},
      detailsHead,
      folds('details', adds('field', fields.fields.map(fieldRow), detailsHead, [addRow(t('table.new_field'), (name) => call(Plan.addField, { org: r.org, plan: r.plan, name }, 'field'))])),
      tasksHead,
      folds('tasks', [
        adds('task:0', tasksIn(0).map(taskRow), tasksHead, [addTask(), ...(taskSections.length ? [] : [addTaskSection()])]),
        taskSections.flatMap((x, i) => { const head = taskSectRow(x);
          return [head, ...folds('ts' + x.id, adds('task:' + x.id, tasksIn(x.id).map(taskRow), head, [addTask(x), ...(i === taskSections.length - 1 ? [addTaskSection()] : [])]))]; })]),
      h('tr', { class: 'sect draft' }, h('td', { colspan: String(cols + 2) }, bar)),
      folds('draft', [tasks.map((tk, i) => [
        // the task sections' headings in the draft too, so it reads as the tasks above
        tk.section && (i === 0 || tasks[i - 1].section !== tk.section)
          ? h('tr', { class: 'draft-sect' }, h('th', { scope: 'row' }, (taskSections.find((x) => x.id === tk.section) || {}).name || ''), h('td', { colspan: String(cols + 1) })) : '',
        h('tr', { class: 'draft-row' }, h('th', { scope: 'row' }, org.level !== 'view' ? h('div', { class: 'rl' }, h('span', { class: 'nm' }, tk.name),
          groupLock(events.map((ev) => [ev, tk]), t('lock.row', { name: tk.name }), t('unlock.row', { name: tk.name }))) : tk.name),
        events.map((ev) => putDraftCell(ev, tk)), rowPick('draft', tk.id, tk.name))]),
        // a lock per date, under the draft
        org.level !== 'view' && done ? h('tr', { class: 'draft-locks' }, h('th', { scope: 'row' }, h('span', { class: 'small muted' }, t('lock.col_head'))),
          events.map((ev) => h('td', {}, groupLock(tasks.map((tk) => [ev, tk]), t('lock.col', { date: label(ev) }), t('unlock.col', { date: label(ev) })))), h('td', {})) : '']),
      [
        availHead = dropOn(h('tr', { class: 'sect' }, h('th', { scope: 'row' }, h('div', { class: 'rl' }, foldBtn('avail', t('table.availability')), t('table.availability'), infoBtn(t('table.availability'), [t('table.availability_info'), t('sel.info')])),
          people.length > 1 ? whoBtn : '', people.length ? tagsBtn : '', people.length > 8 ? filterBox() : ''),
          events.map(countCell), filler()),
          (k) => k === 'person', (d) => movePerson(d, 0, inSection(0).filter((o) => o.id !== d.id).length), true),
        folds('avail', [
          adds('person:0', inSection(0).map(personRow), availHead, [addPerson(), ...(sections.length ? [] : [addSection()])]),
          sections.flatMap((x, i) => { const head = sectionRow(x, i);
            return [head, ...folds('s' + x.id, adds('person:' + x.id, inSection(x.id).map(personRow), head, [addPerson(x), ...(i === sections.length - 1 ? [addSection()] : [])]))]; })])])));
  applyFolds(table);
  table.addEventListener('pointerdown', onCellDown);
  // Frozen details rows stay in sight: each sticks under the dates and the frozen rows above it (measured again when the table
  // changes size, as when a section folds or the dates wrap).
  const stick = () => {
    const tb = table.querySelector('table')!;
    let top = tb.tHead!.offsetHeight;
    for (const tr of tb.querySelectorAll<HTMLElement>('tr.field-row.frozen')) { tr.style.setProperty('--stick', top + 'px'); top += tr.offsetHeight; }
  };
  new ResizeObserver(stick).observe(table.querySelector('table')!);
  if (planState.sel) {
    const keys = new Set(planState.sel.keys.filter((k) => grids.draft.cells.has(k)));
    if (keys.size) { sel = { grid: 'draft', keys, anchor: planState.sel.anchor, base: new Set(keys) }; paintSel(); }
    planState.sel = null;
  }

  events.forEach((ev) => recount(ev.id));

  const root: Page = h('div', { class: 'plan-page' },
    h('div', { class: 'title-bar' },
      back(),
      h('h1', {}, plan.name),
      infoBtn(plan.name, [t('table.hint')]),
      modeSwitch,
      events.length ? sideways(table, events) : '',
      shareBtn(),
      h('button', { type: 'button', class: 'ghost settings', 'aria-label': t('settings.open'), onclick: settingsDialog }, '⚙', h('span', { class: 'lbl' }, ' ' + t('settings.open')))),
    viewOnly(org),
    events.length ? '' : h('p', { class: 'muted small' }, t('table.no_events')),
    table,
    selbar,
    notePop, tallyTip);
  dragScroll(table);
  table.addEventListener('pointerdown', onRowDown);
  // the field in a cell opens again after a change (editCell), for the next name
  const editing: [any, any] | undefined = planState.edit ? events.flatMap((ev) => tasks.map((tk) => [ev, tk] as [any, any])).find(([ev, tk]) => at(ev, tk) === planState.edit) : undefined;
  planState.edit = null;
  root.shown = () => {
    if (editing && draftTd[at(...editing)]) editCell(...editing, draftTd[at(...editing)], planState.editInput ? planState.editInput.value : '');
    return reopenedPage();
  };
  const reopenedPage = () => reopened({
    task: (id) => { const x = tasks.find((tk) => tk.id === id); if (x) taskDialog(x); },
    person: (id) => { const x = people.find((p) => p.id === id); if (x) personDialog(x); },
    place: (k) => { const [e, tk] = k.split('/').map(Number); const ev = events.find((x) => x.id === e), task = tasks.find((x) => x.id === tk); if (ev && task) placeDialog(ev, task); },
    settings: () => settingsDialog(),
    priorities: () => prioritiesDialog(),
  });
  return root;
}
