'use strict';
// Tjenesteplan: liten frontend uten rammeverk. Organisasjonen ligger i adressen (/o/<org>/...), så å bytte
// organisasjon er bare å gå til en annen adresse. Ingen innlogging: «Hvem er du?» er selvoppgitt og brukes i endringsloggen.

const store = {
  get: (k) => { try { return localStorage.getItem('tp.' + k) || ''; } catch (e) { return ''; } },
  set: (k, v) => { try { localStorage.setItem('tp.' + k, v); } catch (e) { /* privat modus */ } },
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

async function api(method, path, body) {
  const headers = {};
  if (body !== undefined) headers['Content-Type'] = 'application/json';
  if (method !== 'GET') headers['X-Actor'] = encodeURIComponent(store.get('actor'));
  const res = await fetch('/api' + path, { method, headers, body: body === undefined ? undefined : JSON.stringify(body) });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw Object.assign(new Error(data.message || res.statusText), { code: data.error, status: res.status });
  return data;
}

// Skrivehandlinger: krever et navn øverst, viser feil i stedet for å kaste.
async function write(action) {
  if (!store.get('actor').trim()) {
    say('Skriv hvem du er øverst på siden først. Navnet brukes i endringsloggen.', 'err');
    return false;
  }
  try {
    await action();
    say('');
    return true;
  } catch (e) {
    if (e.cancelled) return false;
    say(e.code === 'conflict' ? 'Finnes allerede: ' + e.message : e.message, 'err');
    return false;
  }
}

const enc = encodeURIComponent;
const fmtTime = (iso) => new Date(iso).toLocaleString('nb-NO', { dateStyle: 'short', timeStyle: 'short' });

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
    if (parts[2] === 'p' && parts[3] && parts[4] === 'meg' && parts[5]) return { page: 'me', org: parts[1], plan: parts[3], person: parts[5] };
    if (parts[2] === 'p' && parts[3]) return { page: 'plan', org: parts[1], plan: parts[3] };
    if (parts[2] === 'oppsett') return { page: 'setup', org: parts[1] };
    return { page: 'org', org: parts[1] };
  }
  return { page: 'home' };
}

async function renderHeader(r, people) {
  const orgs = await api('GET', '/orgs');
  const select = h('select', { 'aria-label': 'Organisasjon', onchange: (e) => go(e.target.value === '' ? '/?bytt' : '/o/' + enc(e.target.value)) },
    h('option', { value: '' }, r.org ? 'Bytt organisasjon…' : 'Velg organisasjon…'),
    orgs.map((o) => h('option', { value: o.slug, selected: o.slug === r.org }, o.name)));
  const actor = h('input', { type: 'text', id: 'actor', list: 'people', value: store.get('actor'), placeholder: 'Skriv eller velg navnet ditt', maxlength: '80', autocomplete: 'off',
    onchange: (e) => store.set('actor', e.target.value.trim()) });
  document.getElementById('top').replaceChildren(
    link('/?bytt', 'Tjenesteplan', { class: 'brand' }),
    h('label', {}, 'Organisasjon', select),
    h('span', { class: 'spacer' }),
    h('label', { title: 'Brukes i endringsloggen. Det er ingen innlogging.' }, 'Hvem er du?', actor),
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
  if (last && !location.search.includes('bytt') && orgs.some((o) => o.slug === last)) return go('/o/' + enc(last));
  return h('div', {},
    h('h1', {}, 'Velg organisasjon'),
    orgs.length ? h('ul', { class: 'plain' }, orgs.map((o) => h('li', {}, link('/o/' + enc(o.slug), o.name)))) : h('p', { class: 'muted' }, 'Ingen organisasjoner ennå. Lag den første under.'),
    h('h2', {}, 'Ny organisasjon'),
    nameForm('Navn, f.eks. Menigheten', 'Opprett', async (name) => {
      const o = await api('POST', '/orgs', { name });
      store.set('org', o.slug);
      go('/o/' + enc(o.slug));
    }),
    h('p', { class: 'muted small' }, 'Alle som kjenner adressen ser organisasjonene her. Det er ingen innlogging ennå.'));
}

function logTable(entries) {
  if (!entries.length) return h('p', { class: 'muted' }, 'Ingen endringer ennå.');
  return h('ul', { class: 'log' }, entries.map((e) => h('li', {},
    h('span', { class: 'when' }, fmtTime(e.at)), h('span', { class: 'who' }, e.actor),
    h('span', { class: 'what' }, e.action + (e.plan ? ' (' + e.plan + ')' : '')), h('span', { class: 'detail' }, e.detail))));
}

async function pageOrg(r, org, people) {
  const [plans, log] = await Promise.all([api('GET', `/orgs/${enc(r.org)}/plans`), api('GET', `/orgs/${enc(r.org)}/log?limit=30`)]);
  const refresh = () => render();
  const peopleRows = people.map((p) => h('tr', {},
    h('td', {}, p.name),
    h('td', {}, h('input', { type: 'checkbox', 'aria-label': 'Aktiv: ' + p.name, checked: p.active,
      onchange: async (e) => { if (await write(() => api('PATCH', `/orgs/${enc(r.org)}/people/${p.id}`, { active: e.target.checked }))) refresh(); else e.target.checked = p.active; } })),
    h('td', {}, h('input', { type: 'checkbox', 'aria-label': 'Kan foreslås: ' + p.name, checked: p.proposable,
      onchange: async (e) => { if (await write(() => api('PATCH', `/orgs/${enc(r.org)}/people/${p.id}`, { proposable: e.target.checked }))) refresh(); else e.target.checked = p.proposable; } }))));
  return h('div', {},
    h('h1', {}, org.name),
    h('p', {}, link(`/o/${enc(r.org)}/oppsett`, 'Oppsett: oppgaver, dagtyper og kombinasjoner →')),
    h('h2', {}, 'Planer'),
    plans.length ? h('ul', { class: 'plain' }, plans.map((p) => h('li', {}, link(`/o/${enc(r.org)}/p/${enc(p.slug)}`, p.name)))) : h('p', { class: 'muted' }, 'Ingen planer ennå. Lag en, f.eks. «Høst 2026».'),
    nameForm('Ny plan, f.eks. Høst 2026', 'Opprett plan', async (name) => { const p = await api('POST', `/orgs/${enc(r.org)}/plans`, { name }); go(`/o/${enc(r.org)}/p/${enc(p.slug)}`); }),
    h('h2', {}, 'Personer'),
    people.length ? h('table', {}, h('thead', {}, h('tr', {}, h('th', {}, 'Navn'), h('th', {}, 'Aktiv'), h('th', {}, 'Kan foreslås'))), h('tbody', {}, peopleRows)) : h('p', { class: 'muted' }, 'Ingen personer ennå.'),
    nameForm('Ny person', 'Legg til', async (name) => { await api('POST', `/orgs/${enc(r.org)}/people`, { name }); refresh(); }),
    h('p', { class: 'muted small' }, '«Kan foreslås» av: planleggeren velger aldri personen selv, men du kan legge dem inn for hånd.'),
    h('h2', {}, 'Endringslogg'),
    logTable(log));
}

const fmtDate = (iso) => new Date(iso + 'T00:00:00Z').toLocaleDateString('nb-NO', { weekday: 'short', day: 'numeric', month: 'short', year: 'numeric', timeZone: 'UTC' });
const SELECT_LEVELS = [['standard', 'standard'], ['kan_ikke', 'kan ikke kombineres'], ['upraktisk', 'upraktisk'], ['praktisk', 'praktisk'], ['bor', 'bør kombineres']];

function section(title, ...kids) { return h('section', {}, h('h2', {}, title), ...kids); }

async function pageSetup(r, org) {
  const base = `/orgs/${enc(r.org)}`;
  const [tasks, dayTypes, combos] = await Promise.all([api('GET', base + '/tasks'), api('GET', base + '/day-types'), api('GET', base + '/combinations')]);
  const refresh = () => render();
  const act = (fn) => async () => { if (await write(fn)) refresh(); };

  const taskRows = tasks.map((t) => h('tr', {},
    h('td', {}, t.name),
    h('td', {}, h('input', { type: 'checkbox', 'aria-label': 'Fordeles av planleggeren: ' + t.name, checked: t.planned,
      onchange: async (e) => { if (await write(() => api('PATCH', `${base}/tasks/${t.id}`, { planned: e.target.checked }))) refresh(); else e.target.checked = t.planned; } })),
    h('td', {}, h('button', { type: 'button', class: 'danger', onclick: act(async () => { if (confirm(`Slette «${t.name}»?`)) await api('DELETE', `${base}/tasks/${t.id}`); else throw Object.assign(new Error('Avbrutt'), { cancelled: true }); }) }, 'Slett'))));

  const pasted = h('textarea', { rows: '4', placeholder: 'Flere oppgaver på en gang: ett navn per linje', 'aria-label': 'Flere oppgaver, ett navn per linje' });

  const dayTypeBlocks = dayTypes.map((d) => {
    const inputs = tasks.map((t) => {
      const cur = d.tasks.find((x) => x.task_id === t.id);
      return { t, input: h('input', { type: 'number', min: '0', max: '50', value: cur ? String(cur.slots) : '0', 'aria-label': `Plasser for ${t.name} på ${d.name}` }) };
    });
    const name = h('input', { type: 'text', value: d.name, maxlength: '80', 'aria-label': 'Navn på dagtype' });
    return h('div', { class: 'card' },
      h('div', { class: 'row-inline' }, name,
        h('button', { type: 'button', onclick: act(() => api('PATCH', `${base}/day-types/${d.id}`, { name: name.value })) }, 'Gi nytt navn'),
        h('button', { type: 'button', class: 'danger', onclick: act(async () => { if (confirm(`Slette dagtypen «${d.name}»?`)) await api('DELETE', `${base}/day-types/${d.id}`); else throw Object.assign(new Error('Avbrutt'), { cancelled: true }); }) }, 'Slett')),
      tasks.length ? h('table', {}, h('thead', {}, h('tr', {}, h('th', {}, 'Oppgave'), h('th', {}, 'Plasser (0 = ikke med)'))),
        h('tbody', {}, inputs.map(({ t, input }) => h('tr', {}, h('td', {}, t.name), h('td', {}, input))))) : h('p', { class: 'muted' }, 'Lag oppgaver først.'),
      tasks.length ? h('button', { type: 'button', onclick: act(() => api('PUT', `${base}/day-types/${d.id}/tasks`, {
        tasks: inputs.map(({ t, input }) => ({ task_id: t.id, slots: parseInt(input.value, 10) || 0 })).filter((x) => x.slots > 0),
      })) }, 'Lagre oppgaver') : '');
  });

  const taskName = (id) => (tasks.find((t) => t.id === id) || {}).name;
  const level = (a, b) => (combos.find((c) => c.a === a && c.b === b) || { level: 'standard' }).level;
  const pairRows = [];
  for (let i = 0; i < tasks.length; i++) for (let j = i + 1; j < tasks.length; j++) {
    const x = tasks[i], y = tasks[j];
    const select = h('select', { 'aria-label': `${x.name} og ${y.name}`, onchange: async (e) => { if (!(await write(() => api('PUT', base + '/combinations', { a: x.id, b: y.id, level: e.target.value })))) e.target.value = level(Math.min(x.id, y.id), Math.max(x.id, y.id)); } },
      SELECT_LEVELS.map(([v, label]) => h('option', { value: v, selected: v === level(Math.min(x.id, y.id), Math.max(x.id, y.id)) }, label)));
    pairRows.push(h('tr', {}, h('td', {}, taskName(x.id) + ' + ' + taskName(y.id)), h('td', {}, select)));
  }

  return h('div', {},
    h('p', { class: 'small' }, link(`/o/${enc(r.org)}`, '← ' + org.name)),
    h('h1', {}, 'Oppsett'),
    h('p', { class: 'muted' }, 'Dette gjelder alle planene i organisasjonen. Alt her lager du selv.'),
    section('Oppgaver',
      tasks.length ? h('table', {}, h('thead', {}, h('tr', {}, h('th', {}, 'Navn'), h('th', {}, 'Fordeles av planleggeren'), h('th', {}, ''))), h('tbody', {}, taskRows)) : h('p', { class: 'muted' }, 'Ingen oppgaver ennå.'),
      nameForm('Ny oppgave', 'Legg til', async (name) => { await api('POST', base + '/tasks', { name }); refresh(); }),
      h('form', { class: 'stack', onsubmit: async (e) => { e.preventDefault(); if (await write(async () => { await api('POST', base + '/tasks/bulk', { names: pasted.value }); })) refresh(); } },
        pasted, h('button', { type: 'submit' }, 'Legg til alle'))),
    section('Dagtyper',
      h('p', { class: 'muted small' }, 'En dagtype sier hvilke oppgaver en hendelse har og hvor mange plasser (f.eks. «Gudstjeneste»: Lyd 1, Kafé 3).'),
      dayTypes.length ? dayTypeBlocks : h('p', { class: 'muted' }, 'Ingen dagtyper ennå.'),
      nameForm('Ny dagtype, f.eks. Gudstjeneste', 'Opprett', async (name) => { await api('POST', base + '/day-types', { name }); refresh(); })),
    section('Hva kan kombineres samme dag?',
      h('p', { class: 'muted small' }, '«Standard» regnes som upraktisk. Innstillingen er felles for alle i organisasjonen.'),
      pairRows.length ? h('table', {}, h('tbody', {}, pairRows)) : h('p', { class: 'muted' }, 'Lag minst to oppgaver først.')));
}

function dayTypeSelect(dayTypes, value, onchange, label) {
  return h('select', { 'aria-label': label, onchange }, dayTypes.map((d) => h('option', { value: String(d.id), selected: d.id === value }, d.name)));
}

// «Plan»: siste kjøring av planleggeren som en liste over hendelser (mobil først), med knapp for å planlegge eller prøve igjen.
// Kjøringen skjer i tjeneren; panelet spør etter status hvert sekund til den er ferdig og tegner bare seg selv på nytt.
function planPanel(r, canPlan, first) {
  const path = `/orgs/${enc(r.org)}/plans/${enc(r.plan)}/runs`;
  const box = h('div', { class: 'plan-panel' });
  const limits = [[10, '10 s'], [20, '20 s'], [60, '60 s']];
  const limitSel = h('select', { 'aria-label': 'Tid planleggeren får' }, limits.map(([v, t]) => h('option', { value: String(v), selected: String(v) === (store.get('limit') || '20') }, t)));
  limitSel.addEventListener('change', () => store.set('limit', limitSel.value));
  let timer = 0;

  async function start() {
    let view;
    if (await write(async () => { const res = await api('POST', path, { time_limit: parseInt(limitSel.value, 10) }); view = await api('GET', `${path}/${res.id}`); })) show(view);
  }
  async function poll() {
    if (!box.isConnected) return;  // siden er byttet ut
    try { show(await api('GET', `${path}/latest`)); } catch (e) { say(e.message, 'err'); }
  }
  function show(view) {
    clearTimeout(timer);
    box.replaceChildren(...body(view));
    if (view && view.run.status === 'running') timer = setTimeout(poll, 1000);
  }

  function cell(c, taskName) {
    const who = c.people.map((p) => h('span', { class: 'person' }, p.name, p.weak ? h('span', { class: 'tag' }, 'dårlig') : ''));
    if (c.missing > 0) who.push(h('span', { class: 'err' }, c.missing === c.slots ? 'Ikke fylt' : `Mangler ${c.missing}`));
    return [h('div', { class: 'task' }, taskName),
      h('div', {}, h('div', { class: 'who' }, who), c.missing > 0 && c.explanation ? h('div', { class: 'small muted' }, c.explanation) : '')];
  }

  function body(view) {
    const running = !!view && view.run.status === 'running';
    const out = [h('div', { class: 'row-inline' },
      h('button', { type: 'button', disabled: !canPlan || running, onclick: start }, running ? 'Planlegger…' : view ? 'Prøv igjen' : 'Planlegg'),
      h('label', { class: 'small muted' }, 'Tid ', limitSel))];
    if (!canPlan) out.push(h('p', { class: 'muted' }, 'Planleggeren er ikke satt opp på denne tjeneren.'));
    if (!view) { out.push(h('p', { class: 'muted' }, 'Ingen plan er laget ennå. Når folk har svart, trykk «Planlegg».')); return out; }
    const run = view.run;
    if (running) { out.push(h('p', { role: 'status' }, `Planlegger … det tar opptil ${run.time_limit} sekunder.`)); return out; }
    if (run.status === 'failed') { out.push(h('p', { class: 'err', role: 'alert' }, 'Planleggingen feilet: ' + (run.error || 'ukjent feil'))); return out; }

    out.push(h('p', { class: 'small muted' }, `Laget av ${run.actor} ${fmtTime(run.finished_at || run.started_at)}. ` + (run.solver_status === 'OPTIMAL' ? 'Beste mulige plan funnet.' : 'God plan funnet; kan finnes en enda bedre om den får mer tid.')));
    if (run.stale) out.push(h('p', { class: 'warn', role: 'status' }, 'Svar eller oppsett er endret siden denne planen ble laget. Trykk «Prøv igjen» for en oppdatert plan.'));
    if (run.checked && run.violations.length) out.push(h('div', { class: 'err', role: 'alert' }, h('p', {}, 'Den uavhengige kontrollen fant brudd mot reglene:'), h('ul', {}, run.violations.map((v) => h('li', {}, v)))));
    else if (run.checked) out.push(h('p', { class: 'small muted' }, 'Kontrollert uavhengig: ingen brudd mot reglene.'));
    if (view.warnings.length) out.push(h('ul', { class: 'warn small' }, view.warnings.map((w) => h('li', {}, w))));

    const names = new Map(view.tasks.map((t) => [t.id, t.name]));
    out.push(h('ul', { class: 'plan-events' }, view.events.map((ev) => h('li', { class: 'plan-event' },
      h('div', {}, h('strong', {}, fmtDate(ev.date)), ' ', h('span', { class: 'muted' }, ev.day_type + (ev.note ? ' · ' + ev.note : ''))),
      h('div', { class: 'plan-cells' }, ev.cells.flatMap((c) => cell(c, names.get(c.task_id) || '?')))))));
    if (view.weak.length) out.push(h('details', {}, h('summary', {}, `Kompromisser (${view.weak.length})`), h('ul', { class: 'small' }, view.weak.map((w) => h('li', {}, w)))));
    if (view.load.length) out.push(h('details', {}, h('summary', {}, 'Fordeling'), h('ul', { class: 'small' }, view.load.map((l) => h('li', {}, `${l.name}: ${l.count} (jevn andel ${l.target.toLocaleString('nb-NO')})`)))));
    return out;
  }

  show(first);
  return box;
}

// Siste kjøring, eller null. Lista er alltid 200; å spørre rett etter «latest» uten kjøringer ville gi 404 i nettleserens konsoll.
async function latestRun(path) {
  return (await api('GET', path)).length ? api('GET', path + '/latest') : null;
}

async function pagePlan(r, org) {
  const base = `/orgs/${enc(r.org)}`;
  const [plan, dates, dayTypes, log, people, health, latest] = await Promise.all([
    api('GET', `${base}/plans/${enc(r.plan)}`), api('GET', `${base}/plans/${enc(r.plan)}/events`),
    api('GET', base + '/day-types'), api('GET', `${base}/log?limit=30&plan=${enc(r.plan)}`), api('GET', base + '/people'),
    api('GET', '/health'), latestRun(`${base}/plans/${enc(r.plan)}/runs`)]);
  const refresh = () => render();
  const dpath = `${base}/plans/${enc(r.plan)}/events`;

  const rows = dates.map((d) => {
    const label = d.date + ' ' + d.day_type;
    const note = h('input', { type: 'text', value: d.note, maxlength: '200', placeholder: 'Merknad', 'aria-label': 'Merknad ' + label, onchange: async (e) => { if (await write(() => api('PATCH', `${dpath}/${d.id}`, { note: e.target.value }))) refresh(); else e.target.value = d.note; } });
    return h('li', { class: 'event-row' },
      h('strong', { class: 'when' }, fmtDate(d.date)),
      dayTypeSelect(dayTypes, d.day_type_id, async (e) => { if (await write(() => api('PATCH', `${dpath}/${d.id}`, { day_type_id: parseInt(e.target.value, 10) }))) refresh(); else e.target.value = String(d.day_type_id); }, 'Dagtype ' + label),
      note,
      h('button', { type: 'button', class: 'danger', 'aria-label': 'Slett hendelsen ' + label, onclick: async () => {
        if (!confirm(`Slette hendelsen ${fmtDate(d.date)} (${d.day_type})? Svarene folk har gitt for den, forsvinner også.`)) return;
        if (await write(() => api('DELETE', `${dpath}/${d.id}`))) refresh();
      } }, 'Slett'));
  });

  const active = people.filter((p) => p.active);
  const pick = h('select', { 'aria-label': 'Velg navnet ditt' }, h('option', { value: '' }, 'Velg navnet ditt…'), active.map((p) => h('option', { value: String(p.id) }, p.name)));
  const mine = (id) => `/o/${enc(r.org)}/p/${enc(r.plan)}/meg/${id}`;
  const fillIn = section('Fyll ut når du kan',
    active.length ? h('div', { class: 'row-inline' }, pick, h('button', { type: 'button', onclick: () => { if (pick.value) go(mine(pick.value)); } }, 'Åpne min side')) : h('p', { class: 'muted' }, 'Legg til personer på ', link(`/o/${enc(r.org)}`, 'organisasjonssiden'), ' først.'),
    active.length ? h('p', { class: 'muted small' }, 'Hver person har sin egen side. Lenken kan sendes til personen.') : '',
    active.length ? h('div', { class: 'chips' }, active.map((p) => link(mine(p.id), p.name, { class: 'chip' }))) : '');

  const noTypes = !dayTypes.length;
  let addForm = '', bulkForm = '';
  if (!noTypes) {
    const date = h('input', { type: 'date', required: true, 'aria-label': 'Dato' });
    const typeSel = h('select', { 'aria-label': 'Dagtype' }, dayTypes.map((d) => h('option', { value: String(d.id) }, d.name)));
    const noteIn = h('input', { type: 'text', placeholder: 'Merknad (valgfri)', maxlength: '200', 'aria-label': 'Merknad' });
    addForm = h('form', { class: 'row', onsubmit: async (e) => { e.preventDefault(); if (await write(() => api('POST', dpath, { date: date.value, day_type_id: parseInt(typeSel.value, 10), note: noteIn.value }))) refresh(); } },
      date, typeSel, noteIn, h('button', { type: 'submit' }, 'Legg til hendelse'));

    const from = h('input', { type: 'date', required: true, 'aria-label': 'Fra' });
    const to = h('input', { type: 'date', required: true, 'aria-label': 'Til' });
    const days = ['Man', 'Tir', 'Ons', 'Tor', 'Fre', 'Lør', 'Søn'].map((n, i) => ({ n, v: i + 1, box: h('input', { type: 'checkbox', checked: i === 6, 'aria-label': n }) }));
    const bType = h('select', { 'aria-label': 'Dagtype for alle' }, dayTypes.map((d) => h('option', { value: String(d.id) }, d.name)));
    bulkForm = h('form', { class: 'stack', onsubmit: async (e) => {
      e.preventDefault();
      let res;
      if (await write(async () => { res = await api('POST', dpath + '/bulk', { from: from.value, to: to.value, weekdays: days.filter((x) => x.box.checked).map((x) => x.v), day_type_id: parseInt(bType.value, 10) }); })) {
        await render(); say(`${res.created.length} nye hendelser${res.skipped.length ? ', ' + res.skipped.length + ' fantes fra før' : ''}.`);
      }
    } },
      h('div', { class: 'row-inline' }, h('label', {}, 'Fra ', from), h('label', {}, 'Til ', to)),
      h('div', { class: 'row-inline' }, days.map((x) => h('label', { class: 'chip' }, x.box, ' ' + x.n))),
      h('div', { class: 'row-inline' }, bType, h('button', { type: 'submit' }, 'Legg til alle')));
  }

  return h('div', {},
    h('p', { class: 'small' }, link(`/o/${enc(r.org)}`, '← ' + org.name)),
    h('h1', {}, plan.name),
    fillIn,
    section('Plan', planPanel(r, !!health.planner, latest)),
    section('Hendelser',
      noTypes ? h('p', { class: 'muted' }, 'Lag minst én dagtype under ', link(`/o/${enc(r.org)}/oppsett`, 'oppsett'), ' for å legge til hendelser.') : '',
      dates.length ? h('ul', { class: 'events' }, rows) : h('p', { class: 'muted' }, 'Ingen hendelser ennå.'),
      addForm),
    section('Legg til flere hendelser (f.eks. alle søndager)', noTypes ? h('p', { class: 'muted' }, 'Trenger en dagtype.') : bulkForm),
    section('Endringslogg for planen', logTable(log)));
}

const TASK_LEVELS = [['ikke', 'Ikke'], ['darlig', 'Dårlig'], ['noytral', 'Nøytral'], ['godt', 'Godt']];
const EVENT_LEVELS = [['ikke', 'Passer ikke'], ['darlig', 'Dårlig'], ['godt', 'Godt']];

// Gruppe av knapper der én kan være valgt (aria-pressed). set(level) oppdaterer visningen uten å tegne siden på nytt.
function seg(label, options, current, onpick) {
  const buttons = options.map(([value, text]) => h('button', { type: 'button', 'data-level': value, 'aria-pressed': String(current === value), onclick: () => onpick(value) }, text));
  return { el: h('div', { class: 'seg', role: 'group', 'aria-label': label }, buttons),
    set(level) { buttons.forEach((b) => b.setAttribute('aria-pressed', String(b.dataset.level === level))); } };
}

// «Min side»: hva jeg kan være med på (per organisasjon) og når jeg kan (per hendelse i planen). Svarene lagres med en gang.
async function pageMe(r, org) {
  const base = `/orgs/${enc(r.org)}`;
  const [plan, me] = await Promise.all([api('GET', `${base}/plans/${enc(r.plan)}`), api('GET', `${base}/plans/${enc(r.plan)}/people/${enc(r.person)}/me`)]);
  const taskPath = (id) => `${base}/people/${me.person.id}/tasks/${id}`;
  const availPath = `${base}/plans/${enc(r.plan)}/people/${me.person.id}/availability`;
  const refresh = () => render();
  const list = (cls, items) => h('ul', { class: cls }, items);

  // --- oppgaver
  const chosen = me.tasks.filter((t) => t.level);
  const rest = me.tasks.filter((t) => !t.level);
  const taskRows = chosen.map((t) => {
    const ctl = seg('Hvor gjerne: ' + t.name, TASK_LEVELS, t.level, async (v) => { if (v !== t.level && await write(() => api('PUT', taskPath(t.task_id), { level: v }))) { t.level = v; ctl.set(v); } });
    return h('li', { class: 'card-row' },
      h('div', { class: 'row-head' }, h('strong', {}, t.name), h('button', { type: 'button', class: 'link-button', 'aria-label': 'Ta bort ' + t.name, onclick: async () => { if (await write(() => api('DELETE', taskPath(t.task_id)))) refresh(); } }, 'Ta bort')),
      ctl.el);
  });
  const addRows = rest.map((t) => h('li', { class: 'card-row row-head' }, h('strong', {}, t.name),
    h('button', { type: 'button', 'aria-label': 'Legg til ' + t.name, onclick: async () => { if (await write(() => api('PUT', taskPath(t.task_id), { level: 'noytral' }))) refresh(); } }, 'Legg til')));
  const taskSection = section('Hva kan du være med på?',
    !me.tasks.length ? h('p', { class: 'muted' }, 'Ingen oppgaver er satt opp ennå. ', link(`/o/${enc(r.org)}/oppsett`, 'Oppsett')) : '',
    chosen.length ? list('cards', taskRows) : (me.tasks.length ? h('p', { class: 'muted' }, 'Du har ikke valgt noen oppgaver ennå. Velg de du kan tenke deg.') : ''),
    me.tasks.length ? h('details', { open: !chosen.length }, h('summary', {}, chosen.length ? 'Velg flere oppgaver' : 'Velg oppgaver'), rest.length ? list('cards', addRows) : h('p', { class: 'muted' }, 'Du har valgt alle oppgavene.')) : '');

  // --- når kan du? Ett svar per hendelse: passer ikke / dårlig / godt, ikke markert er nøytralt.
  const levels = {};
  me.events.forEach((e) => { levels[e.event_id] = e.level; });
  const ctls = {};
  const summary = h('p', { class: 'muted small', 'aria-live': 'polite' });
  const updateSummary = () => {
    const c = { ikke: 0, darlig: 0, godt: 0 };
    Object.values(levels).forEach((l) => { if (l) c[l]++; });
    summary.textContent = `${c.ikke} passer ikke · ${c.darlig} dårlig · ${c.godt} godt · ${me.events.length - c.ikke - c.darlig - c.godt} nøytrale`;
  };
  async function setLevels(changes) {
    if (!(await write(() => api('PUT', availPath, { levels: changes })))) return false;
    for (const [id, l] of Object.entries(changes)) { levels[id] = l; ctls[id].set(l); }
    updateSummary();
    return true;
  }
  const rows = me.events.map((e) => {
    const ctl = seg('Kan du ' + fmtDate(e.date) + '?', EVENT_LEVELS, e.level, (v) => setLevels({ [e.event_id]: levels[e.event_id] === v ? '' : v }));
    ctls[e.event_id] = ctl;
    return h('li', { class: 'avail-row' },
      h('div', { class: 'what' }, h('strong', { class: 'when' }, fmtDate(e.date)), h('span', { class: 'muted small' }, e.day_type + (e.note ? ' · ' + e.note : ''))), ctl.el);
  });
  const all = (v) => async () => {
    const changes = {};
    for (const e of me.events) if ((levels[e.event_id] || '') !== v) changes[e.event_id] = v;
    if (!Object.keys(changes).length) return;
    const overwritten = me.events.filter((e) => levels[e.event_id] && levels[e.event_id] !== v).length;
    if (overwritten && !confirm(`Dette endrer ${overwritten} svar du allerede har gitt. Fortsette?`)) return;
    await setLevels(changes);
  };
  updateSummary();
  const bulk = me.events.length ? h('div', { class: 'row-inline' }, h('span', { class: 'muted small' }, 'Sett alle:'),
    h('button', { type: 'button', class: 'ghost', onclick: all('ikke') }, 'Passer ikke'), h('button', { type: 'button', class: 'ghost', onclick: all('darlig') }, 'Dårlig'),
    h('button', { type: 'button', class: 'ghost', onclick: all('godt') }, 'Godt'), h('button', { type: 'button', class: 'ghost', onclick: all('') }, 'Fjern alle')) : '';

  const root = h('div', {},
    h('p', { class: 'small' }, link(`/o/${enc(r.org)}/p/${enc(r.plan)}`, '← ' + plan.name)),
    h('h1', {}, me.person.name),
    h('p', { class: 'muted' }, 'Svarene lagres med en gang. Hendelsen gjelder, ikke dagen: har du to hendelser samme dag, svarer du på hver for seg.'),
    me.person.active ? '' : h('p', { class: 'err' }, 'Denne personen er markert som inaktiv.'),
    taskSection,
    section('Når kan du? ' + plan.name,
      me.events.length ? [h('p', { class: 'muted small' }, 'Trykk på et valg. Trykk på det samme igjen for å fjerne svaret (da regnes det som nøytralt).'), bulk, summary, list('avail', rows)] : h('p', { class: 'muted' }, 'Det er ikke lagt inn noen hendelser i denne planen ennå.')));
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
    const y = location.pathname === lastPath ? window.scrollY : 0;  // samme side på nytt (etter en endring): bli stående
    main.replaceChildren(body);
    lastPath = location.pathname;
    window.scrollTo(0, y);
    if (r.page === 'me' && body.dataset.person && !store.get('actor').trim()) {  // den som åpner sin egen side er nok personen selv
      store.set('actor', body.dataset.person);
      const a = document.getElementById('actor');
      if (a) a.value = body.dataset.person;
    }
    document.title = (r.page === 'plan' ? '' : '') + 'Tjenesteplan' + (org ? ' · ' + org.name : '');
  } catch (e) {
    if (seq !== renderSeq) return;
    if (e.status === 404 && r.org) store.set('org', '');
    main.replaceChildren(h('div', {}, h('h1', {}, e.status === 404 ? 'Fant ikke siden' : 'Noe gikk galt'), h('p', {}, link('/?bytt', 'Velg organisasjon')), h('p', { class: 'muted small' }, e.message)));
  }
}

window.addEventListener('popstate', render);
render();
