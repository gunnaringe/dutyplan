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
  const m = document.getElementById('msg');
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
const TASK_LEVELS = [['never', 'level.task.never'], ['poor', 'level.poor'], ['neutral', 'level.neutral'], ['good', 'level.good']];
const EVENT_LEVELS = [['never', 'level.event.never'], ['poor', 'level.poor'], ['good', 'level.good']];

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

function dayTypeSelect(dayTypes, value, onchange, label) {
  return h('select', { 'aria-label': label, onchange }, dayTypes.map((d) => h('option', { value: String(d.id), selected: d.id === value }, d.name)));
}

// «Plan»: a run of the planner (the latest at first) as a list of events (mobile first), with a button to plan or try again, the
// earlier runs, and a pin that makes a finished run the plan people see under «My assignments».
// The run happens on the server; the panel asks for status every second until it is done and redraws only itself.
function planPanel(r, canPlan, first, firstRuns) {
  const path = `/orgs/${enc(r.org)}/plans/${enc(r.plan)}/runs`;
  const box = h('div', { class: 'plan-panel' });
  const limits = [[10, '10 s'], [20, '20 s'], [60, '60 s']];
  const limitSel = h('select', { 'aria-label': t('run.limit_aria') }, limits.map(([v, text]) => h('option', { value: String(v), selected: String(v) === (store.get('limit') || '20') }, text)));
  limitSel.addEventListener('change', () => store.set('limit', limitSel.value));
  let timer = 0;
  let lastView = first;
  let runs = firstRuns;  // newest first, with the pinned run too
  let historyOpen = false;  // the list of earlier runs stays open when the panel is redrawn
  // Grid on wide screens, list on narrow ones, until the user chooses.
  const planView = () => store.get('planview') || (window.matchMedia('(min-width: 56rem)').matches ? 'grid' : 'list');

  async function start() {
    let view;
    if (await write(async () => { const res = await api('POST', path, { time_limit: parseInt(limitSel.value, 10) }); runs = await api('GET', path); view = await api('GET', `${path}/${res.id}`); })) show(view);
  }
  async function poll() {
    if (!box.isConnected) return;  // the page has been replaced
    try {
      const view = await api('GET', `${path}/latest`);
      if (view.run.status !== 'running') runs = await api('GET', path);
      show(view);
    } catch (e) { say(errorText(e), 'err'); }
  }
  async function showRun(id) {
    try { show(await api('GET', `${path}/${id}`)); } catch (e) { say(errorText(e), 'err'); }
  }
  // Pin or unpin the run on show, then show it again (not the latest: the user is looking at this one).
  async function pin(id, on) {
    if (!(await write(() => api(on ? 'PUT' : 'DELETE', `${path}/${id}/pin`)))) return;
    try { runs = await api('GET', path); show(await api('GET', `${path}/${id}`)); say(on ? t('run.pin_done', { id }) : t('run.unpin_done')); } catch (e) { say(errorText(e), 'err'); }
  }
  function show(view) {
    lastView = view;
    clearTimeout(timer);
    box.replaceChildren(...body(view));
    if (view && view.run.status === 'running') timer = setTimeout(poll, 1000);
  }

  async function lock(evId, taskId, personId) {
    if (await write(() => api('POST', `/orgs/${enc(r.org)}/plans/${enc(r.plan)}/cells`, { events: [evId], task: taskId, person: personId }))) render();
  }

  // What is in a cell: people, groups and texts, and what is missing. compact (the grid): the explanation behind a «Why?».
  function cellContent(c, taskName, evId, compact) {
    const tag = (text, cls) => h('span', { class: 'tag' + (cls ? ' ' + cls : '') }, text);
    const times = (n) => (n > 1 ? ` ×${n}` : '');
    const who = [
      ...c.people.map((p) => h('span', { class: 'person' }, p.name, p.locked ? tag(t('cell.locked'), 'lock') : '', p.weak ? tag(t('cell.poor')) : '',
        p.locked ? '' : h('button', { type: 'button', class: 'lockbtn', title: t('cell.lock_title', { name: p.name }), 'aria-label': t('cell.lock_aria', { name: p.name, task: taskName }), onclick: () => lock(evId, c.task_id, p.id) }, t('cell.lock')))),
      ...c.groups.map((g) => h('span', { class: 'person' }, `${g.name} (${g.members.join(', ')})${times(g.fills)}`, g.locked ? tag(t('cell.locked'), 'lock') : '', g.weak ? tag(t('cell.poor')) : '')),
      ...c.texts.map((x) => h('span', { class: 'person' }, x.text + times(x.fills), tag(t('cell.locked'), 'lock'))),
    ];
    if (c.missing > 0) who.push(h('span', { class: 'err' }, c.missing === c.slots ? t('cell.not_filled') : t('cell.missing', { n: c.missing })));
    const why = c.missing > 0 && c.explanation
      ? (compact ? h('details', { class: 'why' }, h('summary', {}, t('cell.why')), h('div', { class: 'small muted' }, c.explanation)) : h('div', { class: 'small muted' }, c.explanation))
      : '';
    return h('div', {}, h('div', { class: 'who' }, who), why);
  }

  // The grid: events across, tasks down. It scrolls sideways in its own box, not the page.
  function gridTable(view) {
    const head = h('tr', {}, h('th', { class: 'corner' }, ''),
      view.events.map((ev) => h('th', { scope: 'col' }, fmtShortDate(ev.date), h('div', { class: 'small muted' }, ev.day_type + (ev.note ? ' · ' + ev.note : '')))));
    const rows = view.tasks.map((tk) => h('tr', {}, h('th', { scope: 'row' }, tk.name),
      view.events.map((ev) => { const c = ev.cells.find((x) => x.task_id === tk.id); return h('td', {}, c ? cellContent(c, tk.name, ev.id, true) : h('span', { class: 'muted' }, '–')); })));
    return h('div', { class: 'grid-scroll' }, h('table', { class: 'plan-grid' }, h('thead', {}, head), h('tbody', {}, rows)));
  }

  function body(view) {
    const running = !!view && view.run.status === 'running';
    const out = [h('div', { class: 'row-inline' },
      h('button', { type: 'button', disabled: !canPlan || running, onclick: start }, running ? t('run.planning') : view ? t('run.retry') : t('run.plan')),
      h('label', { class: 'small muted' }, t('run.time') + ' ', limitSel))];
    if (!canPlan) out.push(h('p', { class: 'muted' }, t('run.no_planner')));
    if (!view) { out.push(h('p', { class: 'muted' }, t('run.none_yet'))); return out; }
    const run = view.run;
    if (running) { out.push(h('p', { role: 'status' }, t('run.running', { n: run.time_limit }))); return out; }
    if (run.status === 'failed') { out.push(h('p', { class: 'err', role: 'alert' }, t('run.failed', { error: run.error || t('run.unknown_error') }))); return out; }

    out.push(h('p', { class: 'small muted' }, t('run.made_by', { actor: run.actor, time: fmtTime(run.finished_at || run.started_at) }) + ' ' + (run.solver_status === 'OPTIMAL' ? t('run.optimal') : t('run.feasible'))));
    if (runs.length && run.id !== runs[0].id) out.push(h('p', { class: 'small' }, t('run.older_note', { id: run.id }), ' ', h('button', { type: 'button', class: 'link-button', onclick: () => showRun(runs[0].id) }, t('run.show_newest'))));
    out.push(run.pinned
      ? h('p', { class: 'small' }, h('span', { class: 'tag lock' }, t('run.pinned_tag')), ' ' + t('run.pinned_note') + ' ', h('button', { type: 'button', class: 'link-button', onclick: () => pin(run.id, false) }, t('run.unpin')))
      : h('p', { class: 'small muted' }, h('button', { type: 'button', class: 'ghost', onclick: () => pin(run.id, true) }, t('run.pin')), ' ' + t('run.pin_hint')));
    if (run.stale) out.push(h('p', { class: 'warn', role: 'status' }, t('run.stale')));
    if (run.checked && run.violations.length) out.push(h('div', { class: 'err', role: 'alert' }, h('p', {}, t('run.violations')), h('ul', {}, run.violations.map((v) => h('li', {}, v)))));
    else if (run.checked) out.push(h('p', { class: 'small muted' }, t('run.checked_ok')));
    if (view.warnings.length) out.push(h('ul', { class: 'warn small' }, view.warnings.map((w) => h('li', {}, w))));

    const names = new Map(view.tasks.map((tk) => [tk.id, tk.name]));
    const mode = planView();
    out.push(h('div', { class: 'view-toggle', role: 'group', 'aria-label': t('run.view_aria') },
      [['list', t('run.view_list')], ['grid', t('run.view_grid')]].map(([v, text]) => h('button', { type: 'button', class: 'ghost', 'aria-pressed': String(mode === v), onclick: () => { store.set('planview', v); show(lastView); } }, text))));
    if (mode === 'grid') out.push(gridTable(view));
    else out.push(h('ul', { class: 'plan-events' }, view.events.map((ev) => h('li', { class: 'plan-event' },
      h('div', {}, h('strong', {}, fmtDate(ev.date)), ' ', h('span', { class: 'muted' }, ev.day_type + (ev.note ? ' · ' + ev.note : ''))),
      h('div', { class: 'plan-cells' }, ev.cells.flatMap((c) => [h('div', { class: 'task' }, names.get(c.task_id) || '?'), cellContent(c, names.get(c.task_id) || '?', ev.id, false)]))))));
    if (view.weak.length) out.push(h('details', {}, h('summary', {}, t('run.compromises', { n: view.weak.length })), h('ul', { class: 'small' }, view.weak.map((w) => h('li', {}, w)))));
    if (runs.length > 1) out.push(h('details', { open: historyOpen, ontoggle: (e) => { historyOpen = e.target.open; } }, h('summary', {}, t('run.history', { n: runs.length })), h('ul', { class: 'plain runs' }, runs.map((x) => h('li', { class: 'run-row' + (x.id === run.id ? ' shown' : '') },
      h('span', {}, '#' + x.id), h('span', { class: 'muted small' }, `${x.actor} · ${fmtTime(x.finished_at || x.started_at)}`),
      x.status !== 'done' ? h('span', { class: 'tag' }, t('run.status.' + x.status)) : '', x.pinned ? h('span', { class: 'tag lock' }, t('run.pinned_tag')) : '',
      x.id === run.id ? '' : h('button', { type: 'button', class: 'link-button', 'aria-label': t('run.show_aria', { id: x.id }), onclick: () => showRun(x.id) }, t('run.show')))))));
    if (view.load.length) out.push(h('details', {}, h('summary', {}, t('run.distribution')), h('ul', { class: 'small' }, view.load.map((l) => h('li', {}, t('run.load_line', { name: l.name, count: l.count, target: l.target.toLocaleString(locale()) }))))));
    return out;
  }

  show(first);
  return box;
}

// Several events at a time: an expandable list with ticks. selected() gives the ids.
function eventPicker(dates, label) {
  const boxes = dates.map((d) => ({ d, box: h('input', { type: 'checkbox', 'aria-label': `${d.date} ${d.day_type}` }) }));
  const summary = h('summary', {}, label);
  const update = () => { const n = boxes.filter((x) => x.box.checked).length; summary.textContent = n ? t('rules.picker_selected', { label, n }) : label; };
  boxes.forEach((x) => x.box.addEventListener('change', update));
  const set = (v) => () => { boxes.forEach((x) => { x.box.checked = v; }); update(); };
  const el = h('details', { class: 'picker' }, summary,
    h('div', { class: 'row-inline' }, h('button', { type: 'button', class: 'ghost', onclick: set(true) }, t('rules.select_all')), h('button', { type: 'button', class: 'ghost', onclick: set(false) }, t('rules.clear'))),
    h('div', { class: 'chips' }, boxes.map(({ d, box }) => h('label', { class: 'chip' }, box, ` ${d.date} ${d.day_type}`))));
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
    h('strong', {}, fmtDate(d.date)), ' ', h('span', { class: 'muted' }, d.day_type),
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

// The plan's runs and the latest one's view (or null). The list is always 200; asking for «latest» right away with no runs
// would give a 404 in the browser console.
async function runsAndLatest(path) {
  const runs = await api('GET', path);
  return { runs, latest: runs.length ? await api('GET', path + '/latest') : null };
}

async function pagePlan(r, org) {
  const base = `/orgs/${enc(r.org)}`;
  const [plan, dates, dayTypes, log, people, health, { runs, latest }, tasks, groups, cells, pools, weights] = await Promise.all([
    api('GET', `${base}/plans/${enc(r.plan)}`), api('GET', `${base}/plans/${enc(r.plan)}/events`),
    api('GET', base + '/day-types'), api('GET', `${base}/log?limit=30&plan=${enc(r.plan)}`), api('GET', base + '/people'),
    api('GET', '/health'), runsAndLatest(`${base}/plans/${enc(r.plan)}/runs`),
    api('GET', base + '/tasks'), api('GET', base + '/groups'), api('GET', `${base}/plans/${enc(r.plan)}/cells`), api('GET', `${base}/plans/${enc(r.plan)}/pools`), api('GET', `${base}/plans/${enc(r.plan)}/weights`)]);
  const refresh = () => render();
  const dpath = `${base}/plans/${enc(r.plan)}/events`;

  const rows = dates.map((d) => {
    const label = d.date + ' ' + d.day_type;
    const note = h('input', { type: 'text', value: d.note, maxlength: '200', placeholder: t('plan.note'), 'aria-label': t('plan.note_aria', { label }), onchange: async (e) => { if (await write(() => api('PATCH', `${dpath}/${d.id}`, { note: e.target.value }))) refresh(); else e.target.value = d.note; } });
    return h('li', { class: 'event-row' },
      h('strong', { class: 'when' }, fmtDate(d.date)),
      dayTypeSelect(dayTypes, d.day_type_id, async (e) => { if (await write(() => api('PATCH', `${dpath}/${d.id}`, { day_type_id: parseInt(e.target.value, 10) }))) refresh(); else e.target.value = String(d.day_type_id); }, t('plan.daytype_of_aria', { label })),
      note,
      h('button', { type: 'button', class: 'danger', 'aria-label': t('plan.delete_event_aria', { label }), onclick: async () => {
        if (!confirm(t('plan.delete_event_confirm', { date: fmtDate(d.date), daytype: d.day_type }))) return;
        if (await write(() => api('DELETE', `${dpath}/${d.id}`))) refresh();
      } }, t('common.delete')));
  });

  const active = people.filter((p) => p.active);
  const pick = h('select', { 'aria-label': t('plan.pick_aria') }, h('option', { value: '' }, t('plan.pick_name')), active.map((p) => h('option', { value: String(p.id) }, p.name)));
  const mine = (id) => `/o/${enc(r.org)}/p/${enc(r.plan)}/me/${id}`;
  const fillIn = section(t('plan.fill_in'),
    active.length ? h('div', { class: 'row-inline' }, pick, h('button', { type: 'button', onclick: () => { if (pick.value) go(mine(pick.value)); } }, t('plan.open_mine'))) : h('p', { class: 'muted' }, t('plan.add_people_a'), link(`/o/${enc(r.org)}`, t('plan.add_people_link')), t('plan.add_people_b')),
    active.length ? h('p', { class: 'muted small' }, t('plan.personal_hint')) : '',
    active.length ? h('div', { class: 'chips' }, active.map((p) => link(mine(p.id), p.name, { class: 'chip' }))) : '');

  const noTypes = !dayTypes.length;
  let addForm = '', bulkForm = '';
  if (!noTypes) {
    const date = h('input', { type: 'date', required: true, 'aria-label': t('plan.date_aria') });
    const typeSel = h('select', { 'aria-label': t('plan.daytype_aria') }, dayTypes.map((d) => h('option', { value: String(d.id) }, d.name)));
    const noteIn = h('input', { type: 'text', placeholder: t('plan.note_optional'), maxlength: '200', 'aria-label': t('plan.note') });
    addForm = h('form', { class: 'row', onsubmit: async (e) => { e.preventDefault(); if (await write(() => api('POST', dpath, { date: date.value, day_type_id: parseInt(typeSel.value, 10), note: noteIn.value }))) refresh(); } },
      date, typeSel, noteIn, h('button', { type: 'submit' }, t('plan.add_event')));

    const from = h('input', { type: 'date', required: true, 'aria-label': t('plan.from') });
    const to = h('input', { type: 'date', required: true, 'aria-label': t('plan.to') });
    const days = [1, 2, 3, 4, 5, 6, 7].map((v) => ({ n: t('week.' + v), v, box: h('input', { type: 'checkbox', checked: v === 7, 'aria-label': t('week.' + v) }) }));
    const bType = h('select', { 'aria-label': t('plan.bulk_type_aria') }, dayTypes.map((d) => h('option', { value: String(d.id) }, d.name)));
    bulkForm = h('form', { class: 'stack', onsubmit: async (e) => {
      e.preventDefault();
      let res;
      if (await write(async () => { res = await api('POST', dpath + '/bulk', { from: from.value, to: to.value, weekdays: days.filter((x) => x.box.checked).map((x) => x.v), day_type_id: parseInt(bType.value, 10) }); })) {
        await render(); say(t('plan.bulk_done', { created: res.created.length, skipped: res.skipped.length }));
      }
    } },
      h('div', { class: 'row-inline' }, h('label', {}, t('plan.from') + ' ', from), h('label', {}, t('plan.to') + ' ', to)),
      h('div', { class: 'row-inline' }, days.map((x) => h('label', { class: 'chip' }, x.box, ' ' + x.n))),
      h('div', { class: 'row-inline' }, bType, h('button', { type: 'submit' }, t('common.add_all'))));
  }

  return h('div', {},
    h('p', { class: 'small' }, link(`/o/${enc(r.org)}`, t('common.back', { name: org.name }))),
    h('h1', {}, plan.name),
    fillIn,
    section(t('plan.section'), planPanel(r, !!health.planner, latest, runs)),
    rulesSection(r, base, dates, tasks, groups, people, cells, pools),
    weightsSection(r, weights),
    section(t('plan.events'),
      noTypes ? h('p', { class: 'muted' }, t('plan.needs_daytype_a'), link(`/o/${enc(r.org)}/setup`, t('plan.needs_daytype_link')), t('plan.needs_daytype_b')) : '',
      dates.length ? h('ul', { class: 'events' }, rows) : h('p', { class: 'muted' }, t('plan.no_events')),
      addForm),
    section(t('plan.bulk_title'), noTypes ? h('p', { class: 'muted' }, t('plan.needs_daytype')) : bulkForm),
    section(t('log.title') + ': ' + plan.name, logTable(log)),
    managePlan(r, plan));
}

// A group of buttons where one can be chosen (aria-pressed). set(level) updates the view without redrawing the page.
function seg(label, options, current, onpick) {
  const buttons = options.map(([value, key]) => h('button', { type: 'button', 'data-level': value, 'aria-pressed': String(current === value), onclick: () => onpick(value) }, t(key)));
  return { el: h('div', { class: 'seg', role: 'group', 'aria-label': label }, buttons),
    set(level) { buttons.forEach((b) => b.setAttribute('aria-pressed', String(b.dataset.level === level))); } };
}

// «My page»: what I can take part in (per organisation) and when I can (per event in the plan). Answers are saved at once.
async function pageMe(r, org) {
  const base = `/orgs/${enc(r.org)}`;
  const [plan, me, mine] = await Promise.all([api('GET', `${base}/plans/${enc(r.plan)}`), api('GET', `${base}/plans/${enc(r.plan)}/people/${enc(r.person)}/me`), api('GET', `${base}/plans/${enc(r.plan)}/people/${enc(r.person)}/assignments`)]);
  const taskPath = (id) => `${base}/people/${me.person.id}/tasks/${id}`;
  const availPath = `${base}/plans/${enc(r.plan)}/people/${me.person.id}/availability`;
  const refresh = () => render();
  const list = (cls, items) => h('ul', { class: cls }, items);

  // --- my assignments: from the latest finished plan
  const mineSection = (() => {
    if (!mine.has_plan) return section(t('me.assignments'), h('p', { class: 'muted' }, t('me.no_plan')));
    const rows = mine.assignments.map((a) => h('li', { class: 'mine-row' },
      h('strong', {}, fmtDate(a.date)),
      h('span', {}, a.via ? t('me.with', { task: a.task, via: a.via }) : a.task, a.locked ? h('span', { class: 'tag lock' }, t('cell.locked')) : '', a.weak ? h('span', { class: 'tag' }, t('cell.poor')) : ''),
      h('span', { class: 'small muted' }, a.day_type + (a.note ? ' · ' + a.note : ''))));
    return section(t('me.assignments'),
      h('p', { class: 'small muted' }, t(mine.run.pinned ? 'me.from_pinned' : 'me.from_latest', { actor: mine.run.actor, time: fmtTime(mine.run.finished_at) })),
      mine.run.stale ? h('p', { class: 'warn small', role: 'status' }, t('me.stale')) : '',
      rows.length ? [h('p', { class: 'small' }, tn('me.count', rows.length)), h('ul', { class: 'mine' }, rows)] : h('p', { class: 'muted' }, t('me.none')));
  })();

  // --- tasks
  const chosen = me.tasks.filter((tk) => tk.level);
  const rest = me.tasks.filter((tk) => !tk.level);
  const taskRows = chosen.map((tk) => {
    const ctl = seg(t('me.how_willing_aria', { name: tk.name }), TASK_LEVELS, tk.level, async (v) => { if (v !== tk.level && await write(() => api('PUT', taskPath(tk.task_id), { level: v }))) { tk.level = v; ctl.set(v); } });
    return h('li', { class: 'card-row' },
      h('div', { class: 'row-head' }, h('strong', {}, tk.name), h('button', { type: 'button', class: 'link-button', 'aria-label': t('me.remove_aria', { name: tk.name }), onclick: async () => { if (await write(() => api('DELETE', taskPath(tk.task_id)))) refresh(); } }, t('common.remove'))),
      ctl.el);
  });
  const addRows = rest.map((tk) => h('li', { class: 'card-row row-head' }, h('strong', {}, tk.name),
    h('button', { type: 'button', 'aria-label': t('me.add_aria', { name: tk.name }), onclick: async () => { if (await write(() => api('PUT', taskPath(tk.task_id), { level: 'neutral' }))) refresh(); } }, t('common.add'))));
  const taskSection = section(t('me.tasks'),
    !me.tasks.length ? h('p', { class: 'muted' }, t('me.no_tasks'), link(`/o/${enc(r.org)}/setup`, t('me.setup_link'))) : '',
    chosen.length ? list('cards', taskRows) : (me.tasks.length ? h('p', { class: 'muted' }, t('me.no_chosen')) : ''),
    me.tasks.length ? h('details', { open: !chosen.length }, h('summary', {}, chosen.length ? t('me.choose_more') : t('me.choose')), rest.length ? list('cards', addRows) : h('p', { class: 'muted' }, t('me.all_chosen'))) : '');

  // --- when can you? One answer per event: does not suit / poor / good; not marked counts as neutral.
  const levels = {};
  me.events.forEach((e) => { levels[e.event_id] = e.level; });
  const ctls = {};
  const summary = h('p', { class: 'muted small', 'aria-live': 'polite' });
  const updateSummary = () => {
    const c = { never: 0, poor: 0, good: 0 };
    Object.values(levels).forEach((l) => { if (l) c[l]++; });
    summary.textContent = t('me.summary', { ...c, neutral: me.events.length - c.never - c.poor - c.good });
  };
  async function setLevels(changes) {
    if (!(await write(() => api('PUT', availPath, { levels: changes })))) return false;
    for (const [id, l] of Object.entries(changes)) { levels[id] = l; ctls[id].set(l); }
    updateSummary();
    return true;
  }
  const rows = me.events.map((e) => {
    const ctl = seg(t('me.can_you', { date: fmtDate(e.date) }), EVENT_LEVELS, e.level, (v) => setLevels({ [e.event_id]: levels[e.event_id] === v ? '' : v }));
    ctls[e.event_id] = ctl;
    return h('li', { class: 'avail-row' },
      h('div', { class: 'what' }, h('strong', { class: 'when' }, fmtDate(e.date)), h('span', { class: 'muted small' }, e.day_type + (e.note ? ' · ' + e.note : ''))), ctl.el);
  });
  const all = (v) => async () => {
    const changes = {};
    for (const e of me.events) if ((levels[e.event_id] || '') !== v) changes[e.event_id] = v;
    if (!Object.keys(changes).length) return;
    const overwritten = me.events.filter((e) => levels[e.event_id] && levels[e.event_id] !== v).length;
    if (overwritten && !confirm(tn('me.overwrite', overwritten))) return;
    await setLevels(changes);
  };
  updateSummary();
  const bulk = me.events.length ? h('div', { class: 'row-inline' }, h('span', { class: 'muted small' }, t('me.set_all')),
    h('button', { type: 'button', class: 'ghost', onclick: all('never') }, t('level.event.never')), h('button', { type: 'button', class: 'ghost', onclick: all('poor') }, t('level.poor')),
    h('button', { type: 'button', class: 'ghost', onclick: all('good') }, t('level.good')), h('button', { type: 'button', class: 'ghost', onclick: all('') }, t('me.clear_all'))) : '';

  const root = h('div', {},
    h('p', { class: 'small' }, link(`/o/${enc(r.org)}/p/${enc(r.plan)}`, t('common.back', { name: plan.name }))),
    h('h1', {}, me.person.name),
    h('p', { class: 'muted' }, t('me.intro')),
    me.person.active ? '' : h('p', { class: 'err' }, t('me.inactive')),
    mineSection,
    taskSection,
    section(t('me.when', { plan: plan.name }),
      me.events.length ? [h('p', { class: 'muted small' }, t('me.avail_hint')), bulk, summary, list('avail', rows)] : h('p', { class: 'muted' }, t('me.no_events'))));
  root.dataset.person = me.person.name;
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
    const [, body] = await Promise.all([renderHeader(r, people), r.page === 'home' ? pageHome() : r.page === 'org' ? pageOrg(r, org, people) : r.page === 'setup' ? pageSetup(r, org) : r.page === 'me' ? pageMe(r, org) : pagePlan(r, org)]);
    if (seq !== renderSeq || !body) return;
    const y = location.pathname === lastPath ? window.scrollY : 0;  // the same page again (after a change): stay where we are
    main.replaceChildren(body);
    lastPath = location.pathname;
    window.scrollTo(0, y);
    if (r.page === 'me' && body.dataset.person && !store.get('actor').trim()) {  // whoever opens their own page is most likely that person
      store.set('actor', body.dataset.person);
      const a = document.getElementById('actor');
      if (a) a.value = body.dataset.person;
    }
    document.title = t('brand') + (org ? ' · ' + org.name : '');
  } catch (e) {
    if (seq !== renderSeq) return;
    if (e.status === 404 && r.org) store.set('org', '');
    main.replaceChildren(h('div', {}, h('h1', {}, e.status === 404 ? t('page.not_found') : t('page.error')), h('p', {}, link('/?switch', t('page.choose_org'))), h('p', { class: 'muted small' }, errorText(e))));
  }
}

window.addEventListener('popstate', render);
render();
