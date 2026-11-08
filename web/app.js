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
  return h('table', { class: 'log' }, h('tbody', {}, entries.map((e) =>
    h('tr', {}, h('td', {}, fmtTime(e.at)), h('td', {}, e.actor), h('td', {}, e.action + (e.plan ? ' (' + e.plan + ')' : '')), h('td', {}, e.detail)))));
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
      h('p', { class: 'muted small' }, 'En dagtype sier hvilke oppgaver en dato har og hvor mange plasser (f.eks. «Gudstjeneste»: Lyd 1, Kafé 3).'),
      dayTypes.length ? dayTypeBlocks : h('p', { class: 'muted' }, 'Ingen dagtyper ennå.'),
      nameForm('Ny dagtype, f.eks. Gudstjeneste', 'Opprett', async (name) => { await api('POST', base + '/day-types', { name }); refresh(); })),
    section('Hva kan kombineres samme dag?',
      h('p', { class: 'muted small' }, '«Standard» regnes som upraktisk. Innstillingen er felles for alle i organisasjonen.'),
      pairRows.length ? h('table', {}, h('tbody', {}, pairRows)) : h('p', { class: 'muted' }, 'Lag minst to oppgaver først.')));
}

function dayTypeSelect(dayTypes, value, onchange, label) {
  return h('select', { 'aria-label': label, onchange }, dayTypes.map((d) => h('option', { value: String(d.id), selected: d.id === value }, d.name)));
}

async function pagePlan(r, org) {
  const base = `/orgs/${enc(r.org)}`;
  const [plan, dates, dayTypes, log] = await Promise.all([
    api('GET', `${base}/plans/${enc(r.plan)}`), api('GET', `${base}/plans/${enc(r.plan)}/dates`),
    api('GET', base + '/day-types'), api('GET', `${base}/log?limit=30&plan=${enc(r.plan)}`)]);
  const refresh = () => render();
  const dpath = `${base}/plans/${enc(r.plan)}/dates`;

  const rows = dates.map((d) => {
    const note = h('input', { type: 'text', value: d.note, maxlength: '200', 'aria-label': 'Merknad ' + d.date, onchange: async (e) => { if (await write(() => api('PATCH', `${dpath}/${d.id}`, { note: e.target.value }))) refresh(); else e.target.value = d.note; } });
    return h('tr', {}, h('td', {}, fmtDate(d.date)),
      h('td', {}, dayTypeSelect(dayTypes, d.day_type_id, async (e) => { if (await write(() => api('PATCH', `${dpath}/${d.id}`, { day_type_id: parseInt(e.target.value, 10) }))) refresh(); else e.target.value = String(d.day_type_id); }, 'Dagtype ' + d.date)),
      h('td', {}, note),
      h('td', {}, h('button', { type: 'button', class: 'danger', 'aria-label': 'Slett ' + d.date, onclick: async () => { if (await write(() => api('DELETE', `${dpath}/${d.id}`))) refresh(); } }, 'Slett')));
  });

  const noTypes = !dayTypes.length;
  let addForm = '', bulkForm = '';
  if (!noTypes) {
    const date = h('input', { type: 'date', required: true, 'aria-label': 'Dato' });
    const typeSel = h('select', { 'aria-label': 'Dagtype' }, dayTypes.map((d) => h('option', { value: String(d.id) }, d.name)));
    const noteIn = h('input', { type: 'text', placeholder: 'Merknad (valgfri)', maxlength: '200', 'aria-label': 'Merknad' });
    addForm = h('form', { class: 'row', onsubmit: async (e) => { e.preventDefault(); if (await write(() => api('POST', dpath, { date: date.value, day_type_id: parseInt(typeSel.value, 10), note: noteIn.value }))) refresh(); } },
      date, typeSel, noteIn, h('button', { type: 'submit' }, 'Legg til dato'));

    const from = h('input', { type: 'date', required: true, 'aria-label': 'Fra' });
    const to = h('input', { type: 'date', required: true, 'aria-label': 'Til' });
    const days = ['Man', 'Tir', 'Ons', 'Tor', 'Fre', 'Lør', 'Søn'].map((n, i) => ({ n, v: i + 1, box: h('input', { type: 'checkbox', checked: i === 6, 'aria-label': n }) }));
    const bType = h('select', { 'aria-label': 'Dagtype for alle' }, dayTypes.map((d) => h('option', { value: String(d.id) }, d.name)));
    bulkForm = h('form', { class: 'stack', onsubmit: async (e) => {
      e.preventDefault();
      let res;
      if (await write(async () => { res = await api('POST', dpath + '/bulk', { from: from.value, to: to.value, weekdays: days.filter((x) => x.box.checked).map((x) => x.v), day_type_id: parseInt(bType.value, 10) }); })) {
        await render(); say(`${res.created.length} nye datoer${res.skipped.length ? ', ' + res.skipped.length + ' fantes fra før' : ''}.`);
      }
    } },
      h('div', { class: 'row-inline' }, h('label', {}, 'Fra ', from), h('label', {}, 'Til ', to)),
      h('div', { class: 'row-inline' }, days.map((x) => h('label', { class: 'chip' }, x.box, ' ' + x.n))),
      h('div', { class: 'row-inline' }, bType, h('button', { type: 'submit' }, 'Legg til alle')));
  }

  return h('div', {},
    h('p', { class: 'small' }, link(`/o/${enc(r.org)}`, '← ' + org.name)),
    h('h1', {}, plan.name),
    section('Datoer',
      noTypes ? h('p', { class: 'muted' }, 'Lag minst én dagtype under ', link(`/o/${enc(r.org)}/oppsett`, 'oppsett'), ' for å legge til datoer.') : '',
      dates.length ? h('table', {}, h('thead', {}, h('tr', {}, h('th', {}, 'Dato'), h('th', {}, 'Dagtype'), h('th', {}, 'Merknad'), h('th', {}, ''))), h('tbody', {}, rows)) : h('p', { class: 'muted' }, 'Ingen datoer ennå.'),
      addForm),
    section('Legg til flere datoer (f.eks. alle søndager)', noTypes ? h('p', { class: 'muted' }, 'Trenger en dagtype.') : bulkForm),
    h('p', { class: 'muted small' }, 'Preferanser og selve planleggingen kommer i neste faser.'),
    section('Endringslogg for planen', logTable(log)));
}

let renderSeq = 0;
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
    const [, body] = await Promise.all([renderHeader(r, people), r.page === 'home' ? pageHome() : r.page === 'org' ? pageOrg(r, org, people) : r.page === 'setup' ? pageSetup(r, org) : pagePlan(r, org)]);
    if (seq !== renderSeq || !body) return;
    main.replaceChildren(body);
    document.title = (r.page === 'plan' ? '' : '') + 'Tjenesteplan' + (org ? ' · ' + org.name : '');
  } catch (e) {
    if (seq !== renderSeq) return;
    if (e.status === 404 && r.org) store.set('org', '');
    main.replaceChildren(h('div', {}, h('h1', {}, e.status === 404 ? 'Fant ikke siden' : 'Noe gikk galt'), h('p', {}, link('/?bytt', 'Velg organisasjon')), h('p', { class: 'muted small' }, e.message)));
  }
}

window.addEventListener('popstate', render);
render();
