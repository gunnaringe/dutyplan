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

async function pagePlan(r, org) {
  const [plan, log] = await Promise.all([api('GET', `/orgs/${enc(r.org)}/plans/${enc(r.plan)}`), api('GET', `/orgs/${enc(r.org)}/log?limit=30&plan=${enc(r.plan)}`)]);
  return h('div', {},
    h('p', { class: 'small' }, link(`/o/${enc(r.org)}`, '← ' + org.name)),
    h('h1', {}, plan.name),
    h('p', { class: 'muted' }, 'Oppsett av oppgaver og datoer, preferanser og selve planleggingen kommer i neste faser. Planen finnes og kan velges.'),
    h('h2', {}, 'Endringslogg for planen'),
    logTable(log));
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
    const [, body] = await Promise.all([renderHeader(r, people), r.page === 'home' ? pageHome() : r.page === 'org' ? pageOrg(r, org, people) : pagePlan(r, org)]);
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
