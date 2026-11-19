'use strict';
// Dutyplan: a small frontend without a framework. The organisation is in the address (/o/<org>/...), so switching
// organisation is just going to another address. There is no login. All texts come from i18n.js (t, tn); the language is
// a per-browser choice.

const store = {
  get: (k) => { try { return localStorage.getItem('tp.' + k) || ''; } catch (e) { return ''; } },
  set: (k, v) => { try { localStorage.setItem('tp.' + k, v); } catch (e) { /* private mode */ } },
};

function h(tag, props, ...kids) {
  const el = document.createElement(tag);
  for (const [k, v] of Object.entries(props || {})) {
    if (v === false || v == null) continue;
    if (k === 'class') el.className = v;
    else if (k === 'checked' || k === 'value' || k === 'disabled') el[k] = v;
    else if (k.startsWith('on')) el.addEventListener(k.slice(2), v);
    else el.setAttribute(k, v === true ? '' : v);
  }
  for (const kid of kids.flat(2)) el.append(kid instanceof Node ? kid : document.createTextNode(String(kid ?? '')));
  return el;
}

function say(text, kind) {
  const m = document.querySelector('dialog[open] [data-msg]') || document.getElementById('msg');
  m.textContent = text || '';
  m.className = text ? kind || 'ok' : '';
}

const cap = (s) => s.charAt(0).toUpperCase() + s.slice(1);
const locale = () => (currentLang === 'nb' ? 'nb-NO' : 'en-GB');

async function api(method, path, body) {
  // The server writes explanations and warnings in the language of the page.
  if (method === 'GET' && /\/runs/.test(path)) path += (path.includes('?') ? '&' : '?') + 'lang=' + currentLang;
  const headers = {};
  if (body !== undefined) headers['Content-Type'] = 'application/json';
  const res = await fetch('/api' + path, { method, headers, body: body === undefined ? undefined : JSON.stringify(body) });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw Object.assign(new Error(data.message || res.statusText), { kind: data.error, apiCode: data.code, params: data.params, status: res.status });
  return data;
}

// The text for a refused request, in the page's language: from the error code and its parameters when the server sent
// them, else from the kind of error, else the server's own (English) text.
function errorText(e) {
  if (e.apiCode && hasMsg('err.' + e.apiCode)) {
    const p = { ...e.params };
    if (typeof p.kind === 'string') p.kind = cap(hasMsg('kind.' + p.kind.replace(' ', '_')) ? t('kind.' + p.kind.replace(' ', '_')) : p.kind);
    if (typeof p.what === 'string' && hasMsg('what.' + p.what)) p.what = t('what.' + p.what);
    if (typeof p.key === 'string' && hasMsg('weight.' + p.key + '.label')) p.key = t('weight.' + p.key + '.label');
    return t('err.' + e.apiCode, p);
  }
  if (e.kind && hasMsg('err.kind.' + e.kind)) return t('err.kind.' + e.kind);
  return e.message || t('err.generic');
}

const cancelled = () => Object.assign(new Error('cancelled'), { cancelled: true });
// Ask before something destructive; inside a write() action, a «no» ends the action quietly.
function confirmed(text) { if (!confirm(text)) throw cancelled(); }

// A write: shows the error instead of throwing, and says whether it went through.
async function write(action) {
  try {
    await action();
    say('');
    return true;
  } catch (e) {
    if (e.cancelled) return false;
    say(errorText(e), 'err');
    return false;
  }
}

const enc = encodeURIComponent;
const fmtTime = (iso) => new Date(iso).toLocaleString(locale(), { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' });
const fmtDate = (iso) => new Date(iso + 'T00:00:00Z').toLocaleDateString(locale(), { weekday: 'short', day: 'numeric', month: 'short', year: 'numeric', timeZone: 'UTC' });
const fmtShortDate = (iso) => new Date(iso + 'T00:00:00Z').toLocaleDateString(locale(), { weekday: 'short', day: 'numeric', month: 'short', timeZone: 'UTC' });
const addDays = (iso, n) => { const d = new Date(iso + 'T00:00:00Z'); d.setUTCDate(d.getUTCDate() + n); return d.toISOString().slice(0, 10); };
// The dates «every week until»: from each start date, every 7 days up to and including until, without repeats.
function weeklyDates(dates, until) {
  const out = new Set();
  for (const d of dates.filter(Boolean)) {
    if (!until) { out.add(d); continue; }
    for (let x = d; x <= until && out.size <= 400; x = addDays(x, 7)) out.add(x);
  }
  return [...out];
}

// The time of day of an event: «11:00» or «11:00–12:30», or '' when it has none. A 24-hour clock in both languages.
const clock = (ev) => (ev.start_time ? (ev.end_time ? `${ev.start_time}–${ev.end_time}` : ev.start_time) : '');

function go(path) {
  history.pushState({}, '', path);
  render();
}
function link(path, text, props) {
  return h('a', { href: path, ...props, onclick: (e) => { if (e.metaKey || e.ctrlKey || e.shiftKey) return; e.preventDefault(); go(path); } }, text);
}
const linkBtn = (text, onclick, aria) => h('button', { type: 'button', class: 'link-button', 'aria-label': aria, onclick }, text);

function route() {
  const parts = location.pathname.split('/').filter(Boolean);
  if (parts[0] === 'o' && parts[1]) {
    if (parts[2] === 'p' && parts[3] && parts[4] === 'me' && parts[5]) return { page: 'me', org: parts[1], plan: parts[3], person: parts[5] };
    if (parts[2] === 'p' && parts[3]) return { page: 'plan', org: parts[1], plan: parts[3] };
    return { page: 'org', org: parts[1] };
  }
  return { page: 'home' };
}

async function renderHeader(r) {
  const orgs = await api('GET', '/orgs');
  const select = h('select', { 'aria-label': t('header.org'), onchange: (e) => go(e.target.value === '' ? '/?switch' : '/o/' + enc(e.target.value)) },
    h('option', { value: '' }, r.org ? t('header.switch_org') : t('header.choose_org')),
    orgs.map((o) => h('option', { value: o.slug, selected: o.slug === r.org }, o.name)));
  const lang = h('select', { id: 'lang', 'aria-label': t('lang.label'), onchange: (e) => { setLang(e.target.value); render(); } },
    LANGS.map((l) => h('option', { value: l, selected: l === currentLang }, t('lang.' + l))));
  document.getElementById('top').replaceChildren(
    link('/?switch', t('brand'), { class: 'brand' }),
    h('label', {}, t('header.org'), select),
    h('span', { class: 'spacer' }),
    h('label', { class: 'lang' }, h('span', { class: 'sr-only' }, t('lang.label')), lang));
}

// ---- building blocks -------------------------------------------------------------------------------------------------

// After a change in a dialog the page is drawn again with the new data, and the same dialog opens again on top (the page's
// shown() does it once the new page is in place).
let reopen = null;
const again = (kind, id) => { reopen = { kind, id }; };
function reopened(handlers) {
  const ro = reopen;
  reopen = null;
  if (ro && handlers[ro.kind]) handlers[ro.kind](ro.id);
}

// A dialog over the page; messages (say) go into it while it is open, and it is removed when closed.
function modal(title, ...kids) {
  const dlg = h('dialog', { class: 'modal', 'aria-label': title },
    h('div', { class: 'modal-head' }, h('h2', {}, title), h('button', { type: 'button', class: 'ghost', 'aria-label': t('common.close'), onclick: () => dlg.close() }, '×')),
    h('p', { 'data-msg': '', role: 'status' }), ...kids);
  dlg.addEventListener('close', () => dlg.remove());
  document.body.append(dlg);
  dlg.showModal();
  return dlg;
}
// The bottom line of a dialog: something destructive on the left, the buttons on the right.
const foot = (left, ...right) => h('div', { class: 'modal-foot' }, h('div', {}, left || ''), h('div', { class: 'row-inline' }, ...right));
const closeBtn = () => h('button', { type: 'button', class: 'ghost', onclick: (e) => e.currentTarget.closest('dialog').close() }, t('common.close'));
const field = (text, input) => h('label', { class: 'field' }, h('span', {}, text), input);

// A small form: a name and «+».
function addRow(placeholder, onsubmit) {
  const input = h('input', { type: 'text', placeholder, maxlength: '80', required: true, 'aria-label': placeholder });
  return h('form', { class: 'addrow', onsubmit: async (e) => {
    e.preventDefault();
    if (await write(() => onsubmit(input.value))) render();
  } }, h('span', { class: 'plus', 'aria-hidden': 'true' }, '+'), input, h('button', { type: 'submit', 'aria-label': placeholder }, t('common.add')));
}

// Enter in a field of the table moves on to the same field of the next event (the change is saved when the field is left).
function enterMovesOn(input) {
  input.addEventListener('keydown', (e) => {
    if (e.key !== 'Enter') return;
    e.preventDefault();
    const td = input.closest('td');
    const next = td && td.nextElementSibling && td.nextElementSibling.querySelector('input');
    input.blur();
    if (next) next.focus();
  });
  return input;
}

// ---- the start page and the organisation ----------------------------------------------------------------------------

async function pageHome() {
  const orgs = await api('GET', '/orgs');
  const last = store.get('org');
  if (last && !location.search.includes('switch') && orgs.some((o) => o.slug === last)) return go('/o/' + enc(last));
  const name = h('input', { type: 'text', placeholder: t('home.name_placeholder'), maxlength: '80', required: true, 'aria-label': t('home.name_placeholder') });
  return h('div', {},
    h('h1', {}, t('home.title')),
    orgs.length ? h('ul', { class: 'plain' }, orgs.map((o) => h('li', {}, link('/o/' + enc(o.slug), o.name)))) : h('p', { class: 'muted' }, t('home.none')),
    h('h2', {}, t('home.new')),
    h('form', { class: 'row', onsubmit: async (e) => {
      e.preventDefault();
      let o;
      if (await write(async () => { o = await api('POST', '/orgs', { name: name.value }); })) { store.set('org', o.slug); go('/o/' + enc(o.slug)); }
    } }, name, h('button', { type: 'submit' }, t('common.create'))),
    h('p', { class: 'muted small' }, t('home.warning')));
}

const GROUP_LEVELS = ['poor', 'neutral', 'good'];

// The organisation: its plans and its groups, and rename or delete. Tasks and people are added from a plan's table; they
// belong to the organisation, so every plan has them.
async function pageOrg(r, org, people) {
  const base = `/orgs/${enc(r.org)}`;
  const [plans, groups, tasks] = await Promise.all([api('GET', base + '/plans'), api('GET', base + '/groups'), api('GET', base + '/tasks')]);
  const nameOf = (list, id) => (list.find((x) => x.id === id) || { name: '?' }).name;

  function groupDialog(g) {
    let dlg;
    const gpath = `${base}/groups/${g.id}`;
    const changed = () => { again('group', g.id); dlg.close(); render(); };
    const name = h('input', { type: 'text', value: g.name, maxlength: '80', 'aria-label': t('common.name') });
    const boxes = people.filter((p) => p.active || g.members.includes(p.id)).map((p) => ({ p,
      box: h('input', { type: 'checkbox', checked: g.members.includes(p.id), 'aria-label': t('group.member_aria', { person: p.name, group: g.name }) }) }));
    const proposable = h('input', { type: 'checkbox', checked: g.proposable, 'aria-label': t('group.proposable') });
    proposable.addEventListener('change', async () => { if (await write(() => api('PATCH', gpath, { proposable: proposable.checked }))) changed(); else proposable.checked = g.proposable; });
    const rows = tasks.filter((tk) => tk.planned).map((tk) => {
      const cur = g.tasks.find((x) => x.task_id === tk.id);
      return { tk,
        fills: h('input', { type: 'number', min: '0', max: '50', value: cur ? String(cur.fills) : '0', 'aria-label': t('group.fills_aria', { group: g.name, task: tk.name }) }),
        level: h('select', { 'aria-label': t('group.level_aria', { group: g.name, task: tk.name }) }, GROUP_LEVELS.map((v) => h('option', { value: v, selected: v === (cur ? cur.level : 'neutral') }, t('level.' + v)))) };
    });
    dlg = modal(g.name,
      h('form', { class: 'row', onsubmit: async (e) => {
        e.preventDefault();
        if (name.value.trim() !== g.name && await write(() => api('PATCH', gpath, { name: name.value }))) changed();
      } }, name, h('button', { type: 'submit' }, t('common.rename'))),
      h('h3', {}, t('group.members')),
      boxes.length ? [h('div', { class: 'chips' }, boxes.map(({ p, box }) => h('label', { class: 'chip' }, box, ' ' + p.name + (p.active ? '' : ' ' + t('person.inactive_short'))))),
        h('div', { class: 'row-inline' }, h('button', { type: 'button', onclick: async () => {
          if (await write(() => api('PUT', gpath + '/members', { people: boxes.filter((x) => x.box.checked).map((x) => x.p.id) }))) changed();
        } }, t('group.save_members')))] : h('p', { class: 'muted' }, t('group.no_people')),
      h('h3', {}, t('group.proposed_for')),
      h('label', { class: 'check' }, proposable, ' ' + t('group.proposable')),
      h('p', { class: 'muted small' }, t('group.proposed_hint')),
      rows.length ? [h('table', { class: 'mini' }, h('thead', {}, h('tr', {}, h('th', {}, t('group.col_task')), h('th', {}, t('group.col_fills')), h('th', {}, t('group.col_how')))),
        h('tbody', {}, rows.map(({ tk, fills, level }) => h('tr', {}, h('td', {}, tk.name), h('td', {}, fills), h('td', {}, level))))),
        h('div', { class: 'row-inline' }, h('button', { type: 'button', onclick: async () => {
          const list = rows.map(({ tk, fills, level }) => ({ task_id: tk.id, fills: parseInt(fills.value, 10) || 0, level: level.value })).filter((x) => x.fills > 0);
          if (await write(() => api('PUT', gpath + '/tasks', { tasks: list }))) changed();
        } }, t('group.save_tasks')))] : h('p', { class: 'muted' }, t('group.no_planned_tasks')),
      foot(h('button', { type: 'button', class: 'danger', onclick: async () => {
        if (await write(async () => { confirmed(t('group.delete_confirm', { name: g.name })); await api('DELETE', gpath); })) { dlg.close(); render(); }
      } }, t('group.delete')), closeBtn()));
  }

  // Rename (the address stays) or delete. Deleting needs the organisation to have no plans, and its name typed in.
  function orgDialog() {
    let dlg;
    const name = h('input', { type: 'text', value: org.name, maxlength: '80', 'aria-label': t('org.rename_label') });
    let del;
    if (plans.length) del = h('p', { class: 'muted small' }, t('org.delete_has_plans'));
    else {
      const typed = h('input', { type: 'text', placeholder: t('org.delete_type'), 'aria-label': t('org.delete_type'), autocomplete: 'off', maxlength: '80' });
      const button = h('button', { type: 'submit', class: 'danger', disabled: true }, t('org.delete_button'));
      typed.addEventListener('input', () => { button.disabled = typed.value.trim() !== org.name; });
      del = h('form', { class: 'stack', onsubmit: async (e) => {
        e.preventDefault();
        if (typed.value.trim() !== org.name) return;
        if (await write(() => api('DELETE', `${base}?confirm=${enc(r.org)}`))) { dlg.close(); store.set('org', ''); go('/?switch'); say(t('org.deleted', { name: org.name })); }
      } }, h('p', { class: 'muted small' }, t('org.delete_hint')), h('div', { class: 'row-inline' }, typed, button));
    }
    dlg = modal(org.name,
      h('form', { class: 'row', onsubmit: async (e) => {
        e.preventDefault();
        if (await write(() => api('PATCH', base, { name: name.value }))) { dlg.close(); await render(); say(t('org.renamed')); }
      } }, name, h('button', { type: 'submit' }, t('common.rename'))),
      h('h3', {}, t('org.delete_title')), del,
      foot('', closeBtn()));
  }

  const groupLine = (g) => {
    const members = g.members.map((id) => nameOf(people, id));
    const proposed = g.proposable ? g.tasks.map((x) => tn('org.group_task', x.fills, { task: nameOf(tasks, x.task_id) })) : [];
    return h('div', { class: 'grp' },
      h('button', { type: 'button', class: 'cellbtn inline', 'aria-label': t('org.group_aria', { name: g.name }), onclick: () => groupDialog(g) }, g.name),
      h('span', { class: 'muted' }, ' · ' + (members.length ? members.join(', ') : t('org.group_empty'))),
      h('div', { class: 'small muted' }, proposed.length ? t('org.group_proposed', { tasks: proposed.join(', ') }) : t('org.group_by_hand')));
  };
  const root = h('div', { class: 'org-page' },
    h('h1', {}, org.name),
    h('h2', {}, t('org.plans')),
    h('div', { class: 'plans' },
      plans.length ? plans.map((p) => h('div', { class: 'plan-item' }, h('span', { class: 'nm' }, p.name), link(`/o/${enc(r.org)}/p/${enc(p.slug)}`, t('org.open'), { class: 'button' })))
        : h('p', { class: 'muted' }, t('org.no_plans')),
      addRow(t('org.new_plan_placeholder'), async (name) => { const p = await api('POST', base + '/plans', { name }); history.pushState({}, '', `/o/${enc(r.org)}/p/${enc(p.slug)}`); })),
    h('h2', {}, t('org.groups')),
    h('p', { class: 'muted small' }, t('org.groups_hint')),
    h('div', { class: 'box' }, groups.length ? groups.map(groupLine) : h('p', { class: 'muted' }, t('org.no_groups')), addRow(t('org.new_group'), (name) => api('POST', base + '/groups', { name }))),
    h('p', { class: 'small muted org-foot' }, h('button', { type: 'button', class: 'link-button', onclick: orgDialog }, t('org.manage'))));
  root.shown = () => reopened({ group: (id) => { const g = groups.find((x) => x.id === id); if (g) groupDialog(g); } });
  return root;
}

// ---- the plan page: one table ----------------------------------------------------------------------------------------
// Events across. From the top: the event details (rows the plan defines), the tasks with the places each event needs, the
// draft allocation (who got what, and what is set by hand), and availability (everyone's answer per event, set with a click:
// blank → yes → maybe → no → blank; blank = not answered, which the planner counts as neutral). The names of events, rows,
// tasks and people open a dialog with the rest. Tasks, combinations and people belong to the organisation, not the plan.

const ANSWER_NEXT = { '': 'good', good: 'poor', poor: 'never', never: '' };
const ANSWER_MARK = { '': '', good: '✓', poor: '?', never: '✗' };
const PERSON_TASK_LEVELS = [['', 'person.not_taking_part'], ['never', 'level.task.never'], ['poor', 'level.poor'], ['neutral', 'level.neutral'], ['good', 'level.good']];
const SELECT_LEVELS = ['standard', 'cannot', 'impractical', 'practical', 'preferred'];

// Weighting: what the planner puts weight on, for this plan. The numbers are points in the planner's sum; the defaults are a good
// starting point. Only what differs from the default is stored, so a changed default reaches plans that never chose anything.
const WEIGHT_GROUP = {
  w_fair: 'fair', w_fair_range: 'fair', w_task_fair: 'fair', w_task_range: 'fair',
  'w_task.good': 'wishes', 'w_task.poor': 'wishes', 'w_date.good': 'wishes', 'w_date.poor': 'wishes',
  'w_combo.preferred': 'combos', 'w_combo.practical': 'combos', 'w_combo.impractical': 'combos', default_combination: 'combos',
};
const weightLabel = (key) => t('weight.' + key + '.label');
const weightGroup = (key) => t('weight.group.' + (WEIGHT_GROUP[key] || 'fair'));

function weightsForm(path, wr, saved) {
  const inputs = wr.specs.map((spec) => {
    const cur = spec.key in wr.values ? wr.values[spec.key] : spec.default;
    const input = spec.kind === 'choice'
      ? h('select', { 'aria-label': weightLabel(spec.key) }, spec.options.map((o) => h('option', { value: o, selected: o === cur }, hasMsg('combo.' + o) ? t('combo.' + o) : o)))
      : h('input', { type: 'number', min: String(spec.min), max: String(spec.max), step: 'any', value: String(cur), 'aria-label': weightLabel(spec.key) });
    return { spec, input, changed: spec.key in wr.values };
  });
  const groups = [...new Set(wr.specs.map((x) => weightGroup(x.key)))];
  const blocks = groups.map((g) => h('div', { class: 'weight-group' }, h('h4', {}, g),
    inputs.filter(({ spec }) => weightGroup(spec.key) === g).map(({ spec, input, changed }) => {
      const def = spec.kind === 'choice' ? t('combo.' + spec.default) : spec.default;
      return h('div', { class: 'weight-row' + (changed ? ' changed' : '') },
        h('label', {}, h('span', { class: 'wlabel' }, weightLabel(spec.key), changed ? h('span', { class: 'tag' }, t('weights.changed')) : ''), input),
        h('div', { class: 'small muted' }, `${t('weight.' + spec.key + '.hint')} ${t('weights.default', { value: def })}`));
    })));
  const save = (values, text) => async () => { if (await write(() => api('PUT', path, { values }))) saved(text); };
  const nChanged = Object.keys(wr.values || {}).length;
  return h('details', {}, h('summary', {}, nChanged ? t('weights.show_changed', { n: nChanged }) : t('weights.show')),
    h('p', { class: 'muted small' }, t('weights.hint')),
    h('div', { class: 'stack' }, blocks,
      h('div', { class: 'row-inline' },
        h('button', { type: 'button', onclick: () => {
          const values = {};
          for (const { spec, input } of inputs) {
            if (spec.kind === 'choice') values[spec.key] = input.value;
            else if (input.value.trim() !== '') values[spec.key] = Number(input.value);
          }
          return save(values, t('weights.saved'))();
        } }, t('weights.save')),
        h('button', { type: 'button', class: 'ghost', onclick: save({}, t('weights.reset_done')) }, t('weights.reset')))));
}

// What the page keeps between redraws of the same plan: the draft picked under the earlier drafts.
const planState = { key: '', runId: null };

async function pagePlan(r, org, people) {
  const base = `/orgs/${enc(r.org)}`;
  const pbase = `${base}/plans/${enc(r.plan)}`;
  const epath = pbase + '/events';
  const key = r.org + '/' + r.plan;
  if (planState.key !== key) Object.assign(planState, { key, runId: null });
  const [plan, events, tasks, combos, groups, fields, answers, places, cells, pools, weights, health, runs] = await Promise.all([
    api('GET', pbase), api('GET', epath), api('GET', base + '/tasks'), api('GET', base + '/combinations'), api('GET', base + '/groups'),
    api('GET', pbase + '/fields'), api('GET', pbase + '/answers'), api('GET', pbase + '/places'), api('GET', pbase + '/cells'),
    api('GET', pbase + '/pools'), api('GET', pbase + '/weights'), api('GET', '/health'), api('GET', pbase + '/runs')]);
  // A personal link (…/me/<id>) opens the table with that person's row marked; an unknown id is a page that does not exist.
  const highlight = r.person ? people.find((p) => String(p.id) === r.person) : null;
  if (r.person && !highlight) throw Object.assign(new Error(t('page.not_found')), { status: 404, keepOrg: true });

  // The draft on show: the one picked under the earlier drafts, else the current plan (pinned), else the newest.
  const pinned = runs.find((x) => x.pinned);
  const shownId = runs.some((x) => x.id === planState.runId) ? planState.runId : pinned ? pinned.id : runs.length ? runs[0].id : null;
  const view = shownId ? await api('GET', `${pbase}/runs/${shownId}`) : null;
  const done = !!view && view.run.status === 'done';

  const label = (ev) => fmtShortDate(ev.date) + (ev.start_time ? ' ' + ev.start_time : '');
  const nameOf = (list, id) => (list.find((x) => x.id === id) || { name: '?' }).name;
  const at = (ev, tk) => ev.id + '/' + tk.id;
  const ans = {};  // person -> event -> never | poor | good ('' or missing = not answered)
  answers.events.forEach((a) => { (ans[a.person_id] ||= {})[a.event_id] = a.level; });
  const chosen = {};  // person -> the tasks they take part in, with how willingly
  answers.tasks.forEach((a) => { (chosen[a.person_id] ||= []).push(a); });
  const placesAt = {};  // 'event/task' -> places (missing = not needed)
  places.forEach((p) => { placesAt[p.event_id + '/' + p.task_id] = p.slots; });
  const fieldValue = {};  // 'event/field' -> text
  fields.values.forEach((v) => { fieldValue[v.event_id + '/' + v.field_id] = v.value; });
  const cellsAt = {};  // 'event/task' -> the cells set by hand and the locked names, as they are now
  cells.forEach((c) => { (cellsAt[c.event_id + '/' + c.task_id] ||= []).push(c); });
  const poolAt = {};  // 'event/task' -> whom the planner may pick
  pools.forEach((p) => { poolAt[p.event_id + '/' + p.task_id] = p; });
  const planCell = {};  // 'event/task' -> the cell of the draft on show
  if (done) for (const ev of view.events) for (const c of ev.cells) planCell[ev.id + '/' + c.task_id] = c;

  // What a place in the draft holds now: the cells set by hand and the locked names (always as they are now), then the
  // planner's names in the draft on show that are not among them, then what is still open. A name that was set by hand when
  // the draft was made but has been removed since is not shown.
  const slotsAt = (ev, tk) => {
    const k = at(ev, tk);
    const n = placesAt[k] || 0;
    const items = (cellsAt[k] || []).map((c) => ({ cell: c, kind: c.person_id ? 'person' : c.group_id ? 'group' : 'text', id: c.person_id || c.group_id || 0,
      name: c.person_id ? nameOf(people, c.person_id) : c.group_id ? nameOf(groups, c.group_id) : c.label, fills: c.fills || 1, hand: !c.locked, locked: c.locked }));
    const rc = planCell[k];
    if (rc) {
      const has = (kind, id) => items.some((x) => x.kind === kind && x.id === id);
      rc.people.filter((p) => !p.locked && !has('person', p.id)).forEach((p) => items.push({ kind: 'person', id: p.id, name: p.name, fills: 1, weak: p.weak }));
      rc.groups.filter((g) => !g.locked && !has('group', g.id)).forEach((g) => items.push({ kind: 'group', id: g.id, name: g.name, fills: g.fills, weak: g.weak }));
    }
    const filled = items.reduce((s, x) => s + x.fills, 0);
    return { n, items, open: Math.max(0, n - filled), why: rc && rc.missing > 0 ? rc.explanation || '' : '' };
  };
  const itemText = (x) => x.name + (x.fills > 1 ? ' ×' + x.fills : '');
  const slot = (x) => h('span', { class: 'slot' + (x.hand ? ' hand' : '') }, itemText(x),
    x.locked ? h('span', { class: 'chip' }, t('draft.locked')) : '', x.weak ? h('span', { class: 'chip poor' }, t('draft.maybe')) : '');

  // ---- the draft: make one or try again, and what the draft on show is
  const runsPath = pbase + '/runs';
  const newest = runs[0];
  const running = !!newest && newest.status === 'running';
  const pin = async (id, on) => {
    if (!(await write(() => api(on ? 'PUT' : 'DELETE', `${runsPath}/${id}/pin`)))) return;
    planState.runId = id;
    await render();
    say(on ? t('run.pin_done', { id }) : t('run.unpin_done'));
  };
  const bar = h('div', { class: 'draft-bar' });
  const poll = async () => {
    if (!bar.isConnected) return;  // the page has been replaced
    try {
      const latest = await api('GET', runsPath + '/latest');
      if (latest.run.status === 'running') setTimeout(poll, 1000);
      else { planState.runId = latest.run.id; render(); }
    } catch (e) { say(errorText(e), 'err'); }
  };
  {
    const limits = [[10, '10 s'], [20, '20 s'], [60, '60 s']];
    const limitSel = h('select', { 'aria-label': t('run.limit_aria') }, limits.map(([v, text]) => h('option', { value: String(v), selected: String(v) === (store.get('limit') || '20') }, text)));
    limitSel.addEventListener('change', () => store.set('limit', limitSel.value));
    const start = async () => {
      let id;
      if (await write(async () => { id = (await api('POST', runsPath, { time_limit: parseInt(limitSel.value, 10) })).id; })) { planState.runId = id; render(); }
    };
    const line = [h('button', { type: 'button', disabled: !health.planner || running, onclick: start }, running ? t('run.planning') : view ? t('run.retry') : t('run.plan')), limitSel];
    const notes = [];
    if (!health.planner) line.push(h('span', { class: 'small' }, t('run.no_planner')));
    if (running) { line.push(h('span', { role: 'status' }, t('run.running', { n: newest.time_limit }))); setTimeout(poll, 1000); }
    if (!view) { if (!running) line.push(h('span', { class: 'small' }, t('run.none_yet'))); }
    else if (view.run.status === 'failed') notes.push(h('p', { class: 'err', role: 'alert' }, t('run.failed', { error: view.run.error || t('run.unknown_error') })));
    else if (done) {
      const run = view.run;
      line.push(h('span', { class: 'small' }, t('run.label', { id: run.id, time: fmtTime(run.finished_at || run.started_at) }) + ' · ' + (run.solver_status === 'OPTIMAL' ? t('run.optimal') : t('run.feasible'))));
      line.push(run.pinned ? [h('span', { class: 'pinned' }, '✓ ' + t('run.current')), linkBtn(t('run.unpin'), () => pin(run.id, false))]
        : h('button', { type: 'button', class: 'ghost sm', onclick: () => pin(run.id, true) }, t('run.pin')));
      if (run.id !== newest.id) line.push(linkBtn(t('run.show_newest'), () => { planState.runId = newest.id; render(); }));
      if (run.stale) notes.push(h('p', { class: 'warn small', role: 'status' }, t('run.stale')));
      if (run.checked && run.violations.length) notes.push(h('div', { class: 'err', role: 'alert' }, h('p', {}, t('run.violations')), h('ul', {}, run.violations.map((v) => h('li', {}, v)))));
      if (view.warnings.length) notes.push(h('ul', { class: 'warn small' }, view.warnings.map((w) => h('li', {}, w))));
    }
    const legend = h('span', { class: 'legend small' }, h('span', { class: 'slot hand' }, t('draft.key_hand')),
      h('span', {}, h('span', { class: 'chip' }, t('draft.locked')), ' ' + t('draft.key_locked')));
    bar.replaceChildren(h('div', { class: 'row-inline' }, line, legend), ...notes);
  }

  // ---- dialogs
  function eventDialog(ev) {
    let dlg;
    const start = h('input', { type: 'time', value: ev.start_time, 'aria-label': t('event.starts') });
    const end = h('input', { type: 'time', value: ev.end_time, 'aria-label': t('event.ends') });
    const desc = h('textarea', { rows: '3', maxlength: '2000', placeholder: t('event.description_placeholder'), 'aria-label': t('event.description') }, ev.description);
    const values = fields.fields.map((f) => ({ f, input: h('input', { type: 'text', maxlength: '200', value: fieldValue[ev.id + '/' + f.id] || '', 'aria-label': f.name }) }));
    dlg = modal(fmtDate(ev.date),
      h('form', { class: 'stack', onsubmit: async (e) => {
        e.preventDefault();
        const body = {};
        const endWanted = start.value ? end.value : '';  // an end time needs a start time
        if (start.value !== ev.start_time) body.start_time = start.value;
        if (endWanted !== ev.end_time) body.end_time = endWanted;
        if (desc.value.trim() !== ev.description) body.description = desc.value;
        const todo = values.filter(({ f, input }) => input.value.trim() !== (fieldValue[ev.id + '/' + f.id] || ''));
        if (!Object.keys(body).length && !todo.length) { dlg.close(); return; }
        if (await write(async () => {
          if (Object.keys(body).length) await api('PATCH', `${epath}/${ev.id}`, body);
          for (const { f, input } of todo) await api('PUT', `${epath}/${ev.id}/fields/${f.id}`, { value: input.value });
        })) { dlg.close(); render(); }
      } },
        h('div', { class: 'row-inline' }, field(t('event.starts'), start), field(t('event.ends'), end)),
        values.map(({ f, input }) => field(f.name, input)),
        field(t('event.description'), desc),
        h('p', { class: 'muted small' }, t('event.planner_ignores')),
        foot(h('button', { type: 'button', class: 'danger', onclick: async () => {
          if (await write(async () => { confirmed(t('event.delete_confirm', { date: fmtDate(ev.date) })); await api('DELETE', `${epath}/${ev.id}`); })) { dlg.close(); render(); }
        } }, t('event.delete')),
        h('button', { type: 'button', class: 'ghost', onclick: () => { dlg.close(); copyDialog(ev); } }, '⧉ ' + t('event.copy')),
        h('button', { type: 'submit' }, t('common.save')))));
  }

  // «Every week until»: a box and a date, and how many events that gives.
  function weekly(dates) {
    const box = h('input', { type: 'checkbox', 'aria-label': t('copy.weekly') });
    const until = h('input', { type: 'date', disabled: true, 'aria-label': t('copy.until') });
    const count = h('span', { class: 'small muted', 'aria-live': 'polite' });
    const update = () => {
      until.disabled = !box.checked;
      const n = weeklyDates(dates(), box.checked ? until.value : '').length;
      count.textContent = n ? '→ ' + tn('copy.count', n) : '';
    };
    box.addEventListener('change', update);
    until.addEventListener('input', update);
    return { el: h('div', { class: 'row-inline' }, h('label', { class: 'check' }, box, ' ' + t('copy.weekly')), until, count), until: () => (box.checked ? until.value : ''), update };
  }

  // A copy of an event on one or more dates. What comes along is chosen; answers and the draft are never copied.
  function copyDialog(ev) {
    let dlg;
    const inputs = [];
    const list = h('div', { class: 'dates' });
    const dates = () => inputs.map((i) => i.value).filter(Boolean);
    const week = weekly(dates);
    const addDate = (v) => {
      const i = h('input', { type: 'date', value: v || '', 'aria-label': t('copy.date') });
      i.addEventListener('input', week.update);
      inputs.push(i);
      list.append(i);
      return i;
    };
    addDate(addDays(ev.date, 7));
    const check = (on, text, detail) => {
      const box = h('input', { type: 'checkbox', checked: on, 'aria-label': text });
      return { box, el: h('label', { class: 'check' }, box, ' ' + text, detail ? h('span', { class: 'muted small' }, ' ' + detail) : '') };
    };
    const details = fields.fields.filter((f) => fieldValue[ev.id + '/' + f.id]).map((f) => f.name);
    if (ev.description) details.push(t('event.description'));
    const placeText = tasks.filter((tk) => placesAt[at(ev, tk)]).map((tk) => `${tk.name} ${placesAt[at(ev, tk)]}`).join(', ');
    const times = check(true, t('copy.times'), clock(ev) || t('copy.none'));
    const det = check(true, t('copy.details'), details.join(', ') || t('copy.none'));
    const pl = check(true, t('copy.places'), placeText || t('copy.none'));
    const hand = check(false, t('copy.hand'));
    dlg = modal(t('copy.title_of', { label: fmtShortDate(ev.date) }),
      h('form', { class: 'stack', onsubmit: async (e) => {
        e.preventDefault();
        let made;
        if (await write(async () => {
          made = await api('POST', `${epath}/${ev.id}/copy`, { dates: dates(), until: week.until(), times: times.box.checked, details: det.box.checked, places: pl.box.checked, hand: hand.box.checked });
        })) { dlg.close(); await render(); say(tn('copy.done', made.length)); }
      } },
        field(t('copy.date'), list),
        h('div', {}, linkBtn('+ ' + t('copy.another_date'), () => { addDate('').focus(); week.update(); })),
        week.el,
        h('h3', {}, t('copy.comes_along')),
        times.el, det.el, pl.el, hand.el,
        h('p', { class: 'muted small' }, t('copy.never')),
        foot('', h('button', { type: 'submit' }, t('copy.button')))));
    week.update();
  }

  // A new event: empty, or a copy of one there is; optionally every week until a date.
  function newEventDialog() {
    let dlg;
    const last = events[events.length - 1];
    const date = h('input', { type: 'date', required: true, value: last ? addDays(last.date, 7) : '', 'aria-label': t('event.date') });
    const from = h('select', { 'aria-label': t('event.start_from') }, h('option', { value: '' }, t('event.empty')),
      [...events].reverse().map((ev) => h('option', { value: String(ev.id), selected: !!last && ev.id === last.id }, t('event.copy_of', { label: label(ev) }))));
    const start = h('input', { type: 'time', 'aria-label': t('event.starts') });
    const end = h('input', { type: 'time', 'aria-label': t('event.ends') });
    const week = weekly(() => [date.value]);
    date.addEventListener('input', week.update);
    const timesHint = h('p', { class: 'muted small' });
    const hint = () => { timesHint.textContent = from.value ? t('event.times_from_copy') : ''; };
    from.addEventListener('change', hint);
    dlg = modal(t('event.new'),
      h('form', { class: 'stack', onsubmit: async (e) => {
        e.preventDefault();
        let made;
        const times = { start_time: start.value, end_time: start.value ? end.value : '' };
        if (await write(async () => {
          made = from.value
            ? await api('POST', `${epath}/${from.value}/copy`, { dates: [date.value], until: week.until(), times: !start.value, ...times, details: true, places: true, hand: false })
            : await api('POST', epath, { date: date.value, until: week.until(), ...times });
        })) { dlg.close(); await render(); say(tn('event.added', made.length)); }
      } },
        h('div', { class: 'row-inline' }, field(t('event.date'), date), field(t('event.start_from'), from)),
        h('div', { class: 'row-inline' }, field(t('event.starts'), start), field(t('event.ends'), end)),
        timesHint,
        week.el,
        foot('', h('button', { type: 'submit' }, t('common.add')))));
    hint();
    week.update();
    date.focus();
  }

  // ✎ on a details row: its name, where it goes, delete.
  function fieldDialog(f) {
    let dlg;
    const name = h('input', { type: 'text', value: f.name, maxlength: '80', 'aria-label': t('common.name') });
    const others = fields.fields.filter((x) => x.id !== f.id);
    const now = fields.fields.findIndex((x) => x.id === f.id);
    const where = h('select', { 'aria-label': t('field.where') }, h('option', { value: '0', selected: now === 0 }, t('field.at_top')),
      others.map((x, i) => h('option', { value: String(i + 1), selected: now === i + 1 }, t('field.after', { name: x.name }))));
    dlg = modal(t('field.title', { name: f.name }),
      h('form', { class: 'stack', onsubmit: async (e) => {
        e.preventDefault();
        const pos = parseInt(where.value, 10);
        if (name.value.trim() === f.name && pos === now) { dlg.close(); return; }
        if (await write(async () => {
          if (name.value.trim() !== f.name) await api('PATCH', `${pbase}/fields/${f.id}`, { name: name.value });
          if (pos !== now) await api('PUT', `${pbase}/fields/${f.id}/position`, { position: pos });
        })) { dlg.close(); render(); }
      } },
        field(t('common.name'), name),
        others.length ? field(t('field.where'), where) : '',
        h('p', { class: 'muted small' }, t('field.hint')),
        foot(h('button', { type: 'button', class: 'danger', onclick: async () => {
          if (await write(async () => { confirmed(t('field.delete_confirm', { name: f.name })); await api('DELETE', `${pbase}/fields/${f.id}`); })) { dlg.close(); render(); }
        } }, t('field.delete')), h('button', { type: 'submit' }, t('common.save')))));
  }

  function taskDialog(tk) {
    let dlg;
    const changed = () => { again('task', tk.id); dlg.close(); render(); };
    const name = h('input', { type: 'text', value: tk.name, maxlength: '80', 'aria-label': t('common.name') });
    const planned = h('input', { type: 'checkbox', checked: tk.planned, 'aria-label': t('task.planned_aria', { name: tk.name }) });
    planned.addEventListener('change', async () => { if (await write(() => api('PATCH', `${base}/tasks/${tk.id}`, { planned: planned.checked }))) changed(); else planned.checked = tk.planned; });
    const others = tasks.filter((x) => x.id !== tk.id);
    const levelWith = (o) => (combos.find((c) => c.a === Math.min(tk.id, o.id) && c.b === Math.max(tk.id, o.id)) || { level: 'standard' }).level;
    const alone = others.length > 0 && others.every((o) => levelWith(o) === 'cannot');
    const aloneBox = h('input', { type: 'checkbox', checked: alone, disabled: !others.length, 'aria-label': t('task.alone') });
    aloneBox.addEventListener('change', async () => {
      const on = aloneBox.checked;
      // unticking sets every pair with this task back to standard, also ones set by hand: ask first
      if (await write(async () => { if (!on) confirmed(t('task.alone_off_confirm', { name: tk.name })); await api('PUT', `${base}/tasks/${tk.id}/combinations`, { level: on ? 'cannot' : 'standard' }); })) changed();
      else aloneBox.checked = !on;
    });
    const pairRows = others.map((o) => {
      const cur = levelWith(o);
      return h('tr', {}, h('td', {}, o.name), h('td', {}, h('select', { 'aria-label': `${tk.name} + ${o.name}`, onchange: async (e) => {
        if (await write(() => api('PUT', base + '/combinations', { a: tk.id, b: o.id, level: e.target.value }))) changed(); else e.target.value = cur;
      } }, SELECT_LEVELS.map((v) => h('option', { value: v, selected: v === cur }, t('combo.' + v))))));
    });
    dlg = modal(tk.name,
      h('p', { class: 'muted small' }, t('task.org_wide', { org: org.name })),
      h('form', { class: 'row', onsubmit: async (e) => {
        e.preventDefault();
        if (name.value.trim() !== tk.name && await write(() => api('PATCH', `${base}/tasks/${tk.id}`, { name: name.value }))) changed();
      } }, name, h('button', { type: 'submit' }, t('common.rename'))),
      h('label', { class: 'check' }, planned, ' ' + t('task.planned'), h('span', { class: 'muted small' }, ' ' + t('task.planned_off'))),
      h('h3', {}, t('task.combine')),
      h('label', { class: 'check' }, aloneBox, ' ', h('strong', {}, t('task.alone'))),
      pairRows.length ? h('table', { class: 'mini' }, h('tbody', {}, pairRows)) : '',
      h('p', { class: 'muted small' }, t('task.places_in_table')),
      foot(h('button', { type: 'button', class: 'danger', onclick: async () => {
        if (await write(async () => { confirmed(t('task.delete_confirm', { name: tk.name })); await api('DELETE', `${base}/tasks/${tk.id}`); })) { dlg.close(); render(); }
      } }, t('task.delete')), closeBtn()));
  }

  function personDialog(p) {
    let dlg;
    const changed = () => { again('person', p.id); dlg.close(); render(); };
    const ppath = `${base}/people/${p.id}`;
    const name = h('input', { type: 'text', value: p.name, maxlength: '80', 'aria-label': t('common.name') });
    const flag = (k, text, aria, off) => {
      const box = h('input', { type: 'checkbox', checked: p[k], 'aria-label': aria });
      box.addEventListener('change', async () => { if (await write(() => api('PATCH', ppath, { [k]: box.checked }))) changed(); else box.checked = p[k]; });
      return h('label', { class: 'check' }, box, ' ' + text, off ? h('span', { class: 'muted small' }, ' ' + off) : '');
    };
    const levelOf = (tk) => ((chosen[p.id] || []).find((x) => x.task_id === tk.id) || { level: '' }).level;
    const taskRows = tasks.filter((tk) => tk.planned).map((tk) => {
      const cur = levelOf(tk);
      return h('tr', {}, h('td', { class: cur ? '' : 'muted' }, tk.name), h('td', {}, h('select', { 'aria-label': t('person.task_aria', { name: p.name, task: tk.name }), onchange: async (e) => {
        const v = e.target.value;
        if (await write(() => (v ? api('PUT', `${ppath}/tasks/${tk.id}`, { level: v }) : api('DELETE', `${ppath}/tasks/${tk.id}`)))) changed(); else e.target.value = cur;
      } }, PERSON_TASK_LEVELS.map(([v, k]) => h('option', { value: v, selected: v === cur }, t(k))))));
    });
    const url = location.origin + `/o/${enc(r.org)}/p/${enc(r.plan)}/me/${p.id}`;
    const urlInput = h('input', { type: 'text', readonly: true, value: url, class: 'wide', 'aria-label': t('person.link'), onfocus: (e) => e.target.select() });
    const copy = async () => {
      try { await navigator.clipboard.writeText(url); } catch (e) { urlInput.select(); document.execCommand('copy'); }
      say(t('person.link_copied'));
    };
    dlg = modal(p.name,
      p.active ? '' : h('p', { class: 'err' }, t('person.inactive')),
      h('form', { class: 'row', onsubmit: async (e) => {
        e.preventDefault();
        if (name.value.trim() !== p.name && await write(() => api('PATCH', ppath, { name: name.value }))) changed();
      } }, name, h('button', { type: 'submit' }, t('common.rename'))),
      h('h3', {}, t('person.tasks')),
      taskRows.length ? h('table', { class: 'mini' }, h('tbody', {}, taskRows)) : h('p', { class: 'muted' }, t('person.no_tasks')),
      h('h3', {}, t('person.link')),
      h('div', { class: 'row-inline' }, urlInput, h('button', { type: 'button', class: 'ghost sm', onclick: copy }, t('person.copy'))),
      h('p', { class: 'muted small' }, t('person.link_hint', { name: p.name })),
      flag('active', t('person.active'), t('person.active_aria', { name: p.name })),
      flag('proposable', t('person.proposable', { name: p.name }), t('person.proposable_aria', { name: p.name }), t('person.proposable_off')),
      foot(h('span', { class: 'small muted' }, t('person.org_wide', { org: org.name })), closeBtn()));
  }

  // A place in the draft: what is set by hand (violet), the planner's names (lock one to keep it), and whom the planner
  // may pick there.
  function placeDialog(ev, tk) {
    let dlg;
    const k = at(ev, tk);
    const changed = () => { again('place', k); dlg.close(); render(); };
    const act = (fn) => async () => { if (await write(fn)) changed(); };
    const s = slotsAt(ev, tk);
    const rows = s.items.map((x) => h('tr', {},
      h('td', {}, slot(x)),
      h('td', { class: 'small muted' }, x.hand ? t('place.by_hand') : t('place.from_planner')),
      h('td', { class: 'right' }, x.cell
        ? linkBtn(x.locked ? t('place.unlock') : t('common.remove'), act(() => api('DELETE', `${pbase}/cells/${x.cell.id}`)), t(x.locked ? 'place.unlock_aria' : 'place.remove_aria', { name: x.name }))
        : h('button', { type: 'button', class: 'ghost sm', 'aria-label': t('place.lock_aria', { name: x.name }),
          onclick: act(() => api('POST', pbase + '/cells', { events: [ev.id], task: tk.id, [x.kind]: x.id, fills: x.kind === 'group' ? x.fills : 0, locked: true })) }, t('place.lock')))));
    const active = people.filter((p) => p.active);
    const kind = h('select', { 'aria-label': t('place.kind') }, [['person', t('common.person')], ['group', t('common.group')], ['text', t('common.text')]].map(([v, text]) => h('option', { value: v }, text)));
    const personSel = h('select', { 'aria-label': t('common.person') }, active.map((p) => h('option', { value: String(p.id) }, p.name)));
    const groupSel = h('select', { 'aria-label': t('common.group'), hidden: true }, groups.map((g) => h('option', { value: String(g.id) }, g.name)));
    const textIn = h('input', { type: 'text', maxlength: '80', placeholder: t('place.text_placeholder'), 'aria-label': t('common.text'), hidden: true });
    const fills = h('input', { type: 'number', min: '1', max: '50', value: '1', 'aria-label': t('place.fills'), hidden: true });
    kind.addEventListener('change', () => { personSel.hidden = kind.value !== 'person'; groupSel.hidden = kind.value !== 'group'; textIn.hidden = kind.value !== 'text'; fills.hidden = kind.value === 'person'; });
    const setForm = h('form', { class: 'row-inline', onsubmit: async (e) => {
      e.preventDefault();
      const body = { events: [ev.id], task: tk.id };
      if (kind.value === 'person') body.person = parseInt(personSel.value, 10);
      else if (kind.value === 'group') { body.group = parseInt(groupSel.value, 10); body.fills = parseInt(fills.value, 10) || 1; }
      else { body.label = textIn.value; body.fills = parseInt(fills.value, 10) || 1; }
      if (await write(() => api('POST', pbase + '/cells', body))) changed();
    } }, kind, personSel, groupSel, textIn, fills, h('button', { type: 'submit', 'aria-label': t('place.add') }, '+'));

    let poolPart = h('p', { class: 'muted small' }, t('place.not_planned'));
    if (tk.planned) {
      const pool = poolAt[k] || { people: [], groups: [] };
      const takes = (p) => (chosen[p.id] || []).some((x) => x.task_id === tk.id && x.level !== 'never');
      const pBoxes = active.filter((p) => takes(p) || pool.people.includes(p.id)).map((p) => ({ p, box: h('input', { type: 'checkbox', checked: pool.people.includes(p.id), 'aria-label': t('place.pool_person_aria', { name: p.name }) }) }));
      const gBoxes = groups.filter((g) => g.tasks.some((x) => x.task_id === tk.id) || pool.groups.includes(g.id)).map((g) => ({ g, box: h('input', { type: 'checkbox', checked: pool.groups.includes(g.id), 'aria-label': t('place.pool_group_aria', { name: g.name }) }) }));
      poolPart = [h('h3', {}, t('place.pool')),
        h('p', { class: 'muted small' }, pool.people.length + pool.groups.length ? t('place.pool_set') : t('place.pool_anyone', { task: tk.name })),
        pBoxes.length + gBoxes.length ? [h('div', { class: 'chips' }, pBoxes.map(({ p, box }) => h('label', { class: 'chip' }, box, ' ' + p.name)), gBoxes.map(({ g, box }) => h('label', { class: 'chip' }, box, ' ' + t('place.group_suffix', { name: g.name })))),
          h('div', { class: 'row-inline' }, h('button', { type: 'button', class: 'ghost sm', onclick: act(() => api('PUT', pbase + '/pools', {
            events: [ev.id], task: tk.id, people: pBoxes.filter((x) => x.box.checked).map((x) => x.p.id), groups: gBoxes.filter((x) => x.box.checked).map((x) => x.g.id) })) }, t('place.pool_save')))]
          : h('p', { class: 'muted small' }, t('place.pool_nobody', { task: tk.name }))];
    }
    const hands = s.items.filter((x) => x.hand);
    dlg = modal(`${tk.name} · ${fmtShortDate(ev.date)}`,
      h('p', { class: 'muted small' }, tn('place.places', s.n)),
      done && s.open && s.why ? h('p', { class: 'warn small' }, t('place.why', { why: s.why })) : '',
      rows.length ? h('table', { class: 'mini' }, h('tbody', {}, rows)) : h('p', { class: 'muted small' }, t('place.nothing')),
      h('h3', {}, t('place.set_by_hand')), setForm,
      poolPart,
      h('p', { class: 'muted small' }, t('place.lock_hint')),
      foot(hands.length ? h('button', { type: 'button', class: 'danger', onclick: act(async () => {
        confirmed(t('place.clear_confirm', { task: tk.name, date: fmtShortDate(ev.date) }));
        for (const x of hands) await api('DELETE', `${pbase}/cells/${x.cell.id}`);
      }) }, t('place.clear')) : '', closeBtn()));
  }

  // ⚙: the plan's name, the weighting, the earlier drafts and the pin, delete.
  function settingsDialog() {
    let dlg;
    const name = h('input', { type: 'text', value: plan.name, maxlength: '80', 'aria-label': t('settings.name') });
    const runRows = runs.map((x) => {
      const onShow = !!view && x.id === view.run.id;
      return h('tr', { class: onShow ? 'shown' : false },
        h('td', {}, t('run.label', { id: x.id, time: fmtTime(x.finished_at || x.started_at) })),
        h('td', {}, x.status !== 'done' ? h('span', { class: 'tag' }, t('run.status.' + x.status)) : '', x.pinned ? h('span', { class: 'chip pin' }, t('run.current')) : ''),
        h('td', { class: 'right' }, onShow ? h('span', { class: 'small muted' }, t('run.on_show'))
          : linkBtn(t('run.show'), () => { planState.runId = x.id; dlg.close(); render(); }, t('run.show_aria', { id: x.id })),
        x.status === 'done' && !x.pinned ? [' · ', linkBtn(t('run.pin_short'), () => { dlg.close(); pin(x.id, true); }, t('run.pin_aria', { id: x.id }))] : ''));
    });
    dlg = modal(plan.name,
      h('form', { class: 'row', onsubmit: async (e) => {
        e.preventDefault();
        if (name.value.trim() !== plan.name && await write(() => api('PATCH', pbase, { name: name.value }))) { dlg.close(); await render(); say(t('settings.renamed')); }
      } }, name, h('button', { type: 'submit' }, t('common.rename'))),
      h('h3', {}, t('weights.title')),
      weightsForm(pbase + '/weights', weights, (text) => { again('settings', 0); dlg.close(); render().then(() => say(text)); }),
      h('h3', {}, t('run.history')),
      runs.length ? h('table', { class: 'mini' }, h('tbody', {}, runRows)) : h('p', { class: 'muted small' }, t('run.no_history')),
      foot(h('button', { type: 'button', class: 'danger', onclick: async () => {
        if (await write(async () => { confirmed(t('settings.delete_confirm', { name: plan.name })); await api('DELETE', pbase); })) { dlg.close(); go(`/o/${enc(r.org)}`); say(t('settings.deleted', { name: plan.name })); }
      } }, t('settings.delete')), closeBtn()));
  }

  const listDialog = (title, items) => modal(title, h('ul', { class: 'small' }, items.map((x) => h('li', {}, x))), foot('', closeBtn()));

  // ---- the table
  const cols = events.length;
  const filler = () => h('td', { class: 'addcol' });
  const sect = (title, hint, cls) => h('tr', { class: 'sect' + (cls ? ' ' + cls : '') }, h('th', { scope: 'row' }, title),
    h('td', { colspan: String(cols + 1) }, hint ? h('span', { class: 'small' }, hint) : ''));
  const addLine = (form) => h('tr', { class: 'adds' }, h('th', { scope: 'row' }, form), h('td', { colspan: String(cols + 1) }));

  const head = h('tr', {}, h('th', { scope: 'row', class: 'corner' }, t('table.date')),
    events.map((ev) => h('th', { scope: 'col' }, h('div', { class: 'rl' },
      h('button', { type: 'button', class: 'cellbtn', 'aria-label': t('table.event_aria', { label: label(ev) }), onclick: () => eventDialog(ev) }, fmtShortDate(ev.date)),
      h('button', { type: 'button', class: 'edit', title: t('copy.title'), 'aria-label': t('copy.aria', { label: label(ev) }), onclick: () => copyDialog(ev) }, '⧉')),
      clock(ev) ? h('span', { class: 'sub' }, clock(ev)) : '', ev.description ? h('span', { class: 'sub' }, t('table.has_description')) : '')),
    h('th', { scope: 'col', class: 'addcol' }, h('button', { type: 'button', class: 'ghost', 'aria-label': t('table.add_event'), title: t('table.add_event'), onclick: newEventDialog }, '+')));

  // A text in a cell is saved when the field is left, without redrawing the page (so the next field keeps the focus).
  const textCell = (cur, placeholder, aria, save) => {
    let value = cur;
    const input = enterMovesOn(h('input', { type: 'text', value, maxlength: '200', placeholder, 'aria-label': aria }));
    input.addEventListener('change', async () => {
      if (input.value.trim() === value) { input.value = value; return; }
      if (await write(() => save(input.value))) value = input.value.trim();
      input.value = value;
    });
    return h('td', { class: 'text' }, input);
  };
  // Details rows: ⋮⋮ drags a row to another place, ✎ opens it.
  let dragged = null;
  const fieldRow = (f, i) => {
    const grip = h('span', { class: 'grip', draggable: 'true', title: t('field.drag'), 'aria-hidden': 'true' }, '⋮⋮');
    const row = h('tr', { class: 'field-row' }, h('th', { scope: 'row' }, h('div', { class: 'rl' }, grip, h('span', { class: 'nm' }, f.name),
      h('button', { type: 'button', class: 'edit', 'aria-label': t('field.edit_aria', { name: f.name }), onclick: () => fieldDialog(f) }, '✎'))),
    events.map((ev) => textCell(fieldValue[ev.id + '/' + f.id] || '', t('table.add_value', { name: f.name.toLowerCase() }), t('table.value_aria', { name: f.name, label: label(ev) }),
      async (v) => { await api('PUT', `${epath}/${ev.id}/fields/${f.id}`, { value: v }); fieldValue[ev.id + '/' + f.id] = v.trim(); })), filler());
    grip.addEventListener('dragstart', (e) => { dragged = f; e.dataTransfer.effectAllowed = 'move'; e.dataTransfer.setData('text/plain', f.name); });
    grip.addEventListener('dragend', () => { dragged = null; });
    row.addEventListener('dragover', (e) => { if (dragged && dragged.id !== f.id) { e.preventDefault(); row.classList.add('drop'); } });
    row.addEventListener('dragleave', () => row.classList.remove('drop'));
    row.addEventListener('drop', async (e) => {
      e.preventDefault();
      row.classList.remove('drop');
      const d = dragged;
      dragged = null;
      if (d && d.id !== f.id && await write(() => api('PUT', `${pbase}/fields/${d.id}/position`, { position: i }))) render();
    });
    return row;
  };

  // The draft's cells, so a change of places can redraw the cell below without redrawing the page.
  const draftTd = {};
  const draftCell = (ev, tk) => {
    const s = slotsAt(ev, tk);
    if (!s.n && !s.items.length) return h('td', { class: 'draft none' }, h('span', { class: 'muted small', 'aria-hidden': 'true' }, '–'));
    const lines = s.items.map(slot);
    if (s.open && done) for (let i = 0; i < s.open; i++) lines.push(h('span', { class: 'slot empty' }, t('draft.not_filled'), s.why ? [' · ', h('span', { class: 'why' }, t('draft.why'))] : ''));
    else if (s.open) lines.push(h('span', { class: 'slot open' }, tn('draft.open', s.open)));
    return h('td', { class: 'draft' }, h('button', { type: 'button', class: 'draftbtn', 'aria-label': t('draft.place_aria', { task: tk.name, label: label(ev) }), onclick: () => placeDialog(ev, tk) }, lines));
  };
  const putDraftCell = (ev, tk) => { const td = draftCell(ev, tk); if (draftTd[at(ev, tk)]) draftTd[at(ev, tk)].replaceWith(td); draftTd[at(ev, tk)] = td; return td; };

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
      const path = `${epath}/${ev.id}/places/${tk.id}`;
      if (await write(() => (n === 0 ? api('DELETE', path) : api('PUT', path, { slots: n })))) { if (n) placesAt[k] = n; else delete placesAt[k]; putDraftCell(ev, tk); }
      paint();
    });
    paint();
    return h('td', { class: 'places' }, input);
  };
  const taskHead = (tk) => h('th', { scope: 'row' }, h('button', { type: 'button', class: 'cellbtn', 'aria-label': t('table.task_aria', { name: tk.name }), onclick: () => taskDialog(tk) },
    tk.name, tk.planned ? '' : h('span', { class: 'sub' }, t('table.by_hand'))));

  // Availability: one click on a cell moves the answer on; the count above each event follows.
  const countCells = {};
  const recount = (evId) => {
    const c = { good: 0, poor: 0, never: 0 };
    people.forEach((p) => { const l = p.active && (ans[p.id] || {})[evId]; if (l) c[l]++; });
    countCells[evId].replaceChildren(h('b', { class: 'good' }, '✓ ' + c.good), ' · ', h('b', { class: 'poor' }, '? ' + c.poor), ' · ', h('b', { class: 'never' }, '✗ ' + c.never));
    countCells[evId].setAttribute('aria-label', t('table.count', c));
  };
  const answerCell = (p, ev) => {
    const b = h('button', { type: 'button', class: 'ans' });
    let busy = false;  // a second click while the first is on its way would send the same answer twice
    const paint = () => {
      const l = (ans[p.id] || {})[ev.id] || '';
      b.textContent = ANSWER_MARK[l];
      b.dataset.level = l || 'none';
      b.setAttribute('aria-label', t('table.answer_aria', { name: p.name, label: label(ev), answer: t('answer.' + (l || 'none')) }));
    };
    b.addEventListener('click', async () => {
      if (busy) return;
      busy = true;
      const next = ANSWER_NEXT[(ans[p.id] || {})[ev.id] || ''];
      if (await write(() => api('PUT', `${pbase}/people/${p.id}/availability`, { levels: { [ev.id]: next } }))) { (ans[p.id] ||= {})[ev.id] = next; paint(); recount(ev.id); }
      busy = false;
    });
    paint();
    return h('td', { class: 'answer' }, b);
  };
  const personRow = (p) => {
    const mine = (chosen[p.id] || []).filter((x) => x.level !== 'never' && tasks.some((tk) => tk.id === x.task_id && tk.planned)).map((x) => nameOf(tasks, x.task_id));
    return h('tr', { class: [highlight && p.id === highlight.id ? 'me' : '', p.active ? '' : 'inactive'].filter(Boolean).join(' ') || false, 'data-person': String(p.id) },
      h('th', { scope: 'row' }, h('button', { type: 'button', class: 'cellbtn', 'aria-label': t('table.person_aria', { name: p.name }), onclick: () => personDialog(p) },
        p.name, h('span', { class: 'sub' + (mine.length ? '' : ' prompt') }, mine.length ? mine.join(', ') : t('table.choose_tasks')),
        p.active ? '' : h('span', { class: 'sub' }, t('person.inactive_short')))),
      events.map((ev) => answerCell(p, ev)), filler());
  };

  const draftFoot = done && (view.weak.length || view.load.length || runs.length > 1)
    ? h('tr', { class: 'draft-row foot' }, h('th', { scope: 'row' }), h('td', { colspan: String(cols + 1) }, h('div', { class: 'row-inline small' },
      view.weak.length ? linkBtn(tn('draft.compromises', view.weak.length), () => listDialog(tn('draft.compromises', view.weak.length), view.weak)) : '',
      view.load.length ? linkBtn(t('draft.distribution'), () => listDialog(t('draft.distribution'), view.load.map((l) => t('run.load_line', { name: l.name, count: l.count, target: l.target.toLocaleString(locale()) })))) : '',
      runs.length > 1 ? linkBtn(t('draft.earlier'), settingsDialog) : ''))) : '';

  const table = h('div', { class: 'grid-scroll' }, h('table', { class: 'dp-table' },
    h('thead', {}, head),
    h('tbody', {},
      sect(t('table.details'), t('table.details_hint')),
      fields.fields.map(fieldRow),
      addLine(addRow(t('table.new_field'), (name) => api('POST', pbase + '/fields', { name }))),
      sect(t('table.tasks'), t('table.tasks_hint')),
      tasks.map((tk) => h('tr', {}, taskHead(tk), events.map((ev) => placesCell(tk, ev)), filler())),
      addLine(addRow(t('table.new_task'), (name) => api('POST', base + '/tasks', { name }))),
      h('tr', { class: 'sect draft' }, h('th', { scope: 'row' }, t('draft.title')), h('td', { colspan: String(cols + 1) }, bar)),
      tasks.map((tk) => h('tr', { class: 'draft-row' }, h('th', { scope: 'row' }, tk.name), events.map((ev) => putDraftCell(ev, tk)), filler())),
      draftFoot,
      h('tr', { class: 'sect' }, h('th', { scope: 'row' }, t('table.availability')), events.map((ev) => (countCells[ev.id] = h('td', { class: 'count small' }))), filler()),
      people.filter((p) => p.active).map(personRow), people.filter((p) => !p.active).map(personRow),
      addLine(addRow(t('table.new_person'), (name) => api('POST', base + '/people', { name }))))));
  events.forEach((ev) => recount(ev.id));

  const root = h('div', { class: 'plan-page' },
    h('p', { class: 'small' }, link(`/o/${enc(r.org)}`, t('common.back', { name: org.name }))),
    h('div', { class: 'title-bar' }, h('h1', {}, plan.name), h('button', { type: 'button', class: 'ghost', onclick: settingsDialog }, '⚙ ' + t('settings.open'))),
    h('p', { class: 'muted small' }, events.length ? t('table.hint') : t('table.no_events')),
    table);
  if (highlight) root.dataset.person = highlight.name;
  root.shown = () => reopened({
    task: (id) => { const x = tasks.find((tk) => tk.id === id); if (x) taskDialog(x); },
    person: (id) => { const x = people.find((p) => p.id === id); if (x) personDialog(x); },
    place: (k) => { const [e, tk] = k.split('/').map(Number); const ev = events.find((x) => x.id === e), task = tasks.find((x) => x.id === tk); if (ev && task) placeDialog(ev, task); },
    settings: () => settingsDialog(),
  });
  return root;
}

let renderSeq = 0;
let lastPath = '';
async function render() {
  const seq = ++renderSeq;
  const main = document.getElementById('main');
  const r = route();
  try {
    let people = [];
    let org = null;
    if (r.org) {
      [org, people] = await Promise.all([api('GET', '/orgs/' + enc(r.org)), api('GET', `/orgs/${enc(r.org)}/people`)]);
      store.set('org', r.org);
    }
    const [, body] = await Promise.all([renderHeader(r), r.page === 'home' ? pageHome() : r.page === 'org' ? pageOrg(r, org, people) : pagePlan(r, org, people)]);
    if (seq !== renderSeq || !body) return;
    const y = location.pathname === lastPath ? window.scrollY : 0;  // the same page again (after a change): stay where we are
    main.replaceChildren(body);
    lastPath = location.pathname;
    window.scrollTo(0, y);
    if (body.shown) body.shown();
    if (r.page === 'me' && body.dataset.person && !y) { const row = body.querySelector('tr.me'); if (row) row.scrollIntoView({ block: 'center' }); }
    document.title = t('brand') + (org ? ' · ' + org.name : '');
  } catch (e) {
    if (seq !== renderSeq) return;
    if (e.status === 404 && r.org && !e.keepOrg) store.set('org', '');
    main.replaceChildren(h('div', {}, h('h1', {}, e.status === 404 ? t('page.not_found') : t('page.error')), h('p', {}, link('/?switch', t('page.choose_org'))), h('p', { class: 'muted small' }, errorText(e))));
  }
}

window.addEventListener('popstate', render);
render();
