'use strict';
// Dutyplan: a small frontend without a framework. The organisation is in the address (/o/<org>/...), so switching
// organisation is just going to another address. No login: «Who are you?» is self-declared and used in the change log.
// All texts come from i18n.js (t, tn); the language is a per-browser choice.

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
  for (const kid of kids.flat()) el.append(kid instanceof Node ? kid : document.createTextNode(String(kid ?? '')));
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
  if (method === 'GET' && /\/(runs|assignments)/.test(path)) path += (path.includes('?') ? '&' : '?') + 'lang=' + currentLang;
  const headers = {};
  if (body !== undefined) headers['Content-Type'] = 'application/json';
  if (method !== 'GET') headers['X-Actor'] = encodeURIComponent(store.get('actor'));
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

// Writes: need a name at the top, and show the error instead of throwing.
async function write(action) {
  if (!store.get('actor').trim()) {
    say(t('common.need_actor'), 'err');
    return false;
  }
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
const fmtTime = (iso) => new Date(iso).toLocaleString(locale(), { dateStyle: 'short', timeStyle: 'short' });
const fmtDate = (iso) => new Date(iso + 'T00:00:00Z').toLocaleDateString(locale(), { weekday: 'short', day: 'numeric', month: 'short', year: 'numeric', timeZone: 'UTC' });
const fmtShortDate = (iso) => new Date(iso + 'T00:00:00Z').toLocaleDateString(locale(), { weekday: 'short', day: 'numeric', month: 'short', timeZone: 'UTC' });

// The time of day of an event: «11:00» or «11:00–12:30», or '' when it has none. A 24-hour clock in both languages.
const clock = (ev) => (ev.start_time ? (ev.end_time ? `${ev.start_time}–${ev.end_time}` : ev.start_time) : '');
// What follows the date of an event: its time, day type and note.
const evMeta = (ev) => [clock(ev), ev.day_type, ev.note].filter(Boolean).join(' · ');
// A date with its time, as used in labels: «2026-08-09 11:00 Match day».
const evLabel = (ev) => `${ev.date}${ev.start_time ? ' ' + ev.start_time : ''} ${ev.day_type}`;

function go(path) {
  history.pushState({}, '', path);
  render();
}
function link(path, text, props) {
  return h('a', { href: path, ...props, onclick: (e) => { if (e.metaKey || e.ctrlKey || e.shiftKey) return; e.preventDefault(); go(path); } }, text);
}

function route() {
  const parts = location.pathname.split('/').filter(Boolean);
  if (parts[0] === 'o' && parts[1]) {
    if (parts[2] === 'p' && parts[3] && parts[4] === 'me' && parts[5]) return { page: 'me', org: parts[1], plan: parts[3], person: parts[5] };
    if (parts[2] === 'p' && parts[3]) return { page: 'plan', org: parts[1], plan: parts[3] };
    if (parts[2] === 'setup') return { page: 'setup', org: parts[1] };
    return { page: 'org', org: parts[1] };
  }
  return { page: 'home' };
}

async function renderHeader(r, people) {
  const orgs = await api('GET', '/orgs');
  const select = h('select', { 'aria-label': t('header.org'), onchange: (e) => go(e.target.value === '' ? '/?switch' : '/o/' + enc(e.target.value)) },
    h('option', { value: '' }, r.org ? t('header.switch_org') : t('header.choose_org')),
    orgs.map((o) => h('option', { value: o.slug, selected: o.slug === r.org }, o.name)));
  const actor = h('input', { type: 'text', id: 'actor', list: 'people', value: store.get('actor'), placeholder: t('header.who_placeholder'), maxlength: '80', autocomplete: 'off',
    onchange: (e) => store.set('actor', e.target.value.trim()) });
  const lang = h('select', { id: 'lang', 'aria-label': t('lang.label'), onchange: (e) => { setLang(e.target.value); render(); } },
    LANGS.map((l) => h('option', { value: l, selected: l === currentLang }, t('lang.' + l))));
  document.getElementById('top').replaceChildren(
    link('/?switch', t('brand'), { class: 'brand' }),
    h('label', {}, t('header.org'), select),
    h('span', { class: 'spacer' }),
    h('label', { title: t('header.who_title') }, t('header.who'), actor),
    h('label', { class: 'lang' }, h('span', { class: 'sr-only' }, t('lang.label')), lang),
    h('datalist', { id: 'people' }, (people || []).filter((p) => p.active).map((p) => h('option', { value: p.name }))));
}

function nameForm(placeholder, button, onsubmit) {
  const input = h('input', { type: 'text', placeholder, maxlength: '80', required: true, 'aria-label': placeholder });
  return h('form', { class: 'row', onsubmit: async (e) => {
    e.preventDefault();
    if (await write(() => onsubmit(input.value))) input.value = '';
  } }, input, h('button', { type: 'submit' }, button));
}

async function pageHome() {
  const orgs = await api('GET', '/orgs');
  const last = store.get('org');
  if (last && !location.search.includes('switch') && orgs.some((o) => o.slug === last)) return go('/o/' + enc(last));
  return h('div', {},
    h('h1', {}, t('home.title')),
    orgs.length ? h('ul', { class: 'plain' }, orgs.map((o) => h('li', {}, link('/o/' + enc(o.slug), o.name)))) : h('p', { class: 'muted' }, t('home.none')),
    h('h2', {}, t('home.new')),
    nameForm(t('home.name_placeholder'), t('common.create'), async (name) => {
      const o = await api('POST', '/orgs', { name });
      store.set('org', o.slug);
      go('/o/' + enc(o.slug));
    }),
    h('p', { class: 'muted small' }, t('home.warning')));
}

function logTable(entries) {
  if (!entries.length) return h('p', { class: 'muted' }, t('log.none'));
  return h('ul', { class: 'log' }, entries.map((e) => h('li', {},
    h('span', { class: 'when' }, fmtTime(e.at)), h('span', { class: 'who' }, e.actor),
    h('span', { class: 'what' }, e.action + (e.plan ? ' (' + e.plan + ')' : '')), h('span', { class: 'detail' }, e.detail))));
}

async function pageOrg(r, org, people) {
  const [plans, log] = await Promise.all([api('GET', `/orgs/${enc(r.org)}/plans`), api('GET', `/orgs/${enc(r.org)}/log?limit=30`)]);
  const refresh = () => render();
  const peopleRows = people.map((p) => h('tr', {},
    h('td', {}, h('input', { type: 'text', value: p.name, maxlength: '80', 'aria-label': t('common.name_of_aria', { name: p.name }),
      onchange: async (e) => { if (await write(() => api('PATCH', `/orgs/${enc(r.org)}/people/${p.id}`, { name: e.target.value }))) refresh(); else e.target.value = p.name; } })),
    h('td', {}, h('input', { type: 'checkbox', 'aria-label': t('org.active_aria', { name: p.name }), checked: p.active,
      onchange: async (e) => { if (await write(() => api('PATCH', `/orgs/${enc(r.org)}/people/${p.id}`, { active: e.target.checked }))) refresh(); else e.target.checked = p.active; } })),
    h('td', {}, h('input', { type: 'checkbox', 'aria-label': t('common.can_be_proposed_aria', { name: p.name }), checked: p.proposable,
      onchange: async (e) => { if (await write(() => api('PATCH', `/orgs/${enc(r.org)}/people/${p.id}`, { proposable: e.target.checked }))) refresh(); else e.target.checked = p.proposable; } }))));
  return h('div', {},
    h('h1', {}, org.name),
    h('p', {}, link(`/o/${enc(r.org)}/setup`, t('org.setup_link'))),
    h('h2', {}, t('org.plans')),
    plans.length ? h('ul', { class: 'plain' }, plans.map((p) => h('li', {}, link(`/o/${enc(r.org)}/p/${enc(p.slug)}`, p.name)))) : h('p', { class: 'muted' }, t('org.no_plans')),
    nameForm(t('org.new_plan_placeholder'), t('org.create_plan'), async (name) => { const p = await api('POST', `/orgs/${enc(r.org)}/plans`, { name }); go(`/o/${enc(r.org)}/p/${enc(p.slug)}`); }),
    h('h2', {}, t('org.people')),
    people.length ? h('table', {}, h('thead', {}, h('tr', {}, h('th', {}, t('common.name')), h('th', {}, t('org.col_active')), h('th', {}, t('common.can_be_proposed')))), h('tbody', {}, peopleRows)) : h('p', { class: 'muted' }, t('org.no_people')),
    nameForm(t('org.new_person'), t('common.add'), async (name) => { await api('POST', `/orgs/${enc(r.org)}/people`, { name }); refresh(); }),
    h('p', { class: 'muted small' }, t('org.proposable_hint')),
    h('h2', {}, t('log.title')),
    logTable(log),
    manageOrg(r, org, plans));
}

// Rename (the address stays) or delete an organisation. Deleting needs the organisation to have no plans, and the name typed in.
function manageOrg(r, org, plans) {
  const base = `/orgs/${enc(r.org)}`;
  const name = h('input', { type: 'text', value: org.name, maxlength: '80', 'aria-label': t('org.rename_label') });
  const rename = h('form', { class: 'row', onsubmit: async (e) => {
    e.preventDefault();
    if (await write(() => api('PATCH', base, { name: name.value }))) { await render(); say(t('org.renamed')); }
  } }, name, h('button', { type: 'submit' }, t('common.rename')));
  let del;
  if (plans.length) del = h('p', { class: 'muted' }, t('org.delete_has_plans'));
  else {
    const typed = h('input', { type: 'text', placeholder: t('org.delete_type'), 'aria-label': t('org.delete_type'), autocomplete: 'off', maxlength: '80' });
    const button = h('button', { type: 'submit', class: 'danger', disabled: true }, t('org.delete_button'));
    typed.addEventListener('input', () => { button.disabled = typed.value.trim() !== org.name; });
    del = h('form', { class: 'stack', onsubmit: async (e) => {
      e.preventDefault();
      if (typed.value.trim() !== org.name) return;
      if (await write(() => api('DELETE', `${base}?confirm=${enc(r.org)}`))) { store.set('org', ''); go('/?switch'); say(t('org.deleted', { name: org.name })); }
    } }, h('p', { class: 'muted small' }, t('org.delete_hint')), h('div', { class: 'row' }, typed, button));
  }
  return h('section', {}, h('details', {}, h('summary', {}, t('manage.title')), rename, h('h3', {}, t('org.delete_title')), del));
}

// The same for a plan: renaming keeps every link; deleting takes the plan's events, answers, cells and runs with it.
function managePlan(r, plan) {
  const base = `/orgs/${enc(r.org)}/plans/${enc(r.plan)}`;
  const name = h('input', { type: 'text', value: plan.name, maxlength: '80', 'aria-label': t('plan.rename_label') });
  const rename = h('form', { class: 'row', onsubmit: async (e) => {
    e.preventDefault();
    if (await write(() => api('PATCH', base, { name: name.value }))) { await render(); say(t('plan.renamed')); }
  } }, name, h('button', { type: 'submit' }, t('common.rename')));
  const del = h('div', { class: 'stack' }, h('p', { class: 'muted small' }, t('plan.delete_hint')),
    h('div', {}, h('button', { type: 'button', class: 'danger', onclick: async () => {
      if (!confirm(t('plan.delete_confirm', { name: plan.name }))) return;
      if (await write(() => api('DELETE', base))) { go(`/o/${enc(r.org)}`); say(t('plan.deleted', { name: plan.name })); }
    } }, t('plan.delete_button'))));
  return h('section', {}, h('details', {}, h('summary', {}, t('manage.title')), rename, h('h3', {}, t('plan.delete_title')), del));
}

const SELECT_LEVELS = ['standard', 'cannot', 'impractical', 'practical', 'preferred'];
const COMBO_OPTIONS = ['cannot', 'impractical', 'practical', 'preferred'];
const GROUP_LEVELS = ['poor', 'neutral', 'good'];

function section(title, ...kids) { return h('section', {}, h('h2', {}, title), ...kids); }

async function pageSetup(r, org) {
  const base = `/orgs/${enc(r.org)}`;
  const [tasks, dayTypes, combos, groups, people] = await Promise.all([api('GET', base + '/tasks'), api('GET', base + '/day-types'), api('GET', base + '/combinations'), api('GET', base + '/groups'), api('GET', base + '/people')]);
  const refresh = () => render();
  const act = (fn) => async () => { if (await write(fn)) refresh(); };

  const taskRows = tasks.map((tk) => h('tr', {},
    h('td', {}, h('input', { type: 'text', value: tk.name, maxlength: '80', 'aria-label': t('common.name_of_aria', { name: tk.name }),
      onchange: async (e) => { if (await write(() => api('PATCH', `${base}/tasks/${tk.id}`, { name: e.target.value }))) refresh(); else e.target.value = tk.name; } })),
    h('td', {}, h('input', { type: 'checkbox', 'aria-label': t('setup.planned_aria', { name: tk.name }), checked: tk.planned,
      onchange: async (e) => { if (await write(() => api('PATCH', `${base}/tasks/${tk.id}`, { planned: e.target.checked }))) refresh(); else e.target.checked = tk.planned; } })),
    h('td', {}, h('button', { type: 'button', class: 'danger', onclick: act(async () => { confirmed(t('setup.delete_task_confirm', { name: tk.name })); await api('DELETE', `${base}/tasks/${tk.id}`); }) }, t('common.delete')))));

  const pasted = h('textarea', { rows: '4', placeholder: t('setup.bulk_placeholder'), 'aria-label': t('setup.bulk_aria') });

  const dayTypeBlocks = dayTypes.map((d) => {
    const inputs = tasks.map((tk) => {
      const cur = d.tasks.find((x) => x.task_id === tk.id);
      return { tk, input: h('input', { type: 'number', min: '0', max: '50', value: cur ? String(cur.slots) : '0', 'aria-label': t('setup.places_aria', { task: tk.name, daytype: d.name }) }) };
    });
    const name = h('input', { type: 'text', value: d.name, maxlength: '80', 'aria-label': t('setup.daytype_name_aria') });
    return h('div', { class: 'card' },
      h('div', { class: 'row-inline' }, name,
        h('button', { type: 'button', onclick: act(() => api('PATCH', `${base}/day-types/${d.id}`, { name: name.value })) }, t('common.rename')),
        h('button', { type: 'button', class: 'danger', onclick: act(async () => { confirmed(t('setup.delete_daytype_confirm', { name: d.name })); await api('DELETE', `${base}/day-types/${d.id}`); }) }, t('common.delete'))),
      tasks.length ? h('table', {}, h('thead', {}, h('tr', {}, h('th', {}, t('setup.col_task')), h('th', {}, t('setup.col_places')))),
        h('tbody', {}, inputs.map(({ tk, input }) => h('tr', {}, h('td', {}, tk.name), h('td', {}, input))))) : h('p', { class: 'muted' }, t('setup.tasks_first')),
      tasks.length ? h('button', { type: 'button', onclick: act(() => api('PUT', `${base}/day-types/${d.id}/tasks`, {
        tasks: inputs.map(({ tk, input }) => ({ task_id: tk.id, slots: parseInt(input.value, 10) || 0 })).filter((x) => x.slots > 0),
      })) }, t('setup.save_tasks')) : '');
  });

  // Groups: a named set of people (team, choir, couple). No rules of their own; the planner may propose them for the tasks set up here,
  // and a manual cell can name them. The members count as busy with the task, and the combinations above decide what else they can have.
  const plannedTasks = tasks.filter((tk) => tk.planned);
  const groupBlocks = groups.map((g) => {
    const gname = h('input', { type: 'text', value: g.name, maxlength: '80', 'aria-label': t('setup.group_name_aria') });
    const boxes = people.filter((p) => p.active || g.members.includes(p.id)).map((p) => ({ p, box: h('input', { type: 'checkbox', checked: g.members.includes(p.id), 'aria-label': t('setup.member_aria', { person: p.name, group: g.name }) }) }));
    const rows = plannedTasks.map((tk) => {
      const cur = g.tasks.find((x) => x.task_id === tk.id);
      return { tk,
        fills: h('input', { type: 'number', min: '0', max: '50', value: cur ? String(cur.fills) : '0', 'aria-label': t('setup.fills_aria', { group: g.name, task: tk.name }) }),
        level: h('select', { 'aria-label': t('setup.level_aria', { group: g.name, task: tk.name }) }, GROUP_LEVELS.map((v) => h('option', { value: v, selected: v === (cur ? cur.level : 'neutral') }, t('level.' + v)))) };
    });
    return h('div', { class: 'card' },
      h('div', { class: 'row-inline' }, gname,
        h('button', { type: 'button', onclick: act(() => api('PATCH', `${base}/groups/${g.id}`, { name: gname.value })) }, t('common.rename')),
        h('label', { class: 'chip' }, h('input', { type: 'checkbox', checked: g.proposable, 'aria-label': t('common.can_be_proposed_aria', { name: g.name }),
          onchange: async (e) => { if (await write(() => api('PATCH', `${base}/groups/${g.id}`, { proposable: e.target.checked }))) refresh(); else e.target.checked = g.proposable; } }), ' ' + t('common.can_be_proposed')),
        h('button', { type: 'button', class: 'danger', onclick: act(async () => { confirmed(t('setup.delete_group_confirm', { name: g.name })); await api('DELETE', `${base}/groups/${g.id}`); }) }, t('common.delete'))),
      h('h3', {}, t('setup.members')),
      boxes.length ? h('div', { class: 'chips' }, boxes.map(({ p, box }) => h('label', { class: 'chip' }, box, ' ' + p.name + (p.active ? '' : ' ' + t('setup.inactive'))))) : h('p', { class: 'muted' }, t('setup.add_people_first')),
      boxes.length ? h('button', { type: 'button', onclick: act(() => api('PUT', `${base}/groups/${g.id}/members`, { people: boxes.filter((x) => x.box.checked).map((x) => x.p.id) })) }, t('setup.save_members')) : '',
      h('h3', {}, t('setup.group_proposed_for')),
      plannedTasks.length ? h('table', {}, h('thead', {}, h('tr', {}, h('th', {}, t('setup.col_task')), h('th', {}, t('setup.col_places_group')), h('th', {}, t('setup.col_how_much')))),
        h('tbody', {}, rows.map(({ tk, fills, level }) => h('tr', {}, h('td', {}, tk.name), h('td', {}, fills), h('td', {}, level))))) : h('p', { class: 'muted' }, t('setup.no_planned_tasks')),
      plannedTasks.length ? h('button', { type: 'button', onclick: act(() => api('PUT', `${base}/groups/${g.id}/tasks`, {
        tasks: rows.map(({ tk, fills, level }) => ({ task_id: tk.id, fills: parseInt(fills.value, 10) || 0, level: level.value })).filter((x) => x.fills > 0),
      })) }, t('setup.save_tasks')) : '');
  });

  const taskName = (id) => (tasks.find((tk) => tk.id === id) || {}).name;
  const level = (a, b) => (combos.find((c) => c.a === a && c.b === b) || { level: 'standard' }).level;
  const pairRows = [];
  for (let i = 0; i < tasks.length; i++) for (let j = i + 1; j < tasks.length; j++) {
    const x = tasks[i], y = tasks[j];
    const cur = level(Math.min(x.id, y.id), Math.max(x.id, y.id));
    const select = h('select', { 'aria-label': `${x.name} + ${y.name}`, onchange: async (e) => { if (!(await write(() => api('PUT', base + '/combinations', { a: x.id, b: y.id, level: e.target.value })))) e.target.value = cur; } },
      SELECT_LEVELS.map((v) => h('option', { value: v, selected: v === cur }, t('combo.' + v))));
    pairRows.push(h('tr', {}, h('td', {}, taskName(x.id) + ' + ' + taskName(y.id)), h('td', {}, select)));
  }

  return h('div', {},
    h('p', { class: 'small' }, link(`/o/${enc(r.org)}`, t('common.back', { name: org.name }))),
    h('h1', {}, t('setup.title')),
    h('p', { class: 'muted' }, t('setup.intro')),
    section(t('setup.tasks'),
      tasks.length ? h('table', {}, h('thead', {}, h('tr', {}, h('th', {}, t('common.name')), h('th', {}, t('setup.col_planned')), h('th', {}, ''))), h('tbody', {}, taskRows)) : h('p', { class: 'muted' }, t('setup.none_tasks')),
      nameForm(t('setup.new_task'), t('common.add'), async (name) => { await api('POST', base + '/tasks', { name }); refresh(); }),
      h('form', { class: 'stack', onsubmit: async (e) => { e.preventDefault(); if (await write(async () => { await api('POST', base + '/tasks/bulk', { names: pasted.value }); })) refresh(); } },
        pasted, h('button', { type: 'submit' }, t('common.add_all')))),
    section(t('setup.daytypes'),
      h('p', { class: 'muted small' }, t('setup.daytypes_hint')),
      dayTypes.length ? dayTypeBlocks : h('p', { class: 'muted' }, t('setup.none_daytypes')),
      nameForm(t('setup.new_daytype'), t('common.create'), async (name) => { await api('POST', base + '/day-types', { name }); refresh(); })),
    section(t('setup.groups'),
      h('p', { class: 'muted small' }, t('setup.groups_hint')),
      groups.length ? groupBlocks : h('p', { class: 'muted' }, t('setup.none_groups')),
      nameForm(t('setup.new_group'), t('common.create'), async (name) => { await api('POST', base + '/groups', { name }); refresh(); })),
    section(t('setup.combos'),
      h('p', { class: 'muted small' }, t('setup.combos_hint')),
      pairRows.length ? h('table', {}, h('tbody', {}, pairRows)) : h('p', { class: 'muted' }, t('setup.combos_need_two'))));
}

// What is in a cell of the run on show: people, groups and texts, what is missing, and why (behind a «Why?»). onlock(task, person)
// turns a proposal into a manual cell.
function cellContent(c, taskName, onlock) {
  const tag = (text, cls) => h('span', { class: 'tag' + (cls ? ' ' + cls : '') }, text);
  const times = (n) => (n > 1 ? ` ×${n}` : '');
  const who = [
    ...c.people.map((p) => h('span', { class: 'person' }, p.name, p.locked ? tag(t('cell.locked'), 'lock') : '', p.weak ? tag(t('cell.poor')) : '',
      p.locked ? '' : h('button', { type: 'button', class: 'lockbtn', title: t('cell.lock_title', { name: p.name }), 'aria-label': t('cell.lock_aria', { name: p.name, task: taskName }), onclick: () => onlock(c.task_id, p.id) }, t('cell.lock')))),
    ...c.groups.map((g) => h('span', { class: 'person' }, `${g.name} (${g.members.join(', ')})${times(g.fills)}`, g.locked ? tag(t('cell.locked'), 'lock') : '', g.weak ? tag(t('cell.poor')) : '')),
    ...c.texts.map((x) => h('span', { class: 'person' }, x.text + times(x.fills), tag(t('cell.locked'), 'lock'))),
  ];
  if (c.missing > 0) who.push(h('span', { class: 'err' }, c.missing === c.slots ? t('cell.not_filled') : t('cell.missing', { n: c.missing })));
  const why = c.missing > 0 && c.explanation ? h('details', { class: 'why' }, h('summary', {}, t('cell.why')), h('div', { class: 'small muted' }, c.explanation)) : '';
  return h('div', { class: 'plan-cell' }, h('div', { class: 'who' }, who), why);
}

// Several events at a time: an expandable list with ticks. selected() gives the ids.
function eventPicker(dates, label) {
  const boxes = dates.map((d) => ({ d, box: h('input', { type: 'checkbox', 'aria-label': evLabel(d) }) }));
  const summary = h('summary', {}, label);
  const update = () => { const n = boxes.filter((x) => x.box.checked).length; summary.textContent = n ? t('rules.picker_selected', { label, n }) : label; };
  boxes.forEach((x) => x.box.addEventListener('change', update));
  const set = (v) => () => { boxes.forEach((x) => { x.box.checked = v; }); update(); };
  const el = h('details', { class: 'picker' }, summary,
    h('div', { class: 'row-inline' }, h('button', { type: 'button', class: 'ghost', onclick: set(true) }, t('rules.select_all')), h('button', { type: 'button', class: 'ghost', onclick: set(false) }, t('rules.clear'))),
    h('div', { class: 'chips' }, boxes.map(({ d, box }) => h('label', { class: 'chip' }, box, ' ' + evLabel(d)))));
  return { el, selected: () => boxes.filter((x) => x.box.checked).map((x) => x.d.id) };
}

// Manual cells (locked: the planner never moves them) and selections (only these can be picked), for several events at a time.
function rulesSection(r, base, dates, tasks, groups, people, cells, pools) {
  const pbase = `${base}/plans/${enc(r.plan)}`;
  const active = people.filter((p) => p.active);
  const nameOf = (list, id) => (list.find((x) => x.id === id) || { name: '?' }).name;
  if (!dates.length) return section(t('rules.title'), h('p', { class: 'muted' }, t('rules.add_events_first')));
  const refresh = () => render();

  const pick1 = eventPicker(dates, t('rules.events'));
  const taskSel = h('select', { 'aria-label': t('rules.task_aria') }, tasks.map((tk) => h('option', { value: String(tk.id) }, tk.planned ? tk.name : t('rules.not_distributed', { name: tk.name }))));
  const kind = h('select', { 'aria-label': t('rules.kind_aria') }, [['person', t('common.person')], ['group', t('common.group')], ['text', t('common.text')]].map(([v, text]) => h('option', { value: v }, text)));
  const personSel = h('select', { 'aria-label': t('rules.person_aria') }, active.map((p) => h('option', { value: String(p.id) }, p.name)));
  const groupSel = h('select', { 'aria-label': t('rules.group_aria'), hidden: true }, groups.map((g) => h('option', { value: String(g.id) }, g.name)));
  const textIn = h('input', { type: 'text', maxlength: '80', placeholder: t('rules.text_placeholder'), 'aria-label': t('rules.text_aria'), hidden: true });
  const fills = h('input', { type: 'number', min: '1', max: '50', value: '1', 'aria-label': t('rules.fills_aria'), hidden: true });
  kind.addEventListener('change', () => { personSel.hidden = kind.value !== 'person'; groupSel.hidden = kind.value !== 'group'; textIn.hidden = kind.value !== 'text'; fills.hidden = kind.value === 'person'; });
  const addForm = h('form', { class: 'stack', onsubmit: async (e) => {
    e.preventDefault();
    const events = pick1.selected();
    if (!events.length) { say(t('err.choose_events'), 'err'); return; }
    const body = { events, task: parseInt(taskSel.value, 10) };
    if (kind.value === 'person') body.person = parseInt(personSel.value, 10);
    else if (kind.value === 'group') { body.group = parseInt(groupSel.value, 10); body.fills = parseInt(fills.value, 10) || 1; }
    else { body.label = textIn.value; body.fills = parseInt(fills.value, 10) || 1; }
    let res;
    if (await write(async () => { res = await api('POST', pbase + '/cells', body); })) { await render(); say(t('rules.cells_added', { created: res.created.length, skipped: res.skipped || 0 })); }
  } }, pick1.el, h('div', { class: 'row-inline' }, taskSel, kind, personSel, groupSel, textIn, fills), h('button', { type: 'submit' }, t('rules.add_cell')));

  const pick2 = eventPicker(dates, t('rules.events'));
  const poolTask = h('select', { 'aria-label': t('rules.pool_task_aria') }, tasks.filter((tk) => tk.planned).map((tk) => h('option', { value: String(tk.id) }, tk.name)));
  const pBoxes = active.map((p) => ({ p, box: h('input', { type: 'checkbox', 'aria-label': t('rules.pool_person_aria', { name: p.name }) }) }));
  const gBoxes = groups.map((g) => ({ g, box: h('input', { type: 'checkbox', 'aria-label': t('rules.pool_group_aria', { name: g.name }) }) }));
  const poolForm = h('form', { class: 'stack', onsubmit: async (e) => {
    e.preventDefault();
    const events = pick2.selected();
    if (!events.length) { say(t('err.choose_events'), 'err'); return; }
    const people2 = pBoxes.filter((x) => x.box.checked).map((x) => x.p.id), groups2 = gBoxes.filter((x) => x.box.checked).map((x) => x.g.id);
    if (await write(() => api('PUT', pbase + '/pools', { events, task: parseInt(poolTask.value, 10), people: people2, groups: groups2 }))) { await render(); say(tn(people2.length + groups2.length ? 'rules.pool_set' : 'rules.pool_cleared', events.length)); }
  } }, pick2.el, poolTask,
  h('div', { class: 'chips' }, [...pBoxes.map(({ p, box }) => h('label', { class: 'chip' }, box, ' ' + p.name)), ...gBoxes.map(({ g, box }) => h('label', { class: 'chip' }, box, ' ' + t('rules.group_suffix', { name: g.name })))]),
  h('button', { type: 'submit' }, t('rules.set_pool')), h('p', { class: 'muted small' }, t('rules.pool_hint')));

  const rows = dates.map((d) => ({ d, cs: cells.filter((c) => c.event_id === d.id), ps: pools.filter((p) => p.event_id === d.id) })).filter((x) => x.cs.length || x.ps.length);
  const list = rows.length ? h('ul', { class: 'plain' }, rows.map(({ d, cs, ps }) => h('li', {},
    h('strong', {}, fmtDate(d.date)), ' ', h('span', { class: 'muted' }, evMeta(d)),
    cs.map((c) => {
      const what = c.person_id ? nameOf(people, c.person_id) : c.group_id ? nameOf(groups, c.group_id) : c.label;
      return h('div', { class: 'rule' }, h('span', {}, `${nameOf(tasks, c.task_id)}: ${what}${c.fills > 1 ? ' ×' + c.fills : ''}`),
        h('button', { type: 'button', class: 'link-button', 'aria-label': t('rules.remove_cell_aria', { task: nameOf(tasks, c.task_id), what, date: d.date }), onclick: async () => { if (await write(() => api('DELETE', `${pbase}/cells/${c.id}`))) refresh(); } }, t('common.remove')));
    }),
    ps.map((p) => {
      const names = [...p.people.map((id) => nameOf(people, id)), ...p.groups.map((id) => t('rules.group_suffix', { name: nameOf(groups, id) }))];
      return h('div', { class: 'rule' }, h('span', {}, t('rules.pool_line', { task: nameOf(tasks, p.task_id), names: names.join(', ') })),
        h('button', { type: 'button', class: 'link-button', 'aria-label': t('rules.remove_pool_aria', { task: nameOf(tasks, p.task_id), date: d.date }), onclick: async () => { if (await write(() => api('PUT', pbase + '/pools', { events: [d.id], task: p.task_id, people: [], groups: [] }))) refresh(); } }, t('common.remove')));
    })))) : h('p', { class: 'muted' }, t('rules.none'));

  return section(t('rules.title'),
    h('p', { class: 'muted small' }, t('rules.hint')),
    list,
    h('details', {}, h('summary', {}, t('rules.new_cell')), addForm),
    h('details', {}, h('summary', {}, t('rules.new_pool')), poolForm));
}

// Weighting: what the planner puts weight on, for this plan. The numbers are points in the planner's sum; the defaults are a good
// starting point. Only what differs from the default is stored, so a changed default reaches plans that never chose anything.
const WEIGHT_GROUP = {
  w_fair: 'fair', w_fair_range: 'fair', w_task_fair: 'fair', w_task_range: 'fair',
  'w_task.good': 'wishes', 'w_task.poor': 'wishes', 'w_date.good': 'wishes', 'w_date.poor': 'wishes',
  'w_combo.preferred': 'combos', 'w_combo.practical': 'combos', 'w_combo.impractical': 'combos', default_combination: 'combos',
};
const weightLabel = (key) => t('weight.' + key + '.label');
const weightGroup = (key) => t('weight.group.' + (WEIGHT_GROUP[key] || 'fair'));

function weightsSection(r, wr) {
  const path = `/orgs/${enc(r.org)}/plans/${enc(r.plan)}/weights`;
  const inputs = wr.specs.map((spec) => {
    const cur = spec.key in wr.values ? wr.values[spec.key] : spec.default;
    const input = spec.kind === 'choice'
      ? h('select', { 'aria-label': weightLabel(spec.key) }, spec.options.map((o) => h('option', { value: o, selected: o === cur }, hasMsg('combo.' + o) ? t('combo.' + o) : o)))
      : h('input', { type: 'number', min: String(spec.min), max: String(spec.max), step: 'any', value: String(cur), 'aria-label': weightLabel(spec.key) });
    return { spec, input, changed: spec.key in wr.values };
  });
  const groups = [...new Set(wr.specs.map((x) => weightGroup(x.key)))];
  const blocks = groups.map((g) => h('div', { class: 'weight-group' }, h('h3', {}, g),
    inputs.filter(({ spec }) => weightGroup(spec.key) === g).map(({ spec, input, changed }) => {
      const def = spec.kind === 'choice' ? t('combo.' + spec.default) : spec.default;
      return h('div', { class: 'weight-row' + (changed ? ' changed' : '') },
        h('label', {}, h('span', { class: 'wlabel' }, weightLabel(spec.key), changed ? h('span', { class: 'tag' }, t('weights.changed')) : ''), input),
        h('div', { class: 'small muted' }, `${t('weight.' + spec.key + '.hint')} ${t('weights.default', { value: def })}`));
    })));
  const save = (values, text) => async () => {
    if (await write(() => api('PUT', path, { values }))) { await render(); say(text); }
  };
  const nChanged = wr.values ? Object.keys(wr.values).length : 0;
  return section(t('weights.title'),
    h('p', { class: 'muted small' }, t('weights.hint')),
    h('details', {}, h('summary', {}, nChanged ? t('weights.show_changed', { n: nChanged }) : t('weights.show')),
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
          h('button', { type: 'button', class: 'ghost', onclick: save({}, t('weights.reset_done')) }, t('weights.reset'))))));
}

// ---- the plan page: one table ----------------------------------------------------------------------------------------
// Events across. Above the people: what each event is and needs (date, day type, note, the plan's info rows, places per task).
// Below: everyone's answer per event, set with a click (blank → yes → maybe → no → blank; blank = not answered, which the
// planner counts as neutral). «Show plan» puts the run on show into the same table. The names of dates, info rows, tasks and
// people open a dialog with the rest. Tasks, day types, combinations and people belong to the organisation, not the plan.

const ANSWER_NEXT = { '': 'good', good: 'poor', poor: 'never', never: '' };
const ANSWER_MARK = { '': '', good: '✓', poor: '?', never: '✗' };
const PERSON_TASK_LEVELS = [['', 'person.not_taking_part'], ['never', 'level.task.never'], ['poor', 'level.poor'], ['neutral', 'level.neutral'], ['good', 'level.good']];

// What the page keeps between redraws of the same plan: the run picked in the history, and a dialog to open again after a change in it.
const planState = { key: '', runId: null, reopen: null };

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

// A small form in the first column of the table: a name and «+».
function addRow(placeholder, onsubmit) {
  const input = h('input', { type: 'text', placeholder, maxlength: '80', required: true, 'aria-label': placeholder });
  return h('form', { class: 'addrow', onsubmit: async (e) => {
    e.preventDefault();
    if (await write(() => onsubmit(input.value))) render();
  } }, input, h('button', { type: 'submit', 'aria-label': placeholder }, '+'));
}

async function pagePlan(r, org, people) {
  const base = `/orgs/${enc(r.org)}`;
  const pbase = `${base}/plans/${enc(r.plan)}`;
  const epath = pbase + '/events';
  const key = r.org + '/' + r.plan;
  if (planState.key !== key) Object.assign(planState, { key, runId: null, reopen: null });
  const [plan, events, dayTypes, tasks, combos, groups, fields, answers, places, cells, pools, weights, log, health, runs] = await Promise.all([
    api('GET', pbase), api('GET', epath), api('GET', base + '/day-types'), api('GET', base + '/tasks'), api('GET', base + '/combinations'),
    api('GET', base + '/groups'), api('GET', pbase + '/fields'), api('GET', pbase + '/answers'), api('GET', pbase + '/places'),
    api('GET', pbase + '/cells'), api('GET', pbase + '/pools'), api('GET', pbase + '/weights'), api('GET', `${base}/log?limit=30&plan=${enc(r.plan)}`),
    api('GET', '/health'), api('GET', pbase + '/runs')]);
  // A personal link (…/me/<id>) opens the table with that person's row marked; an unknown id is a page that does not exist.
  const highlight = r.person ? people.find((p) => String(p.id) === r.person) : null;
  if (r.person && !highlight) throw Object.assign(new Error(t('page.not_found')), { status: 404, keepOrg: true });

  // The run on show: the one picked in the history, else the pinned one (the plan people go by), else the newest.
  const pinned = runs.find((x) => x.pinned);
  const shownId = runs.some((x) => x.id === planState.runId) ? planState.runId : pinned ? pinned.id : runs.length ? runs[0].id : null;
  const view = shownId ? await api('GET', `${pbase}/runs/${shownId}`) : null;
  const done = !!view && view.run.status === 'done';
  const showPlan = done && store.get('showplan') !== 'off';

  const label = (ev) => evLabel(ev);
  const taskName = (id) => (tasks.find((x) => x.id === id) || { name: '?' }).name;
  const ans = {};  // person -> event -> never | poor | good ('' or missing = not answered)
  answers.events.forEach((a) => { (ans[a.person_id] ||= {})[a.event_id] = a.level; });
  const chosen = {};  // person -> the tasks they take part in, with how willingly
  answers.tasks.forEach((a) => { (chosen[a.person_id] ||= []).push(a); });
  const own = {};  // 'event/task' -> the event's own places
  places.forEach((p) => { own[p.event_id + '/' + p.task_id] = p.slots; });
  const dtSlots = {};  // day type -> task -> places
  dayTypes.forEach((d) => { dtSlots[d.id] = {}; d.tasks.forEach((x) => { dtSlots[d.id][x.task_id] = x.slots; }); });
  const fieldValue = {};  // 'event/field' -> text
  fields.values.forEach((v) => { fieldValue[v.event_id + '/' + v.field_id] = v.value; });

  // The run on show, by cell and by person (work through a group counts for each member).
  const planCell = {};  // 'event/task' -> cell
  const assigned = {};  // 'person/event' -> [{task, via}]
  if (done) {
    const viewTask = new Map(view.tasks.map((x) => [x.id, x.name]));
    for (const ev of view.events) for (const c of ev.cells) {
      planCell[ev.id + '/' + c.task_id] = c;
      const put = (pid, via) => { (assigned[pid + '/' + ev.id] ||= []).push({ task: viewTask.get(c.task_id) || taskName(c.task_id), via }); };
      c.people.forEach((p) => put(p.id, ''));
      c.groups.forEach((g) => (g.member_ids || []).forEach((m) => put(m, g.name)));
    }
  }
  const lock = async (evId, taskId, personId) => { if (await write(() => api('POST', pbase + '/cells', { events: [evId], task: taskId, person: personId }))) render(); };

  // ---- the plan bar: plan or try again, and what the run on show is
  const runsPath = pbase + '/runs';
  const newest = runs[0];
  const running = !!newest && newest.status === 'running';
  const bar = h('div', { class: 'plan-bar' });
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
    const pin = async (id, on) => {
      if (!(await write(() => api(on ? 'PUT' : 'DELETE', `${runsPath}/${id}/pin`)))) return;
      planState.runId = id;
      await render();
      say(on ? t('run.pin_done', { id }) : t('run.unpin_done'));
    };
    const out = [h('div', { class: 'row-inline' },
      h('button', { type: 'button', disabled: !health.planner || running, onclick: start }, running ? t('run.planning') : view ? t('run.retry') : t('run.plan')),
      h('label', { class: 'small muted' }, t('run.time') + ' ', limitSel),
      done ? h('button', { type: 'button', class: 'ghost toggle', 'aria-pressed': String(showPlan), onclick: () => { store.set('showplan', showPlan ? 'off' : 'on'); render(); } }, t('table.show_plan')) : '')];
    if (!health.planner) out.push(h('p', { class: 'muted' }, t('run.no_planner')));
    if (running) { out.push(h('p', { role: 'status' }, t('run.running', { n: newest.time_limit }))); setTimeout(poll, 1000); }
    if (!view) { if (!running) out.push(h('p', { class: 'muted' }, t('run.none_yet'))); }
    else if (view.run.status === 'failed') out.push(h('p', { class: 'err', role: 'alert' }, t('run.failed', { error: view.run.error || t('run.unknown_error') })));
    else if (done) {
      const run = view.run;
      out.push(h('p', { class: 'small muted' }, t('run.made_by', { actor: run.actor, time: fmtTime(run.finished_at || run.started_at) }) + ' ' + (run.solver_status === 'OPTIMAL' ? t('run.optimal') : t('run.feasible'))));
      if (run.id !== newest.id) out.push(h('p', { class: 'small' }, t('run.older_note', { id: run.id }), ' ', h('button', { type: 'button', class: 'link-button', onclick: () => { planState.runId = newest.id; render(); } }, t('run.show_newest'))));
      out.push(run.pinned
        ? h('p', { class: 'small' }, h('span', { class: 'tag lock' }, t('run.pinned_tag')), ' ' + t('run.pinned_note') + ' ', h('button', { type: 'button', class: 'link-button', onclick: () => pin(run.id, false) }, t('run.unpin')))
        : h('p', { class: 'small muted' }, h('button', { type: 'button', class: 'ghost', onclick: () => pin(run.id, true) }, t('run.pin')), ' ' + t('run.pin_hint')));
      if (run.stale) out.push(h('p', { class: 'warn', role: 'status' }, t('run.stale')));
      if (run.checked && run.violations.length) out.push(h('div', { class: 'err', role: 'alert' }, h('p', {}, t('run.violations')), h('ul', {}, run.violations.map((v) => h('li', {}, v)))));
      else if (run.checked) out.push(h('p', { class: 'small muted' }, t('run.checked_ok')));
      if (view.warnings.length) out.push(h('ul', { class: 'warn small' }, view.warnings.map((w) => h('li', {}, w))));
    }
    bar.replaceChildren(...out);
  }

  // ---- dialogs
  const again = (kind, id) => { planState.reopen = { kind, id }; };

  function eventDialog(ev) {
    const isNew = !ev;
    const date = h('input', { type: 'date', required: true, 'aria-label': t('plan.date_aria') });
    const typeSel = h('select', { 'aria-label': t('plan.daytype_aria') }, dayTypes.map((d) => h('option', { value: String(d.id), selected: !!ev && d.id === ev.day_type_id }, d.name)));
    const start = h('input', { type: 'time', value: ev ? ev.start_time : '', 'aria-label': t('plan.start_time') });
    const end = h('input', { type: 'time', value: ev ? ev.end_time : '', 'aria-label': t('plan.end_time') });
    const note = h('input', { type: 'text', maxlength: '200', value: ev ? ev.note : '', placeholder: t('plan.note_optional'), 'aria-label': t('plan.note') });
    const desc = h('textarea', { rows: '4', maxlength: '2000', placeholder: t('plan.description_placeholder'), 'aria-label': t('plan.description') }, ev ? ev.description : '');
    const field = (text, input) => h('label', { class: 'field' }, h('span', {}, text), input);
    const dlg = modal(isNew ? t('event.new') : fmtDate(ev.date) + (clock(ev) ? ' ' + clock(ev) : ''),
      h('form', { class: 'stack', onsubmit: async (e) => {
        e.preventDefault();
        let body;
        if (isNew) body = { date: date.value, day_type_id: parseInt(typeSel.value, 10), note: note.value, start_time: start.value, end_time: end.value, description: desc.value };
        else {
          body = {};
          const endWanted = start.value ? end.value : '';  // an end time needs a start time
          if (parseInt(typeSel.value, 10) !== ev.day_type_id) body.day_type_id = parseInt(typeSel.value, 10);
          if (note.value !== ev.note) body.note = note.value;
          if (start.value !== ev.start_time) body.start_time = start.value;
          if (endWanted !== ev.end_time) body.end_time = endWanted;
          if (desc.value !== ev.description) body.description = desc.value;
          if (!Object.keys(body).length) { dlg.close(); return; }
        }
        if (await write(() => (isNew ? api('POST', epath, body) : api('PATCH', `${epath}/${ev.id}`, body)))) { dlg.close(); render(); }
      } },
        isNew ? field(t('plan.date_aria'), date) : '',
        field(t('plan.daytype_aria'), typeSel),
        h('div', { class: 'row-inline' }, field(t('plan.start_time'), start), field(t('plan.end_time'), end)),
        field(t('plan.note'), note),
        field(t('plan.description'), desc),
        h('div', { class: 'row-inline' }, h('button', { type: 'submit' }, isNew ? t('plan.add_event') : t('common.save')),
          isNew ? '' : h('button', { type: 'button', class: 'danger', onclick: async () => {
            if (await write(async () => { confirmed(t('plan.delete_event_confirm', { date: fmtDate(ev.date), daytype: ev.day_type })); await api('DELETE', `${epath}/${ev.id}`); })) { dlg.close(); render(); }
          } }, t('event.delete')))));
    if (isNew) date.focus();
  }

  function fieldDialog(f) {
    const name = h('input', { type: 'text', value: f.name, maxlength: '80', 'aria-label': t('field.name_label') });
    const dlg = modal(f.name,
      h('form', { class: 'row', onsubmit: async (e) => {
        e.preventDefault();
        if (name.value.trim() === f.name) { dlg.close(); return; }
        if (await write(() => api('PATCH', `${pbase}/fields/${f.id}`, { name: name.value }))) { dlg.close(); render(); }
      } }, name, h('button', { type: 'submit' }, t('common.rename'))),
      h('p', { class: 'muted small' }, t('field.hint')),
      h('button', { type: 'button', class: 'danger', onclick: async () => {
        if (await write(async () => { confirmed(t('field.delete_confirm', { name: f.name })); await api('DELETE', `${pbase}/fields/${f.id}`); })) { dlg.close(); render(); }
      } }, t('field.delete')));
  }

  function taskDialog(tk) {
    let dlg;
    const changed = () => { again('task', tk.id); dlg.close(); render(); };
    const name = h('input', { type: 'text', value: tk.name, maxlength: '80', 'aria-label': t('common.name_of_aria', { name: tk.name }) });
    const planned = h('input', { type: 'checkbox', checked: tk.planned, 'aria-label': t('setup.planned_aria', { name: tk.name }) });
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
    const slots = dayTypes.map((d) => ({ d, cur: (dtSlots[d.id] || {})[tk.id] || 0 }));
    slots.forEach((x) => { x.input = h('input', { type: 'number', min: '0', max: '50', value: String(x.cur), 'aria-label': t('setup.places_aria', { task: tk.name, daytype: x.d.name }) }); });
    // A day type's list of tasks is written as a whole: keep the other tasks as they are.
    const savePlaces = async () => {
      const todo = slots.filter((x) => (parseInt(x.input.value, 10) || 0) !== x.cur);
      if (!todo.length) return;
      if (await write(async () => {
        for (const { d, input } of todo) {
          const n = parseInt(input.value, 10) || 0;
          const list = d.tasks.filter((x) => x.task_id !== tk.id).map((x) => ({ task_id: x.task_id, slots: x.slots }));
          if (n > 0) list.push({ task_id: tk.id, slots: n });
          await api('PUT', `${base}/day-types/${d.id}/tasks`, { tasks: list });
        }
      })) changed();
    };
    const pairRows = others.map((o) => {
      const cur = levelWith(o);
      return h('tr', {}, h('td', {}, o.name), h('td', {}, h('select', { 'aria-label': `${tk.name} + ${o.name}`, onchange: async (e) => {
        if (await write(() => api('PUT', base + '/combinations', { a: tk.id, b: o.id, level: e.target.value }))) changed(); else e.target.value = cur;
      } }, SELECT_LEVELS.map((v) => h('option', { value: v, selected: v === cur }, t('combo.' + v))))));
    });
    dlg = modal(tk.name,
      h('p', { class: 'muted small' }, t('task.org_wide')),
      h('form', { class: 'row', onsubmit: async (e) => {
        e.preventDefault();
        if (name.value.trim() !== tk.name && await write(() => api('PATCH', `${base}/tasks/${tk.id}`, { name: name.value }))) changed();
      } }, name, h('button', { type: 'submit' }, t('common.rename'))),
      h('label', { class: 'check' }, planned, ' ' + t('setup.col_planned')),
      h('h3', {}, t('task.places')),
      dayTypes.length ? [h('table', {}, h('tbody', {}, slots.map(({ d, input }) => h('tr', {}, h('td', {}, d.name), h('td', {}, input))))),
        h('button', { type: 'button', onclick: savePlaces }, t('task.save_places'))] : h('p', { class: 'muted' }, t('task.no_daytypes')),
      h('h3', {}, t('task.combine')),
      h('label', { class: 'check' }, aloneBox, ' ' + t('task.alone')),
      h('p', { class: 'muted small' }, t('task.alone_hint')),
      pairRows.length ? h('table', {}, h('tbody', {}, pairRows)) : '',
      h('div', { class: 'row-inline' }, h('button', { type: 'button', class: 'danger', onclick: async () => {
        if (await write(async () => { confirmed(t('setup.delete_task_confirm', { name: tk.name })); await api('DELETE', `${base}/tasks/${tk.id}`); })) { dlg.close(); render(); }
      } }, t('task.delete'))));
  }

  function personDialog(p) {
    let dlg;
    const changed = () => { again('person', p.id); dlg.close(); render(); };
    const ppath = `${base}/people/${p.id}`;
    const name = h('input', { type: 'text', value: p.name, maxlength: '80', 'aria-label': t('common.name_of_aria', { name: p.name }) });
    const flag = (k, text, aria) => {
      const box = h('input', { type: 'checkbox', checked: p[k], 'aria-label': aria });
      box.addEventListener('change', async () => { if (await write(() => api('PATCH', ppath, { [k]: box.checked }))) changed(); else box.checked = p[k]; });
      return h('label', { class: 'check' }, box, ' ' + text);
    };
    const levelOf = (tk) => ((chosen[p.id] || []).find((x) => x.task_id === tk.id) || { level: '' }).level;
    const taskRows = tasks.filter((tk) => tk.planned).map((tk) => {
      const cur = levelOf(tk);
      return h('tr', {}, h('td', {}, tk.name), h('td', {}, h('select', { 'aria-label': t('person.task_aria', { name: p.name, task: tk.name }), onchange: async (e) => {
        const v = e.target.value;
        if (await write(() => (v ? api('PUT', `${ppath}/tasks/${tk.id}`, { level: v }) : api('DELETE', `${ppath}/tasks/${tk.id}`)))) changed(); else e.target.value = cur;
      } }, PERSON_TASK_LEVELS.map(([v, k]) => h('option', { value: v, selected: v === cur }, t(k))))));
    });
    const url = location.origin + `/o/${enc(r.org)}/p/${enc(r.plan)}/me/${p.id}`;
    dlg = modal(p.name,
      p.active ? '' : h('p', { class: 'err' }, t('person.inactive')),
      h('h3', {}, t('person.tasks')),
      h('p', { class: 'muted small' }, t('person.tasks_hint')),
      taskRows.length ? h('table', {}, h('tbody', {}, taskRows)) : h('p', { class: 'muted' }, t('person.no_tasks')),
      h('h3', {}, t('person.link')),
      h('input', { type: 'text', readonly: true, value: url, class: 'wide', 'aria-label': t('person.link'), onfocus: (e) => e.target.select() }),
      h('p', { class: 'muted small' }, t('person.link_hint')),
      h('h3', {}, t('person.about')),
      h('p', { class: 'muted small' }, t('person.org_wide')),
      h('form', { class: 'row', onsubmit: async (e) => {
        e.preventDefault();
        if (name.value.trim() !== p.name && await write(() => api('PATCH', ppath, { name: name.value }))) changed();
      } }, name, h('button', { type: 'submit' }, t('common.rename'))),
      flag('active', t('person.active'), t('org.active_aria', { name: p.name })),
      flag('proposable', t('common.can_be_proposed'), t('common.can_be_proposed_aria', { name: p.name })),
      h('p', { class: 'muted small' }, t('org.proposable_hint')));
  }

  // ---- the table
  const cols = events.length;
  const filler = () => h('td', { class: 'addcol' });
  const sect = (text, hint, cells2) => h('tr', { class: 'sect' }, h('th', { scope: 'row' }, text), cells2 || h('td', { colspan: String(cols + 1) }, hint ? h('span', { class: 'small muted' }, hint) : ''));
  const head = h('tr', {}, h('th', { scope: 'row', class: 'corner' }, t('table.date')),
    events.map((ev) => h('th', { scope: 'col' }, h('button', { type: 'button', class: 'cellbtn', 'aria-label': t('table.event_aria', { label: label(ev) }), onclick: () => eventDialog(ev) },
      fmtShortDate(ev.date), clock(ev) ? h('span', { class: 'sub' }, clock(ev)) : '', ev.description ? h('span', { class: 'sub' }, t('table.has_description')) : ''))),
    h('th', { scope: 'col', class: 'addcol' }, h('button', { type: 'button', class: 'ghost', 'aria-label': t('table.add_event'), title: t('table.add_event'), disabled: !dayTypes.length, onclick: () => eventDialog(null) }, '+')));

  const patch = (ev, body, revert) => async (e) => { if (await write(() => api('PATCH', `${epath}/${ev.id}`, body(e.target.value)))) render(); else revert(e.target); };
  const typeRow = h('tr', {}, h('th', { scope: 'row' }, t('table.type')), events.map((ev) => h('td', {},
    h('select', { 'aria-label': t('plan.daytype_of_aria', { label: label(ev) }), onchange: patch(ev, (v) => ({ day_type_id: parseInt(v, 10) }), (el) => { el.value = String(ev.day_type_id); }) },
      dayTypes.map((d) => h('option', { value: String(d.id), selected: d.id === ev.day_type_id }, d.name))))), filler());
  // A text in a cell is saved when the field is left, without redrawing the page (so the next field keeps the focus).
  const textCell = (cur, aria, save) => {
    let value = cur;
    const input = h('input', { type: 'text', value, maxlength: '200', 'aria-label': aria });
    input.addEventListener('change', async () => {
      if (input.value.trim() === value) { input.value = value; return; }
      if (await write(() => save(input.value))) value = input.value.trim();
      input.value = value;
    });
    return h('td', {}, input);
  };
  const noteRow = h('tr', {}, h('th', { scope: 'row' }, t('plan.note')),
    events.map((ev) => textCell(ev.note, t('plan.note_aria', { label: label(ev) }), (v) => api('PATCH', `${epath}/${ev.id}`, { note: v }))), filler());
  const fieldRows = fields.fields.map((f) => h('tr', {},
    h('th', { scope: 'row' }, h('button', { type: 'button', class: 'cellbtn', 'aria-label': t('table.field_aria', { name: f.name }), onclick: () => fieldDialog(f) }, f.name)),
    events.map((ev) => textCell(fieldValue[ev.id + '/' + f.id] || '', t('table.value_aria', { name: f.name, label: label(ev) }), (v) => api('PUT', `${epath}/${ev.id}/fields/${f.id}`, { value: v }))), filler()));
  const addField = h('tr', { class: 'adds' }, h('th', { scope: 'row' }, addRow(t('table.new_field'), (name) => api('POST', pbase + '/fields', { name }))), h('td', { colspan: String(cols + 1) }));

  // Places: empty shows the day type's number (faded); a number of its own is bold. Emptying it, or typing the day type's
  // number, goes back to the day type. Only a real change is sent.
  const placesCell = (tk, ev) => {
    const def = (dtSlots[ev.day_type_id] || {})[tk.id] || 0;
    let cur = own[ev.id + '/' + tk.id];  // undefined: as the day type
    const input = h('input', { type: 'number', min: '0', max: '50', inputmode: 'numeric', placeholder: String(def), title: t('table.places_title', { n: def }), 'aria-label': t('table.places_aria', { task: tk.name, label: label(ev) }) });
    const paint = () => { input.value = cur === undefined ? '' : String(cur); input.classList.toggle('own', cur !== undefined); };
    input.addEventListener('change', async () => {
      const raw = input.value.trim();
      const n = raw === '' ? def : parseInt(raw, 10);
      const reset = n === def;
      if (reset ? cur === undefined : n === cur) { paint(); return; }
      const path = `${epath}/${ev.id}/places/${tk.id}`;
      if (await write(() => (reset ? api('DELETE', path) : api('PUT', path, { slots: n })))) cur = reset ? undefined : n;
      paint();
    });
    paint();
    const c = showPlan ? planCell[ev.id + '/' + tk.id] : null;
    return h('td', { class: 'places' }, input, c ? cellContent(c, tk.name, (task, person) => lock(ev.id, task, person)) : '');
  };
  const taskRows = tasks.map((tk) => h('tr', {},
    h('th', { scope: 'row' }, h('button', { type: 'button', class: 'cellbtn', 'aria-label': t('table.task_aria', { name: tk.name }), onclick: () => taskDialog(tk) },
      tk.name, tk.planned ? '' : h('span', { class: 'sub' }, t('table.by_hand')))),
    events.map((ev) => placesCell(tk, ev)), filler()));
  const addTask = h('tr', { class: 'adds' }, h('th', { scope: 'row' }, addRow(t('setup.new_task'), (name) => api('POST', base + '/tasks', { name }))), h('td', { colspan: String(cols + 1) }));

  // People: one click on a cell moves the answer on; the count above each event follows.
  const countCells = {};
  const recount = (evId) => {
    const c = { good: 0, poor: 0, never: 0 };
    people.forEach((p) => { const l = p.active && (ans[p.id] || {})[evId]; if (l) c[l]++; });
    countCells[evId].textContent = t('table.count', c);
  };
  const countRow = sect(t('table.people'), '', [...events.map((ev) => (countCells[ev.id] = h('td', { class: 'count small muted', 'aria-live': 'polite' }))), filler()]);
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
    const mine = showPlan ? assigned[p.id + '/' + ev.id] || [] : [];
    return h('td', { class: 'answer' }, b, mine.length ? h('div', { class: 'assigned' }, mine.map((x) => h('span', { class: 'tag task' }, x.via ? t('table.via', { task: x.task, group: x.via }) : x.task))) : '');
  };
  const personRow = (p) => {
    const mine = (chosen[p.id] || []).filter((x) => tasks.some((tk) => tk.id === x.task_id && tk.planned)).map((x) => taskName(x.task_id));
    return h('tr', { class: [highlight && p.id === highlight.id ? 'me' : '', p.active ? '' : 'inactive'].filter(Boolean).join(' ') || false, 'data-person': String(p.id) },
      h('th', { scope: 'row' }, h('button', { type: 'button', class: 'cellbtn', 'aria-label': t('table.person_aria', { name: p.name }), onclick: () => personDialog(p) },
        p.name, h('span', { class: 'sub' + (mine.length ? '' : ' prompt') }, mine.length ? mine.join(', ') : t('table.choose_tasks')),
        p.active ? '' : h('span', { class: 'sub' }, t('setup.inactive')))),
      events.map((ev) => answerCell(p, ev)), filler());
  };
  const addPerson = h('tr', { class: 'adds' }, h('th', { scope: 'row' }, addRow(t('org.new_person'), (name) => api('POST', base + '/people', { name }))), h('td', { colspan: String(cols + 1) }));

  const table = h('div', { class: 'grid-scroll' }, h('table', { class: 'dp-table' },
    h('thead', {}, head),
    h('tbody', {}, typeRow, noteRow, fieldRows, addField,
      sect(t('table.tasks'), t('table.tasks_hint')), taskRows, addTask,
      countRow, people.filter((p) => p.active).map(personRow), people.filter((p) => !p.active).map(personRow), addPerson)));
  events.forEach((ev) => recount(ev.id));

  // ---- below the table, folded away
  const fold = (title, ...kids) => h('details', { class: 'fold' }, h('summary', {}, title), ...kids);
  const history = runs.length > 1 ? fold(t('run.history', { n: runs.length }), h('ul', { class: 'plain runs' }, runs.map((x) => h('li', { class: 'run-row' + (view && x.id === view.run.id ? ' shown' : '') },
    h('span', {}, '#' + x.id), h('span', { class: 'muted small' }, `${x.actor} · ${fmtTime(x.finished_at || x.started_at)}`),
    x.status !== 'done' ? h('span', { class: 'tag' }, t('run.status.' + x.status)) : '', x.pinned ? h('span', { class: 'tag lock' }, t('run.pinned_tag')) : '',
    view && x.id === view.run.id ? '' : h('button', { type: 'button', class: 'link-button', 'aria-label': t('run.show_aria', { id: x.id }), onclick: () => { planState.runId = x.id; render(); } }, t('run.show')))))) : '';

  let bulkForm = '';
  if (dayTypes.length) {
    const from = h('input', { type: 'date', required: true, 'aria-label': t('plan.from') });
    const to = h('input', { type: 'date', required: true, 'aria-label': t('plan.to') });
    const days = [1, 2, 3, 4, 5, 6, 7].map((v) => ({ n: t('week.' + v), v, box: h('input', { type: 'checkbox', checked: v === 7, 'aria-label': t('week.' + v) }) }));
    const bType = h('select', { 'aria-label': t('plan.bulk_type_aria') }, dayTypes.map((d) => h('option', { value: String(d.id) }, d.name)));
    const bStart = h('input', { type: 'time', 'aria-label': t('plan.bulk_start_aria') });
    const bEnd = h('input', { type: 'time', 'aria-label': t('plan.bulk_end_aria') });
    bulkForm = h('form', { class: 'stack', onsubmit: async (e) => {
      e.preventDefault();
      let res;
      if (await write(async () => { res = await api('POST', epath + '/bulk', { from: from.value, to: to.value, weekdays: days.filter((x) => x.box.checked).map((x) => x.v), day_type_id: parseInt(bType.value, 10), start_time: bStart.value, end_time: bEnd.value }); })) {
        await render(); say(t('plan.bulk_done', { created: res.created.length, skipped: res.skipped.length }));
      }
    } },
      h('div', { class: 'row-inline' }, h('label', {}, t('plan.from') + ' ', from), h('label', {}, t('plan.to') + ' ', to)),
      h('div', { class: 'row-inline' }, days.map((x) => h('label', { class: 'chip' }, x.box, ' ' + x.n))),
      h('div', { class: 'row-inline' }, bType, bStart, bEnd, h('button', { type: 'submit' }, t('common.add_all'))));
  }

  const root = h('div', { class: 'plan-page' },
    h('p', { class: 'small' }, link(`/o/${enc(r.org)}`, t('common.back', { name: org.name }))),
    h('h1', {}, plan.name),
    bar,
    dayTypes.length ? '' : h('p', { class: 'muted' }, t('plan.needs_daytype_a'), link(`/o/${enc(r.org)}/setup`, t('plan.needs_daytype_link')), t('plan.needs_daytype_b')),
    h('p', { class: 'muted small' }, events.length ? t('table.hint') : t('table.no_events')),
    table,
    h('div', { class: 'below' },
      done && view.weak.length ? fold(t('run.compromises', { n: view.weak.length }), h('ul', { class: 'small' }, view.weak.map((w) => h('li', {}, w)))) : '',
      done && view.load.length ? fold(t('run.distribution'), h('ul', { class: 'small' }, view.load.map((l) => h('li', {}, t('run.load_line', { name: l.name, count: l.count, target: l.target.toLocaleString(locale()) }))))) : '',
      history,
      dayTypes.length ? fold(t('plan.bulk_title'), bulkForm) : '',
      fold(t('log.title') + ': ' + plan.name, logTable(log))),
    rulesSection(r, base, events, tasks, groups, people, cells, pools),
    weightsSection(r, weights),
    managePlan(r, plan));
  if (highlight) root.dataset.person = highlight.name;

  // After a change in a dialog, the page is drawn again with the new data: open the same dialog again on top, once the new
  // page is in place (render calls this).
  root.shown = () => {
    const ro = planState.reopen;
    planState.reopen = null;
    if (ro && ro.kind === 'task') { const x = tasks.find((tk) => tk.id === ro.id); if (x) taskDialog(x); }
    if (ro && ro.kind === 'person') { const x = people.find((p) => p.id === ro.id); if (x) personDialog(x); }
  };
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
    const [, body] = await Promise.all([renderHeader(r, people), r.page === 'home' ? pageHome() : r.page === 'org' ? pageOrg(r, org, people) : r.page === 'setup' ? pageSetup(r, org) : pagePlan(r, org, people)]);
    if (seq !== renderSeq || !body) return;
    const y = location.pathname === lastPath ? window.scrollY : 0;  // the same page again (after a change): stay where we are
    main.replaceChildren(body);
    lastPath = location.pathname;
    window.scrollTo(0, y);
    if (body.shown) body.shown();
    if (r.page === 'me' && body.dataset.person) {
      if (!y) { const row = body.querySelector('tr.me'); if (row) row.scrollIntoView({ block: 'center' }); }
      if (!store.get('actor').trim()) {  // whoever opens their own link is most likely that person
        store.set('actor', body.dataset.person);
        const a = document.getElementById('actor');
        if (a) a.value = body.dataset.person;
      }
    }
    document.title = t('brand') + (org ? ' · ' + org.name : '');
  } catch (e) {
    if (seq !== renderSeq) return;
    if (e.status === 404 && r.org && !e.keepOrg) store.set('org', '');
    main.replaceChildren(h('div', {}, h('h1', {}, e.status === 404 ? t('page.not_found') : t('page.error')), h('p', {}, link('/?switch', t('page.choose_org'))), h('p', { class: 'muted small' }, errorText(e))));
  }
}

window.addEventListener('popstate', render);
render();
